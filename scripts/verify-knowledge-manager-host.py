#!/usr/bin/env python3
"""Actual Claude Code + declared manager MCP, local scripted API only.

Tests transport/lifecycle, not model intelligence, paid inference or independent
human acceptance. All sources, fake credentials, signals and state are owned.
"""
import argparse
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shlex
import shutil
import signal
import subprocess
import sys
import threading
import time

ROOT = Path(__file__).resolve().parents[1]
PLUGIN = ROOT/'plugins/knowledge-manager'
MANAGER = PLUGIN/'scripts/manager.py'
TRANSPORT = ROOT/'scripts/operational-host-probe.py'
PREFIX = 'mcp__plugin_knowledge-manager_cks-manager__'


def require(value,message):
    if not value:
        raise ValueError(message)


def write(path,value):
    path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def tool_value(block):
    """Decode actual host tool-result text, not a scripted tool response."""
    content = block.get('content',[])
    texts = [content]if isinstance(content,str)else [r['text']for r in content if r.get('type')=='text']
    decoder = json.JSONDecoder()
    for text in texts:
        for index,char in enumerate(text):
            if char=='{':
                try:
                    value,_ = decoder.raw_decode(text[index:])
                except ValueError:
                    continue
                if isinstance(value,dict)and ('schema_version'in value or 'evidence'in value or 'profile'in value or 'format_version'in value and 'coordinates'in value):
                    return value
    raise ValueError('actual tool result did not contain a manager/native response')


