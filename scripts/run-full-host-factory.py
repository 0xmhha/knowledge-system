import argparse,importlib.util,json,os,shutil,subprocess
from pathlib import Path
ROOT=Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
spec=importlib.util.spec_from_file_location('owned',ROOT/'scripts/run-wi4-target-subscription.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
FIXTURE=ROOT/'scripts/fixtures/full-host-factory.go.fixture'
def run(WORK, module_cache):
 WORK.mkdir();out=WORK/'evidence';out.mkdir()
 go='/Users/kevin/.gvm/gos/go1.23.12/bin/go'
 settings=json.loads(Path('/private/tmp/ks-wi4-target-20261019/dependency-plan.json').read_text())['task_environment']
 settings.update(GOMODCACHE=str(module_cache),GOCACHE='/private/tmp/ks-ticket-rpc-20261019/go-cache')
 env={k:os.environ[k]for k in ('HOME','USER','TMPDIR','LANG','LC_ALL')if k in os.environ};env.update(settings);env['PATH']=str(Path(go).parent)+':/usr/bin:/bin:/usr/sbin:/sbin'
 scope=dict(explicit_corrective_attempt=4,previous_attempts=['missing-offline-dependency','genesis-only-indexer-incomplete','test-generator-nil-inner-close','generated-block-wrong-difficulty'],correction='same original API/timer; closable generator with explicit merged PoS block difficulty; complete only after run defers return',indexer_ready_timeout_seconds=5,import_blocks=1,driver_sha256=m.sha(Path(__file__).read_bytes()),kind='WI_V1_actual_node_eth_factory_DEV_no_independent_acceptance',source_commit=m.COMMIT,fixture_sha256=m.sha(FIXTURE.read_bytes()),target_toolchain=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'),variants=['general','race'],repetitions=1,program_timeout_seconds=95,build_timeout_seconds=900,automatic_retries=0,original_timer_seconds=60,modified_existing_project_files=[],program_addition='cmd/ks-owned-full-host/main.go',operational_nodes_started=0,owned_test_nodes_planned=2,external_P2P_dial=False,external_listener=False,real_downloader=True,tx_indexer_must_be_enabled=True,consensus_fixture='merged_ethash_not_WBFT',full5_schedules12_contracts_complete=False,installed_role_oracle=False,formal_completed=0,independent_approved=0,FINAL=0,paid_or_external_LLM_calls=0,planned_events=['factory_started','actual_block_imported','active_tx_index_ready','real_tx_index_progress','real_downloader_progress','rpc_syncing','rpc_subscription_response','start_event_posted','native_timer_notification','rpc_unsubscribe_returned','client_subscription_error_channel_closed','node_close_returned','complete'])
 m.write(out/'SCOPE.json',scope);m.write(out/'environment.json',dict(allowed_inherited_keys=sorted(set(env)&{'HOME','USER','TMPDIR','LANG','LC_ALL'}),task_settings=settings,PATH=env['PATH'],credential_environment_inherited=False))
 src=WORK/'source';parent=Path('/private/tmp/ks-wi4-target-20261019/source')
 original_status=m.git(parent,'status','--porcelain','--untracked-files=all');original=m.source_inventory(parent)
 m.write(out/'original-before.json',original);(out/'original-status-before.txt').write_bytes(original_status)
 commands=[('clone',['git','-c','core.hooksPath=/dev/null','clone','--shared','--no-checkout',str(parent),str(src)],WORK),('checkout',['git','-c','core.hooksPath=/dev/null','checkout','--detach',m.COMMIT],src)]
 for label,argv,cwd in commands:
  r=m.command(argv,cwd,out/label,env,60)
  if r['exit_code']or r['timeout']or r['remaining_live_group_members']:raise ValueError(label+' failed')
 if m.git(src,'status','--porcelain'):raise ValueError('restored source dirty')
 base=m.source_inventory(src)
 expected=json.loads(Path('/private/tmp/ks-wi4-target-20261019/subscription-trials-v1/source-binding.json').read_text())['tracked']
 if base!=expected:raise ValueError('original tracked source binding differs')
 target=src/scope['program_addition'];target.parent.mkdir();target.write_bytes(FIXTURE.read_bytes())
 m.write(out/'source-binding.json',dict(source_commit=m.COMMIT,tracked=base,addition=dict(path=scope['program_addition'],sha256=m.sha(target.read_bytes())),modified_existing_project_files=[]))
 profile=json.loads(subprocess.check_output([go,'env','-json','GOVERSION','GOOS','GOARCH','CGO_ENABLED'],cwd=src,env=env));m.write(out/'profile.json',profile)
 if profile!=scope['target_toolchain']:raise ValueError('profile differs')
 binaries={};attempts=[]
 for variant in scope['variants']:
  binary=out/(variant+'.program');argv=[go,'build']+(['-race']if variant=='race'else[])+['-o',str(binary),'./cmd/ks-owned-full-host']
  r=m.command(argv,src,out/('build-'+variant),env,900)
  if r['exit_code']or r['timeout']or r['remaining_live_group_members']:raise ValueError('build failed '+variant)
  binaries[variant]=dict(path=str(binary),sha256=m.sha(binary.read_bytes()))
 m.write(out/'binaries.json',binaries)
 for variant in scope['variants']:
  q=out/'slots'/variant;q.mkdir(parents=True);tmp=q/'tmp';tmp.mkdir()
  local={**env,'TMPDIR':str(tmp)};r=m.command([binaries[variant]['path']],src,q/'process',local,scope['program_timeout_seconds'])
  r.update(variant=variant,binary_sha256=binaries[variant]['sha256'],tmp_children_after=sorted(x.name for x in tmp.iterdir()));m.write(q/'receipt.json',r);attempts.append(r);m.write(out/'attempts.json',attempts)
  print(json.dumps(dict(variant=variant,exit_code=r['exit_code'],timeout=r['timeout'],remaining_live_group_members=r['remaining_live_group_members'])),flush=True)
 if m.source_inventory(src)!=base or m.git(src,'status','--porcelain','--untracked-files=all').decode()!='?? '+scope['program_addition']+'\n' or target.read_bytes()!=FIXTURE.read_bytes():raise ValueError('owned source changed')
 original_after=m.source_inventory(parent);m.write(out/'original-after.json',original_after);(out/'original-status-after.txt').write_bytes(m.git(parent,'status','--porcelain','--untracked-files=all'))
 if original_after!=original or m.git(parent,'status','--porcelain','--untracked-files=all')!=original_status:raise ValueError('original changed')
 if any(m.sha(Path(v['path']).read_bytes())!=v['sha256']for v in binaries.values()):raise ValueError('binary changed')
 m.write(out/'receipt.json',dict(planned=2,attempted=len(attempts),all_processes_clean=all(not r['remaining_live_group_members']and r['reaped']for r in attempts),source_tracked_preserved=True,original_preserved=True,existing_project_modifications=0,program_additions=1,automatic_retries=0,formal_completed=0,independent_approved=0,full_matrix_complete=False))
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);p.add_argument('--module-cache',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve(),a.module_cache.resolve())
