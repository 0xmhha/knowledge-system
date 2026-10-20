#!/usr/bin/env python3
"""Capture pinned project functions and observe local rules; never run project code."""
import argparse
from collections import Counter
from functools import lru_cache
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

from source_packet import SourceCapture, safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR = Path('docs/spec-driven/operational-preparation/policy-security-drafts')
TEMPLATE = Path('scripts/fixtures/project_rule_observation_test.go.txt')
VIRTUAL = Path('pkg/graph/valueflow/rules_test.go')
CONTROL_NAMES = ['closure_sibling', 'nested_closure', 'direct_assignment', 'context_argument', 'shadowed_owner']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


old = load('run-rule-observations.py')


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def census():
    return json.loads((ROOT / PRIOR / 'private/rule-source-census.json').read_text())


def bindings():
    paths = [Path('go.mod'), Path('go.sum'), Path('.golangci.yml'), TEMPLATE, Path('scripts/source_packet.py'),
             Path('scripts/run-project-rule-review.py'), Path('scripts/run-rule-observations.py'),
             PRIOR / 'manifest.json', PRIOR / 'private/rule-source-census.json', BASE / 'rule-observations-v1/manifest.json']
    paths.extend(sorted(p.relative_to(ROOT) for p in (ROOT / 'pkg/graph/valueflow').glob('*.go')))
    return {str(p): sha((ROOT / p).read_bytes()) for p in paths}


def render_inputs(packet):
    data = census()
    proof = verify(packet, data['source_commit'])
    if set(proof['files']) != {c['path'] for c in data['calls']}:
        raise ValueError('project source file inventory mismatch')
    descriptor = {'repository_commit': data['source_commit'], 'files': [dict(path=p, **{k: proof['files'][p][k] for k in ['git_blob', 'sha256']}) for p in sorted(proof['files'])]}
    snapshot_sha = sha(json.dumps(descriptor, sort_keys=True).encode())
    groups = {}
    for call in data['calls']:
        groups.setdefault((call['path'], call['function_start_byte']), []).append(call)
    cases = []
    for index, (key, calls) in enumerate(sorted(groups.items()), 1):
        first = calls[0]
        raw = safe(packet, proof['files'][key[0]]['artifact']).read_bytes()
        a, b = key[1], first['function_end_byte_exclusive']
        code = raw[a:b]
        if sha(raw) != first['file_sha256'] or proof['files'][key[0]]['git_blob'] != first['git_blob'] or sha(code) != first['function_sha256']:
            raise ValueError('project function source hash mismatch')
        if raw[:a].count(b'\n') + 1 != first['function_start_line'] or raw[:b].count(b'\n') + 1 != first['function_end_line']:
            raise ValueError('project function source line mismatch')
        for c in calls:
            if c['function_end_byte_exclusive'] != b or c['function_sha256'] != sha(code) or c['function_start_line'] != first['function_start_line']:
                raise ValueError('same-function call metadata disagrees')
            if sha(raw[c['start_byte']:c['end_byte_exclusive']]) != c['span_sha256'] or code != safe(ROOT / PRIOR / 'private', c['source_excerpt_private']).read_bytes():
                raise ValueError('prior call excerpt/commit span mismatch')
        identity = f'PROJECT-{index:03d}'
        cases.append({'id': identity, 'code': code.decode(), 'source': {
            'node_id': identity, 'file': key[0], 'commit': data['source_commit'], 'sha256': sha(code), 'language': 'go',
            'start_line': first['function_start_line'], 'start_byte': a, 'bytes': len(code),
            'snapshot_sha256': snapshot_sha, 'function_name': first['enclosing_function'].rsplit('.', 1)[-1]}})
    if len(cases) != 51:
        raise ValueError('exactly 51 source functions required')
    return descriptor, {'kind': 'project_function_source_preparation_no_gold',
                        'snapshot_kind': 'synthetic_source_bundle_not_retained_graph', 'snapshot_sha256': snapshot_sha, 'cases': cases}


