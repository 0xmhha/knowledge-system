#!/usr/bin/env python3
"""Rebuild WI-V1/V2 gap accounting without granting independent oracle or candidate acceptance."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha, verify

ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('ticket_gap_prepare',ROOT/'scripts/prepare-ticket-gap.py')
prepare=importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


def check(packet,check_references=True):
    errors=[]

    def require(condition,message):
        if not condition:
            errors.append(message)

    try:
        manifest=json.loads((packet/'manifest.json').read_text())
        require(manifest['kind']=='ticket_gap_preparation_only','packet kind')
        require(set(manifest['input_bindings'])=={str(p) for p in prepare.bindings()},'input binding inventory')
        for path,digest in manifest['input_bindings'].items():
            require(sha(safe(ROOT,path).read_bytes())==digest,'input binding: '+path)
        raw=(ROOT/'scripts/prepare-ticket-gap.py').read_bytes()
        require(manifest['generator_sha256']==sha(raw) and (packet/'generator.py.txt').read_bytes()==raw,'generator binding')
        if errors:
            return errors
        data=prepare.inputs(packet)
        expected=prepare.generate(data)
        proof_paths=set()
        for owner, revision in [('project',prepare.REVISION), ('consumer',data['host']['source_adoptions']['coding_agent']['source_commit'])]:
            root=packet/'source'/owner
            proof=verify(root,revision)
            prefix='source/'+owner+'/'
            proof_paths|={prefix+'git-proof.json'}|{prefix+p for p in proof['objects'].values()}|{prefix+r['artifact'] for r in proof['files'].values()}
        inventory={p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name!='manifest.json'}
        require(inventory==set(manifest['artifacts']),'artifact inventory')
        require(inventory==set(expected)|proof_paths|{'.gitattributes','generator.py.txt','formal-state.json','REVIEW-TABLE.txt'},'unexpected artifact')
        for name,digest in manifest['artifacts'].items():
            require(sha(safe(packet,name).read_bytes())==digest,'artifact hash: '+name)
        for name,value in expected.items():
            require(json.loads((packet/name).read_text())==value,'ticket gap accounting: '+name)
        require(manifest['counts']==expected['gap-summary.json']['counts'],'manifest counts')
        require((packet/'REVIEW-TABLE.txt').read_text()==prepare.table(expected),'human review table')
        require((packet/'.gitattributes').read_bytes()==prepare.ATTRIBUTES,'Git object preservation attributes')
        require((packet/'public-ticket.json').read_bytes()==(ROOT/prepare.INPUTS['public']).read_bytes(),'public input bytes changed')
        formal=json.loads((packet/'formal-state.json').read_text())
        require(formal==prepare.load('prepare-source-review-supplement.py').formal_state(ROOT),'formal state changed')
        require(formal['freeze'] is None and formal['execution_allowed'] is False,'formal execution promotion')
        if check_references:
            ref={'manifest':(packet/'manifest.json').relative_to(ROOT).as_posix(),
                 'sha256':sha((packet/'manifest.json').read_bytes()),
                 'status':'api_ticket_oracle_and_acceptable_set_gaps_not_acceptance'}
            for name in ['PROTOCOL','PROGRESS']:
                doc=json.loads((ROOT/f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('ticket_gap_preparation')==ref,'ledger reference: '+name)
    except (OSError,ValueError,KeyError,TypeError,IndexError,subprocess.SubprocessError) as exc:
        errors.append('invalid packet: '+str(exc))
    return errors


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--packet',type=Path,required=True)
    parser.add_argument('--no-references',action='store_true')
    args=parser.parse_args()
    failures=check(args.packet.resolve(),not args.no_references)
    for failure in failures:
        print(failure)
    if not failures:
        print('ticket gap preparation checks passed')
    raise SystemExit(bool(failures))
