import copy,gzip,hashlib,json,shutil
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/oe-03-score';base.mkdir(parents=True,exist_ok=True);previous=root/'system/eval/knowledge-roadmap-2026-10-16/oe-02-capture';actual=Path('/private/tmp/ks-OE012-expanded-third-20261016')
def sha(data):return hashlib.sha256(data).hexdigest()
def put(name,data):
 p=base/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
def write(name,v):put(name,(json.dumps(v,ensure_ascii=False,indent=2)+'\n').encode())
if not(base/'runtime').exists():shutil.copytree(previous/'runtime',base/'runtime')
for name in('effect-summary.json','function-retrieval-scores.json'):put('runtime/'+name,(actual/name).read_bytes())
write('recommendation.json',{'ontology_default':'off','quality':'inconclusive','independent_acceptance':False,'human_gold':False,'source_retrieval_scope':'finite_complete_function_tasks','benefit_question_ids':['EX-08'],'relation_links_available':0,'generated_answer_quality':'not_executed','production_activation':False})
for src,dst in [('ks-OE013-history.json','historical-design.json'),('ks-OE013-docs-final.txt','docs-final.txt'),('ks-OE013-roadmap-tests.txt','roadmap-tests.txt'),('ks-OE013-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
v=json.loads((previous/'validation.json').read_text());v['gates'].update(effect_recomputed=0,benefit_question_source_audit=0,scope_cost_latency_audit=0);write('validation.json',v)
b=copy.deepcopy(json.loads((previous/'source-binding.json').read_text()));b.update(scope='OE01_3_expanded_function_retrieval_effect_scored',previous_source_binding={'revision':'e44c58f0','path':'system/eval/knowledge-roadmap-2026-10-16/oe-02-capture/source-binding.json'});write('source-binding.json',b)
files=set(b['source_sha256'])|{'scripts/knowledge_evidence.py','scripts/check-knowledge-roadmap.py','scripts/test_knowledge_roadmap_current.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/run-ontology-effect-expanded.py','scripts/score-ontology-effect-expanded.py','scripts/score-ontology-effect-dev.py','scripts/check-ontology-expanded-score.py','scripts/test_ontology_expanded_score.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/ONTOLOGY-EFFECT-DEV-OBSERVATION.md'};put('capture-recipe.py',Path(__file__).read_bytes());m={'schema_version':1,'checkpoint':'OE01_3_expanded_function_retrieval_effect_scored','effect_scored':True,'quality_acceptance':False,'independent_acceptance':False,'human_label_approval':False,'full_OE01_complete':False,'LLM_answer_quality_evaluated':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(files)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print('expanded effect scores captured; off/inconclusive retained')
