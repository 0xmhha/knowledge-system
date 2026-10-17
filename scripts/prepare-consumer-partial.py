#!/usr/bin/env python3
"""Freeze T10 supported partials and refused incomplete/unknown promotion."""
import argparse
import copy
from functools import lru_cache
import io
import json
import os
from pathlib import Path
import shutil
import sqlite3
import subprocess
import tarfile
import importlib.util
from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PREVIOUS = BASE / 'followups/consumer-foreign-pin-v1'
PACKET = BASE / 'followups/consumer-partial-v1'
BASE_COMMIT = '0821b6c1d3d42b16245e3fed43161c16bab68010'


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


old = module('prepare-consumer-foreign-pin.py')
driver = module('run-consumer-partial.py')
read, dump, lines, require, digest = old.read, old.dump, old.lines, old.require, old.digest
CASES = ['T10-build-seed', 'T10-matrix', 'T10-legacy', 'T10-build-field', 'T10-controls', 'T10-field-delivery']
CONTROLS = ['unsupported', 'truncated-real', 'incomplete-rules', 'requires-reindex', 'all-unknown', 'missing-section', 'truncated-flag', 'unknown-observed', 'unknown-empty', 'safe-status']
REASONS = ['incomplete/unavailable base analysis cannot finish review', 'incomplete/unavailable base analysis cannot finish review', 'unsupported/incomplete rules cannot finish review', 'structural review identity/location differs', 'all-unknown placeholder cannot complete a deep review', 'missing/unknown fields: data_state,guards_exceptions,complexity,resources,io,priority_tradeoffs', 'truncated/wrong base analysis', 'invalid reasoning section resources', 'unknown section requires limitations', 'report/source/computed/origin/limitations changed']
INPUTS = [PREVIOUS / 'manifest.json', Path('scripts/fixtures/consumer-partial-plan.json'), Path('scripts/run-consumer-partial.py'), *old.INPUTS, Path('scripts/prepare-consumer-foreign-pin.py'), Path('scripts/check-consumer-foreign-pin.py'), Path('scripts/test_consumer_foreign_pin.py')]


@lru_cache(maxsize=1)
def expected_preservation():
    keep = dict(old.preservation()['files'])
    manifest = read(ROOT / PREVIOUS / 'manifest.json')
    names = [str(PREVIOUS / 'manifest.json'), *[str(PREVIOUS / f) for f in manifest['artifacts']], 'scripts/prepare-consumer-foreign-pin.py', 'scripts/check-consumer-foreign-pin.py', 'scripts/test_consumer_foreign_pin.py']
    archive = subprocess.check_output(['git', 'archive', BASE_COMMIT, *sorted(set(names))], cwd=ROOT)
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        for entry in tar:
            if entry.isfile():
                keep[entry.name] = sha(tar.extractfile(entry).read())
    return {'base_commit': BASE_COMMIT, 'prior_packets': 33, 'files': keep}


def preservation():
    expected = expected_preservation()
    for name, value in expected['files'].items():
        require(digest(safe(ROOT, name)) == value, 'prior packet/source changed: ' + name)
    return expected


