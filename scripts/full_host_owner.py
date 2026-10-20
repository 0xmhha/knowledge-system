"""Replay real-factory owner observations without substituting a formal oracle."""
import argparse,gzip,hashlib,importlib.util,json,os,subprocess
from pathlib import Path
import full_host_factory as factory
import owner_schedule_contract as contract
import native_reliability as native_checkpoint
import wi6_version_readiness as history
from ticket_actual_rpc import flat_map,tree_oid
from source_packet import oid
ROOT=factory.ROOT
PACKET=factory.PACKET.parent/'full-host-owner-v1'
REVISION='0bbc94c410db1f2a0cf220e6c1731f4e06135149'
TOOLS=['scripts/full_host_owner.py','scripts/test_full_host_owner.py','scripts/run-full-host-owner.py','scripts/fixtures/full-host-owner.go.fixture','scripts/fixtures/full-host-owner-main.go.fixture','scripts/owner_schedule_contract.py','scripts/test_owner_schedule_contract.py','scripts/ticket_actual_rpc.py','scripts/source_packet.py','scripts/native_reliability.py']
read,raw,sha,write=factory.read,factory.raw,factory.sha,factory.write
ROLES=['ORIGINAL','CAND-A','CAND-B','MUT-REINTRO','MUT-EARLY']
SCENARIOS=['normal_delivery_then_unsubscribe','blocked_delivery_then_consumer_exit','confirmation_delayed','concurrent_unsubscribe_once']

def events(raw_bytes):return [json.loads(line)for line in raw_bytes.splitlines()if line.strip()]
def only(rows,event,phase=None):
 found=[r for r in rows if r['event']==event and (phase is None or r['phase']==phase)]
 if len(found)!=1:raise ValueError('nonunique owner observation '+event+' '+str(phase))
 return found[0]

def observation(rows,role):
 if any(a['elapsed_ns']>b['elapsed_ns']for a,b in zip(rows,rows[1:])):raise ValueError('owner observation clock order differs')
 if sum(r['event']=='method_entry'for r in rows)!=7 or sum(r['event']=='method_return_defer'for r in rows)!=7:raise ValueError('not every actual method call returned in the owned observation')
 identity=only(rows,'factory_api_identity')['value']
 if identity!=dict(same_downloader=True,same_blockchain=True,same_mux=True,new_API_calls_from_probe=0):raise ValueError('actual factory identity differs')
 if only(rows,'actual_notifier_unsupported')['value']!='notifications not supported':raise ValueError('unsupported notifier path differs')
 if only(rows,'cleanup_members_after')['value']!=0 or only(rows,'probe_end')['value']!={'error':'<nil>'}:raise ValueError('actual owner cleanup or probe completion missing')
 if only(rows,'late_after_accept_not_injected')['value']!=dict(actual_owner_has_no_send_between_delete_and_ack=True,separate_controlled_owner_required=True,full_original5_complete=False,independent_approved=0):raise ValueError('unreachable owner schedule injected or falsely completed')
 result=[]
 for phase in SCENARIOS:
  observation=only(rows,'schedule_observation',phase);value=observation['value'];channel=observation['channel_id']
  if channel<=0:raise ValueError('owner channel identity missing')
  local=[r for r in rows if r['channel_id']==channel]
  owner=[r for r in local if r['owner_map_observed']]
  if any(r['member']for r in owner if r['event']in('after_delete','ack_close_begin','ack_close_end')):raise ValueError('ack precedes actual owner deletion')
  deleted=[i for i,r in enumerate(local)if r['event']=='after_delete'];ack=[i for i,r in enumerate(local)if r['event']=='ack_close_begin']
  if len(deleted)!=1 or len(ack)!=1 or deleted[0]>=ack[0]:raise ValueError('actual final map removal/ack order differs')
  if role=='MUT-EARLY':
   expected=dict(returned=True,owner_member=True,owner_delete=0,owner_ack=0)if phase==SCENARIOS[0]else dict(returned_before_rescue=True,owner_member=True,owner_send_begin=1,owner_send_end=0,owner_delete=0,owner_ack=0)if phase==SCENARIOS[1]else dict(gate_entered=False,all_calls_joined=True,method_entries=3 if phase==SCENARIOS[3]else 1,owner_delete=0,owner_ack=0,owner_member=True)
   outcome='returned_without_actual_owner_remove_or_ack'
  elif phase==SCENARIOS[0]:expected=dict(returned=True,owner_member=False,owner_delete=1,owner_ack=1);outcome='returned_after_actual_owner_remove_and_ack'
  elif phase==SCENARIOS[1]:
   blocked=role in ('ORIGINAL','MUT-REINTRO')
   expected=dict(returned_before_rescue=not blocked,owner_member=blocked,owner_send_begin=1,owner_send_end=0 if blocked else 1,owner_delete=0 if blocked else 1,owner_ack=0 if blocked else 1)
   outcome='paired_waits_pending_before_rescue'if blocked else'drained_real_status_then_returned_after_owner_ack'
  else:expected=dict(gate_entered=True,all_calls_joined=True,method_entries=3 if phase==SCENARIOS[3]else 1,owner_delete=1,owner_ack=1,owner_member=False);outcome='held_until_actual_owner_ack_then_once_calls_joined'
  if value!=expected:raise ValueError('unexpected finite owner observation '+role+' '+phase+': '+str(value))
  if phase in SCENARIOS[2:]:
   held=only(rows,'held_ack_observation',phase)['value'];early=role=='MUT-EARLY';concurrent=phase==SCENARIOS[3]
   if held!=dict(gate_entered=not early,first_returned=early,second_returned=early and concurrent,method_entries=2 if concurrent else 1,owner_delete=0 if early else 1,owner_member=early,owner_ack_end=0):raise ValueError('ack gate/Once pre-release observation differs')
  if role=='MUT-EARLY'and not any(r['event']=='owned_cleanup_intervention'for r in local[local.index(observation)+1:]):raise ValueError('early-return cleanup omitted or moved before observation')
  if phase==SCENARIOS[1]and role in('ORIGINAL','MUT-REINTRO')and not any(r['event']=='owned_cleanup_intervention'for r in local[local.index(observation)+1:]):raise ValueError('paired waits rescue omitted or moved before observation')
  result.append(dict(scenario=phase,channel_id=channel,outcome=outcome,observation_elapsed_ns=observation['elapsed_ns']))
 return result

