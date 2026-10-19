"""Source-bound native60s progress over owned JSON-RPC; not independent acceptance."""
import argparse,gzip,json,re,shutil,subprocess,tempfile,os
from pathlib import Path
import ticket_actual_rpc as prior
import ticket_method_schedules as previous
from source_packet import oid,verify
ROOT,BASE=prior.ROOT,prior.BASE
PACKET=BASE/'followups/ticket-rpc-overlap-v1'
REVISION='4b7b9decb76e234de8d2bae12575355f5f8415c1'
KIND='WIV1_V2_native_RPC_overlap_atomic_counter_DEV_not_independent_acceptance'
COMMIT=prior.COMMIT
ROLES=['ORIGINAL','CAND-A','CAND-B']
API='eth/downloader/api.go';BRIDGE='eth/downloader/ks_owned_rpc_overlap.go';MAIN='cmd/ks-owned-rpc-overlap/main.go'
COVER='github.com/ethereum/go-ethereum/eth/downloader,github.com/ethereum/go-ethereum/cmd/ks-owned-rpc-overlap'
TOOLS=['scripts/'+n for n in ('ticket_rpc_overlap.py','test_ticket_rpc_overlap.py','run-ticket-rpc-overlap.py','fixtures/ticket-rpc-overlap.go.fixture','fixtures/ticket-rpc-overlap-main.go.fixture','run-wi4-target-subscription.py','ticket_actual_rpc.py','ticket_method_schedules.py','ticket_native_progress.py','ticket_dev_gold.py','source_packet.py','check-operational-acceptance.py','test_operational_acceptance.py')]
write,read,raw,sha=prior.write,prior.read,prior.raw,prior.sha
native_record=prior.native_record
flat_map,tree_oid=prior.flat_map,prior.tree_oid
GO='/Users/kevin/.gvm/gos/go1.23.12/bin/go'
SCOPE_SHA='03568a6b80b37852a42f577618c1d8fa178e19ce8641ccbdb38bf2ea44b1b7e0'

def historical_errors(root=ROOT):
    errors=previous.historical_errors(root);p=root/previous.PACKET;m=read(p/'manifest.json')
    if(p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(previous.PACKET/'manifest.json')],cwd=root):errors.append('stage66 manifest changed')
    for n,h in m['artifacts'].items():
        if sha((p/n).read_bytes())!=h:errors.append('stage66 evidence changed '+n)
    for r in m['tools']:
        if sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('stage66 tool at signed revision differs')
    return errors



def seal(p):write(p/'manifest.json',dict(kind=KIND,previous_source_revision=REVISION,tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={x.relative_to(p).as_posix():sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))



def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=sha((root/PACKET/'manifest.json').read_bytes()),status=KIND,report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))



