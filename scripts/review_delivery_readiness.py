"""Actual role/CLI/hook delivery and browser rendering; no semantic approval."""
import gzip,json,subprocess,urllib.parse
from pathlib import Path
import four_role_readiness as roles
import intent_startup_batch as native
ROOT=native.ROOT;PACKET=native.PACKET.parent/'review-delivery-current-v1'
TOOLS=['scripts/'+n for n in ('review_delivery_readiness.py','test_review_delivery_readiness.py','consumer-role-host.py','operational-host-probe.py','run-wi6-pair-pilot.py','wi6_pair_contract.py')]
read,raw,write,sha,compress=native.read,native.raw,native.write,native.sha,native.compress
ARTIFACTS=['deep-review-evidence.json','deep-review.json','review-report.md','sequence.mmd','modules.mmd']

def require(value,message):
 if not value:raise ValueError(message)

def inspect(p,adopted=False,unsupported=False):
 scope=read(p/'evidence/SCOPE.json.gz');receipt=read(p/'evidence/receipt.json.gz');initial=read(p/'evidence/initial-state.json.gz');final=read(p/'evidence/final-state.json.gz')
 require(scope['actual_LLM_calls']==scope['paid_external_calls']==scope['formal_completed']==scope['independent_approved']==0 and scope['security_only_evaluations_executed'] is False and scope['global_install'] is False,'role scope promoted')
 require(scope['consumer_revision']==roles.CONSUMER_REV and scope['required_artifacts']==ARTIFACTS and scope['roles']==['analyzer','planner'],'consumer/role/artifact scope differs')
 expected=read(ROOT/roles.PACKET/'actual/evidence/consumer-before.json.gz')
 require(read(p/'evidence/consumer-before.json.gz')==read(p/'evidence/consumer-after.json.gz')==expected,'approved consumer source changed')
 require(read(p/'evidence/original-before.json.gz')==read(p/'evidence/original-after.json.gz')==read(p/'evidence/owned-after.json.gz') and read(p/'evidence/original-before.json.gz')==read(ROOT/roles.PACKET/'actual/evidence/original-data-before.json.gz') and len(read(p/'evidence/original-before.json.gz'))==2248,'original/owned payload changed')
 require(read(p/'evidence/source-before.json.gz')==read(p/'evidence/source-after.json.gz') and read(p/'evidence/source-before.json.gz')==read(ROOT/roles.PACKET/'actual/evidence/target-source-before.json.gz') and len(read(p/'evidence/source-before.json.gz'))==2228,'actual source checkout changed')
 require(initial['ticket_type']==final['ticket_type']=='code_review' and receipt['actual_roles']==2 and receipt['actual_LLM_calls']==receipt['formal_completed']==receipt['independent_approved']==0 and receipt['original_owned_preserved'],'fixture identity/acceptance state differs')
 summaries=[];all_cli=[]
 for role in ['analyzer','planner']:
  q=p/('P79-'+role);exit=read(q/'exit.json.gz');inv=read(q/'invocation.json.gz');env=inv['env'];url=urllib.parse.urlparse(env['ANTHROPIC_BASE_URL'])
  require(exit['host_exit']==0 and exit['host_reaped'] and not exit['forced_host_termination'] and exit['responder_closed'] and exit['responder_thread_joined'] and exit['real_model_calls']==0 and url.hostname=='127.0.0.1' and url.port==exit['responder_port'],'host/local gateway cleanup differs')
  require(inv['api_credential_kind']=='constant_fake_local_token' and inv['cks_sha256']==scope['binary_sha256'] and inv['host_sha256']=='475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3','actual host/binary identity changed')
  argv=inv['argv'];require(argv[argv.index('--agent')+1]=='coding-agent:'+role and '--strict-mcp-config'in argv and '--setting-sources'in argv and '--no-session-persistence'in argv,'actual installed role/config selection differs')
  frames=native.events(raw(q/'mcp-frames.jsonl.gz'));exits=[r for r in frames if r['kind']=='exit'];require(len(exits)==1 and exits[0]['exit_code']==0 and not exits[0]['terminate_sent'] and not exits[0]['forced_kill'] and exits[0]['child_reaped'],'actual CKS cleanup differs')
  requests=[json.loads(r['raw'])for r in frames if r['kind']=='request'];calls=[r for r in requests if r.get('method')=='tools/call'];require([r['params']['name']for r in calls]==(['cks.context.rule_candidates','cks.context.review_function']if role=='analyzer'else[]),'actual dual-tool route differs')
  gateways=read(q/'gateway-requests.json.gz');actions,results=roles.tool_results(gateways);cli=[];errors=[]
  for key,action in actions.items():
   result=results.get(key,{});text=roles.text_content(result)
   if result.get('is_error'):errors.append(dict(tool=action['name'],text=text.split('<system-reminder>')[0].strip()))
   if action['name']=='Bash':cli.append(dict(error=bool(result.get('is_error')),output=text.split('<system-reminder>')[0].strip()))
  all_cli.extend(cli)
  if adopted:
   require(len(actions)==len(results)==(8 if role=='analyzer'else 11),'actual role actions/results omitted')
   require(len(cli)==2 and all(not r['error']for r in cli),'actual CLI build/verification failed')
   if role=='analyzer':require(not errors and 'structured_partial_review'in cli[0]['output'] and 'validated_delivery'in cli[1]['output'],'Analyzer did not build and validate report')
   else:require(len(errors)==1 and errors[0]['tool']=='Write' and 'completion requires bound delivered review report'in errors[0]['text'],'bad report terminal Write was not denied by actual deep-review hook')
   saved=read(q/'workspace-after/state.json.gz')
   require(saved['current_state']=='PLANNING' and saved['ticket_type']=='code_review' and saved['states']['ANALYSIS']['status']=='completed','actual handoff state missing')
   require(saved['states']['ANALYSIS']['deep_review_sha256']==sha(raw(q/'workspace-after/deep-review.json.gz')),'actual source report binding differs')
   if role=='planner':require(saved['states']['PLANNING']==dict(status='completed',review_report_sha256=sha(raw(q/'workspace-after/review-report.md.gz'))),'Planner did not bind actual report bytes')
   reads=[a['input']['file_path']for a in actions.values()if a['name']=='Read']
   require(any(x.endswith('/structured-review/SKILL.md')for x in reads),'actual role did not read structured-review skill')
   if role=='planner':require(set(ARTIFACTS)<=set(Path(x).name for x in reads),'Planner did not actually Read all five artifacts')
  summaries.append(dict(role=role,host_exit=0,CKS_exit=0,actual_tool_calls=len(calls),actions=len(actions),CLI_calls=len(cli),errors=errors))
 if adopted:
  require('publication_policy'not in initial['config'] and scope['no_publication_actions'] is True and scope['terminal_success_not_requested'] is True,'supported legacy review-only scope changed')
  a=read(p/'evidence/analyzer-artifacts.json.gz');b=read(p/'evidence/final-artifacts.json.gz');require(a==b and set(a)==set(ARTIFACTS),'five-artifact handoff changed')
  q=p/'P79-planner/workspace-after';report=read(q/'deep-review.json.gz');bundle=read(q/'deep-review-evidence.json.gz')
  require(report['status']=='structured_partial_review' and report['independent_acceptance']=='not_recorded' and report['source']['node_id']==scope['node']['node_id'] and report['source']['sha256']==scope['node']['function_sha256'],'typed source identity or acceptance changed')
  require(report['evidence_sha256']==sha(json.dumps(bundle,sort_keys=True,ensure_ascii=False,separators=(',',':'),allow_nan=False).encode()),'typed bundle binding differs')
  require(set(report['reasoning']['sections'])=={'data_state','guards_exceptions','complexity','resources','io','priority_tradeoffs'} and report['reasoning']['findings'][0]['priority']=='medium' and report['reasoning']['findings'][0]['state']=='review_candidate','reasoning fields/priority/origin changed')
  require(report['computed']['sequence']['kind']=='bounded_cfg_not_execution_trace' and report['computed']['module']['interprocedural_edges']=='unknown' and report['structural_review']['snapshot_association']=='identity_only_unverified_snapshot','source vs inferred/unknown boundaries promoted')
  # Both receipts must be responses actually seen on the Analyzer CKS stream.
  frames=native.events(raw(p/'P79-analyzer/mcp-frames.jsonl.gz'));reply={json.loads(r['raw'])['id']:json.loads(r['raw'])for r in frames if r['kind']=='response'};calls=[json.loads(r['raw'])for r in frames if r['kind']=='request'and json.loads(r['raw']).get('method')=='tools/call']
  for key,call in zip(['rules','function_review'],calls):require(bundle[key]==dict(tool=call['params']['name'],arguments=call['params']['arguments'],result=reply[call['id']]['result'].get('structuredContent') or json.loads(reply[call['id']]['result']['content'][0]['text'])),'bundle not actual dual-tool wire')
  require(final['current_state']=='PLANNING' and receipt['artifact_files']==5,'failed terminal changed state or artifacts missing')
 elif unsupported:
  require(receipt['artifact_files']==5 and final['current_state']=='ANALYSIS' and any('evaluation_only requires an intake policy lock'in e['text']for s in summaries for e in s['errors']),'unsupported fixture failure omitted')
 else:require(receipt['artifact_files']==0 and final['current_state']=='ANALYSIS' and any(r['error']for r in all_cli),'initial script selector failure omitted')
 return dict(actual_roles=2,host_and_CKS_exit0=True,typed_handoff_verified=adopted,artifact_files=receipt['artifact_files'],roles=summaries,actual_LLM_calls=0,formal_completed=0,independent_approved=0)

