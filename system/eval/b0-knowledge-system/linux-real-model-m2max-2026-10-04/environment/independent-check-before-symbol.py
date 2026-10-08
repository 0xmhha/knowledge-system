from pathlib import Path
import json,hashlib,copy,sqlite3,math,struct,sys
from urllib.parse import quote
DIGEST='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def load(path):return json.loads(path.read_text())
def verify(base, matrix):
    report = load(matrix / 'report.json')
    rows = [json.loads(line) for line in (matrix / 'rows.jsonl').read_text().splitlines()]
    assert report['state'] == 'captured' and report['rows'] == len(rows) == 24
    assert report['quality_metrics'] is None
    assert report['binary_sha256'] == report['binary_sha256_after']
    assert report['locked_files_sha256'] == report['locked_files_sha256_after']
    assert report['source_head'] == report['source_head_after']
    assert report['live_executable_bits'] == report['live_executable_bits_after']
    assert report['source_head'] != report['dataset_identity']['source']['source_commit'], 'rollback must retain old evidence'
    for side in ('before', 'after'):
        env = report['environment_' + side]
        assert env['valid'] and env['model']['valid']
        model = env['model']
        assert model['provider'] == 'ollama' and model['dimension'] == 1024
        assert model['model'] == 'bge-m3:latest' and model['digest'] == DIGEST
        assert model['server_version'] == '0.35.1'
        assert model['runtime_options'] == {'num_ctx':8192, 'num_batch':8192}
        resident = model['residency']
        assert resident['other_resident_model_count'] == 0
        assert len(resident['selected_models']) == 1
        assert resident['selected_models'][0]['digest'] == DIGEST
        assert resident['selected_models'][0]['size_vram'] == 0
        hw = env['hardware']
        assert hw['os'] == 'linux' and hw['cpu_brand'] and hw['process_count'] > 0
        assert hw['process_pressure_method'].startswith('Linux /proc/')
        assert hw['resource_limits']['v2.cpu.max'] == '100000 100000'
        assert hw['resource_limits']['v2.memory.max'] == '536870912'
        assert all(set(p) == {'pid', 'parent_pid', 'cpu_percent', 'rss_bytes'} for p in hw['top_cpu_processes'])
    version = (base / 'dataset/current').resolve()
    manifest = load(version / 'sources/manifest.json')
    source = {entry['path']: (version / 'sources/blobs' / entry['sha256']).read_bytes()
              for entry in manifest['files'] if entry['kind'] == 'regular'}
    identity = report['dataset_identity']
    comparisons, ids, citations = {}, set(), 0
    scopes, startups, backend_errors = {}, 0, []
    for file in (matrix / 'footprints').rglob('*.jsonl'):
        for line in file.read_text().splitlines():
            event = json.loads(line)
            if event.get('event') != 'measurement.backend_calls':
                continue
            summary = event['summary']
            if summary['tool'] == 'startup.intent_anchors':
                startups += 1
                continue
            assert summary['measurement_id'] not in scopes
            scopes[summary['measurement_id']] = summary
    for original, copied in report['locked_input_copies'].items():
        raw = (matrix / copied['path']).read_bytes()
        assert sha(raw) == copied['sha256'] == copied['sha256_after'] == report['locked_files_sha256'][original]
        assert (matrix / copied['path']).stat().st_mode & 0o777 == 0o600
    for arm in report['arms']:
        assert arm['config_sha256'] == arm['config_sha256_after']
    arm_ids = [arm['id'] for arm in report['arms']]
    for row in rows:
        assert row['arm'] == arm_ids[(row['group'] + row['position'] - 1) % 8]
        call = row['call']
        assert not call.get('error') and call['elapsed_ns'] > 0
        assert call['measurement_id'] not in ids
        ids.add(call['measurement_id'])
        summary = scopes[call['measurement_id']]
        assert summary['tool'] == call['tool'] and summary['outcome'] == 'returned'
        http = [c for c in summary['calls'] if c['backend'] == 'ollama_http']
        assert http and any(c['options']['path'] == '/api/embed' for c in http)
        assert all(c['outcome'] == 'returned' and c['http_status'] == 200 for c in http)
        assert all(c['options']['path'] in ['/api/tags','/api/embed'] for c in http)
        search = [c for c in summary['calls'] if c['backend'] == 'ckv' and c['method'] == 'semantic_search']
        assert search and search[0]['options']['K'] == 20
        backend_errors.extend({'measurement_id': call['measurement_id'], 'call': c}
                              for c in summary['calls'] if c['outcome'] != 'returned')
        response = call['response']
        assert not response.get('isError')
        pack = response.get('structuredContent') or json.loads(response['content'][0]['text'])
        clone = copy.deepcopy(pack)
        expected = clone['metadata'].pop('integrity_hash')
        assert sha(json.dumps(clone, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()) == expected
        assert pack['coordinates']['dataset_id'] == identity['dataset_id']
        assert pack['citations'], 'synthetic guide must return retained evidence'
        for citation in pack['citations']:
            for key, expected in {'project_id': identity['source']['project_id'],
                                  'dataset_id': identity['dataset_id'],
                                  'snapshot_id': identity['source']['snapshot_id'],
                                  'commit_hash': identity['source']['source_commit']}.items():
                assert citation[key] == expected
            raw = source[citation['file']]
            lines = raw.splitlines(keepends=True)
            assert 1 <= citation['start_line'] <= citation['end_line'] <= len(lines)
            span = b''.join(lines[citation['start_line'] - 1:citation['end_line']])
            assert sha(raw) == citation['file_sha256'] and sha(span) == citation['content_sha256']
            citations += 1
        for body in pack['bodies']:
            assert body['citation'] in pack['citations']
            assert sha(body['text'].encode()) == body['citation']['content_sha256']
        value = {'citations': pack['citations'], 'bodies': pack['bodies']}
        mode = row['arm'].rsplit('_', 1)[0]
        ontology = pack['metadata'].get('ontology')
        if mode == 'baseline':
            assert ontology is None
        else:
            assert ontology['state'] == 'unavailable' and ontology['mode'] == mode
        key = call['arguments']['include_knowledge']
        if key in comparisons:
            assert value == comparisons[key], 'fallback arm/phase changed retained evidence'
        comparisons[key] = value
    assert len(ids) == len(scopes) == 24 and startups == 16
    assert (matrix / 'report.json').stat().st_mode & 0o777 == 0o600
    assert (matrix / 'rows.jsonl').stat().st_mode & 0o777 == 0o600
    return {'rows': len(rows), 'citations_verified': citations, 'unique_measurement_ids': len(ids),
            'state': report['state'], 'quality_metrics': None, 'source_head': report['source_head'],
            'indexed_commit': identity['source']['source_commit'], 'dataset_id': identity['dataset_id'],
            'startup_scopes': startups, 'backend_errors': backend_errors, 'source_snapshot': manifest}


def vector_audit(base):
    current=(base/'dataset/current').resolve()
    snapshot=load(current/'sources/manifest.json')
    source={entry['path']:(current/'sources/blobs'/entry['sha256']).read_bytes() for entry in snapshot['files'] if entry['kind']=='regular'}
    with sqlite3.connect('file:'+quote(str(current/'vector/vector.db'),safe='/')+'?mode=ro',uri=True) as db:
        db.row_factory=sqlite3.Row
        manifest=dict(db.execute('select key,value from manifest').fetchall())
        assert manifest['embedding_dim']=='1024' and manifest['embedding_model_digest']==DIGEST
        chunks={x['id']:dict(x) for x in db.execute('select * from chunks')}
        vectors=[dict(x) for x in db.execute('select * from chunk_vec_rowids')]
        blocks=dict(db.execute('select rowid,vectors from chunk_vec_vector_chunks00'))
        assert set(chunks)=={x['id'] for x in vectors}
        norms=[]
        for mapping in vectors:
            offset=mapping['chunk_offset']*4096
            raw=blocks[mapping['chunk_id']][offset:offset+4096]
            assert len(raw)==4096
            v=struct.unpack('<1024f',raw)
            assert all(math.isfinite(x) for x in v)
            norm=math.sqrt(sum(x*x for x in v));assert abs(norm-1)<.001;norms.append(norm)
        for chunk in chunks.values():
            raw=source[chunk['file']];lines=raw.splitlines(keepends=True)
            assert 1<=chunk['start_line']<=chunk['end_line']<=len(lines)
            span=b''.join(lines[chunk['start_line']-1:chunk['end_line']])
            assert span.decode()==chunk['text']
            assert sha(span)==chunk['content_sha256']
            assert chunk['commit_hash']==manifest['indexed_head']
    first=load(base/'doctor.stdout.txt');second=load(base/'second-doctor.stdout.txt');rolled=load(base/'rollback-doctor.stdout.txt')
    assert first['dataset_version']!=second['dataset_version']
    assert rolled['dataset_version']==first['dataset_version']
    assert rolled['indexed_commit']==first['commit'] and rolled['commit']==second['commit']
    assert (base/('mcp.stdout.txt' if base.name=='empty-go' else 'mcp-retry-90s.stdout.txt')).read_bytes()==(base/'mcp-restarted.stdout.txt').read_bytes()
    query=load(base/'rollback-query.stdout.txt');assert query['hits']
    assert all(x['citation']['commit_hash']==first['commit'] for x in query['hits'])
    return {'manifest':manifest,'chunks':len(chunks),'finite_normalized_vectors':len(vectors),'norms':norms,'first_doctor':first,'second_doctor':second,'rolled_doctor':rolled}

if __name__=='__main__':
    root=Path(sys.argv[1]);out={'scope':'Linux arm64 actual CPU BGE-M3 synthetic diagnostic; not official quality/release','cases':{}}
    for kind in ['empty-go','typescript','unsupported-python']:
        base=root/'diagnostic-after'/kind
        case=verify(base,base/'matrix');case['vector_audit']=vector_audit(base)
        from collections import Counter
        counts={'query':Counter(),'startup':Counter()}
        for f in (base/'matrix/footprints').rglob('*.jsonl'):
            for line in f.read_text().splitlines():
                event=json.loads(line)
                if event.get('event')!='measurement.backend_calls':continue
                summary=event['summary'];phase='startup' if summary['tool']=='startup.intent_anchors' else 'query'
                for call in summary['calls']:
                    key=call['backend']+'.'+call['method']
                    if call['backend']=='ollama_http':key+='.'+call['options']['path']
                    counts[phase][key]+=1
        case['backend_call_counts']={phase:dict(c) for phase,c in counts.items()}
        case['neighbors_failures_preserved']=sum(c['call']['backend']=='ckg' and c['call']['method']=='neighbors' for c in case['backend_errors'])
        out['cases'][kind]=case
    (root/'independent-verification.json').write_text(json.dumps(out,indent=2)+'\n')
    print('72 rows, retained source/body hashes, rollback, model, quotas, inputs and vector DB verified')
