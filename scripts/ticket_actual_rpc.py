"""Bound real in-process RPC DEV paths; no independent candidate acceptance."""
import argparse,gzip,hashlib,json,re,shutil,subprocess
from pathlib import Path
import wi4_target_schedules as prior
import ticket_dev_gold as draft
from source_packet import oid,verify
ROOT,BASE=prior.ROOT,prior.BASE
PACKET=BASE/'followups/ticket-actual-rpc-v1'
REVISION='f91559fd59ad9b332fc22152f2540e29ccec9b1e'
KIND='WIV1_V2_actual_inprocess_RPC_DEV_not_independent_acceptance'
COMMIT=prior.COMMIT
ROLES=['ORIGINAL','CAND-A','CAND-B','MUT-REINTRO','MUT-EARLY']
CASES=['RPC-live-normal','RPC-ack-held','RPC-status-before-request']
API='eth/downloader/api.go';TEST='eth/downloader/ticket_owned_rpc_test.go'
TOOLS=['scripts/'+n for n in ('ticket_actual_rpc.py','test_ticket_actual_rpc.py','run-ticket-actual-rpc.py','fixtures/ticket-owned-rpc.go.fixture','run-wi4-target-subscription.py','wi4_target_schedules.py','ticket_dev_gold.py','source_packet.py','check-operational-acceptance.py','test_operational_acceptance.py','run-ticket-rpc-startup-recovery.py')]
write,read,raw,sha=prior.write,prior.read,prior.raw,prior.sha
native_record=prior.prior.prior.native_reuse_record


def flat_map(rows):
    result={}
    for r in rows:
        p=Path(r['path'])
        if r['path']in ('','.')or '\0'in r['path']or p.as_posix()!=r['path']or p.is_absolute()or any(x in ('..','.')for x in p.parts)or r['path']in result or not re.fullmatch('[0-9a-f]{40}',r['oid']):raise ValueError('unsafe/duplicate Git tree path')
        if (r['mode'],r['kind'])not in [('100644','blob'),('100755','blob'),('120000','blob'),('160000','commit')]:raise ValueError('unexpected tree mode/type')
        result[r['path']]=dict(mode=r['mode'],kind=r['kind'],oid=r['oid'])
    return result


def tree_oid(rows):
    root={}
    for path,r in flat_map(rows).items():
        node=root;parts=path.split('/')
        for name in parts[:-1]:
            if name in node and not isinstance(node[name],dict):raise ValueError('Git tree prefix collision')
            node=node.setdefault(name,{})
        if parts[-1]in node:raise ValueError('Git tree prefix collision')
        node[parts[-1]]=(r['mode'],r['oid'])
    def build(node):
        entries=[]
        for name,value in node.items():
            directory=isinstance(value,dict);mode,identity=('40000',build(value))if directory else value
            entries.append((name.encode()+ (b'/'if directory else b''),mode.encode()+b' '+name.encode()+b'\0'+bytes.fromhex(identity)))
        return oid('tree',b''.join(v for _,v in sorted(entries)))
    return build(root)


def stack_frame(text,*parts):return any(all(p in g for p in parts)for g in text.split('\n\n'))


