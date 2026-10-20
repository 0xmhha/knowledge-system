#!/usr/bin/env python3
"""Owned actual RPC/native timer program with declared source lifecycle hooks."""
import argparse,concurrent.futures,importlib.util,json,os,subprocess
from lifecycle_instrumentation import instrument
from pathlib import Path
spec=importlib.util.spec_from_file_location('owned',Path(__file__).with_name('run-wi4-target-subscription.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
ROOT=m.ROOT;PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/ticket-rpc-lifecycle-v1';DRAFT=PACKET.parent/'ticket-dev-gold-v1'
API='eth/downloader/api.go';BRIDGE='eth/downloader/ks_owned_rpc_lifecycle.go';MAIN='cmd/ks-owned-rpc-lifecycle/main.go'
FIXTURE=ROOT/'scripts/fixtures/ticket-rpc-lifecycle.go.fixture';MAIN_FIXTURE=ROOT/'scripts/fixtures/ticket-rpc-lifecycle-main.go.fixture'

def flat_tree(src,tree):
    rows=[]
    for row in m.git(src,'ls-tree','-rz',tree).split(b'\0'):
        if not row:continue
        header,name=row.split(b'\t',1);mode,kind,oid=header.decode().split();rows.append(dict(path=name.decode(),mode=mode,kind=kind,oid=oid))
    return rows

def tree_input(src,q,env,overlay,bridge,main):
    e={**env,'GIT_INDEX_FILE':str(q/'owned-index')};subprocess.run(['git','read-tree',m.COMMIT],cwd=src,env=e,check=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    objects={}
    for name,data in [(API,overlay),(BRIDGE,bridge),(MAIN,main)]:
        h=subprocess.check_output(['git','hash-object','-w','--stdin'],input=data,cwd=src,env=e).decode().strip();objects[name]=h
        subprocess.run(['git','update-index','--add','--cacheinfo','100644,'+h+','+name],cwd=src,env=e,check=True)
        if name==API:project=subprocess.check_output(['git','write-tree'],cwd=src,env=e).decode().strip()
    build=subprocess.check_output(['git','write-tree'],cwd=src,env=e).decode().strip();(q/'owned-index').unlink()
    return dict(base_commit=m.COMMIT,applied_project_commit=None,applied_project_tree=project,build_input_tree=build,source_mode='owned_working_tree_real_Git_trees_plus_two_program_additions_observation_hooks',objects=objects,project_flat_tree=flat_tree(src,project),build_flat_tree=flat_tree(src,build))

def run(work):
    work.mkdir();out=work/'rpc-lifecycle-trials-v1';out.mkdir();scope=json.loads((PACKET/'SCOPE.json').read_text());m.write(out/'SCOPE.json',scope)
    if m.sha(FIXTURE.read_bytes())!=scope['fixture_sha256']or m.sha(MAIN_FIXTURE.read_bytes())!=scope['main_fixture_sha256']:raise ValueError('frozen fixture differs')
    parent=Path('/private/tmp/ks-wi4-target-20261019');dep=json.loads((parent/'dependency-plan.json').read_text());go=dep['argv'][0];settings={**dep['task_environment'],'GOMODCACHE':'/Users/kevin/.gvm/pkgsets/go1.23.12/global/pkg/mod','GOCACHE':'/private/tmp/ks-ticket-rpc-20261019/go-cache'}
    env={k:os.environ[k]for k in ('HOME','USER','TMPDIR','LANG','LC_ALL')if k in os.environ};env.update(settings);env['PATH']=str(Path(go).parent)+':/usr/bin:/bin:/usr/sbin:/sbin'
    m.write(out/'environment.json',dict(credential_environment_inherited=False,allowed_inherited_keys=sorted(set(env)&{'HOME','USER','TMPDIR','LANG','LC_ALL'}),task_settings=settings,PATH=env['PATH'],scope='allowlisted_process_environment_not_independent_frozen_environment'))
    candidates={r['id']:r for r in json.loads((DRAFT/'private/candidates.json').read_text())};baseline_expected=json.loads((parent/'subscription-trials-v1/source-binding.json').read_text())['tracked'];built={}
    def build(role):
        candidate=candidates[role];q=out/'candidates'/role;q.mkdir(parents=True);src=work/('source-'+role)
        r=m.command(['git','-C',str(parent/'source'),'worktree','add','--detach',str(src),m.COMMIT],work,q/'restore',env,60)
        if r['exit_code']or r['remaining_live_group_members']:raise ValueError('owned candidate restore failed')
        baseline=m.source_inventory(src)
        if baseline!=baseline_expected or m.git(src,'status','--porcelain'):raise ValueError('base source differs')
        retained=(DRAFT/'private/overlays'/(role+'.go.txt')).read_bytes()
        if m.sha(retained)!=candidate['full_file_overlay_sha256']:raise ValueError('candidate overlay differs')
        overlay,anchors=instrument(retained)
        if anchors!=scope['source_anchors'][role]or m.sha(overlay)!=scope['instrumented_api_sha256'][role]:raise ValueError('predeclared instrumented source differs')
        (src/API).write_bytes(overlay);(src/BRIDGE).write_bytes(FIXTURE.read_bytes());(src/MAIN).parent.mkdir();(src/MAIN).write_bytes(MAIN_FIXTURE.read_bytes())
        expected=dict(baseline);expected[API]=dict(mode=baseline[API]['mode'],sha256=m.sha(overlay));after=m.source_inventory(src)
        status=m.git(src,'status','--porcelain','--untracked-files=all').decode();allowed={'?? '+BRIDGE,'?? '+MAIN}|({' M '+API}if overlay!=m.git(src,'show',m.COMMIT+':'+API)else set())
        if after!=expected or set(status.splitlines())!=allowed:raise ValueError('undeclared source additions/changes')
        binding=dict(candidate_id=role,base_commit=m.COMMIT,base_tracked=baseline,applied_tracked=after,api_sha256=m.sha(overlay),retained_api_sha256=m.sha(retained),method_sha256=candidate['method_sha256'],instrumented_api=True,source_anchors=anchors,fixture_sha256=m.sha(FIXTURE.read_bytes()),main_fixture_sha256=m.sha(MAIN_FIXTURE.read_bytes()),additions=[BRIDGE,MAIN],production_patch=False,original_test_init_preserved_in_source_but_not_executed=True)
        m.write(q/'source-binding.json',binding);m.write(q/'tree-binding.json',tree_input(src,q,env,overlay,FIXTURE.read_bytes(),MAIN_FIXTURE.read_bytes()));m.write(q/'base-flat-tree.json',flat_tree(src,m.COMMIT))
        profile=json.loads(subprocess.check_output([go,'env','-json','GOVERSION','GOOS','GOARCH','CGO_ENABLED'],cwd=src,env=env,text=True));m.write(q/'profile.json',profile)
        if profile!=scope['target_toolchain']:raise ValueError('actual profile differs')
        binaries={}
        for variant in ('general','race'):
            binary=q/(variant+'.program');argv=[go,'build']+(['-race']if variant=='race'else[])+['-o',str(binary),'./cmd/ks-owned-rpc-lifecycle'];r=m.command(argv,src,q/('build-'+variant),env,scope['build_timeout_seconds'])
            if r['exit_code']or r['timeout']or r['remaining_live_group_members']:raise ValueError('owned program build failed')
            binaries[variant]=dict(path=str(binary),sha256=m.sha(binary.read_bytes()))
        m.write(q/'binaries.json',binaries);return role,dict(src=src,binaries=binaries,binding=binding,status=status,anchors=anchors)
    with concurrent.futures.ThreadPoolExecutor(max_workers=scope['build_parallelism'])as pool:
        for role,b in pool.map(build,scope['candidate_roles']):built[role]=b
    slots=scope['planned_slots'];m.write(out/'planned.json',slots)
    def execute(slot):
        q=out/'slots'/slot['slot_id'];q.mkdir(parents=True);tmp=q/'tmp';tmp.mkdir();b=built[slot['candidate']];binary=b['binaries'][slot['variant']]
        local={**env,'TMPDIR':str(tmp),'TICKET_LIFECYCLE_TRACE':str(q/'events.jsonl'),'TICKET_LIFECYCLE_ROLE':slot['candidate'],'TICKET_LIFECYCLE_BROADCAST_SITE':'/eth/downloader/api.go:'+str(b['anchors']['broadcast']),'TICKET_LIFECYCLE_UNINSTALL_SITE':'/eth/downloader/api.go:'+str(b['anchors'].get('original_uninstall','no_plain_send'))}
        r=m.command([binary['path']],b['src'],q/'process',local,scope['owned_process_timeout_seconds']);r.update(slot,binary_sha256=binary['sha256'],fixture_sha256=m.sha(FIXTURE.read_bytes()),main_fixture_sha256=m.sha(MAIN_FIXTURE.read_bytes()),tmp_children_after=sorted(p.name for p in tmp.iterdir()));m.write(q/'receipt.json',r)
        print(json.dumps(dict(slot_id=slot['slot_id'],exit_code=r['exit_code'],remaining_live_group_members=r['remaining_live_group_members'])),flush=True);return r
    results=[]
    with concurrent.futures.ThreadPoolExecutor(max_workers=scope['parallelism'])as pool:
        for r in pool.map(execute,slots):results.append(r);m.write(out/'attempts.json',results)
    for role,b in built.items():
        if m.source_inventory(b['src'])!=b['binding']['applied_tracked']or m.git(b['src'],'status','--porcelain','--untracked-files=all').decode()!=b['status']or(b['src']/BRIDGE).read_bytes()!=FIXTURE.read_bytes()or(b['src']/MAIN).read_bytes()!=MAIN_FIXTURE.read_bytes():raise ValueError('source changed during program trials')
    post={r:{v:m.sha(Path(b['path']).read_bytes())for v,b in d['binaries'].items()}for r,d in built.items()}
    if any(post[r][v]!=b['sha256']for r,d in built.items()for v,b in d['binaries'].items()):raise ValueError('postrun binary changed')
    m.write(out/'binary-postrun-check.json',post);m.write(out/'receipt.json',dict(planned=len(slots),attempted=len(results),automatic_retries=0,source_commit=m.COMMIT,tracked_changes_only_declared_instrumented_api=True,program_additions=[BRIDGE,MAIN],source_lifecycle_hooks=True,original_package_tests_executed=False,production_patch=False,formal_completed=0,formal_total=9,independent_approved=0,FINAL=0,paid_or_external_LLM_calls=0,operational_nodes_started=0))

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
