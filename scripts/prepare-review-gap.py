#!/usr/bin/env python3
"""Audit WI-C contracts, rubric, artifact handoff and adopted role gaps."""
import argparse
import importlib.util
import json
from pathlib import Path
import re

from source_packet import SourceCapture, safe, sha, verify

ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR=BASE/'review-coverage-v1'
HOST=Path('system/eval/operational-preparation-2026-10-15/isolated-host')
OLD=Path('system/eval/upper-work-items-implementation-2026-10-14/wi-c')
CONSUMER=['plugin/'+p for p in ['agents/analyzer.md','agents/planner.md','agents/orchestrator.md',
 'contracts/cks-tools.schema.json','scripts/deep_review.py','scripts/tests/test_deep_review.py',
 'scripts/tests/fixtures/deep-review-bundle.json','scripts/tests/fixtures/deep-review-judgments.json',
 'hooks/deep-review-guard.py','hooks/completion-guard.py','hooks/hooks.json',
 'scripts/oracle_gate.py','scripts/publication_policy.py','scripts/fix_selection.py',
 'skills/state-machine/SKILL.md','skills/structured-review/SKILL.md']]
SOURCE=[Path(p) for p in ['internal/system/mcp/review.go','internal/system/ckgclient/real.go',
 'internal/graph/persist/sqlite_review.go','internal/graph/persist/review.go',
 'internal/system/mcp/testdata/agent-mcp.schema.json']]
NATIVE_SOURCE=[p.relative_to(ROOT) for folder in ['internal/system/mcp','internal/system/ckgclient','internal/graph/persist']
               for p in sorted(Path(ROOT/folder).glob('*.go'))]
ATTRIBUTES=b'consumer-source/objects/*.commit binary\nconsumer-source/objects/*.tree binary\n'
INPUTS={'public':PRIOR/'public-inputs.json','private':PRIOR/'private-coverage.json',
 'prior_summary':PRIOR/'coverage-summary.json','pairs':PRIOR/'pairwise-review.json',
 'proposals':Path('scripts/fixtures/review-gap-proposals.json'),'host':HOST/'manifest.json',
 'consumer_inventory':HOST/'coding_agent-source-files.json','old_manifest':OLD/'manifest.json',
 'old_summary':OLD/'execution/summary.json','old_bundle':OLD/'execution/bundle.json',
 'old_judgments':OLD/'execution/judgments.json',
 'old_requests':OLD/'execution/cks/mcp.stdin.jsonl','old_responses':OLD/'execution/cks/mcp.stdout.jsonl',
 'old_report':OLD/'execution/.coding-agent/tickets/DEEP-REVIEW-FIXTURE/deep-review.json',
 'old_state':OLD/'execution/.coding-agent/tickets/DEEP-REVIEW-FIXTURE/state.json',
 'old_verification':OLD/'execution/verify-delivery.stdout.txt'}
SECTIONS=['data_state','guards_exceptions','complexity','resources','io','priority_tradeoffs']
OBLIGATIONS=['independent_case_module_exposure_split','complete_required_items_gold',
 'requirements_impact_and_priority_rubric','actual_retained_project_dual_tool_receipts',
 'structural_snapshot_scope','typed_report_and_diagram_scope','source_artifact_state_hashes',
 'planner_handoff_contract_alignment','installed_role_tool_hook_and_cli_receipts',
 'unknown_error_abstention_denominators','independent_final_acceptance']


def load(name):
    spec=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


helpers=load('prepare-semantic-gap.py')
dump,unique,span=helpers.dump,helpers.unique,helpers.span


def bindings():
    prior=load('prepare-review-coverage.py')
    return sorted(set(INPUTS.values())|set(SOURCE)|set(NATIVE_SOURCE)|set(prior.bindings(prior.source_inputs()))|
      {PRIOR/'manifest.json',Path('scripts/prepare-review-coverage.py'),Path('scripts/prepare-semantic-gap.py'),
       Path('scripts/source_packet.py'),Path('scripts/run-review-contract-probe.py')})


