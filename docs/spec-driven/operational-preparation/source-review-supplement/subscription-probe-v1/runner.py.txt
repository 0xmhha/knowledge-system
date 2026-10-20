#!/usr/bin/env python3
"""Run a bounded, offline extracted-method fixture; never execute the full target API."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def run(output, goroot):
    spec = importlib.util.spec_from_file_location('ticket_guard', ROOT / 'scripts/check-subscription-ticket.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    ticket = ROOT / BASE / 'subscription-ticket-v1'
    errors = guard.check(ticket)
    if errors:
        raise ValueError('\n'.join(errors))
    if output.exists():
        raise ValueError('output exists; do not overwrite prior observations')
    packet, work = output / 'packet', output / 'work'
    packet.mkdir(parents=True)
    work.mkdir()
    scratch = output / 'tmp'
    scratch.mkdir()
    env = dict(os.environ, GOROOT=str(goroot), PATH=str(goroot / 'bin') + os.pathsep + os.environ.get('PATH', ''),
               GOTOOLCHAIN='local', GOWORK='off', GOENV='off', GOFLAGS='', GOPROXY='off', GOSUMDB='off',
               GOTELEMETRY='off', GOCACHE='/private/tmp/ks-subscription-probe-go-cache',
               GOMODCACHE=str(output / 'modcache'), GOTMPDIR=str(scratch))
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit go1.26.9 toolchain required')
    private = json.loads((ticket / 'private-review.json').read_text())
    harness = (ROOT / 'scripts/fixtures/subscription_probe_test.go.txt').read_bytes()
    types = b'package subscriptionfixture\n\nimport "sync"\n\n' + (ticket / 'source-types.go.txt').read_bytes()
    types += b'\ntype DownloaderAPI struct { uninstallSyncSubscription chan *uninstallSyncSubscriptionRequest }\n'
    logs, observations = [], []
    compiled = {}
    for variant in private['variants']:
        identity = variant['id']
        directory = work / identity
        directory.mkdir()
        inputs = {'go.mod': b'module subscriptionfixture\n\ngo 1.26.9\n',
                  'types.go': types, 'variant.go': b'package subscriptionfixture\n\n' + (ticket / variant['artifact']).read_bytes(),
                  'subscription_test.go': harness,
                  'expectations.json': (json.dumps(private['expected_draft_matrix'], sort_keys=True) + '\n').encode()}
        for name, raw in inputs.items():
            (directory / name).write_bytes(raw)
            captured = packet / 'compiled-inputs' / identity / (name + '.txt')
            captured.parent.mkdir(parents=True, exist_ok=True)
            captured.write_bytes(raw)
        compiled[identity] = {name: sha(raw) for name, raw in inputs.items()}
        # gofmt checks generated files without changing the exact extracted method.
        formatted = subprocess.run([str(goroot / 'bin/gofmt'), '-l', 'types.go', 'variant.go', 'subscription_test.go'],
                                   cwd=directory, env=env, capture_output=True, text=True, timeout=20, check=True)
        # The minimal API declaration is generated; format that wrapper only.
        if formatted.stdout.strip():
            for name in formatted.stdout.splitlines():
                if name != 'types.go':
                    raise ValueError('unformatted fixture input: ' + identity + '/' + name)
            subprocess.run([str(goroot / 'bin/gofmt'), '-w', 'types.go'], cwd=directory, env=env, check=True, timeout=20)
            captured = packet / 'compiled-inputs' / identity / 'types.go.txt'
            captured.write_bytes((directory / 'types.go').read_bytes())
            compiled[identity]['types.go'] = sha(captured.read_bytes())
        variant_env = dict(env, KS_DRAFT_VARIANT=identity)
        for mode in ['normal', 'race']:
            args = [str(go), 'test', '-json', '-count=1', '-timeout=20s']
            if mode == 'race':
                args.append('-race')
            args.append('./...')
            result = subprocess.run(args, cwd=directory, env=variant_env, capture_output=True, timeout=120)
            name = identity + '-' + mode
            (packet / 'logs').mkdir(exist_ok=True)
            (packet / 'logs' / (name + '.jsonl')).write_bytes(result.stdout)
            (packet / 'logs' / (name + '.stderr.txt')).write_bytes(result.stderr)
            if result.returncode:
                raise RuntimeError('fixture failed: ' + name + '; inspect ' + str(packet / 'logs'))
            events = [json.loads(line) for line in result.stdout.splitlines() if line]
            output_text = ''.join(e.get('Output', '') for e in events)
            rows = []
            for line in output_text.splitlines():
                if 'DRAFT_OBSERVATION ' in line:
                    row = json.loads(line.split('DRAFT_OBSERVATION ', 1)[1])
                    row['mode'] = mode
                    rows.append(row)
            if len(rows) != 5 or any(e.get('Action') == 'fail' for e in events):
                raise ValueError('incomplete fixture events: ' + name)
            observations.extend(rows)
            logs.append({'variant': identity, 'mode': mode, 'exit_code': result.returncode,
                         'command': args[1:], 'working_directory': str(directory), 'observations': len(rows)})
            print(name + ': 5 observations; owned goroutines stopped', flush=True)
    summary = {'kind': 'local_extracted_method_probe_not_full_project_or_independent_acceptance',
               'toolchain': version, 'observations': observations, 'runs': logs, 'compiled_inputs_sha256': compiled,
               'local_model_calls': 0, 'nodes_started': 0, 'formal_runtime_executions': 0,
               'independent_acceptable_candidate_set': None, 'independent_oracle': None,
               'verified_project_defect': False, 'independent_families': 0,
               'candidate_result_scope': 'AI fixture criteria only; no production patch adopted',
               'limitations': private['limitations']}
    dump(packet / 'summary.json', summary)
    (packet / 'runner.py.txt').write_bytes(Path(__file__).read_bytes())
    manifest = {'schema_version': 1, 'kind': 'subscription_extracted_method_local_probe',
                'ticket_manifest': str(BASE / 'subscription-ticket-v1/manifest.json'),
                'ticket_manifest_sha256': sha((ticket / 'manifest.json').read_bytes()),
                'harness': 'scripts/fixtures/subscription_probe_test.go.txt', 'harness_sha256': sha(harness),
                'counts': {'variants': 5, 'correlated_schedules': 5, 'modes': 2, 'observations': len(observations)},
                'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}}
    dump(packet / 'manifest.json', manifest)
    print('Probe packet: ' + str(packet), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--goroot', type=Path, required=True)
    args = parser.parse_args()
    run(args.output.resolve(), args.goroot.resolve())
