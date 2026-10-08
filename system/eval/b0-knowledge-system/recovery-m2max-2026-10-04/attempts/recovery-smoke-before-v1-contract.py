"""Small owned-fixture recovery diagnostic; not operating restore/quality approval."""
from pathlib import Path
import argparse,subprocess,os,json,hashlib,shutil,time,sys,copy
parser=argparse.ArgumentParser();parser.add_argument('--bin-dir',type=Path,required=True);parser.add_argument('--scripts',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=args.out;out.mkdir();bins=args.bin_dir;scripts=args.scripts;src=out/'src';src.mkdir();dataset=out/'dataset';config=out/'setup.yaml';env=dict(os.environ,GIT_AUTHOR_DATE='2026-10-04T00:00:00Z',GIT_COMMITTER_DATE='2026-10-04T00:00:00Z');commands=[]
def sha(raw):return hashlib.sha256(raw).hexdigest()
def load(path):return json.loads(path.read_text())
def run(name,argv,expected=0):
 start=time.monotonic_ns();r=subprocess.run([str(a) for a in argv],capture_output=True,text=True,env=env,timeout=120);elapsed=time.monotonic_ns()-start;(out/(name+'.stdout.txt')).write_text(r.stdout);(out/(name+'.stderr.txt')).write_text(r.stderr);commands.append({'id':name,'argv':[str(a) for a in argv],'exit_code':r.returncode,'elapsed_ns_diagnostic':elapsed});(out/'commands.json').write_text(json.dumps(commands,indent=2)+'\n');assert r.returncode==expected,(name,r.returncode,r.stderr);return r

def snapshot(path):
 values={}
 for f in sorted(path.rglob('*')):
  if f.is_file():values[str(f.relative_to(path))]={'bytes':f.stat().st_size,'sha256':sha(f.read_bytes()),'executable':bool(f.stat().st_mode&0o111)}
 return values

def mcp_config(name,root):
 cfg=out/(name+'.yaml');run(name+'-config',[bins/'cks','mcp','gen-config','--dataset-dir',root/'current','--source-root',src,'--sanitize-rules',bins/'policies/sanitization_rules.yaml','--out',cfg]);s=cfg.read_text()
 for before,after in [('provider: ""','provider: mock'),('embed_model: bge-m3','embed_model: mock-feature-hash-v1'),('mcp_stdio: false','mcp_stdio: true'),('transport: http','transport: stdio')]:assert before in s;s=s.replace(before,after)
 cfg.write_text(s);return cfg

def capture(name,cfg,version):
 req=out/(name+'-requests.json');req.write_text(json.dumps({'schema_version':1,'requests':[{'id':'guide-v1','tool':'cks.context.get_for_task','arguments':{'prompt':'Where is the quartz committed guide?'}},{'id':'guide-v2','tool':'cks.context.get_for_task_v2','arguments':{'prompt':'Where is the quartz committed guide?'}}]})+'\n');dest=out/(name+'-capture.json');run(name+'-capture',[bins/'cks','eval','capture','--requests',req,'--config',cfg,'--output',dest,'--retrieval-runs','1','--warmup','0','--warm-runs','0','--cold-runs','0','--call-timeout','30s']);report=load(dest);rows=report['rows'];assert report['state']=='captured' and len(rows)==2 and report['quality_metrics'] is None
 source=load(version/'sources/manifest.json');sources={f['path']:(version/'sources/blobs'/f['sha256']).read_bytes() for f in source['files'] if f['kind']=='regular'};identity=load(version/'dataset-identity.json');citations=0
 for row in rows:
  call=row['call'];assert not call.get('error');response=call['response'];assert not response.get('isError');pack=response.get('structuredContent') or json.loads(response['content'][0]['text']);assert pack['citations']
  for c in pack['citations']:
   assert c['commit_hash']==identity['source']['source_commit'];raw=sources[c['file']];lines=raw.splitlines(keepends=True);assert 1<=c['start_line']<=c['end_line']<=len(lines);assert sha(b''.join(lines[c['start_line']-1:c['end_line']]))==c['content_sha256'];citations+=1
   if pack.get('format_version')==2:assert c['file_sha256']==sha(raw) and c['dataset_id']==identity['dataset_id'] and c['snapshot_id']==identity['source']['snapshot_id']
  for body in pack['bodies']:assert body['citation'] in pack['citations'] and sha(body['text'].encode())==body['citation']['content_sha256']
  if pack.get('format_version')==2:
   clone=copy.deepcopy(pack);expected=clone['metadata'].pop('integrity_hash');assert sha(json.dumps(clone,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode())==expected
 return {'rows':len(rows),'citations':citations,'dataset_id':identity['dataset_id'],'source_commit':identity['source']['source_commit']}

(src/'README.md').write_text('# Guide\nThe quartz committed guide records revision one.\n');(src/'main.ts').write_text('export function greet(): string { return "hello"; }\n')
run('git-init',['git','-C',src,'init','-q']);run('git-add',['git','-C',src,'add','.']);run('git-commit',['git','-C',src,'-c','user.name=Fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','recovery baseline'])
run('init',[bins/'cks','init','--src',src,'--dataset',dataset,'--config-out',config,'--embedder','mock']);run('baseline',[bins/'cks','setup','--config',config,'--version','baseline']);old=dataset/'baseline';oldhash=snapshot(old);backup=out/'backup/baseline';shutil.copytree(old,backup);assert snapshot(backup)==oldhash;cfg=mcp_config('runtime',dataset);checks={'baseline':capture('baseline',cfg,old)};assert snapshot(old)==oldhash
(src/'README.md').write_text('# Guide\nThe quartz committed guide records revision two.\n');run('second-add',['git','-C',src,'add','README.md']);run('second-commit',['git','-C',src,'-c','user.name=Fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','recovery update'])
run('test-gate-failure',[bins/'cks','setup','--config',config,'--version','failed-test','--gate-test-bin','/bin/false'],expected=1);assert os.readlink(dataset/'current')=='baseline' and snapshot(old)==oldhash;assert (dataset/'failed-test/test-gate.json').exists();assert load(dataset/'failed-test/test-gate.json')['exit_code']!=0
fake=out/'fail-vector.py';fake.write_text('#!/usr/bin/env python3\nimport subprocess,sys\nif "build" in sys.argv[1:]:sys.exit(23)\nsys.exit(subprocess.run(['+repr(str(bins/'ckv'))+',*sys.argv[1:]]).returncode)\n');fake.chmod(0o755)
run('vector-build-failure',[bins/'cks','setup','--config',config,'--version','failed-vector','--vector-bin',fake],expected=1);assert os.readlink(dataset/'current')=='baseline' and snapshot(old)==oldhash;assert (dataset/'failed-vector/graph/graph.db').exists();assert (dataset/'failed-vector').exists();checks['failed_candidates_preserved']=True
bad=dataset/'corrupt-target';shutil.copytree(old,bad);blob=next((bad/'sources/blobs').iterdir());blob.write_bytes(blob.read_bytes()+b'synthetic corrupt target');run('corrupt-target-rollback',[bins/'cks','rollback','corrupt-target','--config',config],expected=1);assert os.readlink(dataset/'current')=='baseline' and snapshot(old)==oldhash
control=out/'pin-control';probe_stdout=(out/'pin.stdout.txt').open('w');probe_stderr=(out/'pin.stderr.txt').open('w');proc=subprocess.Popen(['python3',str(scripts/'wbs-mcp-pin-probe.py'),str(bins/'cks'),str(cfg),str(control)],stdout=probe_stdout,stderr=probe_stderr,env=env)
try:
 deadline=time.monotonic()+25
 while not (control/'ready.json').exists():
  assert proc.poll() is None,'pin probe exited';assert time.monotonic()<deadline;time.sleep(.05)
 run('updated-build',[bins/'cks','setup','--config',config,'--version','updated']);assert os.readlink(dataset/'current')=='updated' and snapshot(old)==oldhash
 (control/'next').write_text('continue\n');assert proc.wait(timeout=30)==0
finally:
 if proc.poll() is None:proc.terminate();proc.wait(timeout=5)
 probe_stdout.close();probe_stderr.close()
assert load(control/'ready.json')==load(control/'after.json');checks['running_mcp_remains_baseline']=load(control/'after.json');checks['updated']=capture('updated',cfg,dataset/'updated')
run('normal-rollback',[bins/'cks','rollback','baseline','--config',config]);assert os.readlink(dataset/'current')=='baseline';checks['normal_rollback']=capture('normal-rollback',cfg,old);assert snapshot(old)==oldhash
# A damaged current capture cannot be trusted as a rollback authority. Preserve
# the original root and prove supported restore into a fresh root from backup.
run('reactivate-updated',[bins/'cks','rollback','updated','--config',config]);activeblob=next((dataset/'updated/sources/blobs').iterdir());activeblob.write_bytes(activeblob.read_bytes()+b'synthetic corrupt active');run('corrupt-current-direct-rollback',[bins/'cks','rollback','baseline','--config',config],expected=1);assert os.readlink(dataset/'current')=='updated';assert snapshot(old)==oldhash
recovered=out/'restored-dataset';recovered.mkdir();shutil.copytree(backup,recovered/'restored');assert snapshot(recovered/'restored')==oldhash;run('restore-backup',[bins/'cks','rollback','restored','--out',recovered]);assert os.readlink(recovered/'current')=='restored';recoveredcfg=mcp_config('recovered-runtime',recovered);checks['restored']=capture('restored',recoveredcfg,recovered/'restored')
src.rename(out/'source-removed');checks['without_original_source']=capture('source-removed',recoveredcfg,recovered/'restored');assert checks['without_original_source']==checks['restored'];assert snapshot(old)==oldhash and snapshot(backup)==oldhash and snapshot(recovered/'restored')==oldhash
checks.update({'scope':'small TypeScript/mock recovery wiring with extracted preview binaries; not legacy migration/operating restore/quality approval','original_version_sha_before':oldhash,'original_version_sha_after':snapshot(old),'backup_sha_after':snapshot(backup),'restored_sha_after':snapshot(recovered/'restored'),'damaged_root_preserved':dataset.exists(),'failed_candidate_dirs_preserved':True,'original_source_removed_for_final_replay':True,'quality_metrics':None,'binary_sha256':{name:sha((bins/name).read_bytes()) for name in ['cks','ckg','ckv']}});(out/'verification.json').write_text(json.dumps(checks,indent=2)+'\n');print('candidate failures, pin/update/rollback, corrupt-target rejection, fresh-root backup restore and source-absent replay verified')
