"""Reject inflated metrics, hidden scope mismatch, cost and quality claims."""
import importlib.util
import json
from pathlib import Path
import unittest

def module(name,file):
 s=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

guard=module('expanded_score_guard','check-ontology-expanded-score.py');base=module('expanded_score_helpers','test_maintenance_runtime_evidence.py');base.guard=guard
class ExpandedScoreEvidenceTests(unittest.TestCase):
 setUp=base.MaintenanceRuntimeEvidenceTests.setUp
 rehash=base.MaintenanceRuntimeEvidenceTests.rehash
 def rejects(self,name,mutate):base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
 def promote(self,key):
  p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
 def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
 def test_no_general_quality(self):self.promote('quality_acceptance')
 def test_no_independent_acceptance(self):self.promote('independent_acceptance')
 def test_no_human_gold(self):self.promote('human_label_approval')
 def test_no_answer_quality(self):self.promote('LLM_answer_quality_evaluated')
 def test_no_full_OE(self):self.promote('full_OE01_complete')
 def test_inflated_recall(self):self.rejects('runtime/effect-summary.json',lambda v:v['arms']['concept_text']['family_macro'].update(recall_at10=1))
 def test_invented_relation_effect(self):self.rejects('runtime/effect-summary.json',lambda v:v['arms']['relations'].update(applied_relations_all_phases=1))
 def test_scope_mismatch_hidden(self):self.rejects('runtime/function-retrieval-scores.json',lambda rows:next(q for q in rows if q['id']=='EX-16'and q['arm']=='concept_text')['observations'][0].update(out_of_intent_concepts=[]))
 def test_latency_hidden(self):self.rejects('runtime/effect-summary.json',lambda v:v['arms']['concept_text'].update(warm_p95_ms=0))
 def test_cost_hidden(self):self.rejects('runtime/effect-summary.json',lambda v:v['arms']['concept_text'].update(embedding_API_requests_in_capture_batches=0))
 def test_default_promoted(self):self.rejects('recommendation.json',lambda v:v.update(ontology_default='combined',quality='accepted'))
 def test_wrong_benefit_scope(self):self.rejects('recommendation.json',lambda v:v.update(benefit_question_ids=['EX-01','EX-08']))
 def test_missing_raw_call(self):self.rejects('runtime/EX-01-off.json.gz',lambda v:v['rows'].pop())
 def test_reused_Go_not_fresh(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))
if __name__=='__main__':unittest.main()
