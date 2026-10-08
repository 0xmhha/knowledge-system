#!/usr/bin/env python3
"""Run a held source-index preparation; relay only to local approved Ollama."""
import hashlib,json,os,subprocess,threading,time,urllib.request,urllib.error
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
P=Path('system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08').resolve()
prep=json.loads((P/'preparation.json').read_text());base=Path(prep['base'])
assert str(base).startswith('/private/tmp/ks-n12-proposed-lock-')
records=[];lock=threading.Lock()
class Relay(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):self.forward()
 def do_POST(self):self.forward()
 def forward(self):
  assert self.path in {'/api/tags','/api/version','/api/show','/api/embed'},self.path
  body=self.rfile.read(int(self.headers.get('Content-Length','0')));start=time.monotonic_ns()
  req=urllib.request.Request('http://127.0.0.1:11434'+self.path,data=body if self.command=='POST'else None,method=self.command,headers={'Content-Type':'application/json'})
  row={'method':self.command,'path':self.path,'request_bytes':len(body),'request_sha256':hashlib.sha256(body).hexdigest()}
  if self.path=='/api/embed':
   inputs=json.loads(body);value=inputs.get('input',[])
   row.update({'embedding_inputs':len(value)if isinstance(value,list)else 1,'truncate':inputs.get('truncate'),'model':inputs.get('model')})
   assert inputs.get('model')=='bge-m3:latest'and inputs.get('truncate')is False
  try:
   try:
    with urllib.request.urlopen(req,timeout=600)as r:status=r.status;data=r.read();kind=r.headers.get('Content-Type','application/json')
   except urllib.error.HTTPError as e:status=e.code;data=e.read();kind=e.headers.get('Content-Type','application/json')
   row.update({'status':status,'response_bytes':len(data),'response_sha256':hashlib.sha256(data).hexdigest()})
   if self.path=='/api/embed'and status==200:
    obj=json.loads(data);row.update({'embedding_outputs':len(obj.get('embeddings',[])),'prompt_eval_count':obj.get('prompt_eval_count'),'model_total_duration_ns':obj.get('total_duration'),'model_load_duration_ns':obj.get('load_duration')})
   self.send_response(status);self.send_header('Content-Type',kind);self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
  except Exception as e:
   row['transport_error_type']=type(e).__name__
   self.send_error(502,'local relay transport failure')
  finally:
   row['elapsed_ns']=time.monotonic_ns()-start
   with lock:
    records.append(row)
    with(P/'model-http-observations.jsonl').open('a')as f:f.write(json.dumps(row)+'\n')
server=ThreadingHTTPServer(('127.0.0.1',0),Relay);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
endpoint='http://127.0.0.1:'+str(server.server_port)
argv=[str(base/'bin/cks'),'setup','--src',prep['source'],'--out',prep['dataset_root'],'--project-id',prep['project_id'],'--source-mode','committed','--version','n12-candidate','--hold-for-review','--graph-bin',str(base/'bin/ckg'),'--vector-bin',str(base/'bin/ckv'),'--embedder','ollama','--model-name','bge-m3:latest','--embed-dim','1024','--ollama-url',endpoint,'--query-prefix-policy','registry','--filelist',str(P/'filelist.yaml'),'--vector-build-sources',str(P/'build-sources.json'),'--gate-min-canonical','1','--build-timeout','30m','--progress','json']
env=dict(os.environ,CKV_REQUIRE_COMPLETE_EMBEDDINGS='1')
(P/'setup-command.json').write_text(json.dumps({'argv':argv,'environment_overrides':{'CKV_REQUIRE_COMPLETE_EMBEDDINGS':'1'},'endpoint_scope':'transparent loopback relay to local127.0.0.1:11434; no contents/vectors retained'},indent=2)+'\n')
start=time.monotonic_ns()
with(P/'setup.stdout.jsonl').open('w')as out,(P/'setup.stderr.txt').open('w')as err:
 result=subprocess.run(argv,env=env,stdout=out,stderr=err)
server.shutdown();server.server_close()
embeds=[r for r in records if r['path']=='/api/embed']
(P/'setup-result.json').write_text(json.dumps({'process_exit_code':result.returncode,'elapsed_ns':time.monotonic_ns()-start,'http_attempts':len(records),'embedding_http_attempts':len(embeds),'embedding_input_attempts':sum(r['embedding_inputs']for r in embeds),'successful_embedding_outputs':sum(r.get('embedding_outputs',0)for r in embeds),'embedding_failed_http_attempts':sum(r.get('status')!=200 for r in embeds),'new_DEV_FINAL_queries':0,'human_approved':0,'promotion_requested':False},indent=2)+'\n')
print(json.dumps({'exit_code':result.returncode,'http_attempts':len(records),'embedding_http_attempts':len(embeds)}))
raise SystemExit(result.returncode)
