"""Prepare a scoped N11 corpus and draft source rebinding; grants no review."""
import hashlib,json,shutil,subprocess,tempfile
from pathlib import Path
P=Path(__file__).resolve().parent;ROOT=P.parents[3]
OLD=P.parent/'refactoring-n12-proposed-lock-2026-10-08';CAND=P.parent/'refactoring-n11-coverage-candidates-2026-10-08'
sha=lambda b:hashlib.sha256(b).hexdigest()
def save(n,x):(P/n).write_text(json.dumps(x,ensure_ascii=False,indent=2)+'\n')
old=json.loads((OLD/'preparation.json').read_text());book=json.loads((CAND/'candidate-question-book.json').read_text());protocol=json.loads((CAND/'candidate-protocol.json').read_text());oldversion=Path(old['dataset_root'])/'n12-candidate'
archive=json.loads((oldversion/'sources/manifest.json').read_text());retained={x['path']:x for x in archive['files']if x['origin_id']=='repo'};graph=json.loads((oldversion/'graph/manifest.json').read_text());gf={x['path']for x in graph['files']};vector=json.loads((oldversion/'vector/manifest.json').read_text());vf={x['path']for x in vector['input_files']}
refs=[];required=set()
for q in book['questions']:
 for role,rs in [('positive_gold',q.get('evidence',[])),('absence_review_basis',q.get('abstention_basis_sources',[]))]:
  for r in rs:
   body=subprocess.check_output(['git','show',book['source_commit']+':'+r['path']],cwd=ROOT)
   span=b''.join(body.splitlines(keepends=True)[r['first']-1:r['last']]);assert sha(body)==r['file_sha256']and sha(span)==r['content_sha256']
   refs.append({'question_id':q['id'],'split':q['split'],'group':q['group'],'role':role,'ref':r,'original_commit':book['source_commit'],'previous_retained':r['path']in retained,'previous_CKG_indexed':r['path']in gf,'previous_CKV_indexed':r['path']in vf,'positive_answer_citation_allowed':role=='positive_gold'})
   required.add(r['path'])
missing_index=sorted({r['ref']['path']for r in refs if r['role']=='positive_gold'and not r['previous_CKG_indexed']});missing_retained=sorted({r['ref']['path']for r in refs if not r['previous_retained']});assert len(missing_index)==6 and len(missing_retained)==2
save('prior-candidate-scope-audit.json',{'question_count':16,'positive_gold_refs':14,'absence_basis_refs':2,'previous_dataset_identity':json.loads((oldversion/'dataset-identity.json').read_text()),'book_source_commit_matches_previous_candidate':False,'positive_files_missing_index':missing_index,'basis_files_missing_retained':missing_retained,'references':refs,'human_approved':0,'new_queries':0})
base=Path(tempfile.mkdtemp(prefix='ks-n11-held-corpus-',dir='/private/tmp'));source=base/'source';shutil.copytree(old['source'],source);shutil.copytree(Path(old['base'])/'bin',base/'bin')
original={p.relative_to(source).as_posix():sha(p.read_bytes())for p in source.rglob('*')if p.is_file()and '.git'not in p.relative_to(source).parts};assert len(original)==1416
added=[]
for path in missing_retained:
 data=subprocess.check_output(['git','show',book['source_commit']+':'+path],cwd=ROOT);target=source/path;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data);added.append({'path':path,'sha256':sha(data),'bytes':len(data),'origin_commit':book['source_commit']})
subprocess.run(['git','add','--force',*missing_retained],cwd=source,check=True,capture_output=True)
subprocess.run(['git','-c','user.name=N11 preparation tooling','-c','user.email=n11-preparation@localhost','-c','commit.gpgsign=false','commit','-m','Retain bounded N11 absence review inputs'],cwd=source,check=True,capture_output=True)
commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=source,text=True).strip();assert not subprocess.check_output(['git','status','--porcelain'],cwd=source)
for path,d in original.items():assert sha((source/path).read_bytes())==d
for r in refs:
 body=(source/r['ref']['path']).read_bytes();assert sha(body)==r['ref']['file_sha256'];assert sha(b''.join(body.splitlines(keepends=True)[r['ref']['first']-1:r['ref']['last']]))==r['ref']['content_sha256'];r['new_corpus_commit']=commit;r['original_bytes_unchanged']=True
book['previous_question_book_sha256']=sha((CAND/'candidate-question-book.json').read_bytes());book['source_commit']=commit;book['purpose']='N11 scoped held corpus draft; original question/gold bytes unchanged; corpus/split/gold/independence/contamination/absence scope unapproved'
for q in book['questions']+book['excluded_candidate_questions']:q['source_commit']=commit
protocol['source_commit']=commit;protocol['previous_protocol_sha256']=sha((CAND/'candidate-protocol.json').read_bytes());protocol['candidate_book_sha256']=sha((json.dumps(book,ensure_ascii=False,indent=2)+'\n').encode());protocol['execution_allowed']=False;protocol['release_eligible']=False;protocol['freeze']=None;protocol['results']=None
save('candidate-question-book.json',book);save('candidate-protocol.json',protocol);save('source-rebinding.json',refs);save('added-source-inputs.json',added);save('original-corpus-inputs.json',original)
# Every required Go and Markdown goes into actual filelist; JSON/YAML are retained only.
filelist=(OLD/'filelist.yaml').read_text();extra=sorted(required-set(old['required_engine_index_files']))
assert len(extra)==8
filelist=filelist.replace('exclude_globs: []\n',''.join('  - '+json.dumps(path)+'\n'for path in extra)+'exclude_globs: []\n');(P/'filelist.yaml').write_text(filelist);shutil.copyfile(OLD/'build-sources.json',P/'build-sources.json')
prep=dict(old);prep.update({'base':str(base),'source':str(source),'source_commit':commit,'dataset_root':str(base/'dataset'),'project_id':'knowledge-system','runtime_source_equivalent_revision':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'runtime_revision':'3e204ee3461fc94097e08a408b5e4aa3ae0ca010','runtime_reused':True,'capture_files_expected':1419,'required_gold_source_paths':sorted(required),'extra_selected_input_paths':extra,'human_approved':0,'new_queries':0,'scope_approved':False,'original_book_source_revision':json.loads((CAND/'candidate-question-book.json').read_text())['source_commit']})
save('preparation.json',prep)
runner=(OLD/'run-held-setup.py').read_text().replace('refactoring-n12-proposed-lock-2026-10-08','refactoring-n11-held-corpus-2026-10-08').replace('ks-n12-proposed-lock-','ks-n11-held-corpus-').replace('n12-candidate','n11-candidate');(P/'run-held-setup.py').write_text(runner)
prior=json.loads((P.parent/'refactoring-n12-coordinated-preparation-2026-10-08/runtime-binaries.json').read_text());bindings={}
for name,row in prior['binaries'].items():
 target=base/'bin'/name;assert sha(target.read_bytes())==row['sha256'];bindings[name]={'path':str(target),'sha256':row['sha256']}
save('runtime-binaries.json',{'original_build_revision':prior['source_revision'],'current_equivalent_source_revision':prep['runtime_source_equivalent_revision'],'reused_binaries':True,'new_build_count':0,'binaries':bindings})
print(json.dumps({'base':str(base),'source_commit':commit,'old_index_gap_Go_files':6,'basis_inputs_added':2,'gold_basis_ref_bytes_verified':16,'human_approved':0}))
