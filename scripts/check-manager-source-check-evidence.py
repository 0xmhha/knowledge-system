"""Native original-source publication checks do not complete the worker."""
import ast
import io
import json
from pathlib import Path, PurePosixPath
import re
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,current_go

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-04-source-check'
SUITES={'registration':14,'collection':23,'review':35,'broker':31,'update_contract':27,'update_inputs':32,'source_check':12}


def check(folder=PACKET,root=ROOT):
    try:
        manifest,binding=verify_packet(folder,root)
        require(manifest['checkpoint']=='KM04_3_A_native_original_source_publication_check'
                and manifest['source_check_checkpoint_complete']is True and manifest['full_KM04_3_complete']is False
                and manifest['full_KM04_complete']is False and manifest['scheduler_runtime_verified']is False
                and manifest['independent_acceptance']is False,'native check promoted to worker/full/independent acceptance')
        validation=load(folder,'validation.json');roadmap=current_go(root,folder,validation,binding)
        require(all(v==0 for v in validation['gates'].values())and validation['manager_python_tests']==174,'failed/reduced validation')
        log=raw(folder,'python-tests.txt').decode();require([int(n)for n in re.findall(r'Ran (\d+) tests',log)]==list(SUITES.values())and log.count('\nOK\n')==7
                and 'FAILED'not in log and 'ResourceWarning'not in log,'174 regressions differ')
        inventory=load(folder,'python-test-inventory.json')
        for label,count in SUITES.items():
            name='scripts/test_knowledge_manager_'+label+'.py';tree=ast.parse((root/name).read_text());names=sorted(n.name for n in ast.walk(tree)if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))
            require(inventory[name]==names and len(names)==count and all(n+' ('in log for n in names),'Python inventory differs')
        require('0 issues.'in raw(folder,'build-lint.txt').decode()and 'documentation: OK (132 live documents)'in raw(folder,'build-lint.txt').decode(),'lint/docs failure')
        diagnostic=[json.loads(line)for line in raw(folder,'guardian-strengthened.jsonl').splitlines()]
        names={'TestGuardianAbsoluteDeadlineWorksWhileOwnerSocketStaysOpen','TestGuardianPastAbsoluteDeadlineCannotStartCommand'}
        require(all(sum(r.get('Test')==n and r['Action']=='pass'for r in diagnostic)==5 for n in names)
                and not any(r['Action']in ('fail','build-fail')for r in diagnostic),'absolute deadline stress failure hidden')
        summary=load(folder,'runtime/summary.json');calls=load(folder,'runtime/calls.json');checks=load(folder,'runtime/checks.json')
        require(summary['scope']=='KM04_3_A_actual_original_source_publication_check_not_worker'
                and summary['checks']==len(checks)==14 and len(set(checks))==14 and summary['builds']==3 and summary['published']==2 and summary['stale_refusals']==1
                and summary['source_check_checkpoint_complete']is True and summary['full_KM04_3_complete']is False
                and summary['scheduler_runtime_verified']is False and summary['paid_model_calls']==0 and summary['independent_acceptance']is False
                and summary['EOF_exit_codes']==[0]*5 and summary['MCP_sessions']==5 and summary['actual_tool_calls']==len(calls),'native runtime scope/results differ')
        require(len({r['session']for r in calls})==5,'native session inventory differs')
        for session in sorted({r['session']for r in calls}):
            requests=[json.loads(line)for line in raw(folder,'runtime/'+session+'/mcp.stdin.jsonl').splitlines()]
            replies=[json.loads(line)for line in raw(folder,'runtime/'+session+'/mcp.stdout.jsonl').splitlines()];responses={r['id']:r for r in replies}
            require(len(responses)==len(replies)and all(r['id']in responses for r in requests),'native reply omitted')
            wire=[r for r in requests if r['method']=='tools/call'];seen=[r for r in calls if r['session']==session];require(len(wire)==len(seen),'native calls omitted')
            for request,observation in zip(wire,seen,strict=True):
                value=observation['response'];expected=observation['expected_error']
                require(request['params']=={'name':observation['tool'],'arguments':observation['arguments']}and responses[request['id']]['result']==value,'fabricated native response')
                require(bool(value.get('isError'))==bool(expected)and(not expected or value['structuredContent']['code']==expected),'native error hidden/changed')
        jobs={n:load(folder,'runtime/v'+str(n)+'-job.json')for n in (1,2,3)}
        require(len({job['job_id']for job in jobs.values()})==3 and all(any(c['response'].get('structuredContent')==job for c in calls)for job in jobs.values()),'job result absent from native trace')
        require(jobs[2]['state']=='failed'and jobs[2]['error']['code']=='source_stale'and jobs[2]['activated']is False
                and load(folder,'runtime/v2-replay.json')==jobs[2],'stale input published/replayed differently')
        for n in (1,3):
            job=jobs[n];current=load(folder,'runtime/v'+str(n)+'-current.json');pack=load(folder,'runtime/v'+str(n)+'-read.json')
            require(job['state']=='succeeded'and job['terminal']is True and job['build_complete']is True and job['activated']is True
                    and current['version']==job['target_version']==job['publication']['version']and current['coordinates']==job['candidate']['coordinates']==pack['coordinates']
                    and all(c['project_id']==job['project_id']and c['dataset_id']==pack['coordinates']['dataset_id']for c in pack['citations']),'actual publication/read coordinates differ')
            require(any(c['response'].get('structuredContent')==pack for c in calls)
                    and any(c['response'].get('structuredContent',{}).get('profile',{}).get('observed_base')==current for c in calls),'published current/read not actually observed')
        boundary=load(folder,'runtime/v2-current.json');require(boundary['link_target']=='guarded-v1'and boundary['original_source_sha256']!=boundary['immutable_v2_source_sha256'],'stale publication boundary not observed')
        old=load(folder,'runtime/v1-after-refusal.json');first=load(folder,'runtime/v1-read.json');latest=load(folder,'runtime/v3-read.json')
        require(any(c['response'].get('structuredContent')==old for c in calls),'retained read absent from actual trace')
        require(old['coordinates']==first['coordinates']and old['bodies']==first['bodies']and old['citations']==first['citations']
                and 'return 7'in json.dumps(first['bodies'])and 'return 9'in json.dumps(latest['bodies']),'reader mixed old/new original source')
        for n in (1,2,3):
            config=load(folder,'runtime/guarded-v'+str(n)+'-maintenance.json')['maintenance']['profiles'][0]
            prepared=load(folder,'runtime/v'+str(n)+'-prepared.json');check=config['source_check']
            require(config['allowed_activation']==['activate_after_gate']and check['command'][2]=='verify-update-source'
                    and check['command'][-1]==prepared['input_id']and check['timeout_seconds']==60,'source check not installed in operator profile')
            for file in ('manager.py','update_inputs.py','update_contract.py','collection.py','broker.py','review.py'):
                digest=sha((root/'plugins/knowledge-manager/scripts'/file).read_bytes())
                require(any(p.endswith('/plugins/knowledge-manager/scripts/'+file)and h==digest for p,h in check['pinned_files'].items()),'executed manager code not byte-pinned')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files={};links={}
            for member in archive:
                path=PurePosixPath(member.name);require(not path.is_absolute()and '..'not in path.parts and member.name not in files and member.name not in links,'unsafe fixture path')
                if member.issym():
                    require(member.name=='fixture/dataset/current'and member.linkname=='guarded-v3','unsafe/unexpected fixture link');links[member.name]=member.linkname
                else:
                    require(member.isfile()or member.isdir(),'unsafe fixture type');files[member.name]=archive.extractfile(member).read()if member.isfile()else None
        require(links=={'fixture/dataset/current':'guarded-v3'}and sha(files['fixture/source/value.go'])==boundary['original_source_sha256'],'final current/original source differs')
        prepared2=load(folder,'runtime/v2-prepared.json');copied='fixture/manager/updates/inputs/'+prepared2['input_id']+'/project/value.go'
        require(sha(files[copied])==boundary['immutable_v2_source_sha256']and not any('/workflows/'in name for name in files),'stale immutable input or review fabricated')
        for name,data in files.items():
            if '/sources/blobs/'in name and data is not None:require(sha(data)==name.rsplit('/',1)[1]and b'PRIVATE_SOURCE_CHECK_SENTINEL'not in data,'private/corrupt retained source')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('2c03322f')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'previous input proof changed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==14 and progress['implementation_completed']==3 and item['status']=='in_progress'
                and [s['status']for s in item['subtasks']]==['complete','complete','in_progress','not_started']
                and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'A native check marked full worker complete')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager original source check:','FAIL'if errors else 'PASS; native boundary only; worker still pending')
    raise SystemExit(bool(errors))
