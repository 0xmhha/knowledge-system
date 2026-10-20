"""Recompute the Notify-in-flight and ungated transport-close generation boundaries."""
import json
from pathlib import Path
from ticket_actual_rpc import flat_map, tree_oid, stack_frame
from source_packet import oid
import wiv1_generation_boundary as previous
import wiv1_transport_generation as stage92
from wiv1_generation_boundary import sha, raw, read, require, one, count

ROOT = previous.ROOT
FOLLOWUPS = previous.FOLLOWUPS
PACKET = FOLLOWUPS/'wiv1-notify-transport-generation-v1'
COMMIT = previous.COMMIT
SCHEDULES = {'notify-in-flight': ['ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-EARLY'], 'transport-ungated': ['ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-STALE-ACK']}
PROFILES = previous.PROFILES
API, BRIDGE, MAIN = previous.API, 'eth/downloader/ks_notify_generation.go', 'cmd/ks-notify-generation/main.go'
EXITS = ('callback_exit_rpc_error', 'callback_exit_transport_closed')
TOOLS = ['scripts/wiv1_notify_transport_generation.py', 'scripts/test_wiv1_notify_transport_generation.py', 'scripts/wiv1_transport_generation.py', 'scripts/wiv1_generation_boundary.py',
         'scripts/ticket_actual_rpc.py', 'scripts/source_packet.py']


