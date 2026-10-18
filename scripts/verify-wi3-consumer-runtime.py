#!/usr/bin/env python3
"""Read-only owned retained CKS symptom probes. Never feed gold into requests."""
import argparse
import gzip
import importlib.util
import json
from pathlib import Path
import shutil
import sys
import time
import urllib.request

R=Path(__file__).resolve().parents[1];sys.path.insert(0,str(R/'scripts'))
import wi3_dev_gold as g


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--workspace',type=Path,required=True);parser.add_argument('--prepared',type=Path,required=True);a=parser.parse_args()
    out=a.workspace.resolve();out.mkdir(parents=True);prepared=a.prepared.resolve();dataset=prepared/'stablenet/retained-origins-v2'
    def write(name,v):
        p=out/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    def inventory():return {str(p.relative_to(dataset)):g.f.sha(p.read_bytes()) for sub in ('graph','sources','vector') for p in (dataset/sub).rglob('*') if p.is_file() and not p.name.endswith(('lock','lease'))}
    def model():
        def get(path,data=None):
            req=urllib.request.Request('http://127.0.0.1:11434/api/'+path,data=json.dumps(data).encode() if data else None,headers={'Content-Type':'application/json'})
            with urllib.request.urlopen(req,timeout=10) as response:return json.load(response)
        models=get('tags')['models'];selected=next(m for m in models if m['name']=='bge-m3:latest')
        assert selected['digest']=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
        return dict(runtime=get('version'),model=selected,show=get('show',{'model':'bge-m3:latest'}))
    write('model-before.json',model());before=inventory();write('retained-before.json',before)
    for name in ('dataset-identity.json','sources/manifest.json','graph/manifest.json'):
        raw=(dataset/name).read_bytes();p=out/('bindings/'+name+'.gz');p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(raw,mtime=0))
    shutil.copyfile(prepared/'go-stablenet.yaml',out/'config.txt')
    binary=prepared/'read-cks';assert g.f.sha(binary.read_bytes())=='0f96f0d527732ad414416217506dc564a5b0014d8b8f81e335f8c1c88896e54c'
    spec=importlib.util.spec_from_file_location('transport',R/'scripts/eval-value-dependencies.py');w=importlib.util.module_from_spec(spec);spec.loader.exec_module(w)
    session=out/'primary';session.mkdir();client=w.Client(None,None,session,command=[str(binary),'mcp','--config',str(prepared/'go-stablenet.yaml')],namespace='cks.context.')
    client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'wi3-symptom-probe','version':'1'}})
    write('tools.json',client.request('tools/list')['result']['tools'])
    requests=g.f.read(R/g.PACKET/'public/wire-requests.json')['requests'];rows=[];boundaries=[]
    def call(request):
        started=time.monotonic();raw=client.request('tools/call',{'name':request['name'],'arguments':request['arguments']})
        return dict(case_id=request['case_id'],arguments=request['arguments'],raw_response=raw,wall_seconds=time.monotonic()-started)
    try:
        for request in requests:
            row=call(request);rows.append(row);write('primary-results.json',rows)
            value=row['raw_response'].get('result',{});print(request['case_id'],'tool_error' if value.get('isError') else 'structured_response',flush=True)
        for label,args in [('wrong_commit',dict(requests[0]['arguments'],commit='0'*40)),('invalid_candidate_cap',dict(requests[0]['arguments'],max_candidates=21))]:
            row=call(dict(case_id=label,name=requests[0]['name'],arguments=args));boundaries.append(row)
        # Expansion belongs to another retained project/dataset. It must fail before composition.
        known=g.f.read(R/g.previous.PACKET/'runtime/controlled-results.json') if False else None
        refs=[]
        for row in rows:
            result=row['raw_response'].get('result',{}).get('structuredContent')
            if result:
                refs.extend(c['retained_citation'] for c in result.get('candidates',[]) if c.get('retained_citation'))
        if refs:
            c=dict(refs[0],project_id='other-project',dataset_id='other-dataset')
            args=dict(requests[0]['arguments'],expand=[c]);boundaries.append(call(dict(case_id='foreign_expansion',name=requests[0]['name'],arguments=args)))
    finally:
        client.close();client.proc.stdout.close();write('session.json',dict(exit=client.proc.returncode,requests=client.id))
    write('boundaries.json',boundaries);after=inventory();write('retained-after.json',after);assert before==after
    write('model-after.json',model())
    write('receipt.json',dict(planned=20,attempted=len(rows),unreached=20-len(rows),primary_retries=0,
        eof_exit=client.proc.returncode,retained_preserved=True,binary_sha256=g.f.sha(binary.read_bytes()),
        local_embedding_provider='Ollama BGE-M3',paid_or_external_model_calls=0,installed_agent_role_runs=0,
        independent_approved=0,current_scores=None,formal_completed=0,freeze=None))


if __name__=='__main__':main()
