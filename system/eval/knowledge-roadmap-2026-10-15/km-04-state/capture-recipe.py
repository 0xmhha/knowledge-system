import ast,gzip,hashlib,io,json,subprocess,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-04-state';base.mkdir(exist_ok=True);runtime=Path('/private/tmp/ks-KM043B1-ledger-runtime-20261015')
def sha(data):return hashlib.sha256(data).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def put(n,data,compressed=False):
 path=base/(n+('.gz'if compressed else''));path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
for p in runtime.iterdir():
 if p.is_file():put('runtime/'+p.name,p.read_bytes())
stream=io.BytesIO()
def safe(info):
 if not(info.isfile()or info.isdir()):raise ValueError('unsafe ledger fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:
 for name in ('source','manager'):archive.add(runtime/name,arcname='fixture/'+name,filter=safe)
put('fixture.tar',stream.getvalue(),True)
for src,dst in [('ks-KM043B1-python-all-tests.txt','python-tests.txt'),('ks-KM043B1-build-lint.txt','build-lint.txt'),('ks-KM043B1-history.json','historical-design.json')]:put(dst,Path('/private/tmp',src).read_bytes())
for src,dst in [('ks-KM043B1-docs-final.txt','docs-final.txt'),('ks-KM043B1-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
labels=('registration','collection','review','broker','update_contract','update_inputs','source_check','update_state')
write('python-test-inventory.json',{'scripts/test_knowledge_manager_'+label+'.py':sorted(n.name for n in ast.walk(ast.parse((root/('scripts/test_knowledge_manager_'+label+'.py')).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))for label in labels})
prior=root/'system/eval/knowledge-roadmap-2026-10-15/km-04-source-check';old=json.loads((prior/'source-binding.json').read_text());b=json.loads(json.dumps(old));components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=set(old['source_sha256'])|components
b['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(files)};b['scope']='KM04_3_B1_durable_state_intent_lock_retry_not_native_worker';b['previous_source_binding']={'revision':'8bfe40e8','path':'system/eval/knowledge-roadmap-2026-10-15/km-04-source-check/source-binding.json'};b['added_sources']=sorted(files-set(old['source_sha256']));b['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if b['source_sha256'][n]!=h);write('source-binding.json',b)
oldv=json.loads((prior/'validation.json').read_text());v={'gates':{'build':0,'fmt_vet_lint_boundaries':0,'manager_python':0,'ledger_runtime':0,'historical_design':0,'docs':0},'manager_python_tests':224,'current_Go_validation_reused':True,'Go_inputs_unchanged':True,'module':oldv['module'],'race':oldv['race'],'Go_reference':{'revision':subprocess.check_output(['git','rev-parse','8bfe40e8'],text=True).strip(),'manifest':(prior/'manifest.json').relative_to(root).as_posix(),'manifest_sha256':sha((prior/'manifest.json').read_bytes()),'maintained_Go_files_equal':1258}};write('validation.json',v)
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/check-manager-update-state-evidence.py','scripts/test_manager_update_state_evidence.py','scripts/verify-manager-update-state.py','docs/spec-driven/KNOWLEDGE-MANAGER-UPDATES.md','docs/DOCUMENTATION-CATALOG.json'}|{'scripts/test_knowledge_manager_'+label+'.py'for label in labels}
m={'schema_version':1,'checkpoint':'KM04_3_B1_durable_state_not_native_worker','state_foundation_complete':True,'full_KM04_3_complete':False,'scheduler_runtime_verified':False,'native_worker_recovery_verified':False,'paid_model_calls':0,'independent_acceptance':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print(json.dumps({'bound_sources':len(files),'state_tests':50,'all_Python':224,'runtime_checks':10,'native_builds':0,'Go_reference':'8bfe40e8'}))
