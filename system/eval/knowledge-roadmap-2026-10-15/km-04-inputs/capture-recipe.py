import ast,gzip,hashlib,io,json,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-04-inputs';base.mkdir(exist_ok=True);previous=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-host';fixture=Path('/private/tmp/ks-KM042-inputs-final-20261015')
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
 if not(info.isfile()or info.isdir()):raise ValueError('unsafe owned input fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:
 for label in ('source-a','source-b','manager-a','manager-b','dataset-a','dataset-b'):archive.add(fixture/label,arcname='fixture/'+label,filter=safe)
put('fixture.tar',stream.getvalue(),True)
for src,dst in [('ks-KM042-input-tests.txt','input-tests.txt'),('ks-KM042-contract-tests.txt','contract-tests.txt'),('ks-KM042-build-lint-manager.txt','build-lint-manager.txt'),('ks-KM042-history.json','historical-design.json'),('ks-KM042-evidence-tests.txt','evidence-tests.txt'),('ks-KM042-docs-final.txt','docs-final.txt')]:put(dst,Path('/private/tmp',src).read_bytes())
for label,file in [('input','scripts/test_knowledge_manager_update_inputs.py'),('contract','scripts/test_knowledge_manager_update_contract.py')]:write(label+'-test-inventory.json',sorted(n.name for n in ast.walk(ast.parse((root/file).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_')))
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-15/km-04-contract/source-binding.json').read_text());binding=json.loads(json.dumps(old));components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=set(old['source_sha256'])|components
binding['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(files)};binding['scope']='observed_manager_inputs_change_delete_and_native_noop_pins_not_scheduler';binding['previous_source_binding']={'revision':'7268ac7f','path':'system/eval/knowledge-roadmap-2026-10-15/km-04-contract/source-binding.json'};binding['added_sources']=sorted(files-set(old['source_sha256']));binding['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if binding['source_sha256'][n]!=h);write('source-binding.json',binding)
v=json.loads((previous/'validation.json').read_text());v['referenced_Go_commands']=v.pop('commands');v.pop('manager_unit_tests');v.update(current_Go_validation_reused=True,Go_inputs_unchanged=True,input_unit_tests=32,policy_unit_tests=27,manager_unit_tests=103);v['Go_reference']={'revision':'cea1c943','manifest':(previous/'manifest.json').relative_to(root).as_posix(),'manifest_sha256':sha((previous/'manifest.json').read_bytes()),'maintained_Go_files_equal':1256,'scope':'unchanged_C_Go_dependencies_and_fixture_bytes_not_fresh_run'};v['gates']={'build':0,'lint':0,'input_unit':0,'policy_unit':0,'manager_unit':0,'runtime':0,'historical_design':0,'roadmap':0};write('validation.json',v)
put('initial-failures/wrong-stale-error-name.txt',Path('/private/tmp/ks-KM042-input-tests-initial.txt').read_bytes());put('initial-failures/concurrent-build.txt',Path('/private/tmp/ks-KM042-concurrent-build-input-tests.txt').read_bytes());write('initial-failures/scope.json',{'first':'Expected the actual collection_stale refusal under an incorrect test error name; corrected the assertion only.','concurrent_build':'make build and unit tests were initially run concurrently. Strict registration correctly refused two binary_changed operations as workspace executables were rebuilt with new VCS identity. Final unit tests run after build completion, not by disabling identity checks.','product_guards_relaxed':False,'original_approval_or_completion_states_changed':False})
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/check-manager-update-inputs-evidence.py','scripts/test_manager_update_inputs_evidence.py','scripts/verify-manager-update-inputs.py','scripts/test_knowledge_manager_update_inputs.py','scripts/test_knowledge_manager_update_contract.py','scripts/eval-value-dependencies.py','docs/spec-driven/KNOWLEDGE-MANAGER-UPDATES.md','docs/DOCUMENTATION-CATALOG.json'}
m={'schema_version':1,'checkpoint':'KM04_2_observed_input_change_delete_and_noop_pins','KM04_2_complete':True,'full_KM04_complete':False,'scheduler_runtime_verified':False,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print('input packet:',len(files),'bound inputs; unit162; actual16/4DB/82calls/11EOF0; Go C reference')
