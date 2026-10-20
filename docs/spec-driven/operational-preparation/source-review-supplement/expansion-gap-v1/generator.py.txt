#!/usr/bin/env python3
"""Audit WI-6 independent families, paired arms, relevance/noise and warm latency gaps."""
import argparse
from collections import Counter
import importlib.util
import itertools
import json
from pathlib import Path

from source_packet import SourceCapture, safe, sha, verify

ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR=BASE/'expansion-v1'
HOST=Path('system/eval/operational-preparation-2026-10-15/isolated-host')
OLD=Path('system/eval/upper-work-items-implementation-2026-10-14/wi-6')
SOURCE=[Path(p) for p in ['pkg/graph/valueflow/expand.go','pkg/graph/valueflow/query.go',
 'pkg/graph/mcphandlers/valueflow.go','internal/system/mcp/valueflow.go',
 'internal/system/mcp/value_expansion.go','internal/system/ckgclient/valueflow.go',
 'internal/system/mcp/testdata/agent-mcp.schema.json']]
CONSUMER=['plugin/agents/analyzer.md','plugin/contracts/cks-tools.schema.json']
ATTRIBUTES=b'consumer-source/objects/*.commit binary\nconsumer-source/objects/*.tree binary\n'
MANIFESTS=[BASE/n/'manifest.json' for n in ['expansion-v1','lexical-coverage-v1','function-sample-v1']]
INPUTS={'public':PRIOR/'public-inputs.json','private':PRIOR/'private-ai-review.json',
 'sample':BASE/'function-sample-v1/sample-summary.json',
 'proposals':Path('scripts/fixtures/expansion-gap-proposals.json'),
 'host':HOST/'manifest.json','consumer_inventory':HOST/'coding_agent-source-files.json',
 'old_manifest':OLD/'manifest.json','old_summary':OLD/'execution/summary.json',
 'old_baseline':OLD/'execution/baseline.json','old_expanded':OLD/'execution/expanded.json',
 'old_requests':OLD/'execution/cks/mcp.stdin.jsonl','old_responses':OLD/'execution/cks/mcp.stdout.jsonl',
 'old_ckg_requests':OLD/'execution/ckg/mcp.stdin.jsonl','old_ckg_responses':OLD/'execution/ckg/mcp.stdout.jsonl',
 'old_failure':OLD/'historical-failures/cause.json'}
for name in ['functions','candidates','nodes','edges','facts']:
    INPUTS['old_cap_'+name]=OLD/('execution/cap-'+name+'.json')
OBLIGATIONS=['independent_intent_seed_family_exposure_split','production_sample_sufficiency',
 'complete_independent_relevance_gold','canonical_function_evidence_matching',
 'same_snapshot_local_limits_and_declared_aggregate_budgets','paired_actual_requests_and_arm_order',
 'partial_empty_unknown_and_failure_denominators','family_macro_recall_noise_mapping',
 'warm_definition_repetitions_clock_quantile_and_ratio','installed_consumer_role_and_handoff',
 'human_review_and_final_acceptance']


def load(name):
    spec=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


helpers=load('prepare-semantic-gap.py')
dump,unique,span=helpers.dump,helpers.unique,helpers.span


def bindings():
    return sorted(set(INPUTS.values())|set(MANIFESTS)|set(SOURCE)|{Path('scripts/prepare-semantic-gap.py'),Path('scripts/source_packet.py')})


def inputs(packet):
    d={k:([json.loads(s) for s in (ROOT/p).read_text().splitlines() if s.strip()] if p.suffix=='.jsonl'
          else json.loads((ROOT/p).read_text())) for k,p in INPUTS.items()}
    for p in MANIFESTS:
        for name,digest in json.loads((ROOT/p).read_text())['artifacts'].items():
            if sha(safe(ROOT/p.parent,name).read_bytes())!=digest:raise ValueError('prior artifact binding: '+name)
    for p in INPUTS.values():
        if p.is_relative_to(OLD) and p.name!='manifest.json':
            if d['old_manifest']['files_sha256'][p.relative_to(OLD).as_posix()]!=sha((ROOT/p).read_bytes()):
                raise ValueError('historical evidence binding')
    revision=json.loads((ROOT/PRIOR/'manifest.json').read_text())['repository_commit']
    proof=verify(ROOT/BASE/'lexical-coverage-v1',revision)
    d['project_source']={p:safe(ROOT/BASE/'lexical-coverage-v1',r['artifact']).read_bytes() for p,r in proof['files'].items()}
    revision=d['host']['source_adoptions']['coding_agent']['source_commit']
    proof=verify(packet/'consumer-source',revision)
    if set(proof['files'])!=set(CONSUMER):raise ValueError('consumer source inventory')
    d['consumer_source']={p:safe(packet/'consumer-source',r['artifact']).read_bytes() for p,r in proof['files'].items()}
    inventory={r['path']:r['sha256'] for r in d['consumer_inventory']}
    if any(sha(raw)!=inventory[p] for p,raw in d['consumer_source'].items()):raise ValueError('adopted source binding')
    d['current_source']={str(p):(ROOT/p).read_bytes() for p in SOURCE}
    protocol=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json').read_text())
    progress=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json').read_text())
    d['criteria']={'protocol':next(r for r in protocol['input_groups'] if r['id']=='WI-6'),
                   'progress':next(r for r in progress['items'] if r['id']=='WI-6')}
    return d


