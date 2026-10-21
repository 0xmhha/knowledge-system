"""Reject rehashed but incomplete, foreign or overstated public-read evidence."""
import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

spec = importlib.util.spec_from_file_location('collection_guard', Path(__file__).with_name('check-collection-public.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class PublicCollectionTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='ks-public-guard-', dir='/private/tmp')
        self.addCleanup(temp.cleanup)
        self.folder = Path(temp.name) / 'packet'
        shutil.copytree(guard.PACKET, self.folder)

    def edit(self, name, mutate):
        path = self.folder / name
        compressed = path.suffix == '.gz'
        data = gzip.decompress(path.read_bytes()) if compressed else path.read_bytes()
        value = [json.loads(x) for x in data.splitlines()] if '.jsonl' in name else json.loads(data)
        mutate(value)
        data = (b''.join((json.dumps(r) + '\n').encode() for r in value) if '.jsonl' in name
                else json.dumps(value).encode())
        path.write_bytes(gzip.compress(data, mtime=0) if compressed else data)
        self.rehash(name)

    def rehash(self, name):
        path = self.folder / name
        p = self.folder / 'manifest.json'
        manifest = json.loads(p.read_text())
        row = next(r for r in manifest['artifacts'] if r['path'] == name)
        row.update(sha256=hashlib.sha256(path.read_bytes()).hexdigest(), bytes=path.stat().st_size)
        if 'uncompressed_sha256' in row:
            row['uncompressed_sha256'] = hashlib.sha256(gzip.decompress(path.read_bytes())).hexdigest()
        p.write_text(json.dumps(manifest))

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_missing_reply_even_after_rehash(self):
        self.edit('public/mcp.stdout.jsonl.gz', lambda rows: rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_restamped_foreign_citation(self):
        def mutate(rows):
            pack = next(r['result']['structuredContent']['groups'][0]['evidence'] for r in rows
                        if r.get('result', {}).get('structuredContent', {}).get('groups'))
            pack['citations'][0]['project_id'] = 'foreign'
            for body in pack['bodies']:
                body['citation']['project_id'] = 'foreign'
            value = copy.deepcopy(pack)
            value['metadata'].pop('integrity_hash')
            pack['metadata']['integrity_hash'] = hashlib.sha256(json.dumps(value, sort_keys=True,
                ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()
        self.edit('public/mcp.stdout.jsonl.gz', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_no_full_feature_or_quality_promotion(self):
        path = self.folder / 'manifest.json'
        manifest = json.loads(path.read_text())
        manifest.update(full_KM01_complete=True, independent_acceptance=True)
        path.write_text(json.dumps(manifest))
        self.assertTrue(guard.check(self.folder))

    def test_changed_retained_bytes_even_after_rehash(self):
        path = next(p for p in (self.folder / 'source-blobs').iterdir() if b'SharedConsensus' in p.read_bytes())
        path.write_bytes(path.read_bytes().replace(b'SharedConsensus', b'ForeignConsensus'))
        self.rehash(path.relative_to(self.folder).as_posix())
        self.assertTrue(guard.check(self.folder))

    def test_invalid_startup_cannot_be_allowed(self):
        self.edit('startup-rejections.json', lambda rows: rows[0].update(exit_code=0))
        self.assertTrue(guard.check(self.folder))

    def test_whole_module_skip_cannot_be_hidden(self):
        self.edit('validation.json', lambda data: data['module'].update(skipped_tests=0, passed_tests=3619))
        self.assertTrue(guard.check(self.folder))

    def test_race_failure_cannot_be_hidden(self):
        def mutate(rows):
            next(r for r in rows if r.get('Test') and r.get('Action') == 'pass')['Action'] = 'fail'
        self.edit('adapter-race.jsonl.gz', mutate)
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