def render(p):
 scope=read(p/'SCOPE.json');result=read(p/'render-result.txt');visual=read(p/'VISUAL-REVIEW.json');server=read(p/'server-receipt.json')
 require(result['complete'] and result['errors']==[] and len(result['svg'])==len(result['boxes'])==2 and all(x['width']>0 and x['height']>0 for x in result['boxes']),'actual SVG rendering failed')
 require(visual['visual_inspected'] and visual['semantic_quality_approved'] is False and visual['independent_approved']==0,'visual check promoted to semantic approval')
 require(read(p/'renderer-library.json')==scope['renderer'] and read(p/'process-cleanup.json')['browser_PID_live'] is False and read(p/'process-cleanup.json')['server_PID_live'] is False, 'renderer pin or owned process cleanup differs')
 require(server['closed'] and server['stopped_by_owned_marker'] and [r['path']for r in server['requests']]==['/','/mermaid.min.js','/favicon.ico'],'owned renderer requests/cleanup changed')
 labels=read(p/'visible-result.json')['labels'];require('No CFG successors; no safety conclusion'in labels[0] and 'eth/downloader/api.go'in labels[1] and 'Interprocedural relationships unavailable'in labels[1],'bounded/unknown labels missing')
 for name,h in scope['inputs'].items():require(sha(raw(p.parent/'actual/P79-planner/workspace-after'/(name+'.gz')))==h,'rendered Mermaid differs from delivered artifact')
 return dict(actual_SVGs=2,visual_inspection=True,clipped_labels=False,renderer_version=scope['renderer']['version'],semantic_acceptance=False)

