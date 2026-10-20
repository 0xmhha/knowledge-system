"""Explicit supplied review records and scoped adoption; no invented approval."""
import argparse
import itertools
import os
from pathlib import Path
import re
import subprocess
import uuid

from manager import canonical,file_hash,require,strict_read,sync_dir,write_new
from collection import domain_hash,gather,load_collection,load_draft,now,owned_directory,safe_relative,SECRET,write_bytes,writer

DISPOSITIONS = {'accepted','rejected','deferred','excluded'}
MAX_EVENTS = 128


def text(value,limit):
    return isinstance(value,str)and 0<len(value.strip())<=limit and not SECRET.search(value)


def claims_by_id(draft):
    return {c['claim_id']:c for c in draft['claims']}


def overlaps(a,b):
    return any(x['origin_id']==y['origin_id']and x['path']==y['path']and max(x['start_line'],y['start_line'])<=min(x['end_line'],y['end_line'])for x in a['evidence']for y in b['evidence'])


def concerns(draft):
    unique = claims_by_id(draft)
    duplicates,interpretations = [],[]
    for a,b in itertools.combinations(unique.values(),2):
        ids = sorted((a['claim_id'],b['claim_id']))
        same = ' '.join(a['statement'].casefold().split())==' '.join(b['statement'].casefold().split())and a['kind']==b['kind']
        if same:
            duplicates.append({'claim_ids':ids,'classification':'possible_duplicate_not_automatically_merged'})
        elif overlaps(a,b):
            interpretations.append({'claim_ids':ids,'classification':'shared_evidence_distinct_interpretations_not_proven_contradiction'})
    return duplicates,interpretations


def plan(r,draft_id):
    _,draft = load_draft(r,draft_id)
    duplicates,interpretations = concerns(draft)
    questions = []
    for claim in claims_by_id(draft).values():
        refs = [e['evidence_id']for e in claim['evidence']]
        if claim['kind']=='question':
            prompt = claim['statement']
        elif claim['kind']=='hypothesis':
            prompt = 'What observation supports or refutes this hypothesis? Keep it a hypothesis until reviewed.'
        elif claim['kind']=='reported_decision':
            prompt = 'Does the reported decision apply to this project/version, and who actually provided its rationale?'
        elif claim['kind']=='reported_command_result':
            prompt = 'What environment/version produced this supplied result? It has not been executed by the manager.'
        else:
            prompt = 'Which constraints and alternatives explain this observation? Supply the actual decision source or leave the rationale unknown.'
        questions.append({'claim_id':claim['claim_id'],'kind':claim['kind'],'question':prompt,'evidence_ids':refs,'answer_status':'not_provided'})
    state = status(r,draft_id)
    return {'schema_version':1,'project_id':r['project_id'],'draft_id':draft_id,'collection_id':draft['collection_id'],
            'as_of':'immutable_collection_not_current_source_verification','revision':state['revision'],'questions':questions,
            'possible_duplicates':duplicates,'shared_evidence_interpretations':interpretations,
            'duplicate_claim_occurrences':len(draft['claims'])-len(claims_by_id(draft)),
            'claims':[{'claim_id':c['claim_id'],'kind':c['kind'],'statement':c['statement'],'status':'proposed','evidence':c['evidence']}for c in claims_by_id(draft).values()],
            'review_authentication':'not_performed','human_approval':'not_inferred','db_adoption':'not_run'}


def workflow(r,draft_id,create=False):
    parent = Path(r['state_root'])/'workflows'
    directory = parent/draft_id
    if create:
        owned_directory(parent,r)
        owned_directory(directory,r)
    elif parent.exists():
        require(not parent.is_symlink()and strict_read(parent/'owner.json')=={'schema_version':1,'project_id':r['project_id'],'token':r['token']},'review_owner_mismatch')
        if directory.exists():
            require(not directory.is_symlink()and strict_read(directory/'owner.json')=={'schema_version':1,'project_id':r['project_id'],'token':r['token']},'review_owner_mismatch')
    return directory


