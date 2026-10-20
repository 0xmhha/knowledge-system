"""Correlated errors, exact direct query scope, denominator and false completion."""
import copy,gzip,json,shutil,tempfile,unittest
from pathlib import Path
import acceptance_debt_diagnostic as g
import evidence_revision


class DiagnosticTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.data=g.derive();cls.cases=cls.data['cases']
    def test_original20partial68slots_retained(self):self.assertEqual(len(self.cases),20);self.assertEqual(sum(x['planned_candidate_slots'] for x in self.cases),68);self.assertTrue(all(x['partial'] for x in self.cases))
    def test_26_unresolved_seeds_across17cases(self):self.assertEqual(sum(len(x['unresolved_seeds']) for x in self.cases),26);self.assertEqual(sum(bool(x['unresolved_seeds']) for x in self.cases),17)
    def test_102labels_not_independent_failures(self):self.assertEqual(sum(x['shared_error_slots'] for x in self.cases),102);self.assertIsNone(self.data['summary']['historical_backend_outage_count'])
    def test_fully_resolved3cases_had9observed_each_signal(self):
        cases=[x for x in self.cases if not x['unresolved_seeds']];self.assertEqual([x['case_id'] for x in cases],['RQ-003','RQ-004','RQ-019'])
        for kind in ('impact_category','concurrency'):self.assertEqual(sum(x['signal_states'].get(kind+':observed',0) for x in cases),9)
    def test_missing_seed_sufficient_not_exclusive_cause(self):
        for x in self.cases:
            if x['unresolved_seeds']:self.assertEqual(x['shared_error_slots'],6);self.assertIsNotNone(x['sufficient_source_failure_path'])
            self.assertFalse(x['exclusive_historical_cause_proven']);self.assertIsNone(x['actual_past_backend_error_trace'])
    def test_all40distinct_observed_symbols_actual80calls(self):self.assertEqual(len(g.symbols()),40);self.assertEqual(self.data['summary']['current_direct_states'],{'structured_response':80})
    def test_current_successes_never_erase_original_partial(self):self.assertEqual(self.data['summary']['original_partial'],20);self.assertFalse(self.data['summary']['exclusive_historical_cause_proven']);self.assertIsNone(self.data['summary']['quality_scores'])
    def test_negative_controls2_are_not_actual_outages(self):self.assertEqual(self.data['summary']['unindexed_controls'],2);self.assertEqual(self.data['summary']['exploratory_calls'],6)
    def test_WI1_58_debts_not_resolved_from_graph_calls(self):
        d=self.data['debt'][0];self.assertEqual(d['counts'],{'unmatched_occurrence_phase':53,'unmatched_block':3,'lexical_difference':2});self.assertFalse(d['independently_resolved'])
    def test_WI2_unavailable_and_source_disagreements_retained(self):
        d=self.data['debt'][1];self.assertEqual(d['counts']['AI_source_unknown_units'],13);self.assertEqual(d['counts']['returned_source_disagreements'],4);self.assertEqual(d['counts']['unreturned_AI_positive'],2)
    def test_WI3_outside66_remain_unknown(self):self.assertEqual(self.data['debt'][2]['counts']['unlabelled_or_unmapped_slots'],66)
    def test_no_model_role_or_independent_approval(self):self.assertEqual(self.data['summary']['new_paid_or_external_model_calls'],0);self.assertEqual(self.data['summary']['independent_approved'],0);self.assertTrue(all(x['installed_role_receipt']is None for x in self.data['debt']))
    def test_sandbox_denied_CKV_not_full_retrieval_readiness(self):self.assertFalse(self.data['summary']['full_retrieval_readiness_verified']);self.assertEqual(self.data['summary']['startup_identity_probe_failures'],2);self.assertEqual(self.data['summary']['CKV_state'],'degraded_startup_identity_probe_sandbox_denied')
    def test_readonly_inventory_preserved(self):self.assertTrue(self.data['summary']['retained_preserved']);self.assertGreater(self.data['summary']['retained_inventory_files'],1000)
    def test_same_direct_name_without_valid_response_cannot_be_success(self):
        rows=g.packed(g.ROOT/g.PACKET/'runtime/all-symbols/responses.json.gz');rows[0]['response']={'error':{'message':'control'}};self.assertEqual(g.diagnostic_rows(rows,g.symbols())[0]['state'],'protocol_error')
    def test_tool_error_not_empty_success(self):
        rows=g.packed(g.ROOT/g.PACKET/'runtime/all-symbols/responses.json.gz');rows[0]['response']={'result':{'isError':True}};self.assertEqual(g.diagnostic_rows(rows,g.symbols())[0]['state'],'tool_error')
    def test_response_foreign_seed_invalid(self):
        rows=g.packed(g.ROOT/g.PACKET/'runtime/all-symbols/responses.json.gz');rows[0]['response']['result']['structuredContent']['seed']='other';self.assertEqual(g.diagnostic_rows(rows,g.symbols())[0]['state'],'invalid_response')
    def test_direct_planned_slot_cannot_drop(self):
        rows=g.packed(g.ROOT/g.PACKET/'runtime/all-symbols/responses.json.gz');rows.pop()
        with self.assertRaises(ValueError):g.diagnostic_rows(rows,g.symbols())
    def test_direct_order_cannot_relabel(self):
        rows=g.packed(g.ROOT/g.PACKET/'runtime/all-symbols/responses.json.gz');rows.reverse()
        with self.assertRaises(ValueError):g.diagnostic_rows(rows,g.symbols())
    def test_requested_cap_not_enlarged(self):
        rows=g.packed(g.ROOT/g.PACKET/'runtime/all-symbols/responses.json.gz');rows[0]['arguments']['max_total']=100
        with self.assertRaises(ValueError):g.diagnostic_rows(rows,g.symbols())
    def test_real_name_never_required(self):self.assertFalse(g.contract()['privacy']['real_name_required'])


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change,compressed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);x=p/name;raw=x.read_bytes();v=json.loads(gzip.decompress(raw) if compressed else raw);old=copy.deepcopy(v);change(v);self.assertNotEqual(v,old);raw=g.f.canonical(v);x.write_bytes(gzip.compress(raw,mtime=0) if compressed else raw);m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p))
    def test_current_packet_and_indices(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_generic_labels_claimed_as102outages(self):self.mutant('SUMMARY.json',lambda v:v.update(historical_backend_outage_count=102))
    def test_exclusive_past_cause_claimed(self):self.mutant('SUMMARY.json',lambda v:v.update(exclusive_historical_cause_proven=True))
    def test_original_partial_replaced_with_current_success(self):self.mutant('SUMMARY.json',lambda v:v.update(original_partial=0))
    def test_WI2_debt_falsely_resolved(self):self.mutant('REVIEW-LEDGER.json',lambda v:v[1].update(independently_resolved=True))
    def test_actual_call_denominator_filtered(self):self.mutant('runtime/all-symbols/responses.json.gz',lambda v:v.pop(),True)
    def test_inventory_mutation(self):self.mutant('runtime/all-symbols/after.json.gz',lambda v:v.update({'graph/graph.db':'0'*64}),True)
    def test_wrong_binary_binding(self):self.mutant('runtime/identity.json',lambda v:v.update(binary_sha256='0'*64))
    def test_negative_control_faked_success(self):self.mutant('runtime/direct/responses.json.gz',lambda v:v[-1]['response']['result'].update(isError=False),True)
    def test_cleanup_not_zero(self):self.mutant('runtime/all-symbols/session.json',lambda v:v.update(exit=1))
    def test_semantic_score_fabricated(self):self.mutant('SUMMARY.json',lambda v:v.update(quality_scores={'precision':1}))
    def test_formal_completed_fabricated(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
