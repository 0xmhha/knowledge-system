#!/usr/bin/env python3
"""Verify/replay the offline lexical census; do not promote observations to gold."""
import argparse
from functools import lru_cache
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

from source_packet import safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')
CONTROLS = {'sibling_names': (0, 0), 'nested_block': (1, 0), 'mixed_short': (0, 1),
            'parameter_reuse': (0, 1), 'named_result_reuse': (0, 1), 'if_initializer': (1, 0),
            'for_initializer': (1, 0), 'range_define': (1, 0), 'range_assignment': (0, 0),
            'closure_excluded': (0, 0), 'closure_initializer_outer_name': (1, 0), 'switch_case_scopes': (2, 0),
            'type_switch_guards': (2, 0), 'type_case_guard_reuse': (1, 1), 'select_receive': (1, 0),
            'select_case_siblings': (0, 0), 'blank_ignored': (0, 0), 'type_blocks_outer_variable': (0, 0),
            'var_initializer': (1, 0), 'package_variable_excluded': (0, 0), 'receiver_reuse': (0, 1),
            'parenthesized_assignment': (0, 0)}


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


@lru_cache(maxsize=2)
def replay(payload, helper):
    """Execute only the trusted live helper. Target source is parsed, never run.

    The evidence itself uses Go 1.26.9. This deterministic replay can also use
    an installed compatible Go version; it never downloads a toolchain/module.
    """
    go = shutil.which('go')
    if not go:
        raise ValueError('installed Go is required for deterministic census replay')
    env = dict(os.environ, GOTOOLCHAIN='local', GOENV='off', GOWORK='off', GOFLAGS='',
               GOPROXY='off', GOSUMDB='off', GOTELEMETRY='off', GOCACHE='/private/tmp/ks-corpus-go-cache')
    with tempfile.TemporaryDirectory(prefix='ks-lexical-replay-') as name:
        work = Path(name)
        (work / 'main.go').write_bytes(helper)
        (work / 'go.mod').write_text('module lexicalreplay\n\ngo 1.25.13\n')
        build = subprocess.run([go, 'build', '-o', str(work / 'census'), '.'], cwd=work, env=env,
                               capture_output=True, timeout=60)
        if build.returncode:
            raise ValueError('offline census replay build failed: ' + build.stderr.decode()[-1000:])
        result = subprocess.run([str(work / 'census')], input=payload.encode(), capture_output=True, timeout=30)
        if result.returncode:
            raise ValueError('offline census replay failed')
        return json.loads(result.stdout)


