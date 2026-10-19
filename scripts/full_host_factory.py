"""Replay an unchanged actual node factory probe without promoting DEV to acceptance."""
import argparse
import gzip
import hashlib
import json
import subprocess
from pathlib import Path
import wi6_version_readiness as history

ROOT=Path(__file__).resolve().parents[1]
PACKET=Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/full-host-factory-v1')
NATIVE=PACKET.parent/'local-reliability-v1'
REVISION='38b4c5e279b5bb09153a0c80838f36f4ccf3b0be'
COMMIT='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'
TOOLS=['scripts/full_host_factory.py','scripts/test_full_host_factory.py','scripts/run-full-host-factory.py','scripts/fixtures/full-host-factory.go.fixture']
EVENTS=['factory_started','actual_block_imported','active_tx_index_ready','real_tx_index_progress','real_downloader_progress','rpc_syncing','rpc_subscription_response','start_event_posted','native_timer_notification','rpc_unsubscribe_returned','client_subscription_error_channel_closed','node_close_returned','complete']

def sha(raw):return hashlib.sha256(raw).hexdigest()
def write(path,data):path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
def read(path):
 raw=path.read_bytes()
 return json.loads(gzip.decompress(raw)if path.suffix=='.gz'else raw)
def raw(path):return gzip.decompress(path.read_bytes())
def events(data):return [json.loads(line)for line in data.splitlines()if line.strip()]
def receipt(p,code=0):
 r=read(p)
 if r['exit_code']!=code or r['timeout']or r['term_sent']or r['kill_sent']or not r['reaped']or r['remaining_live_group_members']:
  raise ValueError('owned process outcome or cleanup differs: '+str(p))
 for suffix in ('stdout','stderr'):
  if sha(raw(p.parent/(suffix+'.gz')))!=r[suffix+'_sha256']:raise ValueError('owned command raw log hash differs')
 return r

def trace(rows):
 if [r['event']for r in rows]!=EVENTS:raise ValueError('full factory event population/order differs')
 if any(a['elapsed_ns']>b['elapsed_ns']for a,b in zip(rows,rows[1:])):raise ValueError('event clock order differs')
 data={r['event']:r['value']for r in rows};factory=data['factory_started']
 if factory!=dict(consensus_fixture='merged_ethash_not_WBFT',listen_addr='',max_peers=0,no_dial=True,no_discovery=True,real_downloader=True,tx_lookup_limit=128):raise ValueError('actual factory isolation/identity differs')
 if data['actual_block_imported']!={'count':1,'head':1}:raise ValueError('actual chain import missing')
 for key in ('active_tx_index_ready','real_tx_index_progress'):
  if data[key]!={'Indexed':2,'Remaining':0}:raise ValueError('enabled actual indexer not complete')
 if data['real_downloader_progress']['CurrentBlock']!=1 or data['rpc_syncing']is not False or data['rpc_subscription_response']is not True or data['start_event_posted']is not True or data['native_timer_notification']is not False:raise ValueError('actual progress/RPC notification not complete')
 notification=next(r for r in rows if r['event']=='native_timer_notification')
 if not 59_000_000_000<=notification['elapsed_ns']<=95_000_000_000:raise ValueError('original native timer outside declared observation window')
 if data['client_subscription_error_channel_closed']is not True or data['node_close_returned']['error']!='<nil>':raise ValueError('subscription or node close not acknowledged')
 if data['complete']!=dict(formal_completed=0,independent_approved=0,original_timer_seconds=60,full_matrix_complete=False):raise ValueError('bounded DEV promoted to complete matrix or independent acceptance')
 return dict(native_notification_elapsed_ns=notification['elapsed_ns'],unsubscribe_elapsed_ns=data['rpc_unsubscribe_returned']['elapsed_ns'],close_elapsed_ns=data['node_close_returned']['elapsed_ns'])

