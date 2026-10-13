from pathlib import Path
import json,yaml,subprocess,hashlib,collections,statistics,importlib.util,os,tempfile
root=Path.cwd();base=Path(Path('/private/tmp/ks-o03-accepted-base.txt').read_text());ef=root/'system/eval/b0-knowledge-system/ontology-o04-coordinate-reuse-2026-10-13';p=json.loads((ef/'protocol-frozen.json').read_text());sha=lambda path:hashlib.sha256(path.read_bytes()).hexdigest();psha=sha(ef/'protocol-frozen.json');cks=base/'bin/cks-coordinate-cache';cfg=yaml.safe_load((base/'stdio.yaml').read_text());prep=json.loads((base/'preparation.json').read_text());candidate=base/'dataset/v-approved-wbft-baca2869';identity=json.loads((candidate/'dataset-identity.json').read_text());out=base/'coordinate-reuse-2026-10-13';out.mkdir(exist_ok=False);dbpaths=[candidate/'graph/graph.db',candidate/'vector/vector.db',base/'semantic.sqlite'];before={str(f):sha(f)for f in dbpaths};spec=importlib.util.spec_from_file_location('base_metrics',root/'scripts/b1-summarize-matrix.py');M=importlib.util.module_from_spec(spec);spec.loader.exec_module(M)
def guard():
 assert sha(ef/'protocol-frozen.json')==psha and sha(cks)==p['binary_sha256'];assert all(sha(root/f)==h for f,h in p['changed_source_sha256'].items());assert sha(Path(prep['operator_config']))==prep['operator_config_sha256'];assert os.readlink(prep['operator_current'])==prep['operator_current_target'];assert not (base/'dataset/current').exists();assert subprocess.check_output(['git','-C',prep['original_source'],'rev-parse','HEAD'],text=True).strip()==prep['original_source_head']
guard();rows={};summaries=[];seq=0
for q in p['queries']:
 for block,order in enumerate(p['schedule'],1):
  normal=[]
  for arm in order:
   guard();seq+=1;d=out/q['id']/str(block)/arm;d.mkdir(parents=True);req={'schema_version':1,'requests':[{'id':q['id'],'tool':q['tool'],'arguments':q['arguments']}]};(d/'requests.json').write_text(json.dumps(req,ensure_ascii=False,indent=2)+'\n');c=yaml.safe_load(yaml.safe_dump(cfg));c['semantic']['ontology_mode']=p['arms'][arm];c['logging'].update(measure_backend_calls=True,footprint_dir=str(d/'events'),mode='prod',level='info');(d/'config.yaml').write_text(yaml.safe_dump(c,sort_keys=False));n=json.loads(json.dumps(c));n['semantic'].pop('ontology_mode');n['logging'].pop('footprint_dir');normal.append(n);argv=[str(cks),'eval','capture','--requests',str(d/'requests.json'),'--config',str(d/'config.yaml'),'--cks-mcp',str(cks),'--output',str(d/'capture.json'),'--warmup','1','--retrieval-runs','1','--warm-runs','5','--cold-runs','0','--call-timeout','60s'];r=subprocess.run(argv,capture_output=True,text=True,timeout=600);(d/'command.json').write_text(json.dumps({'argv':argv,'exit':r.returncode,'stdout':r.stdout,'stderr':r.stderr},indent=2)+'\n');assert (d/'capture.json').exists(),r.stderr;cap=json.loads((d/'capture.json').read_text());assert cap['request_sha256']==sha(d/'requests.json')and cap['config_sha256']==cap['config_sha256_after']==sha(d/'config.yaml')and cap['binary_sha256']==cap['binary_sha256_after']==p['binary_sha256'];assert cap['counts']==p['per_block_counts'];events=[M.decode(l)for l in (d/'events/cks-mcp.jsonl').read_bytes().splitlines()];scopes={};HTTP=collections.Counter();methods=collections.Counter();warm=[];failed=0;pending=0;states=collections.Counter();textcalls=[]
   for e in events:
    if e.get('event')=='measurement.backend_calls':s=e['summary'];assert s['measurement_id']not in scopes;scopes[s['measurement_id']]=s
   for row in cap['rows']:
    key=(q['id'],block,arm,row['phase'],row['iteration']);assert key not in rows;rows[key]=row;call=row['call'];assert call['arguments']==q['arguments'];scope=scopes[call['measurement_id']];pending+=scope['pending_calls'];failure=bool(row.get('error')or call.get('transport_error')or call.get('response',{}).get('isError'));failed+=failure
    if not failure:
     pack=call['response']['structuredContent'];M.verify_pack(pack,identity,base/'source');assert pack['query']==q['question'];ont=pack['metadata'].get('ontology',{});states[ont.get('state')]+=1;textcalls.append(ont.get('text_search_calls',0));assert not ont.get('applied_relations',0)
    if row['phase']=='warm_latency'and not failure:warm.append(call['elapsed_ns'])
    for call in scope['calls']:
     methods[call['backend']+'.'+call['method']]+=1
     if call['backend']=='ollama_http':HTTP[call.get('options',{}).get('path','unknown')]+=1
   summary={'sequence':seq,'id':q['id'],'block':block,'arm':arm,'planned':7,'observed':len(cap['rows']),'failed':failed,'missing':7-len(cap['rows']),'pending':pending,'warm_ns':warm,'HTTP_attempts':dict(HTTP),'backend_methods':dict(methods),'ontology_states':dict(states),'text_search_calls':textcalls};summaries.append(summary);(out/'progress.json').write_text(json.dumps(summaries,indent=2)+'\n');print(json.dumps({k:summary[k]for k in ['sequence','id','block','arm','observed','failed','missing']}),flush=True)
  assert normal[0]==normal[1]==normal[2]
