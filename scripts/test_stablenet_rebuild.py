"""Protect real paired corpus evidence from provenance and completion promotion."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('rebuild', Path(__file__).with_name('check-stablenet-rebuild.py'))
rebuild = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rebuild)


class RebuildEvidenceTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.packet = Path(temp.name) / 'packet'
        shutil.copytree(rebuild.ROOT / rebuild.REL, self.packet)

    def seal(self):
        path = self.packet / 'manifest.json'
        manifest = json.loads(path.read_text())
        manifest['artifacts'] = {str(p.relative_to(self.packet)): rebuild.sha(p.read_bytes()) for p in self.packet.rglob('*') if p.is_file() and p != path}
        path.write_text(json.dumps(manifest))

    def change(self, name, operation):
        path = self.packet / name
        obj = json.loads(path.read_text())
        operation(obj)
        path.write_text(json.dumps(obj, ensure_ascii=False))
        self.seal()

    def response(self, case, label, operation):
        path = self.packet / 'sessions' / case / 'messages.json'
        rows = json.loads(path.read_text())
        row = next(r for r in rows if r['label'] == label)
        operation(row['response'])
        result = row['response'].get('result', {})
        for content in result.get('content', []):
            if content.get('type') == 'text':
                try:
                    json.loads(content['text'])
                    content['text'] = json.dumps(result['structuredContent'])
                except (ValueError, KeyError):
                    pass
        path.write_text(json.dumps(rows))
        raw = path.with_name('stdout.jsonl')
        wire = [json.loads(line) for line in raw.read_text().splitlines()]
        raw.write_text(''.join(json.dumps(row['response'] if r.get('id') == row['response']['id'] else r) + '\n' for r in wire))
        self.seal()

    def rejected(self):
        self.assertTrue(rebuild.check(self.packet))

    def test_real_rebuild_and_all_failed_old_calls_remain_valid_evidence(self):
        self.assertEqual([], rebuild.check(self.packet))

    def test_cannot_promote_formal_acceptance_from_local_success(self):
        self.change('execution.json', lambda d: d.update(formal_completed=9))
        self.rejected()

    def test_cannot_drop_old_failed_input(self):
        self.change('sessions/paired-old/messages.json', lambda d: d.__setitem__(slice(None), [r for r in d if r['label'] != 'Q1-vector-all']))
        self.rejected()

    def test_new_primary_input_cannot_change_from_old_arm(self):
        self.change('sessions/paired-new/messages.json', lambda rows: next(r for r in rows if r['label'] == 'Q1-vector-all')['request']['params']['arguments'].update(query='easier replacement'))
        self.rejected()

    def test_external_origin_must_keep_its_namespace(self):
        path = self.packet / 'chunk-source-ledger.json.gz'
        ledger = json.loads(gzip.decompress(path.read_bytes()))
        row = next(r for r in ledger if r.get('origin_id') == 'docs:corpus.0')
        row['file'] = row['path']
        path.write_bytes(gzip.compress(json.dumps(ledger).encode(), mtime=0))
        self.seal()
        self.rejected()

    def test_convention_flag_cannot_disappear(self):
        self.response('paired-new', 'convention-contract', lambda r: r['result']['structuredContent']['hits'][0].pop('non_citable_metadata'))
        self.rejected()

    def test_convention_cannot_become_legacy_source_citation(self):
        self.response('paired-new', 'legacy-knowledge-separation', lambda r: r['result']['structuredContent']['citations'][0].update(file='<convention>'))
        self.rejected()

    def test_foreign_v2_project_is_rejected_even_with_recomputed_integrity(self):
        def alter(response):
            pack = response['result']['structuredContent']
            pack['coordinates']['project_id'] = 'foreign'
            self.restamp(pack)
        self.response('paired-new', 'Q1-pack-v2', alter)
        self.rejected()

    @staticmethod
    def restamp(pack):
        pack['metadata'].pop('integrity_hash', None)
        pack['metadata']['integrity_hash'] = hashlib.sha256(json.dumps(pack, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()).hexdigest()

    def test_forged_body_is_rejected_even_with_valid_pack_integrity(self):
        def alter(response):
            pack = response['result']['structuredContent']
            pack['bodies'][0]['text'] = 'forged source'
            self.restamp(pack)
        self.response('paired-new', 'Q1-pack-v2', alter)
        self.rejected()

    def test_wrong_pin_requires_absent_analysis(self):
        self.response('boundaries-new', 'wrong-commit', lambda r: r['result']['structuredContent'].update(analysis={}))
        self.rejected()

    def test_held_candidate_must_not_promote_current(self):
        self.change('build-exit.json', lambda d: d.update(owned_current_exists=True))
        self.rejected()

    def test_live_owned_identity_cannot_count_as_cleaned_up(self):
        self.change('process-audit.json', lambda d: d['probes'][0].update(identity_gone=False))
        self.rejected()


if __name__ == '__main__':
    unittest.main()
