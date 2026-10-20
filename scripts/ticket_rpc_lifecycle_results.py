"""Finite source-hook lifecycle observations; supplemental, never acceptance."""
import argparse
import gzip
import json
import re
import shutil
import subprocess
from pathlib import Path

import ticket_rpc_overlap as prior
from lifecycle_instrumentation import instrument
from lifecycle_trace_contract import validate_prefix
from source_packet import oid, verify

ROOT, BASE = prior.ROOT, prior.BASE
PREPARATION = BASE / 'followups/ticket-rpc-lifecycle-v1'
PACKET = BASE / 'followups/ticket-rpc-lifecycle-results-v1'
REVISION = 'f5d7da318e92f3dbbb024f396895b99033d1383d'
KIND = 'WIV1_V2_finite_instrumented_lifecycle_supplement_not_independent_acceptance'
API = 'eth/downloader/api.go'
BRIDGE = 'eth/downloader/ks_owned_rpc_lifecycle.go'
MAIN = 'cmd/ks-owned-rpc-lifecycle/main.go'
GO = '/Users/kevin/.gvm/gos/go1.23.12/bin/go'
SCOPE_SHA = 'cfe5f9e673c49e63a0085f46a888b33db831b6502695dc59500998199a079dee'
TOOLS = ['scripts/' + s for s in ('ticket_rpc_lifecycle_results.py', 'test_ticket_rpc_lifecycle_results.py',
         'lifecycle_instrumentation.py', 'lifecycle_trace_contract.py', 'run-ticket-rpc-lifecycle.py',
         'fixtures/ticket-rpc-lifecycle.go.fixture', 'fixtures/ticket-rpc-lifecycle-main.go.fixture',
         'run-wi4-target-subscription.py', 'source_packet.py')]
read, write, sha, raw = prior.read, prior.write, prior.sha, prior.raw
EVENTS = ['trace_begin', 'actual_RPC_subscription', 'test_owned_StartEvent',
          'first_native_write_held', 'second_native_broadcast_blocked', 'lifecycle_fence',
          'actual_server_connection_closed', 'write_gate_released', 'observation_frozen',
          'lifecycle_fence', 'post_ack_native_probe', 'lifecycle_fence', 'cleanup_begin',
          'cleanup_end', 'trace_end']


