#!/usr/bin/env python3
"""Freeze isolated host-compatible candidates and actual role receipts, without adoption."""
import argparse
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
from source_packet import safe,sha as hash_bytes,verify,SourceCapture
ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR=BASE/'consumer-local-readiness-v1'
P05=BASE/'consumer-stdio-readiness-v1'
CASES=['P06-analyzer-03','P06-analyzer-handoff','P06-planner-delivery','P06-analyzer-revision','P06-planner-reentry','P06-planner-denied','P06-analyzer-unavailable','P06-planner-boundary']
DIAGNOSTIC=[]
FULL=['T01','T02','T03','T05','T06','T07','T08','T11','T12','T13','T14','T17','T19','T20','T21']
INPUTS=[BASE/'consumer-role-readiness-v1/manifest.json',P05/'manifest.json',PRIOR/'manifest.json',
 Path('scripts/fixtures/host-schema-compatibility-plan.json'),Path('scripts/consumer-compatible-host.py'),Path('scripts/run-compatible-role-cases.py'),Path('scripts/run-host-schema-matrix.py'),Path('scripts/consumer-role-adapter.py'),Path('scripts/operational-host-probe.py'),Path('scripts/source_packet.py'),Path('scripts/prepare-source-review-supplement.py')]
TOP=['invocation.json','settings.json','mcp.json','scripted-actions.json','exit.json','host.stdout.jsonl','host.stderr.txt','host.debug.txt','gateway-requests.json','mcp-frames.jsonl','hook-inputs.jsonl','workspace-before.json','workspace-after.json','action-resolutions.jsonl']


def sha(value):return hash_bytes(value.read_bytes()if isinstance(value,Path)else value)

def require(v,s):
    if not v:raise ValueError(s)
def dump(p,v):p.write_text(json.dumps(v,ensure_ascii=False,indent=2,allow_nan=False)+'\n')
def load(name):
    s=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
