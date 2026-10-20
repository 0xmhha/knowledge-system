"""Preserve semantic negatives/unknowns and exact original query denominators."""
import copy,gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi2_contract_audit as g
import evidence_revision


class ScopeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.data=g.derive()
    def test_eight_type_valid_supplements_six_original_fragments(self):self.assertEqual(len(self.data['scope']),8);self.assertEqual(len({r['case_id']for r in self.data['scope']}),6)
    def test_two_shadow_cases_are_local_not_stdlib(self):
        rows=[r for r in self.data['scope']if r['case_id']in('RULE-1-07','RULE-2-07')];self.assertEqual(len(rows),2);self.assertTrue(all(r['compiler_target']=='supplemental_local_method'for r in rows))
    def test_import_aliases_resolve_in_supplemental_full_file_only(self):
        rows={r['case_id']:r for r in self.data['scope']};self.assertEqual(rows['RULE-1-15']['object_package'],'os/exec');self.assertEqual(rows['RULE-2-16']['object_package'],'strconv');self.assertTrue(all('supplemental' in r['scope']for r in rows.values()))
    def test_opaque_same_F_bytes_can_have_distinct_helper_effects(self):
        rows=g.f.read(g.ROOT/g.PACKET/'compiler/witness-catalog.json')
        for id in ('RULE-1-05','RULE-2-06'):
            p=[r for r in rows if r['original_case_id']==id];self.assertEqual(p[0]['fragment_sha256'],p[1]['fragment_sha256']);self.assertNotEqual(p[0]['full_file_sha256'],p[1]['full_file_sha256'])
    def test_original_prefix_is_not_claimed_valid_program(self):
        out={r['ID']:r for r in g.f.read(g.ROOT/g.PACKET/'compiler/oracle-output.json')};self.assertTrue(all(out['ORIGINAL-'+id]['Errors']for id in g.IDS))
    def test_no_independent_human_approval_from_compiler(self):self.assertTrue(all(not r['independent_semantic_approval']for r in self.data['scope']));self.assertEqual(self.data['summary']['independent_approved'],0)
    def test_nine_functions_seventeen_inputs_still_in_denominator(self):self.assertEqual(len(self.data['missing']),9);self.assertEqual(sum(len(r['case_ids'])for r in self.data['missing']),17)
    def test_four_original_files_absent_from_recorded_manifest(self):self.assertEqual(len({r['path']for r in self.data['missing']}),4);self.assertTrue(all(r['recorded_graph_manifest_contains_file']is False for r in self.data['missing']))
    def test_current_profiles_do_not_invent_historical_environment(self):self.assertTrue(all(r['historical_build_environment']is None for r in self.data['missing']))
    def test_linux_and_tag_contexts_are_distinct(self):
        by={r['path']:r['current_MatchFile_profiles']for r in self.data['missing']};self.assertTrue(by['metrics/disk_linux.go']['linux_default']);self.assertFalse(by['metrics/disk_linux.go']['darwin_default']);self.assertTrue(by['build/ci.go']['darwin_none_tag']);self.assertTrue(by['crypto/signify/signify_fuzz.go']['darwin_gofuzz_tag'])
    def test_actual_contract_candidate_counts_unchanged(self):self.assertEqual([r['candidates']for r in self.data['runtime']],[1,1,0,1,1,0,0,0])
    def test_old_budget_failures_preserved_current_probe_separate(self):
        rows=self.data['runtime'][-2:];self.assertTrue(all(r['prior_status']=='budget_exhausted' and r['status']=='partial'for r in rows));self.assertTrue(all('not_replacement' in r['observation_kind']for r in rows))
    def test_no_semantic_precision_from_literal_contract(self):self.assertIsNone(self.data['summary']['current_semantic_quality_scores']);self.assertIn('semantic precision',g.contract()['shadow'])
    def test_no_automatic_rule_pack_upgrade(self):self.assertIn('1.0.0',g.contract()['pack']);self.assertFalse(g.contract()['formal_scoring_allowed'])
    def test_real_names_not_required(self):self.assertFalse(g.contract()['privacy']['real_name_required'])


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change,compressed=False):
        with tempfile.TemporaryDirectory()as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);x=p/name;raw=x.read_bytes();v=json.loads(gzip.decompress(raw)if compressed else raw);before=copy.deepcopy(v);change(v);self.assertNotEqual(v,before);raw=g.f.canonical(v);x.write_bytes(gzip.compress(raw,mtime=0)if compressed else raw);m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p))
    def test_current_packet_indices(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_shadow_promoted_to_import(self):self.mutant('compiler/oracle-output.json',lambda v:v[2]['Calls'][0].update(ImportPath='os/exec'))
    def test_alias_target_lost(self):self.mutant('compiler/oracle-output.json',lambda v:v[3]['Calls'][0].update(ObjectPackage='unknown'))
    def test_helper_counter_context_replaced_by_identity(self):self.mutant('compiler/oracle-output.json',lambda v:v[1]['HelperReturns'].update(sanitize='v'))
    def test_original_invalid_prefix_claimed_complete(self):self.mutant('compiler/oracle-output.json',lambda v:next(r for r in v if r['ID']=='ORIGINAL-RULE-1-07').update(Errors=[]))
    def test_profile_evidence_not_permission_to_drop_inputs(self):self.mutant('index-scope-review.json',lambda v:v.pop())
    def test_historical_platform_invented(self):self.mutant('index-scope-review.json',lambda v:v[0].update(historical_build_environment={'GOOS':'darwin'}))
    def test_current_profile_changed(self):self.mutant('compiler/oracle-output.json',lambda v:next(r for r in v if r['ID']=='metrics/disk_linux.go')['Matches'].update(darwin_default=True))
    def test_runtime_row_filtered(self):self.mutant('runtime/responses.json',lambda v:v.pop())
    def test_runtime_unverified_target_promoted(self):self.mutant('runtime/responses.json',lambda v:v[0]['raw_response']['result']['structuredContent']['analysis']['rules']['candidates'][0].update(target_resolution='verified'))
    def test_original_query_budget_changed(self):self.mutant('runtime/responses.json',lambda v:v[0]['arguments'].update(max_flow_steps=4095))
    def test_budget_primary_failure_erased(self):self.mutant('actual-contract-review.json',lambda v:v[-1].update(prior_status='partial'))
    def test_controlled_graph_mutation(self):self.mutant('runtime/controlled-after.json.gz',lambda v:v.update({'graph.db':'0'*64}),True)
    def test_runtime_binary_mismatch(self):self.mutant('runtime/receipt.json',lambda v:v.update(binary_sha256='0'*64))
    def test_semantic_score_fabricated(self):self.mutant('SUMMARY.json',lambda v:v.update(current_semantic_quality_scores={'precision':1}))
    def test_formal_acceptance_fabricated(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
