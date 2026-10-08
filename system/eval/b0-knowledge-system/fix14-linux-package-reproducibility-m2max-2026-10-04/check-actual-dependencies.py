from pathlib import Path
import subprocess
root=Path('/private/tmp/ks-fix14-20261004');program="""from pathlib import Path
import json,tarfile,sys,importlib.util,hashlib
sys.path.insert(0,'/scripts');s=importlib.util.spec_from_file_location('package_host','/scripts/package-host.py');m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
archive=next(Path('/pair/first').glob('*.tar.gz'));rows=[]
with tarfile.open(archive) as tar:
 members=tar.getmembers();base=members[0].name.split('/')[0];manifest=json.load(tar.extractfile(base+'/manifest.json'))
 for name in ['ckg','ckv','cks']:
  data=tar.extractfile(base+'/'+name).read();sha=hashlib.sha256(data).hexdigest();assert sha==manifest['binaries'][name]['sha256'];binary=Path('/tmp')/name;binary.write_bytes(data);binary.chmod(0o700)
  first=m.native_dependencies(binary);second=m.native_dependencies(binary);assert first==second==manifest['native_dependencies'][name],name
  rows.append({'binary':name,'sha256':sha,'first':first,'second':second,'packaged':manifest['native_dependencies'][name],'identical':True})
print(json.dumps({'scope':'two real dependency inspections of each owned/verified Linux binary match packaged normalized metadata','binaries':rows},indent=2))
"""
for arch in ['arm64','amd64']:
 base=root/arch;args=['docker','run','--rm','--platform','linux/'+arch,'-v',str(base)+':/pair:ro','-v',str(Path.cwd()/'scripts')+':/scripts:ro','knowledge-system-runtime-smoke:bookworm-'+arch,'python3','-c',program];r=subprocess.run(args,capture_output=True,text=True);(base/'actual-dependencies.json').write_text(r.stdout);(base/'actual-dependencies.stderr.txt').write_text(r.stderr);assert r.returncode==0,r.stderr;print(arch,'three real binary inspections repeat exactly')