def read(p):return json.loads(p.read_text())
def lines(p):return [json.loads(l)for l in p.read_text().splitlines()]
def derive(packet):
    execution=read(packet/'execution.json');prior=read(ROOT/PRIOR/'execution.json');old=load('prepare-consumer-local-readiness.py');old.inputs(ROOT/PRIOR)
    require(execution['consumer_postrun']=={'commit':execution['consumer_commit'],'porcelain':''},'candidate changed')
    require(execution['original_postrun']==prior['original_adopted_after'],'original changed')
    require(execution['maintained_source_changes']==[] and execution['candidate_adopted'] is False,'scope change')
    for k in ['real_model_calls','formal_runs','external_publication','production_writes']:require(execution[k]==0,'acceptance or external scope promotion')
    require(execution['cases']==CASES,'case denominator')
    session=[];all_pairs=set();api_attempts=0;stdio_calls=0;details={}
    for case in CASES:
        p=packet/'runs'/case;inv=read(p/'invocation.json');end=read(p/'exit.json');argv=inv['argv'];role=inv['role']
        require(inv['host_sha256']=='475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3','host pin')
        require(inv['cks_sha256']==read(packet/'binary-pins.json')['cks_sha256'],'CKS pin')
        require('--agent' in argv and argv[argv.index('--agent')+1]=='coding-agent:'+role,'actual requested role')
        for flag in ['--strict-mcp-config','--setting-sources','--permission-prompts','--plugin-dir','--include-hook-events']:require(flag in argv,'host boundary flag')
        require(argv[argv.index('--setting-sources')+1]=='' and argv[argv.index('--permission-prompts')+1]=='none','settings/permission boundary')
        require(not any(x in argv for x in ['--bare','--safe-mode','--dangerously-skip-permissions']),'permission bypass')
        require(inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:') and inv['api_credential_kind']=='constant_fake_local_token','nonlocal provider')
        helper=ROOT/'scripts/consumer-compatible-host.py'
        require(sha(helper)==inv['helper_sha256'],'executed host helper pin')
        require(end['host_exit']==0 and end['host_reaped'] and not end['forced_host_termination'],'host outcome')
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
            body=role_source_new(packet,role).split('---',2)[2].strip().replace('${CLAUDE_PLUGIN_ROOT}',plugin['path'])
            for r in actual:
                system=r['body']['system'];text='\n'.join(x['text']for x in system if x.get('type')=='text')if isinstance(system,list)else system
                require(text.endswith(body),'actual role body differs from pinned source')
            tools=[t['name']for t in actual[0]['body']['tools']];require(len(tools)==(25 if role=='analyzer' else 18 if case=='P06-planner-denied' else 19),'effective role tool count')
            role_tools=set(old.tools(role_source_new(packet,role).encode()));wire_aliases={'mcp__plugin_coding-agent_cks__'+q['params']['name'].replace('.','_')for q in calls}
            expected_tools=role_tools&set(init['tools']);require(set(tools)==expected_tools==set(init['tools']),'actual API/init/role inventory')
            require(wire_aliases<=set(tools),'non-exposed tool executed')
            listing=next(q for q in req if q.get('method')=='tools/list');registered=resp[listing['id']]['result']['tools'];require(len(registered)==33,'registered tool denominator')
            debug=(p/'host.debug.txt').read_text()
            for t in ['value_dependencies','policy_security']:
                require('Skipping tool "cks.context.'+t+'": its input schema' not in debug,'host still rejects metadata')
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
    expected={('analyzer','cks.context.'+t)for t in ['value_dependencies','rule_candidates','value_dependency_expansion','policy_security','review_function']}|{('planner','cks.context.'+t)for t in ['rule_candidates','review_function']}
    require(all_pairs==expected,'required successful role/tool pairs')
    observed=details['P06-analyzer-03']
    require(len(observed['actions'])==len(observed['calls'])==9,'nine Analyzer calls required')
    outputs=[x['result']['structuredContent'] for q in observed['calls'] for x in [observed['responses'][q['id']]]]
    require(outputs[0]['snapshot_sha256']==outputs[1]['snapshot_sha256']==outputs[2]['graph']['snapshot_sha256'],'same snapshot baseline/rules/expansion')
    require(outputs[3]['status']=='annotations_found' and outputs[5]['status']=='no_annotations' and outputs[6]['status']=='budget_exhausted' and outputs[7]['graph']['status']=='budget_exhausted' and outputs[8]['status']=='not_found','actual role provenance/untagged/caps/missing paths')
    require({x['kind']for x in outputs[3]['annotations']}=={'Policy','SecurityPattern'},'actual policy/security annotations')
    for x in outputs[3]['annotations']:
        require(x['provenance']['origin']=='operator_curated_metadata' and x['provenance']['identity_trust']=='not_authenticated','manual provenance promoted')
        for key in ['source_sha256','source_line','version','reviewer','reviewed_at','approval']:require(x['provenance'][key]['status']=='unknown','unknown source metadata promoted')
    require(outputs[3]['safety_verdict']==outputs[5]['safety_verdict']=='unknown' and outputs[3]['automatic_rules']=='separate_opt_in_rule_candidates_not_executed','annotation promoted to safety/automatic result')
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
    require(len(session)==8 and api_attempts==39 and stdio_calls==14,'original request denominator')
    cases=[{'id':f'T{i:02}','full_case_status':'pass'if f'T{i:02}'in FULL else'not_run','actual_role_quality':'not_evaluated'}for i in range(1,23)]
    result={'kind':'P06_isolated_host_compatible_actual_role_not_adopted','P06_complete':True,'candidate_adopted':False,
      'role_tool_pairs_required':7,'role_tool_pairs_succeeded':7,'successful_pairs':[list(x)for x in sorted(all_pairs)],'failed_pairs':[],
      'registered_CKS_tools':33,'effective_analyzer_tools':25,'effective_planner_tools':19,'configured_denied_planner_tools':18,
      'host_sessions':8,'API_tool_attempts':api_attempts,'stdio_tool_requests':stdio_calls,'owned_CKS_children_reaped':8,
      'actual_Write_Edit_guard_refusals':6,'sessions':session,'two_review_generations_validated':True,'actual_role_reasoning_quality':'not_evaluated','reasoning_generator':'local_scripted_adapter_not_real_model',
      'full_plan_cases_passed':FULL,'full_plan_cases_failed':[],'full_plan_cases_pending':7,'planned_case_progress':cases,
      'formal_completed':0,'formal_total':9,'independent_acceptance':'not_recorded','real_model_calls':0,'environment_network_security_sandbox_proven':False,
      'next':'P07 renderer/cancellation; remaining T04/T09/T10/T16 integrations; then P08 readiness, without formal acceptance promotion'}
    report=['준비30단계: 격리 후보 host 호환 보완 및 P06 실제 역할 재검증',
      '두 최상위 oneOf 제거 및 공유 중첩 function metadata 동기화: CKS5파일/consumer mirror2파일의 별도 서명 커밋. 현재 ROOT runtime/schema와 기존 후보는 보존.',
      '실제 host2.1.300 Analyzer25/Planner19 도구, 요청한7쌍 모두 실제 stdio 응답 수신. proxy schema rewrite 없음.',
      '8개 host 세션, API39도구 시도/CKS14호출, 모든 host·CKS reap 및 로컬 responder 닫힘.',
      '같은 snapshot baseline/rules/expansion, 수동 정책/보안 주석과 별도 자동 규칙·untagged·missing·상한 결과 및 실제 전달/거부6건·두 세대 hash 재검증.',
      '이전 binary와 새 binary에47개씩94개 native stdio 요청, elapsed_micros만 제외한 runtime 결과 동등. strict selector/중복/null/type/상한/pin 거부 보존.',
      '계획22사례: 통과15/실패0/미완료7. P06 격리 시험 완료이며 후보 채택·전체 준비 완료가 아님.',
      '다음 P07 renderer/취소·정리, 남은 T04/T09/T10/T16, 이후 P08. 추론은 로컬 scripted 응답으로 모델 품질·독립 gold·온톨로지 효과 수용은 미평가.',
      '정식0/9·담당자미정·freeze null·execution_allowed false·실제 모델 비용0 유지.']
    return {'role-readiness.json':result,'ROLE-READINESS-REPORT.txt':'\n'.join(report)+'\n'}


def role_source_new(packet,role):
    proof=verify(packet/'consumer-source',read(packet/'execution.json')['consumer_commit'])
    return safe(packet/'consumer-source',proof['files']['plugin/agents/'+role+'.md']['artifact']).read_text()

CKS_BASE='5b39c413c4b7e6cf0a8fe51ce4c1567ae8a1ee7b'
CONSUMER_BASE='af4ff8b1b7778cb8dd783af684ecefd01b76283c'
CKS_CHANGED=['internal/system/mcp/annotations.go','internal/system/mcp/host_selector_contract_test.go','internal/system/mcp/testdata/agent-mcp.schema.json','pkg/graph/mcphandlers/valueflow.go','pkg/graph/mcphandlers/valueflow_test.go']
CONSUMER_CHANGED=['contract/agent-mcp.schema.json','plugin/contracts/cks-tools.schema.json']
RETAINED=['go.mod','go.sum','pkg/graph/valueflow/query.go','pkg/graph/valueflow/rules.go','pkg/graph/valueflow/expand.go','pkg/graph/annotations/query.go','internal/system/mcp/valueflow.go']
CONSUMER_RETAINED=['plugin/agents/analyzer.md','plugin/agents/planner.md','plugin/scripts/deep_review.py','plugin/scripts/suspect_labels.py','plugin/hooks/hooks.json']


def git(repo,*args):return subprocess.check_output(['git','-C',str(repo),*args])
def state(repo):return {'commit':git(repo,'rev-parse','HEAD').decode().strip(),'porcelain':git(repo,'status','--porcelain').decode()}


def preservation():
    import io,tarfile
    roots=sorted(str(p.relative_to(ROOT).parent)for p in (ROOT/BASE).glob('*-v1/manifest.json'))
    roots=[p for p in roots if not p.endswith('/host-schema-compatibility-v1')]
    require(len(roots)==29,'prior frozen packet denominator')
    retained=['docs/REMAINING-WORK.md','docs/spec-driven/CURRENT-HANDOFF.md','internal/system/mcp/testdata/agent-mcp.schema.json','scripts/consumer-role-host.py','scripts/prepare-consumer-role-readiness.py','scripts/check-consumer-role-readiness.py','scripts/test_consumer_role_readiness.py']
    result={}
    with tarfile.open(fileobj=io.BytesIO(git(ROOT,'archive',CKS_BASE,*roots,*retained)))as archive:
        for item in archive:
            if item.isfile():result[item.name]=sha(archive.extractfile(item).read())
    for path,digest in result.items():require(sha(safe(ROOT,path))==digest,'prior frozen/current bound source changed: '+path)
    return {'base':CKS_BASE,'prior_packets':29,'files':result}


def capture(repo,revision,output,paths):
    c=SourceCapture(repo,output,revision)
    for p in paths:c.source(p)
    dump(output/'git-proof.json',c.proof());return c.proof()


def verify_sources(packet):
    e=read(packet/'execution.json');old=load('prepare-consumer-local-readiness.py')
    for name,base,commit,changed,retained in [('cks',CKS_BASE,e['cks_commit'],CKS_CHANGED,RETAINED),('consumer',CONSUMER_BASE,e['consumer_commit'],CONSUMER_CHANGED,CONSUMER_RETAINED)]:
        a=verify(packet/(name+'-base'),base);b=verify(packet/(name+'-source'),commit)
        require(old.changed_paths(packet/(name+'-base'),a,packet/(name+'-source'),b)==changed,'candidate changed path inventory')
        for path in retained:require(a['files'][path]['sha256']==b['files'][path]['sha256'],'runtime or role changed: '+path)
        require(e[name+'_postrun']=={'commit':commit,'porcelain':''},'candidate not clean')
        # Cryptographic commit objects are retained; verification output is an execution receipt.
        receipt=(packet/'validation'/(name+'-signature.txt')).read_text()
        require('Good signature' in receipt,'signed candidate verification missing')
    import copy
    def source(name,path):
        p=read(packet/name/'git-proof.json');return read(safe(packet/name,p['files'][path]['artifact']))
    old=source('cks-base','internal/system/mcp/testdata/agent-mcp.schema.json');new=source('cks-source','internal/system/mcp/testdata/agent-mcp.schema.json');expected=copy.deepcopy(old)
    for tool in ['value_dependencies','policy_security','rule_candidates','value_dependency_expansion']:
        name='cks.context.'+tool;schema=expected['tools'][name]['input']
        if tool in ['rule_candidates','value_dependency_expansion']:schema=schema['properties']['function']
        require(len(schema.pop('oneOf'))==2,'original exclusive metadata branches')
        if tool in ['value_dependencies','policy_security']:
            description=new['tools'][name]['description'];require('exactly one'in description and 'runtime'in description,'runtime selector documentation')
            expected['tools'][name]['description']=description
    require(expected==new,'native metadata changed beyond four composition inputs/two descriptions')
    for path in CONSUMER_CHANGED:
        mirror=source('consumer-source',path)['providers']['cks']['tools']
        require({v['wire_name']:{'input':v['input'],'description':v['description']}for v in mirror.values()}==new['tools'],'consumer mirror exact native metadata')
    return {'cks_changed_files':CKS_CHANGED,'consumer_changed_files':CONSUMER_CHANGED,'runtime_and_roles_byte_identical':True,'metadata_delta_only_four_composition_inputs_two_descriptions':True}


def verify_matrix(packet):
    m=load('run-host-schema-matrix.py');results={}
    for label in ['old','new']:
        p=packet/'native-matrix'/label;life=read(p/'lifecycle.json');rows=read(p/'messages.json');named={r['label']:r for r in rows}
        require(len(rows)==49 and life['rpc_requests']==49,'native RPC denominator')
        sent=(p/'stdin.jsonl').read_text().splitlines();received=(p/'stdout.jsonl').read_text().splitlines()
        requests=[json.loads(line)for line in sent if 'id'in json.loads(line)]
        require(requests==[r['request']for r in rows] and [json.loads(line)for line in received]==[r['response']for r in rows],'native wire/message association')
        require(life['exit_code']==0 and life['eof_sent'] and life['reader_joined'] and life['child_reaped'] and not life['terminate_sent'] and not life['kill_sent'],'native child cleanup')
        require(life['elapsed_seconds']<900,'native matrix session budget')
        want=read(packet/'binary-pins.json')['cks_sha256']if label=='new'else read(ROOT/P05/'execution.json')['binary_sha256']['cks']
        require(life['binary_sha256_before']==life['binary_sha256_after']==want,'matrix binary pin')
        require(life['tool_requests']==47,'matrix denominator')
        results[label]={}
        for name,tool,raw,expected in m.cases():
            r=named[name];require(r['request']['params']=={'name':'cks.context.'+tool,'arguments':json.loads(raw)},'native request differs')
            require(r['request_raw'].endswith('"arguments":'+raw+'}}\n'),'raw JSON argument bytes')
            require(r['request_raw'].rstrip('\n')in sent,'raw request was not sent')
            v=r['response']
            if expected=='refused':require(m.base.refused(v),'invalid input accepted')
            elif expected!='success':require(m.base.payload(v)['status']==expected,'native status differs')
            else:require(not v.get('error')and not v.get('result',{}).get('isError'),'native positive refused')
            results[label][name]=m.normalize(v)
        require(set(named)=={'initialize','tools'}|set(results[label]),'extra matrix requests')
    require(results['old']==results['new'],'old/new runtime differs')
    tools=named['tools']['response']['result']['tools']
    proof=verify(packet/'consumer-source',read(packet/'execution.json')['consumer_commit']);namespace={'__name__':'pinned_contract'}
    exec(compile(safe(packet/'consumer-source',proof['files']['plugin/scripts/suspect_labels.py']['artifact']).read_bytes(),'<pinned contract>','exec'),namespace)
    contract=safe(packet/'consumer-source',proof['files']['plugin/contracts/cks-tools.schema.json']['artifact'])
    require(namespace['check_contract'](tools,contract)==33,'consumer mirror mismatch')
    import copy,tempfile
    negatives={}
    stale=read(ROOT/PRIOR/'candidate-source'/read(ROOT/PRIOR/'candidate-source/git-proof.json')['files']['plugin/contracts/cks-tools.schema.json']['artifact'])
    with tempfile.TemporaryDirectory(prefix='ks-schema-negative-')as tmp:
        p=Path(tmp)/'contract.json';p.write_text(json.dumps(stale))
        for label,rows,c in [('stale',tools,p),('missing',tools[:-1],contract),('mutated',copy.deepcopy(tools),contract)]:
            if label=='mutated':rows[0]['inputSchema']['additionalProperties']='wrong-type'
            try:namespace['check_contract'](rows,c)
            except ValueError:negatives[label]='rejected'
            else:raise ValueError('schema negative accepted: '+label)
    for t in tools:
        if t['name']in ['cks.context.value_dependencies','cks.context.policy_security']:
            require(t['inputSchema'].get('type')=='object'and t['inputSchema'].get('additionalProperties')is False,'object metadata')
            require(t['annotations']['readOnlyHint']is True,'read-only hint lost')
            require(not any(k in t['inputSchema']for k in ['oneOf','anyOf','allOf']),'host incompatible metadata')
    return {'cases_per_binary':47,'total_native_tool_requests':94,'runtime_results_equal_after_omitting_only_elapsed_micros':True,'actual_schema_mirror_tools':33,'negative_schema_controls':negatives}


def verify_validation(packet):
    import gzip
    summary={}
    for name in ['build','test','race','lint']:
        receipt=read(packet/'validation'/(name+'.receipt.json'));require(receipt['exit_code']==0 and receipt['elapsed_seconds']<(1200 if name=='race'else 900),'native validation failed/budget')
        expected={'build':['go','build','./...'],'test':['go','test','-json','-count=1','-timeout=14m','./...'],'race':['go','test','-race','-json','-count=1','-timeout=18m','./...'],'lint':['make','lint','GOLANGCI_LINT=/Users/kevin/.gvm/pkgsets/go1.26.8/global/bin/golangci-lint']}[name]
        require(receipt['command']==expected,'native validation command changed')
        raw=gzip.decompress((packet/'validation'/(name+'.stdout.txt.gz')).read_bytes())
        require(sha(raw)==receipt['stdout_sha256'],'native validation stdout hash')
        if name in ['test','race']:
            rows=[json.loads(l)for l in raw.splitlines()]
            for kind,key in [(True,'test_results'),(False,'package_results')]:
                counts={k:sum(r.get('Action')==k and bool(r.get('Test'))==kind for r in rows)for k in ['pass','skip','fail']}
                require(counts==receipt[key]and counts['fail']==0,'native validation event accounting')
            require(receipt['test_results']['pass']>3900,'truncated native tests')
        summary[name]=receipt
    require('0 issues.'in gzip.decompress((packet/'validation/lint.stdout.txt.gz').read_bytes()).decode(),'linter receipt')
    for name,count in [('consumer-review',15),('consumer-role',16),('consumer-schema',14)]:
        raw=(packet/'validation'/(name+'.txt')).read_text();require('Ran '+str(count)+' tests'in raw and '\nOK'in raw,'consumer regression receipt')
    restoration=read(packet/'validation/golden-envelope-restoration.json')
    proof=read(packet/'cks-source/git-proof.json');golden=proof['files']['internal/system/mcp/testdata/agent-mcp.schema.json']
    require(restoration['restored_file_sha256']==golden['sha256'] and restoration['executable_tools_unchanged'] and restoration['restored_envelope_matches_base'],'golden documentation restoration altered executable input')
    require(restoration['outer_keys_restored']==['$schema','description','title'],'unexpected golden restoration')
    require(read(packet/'validation/post-envelope.receipt.json')['exit_code']==0,'post-restoration contract validation failed')
    pins=read(packet/'binary-pins.json');build=read(packet/'validation/build-receipt.txt')
    require(len(build)==2 and all(r['exit_code']==0 and r['vcs_modified']is False and r['source_commit']==pins['cks_commit']for r in build),'final clean source build failed')
    for name in ['cks','ckg']:
        info=(packet/'validation'/(name+'-buildinfo.txt')).read_text()
        require('vcs.revision='+pins['cks_commit']in info and 'vcs.modified=false'in info,'binary does not embed clean source pin')
    diagnostics=read(packet/'native-diagnostics.json')
    require(set(diagnostics)=={'test-initial','race-initial','race-14min-initial'},'native timeout attempts omitted')
    for name,r in diagnostics.items():
        raw=gzip.decompress((packet/'validation'/(name+'.stdout.txt.gz')).read_bytes())
        timeout=b'14m0s'if name=='race-14min-initial'else b'10m0s'
        require(sha(raw)==r['stdout_sha256'] and r['exit_code']==1 and b'panic: test timed out after '+timeout in raw,'initial timeout diagnostic lost')
        require(r['test_results']['fail']==0 and r['package_results']['fail']==1,'unexpected diagnostic failure omitted')
    return summary


def prepare(owned,output):
    import gzip
    require(not output.exists(),'fresh output required');output.mkdir(parents=True);(output/'runs').mkdir();(output/'validation').mkdir()
    inventory={}
    for case in CASES:
        source=owned/case;dest=output/'runs'/case;dest.mkdir()
        for name in TOP:
            if(source/name).exists():shutil.copyfile(source/name,dest/name)
        if(source/'workspace-after').exists():
            saved=dest/'workspace-after';saved.mkdir()
            for f in (source/'workspace-after').iterdir():
                if f.is_file():shutil.copyfile(f,saved/(f.name+'.txt'if f.suffix=='.md'else f.name))
        inventory[case]=sorted(p.relative_to(dest).as_posix()for p in dest.rglob('*')if p.is_file())
    cks=Path('/private/tmp/ks-host-schema-cks-20261017');consumer=Path('/private/tmp/ks-host-schema-consumer-20261017')
    c=state(cks);u=state(consumer)
    for name,repo,base,commit,changed,retained in [('cks',cks,CKS_BASE,c['commit'],CKS_CHANGED,RETAINED),('consumer',consumer,CONSUMER_BASE,u['commit'],CONSUMER_CHANGED,CONSUMER_RETAINED)]:
        capture(repo,base,output/(name+'-base'),[p for p in changed if p!='internal/system/mcp/host_selector_contract_test.go']+retained)
        capture(repo,commit,output/(name+'-source'),changed+retained)
        (output/(name+'.patch')).write_bytes(git(repo,'diff','--binary',base,commit))
        shutil.copyfile('/private/tmp/ks-schema-validation/'+name+'-signature.txt',output/'validation'/(name+'-signature.txt'))
    (output/'.gitattributes').write_text('**/*.commit binary\n**/*.tree binary\n*.patch -whitespace\nvalidation/* -whitespace\n')
    for name in ['build','test','race','lint']:
        source=Path('/private/tmp/ks-schema-validation');r=read(source/(name+'.receipt.json'));raw=(source/(name+'.stdout.txt')).read_bytes();r['stdout_sha256']=sha(raw)
        dump(output/'validation'/(name+'.receipt.json'),r);(output/'validation'/(name+'.stdout.txt.gz')).write_bytes(gzip.compress(raw,mtime=0));shutil.copyfile(source/(name+'.stderr.txt'),output/'validation'/(name+'.stderr.txt'))
    for name in ['consumer-review','consumer-role','consumer-schema']:shutil.copyfile('/private/tmp/ks-schema-'+name+'.txt',output/'validation'/(name+'.txt'))
    for name in ['golden-envelope-restoration.json','post-envelope.receipt.json','post-envelope.stdout.txt']:shutil.copyfile('/private/tmp/ks-schema-validation/'+name,output/'validation'/name)
    diagnostics={}
    source=Path('/private/tmp/ks-schema-validation')
    for receipt in sorted(source.glob('*-initial.receipt.json')):
        name=receipt.name.removesuffix('.receipt.json');r=read(receipt);raw=(source/(name+'.stdout.txt')).read_bytes();r['stdout_sha256']=sha(raw);diagnostics[name]=r
        dump(output/'validation'/receipt.name,r);(output/'validation'/(name+'.stdout.txt.gz')).write_bytes(gzip.compress(raw,mtime=0));shutil.copyfile(source/(name+'.stderr.txt'),output/'validation'/(name+'.stderr.txt'))
    dump(output/'native-diagnostics.json',diagnostics)
    dump(output/'preservation.json',preservation())
    for p in Path('/private/tmp/ks-host-schema-runtime-20261017').glob('*.txt'):shutil.copyfile(p,output/'validation'/p.name)
    shutil.copytree(owned/'native-matrix',output/'native-matrix');shutil.copyfile(owned/'binary-pins.json',output/'binary-pins.json')
    original=Path('/private/tmp/ks-isolated-host-20261015/coding-agent');old=Path('/private/tmp/ks-consumer-contract-candidate-20261017')
    changes=git(ROOT,'status','--porcelain','--','cmd','internal','pkg','go.mod','go.sum').decode()
    dump(output/'execution.json',{'cases':CASES,'cks_commit':c['commit'],'consumer_commit':u['commit'],'cks_postrun':c,'consumer_postrun':u,'original_postrun':state(original),'prior_candidate_postrun':state(old),'candidate_adopted':False,'maintained_source_changes':[]if not changes else[changes],'real_model_calls':0,'formal_runs':0,'external_publication':0,'production_writes':0,'run_inventory':inventory,'native_tests':'Go code/executable metadata unchanged during validation; golden documentation envelope restored with receipt and explicit contract rerun; final binary rebuilt from clean signed source'})
    for name,value in derive(output).items():
        if isinstance(value,str):(output/name).write_text(value)
        else:dump(output/name,value)
    dump(output/'source-verification.json',verify_sources(output));dump(output/'matrix-verification.json',verify_matrix(output));dump(output/'validation-verification.json',verify_validation(output))
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT));(output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'manifest.json',{'schema_version':1,'kind':'isolated_host_schema_compatibility_P06_not_adoption_or_acceptance','input_bindings':{str(p):sha(ROOT/p)for p in INPUTS},'generator_sha256':sha(Path(__file__).read_bytes()),'artifacts':{p.relative_to(output).as_posix():sha(p)for p in sorted(output.rglob('*'))if p.is_file()}})
    print('Host compatibility frozen: actual role7/7; plan15/22; formal0/9')


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--owned',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();prepare(a.owned.resolve(),a.output.resolve())
