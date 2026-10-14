#!/usr/bin/env python3
"""Owned real CKS -> source-bound partial review -> hook/delivery evaluation.

Runs compiled state/guard/resource/priority fixture and tamper controls, never
executes a command object, modifies production code or grants reviewer acceptance.
"""
from pathlib import Path
import argparse
import copy
import hashlib
import importlib.util
import json
import subprocess
import sys
import time

ROOT=Path(__file__).resolve().parents[1]
_spec=importlib.util.spec_from_file_location('value_probe',ROOT/'scripts/eval-value-dependencies.py');probe=importlib.util.module_from_spec(_spec);_spec.loader.exec_module(probe)
encode,require,sha,Client=probe.encode,probe.require,probe.sha,probe.Client
SOURCE='''package fixture
import "os/exec"
type Box struct{ Total int }
func Repeat(command string,values []int){for range values {exec.Command(command)}}
func Process(box *Box,command string,values []int,strict bool)*exec.Cmd {
 total:=0
 for _,value:=range values {total+=value}
 box.Total=total
 if strict && total<0 {command="fixed"}
 return exec.Command(command)
}
'''
TEST='''package fixture
import "testing"
func TestIndependentStateAndGuard(t *testing.T){
 Repeat("/fixture/command",[]int{1,2})
 box:=&Box{};cmd:=Process(box,"/fixture/command",[]int{1,-2},true);if box.Total!=-1||cmd.Path!="fixed"{t.Fatal("negative strict guard/state")}
 cmd=Process(box,"/fixture/command",[]int{1,-2},false);if box.Total!=-1||cmd.Path!="/fixture/command"{t.Fatal("unrestricted branch/state")}
 cmd=Process(box,"/fixture/command",[]int{1,2},true);if box.Total!=3||cmd.Path!="/fixture/command"{t.Fatal("positive branch/state")}
}
'''
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--binary',type=Path,required=True);parser.add_argument('--cks-binary',type=Path,required=True);parser.add_argument('--plugin-root',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args()
 root=args.out.resolve();require(not root.exists(),'fresh owned evaluation root required');root.mkdir();source=root/'source';source.mkdir();plugin=args.plugin_root.resolve();script=plugin/'scripts/deep_review.py';hook=plugin/'hooks/deep-review-guard.py'
 require(script.is_file()and hook.is_file(),'structured review implementation missing')
 def run(label,command,cwd=root,stdin=None,expected=0):
  started=time.monotonic();out=subprocess.run(command,cwd=cwd,input=stdin,capture_output=True,timeout=150)
  (root/(label+'.stdout.txt')).write_bytes(out.stdout);(root/(label+'.stderr.txt')).write_bytes(out.stderr);(root/(label+'.receipt.json')).write_bytes(encode({'command':command,'exit_code':out.returncode,'elapsed_seconds':time.monotonic()-started,'stdout_sha256':sha(out.stdout),'stderr_sha256':sha(out.stderr)}));require(out.returncode==expected,'execution failed '+label);return out.stdout
 for name,text in {'go.mod':'module example.invalid/deep-fixture\n\ngo 1.26.9\n','logic.go':SOURCE,'logic_test.go':TEST}.items():(source/name).write_text(text)
 run('format',['gofmt','-w','logic.go','logic_test.go'],source);run('git-init',['git','init','-q'],source);run('git-add',['git','add','.'],source)
 run('git-commit',['git','-c','user.name=Owned structured review fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','test: capture state guard and priority review conditions'],source)
 commit=run('git-head',['git','rev-parse','HEAD'],source).decode().strip();run('fixture-race',['go','test','-race','-count=1','./...'],source);run('fixture-vet',['go','vet','./...'],source);run('fixture-bundle',['git','bundle','create',str(root/'fixture.bundle'),'HEAD'],source)
 graph=root/'graph';run('index',[str(args.binary.resolve()),'build','--src',str(source),'--out',str(graph),'--lang','go'])
 ckgroot=root/'ckg';ckgroot.mkdir();ckg=Client(args.binary.resolve(),graph,ckgroot)
 try:
  ckg.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-deep-selection','version':'1'}})
  found=ckg.tool('find_symbol',{'name':'Process','exact':False,'include_blobs':True})['nodes'];require(len(found)==1,'exact fixture selection failed');node=found[0]
  repeats=ckg.tool('find_symbol',{'name':'Repeat','exact':False,'include_blobs':True})['nodes'];require(len(repeats)==1,'range fixture selection failed');repeat=repeats[0]
 finally:ckg.close()
 cksroot=root/'cks';cksroot.mkdir();sanitizer=cksroot/'sanitization_rules.yaml';sanitizer.write_bytes((ROOT/'system/policies/sanitization_rules.yaml').read_bytes());config=cksroot/'cks.yaml';config.write_text('version: 1\nname: owned-deep-review\nbackends:\n  ckg:\n    path: '+str(graph/'graph.db')+'\n  ckv:\n    provider: mock\nlisten:\n  transport: stdio\nlogging:\n  level: error\nsanitize:\n  rules_path: '+str(sanitizer)+'\n  default_action: drop\n  fail_closed_on_unknown_rule: true\n')
 cks=Client(args.cks_binary.resolve(),graph,cksroot,command=[str(args.cks_binary.resolve()),'mcp','--config',str(config)],namespace='cks.context.')
 try:
  cks.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-structured-review','version':'1'}})
  tools=cks.request('tools/list')['result']['tools'];(cksroot/'tools.json').write_bytes(encode(tools))
  sys.path.insert(0,str(plugin/'scripts'));import suspect_labels
  contract_count=suspect_labels.check_contract(tools,plugin/'contracts/cks-tools.schema.json');require(contract_count==len(tools),'consumer contract differs from actual tools')
  blob=node['source'].encode();require(blob in (source/'logic.go').read_bytes(),'retained function blob differs from compiled fixture')
  rules_args={'rule_pack':'go-local-review@1.0.0','function':{'node_id':node['id'],'expected_commit':commit,'expected_source_sha256':sha(blob)}}
  range_rules=cks.tool('rule_candidates',{'rule_pack':'go-local-review@1.0.0','function':{'node_id':repeat['id'],'expected_commit':commit,'expected_source_sha256':sha(repeat['source'].encode())}})
  require(any(c.get('location_status')=='structural_block_unlocated'and 'location'not in c for candidate in range_rules['analysis']['rules']['candidates']for c in candidate['conditions']),'range control fabricated a source location');(root/'range-control.json').write_bytes(encode(range_rules))
  rules=cks.tool('rule_candidates',rules_args);require(rules.get('analysis')is not None,'retained Go analysis unavailable');review_args={'symbol':node['canonical_id'],'caller_depth':2};review=cks.tool('review_function',review_args)
 finally:cks.close()
 ticket='DEEP-REVIEW-FIXTURE';workspace=root/'.coding-agent/tickets'/ticket;workspace.mkdir(parents=True)
 bundle={'schema_version':1,'ticket_id':ticket,'rules':{'tool':'cks.context.rule_candidates','arguments':rules_args,'result':rules},'function_review':{'tool':'cks.context.review_function','arguments':review_args,'result':review}}
 (root/'bundle.json').write_bytes(encode(bundle));analysis=rules['analysis'];ids={n['text'].strip():n['id']for n in analysis['nodes']};command=ids['return exec.Command(command)'];state_node=ids['box.Total = total'];loop=ids['values'];candidates=analysis['rules']['candidates'];require(candidates,'expected command candidate absent')
 def section(claim,evidence,assumptions,unknowns):return {'status':'inferred','claim':claim,'evidence_nodes':evidence,'assumptions':assumptions,'unknowns':unknowns,'origin':'agent_reasoning'}
 judgments={'sections':{
 'data_state':section('The retained syntax writes the accumulated total into box.Total',[state_node],['Box has the fixture Total field'],['Memory/alias/concurrent effect and range value dependencies are not proven by the graph']),
 'guards_exceptions':section('Strict negative totals replace the command, while other paths may retain the parameter',[command],['Fixture strict flag is intended to control the negative-total guard'],['Local rule paths do not prove all caller authorization, target resolution or exception behavior']),
 'complexity':section('The source loop suggests O(len(values)) local traversal excluding external construction work',[loop],['Each range iteration has bounded integer addition; all arithmetic cost assumptions are local'],['Range binding/value semantics and external API complexity are not proven or measured']),
 'resources':section('The local total has bounded scalar storage; resource cost of the constructed command is not established',[state_node,command],['Only the source local storage is assessed'],['Input ownership, external allocations and memory use have no runtime measurements']),
 'io':section('The source constructs a command; process execution is outside this function view',[command],['This selector refers to the intended standard library API'],['Target/import resolution and command lookup/environment I/O are unverified']),
 'priority_tradeoffs':section('Prioritize intended command-name restrictions and state ownership before loop optimization; stricter guards can reduce compatibility',[state_node,command],['Review goal prioritizes stable behavior and controlled state changes'],['Caller requirements, threat context and measured throughput are unavailable'])},
 'findings':[{'id':'guard-scope','category':'guard','claim':'Review the unrestricted command-name path under the intended caller policy','suggestion':'Clarify allowed commands and strict=false callers before changing the guard','priority':'high','evidence_nodes':[command],'rule_candidates':[candidates[0]['id']],'assumptions':['Command selection is a review concern in the fixture'],'unknowns':['No exploit or whole-program access-control gap is established'],'origin':'agent_reasoning','state':'review_candidate'},
 {'id':'state-priority','category':'tradeoff','claim':'State ownership/guard correctness should be reviewed before optimizing the accumulation loop','suggestion':'Document ownership and compatibility conditions, then measure external costs separately','priority':'medium','evidence_nodes':[state_node,loop],'rule_candidates':[],'assumptions':['State stability is more important than unmeasured speed'],'unknowns':['No production concurrency contract or benchmark supplied'],'origin':'agent_reasoning','state':'review_candidate'}],
 'criteria':[{'id':name,'requirement':'Review '+name,'section':name,'status':'covered','reason':'Scoped original-node evidence and explicit assumptions supplied; independent acceptance not recorded'}for name in ('data_state','guards_exceptions','complexity','resources','io','priority_tradeoffs')]}
 (root/'judgments.json').write_bytes(encode(judgments))
 state={'ticket_id':ticket,'ticket_type':'code_review','pipeline_variant':'review_only','current_state':'ANALYSIS','states':{'ANALYSIS':{'status':'in_progress'},'PLANNING':{'status':'pending'}}};(workspace/'state.json').write_bytes(encode(state));checks=['public-range-control-location-unknown']
 def invoke(label,candidate,directory=workspace,deny=False):
  event={'tool_name':'Write','cwd':str(directory),'tool_input':{'file_path':str(directory/'state.json'),'content':json.dumps(candidate)}};out=run(label,[sys.executable,str(hook)],stdin=encode(event));decision=json.loads(out)['hookSpecificOutput']['permissionDecision']if out.strip()else'allow';require(decision==('deny'if deny else'allow'),'hook decision mismatch '+label);checks.append(label)
 proposed=copy.deepcopy(state);proposed['current_state']='COMPLETED';invoke('incomplete-review-denied',proposed,deny=True)
 run('build-review',[sys.executable,str(script),'build','--workspace',str(workspace),'--bundle',str(root/'bundle.json'),'--judgments',str(root/'judgments.json')]);checks.append('actual-artifact-builder')
 state['current_state']='PLANNING';state['states']['ANALYSIS']={'status':'completed','deep_review_sha256':sha((workspace/'deep-review.json').read_bytes())};invoke('bound-analysis-handoff',state);(workspace/'state.json').write_bytes(encode(state))
 final=copy.deepcopy(state);final['current_state']='COMPLETED';invoke('unbound-planning-denied',final,deny=True);final['states']['PLANNING']={'status':'completed','review_report_sha256':sha((workspace/'review-report.md').read_bytes())};invoke('bound-review-delivery',final);(workspace/'state.json').write_bytes(encode(final))
 verification=json.loads(run('verify-delivery',[sys.executable,str(script),'verify','--workspace',str(workspace)]));require(verification['independent_acceptance']=='not_recorded','review claimed human acceptance');checks.append('standalone-delivery-verifier')
 for case in ('type','source','computed-edge','acceptance','dimension','report','diagram','deleted-artifact','frozen-hash'):
  import shutil
  copydir=root/'.coding-agent/tickets'/('CONTROL-'+case);shutil.copytree(workspace,copydir);candidate=copy.deepcopy(final)
  if case=='type':candidate['ticket_type']='feature'
  elif case=='report':(copydir/'review-report.md').write_text('review delivered without original evidence')
  elif case=='diagram':(copydir/'sequence.mmd').write_text('flowchart TD\nforged --> safe\n')
  elif case=='deleted-artifact':(copydir/'deep-review-evidence.json').unlink()
  elif case=='frozen-hash':candidate['states']['ANALYSIS']['deep_review_sha256']='0'*64
  else:
   report=json.loads((copydir/'deep-review.json').read_text())
   if case=='source':report['source']['sha256']='0'*64
   if case=='computed-edge':report['computed']['sequence']['edges'][0]['condition']='proved_safe'
   if case=='acceptance':report['independent_acceptance']='accepted'
   if case=='dimension':report['reasoning']['sections'].pop('io')
   (copydir/'deep-review.json').write_bytes(encode(report))
  invoke('tamper-'+case+'-denied',candidate,copydir,True)
 (root/'summary.json').write_bytes(encode({'schema_version':1,'scope':'owned compiled Go state/guard/cost/tradeoff fixture -> actual CKS31 contract -> typed partial review/diagrams -> actual hook and standalone delivery validation; not model quality or human acceptance','fixture_commit':commit,'checks':checks,'cks_contract_tools':contract_count,'ckg_exit':ckg.proc.returncode,'cks_exit':cks.proc.returncode,'cks_requests':cks.id,'source_sha256':analysis['source']['sha256'],'snapshot_sha256':analysis['source']['snapshot_sha256'],'validator_sha256':sha(script.read_bytes()),'hook_sha256':sha(hook.read_bytes()),'binary_sha256':sha(args.binary.read_bytes()),'cks_binary_sha256':sha(args.cks_binary.read_bytes()),'independent_acceptance':'not_recorded'}));print(json.dumps({'out':str(root),'checks':len(checks),'contract_tools':contract_count,'ckg_exit':ckg.proc.returncode,'cks_exit':cks.proc.returncode}))
if __name__=='__main__':main()