def sources(root, packet, scope):
    actual = packet / 'actual'
    expected = read(root / BASE / 'followups/wi4-target-subscription-v1/initial/source-binding.json')['tracked']
    candidates = {r['id']: r for r in read(root / BASE / 'followups/ticket-dev-gold-v1/private/candidates.json')}
    proof = verify(root / BASE / 'schedule-candidates-v1', prior.COMMIT)
    commit = (root / BASE / 'schedule-candidates-v1' / proof['objects']['commit:' + prior.COMMIT]).read_bytes()
    base_tree = commit.splitlines()[0].decode().removeprefix('tree ')
    fixture = (root / 'scripts/fixtures/ticket-rpc-lifecycle.go.fixture').read_bytes()
    main = (root / 'scripts/fixtures/ticket-rpc-lifecycle-main.go.fixture').read_bytes()
    if sha(fixture) != scope['fixture_sha256'] or sha(main) != scope['main_fixture_sha256']:
        raise ValueError('frozen lifecycle fixtures changed')
    post = read(actual / 'binary-postrun-check.json')
    if set(post) != set(scope['candidate_roles']):
        raise ValueError('postrun candidate population differs')
    binaries = {}
    for role in scope['candidate_roles']:
        q = actual / 'candidates' / role
        retained = (root / BASE / 'followups/ticket-dev-gold-v1/private/overlays' / (role + '.go.txt')).read_bytes()
        overlay, anchors = instrument(retained)
        if sha(retained) != candidates[role]['full_file_overlay_sha256'] or anchors != scope['source_anchors'][role] or sha(overlay) != scope['instrumented_api_sha256'][role]:
            raise ValueError('retained/instrumented candidate differs')
        applied = dict(expected)
        applied[API] = dict(mode=expected[API]['mode'], sha256=sha(overlay))
        binding = dict(candidate_id=role, base_commit=prior.COMMIT, base_tracked=expected,
                       applied_tracked=applied, api_sha256=sha(overlay), retained_api_sha256=sha(retained),
                       method_sha256=candidates[role]['method_sha256'], instrumented_api=True,
                       source_anchors=anchors, fixture_sha256=sha(fixture), main_fixture_sha256=sha(main),
                       additions=[BRIDGE, MAIN], production_patch=False,
                       original_test_init_preserved_in_source_but_not_executed=True)
        if read(q / 'source-binding.json') != binding:
            raise ValueError('source binding differs')
        flat = read(q / 'base-flat-tree.json')
        baseline = prior.flat_map(flat)
        if set(baseline) != set(expected) or len(flat) != 2228 or prior.tree_oid(flat) != base_tree:
            raise ValueError('complete original Git tree differs')
        project = dict(baseline)
        project[API] = dict(mode='100644', kind='blob', oid=oid('blob', overlay))
        build = dict(project)
        for name, data in [(BRIDGE, fixture), (MAIN, main)]:
            build[name] = dict(mode='100644', kind='blob', oid=oid('blob', data))
        tree = read(q / 'tree-binding.json')
        if (tree['base_commit'] != prior.COMMIT or tree['applied_project_commit'] is not None
                or tree['source_mode'] != 'owned_working_tree_real_Git_trees_plus_two_program_additions_observation_hooks'
                or prior.flat_map(tree['project_flat_tree']) != project
                or prior.flat_map(tree['build_flat_tree']) != build
                or prior.tree_oid(tree['project_flat_tree']) != tree['applied_project_tree']
                or prior.tree_oid(tree['build_flat_tree']) != tree['build_input_tree']
                or tree['objects'] != {n: oid('blob', d) for n, d in [(API, overlay), (BRIDGE, fixture), (MAIN, main)]}):
            raise ValueError('instrumented project/program Git tree differs')
        if read(q / 'profile.json') != scope['target_toolchain']:
            raise ValueError('actual toolchain differs')
        bs = read(q / 'binaries.json')
        if set(bs) != {'general', 'race'} or post[role] != {v: b['sha256'] for v, b in bs.items()}:
            raise ValueError('binary population/postrun binding differs')
        for variant, binary in bs.items():
            r = read(q / ('build-' + variant) / 'receipt.json')
            args = [GO, 'build'] + (['-race'] if variant == 'race' else []) + ['-o', binary['path'], './cmd/ks-owned-rpc-lifecycle']
            if r['argv'] != args or r['exit_code'] or r['timeout'] or not r['reaped'] or r['remaining_live_group_members']:
                raise ValueError('actual build flags/result differ')
            check_raw(q / ('build-' + variant), r)
        binaries[role] = bs
    return binaries


def check_raw(directory, receipt):
    for stream in ('stdout', 'stderr'):
        if sha(raw(directory / (stream + '.gz'))) != receipt[stream + '_sha256']:
            raise ValueError('raw process output differs')


