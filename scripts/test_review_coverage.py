#!/usr/bin/env python3
"""Test semantic boundaries even when modified packet hashes are refreshed."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/review-coverage-v1'
spec = importlib.util.spec_from_file_location('review_coverage_guard', ROOT / 'scripts/check-review-coverage.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class ReviewCoverageTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.inputs = guard.prepare.source_inputs()

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        path = self.packet / 'manifest.json'
        data = json.loads(path.read_text())
        data['artifacts'] = {p.relative_to(self.packet).as_posix(): guard.sha(p.read_bytes())
                             for p in self.packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        path.write_text(json.dumps(data))

    def change(self, name, action):
        path = self.packet / name
        data = json.loads(path.read_text())
        action(data)
        path.write_text(json.dumps(data, ensure_ascii=False))
        self.rehash()

    def reject(self, message):
        self.assertIn(message, guard.check(self.packet, check_references=False))

    def private(self, action):
        self.change('private-coverage.json', lambda d: action(d['cases'][0]))
        self.reject('review coverage accounting: private-coverage.json')

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_public_gold_leakage(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(expected_source_fact_draft='answer'))
        self.reject('review coverage accounting: public-inputs.json')

    def test_context_inflates_case_count(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(public_case_proposals=15))
        self.reject('review coverage accounting: coverage-summary.json')

    def test_wrong_module_denominator(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(module_labels=9))
        self.reject('review coverage accounting: coverage-summary.json')

    def test_replacement_reversed(self):
        self.change('coverage-summary.json', lambda d: d['historical_replacement'].update(replacement='SRC-026'))
        self.reject('review coverage accounting: coverage-summary.json')

    def test_material_count_becomes_independence(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(approved_independent_reviews=10))
        self.reject('review coverage accounting: coverage-summary.json')

    def test_source_hash_forgery(self):
        self.private(lambda r: r['source'].update(function_sha256='0' * 64))

    def test_rubric_span_forgery(self):
        self.private(lambda r: r['existing_ai_anchors'][0].update(start_byte=0))

    def test_resource_mapping_forgery(self):
        self.private(lambda r: r['categories_proposal']['resources_cost'].update(anchor_ids=['SRC-001-R99']))

    def test_priority_assigned_without_review(self):
        self.private(lambda r: r['categories_proposal']['priority'].update(assigned_level='P0'))

    def test_required_items_gold_claim(self):
        self.private(lambda r: r.update(required_items_gold=['all'], gold_exhaustive=True))

    def test_unexposed_holdout_claim(self):
        self.private(lambda r: r.update(exposure='fresh_holdout', split='FINAL'))

    def test_unapproved_context_removed(self):
        self.private(lambda r: r['context_supplement'].update(anchors=[], available=False))

    def test_scope_limit_removed(self):
        self.private(lambda r: r.update(resource_scope_limit='safe', existing_unknowns=[]))

    def test_typed_artifact_invented(self):
        self.private(lambda r: r.update(typed_report_artifact='report.json', diagram_artifact='diagram.svg'))

    def test_host_handoff_invented(self):
        self.private(lambda r: r.update(state_handoff_receipt='passed', host_execution_receipt='passed'))

    def test_family_pair_omitted(self):
        self.change('pairwise-review.json', lambda d: d.pop())
        self.reject('review coverage accounting: pairwise-review.json')

    def test_shared_file_becomes_independence(self):
        self.change('pairwise-review.json', lambda d: next(r for r in d if r['same_source_file']).update(semantic_independence='approved'))
        self.reject('review coverage accounting: pairwise-review.json')

    def test_human_table_priority_forgery(self):
        (self.packet / 'REVIEW-TABLE.txt').write_text('all priorities approved')
        self.rehash()
        self.reject('human review table')

    def test_execution_promotion(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.reject('formal state changed')

    def test_extra_artifact(self):
        (self.packet / 'extra.txt').write_text('not accounted')
        self.rehash()
        self.reject('unexpected artifact')

    def test_path_escape(self):
        path = self.packet / 'manifest.json'
        data = json.loads(path.read_text())
        data['artifacts']['../escape'] = '0' * 64
        path.write_text(json.dumps(data))
        self.assertTrue(any('unsafe source artifact' in e for e in guard.check(self.packet, check_references=False)))

    def test_duplicate_case_input(self):
        data = copy.deepcopy(self.inputs)
        data['public']['cases'].append(data['public']['cases'][0])
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            guard.prepare.generate(data)

    def test_unknown_category_anchor_input(self):
        data = copy.deepcopy(self.inputs)
        data['mapping'][0]['state_guard'].append(99)
        with self.assertRaisesRegex(ValueError, 'invalid category'):
            guard.prepare.generate(data)

    def test_inventory_properties(self):
        result = guard.prepare.generate(self.inputs)
        counts = result['coverage-summary.json']['counts']
        self.assertEqual((counts['public_case_proposals'], counts['module_labels'], counts['source_files']), (10, 8, 9))
        self.assertEqual((counts['existing_ai_rubric_anchors'], counts['existing_context_anchors']), (45, 19))
        self.assertEqual((counts['case_pairs'], counts['same_module_pairs'], counts['same_file_pairs']), (45, 2, 1))
        self.assertEqual(result['public-inputs.json'], self.inputs['public'])
        self.assertTrue(all(r['categories_proposal']['priority']['assigned_level'] is None for r in result['private-coverage.json']['cases']))
        self.assertNotIn('SRC-026', [r['candidate_id'] for r in result['private-coverage.json']['cases']])


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
