#!/usr/bin/env python3
"""Build, format-check, vet and lint the exact previously recorded fixture inputs."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def validate(packet, work, goroot, lint):
    if (packet / 'quality.json').exists():
        raise ValueError('preserve existing quality receipt')
    summary = json.loads((packet / 'summary.json').read_text())
    env = dict(os.environ, GOROOT=str(goroot), GOTOOLCHAIN='local', GOENV='off', GOWORK='off',
               GOFLAGS='', GOPROXY='off', GOSUMDB='off', GOTELEMETRY='off',
               PATH=str(goroot / 'bin') + os.pathsep + os.environ.get('PATH', ''),
               GOCACHE='/private/tmp/ks-subscription-probe-go-cache',
               GOLANGCI_LINT_CACHE='/private/tmp/ks-subscription-probe-lint-cache')
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit go1.26.9 required')
    lint_version = subprocess.check_output([str(lint), 'version'], env=env, text=True).strip()
    checks = []
    for variant, inputs in summary['compiled_inputs_sha256'].items():
        directory = work / variant
        for name, digest in inputs.items():
            if sha((directory / name).read_bytes()) != digest:
                raise ValueError('worktree differs from recorded compiled input: ' + variant + '/' + name)
        commands = {'build': [str(go), 'build', './...'], 'fmt': [str(goroot / 'bin/gofmt'), '-l', 'types.go', 'variant.go', 'subscription_test.go'],
                    'vet': [str(go), 'vet', './...'], 'lint': [str(lint), 'run', '--no-config', '--timeout=60s']}
        for kind, args in commands.items():
            result = subprocess.run(args, cwd=directory, env=env, capture_output=True, timeout=90)
            stem = 'quality-' + variant + '-' + kind
            (packet / 'logs' / (stem + '.stdout.txt')).write_bytes(result.stdout)
            (packet / 'logs' / (stem + '.stderr.txt')).write_bytes(result.stderr)
            if result.returncode or (kind == 'fmt' and result.stdout.strip()):
                raise RuntimeError('quality check failed: ' + stem)
            checks.append({'variant': variant, 'check': kind, 'command': args[1:], 'exit_code': result.returncode, 'log_stem': stem})
        print(variant + ': build/fmt/vet/lint passed', flush=True)
    receipt = {'kind': 'quality_checks_of_recorded_extracted_fixture_not_production', 'go_version': version,
               'lint_version': lint_version, 'compiled_inputs_sha256': summary['compiled_inputs_sha256'], 'checks': checks}
    (packet / 'quality.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
    (packet / 'quality-validator.py.txt').write_bytes(Path(__file__).read_bytes())
    p = packet / 'manifest.json'
    m = json.loads(p.read_text())
    m['artifacts'] = {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file() and p.name != 'manifest.json'}
    p.write_text(json.dumps(m, ensure_ascii=False, indent=2) + '\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--packet', type=Path, required=True)
    parser.add_argument('--work-root', type=Path, required=True)
    parser.add_argument('--goroot', type=Path, required=True)
    parser.add_argument('--lint', type=Path, required=True)
    args = parser.parse_args()
    validate(args.packet.resolve(), args.work_root.resolve(), args.goroot.resolve(), args.lint.resolve())
