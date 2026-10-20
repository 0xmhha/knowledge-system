"""Schedule counterexamples, source boundaries and false-completion controls."""
import copy
import json
from pathlib import Path
import shutil
import tempfile
import unittest

import wi4_dev_gold as g


class SourceAndSchedule(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.data = g.derive(); cls.rows = {r['case_id']: r for r in cls.data['gold']}

    def labels(self, name, label): return self.rows[name]['answer_sets'][label]

    def observation(self):
        r = self.rows['WORK-both']
        return dict(case_id=r['case_id'], family_proposal=r['family_proposal'],
                    source_commit=r['source_commit'], source_project='go-stablenet', source_mode='committed',
                    binary_sha256='a'*64, driver_sha256='b'*64, environment_sha256='c'*64,
                    entry_reached=True, preconditions_verified=True, seed='controlled', arm='baseline',
                    order=0, trace_sha256='d'*64, outcome_id='send_chosen')

    def test_all_original_schedules_have_complete_finite_partition(self):
        self.assertEqual(len(self.rows), 21)
        self.assertEqual(self.data['summary']['outcome_units'], 150)
        for r in self.rows.values():
            self.assertEqual(sorted(u['id'] for u in r['universe']), sorted(k for v in r['answer_sets'].values() for k in v))
            self.assertTrue(r['complete_within_declared_vocabulary']); self.assertFalse(r['complete_trace_or_product_gold'])
            self.assertTrue(all(r['empty_set_rationales'].get(k) for k,v in r['answer_sets'].items() if not v))

    def test_each_schedule_has_unknown_and_four_failure_states(self):
        for r in self.rows.values():
            self.assertIn('unmodelled_trace', r['answer_sets']['unknown'])
            self.assertEqual(r['answer_sets']['infra_or_unreached'], [k for k,_ in g.INFRA])

    def test_ready_select_has_both_legal_outcomes(self):
        self.assertEqual(self.labels('WORK-both', 'legal'), ['send_chosen', 'exit_chosen'])
        self.assertEqual(self.labels('WORK-both', 'symptom_candidate'), [])

    def test_closure_and_outer_return_differ(self):
        self.assertIn('closure_returns_without_send', self.labels('WORK-exit','legal'))
        self.assertIn('outer_loop_event_or_exit', self.labels('WORK-exit','unknown'))

    def test_backlog_is_not_deterministic_starvation_or_leak(self):
        self.assertEqual(len(self.labels('WORK-backlog','legal')),2)
        self.assertIn('finite_window_no_return',self.labels('WORK-backlog','unknown'))

    def test_udp_same_mechanism_branches_not_extra_families(self):
        names=[n for n in self.rows if n.startswith('UDP-')]
        self.assertEqual({self.rows[n]['family_proposal'] for n in names},{'SCHED-002'})
        self.assertIn('response_not_done_continues',self.labels('UDP-response-timeout','legal'))

    def test_synthetic_fix_outcome_never_original_observation(self):
        self.assertIn('saved_successor_both_removed',self.labels('UDP-far-pair','unknown'))
        self.assertIn('first_removed_second_left_timer_stop',self.labels('UDP-far-pair','symptom_candidate'))
        self.assertFalse(self.data['toolchain']['target_runtime_adopted'])

    def test_gossip_suppression_not_product_violation(self):
        self.assertIn('one_attempt_second_suppressed',self.labels('GOSSIP-fail-repeat','legal'))
        self.assertEqual(self.labels('GOSSIP-fail-repeat','symptom_candidate'),[])
        self.assertIn('suppression_violates_retry',self.labels('GOSSIP-fail-repeat','unknown'))

    def test_pool_buffer_control_not_all_load_guarantee(self):
        self.assertIn('finite_buffer_all_load_safe',self.labels('POOL-buffer','unknown'))
        self.assertEqual(self.labels('POOL-buffer','symptom_candidate'),[])
        self.assertIn('no other blocked subscriber', self.rows['POOL-buffer']['strengthened_preconditions'])

    def test_pool_cycle_is_conditional_not_production_reachability(self):
        self.assertIn('lock_send_cycle_before_rescue',self.labels('POOL-lock-cycle','symptom_candidate'))
        self.assertIsNone(self.rows['POOL-lock-cycle']['actual_target_trial_receipt'])

    def test_subscription_rescue_and_ack_are_separate(self):
        self.assertIn('rescue_then_ack_return',self.labels('SUB-blocked','legal'))
        self.assertIn('two_waits_before_rescue',self.labels('SUB-blocked','symptom_candidate'))
        self.assertIn('early_return',self.labels('SUB-ack','symptom_candidate'))

    def test_same_once_object_is_explicit_precondition(self):
        self.assertIn('two calls share the same fresh subscription object', self.rows['SUB-once']['strengthened_preconditions'])

    def test_every_source_span_hash_and_bytes_rechecked(self):
        for r in self.rows.values():
            for a in r['source_refs']:
                raw=(g.ROOT/g.SOURCE/'sources'/(a['path']+'.txt')).read_bytes()
                self.assertEqual(g.f.sha(raw),a['file_sha256'])
                self.assertEqual(raw[a['start_byte']:a['end_byte_exclusive']].decode(),a['text'])
                self.assertEqual(g.f.sha(a['text'].encode()),a['span_sha256'])

    def test_public_does_not_include_private_labels_or_outcome_sets(self):
        self.assertEqual(len(self.data['public']),21)
        for r in self.data['public']:
            self.assertFalse(r['contains_gold']);self.assertEqual(r['split'],'DEV')
            self.assertTrue({'universe','answer_sets','source_refs','outcomes','rationale'}.isdisjoint(r))

    def test_fourth_unknown_set_not_silently_infra_or_legal(self):
        self.assertEqual(set(self.rows['WORK-both']['answer_sets']),set(g.LABELS))
        self.assertTrue(self.rows['WORK-both']['formal_gold_schema_gap'])

    def test_full_identity_still_only_ai_vocabulary(self):
        result=g.match_observation(self.observation(),self.rows['WORK-both'])
        self.assertEqual(result['label'],'legal');self.assertFalse(result['independent_oracle'])
        self.assertFalse(result['product_violation_established']);self.assertIsNone(result['quality_score'])

    def test_scope_mismatch_never_name_match(self):
        for k in ('case_id','family_proposal','source_commit','source_project','source_mode'):
            o=self.observation();o[k]='wrong'
            self.assertEqual(g.match_observation(o,self.rows['WORK-both'])['status'],'refused_wrong_scope')

    def test_missing_or_bad_runtime_hash_unknown(self):
        for k in ('binary_sha256','driver_sha256','environment_sha256'):
            for value in ('','bad','x'*64,None,True,123,[]):
                o=self.observation();o[k]=value
                self.assertEqual(g.match_observation(o,self.rows['WORK-both'])['status'],'unknown_unbound_runtime')

    def test_false_entry_or_precondition_or_bad_order_unknown(self):
        for k,value in [('entry_reached',False),('preconditions_verified',False),('order',True),('order',-1),('trace_sha256','bad'),('trace_sha256',123),('seed',''),('seed',123),('arm','other')]:
            o=self.observation();o[k]=value
            self.assertEqual(g.match_observation(o,self.rows['WORK-both'])['status'],'unknown_unverified_trial')

    def test_named_outcome_without_receipt_never_runtime_evidence(self):
        self.assertEqual(g.match_observation(dict(outcome_id='send_chosen'),self.rows['WORK-both'])['status'],'unknown_missing_scope')
        for value in (None, [], 'send_chosen'):
            self.assertEqual(g.match_observation(value,self.rows['WORK-both'])['status'],'unknown_missing_scope')

    def test_unmodelled_outcome_stays_unknown(self):
        o=self.observation();o['outcome_id']='new_state'
        self.assertEqual(g.match_observation(o,self.rows['WORK-both'])['status'],'unknown_unmodelled_trace')

    def test_no_runtime_families_or_scores_invented(self):
        s=self.data['summary']
        for k in ('independent_families','independent_approved','FINAL','formal_completed','new_target_trials','new_MCP_calls','new_nodes','new_model_calls'):self.assertEqual(s[k],0)
        self.assertIsNone(s['current_scores']);self.assertIsNone(s['freeze']);self.assertFalse(s['execution_allowed'])


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p)
            path=p/name;v=g.f.read(path);change(v);path.write_text(json.dumps(v,ensure_ascii=False))
            manifest=g.f.read(p/'manifest.json')
            manifest['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
            (p/'manifest.json').write_text(json.dumps(manifest))
            self.assertTrue(g.validate(packet=p),name)

    def test_current_packet_and_live_references(self): self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_missing_schedule(self): self.mutant('private/AI-gold.json',lambda v:v.pop())
    def test_missing_outcome(self): self.mutant('private/AI-gold.json',lambda v:v[0]['universe'].pop())
    def test_unknown_turned_into_pass(self): self.mutant('private/AI-gold.json',lambda v:next(u for u in v[0]['universe'] if u['label']=='unknown').update(label='legal'))
    def test_ready_select_one_branch_erased(self): self.mutant('private/AI-gold.json',lambda v:next(r for r in v if r['case_id']=='WORK-both')['answer_sets']['legal'].pop())
    def test_source_fact_rewritten(self): self.mutant('private/AI-gold.json',lambda v:v[0]['source_refs'][0].update(span_sha256='0'*64))
    def test_fake_actual_trial(self): self.mutant('private/AI-gold.json',lambda v:v[0].update(actual_target_trial_receipt={'exit':0}))
    def test_rationale_erased(self): self.mutant('private/AI-gold.json',lambda v:next(r for r in v if r['case_id']=='WORK-both')['empty_set_rationales'].clear())
    def test_public_oracle_leak(self): self.mutant('public/DEV-inputs.json',lambda v:v[0].update(answer_sets={}))
    def test_final_relabel(self): self.mutant('public/DEV-inputs.json',lambda v:v[0].update(split='FINAL'))
    def test_product_contract_invented(self): self.mutant('private/AI-gold.json',lambda v:v[0].update(complete_trace_or_product_gold=True))
    def test_schedules_promoted_to_families(self): self.mutant('SUMMARY.json',lambda v:v.update(independent_families=21))
    def test_actual_trials_invented(self): self.mutant('SUMMARY.json',lambda v:v.update(new_target_trials=100))
    def test_score_invented(self): self.mutant('SUMMARY.json',lambda v:v.update(current_scores={'ratio':0}))
    def test_target_runtime_adopted(self): self.mutant('toolchain-scope.json',lambda v:v.update(target_runtime_adopted=True))
    def test_debt_erased(self): self.mutant('REVIEW-LEDGER.json',lambda v:v.update(missing=[]))
    def test_watchdog_promoted_to_contract(self): self.mutant('matching-contract.json',lambda v:v.update(select='exit priority and fixed2000ms return'))
    def test_zero_baseline_promoted_to_success(self): self.mutant('matching-contract.json',lambda v:v.update(statistics='zero baseline = success'))
    def test_no_recurrence_promoted_to_success(self): self.mutant('matching-contract.json',lambda v:v.update(observation='no recurrence = accepted fix'))
    def test_reviewer_invented(self): self.mutant('WORKLIST.json',lambda v:v.update(reviewer_assignment='AI'))
    def test_formal_completed(self): self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))


if __name__ == '__main__': unittest.main()
