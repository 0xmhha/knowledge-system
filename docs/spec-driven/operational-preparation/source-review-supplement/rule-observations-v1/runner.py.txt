#!/usr/bin/env python3
"""Observe the real bounded rule analyzer on owned strings, never execute strings."""
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

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
INPUT = BASE / 'rule-coverage-v1'
TEMPLATE = Path('scripts/fixtures/rule_observation_test.go.txt')
VIRTUAL = Path('pkg/graph/valueflow/rules_test.go')
CONTROL_NAMES = ['candidate_cap', 'flow_cap', 'node_cap', 'cancelled', 'default_off', 'pack_rejected',
                 'candidate_bound_rejected', 'flow_bound_rejected', 'node_identity', 'snapshot_identity',
                 'blob_identity', 'range_unknown', 'unsupported_language', 'invalid_syntax']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def bindings():
    paths = [Path('go.mod'), Path('go.sum'), Path('.golangci.yml'), TEMPLATE,
             Path('scripts/run-rule-observations.py'), INPUT / 'manifest.json', INPUT / 'public-inputs.json']
    paths.extend(sorted(p.relative_to(ROOT) for p in (ROOT / 'pkg/graph/valueflow').glob('*.go')))
    return {str(p): sha((ROOT / p).read_bytes()) for p in paths}


def environment(goroot=None):
    env = dict(os.environ, GOTOOLCHAIN='local', GOENV='off', GOWORK='off', GOFLAGS='', GOPROXY='off',
               GOSUMDB='off', GOTELEMETRY='off', GOCACHE='/private/tmp/ks-corpus-go-cache',
               GOLANGCI_LINT_CACHE='/private/tmp/ks-collection-lint-cache')
    if goroot is not None:
        env.update(GOROOT=str(goroot), PATH=str(goroot / 'bin') + os.pathsep + env.get('PATH', ''))
    return env


def overlay_source():
    base = (ROOT / VIRTUAL).read_bytes()
    template = (ROOT / TEMPLATE).read_bytes()
    prefix = b'package valueflow\n'
    if not base.startswith(prefix) or not template.startswith(prefix):
        raise ValueError('unexpected test package')
    body = template.split(b')\n\n', 1)[1]
    # Existing imports already provide context/json/reflect/strings/testing.
    return prefix + b'\nimport "os"\n' + base[len(prefix):] + b'\n' + body


def make_work(work, public):
    work.mkdir()
    backing = work / 'rules_test.go'
    backing.write_bytes(overlay_source())
    dump(work / 'overlay.json', {'Replace': {str(ROOT / VIRTUAL): str(backing)}})
    (work / 'public-inputs.json').write_bytes(public)
    return work / 'overlay.json', backing


def read_events(raw):
    events = [json.loads(line) for line in raw.decode().splitlines() if line]
    text = ''.join(e.get('Output', '') for e in events)
    if any(e['Action'] in ['fail', 'skip'] for e in events) or 'WARNING: DATA RACE' in text:
        raise ValueError('failed/skipped/racing native observation')
    observations, controls = [], []
    for line in text.splitlines():
        if 'RULE_OBSERVATION ' in line:
            observations.append(json.loads(line.split('RULE_OBSERVATION ', 1)[1]))
        if 'RULE_CONTROL ' in line:
            controls.append(json.loads(line.split('RULE_CONTROL ', 1)[1]))
    ids = {r['id'] for r in json.loads((ROOT / INPUT / 'public-inputs.json').read_text())['cases']}
    expected_passes = {'TestRulePreparationInputs', 'TestRulePreparationControls'} | {
        'TestRulePreparationInputs/' + cid for cid in ids} | {'TestRulePreparationControls/' + cid for cid in CONTROL_NAMES}
    passes = [e.get('Test', '') for e in events if e['Action'] == 'pass']
    if len(observations) != 40 or {r['case_id'] for r in observations} != ids or len(controls) != 14 or {r['control_id'] for r in controls} != set(CONTROL_NAMES):
        raise ValueError('incomplete/duplicate observation matrix')
    if set(passes) != expected_passes | {''} or len(passes) != len(expected_passes) + 1:
        raise ValueError('missing/repeated native completion events')
    return observations, controls


def command(go, overlay, mode):
    return [str(go), 'test', '-overlay=' + str(overlay), '-json', '-count=1', '-run=^TestRulePreparation',
            '-timeout=60s'] + (['-race'] if mode == 'race' else []) + ['./pkg/graph/valueflow']


