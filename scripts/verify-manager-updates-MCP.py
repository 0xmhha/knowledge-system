#!/usr/bin/env python3
"""Actual updates MCP, concurrent retained reads, owner EOF/kill and exact recovery."""
import argparse
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import time

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
import update_state as state


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True)
    out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    checks,calls,sessions,process_observations=[],[],[],[]
    def write(n,v):(out/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    def check(label,value):
        manager.require(value,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    spec=importlib.util.spec_from_file_location('updates_MCP_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)
    source=out/'source';source.mkdir();code=source/'value.go';code.write_text('package fixture\nfunc Value()int{return 7}\n');(source/'go.mod').write_text('module example.invalid/updates-MCP\n\ngo 1.26.9\n')
    (source/'old.md').write_text('Original public source note\n');(source/'.env').write_text('PRIVATE_UPDATES_MCP_SENTINEL\n')
    graph=out/'owned-graph.py';graph.write_text('#!'+str(Path(sys.executable).resolve())+'\nimport json,os,sys,time\nfrom pathlib import Path\nroot=Path('+repr(str(out))+')\n'
        'if len(sys.argv)>1 and sys.argv[1]=="build" and(root/"controlled-delay").exists():\n'
        ' label=(root/"controlled-delay").read_text().strip()\n (root/("graph-"+label+".json")).write_text(json.dumps({"pid":os.getpid(),"started_monotonic":time.monotonic()}))\n'
        ' time.sleep(8)\n (root/("late-"+label)).write_text("late graph escaped owner")\n'
        'os.execv('+repr(str(ROOT/'bin/ckg'))+',['+repr(str(ROOT/'bin/ckg'))+']+sys.argv[1:])\n');graph.chmod(0o700)
    proxy=out/'owned-cks.py';proxy.write_text('#!'+str(Path(sys.executable).resolve())+'\nimport json,os,sys\nfrom pathlib import Path\n'
        'if len(sys.argv)>1 and sys.argv[1]=="mcp":\n with Path('+repr(str(out/'native-processes.jsonl'))+').open("a")as stream:stream.write(json.dumps({"pid":os.getpid(),"argv":sys.argv[1:]})+"\\n")\n'
        'os.execv('+repr(str(ROOT/'bin/cks'))+',['+repr(str(ROOT/'bin/cks'))+']+sys.argv[1:])\n');proxy.chmod(0o700)
    manager.register(argparse.Namespace(source=str(source),dataset=str(out/'dataset'),state_dir=str(out/'manager'),cks_bin=str(proxy),graph_bin=str(graph),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
    registration=str(out/'manager/registration.json');r=manager.load_registration(registration)
    def connect(label):
        folder=out/label;folder.mkdir();client=wire.Client(None,None,folder,command=[sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'serve-updates','--registration',registration],namespace='')
        sessions.append(client);init=client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-updates-MCP','version':'1'}})
        manager.require(init['result']['serverInfo']['name']=='knowledge-manager-updates','wrong MCP mode');return client
    def tool(client,operation,args=None,error=None):
        name='cks.manager.update_'+operation if operation!='legacy'else'cks.ops.index'
        response=client.request('tools/call',{'name':name,'arguments':args or{}})['result']
        calls.append({'session':client.root.name,'tool':name,'arguments':copy.deepcopy(args or{}),'response':copy.deepcopy(response),'expected_error':error});write('calls.json',calls)
        if error:
            manager.require(response.get('isError')is True and json.loads(response['content'][0]['text'])['code']==error,'wrong actual MCP refusal');return response
        manager.require(not response.get('isError'),'actual MCP refusal: '+str(response));return response['structuredContent']
    def wait_success(client,prior=None):
        deadline=time.monotonic()+60
        while True:
            result=tool(client,'status')
            if result['last_success']and result['last_success']['intent_id']!=prior:return result
            manager.require(result['process']['exit_code']is None,'worker ended before actual success')
            manager.require(time.monotonic()<deadline,'actual success timeout');time.sleep(.05)
    def wait_end(client):
        deadline=time.monotonic()+60
        while True:
            result=tool(client,'status')
            if result['process']['running']is False:return result
            manager.require(time.monotonic()<deadline,'worker stop timeout');time.sleep(.05)
    def read(client,row,label,value):
        pack=tool(client,'read',{'job_id':row['job']['job_id'],'expected_coordinates':row['job']['candidate']['coordinates'],'prompt':'Value observed source'})
        write(label+'.json',pack);manager.require(any('return '+str(value)in json.dumps(b)for b in pack['pack']['bodies']),'wrong actual retained source');return pack
    def closed(client,killed=False):
        if killed:
            client.proc.kill();client.proc.wait(timeout=10)
            try:client.proc.stdin.close()
            except OSError:pass
            client.thread.join(timeout=2);client.stderr.close();client.stdin.close();client.stdout.close()
        else:client.close()
        client.proc.stdout.close()
    def proc_status(pid):
        result=subprocess.run(['ps','-p',str(pid),'-o','stat='],capture_output=True,text=True,timeout=2);value=result.stdout.strip()
        return {'pid':pid,'stat':value,'ps_exit_code':result.returncode,'running':result.returncode==0 and bool(value) and not value.startswith('Z')}
    try:
        client=connect('primary-session');listed=client.request('tools/list')['result']['tools'];write('tools.json',listed)
        check('actual updates mode exposes six tools without legacy writes',len(listed)==6 and all(t['name'].startswith('cks.manager.update_')for t in listed))
        policy=tool(client,'policy_template');check('actual template is disabled hold and scoped',policy['enabled']is False and policy['activation']=='hold_for_review' and policy['project_id']==r['project_id'])
        policy.update(enabled=True,triggers=['manual','periodic','change'],interval_seconds=1,activation='activate_after_gate',deadline_seconds=120)
        policy['retry']={'max_attempts':2,'initial_seconds':1,'max_seconds':2};write('policy.json',policy)
        configured=tool(client,'configure',{'policy_file':str(out/'policy.json')})
        check('configuration alone starts neither process nor dataset',configured['process']['running']is False and not(out/'dataset').exists())
        started=tool(client,'start',{'trigger':'periodic','max_iterations':10000,'max_seconds':90,'poll_seconds':.05});write('initial-start.json',started)
        check('actual start acknowledges an owned process without job inference',started['owner_stdin_EOF_enabled']is True and started['actual_job_or_read_success_inferred']is False)
        first=wait_success(client)['last_success'];write('v1.json',first);pack1=read(client,first,'v1-read',7)
        check('actual background worker publishes and reads scoped source7',first['outcome']=='succeeded' and pack1['current_selected_at_pointer_checks']is True and pack1['pack']['coordinates']==first['job']['candidate']['coordinates'])
        bad=copy.deepcopy(first['job']['candidate']['coordinates']);bad['project_id']='foreign'
        tool(client,'read',{'job_id':first['job']['job_id'],'expected_coordinates':bad,'prompt':'Value'},error='update_reader_coordinate_mismatch')
        tool(client,'legacy',error='unknown_tool');check('foreign coordinates and legacy mutation are actually refused',True)
        code.write_text('package fixture\nfunc Value()int{return 8}\n');(source/'old.md').unlink();second=wait_success(client,first['intent_id'])['last_success'];write('v2.json',second)
        pack2=read(client,second,'v2-read',8);old=read(client,first,'v1-during-update-read',7)
        check('same active MCP session updates code/deletion and preserves old reads',second['input_id']!=first['input_id'] and old['pack']['bodies']==pack1['pack']['bodies'] and old['pack']['citations']==pack1['pack']['citations'])
        deadline=time.monotonic()+20
        while True:
            status=tool(client,'status')
            if status['phase']=='no_op':break
            manager.require(time.monotonic()<deadline,'actual checked no-op timeout');time.sleep(.05)
        write('noop.json',status);check('periodic no-op uses one existing candidate without duplicate jobs',status['attempts_recorded']==2 and status['last_success']['intent_id']==second['intent_id'])
        stopped=tool(client,'stop');write('explicit-stop.json',stopped)
        check('explicit MCP stop ends only its owned worker normally',stopped['status']['process']['running']is False and stopped['status']['process']['exit_code']==0)
        previous=second
        for label,killed,value in [('parent-kill',True,9),('parent-EOF',False,10)]:
            code.write_text('package fixture\nfunc Value()int{return '+str(value)+'}\n');(out/'controlled-delay').write_text(label+'\n')
            count=len((out/'native-processes.jsonl').read_text().splitlines());start=tool(client,'start',{'trigger':'periodic','max_iterations':10000,'max_seconds':90,'poll_seconds':.03});write(label+'-start.json',start)
            marker=out/('graph-'+label+'.json');deadline=time.monotonic()+30
            while True:
                if marker.exists():
                    try:graph_record=json.loads(marker.read_text());break
                    except json.JSONDecodeError:pass
                manager.require(time.monotonic()<deadline,'controlled graph did not start');time.sleep(.03)
            before=state.read_state(out/'manager/updates/worker/state.json',r);pending=next(row for row in before['attempts']if row['id']==before['pending']);write(label+'-before.json',before)
            check(label+': actual native candidate starts with a durable scoped intent',pending['input_id']!=previous['input_id'])
            old_link=str((out/'dataset/current').readlink());native=[json.loads(line)for line in(out/'native-processes.jsonl').read_text().splitlines()[count:]]
            closed(client,killed)
            pids=[start['process']['pid'],graph_record['pid'],*[row['pid']for row in native]]
            deadline=time.monotonic()+25
            while any(proc_status(pid)['running']for pid in pids):
                manager.require(time.monotonic()<deadline,'owned child survived parent end');time.sleep(.05)
            while time.monotonic()<graph_record['started_monotonic']+9:time.sleep(.1)
            observation={'label':label,'manager_exit_code':client.proc.returncode,'observed_monotonic':time.monotonic(),'graph':graph_record,
                'processes':[proc_status(pid)for pid in pids],'native_processes':native,'late_write_exists':(out/('late-'+label)).exists(),
                'current_before':old_link,'current_after':str((out/'dataset/current').readlink()),'signal_used_for_recovered_PIDs':None}
            process_observations.append(observation);write('process-observations.json',process_observations)
            check(label+': owner stdin loss stops worker/native/graph with no late swap',client.proc.returncode==(-9 if killed else 0) and not observation['late_write_exists'] and old_link==observation['current_after'] and not any(row['running']for row in observation['processes']))
            client=connect(label+'-recovery-session');old_status=tool(client,'status');write(label+'-restart-status.json',old_status)
            check(label+': restart preserves unknown prior process ownership',old_status['process']['owned_by_this_host']is False and old_status['process']['previous_unowned_pid_never_signaled']is True and old_status['pending']['intent_id']==pending['id'])
            tool(client,'start',{'trigger':'manual','max_iterations':1,'max_seconds':60,'poll_seconds':.03});settled=wait_end(client);write(label+'-reconciled-status.json',settled)
            ledger=state.read_state(out/'manager/updates/worker/state.json',r);canceled=next(row for row in ledger['attempts']if row['id']==pending['id']);write(label+'-canceled.json',canceled)
            check(label+': original native cancellation is observed before a new admission',canceled['request']==pending['request'] and canceled['outcome']=='canceled' and canceled['job']['activated']is False and settled['attempts_recorded']==len(before['attempts']) and settled['last_success']['intent_id']==previous['intent_id'])
            (out/'controlled-delay').unlink();at=ledger['retry_at'];time.sleep(max(0,min(2,at-time.time()+.01)))
            tool(client,'start',{'trigger':'manual','max_iterations':1,'max_seconds':60,'poll_seconds':.03});success=wait_success(client,previous['intent_id'])['last_success'];wait_end(client)
            recovered=state.read_state(out/'manager/updates/worker/state.json',r);row=next(a for a in recovered['attempts']if a['id']==success['intent_id']);write(label+'-recovered.json',row);pack=read(client,success,label+'-recovered-read',value)
            check(label+': linked retry publishes and actually reads the changed source',row['outcome']=='succeeded' and row['previous_attempt']==pending['id'] and row['attempt']==2 and row['input_id']==pending['input_id'] and pack['current_selected_at_pointer_checks']is True)
            previous=success
        final=tool(client,'stop');write('final-status.json',final);closed(client)
        check('final actual MCP EOF is clean and source receives no manager documentation',client.proc.returncode==0 and not(source/'.cks').exists() and not(out/'manager/workflows').exists())
    finally:
        for client in sessions:
            if client.proc.poll()is None:closed(client)
    write('summary.json',{'schema_version':1,'scope':'KM04_3_C2_actual_updates_MCP_worker_supervision_not_final_multi_project_periodic_acceptance',
          'checks':len(checks),'actual_tool_calls':len(calls),'actual_MCP_sessions':len(sessions),'manager_exit_codes':[c.proc.returncode for c in sessions],
          'actual_parent_termination_cases':2,'actual_linked_recovery_publications':2,'same_session_update_and_retained_read_verified':True,
          'actual_CLI_Claude_verified_here':False,'full_KM04_3_complete':False,'full_KM04_4_complete':False,
          'paid_model_calls':0,'mock_embedding_not_quality':True,'independent_acceptance':False})


if __name__=='__main__':main()
