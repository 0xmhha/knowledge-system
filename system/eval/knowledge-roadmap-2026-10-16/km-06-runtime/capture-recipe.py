import gzip,hashlib,importlib.util,json,shutil
from pathlib import Path
root=Path.cwd();base=root/'system/eval/knowledge-roadmap-2026-10-16/km-06-runtime';base.mkdir(parents=True,exist_ok=True)
def sha(v):return hashlib.sha256(v).hexdigest()
def put(n,data,compressed=False):
 p=base/(n+('.gz'if compressed else''));p.parent.mkdir(exist_ok=True,parents=True);p.write_bytes(gzip.compress(data,mtime=0)if compressed else data)
def write(n,v):put(n,(json.dumps(v,ensure_ascii=False,indent=2)+'\n').encode())
spec=importlib.util.spec_from_file_location('runtime_inventory',root/'scripts/check-knowledge-roadmap.py');inventory=importlib.util.module_from_spec(spec);spec.loader.exec_module(inventory)
validation={'current_Go_validation_reused':False,'Go_count_flag':1,'maintained_Go_files':1272,'gates':{'module':0,'race':0,'build':0,'lint':0,'docs':0,'historical_design':0,'roadmap':0},'commands':{},'execution_order':'final_module_then_final_race_sequentially'}
for label,args in [('module',['go','test','-json','-count=1','./...']),('race',['go','test','-race','-json','-count=1','./cmd/cks/mcpcli','./cmd/cks/setupcli','./internal/setup','./internal/system/config','./internal/system/maintenance','./internal/system/mcp','./internal/system/comparison'])]:
 code=json.loads(Path('/private/tmp/ks-KM062-'+label+'-exit.json').read_text())['exit_code'];assert code==0
 raw=Path('/private/tmp/ks-KM062-'+label+'.jsonl').read_bytes();counts,_=inventory.test_inventory(raw);assert counts['failed_tests']==counts['failed_packages']==0
 validation[label]=counts;validation['commands'][label]={'argv':args,'exit_code':code};put(label+'.jsonl',raw,True);put(label+'.stderr',Path('/private/tmp/ks-KM062-'+label+'.stderr').read_bytes())
write('validation.json',validation)
evidence=Path('/private/tmp/ks-KM062-evidence-tests.txt')
if evidence.exists():put('evidence-tests.txt',evidence.read_bytes())
for source,dst in [('ks-KM062-build-lint-final.txt','build-lint.txt'),('ks-KM062-history.json','historical-design.json'),('ks-KM062-roadmap-tests.txt','roadmap-tests.txt'),('ks-KM062-docs-final.txt','docs-final.txt')]:put(dst,Path('/private/tmp',source).read_bytes())
for source,dst in [('ks-KM062-docs-initial-link.txt','initial-failure/docs-packet-link.txt'),('ks-KM062-runtime-initial.jsonl','initial-failure/runtime-profile-fixture.jsonl'),('ks-KM062-native-initial-20261016/checks.json','initial-failure/native-checks.json'),('ks-KM062-native-initial-20261016/a-job.json','initial-failure/native-job.json'),('ks-KM062-native-initial.txt','initial-failure/native-verifier.txt')]:
 p=Path('/private/tmp',source)
 if p.exists():put(dst,p.read_bytes(),dst.endswith('.jsonl'))
native=Path('/private/tmp/ks-KM062-native-final-20261016')
for p in sorted(native.glob('*.json')):put('runtime/'+p.name,p.read_bytes())
for session in('bootstrap-session','matrix-session'):
 for p in (native/session).iterdir():put('runtime/'+session+'/'+p.name,p.read_bytes(),p.suffix=='.jsonl')
fixtures=json.loads((native/'fixtures.json').read_text())
for f in fixtures:
 source=Path(f['version'])/'sources'
 for p in source.rglob('*'):
  if p.is_file():put('archives/'+f['label']+'/'+p.relative_to(source).as_posix(),p.read_bytes())
old=json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/km-06-contract/source-binding.json').read_text());b=json.loads(json.dumps(old));files=set(old['source_sha256'])|{p.relative_to(root).as_posix()for directory in('cmd/cks/mcpcli','internal/system/comparison','internal/system/mcp')for p in(root/directory).rglob('*.go')if p.is_file()};b.update(scope='KM06_2_actual_readonly_comparison_MCP',source_sha256={n:sha((root/n).read_bytes())for n in sorted(files)},previous_source_binding={'revision':'c7de05e1','path':'system/eval/knowledge-roadmap-2026-10-16/km-06-contract/source-binding.json'});write('source-binding.json',b)
paths=files|{'scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/verify-comparison-MCP.py','scripts/check-comparison-runtime-evidence.py','scripts/test_comparison_runtime_evidence.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/COMPARISON-MCP-CONTRACT.md'}
put('capture-recipe.py',Path(__file__).read_bytes())
m={'schema_version':1,'checkpoint':'KM06_2_actual_readonly_comparison_MCP','full_KM05_complete':True,'comparison_MCP_implemented':True,'full_KM06_complete':False,'quality_acceptance':False,'independent_acceptance':False,'human_mapping_approval':False,'actual_real_project_comparison':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(paths)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if not p.is_file()or p.name=='manifest.json':continue
 data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
 if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
 m['artifacts'].append(row)
write('manifest.json',m);print(json.dumps({k:validation[k]['test_cases']for k in('module','race')}))
