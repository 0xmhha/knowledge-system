#!/usr/bin/env python3
"""Recheck real stdio evidence and remaining gates without rerunning host/model."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe,sha
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('stdio_prepare',ROOT/'scripts/prepare-consumer-stdio-readiness.py')
prepare=importlib.util.module_from_spec(spec);spec.loader.exec_module(prepare)


def raw_inventory():
    fixed={'execution.json','commands.json','fixture.bundle'}
    fixed|={'fixture-source/'+p for p in prepare.source_files()}
    fixed|={f'{p}/{name}' for p in ['index','old-index'] for name in ['graph.db','manifest.json']}
    fixed|={f'command-logs/{p}.{s}.txt' for p in ['format','git-init','git-add','git-commit','git-head','fixture-race','fixture-vet','fixture-bundle','index'] for s in ['stdout','stderr']}
    fixed|={f'sessions/{session}/{name}' for session in ['discovery','main','old-index','unavailable'] for name in ['stdin.jsonl','stdout.jsonl','stderr.txt','messages.json','lifecycle.json']}
    return fixed


def check(packet,references=True):
    errors=[]
    def need(ok,message):
        if not ok:errors.append(message)
    try:
        m=json.loads((packet/'manifest.json').read_text())
        need(m['kind']=='owned_P05_stdio_not_role_or_formal_acceptance','packet scope')
        need(set(m['input_bindings'])=={str(p) for p in prepare.INPUTS},'input inventory')
        for p,digest in m['input_bindings'].items():need(sha(safe(ROOT,p).read_bytes())==digest,'input binding: '+p)
        raw=(ROOT/'scripts/prepare-consumer-stdio-readiness.py').read_bytes()
        need(m['generator_sha256']==sha(raw) and (packet/'generator.py.txt').read_bytes()==raw,'generator binding')
        fixed=raw_inventory()|{'formal-state.json','generator.py.txt','.gitattributes','stdio-readiness.json','STDIO-READINESS-REPORT.txt'}
        fixed|={'diagnostics/initial-run/'+p for p in raw_inventory()}
        fixed|={'diagnostics/old-index-uncheckpointed/'+p for p in ['stdin.jsonl','stdout.jsonl','stderr.txt','messages.json','lifecycle.json']}
        fixed.add('diagnostics/diagnosis.json')
        inventory={p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p!=packet/'manifest.json'}
        need(inventory==set(m['artifacts'])==fixed,'artifact inventory')
        for p,digest in m['artifacts'].items():need(sha(safe(packet,p).read_bytes())==digest,'artifact hash: '+p)
        if errors:return errors
        for name,value in prepare.derive(packet).items():
            actual=(packet/name).read_text() if isinstance(value,str) else json.loads((packet/name).read_text())
            need(actual==value,'stdio accounting: '+name)
        need((packet/'.gitattributes').read_bytes()==prepare.ATTRIBUTES,'binary preservation')
        formal=json.loads((packet/'formal-state.json').read_text())
        need(formal==prepare.load('prepare-source-review-supplement.py').formal_state(ROOT),'formal state changed')
        need(formal['freeze'] is None and formal['execution_allowed'] is False,'formal execution promotion')
        if references:
            ref={'manifest':(packet/'manifest.json').relative_to(ROOT).as_posix(),'sha256':sha((packet/'manifest.json').read_bytes()),'status':'P05_owned_stdio_not_actual_role_or_formal_acceptance'}
            for n in ['PROTOCOL','PROGRESS']:
                d=json.loads((ROOT/f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{n}.json').read_text())
                need(d.get('consumer_stdio_readiness')==ref,'ledger reference: '+n)
    except (OSError,ValueError,KeyError,TypeError,IndexError,subprocess.SubprocessError) as e:errors.append('invalid stdio evidence: '+str(e))
    return errors


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--packet',type=Path,required=True);p.add_argument('--no-references',action='store_true')
    a=p.parse_args();errors=check(a.packet.resolve(),not a.no_references)
    for e in errors:print(e)
    if not errors:print('P05 owned real stdio verified; actual roles/formal acceptance remain pending')
    raise SystemExit(bool(errors))
