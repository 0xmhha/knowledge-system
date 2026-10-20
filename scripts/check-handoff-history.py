#!/usr/bin/env python3
"""Check portable evidence history without using the previous machine's paths."""
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def check(root=ROOT):
    manifest = json.loads((root / 'docs/spec-driven/HANDOFF-HISTORY.json').read_text())
    errors = []
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
    return errors


if __name__ == '__main__':
    findings = check()
    if findings:
        raise SystemExit('\n'.join(findings))
    print('handoff history: PASS; referenced commits reachable from pinned evidence tags')
