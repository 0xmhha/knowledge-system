#!/usr/bin/env python3
"""Freeze actual role receipts, including host admission failures; no false P06 completion."""
import argparse
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
from source_packet import safe,sha as hash_bytes,verify
ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR=BASE/'consumer-local-readiness-v1'
P05=BASE/'consumer-stdio-readiness-v1'
CASES=['P06-analyzer-01','P06-analyzer-02','P06-analyzer-03','P06-analyzer-handoff','P06-planner-delivery','P06-analyzer-revision','P06-planner-reentry','P06-planner-denied','P06-analyzer-unavailable','P06-planner-boundary']
DIAGNOSTIC=CASES[:2]
BLOCKED=['value_dependencies','policy_security']
FULL=['T01','T02','T03','T06','T07','T08','T11','T12','T13','T14','T17','T21']
INPUTS=[PRIOR/'manifest.json',PRIOR/'execution.json',P05/'manifest.json',P05/'execution.json',
 BASE/'consumer-contract-plan-v1/manifest.json',BASE/'consumer-contract-plan-v1/implementation-spec.json',BASE/'consumer-contract-plan-v1/isolated-validation-plan.json',
 Path('scripts/consumer-role-host.py'),Path('scripts/consumer-role-adapter.py'),Path('scripts/operational-host-probe.py'),Path('scripts/source_packet.py'),Path('scripts/prepare-source-review-supplement.py')]
TOP=['invocation.json','settings.json','mcp.json','scripted-actions.json','exit.json','host.stdout.jsonl','host.stderr.txt','host.debug.txt','gateway-requests.json','mcp-frames.jsonl','hook-inputs.jsonl','workspace-before.json','workspace-after.json','action-resolutions.jsonl']


def sha(value):return hash_bytes(value.read_bytes()if isinstance(value,Path)else value)

def require(v,s):
    if not v:raise ValueError(s)
def dump(p,v):p.write_text(json.dumps(v,ensure_ascii=False,indent=2,allow_nan=False)+'\n')
def load(name):
    s=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
def read(p):return json.loads(p.read_text())
def lines(p):return [json.loads(l)for l in p.read_text().splitlines()]
def role_source(role):
    proof=verify(ROOT/PRIOR/'candidate-source',read(ROOT/PRIOR/'execution.json')['candidate_commit'])
    return safe(ROOT/PRIOR/'candidate-source',proof['files']['plugin/agents/'+role+'.md']['artifact']).read_text()
def result_text(row):
    return str(row.get('content',''))


