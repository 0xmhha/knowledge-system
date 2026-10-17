#!/usr/bin/env python3
"""Freeze T09 two-project evidence, including the graph-local ID discovery."""
import argparse
from functools import lru_cache
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import importlib.util
from source_packet import SourceCapture, safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PREVIOUS = BASE / 'followups/consumer-opt-in-v1'
PACKET = BASE / 'followups/consumer-foreign-pin-v1'
BASE_COMMIT = 'f1608eeb6bda1f14ab1519083eaed9b7c55cb6bc'


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


old = module('prepare-consumer-opt-in.py')
driver = module('run-consumer-foreign-pin.py')
read, dump, lines, require, digest = old.read, old.dump, old.lines, old.require, old.digest
INPUTS = [PREVIOUS / 'manifest.json', Path('scripts/fixtures/consumer-foreign-pin-plan.json'), Path('scripts/run-consumer-foreign-pin.py'), *old.INPUTS, Path('scripts/prepare-consumer-opt-in.py'), Path('scripts/check-consumer-opt-in.py'), Path('scripts/test_consumer_opt_in.py')]
CASES = ['T09-matrix', 'T09-build-A', 'T09-build-B', 'T09-delivery']
CONTROLS = ['node', 'canonical', 'file', 'commit', 'source', 'snapshot', 'ticket', 'foreign']
SESSION = ['stdin.jsonl', 'stdout.jsonl', 'stderr.txt', 'messages.json', 'lifecycle.json']
REASONS = ['request/source pin mismatch: node_id', 'request/source pin mismatch: canonical_id', 'structural review identity/location differs', 'request/source pin mismatch: expected_commit', 'request/source pin mismatch: expected_source_sha256', 'request/source pin mismatch: expected_snapshot_sha256', 'review ticket identity/type differs', 'analysis must bind exact validated deep review bytes']


@lru_cache(maxsize=1)
def expected_preservation():
    keep = dict(old.preservation()['files'])
    manifest = read(ROOT / PREVIOUS / 'manifest.json')
    names = [str(PREVIOUS / 'manifest.json'), *[str(PREVIOUS / f) for f in manifest['artifacts']], 'scripts/prepare-consumer-opt-in.py', 'scripts/check-consumer-opt-in.py', 'scripts/test_consumer_opt_in.py']
    archive = subprocess.check_output(['git', 'archive', BASE_COMMIT, *sorted(set(names))], cwd=ROOT)
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        for entry in tar:
            if entry.isfile():
                keep[entry.name] = sha(tar.extractfile(entry).read())
    return {'base_commit': BASE_COMMIT, 'prior_packets': 32, 'files': keep}


def preservation():
    value = expected_preservation()
    for name, expected in value['files'].items():
        require(digest(safe(ROOT, name)) == expected, 'prior source/packet changed: ' + name)
    return value


def snapshot_files(folder, state):
    require(set(state) <= set(old.ARTIFACTS) | {'state.json', 'input-bundle.json', 'scripted-judgments.json', 'adapter-commands.json'}, 'unexpected workspace inventory')
    for name, row in state.items():
        path = safe(folder, name + '.txt' if name.endswith('.md') else name)
        require(digest(path) == row['sha256'], 'workspace source bytes: ' + name)


def materialized(folder, ns):
    with tempfile.TemporaryDirectory(prefix='ks-t09-verify-') as tmp:
        path = Path(tmp) / 'workspace'
        shutil.copytree(folder, path)
        (path / 'review-report.md.txt').rename(path / 'review-report.md')
        return ns['verify_workspace'](path)


def session(folder, binary_pin):
    records = read(folder / 'messages.json')
    life = read(folder / 'lifecycle.json')
    require(life['exit_code'] == 0 and life['child_reaped'] and life['reader_joined'] and life['eof_sent'] and not life['kill_sent'] and not life['terminate_sent'], 'native/CKG cleanup')
    require(life['command'][0] == binary_pin[0], 'native executable selection')
    sent = lines(folder / 'stdin.jsonl')
    require(sent[1] == {'jsonrpc': '2.0', 'method': 'notifications/initialized'} and sent[:1] + sent[2:] == [r['request'] for r in records] and lines(folder / 'stdout.jsonl') == [r['response'] for r in records], 'native raw wire association')
    return records, life


