#!/usr/bin/env python3
"""Bind a consumer correction specification and isolated plan to audited source."""
import argparse
import importlib.util
import json
from pathlib import Path

from source_packet import safe, sha, verify

ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
FIXTURE=Path('scripts/fixtures/consumer-contract-plan.json')
PACKETS=['rule-gap-v1','expansion-gap-v1','policy-security-gap-v1','review-gap-v1']
INPUTS=[FIXTURE,Path('internal/system/mcp/testdata/agent-mcp.schema.json'),
 BASE/'rule-gap-v1/mcp-path-review.json',BASE/'expansion-gap-v1/retrieval-path-review.json',
 BASE/'policy-security-gap-v1/boundary-and-consumer-review.json',BASE/'review-gap-v1/consumer-contract-review.json',
 Path('system/eval/operational-preparation-2026-10-15/isolated-host/manifest.json'),
 Path('scripts/operational-host-probe.py'),Path('scripts/source_packet.py'),Path('scripts/prepare-source-review-supplement.py')]
ROLE_TOOLS={'analyzer':{'value_dependencies','rule_candidates','value_dependency_expansion','policy_security','review_function'},
            'planner':{'rule_candidates','review_function'}}
REQUIRED_CHANGES=['plugin/agents/analyzer.md','plugin/agents/planner.md']
PRESERVED=['plugin/scripts/deep_review.py','plugin/hooks/deep-review-guard.py','plugin/hooks/hooks.json',
 'plugin/contracts/cks-tools.schema.json','plugin/agents/orchestrator.md','plugin/skills/state-machine/SKILL.md']


def load(name):
    spec=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


def dump(path,value):path.write_text(json.dumps(value,ensure_ascii=False,indent=2,allow_nan=False)+'\n')


def bindings():
    return sorted(set(INPUTS)|{BASE/n/'manifest.json' for n in PACKETS})


def unique(rows):
    values={r['id']:r for r in rows}
    if len(values)!=len(rows):raise ValueError('duplicate ID')
    return values


