"""Actual source matching, phase/synthetic boundary controls and resealed mutations."""
import copy
import gzip
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import wi1_consumer_matching as g
import evidence_revision
old=g.old

class MatchingControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.obs={o['name']:o for o in old.packed(g.ROOT/old.PACKET/'controls/observations.json.gz')}
        cls.actual={r['name']:r['response']['analysis'] for r in old.read(g.ROOT/g.PACKET/'runtime/control-results.json')}
    def test_named_return_uses_return_not_declaration(self):
        m=g.match(self.obs['named'],self.actual['named']);r=next(r for r in m['occurrences'] if r['reference'].startswith('u:return@'))
        self.assertIsNotNone(r['consumer']);self.assertEqual(r['consumer_roles'],['implicit_return']);self.assertNotEqual(r['span'],self.obs['named']['bindings'][-1]['declaration'])
    def test_utf8_byte_spans_match(self):
        m=g.match(self.obs['unicode'],self.actual['unicode']);self.assertEqual(m['unmatched_bindings'],0);self.assertEqual(m['unmatched_occurrence_phases'],0)
    def test_read_write_splits_phases(self):
        m=g.match(self.obs['loop'],self.actual['loop']);o=next(o for o in self.obs['loop']['occurrences'] if o['role']=='read_write');r=[r for r in m['occurrences'] if r['reference'][2:]==o['id']];self.assertEqual({x['phase'] for x in r},{'use','definition'});self.assertTrue(all(x['consumer'] for x in r))
    def test_equal_spelling_cannot_override_declaration(self):
        a=copy.deepcopy(self.actual['shadow']);a['variables'][0]['declaration']['start_byte']+=1;m=g.match(self.obs['shadow'],a);self.assertGreater(m['unmatched_bindings'],0)
    def test_node_raw_ids_are_not_matching_identity(self):
        a=copy.deepcopy(self.actual['chain']);a['nodes'][0]['location']['start_byte']+=1;self.assertGreater(g.match(self.obs['chain'],a)['unmatched_nodes'],0)
    def test_ambiguous_identical_node_is_not_selected(self):
        a=copy.deepcopy(self.actual['chain']);n=copy.deepcopy(a['nodes'][0]);n['id']='ambiguous';a['nodes'].append(n);self.assertGreater(g.match(self.obs['chain'],a)['unmatched_nodes'],0)
    def test_implicit_synthetic_return_coordinates(self):
        o=self.obs['forever'];a=self.actual['forever'];m=g.match(o,a)
        for r in m['nodes']:
            if r['synthetic']:self.assertEqual(r['span']['end']-r['span']['start'],1);self.assertIsNotNone(r['consumer'])
    def test_constant_is_not_an_executable_write(self):
        a=self.actual['constant'];self.assertTrue(any(n['kind']=='compile_time_constant' and n['block']==-1 for n in a['nodes']))
        c={'candidate_id':'constant','source':{'path':self.obs['constant']['path'],'repository_commit':old.SOURCE_COMMIT}};gold=old.derive_gold(self.obs['constant'],c);rows=g.semantic_overlay(gold);self.assertTrue(any('compile_time_source_not_runtime_write' in r['unknown_reasons'] for r in rows));self.assertTrue(all(r['concrete_runtime_value']=='unknown' for r in rows))
    def test_reference_and_aggregate_never_prove_memory(self):
        for domain in ('reference_binding','aggregate_binding'):
            gold=dict(bindings=[dict(id='v',value_kind=domain,origin='local',flags=[])],data_universe=[dict(binding='v',definition='d',use='u',label='positive',structural_reaches=True)])
            r=g.semantic_overlay(gold)[0];self.assertEqual(r['lexical_label'],'positive');self.assertEqual(r['value_flow_label'],'unknown')
    def test_negative_lexical_path_does_not_resolve_escape(self):
        gold=dict(bindings=[dict(id='v',value_kind='scalar',origin='local',flags=['address_exposed'])],data_universe=[dict(binding='v',definition='d',use='u',label='unknown',structural_reaches=False)])
        r=g.semantic_overlay(gold)[0];self.assertEqual(r['lexical_label'],'negative');self.assertEqual(r['value_flow_label'],'unknown')
    def test_original_unknown_never_promoted_without_source_review(self):
        gold=dict(bindings=[dict(id='v',value_kind='scalar',origin='local',flags=[])],data_universe=[dict(binding='v',definition='d',use='u',label='unknown',structural_reaches=True)])
        self.assertEqual(g.semantic_overlay(gold)[0]['value_flow_label'],'unknown')
    def test_wrong_source_hash_refused_before_mapping(self):
        a=copy.deepcopy(self.actual['chain']);a['source']['sha256']='0'*64
        with self.assertRaises(AssertionError):g.match(self.obs['chain'],a)
    def test_branch_conditions_are_actual_edges(self):
        m=g.match(self.obs['branch'],self.actual['branch']);self.assertTrue(m['branches']);self.assertEqual({r['consumer_edge']['condition'] for r in m['branches']},{'true','false'});self.assertTrue(all(r['target_matches'] for r in m['branches']))
    def test_no_exit_is_not_empty_safe_control_gold(self):
        self.assertTrue(any(d['code']=='nontermination_control_unknown' for d in self.actual['forever']['diagnostics']))

