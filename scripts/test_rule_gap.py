#!/usr/bin/env python3
"""Reject WI-2 denominator, gold, safety and actual-path evidence promotion."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/rule-gap-v1'
spec=importlib.util.spec_from_file_location('rule_gap_guard',ROOT/'scripts/check-rule-gap.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class RuleGapTests(unittest.TestCase):
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

    def reject(self,message):self.assertIn(message,guard.check(self.packet,check_references=False))

    def sample(self,action):
        self.change('private-sample-gaps.json',action);self.reject('rule gap accounting: private-sample-gaps.json')

    def path(self,action):
        self.change('mcp-path-review.json',action);self.reject('rule gap accounting: mcp-path-review.json')

    def history(self,action):
        self.change('historical-evidence-scope.json',action);self.reject('rule gap accounting: historical-evidence-scope.json')

    def test_real_packet(self):self.assertEqual(guard.check(PACKET),[])

    def test_controlled_drafts_not_independent_cases(self):
        self.sample(lambda d:d['exposure_review'].update(approved_independent_units=40))

    def test_no_exact_match_does_not_prove_fresh_holdout(self):
        self.sample(lambda d:d['exposure_review'].update(fresh_holdout_claimed=True))

    def test_mechanism_label_pairs_not_independent_families(self):
        self.sample(lambda d:d['exposure_review']['same_mechanism_label_pairs'][0].update(independence='approved'))

    def test_ai_agreement_not_independent_label(self):
        self.sample(lambda d:d['controlled'][0].update(independent_label='positive'))

    def test_missing_gold_not_empty_negative_gold(self):
        self.sample(lambda d:d['project'][0].update(exhaustive_candidate_gold=[]))

    def test_empty_candidate_not_safe(self):
        self.sample(lambda d:d['project'][0].update(safety_verdict='safe'))

    def test_closure_excluded_not_true_negative(self):
        self.sample(lambda d:next(r for r in d['project'] if r['call_id']=='CALL-017').update(independent_label='negative'))

    def test_controlled_positives_do_not_fill_project_gold(self):
        self.sample(lambda d:d['per_rule'][0].update(approved_independent_positive=10,project_positive_sufficiency='established'))

    def test_numeric_conversion_sufficiency_not_invented(self):
        self.sample(lambda d:d['per_rule'][1].update(project_calls_with_candidates=10))

    def test_partial_types_not_complete(self):
        self.sample(lambda d:d['controlled'][0].update(base_status='complete'))

    def test_sanitizer_and_external_unknown_preserved(self):
        self.sample(lambda d:d['project'][0]['syntax_hints'].update(sanitizer_effects='proven'))

    def test_candidate_score_not_ai_agreement(self):
        self.sample(lambda d:d['per_rule'][0].update(precision=1.0,recall=1.0))

    def test_matching_not_frozen_by_preparation(self):
        self.sample(lambda d:d.update(canonical_matching_rules='candidate ID equality'))

    def test_caller_references_not_project_denominator(self):
        self.change('gap-summary.json',lambda d:d['counts'].update(project_call_records=82))
        self.reject('rule gap accounting: gap-summary.json')

    def test_manual_annotations_not_rule_samples(self):
        self.change('gap-summary.json',lambda d:d['counts'].update(controlled_drafts=52))
        self.reject('rule gap accounting: gap-summary.json')

    def test_literal_command_not_attacker_control_receipt(self):
        self.change('command-contract-review.json',lambda d:d.update(attacker_control_receipt='trusted literals'))
        self.reject('rule gap accounting: command-contract-review.json')

    def test_syntax_caller_search_not_resolved_graph(self):
        self.change('command-contract-review.json',lambda d:d.update(resolved_call_graph_receipt='complete'))
        self.reject('rule gap accounting: command-contract-review.json')

    def test_command_execution_not_invented(self):
        self.change('command-contract-review.json',lambda d:d.update(actual_command_execution_receipt='passed'))
        self.reject('rule gap accounting: command-contract-review.json')

    def test_missing_direct_tool_not_silently_repaired(self):
        self.path(lambda d:d['role_exposure_finding'].update(explicit_analyzer_tool_present=True))

    def test_missing_explicit_tool_not_runtime_denial_proof(self):
        self.path(lambda d:d['role_exposure_finding'].update(runtime_call_denial_proven=True))

    def test_schema_parity_not_installed_role_success(self):
        self.path(lambda d:d['role_exposure_finding'].update(actual_installed_role_receipt='passed'))

    def test_local_probe_not_retained_project_mcp(self):
        self.path(lambda d:d.update(project_mcp_roundtrip_receipt='passed'))

    def test_optional_pin_contract_question_preserved(self):
        self.path(lambda d:next(r for r in d['contracts'] if r['id']=='MCP-04').update(gap='pins always required by API'))

    def test_historical_mcp_success_not_erased(self):
        self.history(lambda d:d.update(ckg_request_matrix=[],cks_request_matrix=[]))

    def test_historical_parity_not_project_acceptance(self):
        self.history(lambda d:d.update(current_project_cases_covered=68))

    def test_historical_metrics_not_current_quality(self):
        self.history(lambda d:d['metric_boundary'].update(equivalence_to_current_wi2_quality_established=True,current_project_scores={'recall':1.0}))

    def test_historical_missed_cases_not_hidden(self):
        self.history(lambda d:d['summary'].update(known_false_negatives=[],recall=1.0))

    def test_formal_execution_promotion_rejected(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True));self.reject('formal state changed')

    def test_public_gold_injection_rejected(self):
        self.change('public-inputs.json',lambda d:d['cases'][0].update(gold='positive'))
        self.reject('rule gap accounting: public-inputs.json')

    def test_adopted_source_tamper(self):
        p=self.packet/'consumer-source/sources/plugin/agents/analyzer.md.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_raw_object_normalization_policy_rejected(self):
        (self.packet/'.gitattributes').write_text('consumer-source/** text\n');self.rehash();self.reject('Git object preservation attributes')

    def test_duplicate_controlled_inputs_rejected(self):
        d=copy.deepcopy(self.inputs);d['private']['cases'].append(d['private']['cases'][0])
        with self.assertRaisesRegex(ValueError,'duplicate'):guard.prepare.generate(d)

    def test_input_gold_promotion_rejected(self):
        d=copy.deepcopy(self.inputs);d['project_calls'][0]['independent_label']='negative'
        with self.assertRaisesRegex(ValueError,'gold promotion'):guard.prepare.generate(d)

    def test_historical_request_response_omission_rejected(self):
        with self.assertRaisesRegex(ValueError,'request/response'):
            guard.prepare.request_scope(self.inputs['old_requests'],self.inputs['old_responses'][:-1])

    def test_inventory_and_gaps(self):
        d=guard.prepare.generate(self.inputs);c=d['gap-summary.json']['counts']
        self.assertEqual((c['controlled_drafts'],c['project_call_records'],c['caller_references_separate']),(40,68,14))
        self.assertEqual((c['historical_ckg_requests'],c['historical_cks_requests']),(58,9))
        self.assertTrue(all(v=='not_satisfied' for v in d['private-sample-gaps.json']['obligations'].values()))
        r=d['mcp-path-review.json']['role_exposure_finding']
        self.assertTrue(r['required_by_body']);self.assertTrue(r['consumer_input_metadata_parity'])
        self.assertFalse(r['explicit_analyzer_tool_present']);self.assertIsNone(r['effective_host_permissions'])


if __name__=='__main__':unittest.main()
