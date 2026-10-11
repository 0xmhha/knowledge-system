import json,sys,urllib.request
from pathlib import Path
p=Path(__file__).resolve().parent;phase=sys.argv[1];m={}
for key in ['tags','version']:
 with urllib.request.urlopen('http://127.0.0.1:11434/api/'+key)as r:m[key]=json.load(r)
assert next(x['digest']for x in m['tags']['models']if x['name']=='bge-m3:latest')=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
assert m['version']['version']=='0.35.1'
if phase=='after':assert m==json.loads((p/'model-metadata-before.json').read_text())
(p/('model-metadata-'+phase+'.json')).write_text(json.dumps(m,indent=2)+'\n')
print('local approved model metadata verified',phase)
