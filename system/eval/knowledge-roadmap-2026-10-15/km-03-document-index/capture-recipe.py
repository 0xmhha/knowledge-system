import gzip,hashlib,importlib.util,io,json,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-document-index';base.mkdir(exist_ok=True)
fixture=Path('/private/tmp/ks-KM034A-documents-final-20261015')
def sha(b):return hashlib.sha256(b).hexdigest()
def write(name,value):(base/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
def compress(name,data):(base/(name+'.gz')).write_bytes(gzip.compress(data,mtime=0))
for path in fixture.rglob('*'):
 if not path.is_file()or path.is_symlink()or 'prepared'in path.relative_to(fixture).parts or len(path.relative_to(fixture).parts)>=3:continue
 if path.suffix not in ('.json','.jsonl','.txt','.yaml'):continue
 dest=base/'runtime'/path.relative_to(fixture);dest.parent.mkdir(parents=True,exist_ok=True)
 if path.suffix=='.jsonl':Path(str(dest)+'.gz').write_bytes(gzip.compress(path.read_bytes(),mtime=0))
 else:dest.write_bytes(path.read_bytes())
stream=io.BytesIO()
def safe_fixture(info):
 if 'claude-config'in Path(info.name).parts:return None
 if not(info.isfile()or info.isdir()):raise ValueError('nonregular owned fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:archive.add(fixture/'prepared',arcname='prepared',recursive=True,filter=safe_fixture)
compress('fixture.tar',stream.getvalue())
spec=importlib.util.spec_from_file_location('roadmap',root/'scripts/check-knowledge-roadmap.py');guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)
validation={'gates':{'build':0,'lint':0,'runtime':0,'historical_design':0,'roadmap':0},'current_Go_validation_reused':False,'Go_count_flag':1,'commands':{}}
for label in ('module','race'):
 data=Path('/private/tmp/ks-KM034A-'+label+'-final.jsonl').read_bytes();counts,_=guard.test_inventory(data);assert counts['failed_tests']==counts['failed_packages']==0
 compress(label+'.jsonl',data);validation[label]=counts;write(label+'-tests.json',guard.test_rows(data))
 validation['commands'][label]={'argv':['go','test',*(['-race']if label=='race'else[]),'-json','-count=1',*(['./...']if label=='module'else['./cmd/cks/mcpcli','./cmd/cks/setupcli','./internal/setup','./internal/system/config','./internal/system/maintenance','./internal/system/mcp'])],'exit_code':0}
for src,dst in [('ks-KM034A-build-lint.txt','build.txt'),('ks-KM034A-build-lint.txt','lint.txt'),('ks-KM034A-history.json','historical-design.json')]: (base/dst).write_bytes(Path('/private/tmp',src).read_bytes())
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-15/km-03-review/source-binding.json').read_text());binding=json.loads(json.dumps(old));maintained={p.relative_to(root).as_posix()for d in ('cmd','internal','pkg')for p in(root/d).rglob('*.go')if p.is_file()};components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=set(old['source_sha256'])|maintained|components
binding['source_sha256']={n:sha((root/n).read_bytes())for n in sorted(files)};binding['scope']='current_Go_and_manager_inputs_after_explicit_document_corpora';binding['previous_source_binding']={'revision':'3d39801c','path':'system/eval/knowledge-roadmap-2026-10-15/km-03-review/source-binding.json'};binding['added_sources']=sorted(files-set(old['source_sha256']));binding['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if binding['source_sha256'][n]!=h);write('source-binding.json',binding)
validation['maintained_Go_files']=len(maintained);write('validation.json',validation)
initial=base/'initial-failures';initial.mkdir(exist_ok=True)
(initial/'compile.txt').write_bytes(Path('/private/tmp/ks-KM034A-first-compile.txt').read_bytes())
(initial/'runtime-assertion.txt').write_bytes(Path('/private/tmp/ks-KM034A-documents-runtime.txt').read_bytes())
for label in ('module','race'):
 data=Path('/private/tmp/ks-KM034A-'+label+'.jsonl').read_bytes();(initial/(label+'.jsonl.gz')).write_bytes(gzip.compress(data,mtime=0))
(initial/'scope.json').write_text(json.dumps({'compile':'Removed an unused import before tests.','Go_fixture':'Canonicalized the macOS temporary source root; original input_stale failures retained.','runtime_assertion':'Candidate was already correctly held; fixed assertion to check required gates while preserving semantic:not_configured.','production_dataset_touched':False},indent=2)+'\n')
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
sourcepaths=set(files)|{'scripts/knowledge_evidence.py','scripts/verify-manager-document-index.py','scripts/check-manager-document-index-evidence.py','scripts/test_manager_document_index_evidence.py','scripts/verify-knowledge-manager-review.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/eval-value-dependencies.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','docs/spec-driven/KNOWLEDGE-MANAGER-PLUGIN.md','docs/DOCUMENTATION-CATALOG.json'}
manifest={'schema_version':1,'checkpoint':'KM03_4_A_reviewed_document_index_and_retained_reads','document_index_checkpoint_complete':True,'KM03_4_complete':False,'full_KM03_complete':False,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(sourcepaths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 manifest['artifacts'].append(row)
write('manifest.json',manifest)
print(json.dumps({'bound_inputs':len(files),'maintained_Go_files':len(maintained),'module':validation['module'],'race':validation['race'],'runtime':json.loads((fixture/'summary.json').read_text())}))
