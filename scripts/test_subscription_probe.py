#!/usr/bin/env python3
"""Adversarial raw-evidence checks; these tests do not rerun the Go fixture."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/subscription-probe-v1'
spec = importlib.util.spec_from_file_location('probe_guard', ROOT / 'scripts/check-subscription-probe.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class ProbeEvidenceTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        p = self.packet / 'manifest.json'
        m = json.loads(p.read_text())
        m['artifacts'] = {p.relative_to(self.packet).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
                          for p in self.packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        p.write_text(json.dumps(m))

    def change_summary(self, update):
        p = self.packet / 'summary.json'
        d = json.loads(p.read_text())
        update(d)
        p.write_text(json.dumps(d))
        self.rehash()

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_accept_early_return_mutant(self):
        self.change_summary(lambda d: next(r for r in d['observations'] if r['variant'] == 'MUT-EARLY').update(satisfies_ai_fixture_criteria=True))
        self.rejected()

    def test_project_defect_promotion(self):
        self.change_summary(lambda d: d.update(verified_project_defect=True))
        self.rejected()

    def test_independent_candidate_set(self):
        self.change_summary(lambda d: d.update(independent_acceptable_candidate_set=['CAND-A', 'CAND-B']))
        self.rejected()

    def test_family_count_inflation(self):
        self.change_summary(lambda d: d.update(independent_families=5))
        self.rejected()

    def test_missing_observation(self):
        self.change_summary(lambda d: d['observations'].pop())
        self.rejected()

    def test_cleanup_claim_removed(self):
        self.change_summary(lambda d: d['observations'][0].update(all_owned_goroutines_stopped=False))
        self.rejected()

    def test_changed_compiled_method_and_rebound_summary(self):
        p = self.packet / 'compiled-inputs/ORIGINAL/variant.go.txt'
        p.write_bytes(p.read_bytes().replace(b's.api.uninstallSyncSubscription <- &req', b'// removed request send'))
        digest = hashlib.sha256(p.read_bytes()).hexdigest()
        self.change_summary(lambda d: d['compiled_inputs_sha256']['ORIGINAL'].update({'variant.go': digest}))
        self.rejected()

    def test_raw_go_failure_after_rehash(self):
        p = self.packet / 'logs/CAND-A-normal.jsonl'
        with p.open('a') as f:
            f.write(json.dumps({'Action': 'fail', 'Package': 'subscriptionfixture'}) + '\n')
        self.rehash()
        self.rejected()

    def test_changed_raw_observation_after_rehash(self):
        p = self.packet / 'logs/ORIGINAL-normal.jsonl'
        s = p.read_text().replace('two_sender_wait_witness\\\":true', 'two_sender_wait_witness\\\":false')
        self.assertNotEqual(s, p.read_text())
        p.write_text(s)
        self.rehash()
        self.rejected()

    def test_fabricated_wait_witness_even_if_raw_and_summary_agree(self):
        for p in (self.packet / 'logs').glob('*.jsonl'):
            s = p.read_text().replace('[chan send]', '[running]')
            p.write_text(s)
        self.change_summary(lambda d: [r.update(two_sender_wait_stacks=r['two_sender_wait_stacks'].replace('[chan send]', '[running]')) for r in d['observations']])
        self.rejected()

    def test_race_mode_omitted(self):
        self.change_summary(lambda d: d['runs'].pop())
        self.rejected()

    def test_quality_input_binding_changed(self):
        p = self.packet / 'quality.json'
        d = json.loads(p.read_text())
        d['compiled_inputs_sha256']['ORIGINAL']['variant.go'] = '0' * 64
        p.write_text(json.dumps(d))
        self.rehash()
        self.rejected()

    def test_lint_failure_hidden_in_receipt(self):
        p = self.packet / 'logs/quality-CAND-A-lint.stderr.txt'
        p.write_text('lint failed\n')
        self.rehash()
        self.rejected()

    def test_observer_failures_erased(self):
        p = self.packet / 'observer-diagnostics.json'
        d = json.loads(p.read_text())
        d['attempts'] = []
        p.write_text(json.dumps(d))
        self.rehash()
        self.rejected()


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
