#!/usr/bin/env python3
"""Run real request parsing through a temporary Go overlay; no handler/backend calls."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess

from source_packet import sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def overlay_source(original, template):
    prefix = b'package mcp\n'
    if not original.startswith(prefix) or not template.startswith(prefix):
        raise ValueError('unexpected test package')
    body = template.split(b')\n\n', 1)[1]
    imports = b'\nimport (\n\t"encoding/json"\n\t"fmt"\n)\n'
    return prefix + imports + original[len(prefix):] + b'\n' + body


def run(output, goroot, lint, prior_failed_output=None):
    inputs = ROOT / BASE / 'symptom-inputs-v1'
    errors = load('check-symptom-inputs.py').check(inputs)
    if errors:
        raise ValueError('\n'.join(errors))
    if output.exists():
        raise ValueError('use a new output directory')
    packet, work = output / 'packet', output / 'work'
    (packet / 'logs').mkdir(parents=True)
    work.mkdir()
    public_raw = (inputs / 'public-inputs.json').read_bytes()
    wire = load('export-symptom-wire.py').render(json.loads(public_raw))
    public_bundle = packet / 'public-bundle'
    dump(public_bundle / 'wire-requests.json', wire)
    template = (ROOT / 'scripts/fixtures/symptom_wire_preflight_test.go.txt').read_bytes()
    test_file = work / 'symptom_wire_preflight_test.go'
    virtual = ROOT / 'internal/system/mcp/suspects_fixture_test.go'
    original_test = virtual.read_bytes()
    # Preserve all existing tests; append the preparation test in the overlay only.
    backing = overlay_source(original_test, template)
    test_file.write_bytes(backing)
    overlay = work / 'overlay.json'
    dump(overlay, {'Replace': {str(virtual): str(test_file)}})
    env = dict(os.environ, GOROOT=str(goroot), PATH=str(goroot / 'bin') + os.pathsep + os.environ.get('PATH', ''),
               GOTOOLCHAIN='local', GOENV='off', GOWORK='off', GOFLAGS='', GOPROXY='off', GOSUMDB='off',
               GOTELEMETRY='off', GOCACHE='/private/tmp/ks-corpus-go-cache',
               GOLANGCI_LINT_CACHE='/private/tmp/ks-collection-lint-cache',
               KS_SYMPTOM_WIRE_PATH=str(public_bundle / 'wire-requests.json'))
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit Go1.26.9 required')
    reference = subprocess.check_output(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip()
    if subprocess.check_output(['git', '-C', str(ROOT), 'diff', reference, '--', '*.go']):
        raise ValueError('maintained Go source changed before preflight')
    quality = []
    commands = {'build': [str(go), 'test', '-overlay=' + str(overlay), '-c', '-o', str(work / 'parser.test'), './internal/system/mcp'],
                'fmt': [str(goroot / 'bin/gofmt'), '-l', str(test_file)],
                'vet': [str(go), 'vet', '-overlay=' + str(overlay), './internal/system/mcp'],
                'lint': [str(lint), 'run', '--timeout=120s', './internal/system/mcp']}
    for kind, command in commands.items():
        run_env = dict(env, GOFLAGS='-overlay=' + str(overlay)) if kind == 'lint' else env
        result = subprocess.run(command, cwd=ROOT, env=run_env, capture_output=True, timeout=180)
        (packet / 'logs' / (kind + '.stdout.txt')).write_bytes(result.stdout)
        (packet / 'logs' / (kind + '.stderr.txt')).write_bytes(result.stderr)
        if result.returncode or (kind == 'fmt' and result.stdout.strip()):
            raise RuntimeError('preflight quality failed: ' + kind + '; logs: ' + str(packet / 'logs'))
        quality.append({'kind': kind, 'exit_code': 0, 'command': command[1:],
                        'go_overlay_enabled': kind != 'fmt'})
        print('Parser overlay ' + kind + ': passed', flush=True)
    runs, observations, controls = [], [], []
    for mode in ['normal', 'race']:
        command = [str(go), 'test', '-overlay=' + str(overlay), '-json', '-count=1',
                   '-run=^TestSymptomWirePreflight', '-timeout=60s'] + (['-race'] if mode == 'race' else []) + ['./internal/system/mcp']
        result = subprocess.run(command, cwd=ROOT, env=env, capture_output=True, timeout=180)
        (packet / 'logs' / (mode + '.jsonl')).write_bytes(result.stdout)
        (packet / 'logs' / (mode + '.stderr.txt')).write_bytes(result.stderr)
        if result.returncode:
            raise RuntimeError('actual parser preflight failed: ' + mode)
        events = [json.loads(line) for line in result.stdout.splitlines() if line]
        text = ''.join(e.get('Output', '') for e in events)
        rows, checks = [], []
        for line in text.splitlines():
            if 'WIRE_OBSERVATION ' in line:
                rows.append(dict(json.loads(line.split('WIRE_OBSERVATION ', 1)[1]), mode=mode))
            if 'WIRE_CONTROL ' in line:
                checks.append(dict(json.loads(line.split('WIRE_CONTROL ', 1)[1]), mode=mode))
        if len(rows) != 20 or len(checks) != 6:
            raise ValueError('incomplete parser observations')
        observations.extend(rows)
        controls.extend(checks)
        runs.append({'mode': mode, 'exit_code': 0, 'command': command[1:], 'public_requests': 20, 'controls': 6})
        print(mode + ': 20 actual parser requests and 6 controls passed; backend calls 0', flush=True)
    bindings = {}
    for name in ['go.mod', 'go.sum', '.golangci.yml', 'internal/system/mcp/suspects.go',
                 'internal/system/mcp/server_test.go', 'internal/system/mcp/suspects_fixture_test.go', 'internal/system/suspects/compose.go',
                 'scripts/fixtures/symptom_wire_preflight_test.go.txt', 'scripts/export-symptom-wire.py']:
        bindings[name] = sha((ROOT / name).read_bytes())
    dump(packet / 'summary.json', {'kind': 'actual_request_parser_preflight_not_search_or_host_evaluation',
         'go_version': version, 'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(),
         'go_source_reference_commit': reference, 'maintained_go_worktree_unchanged': True,
         'quality_checks': quality, 'runs': runs, 'observations': observations, 'controls': controls,
         'backend_calls': 0, 'retrieval_runs': 0, 'model_calls': 0, 'agent_role_runs': 0,
         'formal_runs': 0, 'independent_gold_records': 0, 'macro_recall_at_10': None,
         'public_bundle_scope': 'request packaging separation; not an OS filesystem access boundary',
         'virtual_test_file': str(virtual), 'physical_test_file_created_in_repo': False,
         'existing_test_file_byte_unchanged': virtual.read_bytes() == original_test})
    (packet / 'template.go.txt').write_bytes(template)
    (packet / 'overlay-base.go.txt').write_bytes(original_test)
    (packet / 'overlay-backing.go.txt').write_bytes(backing)
    (packet / 'overlay.json.txt').write_bytes(overlay.read_bytes())
    (packet / 'runner.py.txt').write_bytes(Path(__file__).read_bytes())
    (packet / 'exporter.py.txt').write_bytes((ROOT / 'scripts/export-symptom-wire.py').read_bytes())
    diagnostics = []
    if prior_failed_output is not None:
        old = prior_failed_output / 'packet/logs'
        target = packet / 'prior-overlay-diagnostic'
        shutil.copytree(old, target)
        shutil.copyfile(prior_failed_output / 'work/overlay.json', target / 'overlay.json.txt')
        shutil.copyfile(prior_failed_output / 'work/symptom_wire_preflight_test.go', target / 'template.go.txt')
        diagnostics.append({'kind': 'new_virtual_file_unreadable_by_lint',
                            'scope': 'prior_partial_quality_attempt_not_final_parser_runs',
                            'reason': 'lint source loader reads disk paths; append tests to an existing file through the overlay',
                            'go_files_changed_in_repo': False, 'backend_calls': 0})
    dump(packet / 'diagnostics.json', diagnostics)
    dump(packet / 'manifest.json', {'schema_version': 1, 'kind': 'symptom_wire_parser_preparation_only',
         'input_manifest': str(BASE / 'symptom-inputs-v1/manifest.json'),
         'input_manifest_sha256': sha((inputs / 'manifest.json').read_bytes()),
         'public_input_sha256': sha(public_raw), 'source_sha256': bindings,
         'counts': {'public_requests': 20, 'modes': 2, 'request_observations': 40, 'controls': 12, 'quality_checks': 4},
         'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}})
    print('Parser preflight packet: ' + str(packet), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--goroot', type=Path, required=True)
    parser.add_argument('--lint', type=Path, required=True)
    parser.add_argument('--prior-failed-output', type=Path)
    args = parser.parse_args()
    run(args.output.resolve(), args.goroot.resolve(), args.lint.resolve(), args.prior_failed_output)