def rpc_rows(requests,responses):
    req,res=unique(requests,'id'),unique(responses,'id')
    if set(req)!=set(res):raise ValueError('historical request/response identity')
    rows=[]
    for key,r in req.items():
        result=res[key].get('result',{});value=result.get('structuredContent')
        if value is None:
            text=''.join(c.get('text','') for c in result.get('content',[]) if c.get('type')=='text')
            try:value=json.loads(text)
            except ValueError:value={}
        value=value if isinstance(value,dict) else {}
        body=value.get('graph',value);params=r.get('params',{})
        rows.append({'id':key,'method':r['method'],'tool':params.get('name'),
          'arguments':params.get('arguments'),'rpc_error':'error' in res[key],
          'tool_error':result.get('isError',False),'status':body.get('status'),
          'returned_function_wrappers':len(body.get('functions',[])) if 'graph' in value else None,
          'stop_reasons':body.get('stop_reasons')})
    return rows


def generate(d):
    public=unique(d['public']['cases'],'case_id');private=unique(d['private']['review_drafts'],'case_id')
    if len(public)!=20 or set(public)!=set(private):raise ValueError('twenty source input inventory')
    if d['private']['approved_gold_records']!=0 or d['private']['runtime_executions']!=0:raise ValueError('gold/execution promotion')
    cases=[]
    selected=set(d['sample']['selected_proposal'])
    for key,p in public.items():
        r=private[key];src=p['source'];raw=d['project_source'][src['path']]
        if sha(raw)!=src['file_sha256'] or sha(raw[src['start_byte']:src['end_byte_exclusive']])!=src['function_sha256']:
            raise ValueError('seed source binding')
        if p['approved'] is not False or p['split'] is not None or r['approved'] is not False or r['independence_verified'] is not False or r['relevance_labels'] is not None:
            raise ValueError('independent relevance/family promotion')
        for anchor in r['anchors']:
            if span(raw,anchor['start_line'],anchor['end_line'])!={k:v for k,v in anchor.items() if k!='expected_source_fact_draft'}:
                raise ValueError('existing project anchor')
        stress=p['candidate_id']=='SRC-026'
        if p['scope']!=('development_stress_source_only' if stress else 'production_source_candidate'):raise ValueError('stress scope')
        cases.append({'case_id':key,'candidate_id':p['candidate_id'],'intent_label':p['intent'],
          'source':src,'scope':p['scope'],'existing_ai_anchors':r['anchors'],
          'existing_unknowns':r['unknowns'],'intent_relevance_question':d['proposals']['intent_questions'][p['intent']],
          'same_seed_exposed_in_wi1_sample':p['candidate_id'] in selected,
          'family_independence':'not_reviewed','approved_independent_family':False,
          'operational_execution_allowed':False,'independent_relevance_gold':None,
          'seed_inclusion_policy':None,'negative_and_unknown_labels':None,'universe_or_completeness_review':None,
          'baseline_tool_proposal':'context.value_dependencies',
          'expansion_tool_proposal':'context.value_dependency_expansion',
          'resolved_retained_seed':None,'expected_snapshot_sha256':None,'baseline_request':None,'expansion_request':None,
          'local_limit_policy':None,'aggregate_limit_policy':None,'arm_order_policy':None,
          'baseline_returned_functions':None,'expanded_returned_functions':None,
          'recall_delta':None,'noise_fraction_delta':None,'warm_p95_latency_ratio':None,
          'paired_mcp_receipt':None,'warm_raw_samples':None,'reviewer':None,'review_receipt':None,'split':None})
    production=[r for r in cases if r['scope']=='production_source_candidate']
    if len(production)!=19 or len({r['intent_label'] for r in cases})!=4:raise ValueError('production/intent denominator')
    pairs=[{'a':a['case_id'],'b':b['case_id'],'same_file':a['source']['path']==b['source']['path'],
            'same_intent_label':a['intent_label']==b['intent_label'],'semantic_independence':'not_reviewed'}
           for a,b in itertools.combinations(cases,2)]
    contracts=[]
    for r in d['proposals']['contracts']:
        source=d['consumer_source'] if r['scope']=='adopted_consumer' else d['current_source']
        contracts.append({**r,'anchor':span(source[r['path']],*r['lines']),'approved':False})
    unique(contracts,'id')
    front=d['consumer_source']['plugin/agents/analyzer.md'].decode().split('---',2)[1]
    tools=[s.strip()[2:] for s in front.split('tools:\n',1)[1].split('\nskills:',1)[0].splitlines() if s.strip().startswith('- ')]
    consumer=json.loads(d['consumer_source']['plugin/contracts/cks-tools.schema.json'])['providers']['cks']['tools']
    metadata=json.loads(d['current_source']['internal/system/mcp/testdata/agent-mcp.schema.json'])['tools']
    exposure=[]
    for name in ['value_dependencies','value_dependency_expansion']:
        alias='mcp__plugin_coding-agent_cks__cks_context_'+name
        c=consumer['cks_context_'+name];m=metadata['cks.context.'+name]
        exposure.append({'tool':name,'wire_name':c['wire_name'],'metadata_input_parity':c['input']==m['input'],
          'explicit_analyzer_alias':alias,'explicit_analyzer_tool_present':alias in tools,
          'effective_host_permissions':None,'runtime_denial_proven':False,'installed_role_receipt':None})
    if any(r['explicit_analyzer_tool_present'] or not r['metadata_input_parity'] for r in exposure):
        raise ValueError('consumer exposure changed; review gap scope')
    summary=d['old_summary'];base=d['old_baseline'];expanded=d['old_expanded'];graph=expanded['graph']
    if summary['independent_acceptance']!='not_recorded' or graph['status']!='expanded_bounded' or len(graph['functions'])!=5:
        raise ValueError('historical fixture scope')
    cks=rpc_rows(d['old_requests'],d['old_responses']);ckg=rpc_rows(d['old_ckg_requests'],d['old_ckg_responses'])
    if len(cks)!=13 or len(ckg)!=7:raise ValueError('historical MCP denominator')
    baseline_request=next(r['arguments'] for r in cks if r['id']==3)
    expanded_request=next(r['arguments'] for r in cks if r['id']==4)
    if baseline_request!=expanded_request['function'] or base['snapshot_sha256']!=graph['snapshot_sha256']:
        raise ValueError('historical seed/snapshot pair')
    local_limits_equal=base['analysis']['limits']==graph['functions'][0]['result']['analysis']['limits']
    if not local_limits_equal:raise ValueError('historical seed local limits')
    metrics=summary['metrics']
    arithmetic={'kind':'derived_from_historical_fixture_declared_labels_not_current_quality',
      'baseline_recall':metrics['baseline_recall'],'expanded_recall':metrics['expanded_recall'],
      'recall_delta':metrics['expanded_recall']-metrics['baseline_recall'],
      'baseline_noise_fraction':0.0,'expanded_noise_fraction':metrics['expanded_noise']/metrics['expanded_returned'],
      'noise_fraction_delta':metrics['expanded_noise']/metrics['expanded_returned'],
      'single_server_elapsed_ratio':metrics['expanded_elapsed_micros']/metrics['baseline_elapsed_micros'],
      'warm_p95_latency_ratio':None,'formal_threshold_verdict':None,
      'empty_return_noise_fraction_policy':None,'current_project_scores':None}
    old={'kind':'historical_one_six_function_fixture_not_twenty_independent_project_families',
      'summary':summary,'derived_arithmetic':arithmetic,'ckg_request_matrix':ckg,'cks_request_matrix':cks,
      'paired_same_seed_request':True,'paired_same_snapshot':True,'paired_seed_local_limits_equal':local_limits_equal,
      'baseline_limits':base['analysis']['limits'],'expanded_aggregate_limits':graph['limits'],
      'baseline_function':base['analysis']['source'],'expanded_functions':[r['result']['analysis']['source'] for r in graph['functions']],
      'hops':graph['hops'],'curated_flow':expanded['curated_flow'],
      'cap_results':{name:{'status':d['old_cap_'+name]['graph']['status'],
                          'stop_reasons':d['old_cap_'+name]['graph']['stop_reasons']} for name in ['functions','candidates','nodes','edges','facts']},
      'historical_failure_status':d['old_failure']['status'],'current_project_cases_covered':0,
      'warm_repeated_samples':None,'warm_definition_receipt':None,'actual_installed_analyzer_pair_receipt':None}
    counts={'draft_inputs':20,'production_seed_proposals':19,'development_stress_source_only':1,'intent_labels':4,
      'project_source_files':len({r['source']['path'] for r in cases}),'existing_ai_source_anchors':sum(len(r['existing_ai_anchors']) for r in cases),
      'pairwise_reviews':len(pairs),'same_file_pairs':sum(r['same_file'] for r in pairs),
      'same_intent_pairs':sum(r['same_intent_label'] for r in pairs),'overlap_with_exposed_wi1_sample':sum(r['same_seed_exposed_in_wi1_sample'] for r in cases),
      'approved_independent_families':0,'pending_independent_family_minimum':20,
      'independent_relevance_gold_records':0,'current_project_paired_receipts':0,'current_warm_sample_receipts':0,
      'contract_anchors':len(contracts),'adopted_consumer_source_files':2,'missing_explicit_analyzer_tools':2,
      'historical_ckg_requests':7,'historical_cks_requests':13,'historical_fixture_seed_pairs':1,
      'new_mcp_calls':0,'new_model_calls':0,'formal_runs':0}
    return {'public-inputs.json':d['public'],
      'private-family-pair-gaps.json':{'kind':'ai_family_pair_and_relevance_gap_review_not_gold','cases':cases,
        'all_intent_counts':dict(Counter(r['intent_label'] for r in cases)),
        'production_intent_counts':dict(Counter(r['intent_label'] for r in production)),
        'obligations':{k:'not_satisfied' for k in OBLIGATIONS},'matching_questions':d['proposals']['matching_questions']},
      'pairwise-review.json':pairs,
      'retrieval-path-review.json':{'kind':'source_inspection_not_actual_project_retrieval_or_installed_role',
        'contracts':contracts,'criteria_snapshot':d['criteria'],'consumer_exposure_gaps':exposure,
        'intent_is_engine_query_field':False,'incoming_call_expansion_implemented_here':False,
        'computed_argument_return_transfer':False,'verified_curated_dataset_join':False,
        'actual_project_retained_graph_receipt':None,'actual_project_paired_mcp_receipt':None},
      'warm-latency-contract.json':{'kind':'unfrozen_measurement_questions_not_benchmark',
        'questions':d['proposals']['warm_questions'],'warm_definition':None,'warmup_count':None,
        'repetitions_per_arm':None,'arm_order':None,'clock_and_measurement_boundary':None,
        'p95_definition':None,'ratio_definition':None,'invalid_samples_denominator':None,
        'family_aggregation':None,'raw_samples':None,'warm_p95_ratio':None,'review_receipt':None},
      'historical-evidence-scope.json':old,
      'gap-summary.json':{'kind':'expansion_family_pair_gold_path_and_warm_gaps_preparation_only','counts':counts,
        'operational_draft_shortfall_even_before_independence_review':1,
        'work_item_scores':None,'protocol_or_criteria_changed':False,
        'stress_excluded_from_operational_family_denominator':True,'missing_gold_not_empty_approved_gold':True}}


