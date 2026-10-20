"""Recompute post-ack status-channel use and subscription generation boundaries."""
import gzip
import hashlib
import json
import re
from pathlib import Path
from ticket_actual_rpc import flat_map, tree_oid, stack_frame
from source_packet import oid

ROOT = Path(__file__).resolve().parents[1]
FOLLOWUPS = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups')
PACKET = FOLLOWUPS/'wiv1-generation-boundary-v1'
COMMIT = '0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'
SCHEDULES = {'overlap': ['ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-EARLY'], 'timer': ['ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-STALE-ACK']}
PROFILES = ('general', 'race')
USES = {'install_send_begin', 'native_send_begin_progress', 'native_send_begin_done', 'method_status_received', 'callback_status_received', 'probe_status_received'}
API, BRIDGE, MAIN = 'eth/downloader/api.go', 'eth/downloader/ks_generation_boundary.go', 'cmd/ks-generation-boundary/main.go'
STALE_DELETE = b'\t\t\tdelete(syncSubscriptions, u.c)\n'
SITE = re.compile(r'(?:\bi <- false|\bc <- (?:false|notification)|<-statuses|<-s\.c\b|installSyncSubscription <- status|'
                  r'uninstallSyncSubscription <- &req|send <- &req|<-req\.uninstalled|close\(u\.uninstalled\))')
TOOLS = ['scripts/wiv1_generation_boundary.py', 'scripts/test_wiv1_generation_boundary.py', 'scripts/ticket_actual_rpc.py', 'scripts/source_packet.py']
PLACEHOLDER = re.compile(r'\$\{(WORK_ROOT|UPSTREAM_SOURCE|GOMODCACHE|GOROOT|KS_ROOT|HOME)\}')
LIVE_PATH = re.compile(rb'/(?:Users|home|private/tmp|tmp|var/folders)/[^\s"\']+')


def sha(data):
    return hashlib.sha256(data).hexdigest()


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


def overlay(role):
    base = ROOT/FOLLOWUPS/'ticket-dev-gold-v1/private/overlays'
    if role == 'MUT-STALE-ACK':
        source = (base/'CAND-A.go.txt').read_bytes()
        require(source.count(STALE_DELETE) == 1, 'stale-ack control derivation anchor changed')
        return source.replace(STALE_DELETE, b'')
    return (base/(role+'.go.txt')).read_bytes()


def sites_covered(retained, api):
    lines = api.decode().splitlines()
    for site in [x.strip() for x in retained.decode().splitlines() if SITE.search(x)]:
        hits = [i for i, text in enumerate(lines) if text.strip() == site]
        require(hits and all(any('ksNative' in lines[j] for j in (i-1, i+1) if 0 <= j < len(lines)) for i in hits), 'status channel site not instrumented: '+site)
    return len([x for x in retained.decode().splitlines() if SITE.search(x)])


def generations(rows, api):
    """Recompute generation and post-ack flags from install/ack events alone."""
    gen, acked, uses = {}, {}, {}
    for r in rows:
        if r['api_id'] != api or not r['channel_id']:
            continue
        c = r['channel_id']
        if r['event'] == 'install':
            gen[c] = gen.get(c, 0)+1
        require(r['generation'] == gen.get(c, 0), 'recorded generation differs from install count')
        if r['event'] == 'ack_close_end':
            acked[c] = gen.get(c, 0)
        flag = r['event'] in USES and acked.get(c, 0) > 0 and acked[c] == gen.get(c, 0)
        require(r['post_ack_same_generation_use'] == flag and r['channel_use'] == (r['event'] in USES), 'recorded post-ack use flag differs')
        if r['event'] in USES:
            uses.setdefault(c, []).append(dict(event=r['event'], generation=r['generation'], post_ack=flag))
    return gen, acked, uses


def count(rows, channel, event, generation=None):
    return len([r for r in rows if r['channel_id'] == channel and r['event'] == event and (generation is None or r['generation'] == generation)])