def inspect_incomplete(q,slot,binary,fixture_sha,partial=False):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    if any(r.get(k)!=v for k,v in {**process,**slot}.items())or slot['variant']!='race':raise ValueError('unentered process identity differs')
    if r['binary_sha256']!=binary['sha256']or r['fixture_sha256']!=fixture_sha or r['argv']!=[binary['path'],'-test.v=true','-test.run=^TestTicketOwnedRPC$','-test.count=1','-test.timeout=20s']:raise ValueError('unentered binary/fixture differs')
    if r['exit_code']!=-15 or not r['timeout']or not r['term_sent']or r['kill_sent']or not r['owned_new_session']or not r['reaped']or r['remaining_live_group_members']or r['tmp_children_after']or not 25<=r['elapsed_seconds']<35:raise ValueError('unentered timeout/cleanup differs')
    stdout=raw(q/'process/stdout.gz');stderr=raw(q/'process/stderr.gz')
    if sha(stdout)!=r['stdout_sha256']or sha(stderr)!=r['stderr_sha256']or stderr:raise ValueError('incomplete raw output cannot be invented or suppressed')
    row=dict(**slot,attempt_plan='initial',classification='external_timeout_without_test_entry',valid_technical_observation=False,actual_RPC_requests=0,direct_non_RPC_notifier_controls=0,route=None,observed=None,independent_ticket=False,independent_oracle=False,independent_acceptable_membership=None,quality_score=None,internal_callback_cleanup_proved=False,owned_process_group_reaped=True)
    if not partial:
        if stdout or any(q.glob('events.jsonl*')):raise ValueError('entered/partial trace cannot be classified as unentered')
        return row
    if stdout!=b'=== RUN   TestTicketOwnedRPC\n'or slot['case_id']!='RPC-status-before-request':raise ValueError('partial test entry differs')
    events=[json.loads(l)for l in raw(q/'events.jsonl.gz').decode().splitlines()];names=[e['event']for e in events];by={e['event']:e for e in events}
    expected=['trace_begin','unsupported_notifier_control','unknown_subscription_control','actual_RPC_subscribed','controlled_backend_install','actual_callback_installed','foreign_session_control','actual_RPC_unsubscribe_true','repeat_unsubscribe_control','status_without_backend_receiver_window_begin']
    if slot['candidate']in ('CAND-A','CAND-B'):expected+=['owned_status_sender_returned']
    if names!=expected or [e['sequence']for e in events]!=list(range(1,len(events)+1))or [e['elapsed_ns']for e in events]!=sorted(e['elapsed_ns']for e in events)or any(e['candidate']!=slot['candidate']or e['case_id']!=slot['case_id']for e in events):raise ValueError('partial trace/order changed or falsely completed')
    begin=by['trace_begin']
    if begin['base_commit']!=COMMIT or begin['transport']!='rpc.DialInProc_net.Pipe'or begin['step_watchdog_ms']!=2000 or begin['opened_listener']is not False or begin['Ethereum_factory_started']is not False:raise ValueError('partial RPC transport/host scope falsely promoted')
    if by['unsupported_notifier_control']['typed_error']is not True or by['unsupported_notifier_control']['RPC_request']is not False or by['unknown_subscription_control']['error']is not True:raise ValueError('partial controls changed')
    sub=by['actual_RPC_subscribed'];sid=sub['subscription_id']
    if not re.fullmatch('0x[0-9a-f]+',sid)or sub['namespace']!='eth'or sub['subscription']!='syncing'or sub['backend']!='controlled_backend'or by['controlled_backend_install']['status_capacity']!=0:raise ValueError('partial real subscription/control backend differs')
    if any(by[n][k]!=sid for n,k in [('foreign_session_control','same_id'),('actual_RPC_unsubscribe_true','subscription_id'),('repeat_unsubscribe_control','same_id')])or by['actual_RPC_unsubscribe_true']['same_session']is not True or any(by[n]['subscription_not_found']is not True for n in ('foreign_session_control','repeat_unsubscribe_control')):raise ValueError('partial RPC session/id mismatch')
    installed=by['actual_callback_installed']
    if installed['callback_count']!=1 or installed['live_eventLoop']is not False or installed['progress_notification_exercised']is not False or not stack_frame(raw(q/'events.jsonl.callback-installed.stack.gz').decode(),'(*DownloaderAPI).Syncing.func1','/eth/downloader/api.go:','[select]'):raise ValueError('partial actual callback not proved')
    return dict(row,classification='external_timeout_during_observation',actual_RPC_requests=5,direct_non_RPC_notifier_controls=1,route='controlled_backend_partial_not_frozen',fragment_events=names,subscription_id_sha256=sha(sid.encode()),trace_sha256=sha(raw(q/'events.jsonl.gz')))


