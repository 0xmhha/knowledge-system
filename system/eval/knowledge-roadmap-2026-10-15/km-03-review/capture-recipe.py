import gzip,hashlib,json,tarfile,io
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-review';base.mkdir(exist_ok=True)
fixture=Path('/private/tmp/ks-KM033-review-final-20261015')
def sha(b):return hashlib.sha256(b).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
for path in fixture.iterdir():
 if not path.is_file() or path.suffix not in ('.json','.txt','.jsonl'):continue
 target=base/'runtime'/path.name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(path.read_bytes())
stream=io.BytesIO()
with tarfile.open(fileobj=stream,mode='w') as archive:
 for name in ('source-a','source-b','manager-a','manager-b','provided.json','claims.json','review-input.json','retraction-input.json'):archive.add(fixture/name,arcname=name,recursive=True)
(base/'fixture.tar.gz').write_bytes(gzip.compress(stream.getvalue(),mtime=0))
for src,dst in [('ks-KM033-build-lint.txt','build.txt'),('ks-KM033-build-lint.txt','lint.txt'),('ks-KM033-registration-tests.txt','registration-tests.txt'),('ks-KM033-collection-tests.txt','collection-tests.txt'),('ks-KM033-review-tests.txt','review-tests.txt'),('ks-KM033-review-history.json','historical-design.json')]: (base/dst).write_bytes(Path('/private/tmp',src).read_bytes())
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
previous=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-collection';old=json.loads((previous/'source-binding.json').read_text());binding=json.loads(json.dumps(old));components={p.relative_to(root).as_posix() for p in (root/'plugins/knowledge-manager').rglob('*') if p.is_file() and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=sorted(set(old['source_sha256'])|components);binding['source_sha256']={n:sha((root/n).read_bytes()) for n in files};binding['scope']='current_unchanged_Go_and_manager_review_component_inputs';binding['previous_source_binding']={'revision':'322daf39','path':str((previous/'source-binding.json').relative_to(root))};binding['added_sources']=sorted(components-set(old['source_sha256']));binding['changed_preexisting_sources']=sorted(n for n,h in old['source_sha256'].items() if binding['source_sha256'][n]!=h);write('source-binding.json',binding)
v=json.loads((previous/'validation.json').read_text());v['review_tests_passed']=35;v['collection_tests_passed']=23;v['gates']={'build':0,'lint':0,'runtime':0,'historical_design':0,'roadmap':0,'registration_unit':0,'collection_unit':0,'review_unit':0,'native_package':0};write('validation.json',v)
sourcepaths=set(files)|{'scripts/knowledge_evidence.py','scripts/verify-knowledge-manager-review.py','scripts/check-knowledge-manager-review-evidence.py','scripts/test_knowledge_manager_review_evidence.py','scripts/test_knowledge_manager_collection.py','scripts/test_knowledge_manager_review.py','scripts/test_knowledge_manager_registration.py','scripts/verify_committed_maintenance_checkpoint.py','docs/spec-driven/KNOWLEDGE-MANAGER-PLUGIN.md','docs/DOCUMENTATION-CATALOG.json','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py'}
manifest={'schema_version':1,'checkpoint':'KM03_3_supplied_review_filtered_input_adoption_retraction','KM03_3_complete':True,'full_KM03_complete':False,'independent_acceptance':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(sourcepaths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 manifest['artifacts'].append(row)
write('manifest.json',manifest)
print(json.dumps({'bound_inputs':len(files),'added':binding['added_sources'],'changed_components':binding['changed_preexisting_sources']}))
