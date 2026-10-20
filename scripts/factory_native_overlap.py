"""Recompute real-factory native broadcast/RPC cancellation observations."""
import gzip
import hashlib
import json
import re
from pathlib import Path
from ticket_actual_rpc import flat_map, tree_oid, stack_frame
from source_packet import oid

ROOT = Path(__file__).resolve().parents[1]
PACKET = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/factory-native-overlap-v1')
COMMIT = '0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'
ROLES = ['ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-REINTRO', 'MUT-EARLY']
TOOLS = ['scripts/factory_native_overlap.py', 'scripts/test_factory_native_overlap.py', 'scripts/ticket_actual_rpc.py', 'scripts/source_packet.py']


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def raw(p):
    data = p.read_bytes()
    return gzip.decompress(data) if p.suffix == '.gz' else data


def read(p):
    return json.loads(raw(p))


def require(condition, message):
    if not condition:
        raise ValueError(message)


def one(rows, event):
    matches = [r for r in rows if r['event'] == event]
    require(len(matches) == 1, 'missing/duplicate event: '+event)
    return matches[0]


def inspect_trace(rows, role):
    require([r['sequence'] for r in rows] == list(range(1, len(rows)+1)), 'trace sequence incomplete')
    require(all(a['elapsed_ns'] <= b['elapsed_ns'] for a, b in zip(rows, rows[1:])), 'trace clock order differs')
    require(len([r for r in rows if r['event'] == 'factory_API_created']) == 2, 'not exactly two factory APIs')
    selected = one(rows, 'selected_factory_subscription')
    api, channel, sid = selected['api_id'], selected['channel_id'], selected['subscription_id']
    require(api > 0 and channel > 0 and re.fullmatch(r'0x[0-9a-f]+', sid), 'factory/subscription identity missing')
    require(selected['value'] == dict(same_downloader=True, same_blockchain=True, same_mux=True, new_API_calls_from_probe=0, unbuffered_status_channel=True, subscription_id=sid), 'actual factory identity differs')
    local = [r for r in rows if r['api_id'] == api and r['channel_id'] == channel]
    require(all(r['subscription_id'] == sid for r in local), 'channel/RPC subscription identity changed')
    source_start = one([r for r in rows if r['api_id'] == api], 'source_StartEvent_received')
    timer = one([r for r in rows if r['api_id'] == api], 'native_timer_fired')
    native = one(local, 'native_send_begin_done')
    require(timer['api_age_ns'] >= 60_000_000_000 and source_start['sequence'] < timer['sequence'] < native['sequence'], 'original native60 timer/source event absent')
    install = one(local, 'install')
    gate = one(local, 'native_broadcast_gate_observed')
    cancel = one(local, 'actual_RPC_unsubscribe_true')
    exit_row = one(local, 'callback_exit_rpc_error')
    method = one(local, 'method_entry')
    release = one(local, 'native_gate_release')
    before_release = one(local, 'callback_exit_and_method_entered_before_gate_release')
    require(before_release['value'] == dict(callback_exit_rpc_error=1, method_entry=1, owner_delete=0, ack=0), 'pre-release callback/method boundary differs')
    frozen = one(local, 'observation_frozen')
    cleanup = one(local, 'cleanup_begin')
    end = one(local, 'cleanup_end')
    require(install['owner_map_observed'] and install['member'], 'actual owner installation absent')
    require(install['sequence'] < native['sequence'] < gate['sequence'] < cancel['sequence'] < release['sequence'] < frozen['sequence'] < cleanup['sequence'] < end['sequence'], 'gate/cancellation/observation/rescue boundary differs')
    require(exit_row['sequence'] < method['sequence'] < before_release['sequence'] < release['sequence'] and not any(r['event'] == 'callback_exit_transport_closed' for r in local), 'callback was not the actual RPC Err route before gate release')
    require(gate['value'] == dict(native_timer_seconds=60, payload=False, send_not_yet_offered=True, declared_schedule_gate=True), 'undeclared/shortened native send schedule')
    require(cancel['value'] == dict(subscription_id=sid, same_client_session=True, downstream_complete_not_inferred=True), 'RPC true promoted to downstream completion')
    fixed, early = role in ('CAND-A', 'CAND-B'), role == 'MUT-EARLY'
    expected = dict(native_send_begin=1, native_send_end=int(fixed), callback_exit_rpc_error=1, callback_exit_transport_closed=0, callback_returned=int(fixed or early), method_entry=1, method_return=int(fixed or early), method_status_received=int(fixed), method_ack_observed=int(fixed), owner_delete=int(fixed), owner_ack=int(fixed), owner_member=not fixed, rescue_started=False)
    require(frozen['value'] == expected, 'pre-rescue candidate outcome differs')
    delete = one(local, 'after_delete')
    before_delete = one(local, 'before_delete')
    ack_begin = one(local, 'ack_close_begin')
    ack_end = one(local, 'ack_close_end')
    returned = one(local, 'method_return')
    callback = one(local, 'callback_returned')
    send_end = one(local, 'native_send_end_done')
    require(send_end['sequence'] < before_delete['sequence'] < delete['sequence'] < ack_begin['sequence'] < ack_end['sequence'], 'real owner send/delete/ack order differs')
    require(before_delete['owner_map_observed'] and before_delete['member'] and all(r['owner_map_observed'] and not r['member'] for r in (delete, ack_begin, ack_end)), 'real owner membership/delete not observed')
    rescues = [r for r in local if r['event'] == 'owned_rescue_status_receive_begin']
    manual = [r for r in local if r['event'] == 'owned_rescue_manual_uninstall_begin']
    require(len(rescues) == int(not fixed) and len(manual) == int(early), 'cleanup interventions omitted/promoted')
    require(all(r['sequence'] > frozen['sequence'] for r in rescues+manual), 'rescue started before frozen observation')
    if fixed:
        drain = one(local, 'method_status_received')
        observed_ack = one(local, 'method_ack_observed')
        require(release['sequence'] < drain['sequence'] < before_delete['sequence'] and ack_begin['sequence'] < observed_ack['sequence'] < returned['sequence'] < callback['sequence'] < frozen['sequence'], 'candidate did not drain/await actual owner ack before returning')
    elif early:
        require(returned['sequence'] < callback['sequence'] < frozen['sequence'] < delete['sequence'], 'early return control not distinguished')
    else:
        require(frozen['sequence'] < delete['sequence'] < ack_begin['sequence'] < one(local, 'method_ack_observed')['sequence'] < returned['sequence'] < callback['sequence'], 'original rescue incorrectly counted as success')
    require(end['value'] == dict(rescue_used=not fixed, owner_member=False, callback_returned=1, method_return=1, real_owner_delete=1, real_owner_ack=1, fake_acknowledgements=0), 'owned cleanup or actual acknowledgement differs')
    return dict(outcome='drained_native_and_real_owner_ack' if fixed else 'early_return_without_owner_delete_ack' if early else 'paired_native_and_uninstall_sends_before_rescue', api_id=api, channel_id=channel, subscription_id_sha256=sha(sid.encode()), native_age_ns=timer['api_age_ns'], counts_before_rescue=expected, rescue_status_receive=not fixed, rescue_manual_uninstall=early)


