"""Reject stale publication and promotion of native checks to a scheduler."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));value=importlib.util.module_from_spec(spec);spec.loader.exec_module(value);return value


guard=module('source_check_guard','check-manager-source-check-evidence.py')
base=module('source_check_evidence_helpers','test_maintenance_runtime_evidence.py');base.guard=guard


class SourceCheckEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash

    def edit(self,name,mutate):
        if not(self.folder/name).exists()and(self.folder/(name+'.gz')).exists():name+='.gz'
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate)

    def rejects(self,name,mutate):self.edit(name,mutate);self.assertTrue(guard.check(self.folder))

    def test_current_packet(self):self.assertEqual(guard.check(self.folder),[])

    def test_A_is_not_full_KM04_3(self):
        path=self.folder/'manifest.json';value=json.loads(path.read_text());value['full_KM04_3_complete']=True;path.write_text(json.dumps(value));self.assertTrue(guard.check(self.folder))

    def test_native_build_is_not_scheduler(self):self.rejects('runtime/summary.json',lambda r:r.update(scheduler_runtime_verified=True))

    def test_independent_acceptance_not_invented(self):
        path=self.folder/'manifest.json';value=json.loads(path.read_text());value['independent_acceptance']=True;path.write_text(json.dumps(value));self.assertTrue(guard.check(self.folder))

    def test_current_cannot_switch_on_stale_input(self):self.rejects('runtime/v2-current.json',lambda r:r.update(link_target='guarded-v2'))

    def test_stale_job_not_success(self):self.rejects('runtime/v2-job.json',lambda r:r.update(state='succeeded',activated=True))

    def test_open_reader_cannot_mix_new_source(self):self.rejects('runtime/v1-after-refusal.json',lambda r:r.update(bodies=[]))

    def test_operator_check_must_use_bound_source_probe(self):
        self.rejects('runtime/guarded-v3-maintenance.json',lambda r:r['maintenance']['profiles'][0]['source_check']['command'].__setitem__(2,'doctor'))

    def test_manager_code_pins_must_match(self):
        def mutate(row):
            pins=row['maintenance']['profiles'][0]['source_check']['pinned_files'];key=next(k for k in pins if k.endswith('/manager.py'));pins[key]='0'*64
        self.rejects('runtime/guarded-v1-maintenance.json',mutate)

    def test_native_response_required(self):self.rejects('runtime/v1-reader-session/mcp.stdout.jsonl',lambda rows:rows.pop())

    def test_EOF_error_not_hidden(self):self.rejects('runtime/summary.json',lambda r:r['EOF_exit_codes'].__setitem__(0,1))

    def test_deadline_diagnostic_cannot_omit_a_success(self):
        self.rejects('guardian-strengthened.jsonl',lambda rows:rows.pop(next(i for i,r in enumerate(rows)if r.get('Test')and r['Action']=='pass')))

    def test_skip_not_pass(self):self.rejects('validation.json',lambda r:r['module'].update(skipped_tests=0))

    def test_failed_lint(self):self.rejects('validation.json',lambda r:r['gates'].update(lint=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
