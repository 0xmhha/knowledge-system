from pathlib import Path
import sys,os,subprocess,json,io,hashlib
root=Path.cwd();sys.path.insert(0,str(root/'tools/setup'))
from ks_setup.options import Options
from ks_setup import pipeline,mcp_check
base=Path(Path('/private/tmp/ks-o03-base.txt').read_text().strip());src=base/'source';out=base/'dataset';old_current=os.readlink(out/'current');op=[base/'dataset-cks.yaml',base/'dataset-cks-stdio.yaml'];before={str(p):hashlib.sha256(p.read_bytes()).hexdigest()for p in op}
kw=dict(src=src,out=out,name='owned-semantic',project_id='demo',embedder='mock',model='mock',embed_dim=8,ollama_url='http://127.0.0.1:1',bin_dir=base/'bin',build=False,min_canonical=0.9,ontology_budget_ms=5000)
env=dict(os.environ,GIT_AUTHOR_NAME='owned-fixture',GIT_AUTHOR_EMAIL='fixture@local.invalid',GIT_COMMITTER_NAME='owned-fixture',GIT_COMMITTER_EMAIL='fixture@local.invalid')
def commit():
 for argv in [['git','add','.'],['git','-c','commit.gpgsign=false','commit','-qm','owned negative source change']]:subprocess.run(argv,cwd=src,env=env,check=True)
def preserve():
 assert os.readlink(out/'current')==old_current
 assert all(hashlib.sha256(Path(p).read_bytes()).hexdigest()==v for p,v in before.items())
# New source tuple cannot borrow the existing projection.
(src/'README.md').write_text('# Fixture\nNew snapshot, no accepted meaning projection.\n');commit()
log=io.StringIO();opts=Options(**kw,semantic_store=base/'dataset-semantic.sqlite',ontology_mode='combined');p=pipeline.Pipeline(opts,pipeline.new_run(opts),log);assert not p.execute();(base/'stale-store.log').write_text(log.getvalue());assert 'semantic data is not readable' in p.run.error,p.run.error;preserve();stale=p.run.error
# Captured YAML with another project must be refused before publication.
file=src/'ontology.yaml';file.write_text(file.read_text().replace('project_id: demo','project_id: other'));commit()
log=io.StringIO();opts=Options(**kw,ontology='ontology.yaml',spec='spec.yaml');p=pipeline.Pipeline(opts,pipeline.new_run(opts),log);assert not p.execute();(base/'foreign-project.log').write_text(log.getvalue());assert 'semantic preparation failed' in p.run.error,p.run.error;preserve()
(base/'negative-integration.json').write_text(json.dumps({'stale_existing_projection_refused':stale,'foreign_project_input_refused':p.run.error,'current_unchanged':True,'operator_config_SHA_unchanged':before,'actual_human_review_count':0,'operating_project_modified':False},indent=2)+'\n');print('actual negative integration: PASS; stale projection and foreign project refused, current/operator configs unchanged')
