import subprocess,json,tempfile,time,urllib.request
from pathlib import Path
base=Path('/private/tmp/ks-wi4-public-probe');base.mkdir(exist_ok=True)
cli='/private/tmp/ks-wi4-public-chainbench';mcp='/private/tmp/ks-wi4-public-mcp'
cwd='/private/tmp/ks-upper-wi-chainbench'
def command(name,args,allow=False):
 r=subprocess.run(args,cwd=cwd,capture_output=True,text=True,timeout=180)
 (base/(name+'.stdout')).write_text(r.stdout);(base/(name+'.stderr')).write_text(r.stderr)
 if not allow and r.returncode:raise RuntimeError(f'{name}: exit {r.returncode}')
 return r
inp=json.loads(Path('/private/tmp/ks-wi4-public-four-input.json').read_text());inp['artifact_root']=tempfile.mkdtemp(prefix='w4m-',dir='/tmp');Path(inp['artifact_root']).rmdir()
requests=[{'jsonrpc':'2.0','id':1,'method':'initialize','params':{}},{'jsonrpc':'2.0','id':2,'method':'tools/list','params':{}},{'jsonrpc':'2.0','id':3,'method':'tools/call','params':{'name':'chainbench_experiment','arguments':{'input':inp}}}]
r=subprocess.run([mcp],input='\n'.join(json.dumps(q)for q in requests)+'\n',capture_output=True,text=True,cwd=cwd,timeout=180)
(base/'mcp-requests.jsonl').write_text('\n'.join(json.dumps(q)for q in requests)+'\n');(base/'mcp-responses.jsonl').write_text(r.stdout);(base/'mcp.stderr').write_text(r.stderr)
assert r.returncode==0
responses=[json.loads(line)for line in r.stdout.splitlines()]
assert len(responses)==3 and 'error' not in responses[-1]
report=json.loads(responses[-1]['result']['content'][0]['text']);assert report['verdict']=='INCONCLUSIVE'and report['before']['completed']==1 and report['before']['symptom_failures']==1 and report['trials'][0]['cleanup_ok']
command('mcp-report-verify',[cli,'experiment','verify','--report',inp['artifact_root']+'/report.json'])
owner=tempfile.mkdtemp(prefix='w4o-',dir='/tmp');(base/'owner.txt').write_text(owner)
summary={'mcp':{'verdict':report['verdict'],'before':report['before'],'cleanup_ok':report['trials'][0]['cleanup_ok']},'observation':[]}
try:
 command('owner-up',[cli,'chain','up','--chain','stablenet','--binary','/private/tmp/ks-wi4-gstable','--bp','4','--keys-source','generate','--keys',owner+'/keys','--workspace-dir',owner,'--target',owner,'--json'])
 statePath=Path(owner)/'chain-record.json';state=json.loads(statePath.read_text());before=statePath.read_bytes()
 for n in state['nodes']:
  url='http://127.0.0.1:'+str(n['http']);end=time.monotonic()+20
  while True:
   try:
    data=json.dumps({'jsonrpc':'2.0','id':1,'method':'eth_chainId','params':[]}).encode();response=json.load(urllib.request.urlopen(urllib.request.Request(url,data=data,headers={'Content-Type':'application/json'}),timeout=2));assert response.get('result')=='0x205b';break
   except Exception:
    if time.monotonic()>end:raise
    time.sleep(.1)
 for mode,expected in [('window','8283'),('recurrence','8284')]:
  observation=json.loads(json.dumps(inp));observation['observe_workspace']=owner;observation['artifact_root']=tempfile.mkdtemp(prefix='w4r-',dir='/tmp');Path(observation['artifact_root']).rmdir()
  observation['case']={'schemaVersion':'1','id':'observe-probe','chain':{'name':'stablenet','binary':'pinned'},'assertions':[{'assert':'chainId','on':'node1','compare':'Equal','expected':expected},{'assert':'blockNumber','on':'node1','compare':'GreaterOrEqual','expected':'0'}]}
  p=observation['plan'];p['mode']='observation';p['id']='observe-'+mode;p['symptom']='controlled read-only predicate';p['assertion_id']='observe-probe/assert/0';p['normal_assertion_ids']=['observe-probe/assert/1'];p['injection']=None;p['resume']={'event':'designated_failure','assertion_id':p['assertion_id'],'window_seconds':1};p['before']={'commit':p['before']['commit']};p['trial_timeout_seconds']=30;p['total_timeout_seconds']=60
  unfrozen=base/(mode+'-unfrozen.json');unfrozen.write_text(json.dumps(observation))
  prepared=command(mode+'-prepare',[cli,'experiment','prepare','--input',str(unfrozen)]);inputPath=base/(mode+'-input.json');inputPath.write_text(prepared.stdout)
  observed=command(mode+'-execute',[cli,'experiment','--input',str(inputPath)]);report=json.loads(observed.stdout)
  assert report['verdict']==('AWAITING_OBSERVATION'if mode=='window'else'RECURRED'),report['verdict']
  assert report['resume_requested']==(mode=='recurrence') and report['acceptance']=='not_recorded'
  assert statePath.read_bytes()==before,'observer altered owner state'
  observedNodes=command(mode+'-nodes',[cli,'chain','observe','--workspace-dir',owner]);(base/(mode+'-owner-state.json')).write_bytes(before)
  command(mode+'-verify',[cli,'experiment','verify','--report',observation['artifact_root']+'/report.json'])
  summary['observation'].append({'mode':mode,'verdict':report['verdict'],'resume_requested':report['resume_requested'],'polls':report['trials'][0]['observation']['polls'],'watched_seconds':report['trials'][0]['observation']['watched_seconds'],'owner_state_unchanged':True,'cleanup_read_only':report['trials'][0]['cleanup_ok']})
finally:
 stopped=command('owner-stop',[cli,'chain','stop','--workspace-dir',owner],allow=True)
 summary['owner_stop_exit']=stopped.returncode
 command('owner-final-observe',[cli,'chain','observe','--workspace-dir',owner],allow=True)
 (base/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
 if stopped.returncode:raise RuntimeError('owned fixture cleanup unresolved')
print(json.dumps(summary))