class Actions:
    def __init__(self,case,fixture,out):
        self.case,self.fixture,self.out = case,fixture,out
        self.steps,self.observed,self.fault = [],[],None
        self.index,self.awaiting = 0,None
        self.done = case=='missing'
        self.kill_ready,self.kill_release = threading.Event(),threading.Event()
        self.profile = {'schema_version':1,'project_id':fixture['registration']['project_id'],'profile_id':fixture['registration']['profile_id']}
        self.state = fixture.setdefault('workflow',{})
        if case=='missing':
            return
        if case=='insufficient':
            self.add('cks.manager.collect',lambda:{},lambda value:require(value['code']=='insufficient_evidence','wrong insufficient-evidence refusal'),True)
            return
        if case=='restart':
            self.add('cks.context.get_for_task_v2',lambda:{'prompt':'HostValue returns 42'},self.read)
            self.add('cks.manager.status',lambda:{},self.manager_status)
            return
        if case=='recover':
            self.add('cks.dataset.status',self.reference,self.recovered)
            self.add('cks.manager.resume_build',lambda:{'job_id':self.state['job']['job_id'],'original_request':self.state['request']},self.recovered)
            self.add('cks.dataset.inspect_profile',lambda:self.profile,self.inspection)
            self.add('cks.dataset.build',self.request,self.accepted)
            self.add('cks.dataset.status',self.reference,self.finished)
            self.add('cks.manager.select_reader',self.reader_args,self.reader_selected)
            self.add('cks.context.get_for_task_v2',lambda:{'prompt':'HostValue returns 42'},self.read)
            return
        self.add('cks.manager.collect',lambda:{},self.collected)
        self.add('cks.manager.evidence',lambda:{'collection_id':self.state['collection_id'],'path':'value.go','start_line':4,'end_line':4},self.evidence)
        self.add('cks.manager.draft',lambda:{'collection_id':self.state['collection_id']},self.drafted)
        self.add('cks.manager.review_plan',lambda:{'draft_id':self.state['draft_id']},self.planned)
        self.add('cks.manager.review',lambda:{'response_file':str(self.out/'response.json')},self.reviewed)
        self.add('cks.manager.adopt',lambda:{'draft_id':self.state['draft_id'],'review_revision':self.state['review_revision']},self.adopted)
        self.add('cks.manager.select_input',lambda:{'adoption_id':self.state['adoption']['adoption_id']},self.input_selected)
        self.add('cks.dataset.inspect_profile',lambda:{**self.profile,'project_id':'foreign-project'},lambda value:require(value['code']=='project_or_profile_mismatch','foreign project not refused'),True)
        self.add('cks.dataset.inspect_profile',lambda:self.profile,self.inspection)
        self.add('cks.dataset.build',self.request,self.accepted)
        self.add('cks.dataset.status',self.reference,self.inflight if case in ('active-stop','kill')else self.finished)
        if case not in ('active-stop','kill'):
            self.add('cks.manager.select_reader',self.reader_args,self.reader_selected)
            self.add('cks.context.get_for_task_v2',lambda:{'prompt':'HostValue returns 42'},self.read)
            self.add('cks.manager.status',lambda:{},self.manager_status)

    def add(self,name,args,consume,error=False):
        self.steps.append((name,args,consume,error))

    def __len__(self):
        return 0 if self.done else 1000

    def __getitem__(self,request_index):
        try:
            body = self.gateway.requests[-1]['body']
            if self.awaiting:
                found = [block for message in body.get('messages',[])if isinstance(message.get('content'),list)
                         for block in message['content']if block.get('type')=='tool_result'and block.get('tool_use_id')==self.awaiting]
                require(len(found)==1,'actual host tool result missing or duplicated')
                name,_,consume,error = self.steps[self.index]
                require(bool(found[0].get('is_error'))==error,'actual host tool failed/incorrect refusal: '+name)
                value = tool_value(found[0])
                self.observed.append({'tool':name,'actual_tool_use_id':self.awaiting,'is_error':error,'value':copy.deepcopy(value)})
                write(self.out/'actual-tool-results.json',self.observed)
                advance = consume(value)
                self.awaiting=None
                if advance is not False:
                    self.index+=1
                if self.index==len(self.steps):
                    self.done=True
                    write(self.out/'workflow.json',self.state)
                    return None
            name,args,_,_ = self.steps[self.index]
            alias = PREFIX+name.replace('.','_')
            require(any(row.get('name')==alias for row in body.get('tools',[])),'declared manager tool absent from actual host API: '+alias)
            self.awaiting='fixture_tool_'+str(request_index)
            return {'name':alias,'input':args()}
        except Exception as error:
            self.fault=str(error)
            self.done=True
            write(self.out/'workflow-fault.json',{'stage':self.index,'error':self.fault})
            return None

    def collected(self,value):
        self.state['collection_id']=value['collection_id']

    def evidence(self,value):
        require(value['evidence']['project_id']==self.profile['project_id']and 'return 42'in value['text'],'foreign/missing actual source evidence')
        self.state['evidence']=value

    def drafted(self,value):
        self.state['draft_id']=value['draft_id']
        draft = json.loads((Path(value['draft_dir'])/'draft.json').read_text())
        require(draft['human_approval']=='not_recorded'and all(c['status']=='proposed'for c in draft['claims']),'draft not proposed')
        self.state['draft']=draft

    def planned(self,value):
        require(value['questions']and value['human_approval']=='not_inferred'and all(q['answer_status']=='not_provided'for q in value['questions']),'unknown reasons were invented')
        write(self.out/'response.json',{'schema_version':1,'project_id':self.profile['project_id'],'draft_id':self.state['draft_id'],'expected_revision':value['revision'],
            'reviewer':'controlled-host-fixture','response_source':{'kind':'controlled_fixture','reference':'Explicit local transport fixture; not independent human acceptance'},
            'decisions':[{'claim_id':claim_id,'disposition':'accepted','reason':'Accept the cited bytes as a controlled observation only; rationale remains unknown.'}for claim_id in sorted({c['claim_id']for c in self.state['draft']['claims']})],
            'resolutions':[]})

    def reviewed(self,value):
        require(value['review_authentication']=='not_performed','fixture response authenticated incorrectly')
        self.state['review_revision']=value['review_revision']

    def adopted(self,value):
        require(value['stage']=='input_adopted_not_DB_published','unexpected adoption stage')
        self.state['adoption']=value

    def input_selected(self,value):
        require(value['selected_input']==self.state['adoption']['adoption_id']and value['activation_policy']=='hold_for_review_only','unfiltered input or publication selected')

    def inspection(self,value):
        self.state['profile']=value['profile']

    def request(self):
        p = self.state['profile']
        suffix = 'v2'if self.case=='recover'else 'v1'
        value = {**self.profile,**{k:p[k]for k in ('profile_digest','source_pin','input_digest','embedding_digest')},'base':p['observed_base'],
                 'new_version':'host-reviewed-'+suffix,'activation':'hold_for_review','idempotency_key':'owned-actual-host-'+suffix,'deadline_seconds':120}
        self.state['request']=value
        return value

    def accepted(self,value):
        require(value['project_id']==self.profile['project_id']and value['profile_id']==self.profile['profile_id'],'foreign job accepted')
        self.state['job']=value

    def reference(self):
        return {**self.profile,'job_id':self.state['job']['job_id']}

    def finished(self,value):
        self.state['job']=value
        if not value['terminal']:
            return False
        require(value['state']=='held_for_review'and value['activated']is False and value['candidate']is not None,'actual host candidate failed/published')

    def inflight(self,value):
        self.state['job']=value
        require(not value['terminal'],'controlled delayed graph ended before host interruption')
        if not(self.fixture['root']/'graph-started').exists():
            return False
        self.state['inflight_graph_observed']=True
        write(self.out/'workflow.json',self.state)
        if self.case=='kill':
            self.kill_ready.set()
            require(self.kill_release.wait(timeout=10),'owned host kill was not delivered')

    def recovered(self,value):
        require(value['job_id']==self.state['job']['job_id']and value['terminal']and value['state']=='canceled'and value['activated']is False,'actual parent shutdown did not persist cancellation')
        self.state['canceled_job']=value

    def reader_args(self):
        return {'job_id':self.state['job']['job_id'],'expected_coordinates':self.state['job']['candidate']['coordinates']}

    def reader_selected(self,value):
        require(value['stage']=='reader_initialized_unqueried'and value['serving_read_observed']is False,'initialization counted as a read')

    def read(self,value):
        require(value['coordinates']==self.state['job']['candidate']['coordinates']and value['citations']and any(c['origin_id']=='docs:corpus.0'for c in value['citations'])
                and all(c['project_id']==self.profile['project_id']for c in value['citations'])and 'HostValue'in json.dumps(value['bodies'])and 'return 42'in json.dumps(value['bodies']),'actual host read missing/mixed source')
        if self.case=='restart':
            require(value['coordinates']==self.state['read']['coordinates']and value['citations']==self.state['read']['citations']and value['bodies']==self.state['read']['bodies'],'actual restarted host changed its read')
        self.state['read']=value

    def manager_status(self,value):
        require(value['last_validated_read']['coordinates']==self.state['read']['coordinates']and value['last_validated_read']['candidate_publication_inferred']is False,'observed read/pub boundary differs')


