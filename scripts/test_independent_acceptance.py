#!/usr/bin/env python3
"""False-ready, leakage, matching and metric controls using synthetic test data."""
import copy
import json
import shutil
import tempfile
import unittest
from pathlib import Path

import independent_acceptance as guard


def example_gold():
    # A structural unit-test double, never a real human approval or source gold.
    gold = guard.gold_form('WI-3')
    gold.update(case_id='synthetic-unit-test', family_id='synthetic-family',
                source_pins={'project': 'test-only'},
                universe={'unit_ids': ['a', 'b', 'c']},
                completeness_review='synthetic bounded test universe',
                matching_policy='test scoped exact identity',
                unknown_and_abstention_policy='unknown retained',
                denominator_policy='all3 synthetic units', author='test-double',
                public_input_sha256='a' * 64)
    for field, unit in zip(guard.GOLD_SETS['WI-3'], ['a', 'b', 'c']):
        gold['answer_sets'][field] = [{'id': unit, 'source_ref': 'synthetic-only', 'rationale': 'unit test label'}]
    bind(gold)
    return gold


def bind(gold):
    gold['independent_review_receipt'] = {
        'kind': 'human', 'name': 'synthetic-test-double', 'role': 'test-only',
        'decision': 'approved', 'independence_declaration': 'synthetic; not an actual review',
        'evidence_path': 'synthetic-test-only', 'artifact_sha256': guard.gold_content_hash(gold),
    }


class GoldAndSplitTests(unittest.TestCase):
    def test_bounded_structural_example_not_semantic_acceptance(self):
        self.assertEqual(guard.gold_errors(example_gold()), [])

    def test_all_authoring_forms_are_unscoreable(self):
        for work_id in (*guard.WORK_IDS, 'ONTOLOGY'):
            with self.subTest(work_id=work_id):
                self.assertTrue(guard.gold_errors(guard.gold_form(work_id)))

    def test_null_labels_not_negative_gold(self):
        gold = example_gold(); gold['answer_sets']['irrelevant_locations'] = None
        bind(gold); self.assertIn('unlabelled answer set irrelevant_locations', guard.gold_errors(gold))

    def test_empty_set_requires_reviewed_reason(self):
        gold = example_gold(); gold['answer_sets']['irrelevant_locations'] = []
        gold['universe']['unit_ids'].remove('b'); bind(gold)
        self.assertTrue(guard.gold_errors(gold))
        gold['empty_set_rationales']['irrelevant_locations'] = 'bounded synthetic universe contains no negative'; bind(gold)
        self.assertEqual(guard.gold_errors(gold), [])

    def test_incomplete_universe_not_complete_gold(self):
        gold = example_gold(); gold['universe']['unit_ids'].append('missing'); bind(gold)
        self.assertTrue(guard.gold_errors(gold))

    def test_duplicate_or_conflicting_labels_refused(self):
        gold = example_gold(); gold['answer_sets']['irrelevant_locations'][0]['id'] = 'a'; bind(gold)
        self.assertTrue(guard.gold_errors(gold))

    def test_label_outside_universe_refused(self):
        gold = example_gold(); gold['answer_sets']['relevant_locations'][0]['id'] = 'foreign'; bind(gold)
        self.assertTrue(guard.gold_errors(gold))

    def test_source_and_rationale_required(self):
        gold = example_gold(); gold['answer_sets']['relevant_locations'][0].pop('source_ref'); bind(gold)
        self.assertTrue(guard.gold_errors(gold))

    def test_ai_receipt_cannot_replace_human(self):
        gold = example_gold(); gold['independent_review_receipt']['kind'] = 'AI'
        self.assertTrue(guard.gold_errors(gold))

    def test_changed_gold_invalidates_receipt(self):
        gold = example_gold(); gold['matching_policy'] = 'after seeing results'
        self.assertIn('review receipt is not bound to current gold content', guard.gold_errors(gold))

    def test_host_cannot_read_private_gold(self):
        gold = example_gold(); gold['host_access_policy'] = 'all-repo'; bind(gold)
        self.assertTrue(guard.gold_errors(gold))

    def test_semantic_family_leak_despite_different_text(self):
        cases = [{'split': split, 'family_id': 'same-fact', 'exposed_before_freeze': False,
                  'semantic_family_review_receipt': 'synthetic'} for split in ('DEV', 'FINAL')]
        self.assertIn('family leaks across DEV/FINAL', guard.split_errors(cases))

    def test_duplicate_source_with_renamed_family_refused(self):
        cases = [{'split': split, 'family_id': split, 'source_function_sha256': 'a' * 64,
                  'exposed_before_freeze': False, 'semantic_family_review_receipt': 'synthetic'} for split in ('DEV', 'FINAL')]
        self.assertIn('exact input/source leaks across DEV/FINAL', guard.split_errors(cases))

    def test_unknown_or_exposed_final_refused(self):
        for exposed in (None, True):
            self.assertTrue(guard.split_errors([{'split': 'FINAL', 'family_id': 'f', 'exposed_before_freeze': exposed,
                                                'semantic_family_review_receipt': 'synthetic'}]))

    def test_hash_uniqueness_does_not_prove_independence(self):
        self.assertTrue(guard.split_errors([{'split': 'FINAL', 'family_id': 'unique', 'exposed_before_freeze': False}]))

    def test_repeats_do_not_inflate_family_count(self):
        cases = [dict(split='FINAL', family_id='one', independent_reviewed=True, exposed_before_freeze=False)] * 100
        self.assertEqual(guard.sufficiency(cases, 20)['independent_FINAL_families'], 1)
        self.assertEqual(guard.sufficiency(cases, 20)['verdict'], 'inconclusive')

    def test_floor_met_still_requires_precision_review(self):
        cases = [dict(split='FINAL', family_id=str(n), independent_reviewed=True, exposed_before_freeze=False) for n in range(20)]
        self.assertEqual(guard.sufficiency(cases, 20)['verdict'], 'requires_precision_and_coverage_review')

    def test_undeclared_minimum_not_pass(self):
        self.assertEqual(guard.sufficiency([], None)['verdict'], 'inconclusive')

    def test_same_name_location_across_projects_does_not_match(self):
        location = dict(project_id='one', dataset_version='v', source_mode='repo', path='tx.go',
                        file_sha256='a' * 64, start_byte=0, end_byte_exclusive=10, kind='function')
        other = dict(location, project_id='two')
        self.assertNotEqual(guard.location_key(location), guard.location_key(other))

    def test_invalid_byte_spans_refused(self):
        with self.assertRaises(ValueError):
            guard.location_key(dict(project_id='one', dataset_version='v', source_mode='repo', path='tx.go',
                                    file_sha256='a' * 64, start_byte=10, end_byte_exclusive=10, kind='function'))


