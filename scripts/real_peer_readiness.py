"""Unmodified-source real loopback sync; no production or independent acceptance."""
import argparse,gzip,json,subprocess
from pathlib import Path
import intent_startup_batch as native
ROOT=native.ROOT
PACKET=native.PACKET.parent/'real-peer-sync-v1'
TOOLS=['scripts/'+n for n in ('real_peer_readiness.py','test_real_peer_readiness.py','run-real-peer-sync.py','run-wi4-target-subscription.py','fixtures/real-peer-sync.go.fixture')]
read,raw,sha,write,compress=native.read,native.raw,native.sha,native.write,native.compress

def require(v,msg):
 if not v:raise ValueError(msg)

def inspect(p):
 scope=read(p/'evidence/SCOPE.json.gz');receipt=read(p/'evidence/receipt.json.gz');binding=read(p/'evidence/source-binding.json.gz')
 require(scope['source_commit']=='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8' and scope['actual_sync_not_mux_injection'] and scope['automatic_retries']==0 and scope['consensus_fixture']=='legacy_PoW_ethash_not_WBFT','source/sync scope changed')
 fixture=raw(p/'source-program.go.txt.gz')
 require(sha(fixture)==scope['fixture_sha256']==binding['addition']['sha256'] and b'.Post(' not in fixture and b'.AddPeer(' in fixture and b'eth.New(' in fixture and b'node.New(' in fixture,'actual fixture replaced with mux injection or different source')
 require(scope['driver_sha256']==sha(raw(p/'executed-driver.py.gz')), 'executed owner runner changed')
 require(scope['original_timer_seconds']==60 and scope['variants']==['general','race'] and scope['repetitions']==1 and scope['full5_schedules12_contracts_complete'] is False and scope['formal_completed']==scope['independent_approved']==scope['FINAL']==scope['paid_or_external_LLM_calls']==scope['operational_nodes_started']==0,'schedule population changed')
 require(len(binding['tracked'])==2228 and binding['modified_existing_project_files']==[],'upstream source changed')
 for name in ['original-before','original-after']:
  require(read(p/'evidence'/(name+'.json.gz'))==binding['tracked'],'original source population changed')
 require(raw(p/'evidence/original-status-before.txt.gz')==raw(p/'evidence/original-status-after.txt.gz'),'original checkout status changed')
 require(receipt==dict(planned=2,attempted=2,all_processes_clean=True,source_tracked_preserved=True,original_preserved=True,existing_project_modifications=0,program_additions=1,automatic_retries=0,formal_completed=0,independent_approved=0,full_matrix_complete=False),'peer run receipt promoted')
 require(read(p/'evidence/profile.json.gz')==scope['target_toolchain']==dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'),'upstream toolchain differs')
 binaries=read(p/'evidence/binaries.json.gz');attempts=read(p/'evidence/attempts.json.gz');results=[]
 for index,variant in enumerate(['general','race']):
  build=read(p/'evidence'/('build-'+variant)/'receipt.json.gz');q=p/'evidence/slots'/variant;run=read(q/'receipt.json.gz')
  require(run==attempts[index] and run['binary_sha256']==binaries[variant]['sha256'],'program binding differs')
  for label,r in [('build-'+variant,build),(variant,run)]:
   require(r['exit_code']==0 and not r['timeout'] and not r['term_sent'] and not r['kill_sent'] and r['reaped'] and r['remaining_live_group_members']==0 and r['owned_new_session'],'peer build/run failed or forced cleanup')
   logs=p/'evidence'/('build-'+variant) if label.startswith('build-') else q/'process'
   require(all(sha(raw(logs/(stream+'.gz')))==r[stream+'_sha256'] for stream in ['stdout','stderr']),'peer raw stream changed')
  require(run['tmp_children_after']==[],'owned node data remains')
  rows=native.events(raw(q/'process/stdout.gz'));names=[r['event']for r in rows]
  expected=['factory_started','factory_started','seeder_imported','rpc_subscription_response','loopback_peer_added','actual_sync_start','actual_sync_done','actual_receiver_head','actual_indexer_complete','actual_downloader_progress','native_timer_notification','rpc_unsubscribe_returned','receiver_close_returned','seeder_close_returned','complete']
  require(names==expected,'actual source sync/notification/cleanup trace incomplete')
  values={r['event']:r['value']for r in rows}
  for row in rows[:2]:require(row['value']['node_ip']=='127.0.0.1' and row['value']['network_id']==71377 and row['value']['discovery'] is False and row['value']['real_downloader'],'non-isolated node fixture')
  require(values['loopback_peer_added']=='127.0.0.1' and values['seeder_imported']['blocks']==3 and values['actual_receiver_head']==dict(number=3,hash=values['seeder_imported']['head'],matches_seeder=True),'actual receiver did not reach seeded head')
  require(values['actual_indexer_complete']==dict(Indexed=4,Remaining=0),'indexer not complete')
  require(values['actual_downloader_progress']['CurrentBlock']==values['actual_downloader_progress']['HighestBlock']==3,'downloader not complete')
  require(values['native_timer_notification'] is False and next(r['elapsed_ns'] for r in rows if r['event']=='native_timer_notification')>=55_000_000_000,'original60s notification missing/shortened')
  require(values['rpc_unsubscribe_returned'] is True and values['receiver_close_returned'] is True and values['seeder_close_returned'] is True and values['complete']['formal_completed']==values['complete']['independent_approved']==0,'cleanup or acceptance state differs')
  results.append(dict(variant=variant,actual_peer_nodes=2,source_sync_started=True,source_sync_done=True,matching_blocks=3,indexed=4,index_remaining=0,native60_false=True,process_exit=0,forced_cleanup=False,process_groups_remaining=0,tmp_children=0))
 return dict(kind='actual_loopback_peer_sync_DEV_not_independent_acceptance',runs=results,actual_nodes=4,automatic_retries=0,unmodified_upstream_files=2228,source_commit=scope['source_commit'],consensus='legacy_PoW_ethash_fixture_not_WBFT_production',full5_schedules12_contracts_complete=False,independent_families=0,correlated_fixture_families=1,repetitions_per_profile=1,independent_approved=0,formal_completed=0,FINAL=0,freeze=None,quality_score=None,actual_LLM_calls=0,external_peer_dial=False,scope_event_labels='inherited_initial_labels_inaccurate; exact_preexecuted_fixture_and_raw_events_are_authority',client_error_channel_closed='not_explicitly_observed; unsubscribe_return_observed')

def reference():return dict(kind='actual_loopback_peer_sync_DEV_not_independent_acceptance',manifest=str(PACKET/'manifest.json'),manifest_sha256=sha((ROOT/PACKET/'manifest.json').read_bytes()))
def seal(p):write(p/'manifest.json',dict(tools={n:sha((ROOT/n).read_bytes())for n in TOOLS},artifacts={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'}))
def check(packet=None):
 p=packet or ROOT/PACKET
 try:
  m=read(p/'manifest.json');require(m['tools']=={n:sha((ROOT/n).read_bytes())for n in TOOLS}and m['artifacts']=={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'},'peer evidence/tools changed')
  require(read(p/'SUMMARY.json')==inspect(p),'peer evidence summary changed/promoted')
  require(not native.validate(),'current native source evidence differs')
  if packet is None:
   for n in native.PATHS:require(read(ROOT/n)['latest_real_peer_followup']==reference(),'peer current index differs')
  return []
 except(ValueError,OSError,KeyError,TypeError,IndexError)as e:return[str(e)]
def prepare(work):
 p=ROOT/PACKET;require(not p.exists(),'refusing peer evidence overwrite');p.mkdir()
 for x in (work/'evidence').rglob('*'):
  if not x.is_file()or x.name.endswith('.program'):continue
  compress(p/(str(x.relative_to(work))+'.gz'),x.read_bytes())
 compress(p/'executed-driver.py.gz',Path('/private/tmp/run-phase78-real-peer.py').read_bytes())
 compress(p/'source-program.go.txt.gz',(ROOT/'scripts/fixtures/real-peer-sync.go.fixture').read_bytes())
 write(p/'SUMMARY.json',inspect(p));seal(p)
def update_indices():
 for n in native.PATHS:
  d=read(ROOT/n);d['latest_real_peer_followup']=reference();write(ROOT/n,d)
 native.update_indices()
def stored():
 revision=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip()
 require(bool(revision),'peer packet not committed')
 paths=subprocess.check_output(['git','diff-tree','--root','--no-commit-id','--name-only','-r',revision],cwd=ROOT,text=True).splitlines()
 for name in paths:require(subprocess.check_output(['git','show',revision+':'+name],cwd=ROOT)==(ROOT/name).read_bytes(),'stored changed blob differs: '+name)
 require(subprocess.run(['git','verify-commit',revision],cwd=ROOT,capture_output=True).returncode==0,'peer signature not verified')
 return dict(revision=revision,changed_blobs_verified=len(paths),signature_verified=True)
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('command',choices=['prepare','check','update-indices']);parser.add_argument('--workspace',type=Path);a=parser.parse_args()
 if a.command=='prepare':prepare(a.workspace)
 elif a.command=='update-indices':update_indices()
 else:
  errors=check()
  if errors:raise SystemExit('\n'.join(errors))
  print(json.dumps(read(ROOT/PACKET/'SUMMARY.json'),indent=2))