def fixture(root,kind):
    root.mkdir()
    source = root/'source'
    source.mkdir()
    (source/'.env').write_text('PRIVATE_HOST_INPUT_SENTINEL\n')
    if kind!='insufficient':
        (source/'go.mod').write_text('module example.invalid/actual-host\n\ngo 1.26.9\n')
        (source/'value.go').write_text('package fixture\n\n// HostValue returns the owned fixture marker.\nfunc HostValue() int { return 42 }\n')
    graph = ROOT/'bin/ckg'
    if kind in ('active-stop','kill'):
        graph = root/'delayed-ckg.sh'
        graph.write_text('#!/bin/sh\nif [ "$1" = "build" ] && [ ! -f '+shlex.quote(str(root/'allow-graph'))+' ]; then\n'
                         '  printf "%s\\n" "$$" > '+shlex.quote(str(root/'graph-started'))+'\n  exec /bin/sleep 30\nfi\nexec '+shlex.quote(str(ROOT/'bin/ckg'))+' "$@"\n')
        graph.chmod(0o700)
    cks = ROOT/'bin/cks'
    if kind=='missing':
        cks = root/'missing-cks.sh'
        cks.write_text('#!/bin/sh\nexec '+shlex.quote(str(ROOT/'bin/cks'))+' "$@"\n')
        cks.chmod(0o700)
    command = [sys.executable,str(MANAGER),'register','--source',str(source),'--dataset',str(root/'dataset'),'--state-dir',str(root/'manager'),
               '--cks-bin',str(cks),'--graph-bin',str(graph),'--vector-bin',str(ROOT/'bin/ckv'),'--embedder','mock','--model-name','mock']
    p = subprocess.run(command,capture_output=True,timeout=30)
    (root/'register.stdout.json').write_bytes(p.stdout)
    (root/'register.stderr.txt').write_bytes(p.stderr)
    write(root/'register-command.json',{'argv':command,'exit_code':p.returncode})
    require(p.returncode==0,'owned registration failed')
    if kind=='missing':
        cks.unlink()
    registration = root/'manager/registration.json'
    return {'root':root,'source':source,'path':registration,'registration':json.loads(registration.read_text())['registration'],
            'source_before':{p.relative_to(source).as_posix():sha(p)for p in source.rglob('*')if p.is_file()}}


