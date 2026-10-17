#!/usr/bin/env python3
"""Freeze T16 actual errors without dropping attempts or promoting late replies."""
import argparse
from functools import lru_cache
import io
import json
import os
from pathlib import Path
import shutil
import signal
import sqlite3
import subprocess
import tarfile
import importlib.util
from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PREVIOUS = BASE / 'followups/consumer-partial-v1'
PACKET = BASE / 'followups/consumer-errors-v1'
BASE_COMMIT = 'a7c9d929965e6d649b18446d7902d0c4b4475a25'
SESSIONS = ['main', 'legacy', 'provider', 'timeout', 'startup']
FILES = ['stdin.jsonl', 'stdout.jsonl', 'stderr.txt', 'attempts.json', 'messages.json', 'lifecycle.json', 'faults.json']


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


old = module('prepare-consumer-partial.py')
read, dump, lines, require, digest = old.read, old.dump, old.lines, old.require, old.digest
INPUTS = [PREVIOUS / 'manifest.json', Path('scripts/fixtures/consumer-errors-plan.json'), Path('scripts/run-consumer-errors.py'), *old.INPUTS, Path('scripts/prepare-consumer-partial.py'), Path('scripts/check-consumer-partial.py'), Path('scripts/test_consumer_partial.py')]


@lru_cache(maxsize=1)
def expected_preservation():
    keep = dict(old.preservation()['files'])
    manifest = read(ROOT / PREVIOUS / 'manifest.json')
    names = [str(PREVIOUS / 'manifest.json'), *[str(PREVIOUS / f) for f in manifest['artifacts']], 'scripts/prepare-consumer-partial.py', 'scripts/check-consumer-partial.py', 'scripts/test_consumer_partial.py']
    archive = subprocess.check_output(['git', 'archive', BASE_COMMIT, *sorted(set(names))], cwd=ROOT)
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        for entry in tar:
            if entry.isfile():
                keep[entry.name] = sha(tar.extractfile(entry).read())
    return {'base_commit': BASE_COMMIT, 'prior_packets': 34, 'files': keep}


def preservation():
    expected = expected_preservation()
    for name, value in expected['files'].items():
        require(digest(safe(ROOT, name)) == value, 'prior packet/source changed: ' + name)
    return expected


