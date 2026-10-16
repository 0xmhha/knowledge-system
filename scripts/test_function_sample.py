#!/usr/bin/env python3
"""Reject sample-count, leakage, family and acceptance overclaims after rehashing."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/function-sample-v1'
spec = importlib.util.spec_from_file_location('sample_guard', ROOT / 'scripts/check-function-sample.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class FunctionSampleTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['artifacts'] = {p.relative_to(self.packet).as_posix(): guard.sha(p.read_bytes())
                          for p in self.packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        p.write_text(json.dumps(d))

    def change(self, name, fn):
        p = self.packet / name
        d = json.loads(p.read_text())
        fn(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.rehash()

    def rejected(self, message=None):
        errors = guard.check(self.packet, check_references=False)
        self.assertTrue(errors)
        if message:
            self.assertIn(message, errors)

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_stress_main_included(self):
        self.change('sample-summary.json', lambda d: d['selected_proposal'].__setitem__(0, 'SRC-026'))
        self.rejected('sample/exposure accounting: sample-summary.json')

    def test_duplicate_selected_function(self):
        self.change('sample-summary.json', lambda d: d['selected_proposal'].__setitem__(1, d['selected_proposal'][0]))
        self.rejected('sample/exposure accounting: sample-summary.json')

    def test_number_promoted_to_independence(self):
        self.change('sample-summary.json', lambda d: d['counts'].update(approved_independent_functions=30))
        self.rejected('sample/exposure accounting: sample-summary.json')

    def test_module_quota_tamper(self):
        self.change('sample-summary.json', lambda d: d['module_quotas'].update(rpc=4))
        self.rejected('sample/exposure accounting: sample-summary.json')

    def test_old_hint_replaces_actual_shadow(self):
        self.change('sample-summary.json', lambda d: d['construct_function_counts'].update(shadow=30))
        self.rejected('sample/exposure accounting: sample-summary.json')

    def test_public_exposure_removed(self):
        self.change('function-inventory.json', lambda d: d['functions'][0]['exposure'].update(candidate_pool_material=False, unexposed_holdout=True))
        self.rejected('sample/exposure accounting: function-inventory.json')

    def test_private_dependency_review_hidden(self):
        def alter(d):
            next(r for r in d['functions'] if r['exposure']['private_dependency_review'])['exposure']['private_dependency_review'] = False
        self.change('function-inventory.json', alter)
        self.rejected('sample/exposure accounting: function-inventory.json')

    def test_unknown_topics_marked_fresh(self):
        def alter(d):
            next(r for r in d['functions'] if r['exposure']['prior_ai_topic_flags'] is None)['exposure']['semantic_decontamination'] = 'passed'
        self.change('function-inventory.json', alter)
        self.rejected('sample/exposure accounting: function-inventory.json')

    def test_final_allocation(self):
        self.change('function-inventory.json', lambda d: d['functions'][0].update(split='FINAL', final_eligibility='approved'))
        self.rejected('sample/exposure accounting: function-inventory.json')

    def test_prior_literal_mismatch_becomes_independence(self):
        self.change('prior-question-review.json', lambda d: d[0].update(semantic_independence='independent'))
        self.rejected('sample/exposure accounting: prior-question-review.json')

    def test_same_file_pair_omitted(self):
        self.change('pairwise-review.json', lambda d: next(r for r in d if r['both_selected'] and r['same_source_file']).update(same_source_file=False))
        self.rejected('sample/exposure accounting: pairwise-review.json')

    def test_file_groups_promoted_to_families(self):
        self.change('sample-summary.json', lambda d: d.update(file_groups_are_independent_families=True))
        self.rejected('sample/exposure accounting: sample-summary.json')

    def test_ai_family_flag_treated_as_approved(self):
        self.change('pairwise-review.json', lambda d: next(r for r in d if r['shared_unapproved_symptom_group']).update(semantic_family_review='approved'))
        self.rejected('sample/exposure accounting: pairwise-review.json')

    def test_pair_inventory_missing(self):
        self.change('pairwise-review.json', lambda d: d.pop())
        self.rejected('sample/exposure accounting: pairwise-review.json')

    def test_prior_comparisons_missing(self):
        self.change('prior-question-review.json', lambda d: d.pop())
        self.rejected('sample/exposure accounting: prior-question-review.json')

    def test_reviewed_examples_become_exhaustive_gold(self):
        self.change('review-checklist.json', lambda d: d[0].update(exhaustive_data_gold='approved'))
        self.rejected('sample/exposure accounting: review-checklist.json')

    def test_selection_trace_tamper(self):
        self.change('selection-trace.json', lambda d: d[0].update(new_source_file=False))
        self.rejected('sample/exposure accounting: selection-trace.json')

    def test_representative_random_sample_claim(self):
        self.change('selection-policy.json', lambda d: d.update(representativeness='proved', kind='random_sample'))
        self.rejected('sample/exposure accounting: selection-policy.json')

    def test_source_identity_tamper(self):
        self.change('function-inventory.json', lambda d: d['functions'][0]['source'].update(function_sha256='0'*64))
        self.rejected('sample/exposure accounting: function-inventory.json')

    def test_formal_execution_promotion(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state changed')

    def test_slot_and_reserve_properties(self):
        rows = json.loads((ROOT / guard.prepare.INPUTS['lexical']).read_text())['functions']
        selected, quotas, counts, trace = guard.prepare.select(list(reversed(rows)))
        self.assertEqual(selected, guard.prepare.select(rows)[0])
        self.assertEqual(len(selected), len(set(selected)))
        self.assertNotIn('SRC-026', selected)
        self.assertEqual(sum(quotas.values()), 30)
        self.assertTrue(all(c >= 4 for c in counts.values()))
        self.assertEqual([r['slot'] for r in trace], list(range(1, 31)))


if __name__ == '__main__':
    unittest.main()