def host(folder, case, execution, pins, text, count):
    inv, end = read(folder / 'invocation.json'), read(folder / 'exit.json')
    argv = inv['argv']
    role = 'planner' if case in ['T10-controls', 'T10-field-delivery'] else 'analyzer'
    require(inv['case'] == case and inv['role'] == role and inv['driver_sha256'] == execution['host_helper_sha256'], 'actual role/helper identity')
    require(argv[argv.index('--agent') + 1] == 'coding-agent:' + role and 'KS_HOST_PROBE:T04-' + case in argv[-1], 'actual role dispatch')
    require(inv['host_sha256'] == '475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3' and inv['cks_sha256'] == pins['cks_sha256'], 'host/CKS pins')
    require(inv['env']['ANTHROPIC_BASE_URL'].startswith('http://127.0.0.1:') and inv['api_credential_kind'] == 'constant_fake_local_token', 'nonlocal model provider')
    require(argv[argv.index('--setting-sources') + 1] == '' and '--strict-mcp-config' in argv and '--dangerously-skip-permissions' not in argv and '--bare' not in argv, 'settings/permission boundary')
    require(end['host_exit'] == 0 and end['host_reaped'] and not end['forced'] and end['elapsed_seconds'] < 90 and not end['rescue'] and end['cleanup']['all_observed_identities_gone'], 'host cleanup')
    require(end['real_model_calls'] == end['formal_runs'] == 0, 'formal/model promotion')
    g = end['gateway_cleanup']
    require(g['all_handler_threads_joined'] and g['server_socket_closed'] and g['server_thread_joined'] and not g['errors'], 'gateway cleanup')
    requests, responses, frames = old.old.calls(folder)
    require(len(requests) == count, 'CKS request denominator')
    starts, exits = [r for r in frames if r['kind'] == 'start'], [r for r in frames if r['kind'] == 'exit']
    owner = next(r for r in frames if r['kind'] == 'owner')
    require(owner['owner'] == inv['owner'] and owner['driver_sha256'] == execution['host_helper_sha256'] and owner['proxy_sha256'] == digest(ROOT / 'scripts/consumer-lifecycle-proxy.py'), 'proxy pin/owner')
    require(len(starts) == len(exits) == 1 and starts[0]['child_pid'] == exits[0]['child_pid'] and starts[0]['binary_sha256'] == pins['cks_sha256'], 'CKS child identity')
    end_child = exits[0]
    require(end_child['exit_code'] == 0 and end_child['child_reaped'] and end_child['signal_worker_joined'] and end_child['output_workers_joined'] and not end_child['forced_kill'] and not end_child['terminate_sent'], 'CKS exit/reap proof')
    gateway = [r for r in read(folder / 'gateway-requests.json') if 'scripted_response' in r]
    plugin = argv[argv.index('--plugin-dir') + 1]
    require(plugin == '/private/tmp/ks-host-schema-consumer-20261017/plugin', 'plugin pin')
    body = text('plugin/agents/' + role + '.md').split('---', 2)[2].strip().replace('${CLAUDE_PLUGIN_ROOT}', plugin)
    require(any(body in '\n'.join(x.get('text', '') for x in r['body'].get('system', [])) for r in gateway), 'actual role source not loaded')
    actions = [a for r in gateway for a in r['scripted_response']['content'] if a['type'] == 'tool_use']
    results = {v['tool_use_id']: v for r in gateway for m in r['body']['messages'] if m['role'] == 'user' for v in m.get('content', []) if isinstance(v, dict) and v.get('type') == 'tool_result'}
    require(all(a['id'] in results for a in actions), 'actual tool result omitted')
    declared = read(folder / 'scripted-actions.json')
    require(len(declared) == len(actions), 'scripted/API denominator')
    resolutions = lines(folder / 'action-resolutions.jsonl') if (folder / 'action-resolutions.jsonl').exists() else []
    for i, (spec, action) in enumerate(zip(declared, actions)):
        if 'state_operation' in spec:
            resolution = next(r for r in resolutions if r['index'] == i)
            require(resolution['operation'] == spec['state_operation'] and action['name'] == 'Write' and json.loads(action['input']['content']) == resolution['proposed'], 'state action resolution')
        else:
            require(spec == {'name': action['name'], 'input': action['input']}, 'scripted/API selection')
    tool_actions = [a for a in actions if a['name'].startswith('mcp__')]
    require(len(tool_actions) == len(requests), 'API/wire tool denominator')
    for action, request in zip(tool_actions, requests):
        require(action['name'] == 'mcp__plugin_coding-agent_cks__' + request['params']['name'].replace('.', '_') and action['input'] == request['params']['arguments'] and request['params']['_meta']['claudecode/toolUseId'] == action['id'], 'API/wire tool selection')
    old.snapshot_files(folder / 'workspace-after', read(folder / 'workspace-after.json'))
    return requests, responses, actions, results, {end['host_pid'], starts[0]['wrapper_pid'], starts[0]['child_pid']}


