#!/usr/bin/env python3
"""Do not promote partial actual-role results, skipped tools or failed controls."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/consumer-role-readiness-v1'
s=importlib.util.spec_from_file_location('guard',ROOT/'scripts/check-consumer-role-readiness.py');guard=importlib.util.module_from_spec(s);s.loader.exec_module(guard)


class RoleReadinessTests(unittest.TestCase):
    def setUp(self):
        tmp=tempfile.TemporaryDirectory();self.addCleanup(tmp.cleanup);self.packet=Path(tmp.name)/'packet';shutil.copytree(PACKET,self.packet)
    def rehash(self):
        p=self.packet/'manifest.json';d=json.loads(p.read_text());d['artifacts']={q.relative_to(self.packet).as_posix():guard.sha(q)for q in self.packet.rglob('*')if q.is_file()and q!=p};p.write_text(json.dumps(d))
    def change(self,name,action):
        p=self.packet/name;d=json.loads(p.read_text());action(d);p.write_text(json.dumps(d));self.rehash()
    def rejected(self):self.assertTrue(guard.check(self.packet,False))
    def accounting(self,action):self.change('role-readiness.json',action);self.assertIn('role accounting: role-readiness.json',guard.check(self.packet,False))
    def gateway(self,case,action):self.change('runs/'+case+'/gateway-requests.json',action);self.rejected()
    def life(self,case,action):self.change('runs/'+case+'/exit.json',action);self.rejected()
    def test_current_packet(self):self.assertEqual(guard.check(PACKET),[])
    def test_P06_not_complete(self):self.accounting(lambda d:d.update(P06_complete=True))
    def test_five_not_seven_successful_pairs(self):self.accounting(lambda d:d.update(role_tool_pairs_succeeded=7))
    def test_missing_tools_not_erased(self):self.accounting(lambda d:d.update(failed_pairs=[]))
    def test_T05_failure_not_pass(self):self.accounting(lambda d:d['planned_case_progress'][4].update(full_case_status='pass'))
    def test_T19_failure_not_pass(self):self.accounting(lambda d:d['planned_case_progress'][18].update(full_case_status='pass'))
    def test_T20_failure_not_pass(self):self.accounting(lambda d:d['planned_case_progress'][19].update(full_case_status='pass'))
    def test_renderer_not_pass(self):self.accounting(lambda d:d['planned_case_progress'][14].update(full_case_status='pass'))
    def test_no_model_quality(self):self.accounting(lambda d:d.update(actual_role_reasoning_quality='passed'))
    def test_no_independent_acceptance(self):self.accounting(lambda d:d.update(formal_completed=1,independent_acceptance='accepted'))
    def test_candidate_not_adopted(self):self.accounting(lambda d:d.update(candidate_adopted=True))
    def test_no_hidden_startup_failure(self):self.life('P06-analyzer-02',lambda d:d.update(host_exit=0))
    def test_host_must_reap(self):self.life('P06-planner-delivery',lambda d:d.update(host_reaped=False))
    def test_forced_exit_not_pass(self):self.life('P06-planner-delivery',lambda d:d.update(forced_host_termination=True))
    def test_responder_cleanup(self):self.life('P06-planner-delivery',lambda d:d.update(responder_closed=False))
    def test_wrong_actual_role(self):
        self.change('runs/P06-planner-delivery/invocation.json',lambda d:d.update(role='analyzer'));self.rejected()
    def test_nonlocal_provider(self):
        self.change('runs/P06-planner-delivery/invocation.json',lambda d:d['env'].update(ANTHROPIC_BASE_URL='https://api.example.invalid'));self.rejected()
    def test_host_binary_pin(self):
        self.change('runs/P06-planner-delivery/invocation.json',lambda d:d.update(host_sha256='0'*64));self.rejected()
    def test_helper_source_pin(self):
        p=self.packet/'helper-versions/P06-analyzer-03.py.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash();self.rejected()
    def test_role_system_body_not_replaced(self):
        self.gateway('P06-planner-delivery',lambda rows:rows[0]['body'].update(system='Planner-like text is not the pinned role.'))
    def test_effective_tool_inventory_not_fabricated(self):
        self.gateway('P06-analyzer-03',lambda rows:rows[0]['body']['tools'].pop())
    def test_actual_two_tool_failures_preserved(self):
        def change(rows):
            for r in rows:
                for m in r['body'].get('messages',[]):
                    if m['role']=='user':
                        for x in m.get('content',[]):
                            if isinstance(x,dict)and x.get('tool_use_id')=='fixture_tool_0':x.update(is_error=False,content='success')
        self.gateway('P06-analyzer-03',change)
    def test_host_schema_cause_not_removed(self):
        p=self.packet/'runs/P06-analyzer-03/host.debug.txt';p.write_text(p.read_text().replace('top-level oneOf','accepted schema'));self.rehash();self.rejected()
    def test_explicit_permission_deny_required(self):
        self.change('runs/P06-planner-denied/settings.json',lambda d:d['permissions'].update(deny=[]));self.rejected()
    def test_real_API_result_bound_to_stdio(self):
        def change(rows):
            for r in rows:
                for m in r['body'].get('messages',[]):
                    if m['role']=='user':
                        for x in m.get('content',[]):
                            if isinstance(x,dict)and x.get('tool_use_id')=='fixture_tool_0':x.update(content='{}')
        self.gateway('P06-analyzer-handoff',change)
    def test_no_failure_state_change(self):
        self.change('runs/P06-analyzer-unavailable/workspace-after.json',lambda d:d['state.json']['json'].update(current_state='PLANNING'));self.rejected()
    def test_analysis_hash_bound(self):
        self.change('runs/P06-analyzer-handoff/workspace-after/state.json',lambda d:d['states']['ANALYSIS'].update(deep_review_sha256='0'*64));self.rejected()
    def test_planning_hash_bound(self):
        self.change('runs/P06-planner-reentry/workspace-after/state.json',lambda d:d['states']['PLANNING'].update(review_report_sha256='0'*64));self.rejected()
    def test_snapshot_atomicity_not_invented(self):
        self.change('runs/P06-analyzer-handoff/workspace-after/deep-review.json',lambda d:d['structural_review'].update(snapshot_association='atomically_verified'));self.rejected()
    def test_scripted_findings_not_independently_accepted(self):
        self.change('runs/P06-analyzer-handoff/workspace-after/deep-review.json',lambda d:d.update(independent_acceptance='accepted'));self.rejected()
    def test_shell_failure_not_success(self):
        self.change('runs/P06-planner-boundary/workspace-after/shell-tamper-receipt.json',lambda d:d.update(exit_code=0,delivery_allowed=True));self.rejected()
    def test_diagrams_not_modified(self):
        p=self.packet/'runs/P06-planner-reentry/workspace-after/modules.mmd';p.write_text('flowchart LR\n A-->B\n');self.rehash();self.rejected()
    def test_request_denominator_not_hidden(self):self.accounting(lambda d:d.update(API_tool_attempts=7,stdio_tool_requests=7,host_sessions=8))
    def test_candidate_change_not_hidden(self):
        self.change('execution.json',lambda d:d['candidate_postrun'].update(porcelain=' M plugin/contracts/cks-tools.schema.json'));self.rejected()
    def test_original_change_not_hidden(self):
        self.change('execution.json',lambda d:d['original_postrun'].update(commit='0'*40));self.rejected()
    def test_maintained_schema_change_not_hidden(self):
        self.change('execution.json',lambda d:d.update(maintained_source_changes=['internal/system/mcp/annotations.go']));self.rejected()
    def test_formal_unchanged(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True));self.rejected()
    def test_extra_artifact_rejected_after_rehash(self):
        (self.packet/'unrecorded.txt').write_text('extra');self.rehash();self.rejected()
    def test_scope_cannot_claim_compatibility_implementation(self):
        p=self.packet/'COMPATIBILITY-FOLLOWUP.txt';p.write_text(p.read_text().replace('미구현','완료'));self.rehash();self.rejected()


if __name__=='__main__':unittest.main()