def inspect_slot(q,slot,binary,fixture_sha,trees):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    if any(r.get(k)!=v for k,v in {**process,**slot}.items()):raise ValueError('RPC slot/process identity differs')
    if r['binary_sha256']!=binary['sha256']or r['fixture_sha256']!=fixture_sha:raise ValueError('RPC binary/fixture changed')
    if r['argv']!=[binary['path'],'-test.v=true','-test.run=^TestTicketOwnedRPC$','-test.count=1','-test.timeout=20s']or r['exit_code']or r['timeout']or r['term_sent']or r['kill_sent']or not r['owned_new_session']or not r['reaped']or r['remaining_live_group_members']or r['tmp_children_after']:raise ValueError('RPC slot failed or cleanup unproved; preserve actual failure')
    for ext in ('stdout','stderr'):
        if sha(raw(q/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('RPC raw output changed')
    rows=[json.loads(l)for l in raw(q/'events.jsonl.gz').decode().splitlines()if l.strip()];names=[e['event']for e in rows]
    if len(set(names))!=len(names)or [e['sequence']for e in rows]!=list(range(1,len(rows)+1))or any(e['case_id']!=slot['case_id']or e['candidate']!=slot['candidate']for e in rows)or [e['elapsed_ns']for e in rows]!=sorted(e['elapsed_ns']for e in rows):raise ValueError('RPC trace scope/order differs')
    by={e['event']:e for e in rows}
    def ordered(*labels):
        positions=[names.index(n)for n in labels]
        if positions!=sorted(positions):raise ValueError('RPC causal ordering differs')
    begin=by['trace_begin'];obs=by['observation_frozen'];cid,role=slot['case_id'],slot['candidate'];live=cid=='RPC-live-normal';early=role=='MUT-EARLY'
    if begin['base_commit']!=COMMIT or begin['transport']!='rpc.DialInProc_net.Pipe'or begin['step_watchdog_ms']!=2000 or begin['opened_listener']is not False or begin['Ethereum_factory_started']is not False:raise ValueError('RPC transport/host scope falsely promoted')
    if by['trace_end']['test_failed']is not False or by['cleanup_end']!={**{k:by['cleanup_end'][k]for k in ('event','case_id','candidate','sequence','elapsed_ns')},'joined':True,'remaining_callbacks':0,'remaining_event_loops':0}:raise ValueError('RPC cleanup unproved')
    ordered('trace_begin','unsupported_notifier_control','unknown_subscription_control','actual_RPC_subscribed','actual_callback_installed','foreign_session_control','actual_RPC_unsubscribe_true','repeat_unsubscribe_control','observation_frozen','cleanup_begin','cleanup_end','trace_end')
    if by['unsupported_notifier_control']['typed_error']is not True or by['unsupported_notifier_control']['RPC_request']is not False or by['unknown_subscription_control']['error']is not True:raise ValueError('unsupported/unknown control falsely accepted')
    sid=by['actual_RPC_subscribed']['subscription_id']
    if not isinstance(sid,str)or not re.fullmatch('0x[0-9a-f]+',sid)or any(by[n].get(k)!=sid for n,k in [('foreign_session_control','same_id'),('actual_RPC_unsubscribe_true','subscription_id'),('repeat_unsubscribe_control','same_id')]):raise ValueError('RPC session/subscription id mismatch')
    if by['actual_RPC_subscribed']['namespace']!='eth'or by['actual_RPC_subscribed']['subscription']!='syncing'or by['actual_RPC_subscribed']['backend']!=('actual_eventLoop'if live else'controlled_backend'):raise ValueError('RPC registration/backend route differs')
    if by['actual_RPC_unsubscribe_true']['same_session']is not True or any(by[n]['subscription_not_found']is not True for n in ('foreign_session_control','repeat_unsubscribe_control')):raise ValueError('cross-session or registry-removal control failed')
    if by['actual_callback_installed']['callback_count']!=1 or by['actual_callback_installed']['live_eventLoop']!=live or by['actual_callback_installed']['progress_notification_exercised']is not False:raise ValueError('callback/notification coverage falsely promoted')
    installed=raw(q/'events.jsonl.callback-installed.stack.gz').decode()
    if not stack_frame(installed,'(*DownloaderAPI).Syncing.func1','/eth/downloader/api.go:','[select]'):raise ValueError('actual installed Syncing callback not proved in same goroutine')
    acknowledgements=[e for e in rows if e['event']=='controlled_ack_closed']
    if live:
        if obs['route']!='actual_eventLoop'or obs['callback_returned']is not True or obs['backend_membership_proved']is not False or obs['native_progress_timer_fired']is not False or acknowledgements:raise ValueError('live normal result falsely became full progress/membership proof')
    else:
        if by['controlled_backend_install']['status_capacity']!=0 or obs['route']!='controlled_backend'or obs['backend_membership_proved']is not False or obs['observation_ms']!=2000:raise ValueError('controlled install falsely became full backend proof')
        ordered('actual_RPC_subscribed','controlled_backend_install','actual_callback_installed')
        if len(acknowledgements)!=(0 if early else 1)or any(e['same_status_channel']is not True or not by['cleanup_begin']['elapsed_ns']<=e['elapsed_ns']<by['cleanup_end']['elapsed_ns']for e in acknowledgements):raise ValueError('held ack released before frozen observation or foreign channel')
        snapshot=raw(q/'events.jsonl.observation.stack.gz').decode()
        if cid=='RPC-ack-held':
            label='no_request_window_begin'if early else'ack_withheld_window_begin'
            ordered('repeat_unsubscribe_control',label,'observation_frozen')
            if obs['elapsed_ns']-by[label]['elapsed_ns']<2_000_000_000 or obs['request_received']!= (not early)or obs['callback_returned']!=early:raise ValueError('RPC true conflated with downstream method completion')
            if not early:
                if by['actual_backend_request_received']['same_status_channel']is not True or by['actual_backend_request_received']['ack_closed']is not False:raise ValueError('controlled request identity/ack differs')
                for phase in ('before-observation','observation'):
                    if not stack_frame(raw(q/('events.jsonl.'+phase+'.stack.gz')).decode(),'(*DownloaderAPI).Syncing.func1','(*SyncStatusSubscription).Unsubscribe','/eth/downloader/api.go:'):raise ValueError('actual deferred Unsubscribe wait missing')
        elif cid=='RPC-status-before-request':
            window=by['status_without_backend_receiver_window_begin']
            if obs['elapsed_ns']-window['elapsed_ns']<2_000_000_000 or obs['status_drained_before_request_receive']!=(role in ('CAND-A','CAND-B'))or obs['callback_returned']!=early:raise ValueError('actual status-before-request observation differs')
            sender=by['owned_status_sender_returned']
            if role in ('CAND-A','CAND-B'):
                if sender['elapsed_ns']>=obs['elapsed_ns']:raise ValueError('candidate drain happened only after cleanup rescue')
            elif not by['cleanup_begin']['elapsed_ns']<=sender['elapsed_ns']<by['cleanup_end']['elapsed_ns']:raise ValueError('source blocked sender outcome lost before cleanup')
            if not early and not stack_frame(snapshot,'(*DownloaderAPI).Syncing.func1','(*SyncStatusSubscription).Unsubscribe','/eth/downloader/api.go:'):raise ValueError('actual source method before request not proved')
        else:raise ValueError('RPC scenario outside declared plan')
    return dict(**slot,actual_RPC_requests=5,direct_non_RPC_notifier_controls=1,subscription_id_sha256=sha(sid.encode()),valid_technical_observation=True,route=obs['route'],observed=obs,binary_sha256=binary['sha256'],driver_sha256=fixture_sha,trace_sha256=sha(raw(q/'events.jsonl.gz')),applied_project_tree=trees['applied_project_tree'],build_input_tree=trees['build_input_tree'],independent_ticket=False,independent_oracle=False,full_12_requirement_matching=False,independent_acceptable_membership=None,quality_score=None)


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;q=p/'actual';scope=read(p/'SCOPE.json');fixture=(root/TOOLS[3]).read_bytes();fixture_sha=sha(fixture)
    slots=[dict(slot_id=f'{role}-{v}-r{r}-{case}',candidate=role,variant=v,repetition=r,case_id=case)for role in ROLES for v in ('general','race')for r in (1,2,3)for case in CASES]
    if scope['planned_slots']!=slots or read(q/'planned.json')!=slots or read(q/'SCOPE.json')!=scope or scope['RPC_requests_per_slot']!=5:raise ValueError('predeclared RPC plan differs')
    if any(scope.get(k)!=v for k,v in dict(observation_watchdog_ms=2000,test_timeout_seconds=20,owned_process_timeout_seconds=25,parallelism=2,retries=0).items()):raise ValueError('initial timing or retry contract changed')
    if scope['security']!=dict(external_network=False,operational_node=False,system_clock_change=False,global_settings=False,credential_environment_inheritance=False,publication=False,paid_or_external_LLM=False,owned_process_group_only=True):raise ValueError('owned/offline scope changed')
    if scope['acceptance_limits']!=dict(independent_approved=0,independent_tickets=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,independent_acceptable_candidate_set=None,canonical_selection=None,quality_score=None):raise ValueError('RPC scope falsely accepted')
    env=read(q/'environment.json')
    if env['credential_environment_inherited']is not False or set(env['allowed_inherited_keys'])-{'HOME','USER','TMPDIR','LANG','LC_ALL'}or any(env['task_settings'].get(k)!=v for k,v in dict(GOPROXY='off',GOSUMDB='off',GOVCS='*:off',GOTOOLCHAIN='local',GOFLAGS='-mod=readonly',GOMODCACHE='/Users/kevin/.gvm/pkgsets/go1.23.12/global/pkg/mod').items()):raise ValueError('offline/credential boundary differs')
    expected=read(root/BASE/'followups/wi4-target-subscription-v1/initial/source-binding.json')['tracked'];candidates={c['id']:c for c in read(root/draft.PACKET/'private/candidates.json')}
    proof=verify(root/BASE/'schedule-candidates-v1',COMMIT);commit=(root/BASE/'schedule-candidates-v1'/proof['objects']['commit:'+COMMIT]).read_bytes();base_tree=commit.splitlines()[0].decode().removeprefix('tree ')
    built={};tree_records={};post=read(q/'binary-postrun-check.json');all_base=None
    if set(post)!=set(ROLES):raise ValueError('candidate binary role population differs')
    for role in ROLES:
        c=candidates[role];d=q/'candidates'/role;binding=read(d/'source-binding.json');trees=read(d/'tree-binding.json');overlay=(root/draft.PACKET/'private/overlays'/(role+'.go.txt')).read_bytes();applied=dict(expected);applied[API]=dict(mode=expected[API]['mode'],sha256=sha(overlay))
        if sha(overlay)!=c['full_file_overlay_sha256']or binding!=dict(candidate_id=role,base_commit=COMMIT,base_tracked=expected,applied_tracked=applied,api_sha256=sha(overlay),method_sha256=c['method_sha256'],fixture_sha256=fixture_sha,test_addition=TEST,production_patch=False):raise ValueError('candidate source/fixture binding differs')
        flat=read(d/'base-flat-tree.json');baseline=flat_map(flat)
        if tree_oid(flat)!=base_tree or len(flat)!=len(expected)or set(baseline)!=set(expected):raise ValueError('base full Git tree not bound to retained commit')
        if all_base is None:all_base=flat
        if flat!=all_base:raise ValueError('candidate base trees differ')
        project=dict(baseline);project[API]=dict(mode='100644',kind='blob',oid=oid('blob',overlay));build=dict(project);build[TEST]=dict(mode='100644',kind='blob',oid=oid('blob',fixture))
        if trees['base_commit']!=COMMIT or trees['applied_project_commit']is not None or trees['source_mode']!='owned_working_tree_with_real_Git_tree_objects_not_candidate_commit'or trees['api_blob']!=oid('blob',overlay)or trees['test_blob']!=oid('blob',fixture)or flat_map(trees['project_flat_tree'])!=project or flat_map(trees['build_flat_tree'])!=build:raise ValueError('real applied project/build tree differs')
        if tree_oid(trees['project_flat_tree'])!=trees['applied_project_tree']or tree_oid(trees['build_flat_tree'])!=trees['build_input_tree']:raise ValueError('Git tree object hash differs')
        if read(d/'profile.json')!=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'):raise ValueError('actual candidate toolchain differs')
        binaries=read(d/'binaries.json')
        if set(binaries)!={'general','race'}or post[role]!={v:b['sha256']for v,b in binaries.items()}:raise ValueError('candidate postrun binary differs')
        for v,b in binaries.items():
            br=read(d/('build-'+v)/'receipt.json');argv=br['argv']
            if argv!=[argv[0],'test']+(['-race']if v=='race'else[])+['-c','-o',b['path'],'./eth/downloader']or br['exit_code']or br['timeout']or not br['reaped']or br['remaining_live_group_members']:raise ValueError('candidate build failed/narrowed')
            for ext in ('stdout','stderr'):
                if sha(raw(d/('build-'+v)/(ext+'.gz')))!=br[ext+'_sha256']:raise ValueError('candidate build raw output differs')
        built[role]=binaries;tree_records[role]=trees
    if len({candidates[r]['method_sha256']for r in ROLES})!=4 or len({tree_records[r]['applied_project_tree']for r in ROLES})!=4 or tree_records['ORIGINAL']['applied_project_tree']!=tree_records['MUT-REINTRO']['applied_project_tree']:raise ValueError('role aliases inflated into independent variants')
    receipt=read(q/'receipt.json')
    if receipt!=dict(planned=90,attempted=90,retries=0,tracked_changes_only_exact_candidate_api=True,test_addition_only=True,production_patch=False,source_commit=COMMIT,independent_approved=0,independent_tickets=0,FINAL=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0):raise ValueError('all-attempt candidate scope differs')
    attempts=read(q/'attempts.json')
    if len(attempts)!=90:raise ValueError('planned RPC attempt missing')
    rows=[]
    for slot,attempt in zip(slots,attempts):
        d=q/'slots'/slot['slot_id']
        if read(d/'receipt.json')!=attempt:raise ValueError('RPC attempt aggregate differs')
        if attempt['timeout']:row=inspect_incomplete(d,slot,built[slot['candidate']]['race'],fixture_sha,partial=(d/'events.jsonl.gz').exists())
        else:row=dict(inspect_slot(d,slot,built[slot['candidate']][slot['variant']],fixture_sha,tree_records[slot['candidate']]),attempt_plan='initial',classification='observed')
        row['environment_sha256']=sha((q/'environment.json').read_bytes());rows.append(row)
    recovery=p/'recovery';rs=read(p/'RECOVERY-SCOPE.json');race_slots=[s for s in slots if s['slot_id']in {r['slot_id']for r in rows if not r['valid_technical_observation']}]
    partial_ids=[r['slot_id']for r in rows if r['classification']=='external_timeout_during_observation'];unentered=[r for r in rows if r['classification']=='external_timeout_without_test_entry'];initial_race_valid=[r['slot_id']for r in rows if r['variant']=='race'and r['valid_technical_observation']]
    if len(unentered)!=41 or len(partial_ids)!=3 or rs['initial_partial_slots']!=partial_ids or rs['initial_unentered_timeouts']!=41 or rs['initial_partial_timeouts']!=3 or rs['already_observed_initial_race_slots']!=initial_race_valid or initial_race_valid!=['MUT-EARLY-race-r3-RPC-status-before-request']:raise ValueError('initial complete/partial/unentered denominator differs')
    for name,scope_name,runner_name in [('preflight1','RECOVERY-INITIAL-SCOPE.json','runner-before-population-guard.py.txt'),('preflight2','RECOVERY-SECOND-SCOPE.json','runner-before-fragment-guard.py.txt')]:
        d=p/name;r=read(d/'receipt.json')
        if read(d/'SCOPE.json')!=read(p/scope_name)or set(x.name for x in d.iterdir())!={'SCOPE.json',runner_name,'stderr.txt','receipt.json'}or r['exit_code']!=1 or r['target_attempts']!=0 or r['scope_sha256']!=sha((d/'SCOPE.json').read_bytes())or r['runner_sha256']!=sha((d/runner_name).read_bytes())or r['stderr_sha256']!=sha((d/'stderr.txt').read_bytes())or 'population differs'not in(d/'stderr.txt').read_text():raise ValueError('preflight guard failure/zero-attempt evidence missing')
    if rs!=read(recovery/'SCOPE.json')or rs['planned_slots']!=race_slots or rs['planned_attempts']!=44 or rs['corrective_reattempts']!=44 or rs['owned_process_timeout_seconds']!=150 or rs['test_timeout_seconds']!=20 or rs['observation_watchdog_ms']!=2000 or rs['parallelism']!=2 or rs['automatic_retries']!=0 or rs['same_source_fixture_binary']is not True or rs['initial_failures_preserved']is not True or rs['no_removed_or_rebuilt_original_tests']is not True:raise ValueError('corrective predeclared startup-only budget differs')
    if read(recovery/'environment.json')!=env or read(recovery/'planned.json')!=race_slots or read(recovery/'reused-binaries.json')!={role:built[role]['race']for role in ROLES}:raise ValueError('corrective environment/binary/plan differs')
    if read(recovery/'receipt.json')!=dict(planned=44,attempted=44,corrective_reattempts=44,automatic_retries=0,initial_attempts_retained=90,same_source_fixture_binary=True,observation_watchdog_ms=2000,test_timeout_seconds=20,owned_process_timeout_seconds=150,operational_nodes_started=0,paid_or_external_LLM_calls=0,independent_approved=0,formal_completed=0,formal_total=9):raise ValueError('corrective denominator or scope differs')
    attempts=read(recovery/'attempts.json')
    if len(attempts)!=44:raise ValueError('corrective planned attempt missing')
    for slot,attempt in zip(race_slots,attempts):
        d=recovery/'slots'/slot['slot_id']
        if read(d/'receipt.json')!=attempt or attempt['elapsed_seconds']>=150:raise ValueError('corrective actual attempt differs')
        row=dict(inspect_slot(d,slot,built[slot['candidate']]['race'],fixture_sha,tree_records[slot['candidate']]),attempt_plan='explicit_startup_budget_correction',classification='observed');row['environment_sha256']=sha((recovery/'environment.json').read_bytes());rows.append(row)
    samples=list(q.glob('slots/*/process/startup-sample.txt'))
    if len(samples)!=2 or any(not all(frame in x.read_text()for frame in ('eth/downloader.init.2','newTestBlockchain','InsertChain','__tsan'))for x in samples):raise ValueError('owned startup stack diagnosis missing')
    valid=[r for r in rows if r['valid_technical_observation']]
    return dict(summary=dict(kind=KIND,planned_attempts=134,actual_attempts=134,initial_attempts=90,initial_general_observed=45,initial_race_observed=1,initial_race_unentered_timeouts=41,initial_partial_observation_timeouts=3,explicit_corrective_race_reattempts=44,corrective_race_observed=44,corrective_preflight_guard_refusals=2,corrective_preflight_target_attempts=0,valid_technical_observations=len(valid),all_initial_attempts_passed=False,actual_caller_RPC_requests=sum(r['actual_RPC_requests']for r in rows),direct_non_RPC_notifier_controls=sum(r['direct_non_RPC_notifier_controls']for r in rows),actual_live_eventLoop_slots=30,controlled_backend_slots=60,controlled_backend_partial_slots=3,caller_RPC_requests_from_complete_observations=450,caller_RPC_requests_from_partial_traces=15,candidate_roles=5,unique_method_variants=4,unique_applied_project_trees=4,exposed_DEV_tickets=1,independent_tickets=0,full_12_requirement_matching=False,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,independent_acceptable_candidate_set=None,canonical_selection=None,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0,full_Ethereum_factory_or_installed_role=False),observations=rows)


def historical_errors(root=ROOT):
    errors=prior.historical_errors(root);p=root/prior.PACKET;m=read(p/'manifest.json')
    if(p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(prior.PACKET/'manifest.json')],cwd=root):errors.append('stage63 manifest changed')
    for n,h in m['artifacts'].items():
        if sha((p/n).read_bytes())!=h:errors.append('stage63 evidence changed '+n)
    for r in m['tools']:
        if sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('stage63 tool at signed revision differs')
    return errors



def seal(p):write(p/'manifest.json',dict(kind=KIND,previous_source_revision=REVISION,tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={x.relative_to(p).as_posix():sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))



def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=sha((root/PACKET/'manifest.json').read_bytes()),status=KIND,report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))



