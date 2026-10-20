#!/usr/bin/env python3
"""Actual host transport proof must not become model/independent acceptance."""
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,current_go

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-03-host'
CASES = ('full','full-restart','insufficient','missing','active-stop','active-stop-recover','kill','kill-recover')
KINDS = ('full','restart','insufficient','missing','active-stop','recover','kill','recover')
PREFIX = 'mcp__plugin_knowledge-manager_cks-manager__'


def records(data,truncated=False):
    result=[]
    lines=data.splitlines(keepends=True)
    for index,line in enumerate(lines):
        if not line.startswith(b'{'):
            continue
        try:
            result.append(json.loads(line))
        except ValueError:
            require(truncated and index==len(lines)-1 and not line.endswith(b'\n'),'unexpected truncated record')
    return result


def check(folder=PACKET,root=ROOT):
    try:
        manifest,binding = verify_packet(folder,root)
        require(manifest['checkpoint']=='KM03_4_C_actual_Claude_host_local_transport_and_lifecycle'and manifest['KM03_4_complete']is True
                and manifest['full_KM03_complete']is True and manifest['independent_acceptance']is False,'host completion scope differs')
        validation = load(folder,'validation.json')
        roadmap = current_go(root,folder,validation,binding)
        require(all(v==0 for v in validation['gates'].values())and validation['manager_unit_tests']==103,'manager/gate proof missing')
        log = raw(folder,'manager-tests.txt').decode()
        require(all('Ran '+str(count)+' tests'in log for count in (14,23,35,31))and log.count('\nOK\n')==4
                and 'ResourceWarning'not in log and 'FAILED'not in log and 'documentation: OK'in raw(folder,'lint.txt').decode(),'103 regressions/log or lint/docs differ')
        summary = load(folder,'runtime/summary.json')
        require(summary['scope']=='actual_Claude_Code_local_scripted_transport_not_real_model_quality'and summary['runs']==8
                and summary['real_model_calls']==0 and summary['real_model_cost_USD']==0 and summary['mock_embedding_not_quality']is True
                and summary['all_requested_cases']is True and summary['KM03_4_complete']is True and summary['full_KM03_complete']is True
                and summary['independent_acceptance']is False,'scripted transport promoted to paid/model/independent quality')
        require([r['case']for r in summary['observations']]==list(KINDS),'actual host run inventory differs')
        version = load(folder,'runtime/host-version.json')
        require(version['version']==summary['host_version']=='2.1.300 (Claude Code)'and len(version['sha256'])==64,'actual CLI identity missing')
        native = load(folder,'native/commands.json')
        require(len(native)==3 and all(r['exit_code']==0 for r in native),'native validator execution missing')
        for label in ('manifest','skills','agents'):
            row = load(folder,'native/'+label+'.stdout.json')
            require(row['success']is True and row['strict']is True,'native package validation failed')
        all_calls={}
        for label,kind,observation in zip(CASES,KINDS,summary['observations'],strict=True):
            prefix='runtime/'+label+'/'
            require(observation==load(folder,prefix+'observation.json'),'run observation differs')
            require(observation['host_exit_code']==(-9 if kind=='kill'else 0)and observation['intentional_SIGKILL']==(kind=='kill')
                    and observation['manager_forced_kill']is False and observation['real_model_calls']==0
                    and observation['DB_published']is False and observation['independent_acceptance']is False
                    and observation['agent_autonomy_or_model_quality_verified']is False,'host signal/quality/result inflated')
            inv = load(folder,prefix+'invocation.json')
            require(inv['credential_kind']=='constant_fake_local_token'and inv['scope']=='actual_CLI_with_local_scripted_API_not_real_model'
                    and inv['real_model_calls']==0 and inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:')
                    and inv['MCP_source_sha256']==sha((root/'plugins/knowledge-manager/.mcp.json').read_bytes()),'host API escaped local fixture or plugin declaration changed')
            for flag in ('--plugin-dir','--strict-mcp-config','--mcp-config','--setting-sources','--permission-prompts','--no-session-persistence'):
                require(flag in inv['argv'],'isolated host flag missing')
            events=records(raw(folder,prefix+'host.stdout.jsonl'),kind=='kill')
            init=next(r for r in events if r.get('subtype')=='init')
            expected_status='failed'if kind=='missing'else 'connected'
            require(init['mcp_servers']==[{'name':'plugin_knowledge-manager_cks-manager','status':expected_status,'source':'dynamic'}]
                    and observation['MCP_status']==expected_status and any(p['name']=='knowledge-manager'and p['version']=='0.4.1'for p in init['plugins'])
                    and 'knowledge-manager:manage'in init['skills']and 'knowledge-manager:knowledge-manager'in init['agents'],'actual plugin/skill/agent/MCP discovery differs')
            gateway=load(folder,prefix+'gateway-requests.json')
            require(len(gateway)==observation['API_requests']and all('/messages'in r['path']for r in gateway),'gateway requests not recorded')
            require(any('<command-message>knowledge-manager:manage</command-message>'in json.dumps(r['body']['messages'])and 'Manage the selected project'in json.dumps(r['body']['messages'])for r in gateway),'manual skill not expanded by actual host')
            frames=records(raw(folder,prefix+'mcp-frames.jsonl'))
            starts=[r for r in frames if r['kind']=='start']
            exits=[r for r in frames if r['kind']=='exit']
            expected_exit=2 if kind=='missing'else 0
            require(len(starts)==len(exits)==observation['manager_start_attempts']and {r['child_pid']for r in starts}=={r['child_pid']for r in exits}
                    and [r['exit_code']for r in exits]==observation['manager_exit_codes']and all(r['exit_code']==expected_exit and r['forced_kill']is False for r in exits),'missing/restarted child or bad signal cleanup')
            requests=[json.loads(r['raw'])for r in frames if r['kind']=='request']
            replies=[json.loads(r['raw'])for r in frames if r['kind']=='response']
            responses={r['id']:r for r in replies if 'id'in r}
            require(len(responses)==len(replies),'duplicate/nonresponse frame')
            if kind=='missing':
                require(observation['actual_tool_results']==0 and not replies and not any(r['method']=='tools/call'for r in requests),'missing engine used a fallback tool')
                continue
            require(len(exits)==1 and all('error'not in r for r in replies),'normal child/protocol failed')
            calls=[r for r in requests if r['method']=='tools/call']
            actual=load(folder,prefix+'actual-tool-results.json')
            require(len(calls)==len(actual)==observation['actual_tool_results'],'host/native tool count differs')
            for call,seen in zip(calls,actual,strict=True):
                response=responses[call['id']]['result']
                require(call['params']['name']==seen['tool']and bool(response.get('isError'))==seen['is_error'],'host tool result differs from actual manager')
                value=response.get('structuredContent')
                if value is None:
                    value=json.loads(response['content'][0]['text'])
                require(value==seen['value'],'gateway input fabricated native result')
                aliases=[r['scripted_response']['content'][0]for r in gateway if r.get('scripted_response',{}).get('content',[{}])[0].get('type')=='tool_use'
                         and r['scripted_response']['content'][0]['id']==seen['actual_tool_use_id']]
                require(len(aliases)==1 and aliases[0]['name']==PREFIX+seen['tool'].replace('.','_')and aliases[0]['input']==call['params']['arguments'],'scripted action not routed by actual host')
            require(all(r.get('id')in responses for r in requests if 'id'in r),'missing actual request response')
            all_calls[label]=[(r['params']['name'],r['params']['arguments'],responses[r['id']]['result'])for r in calls]
        full=load(folder,'runtime/full/workflow.json')
        restarted=load(folder,'runtime/full-restart/workflow.json')
        require(full['job']['state']=='held_for_review'and full['job']['activated']is False and full['read']['coordinates']==full['job']['candidate']['coordinates']
                and full['read']['bodies']==restarted['read']['bodies']and full['read']['citations']==restarted['read']['citations'],'full/restarted actual read changed dataset')
        require(any(c['origin_id']=='docs:corpus.0'for c in full['read']['citations'])and all(c['project_id']==full['job']['project_id']for c in full['read']['citations'])
                and 'HostValue'in json.dumps(full['read']['bodies'])and 'return 42'in json.dumps(full['read']['bodies']),'actual reviewed source/body missing or foreign')
        insufficient=all_calls['insufficient']
        require(len(insufficient)==1 and insufficient[0][0]=='cks.manager.collect'and insufficient[0][2]['isError']is True,'insufficient evidence was invented/succeeded')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files={}
            for item in archive:
                path=PurePosixPath(item.name)
                require(not path.is_absolute()and '..'not in path.parts and item.name not in files and(item.isfile()or item.isdir()),'unsafe host fixture')
                files[item.name]=archive.extractfile(item).read()if item.isfile()else None
        project_ids=[]
        for label in ('full','insufficient','missing','active-stop','kill'):
            prefix=label+'-fixture/'
            r=json.loads(files[prefix+'manager/registration.json'])['registration']
            project_ids.append(r['project_id'])
            require(not any(n==prefix+'dataset/current'or n.startswith(prefix+'source/.cks/')for n in files),'original source adopted or held DB published')
            for name,data in files.items():
                if name.startswith(prefix+'dataset/')and '/sources/blobs/'in name and data is not None:
                    require(sha(data)==name.rsplit('/',1)[1]and b'PRIVATE_HOST_INPUT_SENTINEL'not in data,'private/tampered captured source')
        require(len(set(project_ids))==5,'projects merged')
        for label in ('active-stop','kill'):
            active=load(folder,'runtime/'+label+'/workflow.json')
            recovery=load(folder,'runtime/'+label+'-recover/workflow.json')
            quiescence=json.loads(files[label+'-fixture/graph-quiescence.json'])
            require(active['inflight_graph_observed']is True and active['job']['terminal']is False and quiescence['process_gone_before_recovery']is True
                    and quiescence['signal_used_for_this_check']==0,'actual graph was not interrupted/quiescent')
            canceled=recovery['canceled_job']
            require(canceled['job_id']==active['job']['job_id']and canceled['request_digest']==active['job']['request_digest']and canceled['terminal']is True
                    and canceled['state']=='canceled'and canceled['activated']is False and canceled['candidate']is None,'accepted cancellation not durably restored')
            calls=all_calls[label+'-recover']
            replay=next(row for name,args,row in calls if name=='cks.manager.resume_build')
            replay_args=next(args for name,args,row in calls if name=='cks.manager.resume_build')
            require(replay['structuredContent']==canceled and replay_args['original_request']==active['request'],'recovery replay changed intent')
            require(recovery['job']['state']=='held_for_review'and recovery['job']['job_id']!=canceled['job_id']and recovery['request']['new_version']!=active['request']['new_version']
                    and recovery['read']['coordinates']==recovery['job']['candidate']['coordinates'],'new actual build/read did not recover after interruption')
        history=load(folder,'historical-design.json')
        require(history['revision'].startswith('c51f7423')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'B historical proof differs')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item=next(i for i in progress['items']if i['id']=='KM-03')
        require(progress['completed_subtasks']==12 and progress['implementation_completed']==3 and item['status']=='complete'
                and all(s['status']=='complete'for s in item['subtasks'])and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'host roadmap count/acceptance mismatch')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:
        return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:
        print('FAIL:',error)
    print('manager host evidence:','FAIL'if errors else 'PASS; actual local transport/lifecycle; model/independent quality separate')
    raise SystemExit(bool(errors))
