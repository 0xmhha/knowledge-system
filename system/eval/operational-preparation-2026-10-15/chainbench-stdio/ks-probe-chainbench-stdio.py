import subprocess,json,time,signal,hashlib,os
from pathlib import Path
folder=Path('/private/tmp/ks-op-chainbench-shutdown-evidence'); binary=Path('/private/tmp/ks-op-chainbench-shutdown/bin/chainbench-mcp')
env=os.environ.copy()
for k in list(env):
 if k.startswith('CHAINBENCH_'):env.pop(k)
env['CHAINBENCH_DATA_ROOT']='/private/tmp/ks-op-chainbench-owned-state'
init={'jsonrpc':'2.0','id':1,'method':'initialize','params':{}}; tools={'jsonrpc':'2.0','id':2,'method':'tools/list'}
requests='\n'.join(json.dumps(x) for x in [init,tools])+'\n'
before=subprocess.run(['/private/tmp/ks-chainbench-mcp-before'],input=requests,text=True,capture_output=True,timeout=5,cwd='/private/tmp',env=env)
assert before.returncode==0
old=json.loads(before.stdout.splitlines()[1])['result']['tools']
(folder/'before-metadata.json').write_text(json.dumps({'command':['/private/tmp/ks-chainbench-mcp-before'],'requests':[init,tools],'stdout':before.stdout,'stderr':before.stderr,'exit_code':before.returncode,'kind':'controlled_metadata_only'},indent=2)+'\n')
records=[]
for mode,n in [('EOF',3),('idle_SIGTERM',3),('idle_SIGINT',1)]:
 for index in range(n):
  start=time.monotonic();p=subprocess.Popen([str(binary)],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,cwd='/private/tmp',env=env);forced=False;rows=[]
  if mode=='EOF':
   stdout,stderr=p.communicate(requests,timeout=5)
   rows=[json.loads(x) for x in stdout.splitlines()]
  else:
   p.stdin.write(requests);p.stdin.flush()
   rows=[json.loads(p.stdout.readline()) for _ in range(2)]
   p.send_signal(signal.SIGTERM if mode=='idle_SIGTERM' else signal.SIGINT)
   p.wait(timeout=5);stdout='\n'.join(json.dumps(x) for x in rows)+'\n'+p.stdout.read();stderr=p.stderr.read();p.stdin.close()
  assert p.returncode==0,(mode,p.returncode,stderr)
  assert len(rows)==2 and rows[0]['id']==1 and rows[1]['id']==2
  assert rows[1]['result']['tools']==old,'advertised tool schema changed'
  records.append({'mode':mode,'repeat':index+1,'exit_code':p.returncode,'forced_kill':forced,'elapsed_seconds':time.monotonic()-start,'requests':[init,tools],'stdout':stdout,'stderr':stderr,'tool_contract_equal_before':True})
result={'schema_version':1,'kind':'controlled_metadata_only_regression_not_installed_host_or_node_acceptance','binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'binary':str(binary),'actual_node_operations':0,'model_calls':0,'tools_count':len(old),'records':records,'all_passed':True}
(folder/'after-metadata.json').write_text(json.dumps(result,indent=2)+'\n')
print('Actual binary: EOF3, idleSIGTERM3, idleSIGINT1 exit0, forced kills0, tool schemas unchanged:',len(old))
