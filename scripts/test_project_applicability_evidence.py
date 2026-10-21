"""Reject false scope and denominator claims for proposed source binding foundation."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision

def module(name,file):
    s=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

guard=module('applicability_guard','check-project-applicability-evidence.py');base=module('applicability_helpers','test_maintenance_runtime_evidence.py');base.guard=guard
class ApplicabilityEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash
    def rejects(self,name,mutate):
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
    def promote(self,key):
        p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
    def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
    def test_full_library_scope_required(self):
        p=self.folder/'manifest.json';v=json.loads(p.read_text());v['full_KM05_complete']=False;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
    def test_unsupported_not_project_absence(self):self.rejects('coverage-scope.json',lambda v:v.update(unsupported_scope='project_feature_absent'))
    def test_uncollected_not_whole_project_absence(self):self.rejects('coverage-scope.json',lambda v:v.update(not_collected_scope='entire_project_unindexed'))
    def test_no_authentication_claim(self):self.rejects('review-scope.json',lambda v:v.update(reviewer_identity_authenticated=True))
    def test_no_human_approval_invented(self):self.promote('human_mapping_approval')
    def test_no_MCP(self):self.promote('comparison_MCP_implemented')
    def test_no_quality_acceptance(self):self.promote('quality_acceptance')
    def test_go_not_reused(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=True))
    def test_missing_race_package(self):self.rejects('validation.json',lambda v:v['race'].update(packages=6))
    def test_failed_lint(self):self.rejects('validation.json',lambda v:v['gates'].update(lint=1))
    def test_skip_not_pass(self):self.rejects('validation.json',lambda v:v['module'].update(skipped_tests=0))
    def test_old_scheme_proof_required(self):self.rejects('historical-design.json',lambda v:v.update(verification_exit_code=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