def inputs(packet):
    d={k:([json.loads(s) for s in (ROOT/p).read_text().splitlines() if s.strip()] if p.suffix=='.jsonl'
          else json.loads((ROOT/p).read_text())) for k,p in INPUTS.items()}
    prior=load('prepare-review-coverage.py');original=prior.generate(prior.source_inputs())
    manifest=json.loads((ROOT/PRIOR/'manifest.json').read_text())
    for name,digest in manifest['artifacts'].items():
        if sha(safe(ROOT/PRIOR,name).read_bytes())!=digest:raise ValueError('prior artifact binding')
    for name,value in original.items():
        if json.loads((ROOT/PRIOR/name).read_text())!=value:raise ValueError('prior review derivation')
    for p in INPUTS.values():
        if p.is_relative_to(OLD) and p.name!='manifest.json':
            if d['old_manifest']['files_sha256'][p.relative_to(OLD).as_posix()]!=sha((ROOT/p).read_bytes()):
                raise ValueError('historical evidence binding')
    revision=d['host']['source_adoptions']['coding_agent']['source_commit']
    proof=verify(packet/'consumer-source',revision)
    if set(proof['files'])!=set(CONSUMER):raise ValueError('consumer source inventory')
    d['consumer_source']={p:safe(packet/'consumer-source',r['artifact']).read_bytes() for p,r in proof['files'].items()}
    inventory={r['path']:r['sha256'] for r in d['consumer_inventory']}
    if any(sha(raw)!=inventory[p] for p,raw in d['consumer_source'].items()):raise ValueError('adopted source binding')
    d['current_source']={str(p):(ROOT/p).read_bytes() for p in SOURCE}
    protocol=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json').read_text())
    progress=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json').read_text())
    d['criteria']={'protocol':next(r for r in protocol['input_groups'] if r['id']=='WI-C'),
                   'progress':next(r for r in progress['items'] if r['id']=='WI-C')}
    return d


def evidence(packet,d):
    probe=json.loads((packet/'probe/result.json').read_text())
    if probe['kind']!='owned_local_contract_probe_not_model_or_installed_host':raise ValueError('probe scope')
    if probe['original_tests']!={'run':15,'failures':0,'errors':0,'skipped':0}:raise ValueError('adopted regression result')
    log=(packet/'probe/original-tests.txt').read_text()
    if not re.search(r'Ran 15 tests in [\d.]+s\s+OK\s*$',log):raise ValueError('adopted test log')
    expected={'baseline_valid':True,'one_criterion_valid':True,'empty_criteria_rejected':True,
      'empty_findings_valid':True,'changed_priority_valid':True,'semantic_claim_not_adjudicated':True,
      'request_expected_pins_optional':True,'structural_content_not_snapshot_verified':True,
      'all_unknown_rejected':True,'report_tamper_rejected':True,'diagram_tamper_rejected':True,
      'legacy_planner_missing_report_hash_rejected':True}
    if probe['checks']!=expected or probe['new_mcp_calls']!=0 or probe['new_model_calls']!=0 or probe['formal_acceptance']!='not_recorded':raise ValueError('contract probe accounting')
    expected_hashes={p:sha(raw) for p,raw in d['consumer_source'].items()}
    if probe['adopted_source_sha256']!=expected_hashes:raise ValueError('probe source bindings')
    if probe['runner_sha256']!=sha((ROOT/'scripts/run-review-contract-probe.py').read_bytes()):raise ValueError('probe runner binding')
    native=[]
    for mode in ['normal','race']:
        rows=[json.loads(s) for s in (packet/f'native/{mode}.jsonl').read_text().splitlines()]
        tests=sorted((r['Package'],r['Test']) for r in rows if r.get('Action')=='pass' and 'Test'in r)
        packages=sorted(r['Package'] for r in rows if r.get('Action')=='pass' and 'Test'not in r)
        receipt=json.loads((packet/f'native/{mode}-receipt.json').read_text())
        expected_names={'TestHandleReviewFunctionSeparatesTheViews','TestHandleReviewFunctionErrors',
          'TestReviewFunctionResolvesTheSeedAndWrapsIndexErrors','TestRebuildReviewIndex',
          'TestReviewContainsQueryUsesTheSourceIndex','TestReviewCallerWalkIsBounded','TestReviewCallerQueryUsesTheDestinationIndex'}
        expected_packages={'github.com/0xmhha/knowledge-system/'+p for p in ['internal/system/mcp','internal/system/ckgclient','internal/graph/persist']}
        if len(tests)!=7 or {n for _,n in tests}!=expected_names or set(packages)!=expected_packages or len(packages)!=3 or any(r.get('Action')in('fail','skip') for r in rows):raise ValueError('native test inventory/result')
        if receipt['exit_code']!=0 or receipt['passed_test_records']!=7 or sorted(receipt['tests'])!=sorted(expected_names) or sorted(receipt['packages'])!=packages:raise ValueError('native receipt')
        if ('-race' in receipt['command'])!=(mode=='race') or '-count=1' not in receipt['command']:raise ValueError('native mode/repetition')
        if (packet/f'native/{mode}.stderr.txt').read_bytes():raise ValueError('native stderr')
        native.append({'mode':mode,'passed_tests':7,'completed_packages':3,'scope':'owned_tests_not_actual_project_or_deployed_host'})
    return {'kind':'owned_source_bound_contract_checks_not_independent_quality',
      'probe':probe,'native_modes':native,'new_external_mcp_sessions':0,'new_network_listeners':0,
      'new_project_commands_executed':0,'installed_host_verified':False,'independent_acceptance':'not_recorded'}


