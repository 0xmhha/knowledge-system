from pathlib import Path
import hashlib,json,shutil,subprocess
scratch=Path('/private/tmp/ks-linux-env-20261004')
repo=Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
out=repo/'system/eval/b0-knowledge-system/linux-environment-m2max-2026-10-04'
out.mkdir()
def copy(source,target):
 target.parent.mkdir(parents=True,exist_ok=True)
 shutil.copyfile(source,target)
for p in (scratch/'probe').iterdir():
 if p.suffix in ('.json','.txt'): copy(p,out/'arm64-probes'/p.name)
for name in ('mac-tests.txt','mac-final-tests.txt','go-tests.txt','race-tests.txt','vet.txt','boundaries.txt','docs-check.txt','amd64-build.log','runtime-arm64.log','runtime-arm64-corrected.log','runtime-amd64.log','archive-initial-error.txt'):
 copy(scratch/name,out/'checks'/name)
for p in (scratch/'provenance').iterdir(): copy(p,out/'provenance'/p.name)
for side in ('before','after'):
 for name in ('matrix_environment.go','matrix_environment_test.go','matrix_environment_linux_data.go'):
  p=scratch/(side+'-source')/'cmd/cks/evalcli'/name
  if p.exists(): copy(p,out/'frozen-source'/side/name)
for p in (scratch/'probe-amd64').iterdir():
 if p.suffix in ('.json','.txt'): copy(p,out/'amd64-probes'/p.name)
for name in ('runtime-arm64','runtime-arm64-corrected','runtime-amd64'):
 root=scratch/name
 for p in root.iterdir():
  if p.is_file(): copy(p,out/name/p.name)
  elif p.name.startswith('matrix-'):
   for f in p.rglob('*'):
    if f.is_file(): copy(f,out/name/f.relative_to(root))
 for kind in ('empty-go','typescript','unsupported-python'):
  base=root/'install'/kind
  for p in base.iterdir():
   if p.is_file(): copy(p,out/name/'install'/kind/p.name)
  for p in (base/'src').iterdir():
   if p.is_file(): copy(p,out/name/'install'/kind/'live-source'/p.name)
  sources=(base/'dataset/current').resolve()/'sources'
  for p in sources.iterdir():
   if p.name=='manifest.json': copy(p,out/name/'install'/kind/'retained-source'/p.name)
   elif p.name=='blobs':
    for f in p.iterdir(): copy(f,out/name/'install'/kind/'retained-source/blobs'/f.name)
# The baseline collector was original HEAD; only the opt-in test was added.
copy(Path(__file__),out/'checks/archive-evidence.py')
head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()
assert (out/'frozen-source/before/matrix_environment.go').read_bytes()==subprocess.check_output(['git','show',head+':cmd/cks/evalcli/matrix_environment.go'],cwd=repo)
for name in ('matrix_environment.go','matrix_environment_linux_data.go'):
 assert (out/'frozen-source/after'/name).read_bytes()==(repo/'cmd/cks/evalcli'/name).read_bytes()
binaries={}
for arch,dirname in [('arm64','probe'),('amd64','probe-amd64')]:
 binaries[arch]={p.name:{'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'size':p.stat().st_size}
                 for p in (scratch/dirname).iterdir() if p.name.endswith('-after') or p.suffix=='.test'}
images={}
for arch in ('arm64','amd64'):
 for kind,tag in [('build','go1.25.13'),('runtime-smoke','bookworm')]:
  name=('knowledge-system-linux-build' if kind=='build' else 'knowledge-system-runtime-smoke')+':'+tag+'-'+arch
  images[name]=json.loads(subprocess.check_output(['docker','image','inspect',name,'--format','{{json .}}'],text=True))['Id']
summary={'schema_version':1,'derived_from_code':head,'scope':'actual minimal Linux containers/mock synthetic integration; amd64 runs via emulation on arm64 Mac',
 'status':'verified_with_limits','goal_complete':False,'official_quality_metrics':None,'official_protocol_approval':False,
 'build':{'go':'1.25.13','source':'provenance/frozen-source-inventory.json','original_git_metadata_present':False,'binary_version':'devel','binaries':binaries,'images':images},
 'runtime_options':{'cpus':1,'memory_bytes':536870912,'go_available':False,'ps_available':False,'other_host_work_excluded':False},
 'before':{'arm64_builder_errors':['hardware_unavailable:cpu_brand'],'arm64_minimal_runtime_errors':['hardware_unavailable:cpu_brand','hardware_unavailable:process_pressure']},
 'corrections':[{'type':'product','detail':'preserve ARM CPU identifiers; collect numeric process pressure from /proc without ps; record cgroup quota snapshots and reject quota drift'},
                {'type':'operator','detail':'initial make build-bins attempted output in a read-only source mount; changed output to /out'},
                {'type':'diagnostic_input','detail':'initial matrix omitted required knowledge date/subsystem; retained 24 rows, including 12 IsError responses, in runtime-arm64'}],
 'after':{'arm64':{'cases':3,'sdk_rows':72,'post_audit':'runtime-arm64-corrected/post-audit.json'},'amd64_emulated':{'cases':3,'sdk_rows':72,'verification':'runtime-amd64/verification.json'}},
 'checks':{'full_go':'passed','evalcli_race':'passed','vet':'passed','boundaries':'passed','documentation':'136 live documents passed'},
 'limitations':['mock verifies wiring, not BGE-M3 Linux performance or official semantic quality','no reviewed semantic store/pack; ontology arms preserve fallback evidence','CPU/RAM are kernel-visible totals; exposed cgroup values are separate raw constraints','numeric Linux process CPU uses lifetime averages and PID namespace scope','two observations do not prove exclusivity or absence of transient changes','amd64 is emulated, not native amd64 hardware','binaries are frozen working-source diagnostics, not signed release packages','official B0/B1/C0/C1 and human decisions remain pending']}
items=[]
for p in sorted(out.rglob('*')):
 if p.is_file(): items.append({'path':str(p.relative_to(out)),'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'size':p.stat().st_size})
summary['artifacts']=items
(out/'summary.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n')
print(len(items),'artifacts; bytes',sum(i['size'] for i in items))
