#!/usr/bin/env python3
"""Freeze and derive T04 workflow evidence without promoting scripted selections."""
import argparse
import ast
import importlib.util
import io
import json
import os
from functools import lru_cache
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
from source_packet import SourceCapture, safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR = BASE / 'host-schema-compatibility-v1'
P07 = BASE / 'followups/consumer-render-lifecycle-v1'
PACKET = BASE / 'followups/consumer-opt-in-v1'
INPUTS = [Path('scripts/fixtures/consumer-opt-in-plan.json'), Path('scripts/run-consumer-opt-in.py'), PRIOR / 'manifest.json', P07 / 'manifest.json', *[Path('scripts') / n for n in ['consumer-compatible-host.py', 'consumer-lifecycle-proxy.py', 'consumer-process-ownership.py', 'consumer-role-adapter.py', 'run-consumer-lifecycle.py', 'operational-host-probe.py', 'prepare-consumer-stdio-readiness.py', 'prepare-source-review-supplement.py', 'source_packet.py']]]
CASES = ['feature-default', 'feature-value', 'review-build', 'analyzer-reuse', 'planner-delivery', 'planner-reuse', 'duplicate-control']
TOP = ['invocation.json', 'mcp.json', 'settings.json', 'scripted-actions.json', 'workspace-before.json', 'workspace-after.json', 'gateway-requests.json', 'host.debug.txt', 'host.stdout.jsonl', 'host.stderr.txt', 'mcp-frames.jsonl', 'hook-inputs.jsonl', 'exit.json']
ARTIFACTS = ['deep-review-evidence.json', 'deep-review.json', 'review-report.md', 'sequence.mmd', 'modules.mmd']


def require(value, message):
    if not value:
        raise ValueError(message)


def read(path):
    return json.loads(path.read_text())


