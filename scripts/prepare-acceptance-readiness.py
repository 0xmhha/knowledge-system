#!/usr/bin/env python3
"""Refresh isolated, cost-free metadata; never freeze or accept the pilot."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
from datetime import datetime,timezone
import urllib.request
ROOT=Path(__file__).resolve().parents[1]
MODEL='bge-m3:latest'
DIGEST='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
def sha(data):return hashlib.sha256(data).hexdigest()
def require(value,message):
 if not value:raise ValueError(message)
def main():
 ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--out',type=Path,required=True);args=ap.parse_args();out=args.out.resolve();require(not out.exists(),'fresh owned output required');out.mkdir(parents=True)
 env=dict(os.environ,GOTOOLCHAIN='go1.26.9',GOCACHE='/private/tmp/ks-corpus-go-cache',GOPROXY='off',GOSUMDB='off',PYTHONDONTWRITEBYTECODE='1');commands=[]
 def write(name,value):
  path=out/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
 def run(label,argv,cwd=ROOT,environment=env,timeout=120):
  r=subprocess.run(argv,cwd=cwd,env=environment,capture_output=True,timeout=timeout);(out/(label+'.stdout.txt')).write_bytes(r.stdout);(out/(label+'.stderr.txt')).write_bytes(r.stderr);commands.append({'label':label,'argv':argv,'cwd':str(cwd),'exit_code':r.returncode,'stdout_sha256':sha(r.stdout),'stderr_sha256':sha(r.stderr)});write('commands.json',commands);require(r.returncode==0,'command failed: '+label);return r.stdout.decode().strip()
 goroot=run('toolchain-root',['go','env','GOROOT'],environment=dict(os.environ,GOTOOLCHAIN='go1.26.9',GOCACHE=env['GOCACHE'],GOPROXY='off'))
 env.update(GOTOOLCHAIN='local',GOROOT=goroot,PATH=str(Path(goroot)/'bin')+os.pathsep+env['PATH'])
 write('build-environment.json',{k:env[k]for k in ('GOTOOLCHAIN','GOROOT','GOCACHE','GOPROXY','GOSUMDB')})
 revision=run('knowledge-head',['git','rev-parse','HEAD']);source=out/'knowledge-source';run('clone',['git','clone','--shared','--no-checkout','--quiet',str(ROOT),str(source)]);run('sparse',['git','sparse-checkout','set','cmd','internal','pkg'],source);run('checkout',['git','checkout','--quiet','--detach',revision],source)
 require(run('source-status',['git','status','--porcelain'],source)=='','owned source not clean');tree=run('source-tree',['git','rev-parse','HEAD^{tree}'],source)
 prior=ROOT/'system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration';binding=json.loads((prior/'source-binding.json').read_text());maintained={n:h for n,h in binding['source_sha256'].items()if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))};require(len(maintained)==1275 and all(sha((source/n).read_bytes())==h for n,h in maintained.items()),'prepared maintained Go differs from last tested input')
 binary=out/'cks';run('build',['go','build','-o',str(binary),'./cmd/cks'],source,timeout=300);buildinfo=run('buildinfo',['go','version','-m',str(binary)]);require('vcs.revision='+revision in buildinfo and 'vcs.modified=false'in buildinfo,'fresh CKS provenance missing')
 host_packet=ROOT/'system/eval/operational-preparation-2026-10-15/isolated-host';host=json.loads((host_packet/'manifest.json').read_text());repos={}
 for key in('coding_agent','chainbench'):
  adopted=host['source_adoptions'][key];path=Path(adopted['source_root']);head=run(key+'-head',['git','rev-parse','HEAD'],path);status=run(key+'-status',['git','status','--porcelain','--untracked-files=no'],path);require(head==adopted['source_commit']and not status,'adopted source drifted')
  rows=json.loads((host_packet/(key+'-source-files.json')).read_text());require(all(sha((path/row['path']).read_bytes())==row['sha256']for row in rows),'adopted tracked bytes differ');repos[key]={'source_path':str(path),'commit':head,'verified_tracked_files':len(rows),'tracked_status':status,'adoption_scope':'owned_local_session_only'}
 consumer_path=Path(repos['coding_agent']['source_path'])/'plugin/contracts/cks-tools.schema.json';consumer=json.loads(consumer_path.read_text())['providers']['cks']['tools'];write('consumer-tools.json',consumer)
 claude=Path('/Users/kevin/.local/bin/claude').resolve();hostenv=dict(env,CLAUDE_CONFIG_DIR=str(out/'empty-host-config'));(out/'empty-host-config').mkdir();version=run('claude-version',[str(claude),'--version'],environment=hostenv)
 # Metadata GETs only. No embedding, model generation, or external provider call.
 for label,endpoint in(('ollama-version','version'),('ollama-tags','tags')):
  request=urllib.request.Request('http://127.0.0.1:11434/api/'+endpoint,method='GET')
  with urllib.request.urlopen(request,timeout=10)as response:data=response.read(1024*1024+1)
  require(len(data)<=1024*1024,'model metadata too large');(out/(label+'.json')).write_bytes(data)
 tags=json.loads((out/'ollama-tags.json').read_text());model=next(m for m in tags['models']if m['name']==MODEL);require(model['digest']==DIGEST,'approved model bytes changed')
 old=Path('/private/tmp/ks-policy-regeneration-current-20261016/cks');graph=Path('/private/tmp/ks-policy-regeneration-current-20261016/graph/graph.db');rules=out/'sanitization_rules.yaml';rules.write_bytes((old/'sanitization_rules.yaml').read_bytes());config=out/'cks.yaml';text=(old/'cks.yaml').read_text().replace(str(old/'sanitization_rules.yaml'),str(rules));config.write_text(text)
 original={str(p):sha(p.read_bytes())for p in(graph,rules,config,binary)};write('before.json',original)
 spec=importlib.util.spec_from_file_location('readiness_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire);sessions=[];tools={}
 chainbinary=Path(host['source_adoptions']['chainbench']['binary']);require(sha(chainbinary.read_bytes())==host['source_adoptions']['chainbench']['binary_sha256'],'ChainBench binary drifted');run('chainbench-buildinfo',['go','version','-m',str(chainbinary)])
 for key,command in(('cks',[str(binary),'mcp','--config',str(config)]),('chainbench',['/usr/bin/env','CHAINBENCH_DATA_ROOT='+str(out/'owned-chain-state'),'CHAINBENCH_DIR='+repos['chainbench']['source_path'],str(chainbinary)])):
  directory=out/(key+'-session');directory.mkdir();client=wire.Client(None,None,directory,command=command,namespace='');sessions.append(client)
  try:
   client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'acceptance-metadata-preparation','version':'1'}});tools[key]=client.request('tools/list')['result']['tools'];write(key+'-tools.json',tools[key])
  finally:client.close();client.proc.stdout.close()
 producer={t['name']:t for t in tools['cks']};missing=[];differences=[]
 for alias,value in consumer.items():
  if value['wire_name']not in producer:missing.append(alias)
  elif value['input']!=producer[value['wire_name']]['inputSchema']:differences.append(alias)
 extra=sorted(set(producer)-{v['wire_name']for v in consumer.values()});write('schema-parity.json',{'consumer_aliases':len(consumer),'producer_tools':len(producer),'missing_aliases':missing,'input_schema_differences':differences,'extra_producer_tools':extra,'comparison':'exact parsed JSON input schemas; alias -> explicit wire_name','end_to_end_agent_dispatch_verified':False})
 after={str(p):sha(p.read_bytes())for p in(graph,rules,config,binary)};write('after.json',after);require(original==after,'metadata preflight mutated retained inputs')
 write('runtime.json',{'observed_at':datetime.now(timezone.utc).isoformat(timespec='seconds'),'knowledge_source_commit':revision,'knowledge_source_tree':tree,'prepared_source_status':'clean','maintained_Go_files_equal_last_tested':len(maintained),'knowledge_binary_sha256':sha(binary.read_bytes()),'consumer_schema_sha256':sha(consumer_path.read_bytes()),'repositories':repos,'host':{'path':str(claude),'version':version,'sha256':sha(claude.read_bytes()),'agent_session_started':False,'personal_authentication_inspected':False},'model':{'name':MODEL,'digest':DIGEST,'server_version':json.loads((out/'ollama-version.json').read_text())['version'],'embedding_dimensions':'not_remeasured_metadata_only'},'tools_counts':{k:len(v)for k,v in tools.items()},'EOF_exit_codes':[c.proc.returncode for c in sessions],'MCP_sessions':len(sessions),'MCP_tool_calls':0,'metadata_requests':4,'actual_model_calls':0,'paid_model_calls':0,'real_chain_nodes_started':0,'CKV':'mock; tools metadata only; no retrieval or embedding execution','HTTP_consumer_transport_tested':False,'full_agent_dispatch_tested':False,'independent_acceptance':False,'protocol_frozen':False,'operational_execution_allowed':False})
 require(not missing and not differences and not extra,'schema parity drift: inspect schema-parity.json')
 print('PASS: committed source build; adopted sources; current model/CLI metadata; native tools schemas; two EOF0; no operational acceptance')
if __name__=='__main__':main()
