#!/usr/bin/env python3
"""Observe the thirty-function sample and quarantine unsupported scope matches."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess

from source_packet import safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
SAMPLE = BASE / 'function-sample-v1'
LEXICAL = BASE / 'lexical-coverage-v1'
DEPENDENCY = BASE / 'dependency-review-v1'
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')
HELPER = Path('scripts/dependency_review_locations.go.txt')
CONTROLS = Path('scripts/fixtures/dependency_review_locations_test.go.txt')
ENGINE = Path('pkg/graph/valueflow/dependence.go')


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def source_inputs():
    summary = json.loads((ROOT / SAMPLE / 'sample-summary.json').read_text())
    pool = json.loads((ROOT / POOL).read_text())
    proof = verify(ROOT / LEXICAL, pool['repository_commit'])
    grouped = {}
    for source in pool['functions']:
        if source['candidate_id'] not in summary['selected_proposal']:
            continue
        raw = safe(ROOT / LEXICAL, proof['files'][source['path']]['artifact']).read_bytes()
        if sha(raw) != source['file_sha256'] or sha(raw[source['start_byte']:source['end_byte_exclusive']]) != source['function_sha256']:
            raise ValueError('selected function identity mismatch')
        grouped.setdefault(source['path'], {'Path': source['path'], 'Source': raw.decode(), 'Starts': []})['Starts'].append(source['start_byte'])
    return list(grouped.values())


def classify_occurrence(o, prior, body_start):
    possible = [b for b in prior['bindings'] if b['kind'] == 'var' and b['name'] == o['name']
                and b['site']['start_byte'] == o['declaration']['start_byte']]
    if len(possible) == 1:
        status = 'corroborated_own_function_variable'
    elif len(possible) > 1:
        status = 'ambiguous_lexical_scope_not_collapsed'
    elif body_start is not None and o['role'] == 'definition' and o['site']['start_byte'] < body_start:
        status = 'quarantined_non_body_signature_scope'
    else:
        status = 'unmatched_scope_not_assumed_local'
    return {'occurrence': o, 'status': status, 'possible_lexical_bindings': possible,
            'approved_local_binding': False, 'value_kind': None, 'gold': False}


def summarize(observations):
    sample = json.loads((ROOT / SAMPLE / 'sample-summary.json').read_text())
    pool = {r['candidate_id']: r for r in json.loads((ROOT / POOL).read_text())['functions']}
    prior = {(r['path'], r['site']['start_byte']): r for r in json.loads((ROOT / LEXICAL / 'logs/analyzer.json').read_text())}
    old = {(f['path'], f['site']['start_byte']): f for row in json.loads((ROOT / DEPENDENCY / 'logs/locations.json').read_text()) for f in row['functions']}
    by_site = {(f['path'], f['site']['start_byte']): f for row in observations for f in row['functions']}
    diagnostics = {row['path']: {'count': row['partial_type_error_count'], 'examples': row['partial_type_error_examples']} for row in observations}
    if len(by_site) != 30 or sum(len(row['functions']) for row in observations) != 30:
        raise ValueError('exactly thirty distinct selected functions required')
    functions, checklist = [], []
    for cid in sorted(sample['selected_proposal']):
        source = pool[cid]
        key = source['path'], source['start_byte']
        row, lex = by_site[key], prior[key]
        if row['function_sha256'] != source['function_sha256'] or row['name'] != source['name'] or row['site']['end_byte_exclusive'] != source['end_byte_exclusive']:
            raise ValueError('observed function identity mismatch')
        scopes = {int(b['scope'].split('@', 1)[1]) for b in lex['bindings'] if b['scope'].startswith('function@')}
        body_start = next(iter(scopes)) if len(scopes) == 1 else None
        audits = [classify_occurrence(o, lex, body_start) for o in row['occurrences']]
        matched = [a for a in audits if a['status'] == 'corroborated_own_function_variable']
        seen = {a['possible_lexical_bindings'][0]['id'] for a in matched}
        unseen = [b for b in lex['bindings'] if b['kind'] == 'var' and b['id'] not in seen]
        status_counts = {key: sum(a['status'] == key for a in audits) for key in [
            'corroborated_own_function_variable', 'quarantined_non_body_signature_scope',
            'ambiguous_lexical_scope_not_collapsed', 'unmatched_scope_not_assumed_local']}
        exits = []
        for marker in row['markers']:
            if marker['kind'] in ('break', 'continue', 'goto', 'fallthrough', 'return'):
                exits.append({'marker': marker, 'owner_status': 'syntactic_owner_recorded' if marker.get('target') else 'owner_not_resolved',
                              'computed_control_dependence': False, 'runtime_reachability': None})
        old_prepared = key in old
        if old_prepared and old[key] != row:
            raise ValueError('old four-function observations changed')
        limits = {'partial_project_typecheck': True, 'full_project_typecheck_passed': False,
                  'file_type_diagnostics': diagnostics[source['path']],
                  'range_bindings': sum(b['kind'] == 'var' and b['origin'] == 'range' for b in lex['bindings']),
                  'range_backend_mapping': 'range_binding_unsupported',
                  'excluded_closures': row['excluded_closures'],
                  'go_statements': sum(m['kind'] == 'go' for m in row['markers']),
                  'defer_statements': sum(m['kind'] == 'defer' for m in row['markers']),
                  'panic_syntax': [m for m in row['markers'] if m['kind'] == 'panic_call_syntax_only'],
                  'scalar_reference_aggregate_kind': 'not_established', 'memory_alias_and_call_effects': 'not_modeled',
                  'reaching_definitions': 'not_computed', 'normal_exit_postdominators': 'not_computed'}
        functions.append({'candidate_id': cid, 'source': source, 'function_body_start_byte': body_start,
                          'occurrence_audit': audits, 'scope_status_counts': status_counts,
                          'unobserved_prior_variable_bindings': unseen, 'syntax_markers': row['markers'],
                          'exit_ownership_audit': exits, 'limitations': limits,
                          'old_four_function_observations_unchanged': True if old_prepared else None,
                          'gold': False, 'reviewer': None, 'independence_approved': False,
                          'exhaustive_data_edges': None, 'exhaustive_control_edges': None, 'scoring_allowed': False})
        checklist.append({'candidate_id': cid, 'source_observations': 'captured_non_exhaustive',
                          'new_source_location_capture': not old_prepared,
                          'private_relation_proposals': 'existing_non_exhaustive_packet' if old_prepared else 'not_authored',
                          'scope_status_counts': status_counts, 'unobserved_prior_variable_bindings': len(unseen),
                          'scalar_reference_mapping': 'not_established', 'independent_review': 'not_assigned',
                          'exhaustive_data_gold': 'not_authored', 'exhaustive_control_gold': 'not_authored',
                          'exact_backend_matching': 'not_frozen', 'split': None, 'formal_execution': 'not_run', 'acceptance': 'not_recorded'})
    sums = {key: sum(f['scope_status_counts'][key] for f in functions) for key in functions[0]['scope_status_counts']}
    summary = {'kind': 'thirty_function_source_observations_not_edge_gold',
               'counts': {'functions': len(functions), 'new_function_captures': sum(c['new_source_location_capture'] for c in checklist),
                          'unchanged_existing_function_captures': sum(c['private_relation_proposals'] != 'not_authored' for c in checklist),
                          'source_files': len(observations), 'raw_occurrences': sum(len(f['occurrence_audit']) for f in functions),
                          **sums, 'syntax_markers': sum(len(f['syntax_markers']) for f in functions),
                          'unobserved_prior_variable_bindings': sum(len(f['unobserved_prior_variable_bindings']) for f in functions),
                          'excluded_closures': sum(len(f['limitations']['excluded_closures']) for f in functions),
                          'functions_with_range_bindings': sum(f['limitations']['range_bindings'] > 0 for f in functions),
                          'approved_independent_functions': 0, 'independent_gold_records': 0},
               'partial_file_type_error_total': sum(v['count'] for v in diagnostics.values()),
               'source_observation_coverage': '30_of_30_selected_not_semantic_or_gold_coverage',
               'data_edges': None, 'control_edges': None, 'quality_scores': None,
               'formal_runs': 0, 'dependency_backend_calls': 0, 'model_calls': 0, 'nodes_started': 0,
               'full_project_typecheck_passed': False,
               'limitations': ['partial file type diagnostics are retained; imported types and full project build are outside scope',
                              'callback/function-type signature variables cannot be promoted to own function body variables',
                              'matching local declaration locations does not compute reaching definitions, data/control edges or value kinds',
                              'ambiguous/unmatched/unobserved bindings remain explicit; no first-match or fabricated occurrence',
                              'range/closure/memory/call/concurrency and postdominator limitations remain separate',
                              'all thirty functions are exposed proposals without independent review, gold or DEV/FINAL freeze']}
    return {'coverage-summary.json': summary, 'scope-audit.json': {'kind': 'private_source_scope_observations_only', 'functions': functions},
            'function-checklist.json': checklist}


def prepare(output, goroot, lint):
    if output.exists():
        raise ValueError('use a new output directory')
    for checker, packet in [('check-function-sample.py', SAMPLE), ('check-dependency-review.py', DEPENDENCY)]:
        errors = load(checker).check(ROOT / packet)
        if errors:
            raise ValueError('\n'.join(errors))
    packet, work = output / 'packet', output / 'work'
    (packet / 'logs').mkdir(parents=True)
    work.mkdir()
    env = dict(os.environ, GOROOT=str(goroot), PATH=str(goroot / 'bin') + os.pathsep + os.environ.get('PATH', ''),
               GOTOOLCHAIN='local', GOENV='off', GOWORK='off', GOFLAGS='', GOPROXY='off', GOSUMDB='off',
               GOTELEMETRY='off', GOCACHE='/private/tmp/ks-corpus-go-cache', GOLANGCI_LINT_CACHE='/private/tmp/ks-collection-lint-cache')
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit Go 1.26.9 required')
    for source, name in [(HELPER, 'main.go'), (CONTROLS, 'main_test.go')]:
        (work / name).write_bytes((ROOT / source).read_bytes())
        (packet / (name + '.txt')).write_bytes((ROOT / source).read_bytes())
    (work / 'go.mod').write_text('module samplelocations\n\ngo 1.26.9\n')
    (packet / 'go.mod.txt').write_bytes((work / 'go.mod').read_bytes())
    checks, runs = [], []
    commands = {'build': [str(go), 'build', '-o', str(work / 'observe'), '.'],
                'fmt': [str(goroot / 'bin/gofmt'), '-l', 'main.go', 'main_test.go'],
                'vet': [str(go), 'vet', './...'], 'lint': [str(lint), 'run', '--no-config', '--timeout=60s']}
    for kind, command in commands.items():
        result = subprocess.run(command, cwd=work, env=env, capture_output=True, timeout=90)
        for stream in ['stdout', 'stderr']:
            (packet / 'logs' / (kind + '.' + stream + '.txt')).write_bytes(getattr(result, stream))
        if result.returncode or (kind == 'fmt' and result.stdout.strip()):
            raise ValueError('observer quality check failed: ' + kind)
        checks.append({'kind': kind, 'exit_code': 0})
        print(kind + ': pass', flush=True)
    for mode in ['normal', 'race']:
        command = [str(go), 'test', '-json', '-count=1', '-timeout=30s'] + (['-race'] if mode == 'race' else []) + ['./...']
        result = subprocess.run(command, cwd=work, env=env, capture_output=True, timeout=90)
        (packet / f'logs/{mode}.jsonl').write_bytes(result.stdout)
        (packet / f'logs/{mode}.stderr.txt').write_bytes(result.stderr)
        if result.returncode:
            raise ValueError('development controls failed: ' + mode)
        events = [json.loads(line) for line in result.stdout.splitlines() if line]
        count = ''.join(e.get('Output', '') for e in events).count('DEPENDENCY_LOCATION_CONTROL ')
        runs.append({'mode': mode, 'exit_code': 0, 'control_observations': count})
        print(mode + ': ' + str(count) + ' development controls passed', flush=True)
    inputs = source_inputs()
    dump(packet / 'observation-inputs.json', inputs)
    result = subprocess.run([str(work / 'observe')], input=json.dumps(inputs).encode(), capture_output=True, timeout=60)
    (packet / 'logs/source-observations.json').write_bytes(result.stdout)
    (packet / 'logs/source-observations.stderr.txt').write_bytes(result.stderr)
    if result.returncode:
        raise ValueError('source observation failed; diagnostics preserved')
    outputs = summarize(json.loads(result.stdout))
    for name, value in outputs.items():
        dump(packet / name, value)
    dump(packet / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(packet / 'validation.json', {'kind': 'source_observation_not_project_or_backend_execution', 'go_version': version,
         'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(), 'quality_checks': checks,
         'runs': runs, 'project_packages_built': 0, 'full_project_typecheck_passed': False,
         'dependency_backend_calls': 0, 'model_calls': 0, 'nodes_started': 0, 'formal_runs': 0})
    (packet / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    bindings = [POOL, HELPER, CONTROLS, ENGINE, Path(__file__).relative_to(ROOT)]
    for name in [SAMPLE, LEXICAL, DEPENDENCY]:
        bindings.append(name / 'manifest.json')
    dump(packet / 'manifest.json', {'schema_version': 1, 'kind': 'selected_function_location_preparation_only',
         'input_bindings': {str(p): sha((ROOT / p).read_bytes()) for p in bindings},
         'counts': outputs['coverage-summary.json']['counts'],
         'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}})
    print(json.dumps(outputs['coverage-summary.json']['counts']), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['output', 'goroot', 'lint']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    prepare(args.output.resolve(), args.goroot.resolve(), args.lint.resolve())
