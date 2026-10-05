from pathlib import Path
import subprocess,json,hashlib,time,datetime
root=Path.cwd();r=Path('/private/tmp/ks-approved-scope-20261006');out=Path('/private/tmp/ks-final-linux-real-20261006');out.mkdir(mode=0o700,exist_ok=False);server='ks-final-bge-linux-arm64-20261006';image='sha256:47e391932267b8d18bcf50659b67737b63af4b16863e8da84b9e5d29f17d4ebb';models=Path('/private/tmp/ks-linux-real-model-20261004/models');blob=Path('/Users/kevin/.ollama/models/blobs/sha256-daec91ffb5dd0c27411bd71f29932917c49cf529a641d0168496c3a501e3062c');sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest();checks=[]
def run(label,args,allowed=(0,)):
 p=out/(label+'.stdout.txt');err=out/(label+'.stderr.txt')
 with p.open('xb')as stream,err.open('xb')as errors:code=subprocess.run([str(v)for v in args],stdout=stream,stderr=errors).returncode
 checks.append({'id':label,'command':[str(v)for v in args],'exit_code':code,'stdout_sha256':sha(p),'stderr_sha256':sha(err)});(out/'execution.json').write_text(json.dumps({'status':'running','checks':checks},indent=2)+'\n');print(label,code,flush=True);assert code in allowed,label
before={str(p):sha(p)for p in models.rglob('*')if p.is_file()};before[str(blob)]=sha(blob)
(out/'model-seals-before.json').write_text(json.dumps(before,indent=2)+'\n');assert before[str(blob)]=='daec91ffb5dd0c27411bd71f29932917c49cf529a641d0168496c3a501e3062c'
started=False
try:
 run('server-start',['docker','run','-d','--name',server,'--platform','linux/arm64','--network','none','--cpus','4','--memory','4g','-e','OLLAMA_HOST=127.0.0.1:11434','-e','OLLAMA_MODELS=/models','-e','OLLAMA_NUM_PARALLEL=1','-e','OLLAMA_MAX_LOADED_MODELS=1','-v',str(models)+':/models:ro','-v',str(blob)+':/models/blobs/'+blob.name+':ro',image,'serve']);started=True
 probe='import urllib.request,time; deadline=time.monotonic()+30\nwhile True:\n try:\n  print(urllib.request.urlopen("http://127.0.0.1:11434/api/version",timeout=2).read().decode());break\n except Exception:\n  if time.monotonic()>deadline:raise\n  time.sleep(.5)'
 run('server-ready',['docker','run','--rm','--platform','linux/arm64','--network','container:'+server,'knowledge-system-runtime-smoke:bookworm-arm64','python3','-c',probe])
 run('server-inspect',['docker','inspect',server]);run('runtime-focused',['docker','run','--rm','--platform','linux/arm64','--network','container:'+server,'--cpus','1','--memory','512m','-v',str(out)+':/out','-v','/private/tmp/ks-final-package-linux-arm64-20261006/dist:/package/dist:ro','-v',str(root/'scripts')+':/scripts:ro','-v',str(r/'run-final-linux-real-focused.py')+':/run-focused.py:ro','knowledge-system-runtime-smoke:bookworm-arm64','python3','/run-focused.py'])
 run('server-inspect-after',['docker','inspect',server]);run('server-logs',['docker','logs',server]);after={p:sha(Path(p))for p in before};assert before==after;(out/'model-seals-after.json').write_text(json.dumps(after,indent=2)+'\n')
 (out/'execution.json').write_text(json.dumps({'status':'captured_pending_independent_source_sdk_audit','checks':checks,'model_bytes_unchanged':True,'native_linux_arm64_vm':True,'native_amd64':False,'operating_release':False,'quality_metrics':None,'finished_at':datetime.datetime.now(datetime.timezone.utc).isoformat()},indent=2)+'\n')
finally:
 if started:
  cleanup=subprocess.run(['docker','rm','-f',server],capture_output=True,text=True);(out/'server-cleanup.json').write_text(json.dumps({'server':server,'exit_code':cleanup.returncode,'stdout':cleanup.stdout,'stderr':cleanup.stderr})+'\n')
print('Linux actual BGE3-project/6-SDK capture complete; task server removed',flush=True)
