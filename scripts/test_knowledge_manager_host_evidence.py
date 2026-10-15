"""Reject invented host, lifecycle, quality and completion evidence."""
import importlib.util
from pathlib import Path
import unittest


def module(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = module('host_guard', 'check-knowledge-manager-host-evidence.py')
base = module('host_evidence_helpers', 'test_maintenance_runtime_evidence.py')
base.guard = guard


class HostEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash

    def edit(self, name, mutate):
        if not (self.folder/name).exists() and (self.folder/(name+'.gz')).exists():
            name += '.gz'
        base.MaintenanceRuntimeEvidenceTests.edit(self, name, mutate)

    def rejects(self, name, mutate):
        self.edit(name, mutate)
        self.assertTrue(guard.check(self.folder))

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_scripted_API_is_not_model_quality(self):
        self.rejects('runtime/summary.json', lambda r:r.update(real_model_calls=1))

    def test_independent_acceptance_not_invented(self):
        self.rejects('runtime/summary.json', lambda r:r.update(independent_acceptance=True))

    def test_failed_lint(self):
        self.rejects('validation.json', lambda r:r['gates'].update(lint=1))

    def test_skip_cannot_be_pass(self):
        self.rejects('validation.json', lambda r:r['module'].update(passed_tests=r['module']['test_cases'], skipped_tests=0))

    def test_local_API_cannot_be_replaced(self):
        self.rejects('runtime/full/invocation.json', lambda r:r['env'].update(ANTHROPIC_BASE_URL='https://external.invalid'))

    def test_raw_native_response_required(self):
        self.rejects('runtime/full/mcp-frames.jsonl', lambda rows:rows.pop(next(i for i,r in enumerate(rows) if r['kind']=='response')))

    def test_host_cannot_fabricate_native_result(self):
        self.rejects('runtime/full/actual-tool-results.json', lambda rows:rows[0].update(value={'fabricated':True}))

    def test_registered_skill_required(self):
        self.rejects('runtime/full/host.stdout.jsonl', lambda rows:next(r for r in rows if r.get('subtype')=='init').update(skills=[]))

    def test_manager_signal_failure_not_hidden(self):
        self.rejects('runtime/full/mcp-frames.jsonl', lambda rows:next(r for r in rows if r['kind']=='exit').update(exit_code=-2))

    def test_missing_engine_cannot_fallback(self):
        self.rejects('runtime/missing/mcp-frames.jsonl', lambda rows:next(r for r in rows if r['kind']=='exit').update(exit_code=0))

    def test_restart_must_read_same_candidate(self):
        self.rejects('runtime/full-restart/workflow.json', lambda r:r['read'].update(bodies=[]))

    def test_foreign_project_citation(self):
        self.rejects('runtime/full/workflow.json', lambda r:r['read']['citations'][0].update(project_id='foreign'))

    def test_cancel_cannot_be_success(self):
        self.rejects('runtime/kill-recover/workflow.json', lambda r:r['canceled_job'].update(state='succeeded'))

    def test_recovery_cannot_change_original_request(self):
        def mutate(rows):
            row=next(r for r in rows if r['kind']=='request' and 'cks.manager.resume_build' in r['raw'])
            row['raw']=row['raw'].replace('cks.manager.resume_build','cks.manager.status')
        self.rejects('runtime/kill-recover/mcp-frames.jsonl', mutate)

    def test_actual_native_validator_required(self):
        self.rejects('native/agents.stdout.json', lambda r:r.update(success=False))


if __name__ == '__main__':
    unittest.main()
