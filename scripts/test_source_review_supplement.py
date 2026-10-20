#!/usr/bin/env python3
"""Adversarial checks for source preparation; no target project execution."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/expansion-v1'
spec = importlib.util.spec_from_file_location('supplement_guard', ROOT / 'scripts/check-source-review-supplement.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class DraftGuardTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.packet = Path(self.tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def change(self, name, update):
        path = self.packet / name
        value = json.loads(path.read_text())
        update(value)
        path.write_text(json.dumps(value, ensure_ascii=False))
        manifest = self.packet / 'manifest.json'
        m = json.loads(manifest.read_text())
        m['artifacts'][name] = hashlib.sha256(path.read_bytes()).hexdigest()
        manifest.write_text(json.dumps(m))

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_private_approval_rejected_even_after_rehash(self):
        self.change('private-ai-review.json', lambda d: d['review_drafts'][0].update(approved=True))
        self.rejected()

    def test_private_gold_scoring_rejected(self):
        self.change('private-ai-review.json', lambda d: d['review_drafts'][0].update(precision_recall_scoring_allowed=True))
        self.rejected()

    def test_runtime_claim_rejected(self):
        self.change('private-ai-review.json', lambda d: d.update(runtime_executions=1))
        self.rejected()

    def test_public_private_field_leak_rejected(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(relevance_labels=['secret']))
        self.rejected()

    def test_final_allocation_rejected(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(split='FINAL'))
        self.rejected()

    def test_source_replacement_rejected_even_after_rehash(self):
        d = json.loads((self.packet / 'public-inputs.json').read_text())
        path = d['cases'][0]['source_artifact']
        p = self.packet / path
        p.write_bytes(p.read_bytes() + b'// modified\n')
        self.change('public-inputs.json', lambda d: None)
        mpath = self.packet / 'manifest.json'
        m = json.loads(mpath.read_text())
        m['artifacts'][path] = hashlib.sha256(p.read_bytes()).hexdigest()
        mpath.write_text(json.dumps(m))
        self.rejected()

    def test_anchor_line_mismatch_rejected(self):
        self.change('private-ai-review.json', lambda d: d['review_drafts'][0]['anchors'][0].update(start_line=1))
        self.rejected()

    def test_missing_pair_rejected(self):
        self.change('private-ai-review.json', lambda d: d['review_drafts'].pop())
        self.rejected()

    def test_stress_scope_promotion_rejected(self):
        self.change('public-inputs.json', lambda d: next(c for c in d['cases'] if c['candidate_id'] == 'SRC-026').update(scope='production_source_candidate'))
        self.rejected()

    def test_formal_freeze_change_rejected(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected()

    def test_manifest_count_inflation_rejected(self):
        p = self.packet / 'manifest.json'
        m = json.loads(p.read_text())
        m['counts']['production_candidates'] = 20
        p.write_text(json.dumps(m))
        self.rejected()


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
