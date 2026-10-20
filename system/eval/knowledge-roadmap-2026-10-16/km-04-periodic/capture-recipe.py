import gzip,hashlib,io,json,tarfile
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/km-04-periodic';base.mkdir(exist_ok=True,parents=True);runtime=Path('/private/tmp/ks-KM044-periodic-second-20261016')
def sha(v):return hashlib.sha256(v).hexdigest()
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def put(n,data,compressed=False):
 p=base/(n+('.gz'if compressed else''));p.parent.mkdir(exist_ok=True,parents=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
for p in runtime.iterdir():
 if p.is_file():put('runtime/'+p.name,p.read_bytes(),p.suffix=='.jsonl')
 elif p.is_dir()and p.name.endswith('-session'):
  for child in p.iterdir():
   if child.is_file():put('runtime/'+p.name+'/'+child.name,child.read_bytes(),child.suffix=='.jsonl')
stream=io.BytesIO()
def safe(info):
 if info.issym():
  assert info.name in ('fixture/project-a/dataset/current','fixture/project-b/dataset/current') and '/'not in info.linkname
 else:assert info.isfile()or info.isdir()
 return info
with tarfile.open(fileobj=stream,mode='w')as archive:
 for name in('project-a','project-b'):archive.add(runtime/name,arcname='fixture/'+name,filter=safe)
put('fixture.tar',stream.getvalue(),True)
initial=Path('/private/tmp/ks-KM044-periodic-initial-20261016')
for src,dst in [('a1-terms-read.json','initial-identifier-read.json'),('query-diagnosis.json','initial-query-diagnosis.json')]:put(dst,(initial/src).read_bytes())
put('initial-failed-run.txt',Path('/private/tmp/ks-KM044-runtime-initial.txt').read_bytes())
write('query-limitation.json',{'standalone_synthetic_identifier_zero_bodies':True,'same_material_retrieved_with_natural_language':True,'purpose':'lifecycle data preservation, not semantic retrieval quality','followup':'OE-01 identifier/natural-language interpretation and recall coverage','fixed_retrieval_quality':False})
for src,dst in [('ks-KM044-build-lint.txt','build-lint.txt'),('ks-KM044-history-final.json','historical-design.json'),('ks-KM044-roadmap.txt','roadmap.txt'),('ks-KM044-roadmap-tests.txt','roadmap-tests.txt'),('ks-KM044-runtime-second.txt','runtime-log.txt')]:put(dst,Path('/private/tmp',src).read_bytes())
for src,dst in [('ks-KM044-docs-final.txt','docs-final.txt'),('ks-KM044-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
prior=root/'system/eval/knowledge-roadmap-2026-10-16/km-04-updates-MCP';b=json.loads((prior/'source-binding.json').read_text());assert all(sha((root/n).read_bytes())==h for n,h in b['source_sha256'].items());b['scope']='KM04_complete_actual_two_project_periodic_change_lifecycle';b['previous_source_binding']={'revision':'4a2ca4cd','path':str((prior/'source-binding.json').relative_to(root))};write('source-binding.json',b)
v=json.loads((prior/'validation.json').read_text());v['gates']={'build_lint':0,'actual_periodic':0,'historical_design':0,'roadmap':0,'docs':0};v['manager_python_tests_reused']=v.pop('manager_python_tests');v['python_reference']={'revision':'4a2ca4cd','byte_inventory_unchanged':True,'test_cases':268};write('validation.json',v)
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
paths=set(b['source_sha256'])|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/eval-value-dependencies.py','scripts/verify-manager-periodic-updates.py','scripts/check-manager-periodic-evidence.py','scripts/test_manager_periodic_evidence.py','docs/spec-driven/KNOWLEDGE-MANAGER-UPDATES.md'}
m={'schema_version':1,'checkpoint':'KM04_4_actual_periodic_two_project_final','full_KM04_complete':True,'independent_acceptance':False,'paid_model_calls':0,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print('Captured periodic lifecycle:',len(m['artifacts']),'artifacts')
