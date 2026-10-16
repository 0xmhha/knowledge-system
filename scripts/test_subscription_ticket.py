#!/usr/bin/env python3
"""Reject source tampering and promotion of AI ticket drafts to independent evidence."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/subscription-ticket-v1'
spec = importlib.util.spec_from_file_location('ticket_guard', ROOT / 'scripts/check-subscription-ticket.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class TicketTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        p = self.packet / 'manifest.json'
        m = json.loads(p.read_text())
        m['artifacts'] = {q.name: hashlib.sha256(q.read_bytes()).hexdigest() for q in self.packet.iterdir() if q.name != 'manifest.json'}
        p.write_text(json.dumps(m))

    def change(self, name, update):
        p = self.packet / name
        d = json.loads(p.read_text())
        update(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.rehash()

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_original_source_modified_even_after_rehash(self):
        p = self.packet / 'original.go.txt'
        p.write_bytes(p.read_bytes() + b'// modified\n')
        self.rehash()
        self.rejected()

    def test_public_oracle_leak(self):
        self.change('public-ticket.json', lambda d: d.update(expected_result='pass'))
        self.rejected()

    def test_final_assignment(self):
        self.change('public-ticket.json', lambda d: d.update(split='FINAL'))
        self.rejected()

    def test_ai_approval(self):
        self.change('private-review.json', lambda d: d.update(approved=True))
        self.rejected()

    def test_independent_oracle_claim(self):
        self.change('private-review.json', lambda d: d.update(independently_derived_oracle='AI'))
        self.rejected()

    def test_independent_candidate_set_claim(self):
        self.change('private-review.json', lambda d: d.update(independent_acceptable_candidate_set=['CAND-A']))
        self.rejected()

    def test_family_inflation(self):
        self.change('private-review.json', lambda d: d.update(independent_family_count=5))
        self.rejected()

    def test_project_defect_claim(self):
        self.change('private-review.json', lambda d: d.update(verified_project_defect=True))
        self.rejected()

    def test_execution_claim(self):
        self.change('private-review.json', lambda d: d.update(local_fixture_executions=25))
        self.rejected()

    def test_early_return_expectation_rejected(self):
        self.change('private-review.json', lambda d: d['expected_draft_matrix']['MUT-EARLY'].update(normal_delivery_then_unsubscribe=True))
        self.rejected()

    def test_formal_state_tampering(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected()


if __name__ == '__main__':
    unittest.main()
