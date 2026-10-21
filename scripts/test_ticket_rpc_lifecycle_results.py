"""Finite lifecycle matching, cleanup separation and false-completion tests."""
import unittest
import gzip
import json
import shutil
import tempfile
from pathlib import Path

import ticket_rpc_lifecycle_results as g
import evidence_revision


def sample(returned=True, role='CAND-A', drain=True):
    kinds = [('install', 1), ('send_begin_progress', 1), ('send_end_progress', 1),
             ('send_begin_progress', 1), ('method_entry', 1)]
    if returned:
        kinds += [('send_end_progress', 1)]
        if drain:
            kinds += [('method_status_received_after_io', 1)]
        kinds += [(k, 1) for k in ['before_delete', 'after_delete', 'ack_close_begin',
                                   'ack_close_end', 'method_ack_observed', 'method_return_defer']]
    before_rescue = len(kinds)
    if returned:
        kinds += [(k, 2) for k in ['install', 'send_begin_progress', 'send_end_progress',
                                   'method_entry', 'before_delete', 'after_delete', 'ack_close_begin',
                                   'ack_close_end', 'method_ack_observed', 'method_return_defer']]
    final_count = len(kinds)
    if not returned:
        # Cleanup's controlled ack is intentionally outside all success prefixes.
        kinds += [('method_ack_observed', 1), ('method_return_defer', 1)]
    members, life = set(), []
    for i, (kind, channel) in enumerate(kinds, 1):
        owner = not kind.startswith('method_')
        if kind == 'install': members.add(channel)
        if kind == 'after_delete': members.remove(channel)
        life.append(dict(sequence=i, candidate=role, elapsed_ns=i, event=kind,
                         channel_id=channel, map_observed=owner,
                         membership=(channel in members) if owner else None,
                         member_ids=(sorted(members) or None) if owner else None))
    times = [0, 1, 2, 60_000_000_000, 120_000_000_000]
    times += [120_000_000_001 + i for i in range(5)]
    times += [180_000_000_000 + i for i in range(5)]
    events = [dict(event=kind, sequence=i, candidate=role, elapsed_ns=t)
              for i, (kind, t) in enumerate(zip(g.EVENTS, times), 1)]
    events[0].update(base_commit=g.prior.COMMIT, native_timer_seconds=60,
                     standalone_program=True,
                     source_lifecycle_hooks=True, temporary_instrumented_API=True,
                     original_package_tests_executed=False, independent_oracle=False,
                     external_listener=False)
    events[1].update(subscription_id='0xabcdef', caller_RPC_requests=1,
                     route='Server.ServeCodec_NewCodec_net.Pipe', network_sync=False, controlled_height=1)
    events[3].update(wire_notification_delivered=False, actual_notifier=True, owned_transport_gate=True)
    events[4].update(native_timer_fires=2, sole_RPC_consumer_in_Notify=True,
                     eventLoop_send_site='/eth/downloader/api.go:127')
    for index, label, count in [(5, 'before-close', 4), (9, 'before-rescue', before_rescue),
                                (11, 'final-before-rescue', final_count)]:
        events[index].update(label=label, through_sequence=count, before_rescue=True,
                              global_transactional_snapshot=False)
    events[6].update(write_gate_still_held=True, RPC_unsubscribe_response=False)
    events[8].update(callbacks=0 if returned else 1, rescue_started=False,
                     uninstrumented_API_map_directly_observed=False,
                     all_post_ack_channel_use_proved=False, independent_acceptable_candidate_set=None,
                     select_exit_vs_second_status_controlled=False,
                     eventLoop_send_pending=not returned, method_uninstall_send_pending=not returned)
    events[10].update(executed=returned, native_timer_fires=3 if returned else 2,
                      universal_future_nonuse_proved=False, probe_unsubscribed=returned,
                      observer_scope='instrumented_API_only')
    events[13].update(rescue_used=not returned, rescue_ack_is_controlled_not_real_map_ack=not returned,
                      callbacks=0, event_loops=0, ServeCodec_joined=True, memory_chain_stopped=True)
    events[14]['failed'] = False
    scope = dict(minimum_first_native_ns=60_000_000_000, minimum_second_native_ns=120_000_000_000,
                 minimum_third_native_ns=180_000_000_000, source_anchors={role: dict(broadcast=127)})
    return events, life, role, scope


