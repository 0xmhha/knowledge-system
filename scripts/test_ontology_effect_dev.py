"""Meaningful false-completion and retained-source controls for the real packet."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('ontology_effect_guard', Path(__file__).with_name('check-ontology-effect-dev.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class EffectTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='ks-oe-false-completion-', dir='/private/tmp')
        self.addCleanup(self.temp.cleanup)
        self.folder = Path(self.temp.name) / 'packet'
        shutil.copytree(guard.PACKET, self.folder)

    def mutate(self, name, action):
        p = self.folder / name
        raw = p.read_bytes()
        data = json.loads(gzip.decompress(raw) if name.endswith('.gz') else raw)
        action(data)
        raw = (json.dumps(data, ensure_ascii=False, indent=2) + '\n').encode()
        p.write_bytes(gzip.compress(raw, mtime=0) if name.endswith('.gz') else raw)
        manifest = json.loads((self.folder / 'manifest.json').read_text())
        for row in manifest['artifacts']:
            if row['path'] == name:
                row.update(sha256=hashlib.sha256(p.read_bytes()).hexdigest(), bytes=p.stat().st_size)
                if 'uncompressed_sha256' in row:
                    row['uncompressed_sha256'] = hashlib.sha256(raw).hexdigest()
        (self.folder / 'manifest.json').write_text(json.dumps(manifest))

    def test_real_observations_remain_inconclusive(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_rehashed_missing_calls_are_rejected(self):
        self.mutate('OE-D01-off.json.gz', lambda d: d['rows'].pop())
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_duplicate_slots_are_rejected(self):
        self.mutate('OE-D01-off.json.gz', lambda d: d['rows'].__setitem__(1, d['rows'][0]))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_wrong_arguments_are_rejected(self):
        self.mutate('OE-D01-off.json.gz', lambda d: d['rows'][0]['call']['arguments'].update(prompt='other question'))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_foreign_citation_with_valid_pack_hash_is_rejected(self):
        def action(d):
            pack = d['rows'][0]['call']['response']['structuredContent']
            pack['citations'][0]['project_id'] = 'foreign-project'
            pack['metadata'].pop('integrity_hash')
            pack['metadata']['integrity_hash'] = guard.score.sha(guard.score.canonical(pack))
        self.mutate('OE-D01-off.json.gz', action)
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_retained_source_tampering_is_rejected(self):
        blob = next((self.folder / 'source-blobs').glob('*.txt'))
        blob.write_bytes(blob.read_bytes() + b'changed source\n')
        manifest = json.loads((self.folder / 'manifest.json').read_text())
        for row in manifest['artifacts']:
            if row['path'] == blob.relative_to(self.folder).as_posix():
                row.update(sha256=hashlib.sha256(blob.read_bytes()).hexdigest(), bytes=blob.stat().st_size)
        (self.folder / 'manifest.json').write_text(json.dumps(manifest))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_nonmode_configuration_change_is_rejected(self):
        p = self.folder / 'combined.yaml'
        p.write_text(p.read_text().replace('embed_dim: 1024', 'embed_dim: 768'))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_quality_promotion_is_rejected(self):
        self.mutate('protocol.json', lambda d: d.update(quality_acceptance=True))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_fabricated_metric_is_rejected(self):
        self.mutate('summary.json', lambda d: d['arms']['off']['family_macro'].update(source_location_hit_at_10=9.0))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_gold_approval_is_rejected(self):
        self.mutate('private-location-references.json', lambda d: d.update(independent_approval=True))
        self.assertTrue(guard.check(self.folder))


if __name__ == '__main__':
    unittest.main()
