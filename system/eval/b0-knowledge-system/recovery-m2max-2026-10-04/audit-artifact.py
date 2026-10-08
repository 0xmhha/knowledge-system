from pathlib import Path
import json,hashlib,copy
root=Path('system/eval/b0-knowledge-system/recovery-m2max-2026-10-04')
def load(path):return json.loads((root/path).read_text())
def sha(raw):return hashlib.sha256(raw).hexdigest()
a=load('darwin-final/verification.json');b=load('legacy-final/verification.json');summary=load('summary.json')
assert a['require_complete_embeddings']==b['require_complete_embeddings']=='1'
assert a['original_version_sha_before']==a['original_version_sha_after']==a['backup_sha_after']==a['restored_sha_after']
assert b['pre_current_consumer_payload']==b['original_payload_before']==b['original_payload_after']==b['backup_payload_after']
assert b['old_v1_before']==b['old_v1_after']
assert load('darwin-final/pin-control/ready.json')==load('darwin-final/pin-control/after.json')
commands={r['id']:r for r in load('darwin-final/commands.json')}
for name in ['test-gate-failure','vector-build-failure','corrupt-target-rollback','corrupt-current-direct-rollback']:assert commands[name]['exit_code']==1
for name in ['normal-rollback','restore-backup','reactivate-updated','updated-build']:assert commands[name]['exit_code']==0
for snap in load('darwin-final/runtime-sidecar-snapshots.json'):
 for path,entry in snap['transient_runtime_files'].items():
  assert path in {'graph/graph.db-shm','graph/graph.db-wal','vector/vector.db-shm','vector/vector.db-wal'}
  if path.endswith('-wal'):assert entry['bytes']==0 and entry['sha256']==sha(b'')
for name in ['corrupt-target-rollback','corrupt-current-direct-rollback']:assert json.loads((root/'darwin-final'/f'{name}.stderr.txt').read_text())['code']=='snapshot_mismatch'
assert load('legacy-final/new-doctor.json')['identity_status']=='legacy_unpinned'
assert load('legacy-final/new-v2-error.json')['code']=='reindex_required'
rows=0;citations=0;v2=0
for path,version in [('darwin-final/baseline-capture.json','baseline'),('darwin-final/updated-capture.json','updated'),('darwin-final/normal-rollback-capture.json','baseline'),('darwin-final/restored-capture.json','restored'),('darwin-final/source-removed-capture.json','restored'),('legacy-final/rebuilt-capture.json','reindexed')]:
 report=load(path);assert report['state']=='captured' and report['quality_metrics'] is None
 assert report['binary_sha256']==report['binary_sha256_after']==summary['binary_sha256']['cks']
 assert report['config_sha256']==report['config_sha256_after']
 base=root/'retained'/version;identity=json.loads((base/'dataset-identity.json').read_text());source=json.loads((base/'sources/manifest.json').read_text());files={f['path']:f for f in source['files'] if f['kind']=='regular'}
 for row in report['rows']:
  assert not row.get('error') and not row['call'].get('error');response=row['call']['response'];assert not response.get('isError');pack=response['structuredContent'];assert pack['citations'];rows+=1
  for c in pack['citations']:
   raw=(base/'sources/blobs'/files[c['file']]['sha256']).read_bytes();assert sha(raw)==files[c['file']]['sha256'];assert c['commit_hash']==identity['source']['source_commit'];lines=raw.splitlines(keepends=True);assert 1<=c['start_line']<=c['end_line']<=len(lines);citations+=1
   if pack.get('format_version')==2:
    assert c['file_sha256']==sha(raw) and c['content_sha256']==sha(b''.join(lines[c['start_line']-1:c['end_line']]))
    assert c['dataset_id']==identity['dataset_id'] and c['snapshot_id']==identity['source']['snapshot_id']
  for body in pack['bodies']:
   c=body['citation'];assert c in pack['citations'];raw=(base/'sources/blobs'/files[c['file']]['sha256']).read_bytes();selected=b''.join(raw.splitlines(keepends=True)[c['start_line']-1:c['end_line']]);assert body['text'].encode()==(selected if pack.get('format_version')==2 else selected.rstrip(b'\n'))
  if pack.get('format_version')==2:
   v2+=1;canonical=copy.deepcopy(pack);expected=canonical['metadata'].pop('integrity_hash');assert sha(json.dumps(canonical,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode())==expected
result={'scope':'independent replay of retained owned fixture payload ledgers, SDK/source/integrity, typed failures and pin/legacy evidence; no DB re-execution or operating approval','sdk_rows':rows,'citations':citations,'v2_integrity_responses':v2,'before_after_backup_payload':True,'pin_before_after_equal':True,'quality_metrics':None};assert rows==11 and citations==11 and v2==6;(root/'independent-audit.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
