#!/usr/bin/env python3
"""Owned fixture/index and real CKS stdio receipts; never actual role acceptance."""
import argparse
import copy
from contextlib import closing
import importlib.util
import json
import os
from pathlib import Path
import queue
import shutil
import sqlite3
import subprocess
import threading
import time

from source_packet import safe, sha

ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR=BASE/'consumer-local-readiness-v1'
TOOLS=['value_dependencies','rule_candidates','value_dependency_expansion','policy_security','review_function']
LOCAL=['T01','T02','T08','T11','T12','T17']
INPUTS=[PRIOR/'manifest.json',PRIOR/'execution.json',PRIOR/'build-receipts.json',
 BASE/'consumer-contract-plan-v1/manifest.json',BASE/'consumer-contract-plan-v1/isolated-validation-plan.json',
 Path('internal/system/mcp/testdata/agent-mcp.schema.json'),Path('system/policies/sanitization_rules.yaml'),
 Path('scripts/source_packet.py'),Path('scripts/prepare-source-review-supplement.py')]
ATTRIBUTES=b'**/*.bundle binary\n**/*.db binary\n'
SOURCE='''package fixture
import "os/exec"
type Box struct{ Total int }
func Seed(command string, values []int, strict bool) *exec.Cmd {
 total:=0
 for _, value:=range values { total+=value }
 if strict && total<0 {command="fixed"}
 Helper(total)
 Empty(total)
 return exec.Command(command)
}
func Helper(x int) int {return x+1}
func Empty(x int) int {return x*2}
func Field(box *Box,x int) int {box.Total=x; return box.Total}
func Branch(a,b int,flag bool) int { y:=a; if flag {y=b}; return y }
'''
TEST='''package fixture
import "testing"
func TestFixture(t *testing.T) {
 if Helper(2)!=3 || Empty(3)!=6 || Field(&Box{},4)!=4 || Branch(1,2,true)!=2 {t.Fatal("fixture contract")}
 if Seed("owned-nonexecuted-command",[]int{1,2},false).Path!="owned-nonexecuted-command" {t.Fatal("command construction")}
}
'''
POLICY='''policies:
  - id: owned-policy
    name: Owned review policy
    category: review
    description: Preserve command construction assumptions
    governs: [fixture.Seed]
'''
SECURITY='''security_patterns:
  - id: owned-command-boundary
    name: Manual command boundary
    category: review
    severity: high
    description: Manual hypothesis, not a confirmed vulnerability
    remediation: Review input assumptions before editing
    matches: [fixture.Seed]
  - id: owned-secondary
    name: Secondary manual hypothesis
    severity: low
    description: Manual metadata only
    matches: [fixture.Seed]
'''


def require(ok,message):
    if not ok:raise ValueError(message)


def load(path):
    s=importlib.util.spec_from_file_location(path.replace('-','_'),ROOT/'scripts'/path)
    m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m


def dump(path,value):path.write_text(json.dumps(value,ensure_ascii=False,indent=2,allow_nan=False)+'\n')


def source_files():
    stress='func Stress(x int) int {\n'+''.join(f'v{i} := x+{i}\n' for i in range(180))+'return '+'+'.join(f'v{i}' for i in range(180))+'\n}\n'
    return {'go.mod':'module example.invalid/consumer-stdio-fixture\n\ngo 1.26.9\n',
      'logic.go':SOURCE+stress,'logic_test.go':TEST,'policy.yaml':POLICY,'security.yaml':SECURITY,
      'a/same.go':'package a\nfunc Same(x int) int {return x+1}\n',
      'b/same.go':'package b\nfunc Same(x int) int {return x+2}\n'}