def derive(root=ROOT,packet=None):
 p=packet or root/PACKET
 historical=history.snapshot(root,NATIVE,REVISION)
 binding=read(root/NATIVE/'source-binding.json')
 if any(sha((root/n).read_bytes())!=h for n,h in {**binding['source_sha256'],**binding['approved_input_sha256']}.items()):raise ValueError('current native source changed after verified checkpoint')
 live=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True).splitlines()
 if {n for n in live if n.startswith(('cmd/','pkg/','internal/'))}!={n for n in binding['source_sha256']if n.endswith('.go')}:raise ValueError('native source inventory changed')
 scope=read(p/'actual/SCOPE.json.gz')
 if scope['driver_sha256']!=sha((root/TOOLS[2]).read_bytes())or scope['fixture_sha256']!=sha((root/TOOLS[3]).read_bytes())or scope['source_commit']!=COMMIT or scope['variants']!=['general','race']or scope['repetitions']!=1 or scope['automatic_retries']!=0 or scope['import_blocks']!=1 or scope['indexer_ready_timeout_seconds']!=5 or scope['program_timeout_seconds']!=95 or scope['original_timer_seconds']!=60 or scope['planned_events']!=EVENTS or scope['explicit_corrective_attempt']!=4 or scope['modified_existing_project_files']!=[] or scope['full5_schedules12_contracts_complete']is not False:raise ValueError('actual full-host scope changed')
 base=read(p/'actual/source-binding.json.gz')
 if base['source_commit']!=COMMIT or base['modified_existing_project_files']!=[] or base['addition']!=dict(path='cmd/ks-owned-full-host/main.go',sha256=sha((root/TOOLS[3]).read_bytes()))or base['tracked']!=read(p/'actual/owned-after.json.gz'):raise ValueError('original host source or single addition changed')
 # Compare against the already retained exact upstream source, not only a success flag.
 upstream=json.loads(history.read_at(root,REVISION,PACKET.parent/'wi4-target-subscription-v1/corrective/source-binding.json'))
 if upstream['tracked']!=base['tracked']:raise ValueError('upstream source population differs')
 if raw(p/'actual/owned-status-after.txt.gz').decode()!='?? cmd/ks-owned-full-host/main.go\n':raise ValueError('undeclared project file modification')
 if read(p/'actual/original-before.json.gz')!=read(p/'actual/original-after.json.gz')or raw(p/'actual/original-status-before.txt.gz')!=raw(p/'actual/original-status-after.txt.gz'):raise ValueError('original source changed')
 if read(p/'actual/profile.json.gz')!=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'):raise ValueError('actual toolchain differs')
 binaries=read(p/'actual/binaries.json.gz');comparisons=[]
 if read(p/'actual/binary-after.json.gz')!={variant:data['sha256']for variant,data in binaries.items()}:raise ValueError('postrun binary bytes changed')
 for variant in scope['variants']:
  build=receipt(p/'actual'/('build-'+variant)/'receipt.json.gz')
  if build['argv'][1:]!=['build']+(['-race']if variant=='race'else[])+['-o',binaries[variant]['path'],'./cmd/ks-owned-full-host']:raise ValueError('full-host build changed')
  process=p/'actual/slots'/variant/'process';r=receipt(process/'receipt.json.gz');slot=read(p/'actual/slots'/variant/'receipt.json.gz')
  for suffix in ('stdout','stderr'):
   if sha(raw(process/(suffix+'.gz')))!=r[suffix+'_sha256']:raise ValueError('runtime output hash differs')
  if raw(process/'stderr.gz')or slot['tmp_children_after']or slot['binary_sha256']!=binaries[variant]['sha256']or r['argv']!=[binaries[variant]['path']]:raise ValueError('runtime race stderr, data cleanup or binary changed')
  comparisons.append(dict(variant=variant,**trace(events(raw(process/'stdout.gz')))))
 final=read(p/'actual/receipt.json.gz')
 if final!=dict(planned=2,attempted=2,all_processes_clean=True,source_tracked_preserved=True,original_preserved=True,existing_project_modifications=0,program_additions=1,automatic_retries=0,formal_completed=0,independent_approved=0,full_matrix_complete=False):raise ValueError('final actual probe receipt differs')
 for label in ('actual','initial-missing-dependency','genesis-only-indexer-incomplete','test-generator-nil-inner-close','generated-block-wrong-difficulty'):
  archived=p/label;frozen=read(archived/'SCOPE.json.gz')
  if sha(raw(archived/'used-tools/driver.py.gz'))!=frozen['driver_sha256']or sha(raw(archived/'used-tools/fixture.go.fixture.gz'))!=frozen['fixture_sha256']:raise ValueError('earlier or final frozen tool archive differs')
 failed=p/'initial-missing-dependency';receipt(failed/'build-general/receipt.json.gz',1)
 if b'module lookup disabled by GOPROXY=off'not in raw(failed/'build-general/stderr.gz'):raise ValueError('initial offline dependency failure omitted')
 initial=p/'genesis-only-indexer-incomplete';previous=read(initial/'SCOPE.json.gz')
 for variant in ('general','race'):
  process=initial/'slots'/variant/'process';receipt(process/'receipt.json.gz',1);rows=events(raw(process/'stdout.gz'));values={r['event']:r['value']for r in rows}
  if values['real_tx_index_progress']!={'Indexed':0,'Remaining':1}or values['native_timer_notification']['syncing']is not True or values['native_timer_notification']['status']['TxIndexRemainingBlocks']!=1 or rows[-1]['event']!='error':raise ValueError('genesis-only failed observation was omitted or promoted')
  if read(initial/'slots'/variant/'receipt.json.gz')['tmp_children_after']:raise ValueError('earlier owned node data remains')
 if previous['automatic_retries']!=0 or read(initial/'original-before.json.gz')!=read(initial/'original-after.json.gz'):raise ValueError('earlier source or retry boundary changed')
 failed_cleanup=p/'test-generator-nil-inner-close'
 for variant in ('general','race'):
  process=failed_cleanup/'slots'/variant/'process';receipt(process/'receipt.json.gz',2)
  if b'consensus/beacon.(*Beacon).Close'not in raw(process/'stderr.gz')or b'invalid memory address'not in raw(process/'stderr.gz'):raise ValueError('test generator cleanup failure omitted')
  rows=events(raw(process/'stdout.gz'));trace(rows)
  if read(failed_cleanup/'slots'/variant/'receipt.json.gz')['tmp_children_after']:raise ValueError('failed generator node data remains')
 if read(failed_cleanup/'original-before.json.gz')!=read(failed_cleanup/'original-after.json.gz'):raise ValueError('failed generator original changed')
 wrong=p/'generated-block-wrong-difficulty'
 for variant in ('general','race'):
  process=wrong/'slots'/variant/'process';receipt(process/'receipt.json.gz',1)
  rows=events(raw(process/'stdout.gz'))
  if rows[-1]['event']!='error'or 'actual block import: invalid difficulty: have 131072, want 0'not in rows[-1]['value']:raise ValueError('generated block failure omitted or promoted')
  if read(wrong/'slots'/variant/'receipt.json.gz')['tmp_children_after']:raise ValueError('failed block node data remains')
 if read(wrong/'original-before.json.gz')!=read(wrong/'original-after.json.gz'):raise ValueError('failed block original changed')
 return dict(kind='actual_full_host_factory_DEV_not_independent_acceptance',source_commit=COMMIT,actual_factory_variants=2,actual_owned_node_attempts=8,earlier_genesis_only_assertion_failures=2,earlier_offline_compile_failures=1,earlier_test_generator_cleanup_panics=2,earlier_generated_block_assertion_failures=2,explicit_corrective_attempts=4,automatic_retries=0,modified_existing_project_files=0,actual_imported_blocks_each=1,real_indexed_blocks_each=2,real_index_remaining_each=0,original_timer_seconds=60,real_network_sync=False,start_event_source='owned_mux_post_not_syncWithPeer',owned_process_groups_remaining=0,original_source_preserved=True,native_checkpoint_historical_bytes_verified=historical['historical_bytes_verified'],full5_schedule12_contract_matrix_complete=False,installed_consumer_oracle=False,independent_approved=0,formal_completed=0,FINAL=0,freeze=None,execution_allowed=False,comparisons=comparisons)

