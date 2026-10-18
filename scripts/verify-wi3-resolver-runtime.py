#!/usr/bin/env python3
"""Owned old/new CKS stdio with measurement off/on and unchanged retained input."""
import argparse,copy,hashlib,importlib.util,json,queue,shutil,time,urllib.request
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
BASE=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups'
DIGEST='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'


def sha(raw):return hashlib.sha256(raw).hexdigest()


def run(binary,prepared,out):
    out.mkdir(parents=True)
    def write(name,value):
        p=out/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    data=prepared/'stablenet/retained-origins-v2'
    def inventory():return {x.relative_to(data).as_posix():sha(x.read_bytes())for sub in ('sources','graph','vector')for x in(data/sub).rglob('*')if x.is_file()and not x.name.endswith(('lock','lease'))}
    def model():
        with urllib.request.urlopen('http://127.0.0.1:11434/api/tags',timeout=10)as r:tags=json.load(r)
        selected=next(x for x in tags['models']if x['name']=='bge-m3:latest');assert selected['digest']==DIGEST
        with urllib.request.urlopen('http://127.0.0.1:11434/api/version',timeout=10)as r:version=json.load(r)
        return dict(model=selected,runtime=version)
    before=inventory();write('before.json',before);write('model-before.json',model())
    old=json.loads((BASE/'wi3-consumer-matching-v1/runtime/primary-results.json').read_text())
    good=next(c for c in old[0]['raw_response']['result']['structuredContent']['candidates']if c['symbol'])
    limits=dict(symptom='owned resolver measurement parity',max_candidates=4,max_seeds=1,max_enriched=1,max_calls=32,depth=1,budget_ms=5000)
    plans=[dict(case_id='site',arguments=dict(limits,file=good['citation']['file'],line=258)),
        dict(case_id='site_comment',arguments=dict(limits,file=good['citation']['file'],line=good['citation']['start_line'])),
        dict(case_id='symbol',arguments=dict(limits,symbol=good['symbol'])),
        dict(case_id='expand',arguments=dict(limits,expand=[good['retained_citation']])),
        dict(case_id='wrong_commit',arguments=dict(limits,expand=[good['retained_citation']],commit='0'*40)),
        dict(case_id='foreign_dataset',arguments=dict(limits,expand=[dict(good['retained_citation'],dataset_id='foreign')])),
        dict(case_id='forged_hash',arguments=dict(limits,expand=[dict(good['retained_citation'],content_sha256='0'*64)])),
        dict(case_id='unknown_site',arguments=dict(limits,file='__owned_absent_source__.go',line=1))]
    write('planned.json',plans)
    sp=importlib.util.spec_from_file_location('wire',ROOT/'scripts/eval-value-dependencies.py');m=importlib.util.module_from_spec(sp);sp.loader.exec_module(m)
    class Client(m.Client):
        def request(self,method,params=None):
            self.id+=1;msg=dict(jsonrpc='2.0',id=self.id,method=method)
            if params is not None:msg['params']=params
            line=json.dumps(msg,separators=(',',':'))+'\n';self.stdin.write(line);self.stdin.flush();self.proc.stdin.write(line);self.proc.stdin.flush()
            end=time.monotonic()+(120 if method=='initialize'else 20)
            while time.monotonic()<end:
                try:raw=self.queue.get(timeout=max(.01,end-time.monotonic()))
                except queue.Empty:raise TimeoutError('MCP '+method+' deadline')from None
                if raw is None:raise RuntimeError('MCP stream ended')
                self.stdout.write(raw);self.stdout.flush();value=json.loads(raw)
                if value.get('id')==self.id:return value
            raise TimeoutError('MCP '+method+' deadline')
    sessions=[];results=[]
    for enabled in (False,True):
        label='on'if enabled else'off';p=out/label;p.mkdir()
        # Identical data/model/sanitizer and logging level; only opt-in and owned log root differ.
        config=(prepared/'go-stablenet.yaml').read_text();config=config.replace('logging:\n  level: error\n',f'logging:\n  level: info\n  mode: prod\n  footprint_dir: {p / "footprints"}\n  measure_backend_calls: {str(enabled).lower()}\n')
        assert 'measure_backend_calls:'in config;(p/'config.yaml').write_text(config)
        c=Client(None,None,p,command=[str(binary),'mcp','--config',str(p/'config.yaml')],namespace='cks.context.')
        start=time.monotonic();startup=None;attempted=0
        try:
            init=c.request('initialize',dict(protocolVersion='2024-11-05',capabilities={},clientInfo=dict(name='owned-resolver-regression',version='1')))
            assert 'error'not in init;startup=time.monotonic()-start
            write(label+'/tools.json',c.request('tools/list')['result']['tools'])
            for plan in plans:
                attempted+=1;raw=c.request('tools/call',dict(name='cks.context.label_suspects',arguments=plan['arguments']))
                results.append(dict(measurement=label,case_id=plan['case_id'],arguments=plan['arguments'],raw_response=raw));write('responses.json',results)
                print(label,plan['case_id'],'recorded',flush=True)
        finally:
            c.close();c.proc.stdout.close();sessions.append(dict(measurement=label,attempted=attempted,requests=c.id,EOF_exit=c.proc.returncode,startup_seconds=startup,initialize_timeout_seconds=120,tool_transport_timeout_seconds=20));write('sessions.json',sessions)
    after=inventory();write('after.json',after);assert before==after;write('model-after.json',model())
    write('receipt.json',dict(binary_sha256=sha(binary.read_bytes()),planned=16,attempted=len(results),retries=0,retained_preserved=True,
        auto_label_measurement_scope_added=False,embedding_inference_call_count=None,installed_agent_role_runs=0,paid_or_external_LLM_calls=0,new_independent_samples=0,quality_score=None,formal_completed=0))


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--binary',type=Path,required=True);p.add_argument('--prepared',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();run(a.binary.resolve(),a.prepared.resolve(),a.out.resolve())
