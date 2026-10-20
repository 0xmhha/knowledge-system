#!/usr/bin/env python3
"""T16: actual owned stdio faults, lossless attempts and late-response accounting."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import queue
import signal
import socket
import sqlite3
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


base = module('consumer-compatible-host.py')
stdio = module('prepare-consumer-stdio-readiness.py')
own = module('consumer-process-ownership.py')
dump, sha, require = base.dump, base.sha, base.require
PLAN = ROOT / 'scripts/fixtures/consumer-errors-plan.json'


class Client(stdio.OwnedClient):
    """Persist attempts BEFORE writing; a timeout can never disappear or become success."""
    def __init__(self, command, path):
        self.attempts = []
        self.signals = []
        self.late = []
        super().__init__(command, path)
        self.identity = own.snapshot(self.proc.pid)

    def request(self, label, method, params=None, deadline_seconds=10):
        require(time.monotonic() - self.started < 180, 'session budget')
        if method == 'tools/call':
            require(self.calls < 16, 'tool cap')
            self.calls += 1
        self.id += 1
        req = {'jsonrpc': '2.0', 'id': self.id, 'method': method}
        if params is not None:
            req['params'] = params
        row = {'label': label, 'request': req, 'outcome': 'attempted', 'deadline_seconds': deadline_seconds, 'response': None}
        self.attempts.append(row)
        dump(self.path / 'attempts.json', self.attempts)
        raw = json.dumps(req) + '\n'
        self.ins.write(raw)
        self.ins.flush()
        started = time.monotonic()
        try:
            self.proc.stdin.write(raw)
            self.proc.stdin.flush()
            end = started + deadline_seconds
            while True:
                line = self.queue.get(timeout=max(.001, end - time.monotonic()))
                if line is None:
                    row['outcome'] = 'eof_without_response'
                    break
                response = json.loads(line)
                if response.get('id') == self.id:
                    row.update(outcome='response', response=response)
                    self.messages.append({'label': label, 'request': req, 'response': response})
                    break
                self.late.append(response)
        except queue.Empty:
            row['outcome'] = 'transport_timeout'
        except (BrokenPipeError, OSError) as error:
            row.update(outcome='write_failure', error=type(error).__name__)
        row['elapsed_seconds'] = time.monotonic() - started
        dump(self.path / 'attempts.json', self.attempts)
        return row['response']

    def suspend(self):
        identity = own.snapshot(self.proc.pid)
        require(identity and all(identity[k] == self.identity[k] for k in ['pid', 'ppid', 'birth', 'command']) and self.proc.poll() is None, 'exact live owned child identity')
        self.proc.send_signal(signal.SIGSTOP)
        stopped = None
        for _ in range(100):
            q = subprocess.run(['ps', '-p', str(self.proc.pid), '-o', 'stat='], text=True, capture_output=True)
            if q.returncode == 0 and 'T' in q.stdout:
                stopped = q.stdout.strip()
                break
            time.sleep(.01)
        require(stopped is not None, 'owned child did not stop')
        self.signals.append({'signal': int(signal.SIGSTOP), 'target_pid': self.proc.pid, 'identity': identity, 'observed_state': stopped, 'owned_Popen_child': True})

    def resume(self):
        identity = own.snapshot(self.proc.pid)
        require(identity and identity['pid'] == self.identity['pid'] and identity['ppid'] == self.identity['ppid'] and identity['command'] == self.identity['command'] and identity['birth'] == self.identity['birth'], 'resume owned child identity')
        self.proc.send_signal(signal.SIGCONT)
        self.signals.append({'signal': int(signal.SIGCONT), 'target_pid': self.proc.pid, 'identity': identity, 'owned_Popen_child': True})

    def close(self):
        super().close()
        dump(self.path / 'attempts.json', self.attempts)
        dump(self.path / 'faults.json', {'identity': self.identity, 'signals': self.signals, 'late_responses': self.late})


def run(root):
    require(str(root).startswith('/private/tmp/ks-consumer-errors-'), 'owned root required')
    base.setup(root)
    plan = json.loads(PLAN.read_text())
    pins = json.loads((ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/host-schema-compatibility-v1/binary-pins.json').read_text())
    require(sha(base.BINARY) == pins['cks_sha256'], 'binary pin')
    query = json.loads((base.P05 / 'execution.json').read_text())['query']
    query['expected_snapshot_sha256'] = next(r['response']['result']['structuredContent']['snapshot_sha256'] for r in json.loads((base.P05 / 'sessions/main/messages.json').read_text()) if r['label'] == 'baseline')
    old = root / 'legacy.db'
    old.write_bytes((root / 'graph/graph.db').read_bytes())
    with sqlite3.connect(old) as db:
        db.execute('DROP TABLE code_edges')
        db.commit()
        checkpoint = db.execute('PRAGMA wal_checkpoint(TRUNCATE)').fetchall()
    (root / 'broken.db').write_bytes(b'owned T16 invalid SQLite database\n')
    configs = {}
    plain = (root / 'cks.yaml').read_text()
    for name, text in [('main', plain), ('timeout', plain), ('legacy', plain.replace(str(root / 'graph/graph.db'), str(old))), ('startup', plain.replace(str(root / 'graph/graph.db'), str(root / 'broken.db')))]:
        path = root / (name + '.yaml')
        path.write_text(text)
        configs[name] = path
    # Bound but not listening: the OS must reject connections; the reserved
    # loopback port cannot be reused by a different listener during the probe.
    reservation = socket.socket()
    reservation.bind(('127.0.0.1', 0))
    endpoint = 'http://127.0.0.1:' + str(reservation.getsockname()[1])
    probe = socket.socket()
    probe.settimeout(2)
    errno = probe.connect_ex(reservation.getsockname())
    probe.close()
    require(errno != 0, 'reserved endpoint unexpectedly listening')
    text = plain.replace('    provider: mock', '    provider: ollama\n    path: ' + str(root / 'owned-vector') + '\n    embed_model: bge-m3\n    ollama_url: ' + endpoint)
    configs['provider'] = root / 'provider.yaml'
    configs['provider'].write_text(text)
    dump(root / 'fault-setup.json', {'endpoint': endpoint, 'bound_not_listening': True, 'probe_connect_errno': errno, 'normal_db_sha256': sha(root / 'graph/graph.db'), 'legacy_db_sha256': sha(old), 'broken_db_sha256': sha(root / 'broken.db'), 'checkpoint': checkpoint, 'operation': 'DROP TABLE code_edges; checkpoint WAL', 'owned_fault_not_operational_outage': True})
    clients = []
    try:
        for name in plan['sessions']:
            client = Client([str(base.BINARY), 'mcp', '--config', str(configs[name])], root / name)
            clients.append(client)
            try:
                if name == 'startup':
                    client.request('startup', 'initialize', {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'owned-T16', 'version': '1'}})
                    continue
                client.initialize()
                if name == 'main':
                    client.tool('positive', 'value_dependencies', query)
                    client.tool('missing', 'value_dependencies', {'node_id': 'missing-owned-symbol'})
                    client.tool('ambiguous', 'review_function', {'symbol': 'Same'})
                elif name == 'legacy':
                    client.tool('legacy', 'review_function', {'symbol': 'example.invalid/consumer-stdio-fixture.Seed'})
                elif name == 'provider':
                    client.request('provider-health', 'tools/call', {'name': 'cks.ops.health', 'arguments': {}})
                    client.tool('provider', 'get_for_task', {'prompt': 'owned T16 provider fault'})
                else:
                    client.suspend()
                    try:
                        client.request('timeout', 'tools/call', {'name': 'cks.context.value_dependencies', 'arguments': query}, plan['transport_deadline_seconds'])
                    finally:
                        client.resume()
                    # The following request receives and retains the late reply
                    # for the expired request, then awaits its own distinct id.
                    client.tool('recovery', 'value_dependencies', query)
            finally:
                client.close()
    finally:
        reservation.close()
    dump(root / 'execution.json', {'root': str(root), 'query': query, 'driver_sha256': sha(Path(__file__)), 'plan_sha256': sha(PLAN), 'binary_sha256': sha(base.BINARY), 'config_hashes': {name: sha(path) for name, path in configs.items()}, 'sessions': plan['sessions'], 'candidate_adopted': False, 'real_model_calls': 0, 'formal_runs': 0, 'actual_role_sessions': 0})
    print('T16 owned execution retained:5 children;7 evaluation tools +1 diagnostic +1 startup case')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    args = parser.parse_args()
    run(args.root.resolve())
