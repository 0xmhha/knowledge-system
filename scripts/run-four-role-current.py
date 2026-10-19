#!/usr/bin/env python3
"""Installed Claude role delivery against current CKS and actual retained source, no real LLM."""
import argparse,copy,gzip,importlib.util,json,os,shlex,shutil,sqlite3,subprocess,sys,threading,urllib.error,urllib.request
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('role_host',ROOT/'scripts/consumer-role-host.py');host=importlib.util.module_from_spec(spec);spec.loader.exec_module(host)
spec=importlib.util.spec_from_file_location('pilot',ROOT/'scripts/run-wi6-pair-pilot.py');pilot=importlib.util.module_from_spec(spec);spec.loader.exec_module(pilot)
import wi6_pair_contract as contract
REVISION='49197f0f3d5967913355278ba5d45d4a32ae614a';CONSUMER=Path('/private/tmp/ks-adopted-consumer-20261017');CONSUMER_REV='d1ebafb8ddf4eb487b8f1d6285135e174c40834b';SOURCE_REV='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8';TICKET='KS-P06-REVIEW'

def source_inventory(src):
 result={}
 for raw in subprocess.check_output(['git','ls-files','--stage','-z'],cwd=src).split(b'\0'):
  if not raw:continue
  header,name=raw.split(b'\t');mode,identity,_=header.decode().split();p=src/name.decode();result[name.decode()]=dict(mode=mode,git_object=identity)if mode=='160000'else dict(mode=mode,sha256=contract.sha(os.readlink(p).encode()if mode=='120000'else p.read_bytes()))
 return result

def get_wire(out):
 rows=[json.loads(l)for l in (out/'mcp-frames.jsonl').read_text().splitlines()]
 replies=[json.loads(r['raw'])for r in rows if r['kind']=='response'and 'result'in json.loads(r['raw'])and 'content'in json.loads(r['raw'])['result']]
 if not replies:raise ValueError('actual MCP response absent before scripted handoff')
 return replies[-1]

class Actions(list):
 def __init__(self,rows,root,out,role,pins):super().__init__(rows);self.root=root;self.out=out;self.role=role;self.pins=pins
 def __getitem__(self,index):
  row=super().__getitem__(index)
  if row.get('derived_handoff'):
   reply=get_wire(self.out);payload=host.payload(reply)
   value=dict(kind='scripted_role_transport_not_independent_semantic_review',role=self.role,scope_binding=self.pins,scope_binding_origin='copied_unchanged_dataset_metadata_not_returned_project_fields',actual_tool_response=payload,formal_completed=0,independent_approved=0,quality_score=None)
   host.dump(self.out/('resolved-handoff-'+str(index)+'.json'),dict(actual_MCP_reply=reply,derived_context=value))
   return dict(name='Write',input=dict(file_path=str(self.root/'source/.coding-agent/tickets'/TICKET/(self.role+'-context.json')),content=json.dumps(value,ensure_ascii=False,indent=2)))
  return row

