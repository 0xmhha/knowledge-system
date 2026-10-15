#!/usr/bin/env python3
"""Actual pinned real-project comparison MCP; proposed mappings, no quality approval."""
import argparse
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
from datetime import datetime,timezone
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager

# Explicit excerpts selected from pinned commits. They are not full behavior or
# policy judgments, and their hashes are checked against retained archive bytes.
ANCHORS={
 'go-stablenet':{
  'consensus': [('consensus/wbft/validator/default.go',97,111,'func (valSet *defaultSet) List', ['participants']),('consensus/wbft/validator/default.go',222,229,'func (valSet *defaultSet) QuorumSize',['quorum']),('consensus/wbft/core/commit.go',90,131,'func (c *Core) handleCommitMsg',['quorum'])],
  'finality': [('consensus/wbft/core/commit.go',122,138,'c.commitWBFT()',['condition'])],
  'mempool-admission':[('core/txpool/validation.go',55,174,'func ValidateTransaction',['admission'])],
  'transaction-ordering':[('miner/ordering.go',69,100,'func (s *txByPriceAndTime) Less',['ordering'])],
 },
 'go-wemix':{
  'consensus':[('consensus/clique/clique.go',462,495,'func (c *Clique) verifySeal',['participants'])],
  'mempool-admission':[('core/tx_pool.go',645,739,'func (pool *TxPool) validateTx',['admission'])],
  'transaction-ordering':[('core/types/transaction.go',572,580,'func (s TxByPriceAndTime) Less',['ordering'])],
 },
}
CONCEPTS=['staking','staking-policy','consensus','finality','mempool-admission','transaction-ordering']


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--prepared',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();prepared=args.prepared.resolve();out=args.out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir();fixtures=json.loads((prepared/'fixtures.json').read_text());manager.require(len(fixtures)==2 and json.loads((prepared/'summary.json').read_text())['actual_native_dataset_builds']==2,'actual_builds_required')
    def sha(data):return hashlib.sha256(data).hexdigest()
    def write(name,v):(out/name).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    checks,calls,sessions=[],[],[]
    def check(label,v):manager.require(v,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    def inventory():
        result={}
        for f in fixtures:
            version=Path(f['version']);manager.require(version.is_relative_to(prepared),'owned_dataset_required')
            for p in list((version/'sources').rglob('*'))+[version/n for n in('dataset-identity.json','reader-protocol.json','graph/graph.db','graph/manifest.json','vector/vector.db','vector/manifest.json')]:
                if p.is_file():result[p.relative_to(prepared).as_posix()]=sha(p.read_bytes())
            result[(version.parent/'current').relative_to(prepared).as_posix()+'#target']=os.readlink(version.parent/'current')
        return result
    before=inventory();write('before.json',before)
    catalog={'schema_version':1,'projects':[{'id':f['coordinates']['project_id'],'name':f['project']}for f in fixtures],'versions':[{'coordinates':f['coordinates'],'version_dir':f['version'],'origins':f['origins']}for f in fixtures],'collections':[{'id':'real-chains','members':[f['coordinates']for f in fixtures]}]};write('catalog.json',catalog)
    observed=datetime.now(timezone.utc).isoformat(timespec='seconds');scopes=[]
    for f in fixtures:
        f['conditions']={}
        for role in('l1','staking-network'):
            conditions={'network':'not-selected-source-study','configuration':('wbft'if f['project']=='go-stablenet'else'clique')+'-source-excerpts-no-live-network','component_role':role,'observed_at':observed};f['conditions'][role]=conditions;scopes.append({'coordinates':f['coordinates'],'industry':'blockchain','conditions':conditions})
    profile={'schema_version':1,'name':'pinned-real-project-comparison','catalog_path':str(out/'catalog.json'),'scheme_root':str(ROOT/'system/packs/industry-blockchain'),'sanitize_rules_path':str(ROOT/'plugins/knowledge-manager/config/sanitization_rules.yaml'),'scopes':scopes};write('profile-bootstrap.json',profile)
    spec=importlib.util.spec_from_file_location('real_comparison_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)
    def connect(label,path):
        d=out/label;d.mkdir();c=wire.Client(None,None,d,command=[str(ROOT/'bin/cks'),'mcp','comparison','--profile',str(path)],namespace='');sessions.append(c);c.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'pinned-real-source-study','version':'1'}});return c
    def tool(c,operation,args=None,error=None):
        response=c.request('tools/call',{'name':'cks.comparison.'+operation,'arguments':args or{}})['result'];calls.append({'session':c.root.name,'tool':'cks.comparison.'+operation,'arguments':copy.deepcopy(args or{}),'response':copy.deepcopy(response),'expected_error':error});write('calls.json',calls)
        if error:manager.require(response.get('isError')and response['structuredContent']['code']==error,'wrong refusal');return response
        manager.require(not response.get('isError'),'actual comparison refusal '+str(response));return response['structuredContent']
    first=connect('bootstrap-session',out/'profile-bootstrap.json')
    try:d=tool(first,'describe');write('initial-description.json',d)
    finally:first.close();first.proc.stdout.close()
    definitions={c['key']['concept_id']:c for c in d['concepts']};bindings=[];origin_equal={}
    for f in fixtures:
        version=Path(f['version']);manifest=json.loads((version/'sources/manifest.json').read_text());manager.require(f['coordinates']['source_mode']in('committed','working-tree')and f['coordinates']['base_commit']==f['source_commit'],'actual_source_commit_lost')
        for concept,anchors in ANCHORS[f['project']].items():
            refs=[]
            for i,(filename,start,end,marker,facets)in enumerate(anchors):
                record=next(r for r in manifest['files']if r['origin_id']=='repo'and r['path']==filename);source=(version/'sources/blobs'/record['sha256']).read_bytes();original_source=subprocess.check_output(['git','-C',f['original_root'],'show',f['source_commit']+':'+filename]);manager.require(source==original_source,'compared_code_differs_from_original_commit');origin_equal[f['project']+':'+filename]={'source_commit':f['source_commit'],'source_mode':f['coordinates']['source_mode'],'original_sha256':sha(original_source),'retained_sha256':sha(source),'equal':True};write('source-equality.json',origin_equal);body=b''.join(source.splitlines(keepends=True)[start-1:end]);manager.require(sha(source)==record['sha256']and marker in body.decode()and end-start<256,'pinned_source_anchor_mismatch');citation={**f['coordinates'],'file':filename,'start_line':start,'end_line':end,'commit_hash':f['source_commit']if f['coordinates']['source_mode']=='committed'else'','origin_id':'repo','file_sha256':record['sha256'],'content_sha256':sha(body)};refs.append({'id':'source-'+str(i+1),'kind':'code','facets':facets,'citation':citation})
            reason='Proposed mapping of pinned '+f['project']+' '+concept+' source excerpts for an unexecuted study; no human review, live network policy, superiority, TPS or security proof. Missing facets remain unknown.'
            b={'id':concept,'concept':definitions[concept]['key'],'coordinates':f['coordinates'],'conditions':f['conditions']['l1'],'local_concept':f['project']+' '+concept+' source excerpts','mapping':'close','reason':reason,'evidence':refs}
            if concept=='finality':b.update(mapping='related',applicability='unknown',reason='WBFT quorum dispatch and committed-state excerpt only. It does not establish irreversibility, rollback or L2 settlement; applicability remains unknown and the mapping is not human reviewed.')
            bindings.append(b)
    write('bindings.json',{'schema_version':2,'bindings':bindings});profile['bindings_path']=str(out/'bindings.json');write('profile.json',profile);write('fixtures.json',fixtures)
    c=connect('comparison-session',out/'profile.json');matrices={};queries={}
    try:
        tools=c.request('tools/list')['result']['tools'];write('tools.json',tools);check('only two explicitly readonly native comparison tools',len(tools)==2 and all(t['annotations']=={'readOnlyHint':True,'destructiveHint':False,'idempotentHint':True,'openWorldHint':False}for t in tools))
        d=tool(c,'describe');write('description.json',d);check('four explicit project role scopes and configured versus readiness distinction',len(d['scopes'])==4 and d['state']=='configured'and d['source_readiness']=='not_checked'and d['write_enabled']is False)
        for concept in CONCEPTS:
            role='staking-network'if concept.startswith('staking')else'l1';ids=[next(s['scope_id']for s in d['scopes']if s['scope']['coordinates']==f['coordinates']and s['scope']['conditions']['component_role']==role)for f in fixtures];q={'schema_version':1,'mode':'comparison','profile_digest':d['profile_digest'],'catalog_digest':d['catalog_digest'],'collection_id':'real-chains','participants':ids,'concept':definitions[concept]['key'],'facets':[f['id']for f in definitions[concept]['view']['facets']],'question':'고정 원문으로 '+concept+' 근거와 모르는 부분을 프로젝트별로 비교한다.','as_of':datetime.now(timezone.utc).isoformat(timespec='seconds'),'budget':{'max_body_bytes':262144,'max_citations':128,'timeout_ms':30000},'include_proposed':True};queries[concept]=q;matrix=tool(c,'extract',q);matrices[concept]=matrix;write('matrix-'+concept+'.json',matrix)
            manager.require(len(matrix['groups'])==2 and all(g['participant']['scope']['coordinates']==f['coordinates']and g['participant']['scope_id']==scope_id for g,f,scope_id in zip(matrix['groups'],fixtures,ids,strict=True)),'project_selection_mixed')
            for group,f in zip(matrix['groups'],fixtures,strict=True):
                for cell in group['cells']:
                    for mapping in cell['mappings']:
                        manager.require(mapping['binding']['review_state']=='proposed'and mapping['review']['reviewer_identity_authenticated']is False,'invented_human_mapping_review')
                        for e in mapping['binding']['evidence']:
                            cite=e['reference']['citation'];manager.require(all(cite[k]==v for k,v in f['coordinates'].items())and cite['commit_hash']==(f['source_commit']if f['coordinates']['source_mode']=='committed'else''),'foreign_project_or_commit');source=(Path(f['version'])/'sources/blobs'/cite['file_sha256']).read_bytes();body=b''.join(source.splitlines(keepends=True)[cite['start_line']-1:cite['end_line']]);manager.require(sha(source)==cite['file_sha256']and sha(body)==cite['content_sha256']and e['body']==body.decode(),'retained_body_or_hash_mismatch')
            check('actual pinned source matrix and project citation separation '+concept,True)
        check('staking and staking policy absence of collected mapping is not feature absence',all(cell['state']=='not_collected'and not cell['mappings']for key in('staking','staking-policy')for g in matrices[key]['groups']for cell in g['cells']))
        check('finality is not inferred from commit transition or Finalize name',matrices['finality']['groups'][0]['cells'][0]['state']=='unknown'and len(matrices['finality']['groups'][0]['cells'][0]['mappings'])==1 and all(cell['state']=='not_collected'for cell in matrices['finality']['groups'][1]['cells']))
        check('uncollected fairness fault assumptions and other axes remain visible',all(g['cells'][-1]['state']=='unknown'for key in('consensus','transaction-ordering','mempool-admission')for g in matrices[key]['groups']))
        check('common difference candidates carry evidence on both explicit sides',all(matrices[key]['candidates']and all(len(row['participants'])==2 and {e['scope_id']for e in row['evidence']}==set(row['participants'])and all(e['citations']for e in row['evidence'])for row in matrices[key]['candidates'])for key in('consensus','mempool-admission','transaction-ordering')))
        default=copy.deepcopy(queries['consensus']);default['include_proposed']=False;hidden=tool(c,'extract',default);write('default-reviewed-only.json',hidden);check('actual real project mappings are not silently promoted to human reviewed',all(not cell['mappings']and cell['state']=='unknown'for g in hidden['groups']for cell in g['cells']))
        live=Path(fixtures[0]['owned_clone'])/'core/txpool/validation.go';manager.require(live.is_relative_to(prepared),'owned_clone_required');data=live.read_bytes();mode=stat.S_IMODE(live.stat().st_mode)
        try:
            live.unlink();replay=tool(c,'extract',queries['mempool-admission']);write('after-live-delete.json',replay);check('retained comparison is unchanged after owned live source deletion',replay==matrices['mempool-admission'])
        finally:live.write_bytes(data);live.chmod(mode)
        manifest=Path(fixtures[1]['version'])/'sources/manifest.json';data=manifest.read_bytes();mode=stat.S_IMODE(manifest.stat().st_mode)
        try:
            manifest.unlink();partial=tool(c,'extract',queries['mempool-admission']);write('missing-B.json',partial);check('real B archive failure retains exact A group and no two sided candidate',partial['groups'][0]==matrices['mempool-admission']['groups'][0]and partial['groups'][1]['participant']==matrices['mempool-admission']['groups'][1]['participant']and all(cell['state']=='unavailable'and not cell['mappings']for cell in partial['groups'][1]['cells'])and not partial['candidates'])
        finally:manifest.write_bytes(data);manifest.chmod(mode)
        small=copy.deepcopy(queries['transaction-ordering']);small['budget']['max_citations']=2;limited=tool(c,'extract',small);write('candidate-budget.json',limited);check('real comparison counts candidate proof citations and omits complete candidate',limited['used_citations']==2 and limited['omitted_candidates']==1 and limited['candidate_state']=='budget_exceeded'and not limited['candidates'])
        small['budget']['max_citations']=128;small['budget']['max_body_bytes']=1;limited=tool(c,'extract',small);write('body-budget.json',limited);check('real source excerpts never silently truncate to fit budget',limited['used_body_bytes']==limited['used_citations']==0 and all(g['cells'][1]['state']=='budget_exceeded'and not g['cells'][1]['mappings']for g in limited['groups']))
        for field,v in(('participants',[queries['consensus']['participants'][0],'f'*64]),('mode','implementation'),('target',fixtures[1]['coordinates']),('facets',['not-defined'])):
            invalid=copy.deepcopy(queries['consensus']);invalid[field]=v;tool(c,'extract',invalid,error='request_invalid')
        check('foreign scope implementation target injection and undefined axes refuse without fallback',True)
    finally:c.close();c.proc.stdout.close()
    bad=copy.deepcopy(bindings);bad[0]['evidence'][0]['citation']['file_sha256']='0'*64;write('bad-bindings.json',{'schema_version':2,'bindings':bad});badprofile=copy.deepcopy(profile);badprofile['bindings_path']=str(out/'bad-bindings.json');write('bad-profile.json',badprofile);c=connect('bad-mapping-session',out/'bad-profile.json')
    try:
        d=tool(c,'describe');q=copy.deepcopy(queries['consensus']);q.update(profile_digest=d['profile_digest'],catalog_digest=d['catalog_digest']);refused=tool(c,'extract',q);write('bad-mapping.json',refused);check('syntactically valid mispinned mapping never exposes unrelated source',refused['groups'][0]['cells'][0]['state']=='unavailable'and not refused['groups'][0]['cells'][0]['mappings']and refused['groups'][1]==matrices['consensus']['groups'][1]and not refused['candidates'])
    finally:c.close();c.proc.stdout.close()
    after=inventory();write('after.json',after);check('actual DBs and complete retained archives unchanged after comparisons and controlled recovery',before==after)
    originals=[]
    for f in fixtures:
        repo=Path(f['original_root']);head=subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD']).decode().strip();status=subprocess.check_output(['git','-C',str(repo),'status','--porcelain=v1']).decode().strip();manager.require(head==f['source_commit']and not status,'original_project_changed');manager.require(subprocess.check_output(['git','-C',f['owned_clone'],'status','--porcelain=v1']).decode().strip()==f.get('prepared_git_status',''),'owned_clone_not_restored');originals.append({'project':f['project'],'source_root':str(repo),'head':head,'status':status})
    write('original-after.json',originals);check('both original projects stay clean and owned clones return to declared prepared scope',True);check('three actual comparison sessions close with EOF0',len(sessions)==3 and all(c.proc.returncode==0 for c in sessions))
    write('queries.json',queries);write('summary.json',{'schema_version':1,'checks':len(checks),'actual_tool_calls':len(calls),'actual_comparison_MCP_sessions':len(sessions),'EOF_exit_codes':[c.proc.returncode for c in sessions],'actual_native_project_datasets':len(fixtures),'source_commits':{f['project']:f['source_commit']for f in fixtures},'concept_queries':CONCEPTS,'paid_model_calls':0,'mapping_review':'proposed_not_human_authenticated','live_network_policy_or_behavior_measured':False,'semantic_quality_or_independent_acceptance':False,'scope':'KM06_4_actual_pinned_project_source_comparison_not_quality_or_OE_effect'})
    review=['# 실제 프로젝트 비교 자료','','고정 원문의 source excerpt 비교다. 명시한 WBFT/Clique 구현을 실행하거나 실제 네트워크의 정책·staking·finality·성능을 판정한 결과가 아니다. 모든 매핑은 미검토 후보다.','','| 공통 개념 | go-stablenet | go-wemix |','|---|---|---|']
    for key in CONCEPTS:
        states=['; '.join(cell['facet']+': '+cell['state']for cell in g['cells'])for g in matrices[key]['groups']];review.append('| '+key+' | '+' | '.join(states)+' |')
    review.extend(['','본문·인용·매핑 이유와 양쪽 비교 후보는 같은 폴더의 matrix-*.json 및 calls.json에 보존했다. not_collected/unknown은 기능 부재가 아니다. 근거 없는 우월성·보안성·TPS·L2 settlement를 만들지 않는다.']);(out/'review.md').write_text('\n'.join(review)+'\n')

if __name__=='__main__':main()
