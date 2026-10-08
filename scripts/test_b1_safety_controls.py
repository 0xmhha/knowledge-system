"""Safety audit failure oracles over retained synthetic controls, not official gold."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location('safety_controls', Path(__file__).with_name('b1-audit-safety-controls.py'))
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)
ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / 'system/eval/b0-knowledge-system'
PREP = DATA / 'safety-audit-preparation-m2max-2026-10-04'


class SafetyControlTest(unittest.TestCase):
    def setUp(self):
        self.folder = DATA / 'matrix-capture-m2max-2026-10-04/real'
        self.source = DATA / 'b1-pack-matrix-m2max-2026-10-04/real/source'
        self.report = json.loads((self.folder / 'report.json').read_bytes())
        self.requests = json.loads((self.folder / 'requests.json').read_bytes())
        self.rows = [json.loads(line) for line in (self.folder / 'rows.jsonl').read_bytes().splitlines()]
        self.controls = json.loads((PREP / 'matrix-capture-real-controls.json').read_bytes())
        self.contract = json.loads((PREP / 'matrix-capture-real-go-contract.json').read_bytes())

    def result(self):
        return M.audit(self.report, self.requests, self.rows, self.controls, self.source, self.contract)

    def mutated(self, qid, arm='baseline_on'):
        row = next(r for r in self.rows if r['request_id'] == qid and r['arm'] == arm and r['phase'] == 'retrieval')
        return row, row['call']['response']['structuredContent']

    def seal(self, pack):
        pack['metadata'].pop('integrity_hash')
        pack['metadata']['integrity_hash'] = M.BASE.sha(M.canonical(pack).encode())

    def issue(self, result, text):
        return any(text in issue for d in result['diagnostics'] for issue in d['issues'])

    def test_all_six_declared_states_nested_references_and_base_preservation(self):
        result = self.result()
        self.assertEqual(result['status'], 'control_audit_pass')
        self.assertEqual(result['observed_rows'], 288)
        self.assertEqual(result['nested_semantic_citation_references_checked'], 1320)
        self.assertEqual(len(result['paired_preservation_comparisons']), 144)
        self.assertEqual(result['pack_on_states'], {'complete':24,'unknown':48,'stale':24,'restricted':24,'conflict':24})
        self.assertIsNone(result['quality_metrics'])
        self.assertIsNone(result['human_verdicts'])
        self.assertEqual(result['official_B1_05_verdict'], 'pending')

    def test_nested_foreign_reference_is_rejected_even_with_valid_outer_integrity(self):
        _, pack = self.mutated('alpha')
        pack['semantic']['coding_context']['evidence'][0]['project_id'] = 'different-project'
        self.seal(pack)
        self.assertTrue(self.issue(self.result(), 'nested_reference_not_in_verified_registry'))

    def test_forbidden_authored_payload_in_any_sdk_field_is_detected(self):
        row, _ = self.mutated('restricted')
        row['call']['response']['content'].append({'type':'text','text':self.controls['forbidden_payload_markers'][2]})
        self.assertTrue(self.issue(self.result(), 'forbidden_synthetic_payload_marker'))

    def test_proposed_expired_and_restricted_cannot_be_promoted_to_current(self):
        for qid in ['proposed','expired','restricted']:
            with self.subTest(qid=qid):
                original = copy.deepcopy(self.rows)
                _, pack = self.mutated(qid)
                pack['semantic']['knowledge_context']['state'] = 'complete'
                pack['semantic']['coding_context']['required_behavior'] = [{'id':qid,'state':'current','citation':pack['citations'][0]}]
                self.seal(pack)
                result = self.result()
                self.assertTrue(self.issue(result, 'unexpected_policy_state'))
                self.assertTrue(self.issue(result, 'incorrect_policy_to_required_behavior_projection'))
                self.rows = original

    def test_conflict_must_not_be_projected_into_required_behavior(self):
        _, pack = self.mutated('conflict')
        pack['semantic']['coding_context']['required_behavior'] = [{'id':'conflict-a','state':'current','citation':pack['citations'][0]}]
        pack['semantic']['knowledge_context']['conflicts'][0]['right_id'] = 'public'
        self.seal(pack)
        result = self.result()
        self.assertTrue(self.issue(result, 'incorrect_policy_to_required_behavior_projection'))
        self.assertTrue(self.issue(result, 'conflict_endpoints_or_reason_differ'))

    def test_pack_off_overlay_and_invented_implementation_are_detected(self):
        _, on = self.mutated('alpha')
        _, off = self.mutated('alpha','baseline_off')
        off['semantic'] = copy.deepcopy(on['semantic']); self.seal(off)
        on['semantic']['coding_context']['implemented_behavior'] = [{'id':'invented','state':'reviewed_trace','citation':on['citations'][0]}]
        self.seal(on)
        result = self.result()
        self.assertTrue(self.issue(result, 'pack_off_exposes_knowledge_overlay'))
        self.assertTrue(self.issue(result, 'control_invented_implementation'))

    def test_dropped_base_body_and_changed_mode_candidates_are_reported(self):
        _, pack = self.mutated('alpha','combined_on')
        pack['bodies'].pop(0); self.seal(pack)
        result = self.result()
        self.assertEqual(result['status'], 'control_audit_failed_or_incomplete')
        self.assertTrue(any('pack_on_dropped_base_bodies' in d['issues'] for d in result['paired_preservation_comparisons']))

    def test_public_verifier_failure_and_cross_row_binding_cannot_disappear(self):
        self.contract['records'][0]['status'] = 'invalid_contract'; self.contract['failed'] = 1
        result = self.result()
        self.assertTrue(self.issue(result, 'public_v2_contract_invalid_or_capture_error'))
        self.assertEqual(result['public_contract_failed_records'], 1)
        self.contract['records'][0]['measurement_id'] = 'different-query'
        with self.assertRaisesRegex(ValueError, 'bound'):
            self.result()

    def test_wrong_scope_lock_and_removed_uncertainty_are_detected(self):
        _, pack = self.mutated('wrong-scope')
        pack['semantic']['knowledge_context']['lock_digest'] = '0'*64
        pack['semantic']['coding_context']['unknowns'] = []
        self.seal(pack)
        result = self.result()
        self.assertTrue(self.issue(result, 'knowledge_lock_differs'))
        self.assertTrue(self.issue(result, 'missing_implementation_uncertainty'))

    def test_foreign_dataset_or_source_controls_cannot_be_substituted(self):
        self.controls['dataset_id'] = 'foreign'
        with self.assertRaisesRegex(ValueError, 'coordinates'):
            self.result()
        self.controls['dataset_id'] = self.report['dataset_identity']['dataset_id']
        self.controls['source_sha256']['main.go'] = '0'*64
        with self.assertRaisesRegex(ValueError, 'source bytes'):
            self.result()


if __name__ == '__main__':
    unittest.main()
