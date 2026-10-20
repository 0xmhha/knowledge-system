"""Verify immutable KM-01 evidence in its committed document/source snapshot.

Later roadmap counters must not rewrite earlier evidence or make its completed
runtime observation fail merely because another subtask has since finished.
"""
import hashlib
import importlib.util
import io
import json
from pathlib import Path, PurePosixPath
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
REVISION = 'cb373e94'
BASE = 'system/eval/knowledge-roadmap-2026-10-15/'


def verify(root=ROOT):
    revision = subprocess.check_output(['git', 'rev-parse', REVISION + '^{commit}'], cwd=root, text=True).strip()
    def read(name):
        return subprocess.check_output(['git', 'show', revision + ':' + name], cwd=root)
    final = json.loads(read(BASE + 'km-01-final-isolation/manifest.json'))
    foundation = json.loads(read(BASE + 'km-01-foundation/manifest.json'))
    binding = json.loads(read(BASE + 'km-01-final-isolation/reused-source-binding.json'))
    paths = set(final['source_sha256']) | set(foundation['source_sha256']) | set(binding['source_sha256'])
    paths.update(BASE + folder for folder in ('km-01-foundation', 'km-01-public-read', 'km-01-final-isolation'))
    paths.update('docs/spec-driven/' + name for name in ('KNOWLEDGE-ROADMAP-PROGRESS.json',
        'KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md', 'KNOWLEDGE-ROADMAP-TEST-MATRIX.md', 'PROJECT-COLLECTION-ISOLATION.md'))
    for name in paths:
        p = PurePosixPath(name)
        if p.is_absolute() or '..' in p.parts or str(p) != name:
            raise ValueError('unsafe committed evidence path')
    data = subprocess.check_output(['git', 'archive', '--format=tar', revision, '--', *sorted(paths)], cwd=root)
    with tempfile.TemporaryDirectory(prefix='ks-committed-collection-', dir='/private/tmp') as temp:
        snapshot = Path(temp).resolve()
        with tarfile.open(fileobj=io.BytesIO(data)) as archive:
            for item in archive:
                p = (snapshot / item.name).resolve()
                if not p.is_relative_to(snapshot) or not (item.isfile() or item.isdir()):
                    raise ValueError('unsafe committed evidence archive')
                if item.isdir():
                    p.mkdir(parents=True, exist_ok=True)
                else:
                    p.parent.mkdir(parents=True, exist_ok=True)
                    p.write_bytes(archive.extractfile(item).read())
        path = snapshot / 'scripts/check-collection-isolation.py'
        spec = importlib.util.spec_from_file_location('committed_collection_guard', path)
        guard = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(guard)
        errors = guard.check(folder=snapshot / BASE / 'km-01-final-isolation', root=snapshot)
        if errors:
            raise ValueError('committed collection proof failed: ' + '; '.join(errors))
    matching = sum((root / name).is_file() and hashlib.sha256((root / name).read_bytes()).hexdigest() == value
                   for name, value in binding['source_sha256'].items())
    dependencies_match = all((root / name).read_bytes() == read(name) for name in ('go.mod', 'go.sum'))
    return {'scope': 'historical_committed_KM01_runtime_and_metadata_only', 'revision': revision,
            'manifest': BASE + 'km-01-final-isolation/manifest.json',
            'manifest_sha256': hashlib.sha256(read(BASE + 'km-01-final-isolation/manifest.json')).hexdigest(),
            'verification_exit_code': 0, 'committed_Go_sources': len(binding['source_sha256']),
            'matching_current_Go_sources': matching, 'dependency_inputs_match': dependencies_match,
            'current_Go_validation_reusable': matching == len(binding['source_sha256']) and dependencies_match,
            'current_roadmap_validation': 'separate; no historical counter applied to current progress'}


if __name__ == '__main__':
    print(json.dumps(verify(), indent=2))
