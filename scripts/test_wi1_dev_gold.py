#!/usr/bin/env python3
"""Hand-authored reference controls, full-case consistency and false-approval mutants."""
import copy
import gzip
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

import wi1_dev_gold as guard
import evidence_revision

FIXTURES = Path('scripts/fixtures/wi1-dev-controls.json')


def observer_case(observation):
    return {'candidate_id': observation['name'], 'source': {'path': observation['path'], 'repository_commit': guard.SOURCE_COMMIT}}


class HandAuthoredControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.rows = {o['name']: o for o in guard.packed(guard.ROOT / guard.PACKET / 'controls/observations.json.gz')}
        cls.specs = guard.read(guard.ROOT / FIXTURES)

    def test_all_hand_authored_positive_sets(self):
        for spec in self.specs:
            if 'positive' not in spec:
                continue
            with self.subTest(function=spec['name']):
                observation = self.rows[spec['name']];gold = guard.derive_gold(observation, observer_case(observation))
                occurrences = {o['id']: o for o in gold['occurrences']}
                actual = {(occurrences[r['definition'][2:]]['span']['start'], occurrences[r['use'][2:]]['span']['start']) for r in gold['data_universe'] if r['label'] == 'positive'}
                raw = ('package devcontrols\n' + spec['source'] + '\n').encode()
                expected = {(raw.index(d.encode()) + do, raw.index(u.encode()) + uo) for d, do, u, uo in spec['positive']}
                self.assertEqual(actual, expected)

    def test_bare_return_read_uses_actual_return_coordinate(self):
        observation = self.rows['named'];gold = guard.derive_gold(observation, observer_case(observation))
        implicit = [o for o in gold['occurrences'] if o['mapping_reason'] == 'implicit_named_result_at_return']
        self.assertEqual(len(implicit), 1);self.assertNotEqual(implicit[0]['span']['start'], next(b['declaration']['start'] for b in gold['bindings'] if b['name'] == 'r'))

    def test_unknown_effects_are_not_negative_or_scalar_gold(self):
        for spec in self.specs:
            if not spec.get('unknown_names'):
                continue
            with self.subTest(function=spec['name']):
                observation = self.rows[spec['name']];gold = guard.derive_gold(observation, observer_case(observation))
                bindings = {b['id'] for b in gold['bindings'] if b['name'] in spec['unknown_names']}
                rows = [r for r in gold['data_universe'] if r['binding'] in bindings]
                self.assertTrue(rows);self.assertEqual({r['label'] for r in rows}, {'unknown'})

    def test_shadow_identity_negatives(self):
        observation = self.rows['shadow'];gold = guard.derive_gold(observation, observer_case(observation))
        self.assertEqual(len(gold['shadow_identity_negative_controls']), 3)

    def test_branch_control_targets_by_source_not_engine_ID(self):
        observation = self.rows['branch'];gold = guard.derive_gold(observation, observer_case(observation))
        nodes = {gold['matching']['node_keys'][n['id']]: n for n in observation['nodes']}
        raw = ('package devcontrols\n' + next(s['source'] for s in self.specs if s['name'] == 'branch') + '\n').encode()
        targets = {raw[nodes[e['dependent']]['span']['start']:nodes[e['dependent']]['span']['end']].decode() for e in gold['control_node_projection'] if e['dependent'] in nodes}
        self.assertEqual(targets, {'y=1'})

    def test_unicode_byte_offsets(self):
        observation = self.rows['unicode']
        raw = ('package devcontrols\n' + next(s['source'] for s in self.specs if s['name'] == 'unicode') + '\n').encode()
        for binding in observation['bindings']:
            self.assertEqual(raw[binding['declaration']['start']:binding['declaration']['end']].decode(), binding['name'])

    def test_loop_self_definition_reaches_only_via_cycle(self):
        observation = self.rows['loop'];gold = guard.derive_gold(observation, observer_case(observation))
        same = [r for r in gold['data_universe'] if r['definition'][2:] == r['use'][2:]]
        self.assertTrue(same);self.assertTrue(all(r['structural_reaches'] for r in same))

    def test_nested_signature_parameters_are_not_outer_bindings(self):
        observation = self.rows['callback']
        self.assertEqual([b['name'] for b in observation['bindings']], ['f'])
        self.assertEqual([r['name'] for r in observation['quarantined_signature_occurrences']], ['x'])

    def test_nonexiting_regions_not_safe_empty_control_gold(self):
        observation = self.rows['forever'];gold = guard.derive_gold(observation, observer_case(observation))
        self.assertTrue(gold['control_nonexit_unknown_blocks'])
        self.assertFalse(gold['formal_gold_complete'])

    def test_straight_same_statement_write_does_not_feed_its_rhs(self):
        observation = self.rows['chain'];gold = guard.derive_gold(observation, observer_case(observation))
        defs = {o['id']: o for o in gold['occurrences']}
        rows = [r for r in gold['data_universe'] if defs[r['definition'][2:]]['owner'] == defs[r['use'][2:]]['owner']]
        self.assertTrue(rows);self.assertFalse(any(r['structural_reaches'] for r in rows))

    def test_reference_does_not_claim_independent_approval(self):
        for observation in self.rows.values():
            gold = guard.derive_gold(observation, observer_case(observation))
            self.assertFalse(gold['formal_scoring_allowed']);self.assertFalse(gold['independent_approved'])


class PacketControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.packet = Path(self.temp.name) / 'packet';shutil.copytree(guard.ROOT / guard.PACKET, self.packet)

    def mutate(self, relative, fn):
        target = self.packet / relative
        data = guard.packed(target) if target.suffix == '.gz' else guard.read(target)
        fn(data)
        if target.suffix == '.gz':
            target.write_bytes(gzip.compress(guard.canonical(data), mtime=0))
        else:
            target.write_text(json.dumps(data, ensure_ascii=False) + '\n')
        manifest = guard.read(self.packet / 'manifest.json');manifest['artifacts'][relative] = guard.sha(target.read_bytes())
        (self.packet / 'manifest.json').write_text(json.dumps(manifest) + '\n')
        return guard.validate_packet(guard.ROOT, self.packet)

    def test_actual_packet_complete(self):
        self.assertEqual(guard.validate_packet(guard.ROOT, self.packet), [])

    def test_project_census_preserves_412_bindings_and_quarantines_four(self):
        summary = guard.read(self.packet / 'SUMMARY.json')
        self.assertEqual(summary['bindings'], 412)
        self.assertEqual(summary['occurrences'], 1721)
        self.assertEqual(summary['quarantined_signature_occurrences'], 4)

    def test_readable_review_survives_canonical_JSON_roundtrip(self):
        candidate = next(c for c in guard.read(guard.ROOT / guard.FUNCTIONS)['functions'] if c['candidate_id'] == 'SRC-024')
        source = candidate['source']
        observation = next(o for o in guard.packed(self.packet / 'observations.json.gz') if o['path'] == source['path'] and o['span']['start'] == source['start_byte'])
        gold = guard.derive_gold(observation, candidate)
        self.assertEqual(guard.render_review(gold, observation), guard.render_review(json.loads(guard.canonical(gold)), observation))

    def test_positive_edge_omission_refused_after_reseal(self):
        self.assertTrue(self.mutate('private/SRC-001.json.gz', lambda d: d['data_universe'].pop()))

    def test_negative_edge_flipped_refused_after_reseal(self):
        def edit(d):
            next(r for r in d['data_universe'] if r['label'] == 'negative')['label'] = 'positive'
        self.assertTrue(self.mutate('private/SRC-001.json.gz', edit))

    def test_unknown_not_promoted_to_negative(self):
        def edit(d):
            next(r for r in d['data_universe'] if r['label'] == 'unknown')['label'] = 'negative'
        self.assertTrue(self.mutate('private/SRC-001.json.gz', edit))

    def test_AI_not_human_approved(self):
        self.assertTrue(self.mutate('private/SRC-001.json.gz', lambda d: d.update(independent_approved=True)))

    def test_DEV_not_FINAL(self):
        self.assertTrue(self.mutate('public/DEV-inputs.json', lambda d: d[0].update(split='FINAL')))

    def test_public_input_has_no_gold_paths(self):
        self.assertTrue(self.mutate('public/DEV-inputs.json', lambda d: d[0].update(gold_path='../private/SRC-001.json.gz')))

    def test_source_coordinates_pinned(self):
        self.assertTrue(self.mutate('public/DEV-inputs.json', lambda d: d[0]['source'].update(start_byte=0)))

    def test_control_universe_cannot_drop_negative_pairs(self):
        self.assertTrue(self.mutate('private/SRC-001.json.gz', lambda d: d['control_universe'].pop()))

    def test_faked_score_refused(self):
        self.assertTrue(self.mutate('SUMMARY.json', lambda d: d.update(quality_scores={'precision': 1})))

    def test_worklist_not_formal_completion(self):
        self.assertTrue(self.mutate('WORKLIST.json', lambda d: d.update(formal_completed=9)))

    def test_missing_function_not_30_complete(self):
        self.assertTrue(self.mutate('public/DEV-inputs.json', lambda d: d.pop()))

    def test_missing_raw_artifact(self):
        (self.packet / 'observations.json.gz').unlink()
        self.assertTrue(guard.validate_packet(guard.ROOT, self.packet))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
