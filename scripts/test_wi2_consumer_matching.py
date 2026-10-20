"""Source counterexamples and resealed transport/identity/approval mutations."""
import copy
import gzip
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import wi2_consumer_matching as g
import evidence_revision
f = g.f


class SourceAndRuntime(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.p = g.ROOT/g.PACKET
        cls.overlays = {o['case_id']: o for o in f.packed(cls.p/'private/overlays.json.gz')}
        cls.maps = {o['function_id']: o for o in f.packed(cls.p/'private/mappings.json.gz')}

    def test_all_source_units_retained(self):
        self.assertEqual(len(self.overlays), 108)
        self.assertEqual(sum(len(o['units']) for o in self.overlays.values()), 538)
        self.assertEqual(len(self.maps), 91)

    def test_capture_is_read_only_not_attacker_control(self):
        row = self.overlays['CALL-046']
        self.assertEqual([u['review_label'] for u in row['units']], ['positive', 'negative'])
        self.assertEqual([u['previous_review_label'] for u in row['units']], ['unknown', 'unknown'])
        self.assertTrue(row['readonly_capture_evidence'])
        self.assertTrue(all(u['attacker_control'] == 'not_established' for u in row['units']))

    def capture(self, flags=None, mutation=False, address=False, kind='scalar', origin='parameter'):
        obs = dict(bindings=[dict(id='v@1', origin=origin, value_kind=kind,
                                flags=flags or ['captured_by_closure'], declaration=dict(start=1, end=2))])
        audit = dict(accesses=[dict(binding='v@1', closure_owner=9, mutation=mutation, address=address)])
        return g.readonly_captures(obs, audit)

    def test_closure_mutation_stays_unknown(self):
        self.assertEqual(self.capture(mutation=True), [])

    def test_address_escape_stays_unknown(self):
        self.assertEqual(self.capture(address=True), [])

    def test_reference_and_local_captures_not_refined(self):
        self.assertEqual(self.capture(kind='reference_binding'), [])
        self.assertEqual(self.capture(origin='local'), [])

    def test_other_boundary_flag_not_erased(self):
        self.assertEqual(self.capture(flags=['captured_by_closure', 'address_exposed']), [])

    def test_opaque_calls_are_not_identity_or_sanitizer(self):
        for identity in ('RULE-1-05', 'RULE-2-06', 'CALL-040', 'CALL-062'):
            self.assertTrue(all(u['review_label'] == 'unknown' for u in self.overlays[identity]['units']))

    def test_alias_and_spoof_have_distinct_source_and_contract(self):
        for identity in ('RULE-1-15', 'RULE-2-16'):
            row = self.overlays[identity]['units'][0]
            self.assertEqual((row['review_label'], row['contract_label']), ('positive', 'negative'))
        for identity in ('RULE-1-07', 'RULE-2-07'):
            row = self.overlays[identity]['units'][0]
            self.assertEqual((row['review_label'], row['contract_label']), ('negative', 'positive'))

    def test_actual_spoof_response_is_not_gold(self):
        for identity in ('RULE-1-07', 'RULE-2-07'):
            self.assertTrue(self.maps[identity]['candidates'])
            self.assertEqual(self.overlays[identity]['units'][0]['review_label'], 'negative')

    def test_actual_opaque_response_is_not_gold(self):
        self.assertTrue(self.maps['RULE-1-05']['candidates'])
        self.assertEqual(self.overlays['RULE-1-05']['units'][0]['review_label'], 'unknown')

    def test_closure_owner_not_same_depth_guess(self):
        for identity in ('RULE-1-20', 'RULE-2-19'):
            self.assertGreater(self.overlays[identity]['lexical_closure_owner'], 0)
            self.assertTrue(all(u['review_label'] == 'unknown' for u in self.overlays[identity]['units']))

    def test_bare_named_and_compound_sinks_unknown(self):
        for identity in ('RULE-1-16', 'RULE-1-17', 'RULE-2-17', 'RULE-2-18'):
            self.assertEqual(self.overlays[identity]['units'][0]['review_label'], 'unknown')

    def test_caller_literals_do_not_make_wrapper_safe(self):
        rows = f.packed(self.p/'private/callers.json.gz')
        self.assertEqual(len(rows), 14)
        self.assertEqual(sum(r['literal_at_this_site_only'] for r in rows), 13)
        self.assertTrue(all(r['safety_verdict'] == 'unknown' and not r['transitively_proven']
                            and r['added_independent_cases'] == 0 for r in rows))

    def test_missing_symbols_preserve_every_unit(self):
        missing = [m for m in self.maps.values() if m['status'] == 'consumer_symbol_unavailable_unknown']
        self.assertTrue(missing)
        self.assertTrue(all(u['consumer_presence'] is None for m in missing for u in m['units']))

    def test_budget_exhaustion_is_not_empty_negative(self):
        missing = [m for m in self.maps.values() if m['status'] == 'analysis_unavailable_unknown']
        self.assertTrue(missing)
        self.assertTrue(all(u['consumer_presence'] is None for m in missing for u in m['units']))

    def test_source_unknown_survives_consumer_candidate(self):
        for o in self.overlays.values():
            for u in o['units']:
                self.assertEqual(u['safety_verdict'], 'unknown')
        self.assertIsNone(f.read(self.p/'SUMMARY.json')['current_scores'])

    def test_conditions_retained_in_actual_branch(self):
        row = self.maps['RULE-1-08']
        self.assertTrue(any(c['conditions'] for c in row['candidates']))
        self.assertTrue(all('raw' in c and c['status'] in ('source_node', 'structural_block', 'unknown')
                            for v in row['candidates'] for c in v['conditions']))
        self.assertTrue(all(c['branch_matching'] == 'unique_structural_ordinal' and c['edge_on_candidate_path']
                            for v in row['candidates'] for c in v['conditions']))

    def test_actual_source_counterexamples(self):
        rows = f.read(g.ROOT/'scripts/fixtures/wi2-context-expectations.json')
        audit = f.packed(self.p/'context-probe/audit.json.gz')
        flow = f.packed(self.p/'context-probe/flow.json.gz')
        for row, observation, syntax in zip(rows, flow, audit):
            self.assertEqual(len(g.readonly_captures(observation, syntax)), row['expected_readonly_parameters'])
        sibling = next(a for a in audit if a['path'] == 'context/siblings.go')
        call = next(c for c in sibling['calls'] if c['member'] == 'Atoi')
        self.assertEqual(len({r['closure_owner'] for r in sibling['returns']}), 2)
        self.assertEqual(sum(r['closure_owner'] == call['closure_owner'] for r in sibling['returns']), 1)

    def test_one_candidate_is_not_repeated_per_call(self):
        for m in self.maps.values():
            ids = [r['candidate_id'] for r in m['candidates']]
            self.assertEqual(len(ids), len(set(ids)))
            self.assertTrue(all(len(c['matches']) <= 1 for c in m['candidates']))

    def test_controlled_adapter_preserves_fragments_and_offsets(self):
        adapter = f.read(self.p/'runtime/controlled-source.json')
        self.assertEqual(len(adapter['adapters']), 40)
        self.assertTrue(all(a['byte_offset_delta'] == 0 and len(a['package']) == 6
                            and a['original_file_sha256'] != a['actual_file_sha256'] for a in adapter['adapters']))

    def test_actual_pin_caps_and_EOF(self):
        self.assertEqual(g.wire_errors(self.p), [])
        self.assertEqual([s['exit'] for s in f.read(self.p/'runtime/receipt.json')['sessions']], [0]*4)

    def test_wrong_project_rejected_before_span_join(self):
        p = f.read(g.ROOT/g.old.PACKET/'public/project-DEV.json')[0]
        r = copy.deepcopy(f.read(self.p/'runtime/project-results.json')[0])
        r['project'] = 'other-project'
        with self.assertRaisesRegex(ValueError, 'cross-project'):
            g.response_identity(p, r, 'go-stablenet', {})

    def test_wrong_graph_snapshot_rejected(self):
        p = f.read(g.ROOT/g.old.PACKET/'public/project-DEV.json')[0]
        r = copy.deepcopy(f.read(self.p/'runtime/project-results.json')[0])
        r['response']['analysis']['source']['snapshot_sha256'] = '0'*64
        with self.assertRaises(ValueError):
            g.response_identity(p, r, 'go-stablenet', {})


class ResealedMutants(unittest.TestCase):
    def mutant(self, name, change, compressed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp)/'packet'
            shutil.copytree(g.ROOT/g.PACKET, p)
            path = p/name
            value = f.packed(path) if compressed else f.read(path)
            change(value)
            path.write_bytes(gzip.compress(f.canonical(value), mtime=0)) if compressed else path.write_text(json.dumps(value))
            manifest = f.read(p/'manifest.json')
            manifest['artifacts'] = {x.relative_to(p).as_posix(): f.sha(x.read_bytes())
                                     for x in p.rglob('*') if x.is_file() and x != p/'manifest.json'}
            (p/'manifest.json').write_text(json.dumps(manifest))
            self.assertTrue(g.validate(packet=p), name)

    def test_current_packet_and_indices(self):
        self.assertEqual(g.validate(), [])
        self.assertEqual(g.current_indices(), [])

    def test_source_unknown_not_overwritten(self):
        self.mutant('private/overlays.json.gz', lambda v: next(o for o in v if o['case_id'] == 'RULE-1-05')['units'][0].update(review_label='positive'), True)

    def test_lost_units(self):
        self.mutant('private/mappings.json.gz', lambda v: v[0]['units'].pop(), True)

    def test_reviewer_invention(self):
        self.mutant('WORKLIST.json', lambda v: v.update(reviewer_assignment='AI'))

    def test_quality_score_fabrication(self):
        self.mutant('SUMMARY.json', lambda v: v.update(current_scores=dict(precision=1)))

    def test_independent_approval_fabrication(self):
        self.mutant('SUMMARY.json', lambda v: v.update(independent_approved=108))

    def test_matching_approval_fabrication(self):
        self.mutant('matching-contract.json', lambda v: v.update(formal_scoring_allowed=True))

    def test_caller_safety_fabrication(self):
        self.mutant('private/callers.json.gz', lambda v: v[1].update(safety_verdict='safe'), True)

    def test_candidate_removed_from_raw_backed_response(self):
        self.mutant('runtime/controlled-results.json', lambda v: v[0]['response']['analysis']['rules']['candidates'].clear())

    def test_incomplete_promoted(self):
        self.mutant('runtime/receipt.json', lambda v: v.update(formal_completed=9))

    def test_retained_inventory_changed(self):
        self.mutant('runtime/retained-after.json', lambda v: v.pop(next(iter(v))))

    def test_review_debt_cannot_disappear(self):
        self.mutant('REVIEW-LEDGER.json', lambda v: v['AI_source_unknown_units'].clear())

    def test_graph_dataset_envelope_cannot_change(self):
        self.mutant('runtime/dataset-identity.json', lambda v: v.update(dataset_id='other-dataset'))

    def test_synthetic_adapter_cannot_change_coordinate(self):
        self.mutant('runtime/controlled-source.json', lambda v: v['adapters'][0].update(byte_offset_delta=1))

    def test_actual_conditions_cannot_be_erased(self):
        self.mutant('runtime/controlled-results.json', lambda v: next(r for r in v if r['function_id']=='RULE-1-08')['response']['analysis']['rules']['candidates'][0]['conditions'].clear())


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
