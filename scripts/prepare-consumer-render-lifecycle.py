#!/usr/bin/env python3
"""Freeze P07 actual render/lifecycle receipts; retain adapter failure and nonzero exits."""
import argparse
import ast
import importlib.util
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import xml.etree.ElementTree as ET
from source_packet import safe,sha,verify
ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR=BASE/'host-schema-compatibility-v1'
PACKET=BASE/'followups/consumer-render-lifecycle-v1'
BASE_COMMIT='a6a1de98bb9fca8e8b5adaf11b53895a8189b533'
INPUTS=[PRIOR/'manifest.json',Path('scripts/fixtures/consumer-render-lifecycle-plan.json'),*[Path('scripts')/n for n in ['consumer-process-ownership.py','consumer-lifecycle-proxy.py','run-consumer-render.py','run-consumer-lifecycle.py','consumer-compatible-host.py','operational-host-probe.py','source_packet.py','prepare-source-review-supplement.py']]]
CASES=['normal','sigterm','sigint','deadline','sigkill']
TOP=['invocation.json','mcp.json','settings.json','scripted-actions.json','workspace-before.json','workspace-after.json','gateway-requests.json','host.debug.txt','host.stdout.jsonl','host.stderr.txt','mcp-frames.jsonl','hook-inputs.jsonl','exit.json']

def require(v,s):
    if not v:raise ValueError(s)
def dump(p,d):p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
def read(p):return json.loads(p.read_text())
def lines(p):return [json.loads(l)for l in p.read_text().splitlines()]
def module(n):
    s=importlib.util.spec_from_file_location(n.replace('-','_'),ROOT/'scripts'/n);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

def digest(p):return sha(p.read_bytes())