def events(r,draft_id):
    directory = workflow(r,draft_id)
    if not directory.exists():
        return []
    names = sorted(p.name for p in directory.iterdir()if p.name not in ('owner.json','.lock')and not p.name.startswith('.pending-'))
    require(len(names)<=MAX_EVENTS,'review_limit')
    rows,previous = [],None
    for sequence,name in enumerate(names,1):
        require(re.fullmatch(r'[0-9]{8}-[0-9a-f]{64}\.json',name),'review_journal_invalid')
        row = strict_read(directory/name,4*1024*1024)
        require(set(row)=={'schema_version','project_id','draft_id','registration_token','sequence','previous_revision','recorded_at','kind','payload','request_id','revision'}
                and row['schema_version']==1 and row['project_id']==r['project_id']and row['draft_id']==draft_id and row['registration_token']==r['token']
                and row['sequence']==sequence and row['previous_revision']==previous and row['kind']in ('reviewed','adopted','retracted'),'review_journal_invalid')
        require(row['revision']==domain_hash('cks.manager-review-event.v1',{k:v for k,v in row.items()if k!='revision'})
                and name==f'{sequence:08d}-'+row['revision']+'.json'
                and row['request_id']==domain_hash('cks.manager-review-request.v1',{'kind':row['kind'],'payload':row['payload']}),'review_journal_changed')
        previous = row['revision']
        rows.append(row)
    return rows


def append_event(r,draft_id,kind,payload,previous):
    # Caller owns the workflow lock; only a fully fsynced event becomes visible.
    directory = workflow(r,draft_id)
    old = events(r,draft_id)
    require((old[-1]['revision']if old else None)==previous,'review_revision_conflict')
    require(len(old)<MAX_EVENTS,'review_limit')
    row = {'schema_version':1,'project_id':r['project_id'],'draft_id':draft_id,'registration_token':r['token'],'sequence':len(old)+1,
           'previous_revision':previous,'recorded_at':now(),'kind':kind,'payload':payload,
           'request_id':domain_hash('cks.manager-review-request.v1',{'kind':kind,'payload':payload})}
    row['revision'] = domain_hash('cks.manager-review-event.v1',row)
    temporary = directory/('.pending-'+uuid.uuid4().hex)
    write_new(temporary,row)
    temporary.rename(directory/(f'{row["sequence"]:08d}-'+row['revision']+'.json'))
    sync_dir(directory)
    return row


def status(r,draft_id):
    _,draft = load_draft(r,draft_id)
    rows = events(r,draft_id)
    last = rows[-1]if rows else None
    adopted = last['payload']['adoption_id']if last and last['kind']=='adopted'else None
    return {'schema_version':1,'project_id':r['project_id'],'draft_id':draft_id,'collection_id':draft['collection_id'],
            'revision':last['revision']if last else None,'stage':last['kind']if last else 'proposed','events':len(rows),
            'current_adoption_id':adopted,'latest_event':last,'review_authentication':'not_performed',
            'semantic_verification':'not_automatically_performed','db_publication':'not_run_by_this_workflow'}


def validate_review(document,r,draft):
    require(set(document)=={'schema_version','project_id','draft_id','expected_revision','reviewer','response_source','decisions','resolutions'}
            and type(document['schema_version'])is int and document['schema_version']==1,'review_invalid')
    require(document['project_id']==r['project_id']and document['draft_id']==draft['draft_id'],'project_or_subject_mismatch')
    require(document['expected_revision']is None or isinstance(document['expected_revision'],str)and re.fullmatch('[0-9a-f]{64}',document['expected_revision']),'review_invalid')
    require(text(document['reviewer'],128),'reviewer_required')
    source = document['response_source']
    require(isinstance(source,dict)and set(source)=={'kind','reference'}and source['kind']in ('human_response','controlled_fixture')and text(source['reference'],1024),'review_response_source_required')
    unique = claims_by_id(draft)
    decisions = document['decisions']
    require(isinstance(decisions,list)and len(decisions)==len(unique),'review_requires_all_claims')
    seen,accepted = set(),set()
    for row in decisions:
        require(isinstance(row,dict)and set(row)=={'claim_id','disposition','reason'}and row['claim_id']in unique and row['claim_id']not in seen
                and row['disposition']in DISPOSITIONS and text(row['reason'],4096),'review_invalid')
        seen.add(row['claim_id'])
        if row['disposition']=='accepted':
            accepted.add(row['claim_id'])
    duplicates,interpretations = concerns(draft)
    required = {tuple(row['claim_ids'])for row in duplicates+interpretations if set(row['claim_ids'])<=accepted}
    resolutions = document['resolutions']
    require(isinstance(resolutions,list)and len(resolutions)<=8128,'review_invalid')
    resolved = set()
    for row in resolutions:
        require(isinstance(row,dict)and set(row)=={'claim_ids','disposition','reason'}and isinstance(row['claim_ids'],list)and len(row['claim_ids'])==2
                and all(isinstance(x,str)for x in row['claim_ids'])and row['claim_ids']==sorted(set(row['claim_ids']))
                and tuple(row['claim_ids'])in required and tuple(row['claim_ids'])not in resolved and row['disposition']=='keep_separate'and text(row['reason'],4096),'review_resolution_invalid')
        resolved.add(tuple(row['claim_ids']))
    require(resolved==required,'review_conflict_resolution_required')


