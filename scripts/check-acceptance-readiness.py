"""Validate refreshed preparation evidence without promoting formal acceptance."""
import hashlib
import importlib.util
import json
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/operational-preparation-2026-10-16/readiness-refresh'
IDS={'WI-V1','WI-V2','WI-3','WI-4','WI-1','WI-2','WI-C','WI-6','WI-SP'}
def sha(data):return hashlib.sha256(data).hexdigest()
def check(folder=PACKET,root=ROOT):
 try:
  m,b=verify_packet(folder,root);require(m['kind']=='current_readiness_metadata_and_scripted_host_not_acceptance','wrong evidence kind')
  require(all(m[k]is False for k in('protocol_frozen','operational_acceptance','quality_acceptance','independent_acceptance','real_model_inference','real_chain_nodes_started')),'preparation promoted')
  v=load(folder,'validation.json');unchanged_checkpoint_go(root,v,'17970c86');require(all(code==0 for code in v['gates'].values()),'failed validation gate')
  corpus=load(folder,'runtime/corpus-source.json');require(corpus['candidate_commit']=='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'and corpus['head']==raw(folder,'runtime/corpus-head.stdout.txt').decode().strip()and corpus['candidate_tree']==raw(folder,'runtime/corpus-candidate_tree.stdout.txt').decode().strip()and corpus['candidate_scope']=='pinned_commit_available_only_no_new_execution_clone_or_gold','corpus candidate metadata differs')
  r=load(folder,'runtime/runtime.json');require(r['knowledge_source_commit'].startswith('3a530a25')and r['prepared_source_status']=='clean'and r['maintained_Go_files_equal_last_tested']==1275,'wrong prepared source')
  info=raw(folder,'runtime/buildinfo.stdout.txt').decode();require('vcs.revision='+r['knowledge_source_commit']in info and 'vcs.modified=false'in info and 'go1.26.9'in info,'fresh committed build provenance missing')
  require(raw(folder,'runtime/source-status.stdout.txt')==b''and all(c['exit_code']==0 for c in load(folder,'runtime/commands.json')),'source/command failure')
  require(load(folder,'runtime/before.json')==load(folder,'runtime/after.json'),'prepared indexes or config mutated')
  require({k:(x['commit'],x['verified_tracked_files'],x['tracked_status'])for k,x in r['repositories'].items()}=={'coding_agent':('b7e78689676769e8536626e76eed7165d870e1ba',367,''),'chainbench':('bdf5ad2758662310b23c508e61b13ee318e92b40',2045,'')},'adopted source identity drift')
  require(r['host']['version']=='2.1.301 (Claude Code)'and raw(folder,'runtime/claude-version.stdout.txt').decode().strip()==r['host']['version']and r['host']['agent_session_started']is False and r['host']['personal_authentication_inspected']is False,'host metadata scope differs')
  model=r['model'];require(model['digest']=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'and model['name']=='bge-m3:latest'and model['server_version']==load(folder,'runtime/ollama-version.json')['version']=='0.40.4'and model['embedding_dimensions']=='not_remeasured_metadata_only','model metadata promoted')
  require(any(t['name']==model['name']and t['digest']==model['digest']for t in load(folder,'runtime/ollama-tags.json')['models']),'model digest not in observed tags')
  require(r['tools_counts']=={'cks':33,'chainbench':81}and r['EOF_exit_codes']==[0,0]and r['MCP_sessions']==2 and r['MCP_tool_calls']==0 and r['metadata_requests']==4,'metadata denominators differ')
  require(all(r[k]in(0,False)for k in('actual_model_calls','paid_model_calls','real_chain_nodes_started','HTTP_consumer_transport_tested','full_agent_dispatch_tested','independent_acceptance','protocol_frozen','operational_execution_allowed')),'metadata promoted to operations')
  for key in('cks','chainbench'):
   requests=[json.loads(x)for x in raw(folder,'runtime/'+key+'-session/mcp.stdin.jsonl').splitlines()];responses=[json.loads(x)for x in raw(folder,'runtime/'+key+'-session/mcp.stdout.jsonl').splitlines()];require(len(requests)==len(responses)==2 and [x['method']for x in requests]==['initialize','tools/list'],'unexpected metadata execution');by_id={x['id']:x for x in responses};require(len(by_id)==2 and all(q['id']in by_id and 'error'not in by_id[q['id']]for q in requests),'missing metadata response');require(by_id[requests[1]['id']]['result']['tools']==load(folder,'runtime/'+key+'-tools.json'),'native tool listing differs')
  contract=raw(folder,'runtime/consumer-contract.json');require(sha(contract)==r['consumer_schema_sha256'],'consumer schema identity differs');consumer=json.loads(contract)['providers']['cks']['tools'];require(load(folder,'runtime/consumer-tools.json')==consumer,'consumer schema projection differs');producer={t['name']:t for t in load(folder,'runtime/cks-tools.json')};require(len(consumer)==33 and set(producer)=={x['wire_name']for x in consumer.values()}and all(producer[x['wire_name']]['inputSchema']==x['input']for x in consumer.values()),'actual consumer/producer schema drift')
  parity=load(folder,'runtime/schema-parity.json');require(parity['missing_aliases']==parity['input_schema_differences']==parity['extra_producer_tools']==[]and parity['end_to_end_agent_dispatch_verified']is False,'schema parity promoted')
  spec=importlib.util.spec_from_file_location('current_readiness_host_probe',root/'scripts/operational-host-probe.py');probe=importlib.util.module_from_spec(spec);spec.loader.exec_module(probe);cases=load(folder,'host/cases.json');require(len(cases)==5 and {c['assessment']['case']for c in cases}=={'valid','tools','incomplete','tampered','cancel'},'missing host cases')
  for c in cases:
   facts=probe.assess(folder/'host'/c['assessment']['case']);require(facts==c['assessment']and facts['status']=='PASS'and facts['actual_host_version']=='2.1.301','current host raw assessment differs')
  h=load(folder,'host/summary.json');require(h['cases_passed']==5 and h['MCP_calls']==1 and h['server_exit_codes']==[0]*5 and h['owned_MCP_processes_absent']is True and h['host_version']==r['host']['version']and all(h[k]in(0,False)for k in('paid_provider_calls','actual_upstream_model_inference','full_agent_role_pipeline_executed','real_chain_nodes_started','independent_acceptance')),'scripted host promoted')
  p=load(folder,'scoring-review-proposals.json');require(p['approved']is False and p['independent_gold_records']==0 and p['actual_agent_or_node_execution']is False and len(p['groups'])==9 and {g['id']for g in p['groups']}==IDS and all(g['status']=='proposed_not_reviewed'and all(g[k]for k in('baseline_proposal','independent_unit_proposal','canonical_matching_proposal','denominator_and_verdict_proposal'))for g in p['groups'])and all(p['common_scoring_proposal'].values()),'proposal/gold scope differs')
  review=load(folder,'freeze-review-checklist.json');require(review['approved_requirements']==0 and len(review['requirements'])==8 and {g['id']for g in review['requirements']}=={'F-'+str(i).zfill(2)for i in range(1,9)}and all(g['decision_status']=='pending'and g['receipt']is None for g in review['requirements']),'freeze decision invented')
  require(load(folder,'formal-state-before.json')==load(folder,'formal-state-after.json'),'formal decisions changed during preparation')
  require('documentation: OK (134 live documents)'in raw(folder,'docs-final.txt').decode(),'docs gate failed')
 except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,AssertionError)as e:return[str(e)]
 return []
if __name__=='__main__':
 errors=check()
 for error in errors:print('FAIL:',error)
 print('acceptance readiness:','FAIL'if errors else'PASS; current metadata and scripted host; formal acceptance unchanged')
 raise SystemExit(bool(errors))
