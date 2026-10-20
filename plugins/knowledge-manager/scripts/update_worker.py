"""Explicit foreground update worker over owned immutable inputs/native jobs.

No daemon/OS scheduler is installed. Policy chooses trigger/publication; all
admission is durable before forwarding and all recovery precedes new collection.
"""
import copy
import json
import os
import sys
import threading
from pathlib import Path
import signal
import time
import uuid

from manager import Failure,canonical,file_hash,require,strict_read,sync_dir,write_new
from collection import domain_hash,owned_directory
from update_contract import bounded,clock
from update_inputs import prepare,source_check
from update_state import Journal,PRE_ADMISSION_REFUSALS
from broker import Native,GracefulShutdown


from update_native import Refusal,value,code_inventory,check_code
from update_reader import read_observed


class Worker:
    def __init__(self,registration,deadline=None):
        self.code_pin=code_inventory()
        self.journal=Journal(registration,recover=True)
        self.r=self.journal.r
        self.native,self.native_config=None,None
        self.closed=False;self.deadline=deadline
        self.last_read=None;self.forwarded_build_requests=0;self.observed_jobs=set()

    def timeout(self):
        if self.deadline is None:return 120
        remaining=self.deadline-time.monotonic();require(remaining>0,'update_worker_budget_reached')
        return min(120,remaining)

    def bundle(self,prepared):
        j=self.journal;policy=j.policy()
        check=source_check(self.r,Path(j.state['policy']['path']),prepared['input_id'])
        parent=owned_directory(j.directory/'bundles',self.r)
        key=domain_hash('cks.manager-update-bundle.v1',{'registration':j.state['registration_digest'],'policy':j.state['policy']['digest'],
                                                      'input_id':prepared['input_id'],'check':check})
        final=parent/key
        setup=strict_read(Path(self.r['state_root'])/'setup.json')
        corpora=[str(Path(prepared['input_project'])/'.cks/knowledge/questions')]
        if prepared['material_documents']:corpora.append(prepared['material_corpus'])
        setup.update(src=prepared['input_project'],source_mode='snapshot-only',document_corpora=corpora,build_timeout=str(policy['deadline_seconds'])+'s')
        config=strict_read(Path(self.r['state_root'])/'cks.json')
        config['maintenance']['profiles'][0].update(setup_config=str(final/'setup.json'),source_check=check,
                allowed_activation=[policy['activation']],timeout_seconds=max(10,policy['deadline_seconds']))
        expected={'setup.json':setup,'cks.json':config}
        if final.exists():
            require(final.is_dir() and final.resolve()==final and not final.is_symlink()
                    and strict_read(final/'owner.json')=={'schema_version':1,'project_id':self.r['project_id'],'token':self.r['token']}
                    and set(p.name for p in final.iterdir())=={'owner.json',*expected},'update_bundle_changed')
            require(all(strict_read(final/name,1024*1024)==v for name,v in expected.items()),'update_bundle_changed')
        else:
            require(len([p for p in parent.iterdir() if p.is_dir()])<128,'update_bundle_limit')
            stage=parent/('.pending-'+uuid.uuid4().hex);stage.mkdir(mode=0o700)
            write_new(stage/'owner.json',{'schema_version':1,'project_id':self.r['project_id'],'token':self.r['token']})
            for name,payload in expected.items():write_new(stage/name,payload)
            sync_dir(stage);stage.rename(final);sync_dir(parent)
        return {'config':str(final/'cks.json'),'files':{str(final/name):file_hash(final/name) for name in expected}}

    def client(self,bundle):
        check_code(self.code_pin)
        self.journal.verify_bundle({'bundle':bundle})
        require(self.journal.registration_ok(),'registration_changed')
        if self.native_config!=bundle['config']:
            if self.native:
                self.native.close();self.native=None;self.native_config=None
            self.native=Native(self.r,Path(bundle['config']),request_timeout=self.timeout());self.native_config=bundle['config']
        self.native.request_timeout=self.timeout()
        return self.native

    def tool(self,bundle,name,args):
        native=self.client(bundle)
        if name=='cks.dataset.build':self.forwarded_build_requests+=1
        return value(native.tool(name,args))

    def scope(self,job=None):
        result={'schema_version':1,'project_id':self.r['project_id'],'profile_id':self.r['profile_id']}
        if job is not None:result['job_id']=job['job_id']
        return result

    def read(self,row,job,prompt=None):
        check_code(self.code_pin)
        pack,current=read_observed(self.r,row,job,self.journal.directory/'readers',prompt,self.timeout(),Native)
        self.last_read=copy.deepcopy(pack)
        return pack,current

    def reconcile(self,at):
        j=self.journal;row=j.row()
        try:
            if row['job'] is None:
                job=self.tool(row['bundle'],'cks.dataset.build',row['request'])
            else:
                job=self.tool(row['bundle'],'cks.dataset.status',self.scope(row['job']))
        except Refusal as error:
            if row['job'] is None and error.value['code'] in PRE_ADMISSION_REFUSALS:
                j.refuse(error.value['code'],at);return self.describe()
            j.block('update_admission_uncertain');raise
        j.observe(job,at);self.observed_jobs.add(job['job_id'])
        if job['terminal']:
            if job['state'] in ('failed','canceled'):j.settle(at)
            else:
                pack,current=self.read(j.row(),job);j.settle(at,current,pack)
            return self.describe()
        return self.describe()

    def tick(self,trigger='manual',at=None):
        check_code(self.code_pin)
        at=time.time() if at is None else clock(at)
        j=self.journal
        # Never check a new source or overwrite intent before known/uncertain
        # admission has been reconciled through the original pinned config.
        if j.state['pending'] is not None:return self.reconcile(at)
        decision=j.checking(trigger,at)
        if not decision['due']:return {**self.describe(),'trigger_decision':decision}
        try:
            baseline=j.row(j.state['last_success']) if j.state['last_success'] else None
            prepared=prepare(j.policy(),self.r,baseline['input_id'] if baseline else None,allow_missing_material=True,
                             check_dataset=not j.recovery_lock_root,deadline=self.deadline)
            bundle=self.bundle(prepared);profile=self.tool(bundle,'cks.dataset.inspect_profile',self.scope())['profile']
            same=baseline is not None and baseline['input_id']==prepared['input_id'] and baseline['policy']==j.state['policy']
            same=same and all(baseline['job']['candidate'][k]==profile[k] for k in ('input_digest','embedding_digest'))\
                and baseline['job']['candidate']['coordinates']['snapshot_id']==profile['source_pin']['snapshot_id']
            if same:
                job=self.tool(bundle,'cks.dataset.status',self.scope(baseline['job']))
                pack,current=self.read(baseline,job)
                try:
                    j.no_op(prepared,profile,job,at,current,pack);return self.describe()
                except Failure as error:
                    if str(error)!='update_noop_not_verified':raise
            last=j.row(j.state['last_outcome']) if j.state['last_outcome'] else None
            prior=last['id'] if last and last['outcome'] in ('failed','canceled','refused') and last['input_id']==prepared['input_id'] and last['policy']==j.state['policy'] else None
            j.intend(prepared['input_id'],bundle,profile,at,prior)
            return self.reconcile(at)
        except Failure as error:
            if not j.poisoned:j.block(str(error) if str(error).replace('_','').isalnum() and len(str(error))<=96 else 'update_operation_failed')
            raise

    def describe(self):
        value=self.journal.describe()
        value.pop('native_job_executed_by_this_module')
        value.update(stage='foreground_native_update_worker',
                     build_requests_forwarded_this_process=self.forwarded_build_requests,observed_job_ids_this_process=sorted(self.observed_jobs),
                     scheduler_running=not self.closed,background_scheduler_installed=False,last_read=self.last_read)
        return value

    def close(self):
        if self.closed:return
        self.closed=True
        try:
            if self.native:
                try:self.native.close()
                finally:self.native=None;self.native_config=None
            # EOF waits for native quiescence/durable cancellation. Leave the
            # exact pending intent for next-start status, never claim canceled
            # merely because close returned.
            if not self.journal.poisoned and self.journal.state['pending'] is not None:
                state=copy.deepcopy(self.journal.state);state['phase']='stopped';self.journal.commit(state)
        finally:self.journal.close()