def generate(d):
    public=unique(d['public']['cases'],'candidate_id');private=unique(d['private']['cases'],'candidate_id')
    proposals=unique(d['proposals']['cases'],'candidate_id')
    if len(public)!=10 or set(public)!=set(private) or set(public)!=set(proposals):raise ValueError('ten review inventory')
    cases=[]
    for cid,p in public.items():
        r=private[cid];q=proposals[cid]
        if r['approved'] is not False or r['required_items_gold'] is not None or r['quality_scoring_allowed'] is not False or r['split'] is not None:raise ValueError('independent review promotion')
        priority=q['priority_proposal']
        if priority['assigned_level'] is not None or priority['approved'] is not False or priority['requirements_or_impact_receipt'] is not None:raise ValueError('priority promotion')
        if len(q['required_item_questions'])!=2:raise ValueError('case obligations missing')
        cases.append({**r,'required_item_questions':q['required_item_questions'],'priority_triage_draft':priority,
          'six_section_mapping_proposal':{'data_state':'state_guard','guards_exceptions':'state_guard',
            'complexity':'resources_cost','resources':'resources_cost','io':'resources_cost','priority_tradeoffs':'priority'},
          'six_sections_are_complete_gold':False,'requirements_and_caller_contract_receipt':None,
          'independent_priority_gold':None,'matching_rules':None,'omission_count':None,
          'actual_dual_tool_receipt':None,'installed_role_receipt':None})
    contracts=[]
    for r in d['proposals']['contracts']:
        source=d['consumer_source'] if r['scope']=='adopted_consumer' else d['current_source']
        contracts.append({**r,'anchor':span(source[r['path']],*r['lines']),'approved':False})
    unique(contracts,'id')
    schema=json.loads(d['consumer_source']['plugin/contracts/cks-tools.schema.json'])['providers']['cks']['tools']
    current=json.loads(d['current_source']['internal/system/mcp/testdata/agent-mcp.schema.json'])['tools']
    exposure=[]
    for role in ['analyzer','planner']:
        front=d['consumer_source']['plugin/agents/'+role+'.md'].decode().split('---',2)[1]
        tools=[s.strip()[2:] for s in front.splitlines() if s.strip().startswith('- ')]
        for name in ['review_function','rule_candidates']:
            alias='mcp__plugin_coding-agent_cks__cks_context_'+name
            c=schema['cks_context_'+name];m=current['cks.context.'+name]
            exposure.append({'role':role,'tool':name,'wire_name':c['wire_name'],
              'metadata_input_parity':c['input']==m['input'],'explicit_alias':alias,
              'explicit_role_tool_present':alias in tools,'effective_host_permissions':None,
              'runtime_denial_proven':False,'installed_role_receipt':None})
    if any(r['explicit_role_tool_present'] or not r['metadata_input_parity'] for r in exposure):raise ValueError('role exposure changed; review scope')
    requests=unique(d['old_requests'],'id');responses=unique(d['old_responses'],'id')
    if set(requests)!=set(responses) or len(requests)!=5:raise ValueError('historical MCP identities')
    matrix=[]
    for k,r in requests.items():
        response=responses[k];params=r.get('params',{})
        if 'error'in response or response.get('result',{}).get('isError'):raise ValueError('historical MCP error')
        matrix.append({'id':k,'method':r['method'],'tool':params.get('name'),'arguments':params.get('arguments')})
    summary=d['old_summary'];report=d['old_report'];state=d['old_state']
    if summary['independent_acceptance']!='not_recorded' or len(summary['checks'])!=16 or summary['cks_exit']!=0 or summary['ckg_exit']!=0:raise ValueError('historical implementation scope')
    if report['status']!='structured_partial_review' or report['structural_review']['snapshot_association']!='identity_only_unverified_snapshot' or d['old_verification']['independent_acceptance']!='not_recorded':raise ValueError('historical partial/acceptance scope')
    if state['states']['ANALYSIS']['deep_review_sha256']!=sha((ROOT/INPUTS['old_report']).read_bytes()):raise ValueError('historical handoff hash')
    counts={'review_proposals':10,'module_labels':len({r['module'] for r in cases}),
      'project_source_files':len({r['source']['path'] for r in cases}),
      'existing_ai_anchors':sum(len(r['existing_ai_anchors']) for r in cases),
      'same_case_contexts':sum(r['context_supplement']['available'] for r in cases),
      'required_item_questions':sum(len(r['required_item_questions']) for r in cases),
      'priority_pending':10,'pairwise_reviews':len(d['pairs']),
      'same_module_pairs':sum(r['same_module'] for r in d['pairs']),
      'same_file_pairs':sum(r['same_source_file'] for r in d['pairs']),
      'approved_independent_reviews':0,'independent_required_item_gold':0,'current_project_review_receipts':0,
      'contract_anchors':len(contracts),'adopted_consumer_source_files':len(CONSUMER),
      'missing_explicit_role_tool_entries':len(exposure),'planner_contract_conflicts':1,
      'historical_cks_requests':5,'historical_fixture_checks':16,'original_consumer_tests':15,
      'local_contract_probe_checks':12,'native_tests_per_mode':7,'native_modes':2,
      'new_mcp_calls':0,'new_model_calls':0,'formal_runs':0}
    return {'public-inputs.json':d['public'],
      'private-review-gaps.json':{'kind':'ai_required_item_priority_and_review_gap_draft_not_gold','cases':cases,
        'obligations':{k:'not_satisfied' for k in OBLIGATIONS},'matching_questions':d['proposals']['matching_questions']},
      'pairwise-review.json':d['pairs'],
      'consumer-contract-review.json':{'kind':'source_contract_inspection_not_installed_role_execution',
        'contracts':contracts,'criteria_snapshot':d['criteria'],'role_exposure_gaps':exposure,
        'planner_contract_conflict':{'typed_priority_levels':['high','medium','low'],'typed_findings_cap':32,
          'legacy_planner_severity_levels':['critical','high','medium','low'],'legacy_planner_findings_cap':30,
          'legacy_planner_binds_report_hash':False,'legacy_planner_text_requires_typed_regeneration':False,
          'actual_role_failed':None,'external_consumer_modified':False},
        'six_sections_prove_required_item_completeness':False,'priority_enum_proves_priority_correctness':False,
        'criteria_labels_are_independent_gold':False,'structural_snapshot_atomicity_verified':False,
        'cfg_is_runtime_trace':False,'module_diagram_is_interprocedural_analysis':False,
        'diagram_bytes_prove_rendering_or_quality':False,'local_hash_authenticates_transport_or_reviewer':False,
        'hook_registry_proves_installed_hook':False,'shell_edits_guarded_by_deep_review_hook':False,
        'actual_installed_role_receipt':None,'actual_project_dual_tool_receipt':None},
      'historical-evidence-scope.json':{'kind':'preserved_owned_single_fixture_not_ten_independent_reviews',
        'summary':summary,'request_matrix':matrix,'typed_status':report['status'],
        'structural_snapshot_association':report['structural_review']['snapshot_association'],
        'sections':list(report['reasoning']['sections']),'criteria':report['reasoning']['criteria'],
        'delivery_verification':d['old_verification'],'current_project_cases_covered':0,
        'independent_priority_or_required_item_quality':None,'installed_role_execution_verified':False},
      'gap-summary.json':{'kind':'review_contract_rubric_artifact_handoff_and_role_gaps_preparation_only',
        'counts':counts,'work_item_scores':None,'required_omissions':None,
        'protocol_or_criteria_changed':False,'missing_gold_not_empty_approved_gold':True}}


