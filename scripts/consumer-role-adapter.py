#!/usr/bin/env python3
"""Owned scripted role test support, not agent reasoning or a second graph engine."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys


def dump(p,v):p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def load(path):return json.loads(path.read_text())
def require(ok,text):
    if not ok:raise ValueError(text)


def build(plugin,frames,workspace):
    require(str(workspace).startswith('/private/tmp/ks-consumer-role-'),'owned workspace required')
    workspace.mkdir(parents=True,exist_ok=True)
    rows=[json.loads(l)for l in frames.read_text().splitlines()];requests=[json.loads(r['raw'])for r in rows if r['kind']=='request'];responses={json.loads(r['raw']).get('id'):json.loads(r['raw'])for r in rows if r['kind']=='response'}
    receipt={}
    for name,key in [('rule_candidates','rules'),('review_function','function_review')]:
        calls=[q for q in requests if q.get('method')=='tools/call'and q['params']['name']=='cks.context.'+name];require(len(calls)==1,'one actual role call required: '+name);q=calls[0];r=responses[q['id']]['result'];require(not r.get('isError'),'tool error')
        result=r.get('structuredContent') or json.loads(r['content'][0]['text']);receipt[key]={'tool':q['params']['name'],'arguments':q['params']['arguments'],'result':result}
    bundle={'schema_version':1,'ticket_id':'KS-P06-REVIEW',**receipt};analysis=receipt['rules']['result']['analysis'];node=next(n['id']for n in analysis['nodes']if not n.get('synthetic'));sections=['data_state','guards_exceptions','complexity','resources','io','priority_tradeoffs']
    judgments={'sections':{s:{'status':'inferred' if s=='data_state' else 'unknown','claim':'The owned fixture has a retained local computation to inspect.' if s=='data_state' else 'This scripted adapter has not established '+s+'.','evidence_nodes':[node]if s=='data_state'else[],'assumptions':['The pinned owned fixture is the review target.']if s=='data_state'else[],'unknowns':['Scripted transport test: no independent semantic review.'],'origin':'agent_reasoning'}for s in sections},'findings':[],'criteria':[{'id':'owned-local-shape','requirement':'Preserve the source-bound local computation with explicit uncertainty.','section':'data_state','status':'covered','reason':'Scripted transport fixture only; not independent completeness or quality.'}]}
    dump(workspace/'input-bundle.json',bundle);dump(workspace/'scripted-judgments.json',judgments)
    state={'ticket_id':'KS-P06-REVIEW','ticket_type':'code_review','current_state':'ANALYSIS','config':{'autonomy':{'mode':'interactive'}},'states':{'ANALYSIS':{'status':'in_progress'},'PLANNING':{'status':'pending'}}};dump(workspace/'state.json',state)
    commands=[]
    def cli(*args):
        cmd=[sys.executable,str(plugin/'scripts/deep_review.py'),*args];p=subprocess.run(cmd,capture_output=True,text=True,timeout=20);commands.append({'command':cmd,'exit_code':p.returncode,'stdout':p.stdout,'stderr':p.stderr});require(p.returncode==0,'typed CLI failed');return p
    cli('build','--workspace',str(workspace),'--bundle',str(workspace/'input-bundle.json'),'--judgments',str(workspace/'scripted-judgments.json'))
    state['states']['ANALYSIS']['deep_review_sha256']=sha(workspace/'deep-review.json');dump(workspace/'state.json',state)
    cli('verify','--workspace',str(workspace));dump(workspace/'adapter-commands.json',commands)
    print(json.dumps({'status':'scripted_owned_analysis_prepared_and_verified','actual_role_reasoning_quality':'not_evaluated','state':state}))


def revise(plugin,workspace):
    state=load(workspace/'state.json');require(state['current_state']=='ANALYSIS','re-enter ANALYSIS before revisions')
    bundle=load(workspace/'deep-review-evidence.json');judgments=load(workspace/'scripted-judgments.json');judgments['sections']['data_state']['claim']+=' This is the second scripted review generation.'
    dump(workspace/'scripted-judgments.json',judgments)
    cmd=[sys.executable,str(plugin/'scripts/deep_review.py'),'build','--workspace',str(workspace),'--bundle',str(workspace/'deep-review-evidence.json'),'--judgments',str(workspace/'scripted-judgments.json')]
    p=subprocess.run(cmd,capture_output=True,text=True,timeout=20);require(p.returncode==0,'rebuild failure')
    state['states']['ANALYSIS']={'status':'in_progress','deep_review_sha256':sha(workspace/'deep-review.json')};state['states']['PLANNING']={'status':'pending'};dump(workspace/'state.json',state)
    p=subprocess.run([sys.executable,str(plugin/'scripts/deep_review.py'),'verify','--workspace',str(workspace)],capture_output=True,text=True,timeout=20);require(p.returncode==0,'revised verify failure')
    print(json.dumps({'status':'scripted_revision_verified','independent_acceptance':'not_recorded'}))


def shell_control(plugin,workspace,action):
    require(str(workspace).startswith('/private/tmp/ks-consumer-role-'),'owned workspace required')
    report=workspace/'review-report.md';backup=workspace/'owned-report-backup.txt'
    if action=='tamper':backup.write_bytes(report.read_bytes());report.write_bytes(report.read_bytes()+b'\nowned shell tamper control\n')
    else:report.write_bytes(backup.read_bytes())
    command=[sys.executable,str(plugin/'scripts/deep_review.py'),'verify','--workspace',str(workspace)]
    p=subprocess.run(command,capture_output=True,text=True,timeout=20)
    expected=2 if action=='tamper'else 0;require(p.returncode==expected,'unexpected shell control verifier result')
    receipt={'action':action,'command':command,'exit_code':p.returncode,'stdout':p.stdout,'stderr':p.stderr,'delivery_allowed':action=='restore'}
    dump(workspace/('shell-'+action+'-receipt.json'),receipt);print(json.dumps(receipt))


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('action',choices=['build','revise','tamper','restore']);p.add_argument('--plugin',type=Path,required=True);p.add_argument('--frames',type=Path);p.add_argument('--workspace',type=Path,required=True);a=p.parse_args()
    if a.action=='build':build(a.plugin,a.frames,a.workspace)
    elif a.action=='revise':revise(a.plugin,a.workspace)
    else:shell_control(a.plugin,a.workspace,a.action)
