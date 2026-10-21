"""Reject missing/changed calls, false shutdown and effect/quality promotion."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision

def module(name,file):
 s=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

guard=module('expanded_capture_guard','check-ontology-expanded-capture.py');base=module('expanded_capture_helpers','test_maintenance_runtime_evidence.py');base.guard=guard
class ExpandedCaptureEvidenceTests(unittest.TestCase):
 setUp=base.MaintenanceRuntimeEvidenceTests.setUp
 rehash=base.MaintenanceRuntimeEvidenceTests.rehash
 def rejects(self,name,mutate):base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
 def promote(self,key):
  p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
 def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
 def test_not_effect_verdict(self):self.promote('effect_scored')
 def test_no_quality_acceptance(self):self.promote('quality_acceptance')
 def test_no_independent_acceptance(self):self.promote('independent_acceptance')
 def test_no_human_gold(self):self.promote('human_label_approval')
 def test_no_full_OE(self):self.promote('full_OE01_complete')
 def test_missing_capture(self):self.rejects('runtime/capture-inventory.json',lambda v:v.pop())
 def test_changed_request(self):self.rejects('runtime/EX-01-off.json.gz',lambda v:v['rows'][0]['call']['arguments'].update(prompt='different'))
 def test_false_source(self):self.rejects('runtime/EX-01-off.json.gz',lambda v:v['rows'][0]['call']['response']['structuredContent']['coordinates'].update(project_id='foreign'))
 def test_failed_shutdown(self):self.rejects('runtime/EX-01-off.json.gz',lambda v:v['session_closures'][0].update(error='failure'))
 def test_meter_still_active(self):self.rejects('runtime/meter-closure.json',lambda v:v.update(active_requests=1))
 def test_model_drift(self):self.rejects('runtime/model-after.json',lambda v:v.update(digest='0'*64))
 def test_false_verification_summary(self):self.rejects('runtime/verification.json',lambda v:v.update(actual_capture_calls=511))
 def test_source_not_preserved(self):self.rejects('runtime/preservation.json',lambda v:v.update(dataset_files_unchanged=False))
 def test_reused_Go_not_fresh(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
