#!/usr/bin/env python3
"""Actual native publication rechecks the original live manager source."""
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
    spec=importlib.util.spec_from_file_location('source_check_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)
    def connect(label,config):
        folder=out/label;folder.mkdir();client=wire.Client(None,None,folder,command=[str(ROOT/'bin/cks'),'mcp','--config',str(config)],namespace='');sessions.append(client)
        require('error'not in client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-original-source-check','version':'1'}}),'initialize failed');return client
    def tool(client,name,arguments,error=None):
        response=client.request('tools/call',{'name':name,'arguments':arguments});require('error'not in response,'outer protocol failed');result=response['result']
        calls.append({'session':client.root.name,'tool':name,'arguments':arguments,'response':result,'expected_error':error});write('calls.json',calls)
        if error:
            require(result.get('isError')is True,'expected tool refusal');value=result.get('structuredContent')
            require(isinstance(value,dict)and value.get('code')==error,'different refusal: '+str(value));return result
        require(not result.get('isError'),'native failure: '+str(result));return result['structuredContent']
    def close(client):client.close();client.proc.stdout.close()
    source=out/'source';source.mkdir();code=source/'value.go';code.write_text('package fixture\nfunc Value()int{return 7}\n');(source/'go.mod').write_text('module example.invalid/source-check\n\ngo 1.26.9\n')
    (source/'.env').write_text('PRIVATE_SOURCE_CHECK_SENTINEL\n')
    graph=out/'owned-graph.py'
    graph.write_text('#!'+str(Path(sys.executable).resolve())+'\nimport os,sys,time\nfrom pathlib import Path\nroot=Path('+repr(str(out))+')\n'
        'if len(sys.argv)>1 and sys.argv[1]=="build" and (root/"delay-graph").exists():\n'
        ' (root/"graph-running").write_text(str(os.getpid()))\n while not (root/"allow-graph").exists(): time.sleep(.01)\n'
        'os.execv('+repr(str(ROOT/'bin/ckg'))+',['+repr(str(ROOT/'bin/ckg'))+']+sys.argv[1:])\n');graph.chmod(0o700)
    result=manager.register(argparse.Namespace(source=str(source),dataset=str(out/'dataset'),state_dir=str(out/'manager'),cks_bin=str(ROOT/'bin/cks'),
        graph_bin=str(graph),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
    r=manager.load_registration(result['registration_file']);policy=contract.defaults(r);policy.update(enabled=True,activation='activate_after_gate',deadline_seconds=120)
    policy_path,policy=inputs.pin_policy(policy,r);write('policy.json',policy)
    bound=out/'guard-proof.txt';bound.write_text('owned stable guard input\n')
    scope={'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id']}
    def configured(label,prepared):
        setup=manager.strict_read(Path(r['state_root'])/'setup.json');setup.update(src=prepared['input_project'],source_mode='snapshot-only',build_timeout='120s',document_corpora=[str(Path(prepared['input_project'])/'.cks/knowledge/questions')])
        write(label+'-setup.json',setup);config=manager.strict_read(Path(r['state_root'])/'cks.json');check=inputs.source_check(r,policy_path,prepared['input_id']);check['pinned_files'][str(bound)]=manager.file_hash(bound)
        config['maintenance']['profiles'][0].update(setup_config=str(out/(label+'-setup.json')),allowed_activation=['activate_after_gate'],timeout_seconds=120,source_check=check)
        write(label+'-maintenance.json',config);client=connect(label+'-maintenance-session',out/(label+'-maintenance.json'))
        profile=tool(client,'cks.dataset.inspect_profile',scope)['profile'];write(label+'-profile.json',profile)
        request={**scope,**{k:profile[k]for k in ('profile_digest','source_pin','input_digest','embedding_digest')},'base':profile['observed_base'],
                 'new_version':label,'activation':'activate_after_gate','idempotency_key':'owned-original-check-'+label,'deadline_seconds':120}
        write(label+'-request.json',request);return client,request,profile
    def wait(client,job):
        end=time.monotonic()+120
        while not job['terminal']:
            require(time.monotonic()<end,'native build deadline');time.sleep(.05);job=tool(client,'cks.dataset.status',{**scope,'job_id':job['job_id']})
        return job
    def read(label,prepared,job):
        version=Path(r['dataset_root'])/job['candidate']['version'];vector=json.loads((version/'vector/manifest.json').read_text())
        cfg={'version':1,'name':'cks','listen':{'transport':'stdio','mcp_stdio':True},'logging':{'level':'error','mode':'prod'},
             'sanitize':{'rules_path':str(Path(r['state_root'])/'sanitization_rules.yaml'),'default_action':'drop','fail_closed_on_unknown_rule':True},
             'backends':{'ckg':{'path':str(version/'graph/graph.db'),'source_root':prepared['input_project'],'timeout_ms':5000},
                         'ckv':{'path':str(version/'vector'),'provider':'mock','embed_model':vector['embedding_model']}}}
        write(label+'-reader.json',cfg);reader=connect(label+'-reader-session',out/(label+'-reader.json'));pack=tool(reader,'cks.context.get_for_task_v2',{'prompt':'Value observed source'})
        write(label+'-read.json',pack);require(pack['coordinates']==job['candidate']['coordinates'],'wrong reader');return reader,pack
    try:
        first=inputs.prepare(policy,r);write('v1-prepared.json',first);client,request,profile=configured('guarded-v1',first)
        checked('actual profile admits a pinned original-source verification command',bool(json.loads((out/'guarded-v1-maintenance.json').read_text())['maintenance']['profiles'][0]['source_check']['pinned_files']))
        job1=wait(client,tool(client,'cks.dataset.build',request));write('v1-job.json',job1)
        observed=tool(client,'cks.dataset.inspect_profile',scope)['profile'];checked('unchanged original source can actually pass native gates and publish',contract.completed(job1,policy,profile['required_gates'],observed['observed_base']['coordinates'])=='succeeded')
        write('v1-current.json',observed['observed_base'])
        old_reader,old_pack=read('v1',first,job1);checked('actual published v1 reads retained original code',any('return 7'in json.dumps(b)for b in old_pack['bodies']))
        bound.write_text('changed owned guard input\n');tool(client,'cks.dataset.inspect_profile',scope,error='profile_stale');bound.write_text('owned stable guard input\n')
        checked('changed pinned check dependency is refused by actual native inspection',True)
        tool(client,'cks.dataset.inspect_profile',{**scope,'source_check':{}},error='invalid_request');checked('public tool arguments cannot select a source-check executable',True);close(client)
        code.write_text('package fixture\nfunc Value()int{return 8}\n');second=inputs.prepare(policy,r,first['input_id']);write('v2-prepared.json',second)
        client,request2,_=configured('guarded-v2',second);(out/'delay-graph').write_text('controlled lifecycle delay\n');job=tool(client,'cks.dataset.build',request2)
        end=time.monotonic()+30
        while not(out/'graph-running').exists():
            require(time.monotonic()<end,'actual graph did not enter controlled delay');time.sleep(.01)
        checked('actual v2 graph started from immutable observed source8 before external edit',job['terminal']is False)
        code.write_text('package fixture\nfunc Value()int{return 9}\n');(out/'allow-graph').write_text('continue actual native graph\n')
        job2=wait(client,job);write('v2-job.json',job2)
        write('v2-current.json',{'link_target':str((Path(r['dataset_root'])/'current').readlink()),
                               'original_source_sha256':manager.file_hash(code),'immutable_v2_source_sha256':manager.file_hash(Path(second['input_project'])/'value.go')})
        checked('native final original-source check rejects stale candidate before current swap',job2['state']=='failed'and job2['activated']is False and job2['error']['code']=='source_stale'and (Path(r['dataset_root'])/'current').readlink()==Path('guarded-v1'))
        replay=tool(client,'cks.dataset.build',request2);write('v2-replay.json',replay);checked('exact admitted replay survives a now-stale source check',replay==job2);close(client)
        retained=tool(old_reader,'cks.context.get_for_task_v2',{'prompt':'Value observed source'});write('v1-after-refusal.json',retained)
        checked('already-open old reader preserves original bodies and citations after refusal',retained['bodies']==old_pack['bodies']and retained['citations']==old_pack['citations']);close(old_reader)
        (out/'delay-graph').unlink();third=inputs.prepare(policy,r,second['input_id']);write('v3-prepared.json',third);client,request3,profile3=configured('guarded-v3',third)
        job3=wait(client,tool(client,'cks.dataset.build',request3));write('v3-job.json',job3);current=tool(client,'cks.dataset.inspect_profile',scope)['profile']['observed_base']
        write('v3-current.json',current)
        checked('new original source9 can pass a new explicitly scoped publication',contract.completed(job3,policy,profile3['required_gates'],current['coordinates'])=='succeeded'and current['version']=='guarded-v3');close(client)
        fresh_reader,fresh_pack=read('v3',third,job3);checked('new actual reader returns code9 with exact new coordinates',any('return 9'in json.dumps(b)for b in fresh_pack['bodies'])and fresh_pack['coordinates']!=old_pack['coordinates']);close(fresh_reader)
        checked('source receives only deliberate fixture edits and no manager knowledge overlay',sorted(p.name for p in source.iterdir())==['.env','go.mod','value.go'])
        checked('no review or semantic approval is manufactured',not(Path(r['state_root'])/'workflows').exists()and third['semantic_review']=='not_performed'and third['human_approval']=='not_inferred')
        checked('all actual native connections close on EOF0',all(c.proc.returncode==0 for c in sessions))
    finally:
        for client in sessions:
            if client.proc.poll()is None:close(client)
    summary={'schema_version':1,'scope':'KM04_3_A_actual_original_source_publication_check_not_worker','checks':len(checks),'MCP_sessions':len(sessions),'EOF_exit_codes':[c.proc.returncode for c in sessions],
             'actual_tool_calls':len(calls),'builds':3,'published':2,'stale_refusals':1,'source_check_checkpoint_complete':True,'full_KM04_3_complete':False,
             'scheduler_runtime_verified':False,'paid_model_calls':0,'embedding':'mock_lifecycle_not_quality','independent_acceptance':False}
    write('summary.json',summary);print(json.dumps(summary))


if __name__=='__main__':main()
