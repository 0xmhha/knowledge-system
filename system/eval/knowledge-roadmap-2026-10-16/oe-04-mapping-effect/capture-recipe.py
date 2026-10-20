import copy,gzip,hashlib,json,shutil
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect';base.mkdir(parents=True,exist_ok=True);previous=root/'system/eval/knowledge-roadmap-2026-10-16/oe-03-score';actual=Path('/private/tmp/ks-OE014-multiproject-initial-20261016');real=root/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects'
def sha(data):return hashlib.sha256(data).hexdigest()
def put(name,data):
 p=base/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
def write(name,v):put(name,(json.dumps(v,ensure_ascii=False,indent=2)+'\n').encode())
for p in actual.rglob('*'):
 if p.is_file():
  name='runtime/'+p.relative_to(actual).as_posix();data=p.read_bytes();put(name+'.gz',gzip.compress(data,mtime=0))if p.suffix=='.jsonl'else put(name,data)
for name in('fixtures.json','queries.json','bindings.json'):put('inputs/'+name,(real/'runtime'/name).read_bytes())
if not(base/'archives').exists():shutil.copytree(real/'archives',base/'archives')
for p in (base/'archives').rglob('manifest.json'):
 p.with_suffix('.json.gz').write_bytes(gzip.compress(p.read_bytes(),mtime=0));p.unlink()
for src,dst in [('ks-OE014-history.json','historical-design.json'),('ks-OE014-docs-final.txt','docs-final.txt'),('ks-OE014-roadmap-tests.txt','roadmap-tests.txt'),('ks-OE014-evidence-tests.txt','evidence-tests.txt'),('ks-OE014-document-contract.txt','document-contract.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
v=json.loads((previous/'validation.json').read_text());v['gates'].update(actual_mapping_MCP=0,raw_coverage_recomputed=0,project_scope_and_source=0);write('validation.json',v)
b=copy.deepcopy(json.loads((previous/'source-binding.json').read_text()));b.update(scope='OE01_4_explicit_mapping_input_effect',previous_source_binding={'revision':'c0c77897','path':'system/eval/knowledge-roadmap-2026-10-16/oe-03-score/source-binding.json'});write('source-binding.json',b)
files=set(b['source_sha256'])|{'scripts/knowledge_evidence.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/eval-multiproject-mapping-effect.py','scripts/eval-value-dependencies.py','scripts/check-multiproject-mapping-effect.py','scripts/test_multiproject_mapping_effect.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/ONTOLOGY-EFFECT-DEV-OBSERVATION.md','docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json','docs/spec-driven/KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md','docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md','docs/spec-driven/CURRENT-HANDOFF.md','docs/REMAINING-WORK.md'};put('capture-recipe.py',Path(__file__).read_bytes());m={'schema_version':1,'checkpoint':'OE01_4_explicit_mapping_input_effect','full_OE01_complete':True,'full_OE01_scope':'bounded_local_source_retrieval_and_annotation_input_studies_only','quality_acceptance':False,'independent_acceptance':False,'human_mapping_approval':False,'LLM_answer_quality_evaluated':False,'ontology_vocabulary_alone_effect_isolated':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(files)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p==base/'manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print('mapping input effect packet captured; local roadmap complete, quality inconclusive')
