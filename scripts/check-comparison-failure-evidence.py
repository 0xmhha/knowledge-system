"""Verify comparison partial failure and shutdown evidence without promoting quality."""
import hashlib
import json
import re
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,current_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-06-failures'


def check(folder=PACKET,root=ROOT):
    try:
        m,b=verify_packet(folder,root)
        require(m['checkpoint']=='KM06_3_comparison_partial_failure_and_shutdown'and m['comparison_MCP_implemented']is True and m['full_KM05_complete']is True,'runtime scope missing')
        require(all(m[k]is False for k in('full_KM06_complete','quality_acceptance','independent_acceptance','human_mapping_approval','actual_real_project_comparison')),'scope promoted')
        v=load(folder,'validation.json');roadmap=current_go(root,folder,v,b,race_packages=7)
        require(all(code==0 for code in v['gates'].values())and v['maintained_Go_files']==1273,'gate/source inventory changed')
        top=set(re.findall(r'^func (Test\w+)\(', '\n'.join(p.read_text()for p in(root/'internal/system/comparison').glob('*_test.go')),re.M))
        for label in('module','race'):
            rows=[json.loads(line)for line in raw(folder,label+'.jsonl').splitlines()]
            cases=[r for r in rows if r.get('Action')=='pass'and r.get('Test')and r.get('Package','').endswith('/internal/system/comparison')]
            require(len(cases)==192 and len(top)==47 and {r['Test']for r in cases if '/'not in r['Test']}==top,'comparison denominator differs')
        summary=load(folder,'runtime/summary.json');require(summary['checks']==9 and summary['baseline_checks']==12 and summary['actual_tool_calls']==summary['baseline_actual_tool_calls']==9 and summary['actual_MCP_sessions']==4 and summary['EOF_exit_codes']==[0,0,0]and summary['SIGTERM_exit_code']==0 and summary['actual_native_dataset_builds']==2 and summary['paid_model_calls']==0 and summary['semantic_quality_or_independent_acceptance']is False,'native failure/exit/scope differs')
        checks=load(folder,'runtime/checks.json');require(len(checks)==len(set(checks))==9,'native checks missing')
        require(load(folder,'runtime/before.json')==load(folder,'runtime/after.json'),'DB/archive/config not restored exactly')
        signal=load(folder,'runtime/signal.json');require(signal['input_open_before_SIGTERM']is True and signal['exit_code']==0 and 0<=signal['elapsed_seconds']<10,'signal/input/exit proof differs')
        tools=load(folder,'runtime/baseline/tools.json');require({t['name']for t in tools}=={'cks.comparison.describe','cks.comparison.extract'}and all(t['annotations']=={'readOnlyHint':True,'destructiveHint':False,'idempotentHint':True,'openWorldHint':False}for t in tools),'tools/annotations differ')
        baseline=load(folder,'runtime/baseline/calls.json');calls=load(folder,'runtime/calls.json');require(len(calls)==len(baseline)==9 and sum(bool(c['expected_error'])for c in baseline)==4,'wire denominator differs')
        wire=[]
        for session in('baseline/bootstrap-session','baseline/matrix-session','failure-session','signal-session'):
            wire.extend(json.loads(line)for line in raw(folder,'runtime/'+session+'/mcp.stdout.jsonl').splitlines())
        responses=[r['result']for r in wire if 'result'in r]
        for call in baseline+calls:
            require(call['response']in responses,'missing actual native response')
            if call.get('expected_error'):require(call['response'].get('isError')and call['response']['structuredContent']['code']==call['expected_error'],'expected refusal missing')
        result=load(folder,'runtime/baseline/matrix.json');require(result in [c['response'].get('structuredContent')for c in baseline],'matrix not actual MCP response')
        fixtures=load(folder,'runtime/baseline/fixtures.json');require(len(result['groups'])==len(fixtures)==2 and result['state']=='partial','groups missing')
        body_count=0;citation_count=0
        for group,f in zip(result['groups'],fixtures,strict=True):
            coords=f['coordinates'];require(group['participant']['scope']['coordinates']==coords and group['cells'][0]['state']=='similar'and group['cells'][1]['state']=='unknown','foreign/missing project/facet')
            manifest=load(folder,'archives/'+f['label']+'/manifest.json')
            for entry in group['cells'][0]['mappings']:
                binding=entry['binding'];require(binding['review_state']=='proposed'and binding['coordinates']==coords and entry['review']['reviewer_identity_authenticated']is False and entry['review']['independent_quality_acceptance']is False,'review/scope invented')
                for e in binding['evidence']:
                    c=e['reference']['citation'];require(all(c[k]==value for k,value in coords.items()),'foreign citation')
                    record=next(r for r in manifest['files']if r['path']==c['file']and r['origin_id']==c['origin_id'])
                    source=raw(folder,'archives/'+f['label']+'/blobs/'+record['sha256']);span=b''.join(source.splitlines(keepends=True)[c['start_line']-1:c['end_line']])
                    require(hashlib.sha256(source).hexdigest()==record['sha256']==c['file_sha256']and hashlib.sha256(span).hexdigest()==c['content_sha256']and span.decode()==e['body'],'source/span/body differs')
                    body_count+=len(e['body'].encode());citation_count+=1
        citation_count+=sum(len(p['citations'])for c in result['candidates']for p in c['evidence'])
        require(result['used_body_bytes']==body_count and result['used_citations']==citation_count==8,'budget denominator differs')
        require(result['candidates']and all(len(c['participants'])==len(c['evidence'])==2 and all(p['citations']for p in c['evidence'])and len(c['reasons'])==2 for c in result['candidates']),'two-sided candidates missing')
        default=load(folder,'runtime/baseline/default-reviewed-only.json');require(all(c['state']=='unknown'and not c['mappings']for g in default['groups']for c in g['cells']),'unreviewed data silently included')
        budget=load(folder,'runtime/baseline/budget.json');require(budget['used_body_bytes']==budget['used_citations']==0 and all(g['cells'][0]['state']=='budget_exceeded'and not g['cells'][0]['mappings']for g in budget['groups']),'truncated budget evidence')
        original=load(folder,'runtime/before-matrix.json');require(load(folder,'runtime/restored-B.json')==load(folder,'runtime/restored-A.json')==original,'restoration did not restore source results')
        missing=load(folder,'runtime/missing-B.json');corrupt=load(folder,'runtime/corrupt-A.json')
        require(missing['state']==corrupt['state']=='partial'and missing['groups'][0]==original['groups'][0]and corrupt['groups'][1]==original['groups'][1]and not missing['candidates']and not corrupt['candidates'],'unaffected project changed or missing side candidate escaped')
        for bad,index in((missing,1),(corrupt,0)):
            require(bad['groups'][index]['participant']==original['groups'][index]['participant']and bad['used_citations']==2 and all(c['state']=='unavailable'and not c['mappings']for c in bad['groups'][index]['cells']),'unavailable evidence or fallback escaped')
        candidate=load(folder,'runtime/candidate-budget.json');cell=load(folder,'runtime/cell-budget.json')
        require(candidate['used_citations']==4 and candidate['omitted_candidates']==1 and candidate['candidate_state']=='budget_exceeded'and not candidate['candidates']and cell['used_citations']==2 and cell['groups'][1]['cells'][0]['state']=='budget_exceeded'and not cell['groups'][1]['cells'][0]['mappings'],'candidate/cell budget lost')
        require('0 issues.'in raw(folder,'build-lint.txt').decode()and 'documentation: OK (134 live documents)'in raw(folder,'docs-final.txt').decode(),'lint/docs failure')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('5f721674')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'prior contract failed')
        p=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in p['items']if i['id']=='KM-06')
        require(p['completed_subtasks']==23 and p['implementation_completed']==5 and item['status']=='in_progress'and[s['status']for s in item['subtasks']]==['complete','complete','complete','not_started']and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'ledger inconsistent/promoted')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration)as error:return[str(error)]
    return []

if __name__=='__main__':
    errors=check()
    for e in errors:print('FAIL:',e)
    print('comparison partial failure:','FAIL'if errors else'PASS; actual owned fixture sources/MCP; real projects and quality remain')
    raise SystemExit(bool(errors))