class OwnedClient:
    """Only this Popen child can be stopped; bounded calls, EOF then 5s/5s cleanup."""
    def __init__(self,command,path):
        path.mkdir();self.path=path;self.command=command;self.started=time.monotonic()
        self.id=0;self.calls=0;self.queue=queue.Queue();self.messages=[]
        self.err=(path/'stderr.txt').open('wb');self.out=(path/'stdout.jsonl').open('w');self.ins=(path/'stdin.jsonl').open('w')
        self.proc=subprocess.Popen(command,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=self.err,text=True)
        def read():
            for line in self.proc.stdout:self.out.write(line);self.out.flush();self.queue.put(line)
            self.queue.put(None)
        self.thread=threading.Thread(target=read,daemon=True);self.thread.start()
    def request(self,label,method,params=None):
        require(time.monotonic()-self.started<900,'session wallclock budget')
        if method=='tools/call':require(self.calls<64,'tool request budget');self.calls+=1
        self.id+=1;req={'jsonrpc':'2.0','id':self.id,'method':method}
        if params is not None:req['params']=params
        raw=json.dumps(req)+'\n';self.ins.write(raw);self.ins.flush();self.proc.stdin.write(raw);self.proc.stdin.flush()
        deadline=time.monotonic()+30
        while True:
            line=self.queue.get(timeout=max(.001,deadline-time.monotonic()));require(line is not None,'premature stdio EOF')
            response=json.loads(line)
            if response.get('id')==self.id:break
        self.messages.append({'label':label,'request':req,'response':response});return response
    def tool(self,label,name,args):return self.request(label,'tools/call',{'name':'cks.context.'+name,'arguments':args})
    def initialize(self):
        r=self.request('initialize','initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-consumer-P05','version':'1'}})
        require('result' in r,'initialize failed')
        raw=json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n';self.ins.write(raw);self.ins.flush();self.proc.stdin.write(raw);self.proc.stdin.flush()
    def close(self):
        forced=False;terminated=False
        try:
            self.proc.stdin.close()
            try:self.proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                terminated=True;self.proc.terminate()
                try:self.proc.wait(timeout=5)
                except subprocess.TimeoutExpired:forced=True;self.proc.kill();self.proc.wait(timeout=5)
        finally:
            self.thread.join(2);self.out.close();self.ins.close();self.err.close();self.proc.stdout.close()
            dump(self.path/'messages.json',self.messages)
            dump(self.path/'lifecycle.json',{'command':self.command,'pid':self.proc.pid,'owner':'this_runner_Popen_child',
              'exit_code':self.proc.returncode,'eof_sent':True,'terminate_sent':terminated,'kill_sent':forced,
              'reader_joined':not self.thread.is_alive(),'child_reaped':self.proc.poll() is not None,
              'tool_requests':self.calls,'rpc_requests':self.id,'elapsed_seconds':time.monotonic()-self.started,
              'actual_role_identity':None,'listeners_created':0})


def payload(response):
    require('error' not in response and not response.get('result',{}).get('isError'),'expected successful tool envelope')
    r=response['result'];return r.get('structuredContent') or json.loads(r['content'][0]['text'])


def refused(response):return 'error' in response or response.get('result',{}).get('isError') is True


def contract_module():
    prior=load('prepare-consumer-local-readiness.py');_,_,proof=prior.inputs(ROOT/PRIOR)
    raw=safe(ROOT/PRIOR/'candidate-source',proof['files']['plugin/scripts/suspect_labels.py']['artifact']).read_bytes()
    ns={'__file__':str(ROOT/PRIOR/'candidate-source/sources/plugin/scripts/suspect_labels.py.txt'),'__name__':'frozen_consumer_contract'}
    exec(compile(raw,'<pinned consumer contract>','exec'),ns)
    contract=safe(ROOT/PRIOR/'candidate-source',proof['files']['plugin/contracts/cks-tools.schema.json']['artifact'])
    return ns['check_contract'],contract


