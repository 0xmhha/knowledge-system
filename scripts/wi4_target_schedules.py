"""Bound actual schedule DEV observations without promoting injected state to gold."""
import argparse,gzip,json,re,shutil,subprocess
from pathlib import Path
import wi4_target_udp as prior
import wi4_dev_gold as draft

ROOT,BASE=prior.ROOT,prior.BASE
PACKET=BASE/'followups/wi4-target-schedules-v1'
REVISION='6094049ffc65e3ee77aec6469b74fc65c186a422'
KIND='WI4_actual_WORK_GOSSIP_POOL_DEV_not_independent_acceptance'
TOOLS=['scripts/'+n for n in ('wi4_target_schedules.py','test_wi4_target_schedules.py','run-wi4-target-schedules.py','fixtures/wi4-owned-work.go.fixture','fixtures/wi4-owned-gossip.go.fixture','fixtures/wi4-owned-pool.go.fixture','run-wi4-target-subscription.py','wi4_target_udp.py','wi4_dev_gold.py','check-operational-acceptance.py','test_operational_acceptance.py','run-wi4-gossip-corrective.py','fixtures/wi4-owned-gossip-engine.go.fixture')]
GROUPS={'WORK':('miner',['WORK-normal','WORK-exit','WORK-both','WORK-backlog']),'GOSSIP':('consensus/wbft/backend',['GOSSIP-normal','GOSSIP-fail-repeat','GOSSIP-delay-repeat','GOSSIP-cache-control']),'POOL':('core/txpool/blobpool',['POOL-normal','POOL-lock-cycle','POOL-buffer','POOL-unsubscribe'])}
COMMIT=prior.prior.COMMIT

write,read,raw,sha=prior.write,prior.read,prior.raw,prior.sha


def same_frame(text,*parts):return any(all(p in g for p in parts)for g in text.split('\n\n'))


