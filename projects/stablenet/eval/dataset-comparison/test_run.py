import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('dataset_comparison', Path(__file__).with_name('run.py'))
M = importlib.util.module_from_spec(spec); spec.loader.exec_module(M)


class EvidenceComparisonTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name); (self.root / 'sources/blobs').mkdir(parents=True)
        raw = b'func Example() {}\n'
        self.sha = hashlib.sha256(raw).hexdigest(); (self.root / 'sources/blobs' / self.sha).write_bytes(raw)
        self.source = {'project_id': 'project', 'snapshot_id': 's' * 64, 'source_mode': 'committed', 'source_commit': 'c' * 40}
        M.write(self.root / 'dataset-identity.json', {'source': self.source, 'dataset_id': 'd' * 64})
        M.write(self.root / 'sources/manifest.json', {'files': [{'origin_id': 'repo', 'path': 'code.go', 'sha256': self.sha}]})
        coords = {k: self.source[k] for k in ('project_id', 'snapshot_id', 'source_mode')}
        coords.update(dataset_id='d' * 64, base_commit='c' * 40)
        citation = dict(coords, file='code.go', start_line=1, end_line=1, commit_hash='c' * 40,
                        origin_id='repo', file_sha256=self.sha, content_sha256=self.sha)
        self.pack = {'format_version': 2, 'coordinates': coords, 'query': 'Example', 'citations': [citation],
                     'bodies': [{'citation': citation, 'text': raw.decode()}], 'graph_neighbors': [], 'semantic': None,
                     'evidence_state': 'complete', 'metadata': {'integrity_hash_algo': 'sha256-v2'}}
        self.stamp(self.pack)

    def stamp(self, pack):
        pack['metadata'].pop('integrity_hash', None)
        pack['metadata']['integrity_hash'] = M.digest(json.dumps(pack, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode())

    def test_real_source_coordinates_and_hashes_verify(self):
        self.assertTrue(M.verify_pack(self.pack, self.root))

    def test_body_tamper_without_restamping_is_rejected(self):
        changed = copy.deepcopy(self.pack); changed['bodies'][0]['text'] += 'changed'
        with self.assertRaisesRegex(ValueError, 'integrity'): M.verify_pack(changed, self.root)

    def test_restamped_foreign_identity_is_rejected(self):
        changed = copy.deepcopy(self.pack); changed['coordinates']['dataset_id'] = 'x' * 64; self.stamp(changed)
        with self.assertRaisesRegex(ValueError, 'foreign pack'): M.verify_pack(changed, self.root)

    def test_source_blob_tampering_is_rejected(self):
        (self.root / 'sources/blobs' / self.sha).write_bytes(b'new source\n')
        with self.assertRaisesRegex(ValueError, 'source bytes'): M.verify_pack(self.pack, self.root)

    def test_transformed_public_body_keeps_raw_source_provenance(self):
        changed = copy.deepcopy(self.pack); changed['bodies'][0]['text'] = '[sanitized]\n'; self.stamp(changed)
        self.assertTrue(M.verify_pack(changed, self.root))
        self.assertNotEqual(M.summarize_pack(changed)['bodies'], M.summarize_pack(self.pack)['bodies'])

    def test_content_comparison_retains_origin_and_span(self):
        other = copy.deepcopy(self.pack['citations'][0]); other['dataset_id'] = 'other'
        self.assertEqual(M.content_key(other), M.content_key(self.pack['citations'][0]))
        other['origin_id'] = 'knowledge:other.pack'
        self.assertNotEqual(M.content_key(other), M.content_key(self.pack['citations'][0]))

    def test_only_exact_file_hash_and_signature_line_match_anchor(self):
        question = {'expected_anchor': {'file': 'code.go', 'symbol': 'Example', 'file_sha256': self.sha, 'line': 1}}
        self.assertEqual(M.anchor_score(question, self.pack)['hit_at_10'], 1)
        question['expected_anchor']['line'] = 2
        self.assertEqual(M.anchor_score(question, self.pack)['hit_at_10'], 0)


if __name__ == '__main__': unittest.main()