def factory_history(root):
 p=root/factory.PACKET;original=history.read_at(root,REVISION,factory.PACKET/'manifest.json')
 if (p/'manifest.json').read_bytes()!=original:raise ValueError('historical factory manifest differs from Git')
 manifest=json.loads(original)
 if manifest['artifacts']!={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'}:raise ValueError('historical factory artifacts differ')
 for name,digest in manifest['tools'].items():
  history.bounded_path(root,name)
  if sha(history.read_at(root,REVISION,name))!=digest:raise ValueError('historical factory tool differs from its own committed digest')
 return dict(historical_bytes_verified=True,revision=REVISION,manifest_sha256=sha(original))

def derive(root=ROOT,packet=None):
 p=packet or root/PACKET
 historic=factory_history(root)
 native=read(root/factory.NATIVE/'source-binding.json')
 if any(sha((root/n).read_bytes())!=h for n,h in {**native['source_sha256'],**native['approved_input_sha256']}.items()):raise ValueError('native source changed after full check')
 live=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True).splitlines()
 if {n for n in live if n.startswith(('cmd/','pkg/','internal/'))}!={n for n in native['source_sha256']if n.endswith('.go')}:raise ValueError('native Go inventory changed')
 scope=read(p/'actual/SCOPE.json.gz')
 for n,key in [(TOOLS[2],'driver_sha256'),(TOOLS[3],'bridge_fixture_sha256'),(TOOLS[4],'main_fixture_sha256')]:
  if sha((root/n).read_bytes())!=scope[key]:raise ValueError('frozen full owner tools changed')
 planned=[dict(slot_id=r+'-'+v,candidate=r,variant=v,repetition=1)for r in ROLES for v in('general','race')]
 if scope['planned_slots']!=planned or scope['automatic_retries']!=0 or scope['program_timeout_seconds']!=100 or scope['original_timer_seconds']!=60 or scope['post_done_owner_schedules']!=SCENARIOS or scope['late_after_accept_injected']is not False or scope['full_original5_complete']is not False or scope['alias_roles']!={'MUT-REINTRO':'ORIGINAL'}or scope['independent_approved']!=0 or scope['formal_completed']!=0:raise ValueError('owner source/runtime scope changed or promoted')
 if contract.check():raise ValueError('source-scope proposal inconsistent or not independently distinguished')
 baseline=json.loads(history.read_at(root,REVISION,factory.PACKET.parent/'wi4-target-subscription-v1/corrective/source-binding.json'))['tracked'];binaries={}
 baseline_tree=flat_map(json.loads(history.read_at(root,REVISION,factory.PACKET.parent/'ticket-rpc-lifecycle-results-v1/actual/candidates/ORIGINAL/base-flat-tree.json')))
 for role in scope['build_roles']:
  q=p/'actual/candidates'/role;b=read(q/'source-binding.json.gz');api=raw(q/'api.go.gz')
  if b['base_tracked']!=baseline or b['source_commit']!=factory.COMMIT or b['production_patch']is not False or b['instrumented_api_sha256']!=sha(api)or sha(api)!=scope['instrumented_api_sha256'][role]:raise ValueError('candidate source boundary differs')
  changed=[n for n,v in b['applied_tracked'].items()if baseline.get(n)!=v]
  if changed!=['eth/downloader/api.go']or read(q/'source-after.json.gz')!=b['applied_tracked']:raise ValueError('undeclared or changed postrun source')
  if b['additions']!={'eth/downloader/ks_owned_full_host_owner.go':scope['bridge_fixture_sha256'],'cmd/ks-owned-full-host-owner/main.go':scope['main_fixture_sha256']}:raise ValueError('program additions changed')
  trees=read(q/'tree-binding.json.gz')
  if trees['base_commit']!=factory.COMMIT or trees['applied_project_commit']is not None:raise ValueError('owned working trees relabelled as source commit')
  project=flat_map(trees['project_flat_tree']);build_tree=flat_map(trees['build_flat_tree'])
  if tree_oid(trees['project_flat_tree'])!=trees['applied_project_tree']or tree_oid(trees['build_flat_tree'])!=trees['build_input_tree']:raise ValueError('Git tree identities differ')
  expected_project=dict(baseline_tree);expected_project['eth/downloader/api.go']=dict(mode='100644',kind='blob',oid=oid('blob',api))
  if project!=expected_project:raise ValueError('project Git tree changed beyond declared API')
  expected_build=dict(project);expected_build['eth/downloader/ks_owned_full_host_owner.go']=dict(mode='100644',kind='blob',oid=oid('blob',(root/TOOLS[3]).read_bytes()));expected_build['cmd/ks-owned-full-host-owner/main.go']=dict(mode='100644',kind='blob',oid=oid('blob',(root/TOOLS[4]).read_bytes()))
  if project['eth/downloader/api.go']['oid']!=oid('blob',api)or build_tree!=expected_build:raise ValueError('Git build tree source/additions differ')
  binaries[role]=read(q/'binaries.json.gz')
  for variant in ('general','race'):
   build=factory.receipt(q/('build-'+variant)/'receipt.json.gz')
   if build['argv'][1:]!=['build']+(['-race']if variant=='race'else[])+['-o',binaries[role][variant]['path'],'./cmd/ks-owned-full-host-owner']:raise ValueError('fresh build population differs')
   info=raw(q/(variant+'-build-info.txt.gz')).decode()
   if 'go1.23.12'not in info or 'GOOS=darwin'not in info or 'GOARCH=arm64'not in info or 'CGO_ENABLED=1'not in info or 'vcs.revision='+factory.COMMIT not in info or 'vcs.modified=true'not in info or (variant=='race'and '-race=true'not in info):raise ValueError('actual target toolchain/build profile missing')
   if read(q/'binary-after.json.gz')[variant]!=binaries[role][variant]['sha256']:raise ValueError('postrun binary changed')
 binaries['MUT-REINTRO']=binaries['ORIGINAL'];rows=[]
 for slot in planned:
  q=p/'actual/slots'/slot['slot_id'];r=factory.receipt(q/'process/receipt.json.gz');rslot=read(q/'receipt.json.gz');binary=binaries[slot['candidate']][slot['variant']]
  if r['argv']!=[binary['path']]or rslot['binary_sha256']!=binary['sha256']or rslot['tmp_children_after']or raw(q/'process/stderr.gz'):raise ValueError('actual runtime binary, race stderr or cleanup differs')
  main=events(raw(q/'process/stdout.gz'));extra=[x for x in main if x['event']=='actual_owner_schedules_returned']
  if len(extra)!=1 or extra[0]['value']is not True:raise ValueError('full host owner probe not completed')
  native_result=factory.trace([x for x in main if x['event']!='actual_owner_schedules_returned'])
  owner=events(raw(q/'owner.jsonl.gz'));observations=observation(owner,slot['candidate'])
  stack=raw(q/'owner.jsonl.blocked-before-rescue.stack.gz').decode();api=raw(p/'actual/candidates'/('ORIGINAL'if slot['candidate']=='MUT-REINTRO'else slot['candidate'])/'api.go.gz').decode()
  if slot['candidate']in('ORIGINAL','MUT-REINTRO'):
   for statement in ('i <- false','s.api.uninstallSyncSubscription <- &req'):
    line=next(i+1 for i,v in enumerate(api.splitlines())if v.strip()==statement)
    if '/eth/downloader/api.go:'+str(line)not in stack:raise ValueError('paired owner/method source stack absent before rescue')
  rows.append(dict(**slot,**native_result,observations=observations,owner_events=len(owner),cleanup_interventions=sum(r['event']=='owned_cleanup_intervention'for r in owner),RPC_backend_members_before_cleanup=only(owner,'after_RPC_backend_members')['value']))
 if read(p/'actual/original-before.json.gz')!=read(p/'actual/original-after.json.gz')or raw(p/'actual/original-status-before.txt.gz')!=raw(p/'actual/original-status-after.txt.gz'):raise ValueError('original source changed')
 if read(p/'actual/receipt.json.gz')!=dict(planned=10,attempted=10,automatic_retries=0,individual_program_failures=0,full_original5_complete=False,formal_completed=0,independent_approved=0,production_patch=False):raise ValueError('driver denominator or program failures changed')
 author=json.loads(history.read_at(root,REVISION,'scripts/fixtures/ticket-dev-authoring.json'))
 if len(author['requirements'])!=12:raise ValueError('original twelve requirements changed')
 units=[dict(slot_id=row['slot_id'],requirement_id=req['id'],original_tier=req['tier'],scope='not_exercised_real_owner_post_accept_status'if req['id']=='LATE-DRAIN'else'actual_factory_registered_RPC'if req['id'].startswith('API-')or req['id']=='NORMAL-DELIVERY'else'actual_factory_direct_status_owner_with_declared_gates_and_cleanup',independent_acceptance=False,full_original_requirement_satisfied=False)for row in rows for req in author['requirements']]
 return dict(kind='real_factory_owner_schedules_DEV_not_independent',planned_node_attempts=10,actual_node_attempts=10,fresh_unique_builds=8,candidate_roles=5,unique_method_variants=4,actual_factory_owner_schedules=40,original60s_native_notifications=10,cleanup_interventions=sum(r['cleanup_interventions']for r in rows),paired_waits_pending_before_rescue=4,actual_status_drain_and_owner_ack_observations=4,early_return_without_owner_remove_observations=8,all_original_and_owned_source_preserved=True,owned_data_remaining=0,owned_process_groups_remaining=0,source_hooks_observe_actual_owner=True,ack_gate_changes_timing_under_declared_control=True,real_network_sync=False,repetitions_per_role_profile=1,new_independent_families=0,full_original5_complete=False,full12_requirement_acceptance=False,independent_acceptable_candidate_set=None,canonical_selection=None,formal_completed=0,independent_approved=0,FINAL=0,freeze=None,execution_allowed=False,historical_full_factory_bytes_verified=historic['historical_bytes_verified'],slots=rows,original_requirement_units=units)

