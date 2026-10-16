#!/usr/bin/env python3
"""Replay native observations and reject false source, safety or acceptance claims."""
import argparse
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha
import importlib.util

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('rule_runner', ROOT / 'scripts/run-rule-observations.py')
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


def check(packet, check_references=True):
    errors = []

    def require(value, message):
        if not value:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['schema_version'] == 1 and manifest['kind'] == 'controlled_rule_observation_preparation_only', 'packet identity')
        require(manifest['input_bindings'] == runner.bindings(), 'live analyzer/input binding')
        require(manifest['counts'] == {'sources': 40, 'modes': 2, 'source_observations': 80, 'control_observations': 28, 'quality_checks': 4}, 'observation counts')
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        expected_artifacts = {'coverage-summary.json', 'case-review.json', 'disagreements.json', 'observations.json', 'controls.json',
                              'formal-state.json', 'validation.json', 'template.go.txt', 'overlay-base.go.txt', 'overlay-backing.go.txt',
                              'overlay.json.txt', 'runner.py.txt', 'public-bundle/public-inputs.json'}
        expected_artifacts.update(f'logs/{k}.{s}.txt' for k in ['build', 'fmt', 'vet', 'lint'] for s in ['stdout', 'stderr'])
        expected_artifacts.update(f'logs/{m}.{suffix}' for m in ['normal', 'race', 'regression-normal', 'regression-race'] for suffix in ['jsonl', 'stderr.txt'])
        require(inventory == set(manifest['artifacts']) == expected_artifacts, 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        require((packet / 'runner.py.txt').read_bytes() == (ROOT / 'scripts/run-rule-observations.py').read_bytes(), 'runner snapshot')
        require((packet / 'template.go.txt').read_bytes() == (ROOT / runner.TEMPLATE).read_bytes()
                and (packet / 'overlay-base.go.txt').read_bytes() == (ROOT / runner.VIRTUAL).read_bytes()
                and (packet / 'overlay-backing.go.txt').read_bytes() == runner.overlay_source(), 'overlay changes trusted template or existing tests')
        overlay = json.loads((packet / 'overlay.json.txt').read_text())
        require(set(overlay) == {'Replace'} and set(overlay['Replace']) == {str(ROOT / runner.VIRTUAL)}, 'overlay scope')
        require({p.relative_to(packet / 'public-bundle').as_posix() for p in (packet / 'public-bundle').rglob('*') if p.is_file()} == {'public-inputs.json'}, 'private data in public bundle')
        public = (ROOT / runner.INPUT / 'public-inputs.json').read_bytes()
        require((packet / 'public-bundle/public-inputs.json').read_bytes() == public, 'public-only input payload')
        errors.extend(runner.load('check-rule-coverage.py').check(ROOT / runner.INPUT))
        if errors:
            return errors
        observations = json.loads((packet / 'observations.json').read_text())
        controls = json.loads((packet / 'controls.json').read_text())
        trusted_observations, trusted_controls = runner.replay(public.decode(), json.dumps(runner.bindings(), sort_keys=True))
        require(observations == trusted_observations, 'actual analyzer replay mismatch')
        require(controls == trusted_controls, 'actual control replay mismatch')
        derived = runner.derive(trusted_observations)
        for name, value in derived.items():
            require(json.loads((packet / name).read_text()) == value, 'derived source/quality boundary: ' + name)
        validation = json.loads((packet / 'validation.json').read_text())
        require(validation['kind'] == 'actual_controlled_analyzer_not_project_mcp_or_independent_acceptance', 'validation scope')
        require(validation['go_version'].startswith('go version go1.26.9 ') and bool(validation['lint_version']), 'recorded native tools')
        require(validation['normalized_normal_race_equal'] is True and validation['existing_tests_preserved'] is True
                and validation['maintained_go_source_unchanged'] is True and validation['analyzed_source_executed'] is False,
                'source execution/modification boundary')
        require(validation['source_analysis_invocations_per_mode'] == 80, 'repeat source call accounting')
        require(validation['public_input_scope'] == 'packaging_separation_not_os_filesystem_isolation', 'input isolation overclaim')
        require(not subprocess.check_output(['git', '-C', str(ROOT), 'diff', validation['go_source_reference_commit'], '--', '*.go']), 'maintained Go differs from tested reference')
        checks = validation['quality_checks']
        require(len(checks) == 4 and [q['kind'] for q in checks] == ['build', 'fmt', 'vet', 'lint'] and all(q['exit_code'] == 0 for q in checks), 'native quality checks')
        backing = Path(overlay['Replace'][str(ROOT / runner.VIRTUAL)])
        ov = backing.parent / 'overlay.json'
        expected_commands = {'build': ['test', '-overlay=' + str(ov), '-c', '-o', str(backing.parent / 'rules.test'), './pkg/graph/valueflow'],
                             'fmt': ['-l', str(backing)], 'vet': ['vet', '-overlay=' + str(ov), './pkg/graph/valueflow'],
                             'lint': ['run', '--timeout=120s', './pkg/graph/valueflow']}
        for q in checks:
            k = q['kind']
            require(q['command'] == expected_commands[k], 'quality command scope: ' + k)
            require(not (packet / f'logs/{k}.stderr.txt').read_bytes(), 'native quality diagnostics: ' + k)
            stdout = (packet / f'logs/{k}.stdout.txt').read_text().strip()
            require(stdout in ('', '0 issues.') if k == 'lint' else not stdout, 'native quality finding: ' + k)
        runs = validation['runs']
        require(len(runs) == 2 and [r['mode'] for r in runs] == ['normal', 'race'], 'native run inventory')
        for r in runs:
            mode = r['mode']
            require(r['exit_code'] == 0 and r['source_observations'] == 40 and r['control_observations'] == 14
                    and r['command'] == runner.command('go', ov, mode)[1:], 'native run accounting')
            require(not (packet / f'logs/{mode}.stderr.txt').read_bytes(), 'native observation stderr')
            rows, checks = runner.read_events((packet / f'logs/{mode}.jsonl').read_bytes())
            require(rows == trusted_observations and checks == trusted_controls, 'raw native observation/control mismatch: ' + mode)
        expected_tests = {'TestRulePackScalarCommandCorpus', 'TestRulePackDiscardedErrorCorpus', 'TestRuleBoundsOptInAndDeterministicSourcePins',
                          'TestRuleQueryStrictInputPinsAndChangingSnapshot', 'TestRuleRangeControlWithoutASTLocationRemainsExplicitUnknown'}
        prior_tests = json.loads((ROOT / runner.INPUT / 'existing-fixtures.json').read_text())
        expected_tests.update(row['test'] + '/' + row['name'] for row in prior_tests)
        regressions = validation['existing_rule_regressions']
        require(len(regressions) == 2 and [r['mode'] for r in regressions] == ['normal', 'race'], 'existing regression inventory')
        for r in regressions:
            mode = r['mode']
            expected_cmd = ['test', '-json', '-count=1', '-run=^TestRule(Pack|Bounds|Query|Range)', '-timeout=60s'] + (['-race'] if mode == 'race' else []) + ['./pkg/graph/valueflow']
            require(r['exit_code'] == 0 and r['command'] == expected_cmd and r['named_tests_passed'] == len(expected_tests), 'existing regression accounting')
            events = [json.loads(line) for line in (packet / f'logs/regression-{mode}.jsonl').read_text().splitlines() if line]
            passes = [e.get('Test', '') for e in events if e['Action'] == 'pass']
            require(not any(e['Action'] in ['fail', 'skip'] for e in events) and set(passes) == expected_tests | {''}
                    and len(passes) == len(expected_tests) + 1 and not (packet / f'logs/regression-{mode}.stderr.txt').read_bytes(), 'raw existing regression completion')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == runner.load('prepare-source-review-supplement.py').formal_state(ROOT)
                and formal['freeze'] is None and formal['execution_allowed'] is False, 'formal state promotion')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('controlled_rule_observations') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'controlled_analyzer_only_not_independent_quality_or_project_effect'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, AttributeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid controlled rule observations: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 80 controlled source observations, 28 controls, replayed source pins; independent quality remains unscored')
