#!/usr/bin/env python3
"""Rehashed receipts must retain original failures, late replies and denominators."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('t16_check', ROOT / 'scripts/check-consumer-errors.py')
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)


class Errors(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='ks-t16-guard-')
        self.p = Path(self.tmp.name) / 'packet'
        shutil.copytree(ROOT / c.p.PACKET, self.p)

    def tearDown(self):
        self.tmp.cleanup()

    def edit(self, name, fn):
        path = self.p / name
        value = json.loads(path.read_text())
        fn(value)
        path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

    def reject(self):
        path = self.p / 'manifest.json'
        m = json.loads(path.read_text())
        m['artifacts'] = {f.relative_to(self.p).as_posix(): c.p.digest(f) for f in self.p.rglob('*') if f.is_file() and f != path}
        path.write_text(json.dumps(m) + '\n')
        self.assertTrue(c.check(self.p, False))

    def test_valid(self):
        before = {str(f.relative_to(self.p)): c.p.digest(f) for f in self.p.rglob('*') if f.is_file()}
        self.assertEqual(c.check(self.p, False), [])
        self.assertEqual(before, {str(f.relative_to(self.p)): c.p.digest(f) for f in self.p.rglob('*') if f.is_file()})

    def test_timeout_omitted(self):
        self.edit('sessions/timeout/attempts.json', lambda d: d.pop(1))
        self.reject()

    def test_timeout_promoted_to_success(self):
        self.edit('sessions/timeout/attempts.json', lambda d: d[1].update(outcome='response'))
        self.reject()

    def test_late_response_omitted(self):
        self.edit('sessions/timeout/faults.json', lambda d: d.update(late_responses=[]))
        self.reject()

    def test_late_wrong_id(self):
        self.edit('sessions/timeout/faults.json', lambda d: d['late_responses'][0].update(id=3))
        self.reject()

    def test_signal_owner_changed(self):
        self.edit('sessions/timeout/faults.json', lambda d: d['signals'][0].update(target_pid=1))
        self.reject()

    def test_signal_birth_changed(self):
        self.edit('sessions/timeout/faults.json', lambda d: d['signals'][1]['identity'].update(birth='other process'))
        self.reject()

    def test_stop_not_observed(self):
        self.edit('sessions/timeout/faults.json', lambda d: d['signals'][0].update(observed_state='S'))
        self.reject()

    def test_startup_exit_promoted(self):
        self.edit('sessions/startup/lifecycle.json', lambda d: d.update(exit_code=0))
        self.reject()

    def test_unreaped_child(self):
        self.edit('sessions/provider/lifecycle.json', lambda d: d.update(child_reaped=False))
        self.reject()

    def test_cleanup_kill_hidden(self):
        self.edit('sessions/main/lifecycle.json', lambda d: d.update(kill_sent=True))
        self.reject()

    def test_reader_not_joined(self):
        self.edit('sessions/main/lifecycle.json', lambda d: d.update(reader_joined=False))
        self.reject()

    def test_raw_input_missing(self):
        path = self.p / 'sessions/main/stdin.jsonl'
        path.write_text('\n'.join(path.read_text().splitlines()[:-1]) + '\n')
        self.reject()

    def test_raw_late_response_missing(self):
        path = self.p / 'sessions/timeout/stdout.jsonl'
        value = path.read_text().splitlines()
        path.write_text(value[0] + '\n' + value[-1] + '\n')
        self.reject()

    def test_messages_replace_attempts(self):
        self.edit('sessions/timeout/messages.json', lambda d: d.append({'label': 'timeout', 'response': {'result': 'success'}}))
        self.reject()

    def test_config_changed(self):
        path = self.p / 'configs/provider.yaml'
        path.write_text(path.read_text().replace('provider: ollama', 'provider: mock'))
        self.reject()

    def test_provider_diagnostic_missing(self):
        (self.p / 'sessions/provider/stderr.txt').write_text('')
        self.reject()

    def test_provider_no_fault(self):
        self.edit('fault-setup.json', lambda d: d.update(probe_connect_errno=0))
        self.reject()

    def test_corrupt_index_replaced(self):
        (self.p / 'indexes/broken.db').write_bytes(b'normal')
        self.reject()

    def test_process_audit_empty(self):
        self.edit('postrun-process-audit.json', lambda d: d.update(probes=[]))
        self.reject()

    def test_ps_positive_control_missing(self):
        self.edit('postrun-process-audit.json', lambda d: d['self'].update(exit_code=1))
        self.reject()

    def test_formal_state_changed(self):
        self.edit('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.reject()

    def test_old_packet_preservation_changed(self):
        self.edit('preservation.json', lambda d: d.update(prior_packets=0))
        self.reject()

    def test_source_checkout_changed(self):
        self.edit('integrity.json', lambda d: d['checkouts']['cks_postrun'].update(porcelain='M runtime.go'))
        self.reject()

    def test_runtime_pin_changed(self):
        self.edit('execution.json', lambda d: d.update(binary_sha256='0' * 64))
        self.reject()


def accounting_test(key, value):
    def test(self):
        self.edit('errors.json', lambda d: d.update({key: value}))
        self.reject()
    return test


for name, key, value in [
    ('drop_errors_from_denominator', 'scenario_denominator', 2),
    ('drop_timeout_from_tools', 'tool_evaluation_denominator', 6),
    ('late_response_as_success', 'supported_partial_results', 3),
    ('hide_unusable_cases', 'nonusable_scenarios', 0),
    ('complete_analysis_claim', 'complete_analysis_results', 2),
    ('quality_100_percent_claim', 'quality_precision_recall_safety', {'precision': 1}),
    ('ontology_acceptance_claim', 'ontology_effect_acceptance', 'accepted'),
    ('host_validation_claim', 'actual_role_sessions', 1),
    ('adoption_claim', 'candidate_adopted', True),
    ('formal_completion_claim', 'formal_completed', 9),
    ('remaining_work_hidden', 'full_plan_pending', []),
    ('immediate_refusal_claim', 'provider_connection_observed', 'connection_refused'),
    ('provider_confound_hidden', 'provider_alignment_confounded', False),
]:
    setattr(Errors, 'test_' + name, accounting_test(key, value))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
