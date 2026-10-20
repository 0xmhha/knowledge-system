#!/usr/bin/env python3
"""Audit WI-2 sample/gold, command contracts and actual MCP evidence boundaries."""
import argparse
from collections import Counter
import importlib.util
import itertools
import json
from pathlib import Path

from source_packet import SourceCapture, safe, sha, verify

ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
CONTROL=BASE/'rule-coverage-v1'
OBS=BASE/'rule-observations-v1'
PROJECT=BASE/'project-rule-review-v1'
CALLER=BASE/'rule-caller-context-v1'
OLD=Path('system/eval/upper-work-items-implementation-2026-10-14/wi-2')
HOST=Path('system/eval/operational-preparation-2026-10-15/isolated-host')
REVISION='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'
RULES=['parameter-command-name','discarded-conversion-error']
SOURCE=[Path(p) for p in ['pkg/graph/valueflow/rules.go','pkg/graph/valueflow/query.go',
 'pkg/graph/mcphandlers/rules.go','pkg/graph/mcphandlers/valueflow_schema.go',
 'pkg/graph/mcphandlers/namespace.go','pkg/graph/mcphandlers/registerall.go',
 'pkg/graph/mcphandlers/safety.go','internal/system/mcp/valueflow.go',
 'internal/system/ckgclient/valueflow.go','internal/system/mcp/testdata/agent-mcp.schema.json']]
CONSUMER=['plugin/agents/analyzer.md','plugin/contracts/cks-tools.schema.json']
ATTRIBUTES=b'consumer-source/objects/*.commit binary\nconsumer-source/objects/*.tree binary\n'
MANIFESTS=[BASE/name/'manifest.json' for name in ['rule-coverage-v1','rule-observations-v1','project-rule-review-v1','rule-caller-context-v1']]
INPUTS={'public':CONTROL/'public-inputs.json','private':CONTROL/'private-ai-review.json',
 'old_fixtures':CONTROL/'existing-fixtures.json','contract':CONTROL/'contract.json','coverage':CONTROL/'coverage-summary.json',
 'controlled_review':OBS/'case-review.json','controlled_summary':OBS/'coverage-summary.json',
 'project_calls':PROJECT/'call-review.json','project_functions':PROJECT/'function-review.json',
 'project_candidates':PROJECT/'candidate-review.json','project_summary':PROJECT/'coverage-summary.json',
 'callers':CALLER/'caller-review.json','contexts':CALLER/'private-ai-context.json',
 'caller_summary':CALLER/'coverage-summary.json','proposals':Path('scripts/fixtures/rule-gap-proposals.json'),
 'host':HOST/'manifest.json','consumer_inventory':HOST/'coding_agent-source-files.json',
 'old_manifest':OLD/'manifest.json','old_summary':OLD/'execution/summary.json',
 'old_failures':OLD/'historical-failures/causes.json',
 'old_requests':OLD/'execution/mcp.stdin.jsonl','old_responses':OLD/'execution/mcp.stdout.jsonl',
 'old_cks_requests':OLD/'execution/cks/mcp.stdin.jsonl','old_cks_responses':OLD/'execution/cks/mcp.stdout.jsonl'}
for name in ['Parameter','Kill','Discard','Sanitizer','Alias']:
    INPUTS['old_ckg_'+name]=OLD/('execution/'+name+'.json')
    INPUTS['old_cks_'+name]=OLD/('execution/cks/'+name+'.json')
