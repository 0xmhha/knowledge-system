import copy,gzip,hashlib,json
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/oe-01-protocol';base.mkdir(parents=True,exist_ok=True)
def sha(data):return hashlib.sha256(data).hexdigest()
def put(name,data,compress=False):
 p=base/(name+('.gz'if compress else''));p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(data,mtime=0)if compress else data)
def write(name,v):put(name,(json.dumps(v,ensure_ascii=False,indent=2)+'\n').encode())
prior=root/'system/eval/knowledge-roadmap-2026-10-16/km-06-failures';pv=json.loads((prior/'validation.json').read_text());write('validation.json',{'current_Go_validation_reused':True,'Go_inputs_unchanged':True,'Go_reference':{'revision':'dd554e79','manifest':(prior/'manifest.json').relative_to(root).as_posix(),'manifest_sha256':sha((prior/'manifest.json').read_bytes()),'maintained_Go_files_equal':1273},'module':pv['module'],'race':pv['race'],'gates':{'unchanged_Go_build_test_race_fmt_vet_lint_reference':0,'new_input_audit':0,'historical_restore':0,'docs':0,'roadmap':0}})
inputs=Path('/private/tmp/ks-OE011-protocol-sealed-20261016')
for p in inputs.glob('*.json'):put('inputs/'+p.name,p.read_bytes())
pins=json.loads((inputs/'source-pins.json').read_text());version=Path(pins['version_dir']);put('source-manifest.json',(version/'sources/manifest.json').read_bytes());put('dataset-identity.json',(version/'dataset-identity.json').read_bytes());oracle=json.loads((inputs/'private-function-criteria.json').read_text())
for digest in{u['file_sha256']for u in oracle['units']}:put('source-blobs/'+digest,(version/'sources/blobs'/digest).read_bytes())
for src,dst in [('ks-OE011-history.json','historical-design.json'),('ks-OE011-model-server.json','model-server.json'),('ks-OE011-docs-final.txt','docs-final.txt'),('ks-OE011-roadmap-tests.txt','roadmap-tests.txt'),('ks-OE011-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
write('new-query-execution.json',{'new_retrieval_calls':0,'LLM_answer_calls':0,'paid_model_calls':0})
b=copy.deepcopy(json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/source-binding.json').read_text()));b.update(scope='OE01_1_expanded_function_retrieval_preregistration',previous_source_binding={'revision':'efd43995','path':'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/source-binding.json'});write('source-binding.json',b)
files=set(b['source_sha256'])|{'scripts/knowledge_evidence.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/prepare-ontology-effect-expanded.py','scripts/check-ontology-expanded-protocol.py','scripts/test_ontology_expanded_protocol.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/ONTOLOGY-EFFECT-DEV-OBSERVATION.md'};put('capture-recipe.py',Path(__file__).read_bytes())
m={'schema_version':1,'checkpoint':'OE01_1_expanded_function_retrieval_preregistration','preregistration_complete':True,'new_retrieval_executed':False,'quality_acceptance':False,'independent_acceptance':False,'human_label_approval':False,'full_OE01_complete':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(files)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print('expanded protocol packet captured; Go reference unchanged; retrieval0')
