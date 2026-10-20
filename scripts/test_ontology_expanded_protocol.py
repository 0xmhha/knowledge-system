"""Reject fake gold/execution, changed source targets and inflated denominators."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision

def module(name,file):
 s=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

guard=module('expanded_protocol_guard','check-ontology-expanded-protocol.py');base=module('expanded_protocol_helpers','test_maintenance_runtime_evidence.py');base.guard=guard
class ExpandedProtocolEvidenceTests(unittest.TestCase):
 setUp=base.MaintenanceRuntimeEvidenceTests.setUp
 rehash=base.MaintenanceRuntimeEvidenceTests.rehash
 def rejects(self,name,mutate):base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
 def promote(self,key):
  p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
 def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
 def test_no_execution_claim(self):self.promote('new_retrieval_executed')
 def test_no_quality_acceptance(self):self.promote('quality_acceptance')
 def test_no_independent_acceptance(self):self.promote('independent_acceptance')
 def test_no_human_label_approval(self):self.promote('human_label_approval')
 def test_no_full_OE(self):self.promote('full_OE01_complete')
 def test_repeats_not_facts(self):self.rejects('inputs/protocol.json',lambda v:v.update(fact_families=512))
 def test_missing_arm(self):self.rejects('inputs/protocol.json',lambda v:v['arms'].pop())
 def test_control_not_general_relevance_gold(self):self.rejects('inputs/private-function-criteria.json',lambda v:v.update(independent_quality_gold=True))
 def test_source_body_changed(self):self.rejects('inputs/private-function-criteria.json',lambda v:v['units'][0].update(expected_code_body='forged'))
 def test_oracle_not_public_input(self):self.rejects('inputs/requests.json',lambda v:v['requests'][0]['arguments'].update(expected_unit_ids=['quorum-size']))
 def test_changed_model(self):self.rejects('inputs/protocol.json',lambda v:v['model'].update(digest='0'*64))
 def test_actual_calls_not_in_preregistration(self):self.rejects('new-query-execution.json',lambda v:v.update(new_retrieval_calls=512))
 def test_failed_historical_restore(self):self.rejects('historical-design.json',lambda v:v.update(verification_exit_code=1))
 def test_reused_Go_not_fresh(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