OBLIGATIONS=['independent_family_exposure_and_dev_final','per_rule_positive_negative_sufficiency',
 'independent_complete_candidate_gold','canonical_matching_and_unknown_denominators',
 'source_sink_sanitizer_external_identity_contract','actual_command_caller_and_environment_contract',
 'retained_project_graph_and_source_snapshot_pins','actual_mcp_client_schema_and_installed_role_receipts',
 'baseline_limits_and_metric_mapping','human_review_and_final_acceptance']


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
    for path in MANIFESTS:
        for name,digest in json.loads((ROOT/path).read_text())['artifacts'].items():
            if sha(safe(ROOT/path.parent,name).read_bytes())!=digest:raise ValueError('prior artifact binding: '+name)
    for p in INPUTS.values():
        if p.is_relative_to(OLD) and p.name!='manifest.json':
            if d['old_manifest']['files_sha256'][p.relative_to(OLD).as_posix()]!=sha((ROOT/p).read_bytes()):
                raise ValueError('historical evidence binding')
    proof=verify(ROOT/PROJECT,REVISION)
    d['project_source']={p:safe(ROOT/PROJECT,r['artifact']).read_bytes() for p,r in proof['files'].items()}
    revision=d['host']['source_adoptions']['coding_agent']['source_commit']
    proof=verify(packet/'consumer-source',revision)
    if set(proof['files'])!=set(CONSUMER):raise ValueError('consumer source inventory')
    adopted={r['path']:r['sha256'] for r in d['consumer_inventory']}
    d['consumer_source']={p:safe(packet/'consumer-source',r['artifact']).read_bytes() for p,r in proof['files'].items()}
    if any(sha(raw)!=adopted[p] for p,raw in d['consumer_source'].items()):raise ValueError('adopted source binding')
    d['current_source']={str(p):(ROOT/p).read_bytes() for p in SOURCE}
    protocol=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json').read_text())
    progress=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json').read_text())
    d['criteria']={'protocol':next(r for r in protocol['input_groups'] if r['id']=='WI-2'),
                   'progress':next(r for r in progress['items'] if r['id']=='WI-2')}
    return d


def without_elapsed(value):
    if isinstance(value,dict):return {k:without_elapsed(v) for k,v in value.items() if k!='elapsed_micros'}
    if isinstance(value,list):return [without_elapsed(v) for v in value]
    return value


def request_scope(requests,responses):
    req,res=unique(requests,'id'),unique(responses,'id')
    if set(req)!=set(res):raise ValueError('historical RPC request/response identity')
    rows=[]
    for key,r in req.items():
        result=res[key].get('result',{});structured=result.get('structuredContent')
        if structured is None:
            text=''.join(c.get('text','') for c in result.get('content',[]) if c.get('type')=='text')
            try:structured=json.loads(text)
            except ValueError:structured=None
        analysis=(structured or {}).get('analysis') or {} if isinstance(structured,dict) else {}
        params=r.get('params',{});args=params.get('arguments',{})
        rows.append({'id':key,'method':r['method'],'tool':params.get('name'),
                     'explicit_rule_pack':args.get('rule_pack'),'rpc_error':'error' in res[key],
                     'tool_error':result.get('isError',False),
                     'status':structured.get('status') if isinstance(structured,dict) else None,
                     'function':analysis.get('source',{}).get('function_name'),
                     'source_sha256':analysis.get('source',{}).get('sha256'),
                     'rule_status':analysis.get('rules',{}).get('status')})
    return rows


