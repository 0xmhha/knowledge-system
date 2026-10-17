#!/usr/bin/env python3
"""Rehashed mutations must not turn extra computation or failed reuse into done."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('t04_check', ROOT / 'scripts/check-consumer-opt-in.py')
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)


class OptIn(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='ks-t04-guard-')
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
        value = [json.loads(line) for line in path.read_text().splitlines()]
        fn(value)
        path.write_text(''.join(json.dumps(row) + '\n' for row in value))

    def rejected(self, message=None):
        path = self.p / 'manifest.json'
        manifest = json.loads(path.read_text())
        manifest['artifacts'] = {f.relative_to(self.p).as_posix(): c.p.digest(f) for f in self.p.rglob('*') if f.is_file() and f != path}
        path.write_text(json.dumps(manifest) + '\n')
        errors = c.check(self.p, False)
        self.assertTrue(errors)
        if message:
            self.assertIn(message, str(errors))

    def test_valid(self):
        self.assertEqual(c.check(self.p, False), [])

    def test_native_opt_in_accepted(self):
        self.edit('native/messages.json', lambda d: d[1].update(response=d[6]['response']))
        self.rejected('invalid opt-in accepted')

    def test_native_positive_error(self):
        self.edit('native/messages.json', lambda d: d[6].update(response=d[1]['response']))
        self.rejected('positive native error')

    def test_native_case_omitted(self):
        self.edit('native/messages.json', lambda d: d.pop())
        self.rejected('native cases/denominator')

    def test_native_invalid_input_substituted(self):
        self.edit('native/messages.json', lambda d: d[1]['request']['params']['arguments'].update(rule_pack='go-local-review@1.0.0'))
        self.rejected('native invalid controls not actual missing/unknown inputs')

    def test_rule_contract_leaks_to_other_mode(self):
        ns, text = c.p.consumer()
        def changed(name):
            raw = text(name)
            return raw.replace('\n## 7.', '\nExecute go-local-review@1.0.0 globally.\n## 7.', 1) if name.endswith('analyzer.md') else raw
        with patch.object(c.p, 'consumer', return_value=(ns, changed)):
            self.rejected('rule execution contract leaked to other modes')

    def test_wrong_mode_dispatch(self):
        ns, text = c.p.consumer()
        def changed(name):
            raw = text(name)
            return '\n'.join(line.replace('§7', '§9') if line.startswith('|') and 'code_review' in line else line for line in raw.split('\n')) if name.endswith('planner.md') else raw
        with patch.object(c.p, 'consumer', return_value=(ns, changed)):
            self.rejected('mode dispatch does not select structured review')

    def test_native_wire_omitted(self):
        (self.p / 'native/stdin.jsonl').write_text('')
        self.rejected()

    def test_native_not_reaped(self):
        self.edit('native/lifecycle.json', lambda d: d.update(child_reaped=False))
        self.rejected('native cleanup')

    def test_native_forced(self):
        self.edit('native/lifecycle.json', lambda d: d.update(kill_sent=True))
        self.rejected('native cleanup')

    def test_wrong_role(self):
        self.edit('hosts/planner-reuse/invocation.json', lambda d: d.update(role='analyzer'))
        self.rejected('actual role identity')

    def test_wrong_executed_driver(self):
        self.edit('hosts/review-build/invocation.json', lambda d: d.update(driver_sha256='0' * 64))
        self.rejected('host/helper/binary pins')

    def test_nonlocal_provider(self):
        self.edit('hosts/review-build/invocation.json', lambda d: d['env'].update(ANTHROPIC_BASE_URL='https://example.invalid'))
        self.rejected('nonlocal provider')

    def test_host_timeout_hidden(self):
        self.edit('hosts/analyzer-reuse/exit.json', lambda d: d.update(forced=True))
        self.rejected('host cleanup failure')

    def test_CKS_exit_missing(self):
        self.frames('planner-reuse', lambda d: d.__setitem__(slice(None), [r for r in d if r['kind'] != 'exit']))
        self.rejected('CKS denominator/pin')

    def test_CKS_not_reaped(self):
        self.frames('planner-reuse', lambda d: [r.update(child_reaped=False) for r in d if r['kind'] == 'exit'])
        self.rejected('CKS exit/reap proof')

    def test_signal_worker_leak(self):
        self.frames('feature-default', lambda d: [r.update(signal_worker_joined=False) for r in d if r['kind'] == 'exit'])
        self.rejected('CKS exit/reap proof')

    def test_responder_leak(self):
        self.edit('hosts/review-build/exit.json', lambda d: d['gateway_cleanup'].update(all_handler_threads_joined=False))
        self.rejected('responder cleanup failure')

    def test_extra_query_on_reentry(self):
        self.frames('analyzer-reuse', lambda d: d.append(next(r for r in c.p.lines(self.p / 'hosts/feature-value/mcp-frames.jsonl') if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call')))
        self.rejected('extra/missing CKS requests')

    def test_automatic_rule_in_nonreview(self):
        self.frames('feature-default', lambda d: d.append(next(r for r in c.p.lines(self.p / 'hosts/review-build/mcp-frames.jsonl') if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call')))
        self.rejected('extra/missing CKS requests')

    def test_declared_reentry_rebuild(self):
        self.edit('hosts/analyzer-reuse/scripted-actions.json', lambda d: d[1]['input'].update(command='fake build'))
        self.rejected('scripted/API action differs')

    def test_reused_diagram_changed(self):
        path = self.p / 'hosts/analyzer-reuse/workspace-after/sequence.mmd'
        path.write_text(path.read_text() + ' extra-->graph\n')
        digest = c.p.digest(path)
        self.edit('hosts/analyzer-reuse/workspace-after.json', lambda d: d['sequence.mmd'].update(sha256=digest))
        self.rejected('diagram differs')

    def test_consumer_graph_changed(self):
        name = 'hosts/review-build/workspace-after/deep-review.json'
        self.edit(name, lambda d: d['computed']['module'].update(file='different.go'))
        digest = c.p.digest(self.p / name)
        self.edit('hosts/review-build/workspace-after.json', lambda d: d['deep-review.json'].update(sha256=digest))
        self.rejected()

    def test_duplicate_attempt_erased(self):
        def erase(rows):
            index = next(i for i, row in enumerate(rows) if row['kind'] == 'request' and json.loads(row['raw']).get('method') == 'tools/call')
            rows.pop(index)
        self.frames('duplicate-control', erase)
        self.rejected('extra/missing CKS requests')

    def test_duplicate_refusal_erased(self):
        self.edit('hosts/duplicate-control/gateway-requests.json', lambda d: [v.update(is_error=False) for r in d for m in r['body'].get('messages', []) if m['role'] == 'user' for v in m.get('content', []) if isinstance(v, dict) and v.get('tool_use_id') == 'fixture_tool_3'])
        self.rejected('fixture adapter duplicate refusal missing')

    def test_false_duplicate_blocker_claim(self):
        self.edit('opt-in.json', lambda d: d.update(server_cache_or_duplicate_blocker_verified=True))
        self.rejected('derived T04 accounting/scope')

    def test_false_model_adherence_claim(self):
        self.edit('opt-in.json', lambda d: d.update(autonomous_model_adherence='verified'))
        self.rejected('derived T04 accounting/scope')

    def test_false_ontology_claim(self):
        self.edit('opt-in.json', lambda d: d.update(ontology_effect_acceptance='accepted'))
        self.rejected('derived T04 accounting/scope')

    def test_formal_execution(self):
        self.edit('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state changed')

    def test_adoption(self):
        self.edit('execution.json', lambda d: d.update(candidate_adopted=True))
        self.rejected('execution/adoption scope')

    def test_prior_packet_inventory_omitted(self):
        self.edit('preservation.json', lambda d: d['files'].pop(next(iter(d['files']))))
        self.rejected('prior packet inventory')

    def test_native_source_tampered(self):
        path = self.p / 'native-source/sources/pkg/graph/valueflow/rules.go.txt'
        path.write_bytes(path.read_bytes() + b'// changed\n')
        self.rejected('commit-to-source binding mismatch')

    def test_source_pin_changed(self):
        self.edit('source-integrity.json', lambda d: d['checkouts']['consumer_postrun'].update(commit='0' * 40))
        self.rejected('candidate/original source pin')

    def test_live_PID(self):
        self.edit('postrun-process-audit.json', lambda d: d['probes'][0].update(exit_code=0, stdout='123'))
        self.rejected('owned process still present')

    def test_audit_failed_open(self):
        self.edit('postrun-process-audit.json', lambda d: d['self'].update(exit_code=1, stdout=''))
        self.rejected('audit live positive control')

    def test_audit_PID_omitted(self):
        self.edit('postrun-process-audit.json', lambda d: d['probes'].pop())
        self.rejected('owned PID audit denominator')

    def test_extra_profile(self):
        path = self.p / 'hosts/feature-default/claude-config'
        path.mkdir()
        (path / 'settings.json').write_text('{}')
        self.rejected('artifact inventory')

    def test_failure_limit_removed_from_report(self):
        (self.p / 'REPORT.txt').write_text('T04 passed')
        self.rejected('report omits limits/failure')


if __name__ == '__main__':
    unittest.main()