def inspect_events(events, life, role, scope):
    if [r['event'] for r in events] != EVENTS or [r['sequence'] for r in events] != list(range(1, 16)):
        raise ValueError('RPC event sequence differs')
    for rows in (events, life):
        if any(r['candidate'] != role for r in rows) or [r['elapsed_ns'] for r in rows] != sorted(r['elapsed_ns'] for r in rows):
            raise ValueError('trace identity/clock differs')
    begin = events[0]
    if begin['base_commit'] != prior.COMMIT or begin['native_timer_seconds'] != 60 or begin['standalone_program'] is not True or begin['source_lifecycle_hooks'] is not True or begin['temporary_instrumented_API'] is not True or begin['original_package_tests_executed'] is not False or begin['independent_oracle'] is not False or begin['external_listener'] is not False:
        raise ValueError('source hook observation scope differs')
    rpc = events[1]
    if (not re.fullmatch('0x[0-9a-f]+', rpc['subscription_id']) or rpc['caller_RPC_requests'] != 1
            or rpc['route'] != 'Server.ServeCodec_NewCodec_net.Pipe' or rpc['network_sync'] is not False
            or rpc['controlled_height'] != 1):
        raise ValueError('actual local RPC subscription identity/route differs')
    if events[3]['elapsed_ns'] < scope['minimum_first_native_ns'] or events[4]['elapsed_ns'] < scope['minimum_second_native_ns']:
        raise ValueError('unchanged native timer timing differs')
    if events[3]['wire_notification_delivered'] is not False or events[3]['actual_notifier'] is not True or events[3]['owned_transport_gate'] is not True or events[4]['native_timer_fires'] != 2 or events[4]['sole_RPC_consumer_in_Notify'] is not True or events[6]['write_gate_still_held'] is not True or events[6]['RPC_unsubscribe_response'] is not False:
        raise ValueError('actual gated RPC/native overlap facts differ')
    if events[4]['eventLoop_send_site'] != '/eth/downloader/api.go:' + str(scope['source_anchors'][role]['broadcast']):
        raise ValueError('native blocked source site differs')
    fences = [events[5], events[9], events[11]]
    if [r['label'] for r in fences] != ['before-close', 'before-rescue', 'final-before-rescue']:
        raise ValueError('lifecycle observation fences differ')
    counts = [r['through_sequence'] for r in fences]
    if any(type(n) is not int or n < 1 or n > len(life) for n in counts) or counts != sorted(counts):
        raise ValueError('finite lifecycle prefix bounds differ')
    prefixes = [validate_prefix(life[:n], role) for n in counts]
    for fence, count in zip(fences, counts):
        if fence['before_rescue'] is not True or fence['global_transactional_snapshot'] is not False or life[count - 1]['elapsed_ns'] > fence['elapsed_ns']:
            raise ValueError('fence not a pre-rescue finite prefix')
    first = prefixes[0]
    if first['installed_ids'] != [1] or first['pending_send'] != (1, 'progress'):
        raise ValueError('actual native send was not pending on original owned channel')
    frozen, probe, cleanup = events[8], events[10], events[13]
    if frozen['rescue_started'] is not False or frozen['uninstrumented_API_map_directly_observed'] is not False or frozen['all_post_ack_channel_use_proved'] is not False or frozen['independent_acceptable_candidate_set'] is not None or frozen['select_exit_vs_second_status_controlled'] is not False:
        raise ValueError('scope promoted from instrumentation to general proof')
    returned = frozen['callbacks'] == 0
    if frozen['callbacks'] not in (0, 1) or probe['executed'] is not returned or probe['universal_future_nonuse_proved'] is not False:
        raise ValueError('callback/probe population differs')
    if returned:
        final = prefixes[2]
        if frozen['eventLoop_send_pending'] is not False or frozen['method_uninstall_send_pending'] is not False:
            raise ValueError('returned callback contradicts paired-wait observation')
        if prefixes[1]['method_states'].get(1) != 'return_defer_observed' or 1 in prefixes[1]['installed_ids']:
            raise ValueError('RPC method returned without observed owner removal/ack')
        if probe['native_timer_fires'] != 3 or probe['elapsed_ns'] < scope['minimum_third_native_ns'] or probe['probe_unsubscribed'] is not True or probe['observer_scope'] != 'instrumented_API_only':
            raise ValueError('actual subsequent native probe differs')
        if final['installed_ids'] or final['pending_send'] is not None or final['method_states'].get(2) != 'return_defer_observed':
            raise ValueError('fresh probe did not complete actual subscription/unsubscription')
        progress = [r['channel_id'] for r in life[:counts[2]] if r['event'] == 'send_begin_progress']
        if progress != [1, 1, 2]:
            raise ValueError('post-ack native send population differs')
        if cleanup['rescue_used'] is not False:
            raise ValueError('returned method unexpectedly required controlled rescue')
    else:
        if role != 'ORIGINAL' or frozen['eventLoop_send_pending'] is not True or frozen['method_uninstall_send_pending'] is not True or probe['native_timer_fires'] != 2 or cleanup['rescue_used'] is not True or cleanup['rescue_ack_is_controlled_not_real_map_ack'] is not True:
            raise ValueError('paired waits/rescue were erased or promoted')
        if prefixes[1]['pending_send'] != (1, 'progress') or prefixes[1]['method_states'].get(1) != 'entered':
            raise ValueError('paired wait lifecycle differs')
    if cleanup['callbacks'] or cleanup['event_loops'] or cleanup['ServeCodec_joined'] is not True or cleanup['memory_chain_stopped'] is not True or events[14]['failed'] is not False:
        raise ValueError('owned runtime cleanup failed')
    if life and life[-1]['elapsed_ns'] > events[14]['elapsed_ns']:
        raise ValueError('lifecycle activity after trace termination')
    receives = sum(r['event'] == 'method_status_received_after_io' and r['channel_id'] == 1 for r in life[:counts[1]])
    return dict(actual_owner_delete_ack_observed=returned,
                RPC_callback_method_returned=returned,
                exit_method_drain_overlap_observed=bool(returned and receives),
                select_consumed_second_status_before_exit=bool(returned and not receives),
                original_paired_wait_observed=not returned,
                post_ack_native_probe_executed=returned,
                native_timer_fires=probe['native_timer_fires'],
                pre_rescue_prefixes=json.loads(json.dumps(prefixes)),
                lifecycle_records_total=len(life), pre_rescue_records=counts[2],
                cleanup_records_excluded_from_success=len(life) - counts[2],
                finite_followup_interval_only=True, all_future_nonuse_proved=False,
                receive_IO_linearization_proved=False, independent_approved=False)


