"""Updates-only MCP session supervising one explicitly started foreground worker.

MCP stays responsive while the child owns the manager writer lease. Retained
reads use exact owned job coordinates, not the mutable maintenance backend.
Closing the parent's worker stdin is an ownership signal, including parent kill.
"""
import copy
import json
import math
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
import uuid

from manager import Failure,canonical,digest,load_registration,require,strict_read,sync_dir,write_new
from collection import domain_hash,now,owned_directory,writer
from update_contract import bounded,defaults
from update_state import Journal,default_state,read_state
from update_reader import read_observed
from update_native import code_inventory,check_code
from broker import COORDINATES,FRAME_LIMIT,GracefulShutdown,STRING,failure,parse,result,schema

PREFIX='cks.manager.update_'


def tools():
    definitions={
        'policy_template':('Return this registration\'s disabled/hold update policy. Does not start a worker.',{},[],True),
        'configure':('Pin an explicitly supplied scoped policy file. No DB build or scheduler starts.',{'policy_file':STRING},None,False),
        'status':('Read sealed manager state and this host\'s owned process status. Past publication is not current/read readiness.',{},[],True),
        'start':('Spawn one bounded foreground worker with an owner-stdin lease. Explicit policy controls triggers and publication. Spawn is not job success.',
                 {'trigger':{'type':'string','enum':['manual','periodic','change']},'max_seconds':{'type':'integer','minimum':1,'maximum':86400},
                  'max_iterations':{'type':'integer','minimum':1,'maximum':10000},'poll_seconds':{'type':'number','minimum':.01,'maximum':60}},['trigger'],False),
        'stop':('Close only this host\'s owned worker input and wait for process shutdown. Pending intent still requires native reconciliation.',{},[],False),
        'read':('Read an explicitly selected retained held/published job and exact expected coordinates while updates may run. Never fall back to current or another project.',
                {'job_id':STRING,'expected_coordinates':COORDINATES,'prompt':STRING},None,True),
    }
    return [{'name':PREFIX+name,'description':description,'inputSchema':schema(properties,required),
             'annotations':{'readOnlyHint':readonly,'destructiveHint':False,'idempotentHint':readonly,'openWorldHint':False}}
            for name,(description,properties,required,readonly)in definitions.items()]


