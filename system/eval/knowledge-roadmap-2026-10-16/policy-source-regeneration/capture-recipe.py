import copy,gzip,hashlib,importlib.util,json,shutil,subprocess
from pathlib import Path
root=Path.cwd();actual=Path('/private/tmp/ks-policy-regeneration-current-20261016');gates=Path('/private/tmp/ks-policy-regeneration-gates-20261016');base=root/'system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration';base.mkdir(parents=True,exist_ok=True)
def sha(data):return hashlib.sha256(data).hexdigest()
def put(name,data):
 p=base/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
def write(name,v):put(name,(json.dumps(v,ensure_ascii=False,indent=2)+'\n').encode())
commands=json.loads((gates/'gates.json').read_text());assert set(commands)=={'module','race','build','fmt','lint'}and all(x['exit_code']==0 for x in commands.values())
for p in actual.iterdir():
 if p.is_file():put(p.name,p.read_bytes())
for p in(actual/'cks').iterdir():
 if p.is_file():put('cks/'+p.name,p.read_bytes())
put('owned-corrected-A11.yaml.txt',(actual/'owned-domain-knowledge/entries/A11.txpool.type_taxonomy_admission.yaml').read_bytes())
for p in gates.iterdir():
 if p.is_file():put(p.name+'.gz',gzip.compress(p.read_bytes(),mtime=0))if p.suffix=='.jsonl'else put('gates/'+p.name,p.read_bytes())
for n in ('initial','second'):
 folder=Path('/private/tmp/ks-policy-regeneration-'+n+'-20261016')
 for p in folder.iterdir():
  if p.is_file():put('development/'+n+'/'+p.name,p.read_bytes())
for p in Path('/private/tmp/ks-policy-regeneration-gates-initial-stopped-20261016').iterdir():
 if p.is_file():put('development/initial-validation/'+p.name+'.gz',gzip.compress(p.read_bytes(),mtime=0))
for p in Path('/private/tmp/ks-policy-regeneration-gates-before-close-fix-20261016').iterdir():
 if p.is_file():put('development/before-close-fix/'+p.name+'.gz',gzip.compress(p.read_bytes(),mtime=0))
put('development/evidence-test-inventory-failed.txt',Path('/private/tmp/ks-policy-regeneration-evidence-inventory-failed.txt').read_bytes())
put('development/evidence-test-initial.txt',Path('/private/tmp/ks-policy-regeneration-evidence-initial.txt').read_bytes())
for name in ('ks-policy-regeneration-initial.txt','ks-policy-regeneration-second.txt'):
 put('development/'+name,Path('/private/tmp',name).read_bytes())
