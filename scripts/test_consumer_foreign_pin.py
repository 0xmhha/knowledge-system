#!/usr/bin/env python3
"""T09 evidence mutations must not hide mixed sources or failed delivery guards."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('t09_check', ROOT / 'scripts/check-consumer-foreign-pin.py')
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)


class ForeignPin(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='ks-t09-guard-')
        self.p = Path(self.tmp.name) / 'packet'
        shutil.copytree(ROOT / c.p.PACKET, self.p)

    def tearDown(self):
        self.tmp.cleanup()

    def edit(self, name, fn):
        path = self.p / name
        value = json.loads(path.read_text())
        fn(value)
        path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

    def frames(self, case, fn):
        path = self.p / 'hosts' / case / 'mcp-frames.jsonl'
        value = c.p.lines(path)
        fn(value)
        path.write_text(''.join(json.dumps(row) + '\n' for row in value))

    def rejected(self, message=None):
        path = self.p / 'manifest.json'
        m = json.loads(path.read_text())
        m['artifacts'] = {f.relative_to(self.p).as_posix(): c.p.digest(f) for f in self.p.rglob('*') if f.is_file() and f != path}
        path.write_text(json.dumps(m) + '\n')
        errors = c.check(self.p, False)
        self.assertTrue(errors)
        if message:
            self.assertIn(message, str(errors))

    def test_valid(self):
        self.assertEqual(c.check(self.p, False), [])

    def test_matrix_case_omitted(self):
        self.edit('matrix-cases.json', lambda d: d.pop())
        self.rejected('matrix expectation inputs')

    def test_collision_as_foreign_refusal(self):
        self.edit('matrix-cases.json', lambda d: d[4].update(expected='not_found'))
        self.rejected('matrix expectation inputs')

    def test_projects_same_pin(self):
        self.edit('execution.json', lambda d: d['foreign'].update(expected_commit=d['query']['expected_commit']))
        self.rejected('projects not distinct')

    def test_global_ID_claim(self):
        self.edit('foreign-pin.json', lambda d: d.update(node_IDs_globally_unique=True))
        self.rejected('derived T09 accounting/scope')

    def test_structural_pin_claim(self):
        self.edit('foreign-pin.json', lambda d: d.update(structural_expected_pins_supported=True))
        self.rejected('derived T09 accounting/scope')

    def test_tenant_authentication_claim(self):
        self.edit('foreign-pin.json', lambda d: d.update(tenant_ACL_or_transport_authentication_proven=True))
        self.rejected('derived T09 accounting/scope')

    def test_wrong_driver_pin(self):
        self.edit('execution.json', lambda d: d.update(driver_sha256='0' * 64))
        self.rejected('executed driver/plan/helper pin')

    def test_actual_host_role(self):
        self.edit('hosts/T09-delivery/invocation.json', lambda d: d.update(role='analyzer'))
        self.rejected('actual host/helper identity')

    def test_host_timeout(self):
        self.edit('hosts/T09-matrix/exit.json', lambda d: d.update(forced=True))
        self.rejected('host cleanup')

    def test_CKS_exit_missing(self):
        self.frames('T09-build-A', lambda d: d.__setitem__(slice(None), [r for r in d if r['kind'] != 'exit']))
        self.rejected('CKS child identity')

    def test_CKS_not_reaped(self):
        self.frames('T09-build-B', lambda d: [r.update(child_reaped=False) for r in d if r['kind'] == 'exit'])
        self.rejected('CKS exit/reap')

    def test_gateway_leak(self):
        self.edit('hosts/T09-matrix/exit.json', lambda d: d['gateway_cleanup'].update(server_socket_closed=False))
        self.rejected('gateway cleanup')

    def test_nonlocal_provider(self):
        self.edit('hosts/T09-matrix/invocation.json', lambda d: d['env'].update(ANTHROPIC_BASE_URL='https://example.invalid'))
        self.rejected('nonlocal provider')

    def test_extra_actual_query(self):
        self.frames('T09-matrix', lambda d: d.append(next(r for r in d if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call')))
        self.rejected('actual CKS call denominator')

    def test_refusal_promoted_to_positive(self):
        def change(rows):
            request_ids = [json.loads(r['raw'])['id'] for r in rows if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call']
            row = next(r for r in rows if r['kind'] == 'response' and json.loads(r['raw']).get('id') == request_ids[2])
            response = json.loads(row['raw'])
            response['result']['structuredContent']['status'] = 'partial'
            row['raw'] = json.dumps(response) + '\n'
        self.frames('T09-matrix', change)
        self.rejected('matrix refusal differs')

    def test_refusal_leaks_analysis(self):
        def change(rows):
            request_ids = [json.loads(r['raw'])['id'] for r in rows if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call']
            row = next(r for r in rows if r['kind'] == 'response' and json.loads(r['raw']).get('id') == request_ids[2])
            response = json.loads(row['raw'])
            response['result']['structuredContent']['analysis'] = {'foreign': True}
            row['raw'] = json.dumps(response) + '\n'
        self.frames('T09-matrix', change)
        self.rejected('refusal leaked usable evidence')

    def test_cli_refusal_as_success(self):
        self.edit('controls/commit/mutated.json', lambda d: d['verification'].update(exit_code=0))
        self.rejected('consumer CLI mismatch refusal')

    def test_denied_write_changed_state(self):
        self.edit('controls/source/after-attempt.json', lambda d: d['state.json']['json'].update(current_state='COMPLETED'))
        self.rejected('denied Write still changed workspace')

    def test_restore_not_identical(self):
        self.edit('controls/file/restore.json', lambda d: d['workspace']['state.json'].update(sha256='0' * 64))
        self.rejected('control restore failed')

    def test_ticket_stale_read_is_not_hook_proof(self):
        self.edit('hosts/T09-delivery/gateway-requests.json', lambda d: [r.update(content='File has been modified since read') for row in d for m in row['body']['messages'] if m['role'] == 'user' for r in m.get('content', []) if isinstance(r, dict) and r.get('tool_use_id') == 'fixture_tool_27'])
        self.rejected('actual Write hook did not refuse expected mismatch')

    def test_rebind_refusal_hidden(self):
        self.edit('hosts/T09-delivery/gateway-requests.json', lambda d: [r.update(is_error=False) for row in d for m in row['body']['messages'] if m['role'] == 'user' for r in m.get('content', []) if isinstance(r, dict) and r.get('tool_use_id') == 'fixture_tool_32'])
        self.rejected('foreign frozen rebind allowed')

    def test_final_positive_failed(self):
        self.edit('hosts/T09-delivery/gateway-requests.json', lambda d: [r.update(is_error=True) for row in d for m in row['body']['messages'] if m['role'] == 'user' for r in m.get('content', []) if isinstance(r, dict) and r.get('tool_use_id') == 'fixture_tool_35'])
        self.rejected('genuine A completion blocked')

    def test_initial_failure_hidden(self):
        self.edit('diagnostics/summary.json', lambda d: d.update(initial_full_run_not_T09_pass=False))
        self.rejected('diagnostic failures hidden')

    def test_initial_node_allow_hidden(self):
        self.edit('diagnostics/integration/controls/node/mutated.json', lambda d: d['verification'].update(exit_code=2))
        self.rejected('initial node no-op validation hidden')

    def test_initial_discovery_failure_hidden(self):
        (self.p / 'diagnostics/discovery-controller.stderr.txt').write_text('success')
        self.rejected('initial controller failure omitted')

    def test_foreign_source_forged(self):
        p = self.p / 'project-B-source/sources/logic.go.txt'
        p.write_bytes(p.read_bytes() + b'// changed\n')
        self.rejected('commit-to-source binding mismatch')

    def test_foreign_database_changed(self):
        p = self.p / 'indexes/B/graph.db'
        p.write_bytes(p.read_bytes() + b'changed')
        self.rejected('B index hash')

    def test_native_binary_pin_changed(self):
        self.edit('integrity.json', lambda d: d['binary_sha256'].update(cks='0' * 64))
        self.rejected('native executable pins')

    def test_prior_inventory_omitted(self):
        self.edit('preservation.json', lambda d: d['files'].pop(next(iter(d['files']))))
        self.rejected('prior32 inventory')

    def test_live_PID(self):
        self.edit('postrun-process-audit.json', lambda d: d['probes'][0].update(exit_code=0, stdout='123'))
        self.rejected('owned cleanup audit')

    def test_audit_failed_open(self):
        self.edit('postrun-process-audit.json', lambda d: d['self'].update(exit_code=1, stdout=''))
        self.rejected('owned cleanup audit')

    def test_audit_PID_omitted(self):
        self.edit('postrun-process-audit.json', lambda d: d['probes'].pop())
        self.rejected('owned PID audit denominator')

    def test_formal_execution(self):
        self.edit('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state changed')

    def test_adoption(self):
        self.edit('execution.json', lambda d: d.update(candidate_adopted=True))
        self.rejected('execution scope')

    def test_ontology_claim(self):
        self.edit('foreign-pin.json', lambda d: d.update(ontology_effect_acceptance='accepted'))
        self.rejected('derived T09 accounting/scope')

    def test_extra_profile(self):
        p = self.p / 'hosts/T09-matrix/claude-config'
        p.mkdir()
        (p / 'settings.json').write_text('{}')
        self.rejected('artifact inventory')

    def test_report_hides_collision(self):
        (self.p / 'REPORT.txt').write_text('T09 pass')
        self.rejected('report hides collision/failures/scope')


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