def validate(plan):
    def need(condition,message):
        if not condition:raise ValueError(message)
    need(plan['kind']=='consumer_contract_spec_and_isolated_plan_not_execution' and plan['status']=='specified_not_implemented_or_executed','specification scope')
    scope=plan['scope'];need(scope['required_consumer_changes']==REQUIRED_CHANGES,'minimal source change scope')
    need(scope['preserved_runtime_sources']==PRESERVED,'preserved runtime sources')
    need(scope['align_legacy_review_dispatch_and_return_text'] is True,'role dispatch alignment obligation')
    need(scope['allowed_test_changes']==['plugin/scripts/tests/test_deep_review.py','plugin/scripts/tests/test_review_role_contract.py'],'test change scope')
    for key in ['current_task_external_writes','current_task_implementation_performed','current_task_runtime_execution_performed',
                'backend_schema_or_permission_model_change','automatic_publication','existing_harness_proves_new_role_paths']:
        need(scope[key] is False,'implementation/execution promotion: '+key)
    need(plan['baseline']['consumer_commit']=='b7e78689676769e8536626e76eed7165d870e1ba','adopted baseline')
    need(plan['baseline']['consumer_source_scope']=='owned_local_adoption_not_production_installation','baseline scope')
    need(plan['baseline']['historical_host_version_is_execution_pin'] is False and plan['baseline']['new_execution_requires_binary_and_source_pins'] is True,'new execution pins')
    rows=plan['role_tool_changes'];pairs={(r['role'],r['tool']) for r in rows}
    expected={(role,tool) for role,tools in ROLE_TOOLS.items() for tool in tools}
    need(len(rows)==7 and pairs==expected,'exact seven role/tool entries')
    for r in rows:
        need(r['alias']=='mcp__plugin_coding-agent_cks__cks_context_'+r['tool'] and r['wire_name']=='cks.context.'+r['tool'] and r['expected_present_after_change'] is True,'exact MCP alias/wire')
    req=unique(plan['requirements']);tests=unique(plan['validation_cases']);steps=unique(plan['implementation_steps'])
    need(set(req)=={f'CCP-{n:02}'for n in range(1,13)},'twelve requirements')
    need(set(tests)=={f'T{n:02}'for n in range(1,23)},'twenty-two validation cases')
    need(set(steps)=={f'P{n:02}'for n in range(1,9)},'eight implementation steps')
    covered=set();step_covered=set();done=set()
    for r in plan['requirements']:
        need(r['implementation_status']=='not_implemented','requirement implementation promotion')
        need(bool(r['required_behavior'].strip()) and r['test_ids'] and set(r['test_ids'])<=set(tests),'requirement test coverage')
        covered.update(r['test_ids'])
    for r in plan['validation_cases']:
        need(r['status']=='not_run' and r['receipt'] is None,'planned test execution promotion')
        need(all(isinstance(r[k],str) and r[k].strip() for k in ['procedure','expected','layer']),'test procedure/oracle missing')
    for r in plan['implementation_steps']:
        need(r['status']=='not_started' and r['implementation_commit'] is None and r['validation_receipt'] is None,'step completion promotion')
        need(set(r['depends_on'])<=done and r['id'] not in done,'step dependency cycle/order')
        need(bool(r['test_ids']) and set(r['test_ids'])<=set(tests),'step tests missing')
        done.add(r['id']);step_covered.update(r['test_ids'])
    need(covered==set(tests) and step_covered==set(tests),'uncovered validation case')
    resources=plan['resource_plan']
    for k in ['real_model_calls_max','real_chain_nodes_max','production_writes_max','external_tracker_messages_max','push_or_pr_max']:
        need(resources[k]==0 and type(resources[k])is int,'zero side effect budget: '+k)
    exact={'parallel_host_sessions_max':1,'owned_mcp_children_max':2,'owned_loopback_responders_max':1,
      'tool_requests_per_process_max':64,'session_wallclock_seconds_max':900,'single_command_wallclock_seconds_max':180,
      'termination_grace_seconds':5,'forced_kill_grace_seconds':5,'new_session_retries_max':1,
      'provider_mode':'local_scripted_responder_no_upstream_provider','mcp_transport':'stdio',
      'responder_bind':'127.0.0.1 ephemeral port only','process_and_listener_cleanup_required':True,
      'global_settings_or_real_credentials_required':False,'environment_isolation_is_proven_security_sandbox':False}
    need(all(type(resources[k])is type(v) and resources[k]==v for k,v in exact.items()),'owned resource boundary')
    receipt=plan['receipt_contract']
    need(receipt['role_receipt_cannot_be_replaced_by_direct_client'] is True and receipt['no_fabricated_runtime_receipts'] is True,'actual role evidence boundary')
    need(receipt['model_quality_evaluated'] is False and receipt['independent_acceptance']=='not_recorded','receipt acceptance promotion')
    need(receipt['allowed_case_statuses']==['not_run','pass','fail','blocked','inconclusive'],'case status taxonomy')
    need(len(receipt['required_fields'])==22 and len(set(receipt['required_fields']))==22,'evidence field inventory')
    for key in ['actual_role_identity','effective_role_tool_inventory','raw_request_response_and_stderr_paths_sha256',
                'state_before_after_and_hash_bindings','hook_event_and_cli_exit_receipts','responder_bind_and_cleanup_receipt']:
        need(key in receipt['required_fields'],'required runtime evidence field')
    c=plan['completion']
    need(c['required_validation_cases']==22 and c['requires_real_role_receipts_for_T05'] is True and c['requires_cleanup_receipt'] is True,'candidate completion gate')
    for k in ['blocked_or_inconclusive_is_pass','rendering_not_available_is_pass','independent_quality_or_ontology_effect_evaluated','formal_execution_allowed']:
        need(c[k] is False,'completion promotion: '+k)
    need(c['formal_completed']==0 and c['formal_total']==9 and c['reviewer_assignment']=='미정' and c['freeze'] is None,'formal acceptance promotion')
    return plan


