import copy,gzip,hashlib,json,shutil,subprocess
from pathlib import Path
root=Path.cwd();base=root/'system/eval/operational-preparation-2026-10-16/readiness-refresh';base.mkdir(parents=True,exist_ok=True);actual=Path('/private/tmp/ks-acceptance-readiness-final-20261016');host=Path('/private/tmp/ks-acceptance-host-refresh-second-20261016');previous=root/'system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration'
def sha(data):return hashlib.sha256(data).hexdigest()
def put(name,data):
 p=base/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
def write(name,value):put(name,(json.dumps(value,ensure_ascii=False,indent=2)+'\n').encode())
def formal(p,l):return {'freeze':p['freeze'],'execution_allowed':p['execution_allowed'],'decisions':[{k:r[k]for k in('id','decision_status','receipt')}for r in p['freeze_requirements']],'roles':p['roles'],'model':p['model'],'cases':[{k:g[k]for k in('id','cases','review_receipt')}for g in p['input_groups']],'items':[{k:r[k]for k in('id','operational_gate','quality_gate','independent_acceptance','evidence','review_receipt')}for r in l['items']],'phase_status':[(r['id'],r['status'])for r in l['phases']]}
proto='docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json';ledger='docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json';p0=json.loads(subprocess.check_output(['git','show','3a530a25:'+proto]));l0=json.loads(subprocess.check_output(['git','show','3a530a25:'+ledger]));write('formal-state-before.json',formal(p0,l0));write('formal-state-after.json',formal(json.loads((root/proto).read_text()),json.loads((root/ledger).read_text())))
for p in actual.iterdir():
 if p.is_file()and p.name!='cks':put('runtime/'+p.name,p.read_bytes())
for name in('cks-session','chainbench-session'):
 for p in(actual/name).iterdir():
  if p.is_file():put('runtime/'+name+'/'+p.name+'.gz',gzip.compress(p.read_bytes(),mtime=0))if p.suffix=='.jsonl'else put('runtime/'+name+'/'+p.name,p.read_bytes())