def run(registration,trigger,max_iterations,max_seconds,poll_seconds=1,stop_on_stdin_eof=False):
    require(bounded(max_iterations,1,10000) and bounded(max_seconds,1,86400)
            and type(poll_seconds) in (int,float) and 0.01<=poll_seconds<=60 and type(stop_on_stdin_eof)is bool,'update_worker_budget_invalid')
    deadline=time.monotonic()+max_seconds
    worker,previous,stopping,finishing=None,{},False,False
    watch_done=threading.Event();watch_gate=threading.Lock()
    iterations=0;stop_reason='completed'
    def stop(_number,_frame):
        nonlocal stopping
        if not stopping:
            stopping=True
            if not finishing:raise GracefulShutdown()
    try:
        for number in (signal.SIGINT,signal.SIGTERM):previous[number]=signal.signal(number,stop)
        if stop_on_stdin_eof:
            # Raw fd reads avoid a daemon holding Python's buffered-stdin lock
            # during interpreter shutdown. The owner sends no input data.
            owner_fd=sys.stdin.fileno()
            def watch_owner():
                try:
                    while os.read(owner_fd,4096):
                        if watch_done.is_set():return
                except OSError:pass
                with watch_gate:
                    if not watch_done.is_set():os.kill(os.getpid(),signal.SIGTERM)
            threading.Thread(target=watch_owner,daemon=True).start()
        worker=Worker(registration,deadline);iterations=0
        while iterations<max_iterations and time.monotonic()<deadline:
            try:worker.tick(trigger)
            except Failure as error:
                transient=(trigger in('periodic','change') and str(error)in('source_changed','source_stale')
                           and not worker.journal.poisoned and worker.journal.state['pending']is None)
                if not transient:raise
                # The interval and foreground iteration/time budget bound these
                # deferred checks. No admitted job/intent is relabeled or lost.
                print(json.dumps({'schema_version':1,'event':'update_check_deferred','code':str(error),
                    'project_id':worker.r['project_id'],'ledger_revision':worker.journal.state['revision']},ensure_ascii=False),file=sys.stderr,flush=True)
            # Admission/status polls are one execution obligation. Only a
            # completed or not-due check advances the bounded loop counter.
            if worker.journal.state['pending'] is None:iterations+=1
            if iterations<max_iterations:
                time.sleep(min(poll_seconds,max(0,deadline-time.monotonic())))
        if time.monotonic()>=deadline:stop_reason='budget'
    except GracefulShutdown:stop_reason='signal'
    except Failure as error:
        if str(error)!='update_worker_budget_reached' and not (time.monotonic()>=deadline and str(error) in ('native_timeout','native_unavailable')):raise
        stop_reason='budget'
    finally:
        finishing=True
        with watch_gate:watch_done.set()
        try:
            if worker:worker.close()
        finally:
            for number,handler in previous.items():signal.signal(number,handler)
    if worker is None and stop_reason=='signal':
        return {'schema_version':1,'stage':'worker_stopped_before_registration_ready','stop_reason':'signal',
                'worker_stop_outcome':'closed_before_worker_initialization','scheduler_running':False,
                'state_not_read_by_process':True,'new_job_admitted_by_this_process':False}
    require(worker is not None,'update_worker_not_started')
    value=worker.describe();value['iterations_completed']=iterations;value['stop_reason']=stop_reason
    value['worker_stop_outcome']='closed_requires_persistence_reconciliation' if worker.journal.poisoned else 'closed_with_pending_intent' if worker.journal.state['pending'] else 'closed'
    return value
