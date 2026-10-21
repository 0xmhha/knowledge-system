"""Source identity, current consumer structure and false semantic completion tests."""
import copy,gzip,json,shutil,tempfile,types,unittest
from pathlib import Path
import wic_dev_gold as g
import evidence_revision


class DraftTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.data=g.derive();cls.gold={r['candidate_id']:r for r in cls.data['gold']}
    def test10_reviews8modules_not10independent_families(self):
        self.assertEqual(len(self.gold),10);self.assertEqual(self.data['summary']['modules'],8);self.assertEqual(self.data['summary']['independent_families'],0)
    def test64_exact_source_observations_and20_unknown_followups(self):
        self.assertEqual(sum(len(r['required_source_observations']) for r in self.gold.values()),64)
        self.assertEqual(sum(len(r['required_followups']) for r in self.gold.values()),20)
        for r in self.gold.values():
            for a in r['required_source_observations']:self.assertEqual(g.f.sha(a['text'].encode()),a['source']['span_sha256']);self.assertIsNone(a['semantic_match'])
            for a in r['required_followups']:self.assertEqual(a['answer_status'],'unknown');self.assertIsNone(a['answer'])
    def test_helper_name_normalized_preserves_original(self):
        q=self.gold['SRC-001']['required_followups'][0];self.assertIn('getStartEnd',q['original']);self.assertIn('calculateBlockRange',q['question'])
    def test_optional_experiments_not_actual_runs(self):self.assertTrue(all(r['optional_experiment']['actual']is None for r in self.gold.values()))
    def test_required_product_gold_not_invented(self):
        for r in self.gold.values():self.assertFalse(r['semantic_required_gold_complete']);self.assertIsNone(r['omission_count']);self.assertIsNone(r['priority_correctness'])
    def test_priorities_remain_conditional_not_assigned(self):
        for r in self.gold.values():self.assertIsNone(r['priority']['assigned_level']);self.assertEqual(r['priority']['allowed_consumer_enum'],['high','medium','low'])
    def test_public_has_no_gold_and_matches_exact_function_bytes(self):
        for p,r in zip(self.data['public'],self.data['gold']):
            self.assertFalse(p['contains_gold']);self.assertTrue({'required_source_observations','required_followups','unsupported_conclusions'}.isdisjoint(p))
            self.assertEqual((g.ROOT/g.PACKET/'public'/p['source_excerpt']).read_bytes(),r['source']['body'].encode())
    def test_adopted_conflict_is_resolved_not_reopened(self):self.assertEqual(self.data['consumer']['prior_alias_and_priority_conflict'],'resolved_by_adopted_candidate_not_reopened')
    def test_render_not_claimed_from_diagram_bytes(self):self.assertIsNone(self.data['consumer']['actual_render_receipt']);self.assertIn('does not prove diagram rendering',self.data['consumer']['limitations'])
    def test_sixsection_and_id_presence_not_semantic_gold(self):self.assertIn('criterion/section/id/name presence',g.contract()['matching']);self.assertIsNone(self.data['summary']['actual_quality_scores'])
    def test_real_name_not_required(self):self.assertFalse(g.review_ledger(self.data)['reviewer_identity_policy']['real_name_required'])


class AdoptedConsumerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        p=g.ROOT/g.PACKET/'private/consumer';proof=g.verify(p,g.CONSUMER_REV)
        path=g.safe(p,proof['files']['plugin/scripts/deep_review.py']['artifact']);cls.review=types.ModuleType('wic_adopted_review');exec(compile(path.read_text(),str(path),'exec'),cls.review.__dict__)
        cls.b=json.loads(g.safe(p,proof['files']['plugin/scripts/tests/fixtures/deep-review-bundle.json']['artifact']).read_text());cls.j=json.loads(g.safe(p,proof['files']['plugin/scripts/tests/fixtures/deep-review-judgments.json']['artifact']).read_text())
    def build(self,j=None,b=None):return self.review.build(copy.deepcopy(self.b if b is None else b),copy.deepcopy(self.j if j is None else j))
    def test_fixture_is_synthetic_not_actual_project_receipt(self):
        r=self.build();self.assertEqual(r['status'],'structured_partial_review');self.assertEqual(r['independent_acceptance'],'not_recorded')
        self.assertEqual(g.derive()['summary']['actual_project_dual_tool_calls'],0)
    def test_snapshot_identity_is_not_atomic(self):self.assertEqual(self.build()['structural_review']['snapshot_association'],'identity_only_unverified_snapshot')
    def test_cfg_not_runtime_sequence_and_modules_not_callgraph(self):
        r=self.build();self.assertIn('flowchart TD',self.review.sequence_diagram(r));self.assertEqual(r['computed']['sequence']['kind'],'bounded_cfg_not_execution_trace');self.assertEqual(r['computed']['module']['interprocedural_edges'],'unknown')
    def test_critical_priority_denied(self):
        j=copy.deepcopy(self.j);j['findings'][0]['priority']='critical'
        with self.assertRaises(self.review.Invalid):self.build(j=j)
    def test_33_findings_denied(self):
        j=copy.deepcopy(self.j);j['findings']=[dict(j['findings'][0],id='F'+str(i)) for i in range(33)]
        with self.assertRaises(self.review.Invalid):self.build(j=j)
    def test_priority_format_does_not_prove_correctness(self):
        for priority in ('high','medium','low'):
            j=copy.deepcopy(self.j);j['findings'][0]['priority']=priority;self.assertEqual(self.build(j=j)['independent_acceptance'],'not_recorded')
    def test_criterion_id_and_arbitrary_text_not_semantically_verified(self):
        j=copy.deepcopy(self.j);j['criteria'][0]['requirement']='unreviewed unrelated requirement';r=self.build(j=j);self.assertEqual(r['reasoning']['criteria'][0]['requirement'],j['criteria'][0]['requirement']);self.assertEqual(r['independent_acceptance'],'not_recorded')
    def test_all_unknown_placeholder_denied(self):
        j=copy.deepcopy(self.j)
        for s in j['sections'].values():s.update(status='unknown',unknowns=['unreviewed'])
        with self.assertRaises(self.review.Invalid):self.build(j=j)
    def test_foreign_pins_denied(self):
        b=copy.deepcopy(self.b);b['rules']['arguments']['function']['expected_commit']='0'*40
        with self.assertRaises(self.review.Invalid):self.build(b=b)
    def test_confirmed_vulnerability_denied(self):
        j=copy.deepcopy(self.j);j['findings'][0]['state']='confirmed'
        with self.assertRaises(self.review.Invalid):self.build(j=j)
    def workspace(self,p):
        self.review.write_review(p,copy.deepcopy(self.b),copy.deepcopy(self.j));raw=(p/'deep-review.json').read_bytes()
        return dict(ticket_id=self.b['ticket_id'],ticket_type='code_review',current_state='PLANNING',states={'ANALYSIS':{'status':'completed','deep_review_sha256':self.review.sha(raw)},'PLANNING':{'status':'pending'}})
    def test_five_artifacts_bound_and_cli_scope_is_delivery_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp);state=self.workspace(p);self.assertEqual({x.name for x in p.iterdir()},set(g.ARTIFACTS));result=self.review.verify_workspace(p,state);self.assertEqual(result['status'],'validated_delivery');self.assertEqual(result['independent_acceptance'],'not_recorded')
    def test_completion_requires_planner_report_binding(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp);state=self.workspace(p);state['current_state']='COMPLETED'
            with self.assertRaises(self.review.Invalid):self.review.verify_workspace(p,state)
            state['states']['PLANNING']={'status':'completed','review_report_sha256':self.review.sha((p/'review-report.md').read_bytes())};self.assertEqual(self.review.verify_workspace(p,state)['independent_acceptance'],'not_recorded')
    def test_any_diagram_byte_tamper_denied(self):
        for name in ('sequence.mmd','modules.mmd'):
            with tempfile.TemporaryDirectory() as tmp:
                p=Path(tmp);state=self.workspace(p);(p/name).write_text('flowchart TD\nchanged')
                with self.assertRaises(self.review.Invalid):self.review.verify_workspace(p,state)
    def test_stale_analysis_hash_denied(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp);state=self.workspace(p);state['states']['ANALYSIS']['deep_review_sha256']='0'*64
            with self.assertRaises(self.review.Invalid):self.review.verify_workspace(p,state)


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change,compressed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);x=p/name;raw=x.read_bytes();v=json.loads(gzip.decompress(raw) if compressed else raw);old=copy.deepcopy(v);change(v);self.assertNotEqual(v,old)
            raw=g.f.canonical(v);x.write_bytes(gzip.compress(raw,mtime=0) if compressed else raw);m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p))
    def test_current_packet_and_live_indices(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_omission0_fabricated(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0].update(omission_count=0),True)
    def test_priority_correctness_fabricated(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0].update(priority_correctness=True),True)
    def test_full_gold_fabricated(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0].update(semantic_required_gold_complete=True),True)
    def test_source_claim_deleted(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['required_source_observations'].pop(),True)
    def test_unknown_followup_answer_invented(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['required_followups'][0].update(answer='safe'),True)
    def test_current_conflict_reopened(self):self.mutant('consumer-contract.json',lambda v:v.update(priority_enum=['critical','high']))
    def test_actual_installed_run_fabricated(self):self.mutant('SUMMARY.json',lambda v:v.update(actual_installed_role_runs=10))
    def test_6section_presence_promoted_to_semantic_matching(self):self.mutant('matching-contract.json',lambda v:v.update(matching='criterion id presence is sufficient'))
    def test_debt_erased(self):self.mutant('REVIEW-LEDGER.json',lambda v:v.update(missing=[]))
    def test_public_gold_leak(self):self.mutant('public/DEV-inputs.json',lambda v:v[0].update(required_gold=[]))
    def test_formal_acceptance_fabricated(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
