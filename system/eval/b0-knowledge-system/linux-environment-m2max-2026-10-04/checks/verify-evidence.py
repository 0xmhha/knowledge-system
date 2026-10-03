#!/usr/bin/env python3
"""Read-only audit of archived bytes; runtime permissions were checked separately."""
import collections
import copy
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent

def digest(raw):
    return hashlib.sha256(raw).hexdigest()

def load(path):
    return json.loads(path.read_text())

summary=load(ROOT/'summary.json')
expected={item['path'] for item in summary['artifacts']}
actual={str(p.relative_to(ROOT)) for p in ROOT.rglob('*') if p.is_file() and p.name!='summary.json'}
assert expected==actual
for item in summary['artifacts']:
    raw=(ROOT/item['path']).read_bytes()
    assert len(raw)==item['size'] and digest(raw)==item['sha256'],item['path']
rows_total=0
for runtime in ('runtime-arm64-corrected','runtime-amd64'):
    errors=collections.Counter()
    for kind in ('empty-go','typescript','unsupported-python'):
        directory=ROOT/runtime/('matrix-'+kind)
        report=load(directory/'report.json')
        assert report['state']=='captured' and report['quality_metrics'] is None
        assert report['binary_sha256']==report['binary_sha256_after']==summary['build']['binaries']['arm64' if runtime.endswith('corrected') else 'amd64']['cks-after']['sha256']
        assert report['locked_files_sha256']==report['locked_files_sha256_after']
        assert report['source_head']==report['source_head_after']!=report['dataset_identity']['source']['source_commit']
        assert report['live_executable_bits']==report['live_executable_bits_after']
        for side in ('before','after'):
            env=report['environment_'+side]
            assert env['valid'] and env['model']['provider']=='mock' and env['model']['dimension']==64
            hw=env['hardware']
            assert hw['os']=='linux' and hw['process_count']>0 and hw['cpu_brand']
            assert hw['resource_limits']['v2.cpu.max']=='100000 100000'
            assert hw['resource_limits']['v2.memory.max']=='536870912'
        for original,value in report['locked_input_copies'].items():
            assert digest((directory/value['path']).read_bytes())==value['sha256']==value['sha256_after']==report['locked_files_sha256'][original]
        for arm in report['arms']:
            assert digest((directory/(arm['id']+'.yaml')).read_bytes())==arm['config_sha256']==arm['config_sha256_after']
        raw=(directory/'rows.jsonl').read_bytes()
        assert digest(raw)==report['rows_sha256']
        rows=[json.loads(line) for line in raw.splitlines()]
        assert len(rows)==report['rows']==24
        scopes={};startups=0
        for file in (directory/'footprints').rglob('*.jsonl'):
            for line in file.read_text().splitlines():
                event=json.loads(line)
                if event.get('event')!='measurement.backend_calls':continue
                value=event['summary']
                if value['tool']=='startup.intent_anchors':startups+=1;continue
                assert value['measurement_id'] not in scopes
                scopes[value['measurement_id']]=value
        assert startups==16 and len(scopes)==24
        source=ROOT/runtime/'install'/kind/'retained-source'
        manifest=load(source/'manifest.json')
        files={f['path']:(source/'blobs'/f['sha256']).read_bytes() for f in manifest['files'] if f['kind']=='regular'}
        identity=report['dataset_identity'];arms=[a['id'] for a in report['arms']];ids=set();comparisons={}
        for row in rows:
            assert row['arm']==arms[(row['group']+row['position']-1)%8]
            call=row['call'];ids.add(call['measurement_id'])
            scope=scopes[call['measurement_id']]
            assert scope['tool']==call['tool'] and scope['outcome']=='returned'
            searches=[c for c in scope['calls'] if c['backend']=='ckv' and c['method']=='semantic_search']
            assert searches[0]['options']['K']==20 and not scope.get('http')
            errors.update((c['backend'],c['method']) for c in scope['calls'] if c['outcome']!='returned')
            assert not call.get('error') and not call['response'].get('isError')
            pack=call['response']['structuredContent'];clone=copy.deepcopy(pack)
            want=clone['metadata'].pop('integrity_hash')
            assert digest(json.dumps(clone,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode())==want
            assert pack['coordinates']['dataset_id']==identity['dataset_id'] and pack['citations']
            for citation in pack['citations']:
                assert citation['commit_hash']==identity['source']['source_commit'] and citation['dataset_id']==identity['dataset_id']
                assert citation['project_id']==identity['source']['project_id'] and citation['snapshot_id']==identity['source']['snapshot_id']
                raw=files[citation['file']];lines=raw.splitlines(keepends=True)
                assert 1<=citation['start_line']<=citation['end_line']<=len(lines)
                span=b''.join(lines[citation['start_line']-1:citation['end_line']])
                assert digest(raw)==citation['file_sha256'] and digest(span)==citation['content_sha256']
            for body in pack['bodies']:
                assert body['citation'] in pack['citations'] and digest(body['text'].encode())==body['citation']['content_sha256']
            value={'citations':pack['citations'],'bodies':pack['bodies']};key=call['arguments']['include_knowledge']
            if key in comparisons:assert comparisons[key]==value
            comparisons[key]=value
        assert len(ids)==24;rows_total+=len(rows)
    assert errors=={('ckg','neighbors'):72},errors
failed=load(ROOT/'runtime-arm64/matrix-empty-go/report.json')
assert failed['state']=='partial' and failed['rows']==24
bad=[json.loads(l) for l in (ROOT/'runtime-arm64/matrix-empty-go/rows.jsonl').read_text().splitlines()]
assert sum(bool(r['call']['response'].get('isError')) for r in bad)==12
assert rows_total==144
print(json.dumps({'artifacts':len(expected),'sdk_rows':rows_total,'state':'verified_with_limits','quality_metrics':None}))