def generate(d):
    public=unique(d['public']['cases'],'id');private=unique(d['private']['cases'],'id')
    observations=unique(d['controlled_review'],'case_id')
    if len(public)!=40 or set(public)!=set(private) or set(public)!=set(observations):raise ValueError('forty controlled inventory')
    controlled=[]
    for key,r in private.items():
        obs=observations[key]
        if r['gold'] is not False or r['approved'] is not False or obs['gold'] is not False or obs['approved'] is not False:
            raise ValueError('controlled gold promotion')
        if sha(public[key]['code'].encode())!=public[key]['source_sha256']:raise ValueError('controlled source binding')
        controlled.append({'id':key,'rule':r['rule_proposal'],'mechanism_label':r['mechanism'],
          'coverage_tags':r['coverage_tags'],'source_sha256':public[key]['source_sha256'],
          'ai_proposal_label':r['proposal_label'],'observed_candidates':obs['observed_candidates'],
          'base_status':obs['base_status'],'diagnostic_codes':obs['diagnostic_codes'],
          'candidate_spans':obs['candidate_spans'],
          'exact_old_fixture_matches':[x['name'] for x in d['old_fixtures'] if x['code']==public[key]['code']],
          'same_mechanism_peers':[k for k,v in private.items() if k!=key and v['mechanism']==r['mechanism']],
          'prior_exposure':r['prior_exposure'],'semantic_independence':'not_reviewed',
          'exhaustive_candidate_gold':None,'independent_label':None,'split':None,
          'safety_verdict':'unknown','approved':False})
    calls=unique(d['project_calls'],'call_id');functions=unique(d['project_functions'],'case_id')
    if len(calls)!=68 or len(functions)!=51 or len(d['project_candidates'])!=2:raise ValueError('project denominator')
    project=[]
    for key,r in calls.items():
        src=r['source'];raw=d['project_source'][src['path']]
        if sha(raw)!=src['file_sha256'] or sha(raw[src['start_byte']:src['end_byte_exclusive']])!=src['span_sha256']:
            raise ValueError('project call source binding')
        if r['independent_label'] is not None or r['gold'] is not False or r['approved'] is not False:raise ValueError('project gold promotion')
        project.append({'call_id':key,'case_id':r['case_id'],'path':src['path'],'function':src['enclosing_function'],
          'span_sha256':src['span_sha256'],'function_sha256':src['function_sha256'],
          'rule':RULES[0] if src['import_path_spelling']=='os/exec' else RULES[1],
          'observation_status':r['observation_status'],'candidate_ids':r['candidate_ids'],
          'rule_status':r['rules_status'],'syntax_hints':r['syntax_hints_not_semantic_gold'],
          'independent_label':None,'exhaustive_candidate_gold':None,'independent_family':None,
          'runtime_reachability':'not_established','safety_verdict':'unknown','split':None,'approved':False})
    caller=d['caller_summary']
    if len(d['callers'])!=14 or caller['original_project_call_denominator_unchanged']!=68 or caller['original_project_candidates_unchanged']!=2:
        raise ValueError('caller/original denominator separation')
    contexts=d['contexts']['reviews']
    for r in contexts:
        if r['approved'] is not False or r['gold'] is not False or r['independence_approved'] is not False:raise ValueError('caller approval promotion')
        for a in r['anchors']:
            if {k:v for k,v in a.items() if k in span(d['project_source'][a['path']],a['start_line'],a['end_line'])}!=span(d['project_source'][a['path']],a['start_line'],a['end_line']):
                raise ValueError('caller anchor binding')
    per_rule=[]
    for rule in RULES:
        rows=[r for r in controlled if r['rule']==rule];ps=[r for r in project if r['rule']==rule]
        per_rule.append({'rule':rule,'controlled_drafts':len(rows),'ai_label_counts':dict(Counter(r['ai_proposal_label'] for r in rows)),
          'controlled_with_candidates':sum(r['observed_candidates']>0 for r in rows),
          'coverage_tag_proposals':dict(Counter(t for r in rows for t in r['coverage_tags'])),
          'project_calls':len(ps),'project_calls_with_candidates':sum(bool(r['candidate_ids']) for r in ps),
          'approved_independent_positive':0,'approved_independent_negative':0,
          'pending_minimum_independent_positive':10,'pending_minimum_independent_negative':10,
          'project_positive_sufficiency':'not_established','precision':None,'recall':None,'confusion_matrix':None})
    contracts=[]
    for r in d['proposals']['contracts']:
        source=d['consumer_source'] if r['scope']=='adopted_consumer' else d['current_source']
        contracts.append({**r,'anchor':span(source[r['path']],*r['lines']),'approved':False})
    unique(contracts,'id')
    agent=d['consumer_source']['plugin/agents/analyzer.md'].decode()
    front=agent.split('---',2)[1]
    tools=[s.strip()[2:] for s in front.split('tools:\n',1)[1].split('\nskills:',1)[0].splitlines() if s.strip().startswith('- ')]
    alias='mcp__plugin_coding-agent_cks__cks_context_rule_candidates'
    consumer=json.loads(d['consumer_source']['plugin/contracts/cks-tools.schema.json'])['providers']['cks']['tools']['cks_context_rule_candidates']
    metadata=json.loads(d['current_source']['internal/system/mcp/testdata/agent-mcp.schema.json'])['tools']['cks.context.rule_candidates']
    role={'id':'ROLE-RULE-TOOL-01','required_by_body': 'rule_candidates responses' in agent,
      'consumer_contract_tool_present':True,'consumer_wire_name':consumer['wire_name'],
      'consumer_input_metadata_parity':metadata['input']==consumer['input'],
      'consumer_description_parity':metadata['description']==consumer['description'],
      'expected_direct_tool_alias':alias,'explicit_analyzer_tool_present':alias in tools,
      'explicit_tool_count':len(tools),'effective_host_permissions':None,'runtime_call_denial_proven':False,
      'actual_installed_role_receipt':None,
      'remaining':'resolve body/explicit allowlist difference and verify effective host exposure/call/handoff without assuming Bash grants a direct MCP tool'}
    if role['explicit_analyzer_tool_present'] or not role['required_by_body'] or not role['consumer_input_metadata_parity']:
        raise ValueError('adopted consumer exposure finding changed; review scope')
    summary=d['old_summary'];ckg=request_scope(d['old_requests'],d['old_responses']);cks=request_scope(d['old_cks_requests'],d['old_cks_responses'])
    if len(ckg)!=58 or len(cks)!=9 or summary['cases']!=23 or summary['matching_contract_passed']!=23 or summary['independent_acceptance']!='not_recorded':
        raise ValueError('historical MCP scope')
    parity=[{'function':name,'equal_except_elapsed':without_elapsed(d['old_ckg_'+name])==without_elapsed(d['old_cks_'+name])}
            for name in ['Parameter','Kill','Discard','Sanitizer','Alias']]
    if not all(r['equal_except_elapsed'] for r in parity):raise ValueError('historical CKG/CKS parity')
    counts={'supported_rules':2,'controlled_drafts':40,'controlled_local_observations':80,'project_call_records':68,
      'project_source_functions':len(functions),'project_source_files':len(d['project_source']),'project_local_observations':102,
      'existing_developer_fixtures':len(d['old_fixtures']),'manual_annotation_queries_separate':d['coverage']['counts']['annotation_queries_separate'],
      'project_calls_with_candidates':2,'project_empty_unlabelled':65,'project_closure_excluded_unlabelled':1,
      'caller_references_separate':14,'caller_context_reviews':5,'caller_context_anchors':11,
      'approved_independent_positives':0,'approved_independent_negatives':0,'independent_gold_records':0,
      'contract_anchors':len(contracts),'adopted_consumer_source_files':2,'consumer_exposure_findings':1,
      'controlled_pairwise_comparisons':780,'controlled_vs_old_fixture_comparisons':920,
      'historical_ckg_requests':58,'historical_cks_requests':9,'historical_compiled_cases':23,
      'new_mcp_calls':0,'new_model_calls':0,'formal_runs':0}
    exposure={'controlled_pairwise_comparisons':780,
      'same_mechanism_label_pairs':[{'a':a['id'],'b':b['id'],'independence':'not_reviewed'} for a,b in itertools.combinations(controlled,2) if a['mechanism_label']==b['mechanism_label']],
      'controlled_vs_old_fixture_comparisons':920,'exact_old_matches':sum(len(r['exact_old_fixture_matches']) for r in controlled),
      'all_forty_exposed_development_inputs':True,'fresh_holdout_claimed':False,'approved_independent_units':0}
    old={'kind':'historical_compiled_fixture_mcp_not_current_project_or_independent_acceptance',
      'summary':summary,'ckg_request_matrix':ckg,'cks_request_matrix':cks,'five_function_parity':parity,
      'historical_failure_status':d['old_failures']['status'],
      'metric_boundary':{'historical_label_kind':'fixture_review_condition_not_current_independent_candidate_gold',
        'historical_precision':summary['precision'],'historical_recall':summary['recall'],
        'equivalence_to_current_wi2_quality_established':False,'current_project_scores':None},
      'current_project_cases_covered':0,'installed_analyzer_role_executed_for_current_cases':False,'new_backend_calls':0}
    return {'public-inputs.json':d['public'],
      'private-sample-gaps.json':{'kind':'exposed_source_and_observation_review_not_independent_gold',
        'controlled':controlled,'project':project,'per_rule':per_rule,'exposure_review':exposure,
        'obligations':{k:'not_satisfied' for k in OBLIGATIONS},'matching_questions':d['proposals']['matching_questions'],
        'canonical_matching_rules':None,'baseline':None,'sample_sufficiency_review':None,'review_receipt':None},
      'command-contract-review.json':{'kind':'source_syntax_and_ai_context_not_command_execution',
        'candidate_calls':d['project_candidates'],'callers_separate_from_project_denominator':d['callers'],
        'existing_ai_context_reviews':contexts,'contract_questions':d['proposals']['command_contract_questions'],
        'actual_command_execution_receipt':None,'attacker_control_receipt':None,'resolved_call_graph_receipt':None,
        'new_project_commands_executed':0,'formal_runs':0},
      'mcp-path-review.json':{'kind':'current_and_adopted_source_inspection_not_current_project_host_execution',
        'contracts':contracts,'criteria_snapshot':d['criteria'],'role_exposure_finding':role,
        'project_retained_dataset_receipt':None,'project_mcp_roundtrip_receipt':None,
        'independent_scoring_receipt':None,'new_mcp_calls':0,'formal_runs':0},
      'historical-evidence-scope.json':old,
      'gap-summary.json':{'kind':'rule_sample_gold_command_and_mcp_gaps_preparation_only','counts':counts,
        'current_scores':None,'controlled_and_project_denominators_separate':True,
        'empty_candidates_never_safety_or_negative_gold':True,'protocol_or_criteria_changed':False}}