class LifecycleResults(unittest.TestCase):
    def reject(self, change, returned=True, role='CAND-A'):
        args = sample(returned, role)
        change(*args)
        with self.assertRaises(ValueError): g.inspect_events(*args)

    def test_returned_candidate_actual_delete_ack_and_probe_are_finite(self):
        result = g.inspect_events(*sample())
        self.assertTrue(result['actual_owner_delete_ack_observed'])
        self.assertTrue(result['post_ack_native_probe_executed'])
        self.assertTrue(result['exit_method_drain_overlap_observed'])
        self.assertFalse(result['all_future_nonuse_proved'])
        self.assertFalse(result['receive_IO_linearization_proved'])
        self.assertFalse(result['independent_approved'])

    def test_original_can_return_by_uncontrolled_second_status_selection(self):
        result = g.inspect_events(*sample(True, 'ORIGINAL', False))
        self.assertTrue(result['select_consumed_second_status_before_exit'])
        self.assertFalse(result['exit_method_drain_overlap_observed'])

    def test_original_paired_wait_cleanup_ack_does_not_become_owner_proof(self):
        result = g.inspect_events(*sample(False, 'ORIGINAL'))
        self.assertTrue(result['original_paired_wait_observed'])
        self.assertFalse(result['actual_owner_delete_ack_observed'])
        self.assertFalse(result['post_ack_native_probe_executed'])
        self.assertEqual(result['cleanup_records_excluded_from_success'], 2)

    def test_result_survives_JSON_roundtrip_for_pending_tuple_and_integer_keys(self):
        for args in (sample(), sample(False, 'ORIGINAL')):
            result = g.inspect_events(*args)
            self.assertEqual(json.loads(json.dumps(result)), result)

    def test_cleanup_controlled_ack_cannot_be_included_in_finite_success_prefix(self):
        self.reject(lambda e, l, r, s: e[11].update(through_sequence=len(l)), False, 'ORIGINAL')

    def test_candidate_cannot_be_silently_relabelled_as_original_paired_wait(self):
        with self.assertRaises(ValueError): g.inspect_events(*sample(False, 'CAND-A'))

    def test_unexecuted_probe_cannot_be_promoted(self):
        self.reject(lambda e, l, r, s: e[10].update(executed=True), False, 'ORIGINAL')

    def test_returned_path_requires_its_native_probe(self):
        self.reject(lambda e, l, r, s: e[10].update(executed=False))

    def test_shortened_native_timer_cannot_pass(self):
        self.reject(lambda e, l, r, s: e[3].update(elapsed_ns=59_000_000_000))

    def test_probe_without_third_native_timer_cannot_pass(self):
        self.reject(lambda e, l, r, s: e[10].update(native_timer_fires=2))

    def test_wrong_native_blocked_source_site_rejected(self):
        self.reject(lambda e, l, r, s: e[4].update(eventLoop_send_site='/eth/downloader/api.go:119'))

    def test_future_or_out_of_population_fence_rejected(self):
        self.reject(lambda e, l, r, s: e[11].update(through_sequence=len(l)+1))
        self.reject(lambda e, l, r, s: e[5].update(through_sequence=True))

    def test_wrong_rpc_or_lifecycle_candidate_rejected(self):
        self.reject(lambda e, l, r, s: e[7].update(candidate='OTHER'))
        self.reject(lambda e, l, r, s: l[0].update(candidate='OTHER'))

    def test_reordered_rpc_events_rejected(self):
        self.reject(lambda e, l, r, s: e[7].update(sequence=99))

    def test_uninstrumented_or_universal_proof_claim_rejected(self):
        self.reject(lambda e, l, r, s: e[8].update(uninstrumented_API_map_directly_observed=True))
        self.reject(lambda e, l, r, s: e[8].update(all_post_ack_channel_use_proved=True))
        self.reject(lambda e, l, r, s: e[10].update(universal_future_nonuse_proved=True))

    def test_independent_candidate_selection_not_invented(self):
        self.reject(lambda e, l, r, s: e[8].update(independent_acceptable_candidate_set=['CAND-A']))

    def test_controlled_select_order_claim_rejected(self):
        self.reject(lambda e, l, r, s: e[8].update(select_exit_vs_second_status_controlled=True))

    def test_cleanup_or_trace_failure_cannot_pass(self):
        self.reject(lambda e, l, r, s: e[13].update(event_loops=1))
        self.reject(lambda e, l, r, s: e[14].update(failed=True))

    def test_returned_candidate_cannot_require_controlled_rescue(self):
        self.reject(lambda e, l, r, s: e[13].update(rescue_used=True))

    def test_visible_acks_do_not_allow_send_on_deleted_original_channel(self):
        def change(e, l, r, s):
            next(row for row in l if row['event'] == 'send_begin_progress' and row['channel_id'] == 2)['channel_id'] = 1
        self.reject(change)

    def test_no_lifecycle_activity_after_trace_termination(self):
        self.reject(lambda e, l, r, s: l[-1].update(elapsed_ns=e[-1]['elapsed_ns']+1))

    def test_other_rpc_route_or_subscription_identity_rejected(self):
        self.reject(lambda e, l, r, s: e[1].update(route='synthetic'))
        self.reject(lambda e, l, r, s: e[1].update(subscription_id='unbound'))

    def test_returned_callback_cannot_still_be_in_recorded_paired_wait(self):
        self.reject(lambda e, l, r, s: e[8].update(method_uninstall_send_pending=True))


