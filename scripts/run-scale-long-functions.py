#!/usr/bin/env python3
"""Measure one owned retained graph; local BGE, never independent gold/SLA."""
import argparse
import gzip
import importlib.util
import json
import re
import shutil
import subprocess
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import viewer_readiness as v
import wi6_pair_contract as contract

ROOT=v.ROOT
PACKET=v.PACKET.parent/'scale-long-functions-v1'
spec=importlib.util.spec_from_file_location('pilot',ROOT/'scripts/run-wi6-pair-pilot.py')
pilot=importlib.util.module_from_spec(spec);spec.loader.exec_module(pilot)

def run(work,binary,native):
    scope=v.read(ROOT/PACKET/'SPEC.json');pins=v.read(ROOT/PACKET/'RUNTIME-PINS.json')
    v.require(v.sha((ROOT/PACKET/'SPEC.json').read_bytes())==pins['spec_sha256'],'scale specification changed')
    v.require(v.sha(binary.read_bytes())==scope['binary_sha256']==v.read(native/'binary.json')['sha256'],'fully validated runtime binary differs')
    receipts=v.read(native/'receipts.json')
    v.require(len(receipts)==7 and all(r['exit_code']==0 and not r['timeout'] and r['reaped']for r in receipts)and(native/'graph-binary.json').exists(),'whole native execution gate not satisfied')
    v.require(v.sha((native/'source-binding.json').read_bytes())==scope['binary_source_binding_sha256'],'native source inputs differ')
    binding=v.read(native/'source-binding.json')
    def check_source_inputs():
        v.require(all(v.sha((ROOT/n).read_bytes())==h for n,h in {**binding['source_sha256'],**binding['approved_input_sha256']}.items()),'validated source inputs changed')
    check_source_inputs()
    ref=ROOT/pins['inventory'];v.require(v.sha(ref.read_bytes())==pins['inventory_sha256'],'retained inventory reference changed')
    before=pilot.inventory(pilot.DATA);v.require(before==v.read(ref),'original retained payload differs')
    model=Path.home()/'.ollama/models/manifests/registry.ollama.ai/library/bge-m3/latest'
    v.require(v.sha(model.read_bytes())==pins['model_manifest_digest'],'approved local BGE identity changed')
    work.mkdir();v.write(work/'SCOPE.json',dict(scope,spec_sha256=pins['spec_sha256'],runtime_pins_sha256=v.sha((ROOT/PACKET/'RUNTIME-PINS.json').read_bytes()),driver_sha256=v.sha(Path(__file__).read_bytes())))
    v.write(work/'original-before.json',before)
    shutil.copytree(pilot.DATA,work/'data',ignore=lambda parent,names:[n for n in names if n.endswith(('.lock','.lease'))])
    contract.restore_reader_coordination(work/'data');v.write(work/'owned-before.json',pilot.inventory(work/'data'))
    v.require(pilot.inventory(work/'data')==before,'owned retained payload differs')
    events=[];lock=threading.Lock();active={'HTTP_handlers':0}
    class Proxy(BaseHTTPRequestHandler):
        def log_message(self,*args):pass
        def handle(self):
            with lock:active['HTTP_handlers']+=1
            try:super().handle()
            finally:
                with lock:active['HTTP_handlers']-=1
        def relay(self):
            n=int(self.headers.get('Content-Length','0'))
            if n>1024*1024:self.send_error(413);return
            body=self.rfile.read(n)if n else None
            request=urllib.request.Request('http://127.0.0.1:11434'+self.path,data=body,method=self.command,headers={'Content-Type':'application/json'})
            start=time.perf_counter_ns()
            try:response=urllib.request.urlopen(request,timeout=15)
            except urllib.error.HTTPError as error:response=error
            with response:
                payload=response.read(8*1024*1024+1)
                if len(payload)>8*1024*1024:self.send_error(502);return
                row=dict(method=self.command,path=self.path,status=response.status,elapsed_ns=time.perf_counter_ns()-start,request_sha256=v.sha(body or b''),response_sha256=v.sha(payload),response_bytes=len(payload))
                if self.path in('/api/embed','/api/embeddings'):
                    req=json.loads(body);answer=json.loads(payload);inputs=req.get('input',req.get('prompt',[]));vectors=answer.get('embeddings',[])
                    row.update(input_count=1 if isinstance(inputs,str)else len(inputs),output_count=len(vectors),dimensions=sorted({len(x)for x in vectors}),model=req.get('model'))
                if self.path=='/api/tags':row['bge_digests']=[x['digest']for x in json.loads(payload).get('models',[])if x.get('name')==pins['model']]
                with lock:events.append(row)
                self.send_response(response.status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
        do_GET=relay
        do_POST=relay
    q=work/'session';q.mkdir();(work/'empty-live-source').mkdir()
    template=gzip.decompress((ROOT/v.PACKET.parent/'wi6-warm-runtime-v1/actual/config.yaml.gz').read_bytes()).decode().replace('/private/tmp/ks-wi6-warm-20261019',str(work))
    template,count=re.subn(r'(?m)^(\s*)ollama_url:.*$',r'\1ollama_url: http://127.0.0.1:__OWNED_PORT__',template)
    v.require(count==1,'local provider endpoint not uniquely bound')
    config=q/'config.yaml'
    proxy=ThreadingHTTPServer(('127.0.0.1',0),Proxy);thread=threading.Thread(target=proxy.serve_forever,daemon=True);proxy_started=False
    pilot.BINARY=binary;client=None;rows=[];failure=None;samples=[];stop=threading.Event();sampler=None
    try:
        config.write_text(template.replace('__OWNED_PORT__',str(proxy.server_port)))
        thread.start();proxy_started=True
        client=pilot.Client(q,config)
        def sample():
            while not stop.is_set():
                r=subprocess.run(['/bin/ps','-o','rss=','-p',str(client.proc.pid)],capture_output=True,text=True,timeout=2)
                if r.returncode==0 and r.stdout.strip():samples.append(dict(elapsed_ns=time.perf_counter_ns()-client.start,rss_KiB=int(r.stdout.strip())))
                stop.wait(.25)
        sampler=threading.Thread(target=sample,daemon=True);sampler.start()
        v.write(q/'initialize.json',client.request('initialize',dict(protocolVersion='2024-11-05',capabilities={},clientInfo=dict(name='owned-scale-long-functions',version='1'))))
        v.write(q/'tools.json',client.request('tools/list'))
        def call(phase,name,args,node=None,repeat=None):
            reply=client.request('tools/call',dict(name=name,arguments=args));row=dict(reply,phase=phase,node_id=node,repeat=repeat);rows.append(row);v.write(work/'calls.json',rows);return row
        call('health_before','cks.ops.health',{})
        with lock:v.write(work/'startup-HTTP.json',list(events))
        for node in scope['long_functions']:
            ordinary=dict(node_id=node['node_id'],expected_commit=scope['source_commit'],expected_source_sha256=node['function_sha256'],expected_snapshot_sha256=pins['graph_snapshot_sha256'],limits=scope['ordinary_limits'])
            call('warmup','cks.context.value_dependencies',ordinary,node['node_id'],0)
            small=dict(ordinary,limits={**scope['ordinary_limits'],**scope['small_budget_override']})
            call('small_budget','cks.context.value_dependencies',small,node['node_id'],0)
            for repeat in range(scope['measured_repetitions_per_function']):call('measured','cks.context.value_dependencies',ordinary,node['node_id'],repeat)
        call('health_after','cks.ops.health',{})
    except Exception as error:failure=dict(type=type(error).__name__,message=str(error))
    finally:
        if client is not None:client.close()
        stop.set()
        if sampler is not None:sampler.join(3)
        if proxy_started:proxy.shutdown()
        proxy.server_close()
        if proxy_started:thread.join(3)
        deadline=time.monotonic()+3
        while active['HTTP_handlers']and time.monotonic()<deadline:time.sleep(.05)
        v.write(work/'calls.json',rows);v.write(work/'RSS-samples.json',samples);v.write(work/'HTTP.json',events)
        v.write(work/'original-after.json',pilot.inventory(pilot.DATA));v.write(work/'owned-after.json',pilot.inventory(work/'data'))
        v.write(work/'cleanup.json',dict(active_HTTP_handlers_at_close=active['HTTP_handlers'],proxy_thread_live=thread.is_alive(),sampler_thread_live=sampler.is_alive()if sampler else False,original_owned_payload_preserved=before==pilot.inventory(pilot.DATA)==pilot.inventory(work/'data'),model_manifest_unchanged=v.sha(model.read_bytes())==pins['model_manifest_digest'],failure=failure))
    if failure:raise RuntimeError(failure)
    check_source_inputs()
    v.require(before==pilot.inventory(pilot.DATA)==pilot.inventory(work/'data')and not thread.is_alive()and not sampler.is_alive()and not active['HTTP_handlers'],'scale resources or input preservation failed')
    v.write(work/'receipt.json',dict(value_queries=sum(r['request']['params']['name']=='cks.context.value_dependencies'for r in rows),tool_calls=len(rows),actual_LLM_calls=0,paid_external_calls=0,formal_completed=0,independent_approved=0,capacity_SLA_accepted=False))
    print('owned scale/long-function execution complete',flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--work',type=Path,required=True);p.add_argument('--binary',type=Path,required=True);p.add_argument('--native',type=Path,required=True);a=p.parse_args();run(a.work.resolve(),a.binary.resolve(),a.native.resolve())
