"""Shared-ticket completeness, non-independent oracle and mutation controls."""
import copy,json,shutil,tempfile,unittest
from pathlib import Path
import ticket_dev_gold as g
import evidence_revision


class SourceAndOracle(unittest.TestCase):
    @classmethod
    def setUpClass(cls): cls.data=g.derive()
    def trace(self):
        events=[dict(type='call_start',call_id='call1'),dict(type='uninstall_request',request_id='r1'),
            dict(type='removed',request_id='r1'),dict(type='ack',request_id='r1'),dict(type='call_return',call_id='call1'),
            dict(type='cleanup_done',owned_remaining=0)]
        return dict(kind='controlled_AI_DEV_trace',subscription_id='sub1',complete_event_types=list(g.EVENTS),expected_call_ids=['call1'],
            events=[dict(e,seq=i,subscription_id='sub1') for i,e in enumerate(events)])
    def append(self,t,kind,**kw):t['events'].append(dict(type=kind,seq=len(t['events']),subscription_id='sub1',**kw))
    def test_one_ticket_not_two_independent_work_items(self):
        s=self.data['summary'];self.assertEqual(s['DEV_tickets'],1);self.assertEqual(s['work_items'],['WI-V1','WI-V2']);self.assertEqual(s['independent_tickets'],0)
    def test_all_25_cells_all12_requirements_partitioned(self):
        self.assertEqual(len(self.data['matrix']),25)
        for r in self.data['matrix']:
            self.assertEqual(len(r['units']),12)
            self.assertEqual(sorted(r['unit_ids']),sorted(k for v in r['answer_sets'].values() for k in v))
            self.assertTrue(all(r['empty_set_rationales'].get(k) for k,v in r['answer_sets'].items() if not v))
    def test_conditional_expected_matrix_has18fits7conflicts(self):
        self.assertEqual(self.data['summary']['local_method_fits'],18);self.assertEqual(self.data['summary']['local_method_conflicts'],7)
    def test_reintroduction_alias_not_independent_source(self):
        self.assertEqual(self.data['summary']['unique_method_byte_variants'],4)
        self.assertEqual(self.data['overlays']['ORIGINAL'],self.data['overlays']['MUT-REINTRO'])
        self.assertEqual(self.data['diffs']['ORIGINAL'],b'')
    def test_candidate_overlay_changes_only_exact_original_method(self):
        original=self.data['overlays']['ORIGINAL'];method=(g.ROOT/g.TICKET/'original.go.txt').read_bytes()
        for c in self.data['candidates']:
            new=(g.ROOT/c['artifact']).read_bytes()
            self.assertEqual(self.data['overlays'][c['id']],original.replace(method,new,1))
            self.assertEqual(g.f.sha(self.data['overlays'][c['id']]),c['full_file_overlay_sha256'])
            self.assertIsNone(c['applied_project_commit']);self.assertIsNone(c['applied_project_tree'])
    def test_local_fits_never_actual_API_membership(self):
        m=self.data['membership'];self.assertEqual(m['AI_bounded_answer_sets']['acceptable'],['CAND-A','CAND-B'])
        self.assertEqual(len(m['actual_API_unknown']),5);self.assertIsNone(m['independent_acceptable_candidate_set']);self.assertIsNone(m['canonical_selection'])
    def test_every_API_requirement_remains_runtime_unknown(self):
        for r in self.data['matrix']:
            self.assertTrue({'API-ENTRY','API-UNSUB','API-NEGATIVE'}.issubset(r['answer_sets']['runtime_unknown']))
            self.assertIn('OWNED-CLEANUP',r['answer_sets']['runtime_unknown']);self.assertIn('SOURCE-BOUNDARY',r['answer_sets']['runtime_unknown'])
    def test_early_return_is_not_normal_handshake(self):
        for r in self.data['matrix']:
            if r['candidate_id']=='MUT-EARLY':self.assertIn('METHOD-ACK',r['answer_sets']['source_conflict'])
    def test_source_anchor_text_hash_complete(self):
        self.assertEqual(len(self.data['anchors']),13)
        for a in self.data['anchors'].values():self.assertEqual(g.f.sha(a['text'].encode()),a['span_sha256'])
    def test_secondary_is_explicitly_same_author_not_independent(self):
        self.assertEqual(self.data['secondary']['kind'],'AI_source_wait_dependency_graph_draft_not_independent')
        self.assertIn('Same AI',self.data['secondary']['shared_bias'])
    def test_public_ticket_no_candidate_labels(self):
        p=self.data['public'];self.assertFalse(p['contains_gold']);self.assertEqual(p['split'],'DEV')
        self.assertTrue({'requirements','candidates','acceptable_candidates','answer_sets'}.isdisjoint(p))
    def test_controlled_trace_pass_not_API_or_independent(self):
        r=g.trace_oracle(self.trace());self.assertEqual(r['status'],'BOUNDED_PASS');self.assertFalse(r['independent_oracle']);self.assertFalse(r['actual_API_receipt']);self.assertIsNone(r['quality_score'])
    def test_RPC_true_does_not_substitute_downstream_ack(self):
        t=self.trace();t['events']=t['events'][:2];self.append(t,'rpc_unsubscribe_true');self.append(t,'call_return',call_id='call1');self.append(t,'cleanup_done',owned_remaining=0)
        r=g.trace_oracle(t);self.assertEqual(r['status'],'BOUNDED_FAIL');self.assertIn('return_before_ack',r['violations'])
    def test_missing_trace_events_unknown_not_pass(self):
        t=self.trace();t['events']=[];self.assertEqual(g.trace_oracle(t)['status'],'UNKNOWN')
        t.pop('complete_event_types');self.assertEqual(g.trace_oracle(t)['status'],'UNKNOWN')
    def test_bad_call_population_not_exception_or_pass(self):
        for ids in ([[]],['call1','call1'],[],[123]):
            t=self.trace();t['expected_call_ids']=ids;self.assertEqual(g.trace_oracle(t)['status'],'UNKNOWN')
    def test_status_channel_use_after_return_is_violation(self):
        t=self.trace();self.append(t,'status_use');self.assertIn('status_use_after_return',g.trace_oracle(t)['violations'])
    def test_ack_before_removal_is_violation(self):
        t=self.trace();t['events'][2],t['events'][3]=t['events'][3],t['events'][2]
        for i,e in enumerate(t['events']):e['seq']=i
        self.assertIn('ack_before_removal',g.trace_oracle(t)['violations'])
    def test_duplicate_uninstall_is_not_pass(self):
        t=self.trace();t['events'].insert(2,dict(type='uninstall_request',request_id='r2',subscription_id='sub1'))
        for i,e in enumerate(t['events']):e['seq']=i
        r=g.trace_oracle(t);self.assertEqual(r['status'],'BOUNDED_FAIL');self.assertIn('more_than_one_request',r['violations'])
    def test_foreign_handshake_is_error_not_symptom_RED(self):
        t=self.trace();t['events'][2]['request_id']='foreign';self.assertEqual(g.trace_oracle(t)['status'],'ERROR')
    def test_invalid_event_order_and_foreign_subscription_error(self):
        for key,value in [('seq',True),('seq',0),('subscription_id','other'),('type','unobserved')]:
            t=self.trace();t['events'][2][key]=value;self.assertEqual(g.trace_oracle(t)['status'],'ERROR')
    def test_unreturned_second_call_remains_unknown(self):
        t=self.trace();t['expected_call_ids'].append('call2');self.assertEqual(g.trace_oracle(t)['status'],'UNKNOWN')
    def test_missing_normal_notification_unknown(self):
        t=self.trace();t['requires_notification']=True;self.assertEqual(g.trace_oracle(t)['status'],'UNKNOWN')
    def test_owned_cleanup_error_not_pass(self):
        for n in (1,None,True):
            t=self.trace();t['events'][-1]['owned_remaining']=n;self.assertEqual(g.trace_oracle(t)['status'],'ERROR')
    def test_early_cleanup_receipt_not_final_cleanup_proof(self):
        t=self.trace();t['events'][-1],t['events'][-2]=t['events'][-2],t['events'][-1]
        for i,e in enumerate(t['events']):e['seq']=i
        self.assertEqual(g.trace_oracle(t)['status'],'UNKNOWN')


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);x=p/name;v=g.f.read(x);original=copy.deepcopy(v);change(v)
            self.assertNotEqual(v,original,'mutation must change the selected fixture')
            x.write_text(json.dumps(v,ensure_ascii=False))
            m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
            (p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p))
    def test_current_packet_and_live_references(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_missing_requirement_unit(self):self.mutant('private/AI-matrix.json',lambda v:v[0]['units'].pop())
    def test_empty_set_rationale_erased(self):self.mutant('private/AI-matrix.json',lambda v:v[0]['empty_set_rationales'].clear())
    def test_API_unknown_promoted_to_fit(self):self.mutant('private/AI-matrix.json',lambda v:v[0]['units'][0].update(label='conditional_source_fit'))
    def test_invented_candidate_tree(self):self.mutant('private/candidates.json',lambda v:v[1].update(applied_project_tree='a'*40))
    def test_source_witness_tamper(self):self.mutant('private/source-anchors.json',lambda v:v['API-01'].update(span_sha256='0'*64))
    def test_independent_oracle_fabricated(self):self.mutant('private/secondary-derivation.json',lambda v:v.update(kind='independent_executable_oracle'))
    def test_empty_approved_membership_fabricated(self):self.mutant('private/AI-membership.json',lambda v:v.update(independent_acceptable_candidate_set=[]))
    def test_A_chosen_without_actual_API_gate(self):self.mutant('private/AI-membership.json',lambda v:v.update(canonical_selection='CAND-A'))
    def test_reintro_counted_as_independent_source(self):self.mutant('SUMMARY.json',lambda v:v.update(unique_method_byte_variants=5))
    def test_ticket_double_counted(self):self.mutant('SUMMARY.json',lambda v:v.update(DEV_tickets=2))
    def test_actual_API_calls_invented(self):self.mutant('SUMMARY.json',lambda v:v.update(actual_API_calls=5))
    def test_public_candidate_leak(self):self.mutant('public/DEV-ticket.json',lambda v:v.update(acceptable_candidates=['CAND-A']))
    def test_public_final_relabel(self):self.mutant('public/DEV-ticket.json',lambda v:v.update(split='FINAL'))
    def test_review_debt_erased(self):self.mutant('REVIEW-LEDGER.json',lambda v:v.update(missing=[]))
    def test_no_abstention_contract(self):self.mutant('matching-contract.json',lambda v:v.update(candidate_decision='always pick A'))
    def test_reviewer_and_formal_completion(self):self.mutant('WORKLIST.json',lambda v:v.update(reviewer_assignment='AI',formal_completed=2))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
