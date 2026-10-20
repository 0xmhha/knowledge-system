"""Reject false document origins, held publication, broker/host and Go claims."""
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


guard = module('document_index_guard','check-manager-document-index-evidence.py')
base = module('collection_evidence_helpers','test_maintenance_runtime_evidence.py')
base.guard = guard


class DocumentIndexEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash
    edit = base.MaintenanceRuntimeEvidenceTests.edit

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder),[])

    def test_missing_actual_response(self):
        self.edit('runtime/reader-session/mcp.stdout.jsonl.gz',lambda rows:rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_held_candidate_cannot_be_published(self):
        self.edit('runtime/job.json',lambda row:row.update(state='succeeded',activated=True))
        self.assertTrue(guard.check(self.folder))

    def test_EOF_failure_is_not_hidden(self):
        self.edit('runtime/summary.json',lambda row:row.update(EOF_exit_codes=[0,1,0]))
        self.assertTrue(guard.check(self.folder))

    def test_native_reader_is_not_manager_broker(self):
        self.edit('runtime/summary.json',lambda row:row.update(manager_reader_broker_verified=True))
        self.assertTrue(guard.check(self.folder))

    def test_native_MCP_is_not_actual_Claude_host(self):
        self.edit('runtime/summary.json',lambda row:row.update(actual_Claude_host_workflow_verified=True))
        self.assertTrue(guard.check(self.folder))

    def test_document_chunks_cannot_be_zero(self):
        self.edit('runtime/vector-manifest.json',lambda row:row['languages'].update(markdown=0))
        self.assertTrue(guard.check(self.folder))

    def test_changed_source_fallback_refused(self):
        self.edit('runtime/document-read-after-live-deletion.json',lambda row:row.update(bodies=[]))
        self.assertTrue(guard.check(self.folder))

    def test_foreign_project_citation_refused(self):
        self.edit('runtime/document-read.json',lambda row:row['citations'][0].update(project_id='foreign'))
        self.assertTrue(guard.check(self.folder))

    def test_retained_document_origin_required(self):
        self.edit('runtime/retained-manifest.json',lambda row:row['files'].__setitem__(slice(None),[r for r in row['files']if r['origin_id']=='repo']))
        self.assertTrue(guard.check(self.folder))

    def test_old_Go_cannot_be_reused_for_changed_source(self):
        self.edit('validation.json',lambda row:row.update(current_Go_validation_reused=True))
        self.assertTrue(guard.check(self.folder))

    def test_skips_cannot_be_reclassified_as_pass(self):
        def mutate(row):
            row['module']['passed_tests']+=row['module']['skipped_tests']
            row['module']['skipped_tests']=0
        self.edit('validation.json',mutate)
        self.assertTrue(guard.check(self.folder))

    def test_failed_lint(self):
        self.edit('validation.json',lambda row:row['gates'].update(lint=1))
        self.assertTrue(guard.check(self.folder))

    def test_partial_A_is_not_full_KM03_4(self):
        path = self.folder/'manifest.json'
        data = json.loads(path.read_text())
        data['KM03_4_complete']=True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':
    unittest.main()
