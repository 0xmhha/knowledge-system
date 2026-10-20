#!/usr/bin/env python3
"""False approval/leakage controls for the unapproved source packet."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('packet_check',ROOT/'scripts/check-policy-security-preparation.py')
check=importlib.util.module_from_spec(spec);spec.loader.exec_module(check)
PACKET=ROOT/'docs/spec-driven/operational-preparation/policy-security-drafts'

class PacketTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory(prefix='ks-source-packet-test-',dir='/private/tmp');self.addCleanup(self.temp.cleanup)
  self.folder=Path(self.temp.name)/'packet';shutil.copytree(PACKET,self.folder)
 def mutate(self,name,action):
  p=self.folder/name;value=json.loads(p.read_text());action(value);p.write_text(json.dumps(value))
  manifest=json.loads((self.folder/'manifest.json').read_text())
  for row in manifest['artifacts']:
   if row['path']==name:row['sha256']=hashlib.sha256(p.read_bytes()).hexdigest()
  (self.folder/'manifest.json').write_text(json.dumps(manifest))
 def test_current_source_packet_is_valid_but_not_gold(self):
  self.assertEqual(check.check(self.folder),[])
 def test_rehashed_private_approval_is_not_accepted(self):
  self.mutate('private/review-drafts.json',lambda d:d.update(approved=True))
  self.assertTrue(check.check(self.folder))
 def test_public_expected_answers_are_rejected(self):
  self.mutate('public/inputs.json',lambda d:d['cases'][0].update(expected_policy_ids=['answer']))
  self.assertTrue(check.check(self.folder))
 def test_nested_source_answer_fields_are_rejected(self):
  self.mutate('public/inputs.json',lambda d:d['cases'][0]['source'].update(expected_security=True))
  self.assertTrue(check.check(self.folder))
 def test_premature_final_or_scoring_is_rejected(self):
  self.mutate('public/inputs.json',lambda d:d['cases'][0].update(split='FINAL',scoring_allowed=True))
  self.assertTrue(check.check(self.folder))
 def test_call_hint_cannot_be_promoted_to_resolved_gold(self):
  self.mutate('private/rule-source-census.json',lambda d:d['calls'][0].update(semantic_binding='resolved',gold_status='approved'))
  self.assertTrue(check.check(self.folder))
 def test_source_reference_is_not_a_graph_result(self):
  self.mutate('private/review-drafts.json',lambda d:d['policy_reference_inventory'][0].update(graph_resolution='pass'))
  self.assertTrue(check.check(self.folder))
 def test_tampered_excerpts_and_artifact_paths_fail(self):
  (self.folder/'public/SPC-001-source.txt').write_text('changed')
  self.assertTrue(check.check(self.folder))
  manifest=json.loads((self.folder/'manifest.json').read_text());manifest['artifacts'][0]['path']='../outside'
  (self.folder/'manifest.json').write_text(json.dumps(manifest))
  self.assertTrue(check.check(self.folder))

if __name__=='__main__':unittest.main()
