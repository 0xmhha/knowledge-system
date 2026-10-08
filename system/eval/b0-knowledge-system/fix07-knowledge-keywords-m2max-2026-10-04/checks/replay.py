import json,hashlib,subprocess,time
from pathlib import Path
root=Path.cwd(); before=Path('/private/tmp/ks-b0-calls-final-real-20261004'); out=Path('/private/tmp/ks-fix07-real-20261004'); out.mkdir(exist_ok=False)
binary=root/'bin/cks'; old=json.loads((before/'summary.json').read_text()); locked=old['locked_files_sha256']
for p,h in locked.items(): assert hashlib.sha256(Path(p).read_bytes()).hexdigest()==h
report={'state':'running','quality_metrics':None,'base_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'source_hashes':{p:hashlib.sha256((root/p).read_bytes()).hexdigest() for p in ['internal/system/composer/stage1/candidates.go','internal/system/composer/stage1/extractor_knowledge_test.go']},'binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'locked_files_sha256':locked,'arm_rotation':False,'arms':[]}
(out/'summary.json').write_text(json.dumps(report,indent=2)+'\n')
for mode in ['baseline','concept_text','relations','combined']:
 for enabled in [False,True]:
  arm=mode+('_on' if enabled else '_off'); cfg=out/(arm+'.yaml'); cfg.write_text((before/(arm+'.yaml')).read_text().replace(str(before/'telemetry'),str(out/'telemetry')))
  cmd=[str(binary),'eval','capture','--requests',str(before/(arm+'-requests.json')),'--config',str(cfg),'--output',str(out/(arm+'.json')),'--warmup','0','--retrieval-runs','1','--warm-runs','0','--cold-runs','0']
  with (out/(arm+'.txt')).open('w') as log: code=subprocess.run(cmd,stdout=log,stderr=subprocess.STDOUT).returncode
  assert code==0,(arm,code)
  captured=json.loads((out/(arm+'.json')).read_text()); prior=json.loads((before/(arm+'.json')).read_text())
  assert captured['state']=='captured' and len(captured['rows'])==6
  events=[json.loads(l) for l in (out/'telemetry'/mode/'cks-mcp.jsonl').read_text().splitlines()]
  observations=[]
  for a,b in zip(captured['rows'],prior['rows']):
   assert a['request_id']==b['request_id'] and a['call']['response']==b['call']['response'],(arm,a['request_id'],'response changed')
   ident=a['call']['measurement_id']; matches=[e['summary'] for e in events if e.get('event')=='measurement.backend_calls' and e['summary']['measurement_id']==ident]
   assert len(matches)==1; s=matches[0]; assert s['outcome']=='returned' and s['pending_calls']==0
   failures=[c for c in s['calls'] if c['outcome']!='returned']; assert len(failures)==8 and all(c['backend']=='ckg' and c['method']=='neighbors' and c['outcome']=='backend_error' for c in failures)
   searches=[c for c in s['calls'] if c['method']=='bm25_search']; assert searches and all(c['outcome']=='returned' for c in searches)
   assert all(c.get('input_sha256')!='00222b511ec483f24303e24545cdd6897e58ff880a6f1bb372d9e7b461a56608' for c in searches)
   observations.append({'request_id':a['request_id'],'measurement_id':ident,'bm25_calls':len(searches),'bm25_errors':0,'neighbors_errors':len(failures),'sdk_response_identical_to_before':True})
  report['arms'].append({'arm':arm,'command':cmd,'exit_code':code,'capture_sha256':hashlib.sha256((out/(arm+'.json')).read_bytes()).hexdigest(),'observations':observations})
  (out/'summary.json').write_text(json.dumps(report,indent=2)+'\n'); print(arm,'6 unchanged responses, BM25 errors removed',flush=True)
for p,h in locked.items(): assert hashlib.sha256(Path(p).read_bytes()).hexdigest()==h
assert hashlib.sha256(binary.read_bytes()).hexdigest()==report['binary_sha256']
report.update(state='verified',requests=48,sdk_responses_unchanged=48,bm25_errors_before=96,bm25_errors_after=0,neighbors_errors_after=384,physical_inputs_unchanged=True)
(out/'summary.json').write_text(json.dumps(report,indent=2)+'\n')
