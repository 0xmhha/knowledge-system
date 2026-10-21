#!/usr/bin/env python3
"""Reject tampered raw evidence and promotion of synthetic branches to real failures."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/udp-cursor-probe-v1'
spec = importlib.util.spec_from_file_location('cursor_guard', ROOT / 'scripts/check-udp-cursor-probe.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class CursorProbeTests(unittest.TestCase):
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

    def change(self, update):
        p = self.packet / 'summary.json'
        d = json.loads(p.read_text())
        update(d)
        p.write_text(json.dumps(d))
        self.rehash()

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_original_closure_changed(self):
        p = self.packet / 'original-closure.go.txt'
        p.write_bytes(p.read_bytes() + b'// changed\n')
        self.rehash()
        self.rejected()

    def test_raw_go_failure(self):
        p = self.packet / 'logs/ORIGINAL-normal.jsonl'
        with p.open('a') as f:
            f.write(json.dumps({'Action': 'fail', 'Package': 'udpcursorfixture'}) + '\n')
        self.rehash()
        self.rejected()

    def test_summary_differs_from_raw(self):
        self.change(lambda d: d['observations'][0].update(remaining_elements=77))
        self.rejected()

    def test_host_clock_change_claim(self):
        self.change(lambda d: d.update(host_clock_changes=1))
        self.rejected()

    def test_real_timer_delivery_claim(self):
        self.change(lambda d: d.update(timer_scope='real timer firing verified'))
        self.rejected()

    def test_independent_family_claim(self):
        self.change(lambda d: d.update(accepted_independent_families=1))
        self.rejected()

    def test_production_patch_claim(self):
        self.change(lambda d: d.update(production_patch_applied=True))
        self.rejected()

    def test_missing_normal_case(self):
        self.change(lambda d: d['observations'].pop())
        self.rejected()

    def test_race_mode_missing(self):
        self.change(lambda d: d['runs'].pop())
        self.rejected()

    def test_hidden_quality_error(self):
        p = self.packet / 'logs/CURSOR-DRAFT-lint.stderr.txt'
        p.write_text('lint error\n')
        self.rehash()
        self.rejected()

    def test_operational_reachability_claim(self):
        self.change(lambda d: d.update(operational_reachability='verified'))
        self.rejected()

    def test_network_reproduction_claim(self):
        self.change(lambda d: d.update(network_calls=1))
        self.rejected()


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
