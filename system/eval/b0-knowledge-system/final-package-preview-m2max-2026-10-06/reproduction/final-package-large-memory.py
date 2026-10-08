import json,subprocess,time,hashlib
from pathlib import Path
R=Path('/private/tmp/ks-final-package-large-cost-20261006');BIN=Path('/private/tmp/ks-final-package-darwin-20261006/unpacked/knowledge-system-darwin-arm64-10cbffaa104e-dirty-preview/cks');sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
config=R/'memory-config.yaml';config.write_text((R/'config.yaml').read_text().replace(str(R/'footprints'),str(R/'memory-footprints')))
cmd=[str(BIN),'eval','capture','--requests',str(R/'requests.json'),'--config',str(config),'--output',str(R/'memory-capture.json'),'--warmup','0','--retrieval-runs','1','--warm-runs','0','--cold-runs','0','--call-timeout','90s']
start=time.monotonic();before={str(p):sha(p) for p in [BIN,config,R/'requests.json']};samples=[]
with (R/'memory.stdout.txt').open('wb') as out,(R/'memory.stderr.txt').open('wb') as err:
 proc=subprocess.Popen(cmd,stdout=out,stderr=err)
 while proc.poll() is None:
  raw=subprocess.check_output(['ps','-axo','pid=,ppid=,rss=,comm='],text=True);allrows=[]
  for line in raw.splitlines():
   parts=line.strip().split(None,3)
   if len(parts)==4:allrows.append({'pid':int(parts[0]),'ppid':int(parts[1]),'rss_kib':int(parts[2]),'command':parts[3]})
  included={proc.pid};changed=True
  while changed:
   prior=set(included);included.update(x['pid'] for x in allrows if x['ppid'] in included);changed=prior!=included
  samples.append({'elapsed_seconds':time.monotonic()-start,'processes':[x for x in allrows if x['pid'] in included]});time.sleep(.25)
after={str(p):sha(p) for p in [BIN,config,R/'requests.json']};assert before==after
by_pid={}
for sample in samples:
 for row in sample['processes']:
  item=by_pid.setdefault(str(row['pid']),{'command':row['command'],'ppid':row['ppid'],'observations':0,'max_observed_rss_kib':0})
  item['observations']+=1;item['max_observed_rss_kib']=max(item['max_observed_rss_kib'],row['rss_kib'])
(R/'memory-samples.json').write_text(json.dumps(samples,indent=2)+'\n')
result={'diagnostic_only':True,'quality_metrics':None,'command':cmd,'exit_code':proc.returncode,'wall_seconds':time.monotonic()-start,'bindings_before':before,'bindings_after':after,'processes':by_pid,'sample_count':len(samples),'max_observed_sum_rss_kib':max(sum(p['rss_kib'] for p in s['processes']) for s in samples),'limitations':['sampled RSS; peaks between samples may be missed','sum RSS can double count shared pages','model daemon/runner excluded; model already resident; sampler overhead not latency baseline']}
(R/'memory-summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,indent=2))
assert proc.returncode==0