def host(folder, case, helper_hash, pins, text, expected_calls):
    inv, end = read(folder / 'invocation.json'), read(folder / 'exit.json')
    argv = inv['argv']
    role = 'planner' if case == 'T09-delivery' else 'analyzer'
    require(inv['case'] == case and inv['role'] == role and inv['driver_sha256'] == helper_hash, 'actual host/helper identity')
    require(argv[argv.index('--agent') + 1] == 'coding-agent:' + role and 'KS_HOST_PROBE:T04-' + case in argv[-1], 'actual role/case dispatch')
    require(inv['host_sha256'] == '475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3' and inv['cks_sha256'] == pins['cks_sha256'], 'host/CKS pin')
    require(inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:') and inv['api_credential_kind'] == 'constant_fake_local_token', 'nonlocal provider')
    require(argv[argv.index('--setting-sources') + 1] == '' and '--strict-mcp-config' in argv and '--dangerously-skip-permissions' not in argv and '--bare' not in argv, 'settings/permission boundary')
    require(end['host_exit'] == 0 and end['host_reaped'] and not end['forced'] and end['elapsed_seconds'] < 90 and not end['rescue'] and end['cleanup']['all_observed_identities_gone'], 'host cleanup')
    require(end['real_model_calls'] == end['formal_runs'] == 0, 'formal/model run promotion')
    g = end['gateway_cleanup']
    require(g['all_handler_threads_joined'] and g['server_socket_closed'] and g['server_thread_joined'] and not g['errors'], 'gateway cleanup')
    requests, responses, frames = old.calls(folder)
    require(len(requests) == expected_calls, 'actual CKS call denominator: ' + case)
    owner = next(r for r in frames if r['kind'] == 'owner')
    starts, exits = [r for r in frames if r['kind'] == 'start'], [r for r in frames if r['kind'] == 'exit']
    require(owner['owner'] == inv['owner'] and owner['driver_sha256'] == helper_hash and owner['proxy_sha256'] == digest(ROOT / 'scripts/consumer-lifecycle-proxy.py'), 'proxy source/owner')
    require(len(starts) == len(exits) == 1 and starts[0]['child_pid'] == exits[0]['child_pid'] and starts[0]['binary_sha256'] == pins['cks_sha256'], 'CKS child identity')
    exit_row = exits[0]
    require(exit_row['exit_code'] == 0 and exit_row['child_reaped'] and not exit_row['forced_kill'] and not exit_row['terminate_sent'] and exit_row['signal_worker_joined'] and exit_row['output_workers_joined'], 'CKS exit/reap')
    gateway = [r for r in read(folder / 'gateway-requests.json') if 'scripted_response' in r]
    plugin = argv[argv.index('--plugin-dir') + 1]
    require(plugin == '/private/tmp/ks-host-schema-consumer-20261017/plugin', 'unexpected plugin')
    body = text('plugin/agents/' + role + '.md').split('---', 2)[2].strip().replace('${CLAUDE_PLUGIN_ROOT}', plugin)
    require(any(body in '\n'.join(v.get('text', '') for v in row['body'].get('system', [])) for row in gateway), 'actual role source not loaded')
    actions = [a for row in gateway for a in row['scripted_response']['content'] if a['type'] == 'tool_use']
    results = {v['tool_use_id']: v for row in gateway for m in row['body']['messages'] if m['role'] == 'user' for v in m.get('content', []) if isinstance(v, dict) and v.get('type') == 'tool_result'}
    require(all(a['id'] in results for a in actions), 'actual tool result missing')
    tool_actions = [a for a in actions if a['name'].startswith('mcp__')]
    require(len(tool_actions) == len(requests), 'API/wire denominator')
    for a, q in zip(tool_actions, requests):
        require(a['name'] == 'mcp__plugin_coding-agent_cks__' + q['params']['name'].replace('.', '_') and a['input'] == q['params']['arguments'] and q['params']['_meta']['claudecode/toolUseId'] == a['id'], 'API/wire selection')
        require(not results[a['id']].get('is_error'), 'actual MCP transport error')
    declared = read(folder / 'scripted-actions.json')
    require(len(declared) == len(actions), 'scripted/API denominator')
    resolutions = lines(folder / 'action-resolutions.jsonl') if (folder / 'action-resolutions.jsonl').exists() else []
    for index, (spec, action) in enumerate(zip(declared, actions)):
        if 'state_operation' in spec:
            resolution = next(r for r in resolutions if r['index'] == index)
            require(resolution['operation'] == spec['state_operation'] and action['name'] == 'Write' and json.loads(action['input']['content']) == resolution['proposed'], 'state action resolution')
        else:
            require(spec == {'name': action['name'], 'input': action['input']}, 'declared action differs')
    snapshot_files(folder / 'workspace-after', read(folder / 'workspace-after.json'))
    return requests, responses, actions, results, {end['host_pid'], starts[0]['wrapper_pid'], starts[0]['child_pid']}


def derive(packet):
    ns, text = old.consumer()
    pins = read(ROOT / old.PRIOR / 'binary-pins.json')
    execution = read(packet / 'execution.json')
    require(execution['driver_sha256'] == digest(ROOT / 'scripts/run-consumer-foreign-pin.py') and execution['plan_sha256'] == digest(ROOT / 'scripts/fixtures/consumer-foreign-pin-plan.json') and execution['host_helper_sha256'] == digest(ROOT / 'scripts/run-consumer-opt-in.py'), 'executed driver/plan/helper pin')
    require(execution['host_cases'] == CASES and execution['candidate_adopted'] is False and execution['formal_runs'] == execution['real_model_calls'] == 0, 'execution scope')
    require(read(packet / 'formal-state.json') == module('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
    require(read(packet / 'preservation.json') == preservation(), 'prior32 inventory')
    query, foreign = execution['query'], execution['foreign']
    require(query['node_id'] == foreign['node_id'] and execution['other_node'] != query['node_id'], 'graph-local collision/other-function control missing')
    for name in ['expected_commit', 'expected_source_sha256', 'expected_snapshot_sha256']:
        require(query[name] != foreign[name], 'projects not distinct: ' + name)
    a_canonical = read(module('run-consumer-opt-in.py').base.P05 / 'execution.json')['selected']['canonical_id']
    require(a_canonical != foreign['canonical_id'] and foreign['canonical_id'].startswith('example.invalid/foreign-pin-fixture.'), 'distinct module canonical required')
    proof = verify(packet / 'project-B-source', foreign['expected_commit'])
    blob = safe(packet / 'project-B-source', proof['files']['logic.go']['artifact']).read_bytes()
    require(b'foreign-pin-fixture' in safe(packet / 'project-B-source', proof['files']['go.mod']['artifact']).read_bytes(), 'foreign source module')
    binary = '/private/tmp/ks-host-schema-runtime-20261017/bin/'
    discovered, disc_life = session(packet / 'discovery-B', (binary + 'ckg', pins['ckg_sha256']))
    require(len(discovered) == 2 and discovered[1]['request']['params']['arguments']['exact'] is False, 'B discovery contract')
    node = next(n for n in module('prepare-consumer-stdio-readiness.py').payload(discovered[1]['response'])['nodes'] if n['id'] == foreign['node_id'])
    require(node['canonical_id'] == foreign['canonical_id'] and sha(node['source'].encode()) == foreign['expected_source_sha256'] and node['source'].encode() in blob, 'foreign retained source not in committed B')
    native, native_life = session(packet / 'native-B', (binary + 'cks', pins['cks_sha256']))
    require([r['label'] for r in native] == ['initialize', 'B-positive', 'stale-A-on-B'], 'B native denominator')
    payload = module('prepare-consumer-stdio-readiness.py').payload
    positive, stale = payload(native[1]['response']), payload(native[2]['response'])
    require(positive['snapshot_sha256'] == foreign['expected_snapshot_sha256'] and positive['analysis']['source']['canonical_id'] == foreign['canonical_id'], 'B positive pins')
    require(stale['status'] == 'source_mismatch' and stale.get('analysis') is None and native[2]['request']['params']['arguments'] == query, 'stale A-on-B refusal')
    require(read(packet / 'matrix-cases.json') == driver.matrix(query, foreign, execution['other_node']), 'matrix expectation inputs')
    pids = {native_life['pid'], disc_life['pid']}
    matrix_rows = read(packet / 'matrix-cases.json')
    q, responses, _, _, ids = host(packet / 'hosts/T09-matrix', 'T09-matrix', execution['host_helper_sha256'], pins, text, 36)
    pids.update(ids)
    refusals = 0
    for row, request in zip(matrix_rows, q):
        require(request['params']['name'] == 'cks.context.' + row['tool'] and request['params']['arguments'] == row['arguments'], 'matrix actual query')
        value = payload(responses[request['id']])
        is_expansion = row['tool'] == 'value_dependency_expansion'
        result = value['graph'] if is_expansion else value
        if row['expected'] == 'positive':
            analysis = result['functions'][0]['result']['analysis'] if is_expansion else result.get('analysis')
            if row['tool'] == 'policy_security':
                require(result['status'] == 'annotations_found' and result['symbol']['canonical_id'] == a_canonical and result['symbol']['indexed_commit'] == query['expected_commit'] and result['symbol']['source_sha256']['value'] == query['expected_source_sha256'] and result['snapshot_sha256'] == query['expected_snapshot_sha256'], 'positive scoped policy')
            else:
                require(analysis['source']['canonical_id'] == a_canonical and analysis['source']['commit'] == query['expected_commit'] and analysis['source']['sha256'] == query['expected_source_sha256'], 'positive scoped A response')
        else:
            require(result['status'] == row['expected'], 'matrix refusal differs: ' + row['label'])
            require(not result.get('analysis') and not result.get('annotations') and all(not f['result'].get('analysis') for f in result.get('functions', [])), 'refusal leaked usable evidence')
            refusals += 1
    require(refusals == 24, 'matrix refusal denominator')
    builds = {}
    for case, source in [('T09-build-A', query), ('T09-build-B', foreign)]:
        folder = packet / 'hosts' / case
        requests, responses, _, results, ids = host(folder, case, execution['host_helper_sha256'], pins, text, 2)
        pids.update(ids)
        require(all(not v.get('is_error') for v in results.values()), 'positive build tool error')
        materialized(folder / 'workspace-after', ns)
        bundle = read(folder / 'workspace-after/deep-review-evidence.json')
        for r, key in zip(requests, ['rules', 'function_review']):
            require(bundle[key] == {'tool': r['params']['name'], 'arguments': r['params']['arguments'], 'result': payload(responses[r['id']])}, 'actual bundle receipt')
        selected = bundle['rules']['result']['analysis']['source']
        require(selected['commit'] == source['expected_commit'] and selected['sha256'] == source['expected_source_sha256'] and selected['snapshot_sha256'] == source['expected_snapshot_sha256'] and selected['file'] == 'logic.go' and selected['function_name'] == 'Seed', 'project build identity')
        builds[case] = {n: read(folder / 'workspace-after.json')[n]['sha256'] for n in old.ARTIFACTS}
    require(builds['T09-build-A']['deep-review.json'] != builds['T09-build-B']['deep-review.json'], 'coherent foreign artifact not distinct')
    folder = packet / 'hosts/T09-delivery'
    _, _, actions, results, ids = host(folder, 'T09-delivery', execution['host_helper_sha256'], pins, text, 0)
    pids.update(ids)
    writes = [a for a in actions if a['name'] == 'Write']
    require(len(writes) == 10, 'Write attempt denominator')
    for action, reason in zip(writes[:8], REASONS):
        result = results[action['id']]
        require(result.get('is_error') is True and 'Structured review delivery blocked: ' + reason in str(result['content']), 'actual Write hook did not refuse expected mismatch')
    require(results[writes[8]['id']].get('is_error') is True and 'frozen review identity changed outside analysis' in str(results[writes[8]['id']]['content']), 'foreign frozen rebind allowed')
    require(not results[writes[-1]['id']].get('is_error'), 'genuine A completion blocked')
    for case, reason in zip(CONTROLS, REASONS):
        c = packet / 'controls' / case
        before, mutated, after, restored = read(c / 'before.json'), read(c / 'mutated.json'), read(c / 'after-attempt.json'), read(c / 'restore.json')
        require(mutated['verification']['exit_code'] == 2 and json.loads(mutated['verification']['stdout']) == {'status': 'invalid', 'reason': reason}, 'consumer CLI mismatch refusal')
        require(mutated['workspace'] == after, 'denied Write still changed workspace')
        require(before == restored['workspace'] and restored['verification']['exit_code'] == 0, 'control restore failed')
        for name in old.ARTIFACTS:
            require(before[name]['sha256'] == builds['T09-build-A'][name], 'control did not start from A')
        snapshot_files(c / 'original', {name: before[name] for name in driver.FILES})
        if case == 'foreign':
            require(all(mutated['workspace'][n]['sha256'] == builds['T09-build-B'][n] for n in old.ARTIFACTS), 'coherent B replacement not exercised')
    materialized(folder / 'workspace-after', ns)
    require(all(read(folder / 'workspace-after.json')[n]['sha256'] == builds['T09-build-A'][n] for n in old.ARTIFACTS), 'final delivery not original A')
    require(all(not v.get('is_error') for a in actions if a['name'] == 'Bash' for v in [results[a['id']]]), 'control/restore/final Bash failed')
    diagnostic = read(packet / 'diagnostics/summary.json')
    require(diagnostic == {'initial_discovery_controller_exit': 1, 'initial_discovery_empty': True, 'initial_full_run_not_T09_pass': True, 'node_ID_collision_invalidated_foreign_node_test': True, 'initial_ticket_Write_blocked_by_stale_read_not_hook': True, 'initial_controller_source': 'driver-before-collision.py.txt', 'final_controller_source': 'scripts/run-consumer-foreign-pin.py'}, 'diagnostic failures hidden')
    init, first_life = session(packet / 'diagnostics/discovery-initial', (binary + 'ckg', pins['ckg_sha256']))
    require(init[1]['request']['params']['arguments']['exact'] is True and payload(init[1]['response'])['nodes'] == [], 'initial empty discovery omitted')
    require('StopIteration' in (packet / 'diagnostics/discovery-controller.stderr.txt').read_text(), 'initial controller failure omitted')
    pids.add(first_life['pid'])
    initial = packet / 'diagnostics/integration'
    old_execution = read(initial / 'execution.json')
    require(old_execution['driver_sha256'] == digest(packet / 'diagnostics/driver-before-collision.py.txt') and old_execution['plan_sha256'] == digest(packet / 'diagnostics/plan-before-collision.json'), 'initial executed controller version')
    require(read(initial / 'controls/node/mutated.json')['verification']['exit_code'] == 0, 'initial node no-op validation hidden')
    for case, count in zip(CASES, [32, 2, 2, 0]):
        _, _, aa, rr, ids = host(initial / 'hosts' / case, case, execution['host_helper_sha256'], pins, text, count)
        pids.update(ids)
        if case == 'T09-delivery':
            ww = [a for a in aa if a['name'] == 'Write']
            require(not rr[ww[0]['id']].get('is_error') and 'File has been modified since read' in str(rr[ww[6]['id']]['content']), 'initial unexpected allow/stale-read not retained')
    for name, binary_name in [('native-B', 'cks'), ('discovery-B', 'ckg')]:
        _, receipt = session(initial / name, (binary + binary_name, pins[binary_name + '_sha256']))
        pids.add(receipt['pid'])
    integrity = read(packet / 'integrity.json')
    require(integrity['binary_sha256'] == {name: pins[name + '_sha256'] for name in ['cks', 'ckg']}, 'native executable pins')
    expected = read(ROOT / old.PRIOR / 'execution.json')
    require(all(row == expected[key] for key, row in integrity['checkouts'].items()) and len(integrity['checkouts']) == 4, 'source checkout pin/cleanliness')
    for name in ['graph.db', 'manifest.json']:
        require(digest(packet / 'indexes/A' / name) == digest(ROOT / BASE / 'consumer-stdio-readiness-v1/index' / name), 'A index changed')
        require(digest(packet / 'indexes/B' / name) == integrity['index_B'][name], 'B index hash')
    require(all(c['exit_code'] == 0 for c in read(packet / 'commands.json')) and {c['label'] for c in read(packet / 'commands.json')} == {'format', 'fixture-test', 'fixture-vet', 'git-add', 'git-commit', 'git-head', 'index'}, 'B setup commands')
    audit = read(packet / 'postrun-process-audit.json')
    require({r['pid'] for r in audit['probes']} == pids and len(audit['probes']) == len(pids), 'owned PID audit denominator')
    require(all(r['exit_code'] == 1 and not r['stdout'].strip() for r in audit['probes']) and audit['self']['exit_code'] == 0 and str(audit['pid']) in audit['self']['stdout'], 'owned cleanup audit/positive control')
    return {'test_id': 'T09', 'status': 'isolated_two_project_pin_and_frozen_delivery_pass', 'final_host_sessions': 4, 'final_CKS_requests': 42, 'final_CKS_exit0_reaped': 5, 'final_CKG_exit0_reaped': 1, 'matrix_requests': 36, 'matrix_refusals': 24, 'matrix_positives_including_local_ID_collision': 12, 'native_stale_A_on_B_refusals': 1, 'consumer_CLI_refusals': 8, 'actual_Write_hook_refusals': 9, 'genuine_A_completion': True, 'all_attempts_host_sessions': 8, 'all_attempts_CKS_requests': 80, 'all_attempts_CKS_exit0_reaped': 10, 'all_attempts_CKG_exit0_reaped': 3, 'owned_PIDs_absent': len(pids), 'node_IDs_globally_unique': False, 'structural_expected_pins_supported': False, 'tenant_ACL_or_transport_authentication_proven': False, 'autonomous_model_adherence': 'not_evaluated', 'prior_frozen_packets_unchanged': 32, 'formal_completed': 0, 'formal_total': 9, 'ontology_effect_acceptance': 'not_evaluated', 'candidate_adopted': False, 'real_model_calls': 0, 'formal_runs': 0, 'full_plan_passed': 19, 'full_plan_pending': ['T10', 'T16', 'T22'], 'next': 'T10 partial/unknown, then T16 and P08/T22'}


def copy_workspace(source, target):
    shutil.copytree(source, target)
    for path in list(target.rglob('*.md')):
        path.rename(path.with_suffix('.md.txt'))


def copy_hosts(a, b, target):
    for case in CASES:
        source = (b if case == 'T09-build-B' else a) / case
        dest = target / 'hosts' / case
        dest.mkdir(parents=True)
        for name in old.TOP:
            shutil.copyfile(source / name, dest / name)
        if (source / 'action-resolutions.jsonl').exists():
            shutil.copyfile(source / 'action-resolutions.jsonl', dest / 'action-resolutions.jsonl')
        copy_workspace(source / 'workspace-after', dest / 'workspace-after')


def prepare(a, b, initial_a, initial_b, discovery_initial, output):
    require(not output.exists(), 'fresh evidence output required')
    output.mkdir(parents=True)
    copy_hosts(a, b, output)
    for name in ['execution.json', 'commands.json', 'matrix-cases.json']:
        shutil.copyfile(a / name, output / name)
    for name in ['discovery-B', 'native-B']:
        shutil.copytree(a / name, output / name)
    copy_workspace(a / 'controls', output / 'controls')
    diag = output / 'diagnostics'
    diag.mkdir()
    shutil.copytree(discovery_initial / 'discovery-B', diag / 'discovery-initial')
    shutil.copyfile(discovery_initial / 'driver-initial.py.txt', diag / 'driver-initial.py.txt')
    shutil.copyfile(discovery_initial / 'controller.stderr.txt', diag / 'discovery-controller.stderr.txt')
    shutil.copyfile(initial_a / 'driver-before-collision.py.txt', diag / 'driver-before-collision.py.txt')
    shutil.copyfile(initial_a / 'plan-before-collision.json', diag / 'plan-before-collision.json')
    copy_hosts(initial_a, initial_b, diag / 'integration')
    shutil.copyfile(initial_a / 'execution.json', diag / 'integration/execution.json')
    for name in ['discovery-B', 'native-B']:
        shutil.copytree(initial_a / name, diag / 'integration' / name)
    copy_workspace(initial_a / 'controls', diag / 'integration/controls')
    dump(diag / 'summary.json', {'initial_discovery_controller_exit': 1, 'initial_discovery_empty': True, 'initial_full_run_not_T09_pass': True, 'node_ID_collision_invalidated_foreign_node_test': True, 'initial_ticket_Write_blocked_by_stale_read_not_hook': True, 'initial_controller_source': 'driver-before-collision.py.txt', 'final_controller_source': 'scripts/run-consumer-foreign-pin.py'})
    capture = SourceCapture(b / 'source', output / 'project-B-source', read(a / 'execution.json')['foreign']['expected_commit'])
    for name in subprocess.check_output(['git', '-C', str(b / 'source'), 'ls-files'], text=True).splitlines():
        capture.source(name)
    dump(output / 'project-B-source/git-proof.json', capture.proof())
    for name, src in [('A', a / 'graph'), ('B', b / 'foreign-graph')]:
        target = output / 'indexes' / name
        target.mkdir(parents=True)
        for f in ['graph.db', 'manifest.json']:
            shutil.copyfile(src / f, target / f)
    dump(output / 'formal-state.json', module('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output / 'preservation.json', preservation())
    repos = {'cks_postrun': '/private/tmp/ks-host-schema-cks-20261017', 'consumer_postrun': '/private/tmp/ks-host-schema-consumer-20261017', 'original_postrun': '/private/tmp/ks-isolated-host-20261015/coding-agent', 'prior_candidate_postrun': '/private/tmp/ks-consumer-contract-candidate-20261017'}
    dump(output / 'integrity.json', {'checkouts': {key: {'commit': subprocess.check_output(['git', '-C', path, 'rev-parse', 'HEAD'], text=True).strip(), 'porcelain': subprocess.check_output(['git', '-C', path, 'status', '--porcelain'], text=True)} for key, path in repos.items()}, 'index_B': {name: digest(b / 'foreign-graph' / name) for name in ['graph.db', 'manifest.json']}, 'binary_sha256': {name: digest(Path('/private/tmp/ks-host-schema-runtime-20261017/bin') / name) for name in ['cks', 'ckg']}})
    pids = set()
    for target in [output, diag / 'integration']:
        for case in CASES:
            pids.add(read(target / 'hosts' / case / 'exit.json')['host_pid'])
            for row in lines(target / 'hosts' / case / 'mcp-frames.jsonl'):
                if row['kind'] == 'start':
                    pids.update([row['wrapper_pid'], row['child_pid']])
        for name in ['discovery-B', 'native-B']:
            pids.add(read(target / name / 'lifecycle.json')['pid'])
    pids.add(read(diag / 'discovery-initial/lifecycle.json')['pid'])
    def ps(pid):
        q = subprocess.run(['ps', '-p', str(pid), '-o', 'pid=,ppid=,stat=,lstart=,command='], capture_output=True, text=True)
        return {'pid': pid, 'exit_code': q.returncode, 'stdout': q.stdout, 'stderr': q.stderr}
    dump(output / 'postrun-process-audit.json', {'pid': os.getpid(), 'self': ps(os.getpid()), 'probes': [ps(pid) for pid in sorted(pids)]})
    dump(output / 'foreign-pin.json', derive(output))
    (output / 'REPORT.txt').write_text('준비33/T09 — 실제 A/B 프로젝트 pin·동결 전달 검증\n최종4host/42CKS요청: matrix36 중 거부24·정상/graph-local ID 대조12, stale A→B1거부, 실제 CLI8거부·Write hook9거부 및 정상 A완료.\n두 프로젝트의 node ID가 충돌한다. canonical/commit/source/snapshot+선택 graph를 함께 사용해야 하며 node ID의 전역 유일성이나 tenant ACL을 주장하지 않는다. structural snapshot pin은 미지원이다.\n초기 발견 질의 빈 결과·첫 통합 시도의 node no-op 허용과 ticket stale-read를 보존한다. 초기4+최종4 host, CKS80요청/10회수, CKG3회수. 실패한 초기 시도를 T09 통과로 세지 않음.\n이전32패킷/runtime/schema 보존. 계획19/22·남은 T10/T16/T22, 정식0/9·담당자미정·freeze null·실제모델0·후보미채택·온톨로지 효과 미평가.\n')
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    (output / '.gitattributes').write_text('**/*.commit binary\n**/*.tree binary\n**/*.db binary\n')
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'T09_two_project_pins_not_tenant_authentication', 'input_bindings': {str(p): digest(ROOT / p) for p in INPUTS}, 'generator_sha256': digest(Path(__file__)), 'artifacts': {p.relative_to(output).as_posix(): digest(p) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('T09 frozen:4 final hosts/42 calls; initial failures retained; plan19/22 formal0/9')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['a', 'b', 'initial-a', 'initial-b', 'discovery-initial', 'output']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    prepare(args.a.resolve(), args.b.resolve(), args.initial_a.resolve(), args.initial_b.resolve(), args.discovery_initial.resolve(), args.output.resolve())
