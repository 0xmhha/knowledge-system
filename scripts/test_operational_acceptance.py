#!/usr/bin/env python3
"""Regression controls for false completion and holdout leakage in the ledger."""
import copy
import importlib.util
import json
import hashlib
import shutil
import tempfile
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

    def test_current_readiness_cannot_be_promoted(self):
        for value in (self.protocol,self.ledger):
            value['current_readiness_preparation']['kind']='operational_acceptance'
        self.assertTrue(self.errors())

    def test_WI1_DEV_draft_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi1_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI1_consumer_mapping_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi1_consumer_matching_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI2_AI_draft_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi2_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI2_consumer_matching_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi2_consumer_matching_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI3_AI_draft_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi3_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI3_actual_partial_matching_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi3_consumer_matching_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI4_conditional_outcomes_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi4_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_shared_ticket_AI_membership_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['ticket_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI6_bounded_relevance_cannot_be_independent_gold(self):
        for value in (self.protocol, self.ledger):
            value['wi6_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI2_compiler_contract_audit_cannot_be_formal_acceptance(self):
        for value in (self.protocol,self.ledger):
            value['wi2_contract_audit']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI3_fresh_native_reason_fix_cannot_be_formal_acceptance(self):
        for value in (self.protocol,self.ledger):
            value['wi3_reason_fix']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WI123_direct_diagnostic_cannot_be_formal_acceptance(self):
        for value in (self.protocol,self.ledger):
            value['wi123_debt_diagnostic']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WIC_source_vocabulary_cannot_be_independent_required_gold(self):
        for value in (self.protocol,self.ledger):
            value['wic_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_WISP_manual_metadata_cannot_be_independent_security_gold(self):
        for value in (self.protocol, self.ledger):
            value['wisp_dev_gold_preparation']['status'] = 'independent_approved'
        self.assertTrue(self.errors())

    def test_real_name_collection_is_not_an_acceptance_requirement(self):
        self.protocol['reviewer_identity_policy']['real_name_required'] = True
        self.assertTrue(self.errors())

    def test_previous_authoring_source_must_stay_pinned(self):
        self.protocol['independent_freeze_preparation']['historical_source_revision'] = 'HEAD'
        self.assertTrue(self.errors())

    def test_current_readiness_reference_must_match(self):
        self.protocol.pop('current_readiness_preparation')
        self.assertTrue(self.errors())

    def test_historical_readiness_cannot_be_current_acceptance(self):
        for value in (self.protocol, self.ledger):
            value['historical_readiness_preparation']['status'] = 'current_source_validated'
        self.assertTrue(self.errors())

    def test_historical_readiness_reference_must_be_preserved(self):
        self.protocol.pop('historical_readiness_preparation')
        self.assertTrue(self.errors())

    def test_unrelated_manifest_cannot_replace_historical_readiness(self):
        for value in (self.protocol, self.ledger):
            value['historical_readiness_preparation'].update(
                manifest=value['current_readiness_preparation']['manifest'],
                manifest_sha256=value['current_readiness_preparation']['manifest_sha256'])
        self.assertTrue(self.errors())

    def test_current_readiness_hash_is_bound(self):
        for value in (self.protocol,self.ledger):
            value['current_readiness_preparation']['manifest_sha256']='0'*64
        self.assertTrue(self.errors())

    def test_current_documents_are_consistent(self):
        self.assertEqual(guard.check(guard.ROOT, self.actual_protocol, self.actual_ledger)[0], [])

    def test_preparation_draft_is_consistent(self):
        self.assertEqual(self.errors(), [])

    def test_draft_cannot_start_execution(self):
        self.protocol["execution_allowed"] = True
        self.assertTrue(self.errors())

    def test_draft_does_not_satisfy_frozen_gate(self):
        self.assertTrue(self.errors(True))

    def test_local_host_adoption_cannot_expand_to_production(self):
        if "host_connection_verification" not in self.protocol:
            self.skipTest("local host preparation not yet recorded")
        self.protocol["host_connection_verification"]["scope"] = "production"
        self.ledger["host_connection_verification"]["scope"] = "production"
        self.assertTrue(self.errors())

    def test_local_host_adopted_source_cannot_change_without_evidence(self):
        if "host_connection_verification" not in self.protocol:
            self.skipTest("local host preparation not yet recorded")
        for rows in (self.protocol["readiness_candidate_patches"], self.ledger["readiness_fixes"]):
            rows[0]["adopted_commit"] = "0" * 40
        self.assertTrue(self.errors())

    def test_policy_security_source_preparation_reference_cannot_disappear(self):
        if "policy_security_input_preparation" not in self.protocol:
            self.skipTest("source packet not recorded")
        self.ledger.pop("policy_security_input_preparation")
        self.assertTrue(self.errors())

    def test_hash_tampering_is_rejected(self):
        self.protocol["source_inventory_sha256"] = "0" * 64
        self.assertTrue(self.errors())

    def test_policy_reference_diagnosis_cannot_claim_approved_scope(self):
        if 'policy_reference_diagnosis' not in self.protocol:
            self.skipTest('diagnosis not yet recorded')
        for value in (self.protocol, self.ledger):
            value['policy_reference_diagnosis']['kind'] = 'approved_operational_acceptance'
        self.assertTrue(self.errors())

    def test_policy_reference_diagnosis_reference_cannot_disappear(self):
        if 'policy_reference_diagnosis' not in self.protocol:
            self.skipTest('diagnosis not yet recorded')
        self.ledger.pop('policy_reference_diagnosis')
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

    def test_pseudonymous_role_receipt_needs_no_personal_name(self):
        digest = guard.protocol_hash(self.protocol)
        receipt = {'kind':'human','reviewer_id':'acceptance-reviewer','decision':'approved','protocol_sha256':digest}
        self.assertTrue(guard.human_receipt(receipt,digest))

    def test_role_alias_cannot_approve_or_bind_itself(self):
        digest = guard.protocol_hash(self.protocol)
        for fields in ({'decision':'pending'},{'kind':'AI'},{'protocol_sha256':'0'*64}):
            receipt = dict(kind='human',reviewer_id='acceptance-reviewer',decision='approved',protocol_sha256=digest)
            receipt.update(fields);self.assertFalse(guard.human_receipt(receipt,digest))

    def test_positive_verdict_requires_hash_bound_evidence(self):
        self.ledger["items"][0]["operational_gate"] = "pass"
        self.assertTrue(any("hash-bound evidence" in e for e in self.errors()))

    def test_unknown_verdict_cannot_bypass_guard(self):
        self.ledger["items"][0]["quality_gate"] = "looks_good"
        self.assertTrue(any("unknown item verdict" in e for e in self.errors()))

    def test_candidate_hash_tampering_is_rejected(self):
        self.protocol["input_candidate_pool"]["sha256"] = "0" * 64
        self.assertTrue(any("preparation reference" in e for e in self.errors()))

    def test_host_capture_hash_tampering_is_rejected(self):
        self.protocol["host_preflight"]["sha256"] = "0" * 64
        self.assertTrue(any("preparation reference" in e for e in self.errors()))

    def test_host_recipe_cannot_skip_hooks_or_permissions(self):
        original = copy.deepcopy(self.protocol)
        for flag in ("--bare", "--safe-mode", "--dangerously-skip-permissions", "bypassPermissions"):
            with self.subTest(flag=flag):
                self.protocol = copy.deepcopy(original)
                self.protocol["runtime_recipes"][0]["argv_template"].append(flag)
                self.assertTrue(any("bypasses" in e for e in self.errors()))

    def test_recipe_run_cannot_precede_freeze(self):
        self.protocol["runtime_recipes"][0]["execution_status"] = "pass"
        self.assertTrue(any("recipe execution" in e for e in self.errors()))

    def mutated_packet_errors(self, kind, mutate):
        reference = copy.deepcopy(self.protocol["review_draft_packet"])
        folder = Path("docs/spec-driven/operational-preparation/review-drafts")
        with tempfile.TemporaryDirectory(dir="/private/tmp") as tmp:
            root = Path(tmp)
            shutil.copytree(guard.ROOT / folder, root / folder)
            path_field = "public_path" if kind == "public" else "gold_draft_path"
            hash_field = "public_sha256" if kind == "public" else "gold_draft_sha256"
            path = root / reference[path_field]
            data = json.loads(path.read_text())
            mutate(data)
            path.write_text(json.dumps(data))
            reference[hash_field] = hashlib.sha256(path.read_bytes()).hexdigest()
            return guard.check_review_draft_packet(root, reference)

    def test_gold_labels_cannot_leak_into_public_input(self):
        errors = self.mutated_packet_errors("public", lambda d: d["cases"][0].update(expected_answer="fake answer"))
        self.assertTrue(any("contains gold" in e for e in errors))

    def test_ai_rubric_cannot_be_marked_independently_approved(self):
        errors = self.mutated_packet_errors("private", lambda d: d["review_drafts"][0].update(approved=True))
        self.assertTrue(any("independently approved" in e for e in errors))

    def test_non_exhaustive_edges_cannot_enable_recall_scoring(self):
        errors = self.mutated_packet_errors("private", lambda d: d["value_control_anchor_drafts"][0].update(precision_recall_scoring_allowed=True))
        self.assertTrue(any("scorable gold" in e for e in errors))

    def test_review_anchor_tampering_is_rejected(self):
        def mutate(d):
            d["review_drafts"][0]["required_rubric_anchors_draft"][0]["span_sha256"] = "0" * 64
        errors = self.mutated_packet_errors("private", mutate)
        self.assertTrue(any("anchor bytes/hash" in e for e in errors))

    def test_readiness_manifest_hash_tampering_is_rejected(self):
        self.protocol["readiness_candidate_patches"][0]["manifest_sha256"] = "0" * 64
        self.ledger["readiness_fixes"] = copy.deepcopy(self.protocol["readiness_candidate_patches"])
        self.assertTrue(any("patch manifest" in e for e in self.errors()))

    def test_controlled_source_patch_is_not_operational_acceptance(self):
        self.protocol["readiness_candidate_patches"][0]["operational_gate"] = "pass"
        self.ledger["readiness_fixes"] = copy.deepcopy(self.protocol["readiness_candidate_patches"])
        self.assertTrue(any("readiness patch promoted" in e for e in self.errors()))


if __name__ == "__main__":
    unittest.main()
