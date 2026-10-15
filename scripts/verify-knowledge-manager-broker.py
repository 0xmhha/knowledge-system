#!/usr/bin/env python3
"""Actual plugin launcher -> scoped manager tools -> public CKS build/retained read."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]
MANAGER = ROOT/'plugins/knowledge-manager/scripts/manager.py'


def require(value,message):
    if not value:
        raise ValueError(message)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',required=True,type=Path)
    out = parser.parse_args().out.resolve()
    require(not out.exists(),'fresh owned output required')
    out.mkdir()
    checks,calls,commands,sessions = [],[],[],[]
    def write(name,value):
        (out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def checked(label,value):
        require(value,label)
        checks.append(label)
        write('checks.json',checks)
        print('PASS: '+label,flush=True)
    prepared = out/'prepared'
    run = subprocess.run([sys.executable,str(ROOT/'scripts/verify-knowledge-manager-review.py'),'--out',str(prepared)],cwd=ROOT,
                         env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1'),capture_output=True,timeout=120)
    (out/'prepare.stdout.txt').write_bytes(run.stdout)
    (out/'prepare.stderr.txt').write_bytes(run.stderr)
    commands.append({'label':'prepare','exit_code':run.returncode})
    write('commands.json',commands)
    require(run.returncode==0,'owned review fixture setup failed')
    registration = prepared/'manager-a/registration.json'
    r = json.loads(registration.read_text())['registration']
    profile = {'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id']}
    source_before = json.loads((prepared/'source-before.json').read_text())
    spec = importlib.util.spec_from_file_location('broker_wire_client',ROOT/'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(lib)
    def connect(label):
        folder = out/label
        folder.mkdir()
        client = lib.Client(None,None,folder,command=[sys.executable,str(MANAGER),'serve','--registration',str(registration)])
        sessions.append(client)
        reply = client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-manager-proxy','version':'1'}})
        require('error'not in reply,'broker initialize failed')
        return client
    def tool(client,name,args,error=None):
        reply = client.request('tools/call',{'name':name,'arguments':args})
        require('error'not in reply,'outer protocol failure')
        response = reply['result']
        calls.append({'session':client.root.name,'tool':name,'arguments':args,'response':response,'expected_error':error})
        write('calls.json',calls)
        if error:
            require(response.get('isError')is True,'expected broker refusal: '+name)
            if error is not True:
                code = json.loads(response['content'][0]['text'])['code']
                require(code==error,'different broker refusal: '+str(code))
            return response
        require(not response.get('isError'),'broker tool failed: '+str(response))
        return response['structuredContent']
    try:
        client = connect('manager-session')
        tools = client.request('tools/list',{})['result']['tools']
        write('tools.json',tools)
        names = {row['name']for row in tools}
        checked('actual launcher discovers 18 scoped tools without legacy mutation aliases',len(tools)==len(names)==18 and {'cks.dataset.build','cks.manager.select_input','cks.manager.resume_build','cks.context.get_for_task_v2'}<=names and not any(n.startswith('cks.ops.')for n in names))
        tool(client,'cks.context.get_for_task_v2',{'prompt':'Value'},error='reader_selection_required')
        tool(client,'cks.dataset.inspect_profile',profile,error='adoption_required')
        checked('metadata server cannot stand in for selected input or a real DB reader',not Path(r['dataset_root']).exists())
        collection = tool(client,'cks.manager.collect',{'exclude':['private/**'],'context_file':str(prepared/'provided.json')})
        original_cid = json.loads((prepared/'collect.stdout.txt').read_text())['collection_id']
        checked('actual manager collection tool replays immutable scoped evidence',collection['collection_id']==original_cid and collection['stage']=='collection_replayed')
        evidence = tool(client,'cks.manager.evidence',{'collection_id':original_cid,'path':'value.go','start_line':3,'end_line':3})
        checked('actual manager evidence tool preserves source span hashes',evidence['evidence']['project_id']==r['project_id']and evidence['evidence']['content_sha256']==hashlib.sha256(evidence['text'].encode()).hexdigest())
        proposed = tool(client,'cks.manager.draft',{'collection_id':original_cid,'claims_file':str(prepared/'claims.json')})
        did = proposed['draft_id']
        checked('actual manager draft tool replays proposed native knowledge',proposed['stage']=='draft_replayed')
        plan = tool(client,'cks.manager.review_plan',{'draft_id':did})
        checked('actual manager review plan preserves questions and no inferred approval',len(plan['questions'])==7 and plan['human_approval']=='not_inferred')
        response = json.loads((prepared/'review-input.json').read_text())
        response['expected_revision']=plan['revision']
        response['decisions'][0]['reason']='Explicit controlled proxy protocol response; no independent human acceptance.'
        write('response.json',response)
        reviewed = tool(client,'cks.manager.review',{'response_file':str(out/'response.json')})
        checked('actual manager review tool records supplied response without authentication claim',reviewed['stage']=='review_recorded'and reviewed['review_authentication']=='not_performed')
        adopted = tool(client,'cks.manager.adopt',{'draft_id':did,'review_revision':reviewed['review_revision']})
        write('adoption.json',adopted)
        selected = tool(client,'cks.manager.select_input',{'adoption_id':adopted['adoption_id']})
        checked('actual manager selects filtered adoption with hold-only activation',selected['selected_input']==adopted['adoption_id']and selected['activation_policy']=='hold_for_review_only')
        inspection = tool(client,'cks.dataset.inspect_profile',profile)
        write('inspection.json',inspection)
        tool(client,'cks.dataset.inspect_profile',{**profile,'project_id':'foreign-project'},error='project_or_profile_mismatch')
        checked('foreign project cannot inspect registered maintenance',True)
        p = inspection['profile']
        request = {**profile,**{key:p[key]for key in ('profile_digest','source_pin','input_digest','embedding_digest')},'base':p['observed_base'],
                   'new_version':'broker-reviewed-v1','activation':'hold_for_review','idempotency_key':'owned-manager-proxy-0001','deadline_seconds':120}
        write('request.json',request)
        tool(client,'cks.dataset.build',dict(request,activation='activate_after_gate'),error='activation_not_authorized')
        checked('input review does not authorize automatic dataset publication',not Path(r['dataset_root']).exists())
        job = tool(client,'cks.dataset.build',request)
        ref = {**profile,'job_id':job['job_id']}
        deadline = time.monotonic()+120
        observations = [job]
        while not job['terminal']:
            require(time.monotonic()<deadline,'actual broker build timeout')
            time.sleep(.05)
            job = tool(client,'cks.dataset.status',ref)
            observations.append(job)
        write('observations.json',observations)
        write('job.json',job)
        checked('actual manager routes public build to a gated held candidate',job['state']=='held_for_review'and job['activated']is False and all(job['gates'][g]=='passed'for g in p['required_gates']))
        coordinates = job['candidate']['coordinates']
        tool(client,'cks.manager.select_reader',{'job_id':job['job_id'],'expected_coordinates':dict(coordinates,dataset_id='0'*64)},error='candidate_input_mismatch')
        checked('reader never falls back from mismatched candidate coordinates',True)
        chosen = tool(client,'cks.manager.select_reader',{'job_id':job['job_id'],'expected_coordinates':coordinates})
        checked('actual reader initialization is separated from an observed read',chosen['stage']=='reader_initialized_unqueried'and chosen['serving_read_observed']is False)
        pack = tool(client,'cks.context.get_for_task_v2',{'prompt':'bounded queue fixture owner reports'})
        write('read-first.json',pack)
        checked('actual broker returns reviewed document bodies and project-scoped citations',pack['coordinates']==coordinates and any('bounded queue'in json.dumps(body)for body in pack['bodies'])and any(c['origin_id']=='docs:corpus.0'for c in pack['citations'])and all(c['project_id']==r['project_id']for c in pack['citations']))
        state = tool(client,'cks.manager.status',{})
        write('state-after-read.json',state)
        checked('only a validated actual response records observed reader coordinates',state['last_validated_read']['coordinates']==coordinates and state['last_validated_read']['candidate_publication_inferred']is False)
        client.close()
        restarted = connect('manager-restart-session')
        repeated = tool(restarted,'cks.context.get_for_task_v2',{'prompt':'bounded queue fixture owner reports'})
        write('read-restart.json',repeated)
        checked('restarted actual launcher revalidates same stored job and immutable bodies',repeated['coordinates']==pack['coordinates']and repeated['citations']==pack['citations']and repeated['bodies']==pack['bodies'])
        source = prepared/'source-a/value.go'
        original = source.read_bytes()
        source.write_text('package fixture\nfunc Value()int{return 999}\n')
        tool(restarted,'cks.context.get_for_task_v2',{'prompt':'bounded queue'},error='collection_stale')
        tool(restarted,'cks.dataset.build',request,error='collection_stale')
        checked('source drift refuses both current manager read and new admission',True)
        status = tool(restarted,'cks.dataset.status',ref)
        replay = tool(restarted,'cks.manager.resume_build',{'job_id':job['job_id'],'original_request':request})
        checked('known admitted job status and exact replay survive source drift',status==replay==job)
        tool(restarted,'cks.manager.resume_build',{'job_id':job['job_id'],'original_request':dict(request,new_version='different-intent')},error='original_request_mismatch')
        checked('resume cannot admit a changed request under an existing job',True)
        source.write_bytes(original)
        identity = Path(r['dataset_root'])/job['target_version']/'dataset-identity.json'
        original_identity = identity.read_bytes()
        identity.write_text('{}')
        corrupted = tool(restarted,'cks.context.get_for_task_v2',{'prompt':'bounded queue'},error=True)
        write('corruption-refusal.json',corrupted)
        checked('already open actual reader refuses corrupted dataset identity',corrupted.get('isError')is True)
        identity.write_bytes(original_identity)
        restored = tool(restarted,'cks.context.get_for_task_v2',{'prompt':'bounded queue fixture owner reports'})
        checked('restored fixture identity returns same pinned coordinates',restored['coordinates']==coordinates)
        retraction = {'schema_version':1,'project_id':r['project_id'],'adoption_id':adopted['adoption_id'],'expected_revision':adopted['revision'],
                      'actor':'explicit-fixture-reviewer','reason':'Controlled broker input withdrawal; published DB rollback not requested.',
                      'response_source':response['response_source']}
        write('retraction.json',retraction)
        withdrawn = tool(restarted,'cks.manager.retract',{'response_file':str(out/'retraction.json')})
        tool(restarted,'cks.context.get_for_task_v2',{'prompt':'bounded queue'},error='adoption_not_current')
        checked('input withdrawal blocks a previously selected actual reader',withdrawn['stage']=='input_retracted_DB_rollback_not_run')
        replay = tool(restarted,'cks.manager.resume_build',{'job_id':job['job_id'],'original_request':request})
        checked('withdrawal preserves original held job without creating publication',replay==job and not Path(r['dataset_root']).joinpath('current').exists())
        restarted.close()
        third = connect('manager-retired-restart-session')
        tool(third,'cks.context.get_for_task_v2',{'prompt':'bounded queue'},error='adoption_not_current')
        checked('restart does not silently revive withdrawn reader selection',True)
        third.close()
        after = {str(file.relative_to(prepared)):hashlib.sha256(file.read_bytes()).hexdigest()for name in ('source-a','source-b')for file in(prepared/name).rglob('*')if file.is_file()}
        checked('original project bytes and excluded private material remain unchanged',after==source_before and not(prepared/'source-a/.cks').exists())
        checked('actual plugin launcher sessions and all owned native children close cleanly',len(sessions)==3 and all(c.proc.returncode==0 for c in sessions))
    finally:
        for client in sessions:
            if client.proc.poll()is None:
                client.close()
    summary = {'schema_version':1,'scope':'KM03_4_B_actual_plugin_launcher_and_adoption_bound_maintenance_reader_only','checks':len(checks),
               'MCP_sessions':len(sessions),'EOF_exit_codes':[c.proc.returncode for c in sessions],'paid_model_calls':0,'embedding':'mock_lifecycle_not_quality',
               'candidate_state':'held_for_review','DB_published':False,'manager_reader_broker_verified':True,'actual_Claude_host_workflow_verified':False,
               'KM03_4_complete':False,'full_KM03_complete':False,'independent_acceptance':False}
    write('summary.json',summary)
    print(json.dumps(summary))


if __name__=='__main__':
    main()
