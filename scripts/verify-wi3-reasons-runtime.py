#!/usr/bin/env python3
"""Actual retained MCP regression: unchanged20 symptoms and explicit error controls."""
import argparse,gzip,hashlib,importlib.util,json,queue,shutil,sys,time,urllib.request
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
BASE=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups'
DIGEST='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'
def sha(raw):return hashlib.sha256(raw).hexdigest()


def run(binary,prepared,out):
    out.mkdir(parents=True);dataset=prepared/'stablenet/retained-origins-v2'
    def write(name,value):
        p=out/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def inventory():return {p.relative_to(dataset).as_posix():sha(p.read_bytes()) for sub in ('graph','sources','vector') for p in (dataset/sub).rglob('*') if p.is_file() and not p.name.endswith(('lock','lease'))}
    def model():
        def get(path,data=None):
            req=urllib.request.Request('http://127.0.0.1:11434/api/'+path,data=json.dumps(data).encode() if data else None,headers={'Content-Type':'application/json'})
            with urllib.request.urlopen(req,timeout=10) as r:return json.load(r)
        selected=next(x for x in get('tags')['models'] if x['name']=='bge-m3:latest');assert selected['digest']==DIGEST
        return dict(runtime=get('version'),model=selected,show=get('show',{'model':'bge-m3:latest'}))
    write('model-before.json',model());before=inventory();write('retained-before.json',before);shutil.copyfile(prepared/'go-stablenet.yaml',out/'config.txt')
    for name in ('dataset-identity.json','sources/manifest.json','graph/manifest.json'):
        p=out/'bindings'/(name+'.gz');p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress((dataset/name).read_bytes(),mtime=0))
    sp=importlib.util.spec_from_file_location('transport',ROOT/'scripts/eval-value-dependencies.py');m=importlib.util.module_from_spec(sp);sp.loader.exec_module(m)
    class Client(m.Client):
        def request(self,method,params=None):
            self.id+=1;message={'jsonrpc':'2.0','id':self.id,'method':method}
            if params is not None:message['params']=params
            line=json.dumps(message,separators=(',',':'))+'\n';self.stdin.write(line);self.stdin.flush();self.proc.stdin.write(line);self.proc.stdin.flush()
            deadline=time.monotonic()+(120 if method=='initialize' else 20)
            while time.monotonic()<deadline:
                try:value=self.queue.get(timeout=max(.01,deadline-time.monotonic()))
                except queue.Empty:raise TimeoutError('MCP '+method+' response deadline exceeded') from None
                if value is None:raise RuntimeError('MCP stream ended')
                self.stdout.write(value);self.stdout.flush();reply=json.loads(value)
                if reply.get('id')==self.id:return reply
            raise TimeoutError('MCP '+method+' response deadline exceeded')
    session=out/'session';session.mkdir();c=Client(None,None,session,command=[str(binary),'mcp','--config',str(prepared/'go-stablenet.yaml')],namespace='cks.context.')
    primary=[];controls=[];startup_started=time.monotonic();startup_elapsed=None;requests=json.loads((BASE/'wi3-dev-gold-v1/public/wire-requests.json').read_text())['requests']
    old=json.loads((BASE/'wi3-consumer-matching-v1/runtime/primary-results.json').read_text());cand=old[0]['raw_response']['result']['structuredContent']['candidates'];missing=next(x['retained_citation'] for x in cand if x['symbol']is None);good=next(x['retained_citation'] for x in cand if x['symbol'])
    control_args=dict(requests[0]['arguments']);control_args['symptom']='controlled source-qualified seed identity regression'
    plan=[dict(case_id=name,arguments=dict(control_args,expand=refs)) for name,refs in [('missing_only',[missing]),('missing_then_known',[missing,good]),('known_then_missing',[good,missing]),('known_only',[good])]]
    plan += [dict(case_id='foreign_project',arguments=dict(control_args,expand=[dict(good,project_id='other-project')])),dict(case_id='invalid_candidate_cap',arguments=dict(control_args,expand=[good],max_candidates=21)),dict(case_id='wrong_commit',arguments=dict(control_args,expand=[good],commit='0'*40))]
    write('planned-controls.json',plan)
    def call(row):
        name=row.get('name','cks.context.label_suspects');start=time.monotonic();raw=c.request('tools/call',{'name':name,'arguments':row['arguments']});return dict(case_id=row['case_id'],arguments=row['arguments'],raw_response=raw,wall_seconds=time.monotonic()-start)
    try:
        c.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'wi3-reason-regression','version':'1'}});write('tools.json',c.request('tools/list')['result']['tools']);startup_elapsed=time.monotonic()-startup_started
        for row in requests:
            primary.append(call(row));write('primary-results.json',primary);print(row['case_id'],'recorded',flush=True)
        for row in plan:controls.append(call(row));write('controls.json',controls);print(row['case_id'],'recorded',flush=True)
    finally:
        c.close();c.proc.stdout.close();write('session.json',dict(exit=c.proc.returncode,requests=c.id,startup_elapsed_seconds=startup_elapsed,startup_timeout_seconds=120,tool_transport_timeout_seconds=20))
    after=inventory();write('retained-after.json',after);assert before==after;write('model-after.json',model());write('receipt.json',dict(planned_primary=20,attempted_primary=len(primary),planned_controls=7,attempted_controls=len(controls),primary_retries=0,startup_timeout_seconds=120,unreached_primary=20-len(primary),EOF_exit=c.proc.returncode,binary_sha256=sha(binary.read_bytes()),retained_preserved=True,
        local_embedding_provider='Ollama BGE-M3',paid_or_external_LLM_calls=0,installed_agent_role_runs=0,embedding_inference_call_count=None,scope='20 original exposed symptom regressions plus7 explicit controlled requests; startup embeddings not physically counted',formal_completed=0,independent_approved=0,current_scores=None))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--binary',type=Path,required=True);p.add_argument('--prepared',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();run(a.binary.resolve(),a.prepared.resolve(),a.out.resolve())
