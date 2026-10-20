"""Fresh native/MCP evidence cannot erase primary failures or approve quality."""
import copy,gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi3_reason_fix as g


class ActualRegressionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.data=g.derive();cls.runtime=g.runtime_data(g.ROOT,g.ROOT/g.PACKET)
    def test_fresh_full_native_checks(self):self.assertEqual(self.data['summary']['native_checks'],g.NATIVE_LABELS);self.assertEqual(g.native_errors(g.ROOT,g.ROOT/g.PACKET),[])
    def test_new_native_tests_executed_in_general_and_race(self):
        for label in ('test','race'):
            rows=g.events(gzip.decompress((g.ROOT/g.PACKET/'native'/(label+'.stdout.gz')).read_bytes()));passed={x.get('Test') for x in rows if x['Action']=='pass'};self.assertIn('TestFailureReasonsDoNotConflateSeedBackendAndBudget',passed);self.assertIn('TestUnresolvedSeedSharedSignalsStayPartialBeforeOrAfterSuccess',passed)
    def test_all20_original_inputs_preserved(self):self.assertEqual(len(self.data['cases']),20);self.assertEqual(sum(self.runtime['statuses'].values()),20)
    def test_all20_candidate_source_identities_order_unchanged(self):self.assertEqual(self.data['summary']['original_candidate_identity_order_equal_cases'],20)
    def test_no_quality_score_from_changed_error_reason(self):self.assertIsNone(self.data['summary']['primary_quality_scores']);self.assertFalse(self.data['summary']['execution_allowed'])
    def test_initial_timeout_and20_unreached_not_hidden(self):self.assertEqual(self.data['summary']['initial_unreached_primary'],20);self.assertEqual(self.data['summary']['initial_tools_attempted'],0);self.assertEqual(self.data['summary']['initialization_attempts'],2)
    def test_concurrent_errors_not_filtered(self):self.assertEqual(self.data['summary']['concurrent_primary_statuses'],{'tool_error':11,'partial':9})
    def test_controlled_request_denominator_seven(self):self.assertEqual(len(self.runtime['controls']),7);self.assertEqual(self.data['summary']['controlled_requests'],7)
    def test_missing_seeds_both_orders_error_and_null(self):
        controls=g.f.read(g.ROOT/g.PACKET/'runtime/controls.json')
        for row in controls[:3]:
            v,_=g.unpack(row);self.assertTrue(v['partial'])
            for c in v['candidates'][:3]:
                for k in ('impact_category','concurrency'):self.assertEqual(c['signals'][k]['reason'],'seed_unresolved');self.assertIsNone(c['signals'][k]['value'])
    def test_known_only_preserves_observed(self):
        row=g.f.read(g.ROOT/g.PACKET/'runtime/controls.json')[3];v,_=g.unpack(row);self.assertEqual(v['candidates'][0]['signals']['impact_category']['status'],'observed')
    def test_invalid_scope_refusals_remain_tool_errors(self):
        for row in g.f.read(g.ROOT/g.PACKET/'runtime/controls.json')[4:]:self.assertEqual(g.unpack(row)[1],'tool_error')
    def test_model_digest_actual_before_after(self):
        for phase in ('before','after'):self.assertEqual(g.f.read(g.ROOT/g.PACKET/'runtime'/('model-'+phase+'.json'))['model']['digest'],'7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab')
    def test_no_fake_embedding_count(self):self.assertIsNone(self.data['summary']['embedding_inference_call_count']);self.assertEqual(self.data['summary']['paid_or_external_LLM_calls'],0)
    def test_primary_errors_retain_unknown_candidate_denominator(self):
        for r in self.data['cases']:
            if r['status']in('tool_error','protocol_error','invalid_response'):self.assertIsNone(r['candidates']);self.assertIsNone(r['failure_reasons']);self.assertIsNone(r['stop_reasons'])
    def test_tool_error_does_not_turn_into_empty_observation(self):self.assertEqual(g.unpack({'raw_response':{'result':{'isError':True}}}),(None,'tool_error'))
    def test_protocol_error_distinct_from_tool_error(self):self.assertEqual(g.unpack({'raw_response':{'error':{'code':-1}}}),(None,'protocol_error'))
    def test_aggregate_not_exclusive_past_fault_proof(self):self.assertIn('not a complete per-call failure timeline',g.contract()['aggregate'])
    def test_real_names_not_required(self):self.assertFalse(g.contract()['privacy']['real_name_required'])


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change,compressed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);x=p/name;raw=x.read_bytes();v=json.loads(gzip.decompress(raw) if compressed else raw);old=copy.deepcopy(v);change(v);self.assertNotEqual(v,old);raw=g.f.canonical(v);x.write_bytes(gzip.compress(raw,mtime=0) if compressed else raw);m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p))
    def test_current_packet_and_live_indices(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_old_native_tests_reused_as_current(self):self.mutant('native/receipts.json',lambda v:v[1].update(source_head='38dd5c5449b1e57759e2532090dce101b3ad9124'))
    def test_native_test_scope_narrowed(self):self.mutant('native/receipts.json',lambda v:v[1]['argv'].__setitem__(-1,'./internal/system/suspects'))
    def test_race_missing(self):self.mutant('native/receipts.json',lambda v:v.pop(2))
    def test_wrong_linter_version_rejected(self):self.mutant('native/installed-tools.json',lambda v:v.update(lint_version='golangci-lint has version unknown'))
    def test_native_failure_not_ignored(self):self.mutant('native/receipts.json',lambda v:v[1].update(exit_code=1))
    def test_bound_source_removed(self):self.mutant('source-binding.json',lambda v:v['source_sha256'].pop('internal/system/suspects/compose.go'))
    def test_wrong_native_binary(self):self.mutant('native/binary.json',lambda v:v.update(sha256='0'*64))
    def test_controlled_seed_failure_misclassified(self):self.mutant('runtime/controls.json',lambda v:v[0]['raw_response']['result']['structuredContent']['candidates'][0]['signals']['impact_category'].update(reason='backend_error'))
    def test_known_success_promoted_from_unknown(self):self.mutant('runtime/controls.json',lambda v:v[3]['raw_response']['result']['structuredContent']['candidates'][0]['signals']['impact_category'].update(status='error'))
    def test_invalid_foreign_refusal_faked_success(self):self.mutant('runtime/controls.json',lambda v:v[4]['raw_response']['result'].update(isError=False))
    def test_primary_source_identity_changed(self):self.mutant('runtime/primary-results.json.gz',lambda v:v[0]['raw_response']['result']['structuredContent']['candidates'][0]['citation'].update(start_line=1),True)
    def test_planned20row_dropped(self):self.mutant('runtime/primary-results.json.gz',lambda v:v.pop(),True)
    def test_concurrent_errors_erased(self):self.mutant('concurrent-runtime/primary-results.json.gz',lambda v:v.pop(),True)
    def test_initial_unreached_erased(self):self.mutant('initial-startup/attempt.json',lambda v:v.update(unreached_primary=0))
    def test_embedding_calls_fabricated_zero(self):self.mutant('runtime/receipt.json',lambda v:v.update(embedding_inference_call_count=0))
    def test_primary_retry_hidden(self):self.mutant('runtime/receipt.json',lambda v:v.update(primary_retries=1))
    def test_frozen_gold_fabricated(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))


if __name__=='__main__':unittest.main()
