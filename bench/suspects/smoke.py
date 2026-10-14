#!/usr/bin/env python3
"""Execute real indexed Go graph -> MCP -> consumer factual artifact."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import select


def main():
    p=argparse.ArgumentParser();p.add_argument('--ckg',required=True);p.add_argument('--server',required=True);p.add_argument('--consumer',required=True);p.add_argument('--out',required=True);a=p.parse_args()
    out=Path(a.out).resolve();out.mkdir(parents=True,exist_ok=False)
    source=out/'subject';source.mkdir()
    (source/'go.mod').write_text('module example.org/suspect-fixture\n\ngo 1.26.9\n')
    (source/'path.go').write_text('package fixture\n\nfunc Leaf() int { return 1 }\nfunc Middle() int { return Leaf() }\nfunc Top() int { return Middle() }\n')
    def run(argv,cwd=None):
        r=subprocess.run(argv,cwd=cwd,text=True,capture_output=True,timeout=120)
        if r.returncode:raise RuntimeError(r.stderr+r.stdout)
        return r.stdout
    run(['git','init','-q'],source);run(['git','config','commit.gpgsign','false'],source)
    run(['git','add','go.mod','path.go'],source)
    run(['git','-c','user.name=Owned fixture','-c','user.email=fixture@example.invalid','commit','-qm','test: add call path fixture'],source)
    head=run(['git','rev-parse','HEAD'],source).strip()
    (out/'graph-build.stdout').write_text(run([a.ckg,'build','--src',str(source),'--out',str(out/'graph')]))
    env=dict(os.environ,KS_SUSPECT_FIXTURE_GRAPH=str(out/'graph'/'graph.db'))
    stderr=(out/'server.stderr').open('w')
    proc=subprocess.Popen([a.server,'-test.run=TestServeSuspectFixture'],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=stderr,text=True,bufsize=1)
    wire=[]
    def request(id,method,params):
        msg={'jsonrpc':'2.0','id':id,'method':method,'params':params};proc.stdin.write(json.dumps(msg)+'\n');proc.stdin.flush()
        while True:
            if not select.select([proc.stdout],[],[],30)[0]:raise RuntimeError('MCP response timeout')
            line=proc.stdout.readline()
            if not line:raise RuntimeError('server exited before response')
            try:response=json.loads(line)
            except json.JSONDecodeError:continue
            if response.get('id')==id:
                wire.append({'request':msg,'response':response})
                if 'error' in response:raise RuntimeError(response['error'])
                return response['result']
    try:
        request(1,'initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-fixture-consumer','version':'1'}})
        proc.stdin.write(json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n');proc.stdin.flush()
        tools=request(2,'tools/list',{})['tools'];(out/'tools.json').write_text(json.dumps(tools,indent=2)+'\n')
        run([sys.executable,a.consumer,'--check-contract',str(out/'tools.json')])
        response=request(3,'tools/call',{'name':'cks.context.label_suspects','arguments':{'symptom':'hang in Leaf call path','symbol':'Leaf','depth':2}})
        if response.get('isError'):raise RuntimeError(response)
        round=response['structuredContent'];(out/'round-1.json').write_text(json.dumps(round,indent=2)+'\n')
        distances={c['symbol'].split('.')[-1]:c['signals']['distance']['value'] for c in round['candidates']}
        if distances!={'Leaf':0,'Middle':1,'Top':2}:raise RuntimeError(f'incorrect real graph distances: {distances}')
        if round['snapshot']!=head:raise RuntimeError('wrong source revision')
        middle=next(c['citation'] for c in round['candidates'] if c['symbol'].endswith('.Middle'))
        second=request(4,'tools/call',{'name':'cks.context.label_suspects','arguments':{'symptom':'hang in Leaf call path','expand':[middle],'depth':1}})['structuredContent']
        (out/'round-2.json').write_text(json.dumps(second,indent=2)+'\n')
        run([sys.executable,a.consumer,'--round',str(out/'round-1.json'),'--state',str(out/'suspect-labels.json')])
        run([sys.executable,a.consumer,'--round',str(out/'round-2.json'),'--state',str(out/'suspect-labels.json'),'--stop','no_new_candidates'])
        run([sys.executable,a.consumer,'--check',str(out/'suspect-labels.json')])
        bad=request(5,'tools/call',{'name':'cks.context.label_suspects','arguments':{'symptom':'hang','symbol':'Missing'}})
        if not bad.get('isError'):raise RuntimeError('unresolved seed accepted')
        (out/'summary.json').write_text(json.dumps({'status':'PASS','source_commit':head,'distances':distances,'rounds':2,'schema_tools':len(tools),'scope':'owned real Go SQLite graph and actual stdio MCP; vector backend not exercised; no cause acceptance or production host claim'},indent=2)+'\n')
    finally:
        (out/'wire.json').write_text(json.dumps(wire,indent=2)+'\n')
        proc.stdin.close()
        try:proc.wait(timeout=10)
        except subprocess.TimeoutExpired:proc.terminate();proc.wait(timeout=10)
        stderr.close()
    print('real graph -> MCP -> factual consumer: PASS')

if __name__=='__main__':main()
