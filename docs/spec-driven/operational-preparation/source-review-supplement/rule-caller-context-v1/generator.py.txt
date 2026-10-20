#!/usr/bin/env python3
"""Prepare source-bound caller syntax and unapproved command-contract notes."""
import argparse
from collections import Counter
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR = BASE / 'project-rule-review-v1'
PATHS = ['build/ci.go', 'internal/build/util.go']
HELPER = Path('scripts/rule_caller_context.go.txt')
CONTROLS = Path('scripts/fixtures/rule_caller_context_test.go.txt')
CURATION = Path('scripts/fixtures/rule-caller-context-proposals.json')
CONTROL_NAMES = ['literal', 'renamed_import', 'comments_strings', 'value_reference', 'unqualified', 'shadowed_owner', 'nested_closure', 'foreign_import', 'crlf_utf8']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


old = load('run-rule-observations.py')
replay = load('check-lexical-coverage.py').replay


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def bindings():
    paths = [HELPER, CONTROLS, CURATION, Path('scripts/prepare-rule-caller-context.py'), Path('scripts/source_packet.py'),
             Path('scripts/run-rule-observations.py'), Path('scripts/check-lexical-coverage.py'), PRIOR / 'manifest.json']
    return {str(p): sha((ROOT / p).read_bytes()) for p in paths}


def source_data():
    manifest = json.loads((ROOT / PRIOR / 'manifest.json').read_text())
    for name, digest in manifest['artifacts'].items():
        if sha(safe(ROOT / PRIOR, name).read_bytes()) != digest:
            raise ValueError('prior project evidence hash mismatch')
    for name, digest in manifest['input_bindings'].items():
        if sha(safe(ROOT, name).read_bytes()) != digest:
            raise ValueError('prior project live input binding mismatch')
    proof = verify(ROOT / PRIOR, manifest['repository_commit'])
    payload = [{'Path': p, 'Source': safe(ROOT / PRIOR, proof['files'][p]['artifact']).read_bytes().decode()} for p in PATHS]
    return manifest['repository_commit'], proof, payload


def span(raw, start, end):
    lines = raw.splitlines(keepends=True)
    if not 1 <= start <= end <= len(lines):
        raise ValueError('context anchor line outside source')
    a, b = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
    return {'start_line': start, 'end_line': end, 'start_byte': a, 'end_byte_exclusive': b,
            'span_sha256': sha(raw[a:b]), 'source_text': raw[a:b].decode()}


def site_text(raw, site):
    a, b = site['start_byte'], site['end_byte_exclusive']
    if not 0 <= a < b <= len(raw) or raw[:a].count(b'\n') + 1 != site['start_line'] or raw[:b].count(b'\n') + 1 != site['end_line']:
        raise ValueError('caller source span mismatch')
    return raw[a:b].decode()


