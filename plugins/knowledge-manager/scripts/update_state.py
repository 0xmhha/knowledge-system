"""Durable bounded update intent ledger. This module never runs a build.

The reviewed-input broker and the automatic workflow share one lifetime flock.
Disk state, not an in-memory request/ack, defines the recovery obligation.
"""
import copy
from pathlib import Path
import re
import uuid

from manager import Failure, canonical, digest, file_hash, load_registration, require, strict_read, sync_dir, write_new
from collection import domain_hash, owned_directory, writer
from update_contract import clock, completed, due, retry_delay, validate
from update_inputs import load_input, may_skip, pin_policy

PRE_ADMISSION_REFUSALS = frozenset(('source_stale','input_stale','profile_stale','embedding_stale','base_stale',
    'version_exists','writer_busy','unsupported_capability','project_mismatch','profile_unknown','invalid_request'))
LIMIT = 128
STATE_LIMIT = 4*1024*1024
DOMAIN = 'cks.manager-update-state.v1'
HASH = re.compile('[0-9a-f]{64}')


def request_digest(request):
    normalized=copy.deepcopy(request)
    normalized['base']={k:v for k,v in request['base'].items() if k=='state' or k=='version' and v or k=='coordinates' and v is not None}
    return domain_hash('cks.dataset-build.v1',normalized)


def default_state(r):
    return {'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id'],
            'registration_token':r['token'],'registration_digest':digest(r),'revision':0,
            'policy':None,'phase':'idle','last_checked':None,'pending':None,'attempts':[],
            'last_success':None,'last_outcome':None,'retry_at':None,'last_error':None}


