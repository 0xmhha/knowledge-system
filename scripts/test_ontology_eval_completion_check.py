"""Task closure cannot hide incomplete measurements or invent quality approval."""
import copy
import importlib.util
from pathlib import Path
import unittest

S=importlib.util.spec_from_file_location('completion',Path(__file__).with_name('ontology-eval-completion-check.py'))
C=importlib.util.module_from_spec(S);S.loader.exec_module(C)

class CompletionContract(unittest.TestCase):
    def setUp(self):
        self.book={'scope':'source_bound_candidate_observation','questions':[dict(id=str(i),kind='retrieval',independent_FINAL_eligible=False)for i in range(13)]+[dict(id='g'+str(i),kind='trace_guard',independent_FINAL_eligible=False)for i in range(2)]}
        self.protocol=dict(scope=self.book['scope'],planned_retrieval_questions=13,retrieval_question_ids=[str(i)for i in range(13)],approved_independent_units=0,counts=dict(warmup=2,retrieval=5,warm_latency=20,cold_process=0),proposed_warm_p95_ratio_max=1.25)
        arm=dict(planned_calls=351,observed_calls=351,failed_calls=0,missing_calls=0,warm_p95_ns=100)
        self.result=dict(scope=self.book['scope'],approved_independent_units=0,arms=dict(off=copy.deepcopy(arm),on=copy.deepcopy(arm)),verified_pairs=351,failed_or_missing_pairs=0,pairs=[{}]*351,execution_runs=[dict(exit=0,observed=27,planned=27)]*26,trace_guards={'g0':dict(passed=True),'g1':dict(passed=True)},backend={k:dict(off=0,on=0)for k in ('failed_HTTP_attempts','pending_calls','unbound_request_scopes')},warm_p95_ratio_on_to_off=1.0,pilot_cost_budget_observation='within_budget',location_metrics_evaluated=True,answer_quality_evaluated=False,new_FINAL_samples=0,quality_verdict='inconclusive: no independent review',**{k:True for k in ('paired_execution_inputs_verified','model_preflight_digest_verified','held_index_and_semantic_SHA_preserved','operator_source_current_config_preserved')})
        self.verdict=dict(approved_independent_units=0,cost_budget_observation='within_budget',new_FINAL_samples=0,human_independence_review=False,independent_evaluation_eligibility='not_established',independent_quality_accepted=False,quality_verdict=self.result['quality_verdict'],task_completion_basis='specified_evaluation_and_limitations_recorded',criteria={k:['evidence']for k in ('O04-A','O04-B','O04-C','O04-D')},limitations=['no independent review'],full_task_complete=True,**{k:False for k in ('default_on','activation','release_eligible','code_relation_approval','WBFT_criterion_acceptance','public_transfer')})
    def check(self):return C.check(self.book,self.protocol,self.result,self.verdict)
    def test_honest_inconclusive_closure_keeps_quality_unaccepted(self):
        r=self.check();self.assertTrue(r['task_complete']);self.assertFalse(r['quality_accepted'])
    def test_no_quality_approval_from_ten_proposed_clusters(self):
        self.verdict['approved_independent_units']=10
        with self.assertRaises(ValueError):self.check()
    def test_incomplete_calls_pairs_guards_backend_and_preservation_rejected(self):
        for mutate in [lambda r:r['arms']['off'].update(missing_calls=1),lambda r:r.update(verified_pairs=350),lambda r:r['trace_guards'].pop('g0'),lambda r:r['backend']['pending_calls'].update(on=1),lambda r:r.update(held_index_and_semantic_SHA_preserved=False),lambda r:r['execution_runs'].pop()]:
            saved=copy.deepcopy(self.result);mutate(self.result)
            with self.assertRaises(ValueError):self.check()
            self.result=saved
    def test_final_human_review_and_answer_claims_rejected(self):
        for field,value in [('new_FINAL_samples',13),('human_independence_review',True),('independent_quality_accepted',True)]:
            saved=copy.deepcopy(self.verdict);self.verdict[field]=value
            with self.assertRaises(ValueError):self.check()
            self.verdict=saved
        self.result['answer_quality_evaluated']=True
        with self.assertRaises(ValueError):self.check()
    def test_bad_latency_ratio_and_hidden_over_budget_rejected(self):
        self.result['arms']['on']['warm_p95_ns']=150
        with self.assertRaises(ValueError):self.check()
        self.result['warm_p95_ratio_on_to_off']=1.5
        with self.assertRaises(ValueError):self.check()
        self.result['pilot_cost_budget_observation']=self.verdict['cost_budget_observation']='over_budget'
        self.assertEqual(self.check()['status'],'PASS')
    def test_release_activation_and_relation_acceptance_rejected(self):
        for key in ('default_on','activation','release_eligible','code_relation_approval','WBFT_criterion_acceptance','public_transfer'):
            self.verdict[key]=True
            with self.assertRaises(ValueError):self.check()
            self.verdict[key]=False
    def test_changed_inventory_and_missing_criterion_rejected(self):
        self.verdict['criteria'].pop('O04-C')
        with self.assertRaises(ValueError):self.check()
        self.setUp();self.book['questions'][0]['independent_FINAL_eligible']=True
        with self.assertRaises(ValueError):self.check()
    def test_missing_limitations_and_unknown_closure_basis_rejected(self):
        self.verdict['limitations']=[]
        with self.assertRaises(ValueError):self.check()
        self.verdict['limitations']=['limit'];self.verdict['task_completion_basis']='quality_passed'
        with self.assertRaises(ValueError):self.check()

if __name__=='__main__':unittest.main()