class Host:
    def __init__(self,registration):
        self.registration=str(Path(registration).resolve());self.r=load_registration(self.registration,check_dataset=False)
        self.code_pin=code_inventory()
        parent=owned_directory(Path(self.r['state_root'])/'updates',self.r)
        self.directory=owned_directory(parent/'mcp-host',self.r)
        self.lease=writer(self.directory);self.lease.__enter__()
        self.closed=False;self.child=None;self.run_directory=None;self.last_run=None;self.finalized=False
        try:
            self.runs=owned_directory(self.directory/'runs',self.r)
            path=self.directory/'last-run.json'
            if path.exists() or path.is_symlink():
                v=strict_read(path);record=v.get('record',{})
                require(set(v)=={'record','checksum'} and v['checksum']==domain_hash('cks.manager-update-host-run.v1',record)
                        and record.get('project_id')==self.r['project_id'] and record.get('profile_id')==self.r['profile_id']
                        and record.get('registration_token')==self.r['token'] and isinstance(record.get('run_id'),str)
                        and re.fullmatch('[0-9a-f]{32}',record['run_id']),'update_host_state_invalid')
                self.last_run=record['run_id']
                self.load_run(self.last_run)
        except BaseException:
            self.close();raise

    def load_run(self,run_id):
        path=self.runs/run_id
        require(path.resolve()==path and not path.is_symlink() and strict_read(path/'owner.json')==
                {'schema_version':1,'project_id':self.r['project_id'],'token':self.r['token']},'update_host_state_invalid')
        record=strict_read(path/'launch.json')
        require(record.get('run_id')==run_id and record.get('registration_digest')==digest(self.r)
                and type(record.get('pid'))is int and record['pid']>0,'update_host_state_invalid')
        return path,record

    def ledger(self):
        # Read-only diagnostics may inspect an early admission without a root
        # owner. This neither claims that root nor starts a native process.
        require(load_registration(self.registration,check_dataset=False)==self.r,'registration_changed')
        path=Path(self.r['state_root'])/'updates/worker/state.json'
        if not path.exists() and not path.is_symlink():return default_state(self.r)
        require(path.resolve()==path,'update_state_invalid')
        return read_state(path,self.r)

    def observe_exit(self):
        if self.child is None or self.child.poll() is None or self.finalized:return
        self.finalized=True
        if self.child.stdin and not self.child.stdin.closed:self.child.stdin.close()
        record={'schema_version':1,'run_id':self.last_run,'pid':self.child.pid,'exit_code':self.child.returncode,'observed_at':now()}
        write_new(self.run_directory/'exit.json',record);sync_dir(self.run_directory)

    def child_report(self,path):
        stdout=path/'stdout.json'
        if not stdout.exists()or stdout.stat().st_size==0:return {'available':False}
        try:reported=strict_read(stdout,16*1024*1024)
        except (OSError,ValueError,Failure):return {'available':False,'error':'worker_report_unavailable'}
        require(isinstance(reported,dict),'update_host_state_invalid')
        return {'available':True,**{key:reported.get(key)for key in('stage','stop_reason','worker_stop_outcome',
            'disk_state_requires_reconciliation','state_snapshot_origin','state_not_read_by_process')}}

    def process(self):
        self.observe_exit()
        if self.last_run is None:return {'owned_by_this_host':False,'running':False,'run_id':None}
        path,launch=self.load_run(self.last_run)
        exit_path=path/'exit.json';exit_record=strict_read(exit_path)if exit_path.exists()or exit_path.is_symlink()else None
        if exit_record is not None:
            require(exit_record.get('run_id')==self.last_run and exit_record.get('pid')==launch['pid']
                    and type(exit_record.get('exit_code'))is int,'update_host_state_invalid')
        owned=self.child is not None
        return {'owned_by_this_host':owned,'run_id':self.last_run,'pid':launch['pid'],
                'running':self.child.poll()is None if owned else False if exit_record is not None else None,
                'exit_code':self.child.returncode if owned else exit_record['exit_code']if exit_record is not None else None,
                'previous_unowned_pid_never_signaled':not owned,'native_quiescence_not_inferred_from_parent_exit':True,'worker_report':self.child_report(path)}

    def compact(self,row):
        if row is None:return None
        job=row['job']
        return {'intent_id':row['id'],'input_id':row['input_id'],'attempt':row['attempt'],'outcome':row['outcome'],
                'job':None if job is None else {k:copy.deepcopy(job.get(k))for k in
                    ('job_id','state','phase','revision','terminal','build_complete','activated','candidate','gates','error')},
                'original_request_preserved_on_disk':True}

    def status(self):
        ledger=self.ledger();rows={r['id']:r for r in ledger['attempts']};process=self.process()
        return {'schema_version':1,'stage':'updates_MCP_status_not_current_readiness','mode':'updates',
                'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],'process':process,
                'ledger_revision':ledger['revision'],'phase':ledger['phase'],'configured_policy':ledger['policy'],
                'last_checked':ledger['last_checked'],'retry_at':ledger['retry_at'],'last_error':ledger['last_error'],
                'pending':self.compact(rows.get(ledger['pending'])),'last_success':self.compact(rows.get(ledger['last_success'])),
                'last_outcome':self.compact(rows.get(ledger['last_outcome'])),'attempts_recorded':len(rows),
                'snapshot_origin':'sealed_disk_state_as_of_this_call','publication_or_serving_read_inferred':False,
                'native_reconciliation_required':ledger['pending']is not None and process['running']is not True,
                'background_OS_scheduler_installed':False,'independent_acceptance':False}

    def ensure_idle(self):
        self.observe_exit();require(self.child is None or self.child.poll()is not None,'update_worker_already_running')

    def configure(self,path):
        self.ensure_idle();check_code(self.code_pin)
        with Journal(self.registration)as journal:journal.configure(strict_read(Path(path)))
        return self.status()

    def start(self,args):
        self.ensure_idle();check_code(self.code_pin)
        trigger=args['trigger'];seconds=args.get('max_seconds',3600);iterations=args.get('max_iterations',1 if trigger=='manual'else 10000);poll=args.get('poll_seconds',1)
        require(trigger in ('manual','periodic','change') and bounded(seconds,1,86400) and bounded(iterations,1,10000)
                and type(poll)in(int,float) and math.isfinite(poll) and .01<=poll<=60,'update_worker_budget_invalid')
        with Journal(self.registration,recover=True)as journal:
            journal.broker_idle();policy=journal.policy()
            require(journal.state['pending']is not None or policy['enabled'] and trigger in policy['triggers'],'update_trigger_not_enabled')
            if not policy['enabled']:iterations=1
        require(len([p for p in self.runs.iterdir()if p.is_dir()])<128,'update_host_run_limit')
        run_id=uuid.uuid4().hex;directory=owned_directory(self.runs/run_id,self.r)
        argv=[str(Path(sys.executable).resolve()),str(Path(__file__).resolve().parent/'manager.py'),'run-updates','--registration',self.registration,
              '--trigger',trigger,'--max-seconds',str(seconds),'--max-iterations',str(iterations),'--poll-seconds',str(poll),'--stop-on-stdin-eof']
        stdout,stderr=directory/'stdout.json',directory/'stderr.txt'
        try:
            with stdout.open('xb')as output,stderr.open('xb')as errors:
                os.chmod(stdout,0o600);os.chmod(stderr,0o600)
                self.child=subprocess.Popen(argv,stdin=subprocess.PIPE,stdout=output,stderr=errors)
            self.run_directory=directory;self.last_run=run_id;self.finalized=False
            write_new(directory/'launch.json',{'schema_version':1,'run_id':run_id,'registration_digest':digest(self.r),'pid':self.child.pid,
                    'argv':argv,'created_at':now(),'policy_digest':domain_hash('cks.manager-update-policy.v1',policy)})
            sync_dir(directory)
            record={'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],'registration_token':self.r['token'],'run_id':run_id}
            stage=self.directory/('.pending-'+uuid.uuid4().hex);write_new(stage,{'record':record,'checksum':domain_hash('cks.manager-update-host-run.v1',record)})
            stage.replace(self.directory/'last-run.json');sync_dir(self.directory)
        except BaseException:
            self.stop_owned();raise
        return {'schema_version':1,'stage':'foreground_worker_process_spawned_not_job_success','process':self.process(),
                'owner_stdin_EOF_enabled':True,'actual_job_or_read_success_inferred':False}

    def stop_owned(self):
        if self.child is None:return
        if self.child.poll()is None:
            try:self.child.stdin.close()
            except OSError:pass
            try:self.child.wait(timeout=45)
            except subprocess.TimeoutExpired:
                self.child.terminate()
                try:self.child.wait(timeout=5)
                except subprocess.TimeoutExpired:self.child.kill();self.child.wait(timeout=5)
        self.observe_exit()

    def read(self,args):
        check_code(self.code_pin);ledger=self.ledger()
        selected=next((row for row in ledger['attempts']if row['job']and row['job']['job_id']==args['job_id']),None)
        require(selected is not None and selected['outcome']in('held','succeeded'),'update_read_selection_required')
        require(set(args['expected_coordinates'])==set(COORDINATES['properties'])
                and args['expected_coordinates']==selected['job']['candidate']['coordinates'],'update_reader_coordinate_mismatch')
        pack,current=read_observed(self.r,selected,selected['job'],self.directory/'readers',args['prompt'],60)
        return {'schema_version':1,'stage':'actual_retained_update_read','job_id':args['job_id'],'pack':pack,
                'current_selected_at_pointer_checks':current==pack['coordinates'],'semantic_review':'not_performed',
                'publication_inferred_from_read':False}

    def call(self,name,args):
        require(not self.closed and isinstance(args,dict),'invalid_request')
        definitions={t['name']:t['inputSchema']for t in tools()};require(name in definitions,'unknown_tool')
        shape=definitions[name];require(set(shape['required'])<=set(args)<=set(shape['properties']),'invalid_request')
        for key,item in args.items():
            kind=shape['properties'][key]['type']
            require(isinstance(item,str)and 0<len(item)<=4096 if kind=='string'else type(item)is int if kind=='integer'
                    else type(item)in(int,float)and math.isfinite(item)if kind=='number'else isinstance(item,dict),'invalid_request')
        operation=name.removeprefix(PREFIX)
        if operation=='policy_template':return defaults(self.r)
        if operation=='status':return self.status()
        if operation=='configure':return self.configure(args['policy_file'])
        if operation=='start':return self.start(args)
        if operation=='read':return self.read(args)
        self.stop_owned();return {'schema_version':1,'stage':'owned_worker_process_ended_native_state_requires_status','status':self.status()}

    def close(self):
        if self.closed:return
        self.closed=True
        try:self.stop_owned()
        finally:self.lease.__exit__(None,None,None)


def serve(registration):
    host,previous,stopping,finishing=None,{},False,False
    def stop(_number,_frame):
        nonlocal stopping
        if not stopping:
            stopping=True
            if not finishing:raise GracefulShutdown()
    try:
        for number in(signal.SIGINT,signal.SIGTERM):previous[number]=signal.signal(number,stop)
        host=Host(registration)
        while True:
            data=sys.stdin.buffer.readline(2*1024*1024+1)
            if not data:break
            require(len(data)<=2*1024*1024 and data.endswith(b'\n'),'request_limit')
            request=parse(data);require(isinstance(request,dict)and request.get('jsonrpc')=='2.0'and isinstance(request.get('method'),str),'invalid_request')
            if 'id'not in request:continue
            method,params=request['method'],request.get('params',{})
            try:
                if method=='initialize':payload={'protocolVersion':'2024-11-05','capabilities':{'tools':{}},'serverInfo':{'name':'knowledge-manager-updates','version':'0.6.0'}}
                elif method=='ping':payload={}
                elif method=='tools/list':payload={'tools':tools()}
                elif method=='tools/call':payload=result(host.call(params['name'],params.get('arguments',{})))
                else:raise Failure('unknown_method')
                require(len(canonical(payload))<FRAME_LIMIT-128,'response_limit')
            except Failure as error:payload=failure(str(error))
            except(OSError,ValueError,KeyError,TypeError,RecursionError,subprocess.SubprocessError):payload=failure('operation_failed')
            sys.stdout.buffer.write(canonical({'jsonrpc':'2.0','id':request['id'],'result':payload})+b'\n');sys.stdout.buffer.flush()
    except GracefulShutdown:pass
    finally:
        finishing=True
        try:
            if host:host.close()
        finally:
            for number,handler in previous.items():signal.signal(number,handler)
