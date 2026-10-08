#!/usr/bin/env python3
"""Prepare a private, proposed-only two-ADR lock; never supply unknown owners."""
import hashlib,json,shutil,subprocess,tempfile
from pathlib import Path
P=Path(__file__).resolve().parent
OLD=P.parent/'refactoring-n12-coordinated-preparation-2026-10-08'
base=Path(tempfile.mkdtemp(prefix='ks-n12-proposed-lock-',dir='/private/tmp'))
source=base/'source'
shutil.copytree('/private/tmp/ks-n12-scoped-capture-iuu2zemk/archive-only-source',source)
shutil.copytree('/private/tmp/ks-n12-coordinated-6j88bw75/bin',base/'bin')
def save(path,obj):path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(obj,ensure_ascii=False,indent=2)+'\n')
def sha(data):return hashlib.sha256(data).hexdigest()
def cli(name,args,expect=0):
 r=subprocess.run([str(base/'bin/cks')]+args,capture_output=True)
 (P/(name+'.stdout.txt')).write_bytes(r.stdout);(P/(name+'.stderr.txt')).write_bytes(r.stderr)
 assert r.returncode==expect,(name,r.returncode,r.stderr.decode())
 return r
before={str(f.relative_to(source)):sha(f.read_bytes())for f in source.rglob('*')if f.is_file()and'.git'not in f.relative_to(source).parts}
assert len(before)==1410
cli('init',['knowledge','--project-root',str(source),'init','--project-id','knowledge-system'])
pack={'pack_schema_version':1,'pack_id':'local.n12','version':'0.1.0','owner':'Codex preparation tooling; no human authority or operating owner assigned','scope':'N12 proposed-only technical preparation; source contract applicability awaits human review','requires':[], 'concepts':[{'id':'source-adr','kind':'artifact','definition':'Proposed source-snapshot ADR extract. A type registration and lock are not human approval.'}], 'relation_types':[], 'constraints':[{'id':'no-inferred-approval','description':'No inferred reviewer, operating owner, effective date, requirement trace, or approved relationship.'}], 'competency_questions':['Which original source contract supports each proposed source-snapshot ADR extract?']}
packpath=source/'packs/n12-proposed/pack.yaml';save(packpath,pack)
r=cli('pack-digest',['knowledge','--project-root',str(source),'digest','--source','packs/n12-proposed'])
dig=json.loads(r.stdout)['sha256']
manifest={'schema_version':1,'project_id':'knowledge-system','selected_packs':[{'pack_id':'local.n12','version':'0.1.0','source':'packs/n12-proposed','sha256':dig}], 'overlay_root':'.cks/knowledge','review_policy':{'min_approvals':1}}
save(source/'.cks/knowledge/manifest.yaml',manifest)
seeds=json.loads((P.parent/'refactoring-n12-policy-adr-review-2026-10-06/claim-seeds.json').read_text())
adrbytes=(source/'docs/spec-driven/SOURCE-SNAPSHOT-ADR.md').read_bytes();lines=adrbytes.splitlines(keepends=True)
provenance=[]
for seed in seeds[-2:]:
 id=seed['id'];rel='.cks/knowledge/decisions/'+id+'.md'
 if id=='P12-P05':
  problem='공유 Git worktree는 개발 저장소의 이후 이력을 읽을 수 있다.'
  rationale='원본 저장소가 삭제되거나 GC된 뒤에도 보관본만으로 같은 입력을 재생해야 한다.'
  alternatives=['초기 ADR의 임시 Git worktree + 오버레이: 이후 이력 격리 계약에 의해 보강됨.']
  scope_first,scope_last=61,65
 else:
  problem='파일 바이트를 다시 쓰자 Git checkout의 실행 비트가 사라져 승격 게이트가 거부했다.'
  rationale='같은 바이트의 실행 비트 변경도 다른 신원으로 만들고 과거 v2–v4를 재해석하지 않아야 한다.'
  alternatives=['내용 바이트만의 구 v2–v4 신원: 과거 보관본 읽기에 한정하며 신규 실행 비트 신원을 대신하지 않음.']
  scope_first,scope_last=67,69
 front={'id':id,'type':{'pack_id':'local.n12','local_id':'source-adr'},'problem':problem,'decision':seed['statement'],'rationale':rationale,'alternatives':alternatives,'assumptions':['위 메타데이터는 원문에서 작성한 proposed 추출이며 내용·권위·대안 해석은 사람 검토 대기.'],'scope':{'project':'knowledge-system','contract':'SOURCE-SNAPSHOT-ADR','stage':'proposed-review'},'date':'','status':'proposed','reviewed_by':'','review_reason':'','visibility':'public','supersedes':'','requirement_ids':[], 'source_ref':{'origin_id':'repo','path':rel}}
 span=b''.join(lines[scope_first-1:scope_last])
 body='N12 검토 대기 ADR 추출. 작성일/유효일·검토자·운영 담당자 승인은 미기록.\n\n원본667a9af5 / source562e37aa, docs/spec-driven/SOURCE-SNAPSHOT-ADR.md:'+str(scope_first)+'–'+str(scope_last)+'\n원문 file SHA256 '+sha(adrbytes)+' / span SHA256 '+sha(span)+'\n\n'+span.decode()
 path=source/rel;path.write_text('---\n'+json.dumps(front,ensure_ascii=False,indent=2)+'\n---\n'+body)
 provenance.append({'id':id,'overlay_path':rel,'overlay_sha256':sha(path.read_bytes()),'original_path':seed['path'],'original_first':scope_first,'original_last':scope_last,'original_file_sha256':sha(adrbytes),'original_span_sha256':sha(span),'status':'proposed','date':None,'reviewer':None,'metadata_human_interpretation_approved':False})
