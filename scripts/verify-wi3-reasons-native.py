#!/usr/bin/env python3
"""Run fresh whole-module native checks on an owned source-bound clone."""
import argparse,hashlib,json,os,signal,subprocess,time
from pathlib import Path


def sha(raw):return hashlib.sha256(raw).hexdigest()
def write(p,v):p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def check_sources(root,binding):
    for name,h in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
        if sha((root/name).read_bytes())!=h:raise ValueError('native input changed: '+name)


def run(root,out,binding,cache):
    out.mkdir(parents=True);check_sources(root,binding);write(out/'source-binding.json',binding);receipts=[]
    def call(label,argv,limit):
        check_sources(root,binding);start=time.monotonic();timed_out=False;term=False;kill=False
        with (out/(label+'.stdout')).open('wb') as stdout,(out/(label+'.stderr')).open('wb') as stderr:
            p=subprocess.Popen(argv,cwd=root,stdout=stdout,stderr=stderr,env={**os.environ,'PYTHONDONTWRITEBYTECODE':'1','GOCACHE':str(cache)},start_new_session=True)
            try:p.wait(timeout=limit)
            except subprocess.TimeoutExpired:
                timed_out=True;term=True;os.killpg(p.pid,signal.SIGTERM)
                try:p.wait(timeout=5)
                except subprocess.TimeoutExpired:kill=True;os.killpg(p.pid,signal.SIGKILL);p.wait(timeout=5)
        r=dict(label=label,argv=argv,cwd=str(root),pid=p.pid,owned_new_session=True,exit_code=p.returncode,timeout=timed_out,term_sent=term,kill_sent=kill,reaped=p.poll()is not None,elapsed_seconds=time.monotonic()-start,
            source_binding_sha256=sha((out/'source-binding.json').read_bytes()),source_head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip())
        if '-json'in argv:
            counts={'pass':0,'fail':0,'skip':0};packages={'pass':0,'fail':0,'skip':0}
            for line in (out/(label+'.stdout')).read_text().splitlines():
                d=json.loads(line);target=counts if d.get('Test') else packages;action=d.get('Action')
                if action in target:target[action]+=1
            r.update(tests=counts,packages=packages)
        for suffix in ('stdout','stderr'):r[suffix+'_sha256']=sha((out/(label+'.'+suffix)).read_bytes())
        receipts.append(r);write(out/'receipts.json',receipts);print(json.dumps({k:r[k] for k in ('label','exit_code','elapsed_seconds','timeout')}),flush=True)
        if p.returncode!=0 or timed_out:raise RuntimeError('native check failed: '+label)
        check_sources(root,binding)
    call('build',['go','build','./...'],900)
    call('test',['go','test','-json','-count=1','-timeout=18m','./...'],1320)
    call('race',['go','test','-race','-json','-count=1','-timeout=20m','./...'],1440)
    call('lint',['make','lint','GOLANGCI_LINT=/Users/kevin/.gvm/pkgsets/go1.26.8/global/bin/golangci-lint'],1200)
    call('build-bins',['make','build-bins'],900)
    call('build-runtime',['go','build','-o',str(out/'read-cks'),'./cmd/cks'],900)
    call('build-info',['go','version','-m',str(out/'read-cks')],30)
    write(out/'binary.json',dict(sha256=sha((out/'read-cks').read_bytes()),source_head=receipts[0]['source_head'],source_mode='working_tree_two_bound_Go_changes',native_source_binding_sha256=sha((out/'source-binding.json').read_bytes())))
    print('fresh native validation complete',flush=True)


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--binding',type=Path,required=True);p.add_argument('--cache',type=Path,required=True);a=p.parse_args();run(a.source.resolve(),a.out.resolve(),json.loads(a.binding.read_text()),a.cache.resolve())
