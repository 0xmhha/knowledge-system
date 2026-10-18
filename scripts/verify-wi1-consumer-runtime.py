#!/usr/bin/env python3
"""Owned local retained CKS matching probe. No scoring or independent approval."""
import sys,importlib.util,json,pathlib,gzip,hashlib,subprocess,shutil
R=pathlib.Path(__file__).resolve().parents[1];sys.path.insert(0,str(R/'scripts'));import wi1_dev_gold as g
import argparse
parser=argparse.ArgumentParser();parser.add_argument('--workspace',type=pathlib.Path,required=True);parser.add_argument('--prepared',type=pathlib.Path,required=True);args=parser.parse_args();S=args.workspace.resolve();prepared=args.prepared.resolve();out=S/'runtime';out.mkdir()
spec=importlib.util.spec_from_file_location('wire',R/'scripts/eval-value-dependencies.py');w=importlib.util.module_from_spec(spec);spec.loader.exec_module(w)
def write(name,v):p=out/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def inventory():
 base=prepared/'stablenet/retained-origins-v2'
 return {str(p.relative_to(base)):g.sha(p.read_bytes()) for sub in ['graph','sources','vector'] for p in (base/sub).rglob('*') if p.is_file() and not p.name.endswith(('lock','lease'))}
before=inventory();write('before.json',before)
cs=g.read(R/g.FUNCTIONS)['functions'];obs=g.packed(R/g.PACKET/'observations.json.gz');files=g.read(R/g.SOURCES);inp=[dict(x,Starts=[c['source']['start_byte'] for c in cs if c['source']['path']==x['Path']]) for x in files]
controls=g.read(R/'scripts/fixtures/wi1-dev-controls.json');control_inputs=[dict(Path='controls/'+x['name']+'.go',Source='package devcontrols\n'+x['source']+'\n',Starts=[len(b'package devcontrols\n')]) for x in controls]
for label,inputs in [('project',inp),('controls',control_inputs)]:
 raw=json.dumps(inputs,ensure_ascii=False).encode();p=subprocess.run([str(S/'type-audit')],input=raw,capture_output=True,timeout=60);write('types-'+label+'-receipt.json',dict(exit=p.returncode,stdin_sha256=g.sha(raw),stdout_sha256=g.sha(p.stdout),stderr_sha256=g.sha(p.stderr),binary_sha256=g.sha((S/'type-audit').read_bytes())));(out/('types-'+label+'.json.gz')).write_bytes(gzip.compress(p.stdout,mtime=0));(out/('types-'+label+'-stderr.txt')).write_bytes(p.stderr);p.check_returncode()
# Graph discovery uses a separate owned copy, never an operational index.
graph=S/'graph';shutil.copytree(prepared/'stablenet/retained-origins-v2/graph',graph)
def connect(label,cmd,namespace):
 p=out/label;p.mkdir();c=w.Client(None,None,p,command=cmd,namespace=namespace);c.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'wi1-source-matching','version':'1'}});write(label+'/tools.json',c.request('tools/list')['result']['tools']);return c
ckg=str(prepared/'bin/ckg');cks=str(prepared/'read-cks');calls=[];selected=[]
a=connect('graph-discovery',[ckg,'mcp','--graph',str(graph),'--namespace','wi1'],'wi1.context.')
try:
 for c in cs:
  src=c['source'];res=a.tool('find_symbol',{'name':src['name'],'exact':False});calls.append(dict(case_id=c['candidate_id'],operation='graph_find_symbol',result=res));nodes=[n for n in res['nodes'] if n['file']==src['path'] and n['line']==src['start_line'] and n.get('canonical_id')];assert len(nodes)==1,(c['candidate_id'],len(nodes),res)
  selected.append(dict(case_id=c['candidate_id'],node=nodes[0]))