def check(packet, check_references=True):
    errors = []

    def require(value, message):
        if not value:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'local_lexical_coverage_preparation_only', 'packet kind')
        pool = json.loads((ROOT / POOL).read_text())
        require(manifest['candidate_pool'] == str(POOL) and manifest['candidate_pool_sha256'] == sha((ROOT / POOL).read_bytes()), 'original pool binding')
        require(manifest['repository_commit'] == pool['repository_commit'], 'source revision')
        actual = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(actual == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        bindings = {'scripts/lexical_coverage.go.txt': 'main.go.txt',
                    'scripts/fixtures/lexical_coverage_test.go.txt': 'main_test.go.txt',
                    'scripts/prepare-lexical-coverage.py': 'generator.py.txt',
                    'scripts/source_packet.py': 'source-packet.py.txt'}
        require(set(manifest['source_bindings']) == set(bindings), 'analyzer binding inventory')
        for source, artifact in bindings.items():
            raw = safe(ROOT, source).read_bytes()
            require(manifest['source_bindings'][source] == sha(raw) and raw == safe(packet, artifact).read_bytes(), 'analyzer source binding: ' + source)
        require((packet / 'go.mod.txt').read_bytes() == b'module lexicalcensus\n\ngo 1.26.9\n', 'isolated module binding')
        proof = verify(packet, pool['repository_commit'])
        require(set(proof['files']) == {r['path'] for r in pool['functions']}, 'source file inventory')
        inputs = {}
        for f in pool['functions']:
            raw = safe(packet, proof['files'][f['path']]['artifact']).read_bytes()
            require(sha(raw) == f['file_sha256'] and sha(raw[f['start_byte']:f['end_byte_exclusive']]) == f['function_sha256'], 'candidate source: ' + f['candidate_id'])
            inputs.setdefault(f['path'], {'Path': f['path'], 'Source': raw.decode(), 'Starts': []})['Starts'].append(f['start_byte'])
        payload = json.loads((packet / 'analyzer-inputs.json').read_text())
        require(payload == list(inputs.values()), 'analyzer input/selection mismatch')
        if errors:
            return errors
        rows = json.loads((packet / 'logs/analyzer.json').read_text())
        require(rows == replay(json.dumps(list(inputs.values())), (ROOT / 'scripts/lexical_coverage.go.txt').read_bytes()), 'actual scope replay mismatch')
        require(not (packet / 'logs/analyzer.stderr.txt').read_text().strip(), 'unexpected analyzer diagnostics')
        summary = json.loads((packet / 'coverage.json').read_text())
        require(summary == load('prepare-lexical-coverage.py').summarize(pool, rows), 'coverage counts/claims mismatch')
        validation = json.loads((packet / 'validation.json').read_text())
        require(validation['kind'] == 'standalone_control_validation_not_project_typecheck', 'validation scope')
        require(validation['go_version'].startswith('go version go1.26.9 ') and bool(validation['lint_version']), 'tool identity')
        require(validation['quality_checks'] == [{'kind': k, 'exit_code': 0} for k in ['build', 'fmt', 'vet', 'lint']], 'quality checks')
        for kind in ['build', 'fmt', 'vet', 'lint']:
            require(not (packet / f'logs/{kind}.stderr.txt').read_text().strip(), 'quality diagnostics: ' + kind)
            if kind == 'fmt':
                require(not (packet / 'logs/fmt.stdout.txt').read_text().strip(), 'unformatted helper')
        expected_tests = {'TestLexicalCoverage/' + name for name in CONTROLS}
        require([r['mode'] for r in validation['runs']] == ['normal', 'race'], 'control modes')
        for run in validation['runs']:
            require(run['exit_code'] == 0 and set(run['passed_controls']) == expected_tests and len(run['passed_controls']) == 22, 'control summary')
            events = [json.loads(line) for line in (packet / ('logs/' + run['mode'] + '.jsonl')).read_text().splitlines() if line]
            passed = [e['Test'] for e in events if e['Action'] == 'pass' and e.get('Test', '').startswith('TestLexicalCoverage/')]
            require(passed == run['passed_controls'], 'control raw pass list')
            require(not any(e['Action'] == 'fail' for e in events) and sum(e['Action'] == 'pass' and e.get('Test') == 'TestLexicalCoverage' for e in events) == 1
                    and sum(e['Action'] == 'pass' and not e.get('Test') for e in events) == 1, 'control raw completion')
            lines = ''.join(e.get('Output', '') for e in events).splitlines()
            observations = [line.split('LEXICAL_CONTROL ', 1)[1] for line in lines if 'LEXICAL_CONTROL ' in line]
            require(len(observations) == 22 and set(observations) == {f'{name} shadows={n} same_scope_reuse={r} go_types_match=true' for name, (n, r) in CONTROLS.items()}, 'go/types control observations')
            require(not (packet / ('logs/' + run['mode'] + '.stderr.txt')).read_text().strip(), 'test stderr')
        require(validation['source_census_runs'] == 1 and all(validation[k] == 0 for k in ['project_packages_compiled', 'knowledge_system_backend_calls', 'nodes_started', 'model_calls', 'formal_runs']), 'execution boundary')
        require(manifest['counts'] == {'functions': 40, 'source_files': 28, 'control_cases': 22, 'control_observations': 44, 'quality_checks': 4, 'independent_gold_records': 0}, 'manifest counts')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == load('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
        require(formal['freeze'] is None and formal['execution_allowed'] is False, 'frozen/execution promotion')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('lexical_coverage_preparation') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'local_scope_census_not_independent_dependency_gold'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid lexical packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    problems = check(args.packet.resolve())
    if problems:
        print('\n'.join(problems))
        raise SystemExit(1)
    print('PASS: 40 source candidates replayed, 44 go/types controls, independent gold and formal acceptance unchanged')
