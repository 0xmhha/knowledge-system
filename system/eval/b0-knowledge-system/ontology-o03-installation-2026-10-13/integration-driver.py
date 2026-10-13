from pathlib import Path
import os,sys,json,subprocess,io,hashlib
root=Path.cwd();sys.path.insert(0,str(root/'tools/setup'))
from ks_setup.options import Options
from ks_setup import pipeline,mcp_check
base=Path('/private/tmp/ks-o03-base.txt').read_text().strip();base=Path(base);src=base/'source';src.mkdir();out=base/'dataset'
files={'go.mod':'module demo\n\ngo 1.25\n','main.go':'package demo\nfunc Alpha() int { return Beta() }\nfunc Beta() int { return 1 }\n','README.md':'# Fixture\nOwned semantic preparation fixture.\n','ontology.yaml':'''version: 1
project_id: demo
domain: owned fixture
competency_questions: ["Which feature?"]
concepts:
  - id: feature
    kind: entity
    definition: A fixture feature.
    includes: [fixture]
    excludes: [quality]
    terms: [{lang: en, value: feature, preferred: true}]
    status: proposed
''','spec.yaml':'''version: 1
project_id: demo
requirements:
  - id: req-feature
    version: 3
    title: Inspect feature
    statement: The source-backed fixture is inspectable.
    concept_ids: [feature]
    status: proposed
    acceptance_criteria:
      - id: ac-feature
        given: Captured source
        when: A semantic read is made
        then: Proposed status is retained
'''}
for name,body in files.items():(src/name).write_text(body)
env=dict(os.environ,GIT_AUTHOR_NAME='owned-fixture',GIT_AUTHOR_EMAIL='fixture@local.invalid',GIT_COMMITTER_NAME='owned-fixture',GIT_COMMITTER_EMAIL='fixture@local.invalid')
for argv in [['git','init','-q'],['git','add','.'],['git','-c','commit.gpgsign=false','commit','-qm','owned semantic fixture']]:subprocess.run(argv,cwd=src,env=env,check=True)
kw=dict(src=src,out=out,name='owned-semantic',project_id='demo',embedder='mock',model='mock',embed_dim=8,ollama_url='http://127.0.0.1:1',bin_dir=base/'bin',build=False,min_canonical=0.9)
log=io.StringIO();old=Options(**kw,verify=False);p=pipeline.Pipeline(old,pipeline.new_run(old),log);ok=p.execute();(base/'baseline.log').write_text(log.getvalue());assert ok,p.run.error
old_current=os.readlink(out/'current');old_configs={str(x):hashlib.sha256(x.read_bytes()).hexdigest()for x in [old.server_config,old.stdio_config]}
log=io.StringIO();opts=Options(**kw,ontology='ontology.yaml',spec='spec.yaml',ontology_budget_ms=5000);p=pipeline.Pipeline(opts,pipeline.new_run(opts),log);ok=p.execute();(base/'semantic.log').write_text(log.getvalue());assert ok,p.run.error
assert os.readlink(out/'current')==old_current
assert all(hashlib.sha256(Path(path).read_bytes()).hexdigest()==digest for path,digest in old_configs.items())
client=mcp_check.StdioClient(base/'bin'/'cks',opts.stdio_config)
try:
 client.initialize();names=client.list_tools()
 status=client.call('cks.semantic.status',{})
 lookup=client.call('cks.semantic.lookup_term',{'language':'en','term':'feature'})
 concept=client.call('cks.semantic.get_concept',{'concept_id':'feature'})
 trace=client.call('cks.semantic.trace_requirement',{'requirement_id':'req-feature'})
finally:client.close()
assert status['read_tools_ready']and not status['retrieval_ready']and status['mode']=='off'
assert lookup['result']['candidates'][0]['status']=='proposed'
assert concept['result']['concept']['status']=='proposed' and not concept['result']['concept'].get('reviewed_by')
assert trace['result']['trace']['state']=='spec_unapproved' and not trace['result']['acceptance_evaluated']
report={'base':str(base),'fixture_source_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=src,text=True).strip(),'previous_current':old_current,'candidate':str(opts.serving_dir),'current_unchanged':True,'existing_operator_configs_SHA_unchanged':old_configs,'new_candidate_config':str(opts.stdio_config),'status':status,'lookup':lookup,'concept':concept,'trace':trace,'tool_count':len(names),'semantic_candidate_only':True,'actual_human_review_count':0,'real_embedding_model_used':False,'model_endpoint':'closed owned-test loopback endpoint','model_queries_for_read_tools':0}
(base/'integration.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'integration':'PASS','tools':len(names),'read_tools_ready':status['read_tools_ready'],'mode':status['mode'],'retrieval_ready':status['retrieval_ready'],'current_preserved':True,'candidate_config':str(opts.stdio_config)}))
