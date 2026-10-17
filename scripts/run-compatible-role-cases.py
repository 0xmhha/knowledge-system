#!/usr/bin/env python3
"""Run one bounded actual-role phase against the isolated compatible candidates."""
import importlib.util,json,shlex,sys
from pathlib import Path
root=Path(__file__).resolve().parents[1];s=importlib.util.spec_from_file_location('role',root/'scripts/consumer-compatible-host.py');m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
owned=Path('/private/tmp/ks-consumer-role-compatible-20261017');workspace=owned/'source/.coding-agent/tickets/KS-P06-REVIEW';plugin=m.CANDIDATE/'plugin';r=json.loads((m.P05/'execution.json').read_text());q=dict(r['query']);q['expected_snapshot_sha256']=next(x['response']['result']['structuredContent']['snapshot_sha256']for x in json.loads((m.P05/'sessions/main/messages.json').read_text())if x['label']=='baseline')
def bash(parts):return {'name':'Bash','input':{'command':shlex.join([str(p)for p in parts]),'description':'Owned P06 scripted transport validation'}}
def tool(t):return {'name':'mcp__plugin_coding-agent_cks__cks_context_'+t,'input':{'rule_pack':'go-local-review@1.0.0','function':q}if t=='rule_candidates'else{'symbol':r['selected']['canonical_id']}}
read={'name':'Read','input':{'file_path':str(workspace/'state.json')}}
verify=bash([sys.executable,plugin/'scripts/deep_review.py','verify','--workspace',workspace])
phase=sys.argv[1]
if phase=='exposure':
 case='P06-analyzer-03';role='analyzer';a=[{'name':'mcp__plugin_coding-agent_cks__cks_context_value_dependencies','input':q},tool('rule_candidates'),{'name':'mcp__plugin_coding-agent_cks__cks_context_value_dependency_expansion','input':{'function':q,'limits':{'max_functions':8,'max_hops':3}}},{'name':'mcp__plugin_coding-agent_cks__cks_context_policy_security','input':q},tool('review_function')]
 rows={x['label']:x for x in json.loads((m.P05/'sessions/discovery/messages.json').read_text())}
 empty=rows['find-Empty']['response']['result']['structuredContent']['nodes'][0]['id'];branch=rows['find-Branch']['response']['result']['structuredContent']['nodes'][0]['id']
 a += [{'name':'mcp__plugin_coding-agent_cks__cks_context_policy_security','input':{'node_id':empty}},{'name':'mcp__plugin_coding-agent_cks__cks_context_value_dependencies','input':{'node_id':branch,'limits':{'max_iterations':1}}},{'name':'mcp__plugin_coding-agent_cks__cks_context_value_dependency_expansion','input':{'function':q,'limits':{'max_functions':1}}},{'name':'mcp__plugin_coding-agent_cks__cks_context_policy_security','input':{'node_id':'missing-owned-symbol'}}]
elif phase=='handoff':
 case='P06-analyzer-handoff';a=[tool('rule_candidates'),tool('review_function'),bash([sys.executable,root/'scripts/consumer-role-adapter.py','build','--plugin',plugin,'--frames',owned/case/'mcp-frames.jsonl','--workspace',workspace]),read,{'state_operation':'handoff'}];role='analyzer'
elif phase=='delivery':
 case='P06-planner-delivery';a=[tool('review_function'),tool('rule_candidates'),read,verify,{'state_operation':'bad_report_hash'},{'state_operation':'bad_type'},{'state_operation':'bad_analysis_hash'},{'state_operation':'complete'},bash(['cp',workspace/'state.json',workspace/'generation1-state.json']),{'state_operation':'reenter'}];role='planner'
elif phase=='revision':
 case='P06-analyzer-revision';a=[bash([sys.executable,root/'scripts/consumer-role-adapter.py','revise','--plugin',plugin,'--workspace',workspace]),read,{'state_operation':'handoff'}];role='analyzer'
elif phase=='reentry':
 case='P06-planner-reentry';a=[read,verify,{'state_operation':'stale_completion'},{'state_operation':'complete'}];role='planner'
elif phase=='boundary':
 case='P06-planner-boundary';a=[read,{'name':'Edit','input':{'file_path':str(workspace/'state.json'),'old_string':'"ticket_type": "code_review"','new_string':'"ticket_type": "feature"'}},bash([sys.executable,root/'scripts/consumer-role-adapter.py','tamper','--plugin',plugin,'--workspace',workspace]),{'state_operation':'complete'},bash([sys.executable,root/'scripts/consumer-role-adapter.py','restore','--plugin',plugin,'--workspace',workspace]),{'state_operation':'complete'}];role='planner'
elif phase=='denied':
 case='P06-planner-denied';a=[tool('rule_candidates')];role='planner'
 out=m.run(owned,case,role,a,deny=[a[0]['name']]);print(json.dumps({'case':case,'out':str(out),'exit':json.loads((out/'exit.json').read_text())['host_exit']}));sys.exit()
elif phase=='unavailable':
 case='P06-analyzer-unavailable';a=[tool('rule_candidates')];role='analyzer'
 cfg=owned/'cks-unavailable.yaml';cfg.write_text((owned/'cks.yaml').read_text().replace('path: '+str(owned/'graph/graph.db'),'path: ""'))
 out=m.run(owned,case,role,a,config=cfg);print(json.dumps({'case':case,'out':str(out),'exit':json.loads((out/'exit.json').read_text())['host_exit']}));sys.exit()
else:raise ValueError(phase)
out=m.run(owned,case,role,a);print(json.dumps({'case':case,'out':str(out),'exit':json.loads((out/'exit.json').read_text())['host_exit']}))