def overlay_source():
    prefix = b'package valueflow\n'
    base, template = (ROOT / VIRTUAL).read_bytes(), (ROOT / TEMPLATE).read_bytes()
    if not base.startswith(prefix) or not template.startswith(prefix):
        raise ValueError('unexpected test package')
    return prefix + b'\nimport (\n\t"go/ast"\n\t"go/parser"\n\t"go/token"\n\t"os"\n)\n' + base[len(prefix):] + b'\n' + template.split(b')\n\n', 1)[1]


def make_work(work, payload):
    work.mkdir()
    backing = work / 'rules_test.go'
    backing.write_bytes(overlay_source())
    overlay = work / 'overlay.json'
    dump(overlay, {'Replace': {str(ROOT / VIRTUAL): str(backing)}})
    (work / 'public-inputs.json').write_bytes(payload)
    return overlay, backing


def command(go, overlay, mode):
    return [str(go), 'test', '-overlay=' + str(overlay), '-json', '-count=1', '-run=^TestProjectRule',
            '-timeout=90s'] + (['-race'] if mode == 'race' else []) + ['./pkg/graph/valueflow']


def read_events(raw, ids):
    events = [json.loads(line) for line in raw.decode().splitlines() if line]
    text = ''.join(e.get('Output', '') for e in events)
    if any(e['Action'] in ['fail', 'skip'] for e in events) or 'WARNING: DATA RACE' in text:
        raise ValueError('failed/skipped/racing project source observation')
    rows, controls = [], []
    for line in text.splitlines():
        if 'PROJECT_RULE_OBSERVATION ' in line:
            rows.append(json.loads(line.split('PROJECT_RULE_OBSERVATION ', 1)[1]))
        if 'PROJECT_RULE_SYNTAX_CONTROL ' in line:
            controls.append(json.loads(line.split('PROJECT_RULE_SYNTAX_CONTROL ', 1)[1]))
    expected = {'TestProjectRulePreparation', 'TestProjectRuleSyntaxControls'} | {'TestProjectRulePreparation/' + cid for cid in ids} | {'TestProjectRuleSyntaxControls/' + cid for cid in CONTROL_NAMES}
    passed = [e.get('Test', '') for e in events if e['Action'] == 'pass']
    if len(rows) != len(ids) or {r['case_id'] for r in rows} != set(ids) or len(controls) != 5 or {r['control_id'] for r in controls} != set(CONTROL_NAMES):
        raise ValueError('incomplete/duplicate project observation matrix')
    if set(passed) != expected | {''} or len(passed) != len(expected) + 1:
        raise ValueError('native project completion inventory mismatch')
    return rows, controls


@lru_cache(maxsize=2)
def replay(public_text, binding_key):
    if binding_key != json.dumps(bindings(), sort_keys=True):
        raise ValueError('live project observer changed before replay')
    go = shutil.which('go')
    if not go:
        raise ValueError('installed compatible Go required for offline replay')
    with tempfile.TemporaryDirectory(prefix='ks-project-rule-replay-') as tmp:
        work = Path(tmp) / 'work'
        overlay, _ = make_work(work, public_text.encode())
        env = dict(old.environment(), KS_PROJECT_RULE_INPUT_PATH=str(work / 'public-inputs.json'))
        p = subprocess.run(command(go, overlay, 'normal'), cwd=ROOT, env=env, capture_output=True, timeout=180)
        if p.returncode or p.stderr:
            raise ValueError('offline project replay failed: ' + (p.stderr + p.stdout[-1000:]).decode(errors='replace'))
        return read_events(p.stdout, [r['id'] for r in json.loads(public_text)['cases']])


def location_text(code, source, loc):
    raw = code.encode()
    a, b = loc['start_byte'] - source['start_byte'], loc['end_byte'] - source['start_byte']
    if not 0 <= a < b <= len(raw) or loc['start_line'] != source['start_line'] + raw[:a].count(b'\n') or loc['end_line'] != source['start_line'] + raw[:b].count(b'\n'):
        raise ValueError('project source span/line outside function')
    return raw[a:b].decode()