write('development/interrupted-validation.json',{'result':'excluded_incomplete_pre_fix_execution','controlled_stop':True,'reason':'missing struct/interface definition support discovered during real anchor verification','latest_run_restarted_on_final_source':True})
put('historical-design.json',Path('/private/tmp/ks-policy-regeneration-history.json').read_bytes())
for src,dst in [('ks-policy-regeneration-docs.txt','docs-final.txt'),('ks-policy-regeneration-roadmap.txt','roadmap-tests.txt'),('ks-policy-regeneration-evidence.txt','evidence-tests.txt'),('ks-policy-regeneration-document-contract.txt','document-contract.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
# Original source status has a previously signed baseline; original inputs are
# checked against HEAD, not re-labelled as a pre-trial sample taken afterwards.
original=json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/build/original-before.json').read_text())[0]
original_after=dict(original,head=subprocess.check_output(['git','-C',original['source_root'],'rev-parse','HEAD'],text=True).strip(),status=subprocess.check_output(['git','-C',original['source_root'],'status','--porcelain'],text=True).strip())
inputs=list((root/'projects/stablenet/domain-knowledge').rglob('*.yaml'))+list((root/'projects/stablenet/evaluation').glob('*'))+[root/'projects/stablenet/policies/graph.yaml']
before={p.relative_to(root).as_posix():sha(subprocess.check_output(['git','show','HEAD:'+p.relative_to(root).as_posix()]))for p in inputs if p.is_file()};after={p.relative_to(root).as_posix():sha(p.read_bytes())for p in inputs if p.is_file()};write('preservation-before.json',{'previously_committed_input_hashes':before,'previously_signed_original_source_state':original});write('preservation-after.json',{'previously_committed_input_hashes':after,'previously_signed_original_source_state':original_after});assert before==after and original==original_after
previous=root/'system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect';b=copy.deepcopy(json.loads((previous/'source-binding.json').read_text()));files=set(b['source_sha256'])|{p.relative_to(root).as_posix()for d in ('cmd','internal','pkg')for p in(root/d).rglob('*.go')if p.is_file()};b.update(scope='explicit_source_aware_policy_regeneration',previous_source_binding={'revision':'56c59c88','path':'system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect/source-binding.json'},source_sha256={n:sha((root/n).read_bytes())for n in sorted(files)});write('source-binding.json',b)
spec=importlib.util.spec_from_file_location('inventory',root/'scripts/check-knowledge-roadmap.py');inventory=importlib.util.module_from_spec(spec);spec.loader.exec_module(inventory)
v={'current_Go_validation_reused':False,'Go_count_flag':1,'maintained_Go_files':sum(n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))for n in files),'commands':{k:{'argv':r['command'],'exit_code':r['exit_code']}for k,r in commands.items()},'gates':{k:r['exit_code']for k,r in commands.items()}}
for label in('module','race'):v[label],_=inventory.test_inventory((gates/(label+'.jsonl')).read_bytes())
write('validation.json',v)
write('test-inventory.json',{label:inventory.test_rows((gates/(label+'.jsonl')).read_bytes())for label in ('module','race')})
summary_check=json.loads((actual/'summary.json').read_text());assert all(sha((root/'bin'/name).read_bytes())==digest for name,digest in summary_check['binary_sha256'].items())
profile='projects/stablenet/evaluation/policy-0bf2f4d1.profile.json';pr=json.loads((root/profile).read_text());summary=json.loads((actual/'summary.json').read_text());extra={'scripts/eval-source-policy-regeneration.py','scripts/check-source-policy-regeneration.py','scripts/test_source_policy_regeneration.py','scripts/knowledge_evidence.py','scripts/verify_committed_maintenance_checkpoint.py','scripts/test_knowledge_roadmap_current.py','docs/DOCUMENTATION-CATALOG.json','docs/spec-driven/POLICY-OVERLAY-ADOPTION.md','docs/spec-driven/POLICY-REFERENCE-DIAGNOSIS.md','docs/spec-driven/CURRENT-HANDOFF.md','docs/REMAINING-WORK.md','docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json','docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md'};files|=extra|{p.relative_to(root).as_posix()for p in(root/'docs/spec-driven/operational-preparation/policy-reference-diagnosis').rglob('*')if p.is_file()};put('capture-recipe.py',Path(__file__).read_bytes());m={'schema_version':1,'kind':'source_aware_policy_regeneration_not_independent_acceptance','profile':profile,'profile_sha256':sha((root/profile).read_bytes()),'runner':'scripts/eval-source-policy-regeneration.py','runner_sha256':sha((root/'scripts/eval-source-policy-regeneration.py').read_bytes()),'source_commit':pr['source_commit'],'owned_graph_sha256':summary['graph_sha256'],'production_activated':False,'independent_acceptance':'not_recorded','source_sha256':{n:sha((root/n).read_bytes())for n in sorted(files)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if p.is_file()and p.name!='manifest.json':
  data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
  if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
  m['artifacts'].append(row)
write('manifest.json',m);print(json.dumps({'source_bound_inputs':len(b['source_sha256']),'Go_files':v['maintained_Go_files'],'module':v['module']['test_cases'],'race':v['race']['test_cases'],'scope':'local source-name consistency only'}))
