#!/usr/bin/env python3
"""Rehashed final packets cannot promote readiness to adoption or acceptance."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('t22_check', ROOT / 'scripts/check-consumer-final.py')
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)


class Final(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='ks-t22-guard-')
        self.p = Path(self.tmp.name) / 'packet'
        shutil.copytree(ROOT / c.p.PACKET, self.p)

    def tearDown(self):
        self.tmp.cleanup()

    def edit(self, name, fn):
        path = self.p / name
        d = json.loads(path.read_text())
        fn(d)
        path.write_text(json.dumps(d, ensure_ascii=False, indent=2) + '\n')

    def reject(self):
        path = self.p / 'manifest.json'
        d = json.loads(path.read_text())
        d['artifacts'] = {f.relative_to(self.p).as_posix(): c.p.digest(f) for f in self.p.rglob('*') if f.is_file() and f != path}
        path.write_text(json.dumps(d) + '\n')
        self.assertTrue(c.check(self.p, False))

    def test_valid_and_read_only(self):
        before = {str(f.relative_to(self.p)): c.p.digest(f) for f in self.p.rglob('*') if f.is_file()}
        self.assertEqual(c.check(self.p, False), [])
        self.assertEqual(before, {str(f.relative_to(self.p)): c.p.digest(f) for f in self.p.rglob('*') if f.is_file()})

    def test_global_changed(self):
        self.edit('after.json', lambda d: d['global']['.claude/settings.json'].update(sha256='0' * 64))
        self.reject()

    def test_operational_pointer_changed(self):
        self.edit('after.json', lambda d: d['operations']['metadata'].clear())
        self.reject()

    def test_maintained_source_changed(self):
        for name in ['before.json', 'after.json']:
            self.edit(name, lambda d: d['sources']['maintained_files']['go.mod'].update(sha256='0' * 64))
        self.reject()

    def test_global_contents_leaked(self):
        for name in ['before.json', 'after.json']:
            self.edit(name, lambda d: d['global']['.claude/settings.json'].update(contents='should never be published'))
        self.reject()

    def test_original_checkout_dirty(self):
        for name in ['before.json', 'after.json']:
            self.edit(name, lambda d: d['sources']['repos']['original_postrun'].update(porcelain='M plugin/agents/analyzer.md'))
        self.reject()

    def test_candidate_unsigned(self):
        for name in ['before.json', 'after.json']:
            self.edit(name, lambda d: d['sources']['repos']['consumer_postrun'].update(signature_exit=1))
        self.reject()

    def test_remote_refs_changed(self):
        self.edit('after.json', lambda d: d['sources'].update(root_remote_refs='changed'))
        self.reject()

    def test_existing_engine_marked_owned(self):
        for name in ['before.json', 'after.json']:
            self.edit(name, lambda d: d['operations']['engines'][0].update(owned_by_T22=True))
        self.reject()

    def test_restore_not_exact(self):
        self.edit('restored.json', lambda d: d['state']['cks.yaml'].update(sha256='0' * 64))
        self.reject()

    def test_no_mutation_control(self):
        self.edit('restore-control.json', lambda d: d.update(mutated=d['original']))
        self.reject()

    def test_symlink_escape_allowed(self):
        self.edit('restore-control.json', lambda d: d['boundary_controls'][1].update(refused_before_mutation=False))
        self.reject()

    def test_backup_corrupted(self):
        (self.p / 'backup/source/logic.go.txt').write_text('wrong source')
        self.reject()

    def test_query_after_restore_changed_index(self):
        self.edit('restored.json', lambda d: d['after_query']['graph/graph.db'].update(sha256='0' * 64))
        self.reject()

    def test_query_request_pin_changed(self):
        self.edit('restored.json', lambda d: d['query'].update(expected_commit='0' * 40))
        self.reject()

    def test_raw_query_response_omitted(self):
        path = self.p / 'query/stdout.jsonl'
        path.write_text(path.read_text().splitlines()[0] + '\n')
        self.reject()

    def test_query_unreaped(self):
        self.edit('query/lifecycle.json', lambda d: d.update(child_reaped=False))
        self.reject()

    def test_query_forced_cleanup(self):
        self.edit('query/lifecycle.json', lambda d: d.update(kill_sent=True))
        self.reject()

    def test_live_owned_child(self):
        self.edit('cleanup.json', lambda d: d.update(matching_identity_alive=True))
        self.reject()

    def test_signal_other_session(self):
        self.edit('cleanup.json', lambda d: d.update(other_sessions_signalled=1))
        self.reject()

    def test_listener_not_closed(self):
        self.edit('listener.json', lambda d: d.update(socket_fileno_after_close=5))
        self.reject()

    def test_listener_no_positive_control(self):
        self.edit('listener.json', lambda d: d.update(before_connect_errno=61))
        self.reject()

    def test_historical_pid_as_exit_receipt(self):
        self.edit('historical-process-audit.json', lambda d: d.update(PID_absence_is_exit_receipt=True))
        self.reject()

    def test_historical_pid_omitted(self):
        self.edit('historical-process-audit.json', lambda d: d['probes'].pop())
        self.reject()

    def test_packet_commit_omitted(self):
        self.edit('commit-boundary.json', lambda d: d['packet_commits'].pop(next(iter(d['packet_commits']))))
        self.reject()

    def test_packet_commit_wrong(self):
        self.edit('commit-boundary.json', lambda d: d['packet_commits'].update({next(iter(d['packet_commits'])): '0' * 40}))
        self.reject()

    def test_remote_contact(self):
        self.edit('commit-boundary.json', lambda d: d.update(remote_contact_performed=True))
        self.reject()

    def test_formal_state_changed(self):
        self.edit('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.reject()

    def test_case_omitted(self):
        self.edit('case-matrix.json', lambda d: d.pop())
        self.reject()

    def test_handoff_omits_remaining(self):
        self.edit('handoff.json', lambda d: d.update(remaining_formal_ids=[]))
        self.reject()


def boundary_test(key, value):
    def test(self):
        self.edit('boundary.json', lambda d: d.update({key: value}))
        self.reject()
    return test


for name, key, value in [
    ('adoption_claim', 'candidate_adopted', True),
    ('formal_completion_claim', 'formal_completed', 9),
    ('ontology_acceptance_claim', 'ontology_effect_acceptance', 'accepted'),
    ('historical_global_claim', 'global_preservation_scope', 'all_historical_stages'),
    ('missing_exit_erased', 'historical_unconfirmed_CKS_exit_receipts', 0),
    ('production_verification_claim', 'operational_full_database_or_availability_verified', True),
    ('external_publication_claim', 'external_publication', 1),
    ('self_commit_circular_proof', 'self_commit_needs_post_creation_verification', False),
]:
    setattr(Final, 'test_' + name, boundary_test(key, value))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