def update_indices():
    p=ROOT/PACKET;seal(p);ref=reference();state=read(p/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);base=ROOT/'docs/spec-driven'
    proposal=base/'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json';d=read(proposal);d.update(ticket_actual_rpc=ref,wi4_target_schedules=dict(d['wi4_target_schedules'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state);write(proposal,d)
    link=dict(path=str(proposal.relative_to(ROOT)),sha256=sha(proposal.read_bytes()),status='D04_review_pending_WI4_actual_ticket_RPC_DEV_not_independent_acceptance')
    for name in ('OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','operational-preparation/source-review-supplement/GAPS.json'):
        q=base/name;d=read(q);d.update(ticket_actual_rpc=ref,wi4_target_schedules=dict(d['wi4_target_schedules'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state,adoption_acceptance_decision_proposal=link)
        if q.name=='OPERATIONAL-ACCEPTANCE-PROGRESS.json':d['phases'][0]['evidence'].update(ticket_actual_rpc=ref,wi4_target_schedules=d['wi4_target_schedules'],current_readiness_preparation=current,adoption_acceptance_decision_proposal=link)
        write(q,d)



def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[read(root/n)for n in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('ticket_actual_rpc')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current ticket_RPC reference/state differs')
        if d.get('reviewer_identity_policy')!=old.get('reviewer_identity_policy'):errors.append('privacy policy changed')
        for k,v in old.items():
            if k=='current_readiness_preparation'or not isinstance(v,dict)or'manifest'not in v:continue
            expected=dict(v)
            if k=='wi4_target_schedules':expected['historical_source_revision']=REVISION
            if d.get(k)!=expected:errors.append('historical preparation changed '+k)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical ticket_RPC probes cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('current proposal link differs')
    return errors




def worklist(root=ROOT):
    state=read(root/prior.PACKET/'WORKLIST.json');state.update(current_stage='64_WIV1_V2_actual_inprocess_RPC_candidate_paths',completed_in_this_scope=['5_roles_4_exact_method_variants_applied_in_owned_source_with_real_project_and_build_tree_objects','134attempts_initial46observed41unentered3partial_then44same_binary_corrections_90valid465RPC93direct','live_eventLoop_separate_from_controlled_backend_rpc_true_separate_from_deferred_completion'],next_task='WI_V1_V2_remaining_notification_original_schedules_full_requirement_oracle_and_installed_role_debts_then_WI6',last_verified_date='2026-10-19')
    optional=state['remaining_all'][-1]
    tasks=[
        ('WI-V1','actual notification/progress/native timer and full Ethereum factory; original5 schedules and complete12 requirement matching; independent ticket/primary and derived oracle; installed4roles/finalHEAD'),
        ('WI-V2','independent acceptable candidate set remains null; role aliases are not independent variants; complete primary/derived oracle and full ticket requirements before membership/selection'),
        ('WI-6','nonseed independent families; actual basic/expanded query pairs, complete relevance/noise gold and warm latency criteria'),
        ('WI-SP','actual overlay/consumer path; manual/automatic provenance, policy/security/untagged gold; ACL/private/forgery boundaries and independent symbols'),
        ('WI-C','real review requirements/priorities/required items; structured reports/diagrams/handoff/installation roles; independent semantic gold'),
        ('WI-1','remaining53 occurrences/3blocks; value/type/unknown/memory meaning and actual consumer matching; independent semantic review'),
        ('WI-2','independent samples/meaning; complete gold; actual command/caller/build-profile/support and installed-role/MCP matching'),
        ('WI-3','independent families, exposure/duplicates and gold; actual default budget/startup SLA/site-rejection semantics and partial/provider/out-of-scope cases'),
        ('WI-4','full original driver and real host/receiver/timing/scheduler contract; independent families/oracle, repeated execution criteria and sample sufficiency'),
        ('INDEPENDENT-PREP','anonymous role/review records and independence; new unexposed DEV/FINAL, complete gold/matching/sufficiency and F01~08 freeze; D04 unassigned, no invented approval'),
        ('ONTOLOGY','independent meaning/retrieval gold, reviewed implementation-concept links and family coverage; approved retrieval-effect gates; actual LLM answers only after model/auth/cost cap decision'),
        ('FORMAL-9','formal execution and acceptance WI-1/2/3/4/V1/V2/6/SP/C; currently0of9; only after independently approved frozen inputs'),
    ]
    state['remaining_all']=[dict(id=key,priority=i+1,status='pending',task=value)for i,(key,value)in enumerate(tasks)]+[dict(optional,priority=13)]
    state['priority_order']=[r['id']for r in state['remaining_all']]
    state['AI_DEV_draft_progress']['WI-V1']='one_exposed_shared_DEV_ticket;actual_inprocess_RPC3scenarios_134attempts90observed41unentered3partial_timeouts_retained;full12requirements_and_independent_oracle_pending'
    state['AI_DEV_draft_progress']['WI-V2']='5roles4method_and_project_tree_variants_applied;limited_actual_RPC_outcomes;independent_acceptable_set_null_no_selection'
    return state


def prepare(work,p):
    if any(x.name not in ('SCOPE.json','RECOVERY-SCOPE.json','RECOVERY-INITIAL-SCOPE.json','RECOVERY-SECOND-SCOPE.json')for x in p.iterdir()):raise ValueError('refusing existing evidence overwrite')
    for source,destination in [('rpc-trials-v1','actual'),('rpc-startup-recovery-v3','recovery'),('rpc-startup-recovery-v1','preflight1'),('rpc-startup-recovery-v2','preflight2')]:
        for x in sorted((work/source).rglob('*')):
            if not x.is_file()or x.suffix=='.test':continue
            y=p/destination/x.relative_to(work/source);y.parent.mkdir(parents=True,exist_ok=True)
            if x.name in ('stdout','stderr')or x.suffix in ('.jsonl','.stack'):y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
            else:shutil.copyfile(x,y)
    for role in ROLES:
        for variant,b in read(p/'actual/candidates'/role/'binaries.json').items():
            if sha(Path(b['path']).read_bytes())!=b['sha256']:raise ValueError('actual postrun binary changed '+role+' '+variant)
    d=derive(packet=p)
    if historical_errors():raise ValueError('historical/native source preservation failed')
    write(p/'SUMMARY.json',d['summary']);write(p/'observations.json',d['observations']);write(p/'native-reuse.json',native_record())
    state=worklist();write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI-V1/V2 actual in-process RPC candidate DEV observations; no independent acceptance\n\n'
        'The same already-exposed shared DEV ticket and5 candidate roles/4 distinct method bytes are preserved. Exact retained full-file overlays are applied to five owned checkouts at0bf2f4d1, changing only eth/downloader/api.go where needed and adding one owned test. Each actual project Git tree and separate build input tree (including test) is independently recomputed from complete flat entries, original commit tree and exact blob bytes. No candidate commit is invented. General/race target binaries use declared Go1.23.12 and are bound to source/tree/driver/log hashes.\n'
        'Initial90 fixed slots:5roles x general/race x3 x3 scenarios. Initial general45 reached test/observation; race1 completed,41 exceeded the original external25s budget without recorded test entry, and3 reached all5 caller RPC controls but timed out during observation before freeze/cleanup. Their partial fragments/calls remain counted and internal callback cleanup is unproved; all timeout process groups were SIGTERM/reaped/groups0. The initial run was46 complete observations/3 partial/41 unentered, not all-pass. Two owned OS stack samples identify unchanged downloader test init/newTestBlockchain/InsertChain and race instrumentation. Original tests and source are not removed. An explicit predeclared corrective44-slot plan reused the exact same race binaries/source/fixture and raised only external whole-process budget to150s; test20s and observation2000ms stayed unchanged. Two corrective preflight guards refused before any target execution: first because one initial race slot was already complete, second because3 timed-out slots had partial traces. Both scopes, runner bytes, diagnostics and zero-attempt receipts remain preserved. Thus134 attempts,90 bounded valid observations,41 initial pre-entry timeouts,3 partials,44 explicit corrective reattempts, zero hidden automatic retries. The initial plan was not an all-pass run. Actual rpc.DialInProc/net.Pipe reaches a test-owned eth registration of DownloaderAPI.Syncing. Across90 complete observations and3 partials, caller issues465 RPC requests (450 complete-scope and15 partial): unknown subscription, real subscribe, wrong-session unsubscribe, correct unsubscribe and repeated unsubscribe. Another93 direct no-notifier controls (90 complete-scope and3 partial) are not RPC requests. Same id/session is checked; wrong-session and repeated unsubscribe return subscription-not-found errors. No external listener/network/node/factory is started.\n'
        'Thirty live-eventLoop normal slots use the actual constructor/eventLoop with nil progress dependencies and no StartEvent. No progress notification/native60second timer path is exercised; its timer is not shortened. Callback return is observed; internal backend membership after the early mutant is not inferred. Sixty controlled-backend slots run actual Syncing and candidate deferred Unsubscribe with test-owned install/uninstall channels. They are not complete native eventLoop or Ethereum factory proof.\n'
        'With held backend acknowledgement, RPC true and registry removal occur while the four non-early roles remain in their actual deferred Unsubscribe for2000ms; the empty early mutant returns without a request. With one status send and no backend request receiver, A/B drain before request acceptance; original/reintro and early senders are released only during post-observation cleanup. Source callback/Unsubscribe frames, source channel identity, observation timing and owned cleanup are preserved. An acknowledgement from the controlled backend is not independent proof of native registry removal.\n'
        'These3 correlated scenarios are additional bounded observations, not replacement of the original5 source scenarios or completion of all12 requirements. There is no new independent ticket, independent primary/derived oracle, approved acceptable set, canonical selection, quality/performance/fairness score or FINAL sample. Independent0/FINAL0/formal0of9/freeze null/execution false. D04 remains unassigned; no personal names/contact required.\n'
        'Only90 complete-observation slots prove internal owned clients/server/callbacks/eventLoops/status senders joined after frozen observation and cleanup rescue. The3 initial partial slots do not prove internal callback cleanup. All134 attempt OS process groups are reaped with no remaining members, and owned temporary children are empty. Credentials are not inherited and dependencies resolve offline from an existing local cache. Global install/settings, production activation, publication and paid/external models remain0. No security-violating action was needed in this bounded owned scope; this is not a project-wide security certification.\n'
        'CKS native inputs1738/approved5/config2 remain identical to the latest proof:4158 general and4158 race pass/18skip each are reused, not fresh CKS runs. Ten upstream target binaries are fresh. Remaining full host/progress/original-schedule/installed-role/independent oracle/membership debts, other WI and ontology effect remain in WORKLIST.json. Signed storage is checked against actual Git with --require-stored.\n')
    seal(p)


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=read(p/'manifest.json');actual={x.relative_to(p).as_posix():sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('RPC evidence inventory differs')
        for r in m['tools']:
            if sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('bound RPC tool changed '+r['path'])
        errors+=historical_errors(root);d=derive(root,p)
        if read(p/'SUMMARY.json')!=d['summary']or read(p/'observations.json')!=d['observations']:errors.append('derived RPC observations differ')
        if read(p/'native-reuse.json')!=native_record(root):errors.append('unchanged native reuse differs')
        state=read(p/'WORKLIST.json')
        if state!=worklist(root):errors.append('remaining/priority/current worklist differs from actual bounded scope')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정'or state['independent_quality_or_ontology_acceptance']is not False:errors.append('RPC worklist falsely accepted')
    except (ValueError,KeyError,TypeError,OSError,IndexError)as exc:errors.append(str(exc))
    return errors


def storage_errors(root=ROOT):
    try:
        code=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=root,text=True).strip()
        if not re.fullmatch('[0-9a-f]{40}',code):return ['real signed RPC storage pending']
        names=[PACKET/'manifest.json',*[Path(n)for n in TOOLS],*[Path('docs/spec-driven')/n for n in ('CURRENT-HANDOFF.md','KNOWLEDGE-ROADMAP-TEST-MATRIX.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json')],BASE/'GAPS.json']
        if any(subprocess.check_output(['git','show',code+':'+str(n)],cwd=root)!=(root/n).read_bytes()for n in names):return ['committed RPC tools/evidence differ']
        r=subprocess.run(['git','verify-commit',code],cwd=root,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        return []if r.returncode==0 else ['actual RPC commit signature not verified']
    except (OSError,subprocess.CalledProcessError):return ['real signed RPC storage pending']


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','index','check']);p.add_argument('--workspace',type=Path);p.add_argument('--require-stored',action='store_true');p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args()
    if a.command=='prepare':prepare(a.workspace.resolve(),ROOT/PACKET);update_indices()
    elif a.command=='index':update_indices()
    else:
        errors=validate()+current_indices()
        if a.require_stored:errors+=storage_errors()
        if a.require_formal_gold:errors.append('Exposed in-process RPC candidate observations are not independent approved/frozen gold')
        print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
