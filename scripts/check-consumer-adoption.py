#!/usr/bin/env python3
"""Recompute approved local adoption evidence; old source guards remain historical."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
REL = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/consumer-adoption-v1')


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read(path):
    return json.loads(path.read_text())


def rows(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def log(packet, suite, label, stream):
    return gzip.decompress((packet / 'validation' / suite / (label + '.' + stream + '.gz')).read_bytes()).decode(errors='replace')


def normalize(value):
    if isinstance(value, dict):
        result = {k: normalize(v) for k, v in value.items() if k != 'elapsed_micros'}
        if value.get('type') == 'text' and isinstance(value.get('text'), str):
            try:
                result['text'] = json.dumps(normalize(json.loads(value['text'])), sort_keys=True, separators=(',', ':'))
            except ValueError:
                pass
        return result
    if isinstance(value, list):
        return [normalize(v) for v in value]
    return value


def derive(packet, live=True):
    state = read(packet / 'integration-result.json')
    require(state['formal_completed'] == 0 and state['formal_total'] == 9 and state['reviewer_assignment'] == '미정' and not state['production_installation'] and state['paid_model_calls'] == state['external_publication'] == 0, 'scope/independence promotion')
    source = read(packet / 'consumer-source.json')
    require(all(x['exit_code'] == 0 for x in source['signature_receipts'].values()) and source['consumer_final_commit'] == state['consumer_final_commit'], 'consumer signed source association')
    for role in ['analyzer', 'planner']:
        require(sha(packet / 'consumer-source/agents' / (role + '.md.txt')) == source['consumer_plugin_sha256']['plugin/agents/' + role + '.md'], 'actual role source hash')
    binding = read(packet / 'source-binding.json')
    require(binding['authorization']['origin'] == 'direct_human_user_message' and binding['authorization']['reviewer_assignment'] == '미정', 'source authorization')
    raw = subprocess.check_output(['git', 'show', state['native_source_commit'] + ':' + str(REL / 'source-binding.json')], cwd=ROOT)
    require(hashlib.sha256(raw).hexdigest() == sha(packet / 'source-binding.json'), 'committed native source binding')
    if live:
        for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256']}.items():
            require(sha(ROOT / name) == digest, 'current source/input differs: ' + name)
    native = read(packet / 'validation/native-isolated/receipts.json')
    require([r['label'] for r in native] == ['build', 'test', 'race', 'lint', 'build-bins'], 'full native command inventory')
    require(all(r['exit_code'] == 0 and not r['timeout'] and r['reaped'] and not r['kill_sent'] and r['source_head'] == state['native_source_commit'] for r in native), 'fresh native validation not complete')
    counts = {}
    for label, flags in [('test', ['-count=1', '-timeout=18m']), ('race', ['-race', '-count=1', '-timeout=20m'])]:
        receipt = next(r for r in native if r['label'] == label)
        require(receipt['argv'][-1] == './...' and all(f in receipt['argv'] for f in flags) and '-run' not in receipt['argv'], 'filtered/cached native test run')
        events = [json.loads(s) for s in log(packet, 'native-isolated', label, 'stdout').splitlines() if s.startswith('{')]
        observed = {a: sum(r.get('Action') == a and bool(r.get('Test')) for r in events) for a in ['pass', 'fail', 'skip']}
        require(observed == receipt['tests'] and observed['pass'] > 4000 and observed['fail'] == 0, 'raw native test totals')
        require(any(r.get('Action') == 'pass' and r.get('Test') == 'TestHostCompatibleMetadataRetainsExclusiveSelectorRuntime' for r in events), 'new runtime selector test missing')
        require(not any(r.get('Action') == 'fail' for r in events), 'native package/test failure hidden')
        counts[label] = observed
    for r in native:
        for stream in ['stdout', 'stderr']:
            data = gzip.decompress((packet / 'validation/native-isolated' / (r['label'] + '.' + stream + '.gz')).read_bytes())
            require(hashlib.sha256(data).hexdigest() == r[stream + '_sha256'], 'native raw log association')
    initial = read(packet / 'validation/native-initial/receipts.json')
    require(initial[0]['exit_code'] == 1 and 'no such file or directory' in log(packet, 'native-initial', 'build', 'stderr'), 'initial shared-cache failure omitted')
    consumer = read(packet / 'validation/consumer/receipts.json')
    require(len(consumer) == 26 and all(r['exit_code'] == 0 and not r['timeout'] and r['reaped'] for r in consumer), 'consumer functional command inventory')
    require(len(log(packet, 'consumer', 'fmt', 'stdout').splitlines()) == 4, 'first consumer formatting failure omitted')
    fixes = read(packet / 'validation/consumer-format-recheck/receipts.json')
    require([r['label'] for r in fixes] == ['fmt', 'jira-build', 'jira-test', 'jira-race', 'jira-vet', 'deep-review-CI', 'role-contract-CI'] and all(r['exit_code'] == 0 for r in fixes), 'consumer corrective checks incomplete')
    require(not log(packet, 'consumer-format-recheck', 'fmt', 'stdout').strip(), 'consumer formatting still drifts')
    python_tests = 0
    import re
    for r in consumer:
        if r['label'] in ['tool-names', 'test_resolve_project', 'hooks']:
            text = log(packet, 'consumer', r['label'], 'stdout') + log(packet, 'consumer', r['label'], 'stderr')
            require(('OK: 137 tool reference(s)' in text) if r['label'] == 'tool-names' else 'ALL PASS' in text, 'consumer script contract failed')
        elif r['argv'][0] == 'python3' and r['label'] not in ['oracle-smoke', 'selection-smoke', 'evaluation-exit']:
            text = log(packet, 'consumer', r['label'], 'stdout') + log(packet, 'consumer', r['label'], 'stderr')
            matches = re.findall(r'Ran (\d+) tests?', text)
            require(matches and '\nOK' in text, 'consumer Python test summary absent')
            python_tests += int(matches[-1])
    pins = read(packet / 'binary-pins.json')
    require(pins['cks_commit'] == state['native_source_commit'] and pins['consumer_commit'] == state['consumer_host_executed_commit'], 'executed source/binary pin')
    for name in ['cks', 'ckg']:
        info = (packet / (name + '-build-info.txt')).read_text()
        require('vcs.revision=' + state['native_source_commit'] in info and 'vcs.modified=false' in info, 'fresh binary clean source revision')
    require(state['consumer_host_plugin_tree'] == state['consumer_final_plugin_tree'] and state['consumer_final_commit'] != state['consumer_host_executed_commit'], 'post-format/CI plugin equality')
    pairset, pids, host_requests = set(), set(), 0
    for case, role, expected in [('analyzer', 'analyzer', 5), ('planner-tools', 'planner', 2), ('planner-delivery', 'planner', 0)]:
        folder = packet / 'hosts' / case
        inv, end = read(folder / 'invocation.json'), read(folder / 'exit.json')
        argv = inv['argv']
        require(inv['role'] == role and argv[argv.index('--agent') + 1] == 'coding-agent:' + role and inv['cks_sha256'] == pins['cks_sha256'], 'actual role/new binary association')
        require(inv['driver_sha256'] == sha(packet / 'helpers/host-driver.py.txt') and '--strict-mcp-config' in argv and argv[argv.index('--setting-sources') + 1] == '' and '--dangerously-skip-permissions' not in argv, 'executed helper/settings boundary')
        require(inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:') and inv['api_credential_kind'] == 'constant_fake_local_token', 'real provider/auth used')
        require(end['host_exit'] == 0 and end['host_reaped'] and not end['forced'] and not end['rescue'] and end['cleanup']['all_observed_identities_gone'] and end['gateway_cleanup']['all_handler_threads_joined'] and end['gateway_cleanup']['server_socket_closed'], 'actual host cleanup')
        pids.add(end['host_pid'])
        frames = rows(folder / 'mcp-frames.jsonl')
        starts, exits = [r for r in frames if r['kind'] == 'start'], [r for r in frames if r['kind'] == 'exit']
        require(len(starts) == len(exits) == 1 and starts[0]['binary_sha256'] == pins['cks_sha256'] and exits[0]['exit_code'] == 0 and exits[0]['child_reaped'] and exits[0]['output_workers_joined'] and exits[0]['signal_worker_joined'] and not exits[0]['forced_kill'], 'actual CKS exit/reap')
        pids.update([starts[0]['wrapper_pid'], starts[0]['child_pid']])
        requests = [json.loads(r['raw']) for r in frames if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call']
        responses = {json.loads(r['raw']).get('id'): json.loads(r['raw']) for r in frames if r['kind'] == 'response'}
        require(len(requests) == expected, 'actual host request denominator')
        gateway = read(folder / 'gateway-requests.json')
        scripted = [r for r in gateway if 'scripted_response' in r]
        actions = [a for r in scripted for a in r['scripted_response']['content'] if a['type'] == 'tool_use']
        results = {v['tool_use_id']: v for r in scripted for m in r['body']['messages'] if m['role'] == 'user' for v in m.get('content', []) if isinstance(v, dict) and v.get('type') == 'tool_result'}
        body = (packet / 'consumer-source' / ('agents/' + role + '.md.txt')).read_text().split('---', 2)[2].strip().replace('${CLAUDE_PLUGIN_ROOT}', argv[argv.index('--plugin-dir') + 1])
        require(any(body in '\n'.join(x.get('text', '') for x in r['body'].get('system', [])) for r in scripted), 'actual role source not loaded')
        tool_actions = [a for a in actions if a['name'].startswith('mcp__')]
        require(len(tool_actions) == expected and all(a['id'] in results for a in actions), 'actual API result denominator')
        for q, a in zip(requests, tool_actions):
            require(a['name'] == 'mcp__plugin_coding-agent_cks__' + q['params']['name'].replace('.', '_') and a['input'] == q['params']['arguments'], 'API/wire tool association')
            require(not results[a['id']].get('is_error') and 'error' not in responses[q['id']] and not responses[q['id']]['result'].get('isError'), 'actual role tool error')
            pairset.add((role, q['params']['name']))
        if case == 'planner-delivery':
            writes = [a for a in actions if a['name'] == 'Write']
            require(len(writes) == 2 and results[writes[0]['id']].get('is_error') is True and 'Structured review delivery blocked: review delivery report differs from typed artifact' in str(results[writes[0]['id']]['content']) and not results[writes[1]['id']].get('is_error'), 'tamper refusal/normal delivery')
            workspace = folder / 'workspace-after'
            require(read(workspace / 'shell-tamper-receipt.json')['exit_code'] == 2 and read(workspace / 'shell-restore-receipt.json')['exit_code'] == 0 and read(workspace / 'state.json')['current_state'] == 'COMPLETED', 'CLI refusal/restore/state')
            report = read(workspace / 'deep-review.json')
            require(report['independent_acceptance'] == 'not_recorded' and report['status'] == 'structured_partial_review' and sum(x['status'] == 'unknown' for x in report['reasoning']['sections'].values()) == 5, 'partial/unknown promoted')
        host_requests += expected
    expected_pairs = {('analyzer', 'cks.context.' + x) for x in ['value_dependencies', 'rule_candidates', 'value_dependency_expansion', 'policy_security', 'review_function']} | {('planner', 'cks.context.' + x) for x in ['rule_candidates', 'review_function']}
    require(pairset == expected_pairs, 'seven actual role/tool pairs')
    matrix = {}
    for side in ['old', 'new']:
        folder = packet / 'native-matrix' / side
        messages, life = read(folder / 'messages.json'), read(folder / 'lifecycle.json')
        require(life['tool_requests'] == 47 and life['exit_code'] == 0 and life['child_reaped'] and life['reader_joined'] and not life['kill_sent'] and life['binary_sha256_before'] == life['binary_sha256_after'], 'native comparison cleanup/pin')
        pids.add(life['pid'])
        if side == 'new':
            require(life['binary_sha256_before'] == pins['cks_sha256'], 'new native binary')
        calls = [r for r in messages if r['request'].get('method') == 'tools/call']
        require(len(calls) == 47 and any('"node_id":"a","node_id":"b"' in r.get('request_raw', '') for r in calls), 'duplicate raw selector proof')
        matrix[side] = {r['label']: normalize(r['response']) for r in calls}
    require(matrix['old'] == matrix['new'], 'runtime behavior mismatch beyond timing')
    audit = read(packet / 'postrun-process-audit.json')
    require({r['pid'] for r in audit['probes']} == pids and all(r['exit_code'] == 1 and not r['stdout'].strip() for r in audit['probes']) and audit['self']['exit_code'] == 0, 'owned PID audit')
    hist = read(packet / 'historical-regression-results.json')
    require(len(hist) == 19 and sum(r['expected'] for r in hist) == 667 and all(r['ok'] and r['expected'] == r['observed'] and r['exit_code'] == 0 for r in hist), 'historical guards failed')
    import re
    for row in hist:
        raw = gzip.decompress((packet / 'validation/historical' / (row['name'] + '.log.gz')).read_bytes()).decode()
        observed = re.search(r'Ran (\d+) tests?', raw)
        require(observed and int(observed.group(1)) == row['observed'] and '\nOK' in raw, 'historical raw accounting')
    require(state['historical_source_ref'] == 'ba6d1d22b3f95a6f3b3948717f9d0ea767870bee' and state['historical_results_are_current_source_tests'] is False, 'historical results mislabelled as current')
    scope = read(packet / 'scope-preservation.json')
    require(scope['global_configuration_equal'] and scope['operational_metadata_pointers_equal'] and scope['original_unowned_engines_preserved'] and scope['old_source_checkouts_unchanged'] and scope['production_writes'] == 0, 'protected scope changed')
    require(scope['extra_unowned_engines_are_not_terminated'] is True and scope['historical_unconfirmed_CKS_exit_receipts'] == 1, 'unowned/historical boundary')
    return {'status': 'local_source_adoption_and_fresh_validation_pass', 'native_test': counts['test'], 'native_race': counts['race'], 'consumer_python_tests': python_tests, 'consumer_functional_commands': 25, 'consumer_corrective_checks': 7, 'consumer_initial_format_failure_retained': True, 'native_initial_shared_cache_failure_retained': True, 'actual_role_tool_pairs': 7, 'actual_hosts': 3, 'host_CKS_requests': host_requests, 'native_comparison_requests': 94, 'CKS_exit0_reaped': 5, 'owned_PID_audit_count': len(pids), 'CLI_tamper_refusal': 1, 'actual_Write_tamper_refusal': 1, 'restored_normal_delivery': 1, 'historical_guard_tests': 667, 'historical_guards_source': state['historical_source_ref'], 'new_quality_or_ontology_measurement': False, 'formal_completed': 0, 'formal_total': 9, 'reviewer_assignment': '미정', 'production_activation': False, 'real_model_calls': 0, 'external_publication': 0}


def check(packet, live=True):
    try:
        manifest = read(packet / 'manifest.json')
        require(manifest['checker_sha256'] == sha(ROOT / manifest['checker']), 'checker source differs')
        actual = {str(p.relative_to(packet)) for p in packet.rglob('*') if p.is_file()}
        require(actual == set(manifest['artifacts']) | {'manifest.json'}, 'packet inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(packet / name) == digest, 'artifact hash: ' + name)
        require(read(packet / 'adoption.json') == derive(packet, live), 'derived adoption accounting')
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration, subprocess.SubprocessError) as error:
        return [str(error)]


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--packet', type=Path, required=True)
    parser.add_argument('--repository', type=Path)
    parser.add_argument('--no-live-source', action='store_true')
    args = parser.parse_args()
    if args.repository:
        ROOT = args.repository.resolve()
    errors = check(args.packet.resolve(), not args.no_live_source)
    for error in errors:
        print(error)
    if not errors:
        print('approved local adoption verified: fresh native/consumer checks and 7 pairs; formal 0/9; no production/publication')
    raise SystemExit(bool(errors))