def lines(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def dump(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def digest(path):
    return sha(path.read_bytes())


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


@lru_cache(maxsize=1)
def historical_expected():
    old = module('prepare-consumer-render-lifecycle.py').preservation()
    keep = dict(old['files'])
    manifest = read(ROOT / P07 / 'manifest.json')
    names = [P07 / 'manifest.json', *[P07 / name for name in manifest['artifacts']], *[Path(n) for n in manifest['input_bindings']], Path('scripts/prepare-consumer-render-lifecycle.py'), Path('scripts/check-consumer-render-lifecycle.py'), Path('scripts/test_consumer_render_lifecycle.py'), BASE / 'followups/.gitattributes']
    archive = subprocess.check_output(['git', 'archive', '9ede28200343434d250f4a53798ef5409b07552a', *sorted({str(n) for n in names})], cwd=ROOT)
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        for entry in tar:
            if entry.isfile():
                keep[entry.name] = sha(tar.extractfile(entry).read())
    return {'base_commit': '9ede28200343434d250f4a53798ef5409b07552a', 'prior_packets': 31, 'files': keep}


def preservation():
    expected = historical_expected()
    for name, value in expected['files'].items():
        require(digest(safe(ROOT, name)) == value, 'historical bytes changed: ' + name)
    return expected


def saved(folder, name):
    return safe(folder / 'workspace-after', name + '.txt' if name.endswith('.md') else name)


def normalized(value):
    if isinstance(value, dict):
        return {k: normalized(v) for k, v in value.items() if k != 'elapsed_micros'}
    if isinstance(value, list):
        return [normalized(v) for v in value]
    return value


def calls(folder):
    frames = lines(folder / 'mcp-frames.jsonl')
    requests = [json.loads(r['raw']) for r in frames if r['kind'] == 'request']
    responses = {json.loads(r['raw']).get('id'): json.loads(r['raw']) for r in frames if r['kind'] == 'response'}
    return [r for r in requests if r.get('method') == 'tools/call'], responses, frames


def consumer():
    pins = read(ROOT / PRIOR / 'binary-pins.json')
    source = ROOT / PRIOR / 'consumer-source'
    proof = verify(source, pins['consumer_commit'])
    def text(name):
        return safe(source, proof['files'][name]['artifact']).read_text()
    raw = text('plugin/scripts/deep_review.py')
    ns = {'__name__': 'pinned_t04_consumer'}
    exec(compile(raw, '<pinned T04 consumer>', 'exec'), ns)
    tree = ast.parse(raw)
    projection = next(n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == 'computed_view')
    require(not any(isinstance(n, ast.Call) for n in ast.walk(projection)), 'consumer projection executes another engine')
    for role in ['analyzer', 'planner']:
        content = text('plugin/agents/' + role + '.md')
        require('go-local-review@1.0.0' in content and 'reuse' in content.lower() and 'code_review' in content and 'rule_candidates' in content, 'explicit role contract missing')
    require('never compute a parallel graph' in text('plugin/agents/analyzer.md'), 'parallel consumer graph prohibited')
    return ns, text


def derive(packet):
    pins = read(ROOT / PRIOR / 'binary-pins.json')
    ns, source = consumer()
    execution = read(packet / 'execution.json')
    require(execution['driver_sha256'] == digest(ROOT / 'scripts/run-consumer-opt-in.py') and execution['plan_sha256'] == digest(ROOT / 'scripts/fixtures/consumer-opt-in-plan.json'), 'executed helper/plan pin')
    require(execution['host_cases'] == CASES and execution['candidate_adopted'] is False and execution['real_model_calls'] == execution['formal_runs'] == 0 and execution['scripted_selection_not_model_adherence'] is True, 'execution/adoption scope')
    require(read(packet / 'formal-state.json') == module('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
    require(read(packet / 'preservation.json') == preservation(), 'prior packet inventory')
    native = read(packet / 'native/messages.json')
    require([r['label'] for r in native] == ['initialize', 'missing-pack', 'unversioned-pack', 'unknown-pack', 'missing-function', 'baseline', 'explicit', 'explicit-repeat'], 'native cases/denominator')
    stdio = module('prepare-consumer-stdio-readiness.py')
    for row in native[1:5]:
        require(stdio.refused(row['response']), 'invalid opt-in accepted')
    for row in native[5:]:
        require(not stdio.refused(row['response']), 'positive native error')
    a, b, repeated = [stdio.payload(r['response'])['analysis'] for r in native[5:]]
    require(not a.get('rules') and b['rules']['status'] == 'matched_bounded', 'ordinary query automatically ran rules')
    require(normalized({k: v for k, v in a.items() if k != 'rules'}) == normalized({k: v for k, v in b.items() if k != 'rules'}), 'base/value view differs beyond timing/rules')
    require(normalized(b) == normalized(repeated), 'explicit repeated rule response differs')
    query = execution['query']
    for row in native[6:]:
        require(row['request']['params'] == {'name': 'cks.context.rule_candidates', 'arguments': {'rule_pack': 'go-local-review@1.0.0', 'function': query}}, 'native explicit selection')
    life = read(packet / 'native/lifecycle.json')
    require(life['exit_code'] == 0 and life['child_reaped'] and life['eof_sent'] and life['reader_joined'] and not life['kill_sent'] and not life['terminate_sent'] and life['tool_requests'] == 7, 'native cleanup')
    sent = lines(packet / 'native/stdin.jsonl')
    require(sent[1] == {'jsonrpc': '2.0', 'method': 'notifications/initialized'} and sent[:1] + sent[2:] == [r['request'] for r in native] and lines(packet / 'native/stdout.jsonl') == [r['response'] for r in native], 'native raw wire association')
    native_proof = verify(packet / 'native-source', pins['cks_commit'])
    def go(name):
        return safe(packet / 'native-source', native_proof['files']['pkg/graph/valueflow/' + name]['artifact']).read_text()
    require('return query(ctx, r, q, nil)' in go('query.go') and go('query.go').count('result, e := build(bounded, source, blob, opts, true, rules)') == 1, 'shared native query build')
    require('return query(ctx, r, q.Function, &o)' in go('rules.go') and 'return build(ctx, source, blob, limits, true, &o)' in go('rules.go'), 'native rule opt-in shared path')
    require(go('cfg.go').count('parser.ParseFile(') == 1 and go('cfg.go').count('a.matchRules(*rules)') == 1 and 'if rules != nil {' in go('cfg.go'), 'native shared parse/rule path')
    summaries = []
    all_pids = {life['pid']}
    expected_counts = [0, 1, 2, 0, 0, 0, 3]
    saved_hashes = None
    for case, count in zip(CASES, expected_counts):
        folder = packet / 'hosts' / case
        inv, end = read(folder / 'invocation.json'), read(folder / 'exit.json')
        argv = inv['argv']
        role = 'planner' if case.startswith('planner-') else 'analyzer'
        require(inv['role'] == role and argv[argv.index('--agent') + 1] == 'coding-agent:' + role and inv['case'] == case, 'actual role identity')
        require(inv['driver_sha256'] == execution['driver_sha256'] and inv['cks_sha256'] == pins['cks_sha256'] and inv['host_sha256'] == '475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3', 'host/helper/binary pins')
        require(inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:') and inv['api_credential_kind'] == 'constant_fake_local_token', 'nonlocal provider')
        require(argv[argv.index('--setting-sources') + 1] == '' and '--strict-mcp-config' in argv and '--dangerously-skip-permissions' not in argv and '--bare' not in argv, 'settings/permission boundary')
        require(end['host_exit'] == 0 and end['host_reaped'] and not end['forced'] and end['elapsed_seconds'] < 90 and not end['rescue'] and end['cleanup']['all_observed_identities_gone'], 'host cleanup failure')
        require(end['real_model_calls'] == end['formal_runs'] == 0, 'formal/model promotion')
        g = end['gateway_cleanup']
        require(g['all_handler_threads_joined'] and g['server_socket_closed'] and g['server_thread_joined'] and not g['errors'], 'responder cleanup failure')
        requests, responses, frames = calls(folder)
        require(len(requests) == count, 'extra/missing CKS requests: ' + case)
        owner = next(r for r in frames if r['kind'] == 'owner')
        start = [r for r in frames if r['kind'] == 'start']
        exits = [r for r in frames if r['kind'] == 'exit']
        require(owner['owner'] == inv['owner'] and owner['driver_sha256'] == execution['driver_sha256'] and owner['proxy_sha256'] == digest(ROOT / 'scripts/consumer-lifecycle-proxy.py'), 'owned proxy association')
        require(len(start) == len(exits) == 1 and start[0]['child_pid'] == exits[0]['child_pid'] and start[0]['binary_sha256'] == pins['cks_sha256'], 'CKS denominator/pin')
        child = exits[0]
        require(child['exit_code'] == 0 and child['child_reaped'] and not child['forced_kill'] and not child['terminate_sent'] and child['signal_worker_joined'] and child['output_workers_joined'], 'CKS exit/reap proof')
        all_pids.update([end['host_pid'], start[0]['wrapper_pid'], start[0]['child_pid']])
        gateway = read(folder / 'gateway-requests.json')
        main = [r for r in gateway if 'scripted_response' in r]
        plugin = argv[argv.index('--plugin-dir') + 1]
        require(plugin == '/private/tmp/ks-host-schema-consumer-20261017/plugin', 'unexpected plugin source')
        role_body = source('plugin/agents/' + role + '.md').split('---', 2)[2].strip().replace('${CLAUDE_PLUGIN_ROOT}', plugin)
        require(any(role_body in '\n'.join(x.get('text', '') for x in row['body'].get('system', [])) for row in main), 'actual role body not loaded')
        actions = [v for r in main for v in r['scripted_response']['content'] if v['type'] == 'tool_use']
        declared = read(folder / 'scripted-actions.json')
        require(len(declared) == len(actions), 'scripted/API action denominator')
        resolutions = lines(folder / 'action-resolutions.jsonl') if (folder / 'action-resolutions.jsonl').exists() else []
        for index, (selection, action) in enumerate(zip(declared, actions)):
            if 'state_operation' in selection:
                resolved = next(r for r in resolutions if r['index'] == index)
                require(resolved['operation'] == selection['state_operation'] and action['name'] == 'Write' and json.loads(action['input']['content']) == resolved['proposed'], 'actual state action resolution')
            else:
                require(selection == {'name': action['name'], 'input': action['input']}, 'scripted/API action differs')
        tool_actions = [a for a in actions if a['name'].startswith('mcp__')]
        require(len(tool_actions) == len(requests), 'API/wire tool count')
        for action, request in zip(tool_actions, requests):
            require(action['name'] == 'mcp__plugin_coding-agent_cks__' + request['params']['name'].replace('.', '_') and action['input'] == request['params']['arguments'], 'API/wire selection differs')
            require(request['params']['_meta']['claudecode/toolUseId'] == action['id'], 'host toolUseId/wire association')
            require(not stdio.refused(responses[request['id']]), 'actual role MCP error')
        results = {v['tool_use_id']: v for r in main for m in r['body']['messages'] if m['role'] == 'user' for v in m.get('content', []) if isinstance(v, dict) and v.get('type') == 'tool_result'}
        require(all(action['id'] in results for action in actions), 'actual tool result missing')
        after, before = read(folder / 'workspace-after.json'), read(folder / 'workspace-before.json')
        for name, value in after.items():
            require(digest(saved(folder, name)) == value['sha256'], 'saved workspace hash')
        if case in ['feature-default', 'feature-value']:
            require(before == after and set(after) == {'state.json'} and after['state.json']['json']['ticket_type'] == 'feature', 'non-review generated rule artifacts')
            if case == 'feature-value':
                require(requests[0]['params']['name'] == 'cks.context.value_dependencies' and requests[0]['params']['arguments'] == query and not stdio.payload(responses[requests[0]['id']])['analysis'].get('rules'), 'ordinary role query ran rules')
        elif case == 'duplicate-control':
            require([r['params']['name'] for r in requests] == ['cks.context.rule_candidates', 'cks.context.rule_candidates', 'cks.context.review_function'], 'duplicate control missing actual attempts')
            result = results[actions[-1]['id']]
            require(result.get('is_error') is True and 'Exit code 1' in str(result['content']) and 'one actual role call required: rule_candidates' in str(result['content']), 'fixture adapter duplicate refusal missing')
            require(before == after and set(after) == {'state.json'}, 'duplicate adapter refusal produced delivery')
        else:
            workspace = folder / 'workspace-after'
            with tempfile.TemporaryDirectory(prefix='ks-t04-verify-') as temp:
                materialized = Path(temp) / 'workspace'
                shutil.copytree(workspace, materialized)
                (materialized / 'review-report.md.txt').rename(materialized / 'review-report.md')
                validation = ns['verify_workspace'](materialized)
            bundle, report = read(workspace / 'deep-review-evidence.json'), read(workspace / 'deep-review.json')
            analysis = bundle['rules']['result']['analysis']
            require(report['computed'] == ns['computed_view'](analysis), 'consumer recomputed/substituted graph')
            hashes = {n: after[n]['sha256'] for n in ARTIFACTS}
            if case == 'review-build':
                saved_hashes = hashes
                require([r['params']['name'] for r in requests] == ['cks.context.rule_candidates', 'cks.context.review_function'], 'review duplicate/extra query')
                for request, key in zip(requests, ['rules', 'function_review']):
                    require(bundle[key] == {'tool': request['params']['name'], 'arguments': request['params']['arguments'], 'result': stdio.payload(responses[request['id']])}, 'bundle not real role receipt')
                require(bundle['rules']['arguments'] == {'rule_pack': 'go-local-review@1.0.0', 'function': query}, 'review pack/function not explicit')
            else:
                require(hashes == saved_hashes and all(before[n]['sha256'] == after[n]['sha256'] for n in ARTIFACTS), 're-entry regenerated artifacts')
                verify_actions = [a for a in actions if a['name'] == 'Bash']
                require(len(verify_actions) == 1 and 'deep_review.py verify --workspace' in verify_actions[0]['input']['command'], 're-entry verifier not executed')
                outcome = results[verify_actions[0]['id']]
                require(not outcome.get('is_error') and 'validated_delivery' in str(outcome['content']) and validation['deep_review_sha256'] in str(outcome['content']), 're-entry CLI did not validate same artifact')
                if case != 'planner-delivery':
                    require(before == after, 'reuse-only workspace changed')
            if case.startswith('planner-'):
                state = after['state.json']['json']
                require(state['states']['PLANNING']['status'] == 'completed' and state['states']['PLANNING']['review_report_sha256'] == hashes['review-report.md'], 'Planner hash delivery')
        summaries.append({'case': case, 'role': role, 'CKS_calls': count, 'host_exit': 0, 'CKS_exit': 0, 'fixture_adapter_expected_error': case == 'duplicate-control'})
    audit = read(packet / 'postrun-process-audit.json')
    require(set(r['pid'] for r in audit['probes']) == all_pids and len(audit['probes']) == len(all_pids), 'owned PID audit denominator')
    require(all(r['exit_code'] == 1 and not r['stdout'].strip() for r in audit['probes']), 'owned process still present/audit failed')
    require(audit['self']['exit_code'] == 0 and str(audit['pid']) in audit['self']['stdout'], 'audit live positive control')
    integrity = read(packet / 'source-integrity.json')
    require(all(not row['porcelain'] for row in integrity['checkouts'].values()), 'candidate checkout changed')
    expected = read(ROOT / PRIOR / 'execution.json')
    for key, row in integrity['checkouts'].items():
        require(row == expected[key], 'candidate/original source pin')
    for name, value in integrity['index_sha256'].items():
        require(value == digest(ROOT / BASE / 'consumer-stdio-readiness-v1/index' / name), 'owned index changed')
    require(integrity['fixture_tracked_changes'] == '' and integrity['fixture_commit'] == read(ROOT / BASE / 'consumer-stdio-readiness-v1/execution.json')['fixture_commit'], 'fixture source changed')
    return {'test_id': 'T04', 'status': 'isolated_workflow_pass_not_model_adherence_or_adoption', 'host_sessions': summaries, 'native_tool_requests': 7, 'native_invalid_refusals': 4, 'actual_role_CKS_requests': 6, 'all_CKS_requests': 13, 'CKS_processes_exit0_reaped': 8, 'reused_artifact_sha256': saved_hashes, 'audited_owned_pids_absent': len(all_pids), 'duplicate_native_requests_allowed': True, 'duplicate_fixture_adapter_refusals': 1, 'consumer_graph_projection_verified': True, 'server_cache_or_duplicate_blocker_verified': False, 'autonomous_model_adherence': 'not_evaluated', 'formal_completed': 0, 'formal_total': 9, 'ontology_effect_acceptance': 'not_evaluated', 'candidate_adopted': False, 'real_model_calls': 0, 'formal_runs': 0, 'full_plan_passed': 18, 'full_plan_pending': ['T09', 'T10', 'T16', 'T22'], 'next': 'T09 foreign/pins, then T10/T16 and P08/T22'}


def prepare(owned, output):
    require(not output.exists(), 'fresh output required')
    output.mkdir(parents=True)
    for case in CASES:
        target = output / 'hosts' / case
        target.mkdir(parents=True)
        for name in TOP:
            shutil.copyfile(owned / case / name, target / name)
        for name in ['action-resolutions.jsonl']:
            if (owned / case / name).exists():
                shutil.copyfile(owned / case / name, target / name)
        shutil.copytree(owned / case / 'workspace-after', target / 'workspace-after')
        # Markdown is retained as source evidence, never a new live document.
        md = target / 'workspace-after/review-report.md'
        if md.exists():
            md.rename(md.with_suffix('.md.txt'))
    shutil.copytree(owned / 'native', output / 'native')
    shutil.copyfile(owned / 'execution.json', output / 'execution.json')
    capture = SourceCapture(Path('/private/tmp/ks-host-schema-cks-20261017'), output / 'native-source', read(ROOT / PRIOR / 'binary-pins.json')['cks_commit'])
    for name in ['rules.go', 'query.go', 'cfg.go']:
        capture.source('pkg/graph/valueflow/' + name)
    dump(output / 'native-source/git-proof.json', capture.proof())
    dump(output / 'formal-state.json', module('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output / 'preservation.json', preservation())
    old = read(ROOT / PRIOR / 'execution.json')
    repos = {key: Path(path) for key, path in [('cks_postrun', '/private/tmp/ks-host-schema-cks-20261017'), ('consumer_postrun', '/private/tmp/ks-host-schema-consumer-20261017'), ('original_postrun', '/private/tmp/ks-isolated-host-20261015/coding-agent'), ('prior_candidate_postrun', '/private/tmp/ks-consumer-contract-candidate-20261017')]}
    integrity = {'checkouts': {key: {'commit': subprocess.check_output(['git', '-C', str(path), 'rev-parse', 'HEAD'], text=True).strip(), 'porcelain': subprocess.check_output(['git', '-C', str(path), 'status', '--porcelain'], text=True)} for key, path in repos.items()}, 'index_sha256': {n: digest(owned / 'graph' / n) for n in ['graph.db', 'manifest.json']}, 'fixture_commit': subprocess.check_output(['git', '-C', str(owned / 'source'), 'rev-parse', 'HEAD'], text=True).strip(), 'fixture_tracked_changes': subprocess.check_output(['git', '-C', str(owned / 'source'), 'status', '--porcelain', '--untracked-files=no'], text=True)}
    require(all(integrity['checkouts'][key] == old[key] for key in repos), 'source checkout changed')
    dump(output / 'source-integrity.json', integrity)
    pids = {read(owned / 'native/lifecycle.json')['pid']}
    for case in CASES:
        pids.add(read(owned / case / 'exit.json')['host_pid'])
        for row in lines(owned / case / 'mcp-frames.jsonl'):
            if row['kind'] == 'start':
                pids.update([row['wrapper_pid'], row['child_pid']])
    def ps(pid):
        command = ['ps', '-p', str(pid), '-o', 'pid=,ppid=,stat=,lstart=,command=']
        q = subprocess.run(command, capture_output=True, text=True)
        return {'pid': pid, 'command': command, 'exit_code': q.returncode, 'stdout': q.stdout, 'stderr': q.stderr}
    dump(output / 'postrun-process-audit.json', {'pid': os.getpid(), 'self': ps(os.getpid()), 'probes': [ps(pid) for pid in sorted(pids)]})
    result = derive(output)
    dump(output / 'opt-in.json', result)
    (output / 'REPORT.txt').write_text('준비32/T04 — opt-in·재사용·중복 처리\n실제7host/8CKS exit0/reap, native7/role6=13요청. 잘못된 opt-in4건 거부, 일반 모드 자동rule0, review rule1/review1. Analyzer/Planner 검증3회는 CKS0·5산출물 해시 유지.\n중복 요청은 native/host에서 허용한다. 시험용 어댑터만 중복 rule2건을 거부하며 exit1을 보존했다. 자율 모델 준수·서버 캐시/중복 차단·의미 품질/온톨로지 효과는 미평가다.\n이전31패킷·현재 runtime/schema 보존. 계획18/22, 남은 T09/T10/T16/T22. 정식0/9·담당자미정·freeze null·실제모델0·후보미채택.\n')
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    (output / '.gitattributes').write_text('native-source/objects/*.commit binary\nnative-source/objects/*.tree binary\n')
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'T04_owned_opt_in_not_model_or_formal_acceptance', 'input_bindings': {str(p): digest(ROOT / p) for p in INPUTS}, 'generator_sha256': digest(Path(__file__)), 'artifacts': {p.relative_to(output).as_posix(): digest(p) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('T04 frozen:7 hosts/13 calls; plan18/22; formal0/9')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--owned', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.owned.resolve(), args.output.resolve())
