#!/usr/bin/env python3
"""Replay diagnostic F-04/F-05 DEV source, state and project isolation captures."""
import argparse
import importlib.util
import json
from pathlib import Path

SPEC = importlib.util.spec_from_file_location('state_doc_reader', Path(__file__).with_name('b0-audit-f02-document.py'))
DOC = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DOC)
BASE, RAW = DOC.BASE, DOC.RAW
IDS = ['F-04-DEV-old', 'F-04-DEV-new', 'F-05-DEV-a', 'F-05-DEV-b']
MARKERS = dict(zip(IDS, ['DEV-old-token', 'DEV-new-token', 'DEV-project-a', 'DEV-project-b']))
MODEL = '7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def audit_case(qid, fixture, state, identity, manifest, sources, capture_raw, requests_raw, config_raw, backend_raw, source_root):
    require(qid in IDS and fixture['id'] == qid.rsplit('-', 1)[0]
            and fixture['evaluation_partition'] == 'development', 'only diagnostic F-04/F-05 DEV supported')
    prefix = ('state-' if fixture['id'] == 'F-04-DEV' else 'project-') + state['name'] + '/'
    expected = {k.removeprefix(prefix): v.encode() for k, v in fixture['sources'].items()
                if k == 'go.mod' or k.startswith(prefix)}
    require(sources == expected and state['source_sha256'] == {k: BASE.sha(v) for k,v in sources.items()},
            'frozen/materialized source bytes differ')
    for k, v in expected.items():
        original = k if k == 'go.mod' else prefix + k
        require(BASE.sha(v) == fixture['source_sha256'][original], 'fixture source hash differs')
    source = identity['source']
    require(source['project_id'] == state['project_id'] and source['source_commit'] == state['commit']
            and source['source_mode'] == 'committed', 'materialized identity differs')
    require(all(manifest[k] == v for k,v in {'project_id':source['project_id'], 'dataset_id':identity['dataset_id'],
            'snapshot_id':source['snapshot_id'], 'src_commit':source['source_commit'], 'source_mode':source['source_mode']}.items()),
            'vector/source dataset identity differs')
    require(identity['embedding_identity'] == manifest['embedding_identity_v2']
            and identity['embedding_identity']['model_digest'] == MODEL
            and identity['embedding_identity']['Dim'] == 1024, 'not the pinned BGE-M3 identity')
    cap, requests = BASE.decode(capture_raw), BASE.decode(requests_raw)
    require(cap['quality_metrics'] is None and cap['request_sha256'] == BASE.sha(requests_raw)
            and cap['config_sha256'] == cap['config_sha256_after'] == BASE.sha(config_raw)
            and cap['binary_sha256'] == cap['binary_sha256_after'] and RAW.digest(cap['binary_sha256']),
            'capture input/binary binding differs')
    require(cap['counts'] == {'retrieval':1,'warmup':0,'warm_latency':0,'cold_process':0}, 'unexpected diagnostic repetition')
    require(len(requests['requests']) == 1 and len(fixture['queries']) == 1, 'unexpected DEV query inventory')
    request = requests['requests'][0]
    require(request == {'id':qid,'tool':'cks.context.get_for_task_v2',
                        'arguments':{'prompt':fixture['queries'][0]['prompt']}}, 'query arguments differ or include gold')
    require(len(cap['rows']) <= 1, 'duplicate diagnostic call')
    scopes = {}
    for line in backend_raw.splitlines():
        event = BASE.decode(line)
        if event.get('event') == 'measurement.backend_calls' and event['summary']['tool'] == request['tool']:
            s = event['summary'];require(s['measurement_id'] not in scopes, 'duplicate measurement ID')
            scopes[s['measurement_id']] = s
    result = {'id':qid,'coordinates':{'project_id':source['project_id'],'dataset_id':identity['dataset_id'],
              'snapshot_id':source['snapshot_id'],'base_commit':source['source_commit']},
              'capture_sha256':BASE.sha(capture_raw),'backend_sha256':BASE.sha(backend_raw),
              'binary_sha256':cap['binary_sha256'],'planned_calls':1,'observed_calls':len(cap['rows']),
              'missing_calls':1-len(cap['rows']),'failed_calls':0,'quality_metrics':None}
    if not cap['rows']:
        result['state'] = 'measurement_missing';return result
    row = cap['rows'][0];call = row['call'];response = call.get('response', {})
    require(row['request_id'] == qid and row['phase'] == 'retrieval' and row['iteration'] == 1
            and call['tool'] == request['tool'] and call['arguments'] == request['arguments'], 'call slot differs')
    failed = bool(row.get('error') or call.get('transport_error') or call.get('init_error') or response.get('isError'))
    if failed:
        result.update(state='measurement_failed',failed_calls=1,error_code=response.get('structuredContent',{}).get('code'))
        return result
    require(cap['state'] == 'captured' and not cap.get('errors'), 'success row with capture errors')
    pack = response['structuredContent']
    BASE.verify_pack(pack, identity, source_root)
    require(pack['query'] == request['arguments']['prompt'] and pack['semantic'] is None, 'query/semantic projection differs')
    visible = '\n'.join(b['text'] for b in pack['bodies'])
    require(all(marker not in visible for other,marker in MARKERS.items() if other != qid), 'foreign state/project body')
    refs = pack['citations']
    targets = [('main.go',3,3)] if fixture['id']=='F-04-DEV' else [('main.go',3,3),('docs/policy.md',3,3)]
    coverage = [any(c['file']==file and c['start_line']<=start and c['end_line']>=end
                    and any(b['citation']==c and MARKERS[qid] in b['text'] for b in pack['bodies']) for c in refs) for file,start,end in targets]
    require(call['measurement_id'] in scopes, 'missing backend measurement scope')
    scope = scopes[call['measurement_id']]
    retrieval = [c for c in scope['calls'] if c['backend']=='ckv' and c['method'] in ('semantic_search','find_invariants','get_conventions')]
    # Different intent routes need not run text retrieval. Preserve exactly the recorded attempts.
    require(bool(retrieval) and any(c.get('options',{}).get('K')==10 for c in retrieval)
            and all(c.get('options',{}).get('K') in (6,10) for c in retrieval), 'unexpected recorded retrieval K')
    result.update(state='structural_capture_valid',citations=len(refs),bodies=len(pack['bodies']),
                  source_and_project_binding_valid=True,expected_source_body_coverage=coverage,
                  synthetic_policy_document_only=fixture['id']=='F-05-DEV',reviewed_policy_pack_activated=False,
                  retrieval_attempts=retrieval,backend_nonreturned=sum(c['outcome']!='returned' for c in scope['calls']))
    return result


