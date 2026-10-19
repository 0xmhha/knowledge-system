"""Current-source actual role transport; no model, oracle or acceptance promotion."""
import argparse,gzip,json,subprocess,urllib.parse
from pathlib import Path
import full_host_owner as owner
import native_reliability as native
ROOT=owner.ROOT;PACKET=owner.PACKET.parent/'four-role-current-v1';REVISION='49197f0f3d5967913355278ba5d45d4a32ae614a'
ROLES=['analyzer','planner','implementer','evaluator'];CONSUMER_REV='d1ebafb8ddf4eb487b8f1d6285135e174c40834b'
TOOLS=['scripts/'+n for n in ('four_role_readiness.py','test_four_role_readiness.py','run-four-role-current.py','consumer-role-host.py','operational-host-probe.py','run-wi6-pair-pilot.py','wi6_pair_contract.py','native_reliability.py')]
read,raw,sha,write=owner.read,owner.raw,owner.sha,owner.write
PATHS=native.PATHS

def events(data):return [json.loads(line)for line in data.splitlines()if line.strip()]
def tool_results(rows):
 actions={};results={}
 for row in rows:
  for block in row.get('scripted_response',{}).get('content',[]):
   if block.get('type')=='tool_use':
    if not isinstance(block.get('name'),str)or not isinstance(block.get('input'),dict):raise ValueError('scripted gateway generated an invalid tool block')
    actions[block['id']]=block
  for message in row['body'].get('messages',[]):
   content=message.get('content',[])
   if isinstance(content,list):
    for block in content:
     if block.get('type')=='tool_result':results[block['tool_use_id']]=block
 return actions,results

def text_content(result):
 content=result.get('content','')
 return content if isinstance(content,str)else'\n'.join(x.get('text','')for x in content if isinstance(x,dict))

