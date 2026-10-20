#!/usr/bin/env python3
"""Reject WI-SP draft, provenance, access and deployed-host acceptance promotion."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/policy-security-gap-v1'
spec=importlib.util.spec_from_file_location('policy_security_gap_guard',ROOT/'scripts/check-policy-security-gap.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class PolicySecurityGapTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.inputs=guard.prepare.inputs(PACKET)

    def setUp(self):
        tmp=tempfile.TemporaryDirectory();self.addCleanup(tmp.cleanup)
        self.packet=Path(tmp.name)/'packet';shutil.copytree(PACKET,self.packet)

    def rehash(self):
        p=self.packet/'manifest.json';d=json.loads(p.read_text())
        d['artifacts']={p.relative_to(self.packet).as_posix():guard.sha(p.read_bytes()) for p in self.packet.rglob('*') if p.is_file() and p.name!='manifest.json'}
        p.write_text(json.dumps(d))

    def change(self,name,action):
        p=self.packet/name;d=json.loads(p.read_text());action(d)
        p.write_text(json.dumps(d,ensure_ascii=False));self.rehash()

    def reject(self,message):self.assertIn(message,guard.check(self.packet,check_references=False))

    def symbols(self,action):
        self.change('private-symbol-gaps.json',action);self.reject('policy security gap accounting: private-symbol-gaps.json')

    def case(self,action):self.symbols(lambda d:action(d['cases'][0]))

    def boundary(self,action):
        self.change('boundary-and-consumer-review.json',action);self.reject('policy security gap accounting: boundary-and-consumer-review.json')

    def history(self,action):
        self.change('historical-evidence-scope.json',action);self.reject('policy security gap accounting: historical-evidence-scope.json')

    def test_real_packet(self):self.assertEqual(guard.check(PACKET),[])

    def test_twelve_symbols_not_independent_families(self):self.case(lambda r:r.update(independent_symbol_family='approved'))

    def test_same_file_pair_not_independent(self):
        self.change('pairwise-review.json',lambda d:d[0].update(semantic_independence='approved'));self.reject('policy security gap accounting: pairwise-review.json')

    def test_source_literal_filter_not_gold(self):self.case(lambda r:r.update(gold=True,approved=True))

    def test_missing_gold_not_approved_empty(self):self.case(lambda r:r.update(independent_annotation_gold=[]))

    def test_ai_security_proposal_not_ingested_gold(self):
        self.symbols(lambda d:next(r for r in d['cases'] if r['group_proposal']=='security_proposal').update(independent_annotation_gold=['security:approved']))

    def test_policy_relation_not_current_symbol_response(self):self.case(lambda r:r.update(current_symbol_mcp_receipt='passed'))

    def test_go_commit_not_yaml_or_reviewer_proof(self):self.case(lambda r:r.update(source_yaml_and_review_receipt=r['source']))

    def test_name_hint_not_canonical_resolution(self):self.case(lambda r:r.update(canonical_symbol_resolution_receipt='resolved'))

    def test_untagged_not_safe(self):
        self.symbols(lambda d:next(r for r in d['cases'] if r['group_proposal']=='no_direct_overlay_reference').update(safety_verdict='safe'))

    def test_saved_policy_absence_not_security_absence(self):
        self.change('gap-summary.json',lambda d:d.update(policy_only_saved_edges_do_not_establish_security_absence=False));self.reject('policy security gap accounting: gap-summary.json')

    def test_saved_policy_relation_omission_rejected(self):self.case(lambda r:r.update(saved_source_regeneration_policy_edges=[]))

    def test_legacy_provenance_unknown_not_hidden(self):
        self.boundary(lambda d:next(r for r in d['contracts'] if r['id']=='SP-05').update(gap='metadata SHA is YAML row provenance'))

    def test_acl_not_reviewer_authentication(self):self.boundary(lambda d:d.update(peer_acl_is_reviewer_authentication=True))

    def test_acl_not_per_project_authorization(self):self.boundary(lambda d:d.update(peer_acl_is_per_symbol_or_project_authorization=True))

    def test_metadata_hash_not_authenticated_yaml(self):self.boundary(lambda d:d.update(metadata_hash_authenticates_yaml_or_reviewer=True))

    def test_deployed_listener_receipt_not_invented(self):self.boundary(lambda d:d.update(deployed_listener_and_proxy_receipt='passed'))

    def test_refusal_score_not_unit_test_percentage(self):self.boundary(lambda d:d.update(expected_refusal_fraction=1.0))

    def test_consumer_schema_not_explicit_tool_exposure(self):
        self.boundary(lambda d:d['consumer_exposure_gap'].update(explicit_analyzer_tool_present=True))

    def test_missing_alias_not_runtime_denial_proof(self):
        self.boundary(lambda d:d['consumer_exposure_gap'].update(runtime_denial_proven=True))

    def test_actual_installed_role_receipt_not_invented(self):
        self.boundary(lambda d:d['consumer_exposure_gap'].update(installed_role_receipt='passed'))

    def test_fixture_stdio_success_preserved(self):self.history(lambda d:d['fixture_summary'].update(cks_requests=0))

    def test_historical_httptest_not_deployed_listener(self):self.history(lambda d:d.update(fixture_is_deployed_http_listener=True))

    def test_regeneration_fix_not_regressed_in_report(self):
        self.history(lambda d:d['source_regeneration_status'].update(regeneration_fix_complete=False))

    def test_regeneration_not_ontology_quality_acceptance(self):
        self.history(lambda d:d['source_regeneration_summary'].update(ontology_effect_evaluated=True))

    def test_original_yaml_hash_not_invented(self):
        self.history(lambda d:d['fixture_annotations'][0]['provenance'].update(source_sha256={'status':'verified','value':'a'*64}))

    def test_reviewer_authentication_not_invented(self):
        self.history(lambda d:d['fixture_annotations'][0]['provenance'].update(identity_trust='authenticated'))

    def test_no_annotations_not_same_as_not_found(self):self.history(lambda d:d.update(empty_status='not_found'))

    def test_native_tests_not_twelve_symbol_acceptances(self):self.history(lambda d:d.update(current_twelve_independent_symbol_acceptances=12))

    def test_native_summary_not_rewritten(self):
        self.change('native-check-summary.json',lambda d:d.update(network_listener_started=True));self.reject('policy security gap accounting: native-check-summary.json')

    def test_native_race_skip_rejected(self):
        p=self.packet/'native/race.jsonl';rows=[json.loads(s) for s in p.read_text().splitlines() if s]
        next(r for r in rows if r['Action']=='pass' and r.get('Test')=='TestPolicySecurityHTTPACLRejectsBeforeToolAndIgnoresForwardedIP')['Action']='skip'
        p.write_text(''.join(json.dumps(r)+'\n' for r in rows));self.rehash()
        self.assertTrue(any('native pass/skip/failure inventory' in e for e in guard.check(self.packet,check_references=False)))

    def test_native_hidden_stderr_rejected(self):
        (self.packet/'native/normal.stderr.txt').write_text('hidden failure\n');self.rehash()
        self.assertTrue(any('native raw/summary completion mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_formal_execution_promotion(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True));self.reject('formal state changed')

    def test_public_expected_answer_injection(self):
        self.change('public-inputs.json',lambda d:d['cases'][0].update(expected_annotation_ids=['answer']));self.reject('policy security gap accounting: public-inputs.json')

    def test_project_source_tamper(self):
        p=self.packet/'source/project/sources/systemcontracts/stateutil.go.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_consumer_source_tamper(self):
        p=self.packet/'source/consumer/sources/plugin/agents/analyzer.md.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_raw_object_normalization_rejected(self):
        (self.packet/'.gitattributes').write_text('source/** text\n');self.rehash();self.reject('Git object preservation attributes')

    def test_input_approval_promotion_rejected(self):
        d=copy.deepcopy(self.inputs);d['private']['cases'][0]['gold']=True
        with self.assertRaisesRegex(ValueError,'gold promotion'):guard.prepare.generate(d)

    def test_duplicate_symbol_rejected(self):
        d=copy.deepcopy(self.inputs);d['public']['cases'].append(d['public']['cases'][0])
        with self.assertRaisesRegex(ValueError,'duplicate'):guard.prepare.generate(d)

    def test_current_doc_guard_failure_not_hidden(self):
        self.change('history-context-review.json',lambda d:d.update(current_context_guard_passed=True))
        self.reject('policy security gap accounting: history-context-review.json')

    def test_recorded_doc_context_not_current_document_reversion(self):
        self.change('history-context-review.json',lambda d:d.update(workspace_docs_or_prior_packet_modified=True))
        self.reject('policy security gap accounting: history-context-review.json')

    def test_inventory_and_native_scope(self):
        d=guard.prepare.generate(self.inputs);c=d['gap-summary.json']['counts']
        self.assertEqual((c['source_symbol_proposals'],c['same_file_pairs'],c['saved_selected_policy_edges']),(12,11,5))
        self.assertEqual((c['native_top_level_tests_per_mode'],c['native_pass_records_including_subtests_per_mode'],c['native_modes']),(12,24,2))
        self.assertEqual(c['historical_fixture_cks_requests'],18);self.assertIsNone(d['gap-summary.json']['annotation_match_fraction'])
        self.assertTrue(all(v=='not_satisfied' for v in d['private-symbol-gaps.json']['obligations'].values()))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
