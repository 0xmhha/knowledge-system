"""Verify source-defined preregistration without promoting it to executed quality."""
import hashlib
import json
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/oe-01-protocol'


def check(folder=PACKET,root=ROOT):
    try:
        m,b=verify_packet(folder,root)
        require(m['checkpoint']=='OE01_1_expanded_function_retrieval_preregistration'and m['preregistration_complete']is True,'missing protocol')
        require(all(m[k]is False for k in('new_retrieval_executed','quality_acceptance','independent_acceptance','human_label_approval','full_OE01_complete')),'prepared inputs promoted to execution/gold')
        v=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,v,'dd554e79');require(all(c==0 for c in v['gates'].values()),'failed validation')
        p=load(folder,'inputs/protocol.json');require(p['stage']=='preregistered_no_new_query_execution'and p['question_count']==16 and p['retrieval_scored_questions']==14 and p['unscored_source_controls']==2 and p['fact_families']==5 and p['unique_function_units']==11 and p['approved_concepts']==4 and p['approved_implementation_links']==0,'scope/denominator changed')
        require(p['arms']==['off','concept_text','relations','combined']and p['counts']=={'warmup':1,'retrieval':2,'warm_latency':5,'cold_process':0}and p['planned_capture_calls']==sum(p['counts'].values())*p['question_count']*len(p['arms'])==512,'arm/call denominator changed')
        require(p['quality_acceptance']is False and p['independent_gold_or_holdout']is False and p['LLM_answer_quality']is None and p['paid_model_calls']==0 and p['default_ontology_mode_changed']is False,'quality/answer/default promoted')
        require(p['model']=={'name':'bge-m3:latest','digest':'7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab','dimension':1024}and p['API_instrumentation']['token_counts_inferred_from_bytes']is False,'model/cost scope changed')
        for name,key in(('requests.json','requests_sha256'),('labels.json','labels_sha256'),('private-function-criteria.json','criteria_sha256')):require(hashlib.sha256(raw(folder,'inputs/'+name)).hexdigest()==p[key],'preregistered input changed')
        public=load(folder,'inputs/requests.json');labels=load(folder,'inputs/labels.json');oracle=load(folder,'inputs/private-function-criteria.json');pins=load(folder,'inputs/source-pins.json')
        require(oracle['kind']=='preregistered_finite_function_retrieval_criteria_not_general_semantic_gold'and oracle['human_approval']is False and oracle['independent_quality_gold']is False and oracle['independent_units_approved']==0,'source criteria became human/general gold')
        ids=[q['id']for q in public['requests']];require(ids==[l['id']for l in labels]==['EX-'+str(i).zfill(2)for i in range(1,17)]and len(set(ids))==16,'question/label inventory differs')
        require(all(q['tool']=='cks.context.get_for_task_v2'and set(q['arguments'])=={'prompt','explain_ontology'}and q['arguments']['explain_ontology']is True for q in public['requests']),'oracle leaked or wrong tool')
        require(len(oracle['units'])==len({u['id']for u in oracle['units']})==11 and len(oracle['families'])==5 and sum(l['retrieval_scored']for l in labels)==14 and len({l['family']for l in labels if l['family']})==5,'fact/relevance denominator differs')
        units={u['id']:u for u in oracle['units']}
        for label in labels:
            if label['family']:
                family=oracle['families'][label['family']];require(label['expected_unit_ids']==family['unit_ids']and label['intended_ontology_concepts']==family['intended_ontology_concepts']and set(label['expected_unit_ids'])<=units.keys(),'family/expected source differs')
            else:require(not label['expected_unit_ids']and label['retrieval_scored']is False,'unknown control gained unreviewed source gold')
        manifest=load(folder,'source-manifest.json');source={(r['origin_id'],r['path']):r for r in manifest['files']}
        for u in units.values():
            row=source[('repo',u['file'])];data=raw(folder,'source-blobs/'+row['sha256']);span=b''.join(data.splitlines(keepends=True)[u['start_line']-1:u['end_line']]);require(hashlib.sha256(data).hexdigest()==row['sha256']==u['file_sha256']and hashlib.sha256(span).hexdigest()==u['span_sha256']and span.decode()==u['expected_code_body']and u['canonical_id'],'function source/span differs')
        require(pins['source_commit']=='baca2869118774c5f6219932dc76e197a47e6ecd'and pins['non_test_Go_equal_to_0bf2']==882,'source/corpus changed')
        require(load(folder,'new-query-execution.json')=={'new_retrieval_calls':0,'LLM_answer_calls':0,'paid_model_calls':0},'input preparation claims execution')
        require('documentation: OK (134 live documents)'in raw(folder,'docs-final.txt').decode(),'docs gate failed')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('efd43995')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'actual comparison evidence restore failed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='OE-01');require(progress['completed_subtasks']==25 and progress['implementation_completed']==6 and item['status']=='in_progress'and[s['status']for s in item['subtasks']]==['complete','in_progress','in_progress','not_started']and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'OE completion/ledger promoted')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration)as e:return[str(e)]
    return []

if __name__=='__main__':
    errors=check()
    for e in errors:print('FAIL:',e)
    print('ontology expanded protocol:','FAIL'if errors else'PASS; source-defined preregistration; new retrieval calls0')
    raise SystemExit(bool(errors))
