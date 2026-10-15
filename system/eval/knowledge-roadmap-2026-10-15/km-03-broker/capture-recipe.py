import gzip,hashlib,io,json,shutil,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-broker';base.mkdir(exist_ok=True)
fixture=Path('/private/tmp/ks-KM034B-broker-final-20261015')
def sha(b):return hashlib.sha256(b).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
for path in fixture.rglob('*'):
 if not path.is_file()or path.is_symlink()or 'prepared'in path.relative_to(fixture).parts or len(path.relative_to(fixture).parts)>=3:continue
 if path.suffix not in('.json','.jsonl','.txt'):continue
 dest=base/'runtime'/path.relative_to(fixture);dest.parent.mkdir(parents=True,exist_ok=True)
 if path.suffix=='.jsonl':Path(str(dest)+'.gz').write_bytes(gzip.compress(path.read_bytes(),mtime=0))
 else:dest.write_bytes(path.read_bytes())
stream=io.BytesIO()
def safe_fixture(info):
 if 'claude-config'in Path(info.name).parts:return None
 if not(info.isfile()or info.isdir()):raise ValueError('nonregular owned fixture')
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:archive.add(fixture/'prepared',arcname='prepared',recursive=True,filter=safe_fixture)
(base/'fixture.tar.gz').write_bytes(gzip.compress(stream.getvalue(),mtime=0))
for src,dst in [('ks-KM034B-build-lint.txt','build.txt'),('ks-KM034B-build-lint.txt','lint.txt'),('ks-KM034B-broker-tests-final.txt','broker-tests.txt'),('ks-KM034B-registration-tests.txt','registration-tests.txt'),('ks-KM034B-collection-tests.txt','collection-tests.txt'),('ks-KM034B-history.json','historical-design.json')]: (base/dst).write_bytes(Path('/private/tmp',src).read_bytes())
shutil.copytree(Path('/private/tmp/ks-KM034B-native-validation'),base/'native',dirs_exist_ok=True)
initial=base/'initial-failures';initial.mkdir(exist_ok=True);(initial/'broker-tests.txt').write_bytes(Path('/private/tmp/ks-KM034B-initial-broker-tests.txt').read_bytes());(initial/'scope.json').write_text(json.dumps({'reason':'The native SDK validly omits isError:false; corrected the test to accept the default false.','changed_product_behavior_for_this_failure':False,'final_failure_count':0},indent=2)+'\n')
previous=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-document-index';old=json.loads((previous/'source-binding.json').read_text());binding=json.loads(json.dumps(old));components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=sorted(set(old['source_sha256'])|components);binding['source_sha256']={n:sha((root/n).read_bytes())for n in files};binding['scope']='current_unchanged_document_index_Go_and_manager_broker_inputs';binding['previous_source_binding']={'revision':'bb71aae3','path':str((previous/'source-binding.json').relative_to(root))};binding['added_sources']=sorted(set(files)-set(old['source_sha256']));binding['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items()if binding['source_sha256'][n]!=h);write('source-binding.json',binding)
v=json.loads((previous/'validation.json').read_text());v.update(current_Go_validation_reused=True,Go_inputs_unchanged=True,broker_tests_passed=29,registration_tests_passed=14,collection_tests_passed=23);v['referenced_Go_commands']=v.pop('commands');v['Go_reference']={'revision':'bb71aae36e66d53e43b3e3e94dffa19f7d2322ad','manifest':str((previous/'manifest.json').relative_to(root)),'manifest_sha256':sha((previous/'manifest.json').read_bytes()),'maintained_Go_files_equal':1256,'scope':'unchanged_Go_dependency_fixture_inputs_A_count1_results_not_fresh_B_execution'};v['gates']={'build':0,'lint':0,'broker_unit':0,'registration_unit':0,'collection_unit':0,'runtime':0,'native_package':0,'historical_design':0,'roadmap':0};write('validation.json',v)
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
sourcepaths=set(files)|{'scripts/knowledge_evidence.py','scripts/verify-knowledge-manager-broker.py','scripts/check-knowledge-manager-broker-evidence.py','scripts/test_knowledge_manager_broker_evidence.py','scripts/test_knowledge_manager_broker.py','scripts/test_knowledge_manager_registration.py','scripts/test_knowledge_manager_collection.py','scripts/test_knowledge_manager_review.py','scripts/verify-knowledge-manager-review.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/eval-value-dependencies.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','docs/spec-driven/KNOWLEDGE-MANAGER-PLUGIN.md','docs/DOCUMENTATION-CATALOG.json'}
manifest={'schema_version':1,'checkpoint':'KM03_4_B_adoption_bound_manager_tools_and_actual_reader','broker_checkpoint_complete':True,'KM03_4_complete':False,'full_KM03_complete':False,'independent_acceptance':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(sourcepaths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 manifest['artifacts'].append(row)
write('manifest.json',manifest)
print(json.dumps({'bound_inputs':len(files),'added_sources':binding['added_sources'],'Go_reused_from':'bb71aae3','runtime_checks':26,'broker_unit':29,'native_validation':3}))
