import hashlib,importlib.util,json
from collections import Counter
from pathlib import Path
root=Path.cwd();spec=importlib.util.spec_from_file_location('h',root/'scripts/wbs-ontology-relations-smoke.py');h=importlib.util.module_from_spec(spec);spec.loader.exec_module(h)
for kind,output,original,k in [('real','system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04/real','system/eval/b0-knowledge-system/recall-k-m2max-2026-10-04/real',10),('mock','system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04/mock','system/eval/b0-knowledge-system/backend-calls-m2max-2026-10-04/mock',20)]:
 p=Path(output);old=Path(original);report=json.loads((p/'report.json').read_text());rows=[json.loads(l) for l in (p/'rows.jsonl').read_text().splitlines()];assert report['state']=='captured' and report['rows']==len(rows)==288
 assert report['locked_files_sha256']==report['locked_files_sha256_after'] and report['binary_sha256']==report['binary_sha256_after']
 assert report['source_head']==report['source_head_after'] and report['live_executable_bits']==report['live_executable_bits_after']
 assert hashlib.sha256((p/'rows.jsonl').read_bytes()).hexdigest()==report['rows_sha256']
 for path,sha in report['locked_files_sha256'].items():
  if sha=='missing':assert not Path(path).exists()
  else:assert hashlib.sha256(Path(path).read_bytes()).hexdigest()==sha,path
 for arm in report['arms']:assert hashlib.sha256((p / Path(arm['config']).name).read_bytes()).hexdigest()==arm['config_sha256']==arm['config_sha256_after']
 summary=json.loads((old/'summary.json').read_text());oldresponses={};events={}
 for arm in report['arms']:
  captured=json.loads((old/(arm['id']+'.json')).read_text());oldresponses[arm['id']]={r['request_id']:r['call']['response'] for r in captured['rows']}
  events[arm['id']]=[json.loads(l) for l in (p/'footprints'/arm['id']/'cks-mcp.jsonl').read_text().splitlines()]
 ids=set();counts=Counter();errors=Counter();startup=0;groups={}
 for row in rows:
  i=row['sequence']-1;assert row['group']==i//8 and row['position']==i%8+1 and row['arm']==report['arms'][(row['group']+row['position']-1)%8]['id']
  counts[row['phase']]+=1;groups.setdefault(row['group'],[]).append(row)
  assert not row.get('error');assert row['call']['response']==oldresponses[row['arm']][row['request_id']],(kind,row['arm'],row['request_id'],'SDK response changed')
  cold=row['phase']=='cold_process';assert (row.get('process_start_through_response_ns',0)>0)==cold
  if cold:assert row['process_start_through_response_ns']>=row['call']['elapsed_ns']
  pack=row['call']['response']['structuredContent'];copy=json.loads(json.dumps(pack));expect=copy['metadata'].pop('integrity_hash');assert hashlib.sha256(json.dumps(copy,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode()).hexdigest()==expect
  h.verify_v2_sources(pack,old/'source',summary['coordinates']);h.verify_knowledge(pack,row['arm'].endswith('_on'),row['request_id'])
  ident=row['call']['measurement_id'];assert ident not in ids;ids.add(ident)
  found=[e for e in events[row['arm']] if e.get('event')=='measurement.backend_calls' and e['summary']['measurement_id']==ident];assert len(found)==1 and found[0]['trace_id']==ident
  s=found[0]['summary'];assert s['outcome']=='returned' and s['pending_calls']==0;calls=s['calls'];assert [c['ordinal'] for c in calls]==list(range(1,len(calls)+1))
  searches=[c for c in calls if c['method']=='semantic_search'];raw=[c for c in searches if not c['options']['Filter']['ChunkKinds']];knowledge=[c for c in searches if c['options']['Filter']['ChunkKinds']]
  assert raw and all(c['options']['K']==k and c['options']['BM25Rerank'] for c in raw);assert len(knowledge)==1 and knowledge[0]['options']['K']==6
  assert all(c['outcome']=='returned' for c in calls if c['method']=='bm25_search')
  errors.update(c['backend']+'.'+c['method'] for c in calls if c['outcome']!='returned')
  http=[c for c in calls if c['backend']=='ollama_http']
  if kind=='real':assert http and all(c['outcome']=='returned' and c['http_status']==200 for c in http)
  else:assert not http
 for group,rs in groups.items():
  assert len(rs)==8 and len({r['arm'] for r in rs})==8 and len({(r['request_id'],r['phase'],r['iteration']) for r in rs})==1
  normalized=[{a:b for a,b in r['call']['arguments'].items() if a!='include_knowledge'} for r in rs];assert all(a==normalized[0] for a in normalized)
 assert counts=={'warmup':48,'retrieval':96,'warm_latency':96,'cold_process':48}
 for arm in report['arms']:
  anchors=[e['summary'] for e in events[arm['id']] if e.get('event')=='measurement.backend_calls' and e['summary']['tool']=='startup.intent_anchors'];assert len(anchors)==7 and all(s['outcome']=='returned' for s in anchors);startup+=len(anchors)
 print(kind,': 288 source/integrity/policy-valid unchanged SDK responses, 36 rotated groups, 56 separate startup scopes; raw/text K',k,'knowledge K6, original DB/input/HEAD/modes preserved; internal errors',dict(errors))
p=Path('system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04/init-failure');r=json.loads((p/'report.json').read_text());rows=[json.loads(l) for l in (p/'rows.jsonl').read_text().splitlines()];assert r['state']=='partial' and len(rows)==96 and all(row['error'] for row in rows)
assert r['locked_files_sha256']==r['locked_files_sha256_after'] and any('deadline' in e for e in r['errors'])
assert all(not row['call'].get('response') for row in rows)
print('actual SDK initialization deadline: 96 failure rows preserved, no fabricated responses')
