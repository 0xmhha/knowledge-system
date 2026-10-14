#!/usr/bin/env python3
"""Compiled owned Go corpus with retained policy/security overlays -> public CKS.

No real commands execute. The evaluator preserves actual wire traffic, controls,
and original source even after deleting live source/overlays to forbid fallback.
"""
from pathlib import Path
import argparse, importlib.util, json, subprocess, time, sys
ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('value_probe', ROOT/'scripts/eval-value-dependencies.py')
p = importlib.util.module_from_spec(spec); spec.loader.exec_module(p)
SOURCE = 'package fixture\nfunc Seed(x int) int {return x+1}\nfunc Empty(x int) int {return x*2}\n'
TEST = 'package fixture\nimport "testing"\nfunc TestFixture(t *testing.T){if Seed(3)!=4||Empty(3)!=6{t.Fatal("fixture")}}\n'
POLICY = '''policies:
  - id: fork-review
    name: Fork review
    category: consensus
    description: Preserve fork activation behavior
    governs: [fixture.Seed]
'''
SECURITY = '''security_patterns:
  - id: manual-boundary
    name: Operator boundary review
    category: review
    severity: high
    description: Manual risk hypothesis, not a confirmed vulnerability
    remediation: Review input assumptions before editing
    matches: [fixture.Seed]
  - id: manual-overflow
    name: Arithmetic review
    severity: low
    description: Manual arithmetic hypothesis
    matches: [fixture.Seed]
'''
def main():
 ap=argparse.ArgumentParser()
 for flag in ('binary','cks-binary','plugin-root','out'):ap.add_argument('--'+flag,type=Path,required=True)
 args=ap.parse_args();root=args.out.resolve();p.require(not root.exists(),'fresh owned root required');root.mkdir();repo=root/'source';repo.mkdir()
 def run(label,cmd,cwd=repo):
  before=time.monotonic();r=subprocess.run(cmd,cwd=cwd,capture_output=True,timeout=150)
  (root/(label+'.stdout.txt')).write_bytes(r.stdout);(root/(label+'.stderr.txt')).write_bytes(r.stderr);(root/(label+'.receipt.json')).write_bytes(p.encode({'command':cmd,'exit_code':r.returncode,'elapsed_seconds':time.monotonic()-before}));p.require(r.returncode==0,'failed '+label);return r.stdout.decode().strip()
 for name,text in {'go.mod':'module example.invalid/annotation-fixture\n\ngo 1.26.9\n','logic.go':SOURCE,'logic_test.go':TEST,'policy.yaml':POLICY,'security.yaml':SECURITY}.items():(repo/name).write_text(text)
 run('format',['gofmt','-w','logic.go','logic_test.go']);run('git-init',['git','init','-q']);run('git-add',['git','add','.']);run('git-commit',['git','-c','user.name=Owned annotation fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','test: capture operator annotation and untagged controls']);commit=run('git-head',['git','rev-parse','HEAD']);run('fixture-race',['go','test','-race','-count=1','./...']);run('fixture-vet',['go','vet','./...']);run('fixture-bundle',['git','bundle','create',str(root/'fixture.bundle'),'HEAD'])
 original=root/'original-source';original.mkdir()
 for name in ('logic.go','logic_test.go','go.mod','policy.yaml','security.yaml'):(original/(name+'.txt')).write_bytes((repo/name).read_bytes())
 graph=root/'graph';run('index',[str(args.binary.resolve()),'build','--src',str(repo),'--out',str(graph),'--lang','go','--policy-file',str(repo/'policy.yaml'),'--security-pattern-file',str(repo/'security.yaml')],root)
 ckgroot=root/'ckg';ckgroot.mkdir();ckg=p.Client(args.binary.resolve(),graph,ckgroot);found={}
 try:
  ckg.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-annotation-selection','version':'1'}})
  for name in ('Seed','Empty'):
   nodes=ckg.tool('find_symbol',{'name':name,'exact':False,'include_blobs':True})['nodes'];p.require(len(nodes)==1,'ambiguous selection');found[name]=nodes[0]
 finally:ckg.close()
 cksroot=root/'cks';cksroot.mkdir();sanitize=cksroot/'sanitization_rules.yaml';sanitize.write_bytes((ROOT/'system/policies/sanitization_rules.yaml').read_bytes());config=cksroot/'cks.yaml';config.write_text('version: 1\nname: owned-policy-security\nbackends:\n  ckg:\n    path: '+str(graph/'graph.db')+'\n  ckv:\n    provider: mock\nlisten:\n  transport: stdio\nlogging:\n  level: error\nsanitize:\n  rules_path: '+str(sanitize)+'\n  default_action: drop\n  fail_closed_on_unknown_rule: true\n')
 client=p.Client(None,None,cksroot,command=[str(args.cks_binary.resolve()),'mcp','--config',str(config)],namespace='cks.context.');checks=[]
 def save(name,value):(root/(name+'.json')).write_bytes(p.encode(value));return value
 try:
  client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-annotation-evaluator','version':'1'}});tools=client.request('tools/list')['result']['tools'];save('tools',tools);sys.path.insert(0,str(args.plugin_root.resolve()/'scripts'));import suspect_labels
  count=suspect_labels.check_contract(tools,args.plugin_root.resolve()/'contracts/cks-tools.schema.json');p.require(count==33,'consumer contract drift');tool=next(t for t in tools if t['name']=='cks.context.policy_security');p.require(tool['annotations']['readOnlyHint']is True,'read-only missing');checks.append('actual33 registered tools and exact consumer input contract')
  seed=found['Seed'];query={'node_id':seed['id'],'expected_commit':commit,'expected_source_sha256':p.sha(seed['source'].encode())};out=save('annotations',client.tool('policy_security',query));p.require(out['status']=='annotations_found'and len(out['annotations'])==3 and not out['more'],'annotation relation loss');p.require(all(a['related_symbol_id']==seed['id']for a in out['annotations']),'mixed symbol');checks.append('positive policy and two security relations pinned to selected symbol')
  byid={a['id']:a for a in out['annotations']};a=byid['security:manual-boundary'];p.require(a['explanation']['value']=='Manual risk hypothesis, not a confirmed vulnerability'and a['modification_hint']['value']=='Review input assumptions before editing'and a['severity']['value']=='high','typed fields lost');checks.append('exact typed description remediation severity persistence')
  for a in out['annotations']:
   prov=a['provenance'];p.require(prov['origin']=='operator_curated_metadata'and prov['identity_trust']=='not_authenticated','manual identity elevated');p.require(all(prov[k]['status']=='unknown'and prov[k]['value']==''for k in ('source_sha256','source_line','version','reviewer','reviewed_at','approval')),'invented original provenance');p.require(len(prov['metadata_sha256'])==64,'metadata hash missing')
  checks.append('source row YAML hash version reviewer time approval unknown, separate graph metadata hash')
  p.require(out['safety_verdict']=='unknown'and out['automatic_rules']=='separate_opt_in_rule_candidates_not_executed','manual tags promoted to verdict');rules=save('rules-separate',client.tool('rule_candidates',{'rule_pack':'go-local-review@1.0.0','function':query}));p.require(rules['analysis']['rules']['candidates']==[],'manual tags mixed with automatic local rule results');checks.append('manual tags and opt-in automatic rules remain distinct without safety verdict')
  empty=save('empty',client.tool('policy_security',{'node_id':found['Empty']['id']}));p.require(empty['status']=='no_annotations'and empty['annotations']==[]and empty['safety_verdict']=='unknown','empty implies safe');checks.append('untagged negative does not imply safety')
  missing=save('missing',client.tool('policy_security',{'node_id':'missing'}));p.require(missing['status']=='not_found'and not missing['annotations'],'missing fabricated');checks.append('not-found distinct from empty annotations')
  canonical=seed.get('canonical_id');p.require(canonical,'exact canonical identity not indexed');same=save('canonical',client.tool('policy_security',{'canonical_id':canonical}));p.require(same['annotations']==out['annotations'],'canonical/node mismatch');checks.append('exact canonical identity parity')
  cap=save('cap',client.tool('policy_security',{**query,'max_annotations':1}));p.require(cap['status']=='partial'and cap['more']and len(cap['annotations'])==1 and cap['reason']=='max_annotations','hidden cap');checks.append('actual SQL row cap and partial/more reason')
  for key in ('expected_commit','expected_source_sha256','expected_snapshot_sha256'):
   wrong='0'* (40 if key=='expected_commit'else 64);bad=save('stale-'+key,client.tool('policy_security',{**query,key:wrong}));p.require(bad['status']=='source_mismatch'and bad['symbol']is None and not bad['annotations'],'stale data leaked');checks.append(key+' mismatch refusal')
  for label,argsbad in [('zero',{**query,'max_annotations':0}),('fraction',{**query,'max_annotations':1.5}),('overcap',{**query,'max_annotations':129}),('unknown',{**query,'invented':True}),('dual',{**query,'canonical_id':canonical}),('null',{'node_id':None})]:
   bad=save('input-'+label,client.request('tools/call',{'name':'cks.context.policy_security','arguments':argsbad}));p.require('error'in bad or bad.get('result',{}).get('isError'),'input bypass '+label)
  checks.append('six actual runtime input refusals')
  for name in ('logic.go','policy.yaml','security.yaml'):(repo/name).unlink()
  again=save('after-deletion',client.tool('policy_security',{**query,'expected_snapshot_sha256':out['snapshot_sha256']}));p.require(again['annotations']==out['annotations']and again['symbol']==out['symbol'],'live file fallback');checks.append('identical retained results after source and both overlays deleted')
 finally:client.close()
 summary={'schema_version':1,'fixture_commit':commit,'checks':checks,'cks_tools':count,'ckg_exit':ckg.proc.returncode,'cks_exit':client.proc.returncode,'cks_requests':client.id,'binary_sha256':p.sha(args.binary.read_bytes()),'cks_binary_sha256':p.sha(args.cks_binary.read_bytes()),'independent_acceptance':'not_recorded','http_scope':'middleware and existing CIDR tests separately; stdio fixture does not authenticate a reviewer'};(root/'summary.json').write_bytes(p.encode(summary));print(json.dumps({'out':str(root),'checks':len(checks),'mcp_exit':client.proc.returncode}))
if __name__=='__main__':main()
