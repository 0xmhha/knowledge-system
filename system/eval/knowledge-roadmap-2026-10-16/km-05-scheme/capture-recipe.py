import gzip,hashlib,importlib.util,json,subprocess
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/km-05-scheme';base.mkdir(parents=True,exist_ok=True)
def sha(v):return hashlib.sha256(v).hexdigest()
def put(n,data,compressed=False):
 p=base/(n+('.gz'if compressed else''));p.parent.mkdir(exist_ok=True,parents=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
spec=importlib.util.spec_from_file_location('scheme_inventory',root/'scripts/check-knowledge-roadmap.py');inventory=importlib.util.module_from_spec(spec);spec.loader.exec_module(inventory)
validation={'current_Go_validation_reused':False,'Go_count_flag':1,'maintained_Go_files':1260,'gates':{'module':0,'race':0,'build':0,'lint':0,'docs':0,'historical_design':0,'roadmap':0},'commands':{},'execution_order':'final_resolver_race_then_final_module_sequentially'}
for label,args in [('module',['go','test','-json','-count=1','./...']),('race',['go','test','-race','-json','-count=1','./cmd/cks/mcpcli','./cmd/cks/setupcli','./internal/setup','./internal/system/config','./internal/system/maintenance','./internal/system/mcp','./internal/system/comparison'])]:
 exit=json.loads(Path('/private/tmp/ks-KM051-'+label+'-exit.json').read_text())['exit_code'];assert exit==0
 raw=Path('/private/tmp/ks-KM051-'+label+'.jsonl').read_bytes();counts,_=inventory.test_inventory(raw);assert counts['failed_tests']==counts['failed_packages']==0
 validation[label]=counts;validation['commands'][label]={'argv':args,'exit_code':exit};put(label+'.jsonl',raw,True);put(label+'.stderr',Path('/private/tmp/ks-KM051-'+label+'.stderr').read_bytes())
write('validation.json',validation)
put('initial-lint-failure.txt',Path('/private/tmp/ks-KM051-initial-lint-failure.txt').read_bytes())
write('initial-lint-scope.json',{'findings':1,'rule':'QF1001','fix':'Equivalent positive character membership branch; no contract or test expectation weakened.','final_module_and_race_reexecuted':True})
for source,dst in [('ks-KM051-build-lint.txt','build-lint.txt'),('ks-KM051-history.json','historical-design.json'),('ks-KM051-roadmap-tests.txt','roadmap-tests.txt'),('ks-KM051-initial.jsonl','initial-scheme-tests.jsonl')]:put(dst,Path('/private/tmp',source).read_bytes(),dst.endswith('.jsonl'))
for source,dst in [('ks-KM051-docs-final.txt','docs-final.txt'),('ks-KM051-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',source)
 if p.exists():put(dst,p.read_bytes())
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/km-04-periodic/source-binding.json').read_text());b=json.loads(json.dumps(old));files=set(old['source_sha256'])|{p.relative_to(root).as_posix()for directory in('internal/system/comparison','system/packs/industry-blockchain')for p in(root/directory).rglob('*')if p.is_file()};b.update(scope='KM05_1_optional_concept_scheme',source_sha256={n:sha((root/n).read_bytes())for n in sorted(files)},previous_source_binding={'revision':'2f9ddb52','path':'system/eval/knowledge-roadmap-2026-10-16/km-04-periodic/source-binding.json'});write('source-binding.json',b)
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/check-industry-scheme-evidence.py','scripts/test_industry_scheme_evidence.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/INDUSTRY-CONCEPT-COMPARISON.md'}
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
m={'schema_version':1,'checkpoint':'KM05_1_optional_concept_scheme','full_KM05_complete':False,'project_bindings_implemented':False,'comparison_MCP_implemented':False,'quality_acceptance':False,'independent_acceptance':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print(json.dumps({k:validation[k]['test_cases']for k in('module','race')}))
