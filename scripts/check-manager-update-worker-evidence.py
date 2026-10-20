"""Actual foreground worker evidence, distinct from sustained polling/host kill."""
import ast
import io
import json
from pathlib import Path,PurePosixPath
import re
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,current_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-04-worker'
SUITES={'registration':14,'collection':23,'review':35,'broker':31,'update_contract':27,'update_inputs':32,'source_check':12,'update_state':50,'update_worker':23}


def check(folder=PACKET,root=ROOT):
    try:
        manifest,binding=verify_packet(folder,root)
        require(manifest['checkpoint']=='KM04_3_B2_foreground_native_worker'
                and manifest['foreground_worker_verified']is True and manifest['full_KM04_3_complete']is False
                and manifest['full_KM04_4_complete']is False and manifest['sustained_schedule_verified']is False
                and manifest['actual_worker_host_SIGKILL_verified']is False and manifest['independent_acceptance']is False,'worker promoted to sustained/kill/full/quality acceptance')
        validation=load(folder,'validation.json');roadmap=current_go(root,folder,validation,binding)
        require(validation['manager_python_tests']==sum(SUITES.values())==247 and all(v==0 for v in validation['gates'].values()),'failed/reduced validation')
        log=raw(folder,'python-tests.txt').decode();require([int(n)for n in re.findall(r'Ran (\d+) tests',log)]==list(SUITES.values())
                and log.count('\nOK\n')==9 and 'FAILED'not in log and 'ResourceWarning'not in log,'Python completions differ')
        inventory=load(folder,'python-test-inventory.json')
        for label,count in SUITES.items():
            path='scripts/test_knowledge_manager_'+label+'.py';names=sorted(n.name for n in ast.walk(ast.parse((root/path).read_text())) if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))
            require(len(names)==count and inventory[path]==names and all(n+' ('in log for n in names),'Python names omitted')
            require(path in (root/'Makefile').read_text(),'standard manager checks omit a suite')
        require('0 issues.'in raw(folder,'build-lint.txt').decode() and 'documentation: OK (132 live documents)'in raw(folder,'docs-final.txt').decode(),'lint/docs failure')
        summary=load(folder,'runtime/summary.json');checks=load(folder,'runtime/checks.json');calls=load(folder,'runtime/native-calls.json');connections=load(folder,'runtime/connections.json')
        require(summary['scope']=='KM04_3_B2_actual_foreground_native_worker_not_sustained_schedule_or_killed_host'
                and summary['checks']==len(checks)==len(set(checks))==14 and summary['native_RPC_observations']==len(calls)
                and summary['native_sessions']==len(connections)==16 and summary['native_EOF_exit_codes']==[0]*16
                and summary['actual_build_jobs']==6 and summary['actual_published_at_least']==4
                and summary['exact_admission_replay_observed']is True and summary['periodic_due_retry_observed']is True
                and summary['sustained_periodic_schedule_verified']is False and summary['actual_worker_host_SIGKILL_verified']is False
                and summary['full_KM04_3_complete']is False and summary['full_KM04_4_complete']is False
                and summary['paid_model_calls']==0 and summary['independent_acceptance']is False,'runtime counts/scope differ')
        require([c['id']for c in connections]==list(range(1,17)) and all(c['EOF_exit_code']==0 for c in connections),'native EOF hidden')
        for session in connections:
            rows=[r for r in calls if r['session']==session['id']]
            require(rows and rows[0]['method']=='initialize' and [r['id']for r in rows]==list(range(1,len(rows)+1)),'native observation missing/duplicated')
        toolcalls=[r for r in calls if r['method']=='tools/call']
        require(all(not r['result'].get('isError') for r in toolcalls),'unreported native tool refusal')
        observed=[r['result'].get('structuredContent') for r in toolcalls]
        names=('v1-attempt','v2-attempt','failed-attempt','retry-attempt','replayed-attempt','final-attempt')
        attempts={name:load(folder,'runtime/'+name+'.json')for name in names}
        require(len({r['job']['job_id']for r in attempts.values()})==6,'native jobs duplicated')
        for row in attempts.values():
            job=row['job'];require(job in observed and job['terminal']is True and job['request_digest']==row['request_digest']
                    and job['target_version']==row['request']['new_version'] and job['project_id']==row['profile']['project_id'],'job not actually observed')
            require(any(r['params']['name']=='cks.dataset.build' and r['params']['arguments']==row['request']for r in toolcalls),'exact build request absent')
            require(any(r['params']['name']=='cks.dataset.inspect_profile' and r['result']['structuredContent']['profile']==row['profile']for r in toolcalls),'profile pins not actually inspected')
        for name,readname,expected in [('v1-attempt','v1-read',7),('v2-attempt','v2-read',8),('retry-attempt','v3-read',9),('final-attempt','final-read',11)]:
            row=attempts[name];pack=load(folder,'runtime/'+readname+'.json')
            require(row['outcome']=='succeeded' and row['job']['activated']is True and pack in observed
                    and pack['coordinates']==row['job']['candidate']['coordinates']
                    and any('return '+str(expected) in json.dumps(b)for b in pack['bodies'])
                    and all(c['project_id']==pack['coordinates']['project_id'] and c['dataset_id']==pack['coordinates']['dataset_id']for c in pack['citations']),'publication/read not verified')
        first=load(folder,'runtime/v1-read.json');retained=load(folder,'runtime/v1-retained-read.json')
        require(retained in observed and first['bodies']==retained['bodies'] and first['citations']==retained['citations'],'retained original read changed')
        failed,retry=attempts['failed-attempt'],attempts['retry-attempt']
        require(failed['outcome']=='failed' and failed['job']['activated']is False and retry['attempt']==2
                and retry['previous_attempt']==failed['id'] and retry['input_id']==failed['input_id'] and retry['cycle_id']==failed['cycle_id']
                and retry['request']['idempotency_key']!=failed['request']['idempotency_key']
                and retry['profile']['observed_base']['coordinates']==attempts['v2-attempt']['job']['candidate']['coordinates'],'failed input retry/current differs')
        pending=load(folder,'runtime/uncertain-intent.json');ack=load(folder,'runtime/lost-ack.json');replayed=attempts['replayed-attempt']
        require(ack in observed and pending['job']is None and pending['request']==replayed['request']
                and pending['input_id']==replayed['input_id'] and ack['job_id']==replayed['job']['job_id']
                and replayed['outcome']=='canceled' and replayed['job']['activated']is False,'lost admission replaced/hidden')
        require(sum(r['params']['name']=='cks.dataset.build' and r['params']['arguments']==pending['request']for r in toolcalls)==2,'unknown admission not exact replayed')
        noop=load(folder,'runtime/noop.json');final=load(folder,'runtime/final-state.json');stopped=load(folder,'runtime/stopped-state.json')
        require(noop['state']['phase']=='no_op' and len(noop['state']['attempts'])==1 and noop['state']['last_success']==attempts['v1-attempt']['id']
                and final['state']['last_success']==attempts['final-attempt']['id'] and final['state']['pending']is None
                and stopped['scheduler_running']is False and stopped['state']==final['state'],'no-op/final state differs')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files={};links={}
            for member in archive:
                path=PurePosixPath(member.name);require(not path.is_absolute()and '..'not in path.parts and member.name not in files and member.name not in links,'unsafe fixture path')
                if member.issym():
                    require(member.name=='fixture/dataset/current' and member.linkname==attempts['final-attempt']['request']['new_version'],'unexpected fixture link');links[member.name]=member.linkname
                else:
                    require(member.isfile()or member.isdir(),'unsafe fixture type');files[member.name]=archive.extractfile(member).read()if member.isfile()else None
        require(len(links)==1 and not any('/workflows/'in p for p in files),'current/review invented')
        one='fixture/manager/updates/inputs/'+attempts['v1-attempt']['input_id']+'/project/old.md';two='fixture/manager/updates/inputs/'+attempts['v2-attempt']['input_id']+'/project/old.md'
        require(one in files and two not in files and b'return 11'in files['fixture/source/value.go'],'deletion/new source not retained correctly')
        require(all(b'PRIVATE_WORKER_SOURCE_SENTINEL'not in content for name,content in files.items()if content is not None and ('/sources/blobs/'in name or '/updates/inputs/'in name)),'excluded private data copied')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('d00224a5') and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'B1 proof changed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==14 and progress['implementation_completed']==3 and [s['status']for s in item['subtasks']]==['complete','complete','in_progress','not_started']
                and item['independent_acceptance']=='not_recorded' and not roadmap.check(root=root),'worker marked sustained/host completion')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager update worker:','FAIL'if errors else 'PASS; foreground/native only; sustained/host-kill still pending')
    raise SystemExit(bool(errors))