def run_host(cli,subject,case,out,transport,actions_factory=Actions,manager_mode='reviewed'):
    out.mkdir()
    (out/'claude-config').mkdir()
    (out/'tmp').mkdir()
    declared = json.loads((PLUGIN/'.mcp.json').read_text())['mcpServers']['cks-manager']
    command = [declared['command'],*[v.replace('${CLAUDE_PLUGIN_ROOT}',str(PLUGIN))for v in declared['args']]]
    launcher = out/'launcher.sh'
    launcher.write_text('#!/bin/sh\nexec '+shlex.join(command)+'\n')
    launcher.chmod(0o700)
    mcp = {'mcpServers':{'plugin_knowledge-manager_cks-manager':{'command':sys.executable,
        'args':[str(TRANSPORT),'--proxy','--binary',str(launcher),'--log',str(out/'mcp-frames.jsonl')]}}}
    write(out/'mcp.json',mcp)
    observer = shlex.join([sys.executable,str(TRANSPORT),'--observe','--log',str(out/'hooks.jsonl')])
    settings = {'permissions':{'allow':[PREFIX+'*']},'disableClaudeAiConnectors':True,
                'hooks':{event:[{'hooks':[{'type':'command','command':observer,'timeout':5}],**({'matcher':'*'}if event in ('PreToolUse','PostToolUse')else{})}]for event in ('SessionStart','Stop','PreToolUse','PostToolUse')}}
    write(out/'settings.json',settings)
    env = {'PATH':os.environ.get('PATH','/usr/bin:/bin:/usr/sbin:/sbin'),'CLAUDE_CONFIG_DIR':str(out/'claude-config'),'TMPDIR':str(out/'tmp'),
           'CKS_MANAGER_REGISTRATION':str(subject['path']),'CKS_MANAGER_MODE':manager_mode,'PYTHONDONTWRITEBYTECODE':'1','ANTHROPIC_API_KEY':'local-fixture-not-a-real-key',
           'CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','DISABLE_TELEMETRY':'1','DISABLE_ERROR_REPORTING':'1','ENABLE_CLAUDEAI_MCP_SERVERS':'false',
           'API_TIMEOUT_MS':'10000','GOTOOLCHAIN':'go1.26.9','GOCACHE':'/private/tmp/ks-corpus-go-cache'}
    if 'HOME'in os.environ:
        env['HOME']=os.environ['HOME']
    marker='manager-'+case
    prompt='/knowledge-manager:manage KS_HOST_PROBE:'+marker+'. Controlled local transport fixture; use only the registered project and actual supplied test response.'
    argv = [cli,'--print','--model','claude-sonnet-4-6','--max-budget-usd','0.10','--plugin-dir',str(PLUGIN),
            '--strict-mcp-config','--mcp-config',str(out/'mcp.json'),'--setting-sources','','--settings',str(out/'settings.json'),
            '--permission-mode','manual','--permission-prompts','none','--include-hook-events','--output-format','stream-json','--verbose',
            '--no-session-persistence','--no-chrome','--tools','Skill','--system-prompt','Local scripted transport test only; no real model or independent acceptance.',prompt]
    actions = actions_factory(case,subject,out)
    gateway = transport.Gateway(marker,actions,out)
    actions.gateway=gateway
    with gateway as url:
        env['ANTHROPIC_BASE_URL']=url
        write(out/'invocation.json',{'argv':argv,'cwd':str(subject['source']),'env':{k:v for k,v in env.items()if k not in ('HOME','PATH','ANTHROPIC_API_KEY')},
            'credential_kind':'constant_fake_local_token','MCP_source_sha256':sha(PLUGIN/'.mcp.json'),'resolved_declared_command':command,
            'scope':'actual_CLI_with_local_scripted_API_not_real_model','real_model_calls':0})
        with(out/'host.stdout.jsonl').open('w')as stdout_file,(out/'host.stderr.txt').open('w')as stderr_file:
            p = subprocess.Popen(argv,cwd=subject['source'],env=env,stdout=stdout_file,stderr=stderr_file,text=True,start_new_session=True)
            killed=False
            if case=='kill':
                if actions.kill_ready.wait(timeout=90):
                    p.kill()
                    killed=True
                    actions.kill_release.set()
            try:
                p.wait(timeout=120)
            except subprocess.TimeoutExpired:
                os.killpg(p.pid,signal.SIGTERM)
                try:
                    p.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(p.pid,signal.SIGKILL)
                    p.wait(timeout=5)
                raise ValueError('actual host exceeded owned fixture deadline')
    stdout=(out/'host.stdout.jsonl').read_text()
    deadline=time.monotonic()+20
    while time.monotonic()<deadline:
        frames=out/'mcp-frames.jsonl'
        rows=[json.loads(line)for line in frames.read_text().splitlines()]if frames.exists()else[]
        if any(row['kind']=='exit'for row in rows):
            break
        time.sleep(.1)
    host=[]
    lines=stdout.splitlines(keepends=True)
    for index,line in enumerate(lines):
        if not line.startswith('{'):
            continue
        try:
            host.append(json.loads(line))
        except ValueError:
            require(case=='kill'and index==len(lines)-1 and not line.endswith('\n'),'unexpected truncated host event')
    init=next((row for row in host if row.get('subtype')=='init'),{})
    expected_server='failed'if case=='missing'else 'connected'
    require(any(row['name']=='plugin_knowledge-manager_cks-manager'and row['status']==expected_server for row in init.get('mcp_servers',[])),'actual host did not observe expected manager server status')
    require(any(row['name']=='knowledge-manager'for row in init.get('plugins',[]))and 'knowledge-manager:manage'in init.get('skills',[]),'actual plugin/skill not registered')
    require('knowledge-manager:knowledge-manager'in init.get('agents',[]),'actual manager agent definition not registered')
    require(any('<command-message>knowledge-manager:manage</command-message>'in json.dumps(row['body'].get('messages',[]))
                and 'Manage the selected project'in json.dumps(row['body'].get('messages',[]))for row in gateway.requests),'manual plugin skill was not actually expanded')
    require(p.returncode==(-9 if case=='kill'else 0)and actions.fault is None and actions.done and (case=='missing'or actions.index==len(actions.steps)),
            'actual host workflow failed: '+str(actions.fault))
    exits=[row for row in rows if row['kind']=='exit']
    starts=[row for row in rows if row['kind']=='start']
    require((1<=len(exits)<=4 if case=='missing'else len(exits)==1)and len(starts)==len(exits)
            and {r['child_pid']for r in starts}=={r['child_pid']for r in exits}
            and all(row['exit_code']==(2 if case=='missing'else 0)and row['forced_kill']is False for row in exits),'owned manager shutdown failed: '+str(exits))
    after={p.relative_to(subject['source']).as_posix():sha(p)for p in subject['source'].rglob('*')if p.is_file()}
    require(after==subject['source_before'],'actual host modified original source')
    require(not(subject['root']/'dataset/current').exists(),'actual host published held fixture')
    observation={'case':case,'host_exit_code':p.returncode,'intentional_SIGKILL':killed,'manager_exit_code':exits[0]['exit_code'],'manager_forced_kill':False,
                 'manager_start_attempts':len(starts),'manager_exit_codes':[row['exit_code']for row in exits],
                 'actual_tool_results':len(actions.observed),'API_requests':len(gateway.requests),'plugin_registered':True,'skill_registered':True,
                 'agent_definition_registered':True,'manual_skill_expanded':True,'agent_autonomy_or_model_quality_verified':False,
                 'MCP_status':expected_server,'original_source_unchanged':True,'DB_published':False,'real_model_calls':0,'independent_acceptance':False}
    write(out/'observation.json',observation)
    return observation


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',required=True,type=Path)
    parser.add_argument('--claude',default=shutil.which('claude'))
    parser.add_argument('--case',choices=('all','full','insufficient','missing','active-stop','kill'),default='all')
    args=parser.parse_args()
    out=args.out.resolve()
    require(not out.exists()and args.claude,'fresh output and installed Claude CLI required')
    out.mkdir()
    version=subprocess.check_output([args.claude,'--version'],text=True).strip()
    write(out/'host-version.json',{'version':version,'executable':str(Path(args.claude).resolve()),'sha256':sha(Path(args.claude).resolve())})
    spec=importlib.util.spec_from_file_location('manager_host_transport',TRANSPORT)
    transport=importlib.util.module_from_spec(spec)
    spec.loader.exec_module(transport)
    cases=('full','insufficient','missing','active-stop','kill')if args.case=='all'else(args.case,)
    observations=[]
    for case in cases:
        subject=fixture(out/(case+'-fixture'),case)
        observations.append(run_host(args.claude,subject,case,out/case,transport))
        print('PASS: actual Claude host '+case,flush=True)
        if case=='full':
            observations.append(run_host(args.claude,subject,'restart',out/'full-restart',transport))
            print('PASS: actual Claude host persisted reader restart',flush=True)
        if case in ('active-stop','kill'):
            require(subject['workflow'].get('inflight_graph_observed'),'actual graph was not observed before host termination')
            pid=int((subject['root']/'graph-started').read_text().strip())
            deadline=time.monotonic()+5
            gone=False
            while time.monotonic()<deadline:
                try:
                    os.kill(pid,0)
                except ProcessLookupError:
                    gone=True
                    break
                time.sleep(.1)
            require(gone,'owned delayed graph child survived manager shutdown')
            write(subject['root']/'graph-quiescence.json',{'owned_graph_pid':pid,'process_gone_before_recovery':True,'signal_used_for_this_check':0})
            (subject['root']/'allow-graph').write_text('Allow the known real graph binary for the next recovery version.\n')
            observations.append(run_host(args.claude,subject,'recover',out/(case+'-recover'),transport))
            print('PASS: actual Claude host canceled receipt/replay and new real build after '+case,flush=True)
    summary={'schema_version':1,'scope':'actual_Claude_Code_local_scripted_transport_not_real_model_quality','host_version':version,'runs':len(observations),
             'observations':observations,'real_model_calls':0,'real_model_cost_USD':0,'mock_embedding_not_quality':True,
             'all_requested_cases':args.case=='all','KM03_4_complete':args.case=='all','full_KM03_complete':args.case=='all','independent_acceptance':False}
    write(out/'summary.json',summary)
    print(json.dumps(summary))


if __name__=='__main__':
    main()
