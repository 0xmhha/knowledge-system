"""Reject fixture success promoted from incomplete or rescued observations."""
import gzip
import json
import tempfile
import unittest
from pathlib import Path
import factory_native_overlap as x


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

    def test_actual_matrix_and_scope(self):
        self.assertEqual(x.check(), [])
        d = x.derive(x.ROOT/x.PACKET)
        self.assertEqual((d['actual_slots'], d['actual_owned_nodes']), (10, 20))
        self.assertFalse(d['full12_acceptance'])

    def test_missing_native_slot_not_complete(self):
        self.mutate('actual/attempts.json.gz', lambda d: d.pop())

    def test_changed_timer_scope_not_native(self):
        self.mutate('actual/SCOPE.json.gz', lambda d: d.update(native_timer_seconds=1))

    def test_formal_acceptance_not_inferred(self):
        self.mutate('SUMMARY.json', lambda d: d.update(formal_completed=1, independent_approved=1, full12_acceptance=True))

    def test_cleanup_receipt_not_assumed_when_source_changed(self):
        self.mutate('actual/original-after.json.gz', lambda d: d.pop(next(iter(d))))

    def test_real_sync_not_replaced_by_mux_injection(self):
        self.mutate('actual/SCOPE.json.gz', lambda d: d.update(actual_source_sync_not_injected=False))

    def trace_mutation(self, change):
        p = x.ROOT/x.PACKET/'actual/slots/CAND-A-general/events.jsonl.gz'
        rows = [json.loads(line) for line in x.raw(p).splitlines()]
        change(rows)
        with self.assertRaises(ValueError):
            x.inspect_trace(rows, 'CAND-A')

    def test_actual_RPC_exit_not_relabelled_transport_close(self):
        def change(rows):
            x.one(rows, 'callback_exit_rpc_error')['event'] = 'callback_exit_transport_closed'
        self.trace_mutation(change)

    def test_native_timer_observation_not_shortened(self):
        def change(rows):
            api = x.one(rows, 'selected_factory_subscription')['api_id']
            x.one([r for r in rows if r['api_id'] == api], 'native_timer_fired')['api_age_ns'] = 1
        self.trace_mutation(change)

    def test_candidate_completion_requires_observed_owner_ack(self):
        self.trace_mutation(lambda rows: x.one(rows, 'observation_frozen')['value'].update(owner_ack=0))


if __name__ == '__main__':
    unittest.main()
