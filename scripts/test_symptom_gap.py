#!/usr/bin/env python3
"""Reject denominator, exposure and runtime-scope overclaims after rehashing."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/symptom-gap-v1'
spec = importlib.util.spec_from_file_location('symptom_gap_guard', ROOT / 'scripts/check-symptom-gap.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class SymptomGapTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.inputs = guard.prepare.inputs(PACKET)

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

    def change(self, name, action):
        p = self.packet / name
        d = json.loads(p.read_text())
        action(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.rehash()

    def reject(self, message):
        self.assertIn(message, guard.check(self.packet, check_references=False))

    def case(self, action, case_id=None):
        self.change('private-case-gaps.json', lambda d: action(next(r for r in d['cases'] if case_id is None or r['case_id'] == case_id)))
        self.reject('symptom gap accounting: private-case-gaps.json')

    def runtime(self, action):
        self.change('retrieval-path-review.json', action)
        self.reject('symptom gap accounting: retrieval-path-review.json')

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_twenty_cards_become_independent_families(self):
        self.change('gap-summary.json', lambda d: d['counts'].update(approved_independent_families=20))
        self.reject('symptom gap accounting: gap-summary.json')

    def test_group_gap_becomes_split_permission(self):
        self.change('gap-summary.json', lambda d: d.update(family_deficit_is_not_a_split_authorization=False))
        self.reject('symptom gap accounting: gap-summary.json')

    def test_family_reclassified_without_review(self):
        self.change('family-review.json', lambda d: d[2].update(independent_family_count=2, approved=True))
        self.reject('symptom gap accounting: family-review.json')

    def test_same_file_pair_hidden(self):
        self.change('pairwise-review.json', lambda d: next(r for r in d if r['same_source_file']).update(same_source_file=False))
        self.reject('symptom gap accounting: pairwise-review.json')

    def test_pair_omitted(self):
        self.change('pairwise-review.json', lambda d: d.pop())
        self.reject('symptom gap accounting: pairwise-review.json')

    def test_prior_comparison_omitted(self):
        self.change('prior-question-review.json', lambda d: d.pop())
        self.reject('symptom gap accounting: prior-question-review.json')

    def test_no_literal_match_becomes_independent(self):
        self.change('prior-question-review.json', lambda d: d[0].update(semantic_independence='approved'))
        self.reject('symptom gap accounting: prior-question-review.json')

    def test_no_topic_flag_becomes_fresh_holdout(self):
        self.case(lambda r: r['exposure'].update(fresh_holdout=True, semantic_decontamination='passed'), 'RQ-015')

    def test_source_file_exposure_removed(self):
        self.case(lambda r: r['exposure'].update(public_review_same_file_cases=[]), 'RQ-002')

    def test_source_span_forged(self):
        self.case(lambda r: r['source_anchors'][0].update(span_sha256='0' * 64))

    def test_unknown_contract_removed(self):
        self.case(lambda r: r.update(source_unknowns=[]))

    def test_gold_missing_becomes_empty(self):
        self.case(lambda r: r.update(gold_locations=[], exhaustive_relevance_labels=[]))

    def test_hypothetical_root_cause_confirmed(self):
        self.case(lambda r: r.update(root_cause_established=True))

    def test_parser_receipt_becomes_retrieval(self):
        self.case(lambda r: r['current_case_receipts'].update(semantic_search='passed'))

    def test_symbol_smoke_becomes_text_retrieval(self):
        self.runtime(lambda d: d['historical_real_graph_smoke'].update(current_symptom_text_requests_covered=20, seed_modes=['text_only']))

    def test_chainbench_probe_becomes_wi3_host_run(self):
        self.runtime(lambda d: d['later_isolated_host'].update(current_symptom_text_requests_covered=20))

    def test_adopted_source_becomes_installed_execution(self):
        self.runtime(lambda d: d['adopted_consumer'].update(installed_role_execution_established=True))

    def test_top10_is_assumed_vector_k10(self):
        self.runtime(lambda d: d.update(top_10_evaluation_target='vector_top10', cross_backend_cause_ranking='proven'))

    def test_recall_score_invented(self):
        self.runtime(lambda d: d.update(macro_recall_at_10=1.0, formal_runs=20))

    def test_aggregation_frozen_without_review(self):
        self.runtime(lambda d: d.update(case_vs_family_aggregation='one_card_one_family', canonical_matching='exact_line'))

    def test_public_gold_injection(self):
        self.change('public-bundle/wire-requests.json', lambda d: d['requests'][0]['arguments'].update(expected_source='private'))
        self.reject('symptom gap accounting: public-bundle/wire-requests.json')

    def test_consumer_git_source_tamper(self):
        p = self.packet / 'consumer-source/sources/plugin/agents/analyzer.md.txt'
        p.write_bytes(p.read_bytes()+b'\n')
        self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet, check_references=False)))

    def test_current_source_tamper(self):
        p = self.packet / 'current-source/internal/system/suspects/compose.go.txt'
        p.write_bytes(p.read_bytes()+b'\n')
        self.rehash()
        self.reject('current source binding: current-source/internal/system/suspects/compose.go.txt')

    def test_formal_execution_promotion(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.reject('formal state changed')

    def test_extra_private_public_file(self):
        (self.packet / 'public-bundle/gold.json').write_text('{}')
        self.rehash()
        self.reject('unexpected artifact')

    def test_git_object_text_normalization_policy_rejected(self):
        (self.packet / '.gitattributes').write_text('consumer-source/objects/* text\n')
        self.rehash()
        self.reject('Git object byte-preservation attributes')

    def test_duplicate_case_input(self):
        d = copy.deepcopy(self.inputs)
        d['public']['cases'].append(d['public']['cases'][0])
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            guard.prepare.generate(d)

    def test_inventory_properties(self):
        out = guard.prepare.generate(self.inputs)
        c = out['gap-summary.json']['counts']
        self.assertEqual((c['input_cards'], c['family_groups_proposed'], c['approved_independent_families']), (20, 15, 0))
        self.assertEqual((c['pairwise_comparisons'], c['same_group_pairs'], c['same_file_pairs']), (190, 5, 3))
        self.assertEqual((c['prior_question_comparisons'], c['prior_ai_topic_flags'], c['cards_with_prior_ai_topic_flags']), (840, 29, 16))
        self.assertEqual(out['public-bundle/wire-requests.json'], self.inputs['wire'])
        self.assertTrue(all(all(v is None for v in r['current_case_receipts'].values()) for r in out['private-case-gaps.json']['cases']))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
