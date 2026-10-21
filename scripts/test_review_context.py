#!/usr/bin/env python3
"""Mutation tests for immutable context evidence and interpretation boundaries."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/context-v1'
spec = importlib.util.spec_from_file_location('context_guard', ROOT / 'scripts/check-review-context.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class ContextGuardTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        path = self.packet / 'manifest.json'
        m = json.loads(path.read_text())
        m['artifacts'] = {p.relative_to(self.packet).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
                          for p in self.packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        path.write_text(json.dumps(m))

    def change(self, update):
        p = self.packet / 'private-context-review.json'
        d = json.loads(p.read_text())
        update(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.rehash()

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_changed_blob_even_with_rebound_manifest_and_metadata(self):
        p = self.packet / 'git-proof.json'
        proof = json.loads(p.read_text())
        record = proof['files']['rpc/http.go']
        source = self.packet / record['artifact']
        raw = source.read_bytes() + b'// modified\n'
        source.write_bytes(raw)
        record.update(git_blob=guard.oid('blob', raw), sha256=guard.sha(raw))
        p.write_text(json.dumps(proof))
        self.rehash()
        self.rejected()

    def test_changed_commit_object(self):
        proof = json.loads((self.packet / 'git-proof.json').read_text())
        path = self.packet / proof['objects']['commit:' + proof['commit']]
        path.write_bytes(path.read_bytes() + b'changed\n')
        self.rehash()
        self.rejected()

    def test_changed_tree_object(self):
        proof = json.loads((self.packet / 'git-proof.json').read_text())
        path = self.packet / next(v for k, v in proof['objects'].items() if k.startswith('tree:'))
        path.write_bytes(path.read_bytes() + b'changed')
        self.rehash()
        self.rejected()

    def test_changed_anchor_line(self):
        self.change(lambda d: d['reviews'][0]['anchors'][0].update(start_line=1))
        self.rejected()

    def test_anchor_outside_file(self):
        self.change(lambda d: d['reviews'][0]['anchors'][0].update(end_line=1000000))
        self.rejected()

    def test_verified_defect_claim(self):
        self.change(lambda d: d['reviews'][3].update(verified_defect=True))
        self.rejected()

    def test_gold_promotion(self):
        self.change(lambda d: d['reviews'][0].update(approved=True))
        self.rejected()

    def test_expected_answer_promotion(self):
        self.change(lambda d: d['reviews'][0]['followup_candidate'].update(independent_expected_result='pass'))
        self.rejected()

    def test_ticket_promotion(self):
        self.change(lambda d: d['reviews'][3]['followup_candidate'].update(tickets_promoted_to_protocol=1))
        self.rejected()

    def test_missing_limits(self):
        self.change(lambda d: d['reviews'][0].update(remaining_unknowns=[]))
        self.rejected()

    def test_public_proposal_includes_private_fact(self):
        self.change(lambda d: d['reviews'][0]['followup_candidate'].update(public_input_proposal=d['reviews'][0]['anchors'][0]['source_fact_draft']))
        self.rejected()


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
