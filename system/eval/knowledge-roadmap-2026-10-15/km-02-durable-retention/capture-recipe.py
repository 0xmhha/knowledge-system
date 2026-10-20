import gzip,hashlib,importlib.util,json,subprocess,tarfile,io
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-retention';base.mkdir(exist_ok=True)
fixture=Path('/private/tmp/ks-E2-retention-verified-20261015')
def sha(b):return hashlib.sha256(b).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def compress(n,b):(base/(n+'.gz')).write_bytes(gzip.compress(b,mtime=0))
for path in fixture.rglob('*'):
 if not path.is_file() or path.is_symlink() or len(path.relative_to(fixture).parts)>=3:continue
 if path.suffix not in ('.json','.jsonl','.txt') or path.parent.name in ('project-a','project-b'):continue
 target=base/'runtime'/path.relative_to(fixture);target.parent.mkdir(parents=True,exist_ok=True)
 if path.suffix=='.jsonl' or path.name in ('calls.json','observations.json'):Path(str(target)+'.gz').write_bytes(gzip.compress(path.read_bytes(),mtime=0))
 else:target.write_bytes(path.read_bytes())
stream=io.BytesIO()
with tarfile.open(fileobj=stream,mode='w')as archive:
 for p in sorted(fixture.iterdir()):
  if p.name.startswith('project-'):archive.add(p,arcname=p.name,recursive=True)
compress('fixture.tar',stream.getvalue())
for src,dst in [('ks-E2-all-go.jsonl','module.jsonl'),('ks-E2-related-race.jsonl','race.jsonl')]:compress(dst,Path('/private/tmp',src).read_bytes())
for dst in ['build.txt','lint.txt']:(base/dst).write_bytes(Path('/private/tmp/ks-E2-build-lint.txt').read_bytes())
(base/'historical-design.json').write_bytes(Path('/private/tmp/ks-E2-history.json').read_bytes())
spec=importlib.util.spec_from_file_location('guard',root/'scripts/check-knowledge-roadmap.py');guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)
validation={'gates':{'build':0,'lint':0,'runtime':0,'historical_design':0,'roadmap':0},'current_Go_validation_reused':False,'Go_count_flag':1}
for name in ('module','race'):
 data=gzip.decompress((base/(name+'.jsonl.gz')).read_bytes());validation[name]=guard.test_inventory(data)[0];write(name+'-tests.json',guard.test_rows(data))
write('validation.json',validation)
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/source-binding.json').read_text())
extra={'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/'+n for n in ['fixture.tar.gz','manifest.json','runtime/observations.json.gz']}
binding=json.loads(json.dumps(old));tracked=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','--','*.go'],text=True).splitlines();files=sorted({n for n in tracked if n.startswith(('cmd/','internal/','pkg/'))}|set(old['source_sha256'])|extra)
binding['source_sha256']={n:sha((root/n).read_bytes())for n in files};binding['scope']='current_sources_and_actual_E1_fixture_inputs_after_KM02_3_E2';binding['previous_source_binding']={'revision':'0fa42cef','path':'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/source-binding.json'};binding['added_sources']=sorted(set(files)-set(old['source_sha256']));binding['changed_preexisting_sources']=sorted(n for n in old['source_sha256']if binding['source_sha256'][n]!=old['source_sha256'][n]);write('source-binding.json',binding)
summary=json.loads((fixture/'summary.json').read_text());wirepairs=sum(len(p.read_text().splitlines())for p in fixture.glob('*/mcp.stdin.jsonl'))
sourcepaths=set(files)|{'scripts/verify-durable-retention.py','scripts/check-maintenance-retention-evidence.py','scripts/test_maintenance_retention_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/dataset_contract.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','system/contracts/versioned-maintenance/v1/schema.json','system/contracts/versioned-maintenance/v1/state-machine.json'}
manifest={'schema_version':1,'checkpoint':'KM02_3_E2_actual_public_reader_rollback_GC','KM02_3_complete':True,'full_KM02_complete':False,'independent_acceptance':False,'paid_model_calls':0,'wire_pairs':wirepairs,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(sourcepaths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file() or p.name=='manifest.json':continue
 raw=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(raw),'bytes':len(raw)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(raw))
 manifest['artifacts'].append(row)
write('manifest.json',manifest)
print(json.dumps({'module':validation['module'],'race':validation['race'],'bound_inputs':len(files),'wire_pairs':wirepairs,'runtime':summary},ensure_ascii=False))