@lru_cache(maxsize=2)
def replay(public_text, binding_key):
    # Only the trusted live template is compiled. Evidence never chooses executable code.
    if binding_key != json.dumps(bindings(), sort_keys=True):
        raise ValueError('live analyzer/template changed before replay')
    go = shutil.which('go')
    if not go:
        raise ValueError('installed compatible Go required for offline replay')
    with tempfile.TemporaryDirectory(prefix='ks-rule-replay-') as tmp:
        work = Path(tmp) / 'work'
        overlay, _ = make_work(work, public_text.encode())
        env = dict(environment(), KS_RULE_INPUT_PATH=str(work / 'public-inputs.json'))
        p = subprocess.run(command(go, overlay, 'normal'), cwd=ROOT, env=env, capture_output=True, timeout=180)
        if p.returncode or p.stderr:
            raise ValueError('offline native replay failed: ' + (p.stderr + p.stdout[-1000:]).decode(errors='replace'))
        return read_events(p.stdout)


def location_text(code, loc):
    a, b = loc['start_byte'] - 400, loc['end_byte'] - 400
    raw = code.encode()
    if not 0 <= a < b <= len(raw) or loc['start_line'] != 20 + raw[:a].count(b'\n') or loc['end_line'] != 20 + raw[:b].count(b'\n'):
        raise ValueError('retained fixture location outside source')
    return raw[a:b].decode()


def derive(observations):
    public = {r['id']: r for r in json.loads((ROOT / INPUT / 'public-inputs.json').read_text())['cases']}
    private = {r['id']: r for r in json.loads((ROOT / INPUT / 'private-ai-review.json').read_text())['cases']}
    pack = json.loads((ROOT / INPUT / 'contract.json').read_text())['pack_definition_sha256']
    reviewed, disagreements = [], []
    for record in observations:
        cid, analysis = record['case_id'], record['analysis']
        code, proposed = public[cid]['code'], private[cid]
        source = analysis['source']
        if source['sha256'] != sha(code.encode()) or source['bytes'] != len(code.encode()) or source['node_id'] != cid or source['snapshot_sha256'] != 'c' * 64 or source['start_byte'] != 400 or source['start_line'] != 20:
            raise ValueError('synthetic retained source pin mismatch')
        if analysis['elapsed_micros'] != 0 or record['repeat_equal_except_elapsed'] is not True:
            raise ValueError('timing normalization/determinism mismatch')
        rules = analysis['rules']
        if rules['pack_sha256'] != pack or rules['engine_version'] != '2' or rules['origin'] != 'automatic' or rules['status'] != 'matched_bounded':
            raise ValueError('unexpected rule pack/completion')
        nodes = {n['id']: n for n in analysis['nodes']}
        occurrences = {o['id']: o for o in analysis['occurrences']}
        spans = []
        for candidate in rules['candidates']:
            if candidate['origin'] != 'automatic' or candidate['certainty'] != 'lexical_may_reach' or candidate['target_resolution'] != 'syntax_only_unverified' or candidate['sanitizer'] != 'not_proven':
                raise ValueError('candidate certainty/target/sanitizer overclaim')
            definition, use = occurrences[candidate['source_occurrence']], occurrences[candidate['sink_occurrence']]
            if definition['role'] != 'definition' or use['role'] != 'use' or not definition['reachable'] or not use['reachable'] or definition['location'] != candidate['source_location'] or use['location'] != candidate['sink_location']:
                raise ValueError('candidate occurrence identity mismatch')
            path = candidate['path']
            if len(path) < 2 or any(n not in nodes for n in path) or path[0] != definition['node'] or path[-1] != use['node']:
                raise ValueError('candidate path ownership mismatch')
            expected_id = sha('/'.join([pack, cid, source['snapshot_sha256'], source['sha256'], candidate['rule_id'], definition['id'], use['id']]).encode())
            if candidate['id'] != expected_id:
                raise ValueError('candidate pin hash mismatch')
            conditions = []
            for condition in candidate['conditions']:
                if not any(e['kind'] == 'control_dependence' and e['from'] == condition['node'] and e['to'] in path and e.get('condition', '') == condition['condition'] for e in analysis['edges']):
                    raise ValueError('candidate condition lacks structural control edge')
                if condition['location_status'] == 'source_node':
                    if condition['location'] != nodes[condition['node']]['location']:
                        raise ValueError('condition source location mismatch')
                    text = location_text(code, condition['location'])
                elif condition['location_status'] == 'structural_block_unlocated' and condition.get('location') is None and condition['node'] not in nodes:
                    text = None
                else:
                    raise ValueError('fabricated/unknown condition location status')
                conditions.append({'condition': condition, 'source_text': text})
            spans.append({'candidate_id': candidate['id'], 'rule_id': candidate['rule_id'],
                          'source_text': location_text(code, candidate['source_location']),
                          'sink_text': location_text(code, candidate['sink_location']), 'conditions': conditions,
                          'path': path, 'safety_verdict': 'unknown'})
        actual = len([c for c in rules['candidates'] if c['rule_id'] == proposed['rule_proposal']])
        if actual != len(rules['candidates']):
            raise ValueError('unexpected other-rule candidate in fixture')
        matches = actual == proposed['candidate_count_proposal']
        row = {'case_id': cid, 'mechanism': proposed['mechanism'], 'coverage_tags': proposed['coverage_tags'],
               'rule': proposed['rule_proposal'], 'proposed_candidates': proposed['candidate_count_proposal'],
               'observed_candidates': actual, 'matches_ai_proposal': matches,
               'base_status': analysis['status'], 'capabilities': analysis['capability'],
               'diagnostic_codes': [d['code'] for d in analysis['diagnostics']], 'candidate_spans': spans,
               'gold': False, 'approved': False, 'reviewer': None, 'safety_verdict': 'unknown', 'split': None}
        reviewed.append(row)
        if not matches:
            disagreements.append(row)
    summary = {'kind': 'controlled_analyzer_observations_not_independent_quality_or_project_effect',
               'counts': {'controlled_sources': len(reviewed), 'distinct_controlled_sources_with_candidates': sum(r['observed_candidates'] > 0 for r in reviewed),
                          'candidates_per_mode': sum(r['observed_candidates'] for r in reviewed),
                          'ai_proposal_matches': sum(r['matches_ai_proposal'] for r in reviewed),
                          'ai_proposal_disagreements': len(disagreements), 'independent_gold_records': 0, 'approved_independent_cases': 0},
               'by_rule': {rule: {'inputs': sum(r['rule'] == rule for r in reviewed),
                                 'with_candidates': sum(r['rule'] == rule and r['observed_candidates'] > 0 for r in reviewed),
                                 'empty': sum(r['rule'] == rule and r['observed_candidates'] == 0 for r in reviewed)} for rule in sorted({r['rule'] for r in reviewed})},
               'diagnostic_counts': dict(Counter(code for r in reviewed for code in r['diagnostic_codes'])),
               'project_cases_analyzed': 0, 'mcp_calls': 0, 'model_calls': 0, 'nodes_started': 0,
               'formal_runs': 0, 'precision': None, 'recall': None, 'confusion_matrix': None,
               'analyzed_source_executed': False, 'latency_measurement': 'elapsed_micros_zeroed_not_benchmarked',
               'limits': ['AI proposal agreement is not accuracy or independent gold', 'controlled examples do not fill project positive denominators',
                          'synthetic retained offsets and snapshot pins are not a real retained graph or MCP round trip',
                          'isolated function type diagnostics remain partial; callee identity and sanitizer effects unresolved',
                          'native analyzer determinism and bounds do not prove runtime safety or production acceptance']}
    return {'coverage-summary.json': summary, 'case-review.json': reviewed,
            'disagreements.json': {'kind': 'observed_vs_ai_proposal_not_gold_errors', 'cases': disagreements}}


