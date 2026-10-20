#!/usr/bin/env python3
"""Reject promotion of WI-C draft, shape checks or fixture delivery to acceptance."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/review-gap-v1'
spec=importlib.util.spec_from_file_location('review_gap_guard',ROOT/'scripts/check-review-gap.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class ReviewGapTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.inputs=guard.prepare.inputs(PACKET)

    def setUp(self):
        tmp=tempfile.TemporaryDirectory();self.addCleanup(tmp.cleanup)
        self.packet=Path(tmp.name)/'packet';shutil.copytree(PACKET,self.packet)

    def rehash(self):
        p=self.packet/'manifest.json';d=json.loads(p.read_text())
        d['artifacts']={p.relative_to(self.packet).as_posix():guard.sha(p.read_bytes()) for p in self.packet.rglob('*') if p.is_file() and p.name!='manifest.json'}
        p.write_text(json.dumps(d))

    def change(self,name,action):
        p=self.packet/name;d=json.loads(p.read_text());action(d)
        p.write_text(json.dumps(d,ensure_ascii=False));self.rehash()

    def reject(self,name):self.assertIn('review gap accounting: '+name,guard.check(self.packet,False))

    def case(self,action):
        self.change('private-review-gaps.json',lambda d:action(d['cases'][0]));self.reject('private-review-gaps.json')

    def contract(self,action):
        self.change('consumer-contract-review.json',action);self.reject('consumer-contract-review.json')

    def history(self,action):
        self.change('historical-evidence-scope.json',action);self.reject('historical-evidence-scope.json')

    def test_current_packet(self):self.assertEqual(guard.check(PACKET),[])
    def test_ten_drafts_not_independent_reviews(self):self.case(lambda r:r.update(approved=True,independent_family='accepted'))
    def test_same_case_context_not_new_review(self):self.case(lambda r:r['context_supplement'].update(same_case_not_new_review=False))
    def test_missing_gold_not_empty_gold(self):self.case(lambda r:r.update(required_items_gold=[]))
    def test_anchor_mapping_not_exhaustive_gold(self):self.case(lambda r:r.update(required_items_gold=r['existing_ai_anchors'],gold_exhaustive=True))
    def test_six_sections_not_complete_gold(self):self.case(lambda r:r.update(six_sections_are_complete_gold=True))
    def test_priority_not_assigned(self):self.case(lambda r:r['priority_triage_draft'].update(assigned_level='high'))
    def test_missing_priority_gold_not_inferred(self):self.case(lambda r:r.update(independent_priority_gold='high'))
    def test_required_item_questions_not_removed(self):self.case(lambda r:r.update(required_item_questions=[]))
    def test_unknowns_preserved(self):self.case(lambda r:r.update(existing_unknowns=[]))
    def test_requirements_and_impact_not_assumed(self):self.case(lambda r:r.update(requirements_and_caller_contract_receipt='reviewed'))
    def test_omissions_unknown_not_zero(self):self.case(lambda r:r.update(omission_count=0))
    def test_matching_not_frozen(self):self.case(lambda r:r.update(matching_rules='exact-name'))
    def test_current_project_receipt_not_invented(self):self.case(lambda r:r.update(actual_dual_tool_receipt='passed'))
    def test_typed_artifact_not_existing_for_each_case(self):self.case(lambda r:r.update(typed_report_artifact='fixture/deep-review.json'))
    def test_handoff_not_assumed(self):self.case(lambda r:r.update(state_handoff_receipt='passed'))
    def test_installed_role_not_assumed(self):self.case(lambda r:r.update(installed_role_receipt='passed'))
    def test_exposure_pairs_not_independence(self):
        self.change('pairwise-review.json',lambda d:d[0].update(semantic_independence='accepted'));self.reject('pairwise-review.json')
    def test_section_presence_not_omission_zero(self):self.contract(lambda d:d.update(six_sections_prove_required_item_completeness=True))
    def test_enum_not_correct_priority(self):self.contract(lambda d:d.update(priority_enum_proves_priority_correctness=True))
    def test_criteria_are_not_independent_gold(self):self.contract(lambda d:d.update(criteria_labels_are_independent_gold=True))
    def test_structural_identity_not_atomic_snapshot(self):self.contract(lambda d:d.update(structural_snapshot_atomicity_verified=True))
    def test_cfg_not_runtime_trace(self):self.contract(lambda d:d.update(cfg_is_runtime_trace=True))
    def test_module_membership_not_interprocedural(self):self.contract(lambda d:d.update(module_diagram_is_interprocedural_analysis=True))
    def test_diagram_bytes_not_rendering_quality(self):self.contract(lambda d:d.update(diagram_bytes_prove_rendering_or_quality=True))
    def test_local_hash_not_authenticated_transport(self):self.contract(lambda d:d.update(local_hash_authenticates_transport_or_reviewer=True))
    def test_registry_not_installed_hook(self):self.contract(lambda d:d.update(hook_registry_proves_installed_hook=True))
    def test_shell_edits_outside_hook(self):self.contract(lambda d:d.update(shell_edits_guarded_by_deep_review_hook=True))
    def test_missing_role_alias_not_hidden(self):self.contract(lambda d:d['role_exposure_gaps'][0].update(explicit_role_tool_present=True))
    def test_missing_alias_not_denial_proof(self):self.contract(lambda d:d['role_exposure_gaps'][0].update(runtime_denial_proven=True))
    def test_planner_contract_conflict_preserved(self):self.contract(lambda d:d['planner_contract_conflict'].update(legacy_planner_binds_report_hash=True))
    def test_planner_old_priority_cap_not_silently_changed(self):self.contract(lambda d:d['planner_contract_conflict'].update(legacy_planner_findings_cap=32))
    def test_fixture_success_preserved(self):self.history(lambda d:d['summary'].update(cks_exit=1))
    def test_fixture_not_ten_current_reviews(self):self.history(lambda d:d.update(current_project_cases_covered=10))
    def test_fixture_delivery_not_independent_quality(self):self.history(lambda d:d.update(independent_priority_or_required_item_quality='accepted'))
    def test_fixture_unpinned_scope_preserved(self):self.history(lambda d:d.update(structural_snapshot_association='verified'))
    def test_probe_semantics_not_quality_acceptance(self):
        self.change('local-contract-checks.json',lambda d:d.update(independent_acceptance='accepted'));self.reject('local-contract-checks.json')
    def test_probe_receipt_promotion_rejected(self):
        self.change('probe/result.json',lambda d:d.update(formal_acceptance='accepted'));self.assertTrue(any('contract probe accounting' in e for e in guard.check(self.packet,False)))
    def test_probe_source_binding_rejected(self):
        self.change('probe/result.json',lambda d:d['adopted_source_sha256'].update({'plugin/scripts/deep_review.py':'0'*64}));self.assertTrue(any('probe source bindings' in e for e in guard.check(self.packet,False)))
    def test_native_race_skip_rejected(self):
        p=self.packet/'native/race.jsonl';rows=[json.loads(s) for s in p.read_text().splitlines()]
        next(r for r in rows if r.get('Action')=='pass' and 'Test'in r)['Action']='skip'
        p.write_text(''.join(json.dumps(r)+'\n' for r in rows));self.rehash()
        self.assertTrue(any('native test inventory/result' in e for e in guard.check(self.packet,False)))
    def test_native_stderr_not_hidden(self):
        (self.packet/'native/normal.stderr.txt').write_text('unexpected warning');self.rehash()
        self.assertTrue(any('native stderr' in e for e in guard.check(self.packet,False)))
    def test_native_race_mode_not_assumed(self):
        self.change('native/race-receipt.json',lambda d:d.update(command=[s for s in d['command'] if s!='-race']))
        self.assertTrue(any('native mode/repetition' in e for e in guard.check(self.packet,False)))
    def test_public_gold_not_injected(self):
        self.change('public-inputs.json',lambda d:d['cases'][0].update(required_items_gold=['safe']));self.reject('public-inputs.json')
    def test_formal_execution_not_promoted(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True));self.assertIn('formal state changed',guard.check(self.packet,False))
    def test_consumer_source_tamper(self):
        p=self.packet/'consumer-source/sources/plugin/agents/planner.md.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,False)))
    def test_original_git_objects_preserved(self):
        (self.packet/'.gitattributes').write_text('consumer-source/** text\n');self.rehash()
        self.assertIn('Git object preservation attributes',guard.check(self.packet,False))
    def test_duplicate_case_input_rejected(self):
        d=copy.deepcopy(self.inputs);d['public']['cases'].append(d['public']['cases'][0])
        with self.assertRaisesRegex(ValueError,'duplicate'):guard.prepare.generate(d)
    def test_priority_input_promotion_rejected(self):
        d=copy.deepcopy(self.inputs);d['proposals']['cases'][0]['priority_proposal']['approved']=True
        with self.assertRaisesRegex(ValueError,'priority promotion'):guard.prepare.generate(d)
    def test_required_questions_input_missing_rejected(self):
        d=copy.deepcopy(self.inputs);d['proposals']['cases'][0]['required_item_questions']=[]
        with self.assertRaisesRegex(ValueError,'case obligations missing'):guard.prepare.generate(d)
    def test_historical_missing_response_rejected(self):
        d=copy.deepcopy(self.inputs);d['old_responses']=d['old_responses'][:-1]
        with self.assertRaisesRegex(ValueError,'historical MCP identities'):guard.prepare.generate(d)
    def test_count_inventory_and_null_scores(self):
        a=guard.prepare.generate(self.inputs);c=a['gap-summary.json']['counts']
        self.assertEqual((c['review_proposals'],c['module_labels'],c['project_source_files'],c['existing_ai_anchors']),(10,8,9,45))
        self.assertEqual((c['pairwise_reviews'],c['same_module_pairs'],c['same_file_pairs']),(45,2,1))
        self.assertEqual(c['missing_explicit_role_tool_entries'],4);self.assertIsNone(a['gap-summary.json']['required_omissions'])


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