def seal(p):write(p/'manifest.json',dict(kind='full_host_factory_DEV',tools={n:sha((ROOT/n).read_bytes())for n in TOOLS},artifacts={str(x.relative_to(p)):sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x.name!='manifest.json'}))
def check(root=ROOT,packet=None):
 p=packet or root/PACKET
 try:
  m=read(p/'manifest.json')
  if m['tools']!={n:sha((root/n).read_bytes())for n in TOOLS}or m['artifacts']!={str(x.relative_to(p)):sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x.name!='manifest.json'}:raise ValueError('full host evidence/tool bytes changed')
  if read(p/'SUMMARY.json')!=derive(root,p):raise ValueError('actual full host result changed or falsely promoted')
  return []
 except (ValueError,KeyError,OSError,TypeError,IndexError)as error:return [str(error)]

def prepare(actual,initial,genesis,cleanup,wrong):
 p=ROOT/PACKET
 if p.exists():raise ValueError('refusing packet overwrite')
 p.mkdir()
 for label,source in [('actual',actual),('initial-missing-dependency',initial),('genesis-only-indexer-incomplete',genesis),('test-generator-nil-inner-close',cleanup),('generated-block-wrong-difficulty',wrong)]:
  for x in source.rglob('*'):
   if not x.is_file()or x.name.endswith('.program')or x.name=='live.json'or 'tmp'in x.relative_to(source).parts:continue
   target=p/label/(str(x.relative_to(source))+'.gz');target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(gzip.compress(x.read_bytes(),mtime=0))
 write(p/'SUMMARY.json',derive());seal(p)

def stored():
 r=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip();m=read(ROOT/PACKET/'manifest.json')
 names=[*[str(PACKET/n)for n in m['artifacts']],str(PACKET/'manifest.json'),*TOOLS,'docs/spec-driven/CURRENT-HANDOFF.md','docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md','docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.md','docs/spec-driven/REMAINING-13-EXECUTION.json','docs/REMAINING-WORK.md']
 if not r or any(subprocess.check_output(['git','show',r+':'+n],cwd=ROOT)!=(ROOT/n).read_bytes()for n in names):raise ValueError('stored full host evidence/current docs differ')
 subprocess.run(['git','verify-commit',r],cwd=ROOT,check=True,capture_output=True)
 return r

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','check']);p.add_argument('--actual',type=Path);p.add_argument('--initial',type=Path);p.add_argument('--genesis',type=Path);p.add_argument('--cleanup',type=Path);p.add_argument('--wrong-block',type=Path);p.add_argument('--require-stored',action='store_true');a=p.parse_args()
 if a.command=='prepare':prepare(a.actual,a.initial,a.genesis,a.cleanup,a.wrong_block)
 else:
  errors=check()
  if errors:raise SystemExit('\n'.join(errors))
  if a.require_stored:stored()
  print(json.dumps(read(ROOT/PACKET/'SUMMARY.json'),ensure_ascii=False))
