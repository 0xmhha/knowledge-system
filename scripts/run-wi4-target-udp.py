#!/usr/bin/env python3
"""Run the predeclared UDP DEV probes only in an owned unchanged checkout."""
import argparse,concurrent.futures,importlib.util,json,os,subprocess
from pathlib import Path
spec=importlib.util.spec_from_file_location('owned',Path(__file__).with_name('run-wi4-target-subscription.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
PACKET=m.ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/wi4-target-udp-v1'
FIXTURE=m.ROOT/'scripts/fixtures/wi4-owned-udp.go.fixture'


def run(work):
    work.mkdir();out=work/'udp-trials-v1';out.mkdir();scope=json.loads((PACKET/'SCOPE.json').read_text());m.write(out/'SCOPE.json',scope)
    parent=Path('/private/tmp/ks-wi4-target-20261019');dep=json.loads((parent/'dependency-plan.json').read_text());go=dep['argv'][0]
    # No credential environment is inherited by upstream test processes.
    env={k:os.environ[k]for k in ('HOME','USER','TMPDIR','LANG','LC_ALL')if k in os.environ}
    env.update(dep['task_environment']);env['PATH']=str(Path(go).parent)+':/usr/bin:/bin:/usr/sbin:/sbin'
    env['GOCACHE']=str(work/'go-cache')
    src=work/'source'
    r=m.command(['git','-C',str(parent/'source'),'worktree','add','--detach',str(src),m.COMMIT],work,out/'restore',env,60)
    if r['exit_code']or r['remaining_live_group_members']:raise ValueError('owned target restore failed')
    baseline=m.source_inventory(src)
    previous=json.loads((parent/'subscription-trials-v1/source-binding.json').read_text())
    if baseline!=previous['tracked']or m.git(src,'status','--porcelain'):raise ValueError('upstream source identity differs')
    target=src/'p2p/discover/wi4_owned_udp_test.go';target.write_bytes(FIXTURE.read_bytes());status=m.git(src,'status','--porcelain').decode()
    if status!='?? p2p/discover/wi4_owned_udp_test.go\n':raise ValueError('unexpected source mutation')
    m.write(out/'source-binding.json',dict(source_commit=m.COMMIT,tracked=baseline,test_addition=dict(path='p2p/discover/wi4_owned_udp_test.go',sha256=m.sha(FIXTURE.read_bytes())),production_patch=False))
    profile=json.loads(subprocess.check_output([go,'env','-json','GOVERSION','GOOS','GOARCH','CGO_ENABLED'],cwd=src,env=env,text=True));m.write(out/'profile.json',profile)
    if profile!=scope['target_toolchain']:raise ValueError('actual target toolchain differs')
    m.write(out/'environment.json',dict(allowed_inherited_keys=sorted(k for k in env if k in os.environ and k in ('HOME','USER','TMPDIR','LANG','LC_ALL')),credential_environment_inherited=False,task_settings=dep['task_environment'],effective_GOCACHE=env['GOCACHE'],PATH=env['PATH'],scope='allowlisted_process_environment_not_independent_frozen_environment'))
    binaries={}
    for variant in ('general','race'):
        binary=out/(variant+'.test');argv=[go,'test']+(['-race']if variant=='race'else[])+['-c','-o',str(binary),'./p2p/discover'];r=m.command(argv,src,out/('build-'+variant),env,scope['build_timeout_seconds'])
        if r['exit_code']or r['remaining_live_group_members']:raise ValueError('target compile failed '+variant)
        binaries[variant]=dict(path=str(binary),sha256=m.sha(binary.read_bytes()))
    m.write(out/'binaries.json',binaries);slots=scope['planned_slots'];m.write(out/'planned.json',slots)
    def execute(slot):
        q=out/'slots'/slot['slot_id'];q.mkdir(parents=True);tmp=q/'tmp';tmp.mkdir();trace=q/'events.jsonl';binary=binaries[slot['variant']]
        local={**env,'TMPDIR':str(tmp),'WI4_CASE':slot['case_id'],'WI4_TRACE_PATH':str(trace)}
        argv=[binary['path'],'-test.v=true','-test.run=^TestWI4OwnedUDP$','-test.count=1','-test.timeout=15s'];r=m.command(argv,src,q/'process',local,scope['owned_process_timeout_seconds'])
        r.update(slot,binary_sha256=binary['sha256'],tmp_children_after=sorted(p.name for p in tmp.iterdir()),source_test_addition_sha256=m.sha(target.read_bytes()));m.write(q/'receipt.json',r)
        print(json.dumps(dict(slot_id=slot['slot_id'],exit_code=r['exit_code'],timeout=r['timeout'],remaining_live_group_members=r['remaining_live_group_members'])),flush=True);return r
    results=[]
    with concurrent.futures.ThreadPoolExecutor(max_workers=scope['parallelism'])as pool:
        futures=[pool.submit(execute,s)for s in slots]
        for future in futures:results.append(future.result());m.write(out/'attempts.json',results)
    if m.source_inventory(src)!=baseline or m.git(src,'status','--porcelain').decode()!=status or target.read_bytes()!=FIXTURE.read_bytes():raise ValueError('source changed during trials')
    m.write(out/'binary-postrun-check.json',dict(observed_binary_sha256={v:m.sha(Path(b['path']).read_bytes())for v,b in binaries.items()},all_match_build_receipts=all(m.sha(Path(b['path']).read_bytes())==b['sha256']for b in binaries.values())))
    m.write(out/'receipt.json',dict(planned=len(slots),attempted=len(results),retries=0,tracked_source_preserved=True,only_test_addition=True,production_patch=False,source_commit=m.COMMIT,independent_approved=0,new_independent_families=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
