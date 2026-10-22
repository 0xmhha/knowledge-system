"""Recompute the in-progress (Syncing=true) notification generation boundaries."""
import json
from ticket_actual_rpc import flat_map, tree_oid
from source_packet import oid
import wiv1_generation_boundary as previous
import wiv1_transport_generation as stage92
from wiv1_generation_boundary import sha, raw, read, require, one, count

ROOT = previous.ROOT
FOLLOWUPS = previous.FOLLOWUPS
PACKET = FOLLOWUPS/'wiv1-progress-true-generation-v1'
COMMIT = previous.COMMIT
SCHEDULES = {'progress-retained': ['ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-STALE-ACK'], 'progress-to-done': ['ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-EARLY']}
PROFILES = previous.PROFILES
API, BRIDGE, MAIN = previous.API, 'eth/downloader/ks_progress_generation.go', 'cmd/ks-progress-generation/main.go'
FIXTURE_BLOCKS = 1024
BASELINE = FOLLOWUPS/'wi4-target-subscription-v1/corrective/source-binding.json'
PROGRAM_TIMEOUT = 330
EXITS = ('callback_exit_rpc_error', 'callback_exit_transport_closed')
TOOLS = ['scripts/wiv1_progress_generation.py', 'scripts/test_wiv1_progress_generation.py', 'scripts/wiv1_transport_generation.py', 'scripts/wiv1_generation_boundary.py',
         'scripts/ticket_actual_rpc.py', 'scripts/source_packet.py']


def stalled(send):
    """Return the progress the owner sent, refusing a send that reports a finished sync."""
    value = send['value']
    require(isinstance(value, dict) and value['current_block'] < value['highest_block'] == FIXTURE_BLOCKS, 'progress send does not report a stalled sync')
    return value


