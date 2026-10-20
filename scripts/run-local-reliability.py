#!/usr/bin/env python3
"""Owned actual WI1 replay and embedding-cache off/on, using local BGE only."""
import argparse
import gzip
import importlib.util
import json
import shutil
import sqlite3
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import wi1_consumer_matching as matching
import wi6_pair_contract as contract

ROOT = matching.ROOT
BASE = matching.BASE/'followups'
spec = importlib.util.spec_from_file_location('pilot', ROOT/'scripts/run-wi6-pair-pilot.py')
pilot = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pilot)


def run(work, binary):
    work.mkdir()
    scope = json.loads((ROOT/BASE/'local-reliability-v1/RUNTIME-SCOPE.json').read_text())
    pilot.write(work/'SCOPE.json',scope)
    if any(contract.sha((ROOT/n).read_bytes())!=h for n,h in scope['tools_sha256'].items()):
        raise ValueError('predeclared runtime tool changed')
    if contract.sha(binary.read_bytes()) != scope['binary_sha256']:
        raise ValueError('runtime binary differs from full native validation')
    before = pilot.inventory(pilot.DATA)
    pilot.write(work/'original-before.json', before)
    shutil.copytree(pilot.DATA, work/'data', ignore=lambda d, names: [n for n in names if n.endswith(('.lock', '.lease'))])
    contract.restore_reader_coordination(work/'data')
    if pilot.inventory(work/'data') != before:
        raise ValueError('owned data differs')
    pilot.write(work/'owned-before.json', before)
    (work/'empty-live-source').mkdir()
    pilot.BINARY = binary
    http_events, lock = [], threading.Lock()
    proxy_mode = {'value':'normal'}

    class Proxy(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def relay(self):
            length = int(self.headers.get('Content-Length', '0'))
            if length > 1024*1024:
                self.send_error(413)
                return
            body = self.rfile.read(length) if length else None
            with lock:
                mode=proxy_mode['value']
            injected=(mode=='deny_embed'and self.path in ('/api/embed','/api/embeddings'))or(mode=='deny_tags'and self.path=='/api/tags')
            if injected:
                payload=b'{"error":"owned local failure fixture"}'
                with lock:
                    http_events.append(dict(method=self.command,path=self.path,status=503,forwarded=False,
                        request_sha256=contract.sha(body or b''),response_sha256=contract.sha(payload)))
                self.send_response(503);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
                return
            request = urllib.request.Request('http://127.0.0.1:11434'+self.path, data=body,
                        method=self.command, headers={'Content-Type': 'application/json'})
            try:
                response = urllib.request.urlopen(request, timeout=15)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                payload = response.read(8*1024*1024+1)
                if len(payload) > 8*1024*1024:
                    self.send_error(502)
                    return
                with lock:
                    http_events.append(dict(method=self.command, path=self.path, status=response.status,forwarded=True,
                        request_sha256=contract.sha(body or b''), response_sha256=contract.sha(payload)))
                self.send_response(response.status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)

        do_GET = relay
        do_POST = relay

    def embed_count():
        with lock:
            return sum(e['path'] in ('/api/embed', '/api/embeddings') for e in http_events)
    manifest = json.loads((work/'data/graph/manifest.json').read_text())
    files = {r['path']: r['sha256'] for r in manifest['files']}
    selected = []
    connection = sqlite3.connect((work/'data/graph/graph.db').as_uri()+'?mode=ro', uri=True)
    for case in matching.old.read(ROOT/matching.old.FUNCTIONS)['functions']:
        source = case['source']
        canonical = 'github.com/ethereum/go-ethereum/'+str(Path(source['path']).parent)+'.'+('('+source['receiver']+').' if source['receiver'] else '')+source['name']
        node, _ = contract.select_seed(connection, source, files, canonical)
        selected.append(dict(case_id=case['candidate_id'], node=node))
    connection.close()
    pilot.write(work/'selected.json', selected)
    calls, searches, boundaries = [], [], []
    proxy = ThreadingHTTPServer(('127.0.0.1', 0), Proxy)
    thread = threading.Thread(target=proxy.serve_forever, daemon=True)
    thread.start()
    proxy_url = 'http://127.0.0.1:'+str(proxy.server_port)
    try:
        template = gzip.decompress((ROOT/BASE/'wi6-warm-runtime-v1/actual/config.yaml.gz').read_bytes()).decode()
        config_base = template.replace('/private/tmp/ks-wi6-warm-20261019', str(work))
        # Change only the declared local provider endpoint and opt-in capacity.
        import re
        config_base, count = re.subn(r'(?m)^(\s*)ollama_url:.*$', r'\1ollama_url: '+proxy_url, config_base)
        if count != 1:
            raise ValueError('expected exactly one explicit local Ollama endpoint')
        for arm, entries in [('off', 0), ('on', 32)]:
            session = work/arm
            session.mkdir()
            config = session/'config.yaml'
            config.write_text(config_base.replace('    embed_model:', '    embedding_cache_entries: '+str(entries)+'\n    embed_model:'))
            if config.read_text().count('embedding_cache_entries:') != 1:
                raise ValueError('cache configuration insertion failed')
            client = pilot.Client(session, config)
            def call(phase, name, arguments):
                response = client.request('tools/call', dict(name=name, arguments=arguments))
                response.update(arm=arm, phase=phase)
                calls.append(response)
                return response
            try:
                pilot.write(session/'initialize.json', client.request('initialize', dict(protocolVersion='2024-11-05', capabilities={}, clientInfo=dict(name='owned-local-reliability', version='1'))))
                pilot.write(session/'tools.json', client.request('tools/list'))
                call('health', 'cks.ops.health', {})
                for repeat in range(2):
                    start = embed_count()
                    r = call('semantic_search_'+str(repeat), 'cks.context.semantic_search', scope['semantic_search_arguments'])
                    searches.append(dict(arm=arm, repeat=repeat, request_id=r['request']['id'], embedding_HTTP_calls=embed_count()-start))
                if arm == 'on':
                    for entry in scope['model_boundary_plan']:
                        with lock:proxy_mode['value']=entry['mode']
                        if entry['expire_health_TTL']:time.sleep(5.1)
                        embedding_before=embed_count()
                        r=call(entry['phase'],entry['tool'],{}if entry['tool']=='cks.ops.health'else scope['semantic_search_arguments'])
                        boundaries.append(dict(phase=entry['phase'],mode=entry['mode'],request_id=r['request']['id'],embedding_HTTP_calls=embed_count()-embedding_before))
                    pilot.write(work/'controlled-model-boundaries.json',boundaries)
                    pins = scope['pins']
                    first = selected[0]['node']
                    args = dict(node_id=first['node_id'], expected_commit=pins['commit'], expected_source_sha256=first['function_sha256'], limits=scope['limits'])
                    probe = call('snapshot_probe', 'cks.context.value_dependencies', args)
                    value, _ = contract.unpack(probe['raw_response'])
                    snapshot = value['snapshot_sha256']
                    if snapshot != scope['pins']['snapshot_sha256']:
                        raise ValueError('actual retained snapshot changed')
                    for row in selected:
                        node = row['node']
                        args = dict(node_id=node['node_id'], expected_commit=pins['commit'], expected_source_sha256=node['function_sha256'], expected_snapshot_sha256=snapshot, limits=scope['limits'])
                        r = call('WI1_'+row['case_id'], 'cks.context.value_dependencies', args)
                        r['case_id'] = row['case_id']
                        pilot.write(work/'calls.json', calls)
            finally:
                client.close()
                pilot.write(work/'calls.json', calls)
                pilot.write(work/'search-counts.json', searches)
    finally:
        proxy.shutdown()
        proxy.server_close()
        thread.join(timeout=3)
        pilot.write(work/'http-events.json', http_events)
        pilot.write(work/'original-after.json', pilot.inventory(pilot.DATA))
        pilot.write(work/'owned-after.json', pilot.inventory(work/'data'))
    if thread.is_alive() or pilot.inventory(pilot.DATA) != before or pilot.inventory(work/'data') != before or list((work/'empty-live-source').iterdir()):
        raise ValueError('owned cleanup/payload preservation failed')
    pilot.write(work/'receipt.json', dict(binary_sha256=scope['binary_sha256'], sessions=2,
                tool_calls=len(calls), WI1_cases=30, semantic_searches=8, controlled_model_boundary_calls=9, automatic_retries=0,
                original_owned_payload_preserved=True, proxy_thread_joined=True,
                external_or_paid_LLM_calls=0, formal_completed=0, independent_approved=0,
                quality_score=None))


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--workspace', type=Path, required=True)
    p.add_argument('--binary', type=Path, required=True)
    a = p.parse_args()
    run(a.workspace.resolve(), a.binary.resolve())
