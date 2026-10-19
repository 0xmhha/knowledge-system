"""Static source/observer contract tests; these do not execute the target program."""
import hashlib
import json
import unittest
from pathlib import Path

from lifecycle_instrumentation import instrument
from lifecycle_trace_contract import validate_prefix

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/followups/ticket-rpc-lifecycle-v1'


def trace(events):
    rows, members = [], set()
    for i, kind in enumerate(events, 1):
        observed = not kind.startswith('method_')
        if kind == 'install':
            members.add(1)
        if kind == 'after_delete':
            members.remove(1)
        rows.append(dict(sequence=i, elapsed_ns=i, candidate='ORIGINAL', event=kind,
                         channel_id=1, map_observed=observed,
                         membership=(1 in members) if observed else None,
                         member_ids=sorted(members) or None if observed else None))
    return rows


class LifecyclePreparation(unittest.TestCase):
    def test_three_frozen_instrumented_sources_and_actual_anchors(self):
        scope = json.loads((PACKET / 'SCOPE.json').read_text())
        for role in scope['candidate_roles']:
            original = (PACKET.parent / 'ticket-dev-gold-v1/private/overlays' / (role + '.go.txt')).read_bytes()
            output, anchors = instrument(original)
            self.assertEqual(hashlib.sha256(output).hexdigest(), scope['instrumented_api_sha256'][role])
            self.assertEqual(anchors, scope['source_anchors'][role])
            for event in ['install', 'before_delete', 'after_delete', 'ack_close_begin', 'ack_close_end']:
                self.assertEqual(output.count(('ksOwnedLifecycle("' + event + '"').encode()), 1)

    def test_all_three_owner_send_sites_are_instrumented(self):
        original = (PACKET.parent / 'ticket-dev-gold-v1/private/overlays/CAND-A.go.txt').read_bytes()
        output, _ = instrument(original)
        for kind in ['progress', 'done_false', 'install_false']:
            for edge in ['begin', 'end']:
                self.assertEqual(output.count(('"send_' + edge + '_' + kind + '"').encode()), 1)

    def test_unknown_source_anchor_refused(self):
        with self.assertRaises(ValueError):
            instrument(b'package downloader\n')

    def test_finite_valid_lifecycle_is_not_universal_or_independent(self):
        rows = trace(['install', 'method_entry', 'before_delete', 'after_delete',
                      'ack_close_begin', 'ack_close_end', 'method_ack_observed', 'method_return_defer'])
        result = validate_prefix(rows, 'ORIGINAL')
        self.assertEqual(result['installed_ids'], [])
        self.assertFalse(result['universal_future_nonuse_proved'])
        self.assertFalse(result['independent_approved'])

    def test_pending_source_defect_can_be_an_incomplete_prefix(self):
        result = validate_prefix(trace(['install', 'send_begin_progress', 'method_entry']), 'ORIGINAL')
        self.assertEqual(result['pending_send'], (1, 'progress'))
        self.assertEqual(result['installed_ids'], [1])

    def test_method_ack_can_log_before_owner_close_end(self):
        validate_prefix(trace(['install', 'method_entry', 'before_delete', 'after_delete',
                               'ack_close_begin', 'method_ack_observed', 'method_return_defer',
                               'ack_close_end']), 'ORIGINAL')

    def test_post_receive_hook_is_not_IO_linearization(self):
        validate_prefix(trace(['install', 'method_entry', 'send_begin_progress', 'send_end_progress',
                               'before_delete', 'after_delete', 'ack_close_begin',
                               'method_status_received_after_io', 'method_ack_observed',
                               'method_return_defer', 'ack_close_end']), 'ORIGINAL')

    def test_ack_before_delete_refused(self):
        with self.assertRaises(ValueError):
            validate_prefix(trace(['install', 'method_entry', 'ack_close_begin']), 'ORIGINAL')

    def test_owner_cannot_advance_while_send_is_pending(self):
        with self.assertRaises(ValueError):
            validate_prefix(trace(['install', 'send_begin_progress', 'before_delete']), 'ORIGINAL')

    def test_post_ack_send_refused(self):
        with self.assertRaises(ValueError):
            validate_prefix(trace(['install', 'before_delete', 'after_delete', 'ack_close_begin',
                                   'send_begin_progress']), 'ORIGINAL')

    def test_false_deleted_membership_refused(self):
        rows = trace(['install', 'before_delete', 'after_delete'])
        rows[-1].update(membership=True, member_ids=[1])
        with self.assertRaises(ValueError):
            validate_prefix(rows, 'ORIGINAL')

    def test_method_cannot_claim_owner_map_observation(self):
        rows = trace(['install', 'method_entry'])
        rows[-1].update(map_observed=True, membership=True, member_ids=[1])
        with self.assertRaises(ValueError):
            validate_prefix(rows, 'ORIGINAL')

    def test_return_without_actual_owner_ack_refused(self):
        with self.assertRaises(ValueError):
            validate_prefix(trace(['install', 'method_entry', 'method_return_defer']), 'ORIGINAL')

    def test_sequence_candidate_and_clock_tampering_refused(self):
        for field, value in [('sequence', 99), ('candidate', 'CAND-A'), ('elapsed_ns', -1)]:
            rows = trace(['install', 'method_entry'])
            rows[-1][field] = value
            with self.assertRaises(ValueError):
                validate_prefix(rows, 'ORIGINAL')

    def test_unmatched_send_completion_refused(self):
        with self.assertRaises(ValueError):
            validate_prefix(trace(['install', 'send_end_progress']), 'ORIGINAL')


if __name__ == '__main__':
    unittest.main()
