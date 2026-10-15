#!/usr/bin/env python3
"""Actual read-only native comparison MCP with two owned real mock indexes."""
import argparse
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import sys
import time
from datetime import datetime,timezone

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
import update_contract
import update_worker


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    checks,calls,sessions=[],[],[]
    def write(name,value):(out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def check(label,value):manager.require(value,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    spec=importlib.util.spec_from_file_location('comparison_native_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)
    fixtures=[]
    for label,method,value in [('a','Stake',32),('b','Bond',64)]:
        base=out/label;base.mkdir();source=base/'source';source.mkdir();(source/'go.mod').write_text('module example.invalid/comparison-fixture\n\ngo 1.26.9\n');(source/'stake.go').write_text('package fixture\nfunc '+method+'()int{return '+str(value)+'}\n');(source/'POLICY.md').write_text('Controlled fixture '+method+' entry uses '+str(value)+' units. This is not a real chain policy.\n')
        manager.register(argparse.Namespace(source=str(source),dataset=str(base/'dataset'),state_dir=str(base/'manager'),cks_bin=str(ROOT/'bin/cks'),graph_bin=str(ROOT/'bin/ckg'),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
        registration=str(base/'manager/registration.json');r=manager.load_registration(registration);worker=update_worker.Worker(registration)
        try:
            policy=update_contract.defaults(r);policy.update(enabled=True,activation='activate_after_gate',deadline_seconds=120);worker.journal.configure(policy);end=time.monotonic()+100
            while True:
                worker.tick();success=worker.journal.state['last_success']
                if success:row=copy.deepcopy(worker.journal.row(success));break
                manager.require(time.monotonic()<end,'build_timeout');time.sleep(.05)
        finally:worker.close()
        version=base/'dataset'/row['job']['candidate']['version'];manifest=json.loads((version/'sources/manifest.json').read_text());origins=sorted({'repo'}|{f['origin_id']for f in manifest['files']});conditions={'network':'fixture-network','configuration':'fixture-config','component_role':'staking-network','observed_at':datetime.now(timezone.utc).isoformat(timespec='seconds')}
        item={'label':label,'method':method,'value':value,'root':str(base),'version':str(version),'coordinates':row['job']['candidate']['coordinates'],'origins':origins,'conditions':conditions};fixtures.append(item);write(label+'-job.json',row)
        check('actual native graph vector and retained source build '+label,row['outcome']=='succeeded'and(version/'graph/graph.db').is_file()and(version/'vector/manifest.json').is_file()and(version/'vector/vector.db').is_file())
    catalog={'schema_version':1,'projects':[{'id':f['coordinates']['project_id'],'name':'same-name'}for f in fixtures], 'versions':[{'coordinates':f['coordinates'],'version_dir':f['version'],'origins':f['origins']}for f in fixtures], 'collections':[{'id':'chains','members':[f['coordinates']for f in fixtures]}]};write('catalog.json',catalog)
    scopes=[{'coordinates':f['coordinates'],'industry':'blockchain','conditions':f['conditions']}for f in fixtures]
    profile={'schema_version':1,'name':'owned-comparison-MCP','catalog_path':str(out/'catalog.json'),'scheme_root':str(ROOT/'system/packs/industry-blockchain'),'sanitize_rules_path':str(ROOT/'plugins/knowledge-manager/config/sanitization_rules.yaml'),'scopes':scopes};write('profile.json',profile)
    def connect(label):
        directory=out/label;directory.mkdir();client=wire.Client(None,None,directory,command=[str(ROOT/'bin/cks'),'mcp','comparison','--profile',str(out/'profile.json')],namespace='');sessions.append(client);client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-comparison','version':'1'}});return client
    def tool(client,operation,args=None,error=None):
        response=client.request('tools/call',{'name':'cks.comparison.'+operation,'arguments':args or{}})['result'];calls.append({'session':client.root.name,'tool':'cks.comparison.'+operation,'arguments':copy.deepcopy(args or{}),'response':copy.deepcopy(response),'expected_error':error});write('calls.json',calls)
        if error:manager.require(response.get('isError')and response['structuredContent']['code']==error,'wrong actual refusal');return response
        manager.require(not response.get('isError'),'actual tool refusal: '+str(response));return response['structuredContent']
    first=connect('bootstrap-session')
    try:initial=tool(first,'describe');write('initial-description.json',initial)
    finally:first.close();first.proc.stdout.close()
    key=next(c['key']for c in initial['concepts']if c['key']['concept_id']=='staking');bindings=[]
    for f in fixtures:
        version=Path(f['version']);manifest=json.loads((version/'sources/manifest.json').read_text());evidence=[]
        for filename,kind,line in [('stake.go','code',2),('POLICY.md','policy',1)]:
            rec=next(x for x in manifest['files']if x['origin_id']=='repo'and x['path']==filename);data=(version/'sources/blobs'/rec['sha256']).read_bytes();body=data.splitlines(keepends=True)[line-1];c={**f['coordinates'],'file':filename,'start_line':line,'end_line':line,'commit_hash':'','origin_id':'repo','file_sha256':rec['sha256'],'content_sha256':hashlib.sha256(body).hexdigest()};evidence.append({'id':'evidence-'+kind,'kind':kind,'facets':['entry'],'citation':c})
        bindings.append({'id':'staking','concept':key,'coordinates':f['coordinates'],'conditions':f['conditions'],'local_concept':f['method'],'mapping':'close','reason':'Controlled fixture mapping candidate, source integrity only; no real chain or human approval.','evidence':evidence})
    write('bindings.json',{'schema_version':2,'bindings':bindings});profile['bindings_path']=str(out/'bindings.json');write('profile.json',profile)
    client=connect('matrix-session')
    try:
        tools=client.request('tools/list')['result']['tools'];write('tools.json',tools);check('actual native isolated server exposes only two readonly comparison tools',{t['name']for t in tools}=={'cks.comparison.describe','cks.comparison.extract'}and all(t['annotations']=={'readOnlyHint':True,'destructiveHint':False,'idempotentHint':True,'openWorldHint':False}for t in tools))
        description=tool(client,'describe');write('description.json',description);check('configuration is distinct from archive readiness',description['state']=='configured'and description['source_readiness']=='not_checked'and description['write_enabled']is False and len(description['scopes'])==2)
        ids=[next(x['scope_id']for x in description['scopes']if x['scope']['coordinates']==f['coordinates'])for f in fixtures]
        query={'schema_version':1,'profile_digest':description['profile_digest'],'catalog_digest':description['catalog_digest'],'collection_id':'chains','mode':'comparison','participants':ids,'concept':key,'facets':['entry','exit'],'as_of':datetime.now(timezone.utc).isoformat(timespec='seconds'),'budget':{'max_body_bytes':262144,'max_citations':128,'timeout_ms':30000},'include_proposed':True}
        result=tool(client,'extract',query);write('matrix.json',result)
        check('actual public matrix keeps both project coordinates and missing exit facet',len(result['groups'])==2 and result['state']=='partial'and all(g['cells'][0]['state']=='similar'and g['cells'][1]['state']=='unknown'for g in result['groups']))
        for group,f in zip(result['groups'],fixtures,strict=True):
            cell=group['cells'][0];bodies=[e for m in cell['mappings']for e in m['binding']['evidence']];check('actual source and citations remain in selected project '+f['label'],group['participant']['scope']['coordinates']==f['coordinates']and any(f['method']in e['body']and str(f['value'])in e['body']for e in bodies)and all(e['reference']['citation']['project_id']==f['coordinates']['project_id']for e in bodies)and all(m['binding']['review_state']=='proposed'for m in cell['mappings']))
        check('cross-project candidates retain both evidence and reasons',len(result['candidates'])>0 and all(len(c['participants'])==2 and len(c['evidence'])==2 and all(e['citations']for e in c['evidence'])and len(c['reasons'])==2 for c in result['candidates']))
        default=copy.deepcopy(query);default['include_proposed']=False;hidden=tool(client,'extract',default);write('default-reviewed-only.json',hidden);check('default does not invent human approval for candidate mappings',all(c['state']=='unknown'and not c['mappings']for g in hidden['groups']for c in g['cells']))
        wrong=copy.deepcopy(query);wrong['participants'][1]='f'*64;tool(client,'extract',wrong,error='request_invalid');wrong=copy.deepcopy(query);wrong['profile_digest']=initial['profile_digest'];tool(client,'extract',wrong,error='stale_configuration');wrong=copy.deepcopy(query);wrong['mode']='implementation';tool(client,'extract',wrong,error='request_invalid');tool(client,'describe',{'config_path':'/private/unregistered'},error='request_invalid');check('foreign scope stale pins implementation mode and config rewrite are refused',True)
        small=copy.deepcopy(query);small['budget']['max_body_bytes']=1;limited=tool(client,'extract',small);write('budget.json',limited);check('budget omits complete facets without truncated source',limited['used_body_bytes']==0 and limited['used_citations']==0 and all(g['cells'][0]['state']=='budget_exceeded'and not g['cells'][0]['mappings']for g in limited['groups']))
    finally:client.close();client.proc.stdout.close()
    check('both native MCP sessions close with EOF0',all(c.proc.returncode==0 for c in sessions))
    write('fixtures.json',fixtures);write('summary.json',{'schema_version':1,'checks':len(checks),'actual_tool_calls':len(calls),'actual_MCP_sessions':2,'EOF_exit_codes':[c.proc.returncode for c in sessions],'actual_native_dataset_builds':2,'paid_model_calls':0,'embedding':'mock','semantic_quality_or_independent_acceptance':False,'full_KM06_complete':False,'scope':'KM06_2_actual_native_source_matrix_not_actual_real_project_comparison'})

if __name__=='__main__':main()
