from pathlib import Path
import json,hashlib,subprocess,tempfile,os
root=Path.cwd();out=root/'system/eval/b0-knowledge-system/refactoring-n11-eval-corpus-2026-10-09'
sha=lambda b:hashlib.sha256(b).hexdigest()
def save(name,x):(out/name).write_text(json.dumps(x,ensure_ascii=False,indent=2)+'\n')
revision=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
L=json.loads((root/'docs/spec-driven/REFACTORING-PROGRESS.json').read_text())['active_source_binding'];previous=json.loads(subprocess.check_output(['git','show',L['revision']+':'+L['manifest']]))
for p,d in {**previous['source_sha256'],**previous['approved_input_sha256']}.items():assert sha((root/p).read_bytes())==d,p
entries=subprocess.check_output(['git','ls-tree','-r',revision],text=True).splitlines()
requested={p for p in previous['source_sha256'] if not p.startswith('system/eval/')}
requested.update(['docs/spec-driven/PUBLIC-CONTRACT-V2.md','docs/spec-driven/SOURCE-SNAPSHOT-ADR.md','docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md','docs/spec-driven/ontology-pilot-v2.yaml','docs/spec-driven/ontology-pilot-functional-v2.yaml','docs/spec-driven/spec-pilot-v3.yaml','docs/spec-driven/C1-OPERATIONS-REVIEW.md','docs/spec-driven/C1-RECOVERY-VALIDATION.md'])
base=Path(tempfile.mkdtemp(prefix='ks-n11-eval-',dir='/private/tmp'));source=base/'source';source.mkdir();(base/'bin').mkdir()
files={};modes={}
selected=[]
for line in entries:
 meta,path=line.split('\t',1);mode,kind,oid=meta.split()
 if path.startswith(('cmd/','internal/','pkg/')) or path in requested:
  assert kind=='blob' and mode in ['100644','100755'],(path,meta)
  selected.append((path,mode,oid))
proc=subprocess.Popen(['git','cat-file','--batch'],stdin=subprocess.PIPE,stdout=subprocess.PIPE)
for path,mode,oid in selected:
 proc.stdin.write((oid+'\n').encode());proc.stdin.flush();header=proc.stdout.readline().decode().split();data=proc.stdout.read(int(header[2]));assert proc.stdout.read(1)==b'\n'
 assert sha(data)==sha((root/path).read_bytes()),path
 target=source/path;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data);target.chmod(0o755 if mode=='100755' else 0o644)
 files[path]=sha(data);modes[path]=mode
proc.stdin.close();assert proc.wait()==0
assert requested<=set(files),sorted(requested-set(files))
import fnmatch
bad=[x for x in files if x.startswith('system/eval/') or fnmatch.fnmatch(x,'docs/spec-driven/N11-*') or fnmatch.fnmatch(x,'docs/spec-driven/N12-*')]
assert not bad,bad
subprocess.run(['git','init','-q','-b','preparation'],cwd=source,check=True)
subprocess.run(['git','add','--force','--all'],cwd=source,check=True)
subprocess.run(['git','-c','user.name=knowledge-system source preparation','-c','user.email=source-preparation@local.invalid','-c','commit.gpgsign=false','commit','-qm','Capture N11 evaluation corpus: current implementation sources and selected documents, evaluation material excluded'],cwd=source,check=True)
commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=source,text=True).strip()
assert not subprocess.check_output(['git','status','--porcelain'],cwd=source)
save('source-inventory.json',{'original_revision':revision,'derived_commit':commit,'files':files,'modes':modes,'original_history_copied':False,'user_untracked_content_included':False,'scope':'N11 evaluation corpus: committed cmd/internal/pkg plus active non-evaluation dependencies and selected documents; docs/spec-driven/N11-*, N12-* and system/eval/** excluded'})
save('preparation.json',{'base':str(base),'source':str(source),'dataset_root':str(base/'dataset'),'project_id':'knowledge-system','source_commit':commit,'runtime_revision':revision,'source_file_count':len(files),'original_source_sha_bindings_preserved':True,'approved_input_sha256':previous['approved_input_sha256'],'old_projection_review_carried_forward':False,'actual_human_review_count':0,'reviewer_label':'opus5.5 검토','reviewer_kind':'ai_model_label_designated_by_user'})
(base/'source.bundle').parent.mkdir(exist_ok=True);subprocess.run(['git','bundle','create',str(out/'source.bundle'),'HEAD'],cwd=source,check=True,capture_output=True)
print(json.dumps({'base':str(base),'source_file_count':len(files),'commit':commit}))
