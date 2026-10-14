#!/usr/bin/env python3
"""Opt-in source-bound Go index -> real stdio MCP dependence verification.

Only creates/reads an owned fixture directory. No personal source/session logs.
Produces factual raw protocol and expected-relation checks, not human acceptance.
"""
from pathlib import Path
import argparse
import hashlib
import json
import queue
import subprocess
import threading
import time

SOURCE_A='''package p

type Box struct { Value int }
func Same(v int) int { return v+1 }
func Branch(a,b int,flag bool) int { y:=a; if flag { y=b }; return y }
func Kill(a,b int) int { y:=a; y=b; return y }
func Loop(x int) int { y:=0; for x>0 { x--; y=x }; return y }
func Field(box *Box,x int) int { box.Value=x; return box.Value }
func Named(flag bool)(out int) { if flag {out=1};return }
func Closure(x int) int { f:=func(){x=9};_ = f;return x }
func Nonexit(flag bool) { if flag { for {} } }
func Const() int { const value=3;return value }
func Pointer(pointer *int)*int { copy:=pointer;return copy }
func (box *Box) Scalar(x int)int{y:=x;return y}
func Generic[T any](input T)T{copy:=input;return copy}
'''
TEST_A='''package p
import "testing"
func TestNormalContracts(t *testing.T) {
 if Same(3)!=4||Branch(1,2,false)!=1||Branch(1,2,true)!=2||Kill(1,2)!=2||Loop(0)!=0||Loop(3)!=0||Named(false)!=0||Named(true)!=1||Closure(3)!=3 {t.Fatal("scalar contracts")}
 box:=&Box{};if Field(box,4)!=4||box.Value!=4 {t.Fatal("field contract")}
 if Const()!=3||*Pointer(new(int))!=0||box.Scalar(3)!=3||Generic(3)!=3{t.Fatal("constant/reference/method/generic contracts")}
 Nonexit(false)
}
'''
SOURCE_B='''package p
func Same(v int)int { return v*2 }
'''
TEST_B='''package p
import "testing"
func TestSeparatePackageSameName(t *testing.T){if Same(3)!=6{t.Fatal("different function")}}
'''
def require(value,message):
 if not value:raise RuntimeError(message)
