"""Hand-authored source expectations, source/phase boundaries and resealed mutants."""
from collections import defaultdict
import copy
import gzip
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import wi2_dev_gold as g
import evidence_revision
f=g.flow

class SourceExpectations(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.packet=g.ROOT/g.PACKET
        cls.public={r['case_id']:r for file in ('controlled-DEV.json','project-DEV.json') for r in f.read(cls.packet/'public'/file)}
        cls.obs={(o['path'],o['span']['start']):o for o in f.packed(cls.packet/'flow-stdout.json.gz')}
    def gold(self,identity):return f.packed(self.packet/'private'/(identity+'.json.gz'))
    def test_hand_authored_all40_review_sets(self):
        expected=f.read(g.ROOT/'scripts/fixtures/wi2-dev-expectations.json')
        for identity,row in self.public.items():
            if row['stratum']!='controlled':continue
            gold=self.gold(identity)
            with self.subTest(identity=identity):
                if row['rule_id']==g.RULE1:
                    names={b['id']:b['name'] for b in self.obs[row['source']['path'],row['source']['start']]['bindings']};actual=defaultdict(list)
                    for unit in gold['universe']:actual[names[unit['source_identity']]].append(unit['review_label'])
                    self.assertEqual(dict(actual),expected['command'][identity])
                else:self.assertEqual([u['review_label'] for u in gold['universe']],expected['conversion'][identity])
    def test_108_inputs_never_108_independent_families(self):
        self.assertEqual(len(self.public),108);self.assertTrue(all(r['split']=='DEV' and r['formal_split'] is None for r in self.public.values()));self.assertEqual(f.read(self.packet/'SUMMARY.json')['independent_approved'],0)
    def test_all_units_partitioned_once(self):
        for identity in self.public:
            gold=self.gold(identity);ids=[r['id'] for r in gold['universe']];answers=[i for values in gold['answer_sets'].values() for i in values]
            self.assertEqual(len(ids),len(set(ids)));self.assertEqual(sorted(ids),sorted(answers))
            self.assertTrue(all(gold['empty_set_rationales'].get(name) for name,values in gold['answer_sets'].items() if not values))
    def test_opaque_sanitizer_never_identity(self):
        for identity in ('RULE-1-05','RULE-2-06'):
            unit=self.gold(identity)['universe'][0];self.assertEqual(unit['review_label'],'unknown');self.assertIn('opaque_call_result_not_identity_or_sanitizer_proof',unit['flow']['unknown_reasons'])
    def test_alias_not_silently_supported_by_consumer(self):
        for identity in ('RULE-1-15','RULE-2-16'):
            unit=self.gold(identity)['universe'][0];self.assertEqual(unit['contract_label'],'negative');self.assertEqual(unit['review_label'],'positive')
    def test_local_owner_spoof_separate_from_spelling(self):
        for identity in ('RULE-1-07','RULE-2-07'):
            unit=self.gold(identity)['universe'][0];self.assertEqual(unit['callee_status'],'local_object_not_declared_import_target');self.assertEqual(unit['review_label'],'negative');self.assertEqual(unit['contract_label'],'positive')
    def test_non_name_argument_never_command_name_flow(self):self.assertEqual(self.gold('RULE-1-18')['universe'][0]['review_label'],'negative')
    def test_shadow_and_parallel_rhs_do_not_join_by_name(self):
        for identity in ('RULE-1-13','RULE-1-19','RULE-2-14'):
            self.assertTrue(all(r['review_label']=='negative' for r in self.gold(identity)['universe']))
    def test_compound_prior_value_is_preserved(self):
        for identity in ('RULE-1-09','RULE-2-08'):self.assertEqual(self.gold(identity)['universe'][0]['review_label'],'positive')
    def test_bare_return_remains_outside_explicit_consumer_contract(self):
        unit=self.gold('RULE-2-18')['universe'][0];self.assertEqual(unit['sink_kind'],'bare_named_return');self.assertEqual(unit['review_label'],'unknown');self.assertTrue(any(x.startswith('conversion:') for x in unit['flow']['origins']))
    def test_multiple_named_return_slots_not_collapsed(self):
        rows=[r for r in self.gold('CALL-001')['universe'] if r['sink_kind']=='bare_named_return'];self.assertEqual(len(rows),2);self.assertEqual(rows[0]['sink_span'],rows[1]['sink_span']);self.assertNotEqual(rows[0]['sink_binding'],rows[1]['sink_binding']);self.assertNotEqual(rows[0]['id'],rows[1]['id'])
    def test_written_error_variable_is_not_proof_it_was_checked(self):
        for identity in ('RULE-2-11','RULE-2-20'):
            unit=self.gold(identity)['universe'][0];self.assertFalse(unit['blank_second_assignment']);self.assertEqual(unit['review_label'],'negative');self.assertEqual(unit['safety_verdict'],'unknown')
    def test_closure_is_not_approved_empty_gold(self):
        for identity in ('RULE-1-20','RULE-2-19'):self.assertTrue(all(u['review_label']=='unknown' for u in self.gold(identity)['universe']))
    def test_control_is_structural_not_runtime_feasibility(self):
        conditions=self.gold('RULE-1-08')['universe'][0]['flow']['conditions'];self.assertTrue(conditions);self.assertTrue(all('no true/false' in c['meaning'] for c in conditions))
    def test_controlled_import_context_is_explicit_assumption(self):
        row=self.public['RULE-1-01'];self.assertIn('assumption',row['source']['import_context']);self.assertEqual(row['source']['fragment_prefix_bytes'],len(g.PREFIX.encode()));self.assertIsNone(row['source']['source_commit'])
    def test_caller14_not_added_to_DEV_denominators(self):
        caller=f.read(self.packet/'private/caller-context.json');self.assertEqual(len(caller['references']),14);self.assertTrue(caller['not_added_to_108_DEV_inputs']);self.assertIsNone(caller['independent_review_receipt'])
    def test_source_observers_do_not_invoke_candidate_engine(self):
        for label in ('syntax','flow'):self.assertIs(f.read(self.packet/(label+'-receipt.json'))['candidate_engine_imported_or_invoked'],False)

class ResealedMutations(unittest.TestCase):
    def test_current_packet(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def mutant(self,name,change,packed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);path=p/name;value=f.packed(path) if packed else f.read(path);change(value)
            if packed:path.write_bytes(gzip.compress(f.canonical(value),mtime=0))
            else:path.write_text(json.dumps(value))
            manifest=f.read(p/'manifest.json');manifest['artifacts']={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(manifest));self.assertTrue(g.validate(packet=p),name)
    def test_unknown_cannot_be_negative(self):self.mutant('private/RULE-1-05.json.gz',lambda v:v['universe'][0].update(review_label='negative'),True)
    def test_missing_return_slot(self):self.mutant('private/CALL-001.json.gz',lambda v:v['universe'].pop(),True)
    def test_missing_answer_set_member(self):self.mutant('private/RULE-1-01.json.gz',lambda v:v['answer_sets']['rule_candidates'].clear(),True)
    def test_empty_rationale_cannot_disappear(self):self.mutant('private/RULE-1-01.json.gz',lambda v:v['empty_set_rationales'].clear(),True)
    def test_duplicate_unit_cannot_hide(self):self.mutant('private/RULE-2-05.json.gz',lambda v:v['universe'][1].update(id=v['universe'][0]['id']),True)
    def test_fabricated_independent_gold(self):self.mutant('SUMMARY.json',lambda v:v.update(independent_approved=108))
    def test_scores_not_allowed(self):self.mutant('SUMMARY.json',lambda v:v.update(current_scores={'precision':1}))
    def test_no_formal_completion(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))
    def test_no_reviewer_invention(self):self.mutant('WORKLIST.json',lambda v:v.update(reviewer_assignment='AI'))
    def test_no_unexecuted_MCP_receipt(self):self.mutant('matching-contract.json',lambda v:v.update(retained_MCP_receipt={'exit':0}))
    def test_controlled_context_not_original_proof(self):self.mutant('public/controlled-context.json',lambda v:v.update(status='independent_approved'))
    def test_public_cannot_leak_private_labels(self):self.mutant('public/controlled-DEV.json',lambda v:v[0].update(answer_sets={}))
    def test_source_observer_receipt_cannot_change(self):self.mutant('syntax-receipt.json',lambda v:v.update(candidate_engine_imported_or_invoked=True))
    def test_caller_population_not_promoted(self):self.mutant('private/caller-context.json',lambda v:v.update(not_added_to_108_DEV_inputs=False))
    def test_heuristic_cannot_be_safety(self):self.mutant('private/RULE-1-01.json.gz',lambda v:v['universe'][0].update(safety_verdict='safe'),True)


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
