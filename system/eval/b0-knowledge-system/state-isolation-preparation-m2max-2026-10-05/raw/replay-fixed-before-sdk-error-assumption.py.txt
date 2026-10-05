import hashlib,json,subprocess,urllib.request
from pathlib import Path
p=Path(__file__).resolve().parent
cases=json.loads((p/'cases.json').read_bytes())['cases']
def sha(b): return hashlib.sha256(b).hexdigest()
def seal():
 result={}
 for c in cases:
  for f in Path(c['version']).rglob('*'):
   if f.is_file():
    if f.name.endswith('-wal') and f.stat().st_size: raise ValueError('uncheckpointed WAL '+str(f))
    if not f.name.endswith(('-wal','-shm')):result[str(f)]=sha(f.read_bytes())
  for f in Path(c['repo']).rglob('*'):
   if f.is_file() and '.git' not in f.parts:result[str(f)]=sha(f.read_bytes())
  for k in ('config','requests'):result[c[k]]=sha(Path(c[k]).read_bytes())
 return result
before=seal(); (p/'fixed-replay-before-seal.json').write_text(json.dumps(before,indent=2)+'\n')
def model():
 with urllib.request.urlopen('http://127.0.0.1:11434/api/tags',timeout=20) as r: d=json.load(r)
 m=next(m for m in d['models'] if m['name']=='bge-m3:latest')
 assert m['digest']=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
 return m
mb=model();(p/'model-before.json').write_text(json.dumps(mb,indent=2)+'\n')
commands=[]
def run(label,args,expected=0):
 r=subprocess.run(args,capture_output=True,timeout=180)
 (p/(label+'-stdout.txt')).write_bytes(r.stdout);(p/(label+'-stderr.txt')).write_bytes(r.stderr)
 commands.append({'label':label,'command':args,'exit_code':r.returncode})
 (p/'fixed-replay-commands.json').write_text(json.dumps(commands,indent=2)+'\n')
 if r.returncode!=expected:raise ValueError((label,r.returncode,r.stderr.decode()))
 return r
old=json.loads((p/'stale-query-command.json').read_bytes())['command'];old[0]=str(p/'fixed-bin/ckv')
a=run('old-index-new-source-fixed',old)
before_response=json.loads((p/'old-index-new-source-stdout.json').read_bytes());after_response=json.loads(a.stdout)
assert before_response['metadata']['fresh'] is True and after_response['metadata']['fresh'] is False
assert all(h['stale_citation'] for h in after_response['hits'])
normalized_before=json.loads(json.dumps(before_response));normalized_after=json.loads(json.dumps(after_response))
for d in (normalized_before,normalized_after):
 d['metadata'].pop('fresh');d['metadata'].pop('trace_id',None)
assert normalized_before==normalized_after
(p/'fix18-live-comparison.json').write_text(json.dumps({'before_fresh':True,'after_fresh':False,'stale_hits':len(after_response['hits']),'all_other_response_fields_equal_except_trace_id':True,'comparison_excluded_fields':['metadata.fresh','metadata.trace_id'],'quality_metrics':None},indent=2)+'\n')
for c in cases:
 prompt=json.loads(Path(c['requests']).read_bytes())['requests'][0]['arguments']['prompt']
 run(c['id']+'-fixed-ckv',[str(p/'fixed-bin/ckv'),'query',prompt,'--out',str(Path(c['version'])/'vector'),'--src',c['repo'],'--top','10','--threshold','-1','--json','--model-name','bge-m3:latest','--no-footprint'])
 run(c['id']+'-fixed-sdk',[str(p/'fixed-bin/cks'),'eval','capture','--config',c['config'],'--requests',c['requests'],'--output',str(p/(c['id']+'-fixed-sdk.json')),'--warmup','0','--retrieval-runs','1','--warm-runs','0','--cold-runs','0'])
 if c['id'].endswith('-a'):
  other=next(x for x in cases if x['id']=='F-05-DEV-b')
  cfg=Path(c['config']).read_text().replace(str(Path(c['version'])/'graph/graph.db'),str(Path(other['version'])/'graph/graph.db'))
  (p/'F05-mixed.yaml').write_text(cfg)
  run('F05-mixed',[str(p/'fixed-bin/cks'),'eval','capture','--config',str(p/'F05-mixed.yaml'),'--requests',c['requests'],'--output',str(p/'F05-mixed-sdk.json'),'--warmup','0','--retrieval-runs','1','--warm-runs','0','--cold-runs','0'],1)
  assert not (p/'F05-mixed-sdk.json').exists()
ma=model();(p/'model-after.json').write_text(json.dumps(ma,indent=2)+'\n');assert mb==ma
after=seal();assert before==after
(p/'fixed-replay-after-seal.json').write_text(json.dumps(after,indent=2)+'\n')
print('fixed live CKV + 4 SDK replay: passed; dataset/source/model/config unchanged')
