#!/usr/bin/env python3
"""Verify actual role receipts without hiding missing host tools or promoting P06."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess
from source_packet import safe
ROOT=Path(__file__).resolve().parents[1]
s=importlib.util.spec_from_file_location('role_prepare',ROOT/'scripts/prepare-consumer-role-readiness.py');prepare=importlib.util.module_from_spec(s);s.loader.exec_module(prepare)
sha=prepare.sha


def check(packet,references=True):
    errors=[]
    def need(v,s):
        if not v:errors.append(s)
    try:
        m=json.loads((packet/'manifest.json').read_text());need(m['kind']=='actual_role_partial_P06_host_admission_failure_not_acceptance','scope')
        need(set(m['input_bindings'])=={str(p)for p in prepare.INPUTS},'input inventory')
        for p,digest in m['input_bindings'].items():need(sha(safe(ROOT,p))==digest,'input binding: '+p)
        raw=ROOT/'scripts/prepare-consumer-role-readiness.py';need(m['generator_sha256']==sha(raw)and(packet/'generator.py.txt').read_bytes()==raw.read_bytes(),'generator binding')
        execution=json.loads((packet/'execution.json').read_text());fixed={'manifest.json','execution.json','diagnosis.json','formal-state.json','generator.py.txt','case-driver.py.txt','COMPATIBILITY-FOLLOWUP.txt','role-readiness.json','ROLE-READINESS-REPORT.txt'}
        fixed|={'helper-versions/'+c+'.py.txt'for c in prepare.CASES[:3]}
        allowed={'input-bundle.json','scripted-judgments.json','state.json','deep-review-evidence.json','deep-review.json','review-report.md.txt','sequence.mmd','modules.mmd','adapter-commands.json','generation1-state.json','owned-report-backup.txt','shell-tamper-receipt.json','shell-restore-receipt.json'}
        for case,paths in execution['run_inventory'].items():
            need(case in prepare.CASES,'unexpected case')
            need(all(p in prepare.TOP or(p.startswith('workspace-after/')and p.split('/',1)[1]in allowed)for p in paths),'unexpected run artifact')
            fixed|={'runs/'+case+'/'+p for p in paths}
        inventory={p.relative_to(packet).as_posix()for p in packet.rglob('*')if p.is_file()}
        need(inventory==fixed and set(m['artifacts'])==fixed-{'manifest.json'},'artifact inventory')
        for p,digest in m['artifacts'].items():need(sha(safe(packet,p))==digest,'artifact hash: '+p)
        if errors:return errors
        for name,value in prepare.derive(packet).items():
            actual=(packet/name).read_text()if isinstance(value,str)else json.loads((packet/name).read_text());need(actual==value,'role accounting: '+name)
        need((packet/'COMPATIBILITY-FOLLOWUP.txt').read_text()==prepare.FOLLOWUP,'compatibility followup scope')
        formal=json.loads((packet/'formal-state.json').read_text());need(formal==prepare.load('prepare-source-review-supplement.py').formal_state(ROOT),'formal state changed')
        need(formal['freeze']is None and formal['execution_allowed']is False,'formal execution promotion')
        if references:
            ref={'manifest':(packet/'manifest.json').relative_to(ROOT).as_posix(),'sha256':sha(packet/'manifest.json'),'status':'P06_partial_5_of_7_pairs_two_host_schema_admission_failures'}
            for n in ['PROTOCOL','PROGRESS']:
                d=json.loads((ROOT/f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{n}.json').read_text());need(d.get('consumer_role_readiness')==ref,'ledger reference: '+n)
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,subprocess.SubprocessError)as e:errors.append('invalid role evidence: '+str(e))
    return errors


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--packet',type=Path,required=True);p.add_argument('--no-references',action='store_true');a=p.parse_args();errors=check(a.packet.resolve(),not a.no_references)
    for e in errors:print(e)
    if not errors:print('Actual role evidence verified; P06 incomplete (5/7 pairs), formal0/9')
    raise SystemExit(bool(errors))