comparisons=[]
with tempfile.TemporaryDirectory(prefix='ks-ontology-cost-pairs-')as temp:
 for q in p['queries']:
  for block in range(1,4):
   for phase,count in p['per_block_counts'].items():
    for i in range(1,count+1):
     baseline=rows[(q['id'],block,'baseline',phase,i)]['call']['response']['structuredContent'];(Path(temp)/'baseline.json').write_text(json.dumps(baseline,ensure_ascii=False))
     for arm in ['resolve_only','text']:
      on=rows[(q['id'],block,arm,phase,i)]['call']['response']['structuredContent'];(Path(temp)/'on.json').write_text(json.dumps(on,ensure_ascii=False));r=subprocess.run([str(cks),'semantic','compare-responses','--off',str(Path(temp)/'baseline.json'),'--on',str(Path(temp)/'on.json')],capture_output=True,text=True,timeout=30);assert r.returncode==0,r.stdout+r.stderr;comp=json.loads(r.stdout);comparisons.append({'id':q['id'],'block':block,'arm':arm,'phase':phase,'iteration':i,'comparison':comp});assert arm!='resolve_only'or (not comp['citation_order_changed']and not comp['body_outputs_changed'])
guard();assert all(sha(Path(f))==h for f,h in before.items());assert not any(s['failed']or s['missing']or s['pending']for s in summaries);aggregates={}
for q in p['queries']:
 a={}
 for arm in p['arms']:
  selected=[s for s in summaries if s['id']==q['id']and s['arm']==arm];ns=[n for s in selected for n in s['warm_ns']];http=collections.Counter();methods=collections.Counter()
  for s in selected:http.update(s['HTTP_attempts']);methods.update(s['backend_methods'])
  a[arm]={'warm_n':len(ns),'warm_p95_ns':M.percentile(ns,.95),'warm_median_ns':statistics.median(ns),'HTTP_attempts':dict(http),'backend_methods':dict(methods)}
 a['p95_ratios_to_baseline']={arm:a[arm]['warm_p95_ns']/a['baseline']['warm_p95_ns']for arm in ['resolve_only','text']};aggregates[q['id']]=a
result={'scope':p['scope'],'protocol_sha256':psha,'responses':len(rows),'verified_comparisons':len(comparisons),'failed_missing_pending':0,'aggregates':aggregates,'runs':summaries,'comparisons':comparisons,'original_HEAD_current_config_preserved':True,'held_DB_SHA_preserved':before,'new_FINAL_samples':0,'quality_evaluated':False,'full_task_complete':False};(out/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'diagnostic':'PASS','responses':len(rows),'comparisons':len(comparisons),'p95_ratios':{k:v['p95_ratios_to_baseline']for k,v in aggregates.items()}}),flush=True)