def run(root):
 if not root.is_dir()or not(root/'native/binary.json').is_file():raise ValueError('fresh clean-HEAD native binary required')
 out=root/'evidence';out.mkdir();binary=root/'native/cks';native=json.loads((root/'native/binary.json').read_text());assert native['source_revision']==REVISION and native['sha256']==contract.sha(binary.read_bytes())and native['clean_source']
 if subprocess.check_output(['git','rev-parse','HEAD'],cwd=CONSUMER,text=True).strip()!=CONSUMER_REV or subprocess.check_output(['git','status','--porcelain'],cwd=CONSUMER):raise ValueError('approved consumer source changed')
 consumer_before=source_inventory(CONSUMER);pilot.write(out/'consumer-before.json',consumer_before)
 before=pilot.inventory(pilot.DATA);pilot.write(out/'original-data-before.json',before)
 shutil.copytree(pilot.DATA,root/'data',ignore=lambda d,names:[n for n in names if n.endswith(('.lock','.lease'))]);contract.restore_reader_coordination(root/'data')
 if pilot.inventory(root/'data')!=before:raise ValueError('copied data differs')
 pilot.write(out/'owned-data-before.json',before);(root/'empty-live-source').mkdir()
 parent=Path('/private/tmp/ks-wi4-target-20261019/source');repo=root/'source';env={k:os.environ[k]for k in ('PATH','HOME','USER','TMPDIR','LANG','LC_ALL')if k in os.environ};env['PYTHONDONTWRITEBYTECODE']='1'
 subprocess.run(['git','-c','core.hooksPath=/dev/null','clone','--shared','--no-checkout',str(parent),str(repo)],check=True,capture_output=True,env=env);subprocess.run(['git','-c','core.hooksPath=/dev/null','checkout','--detach',SOURCE_REV],cwd=repo,check=True,capture_output=True,env=env)
 source_before=source_inventory(repo);pilot.write(out/'target-source-before.json',source_before)
 meta=json.loads((root/'data/dataset-identity.json').read_text());graph=json.loads((root/'data/graph/manifest.json').read_text());files={r['path']:r['sha256']for r in graph['files']}
 db=sqlite3.connect((root/'data/graph/graph.db').as_uri()+'?mode=ro',uri=True);node_id=db.execute("SELECT id FROM nodes WHERE canonical_id=?",('github.com/ethereum/go-ethereum/eth/downloader.(*SyncStatusSubscription).Unsubscribe',)).fetchall();assert len(node_id)==1;node,blob=contract.retained_node(db,node_id[0][0],files);db.close()
 original_api=subprocess.check_output(['git','show',SOURCE_REV+':'+node['path']],cwd=repo);assert original_api[node['start_byte']:node['end_byte_exclusive']]==blob and contract.sha(original_api)==node['file_sha256'];pilot.write(out/'selected-source.json',node);(out/'selected-function.go.txt').write_bytes(blob)
 query=dict(node_id=node['node_id'],expected_commit=SOURCE_REV,expected_source_sha256=node['function_sha256']);pins=dict(project_id=meta['source']['project_id'],dataset_id=meta['dataset_id'],source_commit=SOURCE_REV,snapshot_id=meta['source']['snapshot_id'],canonical_id=node['canonical_id'],node_id=node['node_id'],function_sha256=node['function_sha256'])
 workspace=repo/'.coding-agent/tickets'/TICKET;workspace.mkdir(parents=True)
 state=dict(schema_version=1,ticket_id=TICKET,ticket_type='bugfix',project_id='go-stablenet',pipeline_variant='full',requirement_source='local',current_state='TICKET_INTAKE',states={'DESIGN':{}},config=dict(base_ref=SOURCE_REV,publication_policy='evaluation_only',autonomy=dict(mode='auto',auto_merge=False)))
 host.dump(workspace/'state.json',state)
 policy=[sys.executable,str(CONSUMER/'plugin/scripts/publication_policy.py'),'--workspace',str(workspace),'--repo',str(repo),'--init'];locked=subprocess.run(policy,cwd=repo,env=env,capture_output=True,text=True);pilot.write(out/'publication-lock-invocation.json',dict(argv=policy,exit_code=locked.returncode,stdout=locked.stdout,stderr=locked.stderr));assert locked.returncode==0
 state['current_state']='BLOCKED';state['blocked_reason']='Independent gold/selection not assigned; owned role transport probe only';host.dump(workspace/'state.json',state);host.dump(out/'initial-state.json',state)
 host.P05=root/'current-binary-binding';host.P05.mkdir();host.dump(host.P05/'execution.json',dict(binary_sha256={'cks':native['sha256']}));host.BINARY=binary;host.CANDIDATE=CONSUMER
 host.ActionPlan=lambda rows,out,workspace:rows  # Preserve Actions.__getitem__; old wrapper copies dynamic entries into plain rows.
 http_events=[];lock=threading.Lock()
 class Proxy(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def relay(self):
   length=int(self.headers.get('Content-Length','0'))
   if length>1024*1024:self.send_error(413);return
   body=self.rfile.read(length)if length else None
   if self.path not in ('/api/tags','/api/show','/api/embed','/api/embeddings'):self.send_error(404);return
   req=urllib.request.Request('http://127.0.0.1:11434'+self.path,data=body,method=self.command,headers={'Content-Type':'application/json'})
   try:response=urllib.request.urlopen(req,timeout=15)
   except urllib.error.HTTPError as error:response=error
   with response:
    payload=response.read(8*1024*1024+1)
    if len(payload)>8*1024*1024:self.send_error(502);return
    with lock:http_events.append(dict(method=self.command,path=self.path,status=response.status,request_sha256=contract.sha(body or b''),response_sha256=contract.sha(payload)))
    self.send_response(response.status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
  do_GET=relay;do_POST=relay
 proxy=ThreadingHTTPServer(('127.0.0.1',0),Proxy);thread=threading.Thread(target=proxy.serve_forever,daemon=True);thread.start();url='http://127.0.0.1:'+str(proxy.server_port)
 template=(Path('/private/tmp/ks-phase73-model-runtime-corrective-20261019/on/config.yaml')).read_text();template=template.replace('/private/tmp/ks-phase73-model-runtime-corrective-20261019',str(root));import re
 template,count=re.subn(r'(?m)^(\s*)ollama_url:.*$',r'\1ollama_url: '+url,template);assert count==1;template=template.replace('embedding_cache_entries: 32','embedding_cache_entries: 0');config=root/'cks.yaml';config.write_text(template)
 alias=lambda name:'mcp__plugin_coding-agent_cks__cks_context_'+name
 negative=copy.deepcopy(state);negative['current_state']='COMPLETED';denied=dict(name='Write',input=dict(file_path=str(workspace/'state.json'),content=json.dumps(negative)))
 selection=[sys.executable,str(CONSUMER/'plugin/scripts/fix_selection.py'),'--workspace',str(workspace),'--repo',str(repo),'--check']
 plans={
 'analyzer':[dict(name=alias('value_dependencies'),input=query),dict(derived_handoff=True),denied],
 'planner':[dict(name='Read',input={'file_path':str(workspace/'analyzer-context.json')}),dict(name=alias('rule_candidates'),input=dict(function=query,rule_pack='go-local-review@1.0.0')),dict(derived_handoff=True),denied],
 'implementer':[dict(name='Read',input={'file_path':str(workspace/'planner-context.json')}),dict(name='Bash',input=dict(command=shlex.join(selection))),denied],
 'evaluator':[dict(name='Read',input={'file_path':str(workspace/'planner-context.json')}),dict(name='Bash',input=dict(command=shlex.join(selection+['--applied']))),denied]}
 scope=dict(attempt='explicit_corrective1_preserve_dynamic_action_accessor',previous_attempt='/private/tmp/ks-phase76-four-roles-20261019',adapter_override='only preserve typed dynamic action resolver; actual host/MCP/hooks unchanged',kind='four_actual_Claude_roles_current_CKS_scripted_transport_not_model_quality',binary=native,consumer_revision=CONSUMER_REV,host_sha256=host.sha(host.HOST),roles=list(plans),source_pins=pins,CKV_provider='local_BGE_M3',embedding_identity=meta['embedding_identity'],actual_LLM_inference=False,paid_or_external_LLM_calls=0,global_plugin_registration=False,actual_pipeline_completion=False,independent_approved=0,formal_completed=0,FINAL=0,automatic_retries=0,tools_sha256={str(p.relative_to(ROOT)):contract.sha(p.read_bytes())for p in (Path(__file__),ROOT/'scripts/consumer-role-host.py',ROOT/'scripts/operational-host-probe.py',ROOT/'scripts/run-wi6-pair-pilot.py',ROOT/'scripts/wi6_pair_contract.py')})
 pilot.write(out/'SCOPE.json',scope);pilot.write(out/'plans.json',plans);results=[]
 # Preserve the actual adapter source and explicit current-binary binding; never edit old P05 evidence.
 try:
  for role,plan in plans.items():
   case='P76-'+role;actual_out=root/case;actions=Actions(plan,root,actual_out,role,pins);result=host.run(root,case,role,actions,config=config)
   receipt=json.loads((result/'exit.json').read_text());results.append(receipt);pilot.write(out/'role-receipts.json',results);print(json.dumps(dict(role=role,host_exit=receipt['host_exit'],forced=receipt['forced_host_termination'])),flush=True)
 finally:
  proxy.shutdown();proxy.server_close();thread.join(3);pilot.write(out/'http-events.json',http_events);pilot.write(out/'proxy-cleanup.json',dict(thread_alive=thread.is_alive()))
 after=pilot.inventory(pilot.DATA);pilot.write(out/'original-data-after.json',after);owned_after=pilot.inventory(root/'data');pilot.write(out/'owned-data-after.json',owned_after)
 target_after=source_inventory(repo);pilot.write(out/'target-source-after.json',target_after);consumer_after=source_inventory(CONSUMER);pilot.write(out/'consumer-after.json',consumer_after)
 if before!=after or before!=owned_after or source_before!=target_after or consumer_before!=consumer_after:raise ValueError('original/owned source or dataset changed')
 final=json.loads((workspace/'state.json').read_text());pilot.write(out/'final-state.json',final)
 if final!=state:raise ValueError('completion attempt changed blocked unapproved state')
 for role in ('analyzer','planner'):
  handoff=json.loads((workspace/(role+'-context.json')).read_text())
  if handoff['scope_binding']!=pins or handoff['independent_approved']!=0 or handoff['formal_completed']!=0:raise ValueError('actual handoff absent, altered or promoted')
 pilot.write(out/'receipt.json',dict(planned_roles=4,attempted_roles=len(results),actual_LLM_calls=0,paid_or_external_LLM_calls=0,independent_approved=0,formal_completed=0,original_owned_preserved=True,global_plugin_registered=False,pipeline_completed=False))

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
