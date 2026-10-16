#!/usr/bin/env python3
"""Bind private AI dependency-review proposals to source locations, without gold."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess

from source_packet import SourceCapture, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
LEXICAL = BASE / 'lexical-coverage-v1'
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')
HELPER = Path('scripts/dependency_review_locations.go.txt')
TESTS = Path('scripts/fixtures/dependency_review_locations_test.go.txt')
CURATION = Path('scripts/fixtures/dependency_review_proposals.json')
ENGINE = [Path(p) for p in ['pkg/graph/valueflow/cfg.go', 'pkg/graph/valueflow/dependence.go',
                           'pkg/graph/valueflow/query.go', 'internal/system/mcp/valueflow.go']]


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def review(pool, observations, lexical, curation, capture):
    sources = {f['candidate_id']: f for f in pool['functions']}
    funcs = {(f['path'], f['site']['start_byte']): f for obs in observations for f in obs['functions']}
    lex = {(r['path'], r['site']['start_byte']): r for r in lexical}

    def function(cid):
        source = sources[cid]
        if source['path'].startswith('miner/stress/'):
            raise ValueError('stress main is outside this preparation')
        f = funcs[(source['path'], source['start_byte'])]
        if f['function_sha256'] != source['function_sha256'] or f['site']['end_byte_exclusive'] != source['end_byte_exclusive']:
            raise ValueError('function identity mismatch')
        return source, f, lex[(source['path'], source['start_byte'])]

    def resolve(cid, name, ref):
        source, f, prior = function(cid)
        matches = [o for o in f['occurrences'] if o['name'] == name and o['role'] == ref['role'] and o['site']['start_line'] == ref['line']]
        if len(matches) != 1:
            raise ValueError('ambiguous/missing curated occurrence: ' + cid + '/' + name + '/' + str(ref))
        occurrence = matches[0]
        matches = [b for b in prior['bindings'] if b['name'] == name and b['site']['start_byte'] == occurrence['declaration']['start_byte']]
        if len(matches) != 1:
            raise ValueError('binding identity is not corroborated by lexical census')
        return {'occurrence': occurrence, 'lexical_binding': matches[0], 'path': source['path']}

    def marker(cid, ref):
        _, f, _ = function(cid)
        matches = [m for m in f['markers'] if m['kind'] == ref['kind'] and m['site']['start_line'] == ref['line']]
        if len(matches) != 1:
            raise ValueError('ambiguous/missing boundary marker')
        return matches[0]

    def unapproved():
        return {'origin': 'ai_source_review_proposal', 'approved': False, 'reviewer': None,
                'independence_verified': False, 'gold': False, 'split': None,
                'quality_scoring_allowed': False, 'formal_edge_kind': None}

    data, exclusions, boundaries, contexts = [], [], [], []
    for p in curation['data_reviews']:
        use = resolve(p['candidate_id'], p['variable'], p['use'])
        defs = [resolve(p['candidate_id'], p['variable'], ref) for ref in p['definitions']]
        if any(d['lexical_binding']['id'] != use['lexical_binding']['id'] for d in defs):
            raise ValueError('positive binding mismatch')
        if use['occurrence']['role'] not in ('read', 'read_write') or any(d['occurrence']['role'] not in ('definition', 'write', 'read_write') for d in defs):
            raise ValueError('invalid def/read roles')
        data.append({**p, **unapproved(), 'definitions': defs, 'use': use,
                     'complete_definition_set': False, 'value_domain': None,
                     'backend_mapping': 'range_binding_unsupported' if any(d['lexical_binding']['origin'] == 'range' for d in defs) else 'kind_and_exact_node_mapping_pending',
                     'reachability_interpretation': 'structural_may_reach_proposal_not_path_feasibility'})
    for p in curation['exclusion_reviews']:
        definition = resolve(p['candidate_id'], p['variable'], p['definition'])
        r = {**p, **unapproved(), 'definition': definition}
        if 'use' in p:
            use = resolve(p['candidate_id'], p['variable'], p['use'])
            same = definition['lexical_binding']['id'] == use['lexical_binding']['id']
            if p['exclusion_reason_proposal'] == 'different_lexical_binding' and same:
                raise ValueError('expected distinct local bindings')
            r.update(use=use, same_lexical_binding=same)
        else:
            r['target_marker'] = marker(p['candidate_id'], p['target_marker'])
        exclusions.append(r)
    for p in curation['boundaries']:
        m = marker(p['candidate_id'], p['marker'])
        target = m.get('target', {}).get('start_line')
        if target != p['expected_target_start_line']:
            raise ValueError('branch ownership mismatch')
        boundaries.append({**p, **unapproved(), 'marker': m,
                           'meaning': 'syntactic_exit_owner_only_not_computed_control_dependence'})
    for p in curation['contexts']:
        source, _, _ = function(p['candidate_id'])
        anchors = []
        for start, end, text in p['anchors']:
            if not source['start_line'] <= start <= end <= source['end_line']:
                raise ValueError('context outside candidate function')
            anchors.append({**capture.span(source['path'], start, end), 'source_interpretation_draft': text})
        contexts.append({**p, **unapproved(), 'source': source, 'anchors': anchors,
                         'exhaustive_data_gold': None, 'exhaustive_control_gold': None,
                         'final_eligibility': 'not_established'})
    return {'kind': 'private_dependency_source_review_not_independent_gold', 'data_reviews': data,
            'exclusion_reviews': exclusions, 'boundaries': boundaries, 'contexts': contexts,
            'approved_independent_functions': 0, 'independent_gold_records': 0, 'formal_runs': 0,
            'dependency_backend_calls': 0, 'data_precision': None, 'data_recall': None,
            'control_precision': None, 'control_recall': None,
            'limitations': ['four exposed source functions are not thirty independent acceptance functions',
                           'local binding corroboration does not prove reaching definitions or path feasibility',
                           'AI exclusions and exit owners are not computed postdominator/control edges',
                           'partial single-file typecheck diagnostics are preserved; full project typecheck was not run',
                           'reference/address/aggregate/unknown roles and unsupported ranges must stay separate from scalar edges',
                           'manual source anchors and proposed definitions are non-exhaustive and cannot support precision/recall']}


def matching_contract():
    return {'status': 'proposal_not_frozen_or_executed', 'approved': False, 'reviewer': None,
            'matching': ['same project/commit/function source hash and absolute source offsets',
                         'same declaration identity, occurrence location/role and actual CFG node location',
                         'edge kind and certainty kept separate: data/address/reference/aggregate/unknown',
                         'synthetic entry and bare-return occurrences need explicit coordinate rules',
                         'range_binding_unsupported and unmapped facts remain unknown, not automatic negative edges',
                         'control_dependence requires normal-exit postdominators; source nesting/exit owner is not enough'],
            'baseline': None, 'dev_final_allocation': None, 'formal_resource_budget': None,
            'engine_contract_inspection': 'read_only_source_not_backend_execution',
            'gold_manifest': None, 'canonical_matching_frozen': False, 'scoring_allowed': False}


def prepare(repo, output, goroot, lint, prior=None):
    if output.exists():
        raise ValueError('use a new output directory')
    errors = load('check-lexical-coverage.py').check(ROOT / LEXICAL)
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
    for src, dst in [(HELPER, 'main.go'), (TESTS, 'main_test.go')]:
        (work / dst).write_bytes((ROOT / src).read_bytes())
        (packet / (dst + '.txt')).write_bytes((ROOT / src).read_bytes())
    (work / 'go.mod').write_text('module dependencyreview\n\ngo 1.26.9\n')
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
            raise ValueError('quality failed: ' + kind)
        checks.append({'kind': kind, 'exit_code': 0})
        print(kind + ': pass', flush=True)
    for mode in ['normal', 'race']:
        command = [str(go), 'test', '-json', '-count=1', '-timeout=30s'] + (['-race'] if mode == 'race' else []) + ['./...']
        result = subprocess.run(command, cwd=work, env=env, capture_output=True, timeout=90)
        (packet / 'logs' / (mode + '.jsonl')).write_bytes(result.stdout)
        (packet / 'logs' / (mode + '.stderr.txt')).write_bytes(result.stderr)
        if result.returncode:
            raise ValueError('development control failure: ' + mode)
        events = [json.loads(line) for line in result.stdout.splitlines() if line]
        count = ''.join(e.get('Output', '') for e in events).count('DEPENDENCY_LOCATION_CONTROL ')
        runs.append({'mode': mode, 'exit_code': 0, 'control_observations': count})
        print(mode + ': ' + str(count) + ' development controls passed', flush=True)
    pool = json.loads((ROOT / POOL).read_text())
    curation = json.loads((ROOT / CURATION).read_text())
    capture = SourceCapture(repo, packet, pool['repository_commit'])
    by_id = {r['candidate_id']: r for r in pool['functions']}
    payload = []
    for p in curation['contexts']:
        source = by_id[p['candidate_id']]
        raw = capture.source(source['path'])
        if sha(raw) != source['file_sha256']:
            raise ValueError('source mismatch')
        payload.append({'Path': source['path'], 'Source': raw.decode(), 'Starts': [source['start_byte']]})
    dump(packet / 'location-inputs.json', payload)
    result = subprocess.run([str(work / 'observe')], input=json.dumps(payload).encode(), capture_output=True, timeout=30)
    (packet / 'logs/locations.json').write_bytes(result.stdout)
    (packet / 'logs/locations.stderr.txt').write_bytes(result.stderr)
    if result.returncode:
        raise ValueError('source observation failed')
    observations = json.loads(result.stdout)
    lexical = json.loads((ROOT / LEXICAL / 'logs/analyzer.json').read_text())
    private = review(pool, observations, lexical, curation, capture)
    dump(packet / 'private-review.json', private)
    dump(packet / 'matching-contract.json', matching_contract())
    dump(packet / 'git-proof.json', capture.proof())
    dump(packet / 'curation.json', curation)
    dump(packet / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(packet / 'validation.json', {'kind': 'source_locations_and_development_controls_not_dependency_backend',
         'go_version': version, 'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(),
         'quality_checks': checks, 'runs': runs, 'project_typecheck_passed': False,
         'partial_source_type_errors': {o['path']: o['partial_type_error_count'] for o in observations},
         'project_packages_built': 0, 'nodes_started': 0, 'model_calls': 0, 'dependency_backend_calls': 0, 'formal_runs': 0})
    copies = {str(HELPER): 'main.go.txt', str(TESTS): 'main_test.go.txt', str(CURATION): 'curation.json',
              str(Path(__file__).relative_to(ROOT)): 'generator.py.txt', 'scripts/source_packet.py': 'source-packet.py.txt'}
    for source in ENGINE:
        copies[str(source)] = 'engine-source/' + str(source) + '.txt'
    for source, target in copies.items():
        p = packet / target
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_bytes((ROOT / source).read_bytes())
    diagnostics = []
    if prior is not None:
        previous = prior / 'packet'
        for path in sorted(previous.rglob('*')):
            if path.is_file():
                destination = packet / 'prior-quality-diagnostic' / path.relative_to(previous)
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(path, destination)
        diagnostics.append({'kind': 'development_fixture_lint_style_failure', 'finding': 'QF1003 tagged switch suggestion',
                            'project_execution': False, 'backend_calls': 0, 'model_calls': 0})
    dump(packet / 'diagnostics.json', diagnostics)
    dump(packet / 'manifest.json', {'schema_version': 1, 'kind': 'dependency_review_source_preparation_only',
         'repository_commit': pool['repository_commit'], 'candidate_pool_sha256': sha((ROOT / POOL).read_bytes()),
         'lexical_manifest': str(LEXICAL / 'manifest.json'), 'lexical_manifest_sha256': sha((ROOT / LEXICAL / 'manifest.json').read_bytes()),
         'source_bindings': {source: {'sha256': sha((ROOT / source).read_bytes()), 'artifact': target} for source, target in copies.items()},
         'counts': {'functions': len(private['contexts']), 'data_proposals': len(private['data_reviews']),
                    'exclusion_proposals': len(private['exclusion_reviews']), 'exit_ownership_markers': len(private['boundaries']),
                    'context_anchors': sum(len(c['anchors']) for c in private['contexts']), 'development_controls': runs[0]['control_observations'],
                    'control_observations': sum(r['control_observations'] for r in runs), 'quality_checks': len(checks), 'independent_gold_records': 0},
         'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}})
    print('Prepared: 4 private source reviews, 19 data proposals, 7 exclusions, 14 exit owners; gold 0', flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['repo', 'output', 'goroot', 'lint']:
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--prior-failed-output', type=Path)
    args = parser.parse_args()
    prepare(args.repo, args.output.resolve(), args.goroot.resolve(), args.lint.resolve(), args.prior_failed_output)
