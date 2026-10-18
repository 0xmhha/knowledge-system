#!/usr/bin/env python3
"""Run fixed exposed DEV slots against an owned unchanged upstream checkout."""
import argparse,concurrent.futures,hashlib,json,os,signal,subprocess,time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1'
FIXTURE=ROOT/'scripts/fixtures/wi4-owned-subscription.go.fixture'
COMMIT='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'


def sha(raw):return hashlib.sha256(raw).hexdigest()
def write(p,d):p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
def git(src,*argv):return subprocess.check_output(['git','-C',str(src),*argv])
def live_group(pgid):
    rows=subprocess.check_output(['ps','-axo','pid=,pgid=,stat='],text=True).splitlines()
    return [int(p[0])for line in rows if len(p:=line.split())==3 and int(p[1])==pgid and not p[2].startswith('Z')]


def command(argv,src,out,env,timeout):
    out.mkdir(parents=True);start=time.monotonic();timed=False;term=False;kill=False
    with(out/'stdout').open('wb')as stdout,(out/'stderr').open('wb')as stderr:
        p=subprocess.Popen(argv,cwd=src,env=env,stdout=stdout,stderr=stderr,start_new_session=True)
        write(out/'live.json',dict(pid=p.pid,pgid=p.pid,argv=argv,owned_new_session=True))
        try:p.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed=True;term=True;os.killpg(p.pid,signal.SIGTERM)
            try:p.wait(timeout=5)
            except subprocess.TimeoutExpired:kill=True;os.killpg(p.pid,signal.SIGKILL);p.wait(timeout=5)
    remaining=live_group(p.pid)
    receipt=dict(argv=argv,cwd=str(src),pid=p.pid,pgid=p.pid,owned_new_session=True,exit_code=p.returncode,timeout=timed,term_sent=term,kill_sent=kill,reaped=p.poll()is not None,remaining_live_group_members=len(remaining),elapsed_seconds=time.monotonic()-start,stdout_sha256=sha((out/'stdout').read_bytes()),stderr_sha256=sha((out/'stderr').read_bytes()))
    write(out/'receipt.json',receipt);return receipt


def source_inventory(src):
    entries={}
    for raw in git(src,'ls-files','--stage','-z').split(b'\0'):
        if not raw:continue
        header,name=raw.split(b'\t',1);mode,blob,_=header.decode().split();name=name.decode();p=src/name
        if mode=='160000':entries[name]=dict(mode=mode,git_object=blob)
        elif mode=='120000':entries[name]=dict(mode=mode,sha256=sha(os.readlink(p).encode()))
        else:entries[name]=dict(mode=mode,sha256=sha(p.read_bytes()))
    return entries


def run(work):
    src=work/'source';out=work/'subscription-trials-v1';out.mkdir()
    scope=json.loads((PACKET/'SCOPE.json').read_text());write(out/'SCOPE.json',scope)
    if git(src,'rev-parse','HEAD').decode().strip()!=COMMIT or git(src,'status','--porcelain'):raise ValueError('source must be exact clean owned target before test addition')
    baseline=source_inventory(src);target=src/'eth/downloader/wi4_owned_test.go';target.write_bytes(FIXTURE.read_bytes())
    status=git(src,'status','--porcelain').decode()
    if status!='?? eth/downloader/wi4_owned_test.go\n':raise ValueError('unexpected source change')
    write(out/'source-binding.json',dict(source_commit=COMMIT,tracked=baseline,test_addition=dict(path='eth/downloader/wi4_owned_test.go',sha256=sha(FIXTURE.read_bytes())),production_patch=False))
    dep=json.loads((work/'dependency-plan.json').read_text());settings=dep['task_environment'];env={**os.environ,**settings};go=dep['argv'][0]
    profile=json.loads(subprocess.check_output([go,'env','-json','GOVERSION','GOOS','GOARCH','CGO_ENABLED'],cwd=src,env=env,text=True));write(out/'profile.json',profile)
    if profile!=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'):raise ValueError('target build profile differs')
    binaries={}
    for variant in scope['plan']['variants']:
        binary=out/(variant+'.test');argv=[go,'test']+(['-race']if variant=='race'else[])+['-c','-o',str(binary),'./eth/downloader']
        r=command(argv,src,out/('build-'+variant),env,600)
        if r['exit_code']or r['timeout']or r['remaining_live_group_members']:raise ValueError('target probe compile failed '+variant)
        binaries[variant]=dict(path=str(binary),sha256=sha(binary.read_bytes()),build_receipt='build-'+variant+'/receipt.json')
    write(out/'binaries.json',binaries)
    slots=[dict(slot_id=f'{v}-r{rep}-{case}',variant=v,repetition=rep,case_id=case,route='real_API_eventLoop'if case in('SUB-normal','SUB-blocked')else'controlled_receiver_actual_Unsubscribe')for v in scope['plan']['variants']for rep in scope['plan']['repetitions']for case in scope['plan']['cases']]
    write(out/'planned.json',slots)
    def execute(slot):
        q=out/'slots'/slot['slot_id'];q.mkdir(parents=True);tmp=q/'tmp';tmp.mkdir();trace=q/'events.jsonl'
        local={**env,'TMPDIR':str(tmp),'WI4_CASE':slot['case_id'],'WI4_TRACE_PATH':str(trace)};binary=binaries[slot['variant']]
        argv=[binary['path'],'-test.v=true','-test.run=^TestWI4OwnedSubscription$','-test.count=1','-test.timeout=100s']
        r=command(argv,src,q/'process',local,110);r.update(slot,binary_sha256=binary['sha256'],tmp_children_after=sorted(p.name for p in tmp.iterdir()),source_test_addition_sha256=sha(target.read_bytes()))
        write(q/'receipt.json',r);print(json.dumps(dict(slot_id=slot['slot_id'],exit_code=r['exit_code'],timeout=r['timeout'],remaining_live_group_members=r['remaining_live_group_members'])),flush=True);return r
    # Each fixed pair shares one repetition; no trial is retried or omitted.
    results=[]
    for variant in scope['plan']['variants']:
        for rep in scope['plan']['repetitions']:
            group=[s for s in slots if s['variant']==variant and s['repetition']==rep]
            with concurrent.futures.ThreadPoolExecutor(max_workers=2)as pool:
                futures=[pool.submit(execute,s)for s in group]
                for future in futures:results.append(future.result());write(out/'attempts.json',results)
    if source_inventory(src)!=baseline or git(src,'status','--porcelain').decode()!=status or target.read_bytes()!=FIXTURE.read_bytes():raise ValueError('source changed during target trials')
    write(out/'receipt.json',dict(planned=len(slots),attempted=len(results),retries=0,tracked_source_preserved=True,only_test_addition=True,production_patch=False,source_commit=COMMIT,independent_approved=0,new_independent_families=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