def derive(packet, observations):
    _, payload = render_inputs(packet)
    inputs = {c['id']: c for c in payload['cases']}
    calls = census()['calls']
    all_notes, candidates, function_review = {}, [], []
    pack = json.loads((ROOT / BASE / 'rule-coverage-v1/contract.json').read_text())['pack_definition_sha256']
    for row in observations:
        identity, analysis = row['case_id'], row['analysis']
        case, rules = inputs[identity], analysis['rules']
        if analysis['source'] != case['source'] or analysis['elapsed_micros'] != 0 or row['repeat_equal_except_elapsed'] is not True:
            raise ValueError('project source identity/determinism mismatch')
        if rules['pack_sha256'] != pack or rules['engine_version'] != '2' or rules['origin'] != 'automatic':
            raise ValueError('project rule pack identity mismatch')
        source, code = case['source'], case['code']
        local_calls = [c for c in calls if c['path'] == source['file'] and c['function_start_byte'] == source['start_byte']]
        for note in row['syntax_calls']:
            loc = note['location']
            matches = [c for c in local_calls if c['start_byte'] == loc['start_byte'] and c['end_byte_exclusive'] == loc['end_byte']]
            if len(matches) != 1:
                raise ValueError('AST call span is absent/ambiguous in frozen census')
            c = matches[0]
            text = location_text(code, source, loc)
            if sha(text.encode()) != c['span_sha256'] or note['owner'] != c['import_alias'] or note['callee'] != c['callee'] or loc['start_line'] != c['start_line'] or loc['end_line'] != c['end_line']:
                raise ValueError('AST call source binding mismatch')
            if c['id'] in all_notes:
                raise ValueError('duplicate call observation')
            for arg in note['arguments']:
                if location_text(code, source, arg['location']) != arg['text']:
                    raise ValueError('argument source binding mismatch')
            all_notes[c['id']] = {'case_id': identity, 'note': note, 'analysis': analysis, 'raw_call_text': text}
        nodes = {n['id']: n for n in analysis['nodes']}
        occurrences = {o['id']: o for o in analysis['occurrences']}
        local_candidates = []
        for candidate in rules['candidates']:
            definition, use = occurrences[candidate['source_occurrence']], occurrences[candidate['sink_occurrence']]
            if definition['role'] != 'definition' or use['role'] != 'use' or not definition['reachable'] or not use['reachable'] or definition['location'] != candidate['source_location'] or use['location'] != candidate['sink_location']:
                raise ValueError('project candidate occurrence identity mismatch')
            if candidate['origin'] != 'automatic' or candidate['certainty'] != 'lexical_may_reach' or candidate['target_resolution'] != 'syntax_only_unverified' or candidate['sanitizer'] != 'not_proven':
                raise ValueError('project candidate semantic overclaim')
            path = candidate['path']
            if len(path) < 2 or any(n not in nodes for n in path) or path[0] != definition['node'] or path[-1] != use['node']:
                raise ValueError('project candidate path ownership mismatch')
            expected = sha('/'.join([pack, identity, source['snapshot_sha256'], source['sha256'], candidate['rule_id'], definition['id'], use['id']]).encode())
            if candidate['id'] != expected:
                raise ValueError('project candidate pin hash mismatch')
            mapped = []
            for c in local_calls:
                if candidate['rule_id'] == 'parameter-command-name' and c['import_path_spelling'] == 'os/exec':
                    hit = c['start_byte'] <= use['location']['start_byte'] < use['location']['end_byte'] <= c['end_byte_exclusive']
                elif candidate['rule_id'] == 'discarded-conversion-error' and c['import_path_spelling'] == 'strconv':
                    loc = nodes[definition['node']]['location']
                    hit = loc['start_byte'] <= c['start_byte'] < c['end_byte_exclusive'] <= loc['end_byte']
                else:
                    hit = False
                if hit:
                    mapped.append(c['id'])
            if len(mapped) != 1:
                raise ValueError('candidate call ownership unresolved; do not first-match')
            conditions = []
            for condition in candidate['conditions']:
                if not any(e['kind'] == 'control_dependence' and e['from'] == condition['node'] and e['to'] in path and e.get('condition', '') == condition['condition'] for e in analysis['edges']):
                    raise ValueError('project candidate condition ownership mismatch')
                if condition['location_status'] == 'source_node' and condition['location'] == nodes[condition['node']]['location']:
                    text = location_text(code, source, condition['location'])
                elif condition['location_status'] == 'structural_block_unlocated' and condition.get('location') is None and condition['node'] not in nodes:
                    text = None
                else:
                    raise ValueError('project condition source location fabricated')
                conditions.append({'condition': condition, 'source_text': text})
            detail = {'case_id': identity, 'call_id': mapped[0], 'candidate': candidate,
                      'source_text': location_text(code, source, candidate['source_location']),
                      'sink_text': location_text(code, source, candidate['sink_location']), 'conditions': conditions,
                      'runtime_reachability': 'not_established', 'safety_verdict': 'unknown', 'gold': False, 'approved': False}
            local_candidates.append(candidate['id'])
            candidates.append(detail)
        function_review.append({'case_id': identity, 'file': source['file'], 'function_name': source['function_name'],
                                'source_sha256': source['sha256'], 'base_status': analysis['status'], 'rules_status': rules['status'],
                                'stop_reason': rules.get('stop_reason'), 'truncated': analysis['truncated'],
                                'flow_steps': rules['flow_steps'], 'limits': rules['limits'], 'capabilities': analysis['capability'],
                                'diagnostic_codes': [d['code'] for d in analysis['diagnostics']], 'call_ids': [c['id'] for c in local_calls],
                                'candidate_ids': local_candidates, 'runtime_reachability': 'not_established',
                                'independence_approved': False, 'gold': False, 'split': None})
    if set(all_notes) != {c['id'] for c in calls}:
        raise ValueError('project call coverage missing; preserve original denominator')
    reviews = []
    for c in calls:
        entry = all_notes[c['id']]
        note, analysis = entry['note'], entry['analysis']
        matched = [r['candidate']['id'] for r in candidates if r['call_id'] == c['id']]
        if note['closure_depth']:
            status = 'closure_body_excluded_not_confirmed_negative'
        elif analysis['rules']['status'] != 'matched_bounded':
            status = 'incomplete_rule_analysis_not_confirmed_negative'
        elif matched:
            status = 'candidate_observed_not_confirmed_vulnerability'
        else:
            status = 'empty_bounded_candidates_not_confirmed_negative_or_safe'
        command_call = c['import_path_spelling'] == 'os/exec'
        idx = 1 if c['callee'] == 'CommandContext' else 0
        args = note['arguments']
        hints = {'command_name_direct_identifier': args[idx]['identifier'] if command_call and len(args) > idx else None,
                 'conversion_blank_second_lhs': len(note['assignment_lhs']) == 2 and note['assignment_lhs'][1] == '_' if not command_call else None,
                 'callee_identity': 'syntax_only_unverified', 'sanitizer_effects': 'not_proven'}
        reviews.append({'call_id': c['id'], 'case_id': entry['case_id'], 'source': c, 'raw_call_text': entry['raw_call_text'],
                        'syntax': note, 'syntax_hints_not_semantic_gold': hints, 'observation_status': status,
                        'rules_status': analysis['rules']['status'], 'stop_reason': analysis['rules'].get('stop_reason'),
                        'candidate_ids': matched, 'independent_label': None, 'runtime_reachability': 'not_established',
                        'safety_verdict': 'unknown', 'approved': False, 'gold': False, 'split': None})
    summary = {'kind': 'project_local_source_rule_observations_not_independent_quality_or_runtime',
               'counts': {'source_functions': len(function_review), 'source_files': len({r['file'] for r in function_review}),
                          'original_call_denominator': len(calls), 'observed_candidates_per_mode': len(candidates),
                          'functions_with_candidates': sum(bool(r['candidate_ids']) for r in function_review),
                          'calls_with_candidates': sum(bool(r['candidate_ids']) for r in reviews),
                          'closure_calls': sum(r['syntax']['closure_depth'] > 0 for r in reviews),
                          'incomplete_rule_functions': sum(r['rules_status'] != 'matched_bounded' for r in function_review),
                          'calls_in_incomplete_functions': sum(r['rules_status'] != 'matched_bounded' for r in reviews),
                          'independent_positive_gold': 0, 'independent_negative_gold': 0, 'approved_independent_cases': 0},
               'by_rule': {rule: {'original_call_denominator': sum(r['source']['import_path_spelling'] == imported for r in reviews),
                                 'calls_with_candidates': sum(r['source']['import_path_spelling'] == imported and bool(r['candidate_ids']) for r in reviews),
                                 'independent_positive_gold': 0, 'independent_negative_gold': 0} for rule, imported in [('parameter-command-name', 'os/exec'), ('discarded-conversion-error', 'strconv')]},
               'base_status_counts': dict(Counter(r['base_status'] for r in function_review)),
               'rule_status_counts': dict(Counter(r['rules_status'] for r in function_review)),
               'diagnostic_counts': dict(Counter(code for r in function_review for code in r['diagnostic_codes'])),
               'call_status_counts': dict(Counter(r['observation_status'] for r in reviews)),
               'synthetic_snapshot_not_retained_graph': True, 'controlled_cases_added_to_project_denominator': 0,
               'formal_runs': 0, 'mcp_calls': 0, 'model_calls': 0, 'nodes_started': 0, 'analyzed_project_code_executed': False,
               'precision': None, 'recall': None, 'confusion_matrix': None, 'runtime_vulnerabilities_confirmed': None,
               'quality_status': 'not_scored_independent_gold_and_project_positive_sufficiency_unestablished',
               'limits': ['51 exposed functions and 68 calls are not independent examples', 'no candidates or incomplete analysis cannot become confirmed negative labels',
                          'candidate detection does not prove attacker control, resolved callee, sanitizer effect or executable path',
                          'source commit/byte pins are real; node IDs and bundle snapshot are synthetic, not retained graph/MCP proof',
                          'build/platform/generated/closure boundaries and external caller reachability need separate review',
                          'controlled examples cannot repair project positive sufficiency or independent gold']}
    return {'coverage-summary.json': summary, 'function-review.json': function_review, 'call-review.json': reviews,
            'candidate-review.json': candidates}