def derive(packet):
    plan = read(ROOT / 'scripts/fixtures/consumer-errors-plan.json')
    ex = read(packet / 'execution.json')
    pins = read(ROOT / BASE / 'host-schema-compatibility-v1/binary-pins.json')
    require(ex['driver_sha256'] == digest(ROOT / 'scripts/run-consumer-errors.py') and ex['plan_sha256'] == digest(ROOT / 'scripts/fixtures/consumer-errors-plan.json') and ex['binary_sha256'] == pins['cks_sha256'], 'executed driver/plan/binary pin')
    require(ex['sessions'] == SESSIONS and not ex['candidate_adopted'] and ex['real_model_calls'] == ex['formal_runs'] == ex['actual_role_sessions'] == 0, 'execution scope')
    require(read(packet / 'formal-state.json') == module('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
    require(read(packet / 'preservation.json') == preservation(), 'prior34 preservation')
    expected_labels = [['initialize', 'positive', 'missing', 'ambiguous'], ['initialize', 'legacy'], ['initialize', 'provider-health', 'provider'], ['initialize', 'timeout', 'recovery'], ['startup']]
    pids, rows, tool_count, rpc_count = set(), {}, 0, 0
    for name, labels in zip(SESSIONS, expected_labels):
        folder = packet / 'sessions' / name
        attempts, lifecycle, faults = read(folder / 'attempts.json'), read(folder / 'lifecycle.json'), read(folder / 'faults.json')
        require([r['label'] for r in attempts] == labels and [r['request']['id'] for r in attempts] == list(range(1, len(attempts) + 1)), 'attempt denominator/order')
        require(lifecycle['command'] == [str(Path('/private/tmp/ks-host-schema-runtime-20261017/bin/cks')), 'mcp', '--config', ex['root'] + '/' + name + '.yaml'], 'actual child command')
        require(faults['identity']['pid'] == lifecycle['pid'] and faults['identity']['command'] == ' '.join(lifecycle['command']), 'owned child identity')
        require(lifecycle['exit_code'] == (1 if name == 'startup' else 0) and lifecycle['child_reaped'] and lifecycle['reader_joined'] and lifecycle['eof_sent'] and not lifecycle['terminate_sent'] and not lifecycle['kill_sent'], 'raw exit/reap proof')
        require(lifecycle['actual_role_identity'] is None and lifecycle['listeners_created'] == 0 and lifecycle['elapsed_seconds'] < 180, 'stdio scope/budget')
        pids.add(lifecycle['pid'])
        reqs = [r['request'] for r in attempts]
        sent = lines(folder / 'stdin.jsonl')
        notification = {'jsonrpc': '2.0', 'method': 'notifications/initialized'}
        require(sent == (reqs if name == 'startup' else reqs[:1] + [notification] + reqs[1:]), 'raw request association')
        received = lines(folder / 'stdout.jsonl')
        expected_out = []
        for row in attempts:
            if row['label'] == 'recovery':
                expected_out.extend(faults['late_responses'])
            if row['outcome'] == 'response':
                require(row['response']['id'] == row['request']['id'] and 0 <= row['elapsed_seconds'] < row['deadline_seconds'], 'response id/deadline')
                expected_out.append(row['response'])
            else:
                require(row['response'] is None, 'unanswered attempt fabricated response')
            if row['label'] != 'initialize':
                rows[row['label']] = row
        require(received == expected_out, 'raw response/late association')
        require(read(folder / 'messages.json') == [{'label': r['label'], 'request': r['request'], 'response': r['response']} for r in attempts if r['outcome'] == 'response'], 'completed messages cannot replace attempts')
        tools = sum(r['request']['method'] == 'tools/call' for r in attempts)
        require(lifecycle['rpc_requests'] == len(attempts) and lifecycle['tool_requests'] == tools, 'lifecycle denominator')
        tool_count += tools
        rpc_count += len(attempts)
        require(digest(packet / 'configs' / (name + '.yaml')) == ex['config_hashes'][name], 'actual config pin')
        if name != 'timeout':
            require(not faults['signals'] and not faults['late_responses'], 'unexpected signal/late response')
    require(len(pids) == 5 and tool_count == 8 and rpc_count == 13, 'global child/call denominator')
    payload = module('prepare-consumer-stdio-readiness.py').payload
    def value(label):
        return payload(rows[label]['response'])
    for label in ['positive', 'recovery']:
        row = rows[label]
        require(row['outcome'] == 'response' and row['request']['params'] == {'name': 'cks.context.value_dependencies', 'arguments': ex['query']}, 'positive/recovery request')
        v = value(label)
        require(v['status'] == 'partial' and v['analysis'] is not None and not v['analysis']['truncated'], 'supported partial response')
    require(value('missing')['status'] == 'not_found' and value('missing')['analysis'] is None and rows['missing']['request']['params'] == {'name': 'cks.context.value_dependencies', 'arguments': {'node_id': 'missing-owned-symbol'}}, 'missing cannot become result')
    for label, text, symbol in [('ambiguous', 'ambiguous across 2 definitions', 'Same'), ('legacy', 'requires_reindex', 'example.invalid/consumer-stdio-fixture.Seed')]:
        row = rows[label]
        require(row['request']['params'] == {'name': 'cks.context.review_function', 'arguments': {'symbol': symbol}} and row['response']['result'].get('isError') is True and text in str(row['response']), 'actual missing/ambiguity/reindex error')
    health = value('provider-health')
    setup = read(packet / 'fault-setup.json')
    vector = health['backends']['vector']
    reason = vector['reason']
    require(setup['bound_not_listening'] and setup['probe_connect_errno'] != 0 and setup['endpoint'].startswith('http://127.0.0.1:'), 'owned endpoint failure')
    config = (packet / 'configs/provider.yaml').read_text()
    require('provider: ollama' in config and 'path: ' + ex['root'] + '/owned-vector' in config and 'ollama_url: ' + setup['endpoint'] in config, 'configured provider not unset dummy')
    require(health['serviceable'] is False and health['status'] == 'degraded' and vector['reachable'] is False and vector['endpoint'] == setup['endpoint'] and 'model list unavailable' in reason and 'operation timed out' in reason and reason in (packet / 'sessions/provider/stderr.txt').read_text(), 'actual provider failure diagnostic')
    require(rows['provider']['request']['params'] == {'name': 'cks.context.get_for_task', 'arguments': {'prompt': 'owned T16 provider fault'}} and rows['provider']['response']['result'].get('isError') is True and 'service unavailable' in str(rows['provider']['response']), 'service unavailable response')
    require(health['alignment']['ok'] is False and 'manifest missing' in str(health['alignment']['warnings']), 'provider alignment confound omitted')
    t = rows['timeout']
    require(t['outcome'] == 'transport_timeout' and t['response'] is None and t['deadline_seconds'] == plan['transport_deadline_seconds'] and 1 <= t['elapsed_seconds'] < 3, 'hard timeout not retained')
    require(t['request']['params'] == rows['recovery']['request']['params'] and t['request']['id'] != rows['recovery']['request']['id'], 'recovery must be separate call')
    faults = read(packet / 'sessions/timeout/faults.json')
    signals = faults['signals']
    require([r['signal'] for r in signals] == [int(signal.SIGSTOP), int(signal.SIGCONT)] and all(r['owned_Popen_child'] and r['target_pid'] == faults['identity']['pid'] for r in signals) and 'T' in signals[0]['observed_state'] and 'T' in signals[1]['identity']['stat'], 'owned stop/resume proof')
    require(all(all(r['identity'][k] == faults['identity'][k] for k in ['pid', 'ppid', 'birth', 'command']) for r in signals), 'signal identity mismatch')
    require(len(faults['late_responses']) == 1 and faults['late_responses'][0]['id'] == t['request']['id'] and payload(faults['late_responses'][0])['status'] == 'partial', 'late response must remain outside success')
    require(rows['startup']['outcome'] == 'eof_without_response' and rows['startup']['request']['method'] == 'initialize' and json.loads((packet / 'sessions/startup/stderr.txt').read_text())['code'] == 'operation_failed', 'actual startup failure')
    require(digest(packet / 'indexes/normal.db') == setup['normal_db_sha256'] == digest(ROOT / BASE / 'consumer-stdio-readiness-v1/index/graph.db') and digest(packet / 'indexes/legacy.db') == setup['legacy_db_sha256'] and digest(packet / 'indexes/broken.db') == setup['broken_db_sha256'], 'index fault hashes')
    require((packet / 'indexes/broken.db').read_bytes() == b'owned T16 invalid SQLite database\n' and setup['owned_fault_not_operational_outage'] and setup['operation'] == 'DROP TABLE code_edges; checkpoint WAL' and setup['checkpoint'] == [[0, 0, 0]], 'owned schema fault')
    with sqlite3.connect('file:' + str(packet / 'indexes/legacy.db') + '?mode=ro&immutable=1', uri=True) as db:
        require(db.execute("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='code_edges'").fetchone()[0] == 0, 'legacy table not dropped')
    audit = read(packet / 'postrun-process-audit.json')
    require({r['pid'] for r in audit['probes']} == pids and len(audit['probes']) == 5 and all(r['exit_code'] == 1 and not r['stdout'].strip() for r in audit['probes']) and audit['self']['exit_code'] == 0 and str(audit['pid']) in audit['self']['stdout'], 'owned PID cleanup audit')
    require(read(packet / 'integrity.json') == read(ROOT / PREVIOUS / 'integrity.json'), 'source/binary integrity')
    cases = [{'id': key, 'outcome': status} for key, status in zip(plan['evaluation_cases'], ['supported_partial', 'not_found', 'ambiguous_error', 'requires_reindex_error', 'provider_unavailable_error', 'transport_timeout', 'supported_partial', 'startup_failure'])]
    return {'test_id': 'T16', 'status': 'owned_actual_errors_denominators_pass', 'cases': cases, 'scenario_denominator': 8, 'tool_evaluation_denominator': 7, 'diagnostic_tool_calls': 1, 'total_tool_attempts': 8, 'total_rpc_attempts': 13, 'supported_partial_results': 2, 'nonusable_scenarios': 6, 'scenario_supported_partial_fraction': {'numerator': 2, 'denominator': 8}, 'evaluation_tool_supported_partial_fraction': {'numerator': 2, 'denominator': 7}, 'late_responses_outside_success': 1, 'complete_analysis_results': 0, 'startup_failures': 1, 'raw_CKS_exit0_reaped': 4, 'raw_CKS_exit1_reaped': 1, 'owned_PIDs_absent': 5, 'provider_connection_observed': 'operation_timed_out_not_immediate_refusal', 'provider_alignment_confounded': True, 'quality_precision_recall_safety': None, 'ontology_effect_acceptance': 'not_evaluated', 'actual_role_sessions': 0, 'candidate_adopted': False, 'real_model_calls': 0, 'formal_runs': 0, 'formal_completed': 0, 'formal_total': 9, 'prior_frozen_packets_unchanged': 34, 'full_plan_passed': 21, 'full_plan_pending': ['T22'], 'next': 'P08/T22 final readiness/recovery/publication boundary'}


def prepare(source, output):
    require(not output.exists(), 'fresh output required')
    output.mkdir(parents=True)
    for name in SESSIONS:
        dest = output / 'sessions' / name
        dest.mkdir(parents=True)
        for file in FILES:
            shutil.copyfile(source / name / file, dest / file)
    (output / 'configs').mkdir()
    for name in SESSIONS:
        shutil.copyfile(source / (name + '.yaml'), output / 'configs' / (name + '.yaml'))
    for name in ['execution.json', 'fault-setup.json']:
        shutil.copyfile(source / name, output / name)
    (output / 'indexes').mkdir()
    for name, src in [('normal', 'graph/graph.db'), ('legacy', 'legacy.db'), ('broken', 'broken.db')]:
        shutil.copyfile(source / src, output / 'indexes' / (name + '.db'))
    dump(output / 'formal-state.json', module('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output / 'preservation.json', preservation())
    previous = read(ROOT / PREVIOUS / 'integrity.json')
    paths = {'cks_postrun': '/private/tmp/ks-host-schema-cks-20261017', 'consumer_postrun': '/private/tmp/ks-host-schema-consumer-20261017', 'original_postrun': '/private/tmp/ks-isolated-host-20261015/coding-agent', 'prior_candidate_postrun': '/private/tmp/ks-consumer-contract-candidate-20261017'}
    integrity = {'checkouts': {key: {'commit': subprocess.check_output(['git', '-C', path, 'rev-parse', 'HEAD'], text=True).strip(), 'porcelain': subprocess.check_output(['git', '-C', path, 'status', '--porcelain'], text=True)} for key, path in paths.items()}, 'binary_sha256': {name: digest(Path('/private/tmp/ks-host-schema-runtime-20261017/bin') / name) for name in ['cks', 'ckg']}}
    require(integrity == previous, 'source/binary changed')
    dump(output / 'integrity.json', integrity)
    def ps(pid):
        p = subprocess.run(['ps', '-p', str(pid), '-o', 'pid=,ppid=,stat=,lstart=,command='], capture_output=True, text=True)
        return {'pid': pid, 'exit_code': p.returncode, 'stdout': p.stdout, 'stderr': p.stderr}
    pids = [read(output / 'sessions' / name / 'lifecycle.json')['pid'] for name in SESSIONS]
    dump(output / 'postrun-process-audit.json', {'pid': os.getpid(), 'self': ps(os.getpid()), 'probes': [ps(pid) for pid in sorted(pids)]})
    dump(output / 'errors.json', derive(output))
    (output / 'REPORT.txt').write_text('준비35/T16 — 실제 오류와 원래 분모 보존\n소유 stdio5세션: 도구8시도=평가7+사전 지정 health진단1; 시작 initialize 실패1은 별도 시나리오에 포함. 전체8시나리오=지원partial2+비사용6. 완전 분석0; 품질 성공률이 아님.\nmissing/ambiguous/requires_reindex/provider unavailable/전송timeout/startup 실패를 제외하지 않음. SIGSTOP 소유 CKS의1초 전송기한 초과는 실제 계산 예산/Claude host timeout과 다름. SIGCONT 뒤 late response1은 최초 timeout을 성공으로 바꾸지 않으며 별도 recovery만 partial.\nprovider는 연결 즉시 거부를 예상했으나 실제 operation timed out. 원본 진단을 유지하며 configured CKV의 manifest 누락과 alignment 실패가 함께 존재하므로 단일 장애 원인 격리/실제 embedding 품질 증명이 아님.\nCKS 원래 exit0/reap4·의도한 startup exit1/reap1을 구분. 소유PID5부재·이전34패킷/runtime/schema 유지. 후보미채택/실제모델0/정식0/9. 온톨로지 효과 미평가. 계획21/22, 다음 P08/T22.\n')
    (output / '.gitattributes').write_text('**/*.db binary\n')
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'T16_owned_errors_not_quality_acceptance', 'input_bindings': {str(p): digest(ROOT / p) for p in INPUTS}, 'generator_sha256': digest(Path(__file__)), 'artifacts': {p.relative_to(output).as_posix(): digest(p) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('T16 frozen:8 scenarios/7 evaluation tools;2 partial+6 unusable; plan21/22 formal0/9')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.source.resolve(), args.output.resolve())
