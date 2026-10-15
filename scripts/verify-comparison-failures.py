#!/usr/bin/env python3
"""Native comparison partial failures, response bounds and signal shutdown."""
import argparse
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import stat
import subprocess
import sys
import time
from datetime import datetime,timezone

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    def write(name,value):(out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    with(out/'baseline-verifier.txt').open('wb')as log:subprocess.run([sys.executable,str(ROOT/'scripts/verify-comparison-MCP.py'),'--out',str(out/'baseline')],stdout=log,stderr=subprocess.STDOUT,check=True)
    base=out/'baseline';fixtures=json.loads((base/'fixtures.json').read_text());checks=[];calls=[];clients=[]
    def check(label,value):manager.require(value,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    def inventory():
        files=[base/'profile.json',base/'catalog.json',base/'bindings.json']
        for f in fixtures:
            v=Path(f['version']);manager.require(v.is_relative_to(out),'fixture_escape');files.extend(p for p in(v/'sources').rglob('*')if p.is_file());files.extend(v/name for name in('graph/graph.db','graph/manifest.json','vector/vector.db','vector/manifest.json'))
        return{p.relative_to(out).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()for p in files}
    before=inventory();write('before.json',before)
    spec=importlib.util.spec_from_file_location('comparison_failure_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)
    def connect(label):
        d=out/label;d.mkdir();client=wire.Client(None,None,d,command=[str(ROOT/'bin/cks'),'mcp','comparison','--profile',str(base/'profile.json')],namespace='');clients.append(client);client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-comparison-failures','version':'1'}});return client
    def tool(c,operation,args=None):
        response=c.request('tools/call',{'name':'cks.comparison.'+operation,'arguments':args or{}})['result'];calls.append({'session':c.root.name,'tool':'cks.comparison.'+operation,'arguments':copy.deepcopy(args or{}),'response':copy.deepcopy(response)});write('calls.json',calls);manager.require(not response.get('isError'),'actual tool refusal: '+str(response));return response['structuredContent']
    c=connect('failure-session')
    try:
        d=tool(c,'describe');ids=[next(s['scope_id']for s in d['scopes']if s['scope']['coordinates']==f['coordinates'])for f in fixtures];key=next(x['key']for x in d['concepts']if x['key']['concept_id']=='staking')
        q={'schema_version':1,'profile_digest':d['profile_digest'],'catalog_digest':d['catalog_digest'],'collection_id':'chains','mode':'comparison','participants':ids,'concept':key,'facets':['entry','exit'],'as_of':datetime.now(timezone.utc).isoformat(timespec='seconds'),'budget':{'max_body_bytes':262144,'max_citations':128,'timeout_ms':30000},'include_proposed':True}
        original=tool(c,'extract',q);write('before-matrix.json',original);check('baseline includes cell and candidate citation objects',original['used_citations']==8 and original['omitted_candidates']==0 and original['candidate_state']=='complete')
        manifest=Path(fixtures[1]['version'])/'sources/manifest.json';data=manifest.read_bytes();mode=stat.S_IMODE(manifest.stat().st_mode)
        try:
            manifest.unlink();missing=tool(c,'extract',q);write('missing-B.json',missing);check('missing selected B retains A and exact participants without candidates',missing['groups'][0]==original['groups'][0]and missing['groups'][1]['participant']==original['groups'][1]['participant']and all(cell['state']=='unavailable'and not cell['mappings']for cell in missing['groups'][1]['cells'])and not missing['candidates']and missing['used_citations']==2 and missing['state']=='partial')
        finally:manifest.write_bytes(data);manifest.chmod(mode)
        restored=tool(c,'extract',q);write('restored-B.json',restored);check('B exact source restoration restores full matrix',restored==original)
        a=Path(fixtures[0]['version']);archive=json.loads((a/'sources/manifest.json').read_text());record=next(r for r in archive['files']if r['origin_id']=='repo'and r['path']=='stake.go');blob=a/'sources/blobs'/record['sha256'];data=blob.read_bytes();mode=stat.S_IMODE(blob.stat().st_mode)
        try:
            blob.chmod(0o600);blob.write_bytes(data+b'// controlled corruption\n');corrupt=tool(c,'extract',q);write('corrupt-A.json',corrupt);check('corrupted A never leaks earlier evidence or affects B',all(cell['state']=='unavailable'and not cell['mappings']for cell in corrupt['groups'][0]['cells'])and corrupt['groups'][1]==original['groups'][1]and not corrupt['candidates']and corrupt['used_citations']==2)
        finally:blob.write_bytes(data);blob.chmod(mode)
        restored=tool(c,'extract',q);write('restored-A.json',restored);check('A exact source restoration restores full matrix',restored==original)
        small=copy.deepcopy(q);small['budget']['max_citations']=4;limited=tool(c,'extract',small);write('candidate-budget.json',limited);check('candidate citations share explicit response limit and omit whole candidate',limited['used_citations']==4 and len(limited['groups'][0]['cells'][0]['mappings'])==len(limited['groups'][1]['cells'][0]['mappings'])==1 and limited['omitted_candidates']==1 and limited['candidate_state']=='budget_exceeded'and not limited['candidates'])
        small['budget']['max_citations']=3;limited=tool(c,'extract',small);write('cell-budget.json',limited);check('whole B cell omitted when source citations cannot fit',limited['used_citations']==2 and limited['groups'][0]['cells'][0]['state']=='similar'and limited['groups'][1]['cells'][0]['state']=='budget_exceeded'and not limited['groups'][1]['cells'][0]['mappings']and not limited['candidates'])
    finally:c.close();c.proc.stdout.close()
    c=connect('signal-session')
    try:
        tool(c,'describe');manager.require(not c.proc.stdin.closed,'input_was_closed_before_signal');start=time.monotonic();c.proc.terminate();c.proc.wait(timeout=10);elapsed=time.monotonic()-start;write('signal.json',{'input_open_before_SIGTERM':True,'exit_code':c.proc.returncode,'elapsed_seconds':elapsed});check('SIGTERM with input open closes comparison process normally',c.proc.returncode==0 and elapsed<10)
    finally:c.close();c.proc.stdout.close()
    after=inventory();write('after.json',after);check('owned corruption is restored and DB profile catalog bindings remain byte identical',before==after)
    baseline=json.loads((base/'summary.json').read_text());write('summary.json',{'schema_version':1,'checks':len(checks),'baseline_checks':baseline['checks'],'actual_tool_calls':len(calls),'baseline_actual_tool_calls':baseline['actual_tool_calls'],'actual_MCP_sessions':len(clients)+baseline['actual_MCP_sessions'],'EOF_exit_codes':baseline['EOF_exit_codes']+[clients[0].proc.returncode],'SIGTERM_exit_code':clients[1].proc.returncode,'actual_native_dataset_builds':baseline['actual_native_dataset_builds'],'paid_model_calls':0,'semantic_quality_or_independent_acceptance':False,'full_KM06_complete':False,'scope':'KM06_3_actual_owned_source_partial_failure_budget_signal_not_real_chain_quality'})

if __name__=='__main__':main()