def derive(packet):
    execution=read(packet/'execution.json');prior=read(ROOT/PRIOR/'execution.json');old=load('prepare-consumer-local-readiness.py');old.inputs(ROOT/PRIOR)
    require(execution['candidate_postrun']=={'commit':prior['candidate_commit'],'porcelain':''},'candidate changed')
    require(execution['original_postrun']==prior['original_adopted_after'],'original changed')
    require(execution['maintained_source_changes']==[] and execution['candidate_adopted'] is False,'scope change')
    for k in ['real_model_calls','formal_runs','external_publication','production_writes']:require(execution[k]==0,'acceptance or external scope promotion')
    require(execution['cases']==CASES,'case denominator')
    session=[];all_pairs=set();api_attempts=0;stdio_calls=0;details={}
    for case in CASES:
        p=packet/'runs'/case;inv=read(p/'invocation.json');end=read(p/'exit.json');argv=inv['argv'];role=inv['role']
        require(inv['host_sha256']=='475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3','host pin')
        require(inv['cks_sha256']==read(ROOT/P05/'execution.json')['binary_sha256']['cks'],'CKS pin')
        require('--agent' in argv and argv[argv.index('--agent')+1]=='coding-agent:'+role,'actual requested role')
        for flag in ['--strict-mcp-config','--setting-sources','--permission-prompts','--plugin-dir','--include-hook-events']:require(flag in argv,'host boundary flag')
        require(argv[argv.index('--setting-sources')+1]=='' and argv[argv.index('--permission-prompts')+1]=='none','settings/permission boundary')
        require(not any(x in argv for x in ['--bare','--safe-mode','--dangerously-skip-permissions']),'permission bypass')
        require(inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:') and inv['api_credential_kind']=='constant_fake_local_token','nonlocal provider')
        helper=packet/'helper-versions'/(case+'.py.txt') if case in CASES[:3] else ROOT/'scripts/consumer-role-host.py'
        require(sha(helper)==inv['helper_sha256'],'executed host helper pin')
        require(end['host_exit']==(1 if case=='P06-analyzer-02'else 0) and end['host_reaped'] and not end['forced_host_termination'],'host outcome')
        require(end['responder_closed'] and end['responder_thread_joined'] and end['responder_bind']=='127.0.0.1' and end['elapsed_seconds']<180,'responder/session cleanup')
        require(end['actual_role_requested']==role and end['real_model_calls']==end['formal_runs']==0,'role scope')
        frames=lines(p/'mcp-frames.jsonl');starts=[r for r in frames if r['kind']=='start'];exits=[r for r in frames if r['kind']=='exit']
        require(len(starts)==len(exits)==1 and starts[0]['child_pid']==exits[0]['child_pid'] and exits[0]['exit_code']==0 and exits[0]['child_reaped'] and not exits[0]['forced_kill'] and not exits[0]['terminate_sent'],'owned CKS child cleanup')
        require(starts[0]['binary_sha256']==inv['cks_sha256'],'launched CKS pin')
        req=[json.loads(r['raw'])for r in frames if r['kind']=='request'];resp={json.loads(r['raw']).get('id'):json.loads(r['raw'])for r in frames if r['kind']=='response'}
        calls=[q for q in req if q.get('method')=='tools/call'];require(len(calls)<=64,'tool budget');stdio_calls+=len(calls)
        gateway=read(p/'gateway-requests.json')if(p/'gateway-requests.json').exists()else[]
        actual=[r for r in gateway if 'scripted_response' in r];actions=[b for r in actual for b in r['scripted_response']['content']if b['type']=='tool_use'];api_attempts+=len(actions)
        tool_results={x['tool_use_id']:x for r in actual for m in r['body'].get('messages',[])if m['role']=='user'for x in m.get('content',[])if isinstance(x,dict)and x.get('type')=='tool_result'}
        events=lines(p/'host.stdout.jsonl');hooks=lines(p/'hook-inputs.jsonl')
        if case not in DIAGNOSTIC:
            require(argv[-2]=='--' and argv[-1].startswith('KS_HOST_PROBE:'+case),'prompt boundary')
            require(actual and all('KS_HOST_PROBE:'+case in json.dumps(r['body']['messages'])for r in actual),'actual role prompt receipt')
            init=next(e for e in events if e.get('subtype')=='init');require(init['claude_code_version']=='2.1.300','actual host version')
            plugin=next(x for x in init['plugins']if x['name']=='coding-agent');require(plugin['source']=='coding-agent@inline' and plugin['path']==argv[argv.index('--plugin-dir')+1],'plugin source')
            require(init['mcp_servers']==[{'name':'plugin_coding-agent_cks','status':'connected','source':'dynamic'}],'effective MCP server')
            body=role_source(role).split('---',2)[2].strip().replace('${CLAUDE_PLUGIN_ROOT}',plugin['path'])
            for r in actual:
                system=r['body']['system'];text='\n'.join(x['text']for x in system if x.get('type')=='text')if isinstance(system,list)else system
                require(text.endswith(body),'actual role body differs from pinned source')
            tools=[t['name']for t in actual[0]['body']['tools']];require(len(tools)==(23 if role=='analyzer' else 18 if case=='P06-planner-denied' else 19),'effective role tool count')
            role_tools=set(old.tools(role_source(role).encode()));wire_aliases={'mcp__plugin_coding-agent_cks__'+q['params']['name'].replace('.','_')for q in calls}
            expected_tools=role_tools&set(init['tools']);require(set(tools)==expected_tools==set(init['tools']),'actual API/init/role inventory')
            require(wire_aliases<=set(tools),'non-exposed tool executed')
            listing=next(q for q in req if q.get('method')=='tools/list');registered=resp[listing['id']]['result']['tools'];require(len(registered)==33,'registered tool denominator')
            debug=(p/'host.debug.txt').read_text()
            for t in BLOCKED:
                require('cks.context.'+t in {x['name']for x in registered},'registered tool missing')
                require('Skipping tool "cks.context.'+t+'": its input schema uses top-level oneOf' in debug,'host schema failure cause')
            for q in calls:
                alias='mcp__plugin_coding-agent_cks__'+q['params']['name'].replace('.','_')
                chosen=[a for a in actions if a['name']==alias and a['input']==q['params']['arguments']];require(len(chosen)==1,'alias/wire request association')
                reply=resp[q['id']]['result'];require(not reply.get('isError'),'unexpected CKS envelope error')
                value=reply.get('structuredContent') or json.loads(reply['content'][0]['text']);tool_result=tool_results[chosen[0]['id']]
                require(not tool_result.get('is_error') and json.JSONDecoder().raw_decode(tool_result['content'])[0]==value,'actual role result differs from stdio')
                if case!='P06-analyzer-unavailable':all_pairs.add((role,q['params']['name']))
            require(any(e['payload'].get('hook_event_name')=='SessionStart'for e in hooks),'actual SessionStart hook')
        details[case]={'path':p,'invocation':inv,'end':end,'calls':calls,'responses':resp,'actions':actions,'tool_results':tool_results,'events':events,'hooks':hooks}
        session.append({'case':case,'role':role,'host_exit':end['host_exit'],'stdio_tool_requests':len(calls),'API_tool_attempts':len(actions),'kind':'adapter_diagnostic'if case in DIAGNOSTIC else'actual_role_local_scripted_response','all_owned_children_reaped':True})
    expected={('analyzer','cks.context.'+t)for t in ['rule_candidates','value_dependency_expansion','review_function']}|{('planner','cks.context.'+t)for t in ['rule_candidates','review_function']}
    require(all_pairs==expected,'required successful role/tool pairs')
    observed=details['P06-analyzer-03']
    require(len(observed['actions'])==5 and len(observed['calls'])==3,'five attempted vs three observed Analyzer calls')
    for index,t in [(0,'value_dependencies'),(3,'policy_security')]:
        result=observed['tool_results']['fixture_tool_'+str(index)];require(result['is_error'] and 'No such tool available:' in result['content'] and t in result['content'],'host omission not failed')
    den=details['P06-planner-denied'];require(not den['calls'] and den['tool_results']['fixture_tool_0']['is_error'],'configured role denial bypassed')
    require(read(den['path']/'settings.json')['permissions']['deny']==['mcp__plugin_coding-agent_cks__cks_context_rule_candidates'],'explicit deny control')
    unavailable=details['P06-analyzer-unavailable'];q=unavailable['calls'][0];v=unavailable['responses'][q['id']]['result']['structuredContent'];require(v['status']=='unavailable' and v['analysis'] is None,'backend unavailable hidden')
    for case in ['P06-planner-denied','P06-analyzer-unavailable']:
        p=details[case]['path'];require(read(p/'workspace-before.json')==read(p/'workspace-after.json'),'failure changed delivery state')
    controls={'P06-planner-delivery':{4:'completion requires bound delivered review report',5:'cannot bypass review delivery by changing ticket type',6:'frozen review identity changed outside analysis'},'P06-planner-reentry':{2:'frozen review identity changed outside analysis'},'P06-planner-boundary':{1:'cannot bypass review delivery by changing ticket type',3:'review delivery report differs from typed artifact'}}
    for case,expected_errors in controls.items():
        d=details[case]
        for index,why in expected_errors.items():
            result=d['tool_results']['fixture_tool_'+str(index)];require(result['is_error'] and 'Structured review delivery blocked: '+why in result['content'],'actual Write/Edit guard missing')
            require(any(e.get('subtype')=='hook_response' and 'Structured review delivery blocked: '+why in e.get('output','') for e in d['events']),'actual plugin hook result missing')
    proof=verify(ROOT/PRIOR/'candidate-source',prior['candidate_commit']);raw=safe(ROOT/PRIOR/'candidate-source',proof['files']['plugin/scripts/deep_review.py']['artifact']).read_bytes();namespace={'__name__':'pinned_deep_review'};exec(compile(raw,'<pinned verifier>','exec'),namespace)
    states={};reports={}
    for case in ['P06-analyzer-handoff','P06-planner-delivery','P06-analyzer-revision','P06-planner-reentry','P06-planner-boundary']:
        p=details[case]['path']/'workspace-after';report=read(p/'deep-review.json');bundle=read(p/'deep-review-evidence.json');state=read(p/'state.json')
        namespace['validate'](report,bundle);require((p/'review-report.md.txt').read_text()==namespace['markdown'](report),'report/typed content')
        require((p/'sequence.mmd').read_text()==namespace['sequence_diagram'](report)+'\n'and(p/'modules.mmd').read_text()==namespace['module_diagram'](report)+'\n','diagram typed bytes')
        require(state['states']['ANALYSIS']['deep_review_sha256']==sha(p/'deep-review.json'),'ANALYSIS byte hash')
        if state['current_state']=='COMPLETED':require(state['states']['PLANNING']['status']=='completed'and state['states']['PLANNING']['review_report_sha256']==sha(p/'review-report.md.txt'),'PLANNING byte hash')
        require(report['independent_acceptance']=='not_recorded'and report['structural_review']['snapshot_association']=='identity_only_unverified_snapshot','acceptance or snapshot promotion')
        states[case]=state;reports[case]=report
    require([states[c]['current_state']for c in states]==['PLANNING','ANALYSIS','PLANNING','COMPLETED','COMPLETED'],'actual dispatch/return state sequence')
    a=states['P06-analyzer-handoff']['states']['ANALYSIS']['deep_review_sha256'];b=states['P06-analyzer-revision']['states']['ANALYSIS']['deep_review_sha256'];require(a!=b,'revision generation unchanged')
    require(reports['P06-analyzer-handoff']['computed']==reports['P06-analyzer-revision']['computed'],'scripted reasoning revision altered computation')
    original_bundle=read(details['P06-analyzer-handoff']['path']/'workspace-after/deep-review-evidence.json')
    for key,tool in [('rules','cks.context.rule_candidates'),('function_review','cks.context.review_function')]:
        q=next(q for q in details['P06-analyzer-handoff']['calls']if q['params']['name']==tool);v=details['P06-analyzer-handoff']['responses'][q['id']]['result']['structuredContent'];require(original_bundle[key]=={'tool':tool,'arguments':q['params']['arguments'],'result':v},'typed bundle not actual Analyzer frames')
    boundary=details['P06-planner-boundary']['path']/'workspace-after';bad=read(boundary/'shell-tamper-receipt.json');good=read(boundary/'shell-restore-receipt.json')
    require(bad['exit_code']==2 and bad['delivery_allowed']is False and good['exit_code']==0 and good['delivery_allowed']is True,'shell CLI failure/restore boundary')
    require(len(session)==10 and api_attempts==35 and stdio_calls==8,'original request denominator')
    cases=[{'id':f'T{i:02}','full_case_status':'pass'if f'T{i:02}'in FULL else'fail'if i in [5,19,20]else'not_run','actual_role_quality':'not_evaluated'}for i in range(1,23)]
    result={'kind':'P06_actual_role_partial_host_schema_admission_failure_not_complete','P06_complete':False,'candidate_adopted':False,
      'role_tool_pairs_required':7,'role_tool_pairs_succeeded':5,'successful_pairs':[list(x)for x in sorted(all_pairs)],'failed_pairs':[['analyzer','cks.context.'+t]for t in BLOCKED],
      'registered_CKS_tools':33,'effective_analyzer_tools':23,'effective_planner_tools':19,'configured_denied_planner_tools':18,
      'host_sessions':10,'adapter_diagnostic_sessions':2,'API_tool_attempts':api_attempts,'stdio_tool_requests':stdio_calls,'owned_CKS_children_reaped':10,
      'actual_Write_Edit_guard_refusals':6,'sessions':session,'two_review_generations_validated':True,'actual_role_reasoning_quality':'not_evaluated','reasoning_generator':'local_scripted_adapter_not_real_model',
      'full_plan_cases_passed':FULL,'full_plan_cases_failed':['T05','T19','T20'],'full_plan_cases_pending':7,'full_plan_cases_not_passed':10,'planned_case_progress':cases,
      'formal_completed':0,'formal_total':9,'independent_acceptance':'not_recorded','real_model_calls':0,'environment_network_security_sandbox_proven':False,
      'next':'versioned top-level oneOf host compatibility specification/candidate; preserve runtime exact-selector validation; rerun all 7 pairs before P06 completion; then P07/P08'}
    text=['P06 실제 역할 연결·전달/거부 관측 — 준비29단계, P06 미완료',
      '실제 host2.1.300·격리 후보af4ff8b1·고정 CKS·소유 fixture와 로컬 scripted 응답기만 사용.',
      '등록 CKS33개와 별개로 host가 최상위 oneOf의 value_dependencies/policy_security2개를 제외한다. 실제 tool_use도 No such tool available로 실패.',
      '요구7쌍 중 성공5쌍: Analyzer rules/expansion/review + Planner review/rules. 누락2쌍을 성공으로 세지 않는다.',
      'Analyzer→Planner typed5산출물/6절·CLI·두 hash 전달, ANALYSIS 복귀→추론만 변경→재생성→Planner 재진입의2세대 검증.',
      '실제 Write/Edit hook 거부6건: report hash/type/ANALYSIS hash/이전 세대 hash/형식 우회/shell 변조 후 전달. shell 실패 CLI2→복구 CLI0 보존.',
      '명시적 Planner tool deny는 host 유효 도구에서 제외, CKS 호출0. backend 미설정 unavailable은 기존 전달 상태를 바꾸지 않음.',
      '전체 host10세션(API 도구 시도35/실제 CKS8요청), 초기 adapter 진단2세션 포함. host 실패1건·노출 실패2건/명시적거부/기타 대조를 분모에서 숨기지 않음.',
      '소유 host10개 reap·CKS10개 EOF exit0/reap·responder10개 닫힘. host 취소/강제종료·renderer 검증(P07)은 별도 pending.',
      '22계획사례: 전체 통과12, 실패3(T05/T19/T20), 나머지7 pending. P06 전체 완료·운영 후보 준비 완료로 표시하지 않음.',
      '판정 문안은 scripted adapter가 만들었다. 실제 모델 추론·의미 정답/누락0·독립 품질/온톨로지 효과 수용은 미평가.',
      '원본/격리 후보·backend schema/consumer contract를 이번에 변경하지 않음. 정식 수용0/9·담당자미정·freeze null·실행false 유지.',
      '다음: COMPATIBILITY-FOLLOWUP.txt의 새 호환 명세/별도 후보에서 runtime 입력 거부를 보존하며 metadata를 보완하고 P06 재검증. 이후 P07→P08.']
    return {'role-readiness.json':result,'ROLE-READINESS-REPORT.txt':'\n'.join(text)+'\n'}


FOLLOWUP='''실제 host 도구 admission 호환성 후속 명세 초안 — 미구현
문제: tools/list33개·소비자 mirror가 일치해도 host2.1.300은 최상위 oneOf를 가진2도구를 모델 tool 목록에서 제외한다.
근거: analyzer-03 host.debug.txt의 두 Skipping tool 진단, gateway의 두 No such tool available, stdio에 해당 호출0.
동결된 P01~P05·consumer 계약·기존 binary/원자료는 바꾸지 않는다. 현재 범위는 backend_schema_or_permission_model_change=false이다.
권고 후보: 새 버전의 top-level tool metadata에서 host가 수용하는 object/properties 형태를 제공하고 정확히 한 selector 계약은 설명·기존 runtime parser가 계속 강제한다. 이는 metadata 표현 변경이며 무변경으로 포장하지 않는다.
필요 변경의 예상 범위: CKS/CKG 도구 schema 선언·golden metadata와 consumer mirror·schema 회귀. 실제 query/ACL/출처 계산 및 permission 우회는 금지.
첫 구현은 별도 소유 CKS/consumer checkout·commit·binary에서 진행한다. 현재 ROOT의 schema/harness와 동결 input_binding을 덮어쓰지 않는다. 새 버전 guard/실행기와 기존 기록 source 문맥의 guard를 구분하며 과거 hash를 새 값으로 바꿔 통과시키지 않는다.
수용 기준:
1. object metadata가 host에서 두 도구 모두 노출됨. Analyzer5/Planner2의7쌍 실제 호출·응답이 모두 확인됨.
2. node/canonical 정상 요청 및 둘다/둘다없음/null/잘못된 타입/unknown key/중복 JSON/상한 초과가 기존 runtime과 동등하게 처리됨.
3. commit/source/snapshot·private/ACL·bounded 상태와 수동/자동 provenance 계약 보존.
4. 새 schema와 consumer mirror 정합·잘못된/오래된 schema 거부. 필요한 별도 후보·새 binary pin 및 원자료 보존.
5. Go 변경이 생기면 build/test/race/fmt/vet/lint 및 관련 회귀 통과 후 별도 서명 커밋. 기존 동결 packet 덮어쓰기 금지.
6. 실패했던 T05/T19/T20을 새 실제 역할 원자료로 재평가. 우회 proxy·직접 client로 host 성공을 대체하지 않음.
7. P06 전체 충족 후 P07 renderer/host취소·정리, P08 최종 준비 판정. 독립 gold/품질·온톨로지 효과 수용은 별도.
현재 상태: 원인 및 보완 명세 초안만 작성. 구현·새 schema 승인/채택·P06 완료를 주장하지 않는다.
'''


def prepare(owned,output):
    require(not output.exists(),'fresh output required');output.mkdir(parents=True);(output/'runs').mkdir()
    inventory={}
    for case in CASES:
        source=owned/case;dest=output/'runs'/case;dest.mkdir()
        for name in TOP:
            if(source/name).exists():shutil.copyfile(source/name,dest/name)
        if(source/'workspace-after').exists():
            saved=dest/'workspace-after';saved.mkdir()
            for f in (source/'workspace-after').iterdir():
                if f.is_file():shutil.copyfile(f,saved/(f.name+'.txt' if f.suffix=='.md'else f.name))
        inventory[case]=sorted(p.relative_to(dest).as_posix()for p in dest.rglob('*')if p.is_file())
    shutil.copytree(owned/'helper-versions',output/'helper-versions')
    shutil.copyfile('/private/tmp/ks-run-P06-cases.py',output/'case-driver.py.txt')
    candidate=Path('/private/tmp/ks-consumer-contract-candidate-20261017');original=Path('/private/tmp/ks-isolated-host-20261015/coding-agent')
    def state(p):return {'commit':subprocess.check_output(['git','-C',str(p),'rev-parse','HEAD'],text=True).strip(),'porcelain':subprocess.check_output(['git','-C',str(p),'status','--porcelain'],text=True)}
    changes=subprocess.check_output(['git','diff','HEAD','--','cmd','internal','pkg','go.mod','go.sum'],text=True)
    dump(output/'execution.json',{'cases':CASES,'candidate_postrun':state(candidate),'original_postrun':state(original),'source_pin_check_time':'postrun_with_actual_role_body_compared_to_committed_source','candidate_adopted':False,'maintained_source_changes':[]if not changes else[changes],'real_model_calls':0,'formal_runs':0,'external_publication':0,'production_writes':0,'run_inventory':inventory,'adapter_source_scope':'current build/revise functions preserved; shell controls appended before boundary session; exact host helper versions retained'})
    dump(output/'diagnosis.json',{'pre_host_bind_failure':'sandbox PermissionError binding owned localhost responder; no host started; rerun with authorized local permission','initial_adapter_diagnostics':{'P06-analyzer-01':'inherited stdin and variadic tools argument consumed prompt; actual role loaded but no scripted tool actions','P06-analyzer-02':'stdin closed; variadic tools argument consumed prompt; CLI exit1 without API call'},'primary_missing_tools':'actual pinned host rejects top-level oneOf; no proxy schema rewrite performed'})
    for name,value in derive(output).items():
        if isinstance(value,str):(output/name).write_text(value)
        else:dump(output/name,value)
    (output/'COMPATIBILITY-FOLLOWUP.txt').write_text(FOLLOWUP);(output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'actual_role_partial_P06_host_admission_failure_not_acceptance','input_bindings':{str(p):sha((ROOT/p).read_bytes())for p in INPUTS},'generator_sha256':sha(Path(__file__).read_bytes()),'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes())for p in sorted(output.rglob('*'))if p.is_file()}})
    print('Actual role receipts frozen: 5/7 pairs; P06 incomplete, 2 host schema failures')


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--owned',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();prepare(a.owned.resolve(),a.output.resolve())
