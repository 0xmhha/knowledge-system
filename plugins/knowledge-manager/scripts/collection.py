"""Bounded, source-read-only evidence collections and proposed knowledge drafts."""
import contextlib
import datetime
import fcntl
import fnmatch
import hashlib
import json
import os
from pathlib import Path
import re
import select
import time
import stat
import subprocess
import unicodedata
import uuid

from manager import Failure,require,canonical,file_hash,strict_read,write_new,sync_dir

VERSION = '0.2.0'
SKIP_DIRS = {'.git','.cks','.claude','.codex','.idea','.cache','node_modules','vendor','__pycache__'}
SUFFIXES = {'.go','.mod','.sum','.rs','.ts','.tsx','.js','.jsx','.sol','.py','.java','.c','.h','.cpp','.proto','.md','.markdown','.rst','.txt','.json','.yaml','.yml','.toml'}
SECRET_NAMES = ('.env','.env.*','*.pem','*.key','id_rsa*','id_ed25519*','*credentials*','*secrets*')
SECRET = re.compile(r'-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----|\b(?:AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,})\b|(?:api[_-]?key|password|secret|access[_-]?token)[\x22\x27]?\s*[=:]\s*[\x22\x27][^\x22\x27\r\n]{8,}[\x22\x27]',re.I)
CONFIG_SECRET = re.compile(r'(?im)^\s*(?:api[_-]?key|password|secret|access[_-]?token):\s*(?![\x22\x27])[^\s#]{8,}\s*$')
KINDS = {'observed_excerpt','fact','hypothesis','reported_decision','reported_command_result','troubleshooting','question'}


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00','Z')


def domain_hash(domain,value):
    return hashlib.sha256(domain.encode()+b'\0'+canonical(value)).hexdigest()


def safe_relative(value):
    require(isinstance(value,str)and value and not value.startswith('/')and '\\'not in value and '\x00'not in value
            and all(ord(c)>=32 for c in value)and unicodedata.normalize('NFC',value)==value,'unsafe_source_path')
    p = Path(value)
    require(str(p)==value and all(s not in ('','.','..')for s in p.parts),'unsafe_source_path')
    return value


