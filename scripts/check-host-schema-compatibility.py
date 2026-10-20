#!/usr/bin/env python3
"""Recompute isolated source, native comparison and actual role evidence."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess
from source_packet import safe
ROOT=Path(__file__).resolve().parents[1]
s=importlib.util.spec_from_file_location('host_compat_prepare',ROOT/'scripts/prepare-host-schema-compatibility.py');prepare=importlib.util.module_from_spec(s);s.loader.exec_module(prepare)
sha=prepare.sha


def check(packet,references=True):
    errors=[]
    def need(value,message):
        if not value:errors.append(message)
    try:
        m=prepare.read(packet/'manifest.json');need(m['kind']=='isolated_host_schema_compatibility_P06_not_adoption_or_acceptance','packet scope')
        need(set(m['input_bindings'])=={str(p)for p in prepare.INPUTS},'input inventory')
        for p,digest in m['input_bindings'].items():need(sha(safe(ROOT,p))==digest,'input changed: '+p)
        raw=ROOT/'scripts/prepare-host-schema-compatibility.py';need(m['generator_sha256']==sha(raw)and(packet/'generator.py.txt').read_bytes()==raw.read_bytes(),'generator binding')
        inventory={p.relative_to(packet).as_posix()for p in packet.rglob('*')if p.is_file()}
        need(inventory==set(m['artifacts'])|{'manifest.json'},'artifact inventory')
        for p,digest in m['artifacts'].items():need(sha(safe(packet,p))==digest,'artifact changed: '+p)
        if errors:return errors
        for name,value in prepare.derive(packet).items():
            actual=(packet/name).read_text()if isinstance(value,str)else prepare.read(packet/name);need(actual==value,'role accounting: '+name)
        for name,fn in [('source-verification.json',prepare.verify_sources),('matrix-verification.json',prepare.verify_matrix),('validation-verification.json',prepare.verify_validation)]:need(prepare.read(packet/name)==fn(packet),'derived proof: '+name)
        need(prepare.read(packet/'preservation.json')==prepare.preservation(),'prior frozen preservation')
        pins=prepare.read(packet/'binary-pins.json');e=prepare.read(packet/'execution.json');need(pins['cks_commit']==e['cks_commit']and pins['consumer_commit']==e['consumer_commit'],'binary/source commit association')
        need(e['prior_candidate_postrun']=={'commit':prepare.CONSUMER_BASE,'porcelain':''},'prior candidate changed')
        formal=prepare.read(packet/'formal-state.json');need(formal==prepare.load('prepare-source-review-supplement.py').formal_state(ROOT),'formal state changed')
        need(formal['freeze']is None and formal['execution_allowed']is False,'formal execution promoted')
        if references:
            ref={'manifest':(packet/'manifest.json').relative_to(ROOT).as_posix(),'sha256':sha(packet/'manifest.json'),'status':'isolated_P06_7_of_7_not_adopted_plan15_of22'}
            for n in ['PROTOCOL','PROGRESS']:
                d=prepare.read(ROOT/f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{n}.json');need(d.get('host_schema_compatibility')==ref,'ledger reference: '+n)
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,subprocess.SubprocessError)as e:errors.append('invalid compatibility evidence: '+str(e))
    return errors


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--packet',type=Path,required=True);p.add_argument('--no-references',action='store_true');a=p.parse_args();errors=check(a.packet.resolve(),not a.no_references)
    for e in errors:print(e)
    if not errors:print('Verified isolated P06 actual7/7, plan15/22, formal0/9; no adoption')
    raise SystemExit(bool(errors))
