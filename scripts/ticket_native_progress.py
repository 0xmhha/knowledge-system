"""Source-bound native60s progress over owned JSON-RPC; not independent acceptance."""
import argparse,gzip,json,re,shutil,subprocess
from pathlib import Path
import ticket_actual_rpc as prior
from source_packet import oid,verify
ROOT,BASE=prior.ROOT,prior.BASE
PACKET=BASE/'followups/ticket-native-progress-v1'
REVISION='0b7dcf0ba65e04d1f39a14270fbaea76a42e3ca9'
KIND='WIV1_V2_native60_progress_RPC_DEV_not_independent_acceptance'
COMMIT=prior.COMMIT
ROLES=prior.ROLES
API='eth/downloader/api.go';TEST='eth/downloader/ticket_owned_native_progress_test.go'
TOOLS=['scripts/'+n for n in ('ticket_native_progress.py','test_ticket_native_progress.py','run-ticket-native-progress.py','fixtures/ticket-native-progress.go.fixture','run-wi4-target-subscription.py','ticket_actual_rpc.py','ticket_dev_gold.py','source_packet.py','check-operational-acceptance.py','test_operational_acceptance.py')]
write,read,raw,sha=prior.write,prior.read,prior.raw,prior.sha
native_record=prior.native_record
flat_map,tree_oid=prior.flat_map,prior.tree_oid
SCOPE_SHA='f66c3c974bbbbecd45e91796ef23b85a381c30537d9472cb22cf1208f2da83da'

def historical_errors(root=ROOT):
    errors=prior.historical_errors(root);p=root/prior.PACKET;m=read(p/'manifest.json')
    if(p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(prior.PACKET/'manifest.json')],cwd=root):errors.append('stage64 manifest changed')
    for n,h in m['artifacts'].items():
        if sha((p/n).read_bytes())!=h:errors.append('stage64 evidence changed '+n)
    for r in m['tools']:
        if sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('stage64 tool at signed revision differs')
    return errors



def seal(p):write(p/'manifest.json',dict(kind=KIND,previous_source_revision=REVISION,tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={x.relative_to(p).as_posix():sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))



def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=sha((root/PACKET/'manifest.json').read_bytes()),status=KIND,report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))