def derive(packet):
    """Recheck raw transport, fixture source, index mutations and outcomes, not claimed pass counts."""
    prior=json.loads((ROOT/PRIOR/'execution.json').read_text());r=json.loads((packet/'execution.json').read_text())
    require(r['candidate_commit']==prior['candidate_commit'] and r['candidate_adopted'] is False,'candidate scope')
    for k in ['actual_role_sessions','model_calls','listeners','formal_runs']:require(r[k]==0,'scope promotion')
    require(r['candidate_before']==r['candidate_after'] and r['candidate_before']=={'commit':prior['candidate_commit'],'porcelain':''},'candidate changed')
    require(r['original_before']==r['original_after']==prior['original_adopted_after'],'original changed')
    builds=json.loads((ROOT/PRIOR/'build-receipts.json').read_text())
    require(r['binary_sha256']=={b['name']:b['binary_sha256'] for b in builds},'binary pin')
    require(r['go_source_changes']==[],'maintained Go source drift')
    for path,digest in r['fixture_sha256'].items():require(sha(safe(packet/'fixture-source',path).read_bytes())==digest,'fixture bytes')
    require(set(r['fixture_sha256'])==set(source_files()),'fixture source inventory')
    bundle=subprocess.check_output(['git','bundle','list-heads',str(packet/'fixture.bundle')],text=True).strip()
    require(bundle==r['fixture_commit']+' HEAD','fixture bundle head')
    # Git must bind committed source, not merely a self-consistent JSON receipt.
    with __import__('tempfile').TemporaryDirectory() as tmp:
        subprocess.run(['git','clone','-q','--bare',str(packet/'fixture.bundle'),tmp+'/repo'],check=True,capture_output=True,timeout=30)
        for path in source_files():
            raw=subprocess.check_output(['git','--git-dir',tmp+'/repo','show',r['fixture_commit']+':'+path])
            require(raw==safe(packet/'fixture-source',path).read_bytes(),'fixture commit-to-source binding')
    logs={}
    for session in ['discovery','main','old-index','unavailable']:
        folder=packet/'sessions'/session;rows=json.loads((folder/'messages.json').read_text());life=json.loads((folder/'lifecycle.json').read_text())
        require(life['exit_code']==0 and life['eof_sent'] and life['child_reaped'] and life['reader_joined'] and not life['terminate_sent'] and not life['kill_sent'],'stdio child cleanup')
        require(0<life['pid'] and life['owner']=='this_runner_Popen_child' and life['elapsed_seconds']<900 and life['tool_requests']<=64,'resource receipt')
        require(life['actual_role_identity'] is None and life['listeners_created']==0,'direct client not role')
        stderr=(folder/'stderr.txt').read_text()
        expected_stderr={'discovery':['ckg mcp: stdio server bound to '],
          'main':['cks-mcp: source_root derived from graph manifest: ','cks-mcp: alignment warning: ckv manifest missing — alignment coordinates unavailable'],
          'old-index':['cks-mcp: source_root derived from graph manifest: ','cks-mcp: alignment warning: ckv manifest missing — alignment coordinates unavailable'],
          'unavailable':['cks-mcp: ALIGNMENT FAILED — serving non-serviceable for diagnosis: ckg schema dummy < 1.19 — canonical_id values unpopulated']}[session]
        require(len(stderr.splitlines())==len(expected_stderr) and all(fragment in line for fragment,line in zip(expected_stderr,stderr.splitlines())),'unexpected stdio diagnostic')
        requests=[json.loads(l) for l in (folder/'stdin.jsonl').read_text().splitlines()]
        responses=[json.loads(l) for l in (folder/'stdout.jsonl').read_text().splitlines()]
        require([a['request'] for a in rows]==[q for q in requests if 'id' in q],'request transport binding')
        require([a['response'] for a in rows]==[q for q in responses if 'id' in q],'response transport binding')
        require([a['request']['id'] for a in rows]==list(range(1,len(rows)+1)),'RPC identity order')
        for row in rows:
            result=row['response'].get('result',{})
            texts=result.get('content',[])
            if 'structuredContent' in result and len(texts)==1 and texts[0].get('text','').startswith('{'):
                require(json.loads(texts[0]['text'])==result['structuredContent'],'text/structured envelope disagreement')
        require(life['rpc_requests']==len(rows) and life['tool_requests']==sum(a['request']['method']=='tools/call' for a in rows),'request denominator')
        require(len({a['label'] for a in rows})==len(rows),'duplicate labels')
        logs[session]={a['label']:a for a in rows}
    main=logs['main'];get=lambda key:payload(main[key]['response'])
    tools=main['tools']['response']['result']['tools'];check_contract,contract=contract_module();count=check_contract(tools,contract)
    schema_controls=[]
    for name in ['missing_tool','wrong_namespace','stale_schema']:
        changed=copy.deepcopy(tools)
        if name=='missing_tool':changed=[t for t in changed if t['name']!='cks.context.rule_candidates']
        if name=='wrong_namespace':changed[0]['name']='wrong.'+changed[0]['name']
        if name=='stale_schema':
            t=next(t for t in changed if t['name']=='cks.context.policy_security');t['inputSchema']['properties']['max_annotations']['maximum']=999
        try:check_contract(changed,contract)
        except ValueError:schema_controls.append(name)
        else:raise ValueError('schema mutation accepted')
    require(all('cks.context.'+t in {x['name'] for x in tools} for t in TOOLS),'five tool inventory')
    selected=r['selected'];query=r['query'];base=get('baseline');snapshot=base['snapshot_sha256'];src=base['analysis']['source']
    require(query=={'node_id':selected['id'],'expected_commit':r['fixture_commit'],'expected_source_sha256':sha(selected['source'].encode())},'selected input pins')
    require(src['node_id']==selected['id'] and src['canonical_id']==selected['canonical_id'] and src['file']==selected['file'] and src['commit']==r['fixture_commit'] and src['snapshot_sha256']==snapshot,'computed source identity')
    require(src['sha256']==query['expected_source_sha256'],'computed source hash')
    require(get('canonical')['analysis']['source']==src,'canonical parity')
    rules=get('rules');require(rules['analysis']['source']==src and rules['analysis']['rules']['candidates'],'explicit rules with same analysis')
    require(main['rules']['request']['params']['arguments']=={'rule_pack':'go-local-review@1.0.0','function':{**query,'expected_snapshot_sha256':snapshot}},'explicit rule pack')
    for key in ['nodes','edges','def_use']:
        require(rules['analysis'][key]==base['analysis'][key],'reuse rule value/control analysis')
    require(logs['discovery']['find-Seed']['response']['result']['structuredContent']['nodes']==[selected],'discovery-to-selected binding')
    raw_source=(packet/'fixture-source/logic.go').read_text()
    require(selected['source'] in raw_source,'selected retained source not committed fixture')
    expanded=get('expansion');g=expanded['graph'];require(g['snapshot_sha256']==snapshot and len(g['functions'])>=2,'expansion same retained snapshot')
    require(all(f['result']['snapshot_sha256']==snapshot for f in g['functions']),'mixed expansion snapshot')
    require(all(h['origin']=='retained_call_graph' and h['argument_mapping']=='not_computed' for h in g['hops']),'call hop interpretation')
    require(expanded['curated_flow']['status']=='separate_not_joined','curated flow boundary')
    ann=get('annotations');require(len(ann['annotations'])==3 and ann['safety_verdict']=='unknown' and ann['automatic_rules']=='separate_opt_in_rule_candidates_not_executed','manual annotation boundary')
    require(all(a['provenance']['identity_trust']=='not_authenticated' for a in ann['annotations']),'manual trust')
    empty=get('untagged');require(empty['status']=='no_annotations' and empty['annotations']==[] and empty['safety_verdict']=='unknown','empty not safety')
    structural=get('review');require(structural['seed']['id']==selected['id'] and structural['seed']['file_path']==src['file'],'structural identity')
    require('snapshot_sha256' not in structural,'structural unsupported pin promoted')
    mismatches=[]
    for label,row in main.items():
        if label.startswith('mismatch-'):
            d=payload(row['response']);v=d['graph'] if 'graph' in d else d
            require(v['status']=='source_mismatch','supported pin did not refuse: '+label)
            require(not v.get('analysis') and not v.get('functions') and not v.get('annotations'),'mismatch retained data')
            q=row['request']['params']['arguments'];q=q.get('function',q)
            pin=label.split('-')[-1]
            require(q[pin]=='0'*(40 if pin=='expected_commit' else 64) and q['node_id']==selected['id'],'pin negative request binding')
            mismatches.append(label)
    require(len(mismatches)==12,'four tools times three supported pins')
    invalid=[label for label in main if label.startswith('invalid-')]
    require(len(invalid)==8 and all(refused(main[k]['response']) for k in invalid),'runtime refusal controls')
    require(get('missing')['status']=='not_found' and refused(main['missing-review']['response']) and refused(main['ambiguous-review']['response']),'missing/ambiguous errors')
    require(get('unsupported')['status']=='unsupported','unsupported retained type')
    require(get('partial')['status']=='partial' and any(d['code']=='memory_write_untracked' for d in get('partial')['analysis']['diagnostics']),'partial diagnostics missing')
    caps=[]
    for label in [k for k in main if k.startswith('cap-')]:
        v=get(label);v=v.get('graph',v)
        if label=='cap-annotations':require(v['status']=='partial' and v['more'] and len(v['annotations'])==1,'annotation cap')
        elif label=='cap-review':require(len(v['code']['call_sites'])<=1 and v['code']['call_sites_total']>=1,'structural cap')
        else:require(v['status']=='budget_exhausted','analysis cap hidden: '+label)
        caps.append(label)
    require(set(caps)=={'cap-'+k for k in ['max_functions','max_candidates','max_total_nodes','max_total_edges','max_total_facts','iterations','annotations','review']},'cap matrix')
    for key in ['max_functions','max_candidates','max_total_nodes','max_total_edges','max_total_facts']:
        require(main['cap-'+key]['request']['params']['arguments']['limits']=={key:1},'cap request binding')
    require(get('retained')['analysis']['source']==src and get('retained')['snapshot_sha256']==snapshot,'live source fallback')
    require(get('retained-annotations')['annotations']==ann['annotations'],'live YAML fallback')
    old=logs['old-index']['review']['response'];require(refused(old) and 'requires_reindex' in json.dumps(old),'old index refusal')
    for name in ['value_dependencies','rule_candidates','value_dependency_expansion','policy_security']:
        v=payload(logs['unavailable'][name]['response']);v=v.get('graph',v);require(v['status']=='unavailable','missing backend status')
    timeout_rows=[payload(v['response']) for k,v in main.items() if k.startswith('deadline-')]
    observed=[v for v in timeout_rows if v['status']=='budget_exhausted' and any(w in json.dumps(v).lower() for w in ['deadline','context','time budget'])]
    require(len(timeout_rows)==3,'deadline trial denominator')
    commands=json.loads((packet/'commands.json').read_text())
    require({c['label'] for c in commands}=={'format','git-init','git-add','git-commit','git-head','fixture-race','fixture-vet','fixture-bundle','index'},'command inventory')
    for c in commands:
        require(c['exit_code']==0 and c['elapsed_seconds']<180,'fixture command failure')
        require(c['stdout_sha256']==sha((packet/'command-logs'/f"{c['label']}.stdout.txt").read_bytes()) and c['stderr_sha256']==sha((packet/'command-logs'/f"{c['label']}.stderr.txt").read_bytes()),'command log binding')
    for name in ['index','old-index']:
        require(sha((packet/name/'graph.db').read_bytes())==r['index_sha256'][name],'retained database hash')
        with closing(sqlite3.connect('file:'+str(packet/name/'graph.db')+'?mode=ro&immutable=1',uri=True)) as db:
            exists=db.execute("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='code_edges'").fetchone()[0]
            require(exists==(1 if name=='index' else 0),'owned old index mutation')
            manifest=dict(db.execute('SELECT key,value FROM manifest'))
            require(manifest['src_commit']==r['fixture_commit'],'retained DB commit')
            blob=db.execute('SELECT source FROM blobs WHERE node_id=?',(selected['id'],)).fetchone()[0]
            if isinstance(blob,bytes):blob=blob.decode()
            require(blob==selected['source'],'retained DB source')
            node=db.execute('SELECT canonical_id,file_path,start_byte,start_line FROM nodes WHERE id=?',(selected['id'],)).fetchone()
            require(node==(src['canonical_id'],src['file'],src['start_byte'],src['start_line']),'retained DB coordinates')
            external=json.loads((packet/name/'manifest.json').read_text())
            require(external['src_commit']==r['fixture_commit'] and external['src_root']==manifest['src_root'],'retained manifest binding')
    diagnostics=json.loads((packet/'diagnostics/diagnosis.json').read_text())
    require(diagnostics['initial_failure']=='fixture_yaml_mapping_and_single_candidate_not_cap_exhaustion' and diagnostics['old_index_control_failure']=='uncheckpointed_WAL_ignored_by_immutable_reader','diagnostic explanation')
    diagnostic_tools=0
    for folder in [packet/'diagnostics/initial-run/sessions'/k for k in ['discovery','main','old-index','unavailable']]+[packet/'diagnostics/old-index-uncheckpointed']:
        life=json.loads((folder/'lifecycle.json').read_text())
        require(life['exit_code']==0 and life['child_reaped'] and life['reader_joined'] and not life['terminate_sent'] and not life['kill_sent'],'diagnostic child cleanup')
        diagnostic_tools+=life['tool_requests']
    require(diagnostic_tools==57,'diagnostic request denominator')
    initial={a['label']:a for a in json.loads((packet/'diagnostics/initial-run/sessions/main/messages.json').read_text())}
    require(payload(initial['annotations']['response'])['status']=='no_annotations' and payload(initial['cap-max_candidates']['response'])['graph']['status']=='expanded_bounded','initial fixture diagnostic observations')
    previous=json.loads((packet/'diagnostics/old-index-uncheckpointed/messages.json').read_text())
    require(not refused(previous[-1]['response']) and payload(previous[-1]['response'])['seed']['id']==selected['id'],'WAL diagnostic observations')
    full=sorted(LOCAL+['T03'])
    cases=[{'id':f'T{i:02}','full_case_status':'pass' if f'T{i:02}' in full else 'not_run',
      'scope':'local_or_stdio_only' if f'T{i:02}' in full else 'remaining_sublayers_not_run',
      'actual_role_receipt':None} for i in range(1,23)]
    report={'kind':'P05_owned_fixture_real_CKS_stdio_not_actual_role_or_formal_acceptance',
      'candidate_commit':r['candidate_commit'],'candidate_adopted':False,'fixture_commit':r['fixture_commit'],
      'cks_registered_tools':count,'required_tools':['cks.context.'+t for t in TOOLS],
      'schema_negative_controls':schema_controls,'supported_pin_mismatches':mismatches,'runtime_invalid_controls':invalid,
      'caps_observed':caps,'deadline_trials':3,'deadline_exhausted_observations':len(observed),
      'timeout_result':'observed_cooperative_deadline' if observed else 'inconclusive_deadline_not_observed',
      'stdio_sessions':4,'diagnostic_stdio_sessions':5,'total_new_stdio_sessions':9,'diagnostic_tool_requests':diagnostic_tools,'all_new_tool_requests':113,'stdio_tool_requests':sum(len([v for v in rows.values() if v['request']['method']=='tools/call']) for rows in logs.values()),
      'all_owned_children_reaped':True,'max_parallel_mcp_children_observed':1,'actual_role_sessions':0,'model_calls':0,'listeners':0,
      'independent_acceptance':'not_recorded','formal_completed':0,'formal_total':9,
      'structural_association':'identity_only_unverified_snapshot','quality_evaluated':False,
      'planned_case_progress':cases,'full_cases_passed':full,'full_cases_pending':15,
      'partial_case_notes':{
        'T04':'explicit pack and same analysis observed; actual role selection/duplicate execution control pending',
        'T09':'12 supported pin mismatches and missing/canonical controls observed; verifier ticket/file controls retained at P04; new end-to-end foreign bundle controls pending',
        'T10':'unsupported/partial/caps/requires_reindex observed; existing local placeholder guards retained; actual bundle integration pending',
        'T16':'missing/ambiguous, owned index missing review table and no configured retained backend observed; real unavailable endpoint/startup and hard transport timeout not substituted by dummy/cooperative deadline',
        'T19':'manual policy/security/untagged direct stdio only; actual Analyzer pending',
        'T20':'baseline/expansion/rules same snapshot direct stdio only; actual Analyzer pending'},
      'next':'P06 actual role adapter/local responder; P07 renderer/host cancellation; P08 final readiness'}
    text=['P05 소유 fixture/index·실제 CKS stdio — 준비28단계',
      f"소유 fixture {r['fixture_commit']}; CKS/CKG hash는 P01 빌드 영수증과 일치.",
      f"실제 등록 도구 {count}개와 소비자 schema 일치; 필수5도구 호출. schema 누락/namespace/오래된 상한 대조3건 거부.",
      '4도구×commit/source/snapshot 불일치12건 거부. structural은 identity만 대조하며 snapshot pin을 지원한다고 쓰지 않음.',
      '입력/namespace/pack 오류8건, 상한8종, missing/ambiguous·unsupported·partial·구형 review index·retained backend 미설정 응답 보존.',
      f"1ms cooperative deadline3회 중 실제 deadline exhaustion {len(observed)}회. 전송 timeout/host 취소 시험과 구분.",
      '전체 세션9개/요청113개: 주 시험4세션/56요청 + 초기·수정 전 진단5세션/57요청. 진단을 성공률 분모에서 숨기지 않음.',
      'policy/security는 수동 가설·unknown provenance 유지. 미태그는 안전 판정 아님. rule은 명시적 pack/function opt-in.',
      'source/YAML 삭제 후 retained 결과 유지. 원본 복구·소유 자식4개 EOF exit0/reap·리스너0 확인.',
      'stderr의 CKV manifest 미설정 warning과 dummy alignment 실패 진단을 보존. vector 검색/배포 alignment 검증은 수행하지 않음.',
      'fixture gofmt·race·vet/index 통과. 명령은 구성만 하며 외부 명령/실제 체인·모델 실행 없음.',
      '계획22건 중 전체 충족7건/15건 미완료. T09/T10/T16/T19/T20 등은 하위 경로 관측이며 전체 통과로 세지 않음.',
      '후보 채택/설치·실제 역할·모델·품질/온톨로지 효과 평가·정식 수용 없음. 독립 담당자 미정·F01~08 pending·0/9 유지.',
      '다음: P06 실제 Analyzer/Planner와 로컬 응답기. 이후 P07 그림/취소·P08 최종 준비 판정.']
    return {'stdio-readiness.json':report,'STDIO-READINESS-REPORT.txt':'\n'.join(text)+'\n'}


