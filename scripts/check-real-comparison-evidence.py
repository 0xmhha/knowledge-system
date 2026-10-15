"""Check actual real-project comparison evidence, keeping ontology/quality separate."""
import hashlib
import json
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects'
CONCEPTS=['staking','staking-policy','consensus','finality','mempool-admission','transaction-ordering']
PINS={'go-stablenet':'0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8','go-wemix':'1350376a680489298b3203a44d9ebddb7747f9cc'}


def check(folder=PACKET,root=ROOT):
    try:
        m,b=verify_packet(folder,root)
        require(m['checkpoint']=='KM06_4_actual_pinned_project_comparison'and m['full_KM06_complete']is True and m['comparison_MCP_implemented']is True and m['actual_real_project_comparison']is True,'actual comparison missing')
        require(all(m[k]is False for k in('quality_acceptance','independent_acceptance','human_mapping_approval','ontology_effect_evaluated')),'quality/approval/effect promoted')
        v=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,v,'dd554e79');require(all(code==0 for code in v['gates'].values())and v['Go_reference']['maintained_Go_files_equal']==1273,'changed Go input or failed gate')
        summary=load(folder,'runtime/summary.json');require(summary['checks']==22 and summary['actual_tool_calls']==19 and summary['actual_comparison_MCP_sessions']==3 and summary['EOF_exit_codes']==[0,0,0]and summary['actual_native_project_datasets']==2 and summary['concept_queries']==CONCEPTS and summary['source_commits']==PINS and summary['paid_model_calls']==0 and summary['mapping_review']=='proposed_not_human_authenticated'and summary['live_network_policy_or_behavior_measured']is False and summary['semantic_quality_or_independent_acceptance']is False,'actual execution or scope differs')
        require(len(set(load(folder,'runtime/checks.json')))==22,'missing native checks')
        tools=load(folder,'runtime/tools.json');require({t['name']for t in tools}=={'cks.comparison.describe','cks.comparison.extract'}and all(t['annotations']=={'readOnlyHint':True,'destructiveHint':False,'idempotentHint':True,'openWorldHint':False}for t in tools),'wrong exposure/annotations')
        require(load(folder,'runtime/before.json')==load(folder,'runtime/after.json'),'DB/source/identity/current changed')
        require(load(folder,'build/original-before.json')==load(folder,'build/original-after.json')==load(folder,'runtime/original-after.json'),'original projects changed')
        fixtures=load(folder,'runtime/fixtures.json');require(len(fixtures)==2 and {f['project']:f['source_commit']for f in fixtures}==PINS,'wrong projects')
        for f in fixtures:
            label=f['project'];job=load(folder,'build/'+label+'-job.json');require(job['state']=='succeeded'and job['error']is None and job['activated']is True and all(job['gates'][g]=='passed'for g in('graph','vector','identity','source'))and job['candidate']['coordinates']==f['coordinates']and load(folder,'build/'+label+'-build-exit.json')['EOF_exit_code']==0,'real DB did not pass')
            manifest=load(folder,'archives/'+label+'/manifest.json');require(len(manifest['files'])==f['source_files']and f['source_files']>1000 and hashlib.sha256(raw(folder,'archives/'+label+'/manifest.json')).hexdigest()==f['source_manifest_sha256'],'source inventory differs')
            require(f['coordinates']['base_commit']==PINS[label],'original base commit lost')
            if label=='go-stablenet':require(f['coordinates']['source_mode']=='committed','committed scope relabeled')
            else:
                scope=load(folder,'build/go-wemix-source-scope.json');require(f['coordinates']['source_mode']==scope['source_mode']=='working-tree'and f['prepared_git_status']==scope['prepared_git_status']=='D cmd/gwemix'and scope['excluded_non_regular'][0]['path']=='cmd/gwemix'and scope['excluded_non_regular'][0]['target']=='geth','alias exclusion/capture mode hidden')
        equal=load(folder,'runtime/source-equality.json');require(len(equal)==7 and all(row['equal']is True and row['original_sha256']==row['retained_sha256']and row['source_commit']in PINS.values()for row in equal.values()),'compared code not original pinned bytes')
        calls=load(folder,'runtime/calls.json');require(len(calls)==19 and sum(bool(c['expected_error'])for c in calls)==4,'call denominator changed')
        wire=[]
        for session in('bootstrap-session','comparison-session','bad-mapping-session'):
            wire.extend(json.loads(line)for line in raw(folder,'runtime/'+session+'/mcp.stdout.jsonl').splitlines())
        responses=[r['result']for r in wire if 'result'in r]
        require(all(c['response']in responses for c in calls),'missing native response')
        for c in calls:
            if c['expected_error']:require(c['response'].get('isError')and c['response']['structuredContent']['code']==c['expected_error'],'wrong refusal')
        matrices={key:load(folder,'runtime/matrix-'+key+'.json')for key in CONCEPTS};cells=[]
        for key,result in matrices.items():
            require(result in[c['response'].get('structuredContent')for c in calls]and result['mode']=='comparison'and len(result['groups'])==2,'matrix not actual comparison')
            body_count,citation_count=0,0
            for group,f in zip(result['groups'],fixtures,strict=True):
                require(group['participant']['scope']['coordinates']==f['coordinates'],'project group mixed')
                manifest=load(folder,'archives/'+f['project']+'/manifest.json')
                for cell in group['cells']:
                    cells.append(cell);require(cell['entity_identity']is False,'entity identity invented')
                    for mapping in cell['mappings']:
                        require(mapping['binding']['review_state']=='proposed'and mapping['review']['reviewer_identity_authenticated']is False and mapping['review']['independent_quality_acceptance']is False,'review/quality invented')
                        for e in mapping['binding']['evidence']:
                            c=e['reference']['citation'];require(all(c[k]==value for k,value in f['coordinates'].items())and c['commit_hash']==(f['source_commit']if f['coordinates']['source_mode']=='committed'else''),'foreign citation or mode')
                            record=next(row for row in manifest['files']if row['path']==c['file']and row['origin_id']==c['origin_id']);source=raw(folder,'archives/'+f['project']+'/blobs/'+record['sha256']);span=b''.join(source.splitlines(keepends=True)[c['start_line']-1:c['end_line']])
                            require(hashlib.sha256(source).hexdigest()==record['sha256']==c['file_sha256']and hashlib.sha256(span).hexdigest()==c['content_sha256']and span.decode()==e['body'],'retained source/hash/body differs');body_count+=len(e['body'].encode());citation_count+=1
            for candidate in result['candidates']:
                require(len(candidate['participants'])==2 and {e['scope_id']for e in candidate['evidence']}==set(candidate['participants'])and all(e['citations']for e in candidate['evidence']),'one-sided comparison candidate')
                citation_count+=sum(len(e['citations'])for e in candidate['evidence'])
            require(result['used_body_bytes']==body_count and result['used_citations']==citation_count<=128,'response budget counters differ')
        require(len(cells)==40 and sum(c['state']=='similar'for c in cells)==7 and sum(c['state']=='unknown'for c in cells)==18 and sum(c['state']=='not_collected'for c in cells)==15,'coverage/missing state changed')
        require(all(c['state']=='not_collected'and not c['mappings']for key in('staking','staking-policy')for g in matrices[key]['groups']for c in g['cells']),'staking existence/absence invented')
        finality=matrices['finality'];require(finality['groups'][0]['cells'][0]['state']=='unknown'and len(finality['groups'][0]['cells'][0]['mappings'])==1 and not finality['candidates'],'finality invented from commit')
        default=load(folder,'runtime/default-reviewed-only.json');require(all(c['state']=='unknown'and not c['mappings']for g in default['groups']for c in g['cells']),'proposed silently adopted')
        require(load(folder,'runtime/after-live-delete.json')==matrices['mempool-admission'],'live source fallback')
        missing=load(folder,'runtime/missing-B.json');require(missing['groups'][0]==matrices['mempool-admission']['groups'][0]and not missing['candidates']and all(c['state']=='unavailable'and not c['mappings']for c in missing['groups'][1]['cells']),'failed side leaked/overwrote good side')
        bad=load(folder,'runtime/bad-mapping.json');require(bad['groups'][0]['cells'][0]['state']=='unavailable'and not bad['groups'][0]['cells'][0]['mappings']and bad['groups'][1]==matrices['consensus']['groups'][1]and not bad['candidates'],'mispinned mapping escaped')
        budget=load(folder,'runtime/candidate-budget.json');require(budget['used_citations']==2 and budget['omitted_candidates']==1 and budget['candidate_state']=='budget_exceeded'and not budget['candidates'],'candidate budget lost')
        require('documentation: OK (134 live documents)'in raw(folder,'docs-final.txt').decode(),'docs gate failed')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('dd554e79')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'previous protections not verified')
        p=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in p['items']if i['id']=='KM-06');require(p['completed_subtasks']==24 and p['implementation_completed']==6 and item['status']=='complete'and all(s['status']=='complete'for s in item['subtasks'])and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'ledger inconsistent/quality promoted')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration)as error:return[str(error)]
    return []

if __name__=='__main__':
    errors=check()
    for e in errors:print('FAIL:',e)
    print('real project comparison:','FAIL'if errors else'PASS; actual source/MCP; ontology effect and independent quality remain')
    raise SystemExit(bool(errors))