def inspect_trace(rows, schedule, role):
    require([r['sequence'] for r in rows] == list(range(1, len(rows)+1)), 'trace sequence incomplete')
    require(all(a['elapsed_ns'] <= b['elapsed_ns'] for a, b in zip(rows, rows[1:])), 'trace clock order differs')
    require(len([r for r in rows if r['event'] == 'factory_API_created']) == 2, 'not exactly two factory APIs')
    selected = one(rows, 'selected_factory_subscription')
    api, old = selected['api_id'], selected['channel_id']
    require(api > 0 and old > 0 and selected['value']['unbuffered_status_channel'] is True and selected['value']['generation'] == 1, 'factory/subscription identity missing')
    local = [r for r in rows if r['api_id'] == api]
    gen, acked, uses = previous.generations(rows, api)
    early, stale = role == 'MUT-EARLY', role == 'MUT-STALE-ACK'
    starts = [r for r in local if r['event'] == 'source_StartEvent_received']
    fired = [r for r in local if r['event'] == 'native_timer_fired']
    first = [r for r in local if r['channel_id'] == old and r['generation'] == 1 and r['event'] == 'native_send_begin_progress']
    require(len(first) == 1 and starts and starts[0]['sequence'] < first[0]['sequence'], 'old generation did not receive exactly one progress send after the source start')
    first_timer = [r for r in fired if r['sequence'] < first[0]['sequence']]
    require(first_timer and first_timer[-1]['api_age_ns'] >= 60_000_000_000 and starts[0]['sequence'] < first_timer[-1]['sequence'], 'original native60 timer/source event absent')
    progress = stalled(first[0])
    require(all(count(local, old, e, 1) == 1 for e in ('native_send_end_progress', 'callback_status_received', 'callback_notify_begin', 'callback_notify_end')), 'progress not forwarded by the old callback')
    forwarded = one(local, 'old_progress_forwarded')
    begin = one(local, 'actual_RPC_unsubscribe_begin')
    done_call = one(local, 'actual_RPC_unsubscribe_true')
    exits = [r for r in local if r['channel_id'] == old and r['generation'] == 1 and r['event'] in EXITS]
    require(len(exits) == 1 and exits[0]['event'] == 'callback_exit_rpc_error', 'old callback did not end through the RPC unsubscribe')
    notify_end = one([r for r in local if r['channel_id'] == old and r['generation'] == 1], 'callback_notify_end')
    require(begin['channel_id'] == done_call['channel_id'] == old and notify_end['sequence'] < forwarded['sequence'] < begin['sequence'] < exits[0]['sequence'] and begin['sequence'] < done_call['sequence'], 'progress delivery or unsubscribe ordering differs')
    require(not any(r['event'] in ('actual_transport_close_begin', 'actual_transport_closed') for r in local), 'transport close used instead of the RPC unsubscribe')
    frozen = one(local, 'progress_observation_frozen')
    require(isinstance(frozen['value'], dict) and isinstance(frozen['value'].get('counts'), dict), 'frozen observation payload missing')
    counts = frozen['value']['counts']
    expected = dict(callback_returned=1, method_entry=1, method_return=1, method_ack_observed=int(not early), ack_close_end=int(not early), after_delete=int(not early and not stale))
    require({k: counts.get(k, 0) for k in expected} == expected and frozen['value']['member'] is (early or stale), 'unsubscribed candidate outcome differs')
    old_ack = [r for r in local if r['channel_id'] == old and r['event'] == 'ack_close_end' and r['generation'] == 1]
    regen = one(local, 'regeneration_begin')
    require(len(old_ack) == 1 and old_ack[0]['sequence'] < regen['sequence'], 'regeneration began before old real owner ack')
    manual = [r for r in local if r['event'] == 'owned_rescue_manual_uninstall_begin']
    require(all(r['sequence'] > frozen['sequence'] for r in manual), 'manual cleanup before frozen observation')
    reinstall = [r for r in local if r['channel_id'] == old and r['event'] == 'before_install' and r['generation'] == 1]
    require(len(reinstall) == 1 and count(local, old, 'install', 2) == 1 and reinstall[0]['sequence'] > regen['sequence'], 'same-channel generation 2 registration missing')
    require(count(local, old, 'after_delete', 1) == int(not stale) and old_ack[0]['member'] is stale and reinstall[0]['member'] is stale, 'stale membership differs from role')
    fresh_rows = [r for r in local if r['event'] == 'new_rpc_registration_installed']
    require(len(fresh_rows) == 1 and fresh_rows[0]['value'] == dict(generation=1, distinct_channel=True) and fresh_rows[0]['channel_id'] != old, 'new RPC registration not a distinct channel')
    fresh = fresh_rows[0]['channel_id']
    fresh_install = one([r for r in local if r['channel_id'] == fresh], 'install')
    require(count(local, old, 'install_send_begin') == 0 and count(local, fresh, 'install_send_begin') == 0, 'install send although the sync was not done')
    final = 'native_send_begin_progress' if schedule == 'progress-retained' else 'native_send_begin_done'
    old_final = [r for r in local if r['channel_id'] == old and r['generation'] == 2 and r['event'] == final]
    fresh_final = [r for r in local if r['channel_id'] == fresh and r['event'] == final]
    require(len(old_final) == 1 and len(fresh_final) == 1 and count(local, old, 'native_send_begin_done', 1) == 0, 'next broadcast did not address generation 2 and the new registration once')
    timer = [r for r in fired if r['sequence'] < min(old_final[0]['sequence'], fresh_final[0]['sequence'])][-1]
    require(reinstall[0]['sequence'] < timer['sequence'] and fresh_install['sequence'] < timer['sequence'] and timer['api_age_ns'] >= first_timer[-1]['api_age_ns']+60_000_000_000, 'registrations not before the next native timer')
    sends2 = count(local, old, 'native_send_begin_progress', 2)+count(local, old, 'native_send_begin_done', 2)
    require(count(local, old, 'probe_status_received', 2) == sends2 and count(local, fresh, 'native_send_begin_progress')+count(local, fresh, 'native_send_begin_done') == sends2, 'generation 2 and the new registration were not addressed alike')
    timer_frozen = one(local, 'timer_observation_frozen')
    old_ack2 = [r for r in local if r['channel_id'] == old and r['event'] == 'ack_close_end' and r['generation'] == 2]
    require(gen.get(old) == 2 and acked.get(old) == 2 and len(old_ack2) == 1 and timer_frozen['sequence'] < old_ack2[0]['sequence'], 'same-channel generation 2 not observed before its acknowledgement')
    client = [r for r in local if r['event'] == 'client_notification_received']
    fresh_ack = [r for r in local if r['channel_id'] == fresh and r['event'] == 'ack_close_end']
    require(client and all(r['channel_id'] == fresh for r in client) and len(fresh_ack) == 1 and client[-1]['sequence'] < fresh_ack[0]['sequence'], 'new registration delivery or real owner ack differs')
    bodies = [json.loads(r['value']) for r in client]
    restore = [r for r in local if r['event'] in ('peer_restore_begin', 'peer_restore_sync_completed')]
    if schedule == 'progress-retained':
        require(len(client) == 1 and bodies[0]['syncing'] is True and bodies[0]['status']['CurrentBlock'] < bodies[0]['status']['HighestBlock'] == FIXTURE_BLOCKS, 'new registration did not receive a progress result')
        require(sends2 == 1 and not restore and count(local, fresh, 'native_send_begin_done') == 0, 'retained schedule finished the sync')
        stalled(old_final[0])
        stalled(fresh_final[0])
        outcome = 'stale_entry_merged_into_generation2' if stale else 'generation2_and_new_registration_addressed_by_progress'
    else:
        require([r['event'] for r in restore] == ['peer_restore_begin', 'peer_restore_sync_completed'] and fresh_install['sequence'] < restore[0]['sequence'] and restore[1]['sequence'] < timer['sequence'], 'peer restore not between the registrations and the done broadcast')
        require(bodies[-1] is False and all(b is not False and b['syncing'] is True for b in bodies[:-1]) and len(bodies) == sends2, 'new registration did not end on the done result')
        outcome = 'early_return_cleaned_then_done' if early else 'progress_then_done_across_generations'
    rescues = [r for r in local if r['event'] == 'owned_rescue_status_receive_begin']
    require(not rescues and len(manual) == 3*int(early), 'cleanup interventions omitted/promoted')
    violations = [u for c in uses for u in uses[c] if u['post_ack']]
    summary = one(local, 'generation_summary')['value']
    require({r['channel_id']: r['post_ack_same_generation_uses'] for r in summary} == {c: len([u for u in uses.get(c, []) if u['post_ack']]) for c in {r['channel_id'] for r in summary}}, 'generation summary differs from trace')
    require(not violations, 'post-ack same-generation use observed')
    return dict(api_id=api, old_channel_id=old, new_channel_id=fresh, native_age_ns=first_timer[-1]['api_age_ns'], next_native_age_ns=timer['api_age_ns'],
                old_progress=dict(current_block=progress['current_block'], highest_block=progress['highest_block']), old_callback_exit=exits[0]['event'],
                next_broadcast='progress' if schedule == 'progress-retained' else 'done', progress_sends_before_next=sends2-1 if schedule == 'progress-to-done' else 0,
                post_ack_same_generation_uses=len(violations), stale_membership_at_reregistration=reinstall[0]['member'],
                owned_status_receives=len(rescues), owned_manual_uninstalls=len(manual), fake_acknowledgements=0, outcome=outcome)


