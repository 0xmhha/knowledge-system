"""Actual source subscription observations; never independent acceptance."""
import argparse,copy,gzip,hashlib,importlib.util,json,re,shutil,subprocess
from pathlib import Path
import wi3_walk_budget as prior
import wi4_dev_gold as draft
ROOT,BASE,f=prior.ROOT,prior.BASE,prior.f
PACKET=BASE/'followups/wi4-target-subscription-v1'
REVISION='bab348916fdb615c56fa31285ed72a70635a5afe'
KIND='WI4_actual_subscription_DEV_routes_not_formal_execution'
TOOLS=['scripts/'+n for n in ('wi4_target_subscription.py','test_wi4_target_subscription.py','run-wi4-target-subscription.py','run-wi4-subscription-corrective.py','fixtures/wi4-owned-subscription.go.fixture','fixtures/wi4-owned-subscription-two-callers.go.fixture','check-operational-acceptance.py','test_operational_acceptance.py','wi3_walk_budget.py','wi4_dev_gold.py','run-wi4-subscription-receiver.py','fixtures/wi4-owned-subscription-receiver.go.fixture')]
CASES=['SUB-normal','SUB-blocked','SUB-late','SUB-ack','SUB-once']
COMMIT='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'


def write(p,d):p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
def raw(p):return gzip.decompress(p.read_bytes())if p.suffix=='.gz'else p.read_bytes()
def read(p):return json.loads(raw(p))
def sha(b):return hashlib.sha256(b).hexdigest()
def events(p):return [json.loads(l)for l in raw(p).decode().splitlines()if l.strip()]
def frame(s,method,line):return any(re.search(re.escape(method)+r'(?=[.(])',g)and f'/eth/downloader/api.go:{line}'in g and re.search(r'\[chan send(?:\]|,)',g)for g in s.split('\n\n'))


