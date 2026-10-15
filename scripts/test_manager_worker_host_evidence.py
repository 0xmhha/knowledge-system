"""Reject lifecycle claims without actual process/terminal/native read proof."""
import importlib.util
import json
from pathlib import Path
import unittest


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

guard=module('worker_host_guard','check-manager-worker-host-evidence.py')
base=module('worker_host_helpers','test_maintenance_runtime_evidence.py');base.guard=guard


class WorkerHostEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash
    def rejects(self,name,mutate):base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
    def test_current_packet(self):self.assertEqual(guard.check(self.folder),[])
    def test_CLI_not_MCP_operation(self):self.rejects('runtime/summary.json',lambda v:v.update(manager_MCP_worker_operation_verified=True))
    def test_CLI_not_sustained_schedule(self):self.rejects('runtime/summary.json',lambda v:v.update(sustained_periodic_schedule_verified=True))
    def test_unobserved_native_exit_not_zero(self):self.rejects('runtime/summary.json',lambda v:v.update(actual_native_exit_codes=[0,0,0]))
    def test_killed_worker_cannot_exit_normally(self):self.rejects('runtime/commands.json',lambda rows:next(c for c in rows if c['label']=='sigkill').update(exit_code=0))
    def test_late_writer_not_hidden(self):self.rejects('runtime/process-observations.json',lambda rows:rows[0].update(late_write_exists=True))
    def test_current_cannot_swap_on_stop(self):self.rejects('runtime/process-observations.json',lambda rows:rows[0].update(current_after='unpublished'))
    def test_live_owned_child_not_hidden(self):self.rejects('runtime/process-observations.json',lambda rows:rows[1]['post_stop_process_observations'][0].update(running=True))
    def test_pending_intent_not_replaced(self):self.rejects('runtime/sigkill-after.json',lambda v:v.update(pending=None))
    def test_cancellation_must_be_native_terminal_result(self):self.rejects('runtime/sigterm-reconciled.json',lambda v:next(r for r in v['state']['attempts']if r['id']==v['state']['last_outcome'])['job'].update(terminal=False))
    def test_retry_cannot_change_original_input(self):self.rejects('runtime/budget-recovered.json',lambda v:next(r for r in v['state']['attempts']if r['id']==v['state']['last_success']).update(input_id='f'*64))
    def test_recovered_read_cannot_be_metadata(self):self.rejects('runtime/sigkill-recovered-read.json',lambda v:v.update(bodies=[]))
    def test_Go_skip_not_pass(self):self.rejects('validation.json',lambda v:v['module'].update(skipped_tests=0))
    def test_snapshot_origin_not_relabelled_current_disk(self):self.rejects('runtime/sigterm-stopped.json',lambda v:v.update(state_snapshot_origin='latest_disk_state'))
    def test_plugin_errors_not_hidden(self):self.rejects('plugin-validate.json',lambda v:v.update(success=False))
    def test_new_Python_not_claimed_reused(self):self.rejects('validation.json',lambda v:v.update(current_manager_Python_rerun=False))


if __name__=='__main__':unittest.main()
