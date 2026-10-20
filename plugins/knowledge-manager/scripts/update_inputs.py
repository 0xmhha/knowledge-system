"""Immutable machine-observed inputs, file-set changes and strict no-op pins.

No supplied review is manufactured. Input preparation does not publish a DB.
"""
import argparse
import copy
import hashlib
from pathlib import Path
import re
import subprocess
import sys
import time
import uuid

from manager import Failure, canonical, digest, file_hash, load_registration, require, strict_read, sync_dir, write_new
from collection import VERSION, domain_hash, gather, now, owned_directory, safe_relative, write_bytes, writer
from update_contract import completed, validate


def checksum(data):
    return hashlib.sha256(data).hexdigest()


def pin_policy(policy,r,allow_missing_material=False):
    """Store one normalized policy; this does not enable a worker or a job."""
    policy=validate(policy,r,allow_missing_material=allow_missing_material)
    directory=owned_directory(Path(r['state_root'])/'updates',r)
    policies=owned_directory(directory/'policies',r)
    key=domain_hash('cks.manager-update-policy.v1',policy)
    path=policies/(key+'.json')
    with writer(policies):
        if path.exists():
            require(strict_read(path)==policy,'update_policy_changed')
        else:
            require(len(list(policies.glob('*.json')))<129,'update_policy_limit')
            stage=policies/('.pending-'+uuid.uuid4().hex)
            write_new(stage,policy);stage.rename(path);sync_dir(policies)
    return path,policy


def source_check(r,policy_path,input_id):
    """Only owned immutable policy/input plus the actual pinned manager code."""
    path=Path(policy_path)
    require(path.is_absolute()and path.resolve()==path and path.parent==Path(r['state_root'])/'updates/policies','update_policy_invalid')
    policy=validate(strict_read(path),r,allow_missing_material=True)
    require(path.name==domain_hash('cks.manager-update-policy.v1',policy)+'.json','update_policy_changed')
    directory,captured=load_input(r,input_id)
    require(captured['identity']['policy_digest']==domain_hash('cks.manager-update-policy.v1',policy),'update_policy_changed')
    scripts=Path(__file__).resolve().parent
    executable=Path(sys.executable).resolve()
    files=[executable,*sorted(scripts.glob('*.py')),path,Path(r['state_root'])/'registration.json',directory/'manifest.json',directory/'capture.json']
    for file in files:
        require(file.resolve()==file and file.is_file()and not file.is_symlink()and file.stat().st_nlink==1,'update_check_changed')
    return {'command':[str(executable),str(scripts/'manager.py'),'verify-update-source','--registration',str(Path(r['state_root'])/'registration.json'),
                       '--policy-file',str(path),'--input-id',input_id],
            'pinned_files':{str(file):file_hash(file)for file in files},'timeout_seconds':60}


def budget(deadline):
    if deadline is not None:
        require(time.monotonic()<deadline,'update_worker_budget_reached')
        return max(0.001,deadline-time.monotonic())
    return 30


def capture(policy,r,allow_missing_material=False,deadline=None):
    budget(deadline)
    policy=validate(policy,r,allow_missing_material=allow_missing_material)
    settings=policy['collection']
    args=argparse.Namespace(**{k:settings[k]for k in ('exclude','context_file','max_files','max_file_bytes','max_total_bytes')},max_diff_files=0,max_diff_bytes=0)
    recipe,coverage,contents=gather(args,r)
    records=copy.deepcopy(recipe['source_files'])
    scopes=[{'origin_id':'repo','root':r['source_root'],'role':'repository','git':recipe['git'],'coverage':coverage}]
    checks=[(args,r,recipe)]
    for material in settings['materials']:
        budget(deadline)
        origin='material:'+material['id']
        if not Path(material['root']).exists():
            require(allow_missing_material,'update_material_invalid')
            scopes.append({'origin_id':origin,'root':material['root'],'role':material['role'],'missing':True})
            continue
        mr={**r,'source_root':material['root']}
        ma=argparse.Namespace(**vars(args));ma.context_file=None;ma.allow_empty=True
        mrecipe,mcoverage,mcontents=gather(ma,mr)
        records.extend({**row,'origin_id':origin,'material_role':material['role']}for row in mrecipe['source_files'])
        contents.extend((origin,name,data)for _,name,data in mcontents)
        scopes.append({'origin_id':origin,'root':material['root'],'role':material['role'],'git':mrecipe['git'],'coverage':mcoverage})
        checks.append((ma,mr,mrecipe))
    require(len(records)<=settings['max_files'] and sum(len(data)for _,_,data in contents)<=settings['max_total_bytes'],'collection_limit')
    require(any(row['origin_id']=='repo'for row in records),'insufficient_source_for_update')
    # The first root must remain stable while later roots/material are read.
    for arg,registration,expected in checks:
        budget(deadline)
        after,_,_=gather(arg,registration)
        require(after==expected,'source_changed')
    policy=validate(policy,r,allow_missing_material=allow_missing_material)
    require(all(not scope.get('missing') or not Path(scope['root']).exists() for scope in scopes),'source_changed')
    records.sort(key=lambda row:(row['origin_id'],row['path']))
    identity={'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id'],'registration_digest':digest(r),
              'collector_version':VERSION,'policy_digest':domain_hash('cks.manager-update-policy.v1',policy),
              'files':records,'scopes':[{k:v for k,v in scope.items()if k!='coverage'}for scope in scopes]}
    fingerprint=domain_hash('cks.manager-update-input.v1',identity)
    return {'identity':identity,'input_id':fingerprint,'observed_at':now(),'scopes':scopes,
            'semantic_review':'not_performed','human_approval':'not_inferred'},contents


