from pathlib import Path
import subprocess,json,tarfile,os
root=Path('/private/tmp/ks-fix14-20261004');repo=Path.cwd()
def run(base,name,args,env=None):
 r=subprocess.run([str(x) for x in args],env=env,capture_output=True,text=True);(base/(name+'.stdout.txt')).write_text(r.stdout);(base/(name+'.stderr.txt')).write_text(r.stderr);assert r.returncode==0,(name,r.stderr);return r
for arch in ['arm64','amd64']:
 base=root/arch;sign=base/'signature';sign.mkdir();private=sign/'private.pem';public=sign/'public.pem'
 run(base,'keygen',['openssl','genpkey','-algorithm','ED25519','-out',private]);run(base,'public-key',['openssl','pkey','-in',private,'-pubout','-out',public])
 first=next((base/'first').glob('*.tar.gz'));second=next((base/'second').glob('*.tar.gz'));assert first.name==second.name
 run(base,'sign',['python3',repo/'scripts/release-sidecar.py','--archive',first,'--private-key',private,'--public-key',public,'--out-dir',sign/'sidecar'])
 for name,archive in [('first',first),('second',second)]:
  run(base,name+'-python-verify',['python3',repo/'scripts/verify-release.py','--public-key',public,'--release',sign/'sidecar/release.json','--signature',sign/'sidecar/release.json.sig','--archive',archive,'--target-os','linux','--target-arch',arch])
 stage=first.name.removesuffix('.tar.gz')
 command=f'''set -euo pipefail
if command -v go >/dev/null; then echo unexpected-go >&2; exit 1; fi
mkdir -p /tmp/unpacked
tar -xzf /pair/first/{first.name} -C /tmp/unpacked
stage=/tmp/unpacked/{stage}
for pair in first second; do
 "$stage/cks" package verify --public-key /pair/signature/public.pem --release /pair/signature/sidecar/release.json --signature /pair/signature/sidecar/release.json.sig --archive "/pair/$pair/{first.name}" --target-os linux --target-arch {arch}
done
KS_BIN_DIR="$stage" KS_SANITIZE_RULES="$stage/policies/sanitization_rules.yaml" KS_INSTALL_SMOKE_DIR=/logs /scripts/wbs-install-smoke.sh
'''
 logs=base/'runtime';logs.mkdir()
 run(base,'runtime',['docker','run','--rm','--platform','linux/'+arch,'-v',str(base)+':/pair:ro','-v',str(logs)+':/logs','-v',str(repo/'scripts')+':/scripts:ro','knowledge-system-runtime-smoke:bookworm-'+arch,'bash','-lc',command])
 print(arch,'both identical archives share one test signature, both verifiers and three-project install passed',flush=True)