def inspect_process(events, schedule):
    require(len([r for r in events if r['event'] == 'factory_started']) == 2 and one(events, 'actual_sync_start')['value'] is True, 'actual peer sync not observed')
    imported = one(events, 'seeder_imported')['value']
    removed = one(events, 'actual_peer_removed')['value']
    stall = one(events, 'actual_sync_stalled')['value']
    require(imported['blocks'] == FIXTURE_BLOCKS and removed['head'] >= 1 and removed['progress']['HighestBlock'] == FIXTURE_BLOCKS, 'peer not removed during the real sync')
    require(stall['peers'] == 0 and removed['head'] <= stall['current_block'] < stall['highest_block'] == FIXTURE_BLOCKS, 'sync not stalled below the seeder head')
    one(events, 'actual_sync_failed')
    for name in ('old_generation_real_owner_ack_observed', 'generation_boundary_completed'):
        one(events, name)
    restored = [r for r in events if r['event'] in ('loopback_peer_restored', 'actual_sync_done', 'actual_receiver_head', 'actual_indexer_complete')]
    if schedule == 'progress-to-done':
        head = one(events, 'actual_receiver_head')['value']
        require([r['event'] for r in restored] == ['loopback_peer_restored', 'actual_sync_done', 'actual_receiver_head', 'actual_indexer_complete'] and head['number'] == FIXTURE_BLOCKS and head['matches_seeder'] is True and head['hash'] == imported['head'], 'restored sync did not reach the seeder head')
    else:
        require(not restored, 'retained schedule restored the peer')
    require(one(events, 'receiver_close_returned')['value'] is True and one(events, 'seeder_close_returned')['value'] is True and events[-1]['event'] == 'complete', 'real nodes did not close')
    return dict(stalled_current_block=stall['current_block'], peer_removed_at_head=removed['head'])


