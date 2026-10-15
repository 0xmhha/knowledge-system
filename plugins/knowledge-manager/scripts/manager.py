#!/usr/bin/env python3
"""Owned project registration/diagnosis and CKS maintenance launcher.

No database writes, source edits, model calls or global plugin installation.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import queue
import threading
import subprocess
import sys
import uuid

ID = re.compile(r'^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$')
REQUIRED = {'cks.dataset.inspect_profile', 'cks.dataset.build', 'cks.dataset.status', 'cks.dataset.cancel'}


class Failure(Exception):
    pass


def require(value, code):
    if not value:
        raise Failure(code)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()


def digest(value):
    return hashlib.sha256(b'cks.manager-registration.v1\0' + canonical(value)).hexdigest()


def file_hash(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024*1024), b''):
            h.update(chunk)
    return h.hexdigest()


def strict_read(path,limit=65536):
    require(path.is_file() and not path.is_symlink() and path.stat().st_size <= limit and path.stat().st_nlink == 1, 'registration_invalid')
    def pairs(items):
        result = {}
        for key,value in items:
            require(key not in result, 'registration_invalid')
            result[key] = value
        return result
    return json.loads(path.read_text(), object_pairs_hook=pairs)


def directory(path, exists=False):
    original = Path(path).expanduser()
    require(original.is_absolute(), 'absolute_path_required')
    resolved = original.resolve()
    require(not exists or resolved.is_dir(), 'source_missing')
    require(not resolved.exists() or resolved.is_dir(), 'directory_required')
    return resolved


def executable(path):
    p = Path(path).expanduser()
    require(p.is_absolute(), 'absolute_path_required')
    p = p.resolve()
    require(p.is_file() and os.access(p, os.X_OK), 'binary_unavailable')
    return p


def disjoint(paths):
    for index,a in enumerate(paths):
        for b in paths[index+1:]:
            require(not (a == b or a.is_relative_to(b) or b.is_relative_to(a)), 'overlapping_paths')


def dataset_owner(root, project):
    if not root.exists() or not any(root.iterdir()):
        return
    owner = strict_read(root / '.maintenance-project.json')
    require(owner == {'schema_version':1, 'project_id':project}, 'project_mismatch')


def write_new(path, value):
    data = json.dumps(value, ensure_ascii=False, indent=2).encode() + b'\n'
    with path.open('xb') as stream:
        os.chmod(path, 0o600)
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())


def sync_dir(path):
    fd = os.open(path, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def register(args):
    source, dataset, state = directory(args.source, True), directory(args.dataset), directory(args.state_dir)
    disjoint([source,dataset,state])
    require(not state.exists(), 'state_exists')
    project = args.project_id or 'project-' + uuid.uuid4().hex
    profile = 'profile-' + hashlib.sha256(project.encode()).hexdigest()[:32]
    require(ID.fullmatch(project) and ID.fullmatch(profile), 'project_id_invalid')
    dataset_owner(dataset, project)
    binaries = {role:executable(getattr(args, role+'_bin')) for role in ('cks','graph','vector')}
    require(args.embedder in ('ollama','mock') and args.model_name, 'embedding_invalid')
    token = uuid.uuid4().hex
    # State is a new owned bundle. A failed initialization is diagnosed as
    # incomplete; do not delete/adopt a pre-existing directory on retry.
    state.parent.mkdir(parents=True, exist_ok=True)
    state.mkdir(mode=0o700)
    write_new(state/'owner.json', {'schema_version':1,'project_id':project,'token':token})
    setup = {'project_id':project,'source_mode':args.source_mode,'src':str(source),'out':str(dataset),
             'cks_bin':str(binaries['cks']),'graph_bin':str(binaries['graph']),'vector_bin':str(binaries['vector']),
             'embedder':args.embedder,'model_name':args.model_name,'build_timeout':'1h'}
    config = {'version':1,'name':'cks','listen':{'transport':'stdio','mcp_stdio':True},'logging':{'level':'error','mode':'prod'},'sanitize':{'rules_path':str(state/'sanitization_rules.yaml')},
              'maintenance':{'profiles':[{'id':profile,'project_id':project,'setup_config':str(state/'setup.json'),
                                        'allowed_activation':['hold_for_review','activate_after_gate'],'timeout_seconds':3600}]}}
    policy = Path(__file__).resolve().parents[1]/'config/sanitization_rules.yaml'
    with (state/'sanitization_rules.yaml').open('xb') as stream:
        os.chmod(stream.name,0o600)
        stream.write(policy.read_bytes())
        stream.flush()
        os.fsync(stream.fileno())
    write_new(state/'setup.json',setup)
    write_new(state/'cks.json',config)
    payload = {'schema_version':1,'project_id':project,'profile_id':profile,'source_root':str(source),'dataset_root':str(dataset),
               'state_root':str(state),'token':token,'files':{n:file_hash(state/n) for n in ('setup.json','cks.json','sanitization_rules.yaml')},
               'binaries':{role:{'path':str(p),'sha256':file_hash(p)} for role,p in binaries.items()}}
    write_new(state/'registration.json',{'registration':payload,'checksum':digest(payload)})
    sync_dir(state)
    sync_dir(state.parent)
    return {'schema_version':1,'stage':'registered_not_built','project_id':project,'profile_id':profile,
            'registration_file':str(state/'registration.json'),'dataset_created':False,'source_modified':False,'paid_model_calls':0}


def load_registration(path):
    p = Path(path).expanduser()
    require(p.is_absolute() and not p.is_symlink(), 'absolute_path_required')
    p = p.resolve()
    envelope = strict_read(p)
    require(set(envelope)=={'registration','checksum'}, 'registration_invalid')
    r = envelope['registration']
    require(set(r)=={'schema_version','project_id','profile_id','source_root','dataset_root','state_root','token','files','binaries'}
            and type(r['schema_version']) is int and r['schema_version']==1 and envelope['checksum']==digest(r), 'registration_invalid')
    require(ID.fullmatch(r['project_id']) and ID.fullmatch(r['profile_id']) and re.fullmatch('[0-9a-f]{32}',r['token']), 'registration_invalid')
    state, source, dataset = directory(r['state_root'],True), directory(r['source_root'],True), directory(r['dataset_root'])
    require(p.resolve() == state/'registration.json' and str(p.parent)==str(state), 'registration_invalid')
    require(str(state)==r['state_root'] and str(source)==r['source_root'] and str(dataset)==r['dataset_root'],'registration_invalid')
    disjoint([state,source,dataset])
    require(strict_read(state/'owner.json')=={'schema_version':1,'project_id':r['project_id'],'token':r['token']}, 'owner_mismatch')
    require(set(r['files'])=={'setup.json','cks.json','sanitization_rules.yaml'} and set(r['binaries'])=={'cks','graph','vector'}, 'registration_invalid')
    for name,value in r['files'].items():
        file = state/name
        require(file.is_file() and not file.is_symlink() and file.stat().st_nlink==1 and file_hash(file)==value, 'config_changed')
    for item in r['binaries'].values():
        require(set(item)=={'path','sha256'}, 'registration_invalid')
        exe = executable(item['path'])
        require(str(exe)==item['path'] and file_hash(exe)==item['sha256'], 'binary_changed')
    setup,config = strict_read(state/'setup.json'),strict_read(state/'cks.json')
    require(setup['src']==str(source) and setup['out']==str(dataset) and setup['project_id']==r['project_id'] and
            config['maintenance']['profiles'][0]['id']==r['profile_id'], 'registration_invalid')
    dataset_owner(dataset,r['project_id'])
    return r


def connect(r):
    # Metadata-only. No inspect_profile/model identity query and no build.
    requests = [{'jsonrpc':'2.0','id':1,'method':'initialize','params':{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'knowledge-manager-doctor','version':'0.1'}}},
                {'jsonrpc':'2.0','id':2,'method':'tools/list'}]
    child = subprocess.Popen([r['binaries']['cks']['path'],'mcp','--config',str(Path(r['state_root'])/'cks.json')],
                             stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    replies = queue.Queue()
    def read():
        try:
            while True:
                line = child.stdout.readline(4*1024*1024+1)
                if not line:
                    break
                if len(line)>4*1024*1024:
                    break
                replies.put(json.loads(line))
        except (OSError,ValueError):
            pass
        finally:
            replies.put(None)
    reader = threading.Thread(target=read,daemon=True)
    reader.start()
    by_id = {}
    try:
        for request in requests:
            child.stdin.write(canonical(request)+b'\n')
            child.stdin.flush()
            while True:
                reply = replies.get(timeout=30)
                require(reply is not None,'mcp_unavailable')
                if reply.get('id')==request['id']:
                    require('error'not in reply,'mcp_unavailable')
                    by_id[request['id']] = reply
                    break
            if request['method']=='initialize':
                child.stdin.write(canonical({'jsonrpc':'2.0','method':'notifications/initialized'})+b'\n')
                child.stdin.flush()
        child.stdin.close()
        code = child.wait(timeout=10)
        require(code==0,'mcp_unavailable')
    except queue.Empty:
        raise Failure('mcp_unavailable')
    finally:
        if child.poll()is None:
            child.terminate()
            try:
                child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait(timeout=5)
        child.stdout.close()
        reader.join(timeout=1)
    tools = by_id[2]['result']['tools']
    selected = {t['name']:t for t in tools if t['name']in REQUIRED}
    require(set(selected)==REQUIRED and all(t.get('outputSchema')for t in selected.values()), 'maintenance_tools_missing')
    return {'maintenance_tools':sorted(selected),'transport':'stdio','EOF_exit_code':code,'model_inspection':'not_run','paid_model_calls':0}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='operation',required=True)
    reg = commands.add_parser('register')
    for name in ('source','dataset','state-dir','cks-bin','graph-bin','vector-bin'):
        reg.add_argument('--'+name,required=True)
    reg.add_argument('--project-id')
    reg.add_argument('--source-mode',choices=('snapshot-only','committed','working-tree'),default='snapshot-only')
    reg.add_argument('--embedder',choices=('ollama','mock'),default='ollama')
    reg.add_argument('--model-name',default='bge-m3:latest')
    doctor = commands.add_parser('doctor')
    doctor.add_argument('--registration',required=True)
    doctor.add_argument('--connect',action='store_true')
    launch = commands.add_parser('serve')
    launch.add_argument('--registration',default=os.environ.get('CKS_MANAGER_REGISTRATION'))
    collection = commands.add_parser('collect')
    collection.add_argument('--registration',required=True)
    collection.add_argument('--exclude',action='append',default=[])
    collection.add_argument('--context-file')
    collection.add_argument('--max-files',type=int,default=5000)
    collection.add_argument('--max-file-bytes',type=int,default=2*1024*1024)
    collection.add_argument('--max-total-bytes',type=int,default=128*1024*1024)
    collection.add_argument('--max-diff-files',type=int,default=64)
    collection.add_argument('--max-diff-bytes',type=int,default=256*1024)
    excerpt = commands.add_parser('evidence')
    excerpt.add_argument('--registration',required=True)
    excerpt.add_argument('--collection-id',required=True)
    excerpt.add_argument('--path',required=True)
    excerpt.add_argument('--origin-id',choices=('repo','provided','change'),default='repo')
    excerpt.add_argument('--start-line',type=int,required=True)
    excerpt.add_argument('--end-line',type=int,required=True)
    proposed = commands.add_parser('draft')
    proposed.add_argument('--registration',required=True)
    proposed.add_argument('--collection-id',required=True)
    proposed.add_argument('--claims-file')
    draft_status = commands.add_parser('draft-status')
    draft_status.add_argument('--registration',required=True)
    draft_status.add_argument('--draft-id',required=True)
    for operation in ('review-plan','review-status'):
        command = commands.add_parser(operation)
        command.add_argument('--registration',required=True)
        command.add_argument('--draft-id',required=True)
    for operation,flag in (('review','review-file'),('retract','retraction-file')):
        command = commands.add_parser(operation)
        command.add_argument('--registration',required=True)
        command.add_argument('--'+flag,required=True)
    adoption = commands.add_parser('adopt')
    adoption.add_argument('--registration',required=True)
    adoption.add_argument('--draft-id',required=True)
    adoption.add_argument('--review-revision',required=True)
    adopted_status = commands.add_parser('adoption-status')
    adopted_status.add_argument('--registration',required=True)
    adopted_status.add_argument('--adoption-id',required=True)
    args = parser.parse_args()
    try:
        if args.operation=='register':
            result = register(args)
        else:
            require(args.registration,'registration_required')
            r = load_registration(args.registration)
            if args.operation=='serve':
                binary = r['binaries']['cks']['path']
                os.execv(binary,[binary,'mcp','--config',str(Path(r['state_root'])/'cks.json')])
            if args.operation in ('collect','evidence','draft','draft-status'):
                from collection import collect,evidence,draft,load_draft
                if args.operation=='collect':
                    result = collect(args,r)
                elif args.operation=='evidence':
                    result = evidence(r,args.collection_id,args.path,args.start_line,args.end_line,args.origin_id)
                elif args.operation=='draft':
                    result = draft(args,r)
                else:
                    directory,content = load_draft(r,args.draft_id)
                    result = {'schema_version':1,'stage':'draft_proposed','draft_dir':str(directory),'draft':content}
            elif args.operation in ('review-plan','review-status','review','adopt','retract','adoption-status'):
                from review import plan,status,record_review,adopt,retract,active_adoption
                if args.operation=='review-plan':
                    result = plan(r,args.draft_id)
                elif args.operation=='review-status':
                    result = status(r,args.draft_id)
                elif args.operation=='review':
                    result = record_review(r,args.review_file)
                elif args.operation=='adopt':
                    result = adopt(r,args.draft_id,args.review_revision)
                elif args.operation=='retract':
                    result = retract(r,args.retraction_file)
                else:
                    directory,receipt,state = active_adoption(r,args.adoption_id)
                    result = {'schema_version':1,'stage':'current_reviewed_input_not_DB_publication','input_project':str(directory/'project'),'receipt':receipt,'workflow':state}
            else:
                result = {'schema_version':1,'stage':'registration_verified_not_built','project_id':r['project_id'],'profile_id':r['profile_id'],
                          'draft_collection':True,'review_workflow':True,'managed_reader_broker':False,'scheduling':False,'model_readiness':'not_measured'}
                if args.connect:
                    result['connection'] = connect(r)
        print(json.dumps(result,ensure_ascii=False))
        return 0
    except Failure as error:
        print(json.dumps({'schema_version':1,'code':str(error)}),file=sys.stderr)
        return 2
    except (OSError,ValueError,KeyError,TypeError,subprocess.SubprocessError,RecursionError):
        print(json.dumps({'schema_version':1,'code':'operation_failed'}),file=sys.stderr)
        return 2


if __name__=='__main__':
    # Collection imports this module for the shared error/ownership contract.
    # Reuse this running module so CLI failures keep one exception identity.
    sys.modules['manager'] = sys.modules[__name__]
    raise SystemExit(main())
