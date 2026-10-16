#!/usr/bin/env python3
"""Recheck the installed host with scripted local responses, not real models."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
ROOT=Path(__file__).resolve().parents[1]
def require(value,message):
 if not value:raise ValueError(message)
def main():
 ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--readiness',type=Path,required=True);ap.add_argument('--out',type=Path,required=True);ap.add_argument('--prefix',required=True);args=ap.parse_args();runtime=json.loads((args.readiness/'runtime.json').read_text());out=args.out.resolve();require(not out.exists(),'fresh report output required');out.mkdir(parents=True)
 adopted=Path(runtime['repositories']['coding_agent']['source_path']).parent;claude=runtime['host']['path'];seed=adopted/'attempts/valid-02/host.stdout.jsonl';init=next(json.loads(x)for x in seed.read_text().splitlines()if json.loads(x).get('subtype')=='init');old_aliases={n for n in init['tools']if n.startswith('mcp__')};fresh=json.loads((args.readiness/'chainbench-tools.json').read_text());require(old_aliases=={'mcp__plugin_coding-agent_chainbench__'+t['name'].replace('.','_')for t in fresh},'prior permission seed differs from fresh native tool list')
 (out/'permission-seed-provenance.json').write_text(json.dumps({'kind':'previous_actual_host_init_for_denylist_only_not_new_case','path':str(seed),'old_host_version':init['claude_code_version'],'current_native_tool_count':len(fresh),'aliases_equal_fresh_metadata':True,'host_alias_normalization':'dots_to_underscores; exact alias set equality required'},indent=2)+'\n')
 spec=importlib.util.spec_from_file_location('readiness_host_assess',ROOT/'scripts/operational-host-probe.py');probe=importlib.util.module_from_spec(spec);spec.loader.exec_module(probe);records=[]
 for case in('valid','tools','incomplete','tampered','cancel'):
  attempt=args.prefix+'-'+case;command=['python3',str(ROOT/'scripts/operational-host-probe.py'),'--root',str(adopted),'--case',case,'--attempt',attempt,'--claude',claude];r=subprocess.run(command,cwd=ROOT,env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1'),capture_output=True,timeout=180);(out/(case+'.stdout.txt')).write_bytes(r.stdout);(out/(case+'.stderr.txt')).write_bytes(r.stderr);require(r.returncode==0,'host fixture command failed: '+case);folder=adopted/'attempts'/attempt;result=probe.assess(folder);record={'attempt':attempt,'folder':str(folder),'command':command,'exit_code':r.returncode,'assessment':result};records.append(record);(out/'cases.json').write_text(json.dumps(records,indent=2)+'\n');require(result['status']=='PASS'and result['actual_host_version']==runtime['host']['version'].split()[0],'new host behavior differs: '+str(result));print('PASS: current host '+case,flush=True)
 for record in records:
  frames=[json.loads(x)for x in(Path(record['folder'])/'mcp-frames.jsonl').read_text().splitlines()];start=next(x for x in frames if x['kind']=='start')
  for key in('wrapper_pid','child_pid'):
   try:os.kill(start[key],0)
   except ProcessLookupError:continue
   raise ValueError('owned MCP process still present')
 (out/'summary.json').write_text(json.dumps({'kind':'actual_updated_host_scripted_local_transport_only','host_version':runtime['host']['version'],'cases_passed':len(records),'MCP_calls':sum(r['assessment']['mcp_calls']for r in records),'gateway_requests':sum(r['assessment']['gateway_requests']for r in records),'server_exit_codes':[r['assessment']['server_exit']for r in records],'owned_MCP_processes_absent':True,'paid_provider_calls':0,'actual_upstream_model_inference':False,'full_agent_role_pipeline_executed':False,'real_chain_nodes_started':0,'independent_acceptance':False},indent=2)+'\n')
if __name__=='__main__':main()
