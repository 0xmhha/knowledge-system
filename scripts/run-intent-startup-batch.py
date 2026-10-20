#!/usr/bin/env python3
"""Owned baseline/batched intent startup and declared provider failure observations."""
import argparse,gzip,importlib.util,json,re,shutil,sqlite3,threading,time,urllib.error,urllib.request
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
import wi6_pair_contract as contract
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('pilot',ROOT/'scripts/run-wi6-pair-pilot.py');pilot=importlib.util.module_from_spec(spec);spec.loader.exec_module(pilot)
SPEC=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/intent-startup-batch-v1/SPEC.json'
BASELINE=Path('/private/tmp/ks-phase76-four-roles-20261019/native/cks')
CANDIDATE=Path('/private/tmp/ks-phase77-candidate-measured-20261019/cks')

def run(work):
 work.mkdir();scope=json.loads(SPEC.read_text());before=pilot.inventory(pilot.DATA);pilot.write(work/'original-before.json',before)
 shutil.copytree(pilot.DATA,work/'data',ignore=lambda d,names:[n for n in names if n.endswith(('.lock','.lease'))]);contract.restore_reader_coordination(work/'data');assert pilot.inventory(work/'data')==before;pilot.write(work/'owned-before.json',before);(work/'empty-live-source').mkdir()
 graph=json.loads((work/'data/graph/manifest.json').read_text());files={r['path']:r['sha256']for r in graph['files']};db=sqlite3.connect((work/'data/graph/graph.db').as_uri()+'?mode=ro',uri=True);ids=db.execute("SELECT id FROM nodes WHERE canonical_id=?",('github.com/ethereum/go-ethereum/eth/downloader.(*SyncStatusSubscription).Unsubscribe',)).fetchall();assert len(ids)==1;node,blob=contract.retained_node(db,ids[0][0],files);db.close();pilot.write(work/'selected-source.json',node);(work/'selected-function.go.txt').write_bytes(blob)
 texts=[json.loads(l.strip().rstrip(','))for l in(ROOT/'internal/system/composer/intent/anchors.go').read_text().splitlines()if re.match(r'^\s*".*",?\s*$',l)];assert len(texts)==65 and len(set(texts))==65;pilot.write(work/'anchor-texts.json',texts)
 binaries={'baseline':BASELINE,'batch':CANDIDATE};arms=[dict(id='baseline',binary='baseline',mode='normal',expected_exit=0,expected_startup_embedding_HTTP=66),dict(id='batch',binary='batch',mode='normal',expected_exit=0,expected_startup_embedding_HTTP=2),dict(id='batch_unavailable',binary='batch',mode='unavailable',expected_exit=1,expected_startup_embedding_HTTP=2),dict(id='batch_short',binary='batch',mode='short',expected_exit=1,expected_startup_embedding_HTTP=2),dict(id='batch_measured',binary='batch',mode='normal',expected_exit=0,expected_startup_embedding_HTTP=2,measure=True),dict(id='batch_measured_unavailable',binary='batch',mode='unavailable',expected_exit=1,expected_startup_embedding_HTTP=2,measure=True)]
 run_scope=dict(attempt='explicit_final_default_and_measurement_wrapper1',previous_attempt='/private/tmp/ks-phase77-startup-runtime-20261019',measurement_addendum_sha256=contract.sha((SPEC.parent/'MEASUREMENT-WRAPPER-ADDENDUM.json').read_bytes()),spec_sha256=contract.sha(SPEC.read_bytes()),spec=scope,arms=arms,binaries={name:dict(path=str(path),sha256=contract.sha(path.read_bytes()))for name,path in binaries.items()},source_commit=graph['src_commit'],project_id=graph['project_id'],dataset_id=graph['dataset_id'],snapshot_id=graph['snapshot_id'],resource_exclusivity=False,model_process='already_warm_not_reset',CKS_process='fresh_per_arm',vectors_compared='upstream_JSON_vectors_matched_by_exact_text_not_request_order',automatic_retries=0,LLM_calls=0,paid_external_calls=0,formal_completed=0,independent_approved=0,tools_sha256={str(p.relative_to(ROOT)):contract.sha(p.read_bytes())for p in(Path(__file__),ROOT/'scripts/run-wi6-pair-pilot.py',ROOT/'scripts/wi6_pair_contract.py')})
 pilot.write(work/'SCOPE.json',run_scope)
 active={'arm':None,'mode':'normal'};lock=threading.Lock();http=[]
 class Proxy(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def relay(self):
   length=int(self.headers.get('Content-Length','0'))
   if length>2*1024*1024:self.send_error(413);return
   body=self.rfile.read(length)if length else b'';request_json=json.loads(body)if body else None
   with lock:arm,mode=active['arm'],active['mode']
   inputs=request_json.get('input',[])if isinstance(request_json,dict)else[];injected=self.path=='/api/embed'and isinstance(inputs,list)and len(inputs)>1 and mode!='normal'
   start=time.perf_counter_ns()
   if injected:
    status=503 if mode=='unavailable'else 200;reply={'error':'owned startup provider failure'}if mode=='unavailable'else{'model':'bge-m3:latest','embeddings':[[0.0]*1024]};payload=json.dumps(reply).encode()
   else:
    request=urllib.request.Request('http://127.0.0.1:11434'+self.path,data=body or None,method=self.command,headers={'Content-Type':'application/json'})
    try:response=urllib.request.urlopen(request,timeout=30)
    except urllib.error.HTTPError as error:response=error
    with response:status=response.status;payload=response.read(8*1024*1024+1)
    if len(payload)>8*1024*1024:self.send_error(502);return
    reply=json.loads(payload)
   row=dict(arm=arm,method=self.command,path=self.path,status=status,forwarded=not injected,request_sha256=contract.sha(body),response_sha256=contract.sha(payload),elapsed_ns=time.perf_counter_ns()-start)
   if self.path=='/api/embed':row.update(request=request_json,response=reply)
   if self.path=='/api/tags':row['bge_model_digests']=[m['digest']for m in reply.get('models',[])if m.get('name')=='bge-m3:latest']
   with lock:http.append(row)
   self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
  do_GET=relay;do_POST=relay
 proxy=ThreadingHTTPServer(('127.0.0.1',0),Proxy);thread=threading.Thread(target=proxy.serve_forever,daemon=True);thread.start();url='http://127.0.0.1:'+str(proxy.server_port);outcomes=[]
 try:
  for arm in arms:
   q=work/arm['id'];q.mkdir();active.update(arm=arm['id'],mode=arm['mode']);pilot.BINARY=binaries[arm['binary']]
   template=(Path('/private/tmp/ks-phase73-model-runtime-corrective-20261019/on/config.yaml')).read_text().replace('/private/tmp/ks-phase73-model-runtime-corrective-20261019',str(work));template,count=re.subn(r'(?m)^(\s*)ollama_url:.*$',r'\1ollama_url: '+url,template);assert count==1;template=template.replace('embedding_cache_entries: 32','embedding_cache_entries: 0')
   if arm.get('measure'):template=template.replace('measure_backend_calls: false','measure_backend_calls: true')
   template=template.replace(str(work/'footprints'),str(q/'footprints'))
   config=q/'config.yaml';config.write_text(template)
   client=pilot.Client(q,config);initialized=False;error=None;response=None
   try:
    init=client.request('initialize',dict(protocolVersion='2024-11-05',capabilities={},clientInfo=dict(name='owned-startup-anchor-batch',version='1')));pilot.write(q/'initialize.json',init);initialized=True
    if arm['expected_exit']==0:
     pilot.write(q/'tools.json',client.request('tools/list'))
     response=client.request('tools/call',dict(name='cks.context.value_dependencies',arguments=dict(node_id=node['node_id'],expected_commit=graph['src_commit'],expected_source_sha256=node['function_sha256'])));pilot.write(q/'source-response.json',response)
   except(RuntimeError,TimeoutError)as exc:error=dict(type=type(exc).__name__,message=str(exc))
   finally:client.close()
   session=json.loads((q/'session.json').read_text());embedding=[r for r in http if r['arm']==arm['id']and r['path']=='/api/embed'];outcome=dict(arm=arm['id'],initialized=initialized,error=error,session_exit=session['exit_code'],startup_embedding_HTTP=len(embedding),expected_exit=arm['expected_exit'],source_response_recorded=response is not None);outcomes.append(outcome);pilot.write(work/'outcomes.json',outcomes);print(json.dumps(outcome),flush=True)
 finally:
  proxy.shutdown();proxy.server_close();thread.join(3);pilot.write(work/'proxy-cleanup.json',dict(thread_alive=thread.is_alive()));pilot.write(work/'http-events.json',http)
 after=pilot.inventory(pilot.DATA);owned_after=pilot.inventory(work/'data');pilot.write(work/'original-after.json',after);pilot.write(work/'owned-after.json',owned_after)
 if before!=after or before!=owned_after:raise ValueError('original/owned data payload changed')
 for name,path in binaries.items():
  if contract.sha(path.read_bytes())!=run_scope['binaries'][name]['sha256']:raise ValueError('binary changed during startup observations')
 pilot.write(work/'receipt.json',dict(planned_arms=6,attempted_arms=len(outcomes),automatic_retries=0,original_owned_data_preserved=True,global_model_modified=False,real_LLM_calls=0,paid_external_calls=0,formal_completed=0,independent_approved=0))

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