def derive(p):
 initial=inspect(p/'initial');unsupported=inspect(p/'unsupported-policy',unsupported=True);actual=inspect(p/'actual',adopted=True);r=render(p/'render')
 cleanup=read(p/'process-cleanup.json');require(all(x['remaining_recorded_PIDs']==x['remaining_host_group_members']==0 for x in cleanup),'actual host/proxy/CKS process remains')
 return dict(kind='WIC_actual_installed_typed_delivery_and_render_DEV',initial=initial,unsupported_policy=unsupported,actual=actual,render=r,actual_hosts=6,actual_CKS_sessions=6,initial_script_selector_failed=True,unsupported_policy_fixture_failed=True,explicit_corrective_attempts=2,legacy_review_only_route_used=True,bugfix_full_evaluation_only_extended_to_review=False,normal_terminal_transition_not_requested=True,paid_external_LLM_calls=0,actual_LLM_calls=0,local_embedding_total_HTTP_requests=None,SDK_USD='nominal_scripted_usage_not_billing',required_item_omission_score=None,priority_correctness=None,actual_priority_path='medium_only',exposed_source_case_reused=True,fresh_independent_cases=0,independent_approved=0,formal_completed=0,formal_total=9,FINAL=0,freeze=None,execution_allowed=False)

def reference():return dict(kind='WIC_actual_installed_typed_delivery_and_render_DEV',manifest=str(PACKET/'manifest.json'),manifest_sha256=sha((ROOT/PACKET/'manifest.json').read_bytes()))
def seal(p):write(p/'manifest.json',dict(tools={n:sha((ROOT/n).read_bytes())for n in TOOLS},artifacts={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'}))
def check(packet=None):
 p=packet or ROOT/PACKET
 try:
  m=read(p/'manifest.json');require(m['tools']=={n:sha((ROOT/n).read_bytes())for n in TOOLS}and m['artifacts']=={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'},'review evidence/tools changed')
  require(read(p/'SUMMARY.json')==derive(p),'review delivery result changed/promoted')
  require(not native.validate(),'current native source validation differs')
  require(read(p/'actual/evidence/SCOPE.json.gz')['binary_sha256']==native.read(ROOT/native.PACKET/'native/binary.json')['sha256'],'actual review CKS differs from current tested binary')
  if packet is None:
   for n in native.PATHS:require(read(ROOT/n)['latest_review_delivery_followup']==reference(),'review current index differs')
  return []
 except(ValueError,OSError,KeyError,TypeError,IndexError)as e:return[str(e)]
def prepare():
 p=ROOT/PACKET;require(not p.exists(),'refusing review evidence overwrite');p.mkdir()
 cases=[('initial','ks-phase79-review-delivery-20261019','run-phase79-review-delivery.py'),('unsupported-policy','ks-phase79-review-delivery-corrective1-20261019','run-phase79-review-delivery-corrective1.py'),('actual','ks-phase79-review-delivery-corrective2-20261019','run-phase79-review-delivery-corrective2.py')]
 for label,directory,driver in cases:
  src=Path('/private/tmp')/directory
  for top in[src/'evidence',src/'P79-analyzer',src/'P79-planner']:
   for x in top.rglob('*'):
    if not x.is_file()or x.is_symlink()or'claude-config'in x.parts or'tmp'in x.relative_to(src).parts:continue
    compress(p/label/(str(x.relative_to(src))+'.gz'),x.read_bytes())
  code=Path('/private/tmp',driver).read_bytes();require(sha(code)==read(p/label/'evidence/SCOPE.json.gz')['driver_sha256'],'executed review driver changed');compress(p/label/'executed-driver.py.gz',code)
  compress(p/label/'driver-log.txt.gz',Path('/private/tmp',directory.replace('ks-','').replace('-20261019','')+'-driver.log').read_bytes())
 render_dir=Path('/private/tmp/ks-phase79-render-20261019');(p/'render').mkdir()
 for name in ['SCOPE.json','renderer-library.json','render-result.txt','visible-result.json','VISUAL-REVIEW.json','server-receipt.json','server.json','process-cleanup.json','index.html','serve.py','cli-version.txt','open.txt','snapshot.txt','screenshot.txt','rendered-svg.json']:
  (p/'render'/name).write_bytes((render_dir/name).read_bytes())
 compress(p/'render/close.txt.gz',(render_dir/'close.txt').read_bytes())
 for x in(render_dir/'output/playwright').iterdir():
  if x.is_file():(p/'render'/x.name).write_bytes(x.read_bytes())
 (p/'process-cleanup.json').write_bytes(Path('/private/tmp/phase79-process-cleanup.json').read_bytes())
 write(p/'SUMMARY.json',derive(p));seal(p)
def update_indices():
 for n in native.PATHS:
  d=read(ROOT/n);d['latest_review_delivery_followup']=reference();write(ROOT/n,d)
 native.update_indices()
def stored():
 revision=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip();require(bool(revision),'review packet not committed')
 names=subprocess.check_output(['git','diff-tree','--root','--no-commit-id','--name-only','-r',revision],cwd=ROOT,text=True).splitlines()
 for n in names:require(subprocess.check_output(['git','show',revision+':'+n],cwd=ROOT)==(ROOT/n).read_bytes(),'stored changed blob differs: '+n)
 require(subprocess.run(['git','verify-commit',revision],cwd=ROOT,capture_output=True).returncode==0,'review commit signature not verified');return dict(revision=revision,changed_blobs_verified=len(names),signature_verified=True)
if __name__=='__main__':
 import argparse
 parser=argparse.ArgumentParser();parser.add_argument('command',choices=['prepare','check','update-indices']);parser.add_argument('--require-stored',action='store_true');a=parser.parse_args()
 if a.command=='prepare':prepare()
 elif a.command=='update-indices':update_indices()
 else:
  errors=check()
  if errors:raise SystemExit('\n'.join(errors))
  if a.require_stored:print(json.dumps(stored()))
  print(json.dumps(read(ROOT/PACKET/'SUMMARY.json'),indent=2))