@unittest.skipUnless((g.ROOT / g.PACKET / 'manifest.json').is_file(), 'actual lifecycle capture not available yet')
class ActualCapturedResults(unittest.TestCase):
    def mutant(self, name, change, compressed=False):
        with tempfile.TemporaryDirectory() as tmp:
            packet = Path(tmp) / 'packet'
            shutil.copytree(g.ROOT / g.PACKET, packet)
            target = packet / name
            value = json.loads(gzip.decompress(target.read_bytes()) if compressed else target.read_bytes())
            change(value)
            data = json.dumps(value).encode()
            target.write_bytes(gzip.compress(data, mtime=0) if compressed else data)
            g.seal(packet)
            with self.assertRaises(ValueError): g.validate(packet=packet)

    def test_actual_full_capture_and_finite_scope(self):
        result = g.validate()
        self.assertEqual(result['summary']['actual_attempts'], 6)
        self.assertEqual(result['summary']['process_groups_remaining'], 0)
        self.assertFalse(result['summary']['universal_future_nonuse_proved'])
        self.assertEqual(result['summary']['formal_completed'], 0)

    def test_resealed_actual_source_identity_change_rejected(self):
        self.mutant('actual/candidates/CAND-A/source-binding.json', lambda v: v.update(api_sha256='0'*64))

    def test_resealed_summary_promotion_rejected(self):
        self.mutant('SUMMARY.json', lambda v: v.update(universal_future_nonuse_proved=True))

    def test_resealed_worklist_false_completion_rejected(self):
        self.mutant('WORKLIST.json', lambda v: v.update(formal_completed=1))

    def test_resealed_hidden_retry_claim_rejected(self):
        self.mutant('actual/receipt.json', lambda v: v.update(automatic_retries=1))

    def test_resealed_requirement_completion_rejected(self):
        self.mutant('requirements.json', lambda v: v[0].update(full_requirement_satisfied=True))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
