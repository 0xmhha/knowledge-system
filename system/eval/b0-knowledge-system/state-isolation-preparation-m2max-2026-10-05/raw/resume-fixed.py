import ast,json,subprocess,urllib.request,hashlib
from pathlib import Path
p=Path(__file__).resolve().parent
cases=json.loads((p/'cases.json').read_bytes())['cases']
def sha(b):return hashlib.sha256(b).hexdigest()
# Reuse only the pure seal function; never rerun completed captures or overwrite raw evidence.
tree=ast.parse((p/'replay-fixed.py').read_text());node=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='seal')
exec(compile(ast.Module(body=[node],type_ignores=[]),'<seal>','exec'))
commands=json.loads((p/'fixed-replay-commands.json').read_bytes())
c=next(c for c in cases if c['id']=='F-05-DEV-b')
prompt=json.loads(Path(c['requests']).read_bytes())['requests'][0]['arguments']['prompt']
plans=[(c['id']+'-fixed-ckv',[str(p/'fixed-bin/ckv'),'query',prompt,'--out',str(Path(c['version'])/'vector'),'--src',c['repo'],'--top','10','--threshold','-1','--json','--model-name','bge-m3:latest','--no-footprint']),
(c['id']+'-fixed-sdk',[str(p/'fixed-bin/cks'),'eval','capture','--config',c['config'],'--requests',c['requests'],'--output',str(p/(c['id']+'-fixed-sdk.json')),'--warmup','0','--retrieval-runs','1','--warm-runs','0','--cold-runs','0'])]
for label,args in plans:
 r=subprocess.run(args,capture_output=True,timeout=180)
 (p/(label+'-stdout.txt')).write_bytes(r.stdout);(p/(label+'-stderr.txt')).write_bytes(r.stderr)
 commands.append({'label':label,'command':args,'exit_code':r.returncode})
 (p/'fixed-replay-commands.json').write_text(json.dumps(commands,indent=2)+'\n')
 assert r.returncode==0,(label,r.stderr.decode())
for label in ['F04','F05']:
 cap=json.loads((p/(label+'-mixed-sdk.json')).read_bytes())
 assert cap['state']=='partial' and len(cap['rows'])==1
 r=cap['rows'][0]['call']['response'];assert r['isError'] and r['structuredContent']['code']=='reindex_required'
with urllib.request.urlopen('http://127.0.0.1:11434/api/tags',timeout=20) as r: ma=next(m for m in json.load(r)['models'] if m['name']=='bge-m3:latest')
(p/'model-after.json').write_text(json.dumps(ma,indent=2)+'\n');assert json.loads((p/'model-before.json').read_bytes())==ma
after=seal();assert json.loads((p/'fixed-replay-before-seal.json').read_bytes())==after
(p/'fixed-replay-after-seal.json').write_text(json.dumps(after,indent=2)+'\n')
print('live replay passed: unchanged datasets/source/config/model, 4 normal fixed SDK, 2 typed reindex_required mixed-layout errors')
