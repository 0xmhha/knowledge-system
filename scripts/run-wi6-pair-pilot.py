#!/usr/bin/env python3
"""Owned retained DEV19 -> CKS find_symbol/probe/basic-expansion one-pair pilot."""
import argparse
import gzip
import importlib.util
import json
import os
import queue
import shutil
import signal
import sqlite3
import subprocess
import threading
import time
from pathlib import Path

import wi6_pair_contract as contract
import wi6_version_readiness as preparation

ROOT = preparation.ROOT
PACKET = ROOT / preparation.BASE / 'wi6-paired-runtime-v1'
DATA = Path('/private/tmp/ks-snp04-20261018-01/stablenet/retained-origins-v2')
BINARY = Path('/private/tmp/ks-wi3-walk-20261019/native-source-final-v2/bin/cks')


def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')


def inventory(root):
    return {p.relative_to(root).as_posix(): contract.sha(p.read_bytes()) for p in sorted(root.rglob('*'))
            if p.is_file() and not p.name.endswith(('.lock', '.lease'))}


def live_group(pgid):
    rows = subprocess.check_output(['ps', '-axo', 'pid=,pgid=,stat='], text=True).splitlines()
    return [int(v[0]) for line in rows if len(v := line.split()) == 3 and int(v[1]) == pgid and not v[2].startswith('Z')]


class Client:
    def __init__(self, work, config):
        self.work, self.id, self.q = work, 0, queue.Queue()
        self.stdin = (work / 'stdin.jsonl').open('w')
        self.stdout = (work / 'stdout.jsonl').open('w')
        self.stderr = (work / 'stderr.txt').open('wb')
        self.start = time.perf_counter_ns()
        env = {k: os.environ[k] for k in ('HOME', 'USER', 'TMPDIR', 'LANG', 'LC_ALL') if k in os.environ}
        env.update(PATH='/usr/bin:/bin:/usr/sbin:/sbin', TMPDIR=str(work / 'tmp'))
        (work / 'tmp').mkdir()
        self.argv = [str(BINARY), 'mcp', '--config', str(config)]
        self.proc = subprocess.Popen(self.argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=self.stderr, text=True, env=env, start_new_session=True)
        write(work / 'live.json', dict(pid=self.proc.pid, pgid=self.proc.pid, argv=self.argv,
                                      allowed_environment_keys=sorted(env), owned_new_session=True))
        def reader():
            for line in self.proc.stdout:
                self.q.put(line)
            self.q.put(None)
        self.thread = threading.Thread(target=reader, daemon=True)
        self.thread.start()

    def request(self, method, params=None):
        self.id += 1
        msg = dict(jsonrpc='2.0', id=self.id, method=method)
        if params is not None:
            msg['params'] = params
        line = json.dumps(msg, separators=(',', ':')) + '\n'
        self.stdin.write(line); self.stdin.flush()
        start = time.perf_counter_ns()
        self.proc.stdin.write(line); self.proc.stdin.flush()
        end = time.monotonic() + (120 if method == 'initialize' else 20)
        while time.monotonic() < end:
            try:
                line = self.q.get(timeout=max(.01, end-time.monotonic()))
            except queue.Empty:
                raise TimeoutError('bounded actual MCP response deadline') from None
            if line is None:
                raise RuntimeError('actual MCP stream ended')
            self.stdout.write(line); self.stdout.flush()
            reply = json.loads(line)
            if reply.get('id') == self.id:
                return dict(request=msg, raw_response=reply, elapsed_ns=time.perf_counter_ns()-start)
        raise TimeoutError('bounded actual MCP response deadline')

    def close(self):
        term = kill = False
        try:
            self.proc.stdin.close()
        except BrokenPipeError:
            pass
        try:
            self.proc.wait(timeout=20)
        except subprocess.TimeoutExpired:
            term = True; os.killpg(self.proc.pid, signal.SIGTERM)
            try:
                self.proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                kill = True; os.killpg(self.proc.pid, signal.SIGKILL); self.proc.wait(timeout=5)
        self.thread.join(timeout=2)
        self.proc.stdout.close()
        self.stdin.close(); self.stdout.close(); self.stderr.close()
        write(self.work / 'session.json', dict(argv=self.argv, pid=self.proc.pid, pgid=self.proc.pid,
              exit_code=self.proc.returncode, requests=self.id, reaped=True, owned_new_session=True,
              term_sent=term, kill_sent=kill, reader_joined=not self.thread.is_alive(),
              remaining_live_group_members=len(live_group(self.proc.pid)),
              elapsed_ns=time.perf_counter_ns()-self.start))


