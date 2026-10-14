#!/usr/bin/env python3
"""Regression controls for false completion and holdout leakage in the ledger."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("acceptance_check", Path(__file__).with_name("check-operational-acceptance.py"))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class PreparationGuardTests(unittest.TestCase):
    def setUp(self):
        prefix = guard.ROOT / guard.PREFIX
        self.protocol = json.loads(Path(str(prefix) + "PROTOCOL.json").read_text())
        self.ledger = json.loads(Path(str(prefix) + "PROGRESS.json").read_text())
        self.actual_protocol, self.actual_ledger = copy.deepcopy(self.protocol), copy.deepcopy(self.ledger)
        self.protocol.update(freeze=None, execution_allowed=False, blocked_by=sorted(guard.FREEZE_IDS))
        self.ledger.update(protocol_freeze=None, execution_status="not_started", execution_receipts=[])
        for row in self.protocol["freeze_requirements"]:
            row.update(decision_status="pending", receipt=None)
        for row in self.protocol["input_groups"]:
            row.update(cases=[], review_receipt=None)
        for row in self.ledger["phases"]:
            row["status"] = "preparing" if row["id"] == "0" else "not_started"
        for row in self.ledger["items"]:
            row.update(operational_gate="not_run", quality_gate="not_run", independent_acceptance="not_recorded", evidence=None, review_receipt=None)

    def errors(self, frozen=False):
        return guard.check(guard.ROOT, self.protocol, self.ledger, frozen)[0]

    def test_current_documents_are_consistent(self):
        self.assertEqual(guard.check(guard.ROOT, self.actual_protocol, self.actual_ledger)[0], [])

    def test_preparation_draft_is_consistent(self):
        self.assertEqual(self.errors(), [])

    def test_draft_cannot_start_execution(self):
        self.protocol["execution_allowed"] = True
        self.assertTrue(self.errors())

    def test_draft_does_not_satisfy_frozen_gate(self):
        self.assertTrue(self.errors(True))

    def test_hash_tampering_is_rejected(self):
        self.protocol["source_inventory_sha256"] = "0" * 64
        self.assertTrue(self.errors())

    def test_missing_requirement_cannot_disappear(self):
        self.protocol["freeze_requirements"].pop()
        self.assertTrue(self.errors())

    def test_fake_freeze_is_rejected(self):
        self.protocol["freeze"] = {"protocol_sha256": guard.protocol_hash(self.protocol), "frozen_at": "test"}
        self.ledger["protocol_freeze"] = self.protocol["freeze"]
        self.assertTrue(self.errors())

    def test_phase_zero_cannot_be_marked_complete(self):
        self.ledger["phases"][0]["status"] = "complete"
        self.assertTrue(self.errors())

    def test_runtime_cannot_be_marked_complete(self):
        self.ledger["phases"][1]["status"] = "complete"
        self.assertTrue(self.errors())

    def test_execution_receipt_requires_freeze(self):
        self.ledger["execution_receipts"] = [{"run_id": "fake"}]
        self.assertTrue(self.errors())

    def test_each_gate_cannot_advance_in_draft(self):
        original = copy.deepcopy(self.ledger)
        for field, value in (("operational_gate", "pass"), ("quality_gate", "scoped_validated"),
                             ("independent_acceptance", "scope_accepted")):
            with self.subTest(field=field):
                self.ledger = copy.deepcopy(original)
                self.ledger["items"][0][field] = value
                self.assertTrue(self.errors())

    def test_same_family_cannot_cross_holdout(self):
        self.protocol["input_groups"][0]["cases"] = [
            {"case_id": "a", "family_id": "same", "split": "DEV"},
            {"case_id": "b", "family_id": "same", "split": "FINAL"},
        ]
        self.assertTrue(any("overlap" in e for e in self.errors()))

    def test_family_cannot_leak_across_work_items(self):
        for group, split in zip(self.protocol["input_groups"][:2], ["DEV", "FINAL"]):
            group["cases"] = [{"case_id": split, "family_id": "same", "split": split}]
        self.assertTrue(any("across work items" in e for e in self.errors()))

    def test_old_regression_cannot_become_gold(self):
        self.protocol["input_groups"][0]["cases"] = [
            {"case_id": "a", "family_id": "same", "split": "FINAL", "origin": "regression"}]
        self.assertTrue(any("regression input" in e for e in self.errors()))

    def test_receipt_hash_exclusions_are_narrow(self):
        baseline = guard.protocol_hash(self.protocol)
        self.protocol["freeze"] = {"frozen_at": "test"}
        self.assertEqual(baseline, guard.protocol_hash(self.protocol))
        self.protocol["scope_proposal"]["ontology_enabled"] = True
        self.assertNotEqual(baseline, guard.protocol_hash(self.protocol))

    def test_named_model_is_not_a_human_receipt(self):
        digest = guard.protocol_hash(self.protocol)
        receipt = {"kind": "ai", "name": "test model", "decision": "approved", "protocol_sha256": digest}
        self.assertFalse(guard.human_receipt(receipt, digest))

    def test_positive_verdict_requires_hash_bound_evidence(self):
        self.ledger["items"][0]["operational_gate"] = "pass"
        self.assertTrue(any("hash-bound evidence" in e for e in self.errors()))

    def test_unknown_verdict_cannot_bypass_guard(self):
        self.ledger["items"][0]["quality_gate"] = "looks_good"
        self.assertTrue(any("unknown item verdict" in e for e in self.errors()))


if __name__ == "__main__":
    unittest.main()