def table(artifacts):
    lines=['WI-C 실제 리뷰·우선순위·필수 항목·보고서/그림·인수인계·설치 역할 공백 — 비공개 AI 자료',
      '10후보·8모듈·9파일, 독립 리뷰/필수 정답/현재 프로젝트 결과0개. 형식 검증은 의미 수용이 아니다.']
    for r in artifacts['private-review-gaps.json']['cases']:
        lines+=['',r['case_id']+' '+r['source']['name'],*r['required_item_questions'],
          r['categories_proposal']['priority']['question'],'priority: 미정; 필수 gold·생략 수·실제 호출·handoff 미확보']
    for r in artifacts['consumer-contract-review.json']['contracts']:
        lines+=['',r['id']+' '+r['path']+':'+str(r['lines'][0]),r['fact'],'미충족: '+r['gap']]
    return '\n'.join(lines)+'\n'


def prepare(output,consumer_repo,probe_root,native_root):
    if output.exists():raise ValueError('use a new output; preserve prior evidence')
    output.mkdir(parents=True)
    revision=json.loads((ROOT/INPUTS['host']).read_text())['source_adoptions']['coding_agent']['source_commit']
    capture=SourceCapture(consumer_repo,output/'consumer-source',revision)
    for path in CONSUMER:capture.source(path)
    dump(output/'consumer-source/git-proof.json',capture.proof())
    for folder,root,names in [('probe',probe_root,['result.json','original-tests.txt']),
      ('native',native_root,[mode+suffix for mode in ['normal','race'] for suffix in ['.jsonl','.stderr.txt','-receipt.json']])]:
        (output/folder).mkdir()
        for name in names:(output/folder/name).write_bytes((root/name).read_bytes())
    d=inputs(output);artifacts=generate(d);artifacts['local-contract-checks.json']=evidence(output,d)
    for name,value in artifacts.items():dump(output/name,value)
    (output/'public-inputs.json').write_bytes((ROOT/INPUTS['public']).read_bytes())
    (output/'REVIEW-TABLE.txt').write_text(table(artifacts));(output/'.gitattributes').write_bytes(ATTRIBUTES)
    (output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'review_gap_preparation_only',
      'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in bindings()},
      'generator_sha256':sha(Path(__file__).read_bytes()),'counts':artifacts['gap-summary.json']['counts'],
      'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['output','consumer-repo','probe-root','native-root']:parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();prepare(args.output.resolve(),args.consumer_repo.resolve(),args.probe_root.resolve(),args.native_root.resolve())
