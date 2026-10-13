from pathlib import Path
import json,collections,statistics,math,hashlib
run=Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/comparison-go-stablenet-20261013');out=run/'capture'
byarm={};p95=lambda v:sorted(v)[max(0,math.ceil(len(v)*.95)-1)]if v else None
for arm in ['baseline','candidate']:
 failures=collections.Counter();failedids=collections.Counter();warm=collections.defaultdict(list);warmincluding=collections.defaultdict(list);allcalls=[];telemetry=collections.Counter();pending=0;matched=0;elapsed=[]
 for block in [1,2]:
  d=out/str(block)/arm;cap=json.load(open(d/'capture.json'));expected={r['call'].get('measurement_id')for r in cap['rows']};expected.discard(None)
  elapsed.append(json.load(open(d/'command.json'))['elapsed_s'])
  for row in cap['rows']:
   call=row['call'];resp=call.get('response')or{};bad=bool(row.get('error')or call.get('transport_error')or resp.get('isError'))
   if bad:
    error=(resp.get('structuredContent')or{}).get('code')or row.get('error')or call.get('transport_error')or'unknown'
    failures[str(error)]+=1;failedids[row['request_id']]+=1
   if row['phase']=='warm_latency':
    ms=call['elapsed_ns']/1e6;warmincluding[row['request_id']].append(ms)
    if not bad:warm[row['request_id']].append(ms)
   allcalls.append(row)
  events=d/'events/cks-mcp.jsonl'
  for line in events.read_text().splitlines():
   e=json.loads(line)
   if e.get('event')!='measurement.backend_calls':continue
   s=e['summary']
   if s['measurement_id']not in expected:continue
   matched+=1;pending+=s.get('pending_calls',0)
   for c in s.get('calls',[]):
    if c.get('backend')=='ollama_http':telemetry[c.get('options',{}).get('path','unknown')]+=1
 byarm[arm]={'observed':len(allcalls),'failure_codes':dict(failures),'failed_by_question':dict(failedids),'successful_warm_by_question':{q:{'n':len(v),'p95_ms':p95(v),'median_ms':statistics.median(v)}for q,v in warm.items()},'all_warm_by_question':{q:{'n':len(v),'p95_ms':p95(v),'median_ms':statistics.median(v)}for q,v in warmincluding.items()},'matched_request_measurement_scopes':matched,'pending_calls':pending,'request_HTTP_attempts':dict(telemetry),'capture_elapsed_s':elapsed}
ratios={q:byarm['candidate']['successful_warm_by_question'][q]['p95_ms']/v['p95_ms']for q,v in byarm['baseline']['successful_warm_by_question'].items()if q in byarm['candidate']['successful_warm_by_question']}
summary={'arms':byarm,'successful_warm_p95_ratio_by_question':ratios,'latency_samples_per_query':10,'inference':'descriptive repeated-request timing only; not independent quality or accepted latency gate','failure_denominators_preserved':True,'scope_matching_excludes_startup_and_unmatched_probes':True}
(run/'extra-summary.json').write_text(json.dumps(summary,indent=2)+'\n');print('request failures', {a:s['failure_codes']for a,s in byarm.items()},'p95 ratio range',min(ratios.values()),max(ratios.values()))
