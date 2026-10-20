"""Reject readiness evidence promoted to formal execution or quality."""
import importlib.util
import json
from pathlib import Path
import unittest

def module(name,file):
 s=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

guard=module('acceptance_readiness_guard','check-acceptance-readiness.py');base=module('acceptance_readiness_helpers','test_maintenance_runtime_evidence.py');base.guard=guard
class AcceptanceReadinessTests(unittest.TestCase):
 setUp=base.MaintenanceRuntimeEvidenceTests.setUp
 rehash=base.MaintenanceRuntimeEvidenceTests.rehash
 def rejects(self,name,action):base.MaintenanceRuntimeEvidenceTests.edit(self,name,action);self.assertTrue(guard.check(self.folder))
 def promote(self,key):
  p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
 def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
 def test_no_frozen_protocol(self):self.promote('protocol_frozen')
 def test_no_operational_acceptance(self):self.promote('operational_acceptance')
 def test_no_quality(self):self.promote('quality_acceptance')
 def test_no_human_acceptance(self):self.promote('independent_acceptance')
 def test_no_real_model(self):self.promote('real_model_inference')
 def test_model_metadata_not_inference(self):self.rejects('runtime/runtime.json',lambda v:v.update(actual_model_calls=1))
 def test_host_version_not_historical(self):self.rejects('runtime/runtime.json',lambda v:v['host'].update(version='2.1.300 (Claude Code)'))
 def test_missing_metadata_response(self):self.rejects('runtime/cks-session/mcp.stdout.jsonl.gz',lambda v:v.pop())
 def test_consumer_schema_drift(self):self.rejects('runtime/consumer-tools.json',lambda v:v.pop(next(iter(v))))
 def test_no_invented_gold(self):self.rejects('scoring-review-proposals.json',lambda v:v.update(independent_gold_records=9))
 def test_no_invented_scope_approval(self):self.rejects('freeze-review-checklist.json',lambda v:v['requirements'][0].update(decision_status='approved'))
 def test_host_missing_case(self):self.rejects('host/cases.json',lambda v:v.pop())
 def test_incomplete_not_green(self):self.rejects('host/incomplete/exit.json',lambda v:v.update(receipt_check_exit=0))
 def test_formal_state_preserved(self):self.rejects('formal-state-after.json',lambda v:v.update(execution_allowed=True))
 def test_Go_not_fresh(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))
if __name__=='__main__':unittest.main()