def update_indices():
    p=ROOT/PACKET;seal(p);ref=reference();state=read(p/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);base=ROOT/'docs/spec-driven'
    proposal=base/'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json';d=read(proposal);d.update(ticket_native_progress=ref,ticket_actual_rpc=dict(d['ticket_actual_rpc'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state);write(proposal,d)
    link=dict(path=str(proposal.relative_to(ROOT)),sha256=sha(proposal.read_bytes()),status='D04_review_pending_WIV1_V2_native60_progress_DEV_not_independent_acceptance')
    for name in ('OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','operational-preparation/source-review-supplement/GAPS.json'):
        q=base/name;d=read(q);d.update(ticket_native_progress=ref,ticket_actual_rpc=dict(d['ticket_actual_rpc'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state,adoption_acceptance_decision_proposal=link)
        if q.name=='OPERATIONAL-ACCEPTANCE-PROGRESS.json':d['phases'][0]['evidence'].update(ticket_native_progress=ref,ticket_actual_rpc=d['ticket_actual_rpc'],current_readiness_preparation=current,adoption_acceptance_decision_proposal=link)
        write(q,d)



def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[read(root/n)for n in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('ticket_native_progress')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current ticket_RPC reference/state differs')
        if d.get('reviewer_identity_policy')!=old.get('reviewer_identity_policy'):errors.append('privacy policy changed')
        for k,v in old.items():
            if k=='current_readiness_preparation'or not isinstance(v,dict)or'manifest'not in v:continue
            expected=dict(v)
            if k=='ticket_actual_rpc':expected['historical_source_revision']=REVISION
            if d.get(k)!=expected:errors.append('historical preparation changed '+k)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical ticket_RPC probes cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('current proposal link differs')
    return errors





def bound_sources(root,p):
    q=p/'actual';fixture=(root/TOOLS[3]).read_bytes();fixture_sha=sha(fixture)
    expected=read(root/BASE/'followups/wi4-target-subscription-v1/initial/source-binding.json')['tracked'];candidates={c['id']:c for c in read(root/prior.draft.PACKET/'private/candidates.json')}
    proof=verify(root/BASE/'schedule-candidates-v1',COMMIT);commit=(root/BASE/'schedule-candidates-v1'/proof['objects']['commit:'+COMMIT]).read_bytes();base_tree=commit.splitlines()[0].decode().removeprefix('tree ')
    built={};tree_records={};post=read(q/'binary-postrun-check.json');all_base=None
    if set(post)!=set(ROLES):raise ValueError('candidate binary role population differs')
    for role in ROLES:
        c=candidates[role];d=q/'candidates'/role;binding=read(d/'source-binding.json');trees=read(d/'tree-binding.json');overlay=(root/prior.draft.PACKET/'private/overlays'/(role+'.go.txt')).read_bytes();applied=dict(expected);applied[API]=dict(mode=expected[API]['mode'],sha256=sha(overlay))
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
            if argv!=['/Users/kevin/.gvm/gos/go1.23.12/bin/go','test']+(['-race']if v=='race'else[])+['-c','-o',b['path'],'./eth/downloader']or br['exit_code']or br['timeout']or not br['reaped']or br['remaining_live_group_members']:raise ValueError('candidate build failed/narrowed')
            for ext in ('stdout','stderr'):
                if sha(raw(d/('build-'+v)/(ext+'.gz')))!=br[ext+'_sha256']:raise ValueError('candidate build raw output differs')
        built[role]=binaries;tree_records[role]=trees
    if len({candidates[r]['method_sha256']for r in ROLES})!=4 or len({tree_records[r]['applied_project_tree']for r in ROLES})!=4 or tree_records['ORIGINAL']['applied_project_tree']!=tree_records['MUT-REINTRO']['applied_project_tree']:raise ValueError('role aliases inflated into independent variants')
    return built,tree_records,fixture_sha


EVENTS=['trace_begin','owned_progress_dependencies','native_API_constructor_begin','direct_notifier_control','unknown_subscription_error','malformed_subscription_error','first_RPC_subscription','owned_StartEvent_posted','native_in_progress_notification','controlled_highest_block_updated','native_done_notification','post_done_RPC_subscription','post_done_immediate_notification','first_RPC_unsubscribe','repeat_unsubscribe_error','second_RPC_unsubscribe','observation_frozen','owned_RPC_cleanup_begin','owned_RPC_cleanup_end','owned_chain_stopped','trace_end']


def inspect_slot(q,slot,binary,fixture_sha,trees):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    if any(r.get(k)!=v for k,v in {**process,**slot}.items()):raise ValueError('native slot/process identity differs')
    if r['binary_sha256']!=binary['sha256']or r['fixture_sha256']!=fixture_sha:raise ValueError('native binary/fixture changed')
    if r['argv']!=[binary['path'],'-test.v=true','-test.run=^TestTicketNativeProgress$','-test.count=1','-test.timeout=155s']or r['exit_code']or r['timeout']or r['term_sent']or r['kill_sent']or not r['owned_new_session']or not r['reaped']or r['remaining_live_group_members']or r['tmp_children_after']or r['elapsed_seconds']>=240:raise ValueError('native slot failed or termination/cleanup unproved')
    for ext in ('stdout','stderr'):
        if sha(raw(q/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('native raw output differs')
    rows=[json.loads(l)for l in raw(q/'events.jsonl.gz').decode().splitlines()];by={e['event']:e for e in rows}
    if [e['event']for e in rows]!=EVENTS or [e['sequence']for e in rows]!=list(range(1,len(EVENTS)+1))or any(e['candidate']!=slot['candidate']for e in rows)or [e['elapsed_ns']for e in rows]!=sorted(e['elapsed_ns']for e in rows):raise ValueError('native trace events/order differ')
    begin=by['trace_begin']
    if any(begin[k]!=v for k,v in dict(base_commit=COMMIT,native_timer_seconds=60,transport='rpc.Server.ServeCodec/rpc.NewCodec/net.Pipe',external_listener=False,Ethereum_factory=False,real_network_sync=False,independent_oracle=False).items()):raise ValueError('native timer/transport/host scope promoted')
    dep=by['owned_progress_dependencies'];progress=dep['expected_progress']
    fields=['StartingBlock','CurrentBlock','HighestBlock','PulledStates','KnownStates','SyncedAccounts','SyncedAccountBytes','SyncedBytecodes','SyncedBytecodeBytes','SyncedStorage','SyncedStorageBytes','HealedTrienodes','HealedTrienodeBytes','HealedBytecodes','HealedBytecodeBytes','HealingTrienodes','HealingBytecode','TxIndexFinishedBlocks','TxIndexRemainingBlocks']
    expected_progress={k:(1 if k=='HighestBlock'else 0)for k in fields}
    if any(dep[k]!=v for k,v in dict(memory_chain=True,snap_constructor=True,downloader_constructor=False,indexer='disabled_error_branch',network_peers=0).items())or progress!=expected_progress or any(type(v)is not int for v in progress.values()):raise ValueError('controlled real progress dependency contract differs')
    if by['direct_notifier_control']['RPC_request']is not False or by['direct_notifier_control']['typed_error']is not True:raise ValueError('direct notifier check falsely counted as RPC')
    for event,request,code,message in [('unknown_subscription_error',1,-32601,'no "native_ticket_unknown" subscription in eth namespace'),('malformed_subscription_error',2,-32602,'expected subscription name as first argument')]:
        e=by[event]
        if e['request_id']!=request or e['code']!=code or e['message']!=message or e['callbacks']!=0:raise ValueError('actual negative RPC error/control differs')
    ids=[by[e]['request_id']for e in ['unknown_subscription_error','malformed_subscription_error','first_RPC_subscription','post_done_RPC_subscription','first_RPC_unsubscribe','repeat_unsubscribe_error','second_RPC_unsubscribe']]
    if ids!=list(range(1,8)):raise ValueError('real caller request order/denominator differs')
    sid1,sid2=by['first_RPC_subscription']['subscription_id'],by['post_done_RPC_subscription']['subscription_id']
    if sid1==sid2 or any(not isinstance(s,str)or not re.fullmatch('0x[0-9a-f]+',s)for s in (sid1,sid2)):raise ValueError('native subscription identity differs')
    for label in ('first_RPC_subscription','post_done_RPC_subscription'):
        if by[label]['namespace']!='eth'or by[label]['callback']!='syncing':raise ValueError('native API registration route differs')
    for label,sid in [('native_in_progress_notification',sid1),('native_done_notification',sid1),('post_done_immediate_notification',sid2),('first_RPC_unsubscribe',sid1),('repeat_unsubscribe_error',sid1),('second_RPC_unsubscribe',sid2)]:
        if by[label]['subscription_id']!=sid:raise ValueError('native result bound to wrong subscription')
    current=by['native_in_progress_notification'];payload=current['payload']
    if payload!=current['expected_payload']or payload!=dict(syncing=True,status=progress)or current['actual_source_progress']is not True:raise ValueError('full actual progress payload differs')
    if by['owned_StartEvent_posted']['source']!='test_owned_mux_not_syncWithPeer'or by['controlled_highest_block_updated']!={**{k:by['controlled_highest_block_updated'][k]for k in ('event','sequence','elapsed_ns','candidate')},'highest_block':0,'under_syncStatsLock':True,'network_sync_completed':False}:raise ValueError('test-controlled StartEvent/height mislabeled as real sync')
    first=current['elapsed_ns']-by['native_API_constructor_begin']['elapsed_ns'];gap=by['native_done_notification']['elapsed_ns']-current['elapsed_ns'];immediate=by['post_done_immediate_notification']['elapsed_ns']-by['post_done_RPC_subscription']['elapsed_ns']
    if first<60_000_000_000 or gap<59_000_000_000 or not 0<=immediate<2_000_000_000:raise ValueError('native60s or immediate-done timing unproved/shortened')
    if by['native_done_notification']['payload']is not False or by['post_done_immediate_notification']['payload']is not False:raise ValueError('done notification not false')
    for label,count in [('first_RPC_unsubscribe',1),('second_RPC_unsubscribe',0)]:
        e=by[label]
        if e['RPC_result']is not True or e['remaining_callbacks']!=count or e['backend_membership_proved']is not False:raise ValueError('callback completion falsely proves backend membership')
    if by['repeat_unsubscribe_error']['message']!='subscription not found'or by['repeat_unsubscribe_error']['code']!=-32000:raise ValueError('actual repeated unsubscribe error differs')
    obs=by['observation_frozen']
    if any(obs[k]!=v for k,v in dict(notifications=3,caller_RPC_requests=7,native_timer_fires_observed=2,full_original5_schedules=False,full12_requirements=False,independent_acceptable_candidate_set=None).items()):raise ValueError('native observations promoted to full requirements/candidate acceptance')
    cleanup=by['owned_RPC_cleanup_end']
    if any(cleanup[k]!=v for k,v in dict(callbacks=0,event_loops=0,ServeCodec_joined=True,peer_connections=0).items())or by['owned_chain_stopped']['memory_database']is not True or by['trace_end']['test_failed']is not False:raise ValueError('native owned cleanup unproved')
    if not prior.stack_frame(raw(q/'events.jsonl.installed.stack.gz').decode(),'(*DownloaderAPI).Syncing.func1','/eth/downloader/api.go:','[select]')or '(*DownloaderAPI).Syncing.func1'in raw(q/'events.jsonl.observed.stack.gz').decode():raise ValueError('native source callback lifecycle unproved')
    return dict(**slot,valid_technical_observation=True,caller_RPC_requests=7,direct_non_RPC_notifier_controls=1,notifications=3,native_timer_fires=2,constructor_to_first_notification_ns=first,notification_gap_ns=gap,post_done_notification_ns=immediate,scope='controlled_height_owned_memory_chain_actual_native_timer_and_JSONRPC_not_full_host_or_network_sync',subscription_id_sha256=[sha(s.encode())for s in (sid1,sid2)],trace_sha256=sha(raw(q/'events.jsonl.gz')),binary_sha256=binary['sha256'],fixture_sha256=fixture_sha,applied_project_tree=trees['applied_project_tree'],build_input_tree=trees['build_input_tree'],backend_membership_proved=False,independent_oracle=False,independent_acceptable_candidate_set=None,canonical_selection=None,quality_score=None)


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;q=p/'actual';scope=read(p/'SCOPE.json')
    slots=[dict(slot_id=f'{role}-{v}-native60-progress',candidate=role,variant=v,repetition=1,case_id='native60-progress')for role in ROLES for v in ('general','race')]
    if sha((p/'SCOPE.json').read_bytes())!=SCOPE_SHA or read(q/'SCOPE.json')!=scope or read(q/'planned.json')!=slots or scope['planned_slots']!=slots:raise ValueError('native frozen plan differs')
    env=read(q/'environment.json')
    if env!=read(root/prior.PACKET/'actual/environment.json'):raise ValueError('native task environment differs from explicit offline allowlist/cache profile')
    if env['credential_environment_inherited']is not False or set(env['allowed_inherited_keys'])-{'HOME','USER','TMPDIR','LANG','LC_ALL'}or any(env['task_settings'].get(k)!=v for k,v in dict(GOPROXY='off',GOSUMDB='off',GOVCS='*:off',GOTOOLCHAIN='local',GOFLAGS='-mod=readonly',GOWORK='off',GOMODCACHE='/Users/kevin/.gvm/pkgsets/go1.23.12/global/pkg/mod',GOCACHE='/private/tmp/ks-ticket-rpc-20261019/go-cache').items()):raise ValueError('native offline/credential environment changed')
    built,trees,fixture_sha=bound_sources(root,p)
    if fixture_sha!=scope['fixture_sha256']:raise ValueError('predeclared native fixture changed')
    receipt=read(q/'receipt.json')
    if receipt!=dict(planned=10,attempted=10,retries=0,tracked_changes_only_exact_candidate_api=True,test_addition_only=True,production_patch=False,source_commit=COMMIT,independent_approved=0,independent_tickets=0,FINAL=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0):raise ValueError('native full attempt population differs')
    attempts=read(q/'attempts.json')
    if len(attempts)!=10:raise ValueError('native planned attempt absent')
    rows=[]
    for slot,attempt in zip(slots,attempts):
        d=q/'slots'/slot['slot_id']
        if read(d/'receipt.json')!=attempt:raise ValueError('native attempt aggregate differs')
        rows.append(inspect_slot(d,slot,built[slot['candidate']][slot['variant']],fixture_sha,trees[slot['candidate']]))
    author_bytes=subprocess.check_output(['git','show',REVISION+':'+str(prior.draft.AUTHOR)],cwd=root);notes=json.loads(author_bytes);covered={'API-ENTRY','API-UNSUB','API-NEGATIVE','NORMAL-DELIVERY','OWNED-CLEANUP','SOURCE-BOUNDARY'}
    gaps={
        'API-ENTRY':'Owned registered codec reached Syncing; full Ethereum factory, installed roles/finalHEAD and deployment transport remain unverified.',
        'API-UNSUB':'RPC true/repeated-id registry error and callback return observed; exact backend removal/ack events and other schedules remain uninstrumented.',
        'API-NEGATIVE':'Actual unknown/malformed/repeat errors observed; direct no-notifier is not an unsupported RPC transport test, and full transport/version matrix remains pending.',
        'METHOD-RETURN':'Only normal-flow callbacks returned; every started same-object call after acknowledgement across original5 schedules is unproved.',
        'METHOD-ONCE':'Repeated RPC id fails in registry and does not invoke a second Unsubscribe on the same internal object; concurrent Once calls remain untested.',
        'METHOD-REMOVE':'Backend map is not directly instrumented; source deletion-before-ack fact is not independent runtime observation.',
        'METHOD-ACK':'No complete instrumentation of acknowledgement and all post-ack channel uses; callback absence alone is insufficient.',
        'NORMAL-DELIVERY':'Native progress/false/post-done delivery observed; original full normal scenario still needs method removal/ack/return ordering proof.',
        'LATE-DRAIN':'No finite late status offered after uninstall acceptance and before acknowledgement in this native flow.',
        'BLOCKED-DELIVERY':'No actual eventLoop status send overlapping sole RPC consumer exit; prior controlled backend case is a narrower route.',
        'OWNED-CLEANUP':'Only scoped API/codec/memory chain and process exit proved; preserved original package-init workers are not individually joined, and external host lifecycle is not tested.',
        'SOURCE-BOUNDARY':'Real project/build trees and binaries bound; installed4roles/final agentHEAD and full Ethereum factory remain unverified.',
    }
    units=[dict(slot_id=row['slot_id'],candidate=row['candidate'],requirement=r['id'],label='bounded_observation_with_remaining_scope'if r['id']in covered else'not_exercised',full_requirement_satisfied=False,independent_approved=False,assertion=r['assertion'],source_anchor_ids=r['anchors'],remaining=gaps[r['id']])for row in rows for r in notes['requirements']]
    schedules=[dict(scenario_id=s['id'],new_native_normal_observations=[r['slot_id']for r in rows]if s['id']=='normal_delivery_then_unsubscribe'else[],full_original_scenario_satisfied=False,remaining_preconditions=s['preconditions'])for s in notes['scenarios']]
    return dict(summary=dict(kind=KIND,planned_attempts=10,actual_attempts=10,valid_technical_observations=10,caller_RPC_requests=70,direct_non_RPC_notifier_controls=10,notifications=30,native60s_timer_fires=20,candidate_roles=5,unique_method_variants=4,unique_project_trees=4,repetitions_per_role_profile=1,independent_repeated_execution_criterion_satisfied=False,exposed_shared_DEV_tickets=1,requirements_author_revision=REVISION,requirements_author_sha256=sha(author_bytes),requirement_units=len(units),bounded_scope_units=60,not_exercised_units=60,full_original5_scenarios_satisfied=0,full12_requirement_satisfaction=False,full_Ethereum_factory=False,installed_roles=False,real_network_sync=False,independent_approved=0,independent_tickets=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,independent_acceptable_candidate_set=None,canonical_selection=None,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0),observations=rows,requirements=units,original_schedules=schedules)


def worklist(root=ROOT):
    d=read(root/prior.PACKET/'WORKLIST.json');d.update(current_stage='65_WIV1_V2_native60_progress_RPC_DEV',completed_in_this_scope=['5roles4methods_10fresh_general_race_binaries_10fixed_target_trials','actual_native60s_timer20fires_30notifications_70caller_RPC_and10direct_controls','owned_memory_chain_actual_snap_progress_controlled_height_not_real_network_sync','12requirements_x10bounded_observations_and_original5_schedule_gaps_kept_explicit'],next_task='WI_V1_V2_original5_schedules_same_object_Once_late_drain_ack_removal_actual_overlap_and_installed_roles_then_WI6',last_verified_date='2026-10-19')
    d['remaining_all'][0]['task']='WI-V1: native60s progress/false/post-done notification now observed in owned memory-chain controlled-height normal DEV flow; full Ethereum/Downloader constructors, real syncWithPeer StartEvent, enabled tx-indexer branch, original5 schedules/12requirements, independent ticket/oracles, installed4roles/finalHEAD remain'
    d['remaining_all'][1]['task']='WI-V2: all5 roles can preserve this bounded normal notifier flow; this does not establish acceptable membership. Original/reintroduction alias still1variant; full5 schedules, primary/derived independent oracle and acceptable candidate set remain null/pending'
    d['AI_DEV_draft_progress']['WI-V1']='one_exposed_ticket;phase64actual_RPC134attempts;phase65native60s_normal10trials20fires30notifications;full5schedules12requirements_independent_oracle_pending'
    d['AI_DEV_draft_progress']['WI-V2']='5roles4method/project_variants;normal_native_notifier_flow_observed_all5;independent_membership_null_no_selection'
    return d


def prepare(work,p):
    if any(x.name!='SCOPE.json'for x in p.iterdir()):raise ValueError('refusing existing native evidence overwrite')
    source=work/'native-progress-trials-v1'
    for x in sorted(source.rglob('*')):
        if not x.is_file()or x.suffix=='.test':continue
        y=p/'actual'/x.relative_to(source);y.parent.mkdir(parents=True,exist_ok=True)
        if x.name in ('stdout','stderr')or x.suffix in ('.jsonl','.stack'):y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
        else:shutil.copyfile(x,y)
    for role in ROLES:
        for variant,b in read(p/'actual/candidates'/role/'binaries.json').items():
            if sha(Path(b['path']).read_bytes())!=b['sha256']:raise ValueError('native postrun binary changed')
    data=derive(packet=p)
    if historical_errors():raise ValueError('historical evidence/native inputs changed')
    for name,key in [('SUMMARY.json','summary'),('observations.json','observations'),('requirements.json','requirements'),('original-schedules.json','original_schedules')]:write(p/name,data[key])
    write(p/'native-reuse.json',native_record());write(p/'WORKLIST.json',worklist())
    (p/'REPORT.txt').write_text('Phase65: actual native60second progress notifier over owned JSON-RPC DEV; no independent acceptance\n\n'
        'Five roles/four exact retained method variants are applied to five owned fresh base0bf2f4d1 checkouts. Only declared API overlays change tracked source; one owned test is added. All2228 base entries, real Git project/build trees, profile Go1.23.12/darwin/arm64/CGO1, ten general/race binaries, fixture/raw-output hashes and postrun source/binary preservation are bound. No candidate commit is invented.\n'
        'Fixed10 trials, one per role/profile. All original package tests are kept; expensive package init is outside the155second Go test timeout and within the240second owned process bound. Native timer60seconds is not shortened, mocked or forced. Two native timer progress branches and one immediate post-done install branch reach actual Server.ServeCodec/NewCodec/net.Pipe raw JSON-RPC notification frames for each trial:30 notifications/20timer fires/70caller RPC requests and10 separate direct no-notifier controls. Unknown/malformed subscription and repeated unsubscribe retain source error codes/messages; first and second real subscription ids stay distinct and bind notifications and unsubscribe responses.\n'
        'A new owned in-memory core.BlockChain uses ethash fake consensus and no enabled tx indexer. A real snap.NewSyncer supports actual Downloader.Progress; the Downloader object uses test-controlled stats, not the full Downloader constructor. A test-owned StartEvent and highest-block update under syncStatsLock supply the scenario, not real peer sync completion. Initial progress has19 actual PascalCase integer fields with HighestBlock1 and others0; full typed payload is compared. Native completed and newly installed post-done subscriptions receive false. The disabled-indexer error branch is observed; enabled indexing remains untested.\n'
        'The normal notifier flow is preserved for all5 roles, including ORIGINAL/reintroduction and early-return mutant. It cannot select acceptable repair candidates. Callback termination and RPC registry-repeat error do not independently instrument backend map deletion, acknowledgement, post-ack channel use or same-object concurrent Once. Original5 schedule mapping and12requirements x10 observations retain all gaps:60 units have bounded observations with remaining scope and60 are not exercised; no unit is marked fully satisfied. Whole host/real sync/installed4roles/finalHEAD, independent ticket/primary and derived oracles/sufficient repetitions/candidate set are pending.\n'
        'Observation freezes before owned cleanup. Scoped API callbacks/eventLoop and ServeCodec are joined, memory chain Stop returns and database closes; actual process groups are reaped with no remaining members and empty owned temp children. This does not assert every preserved package-init goroutine was joined before process exit. No external listener/network/operational node, credential inheritance, global settings/install, publication, system-clock change or paid/external model call is required.\n'
        'CKS native inputs1738/approved5/config2 remain identical to prior proof. General/race4158pass18skip each, build/fmt/vet/pinned lint are reused, not fresh CKS runs. Target upstream general/race builds/trials are fresh. Independent0/FINAL0/formal0of9/freeze null/execution false, D04unassigned; no names/contact required. Full remaining priority worklist is preserved and recomputed, and signed storage is checked against actual Git.\n')
    seal(p)


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=read(p/'manifest.json');actual={x.relative_to(p).as_posix():sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('native evidence inventory differs')
        for r in m['tools']:
            if sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('bound native tool changed '+r['path'])
        errors+=historical_errors(root);data=derive(root,p)
        for name,key in [('SUMMARY.json','summary'),('observations.json','observations'),('requirements.json','requirements'),('original-schedules.json','original_schedules')]:
            if read(p/name)!=data[key]:errors.append('derived native result differs '+name)
        if read(p/'native-reuse.json')!=native_record(root):errors.append('unchanged native reuse differs')
        if read(p/'WORKLIST.json')!=worklist(root):errors.append('remaining/priority/current worklist differs from bounded native scope')
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
        if a.require_formal_gold:errors.append('Controlled-height native progress DEV observations are not independent approved/frozen gold')
        print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
