"""Verify later maintenance checkpoints in their immutable Git snapshots.

The snapshot uses the actual read-only repository for git show/archive. It
never changes HEAD or the user's work tree and never reuses old Go results for
new sources.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
BASE = 'system/eval/knowledge-roadmap-2026-10-15/'
CHECKPOINTS = {
    'journal-foundation': {
        'revision': 'fff83134', 'folder': 'km-02-journal-foundation',
        'scripts': ['check-maintenance-journal-foundation.py', 'test_maintenance_journal_foundation.py'],
    },
}


def verify(checkpoint='journal-foundation', root=ROOT):
    selected = CHECKPOINTS[checkpoint]
    revision = subprocess.check_output(['git', 'rev-parse', selected['revision'] + '^{commit}'], cwd=root, text=True).strip()
    def read(name):
        return subprocess.check_output(['git', 'show', revision + ':' + name], cwd=root)
    folder = BASE + selected['folder']
    manifest, binding = json.loads(read(folder + '/manifest.json')), json.loads(read(folder + '/source-binding.json'))
    paths = set(manifest['source_sha256']) | set(binding['approved_input_sha256']) | {'scripts', BASE.rstrip('/'), 'system/contracts/versioned-maintenance'}
    paths.update('docs/spec-driven/' + name for name in ('KNOWLEDGE-ROADMAP-PROGRESS.json', 'KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md',
        'KNOWLEDGE-ROADMAP-TEST-MATRIX.md', 'PROJECT-COLLECTION-ISOLATION.md', 'VERSIONED-MAINTENANCE-CONTRACT.md', 'VERSIONED-MAINTENANCE-RUNTIME.md'))
    for name in paths:
        p = PurePosixPath(name)
        if p.is_absolute() or '..' in p.parts or str(p) != name:
            raise ValueError('unsafe committed path')
    data = subprocess.check_output(['git', 'archive', '--format=tar', revision, '--', *sorted(paths)], cwd=root)
    env = os.environ.copy()
    env.update(GIT_DIR=subprocess.check_output(['git', 'rev-parse', '--absolute-git-dir'], cwd=root, text=True).strip(), PYTHONDONTWRITEBYTECODE='1')
    receipts = []
    with tempfile.TemporaryDirectory(prefix='ks-maintenance-checkpoint-', dir='/private/tmp') as temp:
        snapshot = Path(temp).resolve()
        env['GIT_WORK_TREE'] = str(snapshot)
        with tarfile.open(fileobj=io.BytesIO(data)) as archive:
            for item in archive:
                target = (snapshot / item.name).resolve()
                if not target.is_relative_to(snapshot) or not (item.isdir() or item.isfile()):
                    raise ValueError('unsafe committed archive')
                if item.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                else:
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(archive.extractfile(item).read())
        for script in selected['scripts']:
            result = subprocess.run(['python3', 'scripts/' + script], cwd=snapshot, env=env, capture_output=True, timeout=90)
            receipts.append({'script': script, 'exit_code': result.returncode,
                             'stdout_sha256': hashlib.sha256(result.stdout).hexdigest(), 'stderr': result.stderr.decode().strip()})
            if result.returncode:
                raise ValueError('historical checkpoint proof failed: ' + result.stdout.decode() + result.stderr.decode())
    return {'scope': 'historical_KM02_3_A_private_foundation_and_process_compatibility_only', 'revision': revision,
            'manifest_sha256': hashlib.sha256(read(folder + '/manifest.json')).hexdigest(),
            'verification_exit_code': 0, 'checks': receipts, 'current_Go_validation_reused': False}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--checkpoint', choices=sorted(CHECKPOINTS), default='journal-foundation')
    print(json.dumps(verify(parser.parse_args().checkpoint), indent=2))