def run(repo, output, goroot, lint):
    if output.exists():
        raise ValueError('fresh output required')
    errors = load('check-rule-observations.py').check(ROOT / BASE / 'rule-observations-v1')
    if errors:
        raise ValueError('; '.join(errors))
    packet, work = output / 'packet', output / 'work'
    (packet / 'logs').mkdir(parents=True)
    data = census()
    capture = SourceCapture(repo, packet, data['source_commit'])
    for path in sorted({c['path'] for c in data['calls']}):
        capture.source(path)
    dump(packet / 'git-proof.json', capture.proof())
    descriptor, payload = render_inputs(packet)
    dump(packet / 'source-bundle.json', descriptor)
    dump(packet / 'public-bundle/public-inputs.json', payload)
    public = (packet / 'public-bundle/public-inputs.json').read_bytes()
    overlay, backing = make_work(work, public)
    env = dict(old.environment(goroot), KS_PROJECT_RULE_INPUT_PATH=str(work / 'public-inputs.json'))
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit Go 1.26.9 required for recorded run')
    reference = subprocess.check_output(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip()
    if subprocess.check_output(['git', '-C', str(ROOT), 'diff', reference, '--', '*.go']):
        raise ValueError('maintained Go source changed')
    checks = []
    commands = {'build': [str(go), 'test', '-overlay=' + str(overlay), '-c', '-o', str(work / 'project.test'), './pkg/graph/valueflow'],
                'fmt': [str(goroot / 'bin/gofmt'), '-l', str(backing)], 'vet': [str(go), 'vet', '-overlay=' + str(overlay), './pkg/graph/valueflow'],
                'lint': [str(lint), 'run', '--timeout=120s', './pkg/graph/valueflow']}
    for kind, cmd in commands.items():
        p = subprocess.run(cmd, cwd=ROOT, env=dict(env, GOFLAGS='-overlay=' + str(overlay)) if kind == 'lint' else env, capture_output=True, timeout=180)
        for stream in ['stdout', 'stderr']:
            (packet / f'logs/{kind}.{stream}.txt').write_bytes(getattr(p, stream))
        if p.returncode or (kind == 'fmt' and p.stdout.strip()):
            raise ValueError('project observer quality failed: ' + kind + '; logs retained')
        checks.append({'kind': kind, 'exit_code': 0, 'command': cmd[1:]})
        print(kind + ': passed', flush=True)
    observations, controls, runs = [], [], []
    for mode in ['normal', 'race']:
        cmd = command(go, overlay, mode)
        p = subprocess.run(cmd, cwd=ROOT, env=env, capture_output=True, timeout=180)
        (packet / f'logs/{mode}.jsonl').write_bytes(p.stdout)
        (packet / f'logs/{mode}.stderr.txt').write_bytes(p.stderr)
        if p.returncode:
            raise ValueError('project observation failed: ' + mode + '; logs retained')
        rows, notes = read_events(p.stdout, [r['id'] for r in payload['cases']])
        observations.append(rows)
        controls.append(notes)
        runs.append({'mode': mode, 'exit_code': 0, 'command': cmd[1:], 'function_observations': 51, 'syntax_controls': 5})
        print(mode + ': 51 source functions and 5 syntax controls passed', flush=True)
    if observations[0] != observations[1] or controls[0] != controls[1]:
        raise ValueError('project normal/race results differ')
    for name, value in derive(packet, observations[0]).items():
        dump(packet / name, value)
    dump(packet / 'observations.json', observations[0])
    dump(packet / 'syntax-controls.json', controls[0])
    dump(packet / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(packet / 'validation.json', {'kind': 'project_source_local_kernel_not_runtime_mcp_or_independent_acceptance',
         'go_version': version, 'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(),
         'go_source_reference_commit': reference, 'quality_checks': checks, 'runs': runs,
         'normalized_normal_race_equal': True, 'existing_tests_preserved': True, 'maintained_go_source_unchanged': True,
         'source_analysis_invocations_per_mode': 102, 'analyzed_project_code_executed': False,
         'public_input_scope': 'source_packaging_separation_not_os_filesystem_isolation'})
    for name, raw in [('template.go.txt', (ROOT / TEMPLATE).read_bytes()), ('overlay-base.go.txt', (ROOT / VIRTUAL).read_bytes()),
                      ('overlay-backing.go.txt', backing.read_bytes()), ('overlay.json.txt', overlay.read_bytes()), ('runner.py.txt', Path(__file__).read_bytes())]:
        (packet / name).write_bytes(raw)
    dump(packet / 'manifest.json', {'schema_version': 1, 'kind': 'project_rule_source_preparation_only',
         'repository_commit': data['source_commit'], 'input_bindings': bindings(),
         'counts': {'functions': 51, 'files': 34, 'original_calls': 68, 'function_observations': 102, 'syntax_control_observations': 10, 'quality_checks': 4},
         'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}})
    print('project source packet ready: ' + str(packet), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['repo', 'output', 'goroot', 'lint']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    run(args.repo.resolve(), args.output.resolve(), args.goroot.resolve(), args.lint.resolve())
