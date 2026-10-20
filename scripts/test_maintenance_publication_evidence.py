"""Reject false publication lineage and inflated private/public completion."""
import copy
import gzip
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import unittest


def load(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = load('publication_checkpoint_guard', 'check-maintenance-publication-evidence.py')
base = load('publication_public_proof_guards', 'test_maintenance_runtime_evidence.py')
base.guard.PACKET = guard.PACKET


class PublicationEvidenceTests(base.MaintenanceRuntimeEvidenceTests):
    def edit(self, name, mutate):
        if not (self.folder / name).exists() and (self.folder / (name + '.gz')).exists():
            name += '.gz'
        return super().edit(name, mutate)

    def test_C_scope_and_actual_public_history(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_C_cannot_claim_D_or_public_completion(self):
        path = self.folder / 'manifest.json'
        data = json.loads(path.read_text())
        data.update(pre_gate_child_termination_verified=True, public_durable_connected=True)
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_synthetic_history_cannot_claim_real_DB_publication(self):
        path = self.folder / 'manifest.json'
        data = json.loads(path.read_text())
        data['synthetic_history_is_real_publication'] = True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_restamped_foreign_publication_history_is_rejected(self):
        path = self.folder / 'fixture.tar.gz'
        stream = io.BytesIO()
        changed = False
        with tarfile.open(fileobj=io.BytesIO(gzip.decompress(path.read_bytes()))) as original, tarfile.open(fileobj=stream, mode='w') as rewritten:
            for item in original:
                header = copy.copy(item)
                if item.isfile():
                    raw = original.extractfile(item).read()
                    if not changed and '/.maintenance-publications/' in item.name and item.name.endswith('.json'):
                        data = json.loads(raw)
                        data['proof']['project_id'] = 'project-b'
                        body = json.dumps(data['proof'], ensure_ascii=False, separators=(',', ':')).encode()
                        data['checksum'] = hashlib.sha256(b'cks.managed-publication.v1\0' + len(body).to_bytes(8, 'big') + body).hexdigest()
                        raw = json.dumps(data).encode()
                        changed = True
                    header.size = len(raw)
                    rewritten.addfile(header, io.BytesIO(raw))
                else:
                    rewritten.addfile(header)
        self.assertTrue(changed)
        path.write_bytes(gzip.compress(stream.getvalue(), mtime=0))
        self.rehash('fixture.tar.gz')
        self.assertTrue(guard.check(self.folder))


if __name__ == '__main__':
    unittest.main()
