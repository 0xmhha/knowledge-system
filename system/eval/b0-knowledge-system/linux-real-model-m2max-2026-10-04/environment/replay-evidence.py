"""Replay archived SDK/source/hash evidence only; no live API/DB/hardware verdict."""
from pathlib import Path
import json,hashlib,copy,sys

def load(p):return json.loads(p.read_text())
def sha(raw):return hashlib.sha256(raw).hexdigest()
root=Path(sys.argv[1]);counts={}
for kind in ['empty-go','typescript','unsupported-python']:
    base=root/'projects'/kind;matrix=base/'matrix';report=load(matrix/'report.json')
    rows=[json.loads(x) for x in (matrix/'rows.jsonl').read_text().splitlines()]
    assert report['state']=='captured' and report['rows']==len(rows)==24 and report['quality_metrics'] is None
    assert report['binary_sha256']==report['binary_sha256_after']==load(root/'package/manifest.json')['binaries']['cks']['sha256']
    version=base/'versions'/load(base/'doctor.stdout.txt')['dataset_version']
    manifest=load(version/'sources/manifest.json');sources={}
    for entry in manifest['files']:
        if entry['kind']!='regular':continue
        raw=(version/'sources/blobs'/entry['sha256']).read_bytes();assert sha(raw)==entry['sha256'];sources[entry['path']]=raw
    identity=report['dataset_identity'];scopes={};startups=0
    for f in (matrix/'footprints').rglob('*.jsonl'):
        for line in f.read_text().splitlines():
            event=json.loads(line)
            if event.get('event')!='measurement.backend_calls':continue
            s=event['summary']
            if s['tool']=='startup.intent_anchors':startups+=1;continue
            assert s['measurement_id'] not in scopes;scopes[s['measurement_id']]=s
    ids=set();citations=0
    for row in rows:
        assert row['arm']==report['arms'][(row['group']+row['position']-1)%8]['id']
        call=row['call'];assert not call.get('error') and call['measurement_id'] not in ids;ids.add(call['measurement_id'])
        s=scopes[call['measurement_id']];assert s['outcome']=='returned' and s['tool']==call['tool']
        http=[c for c in s['calls'] if c['backend']=='ollama_http'];assert http and all(c['outcome']=='returned' and c['http_status']==200 for c in http)
        assert any(c['backend']=='ckv' and c['method']=='semantic_search' and c['options']['K']==20 for c in s['calls'])
        response=call['response'];assert not response.get('isError')
        pack=response.get('structuredContent') or json.loads(response['content'][0]['text'])
        clone=copy.deepcopy(pack);expected=clone['metadata'].pop('integrity_hash')
        assert sha(json.dumps(clone,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode())==expected
        assert pack['coordinates']['dataset_id']==identity['dataset_id'] and pack['citations']
        for c in pack['citations']:
            for key,value in {'project_id':identity['source']['project_id'],'dataset_id':identity['dataset_id'],'snapshot_id':identity['source']['snapshot_id'],'commit_hash':identity['source']['source_commit']}.items():assert c[key]==value
            raw=sources[c['file']];lines=raw.splitlines(keepends=True);assert 1<=c['start_line']<=c['end_line']<=len(lines)
            assert sha(raw)==c['file_sha256'] and sha(b''.join(lines[c['start_line']-1:c['end_line']]))==c['content_sha256'];citations+=1
        for body in pack['bodies']:
            assert body['citation'] in pack['citations'] and sha(body['text'].encode())==body['citation']['content_sha256']
    assert len(ids)==len(scopes)==24 and startups==16
    for copied in report['locked_input_copies'].values():assert sha((matrix/copied['path']).read_bytes())==copied['sha256']==copied['sha256_after']
    counts[kind]={'rows':24,'citations':citations,'startup_scopes':startups}
print(json.dumps({'state':'archived-source-SDK-replay-verified','scope':'SDK/source/integrity/backend linkage; live model/DB/permissions/hardware not re-executed','cases':counts},indent=2))