missing=[{'id':s['id'],'statement':s['statement'],'policy_instance_created':False,'reason':'mandatory operating owner not supplied; no owner placeholder created','owner':None,'effective_date':None}for s in seeds[:4]]
save(source/'.cks/knowledge/domain/n12-provenance.json',{'schema_version':1,'original_source_revision':'667a9af5f8608e6e984f58f3c3b61350dde0ea48','source_base_commit':'562e37aa1c9680d283de85ccd6435e5f235c21d6','proposed_decisions':provenance,'missing_policies':missing,'human_approved':0})
# Real schema negative control: ownerless original policy is refused, then removed.
policy={'id':'P12-P01','type':{'pack_id':'local.n12','local_id':'source-adr'},'statement':seeds[0]['statement'],'owner':'','scope':{'project':'knowledge-system'},'status':'proposed','visibility':'public','source_ref':{'origin_id':'repo','path':'.cks/knowledge/policies/P12-P01.yaml'}}
negative=source/'.cks/knowledge/policies/P12-P01.yaml';save(negative,policy)
cli('ownerless-policy-negative',['knowledge','--project-root',str(source),'validate'],1);negative.unlink()
cli('validate-before-lock',['knowledge','--project-root',str(source),'validate'])
cli('lock',['knowledge','--project-root',str(source),'lock'])
cli('validate-after-lock',['knowledge','--project-root',str(source),'validate'])
cli('review-before-capture',['knowledge','--project-root',str(source),'review'])
# Private derived commit; original 1410 source files unchanged. No original full history copied.
for rel,d in before.items():assert sha((source/rel).read_bytes())==d
subprocess.run(['git','add','--force','.cks/knowledge','packs/n12-proposed'],cwd=source,check=True,capture_output=True)
subprocess.run(['git','-c','user.name=N12 preparation tooling','-c','user.email=n12-preparation@localhost','-c','commit.gpgsign=false','commit','-m','Prepare unreviewed N12 source ADR lock'],cwd=source,check=True,capture_output=True)
commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=source,text=True).strip()
assert not subprocess.check_output(['git','status','--porcelain'],cwd=source)
prep=json.loads((OLD/'preparation.json').read_text());prep.update({'base':str(base),'source':str(source),'dataset_root':str(base/'dataset'),'runtime_revision':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'source_commit':commit,'capture_files_original_repo':1410,'knowledge_lock_built':True,'approved_knowledge_lock_built':False,'proposed_decision_count':2,'policy_count':0,'relation_count':0})
prep.pop('initial_project_id',None);prep.pop('initial_dataset_root',None)
save(P/'preparation.json',prep);save(P/'overlay-provenance.json',{'proposed_decisions':provenance,'missing_policies':missing,'original1410_unchanged':True,'original_source_sha256':before})
shutil.copytree(source/'.cks/knowledge',P/'input-overlay');shutil.copytree(source/'packs/n12-proposed',P/'input-pack')
for name in ['filelist.yaml','build-sources.json']:shutil.copyfile(OLD/name,P/name)
runner=(OLD/'run-held-setup.py').read_text().replace('refactoring-n12-coordinated-preparation-2026-10-08','refactoring-n12-proposed-lock-2026-10-08').replace('ks-n12-coordinated-','ks-n12-proposed-lock-')
(P/'run-held-setup.py').write_text(runner)
print(json.dumps({'base':str(base),'source_commit':commit,'proposed_decisions':2,'policies':0,'approved':0,'original_source_unchanged':True}))
