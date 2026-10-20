"""Reject broker publication, fallback, hidden errors and invented host claims."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision


def module(name,file):
    spec = importlib.util.spec_from_file_location(name,Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = module('broker_guard','check-knowledge-manager-broker-evidence.py')
base = module('collection_evidence_helpers','test_maintenance_runtime_evidence.py')
base.guard = guard


class BrokerEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash
    edit = base.MaintenanceRuntimeEvidenceTests.edit

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder),[])

    def test_missing_actual_response(self):
        self.edit('runtime/manager-session/mcp.stdout.jsonl.gz',lambda rows:rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_proxy_cannot_claim_actual_Claude_host(self):
        self.edit('runtime/summary.json',lambda row:row.update(actual_Claude_host_workflow_verified=True))
        self.assertTrue(guard.check(self.folder))

    def test_EOF_failure_is_not_hidden(self):
        self.edit('runtime/summary.json',lambda row:row.update(EOF_exit_codes=[0,1,0]))
        self.assertTrue(guard.check(self.folder))

    def test_held_is_not_published(self):
        self.edit('runtime/job.json',lambda row:row.update(activated=True,state='succeeded'))
        self.assertTrue(guard.check(self.folder))

    def test_legacy_mutations_cannot_be_exposed(self):
        self.edit('runtime/tools.json',lambda rows:rows[0].update(name='cks.ops.index'))
        self.assertTrue(guard.check(self.folder))

    def test_reader_restart_cannot_switch_dataset(self):
        self.edit('runtime/read-restart.json',lambda row:row['coordinates'].update(dataset_id='0'*64))
        self.assertTrue(guard.check(self.folder))

    def test_foreign_project_citation(self):
        self.edit('runtime/read-first.json',lambda row:row['citations'][0].update(project_id='foreign'))
        self.assertTrue(guard.check(self.folder))

    def test_past_read_does_not_prove_publication(self):
        self.edit('runtime/state-after-read.json',lambda row:row['last_validated_read'].update(candidate_publication_inferred=True))
        self.assertTrue(guard.check(self.folder))

    def test_corruption_must_be_refused(self):
        self.edit('runtime/corruption-refusal.json',lambda row:row.update(isError=False))
        self.assertTrue(guard.check(self.folder))

    def test_refusal_cannot_be_hidden(self):
        def mutate(rows):
            next(r for r in rows if r['expected_error'])['response']['isError']=False
        self.edit('runtime/calls.json',mutate)
        self.assertTrue(guard.check(self.folder))

    def test_reused_Go_cannot_be_claimed_fresh(self):
        self.edit('validation.json',lambda row:row.update(current_Go_validation_reused=False))
        self.assertTrue(guard.check(self.folder))

    def test_failed_lint(self):
        self.edit('validation.json',lambda row:row['gates'].update(lint=1))
        self.assertTrue(guard.check(self.folder))

    def test_partial_B_is_not_full_KM03_4(self):
        path = self.folder/'manifest.json'
        data = json.loads(path.read_text())
        data['KM03_4_complete']=True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':
    unittest.main()