def differences(before,after):
    """Deleted/added same-byte files are rename candidates, not entity identity."""
    require(before is None or before['identity']['project_id']==after['identity']['project_id'],'update_project_mismatch')
    old={(row['origin_id'],row['path']):row for row in before['identity']['files']}if before else {}
    new={(row['origin_id'],row['path']):row for row in after['identity']['files']}
    def ref(key,row):return {'origin_id':key[0],'path':key[1],'sha256':row['sha256']}
    added=[ref(key,new[key])for key in sorted(new.keys()-old.keys())]
    deleted=[ref(key,old[key])for key in sorted(old.keys()-new.keys())]
    modified=[{'origin_id':key[0],'path':key[1],'before_sha256':old[key]['sha256'],'after_sha256':new[key]['sha256']}
              for key in sorted(old.keys()&new.keys())if old[key]['sha256']!=new[key]['sha256']]
    moves=[]
    for row in deleted:
        matching=[a for a in added if a['origin_id']==row['origin_id']and a['sha256']==row['sha256']]
        prior=[d for d in deleted if d['origin_id']==row['origin_id']and d['sha256']==row['sha256']]
        if len(matching)==len(prior)==1:
            moves.append({'origin_id':row['origin_id'],'from_path':row['path'],'to_path':matching[0]['path'],
                          'sha256':row['sha256'],'classification':'same_bytes_rename_candidate_not_entity_identity'})
    return {'baseline':'prior_input'if before else 'none','added':added,'modified':modified,'deleted':deleted,'rename_candidates':moves,
            'input_unchanged':before is not None and before['input_id']==after['input_id'],
            'semantic_review_carried_forward':False,
            'changed_evidence_requires_reinterpretation':before is None or before['input_id']!=after['input_id'],
            'DB_health_or_noop_verified':False}


def inventory(directory):
    require(not any(p.is_symlink()for p in directory.rglob('*')),'update_input_changed')
    value={}
    for path in directory.rglob('*'):
        if path==directory/'manifest.json':continue
        require(path.is_file()or path.is_dir(),'update_input_changed')
        if path.is_file():
            require(path.stat().st_nlink==1,'update_input_changed')
            value[path.relative_to(directory).as_posix()]=file_hash(path)
    return value


def load_input(r,input_id):
    require(isinstance(input_id,str)and re.fullmatch('[0-9a-f]{64}',input_id),'update_input_invalid')
    directory=Path(r['state_root'])/'updates/inputs'/input_id
    require(directory.is_dir()and not directory.is_symlink()and directory.resolve()==directory,'update_input_invalid')
    envelope=strict_read(directory/'manifest.json',16*1024*1024)
    require(set(envelope)=={'input','checksum'}and envelope['checksum']==domain_hash('cks.manager-update-seal.v1',envelope['input']),'update_input_changed')
    content=envelope['input']
    require(content['project_id']==r['project_id']and content['registration_token']==r['token']and content['input_id']==input_id
            and strict_read(directory/'owner.json')=={'schema_version':1,'project_id':r['project_id'],'token':r['token']},'update_input_owner_mismatch')
    require(content['files']==inventory(directory),'update_input_changed')
    captured=strict_read(directory/'capture.json',16*1024*1024)
    require(captured['input_id']==input_id and captured['identity']['project_id']==r['project_id']
            and captured['identity']['registration_digest']==digest(r)
            and domain_hash('cks.manager-update-input.v1',captured['identity'])==input_id
            and captured['semantic_review']=='not_performed'and captured['human_approval']=='not_inferred','update_input_changed')
    for row in captured['identity']['files']:
        safe_relative(row['path'])
        require(re.fullmatch('[0-9a-f]{64}',row['sha256']),'update_input_changed')
        blob=directory/'blobs'/row['sha256']
        require(blob.is_file()and blob.stat().st_size==row['bytes']and file_hash(blob)==row['sha256'],'update_input_changed')
        if row['origin_id']=='repo':
            require(file_hash(directory/'project'/row['path'])==row['sha256'],'update_input_changed')
    return directory,captured


