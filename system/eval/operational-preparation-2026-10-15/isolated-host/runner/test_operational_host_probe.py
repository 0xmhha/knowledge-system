#!/usr/bin/env python3
"""Verifier regression tests use synthetic records, never host acceptance data."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('probe', Path(__file__).with_name('operational-host-probe.py'))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name)
        self.exit = {'case': 'valid', 'forced_host_termination': False, 'local_remote_refs': '',
                     'state_bytes_unchanged': True, 'host_exit': 0, 'receipt_check_exit': 0}
        self.events = [{'subtype': 'init', 'mcp_servers': [{'name': 'plugin_coding-agent_chainbench', 'status': 'connected', 'source': 'dynamic'}],
                        'tools': ['Bash', 'Read', 'Write', 'mcp__plugin_coding-agent_chainbench__chainbench_chains'],
                        'plugins': [{'name': 'coding-agent', 'path': '/owned/plugin', 'source': 'coding-agent@inline'}],
                        'claude_code_version': 'synthetic-unit-fixture'},
                       {'subtype': 'hook_response', 'hook_event': 'SessionStart', 'exit_code': 0,
                        'output': 'Local evaluation boundary markers'},
                       {'subtype': 'hook_response', 'hook_event': 'Stop', 'exit_code': 0, 'output': ''}]
        self.inputs = [{'payload': {'hook_event_name': 'Stop', 'stop_hook_active': False}}]
        self.frames = [{'kind': 'start'}, {'kind': 'request', 'raw': json.dumps({'id': 1, 'method': 'tools/list'})},
                       {'kind': 'response', 'raw': json.dumps({'id': 1, 'result': {'tools': [{'name': str(i)} for i in range(81)]}})},
                       {'kind': 'exit', 'exit_code': 0, 'forced_kill': False}]
        self.invocation = {'argv': ['--plugin-dir', '--strict-mcp-config', '--setting-sources', '--permission-prompts', '--include-hook-events'],
                           'env': {'ANTHROPIC_BASE_URL': 'http://127.0.0.1:1234'}}

    def check(self):
        for name, value in [('exit.json', self.exit), ('invocation.json', self.invocation), ('gateway-requests.json', [])]:
            (self.path/name).write_text(json.dumps(value))
        for name, value in [('host.stdout.jsonl', self.events), ('mcp-frames.jsonl', self.frames), ('hook-inputs.jsonl', self.inputs)]:
            (self.path/name).write_text(''.join(json.dumps(row)+'\n' for row in value))
        return probe.assess(self.path)

    def test_complete_typed_shape_is_local_only(self):
        result = self.check()
        self.assertEqual(result['status'], 'PASS')
        self.assertEqual(result['agent_pipeline_acceptance'], 'not_run')
        self.assertEqual(result['real_model_inference'], 'not_run')

    def test_exit_zero_without_actual_hooks_is_insufficient(self):
        self.events = self.events[:1]
        self.assertEqual(self.check()['status'], 'FAIL')

    def test_missing_receipt_or_forced_shutdown_cannot_pass(self):
        self.exit['receipt_check_exit'] = 2
        self.assertEqual(self.check()['status'], 'FAIL')
        self.exit['receipt_check_exit'] = 0
        self.exit['forced_host_termination'] = True
        self.assertEqual(self.check()['status'], 'FAIL')

    def test_wrong_alias_disconnected_server_and_other_plugins_fail(self):
        original = copy.deepcopy(self.events)
        self.events[0]['mcp_servers'][0]['status'] = 'failed'
        self.assertEqual(self.check()['status'], 'FAIL')
        self.events = copy.deepcopy(original)
        self.events[0]['tools'].append('mcp__other__write')
        self.assertEqual(self.check()['status'], 'FAIL')
        self.events = copy.deepcopy(original)
        self.events[0]['plugins'].append({'name': 'other', 'path': '/unowned/plugin', 'source': 'external'})
        self.assertEqual(self.check()['status'], 'FAIL')

    def test_evaluated_must_not_receive_resume_instruction(self):
        self.events[1]['output'] += ' resume with /coding-agent:work'
        self.assertEqual(self.check()['status'], 'FAIL')

    def test_incomplete_cannot_be_counted_as_valid_receipt(self):
        self.exit['case'] = 'incomplete'
        self.inputs.append({'payload': {'hook_event_name': 'Stop', 'stop_hook_active': True}})
        self.events[2]['output'] = json.dumps({'decision': 'block'})
        self.assertEqual(self.check()['status'], 'FAIL')
        self.exit['receipt_check_exit'] = 2
        self.assertEqual(self.check()['status'], 'PASS')
        self.inputs.append({'payload': {'hook_event_name': 'Stop', 'stop_hook_active': True}})
        self.assertEqual(self.check()['status'], 'FAIL')

    def test_killed_mcp_nonlocal_transport_and_bypass_fail(self):
        self.frames[-1]['exit_code'] = -9
        self.assertEqual(self.check()['status'], 'FAIL')
        self.frames[-1]['exit_code'] = 0
        self.invocation['env']['ANTHROPIC_BASE_URL'] = 'https://external.test'
        self.assertEqual(self.check()['status'], 'FAIL')
        self.invocation['env']['ANTHROPIC_BASE_URL'] = 'http://127.0.0.1:1234'
        self.invocation['argv'].append('--bare')
        self.assertEqual(self.check()['status'], 'FAIL')

    def test_missing_files_fail_closed(self):
        self.assertEqual(probe.assess(self.path)['status'], 'FAIL')


if __name__ == '__main__':
    unittest.main()
