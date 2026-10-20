import ast,gzip,hashlib,importlib.util,io,json,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-04-source-check';base.mkdir(exist_ok=True);fixture=Path('/private/tmp/ks-KM043A-source-check-final2-20261015')
def sha(data):return hashlib.sha256(data).hexdigest()
def write(name,value):(base/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
def put(name,data,compressed=False):
 p=base/(name+('.gz'if compressed else''));p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
for p in fixture.iterdir():
 if p.is_file():put('runtime/'+p.name,p.read_bytes())
 elif p.is_dir()and p.name.endswith('-session'):
  for child in p.iterdir():
   if child.is_file():put('runtime/'+p.name+'/'+child.name,child.read_bytes(),child.suffix=='.jsonl')
stream=io.BytesIO()
def safe(info):
 if info.issym():
  if info.name!='fixture/dataset/current'or info.linkname!='guarded-v3':raise ValueError('unexpected fixture symlink')
 elif not(info.isfile()or info.isdir()):raise ValueError('unsafe owned source-check fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:
 for name in ('source','manager','dataset'):archive.add(fixture/name,arcname='fixture/'+name,filter=safe)
put('fixture.tar',stream.getvalue(),True)
spec=importlib.util.spec_from_file_location('roadmap',root/'scripts/check-knowledge-roadmap.py');roadmap=importlib.util.module_from_spec(spec);spec.loader.exec_module(roadmap)
v={'gates':{'build':0,'lint':0,'manager_python':0,'runtime':0,'guardian_stress':0,'historical_design':0,'roadmap':0},'current_Go_validation_reused':False,'Go_count_flag':1,'manager_python_tests':174,'commands':{}}
for label in ('module','race'):
 data=Path('/private/tmp/ks-KM043A-'+label+'-final.jsonl').read_bytes();counts,_=roadmap.test_inventory(data);assert counts['failed_tests']==counts['failed_packages']==0
 put(label+'.jsonl',data,True);v[label]=counts;write(label+'-tests.json',roadmap.test_rows(data))
 v['commands'][label]={'argv':['go','test',*(['-race']if label=='race'else[]),'-json','-count=1',*(['./...']if label=='module'else['./cmd/cks/mcpcli','./cmd/cks/setupcli','./internal/setup','./internal/system/config','./internal/system/maintenance','./internal/system/mcp'])],'exit_code':0}
for src,dst in [('ks-KM043A-python-all-tests.txt','python-tests.txt'),('ks-KM043A-build-lint-final.txt','build-lint.txt'),('ks-KM043A-history.json','historical-design.json'),('ks-KM043A-evidence-tests.txt','evidence-tests.txt'),('ks-KM043A-docs-final.txt','docs-final.txt')]:put(dst,Path('/private/tmp',src).read_bytes())
put('guardian-strengthened.jsonl',Path('/private/tmp/ks-KM043A-guardian-strengthened.jsonl').read_bytes(),True)
write('python-test-inventory.json',{'scripts/test_knowledge_manager_'+label+'.py':sorted(n.name for n in ast.walk(ast.parse((root/('scripts/test_knowledge_manager_'+label+'.py')).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))for label in ('registration','collection','review','broker','update_contract','update_inputs','source_check')})
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-15/km-04-inputs/source-binding.json').read_text());b=json.loads(json.dumps(old));maintained={p.relative_to(root).as_posix()for d in ('cmd','internal','pkg')for p in(root/d).rglob('*.go')if p.is_file()};components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=set(old['source_sha256'])|maintained|components
b['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(files)};b['scope']='native_original_source_publication_check_and_absolute_guardian_exec_deadline_boundary';b['previous_source_binding']={'revision':'2c03322f','path':'system/eval/knowledge-roadmap-2026-10-15/km-04-inputs/source-binding.json'};b['added_sources']=sorted(files-set(old['source_sha256']));b['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if b['source_sha256'][n]!=h);write('source-binding.json',b);v['maintained_Go_files']=len(maintained);write('validation.json',v)
for label in ('module','race'):
 put('initial-failures/'+label+'.jsonl',Path('/private/tmp/ks-KM043A-'+label+'.jsonl').read_bytes(),True)
put('initial-failures/guardian-pre-change-repro.jsonl',Path('/private/tmp/ks-KM043A-guardian-repro.jsonl').read_bytes(),True)
for label in ('first','second'):put('initial-failures/runtime-'+label+'.txt',Path('/private/tmp/ks-KM043A-runtime-'+label+'.txt').read_bytes())
write('initial-failures/scope.json',{'native_probe':'The initial original-source probe checked dataset ownership during native admission after the owned journal existed but before root ownership was claimed. Read-only source verification now leaves DB ownership to native maintenance; normal registration/doctor/preparation/mutations keep the original check.','native_error_parser':'Actual native errors have human-readable content plus typed structuredContent. Corrected verifier to use the actual typed error.','race_observation':'One initial race execution of the existing absolute deadline test observed a late writer. Five repetitions before the code change did not reproduce it; no unique root cause is claimed.','boundary_strengthening':'Check actual wall clock immediately before/after exec, independently of timer callback delivery; add a past-deadline no-exec test. Both boundary cases passed five race repetitions and the final full module and six-package race run passed sequentially.','initial_module_is_not_final_validation':True,'paid_model_calls':0,'independent_acceptance':False})
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/verify-manager-source-check.py','scripts/check-manager-source-check-evidence.py','scripts/test_manager_source_check_evidence.py','scripts/test_knowledge_manager_source_check.py','scripts/test_knowledge_manager_registration.py','scripts/test_knowledge_manager_collection.py','scripts/test_knowledge_manager_review.py','scripts/test_knowledge_manager_broker.py','scripts/test_knowledge_manager_update_contract.py','scripts/test_knowledge_manager_update_inputs.py','scripts/eval-value-dependencies.py','docs/spec-driven/KNOWLEDGE-MANAGER-UPDATES.md','docs/DOCUMENTATION-CATALOG.json'}
m={'schema_version':1,'checkpoint':'KM04_3_A_native_original_source_publication_check','source_check_checkpoint_complete':True,'full_KM04_3_complete':False,'full_KM04_complete':False,'scheduler_runtime_verified':False,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print(json.dumps({'bound_inputs':len(files),'Go_files':len(maintained),'module':{k:v for k,v in v['module'].items()if k!='skipped'},'race':{k:v for k,v in v['race'].items()if k!='skipped'},'native':14,'Python':174}))
