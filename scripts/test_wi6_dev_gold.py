"""Bounded relevance, seed inflation, caps/scope and false-completion controls."""
import copy,gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi6_dev_gold as g
import evidence_revision


class DraftAndProjection(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.data=g.derive();cls.gold={r['seed_id']:r for r in cls.data['gold']};cls.locations=cls.data['locations']
        cls.pins=dict(project_id='controlled-project',dataset_id='controlled-dataset',commit=g.f.SOURCE_COMMIT,source_mode='committed',graph_snapshot_sha256='a'*64)
    def hit(self,name):
        l=next(l for l in self.locations if l['id']==name)
        return dict(self.pins,**{k:v for k,v in l['identity'].items() if k not in ('project','commit','source_mode')},unit_kind='function')
    def test_400_complete_finite_labels(self):
        self.assertEqual(len(self.gold),20)
        for r in self.gold.values():
            self.assertEqual(len(r['universe']),20)
            self.assertEqual(sorted(u['location_key'] for u in r['universe']),sorted(k for v in r['answer_sets'].values() for k in v))
            self.assertTrue(all(r['empty_set_rationales'].get(k) for k,v in r['answer_sets'].items() if not v))
            self.assertFalse(r['repository_wide_complete']);self.assertFalse(r['formal_scoring_allowed'])
    def test_direct_seeds_all20_relevant_but_excluded(self):
        for r in self.gold.values():
            self.assertIn(r['seed_key'],r['answer_sets']['relevant']);self.assertNotIn(r['seed_key'],r['seed_excluded_relevant'])
    def test_explicit_calls_support_relevant_context(self):
        for seed,target in [('SRC-012','SRC-013'),('SRC-013','SRC-012'),('SRC-016','SRC-018'),('SRC-018','SRC-016'),('SRC-008','SRC-007')]:
            self.assertEqual(next(u for u in self.gold[seed]['universe'] if u['location_id']==target)['label'],'relevant')
    def test_same_storage_not_automatic_caller_failure_context(self):
        self.assertEqual(next(u for u in self.gold['SRC-007']['universe'] if u['location_id']=='SRC-008')['label'],'unknown')
    def test_same_consensus_topic_not_automatic_relevant(self):
        self.assertEqual(next(u for u in self.gold['SRC-002']['universe'] if u['location_id']=='SRC-003')['label'],'unknown')
    def test_missing_transport_construction_remains_unknown(self):
        self.assertEqual(next(u for u in self.gold['SRC-037']['universe'] if u['location_id']=='SRC-036')['label'],'unknown')
    def test_seed_only_recall_is_undefined_not_perfect(self):
        self.assertFalse(self.gold['SRC-001']['seed_excluded_recall_defined']);self.assertEqual(self.gold['SRC-001']['seed_excluded_relevant'],[])
        self.assertIsNone(self.gold['SRC-001']['current_scores'])
    def test_stress_program_read_only_not_operational(self):
        self.assertFalse(self.gold['SRC-026']['operational_trial_eligible'])
        self.assertTrue(all(r['runtime_execution_allowed']is False for r in self.data['public']))
        self.assertEqual(self.data['summary']['production_seed_proposals'],19)
    def test_same_intent_file_or_name_never_independent_families(self):
        self.assertEqual(self.data['summary']['independent_families'],0)
        self.assertTrue(all(r['independent_family']is None for r in self.gold.values()))
    def test_source_git_and_all40_anchors_bound(self):
        self.assertEqual(sum(len(l['source_refs']) for l in self.locations),40)
        for l in self.locations:
            for a in l['source_refs']:self.assertEqual(g.f.sha(a['text'].encode()),a['span_sha256'])
    def test_seed_slot_cannot_supply_context_improvement(self):
        r=g.project_output([self.hit('SRC-012')],self.gold['SRC-012'],self.locations,self.pins,2)
        self.assertTrue(r['raw_slots'][0]['seed']);self.assertFalse(r['raw_slots'][0]['primary_metric_eligible']);self.assertIsNone(r['score'])
    def test_verified_nonseed_relevant_preserved(self):
        r=g.project_output([self.hit('SRC-013')],self.gold['SRC-012'],self.locations,self.pins,2)
        self.assertEqual(r['raw_slots'][0]['AI_label'],'relevant');self.assertTrue(r['raw_slots'][0]['primary_metric_eligible'])
    def test_duplicates_do_not_refill_after_cap(self):
        r=g.project_output([self.hit('SRC-013')]*2+[self.hit('SRC-001')],self.gold['SRC-012'],self.locations,self.pins,2)
        self.assertEqual(len(r['raw_slots']),2);self.assertTrue(r['raw_slots'][1]['duplicate']);self.assertFalse(r['raw_slots'][1]['primary_metric_eligible'])
    def test_unknown_outside_and_invalid_slots_preserved(self):
        r=g.project_output([{},None,self.hit('SRC-036')],self.gold['SRC-037'],self.locations,self.pins,3)
        self.assertEqual(len(r['raw_slots']),3);self.assertEqual(r['raw_slots'][2]['AI_label'],'unknown');self.assertTrue(all(x['primary_metric_eligible']is False for x in r['raw_slots']))
    def test_every_wrong_pin_refused(self):
        for key in self.pins:
            hit=self.hit('SRC-013');hit[key]='wrong'
            r=g.project_output([hit],self.gold['SRC-012'],self.locations,self.pins,1)
            self.assertTrue(r['raw_slots'][0]['match']['status'].startswith('refused_wrong_'))
    def test_missing_identity_or_partial_function_not_joined(self):
        hit=self.hit('SRC-013');hit['start_byte']+=1
        r=g.project_output([hit,dict(name='RecoverPruning')],self.gold['SRC-012'],self.locations,self.pins,2)
        self.assertTrue(all(x['match']['location_key']is None for x in r['raw_slots']))
    def test_bad_caps_and_slot_populations_rejected(self):
        for cap in (True,0,-1,65):
            with self.assertRaises(ValueError):g.project_output([],self.gold['SRC-012'],self.locations,self.pins,cap)
        with self.assertRaises(ValueError):g.project_output(None,self.gold['SRC-012'],self.locations,self.pins,2)
    def test_no_paired_or_warm_results_fabricated(self):
        for r in self.gold.values():
            self.assertIsNone(r['actual_baseline']);self.assertIsNone(r['actual_expansion']);self.assertIsNone(r['warm_samples'])
        self.assertEqual(self.data['summary']['new_MCP_calls'],0);self.assertEqual(self.data['summary']['new_target_executions'],0)


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change,compressed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);x=p/name;raw=x.read_bytes();v=json.loads(gzip.decompress(raw) if compressed else raw);old=copy.deepcopy(v);change(v);self.assertNotEqual(v,old)
            raw=g.f.canonical(v);x.write_bytes(gzip.compress(raw,mtime=0) if compressed else raw)
            m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p))
    def test_current_packet_and_live_references(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_missing_universe_location(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['universe'].pop(),True)
    def test_unknown_promoted_to_noise(self):self.mutant('private/AI-gold.json.gz',lambda v:next(u for u in v[0]['universe'] if u['label']=='unknown').update(label='noise'),True)
    def test_seed_inflated_to_full_recall(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0].update(current_scores={'recall':1}),True)
    def test_actual_pair_invented(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0].update(actual_baseline={'exit':0}),True)
    def test_stress_operationalized(self):self.mutant('private/AI-gold.json.gz',lambda v:next(r for r in v if r['seed_id']=='SRC-026').update(operational_trial_eligible=True),True)
    def test_source_span_hash_tamper(self):self.mutant('private/location-catalog.json',lambda v:v[0]['identity'].update(function_sha256='0'*64))
    def test_independent_family_count_invented(self):self.mutant('SUMMARY.json',lambda v:v.update(independent_families=20))
    def test_public_gold_leak(self):self.mutant('public/DEV-inputs.json',lambda v:v[0].update(answer_sets={}))
    def test_final_relabel(self):self.mutant('public/DEV-inputs.json',lambda v:v[0].update(split='FINAL'))
    def test_seed_policy_changed_without_review(self):self.mutant('matching-contract.json',lambda v:v.update(seed='count seed as recall1'))
    def test_warm_measurement_boundary_changed(self):self.mutant('matching-contract.json',lambda v:v.update(measurement='server-only time'))
    def test_wrong_p95_ratio_definition(self):self.mutant('matching-contract.json',lambda v:v.update(quantile='p95 of paired ratios'))
    def test_undefined_baseline_zero_filled(self):self.mutant('matching-contract.json',lambda v:v.update(empty='zero relevant is recall1'))
    def test_warm_raw_samples_invented(self):self.mutant('REVIEW-LEDGER.json',lambda v:v.update(warm_raw_samples=[1]))
    def test_review_debt_erased(self):self.mutant('REVIEW-LEDGER.json',lambda v:v.update(missing=[]))
    def test_formal_acceptance_invented(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
