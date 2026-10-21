"""Reject invented manager completion or missing actual MCP/Claude evidence."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

guard=module('integrated_updates_guard','check-manager-updates-MCP-evidence.py')
base=module('integrated_updates_helpers','test_maintenance_runtime_evidence.py');base.guard=guard


class UpdatesEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash
    def rejects(self,name,mutate):
        if not(self.folder/name).exists()and(self.folder/(name+'.gz')).exists():name+='.gz'
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
    def manifest(self,key,value):
        p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=value;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
    def test_current_packet(self):self.assertEqual(guard.check(self.folder),[])
    def test_updater_not_final_periodic_feature(self):self.manifest('full_KM04_complete',True)
    def test_not_independent_quality_acceptance(self):self.manifest('independent_acceptance',True)
    def test_CLI_not_real_model_calls(self):self.rejects('Claude/summary.json',lambda v:v.update(real_model_calls=1))
    def test_fake_local_API_not_remote(self):self.rejects('Claude/updates-full/invocation.json',lambda v:v['env'].update(ANTHROPIC_BASE_URL='https://example.invalid'))
    def test_actual_CLI_tool_result_not_omitted(self):self.rejects('Claude/updates-full/actual-tool-results.json',lambda rows:rows.pop())
    def test_actual_manager_EOF_error_not_hidden(self):self.rejects('Claude/updates-full/observation.json',lambda v:v.update(manager_exit_code=1))
    def test_actual_agent_autonomy_not_invented(self):self.rejects('Claude/updates-full/observation.json',lambda v:v.update(agent_autonomy_or_model_quality_verified=True))
    def test_CLI_restart_does_not_admit_new_source_job(self):self.rejects('Claude/updates-restart/workflow.json',lambda v:v['actual_noop_status'].update(attempts_recorded=2))
    def test_raw_manager_kill_not_normal_exit(self):self.rejects('MCP/summary.json',lambda v:v.update(manager_exit_codes=[0,0,0]))
    def test_late_writer_not_hidden(self):self.rejects('MCP/process-observations.json',lambda rows:rows[0].update(late_write_exists=True))
    def test_parent_exit_not_native_cancellation(self):self.rejects('MCP/parent-kill-canceled.json',lambda v:v['job'].update(terminal=False))
    def test_exact_pending_request_not_replaced(self):self.rejects('MCP/parent-EOF-canceled.json',lambda v:v['request'].update(idempotency_key='invented-new-key'))
    def test_recovery_does_not_relabel_input(self):self.rejects('MCP/parent-EOF-recovered.json',lambda v:v.update(input_id='f'*64))
    def test_old_reader_does_not_follow_current(self):self.rejects('MCP/v1-during-update-read.json',lambda v:v['pack'].update(bodies=[]))
    def test_actual_MCP_wire_reply_not_omitted(self):self.rejects('MCP/primary-session/mcp.stdout.jsonl',lambda rows:rows.pop())
    def test_python_denominator_not_reduced(self):self.rejects('validation.json',lambda v:v.update(manager_python_tests=248))
    def test_go_reference_not_claimed_fresh(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))
    def test_go_skips_not_pass(self):self.rejects('validation.json',lambda v:v['module'].update(skipped_tests=0))
    def test_failed_lint_not_accepted(self):self.rejects('validation.json',lambda v:v['gates'].update(lint=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
