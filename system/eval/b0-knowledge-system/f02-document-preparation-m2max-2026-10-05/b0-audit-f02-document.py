#!/usr/bin/env python3
"""Audit diagnostic F-02 DEV child/parent bytes and captured raw/query evidence."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sqlite3
import sys

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location('f02_raw_reader', Path(__file__).with_name('b0-summarize-vector-probe.py'))
RAW = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RAW)
BASE = RAW.BASE


def chunk_id(file, start, end, text_sha):
    return BASE.sha(f'{file}\n{start}:{end}\n{text_sha}'.encode())


def audit_children(chunks, fixture, source, commit, cap):
    RAW.require(fixture['id'] == 'F-02-DEV' and fixture['evaluation_partition'] == 'development',
                'only diagnostic F-02 DEV supported; FINAL approval/collector is separate')
    oracle = fixture['expected']
    RAW.require(oracle.get('parent_id_required') is True and oracle.get('reassembled_children_equal_source') is True
                and oracle.get('truncation_count') == 0, 'unexpected F-02 source contract')
    citation = oracle['citations'][0]
    file = citation['path']
    RAW.require(BASE.sha(source) == fixture['source_sha256'][file]
                and source == fixture['sources'][file].encode(), 'frozen F-02 source differs')
    lines = source.splitlines(keepends=True)
    start, end = oracle['parent_span']
    RAW.require(start == 1 and end == len(lines) and RAW.integer(cap,1)
                and all(len(line) <= cap for line in lines), 'unsupported parent/span/oversized-line fixture')
    parent_sha = BASE.sha(b''.join(lines[start-1:end]))
    parent_id = chunk_id(file,start,end,parent_sha)
    RAW.require(len(chunks) > 1, 'F-02 parent was not split into children')
    ordered = sorted(chunks,key=lambda c:c['part_ordinal'])
    seen, cursor, joined, headings = set(), start, [], None
    records = []
    for ordinal, child in enumerate(ordered,1):
        RAW.require(child['file'] == file and child['commit_hash'] == commit and child['language'] == 'markdown'
                    and child['chunk_kind'] == 'doc', 'foreign/non-document child')
        RAW.require(RAW.integer(child['part_ordinal'],1) and child['part_ordinal'] == ordinal
                    and RAW.integer(child['start_line'],1) and RAW.integer(child['end_line'],1)
                    and child['start_line'] == cursor and cursor <= child['end_line'] <= end,
                    'duplicate/gapped child ordinal or source coverage')
        RAW.require(RAW.integer(child['parent_start_line'],1) and RAW.integer(child['parent_end_line'],1)
                    and child['parent_id'] == parent_id and child['parent_start_line'] == start
                    and child['parent_end_line'] == end, 'parent coordinate/hash differs')
        text = child['text'].encode()
        RAW.require(text == b''.join(lines[child['start_line']-1:child['end_line']])
                    and 0 < len(text) <= cap and BASE.sha(text) == child['content_sha256'], 'child bytes/hash/cap differ')
        RAW.require(child['id'] not in seen and child['id'] == chunk_id(file,child['start_line'],child['end_line'],BASE.sha(text)),
                    'duplicate or noncanonical child ID')
        heading = child['heading_path']
        RAW.require(isinstance(heading,list) and bool(heading) and all(isinstance(s,str) and s for s in heading)
                    and heading == [lines[0].decode().strip().lstrip('#').strip()]
                    and (headings is None or headings == heading), 'heading path lost or changed')
        seen.add(child['id']); joined.append(text); cursor = child['end_line']+1; headings = heading
        records.append({'id':child['id'],'ordinal':ordinal,'start_line':child['start_line'],
                        'end_line':child['end_line'],'bytes':len(text),'text_sha256':BASE.sha(text)})
    RAW.require(cursor == end+1 and b''.join(joined) == source, 'reassembly lost source/tail bytes')
    tail = b''.join(lines[citation['start_line']-1:citation['end_line']])
    tail_ids = [c['id'] for c in ordered if c['start_line'] <= citation['start_line']
                and c['end_line'] >= citation['end_line'] and tail in c['text'].encode()]
    RAW.require(bool(tail_ids) and ordered[-1]['id'] in tail_ids, 'tail source is not preserved by final child')
    return {'parent_id':parent_id,'parent_span':[start,end],'parent_sha256':parent_sha,
            'source_bytes':len(source),'child_count':len(ordered),'children':records,
            'child_bytes_reassemble_source':True,'tail_child_ids':tail_ids,'tail_span_sha256':BASE.sha(tail),
            'byte_cap':cap,'heading_path':headings,'structural_truncation_count':0}


def audit_raw_probe(raw, manifest_raw, fixture, sources, children, structure):
    probe = BASE.decode(raw)
    prompts = {q['prompt'] for q in fixture['queries']}
    RAW.require(probe.get('prompt') in prompts and probe.get('k') == fixture['k'] and probe.get('filter') == fixture['filters'],
                'captured raw query differs from frozen DEV definition')
    if probe.get('state')=='error':
        return {'probe_sha256':BASE.sha(raw),'prompt':probe['prompt'],'state':'measurement_failed',
                'error':probe.get('error'),'tail_raw_ranks':None,'normal_exact_agreement':None}
    expected = [dict(file=c['path'],start_line=c['start_line'],end_line=c['end_line'],commit_hash=probe['coordinates']['commit'])
                for c in fixture['expected']['citations']]
    controls = {'schema_version':1,'diagnostic_only':True,'probe_sha256':BASE.sha(raw),'expected_citations':expected,
                **{k:probe[k] for k in ('coordinates','embedding_identity','prompt','filter','k','max_exact_candidates')}}
    replay = RAW.summarize(raw,manifest_raw,controls,sources)
    RAW.require(not replay['invariant_violations'], 'raw probe invariant violations')
    RAW.require(probe['oracle']['eligible_count'] == len(children)
                and {c['id'] for c in probe['oracle']['eligible_hits']} == {c['id'] for c in children},
                'exact full vector inventory differs from stored children')
    by_id = {c['id']:c for c in children}
    for name in ('unfiltered','filtered','budget'):
        for hit in probe[name]['Hits'] or []:
            RAW.require(hit['chunk']['id'] in by_id, 'raw probe unknown child')
            stored = by_id[hit['chunk']['id']]
            RAW.require(all(hit['chunk'].get(k) == stored[k] for k in ('file','start_line','end_line','commit_hash',
                        'content_sha256','text','parent_id','parent_start_line','parent_end_line','part_ordinal','heading_path')),
                        'raw probe parent/child projection differs')
    return {'probe_sha256':BASE.sha(raw),'prompt':probe['prompt'],'k':probe['k'],
            'search_state':probe['unfiltered']['Status'],'search_reason':probe['unfiltered']['Reason'],
            'returned_child_count':len(probe['unfiltered']['Hits'] or []),
            'tail_raw_ranks':[i+1 for i,h in enumerate(probe['unfiltered']['Hits'] or []) if h['chunk']['id'] in structure['tail_child_ids']],
            'normal_exact_agreement':probe['exact_agreement'],'reader_report':replay}


def audit_query(raw, prompt, children, structure, commit):
    query = BASE.decode(raw)
    RAW.require(query['metadata'].get('indexed_head_ckv') == commit and query['metadata'].get('fresh') is True,
                'CKV query source identity/freshness differs')
    by_id, tail_ranks, snippet_tail = {c['id']:c for c in children}, [], []
    for rank, hit in enumerate(query['hits'],1):
        RAW.require(hit['chunk_id'] in by_id, 'query returned unknown child')
        c = by_id[hit['chunk_id']]
        RAW.require(hit['citation'] == {'file':c['file'],'start_line':c['start_line'],'end_line':c['end_line'],'commit_hash':commit}
                    and hit.get('parent_citation') == {'file':c['file'],'start_line':c['parent_start_line'],
                                                     'end_line':c['parent_end_line'],'commit_hash':commit}
                    and hit.get('heading_path') == ' / '.join(c['heading_path']) and not hit.get('stale_citation'),
                    'query child/parent citation or heading projection differs')
        RAW.require(hit.get('snippet') == density_view(c['text'],hit.get('density')),
                    'query snippet not a declared density view of the child')
        if c['id'] in structure['tail_child_ids']:
            tail_ranks.append(rank)
            # The tail citation can remain valid when density drops its body.
            snippet_tail.append({'rank':rank,'density':hit['density'],'snippet_keeps_final_line':c['text'].splitlines()[-1] in hit['snippet']})
    return {'response_sha256':BASE.sha(raw),'prompt':prompt,'hits':len(query['hits']),
            'tail_citation_ranks':tail_ranks,'tail_snippet_observations':snippet_tail,'metadata':query['metadata']}


def density_view(text, density):
    lines = text.split('\n')
    if density == 'full': return text
    if density == 'signature_only': return next((line for line in lines if line.strip()),'')
    RAW.require(density == 'signature+N', 'unknown snippet density')
    kept, count = [], 0
    for index,line in enumerate(lines):
        if index == 0 or line.strip():
            kept.append(line)
            if index > 0: count += 1
            if count >= 5: break
    return '\n'.join(kept)


def audit_sdk(capture_raw,requests_raw,config_raw,fixture,manifest,sources):
    capture,requests=BASE.decode(capture_raw),BASE.decode(requests_raw)
    RAW.require(capture.get('state') in ('captured','partial') and capture.get('quality_metrics') is None
                and capture['request_sha256']==BASE.sha(requests_raw)
                and capture['config_sha256']==capture['config_sha256_after']==BASE.sha(config_raw)
                and RAW.digest(capture['binary_sha256']) and capture['binary_sha256']==capture['binary_sha256_after'],
                'SDK capture input/binary binding differs')
    RAW.require(capture['counts']=={'retrieval':1,'warmup':0,'warm_latency':0,'cold_process':0}, 'unexpected diagnostic SDK repetitions')
    inventory=requests['requests'];by_id={r['id']:r for r in inventory}
    RAW.require(len(by_id)==len(inventory)==len(fixture['queries'])
                and {r['arguments']['prompt'] for r in inventory}=={q['prompt'] for q in fixture['queries']}
                and all(r['tool']=='cks.context.get_for_task_v2' and set(r['arguments'])=={'prompt'} for r in inventory),
                'SDK request differs from frozen DEV prompts/arguments')
    identity={'dataset_id':manifest['dataset_id'],'source':{'project_id':manifest['project_id'],
              'snapshot_id':manifest['snapshot_id'],'source_commit':manifest['src_commit'],'source_mode':manifest['source_mode']}}
    expected=fixture['expected']['citations'][0]
    lines=BASE.source_bytes(sources,expected['path']).splitlines(keepends=True)
    tail=b''.join(lines[expected['start_line']-1:expected['end_line']]).decode()
    seen,mids,rows=set(),set(),[]
    for row in capture['rows']:
        qid=row['request_id'];call=row['call']
        RAW.require(qid in by_id and qid not in seen and row['phase']=='retrieval' and row['iteration']==1
                    and call['tool']==by_id[qid]['tool'] and call['arguments']==by_id[qid]['arguments'], 'SDK query slot/arguments differ')
        seen.add(qid)
        mid=call.get('measurement_id')
        RAW.require(isinstance(mid,str) and mid and mid not in mids, 'SDK measurement ID missing/duplicate');mids.add(mid)
        response=call.get('response',{})
        failed=bool(call.get('transport_error') or call.get('init_error') or response.get('isError'))
        result={'request_id':qid,'measurement_id':mid,'failed':failed,'response_sha256':BASE.sha(json.dumps(response,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode())}
        if not failed:
            pack=response['structuredContent'];BASE.verify_pack(pack,identity,sources)
            RAW.require(pack['query']==call['arguments']['prompt'] and pack.get('semantic') is None, 'SDK query/semantic projection differs')
            tail_refs=[c for c in pack['citations'] if c['file']==expected['path']
                       and c['start_line']<=expected['start_line'] and c['end_line']>=expected['end_line']]
            result.update(citations=len(pack['citations']),bodies=len(pack['bodies']),tail_source_references=tail_refs,
                          tail_body_count=sum(b['citation'] in tail_refs and tail in b['text'] for b in pack['bodies']))
        rows.append(result)
    return {'capture_sha256':BASE.sha(capture_raw),'request_sha256':BASE.sha(requests_raw),'config_sha256':BASE.sha(config_raw),
            'binary_sha256':capture['binary_sha256'],'planned_calls':len(inventory),'observed_calls':len(rows),
            'failed_calls':sum(r['failed'] for r in rows),'missing_calls':len(by_id)-len(seen),'rows':rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixture-manifest',type=Path,default=ROOT/'system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json')
    parser.add_argument('--vector-dir',type=Path,required=True)
    parser.add_argument('--source-root',type=Path,required=True)
    parser.add_argument('--probe',type=Path,action='append',default=[])
    parser.add_argument('--query',type=Path,action='append',default=[])
    parser.add_argument('--sdk-capture',type=Path)
    parser.add_argument('--sdk-requests',type=Path)
    parser.add_argument('--sdk-config',type=Path)
    parser.add_argument('--out',type=Path,required=True)
    args = parser.parse_args()
    book_raw = args.fixture_manifest.read_bytes()
    book = BASE.decode(book_raw)
    fixture = next(f for f in book['fixtures'] if f['id']=='F-02-DEV')
    vector = args.vector_dir.resolve(); db_path = vector/'vector.db'; manifest_raw = (vector/'manifest.json').read_bytes()
    manifest = BASE.decode(manifest_raw)
    for suffix in ('-wal','-journal'):
        p=Path(str(db_path)+suffix)
        RAW.require(not p.exists() or p.stat().st_size==0, 'unsealed database side file')
    before = BASE.sha(db_path.read_bytes())
    RAW.require(before == manifest['db_sha256'] and manifest['project_id']=='f-02-dev'
                and manifest.get('source_mode')=='committed', 'unsealed/foreign source identity')
    RAW.require(manifest['embedding_identity_v2']['model_digest']=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
                and manifest['embedding_identity_v2']['Dim']==1024, 'not the approved BGE-M3 bytes/dimension')
    db=sqlite3.connect(db_path.as_uri()+'?mode=ro',uri=True);db.row_factory=sqlite3.Row
    try:
        rows=[dict(r) for r in db.execute('SELECT id,file,start_line,end_line,language,chunk_kind,commit_hash,content_sha256,text,parent_id,parent_start_line,parent_end_line,part_ordinal,heading_path FROM chunks ORDER BY id')]
    finally: db.close()
    for row in rows: row['heading_path']=BASE.decode(row['heading_path'])
    source=BASE.source_bytes(args.source_root,fixture['expected']['citations'][0]['path'])
    result=audit_children(rows,fixture,source,manifest['src_commit'],manifest['embedding_identity_v2']['chunk_budget_bytes'])
    probes=[audit_raw_probe(p.read_bytes(),manifest_raw,fixture,args.source_root,rows,result) for p in args.probe]
    RAW.require(not probes or len(probes)==len(fixture['queries']) and {p['prompt'] for p in probes}=={q['prompt'] for q in fixture['queries']}, 'raw probe DEV inventory missing/duplicate')
    RAW.require(not args.query or len(args.query)==len(fixture['queries']), 'CKV query inventory differs')
    queries=[audit_query(p.read_bytes(),q['prompt'],rows,result,manifest['src_commit']) for p,q in zip(args.query,fixture['queries'])]
    RAW.require(all([args.sdk_capture,args.sdk_requests,args.sdk_config]) or not any([args.sdk_capture,args.sdk_requests,args.sdk_config]), 'all SDK capture/requests/config inputs required')
    sdk=audit_sdk(args.sdk_capture.read_bytes(),args.sdk_requests.read_bytes(),args.sdk_config.read_bytes(),fixture,manifest,args.source_root) if args.sdk_capture else None
    after=BASE.sha(db_path.read_bytes())
    RAW.require(before==after and manifest_raw==(vector/'manifest.json').read_bytes(), 'published input changed during audit')
    report={'schema_version':1,'diagnostic_only':True,'fixture_id':'F-02-DEV','fixture_manifest_sha256':BASE.sha(book_raw),
            'database_sha256_before':before,'database_sha256_after':after,'manifest_sha256':BASE.sha(manifest_raw),
            'source_commit':manifest['src_commit'],'structure':result,'stored_children':rows,'raw_probes':probes,'ckv_queries':queries,'sdk_capture':sdk,
            'quality_metrics':None,'official_F02_verdict':None,'human_answer_verdict':None,'official_execution_ready':False}
    with open(args.out,'x',encoding='utf-8',opener=lambda p,f:os.open(p,f,0o600)) as stream:
        json.dump(report,stream,ensure_ascii=False,indent=2,allow_nan=False);stream.write('\n')
    return 0


if __name__=='__main__':
    try: sys.exit(main())
    except (ValueError,KeyError,TypeError,OSError,sqlite3.Error,StopIteration) as error:
        print('F-02 diagnostic audit refused: '+str(error),file=sys.stderr);sys.exit(1)
