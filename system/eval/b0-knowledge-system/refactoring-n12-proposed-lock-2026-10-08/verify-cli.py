import json,subprocess,urllib.request
from pathlib import Path
p=Path(__file__).resolve().parent;q=json.loads((p/'preparation.json').read_text());v=Path(q['dataset_root'])/'n12-candidate';cli=Path(q['base'])/'bin/cks';identity=json.loads((v/'dataset-identity.json').read_text())
for suffix in ['command.json','stdout.txt','stderr.txt']:
 f=p/('semantic-include-packs.'+suffix)if suffix!='command.json'else p/'semantic-include-packs-command.json'
 if f.exists()and not(p/('initial-'+f.name)).exists():f.rename(p/('initial-'+f.name))
(p/'initial-cli-argument-error.json').write_text(json.dumps({'stage':'semantic-include-packs','exit_code':1,'cause':'preparation omitted required project-id, dataset-id and store arguments','product_failure':False,'negative_control_counted':False})+'\n')
args=['semantic','build','--repo',q['source'],'--project-id','knowledge-system','--dataset-id',identity['dataset_id'],'--store',str(Path(q['base'])/'unused-extract-only.sqlite'),'--graph',str(v/'graph'),'--vector',str(v/'vector'),'--version-dir',str(v),'--ontology','docs/spec-driven/ontology-pilot.yaml','--spec','docs/spec-driven/spec-pilot.yaml','--include-packs','--min-canonical-ratio','1','--extract-only','--out',str(p/'cli-projection-proposed.json')]
(p/'semantic-include-packs-command.json').write_text(json.dumps([str(cli)]+args,indent=2)+'\n')
with(p/'semantic-include-packs.stdout.txt').open('w')as out,(p/'semantic-include-packs.stderr.txt').open('w')as err:r=subprocess.run([str(cli)]+args,stdout=out,stderr=err)
assert r.returncode==0,r.returncode
assert not(Path(q['base'])/'unused-extract-only.sqlite').exists()
m={}
for k in ['tags','version']:
 with urllib.request.urlopen('http://127.0.0.1:11434/api/'+k)as r:m[k]=json.load(r)
assert m==json.loads((p/'model-metadata-before.json').read_text());(p/'model-metadata-after.json').write_text(json.dumps(m,indent=2)+'\n')
print('PASS production --include-packs extract-only, canonical floor1.0, no SQLite creation, local model metadata unchanged')
