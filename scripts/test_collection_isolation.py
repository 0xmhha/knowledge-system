"""Refuse altered/missing isolation evidence even when artifact hashes change."""
import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('final_isolation_guard', Path(__file__).with_name('check-collection-isolation.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class FinalIsolationTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='ks-isolation-guard-', dir='/private/tmp')
        self.addCleanup(temp.cleanup)
        self.folder = Path(temp.name) / 'packet'
        shutil.copytree(guard.PACKET, self.folder)

    def edit(self, name, mutate):
        p = self.folder / name
        data = gzip.decompress(p.read_bytes()) if p.suffix == '.gz' else p.read_bytes()
        value = [json.loads(x) for x in data.splitlines()] if '.jsonl' in name else json.loads(data)
        mutate(value)
        data = (b''.join((json.dumps(r) + '\n').encode() for r in value) if '.jsonl' in name else json.dumps(value).encode())
        p.write_bytes(gzip.compress(data, mtime=0) if p.suffix == '.gz' else data)
        manifest_path = self.folder / 'manifest.json'
        m = json.loads(manifest_path.read_text())
        row = next(r for r in m['artifacts'] if r['path'] == name)
        row.update(sha256=hashlib.sha256(p.read_bytes()).hexdigest(), bytes=p.stat().st_size)
        if 'uncompressed_sha256' in row:
            row['uncompressed_sha256'] = hashlib.sha256(data).hexdigest()
        manifest_path.write_text(json.dumps(m))

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_missing_response_rehashed(self):
        self.edit('public/mcp.stdout.jsonl.gz', lambda rows: rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_same_bytes_fork_foreign_citation_restamped(self):
        def mutate(rows):
            pack = next(r['result']['structuredContent']['groups'][0]['evidence'] for r in rows
                        if r.get('result', {}).get('structuredContent', {}).get('groups'))
            pack['citations'][0]['project_id'] = 'project-b'
            for b in pack['bodies']:
                b['citation']['project_id'] = 'project-b'
            value = copy.deepcopy(pack)
            value['metadata'].pop('integrity_hash')
            pack['metadata']['integrity_hash'] = hashlib.sha256(json.dumps(value, sort_keys=True,
                ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()
        self.edit('public/mcp.stdout.jsonl.gz', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_mixed_embedding_response_dimension_rehashed(self):
        def mutate(rows):
            row = next(r for r in rows if r['path'] == '/api/embed')
            row['response']['embeddings'][0].pop()
        self.edit('embedding-http.jsonl.gz', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_rejected_writer_mutation_rehashed(self):
        self.edit('writer-tree-after.json', lambda data: data.update(unexpected_created_file={'sha256': '0'*64}))
        self.assertTrue(guard.check(self.folder))

    def test_shutdown_failure_rehashed(self):
        self.edit('sessions.json', lambda rows: rows[0].update(exit_code=1, clean_EOF=False))
        self.assertTrue(guard.check(self.folder))

    def test_go_skips_cannot_be_promoted(self):
        self.edit('validation.json', lambda value: value['module'].update(skipped_tests=0, passed_tests=3619))
        self.assertTrue(guard.check(self.folder))

    def test_independent_quality_cannot_be_promoted(self):
        p = self.folder / 'manifest.json'
        value = json.loads(p.read_text())
        value.update(independent_acceptance=True, production_activated=True)
        p.write_text(json.dumps(value))
        self.assertTrue(guard.check(self.folder))

    def test_foreign_embedding_dispatch_report_rehashed(self):
        def mutate(rows):
            impl = [r for r in rows if r['tool'] == 'cks.collection.get_context' and not r['response'].get('isError')
                    and r['arguments']['mode'] == 'implementation']
            b1 = next(r for r in impl if r['arguments']['target']['project_id'] == 'project-b')
            b2 = next(r for r in impl if r['arguments']['target']['project_id'] == 'project-b'
                      and r['arguments']['target']['dataset_id'] != b1['arguments']['target']['dataset_id'])
            b1.update(http_start=b2['http_start'], http_end=b2['http_end'])
        self.edit('results.json', mutate)
        self.assertTrue(guard.check(self.folder))


if __name__ == '__main__':
    unittest.main()
