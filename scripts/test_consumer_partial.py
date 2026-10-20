#!/usr/bin/env python3
"""Rehashed T10 mutations must not promote partial/unknown into accepted safety."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('t10_check', ROOT / 'scripts/check-consumer-partial.py')
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)


class Partial(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='ks-t10-guard-')
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
        path.write_text(''.join(json.dumps(r) + '\n' for r in value))

    def rejected(self, message=None):
        path = self.p / 'manifest.json'
        m = json.loads(path.read_text())
        m['artifacts'] = {f.relative_to(self.p).as_posix(): c.p.digest(f) for f in self.p.rglob('*') if f.is_file() and f != path}
        path.write_text(json.dumps(m) + '\n')
        errors = c.check(self.p, False)
        self.assertTrue(errors)
        if message:
            self.assertIn(message, str(errors))

    def response(self, case, index, fn):
        def change(rows):
            requests = [json.loads(r['raw']) for r in rows if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call']
            row = next(r for r in rows if r['kind'] == 'response' and json.loads(r['raw']).get('id') == requests[index]['id'])
            value = json.loads(row['raw'])
            fn(value)
            row['raw'] = json.dumps(value) + '\n'
        self.frames(case, change)

    def test_valid(self):
        self.assertEqual(c.check(self.p, False), [])

    def test_matrix_omitted(self):
        self.edit('matrix-cases.json', lambda d: d.pop())
        self.rejected('matrix cases')

    def test_unsupported_promoted(self):
        self.response('T10-matrix', 0, lambda d: d['result']['structuredContent'].update(status='partial'))
        self.rejected('actual unsupported result')

    def test_truncated_erased(self):
        self.response('T10-matrix', 1, lambda d: d['result']['structuredContent']['analysis'].update(truncated=False))
        self.rejected('actual truncated result')

    def test_incomplete_rules_promoted(self):
        self.response('T10-matrix', 2, lambda d: d['result']['structuredContent']['analysis']['rules'].update(status='matched_bounded'))
        self.rejected('actual incomplete rules')

    def test_requires_reindex_error_erased(self):
        self.response('T10-legacy', 1, lambda d: d['result'].update(isError=False))
        self.rejected('actual requires_reindex omitted')

    def test_requires_reindex_host_error_hidden(self):
        self.edit('hosts/T10-legacy/gateway-requests.json', lambda d: [r.update(is_error=False) for row in d for m in row['body']['messages'] if m['role'] == 'user' for r in m.get('content', []) if isinstance(r, dict) and r.get('tool_use_id') == 'fixture_tool_1'])
        self.rejected('host did not receive legacy error')

    def test_actual_response_mislabelled_as_mutation(self):
        self.edit('controls/incomplete-rules/mutated.json', lambda d: d.update(origin='local_verifier_mutation'))
        self.rejected('synthetic mutation mislabelled')

    def test_synthetic_promoted_to_measurement(self):
        self.edit('controls/truncated-flag/mutated.json', lambda d: d.update(origin='actual_MCP_receipt'))
        self.rejected('synthetic mutation mislabelled')

    def test_control_mutant_not_bound_to_response(self):
        self.edit('controls/unsupported/mutated.json', lambda d: d['workspace']['deep-review-evidence.json'].update(sha256='0' * 64))
        self.edit('controls/unsupported/after-attempt.json', lambda d: d['deep-review-evidence.json'].update(sha256='0' * 64))
        self.rejected('mutated bytes not derived from actual receipt/control')

    def test_cli_placeholder_refusal_erased(self):
        self.edit('controls/all-unknown/mutated.json', lambda d: d['verification'].update(exit_code=0))
        self.rejected('CLI refusal differs')

    def test_refusal_changed_delivery(self):
        self.edit('controls/missing-section/after-attempt.json', lambda d: d['state.json']['json'].update(current_state='COMPLETED'))
        self.rejected('refusal modified delivery or restore failed')

    def test_restore_not_identical(self):
        self.edit('controls/unknown-empty/restore.json', lambda d: d['workspace']['state.json'].update(sha256='0' * 64))
        self.rejected('refusal modified delivery or restore failed')

    def test_host_stale_read_not_hook_proof(self):
        self.edit('hosts/T10-controls/gateway-requests.json', lambda d: [r.update(content='File has been modified since read') for row in d for m in row['body']['messages'] if m['role'] == 'user' for r in m.get('content', []) if isinstance(r, dict) and r.get('tool_use_id') == 'fixture_tool_3'])
        self.rejected('actual hook refusal differs')

    def test_positive_delivery_failed(self):
        self.edit('hosts/T10-field-delivery/gateway-requests.json', lambda d: [r.update(is_error=True) for row in d for m in row['body']['messages'] if m['role'] == 'user' for r in m.get('content', []) if isinstance(r, dict) and r.get('tool_use_id') == 'fixture_tool_2'])
        self.rejected('Field positive delivery failed')

    def test_partial_blanket_failure_claim(self):
        self.edit('partial.json', lambda d: d.update(partial_is_blanket_failure=True))
        self.rejected('derived T10 accounting/scope')

    def test_empty_candidates_as_safety(self):
        self.edit('partial.json', lambda d: d.update(empty_or_unknown_is_safety_or_missing0=True))
        self.rejected('derived T10 accounting/scope')

    def test_semantic_quality_claim(self):
        self.edit('partial.json', lambda d: d.update(semantic_safety_or_completeness_evaluated=True))
        self.rejected('derived T10 accounting/scope')

    def test_ontology_claim(self):
        self.edit('partial.json', lambda d: d.update(ontology_effect_acceptance='accepted'))
        self.rejected('derived T10 accounting/scope')

    def test_unknown_count_promoted(self):
        self.edit('partial.json', lambda d: d.update(unknown_sections_per_review=0))
        self.rejected('derived T10 accounting/scope')

    def test_wrong_driver(self):
        self.edit('execution.json', lambda d: d.update(driver_sha256='0' * 64))
        self.rejected('executed driver/plan/helper pin')

    def test_adoption(self):
        self.edit('execution.json', lambda d: d.update(candidate_adopted=True))
        self.rejected('execution scope')

    def test_formal_execution(self):
        self.edit('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state changed')

    def test_wrong_role(self):
        self.edit('hosts/T10-controls/invocation.json', lambda d: d.update(role='analyzer'))
        self.rejected('actual role/helper identity')

    def test_nonlocal_provider(self):
        self.edit('hosts/T10-controls/invocation.json', lambda d: d['env'].update(ANTHROPIC_BASE_URL='https://example.invalid'))
        self.rejected('nonlocal model provider')

    def test_host_timeout(self):
        self.edit('hosts/T10-build-seed/exit.json', lambda d: d.update(forced=True))
        self.rejected('host cleanup')

    def test_CKS_exit_missing(self):
        self.frames('T10-legacy', lambda d: d.__setitem__(slice(None), [r for r in d if r['kind'] != 'exit']))
        self.rejected('CKS child identity')

    def test_CKS_not_reaped(self):
        self.frames('T10-matrix', lambda d: [r.update(child_reaped=False) for r in d if r['kind'] == 'exit'])
        self.rejected('CKS exit/reap proof')

    def test_gateway_leak(self):
        self.edit('hosts/T10-controls/exit.json', lambda d: d['gateway_cleanup'].update(all_handler_threads_joined=False))
        self.rejected('gateway cleanup')

    def test_extra_MCP_query(self):
        self.frames('T10-build-field', lambda d: d.append(next(r for r in d if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call')))
        self.rejected('CKS request denominator')

    def test_schema_fault_as_operational_outage(self):
        self.edit('legacy-index-control.json', lambda d: d.update(owned_schema_fault_not_operational_outage=False))
        self.rejected('owned schema fault binding')

    def test_database_tamper(self):
        p = self.p / 'indexes/legacy/graph.db'
        p.write_bytes(p.read_bytes() + b'changed')
        self.rejected('owned schema fault binding')

    def test_prior_inventory_omitted(self):
        self.edit('preservation.json', lambda d: d['files'].pop(next(iter(d['files']))))
        self.rejected('prior33 inventory')

    def test_live_PID(self):
        self.edit('postrun-process-audit.json', lambda d: d['probes'][0].update(exit_code=0, stdout='123'))
        self.rejected('owned process cleanup audit')

    def test_audit_failed_open(self):
        self.edit('postrun-process-audit.json', lambda d: d['self'].update(exit_code=1, stdout=''))
        self.rejected('owned process cleanup audit')

    def test_audit_PID_omitted(self):
        self.edit('postrun-process-audit.json', lambda d: d['probes'].pop())
        self.rejected('owned PID audit denominator')

    def test_extra_profile(self):
        path = self.p / 'hosts/T10-matrix/claude-config'
        path.mkdir()
        (path / 'settings.json').write_text('{}')
        self.rejected('artifact inventory')

    def test_report_hides_unknown_scope(self):
        (self.p / 'REPORT.txt').write_text('T10 pass')
        self.rejected('report omitted partial/unknown scope')


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