def encode(value):return (json.dumps(value,ensure_ascii=False,indent=2,allow_nan=False)+'\n').encode()
def sha(raw):return hashlib.sha256(raw).hexdigest()
class Client:
 def __init__(self,binary,graph,root):
  self.root=root;self.id=0;self.queue=queue.Queue();self.stderr=(root/'mcp.stderr.txt').open('wb');self.stdout=(root/'mcp.stdout.jsonl').open('w');self.stdin=(root/'mcp.stdin.jsonl').open('w')
  self.proc=subprocess.Popen([str(binary),'mcp','--graph',str(graph),'--namespace','flow_probe'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=self.stderr,text=True)
  def read():
   for line in self.proc.stdout:self.queue.put(line)
   self.queue.put(None)
  self.thread=threading.Thread(target=read,daemon=True);self.thread.start()
 def request(self,method,params=None):
  self.id+=1;message={'jsonrpc':'2.0','id':self.id,'method':method}
  if params is not None:message['params']=params
  line=json.dumps(message,separators=(',',':'))+'\n';self.stdin.write(line);self.stdin.flush();self.proc.stdin.write(line);self.proc.stdin.flush();deadline=time.monotonic()+20
  while time.monotonic()<deadline:
   value=self.queue.get(timeout=max(.01,deadline-time.monotonic()));require(value is not None,'MCP stream ended');self.stdout.write(value);self.stdout.flush();reply=json.loads(value)
   if reply.get('id')==self.id:return reply
  raise TimeoutError('actual MCP response timed out')
 def tool(self,name,arguments):
  reply=self.request('tools/call',{'name':'flow_probe.context.'+name,'arguments':arguments});require('error'not in reply,'actual tool protocol error: '+str(reply.get('error')));value=reply['result'];require(not value.get('isError'),'actual tool execution error: '+str(value));return value.get('structuredContent')or json.loads(value['content'][0]['text'])
 def close(self):
  self.proc.stdin.close()
  try:self.proc.wait(timeout=20)
  except subprocess.TimeoutExpired:self.proc.terminate();self.proc.wait(timeout=20);raise
  finally:self.thread.join(timeout=2);self.stderr.close();self.stdin.close();self.stdout.close()
  require(self.proc.returncode==0,'MCP EOF termination failed')
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--binary',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();root=args.out.resolve();require(not root.exists(),'new fixture/evidence root required');root.mkdir();repo=root/'source';repo.mkdir()
 def run(label,command,cwd=repo):
  before=time.monotonic();result=subprocess.run(command,cwd=cwd,capture_output=True,timeout=120);(root/(label+'.stdout.txt')).write_bytes(result.stdout);(root/(label+'.stderr.txt')).write_bytes(result.stderr);(root/(label+'.receipt.json')).write_bytes(encode({'command':command,'cwd':str(cwd),'exit_code':result.returncode,'elapsed_seconds':time.monotonic()-before,'stdout_sha256':sha(result.stdout),'stderr_sha256':sha(result.stderr)}));require(result.returncode==0,'command failed: '+label);return result.stdout.decode().strip()
 for name,content in {'go.mod':'module example.invalid/value-fixture\n\ngo 1.26.9\n','a/flow.go':SOURCE_A,'a/flow_test.go':TEST_A,'b/flow.go':SOURCE_B,'b/flow_test.go':TEST_B}.items():
  p=repo/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(content)
 run('format',['gofmt','-w','a','b']);run('git-init',['git','init','-q']);run('git-add',['git','add','.']);run('git-commit',['git','-c','user.name=Owned value fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','test: capture independent value and control cases']);commit=run('git-head',['git','rev-parse','HEAD']);run('normal-race',['go','test','-race','-count=1','./...']);run('fixture-vet',['go','vet','./...']);run('fixture-bundle',['git','bundle','create',str(root/'source.bundle'),'HEAD'])
 graph=root/'graph';binary=args.binary.resolve();run('index',[str(binary),'build','--src',str(repo),'--out',str(graph),'--lang','go'],root)
 client=Client(binary,graph,root);results={};checks=[]
 try:
  init=client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-value-verifier','version':'1'}});require('error'not in init,'initialize failed')
  tools=client.request('tools/list')['result']['tools'];tool=next(t for t in tools if t['name']=='flow_probe.context.value_dependencies');require(tool['annotations']['readOnlyHint']is True,'actual read-only annotation mismatch');(root/'tools.json').write_bytes(encode(tools))
  functions={}
  for name in ('Branch','Kill','Loop','Field','Named','Closure','Nonexit','Same','Const','Pointer','Scalar','Generic'):
   found=client.tool('find_symbol',{'name':name,'exact':False})['nodes'];require(len(found)==(2 if name=='Same'else 1),'unexpected function resolution '+name);functions[name]=found
  for name,nodes in functions.items():
   for node in nodes:
    input={'node_id':node['id'],'expected_commit':commit};out=client.tool('value_dependencies',input);require(out.get('analysis')is not None,'actual retained analysis unavailable: '+str(out));r=out['analysis'];require(r['source']['node_id']==node['id']and r['source']['commit']==commit and not r['truncated'],'source identity/cap lost');key=name+('/'+r['source']['file']if name=='Same'else'');results[key]=out;(root/(key.replace('/','-')+'.json')).write_bytes(encode(out))
  def node_id(r,text):
   found=[n['id']for n in r['nodes']if n['text'].strip()==text];require(len(found)==1,'node expected once: '+text);return found[0]
  def has(r,kind,left,right):return any(e['kind']==kind and e['from']==left and e['to']==right for e in r['edges'])
  r=results['Branch']['analysis'];initial=node_id(r,'y := a');assigned=node_id(r,'y = b');returned=node_id(r,'return y');condition=node_id(r,'flag');require(has(r,'data_dependence',initial,returned)and has(r,'data_dependence',assigned,returned),'actual join definitions wrong');require(has(r,'control_dependence',condition,assigned)and not has(r,'control_dependence',condition,returned),'actual control join wrong');checks.append('branch/join/control')
  r=results['Kill']['analysis'];require(not has(r,'data_dependence',node_id(r,'y := a'),node_id(r,'return y'))and has(r,'data_dependence',node_id(r,'y = b'),node_id(r,'return y')),'actual redefinition kill wrong');checks.append('redefinition kill')
  r=results['Loop']['analysis'];require(has(r,'data_dependence',node_id(r,'y := 0'),node_id(r,'return y'))and has(r,'data_dependence',node_id(r,'y = x'),node_id(r,'return y')),'actual zero/loop values missing');checks.append('loop fixed point')
  for name,code in [('Field','memory_write_untracked'),('Closure','closure_value_unknown'),('Nonexit','nontermination_control_unknown')]:r=results[name]['analysis'];require(r['status']=='partial'and any(d['code']==code for d in r['diagnostics']),'actual unsupported uncertainty lost');checks.append(name+' unknown')
  r=results['Const']['analysis'];require(has(r,'data_dependence',node_id(r,'value = 3'),node_id(r,'return value'))and any(n['kind']=='compile_time_constant'and n['block']==-1 for n in r['nodes']),'compile-time constant source was lost or faked as execution');checks.append('compile-time source distinction')
  r=results['Pointer']['analysis'];require(has(r,'reference_dependence',node_id(r,'copy := pointer'),node_id(r,'return copy'))and not has(r,'data_dependence',node_id(r,'copy := pointer'),node_id(r,'return copy')),'reference was relabeled scalar proof');checks.append('reference versus scalar distinction')
  r=results['Scalar']['analysis'];require(has(r,'data_dependence',node_id(r,'y := x'),node_id(r,'return y')),'method-local binding lost');checks.append('isolated method locals')
  r=results['Generic']['analysis'];require(r['status']=='partial'and has(r,'unknown_dependence',node_id(r,'copy := input'),node_id(r,'return copy')),'generic value type was overclaimed');checks.append('generic uncertainty')
  same=[out['analysis']['source']for key,out in results.items()if key.startswith('Same/')];require(len({x['node_id']for x in same})==2 and len({x['file']for x in same})==2 and len({x['sha256']for x in same})==2,'same name mixed source');checks.append('same-name source separation')
  branch=functions['Branch'][0];bad=client.tool('value_dependencies',{'node_id':branch['id'],'expected_source_sha256':'0'*64});require(bad['status']=='source_mismatch'and bad['analysis']is None,'stale source pin accepted');checks.append('stale source refusal')
  bounded=client.tool('value_dependencies',{'node_id':branch['id'],'limits':{'max_iterations':1}});require(bounded['status']=='budget_exhausted'and bounded['analysis']['truncated'],'actual iteration bound lost');checks.append('bounded incomplete result')
  invalid=client.request('tools/call',{'name':'flow_probe.context.value_dependencies','arguments':{'node_id':branch['id'],'unknown':True}});require('error'in invalid or invalid.get('result',{}).get('isError'),'unknown input accepted');checks.append('actual input refusal')
  # Change the live source, then remove it entirely. The immutable index must
  # still return its original blob/pins/relations; no disk source fallback.
  (repo/'a/flow.go').write_text('package p\nfunc Branch()int{return 999}\n');(repo/'a/flow.go').unlink()
  replay=client.tool('value_dependencies',{'node_id':branch['id'],'expected_source_sha256':results['Branch']['analysis']['source']['sha256'],'expected_snapshot_sha256':results['Branch']['snapshot_sha256']});require(replay['analysis']['source']==results['Branch']['analysis']['source']and replay['analysis']['def_use']==results['Branch']['analysis']['def_use'],'live source mixed with retained index');checks.append('retained index after live deletion')
 finally:client.close()
 summary={'schema_version':1,'scope':'actual compiled owned Go fixture -> index -> namespaced stdio MCP; bounded lexical scalar and normal-exit controls, no whole-program memory proof','fixture_commit':commit,'binary_sha256':sha(binary.read_bytes()),'checks':checks,'tools_count':len(tools),'requests':client.id,'mcp_exit':client.proc.returncode,'acceptance':'not_recorded'};(root/'summary.json').write_bytes(encode(summary));print(json.dumps({'out':str(root),'actual_checks':len(checks),'mcp_exit':client.proc.returncode,'acceptance':'not_recorded'}))
if __name__=='__main__':main()
