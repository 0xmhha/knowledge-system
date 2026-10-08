from pathlib import Path
import sys,json,hashlib,subprocess,os,tarfile
source=Path('/source');out=Path('/out')
subprocess.run(['python3',source/'scripts/prepare-platform-fixture.py','--source',source,'--out','/worktree'],check=True,stdout=(out/'fixture-copy-count.txt').open('w'))
work=Path('/worktree');inputs=json.loads(Path('/inputs/frozen-source-inputs.json').read_text());checks=[]
for x in inputs['files']:
 p=work/x['path'];data=os.fsencode(os.readlink(p)) if p.is_symlink() else p.read_bytes();assert len(data)==x['bytes'] and hashlib.sha256(data).hexdigest()==x['sha256'] and bool(p.lstat().st_mode&0o111)==x['executable'],x['path'];checks.append(x)
(out/'source-copy-audit.json').write_text(json.dumps({'scope':'actual builder copy checked before synthetic Git commit/build','base_commit':inputs['base_commit'],'checked_files':len(checks),'mismatches':[]},indent=2)+'\n')
for args in [['git','init','-q'],['git','config','user.name','Fixture'],['git','config','user.email','fixture@example.invalid'],['git','config','commit.gpgsign','false'],['git','add','-A'],['git','commit','-qm','fixed source for Linux package reproducibility']]:subprocess.run(args,cwd=work,check=True)
(out/'synthetic-source-commit.txt').write_text(subprocess.check_output(['git','rev-parse','HEAD'],cwd=work,text=True))
# Every invocation builds the three binaries through the actual public packager.
for name in ['first','second']:
 r=subprocess.run(['python3','scripts/package-host.py','--out-dir','/out/'+name],cwd=work,capture_output=True,text=True);(out/(name+'-package.json')).write_text(r.stdout);(out/(name+'-package.stderr.txt')).write_text(r.stderr);assert r.returncode==0,r.stderr
first=next((out/'first').glob('*.tar.gz'));second=next((out/'second').glob('*.tar.gz'));audit=[]
for archive in [first,second]:
 entries=[]
 with tarfile.open(archive) as t:
  for m in t.getmembers():
   row={'path':m.name.split('/',1)[1],'mode':oct(m.mode),'bytes':m.size,'type':'file' if m.isfile() else 'directory'}
   if m.isfile():row['sha256']=hashlib.sha256(t.extractfile(m).read()).hexdigest()
   entries.append(row)
 audit.append({'archive':str(archive),'bytes':archive.stat().st_size,'sha256':hashlib.sha256(archive.read_bytes()).hexdigest(),'entries':entries})
(out/'pair-audit.json').write_text(json.dumps({'source_commit':(out/'synthetic-source-commit.txt').read_text().strip(),'archives':audit,'entries_identical':audit[0]['entries']==audit[1]['entries'],'archive_identical':audit[0]['sha256']==audit[1]['sha256']},indent=2)+'\n')
assert audit[0]['entries']==audit[1]['entries'] and audit[0]['sha256']==audit[1]['sha256'],'same-source package bytes differ'
print('two fresh public packager builds match:',audit[0]['sha256'])
