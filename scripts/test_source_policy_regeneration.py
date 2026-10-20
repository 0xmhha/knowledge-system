#!/usr/bin/env python3
"""False-completion controls for actual policy DB/MCP verification evidence."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('source_regeneration_guard', ROOT / 'scripts/check-source-policy-regeneration.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)

class SourceRegenerationTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='ks-source-regeneration-tests-', dir='/private/tmp')
        self.addCleanup(self.tmp.cleanup)
        self.folder = Path(self.tmp.name) / 'packet'
        shutil.copytree(guard.PACKET, self.folder)

    def mutate(self, name, action):
        p = self.folder / name
        d = json.loads(p.read_text())
        action(d)
        p.write_text(json.dumps(d))
        manifest = json.loads((self.folder / 'manifest.json').read_text())
        for row in manifest['artifacts']:
            if row['path'] == name:
                row.update(sha256=hashlib.sha256(p.read_bytes()).hexdigest(), bytes=p.stat().st_size)
        (self.folder / 'manifest.json').write_text(json.dumps(manifest))

    def test_current_packet_is_local_verification(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_rehashed_ontology_quality_claim_is_rejected(self):
        self.mutate('summary.json', lambda d: d.update(ontology_effect_evaluated=True))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_missing_policy_edges_is_rejected(self):
        self.mutate('persisted-policy-edges.json', lambda d: d.pop())
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_wrong_file_match_is_rejected(self):
        self.mutate('persisted-policy-edges.json', lambda d: d[0].update(file_path='wrong-project.go'))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_generator_fix_claim_is_rejected(self):
        self.mutate('generator-status.json', lambda d: d.update(regeneration_fix_complete=False))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_original_yaml_approval_is_rejected(self):
        self.mutate('cks-results.json', lambda d: d[0]['response']['annotations'][0]['provenance']['approval'].update(status='verified'))
        self.assertTrue(guard.check(self.folder))

    def test_wire_canonical_source_parity_is_checked(self):
        p = self.folder / 'cks/mcp.stdout.jsonl'
        rows = [json.loads(line) for line in p.read_text().splitlines()]
        requests = [json.loads(line) for line in (self.folder / 'cks/mcp.stdin.jsonl').read_text().splitlines()]
        target = next(r['id'] for r in requests if 'canonical_id' in r.get('params', {}).get('arguments', {}))
        reply = next(r for r in rows if r.get('id') == target)
        reply['result']['structuredContent']['annotations'] = []
        p.write_text(''.join(json.dumps(r) + '\n' for r in rows))
        manifest = json.loads((self.folder / 'manifest.json').read_text())
        for row in manifest['artifacts']:
            if row['path'] == 'cks/mcp.stdout.jsonl':
                row.update(sha256=hashlib.sha256(p.read_bytes()).hexdigest(), bytes=p.stat().st_size)
        (self.folder / 'manifest.json').write_text(json.dumps(manifest))
        self.assertTrue(guard.check(self.folder))

    def test_omitted_or_tampered_wire_is_rejected(self):
        (self.folder / 'cks/mcp.stdout.jsonl').write_text('')
        self.assertTrue(guard.check(self.folder))

    def test_output_refusal_cannot_be_hidden(self):
        self.mutate('generator-status.json',lambda d:d.update(both_outputs_preserved_on_refusal=False))
        self.assertTrue(guard.check(self.folder))

    def test_source_mode_must_be_explicit(self):
        self.mutate('generator-status.json',lambda d:d.update(owned_entry_file_corrections=2))
        self.assertTrue(guard.check(self.folder))

    def test_no_reuse_of_prior_Go(self):
        self.mutate('validation.json',lambda d:d.update(current_Go_validation_reused=True))
        self.assertTrue(guard.check(self.folder))

    def test_original_inputs_must_be_preserved(self):
        self.mutate('preservation-after.json',lambda d:d.update(original_changed=True))
        self.assertTrue(guard.check(self.folder))

    def test_original_approval_cannot_be_invented(self):
        self.mutate('generator-status.json',lambda d:d.update(independent_acceptance=True))
        self.assertTrue(guard.check(self.folder))

    def test_owned_entry_cannot_gain_approval(self):
        p=self.folder/'owned-corrected-A11.yaml.txt'
        p.write_text(p.read_text()+'\nreviewer: invented\n')
        mpath=self.folder/'manifest.json';m=json.loads(mpath.read_text())
        row=next(r for r in m['artifacts']if r['path']==p.name)
        row.update(sha256=hashlib.sha256(p.read_bytes()).hexdigest(),bytes=p.stat().st_size)
        mpath.write_text(json.dumps(m))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
