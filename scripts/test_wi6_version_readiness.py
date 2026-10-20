"""Historical/current separation and exact paired request preparation checks."""
import copy
import json
import tempfile
import unittest
from pathlib import Path

import wi6_version_readiness as g


class SnapshotTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.packet = Path('packet')
        (self.root / self.packet).mkdir()
        (self.root / 'tool.py').write_bytes(b'original tool')
        (self.root / self.packet / 'evidence.json').write_bytes(b'{"score":null}')
        self.manifest = dict(artifacts={'evidence.json': g.sha(b'{"score":null}')},
                             tools=[dict(path='tool.py', sha256=g.sha(b'original tool'))])
        self.raw = json.dumps(self.manifest).encode()
        (self.root / self.packet / 'manifest.json').write_bytes(self.raw)
        self.stored = {'packet/manifest.json': self.raw, 'tool.py': b'original tool'}

    def check(self):
        return g.snapshot(self.root, self.packet, 'historical',
                          lambda root, revision, path: self.stored[str(path)])

    def test_original_snapshot_passes_without_runtime_claim(self):
        result = self.check()
        self.assertTrue(result['historical_bytes_verified'])
        self.assertFalse(result['runtime_reexecuted'])

    def test_current_tool_change_is_recorded_not_applied_to_historical_digest(self):
        (self.root / 'tool.py').write_bytes(b'new tool')
        self.assertEqual(self.check()['current_tool_differences'], ['tool.py'])

    def test_changed_historical_tool_rejected(self):
        self.stored['tool.py'] = b'wrong old tool'
        with self.assertRaises(ValueError): self.check()

    def test_changed_evidence_rejected(self):
        (self.root / self.packet / 'evidence.json').write_bytes(b'{"score":1}')
        with self.assertRaises(ValueError): self.check()

    def test_local_reseal_cannot_override_historical_git(self):
        self.manifest['artifacts']['evidence.json'] = '0' * 64
        (self.root / self.packet / 'manifest.json').write_text(json.dumps(self.manifest))
        with self.assertRaises(ValueError): self.check()

    def test_unlisted_evidence_rejected(self):
        (self.root / self.packet / 'unexpected.txt').write_text('unexpected')
        with self.assertRaises(ValueError): self.check()

    def test_missing_evidence_rejected(self):
        (self.root / self.packet / 'evidence.json').unlink()
        with self.assertRaises(ValueError): self.check()

    def test_noncanonical_paths_rejected(self):
        for path in ('../escape', '/absolute', 'a/../b', 'a//b'):
            with self.assertRaises(ValueError): g.bounded_path(self.root, path)


class PairPreparationTests(unittest.TestCase):
    def setUp(self):
        self.inputs = g.load(g.ROOT, g.HISTORY / 'public/DEV-inputs.json')
        self.pins = dict(node_id='retained-node', project_id='project', dataset_id='dataset',
                         commit='a' * 40, function_sha256='b' * 64, snapshot_sha256='c' * 64)

    def test_only19_production_cases_not_source_only_stress(self):
        p = g.plan(self.inputs)
        self.assertEqual(len(p['cases']), 19)
        self.assertEqual(len(p['excluded_source_only']), 1)
        self.assertEqual(p['planned_pairs'], 627)
        self.assertEqual(p['planned_tool_calls'], 1254)

    def test_no_resolution_or_calls_fabricated(self):
        p = g.plan(self.inputs)
        self.assertEqual(p['actual_pairs'], 0)
        self.assertEqual(p['actual_tool_calls'], 0)
        self.assertEqual(p['actual_warm_samples'], 0)
        self.assertTrue(all(r['selected_node'] is None and r['snapshot_sha256'] is None
                            and not r['resolved'] and r['actual_baseline_request'] is None
                            and r['actual_expansion_request'] is None for r in p['cases']))

    def test_independent_and_formal_flags_remain_false(self):
        p = g.plan(self.inputs)
        self.assertFalse(p['gold_independent'])
        self.assertFalse(p['formal_scoring_allowed'])
        self.assertFalse(p['execution_allowed'])
        self.assertEqual(p['formal_completed'], 0)
        self.assertIsNone(p['freeze'])

    def test_promoted_final_approved_or_unknown_scope_rejected(self):
        for key, value in [('split', 'FINAL'), ('approved', True), ('scope', 'undeclared')]:
            rows = copy.deepcopy(self.inputs)
            rows[0][key] = value
            with self.assertRaises(ValueError): g.plan(rows)

    def test_population_missing_duplicate_or_stress_promoted_rejected(self):
        with self.assertRaises(ValueError): g.plan(self.inputs[:-1])
        rows = copy.deepcopy(self.inputs)
        rows[-1]['case_id'] = rows[0]['case_id']
        with self.assertRaises(ValueError): g.plan(rows)
        rows = copy.deepcopy(self.inputs)
        next(r for r in rows if r['scope'] == 'development_stress_source_only')['scope'] = 'production_source_candidate'
        with self.assertRaises(ValueError): g.plan(rows)

    def test_actual_go_request_shapes_share_pins_and_limits(self):
        q = g.requests(self.pins)
        self.assertEqual(q['baseline'], q['expansion']['function'])
        self.assertEqual(q['baseline']['limits'], g.LOCAL_LIMITS)
        self.assertEqual(q['expansion']['limits'], g.EXPANSION_LIMITS)
        self.assertEqual(q['session_pins'], dict(project_id='project', dataset_id='dataset'))
        self.assertNotIn('project_id', q['baseline'])
        self.assertNotIn('dataset_id', q['baseline'])

    def test_each_request_has_own_mutable_limits(self):
        q = g.requests(self.pins)
        q['baseline']['limits']['max_nodes'] = 1
        self.assertEqual(q['expansion']['function']['limits']['max_nodes'], 1024)
        self.assertEqual(g.LOCAL_LIMITS['max_nodes'], 1024)

    def test_empty_missing_wrong_type_or_unknown_pin_rejected(self):
        for key in self.pins:
            for value in ('', None, 1):
                pins = dict(self.pins, **{key: value})
                with self.assertRaises(ValueError): g.requests(pins)
            pins = dict(self.pins)
            del pins[key]
            with self.assertRaises(ValueError): g.requests(pins)
        with self.assertRaises(ValueError): g.requests(dict(self.pins, name='ambiguous'))
        with self.assertRaises(ValueError): g.requests(dict(self.pins, node_id='a' * 129))

    def test_nonhex_or_wrong_length_pin_rejected(self):
        for key in ('commit', 'function_sha256', 'snapshot_sha256'):
            with self.assertRaises(ValueError): g.requests(dict(self.pins, **{key: 'z' * len(self.pins[key])}))
            with self.assertRaises(ValueError): g.requests(dict(self.pins, **{key: 'a'}))


if __name__ == '__main__':
    unittest.main()
