"""Reject design evidence omissions and claims of unimplemented public jobs."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

spec = importlib.util.spec_from_file_location('maintenance_evidence', Path(__file__).with_name('check-maintenance-design-evidence.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class MaintenanceDesignEvidenceTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='ks-maintenance-evidence-', dir='/private/tmp')
        self.addCleanup(temp.cleanup)
        self.folder = Path(temp.name) / 'packet'
        shutil.copytree(guard.PACKET, self.folder)

    def edit(self, name, mutate):
        p = self.folder / name
        data = gzip.decompress(p.read_bytes()) if p.suffix == '.gz' else p.read_bytes()
        value = [json.loads(x) for x in data.splitlines()] if '.jsonl' in name else json.loads(data)
        mutate(value)
        data = b''.join((json.dumps(r) + '\n').encode() for r in value) if '.jsonl' in name else json.dumps(value).encode()
        p.write_bytes(gzip.compress(data, mtime=0) if p.suffix == '.gz' else data)
        self.rehash(name)

    def rehash(self, name):
        p = self.folder / name
        mpath = self.folder / 'manifest.json'
        manifest = json.loads(mpath.read_text())
        row = next(r for r in manifest['artifacts'] if r['path'] == name)
        row.update(sha256=hashlib.sha256(p.read_bytes()).hexdigest(), bytes=p.stat().st_size)
        if 'uncompressed_sha256' in row:
            row['uncompressed_sha256'] = hashlib.sha256(gzip.decompress(p.read_bytes())).hexdigest()
        mpath.write_text(json.dumps(manifest))

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_rehashed_missing_case(self):
        self.edit('contract-results.json', lambda data: data['cases'].pop())
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_false_success(self):
        def mutate(data):
            row = next(r for r in data['cases'] if r['id'] == 'ST-reject-no-publication')
            row.update(expected='accept', observed='accept', passed=True)
        self.edit('contract-results.json', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_design_cannot_claim_public_runtime(self):
        p = self.folder / 'manifest.json'
        data = json.loads(p.read_text())
        data.update(KM02_runtime_complete=True, public_versioned_tools_exposed=True)
        p.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_mutating_probe(self):
        def mutate(rows):
            rows[1]['method'] = 'tools/call'
            rows[1]['params'] = {'name': 'cks.ops.setup', 'arguments': {}}
        self.edit('probe/mcp.stdin.jsonl.gz', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_rehashed_missing_CLI_capability(self):
        p = self.folder / 'probe/setup-help.stderr.txt'
        p.write_bytes(p.read_bytes().replace(b'--hold-for-review', b'--not-this-flag'))
        self.rehash('probe/setup-help.stderr.txt')
        self.assertTrue(guard.check(self.folder))

    def test_historical_proof_cannot_invent_current_source_match(self):
        self.edit('committed-collection-verification.json', lambda data: data.update(matching_current_Go_sources=9999))
        self.assertTrue(guard.check(self.folder))

    def test_failed_gate_cannot_be_hidden(self):
        self.edit('validation.json', lambda data: data['gates'].update(documentation=1))
        self.assertTrue(guard.check(self.folder))

    def test_capability_source_audit_cannot_be_changed(self):
        self.edit('source-audit.json', lambda rows: rows[0].update(span_sha256='0'*64))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
