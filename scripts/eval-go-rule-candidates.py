#!/usr/bin/env python3
"""Owned compiled Go corpus -> retained graph -> actual opt-in MCP rule queries.

Reports exact corpus scope and deliberate FP/FN limitations. Never executes a
constructed command, reads personal sessions, or records human acceptance.
"""
from pathlib import Path
import argparse
import hashlib
import json
import sqlite3
import subprocess
import time
import importlib.util
_spec = importlib.util.spec_from_file_location("value_probe", Path(__file__).with_name("eval-value-dependencies.py"))
_probe = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_probe)
Client, encode, require, sha = _probe.Client, _probe.encode, _probe.require, _probe.sha
PACK = 'go-local-review@1.0.0'
SOURCE = '''package cases
import (
 "context"
 "os/exec"
 process "os/exec"
 "strconv"
 conversion "strconv"
)
type Fake struct{}
func (Fake) Command(string) string { return "fixed" }
func sanitize(string)string{return "fixed"}
func Parameter(cmd string)*exec.Cmd{return exec.Command(cmd)}
func Compound(cmd string)*exec.Cmd{cmd+="suffix";return exec.Command(cmd)}
func Copy(cmd string)*exec.Cmd{name:=cmd;return exec.Command(name)}
func Kill(cmd string)*exec.Cmd{name:=cmd;name="fixed";return exec.Command(name)}
func Argument(cmd string)*exec.Cmd{return exec.Command("fixed",cmd)}
func Branch(cmd string,flag bool)*exec.Cmd{name:="fixed";if flag{name=cmd};return exec.Command(name)}
func Loop(cmd string,flag bool)*exec.Cmd{name:="fixed";for flag{name=cmd;flag=false};return exec.Command(name)}
func Shadow(cmd string)*exec.Cmd{name:="fixed";{name:=cmd;_=name};return exec.Command(name)}
func Parallel(cmd string)*exec.Cmd{name,other:="fixed",cmd;_=other;return exec.Command(name)}
func CommandContext(cmd string)*exec.Cmd{return exec.CommandContext(context.Background(),cmd)}
func Sanitizer(cmd string)*exec.Cmd{name:=sanitize(cmd);return exec.Command(name)}
func Alias(cmd string)*exec.Cmd{ref:=&cmd;return exec.Command(*ref)}
func Renamed(cmd string)*exec.Cmd{return process.Command(cmd)}
func Spoof(cmd string,exec Fake)string{return exec.Command(cmd)}
func Closure(cmd string)func()*exec.Cmd{return func()*exec.Cmd{return exec.Command(cmd)}}
func Discard(s string)int{v,_:=strconv.Atoi(s);return v}
func DiscardCopy(s string)int64{v,_:=strconv.ParseInt(s,10,64);out:=v;return out}
func Checked(s string)int{v,err:=strconv.Atoi(s);if err!=nil{return 0};return v}
func ErrorKill(s string)int{v,_:=strconv.Atoi(s);v=3;return v}
func ErrorUnused(s string)int{v,_:=strconv.Atoi(s);_=v;return 3}
func ErrorBranch(s string,flag bool)int{v,_:=strconv.Atoi(s);if flag{return v};return 0}
func ErrorRenamed(s string)int{v,_:=conversion.Atoi(s);return v}
func Multiple(cmd string){exec.Command(cmd);exec.Command(cmd)}
'''
TEST = '''package cases
import "testing"
func TestIndependentRuntimeContracts(t *testing.T){
 input:="/fixture/input-command"
 if Compound(input).Path!=input+"suffix"{t.Fatal("compound input propagation lost")}
 for _,cmd:=range []string{Parameter(input).Path,Copy(input).Path,Branch(input,true).Path,Loop(input,true).Path,CommandContext(input).Path,Alias(input).Path,Renamed(input).Path,Closure(input)().Path}{if cmd!=input{t.Fatal("input-dependent name lost",cmd)}}
 for _,cmd:=range []string{Kill(input).Path,Argument(input).Path,Branch(input,false).Path,Loop(input,false).Path,Shadow(input).Path,Parallel(input).Path,Sanitizer(input).Path,Spoof(input,Fake{})}{if cmd!="fixed"{t.Fatal("fixed command contract",cmd)}}
 if Discard("17")!=17||Discard("bad")!=0||DiscardCopy("17")!=17||Checked("bad")!=0||ErrorKill("bad")!=3||ErrorUnused("bad")!=3||ErrorBranch("17",true)!=17||ErrorBranch("17",false)!=0||ErrorRenamed("17")!=17{t.Fatal("conversion contract")}
 Multiple(input)
}
'''
# (expected count under documented matching contract, independent review-condition
# truth for THIS compiled fixture). Truth concerns data/error handling, not exploits.
CASES = {
 'Parameter': (1,True), 'Compound': (1,True), 'Copy': (1,True), 'Kill': (0,False),
 'Argument': (0,False), 'Branch': (1,True), 'Loop': (1,True),
 'Shadow': (0,False), 'Parallel': (0,False), 'CommandContext': (1,True),
 'Sanitizer': (1,False), 'Alias': (0,True), 'Renamed': (0,True),
 'Spoof': (1,False), 'Closure': (0,True),
 'Discard': (1,True), 'DiscardCopy': (1,True), 'Checked': (0,False),
 'ErrorKill': (0,False), 'ErrorUnused': (0,False), 'ErrorBranch': (1,True),
 'ErrorRenamed': (0,True), 'Multiple': (2,True),
}

