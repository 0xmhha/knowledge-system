from pathlib import Path
import subprocess,json,hashlib,os,sys
out=Path('/out');source=Path('/source');work=Path('/worktree')
subprocess.run(['python3',source/'scripts/prepare-platform-fixture.py','--source',source,'--out',work],check=True,stdout=(out/'fixture-copy-count.txt').open('w'))
inputs=json.loads(Path('/inputs/frozen-source-inputs.json').read_text())
for entry in inputs['files']:
 p=work/entry['path'];raw=os.fsencode(os.readlink(p)) if p.is_symlink() else p.read_bytes();assert len(raw)==entry['bytes'] and hashlib.sha256(raw).hexdigest()==entry['sha256'] and bool(p.lstat().st_mode&0o111)==entry['executable'],entry['path']
(out/'source-copy-audit.json').write_text(json.dumps({'base_commit':inputs['base_commit'],'actual_builder_checked_files':len(inputs['files']),'mismatches':[]},indent=2)+'\n')
env=dict(os.environ,GIT_AUTHOR_DATE='2026-10-04T00:00:00Z',GIT_COMMITTER_DATE='2026-10-04T00:00:00Z')
for args in [['git','init','-q'],['git','config','user.name','Fixture'],['git','config','user.email','fixture@example.invalid'],['git','config','commit.gpgsign','false'],['git','add','-A'],['git','commit','-qm','SQLite notice source audit fixture']]:subprocess.run(args,cwd=work,env=env,check=True)
(out/'synthetic-source-commit.txt').write_text(subprocess.check_output(['git','rev-parse','HEAD'],cwd=work,text=True))
def run(name,args):
 r=subprocess.run([str(a) for a in args],cwd=work,env=env,capture_output=True,text=True);(out/(name+'.stdout.txt')).write_text(r.stdout);(out/(name+'.stderr.txt')).write_text(r.stderr);assert r.returncode==0,(name,r.stderr);return r
run('package',['python3','scripts/package-host.py','--out-dir',out/'package'])
r=run('go-deps',['go','list','-deps','-json','./cmd/graph','./cmd/vector','./cmd/cks']);(out/'go-deps.json').write_text(r.stdout);raw=r.stdout;d=json.JSONDecoder();selected=[]
while raw.strip():
 p,n=d.raw_decode(raw.lstrip());raw=raw.lstrip()[n:]
 if (p.get('Module') or {}).get('Path','').startswith('modernc.org/'):selected.append(p)
(out/'modernc-selected-packages.json').write_text(json.dumps(selected,indent=2)+'\n')
r=run('go-environment',['go','env','-json','GOOS','GOARCH','GOVERSION','GOROOT','GOMODCACHE','CGO_ENABLED']);(out/'go-environment.json').write_text(r.stdout)
r=run('binary-build-info',['go','version','-m','bin/ckg','bin/ckv','bin/cks']);(out/'binary-build-info.txt').write_text(r.stdout)
run('module-verify',['go','mod','verify']);run('translated-audit',['python3','/inputs/audit-translated-generic.py',out]);print('actual Linux package/source ZIP/module audit passed',flush=True)