def derive(p):
    scope = read(p/'actual/SCOPE.json.gz')
    slots = [dict(slot_id=f'{s}-{r}-{v}', schedule=s, candidate=r, variant=v, repetition=1) for s, rs in SCHEDULES.items() for r in rs for v in PROFILES]
    require(scope['source_commit'] == COMMIT and scope['native_timer_seconds'] == 60 and scope['fixture_blocks'] == FIXTURE_BLOCKS and scope['planned_slots'] == slots and scope['automatic_retries'] == 0 and scope['program_timeout'] == PROGRAM_TIMEOUT, 'native source/schedule or slot population changed')
    require(scope['independent_approved'] == scope['formal_completed'] == 0 and scope['freeze'] is None and scope['execution_allowed'] is False, 'acceptance boundary changed')
    # The adapter is the one stage 91 preflighted; its module bytes must not change.
    prior, stage92_packet = ROOT/previous.PACKET, ROOT/stage92.PACKET
    require(scope['adapter_module_sha256'] == sha(raw(prior/'drivers/ks_adapter.py.gz')) and raw(p/'drivers/ks_adapter.py.gz') == raw(prior/'drivers/ks_adapter.py.gz'), 'adapter differs from the preflighted one')
    preflight = read(prior/'adapter/PREFLIGHT.json')
    require(scope['adapter']['previous_locations'] == preflight['adapter']['previous_locations'], 'adapter location map differs')
    bridge, main = raw(p/'drivers/progress-bridge.go.gz'), raw(p/'drivers/progress-main.go.gz')
    require(sha(bridge) == scope['bridge_sha256'] and sha(main) == scope['main_sha256'] and b'.Post(' not in main, 'fixture bytes differ')
    instrument = raw(p/'drivers/transport_instrument.py.gz')
    require(scope['driver_sha256'] == sha(raw(p/'drivers/progress_run.py.gz')) and scope['instrumentation_sha256'] == sha(instrument), 'runtime driver identity differs')
    require(instrument == raw(stage92_packet/'drivers/transport_instrument.py.gz'), 'instrumentation differs from stage 92')
    previous_locations = [r['previous'].encode() for r in preflight['adapter']['previous_locations']]
    for name in sorted([*(p/'actual').rglob('*'), *(p/'checks').rglob('*')]):
        if name.is_file():
            data = raw(name)
            for location in previous_locations:
                data = data.replace(location, b'')
            require(not previous.LIVE_PATH.search(data), 'live machine path archived: '+str(name.relative_to(p)))
    baseline = flat_map(json.loads((ROOT/FOLLOWUPS/'ticket-method-schedules-v1/actual/candidates/ORIGINAL/base-flat-tree.json').read_bytes()))
    binaries, sites = {}, {}
    for role in scope['unique_build_roles']:
        q = p/'actual/candidates'/role
        b, t = read(q/'source-binding.json.gz'), read(q/'tree-binding.json.gz')
        api = raw(q/'instrumented-api.go.gz')
        retained = previous.overlay(role)
        require(b['source_commit'] == COMMIT and b['production_patch'] is False and b['base_tracked_matches_retained'] is True and b['retained_api_sha256'] == sha(retained) and b['applied_api_sha256'] == sha(api), 'candidate source scope differs')
        require(b'checkInterval     = time.Second * 60' in api and api.count(b'ksNativeLifecycle(api, "before_install", i, syncSubscriptions)') == 1, 'native timer or membership rule changed')
        sites[role] = previous.sites_covered(retained, api)
        project = dict(baseline)
        project[API] = dict(mode='100644', kind='blob', oid=oid('blob', api))
        build_tree = dict(project)
        build_tree.update({n: dict(mode='100644', kind='blob', oid=oid('blob', d)) for n, d in ((BRIDGE, bridge), (MAIN, main))})
        require(b['additions'] == {BRIDGE: sha(bridge), MAIN: sha(main)} and flat_map(t['project_flat_tree']) == project and flat_map(t['build_flat_tree']) == build_tree and tree_oid(t['project_flat_tree']) == t['applied_project_tree'] and tree_oid(t['build_flat_tree']) == t['build_input_tree'], 'full project/build Git tree differs')
        binaries[role] = read(q/'binaries.json.gz')
        for variant in PROFILES:
            receipt = read(q/('build-'+variant)/'receipt.json.gz')
            require(receipt['exit_code'] == 0 and receipt['reaped'] and not any(receipt[k] for k in ('timeout', 'remaining_live_group_members')), 'candidate build failed')
            expected = ['${GOROOT}/bin/go', 'build']+(['-race'] if variant == 'race' else [])+['-o', binaries[role][variant]['path'], './cmd/ks-progress-generation']
            require(receipt['argv'] == expected and previous.PLACEHOLDER.match(binaries[role][variant]['path']), 'candidate build profile differs')
            metadata = raw(q/(variant+'-build-info.txt.gz')).decode()
            require('go1.23.12' in metadata and 'GOOS=darwin' in metadata and 'GOARCH=arm64' in metadata and 'vcs.revision='+COMMIT in metadata and (variant != 'race' or '-race=true' in metadata), 'actual native binary profile differs')
    attempts = read(p/'actual/attempts.json.gz')
    require(len(attempts) == len(slots) and [{k: r[k] for k in slot} for r, slot in zip(attempts, slots)] == slots, 'actual planned attempt population differs')
    rows = []
    for slot, attempt in zip(slots, attempts):
        q = p/'actual/slots'/slot['slot_id']
        require(read(q/'receipt.json.gz') == attempt and attempt['binary_sha256'] == binaries[slot['candidate']][slot['variant']]['sha256'] and attempt['exit_code'] == 0 and attempt['reaped'] and not any(attempt[k] for k in ('timeout', 'term_sent', 'kill_sent', 'remaining_live_group_members', 'tmp_children_after')), 'slot result/binary or cleanup differs')
        require(120 <= attempt['elapsed_seconds'] < PROGRAM_TIMEOUT, 'actual native runtime duration differs')
        proc = read(q/'process/receipt.json.gz')
        for stream in ('stdout', 'stderr'):
            require(sha(raw(q/('process/'+stream+'.gz'))) == proc[stream+'_sha256'], 'slot raw process stream differs')
        events = [json.loads(line) for line in raw(q/'process/stdout.gz').splitlines()]
        process = inspect_process(events, slot['schedule'])
        trace = [json.loads(line) for line in raw(q/'events.jsonl.gz').splitlines()]
        result = inspect_trace(trace, slot['schedule'], slot['candidate'])
        require(result['old_progress']['current_block'] >= process['stalled_current_block'], 'progress send precedes the recorded stall')
        rows.append(dict(**slot, **process, **result))
    # The owned upstream clone is checked out at the pin; an empty inventory would compare equal without proving anything.
    original = read(p/'actual/original-before.json.gz')
    require(original == read(ROOT/BASELINE)['tracked'], 'original upstream inventory is not the retained pinned tree')
    require(original == read(p/'actual/original-after.json.gz') and raw(p/'actual/original-status-before.txt.gz') == raw(p/'actual/original-status-after.txt.gz') == b'', 'original upstream source changed')
    retained = [r for r in rows if r['schedule'] == 'progress-retained']
    done = [r for r in rows if r['schedule'] == 'progress-to-done']
    return dict(kind='actual_factory_progress_true_generation_boundary_DEV_not_independent', planned_slots=len(slots), actual_slots=len(rows),
                actual_owned_nodes=2*len(rows), unique_build_roles=len(scope['unique_build_roles']), channel_sites_instrumented=sites, automatic_retries=0,
                fixture_blocks=FIXTURE_BLOCKS, progress_sends_to_old_generation=len(rows),
                old_generation_end={'RPC_unsubscribe': len([r for r in rows if r['old_callback_exit'] == EXITS[0]])},
                stalled_current_block_range=[min(r['stalled_current_block'] for r in rows), max(r['stalled_current_block'] for r in rows)],
                progress_broadcasts_to_generation2=len(retained), done_after_progress_to_generation2=len(done),
                progress_sends_before_done=sum(r['progress_sends_before_next'] for r in done), new_rpc_registration_deliveries=len(rows),
                post_ack_same_generation_uses=sum(r['post_ack_same_generation_uses'] for r in rows),
                stale_membership_at_reregistration={role: len([r for r in retained if r['candidate'] == role and r['stale_membership_at_reregistration']]) for role in SCHEDULES['progress-retained']},
                stale_control_detected_by_channel_use=0, owned_status_receives=sum(r['owned_status_receives'] for r in rows),
                owned_manual_uninstalls=sum(r['owned_manual_uninstalls'] for r in rows), fake_acknowledgements=0,
                progress_true_notification_tested=True, universal_post_ack_claim=False, full12_acceptance=False, independent_families=0,
                formal_completed=0, independent_approved=0, FINAL=0, freeze=None, execution_allowed=False, actual_LLM_calls=0, paid_calls=0, rows=rows)


def seal(p):
    (p/'manifest.json').write_text(json.dumps(dict(tools={n: sha((ROOT/n).read_bytes()) for n in TOOLS}, artifacts={str(n.relative_to(p)): sha(n.read_bytes()) for n in sorted(p.rglob('*')) if n.is_file() and n.name != 'manifest.json'}), indent=2)+'\n')


def check(p=None):
    p = p or ROOT/PACKET
    try:
        m = read(p/'manifest.json')
        require(m['tools'] == {n: sha((ROOT/n).read_bytes()) for n in TOOLS}, 'verification source changed')
        require(m['artifacts'] == {str(n.relative_to(p)): sha(n.read_bytes()) for n in p.rglob('*') if n.is_file() and n.name != 'manifest.json'}, 'packet bytes changed')
        require(read(p/'SUMMARY.json') == derive(p), 'trace/summary or independent acceptance boundary differs')
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration) as error:
        return [str(error)]


if __name__ == '__main__':
    errors = check()
    if errors:
        raise SystemExit('\n'.join(errors))
    print('actual progress (Syncing=true) generation boundary16 slots; post-ack uses0; independent/formal0')
