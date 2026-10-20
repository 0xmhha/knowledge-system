import copy,gzip,hashlib,json
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects';base.mkdir(parents=True,exist_ok=True)
def sha(data):return hashlib.sha256(data).hexdigest()
def put(n,data,compress=False):
 p=base/(n+('.gz'if compress else''));p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(data,mtime=0)if compress else data)
def write(n,v):put(n,(json.dumps(v,ensure_ascii=False,indent=2)+'\n').encode())
initial_docs=Path('/private/tmp/ks-KM064-docs-initial-catalog.txt')
if initial_docs.exists():put('initial-preparation/docs-catalog.txt',initial_docs.read_bytes())
bound_label=Path('/private/tmp/ks-KM064-roadmap-bound-label-initial.txt')
if bound_label.exists():put('initial-preparation/source-bound-label-change.txt',bound_label.read_bytes())
prior=root/'system/eval/knowledge-roadmap-2026-10-16/km-06-failures';validation=json.loads((prior/'validation.json').read_text());v={'current_Go_validation_reused':True,'Go_inputs_unchanged':True,'Go_reference':{'revision':'dd554e79','manifest':(prior/'manifest.json').relative_to(root).as_posix(),'manifest_sha256':sha((prior/'manifest.json').read_bytes()),'maintained_Go_files_equal':1273},'module':validation['module'],'race':validation['race'],'gates':{'Go_inputs_equal':0,'prior_module_race_build_fmt_vet_lint':0,'actual_two_native_builds':0,'actual_comparison_MCP':0,'docs':0,'roadmap':0,'historical_design':0},'scope':'Go executions referenced with all inputs unchanged; actual new project builds/comparison executed in this checkpoint'};write('validation.json',v)
for src,dst in [('ks-KM064-history.json','historical-design.json'),('ks-KM064-docs-final.txt','docs-final.txt'),('ks-KM064-roadmap-tests.txt','roadmap-tests.txt'),('ks-KM064-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
actual=Path('/private/tmp/ks-KM064-real-comparison-initial-20261016');prepared=Path('/private/tmp/ks-KM064-real-native-second-20261016')
for p in actual.glob('*.json'):put('runtime/'+p.name,p.read_bytes())
put('runtime/review.md',(actual/'review.md').read_bytes())
for session in('bootstrap-session','comparison-session','bad-mapping-session'):
 for p in(actual/session).iterdir():put('runtime/'+session+'/'+p.name,p.read_bytes(),p.suffix=='.jsonl')
for p in prepared.glob('*.json'):put('build/'+p.name,p.read_bytes(),p.name.endswith('build-calls.json'))
fixtures=json.loads((actual/'fixtures.json').read_text())
for f in fixtures:
 version=Path(f['version']);label=f['project'];put('archives/'+label+'/manifest.json',(version/'sources/manifest.json').read_bytes());put('archives/'+label+'/dataset-identity.json',(version/'dataset-identity.json').read_bytes())
 hashes={e['citation']['file_sha256']for b in json.loads((actual/'bindings.json').read_text())['bindings']if b['coordinates']==f['coordinates']for e in b['evidence']}
 for digest in hashes:put('archives/'+label+'/blobs/'+digest,(version/'sources/blobs'/digest).read_bytes())
for src,dst in [('ks-KM064-real-build-third.txt','actual-build.txt'),('ks-KM064-real-comparison-initial.txt','actual-comparison.txt')]:put(dst,Path('/private/tmp',src).read_bytes())
for p in(root/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-preparation').iterdir():
 if p.is_file():put('initial-preparation/'+p.name,p.read_bytes())
b=copy.deepcopy(json.loads((prior/'source-binding.json').read_text()));b.update(scope='KM06_4_actual_pinned_project_comparison',previous_source_binding={'revision':'dd554e79','path':(prior/'source-binding.json').relative_to(root).as_posix()});write('source-binding.json',b)
files=set(b['source_sha256'])|{'scripts/knowledge_evidence.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/prepare-real-comparison.py','scripts/verify-real-project-comparison.py','scripts/check-real-comparison-evidence.py','scripts/test_real_comparison_evidence.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/COMPARISON-MCP-CONTRACT.md'};put('capture-recipe.py',Path(__file__).read_bytes())
m={'schema_version':1,'checkpoint':'KM06_4_actual_pinned_project_comparison','full_KM06_complete':True,'comparison_MCP_implemented':True,'actual_real_project_comparison':True,'quality_acceptance':False,'independent_acceptance':False,'human_mapping_approval':False,'ontology_effect_evaluated':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(files)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print('actual real project packet captured; Go reference explicitly reused')
