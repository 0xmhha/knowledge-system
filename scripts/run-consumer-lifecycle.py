#!/usr/bin/env python3
"""Actual-role cancellation and owned stdio budgets; forced exits stay explicit."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shlex
import signal
import subprocess
import sys
import threading
import time
import uuid
ROOT=Path(__file__).resolve().parents[1]
PRIOR=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/host-schema-compatibility-v1'
def module(file):
    s=importlib.util.spec_from_file_location(file.replace('-','_'),ROOT/'scripts'/file);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
base=module('consumer-compatible-host.py');own=module('consumer-process-ownership.py');stdio=module('prepare-consumer-stdio-readiness.py')
dump=base.dump;sha=base.sha;require=base.require
CASES=['normal','sigterm','sigint','deadline','sigkill']


def rows(path):return [json.loads(l)for l in path.read_text().splitlines()]


class Actions(list):
    def __init__(self,action,hold):super().__init__([action,None]if hold else[action]);self.ready=threading.Event();self.release=threading.Event();self.hold_timed_out=False
    def __getitem__(self,index):
        if index==1:self.ready.set();self.hold_timed_out=not self.release.wait(45)
        return super().__getitem__(index)


class Gateway(module('operational-host-probe.py').Gateway):
    def __init__(self,case,actions,out):
        super().__init__(case,actions,out);self.handlers=[];self.errors=[];self.guard=threading.Lock();original=self.server.process_request_thread
        def tracked(*args):
            with self.guard:self.handlers.append(threading.current_thread())
            original(*args)
        self.server.process_request_thread=tracked
        self.server.handle_error=lambda *_args:self.errors.append(type(sys.exc_info()[1]).__name__)
    def close_handlers(self):
        for thread in self.handlers:thread.join(3)
        return {'handler_threads':len(self.handlers),'all_handler_threads_joined':all(not t.is_alive()for t in self.handlers),'errors':self.errors,'server_socket_closed':self.server.fileno()==-1,'server_thread_joined':not self.thread.is_alive()}


def proxy(args):
    require(str(args.log).startswith('/private/tmp/ks-consumer-lifecycle-'),'owned proxy log required')
    require(sha(args.binary)==json.loads((PRIOR/'binary-pins.json').read_text())['cks_sha256'],'proxy binary pin')
    with args.log.open('a')as f:f.write(json.dumps({'kind':'owner','owner':args.owner,'wrapper_pid':os.getpid(),'wrapper_pgid':os.getpgid(0),'wrapper_identity':own.snapshot(os.getpid()),'driver_sha256':sha(Path(__file__)),'base_proxy_sha256':sha(ROOT/'scripts/consumer-compatible-host.py'),'proxy_sha256':sha(ROOT/'scripts/consumer-lifecycle-proxy.py')})+'\n')
    return module('consumer-lifecycle-proxy.py').run(args)


def run_case(root,case):
    out=root/case;out.mkdir();profile=out/'claude-config';profile.mkdir();nonce=uuid.uuid4().hex;repo=root/'source';workspace=repo/'.coding-agent/tickets/KS-P06-REVIEW';before=base.snapshot(workspace)
    pins=json.loads((PRIOR/'binary-pins.json').read_text());require(sha(base.BINARY)==pins['cks_sha256']and sha(base.HOST)=='475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3','host/CKS pins')
    tools=json.loads((ROOT/'internal/system/mcp/testdata/agent-mcp.schema.json').read_text())['tools'];aliases=['mcp__plugin_coding-agent_cks__'+name.replace('.','_')for name in tools]
    observer=shlex.join([sys.executable,str(Path(__file__).resolve()),'--observe','--log',str(out/'hook-inputs.jsonl')])
    settings={'permissions':{'allow':['Bash','Read','Write','Edit']+aliases},'disableClaudeAiConnectors':True,'hooks':{e:[{'hooks':[{'type':'command','command':observer,'timeout':5}],**({'matcher':'*'}if e in ['PreToolUse','PostToolUse']else{})}]for e in ['SessionStart','Stop','PreToolUse','PostToolUse']}}
    dump(out/'settings.json',settings);dump(out/'workspace-before.json',before)
    mcp={'mcpServers':{'plugin_coding-agent_cks':{'command':sys.executable,'args':[str(Path(__file__).resolve()),'--proxy','--owner',nonce,'--binary',str(base.BINARY),'--config',str(root/'cks.yaml'),'--log',str(out/'mcp-frames.jsonl')]}}};dump(out/'mcp.json',mcp)
    p05=json.loads((base.P05/'execution.json').read_text());q=dict(p05['query']);q['expected_snapshot_sha256']=next(x['response']['result']['structuredContent']['snapshot_sha256']for x in json.loads((base.P05/'sessions/main/messages.json').read_text())if x['label']=='baseline')
    actions=Actions({'name':'mcp__plugin_coding-agent_cks__cks_context_value_dependencies','input':q},case!='normal');dump(out/'scripted-actions.json',list(actions))
    env={'PATH':os.environ.get('PATH','/usr/bin:/bin'),'TMPDIR':str(out),'CLAUDE_CONFIG_DIR':str(profile),'ANTHROPIC_API_KEY':'owned-local-fixture-not-a-real-key','CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','DISABLE_TELEMETRY':'1','DISABLE_ERROR_REPORTING':'1','ENABLE_CLAUDEAI_MCP_SERVERS':'false','API_TIMEOUT_MS':'10000','PYTHONDONTWRITEBYTECODE':'1'}
    if 'HOME'in os.environ:env['HOME']=os.environ['HOME']
    label='P07-'+case;argv=[str(base.HOST),'--print','--agent','coding-agent:analyzer','--model','claude-sonnet-4-6','--max-budget-usd','0.10','--plugin-dir',str(base.CANDIDATE/'plugin'),'--strict-mcp-config','--mcp-config',str(out/'mcp.json'),'--setting-sources','','--settings',str(out/'settings.json'),'--permission-mode','manual','--permission-prompts','none','--include-hook-events','--output-format','stream-json','--verbose','--debug-file',str(out/'host.debug.txt'),'--no-session-persistence','--no-chrome','--tools','Bash,Read,Write,Edit','--','KS_HOST_PROBE:'+label+'. Owned lifecycle test, local scripted replies only; no publication or independent quality acceptance.']
    gateway=Gateway(label,actions,out);started=time.monotonic();signal_receipt=None;identities=[];ready=False;overall_timeout=False;collector_error=None;output={}
    with gateway as url:
        env['ANTHROPIC_BASE_URL']=url
        dump(out/'invocation.json',{'argv':argv,'cwd':str(repo),'env':{k:v for k,v in env.items()if k not in ['HOME','PATH','ANTHROPIC_API_KEY']},'api_credential_kind':'constant_fake_local_token','host_sha256':sha(base.HOST),'cks_sha256':sha(base.BINARY),'driver_sha256':sha(Path(__file__)),'owner':nonce,'case':case,'role':'analyzer'})
        process=subprocess.Popen(argv,cwd=repo,env=env,stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,start_new_session=True)
        host_identity=own.snapshot(process.pid)
        def collect():
            nonlocal overall_timeout,collector_error
            try:output['stdout'],output['stderr']=process.communicate(timeout=90)
            except subprocess.TimeoutExpired:
                overall_timeout=True;process.terminate()
                try:output['stdout'],output['stderr']=process.communicate(timeout=5)
                except subprocess.TimeoutExpired:process.kill();output['stdout'],output['stderr']=process.communicate(timeout=5)
            except Exception as e:collector_error=repr(e)
        collector=threading.Thread(target=collect,daemon=True);collector.start()
        if case!='normal':
            ready=actions.ready.wait(30)
            if ready:
                frames=rows(out/'mcp-frames.jsonl');start=next(r for r in frames if r['kind']=='start');require(any(r['kind']=='response'and 'structuredContent'in r['raw']for r in frames),'one tool response before cancellation')
                identities=[own.snapshot(pid)for pid in [process.pid,start['wrapper_pid'],start['child_pid']]];require(all(identities),'live owned host/wrapper/CKS identity')
                require('--owner '+nonce in identities[1]['command']and identities[2]['ppid']==identities[1]['pid'],'owned wrapper-child relationship')
                if case=='deadline':collector.join(1);require(collector.is_alive(),'deadline control completed before expiry')
                number={'sigterm':signal.SIGTERM,'sigint':signal.SIGINT,'deadline':signal.SIGTERM,'sigkill':signal.SIGKILL}[case]
                signal_receipt={'signal':int(number),'target_pid':process.pid,'owned_Popen_child':True,'after_real_tool_response':True,'deadline_expired':case=='deadline','deliberate_forced_fault':case=='sigkill','at_seconds':time.monotonic()-started}
                process.send_signal(number)
            else:process.terminate()
        collector.join(96);actions.release.set()
    handler_cleanup=gateway.close_handlers()
    (out/'host.stdout.jsonl').write_text(output.get('stdout',''));(out/'host.stderr.txt').write_text(output.get('stderr',''))
    frame_path=out/'mcp-frames.jsonl'
    for _ in range(80):
        if frame_path.exists()and any(r['kind']=='exit'for r in rows(frame_path)):break
        time.sleep(.1)
    if not identities and frame_path.exists():
        owner_record=next(r for r in rows(frame_path)if r['kind']=='owner');identities=[r for r in [host_identity,owner_record['wrapper_identity']]if r]
    cleanup=own.wait_gone(identities);rescue=own.rescue(identities)if not cleanup['all_observed_identities_gone']else[]
    after=base.snapshot(workspace);dump(out/'workspace-after.json',after)
    receipt={'case':case,'host_pid':process.pid,'host_exit':process.returncode,'host_reaped':process.poll()is not None,'elapsed_seconds':time.monotonic()-started,'cancel_ready':ready,'signal':signal_receipt,'overall_timeout':overall_timeout,'collector_error':collector_error,'collector_joined':not collector.is_alive(),'hold_timed_out':actions.hold_timed_out,'owned_live_identities_before_cancel':identities,'cleanup':cleanup,'rescue':rescue,'gateway_cleanup':handler_cleanup,'workspace_unchanged':before==after,'real_model_calls':0,'formal_runs':0,'classification':'normal_completion'if case=='normal'else'deliberate_forced_fault_not_normal_success'if case=='sigkill'else'deadline_stop_not_normal_success'if case=='deadline'else'requested_cancellation_not_normal_success'}
    dump(out/'exit.json',receipt);print(json.dumps({'case':case,'raw_host_exit':process.returncode,'classification':receipt['classification'],'output':str(out)}),flush=True)
    exits=[r for r in rows(frame_path)if r['kind']=='exit']
    require(len(exits)==1 and exits[0]['child_reaped']and exits[0]['exit_code']==0,'CKS exit/reap receipt required; PID absence alone is insufficient')
    require(not overall_timeout and collector_error is None and receipt['host_reaped']and receipt['collector_joined']and not actions.hold_timed_out and cleanup['all_observed_identities_gone']and not rescue and handler_cleanup['all_handler_threads_joined'],'owned lifecycle cleanup failed')
    require((case=='normal'and process.returncode==0)or(case!='normal'and ready and signal_receipt is not None),'host/cancel readiness failed')


def budget(root,case='proxy-budget'):
    require(case in ['proxy-budget','proxy-budget-retry'],'owned budget case required')
    out=root/case;out.mkdir();owner=uuid.uuid4().hex;command=[sys.executable,str(Path(__file__).resolve()),'--proxy','--owner',owner,'--binary',str(base.BINARY),'--config',str(root/'cks.yaml'),'--log',str(out/'mcp-frames.jsonl')]
    client=stdio.OwnedClient(command,out/'session')
    query={'name':'cks.context.value_dependencies','arguments':json.loads((base.P05/'execution.json').read_text())['query']}
    try:
        client.initialize();client.request('tools','tools/list')
        for i in range(64):require(not stdio.refused(client.request('call-'+str(i),'tools/call',query)),'pre-cap valid call refused')
        request={'jsonrpc':'2.0','id':client.id+1,'method':'tools/call','params':query};raw=json.dumps(request)+'\n';client.ins.write(raw);client.ins.flush();client.proc.stdin.write(raw);client.proc.stdin.flush()
        eof=client.queue.get(timeout=10);require(eof is None,'65th call reached backend or did not close')
        dump(out/'boundary.json',{'attempted_tool_requests':65,'forwarded_tool_requests':64,'request65':request,'request65_raw':raw,'observed_eof_without_response':True,'scope':'native_stdio_proxy_not_actual_role_65_calls','owner':owner})
    finally:client.close()
    print('Native proxy budget:65 attempted,64 forwarded, EOF refusal',flush=True)


def run(root):
    base.setup(root)
    for name in ['cks.yaml','sanitize.yaml']:
        require((root/name).is_file(),'owned fixture config missing')
    for case in CASES:run_case(root,case)
    budget(root)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--root',type=Path);p.add_argument('--proxy',action='store_true');p.add_argument('--observe',action='store_true');p.add_argument('--owner');p.add_argument('--binary',type=Path);p.add_argument('--config',type=Path);p.add_argument('--log',type=Path);p.add_argument('--budget-only',action='store_true');p.add_argument('--budget-case',default='proxy-budget');a=p.parse_args()
    if a.proxy:raise SystemExit(proxy(a))
    if a.observe:raise SystemExit(base.observe(a))
    require(a.root is not None,'owned root required')
    if a.budget_only:budget(a.root.resolve(),a.budget_case)
    else:run(a.root.resolve())
