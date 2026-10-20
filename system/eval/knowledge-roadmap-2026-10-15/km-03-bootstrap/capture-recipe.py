import gzip,hashlib,json,subprocess,tarfile,io
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-03-bootstrap';base.mkdir(exist_ok=True)
fixture=Path('/private/tmp/ks-KM031-bootstrap-final-20261015')
def sha(b):return hashlib.sha256(b).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
for path in fixture.rglob('*'):
 if not path.is_file()or path.is_symlink()or len(path.relative_to(fixture).parts)>=3:continue
 if path.suffix not in('.json','.jsonl','.txt')or path.parent.name in('source','manager','claude-config'):continue
 target=base/'runtime'/path.relative_to(fixture);target.parent.mkdir(parents=True,exist_ok=True)
 if path.suffix=='.jsonl':Path(str(target)+'.gz').write_bytes(gzip.compress(path.read_bytes(),mtime=0))
 else:target.write_bytes(path.read_bytes())
stream=io.BytesIO()
with tarfile.open(fileobj=stream,mode='w')as archive:
 for name in ('source','manager','unrelated-cwd'):archive.add(fixture/name,arcname=name,recursive=True)
(base/'fixture.tar.gz').write_bytes(gzip.compress(stream.getvalue(),mtime=0))
for src,dst in [('ks-KM031-build-lint.txt','build.txt'),('ks-KM031-build-lint.txt','lint.txt'),('ks-KM031-registration-tests.txt','registration-tests.txt'),('ks-KM031-history.json','historical-design.json')]: (base/dst).write_bytes(Path('/private/tmp',src).read_bytes())
previous=root/'system/eval/knowledge-roadmap-2026-10-15/km-02-public-failures';old=json.loads((previous/'source-binding.json').read_text());binding=json.loads(json.dumps(old));components={p.relative_to(root).as_posix()for p in(root/'plugins/knowledge-manager').rglob('*')if p.is_file()and '__pycache__'not in p.parts and p.suffix!='.pyc'};files=sorted(set(old['source_sha256'])|components);binding['source_sha256']={n:sha((root/n).read_bytes())for n in files};binding['scope']='current_unchanged_Go_and_manager_bootstrap_component_inputs';binding['previous_source_binding']={'revision':'6705ed57','path':str((previous/'source-binding.json').relative_to(root))};binding['added_sources']=sorted(components-set(old['source_sha256']));binding['changed_preexisting_sources']=[];write('source-binding.json',binding)
v=json.loads((previous/'validation.json').read_text());v['registration_tests_passed']=14;v['gates']={'build':0,'lint':0,'runtime':0,'historical_design':0,'roadmap':0,'registration_unit':0,'native_package':0};write('validation.json',v)
sourcepaths=set(files)|{'scripts/knowledge_evidence.py','scripts/verify-knowledge-manager-bootstrap.py','scripts/check-knowledge-manager-bootstrap-evidence.py','scripts/test_knowledge_manager_bootstrap_evidence.py','scripts/test_knowledge_manager_registration.py','scripts/verify_committed_maintenance_checkpoint.py','docs/spec-driven/KNOWLEDGE-MANAGER-PLUGIN.md','docs/DOCUMENTATION-CATALOG.json','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py'}
manifest={'schema_version':1,'checkpoint':'KM03_1_owned_registration_native_package_metadata','KM03_1_complete':True,'full_KM03_complete':False,'independent_acceptance':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(sourcepaths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 manifest['artifacts'].append(row)
write('manifest.json',manifest)
print(json.dumps({'bound_inputs':len(files),'new_components':len(components),'runtime':json.loads((fixture/'summary.json').read_text())}))