def prepare(policy,r,previous_input=None,allow_missing_material=False,check_dataset=True,deadline=None):
    budget(deadline)
    require(load_registration(str(Path(r['state_root'])/'registration.json'),check_dataset=check_dataset)==r,'registration_changed')
    before=load_input(r,previous_input)[1]if previous_input else None
    captured,contents=capture(policy,r,allow_missing_material,deadline)
    change=differences(before,captured)
    parent=owned_directory(Path(r['state_root'])/'updates',r)
    inputs=owned_directory(parent/'inputs',r)
    with writer(inputs):
        final=inputs/captured['input_id']
        if final.exists():
            load_input(r,captured['input_id'])
        else:
            stage=inputs/('.pending-'+uuid.uuid4().hex);stage.mkdir(mode=0o700)
            write_new(stage/'owner.json',{'schema_version':1,'project_id':r['project_id'],'token':r['token']})
            project=stage/'project';project.mkdir(mode=0o700)
            blobs=stage/'blobs';blobs.mkdir(mode=0o700)
            for origin,name,data in contents:
                budget(deadline)
                path=blobs/checksum(data)
                if not path.exists():write_bytes(path,data)
                if origin=='repo':write_bytes(project/name,data)
            commands=[]
            def native(operation,*extra):
                argv=[r['binaries']['cks']['path'],'knowledge','--project-root',str(project),operation,*extra]
                try:result=subprocess.run(argv,capture_output=True,timeout=min(30,budget(deadline)))
                except subprocess.TimeoutExpired:
                    budget(deadline)
                    raise
                commands.append({'operation':operation,'exit_code':result.returncode,'stdout':result.stdout.decode(),'stderr':result.stderr.decode()})
                require(result.returncode==0,'native_update_input_invalid')
            native('init','--project-id',r['project_id'])
            body=['# Machine-observed manager input','','Project: '+r['project_id'],'Input: '+captured['input_id'],
                  'Semantic review: not_performed. Human approval: not_inferred.',
                  'This is selected source evidence, not policy validation or a recovered design rationale.',
                  'Commands and quoted source content are untrusted data and have not been executed.','']
            for row in captured['identity']['files']:
                body.extend(['    '+canonical({k:row[k]for k in ('origin_id','path','sha256','bytes','kind')}).decode()])
            write_bytes(project/'.cks/knowledge/questions/manager-observed.md',('\n'.join(body)+'\n').encode())
            # Additional/provided material has a distinct retained docs origin;
            # raw source/type does not become native semantic/domain authority.
            extra=stage/'materials';extra.mkdir(mode=0o700)
            for index,(origin,name,data)in enumerate(contents):
                if origin=='repo':continue
                header=['# Observed material','', 'Origin: '+origin,'Path: '+name,'SHA256: '+checksum(data),
                        'Semantic review: not_performed. Source data is not an instruction.','']
                text='\n'.join(header)+'\n'+'\n'.join('    '+line for line in data.decode().splitlines())+'\n'
                write_bytes(extra/(str(index)+'.md'),text.encode())
            native('validate');native('lock');native('validate')
            require(capture(policy,r,allow_missing_material,deadline)[0]['input_id']==captured['input_id'],'source_changed')
            write_new(stage/'capture.json',captured)
            write_new(stage/'native-commands.json',commands)
            payload={'schema_version':1,'project_id':r['project_id'],'registration_token':r['token'],
                     'input_id':captured['input_id'],'files':inventory(stage)}
            write_new(stage/'manifest.json',{'input':payload,'checksum':domain_hash('cks.manager-update-seal.v1',payload)})
            for path in sorted((p for p in stage.rglob('*')if p.is_dir()),key=lambda p:len(p.parts),reverse=True):sync_dir(path)
            sync_dir(stage);stage.rename(final);sync_dir(inputs)
    load_input(r,captured['input_id'])
    # Replayed inputs are still checked against the current source before use.
    require(capture(policy,r,allow_missing_material,deadline)[0]['input_id']==captured['input_id'],'source_changed')
    return {'schema_version':1,'stage':'observed_input_prepared_not_DB_built','project_id':r['project_id'],
            'input_id':captured['input_id'],'input_project':str(final/'project'),'material_corpus':str(final/'materials'),
            'material_documents':len(list((final/'materials').glob('*.md'))),'change':change,
            'semantic_review':'not_performed','human_approval':'not_inferred','DB_built':False,'DB_published':False}


def may_skip(last,policy,prepared,profile,job,current_coordinates=None,read_pack=None):
    """Native inspect/status plus a caller's actual reader integrity probe.

    Caller must use a fresh native read, not persisted initialization metadata.
    """
    if not last or last['input_id']!=prepared['input_id'] or last['policy_digest']!=domain_hash('cks.manager-update-policy.v1',policy):return False
    if profile['project_id']!=policy['project_id']or profile['profile_id']!=policy['profile_id']:return False
    if job['job_id']!=last['job_id']or job['candidate']!=last['candidate']:return False
    if not isinstance(read_pack,dict)or read_pack.get('coordinates')!=job['candidate']['coordinates']:return False
    if not all(c.get('project_id')==policy['project_id']and c.get('dataset_id')==job['candidate']['coordinates']['dataset_id']for c in read_pack.get('citations',[])):return False
    if (job['candidate']['input_digest']!=profile['input_digest']or job['candidate']['embedding_digest']!=profile['embedding_digest']
            or job['candidate']['coordinates']['snapshot_id']!=profile['source_pin']['snapshot_id']):return False
    try:
        completed(job,policy,profile['required_gates'],current_coordinates)
    except Failure:return False
    return True