def graph_digest(directory):
 db=next(directory.glob('*.db'))
 with sqlite3.connect(f'file:{db}?mode=ro',uri=True) as connection:
  rows=list(connection.iterdump())
 return sha('\n'.join(rows).encode())

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--binary',type=Path,required=True);parser.add_argument('--cks-binary',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args()
 root=args.out.resolve();require(not root.exists(),'fresh owned execution directory required');root.mkdir();repo=root/'source';repo.mkdir()
 def run(label,command,cwd=repo):
  started=time.monotonic();out=subprocess.run(command,cwd=cwd,capture_output=True,timeout=150)
  (root/(label+'.stdout.txt')).write_bytes(out.stdout);(root/(label+'.stderr.txt')).write_bytes(out.stderr)
  (root/(label+'.receipt.json')).write_bytes(encode({'command':command,'exit_code':out.returncode,'elapsed_seconds':time.monotonic()-started,'stdout_sha256':sha(out.stdout),'stderr_sha256':sha(out.stderr)}))
  require(out.returncode==0,'failed '+label);return out.stdout.decode().strip()
 for name,contents in {'go.mod':'module example.invalid/rule-fixture\n\ngo 1.26.9\n','cases.go':SOURCE,'cases_test.go':TEST,'manual.yaml':'''security_patterns:
  - id: manual-command-review
    name: Manual command review
    category: command
    severity: info
    description: Owned fixture operator annotation; no automatic confirmation
    matches: ["cases.Parameter"]
'''}.items():(repo/name).write_text(contents)
 run('format',['gofmt','-w','cases.go','cases_test.go']);run('git-init',['git','init','-q']);run('git-add',['git','add','.'])
 run('git-commit',['git','-c','user.name=Owned rule fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','test: capture local rule review cases'])
 commit=run('git-head',['git','rev-parse','HEAD']);run('fixture-normal-race',['go','test','-race','-count=1','./...']);run('fixture-vet',['go','vet','./...']);run('fixture-bundle',['git','bundle','create',str(root/'source.bundle'),'HEAD'])
 graph=root/'graph';binary=args.binary.resolve();run('index',[str(binary),'build','--src',str(repo),'--out',str(graph),'--lang','go','--security-pattern-file',str(repo/'manual.yaml')],root)
 before=graph_digest(graph);client=Client(binary,graph,root);checks=[];found={};outputs={};confusion={'tp':0,'tn':0,'fp':0,'fn':0};times=[]
 try:
  require('error'not in client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-rule-verifier','version':'1'}}),'initialize failed')
  tools=client.request('tools/list')['result']['tools'];tool=next(t for t in tools if t['name']=='flow_probe.context.rule_candidates');require(tool['annotations']['readOnlyHint']is True,'rule write annotation');(root/'tools.json').write_bytes(encode(tools))
  manual=client.tool('find_symbol',{'name':'manual-command-review','exact':True})['nodes'];require(any(n['type']=='SecurityPattern'for n in manual),'manual YAML annotation absent before analysis')
  for name,(count,truth) in CASES.items():
   nodes=client.tool('find_symbol',{'name':name,'exact':False})['nodes'];require(len(nodes)==1,'ambiguous '+name);found[name]=nodes[0]
   query={'rule_pack':PACK,'function':{'node_id':nodes[0]['id'],'expected_commit':commit}}
   out=client.tool('rule_candidates',query);r=out.get('analysis');require(r is not None and r.get('rules'),'missing computed rules '+name);rules=r['rules'];require(rules['status']=='matched_bounded'and len(rules['candidates'])==count,'contract mismatch '+name+': '+str(rules))
   require(rules['origin']=='automatic'and len(rules['pack_sha256'])==64 and r['source']['commit']==commit,'pack/source provenance lost');require(not r['truncated'],'base incomplete')
   for c in rules['candidates']:
    require(c['sanitizer']=='not_proven'and c['target_resolution']=='syntax_only_unverified','semantic confidence overclaim')
    require(all(any(n['id']==p for n in r['nodes'])for p in c['path']),'path references missing original nodes')
   if name in ('Branch','ErrorBranch'):require(any(c['conditions']for c in rules['candidates']),'branch conditions lost')
   key='tp'if count and truth else'tn'if not count and not truth else'fp'if count else'fn';confusion[key]+=1
   times.append(out['elapsed_micros']);outputs[name]=out;(root/(name+'.json')).write_bytes(encode(out));checks.append(name+' contract')
  # Boundaries are executed through the actual public MCP, not just unit calls.
  out=client.tool('rule_candidates',{'rule_pack':PACK,'function':{'node_id':found['Multiple']['id']},'max_candidates':1});require(out['analysis']['rules']['status']=='incomplete'and out['analysis']['rules']['stop_reason']=='candidates'and len(out['analysis']['rules']['candidates'])==1,'candidate cap hidden');checks.append('candidate limit')
  out=client.tool('rule_candidates',{'rule_pack':PACK,'function':{'node_id':found['Parameter']['id']},'max_flow_steps':1});require(out['analysis']['rules']['status']=='incomplete'and out['analysis']['rules']['flow_steps']==1,'work limit hidden');checks.append('flow limit')
  out=client.tool('rule_candidates',{'rule_pack':PACK,'function':{'node_id':found['Parameter']['id'],'expected_source_sha256':'0'*64}});require(out['status']=='source_mismatch'and out['analysis']is None,'stale pin accepted');checks.append('stale pin')
  plain=client.tool('value_dependencies',{'node_id':found['Parameter']['id']});require('rules'not in plain['analysis'],'default enabled rules');checks.append('default off')
  for arguments in ({'function':{'node_id':found['Parameter']['id']}},{'rule_pack':'latest','function':{'node_id':found['Parameter']['id']}},{'rule_pack':PACK,'function':{'node_id':found['Parameter']['id']},'max_candidates':0}):
   response=client.request('tools/call',{'name':'flow_probe.context.rule_candidates','arguments':arguments});require('error'in response or response.get('result',{}).get('isError'),'invalid input accepted')
  checks.append('pack/input refusal')
  (repo/'cases.go').unlink()
  out=client.tool('rule_candidates',{'rule_pack':PACK,'function':{'node_id':found['Parameter']['id'],'expected_source_sha256':outputs['Parameter']['analysis']['source']['sha256'],'expected_snapshot_sha256':outputs['Parameter']['snapshot_sha256']}})
  require(out['analysis']['rules']==outputs['Parameter']['analysis']['rules'],'live source fallback or unstable retained results');checks.append('retained source after deletion')
  after_manual=client.tool('find_symbol',{'name':'manual-command-review','exact':True})['nodes'];require(after_manual==manual,'manual annotation overwritten');checks.append('manual annotation retained')
 finally:client.close()
 require(graph_digest(graph)==before,'read-only candidate query changed persisted graph');checks.append('persisted graph unchanged')
 cksroot=root/'cks';cksroot.mkdir();config=cksroot/'cks.yaml'
 sanitizer=Path(__file__).resolve().parents[1]/'system/policies/sanitization_rules.yaml'
 config.write_text('version: 1\nname: owned-rule-verifier\nbackends:\n  ckg:\n    path: '+str(graph/'graph.db')+'\n  ckv:\n    provider: mock\nlisten:\n  transport: stdio\nlogging:\n  level: error\nsanitize:\n  rules_path: '+str(sanitizer)+'\n  default_action: drop\n  fail_closed_on_unknown_rule: true\n')
 cks=Client(args.cks_binary.resolve(),graph,cksroot,command=[str(args.cks_binary.resolve()),'mcp','--config',str(config)],namespace='cks.context.')
 try:
  require('error'not in cks.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-rule-facade','version':'1'}}),'CKS initialize failed')
  cks_tools=cks.request('tools/list')['result']['tools'];(cksroot/'tools.json').write_bytes(encode(cks_tools))
  for name in ('Parameter','Kill','Discard','Sanitizer','Alias'):
   q={'rule_pack':PACK,'function':{'node_id':found[name]['id'],'expected_commit':commit}}
   out=cks.tool('rule_candidates',q);require(out['analysis']['rules']==outputs[name]['analysis']['rules']and out['analysis']['source']==outputs[name]['analysis']['source'],'CKS facade changed graph computation '+name);(cksroot/(name+'.json')).write_bytes(encode(out));checks.append('CKS '+name+' parity')
  plain=cks.tool('value_dependencies',{'node_id':found['Parameter']['id']});require('rules'not in plain['analysis'],'CKS default enabled pack');checks.append('CKS default off')
  stale=cks.tool('rule_candidates',{'rule_pack':PACK,'function':{'node_id':found['Parameter']['id'],'expected_source_sha256':'0'*64}});require(stale['status']=='source_mismatch'and stale['analysis']is None,'CKS lost stale-pin boundary');checks.append('CKS stale pin')
 finally:cks.close()
 require(graph_digest(graph)==before,'CKS facade changed graph');checks.append('CKS graph unchanged')
 denom=len(CASES);summary={'schema_version':1,'scope':'owned compiled Go fixture; review-condition observations, not vulnerability acceptance','source_commit':commit,'binary_sha256':sha(binary.read_bytes()),'checks':checks,'cases':denom,'matching_contract_passed':denom,'review_condition_confusion':confusion,'precision':confusion['tp']/(confusion['tp']+confusion['fp']),'recall':confusion['tp']/(confusion['tp']+confusion['fn']),'known_false_positives':['Sanitizer','Spoof'],'known_false_negatives':['Alias','Renamed','Closure','ErrorRenamed'],'elapsed_micros':{'min':min(times),'max':max(times),'total':sum(times)},'tools_count':len(tools),'cks_tools_count':len(cks_tools),'cks_requests':cks.id,'cks_exit':cks.proc.returncode,'cks_binary_sha256':sha(args.cks_binary.read_bytes()),'requests':client.id,'mcp_exit':client.proc.returncode,'independent_acceptance':'not_recorded'}
 (root/'summary.json').write_bytes(encode(summary));print(json.dumps({'out':str(root),'checks':len(checks),'cases':denom,'confusion':confusion,'mcp_exit':client.proc.returncode}))
if __name__=='__main__':main()