def derive(observations):
    revision, proof, payload = source_data()
    sources = {r['Path']: r['Source'].encode() for r in payload}
    if [r['path'] for r in observations] != PATHS:
        raise ValueError('caller source observation inventory mismatch')
    references, declarations, contexts, tags = [], [], [], []
    for row in observations:
        raw = sources[row['path']]
        if sha(raw) != row['sha256'] or sha(raw) != proof['files'][row['path']]['sha256']:
            raise ValueError('caller observed source hash mismatch')
        for kind, target in [('target_declarations', declarations), ('context_functions', contexts), ('build_constraints', tags)]:
            for anchor in row[kind]:
                target.append(dict(path=row['path'], **anchor, source_text=site_text(raw, anchor['site'])))
        for ref in row['references']:
            if ref['resolution'] != 'syntax_only_unverified':
                raise ValueError('caller target binding overclaim')
            record = dict(path=row['path'], **ref, source_text=site_text(raw, ref['site']))
            if ref.get('function_site'):
                site_text(raw, ref['function_site'])
            if arg := ref.get('first_argument'):
                if site_text(raw, arg['site']) != arg['text']:
                    raise ValueError('caller argument source mismatch')
            references.append(record)
    for index, record in enumerate(references, 1):
        record.update(id=f'CALLER-{index:03d}', runtime_reachability='not_established', attacker_control='not_established',
                      vulnerability_verdict=None, independent_label=None, approved=False, gold=False)
    notes = []
    for proposal in json.loads((ROOT / CURATION).read_text()):
        anchors = [dict(path=p, git_blob=proof['files'][p]['git_blob'], file_sha256=sha(sources[p]), **span(sources[p], a, b)) for p, a, b in proposal['anchors']]
        notes.append({**proposal, 'anchors': anchors, 'origin': 'ai_manual_source_reading', 'approved': False,
                      'gold': False, 'reviewer': None, 'split': None, 'runtime_reachability': 'not_established',
                      'safety_verdict': 'unknown', 'independence_approved': False})
    calls = [r for r in references if r['kind'] == 'direct_call_syntax']
    literals = [r for r in calls if r.get('first_argument', {}).get('kind') == 'string_literal']
    identifiers = [r for r in calls if r.get('first_argument', {}).get('kind') == 'identifier']
    summary = {'kind': 'caller_source_syntax_and_ai_context_not_runtime_or_independent_quality',
               'counts': {'source_files': len(payload), 'syntax_references': len(references), 'direct_call_syntax': len(calls),
                          'other_unresolved_references': len(references) - len(calls), 'target_declarations': len(declarations),
                          'caller_functions': len({(r['path'], r['function_name']) for r in calls}),
                          'literal_command_name_calls': len(literals), 'identifier_command_name_calls': len(identifiers),
                          'context_reviews': len(notes), 'source_anchors': sum(len(n['anchors']) for n in notes),
                          'independent_gold_records': 0, 'approved_independent_cases': 0},
               'direct_calls_by_target': dict(Counter(r['target'] for r in calls)),
               'literal_command_values': dict(Counter(r['first_argument']['literal'] for r in literals)),
               'direct_calls_by_function': dict(Counter(r['function_name'] for r in calls)),
               'original_project_call_denominator_unchanged': 68, 'original_project_candidates_unchanged': 2,
               'new_independent_positive_labels': 0, 'new_independent_negative_labels': 0,
               'analyzed_project_code_executed': False, 'context_scanner_kernel_calls': 0, 'model_calls': 0, 'mcp_calls': 0,
               'nodes_started': 0, 'formal_runs': 0, 'precision': None, 'recall': None,
               'runtime_reachability': 'not_established', 'quality_status': 'not_scored_independent_gold_and_positive_sufficiency_unestablished',
               'limitations': ['two source files selected by a pinned textual search, not an exhaustive resolved repository call graph',
                               'import spelling and literals do not establish binding, trusted PATH, executable content or safe arguments',
                               'function values, dot imports, shadowing, dynamic dispatch and external callers remain unverified',
                               'CI comments/build constraints and source dispatch are not observed production reachability',
                               'AI contract notes and caller multiplicity do not create independent defect cases or gold',
                               'existing 68 project calls / 2 candidates remain a separate denominator from caller references']}
    return {'coverage-summary.json': summary, 'caller-review.json': references,
            'context-anchors.json': {'declarations': declarations, 'functions': contexts, 'build_constraints': tags},
            'private-ai-context.json': {'kind': 'private_unapproved_command_contract_drafts', 'reviews': notes}}