def validate_state(state,r):
    require(isinstance(state,dict) and set(state)==set(default_state(r)),'update_state_invalid')
    for key in ('schema_version','project_id','profile_id','registration_token','registration_digest'):
        require(state[key]==default_state(r)[key] and type(state[key]) is type(default_state(r)[key]),'update_state_owner_mismatch')
    require(type(state['revision']) is int and state['revision']>=0 and isinstance(state['attempts'],list)
            and len(state['attempts'])<=LIMIT,'update_state_invalid')
    require(state['phase'] in ('idle','checking','admitting','building','held','succeeded','no_op','retry_wait','blocked','stopped'),'update_state_invalid')
    for key in ('last_checked','retry_at'):
        if state[key] is not None:clock(state[key])
    if state['policy'] is not None:
        require(set(state['policy'])=={'path','digest'} and HASH.fullmatch(state['policy']['digest'])
                and state['policy']['path']==str(Path(r['state_root'])/'updates/policies'/(state['policy']['digest']+'.json')),'update_state_invalid')
    ids,keys,versions=set(),set(),set()
    previous={}
    for row in state['attempts']:
        require(isinstance(row,dict) and set(row)-{'admission_error'}=={'id','cycle_id','previous_attempt','attempt','input_id','policy','bundle',
                'profile','request','request_digest','created_at','observed_at','job','outcome','read_receipt'},'update_state_invalid')
        require(isinstance(row['id'],str) and re.fullmatch('[0-9a-f]{32}',row['id']) and row['id'] not in ids
                and isinstance(row['cycle_id'],str) and re.fullmatch('[0-9a-f]{32}',row['cycle_id'])
                and isinstance(row['input_id'],str) and HASH.fullmatch(row['input_id'])
                and type(row['attempt']) is int and 1<=row['attempt']<=10,'update_state_invalid')
        clock(row['created_at']);clock(row['observed_at'])
        request=row['request'];profile=row['profile']
        require(isinstance(request,dict) and set(request)=={'schema_version','project_id','profile_id','profile_digest','source_pin','input_digest',
                    'embedding_digest','base','new_version','activation','idempotency_key','deadline_seconds'}
                and type(request['schema_version']) is int and request['schema_version']==1
                and request['project_id']==r['project_id'] and request['profile_id']==r['profile_id']
                and row['request_digest']==request_digest(request),'update_state_invalid')
        require(request['new_version']=='manager-'+row['id'] and request['idempotency_key']=='manager-update-'+row['id']
                and request['idempotency_key'] not in keys and request['new_version'] not in versions
                and request['activation'] in ('hold_for_review','activate_after_gate')
                and type(request['deadline_seconds']) is int and 1<=request['deadline_seconds']<=3600,'update_state_invalid')
        require(isinstance(profile,dict) and profile.get('project_id')==r['project_id'] and profile.get('profile_id')==r['profile_id']
                and all(request[k]==profile.get(k) for k in ('profile_digest','source_pin','input_digest','embedding_digest'))
                and request['base']==profile.get('observed_base') and isinstance(profile.get('required_gates'),list) and profile['required_gates'], 'update_state_invalid')
        require(all(isinstance(request[k],str) and HASH.fullmatch(request[k]) for k in ('profile_digest','input_digest','embedding_digest'))
                and isinstance(request['source_pin'],dict) and set(request['source_pin'])=={'mode','snapshot_id','base_commit'} and request['source_pin'].get('mode')=='snapshot-only'
                and HASH.fullmatch(request['source_pin'].get('snapshot_id','')) and request['source_pin'].get('base_commit','')=='','update_state_invalid')
        policy=row['policy'];bundle=row['bundle']
        require(isinstance(policy,dict) and set(policy)=={'path','digest'} and HASH.fullmatch(policy['digest'])
                and policy['path']==str(Path(r['state_root'])/'updates/policies'/(policy['digest']+'.json'))
                and isinstance(bundle,dict) and set(bundle)=={'config','files'} and isinstance(bundle['files'],dict)
                and 1<=len(bundle['files'])<=64 and bundle['config'] in bundle['files'],'update_state_invalid')
        for name,h in bundle['files'].items():
            path=Path(name)
            require(path.is_absolute() and path.is_relative_to(Path(r['state_root'])/'updates/worker/bundles')
                    and '..' not in path.parts and isinstance(h,str) and HASH.fullmatch(h),'update_state_invalid')
        prior=row['previous_attempt']
        if prior is None:
            require(row['attempt']==1 and row['cycle_id']==row['id'],'update_state_invalid')
        else:
            require(prior in previous,'update_state_invalid');old=previous[prior]
            require(old['outcome'] in ('failed','canceled','refused') and old['input_id']==row['input_id'] and old['policy']==policy
                    and old['cycle_id']==row['cycle_id'] and old['attempt']+1==row['attempt'],'update_state_invalid')
        if row['job'] is not None:validate_job(row,row['job'],r)
        require(row['outcome'] in (None,'held','succeeded','failed','canceled','refused'),'update_state_invalid')
        if row['outcome']=='refused':
            require(row['job'] is None and isinstance(row.get('admission_error'),dict)
                    and set(row['admission_error'])=={'at','code'} and row['admission_error']['code'] in PRE_ADMISSION_REFUSALS,'update_state_invalid')
            clock(row['admission_error']['at'])
        else:
            require('admission_error' not in row,'update_state_invalid')
        if row['outcome'] is not None and row['outcome']!='refused':
            require(row['job'] is not None and row['job']['terminal'] is True,'update_state_invalid')
            if row['outcome'] in ('held','succeeded'):
                require(isinstance(row['read_receipt'],dict) and set(row['read_receipt'])=={'at','coordinates','pack_digest'}
                        and row['read_receipt']['coordinates']==row['job']['candidate']['coordinates']
                        and HASH.fullmatch(row['read_receipt']['pack_digest']),'update_state_invalid')
                clock(row['read_receipt']['at'])
        ids.add(row['id']);keys.add(request['idempotency_key']);versions.add(request['new_version']);previous[row['id']]=row
    open_rows=[row['id'] for row in state['attempts'] if row['outcome'] is None]
    require(open_rows==([] if state['pending'] is None else [state['pending']]),'update_state_invalid')
    if state['last_success'] is not None:
        require(state['last_success'] in previous and previous[state['last_success']]['outcome'] in ('held','succeeded'),'update_state_invalid')
    if state['last_outcome'] is not None:require(state['last_outcome'] in previous,'update_state_invalid')
    return state


