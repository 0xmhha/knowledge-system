import ast,gzip,hashlib,importlib.util,io,json,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-04-worker';base.mkdir(exist_ok=True);fixture=Path('/private/tmp/ks-KM043B2-worker-runtime-20261015')
def sha(data):return hashlib.sha256(data).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def put(n,data,compressed=False):
 path=base/(n+('.gz'if compressed else''));path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
for p in fixture.iterdir():
 if p.is_file():put('runtime/'+p.name,p.read_bytes(),p.name=='native-calls.json')
stream=io.BytesIO()
final=json.loads((fixture/'final-attempt.json').read_text())['request']['new_version']
def safe(info):
 if info.issym():
  if info.name!='fixture/dataset/current'or info.linkname!=final:raise ValueError('unexpected fixture symlink')
 elif not(info.isfile()or info.isdir()):raise ValueError('unsafe worker fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:
 for name in ('source','manager','dataset'):archive.add(fixture/name,arcname='fixture/'+name,filter=safe)
put('fixture.tar',stream.getvalue(),True)
spec=importlib.util.spec_from_file_location('roadmap',root/'scripts/check-knowledge-roadmap.py');roadmap=importlib.util.module_from_spec(spec);spec.loader.exec_module(roadmap)
v={'gates':{'build':0,'lint':0,'manager_python':0,'runtime':0,'historical_design':0,'roadmap':0,'plugin':0,'docs':0},'current_Go_validation_reused':False,'Go_count_flag':1,'manager_python_tests':247,'commands':{},'maintained_Go_files':1258}
for label in ('module','race'):
 data=Path('/private/tmp/ks-KM043B2-'+label+'-final.jsonl').read_bytes();counts,_=roadmap.test_inventory(data);assert counts['failed_tests']==counts['failed_packages']==0
 put(label+'.jsonl',data,True);v[label]=counts;write(label+'-tests.json',roadmap.test_rows(data))
 v['commands'][label]={'argv':['go','test',*(['-race']if label=='race'else[]),'-json','-count=1',*(['./...']if label=='module'else['./cmd/cks/mcpcli','./cmd/cks/setupcli','./internal/setup','./internal/system/config','./internal/system/maintenance','./internal/system/mcp'])],'exit_code':0}
for src,dst in [('ks-KM043B2-python-all-tests.txt','python-tests.txt'),('ks-KM043B2-build-lint.txt','build-lint.txt'),('ks-KM043B2-history.json','historical-design.json'),('ks-KM043B2-roadmap-tests.txt','roadmap-tests.txt'),('ks-KM043B2-plugin-validate.json','plugin-validate.json')]:put(dst,Path('/private/tmp',src).read_bytes())
for src,dst in [('ks-KM043B2-docs-final.txt','docs-final.txt'),('ks-KM043B2-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
put('initial-failures/worker-tests.txt',Path('/private/tmp/ks-KM043B2-worker-initial.txt').read_bytes())
write('initial-failures/scope.json',{'initial_cases':20,'initial_failures':2,'reason':'The generic integrity probe did not ask for the specific Value function. Specific original source assertions now use an explicit Value query and compare actual bodies/citations; they were not removed or replaced with metadata.','final_worker_cases':23,'full_worker_host_or_sustained_schedule_acceptance':False})
labels=('registration','collection','review','broker','update_contract','update_inputs','source_check','update_state','update_worker')
write('python-test-inventory.json',{'scripts/test_knowledge_manager_'+label+'.py':sorted(n.name for n in ast.walk(ast.parse((root/('scripts/test_knowledge_manager_'+label+'.py')).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))for label in labels})
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-15/km-04-state/source-binding.json').read_text());b=json.loads(json.dumps(old));components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=set(old['source_sha256'])|components
b['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(files)};b['scope']='KM04_3_B2_actual_foreground_native_worker_not_sustained_schedule_or_worker_host_kill';b['previous_source_binding']={'revision':'d00224a5','path':'system/eval/knowledge-roadmap-2026-10-15/km-04-state/source-binding.json'};b['added_sources']=sorted(files-set(old['source_sha256']));b['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if b['source_sha256'][n]!=h);write('source-binding.json',b);write('validation.json',v)
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/verify-manager-update-worker.py','scripts/check-manager-update-worker-evidence.py','scripts/test_manager_update_worker_evidence.py','docs/spec-driven/KNOWLEDGE-MANAGER-UPDATES.md','docs/DOCUMENTATION-CATALOG.json'}|{'scripts/test_knowledge_manager_'+label+'.py'for label in labels}
m={'schema_version':1,'checkpoint':'KM04_3_B2_foreground_native_worker','foreground_worker_verified':True,'full_KM04_3_complete':False,'full_KM04_4_complete':False,'sustained_schedule_verified':False,'actual_worker_host_SIGKILL_verified':False,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print(json.dumps({'bound_inputs':len(files),'Go_files':1258,'Python':247,'module_cases':v['module']['test_cases'],'race_cases':v['race']['test_cases'],'runtime_checks':14,'native_RPC':97,'jobs':6,'native_EOF0':16}))
