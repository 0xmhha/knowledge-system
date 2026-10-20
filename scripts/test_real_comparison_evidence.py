"""Reject false actual-MCP, source integrity and completion claims."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision

def module(name,file):
    s=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

guard=module('real_comparison_guard','check-real-comparison-evidence.py');base=module('real_comparison_helpers','test_maintenance_runtime_evidence.py');base.guard=guard
class RealComparisonEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash
    def rejects(self,name,mutate):
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
    def promote(self,key):
        p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
    def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
    def test_no_quality_acceptance(self):self.promote('quality_acceptance')
    def test_no_independent_acceptance(self):self.promote('independent_acceptance')
    def test_no_human_approval(self):self.promote('human_mapping_approval')
    def test_false_readonly_metadata(self):self.rejects('runtime/tools.json',lambda rows:rows[0]['annotations'].update(destructiveHint=True))
    def test_missing_wire_response(self):self.rejects('runtime/comparison-session/mcp.stdout.jsonl.gz',lambda rows:rows.clear())
    def test_foreign_citation(self):self.rejects('runtime/matrix-consensus.json',lambda v:v['groups'][0]['cells'][0]['mappings'][0]['binding']['evidence'][0]['reference']['citation'].update(project_id='foreign'))
    def test_source_body_forgery(self):self.rejects('runtime/matrix-consensus.json',lambda v:v['groups'][0]['cells'][0]['mappings'][0]['binding']['evidence'][0].update(body='forged'))
    def test_shutdown_failure(self):self.rejects('runtime/summary.json',lambda v:v.update(EOF_exit_codes=[0,1,0]))
    def test_failed_lint(self):self.rejects('validation.json',lambda v:v['gates'].update(lint=1))
    def test_skip_not_pass(self):self.rejects('validation.json',lambda v:v['module'].update(skipped_tests=0))
    def test_missing_race_scope(self):self.rejects('validation.json',lambda v:v['race'].update(packages=6))
    def test_prior_contract_failure(self):self.rejects('historical-design.json',lambda v:v.update(verification_exit_code=1))
    def test_missing_candidate_budget_state(self):self.rejects('runtime/candidate-budget.json',lambda v:v.update(omitted_candidates=0))
    def test_affected_project_leaks_body(self):self.rejects('runtime/missing-B.json',lambda v:v['groups'][1]['cells'][0].update(state='similar'))
    def test_changed_DB_after_restoration(self):self.rejects('runtime/after.json',lambda v:v.update({next(iter(v)):'0'*64}))
    def test_no_ontology_effect_claim(self):self.promote('ontology_effect_evaluated')
    def test_original_code_equality_forged(self):self.rejects('runtime/source-equality.json',lambda v:next(iter(v.values())).update(original_sha256='0'*64))
    def test_capture_mode_hidden(self):self.rejects('runtime/fixtures.json',lambda v:v[1]['coordinates'].update(source_mode='committed'))
    def test_actual_DB_failure(self):self.rejects('build/go-wemix-job.json',lambda v:v.update(state='failed'))
    def test_alias_scope_hidden(self):self.rejects('build/go-wemix-source-scope.json',lambda v:v.update(excluded_non_regular=[]))
    def test_referenced_Go_is_not_fresh(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
