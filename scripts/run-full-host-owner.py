#!/usr/bin/env python3
"""Declared real-factory owner schedules, with all bounded observations retained."""
import argparse,concurrent.futures,importlib.util,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('owned',ROOT/'scripts/run-wi4-target-subscription.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
COMMIT=m.COMMIT;API='eth/downloader/api.go';BRIDGE='eth/downloader/ks_owned_full_host_owner.go';MAIN='cmd/ks-owned-full-host-owner/main.go'
BRIDGE_FIXTURE=ROOT/'scripts/fixtures/full-host-owner.go.fixture';MAIN_FIXTURE=ROOT/'scripts/fixtures/full-host-owner-main.go.fixture'
DRAFT=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1'
ROLES=['ORIGINAL','CAND-A','CAND-B','MUT-REINTRO','MUT-EARLY']

def instrument(data,role):
 text=data.decode();counts={};replacements=[
 ('go api.eventLoop()','ksOwnedFactoryCapture(api)\ngo api.eventLoop()'),
 ('syncSubscriptions[i] = struct{}{}','syncSubscriptions[i] = struct{}{}\nksOwnedLifecycle("install",i,syncSubscriptions)'),
 ('delete(syncSubscriptions, u.c)','ksOwnedLifecycle("before_delete",u.c,syncSubscriptions)\ndelete(syncSubscriptions,u.c)\nksOwnedLifecycle("after_delete",u.c,syncSubscriptions)'),
 ('close(u.uninstalled)','ksOwnedLifecycle("ack_close_begin",u.c,syncSubscriptions)\nclose(u.uninstalled)\nksOwnedLifecycle("ack_close_end",u.c,syncSubscriptions)'),
 ('i <- false','ksOwnedLifecycle("send_begin_install_false",i,syncSubscriptions)\ni <- false\nksOwnedLifecycle("send_end_install_false",i,syncSubscriptions)'),
 ('c <- notification','ksOwnedLifecycle("send_begin_progress",c,syncSubscriptions)\nc <- notification\nksOwnedLifecycle("send_end_progress",c,syncSubscriptions)'),
 ('c <- false','ksOwnedLifecycle("send_begin_done_false",c,syncSubscriptions)\nc <- false\nksOwnedLifecycle("send_end_done_false",c,syncSubscriptions)'),
 ('func (s *SyncStatusSubscription) Unsubscribe() {','func (s *SyncStatusSubscription) Unsubscribe() {\nksOwnedLifecycle("method_entry",s.c,nil)\ndefer ksOwnedLifecycle("method_return_defer",s.c,nil)'),
 ('case <-req.uninstalled:','case <-req.uninstalled:\nksOwnedLifecycle("method_ack_observed",s.c,nil)'),
 ('case <-s.c:','case <-s.c:\nksOwnedLifecycle("method_status_received_after_io",s.c,nil)')]
 for old,new in replacements:
  count=text.count(old);expected=0 if role=='MUT-EARLY'and old in ('case <-req.uninstalled:','case <-s.c:')else 2 if role=='CAND-A'and old=='case <-s.c:'else 1
  if count!=expected:raise ValueError('nonunique source anchor '+old)
  counts[old]=count;text=text.replace(old,new)
 return subprocess.check_output(['/Users/kevin/.gvm/gos/go1.23.12/bin/gofmt'],input=text.encode()),counts

def flat_tree(src,tree):
 rows=[]
 for row in m.git(src,'ls-tree','-rz',tree).split(b'\0'):
  if not row:continue
  header,name=row.split(b'\t',1);mode,kind,oid=header.decode().split();rows.append(dict(path=name.decode(),mode=mode,kind=kind,oid=oid))
 return rows

def tree_input(src,q,env,files):
 local={**env,'GIT_INDEX_FILE':str(q/'owned-index')};subprocess.run(['git','read-tree',COMMIT],cwd=src,env=local,check=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 objects={}
 for name,data in files:
  h=subprocess.check_output(['git','hash-object','-w','--stdin'],input=data,cwd=src,env=local).decode().strip();objects[name]=h;subprocess.run(['git','update-index','--add','--cacheinfo','100644,'+h+','+name],cwd=src,env=local,check=True)
  if name==API:project=subprocess.check_output(['git','write-tree'],cwd=src,env=local).decode().strip()
 build=subprocess.check_output(['git','write-tree'],cwd=src,env=local).decode().strip();(q/'owned-index').unlink()
 return dict(base_commit=COMMIT,applied_project_commit=None,applied_project_tree=project,build_input_tree=build,objects=objects,project_flat_tree=flat_tree(src,project),build_flat_tree=flat_tree(src,build))

def run(work,module_cache):
 work.mkdir();out=work/'evidence';out.mkdir();go='/Users/kevin/.gvm/gos/go1.23.12/bin/go'
 dep=json.loads(Path('/private/tmp/ks-wi4-target-20261019/dependency-plan.json').read_text());settings={**dep['task_environment'],'GOMODCACHE':str(module_cache),'GOCACHE':'/private/tmp/ks-ticket-rpc-20261019/go-cache'}
 env={k:os.environ[k]for k in ('HOME','USER','TMPDIR','LANG','LC_ALL')if k in os.environ};env.update(settings);env['PATH']=str(Path(go).parent)+':/usr/bin:/bin:/usr/sbin:/sbin'
 base_source=Path('/private/tmp/ks-wi4-target-20261019/source');baseline_expected=json.loads((ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/corrective/source-binding.json').read_text())['tracked']
 overlays={};anchors={};candidates={r['id']:r for r in json.loads((DRAFT/'private/candidates.json').read_text())}
 for role in ROLES:
  data=(DRAFT/'private/overlays'/(role+'.go.txt')).read_bytes()
  if m.sha(data)!=candidates[role]['full_file_overlay_sha256']:raise ValueError('candidate source changed')
  overlays[role],anchors[role]=instrument(data,role)
 slots=[dict(slot_id=role+'-'+variant,candidate=role,variant=variant,repetition=1)for role in ROLES for variant in ('general','race')]
 scope=dict(kind='real_factory_observed_owner_schedules_DEV_not_independent',source_commit=COMMIT,driver_sha256=m.sha(Path(__file__).read_bytes()),bridge_fixture_sha256=m.sha(BRIDGE_FIXTURE.read_bytes()),main_fixture_sha256=m.sha(MAIN_FIXTURE.read_bytes()),instrumented_api_sha256={r:m.sha(b)for r,b in overlays.items()},source_anchors=anchors,planned_slots=slots,planned_node_attempts=10,build_roles=['ORIGINAL','CAND-A','CAND-B','MUT-EARLY'],alias_roles={'MUT-REINTRO':'ORIGINAL'},runtime_parallelism=2,build_parallelism=1,automatic_retries=0,program_timeout_seconds=100,build_timeout_seconds=900,original_timer_seconds=60,post_done_owner_schedules=['normal_delivery_then_unsubscribe','blocked_delivery_then_consumer_exit','confirmation_delayed','concurrent_unsubscribe_once'],late_after_accept_injected=False,ack_gate='only before actual owner close; deletion has already happened; release after frozen observation',observation_ms=100,ack_entry_wait_ms=250,method_join_seconds=2,real_network_sync=False,owned_test_nodes_planned=10,operational_nodes_started=0,paid_or_external_LLM_calls=0,full_original5_complete=False,independent_approved=0,formal_completed=0,freeze=None,execution_allowed=False)
 m.write(out/'SCOPE.json',scope);m.write(out/'environment.json',dict(allowed_inherited_keys=sorted(set(env)&{'HOME','USER','TMPDIR','LANG','LC_ALL'}),task_settings=settings,PATH=env['PATH'],credential_environment_inherited=False))
 original=m.source_inventory(base_source);original_status=m.git(base_source,'status','--porcelain','--untracked-files=all');m.write(out/'original-before.json',original);(out/'original-status-before.txt').write_bytes(original_status)
 built={}
 for role in scope['build_roles']:
  q=out/'candidates'/role;q.mkdir(parents=True);src=work/('source-'+role)
  for label,argv,cwd in [('clone',['git','-c','core.hooksPath=/dev/null','clone','--shared','--no-checkout',str(base_source),str(src)],work),('checkout',['git','-c','core.hooksPath=/dev/null','checkout','--detach',COMMIT],src)]:
   r=m.command(argv,cwd,q/label,env,60)
   if r['exit_code']or r['timeout']or r['remaining_live_group_members']:raise ValueError('restore failed '+role)
  baseline=m.source_inventory(src)
  if baseline!=baseline_expected or m.git(src,'status','--porcelain'):raise ValueError('restored baseline changed')
  files=[(API,overlays[role]),(BRIDGE,BRIDGE_FIXTURE.read_bytes()),(MAIN,MAIN_FIXTURE.read_bytes())]
  for name,data in files:target=src/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
  expected=dict(baseline);expected[API]=dict(mode=baseline[API]['mode'],sha256=m.sha(overlays[role]));status=m.git(src,'status','--porcelain','--untracked-files=all').decode()
  if m.source_inventory(src)!=expected or set(status.splitlines())!={' M '+API,'?? '+BRIDGE,'?? '+MAIN}:raise ValueError('undeclared project changes')
  m.write(q/'source-binding.json',dict(source_commit=COMMIT,candidate=role,base_tracked=baseline,applied_tracked=expected,additions={BRIDGE:m.sha(BRIDGE_FIXTURE.read_bytes()),MAIN:m.sha(MAIN_FIXTURE.read_bytes())},instrumented_api_sha256=m.sha(overlays[role]),production_patch=False))
  m.write(q/'tree-binding.json',tree_input(src,q,env,files));(q/'api.go').write_bytes(overlays[role]);binaries={}
  for variant in ('general','race'):
   binary=q/(variant+'.program');r=m.command([go,'build']+(['-race']if variant=='race'else[])+['-o',str(binary),'./cmd/ks-owned-full-host-owner'],src,q/('build-'+variant),env,900)
   if r['exit_code']or r['timeout']or r['remaining_live_group_members']:raise ValueError('full host owner build failed '+role+' '+variant)
   binaries[variant]=dict(path=str(binary),sha256=m.sha(binary.read_bytes()))
  m.write(q/'binaries.json',binaries);built[role]=dict(src=src,binaries=binaries,expected=expected,status=status)
 built['MUT-REINTRO']=built['ORIGINAL'];attempts=[]
 def execute(slot):
  q=out/'slots'/slot['slot_id'];q.mkdir(parents=True);tmp=q/'tmp';tmp.mkdir();d=built[slot['candidate']];b=d['binaries'][slot['variant']]
  r=m.command([b['path']],d['src'],q/'process',{**env,'TMPDIR':str(tmp),'KS_OWNER_TRACE':str(q/'owner.jsonl')},scope['program_timeout_seconds']);r.update(slot,binary_sha256=b['sha256'],tmp_children_after=sorted(p.name for p in tmp.iterdir()));m.write(q/'receipt.json',r);print(json.dumps(dict(slot_id=slot['slot_id'],exit_code=r['exit_code'],timeout=r['timeout'],remaining_live_group_members=r['remaining_live_group_members'])),flush=True);return r
 with concurrent.futures.ThreadPoolExecutor(max_workers=2)as pool:
  for result in pool.map(execute,slots):attempts.append(result);m.write(out/'attempts.json',attempts)
 for role in scope['build_roles']:
  d=built[role];after=m.source_inventory(d['src']);m.write(out/'candidates'/role/'source-after.json',after)
  if after!=d['expected']or m.git(d['src'],'status','--porcelain','--untracked-files=all').decode()!=d['status']or(d['src']/BRIDGE).read_bytes()!=BRIDGE_FIXTURE.read_bytes()or(d['src']/MAIN).read_bytes()!=MAIN_FIXTURE.read_bytes():raise ValueError('postrun source changed')
  if any(m.sha(Path(b['path']).read_bytes())!=b['sha256']for b in d['binaries'].values()):raise ValueError('postrun binary changed')
 after=m.source_inventory(base_source);m.write(out/'original-after.json',after);(out/'original-status-after.txt').write_bytes(m.git(base_source,'status','--porcelain','--untracked-files=all'))
 if after!=original or m.git(base_source,'status','--porcelain','--untracked-files=all')!=original_status:raise ValueError('original changed')
 m.write(out/'receipt.json',dict(planned=10,attempted=len(attempts),automatic_retries=0,individual_program_failures=sum(r['exit_code']!=0 for r in attempts),full_original5_complete=False,formal_completed=0,independent_approved=0,production_patch=False))

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);p.add_argument('--module-cache',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve(),a.module_cache.resolve())
