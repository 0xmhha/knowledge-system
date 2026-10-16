#!/usr/bin/env python3
"""Replay private source observations; keep AI dependency proposals unapproved."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import SourceCapture, safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
LEXICAL = BASE / 'lexical-coverage-v1'
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


lexical = load('check-lexical-coverage.py')
prepare = load('prepare-dependency-review.py')
CONTROL_NAMES = {'shadow_identity', 'short_reuse', 'compound_roles', 'parenthesized_write', 'range_shadow',
                 'nested_continue', 'break_switch_inside_loop', 'label_continue', 'named_bare_return',
                 'closure_excluded', 'struct_key_not_variable', 'async_and_panic_syntax', 'shadowed_panic', 'partial_typecheck'}


class CapturedSource:
    span = SourceCapture.span

    def __init__(self, packet, proof):
        self.packet, self.proof = packet, proof

    def source(self, path):
        return safe(self.packet, self.proof['files'][path]['artifact']).read_bytes()


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'dependency_review_source_preparation_only', 'packet kind')
        require(manifest['lexical_manifest'] == str(LEXICAL / 'manifest.json') and manifest['lexical_manifest_sha256'] == sha((ROOT / LEXICAL / 'manifest.json').read_bytes()), 'lexical packet binding')
        errors.extend(lexical.check(ROOT / LEXICAL))
        pool = json.loads((ROOT / POOL).read_text())
        require(manifest['candidate_pool_sha256'] == sha((ROOT / POOL).read_bytes()) and manifest['repository_commit'] == pool['repository_commit'], 'candidate pool/revision')
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        expected_sources = {'scripts/dependency_review_locations.go.txt': 'main.go.txt',
                            'scripts/fixtures/dependency_review_locations_test.go.txt': 'main_test.go.txt',
                            'scripts/fixtures/dependency_review_proposals.json': 'curation.json',
                            'scripts/prepare-dependency-review.py': 'generator.py.txt', 'scripts/source_packet.py': 'source-packet.py.txt'}
        expected_sources.update({str(p): 'engine-source/' + str(p) + '.txt' for p in prepare.ENGINE})
        require(set(manifest['source_bindings']) == set(expected_sources), 'source binding inventory')
        for name, artifact in expected_sources.items():
            raw = safe(ROOT, name).read_bytes()
            require(manifest['source_bindings'][name] == {'sha256': sha(raw), 'artifact': artifact}
                    and safe(packet, artifact).read_bytes() == raw, 'source binding: ' + name)
        require((packet / 'go.mod.txt').read_bytes() == b'module dependencyreview\n\ngo 1.26.9\n', 'isolated module')
        proof = verify(packet, pool['repository_commit'])
        curation = json.loads((packet / 'curation.json').read_text())
        require([p['candidate_id'] for p in curation['contexts']] == ['SRC-002', 'SRC-006', 'SRC-011', 'SRC-036'], 'review function selection')
        sources = {f['candidate_id']: f for f in pool['functions']}
        capture = CapturedSource(packet, proof)
        payload = []
        for row in curation['contexts']:
            f = sources[row['candidate_id']]
            raw = capture.source(f['path'])
            require(sha(raw) == f['file_sha256'] and sha(raw[f['start_byte']:f['end_byte_exclusive']]) == f['function_sha256'], 'candidate function binding')
            payload.append({'Path': f['path'], 'Source': raw.decode(), 'Starts': [f['start_byte']]})
        require(set(proof['files']) == {r['Path'] for r in payload}, 'source file inventory')
        require(json.loads((packet / 'location-inputs.json').read_text()) == payload, 'source observer input mismatch')
        if errors:
            return errors
        observations = json.loads((packet / 'logs/locations.json').read_text())
        require(observations == lexical.replay(json.dumps(payload), (ROOT / 'scripts/dependency_review_locations.go.txt').read_bytes()), 'actual source observation replay mismatch')
        require(len(observations) == 4 and all(o['typecheck_scope'] == 'single_file_no_importer_incomplete_project_typecheck'
                    and o['project_typecheck_passed'] is False and o['partial_type_error_count'] > 0 and 0 < len(o['partial_type_error_examples']) <= 12 for o in observations), 'partial typecheck boundary')
        require(not (packet / 'logs/locations.stderr.txt').read_text().strip(), 'observer execution diagnostics')
        prior = json.loads((ROOT / LEXICAL / 'logs/analyzer.json').read_text())
        require(json.loads((packet / 'private-review.json').read_text()) == prepare.review(pool, observations, prior, curation, capture), 'private review locations/claims mismatch')
        require(json.loads((packet / 'matching-contract.json').read_text()) == prepare.matching_contract(), 'matching proposal/freeze mismatch')
        engine = (ROOT / 'pkg/graph/valueflow/dependence.go').read_text()
        require(all(s in engine for s in ['range_binding_unsupported', 'address_dependence', 'reference_dependence', 'aggregate_dependence', 'unknown_dependence', 'normal_exit_postdominators']), 'engine capability contract')
        validation = json.loads((packet / 'validation.json').read_text())
        require(validation['kind'] == 'source_locations_and_development_controls_not_dependency_backend'
                and validation['project_typecheck_passed'] is False, 'validation scope')
        require(validation['go_version'].startswith('go version go1.26.9 ') and bool(validation['lint_version']), 'tool identity')
        require(validation['partial_source_type_errors'] == {o['path']: o['partial_type_error_count'] for o in observations}, 'partial diagnostics accounting')
        require(all(validation[k] == 0 for k in ['project_packages_built', 'nodes_started', 'model_calls', 'dependency_backend_calls', 'formal_runs']), 'execution boundary')
        require(validation['quality_checks'] == [{'kind': k, 'exit_code': 0} for k in ['build', 'fmt', 'vet', 'lint']], 'quality checks')
        for kind in ['build', 'fmt', 'vet', 'lint']:
            require(not (packet / f'logs/{kind}.stderr.txt').read_text().strip(), 'quality diagnostics: ' + kind)
        require(not (packet / 'logs/fmt.stdout.txt').read_text().strip(), 'unformatted observer')
        require(validation['runs'] == [{'mode': m, 'exit_code': 0, 'control_observations': 14} for m in ['normal', 'race']], 'control run counts')
        for mode in ['normal', 'race']:
            events = [json.loads(line) for line in (packet / f'logs/{mode}.jsonl').read_text().splitlines() if line]
            passed = [e.get('Test', '') for e in events if e['Action'] == 'pass']
            expected = {'TestDependencyReviewLocations/' + n for n in CONTROL_NAMES if n != 'partial_typecheck'} | {'TestDependencyReviewLocations', 'TestPartialSourceTypecheckIsExplicit', ''}
            require(len(passed) == 16 and set(passed) == expected and not any(e['Action'] == 'fail' for e in events), 'raw control completion')
            lines = ''.join(e.get('Output', '') for e in events).splitlines()
            rows = [line.split('DEPENDENCY_LOCATION_CONTROL ', 1)[1] for line in lines if 'DEPENDENCY_LOCATION_CONTROL ' in line]
            require(len(rows) == 14 and set(rows) == {n + ' pass=true' for n in CONTROL_NAMES}, 'raw control observations')
            require(not (packet / f'logs/{mode}.stderr.txt').read_text().strip(), 'control execution stderr')
        require(manifest['counts'] == {'functions': 4, 'data_proposals': 19, 'exclusion_proposals': 7,
                'exit_ownership_markers': 14, 'context_anchors': 8, 'development_controls': 14,
                'control_observations': 28, 'quality_checks': 4, 'independent_gold_records': 0}, 'manifest counts')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == prepare.load('prepare-source-review-supplement.py').formal_state(ROOT) and formal['freeze'] is None
                and formal['execution_allowed'] is False, 'formal state promotion')
        diagnostics = json.loads((packet / 'diagnostics.json').read_text())
        require(diagnostics == [{'kind': 'development_fixture_lint_style_failure', 'finding': 'QF1003 tagged switch suggestion',
                'project_execution': False, 'backend_calls': 0, 'model_calls': 0}], 'prior diagnostic scope')
        require('QF1003' in (packet / 'prior-quality-diagnostic/logs/lint.stdout.txt').read_text(), 'prior lint finding lost')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('dependency_source_review') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'private_source_proposals_not_independent_edge_gold'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid dependency source review: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 4 source reviews, 19 data proposals, 7 exclusions, 14 exit owners; independent gold 0')