def run(output, goroot, lint):
    if output.exists():
        raise ValueError('fresh output required; preserve prior attempts')
    errors = load('check-rule-coverage.py').check(ROOT / INPUT)
    if errors:
        raise ValueError('; '.join(errors))
    packet, work = output / 'packet', output / 'work'
    (packet / 'logs').mkdir(parents=True)
    public = (ROOT / INPUT / 'public-inputs.json').read_bytes()
    overlay, backing = make_work(work, public)
    (packet / 'public-bundle').mkdir()
    (packet / 'public-bundle/public-inputs.json').write_bytes(public)
    env = dict(environment(goroot), KS_RULE_INPUT_PATH=str(work / 'public-inputs.json'))
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit Go 1.26.9 required for recorded run')
    reference = subprocess.check_output(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip()
    if subprocess.check_output(['git', '-C', str(ROOT), 'diff', reference, '--', '*.go']):
        raise ValueError('maintained Go source changed')
    checks = []
    commands = {'build': [str(go), 'test', '-overlay=' + str(overlay), '-c', '-o', str(work / 'rules.test'), './pkg/graph/valueflow'],
                'fmt': [str(goroot / 'bin/gofmt'), '-l', str(backing)],
                'vet': [str(go), 'vet', '-overlay=' + str(overlay), './pkg/graph/valueflow'],
                'lint': [str(lint), 'run', '--timeout=120s', './pkg/graph/valueflow']}
    for kind, cmd in commands.items():
        run_env = dict(env, GOFLAGS='-overlay=' + str(overlay)) if kind == 'lint' else env
        p = subprocess.run(cmd, cwd=ROOT, env=run_env, capture_output=True, timeout=180)
        for stream in ['stdout', 'stderr']:
            (packet / f'logs/{kind}.{stream}.txt').write_bytes(getattr(p, stream))
        if p.returncode or (kind == 'fmt' and p.stdout.strip()):
            raise ValueError('native quality failed: ' + kind + '; see preserved logs')
        checks.append({'kind': kind, 'exit_code': 0, 'command': cmd[1:]})
        print(kind + ': passed', flush=True)
    runs, all_observations, all_controls = [], [], []
    for mode in ['normal', 'race']:
        cmd = command(go, overlay, mode)
        p = subprocess.run(cmd, cwd=ROOT, env=env, capture_output=True, timeout=180)
        (packet / f'logs/{mode}.jsonl').write_bytes(p.stdout)
        (packet / f'logs/{mode}.stderr.txt').write_bytes(p.stderr)
        if p.returncode:
            raise ValueError('native observation failed: ' + mode + '; see preserved logs')
        observations, controls = read_events(p.stdout)
        all_observations.append(observations)
        all_controls.append(controls)
        runs.append({'mode': mode, 'exit_code': 0, 'command': cmd[1:], 'source_observations': 40, 'control_observations': 14})
        print(mode + ': 40 source observations and 14 controls passed', flush=True)
    if all_observations[0] != all_observations[1] or all_controls[0] != all_controls[1]:
        raise ValueError('normal/race normalized results differ')
    regressions = []
    for mode in ['normal', 'race']:
        cmd = [str(go), 'test', '-json', '-count=1', '-run=^TestRule(Pack|Bounds|Query|Range)', '-timeout=60s'] + (['-race'] if mode == 'race' else []) + ['./pkg/graph/valueflow']
        p = subprocess.run(cmd, cwd=ROOT, env=env, capture_output=True, timeout=180)
        (packet / f'logs/regression-{mode}.jsonl').write_bytes(p.stdout)
        (packet / f'logs/regression-{mode}.stderr.txt').write_bytes(p.stderr)
        events = [json.loads(line) for line in p.stdout.decode().splitlines() if line]
        if p.returncode or p.stderr or any(e['Action'] in ['fail', 'skip'] for e in events):
            raise ValueError('existing rule regressions failed: ' + mode)
        passed = [e['Test'] for e in events if e['Action'] == 'pass' and e.get('Test')]
        regressions.append({'mode': mode, 'exit_code': 0, 'command': cmd[1:], 'named_tests_passed': len(passed)})
        print(mode + ': existing rule regression completions ' + str(len(passed)), flush=True)
    for name, value in derive(all_observations[0]).items():
        dump(packet / name, value)
    dump(packet / 'observations.json', all_observations[0])
    dump(packet / 'controls.json', all_controls[0])
    dump(packet / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(packet / 'validation.json', {'kind': 'actual_controlled_analyzer_not_project_mcp_or_independent_acceptance',
         'go_version': version, 'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(),
         'go_source_reference_commit': reference, 'quality_checks': checks, 'runs': runs, 'existing_rule_regressions': regressions,
         'normalized_normal_race_equal': True, 'existing_tests_preserved': True, 'maintained_go_source_unchanged': True,
         'public_input_scope': 'packaging_separation_not_os_filesystem_isolation',
         'source_analysis_invocations_per_mode': 80, 'analyzed_source_executed': False})
    for name, raw in [('template.go.txt', (ROOT / TEMPLATE).read_bytes()), ('overlay-base.go.txt', (ROOT / VIRTUAL).read_bytes()),
                      ('overlay-backing.go.txt', backing.read_bytes()), ('overlay.json.txt', overlay.read_bytes()), ('runner.py.txt', Path(__file__).read_bytes())]:
        (packet / name).write_bytes(raw)
    dump(packet / 'manifest.json', {'schema_version': 1, 'kind': 'controlled_rule_observation_preparation_only',
         'input_bindings': bindings(), 'counts': {'sources': 40, 'modes': 2, 'source_observations': 80, 'control_observations': 28, 'quality_checks': 4},
         'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}})
    print('observation packet ready: ' + str(packet), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['output', 'goroot', 'lint']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    run(args.output.resolve(), args.goroot.resolve(), args.lint.resolve())
