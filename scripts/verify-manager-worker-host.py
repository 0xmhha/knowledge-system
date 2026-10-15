#!/usr/bin/env python3
"""Actual foreground worker SIGTERM/SIGKILL/budget and native quiescence."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
import update_contract as contract
import update_state as state
import update_worker as worker


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True)
    out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    checks,commands,observations=[],[],[]
    def write(n,v):(out/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    def check(label,v):
        manager.require(v,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    source=out/'source';source.mkdir();code=source/'value.go';code.write_text('package fixture\nfunc Value()int{return 7}\n');(source/'go.mod').write_text('module example.invalid/worker-host\n\ngo 1.26.9\n')
    graph=out/'owned-delayed-graph.py';graph.write_text('#!'+str(Path(sys.executable).resolve())+'\nimport json,os,sys,time\nfrom pathlib import Path\nroot=Path('+repr(str(out))+')\n'
        'if len(sys.argv)>1 and sys.argv[1]=="build" and (root/"controlled-delay").exists():\n'
        ' label=(root/"controlled-delay").read_text().strip()\n'
        ' (root/("graph-"+label+".json")).write_text(json.dumps({"pid":os.getpid(),"started_monotonic":time.monotonic()}))\n'
        ' time.sleep(8)\n (root/("late-"+label)).write_text("late writer escaped native ownership")\n'
        'os.execv('+repr(str(ROOT/'bin/ckg'))+',['+repr(str(ROOT/'bin/ckg'))+']+sys.argv[1:])\n');graph.chmod(0o700)
    proxy=out/'owned-cks-proxy.py';proxy.write_text('#!'+str(Path(sys.executable).resolve())+'\nimport json,os,sys,time\nfrom pathlib import Path\n'
        'if len(sys.argv)>1 and sys.argv[1]=="mcp":\n'
        ' with Path('+repr(str(out/'native-processes.jsonl'))+').open("a") as stream: stream.write(json.dumps({"pid":os.getpid(),"argv":sys.argv[1:],"at":time.time()})+"\\n")\n'
        'os.execv('+repr(str(ROOT/'bin/cks'))+',['+repr(str(ROOT/'bin/cks'))+']+sys.argv[1:])\n');proxy.chmod(0o700)
    manager.register(argparse.Namespace(source=str(source),dataset=str(out/'dataset'),state_dir=str(out/'manager'),cks_bin=str(proxy),graph_bin=str(graph),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',source_mode='snapshot-only',project_id=None))
    registration=str(out/'manager/registration.json');r=manager.load_registration(registration);policy=contract.defaults(r);policy.update(enabled=True,triggers=['manual','periodic','change'],interval_seconds=1,activation='activate_after_gate',deadline_seconds=120)
    policy['retry']={'max_attempts':2,'initial_seconds':1,'max_seconds':2}
    with state.Journal(registration) as journal:journal.configure(policy)
    write('policy.json',policy)
    def argv(seconds=60):return [sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'run-updates','--registration',registration,'--trigger','manual','--max-iterations','1','--max-seconds',str(seconds),'--poll-seconds','.02']
    def run(label):
        args=argv();result=subprocess.run(args,capture_output=True,timeout=70)
        commands.append({'label':label,'argv':args,'exit_code':result.returncode,'stdout':result.stdout.decode(),'stderr':result.stderr.decode()});write('commands.json',commands)
        manager.require(result.returncode==0,'worker_cli_failed_'+label)
        return json.loads(result.stdout)
    initial=run('baseline');write('baseline.json',initial)
    first=next(row for row in initial['state']['attempts'] if row['id']==initial['state']['last_success'])
    check('baseline actual CLI publishes a healthy retained DB',initial['state']['phase']=='succeeded' and (out/'dataset/current').readlink()==Path(first['request']['new_version']))
    previous=first
    def status(pid):
        result=subprocess.run(['ps','-p',str(pid),'-o','stat='],capture_output=True,text=True,timeout=2)
        value=result.stdout.strip();return {'pid':pid,'ps_exit_code':result.returncode,'stat':value,'running':result.returncode==0 and bool(value) and not value.startswith('Z')}
    for label,number,source_value in [('sigterm',signal.SIGTERM,8),('sigkill',signal.SIGKILL,9),('budget',None,10)]:
        code.write_text('package fixture\nfunc Value()int{return '+str(source_value)+'}\n');(out/'controlled-delay').write_text(label+'\n')
        native_before=len((out/'native-processes.jsonl').read_text().splitlines())
        args=argv(3 if label=='budget' else 60);child=subprocess.Popen(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        try:
            marker=out/('graph-'+label+'.json');end=time.monotonic()+25
            while True:
                if marker.exists():
                    try:
                        graph_info=json.loads(marker.read_text());break
                    except json.JSONDecodeError:pass
                manager.require(child.poll()is None,'worker_ended_before_controlled_graph_'+label)
                manager.require(time.monotonic()<end,'controlled_graph_not_started_'+label);time.sleep(.03)
            before=state.read_state(out/'manager/updates/worker/state.json',r)
            pending=next(row for row in before['attempts'] if row['id']==before['pending'])
            write(label+'-before.json',before)
            current_before=str((out/'dataset/current').readlink())
            check(label+': actual immutable candidate build starts with durable intent',pending['input_id']!=previous['input_id'] and current_before==previous['request']['new_version'])
            if number is not None:os.kill(child.pid,number)
            stdout,stderr=child.communicate(timeout=35)
            commands.append({'label':label,'argv':args,'signal':None if number is None else number.name,'exit_code':child.returncode,'stdout':stdout.decode(),'stderr':stderr.decode()});write('commands.json',commands)
            expected=-9 if label=='sigkill' else 0
            check(label+': actual worker exits according to requested lifecycle',child.returncode==expected)
            if label!='sigkill':
                result=json.loads(stdout);write(label+'-stopped.json',result)
                uncertain=result['disk_state_requires_reconciliation']
                check(label+': normal stop reports pending reconciliation not canceled success',result['state']['pending']==pending['id']
                    and result['stop_reason']==('budget' if label=='budget' else 'signal')
                    and (uncertain and result['worker_stop_outcome']=='closed_requires_persistence_reconciliation'
                         and result['state_snapshot_origin']=='last_confirmed_in_process_state'
                         or not uncertain and result['state']['phase']=='stopped' and result['worker_stop_outcome']=='closed_with_pending_intent'))
            native=[json.loads(line)for line in(out/'native-processes.jsonl').read_text().splitlines()[native_before:]]
            manager.require(native,'missing native process observation')
            pids=[row['pid']for row in native]+[graph_info['pid']]
            until=time.monotonic()+20
            while any(status(pid)['running']for pid in pids):
                manager.require(time.monotonic()<until,'owned_process_survived_'+label);time.sleep(.05)
            # Wait beyond the controlled graph's late-write point; liveness
            # observations use signal0/ps only, never kill a recovered PID.
            until=graph_info['started_monotonic']+9
            while time.monotonic()<until:time.sleep(min(.1,until-time.monotonic()))
            after=state.read_state(out/'manager/updates/worker/state.json',r)
            observation={'label':label,'observed_monotonic':time.monotonic(),'worker_pid':child.pid,'worker_exit_code':child.returncode,'owned_native_processes':native,'graph':graph_info,
                'post_stop_process_observations':[status(pid)for pid in pids],'late_write_exists':(out/('late-'+label)).exists(),
                'current_before':current_before,'current_after':str((out/'dataset/current').readlink()),'native_exit_codes':'not_observable_after_parent_exit'}
            observations.append(observation);write('process-observations.json',observations);write(label+'-after.json',after)
            check(label+': native and graph quiesce without late writes or current swap',not observation['late_write_exists'] and not any(row['running']for row in observation['post_stop_process_observations']) and observation['current_before']==observation['current_after'])
            check(label+': exact intent survives without a second admission',next(row for row in after['attempts']if row['id']==pending['id'])['request']==pending['request'] and after['pending']==pending['id'])
            (out/'controlled-delay').unlink();reconciled=run(label+'-reconcile');write(label+'-reconciled.json',reconciled)
            original=next(row for row in reconciled['state']['attempts']if row['id']==pending['id'])
            check(label+': restart observes original terminal cancellation before any new source',original['outcome']=='canceled' and original['job']['terminal']is True and original['job']['activated']is False and reconciled['state']['last_success']==previous['id'] and len(reconciled['state']['attempts'])==len(before['attempts']))
            wait_at=reconciled['state']['retry_at'];time.sleep(max(0,min(2,wait_at-time.time()+.01)))
            recovered=run(label+'-retry');write(label+'-recovered.json',recovered)
            final=next(row for row in recovered['state']['attempts']if row['id']==recovered['state']['last_success'])
            with_worker=worker.Worker(registration)
            try:pack,current=with_worker.read(final,final['job'],'Value observed source')
            finally:with_worker.close()
            write(label+'-recovered-read.json',pack)
            check(label+': linked bounded retry publishes and reads new actual code',final['outcome']=='succeeded' and final['previous_attempt']==pending['id'] and final['attempt']==2 and final['input_id']==pending['input_id'] and current==final['job']['candidate']['coordinates'] and any('return '+str(source_value)in json.dumps(body)for body in pack['bodies']))
            previous=final
        finally:
            if child.poll()is None:child.kill();child.communicate(timeout=30)
            child.stdout.close();child.stderr.close()
    write('summary.json',{'schema_version':1,'scope':'KM04_3_C1_actual_foreground_worker_process_stop_kill_budget_not_MCP_operation_or_sustained_schedule',
        'checks':len(checks),'actual_worker_process_cases':3,'signals':['SIGTERM','SIGKILL','worker_budget'],'actual_CLI_commands':len(commands),
        'owned_processes_quiescent':True,'late_write_refusals':3,'actual_linked_recovery_publications':3,'actual_native_exit_codes':'not_observed_not_claimed_zero',
        'manager_MCP_worker_operation_verified':False,'sustained_periodic_schedule_verified':False,'full_KM04_3_complete':False,'full_KM04_4_complete':False,
        'paid_model_calls':0,'independent_acceptance':False})


if __name__=='__main__':main()
