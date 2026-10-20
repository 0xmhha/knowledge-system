#!/usr/bin/env python3
"""Reject WI-6 family, pair, relevance and latency evidence promotion."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/expansion-gap-v1'
spec=importlib.util.spec_from_file_location('expansion_gap_guard',ROOT/'scripts/check-expansion-gap.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class ExpansionGapTests(unittest.TestCase):
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
        self.change('private-family-pair-gaps.json',action);self.reject('expansion gap accounting: private-family-pair-gaps.json')

    def case(self,action):self.sample(lambda d:action(d['cases'][0]))

    def path(self,action):
        self.change('retrieval-path-review.json',action);self.reject('expansion gap accounting: retrieval-path-review.json')

    def warm(self,action):
        self.change('warm-latency-contract.json',action);self.reject('expansion gap accounting: warm-latency-contract.json')

    def history(self,action):
        self.change('historical-evidence-scope.json',action);self.reject('expansion gap accounting: historical-evidence-scope.json')

    def test_real_packet(self):self.assertEqual(guard.check(PACKET),[])

    def test_twenty_cards_not_independent_families(self):
        self.case(lambda r:r.update(approved_independent_family=True))

    def test_stress_not_operational_sample(self):
        self.sample(lambda d:next(r for r in d['cases'] if r['candidate_id']=='SRC-026').update(scope='production_source_candidate',operational_execution_allowed=True))

    def test_production_shortfall_not_hidden(self):
        self.change('gap-summary.json',lambda d:d.update(operational_draft_shortfall_even_before_independence_review=0))
        self.reject('expansion gap accounting: gap-summary.json')

    def test_different_intents_not_independence(self):
        self.change('pairwise-review.json',lambda d:d[0].update(semantic_independence='approved'))
        self.reject('expansion gap accounting: pairwise-review.json')

    def test_exposure_overlap_not_hidden(self):
        self.case(lambda r:r.update(same_seed_exposed_in_wi1_sample=False))

    def test_unknowns_not_removed(self):self.case(lambda r:r.update(existing_unknowns=[]))

    def test_missing_relevance_not_empty_gold(self):self.case(lambda r:r.update(independent_relevance_gold=[]))

    def test_ai_anchors_not_relevance_gold(self):self.case(lambda r:r.update(independent_relevance_gold=r['existing_ai_anchors']))

    def test_canonical_seed_resolution_not_invented(self):self.case(lambda r:r.update(resolved_retained_seed='fake-node'))

    def test_same_snapshot_not_assumed(self):self.case(lambda r:r.update(expected_snapshot_sha256='a'*64))

    def test_actual_pair_receipt_not_invented(self):self.case(lambda r:r.update(paired_mcp_receipt='passed'))

    def test_arm_order_not_frozen(self):self.case(lambda r:r.update(arm_order_policy='baseline-first'))

    def test_local_and_aggregate_limit_contracts_not_implicitly_equal(self):
        self.case(lambda r:r.update(local_limit_policy='defaults',aggregate_limit_policy='same'))

    def test_missing_return_set_not_observed_empty(self):self.case(lambda r:r.update(baseline_returned_functions=[]))

    def test_unscored_deltas_not_zero(self):self.case(lambda r:r.update(recall_delta=0,noise_fraction_delta=0))

    def test_prompt_not_engine_query_field(self):self.path(lambda d:d.update(intent_is_engine_query_field=True))

    def test_outgoing_expansion_not_caller_search(self):self.path(lambda d:d.update(incoming_call_expansion_implemented_here=True))

    def test_call_hops_not_argument_transfer(self):self.path(lambda d:d.update(computed_argument_return_transfer=True))

    def test_curated_metadata_not_verified_join(self):self.path(lambda d:d.update(verified_curated_dataset_join=True))

    def test_consumer_metadata_not_explicit_role_tool(self):
        self.path(lambda d:d['consumer_exposure_gaps'][0].update(explicit_analyzer_tool_present=True))

    def test_missing_explicit_role_tool_not_runtime_denial(self):
        self.path(lambda d:d['consumer_exposure_gaps'][0].update(runtime_denial_proven=True))

    def test_fixture_mcp_not_current_project(self):self.history(lambda d:d.update(current_project_cases_covered=20))

    def test_historical_recall_success_preserved(self):self.history(lambda d:d['derived_arithmetic'].update(expanded_recall=0))

    def test_noise_increase_not_hidden(self):self.history(lambda d:d['derived_arithmetic'].update(noise_fraction_delta=0))

    def test_single_elapsed_not_warm_p95(self):
        self.history(lambda d:d['derived_arithmetic'].update(warm_p95_latency_ratio=d['derived_arithmetic']['single_server_elapsed_ratio']))

    def test_historical_metrics_not_formal_threshold_acceptance(self):
        self.history(lambda d:d['derived_arithmetic'].update(formal_threshold_verdict='passed'))

    def test_other_protocol_warm_count_not_imported(self):self.warm(lambda d:d.update(repetitions_per_arm=20))

    def test_quantile_and_ratio_not_silently_frozen(self):self.warm(lambda d:d.update(p95_definition='nearest-rank',ratio_definition='ratio of p95s'))

    def test_clock_scope_not_frozen(self):self.warm(lambda d:d.update(clock_and_measurement_boundary='server elapsed only'))

    def test_missing_samples_not_empty_success(self):self.warm(lambda d:d.update(raw_samples=[],warm_p95_ratio=1))

    def test_public_gold_injection(self):
        self.change('public-inputs.json',lambda d:d['cases'][0].update(relevance_gold=['target']))
        self.reject('expansion gap accounting: public-inputs.json')

    def test_formal_execution_promotion(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True));self.reject('formal state changed')

    def test_adopted_source_tamper(self):
        p=self.packet/'consumer-source/sources/plugin/agents/analyzer.md.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_raw_object_normalization_policy(self):
        (self.packet/'.gitattributes').write_text('consumer-source/** text\n');self.rehash();self.reject('Git object preservation attributes')

    def test_duplicate_seed_rejected(self):
        d=copy.deepcopy(self.inputs);d['public']['cases'].append(d['public']['cases'][0])
        with self.assertRaisesRegex(ValueError,'duplicate'):guard.prepare.generate(d)

    def test_input_gold_promotion_rejected(self):
        d=copy.deepcopy(self.inputs);d['private']['review_drafts'][0]['relevance_labels']=[]
        with self.assertRaisesRegex(ValueError,'relevance/family promotion'):guard.prepare.generate(d)

    def test_missing_historical_response(self):
        with self.assertRaisesRegex(ValueError,'request/response'):
            guard.prepare.rpc_rows(self.inputs['old_requests'],self.inputs['old_responses'][:-1])

    def test_inventory_arithmetic_and_unknowns(self):
        d=guard.prepare.generate(self.inputs);c=d['gap-summary.json']['counts'];a=d['historical-evidence-scope.json']['derived_arithmetic']
        self.assertEqual((c['production_seed_proposals'],c['development_stress_source_only'],c['approved_independent_families']),(19,1,0))
        self.assertEqual((c['pairwise_reviews'],c['same_file_pairs'],c['same_intent_pairs'],c['overlap_with_exposed_wi1_sample']),(190,7,41,16))
        self.assertAlmostEqual(a['recall_delta'],.75);self.assertAlmostEqual(a['noise_fraction_delta'],.2)
        self.assertAlmostEqual(a['single_server_elapsed_ratio'],2875/1431);self.assertIsNone(a['warm_p95_latency_ratio'])
        self.assertTrue(all(v=='not_satisfied' for v in d['private-family-pair-gaps.json']['obligations'].values()))


if __name__=='__main__':unittest.main()