def derive(packet):
    old=module('check-host-schema-compatibility.py');require(not old.check(ROOT/PRIOR),'P06 input proof failed')
    pins=read(ROOT/PRIOR/'binary-pins.json');sources=read(packet/'source-pins.json');current=(ROOT/'scripts/run-consumer-lifecycle.py').read_bytes();initial=(packet/'helper-versions/driver-initial.py.txt').read_bytes();retry=(packet/'helper-versions/driver-budget-retry.py.txt').read_bytes()
    require(sources['lifecycle_final']==sha(current)and sources['lifecycle_initial']==sha(initial)and sources['lifecycle_budget_retry']==sha(retry),'lifecycle source versions')
    def bodies(raw):return {n.name:ast.dump(n,include_attributes=False)for n in ast.parse(raw).body if isinstance(n,(ast.FunctionDef,ast.ClassDef))}
    a,b,c=bodies(initial),bodies(current),bodies(retry)
    require({n for n in a if a[n]!=c[n]}=={'budget'},'initial alias correction scope')
    require({n for n in c if c[n]!=b[n]}=={'proxy','run_case'},'signal observer and exit-proof correction scope')
    handler=next(n for n in ast.walk(ast.parse((ROOT/'scripts/consumer-lifecycle-proxy.py').read_bytes()))if isinstance(n,ast.FunctionDef)and n.name=='on_signal')
    require(ast.dump(ast.Module(body=handler.body,type_ignores=[]),include_attributes=False)==ast.dump(ast.parse('pending.append(number)'),include_attributes=False),'signal handler must only append; no locks or I/O')
    proof=verify(ROOT/PRIOR/'consumer-source',pins['consumer_commit']);ns={'__name__':'pinned_verifier'};exec(compile(safe(ROOT/PRIOR/'consumer-source',proof['files']['plugin/scripts/deep_review.py']['artifact']).read_bytes(),'<pinned verifier>','exec'),ns)
    renders=read(packet/'renders/renders.json');require(len(renders)==9,'render denominator');rendered=0;descendants=[];labels={};input_hashes={}
    for r in renders:
        folder=Path(r['input']).parent.name;p=packet/'renders'/folder;fmt=r['format'];require(digest(p/'input.mmd')==r['input_sha256'],'render input hash')
        require(r['node_reaped']and not r['forced']and not r['rescue']and r['cleanup']['all_observed_identities_gone']and r['cleanup']['remaining_pids']==[],'renderer cleanup')
        require(r['elapsed_seconds']<60 and r['network_isolation_measured']is False,'renderer scope/budget')
        require(r==read(p/(fmt+'-receipt.json')),'render receipt association');descendants+=r['observed_descendants']
        require(any(str(Path(r['input']).parent/(fmt+'-browser-profile'))in x['command']for x in r['observed_descendants']),'owned browser not observed')
        cfg=read(p/(fmt+'-puppeteer.json'));require(cfg['pipe']is True and cfg['userDataDir']==str(Path(r['input']).parent/(fmt+'-browser-profile'))and '--no-sandbox'not in cfg['args'],'browser profile/transport')
        if r['expected_syntax_refusal']:
            require(folder=='malformed-control'and r['exit_code']!=0 and not(p/'output.svg').exists(),'malformed input promoted')
            require('Parse error'in(p/'svg.stderr.txt').read_text(),'syntax refusal cause');continue
        require(r['exit_code']==0 and 100<r['output_bytes']<5000000 and digest(p/('output.'+fmt))==r['output_sha256'],'missing/forged render output');rendered+=1
        case,kind=folder.rsplit('-',1);source=ROOT/PRIOR/'runs'/case/'workspace-after';report=read(source/'deep-review.json');bundle=read(source/'deep-review-evidence.json');ns['validate'](report,bundle)
        expected=ns['sequence_diagram'if kind=='sequence'else'module_diagram'](report)+'\n';require((p/'input.mmd').read_text()==expected==(source/(kind+'.mmd')).read_text(),'typed diagram bytes differ');input_hashes[folder]=r['input_sha256']
        if fmt=='svg':
            svg=ET.fromstring((p/'output.svg').read_bytes());nodes=[n for n in svg.iter()if n.attrib.get('class','').startswith('node ')];edges=[n for n in svg.iter()if n.tag.endswith('path')and'flowchart-link'in n.attrib.get('class','')];text=' '.join(t.strip()for t in svg.itertext()if t.strip())
            require(len(nodes)==(6 if kind=='sequence'else 3)and len(edges)==(7 if kind=='sequence'else 2),'SVG graph shape')
            required=[f'b{i}'for i in range(6)]if kind=='sequence'else['logic.go','Selected retained function','Interprocedural relationships unavailable','unknown,','computed']
            require(all(t in text for t in required),'SVG labels');labels[folder]={'nodes':len(nodes),'edges':len(edges)}
        else:
            raw=(p/'output.png').read_bytes();require(raw[:8]==b'\x89PNG\r\n\x1a\n'and list(struct.unpack('>II',raw[16:24]))==r['png_dimensions']and min(r['png_dimensions'])>0,'PNG dimensions')
    require(rendered==8 and len(input_hashes)==4 and len(labels)==4,'successful render denominator')
    visual=read(packet/'renders/visual-check.json');require(visual['kind']=='AI_visual_inspection_not_independent_semantic_acceptance'and visual['independent_acceptance']=='not_recorded','visual inspection promoted')
    for row in visual['inspected']:require(digest(safe(packet/'renders',row['path']))==row['sha256'],'visual output association')
    require(len(visual['inspected'])==2,'visual sample denominator')
    toolchain=read(packet/'renders/toolchain.json');require(toolchain['versions']['@mermaid-js/mermaid-cli']=='12.0.0'and toolchain['versions']['mermaid']=='12.1.0'and toolchain['download_commands']==[],'local renderer versions')
    require(toolchain['driver_sha256']==digest(ROOT/'scripts/run-consumer-render.py')and toolchain['ownership_helper_sha256']==digest(ROOT/'scripts/consumer-process-ownership.py'),'renderer helper pin')
    role_source=safe(ROOT/PRIOR/'consumer-source',proof['files']['plugin/agents/analyzer.md']['artifact']).read_text().split('---',2)[2].strip();sessions=[];total_calls=0;api_attempts=0
    for case in CASES:
        p=packet/'hosts'/case;inv=read(p/'invocation.json');end=read(p/'exit.json');argv=inv['argv'];frames=lines(p/'mcp-frames.jsonl');owner=next(x for x in frames if x['kind']=='owner');starts=[x for x in frames if x['kind']=='start'];exits=[x for x in frames if x['kind']=='exit']
        require(inv['driver_sha256']==owner['driver_sha256']==sources['lifecycle_final'],'executed host helper pin');require(owner['base_proxy_sha256']==digest(ROOT/'scripts/consumer-compatible-host.py')and owner['proxy_sha256']==digest(ROOT/'scripts/consumer-lifecycle-proxy.py'),'proxy source pin')
        require(inv['host_sha256']=='475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3'and inv['cks_sha256']==pins['cks_sha256'],'host/binary pin')
        require(inv['role']=='analyzer'and argv[argv.index('--agent')+1]=='coding-agent:analyzer'and argv[-2]=='--','actual role/prompt')
        require(argv[argv.index('--setting-sources')+1]==''and argv[argv.index('--permission-prompts')+1]=='none'and '--strict-mcp-config'in argv,'settings boundaries')
        require(not any(t in argv for t in ['--bare','--safe-mode','--dangerously-skip-permissions']),'permission bypass')
        require(inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:')and inv['api_credential_kind']=='constant_fake_local_token','nonlocal provider')
        require(len(starts)==len(exits)==1 and starts[0]['child_pid']==exits[0]['child_pid']and starts[0]['binary_sha256']==pins['cks_sha256'],'owned CKS identity')
        require(exits[0]['exit_code']==0 and exits[0]['child_reaped']and not exits[0]['forced_kill']and not exits[0]['terminate_sent'],'CKS cleanup/forced exit')
        require(exits[0]['signal_worker_joined']and exits[0]['output_workers_joined']and exits[0]['signal_handler_contract']=='append_only_no_lock_or_IO','proxy signal worker cleanup')
        if case=='sigint':
            require(len([r for r in frames if r['kind']=='signal'and r['signal']==2])>=2,'repeated SIGINT was not exercised')
            require('SIGKILL to MCP server process'not in(p/'host.debug.txt').read_text(),'SDK forced proxy termination persisted')
        require(owner['owner']==inv['owner']and owner['wrapper_pid']==starts[0]['wrapper_pid'],'owner nonce')
        require(end['host_reaped']and end['collector_joined']and end['elapsed_seconds']<90 and not end['overall_timeout']and end['collector_error']is None and not end['hold_timed_out'],'host resource bounds')
        require(end['cleanup']['all_observed_identities_gone']and end['cleanup']['remaining_pids']==[]and not end['rescue'],'host cleanup/rescue')
        g=end['gateway_cleanup'];require(g['handler_threads']>=2 and g['all_handler_threads_joined']and g['server_socket_closed']and g['server_thread_joined']and not g['errors'],'loopback responder cleanup')
        require(end['workspace_unchanged']and read(p/'workspace-before.json')==read(p/'workspace-after.json'),'workspace changed')
        require(end['real_model_calls']==end['formal_runs']==0,'formal/model promotion')
        if case=='normal':require(end['host_exit']==0 and end['signal']is None and end['classification']=='normal_completion','normal outcome')
        else:
            sig=end['signal'];number={'sigterm':15,'sigint':2,'deadline':15,'sigkill':9}[case];require(end['cancel_ready']and sig['signal']==number and sig['target_pid']==end['host_pid']and sig['owned_Popen_child']and sig['after_real_tool_response'],'signal ownership/readiness')
            require(sig['deadline_expired']==(case=='deadline')and sig['deliberate_forced_fault']==(case=='sigkill'),'deadline/forced classification')
            expected='deliberate_forced_fault_not_normal_success'if case=='sigkill'else'deadline_stop_not_normal_success'if case=='deadline'else'requested_cancellation_not_normal_success';require(end['classification']==expected,'cancellation promoted to normal success')
            require(end['host_exit']=={'sigterm':143,'sigint':0,'deadline':143,'sigkill':-9}[case],'raw actual host exit hidden')
            require(len(end['owned_live_identities_before_cancel'])==3,'live host/wrapper/CKS proof');live=end['owned_live_identities_before_cancel'];require(live[0]['pid']==end['host_pid']and live[1]['pid']==starts[0]['wrapper_pid']and live[2]['pid']==starts[0]['child_pid']and live[2]['ppid']==live[1]['pid']and '--owner '+inv['owner']in live[1]['command'],'live PID association')
        requests=[json.loads(x['raw'])for x in frames if x['kind']=='request'];responses={json.loads(x['raw']).get('id'):json.loads(x['raw'])for x in frames if x['kind']=='response'};calls=[q for q in requests if q.get('method')=='tools/call'];require(len(calls)==1 and calls[0]['params']['name']=='cks.context.value_dependencies','one real tool before stop');total_calls+=len(calls)
        q=calls[0];value=responses[q['id']]['result']['structuredContent'];require(value['analysis']is not None and value['snapshot_sha256']==q['params']['arguments']['expected_snapshot_sha256'],'real source-bound tool result')
        gateway=read(p/'gateway-requests.json');actual=[r for r in gateway if 'scripted_response'in r];actions=[x for r in actual for x in r['scripted_response']['content']if x['type']=='tool_use'];require(len(actions)==1 and actions[0]['input']==q['params']['arguments'],'actual API/wire association');api_attempts+=len(actions)
        result=next(x for r in actual for m in r['body']['messages']if m['role']=='user'for x in m.get('content',[])if isinstance(x,dict)and x.get('tool_use_id')==actions[0]['id']);require(not result.get('is_error')and json.JSONDecoder().raw_decode(result['content'])[0]==value,'actual tool response')
        events=lines(p/'host.stdout.jsonl');init=next(e for e in events if e.get('subtype')=='init');require(init['claude_code_version']=='2.1.300'and len(init['tools'])==25,'actual host/tools')
        plugin=next(x for x in init['plugins']if x['name']=='coding-agent');body=role_source.replace('${CLAUDE_PLUGIN_ROOT}',plugin['path'])
        for row in actual:
            system=row['body']['system'];text='\n'.join(x['text']for x in system if x.get('type')=='text')if isinstance(system,list)else system;require(text.endswith(body),'actual role body')
        sessions.append({'case':case,'raw_host_exit':end['host_exit'],'classification':end['classification'],'elapsed_seconds':end['elapsed_seconds'],'CKS_exit':0,'rescue':False})
    require(total_calls==api_attempts==5,'actual role request denominator')
    budget=derive_budget(packet,pins,sources);diagnostics=derive_diagnostics(packet,sources,pins)
    audit=read(packet/'postrun-process-audit.json');require(audit['auditor_self_probe']['exit_code']==0 and audit['auditor_self_probe']['stdout'].strip()==str(audit['auditor_pid']),'process audit failed open')
    required={r['pid']for r in descendants}|{r['node_pid']for r in renders}
    for case in CASES:
        p=packet/'hosts'/case;e=read(p/'exit.json');required.add(e['host_pid']);required.update(r['pid']for r in e['owned_live_identities_before_cancel']);f=lines(p/'mcp-frames.jsonl');required.update(r['child_pid']for r in f if r['kind']=='start')
    for case in CASES:
        p=packet/'diagnostics/hosts'/case;e=read(p/'exit.json');required.add(e['host_pid']);required.update(r['pid']for r in e['owned_live_identities_before_cancel']);required.update(r['child_pid']for r in lines(p/'mcp-frames.jsonl')if r['kind']=='start')
    for p in [packet/'native/proxy-budget',packet/'diagnostics/native/proxy-budget',packet/'diagnostics/native/proxy-budget-retry']:
        required.add(read(p/'session/lifecycle.json')['pid']);required.update(r['child_pid']for r in lines(p/'mcp-frames.jsonl')if r['kind']=='start')
    require({r['pid']for r in audit['probes']}==required,'process audit inventory')
    for r in audit['probes']:require(r['exit_code']==1 and r['stdout']==r['stderr']=='','owned process still present/audit error')
    execution=read(packet/'execution.json');prior=read(ROOT/PRIOR/'execution.json')
    require(execution['final_controller_exit']==0 and execution['initial_controller_exit']==1,'controller outcomes hidden')
    for key in ['cks_postrun','consumer_postrun','original_postrun','prior_candidate_postrun']:require(execution[key]==prior[key],'source checkout changed')
    require(execution['maintained_source_changes']==[]and execution['candidate_adopted']is False,'maintained runtime/adoption changed')
    for name,digest_value in read(packet/'preservation.json')['files'].items():require(digest(safe(ROOT,name))==digest_value,'historical source changed: '+name)
    require(read(packet/'preservation.json')==preservation(),'prior packet/source inventory')
    formal=read(packet/'formal-state.json');require(formal==module('prepare-source-review-supplement.py').formal_state(ROOT)and formal['freeze']is None and formal['execution_allowed']is False,'formal state changed')
    full=sorted(set(read(ROOT/PRIOR/'role-readiness.json')['full_plan_cases_passed'])|{'T15','T18'})
    return {'kind':'P07_owned_render_and_lifecycle_not_quality_or_adoption','P07_complete':True,'test_ids_passed':['T15','T18'],'successful_renders':8,'syntax_refusals':1,'SVG_shapes':labels,'actual_host_sessions':sessions,'normal_host_successes':1,'requested_cancellations':2,'deadline_stops':1,'deliberate_forced_faults':1,'actual_role_tool_calls':5,'native_budget':budget,'initial_diagnostics':diagnostics,'all_observed_owned_processes_absent_postrun':True,'audited_pids':len(required),'loopback_responders_closed':5,'forced_cleanup_rescues':0,'final_CKS_children_reaped':6,'final_normal_CKS_exits':6,'all_attempts_CKS_processes':13,'all_attempts_confirmed_CKS_exit_receipts':12,'historical_unconfirmed_CKS_exit_receipts':1,'full_plan_cases_passed':full,'full_plan_cases_pending':['T04','T09','T10','T16','T22'],'full_plan_cases_failed':[],'formal_completed':0,'formal_total':9,'candidate_adopted':False,'real_model_calls':0,'formal_runs':0,'ontology_effect_acceptance':'not_evaluated','render_semantic_quality':'not_evaluated','network_security_sandbox_proven':False,'in_flight_CKS_query_cancellation':'not_measured_cancel_was_after_completed_tool_while_waiting_for_local_API','next':'T04/T09/T10/T16 integration gaps, then P08 T22 readiness/recovery/publication boundary'}


def derive_budget(packet,pins,sources):
    stdio=module('prepare-consumer-stdio-readiness.py');total=0
    entries=[(packet/'diagnostics/native/proxy-budget',1,'lifecycle_initial'),(packet/'diagnostics/native/proxy-budget-retry',64,'lifecycle_budget_retry'),(packet/'native/proxy-budget',64,'lifecycle_final')]
    for p,expected_count,version in entries:
        f=lines(p/'mcp-frames.jsonl');owner=next(x for x in f if x['kind']=='owner');exits=[x for x in f if x['kind']=='exit'];require(owner['driver_sha256']==sources[version],'native helper version')
        require(len(exits)==1 and exits[0]['exit_code']==0 and exits[0]['child_reaped']and not exits[0]['forced_kill']and not exits[0]['terminate_sent'],'native CKS cleanup')
        require(next(x for x in f if x['kind']=='start')['binary_sha256']==pins['cks_sha256'],'native binary pin')
        calls=[json.loads(x['raw'])for x in f if x['kind']=='request'and json.loads(x['raw']).get('method')=='tools/call'];require(len(calls)==expected_count,'native forwarded denominator');total+=len(calls)
        life=read(p/'session/lifecycle.json');require(life['exit_code']==0 and life['child_reaped']and life['reader_joined']and not life['terminate_sent']and not life['kill_sent']and life['tool_requests']==expected_count,'native proxy cleanup')
        messages=read(p/'session/messages.json');require(len(messages)==expected_count+2,'native completed RPC denominator')
        require([json.loads(line)for line in(p/'session/stdout.jsonl').read_text().splitlines()]==[r['response']for r in messages],'native output/message association')
        responses={json.loads(x['raw']).get('id'):json.loads(x['raw'])for x in f if x['kind']=='response'}
        for r in messages:require(responses[r['response']['id']]==r['response'],'proxy/wire response association')
        if expected_count==1:
            require(stdio.refused(messages[-1]['response'])and calls[0]['params']['name']=='flow_probe.context.find_symbol','initial fixture alias failure omitted');require(read(packet/'diagnostics/budget-diagnosis.json')['initial_controller_exit']==1,'failed controller hidden')
        else:
            boundary=read(p/'boundary.json');require(boundary['attempted_tool_requests']==65 and boundary['forwarded_tool_requests']==64 and boundary['observed_eof_without_response']and boundary['scope']=='native_stdio_proxy_not_actual_role_65_calls','proxy boundary promoted')
            refused=[x for x in f if x['kind']=='budget_refused'];require(len(refused)==1 and refused[0]['tool_requests']==65,'65th not refused before forwarding')
            require(all(not stdio.refused(r['response'])for r in messages),'pre-cap request failed')
            sent=[json.loads(l)for l in(p/'session/stdin.jsonl').read_text().splitlines()if 'id'in json.loads(l)];require(sent[:-1]==[r['request']for r in messages]and sent[-1]==boundary['request65'],'65th wire request missing');require(not any(r['response'].get('id')==boundary['request65']['id']for r in messages),'65th response invented')
    return {'initial_alias_error_forwarded':1,'initial_corrected_proxy_forwarded':64,'final_proxy_attempted':65,'final_proxy_forwarded':64,'final_65th_refused_before_CKS':True,'native_tool_requests_forwarded_all_attempts':total,'all_new_CKS_tool_requests_including_hosts':total+10,'boundary_scope':'native_stdio_proxy_not_actual_role65_calls'}


def derive_diagnostics(packet,sources,pins):
    for case in CASES:
        p=packet/'diagnostics/hosts'/case;inv=read(p/'invocation.json');e=read(p/'exit.json');f=lines(p/'mcp-frames.jsonl')
        require(inv['driver_sha256']==sources['lifecycle_initial']and inv['cks_sha256']==pins['cks_sha256'],'diagnostic source/binary pin')
        starts=[x for x in f if x['kind']=='start'];exits=[x for x in f if x['kind']=='exit'];require(len(starts)==1,'diagnostic CKS denominator')
        require(e['host_exit']=={'normal':0,'sigterm':143,'sigint':0,'deadline':143,'sigkill':-9}[case],'diagnostic raw host exit')
        calls=[json.loads(x['raw'])for x in f if x['kind']=='request'and json.loads(x['raw']).get('method')=='tools/call'];require(len(calls)==1,'diagnostic tool denominator')
        if case=='sigint':
            debug=(p/'host.debug.txt').read_text();require(not exits and debug.count('Sending SIGINT to MCP server process')>=2 and 'SIGKILL to MCP server process'in debug,'initial forced/unconfirmed outcome omitted')
            require(e['cleanup']['all_observed_identities_gone'],'initial inadequate PID-only claim not retained')
        else:require(len(exits)==1 and exits[0]['exit_code']==0 and exits[0]['child_reaped'],'diagnostic known exit receipt')
    return {'host_sessions':5,'initial_host_controller_exit':1,'initial_controller_failure':'proxy_budget_alias_error','SIGINT_observer_SDK_forced_kill':1,'SIGINT_CKS_exit_status':'unknown_missing_exit_receipt','SIGINT_CKS_reaped':'unconfirmed','initial_PID_absence_not_accepted_as_reap_proof':True,'preserved_as_failed_observation':True,'correction':'append-only signal handler plus explicit CKS exit/reap requirement; rerun all5 hosts and budget'}


def preservation():
    keep={}
    roots=sorted(p.parent.relative_to(ROOT).as_posix()for p in(ROOT/BASE).glob('*-v1/manifest.json'));require(len(roots)==30,'historical packet count')
    import io,tarfile
    extra=['docs/REMAINING-WORK.md','docs/spec-driven/CURRENT-HANDOFF.md','cmd','internal','pkg','go.mod','go.sum']
    archive=subprocess.check_output(['git','archive',BASE_COMMIT,*roots,*extra],cwd=ROOT)
    with tarfile.open(fileobj=io.BytesIO(archive))as tar:
        for r in tar:
            if r.isfile():keep[r.name]=sha(tar.extractfile(r).read())
    for name,value in keep.items():require(digest(safe(ROOT,name))==value,'maintained/historical bytes changed')
    return {'base_commit':BASE_COMMIT,'prior_packets':30,'files':keep}


def prepare(render,owned,diagnostic,output):
    require(not output.exists(),'fresh output required');output.mkdir(parents=True);(output/'hosts').mkdir();(output/'native').mkdir();(output/'helper-versions').mkdir()
    for case in CASES:
        dest=output/'hosts'/case;dest.mkdir()
        for name in TOP:shutil.copyfile(owned/case/name,dest/name)
    shutil.copytree(owned/'proxy-budget',output/'native/proxy-budget')
    (output/'diagnostics/hosts').mkdir(parents=True);(output/'diagnostics/native').mkdir()
    for case in CASES:
        dest=output/'diagnostics/hosts'/case;dest.mkdir()
        for name in TOP:shutil.copyfile(diagnostic/case/name,dest/name)
    for case in ['proxy-budget','proxy-budget-retry']:shutil.copytree(diagnostic/case,output/'diagnostics/native'/case)
    shutil.copyfile(diagnostic/'budget-diagnosis.json',output/'diagnostics/budget-diagnosis.json')
    for name in ['driver-initial.py.txt','driver-budget-retry.py.txt']:shutil.copyfile(diagnostic/name,output/'helper-versions'/name)
    target=output/'renders';target.mkdir()
    for name in ['renders.json','toolchain.json','visual-check.json']:shutil.copyfile(render/name,target/name)
    for r in read(render/'renders.json'):
        source=Path(r['input']).parent;dest=target/source.name;dest.mkdir(exist_ok=True)
        for f in source.iterdir():
            if f.is_file():shutil.copyfile(f,dest/f.name)
    sources={'lifecycle_initial':digest(diagnostic/'driver-initial.py.txt'),'lifecycle_budget_retry':digest(diagnostic/'driver-budget-retry.py.txt'),'lifecycle_final':digest(ROOT/'scripts/run-consumer-lifecycle.py')};dump(output/'source-pins.json',sources)
    repos={'cks_postrun':Path('/private/tmp/ks-host-schema-cks-20261017'),'consumer_postrun':Path('/private/tmp/ks-host-schema-consumer-20261017'),'original_postrun':Path('/private/tmp/ks-isolated-host-20261015/coding-agent'),'prior_candidate_postrun':Path('/private/tmp/ks-consumer-contract-candidate-20261017')}
    execution={k:{'commit':subprocess.check_output(['git','-C',str(p),'rev-parse','HEAD'],text=True).strip(),'porcelain':subprocess.check_output(['git','-C',str(p),'status','--porcelain'],text=True)}for k,p in repos.items()};changes=subprocess.check_output(['git','status','--porcelain','--','cmd','internal','pkg','go.mod','go.sum'],cwd=ROOT,text=True)
    execution.update(maintained_source_changes=[]if not changes else[changes],candidate_adopted=False,final_controller_exit=0,initial_controller_exit=1,retry_budget_exit=0,host_initial_source_version='helper-versions/driver-initial.py.txt',observer_changes='append-only signal queue; final exit/reap gate',host_runtime_changes=0,existing_full_Go_validation='reused_pinned_unchanged_sources_and_binaries_from_P06_not_new_Go_test_runs');dump(output/'execution.json',execution)
    dump(output/'preservation.json',preservation())
    dump(output/'formal-state.json',module('prepare-source-review-supplement.py').formal_state(ROOT))
    # Query only known owned PIDs, plus this auditor as a live positive control.
    ids=set()
    for r in read(target/'renders.json'):ids.add(r['node_pid']);ids.update(x['pid']for x in r['observed_descendants'])
    for case in CASES:
        e=read(owned/case/'exit.json');ids.add(e['host_pid']);ids.update(r['pid']for r in e['owned_live_identities_before_cancel']);ids.update(r['child_pid']for r in lines(owned/case/'mcp-frames.jsonl')if r['kind']=='start')
    for case in CASES:
        e=read(diagnostic/case/'exit.json');ids.add(e['host_pid']);ids.update(r['pid']for r in e['owned_live_identities_before_cancel']);ids.update(r['child_pid']for r in lines(diagnostic/case/'mcp-frames.jsonl')if r['kind']=='start')
    for p in [owned/'proxy-budget',diagnostic/'proxy-budget',diagnostic/'proxy-budget-retry']:ids.add(read(p/'session/lifecycle.json')['pid']);ids.update(r['child_pid']for r in lines(p/'mcp-frames.jsonl')if r['kind']=='start')
    def probe(pid):
        p=subprocess.run(['ps','-p',str(pid),'-o','pid='],capture_output=True,text=True);return {'pid':pid,'command':['ps','-p',str(pid),'-o','pid='],'exit_code':p.returncode,'stdout':p.stdout,'stderr':p.stderr}
    dump(output/'postrun-process-audit.json',{'auditor_pid':os.getpid(),'auditor_self_probe':probe(os.getpid()),'probes':[probe(pid)for pid in sorted(ids)]})
    report=derive(output);dump(output/'render-lifecycle.json',report)
    text=['준비31단계/P07 — 로컬 그림 렌더링 및 실제 host 취소·정리',
      '두 P06 검토 세대×CFG/파일 membership을 원본 typed 자료에서 재생성,4입력을 SVG/PNG8개로 렌더. 문법 오류1건은 거부.',
      '로컬 Mermaid CLI12.0.0/Mermaid12.1.0과 고정 Node/Chromium만 사용. 관측 browser/Node 종료·그림 legibility 확인은 의미 정답·실행 trace·온톨로지 효과 수용이 아니다.',
      '실제 Analyzer/CKS5세션: normal0, SIGTERM143, SIGINT0, deadline143, deliberate SIGKILL-9. 취소/강제 종료를 정상 host 성공으로 합산하지 않음.',
      '각1회 실제 도구 응답 후 API 대기 상태에서 취소. CKS 계산 중 취소와 CKS 직접 신호 처리는 미측정; 관측기가 EOF로 전달. 최종6CKS exit0/reap, 추가 rescue0.',
      '검증기 별칭 오류1건을 보존. 재시도 native proxy65시도 중64회 CKS 전달,65번째는 EOF 거부. 실제 역할65회 호출을 검증한 뜻이 아님.',
      '초기 역할5/native65+최종 역할5/native64=전체 CKS139요청, CKS13프로세스. 종료 영수증12개·초기SIGINT1개 unknown/reap미확인을 보존한다.',
      '초기SIGINT에서 반복신호 중 관측기 잠금→SDK강제종료·CKS종료영수증누락을 발견했다. 기존 PID부재만의 정리판정을 실패로 보존, 새 관측기/종료영수증 강제 후 전5시나리오 재시험.',
      '직접 PID 사후 점검 '+str(report['audited_pids'])+'개와 감사기 live positive control 보존. OS 전체 보안/네트워크 sandbox 증명 아님.',
      '계획22개 중17통과/실패0/미완료5(T04/T09/T10/T16/T22). 다음4통합 검증 후 P08.',
      '이전30패킷·현재 runtime/schema/문서·원본/후보 소스 보존. Go 전체검사는 변경 없는 기존 소스/바이너리의 결과를 재사용하며 새 실행으로 세지 않음.',
      '담당자미정/F01~08 pending/freeze null/executionfalse/정식0/9·실제 모델0·후보채택false 유지.']
    (output/'REPORT.txt').write_text('\n'.join(text)+'\n');(output/'generator.py.txt').write_bytes(Path(__file__).read_bytes());(output/'.gitattributes').write_text('**/*.png binary\n')
    dump(output/'manifest.json',{'schema_version':1,'kind':'P07_render_lifecycle_not_quality_or_adoption','input_bindings':{str(p):digest(ROOT/p)for p in INPUTS},'generator_sha256':digest(Path(__file__)),'artifacts':{p.relative_to(output).as_posix():digest(p)for p in sorted(output.rglob('*'))if p.is_file()}})
    print('P07 evidence frozen:8 renders,5 actual lifecycle sessions, plan17/22; formal0/9')


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--render',type=Path,required=True);p.add_argument('--owned',type=Path,required=True);p.add_argument('--diagnostic',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();prepare(a.render.resolve(),a.owned.resolve(),a.diagnostic.resolve(),a.output.resolve())
