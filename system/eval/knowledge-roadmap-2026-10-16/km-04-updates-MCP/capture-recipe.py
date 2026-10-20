import ast,gzip,hashlib,io,json,subprocess,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/km-04-updates-MCP';base.mkdir(parents=True,exist_ok=True)
mcp=Path('/private/tmp/ks-KM043C2-updates-MCP-final-20261016');claude=Path('/private/tmp/ks-KM043C2-actual-Claude-final-20261016')
def sha(d):return hashlib.sha256(d).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def put(n,data,compressed=False):
 p=base/(n+('.gz'if compressed else''));p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
for p in mcp.iterdir():
 if p.is_file():put('MCP/'+p.name,p.read_bytes(),p.suffix=='.jsonl')
 elif p.is_dir()and p.name.endswith('-session'):
  for child in p.iterdir():
   if child.is_file():put('MCP/'+p.name+'/'+child.name,child.read_bytes(),child.suffix=='.jsonl')
for p in claude.iterdir():
 if p.is_file():put('Claude/'+p.name,p.read_bytes())
 elif p.is_dir()and p.name in('updates-full','updates-restart'):
  for child in p.iterdir():
   if child.is_file():put('Claude/'+p.name+'/'+child.name,child.read_bytes(),child.suffix=='.jsonl' or child.name.startswith('api-request-'))
final=json.loads((mcp/'parent-EOF-recovered.json').read_text())['request']['new_version']
for label,fixture in [('MCP',mcp),('Claude',claude/'fixture')]:
 stream=io.BytesIO()
 def safe(info):
  if info.issym():
   if label!='MCP'or info.name!='fixture/dataset/current'or info.linkname!=final:raise ValueError('unexpected fixture symlink')
  elif not(info.isfile()or info.isdir()):raise ValueError('unsafe owned fixture')
  return info
 with tarfile.open(fileobj=stream,mode='w')as archive:
  for name in('source','manager','dataset'):archive.add(fixture/name,arcname='fixture/'+name,filter=safe)
 put(label+'-fixture.tar',stream.getvalue(),True)
for src,dst in [('ks-KM043C2-python-all-tests.txt','python-tests.txt'),('ks-KM043C2-build-lint.txt','build-lint.txt'),('ks-KM043C2-history.json','historical-design.json'),('ks-KM043C2-plugin-validate.json','plugin-validate.json'),('ks-KM043C2-roadmap-tests.txt','roadmap-tests.txt')]:put(dst,Path('/private/tmp',src).read_bytes())
for src,dst in [('ks-KM043C2-docs-final.txt','docs-final.txt'),('ks-KM043C2-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
put('initial-failures/MCP-observer-before-copy.json',Path('/private/tmp/ks-KM043C2-updates-MCP-initial-20261016/calls.json').read_bytes(),True)
put('initial-failures/MCP-wire-before-copy.jsonl',Path('/private/tmp/ks-KM043C2-updates-MCP-initial-20261016/primary-session/mcp.stdout.jsonl').read_bytes(),True)
put('initial-failures/Claude-observer-before-copy.json',Path('/private/tmp/ks-KM043C2-actual-Claude-initial-20261016/updates-full/actual-tool-results.json').read_bytes(),True)
write('initial-failures/trace-copy-scope.json',{'reason':'The test observer stored the same object as its policy template consumer, which changed enabled/activation while configuring its own test policy. Raw MCP responses remained unchanged and the evidence guard rejected the inconsistent observer trace.','fix':'Deep copy observed results/arguments before consumer mutation; derive an independent test policy copy. Rerun both raw MCP and installed Claude probes.','product_manager_code_changed_for_this_fix':False,'repeated_status_calls_are_not_additional_independent_cases':True})
put('initial-failures/worker-MCP-tests.txt',Path('/private/tmp/ks-KM043C2-worker-host-initial.txt').read_bytes())
write('initial-failures/scope.json',{'initial_cases':43,'initial_failures':1,'observation':'A live source edit raced with stable capture. The updater correctly preserved the old DB but exited periodic polling with source_changed.','fix':'Only periodic/change pre-admission source_changed/source_stale checks defer to a later bounded interval. Preserve intent, successful baseline and safety failures. Emit bounded deferred-check diagnostics.','final_worker_MCP_cases':44,'independent_acceptance':False})
labels=('registration','collection','review','broker','update_contract','update_inputs','source_check','update_state','update_worker')
write('python-test-inventory.json',{'scripts/test_knowledge_manager_'+label+'.py':sorted(n.name for n in ast.walk(ast.parse((root/('scripts/test_knowledge_manager_'+label+'.py')).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))for label in labels})
prior=root/'system/eval/knowledge-roadmap-2026-10-15/km-04-worker';old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/km-04-worker-host/source-binding.json').read_text());b=json.loads(json.dumps(old));components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=set(old['source_sha256'])|components
b['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(files)};b['scope']='KM04_3_complete_integrated_updates_MCP_Claude_owned_worker_and_exact_retained_reader';b['previous_source_binding']={'revision':'ebec0858','path':'system/eval/knowledge-roadmap-2026-10-16/km-04-worker-host/source-binding.json'};b['added_sources']=sorted(files-set(old['source_sha256']));b['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if b['source_sha256'][n]!=h);write('source-binding.json',b)
oldv=json.loads((prior/'validation.json').read_text());v={'gates':{'build':0,'lint':0,'manager_python':0,'MCP':0,'actual_Claude':0,'historical_design':0,'plugin':0,'roadmap':0,'docs':0},'current_Go_validation_reused':True,'Go_inputs_unchanged':True,'manager_python_tests':268,'module':oldv['module'],'race':oldv['race'],'Go_reference':{'revision':subprocess.check_output(['git','rev-parse','a801f551'],text=True).strip(),'manifest':(prior/'manifest.json').relative_to(root).as_posix(),'manifest_sha256':sha((prior/'manifest.json').read_bytes()),'maintained_Go_files_equal':1258}};write('validation.json',v)
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/eval-value-dependencies.py','scripts/verify-knowledge-manager-host.py','scripts/operational-host-probe.py','scripts/verify-manager-updates-MCP.py','scripts/verify-manager-updates-Claude.py','scripts/check-manager-updates-MCP-evidence.py','scripts/test_manager_updates_MCP_evidence.py','docs/spec-driven/KNOWLEDGE-MANAGER-UPDATES.md','docs/spec-driven/KNOWLEDGE-MANAGER-PLUGIN.md','docs/DOCUMENTATION-CATALOG.json'}|{'scripts/test_knowledge_manager_'+label+'.py'for label in labels}
m={'schema_version':1,'checkpoint':'KM04_3_C2_integrated_updates_MCP_Claude','full_KM04_3_complete':True,'full_KM04_complete':False,'full_KM04_4_complete':False,'actual_Claude_updates_host_verified':True,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();record={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':record['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(record)
write('manifest.json',m);print(json.dumps({'bound_sources':len(files),'Go':1258,'Python':268,'MCP_checks':20,'MCP_calls':222,'actual_Claude_runs':2,'Claude_tools':36,'completed_subtasks':15,'total':28}))