def read_state(path,r):
    envelope=strict_read(path,STATE_LIMIT)
    require(isinstance(envelope,dict) and set(envelope)=={'state','checksum'}
            and envelope['checksum']==domain_hash(DOMAIN,envelope['state']),'update_state_invalid')
    return validate_state(envelope['state'],r)


def validate_job(row,job,r):
    require(isinstance(job,dict) and job.get('project_id')==r['project_id'] and job.get('profile_id')==r['profile_id']
            and isinstance(job.get('job_id'),str) and 0<len(job['job_id'])<=128
            and type(job.get('revision')) is int and job['revision']>=1 and type(job.get('terminal')) is bool
            and job.get('request_digest')==row['request_digest'] and job.get('target_version')==row['request']['new_version'], 'update_job_mismatch')
    return job


def assert_worker_idle(r):
    """Called only after acquiring the shared broker lease. Read, never adopt."""
    path=Path(r['state_root'])/'updates/worker/state.json'
    if not path.exists() and not path.is_symlink():return
    require(path.resolve()==path,'update_state_invalid')
    state=read_state(path,r)
    require(state['pending'] is None,'pending_update_requires_reconciliation')


class Journal:
    def __init__(self,registration,recover=False):
        self.registration=str(Path(registration).resolve())
        self.r=load_registration(self.registration,check_dataset=not recover)
        self.recovery_lock_root=False
        if recover:
            try:load_registration(self.registration)
            except (Failure,OSError,ValueError,KeyError,TypeError):
                # Only a previously stored, never-observed admission may have
                # left native's lone persistent mutation-lock inode. No data
                # or unknown nonempty root is adopted, and Python never claims it.
                path=Path(self.r['state_root'])/'updates/worker/state.json'
                require(path.resolve()==path,'update_state_invalid')
                prior=read_state(path,self.r)
                require(prior['attempts'] and prior['last_success'] is None
                        and all(row['job'] is None for row in prior['attempts']) and self.lone_native_lock(),'update_recovery_owner_required')
                self.recovery_lock_root=True
        broker=owned_directory(Path(self.r['state_root'])/'broker',self.r)
        self.lease=writer(broker);self.lease.__enter__();self.closed=False;self.poisoned=False
        try:
            parent=owned_directory(Path(self.r['state_root'])/'updates',self.r)
            self.directory=owned_directory(parent/'worker',self.r);self.path=self.directory/'state.json'
            self.state=read_state(self.path,self.r) if self.path.exists() or self.path.is_symlink() else default_state(self.r)
            self.disk_checksum=domain_hash(DOMAIN,self.state) if self.path.exists() else None
        except BaseException:
            self.close();raise

    def lone_native_lock(self):
        root=Path(self.r['dataset_root']);path=root/'.reindex.lock'
        return (root.is_dir() and root.resolve()==root and set(p.name for p in root.iterdir())=={'.reindex.lock'}
                and path.is_file() and not path.is_symlink() and path.stat().st_nlink==1 and path.stat().st_size<4096)

    def registration_ok(self):
        try:return load_registration(self.registration)==self.r
        except (Failure,OSError,ValueError,KeyError,TypeError):
            if not self.recovery_lock_root:raise
            require(self.recovery_lock_root and self.lone_native_lock(),'update_recovery_owner_required')
            return load_registration(self.registration,check_dataset=False)==self.r

    def commit(self,next_state):
        require(not self.closed and not self.poisoned,'update_reconciliation_required')
        require(self.registration_ok(),'registration_changed')
        try:
            current=read_state(self.path,self.r) if self.path.exists() or self.path.is_symlink() else None
        except BaseException:
            self.poisoned=True
            raise
        if (domain_hash(DOMAIN,current) if current else None)!=self.disk_checksum:
            self.poisoned=True
            require(False,'update_state_changed')
        value=copy.deepcopy(next_state);value['revision']=self.state['revision']+1;validate_state(value,self.r)
        envelope={'state':value,'checksum':domain_hash(DOMAIN,value)}
        require(len(canonical(envelope))<STATE_LIMIT,'update_history_limit')
        require(len(list(self.directory.glob('.pending-*')))<LIMIT,'update_pending_file_limit')
        stage=self.directory/('.pending-'+uuid.uuid4().hex)
        try:
            write_new(stage,envelope);stage.replace(self.path);sync_dir(self.directory)
        except BaseException:
            self.poisoned=True
            raise
        self.state=value;self.disk_checksum=envelope['checksum']
        return self.describe()

    def broker_idle(self):
        path=Path(self.r['state_root'])/'broker/state.json'
        if not path.exists() and not path.is_symlink():return
        value=strict_read(path,2*1024*1024);state=value.get('state',{})
        require(set(value)=={'state','checksum'} and value['checksum']==domain_hash('cks.manager-broker-state.v1',state)
                and state.get('project_id')==self.r['project_id'] and state.get('profile_id')==self.r['profile_id']
                and state.get('registration_token')==self.r['token'] and isinstance(state.get('jobs'),dict),'broker_state_invalid')
        require(state.get('pending_build') is None and all(row.get('terminal') is True for row in state['jobs'].values()),'reviewed_job_requires_reconciliation')

    def policy(self):
        reference=self.state['policy'];require(reference is not None,'update_policy_required')
        path=Path(reference['path']);value=validate(strict_read(path),self.r,allow_missing_material=True)
        require(path.resolve()==path and domain_hash('cks.manager-update-policy.v1',value)==reference['digest'],'update_policy_changed')
        return value

    def configure(self,value):
        require(self.state['pending'] is None,'pending_update_requires_reconciliation');self.broker_idle()
        # Missing material roots are accepted only for the already-applied policy.
        prior=self.state['policy'];normalized=validate(value,self.r,allow_missing_material=True)
        same=prior is not None and prior['digest']==domain_hash('cks.manager-update-policy.v1',normalized)
        path,policy=pin_policy(value,self.r,allow_missing_material=same)
        next_state=copy.deepcopy(self.state)
        if prior is None or str(path)!=prior['path']:
            next_state.update(policy={'path':str(path),'digest':domain_hash('cks.manager-update-policy.v1',policy)},
                              phase='idle',last_checked=None,retry_at=None,last_error=None)
        return self.commit(next_state)

    def checking(self,trigger,at):
        require(self.state['pending'] is None,'pending_update_requires_reconciliation');self.broker_idle()
        policy=self.policy();decision=due(policy,trigger,at,self.state['last_checked'])
        if self.state['retry_at'] is not None:
            decision['retry_at']=self.state['retry_at']
            decision['due']=bool(policy['enabled'] and trigger in policy['triggers'] and at>=self.state['retry_at'])
        if decision['due']:
            next_state=copy.deepcopy(self.state);next_state.update(phase='checking',last_checked=at,last_error=None)
            self.commit(next_state)
        return decision

    def row(self,identifier=None):
        selected=identifier or self.state['pending']
        require(selected is not None,'update_intent_required')
        return next(row for row in self.state['attempts'] if row['id']==selected)

    def verify_bundle(self,row):
        for name,expected in row['bundle']['files'].items():
            path=Path(name);strict_read(path,1024*1024)
            require(path.resolve()==path and file_hash(path)==expected,'update_bundle_changed')
        return Path(row['bundle']['config'])

    def intend(self,input_id,bundle,profile,at,previous_attempt=None):
        clock(at);require(self.state['pending'] is None,'pending_update_requires_reconciliation');self.broker_idle()
        require(self.state['phase']=='checking' and self.policy()['enabled'],'update_check_required')
        require(len(self.state['attempts'])<LIMIT,'update_history_limit')
        policy=self.policy();_,captured=load_input(self.r,input_id)
        require(captured['identity']['policy_digest']==self.state['policy']['digest'],'update_policy_changed')
        last=self.row(self.state['last_outcome']) if self.state['last_outcome'] else None
        if last and last['outcome'] in ('failed','canceled','refused') and last['input_id']==input_id and last['policy']==self.state['policy']:
            require(previous_attempt==last['id'],'update_retry_requires_prior_attempt')
        identifier=uuid.uuid4().hex;prior=self.row(previous_attempt) if previous_attempt is not None else None
        if prior is not None:
            require(self.state['retry_at'] is not None and at>=self.state['retry_at'] and prior['outcome'] in ('failed','canceled','refused')
                    and prior['id']==self.state['last_outcome'] and prior['policy']==self.state['policy'] and prior['input_id']==input_id
                    and prior['attempt']<policy['retry']['max_attempts'],'update_retry_invalid')
        request={'schema_version':1,'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],
                 **{k:copy.deepcopy(profile[k]) for k in ('profile_digest','source_pin','input_digest','embedding_digest')},
                 'base':copy.deepcopy(profile['observed_base']),'new_version':'manager-'+identifier,
                 'activation':policy['activation'],'idempotency_key':'manager-update-'+identifier,'deadline_seconds':policy['deadline_seconds']}
        row={'id':identifier,'cycle_id':prior['cycle_id'] if prior else identifier,'previous_attempt':previous_attempt,
             'attempt':prior['attempt']+1 if prior else 1,'input_id':input_id,'policy':copy.deepcopy(self.state['policy']),
             'bundle':copy.deepcopy(bundle),'profile':copy.deepcopy(profile),'request':request,'request_digest':request_digest(request),
             'created_at':at,'observed_at':at,'job':None,'outcome':None,'read_receipt':None}
        # This validation and durable commit happen before any caller forwards build.
        next_state=copy.deepcopy(self.state);next_state['attempts'].append(row);next_state.update(pending=identifier,phase='admitting',retry_at=None,last_error=None)
        validate_state(next_state,self.r);self.verify_bundle(row);self.commit(next_state)
        return copy.deepcopy(self.row())

    def observe(self,job,at):
        clock(at);row=self.row();validate_job(row,job,self.r)
        before=row['job']
        if before is not None:
            require(before['job_id']==job['job_id'] and job['revision']>=before['revision']
                    and (job['revision']>before['revision'] or before==job)
                    and (not before['terminal'] or before==job),'update_job_regressed')
        next_state=copy.deepcopy(self.state);record=next(r for r in next_state['attempts'] if r['id']==row['id'])
        record.update(job=copy.deepcopy(job),observed_at=at);next_state.update(phase='building')
        return self.commit(next_state)

    def refuse(self,code,at):
        """A typed pre-admission refusal, never an ambiguous I/O outcome."""
        clock(at);require(code in PRE_ADMISSION_REFUSALS,'update_admission_uncertain')
        row=self.row();require(row['job'] is None,'update_known_job_requires_status')
        policy=self.policy();delay=retry_delay(policy,row['attempt'])
        next_state=copy.deepcopy(self.state);record=next(r for r in next_state['attempts'] if r['id']==row['id'])
        record.update(outcome='refused',admission_error={'at':at,'code':code},observed_at=at)
        next_state.update(pending=None,last_outcome=row['id'],phase='blocked' if delay is None else 'retry_wait',
                          retry_at=None if delay is None else at+delay,last_error={'code':'retry_exhausted' if delay is None else code})
        return self.commit(next_state)

    def settle(self,at,current_coordinates=None,read_pack=None):
        clock(at);row=self.row();job=row['job'];require(job is not None and job['terminal'] is True,'update_job_not_terminal')
        # Use this intent's policy, even if a caller still holds another object.
        policy=validate(strict_read(Path(row['policy']['path'])),self.r,allow_missing_material=True)
        require(domain_hash('cks.manager-update-policy.v1',policy)==row['policy']['digest'],'update_policy_changed')
        next_state=copy.deepcopy(self.state);record=next(r for r in next_state['attempts'] if r['id']==row['id'])
        if job['state'] in ('failed','canceled'):
            require(job.get('activated') is False,'update_failed_job_activated')
            record['outcome']=job['state'];delay=retry_delay(policy,row['attempt'])
            next_state.update(phase='blocked' if delay is None else 'retry_wait',retry_at=None if delay is None else at+delay,
                              last_error={'code':'retry_exhausted' if delay is None else 'native_'+job['state'],'native_error':job.get('error')})
        else:
            outcome=completed(job,policy,row['profile']['required_gates'],current_coordinates)
            candidate=job['candidate'];profile=row['profile']
            require(candidate.get('input_digest')==profile['input_digest'] and candidate.get('embedding_digest')==profile['embedding_digest']
                    and candidate['coordinates']['snapshot_id']==profile['source_pin']['snapshot_id'],'update_candidate_invalid')
            require(isinstance(read_pack,dict) and read_pack.get('coordinates')==candidate['coordinates']
                    and isinstance(read_pack.get('bodies'),list) and isinstance(read_pack.get('citations'),list)
                    and all(c.get('project_id')==self.r['project_id'] and c.get('dataset_id')==candidate['coordinates']['dataset_id'] for c in read_pack['citations']),'update_read_not_verified')
            record.update(outcome=outcome,read_receipt={'at':at,'coordinates':candidate['coordinates'],
                                                       'pack_digest':domain_hash('cks.manager-update-read.v1',read_pack)})
            next_state.update(phase=outcome,last_success=row['id'],last_error=None,retry_at=None)
        record['observed_at']=at;next_state.update(pending=None,last_outcome=row['id'])
        return self.commit(next_state)

    def no_op(self,prepared,profile,job,at,current_coordinates=None,read_pack=None):
        clock(at);require(self.state['pending'] is None and self.state['phase']=='checking','update_check_required')
        require(self.state['last_success'] is not None,'update_noop_not_verified')
        row=self.row(self.state['last_success'])
        last={'input_id':row['input_id'],'policy_digest':row['policy']['digest'],'job_id':row['job']['job_id'],'candidate':row['job']['candidate']}
        require(may_skip(last,self.policy(),prepared,profile,job,current_coordinates,read_pack),'update_noop_not_verified')
        next_state=copy.deepcopy(self.state);next_state.update(phase='no_op',last_error=None,retry_at=None)
        record=next(r for r in next_state['attempts'] if r['id']==row['id'])
        record.update(observed_at=at,read_receipt={'at':at,'coordinates':job['candidate']['coordinates'],
                      'pack_digest':domain_hash('cks.manager-update-read.v1',read_pack)})
        return self.commit(next_state)

    def block(self,code):
        require(isinstance(code,str) and re.fullmatch('[a-z][a-z0-9_]{0,95}',code),'update_error_invalid')
        next_state=copy.deepcopy(self.state);next_state.update(phase='blocked',last_error={'code':code})
        return self.commit(next_state)

    def describe(self):
        return {'schema_version':1,'stage':'durable_update_state_not_worker_execution','state':copy.deepcopy(self.state),
                'state_snapshot_origin':'last_confirmed_in_process_state','disk_state_requires_reconciliation':self.poisoned,
                'scheduler_running':False,'native_job_executed_by_this_module':False,'independent_acceptance':False}

    def close(self):
        if self.closed:return
        self.closed=True;self.lease.__exit__(None,None,None)

    def __enter__(self):return self
    def __exit__(self,*_args):self.close()
