"""Integrated updater acceptance: actual MCP and installed Claude, not quality."""
import ast
import io
import json
from pathlib import Path,PurePosixPath
import re
import tarfile
from urllib.parse import urlparse

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-04-updates-MCP'
SUITES={'registration':14,'collection':23,'review':35,'broker':31,'update_contract':27,'update_inputs':32,'source_check':12,'update_state':50,'update_worker':44}


def check(folder=PACKET,root=ROOT):
    try:
        manifest,binding=verify_packet(folder,root)
        require(manifest['checkpoint']=='KM04_3_C2_integrated_updates_MCP_Claude'
                and manifest['full_KM04_3_complete']is True and manifest['full_KM04_complete']is False
                and manifest['full_KM04_4_complete']is False and manifest['actual_Claude_updates_host_verified']is True
                and manifest['independent_acceptance']is False and manifest['paid_model_calls']==0,'scope promoted to final/quality/independent acceptance')
        validation=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,validation,'a801f551')
        require(validation['manager_python_tests']==sum(SUITES.values())==268 and all(v==0 for v in validation['gates'].values()),'failed/reduced validation')
        log=raw(folder,'python-tests.txt').decode();inventory=load(folder,'python-test-inventory.json')
        require([int(n)for n in re.findall(r'Ran (\d+) tests',log)]==list(SUITES.values())and log.count('\nOK\n')==9
                and 'FAILED'not in log and 'ResourceWarning'not in log,'Python completion differs')
        for label,count in SUITES.items():
            path='scripts/test_knowledge_manager_'+label+'.py';names=sorted(n.name for n in ast.walk(ast.parse((root/path).read_text()))if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))
            require(len(names)==count and inventory[path]==names and all(n+' ('in log for n in names),'Python names omitted')
        require('0 issues.'in raw(folder,'build-lint.txt').decode()and 'documentation: OK (132 live documents)'in raw(folder,'docs-final.txt').decode(),'lint/docs failure')
        plugin=load(folder,'plugin-validate.json');require(plugin['success']is True and plugin['strict']is True and not plugin['manifest']['errors']and not plugin['manifest']['warnings'],'plugin validation failed')
        summary=load(folder,'MCP/summary.json');checks=load(folder,'MCP/checks.json');calls=load(folder,'MCP/calls.json');observations=load(folder,'MCP/process-observations.json')
        require(summary['checks']==len(checks)==len(set(checks))==20 and summary['actual_tool_calls']==len(calls)
                and summary['actual_MCP_sessions']==3 and summary['manager_exit_codes']==[-9,0,0]
                and summary['actual_parent_termination_cases']==2 and summary['actual_linked_recovery_publications']==2
                and summary['same_session_update_and_retained_read_verified']is True and summary['actual_CLI_Claude_verified_here']is False
                and summary['full_KM04_3_complete']is False and summary['full_KM04_4_complete']is False and summary['paid_model_calls']==0
                and summary['independent_acceptance']is False,'raw MCP scope/results differ')
        toolnames={t['name']for t in load(folder,'MCP/tools.json')}
        require(toolnames=={'cks.manager.update_'+n for n in('policy_template','configure','status','start','stop','read')},'updates mode aliases changed')
        sessions=sorted({r['session']for r in calls});require(len(sessions)==3,'MCP sessions omitted')
        for session in sessions:
            requests=[json.loads(line)for line in raw(folder,'MCP/'+session+'/mcp.stdin.jsonl').splitlines()]
            replies=[json.loads(line)for line in raw(folder,'MCP/'+session+'/mcp.stdout.jsonl').splitlines()];by_id={r['id']:r for r in replies}
            require(len(by_id)==len(replies)and all(r['id']in by_id for r in requests),'wire reply missing/duplicate')
            wire=[r for r in requests if r['method']=='tools/call'];seen=[r for r in calls if r['session']==session]
            require(len(wire)==len(seen),'actual calls omitted')
            for request,item in zip(wire,seen,strict=True):
                require(request['params']=={'name':item['tool'],'arguments':item['arguments']}and by_id[request['id']]['result']==item['response'],'actual tool result replaced')
                require(bool(item['response'].get('isError'))==bool(item['expected_error']),'tool refusal hidden')
        payloads=[c['response'].get('structuredContent')for c in calls if not c['expected_error']]
        first,second=load(folder,'MCP/v1.json'),load(folder,'MCP/v2.json')
        for name,row,expected in [('v1-read',first,7),('v2-read',second,8)]:
            value=load(folder,'MCP/'+name+'.json');pack=value['pack']
            require(value in payloads and row['outcome']=='succeeded'and value['current_selected_at_pointer_checks']is True
                    and pack['coordinates']==row['job']['candidate']['coordinates']and any('return '+str(expected)in json.dumps(b)for b in pack['bodies']),'actual publication/read not observed')
        old=load(folder,'MCP/v1-during-update-read.json');initial=load(folder,'MCP/v1-read.json')
        require(old in payloads and old['pack']['bodies']==initial['pack']['bodies']and old['pack']['citations']==initial['pack']['citations'],'retained read switched to current')
        previous=second
        for label,exit_code,value in [('parent-kill',-9,9),('parent-EOF',0,10)]:
            before=load(folder,'MCP/'+label+'-before.json');pending=next(r for r in before['attempts']if r['id']==before['pending'])
            process=next(o for o in observations if o['label']==label)
            require(process['manager_exit_code']==exit_code and process['observed_monotonic']>=process['graph']['started_monotonic']+9
                    and process['late_write_exists']is False and not any(p['running']for p in process['processes'])
                    and process['current_before']==process['current_after']==previous['job']['candidate']['version']
                    and process['signal_used_for_recovered_PIDs']is None,'late write/live child/current swap hidden')
            restart=load(folder,'MCP/'+label+'-restart-status.json');canceled=load(folder,'MCP/'+label+'-canceled.json');retry=load(folder,'MCP/'+label+'-recovered.json');read=load(folder,'MCP/'+label+'-recovered-read.json')
            require(restart in payloads and restart['process']['owned_by_this_host']is False and restart['process']['previous_unowned_pid_never_signaled']is True
                    and canceled['outcome']=='canceled'and canceled['job']['terminal']is True and canceled['job']['activated']is False
                    and canceled['request']==pending['request']and retry['previous_attempt']==pending['id']and retry['input_id']==pending['input_id']and retry['attempt']==2
                    and retry['outcome']=='succeeded'and read in payloads and read['current_selected_at_pointer_checks']is True
                    and read['pack']['coordinates']==retry['job']['candidate']['coordinates']and any('return '+str(value)in json.dumps(b)for b in read['pack']['bodies']),'parent end replaced/lost original job')
            previous={'job':retry['job']}
        actual=load(folder,'Claude/summary.json');require(actual['actual_host_runs']==2 and actual['actual_tool_results']==sum(r['actual_tool_results']for r in actual['observations'])
                and actual['same_retained_job_after_restart']is True and actual['checked_noop_on_periodic_restart']is True
                and actual['real_model_calls']==actual['real_model_cost_USD']==0 and actual['independent_acceptance']is False,'actual CLI/quality scope differs')
        actual_results=[]
        for case in('updates-full','updates-restart'):
            observed=load(folder,'Claude/'+case+'/observation.json');steps=load(folder,'Claude/'+case+'/actual-tool-results.json');invoke=load(folder,'Claude/'+case+'/invocation.json')
            require(observed['host_exit_code']==observed['manager_exit_code']==0 and observed['manager_forced_kill']is False
                    and observed['plugin_registered']and observed['skill_registered']and observed['agent_definition_registered']and observed['manual_skill_expanded']
                    and observed['agent_autonomy_or_model_quality_verified']is False and observed['DB_published']is False
                    and observed['real_model_calls']==0 and observed['independent_acceptance']is False and observed['actual_tool_results']==len(steps),'actual CLI outcome inflated')
            require(invoke['env']['CKS_MANAGER_MODE']=='updates'and urlparse(invoke['env']['ANTHROPIC_BASE_URL']).hostname=='127.0.0.1'
                    and invoke['credential_kind']=='constant_fake_local_token'and invoke['real_model_calls']==0,'real/remote inference used')
            frames=[json.loads(line)for line in raw(folder,'Claude/'+case+'/mcp-frames.jsonl').splitlines()]
            starts=[r for r in frames if r['kind']=='start'];exits=[r for r in frames if r['kind']=='exit']
            require(len(starts)==len(exits)==1 and exits[0]['exit_code']==0 and exits[0]['forced_kill']is False,'actual manager EOF failed')
            requests=[json.loads(r['raw'])for r in frames if r['kind']=='request'];replies={json.loads(r['raw'])['id']:json.loads(r['raw'])for r in frames if r['kind']=='response'and'id'in json.loads(r['raw'])}
            wire=[r for r in requests if r.get('method')=='tools/call']
            require(len(wire)==len(steps),'actual host tool result omitted')
            for request,step in zip(wire,steps,strict=True):
                require(request['params']['name']==step['tool']and replies[request['id']]['result']['structuredContent']==step['value']and step['is_error']is False,'scripted API substituted native tool value')
            actual_results.extend(steps)
        require(len(actual_results)==actual['actual_tool_results'] and any(s['tool']=='cks.manager.update_read'and any('return 42'in json.dumps(b)for b in s['value']['pack']['bodies'])for s in actual_results),'actual CLI source not read')
        restart_flow=load(folder,'Claude/updates-restart/workflow.json');full_flow=load(folder,'Claude/updates-full/workflow.json')
        require(restart_flow['job']['job_id']==full_flow['job']['job_id']and restart_flow['actual_noop_status']['phase']=='no_op'
                and restart_flow['actual_noop_status']['attempts_recorded']==1,'actual CLI restart duplicated the job')
        for name in('MCP-fixture.tar','Claude-fixture.tar'):
            with tarfile.open(fileobj=io.BytesIO(raw(folder,name)))as archive:
                names=set();links={}
                for member in archive:
                    path=PurePosixPath(member.name);require(not path.is_absolute()and'..'not in path.parts and member.name not in names,'unsafe fixture');names.add(member.name)
                    require(member.isfile()or member.isdir()or member.issym()and member.name=='fixture/dataset/current','unsafe fixture type')
                    if member.issym():links[member.name]=member.linkname
                require(links==({'fixture/dataset/current':previous['job']['candidate']['version']}if name=='MCP-fixture.tar'else{}),'final fixture pointer differs')
                require(not any('/workflows/'in n for n in names),'review/approval manufactured')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('ebec0858')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'C1 immutable proof changed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==15 and progress['implementation_completed']==3 and [s['status']for s in item['subtasks']]==['complete','complete','complete','not_started']
                and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'integrated updater marked final periodic/full/quality complete')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager updates MCP:','FAIL'if errors else'PASS; KM04.3 integrated; final periodic/multi-project and quality remain separate')
    raise SystemExit(bool(errors))
