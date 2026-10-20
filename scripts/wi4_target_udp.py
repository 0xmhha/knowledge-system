"""Bound actual UDP DEV observations without promoting injected state to gold."""
import argparse,gzip,json,re,shutil,subprocess
from pathlib import Path
import wi4_target_subscription as prior
import wi4_dev_gold as draft

ROOT,BASE=prior.ROOT,prior.BASE
PACKET=BASE/'followups/wi4-target-udp-v1'
REVISION='e30579aeee3de5e7353e3f6c35216f4a9b55c7e1'
KIND='WI4_actual_UDP_DEV_paths_not_independent_acceptance'
TOOLS=['scripts/'+n for n in ('wi4_target_udp.py','test_wi4_target_udp.py','run-wi4-target-udp.py','fixtures/wi4-owned-udp.go.fixture','run-wi4-target-subscription.py','wi4_target_subscription.py','wi4_dev_gold.py','check-operational-acceptance.py','test_operational_acceptance.py')]
CASES=['UDP-near','UDP-far-pair','UDP-far-near','UDP-response-timeout']
write,read,raw,sha=prior.write,prior.read,prior.raw,prior.sha


def inspect_slot(q,slot,binary,fixture_sha):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    if any(r.get(k)!=v for k,v in {**process,**slot}.items()):raise ValueError('slot/process identity differs')
    if r['binary_sha256']!=binary['sha256']or r['source_test_addition_sha256']!=fixture_sha:raise ValueError('actual binary/fixture differs')
    expected=[binary['path'],'-test.v=true','-test.run=^TestWI4OwnedUDP$','-test.count=1','-test.timeout=15s']
    if r['argv']!=expected or r['exit_code']!=0 or r['timeout']or not r['owned_new_session']or not r['reaped']or r['remaining_live_group_members']or r['tmp_children_after']:raise ValueError('target slot failed or owned resources unproved; preserve raw attempt')
    for ext in ('stdout','stderr'):
        if sha(raw(q/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('target raw output differs')
    rows=prior.events(q/'events.jsonl.gz');names=[r['event']for r in rows];by={r['event']:r for r in rows}
    if [e['sequence']for e in rows]!=list(range(1,len(rows)+1))or any(e['case_id']!=slot['case_id']for e in rows)or [e['elapsed_ns']for e in rows]!=sorted(e['elapsed_ns']for e in rows):raise ValueError('target trace order/identity differs')
    prefix=['trace_begin','actual_loop_started','actual_registration_barrier'];suffix=['observation_frozen','cleanup_begin','cleanup_end','trace_end']
    cid=slot['case_id'];middle=['actual_pending_pair','native_timeout_1','native_timeout_2']if cid=='UDP-near'else['one_reply_invoke']if cid=='UDP-response-timeout'else['gate_reply_invoke','synthetic_deadlines_injected','actual_clockwarp_head_received','tail_pending_window_begin']
    if names!=prefix+middle+suffix:raise ValueError('target event boundary/inventory differs')
    begin=by['trace_begin']
    if begin['source_commit']!=prior.COMMIT or begin['resp_timeout_ns']!=500_000_000 or begin['step_watchdog_ms']!=2000 or begin['constructed_listener']is not False or begin['opened_socket']is not False:raise ValueError('native timing/no-socket scope differs')
    if by['actual_loop_started']['route']!='minimal_owned_channels_actual_UDPv4_loop'or by['cleanup_end']['joined']is not True or by['trace_end']['test_failed']is not False:raise ValueError('actual entry/cleanup not proved')
    obs=by['observation_frozen'];synthetic=cid in ('UDP-far-pair','UDP-far-near')
    if cid=='UDP-near':
        p=by['actual_pending_pair']
        if p['synthetic_deadlines']is not False or p['single_reset_invocation_count_known']is not False or not 0<=p['deadline_order_ns']<500_000_000:raise ValueError('native registration order/single-reset claim differs')
        if obs['timeouts']!=2 or obs['reset_invocations']is not None or obs['full_original_driver_acceptance']is not False:raise ValueError('native timeout coverage falsely promoted')
        if by['native_timeout_1']['elapsed_ns']-by['actual_loop_started']['elapsed_ns']<450_000_000:raise ValueError('native timeout shortcut')
        outcome='two_native_timeouts_single_reset_count_unobserved'
    elif cid=='UDP-response-timeout':
        if by['one_reply_invoke']['registered']!=2 or by['one_reply_invoke']['selected_arm']!='response':raise ValueError('actual two-matcher response input differs')
        if obs['selected_arm']!='response'or obs['callback_counts']!=[1,0]or obs['first_result']!='nil'or obs['second_pending']is not True or obs['synthetic_deadlines']is not False or obs['product_violation_established']is not False:raise ValueError('actual one-response observation differs')
        outcome='one_response_first_callback_only_second_pending'
    else:
        gate,inj=by['gate_reply_invoke'],by['synthetic_deadlines_injected']
        if gate['registered']!=3 or gate['remaining_after_gate']!=2:raise ValueError('gate removal/two-element state differs')
        if inj['inside_actual_loop_callback']is not True or inj['production_reachability_proved']is not False or inj['head_distance_ns']!=2_000_000_000 or inj['tail_distance_ns']!=(2_500_000_000 if cid=='UDP-far-pair'else 500_000_000):raise ValueError('synthetic injection confused with production deadline')
        if obs['head_result']!='errClockWarp'or obs['tail_pending']is not True or obs['observation_ms']!=2000 or obs['synthetic_deadlines']is not True or obs['timer_stop_directly_observed']is not False or obs['production_reachability_proved']is not False:raise ValueError('conditional state observation falsely promoted')
        if obs['elapsed_ns']-by['tail_pending_window_begin']['elapsed_ns']<2_000_000_000:raise ValueError('tail observation shortened')
        outcome='injected_far_head_clockwarp_tail_pending'
    return dict(**slot,route='minimal_owned_channels_actual_pending_handleReply_loop',binary_sha256=binary['sha256'],driver_sha256=fixture_sha,trace_sha256=sha(raw(q/'events.jsonl.gz')),valid_technical_observation=True,synthetic_deadline_injection=synthetic,full_listener_or_host_entry_proved=False,full_original_driver_acceptance=False,outcome=outcome,independent_oracle=False,product_violation_established=False,quality_score=None)


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;q=p/'actual';scope=read(p/'SCOPE.json');binding=read(q/'source-binding.json');receipt=read(q/'receipt.json');binaries=read(q/'binaries.json')
    slots=[dict(slot_id=f'{v}-r{r}-{c}',variant=v,repetition=r,case_id=c)for v in ['general','race']for r in [1,2,3]for c in CASES]
    if scope['planned_slots']!=slots or read(q/'planned.json')!=slots or read(q/'SCOPE.json')!=scope:raise ValueError('predeclared slots changed')
    if scope['production_patch']is not False or scope['acceptance_limits']!=dict(new_independent_families=0,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,full_original_driver_acceptance=0,freeze=None,execution_allowed=False,quality_score=None):raise ValueError('scope falsely accepted')
    expected=read(root/prior.PACKET/'initial/source-binding.json')
    fixture_sha=sha((root/TOOLS[3]).read_bytes())
    if binding['source_commit']!=prior.COMMIT or binding['tracked']!=expected['tracked']or binding['production_patch']is not False or binding['test_addition']!=dict(path='p2p/discover/wi4_owned_udp_test.go',sha256=fixture_sha):raise ValueError('unchanged upstream source/test binding differs')
    profile=read(q/'profile.json')
    if profile!=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'):raise ValueError('actual target toolchain differs')
    environment=read(q/'environment.json')
    if environment['credential_environment_inherited']is not False or set(environment['allowed_inherited_keys'])-{'HOME','USER','TMPDIR','LANG','LC_ALL'}or any(environment['task_settings'].get(k)!=v for k,v in dict(GOPROXY='off',GOSUMDB='off',GOVCS='*:off',GOTOOLCHAIN='local',GOFLAGS='-mod=readonly').items()):raise ValueError('offline/credential environment boundary differs')
    if receipt!=dict(planned=24,attempted=24,retries=0,tracked_source_preserved=True,only_test_addition=True,production_patch=False,source_commit=prior.COMMIT,independent_approved=0,new_independent_families=0,formal_completed=0,formal_total=9,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0):raise ValueError('all-attempt/source scope differs')
    if read(q/'binary-postrun-check.json')!=dict(observed_binary_sha256={v:b['sha256']for v,b in binaries.items()},all_match_build_receipts=True):raise ValueError('postrun binary bytes differ')
    if set(binaries)!={'general','race'}:raise ValueError('general/race build omitted')
    for v,b in binaries.items():
        br=read(q/('build-'+v)/'receipt.json');argv=br['argv']
        if argv!=[argv[0],'test']+(['-race']if v=='race'else[])+['-c','-o',b['path'],'./p2p/discover']or br['exit_code']or br['timeout']or not br['reaped']or br['remaining_live_group_members']:raise ValueError('target build failed/narrowed')
        for ext in ('stdout','stderr'):
            if sha(raw(q/('build-'+v)/(ext+'.gz')))!=br[ext+'_sha256']:raise ValueError('target build raw output differs')
    attempts=read(q/'attempts.json')
    if len(attempts)!=24:raise ValueError('actual attempt missing')
    gold={r['case_id']:r for r in read(root/draft.PACKET/'private/AI-gold.json')};rows=[]
    for slot,attempt in zip(slots,attempts):
        directory=q/'slots'/slot['slot_id']
        if read(directory/'receipt.json')!=attempt:raise ValueError('aggregate slot differs')
        row=inspect_slot(directory,slot,binaries[slot['variant']],fixture_sha)
        row.update(environment_sha256=sha((q/'environment.json').read_bytes()),environment_scope='allowlisted_process_environment_not_independent_frozen_environment')
        row['formal_adapter_result']=draft.match_observation(dict(case_id=row['case_id'],family_proposal='SCHED-002',source_commit=prior.COMMIT,source_project='go-stablenet',source_mode='committed',binary_sha256=row['binary_sha256'],driver_sha256=row['driver_sha256'],environment_sha256=row['environment_sha256'],trace_sha256=row['trace_sha256'],entry_reached=True,preconditions_verified=False),gold[row['case_id']])
        if row['formal_adapter_result']['status']!='unknown_unverified_trial':raise ValueError('exposed controlled probe falsely promoted')
        rows.append(row)
    preflight=read(p/'preflight/initial-attempt.json')
    if preflight['initial_runner_exit']!=1 or preflight['actual_target_builds']!=0 or preflight['actual_target_slots']!=0 or preflight['subcommand_exit_not_persisted']is not True:raise ValueError('original preflight failure erased/relabelled')
    return dict(summary=dict(kind=KIND,actual_attempts=24,planned_attempts=24,valid_technical_observations=24,native_pending_near_timeout_observations=6,native_pending_one_response_observations=6,controlled_synthetic_deadline_observations=12,full_listener_or_host_entry_proved=0,single_reset_invocation_count_proved=0,full_original_driver_acceptances=0,target_source_files=len(binding['tracked']),target_production_patch=False,original_WI4_schedules=21,subscription_cases_previously_probed=5,UDP_cases_probed=4,other12_schedule_probes_pending=True,preflight_permission_failures_preserved=1,target_build_or_trial_retries=0,independent_families=0,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0),observations=rows)


def historical_errors(root=ROOT):
    errors=prior.historical_errors(root);p=root/prior.PACKET;m=read(p/'manifest.json')
    if(p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(prior.PACKET/'manifest.json')],cwd=root):errors.append('stage61 manifest changed')
    for n,h in m['artifacts'].items():
        if sha((p/n).read_bytes())!=h:errors.append('stage61 evidence changed '+n)
    for r in m['tools']:
        if sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('stage61 tool at signed revision differs')
    return errors


def seal(p):write(p/'manifest.json',dict(kind=KIND,previous_source_revision=REVISION,tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={x.relative_to(p).as_posix():sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))


def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=sha((root/PACKET/'manifest.json').read_bytes()),status='WI4_actual_UDP_DEV_paths_not_independent_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def update_indices():
    p=ROOT/PACKET;seal(p);ref=reference();state=read(p/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);base=ROOT/'docs/spec-driven'
    proposal=base/'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json';d=read(proposal);d.update(wi4_target_udp=ref,wi4_target_subscription=dict(d['wi4_target_subscription'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state);write(proposal,d)
    link=dict(path=str(proposal.relative_to(ROOT)),sha256=sha(proposal.read_bytes()),status='D04_review_pending_WI4_actual_UDP_DEV_not_independent_acceptance')
    for name in ('OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','operational-preparation/source-review-supplement/GAPS.json'):
        q=base/name;d=read(q);d.update(wi4_target_udp=ref,wi4_target_subscription=dict(d['wi4_target_subscription'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state,adoption_acceptance_decision_proposal=link)
        if q.name=='OPERATIONAL-ACCEPTANCE-PROGRESS.json':d['phases'][0]['evidence'].update(wi4_target_udp=ref,wi4_target_subscription=d['wi4_target_subscription'],current_readiness_preparation=current,adoption_acceptance_decision_proposal=link)
        write(q,d)


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[read(root/n)for n in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('wi4_target_udp')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current UDP reference/state differs')
        if d.get('reviewer_identity_policy')!=old.get('reviewer_identity_policy'):errors.append('privacy policy changed')
        for k,v in old.items():
            if k=='current_readiness_preparation'or not isinstance(v,dict)or'manifest'not in v:continue
            expected=dict(v)
            if k=='wi4_target_subscription':expected['historical_source_revision']=REVISION
            if d.get(k)!=expected:errors.append('historical preparation changed '+k)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical UDP probes cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('current proposal link differs')
    return errors


def prepare(work,p):
    if any(x.name not in ('SCOPE.json','preflight')for x in p.iterdir()):raise ValueError('refusing existing evidence overwrite')
    for x in sorted((work/'udp-trials-v1').rglob('*')):
        if not x.is_file()or x.suffix=='.test':continue
        y=p/'actual'/x.relative_to(work/'udp-trials-v1');y.parent.mkdir(parents=True,exist_ok=True)
        if x.name in ('stdout','stderr')or x.suffix=='.jsonl':y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
        else:shutil.copyfile(x,y)
    for v,b in read(p/'actual/binaries.json').items():
        if sha(Path(b['path']).read_bytes())!=b['sha256']:raise ValueError('actual postrun binary changed '+v)
    d=derive(packet=p)
    if historical_errors():raise ValueError('historical/native preservation failed')
    write(p/'SUMMARY.json',d['summary']);write(p/'observations.json',d['observations']);write(p/'native-reuse.json',prior.native_reuse_record())
    state=read(ROOT/prior.PACKET/'WORKLIST.json');state.update(current_stage='62_WI4_actual_UDP_pending_response_and_synthetic_state_routes',completed_in_this_scope=['actual_upstream_general_race_24_fixed_DEV_slots_no_production_source_patch','native_pending_timeouts_and_one_response_separate_from12_synthetic_deadline_observations','preflight_process_audit_permission_failure_preserved_then_separate_owned_run'],next_task='WI4_WORK_GOSSIP_POOL_other12_actual_entry_probes_and_all_original_contract_debts',last_verified_date='2026-10-19')
    state['remaining_all'][0]['task']=state['remaining_all'][0]['task'].replace('WI4 나머지16스케줄','WI4 WORK/GOSSIP/POOL 나머지12스케줄 + SUB/UDP 진입·시간·정답 부채')
    state['AI_DEV_draft_progress']['WI-4']='21_conditional_AI_DEV_drafts;5SUB_and4UDP_limited_actual_probes;12_unprobed;full_driver_acceptance0_independent_gold0'
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI4 actual UDP DEV paths; not independent acceptance\n\n'
        'Twenty-four preregistered slots (general/race x3 x4) use the unchanged upstream0bf2f4d1 p2p/discover package and declared Go1.23.12. Only a test file is added. A minimal owned UDPv4 object runs the actual pending, handleReply and loop methods. No ListenV4, table, socket, node or external network is started. It is not an installed host/public listener proof.\n'
        'UDP-near uses real pending registration and unmatched-reply synchronization, then observes two real native500ms timeout results. It does not expose or count exactly one resetTimeout invocation. UDP-response-timeout explicitly selects the response arm: one handleReply event invokes only the first of two same-match callbacks, delivers its nil result and leaves the second pending at the frozen observation. The timeout alternative is not claimed as separately verified.\n'
        'The two far cases register a gate and two actual pending matchers. The gate callback, executing on the source loop, injects synthetic deadlines before completing/removing itself. This exposes the intended two-element state to the real resetTimeout closure. It observes the far head errClockWarp and the unprocessed tail over2000ms, before cleanup. Synthetic far deadlines are not reachable merely from the normal monotonic Now.Add registration. The Stop request/timer internals are source facts, not directly observed timer state. No wall-clock or production source is changed.\n'
        'All planned slots, process/output/trace hashes and cleanup receipts are retained. Credential environment is not inherited; build resolution is offline. An initial sandbox ps audit permission failure happened after local restore and before target builds/trials. Its original output and missing subcommand-receipt limitation are preserved; the same plan runs once in a separate owned workspace with process-audit permission. No target slot is retried or omitted.\n'
        'Repeated exposed DEV probes are not new independent families, gold, frozen arms/seeds, product-violation findings or quality scores. Full original driver acceptance0; formal adapter refuses every row as unknown_unverified_trial. Independent0/FINAL0/formal0of9/freeze null/execution false. D04 unassigned, with no personal names/contact required.\n'
        'Managed CKS native inputs and configurations remain identical to the prior proof:4158 general and4158 race pass,18 skips each are reused, not rerun. Separate upstream general/race binaries are fresh. WORK/GOSSIP/POOL12 actual probes and all independent/entry/timing contracts, other WI debts and ontology effect remain. No publication/global install/paid model call or operational activation.\n')
    seal(p)


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=read(p/'manifest.json');actual={x.relative_to(p).as_posix():sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('UDP evidence inventory differs')
        for r in m['tools']:
            if sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('bound UDP tool changed '+r['path'])
        errors+=historical_errors(root);d=derive(root,p)
        if read(p/'SUMMARY.json')!=d['summary']or read(p/'observations.json')!=d['observations']:errors.append('derived UDP observations differ')
        if read(p/'native-reuse.json')!=prior.native_reuse_record(root):errors.append('unchanged native proof reuse differs')
        state=read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정'or state['independent_quality_or_ontology_acceptance']is not False:errors.append('technical worklist falsely accepted')
    except (ValueError,KeyError,TypeError,OSError,IndexError)as exc:errors.append(str(exc))
    return errors


def storage_errors(root=ROOT):
    try:
        code=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=root,text=True).strip()
        if not re.fullmatch('[0-9a-f]{40}',code):return ['real signed UDP storage pending']
        names=[PACKET/'manifest.json',*[Path(n)for n in TOOLS],*[Path('docs/spec-driven')/n for n in ('CURRENT-HANDOFF.md','KNOWLEDGE-ROADMAP-TEST-MATRIX.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json')],BASE/'GAPS.json']
        if any(subprocess.check_output(['git','show',code+':'+str(n)],cwd=root)!=(root/n).read_bytes()for n in names):return ['committed UDP tools/evidence differ']
        r=subprocess.run(['git','verify-commit',code],cwd=root,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        return []if r.returncode==0 else ['actual UDP commit signature not verified']
    except (OSError,subprocess.CalledProcessError):return ['real signed UDP storage pending']


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','index','check']);p.add_argument('--workspace',type=Path);p.add_argument('--require-stored',action='store_true');p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args()
    if a.command=='prepare':prepare(a.workspace.resolve(),ROOT/PACKET);update_indices()
    elif a.command=='index':update_indices()
    else:
        errors=validate()+current_indices()
        if a.require_stored:errors+=storage_errors()
        if a.require_formal_gold:errors.append('Actual exposed UDP probes and injected state are not independent approved/frozen gold')
        print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
