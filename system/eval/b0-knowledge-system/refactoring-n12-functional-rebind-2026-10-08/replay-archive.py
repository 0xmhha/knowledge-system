from pathlib import Path
import json,subprocess,tempfile,shutil,hashlib
root=Path.cwd();out=root/'system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08';prep=json.loads((out/'preparation.json').read_text());base=Path(prep['base']);restore=base/'archive-replay'
assert str(base).startswith('/private/tmp/ks-n12-functional-')and restore.is_dir()
originals=[Path(prep['source']),Path(prep['dataset_root']),base/'semantic-functional-proposed.sqlite'];hidden=[]
driver=Path(tempfile.mkdtemp(prefix='.n12-functional-replay-',dir=root));file=driver/'main.go';file.write_bytes((restore/'layer/projection-driver.go.txt').read_bytes())
result=None
try:
 for p in originals:
  target=p.with_name(p.name+'.hidden-for-owned-replay');assert p.exists()and not target.exists();p.rename(target);hidden.append((p,target))
 assert all(not p.exists()for p,_ in hidden)
 argv=['go','run',str(file),str(restore/'layer'),str(restore/'version'),str(restore/'layer/semantic-functional-proposed.sqlite'),prep['source']]
 record={'argv':argv,'driver_sha256':hashlib.sha256(file.read_bytes()).hexdigest(),'original_source_absent':not originals[0].exists(),'original_dataset_absent':not originals[1].exists(),'original_semantic_store_absent':not originals[2].exists(),'input_seed_SHA':hashlib.sha256((restore/'layer/claim-seeds.json').read_bytes()).hexdigest(),'source_logical_coordinate_preserved':prep['source']}
 r=subprocess.run(argv,capture_output=True);(out/'archive-replay.stdout.txt').write_bytes(r.stdout);(out/'archive-replay.stderr.txt').write_bytes(r.stderr);record['exit_code']=r.returncode
 if r.returncode==0:
  for name in ['projection-proposed.json','canonical-source-refs.json','relation-review-items.json','trace-diagnostic.json']:
   assert (out/name).read_bytes()==(restore/'layer'/name).read_bytes(),name
  original=json.loads((out/'projection-report.json').read_text());replayed=json.loads((restore/'layer/projection-report.json').read_text());assert original==replayed
  assert all(hashlib.sha256((restore/row['path']).read_bytes()).hexdigest()==row['sha256']for row in json.loads((out/'archive-inventory.json').read_text()))
  record.update({'projection_canonical_relations_trace_bytes_identical':True,'projection_report_decoded_identical':True,'every_archived_authoritative_file_SHA_unchanged':True,'actual_model_HTTP_calls':0,'actual_human_reviews':0,'actual_activations':0,'operating_portability_accepted':False})
 (out/'archive-replay.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record));print(r.stderr.decode()[:2000]);assert r.returncode==0
finally:
 for p,target in reversed(hidden):
  assert not p.exists()and target.exists();target.rename(p)
 shutil.rmtree(driver)
 assert all(p.exists()for p in originals)
