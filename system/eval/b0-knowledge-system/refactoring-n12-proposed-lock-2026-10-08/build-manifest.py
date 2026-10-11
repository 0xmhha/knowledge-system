import hashlib,json
from pathlib import Path
P=Path(__file__).resolve().parent;root=P.parents[3]
old=json.loads((P.parent/'refactoring-n12-coordinated-preparation-2026-10-08/manifest.json').read_text())
sha=lambda f:hashlib.sha256(f.read_bytes()).hexdigest()
reports=set(old['reports_sha256'])|{'docs/spec-driven/N12-PROPOSED-LOCK-PREPARATION.md'}
for rel,digest in {**old['source_sha256'],**old['approved_input_sha256']}.items():assert sha(root/rel)==digest,rel
negative=json.loads((P/'negative-controls.json').read_text());assert len(negative)==7 and all(x['rejected']for x in negative)
replay=json.loads((P/'archive-originals-hidden.json').read_text());assert replay['exit_code']==0 and replay['original_source_absent']and replay['original_dataset_absent']and replay['original_semantic_store_absent']
report=json.loads((P/'retained-knowledge-report.json').read_text());assert report['retained_decisions']==2 and report['retained_policies']==0 and report['human_approved']==0 and report['document_sections_linked']==27 and report['code_refs']==36 and report['unique_canonical_ids']==33
m={'schema_version':1,'status':'N12_proposed_partial_ADR_lock_retained_source_CKG_CKV_v4_verified_no_human_verdict','source_sha256':old['source_sha256'],'approved_input_sha256':old['approved_input_sha256'],'changed_source_sha256':{},'reports_sha256':{f:sha(root/f)for f in sorted(reports)},'evidence_sha256':{f.relative_to(P).as_posix():sha(f)for f in sorted(P.rglob('*'))if f.is_file()and f.name!='manifest.json'},'proposed_partial_knowledge_lock_built':True,'approved_knowledge_lock_built':False,'technical_CKG_CKV_source_alignment_built':True,'approved_coordinated_dataset_complete':False,'proposed_ADR_instance_count':2,'policy_instance_count':0,'policy_instance_hold_reason':'mandatory operating owner not supplied; no placeholder assignment','actual_human_approved':0,'actual_human_active_review_ns':None,'new_query_or_FINAL_dispatch_count':0,'activation_count':0,'full_task_complete':False}
(P/'manifest.json').write_text(json.dumps(m,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'source_bindings':len(m['source_sha256']),'approved_inputs':len(m['approved_input_sha256']),'reports':len(m['reports_sha256']),'evidence_files':len(m['evidence_sha256']),'whole_tasks_completed_this_step':0}))
