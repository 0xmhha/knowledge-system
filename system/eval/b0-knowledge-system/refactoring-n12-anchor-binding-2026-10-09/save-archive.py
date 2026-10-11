from pathlib import Path
import hashlib,json,gzip,io,tarfile,sqlite3
out=Path(__file__).resolve().parent;prep=json.loads((out/'preparation.json').read_text());base=Path(prep['base']);v=Path(prep['dataset_root'])/'functional-v2-anchor-candidate'
sha=lambda b:hashlib.sha256(b).hexdigest()
def save(n,x):(out/n).write_text(json.dumps(x,ensure_ascii=False,indent=2)+'\n')
source=json.loads((out/'source-inventory.json').read_text())['files'];fl=json.loads((v/'files-from.json').read_text());graph=json.loads((v/'graph/manifest.json').read_text());vector=json.loads((v/'vector/manifest.json').read_text())
go={p for p in fl['include']if p.endswith('.go')};actual_go={f['path']for f in graph['files']};vec={f['path']for f in vector['input_files']};expected_vec={p for p in fl['include']if p.endswith(('.go','.md'))}
assert go==actual_go and vec==expected_vec
assert all(f['sha256']==source[f['path']]for f in graph['files']+vector['input_files'])
assert all(sha((Path(prep['source'])/p).read_bytes())==s for p,s in source.items())
audit=json.loads((out/'whole-tree-audit.stdout.json').read_text());assert not(audit.get('in_db_only')or[])and not(set(audit['in_build_only'])&go)
assert not(Path(prep['dataset_root'])/'current').exists()
save('index-scope-check.json',{'selected_files':len(fl['include']),'actual_CKG_files':len(actual_go),'actual_CKV_files':len(vec),'selected_vs_actual_bijection':True,'all_index_file_SHA_match_owned_git_source':True,'source_files_unchanged':len(source),'unvectorized_documents':[p for p in fl['include']if p not in vec and not p.endswith('.go')],'actual_whole_Go_audit':{'exit_code':1,'verdict':'DRIFT','build_count':audit['build_count'],'db_count':audit['db_count'],'in_both':audit['in_both'],'in_build_only':len(audit['in_build_only']),'in_db_only':0,'missing_all_outside_selected_Go_scope':True},'full_repository_index_coverage_claimed':False})
for rel in ['dataset-identity.json','files-from.json','reader-protocol.json','review-hold.json','sources/manifest.json','vector/manifest.json']:(out/('retained-'+rel.replace('/','-'))).write_bytes((v/rel).read_bytes())
(out/'graph-manifest.json.gz').write_bytes(gzip.compress((v/'graph/manifest.json').read_bytes(),mtime=0))
probe={'model':'bge-m3:latest','input':['dimension probe'],'truncate':False,'options':{'num_ctx':8192,'num_batch':8192}};probe_sha=sha(json.dumps(probe,separators=(',',':')).encode())
obs=[json.loads(l)for l in(out/'model-http-observations.jsonl').read_text().splitlines()];embeds=[r for r in obs if r['path']=='/api/embed'];probes=[r for r in embeds if r['request_sha256']==probe_sha];assert len(probes)==2 
assert all(r['status']==200 and r['truncate']is False and r['embedding_inputs']==r['embedding_outputs']for r in embeds)
assert sum(r['embedding_inputs']for r in embeds)==vector['chunk_count']+2
save('model-call-accounting.json',{'relay_HTTP_calls':len(obs),'embedding_HTTP_calls':len(embeds),'startup_probe_HTTP_calls':len(probes),'startup_probe_inputs':2,'index_embedding_HTTP_calls':len(embeds)-len(probes),'index_embedding_inputs':vector['chunk_count'],'successful_embedding_outputs':sum(r['embedding_outputs']for r in embeds),'failed_embedding_HTTP_calls':0,'prompt_eval_tokens':sum(r.get('prompt_eval_count')or 0 for r in embeds),'model_total_duration_ns':sum(r.get('model_total_duration_ns')or 0 for r in embeds),'query_DEV_FINAL_HTTP_calls':0,'scope':'one local held index build; not search evaluation or warm/cold query latency or human review time'})
assert sha((v/'vector/vector.db').read_bytes())==vector['db_sha256']
store=base/'semantic-anchor-bound-proposed.sqlite';assert store.is_file();db=sqlite3.connect('file:'+str(store)+'?mode=ro',uri=True);assert db.execute('select count(*) from semantic_current').fetchone()[0]==0;db.close()
files=[('version/'+f.relative_to(v).as_posix(),f)for f in sorted(v.rglob('*'))if f.is_file()and not f.name.endswith(('-wal','-shm','.lock'))]
assert not any('current'in Path(name).parts or'locks'in Path(name).parts for name,_ in files)
files += [('layer/semantic-anchor-bound-proposed.sqlite',store)]
for name in ['preparation.json','claim-seeds.json','projection-proposed.json','canonical-source-refs.json','relation-review-items.json','projection-driver.go.txt','trace-diagnostic.json']:
 files.append(('layer/'+name,out/name))
rows=[];archive=out/'held-anchor-bound-candidate.tar.gz'
with archive.open('wb')as f:
 with gzip.GzipFile(fileobj=f,mode='wb',mtime=0,filename='')as z:
  with tarfile.open(fileobj=z,mode='w|')as t:
   for name,path in files:
    data=path.read_bytes();mode=path.stat().st_mode&0o777;rows.append({'path':name,'size':len(data),'sha256':sha(data),'mode':mode});info=tarfile.TarInfo(name);info.size=len(data);info.mode=mode;info.mtime=0;t.addfile(info,io.BytesIO(data))
restore=base/'archive-replay';restore.mkdir()
with tarfile.open(archive,'r:gz')as t:
 assert all(m.isfile()and not m.name.startswith('/')and '..'not in Path(m.name).parts for m in t.getmembers());t.extractall(restore,filter='data')
assert all(sha((restore/r['path']).read_bytes())==r['sha256']for r in rows)
save('archive-inventory.json',rows);save('archive.json',{'sha256':sha(archive.read_bytes()),'bytes':archive.stat().st_size,'files':len(rows),'restored_root':str(restore),'contains_source_blobs_and_history_actual_CKG_CKV_and_proposed_SQLite':True,'live_pointers_included':False,'excluded':'lifecycle lock and SQLite WAL/shared-memory sidecars; closed authoritative DB bytes bound','logical_source_root':prep['source'],'not_operating_portability_acceptance':True})
print(json.dumps({'archive_bytes':archive.stat().st_size,'files':len(rows),'source_files':len(source),'CKG_files':len(actual_go),'CKV_files':len(vec),'index_embedding_inputs':vector['chunk_count'],'startup_probe_inputs':2}))
