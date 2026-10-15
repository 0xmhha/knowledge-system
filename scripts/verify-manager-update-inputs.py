#!/usr/bin/env python3
"""Actual public builds/reads for observed source sets; not a scheduler proof."""
import argparse
import importlib.util
import json
from pathlib import Path
import sys
import time

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
import update_contract as contract
import update_inputs as inputs


def require(value,message):
    if not value:raise ValueError(message)


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',required=True,type=Path)
    out=parser.parse_args().out.resolve();require(not out.exists(),'fresh owned output required');out.mkdir()
    checks,calls,sessions=[],[],[]
    def write(name,value):(out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def checked(name,value):
        require(value,name);checks.append(name);write('checks.json',checks);print('PASS: '+name,flush=True)
    spec=importlib.util.spec_from_file_location('update_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)
    def connect(label,config,r):
        folder=out/label;folder.mkdir();client=wire.Client(None,None,folder,command=[r['binaries']['cks']['path'],'mcp','--config',str(config)],namespace='')
        sessions.append(client);reply=client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-update-input-verifier','version':'1'}})
        require('error'not in reply,'initialize failed');return client
    def tool(client,name,arguments):
        reply=client.request('tools/call',{'name':name,'arguments':arguments});require('error'not in reply,'outer native protocol failure')
        response=reply['result'];calls.append({'session':client.root.name,'tool':name,'arguments':arguments,'response':response});write('calls.json',calls)
        require(not response.get('isError'),'native tool refused: '+str(response));return response['structuredContent']
    def close(client):
        client.close();client.proc.stdout.close()
    def registration(label):
        source=out/('source-'+label);source.mkdir()
        (source/'go.mod').write_text('module example.invalid/observed\n\ngo 1.26.9\n')
        (source/'value.go').write_text('package observed\n\nfunc Value() int { return 7 }\n')
        (source/'.env').write_text('PRIVATE_UPDATE_INPUT_SENTINEL\n')
        result=manager.register(argparse.Namespace(source=str(source),dataset=str(out/('dataset-'+label)),state_dir=str(out/('manager-'+label)),
            cks_bin=str(ROOT/'bin/cks'),graph_bin=str(ROOT/'bin/ckg'),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
        r=manager.load_registration(result['registration_file']);policy=contract.defaults(r);policy['enabled']=True;policy['deadline_seconds']=120
        return source,r,policy
    def reader(label,r,prepared,job):
        version=Path(r['dataset_root'])/job['candidate']['version'];vector=json.loads((version/'vector/manifest.json').read_text());setup=manager.strict_read(Path(r['state_root'])/'setup.json')
        config={'version':1,'name':'cks','listen':{'transport':'stdio','mcp_stdio':True},'logging':{'level':'error','mode':'prod'},
                'sanitize':{'rules_path':str(Path(r['state_root'])/'sanitization_rules.yaml'),'default_action':'drop','fail_closed_on_unknown_rule':True},
                'backends':{'ckg':{'path':str(version/'graph/graph.db'),'source_root':prepared['input_project'],'timeout_ms':5000},
                            'ckv':{'path':str(version/'vector'),'provider':'mock','embed_model':vector['embedding_model']}}}
        write(label+'-reader.json',config);client=connect(label+'-reader-session',out/(label+'-reader.json'),r)
        pack=tool(client,'cks.context.get_for_task_v2',{'prompt':'Value transaction admission observed material','limits':{'max_tokens':20000}})
        write(label+'-read.json',pack);close(client)
        require(pack['coordinates']==job['candidate']['coordinates'],'reader changed coordinates')
        return pack
    def build(label,r,policy,prepared):
        setup=manager.strict_read(Path(r['state_root'])/'setup.json');setup.update(src=prepared['input_project'],source_mode='snapshot-only',build_timeout='120s',
            document_corpora=[str(Path(prepared['input_project'])/'.cks/knowledge/questions')]+([prepared['material_corpus']]if prepared['material_documents']else[]))
        write(label+'-setup.json',setup);config=manager.strict_read(Path(r['state_root'])/'cks.json')
        config['maintenance']['profiles'][0].update(setup_config=str(out/(label+'-setup.json')),allowed_activation=['hold_for_review'],timeout_seconds=120)
        write(label+'-maintenance.json',config);client=connect(label+'-maintenance-session',out/(label+'-maintenance.json'),r)
        scope={'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id']};profile=tool(client,'cks.dataset.inspect_profile',scope)['profile'];write(label+'-profile.json',profile)
        request={**scope,**{key:profile[key]for key in ('profile_digest','source_pin','input_digest','embedding_digest')},'base':profile['observed_base'],
                 'new_version':label,'activation':'hold_for_review','idempotency_key':'owned-observed-input-'+label,'deadline_seconds':120};write(label+'-request.json',request)
        job=tool(client,'cks.dataset.build',request);deadline=time.monotonic()+120
        while not job['terminal']:
            require(time.monotonic()<deadline,'build deadline');time.sleep(.05)
            job=tool(client,'cks.dataset.status',{**scope,'job_id':job['job_id']})
        write(label+'-job.json',job);require(contract.completed(job,policy,profile['required_gates'])=='held','build was not held')
        close(client);pack=reader(label,r,prepared,job)
        return profile,job,pack
    def snapshot(root):return {p.relative_to(root).as_posix():inputs.checksum(p.read_bytes())for p in root.rglob('*')if p.is_file()}
    try:
        source,r,policy=registration('a');other_source,other,other_policy=registration('b')
        material=source/'.cks/ontology';material.mkdir(parents=True);(material/'terms.md').write_text('# Transaction admission\n\nObserved transaction queue policy text: seven slots.\n')
        policy['collection']['materials']=[{'id':'terms','role':'ontology','root':str(material)}];write('policy.json',policy)
        source_before=snapshot(source);write('source-before.json',source_before)
        first=inputs.prepare(policy,r);write('v1-prepared.json',first)
        checked('document-free code is prepared without inferred review or DB',first['semantic_review']=='not_performed'and first['human_approval']=='not_inferred'and not first['DB_built'])
        profile1,job1,pack1=build('observed-v1',r,policy,first)
        checked('actual native build and read retain observed code and scoped material',any('return 7'in json.dumps(b)for b in pack1['bodies'])and any('seven slots'in json.dumps(b)for b in pack1['bodies'])and any(c['origin_id']=='docs:corpus.1'for c in pack1['citations']))
        checked('preparing and building left original source byte-identical',snapshot(source)==source_before)
        second_project=inputs.prepare(other_policy,other);write('other-prepared.json',second_project);_,other_job,_=build('other-v1',other,other_policy,second_project)
        other_before=snapshot(Path(other['dataset_root']));write('other-before.json',other_before)
        checked('same module/function bytes remain separate project/dataset identities',job1['project_id']!=other_job['project_id']and job1['candidate']['coordinates']!=other_job['candidate']['coordinates'])
        again=inputs.prepare(policy,r,first['input_id']);write('noop-prepared.json',again)
        last={'input_id':first['input_id'],'policy_digest':inputs.domain_hash('cks.manager-update-policy.v1',policy),'job_id':job1['job_id'],'candidate':job1['candidate']}
        # Fresh native status/inspect and read must confirm an unchanged input.
        client=connect('noop-maintenance-session',out/'observed-v1-maintenance.json',r)
        scope={'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id']}
        profile=tool(client,'cks.dataset.inspect_profile',scope)['profile'];job=tool(client,'cks.dataset.status',{**scope,'job_id':job1['job_id']});close(client)
        fresh_pack=reader('noop',r,first,job);skip=inputs.may_skip(last,policy,again,profile,job,read_pack=fresh_pack)
        write('noop-validation.json',{'last':last,'profile':profile,'job':job,'read_pack':fresh_pack,'may_skip':skip})
        checked('unchanged input is skippable only with actual matching healthy native candidate',skip and again['change']['input_unchanged'])
        checked('metadata alone cannot prove no-op',not inputs.may_skip(last,policy,again,profile,job))
        (source/'value.go').write_text('package observed\n\nfunc Value() int { return 8 }\n');(material/'terms.md').write_text('# Transaction admission\n\nObserved transaction queue policy text: eight slots.\n')
        changed=inputs.prepare(policy,r,first['input_id']);write('v2-prepared.json',changed)
        checked('source and ontology original bytes independently change input without Git commit',changed['input_id']!=first['input_id']and {d['origin_id']for d in changed['change']['modified']}=={'repo','material:terms'}and changed['change']['semantic_review_carried_forward']is False)
        _,job2,pack2=build('observed-v2',r,policy,changed)
        checked('new actual candidate reads updated code and raw ontology material',any('return 8'in json.dumps(b)for b in pack2['bodies'])and any('eight slots'in json.dumps(b)for b in pack2['bodies']))
        checked('old candidate cannot satisfy changed-input no-op',not inputs.may_skip(last,policy,changed,profile,job,read_pack=fresh_pack))
        old_read=reader('v1-after-change',r,first,job1)
        checked('previous native snapshot still returns original bodies and citations',old_read['bodies']==pack1['bodies']and old_read['citations']==pack1['citations'])
        (source/'value.go').rename(source/'renamed.go');(material/'terms.md').unlink()
        deleted=inputs.prepare(policy,r,changed['input_id']);write('v3-prepared.json',deleted)
        checked('delete plus rename candidate is explicit and deleted material omitted',deleted['material_documents']==0 and any(d['origin_id']=='material:terms'for d in deleted['change']['deleted'])and len(deleted['change']['rename_candidates'])==1)
        _,job3,pack3=build('observed-v3',r,policy,deleted)
        checked('actual newest DB drops deleted material and retains renamed code',any('return 8'in json.dumps(b)for b in pack3['bodies'])and 'eight slots'not in json.dumps(pack3['bodies'])and 'seven slots'not in json.dumps(pack3['bodies']))
        before_failure=snapshot(Path(r['dataset_root']))
        invalid=json.loads(json.dumps(policy));invalid['project_id']='foreign'
        try:inputs.prepare(invalid,r,deleted['input_id'])
        except manager.Failure as error:write('failed-preparation.json',{'code':str(error),'DB_unchanged':snapshot(Path(r['dataset_root']))==before_failure})
        else:raise ValueError('foreign policy accepted')
        checked('failed preparation leaves all existing dataset bytes unchanged',snapshot(Path(r['dataset_root']))==before_failure)
        checked('other project dataset stays byte-identical across all changes',snapshot(Path(other['dataset_root']))==other_before)
        checked('held inputs never publish current or fabricate review workflow',not(Path(r['dataset_root'])/'current').exists()and not(Path(other['dataset_root'])/'current').exists()and not(Path(r['state_root'])/'workflows').exists())
        checked('all owned native sessions close on EOF0',all(c.proc.returncode==0 for c in sessions))
    finally:
        for client in sessions:
            if client.proc.poll()is None:close(client)
    summary={'schema_version':1,'scope':'KM04_2_observed_inputs_and_manual_native_build_reads_not_scheduler','checks':len(checks),'MCP_sessions':len(sessions),
             'EOF_exit_codes':[c.proc.returncode for c in sessions],'actual_tool_calls':len(calls),'builds':4,'held_candidates':4,
             'source_changes':2,'native_noop_validation':True,'scheduler_runtime_verified':False,'DB_published':False,
             'paid_model_calls':0,'embedding':'mock_lifecycle_not_quality','semantic_approval_inferred':False,'full_KM04_complete':False,'independent_acceptance':False}
    write('summary.json',summary);print(json.dumps(summary))


if __name__=='__main__':main()
