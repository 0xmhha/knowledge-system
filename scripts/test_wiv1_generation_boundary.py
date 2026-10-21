"""Reject generation-boundary success promoted from altered or rescued observations."""
import gzip
import json
import tempfile
import unittest
from pathlib import Path
import wiv1_generation_boundary as x


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
        self.assertEqual((d['actual_slots'], d['actual_owned_nodes']), (16, 32))
        self.assertEqual(d['post_ack_same_generation_uses_non_control'], 0)
        self.assertEqual(d['violation_control_detections'], 2)
        self.assertFalse(d['universal_post_ack_claim'] or d['full12_acceptance'])

    def test_missing_slot_not_complete(self):
        self.mutate('actual/attempts.json.gz', lambda d: d.pop())

    def test_shortened_timer_not_native(self):
        self.mutate('actual/SCOPE.json.gz', lambda d: d.update(native_timer_seconds=1))

    def test_formal_acceptance_not_inferred(self):
        self.mutate('SUMMARY.json', lambda d: d.update(formal_completed=1, independent_approved=1, universal_post_ack_claim=True))

    def test_failed_adapter_preflight_blocks_packet(self):
        self.mutate('adapter/PREFLIGHT.json', lambda d: d['checks']['upstream_source'].update(ok=False))

    def test_upstream_source_change_detected(self):
        self.mutate('actual/original-after.json.gz', lambda d: d.pop(next(iter(d))))

    def test_post_ack_flag_cannot_be_cleared(self):
        def change(rows):
            for r in rows:
                if r['post_ack_same_generation_use']:
                    r['post_ack_same_generation_use'] = False
        self.rejects('timer-MUT-STALE-ACK-general', 'timer', 'MUT-STALE-ACK', change)

    def test_generation_cannot_be_relabelled(self):
        def change(rows):
            next(r for r in rows if r['event'] == 'probe_status_received')['generation'] = 1
        self.rejects('overlap-CAND-A-general', 'overlap', 'CAND-A', change)

    def test_wire_true_not_owner_ack(self):
        def change(rows):
            ack = next(r for r in rows if r['event'] == 'ack_close_end')
            ack['event'] = 'ack_close_end_removed'
        self.rejects('timer-CAND-B-general', 'timer', 'CAND-B', change)

    def test_rescue_before_frozen_observation_rejected(self):
        def change(rows):
            frozen = x.one(rows, 'overlap_observation_frozen')
            rescue = next(r for r in rows if r['event'] == 'owned_rescue_status_receive_begin')
            frozen['event'], rescue['event'] = rescue['event'], frozen['event']
        self.rejects('overlap-ORIGINAL-general', 'overlap', 'ORIGINAL', change)

    def test_transport_close_not_relabelled_rpc_error(self):
        def change(rows):
            next(r for r in rows if r['event'] == 'callback_exit_rpc_error')['event'] = 'callback_exit_transport_closed'
        self.rejects('overlap-CAND-B-race', 'overlap', 'CAND-B', change)


if __name__ == '__main__':
    unittest.main()