def derive(root=ROOT, packet=None):
    packet = packet or root / PACKET
    if sha((packet / 'SCOPE.json').read_bytes()) != SCOPE_SHA or (packet / 'SCOPE.json').read_bytes() != (root / PREPARATION / 'SCOPE.json').read_bytes():
        raise ValueError('frozen lifecycle scope differs')
    scope = read(packet / 'SCOPE.json')
    actual = packet / 'actual'
    if read(actual / 'SCOPE.json') != scope or read(actual / 'planned.json') != scope['planned_slots']:
        raise ValueError('actual scope/plan differs')
    expected_receipt = dict(planned=6, attempted=6, automatic_retries=0,
                            source_commit=prior.COMMIT,
                            tracked_changes_only_declared_instrumented_api=True,
                            program_additions=[BRIDGE, MAIN], source_lifecycle_hooks=True,
                            original_package_tests_executed=False, production_patch=False,
                            formal_completed=0, formal_total=9, independent_approved=0, FINAL=0,
                            paid_or_external_LLM_calls=0, operational_nodes_started=0)
    if read(actual / 'receipt.json') != expected_receipt:
        raise ValueError('actual run population or bounded scope differs')
    built = sources(root, packet, scope)
    attempts = read(actual / 'attempts.json')
    if len(attempts) != 6 or len({r['slot_id'] for r in attempts}) != 6:
        raise ValueError('attempt population differs')
    by = {r['slot_id']: r for r in attempts}
    rows = []
    for slot in scope['planned_slots']:
        q = actual / 'slots' / slot['slot_id']
        r = read(q / 'receipt.json')
        process = read(q / 'process/receipt.json')
        binary = built[slot['candidate']][slot['variant']]
        if r != by[slot['slot_id']] or any(r.get(k) != v for k, v in {**slot, **process}.items()) or r['argv'] != [binary['path']] or r['binary_sha256'] != binary['sha256'] or r['fixture_sha256'] != scope['fixture_sha256'] or r['main_fixture_sha256'] != scope['main_fixture_sha256']:
            raise ValueError('actual process/source identity differs')
        if r['exit_code'] or r['timeout'] or r['term_sent'] or r['kill_sent'] or not r['reaped'] or not r['owned_new_session'] or r['remaining_live_group_members'] or r['tmp_children_after'] or r['elapsed_seconds'] >= scope['owned_process_timeout_seconds']:
            raise ValueError('failed program/forced termination/unproved cleanup')
        check_raw(q / 'process', r)
        events = [json.loads(line) for line in raw(q / 'events.jsonl.gz').splitlines()]
        life = [json.loads(line) for line in raw(q / 'events.jsonl.lifecycle.jsonl.gz').splitlines()]
        facts = inspect_events(events, life, slot['candidate'], scope)
        rows.append(dict(slot, **facts, elapsed_seconds=r['elapsed_seconds'], process_group_remaining=0))
    summary = dict(kind=KIND, actual_attempts=6, fresh_program_builds=6,
                   source_commit=prior.COMMIT, temporary_instrumented_API=True,
                   source_baseline_entries=2228, native_timer_seconds=60,
                   native_timer_fires=sum(r['native_timer_fires'] for r in rows),
                   owner_delete_ack_observations=sum(r['actual_owner_delete_ack_observed'] for r in rows),
                   post_ack_native_probes=sum(r['post_ack_native_probe_executed'] for r in rows),
                   method_drain_overlaps=sum(r['exit_method_drain_overlap_observed'] for r in rows),
                   uncontrolled_second_status_schedules=sum(r['select_consumed_second_status_before_exit'] for r in rows),
                   original_paired_waits=sum(r['original_paired_wait_observed'] for r in rows),
                   source_hook_records=sum(r['lifecycle_records_total'] for r in rows),
                   process_groups_remaining=0, source_hook_scheduling_effect=True,
                   gate_ignores_normal_write_deadline_while_held=True,
                   original_package_tests_executed=False, full_host_sync_indexer_verified=False,
                   universal_future_nonuse_proved=False, receive_IO_linearization_proved=False,
                   independent_approved=0, independent_tickets=0, FINAL=0,
                   formal_completed=0, formal_total=9, freeze=None, execution_allowed=False,
                   paid_or_external_LLM_calls=0, operational_nodes_started=0)
    author = subprocess.check_output(['git', 'show', REVISION + ':' + str(prior.prior.draft.AUTHOR)], cwd=root)
    notes = json.loads(author)
    covered = {'API-ENTRY', 'METHOD-RETURN', 'METHOD-REMOVE', 'METHOD-ACK',
               'BLOCKED-DELIVERY', 'OWNED-CLEANUP', 'SOURCE-BOUNDARY'}
    units = [dict(slot_id=row['slot_id'], requirement=r['id'], assertion=r['assertion'],
                  source_anchor_ids=r['anchors'],
                  label='bounded_instrumented_route_only' if r['id'] in covered else 'not_exercised_in_this_route',
                  full_requirement_satisfied=False, independent_approved=False,
                  remaining='Finite perturbed source-hook observation only; original paired waits require controlled cleanup ack. Full original schedules/late/Once/normal route matrix, uninstrumented all-future-use semantics, full host/sync/indexer, installed roles, independent gold and sufficient repeats remain.')
             for row in rows for r in notes['requirements']]
    summary.update(requirement_units=len(units), full_requirement_satisfaction=False,
                   requirements_source_revision=REVISION, requirements_source_sha256=sha(author))
    return dict(summary=summary, observations=rows, requirements=units)


