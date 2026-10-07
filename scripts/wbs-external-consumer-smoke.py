#!/usr/bin/env python3
"""Compile a separate public Go consumer and exercise extracted mock installs.

Uses owned structural fixtures, never N11 gold/FINAL or a production model.
Requires wbs-package-smoke.sh output. No source upload or release approval.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def load(path):
    return json.loads(path.read_bytes())

def audit_v2(pack, source):
    clone = copy.deepcopy(pack)
    digest = clone['metadata'].pop('integrity_hash')
    # These owned fixtures use ASCII strings and integral JSON numbers.
    # This is a fixture audit, not a general-purpose implementation of JCS.
    canonical = json.dumps(clone, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()
    if sha(canonical) != digest:
        raise ValueError('v2 raw integrity mismatch')
    seen = set()
    for c in pack['citations']:
        raw = subprocess.check_output(['git', '-C', str(source), 'show', c['commit_hash']+':'+c['file']])
        span = b''.join(raw.splitlines(keepends=True)[c['start_line']-1:c['end_line']])
        if sha(raw) != c['file_sha256'] or sha(span) != c['content_sha256']:
            raise ValueError('v2 source identity mismatch')
        seen.add(json.dumps(c, sort_keys=True))
    total = 0
    for b in pack['bodies']:
        c = b['citation']
        if json.dumps(c, sort_keys=True) not in seen:
            raise ValueError('v2 uncited body')
        raw = subprocess.check_output(['git', '-C', str(source), 'show', c['commit_hash']+':'+c['file']])
        expected = b''.join(raw.splitlines(keepends=True)[c['start_line']-1:c['end_line']]).decode()
        if b['text'] != expected:
            raise ValueError('v2 body differs from retained source span')
        total += len(b['text'].encode())
    if total > 32000:
        raise ValueError('v2 body budget exceeded')
    return {'citations': len(pack['citations']), 'bodies': len(pack['bodies']), 'body_bytes': total}

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--repo', type=Path, default=ROOT, help='clean source used to build the archive')
    p.add_argument('--package-root', type=Path, required=True)
    p.add_argument('--out-dir', type=Path, required=True)
    p.add_argument('--expect-os', choices=['linux','darwin'])
    p.add_argument('--expect-arch', choices=['amd64','arm64'])
    a=p.parse_args()
    repo, package, out = a.repo.resolve(), a.package_root.resolve(), a.out_dir.resolve()
    host_os = platform.system().lower()
    host_arch = {'x86_64':'amd64','aarch64':'arm64'}.get(platform.machine().lower(),platform.machine().lower())
    if (a.expect_os and host_os!=a.expect_os) or (a.expect_arch and host_arch!=a.expect_arch):
        raise ValueError('caller host does not match expected platform')
    commit = subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip()
    if subprocess.check_output(['git','-C',str(repo),'status','--porcelain'],text=True).strip():
        raise ValueError('source checkout must be clean')
    receipt=load(package/'package.json')
    archive=Path(receipt['archive'])
    if sha(archive.read_bytes())!=receipt['sha256']:
        raise ValueError('archive hash mismatch')
    stages=list((package/'unpacked').iterdir())
    if len(stages)!=1 or not stages[0].is_dir():
        raise ValueError('expected one extracted package')
    stage=stages[0]
    manifest=load(stage/'manifest.json')
    if manifest['commit']!=commit or manifest['dirty'] or manifest['host_os']!=host_os or manifest['host_arch']!=host_arch:
        raise ValueError('clean native package/source identity mismatch')
    hashes={name:sha((stage/name).read_bytes()) for name in ('cks','ckg','ckv')}
    if any(hashes[name]!=manifest['binaries'][name]['sha256'] for name in hashes):
        raise ValueError('extracted binary hash mismatch')
    out.mkdir(parents=True,exist_ok=False)
    commands=[]
    env=dict(os.environ,GOPROXY='off',GOSUMDB='off',GOTOOLCHAIN='local')
    def write(path,value):
        path.write_text(json.dumps(value,indent=2,ensure_ascii=False)+'\n')
    def run(name,args,cwd=None,expected=0):
        begin=time.monotonic_ns()
        r=subprocess.run([str(x) for x in args],cwd=cwd,env=env,capture_output=True,timeout=180)
        (out/(name+'.stdout.txt')).write_bytes(r.stdout)
        (out/(name+'.stderr.txt')).write_bytes(r.stderr)
        commands.append({'id':name,'argv':[str(x) for x in args],'exit_code':r.returncode,'elapsed_ns_diagnostic_only':time.monotonic_ns()-begin})
        write(out/'commands.json',commands)
        if r.returncode!=expected:
            raise RuntimeError(f'{name}: exit {r.returncode}, expected {expected}; see saved stderr')
        return r.stdout
    module=out/'consumer-module';module.mkdir()
    (module/'go.mod').write_text('module example.invalid/external-knowledge-consumer\n\ngo 1.25.13\n\nrequire github.com/0xmhha/knowledge-system v0.0.0\n\nreplace github.com/0xmhha/knowledge-system => '+json.dumps(str(repo))+'\n')
    template=ROOT/'scripts/testdata/external-consumer/main.go.txt'
    (module/'main.go').write_bytes(template.read_bytes())
    run('consumer-build',['go','build','-mod=mod','-o',module/'consumer','.'],cwd=module)
    run('consumer-build-info',['go','version','-m',module/'consumer'])
    gomod=run('consumer-gomod',['go','env','GOMOD'],cwd=module).decode().strip()
    if Path(gomod)!=module/'go.mod':
        raise ValueError('consumer did not compile in its separate module')
    if b'/internal/' in template.read_bytes():
        raise ValueError('consumer must use public package imports only')
    summaries=[]
    for kind in ('empty-go','typescript','unsupported-python'):
        base=package/'installs'/kind
        doctor=load(base/'doctor-rollback.json')
        packs=[]
        for number in (1,2):
            raw=run(f'consumer-{kind}-{number}',[module/'consumer','--binary',stage/'cks','--config',base/'mcp.yaml'])
            pack=json.loads(raw); packs.append(pack)
            if not pack['citations'] or any(c['commit_hash']!=doctor['indexed_commit'] for c in pack['citations']):
                raise ValueError('legacy consumer citation source mismatch')
        if packs[0]['citations']!=packs[1]['citations']:
            raise ValueError('legacy consumer restart changed citation identity')
        summaries.append({'project':kind,'format':'v1','restart_calls':2,'citations':len(packs[0]['citations']),'commit':doctor['indexed_commit']})
    legacy=copy.deepcopy(packs[0]);legacy['metadata']['integrity_hash']='0'*64
    write(out/'bad-legacy-integrity.json',legacy)
    run('reject-legacy-integrity',[module/'consumer','--pack-file',out/'bad-legacy-integrity.json'],expected=1)
    legacy=copy.deepcopy(packs[0]);legacy['citations'][0]['start_line']=0
    write(out/'bad-legacy-coordinate.json',legacy)
    run('reject-legacy-coordinate',[module/'consumer','--pack-file',out/'bad-legacy-coordinate.json'],expected=1)
    src=out/'pinned-source';src.mkdir()
    (src/'go.mod').write_text('module example.invalid/nativewire\n\ngo 1.25.13\n')
    (src/'README.md').write_text('# Native consumer guide\nWireAlpha is the fixture function.\n')
    (src/'main.go').write_text('package nativewire\n\nfunc WireAlpha() {}\n')
    run('pinned-git-init',['git','init','-q',src])
    run('pinned-git-add',['git','-C',src,'add','.'])
    run('pinned-git-commit',['git','-C',src,'-c','commit.gpgsign=false','-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','structural consumer fixture'])
    dataset=out/'pinned-dataset'
    run('pinned-setup',[stage/'cks','setup','--src',src,'--out',dataset,'--version','v2','--project-id','external-consumer','--embedder','mock'])
    config=out/'pinned-mcp.yaml'
    run('pinned-config',[stage/'cks','mcp','gen-config','--dataset-dir',dataset/'current','--source-root',src,'--sanitize-rules',stage/'policies/sanitization_rules.yaml','--out',config])
    text=config.read_text()
    for old,new in [('provider: ""','provider: mock'),('embed_model: bge-m3','embed_model: mock-feature-hash-v1'),('mcp_stdio: false','mcp_stdio: true'),('transport: http','transport: stdio')]:
        if old not in text:raise ValueError('generated fixture config changed: '+old)
        text=text.replace(old,new)
    config.write_text(text)
    before=hashes.copy()
    raw=run('consumer-pinned-v2',[module/'consumer','--binary',stage/'cks','--config',config,'--format','v2','--prompt','Where is WireAlpha implemented?'])
    pack=json.loads(raw)
    v2=audit_v2(pack,src)
    changed=copy.deepcopy(pack);changed['coordinates']['dataset_id']='0'*64
    write(out/'bad-v2-coordinate.json',changed)
    run('reject-v2-coordinate',[module/'consumer','--pack-file',out/'bad-v2-coordinate.json','--format','v2'],expected=1)
    changed=copy.deepcopy(pack);changed['metadata']['integrity_hash']='0'*64
    try:audit_v2(changed,src)
    except ValueError as e:
        if str(e)!='v2 raw integrity mismatch':raise
    else:raise ValueError('tampered v2 integrity was accepted')
    if before!={name:sha((stage/name).read_bytes()) for name in before}:
        raise ValueError('package binary changed during consumer calls')
    write(out/'summary.json',{'schema_version':1,'status':'structural_consumer_controls_passed_task_incomplete','source_commit':commit,'host_os':host_os,'host_arch':host_arch,'kernel':platform.release(),'go_version':subprocess.check_output(['go','version'],text=True).strip(),'source_checkout_clean':True,'archive_sha256':receipt['sha256'],'binary_sha256':hashes,'external_gomod':gomod,'consumer_template_sha256':sha(template.read_bytes()),'legacy_projects':summaries,'v2':v2,'negative_controls':4,'real_model_calls':0,'new_final_calls':0,'third_party_consumer_acceptance':None,'native_linux_amd64_accepted':False,'release_approved':False,'limits':['owned mock fixtures, local replaced Go module, no independent consumer owner','binary upgrade from deployed production version not exercised','timings diagnostic only; no latency or retrieval quality verdict','v2 raw integrity audit restricted to these fixtures; not complete JCS certification']})
    print('External public-module consumer: PASS; legacy 6 MCP calls, v2 1 call, 4 rejection controls; N15 incomplete')

if __name__=='__main__':
    try:main()
    except (ValueError,RuntimeError,OSError,KeyError,subprocess.SubprocessError) as e:
        print('external consumer smoke: '+str(e),file=sys.stderr);sys.exit(1)
