"""Reject false task/test completion, missing cases, and public-scope promotion."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

spec = importlib.util.spec_from_file_location('roadmap_guard', Path(__file__).with_name('check-knowledge-roadmap.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class RoadmapTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='ks-roadmap-guard-', dir='/private/tmp')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.folder = self.root / guard.PACKET.relative_to(guard.ROOT)
        shutil.copytree(guard.PACKET, self.folder)
        manifest = json.loads((self.folder / 'manifest.json').read_text())
        files = list(manifest['source_sha256']) + [
            'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json',
            'docs/spec-driven/KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md',
            'docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md',
            'docs/spec-driven/PROJECT-COLLECTION-ISOLATION.md',
        ]
        for name in files:
            p = self.root / name
            p.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(guard.ROOT / name, p)

    def edit_json(self, path, action):
        p = self.root / path
        value = json.loads(p.read_text())
        action(value)
        p.write_text(json.dumps(value))

    def test_current_rows_and_evidence_agree(self):
        self.assertEqual(guard.check(self.folder, self.root), [])

    def test_partial_feature_cannot_be_marked_complete(self):
        self.edit_json('docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json',
                       lambda d: d['items'][0].update(status='complete'))
        self.assertTrue(guard.check(self.folder, self.root))

    def test_unfinished_checkbox_cannot_be_ticked(self):
        p = self.root / 'docs/spec-driven/KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md'
        p.write_text(p.read_text().replace('- [ ] KM-03.1:', '- [x] KM-03.1:'))
        self.assertTrue(guard.check(self.folder, self.root))

    def test_matrix_cannot_promote_pending_implementation(self):
        p = self.root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md'
        p.write_text(p.read_text().replace('매니저 등록·설치·초기 구축·복구 절차 | 미착수',
                                          '매니저 등록·설치·초기 구축·복구 절차 | 완료'))
        self.assertTrue(guard.check(self.folder, self.root))

    def test_rehashed_omitted_race_case_is_rejected(self):
        p = self.folder / 'collection-race.jsonl.gz'
        rows = [json.loads(line) for line in gzip.decompress(p.read_bytes()).splitlines()]
        target = next(r['Test'] for r in rows if r.get('Test'))
        rows = [r for r in rows if r.get('Test') != target]
        raw = b''.join((json.dumps(r) + '\n').encode() for r in rows)
        p.write_bytes(gzip.compress(raw, mtime=0))
        manifest = json.loads((self.folder / 'manifest.json').read_text())
        for row in manifest['artifacts']:
            if row['path'] == p.name:
                row.update(sha256=hashlib.sha256(p.read_bytes()).hexdigest(), bytes=p.stat().st_size,
                           uncompressed_sha256=hashlib.sha256(raw).hexdigest())
        (self.folder / 'manifest.json').write_text(json.dumps(manifest))
        self.assertTrue(guard.check(self.folder, self.root))

    def test_library_cannot_claim_public_MCP_connection(self):
        path = (self.folder / 'manifest.json').relative_to(self.root).as_posix()
        self.edit_json(path, lambda d: d.update(public_MCP_connected=True, full_KM01_complete=True))
        self.assertTrue(guard.check(self.folder, self.root))

    def test_changed_source_requires_new_evidence(self):
        p = self.root / 'internal/system/collection/session.go'
        p.write_text(p.read_text() + '\n// changed after verification\n')
        self.assertTrue(guard.check(self.folder, self.root))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
