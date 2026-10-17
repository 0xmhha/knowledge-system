#!/usr/bin/env python3
"""Render exact captured Mermaid bytes with local pinned tooling and owned profiles."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import struct
import subprocess
import time
ROOT=Path(__file__).resolve().parents[1]
PRIOR=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/host-schema-compatibility-v1'
MODULES=Path('/Users/kevin/.npm/_npx/fa58c8ce4c502b5d/node_modules')
CLI=MODULES/'@mermaid-js/mermaid-cli/src/cli.js'
BROWSER=Path('/Users/kevin/.cache/puppeteer/chrome-headless-shell/mac_arm-155.0.8059.39/chrome-headless-shell-mac-arm64/chrome-headless-shell')
s=importlib.util.spec_from_file_location('ownership',ROOT/'scripts/consumer-process-ownership.py');own=importlib.util.module_from_spec(s);s.loader.exec_module(own)
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def dump(p,d):p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
def require(v,s):
    if not v:raise ValueError(s)


def run(out):
    require(not out.exists(),'fresh owned render root');out.mkdir();node=Path(subprocess.check_output(['which','node'],text=True).strip()).resolve()
    versions={n:json.loads((MODULES/n/'package.json').read_text())['version']for n in ['@mermaid-js/mermaid-cli','mermaid','puppeteer','puppeteer-core']}
    versions.update(node=subprocess.check_output([str(node),'--version'],text=True).strip(),browser=subprocess.check_output([str(BROWSER),'--version'],text=True).strip())
    dump(out/'toolchain.json',{'versions':versions,'files':{str(p):sha(p)for p in [node,CLI,BROWSER,MODULES/'mermaid/dist/mermaid.core.mjs',MODULES/'@mermaid-js/mermaid-cli/package.json']},'download_commands':[],'driver_sha256':sha(Path(__file__)),'ownership_helper_sha256':sha(ROOT/'scripts/consumer-process-ownership.py')})
    proof=json.loads((PRIOR/'consumer-source/git-proof.json').read_text());raw=(PRIOR/'consumer-source'/proof['files']['plugin/scripts/deep_review.py']['artifact']).read_bytes();namespace={'__name__':'pinned_verifier'};exec(compile(raw,'<pinned verifier>','exec'),namespace)
    receipts=[]
    for case in ['P06-analyzer-handoff','P06-analyzer-revision']:
        source=PRIOR/'runs'/case/'workspace-after';report=json.loads((source/'deep-review.json').read_text());bundle=json.loads((source/'deep-review-evidence.json').read_text());namespace['validate'](report,bundle)
        for kind,fn in [('sequence','sequence_diagram'),('modules','module_diagram')]:
            text=namespace[fn](report)+'\n';require(text==(source/(kind+'.mmd')).read_text(),'typed Mermaid bytes differ')
            folder=out/(case+'-'+kind);folder.mkdir();input=folder/'input.mmd';input.write_text(text)
            for format in ['svg','png']:receipts.append(render(node,folder,input,format,False))
    folder=out/'malformed-control';folder.mkdir();input=folder/'input.mmd';input.write_text('flowchart TD\n A["unterminated\n')
    receipts.append(render(node,folder,input,'svg',True));dump(out/'renders.json',receipts);print(json.dumps({'successful_renders':8,'expected_syntax_refusals':1,'real_model_calls':0}))


def render(node,folder,input,format,negative):
    target=folder/('output.'+format);puppeteer=folder/(format+'-puppeteer.json');mermaid=folder/'mermaid.json'
    dump(puppeteer,{'executablePath':str(BROWSER),'headless':True,'pipe':True,'userDataDir':str(folder/(format+'-browser-profile')),'args':['--disable-background-networking','--no-first-run']})
    dump(mermaid,{'securityLevel':'strict','theme':'neutral','fontFamily':'Arial','flowchart':{'htmlLabels':False}})
    cmd=[str(node),str(CLI),'-i',str(input),'-o',str(target),'-p',str(puppeteer),'-c',str(mermaid),'--no-font-embed','-j','1'];started=time.monotonic();forced=False
    with (folder/(format+'.stdout.txt')).open('wb')as stdout,(folder/(format+'.stderr.txt')).open('wb')as stderr:
        process=subprocess.Popen(cmd,stdin=subprocess.DEVNULL,stdout=stdout,stderr=stderr,start_new_session=True)
        with own.Descendants(process.pid)as monitor:
            try:process.wait(timeout=60)
            except subprocess.TimeoutExpired:forced=True;process.terminate();process.wait(timeout=5)
        observed=list(monitor.records.values());cleanup=own.wait_gone(observed)
        rescue=own.rescue(observed)if not cleanup['all_observed_identities_gone']else[]
    require(not forced and not rescue and cleanup['all_observed_identities_gone'],'renderer cleanup failed')
    require(any(str(folder/(format+'-browser-profile'))in r['command']for r in observed),'actual owned browser not observed')
    require((process.returncode!=0 and not target.exists())if negative else(process.returncode==0 and target.is_file()),'unexpected renderer result')
    record={'input':str(input),'input_sha256':sha(input),'format':format,'command':cmd,'exit_code':process.returncode,'elapsed_seconds':time.monotonic()-started,'expected_syntax_refusal':negative,'forced':forced,'rescue':rescue,'node_pid':process.pid,'node_reaped':process.poll()is not None,'observed_descendants':observed,'cleanup':cleanup,'network_isolation_measured':False}
    if target.exists():
        record.update(output=str(target),output_sha256=sha(target),output_bytes=target.stat().st_size)
        if format=='png':
            raw=target.read_bytes();require(raw[:8]==b'\x89PNG\r\n\x1a\n','invalid PNG');record['png_dimensions']=list(struct.unpack('>II',raw[16:24]))
    dump(folder/(format+'-receipt.json'),record);return record


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--output',type=Path,required=True);a=p.parse_args();run(a.output.resolve())