def inspect_slot(q,slot,binary,fixture_sha,corrected,receiver=False):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    for k,v in process.items():
        if r[k]!=v:raise ValueError('nested process receipt differs')
    for k,v in slot.items():
        if r[k]!=v:raise ValueError('planned slot identity differs')
    if r['binary_sha256']!=binary['sha256']or r['source_test_addition_sha256']!=fixture_sha:raise ValueError('actual binary/test addition differs')
    if r['argv']!=[binary['path'],'-test.v=true','-test.run=^TestWI4OwnedSubscription$','-test.count=1','-test.timeout=100s']:raise ValueError('actual test invocation differs')
    if not r['owned_new_session']or not r['reaped']or r['remaining_live_group_members']!=0 or r['tmp_children_after']:raise ValueError('owned process/temp cleanup not proved')
    for ext in ('stdout','stderr'):
        if sha(raw(q/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('actual raw output differs')
    rows=events(q/'events.jsonl.gz')
    if [e['sequence']for e in rows]!=list(range(1,len(rows)+1))or any(e['case_id']!=slot['case_id']for e in rows)or [e['elapsed_ns']for e in rows]!=sorted(e['elapsed_ns']for e in rows):raise ValueError('trace identity/order differs')
    names=[e['event']for e in rows]
    if len(names)!=len(set(names)):raise ValueError('duplicate trace transition')
    by={e['event']:e for e in rows}
    if by['trace_begin']['native_timer_seconds']!=60 or by['trace_begin']['step_watchdog_ms']!=2000 or by['trace_begin']['source_commit']!=COMMIT:raise ValueError('native timer/original observation contract differs')
    if by['cleanup_end']['joined']is not True or by['cleanup_end']['actual_loop_absent']is not True:raise ValueError('probe cleanup not joined')
    if names.index('cleanup_begin')>=names.index('cleanup_end')or names.index('cleanup_end')>=names.index('trace_end'):raise ValueError('cleanup order differs')
    expected_failure=slot['case_id']=='SUB-blocked'and(not corrected or(not receiver and slot['variant']=='race'))
    valid=r['exit_code']==0 and not r['timeout']and by['trace_end']['test_failed']is False
    result=dict(slot_id=slot['slot_id'],case_id=slot['case_id'],variant=slot['variant'],repetition=slot['repetition'],route=slot['route'],exit_code=r['exit_code'],binary_sha256=r['binary_sha256'],driver_sha256=fixture_sha,trace_sha256=sha(raw(q/'events.jsonl.gz')),valid_technical_observation=valid,original_source_conditions_verified=not(slot['case_id']=='SUB-once'and not corrected)and not(slot['case_id']=='SUB-blocked'and not receiver),full_original_driver_timing_verified=False,independent_oracle=False,product_violation_established=False,quality_score=None)
    if expected_failure:
        if valid or r['exit_code']!=1 or r['timeout']or'observation_frozen'in by:raise ValueError('old observer failure erased or relabelled')
        message=b'precondition/cleanup not proved: actual Unsubscribe registration blocked'if corrected else b'precondition/cleanup not proved: actual eventLoop done broadcast blocked'
        if message not in raw(q/'process/stdout.gz'):raise ValueError('original observer failure cause missing')
        if corrected:
            stack=raw(q/'events.jsonl.precondition-timeout.stack.gz').decode()
            if not frame(stack,'(*DownloaderAPI).eventLoop',125)or not frame(stack,'(*SyncStatusSubscription).Unsubscribe',186):raise ValueError('race observer failure original frames lost')
        result.update(classification='observer_compiler_symbol_failure_observation_not_frozen'if corrected else'observer_coordinate_failure_target_outcome_unproved',draft_outcome_ids=[]);return result
    if not valid:raise ValueError('correct/other planned slot failed; do not mark trial scope complete '+slot['slot_id'])
    obs=by['observation_frozen']
    if obs['route']!=slot['route']or names.index('observation_frozen')>=names.index('cleanup_begin'):raise ValueError('observation/rescue boundary lost')
    cid=slot['case_id'];ids=[]
    if cid in ('SUB-normal','SUB-blocked'):
        if by['actual_constructor_returned']['route']!='real_API_eventLoop'or by['actual_constructor_returned']['status_capacity']!=0:raise ValueError('real API route changed')
        entry=by['actual_done_notification_received']if cid=='SUB-normal'else by['actual_broadcast_block_proved']
        if entry['elapsed_ns']-by['actual_constructor_returned']['elapsed_ns']<59_000_000_000:raise ValueError('native timer path was shortcut')
        if cid=='SUB-normal':
            if not names.index('actual_done_notification_received')<names.index('unsubscribe_invoke')<names.index('unsubscribe_returned')<names.index('observation_frozen'):raise ValueError('normal source route order changed')
            ids=['delivered_ack_return']
        else:
            for label in ('before-observation','after-observation'):
                s=raw(q/('events.jsonl.'+label+'.stack.gz')).decode()
                if not frame(s,'(*DownloaderAPI).eventLoop',125)or not frame(s,'(*SyncStatusSubscription).Unsubscribe',186):raise ValueError('actual paired blocking frames absent')
            if obs['elapsed_ns']-by['two_waits_begin']['elapsed_ns']<2_000_000_000 or obs['observation_ms']!=2000:raise ValueError('blocked observation window shortened')
            if not names.index('observation_frozen')<names.index('cleanup_begin')<names.index('unsubscribe_returned'):raise ValueError('rescue contaminated blocked observation')
            if receiver:
                if not names.index('receiver_started')<names.index('receiver_returned')<names.index('receiver_exited_before_trigger')<names.index('start_event_post_returned')<names.index('actual_broadcast_block_proved'):raise ValueError('unique receiver lifecycle before trigger unproved')
                ids=['two_waits_before_rescue','rescue_then_ack_return']
    else:
        count=2 if corrected else 3 if cid=='SUB-once'else 1
        if by['actual_method_invoked']['route']!='controlled_receiver_actual_Unsubscribe'or by['actual_method_invoked']['concurrent_calls']!=count:raise ValueError('controlled caller input differs')
        if by['registration_received']['registrations']!=1 or by['all_calls_returned']!={'event':'all_calls_returned','case_id':cid,'sequence':by['all_calls_returned']['sequence'],'elapsed_ns':by['all_calls_returned']['elapsed_ns'],'calls':count,'registrations':1}:raise ValueError('actual controlled call/registration denominator differs')
        if obs['elapsed_ns']-by['ack_withheld_begin']['elapsed_ns']<2_000_000_000 or not names.index('observation_frozen')<names.index('controlled_ack_released')<names.index('all_calls_returned'):raise ValueError('ack/observation boundary lost')
        if cid=='SUB-late':
            if not names.index('registration_received')<names.index('late_notification_consumed')<names.index('ack_withheld_begin'):raise ValueError('late drain not proved')
            ids=['drain_then_ack_return']
        elif cid=='SUB-ack':ids=['wait_until_ack']
        elif corrected:ids=['one_request_two_returns']
    result.update(classification='actual_exposed_DEV_source_observation'if result['original_source_conditions_verified']else'additional_no_receiver_lifecycle_state_variant'if cid=='SUB-blocked'else'additional_three_caller_stress_not_original_input',draft_outcome_ids=ids)
    return result


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;results=[];baseline=None
    for label,n,fixture in [('initial',30,TOOLS[4]),('corrective',12,TOOLS[5]),('receiver',6,TOOLS[-1])]:
        q=p/label;receipt=read(q/'receipt.json');binding=read(q/'source-binding.json');profile=read(q/'profile.json');plan=read(q/'planned.json');attempts=read(q/'attempts.json');binaries=read(q/'binaries.json')
        expected=[dict(slot_id=f'{v}-r{rep}-{c}',variant=v,repetition=rep,case_id=c,route='real_API_eventLoop'if c in('SUB-normal','SUB-blocked')else'controlled_receiver_actual_Unsubscribe')for v in['general','race']for rep in[1,2,3]for c in CASES]if label=='initial'else read(p/('FULL-RECEIVER-SCOPE.json'if label=='receiver'else'CORRECTIVE-SCOPE.json'))['additional_slots']
        if plan!=expected or len(plan)!=n or [r['slot_id']for r in attempts]!=[r['slot_id']for r in plan]:raise ValueError('fixed plan/attempt slots changed')
        if receipt['planned']!=n or receipt['attempted']!=n or receipt['retries']!=0 or receipt['tracked_source_preserved']is not True or receipt['only_test_addition']is not True or receipt['production_patch']is not False:raise ValueError('attempt/source scope differs')
        if any(receipt[k]!=0 for k in ('independent_approved','new_independent_families','formal_completed','paid_or_external_LLM_calls','operational_nodes_started'))or receipt['quality_score']is not None:raise ValueError('technical trial falsely promoted')
        if binding['source_commit']!=COMMIT or binding['production_patch']is not False or binding['test_addition']!=dict(path='eth/downloader/wi4_owned_test.go',sha256=sha((root/fixture).read_bytes())):raise ValueError('source/test-only binding differs')
        if baseline is None:baseline=binding['tracked']
        if binding['tracked']!=baseline:raise ValueError('corrective tracked target source differs')
        if profile!=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'):raise ValueError('actual project toolchain/profile differs')
        if read(q/'binary-postrun-check.json')!=dict(observed_binary_sha256={v:b['sha256']for v,b in binaries.items()},all_match_build_receipts=True):raise ValueError('postrun target binary identity differs')
        for variant,binary in binaries.items():
            br=read(q/('build-'+variant)/'receipt.json');argv=br['argv'];expected_argv=[argv[0],'test']+(['-race']if variant=='race'else[])+['-c','-o',binary['path'],'./eth/downloader']
            if argv!=expected_argv or br['exit_code']or br['timeout']or not br['reaped']or br['remaining_live_group_members']:raise ValueError('actual target binary build failed/narrowed')
            for ext in ('stdout','stderr'):
                if sha(raw(q/('build-'+variant)/(ext+'.gz')))!=br[ext+'_sha256']:raise ValueError('binary build raw log differs')
        for slot,attempt in zip(plan,attempts):
            directory=q/'slots'/slot['slot_id']
            if read(directory/'receipt.json')!=attempt:raise ValueError('attempt aggregate differs')
            observed=inspect_slot(directory,slot,binaries[slot['variant']],binding['test_addition']['sha256'],label!='initial',label=='receiver');observed.update(environment_sha256=sha((q/'profile.json').read_bytes()),environment_hash_scope='recorded_Go_build_profile_only_not_full_or_frozen_environment');results.append(observed)
    proof=read(root/draft.SOURCE/'git-proof.json')
    for name,row in proof['files'].items():
        if baseline[name]['sha256']!=row['sha256']:raise ValueError('original7 source hashes differ')
    gold=read(root/draft.PACKET/'private/AI-gold.json');by={r['case_id']:r for r in gold}
    for row in results:
        for unit in row['draft_outcome_ids']:
            if not any(u['id']==unit for u in by[row['case_id']]['universe']):raise ValueError('outcome invented outside original vocabulary')
        # Repetition is not an invented scheduler seed or frozen comparison arm.
        row['formal_adapter_result']=draft.match_observation(dict(case_id=row['case_id'],family_proposal='SCHED-001',source_commit=COMMIT,source_project='go-stablenet',source_mode='committed',binary_sha256=row['binary_sha256'],driver_sha256=row['driver_sha256'],environment_sha256=row['environment_sha256'],entry_reached=row['valid_technical_observation'],preconditions_verified=row['valid_technical_observation']and row['original_source_conditions_verified'],trace_sha256=row['trace_sha256']),by[row['case_id']])
        if row['formal_adapter_result']['status']!='unknown_unverified_trial':raise ValueError('unfrozen repeat falsely became formal trial')
    original=[r for r in results if r['valid_technical_observation']and r['original_source_conditions_verified']]
    if len(original)!=30 or len([r for r in results if not r['valid_technical_observation']])!=9:raise ValueError('retained failure/exact original denominators differ')
    summary=dict(kind=KIND,planned_attempts=48,actual_attempts=len(results),initial_observer_failures_preserved=6,compiler_symbol_observer_failures_preserved=3,total_observer_failures_preserved=9,extra_three_caller_stress=6,extra_no_receiver_lifecycle_variants=3,original_source_condition_observations=len(original),source_condition_real_API_routes=12,source_condition_controlled_receiver_routes=18,original_subscription_cases=5,full_original_driver_timing_acceptances=0,readiness_timing_scope='native60s setup up to75s separate from original2000ms post-readiness observation; no full fresh-start2s claim',original_total_schedules=21,other16_schedules_pending=True,tracked_target_files=len(baseline),target_production_patch=False,source_commit=COMMIT,target_go='go1.23.12',native_timer_seconds=60,step_observation_ms=2000,independent_families=0,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0)
    return dict(summary=summary,observations=results)


def historical_errors(root=ROOT):
    errors=prior.historical_errors(root);p=root/prior.PACKET;m=read(p/'manifest.json')
    if(p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(prior.PACKET/'manifest.json')],cwd=root):errors.append('previous technical packet manifest changed')
    for n,h in m['artifacts'].items():
        if sha((p/n).read_bytes())!=h:errors.append('previous technical evidence changed '+n)
    for r in m['tools']:
        if sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('previous tool changed at historical revision')
    errors.extend(prior.native_errors(root,p))
    return errors


def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=sha((root/PACKET/'manifest.json').read_bytes()),status='WI4_actual_source_DEV_routes_not_independent_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))

def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[read(root/n)for n in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        before=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('wi4_target_subscription')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current WI4 reference/state differs')
        if d.get('reviewer_identity_policy')!=before.get('reviewer_identity_policy'):errors.append('privacy policy changed')
        for k,v in before.items():
            if k=='current_readiness_preparation'or not isinstance(v,dict)or'manifest'not in v:continue
            expected=dict(v)
            if k=='wi3_walk_budget':expected['historical_source_revision']=REVISION
            if d.get(k)!=expected:errors.append('historical preparation changed '+k)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical DEV observations cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in(docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def native_reuse_record(root=ROOT):
    q=root/prior.PACKET;binding=read(q/'source-binding.json');summary=read(q/'SUMMARY.json');final=read(q.parent/'wi3-walk-budget-v1-finalization.json')
    return dict(origin_manifest=str(prior.PACKET/'manifest.json'),origin_manifest_sha256=sha((q/'manifest.json').read_bytes()),source_binding=str(prior.PACKET/'source-binding.json'),source_binding_sha256=sha((q/'source-binding.json').read_bytes()),origin_committed_source=final['code_revision'],native_inputs=len(binding['source_sha256']),approved_inputs=len(binding['approved_input_sha256']),lint_configurations=2,current_inputs_match=True,new_native_CKS_run=False,reused_general_pass=summary['fresh_native_test']['pass'],reused_race_pass=summary['fresh_native_race']['pass'],reused_skip_each=summary['fresh_native_test']['skip'],actual_target_project_compilers_separate=True)


def seal(p):write(p/'manifest.json',dict(kind=KIND,previous_source_revision=REVISION,tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={x.relative_to(p).as_posix():sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))

def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=read(p/'manifest.json');actual={x.relative_to(p).as_posix():sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('WI4 evidence inventory differs')
        for r in m['tools']:
            if sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('WI4 bound tool changed '+r['path'])
        errors.extend(historical_errors(root));d=derive(root,p)
        if read(p/'native-reuse.json')!=native_reuse_record(root):errors.append('unchanged native proof reuse misrepresented')
        if read(p/'SUMMARY.json')!=d['summary']or read(p/'observations.json')!=d['observations']:errors.append('derived WI4 observations differ')
        state=read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정'or state['independent_quality_or_ontology_acceptance']is not False:errors.append('WI4 worklist falsely accepted')
    except(ValueError,KeyError,TypeError,OSError,IndexError)as exc:errors.append(str(exc))
    return errors


def prepare(work,p):
    if any(x.name not in ('SCOPE.json','ORIGINAL-TWO-CALLER-SCOPE.json','CORRECTIVE-SCOPE.json','FULL-RECEIVER-SCOPE.json')for x in p.iterdir()):raise ValueError('refusing existing evidence overwrite')
    for label,directory in [('initial','subscription-trials-v1'),('corrective','subscription-corrective-v1'),('receiver','subscription-receiver-v1')]:
        for x in sorted((work/directory).rglob('*')):
            if not x.is_file()or x.suffix=='.test':continue
            y=p/label/x.relative_to(work/directory);y.parent.mkdir(parents=True,exist_ok=True)
            if x.name in('stdout','stderr')or x.suffix in('.jsonl','.stack'):y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
            else:shutil.copyfile(x,y)
    for label,directory in [('initial','subscription-trials-v1'),('corrective','subscription-corrective-v1'),('receiver','subscription-receiver-v1')]:
        binaries=read(work/directory/'binaries.json');verified={}
        for variant,binary in binaries.items():
            actual=sha(Path(binary['path']).read_bytes())
            if actual!=binary['sha256']:raise ValueError('postrun target binary changed')
            verified[variant]=actual
        write(p/label/'binary-postrun-check.json',dict(observed_binary_sha256=verified,all_match_build_receipts=True))
    preflight_names=['preflight-plan.json','preflight-result.json','preflight-attempt1.json','step0.stdout','step0.stderr','step1.stdout','step1.stderr','recovery0.stdout','recovery0.stderr','recovery1.stdout','recovery1.stderr','recovery-receipts.json','dependency-plan.json','dependency-result.json','dependencies.stdout','dependencies.stderr','compile-plan.json','compile-result.json','compile.stdout','compile.stderr']
    for name in preflight_names:
        y=p/'preflight'/name;y.parent.mkdir(exist_ok=True)
        if name.endswith(('.stdout','.stderr')):y.with_name(name+'.gz').write_bytes(gzip.compress((work/name).read_bytes(),mtime=0))
        else:shutil.copyfile(work/name,y)
    d=derive(packet=p);write(p/'SUMMARY.json',d['summary']);write(p/'observations.json',d['observations'])
    if historical_errors():raise ValueError('historical/native reuse proof failed')
    write(p/'native-reuse.json',native_reuse_record())
    state=read(ROOT/prior.PACKET/'WORKLIST.json');state.update(current_stage='61_WI4_actual_subscription_source_routes_and_corrective_attempts',completed_in_this_scope=['exact_upstream_source_project_toolchain_and_test_only_probe_binaries','all48_attempts_preserved_9_observer_failures_6_stress_3_state_variants_30_original_source_condition_observations_not_full_driver_acceptance','real_API_timer_waits_observed_before_rescue_controlled_receiver_routes_separated'],next_task='WI4_remaining16_schedules_real_paths_then_other_WI_acceptance_debts',last_verified_date='2026-10-19',storage_finalization_record='actual_signed_Git_commit_checked_by_require_stored_no_native_source_change')
    state['remaining_all'][0]['task']='WI4 나머지16스케줄 실제 경로/스케줄/반복·fresh-start/준비/2초 드라이버 계약·전체API와 제어 수신자 범위·실행 환경/독립 oracle·가족·반복 충분성; WI-V1/V2 실제API/독립티켓/수용후보; WI6 검색쌍/관련성·잡음/warm; WI-SP overlay/ACL/출처; WI-C 필수항목/전달/설치역할; WI1 남은53발생/3블록·타입/메모리·독립의미, WI2 독립의미/build profile/지원/설치역할, WI3 독립의미/새가족·기본예산/시작SLA/site거부; 새독립DEV/FINAL·완전gold/matching/충분성·F01~08'
    write(p/'WORKLIST.json',state)
    write(p/'acceptance-review.json',[dict(id='S'+str(i).zfill(2),status='technical_evidence_recorded_not_independent_acceptance'if i!=7 else'technical_evidence_ready_signed_storage_checked_separately',evidence=['SCOPE.json','CORRECTIVE-SCOPE.json','FULL-RECEIVER-SCOPE.json','SUMMARY.json','observations.json','native-reuse.json'])for i in range(1,9)])
    (p/'REPORT.txt').write_text('WI4 actual upstream subscription DEV routes; no independent acceptance\n\n'
        'The unchanged upstream checkout0bf2f4d1 uses its declared installed Go1.23.12 darwin/arm64 CGO1. Only a test file is added; original tracked sources remain identical. The real constructor/eventLoop/Unsubscribe route preserves the native60second timer and uses real upstream local in-memory chain/downloader helpers without starting an operational node. Native readiness setup up to75seconds is distinct from the unchanged2000ms observation/step watchdog.\n'
        'All three executed plans are retained. All48 actual slots are recorded. Six original observer attempts fail because the probe used lines126/188 instead of actual send statements125/186; their target outcome remains unproved and they are never relabelled as passes. Six3caller stress observations do not match the original2caller input. Three race observer attempts also fail because compiler closure suffixes differ; their timeout stacks are preserved without turning failed observations into passes. Three general no-reader-from-start observations are state variants, not proof that the original sole receiver terminated. The final receiver plan starts and joins the sole reader before the trigger, and recognizes the actual method scope at the same source send line. Final6receiver plus6two-caller slots provide30 original-source-condition observations under the separately declared readiness setup: general/race each3 repetitions over5 exposed DEV schedules. Repetitions do not add independent families. Full original driver timing acceptance stays0: native60second readiness is outside the post-readiness2second observation and is not a claim that a fresh API can reach this path within2seconds. The full timing/entry recipe remains review debt.\n'
        'Two routes enter the real API: normal notification then return, and source-stack-proved concurrent broadcast/registration waits observed2seconds before rescue, then return and loop cleanup. Three routes execute the actual Unsubscribe method with a controlled receiver: late notification drain, withheld acknowledgement, and once-only registration for2calls. They are not claimed as complete eventLoop/public host paths. There is no source arm producing an extra notification between uninstall receive and its acknowledgement close.\n'
        'Original21 schedules and150 AI vocabulary units stay unchanged. Matched outcome IDs are exposed AI vocabulary observations only, not product violations or independent gold. Real binary/driver/trace hashes are retained; environment digest covers recorded Go build profile only, not inherited/full/frozen environment. No scheduler seed/frozen baseline-fix arm is invented. The existing formal observation adapter still refuses these rows as unverified.\n'
        'All owned process groups are terminal, tempdirs empty, and cleanup observed separately from the frozen target observation. General/race builds and their raw output, every trace/stack and failure are preserved. CKS native source1738/approved5/config2 still matches the last native proof; prior4158/general and4158/race results are reused, not called fresh. The actual upstream target binaries are separate builds.\n'
        'Independent0/FINAL0/formal0of9/freeze null/execution false; D04 remains unassigned without real names/contact. No global install, production activation, publication, paid/external LLM or quality score. Remaining16 WI4 schedules, independent family/oracle/target environment and statistical sufficiency, other WI debts and ontology effect evaluation remain in WORKLIST.json. Signed storage is verified against actual Git with --require-stored; no new native binding is needed because managed Go inputs did not change.\n')
    seal(p)


def update_indices():
    p=ROOT/PACKET;seal(p);ref=reference();state=read(p/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);base=ROOT/'docs/spec-driven'
    proposal=base/'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json';d=read(proposal);old=dict(d['wi3_walk_budget'],historical_source_revision=REVISION);d.update(wi4_target_subscription=ref,wi3_walk_budget=old,current_readiness_preparation=current,current_execution_state=state);write(proposal,d)
    link=dict(path=str(proposal.relative_to(ROOT)),sha256=sha(proposal.read_bytes()),status='D04_review_pending_WI4_actual_DEV_source_routes_not_independent_acceptance')
    for name in('OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','operational-preparation/source-review-supplement/GAPS.json'):
        q=base/name;d=read(q);old=dict(d['wi3_walk_budget'],historical_source_revision=REVISION);d.update(wi4_target_subscription=ref,wi3_walk_budget=old,current_readiness_preparation=current,current_execution_state=state,adoption_acceptance_decision_proposal=link)
        if q.name=='OPERATIONAL-ACCEPTANCE-PROGRESS.json':d['phases'][0]['evidence'].update(wi4_target_subscription=ref,wi3_walk_budget=old,current_readiness_preparation=current,adoption_acceptance_decision_proposal=link)
        write(q,d)


def storage_errors(root=ROOT):
    errors=[]
    try:
        code=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=root,text=True).strip()
        if not re.fullmatch('[0-9a-f]{40}',code):return ['real signed WI4 storage pending']
        related=[PACKET/'manifest.json',*[Path(n)for n in TOOLS],*[Path('docs/spec-driven')/n for n in ['CURRENT-HANDOFF.md','KNOWLEDGE-ROADMAP-TEST-MATRIX.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json']],BASE/'GAPS.json']
        for name in related:
            if subprocess.check_output(['git','show',code+':'+str(name)],cwd=root)!=(root/name).read_bytes():errors.append('committed tool/evidence differs '+str(name))
        r=subprocess.run(['git','verify-commit',code],cwd=root,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        if r.returncode:errors.append('actual code commit signature not verified')
    except(OSError,subprocess.CalledProcessError):errors.append('real signed WI4 storage pending')
    return errors


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('command',choices=['prepare','check','index']);parser.add_argument('--workspace',type=Path);parser.add_argument('--require-stored',action='store_true');parser.add_argument('--require-formal-gold',action='store_true');args=parser.parse_args()
    if args.command=='prepare':prepare(args.workspace.resolve(),ROOT/PACKET);update_indices()
    elif args.command=='index':update_indices()
    else:
        errors=validate()+current_indices()
        if args.require_stored:errors+=storage_errors()
        if args.require_formal_gold:errors.append('Exposed actual source DEV observations are not independent approved/frozen gold')
        print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
