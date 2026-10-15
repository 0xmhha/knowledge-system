#!/usr/bin/env python3
"""Execute the signed 16-question four-arm study with a local API meter."""
import argparse
import copy
import hashlib
import http.client
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import threading
import time
import urllib.request
from urllib.parse import urlsplit
import yaml
ROOT=Path(__file__).resolve().parents[1]
ARMS=['off','concept_text','relations','combined']


def sha(data):return hashlib.sha256(data).hexdigest()
def write(path,v):path.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')

class LocalMeter:
    def __init__(self,root):
        self.root=root;self.label='initialization';self.rows=[];self.lock=threading.Lock();meter=self
        class Handler(BaseHTTPRequestHandler):
            protocol_version='HTTP/1.1'
            def log_message(self,*args):pass
            def do_GET(self):self.forward()
            def do_POST(self):self.forward()
            def forward(self):
                path=urlsplit(self.path)
                if path.scheme or path.netloc or path.path not in('/api/tags','/api/show','/api/embeddings','/api/embed','/api/version'):
                    self.send_error(400);return
                length=int(self.headers.get('Content-Length','0'))
                if not 0<=length<=8<<20:self.send_error(413);return
                body=self.rfile.read(length);data=json.loads(body)if body else{};model=data.get('model',data.get('name'))
                if model is not None and model not in('bge-m3','bge-m3:latest'):self.send_error(400);return
                with meter.lock:label=meter.label
                start=time.monotonic_ns();status=502;payload=b'{}';usage={};error=None
                connection=http.client.HTTPConnection('127.0.0.1',11434,timeout=35)
                try:
                    connection.request(self.command,self.path,body=body,headers={'Content-Type':'application/json'});response=connection.getresponse();status=response.status;payload=response.read(16<<20)
                    try:decoded=json.loads(payload)
                    except ValueError:decoded={}
                    for key in('prompt_eval_count','total_duration','load_duration','prompt_eval_duration'):
                        if type(decoded.get(key))is int:usage[key]=decoded[key]
                except(OSError,http.client.HTTPException)as e:error=type(e).__name__;payload=json.dumps({'error':'local_meter_upstream_failed'}).encode()
                finally:connection.close()
                row={'batch':label,'method':self.command,'path':self.path,'model':model,'request_sha256':sha(body),'request_bytes':len(body),'status':status,'response_bytes':len(payload),'elapsed_ns':time.monotonic_ns()-start,'server_usage':usage,'error':error}
                with meter.lock:
                    meter.rows.append(row);temp=meter.root/'api-meter.pending';write(temp,meter.rows);temp.replace(meter.root/'api-meter.json')
                self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers()
                try:self.wfile.write(payload)
                except(BrokenPipeError,ConnectionResetError):pass
        self.server=ThreadingHTTPServer(('127.0.0.1',0),Handler);self.thread=threading.Thread(target=self.server.serve_forever,daemon=True);self.thread.start();self.url='http://127.0.0.1:'+str(self.server.server_port)
    def set_label(self,label):
        with self.lock:self.label=label
    def close(self):self.server.shutdown();self.server.server_close();self.thread.join(timeout=5)


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--prepared',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();prepared=args.prepared.resolve();out=args.out.resolve()
    if out.exists():raise ValueError('fresh output required')
    out.mkdir();relative=prepared.relative_to(ROOT);revision=subprocess.check_output(['git','log','-1','--format=%H','--',str(relative.parent/'manifest.json')],cwd=ROOT).decode().strip()
    signature=subprocess.run(['git','verify-commit',revision],cwd=ROOT,capture_output=True);(out/'preregistered-signature.txt').write_bytes(signature.stdout+signature.stderr);signature.check_returncode()
    destination=out/'preregistered';destination.mkdir()
    for p in prepared.glob('*.json'):
        committed=subprocess.check_output(['git','show',revision+':'+p.relative_to(ROOT).as_posix()],cwd=ROOT)
        if p.read_bytes()!=committed:raise ValueError('preregistered input changed')
        (destination/p.name).write_bytes(committed)
    protocol=json.loads((destination/'protocol.json').read_text());pins=json.loads((destination/'source-pins.json').read_text());public=json.loads((destination/'requests.json').read_text());version=Path(pins['version_dir']);repo=Path(pins['source_repo']);binary=ROOT/'bin/cks'
    if sha(binary.read_bytes())!=pins['binary_sha256']:raise ValueError('binary changed after preregistration')
    for name,digest in pins['preserved_files'].items():
        if sha(Path(name).read_bytes())!=digest:raise ValueError('source dataset changed before execution')
    write(out/'input-binding.json',{'preregistered_revision':revision,'signed_preregistration_verified':True,'preregistered_manifest':str(relative.parent/'manifest.json'),'input_sha256':{p.name:sha(p.read_bytes())for p in destination.iterdir()},'binary_sha256':pins['binary_sha256']})
    started=time.monotonic();commands=[];meter=LocalMeter(out)
    def run(label,command,timeout=180):
        if time.monotonic()-started>1800:raise ValueError('study wall-clock budget exhausted')
        meter.set_label(label);before=time.monotonic();r=subprocess.run([str(v)for v in command],cwd=ROOT,capture_output=True,timeout=timeout);(out/(label+'.stdout.txt')).write_bytes(r.stdout);(out/(label+'.stderr.txt')).write_bytes(r.stderr);commands.append({'label':label,'command':[str(v)for v in command],'exit_code':r.returncode,'elapsed_seconds':time.monotonic()-before});write(out/'commands.json',commands);r.check_returncode();return r
    def model(label):
        meter.set_label(label)
        with urllib.request.urlopen(meter.url+'/api/tags',timeout=10)as response:models=json.load(response)['models']
        selected=next(r for r in models if r['name']=='bge-m3:latest')
        with urllib.request.urlopen(meter.url+'/api/version',timeout=10)as response:server=json.load(response)
        if selected['digest']!=protocol['model']['digest']:raise ValueError('model bytes differ')
        return {'name':selected['name'],'digest':selected['digest'],'server_version':server['version']}
    try:
        before=model('model-before');write(out/'model-before.json',before)
        store=out/'semantic.sqlite';projection=out/'projection.json'
        run('semantic-build',[binary,'semantic','build','--repo',repo,'--project-id',pins['project_id'],'--dataset-id',pins['dataset_id'],'--graph',version/'graph','--vector',version/'vector','--version-dir',version,'--store',store,'--out',projection,'--ontology','docs/approved-meaning/ontology.yaml'])
        projected=json.loads(projection.read_text())
        if len(projected['concepts'])!=4 or projected.get('assertions'):raise ValueError('semantic scope changed')
        configs={};normalized=[]
        for arm in ARMS:
            path=out/(arm+'.yaml');run('config-'+arm,[binary,'mcp','gen-config','--dataset-dir',version,'--source-root',repo,'--name','ontology-expanded-development','--semantic-store',store,'--ontology-mode',arm,'--ontology-budget-ms','5000','--embed-model','bge-m3:latest','--embed-dim','1024','--ollama-url',meter.url,'--sanitize-rules',ROOT/'system/policies/sanitization_rules.yaml','--out',path]);cfg=yaml.safe_load(path.read_text());cfg['listen'].update(transport='stdio',mcp_stdio=True);cfg['backends']['ckv']['provider']='ollama';cfg['logging']['level']='error';path.write_text(yaml.safe_dump(cfg,sort_keys=False,allow_unicode=True));same=copy.deepcopy(cfg);same['semantic'].pop('ontology_mode');normalized.append(same);configs[arm]=path
        if any(c!=normalized[0]for c in normalized):raise ValueError('non-mode config differs')
        write(out/'normalized-configuration.json',normalized[0]);write(out/'runtime-protocol.json',{'preregistered_revision':revision,'model':before,'arms':ARMS,'counts':protocol['counts'],'planned_capture_calls':512,'semantic_sha256':sha(store.read_bytes()),'binary_sha256':sha(binary.read_bytes()),'proxy_url':meter.url,'quality_acceptance':False,'LLM_answer_calls':0,'paid_model_calls':0,'mode_defaults_changed':False})
        spec=importlib.util.spec_from_file_location('expanded_stdio',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire);ready={}
        for arm in ARMS:
            d=out/('readiness-'+arm);d.mkdir();meter.set_label('readiness-'+arm);client=wire.Client(None,None,d,command=[str(binary),'mcp','--config',str(configs[arm])],namespace='cks.semantic.')
            try:
                client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'expanded-readiness','version':'1'}});status=client.tool('status',{});ready[arm]=status
                if status['availability']!='ready' or not status['read_tools_ready']or status['mode']!=arm or status['counts']['verified_concepts']!=4 or status['counts']['verified_implementations']!=0:raise ValueError('semantic readiness/scope differs')
            finally:client.close();client.proc.stdout.close()
            write(out/'readiness.json',ready)
        print('four arms ready; signed inputs preserved; capture begins',flush=True);inventory=[]
        for index,q in enumerate(public['requests']):
            request=out/(q['id']+'-request.json');write(request,{'schema_version':1,'requests':[q]});order=ARMS[index%4:]+ARMS[:index%4]
            for arm in order:
                label=q['id']+'-'+arm;path=out/(label+'.json');run(label,[binary,'eval','capture','--config',configs[arm],'--requests',request,'--output',path,'--warmup','1','--retrieval-runs','2','--warm-runs','5','--cold-runs','0','--call-timeout','30s'],timeout=360);report=json.loads(path.read_text())
                if report['state']!='captured' or len(report['rows'])!=8 or report['counts']!=protocol['counts']or report['quality_metrics']is not None:raise ValueError('capture incomplete')
                inventory.append({'question':q['id'],'arm':arm,'file':path.name,'rows':8});write(out/'capture-inventory.json',inventory)
            print(q['id']+': four arms captured '+str(len(inventory)*8)+'/512',flush=True)
        after=model('model-after');write(out/'model-after.json',after)
        if before!=after:raise ValueError('model/server changed during experiment')
        for name,digest in pins['preserved_files'].items():
            if sha(Path(name).read_bytes())!=digest:raise ValueError('source dataset mutated')
        runtime=json.loads((out/'runtime-protocol.json').read_text())
        if sha(store.read_bytes())!=runtime['semantic_sha256']or sha(binary.read_bytes())!=pins['binary_sha256']:raise ValueError('semantic/binary changed')
        write(out/'preservation.json',{'dataset_files_unchanged':True,'semantic_unchanged':True,'binary_unchanged':True,'source_paths':pins['preserved_files']});write(out/'summary.json',{'stage':'captured_not_scored_or_quality_accepted','question_count':16,'capture_files':64,'actual_capture_calls':512,'readiness_calls':4,'capture_sessions_closed':64,'readiness_sessions_EOF0':4,'paid_model_calls':0,'LLM_answer_calls':0,'quality_acceptance':False,'independent_acceptance':False});print('captured512 calls; source preserved; scoring remains',flush=True)
    finally:meter.close();write(out/'meter-closure.json',{'listener_closed':not meter.thread.is_alive(),'recorded_requests':len(meter.rows)})

if __name__=='__main__':main()
