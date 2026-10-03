from pathlib import Path
import subprocess,json,tarfile,hashlib,sys,shutil,io
root=Path('/private/tmp/ks-translated-source-20261004');repo=Path.cwd();arch=sys.argv[1];base=root/('linux-'+arch);archive=next((base/'package').glob('*.tar.gz'));sign=base/'signature';sign.mkdir();private=root/'darwin/signature/private.pem';public=sign/'public.pem';shutil.copyfile(root/'darwin/signature/public.pem',public)
def run(name,args,expected=0):
 r=subprocess.run([str(a) for a in args],capture_output=True,text=True);(base/(name+'.stdout.txt')).write_text(r.stdout);(base/(name+'.stderr.txt')).write_text(r.stderr);assert r.returncode==expected,(name,r.returncode,r.stderr);return r
run('sign',['python3',repo/'scripts/release-sidecar.py','--archive',archive,'--private-key',private,'--public-key',public,'--out-dir',sign/'sidecar'])
run('python-verify',['python3',repo/'scripts/verify-release.py','--public-key',public,'--release',sign/'sidecar/release.json','--signature',sign/'sidecar/release.json.sig','--archive',archive,'--target-os','linux','--target-arch',arch])
with tarfile.open(archive) as t:
 members=t.getmembers();stage=Path(members[0].name).parts[0];inv=json.load(t.extractfile(next(m for m in members if m.name==stage+'/third-party-licenses.json')));entry=next(e for e in inv['modules'] if e['module']=='modernc.org/sqlite');notice=next(f for f in entry['license_files'] if f['path'].endswith('/SQLITE-LICENSE'));raw=t.extractfile(stage+'/'+notice['path']).read()
 source=Path('/Users/kevin/.gvm/pkgsets/go1.26.8/global/pkg/mod/modernc.org/sqlite@v1.54.0/SQLITE-LICENSE').read_bytes();assert raw==source and hashlib.sha256(raw).hexdigest()==notice['sha256']
 command=f'''set -euo pipefail
if command -v go >/dev/null; then echo unexpected-go >&2; exit 1; fi
mkdir /tmp/unpacked
tar -xzf /pair/package/{archive.name} -C /tmp/unpacked
stage=/tmp/unpacked/{stage}
"$stage/cks" package verify --public-key /pair/signature/public.pem --release /pair/signature/sidecar/release.json --signature /pair/signature/sidecar/release.json.sig --archive /pair/package/{archive.name} --target-os linux --target-arch {arch}
KS_BIN_DIR="$stage" KS_SANITIZE_RULES="$stage/policies/sanitization_rules.yaml" KS_INSTALL_SMOKE_DIR=/logs /scripts/wbs-install-smoke.sh
"$stage/cks" package verify --public-key /pair/signature/public.pem --release /pair/tampered/sidecar/release.json --signature /pair/tampered/sidecar/release.json.sig --archive /pair/tampered/{archive.name} --target-os linux --target-arch {arch} >/logs/tamper-go.stdout.txt 2>/logs/tamper-go.stderr.txt && exit 44
status=$?
[[ "$status" == 1 ]]
'''
 target=base/'tampered';target.mkdir();altered=target/archive.name
 with tarfile.open(archive) as original,tarfile.open(altered,'w:gz') as output:
  for member in original.getmembers():
   if member.isfile():
    data=original.extractfile(member).read()
    if member.name==stage+'/'+notice['path']:data+=b'\nsynthetic tamper\n';member.size=len(data)
    output.addfile(member,io.BytesIO(data))
   else:output.addfile(member)
run('tamper-sign',['python3',repo/'scripts/release-sidecar.py','--archive',altered,'--private-key',private,'--public-key',public,'--out-dir',target/'sidecar'])
r=run('tamper-python',['python3',repo/'scripts/verify-release.py','--public-key',public,'--release',target/'sidecar/release.json','--signature',target/'sidecar/release.json.sig','--archive',altered,'--target-os','linux','--target-arch',arch],expected=1);assert 'third-party license file differs' in r.stderr
logs=base/'runtime';logs.mkdir();run('runtime',['docker','run','--rm','--platform','linux/'+arch,'--network','none','-v',str(base)+':/pair:ro','-v',str(logs)+':/logs','-v',str(repo/'scripts')+':/scripts:ro','knowledge-system-runtime-smoke:bookworm-'+arch,'bash','-lc',command])
assert json.loads((logs/'tamper-go.stderr.txt').read_text())['code']=='operation_failed'
(base/'verification.json').write_text(json.dumps({'arch':arch,'execution':'native arm64 in Docker VM' if arch=='arm64' else 'amd64 emulation on arm64 host','archive_sha256':hashlib.sha256(archive.read_bytes()).hexdigest(),'members':len(members),'modules':len(inv['modules']),'notices':sum(len(e['license_files']) for e in inv['modules'])+sum(len(e['license_files']) for e in inv['vendored_assets']),'sqlite_entry':entry,'notice_bytes_match_module_source':True,'both_verifiers_passed':True,'both_verifiers_rejected_resigned_notice_tamper':True,'installation_projects':3,'legal_review':'pending'},indent=2)+'\n');print(arch,'new notice/source/signature/mock install and resigned tamper checks passed')
