#!/usr/bin/env python3
"""Adversarial tests for symptom-only inputs and unreviewed family accounting."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/symptom-inputs-v1'
spec = importlib.util.spec_from_file_location('symptom_guard', ROOT / 'scripts/check-symptom-inputs.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class SymptomInputTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['artifacts'] = {q.relative_to(self.packet).as_posix(): hashlib.sha256(q.read_bytes()).hexdigest()
                          for q in self.packet.rglob('*') if q.is_file() and q.name != 'manifest.json'}
        p.write_text(json.dumps(d))

    def change(self, name, update):
        p = self.packet / name
        d = json.loads(p.read_text())
        update(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.rehash()

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_symbol_hint_even_after_both_curations_change(self):
        for name, update in [('public-inputs.json', lambda d: d['cases'][0].update(prompt='DecodeRLP 오류의 위치를 알려줘.')), ('curation.json', lambda d: d[0].update(prompt='DecodeRLP 오류의 위치를 알려줘.'))]:
            self.change(name, update)
        self.rejected()

    def test_file_hint(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(prompt='consensus/wbft/messages/preprepare.go의 오류 반환을 찾아줘.'))
        self.rejected()

    def test_private_field_in_public(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(candidate_id='SRC-004'))
        self.rejected()

    def test_twenty_independent_families_claim(self):
        self.change('private-source-review.json', lambda d: d.update(approved_independent_families=20))
        self.rejected()

    def test_recall_score_claim(self):
        self.change('private-source-review.json', lambda d: d.update(macro_recall_at_10=0.9))
        self.rejected()

    def test_scoring_source_notes_as_gold(self):
        self.change('private-source-review.json', lambda d: d['reviews'][0].update(quality_scoring_allowed=True))
        self.rejected()

    def test_final_allocation(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(split='FINAL'))
        self.rejected()

    def test_split_correlated_group(self):
        self.change('private-source-review.json', lambda d: d['reviews'][3].update(family_group_proposal='G16'))
        self.rejected()

    def test_pairwise_independence_approved(self):
        self.change('private-source-review.json', lambda d: d['pairwise_overlap_review'][0].update(independence_status='approved'))
        self.rejected()

    def test_prior_comparison_missing(self):
        self.change('private-source-review.json', lambda d: d['prior_question_comparison'].pop())
        self.rejected()

    def test_prior_exact_mismatch_claims_semantic_independence(self):
        self.change('private-source-review.json', lambda d: d['prior_question_comparison'][0].update(semantic_independence='disjoint'))
        self.rejected()

    def test_source_substitution_after_rehash(self):
        p = self.packet / 'git-proof.json'
        d = json.loads(p.read_text())
        r = d['files']['rpc/endpoints.go']
        f = self.packet / r['artifact']
        raw = f.read_bytes() + b'// changed\n'
        f.write_bytes(raw)
        r.update(sha256=hashlib.sha256(raw).hexdigest(), git_blob=hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\0' + raw).hexdigest())
        p.write_text(json.dumps(d))
        self.rehash()
        self.rejected()

    def test_root_cause_from_ai_note(self):
        self.change('private-source-review.json', lambda d: d['reviews'][0].update(independent_root_cause='confirmed'))
        self.rejected()

    def test_retrieval_claim(self):
        self.change('private-source-review.json', lambda d: d.update(retrieval_runs=20))
        self.rejected()

    def test_anchor_line_mismatch(self):
        self.change('private-source-review.json', lambda d: d['reviews'][0]['source_anchors'][0].update(start_line=1))
        self.rejected()

    def test_exposed_input_promoted_to_final(self):
        self.change('private-source-review.json', lambda d: d['reviews'][0].update(final_eligibility='approved'))
        self.rejected()

    def test_duplicate_card(self):
        self.change('public-inputs.json', lambda d: d['cases'].append(d['cases'][0]))
        self.rejected()


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