def prepare(candidate,preflight,owned,output,diagnostics):
    require(not owned.exists() and not output.exists(),'fresh owned and output roots required')
    require(diagnostics.is_dir(),'preserved diagnostic root required')
    require(str(owned).startswith('/private/tmp/'),'owned fixture must be in private tmp')
    old=load('prepare-consumer-local-readiness.py');prior=json.loads((ROOT/PRIOR/'execution.json').read_text())
    original=Path('/private/tmp/ks-isolated-host-20261015/coding-agent')
    def state(p):return {'commit':old.git(p,'rev-parse','HEAD').decode().strip(),'porcelain':old.git(p,'status','--porcelain').decode()}
    before=state(candidate);orig=state(original)
    require(before=={'commit':prior['candidate_commit'],'porcelain':''} and orig==prior['original_adopted_after'],'clean pinned consumer roots')
    binaries={b['name']:sha((preflight/'bin'/b['name']).read_bytes()) for b in json.loads((ROOT/PRIOR/'build-receipts.json').read_text())}
    require(binaries=={b['name']:b['binary_sha256'] for b in json.loads((ROOT/PRIOR/'build-receipts.json').read_text())},'preflight binary mismatch')
    delta=old.git(ROOT,'diff',json.loads((ROOT/PRIOR/'build-receipts.json').read_text())[0]['source_revision'],'--','cmd','internal','pkg','go.mod','go.sum').decode()
    require(not delta,'maintained Go source changed since binary build')
    owned.mkdir();output.mkdir(parents=True);repo=owned/'source';repo.mkdir();logs=output/'command-logs';logs.mkdir();commands=[]
    def run(label,command,cwd=repo):
        started=time.monotonic();p=subprocess.run(command,cwd=cwd,capture_output=True,timeout=180)
        (logs/(label+'.stdout.txt')).write_bytes(p.stdout);(logs/(label+'.stderr.txt')).write_bytes(p.stderr)
        commands.append({'label':label,'command':command,'exit_code':p.returncode,'elapsed_seconds':time.monotonic()-started,'stdout_sha256':sha(p.stdout),'stderr_sha256':sha(p.stderr)})
        dump(output/'commands.json',commands);require(p.returncode==0,'owned command failed: '+label);return p.stdout.decode().strip()
    for path,value in source_files().items():p=repo/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(value)
    run('format',['gofmt','-w','logic.go','logic_test.go','a/same.go','b/same.go']);run('git-init',['git','init','-q']);run('git-add',['git','add','.'])
    run('git-commit',['git','-c','user.name=Owned P05 fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','test: own stdio contract fixture'])
    commit=run('git-head',['git','rev-parse','HEAD']);run('fixture-race',['go','test','-race','-count=1','./...']);run('fixture-vet',['go','vet','./...'])
    run('fixture-bundle',['git','bundle','create',str(output/'fixture.bundle'),'HEAD'])
    shutil.copytree(repo,output/'fixture-source',ignore=shutil.ignore_patterns('.git'))
    ckg=str(preflight/'bin/ckg');cks=str(preflight/'bin/cks');graph=owned/'graph'
    run('index',[ckg,'build','--src',str(repo),'--out',str(graph),'--lang','go','--policy-file',str(repo/'policy.yaml'),'--security-pattern-file',str(repo/'security.yaml')],owned)
    sessions=output/'sessions';sessions.mkdir();discovery=OwnedClient([ckg,'mcp','--graph',str(graph),'--namespace','flow_probe'],sessions/'discovery')
    found={}
    try:
        discovery.initialize()
        for name in ['Seed','Empty','Field','Branch','Stress','Box']:
            d=payload(discovery.request('find-'+name,'tools/call',{'name':'flow_probe.context.find_symbol','arguments':{'name':name,'exact':False,'include_blobs':True}}))
            require(len(d['nodes'])==1,'fixture symbol ambiguity: '+name);found[name]=d['nodes'][0]
    finally:discovery.close()
    policy=owned/'sanitization_rules.yaml';policy.write_bytes((ROOT/'system/policies/sanitization_rules.yaml').read_bytes())
    def config(name,db):
        p=owned/(name+'.yaml');p.write_text('version: 1\nname: owned-P05\nbackends:\n  ckg:\n    path: '+json.dumps(str(db) if db else '')+'\n  ckv:\n    provider: mock\nlisten:\n  transport: stdio\nlogging:\n  level: error\nsanitize:\n  rules_path: '+str(policy)+'\n  default_action: drop\n  fail_closed_on_unknown_rule: true\n');return str(p)
    selected=found['Seed'];query={'node_id':selected['id'],'expected_commit':commit,'expected_source_sha256':sha(selected['source'].encode())}
    client=OwnedClient([cks,'mcp','--config',config('main',graph/'graph.db')],sessions/'main')
    try:
        client.initialize();client.request('tools','tools/list')
        baseline=payload(client.tool('baseline','value_dependencies',query));snapshot=baseline['snapshot_sha256'];pinned={**query,'expected_snapshot_sha256':snapshot}
        client.tool('canonical','value_dependencies',{k:v for k,v in {**pinned,'canonical_id':selected['canonical_id']}.items() if k!='node_id'})
        client.tool('rules','rule_candidates',{'rule_pack':'go-local-review@1.0.0','function':pinned})
        client.tool('expansion','value_dependency_expansion',{'function':pinned,'limits':{'max_functions':8,'max_hops':3}})
        client.tool('annotations','policy_security',pinned);client.tool('untagged','policy_security',{'node_id':found['Empty']['id']})
        client.tool('review','review_function',{'symbol':selected['canonical_id'],'caller_depth':2})
        for tool in ['value_dependencies','rule_candidates','value_dependency_expansion','policy_security']:
            for pin,size in [('expected_commit',40),('expected_source_sha256',64),('expected_snapshot_sha256',64)]:
                q={**query,pin:'0'*size};a={'rule_pack':'go-local-review@1.0.0','function':q} if tool=='rule_candidates' else {'function':q} if tool=='value_dependency_expansion' else q
                client.tool('mismatch-'+tool+'-'+pin,tool,a)
        invalid=[('value_dependencies',{**query,'unknown':True}),('value_dependencies',{**query,'node_id':None}),
          ('rule_candidates',{'function':pinned,'rule_pack':'unknown@0'}),('value_dependency_expansion',{'function':pinned,'limits':{'max_hops':0}}),
          ('policy_security',{**pinned,'max_annotations':129}),('policy_security',{**pinned,'max_annotations':1.5}),
          ('review_function',{}),('wrong_namespace',{})]
        for i,(tool,a) in enumerate(invalid):client.tool('invalid-'+str(i),tool,a)
        client.tool('missing','value_dependencies',{'node_id':'missing-owned-symbol'})
        client.tool('missing-review','review_function',{'symbol':'missing-owned-symbol'})
        client.tool('ambiguous-review','review_function',{'symbol':'Same'})
        client.tool('unsupported','value_dependencies',{'node_id':found['Box']['id']})
        client.tool('partial','value_dependencies',{'node_id':found['Field']['id']})
        for key in ['max_functions','max_candidates','max_total_nodes','max_total_edges','max_total_facts']:
            client.tool('cap-'+key,'value_dependency_expansion',{'function':pinned,'limits':{key:1}})
        client.tool('cap-iterations','value_dependencies',{'node_id':found['Branch']['id'],'limits':{'max_iterations':1}})
        client.tool('cap-annotations','policy_security',{**pinned,'max_annotations':1})
        client.tool('cap-review','review_function',{'symbol':selected['canonical_id'],'max_items':1})
        for i in range(3):client.tool('deadline-'+str(i),'value_dependencies',{'node_id':found['Stress']['id'],'limits':{'timeout_ms':1}})
        removed={p:(repo/p).read_bytes() for p in ['logic.go','policy.yaml','security.yaml']}
        try:
            for p in removed:(repo/p).unlink()
            client.tool('retained','value_dependencies',pinned);client.tool('retained-annotations','policy_security',pinned)
        finally:
            for p,raw in removed.items():(repo/p).write_bytes(raw)
    finally:client.close()
    shutil.copytree(graph,owned/'old-index')
    with closing(sqlite3.connect(owned/'old-index/graph.db')) as db:
        db.execute('DROP TABLE code_edges');db.commit();db.execute('PRAGMA wal_checkpoint(TRUNCATE)')
    old_client=OwnedClient([cks,'mcp','--config',config('old-index',owned/'old-index/graph.db')],sessions/'old-index')
    try:old_client.initialize();old_client.tool('review','review_function',{'symbol':selected['canonical_id']})
    finally:old_client.close()
    unavailable=OwnedClient([cks,'mcp','--config',config('unavailable',None)],sessions/'unavailable')
    try:
        unavailable.initialize()
        for tool in TOOLS[:-1]:
            a={'rule_pack':'go-local-review@1.0.0','function':query} if tool=='rule_candidates' else {'function':query} if tool=='value_dependency_expansion' else query
            unavailable.tool(tool,tool,a)
    finally:unavailable.close()
    for name,p in [('index',graph),('old-index',owned/'old-index')]:
        dest=output/name;dest.mkdir()
        for f in ['graph.db','manifest.json']:(dest/f).write_bytes((p/f).read_bytes())
    dump(output/'execution.json',{'candidate_commit':prior['candidate_commit'],'candidate_adopted':False,
      'candidate_before':before,'candidate_after':state(candidate),'original_before':orig,'original_after':state(original),
      'fixture_commit':commit,'fixture_sha256':{p:sha((repo/p).read_bytes()) for p in source_files()},
      'binary_sha256':binaries,'go_source_changes':[],'selected':selected,'query':query,
      'index_sha256':{n:sha((output/n/'graph.db').read_bytes()) for n in ['index','old-index']},
      'owned_root':str(owned),'actual_role_sessions':0,'model_calls':0,'listeners':0,'formal_runs':0})
    shutil.copytree(diagnostics,output/'diagnostics')
    for name,value in derive(output).items():
        if isinstance(value,str):(output/name).write_text(value)
        else:dump(output/name,value)
    (output/'.gitattributes').write_bytes(ATTRIBUTES);(output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'owned_P05_stdio_not_role_or_formal_acceptance',
      'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in INPUTS},'generator_sha256':sha(Path(__file__).read_bytes()),
      'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('P05 owned stdio evidence captured; actual roles/formal acceptance pending',flush=True)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    for key in ['candidate','preflight','owned','output','diagnostics']:p.add_argument('--'+key,type=Path,required=True)
    a=p.parse_args();prepare(a.candidate.resolve(),a.preflight.resolve(),a.owned.resolve(),a.output.resolve(),a.diagnostics.resolve())