def inputs():
    plan=validate(json.loads((ROOT/FIXTURE).read_text()))
    for name in PACKETS:
        root=ROOT/BASE/name;m=json.loads((root/'manifest.json').read_text())
        for p,digest in m['artifacts'].items():
            if sha(safe(root,p).read_bytes())!=digest:raise ValueError('prior artifact binding: '+name+'/'+p)
    proof_root=ROOT/BASE/'review-gap-v1/consumer-source'
    proof=verify(proof_root,plan['baseline']['consumer_commit'])
    source={p:safe(proof_root,r['artifact']).read_bytes() for p,r in proof['files'].items()}
    consumer=json.loads(source['plugin/contracts/cks-tools.schema.json'])['providers']['cks']['tools']
    current=json.loads((ROOT/INPUTS[1]).read_text())['tools']
    tools={};dispatch=[]
    for role in ROLE_TOOLS:
        front=source['plugin/agents/'+role+'.md'].decode().split('---',2)[1]
        tools[role]=[s.strip()[2:] for s in front.split('tools:\n',1)[1].split('\nskills:',1)[0].splitlines() if s.strip().startswith('- ')]
        raw=source['plugin/agents/'+role+'.md']
        for number,line in enumerate(raw.splitlines(keepends=True),1):
            if line.lstrip().startswith(b'| code_review') and b'DONE' in line:
                dispatch.append({'role':role,'path':'plugin/agents/'+role+'.md','line':number,
                  'source_line':line.decode().rstrip('\n'),'line_sha256':sha(line),'alignment_required':True})
    if {r:len(v) for r,v in tools.items()}!={'analyzer':49,'planner':17}:raise ValueError('baseline tool count drift')
    if len(dispatch)!=2:raise ValueError('legacy dispatch source changed; review plan')
    matrix=[]
    for r in plan['role_tool_changes']:
        c=consumer['cks_context_'+r['tool']];m=current['cks.context.'+r['tool']]
        if c['wire_name']!=r['wire_name'] or c['input']!=m['input']:raise ValueError('current/consumer metadata drift')
        if r['alias'] in tools[r['role']]:raise ValueError('role baseline already corrected; review plan')
        matrix.append({**r,'present_in_adopted_role':False,'input_metadata_parity':True,
          'current_input_sha256':sha(json.dumps(m['input'],sort_keys=True).encode()),
          'effective_host_permissions':None,'actual_runtime_denial_proven':False,'new_role_receipt':None,'basis_ref':str(BASE/('review-gap-v1/consumer-contract-review.json' if r['role']=='planner' or r['tool']=='review_function' else 'rule-gap-v1/mcp-path-review.json' if r['tool']=='rule_candidates' else 'policy-security-gap-v1/boundary-and-consumer-review.json' if r['tool']=='policy_security' else 'expansion-gap-v1/retrieval-path-review.json'))})
    host=json.loads((ROOT/INPUTS[6]).read_text())
    if host['harness']['sha256']!=sha((ROOT/INPUTS[7]).read_bytes()):raise ValueError('host reference source drift')
    audit={'kind':'source_audited_specification_not_implementation_or_host_execution',
      'adopted_commit':proof['commit'],'role_tool_matrix':matrix,'unique_tools':5,'role_tool_entries':7,
      'baseline_tool_inventories':tools,'planned_tool_counts':{'analyzer':54,'planner':19},
      'legacy_dispatch_alignment_sources':dispatch,
      'consumer_source_sha256':{p:sha(raw) for p,raw in source.items()},
      'planner_conflict':json.loads((ROOT/BASE/'review-gap-v1/consumer-contract-review.json').read_text())['planner_contract_conflict'],
      'host_reference':{'harness':host['harness'],'host':host['host'],
        'source_role_path_coverage_for_new_cases':False,'old_probe_success_is_new_role_receipt':False},
      'implementation_changes':0,'new_role_sessions':0,'new_model_calls':0,'new_mcp_calls':0,
      'validation_cases_executed':0,'new_external_writes':0,'formal_runs':0}
    return plan,audit


