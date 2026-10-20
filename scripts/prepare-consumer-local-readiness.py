#!/usr/bin/env python3
"""Run local candidate controls and preserve P01-P04 evidence, not host success."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess

from source_packet import SourceCapture, safe, sha, tree_entries, verify

ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PLAN=BASE/'consumer-contract-plan-v1'
BASE_REV='b7e78689676769e8536626e76eed7165d870e1ba'
CHANGES=['plugin/agents/analyzer.md','plugin/agents/planner.md','plugin/scripts/tests/test_review_role_contract.py']
TESTS={'test_deep_review.py':15,'test_review_role_contract.py':16,'test_suspect_labels.py':14}
FULL_LOCAL=['T01','T02','T08','T11','T12','T17']
INPUTS=[PLAN/'manifest.json',PLAN/'implementation-spec.json',PLAN/'isolated-validation-plan.json',
 BASE/'review-gap-v1/manifest.json',Path('internal/system/mcp/testdata/agent-mcp.schema.json'),
 Path('system/eval/operational-preparation-2026-10-15/isolated-host/manifest.json'),
 Path('scripts/source_packet.py'),Path('scripts/prepare-source-review-supplement.py')]
ATTRIBUTES=b'candidate-source/objects/*.commit binary\ncandidate-source/objects/*.tree binary\ncandidate.bundle binary\ncandidate.patch -whitespace\nbuild-logs/*.info.txt -whitespace\n'


def load(name):
    spec=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


def dump(p,value):p.write_text(json.dumps(value,ensure_ascii=False,indent=2,allow_nan=False)+'\n')


def bindings():return sorted(set(INPUTS)|{Path('scripts/prepare-consumer-contract-plan.py')})


def git(repo,*args):return subprocess.check_output(['git','-C',str(repo),'-c','core.fsmonitor=false',*args])


def tools(raw):
    front=raw.decode().split('---',2)[1]
    return [s.strip()[2:] for s in front.split('tools:\n',1)[1].split('\nskills:',1)[0].splitlines() if s.strip().startswith('- ')]


def changed_paths(before_root,before,after_root,after):
    def raw(root,proof,key):return safe(root,proof['objects'][key]).read_bytes()
    a=raw(before_root,before,'commit:'+before['commit']).splitlines()[0].split()[1].decode()
    b=raw(after_root,after,'commit:'+after['commit']).splitlines()[0].split()[1].decode()
    def walk(left,right,prefix):
        if left==right:return []
        x=tree_entries(raw(before_root,before,'tree:'+left)) if left else {}
        y=tree_entries(raw(after_root,after,'tree:'+right)) if right else {}
        result=[]
        for name in sorted(set(x)|set(y)):
            if x.get(name)==y.get(name):continue
            l,r=x.get(name),y.get(name)
            if (not l or l[0]=='40000') and (not r or r[0]=='40000'):
                result+=walk(l[1] if l else None,r[1] if r else None,prefix+name+'/')
            else:result.append(prefix+name)
        return result
    return walk(a,b,'')


def inputs(packet):
    for folder in [PLAN,BASE/'review-gap-v1']:
        m=json.loads((ROOT/folder/'manifest.json').read_text())
        for name,digest in m['artifacts'].items():
            if sha(safe(ROOT/folder,name).read_bytes())!=digest:raise ValueError('frozen prior artifact binding')
    receipt=json.loads((packet/'execution.json').read_text())
    old_root=ROOT/BASE/'review-gap-v1/consumer-source';old=verify(old_root,BASE_REV)
    new_root=packet/'candidate-source';new=verify(new_root,receipt['candidate_commit'])
    names=set(old['files'])|{'plugin/scripts/tests/test_review_role_contract.py','plugin/scripts/suspect_labels.py','plugin/scripts/tests/test_suspect_labels.py'}
    if set(new['files'])!=names:raise ValueError('candidate source inventory')
    delta=changed_paths(old_root,old,new_root,new)
    if delta!=CHANGES or receipt['changed_paths']!=CHANGES:raise ValueError('candidate change scope')
    source={p:safe(new_root,r['artifact']).read_bytes() for p,r in new['files'].items()}
    for p in old['files']:
        if p not in CHANGES and new['files'][p]['sha256']!=old['files'][p]['sha256']:raise ValueError('preserved runtime drift')
    spec=json.loads((ROOT/PLAN/'implementation-spec.json').read_text())
    exposure=[]
    for role in ['analyzer','planner']:
        original=tools(safe(old_root,old['files']['plugin/agents/'+role+'.md']['artifact']).read_bytes())
        rows=[r for r in spec['role_tool_changes'] if r['role']==role]
        actual=tools(source['plugin/agents/'+role+'.md'])
        if actual!=original+[r['alias'] for r in rows]:raise ValueError('exact role tools or preserved order')
        text=source['plugin/agents/'+role+'.md'].decode()
        dispatch=[l for l in text.splitlines() if l.lstrip().startswith('| code_review')]
        if len(dispatch)!=1 or 'DONE' in dispatch[0] or '§3 ANALYSIS' in dispatch[0]:raise ValueError('legacy review dispatch')
        section=text.split('## 7.',1)[1].split('## 8.',1)[0]
        required=['deep-review-evidence.json','deep-review.json','review-report.md','sequence.mmd','modules.mmd',
          'ANALYSIS.deep_review_sha256','PLANNING.review_report_sha256','identity_only_unverified_snapshot','not_recorded']
        if any(s not in section for s in required):raise ValueError('structured role handoff guidance')
        exposure.append({'role':role,'before':len(original),'after':len(actual),'added_aliases':[r['alias'] for r in rows],
          'metadata_input_parity':True,'effective_host_permissions':None,'actual_role_receipt':None})
    schema=json.loads(source['plugin/contracts/cks-tools.schema.json'])['providers']['cks']['tools']
    current=json.loads((ROOT/INPUTS[4]).read_text())['tools']
    for r in spec['role_tool_changes']:
        c=schema['cks_context_'+r['tool']];m=current[r['wire_name']]
        if c['wire_name']!=r['wire_name'] or c['input']!=m['input']:raise ValueError('static tool metadata parity')
    if receipt['kind']!='owned_local_candidate_unit_verifier_hook_not_host_execution':raise ValueError('execution scope')
    if receipt['base_commit']!=BASE_REV or receipt['candidate_clean_before'] is not True or receipt['candidate_clean_after'] is not True:raise ValueError('candidate baseline/clean state')
    if receipt['original_adopted_before']!=receipt['original_adopted_after'] or receipt['original_adopted_after']!={'commit':BASE_REV,'porcelain':''}:raise ValueError('original adopted source changed')
    if receipt['signature_verification_exit']!=0 or receipt['owned_test_temp_entries_after']!=[]:raise ValueError('signature/temporary cleanup')
    if receipt['test_counts']!=TESTS or receipt['tests_passed']!=45 or receipt['source_sha256']!={p:sha(raw) for p,raw in source.items()}:raise ValueError('local test source/count binding')
    for name,count in TESTS.items():
        text=(packet/'logs'/f'{name}.txt').read_text()
        if not re.search(r'Ran '+str(count)+r' tests in [\d.]+s\s+OK\s*$',text):raise ValueError('local regression log')
    for k in ['actual_role_sessions','new_mcp_sessions','new_model_calls','new_network_listeners','formal_runs']:
        if receipt[k]!=0:raise ValueError('local controls promoted to runtime/acceptance')
    if receipt['candidate_adopted'] is not False:raise ValueError('candidate adoption promotion')
    builds=json.loads((packet/'build-receipts.json').read_text())
    if len(builds)!=2 or {r['name'] for r in builds}!={'cks','ckg'}:raise ValueError('binary preflight inventory')
    for r in builds:
        text=(packet/'build-logs'/('build-'+r['name']+'.info.txt')).read_text()
        if r['exit_code']!=0 or not re.fullmatch('[0-9a-f]{64}',r['binary_sha256']) or 'vcs.revision='+r['source_revision'] not in text or r.get('go_source_changes')!=[] or r.get('go_untracked_sources')!=[] or not re.search(r'vcs.modified=(?:true|false)',text):raise ValueError('built binary/source pin')
        if (packet/'build-logs'/('build-'+r['name']+'.stderr.txt')).read_bytes():raise ValueError('binary build stderr')
    host=json.loads((ROOT/INPUTS[5]).read_text())['host']
    if receipt['host_binary_pin']['sha256']!=host['sha256'] or receipt['host_binary_pin']['new_host_execution_performed'] is not False:raise ValueError('host file pin is not new host execution')
    bundle_heads=subprocess.check_output(['git','bundle','list-heads',str(packet/'candidate.bundle')],text=True).strip()
    if bundle_heads!=receipt['candidate_commit']+' HEAD':raise ValueError('recoverable bundle HEAD')
    return receipt,exposure,new


def generate(packet):
    receipt,exposure,proof=inputs(packet)
    cases=[{'id':f'T{i:02}','full_case_status':'pass' if f'T{i:02}' in FULL_LOCAL else 'not_run',
      'scope':'local_source_or_verifier_only' if f'T{i:02}' in FULL_LOCAL else 'owned_stdio_actual_role_renderer_or_lifecycle_evidence_pending',
      'actual_role_receipt':None} for i in range(1,23)]
    steps=[{'id':f'P{i:02}','requested_local_work':'complete','full_cross_layer_validation':'complete' if i==1 else 'pending_P05_P08',
      'source_commit':receipt['candidate_commit'],'scope':'source_preflight_static_prompts_and_local_verifier_hook_tests_only'}for i in range(1,5)]
    report={'kind':'P01_P04_local_implementation_and_controls_not_full_plan_or_host_acceptance',
      'candidate_commit':receipt['candidate_commit'],'candidate_branch':'codex/consumer-contract-readiness',
      'adopted_consumer_commit_unchanged':BASE_REV,'candidate_adopted':False,'changed_paths':CHANGES,
      'role_tool_exposure':exposure,'steps':steps,'planned_case_progress':cases,
      'local_full_cases':FULL_LOCAL,'full_cases_pending':16,'local_tests_passed':45,
      'real_tools_list_executed':False,'actual_role_permissions_verified':False,'diagrams_rendered':False,
      'current_project_quality_evaluated':False,'independent_acceptance':'not_recorded','formal_completed':0,'formal_total':9}
    text=['소비자 P01~P04 구현·로컬 회귀 검증 — 준비27단계',
      '격리 후보 '+receipt['candidate_commit']+' / 채택 원본 '+BASE_REV+' 유지',
      '두 role 문서·새 회귀 테스트3파일만 변경. 도구 Analyzer49→54, Planner17→19. runtime/schema/기존 테스트 보존.',
      '로컬 회귀45개(기존 review15/관련labels14/새role16) 통과; 서명 commit·원본 유지·임시 시험 경로 정리 확인.',
      'P01~P04 요청 구현/로컬 작업 완료. P02~P04의 실제 stdio/role 교차 검증은 P05~P08에 남아 있다.',
      '22계획사례 중 로컬 범위6개 T01/T02/T08/T11/T12/T17 통과. 나머지16개를 통과로 세지 않는다.',
      '실제 tools/list·역할 호출/권한·renderer·host취소/정리 검증 미수행. 새 MCP/역할/모델/리스너0건.',
      '빌드한 CKS/CKG와 host 파일의 hash를 기록했다. host를 새로 실행한 증거가 아니며 다음 실행 전 재확인한다.',
      'hash를 ANALYSIS in_progress에서 먼저 묶고 CLI 검증 후 handoff한다. 재진입은 양쪽 hash를 갱신하며 stale 완료를 거부한다.',
      'CFG/runtime·file membership/interprocedural·structural unpinned·형식과 의미 정답/품질 수용의 경계 유지.',
      '후보는 아직 채택/운영 설치하지 않았다. 독립 담당자 미정·F-01~F-08 pending·freeze null·정식 수용0/9.',
      '다음: P05 소유 Go fixture/index→실제 CKS stdio 도구/거부 경로 검증. 이후 P06 실제 역할·P07 그림/종료·P08 최종 준비 판정.',
      'candidate.bundle에는 전체 reachable Git 이력과 HEAD가 있다. 새 소유 clone에서 복구하고 원본 채택 checkout은 수정하지 않는다.']
    return {'local-readiness.json':report,'LOCAL-READINESS-REPORT.txt':'\n'.join(text)+'\n'}


def prepare(candidate,preflight,output):
    if output.exists():raise ValueError('fresh output required')
    if git(candidate,'status','--porcelain').strip():raise ValueError('candidate must be committed and clean')
    revision=git(candidate,'rev-parse','HEAD').decode().strip()
    changed=git(candidate,'diff','--name-only',BASE_REV,revision).decode().splitlines()
    if changed!=CHANGES:raise ValueError('only two roles and new test may change')
    git(candidate,'merge-base','--is-ancestor',BASE_REV,revision)
    original=Path(json.loads((ROOT/INPUTS[5]).read_text())['source_adoptions']['coding_agent']['source_root'])
    def original_state():return {'commit':git(original,'rev-parse','HEAD').decode().strip(),'porcelain':git(original,'status','--porcelain').decode()}
    before=original_state()
    if before!={'commit':BASE_REV,'porcelain':''}:raise ValueError('original baseline changed')
    output.mkdir(parents=True);(output/'logs').mkdir();(output/'build-logs').mkdir();temp=output/'owned-test-temp';temp.mkdir()
    old=verify(ROOT/BASE/'review-gap-v1/consumer-source',BASE_REV)
    names=set(old['files'])|{'plugin/scripts/tests/test_review_role_contract.py','plugin/scripts/suspect_labels.py','plugin/scripts/tests/test_suspect_labels.py'}
    capture=SourceCapture(candidate,output/'candidate-source',revision)
    for p in sorted(names):
        raw=capture.source(p)
        if (candidate/p).read_bytes()!=raw:raise ValueError('worktree differs from committed test source')
    dump(output/'candidate-source/git-proof.json',capture.proof())
    for name,count in TESTS.items():
        r=subprocess.run(['python3',str(candidate/'plugin/scripts/tests'/name)],cwd=candidate,
          env={**os.environ,'PYTHONDONTWRITEBYTECODE':'1','TMPDIR':str(temp)},capture_output=True,timeout=180)
        (output/'logs'/f'{name}.txt').write_bytes(r.stdout+r.stderr)
        if r.returncode!=0:raise ValueError('local test failure: '+name)
    entries=sorted(p.name for p in temp.iterdir());temp.rmdir()
    for p in preflight.joinpath('logs').glob('build-*'):(output/'build-logs'/p.name).write_bytes(p.read_bytes())
    (output/'build-receipts.json').write_bytes((preflight/'build-receipts.json').read_bytes())
    for r in json.loads((output/'build-receipts.json').read_text()):
        if sha((preflight/'bin'/r['name']).read_bytes())!=r['binary_sha256']:raise ValueError('preflight binary drift')
    signature=subprocess.run(['git','-C',str(candidate),'verify-commit','HEAD'],capture_output=True)
    (output/'logs/signature.txt').write_bytes(signature.stdout+signature.stderr)
    git(candidate,'bundle','create',str(output/'candidate.bundle'),'HEAD')
    (output/'candidate.patch').write_bytes(git(candidate,'diff','--binary','--full-index',BASE_REV,revision))
    host=json.loads((ROOT/INPUTS[5]).read_text())['host'];host_hash=sha(Path(host['path']).read_bytes())
    receipt={'kind':'owned_local_candidate_unit_verifier_hook_not_host_execution','base_commit':BASE_REV,
      'candidate_commit':revision,'changed_paths':changed,'candidate_clean_before':True,
      'candidate_clean_after':not bool(git(candidate,'status','--porcelain').strip()),
      'original_adopted_before':before,'original_adopted_after':original_state(),'test_counts':TESTS,'tests_passed':45,
      'source_sha256':{p:sha((candidate/p).read_bytes())for p in sorted(names)},'signature_verification_exit':signature.returncode,
      'owned_test_temp_entries_after':entries,'host_binary_pin':{'path':host['path'],'sha256':host_hash,
        'recorded_version':host['version'],'new_host_execution_performed':False},'candidate_adopted':False,
      'actual_role_sessions':0,'new_mcp_sessions':0,'new_model_calls':0,'new_network_listeners':0,'formal_runs':0}
    dump(output/'execution.json',receipt)
    artifacts=generate(output)
    for name,value in artifacts.items():
        if isinstance(value,str):(output/name).write_text(value)
        else:dump(output/name,value)
    (output/'.gitattributes').write_bytes(ATTRIBUTES);(output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'consumer_local_P01_P04_candidate_not_adopted_or_host_accepted',
      'input_bindings':{str(p):sha((ROOT/p).read_bytes())for p in bindings()},'generator_sha256':sha(Path(__file__).read_bytes()),
      'candidate_commit':revision,'counts':{'changed_files':3,'role_tool_entries':7,'local_tests':45,'local_plan_cases_passed':6,
        'planned_cases_pending':16,'actual_role_sessions':0,'new_mcp_sessions':0,'formal_runs':0},
      'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes())for p in sorted(output.rglob('*'))if p.is_file()}})
    print('Candidate local tests45 passed; local plan cases6/22; actual role/MCP/formal0',flush=True)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    for key in ['candidate','preflight','output']:p.add_argument('--'+key,type=Path,required=True)
    a=p.parse_args();prepare(a.candidate.resolve(),a.preflight.resolve(),a.output.resolve())
