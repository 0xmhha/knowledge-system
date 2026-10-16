#!/usr/bin/env python3
"""Reject input leakage and parser-preflight claims presented as retrieval evaluation."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
BASE = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement'
PACKET = BASE / 'symptom-wire-preflight-v1'
spec = importlib.util.spec_from_file_location('wire_guard', ROOT / 'scripts/check-symptom-wire-preflight.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class WirePreflightTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['artifacts'] = {q.relative_to(self.packet).as_posix(): hashlib.sha256(q.read_bytes()).hexdigest()
                          for q in self.packet.rglob('*') if q.is_file() and q.name != 'manifest.json'}
        p.write_text(json.dumps(d))

    def change(self, name, update):
        p = self.packet / name
        d = json.loads(p.read_text())
        update(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.rehash()

    def rejected(self):
        self.assertTrue(guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_public_symbol_seed(self):
        self.change('public-bundle/wire-requests.json', lambda d: d['requests'][0]['arguments'].update(symbol='DecodeRLP'))
        self.rejected()

    def test_retrieval_claim(self):
        self.change('summary.json', lambda d: d.update(retrieval_runs=20))
        self.rejected()

    def test_recall_from_parser_acceptance(self):
        self.change('summary.json', lambda d: d.update(macro_recall_at_10=1.0))
        self.rejected()

    def test_repo_file_modified_claim(self):
        self.change('summary.json', lambda d: d.update(existing_test_file_byte_unchanged=False))
        self.rejected()

    def test_missing_control(self):
        self.change('summary.json', lambda d: d['controls'].pop())
        self.rejected()

    def test_bad_control_accepted(self):
        self.change('summary.json', lambda d: next(r for r in d['controls'] if r['control_id']=='unknown_gold_field').update(observed_valid=True))
        self.rejected()

    def test_public_budget_changed(self):
        self.change('public-bundle/wire-requests.json', lambda d: d['requests'][0]['arguments'].update(max_candidates=20))
        self.rejected()

    def test_raw_go_failure(self):
        p = self.packet / 'logs/normal.jsonl'
        with p.open('a') as f:
            f.write(json.dumps({'Action': 'fail', 'Package': 'github.com/0xmhha/knowledge-system/internal/system/mcp'}) + '\n')
        self.rehash()
        self.rejected()

    def test_parser_source_binding_changed(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['source_sha256']['internal/system/mcp/suspects.go'] = '0'*64
        p.write_text(json.dumps(d))
        self.rejected()

    def test_lint_failure_hidden(self):
        p = self.packet / 'logs/lint.stderr.txt'
        p.write_text('lint error\n')
        self.rehash()
        self.rejected()

    def test_backend_call_claim(self):
        self.change('summary.json', lambda d: d.update(backend_calls=1))
        self.rejected()

    def test_private_file_added_to_public_bundle_even_after_rehash(self):
        (self.packet / 'public-bundle/private-source-notes.json').write_text('{"secret":"synthetic"}')
        self.rehash()
        self.rejected()

    def test_exporter_rejects_private_fields_without_loading_private_packet(self):
        public = json.loads((BASE / 'symptom-inputs-v1/public-inputs.json').read_text())
        public['cases'][0]['source_anchors'] = ['synthetic control']
        with self.assertRaises(ValueError):
            guard.load('export-symptom-wire.py').render(public)


if __name__ == '__main__':
    unittest.main()