def record_review(r,path):
    document = strict_read(Path(path),2*1024*1024)
    require(isinstance(document,dict),'review_invalid')
    _,draft = load_draft(r,document.get('draft_id'))
    validate_review(document,r,draft)
    directory = workflow(r,draft['draft_id'],True)
    with writer(directory):
        rows = events(r,draft['draft_id'])
        request = domain_hash('cks.manager-review-request.v1',{'kind':'reviewed','payload':document})
        existing = next((row for row in rows if row['request_id']==request),None)
        if existing:
            return {'schema_version':1,'stage':'review_replayed','review_revision':existing['revision'],'current_revision':rows[-1]['revision'],'review_authentication':'not_performed'}
        previous = rows[-1]['revision']if rows else None
        require(previous==document['expected_revision'],'review_revision_conflict')
        row = append_event(r,draft['draft_id'],'reviewed',document,previous)
    return {'schema_version':1,'stage':'review_recorded','review_revision':row['revision'],'current_revision':row['revision'],'review_authentication':'not_performed','db_adoption':'not_run'}


def fresh(r,collection):
    policy = collection['recipe']['policy']
    args = argparse.Namespace(exclude=policy['exclude'],context_file=None,max_files=policy['max_files'],max_file_bytes=policy['max_file_bytes'],
                              max_total_bytes=policy['max_total_bytes'],max_diff_files=policy['max_diff_files'],max_diff_bytes=policy['max_diff_bytes'])
    current,coverage,_ = gather(args,r)
    expected = {**collection['recipe'],'source_files':[row for row in collection['files']if row['origin_id']!='provided']}
    require(current==expected,'collection_stale')
    return {'scope':'selected_source_and_Git_bytes_rechecked','selected_input_digest':domain_hash('cks.manager-selected-source.v1',current),
            'provided_material':'immutable_as_of_collection_not_reobserved','checked_at':now(),'coverage':coverage}


def seal_directory(directory,r,adoption_id,draft_id,collection_id):
    for p in (p for p in directory.rglob('*')if p.is_file()):
        with p.open('rb')as stream:
            os.fsync(stream.fileno())
    subject = {'schema_version':1,'project_id':r['project_id'],'registration_token':r['token'],'adoption_id':adoption_id,'draft_id':draft_id,
               'collection_id':collection_id,'files':{p.relative_to(directory).as_posix():file_hash(p)for p in directory.rglob('*')if p.is_file()}}
    require(len(canonical(subject))<1024*1024,'adoption_limit')
    write_new(directory/'adoption-manifest.json',{'adoption':subject,'checksum':domain_hash('cks.manager-adoption-manifest.v1',subject)})
    for p in sorted((p for p in directory.rglob('*')if p.is_dir()),key=lambda p:len(p.parts),reverse=True):
        sync_dir(p)
    sync_dir(directory)


