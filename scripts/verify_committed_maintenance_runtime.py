"""Keep KM-02.2 proof in its original committed source/document snapshot.

This does not reuse historical Go validation for the changed runtime. Git is
read-only; the temporary work tree contains only selected committed evidence.
"""
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
REVISION = '31c1f30d'
BASE = 'system/eval/knowledge-roadmap-2026-10-15/'


def verify(root=ROOT):
    revision = subprocess.check_output(['git', 'rev-parse', REVISION + '^{commit}'], cwd=root, text=True).strip()
    def read(name):
        return subprocess.check_output(['git', 'show', revision + ':' + name], cwd=root)
    binding = json.loads(read(BASE + 'km-02-public-maintenance/source-binding.json'))
    paths = set(binding['source_sha256']) | set(binding['approved_input_sha256']) | {'scripts', 'system/contracts/versioned-maintenance', BASE.rstrip('/')}
    paths.update('docs/spec-driven/' + name for name in ('KNOWLEDGE-ROADMAP-PROGRESS.json',
        'KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md', 'KNOWLEDGE-ROADMAP-TEST-MATRIX.md',
        'PROJECT-COLLECTION-ISOLATION.md', 'VERSIONED-MAINTENANCE-CONTRACT.md', 'VERSIONED-MAINTENANCE-RUNTIME.md'))
    for name in paths:
        p = PurePosixPath(name)
        if p.is_absolute() or '..' in p.parts or str(p) != name:
            raise ValueError('unsafe committed path')
    data = subprocess.check_output(['git', 'archive', '--format=tar', revision, '--', *sorted(paths)], cwd=root)
    env = os.environ.copy()
    env['GIT_DIR'] = subprocess.check_output(['git', 'rev-parse', '--absolute-git-dir'], cwd=root, text=True).strip()
    env['PYTHONDONTWRITEBYTECODE'] = '1'
    receipts = []
    with tempfile.TemporaryDirectory(prefix='ks-maintenance-runtime-history-', dir='/private/tmp') as temp:
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
        for script in ('check-maintenance-runtime-evidence.py', 'test_maintenance_runtime_evidence.py'):
            result = subprocess.run(['python3', 'scripts/' + script], cwd=snapshot, env=env, capture_output=True, timeout=90)
            receipts.append({'script': script, 'exit_code': result.returncode,
                             'stdout_sha256': hashlib.sha256(result.stdout).hexdigest(),
                             'stderr': result.stderr.decode().strip()})
            if result.returncode:
                raise ValueError('committed runtime proof failed: ' + result.stdout.decode() + result.stderr.decode())
    return {'scope': 'historical_KM02_2_process_lifetime_runtime_not_current_Go_validation', 'revision': revision,
            'manifest_sha256': hashlib.sha256(read(BASE + 'km-02-public-maintenance/manifest.json')).hexdigest(),
            'verification_exit_code': 0, 'checks': receipts, 'current_Go_validation_reused': False}


if __name__ == '__main__':
    print(json.dumps(verify(), indent=2))
