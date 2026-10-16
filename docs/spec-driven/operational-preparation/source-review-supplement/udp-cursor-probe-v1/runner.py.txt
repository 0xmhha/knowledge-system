#!/usr/bin/env python3
"""Probe exact resetTimeout statements with synthetic deadlines and a timer recorder."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def cursor_draft(raw):
    before = b'for el := plist.Front(); el != nil; el = el.Next() {'
    after = b'for el := plist.Front(); el != nil; {\n\t\t\tnext := el.Next()'
    if raw.count(before) != 1 or raw.count(b'plist.Remove(el)\n') != 1:
        raise ValueError('unexpected source shape')
    return raw.replace(before, after).replace(b'plist.Remove(el)\n', b'plist.Remove(el)\n\t\t\tel = next\n')


def run(output, goroot, lint):
    spec = importlib.util.spec_from_file_location('schedule_guard', ROOT / 'scripts/check-schedule-candidates.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    candidates = ROOT / BASE / 'schedule-candidates-v1'
    errors = guard.check(candidates)
    if errors:
        raise ValueError('\n'.join(errors))
    if output.exists():
        raise ValueError('use a new output directory')
    packet, work = output / 'packet', output / 'work'
    (packet / 'logs').mkdir(parents=True)
    work.mkdir()
    env = dict(os.environ, GOROOT=str(goroot), PATH=str(goroot / 'bin') + os.pathsep + os.environ.get('PATH', ''),
               GOTOOLCHAIN='local', GOENV='off', GOWORK='off', GOFLAGS='', GOPROXY='off', GOSUMDB='off',
               GOTELEMETRY='off', GOCACHE='/private/tmp/ks-subscription-probe-go-cache',
               GOMODCACHE=str(output / 'modcache'), GOLANGCI_LINT_CACHE='/private/tmp/ks-subscription-probe-lint-cache')
    go = goroot / 'bin/go'
    version = subprocess.check_output([str(go), 'version'], env=env, text=True).strip()
    if not version.startswith('go version go1.26.9 '):
        raise ValueError('explicit Go 1.26.9 required')
    toolchain = json.loads((candidates / 'toolchain-notes.json').read_text())
    for path, record in toolchain['sources'].items():
        if sha((goroot / path).read_bytes()) != record['sha256']:
            raise ValueError('fixture stdlib source identity mismatch')
    source = (candidates / 'sources/p2p/discover/v4_udp.go.txt').read_bytes()
    if b'respTimeout    = 500 * time.Millisecond' not in source:
        raise ValueError('unexpected production timeout constant')
    original = b''.join(source.splitlines(keepends=True)[419:440])
    template = (ROOT / 'scripts/fixtures/udp_cursor_probe_test.go.txt').read_bytes()
    (packet / 'original-closure.go.txt').write_bytes(original)
    draft = cursor_draft(original)
    (packet / 'cursor-draft-closure.go.txt').write_bytes(draft)
    observations, runs, checks, compiled = [], [], [], {}
    for variant, fragment in [('ORIGINAL', original), ('CURSOR-DRAFT', draft)]:
        directory = work / variant
        directory.mkdir()
        code = template.replace(b'\t// RESET_TIMEOUT_SOURCE\n', fragment)
        inputs = {'go.mod': b'module udpcursorfixture\n\ngo 1.26.9\n', 'cursor_test.go': code}
        compiled[variant] = {}
        for name, raw in inputs.items():
            (directory / name).write_bytes(raw)
            captured = packet / 'compiled-inputs' / variant / (name + '.txt')
            captured.parent.mkdir(parents=True, exist_ok=True)
            captured.write_bytes(raw)
            compiled[variant][name] = sha(raw)
        commands = {'build': [str(go), 'build', './...'],
                    'fmt': [str(goroot / 'bin/gofmt'), '-l', 'cursor_test.go'],
                    'vet': [str(go), 'vet', './...'],
                    'lint': [str(lint), 'run', '--no-config', '--timeout=60s']}
        # The fixture has only test code; compile it explicitly for the build check.
        commands['build'] = [str(go), 'test', '-c', '-o', str(directory / 'cursor.test')]
        for kind, args in commands.items():
            result = subprocess.run(args, cwd=directory, env=env, capture_output=True, timeout=90)
            stem = variant + '-' + kind
            (packet / 'logs' / (stem + '.stdout.txt')).write_bytes(result.stdout)
            (packet / 'logs' / (stem + '.stderr.txt')).write_bytes(result.stderr)
            if result.returncode or (kind == 'fmt' and result.stdout.strip()):
                raise RuntimeError('fixture quality failed: ' + stem)
            checks.append({'variant': variant, 'kind': kind, 'exit_code': result.returncode})
        for mode in ['normal', 'race']:
            args = [str(go), 'test', '-json', '-count=1', '-timeout=10s'] + (['-race'] if mode == 'race' else []) + ['./...']
            result = subprocess.run(args, cwd=directory, env=dict(env, KS_CURSOR_VARIANT=variant), capture_output=True, timeout=90)
            stem = variant + '-' + mode
            (packet / 'logs' / (stem + '.jsonl')).write_bytes(result.stdout)
            (packet / 'logs' / (stem + '.stderr.txt')).write_bytes(result.stderr)
            if result.returncode:
                raise RuntimeError('fixture execution failed: ' + stem)
            events = [json.loads(line) for line in result.stdout.splitlines() if line]
            text = ''.join(e.get('Output', '') for e in events)
            rows = [json.loads(line.split('CURSOR_OBSERVATION ', 1)[1]) for line in text.splitlines() if 'CURSOR_OBSERVATION ' in line]
            if len(rows) != 5:
                raise ValueError('incomplete raw observations')
            observations.extend(dict(row, mode=mode) for row in rows)
            runs.append({'variant': variant, 'mode': mode, 'exit_code': 0, 'observations': len(rows), 'command': args[1:]})
            print(stem + ': 5 synthetic observations', flush=True)
    dump(packet / 'summary.json', {'kind': 'synthetic_source_branch_probe_not_udp_or_clock_rollback',
         'go_version': version, 'lint_version': subprocess.check_output([str(lint), 'version'], env=env, text=True).strip(),
         'observations': observations, 'runs': runs, 'quality_checks': checks, 'compiled_inputs_sha256': compiled,
         'timer_scope': 'Reset/Stop call recorder; real timer firing is not tested',
         'network_calls': 0, 'nodes_started': 0, 'host_clock_changes': 0, 'formal_runs': 0,
         'accepted_independent_families': 0, 'verified_project_defect': False,
         'operational_reachability': 'not_verified', 'independent_oracle': None, 'production_patch_applied': False})
    (packet / 'runner.py.txt').write_bytes(Path(__file__).read_bytes())
    manifest = {'schema_version': 1, 'kind': 'udp_cursor_synthetic_local_probe',
                'candidate_manifest': str(BASE / 'schedule-candidates-v1/manifest.json'),
                'candidate_manifest_sha256': sha((candidates / 'manifest.json').read_bytes()),
                'template': 'scripts/fixtures/udp_cursor_probe_test.go.txt', 'template_sha256': sha(template),
                'counts': {'arms': 2, 'synthetic_cases': 5, 'modes': 2, 'observations': 20, 'quality_checks': 8},
                'artifacts': {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*')) if p.is_file()}}
    dump(packet / 'manifest.json', manifest)
    print('Synthetic probe packet: ' + str(packet), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--goroot', type=Path, required=True)
    parser.add_argument('--lint', type=Path, required=True)
    args = parser.parse_args()
    run(args.output.resolve(), args.goroot.resolve(), args.lint.resolve())
