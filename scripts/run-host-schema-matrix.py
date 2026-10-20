#!/usr/bin/env python3
"""Same owned retained inputs against old/new native stdio binaries, no proxy rewrite."""
import argparse
import importlib.util
import json
from pathlib import Path
import queue
import time
ROOT=Path(__file__).resolve().parents[1]
s=importlib.util.spec_from_file_location('stdio',ROOT/'scripts/prepare-consumer-stdio-readiness.py');base=importlib.util.module_from_spec(s);s.loader.exec_module(base)


class RawClient(base.OwnedClient):
    def __init__(self,command,path):
        self.binary_before=base.sha(Path(command[0]).read_bytes());super().__init__(command,path)

    def close(self):
        super().close();p=self.path/'lifecycle.json';record=json.loads(p.read_text())
        record.update(binary_sha256_before=self.binary_before,binary_sha256_after=base.sha(Path(self.command[0]).read_bytes()))
        base.dump(p,record)

    def call(self,label,name,raw_args):
        base.require(time.monotonic()-self.started<900,'native session wallclock budget')
        base.require(self.calls<64,'tool budget');self.calls+=1;self.id+=1
        raw='{"jsonrpc":"2.0","id":'+str(self.id)+',"method":"tools/call","params":{"name":'+json.dumps('cks.context.'+name)+',"arguments":'+raw_args+'}}\n'
        self.ins.write(raw);self.ins.flush();self.proc.stdin.write(raw);self.proc.stdin.flush();deadline=time.monotonic()+30
        while True:
            line=self.queue.get(timeout=max(.001,deadline-time.monotonic()));base.require(line is not None,'premature EOF');response=json.loads(line)
            if response.get('id')==self.id:break
        self.messages.append({'label':label,'request':json.loads(raw),'request_raw':raw,'response':response});return response


def cases():
    packet=ROOT/base.BASE/'consumer-stdio-readiness-v1';r=json.loads((packet/'execution.json').read_text());q=r['query'];selected=r['selected']
    rows=json.loads((packet/'sessions/discovery/messages.json').read_text());nodes={x['label']:x['response']['result'].get('structuredContent',{})for x in rows if x['label'].startswith('find-')}
    cases=[]
    common=['{}','{"node_id":"a","canonical_id":"b"}','{"node_id":null}','{"node_id":17}','{"node_id":""}','{"canonical_id":false}','{"node_id":"a","unknown":true}','{"node_id":"a","node_id":"b"}','{"node_id":"a","expected_commit":"main"}','{"node_id":"a","expected_source_sha256":"bad"}','{"node_id":"a","expected_snapshot_sha256":null}']
    for tool in ['value_dependencies','policy_security']:
        cases.append((tool+'-node',tool,json.dumps(q),'success'))
        cases.append((tool+'-canonical',tool,json.dumps({k:v for k,v in {**q,'canonical_id':selected['canonical_id']}.items()if k!='node_id'}),'success'))
        for i,raw in enumerate(common):cases.append((tool+'-invalid-'+str(i),tool,raw,'refused'))
        extra=[{'limits':{'max_nodes':0}},{'limits':{'max_nodes':1025}},{'limits':{'max_nodes':1.5}},{'limits':{'max_nodes':None}}]if tool=='value_dependencies'else[{'max_annotations':0},{'max_annotations':129},{'max_annotations':1.5},{'timeout_ms':5001}]
        for i,row in enumerate(extra):cases.append((tool+'-bounds-'+str(i),tool,json.dumps({**q,**row}),'refused'))
        for pin,size in [('expected_commit',40),('expected_source_sha256',64),('expected_snapshot_sha256',64)]:cases.append((tool+'-'+pin,tool,json.dumps({**q,pin:'0'*size}),'source_mismatch'))
    branch=nodes['find-Branch']['nodes'][0]['id'];empty=nodes['find-Empty']['nodes'][0]['id']
    cases.extend([('value-cap','value_dependencies',json.dumps({'node_id':branch,'limits':{'max_iterations':1}}),'budget_exhausted'),('policy-cap','policy_security',json.dumps({**q,'max_annotations':1}),'partial'),('policy-untagged','policy_security',json.dumps({'node_id':empty}),'no_annotations'),('missing-value','value_dependencies','{"node_id":"missing-owned-symbol"}','not_found'),('missing-policy','policy_security','{"node_id":"missing-owned-symbol"}','not_found'),('rules-both','rule_candidates','{"rule_pack":"go-local-review@1.0.0","function":{"node_id":"a","canonical_id":"b"}}','refused'),('expansion-both','value_dependency_expansion','{"function":{"node_id":"a","canonical_id":"b"}}','refused')])
    return cases


def normalize(v):
    if isinstance(v,dict):
        result={k:normalize(x)for k,x in v.items()if k!='elapsed_micros'}
        # MCP duplicates structured output as JSON text. Preserve every field
        # while omitting the same timing field in that encoded copy as well.
        if v.get('type')=='text'and isinstance(v.get('text'),str):
            try:result['text']=json.dumps(normalize(json.loads(v['text'])),sort_keys=True,separators=(',',':'))
            except json.JSONDecodeError:pass
        return result
    if isinstance(v,list):return [normalize(x)for x in v]
    return v


def run(owned,new_binary,old_binary):
    folder=owned/'native-matrix';base.require(not folder.exists(),'fresh matrix');folder.mkdir();all_results={}
    for label,binary in [('old',old_binary),('new',new_binary)]:
        client=RawClient([str(binary),'mcp','--config',str(owned/'cks.yaml')],folder/label);result={}
        try:
            client.initialize();client.request('tools','tools/list')
            for name,tool,raw,expected in cases():
                response=client.call(name,tool,raw)
                if expected=='refused':base.require(base.refused(response),'invalid input accepted: '+name)
                else:
                    value=base.payload(response)
                    if expected=='success':base.require(value.get('analysis') is not None or value.get('status')=='annotations_found','positive missing result')
                    else:base.require(value['status']==expected,'unexpected '+name)
                result[name]=normalize(response)
        finally:client.close()
        all_results[label]=result
    base.require(all_results['old']==all_results['new'],'normalized runtime behavior differs')
    report={'cases_per_binary':len(cases()),'tool_requests_total':2*len(cases()),'runtime_results_equal_after_omitting_elapsed_micros':True,'raw_duplicate_keys_preserved':True,'semantic_quality_evaluated':False}
    base.dump(folder/'comparison.json',report);print(json.dumps(report))


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--owned',type=Path,required=True);p.add_argument('--new-binary',type=Path,required=True);p.add_argument('--old-binary',type=Path,required=True);a=p.parse_args();run(a.owned.resolve(),a.new_binary.resolve(),a.old_binary.resolve())