def inspect_slot(q,slot,binary,fixture_sha,known_fixture_failure=False):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    if any(r.get(k)!=v for k,v in {**process,**slot}.items()):raise ValueError('slot/process identity differs')
    if r['binary_sha256']!=binary['sha256']or r['source_test_addition_sha256']!=fixture_sha:raise ValueError('actual binary/fixture differs')
    if r['argv']!=[binary['path'],'-test.v=true','-test.run=^TestWI4OwnedSchedules$','-test.count=1','-test.timeout=20s']or (r['exit_code']!=(2 if known_fixture_failure else 0))or r['timeout']or not r['owned_new_session']or not r['reaped']or r['remaining_live_group_members']or r['tmp_children_after']:raise ValueError('actual trial failed or cleanup unproved; preserve original attempt')
    for ext in ('stdout','stderr'):
        if sha(raw(q/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('actual raw output differs')
    rows=[json.loads(l)for l in raw(q/'events.jsonl.gz').decode().splitlines()if l.strip()];names=[e['event']for e in rows]
    if [e['sequence']for e in rows]!=list(range(1,len(rows)+1))or any(e['case_id']!=slot['case_id']for e in rows)or [e['elapsed_ns']for e in rows]!=sorted(e['elapsed_ns']for e in rows):raise ValueError('actual trace identity/order differs')
    if known_fixture_failure:
        if slot['group']!='GOSSIP'or names!=['trace_begin','cleanup_begin','cleanup_end','trace_end']or rows[2].get('joined')is not True or rows[2].get('fake_calls')!=0:raise ValueError('original nil-engine fixture failure erased/relabelled')
        output=raw(q/'process/stdout.gz')+raw(q/'process/stderr.gz')
        if any(t not in output for t in (b'panic: runtime error: invalid memory address',b'engine.(*Engine).Address',b'backend.(*Backend).Gossip')):raise ValueError('original fixture panic cause missing')
        return dict(**slot,route='initial_owned_fixture_uninitialized_Engine_Address',binary_sha256=binary['sha256'],driver_sha256=fixture_sha,trace_sha256=sha(raw(q/'events.jsonl.gz')),valid_technical_observation=False,full_original_driver_acceptance=False,full_production_host_entry_proved=False,independent_oracle=False,product_violation_established=False,quality_score=None,outcome='fixture_nil_engine_panic_before_send_preserved',trace_test_failed_flag=rows[-1]['test_failed'],authoritative_process_exit=2)
    def one(name):
        matches=[e for e in rows if e['event']==name]
        if len(matches)!=1:raise ValueError('missing/duplicate transition '+name)
        return matches[0]
    def ordered(*labels):
        for label in labels:one(label)
        if [names.index(n)for n in labels]!=sorted(names.index(n)for n in labels):raise ValueError('observation/source ordering differs')
    begin=one('trace_begin');obs=one('observation_frozen')
    if begin['source_commit']!=COMMIT or begin['step_watchdog_ms']!=2000 or begin['production_host']is not False or one('trace_end')['test_failed']is not False or one('cleanup_end')['joined']is not True:raise ValueError('source/timing/cleanup scope differs')
    ordered('trace_begin','observation_frozen','cleanup_begin','cleanup_end','trace_end')
    cid,g=slot['case_id'],slot['group']
    if g=='WORK':
        if one('actual_dispatch_started')['Anzeon_nil']is not True or one('actual_dispatch_started')['route']!='actual_newWorkLoop_Origin_controlled_head'or one('actual_pending_lock_barrier')['before_commit']is not True:raise ValueError('Origin entry/barrier unproved')
        if not same_frame(raw(q/'events.jsonl.barrier.stack.gz').decode(),'(*worker).newWorkLoopOrigin','sync.(*Mutex).lockSlow','/miner/worker.go:'):raise ValueError('actual same-goroutine lock barrier missing')
        ready=one('selection_ready');expected=(cid!='WORK-exit',cid!='WORK-normal')
        if (ready['send_ready'],ready['exit_ready'])!=expected:raise ValueError('send/exit readiness changed')
        ordered('actual_dispatch_started','actual_pending_lock_barrier','selection_ready','observation_frozen')
        total=4 if cid=='WORK-backlog'else 1
        if obs['finite_backlog']!=total or not 1<=obs['head_events_consumed']<=total or obs['remaining_backlog']!=total-obs['head_events_consumed']or not 0<=obs['delivered_work']<=obs['head_events_consumed']or obs['loop_returned']is not True or obs['fairness_or_SLA_proved']is not False or obs['unbuffered_receiver_precondition_proved']is not False:raise ValueError('finite select observation falsely became fixed branch/fairness/receiver SLA')
        if cid=='WORK-normal':
            if obs['delivered_work']!=1 or ready['receiver_kind']!='owned_buffer_capacity':raise ValueError('normal ready-buffer result differs')
            ordered('selection_ready','actual_work_received','observation_frozen')
        elif cid=='WORK-exit'and obs['delivered_work']!=0:raise ValueError('unready send delivered')
        outcome='finite_Origin_select_observation';route='actual_newWorkLoop_Origin_controlled_head_buffer'
    elif g=='GOSSIP':
        first=one('actual_first_cache_and_send');attempts=2 if cid=='GOSSIP-cache-control'else 1;calls=1 if cid=='GOSSIP-normal'else 2;held=cid=='GOSSIP-delay-repeat';failed=cid=='GOSSIP-fail-repeat'
        if first['calls']!=1 or first['self_notifications']!=1 or first['fake_error']!=failed or first['sender_held']!=held:raise ValueError('initial cache/fake input differs')
        if obs['fake_send_attempts']!=attempts or obs['broadcast_calls']!=calls or obs['all_Broadcast_returns_nil']is not True or obs['first_fake_error']!=failed or obs['first_sender_still_held']!=held or obs['network_delivery_proved']is not False or obs['product_violation_established']is not False:raise ValueError('fake send/cache observations falsely promoted')
        completions=[e for e in rows if e['event']=='fake_sender_completed']
        if len(completions)!=attempts or any(e['error']!=failed or e['network_delivery_proved']is not False for e in completions):raise ValueError('owned sender completion count differs')
        if held:
            if not one('cleanup_begin')['elapsed_ns']<=completions[0]['elapsed_ns']<one('cleanup_end')['elapsed_ns']:raise ValueError('held sender released before observation')
        elif any(e['elapsed_ns']>=obs['elapsed_ns']for e in completions):raise ValueError('completed fake sender not joined before observation')
        if one('cleanup_end')['fake_calls']!=attempts:raise ValueError('owned sender cleanup differs')
        ordered('actual_first_cache_and_send','observation_frozen');outcome='owned_fake_peer_cache_observation';route='actual_Broadcast_Gossip_cache_and_owned_mux_fake_peer'
    else:
        initialized=one('actual_pool_initialized')
        if initialized['limbo_seeded_blob']is not True or initialized['StateAt_successes']<1 or initialized['owned_chain_not_operational']is not True:raise ValueError('owned real pool setup unproved')
        if cid in ('POOL-lock-cycle','POOL-unsubscribe'):
            block=one('actual_Reset_Send_blocked');window=one('blocked_observation_frozen')
            if block['nonempty_reinject_proved']is not True or block['StateAt_successes']<2 or window['observation_ms']!=2000 or window['source_Send_waiting']is not True or window['Stats_waiting']!=(cid=='POOL-lock-cycle'):raise ValueError('actual nonempty reorg/StateAt/waits unproved')
            ordered('actual_pool_initialized','actual_Reset_Send_blocked','blocked_window_begin','blocked_observation_frozen','observation_frozen')
            if window['elapsed_ns']-one('blocked_window_begin')['elapsed_ns']<2_000_000_000:raise ValueError('blocked observation shortened')
            for phase in ('before-observation','after-observation'):
                stack=raw(q/('events.jsonl.'+phase+'.stack.gz')).decode()
                if not same_frame(stack,'(*BlobPool).Reset','(*Feed).Send','/core/txpool/blobpool/blobpool.go:818'):raise ValueError('actual same-goroutine Reset Send stack missing')
                if cid=='POOL-lock-cycle'and not same_frame(stack,'(*BlobPool).Stats','sync.(*Mutex).lockSlow','/core/txpool/blobpool/blobpool.go:'):raise ValueError('actual subscriber Stats wait missing')
            if cid=='POOL-lock-cycle':
                ordered('Stats_invoke','actual_Stats_lock_blocked','blocked_observation_frozen','observation_frozen','cleanup_begin','Stats_returned','subscriber_receive_after_Stats','cleanup_end')
                ordered('cleanup_begin','actual_Reset_returned','cleanup_end')
                if obs['controlled_wait_cycle']is not True or obs['production_reachability_proved']is not False:raise ValueError('controlled subscriber cycle falsely promoted')
                if one('subscriber_receive_after_Stats')['received']is not False:raise ValueError('cleanup channel cancellation conflated with successful target receive')
            else:
                ordered('blocked_observation_frozen','owned_unsubscribe_invoke','actual_Reset_returned','Stats_invoke','Stats_returned','observation_frozen')
        elif cid=='POOL-normal':
            if one('actual_event_received')['txs']!=1:raise ValueError('actual reinjected event missing')
            ordered('actual_pool_initialized','actual_event_received','subscriber_Stats_returned','observation_frozen');ordered('actual_Reset_returned','observation_frozen')
        elif cid=='POOL-buffer':
            ordered('actual_Reset_returned','Stats_invoke','Stats_returned','observation_frozen')
            if obs['one_empty_buffer_slot']is not True:raise ValueError('finite empty buffer input differs')
        else:raise ValueError('unknown pool schedule')
        if cid!='POOL-lock-cycle':
            key='Reset_and_subscriber_returned'if cid=='POOL-normal'else'Reset_and_Stats_returned'
            if obs[key]is not True or obs['nonempty_reinject_proved']is not True:raise ValueError('nonempty real Reset completion unproved')
        outcome='actual_nonempty_reorg_reinject_subscription_Stats_observation';route='actual_Reset_reorg_reinject_owned_chain_stores_subscriber'
    return dict(**slot,route=route,binary_sha256=binary['sha256'],driver_sha256=fixture_sha,trace_sha256=sha(raw(q/'events.jsonl.gz')),valid_technical_observation=True,full_original_driver_acceptance=False,full_production_host_entry_proved=False,independent_oracle=False,product_violation_established=False,quality_score=None,outcome=outcome)


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;q=p/'actual';scope=read(p/'SCOPE.json');binding=read(q/'source-binding.json');receipt=read(q/'receipt.json');binaries=read(q/'binaries.json')
    slots=[dict(slot_id=f'{v}-r{r}-{c}',variant=v,repetition=r,group=g,case_id=c)for v in ['general','race']for r in [1,2,3]for g,(_,cs)in GROUPS.items()for c in cs]
    if scope['planned_slots']!=slots or read(q/'planned.json')!=slots or read(q/'SCOPE.json')!=scope:raise ValueError('predeclared slots changed')
    if scope['acceptance_limits']!=dict(new_independent_families=0,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,full_original_driver_acceptance=0,freeze=None,execution_allowed=False,quality_score=None):raise ValueError('scope falsely accepted')
    fixture_sha={g:sha((root/TOOLS[i]).read_bytes())for i,g in enumerate(GROUPS,3)}
    additions={package+'/wi4_owned_schedules_test.go':dict(sha256=fixture_sha[g],group=g)for g,(package,_)in GROUPS.items()}
    expected=read(root/prior.prior.PACKET/'initial/source-binding.json')
    if binding['source_commit']!=COMMIT or binding['tracked']!=expected['tracked']or binding['production_patch']is not False or binding['test_additions']!=additions:raise ValueError('unchanged upstream source/test binding differs')
    if read(q/'profile.json')!=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'):raise ValueError('actual toolchain differs')
    env=read(q/'environment.json')
    if env['credential_environment_inherited']is not False or set(env['allowed_inherited_keys'])-{'HOME','USER','TMPDIR','LANG','LC_ALL'}or any(env['task_settings'].get(k)!=v for k,v in dict(GOPROXY='off',GOSUMDB='off',GOVCS='*:off',GOTOOLCHAIN='local',GOFLAGS='-mod=readonly',GOMODCACHE='/Users/kevin/.gvm/pkgsets/go1.23.12/global/pkg/mod').items()):raise ValueError('offline/credential/cache boundary differs')
    if receipt!=dict(planned=72,attempted=72,retries=0,tracked_source_preserved=True,only_test_addition=True,production_patch=False,source_commit=COMMIT,independent_approved=0,new_independent_families=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0):raise ValueError('all-attempt/source scope differs')
    if set(binaries)!=set(GROUPS)or read(q/'binary-postrun-check.json')!=dict(observed_binary_sha256={g:{v:b['sha256']for v,b in bs.items()}for g,bs in binaries.items()},all_match_build_receipts=True):raise ValueError('actual target binary set/bytes differs')
    for g,bs in binaries.items():
        if set(bs)!={'general','race'}:raise ValueError('general/race build omitted')
        for v,b in bs.items():
            br=read(q/('build-'+g+'-'+v)/'receipt.json');argv=br['argv']
            if argv!=[argv[0],'test']+(['-race']if v=='race'else[])+['-c','-o',b['path'],'./'+GROUPS[g][0]]or br['exit_code']or br['timeout']or not br['reaped']or br['remaining_live_group_members']:raise ValueError('target build failed/narrowed')
            for ext in ('stdout','stderr'):
                if sha(raw(q/('build-'+g+'-'+v)/(ext+'.gz')))!=br[ext+'_sha256']:raise ValueError('target build raw log differs')
    attempts=read(q/'attempts.json')
    if len(attempts)!=72:raise ValueError('actual planned attempt missing')
    gold={r['case_id']:r for r in read(root/draft.PACKET/'private/AI-gold.json')};rows=[]
    for slot,attempt in zip(slots,attempts):
        directory=q/'slots'/slot['slot_id']
        if read(directory/'receipt.json')!=attempt:raise ValueError('aggregate slot differs')
        row=inspect_slot(directory,slot,binaries[slot['group']][slot['variant']],fixture_sha[slot['group']],slot['group']=='GOSSIP');row.update(environment_sha256=sha((q/'environment.json').read_bytes()),environment_scope='allowlisted_process_environment_not_independent_frozen_environment')
        row['formal_adapter_result']=draft.match_observation(dict(case_id=row['case_id'],family_proposal=gold[row['case_id']]['family_proposal'],source_commit=COMMIT,source_project='go-stablenet',source_mode='committed',binary_sha256=row['binary_sha256'],driver_sha256=row['driver_sha256'],environment_sha256=row['environment_sha256'],trace_sha256=row['trace_sha256'],entry_reached=True,preconditions_verified=False),gold[row['case_id']])
        if row['formal_adapter_result']['status']!='unknown_unverified_trial':raise ValueError('exposed controlled probe falsely promoted')
        rows.append(row)
    cq=p/'corrective';cs=read(p/'GOSSIP-CORRECTIVE-SCOPE.json');cslots=[dict(slot_id=f'{v}-r{r}-{c}-engine',variant=v,repetition=r,group='GOSSIP',case_id=c)for v in ['general','race']for r in [1,2,3]for c in GROUPS['GOSSIP'][1]]
    if cs['planned_slots']!=cslots or read(cq/'planned.json')!=cslots or read(cq/'SCOPE.json')!=cs or cs['original_72_preserved']is not True:raise ValueError('corrective plan changed')
    cb=read(cq/'source-binding.json');ch=sha((root/TOOLS[-1]).read_bytes());cbins=read(cq/'binaries.json')
    if cb['tracked']!=binding['tracked']or cb['source_commit']!=COMMIT or cb['production_patch']is not False or cb['test_additions']!={'consensus/wbft/backend/wi4_owned_schedules_test.go':dict(sha256=ch,group='GOSSIP')}:raise ValueError('corrective source/fixture binding differs')
    if read(cq/'profile.json')!=read(q/'profile.json')or read(cq/'environment.json')['task_settings']!=env['task_settings']or read(cq/'environment.json')['credential_environment_inherited']is not False:raise ValueError('corrective toolchain/environment differs')
    cr=read(cq/'receipt.json');expected_cr=dict(receipt,planned=24,attempted=24)
    if cr!=expected_cr or set(cbins)!={'GOSSIP'}or set(cbins['GOSSIP'])!={'general','race'}:raise ValueError('corrective attempts/builds omitted')
    if read(cq/'binary-postrun-check.json')!=dict(observed_binary_sha256={g:{v:b['sha256']for v,b in bs.items()}for g,bs in cbins.items()},all_match_build_receipts=True):raise ValueError('corrective postrun binary differs')
    for v,b in cbins['GOSSIP'].items():
        br=read(cq/('build-GOSSIP-'+v)/'receipt.json');argv=br['argv']
        if argv!=[argv[0],'test']+(['-race']if v=='race'else[])+['-c','-o',b['path'],'./consensus/wbft/backend']or br['exit_code']or br['timeout']or not br['reaped']or br['remaining_live_group_members']:raise ValueError('corrective target build failed')
        for ext in ('stdout','stderr'):
            if sha(raw(cq/('build-GOSSIP-'+v)/(ext+'.gz')))!=br[ext+'_sha256']:raise ValueError('corrective build raw log differs')
    ca=read(cq/'attempts.json')
    if len(ca)!=24:raise ValueError('corrective attempt missing')
    for slot,attempt in zip(cslots,ca):
        directory=cq/'slots'/slot['slot_id']
        if read(directory/'receipt.json')!=attempt:raise ValueError('corrective aggregate differs')
        row=inspect_slot(directory,slot,cbins['GOSSIP'][slot['variant']],ch);row.update(environment_sha256=sha((cq/'environment.json').read_bytes()),environment_scope='allowlisted_process_environment_not_independent_frozen_environment')
        row['formal_adapter_result']=draft.match_observation(dict(case_id=row['case_id'],family_proposal='SCHED-004',source_commit=COMMIT,source_project='go-stablenet',source_mode='committed',binary_sha256=row['binary_sha256'],driver_sha256=row['driver_sha256'],environment_sha256=row['environment_sha256'],trace_sha256=row['trace_sha256'],entry_reached=True,preconditions_verified=False),gold[row['case_id']])
        if row['formal_adapter_result']['status']!='unknown_unverified_trial':raise ValueError('corrective probe falsely promoted')
        rows.append(row)
    initial=read(p/'preflight/initial-attempt.json');br=read(p/'preflight/attempt1/build-WORK-general/receipt.json')
    if initial['initial_runner_exit']!=1 or initial['actual_target_slots']!=0 or initial['actual_general_build_exit']!=1 or br['exit_code']!=1 or br['remaining_live_group_members']or br['timeout']:raise ValueError('initial build failure erased or counted as trial')
    for ext in ('stdout','stderr'):
        if sha(raw(p/('preflight/attempt1/build-WORK-general/'+ext+'.gz')))!=br[ext+'_sha256']:raise ValueError('initial build raw error differs')
    if b'module lookup disabled by GOPROXY=off'not in raw(p/'preflight/attempt1/build-WORK-general/stderr.gz'):raise ValueError('initial cache failure cause missing')
    old=read(p/'preflight/attempt1/source-binding.json')
    for n,row in old['test_additions'].items():
        name='wi4-owned-'+row['group'].lower()+'.go.fixture.gz'
        if sha(raw(p/'preflight/attempt1-tools'/name))!=row['sha256']:raise ValueError('initial fixture inputs lost')
    return dict(summary=dict(kind=KIND,planned_attempts=96,actual_attempts=96,valid_technical_observations=72,original_fixture_failures_preserved=24,WORK_valid=24,GOSSIP_valid=24,POOL_valid=24,original_WI4_schedules=21,total_existing_schedules_with_limited_DEV_probes=21,unprobed_original_case_ids=0,full_original_driver_acceptances=0,full_production_host_entry_proved=0,preflight_build_failure_preserved=1,target_trial_retries=0,tracked_upstream_sources=len(binding['tracked']),target_production_patch=False,independent_families=0,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0),observations=rows)


def historical_errors(root=ROOT):
    errors=prior.historical_errors(root);p=root/prior.PACKET;m=read(p/'manifest.json')
    if(p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(prior.PACKET/'manifest.json')],cwd=root):errors.append('stage62 manifest changed')
    for n,h in m['artifacts'].items():
        if sha((p/n).read_bytes())!=h:errors.append('stage62 evidence changed '+n)
    for r in m['tools']:
        if sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('stage62 tool at signed revision differs')
    return errors


def seal(p):write(p/'manifest.json',dict(kind=KIND,previous_source_revision=REVISION,tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={x.relative_to(p).as_posix():sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))


def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=sha((root/PACKET/'manifest.json').read_bytes()),status='WI4_actual_WORK_GOSSIP_POOL_DEV_not_independent_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def update_indices():
    p=ROOT/PACKET;seal(p);ref=reference();state=read(p/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);base=ROOT/'docs/spec-driven'
    proposal=base/'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json';d=read(proposal);d.update(wi4_target_schedules=ref,wi4_target_udp=dict(d['wi4_target_udp'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state);write(proposal,d)
    link=dict(path=str(proposal.relative_to(ROOT)),sha256=sha(proposal.read_bytes()),status='D04_review_pending_WI4_actual_schedule_DEV_not_independent_acceptance')
    for name in ('OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','operational-preparation/source-review-supplement/GAPS.json'):
        q=base/name;d=read(q);d.update(wi4_target_schedules=ref,wi4_target_udp=dict(d['wi4_target_udp'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state,adoption_acceptance_decision_proposal=link)
        if q.name=='OPERATIONAL-ACCEPTANCE-PROGRESS.json':d['phases'][0]['evidence'].update(wi4_target_schedules=ref,wi4_target_udp=d['wi4_target_udp'],current_readiness_preparation=current,adoption_acceptance_decision_proposal=link)
        write(q,d)


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[read(root/n)for n in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('wi4_target_schedules')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current schedule reference/state differs')
        if d.get('reviewer_identity_policy')!=old.get('reviewer_identity_policy'):errors.append('privacy policy changed')
        for k,v in old.items():
            if k=='current_readiness_preparation'or not isinstance(v,dict)or'manifest'not in v:continue
            expected=dict(v)
            if k=='wi4_target_udp':expected['historical_source_revision']=REVISION
            if d.get(k)!=expected:errors.append('historical preparation changed '+k)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical schedule probes cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('current proposal link differs')
    return errors


def prepare(work,p):
    if any(x.name not in ('SCOPE.json','BUILD-RECOVERY.json','GOSSIP-CORRECTIVE-SCOPE.json','preflight')for x in p.iterdir()):raise ValueError('refusing existing evidence overwrite')
    for label,w in [('actual',work),('corrective',work.parent/(work.name+'-gossip'))]:
        for x in sorted((w/'schedule-trials-v1').rglob('*')):
            if not x.is_file()or x.suffix=='.test':continue
            y=p/label/x.relative_to(w/'schedule-trials-v1');y.parent.mkdir(parents=True,exist_ok=True)
            if x.name in ('stdout','stderr')or x.suffix in ('.jsonl','.stack'):y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
            else:shutil.copyfile(x,y)
        for bs in read(p/label/'binaries.json').values():
            for v,b in bs.items():
                if sha(Path(b['path']).read_bytes())!=b['sha256']:raise ValueError('actual binary changed '+v)
    d=derive(packet=p)
    if historical_errors():raise ValueError('historical/native preservation failed')
    write(p/'SUMMARY.json',d['summary']);write(p/'observations.json',d['observations']);write(p/'native-reuse.json',prior.prior.native_reuse_record())
    state=read(ROOT/prior.PACKET/'WORKLIST.json');state.update(current_stage='63_WI4_WORK_GOSSIP_POOL_actual_controlled_DEV_paths',completed_in_this_scope=['96_fixed_actual_attempts_72_valid_24_nil_engine_fixture_failures_preserved_source_unchanged','Origin_select_readiness_barrier_fake_peer_Broadcast_Gossip_and_nonempty_real_Reset_paths','initial_offline_cache_build_failure_and_old_inputs_preserved_no_target_retries'],next_task='WI_V1_V2_actual_API_independent_ticket_oracle_candidate_set_debts_then_other_WI',last_verified_date='2026-10-19')
    state['remaining_all'][0]['task']='WI4 all21 limited exposed DEV probes are recorded; full source/host/receiver/timing contracts, independent families/oracles and sufficiency remain. WI-V1/V2 actualAPI/independent tickets/acceptable candidates; WI6 relevance/noise/nonseed/pairs/warm; WI-SP actual overlay/ACL/provenance; WI-C required items/report/diagrams/handoff/installed role; WI1 remaining53occurrences/3blocks/type/value/memory/independent semantics; WI2 independent semantics/build profile/support/installed role; WI3 independent families/default budget/startup/site reasons; independent DEV/FINAL/full gold/matching/sufficiency/F01~08'
    state['AI_DEV_draft_progress']['WI-4']='21_conditional_AI_DEV_drafts_with_limited_actual_DEV_probes;full_driver_acceptance0_independent_gold0'
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI4 WORK/GOSSIP/POOL actual controlled DEV paths; no independent acceptance\n\n'
        'Original72 plus corrective24 fixed slots: general/race x3 x12 and GOSSIP x3 x4 across three unchanged upstream packages at0bf2f4d1 with declared Go1.23.12. Only three test additions; all2228 tracked files preserved. First WORK build failed before any target trial because the isolated default cache lacked pinned blst0.3.16. Its original receipt/output/test inputs remain. Recovery explicitly uses the existing Go1.23 local module cache with offline flags; no unrecorded target trial retry. Initial24 Gossip attempts panic before send because the test Backend omitted its Engine, whose Address method is called by Gossip. The process exit2 is authoritative even though t.Failed was false during panic cleanup. All24 failures and raw traces stay retained; a separately preregistered24-slot plan initializes NewEngine without starting an engine/node and supplies the corrected Gossip observations. Subscriber Stats-then-receive was corrected before first target execution.\n'
        'WORK invokes actual newWorkLoop dispatch with Anzeon nil and controlled head events. A real pendingMu contention barrier establishes send/exit readiness before commit. Normal uses an owned ready buffer, not proof of an unbuffered receiver. Both-ready outcomes and finite backlog consumption vary legally; no chosen branch frequency, infinite fairness or production exit SLA is inferred.\n'
        'GOSSIP invokes actual Broadcast validator membership, Gossip/cache and owned self-event mux. One owned fake peer reports success/error/held completion. Same peer/hash suppresses a repeated attempt after fake failure or before completion; a distinct hash permits another attempt. Nil Broadcast or fake success never proves network delivery. Owned sender/mux termination is checked before cleanup completion.\n'
        'POOL invokes actual New/Init/Reset/reorg/reinject/SubscribeTransactions/Stats using owned temporary stores, an ephemeral test key and a controlled chain. The old/new branch and limbo blob generate one real successfully reinjected add; StateAt succeeds. Normal receive-then-Stats and one empty buffer complete. Controlled sole subscriber Stats-before-receive creates source-stack-proved Reset/Feed.Send and Stats/lock waits for2000ms before rescue. Another case proves owned unsubscribe releases Send and subsequent Stats. This is a controlled component route, not proof an operational consumer does this.\n'
        'All observations are frozen before cleanup/rescue, except unsubscribe explicitly belongs to its declared trial steps after the blocked observation. Owned process groups and helper/store cleanup are verified. No external peer/socket/node control, system clock changes, credential environment inheritance, global install/settings, publication or paid model calls.\n'
        'The prior5 subscription and4 UDP plus these12 cases cover existing21 exposed case IDs only with limited DEV probes. Full original driver/production host acceptance0; independent0/FINAL0/formal0of9/freeze null/execution false. No invented seed/frozen comparison arm or independent product oracle; formal adapter rejects all rows. D04 remains unassigned with no personal names/contact needed.\n'
        'CKS native inputs/configs remain identical: prior4158 general and4158 race/18skip each are reused, not called fresh. Target package general/race binaries are fresh and separate. All full entry/receiver/timing, independent/family/statistical contracts, other WI and ontology effect debts remain in WORKLIST.json. Signed storage is checked against actual Git with --require-stored.\n')
    seal(p)


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=read(p/'manifest.json');actual={x.relative_to(p).as_posix():sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('schedule evidence inventory differs')
        for r in m['tools']:
            if sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('bound schedule tool changed '+r['path'])
        errors+=historical_errors(root);d=derive(root,p)
        if read(p/'SUMMARY.json')!=d['summary']or read(p/'observations.json')!=d['observations']:errors.append('derived schedule observations differ')
        if read(p/'native-reuse.json')!=prior.prior.native_reuse_record(root):errors.append('unchanged native proof reuse differs')
        state=read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정'or state['independent_quality_or_ontology_acceptance']is not False:errors.append('technical worklist falsely accepted')
    except (ValueError,KeyError,TypeError,OSError,IndexError)as exc:errors.append(str(exc))
    return errors


def storage_errors(root=ROOT):
    try:
        code=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=root,text=True).strip()
        if not re.fullmatch('[0-9a-f]{40}',code):return ['real signed schedule storage pending']
        names=[PACKET/'manifest.json',*[Path(n)for n in TOOLS],*[Path('docs/spec-driven')/n for n in ('CURRENT-HANDOFF.md','KNOWLEDGE-ROADMAP-TEST-MATRIX.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json')],BASE/'GAPS.json']
        if any(subprocess.check_output(['git','show',code+':'+str(n)],cwd=root)!=(root/n).read_bytes()for n in names):return ['committed schedule tools/evidence differ']
        r=subprocess.run(['git','verify-commit',code],cwd=root,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        return []if r.returncode==0 else ['actual schedule commit signature not verified']
    except (OSError,subprocess.CalledProcessError):return ['real signed schedule storage pending']


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','index','check']);p.add_argument('--workspace',type=Path);p.add_argument('--require-stored',action='store_true');p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args()
    if a.command=='prepare':prepare(a.workspace.resolve(),ROOT/PACKET);update_indices()
    elif a.command=='index':update_indices()
    else:
        errors=validate()+current_indices()
        if a.require_stored:errors+=storage_errors()
        if a.require_formal_gold:errors.append('Actual exposed schedule probes and injected state are not independent approved/frozen gold')
        print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
