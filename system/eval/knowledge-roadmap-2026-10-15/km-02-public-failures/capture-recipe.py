import gzip,hashlib,importlib.util,json,subprocess,tarfile,io
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-02-public-failures';base.mkdir(exist_ok=True)
fixture=Path('/private/tmp/ks-KM024-failure-final-20261015')
def sha(b):return hashlib.sha256(b).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
for path in fixture.rglob('*'):
 if not path.is_file()or path.is_symlink()or len(path.relative_to(fixture).parts)>=3:continue
 if path.suffix not in('.json','.jsonl','.txt')or path.parent.name in('project-a','project-b','foreign-semantic'):continue
 target=base/'runtime'/path.relative_to(fixture);target.parent.mkdir(parents=True,exist_ok=True)
 if path.suffix=='.jsonl'or path.name in('calls.json','observations.json'):Path(str(target)+'.gz').write_bytes(gzip.compress(path.read_bytes(),mtime=0))
 else:target.write_bytes(path.read_bytes())
stream=io.BytesIO()
with tarfile.open(fileobj=stream,mode='w')as archive:
 for p in sorted(fixture.iterdir()):
  if p.name.startswith('project-')or p.name in('foreign-semantic','owned-engine'):archive.add(p,arcname=p.name,recursive=True)
(base/'fixture.tar.gz').write_bytes(gzip.compress(stream.getvalue(),mtime=0))
for dst in ['build.txt','lint.txt']:(base/dst).write_bytes(Path('/private/tmp/ks-KM024-build-lint.txt').read_bytes())
(base/'historical-design.json').write_bytes(Path('/private/tmp/ks-KM024-history.json').read_bytes())
oldbase=root/'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-retention';old=json.loads((oldbase/'source-binding.json').read_text())
for n,h in {**old['source_sha256'],**old['approved_input_sha256']}.items():assert sha((root/n).read_bytes())==h
v=json.loads((oldbase/'validation.json').read_text());v['current_Go_validation_reused']=True;v['Go_inputs_unchanged']=True;v['Go_reference']={'revision':'ae3f987d','manifest':str((oldbase/'manifest.json').relative_to(root)),'manifest_sha256':sha((oldbase/'manifest.json').read_bytes()),'maintained_Go_files_equal':sum(n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))for n in old['source_sha256']),'scope':'same_source_and_inputs_only_count1_E2_execution_not_fresh_KM02_4_Go_execution'};write('validation.json',v)
extra={str((oldbase/n).relative_to(root))for n in ['manifest.json','module.jsonl.gz','race.jsonl.gz']};binding=json.loads(json.dumps(old));files=sorted(set(old['source_sha256'])|extra);binding['source_sha256']={n:sha((root/n).read_bytes())for n in files};binding['scope']='unchanged_Go_and_actual_failure_matrix_inputs_after_KM02_4';binding['previous_source_binding']={'revision':'ae3f987d','path':str((oldbase/'source-binding.json').relative_to(root))};binding['added_sources']=sorted(extra-set(old['source_sha256']));binding['changed_preexisting_sources']=[];write('source-binding.json',binding)
sourcepaths=set(files)|{'scripts/verify-maintenance-failures.py','scripts/check-maintenance-failure-evidence.py','scripts/test_maintenance_failure_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/dataset_contract.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','system/contracts/versioned-maintenance/v1/schema.json','system/contracts/versioned-maintenance/v1/state-machine.json'}
wirepairs=sum(len(p.read_text().splitlines())for p in fixture.glob('*/mcp.stdin.jsonl'));manifest={'schema_version':1,'checkpoint':'KM02_4_actual_public_failure_preservation','full_KM02_complete':True,'independent_acceptance':False,'quality':'not_evaluated','wire_pairs':wirepairs,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(sourcepaths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 manifest['artifacts'].append(row)
write('manifest.json',manifest)
print(json.dumps({'bound_inputs':len(files),'wire_pairs':wirepairs,'runtime':json.loads((fixture/'summary.json').read_text())}))
