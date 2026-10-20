"""Replay measured long-function responses without inventing capacity or gold."""
import gzip,json,math
from collections import Counter
from pathlib import Path
import viewer_readiness as v
import wi6_pair_contract as c
ROOT=v.ROOT
PACKET=v.PACKET.parent/'scale-long-functions-v1'
TOOLS=['scripts/run-scale-long-functions.py','scripts/scale_long_functions.py','scripts/test_scale_long_functions.py','scripts/run-wi6-pair-pilot.py','scripts/wi6_pair_contract.py','scripts/viewer_readiness.py']
def nearest(values,q):return sorted(values)[max(0,math.ceil(len(values)*q)-1)]
def semantic_boundary(d):
    v.require(d['formal_completed']==d['independent_approved']==d['actual_LLM_calls']==d['paid_external_calls']==0 and d['capacity_SLA_accepted']is False,'local timing promoted to capacity/gold/LLM approval')
def derive(p=None):
    p=p or ROOT/PACKET;spec=v.read(p/'SPEC.json');pins=v.read(p/'RUNTIME-PINS.json');census=v.read(p/'CORPUS-CENSUS.json');q=p/'actual'
    scope=v.read(q/'SCOPE.json.gz');v.require(all(scope.get(k)==value for k,value in spec.items())and scope['driver_sha256']==v.sha((ROOT/'scripts/run-scale-long-functions.py').read_bytes()),'actual frozen scope/driver differs');v.require(scope['spec_sha256']==v.sha((p/'SPEC.json').read_bytes())==pins['spec_sha256']and scope['runtime_pins_sha256']==v.sha((p/'RUNTIME-PINS.json').read_bytes()),'scale input pin changed')
    receipt=v.read(q/'receipt.json.gz');semantic_boundary(receipt);rows=v.read(q/'calls.json.gz');session=v.read(q/'session/session.json.gz');cleanup=v.read(q/'cleanup.json.gz')
    v.require(receipt['value_queries']==21 and receipt['tool_calls']==23 and len(rows)==23 and session['requests']==25,'scale request population incomplete')
    v.require(session['exit_code']==0 and session['reaped']and session['reader_joined']and not session['term_sent']and not session['kill_sent']and session['remaining_live_group_members']==0,'scale CKS not cleanly reaped')
    v.require(cleanup['failure']is None and cleanup['original_owned_payload_preserved']and cleanup['model_manifest_unchanged']and not cleanup['proxy_thread_live']and not cleanup['sampler_thread_live']and cleanup['active_HTTP_handlers_at_close']==0,'scale resources/payload changed')
    maps=[v.read(q/(n+'.json.gz'))for n in ('original-before','original-after','owned-before','owned-after')]
    v.require(all(x==maps[0]for x in maps)and len(maps[0])==census['files']==2248 and census['graph_nodes']==spec['corpus_nodes']==244989,'scale corpus changed')
    replies=v.events(v.raw(q/'session/stdout.jsonl.gz'));requests=v.events(v.raw(q/'session/stdin.jsonl.gz'))
    v.require([r['id']for r in requests]==[r['id']for r in replies]==list(range(1,26)),'actual wire response IDs missing')
    actual={r['id']:r for r in replies}
    for r in rows:v.require(actual[r['request']['id']]==r['raw_response'],'derived row not on actual MCP wire')
    v.require(rows[0]['phase']=='health_before'and rows[-1]['phase']=='health_after'and all(c.unpack(r['raw_response'])[0]is not None for r in (rows[0],rows[-1])),'before/after health missing')
    outcomes=[];statuses=Counter();measured=[]
    for i,node in enumerate(spec['long_functions']):
        group=rows[1+i*7:1+(i+1)*7]
        v.require([r['phase']for r in group]==['warmup','small_budget']+['measured']*5,'per-function warmup/control/repetition missing')
        times=[];case_status=[]
        for r in group:
            limits=spec['ordinary_limits']if r['phase']!='small_budget'else{**spec['ordinary_limits'],**spec['small_budget_override']}
            args=dict(node_id=node['node_id'],expected_commit=spec['source_commit'],expected_source_sha256=node['function_sha256'],expected_snapshot_sha256=pins['graph_snapshot_sha256'],limits=limits)
            v.require(r['request']['params']==dict(name='cks.context.value_dependencies',arguments=args),'actual long-function arguments/pin differ')
            value,encoding=c.unpack(r['raw_response']);v.require(value is not None,'actual long-function tool/protocol error')
            status=value.get('status','unknown');statuses[status]+=1;case_status.append(status)
            source=(value.get('analysis')or{}).get('source')or{}
            if r['phase']!='small_budget':
                v.require(source.get('node_id')==node['node_id']and source.get('sha256')==node['function_sha256']and source.get('bytes')==node['bytes']and source.get('commit')==spec['source_commit']and source.get('snapshot_sha256')==pins['graph_snapshot_sha256'],'long-function retained source differs')
            else:v.require(status!='complete','tiny AST budget was falsely complete')
            if r['phase']=='measured':times.append(r['elapsed_ns']);measured.append(r['elapsed_ns'])
        outcomes.append(dict(node_id=node['node_id'],path=node['path'],bytes=node['bytes'],statuses=case_status,measured=5,p50_ns=nearest(times,.5),p95_ns=nearest(times,.95),post_budget_source_readability=True,semantic_quality_accepted=False))
    http=v.read(q/'HTTP.json.gz');emb=[r for r in http if r['path']in('/api/embed','/api/embeddings')];startup=v.read(q/'startup-HTTP.json.gz');first=[r for r in startup if r['path']=='/api/embed']
    v.require([r['input_count']for r in first[:2]]==[1,65]and [r['input_count']for r in first[2:]]in ([],[1])and 2<=len(emb)<=4 and all(r['input_count']==1 for r in emb[2:])and all(r['status']==200 and r['input_count']==r['output_count']and r['dimensions']==[1024]for r in emb),'actual local embedding denominator/dimensions differ')
    v.require(not any(r['path']in('/api/chat','/api/generate')for r in http),'unplanned generation request')
    v.require(any(pins['model_manifest_digest']in r.get('bge_digests',[])for r in http),'actual BGE identity not observed')
    samples=v.read(q/'RSS-samples.json.gz');v.require(samples and all(r['rss_KiB']>0 for r in samples),'resident memory samples missing')
    return dict(kind='local_retained_graph_long_function_profile_DEV_not_capacity_acceptance',corpus=census,source_nodes=3,warmup_queries=3,budget_controls=3,measured_queries=15,CKS_requests=25,outcomes=outcomes,status_counts=dict(statuses),measured_total_ns=sum(measured),sequential_measured_responses_per_second=15/(sum(measured)/1e9),maximum_sampled_RSS_KiB=max(r['rss_KiB']for r in samples),RSS_samples=len(samples),RSS_is_absolute_peak=False,initialize_HTTP_isolated=False,capture_ends_after_first_health=True,HTTP_through_first_health=len(first),additional_single_input_health_probes=len(emb)-2,local_embedding_HTTP_requests=len(emb),local_embedding_input_items=sum(r['input_count']for r in emb),owned_process_groups_remaining=0,model_registry_modified=False,resource_exclusive=False,fresh_independent_families=0,actual_LLM_calls=0,paid_external_calls=0,independent_approved=0,formal_completed=0,capacity_SLA_accepted=False,quality_score=None)
def check(p=None):
    p=p or ROOT/PACKET
    try:
        m=v.read(p/'manifest.json');v.require(m['artifacts']==v.inventory(p),'scale artifact population/bytes changed');v.require(m['tools']=={n:v.sha((ROOT/n).read_bytes())for n in TOOLS},'scale driver/helper changed');v.require(v.read(p/'SUMMARY.json')==derive(p),'scale metric/acceptance changed');return []
    except(ValueError,OSError,KeyError,TypeError,ZeroDivisionError)as e:return[str(e)]
if __name__=='__main__':
    errors=check()
    if errors:raise SystemExit('\n'.join(errors))
    print(json.dumps(derive(),ensure_ascii=False))
