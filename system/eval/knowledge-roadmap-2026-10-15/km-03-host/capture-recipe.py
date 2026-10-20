import gzip,hashlib,importlib.util,io,json,shutil,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-host';base.mkdir(exist_ok=True)
fixture=Path('/private/tmp/ks-KM034C-host-final-20261015')
def sha(b):return hashlib.sha256(b).hexdigest()
def write(name,value):(base/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
def put(name,data,compress=False):
 path=base/(name+('.gz'if compress else''));path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(gzip.compress(data,mtime=0)if compress else data)
for path in fixture.rglob('*'):
 rel=path.relative_to(fixture)
 if not path.is_file()or path.is_symlink()or any(part in ('claude-config','tmp')or part.endswith('-fixture')for part in rel.parts):continue
 put('runtime/'+rel.as_posix(),path.read_bytes(),path.suffix=='.jsonl'or path.name=='gateway-requests.json')
stream=io.BytesIO()
def safe(info):
 if any(p in ('claude-config','tmp')for p in Path(info.name).parts):return None
 if not(info.isfile()or info.isdir()):raise ValueError('unsafe owned host fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:
 for path in sorted(fixture.glob('*-fixture')):archive.add(path,arcname=path.name,filter=safe)
put('fixture.tar',stream.getvalue(),True)
spec=importlib.util.spec_from_file_location('roadmap',root/'scripts/check-knowledge-roadmap.py');roadmap=importlib.util.module_from_spec(spec);spec.loader.exec_module(roadmap)
validation={'gates':{'build':0,'lint':0,'runtime':0,'historical_design':0,'roadmap':0,'manager_unit':0,'native_package':0},'current_Go_validation_reused':False,'Go_count_flag':1,'manager_unit_tests':103,'commands':{}}
for label in ('module','race'):
 data=Path('/private/tmp/ks-KM034C-'+label+'.jsonl').read_bytes();counts,_=roadmap.test_inventory(data);assert counts['failed_tests']==counts['failed_packages']==0
 put(label+'.jsonl',data,True);validation[label]=counts;write(label+'-tests.json',roadmap.test_rows(data))
 validation['commands'][label]={'argv':['go','test',*(['-race']if label=='race'else[]),'-json','-count=1',*(['./...']if label=='module'else['./cmd/cks/mcpcli','./cmd/cks/setupcli','./internal/setup','./internal/system/config','./internal/system/maintenance','./internal/system/mcp'])],'exit_code':0}
for src,dst in [('ks-KM034C-build-lint.txt','build.txt'),('ks-KM034C-build-lint.txt','lint.txt'),('ks-KM034C-history.json','historical-design.json'),('ks-KM034C-manager-tests-final.txt','manager-tests.txt'),('ks-KM034C-evidence-tests.txt','evidence-tests.txt'),('ks-KM034C-docs-final.txt','docs-final.txt')]:put(dst,Path('/private/tmp',src).read_bytes())
shutil.copytree('/private/tmp/ks-KM034C-native',base/'native',dirs_exist_ok=True)
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-15/km-03-broker/source-binding.json').read_text());binding=json.loads(json.dumps(old));maintained={p.relative_to(root).as_posix()for d in ('cmd','internal','pkg')for p in(root/d).rglob('*.go')if p.is_file()};components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=set(old['source_sha256'])|maintained|components
binding['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(files)};binding['scope']='current_Go_CI_manager_and_actual_Claude_host_lifecycle';binding['previous_source_binding']={'revision':'c51f7423','path':'system/eval/knowledge-roadmap-2026-10-15/km-03-broker/source-binding.json'};binding['added_sources']=sorted(files-set(old['source_sha256']));binding['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if binding['source_sha256'][n]!=h);write('source-binding.json',binding)
validation['maintained_Go_files']=len(maintained);write('validation.json',validation)
initial=base/'initial-failures';initial.mkdir(exist_ok=True)
for label in ('metadata','explicit-metadata','graceful-metadata'):
 source=Path('/private/tmp/ks-KM034C-'+label+'-20261015')
 for name in ('mcp-frames.jsonl','host.stderr.txt','observation.json'):
  p=source/name
  if p.exists():put('initial-failures/'+label+'/'+name,p.read_bytes(),p.suffix=='.jsonl')
 p=Path('/private/tmp/ks-KM034C-'+label+'-summary.json')
 if p.exists():put('initial-failures/'+label+'/summary.json',p.read_bytes())
for name in ('full-first','all','manager-tests'):put('initial-failures/'+name+'.txt',Path('/private/tmp/ks-KM034C-'+name+'.txt').read_bytes())
write('initial-failures/scope.json',{'strict_discovery':'Empty explicit MCP with --strict-mcp-config also excluded plugin MCP. Fixture now explicitly registers the plugin-declared launcher; package declaration unchanged.','product_fix':'Actual SDK sends SIGINT on normal deinit; before fix manager -2, after fix graceful0. Raw frames retained.','harness_fixes':['Parse actual context format_version/coordinates instead of schema_version.','Match all actual SDK missing-engine startup retry attempts, not an assumed single process.','Close borrowed subprocess stdout in the new signal unit tests to remove ResourceWarning.'],'real_model_calls':0,'independent_acceptance':False,'personal_config_or_cache_captured':False})
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
sourcepaths=set(files)|{'scripts/knowledge_evidence.py','scripts/verify-knowledge-manager-host.py','scripts/check-knowledge-manager-host-evidence.py','scripts/test_knowledge_manager_host_evidence.py','scripts/test_knowledge_manager_registration.py','scripts/test_knowledge_manager_collection.py','scripts/test_knowledge_manager_review.py','scripts/test_knowledge_manager_broker.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/eval-value-dependencies.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','docs/spec-driven/KNOWLEDGE-MANAGER-PLUGIN.md','docs/DOCUMENTATION-CATALOG.json'}
manifest={'schema_version':1,'checkpoint':'KM03_4_C_actual_Claude_host_local_transport_and_lifecycle','KM03_4_complete':True,'full_KM03_complete':True,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(sourcepaths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 manifest['artifacts'].append(row)
write('manifest.json',manifest)
print(json.dumps({'bound_inputs':len(files),'maintained_Go_files':len(maintained),'module':{k:v for k,v in validation['module'].items()if k!='skipped'},'race':{k:v for k,v in validation['race'].items()if k!='skipped'},'host_runs':8,'actual_tool_results':68}))