finally:a.close();a.proc.stdout.close();write('graph-session.json',dict(exit=a.proc.returncode,requests=a.id))
write('discovery.json',calls);write('selected.json',selected)
b=connect('cks-project',[cks,'mcp','--config',str(prepared/'go-stablenet.yaml')],'cks.context.');results=[];negative=[];attempts=[];retry_count=0
try:
 for c,sel in zip(cs,selected):
  src=c['source'];args=dict(node_id=sel['node']['id'],expected_commit=g.SOURCE_COMMIT,expected_source_sha256=src['function_sha256']);r=b.tool('value_dependencies',args);assert len(r['snapshot_sha256'])==64;args['expected_snapshot_sha256']=r['snapshot_sha256'];r=b.tool('value_dependencies',args);attempts.append(dict(case_id=c['candidate_id'],arguments=dict(args),response=r))
  if r['analysis'] is None or r['analysis']['truncated']:
   retry_count+=1;r=b.tool('value_dependencies',args)
  assert r['analysis'] is not None and not r['analysis']['truncated'],(c['candidate_id'],r['status'])
  results.append(dict(case_id=c['candidate_id'],arguments=args,response=r));write('project-results.json',results);print('project',c['candidate_id'],r['status'],flush=True)
 for key in ['expected_source_sha256','expected_snapshot_sha256','expected_commit']:
  args=dict(results[0]['arguments']);args[key]='0'*len(args[key]);r=b.tool('value_dependencies',args);assert r['status']=='source_mismatch' and r['analysis'] is None;negative.append(dict(arguments=args,response=r))
finally:b.close();b.proc.stdout.close();write('cks-session.json',dict(exit=b.proc.returncode,requests=b.id))
write('pin-negatives.json',negative);write('project-attempts.json',attempts)
# Owned synthetic graph establishes special coordinates separately from project30.
repo=S/'controls-source';repo.mkdir()
for x in control_inputs:
 p=repo/x['Path'];p.parent.mkdir(parents=True,exist_ok=True);p.write_text(x['Source'])
(repo/'go.mod').write_text('module example.invalid/wi1-controls\n\ngo 1.26.8\n')
for command in [['git','init','-q'],['git','add','.'],['git','-c','user.name=WI1 owned fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','test: pin synthetic source coordinates']]:
 run=subprocess.run(command,cwd=repo,capture_output=True,timeout=30);assert run.returncode==0,run.stderr.decode()
write('control-source.json',dict(commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),files={str(p.relative_to(repo)):g.sha(p.read_bytes()) for p in repo.rglob('*') if p.is_file() and '.git' not in p.parts}))
cmd=[ckg,'build','--src',str(repo),'--out',str(S/'controls-graph'),'--lang','go'];run=subprocess.run(cmd,capture_output=True,timeout=120);write('control-build.json',dict(command=cmd,exit=run.returncode,stdout=run.stdout.decode(),stderr=run.stderr.decode()));run.check_returncode()
d=connect('controls',[ckg,'mcp','--graph',str(S/'controls-graph'),'--namespace','wi1'],'wi1.context.');cr=[]
try:
 for x in controls:
  res=d.tool('find_symbol',dict(name=x['name'],exact=False));assert len(res['nodes'])==1
  r=d.tool('value_dependencies',dict(node_id=res['nodes'][0]['id']));assert r['analysis'] is not None
  cr.append(dict(name=x['name'],response=r))
finally:d.close();d.proc.stdout.close();write('controls-session.json',dict(exit=d.proc.returncode,requests=d.id))
write('control-results.json',cr);after=inventory();write('after.json',after);assert before==after
write('receipt.json',dict(project_functions=len(results),synthetic_controls=len(cr),graph_binary_sha256=g.sha(pathlib.Path(ckg).read_bytes()),cks_binary_sha256=g.sha(pathlib.Path(cks).read_bytes()),original_retained_files_preserved=True,EOF_exit_codes=[a.proc.returncode,b.proc.returncode,d.proc.returncode],bounded_retry_calls=retry_count,initial_budget_exhausted_calls=sum(r['response']['status']=='budget_exhausted' for r in attempts),paid_model_calls=0,quality_scores=None,independent_approved=0))
