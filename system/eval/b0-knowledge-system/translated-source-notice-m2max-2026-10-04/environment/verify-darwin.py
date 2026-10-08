from pathlib import Path
import json,subprocess,os,tarfile,io,hashlib
root=Path('/private/tmp/ks-translated-source-20261004');base=root/'darwin';repo=Path.cwd();archive=Path(json.loads((root/'darwin-package.stdout.txt').read_text())['archive']);sign=base/'signature';sign.mkdir()
def run(name,args,expected=0,env=None):
 r=subprocess.run([str(a) for a in args],capture_output=True,text=True,env=env);(base/(name+'.stdout.txt')).write_text(r.stdout);(base/(name+'.stderr.txt')).write_text(r.stderr);assert r.returncode==expected,(name,r.returncode,r.stderr);return r
run('keygen',['openssl','genpkey','-algorithm','ED25519','-out',sign/'private.pem']);run('public-key',['openssl','pkey','-in',sign/'private.pem','-pubout','-out',sign/'public.pem'])
run('sign',['python3',repo/'scripts/release-sidecar.py','--archive',archive,'--private-key',sign/'private.pem','--public-key',sign/'public.pem','--out-dir',sign/'sidecar'])
run('python-verify',['python3',repo/'scripts/verify-release.py','--public-key',sign/'public.pem','--release',sign/'sidecar/release.json','--signature',sign/'sidecar/release.json.sig','--archive',archive,'--target-os','darwin','--target-arch','arm64'])
unpacked=base/'unpacked';unpacked.mkdir()
with tarfile.open(archive) as t:t.extractall(unpacked,filter='data')
stage=next(unpacked.iterdir());inv=json.loads((stage/'third-party-licenses.json').read_text());entry=next(e for e in inv['modules'] if e['module']=='modernc.org/sqlite');notice=next(f for f in entry['license_files'] if f['path'].endswith('/SQLITE-LICENSE'))
source=Path('/Users/kevin/.gvm/pkgsets/go1.26.8/global/pkg/mod/modernc.org/sqlite@v1.54.0/SQLITE-LICENSE');assert (stage/notice['path']).read_bytes()==source.read_bytes();assert hashlib.sha256(source.read_bytes()).hexdigest()==notice['sha256'];assert entry['missing_required_notices']==[]
run('go-verify',[stage/'cks','package','verify','--public-key',sign/'public.pem','--release',sign/'sidecar/release.json','--signature',sign/'sidecar/release.json.sig','--archive',archive,'--target-os','darwin','--target-arch','arm64'])
env=dict(os.environ,KS_BIN_DIR=str(stage),KS_SANITIZE_RULES=str(stage/'policies/sanitization_rules.yaml'),KS_INSTALL_SMOKE_DIR=str(base/'install'))
run('install-smoke',['bash',repo/'scripts/wbs-install-smoke.sh'],env=env)
target=base/'tampered';target.mkdir();altered=target/archive.name
with tarfile.open(archive) as original,tarfile.open(altered,'w:gz') as output:
 for member in original.getmembers():
  if member.isfile():
   raw=original.extractfile(member).read()
   if member.name==stage.name+'/'+notice['path']:raw+=b'\nsynthetic tamper\n';member.size=len(raw)
   output.addfile(member,io.BytesIO(raw))
  else:output.addfile(member)
run('tamper-sign',['python3',repo/'scripts/release-sidecar.py','--archive',altered,'--private-key',sign/'private.pem','--public-key',sign/'public.pem','--out-dir',target/'sidecar'])
r=run('tamper-python',['python3',repo/'scripts/verify-release.py','--public-key',sign/'public.pem','--release',target/'sidecar/release.json','--signature',target/'sidecar/release.json.sig','--archive',altered,'--target-os','darwin','--target-arch','arm64'],expected=1);assert 'third-party license file differs' in r.stderr
run('tamper-go',[stage/'cks','package','verify','--public-key',sign/'public.pem','--release',target/'sidecar/release.json','--signature',target/'sidecar/release.json.sig','--archive',altered,'--target-os','darwin','--target-arch','arm64'],expected=1)
with tarfile.open(archive) as t:members=len(t.getmembers())
(base/'verification.json').write_text(json.dumps({'scope':'Darwin arm64 signed preview/mock install and resigned notice tamper; not legal/release approval','archive_sha256':hashlib.sha256(archive.read_bytes()).hexdigest(),'members':members,'modules':len(inv['modules']),'notices':sum(len(e['license_files']) for e in inv['modules'])+sum(len(e['license_files']) for e in inv['vendored_assets']),'sqlite_entry':entry,'notice_bytes_match_module_source':True,'both_verifiers_passed':True,'both_verifiers_rejected_resigned_notice_tamper':True,'installation_projects':3,'legal_review':'pending'},indent=2)+'\n')
print('new SQLite notice matches source; both signed verifiers/install/tamper checks passed')