def worklist(root, data):
    state = json.loads(subprocess.check_output(['git', 'show', REVISION + ':' + str(prior.PACKET / 'WORKLIST.json')], cwd=root))
    state.update(current_stage='70_finite_source_hook_lifecycle_runtime_supplement',
                 completed_in_this_scope=['six_fresh_general_race_program_builds_and_runs',
                  'actual_owner_thread_membership_delete_ack_and_method_hooks',
                  'post_ack_next_native_timer_finite_probe_for_returned_calls',
                  'controlled_original_cleanup_excluded_from_owner_ack_proof',
                  'WI6_version_aware_preparation_and55_static_regressions'],
                 next_task='WI6_actual_retained_seed_discovery_and_basic_expanded_pairs_then_warm_raw_observations',
                 actual_target_attempts=6, runtime_validated=True, compiled_programs=6,
                 storage_finalization_record='supplemental_runtime; actual_signed_storage_checked_after_commit',
                 last_verified_date='2026-10-19')
    state['remaining_all'][0]['task'] = ('WI-V1: finite instrumented owner membership/delete/ack and next-native-probe now observed for returned calls; '
      'no universal future-use or uninstrumented schedule proof; full original5/12, host factory/sync/indexer, installed roles/finalHEAD and independent tickets/gold/repeats remain')
    state['remaining_all'][2]['task'] = ('WI-6: historical/current version separation and paired request proposal prepared; '
      'actual retained seed discovery/basic-expanded calls/warm raw samples0; broader nonseed independent gold/families/sufficiency remain')
    state['AI_DEV_draft_progress']['WI-V1'] = 'phase70_six_instrumented_programs; finite_map_delete_ack_and_followup; full_contract_pending'
    state['AI_DEV_draft_progress']['WI-6'] = 'phase69_version_aware_static_preparation; actual_pairs0_warm0_independent0'
    state['supplemental_runtime_summary'] = data['summary']
    return state