def historical(root):
 p=root/owner.PACKET;original=subprocess.check_output(['git','show',REVISION+':'+str(owner.PACKET/'manifest.json')],cwd=root);m=json.loads(original)
 if (p/'manifest.json').read_bytes()!=original or m['artifacts']!={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'}:raise ValueError('past real owner evidence changed')
 for name,digest in {**m['tools'],**m['proposal_inputs']}.items():
  if sha(subprocess.check_output(['git','show',REVISION+':'+name],cwd=root))!=digest:raise ValueError('past owner tool/proposal differs from own Git receipt')
 return True

def inspect_attempt(p,initial=False):
 scope=read(p/'evidence/SCOPE.json.gz');pins=scope['source_pins'];node=read(p/'evidence/selected-source.json.gz')
 if scope['consumer_revision']!=CONSUMER_REV or scope['roles']!=ROLES or scope['actual_LLM_inference']is not False or scope['paid_or_external_LLM_calls']!=0 or scope['independent_approved']!=0 or scope['formal_completed']!=0 or scope['global_plugin_registration']is not False:raise ValueError('actual host source or scope promoted')
 if pins['source_commit']!=owner.factory.COMMIT or pins['canonical_id']!='github.com/ethereum/go-ethereum/eth/downloader.(*SyncStatusSubscription).Unsubscribe'or node['node_id']!=pins['node_id']or node['function_sha256']!=sha(raw(p/'evidence/selected-function.go.txt.gz')):raise ValueError('actual retained function source differs')
 metadata=read(p/'dataset-identity.json.gz');graph=read(p/'graph-manifest.json.gz');inventory=read(p/'evidence/owned-data-before.json.gz')
 if sha(raw(p/'dataset-identity.json.gz'))!=inventory['dataset-identity.json']or sha(raw(p/'graph-manifest.json.gz'))!=inventory['graph/manifest.json']:raise ValueError('actual dataset metadata not bound to preserved payload')
 if metadata['dataset_id']!=pins['dataset_id']or metadata['source']['project_id']!=pins['project_id']or metadata['source']['source_commit']!=pins['source_commit']or metadata['source']['snapshot_id']!=pins['snapshot_id']or graph['dataset_id']!=pins['dataset_id']or graph['project_id']!=pins['project_id']:raise ValueError('enriched project/dataset pins differ from actual metadata')
 if read(p/'empty-live-source.json')['files']!=[]:raise ValueError('source fallback directory not empty')
 for kind in ('original-data','owned-data','target-source','consumer'):
  if read(p/'evidence'/(kind+'-before.json.gz'))!=read(p/'evidence'/(kind+'-after.json.gz')):raise ValueError('original/owned data or consumer source changed')
 state=read(p/'evidence/initial-state.json.gz')
 if state!=read(p/'evidence/final-state.json.gz')or state['current_state']!='BLOCKED'or state['config']['publication_policy']!='evaluation_only':raise ValueError('unapproved completion changed fixture state')
 if read(p/'evidence/proxy-cleanup.json.gz')['thread_alive']:raise ValueError('owned model proxy thread remains')
 result=[];tools_expected={'analyzer':'cks.context.value_dependencies','planner':'cks.context.rule_candidates'}
 for role in ROLES:
  q=p/('P76-'+role);r=read(q/'exit.json.gz');inv=read(q/'invocation.json.gz');env=inv['env'];endpoint=urllib.parse.urlparse(env['ANTHROPIC_BASE_URL'])
  if r['host_exit']!=0 or not r['host_reaped']or r['forced_host_termination']or not r['responder_closed']or not r['responder_thread_joined']or r['real_model_calls']!=0 or endpoint.hostname!='127.0.0.1'or endpoint.port!=r['responder_port']or inv['api_credential_kind']!='constant_fake_local_token':raise ValueError('actual host termination or local-only fixture changed')
  if inv['host_sha256']!=scope['host_sha256']or inv['cks_sha256']!=scope['binary']['sha256']or inv['role']!=role or inv['model_kind']!='local_scripted_transport_no_real_inference':raise ValueError('host/role/binary identity changed')
  argv=inv['argv'];agent=argv[argv.index('--agent')+1]
  if agent!='coding-agent:'+role or '--strict-mcp-config'not in argv or '--no-session-persistence'not in argv or '--no-chrome'not in argv:raise ValueError('actual role/config selection changed')
  frames=events(raw(q/'mcp-frames.jsonl.gz'));exits=[x for x in frames if x['kind']=='exit']
  if len(exits)!=1 or exits[0]['exit_code']!=0 or exits[0]['terminate_sent']or exits[0]['forced_kill']or not exits[0]['child_reaped']:raise ValueError('actual CKS stdio not normally closed')
  requests=[json.loads(x['raw'])for x in frames if x['kind']=='request'];calls=[x for x in requests if x.get('method')=='tools/call']
  if [x['params']['name']for x in calls]!=([tools_expected[role]]if role in tools_expected else[]):raise ValueError('role made undeclared/missing CKS tool call')
  gateways=read(q/'gateway-requests.json.gz')
  first=next(x for x in gateways if '/messages'in x['path']and'count_tokens'not in x['path']);catalog=[t['name']for t in first['body'].get('tools',[])];mcp_tools=[n for n in catalog if n.startswith('mcp__')]
  if len(catalog)!=dict(analyzer=25,planner=19,implementer=4,evaluator=4)[role]or len(mcp_tools)!=dict(analyzer=21,planner=15,implementer=0,evaluator=0)[role]:raise ValueError('actual role tool catalog changed')
  try:actions,results=tool_results(gateways)
  except ValueError:
   if not initial:raise
   result.append(dict(role=role,host_exit=0,classification='invalid_scripted_gateway_tool_block_despite_host_exit0',handoff_verified=False));continue
  ordered=[actions[k]for k in sorted(actions,key=lambda s:int(s.rsplit('_',1)[1]))]
  expected_names={'analyzer':['mcp__plugin_coding-agent_cks__cks_context_value_dependencies','Write','Write'],'planner':['Read','mcp__plugin_coding-agent_cks__cks_context_rule_candidates','Write','Write'],'implementer':['Read','Bash','Write'],'evaluator':['Read','Bash','Write']}[role]
  if [x['name']for x in ordered]!=expected_names or len(results)!=len(actions):raise ValueError('role script requests/results incomplete')
  completed=[k for k,a in actions.items()if a['name']=='Write'and Path(a['input']['file_path']).name=='state.json']
  if len(completed)!=1 or results[completed[0]].get('is_error')is not True or 'evaluation_only cannot enter publication/completion'not in text_content(results[completed[0]]):raise ValueError('actual premature completion denial missing')
  reads=[results[k]for k,a in actions.items()if a['name']=='Read']
  if not initial:
   for value in reads:
    text=text_content(value)
    if value.get('is_error')or pins['function_sha256']not in text or pins['canonical_id']not in text or '"status": "partial"'not in text or '"independent_approved": 0'not in text:raise ValueError('downstream Read lost actual source, partial or unapproved status')
  if role in tools_expected and not initial:
   response=[json.loads(x['raw'])for x in frames if x['kind']=='response'and json.loads(x['raw']).get('id')==calls[0]['id']]
   if len(response)!=1:raise ValueError('actual MCP reply missing/duplicated')
   value=response[0]['result'].get('structuredContent')or json.loads(response[0]['result']['content'][0]['text']);source=value['analysis']['source']
   if value['status']!='partial'or source['node_id']!=pins['node_id']or source['sha256']!=pins['function_sha256']or source['commit']!=pins['source_commit']:raise ValueError('actual partial source result changed')
   saved=read(q/'workspace-after'/(role+'-context.json.gz'))
   if saved['actual_tool_response']!=value or saved['scope_binding']!=pins or saved['scope_binding_origin']!='copied_unchanged_dataset_metadata_not_returned_project_fields'or saved['independent_approved']!=0 or saved['formal_completed']!=0 or saved['quality_score']is not None:raise ValueError('actual handoff changed or promoted tool output')
   writing=[k for k,a in actions.items()if a['name']=='Write'and Path(a['input']['file_path']).name==role+'-context.json']
   if len(writing)!=1 or json.loads(actions[writing[0]]['input']['content'])!=saved or results[writing[0]].get('is_error'):raise ValueError('host did not actually write the saved handoff')
  if role in ('implementer','evaluator'):
   bash=[k for k,a in actions.items()if a['name']=='Bash']
   if len(bash)!=1 or results[bash[0]].get('is_error')is not True or 'Exit code 2'not in text_content(results[bash[0]])or 'selection artifact missing, linked or too large'not in text_content(results[bash[0]]):raise ValueError('actual selection CLI did not reject absent oracle selection')
   if '--check'not in actions[bash[0]]['input']['command']or ('--applied'in actions[bash[0]]['input']['command'])!=(role=='evaluator'):raise ValueError('actual check/applied CLI path missing')
  start=next(x['time']for x in frames if x['kind']=='start');first_reply=next(x['time']for x in frames if x['kind']=='response')
  result.append(dict(role=role,host_exit=0,actual_CKS_exit=0,tool_catalog=len(catalog),CKS_tool_calls=len(calls),first_CKS_response_seconds=first_reply-start,partial_source_handoff_verified=not initial and role in tools_expected,actual_read_verified=not initial and bool(reads),completion_refused=True,selection_CLI_refused=role in('implementer','evaluator'),quality_score=None))
 return result

def derive(root=ROOT,packet=None):
 p=packet or root/PACKET;historic=historical(root);scope=read(p/'actual/evidence/SCOPE.json.gz');b=read(p/'native/binary.json.gz');binding=read(p/'native/source-binding.json.gz')
 if b['source_revision']!=REVISION or not b['clean_source']or binding['source_revision']!=REVISION or binding['current_clean']is not True or scope['binary']['sha256']!=b['sha256']:raise ValueError('fresh clean committed CKS identity differs')
 for n,h in {**binding['native_binding']['source_sha256'],**binding['native_binding']['approved_input_sha256']}.items():
  if sha((root/n).read_bytes())!=h:raise ValueError('current native input changed')
 info=raw(p/'native/build-info.txt.gz').decode()
 if 'go1.26.8'not in info or 'vcs.revision='+REVISION not in info or 'vcs.modified=false'not in info:raise ValueError('current clean binary build metadata missing')
 record=read(p/'native/build/receipt.json.gz')
 if record['exit_code']!=0 or record['timeout']or record['term_sent']or record['kill_sent']or not record['reaped']or record['remaining_live_group_members']:raise ValueError('fresh CKS build failed or remains')
 for n in('stdout','stderr'):
  if sha(raw(p/'native/build'/(n+'.gz')))!=record[n+'_sha256']:raise ValueError('native build log changed')
 for name,digest in scope['tools_sha256'].items():
  if sha((root/name).read_bytes())!=digest:raise ValueError('frozen actual role transport tool changed')
 current=inspect_attempt(p/'actual');initial=inspect_attempt(p/'initial',initial=True)
 if [x['role']for x in initial if x.get('classification')=='invalid_scripted_gateway_tool_block_despite_host_exit0']!=['analyzer','planner']:raise ValueError('initial failed handoff was omitted or promoted')
 embedding_requests={}
 for label in('initial','actual'):
  http=read(p/label/'evidence/http-events.json.gz')
  if any(r['status']!=200 for r in http):raise ValueError('undeclared local embedding HTTP failure')
  embedding_requests[label]=sum(r['path']in('/api/embed','/api/embeddings')for r in http)
 cleanup=read(p/'process-cleanup.json')
 if any(x['remaining_recorded_PIDs']or x['remaining_host_group_members']for x in cleanup):raise ValueError('owned host/proxy/CKS process remains')
 return dict(kind='four_actual_roles_current_CKS_transport_not_model_quality',clean_CKS_revision=REVISION,consumer_revision=CONSUMER_REV,actual_roles=4,initial_role_hosts=4,corrective_role_hosts=4,total_role_hosts=8,total_CKS_sessions=8,initial_gateway_handoff_failures=2,explicit_corrective_attempts=1,automatic_retries=0,source_project='go-stablenet',source_commit=owner.factory.COMMIT,current_actual_partial_MCP_calls=2,current_actual_context_writes=2,current_downstream_Reads=3,current_selection_check_refusals=2,current_completion_refusals=4,all_original_owned_data_and_consumer_source_preserved=True,owned_recorded_PIDs_remaining=0,owned_host_groups_remaining=0,proxy_threads_remaining=0,actual_LLM_inferences=0,local_embedding_HTTP_requests=embedding_requests,embedding_inference_items=None,paid_or_external_LLM_calls=0,host_USD_counters='nominal_scripted_usage_not_paid_billing',global_plugin_registration=False,full_pipeline_or_role_semantic_acceptance=False,independent_approved=0,formal_completed=0,FINAL=0,freeze=None,execution_allowed=False,historical_real_owner_bytes_verified=historic,roles=current,initial_roles=initial)

def refs(root=ROOT):return dict(kind='four_actual_roles_current_CKS_transport_not_model_quality',manifest=str(PACKET/'manifest.json'),manifest_sha256=sha((root/PACKET/'manifest.json').read_bytes()),independent_approved=0,formal_completed=0)
def update_indices():
 ledger=Path('docs/spec-driven/REMAINING-13-EXECUTION.json');worklist=dict(path=str(ledger),sha256=sha((ROOT/ledger).read_bytes()))
 for name in PATHS:
  d=read(ROOT/name);d['latest_consumer_role_followup']=refs();d['active_remaining_worklist']=worklist;write(ROOT/name,d)
 native.update_indices()
def seal(p):write(p/'manifest.json',dict(kind='four_role_current_DEV',tools={n:sha((ROOT/n).read_bytes())for n in TOOLS},artifacts={str(x.relative_to(p)):sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x.name!='manifest.json'}))
def check(root=ROOT,packet=None):
 p=packet or root/PACKET
 try:
  m=read(p/'manifest.json')
  if m['tools']!={n:sha((root/n).read_bytes())for n in TOOLS}or m['artifacts']!={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'}:raise ValueError('role evidence or tools changed')
  if read(p/'SUMMARY.json')!=derive(root,p):raise ValueError('role/host evidence or acceptance boundary changed')
  if packet is None:
   ledger=Path('docs/spec-driven/REMAINING-13-EXECUTION.json');expected=dict(path=str(ledger),sha256=sha((root/ledger).read_bytes()))
   if any(read(root/n).get('latest_consumer_role_followup')!=refs(root)or read(root/n).get('active_remaining_worklist')!=expected for n in PATHS):raise ValueError('current role/worklist indices differ')
  return []
 except(ValueError,KeyError,OSError,TypeError,IndexError,StopIteration)as error:return [str(error)]
def prepare(actual,initial,native_dir):
 p=ROOT/PACKET
 if p.exists():raise ValueError('refusing packet overwrite')
 p.mkdir()
 for label,source in [('actual',actual),('initial',initial)]:
  for top in [source/'evidence',*[source/('P76-'+r)for r in ROLES]]:
   for x in top.rglob('*'):
    if not x.is_file()or x.is_symlink():continue
    # Isolated CLI profiles, temp files and copied dataset/source are not deliverables.
    parts=x.relative_to(source).parts
    if 'claude-config'in parts or 'tmp'in parts or any(n.endswith('.lock')for n in parts):continue
    target=p/label/(str(x.relative_to(source))+'.gz');target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(gzip.compress(x.read_bytes(),mtime=0))
  for name in('cks.yaml','current-binary-binding/execution.json','data/dataset-identity.json','data/graph/manifest.json'):
   source_file=source/name;artifact='dataset-identity.json'if name=='data/dataset-identity.json'else'graph-manifest.json'if name=='data/graph/manifest.json'else name
   target=p/label/(artifact+'.gz');target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(gzip.compress(source_file.read_bytes(),mtime=0))
  write(p/label/'empty-live-source.json',dict(files=sorted(str(x.relative_to(source/'empty-live-source'))for x in(source/'empty-live-source').rglob('*')if x.is_file())))
 for x in native_dir.rglob('*'):
  if not x.is_file()or x.name in('cks','live.json'):continue
  target=p/'native'/(str(x.relative_to(native_dir))+'.gz');target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(gzip.compress(x.read_bytes(),mtime=0))
def finalize():
 p=ROOT/PACKET;write(p/'SUMMARY.json',derive());seal(p)
def stored():
 r=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip();m=read(ROOT/PACKET/'manifest.json');names=[str(PACKET/'manifest.json'),*[str(PACKET/n)for n in m['artifacts']],*TOOLS,*[str(n)for n in PATHS],*[str(Path('docs/spec-driven')/n)for n in ('CURRENT-HANDOFF.md','KNOWLEDGE-ROADMAP-TEST-MATRIX.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.md','REMAINING-13-EXECUTION.json')],'docs/REMAINING-WORK.md']
 if not r or any(subprocess.check_output(['git','show',r+':'+n],cwd=ROOT)!=(ROOT/n).read_bytes()for n in names):raise ValueError('current role evidence/docs not stored together')
 subprocess.run(['git','verify-commit',r],cwd=ROOT,capture_output=True,check=True);return r

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','check','update-indices']);p.add_argument('--actual',type=Path);p.add_argument('--initial',type=Path);p.add_argument('--native',type=Path);p.add_argument('--require-stored',action='store_true');a=p.parse_args()
 if a.command=='prepare':prepare(a.actual,a.initial,a.native)
 elif a.command=='update-indices':update_indices()
 else:
  errors=check()
  if errors:raise SystemExit('\n'.join(errors))
  if a.require_stored:stored()
  print(json.dumps(read(ROOT/PACKET/'SUMMARY.json'),ensure_ascii=False))
