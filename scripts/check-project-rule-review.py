#!/usr/bin/env python3
"""Verify commit-bound project observations without creating negative/gold labels."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('project_rule_runner', ROOT / 'scripts/run-project-rule-review.py')
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
prior_guard = runner.load('check-rule-observations.py')


def check(packet, check_references=True):
    errors = []

    def require(value, message):
        if not value:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['schema_version'] == 1 and manifest['kind'] == 'project_rule_source_preparation_only'
                and manifest['repository_commit'] == runner.census()['source_commit'], 'project packet identity')
        require(manifest['input_bindings'] == runner.bindings(), 'live project observer/input binding')
        require(manifest['counts'] == {'functions': 51, 'files': 34, 'original_calls': 68, 'function_observations': 102,
                                      'syntax_control_observations': 10, 'quality_checks': 4}, 'project observation counts')
        descriptor, payload = runner.render_inputs(packet)
        proof = json.loads((packet / 'git-proof.json').read_text())
        expected = {'coverage-summary.json', 'function-review.json', 'call-review.json', 'candidate-review.json',
                    'observations.json', 'syntax-controls.json', 'formal-state.json', 'validation.json',
                    'template.go.txt', 'overlay-base.go.txt', 'overlay-backing.go.txt', 'overlay.json.txt', 'runner.py.txt',
                    'public-bundle/public-inputs.json', 'source-bundle.json', 'git-proof.json'}
        expected.update(f'logs/{kind}.{stream}.txt' for kind in ['build', 'fmt', 'vet', 'lint'] for stream in ['stdout', 'stderr'])
        expected.update(f'logs/{mode}.{suffix}' for mode in ['normal', 'race'] for suffix in ['jsonl', 'stderr.txt'])
        expected.update(proof['objects'].values())
        expected.update(r['artifact'] for r in proof['files'].values())
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']) == expected, 'project artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'project artifact hash: ' + name)
        for source, artifact in [(Path('scripts/run-project-rule-review.py'), 'runner.py.txt'), (runner.TEMPLATE, 'template.go.txt'), (runner.VIRTUAL, 'overlay-base.go.txt')]:
            require((ROOT / source).read_bytes() == (packet / artifact).read_bytes(), 'project trusted source snapshot: ' + artifact)
        require((packet / 'overlay-backing.go.txt').read_bytes() == runner.overlay_source(), 'project overlay changes existing tests')
        overlay = json.loads((packet / 'overlay.json.txt').read_text())
        require(set(overlay) == {'Replace'} and set(overlay['Replace']) == {str(ROOT / runner.VIRTUAL)}, 'project overlay scope')
        require({p.relative_to(packet / 'public-bundle').as_posix() for p in (packet / 'public-bundle').rglob('*') if p.is_file()} == {'public-inputs.json'}, 'private data in project public bundle')
        require(json.loads((packet / 'public-bundle/public-inputs.json').read_text()) == payload, 'project public source payload')
        require(json.loads((packet / 'source-bundle.json').read_text()) == descriptor, 'project synthetic bundle descriptor')
        errors.extend(prior_guard.check(ROOT / runner.BASE / 'rule-observations-v1'))
        if errors:
            return errors
        observations, controls = runner.replay(json.dumps(payload), json.dumps(runner.bindings(), sort_keys=True))
        require(json.loads((packet / 'observations.json').read_text()) == observations, 'actual project analyzer replay mismatch')
        require(json.loads((packet / 'syntax-controls.json').read_text()) == controls, 'actual project syntax control replay mismatch')
        for name, value in runner.derive(packet, observations).items():
            require(json.loads((packet / name).read_text()) == value, 'project derived accounting/boundary: ' + name)
        validation = json.loads((packet / 'validation.json').read_text())
        require(validation['kind'] == 'project_source_local_kernel_not_runtime_mcp_or_independent_acceptance', 'project validation scope')
        require(validation['go_version'].startswith('go version go1.26.9 ') and bool(validation['lint_version']), 'project native tools')
        require(validation['normalized_normal_race_equal'] is True and validation['existing_tests_preserved'] is True
                and validation['maintained_go_source_unchanged'] is True and validation['analyzed_project_code_executed'] is False,
                'project source execution/modification boundary')
        require(validation['source_analysis_invocations_per_mode'] == 102, 'project repeat call accounting')
        require(validation['public_input_scope'] == 'source_packaging_separation_not_os_filesystem_isolation', 'project input isolation overclaim')
        require(not subprocess.check_output(['git', '-C', str(ROOT), 'diff', validation['go_source_reference_commit'], '--', '*.go']), 'project observer Go source drift')
        backing = Path(overlay['Replace'][str(ROOT / runner.VIRTUAL)])
        ov = backing.parent / 'overlay.json'
        expected_commands = {'build': ['test', '-overlay=' + str(ov), '-c', '-o', str(backing.parent / 'project.test'), './pkg/graph/valueflow'],
                             'fmt': ['-l', str(backing)], 'vet': ['vet', '-overlay=' + str(ov), './pkg/graph/valueflow'],
                             'lint': ['run', '--timeout=120s', './pkg/graph/valueflow']}
        quality = validation['quality_checks']
        require(len(quality) == 4 and [r['kind'] for r in quality] == ['build', 'fmt', 'vet', 'lint'], 'project quality inventory')
        for q in quality:
            kind = q['kind']
            require(q['exit_code'] == 0 and q['command'] == expected_commands[kind], 'project quality result/command')
            require(not (packet / f'logs/{kind}.stderr.txt').read_bytes(), 'project quality diagnostics')
            stdout = (packet / f'logs/{kind}.stdout.txt').read_text().strip()
            require(stdout in ('', '0 issues.') if kind == 'lint' else not stdout, 'project quality findings')
        runs = validation['runs']
        require(len(runs) == 2 and [r['mode'] for r in runs] == ['normal', 'race'], 'project run inventory')
        for r in runs:
            mode = r['mode']
            require(r['exit_code'] == 0 and r['command'] == runner.command('go', ov, mode)[1:]
                    and r['function_observations'] == 51 and r['syntax_controls'] == 5, 'project native run accounting')
            require(not (packet / f'logs/{mode}.stderr.txt').read_bytes(), 'project native stderr')
            native_rows, native_controls = runner.read_events((packet / f'logs/{mode}.jsonl').read_bytes(), [r['id'] for r in payload['cases']])
            require(native_rows == observations and native_controls == controls, 'project raw native mismatch: ' + mode)
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == runner.load('prepare-source-review-supplement.py').formal_state(ROOT)
                and formal['freeze'] is None and formal['execution_allowed'] is False, 'project formal state promotion')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('project_rule_source_review') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'project_local_source_observations_not_independent_quality_or_runtime'}, 'project ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, AttributeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid project rule source review: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 51 commit-bound functions / 68 calls replayed; 2 candidates, closure and unknown labels retained')
