from pathlib import Path
source=Path('scripts/wbs-linux-environment-smoke.py').read_text()
verify=source[source.index('def verify('):source.index('\ndef main():')]
verify=verify.replace("assert env['model']['provider'] == 'mock' and env['model']['dimension'] == 64", """model = env['model']
        assert model['provider'] == 'ollama' and model['dimension'] == 1024
        assert model['model'] == 'bge-m3:latest' and model['digest'] == DIGEST
        assert model['server_version'] == '0.35.1'
        assert model['runtime_options'] == {'num_ctx':8192, 'num_batch':8192}
        resident = model['residency']
        assert resident['other_resident_model_count'] == 0
        assert len(resident['selected_models']) == 1
        assert resident['selected_models'][0]['digest'] == DIGEST
        assert resident['selected_models'][0]['size_vram'] == 0""")
verify=verify.replace("assert not summary.get('http')", """http = [c for c in summary['calls'] if c['backend'] == 'ollama_http']
        assert http and any(c['options']['path'] == '/api/embed' for c in http)
        assert all(c['outcome'] == 'returned' and c['http_status'] == 200 for c in http)
        assert all(c['options']['path'] in ['/api/tags','/api/embed'] for c in http)""")
header='''from pathlib import Path
import json,hashlib,copy,sqlite3,math,struct,sys
from urllib.parse import quote
DIGEST='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def load(path):return json.loads(path.read_text())
'''
extra='''
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
    assert (base/'mcp.stdout.txt').read_bytes()==(base/'mcp-restarted.stdout.txt').read_bytes()
    query=load(base/'rollback-query.stdout.txt');assert query['hits']
    assert all(x['citation']['commit_hash']==first['commit'] for x in query['hits'])
    return {'manifest':manifest,'chunks':len(chunks),'finite_normalized_vectors':len(vectors),'norms':norms,'first_doctor':first,'second_doctor':second,'rolled_doctor':rolled}

if __name__=='__main__':
    root=Path(sys.argv[1]);out={'scope':'Linux arm64 actual CPU BGE-M3 synthetic diagnostic; not official quality/release','cases':{}}
    for kind in ['empty-go','typescript','unsupported-python']:
        base=root/'diagnostic-after'/kind
        case=verify(base,base/'matrix');case['vector_audit']=vector_audit(base)
        out['cases'][kind]=case
    (root/'independent-verification.json').write_text(json.dumps(out,indent=2)+'\\n')
    print('72 rows, retained source/body hashes, rollback, model, quotas, inputs and vector DB verified')
'''
Path('/private/tmp/ks-linux-real-model-20261004/independent-check.py').write_text(header+verify+extra)
