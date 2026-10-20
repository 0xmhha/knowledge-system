#!/usr/bin/env python3
"""Verify all signed study slots; score finite function retrieval, not general answer quality."""
import argparse
import collections
import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import statistics
import yaml
ROOT=Path(__file__).resolve().parents[1]
ARMS=['off','concept_text','relations','combined']


def require(v,message):
    if not v:raise ValueError(message)
def sha(data):return hashlib.sha256(data).hexdigest()
def canonical(v):return json.dumps(v,ensure_ascii=False,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
def percentile(values,q):
    values=sorted(values);index=(len(values)-1)*q;low=int(index);return values[low]+(values[min(low+1,len(values)-1)]-values[low])*(index-low)
def read(folder,name):
    p=folder/name;return p.read_bytes()if p.exists()else gzip.decompress((folder/(name+'.gz')).read_bytes())
def load(folder,name):return json.loads(read(folder,name))
def write(path,value):path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')


def verify(folder):
    protocol=load(folder,'preregistered/protocol.json');pins=load(folder,'preregistered/source-pins.json');public=load(folder,'preregistered/requests.json');labels=load(folder,'preregistered/labels.json');oracle=load(folder,'preregistered/private-function-criteria.json');runtime=load(folder,'runtime-protocol.json');binding=load(folder,'input-binding.json')
    require(runtime['preregistered_revision']==binding['preregistered_revision']and binding['signed_preregistration_verified']is True and runtime['quality_acceptance']is False and runtime['LLM_answer_calls']==runtime['paid_model_calls']==0 and runtime['mode_defaults_changed']is False,'runtime scope promoted')
    require(protocol['arms']==runtime['arms']==ARMS and protocol['planned_capture_calls']==runtime['planned_capture_calls']==512 and protocol['question_count']==16 and protocol['fact_families']==5 and oracle['human_approval']is False and oracle['independent_quality_gold']is False,'design/denominator differs')
    for name,key in(('requests.json','requests_sha256'),('labels.json','labels_sha256'),('private-function-criteria.json','criteria_sha256')):require(sha(read(folder,'preregistered/'+name))==protocol[key],'signed input changed')
    require(sha(read(folder,'semantic.sqlite'))==runtime['semantic_sha256'],'semantic store differs')
    projected=load(folder,'projection.json');require(len(projected['concepts'])==4 and not projected.get('assertions'),'semantic code links changed')
    require(load(folder,'model-before.json')==load(folder,'model-after.json')==runtime['model']and runtime['model']['digest']==protocol['model']['digest'],'model/server drift')
    normalized=[]
    for arm in ARMS:
        cfg=yaml.safe_load(read(folder,arm+'.yaml'));require(cfg['backends']['ckv']['provider']=='ollama','wrong provider');cfg['semantic'].pop('ontology_mode',None);normalized.append(cfg)
    require(all(c==normalized[0]for c in normalized),'non-mode settings differ')
    ready=load(folder,'readiness.json');require(set(ready)==set(ARMS)and all(v['availability']=='ready'and v['read_tools_ready']is True and v['mode']==arm and v['counts']['verified_concepts']==4 and v['counts']['verified_implementations']==0 for arm,v in ready.items()),'readiness differs')
    source_manifest=load(folder,'source-manifest.json')if(folder/'source-manifest.json').exists()else json.loads((Path(pins['version_dir'])/'sources/manifest.json').read_text());files={(r['origin_id'],r['path']):r for r in source_manifest['files']};sources={};used=set()
    def source(origin,path,digest):
        record=files[(origin,path)];require(digest is None or digest==record['sha256'],'source absent or foreign hash');digest=record['sha256'];used.add(digest)
        if digest not in sources:
            copied=folder/'source-blobs'/digest;data=copied.read_bytes()if copied.exists()else(Path(pins['version_dir'])/'sources/blobs'/digest).read_bytes();require(sha(data)==digest,'source blob changed');sources[digest]=data
        return sources[digest]
    for unit in oracle['units']:
        data=source('repo',unit['file'],unit['file_sha256']);span=b''.join(data.splitlines(keepends=True)[unit['start_line']-1:unit['end_line']]);require(sha(span)==unit['span_sha256']and span.decode()==unit['expected_code_body'],'function criterion changed')
    spec=importlib.util.spec_from_file_location('old_ontology_source_verifier',ROOT/'scripts/score-ontology-effect-dev.py');old=importlib.util.module_from_spec(spec);spec.loader.exec_module(old)
    questions={q['id']:q for q in public['requests']};slots={};inventory=load(folder,'capture-inventory.json');require(len(inventory)==64,'capture inventory incomplete');seen=set();citations=bodies=0;rows_by_arm=collections.defaultdict(list)
    for entry in inventory:
        qid,arm=entry['question'],entry['arm'];require((qid,arm)not in seen and qid in questions and arm in ARMS,'duplicate/foreign capture');seen.add((qid,arm));q=questions[qid];report=load(folder,entry['file']);require(report['state']=='captured'and report['counts']==protocol['counts']and len(report['rows'])==entry['rows']==8 and report['quality_metrics']is None,'capture incomplete/quality forged')
        require(report['request_sha256']==sha(read(folder,qid+'-request.json'))and load(folder,qid+'-request.json')['requests']==[q]and report['config_sha256']==report['config_sha256_after']==sha(read(folder,arm+'.yaml'))and report['binary_sha256']==report['binary_sha256_after']==pins['binary_sha256'],'call input drift')
        require(len(report['session_closures'])==1 and not report['session_closures'][0].get('error'),'session close failed')
        for row in report['rows']:
            call=row['call'];key=(qid,arm,row['phase'],row['iteration']);require(key not in slots and row['request_id']==qid and row['phase']in protocol['counts']and 1<=row['iteration']<=protocol['counts'][row['phase']],'duplicate/unplanned slot');require(not row.get('error')and not call.get('transport_error')and not call['response'].get('isError')and call['tool']==q['tool']and call['arguments']==q['arguments']and type(call['elapsed_ns'])is int and call['elapsed_ns']>=0,'failed or changed call');pack=call['response']['structuredContent'];ontology=old.verify_pack(pack,q['arguments']['prompt'],arm,source);citations+=len(pack['citations']);bodies+=len(pack['bodies']);slots[key]=(row,pack,ontology);rows_by_arm[arm].append((qid,row,pack,ontology))
    expected={(q['id'],arm,phase,i)for q in public['requests']for arm in ARMS for phase,count in protocol['counts'].items()for i in range(1,count+1)};require(set(slots)==expected and len(slots)==512,'missing/unplanned call slots')
    api=load(folder,'api-meter.json');stream=[json.loads(line)for line in read(folder,'api-meter.jsonl').splitlines()];require(api==stream,'meter stream/snapshot differs');closure=load(folder,'meter-closure.json');require(closure['listener_closed']is True and closure['active_requests']==0 and closure['started_requests']==closure['recorded_requests']==len(api),'meter did not drain')
    require(all(row['status']==200 and row['error']is None and row['path'].split('?')[0]in('/api/tags','/api/version','/api/show','/api/embed','/api/embeddings')for row in api),'meter/API errors or unexpected route')
    preservation=load(folder,'preservation.json');require(preservation['dataset_files_unchanged']is True and preservation['semantic_unchanged']is True and preservation['binary_unchanged']is True and preservation['source_paths']==pins['preserved_files'],'source preservation failed')
    verification={'actual_capture_calls':512,'question_count':16,'fact_families':5,'scored_questions':14,'function_units':11,'readiness_calls':4,'capture_sessions_closed':64,'readiness_sessions_EOF0':4,'missing_slots':0,'failed_calls':0,'citations_verified':citations,'bodies_verified':bodies,'source_blobs_verified':len(used),'meter_requests':len(api),'meter_active_at_exit':0,'source_preserved':True,'semantic_quality_acceptance':False,'independent_acceptance':False,'LLM_answer_calls':0,'paid_model_calls':0}
    return verification,protocol,pins,labels,oracle,slots,api,used,sources


def score(folder):
    verification,protocol,pins,labels,oracle,slots,api,used,sources=verify(folder);units={u['id']:u for u in oracle['units']};labels={l['id']:l for l in labels};per_question=[]
    for qid,label in labels.items():
        for arm in ARMS:
            observations=[]
            for iteration in(1,2):
                row,pack,ontology=slots[(qid,arm,'retrieval',iteration)];items=[];seen=set()
                for c in pack['citations']:
                    key=canonical(c)
                    if key not in seen:items.append(c);seen.add(key)
                    if len(items)==10:break
                found=set();relevant=[]
                for rank,c in enumerate(items,1):
                    body=next((b['text']for b in pack['bodies']if b['citation']==c),None);hit={uid for uid in label['expected_unit_ids']if c['origin_id']=='repo'and c['file']==units[uid]['file']and c['file_sha256']==units[uid]['file_sha256']and c['start_line']<=units[uid]['start_line']and c['end_line']>=units[uid]['end_line']and body is not None and units[uid]['expected_code_body']in body}
                    if hit:relevant.append(rank);found.update(hit)
                observations.append({'iteration':iteration,'precision_at10':len(relevant)/len(items)if items else 0,'recall_at10':len(found)/len(label['expected_unit_ids'])if label['retrieval_scored']else None,'hit_at10':float(bool(relevant))if label['retrieval_scored']else None,'reciprocal_rank_at10':1/min(relevant)if relevant else 0,'found_unit_ids':sorted(found),'intended_concept_match':set(label['intended_ontology_concepts'])<={m['concept_id']for m in ontology.get('explanation',{}).get('matches',[])}if label['intended_ontology_concepts']else None,'out_of_intent_concepts':sorted({m['concept_id']for m in ontology.get('explanation',{}).get('matches',[])}-set(label['intended_ontology_concepts'])),'ontology':ontology})
            record={'id':qid,'arm':arm,'family':label['family'],'kind':label['kind'],'retrieval_scored':label['retrieval_scored'],'expected_unit_ids':label['expected_unit_ids'],'intended_ontology_concepts':label['intended_ontology_concepts'],'observations':observations}
            for key in('precision_at10','recall_at10','hit_at10','reciprocal_rank_at10'):record[key]=statistics.mean(o[key]for o in observations)if label['retrieval_scored']else None
            per_question.append(record)
    summary={'verification':verification,'metric_scope':'source-defined finite complete-function retrieval; not general behavior/policy or answer relevance','human_or_independent_gold':False,'query_variants_are_not_independent_facts':True,'arms':{},'quality_acceptance':False,'independent_acceptance':False,'default_recommendation':'keep_off_pending_reviewed_links_and_broader_independent_evidence','LLM_answer_quality':None}
    for arm in ARMS:
        values=[v for v in per_question if v['arm']==arm and v['retrieval_scored']];families={family:{key:statistics.mean(v[key]for v in values if v['family']==family)for key in('precision_at10','recall_at10','hit_at10','reciprocal_rank_at10')}for family in oracle['families']};warm=[slots[(qid,arm,'warm_latency',i)][0]['call']['elapsed_ns']/1e6 for qid in labels for i in range(1,6)];all_ontology=[slots[key][2]for key in slots if key[1]==arm];batches=[r for r in api if r['batch'].endswith('-'+arm)and r['batch'].startswith('EX-')];embedding=[r for r in batches if r['path'].split('?')[0]in('/api/embed','/api/embeddings')];known=[r['server_usage']['prompt_eval_count']for r in embedding if 'prompt_eval_count'in r['server_usage']]
        summary['arms'][arm]={'question_macro':{key:statistics.mean(v[key]for v in values)for key in('precision_at10','recall_at10','hit_at10','reciprocal_rank_at10')},'family_macro':{key:statistics.mean(v[key]for v in families.values())for key in('precision_at10','recall_at10','hit_at10','reciprocal_rank_at10')},'families':families,'warm_observations':len(warm),'warm_p50_ms':statistics.median(warm),'warm_p95_ms':percentile(warm,.95),'ontology_states':dict(collections.Counter(v['state']for v in all_ontology)),'API_requests_in_capture_batches':len(batches),'embedding_API_requests_in_capture_batches':len(embedding),'server_reported_prompt_tokens':sum(known)if len(known)==len(embedding)else None,'embedding_responses_missing_token_usage':len(embedding)-len(known),'cloud_model_API_calls':0,'local_electricity_hardware_cost':'not_measured','actual_text_search_calls_all_phases':sum(o.get('text_search_calls',0)for o in all_ontology),'boosted_citations_all_phases':sum(o.get('boosted_citations',0)for o in all_ontology),'applied_relations_all_phases':sum(o.get('applied_relations',0)for o in all_ontology)}
    for arm in ARMS:
        comparisons=[];intended=[];scope_mismatch=[]
        for qid,label in labels.items():
            for i in(1,2):
                row,pack,o=slots[(qid,arm,'retrieval',i)];base=slots[(qid,'off','retrieval',i)][1];comparisons.append([canonical(c)for c in pack['citations']]!=[canonical(c)for c in base['citations']]);matched={v['concept_id']for v in o.get('explanation',{}).get('matches',[])}
                if label['intended_ontology_concepts']:intended.append(set(label['intended_ontology_concepts'])<=matched)
                if not label['intended_ontology_concepts']:scope_mismatch.append(bool(matched))
        summary['arms'][arm].update(final_citation_order_changes_vs_off=sum(comparisons),paired_retrieval_comparisons=len(comparisons),intended_concept_match_observations=sum(intended),intended_concept_observations=len(intended),out_of_intent_control_match_observations=sum(scope_mismatch),out_of_intent_control_observations=len(scope_mismatch),ontology_scope_metric='declared intent diagnostic, not human-adjudicated semantic false-positive rate')
    return summary,per_question,verification,used,sources


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--folder',type=Path,required=True);parser.add_argument('--verify-only',action='store_true');args=parser.parse_args();folder=args.folder.resolve()
    if args.verify_only:
        v,*_=verify(folder);write(folder/'verification.json',v);print(json.dumps(v));return
    summary,questions,v,used,sources=score(folder);write(folder/'verification.json',v);write(folder/'function-retrieval-scores.json',questions);write(folder/'effect-summary.json',summary);print(json.dumps(summary,ensure_ascii=False))

if __name__=='__main__':main()
