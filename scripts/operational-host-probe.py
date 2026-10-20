#!/usr/bin/env python3
"""Actual Claude Code + adopted plugin/MCP, with a local scripted API fixture.

This is readiness integration, never production model/agent quality acceptance.
Only an owned fixture repo, local bare remote and chain metadata are exercised.
"""
import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import signal
import shlex
import subprocess
import sys
import threading
import time


def write(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def proxy(args):
    """Transparent stdio proxy: record actual frames and owned child exit/signals."""
    log = args.log
    log.parent.mkdir(parents=True, exist_ok=True)
    lock = threading.Lock()
    def record(kind, **data):
        with lock, log.open('a') as stream:
            stream.write(json.dumps({'kind': kind, 'time': time.time(), **data}) + '\n')
    child = subprocess.Popen([str(args.binary)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, start_new_session=True)
    record('start', wrapper_pid=os.getpid(), child_pid=child.pid, binary=str(args.binary), binary_sha256=sha(args.binary))
    def forward_signal(number, _frame):
        record('signal', signal=number)
        if child.poll() is None:
            child.send_signal(number)
    for number in (signal.SIGTERM, signal.SIGINT):
        signal.signal(number, forward_signal)
    def input_loop():
        try:
            for raw in sys.stdin.buffer:
                record('request', raw=raw.decode('utf-8', errors='replace'))
                child.stdin.write(raw); child.stdin.flush()
        except (BrokenPipeError, ValueError):
            pass
        finally:
            record('stdin_eof')
            try: child.stdin.close()
            except OSError: pass
    def output_loop():
        try:
            for raw in child.stdout:
                record('response', raw=raw.decode('utf-8', errors='replace'))
                sys.stdout.buffer.write(raw); sys.stdout.buffer.flush()
        except BrokenPipeError:
            record('output_closed')
            if child.poll() is None: child.send_signal(signal.SIGTERM)
    def error_loop():
        for raw in child.stderr:
            record('stderr', raw=raw.decode('utf-8', errors='replace'))
    threads = [threading.Thread(target=fn, daemon=True) for fn in (input_loop, output_loop, error_loop)]
    for thread in threads: thread.start()
    code = child.wait()
    for thread in threads[1:]: thread.join(timeout=2)
    record('exit', child_pid=child.pid, exit_code=code, forced_kill=False)
    return code


class Gateway:
    def __init__(self, case, actions, out):
        self.case, self.actions, self.out = case, actions, out
        self.requests = []
        self.main_requests = 0
        self.received = threading.Event()
        owner = self
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_args): pass
            def do_GET(self):
                self.respond({'data': [{'id': 'claude-sonnet-4-6', 'type': 'model', 'display_name': 'Local transport fixture'}]})
            def respond(self, value):
                raw = json.dumps(value).encode()
                self.send_response(200); self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw))); self.end_headers(); self.wfile.write(raw)
            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers.get('Content-Length', '0'))))
                row = {'path': self.path, 'body': body}
                owner.requests.append(row)
                write(out / 'gateway-requests.json', owner.requests)
                if self.path.endswith('/count_tokens'):
                    self.respond({'input_tokens': 1}); return
                if '/messages' not in self.path:
                    self.send_error(404); return
                main = f'KS_HOST_PROBE:{owner.case}' in json.dumps(body.get('messages', []))
                if main:
                    owner.main_requests += 1
                    owner.received.set()
                    if owner.case == 'cancel': time.sleep(5)
                index = owner.main_requests - 1
                action = owner.actions[index] if main and index < len(owner.actions) else None
                content = ([{'type': 'tool_use', 'id': f'fixture_tool_{index}', **action}]
                           if action else [{'type': 'text', 'text': 'Local transport fixture response; no model or independent acceptance.'}])
                message = {'id': f'msg_fixture_{len(owner.requests)}', 'type': 'message', 'role': 'assistant',
                           'model': body.get('model', 'claude-sonnet-4-6'), 'content': content,
                           'stop_reason': 'tool_use' if action else 'end_turn', 'stop_sequence': None,
                           'usage': {'input_tokens': 1, 'output_tokens': 1}}
                row['scripted_response'] = message
                write(out / 'gateway-requests.json', owner.requests)
                if not body.get('stream'):
                    self.respond(message); return
                self.send_response(200); self.send_header('Content-Type', 'text/event-stream'); self.end_headers()
                def event(kind, data):
                    self.wfile.write(('event: '+kind+'\ndata: '+json.dumps(data)+'\n\n').encode()); self.wfile.flush()
                initial = {**message, 'content': [], 'stop_reason': None, 'usage': {'input_tokens': 1, 'output_tokens': 0}}
                try:
                    event('message_start', {'type': 'message_start', 'message': initial})
                    for i, block in enumerate(content):
                        start = {**block, 'input': {}} if block['type'] == 'tool_use' else {'type': 'text', 'text': ''}
                        event('content_block_start', {'type': 'content_block_start', 'index': i, 'content_block': start})
                        delta = ({'type': 'input_json_delta', 'partial_json': json.dumps(block['input'])}
                                 if block['type'] == 'tool_use' else {'type': 'text_delta', 'text': block['text']})
                        event('content_block_delta', {'type': 'content_block_delta', 'index': i, 'delta': delta})
                        event('content_block_stop', {'type': 'content_block_stop', 'index': i})
                    event('message_delta', {'type': 'message_delta', 'delta': {'stop_reason': message['stop_reason'], 'stop_sequence': None}, 'usage': {'output_tokens': 1}})
                    event('message_stop', {'type': 'message_stop'})
                except (BrokenPipeError, ConnectionResetError): pass
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def __enter__(self):
        self.thread.start()
        return f'http://127.0.0.1:{self.server.server_address[1]}'

    def __exit__(self, *_args):
        self.server.shutdown(); self.server.server_close(); self.thread.join(timeout=5)