def table(artifacts):
    lines=['WI-2 독립 표본·정답·명령/호출자 계약·MCP 경로 공백 — 비공개 AI 준비 자료',
      '통제 40개와 프로젝트 68호출/51함수는 별도 분모. 독립 정답·승인 양성/음성 0개.',
      '과거 실제 CKG 58/CKS 9요청 시험은 존재한다. 현재 프로젝트/설치 역할 수용은 미확보.',
      '구체적 공백 ROLE-RULE-TOOL-01: Analyzer 본문/소비자 schema에 도구가 있으나 explicit tools 목록에 없음.']
    for r in artifacts['private-sample-gaps.json']['per_rule']:
        lines+=['',r['rule'],json.dumps(r,ensure_ascii=False)]
    for r in artifacts['mcp-path-review.json']['contracts']:
        lines+=['',r['id']+' '+r['path']+':'+str(r['lines'][0]),r['fact'],'미충족: '+r['gap']]
    lines+=['','독립 정답·매칭 질문:',*artifacts['private-sample-gaps.json']['matching_questions'],
            '','명령/호출자 계약 질문:',*artifacts['command-contract-review.json']['contract_questions']]
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
    dump(output/'manifest.json',{'schema_version':1,'kind':'rule_gap_preparation_only',
      'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in bindings()},
      'generator_sha256':sha(Path(__file__).read_bytes()),'counts':artifacts['gap-summary.json']['counts'],
      'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['output','consumer-repo']:parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();prepare(args.output.resolve(),args.consumer_repo.resolve())
