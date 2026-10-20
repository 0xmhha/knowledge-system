#!/usr/bin/env python3
"""Reject false completion, scope collapse and invented gold after packet rehashing."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/semantic-gap-v1'
spec = importlib.util.spec_from_file_location('semantic_gap_guard', ROOT / 'scripts/check-semantic-gap.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class SemanticGapTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.inputs = guard.prepare.source_inputs()

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        p = self.packet / 'manifest.json'
        data = json.loads(p.read_text())
        data['artifacts'] = {p.relative_to(self.packet).as_posix(): guard.sha(p.read_bytes())
                             for p in self.packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        p.write_text(json.dumps(data))

    def change(self, name, action):
        p = self.packet / name
        data = json.loads(p.read_text())
        action(data)
        p.write_text(json.dumps(data, ensure_ascii=False))
        self.rehash()

    def reject(self, message):
        self.assertIn(message, guard.check(self.packet, check_references=False))

    def function(self, action, cid=None):
        self.change('private-function-gaps.json', lambda d: action(next(r for r in d['functions'] if cid is None or r['candidate_id'] == cid)))
        self.reject('semantic gap accounting: private-function-gaps.json')

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_missing_data_gold_becomes_empty(self):
        self.function(lambda r: r.update(data_gold=[]))

    def test_missing_control_gold_becomes_empty(self):
        self.function(lambda r: r.update(control_gold=[]))

    def test_unknowns_become_negative_gold(self):
        self.function(lambda r: r.update(excluded_or_unknown_gold=[]))

    def test_value_kind_invented(self):
        self.function(lambda r: r['binding_inventory'][0].update(value_kind='scalar'))

    def test_quarantined_signature_promoted(self):
        def alter(r):
            refs = r['source_observation_refs']
            refs['local_occurrence_indices'] += refs['quarantined_signature_indices']
            refs['quarantined_signature_indices'] = []
        self.function(alter, 'SRC-024')

    def test_range_bindings_hidden(self):
        self.function(lambda r: r['syntax_profile'].update(range_bindings=0))

    def test_closure_effects_inlined(self):
        self.function(lambda r: r['syntax_profile'].update(excluded_closures=[]), 'SRC-040')

    def test_bare_return_marker_implies_named_use(self):
        self.function(lambda r: r['syntax_profile'].update(named_result_implicit_uses='proven'), 'SRC-010')

    def test_source_anchor_forgery(self):
        self.function(lambda r: r['semantic_review_draft']['anchor'].update(start_byte=0))

    def test_source_identity_forgery(self):
        self.function(lambda r: r['source'].update(function_sha256='0' * 64))

    def test_requirement_false_completion(self):
        self.function(lambda r: r['requirements'].update(complete_reaching_definition_sets='complete'))

    def test_prior_material_promoted_to_complete(self):
        self.function(lambda r: r['prior_ai_material'].update(all_material_is_non_exhaustive=False))

    def test_ai_review_approval(self):
        self.function(lambda r: r['semantic_review_draft'].update(approved=True))

    def test_family_or_final_approval(self):
        self.function(lambda r: r.update(independence_approved=True, split='FINAL', scoring_allowed=True))

    def test_count_double_counts_legacy_overlap(self):
        self.change('gap-summary.json', lambda d: d['counts'].update(unique_functions_with_any_prior_relation_material=7))
        self.reject('semantic gap accounting: gap-summary.json')

    def test_syntax_availability_becomes_complete_gold(self):
        self.change('gap-summary.json', lambda d: d['counts'].update(functions_missing_complete_data_gold=0, independent_gold_records=30))
        self.reject('semantic gap accounting: gap-summary.json')

    def test_comment_overrides_implementation_coordinate(self):
        self.change('gap-summary.json', lambda d: d['comment_implementation_mismatch'].update(implementation_coordinate='declaration', gold_mapping_frozen=True))
        self.reject('semantic gap accounting: gap-summary.json')

    def test_matching_contract_frozen(self):
        self.change('matching-gap-review.json', lambda d: d.update(canonical_matching_frozen=True, scoring_allowed=True))
        self.reject('semantic gap accounting: matching-gap-review.json')

    def test_evaluation_units_collapsed(self):
        self.change('matching-gap-review.json', lambda d: d.update(evaluation_units_must_remain_separate=['all_edges']))
        self.reject('semantic gap accounting: matching-gap-review.json')

    def test_engine_anchor_forgery(self):
        self.change('matching-gap-review.json', lambda d: d['reviews'][0]['anchor'].update(span_sha256='0' * 64))
        self.reject('semantic gap accounting: matching-gap-review.json')

    def test_engine_copy_changed(self):
        p = self.packet / 'engine-source/pkg/graph/valueflow/cfg.go.txt'
        p.write_bytes(p.read_bytes() + b'\n')
        self.rehash()
        self.reject('engine source binding: engine-source/pkg/graph/valueflow/cfg.go.txt')

    def test_formal_execution_promotion(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.reject('formal state changed')

    def test_extra_artifact(self):
        (self.packet / 'gold.json').write_text('{}')
        self.rehash()
        self.reject('unexpected artifact')

    def test_duplicate_function_input(self):
        data = copy.deepcopy(self.inputs)
        data['scope']['functions'].append(data['scope']['functions'][0])
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            guard.prepare.generate(data)

    def test_stress_source_replacement_input(self):
        data = copy.deepcopy(self.inputs)
        data['sample']['selected_proposal'][0] = 'SRC-026'
        with self.assertRaisesRegex(ValueError, 'selected thirty'):
            guard.prepare.generate(data)

    def test_outside_function_question_input(self):
        data = copy.deepcopy(self.inputs)
        data['proposals']['functions'][0]['lines'] = [1, 2]
        with self.assertRaisesRegex(ValueError, 'outside selected'):
            guard.prepare.generate(data)

    def test_ambiguous_binding_input(self):
        data = copy.deepcopy(self.inputs)
        data['scope']['functions'][0]['occurrence_audit'][0]['possible_lexical_bindings'] *= 2
        with self.assertRaisesRegex(ValueError, 'ambiguous local'):
            guard.prepare.generate(data)

    def test_unresolved_value_kind_input(self):
        data = copy.deepcopy(self.inputs)
        data['scope']['functions'][0]['occurrence_audit'][0]['value_kind'] = 'scalar'
        with self.assertRaisesRegex(ValueError, 'value kind/gold'):
            guard.prepare.generate(data)

    def test_inventory_properties(self):
        out = guard.prepare.generate(self.inputs)
        counts = out['gap-summary.json']['counts']
        self.assertEqual((counts['selected_functions'], counts['local_occurrences'], counts['quarantined_signature_occurrences']), (30, 1721, 4))
        self.assertEqual((counts['local_bindings_needing_value_kind_review'], counts['unique_functions_with_any_prior_relation_material']), (412, 6))
        self.assertEqual((counts['functions_missing_complete_data_gold'], counts['functions_missing_complete_control_gold']), (30, 30))
        for r in out['private-function-gaps.json']['functions']:
            self.assertIsNone(r['data_gold'])
            self.assertIsNone(r['control_gold'])
            self.assertEqual(set(r['requirements']), set(guard.prepare.REQUIREMENTS))
            self.assertTrue(all(v == 'not_satisfied' for v in r['requirements'].values()))


if __name__ == '__main__':
    unittest.main()
