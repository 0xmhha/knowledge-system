#!/usr/bin/env python3
"""Owned compiled call-cycle/noise corpus -> actual bounded CKS expansion.

Independent symbol relevance denominators compare a local seed baseline with
bounded call-context expansion. Does not certify interprocedural value transfer.
"""
from pathlib import Path
import argparse,importlib.util,json,subprocess,time
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('value_probe',ROOT/'scripts/eval-value-dependencies.py');p=importlib.util.module_from_spec(spec);spec.loader.exec_module(p)
SOURCE='''package fixture
func Seed(x int)int { Noise();if x<0{return Cycle(x+1)};return Transform(x) }
func Cycle(x int)int {if x<0{return Seed(x+1)};return Transform(x)}
func Transform(x int)int{return Normalize(x)+1}
func Normalize(x int)int{return x+2}
func Noise()int{return 99}
func Isolated(x int)int{return x*2}
'''
TEST='''package fixture
import "testing"
func TestIndependentCallAndValueContracts(t *testing.T){if Seed(2)!=5||Seed(-2)!=3||Cycle(-1)!=3||Transform(1)!=4||Normalize(1)!=3||Noise()!=99||Isolated(3)!=6{t.Fatal("fixture contract")}}
'''
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--binary',type=Path,required=True);parser.add_argument('--cks-binary',type=Path,required=True);parser.add_argument('--plugin-root',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();root=args.out.resolve();p.require(not root.exists(),'fresh owned root required');root.mkdir();repo=root/'source';repo.mkdir()
 def run(label,cmd,cwd=repo):
  before=time.monotonic();out=subprocess.run(cmd,cwd=cwd,capture_output=True,timeout=150);(root/(label+'.stdout.txt')).write_bytes(out.stdout);(root/(label+'.stderr.txt')).write_bytes(out.stderr);(root/(label+'.receipt.json')).write_bytes(p.encode({'command':cmd,'exit_code':out.returncode,'elapsed_seconds':time.monotonic()-before}));p.require(out.returncode==0,'failed '+label);return out.stdout.decode().strip()
 for name,text in {'go.mod':'module example.invalid/expansion-fixture\n\ngo 1.26.9\n','logic.go':SOURCE,'logic_test.go':TEST}.items():(repo/name).write_text(text)
 run('format',['gofmt','-w','logic.go','logic_test.go']);run('git-init',['git','init','-q']);run('git-add',['git','add','.']);run('git-commit',['git','-c','user.name=Owned expansion fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','test: capture call cycle and value relevance conditions']);commit=run('git-head',['git','rev-parse','HEAD']);run('fixture-race',['go','test','-race','-count=1','./...']);run('fixture-vet',['go','vet','./...']);run('fixture-bundle',['git','bundle','create',str(root/'fixture.bundle'),'HEAD'])
 graph=root/'graph';run('index',[str(args.binary.resolve()),'build','--src',str(repo),'--out',str(graph),'--lang','go'],root);ckgroot=root/'ckg';ckgroot.mkdir();ckg=p.Client(args.binary.resolve(),graph,ckgroot)
 try:
  ckg.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-expansion-selection','version':'1'}});found={}
  for name in ('Seed','Cycle','Transform','Normalize','Noise','Isolated'):
   nodes=ckg.tool('find_symbol',{'name':name,'exact':False,'include_blobs':True})['nodes'];p.require(len(nodes)==1,'ambiguous '+name);found[name]=nodes[0]
 finally:ckg.close()
 cksroot=root/'cks';cksroot.mkdir();policy=cksroot/'sanitization_rules.yaml';policy.write_bytes((ROOT/'system/policies/sanitization_rules.yaml').read_bytes());config=cksroot/'cks.yaml';config.write_text('version: 1\nname: owned-value-expansion\nbackends:\n  ckg:\n    path: '+str(graph/'graph.db')+'\n  ckv:\n    provider: mock\nlisten:\n  transport: stdio\nlogging:\n  level: error\nsanitize:\n  rules_path: '+str(policy)+'\n  default_action: drop\n  fail_closed_on_unknown_rule: true\n')
 client=p.Client(None,None,cksroot,command=[str(args.cks_binary.resolve()),'mcp','--config',str(config)],namespace='cks.context.');checks=[]
 try:
  client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-expansion-evaluator','version':'1'}});tools=client.request('tools/list')['result']['tools'];(cksroot/'tools.json').write_bytes(p.encode(tools));sys_path=args.plugin_root.resolve()/'scripts';import sys;sys.path.insert(0,str(sys_path));import suspect_labels;count=suspect_labels.check_contract(tools,args.plugin_root.resolve()/'contracts/cks-tools.schema.json');p.require(count==32,'consumer contract drift')
  selected=found['Seed'];query={'node_id':selected['id'],'expected_commit':commit,'expected_source_sha256':p.sha(selected['source'].encode())};baseline=client.tool('value_dependencies',query);(root/'baseline.json').write_bytes(p.encode(baseline));expanded=client.tool('value_dependency_expansion',{'function':query,'limits':{'max_functions':8,'max_hops':3}});(root/'expanded.json').write_bytes(p.encode(expanded));result=expanded['graph'];p.require(result['status']=='expanded_bounded','bounded expansion incomplete: '+str(result['stop_reasons']));functions=result['functions'];ids=[f['result']['analysis']['source']['node_id']for f in functions];p.require(len(ids)==len(set(ids))==5 and result['deduplicated']>=1,'cycle/dedup or function counts wrong');p.require(all(f['result']['snapshot_sha256']==result['snapshot_sha256']for f in functions),'mixed snapshots');checks.append('unique source-pinned local analyses with cycle dedup')
  p.require(all(h['origin']=='retained_call_graph'and h['argument_mapping']=='not_computed'for h in result['hops']),'call hop promoted to computed value transfer');p.require(expanded['curated_flow']['status']=='separate_not_joined'and expanded['curated_flow']['source_association']=='not_verified','curated operator data promoted to computed dependence');checks.append('call/value/control/curated provenance separation')
  relevant={found[n]['id']for n in ('Seed','Cycle','Transform','Normalize')};baseline_ids={baseline['analysis']['source']['node_id']};chosen=set(ids);metrics={'fixture_relevant_symbols':4,'baseline_returned':len(baseline_ids),'baseline_recall':len(baseline_ids&relevant)/4,'expanded_returned':len(chosen),'expanded_recall':len(chosen&relevant)/4,'expanded_noise':len(chosen-relevant),'expanded_precision':len(chosen&relevant)/len(chosen),'baseline_elapsed_micros':baseline['elapsed_micros'],'expanded_elapsed_micros':result['elapsed_micros'],'scope':'independently compiled source relevance labels for this6-function fixture only; no whole-program optimality'};p.require(metrics['baseline_recall']==.25 and metrics['expanded_recall']==1 and metrics['expanded_noise']==1,'independent recall/noise denominator mismatch');checks.append('independent relevance recall noise cost comparison')
  for name,limits in [('functions',{'max_functions':1}),('candidates',{'max_candidates':1}),('nodes',{'max_total_nodes':1}),('edges',{'max_total_edges':1}),('facts',{'max_total_facts':1})]:
   value=client.tool('value_dependency_expansion',{'function':query,'limits':limits});(root/('cap-'+name+'.json')).write_bytes(p.encode(value));g=value['graph'];p.require(g['status']=='budget_exhausted'and g['stop_reasons'],'hidden '+name+' cap');checks.append(name+' budget')
  isolated=client.tool('value_dependency_expansion',{'function':{'node_id':found['Isolated']['id']}});p.require(len(isolated['graph']['functions'])==1 and not isolated['graph']['hops'],'isolated function invented neighbors');checks.append('isolated negative')
  bad=client.tool('value_dependency_expansion',{'function':{'node_id':selected['id'],'expected_source_sha256':'0'*64}});p.require(bad['graph']['status']=='source_mismatch'and not bad['graph']['functions'],'stale source published');checks.append('stale source refusal')
  invalid=client.request('tools/call',{'name':'cks.context.value_dependency_expansion','arguments':{'function':query,'limits':{'max_hops':0}}});p.require('error'in invalid or invalid.get('result',{}).get('isError'),'bad limits accepted');checks.append('actual runtime input refusal')
  (repo/'logic.go').unlink();again=client.tool('value_dependency_expansion',{'function':{**query,'expected_snapshot_sha256':result['snapshot_sha256']},'limits':{'max_functions':8,'max_hops':3}});p.require([f['result']['analysis']['source']for f in again['graph']['functions']]==[f['result']['analysis']['source']for f in functions],'live source fallback or unstable selection');checks.append('retained expansion after source deletion')
 finally:client.close()
 summary={'schema_version':1,'fixture_commit':commit,'checks':checks,'metrics':metrics,'cks_tools':count,'ckg_exit':ckg.proc.returncode,'cks_exit':client.proc.returncode,'cks_requests':client.id,'binary_sha256':p.sha(args.binary.read_bytes()),'cks_binary_sha256':p.sha(args.cks_binary.read_bytes()),'independent_acceptance':'not_recorded'};(root/'summary.json').write_bytes(p.encode(summary));print(json.dumps({'out':str(root),'checks':len(checks),'recall':metrics['expanded_recall'],'noise':metrics['expanded_noise'],'mcp_exit':client.proc.returncode}))
if __name__=='__main__':main()
