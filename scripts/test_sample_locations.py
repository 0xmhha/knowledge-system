#!/usr/bin/env python3
"""Adversarial scope, observation coverage and acceptance-boundary tests."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/sample-locations-v1'
spec = importlib.util.spec_from_file_location('locations_guard', ROOT / 'scripts/check-sample-locations.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class SampleLocationTests(unittest.TestCase):
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

    def test_unknown_body_identifier_not_quarantined_as_signature(self):
        occurrence = {'name': 'x', 'role': 'read', 'site': {'start_byte': 40}, 'declaration': {'start_byte': 12}}
        r = guard.prepare.classify_occurrence(occurrence, {'bindings': []}, 20)
        self.assertEqual(r['status'], 'unmatched_scope_not_assumed_local')

    def test_ambiguous_case_bindings_not_first_matched(self):
        b = {'kind': 'var', 'name': 'x', 'site': {'start_byte': 12}}
        occurrence = {'name': 'x', 'role': 'read', 'site': {'start_byte': 40}, 'declaration': {'start_byte': 12}}
        r = guard.prepare.classify_occurrence(occurrence, {'bindings': [dict(b, id='case-a'), dict(b, id='case-b')]}, 20)
        self.assertEqual(r['status'], 'ambiguous_lexical_scope_not_collapsed')
        self.assertEqual(len(r['possible_lexical_bindings']), 2)

    def test_signature_definition_without_body_binding_is_quarantined(self):
        occurrence = {'name': 'x', 'role': 'definition', 'site': {'start_byte': 12}, 'declaration': {'start_byte': 12}}
        r = guard.prepare.classify_occurrence(occurrence, {'bindings': []}, 20)
        self.assertEqual(r['status'], 'quarantined_non_body_signature_scope')

    def test_actual_parameter_before_body_still_corroborated(self):
        b = {'kind': 'var', 'name': 'x', 'id': 'function:x', 'site': {'start_byte': 12}}
        occurrence = {'name': 'x', 'role': 'definition', 'site': {'start_byte': 12}, 'declaration': {'start_byte': 12}}
        r = guard.prepare.classify_occurrence(occurrence, {'bindings': [b]}, 20)
        self.assertEqual(r['status'], 'corroborated_own_function_variable')

    def test_quarantined_callback_promoted_to_own_local(self):
        def alter(d):
            for f in d['functions']:
                for r in f['occurrence_audit']:
                    if r['status'] == 'quarantined_non_body_signature_scope':
                        r['status'] = 'corroborated_own_function_variable'
                        return
        self.change('scope-audit.json', alter)
        self.rejected('scope/coverage accounting: scope-audit.json')

    def test_same_count_with_wrong_new_function(self):
        self.change('function-checklist.json', lambda d: d[0].update(candidate_id='SRC-026'))
        self.rejected('scope/coverage accounting: function-checklist.json')

    def test_false_independent_function_count(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(approved_independent_functions=30))
        self.rejected('scope/coverage accounting: coverage-summary.json')

    def test_capture_promoted_to_full_gold(self):
        self.change('function-checklist.json', lambda d: d[0].update(exhaustive_data_gold='approved', acceptance='pass'))
        self.rejected('scope/coverage accounting: function-checklist.json')

    def test_scalar_kind_inferred_from_partial_types(self):
        self.change('scope-audit.json', lambda d: d['functions'][0]['limitations'].update(scalar_reference_aggregate_kind='scalar_proven'))
        self.rejected('scope/coverage accounting: scope-audit.json')

    def test_range_support_invented(self):
        self.change('scope-audit.json', lambda d: d['functions'][0]['limitations'].update(range_backend_mapping='fully_supported'))
        self.rejected('scope/coverage accounting: scope-audit.json')

    def test_closure_exclusion_erased(self):
        def alter(d):
            next(f for f in d['functions'] if f['limitations']['excluded_closures'])['limitations']['excluded_closures'] = []
        self.change('scope-audit.json', alter)
        self.rejected('scope/coverage accounting: scope-audit.json')

    def test_empty_edges_claimed_as_complete_negative(self):
        self.change('coverage-summary.json', lambda d: d.update(data_edges=[], control_edges=[]))
        self.rejected('scope/coverage accounting: coverage-summary.json')

    def test_postdominators_invented_from_marker(self):
        self.change('scope-audit.json', lambda d: d['functions'][0]['limitations'].update(normal_exit_postdominators='computed'))
        self.rejected('scope/coverage accounting: scope-audit.json')

    def test_raw_location_binding_tamper(self):
        def alter(rows):
            rows[0]['functions'][0]['occurrences'][0]['declaration']['start_byte'] += 1
        self.change('logs/source-observations.json', alter)
        self.rejected('actual selected-source replay mismatch')

    def test_jump_owner_tamper(self):
        def alter(rows):
            for file in rows:
                for f in file['functions']:
                    for m in f['markers']:
                        if m['kind'] == 'continue' and m.get('target'):
                            m['target']['start_line'] += 1
                            return
        self.change('logs/source-observations.json', alter)
        self.rejected('actual selected-source replay mismatch')

    def test_partial_type_diagnostics_hidden(self):
        self.change('logs/source-observations.json', lambda d: d[0].update(partial_type_error_count=0, partial_type_error_examples=[]))
        self.rejected('partial source typecheck boundary')

    def test_missing_selected_source_input(self):
        self.change('observation-inputs.json', lambda d: d[0]['Starts'].pop())
        self.rejected('source observer inputs')

    def test_raw_native_failure_despite_pass_summary(self):
        p = self.packet / 'logs/race.jsonl'
        p.write_text(p.read_text() + json.dumps({'Action': 'fail', 'Test': 'TestDependencyReviewLocations'}) + '\n')
        self.rehash()
        self.rejected('raw control completion')

    def test_raw_native_observation_forged(self):
        p = self.packet / 'logs/normal.jsonl'
        p.write_text(p.read_text().replace('pass=true', 'pass=false'))
        self.rehash()
        self.rejected('raw control observations')

    def test_full_project_build_claim(self):
        self.change('validation.json', lambda d: d.update(full_project_typecheck_passed=True, project_packages_built=25))
        self.rejected('validation scope')

    def test_backend_execution_claim(self):
        self.change('validation.json', lambda d: d.update(dependency_backend_calls=30))
        self.rejected('execution boundary')

    def test_formal_execution_promotion(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state promotion')

    def test_extra_public_input_with_private_scope_data(self):
        (self.packet / 'public-inputs.json').write_text('{"gold":true}\n')
        self.rehash()
        self.rejected('unexpected source observation artifact')


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
