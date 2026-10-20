import copy,gzip,hashlib,importlib.util,json
from pathlib import Path
root=Path.cwd();runtime=Path('/private/tmp/ks-OE012-expanded-third-20261016');base=root/'system/eval/knowledge-roadmap-2026-10-16/oe-02-capture';base.mkdir(parents=True,exist_ok=True)
def sha(data):return hashlib.sha256(data).hexdigest()
def put(name,data,compressed=False):
 p=base/(name+('.gz'if compressed else''));p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
def write(name,v):put(name,(json.dumps(v,ensure_ascii=False,indent=2)+'\n').encode())
spec=importlib.util.spec_from_file_location('expanded_source_verify',root/'scripts/score-ontology-effect-expanded.py');scorer=importlib.util.module_from_spec(spec);spec.loader.exec_module(scorer);verification,*tail=scorer.verify(runtime);used,sources=tail[-2:]
assert verification['actual_capture_calls']==512 and verification['failed_calls']==0
for p in runtime.iterdir():
 if p.is_file():put('runtime/'+p.name,p.read_bytes(),p.name.startswith('EX-')and p.suffix=='.json'and not p.name.endswith('-request.json')or p.suffix=='.jsonl')
for directory in('preregistered',*('readiness-'+a for a in('off','concept_text','relations','combined'))):
 for p in(runtime/directory).iterdir():
  if p.is_file():put('runtime/'+directory+'/'+p.name,p.read_bytes(),p.suffix=='.jsonl')
pins=json.loads((runtime/'preregistered/source-pins.json').read_text());version=Path(pins['version_dir']);put('runtime/source-manifest.json',(version/'sources/manifest.json').read_bytes());put('runtime/dataset-identity.json',(version/'dataset-identity.json').read_bytes())
for digest,data in sources.items():put('runtime/source-blobs/'+digest,data)
for src,dst in [('ks-OE012-history.json','historical-design.json'),('ks-OE012-docs-final.txt','docs-final.txt'),('ks-OE012-roadmap-tests.txt','roadmap-tests.txt'),('ks-OE012-evidence-tests.txt','evidence-tests.txt'),('ks-OE012-expanded-third.txt','capture-log.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
for label in('initial','second'):
 folder=Path('/private/tmp/ks-OE012-expanded-'+label+'-20261016')
 for name in('executed-runner.py','commands.json','meter-closure.json','api-meter.json','capture-inventory.json'):
  p=folder/name
  if p.exists():put('initial-attempts/'+label+'/'+name,p.read_bytes(),p.suffix=='.json'and name=='api-meter.json')
 p=Path('/private/tmp/ks-OE012-expanded-'+label+'.txt')
 if p.exists():put('initial-attempts/'+label+'/stderr.txt',p.read_bytes())
p=Path('/private/tmp/ks-OE012-controlled-stop.json')
if p.exists():put('initial-attempts/controlled-stop.json',p.read_bytes())
prior=root/'system/eval/knowledge-roadmap-2026-10-16/km-06-failures';pv=json.loads((prior/'validation.json').read_text());write('validation.json',{'current_Go_validation_reused':True,'Go_inputs_unchanged':True,'Go_reference':{'revision':'dd554e79','manifest':(prior/'manifest.json').relative_to(root).as_posix(),'manifest_sha256':sha((prior/'manifest.json').read_bytes()),'maintained_Go_files_equal':1273},'module':pv['module'],'race':pv['race'],'gates':{'Go_reference':0,'signed_preregistration':0,'actual_capture':0,'source_verification':0,'EOF_meter_cleanup':0,'docs':0,'roadmap':0,'history':0}})
b=copy.deepcopy(json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/oe-01-protocol/source-binding.json').read_text()));b.update(scope='OE01_2_expanded_four_arm_capture',previous_source_binding={'revision':'f73b9e36','path':'system/eval/knowledge-roadmap-2026-10-16/oe-01-protocol/source-binding.json'});write('source-binding.json',b)
files=set(b['source_sha256'])|{'scripts/knowledge_evidence.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/prepare-ontology-effect-expanded.py','scripts/run-ontology-effect-expanded.py','scripts/score-ontology-effect-expanded.py','scripts/score-ontology-effect-dev.py','scripts/check-ontology-expanded-capture.py','scripts/test_ontology_expanded_capture.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/ONTOLOGY-EFFECT-DEV-OBSERVATION.md'};put('capture-recipe.py',Path(__file__).read_bytes());m={'schema_version':1,'checkpoint':'OE01_2_expanded_four_arm_capture','four_arm_capture_complete':True,'effect_scored':False,'quality_acceptance':False,'independent_acceptance':False,'human_label_approval':False,'full_OE01_complete':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(files)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print('actual512 capture and source proofs sealed; effect scoring not promoted')