class EvidenceMutations(unittest.TestCase):
    def test_current_packet(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def mutant(self,name,change,compressed=False):
        with tempfile.TemporaryDirectory() as d:
            p=Path(d)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);f=p/name;v=old.packed(f) if compressed else old.read(f);change(v)
            f.write_bytes(gzip.compress(old.canonical(v),mtime=0)) if compressed else f.write_text(json.dumps(v))
            # An internally re-sealed packet still has to satisfy semantic/wire/source contracts.
            m=old.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():old.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x.name!='manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p),name)
    def test_cannot_invent_human_approval(self):self.mutant('SUMMARY.json',lambda v:v.update(independent_approved=30))
    def test_cannot_invent_precision(self):self.mutant('SUMMARY.json',lambda v:v.update(quality_scores={'precision':1}))
    def test_unknown_not_negative(self):self.mutant('cases/SRC-001.json.gz',lambda v:v['comparison']['rows'][0].update(value_flow_label='negative'),True)
    def test_wrong_source_pin(self):self.mutant('runtime/project-results.json',lambda v:v[0]['arguments'].update(expected_source_sha256='0'*64))
    def test_changed_actual_response_caught_by_wire(self):self.mutant('runtime/project-results.json',lambda v:v[0]['response']['analysis'].update(elapsed_micros=-1))
    def test_removed_case(self):self.mutant('runtime/project-results.json',lambda v:v.pop())
    def test_consumer_mapping_cannot_be_forged(self):self.mutant('cases/SRC-001.json.gz',lambda v:v['mapping']['bindings'][0].update(consumer='fake'),True)
    def test_cannot_complete_formal_work(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))
    def test_no_fabricated_reviewer(self):self.mutant('WORKLIST.json',lambda v:v.update(reviewer_assignment='AI'))
    def test_failed_shutdown_not_hidden(self):self.mutant('runtime/receipt.json',lambda v:v.update(EOF_exit_codes=[0,1,0,0]))
    def test_changed_retained_data(self):self.mutant('runtime/after.json',lambda v:v.clear())
    def test_review_debt_cannot_disappear(self):self.mutant('REVIEW-LEDGER.json',lambda v:v['items'].clear())
    def test_matching_contract_cannot_be_formal(self):self.mutant('matching-contract.json',lambda v:v.update(scoring_allowed=True))
    def test_prior_failure_not_erased(self):self.mutant('runtime/receipt.json',lambda v:v.update(initial_budget_exhausted_calls=0))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