def source_bytes(root,relative,limit):
    safe_relative(relative)
    initial = root.stat()
    fd = os.open(root,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:
        opened = os.fstat(fd)
        require((opened.st_dev,opened.st_ino)==(initial.st_dev,initial.st_ino),'source_changed')
        for component in Path(relative).parts[:-1]:
            child = os.open(component,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            os.close(fd)
            fd = child
        file = os.open(Path(relative).name,os.O_RDONLY|os.O_NOFOLLOW,dir_fd=fd)
        with os.fdopen(file,'rb')as stream:
            before = os.fstat(stream.fileno())
            require(stat.S_ISREG(before.st_mode)and before.st_nlink==1,'unsafe_source_path')
            require(before.st_size<=limit,'collection_limit')
            data = stream.read(limit+1)
            after = os.fstat(stream.fileno())
            require(len(data)<=limit,'collection_limit')
            require((before.st_ino,before.st_size,before.st_mtime_ns)==(after.st_ino,after.st_size,after.st_mtime_ns),'source_changed')
            return data
    finally:
        os.close(fd)


def git_metadata(root):
    env = dict(os.environ,GIT_OPTIONAL_LOCKS='0')
    for key in ('GIT_DIR','GIT_WORK_TREE','GIT_INDEX_FILE'):
        env.pop(key,None)
    def run(*args):
        result = subprocess.run(['git','--no-pager','--no-optional-locks','-c','core.fsmonitor=false','-C',str(root),*args],env=env,capture_output=True,timeout=5)
        return result.stdout.decode().strip()if result.returncode==0 else None
    try:
        top = run('rev-parse','--show-toplevel')
        if not top or Path(top).resolve()!=root:
            return {'commit':None,'tracked_changes':None,'scope':'no_exact_root_git_metadata'}
        commit = run('rev-parse','--verify','HEAD')
        quiet = subprocess.run(['git','--no-pager','--no-optional-locks','-c','core.fsmonitor=false','-C',str(root),'diff','--quiet','--no-ext-diff','--no-textconv','HEAD','--'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=5)
        return {'commit':commit,'tracked_changes':quiet.returncode==1 if quiet.returncode in (0,1)else None,'scope':'selected_repository_tracked_changes_only'}
    except (OSError,ValueError,subprocess.SubprocessError):
        return {'commit':None,'tracked_changes':None,'scope':'git_metadata_unavailable'}



def git_diff(root,path,limit):
    safe_relative(path)
    env = dict(os.environ,GIT_OPTIONAL_LOCKS='0')
    for key in ('GIT_DIR','GIT_WORK_TREE','GIT_INDEX_FILE'):
        env.pop(key,None)
    command = ['git','--no-pager','--literal-pathspecs','--no-optional-locks','-c','core.fsmonitor=false','-C',str(root),
               'diff','--no-ext-diff','--no-textconv','--no-color','--unified=3','HEAD','--',path]
    child = subprocess.Popen(command,env=env,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,start_new_session=True)
    data,deadline = bytearray(),time.monotonic()+5
    try:
        while True:
            remaining = deadline-time.monotonic()
            if remaining<=0 or not select.select([child.stdout],[],[],remaining)[0]:
                return None
            block = os.read(child.stdout.fileno(),min(32768,limit+1-len(data)))
            if not block:
                break
            data.extend(block)
            if len(data)>limit:
                return None
        return bytes(data)if child.wait(timeout=5)==0 else None
    except (OSError,subprocess.SubprocessError):
        return None
    finally:
        if child.poll()is None:
            child.terminate()
            try:
                child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait(timeout=5)
        child.stdout.close()


def changes(root,source_records,metadata,max_files,max_bytes):
    require(0<=max_files<=1024 and 0<=max_bytes<=4*1024*1024,'collection_limit')
    if not metadata['commit']or max_files==0 or max_bytes==0:
        return [],[],{'state':'not_available_or_disabled','scope':'selected_existing_files_HEAD_to_worktree','unexamined_files':len(source_records)}
    records,contents,used,omitted = [],[],0,{}
    def skip(reason):
        omitted[reason]=omitted.get(reason,0)+1
    for source in source_records[:max_files]:
        limit = min(65536,max_bytes-used)
        if limit<=0:
            skip('budget')
            continue
        data = git_diff(root,source['path'],limit)
        if data is None:
            skip('unavailable_or_limit')
            continue
        if not data:
            continue
        try:
            text = data.decode('utf-8')
        except UnicodeError:
            skip('non_utf8_diff')
            continue
        stripped = '\n'.join(line[1:]if line.startswith(('+','-'))else line for line in text.splitlines())
        if SECRET.search(text)or CONFIG_SECRET.search(stripped):
            skip('sensitive_diff')
            continue
        used+=len(data)
        records.append({'origin_id':'change','path':source['path'],'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data),'kind':'git_diff',
                        'lines':len(data.splitlines()),'source_actor':None,'source_recorded_at':None,'base_commit':metadata['commit'],
                        'working_file_sha256':source['sha256'],'tool':'git diff --no-ext-diff --no-textconv HEAD'})
        contents.append(('change',source['path'],data))
    unexamined = max(0,len(source_records)-max_files)
    return records,contents,{'state':'partial'if omitted or unexamined else 'collected','scope':'selected_existing_files_HEAD_to_worktree',
                             'omitted_counts':omitted,'unexamined_files':unexamined,'diff_files':len(records),'bytes':used,'deleted_files':'separate_KM04_change_tracking'}


def selection(root,patterns,max_paths):
    names,excluded,seen = [],{},set()
    visited = 0
    def count(kind):
        excluded[kind]=excluded.get(kind,0)+1
    def omitted(relative):
        return any(fnmatch.fnmatchcase(relative,p)or fnmatch.fnmatchcase(relative+'/',p)for p in patterns)
    def unreadable(_error):
        raise Failure('source_unreadable')
    for current,dirs,files in os.walk(root,topdown=True,followlinks=False,onerror=unreadable):
        relative = Path(current).relative_to(root)
        kept = []
        for name in sorted(dirs):
            visited+=1
            require(visited<=max_paths,'collection_limit')
            path = Path(current)/name
            rel = (relative/name).as_posix()
            if name in SKIP_DIRS or omitted(rel):
                count('excluded_directory')
            elif path.is_symlink():
                count('linked_path')
            else:
                safe_relative(rel)
                kept.append(name)
        dirs[:] = kept
        for name in sorted(files):
            visited+=1
            require(visited<=max_paths,'collection_limit')
            rel = (relative/name).as_posix()
            safe_relative(rel)
            if omitted(rel)or any(fnmatch.fnmatchcase(name.lower(),p)for p in SECRET_NAMES):
                count('excluded_file')
                continue
            if Path(name).suffix.lower()not in SUFFIXES and name not in ('README','Makefile','Dockerfile'):
                count('unsupported_file_type')
                continue
            info = (root/rel).lstat()
            if not stat.S_ISREG(info.st_mode)or info.st_nlink!=1:
                count('linked_or_nonregular_file')
                continue
            require(rel.casefold()not in seen,'source_path_collision')
            seen.add(rel.casefold())
            names.append(rel)
    return sorted(names),excluded


def write_bytes(path,data):
    path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
    with path.open('xb')as stream:
        os.chmod(path,0o600)
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())


def owned_directory(path,r):
    owner = {'schema_version':1,'project_id':r['project_id'],'token':r['token']}
    try:
        path.mkdir(mode=0o700)
    except FileExistsError:
        require(path.is_dir()and not path.is_symlink()and strict_read(path/'owner.json')==owner,'collection_owner_mismatch')
    else:
        write_new(path/'owner.json',owner)
        sync_dir(path)
        sync_dir(path.parent)
    return path


@contextlib.contextmanager
def writer(directory):
    fd = os.open(directory/'.lock',os.O_CREAT|os.O_RDWR|os.O_NOFOLLOW,0o600)
    try:
        info = os.fstat(fd)
        require(stat.S_ISREG(info.st_mode)and info.st_nlink==1,'collection_owner_mismatch')
        try:
            fcntl.flock(fd,fcntl.LOCK_EX|fcntl.LOCK_NB)
        except BlockingIOError:
            raise Failure('collection_busy')
        yield
    finally:
        os.close(fd)


def provided_records(path,r,limit):
    if not path:
        return [],[],{}
    selected = Path(path).expanduser()
    require(selected.is_absolute()and selected.is_file()and not selected.is_symlink()and selected.stat().st_nlink==1 and selected.stat().st_size<=limit,'provided_material_invalid')
    selected = selected.resolve()
    data = source_bytes(selected.parent,selected.name,limit)
    require(len(data)<=limit,'collection_limit')
    require(not SECRET.search(data.decode('utf-8')),'private_material')
    # Use the same duplicate-key rejection as registration; the bound is
    # larger here because an actual command report can exceed metadata size.
    def pairs(items):
        result = {}
        for key,value in items:
            require(key not in result,'provided_material_invalid')
            result[key]=value
        return result
    document = json.loads(data,object_pairs_hook=pairs)
    require(set(document)=={'schema_version','project_id','records'}and type(document['schema_version'])is int and document['schema_version']==1,'provided_material_invalid')
    require(document['project_id']==r['project_id'],'project_mismatch')
    records = document['records']
    require(isinstance(records,list)and len(records)<=128,'provided_material_invalid')
    # Mixed restricted input is not copied into the collection at all.
    require(all(isinstance(row,dict)and row.get('visibility','public')=='public'for row in records),'private_material')
    files,contents = [],[]
    for index,row in enumerate(records):
        require(set(row)<={'kind','text','actor','recorded_at','visibility','command','exit_code','stdout','stderr'}and row.get('kind')in ('decision','troubleshooting','command_result','note'),'provided_material_invalid')
        require(row.get('actor')is None or isinstance(row['actor'],str)and 0<len(row['actor'])<=128,'provided_material_invalid')
        require(row.get('recorded_at')is None or isinstance(row['recorded_at'],str)and len(row['recorded_at'])<=100,'provided_material_invalid')
        if row['kind']=='command_result':
            require(isinstance(row.get('command'),list)and 1<=len(row['command'])<=32 and all(isinstance(a,str)and len(a)<=4096 for a in row['command'])
                    and type(row.get('exit_code'))is int and isinstance(row.get('stdout',''),str)and isinstance(row.get('stderr',''),str),'provided_material_invalid')
            require(not any(a.lower().split('=')[0]in ('--password','--token','--secret','--api-key')for a in row['command']),'private_material')
        else:
            require(isinstance(row.get('text'),str)and 0<len(row['text'])<=65536,'provided_material_invalid')
        body = json.dumps(row,ensure_ascii=False,indent=2).encode()+b'\n'
        name = 'provided/record-'+str(index+1)+'.json'
        require(len(body)<=limit,'collection_limit')
        contents.append(('provided',name,body))
        files.append({'origin_id':'provided','path':name,'sha256':hashlib.sha256(body).hexdigest(),'bytes':len(body),'kind':'provided_'+row['kind'],'lines':len(body.splitlines()),
                      'source_actor':row.get('actor'),'source_recorded_at':row.get('recorded_at'),'original_file_sha256':hashlib.sha256(data).hexdigest(),
                      'original_json_pointer':'/records/'+str(index),'reported_not_reexecuted':True})
    return files,contents,{'provided_records':len(files)}


def load_collection(r,collection_id):
    require(isinstance(collection_id,str)and re.fullmatch('[0-9a-f]{64}',collection_id),'collection_invalid')
    directory = Path(r['state_root'])/'collections'/collection_id
    require(directory.is_dir()and not directory.is_symlink(),'collection_invalid')
    envelope = strict_read(directory/'manifest.json',16*1024*1024)
    require(set(envelope)=={'collection','checksum'},'collection_invalid')
    c = envelope['collection']
    require(envelope['checksum']==domain_hash('cks.manager-collection-manifest.v1',c)and c['project_id']==r['project_id']and c['collection_id']==collection_id
            and c['registration_token']==r['token']and c['source_root']==r['source_root']and c['files']==c['recipe']['source_files']and c['recipe']['project_id']==r['project_id']and domain_hash('cks.manager-collection.v1',c['recipe'])==collection_id,'collection_owner_mismatch')
    require(strict_read(directory/'owner.json')=={'schema_version':1,'project_id':r['project_id'],'token':r['token']},'collection_owner_mismatch')
    for item in c['files']:
        safe_relative(item['path'])
        require(re.fullmatch('[0-9a-f]{64}',item['sha256']),'collection_invalid')
        blob = directory/'blobs'/item['sha256']
        require(blob.is_file()and not blob.is_symlink()and blob.stat().st_nlink==1 and blob.stat().st_size==item['bytes']and file_hash(blob)==item['sha256'],'collection_changed')
    return directory,c


def gather(args,r):
    require(1<=args.max_files<=10000 and 1<=args.max_file_bytes<=8*1024*1024 and 1<=args.max_total_bytes<=256*1024*1024,'collection_limit')
    patterns = sorted(set(args.exclude or []))
    require(len(patterns)<=64 and all(isinstance(p,str)and 0<len(p)<=256 and not p.startswith('/')and '..'not in p.split('/')and '\x00'not in p for p in patterns),'exclusion_invalid')
    source = Path(r['source_root'])
    root_info = source.stat()
    names,excluded = selection(source,patterns,args.max_files*20)
    require(len(names)<=args.max_files,'collection_limit')
    records,contents,total = [],[],0
    before_git = git_metadata(source)
    for name in names:
        data = source_bytes(source,name,args.max_file_bytes)
        total+=len(data)
        require(total<=args.max_total_bytes,'collection_limit')
        try:
            text = data.decode('utf-8')
        except UnicodeError:
            excluded['non_utf8_file']=excluded.get('non_utf8_file',0)+1
            continue
        if '\x00'in text or SECRET.search(text)or (Path(name).suffix.lower()in ('.json','.yaml','.yml','.toml')and CONFIG_SECRET.search(text)):
            excluded['sensitive_or_binary_content']=excluded.get('sensitive_or_binary_content',0)+1
            continue
        if not data:
            excluded['empty_file']=excluded.get('empty_file',0)+1
            continue
        kind = 'documentation'if Path(name).suffix.lower()in ('.md','.markdown','.rst','.txt')or name=='README'else 'configuration'if Path(name).suffix.lower()in ('.json','.yaml','.yml','.toml','.mod','.sum')else 'code'
        records.append({'origin_id':'repo','path':name,'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data),'kind':kind,'lines':len(data.splitlines()),'source_actor':None,'source_recorded_at':None})
        contents.append(('repo',name,data))
    change_records,change_contents,change_coverage = changes(source,records,before_git,getattr(args,'max_diff_files',64),getattr(args,'max_diff_bytes',256*1024))
    records.extend(change_records)
    contents.extend(change_contents)
    extra,extra_contents,_ = provided_records(args.context_file,r,args.max_file_bytes)
    records.extend(extra)
    contents.extend(extra_contents)
    require(records or getattr(args,'allow_empty',False),'insufficient_evidence')
    require(len(records)<=args.max_files and total+sum(len(data)for _,_,data in extra_contents)+sum(len(data)for _,_,data in change_contents)<=args.max_total_bytes,'collection_limit')
    after_names,_ = selection(source,patterns,args.max_files*20)
    require(after_names==names and git_metadata(source)==before_git,'source_changed')
    for origin,name,data in contents:
        if origin=='repo':
            require(source_bytes(source,name,args.max_file_bytes)==data,'source_changed')
        elif origin=='change':
            require(git_diff(source,name,len(data))==data,'source_changed')
    require(source.resolve()==source and (source.stat().st_dev,source.stat().st_ino)==(root_info.st_dev,root_info.st_ino),'source_changed')
    records.sort(key=lambda row:(row['origin_id'],row['path']))
    recipe = {'schema_version':1,'project_id':r['project_id'],'collector_version':VERSION,'source_files':records,'git':before_git,
              'policy':{'exclude':patterns,'max_files':args.max_files,'max_file_bytes':args.max_file_bytes,'max_total_bytes':args.max_total_bytes,'supported_text_only':True,'built_in_exclusions_version':1,'max_diff_files':getattr(args,'max_diff_files',64),'max_diff_bytes':getattr(args,'max_diff_bytes',256*1024)}}
    coverage = {'selected_source_files_complete':True,'git_changes':change_coverage,'whole_project_semantics_verified':False,'excluded_counts':excluded,'file_count':len(records)}
    return recipe,coverage,contents


def collect(args,r):
    recipe,coverage,contents = gather(args,r)
    records = recipe['source_files']
    source = Path(r['source_root'])
    before_git = recipe['git']
    cid = domain_hash('cks.manager-collection.v1',recipe)
    parent = owned_directory(Path(r['state_root'])/'collections',r)
    with writer(parent):
        final = parent/cid
        if final.exists():
            _,c = load_collection(r,cid)
            return {'schema_version':1,'stage':'collection_replayed','project_id':r['project_id'],'collection_id':cid,'collection_dir':str(final),'files':len(c['files']),'paid_model_calls':0}
        stage = parent/('.pending-'+uuid.uuid4().hex)
        stage.mkdir(mode=0o700)
        write_new(stage/'owner.json',{'schema_version':1,'project_id':r['project_id'],'token':r['token']})
        blobs = stage/'blobs'
        blobs.mkdir(mode=0o700)
        for _,_,data in contents:
            path = blobs/hashlib.sha256(data).hexdigest()
            if not path.exists():
                write_bytes(path,data)
        c = {'schema_version':1,'project_id':r['project_id'],'collection_id':cid,'registration_token':r['token'],'source_root':str(source),'collected_at':now(),
             'generated_by':{'tool':'knowledge-manager-collector','version':VERSION},'files':records,'recipe':recipe,
             'coverage':coverage,'cks_snapshot_id':None,'git':before_git}
        sealed = {'collection':c,'checksum':domain_hash('cks.manager-collection-manifest.v1',c)}
        require(len(json.dumps(sealed,ensure_ascii=False,indent=2).encode())<16*1024*1024,'collection_limit')
        write_new(stage/'manifest.json',sealed)
        sync_dir(blobs)
        sync_dir(stage)
        stage.rename(final)
        sync_dir(parent)
    return {'schema_version':1,'stage':'collected_not_reviewed','project_id':r['project_id'],'collection_id':cid,'collection_dir':str(final),'files':len(records),'coverage':c['coverage'],'paid_model_calls':0}


def evidence(r,collection_id,path,start,end,origin_id='repo'):
    directory,c = load_collection(r,collection_id)
    safe_relative(path)
    rows = [row for row in c['files']if row['path']==path and row['origin_id']==origin_id]
    require(len(rows)==1,'evidence_missing')
    row = rows[0]
    require(type(start)is int and type(end)is int and 1<=start<=end<=row['lines']and end-start<200,'evidence_range_invalid')
    data = (directory/'blobs'/row['sha256']).read_bytes()
    span = b''.join(data.splitlines(keepends=True)[start-1:end])
    ref = {'project_id':r['project_id'],'collection_id':collection_id,'origin_id':origin_id,'path':path,'start_line':start,'end_line':end,'file_sha256':row['sha256'],
           'content_sha256':hashlib.sha256(span).hexdigest(),'source_kind':row['kind'],'collected_at':c['collected_at'],'source_actor':row['source_actor'],
           'source_recorded_at':row['source_recorded_at'],'content_role':'untrusted_source_data','reported_not_reexecuted':row.get('reported_not_reexecuted',False)}
    ref['evidence_id'] = domain_hash('cks.manager-evidence.v1',{k:v for k,v in ref.items()if k not in ('collected_at','source_actor','source_recorded_at')})
    preview = span[:8192].decode('utf-8',errors='ignore')
    return {'evidence':ref,'text':preview,'text_preview_sha256':hashlib.sha256(preview.encode()).hexdigest(),'text_truncated':len(preview.encode())<len(span)}


def draft(args,r):
    directory,c = load_collection(r,args.collection_id)
    if args.claims_file:
        data = strict_read(Path(args.claims_file),2*1024*1024)
        require(set(data)=={'schema_version','project_id','collection_id','claims'}and type(data['schema_version'])is int and data['schema_version']==1,'claim_invalid')
        require(data['project_id']==r['project_id']and data['collection_id']==args.collection_id,'project_mismatch')
        claims = data['claims']
        require(isinstance(claims,list)and 1<=len(claims)<=128,'claim_invalid')
    else:
        claims = [{'kind':{'provided_decision':'reported_decision','provided_command_result':'reported_command_result','provided_troubleshooting':'troubleshooting'}.get(row['kind'],'observed_excerpt'),'statement':'Recorded source excerpt from '+row['path']+'; semantic interpretation remains unreviewed.',
                   'refs':[{'origin_id':row['origin_id'],'path':row['path'],'start_line':1,'end_line':min(row['lines'],40)}]}for row in c['files'][:128]]
    proposed = []
    for claim in claims:
        require(set(claim)=={'kind','statement','refs'}and claim['kind']in KINDS and isinstance(claim['statement'],str)and 0<len(claim['statement'])<=4096
                and isinstance(claim['refs'],list)and 1<=len(claim['refs'])<=8 and not SECRET.search(claim['statement']),'claim_invalid')
        refs,quotes = [],[]
        for request in claim['refs']:
            require(set(request) in ({'path','start_line','end_line'},{'origin_id','path','start_line','end_line'}),'claim_invalid')
            resolved = evidence(r,args.collection_id,request['path'],request['start_line'],request['end_line'],request.get('origin_id','repo'))
            refs.append(resolved['evidence'])
            quotes.append(resolved['text'])
        if claim['kind']=='reported_decision':
            require(any(ref['source_kind']=='documentation'or ref['source_kind'].startswith('provided_')for ref in refs),'decision_requires_reported_source')
        if claim['kind']=='reported_command_result':
            require(any(ref['source_kind']=='provided_command_result'for ref in refs),'command_requires_reported_source')
        payload = {'kind':claim['kind'],'statement':claim['statement'],'evidence':refs}
        proposed.append({'claim_id':domain_hash('cks.manager-claim.v1',{'project_id':r['project_id'],'collection_id':args.collection_id,**payload}),**payload,
                         'status':'proposed','reviewed_by':None,'review_reason':None,'semantic_verification':'not_run','quotes':quotes,'quotes_are_bounded_previews':True})
    require(sum(len(q.encode()) for item in proposed for q in item['quotes'])<=2*1024*1024,'draft_limit')
    did = domain_hash('cks.manager-draft.v1',{'project_id':r['project_id'],'collection_id':args.collection_id,'claims':proposed})
    parent = owned_directory(Path(r['state_root'])/'drafts',r)
    with writer(parent):
        final = parent/did
        if final.exists():
            _,old = load_draft(r,did)
            return {'schema_version':1,'stage':'draft_replayed','project_id':r['project_id'],'collection_id':args.collection_id,'draft_id':did,'draft_dir':str(final),'claims':len(old['claims']),'paid_model_calls':0}
        stage = parent/('.pending-'+uuid.uuid4().hex)
        stage.mkdir(mode=0o700)
        write_new(stage/'owner.json',{'schema_version':1,'project_id':r['project_id'],'token':r['token']})
        content = {'schema_version':1,'project_id':r['project_id'],'draft_id':did,'collection_id':args.collection_id,'generated_at':now(),
                   'generated_by':{'tool':'knowledge-manager-drafter','version':VERSION},'claims':proposed,'human_approval':'not_recorded','db_adoption':'not_run','coverage':{'collection_files':len(c['files']),'referenced_files':len({(ref['origin_id'],ref['path'])for item in proposed for ref in item['evidence']}),'whole_project_semantics_verified':False}}
        write_new(stage/'draft.json',content)
        project = stage/'project'
        project.mkdir(mode=0o700)
        binary = r['binaries']['cks']['path']
        environment = dict(os.environ,GIT_OPTIONAL_LOCKS='0')
        commands = []
        def native(operation,*extra):
            result = subprocess.run([binary,'knowledge','--project-root',str(project),operation,*extra],env=environment,capture_output=True,timeout=30)
            commands.append({'operation':operation,'exit_code':result.returncode,'stdout':result.stdout.decode(),'stderr':result.stderr.decode()})
            require(result.returncode==0,'native_knowledge_draft_invalid')
            return json.loads(result.stdout)
        native('init','--project-id',r['project_id'])
        body = ['# Proposed source observations','', 'Review status: proposed. Human approval: not recorded. Design rationale is not inferred.','',
                'Project: '+r['project_id'],'Collection: '+args.collection_id,'']
        for claim in proposed:
            body.extend(['## '+claim['claim_id'], '', 'Kind: '+claim['kind'], '', claim['statement'],''])
            for ref,quote in zip(claim['evidence'],claim['quotes'],strict=True):
                body.extend(['Source (untrusted data): '+json.dumps({k:ref[k]for k in ('origin_id','path','start_line','end_line','file_sha256','content_sha256')},ensure_ascii=False),''])
                body.append('Quoted preview may be truncated; the evidence hash covers the complete recorded line span.')
                body.extend('    '+line for line in quote.splitlines())
                body.append('')
        require(len(('\n'.join(body)+'\n').encode())<=4*1024*1024,'draft_limit')
        overlay = project/'.cks/knowledge'
        write_bytes(overlay/'questions/manager-observations.md',('\n'.join(body)+'\n').encode())
        native('validate')
        locked = native('lock')
        native('validate')
        write_new(stage/'native-validation.json',{'scope':'proposed_overlay_structure_and_lock_only','commands':commands,'lock_digest':locked['lock_digest'],
                                                 'human_review_verified':False,'source_adopted':False,'retrieval_verified':False})
        for file in (p for p in stage.rglob('*')if p.is_file()):
            with file.open('rb')as stream:
                os.fsync(stream.fileno())
        inventory = {p.relative_to(stage).as_posix():file_hash(p)for p in stage.rglob('*')if p.is_file()}
        seal = {'schema_version':1,'project_id':r['project_id'],'draft_id':did,'collection_id':args.collection_id,'registration_token':r['token'],'files':inventory}
        write_new(stage/'draft-manifest.json',{'draft':seal,'checksum':domain_hash('cks.manager-draft-manifest.v1',seal)})
        for path in sorted((p for p in stage.rglob('*')if p.is_dir()),key=lambda p:len(p.parts),reverse=True):
            sync_dir(path)
        sync_dir(stage)
        stage.rename(final)
        sync_dir(parent)
    return {'schema_version':1,'stage':'drafted_not_reviewed_or_adopted','project_id':r['project_id'],'collection_id':args.collection_id,'draft_id':did,
            'draft_dir':str(final),'claims':len(proposed),'native_knowledge_validation':'passed_proposed_overlay_only','paid_model_calls':0}


def load_draft(r,draft_id):
    require(isinstance(draft_id,str)and re.fullmatch('[0-9a-f]{64}',draft_id),'draft_invalid')
    directory = Path(r['state_root'])/'drafts'/draft_id
    require(directory.is_dir()and not directory.is_symlink(),'draft_invalid')
    envelope = strict_read(directory/'draft-manifest.json',1024*1024)
    require(set(envelope)=={'draft','checksum'},'draft_invalid')
    d = envelope['draft']
    require(envelope['checksum']==domain_hash('cks.manager-draft-manifest.v1',d)and d['project_id']==r['project_id']and d['draft_id']==draft_id
            and d['registration_token']==r['token'],'draft_owner_mismatch')
    require(not any(p.is_symlink()for p in directory.rglob('*')),'draft_changed')
    actual = {p.relative_to(directory).as_posix()for p in directory.rglob('*')if p.is_file()and p.name!='draft-manifest.json'}
    require(actual==set(d['files']),'draft_changed')
    for name,digest in d['files'].items():
        safe_relative(name)
        path = directory/name
        require(path.stat().st_nlink==1 and file_hash(path)==digest,'draft_changed')
    content = strict_read(directory/'draft.json',16*1024*1024)
    require(content['project_id']==r['project_id']and content['draft_id']==draft_id and content['collection_id']==d['collection_id']
            and all(claim['status']=='proposed'and claim['reviewed_by']is None and claim['review_reason']is None for claim in content['claims'])
            and domain_hash('cks.manager-draft.v1',{'project_id':r['project_id'],'collection_id':d['collection_id'],'claims':content['claims']})==draft_id,'draft_changed')
    load_collection(r,d['collection_id'])
    return directory,content
