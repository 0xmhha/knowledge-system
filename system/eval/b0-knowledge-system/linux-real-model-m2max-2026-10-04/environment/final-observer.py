import urllib.request,json
from pathlib import Path
root=Path('/out');observations={}
for endpoint in ['version','tags','ps']:
 with urllib.request.urlopen('http://127.0.0.1:11434/api/'+endpoint,timeout=30) as r:raw=r.read()
 (root/('final-'+endpoint+'-response.json')).write_bytes(raw);observations[endpoint]=json.loads(raw)
assert observations['version']['version']=='0.35.1'
for endpoint in ['tags','ps']:
 models=observations[endpoint]['models'];assert len(models)==1 and models[0]['digest']=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
assert observations['ps']['models'][0]['size_vram']==0
print('Final server version/tag/residency pin verified; CPU only')
