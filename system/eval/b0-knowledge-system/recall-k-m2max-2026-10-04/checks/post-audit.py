from pathlib import Path
import hashlib,importlib.util,json
root=Path.cwd();base=root/'system/eval/b0-knowledge-system/recall-k-m2max-2026-10-04'
spec=importlib.util.spec_from_file_location('harness',root/'scripts/wbs-ontology-relations-smoke.py');h=importlib.util.module_from_spec(spec);spec.loader.exec_module(h)
ids=set();v2=0;requests=0
for kind,k in [('real',10),('compat',20)]:
 p=base/kind;summary=json.loads((p/'summary.json').read_text());m=json.loads((p/'matrix-manifest.json').read_text())
 assert m['settings']['raw_recall_k']==k and m['settings']['knowledge_pass_k']==6
 for f,sha in summary['input_file_sha256'].items(): assert hashlib.sha256((p/'source'/f).read_bytes()).hexdigest()==sha
 for f,sha in summary['locked_files_sha256'].items(): assert hashlib.sha256(Path(f).read_bytes()).hexdigest()==sha
 for arm in m['arms']:
  f=p/arm['capture']; assert hashlib.sha256(f.read_bytes()).hexdigest()==arm['capture_sha256'];r=json.loads(f.read_text());assert r['state']=='captured'
  assert r['binary_sha256']==r['binary_sha256_after']==summary['binary_sha256']
  events=[json.loads(l) for l in (p/'telemetry'/arm['mode']/'cks-mcp.jsonl').read_text().splitlines()]
  for row in r['rows']:
   ident=row['call']['measurement_id'];assert ident not in ids;ids.add(ident)
   found=[e for e in events if e.get('event')=='measurement.backend_calls' and e['summary']['measurement_id']==ident];assert len(found)==1 and found[0]['trace_id']==ident
   s=found[0]['summary'];assert s['outcome']=='returned' and s['pending_calls']==0
   searches=[c for c in s['calls'] if c['method']=='semantic_search'];raw=[c for c in searches if not c['options']['Filter']['ChunkKinds']];knowledge=[c for c in searches if c['options']['Filter']['ChunkKinds']]
   assert raw and all(c['options']['K']==k and c['options']['BM25Rerank'] for c in raw)
   assert len(knowledge)==1 and knowledge[0]['options']['K']==6
   assert all(c['outcome']=='returned' for c in s['calls'] if c['method']=='bm25_search')
   requests+=1;response=row['call']['response'];assert not response.get('isError')
   if kind=='compat' and row['request_id']=='v1':continue
   pack=response['structuredContent'];copy=json.loads(json.dumps(pack));expect=copy['metadata'].pop('integrity_hash');assert hashlib.sha256(json.dumps(copy,ensure_ascii=False,sort_keys=True,separators=(',',':')).encode()).hexdigest()==expect
   h.verify_v2_sources(pack,p/'source',summary['coordinates'])
   if kind=='real':h.verify_knowledge(pack,arm['include_knowledge'],row['request_id'])
   v2+=1
 print(kind,'actual raw/text K',k,'and knowledge K6 verified; preserved original source/DB bytes')
print('independent v2 source/integrity audit',v2,'correlated requests',requests)
