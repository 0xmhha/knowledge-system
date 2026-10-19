"""Actual method schedule DEV evidence with explicit runtime/oracle boundaries."""
import argparse,gzip,json,re,shutil,subprocess
from pathlib import Path
import ticket_actual_rpc as prior
import ticket_native_progress as previous
from source_packet import oid,verify
ROOT,BASE=prior.ROOT,prior.BASE
PACKET=BASE/'followups/ticket-method-schedules-v1'
REVISION='5db56cfa43189aadc55a928052dc4a6418376fb5'
KIND='WIV1_V2_actual_method5_schedules_DEV_not_independent_acceptance'
COMMIT=prior.COMMIT
ROLES=prior.ROLES
API='eth/downloader/api.go';TEST='eth/downloader/ticket_owned_method_schedules_test.go'
TOOLS=['scripts/'+n for n in ('ticket_method_schedules.py','test_ticket_method_schedules.py','run-ticket-method-schedules.py','fixtures/ticket-method-schedules.go.fixture','run-wi4-target-subscription.py','ticket_actual_rpc.py','ticket_dev_gold.py','source_packet.py','check-operational-acceptance.py','test_operational_acceptance.py','ticket_native_progress.py','run-ticket-method-correction.py')]
write,read,raw,sha=prior.write,prior.read,prior.raw,prior.sha
native_record=prior.native_record
flat_map,tree_oid=prior.flat_map,prior.tree_oid
SCOPE_SHA='b24b6c8fe1adfa2e08178f2ef6407422825b4fbed4fe4a2a3218db8dae8ae039'
CORRECTION_SHA='aaf505478d0c08ec71043a7546106d293545d4f16e94a104f1e6d7efb6ab67d0'

def historical_errors(root=ROOT):
    errors=previous.historical_errors(root);p=root/previous.PACKET;m=read(p/'manifest.json')
    if(p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(previous.PACKET/'manifest.json')],cwd=root):errors.append('stage65 manifest changed')
    for n,h in m['artifacts'].items():
        if sha((p/n).read_bytes())!=h:errors.append('stage65 evidence changed '+n)
    for r in m['tools']:
        if sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('stage65 tool at signed revision differs')
    return errors



def seal(p):write(p/'manifest.json',dict(kind=KIND,previous_source_revision=REVISION,tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={x.relative_to(p).as_posix():sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))



def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=sha((root/PACKET/'manifest.json').read_bytes()),status=KIND,report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))



