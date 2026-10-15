"""Do not promote observed inputs/manual native reads to periodic execution."""
import ast
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_checkpoint_go

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-04-inputs'


def check(folder=PACKET,root=ROOT):
    try:
        manifest,_=verify_packet(folder,root)
        require(manifest['checkpoint']=='KM04_2_observed_input_change_delete_and_noop_pins'
                and manifest['KM04_2_complete']is True and manifest['full_KM04_complete']is False
                and manifest['scheduler_runtime_verified']is False and manifest['independent_acceptance']is False,'manual proof promoted to full/scheduler/independent acceptance')
        validation=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,validation,'cea1c943')
        require(all(v==0 for v in validation['gates'].values())and validation['input_unit_tests']==32 and validation['policy_unit_tests']==27,'failed/reduced gates')
        for label,count,path in [('input',32,'scripts/test_knowledge_manager_update_inputs.py'),('contract',27,'scripts/test_knowledge_manager_update_contract.py')]:
            names=sorted(n.name for n in ast.walk(ast.parse((root/path).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))
            log=raw(folder,label+'-tests.txt').decode()
            require(len(names)==count and names==load(folder,label+'-test-inventory.json')and all(n+' ('in log for n in names)
                    and 'Ran '+str(count)+' tests'in log and '\nOK\n'in log and 'FAILED'not in log,'test names/results differ')
        log=raw(folder,'build-lint-manager.txt').decode()
        require('documentation: OK (132 live documents)'in log and all('Ran '+str(c)+' tests'in log for c in (14,23,35,31))
                and '0 issues.'in log and 'FAILED'not in log and 'ResourceWarning'not in log,'manager103/lint/docs failure hidden')
        summary=load(folder,'runtime/summary.json');checks=load(folder,'runtime/checks.json');calls=load(folder,'runtime/calls.json')
        require(summary['scope']=='KM04_2_observed_inputs_and_manual_native_build_reads_not_scheduler'
                and summary['checks']==len(checks)==16 and len(set(checks))==16 and summary['builds']==summary['held_candidates']==4
                and summary['source_changes']==2 and summary['native_noop_validation']is True and summary['scheduler_runtime_verified']is False
                and summary['DB_published']is False and summary['paid_model_calls']==0 and summary['semantic_approval_inferred']is False
                and summary['full_KM04_complete']is False and summary['independent_acceptance']is False,'runtime proof inflated')
        require(summary['MCP_sessions']==len(summary['EOF_exit_codes'])==11 and summary['EOF_exit_codes']==[0]*11
                and summary['actual_tool_calls']==len(calls),'native sessions/calls mismatch')
        for session in sorted({r['session']for r in calls}):
            requests=[json.loads(line)for line in raw(folder,'runtime/'+session+'/mcp.stdin.jsonl').splitlines()]
            responses=[json.loads(line)for line in raw(folder,'runtime/'+session+'/mcp.stdout.jsonl').splitlines()]
            byid={r['id']:r for r in responses};observed=[r for r in calls if r['session']==session]
            wire=[r for r in requests if r['method']=='tools/call']
            require(len(wire)==len(observed)and all(r['id']in byid for r in requests),'missing native response/call')
            for request,seen in zip(wire,observed,strict=True):
                require(request['params']=={'name':seen['tool'],'arguments':seen['arguments']}and byid[request['id']]['result']==seen['response']
                        and not seen['response'].get('isError'),'native response fabricated/failed')
        for label in ('observed-v1','observed-v2','observed-v3','other-v1'):
            job=load(folder,'runtime/'+label+'-job.json');profile=load(folder,'runtime/'+label+'-profile.json');read=load(folder,'runtime/'+label+'-read.json')
            require(job['state']=='held_for_review'and job['terminal']is True and job['activated']is False and job['build_complete']is True
                    and all(job['gates'][g]=='passed'for g in profile['required_gates'])and read['coordinates']==job['candidate']['coordinates']
                    and all(c['project_id']==job['project_id']and c['dataset_id']==read['coordinates']['dataset_id']for c in read['citations']),'held native build/read mismatch')
            require(any(r['response'].get('structuredContent')==job for r in calls)and any(r['response'].get('structuredContent')==read for r in calls),'job/read absent from actual native trace')
        a,b,c=(load(folder,'runtime/'+label+'-prepared.json')for label in ('v1','v2','v3'))
        require(len({r['input_id']for r in (a,b,c)})==3 and {r['origin_id']for r in b['change']['modified']}=={'repo','material:terms'}
                and c['material_documents']==0 and any(r['origin_id']=='material:terms'for r in c['change']['deleted'])and len(c['change']['rename_candidates'])==1
                and all(r['semantic_review']=='not_performed'and r['human_approval']=='not_inferred'and r['DB_built']is False and r['DB_published']is False for r in (a,b,c)),'change/delete/review separation differs')
        first=load(folder,'runtime/observed-v1-read.json');old=load(folder,'runtime/v1-after-change-read.json');latest=load(folder,'runtime/observed-v3-read.json')
        require(first['coordinates']==old['coordinates']and first['bodies']==old['bodies']and first['citations']==old['citations']
                and 'return 7'in json.dumps(first['bodies'])and 'seven slots'in json.dumps(first['bodies'])
                and 'return 8'in json.dumps(latest['bodies'])and all(term not in json.dumps(latest['bodies'])for term in ('seven slots','eight slots')),'old/current retained evidence mixed')
        noop=load(folder,'runtime/noop-validation.json');again=load(folder,'runtime/noop-prepared.json')
        require(noop['may_skip']is True and again['input_id']==a['input_id']and again['change']['input_unchanged']is True
                and again['change']['DB_health_or_noop_verified']is False and noop['read_pack']==load(folder,'runtime/noop-read.json')
                and noop['job']==load(folder,'runtime/observed-v1-job.json'),'no-op skipped actual native integrity check')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files={}
            for member in archive:
                path=PurePosixPath(member.name)
                require(not path.is_absolute()and '..'not in path.parts and member.name not in files and(member.isfile()or member.isdir()),'unsafe input fixture')
                files[member.name]=archive.extractfile(member).read()if member.isfile()else None
        other={name.removeprefix('fixture/dataset-b/'):sha(data)for name,data in files.items()if name.startswith('fixture/dataset-b/')and data is not None}
        require(other==load(folder,'runtime/other-before.json')and not any(name in files for name in ('fixture/dataset-a/current','fixture/dataset-b/current')),'foreign dataset changed or held DB published')
        for name,data in files.items():
            if ('/sources/blobs/'in name or '/updates/inputs/'in name and '/blobs/'in name)and data is not None:
                require(sha(data)==name.rsplit('/',1)[1]and b'PRIVATE_UPDATE_INPUT_SENTINEL'not in data,'private/corrupt captured source')
        history=load(folder,'historical-design.json')
        require(history['revision'].startswith('7268ac7f')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'contract history differs')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==14 and progress['implementation_completed']==3 and item['status']=='in_progress'
                and [s['status']for s in item['subtasks']]==['complete','complete','not_started','not_started']
                and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'input-only ledger scope differs')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager update inputs:','FAIL'if errors else 'PASS; manual native input/build/read; scheduling pending')
    raise SystemExit(bool(errors))
