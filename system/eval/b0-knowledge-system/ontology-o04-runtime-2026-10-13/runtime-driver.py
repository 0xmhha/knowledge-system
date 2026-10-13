from pathlib import Path
import json,subprocess,hashlib,yaml,statistics,collections,tempfile
b=Path(Path('/private/tmp/ks-o04-runtime-base.txt').read_text());cks=Path('/private/tmp/ks-o04-runtime-cks');sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest();before=sha(cks)
p=b/'off-capture-command.txt'
if p.exists() and not (b/'pre-whitelist-capture-command.txt').exists(): (b/'pre-whitelist-capture-command.txt').write_bytes(p.read_bytes())
plan=json.loads((b/'requests.json').read_text());expected={r['id']:r for r in plan['requests']};captures={};configs={};summary={};keys={}
for arm in ['off','on']:
 argv=[str(cks),'eval','capture','--requests',str(b/'requests.json'),'--config',str(b/f'{arm}.yaml'),'--cks-mcp',str(cks),'--output',str(b/f'{arm}-capture.json'),'--warmup','1','--retrieval-runs','1','--warm-runs','2','--cold-runs','1','--call-timeout','30s']
 r=subprocess.run(argv,capture_output=True,text=True,timeout=180);(b/f'{arm}-capture-command.txt').write_text('argv='+repr(argv)+'\nexit='+str(r.returncode)+'\n'+r.stdout+r.stderr);assert r.returncode==0,(r.stdout+r.stderr)[-800:]
 cap=json.loads((b/f'{arm}-capture.json').read_text());captures[arm]=cap;c=yaml.safe_load((b/f'{arm}.yaml').read_text());configs[arm]=c
 assert cap['schema_version']==1 and cap['request_sha256']==sha(b/'requests.json') and cap['config_sha256']==cap['config_sha256_after']==sha(b/f'{arm}.yaml') and cap['binary_sha256']==cap['binary_sha256_after']==before and cap['quality_metrics'] is None
 assert cap['counts']=={'retrieval':1,'warmup':1,'warm_latency':2,'cold_process':1} and not cap.get('errors'),cap.get('errors')
 events=[json.loads(x)for x in (b/arm/'events/cks-mcp.jsonl').read_text().splitlines()];scopes={}
 for e in events:
  if e.get('event')=='measurement.backend_calls':
   s=e['summary'];assert s['measurement_id'] not in scopes;scopes[s['measurement_id']]=s
 result={'planned_calls':10,'observed_calls':len(cap['rows']),'failed_calls':0,'missing_calls':10-len(cap['rows']),'scoped_http_embedding_attempts':0,'scoped_calls':collections.Counter(),'pending_calls':0,'phases':{}}
 keys[arm]={}
 for row in cap['rows']:
  req=expected[row['request_id']];call=row['call'];assert call['tool']==req['tool'] and call['arguments']==req['arguments'];key=(row['request_id'],row['phase'],row['iteration']);assert key not in keys[arm];keys[arm][key]=row
  mid=call['measurement_id'];assert mid in scopes;scope=scopes[mid];assert scope['tool']==req['tool'];result['pending_calls']+=scope['pending_calls']
  fail=bool(row.get('error') or call.get('transport_error') or call.get('response',{}).get('isError'));result['failed_calls']+=fail
  for x in scope['calls']:
   result['scoped_calls'][x['backend']+'.'+x['method']]+=1
   result['scoped_http_embedding_attempts']+=x['backend']=='ollama_http'
  assert call['elapsed_ns']>=0;result['phases'].setdefault(row['phase'],[]).append(call['elapsed_ns'])
 result['scoped_calls']=dict(result['scoped_calls']);result['phases']={p:{'calls':len(v),'median_ns':statistics.median(v),'min_ns':min(v),'max_ns':max(v),'interpretation':'descriptive controlled fixture; no performance acceptance'}for p,v in result['phases'].items()};result['startup_scopes']=[{'tool':s['tool'],'calls':len(s['calls']),'pending_calls':s['pending_calls']}for s in scopes.values()if s['tool']=='startup.intent_anchors'];summary[arm]=result
 assert result['observed_calls']==10 and result['failed_calls']==result['missing_calls']==result['pending_calls']==0,result
# Both modes preserve raw-query-first; all other runtime settings agree except
# the destination of measurements. A default blank off mode would change Stage1.
assert configs['off']['semantic']['ontology_mode']=='baseline' and configs['on']['semantic']['ontology_mode']=='relations'
for c in configs.values(): c['semantic'].pop('ontology_mode');c['logging'].pop('footprint_dir')
assert configs['off']==configs['on'];assert keys['off'].keys()==keys['on'].keys()
pairs=[]
with tempfile.TemporaryDirectory(prefix='ks-o04-pack-compare-')as tmp:
 for key in sorted(keys['off']):
  for arm in ['off','on']:
   p=Path(tmp)/f'{arm}.json';p.write_text(json.dumps(keys[arm][key]['call']['response']['structuredContent']))
  r=subprocess.run([str(cks),'semantic','compare-responses','--off',str(Path(tmp)/'off.json'),'--on',str(Path(tmp)/'on.json')],capture_output=True,text=True,timeout=30);assert r.returncode==0,r.stderr+r.stdout
  comparison=json.loads(r.stdout);assert not comparison['quality_evaluated'];pairs.append({'request_id':key[0],'phase':key[1],'iteration':key[2],'comparison':comparison})
assert sha(cks)==before
prepared=json.loads((b/'preparation.json').read_text());assert all(sha(Path(p))==v for p,v in prepared['operator_config_sha_before'].items())
old=Path(Path('/private/tmp/ks-o03-base.txt').read_text().strip());import os
assert os.readlink(old/'dataset/current')==prepared['old_current']
report={'scope':'actual stdio processes on controlled owned mock fixture, not actual go-stablenet domain evaluation','binary_sha256':before,'same_binary_config_model_requests_verified':True,'search_strategy':'baseline vs relations both raw-query-first; ontology_mode and footprint destination only differ','captured_arm_order':['off','on'],'rows':summary,'response_pairs':pairs,'quality_metrics':None,'quality_evaluated':False,'performance_accepted':False,'independent_units':None,'owned_request_units':2,'new_FINAL_samples':0,'real_model_used':False,'real_domain_review_supplied':False,'current_and_operator_config_preserved':True,'limits':['2 fixture questions and repeated calls are not independent quality samples','sequential arm order is recorded; no order-randomized domain trial','fresh process timing is not machine/model/cache cold','mock feature hashes do not measure semantic quality','O04-B actual domain binary/model/runtime protocol and C/D remain pending']};(b/'runtime-verification.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'runtime':'PASS','arms':2,'responses':20,'verified_pairs':len(pairs),'quality_evaluated':False,'current_preserved':True}))