def run(work, scope_path=None):
    work.mkdir()
    scope = json.loads((scope_path or PACKET / 'SCOPE.json').read_text())
    write(work / 'SCOPE.json', scope)
    if any(contract.sha((ROOT / name).read_bytes()) != h for name, h in scope['tools_sha256'].items()):
        raise ValueError('frozen runtime source changed')
    if contract.sha(BINARY.read_bytes()) != scope['binary_sha256']:
        raise ValueError('verified native binary changed')
    before = inventory(DATA)
    write(work / 'original-before.json', before)
    if before != scope['original_data_sha256']:
        raise ValueError('predeclared original dataset changed')
    owned = work / 'data'
    shutil.copytree(DATA, owned, ignore=lambda d, names: [n for n in names if n.endswith(('.lock', '.lease'))])
    if inventory(owned) != before:
        raise ValueError('owned immutable data copy differs')
    contract.restore_reader_coordination(owned)
    write(work / 'owned-before.json', inventory(owned))
    identity = json.loads((owned / 'dataset-identity.json').read_text())
    manifest = json.loads((owned / 'graph/manifest.json').read_text())
    files = {r['path']: r['sha256'] for r in manifest['files']}
    write(work / 'dataset-pins.json', {k: manifest[k] for k in ('project_id', 'dataset_id', 'source_mode', 'src_commit', 'snapshot_id')})
    if manifest['project_id'] != identity['source']['project_id'] or manifest['dataset_id'] != identity['dataset_id'] or manifest['src_commit'] != scope['source_commit']:
        raise ValueError('source/dataset identity differs')
    connection = sqlite3.connect((owned / 'graph/graph.db').as_uri()+'?mode=ro', uri=True)
    selected, retained = [], {}
    for row in scope['cases']:
        node, blob = contract.select_seed(connection, row['source'], files, row['expected_canonical_id'])
        selected.append(dict(case_id=row['case_id'], node=node))
        retained[node['node_id']] = node
    write(work / 'retained-seeds.json', selected)
    source = work / 'empty-live-source'; source.mkdir()
    config = work / 'config.yaml'
    original = (ROOT / preparation.BASE / 'wi3-walk-budget-v1/after-runtime/off/config.txt').read_text()
    original = original.replace(str(DATA), str(owned)).replace('/private/tmp/ks-snp04-20261018-01/stablenet-source', str(source))
    original = original.replace('/private/tmp/ks-wi3-walk-20261019/after-runtime/off/footprints', str(work / 'footprints'))
    config.write_text(original)
    client = Client(work, config)
    rows, pairs = [], []
    def call(case, phase, name, arguments):
        r = client.request('tools/call', dict(name='cks.context.'+name, arguments=arguments))
        r.update(case_id=case, phase=phase)
        rows.append(r); write(work / 'calls.json', rows)
        return r
    try:
        write(work / 'initialize.json', client.request('initialize', dict(protocolVersion='2024-11-05', capabilities={}, clientInfo=dict(name='owned-wi6-pair-pilot', version='1'))))
        tools = client.request('tools/list'); write(work / 'tools.json', tools)
        names = {r['name'] for r in tools['raw_response']['result']['tools']}
        if not {'cks.context.find_symbol','cks.context.value_dependencies','cks.context.value_dependency_expansion'} <= names:
            raise ValueError('actual required tools not exposed')
        common_snapshot = None
        for i, (case, chosen) in enumerate(zip(scope['cases'], selected)):
            cid, node = case['case_id'], chosen['node']
            found = call(cid, 'discovery', 'find_symbol', dict(name=case['expected_canonical_id'], path_glob=node['path'], kinds='function,method'))
            value, encoding = contract.unpack(found['raw_response'])
            citations = value.get('citations', []) if value else []
            if not any(c.get('file')==node['path'] and c.get('start_line')==node['start_line'] and c.get('end_line')==node['end_line'] and c.get('commit_hash')==scope['source_commit'] for c in citations):
                raise ValueError('actual consumer discovery did not cite exact retained seed')
            probe_args = dict(node_id=node['node_id'], expected_commit=scope['source_commit'], expected_source_sha256=node['function_sha256'], limits=preparation.LOCAL_LIMITS)
            probe = call(cid, 'snapshot_probe', 'value_dependencies', probe_args)
            value, _ = contract.unpack(probe['raw_response'])
            snapshot = value.get('snapshot_sha256') if value else None
            if not isinstance(snapshot,str) or len(snapshot)!=64 or common_snapshot not in (None,snapshot):
                raise ValueError('actual graph snapshot unavailable or changed')
            common_snapshot = snapshot
            pins = dict(node_id=node['node_id'], project_id=manifest['project_id'], dataset_id=manifest['dataset_id'], commit=scope['source_commit'], function_sha256=node['function_sha256'], snapshot_sha256=snapshot)
            requests = preparation.requests(pins)
            pair = dict(case_id=cid, pins=pins, requests=requests, order=['baseline','expansion'] if i%2==0 else ['expansion','baseline'], arms={})
            pairs.append(pair); write(work / 'pairs.json', pairs)
            for arm in pair['order']:
                tool = 'value_dependencies' if arm=='baseline' else 'value_dependency_expansion'
                r = call(cid, 'pilot_'+arm, tool, requests[arm]); pair['arms'][arm] = dict(request_id=r['request']['id'], elapsed_ns=r['elapsed_ns'])
                value, _ = contract.unpack(r['raw_response'])
                if value:
                    for result in contract.function_results(arm, value):
                        s = (result or {}).get('analysis') or {}; sid = s.get('source',{}).get('node_id')
                        if sid and sid not in retained:
                            evidence, blob = contract.retained_node(connection,sid,files);retained[sid]=evidence
                write(work/'retained-nodes.json',retained);write(work/'pairs.json',pairs)
            print(json.dumps(dict(case_id=cid,pair_recorded=True)),flush=True)
    finally:
        client.close(); connection.close()
        write(work/'original-after.json',inventory(DATA));write(work/'owned-after.json',inventory(owned))
    if inventory(DATA)!=before or inventory(owned)!=before:
        raise ValueError('original/owned data changed during read-only pilot')
    if contract.sha(BINARY.read_bytes()) != scope['binary_sha256']:
        raise ValueError('postrun binary changed')
    write(work/'receipt.json',dict(planned_cases=19,actual_pairs=len(pairs),tool_calls=len(rows),automatic_retries=0,
          original_and_owned_data_preserved=True,binary_sha256=scope['binary_sha256'],session_processes=1,
          empty_live_source_preserved=not list(source.iterdir()),warm_samples=0,independent_approved=0,
          formal_completed=0,formal_total=9,quality_scores=None,paid_or_external_LLM_calls=0,
          embedding_inference_calls=None,model_identity_scope='retained_dataset_embedding_metadata; no LLM/model calls invoked by harness'))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);p.add_argument('--scope',type=Path);a=p.parse_args();run(a.workspace.resolve(),a.scope)