def inspect_trace(rows, schedule, role):
    require([r['sequence'] for r in rows] == list(range(1, len(rows)+1)), 'trace sequence incomplete')
    require(all(a['elapsed_ns'] <= b['elapsed_ns'] for a, b in zip(rows, rows[1:])), 'trace clock order differs')
    require(len([r for r in rows if r['event'] == 'factory_API_created']) == 2, 'not exactly two factory APIs')
    selected = one(rows, 'selected_factory_subscription')
    api, old = selected['api_id'], selected['channel_id']
    require(api > 0 and old > 0 and selected['value']['unbuffered_status_channel'] is True and selected['value']['generation'] == 1, 'factory/subscription identity missing')
    local = [r for r in rows if r['api_id'] == api]
    gen, acked, uses = previous.generations(rows, api)
    start = one(local, 'source_StartEvent_received')
    fired = [r for r in local if r['event'] == 'native_timer_fired']
    broadcast = [r for r in local if r['event'] == 'native_send_begin_done']
    require(broadcast and fired and fired[-1]['api_age_ns'] >= 60_000_000_000 and start['sequence'] < fired[-1]['sequence'] < broadcast[0]['sequence'], 'original native60 timer/source event absent')
    old_ack = [r for r in local if r['channel_id'] == old and r['event'] == 'ack_close_end' and r['generation'] == 1]
    require(len(old_ack) == 1, 'old generation real owner ack missing')
    regen = one(local, 'regeneration_begin')
    require(old_ack[0]['sequence'] < regen['sequence'], 'regeneration began before old real owner ack')
    exits = [r for r in local if r['channel_id'] == old and r['generation'] == 1 and r['event'] in EXITS]
    require(len(exits) == 1, 'old callback exit route missing or repeated')
    reinstall = [r for r in local if r['channel_id'] == old and r['event'] == 'before_install' and r['generation'] == 1]
    require(len(reinstall) == 1 and count(local, old, 'install', 2) == 1 and reinstall[0]['sequence'] > regen['sequence'], 'same-channel generation 2 registration missing')
    fresh_rows = [r for r in local if r['event'] == 'new_rpc_registration_installed']
    require(len(fresh_rows) == 1 and fresh_rows[0]['value'] == dict(generation=1, distinct_channel=True) and fresh_rows[0]['channel_id'] != old, 'new RPC registration not a distinct channel')
    fresh = fresh_rows[0]['channel_id']
    client = one(local, 'client_notification_received')
    fresh_ack = [r for r in local if r['channel_id'] == fresh and r['event'] == 'ack_close_end']
    require(client['channel_id'] == fresh and client['value'] == 'false' and len(fresh_ack) == 1 and client['sequence'] < fresh_ack[0]['sequence'], 'new registration delivery or real owner ack differs')
    require(count(local, fresh, 'callback_status_received') == 1 and count(local, fresh, 'callback_notify_end') == 1, 'new registration callback delivery differs')
    old_ack2 = [r for r in local if r['channel_id'] == old and r['event'] == 'ack_close_end' and r['generation'] == 2]
    require(gen.get(old) == 2 and acked.get(old) == 2 and len(old_ack2) == 1 and count(local, old, 'probe_status_received', 2) == 1, 'same-channel generation 2 not delivered and acknowledged')
    rescues = [r for r in local if r['event'] == 'owned_rescue_status_receive_begin']
    manual = [r for r in local if r['event'] == 'owned_rescue_manual_uninstall_begin']
    violations = [u for c in uses for u in uses[c] if u['post_ack']]
    summary = one(local, 'generation_summary')['value']
    require({r['channel_id']: r['post_ack_same_generation_uses'] for r in summary} == {c: len([u for u in uses.get(c, []) if u['post_ack']]) for c in {r['channel_id'] for r in summary}}, 'generation summary differs from trace')
    require(not violations, 'post-ack same-generation use observed')
    early, stale = role == 'MUT-EARLY', role == 'MUT-STALE-ACK'
    row = dict(api_id=api, old_channel_id=old, new_channel_id=fresh, native_age_ns=fired[-1]['api_age_ns'], old_callback_exit=exits[0]['event'],
               post_ack_same_generation_uses=len(violations), stale_membership_at_reregistration=reinstall[0]['member'],
               owned_status_receives=len(rescues), owned_manual_uninstalls=len(manual), fake_acknowledgements=0)
    begin = one(local, 'actual_transport_close_begin')
    closed = one(local, 'actual_transport_closed')
    require(begin['channel_id'] == closed['channel_id'] == old and closed['value'] == dict(owner_ack_not_inferred=True) and begin['sequence'] < exits[0]['sequence'] and begin['sequence'] < closed['sequence'], 'transport close not the old generation end')
    require(not any(r['event'] == 'actual_RPC_unsubscribe_true' for r in local), 'RPC unsubscribe used instead of transport close')
    if schedule == 'notify-in-flight':
        gate = one(local, 'callback_notify_gate_observed')
        settle = one(local, 'transport_teardown_settled')
        release = one(local, 'native_gate_release')
        require(gate['channel_id'] == old and gate['sequence'] < begin['sequence'] < closed['sequence'] < settle['sequence'] < release['sequence'] < exits[0]['sequence'] and settle['value'] == dict(settle_ms=500), 'held Notify ordering differs')
        require(all(count(local, old, e, 1) == 1 for e in ('native_send_begin_done', 'native_send_end_done', 'callback_status_received', 'callback_notify_begin', 'callback_notify_end')), 'done status not held inside Notify')
        require(one([r for r in local if r['channel_id'] == old and r['generation'] == 1], 'callback_notify_end')['sequence'] > release['sequence'], 'Notify finished before the held release')
        frozen = one(local, 'notify_observation_frozen')
        require(isinstance(frozen['value'], dict) and isinstance(frozen['value'].get('counts'), dict), 'frozen observation payload missing')
        counts = frozen['value']['counts']
        expected = dict(callback_returned=1, method_entry=1, method_return=1, method_ack_observed=int(not early), after_delete=int(not early), ack_close_end=int(not early))
        require({k: counts.get(k, 0) for k in expected} == expected and sum(counts.get(k, 0) for k in EXITS) == 1 and frozen['value']['member'] is early, 'held-Notify candidate outcome differs')
        require(all(r['sequence'] > frozen['sequence'] for r in manual), 'manual cleanup before frozen observation')
        require(count(local, old, 'install_send_begin', 2) == 1 and count(local, fresh, 'install_send_begin', 1) == 1 and count(local, fresh, 'native_send_begin_done') == 0, 'post-broadcast registrations not served by install send')
        require(not rescues and len(manual) == 3*int(early) and reinstall[0]['member'] is False, 'cleanup interventions omitted/promoted')
        row.update(outcome='early_return_cleaned_then_regenerated' if early else 'held_notify_ended_then_regenerated', old_generation2_route='install_send')
    else:
        timer = fired[-1]['sequence']
        fresh_install = one([r for r in local if r['channel_id'] == fresh], 'install')
        require(begin['sequence'] < old_ack[0]['sequence'] < reinstall[0]['sequence'] < timer and fresh_install['sequence'] < timer, 'ungated close or registrations not before the native timer')
        require(count(local, old, 'native_send_begin_done', 1) == 0 and count(local, old, 'native_send_begin_done', 2) == 1 and count(local, fresh, 'native_send_begin_done', 1) == 1, 'natural broadcast did not address generation 2 and the new registration once')
        require(count(local, old, 'install_send_begin') == 0 and count(local, fresh, 'install_send_begin') == 0, 'install send before the native broadcast')
        frozen = one(local, 'timer_observation_frozen')
        require(frozen['sequence'] < old_ack2[0]['sequence'], 'generation 2 acknowledged before the broadcast was observed')
        require(count(local, old, 'after_delete', 1) == int(not stale) and old_ack[0]['member'] is stale and reinstall[0]['member'] is stale and old_ack2[0]['member'] is stale, 'stale membership differs from role')
        require(not rescues and not manual, 'rescue observed on the ungated schedule')
        row.update(outcome='stale_entry_merged_into_generation2' if stale else 'generation2_and_new_registration_addressed', old_generation2_route='native_done_broadcast')
    return row


