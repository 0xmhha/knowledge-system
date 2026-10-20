#!/usr/bin/env python3
"""Predeclared DEV19 warm observations, including partial/budget/error samples."""
import argparse
import importlib.util
import json
import os
import platform
import shutil
import time
import urllib.request
from pathlib import Path

import wi6_pair_contract as contract
import wi6_version_readiness as preparation

ROOT=preparation.ROOT
PACKET=ROOT/preparation.BASE/'wi6-warm-runtime-v1'
spec=importlib.util.spec_from_file_location('pilot_transport',ROOT/'scripts/run-wi6-pair-pilot.py')
pilot=importlib.util.module_from_spec(spec);spec.loader.exec_module(pilot)


def environment():
    result=dict(platform=platform.platform(),machine=platform.machine(),processor=platform.processor(),
                cpu_count=os.cpu_count(),load_average=list(os.getloadavg()),
                clock=time.get_clock_info('perf_counter')._asdict() if hasattr(time.get_clock_info('perf_counter'),'_asdict') else str(time.get_clock_info('perf_counter')),
                resource_exclusivity_proved=False)
    models={}
    for label,endpoint in [('version','version'),('tags','tags')]:
        try:
            with urllib.request.urlopen('http://127.0.0.1:11434/api/'+endpoint,timeout=5)as response:
                models[label]=json.load(response)
        except Exception as exc:
            models[label]=dict(available=False,error_type=type(exc).__name__)
    result['local_embedding_runtime_readonly_metadata']=models
    return result


def schedule(cases):
    return [dict(case_id=case['case_id'],phase=phase,index=i,
                 order=['baseline','expansion']if i%2==0 else['expansion','baseline'])
            for case in cases for phase,n in [('warmup',3),('measured',30)]for i in range(n)]


def run(work):
    work.mkdir();scope=json.loads((PACKET/'SCOPE.json').read_text());pilot.write(work/'SCOPE.json',scope)
    for name,h in scope['tools_sha256'].items():
        if contract.sha((ROOT/name).read_bytes())!=h:raise ValueError('frozen warm tool differs')
    if contract.sha(pilot.BINARY.read_bytes())!=scope['binary_sha256']:raise ValueError('native binary differs')
    before=pilot.inventory(pilot.DATA);pilot.write(work/'original-before.json',before)
    if before!=scope['original_data_sha256']:raise ValueError('predeclared data differs')
    shutil.copytree(pilot.DATA,work/'data',ignore=lambda d,names:[n for n in names if n.endswith(('.lock','.lease'))])
    contract.restore_reader_coordination(work/'data');owned_before=pilot.inventory(work/'data')
    if owned_before!=before:raise ValueError('owned copy differs')
    pilot.write(work/'owned-before.json',owned_before);pilot.write(work/'environment-before.json',environment())
    live=work/'empty-live-source';live.mkdir()
    config=work/'config.yaml';template=(ROOT/preparation.BASE/'wi6-paired-runtime-v1/actual/config.yaml.gz')
    import gzip
    old=gzip.decompress(template.read_bytes()).decode();old=old.replace('/private/tmp/ks-wi6-pair-pilot-corrective-20261019',str(work));config.write_text(old)
    client=pilot.Client(work,config);receipts=(work/'calls.jsonl').open('w');pairs=(work/'pairs.jsonl').open('w')
    count=completed=0
    def call(case_id,phase,arm,args,index=None):
        nonlocal count
        name='value_dependencies'if arm=='baseline'else'value_dependency_expansion'
        response=client.request('tools/call',dict(name='cks.context.'+name,arguments=args));count+=1
        record=dict(case_id=case_id,phase=phase,index=index,arm=arm,request=response['request'],elapsed_ns=response['elapsed_ns'])
        receipts.write(json.dumps(record,separators=(',',':'))+'\n');receipts.flush()
        value,encoding=contract.unpack(response['raw_response'])
        surface=value if arm=='baseline'else(value or{}).get('graph',{})
        return dict(request_id=response['request']['id'],elapsed_ns=response['elapsed_ns'],status=(surface or{}).get('status',encoding))
    try:
        pilot.write(work/'initialize.json',client.request('initialize',dict(protocolVersion='2024-11-05',capabilities={},clientInfo=dict(name='owned-wi6-warm',version='1'))))
        pilot.write(work/'tools.json',client.request('tools/list'))
        for case in scope['cases']:
            cid=case['case_id'];pins=case['pins'];request=case['requests'];probe=dict(request['baseline']);probe.pop('expected_snapshot_sha256')
            observed=client.request('tools/call',dict(name='cks.context.value_dependencies',arguments=probe))
            count+=1;record=dict(case_id=cid,phase='discovery_probe',index=None,arm='baseline',request=observed['request'],elapsed_ns=observed['elapsed_ns'])
            receipts.write(json.dumps(record,separators=(',',':'))+'\n');receipts.flush()
            value,_=contract.unpack(observed['raw_response'])
            if not value or value.get('snapshot_sha256')!=pins['snapshot_sha256']:raise ValueError('actual snapshot differs from retained pilot pin')
            for entry in schedule([case]):
                row=dict(entry,arms={})
                for arm in entry['order']:row['arms'][arm]=call(cid,entry['phase'],arm,request[arm],entry['index'])
                pairs.write(json.dumps(row,separators=(',',':'))+'\n');pairs.flush();completed+=1
                if entry['index']%5==0 or entry['index']==29:
                    print(json.dumps(dict(case_id=cid,phase=entry['phase'],completed_index=entry['index'],total_pairs=completed)),flush=True)
    finally:
        receipts.close();pairs.close();client.close()
        pilot.write(work/'environment-after.json',environment());pilot.write(work/'owned-after.json',pilot.inventory(work/'data'));pilot.write(work/'original-after.json',pilot.inventory(pilot.DATA))
    if pilot.inventory(pilot.DATA)!=before or pilot.inventory(work/'data')!=before or list(live.iterdir()):raise ValueError('original/owned payload or live source changed')
    if contract.sha(pilot.BINARY.read_bytes())!=scope['binary_sha256']:raise ValueError('binary changed after warm run')
    pilot.write(work/'receipt.json',dict(planned_pairs=627,completed_pairs=completed,tool_calls=count,warmup_pairs=57,measured_pairs=570,
                automatic_retries=0,binary_sha256=scope['binary_sha256'],original_owned_payload_preserved=True,
                independent_approved=0,formal_completed=0,formal_total=9,quality_scores=None,paid_or_external_LLM_calls=0,embedding_inference_call_count=None))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