def derive(packet):
    ns, text = old.old.consumer()
    pins = read(ROOT / old.old.PRIOR / 'binary-pins.json')
    execution = read(packet / 'execution.json')
    require(execution['driver_sha256'] == digest(ROOT / 'scripts/run-consumer-partial.py') and execution['plan_sha256'] == digest(ROOT / 'scripts/fixtures/consumer-partial-plan.json') and execution['host_helper_sha256'] == digest(ROOT / 'scripts/run-consumer-opt-in.py'), 'executed driver/plan/helper pin')
    require(execution['host_cases'] == CASES and execution['candidate_adopted'] is False and execution['real_model_calls'] == execution['formal_runs'] == 0, 'execution scope')
    require(read(packet / 'formal-state.json') == module('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
    require(read(packet / 'preservation.json') == preservation(), 'prior33 inventory')
    payload = module('prepare-consumer-stdio-readiness.py').payload
    pids, sessions, hashes = set(), {}, {}
    for case, count in zip(CASES, [2, 3, 2, 2, 0, 0]):
        result = host(packet / 'hosts' / case, case, execution, pins, text, count)
        sessions[case] = result
        pids.update(result[4])
    matrix = read(packet / 'matrix-cases.json')
    require([r['label'] for r in matrix] == ['unsupported', 'truncated-real', 'incomplete-rules'], 'matrix cases')
    requests, responses, _, results, _ = sessions['T10-matrix']
    for expected, request in zip(matrix, requests):
        require(request['params']['name'] == 'cks.context.rule_candidates' and request['params']['arguments'] == expected['arguments'], 'actual matrix selection')
    bad = [payload(responses[q['id']]) for q in requests]
    require(bad[0]['status'] == 'unsupported' and bad[0].get('analysis') is None, 'actual unsupported result')
    require(bad[1]['status'] == 'budget_exhausted' and bad[1]['analysis']['truncated'] is True and bad[1]['analysis']['rules']['status'] == 'incomplete', 'actual truncated result')
    require(bad[2]['status'] == 'partial' and not bad[2]['analysis']['truncated'] and bad[2]['analysis']['rules']['status'] == 'incomplete', 'actual incomplete rules')
    require(all(not r.get('is_error') for r in results.values()), 'bounded matrix transport error')
    legacy_requests, legacy_responses, legacy_actions, legacy_results, _ = sessions['T10-legacy']
    require([r['params']['name'] for r in legacy_requests] == ['cks.context.rule_candidates', 'cks.context.review_function'], 'legacy queries')
    legacy_value = payload(legacy_responses[legacy_requests[0]['id']])
    require(legacy_value['analysis']['rules']['status'] == 'matched_bounded', 'legacy control did not retain valid rule query')
    error = legacy_responses[legacy_requests[1]['id']]['result']
    require(error.get('isError') is True and 'requires_reindex' in str(error), 'actual requires_reindex omitted')
    require(legacy_results[legacy_actions[1]['id']].get('is_error') is True and 'requires_reindex' in str(legacy_results[legacy_actions[1]['id']]['content']), 'host did not receive legacy error')
    for build, delivery, query in [('T10-build-seed', 'T10-controls', execution['query']), ('T10-build-field', 'T10-field-delivery', execution['field_query'])]:
        folder = packet / 'hosts' / build
        requests, responses, _, results, _ = sessions[build]
        require(all(not v.get('is_error') for v in results.values()), 'positive build tool error')
        old.materialized(folder / 'workspace-after', ns)
        bundle, report = read(folder / 'workspace-after/deep-review-evidence.json'), read(folder / 'workspace-after/deep-review.json')
        for q, key in zip(requests, ['rules', 'function_review']):
            require(bundle[key] == {'tool': q['params']['name'], 'arguments': q['params']['arguments'], 'result': payload(responses[q['id']])}, 'actual positive bundle receipt')
        value = bundle['rules']['result']
        analysis = value['analysis']
        require(bundle['rules']['arguments'] == {'rule_pack': 'go-local-review@1.0.0', 'function': query} and value['status'] == analysis['status'] == 'partial' and analysis['truncated'] is False and analysis['rules']['status'] == 'matched_bounded', 'supported partial conditions')
        require(report['status'] == 'structured_partial_review' and report['independent_acceptance'] == 'not_recorded' and report['computed']['capability'] == analysis['capability'] and report['computed']['diagnostics'] == analysis['diagnostics'], 'partial promoted or diagnostics lost')
        sections = report['reasoning']['sections']
        require(set(sections) == set(ns['SECTIONS']) and sum(v['status'] == 'unknown' for v in sections.values()) == 5 and sections['data_state']['status'] == 'inferred' and all(v['unknowns'] for v in sections.values()), 'inferred/unknown split lost')
        require(report['computed']['module']['interprocedural_edges'] == 'unknown' and report['structural_review']['snapshot_association'] == 'identity_only_unverified_snapshot' and any('not whole-program safety' in s for s in report['limitations']), 'partial limitations lost')
        require(analysis['rules']['candidates'] if build == 'T10-build-seed' else not analysis['rules']['candidates'], 'empty/nonempty positive contrast')
        if build == 'T10-build-field':
            require(any(d['code'] == 'memory_write_untracked' for d in analysis['diagnostics']) and report['computed']['capability']['memory_alias'] == 'unsupported', 'Field unknown memory capability lost')
        hashes[build] = {name: read(folder / 'workspace-after.json')[name]['sha256'] for name in old.old.ARTIFACTS}
        delivered = packet / 'hosts' / delivery
        old.materialized(delivered / 'workspace-after', ns)
        require(all(read(delivered / 'workspace-after.json')[name]['sha256'] == digest(folder / 'workspace-after' / (name + '.txt' if name.endswith('.md') else name)) for name in old.old.ARTIFACTS), 'delivery replaced partial artifact')
        state = read(delivered / 'workspace-after/state.json')
        require(state['current_state'] == 'COMPLETED' and state['states']['PLANNING']['status'] == 'completed' and state['states']['PLANNING']['review_report_sha256'] == hashes[build]['review-report.md'], 'positive delivery binding')
    _, _, actions, results, _ = sessions['T10-controls']
    writes = [a for a in actions if a['name'] == 'Write']
    require(len(writes) == 11, 'Write attempt denominator')
    for action, reason in zip(writes[:-1], REASONS):
        result = results[action['id']]
        require(result.get('is_error') is True and 'Structured review delivery blocked: ' + reason in str(result['content']), 'actual hook refusal differs')
    require(not results[writes[-1]['id']].get('is_error') and all(not results[a['id']].get('is_error') for a in actions if a['name'] == 'Bash'), 'normal/control Bash failure')
    for case, reason in zip(CONTROLS, REASONS):
        folder = packet / 'controls' / case
        before, mutated, after, restored = read(folder / 'before.json'), read(folder / 'mutated.json'), read(folder / 'after-attempt.json'), read(folder / 'restore.json')
        require(mutated['verification']['exit_code'] == 2 and json.loads(mutated['verification']['stdout']) == {'status': 'invalid', 'reason': reason}, 'CLI refusal differs')
        require(mutated['workspace'] == after and before == restored['workspace'] and restored['verification']['exit_code'] == 0, 'refusal modified delivery or restore failed')
        require(all(before[n]['sha256'] == hashes['T10-build-seed'][n] for n in old.old.ARTIFACTS), 'control did not start from supported partial')
        old.snapshot_files(folder / 'original', {name: before[name] for name in driver.FILES})
        actual = case in ['unsupported', 'truncated-real', 'incomplete-rules', 'requires-reindex']
        require(mutated['origin'] == ('actual_MCP_receipt' if actual else 'local_verifier_mutation'), 'synthetic mutation mislabelled as actual response')
        # Reconstruct the exact mutated bytes from the actual wire receipts and
        # pinned original. A declared origin and changed hash are insufficient.
        bundle = read(folder / 'original/deep-review-evidence.json')
        report = read(folder / 'original/deep-review.json')
        if case in ['unsupported', 'truncated-real', 'incomplete-rules']:
            request = sessions['T10-matrix'][0][['unsupported', 'truncated-real', 'incomplete-rules'].index(case)]
            response = sessions['T10-matrix'][1][request['id']]
            bundle['rules'] = {'tool': request['params']['name'], 'arguments': request['params']['arguments'], 'result': payload(response)}
        elif case == 'requires-reindex':
            request = legacy_requests[1]
            bundle['function_review'] = {'tool': request['params']['name'], 'arguments': request['params']['arguments'], 'result': error}
        elif case == 'all-unknown':
            for section in report['reasoning']['sections'].values():
                section.update(status='unknown', evidence_nodes=[], assumptions=[], unknowns=['Not independently reviewed; no completion claim.'])
        elif case == 'missing-section':
            report['reasoning']['sections'].pop('io')
        elif case == 'truncated-flag':
            bundle['rules']['result']['analysis']['truncated'] = True
        elif case == 'unknown-observed':
            report['reasoning']['sections']['resources']['status'] = 'observed'
        elif case == 'unknown-empty':
            report['reasoning']['sections']['resources']['unknowns'] = []
        elif case == 'safe-status':
            report['status'] = 'complete_safe'
        changed = 'deep-review-evidence.json' if actual or case == 'truncated-flag' else 'deep-review.json'
        value = bundle if changed == 'deep-review-evidence.json' else report
        require(sha((json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode()) == mutated['workspace'][changed]['sha256'], 'mutated bytes not derived from actual receipt/control')
        try:
            ns['validate'](report, bundle)
        except ValueError as rejected:
            require(str(rejected) == reason, 'reconstructed mutant refusal differs')
        else:
            raise ValueError('reconstructed invalid review was accepted')
    field_actions, field_results = sessions['T10-field-delivery'][2:4]
    require(all(not v.get('is_error') for v in field_results.values()) and len([a for a in field_actions if a['name'] == 'Write']) == 1, 'Field positive delivery failed')
    schema_fault = read(packet / 'legacy-index-control.json')
    require(schema_fault['owned_schema_fault_not_operational_outage'] is True and schema_fault['operation'] == 'DROP TABLE code_edges; checkpoint WAL' and schema_fault['source_db_sha256'] == digest(packet / 'indexes/normal/graph.db') and schema_fault['legacy_db_sha256'] == digest(packet / 'indexes/legacy/graph.db') and schema_fault['checkpoint'] == [[0, 0, 0]], 'owned schema fault binding')
    with sqlite3.connect('file:' + str(packet / 'indexes/legacy/graph.db') + '?mode=ro&immutable=1', uri=True) as db:
        require(db.execute("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='code_edges'").fetchone()[0] == 0, 'legacy table still exists')
    require(digest(packet / 'indexes/normal/graph.db') == digest(ROOT / BASE / 'consumer-stdio-readiness-v1/index/graph.db'), 'normal fixture index changed')
    integrity = read(packet / 'integrity.json')
    expected = read(ROOT / old.old.PRIOR / 'execution.json')
    require(len(integrity['checkouts']) == 4 and all(value == expected[key] for key, value in integrity['checkouts'].items()), 'source checkout pin/cleanliness')
    require(integrity['binary_sha256'] == {name: pins[name + '_sha256'] for name in ['cks', 'ckg']}, 'binary pins')
    audit = read(packet / 'postrun-process-audit.json')
    require({r['pid'] for r in audit['probes']} == pids and len(audit['probes']) == len(pids), 'owned PID audit denominator')
    require(all(r['exit_code'] == 1 and not r['stdout'].strip() for r in audit['probes']) and audit['self']['exit_code'] == 0 and str(audit['pid']) in audit['self']['stdout'], 'owned process cleanup audit')
    return {'test_id': 'T10', 'status': 'isolated_supported_partial_and_blocked_incomplete_delivery_pass', 'host_sessions': 6, 'CKS_tool_requests': 9, 'CKS_exit0_reaped': 6, 'actual_backend_requires_reindex_errors': 1, 'supported_partial_reviews_delivered': 2, 'empty_candidates_review_remains_partial': True, 'unknown_sections_per_review': 5, 'real_response_controls': 4, 'synthetic_verifier_controls': 6, 'CLI_refusals': 10, 'actual_Write_hook_refusals': 10, 'restores_byte_identical': 10, 'positive_deliveries': 2, 'owned_PIDs_absent': len(pids), 'partial_is_blanket_failure': False, 'empty_or_unknown_is_safety_or_missing0': False, 'semantic_safety_or_completeness_evaluated': False, 'ontology_effect_acceptance': 'not_evaluated', 'candidate_adopted': False, 'real_model_calls': 0, 'formal_runs': 0, 'formal_completed': 0, 'formal_total': 9, 'prior_frozen_packets_unchanged': 33, 'full_plan_passed': 20, 'full_plan_pending': ['T16', 'T22'], 'next': 'T16 error handling/denominators, then P08/T22'}


def prepare(seed, legacy, field, output):
    require(not output.exists(), 'fresh output required')
    output.mkdir(parents=True)
    for case in CASES:
        source = (legacy if case == 'T10-legacy' else field if case in ['T10-build-field', 'T10-field-delivery'] else seed) / case
        dest = output / 'hosts' / case
        dest.mkdir(parents=True)
        for name in old.old.TOP:
            shutil.copyfile(source / name, dest / name)
        if (source / 'action-resolutions.jsonl').exists():
            shutil.copyfile(source / 'action-resolutions.jsonl', dest / 'action-resolutions.jsonl')
        old.copy_workspace(source / 'workspace-after', dest / 'workspace-after')
    old.copy_workspace(seed / 'controls', output / 'controls')
    for name in ['execution.json', 'matrix-cases.json', 'legacy-index-control.json']:
        shutil.copyfile(seed / name, output / name)
    for name, source in [('normal', seed / 'graph'), ('legacy', legacy / 'legacy-graph')]:
        dest = output / 'indexes' / name
        dest.mkdir(parents=True)
        for f in ['graph.db', 'manifest.json']:
            shutil.copyfile(source / f, dest / f)
    dump(output / 'formal-state.json', module('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output / 'preservation.json', preservation())
    repos = {'cks_postrun': '/private/tmp/ks-host-schema-cks-20261017', 'consumer_postrun': '/private/tmp/ks-host-schema-consumer-20261017', 'original_postrun': '/private/tmp/ks-isolated-host-20261015/coding-agent', 'prior_candidate_postrun': '/private/tmp/ks-consumer-contract-candidate-20261017'}
    dump(output / 'integrity.json', {'checkouts': {key: {'commit': subprocess.check_output(['git', '-C', path, 'rev-parse', 'HEAD'], text=True).strip(), 'porcelain': subprocess.check_output(['git', '-C', path, 'status', '--porcelain'], text=True)} for key, path in repos.items()}, 'binary_sha256': {name: digest(Path('/private/tmp/ks-host-schema-runtime-20261017/bin') / name) for name in ['cks', 'ckg']}})
    pids = set()
    for case in CASES:
        pids.add(read(output / 'hosts' / case / 'exit.json')['host_pid'])
        for row in lines(output / 'hosts' / case / 'mcp-frames.jsonl'):
            if row['kind'] == 'start':
                pids.update([row['wrapper_pid'], row['child_pid']])
    def ps(pid):
        q = subprocess.run(['ps', '-p', str(pid), '-o', 'pid=,ppid=,stat=,lstart=,command='], capture_output=True, text=True)
        return {'pid': pid, 'exit_code': q.returncode, 'stdout': q.stdout, 'stderr': q.stderr}
    dump(output / 'postrun-process-audit.json', {'pid': os.getpid(), 'self': ps(os.getpid()), 'probes': [ps(pid) for pid in sorted(pids)]})
    dump(output / 'partial.json', derive(output))
    (output / 'REPORT.txt').write_text('준비34/T10 — 지원 partial·unknown 전달과 불완전/placeholder 거부\n실제6host/CKS9요청·6exit0/reap. Seed와 빈 후보 Field의 partial2건은 truncated=false/rules matched 조건과5unknown절·capability/diagnostics/한계를 유지해 전달. 빈 후보는 안전·누락0 증명이 아님.\n실제 unsupported/truncated/incomplete-rule 및 owned requires_reindex 오류4개와 synthetic verifier 변이6개를 구분. CLI10/실제Write hook10거부·원본 복구10회, 정상 전달2회 확인.\n구형 인덱스는 소유 schema fault이며 운영 장애가 아님. 자연어 의미 품질·안전·완전성/온톨로지 효과는 미평가. 이전33패킷/runtime/schema 유지, 계획20/22·남은 T16/T22, 정식0/9·담당자미정/freeze null/실제모델0/후보미채택.\n')
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    (output / '.gitattributes').write_text('**/*.db binary\n')
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'T10_partial_unknown_not_safety_or_completeness', 'input_bindings': {str(p): digest(ROOT / p) for p in INPUTS}, 'generator_sha256': digest(Path(__file__)), 'artifacts': {p.relative_to(output).as_posix(): digest(p) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('T10 frozen:2 supported partials/10 refusals;6 hosts/9 CKS; plan20/22 formal0/9')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['seed', 'legacy', 'field', 'output']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    prepare(args.seed.resolve(), args.legacy.resolve(), args.field.resolve(), args.output.resolve())
