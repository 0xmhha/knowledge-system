import gzip,hashlib,importlib.util,json,subprocess
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/km-06-contract';base.mkdir(parents=True,exist_ok=True)
def sha(v):return hashlib.sha256(v).hexdigest()
def put(n,data,compressed=False):
 p=base/(n+('.gz'if compressed else''));p.parent.mkdir(exist_ok=True,parents=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
def write(n,v):(base/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
spec=importlib.util.spec_from_file_location('scheme_inventory',root/'scripts/check-knowledge-roadmap.py');inventory=importlib.util.module_from_spec(spec);spec.loader.exec_module(inventory)
validation={'current_Go_validation_reused':False,'Go_count_flag':1,'maintained_Go_files':1268,'gates':{'module':0,'race':0,'build':0,'lint':0,'docs':0,'historical_design':0,'roadmap':0},'commands':{},'execution_order':'final_module_then_final_race_sequentially'}
for label,args in [('module',['go','test','-json','-count=1','./...']),('race',['go','test','-race','-json','-count=1','./cmd/cks/mcpcli','./cmd/cks/setupcli','./internal/setup','./internal/system/config','./internal/system/maintenance','./internal/system/mcp','./internal/system/comparison'])]:
 exit=json.loads(Path('/private/tmp/ks-KM061-'+label+'-exit.json').read_text())['exit_code'];assert exit==0
 raw=Path('/private/tmp/ks-KM061-'+label+'.jsonl').read_bytes();counts,_=inventory.test_inventory(raw);assert counts['failed_tests']==counts['failed_packages']==0
 validation[label]=counts;validation['commands'][label]={'argv':args,'exit_code':exit};put(label+'.jsonl',raw,True);put(label+'.stderr',Path('/private/tmp/ks-KM061-'+label+'.stderr').read_bytes())
write('validation.json',validation)
put('final-focused.jsonl',Path('/private/tmp/ks-KM061-focused-initial.jsonl').read_bytes(),True)
write('review-scope.json',{'actual_human_reviews_created':0,'reviewer_identity_authenticated':False,'independent_quality_acceptance':False,'controlled_review_records_only':True,'new_project_bindings_auto_promoted':False,'core20_and_legacy_ontology_unchanged':True})
write('contract-scope.json',{'actual_source_reads':0,'actual_MCP_tool_calls':0,'profile_runtime_loader_implemented':False,'request_contract_cases':50,'comparison_package_cases':167,'backend_execution':False,'independent_quality_acceptance':False})
for source,dst in [('ks-KM061-build-lint.txt','build-lint.txt'),('ks-KM061-history.json','historical-design.json'),('ks-KM061-roadmap-tests.txt','roadmap-tests.txt'),('ks-KM061-focused-initial.jsonl','focused-contract-tests.jsonl')]:put(dst,Path('/private/tmp',source).read_bytes(),dst.endswith('.jsonl'))
for source,dst in [('ks-KM061-docs-final.txt','docs-final.txt'),('ks-KM061-evidence-tests.txt','evidence-tests.txt')]:
 p=Path('/private/tmp',source)
 if p.exists():put(dst,p.read_bytes())
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/km-05-applicability/source-binding.json').read_text());b=json.loads(json.dumps(old));files=set(old['source_sha256'])|{p.relative_to(root).as_posix()for directory in('internal/system/comparison','system/packs/industry-blockchain')for p in(root/directory).rglob('*')if p.is_file()};b.update(scope='KM06_1_explicit_readonly_comparison_contract',source_sha256={n:sha((root/n).read_bytes())for n in sorted(files)},previous_source_binding={'revision':'ba807f5e','path':'system/eval/knowledge-roadmap-2026-10-16/km-05-applicability/source-binding.json'});write('source-binding.json',b)
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/check-comparison-contract-evidence.py','scripts/test_comparison_contract_evidence.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/INDUSTRY-CONCEPT-COMPARISON.md','docs/spec-driven/COMPARISON-MCP-CONTRACT.md'}
(base/'capture-recipe.py').write_bytes(Path(__file__).read_bytes())
m={'schema_version':1,'checkpoint':'KM06_1_explicit_readonly_comparison_contract','full_KM05_complete':True,'comparison_contract_implemented':True,'full_KM06_complete':False,'project_bindings_implemented':True,'mapping_review_runtime_implemented':True,'human_mapping_approval':False,'comparison_MCP_implemented':False,'quality_acceptance':False,'independent_acceptance':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print(json.dumps({k:validation[k]['test_cases']for k in('module','race')}))