def update_indices():
    p=ROOT/PACKET;seal(p);ref=reference();state=read(p/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);base=ROOT/'docs/spec-driven'
    proposal=base/'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json';d=read(proposal);d.update(ticket_rpc_overlap=ref,ticket_method_schedules=dict(d['ticket_method_schedules'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state);write(proposal,d)
    link=dict(path=str(proposal.relative_to(ROOT)),sha256=sha(proposal.read_bytes()),status='D04_review_pending_WIV1_V2_RPC_overlap_DEV_not_independent_acceptance')
    for name in ('OPERATIONAL-ACCEPTANCE-PROTOCOL.json','OPERATIONAL-ACCEPTANCE-PROGRESS.json','operational-preparation/source-review-supplement/GAPS.json'):
        q=base/name;d=read(q);d.update(ticket_rpc_overlap=ref,ticket_method_schedules=dict(d['ticket_method_schedules'],historical_source_revision=REVISION),current_readiness_preparation=current,current_execution_state=state,adoption_acceptance_decision_proposal=link)
        if q.name=='OPERATIONAL-ACCEPTANCE-PROGRESS.json':d['phases'][0]['evidence'].update(ticket_rpc_overlap=ref,ticket_method_schedules=d['ticket_method_schedules'],current_readiness_preparation=current,adoption_acceptance_decision_proposal=link)
        write(q,d)



def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[read(root/n)for n in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('ticket_rpc_overlap')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current ticket_RPC reference/state differs')
        if d.get('reviewer_identity_policy')!=old.get('reviewer_identity_policy'):errors.append('privacy policy changed')
        for k,v in old.items():
            if k=='current_readiness_preparation'or not isinstance(v,dict)or'manifest'not in v:continue
            expected=dict(v)
            if k=='ticket_method_schedules':expected['historical_source_revision']=REVISION
            if d.get(k)!=expected:errors.append('historical preparation changed '+k)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical ticket_RPC probes cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('current proposal link differs')
    return errors





def bound_sources(root,p):
    q=p/'actual';fixture=(root/TOOLS[3]).read_bytes();main=(root/TOOLS[4]).read_bytes();expected=read(root/BASE/'followups/wi4-target-subscription-v1/initial/source-binding.json')['tracked'];candidates={c['id']:c for c in read(root/prior.draft.PACKET/'private/candidates.json')}
    proof=verify(root/BASE/'schedule-candidates-v1',COMMIT);commit=(root/BASE/'schedule-candidates-v1'/proof['objects']['commit:'+COMMIT]).read_bytes();base_tree=commit.splitlines()[0].decode().removeprefix('tree ')
    built={};trees={};post=read(q/'binary-postrun-check.json');baseline_flat=None
    if set(post)!=set(ROLES):raise ValueError('fresh three-role binary population differs')
    for role in ROLES:
        d=q/'candidates'/role;c=candidates[role];overlay=(root/prior.draft.PACKET/'private/overlays'/(role+'.go.txt')).read_bytes();applied=dict(expected);applied[API]=dict(mode=expected[API]['mode'],sha256=sha(overlay))
        binding=dict(candidate_id=role,base_commit=COMMIT,base_tracked=expected,applied_tracked=applied,api_sha256=sha(overlay),method_sha256=c['method_sha256'],fixture_sha256=sha(fixture),main_fixture_sha256=sha(main),additions=[BRIDGE,MAIN],production_patch=False,original_test_init_preserved_in_source_but_not_executed=True)
        if read(d/'source-binding.json')!=binding or sha(overlay)!=c['full_file_overlay_sha256']:raise ValueError('owned bridge/main/exact API binding differs')
        flat=read(d/'base-flat-tree.json');baseline=flat_map(flat)
        if len(flat)!=len(expected)or set(baseline)!=set(expected)or tree_oid(flat)!=base_tree:raise ValueError('full baseline Git tree differs')
        if baseline_flat is None:baseline_flat=flat
        if flat!=baseline_flat:raise ValueError('candidate base trees differ')
        project=dict(baseline);project[API]=dict(mode='100644',kind='blob',oid=oid('blob',overlay));build=dict(project)
        for name,data in [(BRIDGE,fixture),(MAIN,main)]:build[name]=dict(mode='100644',kind='blob',oid=oid('blob',data))
        t=read(d/'tree-binding.json')
        if t['base_commit']!=COMMIT or t['applied_project_commit']is not None or t['source_mode']!='owned_working_tree_real_Git_trees_plus_two_program_additions_compiler_atomic_coverage'or t['objects']!={n:oid('blob',v)for n,v in [(API,overlay),(BRIDGE,fixture),(MAIN,main)]}or flat_map(t['project_flat_tree'])!=project or flat_map(t['build_flat_tree'])!=build or tree_oid(t['project_flat_tree'])!=t['applied_project_tree']or tree_oid(t['build_flat_tree'])!=t['build_input_tree']:raise ValueError('actual project/program build tree differs')
        if read(d/'profile.json')!=dict(GOVERSION='go1.23.12',GOOS='darwin',GOARCH='arm64',CGO_ENABLED='1'):raise ValueError('owned program toolchain differs')
        binaries=read(d/'binaries.json')
        if set(binaries)!={'general','race'}or post[role]!={v:b['sha256']for v,b in binaries.items()}:raise ValueError('postrun program binaries differ')
        for v,b in binaries.items():
            r=read(d/('build-'+v)/'receipt.json');args=[GO,'build']+(['-race']if v=='race'else[])+['-cover','-covermode=atomic','-coverpkg='+COVER,'-o',b['path'],'./cmd/ks-owned-rpc-overlap']
            if r['argv']!=args or r['exit_code']or r['timeout']or not r['reaped']or r['remaining_live_group_members']:raise ValueError('atomic program build flags/result differ')
            for ext in ('stdout','stderr'):
                if sha(raw(d/('build-'+v)/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('build raw output differs')
        built[role]=binaries;trees[role]=t
    if len({t['applied_project_tree']for t in trees.values()})!=3:raise ValueError('fresh variant count differs')
    return built,trees


def counter_profile(root,p,q,label,env):
    source=q/('coverage-'+label);converted=q/('coverage-'+label+'.txt');receipt=read(q/('convert-'+label)/'receipt.json');original=q/'receipt.json';cwd=read(original)['cwd'];original_dir=str(Path(read(original)['argv'][0]).parent.parent.parent/'slots'/q.name)
    # Retained absolute paths name the actual owned run, not this replay folder.
    argv=[GO,'tool','covdata','textfmt','-i='+original_dir+'/coverage-'+label,'-o='+original_dir+'/coverage-'+label+'.txt']
    if receipt['argv']!=argv or receipt['cwd']!=cwd or receipt['exit_code']or receipt['timeout']or not receipt['reaped']or receipt['remaining_live_group_members']:raise ValueError('actual covdata conversion receipt differs')
    for ext in ('stdout','stderr'):
        if sha(raw(q/('convert-'+label)/(ext+'.gz')))!=receipt[ext+'_sha256']:raise ValueError('conversion raw output differs')
    files=list(source.iterdir())
    if sum(x.name.startswith('covmeta.')for x in files)!=1 or sum(x.name.startswith('covcounters.')for x in files)!=1 or len(files)!=2:raise ValueError('pre-rescue snapshots merged/contaminated or missing')
    tool=read(p/'coverage-tool.json')
    if tool!={str(n):sha(n.read_bytes())for n in [Path(GO),Path(GO).parent.parent/'pkg/tool/darwin_arm64/covdata']}:raise ValueError('local coverage SDK changed')
    e={k:os.environ[k]for k in ('HOME','USER','TMPDIR','LANG','LC_ALL')if k in os.environ};e.update(env['task_settings']);e['PATH']=env['PATH']
    with tempfile.TemporaryDirectory(dir='/private/tmp')as tmp:
        output=Path(tmp)/'profile.txt';r=subprocess.run([GO,'tool','covdata','textfmt','-i='+str(source.resolve()),'-o='+str(output)],cwd='/private/tmp',env=e,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
        if r.returncode or output.read_bytes()!=converted.read_bytes():raise ValueError('retained text differs from independent re-conversion of raw atomic counters')
    lines=converted.read_text().splitlines()
    if not lines or lines[0]!='mode: atomic':raise ValueError('counter mode differs')
    blocks={}
    for line in lines[1:]:
        m=re.fullmatch(r'(.+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)',line)
        if not m:raise ValueError('malformed coverage span')
        name=m[1];coords=tuple(int(m[i])for i in range(2,7));count=int(m[7])
        if name.endswith('/eth/downloader/api.go'):
            if coords in blocks:raise ValueError('duplicate API coverage span')
            blocks[coords]=count
    if not blocks:raise ValueError('actual API counter population absent')
    return blocks


def span_count(blocks,source,needle):
    lines=source.splitlines();matches=[i+1 for i,l in enumerate(lines)if needle in l]
    if not matches:raise ValueError('source anchor absent '+needle)
    counts=[]
    for line in matches:
        exact=[v for (a,b,c,d,n),v in blocks.items()if a==line]
        if len(exact)!=1:raise ValueError('source anchor has no unique runtime counter '+needle)
        counts+=exact
    return sum(counts)

EVENTS=['trace_begin','actual_RPC_subscription','test_owned_StartEvent','first_native_write_held','second_native_broadcast_blocked','counter_snapshot','actual_server_connection_closed','write_gate_released','observation_frozen','counter_snapshot','cleanup_begin','cleanup_end','trace_end']


def inspect_slot(root,p,q,slot,binary,tree,scope,env):
    r=read(q/'receipt.json');process=read(q/'process/receipt.json')
    if any(r.get(k)!=v for k,v in {**process,**slot}.items())or r['argv']!=[binary['path']]or r['binary_sha256']!=binary['sha256']or r['fixture_sha256']!=scope['fixture_sha256']or r['main_fixture_sha256']!=scope['main_fixture_sha256']:raise ValueError('program receipt/fixture identity differs')
    if r['exit_code']or r['timeout']or r['term_sent']or r['kill_sent']or not r['reaped']or not r['owned_new_session']or r['remaining_live_group_members']or r['tmp_children_after']or r['elapsed_seconds']>=210:raise ValueError('program failed/forced termination/cleanup unproved')
    for ext in ('stdout','stderr'):
        if sha(raw(q/'process'/(ext+'.gz')))!=r[ext+'_sha256']:raise ValueError('program raw output differs')
    events=[json.loads(l)for l in raw(q/'events.jsonl.gz').decode().splitlines()]
    if [e['event']for e in events]!=EVENTS or [e['sequence']for e in events]!=list(range(1,14))or any(e['candidate']!=slot['candidate']for e in events)or [e['elapsed_ns']for e in events]!=sorted(e['elapsed_ns']for e in events):raise ValueError('overlap trace sequence/identity differs')
    def exact(index,expected):
        actual={k:v for k,v in events[index].items()if k not in ('event','sequence','candidate','elapsed_ns')}
        if actual!=expected:raise ValueError('overlap event facts differ '+events[index]['event'])
    exact(0,dict(base_commit=COMMIT,native_timer_seconds=60,standalone_program=True,original_package_tests_executed=False,compiler_atomic_coverage=True,custom_production_source_hooks=False,independent_oracle=False,external_listener=False))
    subscription=events[1]['subscription_id']
    if not re.fullmatch('0x[0-9a-f]+',subscription):raise ValueError('actual RPC subscription id invalid')
    exact(1,dict(subscription_id=subscription,caller_RPC_requests=1,route='Server.ServeCodec_NewCodec_net.Pipe',controlled_height=1,network_sync=False));exact(2,{})
    exact(3,dict(wire_notification_delivered=False,actual_notifier=True,owned_transport_gate=True));exact(4,dict(eventLoop_send_site=119,sole_RPC_consumer_in_Notify=True,unbuffered_status_channel=True,native_timer_fires=2))
    if events[3]['elapsed_ns']<60_000_000_000 or events[4]['elapsed_ns']<120_000_000_000:raise ValueError('native timers shortened/forced')
    for k,label in [(5,'before-close'),(9,'before-rescue')]:exact(k,dict(label=label,before_rescue=True,counter_mode='atomic',global_transactional_snapshot=False))
    exact(6,dict(write_gate_still_held=True,RPC_unsubscribe_response=False));exact(7,{})
    before=raw(q/'events.jsonl.native-broadcast-blocked.stack.gz').decode()
    if not prior.stack_frame(before,'(*DownloaderAPI).eventLoop','/eth/downloader/api.go:119','[chan send')or not prior.stack_frame(before,'(*DownloaderAPI).Syncing.func1','(*Notifier).Notify','(*overlapConn).Write'):raise ValueError('actual same RPC callback and native broadcast overlap precondition absent')
    held=raw(q/'events.jsonl.server-stopped-gate-held.stack.gz').decode()
    if not prior.stack_frame(held,'(*DownloaderAPI).eventLoop','/eth/downloader/api.go:119','[chan send')or not prior.stack_frame(held,'(*DownloaderAPI).Syncing.func1','(*overlapConn).Write'):raise ValueError('connection-close/gate ordering unsupported')
    a=counter_profile(root,p,q,'before-close',env);b=counter_profile(root,p,q,'before-rescue',env)
    if set(a)!=set(b)or any(b[k]<a[k]for k in a):raise ValueError('API counter populations decreased/differ')
    source=(root/prior.draft.PACKET/'private/overlays'/(slot['candidate']+'.go.txt')).read_text()
    def counts(profile):return dict(notify=span_count(profile,source,'case status := <-statuses:'),exit=span_count(profile,source,'case <-rpcSub.Err():')+span_count(profile,source,'case <-notifier.Closed():'),uninstall=span_count(profile,source,'case u := <-api.uninstallSyncSubscription:'),method=span_count(profile,source,'func (s *SyncStatusSubscription) Unsubscribe()'),drain=span_count(profile,source,'case <-s.c:'),ack=span_count(profile,source,'case <-req.uninstalled:'))
    pre,post=counts(a),counts(b)
    if pre!=dict(notify=1,exit=0,uninstall=0,method=0,drain=0,ack=0):raise ValueError('pre-close API route counters differ')
    if post['notify']not in (1,2)or post['exit']!=1 or post['method']!=1:raise ValueError('actual RPC callback exit/method path counters absent')
    paired=slot['candidate']=='ORIGINAL'and post['notify']==1
    if paired:
        if post!=dict(notify=1,exit=1,uninstall=0,method=1,drain=0,ack=0):raise ValueError('original paired wait counters differ')
        outcome='actual_RPC_exit_then_paired_native_and_uninstall_sends'
    else:
        if post['uninstall']!=1 or post['ack']!=1 or post['drain']!=(1 if post['notify']==1 else 0):raise ValueError('returned actual method drain/ack counters inconsistent')
        outcome='actual_RPC_exit_method_drained_native_broadcast_then_ack_return'if post['notify']==1 else'callback_consumed_second_status_before_exit_no_method_drain_overlap'
    exact(8,dict(callbacks=1 if paired else 0,eventLoop_send_pending=paired,method_uninstall_send_pending=paired,rescue_started=False,backend_map_directly_observed=False,all_post_ack_channel_use_proved=False,independent_acceptable_candidate_set=None,select_exit_vs_second_status_controlled=False))
    observed=raw(q/'events.jsonl.before-rescue.stack.gz').decode()
    if prior.stack_frame(observed,'(*DownloaderAPI).eventLoop','/eth/downloader/api.go:119','[chan send')!=paired:raise ValueError('frozen native send state unsupported')
    if paired and not prior.stack_frame(observed,'(*SyncStatusSubscription).Unsubscribe','/eth/downloader/api.go:186','[chan send'):raise ValueError('actual deferred method wait unsupported')
    exact(10,{});exact(11,dict(rescue_used=paired,rescue_ack_is_controlled_not_real_map_ack=paired,callbacks=0,event_loops=0,ServeCodec_joined=True,memory_chain_stopped=True));exact(12,dict(failed=False))
    return dict(**slot,outcome=outcome,actual_exit_method_native_overlap_observed=post['notify']==1,pre_close_counts=pre,pre_rescue_counts=post,coverage_snapshot_mode='atomic_individual_counters_not_transactional',full_original5_scenarios_satisfied=False,backend_map_directly_observed=False,all_post_ack_channel_use_proved=False,independent_oracle=False,independent_acceptable_candidate_set=None,trace_sha256=sha(raw(q/'events.jsonl.gz')),applied_project_tree=tree['applied_project_tree'],build_input_tree=tree['build_input_tree'],binary_sha256=binary['sha256'],subscription_id_sha256=sha(subscription.encode()))


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;q=p/'actual';scope=read(p/'SCOPE.json');slots=[dict(slot_id=f'{r}-{v}-rpc-overlap',candidate=r,variant=v,repetition=1,case_id='rpc-overlap')for r in ROLES for v in ('general','race')]
    if sha((p/'SCOPE.json').read_bytes())!=SCOPE_SHA or read(q/'SCOPE.json')!=scope or scope['planned_slots']!=slots or read(q/'planned.json')!=slots:raise ValueError('frozen native RPC plan differs')
    if scope['fixture_sha256']!=sha((root/TOOLS[3]).read_bytes())or scope['main_fixture_sha256']!=sha((root/TOOLS[4]).read_bytes()):raise ValueError('frozen bridge/main differs')
    env=read(q/'environment.json')
    if env!=read(root/previous.PACKET/'actual/environment.json'):raise ValueError('explicit offline allowlisted environment changed')
    receipt=dict(planned=6,attempted=6,automatic_retries=0,source_commit=COMMIT,tracked_changes_only_exact_candidate_api=True,program_additions=[BRIDGE,MAIN],compiler_atomic_coverage=True,original_package_tests_executed=False,production_patch=False,formal_completed=0,formal_total=9,independent_approved=0,FINAL=0,paid_or_external_LLM_calls=0,operational_nodes_started=0)
    if read(q/'receipt.json')!=receipt:raise ValueError('all actual program attempt population differs')
    attempts=read(q/'attempts.json')
    if len(attempts)!=6 or {a['slot_id']for a in attempts}!={s['slot_id']for s in slots}:raise ValueError('planned program attempt absent/duplicate')
    built,trees=bound_sources(root,p);by={a['slot_id']:a for a in attempts};rows=[]
    for slot in slots:
        d=q/'slots'/slot['slot_id']
        if by[slot['slot_id']]!=read(d/'receipt.json'):raise ValueError('aggregate attempt differs')
        rows.append(inspect_slot(root,p,d,slot,built[slot['candidate']][slot['variant']],trees[slot['candidate']],scope,env))
    author_bytes=subprocess.check_output(['git','show',REVISION+':'+str(prior.draft.AUTHOR)],cwd=root);notes=json.loads(author_bytes)
    covered={'API-ENTRY','METHOD-RETURN','METHOD-ACK','BLOCKED-DELIVERY','OWNED-CLEANUP','SOURCE-BOUNDARY'}
    remaining='Finite source-coupled instrumented DEV only; transport gate ignores ordinary deadline while held and compiler counters/owned additions perturb scheduling. Exact runtime map membership/delete-before-ack and all post-ack channel use, full original5/12requirements, full host/real sync/indexer, sufficient independent repeats/gold and installed roles/finalHEAD remain unproved. No same-object Once/late-owner matrix or independent candidate selection in this route.'
    units=[dict(slot_id=row['slot_id'],candidate=row['candidate'],requirement=r['id'],assertion=r['assertion'],source_anchor_ids=r['anchors'],label='bounded_runtime_route_with_remaining_scope'if r['id']in covered else'not_exercised_or_not_directly_instrumented',full_requirement_satisfied=False,independent_approved=False,remaining=remaining)for row in rows for r in notes['requirements']]
    outcomes={name:sum(r['outcome']==name for r in rows)for name in sorted({r['outcome']for r in rows})}
    return dict(summary=dict(kind=KIND,planned_attempts=6,actual_attempts=6,valid_technical_observations=6,fresh_general_race_program_builds=6,unique_fresh_method_variants=3,previous_role_alias_and_early_mutant_not_fresh=True,native60s_timer_fires=12,caller_RPC_requests=6,wire_notifications_delivered=0,pre_rescue_counter_snapshots=12,raw_snapshot_reconversions=12,outcomes=outcomes,actual_RPC_exit_method_native_overlap_observations=sum(r['actual_exit_method_native_overlap_observed']for r in rows),uncontrolled_select_schedule_observations=sum(not r['actual_exit_method_native_overlap_observed']for r in rows),repetitions_per_role_profile=1,independent_repeated_execution_criterion_satisfied=False,requirements_author_revision=REVISION,requirements_author_sha256=sha(author_bytes),requirement_units=len(units),full_original5_scenarios_satisfied=0,full12_requirement_satisfaction=False,backend_map_directly_observed=False,all_post_ack_channel_use_proved=False,compiler_atomic_coverage=True,transport_gate_ignores_deadline_while_held=True,original_package_tests_executed=False,independent_approved=0,independent_tickets=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,independent_acceptable_candidate_set=None,canonical_selection=None,quality_score=None,paid_or_external_LLM_calls=0,operational_nodes_started=0),observations=rows,requirements=units)


def worklist(root=ROOT):
    d=read(root/previous.PACKET/'WORKLIST.json');d.update(current_stage='67_WIV1_V2_native_RPC_overlap_atomic_counter_DEV',completed_in_this_scope=['original_and_A_B_6fresh_atomic_coverage_general_race_programs','actual_native_timer_and_RPC_write_gate_connection_close_with_pre_rescue_stacks_and_counters','uncontrolled_select_outcome_separated_from_actual_method_drain_overlap','user_deferred_security_only_evaluations_not_marked_complete'],next_task='WI_V1_V2_runtime_map_membership_delete_ack_and_post_ack_use_scope_then_WI6_non_security_retrieval_pairs',last_verified_date='2026-10-19')
    d['remaining_all'][0]['task']='WI-V1: actual native broadcast/RPC connection-close route bounded observations now recorded with atomic counters; direct runtime map membership/delete-before-ack/all post-ack use, original5/12 full scope, full factory/sync/indexer, independent tickets/oracles and installed roles/finalHEAD remain'
    d['remaining_all'][1]['task']='WI-V2: actual method-drain and select-consumed-second-status schedules distinguished; independent acceptable set/selection null. Full matrix, independent primary/derived oracle and sufficient repeated population remain'
    d['remaining_all'][3].update(status='pending_non_security_scope',task='WI-SP active non-security scope: actual overlay/consumer, manual/automatic provenance, policy/untagged gold and independent symbols',user_deferred=['security-specific gold','ACL/authz boundary evaluation','private-data boundary evaluation','forgery boundary evaluation'])
    d['remaining_all'][12].update(task='공개 CLI/HTTP·graph export; graph 점수/확장; domain flow gold; 대규모 corpus/긴 함수; PR/as-of; legacy 출처; CoreML; embedding cache; 운영 감사; viewer; 배포/downstream',user_deferred=['organization authentication/authorization'])
    d['security_scope_adjustment']=dict(basis='direct_user_request_skip_security_related_items',decision='defer_security_only_evaluation_and_features; continue_functional_reliability_work',deferred_is_completed=False,process_watchdogs_and_owned_resource_cleanup='retained_as_reliability_requirements',formal_freeze='not_frozen; deferred criteria cannot be counted as passed')
    d['AI_DEV_draft_progress']['WI-V1']='phase67_actual_native_RPC_close_gated_program6_atomic_pre_rescue_snapshots;uncontrolled_select_separate;map_ack_all_use_and_formal_scope_pending'
    d['AI_DEV_draft_progress']['WI-V2']='3fresh_variants;other2roles_historical_only;independent_set_null'
    return d


def prepare(work,p):
    if any(x.name not in ('SCOPE.json','preflight')for x in p.iterdir()):raise ValueError('refusing existing RPC-overlap evidence overwrite')
    source=work/'rpc-overlap-trials-v1'
    for x in sorted(source.rglob('*')):
        if not x.is_file()or x.suffix=='.program':continue
        y=p/'actual'/x.relative_to(source);y.parent.mkdir(parents=True,exist_ok=True)
        if x.name in ('stdout','stderr')or x.suffix in('.jsonl','.stack'):y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
        else:shutil.copyfile(x,y)
    for role in ROLES:
        for v,b in read(p/'actual/candidates'/role/'binaries.json').items():
            if sha(Path(b['path']).read_bytes())!=b['sha256']:raise ValueError('postrun program changed')
    tools=[Path(GO),Path(GO).parent.parent/'pkg/tool/darwin_arm64/covdata'];write(p/'coverage-tool.json',{str(n):sha(n.read_bytes())for n in tools})
    data=derive(packet=p)
    if historical_errors():raise ValueError('historical source/evidence changed')
    for name,key in [('SUMMARY.json','summary'),('observations.json','observations'),('requirements.json','requirements')]:write(p/name,data[key])
    write(p/'native-reuse.json',native_record());write(p/'WORKLIST.json',worklist())
    (p/'REPORT.txt').write_text('Phase67: actual native RPC connection-close/Unsubscribe overlap DEV observations. No independent acceptance.\n\n'
      'Three exact retained variants ORIGINAL/A/B, six fresh general/race standalone programs, full2228 baseline entries and real Git project/build trees are bound. Only the exact API overlay changes tracked source and two declared observation program files are added. Source tests/init remain in the tree but are not executed; this is not an upstream full-test run. Compiler atomic coverage includes the program entry point and downloader package; counters and the bridge perturb scheduling.\n'
      'Actual owned memory chain/snap/controlled Downloader height1 and test mux StartEvent drive the unchanged native60second timer. Actual Server.ServeCodec/NewCodec/net.Pipe subscription reaches Syncing. The first notification write is held in a test-owned transport gate that intentionally ignores ordinary write deadline while held; it is not a real-network timeout claim. At the second native timer the same only status channel send blocks while its actual RPC callback is in Notify. Client Close/server Stop occur before releasing the write gate. No unsubscribe-true response is fabricated.\n'
      'The callback select between closed exit channels and the ready second status is not forced. Source-bound atomic API counters and real stacks before rescue distinguish actual exit/deferred-method overlap from consuming the second status before exit. Both schedules are retained; absence of overlap in a slot is not silently called proof. Converted snapshots are independently regenerated from raw binary metadata/counters using the pinned local SDK. Each pre-close/pre-rescue directory has one metadata and one counter file; final automatic coverage is separate. Atomic individual counters are not a transactional global snapshot or independent semantic oracle.\n'
      'For original paired waits, rescue receives the actual pending request, drains only that owned status channel, and closes a controlled cleanup ack after observation. This is not an observed real map deletion/ack. Scoped callback/eventLoop/codec/memory chain and process groups are joined/reaped. Direct backend map membership, precise deletion-before-ack/all post-ack uses, full factory/sync/indexer/installed roles, full original5 and12requirements, independent gold/repeats/candidate set remain pending.\n'
      'User-requested security-only evaluations/features are deferred in WORKLIST, never marked complete. Functional reliability and owned cleanup continue. D04 unassigned, anonymous roles allowed, independent0/FINAL0/formal0of9/freeze null/execution false; no paid/external LLM, publication, global install/settings or operational node. CKS prior native build/tests/fmt/vet/lint proof reused only with unchanged input identity, not fresh full CKS runs.\n')
    seal(p)



def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=read(p/'manifest.json');actual={x.relative_to(p).as_posix():sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('native evidence inventory differs')
        for r in m['tools']:
            if sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('bound native tool changed '+r['path'])
        errors+=historical_errors(root);data=derive(root,p)
        for name,key in [('SUMMARY.json','summary'),('observations.json','observations'),('requirements.json','requirements')]:
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
        if a.require_formal_gold:errors.append('Controlled transport/compiler-counter RPC overlap DEV observations are not independent approved/frozen gold')
        print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
