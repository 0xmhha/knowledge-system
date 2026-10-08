import json,subprocess
from pathlib import Path
p=Path(__file__).resolve().parent;prep=json.loads((p/'preparation.json').read_text());base=Path(prep['base']);version=json.loads((p/'archive.json').read_text())['restored_version']
paths=[Path(prep['source']),Path(prep['dataset_root']),base/'semantic-proposed.sqlite'];moved=[]
try:
 for old in paths:
  assert old.exists();hidden=old.with_name(old.name+'-hidden-for-replay');assert not hidden.exists();old.rename(hidden);moved.append((old,hidden))
 assert all(not x.exists()for x in paths)
 with(p/'archive-originals-hidden.stdout.txt').open('w')as out,(p/'archive-originals-hidden.stderr.txt').open('w')as err:r=subprocess.run(['go','run','./.n12-proposed-lock-driver','--replay',version],stdout=out,stderr=err)
 (p/'archive-originals-hidden.json').write_text(json.dumps({'exit_code':r.returncode,'original_source_absent':True,'original_dataset_absent':True,'original_semantic_store_absent':True,'restored_version':version,'logical_source_root_coordinate':prep['source'],'human_approved':0,'new_model_calls':0},indent=2)+'\n')
 assert r.returncode==0
finally:
 for old,hidden in reversed(moved):hidden.rename(old)
print('PASS independent archive with original source, dataset and semantic store temporarily absent; originals restored')
