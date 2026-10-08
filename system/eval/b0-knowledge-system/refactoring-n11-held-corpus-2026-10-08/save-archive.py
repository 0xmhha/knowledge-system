import gzip,hashlib,io,json,subprocess,tarfile
from pathlib import Path
P=Path(__file__).resolve().parent;prep=json.loads((P/'preparation.json').read_text());base=Path(prep['base']);v=Path(prep['dataset_root'])/'n11-candidate'
sha=lambda b:hashlib.sha256(b).hexdigest()
def save(n,x):(P/n).write_text(json.dumps(x,ensure_ascii=False,indent=2)+'\n')
identity=json.loads((v/'dataset-identity.json').read_text());graph=json.loads((v/'graph/manifest.json').read_text());vector=json.loads((v/'vector/manifest.json').read_text());filelist=json.loads((v/'files-from.json').read_text());source=json.loads((v/'sources/manifest.json').read_text());refs=json.loads((P/'source-rebinding.json').read_text())
expected={p for p in filelist['include']if p.endswith('.go')};assert expected=={f['path']for f in graph['files']}and len(expected)==59
expected_vec={p for p in filelist['include']if p.endswith(('.go','.md'))};assert expected_vec=={f['path']for f in vector['input_files']}and len(expected_vec)==63
original=json.loads((P/'original-corpus-inputs.json').read_text());assert len(original)==1416
for path,d in original.items():assert sha((Path(prep['source'])/path).read_bytes())==d
for f in graph['files']+vector['input_files']:assert f['sha256']==sha((Path(prep['source'])/f['path']).read_bytes())
assert not subprocess.check_output(['git','status','--porcelain'],cwd=prep['source'])
audit=json.loads((P/'whole-tree-audit.stdout.txt').read_text());assert not(audit.get('in_db_only')or[]);assert len(audit['in_build_only'])==992 and not set(audit['in_build_only'])&expected
save('index-scope-check.json',{'selected_files':len(filelist['include']),'CKG_Go_files':59,'CKV_Go_Markdown_files':63,'scope_bijection_and_source_SHA_match':True,'retained_files':len(source['files']),'original1416files_unchanged':True,'whole_tree_audit':{'exit_code':1,'build_files':1051,'db_files':59,'missing_outside_selected_scope':992,'extra':0},'JSON_YAML_retained_not_vectorized':[p for p in filelist['include']if p.endswith(('.json','.yaml'))],'full_repository_index_coverage_claimed':False})
for name in ['dataset-identity.json','review-hold.json','reader-protocol.json','files-from.json','sources/manifest.json','vector/manifest.json']:(P/('retained-'+name.replace('/','-'))).write_bytes((v/name).read_bytes())
with(P/'graph-manifest.json.gz').open('wb')as f:
 with gzip.GzipFile(fileobj=f,mode='wb',mtime=0,filename='')as g:g.write((v/'graph/manifest.json').read_bytes())
probe={'model':'bge-m3:latest','input':['dimension probe'],'truncate':False,'options':{'num_ctx':8192,'num_batch':8192}};psha=sha(json.dumps(probe,separators=(',',':')).encode());phases=[]
for prefix in ['initial-','']:
 obs=[json.loads(l)for l in(P/(prefix+'model-http-observations.jsonl')).read_text().splitlines()];emb=[r for r in obs if r['path']=='/api/embed'];probes=[r for r in emb if r['request_sha256']==psha];assert len(probes)==(1 if prefix else 2)and all(r['status']==200 and r['truncate']is False for r in emb)
 phases.append({'phase':'initial_filelist_preparation_error'if prefix else'corrected_held_build','process_exit_code':json.loads((P/(prefix+'setup-result.json')).read_text())['process_exit_code'],'http_attempts':len(obs),'embedding_HTTP':len(emb),'startup_probe_inputs':sum(r['embedding_inputs']for r in probes),'index_embedding_HTTP':len(emb)-len(probes),'index_embedding_inputs':sum(r['embedding_inputs']for r in emb if r not in probes),'failed_embedding_HTTP':0,'successful_embedding_outputs':sum(r['embedding_outputs']for r in emb),'prompt_eval_tokens':sum(r.get('prompt_eval_count')or 0 for r in emb),'model_total_duration_ns':sum(r.get('model_total_duration_ns')or 0 for r in emb)})
assert phases[0]['index_embedding_inputs']==0 and phases[1]['index_embedding_inputs']==785
save('model-call-accounting.json',{'phases':phases,'total_HTTP':sum(r['http_attempts']for r in phases),'total_embedding_HTTP':sum(r['embedding_HTTP']for r in phases),'startup_probe_inputs':3,'index_embedding_inputs':785,'stored_chunks':vector['chunk_count'],'total_embedding_inputs':788,'new_DEV_FINAL_queries':0,'measurement_scope':'preparation startup probes plus one completed index build; not retrieval quality/query cost or p95'})
assert sha((v/'vector/vector.db').read_bytes())==vector['db_sha256'];assert not(Path(prep['dataset_root'])/'current').exists()
files=[(v/n,n)for n in ['dataset-identity.json','files-from.json','review-hold.json','reader-protocol.json','graph/graph.db','graph/manifest.json','vector/vector.db','vector/manifest.json','vector/footprint.jsonl']]+[(f,f.relative_to(v).as_posix())for f in sorted((v/'sources').rglob('*'))if f.is_file()]
files +=[(base/'semantic-proposed.sqlite','semantic/proposed.sqlite'),(P/'cli-semantic-proposed.json','semantic/projection.json')]
for file,name in [('candidate-question-book.json','question-book.json'),('candidate-protocol.json','protocol.json'),('runtime-config-proposed.yaml','runtime-config-proposed.yaml'),('execution-binding-proposed.json','execution-binding-proposed.json')]:files.append((P/file,'review-inputs/'+name))
rows=[];archive=P/'held-candidate.tar.gz'
with archive.open('wb')as f:
 with gzip.GzipFile(fileobj=f,mode='wb',mtime=0,filename='')as z:
  with tarfile.open(fileobj=z,mode='w|')as t:
   for path,name in files:
    data=path.read_bytes();row={'path':name,'size':len(data),'sha256':sha(data),'mode':path.stat().st_mode&0o777};rows.append(row);info=tarfile.TarInfo(name);info.size=len(data);info.mode=row['mode'];info.mtime=0;t.addfile(info,io.BytesIO(data))
restore=base/'archive-only-candidate';restore.mkdir()
with tarfile.open(archive,'r:gz')as t:
 assert all(m.isfile()and not m.name.startswith('/')and '..'not in Path(m.name).parts for m in t.getmembers());t.extractall(restore,filter='data')
for row in rows:assert sha((restore/row['path']).read_bytes())==row['sha256']
save('archive-inventory.json',rows);save('archive.json',{'sha256':sha(archive.read_bytes()),'bytes':archive.stat().st_size,'archived_files':len(rows),'restored_version':str(restore),'contains_live_current_pointers':False,'review_inputs_separate_from_captured_indexed_corpus':True,'logical_absolute_config_source_coordinates_preserved':True,'excluded':'lifecycle locks, SQLite sidecars, initializer backup; authoritative vector DB SHA matched'})
print(json.dumps({'archive_bytes':archive.stat().st_size,'archive_files':len(rows),'source_refs':16,'actual_gold_index_overlap':14,'model_inputs':788,'activated':False}))
