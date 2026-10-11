import gzip,hashlib,io,json,subprocess,tarfile
from pathlib import Path
P=Path(__file__).resolve().parent;prep=json.loads((P/'preparation.json').read_text());base=Path(prep['base']);v=Path(prep['dataset_root'])/'n12-candidate'
sha=lambda b:hashlib.sha256(b).hexdigest()
def save(n,x):(P/n).write_text(json.dumps(x,ensure_ascii=False,indent=2)+'\n')
identity=json.loads((v/'dataset-identity.json').read_text());graph=json.loads((v/'graph/manifest.json').read_text());vector=json.loads((v/'vector/manifest.json').read_text());filelist=json.loads((v/'files-from.json').read_text())
expected={p for p in filelist['include']if p.endswith('.go')};actual={f['path']for f in graph['files']};assert expected==actual and len(actual)==53
vecpaths={f['path']for f in vector['input_files']};expected_vec={p for p in filelist['include']if p.endswith(('.go','.md'))};assert vecpaths==expected_vec and len(vecpaths)==56
original=json.loads((P/'overlay-provenance.json').read_text())['original_source_sha256'];assert len(original)==1410
for path,d in original.items():assert sha((Path(prep['source'])/path).read_bytes())==d
assert all(f['sha256']==original[f['path']]for f in graph['files']);assert all(f['sha256']==original[f['path']]for f in vector['input_files'])
assert not subprocess.check_output(['git','status','--porcelain'],cwd=prep['source'])
audit=json.loads((P/'whole-tree-audit.stdout.txt').read_text());assert not(audit.get('in_db_only')or[]);assert len(audit['in_build_only'])==998;assert not set(audit['in_build_only'])&expected
save('index-scope-check.json',{'filelist_files':len(filelist['include']),'selected_Go_files':53,'actual_CKG_Go_files':53,'actual_CKV_input_files':56,'code_and_markdown_scope_bijection':True,'source_sha_bindings_match':True,'original_source1410_unchanged':True,'whole_tree_audit':{'process_exit_code':1,'build_count':1051,'db_count':53,'in_both':53,'in_build_only':998,'in_db_only':0,'all_missing_files_outside_selected_Go_scope':True},'full_repository_index_coverage_claimed':False,'typed_overlay_vectorized':False})
for name in ['dataset-identity.json','review-hold.json','reader-protocol.json','files-from.json','sources/manifest.json','vector/manifest.json']:(P/('retained-'+name.replace('/','-'))).write_bytes((v/name).read_bytes())
with(P/'graph-manifest.json.gz').open('wb')as f:
 with gzip.GzipFile(fileobj=f,mode='wb',mtime=0,filename='')as g:g.write((v/'graph/manifest.json').read_bytes())
probe={'model':'bge-m3:latest','input':['dimension probe'],'truncate':False,'options':{'num_ctx':8192,'num_batch':8192}};probe_sha=sha(json.dumps(probe,separators=(',',':')).encode())
obs=[json.loads(l)for l in(P/'model-http-observations.jsonl').read_text().splitlines()];embeds=[r for r in obs if r['path']=='/api/embed'];probes=[r for r in embeds if r['request_sha256']==probe_sha];assert len(probes)==2
assert len(embeds)==62 and sum(r['embedding_inputs']for r in embeds)==668 and all(r['status']==200 and r['truncate']is False for r in embeds)
save('model-call-accounting.json',{'http_attempts':len(obs),'embedding_http_attempts':62,'startup_dimension_probe_http_calls':2,'startup_dimension_probe_inputs':2,'index_embedding_http_calls':60,'index_embedding_inputs':666,'embedding_failed_http_attempts':0,'successful_embedding_outputs_including_probes':668,'stored_chunks':vector['chunk_count'],'prompt_eval_tokens':sum(r.get('prompt_eval_count')or 0 for r in embeds),'model_total_duration_ns':sum(r.get('model_total_duration_ns')or 0 for r in embeds),'new_DEV_FINAL_queries':0,'measurement_scope':'one new local held index build; previous124 embed calls remain separate historical evidence; not query p95 or quality evaluation'})
assert sha((v/'vector/vector.db').read_bytes())==vector['db_sha256']
for side in ['vector/vector.db-wal','vector/vector.db-shm']:
 if(v/side).exists():assert(v/side).stat().st_size==0 or side.endswith('-shm')
assert not(Path(prep['dataset_root'])/'current').exists()
files=[v/n for n in ['dataset-identity.json','files-from.json','review-hold.json','reader-protocol.json','graph/graph.db','graph/manifest.json','vector/vector.db','vector/manifest.json','vector/footprint.jsonl']]+sorted((v/'sources').rglob('*'))
files=[f for f in files if f.is_file()];rows=[];archive=P/'held-candidate.tar.gz'
with archive.open('wb')as f:
 with gzip.GzipFile(fileobj=f,mode='wb',mtime=0,filename='')as z:
  with tarfile.open(fileobj=z,mode='w|')as t:
   for path in files:
    data=path.read_bytes();name=path.relative_to(v).as_posix();row={'path':name,'size':len(data),'sha256':sha(data),'mode':path.stat().st_mode&0o777};rows.append(row);info=tarfile.TarInfo(name);info.size=len(data);info.mode=row['mode'];info.mtime=0;t.addfile(info,io.BytesIO(data))
restore=base/'archive-only-candidate';restore.mkdir()
with tarfile.open(archive,'r:gz')as t:
 assert all(m.isfile()and not m.name.startswith('/')and '..'not in Path(m.name).parts for m in t.getmembers());t.extractall(restore,filter='data')
for row in rows:assert sha((restore/row['path']).read_bytes())==row['sha256']
save('archive-inventory.json',rows);save('archive.json',{'sha256':sha(archive.read_bytes()),'bytes':archive.stat().st_size,'archived_files':len(rows),'restored_version':str(restore),'contains_live_pointers':False,'excluded':'lifecycle locks, SQLite WAL/shared-memory sidecars, pre-initialization backup; authoritative vector DB SHA checked'})
print(json.dumps({'archive_bytes':archive.stat().st_size,'archive_files':len(rows),'source_capture_files':1417,'index_embedding_inputs':666,'probes':2,'scope_bijection':True,'activated':False}))