def table(artifacts):
    lines=['WI-6 독립 가족·기본/확장 쌍·relevance/noise·실제 경로·warm 지연 공백 — 비공개 AI 자료',
      '20카드 = 운영 원문 19개 + 개발 stress 1개. 독립 가족/gold/실제 프로젝트 쌍/warm 영수증 0개.',
      '과거 6함수 fixture의 단일 seed 쌍은 현재 20독립 가족 또는 warm p95 증거가 아니다.']
    for r in artifacts['private-family-pair-gaps.json']['cases']:
        lines+=['',r['case_id']+' '+r['intent_label']+' '+r['scope'],r['intent_relevance_question'],*r['existing_unknowns']]
    for r in artifacts['retrieval-path-review.json']['contracts']:
        lines+=['',r['id']+' '+r['path']+':'+str(r['lines'][0]),r['fact'],'미충족: '+r['gap']]
    lines+=['','정답/쌍/분모 질문:',*artifacts['private-family-pair-gaps.json']['matching_questions'],
            '','warm 지연 계약 질문:',*artifacts['warm-latency-contract.json']['questions']]
    return '\n'.join(lines)+'\n'


def prepare(output,consumer_repo):
    if output.exists():raise ValueError('use a new output; preserve prior evidence')
    output.mkdir(parents=True)
    revision=json.loads((ROOT/INPUTS['host']).read_text())['source_adoptions']['coding_agent']['source_commit']
    capture=SourceCapture(consumer_repo,output/'consumer-source',revision)
    for path in CONSUMER:capture.source(path)
    dump(output/'consumer-source/git-proof.json',capture.proof())
    artifacts=generate(inputs(output))
    for name,value in artifacts.items():dump(output/name,value)
    (output/'REVIEW-TABLE.txt').write_text(table(artifacts));(output/'.gitattributes').write_bytes(ATTRIBUTES)
    (output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'expansion_gap_preparation_only',
      'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in bindings()},
      'generator_sha256':sha(Path(__file__).read_bytes()),'counts':artifacts['gap-summary.json']['counts'],
      'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['output','consumer-repo']:parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();prepare(args.output.resolve(),args.consumer_repo.resolve())
