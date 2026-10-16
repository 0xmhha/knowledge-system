#!/usr/bin/env python3
"""Validate the consumer specification without reporting planned tests as run."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha

ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('consumer_plan_prepare',ROOT/'scripts/prepare-consumer-contract-plan.py')
prepare=importlib.util.module_from_spec(spec);spec.loader.exec_module(prepare)


def check(packet,check_references=True):
    errors=[]

    def require(condition,message):
        if not condition:errors.append(message)

    try:
        m=json.loads((packet/'manifest.json').read_text())
        require(m['kind']=='consumer_contract_specification_and_plan_only','packet scope')
        require(set(m['input_bindings'])=={str(p) for p in prepare.bindings()},'input binding inventory')
        for p,digest in m['input_bindings'].items():require(sha(safe(ROOT,p).read_bytes())==digest,'input binding: '+p)
        raw=(ROOT/'scripts/prepare-consumer-contract-plan.py').read_bytes()
        require(m['generator_sha256']==sha(raw) and (packet/'generator.py.txt').read_bytes()==raw,'generator binding')
        if errors:return errors
        expected=prepare.generate()
        inventory={p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name!='manifest.json'}
        require(inventory==set(m['artifacts'])==set(expected)|{'formal-state.json','generator.py.txt'},'artifact inventory')
        for p,digest in m['artifacts'].items():require(sha(safe(packet,p).read_bytes())==digest,'artifact hash: '+p)
        for name,value in expected.items():
            actual=(packet/name).read_text() if isinstance(value,str) else json.loads((packet/name).read_text())
            require(actual==value,'consumer plan accounting: '+name)
        counts={'requirements':12,'validation_cases':22,'implementation_steps':8,'role_tool_entries':7,
          'unique_tools':5,'implementation_changes':0,'new_runtime_sessions':0,'formal_runs':0}
        require(m['counts']==counts,'manifest counts')
        formal=json.loads((packet/'formal-state.json').read_text())
        require(formal==prepare.load('prepare-source-review-supplement.py').formal_state(ROOT),'formal state changed')
        require(formal['freeze'] is None and formal['execution_allowed'] is False,'formal execution promotion')
        if check_references:
            ref={'manifest':(packet/'manifest.json').relative_to(ROOT).as_posix(),
              'sha256':sha((packet/'manifest.json').read_bytes()),'status':'consumer_contract_spec_and_isolated_plan_not_implementation_or_acceptance'}
            for name in ['PROTOCOL','PROGRESS']:
                d=json.loads((ROOT/f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('consumer_contract_plan')==ref,'ledger reference: '+name)
    except (OSError,ValueError,KeyError,TypeError,IndexError,subprocess.SubprocessError) as e:
        errors.append('invalid consumer plan: '+str(e))
    return errors


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--packet',type=Path,required=True)
    parser.add_argument('--no-references',action='store_true');a=parser.parse_args()
    errors=check(a.packet.resolve(),not a.no_references)
    for error in errors:print(error)
    if not errors:print('Consumer specification and isolated plan checks passed; runtime remains not_run')
    raise SystemExit(bool(errors))
