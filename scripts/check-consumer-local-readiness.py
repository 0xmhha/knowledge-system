#!/usr/bin/env python3
"""Verify local candidate evidence without granting adoption or role acceptance."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha, verify

ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('consumer_local_prepare',ROOT/'scripts/prepare-consumer-local-readiness.py')
prepare=importlib.util.module_from_spec(spec);spec.loader.exec_module(prepare)


def check(packet,check_references=True):
    errors=[]
    def need(condition,message):
        if not condition:errors.append(message)
    try:
        m=json.loads((packet/'manifest.json').read_text())
        need(m['kind']=='consumer_local_P01_P04_candidate_not_adopted_or_host_accepted','packet scope')
        need(set(m['input_bindings'])=={str(p) for p in prepare.bindings()},'input inventory')
        for p,digest in m['input_bindings'].items():need(sha(safe(ROOT,p).read_bytes())==digest,'input binding: '+p)
        raw=(ROOT/'scripts/prepare-consumer-local-readiness.py').read_bytes()
        need(m['generator_sha256']==sha(raw) and (packet/'generator.py.txt').read_bytes()==raw,'generator binding')
        if errors:return errors
        expected=prepare.generate(packet)
        proof=verify(packet/'candidate-source',m['candidate_commit'])
        proof_paths={'candidate-source/git-proof.json'}|{'candidate-source/'+p for p in proof['objects'].values()}|{'candidate-source/'+r['artifact'] for r in proof['files'].values()}
        fixed={'execution.json','candidate.bundle','candidate.patch','build-receipts.json','formal-state.json','generator.py.txt','.gitattributes'}
        fixed|={'logs/'+n+'.txt' for n in prepare.TESTS}|{'logs/signature.txt'}
        fixed|={'build-logs/build-'+n+s for n in ['cks','ckg'] for s in ['.stdout.txt','.stderr.txt','.info.txt']}
        inventory={p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name!='manifest.json'}
        need(inventory==set(m['artifacts'])==set(expected)|proof_paths|fixed,'artifact inventory')
        for p,digest in m['artifacts'].items():need(sha(safe(packet,p).read_bytes())==digest,'artifact hash: '+p)
        for name,value in expected.items():
            actual=(packet/name).read_text() if isinstance(value,str) else json.loads((packet/name).read_text())
            need(actual==value,'local readiness accounting: '+name)
        need(m['counts']=={'changed_files':3,'role_tool_entries':7,'local_tests':45,'local_plan_cases_passed':6,
          'planned_cases_pending':16,'actual_role_sessions':0,'new_mcp_sessions':0,'formal_runs':0},'manifest counts')
        need((packet/'.gitattributes').read_bytes()==prepare.ATTRIBUTES,'binary object preservation')
        formal=json.loads((packet/'formal-state.json').read_text())
        need(formal==prepare.load('prepare-source-review-supplement.py').formal_state(ROOT),'formal state changed')
        need(formal['freeze'] is None and formal['execution_allowed'] is False,'formal execution promotion')
        if check_references:
            ref={'manifest':(packet/'manifest.json').relative_to(ROOT).as_posix(),'sha256':sha((packet/'manifest.json').read_bytes()),
              'status':'consumer_P01_P04_local_candidate_not_adopted_or_actual_role_accepted'}
            for name in ['PROTOCOL','PROGRESS']:
                d=json.loads((ROOT/f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                need(d.get('consumer_local_readiness')==ref,'ledger reference: '+name)
    except (OSError,ValueError,KeyError,TypeError,IndexError,subprocess.SubprocessError) as e:
        errors.append('invalid local readiness: '+str(e))
    return errors


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--packet',type=Path,required=True);p.add_argument('--no-references',action='store_true')
    a=p.parse_args();errors=check(a.packet.resolve(),not a.no_references)
    for error in errors:print(error)
    if not errors:print('Local P01-P04 candidate verified; stdio/actual roles/render/lifecycle remain pending')
    raise SystemExit(bool(errors))
