#!/usr/bin/env python3
"""Replay all selected source observations and reject scope/acceptance overclaims."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


prepare = load('prepare-sample-locations.py')
sample_guard = load('check-function-sample.py')
old_guard = load('check-dependency-review.py')
replay = load('check-lexical-coverage.py').replay


def check(packet, check_references=True):
    errors = []

    def require(value, message):
        if not value:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'selected_function_location_preparation_only', 'packet kind')
        expected_bindings = {prepare.POOL, prepare.HELPER, prepare.CONTROLS, prepare.ENGINE,
                             Path('scripts/prepare-sample-locations.py')}
        expected_bindings.update(p / 'manifest.json' for p in [prepare.SAMPLE, prepare.LEXICAL, prepare.DEPENDENCY])
        require(set(manifest['input_bindings']) == {str(p) for p in expected_bindings}, 'input binding inventory')
        for name, digest in manifest['input_bindings'].items():
            require(sha(safe(ROOT, name).read_bytes()) == digest, 'input binding: ' + name)
        for source, artifact in [(prepare.HELPER, 'main.go.txt'), (prepare.CONTROLS, 'main_test.go.txt'),
                                 (Path('scripts/prepare-sample-locations.py'), 'generator.py.txt')]:
            require((ROOT / source).read_bytes() == (packet / artifact).read_bytes(), 'compiled/generator input: ' + str(source))
        require((packet / 'go.mod.txt').read_bytes() == b'module samplelocations\n\ngo 1.26.9\n', 'isolated module identity')
        errors.extend(sample_guard.check(ROOT / prepare.SAMPLE))
        errors.extend(old_guard.check(ROOT / prepare.DEPENDENCY))
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        expected_artifacts = {'main.go.txt', 'main_test.go.txt', 'go.mod.txt', 'generator.py.txt',
                              'observation-inputs.json', 'coverage-summary.json', 'scope-audit.json',
                              'function-checklist.json', 'formal-state.json', 'validation.json',
                              'logs/source-observations.json', 'logs/source-observations.stderr.txt'}
        expected_artifacts.update(f'logs/{kind}.{stream}.txt' for kind in ['build', 'fmt', 'vet', 'lint'] for stream in ['stdout', 'stderr'])
        expected_artifacts.update(f'logs/{mode}.{suffix}' for mode in ['normal', 'race'] for suffix in ['jsonl', 'stderr.txt'])
        require(inventory == expected_artifacts, 'unexpected source observation artifact')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        payload = prepare.source_inputs()
        require(json.loads((packet / 'observation-inputs.json').read_text()) == payload, 'source observer inputs')
        if errors:
            return errors
        observations = json.loads((packet / 'logs/source-observations.json').read_text())
        require(observations == replay(json.dumps(payload), (ROOT / prepare.HELPER).read_bytes()), 'actual selected-source replay mismatch')
        require(not (packet / 'logs/source-observations.stderr.txt').read_text().strip(), 'source observation execution diagnostics')
        require(len(observations) == 25 and len({o['path'] for o in observations}) == 25
                and all(o['typecheck_scope'] == 'single_file_no_importer_incomplete_project_typecheck' and o['project_typecheck_passed'] is False
                        and o['partial_type_error_count'] > 0 and 0 < len(o['partial_type_error_examples']) <= 12 for o in observations), 'partial source typecheck boundary')
        expected = prepare.summarize(observations)
        for name, value in expected.items():
            require(json.loads((packet / name).read_text()) == value, 'scope/coverage accounting: ' + name)
        summary = expected['coverage-summary.json']
        require(manifest['counts'] == summary['counts'], 'manifest counts')
        require(summary['counts']['functions'] == 30 and summary['counts']['new_function_captures'] == 26
                and summary['counts']['unchanged_existing_function_captures'] == 4, 'selected capture counts')
        validation = json.loads((packet / 'validation.json').read_text())
        require(validation['kind'] == 'source_observation_not_project_or_backend_execution'
                and validation['full_project_typecheck_passed'] is False, 'validation scope')
        require(validation['go_version'].startswith('go version go1.26.9 ') and bool(validation['lint_version']), 'native tool identity')
        require(all(validation[k] == 0 for k in ['project_packages_built', 'dependency_backend_calls', 'model_calls', 'nodes_started', 'formal_runs']), 'execution boundary')
        require(validation['quality_checks'] == [{'kind': k, 'exit_code': 0} for k in ['build', 'fmt', 'vet', 'lint']], 'quality check accounting')
        for kind in ['build', 'fmt', 'vet', 'lint']:
            require(not (packet / f'logs/{kind}.stderr.txt').read_text().strip(), 'quality diagnostics: ' + kind)
        require(not (packet / 'logs/fmt.stdout.txt').read_text().strip(), 'unformatted observer source')
        require(validation['runs'] == [{'mode': mode, 'exit_code': 0, 'control_observations': 14} for mode in ['normal', 'race']], 'native control accounting')
        for mode in ['normal', 'race']:
            events = [json.loads(line) for line in (packet / f'logs/{mode}.jsonl').read_text().splitlines() if line]
            passed = [e.get('Test', '') for e in events if e['Action'] == 'pass']
            expected_tests = {'TestDependencyReviewLocations/' + n for n in old_guard.CONTROL_NAMES if n != 'partial_typecheck'} | {'TestDependencyReviewLocations', 'TestPartialSourceTypecheckIsExplicit', ''}
            require(len(passed) == 16 and set(passed) == expected_tests and not any(e['Action'] == 'fail' for e in events), 'raw control completion')
            lines = ''.join(e.get('Output', '') for e in events).splitlines()
            controls = [line.split('DEPENDENCY_LOCATION_CONTROL ', 1)[1] for line in lines if 'DEPENDENCY_LOCATION_CONTROL ' in line]
            require(len(controls) == 14 and set(controls) == {n + ' pass=true' for n in old_guard.CONTROL_NAMES}, 'raw control observations')
            require(not (packet / f'logs/{mode}.stderr.txt').read_text().strip(), 'native control stderr')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == prepare.load('prepare-source-review-supplement.py').formal_state(ROOT)
                and formal['freeze'] is None and formal['execution_allowed'] is False, 'formal state promotion')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('selected_location_preparation') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'thirty_function_locations_not_semantic_gold_or_acceptance'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid selected source observations: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: selected 30/30 source observations, 26 new captures; scope exclusions retained, gold/acceptance unchanged')