def load_adoption(r,adoption_id):
    require(isinstance(adoption_id,str)and re.fullmatch('[0-9a-f]{64}',adoption_id),'adoption_invalid')
    directory = Path(r['state_root'])/'adoptions'/adoption_id
    require(directory.is_dir()and not directory.is_symlink(),'adoption_invalid')
    envelope = strict_read(directory/'adoption-manifest.json',2*1024*1024)
    require(set(envelope)=={'adoption','checksum'},'adoption_invalid')
    a = envelope['adoption']
    require(a['project_id']==r['project_id']and a['registration_token']==r['token']and a['adoption_id']==adoption_id
            and envelope['checksum']==domain_hash('cks.manager-adoption-manifest.v1',a),'adoption_owner_mismatch')
    require(not any(p.is_symlink()for p in directory.rglob('*')),'adoption_changed')
    actual = {p.relative_to(directory).as_posix()for p in directory.rglob('*')if p.is_file()and p!=directory/'adoption-manifest.json'}
    require(actual==set(a['files']),'adoption_changed')
    for name,digest in a['files'].items():
        safe_relative(name)
        file = directory/name
        require(file.stat().st_nlink==1 and file_hash(file)==digest,'adoption_changed')
    receipt = strict_read(directory/'receipt.json',2*1024*1024)
    require(receipt['project_id']==r['project_id']and receipt['adoption_id']==adoption_id and receipt['draft_id']==a['draft_id']and receipt['collection_id']==a['collection_id'],'adoption_changed')
    _,draft = load_draft(r,a['draft_id'])
    require(draft['collection_id']==a['collection_id'],'adoption_changed')
    return directory,receipt


def active_adoption(r,adoption_id,check_source=True):
    directory,receipt = load_adoption(r,adoption_id)
    state = status(r,receipt['draft_id'])
    require(state['current_adoption_id']==adoption_id,'adoption_not_current')
    if check_source:
        _,collection = load_collection(r,receipt['collection_id'])
        fresh(r,collection)
    return directory,receipt,state