def spec_text(plan,audit):
    s=['소비자 도구 노출·Planner 구조화 전달 계약 보완 명세 — 준비26단계',
       '상태: 명세 작성/검증. 구현·새 격리 실행·독립 수용은 수행하지 않았다.',plan['purpose'],
       '기준선 consumer '+plan['baseline']['consumer_commit']+' / KS '+plan['baseline']['knowledge_system_revision'],
       '원본 채택 clone은 읽기 전용. 후속 구현은 별도 소유 checkout/fixture에서 수행한다.',
       '', '필수 변경 파일: '+', '.join(plan['scope']['required_consumer_changes']),
       '보존 runtime: '+', '.join(plan['scope']['preserved_runtime_sources']),
       '', '역할별 추가 도구 — 기존49/17개 보존, 계획54/19개:']
    for r in plan['role_tool_changes']:s.append(r['role']+' / '+r['alias']+' → '+r['wire_name']+' / '+','.join(r['required_by']))
    s+=['','요구사항과 검증 연결:']
    for r in plan['requirements']:s+=['',r['id']+' '+r['title'],r['required_behavior'],'검증: '+', '.join(r['test_ids'])]
    s+=['','완료 구분:',
      '이번 단계는 source/요구사항/테스트 연결·문서/장부 검증 완료만 의미한다.',
      '구현은 P01~P08 및22사례의 실제 증거·정리 확인 후 candidate readiness로만 판정한다.',
      'blocked/inconclusive·renderer 없음·수동 client 성공을 실제 역할 성공으로 대체하지 않는다.',
      '검토자/최종 수용 담당자 미정, F-01~F-08 pending, freeze null, 실행 허용 false, 정식0/9.',
      '독립 required-item/priority gold·LLM 품질·온톨로지 효과 평가는 별도 남은 작업이다.']
    return '\n'.join(s)+'\n'


def validation_text(plan,audit):
    s=['소비자 계약 격리 검증 계획 — 실행하지 않은 계획',
       '기존 host probe는 참고 구현이며 새7역할/도구·리뷰 전달 경로를 검증한 증거가 아니다.',
       '실제 역할 실행이 안 되면 blocked/실패 receipt를 남기며 직접 client로 대체하지 않는다.',
       '','구현·검증 순서:']
    for r in plan['implementation_steps']:
        s+=['',r['id']+' '+r['title']+' [not_started]',
          '선행: '+(','.join(r['depends_on']) or '없음'),r['action'],'검증: '+','.join(r['test_ids'])]
    s+=['','계획된 검증22개 — 모두 not_run:']
    for r in plan['validation_cases']:
        s+=['',r['id']+' '+r['title']+' / '+r['layer'],r['procedure'],'기대: '+r['expected']]
    s+=['','자원/종료 상한 — 정식 평가 프로토콜 동결 아님:',json.dumps(plan['resource_plan'],ensure_ascii=False,indent=2),
        '','필수 원자료 필드:',*plan['receipt_contract']['required_fields'],
        '','판정: 원본 상태와 소유 자식/loopback responder 종료·cleanup을 확인한다.',
        'scripted responder의 출력·usage는 upstream 모델/비용·agent 판단/독립 정답 증거가 아니다.',
        '두 그림 bytes·로컬 rendering과 의미 품질은 별도이며 CFG/runtime·file membership/interprocedural을 구분한다.',
        '후속 코드: 두 role 파일/제한된 테스트·새 owned harness 후보. 지금은 외부 repo/운영/global 상태 변경0건.']
    return '\n'.join(s)+'\n'


def generate():
    plan,audit=inputs()
    return {'implementation-spec.json':{k:plan[k] for k in ['schema_version','kind','date','timezone','status','purpose','baseline','scope','role_tool_changes','requirements','completion']},
      'isolated-validation-plan.json':{k:plan[k] for k in ['kind','status','implementation_steps','validation_cases','resource_plan','receipt_contract','completion']},
      'source-audit.json':audit,'CONSUMER-CONTRACT-SPEC.txt':spec_text(plan,audit),
      'ISOLATED-VALIDATION-PLAN.txt':validation_text(plan,audit)}


def prepare(output):
    if output.exists():raise ValueError('fresh output required')
    artifacts=generate();output.mkdir(parents=True)
    for name,value in artifacts.items():
        if isinstance(value,str):(output/name).write_text(value)
        else:dump(output/name,value)
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    (output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'manifest.json',{'schema_version':1,'kind':'consumer_contract_specification_and_plan_only',
      'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in bindings()},
      'generator_sha256':sha(Path(__file__).read_bytes()),
      'counts':{'requirements':12,'validation_cases':22,'implementation_steps':8,'role_tool_entries':7,
        'unique_tools':5,'implementation_changes':0,'new_runtime_sessions':0,'formal_runs':0},
      'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('Consumer specification12 / tests22 / steps8 / role-tool7; implementation/execution0')


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--output',type=Path,required=True)
    prepare(parser.parse_args().output.resolve())
