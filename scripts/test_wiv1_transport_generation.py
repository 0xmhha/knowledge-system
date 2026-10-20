"""Reject transport-close or same-channel timer success promoted from altered observations."""
import gzip
import json
import tempfile
import unittest
from pathlib import Path
import wiv1_transport_generation as x


class Evidence(unittest.TestCase):
    def mutate(self, name, change):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp)
            for source in (x.ROOT/x.PACKET).rglob('*'):
                if source.is_file() and source.name != 'manifest.json':
                    target = p/source.relative_to(x.ROOT/x.PACKET)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.symlink_to(source)
            target = p/name
            value = x.read(target)
            change(value)
            target.unlink()
            data = json.dumps(value).encode()
            target.write_bytes(gzip.compress(data, mtime=0) if target.suffix == '.gz' else data)
            x.seal(p)
            self.assertTrue(x.check(p))

    def trace(self, slot):
        return [json.loads(line) for line in x.raw(x.ROOT/x.PACKET/'actual/slots'/slot/'events.jsonl.gz').splitlines()]

    def rejects(self, slot, schedule, role, change):
        rows = self.trace(slot)
        change(rows)
        with self.assertRaises(ValueError):
            x.inspect_trace(rows, schedule, role)

    def test_actual_matrix_and_scope(self):
        self.assertEqual(x.check(), [])
        d = x.derive(x.ROOT/x.PACKET)
        self.assertEqual((d['actual_slots'], d['actual_owned_nodes'], d['post_ack_same_generation_uses']), (16, 32, 0))
        self.assertEqual(d['stale_membership_at_reregistration'], {'ORIGINAL': 0, 'CAND-A': 0, 'CAND-B': 0, 'MUT-STALE-ACK': 2})
        self.assertEqual(sum(d['transport_callback_exit_routes'].values()), 8)
        self.assertEqual(d['notifier_closed_branch_reached'], d['transport_callback_exit_routes']['callback_exit_transport_closed'] > 0)
        self.assertFalse(d['universal_post_ack_claim'] or d['full12_acceptance'])

    def test_missing_slot_not_complete(self):
        self.mutate('actual/attempts.json.gz', lambda d: d.pop())

    def test_shortened_timer_not_native(self):
        self.mutate('actual/SCOPE.json.gz', lambda d: d.update(native_timer_seconds=1))

    def test_unpreflighted_adapter_refused(self):
        self.mutate('actual/SCOPE.json.gz', lambda d: d.update(adapter_module_sha256='0'*64))

    def test_formal_acceptance_not_inferred(self):
        self.mutate('SUMMARY.json', lambda d: d.update(formal_completed=1, independent_approved=1, universal_post_ack_claim=True))

    def test_hidden_stale_detection_not_promoted(self):
        self.mutate('SUMMARY.json', lambda d: d.update(stale_control_detected_by_channel_use=2))

    def test_upstream_source_change_detected(self):
        self.mutate('actual/original-after.json.gz', lambda d: d.pop(next(iter(d))))

    def test_post_ack_use_cannot_be_hidden(self):
        def change(rows):
            use = next(r for r in rows if r['channel_use'] and r['generation'] == 2)
            use['generation'] = 1
        self.rejects('timer-same-channel-MUT-STALE-ACK-general', 'timer-same-channel', 'MUT-STALE-ACK', change)

    def test_stale_membership_cannot_be_cleared(self):
        def change(rows):
            next(r for r in rows if r['event'] == 'before_install' and r['generation'] == 1)['member'] = False
        self.rejects('timer-same-channel-MUT-STALE-ACK-race', 'timer-same-channel', 'MUT-STALE-ACK', change)

    def test_transport_close_not_relabelled_rpc_unsubscribe(self):
        def change(rows):
            next(r for r in rows if r['event'] == 'actual_transport_closed')['event'] = 'actual_RPC_unsubscribe_true'
        self.rejects('transport-CAND-A-general', 'transport', 'CAND-A', change)

    def test_broadcast_to_stale_generation_cannot_be_relabelled(self):
        def change(rows):
            old = next(r for r in rows if r['event'] == 'selected_factory_subscription')['channel_id']
            send = next(r for r in rows if r['event'] == 'native_send_begin_done' and r['channel_id'] == old)
            send['generation'] = 1
        self.rejects('timer-same-channel-CAND-B-general', 'timer-same-channel', 'CAND-B', change)

    def test_rescue_before_frozen_observation_rejected(self):
        def change(rows):
            frozen = x.one(rows, 'transport_observation_frozen')
            rescue = next(r for r in rows if r['event'] == 'owned_rescue_status_receive_begin')
            frozen['event'], rescue['event'] = rescue['event'], frozen['event']
        self.rejects('transport-ORIGINAL-race', 'transport', 'ORIGINAL', change)


if __name__ == '__main__':
    unittest.main()
