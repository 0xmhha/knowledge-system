#!/usr/bin/env python3
"""Audit local-variable scopes in the existing source pool, without edge gold."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess

from source_packet import SourceCapture, sha

ROOT = Path(__file__).resolve().parents[1]
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
TEMPLATE = Path('scripts/lexical_coverage.go.txt')
CONTROLS = Path('scripts/fixtures/lexical_coverage_test.go.txt')


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def summarize(pool, rows):
    by_site = {(r['path'], r['site']['start_byte']): r for r in rows}
    if len(rows) != 40 or len(by_site) != 40:
        raise ValueError('expected the original 40 distinct function candidates')
    enriched = []
    for source in pool['functions']:
        row = by_site[(source['path'], source['start_byte'])]
        if row['function_sha256'] != source['function_sha256'] or row['site']['end_byte_exclusive'] != source['end_byte_exclusive'] or row['name'] != source['name'] or row['receiver'] != source['receiver']:
            raise ValueError('function identity mismatch')
        f = row['features']
        enriched.append({'candidate_id': source['candidate_id'], 'module': source['module'],
                         'scope': 'development_stress_source_only' if source['path'].startswith('miner/stress/') else 'production_source_candidate',
                         'source': source, 'features': f,
                         'declaration_and_reassignment': bool(f.get('body_variable_declaration') and f.get('local_variable_reassignment')),
                         'old_name_hint': source['syntax_features'].get('repeated_short_declaration_name_hint', False),
                         'own_body_shadow_events': sum(e['kind'] == 'local_variable_shadow' for e in row['events']),
                         'excluded_closures': len(row['excluded_closures']),
                         'origin': 'offline_local_scope_analyzer_not_independent_gold',
                         'independence_approved': False, 'approved': False, 'reviewer': None,
                         'split': None, 'exhaustive_data_edges': None, 'exhaustive_control_edges': None,
                         'quality_scoring_allowed': False, 'final_eligibility': 'not_established'})
    count = lambda key: sum(bool(r['features'].get(key)) for r in enriched)
    counts = {key: count(key) for key in ['body_variable_declaration', 'local_variable_reassignment',
              'same_scope_short_reuse', 'branch', 'loop', 'local_variable_shadow', 'return']}
    counts['declaration_and_reassignment'] = sum(r['declaration_and_reassignment'] for r in enriched)
    production = [r for r in enriched if r['scope'] == 'production_source_candidate']
    production_counts = {key: sum(bool(r['features'].get(key)) for r in production) for key in counts if key != 'declaration_and_reassignment'}
    production_counts['declaration_and_reassignment'] = sum(r['declaration_and_reassignment'] for r in production)
    return {'kind': 'lexical_coverage_preparation_not_dependency_gold', 'functions': enriched,
            'candidate_functions': len(enriched), 'candidate_modules': len({r['module'] for r in enriched}),
            'production_candidates': len(production), 'development_stress_source_only': len(enriched) - len(production),
            'feature_function_counts': counts,
            'production_feature_function_counts': production_counts,
            'old_repeated_name_hints': sum(r['old_name_hint'] for r in enriched),
            'hint_without_confirmed_local_shadow': [r['candidate_id'] for r in enriched if r['old_name_hint'] and not r['features'].get('local_variable_shadow')],
            'shadow_without_old_hint': [r['candidate_id'] for r in enriched if not r['old_name_hint'] and r['features'].get('local_variable_shadow')],
            'shadow_events': sum(r['own_body_shadow_events'] for r in enriched),
            'excluded_closures': sum(r['excluded_closures'] for r in enriched),
            'requirements_check': {'independent_functions_min': 30, 'approved_independent_functions': 0,
                'functions_per_construct_min': 4, 'syntax_coverage_proposal_only': {
                    'declaration_redefinition': production_counts['declaration_and_reassignment'] >= 4,
                    'branch': production_counts['branch'] >= 4, 'loop': production_counts['loop'] >= 4,
                    'shadow': production_counts['local_variable_shadow'] >= 4, 'return': production_counts['return'] >= 4},
                'formal_sufficiency': 'not_reviewed'},
            'independent_gold_records': 0, 'formal_runs': 0, 'dependency_backend_calls': 0,
            'data_edge_precision': None, 'data_edge_recall': None,
            'control_edge_precision': None, 'control_edge_recall': None,
            'semantics': {
                'shadow': 'a new variable whose nearest visible outer local binding is a distinct variable',
                'same_scope_short_reuse': 'existing local name on := with another new nonblank variable',
                'reassignment': 'write to a known local identifier by =, compound op, increment/decrement, range =, or short reuse',
                'declaration_redefinition_proposal': 'body variable declaration AND known local reassignment in the same function',
                'branch': 'if, expression switch, type switch, select in the own function body',
                'scope': 'own function body, receiver/parameters/results; nested anonymous function bodies excluded',
                'type_switch_guard': 'one implicit binding per case; repeated guard source location is not an independent sample',
                'excluded': 'package/import/universe bindings, field/index memory writes, nested closure internals, type resolution and edge gold'},
            'limitations': ['syntactic local binding observations are not independently reviewed dependency edges',
                           'go/types cross-check covers standalone development controls, not the complete project',
                           'existing exposed candidate pool; semantic independence and FINAL eligibility remain unreviewed',
                           'absence in this restricted census does not establish absence of other shadow mechanisms',
                           'read/write order, branch reachability, aliasing and may-reach edge sets are not derived']}


def prepare(repo, output, goroot, lint):
    if output.exists():
        raise ValueError('use a new output directory; preserve prior evidence')
    packet, work = output / 'packet', output / 'work'
    (packet / 'logs').mkdir(parents=True)
    work.mkdir()
    env = dict(os.environ, GOROOT=str(goroot), PATH=str(goroot / 'bin') + os.pathsep + os.environ.get('PATH', ''),
               GOTOOLCHAIN='local', GOENV='off', GOWORK='off', GOFLAGS='', GOPROXY='off', GOSUMDB='off',
               GOTELEMETRY='off', GOCACHE='/private/tmp/ks-corpus-go-cache',
               GOLANGCI_LINT_CACHE='/private/tmp/ks-collection-lint-cache')
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit Go 1.26.9 required')
    for src, dst in [(TEMPLATE, 'main.go'), (CONTROLS, 'main_test.go')]:
        raw = (ROOT / src).read_bytes()
        (work / dst).write_bytes(raw)
        (packet / (dst + '.txt')).write_bytes(raw)
    (work / 'go.mod').write_text('module lexicalcensus\n\ngo 1.26.9\n')
    (packet / 'go.mod.txt').write_bytes((work / 'go.mod').read_bytes())
    commands = {'build': [str(go), 'build', '-o', str(work / 'census'), '.'],
                'fmt': [str(goroot / 'bin/gofmt'), '-l', 'main.go', 'main_test.go'],
                'vet': [str(go), 'vet', './...'],
                'lint': [str(lint), 'run', '--no-config', '--timeout=60s']}
    checks, runs = [], []
    for kind, command in commands.items():
        result = subprocess.run(command, cwd=work, env=env, capture_output=True, timeout=90)
        for stream in ['stdout', 'stderr']:
            (packet / 'logs' / (kind + '.' + stream + '.txt')).write_bytes(getattr(result, stream))
        if result.returncode or (kind == 'fmt' and result.stdout.strip()):
            raise ValueError('quality check failed: ' + kind)
        checks.append({'kind': kind, 'exit_code': 0})
        print(kind + ': pass', flush=True)
    for mode in ['normal', 'race']:
        command = [str(go), 'test', '-json', '-count=1', '-timeout=30s'] + (['-race'] if mode == 'race' else []) + ['./...']
        result = subprocess.run(command, cwd=work, env=env, capture_output=True, timeout=90)
        (packet / 'logs' / (mode + '.jsonl')).write_bytes(result.stdout)
        (packet / 'logs' / (mode + '.stderr.txt')).write_bytes(result.stderr)
        if result.returncode:
            raise ValueError('development controls failed: ' + mode)
        events = [json.loads(line) for line in result.stdout.splitlines() if line]
        passed = [e['Test'] for e in events if e['Action'] == 'pass' and e.get('Test', '').startswith('TestLexicalCoverage/')]
        runs.append({'mode': mode, 'exit_code': 0, 'passed_controls': passed})
        print(mode + ': ' + str(len(passed)) + ' development controls passed', flush=True)
    pool = json.loads((ROOT / POOL).read_bytes())
    capture = SourceCapture(repo, packet, pool['repository_commit'])
    grouped = {}
    for f in pool['functions']:
        raw = capture.source(f['path'])
        if sha(raw) != f['file_sha256'] or sha(raw[f['start_byte']:f['end_byte_exclusive']]) != f['function_sha256']:
            raise ValueError('candidate source identity mismatch')
        grouped.setdefault(f['path'], {'Path': f['path'], 'Source': raw.decode(), 'Starts': []})['Starts'].append(f['start_byte'])
    payload = list(grouped.values())
    dump(packet / 'analyzer-inputs.json', payload)
    result = subprocess.run([str(work / 'census')], input=json.dumps(payload).encode(), capture_output=True, timeout=30)
    (packet / 'logs/analyzer.json').write_bytes(result.stdout)
    (packet / 'logs/analyzer.stderr.txt').write_bytes(result.stderr)
    if result.returncode:
        raise ValueError('source census failed; raw diagnostics preserved')
    rows = json.loads(result.stdout)
    summary = summarize(pool, rows)
    dump(packet / 'coverage.json', summary)
    dump(packet / 'git-proof.json', capture.proof())
    dump(packet / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(packet / 'validation.json', {'kind': 'standalone_control_validation_not_project_typecheck',
         'go_version': version, 'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(),
         'quality_checks': checks, 'runs': runs, 'source_census_runs': 1,
         'project_packages_compiled': 0, 'knowledge_system_backend_calls': 0, 'nodes_started': 0,
         'model_calls': 0, 'formal_runs': 0})
    for src, dst in [(Path(__file__).relative_to(ROOT), 'generator.py.txt'), (Path('scripts/source_packet.py'), 'source-packet.py.txt')]:
        (packet / dst).write_bytes((ROOT / src).read_bytes())
    dump(packet / 'manifest.json', {'schema_version': 1, 'kind': 'local_lexical_coverage_preparation_only',
         'repository_commit': pool['repository_commit'], 'candidate_pool': str(POOL),
         'candidate_pool_sha256': sha((ROOT / POOL).read_bytes()),
         'source_bindings': {str(p): sha((ROOT / p).read_bytes()) for p in [TEMPLATE, CONTROLS, Path(__file__).relative_to(ROOT), Path('scripts/source_packet.py')]},
         'counts': {'functions': len(rows), 'source_files': len(capture.files),
                    'control_cases': len(runs[0]['passed_controls']), 'control_observations': sum(len(r['passed_controls']) for r in runs),
                    'quality_checks': len(checks), 'independent_gold_records': 0},
         'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}})
    print(json.dumps(summary['feature_function_counts']), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--goroot', type=Path, required=True)
    parser.add_argument('--lint', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.repo, args.output.resolve(), args.goroot.resolve(), args.lint.resolve())