def inspect_trace(rows, schedule, role):
    require([r['sequence'] for r in rows] == list(range(1, len(rows)+1)), 'trace sequence incomplete')
    require(all(a['elapsed_ns'] <= b['elapsed_ns'] for a, b in zip(rows, rows[1:])), 'trace clock order differs')
    require(len([r for r in rows if r['event'] == 'factory_API_created']) == 2, 'not exactly two factory APIs')
    selected = one(rows, 'selected_factory_subscription')
    api, old, sid = selected['api_id'], selected['channel_id'], selected['subscription_id']
    require(api > 0 and old > 0 and re.fullmatch(r'0x[0-9a-f]+', sid), 'factory/subscription identity missing')
    require(selected['value'] == dict(same_downloader=True, same_blockchain=True, same_mux=True, unbuffered_status_channel=True, subscription_id=sid, generation=1), 'actual factory identity differs')
    local = [r for r in rows if r['api_id'] == api]
    gen, acked, uses = generations(rows, api)
    start = one(local, 'source_StartEvent_received')
    fired = [r for r in local if r['event'] == 'native_timer_fired']
    broadcast = [r for r in local if r['event'] == 'native_send_begin_done']
    require(broadcast and fired and fired[-1]['api_age_ns'] >= 60_000_000_000 and start['sequence'] < fired[-1]['sequence'] < broadcast[0]['sequence'], 'original native60 timer/source event absent')
    require(not any(r['event'] == 'callback_exit_transport_closed' for r in local), 'transport close route appeared')
    cancel = one(local, 'actual_RPC_unsubscribe_true')
    require(cancel['channel_id'] == old and cancel['value'] == dict(subscription_id=sid, owner_ack_not_inferred=True), 'RPC true promoted to owner acknowledgement')
    old_ack = [r for r in local if r['channel_id'] == old and r['event'] == 'ack_close_end' and r['generation'] == 1]
    require(len(old_ack) == 1, 'old generation real owner ack missing')
    regen = one(local, 'regeneration_begin')
    require(old_ack[0]['sequence'] < regen['sequence'], 'regeneration began before old real owner ack')
    fresh_rows = [r for r in local if r['event'] == 'new_rpc_registration_installed']
    require(len(fresh_rows) == 1 and fresh_rows[0]['value'] == dict(generation=1, distinct_channel=True) and fresh_rows[0]['channel_id'] != old, 'new RPC registration not a distinct channel')
    fresh = fresh_rows[0]['channel_id']
    require(regen['sequence'] < one([r for r in local if r['channel_id'] == fresh], 'install')['sequence'], 'new registration installed before regeneration')
    client = one(local, 'client_notification_received')
    require(client['channel_id'] == fresh and client['value'] == 'false', 'new registration wire delivery differs')
    fresh_ack = [r for r in local if r['channel_id'] == fresh and r['event'] == 'ack_close_end']
    require(len(fresh_ack) == 1 and client['sequence'] < fresh_ack[0]['sequence'], 'new registration real owner ack missing')
    require(count(local, fresh, 'callback_status_received') == 1 and count(local, fresh, 'callback_notify_end') == 1, 'new registration callback delivery differs')
    rescues = [r for r in local if r['event'] == 'owned_rescue_status_receive_begin']
    manual = [r for r in local if r['event'] == 'owned_rescue_manual_uninstall_begin']
    violations = [u for c in uses for u in uses[c] if u['post_ack']]
    early = role == 'MUT-EARLY'
    fixed = role in ('CAND-A', 'CAND-B')
    summary = one(local, 'generation_summary')['value']
    require({r['channel_id']: r['post_ack_same_generation_uses'] for r in summary} == {c: len([u for u in uses.get(c, []) if u['post_ack']]) for c in {r['channel_id'] for r in summary}}, 'generation summary differs from trace')
    row = dict(api_id=api, old_channel_id=old, new_channel_id=fresh, subscription_id_sha256=sha(sid.encode()), native_age_ns=fired[-1]['api_age_ns'],
               rpc_true_before_old_ack=cancel['sequence'] < old_ack[0]['sequence'], post_ack_same_generation_uses=len(violations),
               owned_status_receives=len(rescues), owned_manual_uninstalls=len(manual), fake_acknowledgements=0)
    if schedule == 'overlap':
        frozen = one(local, 'overlap_observation_frozen')
        require(isinstance(frozen['value'], dict) and isinstance(frozen['value'].get('counts'), dict), 'frozen observation payload missing')
        counts = frozen['value']['counts']
        expected = dict(native_send_begin_done=1, native_send_end_done=int(fixed), callback_exit_rpc_error=1, callback_returned=int(fixed or early), method_entry=1,
                        method_return=int(fixed or early), method_status_received=int(fixed), method_ack_observed=int(fixed), after_delete=int(fixed), ack_close_end=int(fixed))
        require({k: counts.get(k, 0) for k in expected} == expected and frozen['value']['member'] is (not fixed), 'pre-rescue candidate outcome differs')
        require(all(r['sequence'] > frozen['sequence'] for r in rescues+manual), 'rescue started before frozen observation')
        require(gen.get(old) == 2 and acked.get(old) == 2 and count(local, old, 'install', 2) == 1 and count(local, old, 'install_send_begin', 2) == 1 and count(local, old, 'probe_status_received', 2) == 1, 'same-channel generation 2 not delivered and acknowledged')
        require(one(local, 'same_channel_registration_delivered')['sequence'] < [r for r in local if r['channel_id'] == old and r['event'] == 'ack_close_end' and r['generation'] == 2][0]['sequence'], 'generation 2 ack precedes delivery')
        require(count(local, fresh, 'install_send_begin', 1) == 1 and count(local, fresh, 'native_send_begin_done') == 0, 'new registration not served by install send')
        require(len(rescues) == int(not fixed) and len(manual) == 3*int(early), 'cleanup interventions omitted/promoted')
        require(not violations, 'post-ack same-generation use observed')
        row.update(outcome='drained_then_regenerated' if fixed else 'early_return_rescued_then_regenerated' if early else 'paired_send_rescued_then_regenerated',
                   regeneration_after_rescue=not fixed, same_channel_generation2_delivered=True)
    else:
        require(old_ack[0]['sequence'] < fired[-1]['sequence'], 'old generation not acknowledged before native timer')
        stale = role == 'MUT-STALE-ACK'
        old_sends = [r for r in local if r['channel_id'] == old and r['event'] == 'native_send_begin_done']
        require(count(local, fresh, 'native_send_begin_done', 1) == 1 and count(local, fresh, 'install_send_begin') == 0, 'natural broadcast to new generation differs')
        require(count(local, old, 'after_delete', 1) == int(not stale) and old_ack[0]['member'] is stale, 'old generation owner delete/membership differs')
        if stale:
            require(len(old_sends) == 1 and old_sends[0]['post_ack_same_generation_use'] and old_sends[0]['generation'] == 1, 'stale generation not addressed after ack')
            require(len(violations) == 1 and len(rescues) == 1 and not manual and rescues[0]['sequence'] > one(local, 'timer_observation_frozen')['sequence'], 'violation control rescue differs')
        else:
            require(not old_sends and not violations and not rescues and not manual, 'post-ack old-generation use or rescue observed')
        row.update(outcome='stale_generation_addressed_after_ack' if stale else 'only_new_generation_addressed', regeneration_after_rescue=False, same_channel_generation2_delivered=False)
    return row