def derive(p):
    scope = read(p/'actual/SCOPE.json.gz')
    slots = [dict(slot_id=f'{s}-{r}-{v}', schedule=s, candidate=r, variant=v, repetition=1) for s, rs in SCHEDULES.items() for r in rs for v in PROFILES]
    require(scope['source_commit'] == COMMIT and scope['native_timer_seconds'] == 60 and scope['planned_slots'] == slots and scope['automatic_retries'] == 0, 'native source/schedule or slot population changed')
    require(scope['independent_approved'] == scope['formal_completed'] == 0 and scope['freeze'] is None and scope['execution_allowed'] is False, 'acceptance boundary changed')
    # The adapter is the one stage 91 preflighted; its module bytes must not change.
    prior, stage92_packet = ROOT/previous.PACKET, ROOT/stage92.PACKET
    require(scope['adapter_module_sha256'] == sha(raw(prior/'drivers/ks_adapter.py.gz')) and raw(p/'drivers/ks_adapter.py.gz') == raw(prior/'drivers/ks_adapter.py.gz'), 'adapter differs from the preflighted one')
    preflight = read(prior/'adapter/PREFLIGHT.json')
    require(scope['adapter']['previous_locations'] == preflight['adapter']['previous_locations'], 'adapter location map differs')
    bridge, main = raw(p/'drivers/notify-bridge.go.gz'), raw(p/'drivers/notify-main.go.gz')
    require(sha(bridge) == scope['bridge_sha256'] and sha(main) == scope['main_sha256'] and b'.Post(' not in main, 'fixture bytes differ')
    instrument = raw(p/'drivers/transport_instrument.py.gz')
    require(scope['driver_sha256'] == sha(raw(p/'drivers/notify_run.py.gz')) and scope['instrumentation_sha256'] == sha(instrument), 'runtime driver identity differs')
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
            expected = ['${GOROOT}/bin/go', 'build']+(['-race'] if variant == 'race' else [])+['-o', binaries[role][variant]['path'], './cmd/ks-notify-generation']
            require(receipt['argv'] == expected and previous.PLACEHOLDER.match(binaries[role][variant]['path']), 'candidate build profile differs')
            metadata = raw(q/(variant+'-build-info.txt.gz')).decode()
            require('go1.23.12' in metadata and 'GOOS=darwin' in metadata and 'GOARCH=arm64' in metadata and 'vcs.revision='+COMMIT in metadata and (variant != 'race' or '-race=true' in metadata), 'actual native binary profile differs')
    attempts = read(p/'actual/attempts.json.gz')
    require(len(attempts) == len(slots) and [{k: r[k] for k in slot} for r, slot in zip(attempts, slots)] == slots, 'actual planned attempt population differs')
    rows = []
    for slot, attempt in zip(slots, attempts):
        q = p/'actual/slots'/slot['slot_id']
        require(read(q/'receipt.json.gz') == attempt and attempt['binary_sha256'] == binaries[slot['candidate']][slot['variant']]['sha256'] and attempt['exit_code'] == 0 and attempt['reaped'] and not any(attempt[k] for k in ('timeout', 'term_sent', 'kill_sent', 'remaining_live_group_members', 'tmp_children_after')), 'slot result/binary or cleanup differs')
        require(60 <= attempt['elapsed_seconds'] < 150, 'actual native runtime duration differs')
        proc = read(q/'process/receipt.json.gz')
        for stream in ('stdout', 'stderr'):
            require(sha(raw(q/('process/'+stream+'.gz'))) == proc[stream+'_sha256'], 'slot raw process stream differs')
        events = [json.loads(line) for line in raw(q/'process/stdout.gz').splitlines()]
        head = one(events, 'actual_receiver_head')['value']
        require(len([r for r in events if r['event'] == 'factory_started']) == 2 and one(events, 'actual_sync_start')['value'] is True and head['matches_seeder'] is True and head['number'] == 3, 'actual peer sync/head not observed')
        for name in ('actual_sync_done', 'actual_indexer_complete', 'old_generation_real_owner_ack_observed', 'generation_boundary_completed'):
            one(events, name)
        require(len([r for r in events if r['event'] == 'new_rpc_connection_attached']) == 1, 'new RPC connection after transport close differs')
        require(one(events, 'receiver_close_returned')['value'] is True and one(events, 'seeder_close_returned')['value'] is True and events[-1]['event'] == 'complete', 'real nodes did not close')
        trace = [json.loads(line) for line in raw(q/'events.jsonl.gz').splitlines()]
        result = inspect_trace(trace, slot['schedule'], slot['candidate'])
        rows.append(dict(**slot, **result))
    require(read(p/'actual/original-before.json.gz') == read(p/'actual/original-after.json.gz') and raw(p/'actual/original-status-before.txt.gz') == raw(p/'actual/original-status-after.txt.gz'), 'original upstream source changed')
    held = [r for r in rows if r['schedule'] == 'notify-in-flight']
    ungated = [r for r in rows if r['schedule'] == 'transport-ungated']
    routes = lambda group: {e: len([r for r in group if r['old_callback_exit'] == e]) for e in EXITS}
    return dict(kind='actual_factory_notify_in_flight_and_ungated_transport_generation_boundary_DEV_not_independent', planned_slots=len(slots), actual_slots=len(rows),
                actual_owned_nodes=2*len(rows), unique_build_roles=len(scope['unique_build_roles']), channel_sites_instrumented=sites, automatic_retries=0,
                callback_exit_routes={'notify-in-flight': routes(held), 'transport-ungated': routes(ungated)},
                notifier_closed_branch_reached=any(r['old_callback_exit'] == EXITS[1] for r in rows),
                same_channel_generation2_deliveries=len(rows), timer_broadcasts_to_generation2=len(ungated), new_rpc_registration_deliveries=len(rows),
                post_ack_same_generation_uses=sum(r['post_ack_same_generation_uses'] for r in rows),
                stale_membership_at_reregistration={role: len([r for r in ungated if r['candidate'] == role and r['stale_membership_at_reregistration']]) for role in SCHEDULES['transport-ungated']},
                stale_control_detected_by_channel_use=0, owned_status_receives=sum(r['owned_status_receives'] for r in rows),
                owned_manual_uninstalls=sum(r['owned_manual_uninstalls'] for r in rows), fake_acknowledgements=0,
                notify_in_flight_tested=True, ungated_transport_close_tested=True, progress_true_notification_tested=False,
                universal_post_ack_claim=False, full12_acceptance=False, independent_families=0, formal_completed=0, independent_approved=0, FINAL=0,
                freeze=None, execution_allowed=False, actual_LLM_calls=0, paid_calls=0, rows=rows)


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
    print('actual Notify-in-flight and ungated transport boundary16 slots; post-ack uses0; exit branches recorded; independent/formal0')