def seal(p):
 external={str(n):sha((ROOT/n).read_bytes())for n in (contract.PACKET/'PROPOSAL.json',contract.PACKET/'original-api.go.gz',contract.PACKET/'REPORT.txt')}
 write(p/'manifest.json',dict(kind='full_host_owner_DEV',tools={n:sha((ROOT/n).read_bytes())for n in TOOLS},proposal_inputs=external,artifacts={str(x.relative_to(p)):sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x.name!='manifest.json'}))
def index_refs(root=ROOT):
 return dict(kind='real_factory_owner_schedules_DEV_not_independent',manifest=str(PACKET/'manifest.json'),manifest_sha256=sha((root/PACKET/'manifest.json').read_bytes()),formal_completed=0,independent_approved=0,full_original5_complete=False)
def update_indices():
 ref=index_refs();ledger=Path('docs/spec-driven/REMAINING-13-EXECUTION.json');worklist=dict(path=str(ledger),sha256=sha((ROOT/ledger).read_bytes()))
 for name in native_checkpoint.PATHS:
  d=read(ROOT/name);d['latest_scoped_followup']=ref;d['active_remaining_worklist']=worklist;write(ROOT/name,d)
 native_checkpoint.update_indices()

def check(root=ROOT,packet=None):
 p=packet or root/PACKET
 try:
  m=read(p/'manifest.json')
  if m['tools']!={n:sha((root/n).read_bytes())for n in TOOLS}or m['artifacts']!={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'}or any(sha((root/n).read_bytes())!=h for n,h in m['proposal_inputs'].items()):raise ValueError('owner evidence/tool/proposal hash changed')
  if read(p/'SUMMARY.json')!=derive(root,p):raise ValueError('actual owner result or acceptance limit changed')
  if packet is None:
   ledger=Path('docs/spec-driven/REMAINING-13-EXECUTION.json');expected=dict(path=str(ledger),sha256=sha((root/ledger).read_bytes()))
   if any(read(root/n).get('latest_scoped_followup')!=index_refs(root)or read(root/n).get('active_remaining_worklist')!=expected for n in native_checkpoint.PATHS):raise ValueError('latest followup/worklist indices differ')
  return []
 except(ValueError,KeyError,OSError,TypeError,IndexError)as error:return [str(error)]
def capture_metadata(actual):
 actual=actual.resolve();env={k:os.environ[k]for k in ('PATH','HOME','TMPDIR')if k in os.environ}
 env.update(GOTOOLCHAIN='local',GOWORK='off',GOPROXY='off',GOSUMDB='off',GOROOT='/Users/kevin/.gvm/gos/go1.23.12')
 for role in ('ORIGINAL','CAND-A','CAND-B','MUT-EARLY'):
  q=actual/'candidates'/role;binaries=read(q/'binaries.json');post={}
  for variant in ('general','race'):
   binary=Path(binaries[variant]['path']).resolve()
   if not binary.is_relative_to(actual)or sha(binary.read_bytes())!=binaries[variant]['sha256']:raise ValueError('metadata binary outside owned evidence or changed')
   info=subprocess.check_output(['/Users/kevin/.gvm/gos/go1.23.12/bin/go','version','-m',str(binary)],cwd=actual,env=env,timeout=30)
   (q/(variant+'-build-info.txt')).write_bytes(info);post[variant]=sha(binary.read_bytes())
   if post[variant]!=binaries[variant]['sha256']:raise ValueError('binary changed during metadata capture')
  write(q/'binary-after.json',post)

def prepare(actual):
 p=ROOT/PACKET
 if p.exists():raise ValueError('refusing owner packet overwrite')
 capture_metadata(actual)
 p.mkdir()
 for x in actual.rglob('*'):
  if not x.is_file()or x.name.endswith('.program')or x.name=='live.json'or'tmp'in x.relative_to(actual).parts:continue
  target=p/'actual'/(str(x.relative_to(actual))+'.gz');target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(gzip.compress(x.read_bytes(),mtime=0))
 finalize(p)
def finalize(p):
 write(p/'SUMMARY.json',derive());seal(p)
def stored():
 revision=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip();m=read(ROOT/PACKET/'manifest.json')
 names=[str(PACKET/'manifest.json'),*[str(PACKET/n)for n in m['artifacts']],*m['proposal_inputs'],*TOOLS,*[str(n)for n in native_checkpoint.PATHS],'docs/spec-driven/CURRENT-HANDOFF.md','docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md','docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.md','docs/spec-driven/REMAINING-13-EXECUTION.json','docs/REMAINING-WORK.md']
 if not revision or any(subprocess.check_output(['git','show',revision+':'+n],cwd=ROOT)!=(ROOT/n).read_bytes()for n in names):raise ValueError('owner current evidence/docs not stored in same commit')
 subprocess.run(['git','verify-commit',revision],cwd=ROOT,check=True,capture_output=True);return revision

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','check','update-indices']);p.add_argument('--actual',type=Path);p.add_argument('--require-stored',action='store_true');a=p.parse_args()
 if a.command=='prepare':prepare(a.actual)
 elif a.command=='update-indices':update_indices()
 else:
  errors=check()
  if errors:raise SystemExit('\n'.join(errors))
  if a.require_stored:stored()
  print(json.dumps({k:v for k,v in read(ROOT/PACKET/'SUMMARY.json').items()if k!='slots'},ensure_ascii=False))
