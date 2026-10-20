"""Recompute mapping coverage from bounded native wire/source evidence."""
import hashlib
import json
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect'
ARMS=('without-mapping','with-mapping')
CONCEPTS=('staking','staking-policy','consensus','finality','mempool-admission','transaction-ordering')
def sha(data):return hashlib.sha256(data).hexdigest()

def check(folder=PACKET,root=ROOT):
    try:
        m,b=verify_packet(folder,root)
        require(m['checkpoint']=='OE01_4_explicit_mapping_input_effect'and m['full_OE01_complete']is True,'mapping effect execution missing')
        require(all(m[k]is False for k in('quality_acceptance','independent_acceptance','human_mapping_approval','LLM_answer_quality_evaluated','ontology_vocabulary_alone_effect_isolated')),'scope promoted')
        v=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,v,'dd554e79');require(all(c==0 for c in v['gates'].values()),'failed gate')
        protocol=load(folder,'runtime/protocol.json');summary=load(folder,'runtime/summary.json');queries=load(folder,'inputs/queries.json');fixtures=load(folder,'inputs/fixtures.json')
        require(protocol['stage']=='registered_before_calls'and protocol['concepts']==list(CONCEPTS)and protocol['unique_project_facet_cells_per_arm']==40 and protocol['repetitions']==2 and protocol['planned_matrix_calls']==24 and len(protocol['limitations'])==5,'protocol scope changed')
        require(protocol['human_mapping_approval']is False and protocol['independent_quality_gold']is False and protocol['default_mode_changed']is False,'approval invented')
        profiles={a:load(folder,'runtime/'+a+'.json')for a in ARMS};treatment=dict(profiles[ARMS[1]]);treatment.pop('bindings_path');require(treatment==profiles[ARMS[0]],'arm settings differ beyond bindings')
        for arm,key in zip(ARMS,('profile_without_binding_sha256','profile_with_binding_sha256')):require(sha(raw(folder,'runtime/'+arm+'.json'))==protocol[key],'profile pin differs')
        require(sha(raw(folder,'inputs/bindings.json'))==protocol['frozen_binding_sha256']and sha(raw(folder,'inputs/queries.json'))==protocol['queries_sha256'],'frozen input differs')
        require(len(queries)==6 and len(fixtures)==2 and load(folder,'runtime/before.json')==load(folder,'runtime/after.json'),'source or denominator changed')
        checks=load(folder,'runtime/checks.json');require(len(checks)==len(set(checks))==18,'missing checks')
        calls=load(folder,'runtime/calls.json');require(len(calls)==27 and all(not c['response'].get('isError')for c in calls),'missing/failed tools')
        # Each call must match its own session's exact request and response pair,
        # with no duplicates or unused tool calls hidden in a combined transcript.
        for arm in ARMS:
            requests=[json.loads(line)for line in raw(folder,'runtime/'+arm+'/mcp.stdin.jsonl').splitlines()];responses=[json.loads(line)for line in raw(folder,'runtime/'+arm+'/mcp.stdout.jsonl').splitlines()]
            by_id={r['id']:r for r in responses if 'id'in r};wire_calls=[]
            require(len(by_id)==len(responses),'duplicate/missing response IDs')
            for req in requests:
                require(req['id']in by_id and 'error'not in by_id[req['id']],'unanswered native request')
                if req['method']=='tools/call':wire_calls.append({'session':arm,'tool':req['params']['name'],'arguments':req['params']['arguments'],'response':by_id[req['id']]['result']})
            require(wire_calls==[c for c in calls if c['session']==arm],'native transcript differs from call records')
        descriptions=load(folder,'runtime/descriptions.json');require(descriptions[ARMS[0]]['catalog_digest']==descriptions[ARMS[1]]['catalog_digest']and descriptions[ARMS[0]]['scopes']==descriptions[ARMS[1]]['scopes'],'catalog or scopes differ')
        results={}
        for arm in ARMS:
            cells=[];source_items=[];candidates=[];group_count=0
            for concept,q in queries.items():
                values=[load(folder,'runtime/'+arm+'-'+concept+'-'+str(rep)+'.json')for rep in(1,2)];require(values[0]==values[1],'repeat changed')
                args=dict(q,profile_digest=descriptions[arm]['profile_digest'],catalog_digest=descriptions[arm]['catalog_digest'])
                matched=[c for c in calls if c['session']==arm and c['tool']=='cks.comparison.extract'and c['arguments']==args]
                require(len(matched)==2 and all(c['response']['structuredContent']==values[0]for c in matched),'matrix/argument not captured twice')
                value=values[0];require(value['mode']=='comparison'and len(value['groups'])==2 and args['include_proposed']is True,'wrong comparison mode')
                cell_citations={}
                for group,f,sid in zip(value['groups'],fixtures,q['participants'],strict=True):
                    group_count+=1;coords=f['coordinates'];require(group['participant']['scope']['coordinates']==coords and group['participant']['scope_id']==sid and [c['facet']for c in group['cells']]==q['facets'],'project group/axis mixed')
                    cell_citations[sid]=[];manifest=load(folder,'archives/'+f['project']+'/manifest.json')
                    for cell in group['cells']:
                        cells.append(cell);require(cell['entity_identity']is False,'identity equivalence invented')
                        for mapping in cell['mappings']:
                            require(mapping['binding']['review_state']=='proposed'and mapping['review']['reviewer_identity_authenticated']is False and mapping['review']['independent_quality_acceptance']is False,'human review invented')
                            for item in mapping['binding']['evidence']:
                                c=item['reference']['citation'];require(all(c[k]==val for k,val in coords.items())and c['commit_hash']==(f['source_commit']if coords['source_mode']=='committed'else''),'foreign source coordinate/mode')
                                record=next(r for r in manifest['files']if r['path']==c['file']and r['origin_id']==c['origin_id']);data=raw(folder,'archives/'+f['project']+'/blobs/'+c['file_sha256']);span=b''.join(data.splitlines(keepends=True)[c['start_line']-1:c['end_line']]);require(sha(data)==record['sha256']==c['file_sha256']and sha(span)==c['content_sha256']and span.decode()==item['body'],'source hash/body differs');source_items.append(c);cell_citations[sid].append(c)
                for candidate in value['candidates']:
                    require(len(candidate['participants'])==2 and set(candidate['participants'])==set(q['participants'])and {e['scope_id']for e in candidate['evidence']}==set(q['participants']),'one-sided candidate')
                    require(all(e['citations']and all(c in cell_citations[e['scope_id']]for c in e['citations'])for e in candidate['evidence']),'candidate uses foreign/uncited source')
                    candidates.append(candidate)
            source_cells=sum(bool(c['mappings'])for c in cells);known=sum(c['state']in('exact','similar','not_applicable')for c in cells)
            results[arm]={'unique_cells':len(cells),'cells_with_source_evidence':source_cells,'source_evidence_cell_coverage':source_cells/len(cells),'declared_exact_similar_not_applicable_cells':known,'declared_mapping_coverage':known/len(cells),'unknown_cells':sum(c['state']=='unknown'for c in cells),'not_collected_cells':sum(c['state']=='not_collected'for c in cells),'cell_source_citations':len(source_items),'source_citations_verified':len(source_items),'wrong_project_citations':0,'project_group_scopes_checked':group_count,'project_group_errors':0,'two_sided_candidates':len(candidates),'source_accuracy':1.0 if source_items else None,'missing_evidence_is_not_feature_absence':True}
        require(summary['arms']==results,'derived coverage/accuracy differs')
        require(results[ARMS[0]]['not_collected_cells']==40 and results[ARMS[0]]['source_accuracy']is None and results[ARMS[1]]['cells_with_source_evidence']==8 and results[ARMS[1]]['declared_exact_similar_not_applicable_cells']==7 and results[ARMS[1]]['unknown_cells']==18 and results[ARMS[1]]['not_collected_cells']==15 and results[ARMS[1]]['two_sided_candidates']==3,'bounded effect changed')
        require(summary['checks']==18 and summary['actual_matrix_calls']==24 and summary['actual_tool_calls']==27 and summary['MCP_sessions']==2 and summary['EOF_exit_codes']==[0,0]and summary['paid_model_calls']==summary['LLM_answer_calls']==0 and summary['source_corpus_changed']is False and summary['ontology_vocabulary_alone_effect']=='not_isolated_from_annotation_input'and summary['human_mapping_approval']is False and summary['independent_quality_acceptance']is False,'execution/scope changed')
        hidden=load(folder,'runtime/default-reviewed-only.json');require(all(not c['mappings']for g in hidden['groups']for c in g['cells'])and any(c['arguments'].get('include_proposed')is False and c['response']['structuredContent']==hidden for c in calls),'default proposed mappings exposed')
        require('documentation: OK (134 live documents)'in raw(folder,'docs-final.txt').decode(),'docs failed')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('c0c77897')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'prior signed score checkpoint failed')
        p=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());oe=next(i for i in p['items']if i['id']=='OE-01');require(p['completed_subtasks']==28 and p['implementation_completed']==7 and oe['status']=='complete'and all(s['status']=='complete'for s in oe['subtasks'])and oe['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'completion ledger differs')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,AssertionError)as e:return[str(e)]
    return []
if __name__=='__main__':
    errors=check()
    for e in errors:print('FAIL:',e)
    print('multi-project mapping effect:','FAIL'if errors else'PASS; raw/source coverage recomputed; proposed-only, quality inconclusive')
    raise SystemExit(bool(errors))
