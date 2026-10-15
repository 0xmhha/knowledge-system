"""Reject false authenticated review, current adoption and DB readiness."""
import importlib.util
import json
from pathlib import Path
import unittest


def module(name,file):
    spec = importlib.util.spec_from_file_location(name,Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = module('review_guard','check-knowledge-manager-review-evidence.py')
base = module('collection_evidence_helpers','test_maintenance_runtime_evidence.py')
base.guard = guard


class ReviewEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash
    edit = base.MaintenanceRuntimeEvidenceTests.edit

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder),[])

    def test_missing_actual_command(self):
        self.edit('runtime/commands.json',lambda rows:rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_hidden_negative_failure(self):
        self.edit('runtime/commands.json',lambda rows:next(r for r in rows if r['expected_error']).update(exit_code=0))
        self.assertTrue(guard.check(self.folder))

    def test_controlled_response_is_not_external_identity(self):
        self.edit('runtime/summary.json',lambda row:row.update(external_human_identity_authenticated=True))
        self.assertTrue(guard.check(self.folder))

    def test_controlled_response_is_not_independent_acceptance(self):
        self.edit('runtime/summary.json',lambda row:row.update(independent_human_acceptance=True))
        self.assertTrue(guard.check(self.folder))

    def test_input_adoption_is_not_DB_publication(self):
        self.edit('runtime/summary.json',lambda row:row.update(db_published=True))
        self.assertTrue(guard.check(self.folder))

    def test_input_adoption_is_not_reader_verification(self):
        self.edit('runtime/summary.json',lambda row:row.update(managed_reader_verified=True))
        self.assertTrue(guard.check(self.folder))

    def test_questions_cannot_have_invented_answers(self):
        self.edit('runtime/review-plan.stdout.txt',lambda row:row['questions'][0].update(answer_status='approved'))
        self.assertTrue(guard.check(self.folder))

    def test_shared_evidence_does_not_prove_contradiction(self):
        self.edit('runtime/review-plan.stdout.txt',lambda row:row['shared_evidence_interpretations'][0].update(classification='proven_contradiction'))
        self.assertTrue(guard.check(self.folder))

    def test_withdrawn_input_is_not_current(self):
        self.edit('runtime/review-status.stdout.txt',lambda row:row.update(current_adoption_id='0'*64))
        self.assertTrue(guard.check(self.folder))

    def test_source_cannot_change(self):
        self.edit('runtime/source-after.json',lambda row:row.update({next(iter(row)):'0'*64}))
        self.assertTrue(guard.check(self.folder))

    def test_old_Go_cannot_be_claimed_fresh(self):
        self.edit('validation.json',lambda row:row.update(current_Go_validation_reused=False))
        self.assertTrue(guard.check(self.folder))

    def test_failed_lint(self):
        self.edit('validation.json',lambda row:row['gates'].update(lint=1))
        self.assertTrue(guard.check(self.folder))

    def test_full_manager_not_complete(self):
        path = self.folder/'manifest.json'
        data = json.loads(path.read_text())
        data['full_KM03_complete']=True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))


if __name__=='__main__':
    unittest.main()