def run(args):
    root, case = args.root.resolve(), args.case
    out = root / 'attempts' / args.attempt
    out.mkdir(parents=True, exist_ok=False)
    plugin = root / 'coding-agent/plugin'
    binary = root / 'bin/chainbench-mcp'
    sys.path.insert(0, str(root / 'coding-agent/plugin/scripts/tests'))
    from test_publication_policy import PolicyTests
    fixture = PolicyTests()
    fixture.setUp()
    try:
        if case == 'incomplete': fixture.lock()
        else:
            fixture.ready()
            finalized = fixture.cli('--finalize')
            assert finalized.returncode == 0, finalized.stdout + finalized.stderr
        if case == 'tampered':
            (fixture.f.workspace / 'test-report.md').write_text('changed after exit receipt')
        subject = fixture.f.repo
        remote = out / 'local-remote.git'
        subprocess.run(['git', 'init', '--bare', '-q', str(remote)], check=True)
        state = fixture.f.workspace / 'state.json'
        original_state = state.read_bytes()
        tool = 'mcp__plugin_coding-agent_chainbench__chainbench_chains'
        actions = []
        if case == 'tools':
            candidate = json.loads(original_state); candidate['config']['publication_policy'] = 'publish'
            actions = [
                {'name': tool, 'input': {}},
                {'name': 'Bash', 'input': {'command': f'git push {remote} HEAD:refs/heads/probe', 'description': 'Local fixture publication denial probe'}},
                {'name': 'Read', 'input': {'file_path': str(state)}},
                {'name': 'Write', 'input': {'file_path': str(state), 'content': json.dumps(candidate)}},
            ]
        profile = out / 'claude-config'; profile.mkdir()
        data_root = out / 'chainbench-state'; data_root.mkdir()
        mcp = {'mcpServers': {'plugin_coding-agent_chainbench': {
            'command': sys.executable, 'args': [str(Path(__file__).resolve()), '--proxy', '--binary', str(binary), '--log', str(out/'mcp-frames.jsonl')],
            'env': {'CHAINBENCH_DIR': str(root/'chainbench'), 'CHAINBENCH_DATA_ROOT': str(data_root)}}}}
        write(out/'mcp.json', mcp)
        settings = {'permissions': {'allow': ['Bash', 'Read', 'Write', tool]}, 'disableClaudeAiConnectors': True}
        observed = root / 'attempts/valid-02/host.stdout.jsonl'
        if observed.exists():
            init = next(json.loads(line) for line in observed.read_text().splitlines()
                        if json.loads(line).get('subtype') == 'init')
            settings['permissions']['deny'] = [name for name in init['tools']
                                                if name.startswith('mcp__') and name != tool]
        observer = shlex.join([sys.executable, str(Path(__file__).resolve()), '--observe', '--log', str(out/'hook-inputs.jsonl')])
        settings['hooks'] = {event: [{'hooks': [{'type': 'command', 'command': observer, 'timeout': 5}],
                                      **({'matcher': '*'} if event == 'PreToolUse' else {})}]
                             for event in ('SessionStart', 'Stop', 'PreToolUse')}
        write(out/'settings.json', settings)
        env = {'PATH': os.environ.get('PATH', '/usr/bin:/bin:/usr/sbin:/sbin'),
               'TMPDIR': str(out), 'CLAUDE_CONFIG_DIR': str(profile),
               'ANTHROPIC_API_KEY': 'local-fixture-not-a-real-key',
               'CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC': '1', 'DISABLE_TELEMETRY': '1',
               'DISABLE_ERROR_REPORTING': '1', 'ENABLE_CLAUDEAI_MCP_SERVERS': 'false',
               'CHAINBENCH_DIR': str(root/'chainbench'), 'CHAINBENCH_DATA_ROOT': str(data_root),
               'API_TIMEOUT_MS': '10000'}
        # Preserve HOME's inherited default; never override or read personal Claude config/auth.
        if 'HOME' in os.environ: env['HOME'] = os.environ['HOME']
        prompt = f'KS_HOST_PROBE:{case}. Controlled local transport fixture; no production model or agent acceptance.'
        argv = [args.claude, '--print', '--model', 'claude-sonnet-4-6', '--max-budget-usd', '0.10',
                '--plugin-dir', str(plugin), '--strict-mcp-config', '--mcp-config', str(out/'mcp.json'),
                '--setting-sources', '', '--settings', str(out/'settings.json'), '--permission-mode', 'manual',
                '--permission-prompts', 'none', '--include-hook-events', '--output-format', 'stream-json',
                '--verbose', '--no-session-persistence', '--no-chrome', '--tools', 'Bash,Read,Write',
                '--system-prompt', 'Local scripted transport test only. No real model calls. Follow configured tool restrictions.', prompt]
        gateway = Gateway(case, actions, out)
        with gateway as url:
            env['ANTHROPIC_BASE_URL'] = url
            write(out/'invocation.json', {'argv': argv, 'cwd': str(subject),
                  'env': {k: v for k,v in env.items() if k not in ('HOME','PATH','ANTHROPIC_API_KEY')},
                  'api_credential_kind': 'constant_fake_local_fixture_token', 'host_api_timeout_seconds': 90,
                  'kind': 'actual_cli_with_local_scripted_api_not_real_model'})
            start = time.monotonic()
            proc = subprocess.Popen(argv, cwd=subject, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                    text=True, start_new_session=True)
            forced = False
            cancel_requested = False
            if case == 'cancel':
                if gateway.received.wait(timeout=15):
                    cancel_requested = True
                    proc.send_signal(signal.SIGTERM)
            try:
                stdout, stderr = proc.communicate(timeout=90)
            except subprocess.TimeoutExpired:
                forced = True; os.killpg(proc.pid, signal.SIGTERM)
                try: stdout, stderr = proc.communicate(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(proc.pid, signal.SIGKILL); stdout, stderr = proc.communicate(timeout=5)
            (out/'host.stdout.jsonl').write_text(stdout)
            (out/'host.stderr.txt').write_text(stderr)
        # Normal CLI exit can precede server signal cleanup; bound the wait separately.
        for _ in range(30):
            log = out/'mcp-frames.jsonl'
            if log.exists() and any(json.loads(x).get('kind') == 'exit' for x in log.read_text().splitlines()): break
            time.sleep(0.1)
        receipt_check = fixture.cli('--check-exit')
        write(out/'exit.json', {'host_exit': proc.returncode, 'forced_host_termination': forced,
              'elapsed_seconds': time.monotonic()-start, 'state_bytes_unchanged': state.read_bytes()==original_state,
              'receipt_check_exit': receipt_check.returncode, 'receipt_check_stdout': receipt_check.stdout,
              'local_remote_refs': subprocess.check_output(['git','--git-dir',str(remote),'show-ref'],text=True) if subprocess.run(['git','--git-dir',str(remote),'show-ref'],capture_output=True).returncode==0 else '',
              'fixture_subject': str(subject), 'workspace': str(fixture.f.workspace),
              'case': case, 'cancel_requested': cancel_requested, 'real_model_calls': 0, 'real_chain_nodes_started': 0})
        print(json.dumps({'attempt': args.attempt, 'case': case, 'host_exit': proc.returncode,
                          'forced': forced, 'output': str(out), 'stdout_lines': len(stdout.splitlines())}))
        return 0 if (proc.returncode == 0 or cancel_requested) and not forced else 1
    finally:
        fixture.doCleanups()


def assess(folder):
    """Fail closed on absent hooks/MCP/receipts; CLI exit0 alone proves nothing."""
    errors = []
    try:
        outcome = json.loads((folder/'exit.json').read_text())
        invocation = json.loads((folder/'invocation.json').read_text())
        events = [json.loads(x) for x in (folder/'host.stdout.jsonl').read_text().splitlines()]
        frames = [json.loads(x) for x in (folder/'mcp-frames.jsonl').read_text().splitlines()]
        inputs = [json.loads(x)['payload'] for x in (folder/'hook-inputs.jsonl').read_text().splitlines()]
        requests = json.loads((folder/'gateway-requests.json').read_text())
        case = outcome['case']
        if outcome['forced_host_termination'] or outcome['local_remote_refs'] or not outcome['state_bytes_unchanged']:
            errors.append('forced exit, local push or changed state')
        if case != 'cancel' and outcome['host_exit'] != 0:
            errors.append('host failed')
        if case == 'cancel' and (not outcome['cancel_requested'] or outcome['host_exit'] not in (0, -15, 143, 130)):
            errors.append('host cancellation was not established')
        init = next(x for x in events if x.get('subtype') == 'init')
        servers = init['mcp_servers']
        if servers != [{'name': 'plugin_coding-agent_chainbench', 'status': 'connected', 'source': 'dynamic'}]:
            errors.append('unexpected/disconnected MCP configuration')
        aliases = [x for x in init['tools'] if x.startswith('mcp__')]
        if aliases != ['mcp__plugin_coding-agent_chainbench__chainbench_chains']:
            errors.append('host query-only tool restriction is missing')
        plugin = next(x for x in init['plugins'] if x['name'] == 'coding-agent')
        if plugin['source'] != 'coding-agent@inline' or not Path(plugin['path']).is_absolute():
            errors.append('plugin source identity missing')
        if any(x.get('source') != 'coding-agent@inline' and x.get('path') != 'builtin' for x in init['plugins']):
            errors.append('unexpected external plugin')
        hook_responses = [x for x in events if x.get('subtype') == 'hook_response']
        if not any(x.get('hook_event') == 'SessionStart' and x.get('exit_code') == 0 and ('Local evaluation boundary markers' in x.get('output', '') or 'Active coding-agent pipeline' in x.get('output', ''))
                   for x in hook_responses):
            errors.append('actual plugin SessionStart output missing')
        if case != 'incomplete' and any('resume with' in x.get('output', '') for x in hook_responses if x.get('hook_event') == 'SessionStart'):
            errors.append('evaluated session was asked to resume')
        if case != 'cancel':
            stops = [x for x in inputs if x.get('hook_event_name') == 'Stop']
            flags = [x.get('stop_hook_active') for x in stops]
            expected = [False, True] if case in ('incomplete', 'tampered') else [False]
            if flags != expected:
                errors.append('Stop re-entry payloads differ from bounded expectation')
            plugin_stops = [x for x in hook_responses if x.get('hook_event') == 'Stop' and x.get('output')]
            if case in ('incomplete', 'tampered'):
                if outcome['receipt_check_exit'] == 0:
                    errors.append('incomplete/altered boundary was promoted to pass')
                if len(plugin_stops) != 1 or json.loads(plugin_stops[0]['output']).get('decision') != 'block':
                    errors.append('actual blocking Stop response missing')
            elif outcome['receipt_check_exit'] != 0 or plugin_stops:
                errors.append('valid boundary failed or resumed')
        else:
            flags = []
        exits = [x for x in frames if x['kind'] == 'exit']
        if len(exits) != 1 or exits[0]['exit_code'] != 0 or exits[0]['forced_kill']:
            errors.append('owned MCP server did not exit cleanly')
        starts = [x for x in frames if x['kind'] == 'start']
        if len(starts) != 1:
            errors.append('owned MCP process binding missing')
        rpc = [json.loads(x['raw']) for x in frames if x['kind'] == 'request']
        replies = {json.loads(x['raw']).get('id'): json.loads(x['raw']) for x in frames if x['kind'] == 'response'}
        listing = next(x for x in rpc if x.get('method') == 'tools/list')
        schemas = replies[listing['id']]['result']['tools']
        if len(schemas) != 81:
            errors.append('server tool contract count changed')
        calls = [x for x in rpc if x.get('method') == 'tools/call']
        if case == 'tools':
            if len(calls) != 1 or calls[0]['params']['name'] != 'chainbench_chains':
                errors.append('actual MCP metadata call missing or unexpected mutation')
            else:
                reply = replies[calls[0]['id']]['result']
                if reply.get('isError') or 'stablenet' not in json.dumps(reply):
                    errors.append('MCP metadata call did not succeed')
            denials = []
            for hook in hook_responses:
                if hook.get('hook_event') != 'PreToolUse' or not hook.get('output'): continue
                parsed = json.loads(hook['output']).get('hookSpecificOutput', {})
                if parsed.get('permissionDecision') == 'deny': denials.append(parsed.get('permissionDecisionReason', ''))
            if not any('publication/remote-write commands are blocked' in text for text in denials):
                errors.append('publication plugin denial missing')
            if not any('publication policy is immutable' in text for text in denials):
                errors.append('policy mutation plugin denial missing (a builtin refusal is insufficient)')
        elif calls:
            errors.append('unexpected MCP tool execution')
        argv = invocation['argv']
        for required in ('--plugin-dir', '--strict-mcp-config', '--setting-sources', '--permission-prompts', '--include-hook-events'):
            if required not in argv: errors.append('required host boundary flag missing')
        if any(x in argv for x in ('--bare', '--safe-mode', '--dangerously-skip-permissions')):
            errors.append('hook/permission bypass flag present')
        if not invocation['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:'):
            errors.append('model transport not local')
        facts = {'case': case, 'status': 'PASS' if not errors else 'FAIL', 'errors': errors,
                 'actual_host_version': init['claude_code_version'], 'stop_hook_active_flags': flags,
                 'host_exit': outcome['host_exit'], 'receipt_check_exit': outcome['receipt_check_exit'],
                 'server_tool_count': len(schemas), 'mcp_calls': len(calls), 'gateway_requests': len(requests),
                 'server_exit': exits[0]['exit_code'] if exits else None,
                 'server_tool_schema_sha256': hashlib.sha256(json.dumps(schemas,sort_keys=True,separators=(',',':')).encode()).hexdigest(),
                 'local_integration_only': True, 'real_model_inference': 'not_run', 'agent_pipeline_acceptance': 'not_run'}
        return facts
    except (OSError, ValueError, KeyError, TypeError, StopIteration) as exc:
        return {'status': 'FAIL', 'errors': ['missing or malformed actual host evidence: '+str(exc)]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--proxy', action='store_true')
    parser.add_argument('--observe', action='store_true')
    parser.add_argument('--assess', type=Path)
    parser.add_argument('--binary', type=Path)
    parser.add_argument('--log', type=Path)
    parser.add_argument('--root', type=Path)
    parser.add_argument('--case', choices=['valid', 'incomplete', 'tools', 'tampered', 'cancel'])
    parser.add_argument('--attempt')
    parser.add_argument('--claude', default='/Users/kevin/.local/bin/claude')
    args = parser.parse_args()
    if args.assess:
        result = assess(args.assess)
        print(json.dumps(result, indent=2))
        return 0 if result['status'] == 'PASS' else 1
    if args.observe:
        payload = json.load(sys.stdin)
        with args.log.open('a') as stream:
            stream.write(json.dumps({'time': time.time(), 'payload': payload}) + '\n')
        return 0
    return proxy(args) if args.proxy else run(args)


if __name__ == '__main__':
    raise SystemExit(main())
