"""Explicit foreground update worker over owned immutable inputs/native jobs.

No daemon/OS scheduler is installed. Policy chooses trigger/publication; all
admission is durable before forwarding and all recovery precedes new collection.
"""
import copy
import json
from pathlib import Path
import signal
import time
import uuid

from manager import Failure,canonical,file_hash,require,strict_read,sync_dir,write_new
from collection import domain_hash,owned_directory
from update_contract import bounded,clock
from update_inputs import load_input,prepare,source_check
from update_state import Journal,PRE_ADMISSION_REFUSALS
from broker import Native,READ,GracefulShutdown


class Refusal(Failure):
    def __init__(self,value):
        self.value=value
        super().__init__(value['code'])


def value(response):
    payload=response.get('structuredContent')
    require(isinstance(payload,dict),'native_protocol_invalid')
    if response.get('isError'):
        require(isinstance(payload.get('code'),str),'native_protocol_invalid')
        raise Refusal(payload)
    return payload


class Worker:
    def __init__(self,registration,deadline=None):
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
        candidate=job['candidate'];version=Path(self.r['dataset_root'])/candidate['version']
        require(candidate['version']==row['request']['new_version'] and version.is_dir() and version.resolve()==version and not version.is_symlink(),'update_candidate_invalid')
        directory,_=load_input(self.r,row['input_id']);manifest=strict_read(version/'vector/manifest.json',16*1024*1024)
        setup=strict_read(Path(self.r['state_root'])/'setup.json')
        config={'version':1,'name':'cks','listen':{'transport':'stdio','mcp_stdio':True},'logging':{'level':'error','mode':'prod'},
                'sanitize':{'rules_path':str(Path(self.r['state_root'])/'sanitization_rules.yaml'),'default_action':'drop','fail_closed_on_unknown_rule':True},
                'backends':{'ckg':{'path':str(version/'graph/graph.db'),'source_root':str(directory/'project'),'timeout_ms':5000},
                            'ckv':{'path':str(version/'vector'),'provider':setup['embedder'],'embed_model':manifest['embedding_model'],
                                   'embed_dim':setup.get('embed_dim',0),'query_prefix_policy':setup.get('query_prefix_policy','registry'),
                                   'ollama_url':setup.get('ollama_url','http://localhost:11434')}}}
        parent=owned_directory(self.journal.directory/'readers',self.r)
        key=domain_hash('cks.manager-update-reader.v1',config);path=parent/(key+'.json')
        if path.exists():require(strict_read(path,1024*1024)==config,'update_reader_config_changed')
        else:
            require(len(list(parent.glob('*.json')))<129,'update_reader_limit')
            stage=parent/('.pending-'+uuid.uuid4().hex);write_new(stage,config);stage.rename(path);sync_dir(parent)
        current=Path(self.r['dataset_root'])/'current'
        def pointer():
            if not current.is_symlink():return None
            target=current.readlink()
            require(len(target.parts)==1 and target.name not in ('.','..','current'),'update_current_invalid')
            return target.name
        before=pointer()
        reader=Native(self.r,path,request_timeout=self.timeout())
        try:
            pack=value(reader.tool(READ,{'prompt':prompt or 'Observed source and knowledge evidence'}))
            require(pack.get('coordinates')==candidate['coordinates'],'update_read_not_verified')
        finally:reader.close()
        after=pointer();self.last_read=copy.deepcopy(pack)
        # Candidate read is a current observation only while the actual pointer
        # selects that exact version both before and after the read.
        coordinates=pack['coordinates'] if before==after==candidate['version'] else None
        return pack,coordinates

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


def run(registration,trigger,max_iterations,max_seconds,poll_seconds=1):
    require(bounded(max_iterations,1,10000) and bounded(max_seconds,1,86400)
            and type(poll_seconds) in (int,float) and 0.01<=poll_seconds<=60,'update_worker_budget_invalid')
    deadline=time.monotonic()+max_seconds
    worker,previous,stopping=None,{},False
    iterations=0;stop_reason='completed'
    def stop(_number,_frame):
        nonlocal stopping
        if not stopping:stopping=True;raise GracefulShutdown()
    try:
        for number in (signal.SIGINT,signal.SIGTERM):previous[number]=signal.signal(number,stop)
        worker=Worker(registration,deadline);iterations=0
        while iterations<max_iterations and time.monotonic()<deadline:
            worker.tick(trigger)
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
        try:
            if worker:worker.close()
        finally:
            for number,handler in previous.items():signal.signal(number,handler)
    require(worker is not None,'update_worker_not_started')
    value=worker.describe();value['iterations_completed']=iterations;value['stop_reason']=stop_reason
    return value