def audit_query(query, chunks, commit, expected_fresh):
    require(query['metadata']['indexed_head_ckv']==commit and query['metadata']['fresh'] is expected_fresh,
            'query freshness/commit differs')
    known = {c['id']:c for c in chunks}
    for hit in query['hits']:
        require(hit['chunk_id'] in known, 'unknown query chunk')
        c = known[hit['chunk_id']]
        require(hit['citation']=={'file':c['file'],'start_line':c['start_line'],'end_line':c['end_line'],'commit_hash':commit}
                and hit['snippet']==DOC.density_view(c['text'],hit['density'])
                and c['commit_hash']==commit and BASE.sha(c['text'].encode())==c['content_sha256'],
                'query indexed snippet/coordinates differ')
        require(bool(hit.get('stale_citation')) is (not expected_fresh), 'query stale citation flag differs')
    return {'hits':len(query['hits']),'fresh':expected_fresh,'indexed_head_ckv':commit}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args(); root=args.bundle.parent
    descriptor_raw=args.bundle.read_bytes(); descriptor=BASE.decode(descriptor_raw)
    require(descriptor['diagnostic_only'] is True and descriptor.get('quality_metrics') is None, 'diagnostic scope required')
    def read(name):
        p=(root/name).resolve(); require(p.is_relative_to(root.resolve()),'bundle path escapes root')
        return p.read_bytes()
    book_raw=read(descriptor['fixture_book']);book=BASE.decode(book_raw)
    require(BASE.sha(book_raw)==descriptor['fixture_book_sha256'], 'fixture book changed')
    require([c['id'] for c in descriptor['cases']]==IDS, 'missing/duplicate DEV case inventory')
    rows=[];queries=[];coordinates=[]
    for c in descriptor['cases']:
        identity=BASE.decode(read(c['identity']));manifest=BASE.decode(read(c['manifest']))
        coordinates.append((identity['source']['project_id'],identity['dataset_id'],identity['source']['snapshot_id']))
        sources={k:read(v) for k,v in c['sources'].items()}
        fixture=next(f for f in book['fixtures'] if f['id']==c['id'].rsplit('-',1)[0])
        for version in ('original','fixed'):
            rows.append(audit_case(c['id'],fixture,c['state'],identity,manifest,sources,read(c[version+'_capture']),
                        read(c['requests']),read(c['config']),read(c['backend']),(root/c['sources']['main.go']).parent))
            queries.append(audit_query(BASE.decode(read(c[version+'_query'])),BASE.decode(read(c['chunks'])),c['state']['commit'],True))
    require(len(set(coordinates))==4 and coordinates[0][0]==coordinates[1][0]
            and coordinates[2][0]!=coordinates[3][0], 'state/project separation missing')
    old=descriptor['cases'][0]
    stale=[audit_query(BASE.decode(read(descriptor[k])),BASE.decode(read(old['chunks'])),old['state']['commit'],False)
           for k in ('stale_fixed_query',)]
    negatives=[]
    for n in descriptor['mixed_layout_captures']:
        cap=BASE.decode(read(n['capture']));requests_raw=read(n['requests']);config_raw=read(n['config'])
        require(cap['request_sha256']==BASE.sha(requests_raw) and cap['config_sha256']==cap['config_sha256_after']==BASE.sha(config_raw),
                'negative capture input binding differs')
        require(cap['state']=='partial' and len(cap['rows'])==1 and cap['quality_metrics'] is None, 'mixed layout was accepted or omitted')
        response=cap['rows'][0]['call']['response']
        require(response.get('isError') is True and response['structuredContent']['code']=='reindex_required', 'mixed layout error code differs')
        negatives.append({'capture_sha256':BASE.sha(read(n['capture'])),'observed_code':'reindex_required',
                          'expected_gold_code':'snapshot_mismatch','gold_oracle_satisfied':False,
                          'scope':'non-colocated native layout; not a pinned mixed-coordinate EvidencePackV2'})
    report={'schema_version':1,'diagnostic_only':True,'bundle_sha256':BASE.sha(descriptor_raw),'fixture_book_sha256':BASE.sha(book_raw),
            'state':'diagnostic_replayed','rows':rows,'queries':queries,'stale_query':stale,'mixed_layout':negatives,
            'quality_metrics':None,'human_verdict':None,'official_execution_ready':False,
            'remaining_oracle':'native pinned mixed-coordinate snapshot_mismatch; official input/pack/human review'}
    with args.out.open('x') as f:json.dump(report,f,ensure_ascii=False,indent=2);f.write('\n')
    print('F-04/F-05 DEV diagnostic replay: '+str(len(rows))+' normal SDK rows; official metrics remain null')


if __name__=='__main__':main()