def derive(p):
    scope = read(p/'actual/SCOPE.json.gz')
    slots = [dict(slot_id=f'{s}-{r}-{v}', schedule=s, candidate=r, variant=v, repetition=1) for s, rs in SCHEDULES.items() for r in rs for v in PROFILES]
    require(scope['source_commit'] == COMMIT and scope['native_timer_seconds'] == 60 and scope['planned_slots'] == slots and scope['automatic_retries'] == 0, 'native source/schedule or slot population changed')
    require(scope['independent_approved'] == scope['formal_completed'] == 0 and scope['freeze'] is None and scope['execution_allowed'] is False, 'acceptance boundary changed')
    preflight = read(p/'adapter/PREFLIGHT.json')
    require(preflight['ok'] and preflight['nodes_started'] == 0 and all(c['ok'] for c in preflight['checks'].values()), 'adapter preflight not passed')
    require(all(preflight['checks']['phase90_inputs_reproduced']['roles'][r]['ok'] for r in ('ORIGINAL', 'CAND-A', 'CAND-B', 'MUT-EARLY')), 'retained phase-90 inputs not reproduced')
    bridge, main = raw(p/'drivers/generation-bridge.go.gz'), raw(p/'drivers/generation-main.go.gz')
    require(sha(bridge) == scope['bridge_sha256'] and sha(main) == scope['main_sha256'] and b'.Post(' not in main, 'fixture bytes differ')
    require(scope['driver_sha256'] == sha(raw(p/'drivers/generation_run.py.gz')) and scope['instrumentation_sha256'] == sha(raw(p/'drivers/generation_instrument.py.gz')), 'runtime driver identity differs')
    # Previous-machine locations are recorded on purpose in adapter/; run outputs must hold placeholders only.
    previous = [r['previous'].encode() for r in preflight['adapter']['previous_locations']]
    require(scope['adapter']['previous_locations'] == preflight['adapter']['previous_locations'], 'adapter location map differs')
    for name in sorted([*(p/'actual').rglob('*'), *(p/'checks').rglob('*')]):
        if name.is_file():
            data = raw(name)
            for location in previous:
                data = data.replace(location, b'')
            require(not LIVE_PATH.search(data), 'live machine path archived: '+str(name.relative_to(p)))
    baseline = flat_map(json.loads((ROOT/FOLLOWUPS/'ticket-method-schedules-v1/actual/candidates/ORIGINAL/base-flat-tree.json').read_bytes()))
    binaries = {}
    sites = {}
    for role in scope['unique_build_roles']:
        q = p/'actual/candidates'/role
        b, t = read(q/'source-binding.json.gz'), read(q/'tree-binding.json.gz')
        api = raw(q/'instrumented-api.go.gz')
        retained = overlay(role)
        require(b['source_commit'] == COMMIT and b['production_patch'] is False and b['base_tracked_matches_retained'] is True and b['retained_api_sha256'] == sha(retained) and b['applied_api_sha256'] == sha(api), 'candidate source scope differs')
        require(b'checkInterval     = time.Second * 60' in api, 'native timer constant changed')
        sites[role] = sites_covered(retained, api)
        project = dict(baseline)
        project[API] = dict(mode='100644', kind='blob', oid=oid('blob', api))
        build_tree = dict(project)
        build_tree.update({n: dict(mode='100644', kind='blob', oid=oid('blob', d)) for n, d in ((BRIDGE, bridge), (MAIN, main))})
        require(b['additions'] == {BRIDGE: sha(bridge), MAIN: sha(main)} and flat_map(t['project_flat_tree']) == project and flat_map(t['build_flat_tree']) == build_tree and tree_oid(t['project_flat_tree']) == t['applied_project_tree'] and tree_oid(t['build_flat_tree']) == t['build_input_tree'], 'full project/build Git tree differs')
        binaries[role] = read(q/'binaries.json.gz')
        for variant in PROFILES:
            receipt = read(q/('build-'+variant)/'receipt.json.gz')
            require(receipt['exit_code'] == 0 and receipt['reaped'] and not any(receipt[k] for k in ('timeout', 'remaining_live_group_members')), 'candidate build failed')
            expected = ['${GOROOT}/bin/go', 'build']+(['-race'] if variant == 'race' else [])+['-o', binaries[role][variant]['path'], './cmd/ks-generation-boundary']
            require(receipt['argv'] == expected and PLACEHOLDER.match(binaries[role][variant]['path']), 'candidate build profile differs')
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
        one(events, 'actual_sync_done')
        one(events, 'actual_indexer_complete')
        one(events, 'old_generation_real_owner_ack_observed')
        one(events, 'generation_boundary_completed')
        require(one(events, 'receiver_close_returned')['value'] is True and one(events, 'seeder_close_returned')['value'] is True and events[-1]['event'] == 'complete', 'real nodes did not close')
        trace = [json.loads(line) for line in raw(q/'events.jsonl.gz').splitlines()]
        result = inspect_trace(trace, slot['schedule'], slot['candidate'])
        source = raw(p/'actual/candidates'/slot['candidate']/'instrumented-api.go.gz').decode().splitlines()
        send_line = next(i+1 for i, text in enumerate(source) if text.strip() == 'c <- false')
        if slot['schedule'] == 'overlap' and slot['candidate'] in ('ORIGINAL', 'MUT-EARLY'):
            require(stack_frame(raw(q/'events.jsonl.before-rescue.stack.gz').decode(), '(*DownloaderAPI).eventLoop', '/eth/downloader/api.go:'+str(send_line), '[chan send'), 'pending actual native send stack missing')
        if slot['candidate'] == 'MUT-STALE-ACK':
            require(stack_frame(raw(q/'events.jsonl.timer-broadcast.stack.gz').decode(), '(*DownloaderAPI).eventLoop', '/eth/downloader/api.go:'+str(send_line), '[chan send'), 'stale-generation pending send stack missing')
        rows.append(dict(**slot, **result))
    require(read(p/'actual/original-before.json.gz') == read(p/'actual/original-after.json.gz') and raw(p/'actual/original-status-before.txt.gz') == raw(p/'actual/original-status-after.txt.gz'), 'original upstream source changed')
    violation_rows = [r for r in rows if r['post_ack_same_generation_uses']]
    return dict(kind='actual_factory_post_ack_channel_use_and_generation_boundary_DEV_not_independent', planned_slots=len(slots), actual_slots=len(rows),
                actual_owned_nodes=2*len(rows), unique_build_roles=len(scope['unique_build_roles']), channel_sites_instrumented=sites,
                automatic_retries=0, overlap_regenerations=len([r for r in rows if r['schedule'] == 'overlap']),
                regenerations_after_rescue=len([r for r in rows if r['regeneration_after_rescue']]),
                same_channel_generation2_deliveries=len([r for r in rows if r['same_channel_generation2_delivered']]),
                new_rpc_registration_deliveries=len(rows), post_ack_same_generation_uses_non_control=sum(r['post_ack_same_generation_uses'] for r in rows if r['candidate'] != 'MUT-STALE-ACK'),
                violation_control_detections=len([r for r in violation_rows if r['candidate'] == 'MUT-STALE-ACK']),
                rpc_true_before_old_ack=len([r for r in rows if r['rpc_true_before_old_ack']]),
                owned_status_receives=sum(r['owned_status_receives'] for r in rows), owned_manual_uninstalls=sum(r['owned_manual_uninstalls'] for r in rows),
                fake_acknowledgements=0, gate_controls_overlap_schedule=True, transport_close_callback_route_tested=False,
                same_channel_reuse_on_timer_broadcast_tested=False, all_post_ack_channel_use_in_api_sites=True, universal_post_ack_claim=False,
                full12_acceptance=False, independent_families=0, formal_completed=0, independent_approved=0, FINAL=0, freeze=None, execution_allowed=False,
                actual_LLM_calls=0, paid_calls=0, rows=rows)


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
    print('actual post-ack generation boundary16 slots; non-control post-ack uses0, stale control detected; independent/formal0')
