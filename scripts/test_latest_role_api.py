"""Reject incomplete actual execution evidence and promotion to acceptance."""
import gzip
import json
import tempfile
import unittest
from pathlib import Path
import latest_role_api as x


class Evidence(unittest.TestCase):
    def mutate(self, name, change):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp)
            for source in (x.ROOT/x.PACKET).rglob('*'):
                if source.is_file() and source.name != 'manifest.json':
                    target = p/source.relative_to(x.ROOT/x.PACKET)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.symlink_to(source)
            target = p/name
            value = x.read(target)
            change(value)
            target.unlink()
            raw = json.dumps(value).encode()
            target.write_bytes(gzip.compress(raw, mtime=0) if target.suffix == '.gz' else raw)
            x.seal(p)
            self.assertTrue(x.check(p), name)

    def test_actual_population_and_acceptance_boundary(self):
        self.assertEqual(x.check(), [])
        d = x.derive(x.ROOT/x.PACKET)
        self.assertEqual((d['actual_roles'], d['rpc_negative_cases'], d['mcp_negative_or_uncertain_cases']), (4, 120, 35))
        self.assertFalse(d['installed_positive_WIV1_oracle'])

    def test_old_CKS_cannot_be_relabelled_latest(self):
        self.mutate('native/binary.json.gz', lambda d: d.update(source_revision='49197f0f3d5967913355278ba5d45d4a32ae614a'))

    def test_partial_handoff_not_promoted(self):
        self.mutate('roles/P76-planner/workspace-after/planner-context.json.gz', lambda d: d['actual_tool_response'].update(status='complete'))

    def test_missing_RPC_attempt_not_complete(self):
        self.mutate('rpc/attempts.json.gz', lambda d: d.pop())

    def test_missing_MCP_case_not_complete(self):
        self.mutate('mcp/results.json.gz', lambda d: d.pop(0))

    def test_formal_approval_not_inferred_from_errors(self):
        self.mutate('SUMMARY.json', lambda d: d.update(independent_approved=1, formal_completed=1, full12_requirement_acceptance=True))


if __name__ == '__main__':
    unittest.main()
