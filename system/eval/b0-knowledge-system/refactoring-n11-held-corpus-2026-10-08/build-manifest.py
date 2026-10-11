import hashlib,json
from pathlib import Path
P=Path(__file__).resolve().parent;ROOT=P.parents[3];old=json.loads((P.parent/'refactoring-n12-proposed-lock-2026-10-08/manifest.json').read_text());sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
changed=['scripts/refactoring-eval-input-check.py','scripts/test_refactoring_eval_input_check.py','scripts/test_refactoring_eval_run.py']
sources=dict(old['source_sha256'])
for p,d in sources.items():
 if p in changed:sources[p]=sha(ROOT/p)
 else:assert sha(ROOT/p)==d,p
for p,d in old['approved_input_sha256'].items():assert sha(ROOT/p)==d,p
reports=set(old['reports_sha256'])|{'docs/spec-driven/N11-HELD-CORPUS-PREPARATION.md','docs/spec-driven/N11-COVERAGE-CANDIDATE-REVIEW.md','docs/spec-driven/N11-EVALUATION-RUNNER-CONTRACT.md','docs/spec-driven/N11-FRESH-EVALUATION-PREPARATION.md'}
m={'schema_version':1,'status':'N11_held_corpus_and_declared_absence_source_guard_verified_no_human_input_acceptance','source_sha256':sources,'approved_input_sha256':old['approved_input_sha256'],'changed_source_sha256':{p:sha(ROOT/p)for p in changed},'reports_sha256':{p:sha(ROOT/p)for p in sorted(reports)},'evidence_sha256':{p.relative_to(P).as_posix():sha(p)for p in sorted(P.rglob('*'))if p.is_file()and p.name!='manifest.json'},'actual_human_approved':0,'actual_human_review_time_ns':None,'approved_independent_FINAL_count':0,'technical_corpus_CKG_CKV_binding_prepared':True,'input_scope_human_approved':False,'new_query_or_FINAL_dispatch_count':0,'activation_count':0,'full_task_complete':False}
(P/'manifest.json').write_text(json.dumps(m,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'sources':len(sources),'approved_inputs':5,'changed_Python_files':3,'reports':len(m['reports_sha256']),'evidence':len(m['evidence_sha256']),'full_task_complete':False}))
