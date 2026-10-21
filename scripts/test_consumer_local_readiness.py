#!/usr/bin/env python3
"""Keep local source/verifier proof separate from real role and full-plan gates."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/consumer-local-readiness-v1'
spec=importlib.util.spec_from_file_location('local_guard',ROOT/'scripts/check-consumer-local-readiness.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class LocalReadinessTests(unittest.TestCase):
    def setUp(self):
        t=tempfile.TemporaryDirectory();self.addCleanup(t.cleanup);self.packet=Path(t.name)/'packet';shutil.copytree(PACKET,self.packet)

    def rehash(self):
        p=self.packet/'manifest.json';d=json.loads(p.read_text());d['artifacts']={p.relative_to(self.packet).as_posix():guard.sha(p.read_bytes())for p in self.packet.rglob('*')if p.is_file()and p.name!='manifest.json'};p.write_text(json.dumps(d))

    def change(self,name,action):
        p=self.packet/name;d=json.loads(p.read_text());action(d);p.write_text(json.dumps(d));self.rehash()

    def reject_accounting(self,action):
        self.change('local-readiness.json',action);self.assertIn('local readiness accounting: local-readiness.json',guard.check(self.packet,False))

    def reject_receipt(self,action,message):
        self.change('execution.json',action);self.assertTrue(any(message in x for x in guard.check(self.packet,False)))

    def test_current_candidate_packet(self):self.assertEqual(guard.check(PACKET),[])
    def test_not_adopted(self):self.reject_accounting(lambda d:d.update(candidate_adopted=True))
    def test_static_alias_not_actual_permission(self):self.reject_accounting(lambda d:d['role_tool_exposure'][0].update(effective_host_permissions='allowed'))
    def test_actual_role_receipt_not_invented(self):self.reject_accounting(lambda d:d['role_tool_exposure'][0].update(actual_role_receipt='passed'))
    def test_P02_full_validation_pending(self):self.reject_accounting(lambda d:d['steps'][1].update(full_cross_layer_validation='complete'))
    def test_stdio_not_executed(self):self.reject_accounting(lambda d:d.update(real_tools_list_executed=True))
    def test_renderer_not_executed(self):self.reject_accounting(lambda d:d.update(diagrams_rendered=True))
    def test_actual_role_case_not_pass(self):self.reject_accounting(lambda d:d['planned_case_progress'][4].update(full_case_status='pass'))
    def test_six_cases_not_twenty_two(self):self.reject_accounting(lambda d:d.update(full_cases_pending=0))
    def test_no_independent_quality(self):self.reject_accounting(lambda d:d.update(current_project_quality_evaluated=True))
    def test_no_formal_acceptance(self):self.reject_accounting(lambda d:d.update(formal_completed=1,independent_acceptance='accepted'))
    def test_no_extra_consumer_scope(self):self.reject_receipt(lambda d:d['changed_paths'].append('plugin/scripts/deep_review.py'),'candidate change scope')
    def test_original_source_not_modified(self):self.reject_receipt(lambda d:d['original_adopted_after'].update(porcelain=' M plugin/agents/analyzer.md'),'original adopted source changed')
    def test_candidate_clean_required(self):self.reject_receipt(lambda d:d.update(candidate_clean_after=False),'baseline/clean')
    def test_signature_failure_not_pass(self):self.reject_receipt(lambda d:d.update(signature_verification_exit=1),'signature/temporary cleanup')
    def test_temp_cleanup_required(self):self.reject_receipt(lambda d:d.update(owned_test_temp_entries_after=['leftover']),'signature/temporary cleanup')
    def test_test_count_not_inflated(self):self.reject_receipt(lambda d:d.update(tests_passed=46),'source/count binding')
    def test_test_source_must_be_pinned(self):self.reject_receipt(lambda d:d['source_sha256'].update({'plugin/scripts/tests/test_review_role_contract.py':'0'*64}),'source/count binding')
    def test_local_not_host(self):self.reject_receipt(lambda d:d.update(actual_role_sessions=1),'promoted to runtime')
    def test_local_not_mcp(self):self.reject_receipt(lambda d:d.update(new_mcp_sessions=1),'promoted to runtime')
    def test_local_not_model(self):self.reject_receipt(lambda d:d.update(new_model_calls=1),'promoted to runtime')
    def test_candidate_not_silently_adopted(self):self.reject_receipt(lambda d:d.update(candidate_adopted=True),'adoption promotion')
    def test_host_file_hash_not_host_execution(self):self.reject_receipt(lambda d:d['host_binary_pin'].update(new_host_execution_performed=True),'host file pin')
    def test_hidden_test_skip_rejected(self):
        p=self.packet/'logs/test_review_role_contract.py.txt';p.write_text(p.read_text().replace('OK','OK (skipped=1)'));self.rehash()
        self.assertTrue(any('local regression log' in x for x in guard.check(self.packet,False)))
    def test_build_warning_not_hidden(self):
        (self.packet/'build-logs/build-cks.stderr.txt').write_text('cache warning');self.rehash()
        self.assertTrue(any('binary build stderr' in x for x in guard.check(self.packet,False)))
    def test_go_source_delta_not_hidden(self):
        self.change('build-receipts.json',lambda d:d[0].update(go_source_changes=['cmd/cks/main.go']))
        self.assertTrue(any('binary/source pin' in x for x in guard.check(self.packet,False)))
    def test_candidate_source_tamper(self):
        p=self.packet/'candidate-source/sources/plugin/agents/planner.md.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in x for x in guard.check(self.packet,False)))
    def test_binary_objects_preserved(self):
        (self.packet/'.gitattributes').write_text('** text\n');self.rehash();self.assertIn('binary object preservation',guard.check(self.packet,False))
    def test_formal_snapshot_not_advanced(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True));self.assertIn('formal state changed',guard.check(self.packet,False))
    def test_correct_local_inventory_and_pending_states(self):
        d=guard.prepare.generate(PACKET)['local-readiness.json'];self.assertEqual(d['local_tests_passed'],45)
        self.assertEqual([r['id'] for r in d['planned_case_progress']if r['full_case_status']=='pass'],guard.prepare.FULL_LOCAL)
        self.assertEqual(d['full_cases_pending'],16)
        self.assertEqual([r['after'] for r in d['role_tool_exposure']],[54,19])


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