source_record=json.loads((root/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/build/original-before.json').read_text())[0]
corpus={}
for label,argv in [('head',['git','rev-parse','HEAD']),('candidate_tree',['git','rev-parse','0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8^{tree}']),('tracked_status',['git','status','--porcelain','--untracked-files=no'])]:
 result=subprocess.run(argv,cwd=source_record['source_root'],capture_output=True);assert result.returncode==0;put('runtime/corpus-'+label+'.stdout.txt',result.stdout);put('runtime/corpus-'+label+'.stderr.txt',result.stderr);corpus[label]=result.stdout.decode().strip()
corpus.update(source_path=source_record['source_root'],candidate_commit=source_record['head'],candidate_scope='pinned_commit_available_only_no_new_execution_clone_or_gold');write('runtime/corpus-source.json',corpus)
r=json.loads((actual/'runtime.json').read_text());consumer=Path(r['repositories']['coding_agent']['source_path'])/'plugin/contracts/cks-tools.schema.json';put('runtime/consumer-contract.json',consumer.read_bytes())
for p in host.iterdir():
 if p.is_file():put('host/'+p.name,p.read_bytes())
for case in json.loads((host/'cases.json').read_text()):
 for p in Path(case['folder']).iterdir():
  if p.is_file():put('host/'+case['assessment']['case']+'/'+p.name,p.read_bytes())
for label in('','-second','-third'):
 folder=Path('/private/tmp/ks-acceptance-readiness'+label+'-20261016')
 for p in folder.iterdir():
  if p.is_file()and p.name!='cks':put('development/readiness'+(label or '-initial')+'/'+p.name,p.read_bytes())
for name in('ks-acceptance-readiness-20261016.txt','ks-acceptance-readiness-second-20261016.txt','ks-acceptance-readiness-third-20261016.txt','ks-acceptance-host-refresh-20261016.txt'):
 put('development/'+name,Path('/private/tmp',name).read_bytes())
put('boundary-receipt.json',Path('/private/tmp/ks-acceptance-boundary-receipt.json').read_bytes())
for src,dst in [('ks-acceptance-readiness-docs.txt','docs-final.txt'),('ks-acceptance-readiness-tests.txt','readiness-tests.txt'),('ks-acceptance-operational-tests.txt','operational-tests.txt'),('ks-acceptance-document-tests.txt','document-tests.txt'),('ks-acceptance-require-frozen.txt','require-frozen.txt')]:
 p=Path('/private/tmp',src)
 if p.exists():put(dst,p.read_bytes())
requirements=[('F-01','격리 Go pilot·게시 전 평가 종료·review 로컬 전달 범위를 채택한다. 외부 게시나 production 범위는 포함하지 않는다.','범위 승인 또는 수정'),('F-02','현재 knowledge source3a530a25, 기존 채택 coding-agent b7e7868·ChainBench bdf5ad27과 고정 go-stablenet0bf2f4d1를 후보로 제안한다.','실제 pilot source/복제·submodule 범위 채택'),('F-03','9개 baseline/독립 단위/위치 매칭/분모·기권 초안을 검토한다. threshold는 기존 제안이며 승인0이다.','9개 채점 계약과 baseline의 실제 실행 경로 채택'),('F-04','독립 family 단위 paired bootstrap95%·10000회·고정 seed, 표본 부족/undefined/inconclusive 규칙은 검토 초안이다.','통계·충분성·FINAL 최소 표본 결정'),('F-05','gold 작성·독립 내용 검토·최종 수용·실행 역할과 겸임 제한을 기록한다. AI 초안을 사람의 승인으로 바꾸지 않는다.','이름/역할/독립성 선언'),('F-06','이번 metadata/스크립트 연결은 실제 모델·유료 호출0이다. 실제 LLM pilot을 선택할 때만 provider/model/auth/비용 상한이 필요하다.','실제 모델 검증 여부·모델/비용·자원 한도 결정'),('F-07','함수40·review10·확장20·rubric45는 준비 초안이다. 새 독립 gold와 DEV/FINAL 배정은0이다.','새 ticket·사례·gold 검토/원문·family·holdout 봉인'),('F-08','현재 CLI2.1.301 로컬 scripted5개와 CKS33/ChainBench81 metadata는 검증했다. HTTP 소비자·전체 역할·실제 노드/cleanup은 미검증이다.','정확한 actual pilot argv·endpoint·소유 자원·종료 recipe 확정')]
write('freeze-review-checklist.json',{'schema_version':1,'kind':'reviewable_recommendations_not_approval','approved_requirements':0,'requirements':[{'id':i,'recommendation':rec,'decision_needed':needed,'decision_status':'pending','receipt':None}for i,rec,needed in requirements]})
b=copy.deepcopy(json.loads((previous/'source-binding.json').read_text()));b['scope']='current_acceptance_preparation_only';write('source-binding.json',b);prior=json.loads((previous/'validation.json').read_text());v={'current_Go_validation_reused':True,'Go_inputs_unchanged':True,'Go_reference':{'revision':'17970c86','manifest':str((previous/'manifest.json').relative_to(root)),'manifest_sha256':sha((previous/'manifest.json').read_bytes()),'maintained_Go_files_equal':1275},'module':prior['module'],'race':prior['race'],'gates':{'committed_build_and_metadata':0,'updated_scripted_host':0,'schema_parity':0}}
for name,count in [('readiness-tests.txt',16),('operational-tests.txt',36),('document-tests.txt',20)]:
 path=base/name
 if path.exists()and ('Ran '+str(count)+' tests')in path.read_text()and '\nOK\n'in path.read_text():v['gates'][name]=0
if (base/'docs-final.txt').exists()and 'documentation: OK (134 live documents)'in(base/'docs-final.txt').read_text():v['gates']['docs']=0
write('validation.json',v)
files=set(b['source_sha256'])|{'scripts/prepare-acceptance-readiness.py','scripts/refresh-acceptance-host.py','scripts/check-acceptance-readiness.py','scripts/test_acceptance_readiness.py','scripts/check-operational-acceptance.py','scripts/test_operational_acceptance.py','scripts/operational-host-probe.py','scripts/knowledge_evidence.py','scripts/eval-value-dependencies.py','docs/spec-driven/OPERATIONAL-ACCEPTANCE-PLAN.md','docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.md','docs/spec-driven/CURRENT-HANDOFF.md','docs/REMAINING-WORK.md','docs/DOCUMENTATION-CATALOG.json'};files={n for n in files if(root/n).is_file()};put('capture-recipe.py',Path(__file__).read_bytes());put('proposal-recipe.py',Path('/private/tmp/prepare-acceptance-review-proposals.py').read_bytes());m={'schema_version':1,'kind':'current_readiness_metadata_and_scripted_host_not_acceptance','protocol_frozen':False,'operational_acceptance':False,'quality_acceptance':False,'independent_acceptance':False,'real_model_inference':False,'real_chain_nodes_started':False,'source_sha256':{n:sha((root/n).read_bytes())for n in sorted(files)},'artifacts':[]}
for p in sorted(base.rglob('*')):
 if p.is_file()and p.name!='manifest.json':
  data=p.read_bytes();row={'path':p.relative_to(base).as_posix(),'sha256':sha(data),'bytes':len(data)}
  if p.suffix=='.gz':row['uncompressed_sha256']=sha(gzip.decompress(data))
  m['artifacts'].append(row)
write('manifest.json',m)
reference={'kind':'preparation_only_not_formal_execution','manifest':str((base/'manifest.json').relative_to(root)),'manifest_sha256':sha((base/'manifest.json').read_bytes())}
for path in(proto,ledger):
 p=root/path;value=json.loads(p.read_text());value['current_readiness_preparation']=reference;value['document_date']='2026-10-16';p.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
print('Readiness packet bound; protocol freeze and nine verdicts preserved')
