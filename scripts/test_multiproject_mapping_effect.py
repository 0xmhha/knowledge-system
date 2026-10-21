"""Reject inflated coverage, mixed source groups, and invented acceptance."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision

def module(name,file):
 s=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

guard=module('mapping_effect_guard','check-multiproject-mapping-effect.py');base=module('mapping_effect_helpers','test_maintenance_runtime_evidence.py');base.guard=guard
class MappingEffectEvidenceTests(unittest.TestCase):
 setUp=base.MaintenanceRuntimeEvidenceTests.setUp
 rehash=base.MaintenanceRuntimeEvidenceTests.rehash
 def rejects(self,name,action):base.MaintenanceRuntimeEvidenceTests.edit(self,name,action);self.assertTrue(guard.check(self.folder))
 def promote(self,key):
  p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
 def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
 def test_no_quality_acceptance(self):self.promote('quality_acceptance')
 def test_no_independent_acceptance(self):self.promote('independent_acceptance')
 def test_no_human_mapping_approval(self):self.promote('human_mapping_approval')
 def test_no_answer_quality(self):self.promote('LLM_answer_quality_evaluated')
 def test_no_vocabulary_alone_claim(self):self.promote('ontology_vocabulary_alone_effect_isolated')
 def test_coverage_inflation(self):self.rejects('runtime/summary.json',lambda v:v['arms']['with-mapping'].update(source_evidence_cell_coverage=1))
 def test_missing_accuracy_is_not_perfect(self):self.rejects('runtime/summary.json',lambda v:v['arms']['without-mapping'].update(source_accuracy=1))
 def test_missing_unknowns(self):self.rejects('runtime/summary.json',lambda v:v['arms']['with-mapping'].update(unknown_cells=0))
 def test_repeated_facts_not_doubled(self):self.rejects('runtime/summary.json',lambda v:v['arms']['with-mapping'].update(unique_cells=80))
 def test_shutdown_failure(self):self.rejects('runtime/summary.json',lambda v:v.update(EOF_exit_codes=[0,1]))
 def test_omitted_wire_response(self):self.rejects('runtime/with-mapping/mcp.stdout.jsonl.gz',lambda rows:rows.pop())
 def test_scope_mixing(self):self.rejects('runtime/with-mapping-consensus-1.json',lambda v:v['groups'][0]['participant']['scope']['coordinates'].update(project_id='foreign'))
 def test_forged_source_body(self):self.rejects('runtime/with-mapping-consensus-1.json',lambda v:v['groups'][0]['cells'][0]['mappings'][0]['binding']['evidence'][0].update(body='invented'))
 def test_profile_difference(self):self.rejects('runtime/without-mapping.json',lambda v:v.update(max_body_bytes=1))
 def test_no_default_adoption(self):self.rejects('runtime/summary.json',lambda v:v.update(human_mapping_approval=True))
 def test_no_fresh_Go_claim(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
