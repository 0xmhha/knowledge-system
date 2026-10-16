#!/usr/bin/env python3
"""Reject source substitution, independence inflation and unbounded schedule drafts."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/schedule-candidates-v1'
spec = importlib.util.spec_from_file_location('schedule_guard', ROOT / 'scripts/check-schedule-candidates.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class ScheduleCandidateTests(unittest.TestCase):
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
        p.write_text(json.dumps(d))
        self.rehash()

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_rebound_source_still_rejected_by_commit_proof(self):
        p = self.packet / 'git-proof.json'
        d = json.loads(p.read_text())
        record = d['files']['p2p/discover/v4_udp.go']
        f = self.packet / record['artifact']
        raw = f.read_bytes() + b'// replacement\n'
        f.write_bytes(raw)
        record['sha256'] = hashlib.sha256(raw).hexdigest()
        record['git_blob'] = hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()
        p.write_text(json.dumps(d))
        self.rehash()
        self.rejected()

    def test_public_private_field_leak(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(primary_expectation_draft='private'))
        self.rejected()

    def test_candidate_approved(self):
        self.change('private-schedules.json', lambda d: d['families'][0].update(approved=True))
        self.rejected()

    def test_five_independent_families_claim(self):
        self.change('private-schedules.json', lambda d: d.update(accepted_independent_families=5))
        self.rejected()

    def test_zero_baseline_claimed_pass(self):
        self.change('private-schedules.json', lambda d: d['sampling_proposal'].update(zero_or_insufficient_baseline='pass'))
        self.rejected()

    def test_unbounded_schedule(self):
        self.change('private-schedules.json', lambda d: d['families'][0]['schedules'][0].update(step_deadline_ms=0))
        self.rejected()

    def test_missing_owned_cleanup(self):
        self.change('private-schedules.json', lambda d: d['families'][0]['schedules'][0].update(cleanup=[]))
        self.rejected()

    def test_clock_rollback_claim(self):
        self.change('toolchain-notes.json', lambda d: d.update(clock_rollback_reproduced=True))
        self.rejected()

    def test_difference_does_not_approve_independence(self):
        self.change('private-schedules.json', lambda d: d['pairwise_overlap_review'][0].update(independence_status='approved'))
        self.rejected()

    def test_final_allocation(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(split='FINAL'))
        self.rejected()

    def test_project_defect_claim(self):
        self.change('private-schedules.json', lambda d: d['families'][0].update(verified_project_defect=True))
        self.rejected()

    def test_duplicate_family(self):
        self.change('private-schedules.json', lambda d: d['families'].append(d['families'][0]))
        self.rejected()

    def test_wrong_source_anchor(self):
        self.change('private-schedules.json', lambda d: d['families'][0]['source_anchors'][0].update(start_line=1))
        self.rejected()

    def test_formal_state_changed(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected()

    def test_contract_question_erased(self):
        self.change('private-schedules.json', lambda d: d['families'][0].update(contract_questions=[]))
        self.rejected()


if __name__ == '__main__':
    unittest.main()