def seal(packet):
    write(packet / 'manifest.json', dict(kind=KIND, previous_verified_revision=REVISION,
          tools=[dict(path=t, sha256=sha((ROOT / t).read_bytes())) for t in TOOLS],
          artifacts={p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in sorted(packet.rglob('*'))
                     if p.is_file() and p != packet / 'manifest.json'}))


def prepare(work, packet):
    packet.mkdir()
    shutil.copyfile(ROOT / PREPARATION / 'SCOPE.json', packet / 'SCOPE.json')
    for p in sorted((work / 'rpc-lifecycle-trials-v1').rglob('*')):
        if not p.is_file() or p.suffix == '.program':
            continue
        out = packet / 'actual' / p.relative_to(work / 'rpc-lifecycle-trials-v1')
        out.parent.mkdir(parents=True, exist_ok=True)
        if p.name in ('stdout', 'stderr') or p.suffix in ('.stack', '.jsonl'):
            out.with_name(out.name + '.gz').write_bytes(gzip.compress(p.read_bytes(), mtime=0))
        else:
            shutil.copyfile(p, out)
    for role in read(packet / 'SCOPE.json')['candidate_roles']:
        for binary in read(packet / 'actual/candidates' / role / 'binaries.json').values():
            if sha(Path(binary['path']).read_bytes()) != binary['sha256']:
                raise ValueError('actual postrun binary differs')
    data = derive(packet=packet)
    write(packet / 'SUMMARY.json', data['summary'])
    write(packet / 'observations.json', data['observations'])
    write(packet / 'requirements.json', data['requirements'])
    write(packet / 'WORKLIST.json', worklist(ROOT, data))
    seal(packet)


def validate(root=ROOT, packet=None):
    packet = packet or root / PACKET
    manifest = read(packet / 'manifest.json')
    actual = {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in packet.rglob('*')
              if p.is_file() and p != packet / 'manifest.json'}
    if manifest['kind'] != KIND or manifest['artifacts'] != actual or [r['path'] for r in manifest['tools']] != TOOLS:
        raise ValueError('supplemental artifact inventory differs')
    if any(sha((root / r['path']).read_bytes()) != r['sha256'] for r in manifest['tools']):
        raise ValueError('supplemental source tool changed')
    data = derive(root, packet)
    if (read(packet / 'SUMMARY.json') != data['summary'] or read(packet / 'observations.json') != data['observations']
            or read(packet / 'requirements.json') != data['requirements']
            or read(packet / 'WORKLIST.json') != worklist(root, data)):
        raise ValueError('actual lifecycle result was changed or promoted')
    return data


def stored(root=ROOT):
    revision = subprocess.check_output(['git', 'log', '-1', '--format=%H', '--', str(PACKET / 'manifest.json')], cwd=root, text=True).strip()
    if not re.fullmatch('[0-9a-f]{40}', revision):
        raise ValueError('supplemental runtime evidence not committed')
    manifest = read(root / PACKET / 'manifest.json')
    paths = [PACKET / 'manifest.json', *(PACKET / p for p in manifest['artifacts']), *(Path(t) for t in TOOLS)]
    paths += [Path('docs/spec-driven') / p for p in ('CURRENT-HANDOFF.md', 'KNOWLEDGE-ROADMAP-TEST-MATRIX.md', 'OPERATIONAL-ACCEPTANCE-PROTOCOL.md')]
    if any(subprocess.check_output(['git', 'show', revision + ':' + str(p)], cwd=root) != (root / p).read_bytes() for p in paths):
        raise ValueError('actual committed evidence/tools/current documents differ')
    signature = subprocess.run(['git', 'verify-commit', revision], cwd=root, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if signature.returncode:
        raise ValueError('actual Git signature verification failed')
    return revision


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('command', choices=['prepare', 'check'])
    parser.add_argument('--workspace', type=Path)
    parser.add_argument('--require-formal', action='store_true')
    parser.add_argument('--require-stored', action='store_true')
    args = parser.parse_args()
    if args.command == 'prepare':
        prepare(args.workspace.resolve(), ROOT / PACKET)
    else:
        result = validate()
        if args.require_stored:
            stored()
        if args.require_formal:
            raise SystemExit('finite instrumented observations are not independently frozen formal acceptance')
        print(json.dumps(result['summary']))
