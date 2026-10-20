#!/usr/bin/env python3
"""Actual native update/IO reconciliation; sustained schedule/host kill are separate."""
import argparse
import copy
import json
from pathlib import Path
import subprocess
import sys
import time
from unittest.mock import patch

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
import update_contract as contract
import update_worker as worker


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',required=True,type=Path)
    out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    checks,calls,connections=[],[],[]
    def write(n,v):(out/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    def check(label,v):
        manager.require(v,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    class TracedNative(worker.Native):
        def __init__(self,r,config,**kwargs):
            self.trace_id=len(connections)+1;self.trace_config=str(config);self.trace_closed=False
            connections.append({'id':self.trace_id,'config':str(config),'EOF_exit_code':None});write('connections.json',connections)
            super().__init__(r,config,**kwargs)
        def request(self,method,params):
            result=super().request(method,params)
            calls.append({'session':self.trace_id,'id':self.id,'method':method,'params':copy.deepcopy(params),'result':copy.deepcopy(result)})
            write('native-calls.json',calls);return result
        def close(self):
            result=super().close()
            if not self.trace_closed:
                self.trace_closed=True;connections[self.trace_id-1]['EOF_exit_code']=result;write('connections.json',connections)
            return result
    source=out/'source';source.mkdir();code=source/'value.go';code.write_text('package fixture\nfunc Value()int{return 7}\n')
    (source/'go.mod').write_text('module example.invalid/update-worker\n\ngo 1.26.9\n');(source/'old.md').write_text('Old public design observation.\n');(source/'.env').write_text('PRIVATE_WORKER_SOURCE_SENTINEL\n')
    graph=out/'owned-graph.py';graph.write_text('#!'+str(Path(sys.executable).resolve())+'\nimport os,sys\nfrom pathlib import Path\n'
        'if len(sys.argv)>1 and sys.argv[1]=="build" and Path('+repr(str(out/'fail-graph'))+').exists():sys.exit(23)\n'
        'os.execv('+repr(str(ROOT/'bin/ckg'))+',['+repr(str(ROOT/'bin/ckg'))+']+sys.argv[1:])\n');graph.chmod(0o700)
    manager.register(argparse.Namespace(source=str(source),dataset=str(out/'dataset'),state_dir=str(out/'manager'),cks_bin=str(ROOT/'bin/cks'),graph_bin=str(graph),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',source_mode='snapshot-only',project_id=None))
    registration=str(out/'manager/registration.json');r=manager.load_registration(registration);policy=contract.defaults(r)
    policy.update(enabled=True,triggers=['manual','periodic','change'],interval_seconds=1,activation='activate_after_gate',deadline_seconds=120)
    policy['retry']={'max_attempts':2,'initial_seconds':1,'max_seconds':2};write('policy.json',policy)
    argv=[sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'configure-updates','--registration',registration,'--policy-file',str(out/'policy.json')]
    configured=subprocess.run(argv,capture_output=True,timeout=30);write('configure-command.json',{'argv':argv,'exit_code':configured.returncode,'stdout':configured.stdout.decode(),'stderr':configured.stderr.decode()})
    check('actual configure pins an enabled policy without creating a DB',configured.returncode==0 and not(out/'dataset').exists())
    w=None
    def settled(trigger='manual'):
        w.tick(trigger);end=time.monotonic()+60
        while w.journal.state['pending']:
            manager.require(time.monotonic()<end,'worker_fixture_timeout');time.sleep(.04);w.tick(trigger)
        return copy.deepcopy(w.journal.row(w.journal.state['last_outcome']))
    def state_file(n):write(n,w.describe())
    try:
        with patch.object(worker,'Native',TracedNative):
            w=worker.Worker(registration);first=settled();write('v1-attempt.json',first);pack1,_=w.read(first,first['job'],'Value observed source');write('v1-read.json',pack1)
            check('actual worker publishes v1 only after gates and scoped read',first['outcome']=='succeeded' and first['job']['activated']is True and pack1['coordinates']==first['job']['candidate']['coordinates'] and any('return 7'in json.dumps(b)for b in pack1['bodies']))
            count=len(w.journal.state['attempts']);w.tick('manual');state_file('noop.json')
            check('same bytes are no-op only after native status inspection and read',w.journal.state['phase']=='no_op' and len(w.journal.state['attempts'])==count)
            code.write_text('package fixture\nfunc Value()int{return 8}\n');(source/'old.md').unlink();second=settled('manual');write('v2-attempt.json',second);pack2,_=w.read(second,second['job'],'Value observed source');write('v2-read.json',pack2)
            check('changed code and deleted source produce a distinct actual v2',second['input_id']!=first['input_id'] and second['outcome']=='succeeded' and any('return 8'in json.dumps(b)for b in pack2['bodies']))
            old,_=w.read(first,first['job'],'Value observed source');write('v1-retained-read.json',old)
            check('retained v1 still returns original source and citations',old['bodies']==pack1['bodies'] and old['citations']==pack1['citations'])
            source2=Path(r['state_root'])/'updates/inputs'/second['input_id']/'project'
            check('latest immutable project drops deleted evidence while original input retains it',not(source2/'old.md').exists() and (Path(r['state_root'])/'updates/inputs'/first['input_id']/'project/old.md').exists())
            (out/'fail-graph').write_text('controlled actual engine failure\n');code.write_text('package fixture\nfunc Value()int{return 9}\n');failed=settled();write('failed-attempt.json',failed)
            check('actual native failure preserves previous published version and last success',failed['outcome']=='failed' and failed['job']['activated']is False and w.journal.state['last_success']==second['id'] and (out/'dataset/current').readlink()==Path(second['request']['new_version']))
            (out/'fail-graph').unlink();retry_at=w.journal.state['retry_at'];time.sleep(max(0,min(2,retry_at-time.time()+.01)))
            recovered=settled('periodic');write('retry-attempt.json',recovered);pack3,_=w.read(recovered,recovered['job'],'Value observed source');write('v3-read.json',pack3)
            check('due periodic retry uses new key version and linked same input',recovered['outcome']=='succeeded' and recovered['previous_attempt']==failed['id'] and recovered['attempt']==2 and recovered['input_id']==failed['input_id'] and recovered['request']['idempotency_key']!=failed['request']['idempotency_key'])
            check('recovered publication actually reads code9',any('return 9'in json.dumps(b)for b in pack3['bodies']) and (out/'dataset/current').readlink()==Path(recovered['request']['new_version']))
            code.write_text('package fixture\nfunc Value()int{return 10}\n');original=w.tool;ack=[]
            def lose(bundle,name,args):
                result=original(bundle,name,args)
                if name=='cks.dataset.build':ack.append(result);raise manager.Failure('native_unavailable')
                return result
            try:
                with patch.object(w,'tool',side_effect=lose):w.tick('manual')
            except manager.Failure as error:manager.require(str(error)=='native_unavailable','wrong injected transport result')
            else:raise ValueError('response loss not injected')
            pending=copy.deepcopy(w.journal.row());write('lost-ack.json',ack[0]);write('uncertain-intent.json',pending);state_file('before-restart.json')
            check('actual accepted response loss retains unacknowledged exact request',pending['job']is None and len(ack)==1 and pending['request_digest']==ack[0]['request_digest'])
            w.close();code.write_text('package fixture\nfunc Value()int{return 11}\n');w=worker.Worker(registration)
            w.tick('manual');end=time.monotonic()+30
            while w.journal.state['pending']:
                manager.require(time.monotonic()<end,'replay timeout');time.sleep(.03);w.tick('manual')
            replayed=copy.deepcopy(w.journal.row(pending['id']));write('replayed-attempt.json',replayed)
            check('restart resolves same native job before collecting changed source',replayed['job']['job_id']==ack[0]['job_id'] and replayed['request']==pending['request'] and replayed['input_id']==pending['input_id'] and len(w.journal.state['attempts'])==5)
            wait_at=w.journal.state['retry_at'];time.sleep(max(0,min(2,wait_at-time.time()+.01)) if wait_at is not None else 0)
            final=settled();write('final-attempt.json',final);pack4,_=w.read(final,final['job'],'Value observed source');write('final-read.json',pack4)
            check('later changed source is a separate cycle and healthy publication',final['cycle_id']==final['id'] and final['previous_attempt']is None and final['input_id']!=pending['input_id'] and final['outcome']=='succeeded' and any('return 11'in json.dumps(b)for b in pack4['bodies']))
            check('raw observations never create a review or approval',not(out/'manager/workflows').exists() and not(source/'.cks').exists())
            state_file('final-state.json');w.close();write('stopped-state.json',w.describe())
            check('all actual native sessions close normally without detached scheduling',all(c['EOF_exit_code']==0 for c in connections) and w.describe()['scheduler_running']is False)
    finally:
        if w:w.close()
    write('summary.json',{'schema_version':1,'scope':'KM04_3_B2_actual_foreground_native_worker_not_sustained_schedule_or_killed_host',
          'checks':len(checks),'native_RPC_observations':len(calls),'native_sessions':len(connections),'native_EOF_exit_codes':[c['EOF_exit_code']for c in connections],
          'actual_build_jobs':6,'actual_published_at_least':4,'exact_admission_replay_observed':True,'periodic_due_retry_observed':True,
          'sustained_periodic_schedule_verified':False,'actual_worker_host_SIGKILL_verified':False,'full_KM04_3_complete':False,'full_KM04_4_complete':False,
          'paid_model_calls':0,'embedding':'mock_lifecycle_not_quality','independent_acceptance':False})


if __name__=='__main__':main()
