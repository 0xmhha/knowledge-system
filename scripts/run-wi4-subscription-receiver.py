#!/usr/bin/env python3
"""Retain first42 attempts; run6 exact receiver lifecycle slots in a third checkout."""
import argparse,concurrent.futures,importlib.util,json,os,subprocess
from pathlib import Path
spec=importlib.util.spec_from_file_location('owned',Path(__file__).with_name('run-wi4-target-subscription.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
FIXTURE=m.ROOT/'scripts/fixtures/wi4-owned-subscription-receiver.go.fixture'


def run(work):
    first=work/'subscription-trials-v1';receipt=json.loads((first/'receipt.json').read_text())
    if receipt['attempted']!=30 or receipt['retries']!=0 or receipt['tracked_source_preserved']is not True:raise ValueError('first fixed plan must be terminal and retained')
    second=json.loads((work/'subscription-corrective-v1/receipt.json').read_text())
    if second['attempted']!=12 or second['retries']!=0:raise ValueError('second fixed plan must be terminal and retained')
    out=work/'subscription-receiver-v1';out.mkdir();scope=json.loads((m.PACKET/'FULL-RECEIVER-SCOPE.json').read_text());m.write(out/'SCOPE.json',scope)
    src=work/'receiver-source';dep=json.loads((work/'dependency-plan.json').read_text());env={**os.environ,**dep['task_environment']};go=dep['argv'][0]
    r=m.command(['git','-C',str(work/'source'),'worktree','add','--detach',str(src),m.COMMIT],work,out/'restore',env,60)
    if r['exit_code']or r['remaining_live_group_members']:raise ValueError('owned corrective checkout restore failed')
    baseline=m.source_inventory(src);previous=json.loads((first/'source-binding.json').read_text())
    if baseline!=previous['tracked']or m.git(src,'status','--porcelain'):raise ValueError('corrective base differs from original tracked sources')
    target=src/'eth/downloader/wi4_owned_test.go';target.write_bytes(FIXTURE.read_bytes());status=m.git(src,'status','--porcelain').decode()
    if status!='?? eth/downloader/wi4_owned_test.go\n':raise ValueError('unexpected corrective source mutation')
    m.write(out/'source-binding.json',dict(source_commit=m.COMMIT,tracked=baseline,test_addition=dict(path='eth/downloader/wi4_owned_test.go',sha256=m.sha(FIXTURE.read_bytes())),production_patch=False))
    profile=json.loads(subprocess.check_output([go,'env','-json','GOVERSION','GOOS','GOARCH','CGO_ENABLED'],cwd=src,env=env,text=True));m.write(out/'profile.json',profile)
    if profile!=json.loads((first/'profile.json').read_text()):raise ValueError('corrective profile differs')
    binaries={}
    for variant in ('general','race'):
        binary=out/(variant+'.test');argv=[go,'test']+(['-race']if variant=='race'else[])+['-c','-o',str(binary),'./eth/downloader'];r=m.command(argv,src,out/('build-'+variant),env,600)
        if r['exit_code']or r['remaining_live_group_members']:raise ValueError('corrective compile failed')
        binaries[variant]=dict(path=str(binary),sha256=m.sha(binary.read_bytes()),build_receipt='build-'+variant+'/receipt.json')
    m.write(out/'binaries.json',binaries);slots=scope['additional_slots'];m.write(out/'planned.json',slots)
    def execute(slot):
        q=out/'slots'/slot['slot_id'];q.mkdir(parents=True);tmp=q/'tmp';tmp.mkdir();trace=q/'events.jsonl';binary=binaries[slot['variant']]
        local={**env,'TMPDIR':str(tmp),'WI4_CASE':slot['case_id'],'WI4_TRACE_PATH':str(trace)}
        argv=[binary['path'],'-test.v=true','-test.run=^TestWI4OwnedSubscription$','-test.count=1','-test.timeout=100s'];r=m.command(argv,src,q/'process',local,110)
        r.update(slot,binary_sha256=binary['sha256'],tmp_children_after=sorted(p.name for p in tmp.iterdir()),source_test_addition_sha256=m.sha(target.read_bytes()));m.write(q/'receipt.json',r)
        print(json.dumps(dict(slot_id=slot['slot_id'],exit_code=r['exit_code'],timeout=r['timeout'],remaining_live_group_members=r['remaining_live_group_members'])),flush=True);return r
    results=[]
    with concurrent.futures.ThreadPoolExecutor(max_workers=2)as pool:
        futures=[pool.submit(execute,s)for s in slots]
        for future in futures:results.append(future.result());m.write(out/'attempts.json',results)
    if m.source_inventory(src)!=baseline or m.git(src,'status','--porcelain').decode()!=status or target.read_bytes()!=FIXTURE.read_bytes():raise ValueError('corrective sources changed during trials')
    m.write(out/'receipt.json',dict(planned=len(slots),attempted=len(results),retries=0,tracked_source_preserved=True,only_test_addition=True,production_patch=False,source_commit=m.COMMIT,independent_approved=0,new_independent_families=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