class MetricTests(unittest.TestCase):
    def verdict(self, **changes):
        data = dict(delta_recall=.05, ci_lower=.001, delta_precision=-.02, latency_ratio=1.25,
                    contamination=0, citation_fraction=1, sufficient=True, reviewed_links=True,
                    paired_pins_match=True, final_predeclared=True)
        data.update(changes)
        return guard.ontology_verdict(**data)

    def test_gate_boundary_values(self):
        self.assertEqual(self.verdict(), 'proposed_retrieval_gate_met_requires_human_acceptance')

    def test_no_positive_interval_no_benefit(self):
        self.assertEqual(self.verdict(ci_lower=0), 'no_demonstrated_benefit_keep_off')

    def test_each_missing_design_prerequisite_inconclusive(self):
        for field in ('sufficient', 'reviewed_links', 'final_predeclared'):
            with self.subTest(field=field):
                self.assertEqual(self.verdict(**{field: False}), 'inconclusive')

    def test_no_retrieval_gain_cannot_pass(self):
        self.assertEqual(self.verdict(delta_recall=.0499), 'no_demonstrated_benefit_keep_off')

    def test_concept_text_arm_does_not_require_implementation_links(self):
        self.assertEqual(self.verdict(final_mode='concept_text', reviewed_links=False),
                         'proposed_retrieval_gate_met_requires_human_acceptance')

    def test_final_mode_must_be_real_predeclared_treatment(self):
        self.assertEqual(self.verdict(final_mode='off'), 'inconclusive')

    def test_current_indices_are_bound(self):
        self.assertEqual(guard.current_index_errors(), [])

    def test_current_reference_promotion_or_omission_refused(self):
        ref = {'status': 'authoring_prepared_not_frozen_not_accepted'}; state = {'formal_completed': 0}
        document = {'independent_freeze_preparation': ref, 'current_execution_state': state}
        self.assertEqual(guard.index_errors([document], ref, state), [])
        for field in ('independent_freeze_preparation', 'current_execution_state'):
            changed = copy.deepcopy(document); changed.pop(field)
            self.assertTrue(guard.index_errors([changed], ref, state))

    def test_precision_and_latency_regressions(self):
        self.assertEqual(self.verdict(delta_precision=-.0201), 'no_demonstrated_benefit_keep_off')
        self.assertEqual(self.verdict(latency_ratio=1.2501), 'no_demonstrated_benefit_keep_off')

    def test_source_pin_and_cross_project_boundaries(self):
        for field, value in [('paired_pins_match', False), ('contamination', 1), ('citation_fraction', .999)]:
            self.assertEqual(self.verdict(**{field: value}), 'boundary_failure_keep_off')

    def test_missing_or_nonfinite_measurement_inconclusive(self):
        for value in (None, float('nan'), float('inf'), True):
            self.assertEqual(self.verdict(delta_recall=value), 'inconclusive')

    def test_nearest_rank_and_invalid_raw_times(self):
        self.assertEqual(guard.nearest_rank(list(range(1, 21))), 19)
        for values in ([], [-1], [float('nan')], [True]):
            with self.assertRaises(ValueError): guard.nearest_rank(values)


class PacketMutationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.packet = Path(self.temporary.name) / 'packet'
        shutil.copytree(guard.ROOT / guard.PACKET, self.packet)

    def mutate(self, name, action):
        path = self.packet / name; data = guard.read(path); action(data)
        path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')
        if name == 'manifest.json':
            return guard.packet_errors(guard.ROOT, self.packet)
        # Re-seal mutation so these tests exercise semantics, not only hashes.
        manifest_path = self.packet / 'manifest.json'; manifest = guard.read(manifest_path)
        manifest['artifacts'][name] = guard.digest(path)
        manifest_path.write_text(json.dumps(manifest) + '\n')
        return guard.packet_errors(guard.ROOT, self.packet)

    def test_actual_preparation_packet_valid(self):
        self.assertEqual(guard.packet_errors(guard.ROOT, self.packet), [])

    def test_exposed_final_promotion_refused_after_reseal(self):
        self.assertTrue(self.mutate('candidate-ledger.json', lambda d: d['records'][0].update(eligible_fresh_FINAL=True)))

    def test_gold_form_approval_refused_after_reseal(self):
        self.assertTrue(self.mutate('gold-authoring-forms.json', lambda d: d[0].update(status='approved')))

    def test_forged_freeze_decision_refused_after_reseal(self):
        self.assertTrue(self.mutate('freeze-readiness.json', lambda d: d[0].update(decision_status='approved')))

    def test_duplicate_freeze_topic_refused(self):
        self.assertTrue(self.mutate('freeze-readiness.json', lambda d: d.append(d[0])))

    def test_false_formal_completion_refused(self):
        self.assertTrue(self.mutate('formal-state.json', lambda d: d.update(formal_completed=9)))

    def test_sampling_count_inflation_refused(self):
        self.assertTrue(self.mutate('candidate-ledger.json', lambda d: d['counts_by_stratum'].update({'WI-1': 300})))

    def test_unassigned_final_minimum_preserved(self):
        self.assertTrue(self.mutate('sample-sufficiency.json', lambda d: d['groups'][0].update(FINAL_minimum_independent_units=1)))

    def test_threshold_cannot_be_relaxed(self):
        self.assertTrue(self.mutate('ontology-contract.json', lambda d: d['gate'].update(recall_at_10_delta_min=0)))

    def test_sample_floor_not_approved_sufficiency(self):
        self.assertTrue(self.mutate('sample-sufficiency.json', lambda d: d['groups'][0].update(sufficiency_verdict='pass')))

    def test_unreviewed_links_not_reviewed(self):
        self.assertTrue(self.mutate('ontology-contract.json', lambda d: d.update(reviewed_link_count=7)))

    def test_final_manifest_not_invented(self):
        self.assertTrue(self.mutate('holdout-procedure.json', lambda d: d.update(formal_FINAL_manifest='existing-exposed.json')))

    def test_old_preparation_not_formal_acceptance(self):
        self.assertTrue(self.mutate('WORKLIST.json', lambda d: d.update(formal_completed=9)))

    def test_missing_raw_artifact_refused(self):
        (self.packet / 'candidate-ledger.json').unlink()
        self.assertTrue(guard.packet_errors(guard.ROOT, self.packet))

    def test_missing_source_bindings_refused(self):
        self.assertTrue(self.mutate('manifest.json', lambda d: d.update(source_refs=[])))

    def test_authoring_case_omission_refused_after_reseal(self):
        self.assertTrue(self.mutate('private-authoring-index.json', lambda d: d.pop()))

    def test_native_reuse_not_presented_as_fresh(self):
        self.assertTrue(self.mutate('native-reuse.json', lambda d: d.update(fresh_Go_full_tests_this_stage=True)))


if __name__ == '__main__':
    unittest.main()