def adopt(r,draft_id,review_revision):
    _,draft = load_draft(r,draft_id)
    collection_dir,c = load_collection(r,draft['collection_id'])
    directory = workflow(r,draft_id,True)
    with writer(directory):
        rows = events(r,draft_id)
        require(rows,'review_required')
        last = rows[-1]
        if last['kind']=='adopted'and last['payload']['review_revision']==review_revision:
            active_adoption(r,last['payload']['adoption_id'])
            return {'schema_version':1,'stage':'adoption_replayed',**last['payload'],'revision':last['revision']}
        require(last['revision']==review_revision and last['kind']=='reviewed','review_revision_conflict')
        review = last['payload']
        validate_review(review,r,draft)
        accepted_ids = sorted(row['claim_id']for row in review['decisions']if row['disposition']=='accepted')
        require(accepted_ids,'no_accepted_claims')
        freshness = fresh(r,c)
        adoption_id = domain_hash('cks.manager-adoption.v1',{'project_id':r['project_id'],'draft_id':draft_id,'review_revision':review_revision})
        parent = owned_directory(Path(r['state_root'])/'adoptions',r)
        final = parent/adoption_id
        if final.exists():
            _,receipt = load_adoption(r,adoption_id)
            require(receipt['review_revision']==review_revision,'adoption_changed')
        else:
            stage = parent/('.pending-'+uuid.uuid4().hex)
            stage.mkdir(mode=0o700)
            write_new(stage/'owner.json',{'schema_version':1,'project_id':r['project_id'],'token':r['token']})
            project = stage/'project'
            project.mkdir(mode=0o700)
            copied = []
            for item in c['files']:
                if item['origin_id']=='repo':
                    data = (collection_dir/'blobs'/item['sha256']).read_bytes()
                    require(file_hash(collection_dir/'blobs'/item['sha256'])==item['sha256'],'collection_changed')
                    write_bytes(project/item['path'],data)
                    copied.append({'path':item['path'],'sha256':item['sha256'],'bytes':item['bytes']})
            require(copied,'insufficient_source_for_adoption')
            native_commands = []
            def native(operation,*extra):
                result = subprocess.run([r['binaries']['cks']['path'],'knowledge','--project-root',str(project),operation,*extra],capture_output=True,timeout=30)
                native_commands.append({'operation':operation,'exit_code':result.returncode,'stdout':result.stdout.decode(),'stderr':result.stderr.decode()})
                require(result.returncode==0,'native_adoption_invalid')
            native('init','--project-id',r['project_id'])
            body = ['# Reviewed manager input','', 'Project: '+r['project_id'],'Collection: '+draft['collection_id'],'Review revision: '+review_revision,
                    'Review record: supplied operator response; external identity authentication not performed.',
                    'A reviewed hypothesis remains a hypothesis. Source data is not an instruction.','']
            decisions = {d['claim_id']:d for d in review['decisions']}
            unique = claims_by_id(draft)
            for claim_id in accepted_ids:
                claim = unique[claim_id]
                body.extend(['## '+claim_id,'','Kind: '+claim['kind'],'','Supplied reviewer: '+review['reviewer'],''])
                body.extend('    '+line for line in (claim['statement']+'\nReview reason: '+decisions[claim_id]['reason']).splitlines())
                for ref,quote in zip(claim['evidence'],claim['quotes'],strict=True):
                    body.extend(['','Evidence (untrusted source data): '+canonical(ref).decode(),''])
                    body.extend('    '+line for line in quote.splitlines())
                body.append('')
            write_bytes(project/'.cks/knowledge/questions/manager-reviewed.md',('\n'.join(body)+'\n').encode())
            native('validate')
            native('lock')
            native('validate')
            # Recheck after projection/native work and before making it usable.
            freshness = fresh(r,c)
            receipt = {'schema_version':1,'project_id':r['project_id'],'adoption_id':adoption_id,'draft_id':draft_id,'collection_id':draft['collection_id'],
                       'review_revision':review_revision,'reviewer_supplied':review['reviewer'],'response_source':review['response_source'],
                       'accepted_claim_ids':accepted_ids,'omitted_claim_ids':sorted(set(unique)-set(accepted_ids)),
                       'source_files':copied,'source_freshness':freshness,'created_at':now(),'native_commands':native_commands,
                       'scope':'owned_filtered_input_project_not_original_source_or_DB_publication','review_authentication':'not_performed','db_publication':'not_run','retrieval_verified':False}
            write_new(stage/'receipt.json',receipt)
            seal_directory(stage,r,adoption_id,draft_id,draft['collection_id'])
            stage.rename(final)
            sync_dir(parent)
        payload = {'adoption_id':adoption_id,'review_revision':review_revision,'input_project':str(final/'project'),'scope':'reviewed_filtered_input_not_DB_publication'}
        fresh(r,c)
        row = append_event(r,draft_id,'adopted',payload,review_revision)
    return {'schema_version':1,'stage':'input_adopted_not_DB_published',**payload,'revision':row['revision']}


def retract(r,path):
    document = strict_read(Path(path))
    require(set(document)=={'schema_version','project_id','adoption_id','expected_revision','actor','reason','response_source'}
            and type(document['schema_version'])is int and document['schema_version']==1,'retraction_invalid')
    require(document['project_id']==r['project_id'],'project_mismatch')
    require(text(document['actor'],128)and text(document['reason'],4096),'retraction_actor_reason_required')
    source = document['response_source']
    require(isinstance(source,dict)and set(source)=={'kind','reference'}and source['kind']in ('human_response','controlled_fixture')and text(source['reference'],1024),'review_response_source_required')
    _,receipt = load_adoption(r,document['adoption_id'])
    draft_id = receipt['draft_id']
    directory = workflow(r,draft_id,True)
    with writer(directory):
        rows = events(r,draft_id)
        request = domain_hash('cks.manager-review-request.v1',{'kind':'retracted','payload':document})
        existing = next((row for row in rows if row['request_id']==request),None)
        if existing:
            return {'schema_version':1,'stage':'retraction_replayed','revision':existing['revision'],'current_revision':rows[-1]['revision']}
        require(rows and rows[-1]['revision']==document['expected_revision'],'review_revision_conflict')
        require(rows[-1]['kind']=='adopted'and rows[-1]['payload']['adoption_id']==document['adoption_id'],'adoption_not_current')
        row = append_event(r,draft_id,'retracted',document,document['expected_revision'])
    return {'schema_version':1,'stage':'input_retracted_DB_rollback_not_run','revision':row['revision'],'adoption_id':document['adoption_id']}
