"""Verify actual two-project periodic lifecycle without claiming retrieval quality."""
import json
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-04-periodic'


def check(folder=PACKET,root=ROOT):
    try:
        manifest,_=verify_packet(folder,root)
        require(manifest['full_KM04_complete']is True and manifest['independent_acceptance']is False and manifest['paid_model_calls']==0,'completion scope changed')
        validation=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,validation,'a801f551')
        require(all(v==0 for v in validation['gates'].values())and '0 issues.'in raw(folder,'build-lint.txt').decode(),'failed validation')
        s=load(folder,'runtime/summary.json');checks=load(folder,'runtime/checks.json');calls=load(folder,'runtime/calls.json')
        require(s['checks']==len(checks)==len(set(checks))==20 and s['actual_tool_calls']==len(calls)==221
                and s['EOF_exit_codes']==[0,0]and s['projects']==2 and s['project_a_build_jobs']==5
                and s['project_a_publications']==4 and s['intentional_engine_failures']==1 and s['project_b_build_jobs']==1
                and s['mock_embedding_not_quality']is True and s['independent_acceptance']is False and s['paid_model_calls']==0,'actual runtime denominator/scope changed')
        for session in('project-a-session','project-b-session'):
            requests=[json.loads(l)for l in raw(folder,'runtime/'+session+'/mcp.stdin.jsonl').splitlines()]
            replies=[json.loads(l)for l in raw(folder,'runtime/'+session+'/mcp.stdout.jsonl').splitlines()];byid={r['id']:r for r in replies}
            require(len(byid)==len(replies)and all(r['id']in byid for r in requests),'wire incomplete')
            wire=[r for r in requests if r['method']=='tools/call'];seen=[r for r in calls if r['session']==session]
            require(len(wire)==len(seen),'actual calls omitted')
            for req,item in zip(wire,seen,strict=True):
                require(req['params']=={'name':'cks.manager.update_'+item['operation'],'arguments':item['arguments']}and byid[req['id']]['result']==item['response'],'wire observation changed')
                require(bool(item['expected_error'])==bool(item['response'].get('isError')),'tool failure hidden')
        payload=[r['response'].get('structuredContent')for r in calls]
        def read(name):
            v=load(folder,'runtime/'+name+'.json');require(v in payload,'read not an actual response');return v['pack']
        for n,value in [(1,7),(2,8),(3,9),(4,10)]:
            row=load(folder,'runtime/a'+str(n)+'-attempt.json');pack=read('a'+str(n)+'-read')
            require(row['outcome']=='succeeded'and row['job']['activated']is True and pack['coordinates']==row['job']['candidate']['coordinates']and any('return '+str(value)in json.dumps(b)for b in pack['bodies']),'actual version source missing')
        old,retained=read('a1-terms-read'),read('a1-retained-after-delete')
        require(any('OLD_STAKING_POLICY_MARKER'in json.dumps(b)for b in old['bodies'])and all(old[k]==retained[k]for k in('coordinates','bodies','citations')),'retained old material lost')
        deleted=read('a3-deleted-query');require(not any(w in json.dumps(deleted['bodies'])for w in('OLD_STAKING_POLICY_MARKER','OLD_SUPPLIED_PROJECT_NOTE','OLD_PROJECT_DOCUMENT_MARKER'))and any('NEW_SUPPLIED_PROJECT_NOTE'in json.dumps(b)for b in read('a3-provided-query')['bodies']),'deleted/stale or missing new note')
        capture=load(folder,'runtime/a3-capture.json');require(any(s['origin_id']=='material:terms'and s.get('missing')is True for s in capture['scopes'])and capture['semantic_review']=='not_performed'and capture['human_approval']=='not_inferred','material absence or review changed')
        failure=load(folder,'runtime/failed-attempt.json');retry=load(folder,'runtime/a4-attempt.json');v3=load(folder,'runtime/a3-attempt.json');pointer=load(folder,'runtime/failed-current.json')
        require(failure['outcome']=='failed'and failure['job']['activated']is False and pointer['version']==v3['job']['candidate']['version']and pointer['last_success']['intent_id']==v3['id']and retry['previous_attempt']==failure['id']and retry['input_id']==failure['input_id']and retry['attempt']==2,'failure baseline/retry changed')
        require(load(folder,'runtime/a2-noop-status.json')['attempts_recorded']==2 and load(folder,'runtime/final-noop-status.json')['attempts_recorded']==5,'no-op duplicated jobs')
        before,after=read('b1-read'),read('b1-read-after-a-updates')
        require(all(before[k]==after[k]for k in('coordinates','bodies','citations'))and load(folder,'runtime/b-version-before.json')==load(folder,'runtime/b-version-after.json')and before['coordinates']['project_id']!=old['coordinates']['project_id'],'other project changed')
        diag=load(folder,'initial-query-diagnosis.json');initial=load(folder,'initial-identifier-read.json')
        require(initial['pack']['bodies']==[]and any(any('OLD_STAKING_POLICY_MARKER'in json.dumps(b)for b in r['value']['pack']['bodies'])for r in diag),'initial retrieval limitation hidden')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('4a2ca4cd')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'prior integrated proof failed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==16 and progress['implementation_completed']==4 and item['status']=='complete'and all(s['status']=='complete'for s in item['subtasks'])and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'completion ledger inconsistent')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration)as error:return[str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager periodic:', 'FAIL'if errors else'PASS; actual lifecycle; retrieval quality separate')
    raise SystemExit(bool(errors))
