#!/usr/bin/env python3
"""Check portable evidence history without using the previous machine's paths."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def check(root=ROOT, restore_generated=False):
    manifest = json.loads((root / 'docs/spec-driven/HANDOFF-HISTORY.json').read_text())
    errors = []
    for name, row in manifest.get('consumer_bundles', {}).items():
        p = root / row['path']
        if not p.is_file() or hashlib.sha256(p.read_bytes()).hexdigest() != row['sha256']:
            errors.append('missing or changed consumer source bundle: ' + name)
    pending = []
    for name, row in manifest.get('generated_inputs', {}).items():
        p = root / name
        archive = root / row['archive_path']
        if not p.resolve().is_relative_to(root.resolve()) or not archive.resolve().is_relative_to(root.resolve()):
            errors.append('input path escapes checkout')
            continue
        data = gzip.decompress(archive.read_bytes())
        if hashlib.sha256(data).hexdigest() != row['sha256']:
            errors.append('changed generated input capsule: ' + name)
        elif p.exists():
            if not p.is_file() or hashlib.sha256(p.read_bytes()).hexdigest() != row['sha256']:
                errors.append('existing generated input differs; refusing overwrite: ' + name)
        elif restore_generated:
            pending.append((p, data))
        else:
            errors.append('missing generated input; use --restore-generated-inputs: ' + name)
    for name, digest in manifest.get('restored_artifacts', {}).items():
        p = root / name
        if not p.is_file() or hashlib.sha256(p.read_bytes()).hexdigest() != digest:
            errors.append('missing or changed retained artifact: ' + name)
    roots = []
    for row in manifest['history_tags']:
        result = subprocess.run(['git', 'rev-parse', '--verify', row['ref'] + '^{commit}'],
                                cwd=root, capture_output=True, text=True)
        if result.returncode or result.stdout.strip() != row['commit']:
            errors.append('missing or changed history tag: ' + row['ref'])
        else:
            roots.append(row['commit'])
    if errors:
        return errors
    history = set(subprocess.check_output(['git', 'rev-list', *roots], cwd=root,
                                         text=True).splitlines())
    for commit in manifest['referenced_local_commits']:
        if commit not in history:
            errors.append('referenced commit not retained by evidence tags: ' + commit)
    if not errors:
        for p, data in pending:
            p.parent.mkdir(parents=True, exist_ok=True)
            with p.open('xb') as stream:
                stream.write(data)
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--restore-generated-inputs', action='store_true',
                        help='restore missing exact viewer inputs; refuse differing existing files')
    args = parser.parse_args()
    findings = check(restore_generated=args.restore_generated_inputs)
    if findings:
        raise SystemExit('\n'.join(findings))
    print('handoff history: PASS; referenced commits reachable from pinned evidence tags')
