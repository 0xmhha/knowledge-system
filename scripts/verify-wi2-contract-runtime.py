#!/usr/bin/env python3
"""Replay exact exposed WI2 contract boundaries on the latest verified binary."""
import argparse,hashlib,importlib.util,json,queue,shutil,time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
BASE=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups'
IDS=['RULE-1-05','RULE-1-07','RULE-1-15','RULE-2-06','RULE-2-07','RULE-2-16']
def sha(raw):return hashlib.sha256(raw).hexdigest()


def run(out,binary,prepared):
    out.mkdir(parents=True);old=BASE/'wi2-consumer-matching-v1/runtime'
    def write(name,value):
        p=out/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def inventory(root):return {p.relative_to(root).as_posix():sha(p.read_bytes()) for p in root.rglob('*') if p.is_file() and not p.name.endswith(('lock','lease')) and '.git'not in p.parts}
    dataset=prepared/'stablenet/retained-origins-v2';controlled=Path('/private/tmp/ks-wi2-matching-20261018-run4/controlled-graph');before=inventory(dataset);cbefore=inventory(controlled);write('retained-before.json',before);write('controlled-before.json',cbefore)
    config=out/'controlled.yaml';raw=(old/'controlled-config.txt').read_text().replace('/private/tmp/ks-wi2-matching-20261018-run4/mock-vector',str(out/'mock-vector'));config.write_text(raw)
    sp=importlib.util.spec_from_file_location('transport',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(sp);sp.loader.exec_module(wire)
    class Client(wire.Client):
        def request(self,method,params=None):
            self.id+=1;request={'jsonrpc':'2.0','id':self.id,'method':method}
            if params is not None:request['params']=params
            line=json.dumps(request,separators=(',',':'))+'\n';self.stdin.write(line);self.stdin.flush();self.proc.stdin.write(line);self.proc.stdin.flush();deadline=time.monotonic()+(120 if method=='initialize' else 20)
            while time.monotonic()<deadline:
                try:line=self.queue.get(timeout=max(.01,deadline-time.monotonic()))
                except queue.Empty:raise TimeoutError('MCP '+method+' timed out')from None
                if line is None:raise RuntimeError('MCP stream ended')
                self.stdout.write(line);self.stdout.flush();value=json.loads(line)
                if value.get('id')==self.id:return value
            raise TimeoutError('MCP response timed out')
    sessions=[];responses=[]
    for label,cfg,name,ids in [('controlled',config,'controlled-results.json',IDS),('project',prepared/'go-stablenet.yaml','project-results.json',['PROJECT-016','PROJECT-045'])]:
        plans=[r for r in json.loads((old/name).read_text())if r['function_id']in ids];assert[r['function_id']for r in plans]==ids
        session=out/label;session.mkdir();client=Client(None,None,session,command=[str(binary),'mcp','--config',str(cfg)],namespace='cks.context.')
        try:
            client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'wi2-contract-audit','version':'1'}});write(label+'/tools.json',client.request('tools/list')['result']['tools'])
            for r in plans:
                start=time.monotonic();result=client.request('tools/call',{'name':'cks.context.rule_candidates','arguments':r['arguments']});responses.append(dict(function_id=r['function_id'],scope=label,arguments=r['arguments'],raw_response=result,elapsed_seconds=time.monotonic()-start));write('responses.json',responses);print(r['function_id'],'recorded',flush=True)
        finally:client.close();client.proc.stdout.close();sessions.append(dict(scope=label,exit=client.proc.returncode,requests=client.id));write('sessions.json',sessions)
    after=inventory(dataset);cafter=inventory(controlled);write('retained-after.json',after);write('controlled-after.json',cafter);assert before==after and cbefore==cafter
    write('receipt.json',dict(binary_sha256=sha(binary.read_bytes()),planned=8,attempted=len(responses),retries=0,original_inputs_unchanged=True,retained_preserved=True,controlled_graph_preserved=True,paid_or_external_LLM_calls=0,installed_agent_role_runs=0,embedding_inference_calls=None,formal_completed=0,current_scores=None))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--out',type=Path,required=True);p.add_argument('--binary',type=Path,required=True);p.add_argument('--prepared',type=Path,required=True);a=p.parse_args();run(a.out.resolve(),a.binary.resolve(),a.prepared.resolve())
