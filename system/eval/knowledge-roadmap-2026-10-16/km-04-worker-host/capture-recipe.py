import ast,gzip,hashlib,io,json,subprocess,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/km-04-worker-host';base.mkdir(parents=True,exist_ok=True);fixture=Path('/private/tmp/ks-KM043C1-worker-host-final3-20261016')
def sha(d):return hashlib.sha256(d).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def put(n,data,compressed=False):
 p=base/(n+('.gz'if compressed else''));p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
for p in fixture.iterdir():
 if p.is_file():put('runtime/'+p.name,p.read_bytes())
last=json.loads((fixture/'budget-recovered.json').read_text());row=next(r for r in last['state']['attempts']if r['id']==last['state']['last_success']);final=row['request']['new_version'];stream=io.BytesIO()
def safe(info):
 if info.issym():
  if info.name!='fixture/dataset/current'or info.linkname!=final:raise ValueError('unexpected fixture symlink')
 elif not(info.isfile()or info.isdir()):raise ValueError('unsafe host fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:
 for name in ('source','manager','dataset'):archive.add(fixture/name,arcname='fixture/'+name,filter=safe)
put('fixture.tar',stream.getvalue(),True)
for src,dst in [('ks-KM043C1-build-lint.txt','build-lint.txt'),('ks-KM043C1-history.json','historical-design.json'),('ks-KM043C1-plugin-validate.json','plugin-validate.json')]:put(dst,Path('/private/tmp',src).read_bytes())
for src,dst in [('ks-KM043C1-docs-final.txt','docs-final.txt'),('ks-KM043C1-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
put('initial-observation/runtime.txt',Path('/private/tmp/ks-KM043C1-runtime-initial.txt').read_bytes())
write('initial-observation/scope.json',{'checks':21,'result':'passed','final_run_addition':'Record observed_monotonic explicitly and assert the actual observation is at least nine seconds after the controlled graph start; retain the earlier successful log without counting it as another independent case.'})
prior=root/'system/eval/knowledge-roadmap-2026-10-15/km-04-worker';old=json.loads((prior/'source-binding.json').read_text());b=json.loads(json.dumps(old));b['scope']='KM04_3_C1_actual_foreground_process_stop_kill_budget_not_MCP_operation';b['previous_source_binding']={'revision':'a801f551','path':'system/eval/knowledge-roadmap-2026-10-15/km-04-worker/source-binding.json'};b['added_sources']=[];b['changed_preexisting_sources']=[]
b['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(b['source_sha256'])};b['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if b['source_sha256'][n]!=h);write('source-binding.json',b)
oldv=json.loads((prior/'validation.json').read_text());v={'gates':{'build':0,'lint':0,'runtime':0,'historical_design':0,'docs':0},'current_Go_validation_reused':True,'Go_inputs_unchanged':True,'current_manager_Python_rerun':True,'manager_python_tests':248,'module':oldv['module'],'race':oldv['race'],'Go_reference':{'revision':subprocess.check_output(['git','rev-parse','a801f551'],text=True).strip(),'manifest':(prior/'manifest.json').relative_to(root).as_posix(),'manifest_sha256':sha((prior/'manifest.json').read_bytes()),'maintained_Go_files_equal':1258}};write('validation.json',v)
put('python-tests.txt',Path('/private/tmp/ks-KM043C1-python-final-tests.txt').read_bytes())
labels=('registration','collection','review','broker','update_contract','update_inputs','source_check','update_state','update_worker')
write('python-test-inventory.json',{'scripts/test_knowledge_manager_'+label+'.py':sorted(n.name for n in ast.walk(ast.parse((root/('scripts/test_knowledge_manager_'+label+'.py')).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))for label in labels})
put('initial-failures/unreported-persistence-boundary.txt',Path('/private/tmp/ks-KM043C1-runtime-final.txt').read_bytes())
put('initial-failures/unreported-stop.json',Path('/private/tmp/ks-KM043C1-worker-host-final-20261016/sigterm-stopped.json').read_bytes())
write('initial-failures/scope.json',{'actual_observation':'SIGTERM during a state commit can poison persistence and leave the last-confirmed memory phase building. Child shutdown and intent preservation still completed, but the CLI lacked an explicit uncertainty report.','fix':'Report last_confirmed_in_process_state and disk_state_requires_reconciliation; never perform more writes in a poisoned process. Signal stop reports closed_requires_persistence_reconciliation.','deterministic_regression':'Interrupt Journal directory fsync after an actual admitted native job, verify uncertainty output and exact recovery from disk.','final_Python_cases':248,'independent_acceptance':False})
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
paths=set(b['source_sha256'])|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/verify-manager-worker-host.py','scripts/check-manager-worker-host-evidence.py','scripts/test_manager_worker_host_evidence.py','docs/spec-driven/KNOWLEDGE-MANAGER-UPDATES.md','docs/DOCUMENTATION-CATALOG.json'}|{'scripts/test_knowledge_manager_'+label+'.py'for label in labels}
m={'schema_version':1,'checkpoint':'KM04_3_C1_actual_worker_process_lifecycle','actual_worker_process_lifecycle_verified':True,'manager_MCP_worker_operation_verified':False,'sustained_schedule_verified':False,'full_KM04_3_complete':False,'full_KM04_4_complete':False,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();record={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':record['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(record)
write('manifest.json',m);print(json.dumps({'bound_inputs':len(b['source_sha256']),'actual_CLI_checks':21,'worker_process_cases':3,'CLI_commands':10,'recovery_publications':3,'Go_reference':'a801f551','fresh_Python':248}))
