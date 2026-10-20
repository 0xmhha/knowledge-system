#!/usr/bin/env python3
"""Guard exact exposure, executable plan coverage and false completion boundaries."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/consumer-contract-plan-v1'
spec=importlib.util.spec_from_file_location('consumer_plan_guard',ROOT/'scripts/check-consumer-contract-plan.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class ConsumerPlanTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.plan=json.loads((ROOT/guard.prepare.FIXTURE).read_text())

    def setUp(self):
        t=tempfile.TemporaryDirectory();self.addCleanup(t.cleanup);self.packet=Path(t.name)/'packet';shutil.copytree(PACKET,self.packet)

    def change(self,name,action):
        p=self.packet/name;d=json.loads(p.read_text());action(d);p.write_text(json.dumps(d,ensure_ascii=False))
        p=self.packet/'manifest.json';d=json.loads(p.read_text())
        d['artifacts']={p.relative_to(self.packet).as_posix():guard.sha(p.read_bytes()) for p in self.packet.rglob('*') if p.is_file() and p.name!='manifest.json'}
        p.write_text(json.dumps(d))

    def reject_input(self,action,message):
        p=copy.deepcopy(self.plan);action(p)
        with self.assertRaisesRegex(ValueError,message):guard.prepare.validate(p)

    def test_current_plan(self):self.assertEqual(guard.check(PACKET),[])
    def test_exact_seven_role_pairs(self):
        self.reject_input(lambda p:p['role_tool_changes'].pop(),'exact seven')
    def test_duplicate_role_tool_does_not_cover_other_pair(self):
        self.reject_input(lambda p:p['role_tool_changes'].__setitem__(1,p['role_tool_changes'][0]),'exact seven')
    def test_wildcard_not_exact_exposure(self):
        self.reject_input(lambda p:p['role_tool_changes'][0].update(alias='mcp__plugin_coding-agent_cks__*'),'exact MCP')
    def test_wrong_wire_namespace(self):
        self.reject_input(lambda p:p['role_tool_changes'][0].update(wire_name='context.value_dependencies'),'exact MCP')
    def test_backend_scope_not_expanded(self):
        self.reject_input(lambda p:p['scope'].update(backend_schema_or_permission_model_change=True),'implementation/execution promotion')
    def test_original_runtime_not_modified(self):
        self.reject_input(lambda p:p['scope']['preserved_runtime_sources'].remove('plugin/scripts/deep_review.py'),'preserved runtime')
    def test_extra_source_changes_not_allowed(self):
        self.reject_input(lambda p:p['scope']['required_consumer_changes'].append('plugin/settings.json'),'minimal source')
    def test_legacy_dispatch_and_return_alignment_required(self):
        self.reject_input(lambda p:p['scope'].update(align_legacy_review_dispatch_and_return_text=False),'role dispatch alignment')
    def test_external_write_not_claimed(self):
        self.reject_input(lambda p:p['scope'].update(current_task_external_writes=True),'implementation/execution promotion')
    def test_plan_not_implementation(self):
        self.reject_input(lambda p:p['scope'].update(current_task_implementation_performed=True),'implementation/execution promotion')
    def test_plan_not_new_execution(self):
        self.reject_input(lambda p:p['scope'].update(current_task_runtime_execution_performed=True),'implementation/execution promotion')
    def test_old_harness_not_new_role_proof(self):
        self.reject_input(lambda p:p['scope'].update(existing_harness_proves_new_role_paths=True),'implementation/execution promotion')
    def test_old_host_version_not_new_pin(self):
        self.reject_input(lambda p:p['baseline'].update(historical_host_version_is_execution_pin=True),'new execution pins')
    def test_no_missing_requirement(self):self.reject_input(lambda p:p['requirements'].pop(),'twelve requirements')
    def test_no_duplicate_requirement(self):self.reject_input(lambda p:p['requirements'].append(p['requirements'][0]),'duplicate ID')
    def test_requirement_not_implemented(self):
        self.reject_input(lambda p:p['requirements'][0].update(implementation_status='completed'),'requirement implementation')
    def test_unlinked_requirement(self):self.reject_input(lambda p:p['requirements'][0].update(test_ids=[]),'requirement test coverage')
    def test_missing_planned_case(self):self.reject_input(lambda p:p['validation_cases'].pop(),'twenty-two')
    def test_test_status_not_run(self):self.reject_input(lambda p:p['validation_cases'][0].update(status='pass'),'planned test execution')
    def test_no_invented_receipt(self):self.reject_input(lambda p:p['validation_cases'][0].update(receipt='passed'),'planned test execution')
    def test_case_has_expected_outcome(self):self.reject_input(lambda p:p['validation_cases'][0].update(expected=''),'procedure/oracle')
    def test_step_not_complete(self):self.reject_input(lambda p:p['implementation_steps'][0].update(status='completed'),'step completion')
    def test_step_not_fake_commit(self):self.reject_input(lambda p:p['implementation_steps'][0].update(implementation_commit='a'*40),'step completion')
    def test_dependency_cycle_rejected(self):self.reject_input(lambda p:p['implementation_steps'][0].update(depends_on=['P08']),'dependency cycle')
    def test_step_cannot_omit_validation(self):self.reject_input(lambda p:p['implementation_steps'][0].update(test_ids=[]),'step tests')
    def test_every_case_in_step_plan(self):
        self.reject_input(lambda p:[r.update(test_ids=[x for x in r['test_ids'] if x!='T22']) for r in p['implementation_steps']],'step tests|uncovered')
    def test_no_paid_provider(self):self.reject_input(lambda p:p['resource_plan'].update(real_model_calls_max=1),'zero side effect')
    def test_no_real_nodes(self):self.reject_input(lambda p:p['resource_plan'].update(real_chain_nodes_max=1),'zero side effect')
    def test_no_push(self):self.reject_input(lambda p:p['resource_plan'].update(push_or_pr_max=1),'zero side effect')
    def test_responder_loopback_only(self):self.reject_input(lambda p:p['resource_plan'].update(responder_bind='0.0.0.0'),'owned resource')
    def test_termination_budget_not_removed(self):self.reject_input(lambda p:p['resource_plan'].update(termination_grace_seconds=None),'owned resource')
    def test_global_credentials_not_required(self):self.reject_input(lambda p:p['resource_plan'].update(global_settings_or_real_credentials_required=True),'owned resource')
    def test_isolation_not_proven_full_sandbox(self):self.reject_input(lambda p:p['resource_plan'].update(environment_isolation_is_proven_security_sandbox=True),'owned resource')
    def test_role_not_direct_client(self):self.reject_input(lambda p:p['receipt_contract'].update(role_receipt_cannot_be_replaced_by_direct_client=False),'actual role')
    def test_hook_receipt_required(self):self.reject_input(lambda p:p['receipt_contract']['required_fields'].remove('hook_event_and_cli_exit_receipts'),'field inventory')
    def test_blocked_not_pass(self):self.reject_input(lambda p:p['completion'].update(blocked_or_inconclusive_is_pass=True),'completion promotion')
    def test_rendering_missing_not_pass(self):self.reject_input(lambda p:p['completion'].update(rendering_not_available_is_pass=True),'completion promotion')
    def test_formal_state_not_advanced(self):self.reject_input(lambda p:p['completion'].update(formal_completed=1),'formal acceptance')
    def test_reviewer_not_invented(self):self.reject_input(lambda p:p['completion'].update(reviewer_assignment='AI reviewer'),'formal acceptance')
    def test_unknown_role_permissions_not_invented(self):
        self.change('source-audit.json',lambda d:d['role_tool_matrix'][0].update(effective_host_permissions='allowed'))
        self.assertIn('consumer plan accounting: source-audit.json',guard.check(self.packet,False))
    def test_no_runtime_denial_from_missing_alias(self):
        self.change('source-audit.json',lambda d:d['role_tool_matrix'][0].update(actual_runtime_denial_proven=True))
        self.assertIn('consumer plan accounting: source-audit.json',guard.check(self.packet,False))
    def test_plan_outcomes_not_pass(self):
        self.change('isolated-validation-plan.json',lambda d:d['validation_cases'][0].update(status='pass'))
        self.assertIn('consumer plan accounting: isolated-validation-plan.json',guard.check(self.packet,False))
    def test_spec_does_not_claim_implementation(self):
        self.change('implementation-spec.json',lambda d:d['scope'].update(current_task_implementation_performed=True))
        self.assertIn('consumer plan accounting: implementation-spec.json',guard.check(self.packet,False))
    def test_runtime_counter_not_inflated(self):
        self.change('source-audit.json',lambda d:d.update(new_role_sessions=7))
        self.assertIn('consumer plan accounting: source-audit.json',guard.check(self.packet,False))
    def test_formal_snapshot_tamper(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True))
        self.assertIn('formal state changed',guard.check(self.packet,False))
    def test_rendered_spec_cannot_drift(self):
        p=self.packet/'CONSUMER-CONTRACT-SPEC.txt';p.write_text(p.read_text()+'Implementation complete\n')
        self.assertIn('consumer plan accounting: CONSUMER-CONTRACT-SPEC.txt',guard.check(self.packet,False))
    def test_source_inventory_counts(self):
        a=guard.prepare.generate()['source-audit.json'];self.assertEqual((a['role_tool_entries'],a['unique_tools']),(7,5))
        self.assertEqual(a['planned_tool_counts'],{'analyzer':54,'planner':19});self.assertEqual(a['validation_cases_executed'],0)


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
