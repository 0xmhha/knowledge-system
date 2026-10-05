import hashlib,json,os,subprocess,urllib.request
from pathlib import Path
p=Path(__file__).resolve().parent;root=Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
b=json.loads((root/'system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json').read_bytes());f=next(f for f in b['fixtures'] if f['id']=='F-01-DEV')
m=json.loads(Path('/private/tmp/ks-b0-dynamic-dev-20261003/materialization.json').read_bytes());s=next(x for x in m['fixtures'] if x['id']==f['id'])['states'][0]
commands=[]
def sha(b):return hashlib.sha256(b).hexdigest()
def run(label,args,strict=False):
 env=os.environ.copy()
 if strict:env['CKV_REQUIRE_COMPLETE_EMBEDDINGS']='1'
 r=subprocess.run(args,env=env,capture_output=True,timeout=600)
 (p/(label+'-stdout.txt')).write_bytes(r.stdout);(p/(label+'-stderr.txt')).write_bytes(r.stderr)
 commands.append({'label':label,'command':args,'exit_code':r.returncode,'explicit_environment':{'CKV_REQUIRE_COMPLETE_EMBEDDINGS':'1'} if strict else {}})
 (p/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
 assert r.returncode==0,(label,r.returncode,r.stderr.decode());return r
repo=p/'source'
run('clone',['git','clone','--quiet','--no-local','/private/tmp/ks-b0-dynamic-dev-20261003/F-01-DEV/repo',str(repo)])
run('checkout',['git','-C',str(repo),'checkout','--quiet','--detach',s['commit']])
for file,h in s['source_sha256'].items():assert sha((repo/file).read_bytes())==h and (repo/file).read_bytes()==f['sources'][file].encode()
assert subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip()==s['commit']
def model():
 with urllib.request.urlopen('http://127.0.0.1:11434/api/tags',timeout=20) as r:d=json.load(r)
 return next(m for m in d['models'] if m['name']=='bge-m3:latest')
mb=model();assert mb['digest']=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
(p/'model-before.json').write_text(json.dumps(mb,indent=2)+'\n')
cks='/private/tmp/ks-f0405-preparation-20261005/fixed-bin/cks';ckv='/private/tmp/ks-f0405-preparation-20261005/fixed-bin/ckv';ckg='/private/tmp/ks-f02-preparation-20261005/bin/ckg';probe='/private/tmp/ks-f02-preparation-20261005/bin/probe'
run('strict-setup',[cks,'setup','--src',str(repo),'--out',str(p/'dataset'),'--version','diagnostic','--project-id',s['project_id'],'--embedder','ollama','--model-name','bge-m3:latest','--ollama-url','http://127.0.0.1:11434','--query-prefix-policy','registry','--cks-bin',cks,'--vector-bin',ckv,'--graph-bin',ckg],True)
v=p/'dataset/diagnostic/vector';manifest_raw=(v/'manifest.json').read_bytes();manifest=json.loads(manifest_raw)
filter={'language':'go','path':'src/*.go','symbol_kinds':['Function'],'chunk_kinds':['symbol'],'exclude_tests':True}
assert filter==f['filters'];(p/'filter.json').write_text(json.dumps(filter,indent=2)+'\n')
before={str(x):sha(x.read_bytes()) for x in (v/'vector.db',v/'manifest.json',p/'dataset/diagnostic/graph/graph.db')}
r=run('probe',[probe,'--vector-dir',str(v),'--query',f['queries'][0]['prompt'],'--filter-file',str(p/'filter.json'),'--k',str(f['k']),'--max-exact-candidates',str(f['expected']['candidate_limit_subcase']['max_exact_candidates'])])
raw=r.stdout;q=json.loads(raw)
controls={'schema_version':1,'diagnostic_only':True,'probe_sha256':sha(raw),'expected_citations':[{'file':file,'start_line':f['expected']['lines'][0],'end_line':f['expected']['lines'][1],'commit_hash':s['commit']} for file in f['expected']['eligible_files']],**{k:q[k] for k in ('coordinates','embedding_identity','prompt','filter','k','max_exact_candidates')},
'f01':{'k':f['k'],'max_exact_candidates':2,'eligible_count':5,'complete_hit_count':5,'eligible_files':f['expected']['eligible_files']}}
(p/'controls.json').write_text(json.dumps(controls,indent=2)+'\n')
run('reader',['python3',str(root/'scripts/b0-summarize-vector-probe.py'),'--probe',str(p/'probe-stdout.txt'),'--manifest',str(v/'manifest.json'),'--controls',str(p/'controls.json'),'--source-root',str(repo),'--out',str(p/'diagnostic-report.json')])
after={path:sha(Path(path).read_bytes()) for path in before};assert before==after
for file,h in s['source_sha256'].items():assert sha((repo/file).read_bytes())==h
ma=model();assert mb==ma;(p/'model-after.json').write_text(json.dumps(ma,indent=2)+'\n')
(p/'provenance.json').write_text(json.dumps({'implementation_base_commit':'1134a78f','state':s,'binary_sha256':{binary:sha(Path(binary).read_bytes()) for binary in [cks,ckv,ckg,probe]},'payload_hashes_before':before,'payload_hashes_after':after,'fixture_sha256':sha((root/'system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json').read_bytes()),'diagnostic_only':True,'quality_metrics':None},indent=2)+'\n')
report=json.loads((p/'diagnostic-report.json').read_bytes());print(json.dumps(report['f01'],indent=2));print('invariant violations:',report['invariant_violations'])