def derive(p):
    scope = read(p/'actual/SCOPE.json.gz')
    require(scope['source_commit'] == COMMIT and scope['native_timer_seconds'] == 60 and scope['actual_source_sync_not_injected'] and scope['cancellation_route'] == 'same_session_eth_unsubscribe_true_not_connection_close' and scope['original_five_or_full12_acceptance'] is False, 'native source/schedule changed')
    slots = [dict(slot_id=r+'-'+v, candidate=r, variant=v, repetition=1) for r in ROLES for v in ('general', 'race')]
    require(scope['planned_slots'] == slots and scope['automatic_retries'] == 0 and scope['independent_approved'] == scope['formal_completed'] == 0 and scope['freeze'] is None and scope['execution_allowed'] is False, 'slot population/acceptance boundary changed')
    initial = read(p/'initial/candidates/ORIGINAL/build-general/receipt.json.gz')
    require(initial['exit_code'] == 1 and initial['reaped'] and not any(initial[k] for k in ('timeout', 'remaining_live_group_members')), 'initial compile failure omitted or promoted')
    require('as chan interface{} value in struct literal' in raw(p/'initial/candidates/ORIGINAL/build-general/stderr.gz').decode(), 'initial failure classification differs')
    bridge = raw(p/'drivers/phase90-native-rpc-bridge-corrective1.go.gz')
    main = raw(p/'drivers/phase90-native-rpc-main.go.gz')
    initial_bridge = raw(p/'drivers/phase90-native-rpc-bridge-initial.go.gz')
    require(initial_bridge.count(b'uninstalled: make(chan struct{})') == 1 and bridge == initial_bridge.replace(b'uninstalled: make(chan struct{})', b'uninstalled: make(chan interface{})'), 'corrective fixture changed beyond declared ack channel type')
    require(scope['driver_sha256'] == sha(raw(p/'drivers/phase90-run-corrective1.py.gz')) and scope['instrumentation_sha256'] == sha(raw(p/'drivers/phase90-instrument.py.gz')) and raw(p/'drivers/phase90-instrument.py.gz') == raw(p/'drivers/phase90_instrument.py.gz'), 'effective runtime driver/instrumentation identity differs')
    require(read(p/'initial/original-before.json.gz') == read(p/'actual/original-before.json.gz'), 'original source changed across invalid compile/corrective attempt')
    require(sha(bridge) == scope['bridge_sha256'] and sha(main) == scope['main_sha256'] and b'.Post(' not in main and b'syncStatsChainHeight' not in bridge, 'fixture/source sync or declared source bytes differ')
    baseline = flat_map(json.loads((ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/ticket-method-schedules-v1/actual/candidates/ORIGINAL/base-flat-tree.json').read_bytes()))
    binaries = {}
    for role in scope['unique_build_roles']:
        q = p/'actual/candidates'/role
        b, t = read(q/'source-binding.json.gz'), read(q/'tree-binding.json.gz')
        api = raw(q/'instrumented-api.go.gz')
        overlay = (ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/private/overlays'/(role+'.go.txt')).read_bytes()
        require(b['source_commit'] == COMMIT and b['production_patch'] is False and b['retained_api_sha256'] == sha(overlay) and b['instrumented_api_sha256'] == sha(api), 'candidate source scope differs')
        require(b'checkInterval     = time.Second * 60' in api, 'native timer constant changed')
        original = read(p/'actual/original-before.json.gz')
        expected_applied = dict(original)
        expected_applied['eth/downloader/api.go'] = dict(mode=original['eth/downloader/api.go']['mode'], sha256=sha(api))
        require(b['base_tracked'] == original and b['applied_tracked'] == expected_applied, 'source map changed beyond declared API hooks')
        project = dict(baseline); project['eth/downloader/api.go'] = dict(mode='100644', kind='blob', oid=oid('blob', api))
        build_tree = dict(project)
        additions = {'eth/downloader/ks_native_rpc_overlap.go': bridge, 'cmd/ks-native-rpc-overlap/main.go': main}
        build_tree.update({n: dict(mode='100644', kind='blob', oid=oid('blob', data)) for n, data in additions.items()})
        require(b['additions'] == {n: sha(data) for n, data in additions.items()} and flat_map(t['project_flat_tree']) == project and flat_map(t['build_flat_tree']) == build_tree and tree_oid(t['project_flat_tree']) == t['applied_project_tree'] and tree_oid(t['build_flat_tree']) == t['build_input_tree'], 'full project/build Git tree differs')
        binaries[role] = read(q/'binaries.json.gz')
        for variant in ('general', 'race'):
            receipt = read(q/('build-'+variant)/'receipt.json.gz')
            require(receipt['exit_code'] == 0 and receipt['reaped'] and not any(receipt[k] for k in ('timeout', 'remaining_live_group_members')), 'candidate build failed')
            expected = ['/Users/kevin/.gvm/gos/go1.23.12/bin/go', 'build']+(['-race'] if variant == 'race' else [])+['-o', binaries[role][variant]['path'], './cmd/ks-native-rpc-overlap']
            require(receipt['argv'] == expected, 'candidate build profile differs')
            metadata = raw(q/(variant+'-build-info.txt.gz')).decode()
            require('go1.23.12' in metadata and 'GOOS=darwin' in metadata and 'GOARCH=arm64' in metadata and 'vcs.revision='+COMMIT in metadata and (variant != 'race' or '-race=true' in metadata), 'actual native binary profile differs')
    binaries['MUT-REINTRO'] = binaries['ORIGINAL']
    attempts = read(p/'actual/attempts.json.gz')
    require([{k: r[k] for k in slot} for r, slot in zip(attempts, slots)] == slots and len(attempts) == 10, 'actual planned attempt population differs')
    rows = []
    for slot, attempt in zip(slots, attempts):
        q = p/'actual/slots'/slot['slot_id']
        require(read(q/'receipt.json.gz') == attempt and attempt['binary_sha256'] == binaries[slot['candidate']][slot['variant']]['sha256'] and attempt['exit_code'] == 0 and attempt['reaped'] and not any(attempt[k] for k in ('timeout', 'term_sent', 'kill_sent', 'remaining_live_group_members', 'tmp_children_after')), 'slot result/binary or cleanup differs')
        require(60 <= attempt['elapsed_seconds'] < 100, 'actual native runtime duration differs')
        proc = read(q/'process/receipt.json.gz')
        for stream in ('stdout', 'stderr'):
            require(sha(raw(q/('process/'+stream+'.gz'))) == proc[stream+'_sha256'], 'slot raw process stream differs')
        main_events = [json.loads(line) for line in raw(q/'process/stdout.gz').splitlines()]
        require(len([r for r in main_events if r['event'] == 'factory_started']) == 2 and one(main_events, 'actual_sync_start')['value'] is True and one(main_events, 'actual_receiver_head')['value']['matches_seeder'] is True and one(main_events, 'actual_receiver_head')['value']['number'] == 3, 'actual peer sync/head not observed')
        one(main_events, 'actual_sync_done'); one(main_events, 'actual_indexer_complete')
        require(one(main_events, 'receiver_close_returned')['value'] is True and one(main_events, 'seeder_close_returned')['value'] is True and main_events[-1]['event'] == 'complete', 'real nodes did not close')
        events = [json.loads(line) for line in raw(q/'events.jsonl.gz').splitlines()]
        result = inspect_trace(events, slot['candidate'])
        actual_role = 'ORIGINAL' if slot['candidate'] == 'MUT-REINTRO' else slot['candidate']
        source = raw(p/'actual/candidates'/actual_role/'instrumented-api.go.gz').decode().splitlines()
        line = next(i+1 for i, text in enumerate(source) if text.strip() == 'c <- false')
        stack = raw(q/'events.jsonl.before-rescue.stack.gz').decode()
        if slot['candidate'] not in ('CAND-A', 'CAND-B'):
            require(stack_frame(stack, '(*DownloaderAPI).eventLoop', '/eth/downloader/api.go:'+str(line), '[chan send'), 'pending actual native send stack missing')
        if slot['candidate'] in ('ORIGINAL', 'MUT-REINTRO'):
            line = next(i+1 for i, text in enumerate(source) if text.strip() == 's.api.uninstallSyncSubscription <- &req')
            require(stack_frame(stack, '(*SyncStatusSubscription).Unsubscribe', '(*DownloaderAPI).Syncing.func1', '/eth/downloader/api.go:'+str(line), '[chan send'), 'pending actual method uninstall stack missing')
        rows.append(dict(**slot, **result))
    require(read(p/'actual/original-before.json.gz') == read(p/'actual/original-after.json.gz') and raw(p/'actual/original-status-before.txt.gz') == raw(p/'actual/original-status-after.txt.gz'), 'original upstream source changed')
    return dict(kind='actual_factory_peer_native60_broadcast_RPC_callback_overlap_DEV_not_independent',planned_slots=10,actual_slots=10,actual_owned_nodes=20,actual_peer_syncs=10,unique_candidate_source_variants=4,initial_fixture_compile_failures=1,explicit_corrective_attempts=1,automatic_retries=0,paired_send_observations=4,drain_real_owner_ack_observations=4,early_return_observations=2,owned_status_drain_rescues=6,owned_manual_real_owner_uninstalls=2,fake_acknowledgements=0,gate_controls_schedule=True,transport_close_callback_route_tested=False,all_post_ack_channel_use_proved=False,full12_acceptance=False,installed_positive_independent_oracle=False,independent_families=0,formal_completed=0,independent_approved=0,FINAL=0,freeze=None,execution_allowed=False,actual_LLM_calls=0,paid_calls=0,rows=rows)


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
    print('actual factory native/RPC overlap10 slots; owner ack4, paired4, early2; independent/formal0')
