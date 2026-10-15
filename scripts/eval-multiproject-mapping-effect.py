#!/usr/bin/env python3
"""Explicit no-mapping baseline versus proposed mappings on the same real datasets."""
import argparse
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import sys
from datetime import datetime,timezone
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    source=ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/runtime';profile=json.loads((source/'profile.json').read_text());queries=json.loads((source/'queries.json').read_text());fixtures=json.loads((source/'fixtures.json').read_text())
    def sha(data):return hashlib.sha256(data).hexdigest()
    def write(name,v):(out/name).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    # Keep every profile value except the selected binding input identical.
    baseline=copy.deepcopy(profile);baseline.pop('bindings_path');write('without-mapping.json',baseline);write('with-mapping.json',profile)
    binding=Path(profile['bindings_path']);manager.require(binding.read_bytes()==(source/'bindings.json').read_bytes(),'frozen_mapping_bytes_changed')
    paths=[Path(profile['catalog_path']),Path(profile['sanitize_rules_path']),binding]
    for f in fixtures:
        version=Path(f['version']);paths.extend(p for p in(version/'sources').rglob('*')if p.is_file());paths.extend(version/name for name in('graph/graph.db','graph/manifest.json','vector/vector.db','vector/manifest.json','dataset-identity.json','reader-protocol.json'))
    before={str(p):sha(p.read_bytes())for p in paths};write('before.json',before)
    protocol={'schema_version':1,'stage':'registered_before_calls','baseline':'same concept scheme/source/catalog/scopes without project binding input','treatment':'same source with frozen7 proposed project mappings; include_proposed true in both arms','concepts':list(queries),'project_count':2,'unique_project_facet_cells_per_arm':40,'repetitions':2,'planned_matrix_calls':24,'embedding':'mock datasets; source-only lookup, no new embedding calls','independent_quality_gold':False,'human_mapping_approval':False,'default_mode_changed':False,'expected_source_sets':'input-curated mapping coverage only; observed binding absence is not feature absence','limitations':['hand-curated source mappings add prior annotation; not causal benefit of ontology vocabulary alone','same requested scopes/typed identity protection in both arms; zero project error cannot be attributed solely to mappings','in-sample source/function associations, not independent holdout or general semantic/answer quality','no natural-language concept selector or real LLM analyst is evaluated','same40cells repeated twice remain40unique units, not80 independent facts'],'profile_without_binding_sha256':sha((out/'without-mapping.json').read_bytes()),'profile_with_binding_sha256':sha((out/'with-mapping.json').read_bytes()),'frozen_binding_sha256':sha(binding.read_bytes()),'queries_sha256':sha((source/'queries.json').read_bytes()),'registered_at':datetime.now(timezone.utc).isoformat(timespec='seconds')};write('protocol.json',protocol)
    spec=importlib.util.spec_from_file_location('mapping_effect_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire);clients={};descriptions={};calls=[];matrix={};checks=[]
    def check(label,value):manager.require(value,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    def tool(client,operation,args):
        response=client.request('tools/call',{'name':'cks.comparison.'+operation,'arguments':args})['result'];calls.append({'session':client.root.name,'tool':'cks.comparison.'+operation,'arguments':copy.deepcopy(args),'response':copy.deepcopy(response)});write('calls.json',calls);manager.require(not response.get('isError'),'actual MCP refusal');return response['structuredContent']
    try:
        for arm in('without-mapping','with-mapping'):
            directory=out/arm;directory.mkdir();client=wire.Client(None,None,directory,command=[str(ROOT/'bin/cks'),'mcp','comparison','--profile',str(out/(arm+'.json'))],namespace='');clients[arm]=client;client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'mapping-effect-development','version':'1'}});descriptions[arm]=tool(client,'describe',{})
        write('descriptions.json',descriptions);check('only binding selection differs between arm profiles',{k:v for k,v in profile.items()if k!='bindings_path'}==baseline and descriptions['without-mapping']['catalog_digest']==descriptions['with-mapping']['catalog_digest']and descriptions['without-mapping']['scopes']==descriptions['with-mapping']['scopes'])
        for index,(concept,q)in enumerate(queries.items()):
            for repetition in(1,2):
                arms=['without-mapping','with-mapping']if(index+repetition)%2 else['with-mapping','without-mapping']
                for arm in arms:
                    request=copy.deepcopy(q);request.update(profile_digest=descriptions[arm]['profile_digest'],catalog_digest=descriptions[arm]['catalog_digest']);value=tool(clients[arm],'extract',request);matrix[(arm,concept,repetition)]=value;write(arm+'-'+concept+'-'+str(repetition)+'.json',value)
            check('both arms retain explicit scopes and axes '+concept,all(len(matrix[(arm,concept,1)]['groups'])==2 and all(group['participant']['scope']['coordinates']==f['coordinates']and group['participant']['scope_id']==sid and [cell['facet']for cell in group['cells']]==q['facets']for group,f,sid in zip(matrix[(arm,concept,1)]['groups'],fixtures,q['participants'],strict=True))for arm in clients))
            check('repeat stable without new independent facts '+concept,all(matrix[(arm,concept,1)]==matrix[(arm,concept,2)]for arm in clients))
        default=copy.deepcopy(queries['consensus']);default.update(profile_digest=descriptions['with-mapping']['profile_digest'],catalog_digest=descriptions['with-mapping']['catalog_digest'],include_proposed=False);hidden=tool(clients['with-mapping'],'extract',default);write('default-reviewed-only.json',hidden);check('candidate effect does not imply supplied human mapping approval',all(not c['mappings']for g in hidden['groups']for c in g['cells']))
    finally:
        for client in clients.values():client.close();client.proc.stdout.close()
    result={}
    for arm in clients:
        cells=[c for concept in queries for group in matrix[(arm,concept,1)]['groups']for c in group['cells']];source_items=[];wrong=0
        for concept in queries:
            value=matrix[(arm,concept,1)]
            for group in value['groups']:
                coords=group['participant']['scope']['coordinates']
                for cell in group['cells']:
                    for mapping in cell['mappings']:
                        manager.require(mapping['binding']['review_state']=='proposed'and mapping['review']['reviewer_identity_authenticated']is False,'human_review_invented')
                        for item in mapping['binding']['evidence']:
                            c=item['reference']['citation'];wrong+=not all(c[k]==v for k,v in coords.items());data=(Path(next(f['version']for f in fixtures if f['coordinates']==coords))/'sources/blobs'/c['file_sha256']).read_bytes();span=b''.join(data.splitlines(keepends=True)[c['start_line']-1:c['end_line']]);manager.require(sha(data)==c['file_sha256']and sha(span)==c['content_sha256']and span.decode()==item['body'],'source_or_citation_mismatch');source_items.append(c)
        source_cells=sum(bool(c['mappings'])for c in cells);known=sum(c['state']in('exact','similar','not_applicable')for c in cells);candidates=[c for concept in queries for c in matrix[(arm,concept,1)]['candidates']]
        result[arm]={'unique_cells':len(cells),'cells_with_source_evidence':source_cells,'source_evidence_cell_coverage':source_cells/len(cells),'declared_exact_similar_not_applicable_cells':known,'declared_mapping_coverage':known/len(cells),'unknown_cells':sum(c['state']=='unknown'for c in cells),'not_collected_cells':sum(c['state']=='not_collected'for c in cells),'cell_source_citations':len(source_items),'source_citations_verified':len(source_items),'wrong_project_citations':wrong,'project_group_scopes_checked':12,'project_group_errors':0,'two_sided_candidates':len(candidates),'source_accuracy':1.0 if source_items and wrong==0 else None,'missing_evidence_is_not_feature_absence':True}
        manager.require(all({e['scope_id']for e in c['evidence']}==set(c['participants'])and len(c['participants'])==2 and all(e['citations']for e in c['evidence'])for c in candidates),'one_sided_candidate')
    check('baseline no invented evidence and treatment keeps unknowns',result['without-mapping']['unique_cells']==result['with-mapping']['unique_cells']==40 and result['without-mapping']['cells_with_source_evidence']==0 and result['without-mapping']['not_collected_cells']==40 and result['with-mapping']['cells_with_source_evidence']==8 and result['with-mapping']['declared_exact_similar_not_applicable_cells']==7 and result['with-mapping']['unknown_cells']==18 and result['with-mapping']['not_collected_cells']==15)
    check('same identity guard in both arms and all treatment source citations verify',all(v['wrong_project_citations']==v['project_group_errors']==0 for v in result.values())and result['with-mapping']['source_citations_verified']==9 and result['with-mapping']['two_sided_candidates']==3)
    after={str(p):sha(p.read_bytes())for p in paths};write('after.json',after);check('all datasets source archives and configured inputs remain byte identical',before==after);check('both actual native comparison sessions close EOF0',len(clients)==2 and all(c.proc.returncode==0 for c in clients.values()))
    write('summary.json',{'schema_version':1,'scope':'explicit_no_mapping_baseline_vs_frozen_proposed_annotations_not_independent_quality','arms':result,'checks':len(checks),'actual_matrix_calls':24,'actual_tool_calls':len(calls),'MCP_sessions':len(clients),'EOF_exit_codes':[c.proc.returncode for c in clients.values()],'paid_model_calls':0,'LLM_answer_calls':0,'source_corpus_changed':False,'ontology_vocabulary_alone_effect':'not_isolated_from_annotation_input','human_mapping_approval':False,'independent_quality_acceptance':False,'recommendation':'keep_verified_only_default; use_explicit_proposed_study_to_inspect_source; prioritize_missing_mappings_review_and_scope_disambiguation'})

if __name__=='__main__':main()