def update_indices():
    p=ROOT/PACKET;seal(p);ref=reference();state=read(p/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);base=ROOT/'docs/spec-driven'
    proposal=base/'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json';d=read(proposal);d.update(ticket_method_schedules=ref,ticket_native_progress=dict(d['ticket_native_progress'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state);write(proposal,d)
    link=dict(path=str(proposal.relative_to(ROOT)),sha256=sha(proposal.read_bytes()),status='D04_review_pending_WIV1_V2_method5_DEV_not_independent_acceptance')
    for name in ('OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','operational-preparation/source-review-supplement/GAPS.json'):
        q=base/name;d=read(q);d.update(ticket_method_schedules=ref,ticket_native_progress=dict(d['ticket_native_progress'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state,adoption_acceptance_decision_proposal=link)
        if q.name=='OPERATIONAL-ACCEPTANCE-PROGRESS.json':d['phases'][0]['evidence'].update(ticket_method_schedules=ref,ticket_native_progress=d['ticket_native_progress'],current_readiness_preparation=current,adoption_acceptance_decision_proposal=link)
        write(q,d)



def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[read(root/n)for n in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('ticket_method_schedules')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current ticket_RPC reference/state differs')
        if d.get('reviewer_identity_policy')!=old.get('reviewer_identity_policy'):errors.append('privacy policy changed')
        for k,v in old.items():
            if k=='current_readiness_preparation'or not isinstance(v,dict)or'manifest'not in v:continue
            expected=dict(v)
            if k=='ticket_native_progress':expected['historical_source_revision']=REVISION
            if d.get(k)!=expected:errors.append('historical preparation changed '+k)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical ticket_RPC probes cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('current proposal link differs')
    return errors





def bound_sources(root,p,actual_name='actual',roles=ROLES,fixture_path=None):
    q=p/actual_name;fixture=(fixture_path or root/TOOLS[3]).read_bytes();fixture_sha=sha(fixture)
    expected=read(root/BASE/'followups/wi4-target-subscription-v1/initial/source-binding.json')['tracked'];candidates={c['id']:c for c in read(root/prior.draft.PACKET/'private/candidates.json')}
    proof=verify(root/BASE/'schedule-candidates-v1',COMMIT);commit=(root/BASE/'schedule-candidates-v1'/proof['objects']['commit:'+COMMIT]).read_bytes();base_tree=commit.splitlines()[0].decode().removeprefix('tree ')
    built={};tree_records={};post=read(q/'binary-postrun-check.json');all_base=None
    if set(post)!=set(roles):raise ValueError('candidate binary role population differs')
    for role in roles:
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
    if len({tree_records[r]['applied_project_tree']for r in roles})!=len({candidates[r]['method_sha256']for r in roles}):raise ValueError('role aliases inflated into independent variants')
    if 'ORIGINAL'in roles and 'MUT-REINTRO'in roles and tree_records['ORIGINAL']['applied_project_tree']!=tree_records['MUT-REINTRO']['applied_project_tree']:raise ValueError('reintroduction is not exact original alias')
    return built,tree_records,fixture_sha


SCENARIOS=['normal_delivery_then_unsubscribe','blocked_delivery_then_consumer_exit','late_delivery_before_confirmation','confirmation_delayed','concurrent_unsubscribe_once']
EVENTS=['trace_begin','native_completion_wait_begin','native_completion_received','normal_observation_frozen','blocked_precondition','blocked_observation_frozen','post_observation_liveness','real_cleanup_begin','real_cleanup_end']+['controlled_request','controlled_pre_ack_frozen','controlled_calls_joined']*3+['all_observations_frozen','trace_end']


def inspect_slot(q,slot,binary,fixture_sha,trees):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    if any(r.get(k)!=v for k,v in {**process,**slot}.items()):raise ValueError('method slot/process identity differs')
    if r['binary_sha256']!=binary['sha256']or r['fixture_sha256']!=fixture_sha:raise ValueError('method binary/fixture changed')
    if r['argv']!=[binary['path'],'-test.v=true','-test.run=^TestTicketMethodSchedules$','-test.count=1','-test.timeout=100s']or r['exit_code']or r['timeout']or r['term_sent']or r['kill_sent']or not r['owned_new_session']or not r['reaped']or r['remaining_live_group_members']or r['tmp_children_after']or r['elapsed_seconds']>=180:raise ValueError('method slot failed or termination/cleanup unproved')
    for ext in ('stdout','stderr'):
        if sha(raw(q/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('method raw output differs')
    rows=[json.loads(l)for l in raw(q/'events.jsonl.gz').decode().splitlines()]
    if [e['event']for e in rows]!=EVENTS or [e['sequence']for e in rows]!=list(range(1,len(EVENTS)+1))or any(e['candidate']!=slot['candidate']for e in rows)or [e['elapsed_ns']for e in rows]!=sorted(e['elapsed_ns']for e in rows):raise ValueError('method trace population/order differs')
    def exact(index,fields):
        e=rows[index];values={k:v for k,v in e.items()if k not in ('event','sequence','candidate','elapsed_ns')}
        if values!=fields:raise ValueError('method trace facts differ '+e['event'])
    early=slot['candidate']=='MUT-EARLY';drains=slot['candidate']in ('CAND-A','CAND-B')
    exact(0,dict(base_commit=COMMIT,native_timer_seconds=60,RPC=False,independent_oracle=False,source_instrumentation=False))
    exact(1,dict(controlled_height=0,StartEvent_from_test_mux=True));exact(2,dict(payload=False))
    if rows[2]['elapsed_ns']<60_000_000_000:raise ValueError('native timer shortened or forced')
    exact(3,dict(scenario=SCENARIOS[0],route='actual_eventLoop_post_done_install',notification=False,method_returned=True,backend_removal_observed=False,ack_observed=False))
    exact(4,dict(sole_consumer_exited=True,unbuffered_channel=True,send_site=94,route='actual_eventLoop_post_done_install_not_RPC_callback'))
    exact(5,dict(scenario=SCENARIOS[1],method_pending=not(early or drains),eventLoop_send_pending=not drains,backend_removal_observed=False,ack_observed=False,rescue_started=False))
    if rows[5]['elapsed_ns']-rows[4]['elapsed_ns']<300_000_000 and not(early or drains):raise ValueError('blocked pending observation narrowed')
    exact(6,dict(actual_loop_serviced_probe=drains,probe_before_rescue=True));exact(7,{})
    exact(8,dict(calls_joined=3,drains_joined=4 if drains else 3,event_loops=0,memory_chain_stopped=True))
    if not prior.stack_frame(raw(q/'events.jsonl.blocked-before-call.stack.gz').decode(),'(*DownloaderAPI).eventLoop','/eth/downloader/api.go:94','[chan send'):raise ValueError('real loop blocked precondition absent')
    observed=raw(q/'events.jsonl.blocked-observed.stack.gz').decode()
    if prior.stack_frame(observed,'(*DownloaderAPI).eventLoop','/eth/downloader/api.go:94','[chan send')!=(not drains):raise ValueError('real blocked send outcome unsupported')
    if not(early or drains)and not prior.stack_frame(observed,'(*SyncStatusSubscription).Unsubscribe','/eth/downloader/api.go:186','[chan send'):raise ValueError('paired actual method send wait absent')
    observations=[dict(scenario=SCENARIOS[0],route='actual_eventLoop_post_done_install',outcome='notification_then_method_return_not_map_ack_proof'),dict(scenario=SCENARIOS[1],route='actual_eventLoop_post_done_install_not_RPC_callback',outcome='drained_returned_loop_live'if drains else'early_return_loop_still_blocked'if early else'paired_sends_pending_before_rescue')]
    for i,scenario in enumerate(SCENARIOS[2:]):
        k=9+3*i;count=3 if i==2 else 1
        exact(k,dict(scenario=scenario,route='controlled_receiver_actual_Unsubscribe',same_object=True,calls=count,request_received=not early,returned_before_request=early,backend_map_present=False))
        exact(k+1,dict(scenario=scenario,method_pending=not early,individual_returns=count if early else 0,late_sends_completed=3 if i==0 and not early else 0,duplicate_request=False,ack_released=False,rescue_started=False))
        if not early and rows[k+1]['elapsed_ns']-rows[k]['elapsed_ns']<300_000_000:raise ValueError('controlled ack delay narrowed')
        exact(k+2,dict(scenario=scenario,calls=count,request_received=not early,ack_released=not early,duplicate_request=False,all_calls_joined=True,map_removal_proved=False,post_ack_all_channel_use_proved=False))
        stack=raw(q/('events.jsonl.'+scenario+'-before-ack.stack.gz')).decode()
        if not early and '(*SyncStatusSubscription).Unsubscribe'not in stack:raise ValueError('controlled method pending stack absent')
        if i==2 and not early and sum('(*SyncStatusSubscription).Unsubscribe'in g and '/eth/downloader/api.go:'in g for g in stack.split('\n\n'))<3:raise ValueError('three actual same-object method caller stacks absent')
        observations.append(dict(scenario=scenario,route='controlled_receiver_actual_Unsubscribe',outcome='returned_without_request_or_ack'if early else'one_request_finite_late3_then_ack_return'if i==0 else'one_request_all3_calls_wait_then_ack_join'if i==2 else'one_request_wait_then_ack_return',backend_map_removal_proved=False))
    exact(18,dict(schedules=5,full12_requirements=False,independent_acceptable_candidate_set=None));exact(19,dict(test_failed=False))
    return dict(**slot,valid_technical_observation=True,schedules=observations,native_timer_fires=1,controlled_requests=0 if early else 3,finite_late_sends=0 if early else 3,same_object_calls=3,all_controlled_calls_joined=5,real_calls_joined=3,trace_sha256=sha(raw(q/'events.jsonl.gz')),binary_sha256=binary['sha256'],fixture_sha256=fixture_sha,applied_project_tree=trees['applied_project_tree'],build_input_tree=trees['build_input_tree'],full_original5_scenarios_satisfied=0,independent_oracle=False,independent_acceptable_candidate_set=None,quality_score=None)


def initial_cancellation(p):
    d=p/'pretrial-cancelled';scope=read(d/'SCOPE.json');cancel=read(d/'cancellation.json');audit=read(d/'post-cancellation-audit.json')
    if sha((d/'SCOPE.json').read_bytes())!='4fa31e158ed20bc3566ac01458bff353213f345a7a29fcc8516861ac5dfd9a1f'or read(d/'actual/SCOPE.json')!=scope or sha((d/'fixture.go.txt').read_bytes())!=scope['fixture_sha256']:raise ValueError('cancelled pretrial inputs changed')
    if cancel['actual_target_attempts']!=0 or cancel['slot_directories']!=0 or cancel['not_automatic_retry']is not True or len(cancel['actions'])!=1 or not audit['all_groups_empty']or not audit['driver_terminal']or audit['actual_target_attempts']!=0:raise ValueError('cancelled pretrial incorrectly promoted/erased')
    receipts=list((d/'actual').glob('candidates/*/build-*/receipt.json'))
    if len(receipts)!=8 or len(audit['groups'])!=8 or any(r['remaining_live_group_members']for r in audit['groups']):raise ValueError('cancelled initial build population/cleanup differs')
    negative=[]
    for f in receipts:
        r=read(f)
        if not r['reaped']or r['remaining_live_group_members']or r['timeout']:raise ValueError('cancelled initial process cleanup unproved')
        if r['exit_code']!=0:negative.append(r)
        for ext in ('stdout','stderr'):
            if sha(raw(f.parent/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('initial build output changed')
    if len(negative)!=1 or negative[0]['exit_code']!=-15 or negative[0]['pgid']!=cancel['actions'][0]['pgid']:raise ValueError('explicit cancelled build receipt lost')
    if list((d/'actual').glob('slots/*')):raise ValueError('pretrial cancellation cannot hide actual slots')
    return dict(initial_preparation='cancelled_before_target_trials',planned_initial_builds=10,initial_build_receipts=8,initial_builds_not_started=2,successful_initial_builds=7,explicitly_terminated_initial_builds=1,initial_target_attempts=0,initial_groups_remaining=0,corrective_plan_explicit=True)


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;q=p/'actual';scope=read(p/'SCOPE.json');cancel=initial_cancellation(p)
    slots=[dict(slot_id=f'{role}-{v}-method5',candidate=role,variant=v,repetition=1,case_id='method5')for role in ROLES for v in ('general','race')]
    if sha((p/'SCOPE.json').read_bytes())!=SCOPE_SHA or read(q/'SCOPE.json')!=scope or read(q/'planned.json')!=slots or scope['planned_slots']!=slots:raise ValueError('method frozen plan differs')
    if read(q/'environment.json')!=read(root/previous.PACKET/'actual/environment.json'):raise ValueError('method offline environment differs')
    built,trees,fixture_sha=bound_sources(root,p,fixture_path=p/'initial-fixture.go.txt')
    if fixture_sha!=scope['fixture_sha256']:raise ValueError('predeclared initial method fixture changed')
    receipt=read(q/'receipt.json')
    expected_receipt=dict(planned=10,attempted=10,retries=0,tracked_changes_only_exact_candidate_api=True,test_addition_only=True,production_patch=False,source_commit=COMMIT,independent_approved=0,independent_tickets=0,FINAL=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0)
    if receipt!=expected_receipt:raise ValueError('method full initial attempt population differs')
    attempts=read(q/'attempts.json')
    if len(attempts)!=10 or {r['slot_id']for r in attempts}!={s['slot_id']for s in slots}:raise ValueError('method planned attempt absent/duplicate')
    by={r['slot_id']:r for r in attempts};rows=[];failed=[]
    for slot in slots:
        d=q/'slots'/slot['slot_id'];r=read(d/'receipt.json')
        if r!=by[slot['slot_id']]:raise ValueError('method initial attempt aggregate differs')
        if not r['exit_code'] and not r['timeout']:
            row=inspect_slot(d,slot,built[slot['candidate']][slot['variant']],fixture_sha,trees[slot['candidate']]);row['attempt_kind']='initial';rows.append(row)
        else:
            proc=read(d/'process/receipt.json')
            if any(r.get(k)!=v for k,v in {**proc,**slot}.items())or r['binary_sha256']!=built[slot['candidate']][slot['variant']]['sha256']or r['fixture_sha256']!=fixture_sha:raise ValueError('failed initial attempt identity differs')
            if r['argv']!=[built[slot['candidate']][slot['variant']]['path'],'-test.v=true','-test.run=^TestTicketMethodSchedules$','-test.count=1','-test.timeout=100s']or not r['reaped']or r['remaining_live_group_members']or not r['owned_new_session']:raise ValueError('failed initial process identity/cleanup unproved')
            for ext in ('stdout','stderr'):
                if sha(raw(d/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('failed initial raw output changed')
            events=[json.loads(l)for l in raw(d/'events.jsonl.gz').decode().splitlines()]
            if not events or [e['sequence']for e in events]!=list(range(1,len(events)+1))or any(e['candidate']!=slot['candidate']for e in events):raise ValueError('failed partial trace identity differs')
            failed.append(dict(**slot,exit_code=r['exit_code'],outer_process_timeout=r['timeout'],internal_Go_test_timeout='test timed out after 1m40s'in raw(d/'process/stderr.gz').decode(),failure_reason='elapsed_wait_tag_missed_then_unbounded_probe; original defective method observation preserved',partial_trace_events=[e['event']for e in events],internal_cleanup_proved=False,owned_process_group_remaining=0,not_a_valid_complete_observation=True))
    correction=read(p/'CORRECTION-SCOPE.json');cq=p/'actual-corrective';corrective_slots=[s for s in slots if s['slot_id']in {f['slot_id']for f in failed}]
    if correction['initial_attempts_manifest_sha256']!=sha((q/'attempts.json').read_bytes())or correction['initial_scope_sha256']!=SCOPE_SHA or sha((p/'CORRECTION-SCOPE.json').read_bytes())!=CORRECTION_SHA or read(cq/'SCOPE.json')!=correction or correction['planned_slots']!=corrective_slots or read(cq/'planned.json')!=corrective_slots:raise ValueError('explicit corrective population differs from every failed initial slot')
    if read(cq/'environment.json')!=read(q/'environment.json'):raise ValueError('corrective environment differs')
    roles=[r for r in ROLES if any(s['candidate']==r for s in corrective_slots)]
    cb,ct,cf=bound_sources(root,p,'actual-corrective',roles)
    if cf!=correction['fixture_sha256']:raise ValueError('corrective fixture differs')
    if read(cq/'receipt.json')!=dict(expected_receipt,planned=len(corrective_slots),attempted=len(corrective_slots)):raise ValueError('corrective aggregate population differs')
    ca=read(cq/'attempts.json')
    if len(ca)!=len(corrective_slots)or {r['slot_id']for r in ca}!={s['slot_id']for s in corrective_slots}:raise ValueError('corrective attempt absent/duplicate')
    cab={r['slot_id']:r for r in ca}
    for slot in corrective_slots:
        d=cq/'slots'/slot['slot_id']
        if read(d/'receipt.json')!=cab[slot['slot_id']]:raise ValueError('corrective attempt aggregate differs')
        row=inspect_slot(d,slot,cb[slot['candidate']][slot['variant']],cf,ct[slot['candidate']]);row['attempt_kind']='explicit_corrective';rows.append(row)
    rows.sort(key=lambda r:next(i for i,s in enumerate(slots)if s['slot_id']==r['slot_id']))
    author_bytes=subprocess.check_output(['git','show',REVISION+':'+str(prior.draft.AUTHOR)],cwd=root);notes=json.loads(author_bytes)
    covered={'METHOD-RETURN','METHOD-ONCE','NORMAL-DELIVERY','LATE-DRAIN','BLOCKED-DELIVERY','OWNED-CLEANUP','SOURCE-BOUNDARY'}
    gaps={
      'API-ENTRY':'No RPC in this method bundle. Phase64/65 actual RPC remains separate; full factory and installed finalHEAD pending.',
      'API-UNSUB':'Direct subscription method calls are not RPC registry cancellation. Actual callback exit overlap remains pending.',
      'API-NEGATIVE':'No transport/negative RPC tests in this bundle; earlier negative route evidence remains separate.',
      'METHOD-RETURN':'Finite tested calls joined after controlled ack or real-loop handling; every schedule/host and independently accepted liveness still unproved.',
      'METHOD-ONCE':'Three callers use exact same object, controlled owner observes one request and zero individual returns until ack except early mutant; one correlated repetition/profile, not independent sufficient population.',
      'METHOD-REMOVE':'Real loop deletion-before-close is a source fact only. Runtime map deletion is not instrumented; controlled receiver has no real map.',
      'METHOD-ACK':'Controlled ack withheld/released and zero individual returns observed, but exact real-loop ack and absence of every post-ack channel use are uninstrumented.',
      'NORMAL-DELIVERY':'Actual post-done installation delivers false before direct method call. Native broadcast primer is separate; exact removal/ack events and normal live RPC overlap not instrumented.',
      'LATE-DRAIN':'Three finite sends after controlled uninstall acceptance and before controlled ack drain. Not real map-owner deletion order or arbitrary finite schedule proof.',
      'BLOCKED-DELIVERY':'Actual post-done install send overlaps sole direct consumer exit. Not native timer broadcast overlapping actual Syncing RPC callback exit; no scheduler/fairness proof.',
      'OWNED-CLEANUP':'Observed before owned rescue; all eight method calls and scoped loop/drains/memory chain joined per process. Not every retained package-init goroutine individually joined or external host lifecycle.',
      'SOURCE-BOUNDARY':'Full2228 source map, exact API candidate/test Git trees/binaries bound. Installed4roles/finalHEAD and independent oracle remain pending.'}
    units=[dict(slot_id=row['slot_id'],candidate=row['candidate'],requirement=r['id'],label='bounded_observation_with_remaining_scope'if r['id']in covered else'not_exercised_or_uninstrumented',full_requirement_satisfied=False,independent_approved=False,assertion=r['assertion'],source_anchor_ids=r['anchors'],remaining=gaps[r['id']])for row in rows for r in notes['requirements']]
    schedules=[dict(scenario_id=s['id'],new_observations=[dict(slot_id=r['slot_id'],**next(o for o in r['schedules']if o['scenario']==s['id']))for r in rows],full_original_scenario_satisfied=False,remaining_preconditions=s['preconditions'],scope_gaps=[gaps[k]for k in ('METHOD-REMOVE','METHOD-ACK','BLOCKED-DELIVERY')]if s['id']==SCENARIOS[1]else[gaps['METHOD-REMOVE'],gaps['METHOD-ACK']])for s in notes['scenarios']]
    return dict(summary=dict(kind=KIND,**cancel,planned_initial_attempts=10,actual_attempts=10+len(corrective_slots),initial_failed_attempts=len(failed),explicit_corrective_attempts=len(corrective_slots),initial_partial_failures=failed,valid_technical_observations=10,correlated_schedule_observations=50,real_eventLoop_schedules=20,controlled_owner_schedules=30,native60s_timer_fires_valid_bundles=10,native60s_timer_fires_all_attempts=10+sum('native_completion_received'in f['partial_trace_events']for f in failed),valid_bundle_controlled_uninstall_requests=24,valid_bundle_controlled_late_sends=24,valid_bundle_method_calls_joined=80,fresh_target_build_receipts=18+2*len(roles),automatic_retries=0,blocked_real_loop_outcomes=dict(drained_returned_loop_live=4,paired_sends_pending_before_rescue=4,early_return_loop_still_blocked=2),candidate_roles=5,unique_method_variants=4,unique_project_trees=4,repetitions_per_role_profile=1,independent_repeated_execution_criterion_satisfied=False,exposed_shared_DEV_tickets=1,requirements_author_revision=REVISION,requirements_author_sha256=sha(author_bytes),requirement_units=len(units),bounded_scope_units=70,not_exercised_or_uninstrumented_units=50,full_original5_scenarios_satisfied=0,full12_requirement_satisfaction=False,actual_RPC_consumer_exit_overlap=False,backend_map_instrumented=False,independent_approved=0,independent_tickets=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,independent_acceptable_candidate_set=None,canonical_selection=None,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0),observations=rows,requirements=units,original_schedules=schedules)


def worklist(root=ROOT):
    d=read(root/previous.PACKET/'WORKLIST.json');d.update(current_stage='66_WIV1_V2_actual_method5_schedules_DEV',completed_in_this_scope=['10fresh_general_race_binaries_and10fixed_processes_50correlated_schedule_observations','actual_eventLoop_post_done_normal_and_blocked_direct_consumer_exit','controlled_late3_ack_delay_same_object3_once_calls_and_pre_rescue_evidence','original_and_early_defects_retained_no_independent_candidate_selection'],next_task='WI_V1_V2_actual_RPC_callback_exit_native_broadcast_overlap_runtime_removal_ack_boundary_then_WI6',last_verified_date='2026-10-19')
    d['remaining_all'][0]['task']='WI-V1: phase66 actual method5 schedules bounded DEV observed (real post-done normal/blocked, controlled late/ack/once); real native-broadcast/Syncing RPC callback exit overlap, runtime removal/ack/all post-ack channel-use boundary, full factory/real sync/enabled indexer, independent ticket/oracle and installed4roles/finalHEAD remain'
    d['remaining_all'][1]['task']='WI-V2: tested bounded routes distinguish draining candidates from original/early defects; no independent acceptable set or candidate selection. Complete original5 schedules/12 requirements, independent primary/derived oracle and sufficient repeat population remain'
    d['AI_DEV_draft_progress']['WI-V1']='phase66_10method_bundles_50correlated_schedules_actual_post_done_loop_and_controlled_late_ack_once;full_original5_12requirements_pending'
    d['AI_DEV_draft_progress']['WI-V2']='5roles4variants;bounded_defect_discrimination_not_independent_membership;acceptable_set_null'
    return d


def prepare(work,p,corrective_work):
    allowed={'SCOPE.json','CORRECTION-SCOPE.json','pretrial-cancelled','initial-fixture.go.txt','initial-runner.py.txt','PREFLIGHT.txt'}
    if any(x.name not in allowed for x in p.iterdir()):raise ValueError('refusing existing method evidence overwrite')
    for source,dest in [(work/'method-schedules-trials-v1','actual'),(corrective_work/'method-schedules-corrective-v1','actual-corrective')]:
        for x in sorted(source.rglob('*')):
            if not x.is_file()or x.suffix=='.test':continue
            y=p/dest/x.relative_to(source);y.parent.mkdir(parents=True,exist_ok=True)
            if x.name in ('stdout','stderr')or x.suffix in ('.jsonl','.stack'):y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
            else:shutil.copyfile(x,y)
        for role in read(p/dest/'binary-postrun-check.json'):
            for variant,b in read(p/dest/'candidates'/role/'binaries.json').items():
                if sha(Path(b['path']).read_bytes())!=b['sha256']:raise ValueError('method postrun binary changed')
    data=derive(packet=p)
    if historical_errors():raise ValueError('historical evidence/native inputs changed')
    for name,key in [('SUMMARY.json','summary'),('observations.json','observations'),('requirements.json','requirements'),('original-schedules.json','original_schedules')]:write(p/name,data[key])
    write(p/'native-reuse.json',native_record());write(p/'WORKLIST.json',worklist())
    (p/'REPORT.txt').write_text('Phase66: five-schedule source-bound actual method DEV observations; no independent acceptance\n\n'
        'Ten initial fixed owned processes (five candidate roles/four distinct exact retained API variants, general/race) are retained including failures/partials. An explicit correction covers every failed slot; the final ten complete bundles contain five correlated schedule observations each. The summary separates all actual attempts from valid complete bundles. Baseline2228 source entries, real project and build Git trees, fixture, binaries, allowlisted offline environment and source/binary preservation are bound. Original upstream tests/package initialization and native60second timer remain intact. No retries or operational hosts.\n'
        'A real owned memory core.BlockChain/real snap progress/controlled completed Downloader and test-owned StartEvent prime the unchanged native timer. Fresh post-done installations exercise normal delivery and an actual eventLoop unbuffered send whose sole direct consumer exits. Before rescue, ORIGINAL/reintroduction show paired method-send/eventLoop-send waits, A/B drain/return and service a fresh loop probe, and EARLY returns while the loop stays blocked. The primer and direct consumers do not prove actual native timer broadcast overlapping RPC callback exit.\n'
        'Three further controlled owners use actual Unsubscribe: three finite late sends after request acceptance and before acknowledgement, delayed acknowledgement, and three concurrent callers of the same exact object. All non-EARLY roles produce one request/zero individual pre-ack returns and join after acknowledgement; EARLY produces no request/ack. Controlled ownership has no backend map. Method return and probe liveness do not instrument actual map deletion/ack or all post-ack channel use.\n'
        'The initial driver had an elapsed-wait-tag parsing defect and an unbounded probe; every failing/partial initial slot is retained with internal cleanup unknown and is covered by an explicitly fixed corrective plan, never automatic retry. Runtime frames may say chan send, 1 minutes; the hardened predicate accepts that header and bounds/joins probe installation. All50 final schedule observations and all12 requirements x10 are mapped conservatively:70 bounded observations with gaps and50 not exercised/uninstrumented. Full original5 and full12 satisfaction remain false. Source-coupled AI defect discrimination does not approve candidate membership, create independent tickets/families or meet sufficient repeats. Candidate set/selection/quality remain null. Prior actual RPC/native notification proof is retained separately.\n'
        'Observations freeze before each owned rescue. Eight actual method calls per process join, owned drains/loop/chain stop, all owned process groups reap with no members or owned temp children. Preserved package-init workers are not individually joined before process exit. No external network/listener/operational node, credentials, global configuration/install, clock changes, publishing or paid/external LLM. D04 unassigned, anonymous roles allowed, independent0/FINAL0/formal0of9/freeze null/execution false. CKS native tests/build/fmt/vet/lint proof reused only after exact input identity; not fresh CKS runs. All13 remaining groups preserved.\n')
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
    p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','index','check']);p.add_argument('--workspace',type=Path);p.add_argument('--corrective-workspace',type=Path);p.add_argument('--require-stored',action='store_true');p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args()
    if a.command=='prepare':prepare(a.workspace.resolve(),ROOT/PACKET,a.corrective_workspace.resolve());update_indices()
    elif a.command=='index':update_indices()
    else:
        errors=validate()+current_indices()
        if a.require_stored:errors+=storage_errors()
        if a.require_formal_gold:errors.append('Five-schedule source-bound DEV observations are not independent approved/frozen gold')
        print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
