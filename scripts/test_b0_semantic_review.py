import copy
import importlib.util
import json
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location('b0_review', Path(__file__).with_name('b0-prepare-semantic-review.py'))
MOD = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MOD)

class SourceProposalTests(unittest.TestCase):
    def test_policy_preserves_original_wording_and_all_facts_stay_proposed(self):
        book = json.loads(MOD.MAT.DEFAULT.read_bytes())
        for family in ['F-05', 'F-06']:
            fixture = next(f for f in book['fixtures'] if f['family'] == family and f['evaluation_partition'] == 'development')
            for state in ['a', 'b'] if family == 'F-05' else ['current']:
                inputs = MOD.proposed_inputs(fixture, {'name':state,'project_id':MOD.MAT.fixture_project_id(fixture,state)})
                policy = json.loads(inputs['.cks/knowledge/policies/refund-cap.yaml'])
                source = 'README.md' if family == 'F-06' else 'project-' + state + '/docs/policy.md'
                self.assertEqual(policy['statement'], fixture['sources'][source].splitlines()[2])
                self.assertEqual(policy['scope'], {'subsystem':'refund'})
                self.assertEqual(policy['status'], 'proposed')
                self.assertNotIn('reviewed_by', policy)
                ontology = json.loads(inputs['ontology.yaml'])
                self.assertTrue(all(c['status']=='proposed' and not c.get('reviewed_by') for c in ontology['concepts']))
                if family == 'F-06':
                    req = json.loads(inputs['spec.yaml'])['requirements'][0]
                    self.assertEqual(req['statement'], policy['statement'])
                    self.assertEqual(req['status'], 'proposed')
                    self.assertEqual(req['concept_ids'], ['refund-cap'])
                    self.assertNotIn('TestHealth', json.dumps(req))
                else:
                    self.assertNotIn('spec.yaml', inputs)
                    self.assertIn('does not enforce a refund policy', ontology['concepts'][0]['definition'])
    def test_f03_original_core_vocabulary_is_not_rewritten(self):
        fixture = {'family':'F-03'}
        self.assertEqual(MOD.proposed_inputs(fixture, {'project_id':'b0-f03-dev'}), {})
        source = {'family':'F-03','sources':{'ontology.yaml':'project_id: one\nproject_id: two\n'}}
        with self.assertRaisesRegex(ValueError,'one explicit'):
            MOD.MAT.fixture_project_id(source,'current')
    def test_candidate_verification_refuses_unrelated_acceptance_edge_before_source_io(self):
        projection={'snapshot':{},'evidence':[],'assertions':[{'id':'bad','status':'proposed','predicate':'CHECKED_BY'}]}
        with self.assertRaisesRegex(ValueError,'unrelated test'):
            MOD.verify_projection(projection, Path('/unused'))
        projection['assertions'][0]['status']='verified'
        with self.assertRaisesRegex(ValueError,'must not approve'):
            MOD.verify_projection(projection, Path('/unused'))

if __name__=='__main__':
    unittest.main()