def prepare(output, goroot, lint):
    if output.exists():
        raise ValueError('fresh output required')
    revision, proof, payload = source_data()
    packet, work = output / 'packet', output / 'work'
    (packet / 'logs').mkdir(parents=True)
    work.mkdir()
    for source, name in [(HELPER, 'main.go'), (CONTROLS, 'main_test.go')]:
        (work / name).write_bytes((ROOT / source).read_bytes())
        (packet / (name + '.txt')).write_bytes((ROOT / source).read_bytes())
    (work / 'go.mod').write_text('module rulecallercontext\n\ngo 1.26.9\n')
    (packet / 'go.mod.txt').write_bytes((work / 'go.mod').read_bytes())
    env = old.environment(goroot)
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit Go 1.26.9 required')
    checks = []
    commands = {'build': [str(go), 'build', '-o', str(work / 'observe'), '.'],
                'fmt': [str(goroot / 'bin/gofmt'), '-l', 'main.go', 'main_test.go'],
                'vet': [str(go), 'vet', './...'], 'lint': [str(lint), 'run', '--no-config', '--timeout=60s']}
    for kind, cmd in commands.items():
        p = subprocess.run(cmd, cwd=work, env=env, capture_output=True, timeout=90)
        for stream in ['stdout', 'stderr']:
            (packet / f'logs/{kind}.{stream}.txt').write_bytes(getattr(p, stream))
        if p.returncode or (kind == 'fmt' and p.stdout.strip()):
            raise ValueError('caller observer quality failed: ' + kind + '; logs retained')
        checks.append({'kind': kind, 'exit_code': 0})
        print(kind + ': passed', flush=True)
    runs = []
    for mode in ['normal', 'race']:
        cmd = [str(go), 'test', '-json', '-count=1', '-timeout=30s'] + (['-race'] if mode == 'race' else []) + ['./...']
        p = subprocess.run(cmd, cwd=work, env=env, capture_output=True, timeout=90)
        (packet / f'logs/{mode}.jsonl').write_bytes(p.stdout)
        (packet / f'logs/{mode}.stderr.txt').write_bytes(p.stderr)
        if p.returncode:
            raise ValueError('caller syntax controls failed: ' + mode)
        runs.append({'mode': mode, 'exit_code': 0, 'syntax_controls': 9})
        print(mode + ': 9 syntax controls passed', flush=True)
    dump(packet / 'source-inputs.json', payload)
    outputs = []
    for index in range(2):
        p = subprocess.run([str(work / 'observe')], input=json.dumps(payload).encode(), env=env, capture_output=True, timeout=30)
        if index == 0:
            (packet / 'logs/source-observations.json').write_bytes(p.stdout)
            (packet / 'logs/source-observations.stderr.txt').write_bytes(p.stderr)
        if p.returncode or p.stderr:
            raise ValueError('caller source observation failed')
        outputs.append(json.loads(p.stdout))
    if outputs[0] != outputs[1]:
        raise ValueError('nondeterministic caller source observations')
    for name, value in derive(outputs[0]).items():
        dump(packet / name, value)
    dump(packet / 'source-proof-reference.json', {'manifest': str(PRIOR / 'manifest.json'), 'manifest_sha256': sha((ROOT / PRIOR / 'manifest.json').read_bytes()),
         'git_proof': str(PRIOR / 'git-proof.json'), 'git_proof_sha256': sha((ROOT / PRIOR / 'git-proof.json').read_bytes()),
         'repository_commit': revision, 'selected_files': {p: proof['files'][p] for p in PATHS}, 'scope': 'reuse_immutable_commit_to_source_proof'})
    dump(packet / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(packet / 'validation.json', {'kind': 'caller_syntax_and_development_controls_not_project_execution',
         'go_version': version, 'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(),
         'quality_checks': checks, 'runs': runs, 'deterministic_source_repetition': True,
         'target_source_parsed_not_executed': True, 'project_packages_built': 0, 'context_scanner_kernel_calls': 0,
         'model_calls': 0, 'mcp_calls': 0, 'nodes_started': 0, 'formal_runs': 0})
    (packet / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    (packet / 'curation.json').write_bytes((ROOT / CURATION).read_bytes())
    dump(packet / 'manifest.json', {'schema_version': 1, 'kind': 'rule_caller_context_preparation_only', 'repository_commit': revision,
         'input_bindings': bindings(), 'counts': derive(outputs[0])['coverage-summary.json']['counts'],
         'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}})
    print('caller context packet ready: ' + str(packet), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['output', 'goroot', 'lint']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    prepare(args.output.resolve(), args.goroot.resolve(), args.lint.resolve())
