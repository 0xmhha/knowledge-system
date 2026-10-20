#!/usr/bin/env python3
"""Recompute P07 evidence; PID absence never substitutes for exit/reap proof."""
import argparse
import importlib.util
from pathlib import Path
import subprocess
from source_packet import safe
ROOT=Path(__file__).resolve().parents[1]
s=importlib.util.spec_from_file_location('render_lifecycle_prepare',ROOT/'scripts/prepare-consumer-render-lifecycle.py');p=importlib.util.module_from_spec(s);s.loader.exec_module(p)

def check(packet,references=True):
    errors=[]
    def need(v,s):
        if not v:errors.append(s)
    try:
        m=p.read(packet/'manifest.json');need(m['kind']=='P07_render_lifecycle_not_quality_or_adoption','scope')
        need(set(m['input_bindings'])=={str(f)for f in p.INPUTS},'input inventory')
        for f,h in m['input_bindings'].items():need(p.digest(safe(ROOT,f))==h,'input binding: '+f)
        source=ROOT/'scripts/prepare-consumer-render-lifecycle.py';need(m['generator_sha256']==p.digest(source)and(packet/'generator.py.txt').read_bytes()==source.read_bytes(),'generator binding')
        expected={'manifest.json','generator.py.txt','.gitattributes','execution.json','source-pins.json','preservation.json','formal-state.json','postrun-process-audit.json','render-lifecycle.json','REPORT.txt','helper-versions/driver-initial.py.txt','helper-versions/driver-budget-retry.py.txt','diagnostics/budget-diagnosis.json','renders/renders.json','renders/toolchain.json','renders/visual-check.json'}
        for prefix in ['hosts','diagnostics/hosts']:
            expected|={prefix+'/'+case+'/'+name for case in p.CASES for name in p.TOP}
        for prefix,boundary in [('native/proxy-budget',True),('diagnostics/native/proxy-budget',False),('diagnostics/native/proxy-budget-retry',True)]:
            expected.add(prefix+'/mcp-frames.jsonl');expected|={prefix+'/session/'+f for f in ['stdin.jsonl','stdout.jsonl','stderr.txt','messages.json','lifecycle.json']}
            if boundary:expected.add(prefix+'/boundary.json')
        for r in p.read(packet/'renders/renders.json'):
            folder=Path(r['input']).parent.name;fmt=r['format'];names=['input.mmd','mermaid.json',fmt+'-puppeteer.json',fmt+'-receipt.json',fmt+'.stdout.txt',fmt+'.stderr.txt']
            if not r['expected_syntax_refusal']:names.append('output.'+fmt)
            expected|={'renders/'+folder+'/'+name for name in names}
        actual={f.relative_to(packet).as_posix()for f in packet.rglob('*')if f.is_file()}
        need(actual==expected and set(m['artifacts'])==expected-{'manifest.json'},'artifact inventory')
        for f,h in m['artifacts'].items():need(p.digest(safe(packet,f))==h,'artifact hash: '+f)
        if errors:return errors
        need(p.read(packet/'render-lifecycle.json')==p.derive(packet),'derived case/result accounting')
        report=(packet/'REPORT.txt').read_text();need('CKS139요청'in report and 'unknown/reap미확인'in report and '정식0/9'in report,'report omitted failures/scope')
        if references:
            ref={'manifest':(packet/'manifest.json').relative_to(ROOT).as_posix(),'sha256':p.digest(packet/'manifest.json'),'status':'P07_render_lifecycle_pass_plan17_of22_not_adopted_or_accepted'}
            for n in ['PROTOCOL','PROGRESS']:need(p.read(ROOT/f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{n}.json').get('consumer_render_lifecycle')==ref,'ledger reference: '+n)
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,SyntaxError,ETError,subprocess.SubprocessError)as e:errors.append('invalid P07 evidence: '+str(e))
    return errors

# XML parser failures are evidence failures, not a successful empty diagram.
from xml.etree.ElementTree import ParseError as ETError
if __name__=='__main__':
    a=argparse.ArgumentParser(description=__doc__);a.add_argument('--packet',type=Path,required=True);a.add_argument('--no-references',action='store_true');args=a.parse_args();errors=check(args.packet.resolve(),not args.no_references)
    for e in errors:print(e)
    if not errors:print('P07 verified:8 renders; final6 CKS exits/reaped; initial1 unknown retained; plan17/22, formal0/9')
    raise SystemExit(bool(errors))
