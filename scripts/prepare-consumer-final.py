#!/usr/bin/env python3
"""P08/T22: final owned-readiness boundary, preserving unproven historical scope."""
import argparse
from functools import lru_cache
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import importlib.util
from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PREVIOUS = BASE / 'followups/consumer-errors-v1'
PACKET = BASE / 'followups/consumer-final-v1'
BASE_COMMIT = '39f4554034ae8fb9ad7a87b8b22b9a21cbebf368'
TOP = ['before.json', 'after.json', 'restore-control.json', 'restored.json', 'listener.json', 'cleanup.json', 'execution.json']
QUERY = ['stdin.jsonl', 'stdout.jsonl', 'stderr.txt', 'messages.json', 'lifecycle.json']


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


old = module('prepare-consumer-errors.py')
read, dump, lines, require, digest = old.read, old.dump, old.lines, old.require, old.digest
INPUTS = [PREVIOUS / 'manifest.json', Path('scripts/fixtures/consumer-final-plan.json'), Path('scripts/run-consumer-final.py'), *old.INPUTS, Path('scripts/prepare-consumer-errors.py'), Path('scripts/check-consumer-errors.py'), Path('scripts/test_consumer_errors.py')]


@lru_cache(maxsize=1)
def expected_preservation():
    keep = dict(old.preservation()['files'])
    manifest = read(ROOT / PREVIOUS / 'manifest.json')
    names = [str(PREVIOUS / 'manifest.json'), *[str(PREVIOUS / f) for f in manifest['artifacts']], 'scripts/prepare-consumer-errors.py', 'scripts/check-consumer-errors.py', 'scripts/test_consumer_errors.py']
    archive = subprocess.check_output(['git', 'archive', BASE_COMMIT, *sorted(set(names))], cwd=ROOT)
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        for entry in tar:
            if entry.isfile():
                keep[entry.name] = sha(tar.extractfile(entry).read())
    return {'base_commit': BASE_COMMIT, 'prior_packets': 35, 'files': keep}


def preservation():
    expected = expected_preservation()
    for name, value in expected['files'].items():
        require(digest(safe(ROOT, name)) == value, 'prior packet/source changed: ' + name)
    return expected


def coverage():
    host = BASE / 'host-schema-compatibility-v1'
    rows = {key: {'id': key, 'status': 'pass', 'scope': 'isolated_candidate_scope_not_independent_acceptance', 'manifest': str(host / 'manifest.json')} for key in read(ROOT / host / 'role-readiness.json')['full_plan_cases_passed']}
    for key, folder in [('T04', 'consumer-opt-in-v1'), ('T09', 'consumer-foreign-pin-v1'), ('T10', 'consumer-partial-v1'), ('T15', 'consumer-render-lifecycle-v1'), ('T18', 'consumer-render-lifecycle-v1'), ('T16', 'consumer-errors-v1')]:
        rows[key] = {'id': key, 'status': 'pass', 'scope': 'isolated_candidate_scope_not_independent_acceptance', 'manifest': str(BASE / 'followups' / folder / 'manifest.json')}
    rows['T22'] = {'id': 'T22', 'status': 'pass', 'scope': 'current_T22_window_and_historical_receipts_with_explicit_limits', 'evidence': 'this_packet:boundary.json'}
    return [rows[k] for k in sorted(rows)]


def handoff():
    return {'candidate': 'isolated_P01_P08_ready_not_adopted', 'formal_completed': 0, 'formal_total': 9, 'reviewer_assignment': '미정', 'freeze': None, 'execution_allowed': False, 'ontology_effect': 'not_evaluated_required_in_independent_acceptance', 'next': 'assign_independent_gold_reviewer_and_final_owner_then_freeze_DEV_FINAL_gold_matching_budget_and_run_scope', 'remaining_formal_ids': ['WI-1', 'WI-2', 'WI-3', 'WI-4', 'WI-V1', 'WI-V2', 'WI-6', 'WI-SP', 'WI-C'], 'optional_backlog_count': 12, 'optional_backlog': ['조직 인증/권한', '공개 CLI/HTTP·graph export', 'graph 점수/확장 조정', 'domain flow gold 평가', '대규모 corpus/긴 함수', 'PR/as-of 지식', 'legacy 출처', 'CoreML', 'embedding cache 연결', '운영 감사 연결', 'viewer 상호작용', '배포/downstream 동기화'], 'final_self_containing_commit': 'verify_actual_signature_and_clean_tree_after_packet_creation_not_embedded_in_self_hash', 'production_deployment_or_global_historical_preservation_proven': False}


def derive(packet):
    plan = read(ROOT / 'scripts/fixtures/consumer-final-plan.json')
    ex = read(packet / 'execution.json')
    require(ex['driver_sha256'] == digest(ROOT / 'scripts/run-consumer-final.py') and ex['plan_sha256'] == digest(ROOT / 'scripts/fixtures/consumer-final-plan.json') and ex['base_commit'] == BASE_COMMIT, 'driver/plan/base pin')
    require(ex['actual_CKS_tools'] == 1 and ex['actual_host_sessions'] == ex['real_model_calls'] == ex['formal_runs'] == ex['push_PR_tracker_operations'] == ex['production_mutations'] == 0 and not ex['candidate_adopted'], 'execution/publication/adoption scope')
    require(ex['snapshot_scope'] == 'current_T22_window_not_historical_global_baseline', 'historical baseline cannot be invented')
    require(read(packet / 'formal-state.json') == module('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
    require(read(packet / 'preservation.json') == preservation(), 'prior35 preservation')
    before, after = read(packet / 'before.json'), read(packet / 'after.json')
    require(before == after, 'protected before/after mismatch')
    require(set(before) == {'global', 'operations', 'sources'}, 'snapshot inventory')
    global_files = before['global']
    require(set(global_files) >= set(plan['protected_global_files']) | {'.claude.json:selected_mcp_configuration'}, 'global protected inventory')
    require(all(set(v) <= {'kind', 'sha256', 'mode', 'bytes', 'target'} and v['kind'] in ['absent', 'file', 'symlink', 'json_projection'] for v in global_files.values()), 'configuration contents/credentials cannot be published')
    ops, src = before['operations'], before['sources']
    require(ops['root_exists'] and len(ops['metadata']) > 0 and any(Path(k).name == 'current' and v['kind'] == 'symlink' for k, v in ops['metadata'].items()), 'operational metadata/pointer census')
    require(not ops['full_database_bytes_hashed'] and not ops['availability_or_auth_probed'], 'operational scope overclaim')
    require(all(r['owned_by_T22'] is False and set(r) == {'pid', 'ppid', 'birth', 'binary', 'command_sha256', 'parent_binary', 'owned_by_T22'} for r in ops['engines']), 'existing engines ownership/privacy')
    expected = read(ROOT / PREVIOUS / 'integrity.json')
    require(set(src['repos']) == set(expected['checkouts']) | {'chainbench'}, 'source checkout inventory')
    for label, state in src['repos'].items():
        if label in expected['checkouts']:
            require(state['commit'] == expected['checkouts'][label]['commit'], 'previous checkout pin')
        else:
            require(state['commit'] == 'bdf5ad2758662310b23c508e61b13ee318e92b40', 'adopted ChainBench pin')
        require(state['porcelain'] == '' and state['signature_exit'] == 0, 'clean signed candidate/original commit')
    require(src['binaries'] == expected['binary_sha256'] and src['host_sha256'] == '475517caa611e3318a2f298fa071287ec98955538df78c58f709ecc01e5ae2d3' and src['chainbench_sha256'] == 'b7871b46a474214e69cb71e1e62f5c8c35358e08b50e9988c73a0c8b0bf3da2c' and src['root_head'] == BASE_COMMIT, 'current binary/root pins')
    tracked = subprocess.check_output(['git', 'ls-tree', '-r', '--name-only', BASE_COMMIT, '--', 'cmd', 'internal', 'pkg', 'go.mod', 'go.sum'], cwd=ROOT, text=True).splitlines()
    names = {*tracked, 'docs/REMAINING-WORK.md', 'docs/spec-driven/CURRENT-HANDOFF.md'}
    require(set(src['maintained_files']) == names, 'maintained source inventory')
    fingerprint = module('run-consumer-final.py').fingerprint
    require(all(fingerprint(ROOT / name) == value for name, value in src['maintained_files'].items()), 'maintained source/bound document changed')
    control, restored = read(packet / 'restore-control.json'), read(packet / 'restored.json')
    original, mutated = control['original'], control['mutated']
    require(set(original) == set(mutated) == set(plan['owned_restore_files']) | {'active'}, 'owned restore inventory')
    require({name for name in original if original[name] != mutated[name]} == {'source/logic.go', 'graph/graph.db', 'cks.yaml', 'active'} and control['comparison_before_restore'] == 'rejected_not_byte_identical', 'restore negative control')
    require(mutated['active'] == {'kind': 'symlink', 'target': 'fault-version'} and original['active'] == {'kind': 'symlink', 'target': 'graph'}, 'owned pointer mutation/restore')
    require([r['case'] for r in control['boundary_controls']] == ['foreign', 'symlink-escape'] and all(r['refused_before_mutation'] and r['reason'] == 'foreign path or symlink escape refused' for r in control['boundary_controls']), 'foreign mutation boundaries')
    require(restored['state'] == restored['after_query'] == original and restored['source_git_clean'], 'restored bytes/source differ')
    for name in plan['owned_restore_files']:
        require(digest(safe(packet / 'backup', name + '.txt' if name.endswith('.go') else name)) == original[name]['sha256'], 'restore backup pin')
    require(original['graph/graph.db']['sha256'] == digest(ROOT / BASE / 'consumer-stdio-readiness-v1/index/graph.db'), 'owned index origin')
    query = packet / 'query'
    messages, lifecycle = read(query / 'messages.json'), read(query / 'lifecycle.json')
    require([r['label'] for r in messages] == ['initialize', 'restored-pinned-query'] and lifecycle['tool_requests'] == 1 and lifecycle['rpc_requests'] == 2, 'restored query denominator')
    req = messages[1]['request']
    require(req['params'] == {'name': 'cks.context.value_dependencies', 'arguments': restored['query']}, 'restored query pin')
    prior_query = read(ROOT / BASE / 'consumer-stdio-readiness-v1/execution.json')['query']
    prior_query['expected_snapshot_sha256'] = next(r['response']['result']['structuredContent']['snapshot_sha256'] for r in read(ROOT / BASE / 'consumer-stdio-readiness-v1/sessions/main/messages.json') if r['label'] == 'baseline')
    require(restored['query'] == prior_query, 'restored query source/snapshot identity')
    value = module('prepare-consumer-stdio-readiness.py').payload(messages[1]['response'])
    require(value['status'] == 'partial' and value['analysis'] is not None and value['snapshot_sha256'] == restored['query']['expected_snapshot_sha256'], 'actual restored partial query')
    notification = {'jsonrpc': '2.0', 'method': 'notifications/initialized'}
    require(lines(query / 'stdin.jsonl') == [messages[0]['request'], notification, req] and lines(query / 'stdout.jsonl') == [r['response'] for r in messages], 'query raw wire association')
    require(lifecycle['command'] == ['/private/tmp/ks-host-schema-runtime-20261017/bin/cks', 'mcp', '--config', ex['root'] + '/cks.yaml'] and lifecycle['exit_code'] == 0 and lifecycle['child_reaped'] and lifecycle['reader_joined'] and lifecycle['eof_sent'] and not lifecycle['terminate_sent'] and not lifecycle['kill_sent'], 'actual query cleanup')
    cleanup = read(packet / 'cleanup.json')
    require(cleanup['child_identity'] == restored['identity'] and cleanup['child_identity']['pid'] == lifecycle['pid'] and cleanup['child_identity']['command'] == ' '.join(lifecycle['command']), 'owned query identity')
    require(cleanup['child_probe']['pid'] == lifecycle['pid'] and cleanup['child_probe']['exit_code'] == 1 and not cleanup['child_probe']['stdout'].strip() and not cleanup['matching_identity_alive'] and cleanup['self_probe']['exit_code'] == 0 and str(cleanup['pid']) in cleanup['self_probe']['stdout'], 'current owned cleanup/positive control')
    require(cleanup['other_sessions_signalled'] == 0 and cleanup['retained_owned_fixture_for_review'] is True, 'unowned process/evidence cleanup boundary')
    listener = read(packet / 'listener.json')
    require(listener['address'][0] == '127.0.0.1' and listener['before_connect_errno'] == 0 and listener['after_connect_errno'] != 0 and listener['socket_fileno_after_close'] == -1 and listener['listener_owned_by_this_runner'] and listener['old_endpoints_not_contacted'], 'owned listener cleanup/positive control')
    history = read(packet / 'historical-process-audit.json')
    audits = {name for name in preservation()['files'] if name.endswith('/postrun-process-audit.json')}
    old_pids = {r['pid'] for name in audits for r in read(ROOT / name)['probes']}
    require(set(history['source_audits']) == audits and history['source_audits'] == {name: digest(ROOT / name) for name in sorted(audits)}, 'historical audit source inventory')
    require({r['pid'] for r in history['probes']} == old_pids and len(history['probes']) == len(old_pids) and all(r['exit_code'] == 1 and not r['stdout'].strip() for r in history['probes']) and history['self_probe']['exit_code'] == 0 and str(history['pid']) in history['self_probe']['stdout'], 'historical PID observations')
    p07 = read(ROOT / BASE / 'followups/consumer-render-lifecycle-v1/render-lifecycle.json')
    require(p07['historical_unconfirmed_CKS_exit_receipts'] == 1 and history['PID_absence_is_exit_receipt'] is False, 'missing historical exit cannot be repaired by PID absence')
    commits = read(packet / 'commit-boundary.json')
    expected_packets = sorted(name for name in preservation()['files'] if name.startswith(str(BASE) + '/') and name.endswith('/manifest.json'))
    # Only preparation packet manifests, not nested source Git manifests.
    expected_packets = [name for name in expected_packets if Path(name).parent.parent in [BASE, BASE / 'followups']]
    require(set(commits['packet_commits']) == set(expected_packets) and len(expected_packets) == 35, '35 committed packet inventory')
    for name, commit in commits['packet_commits'].items():
        blob = subprocess.check_output(['git', 'show', commit + ':' + name], cwd=ROOT)
        require(sha(blob) == digest(ROOT / name) and subprocess.run(['git', 'merge-base', '--is-ancestor', commit, BASE_COMMIT], cwd=ROOT).returncode == 0, 'packet local commit/ancestry proof')
    require(commits['base_commit'] == BASE_COMMIT and commits['base_signature_exit'] == 0 and commits['remote_contact_performed'] is False and commits['push_PR_tracker_operations'] == 0 and commits['self_commit_embedded'] is False, 'local publication/commit boundary')
    matrix = coverage()
    require(read(packet / 'case-matrix.json') == matrix and [r['id'] for r in matrix] == ['T%02d' % i for i in range(1, 23)], '22-case coverage/scope')
    require(read(packet / 'handoff.json') == handoff(), 'handoff formal/adoption/remaining scope')
    for item in matrix:
        if 'manifest' in item:
            require(item['manifest'] in preservation()['files'], 'case evidence outside preserved packets')
    return {'test_id': 'T22', 'step_id': 'P08', 'status': 'isolated_final_readiness_pass_not_adopted', 'full_plan_passed': 22, 'full_plan_pending': [], 'P01_P08_isolated_complete': True, 'protected_global_configuration_entries': len(global_files), 'operational_metadata_entries': len(ops['metadata']), 'existing_unowned_engines_preserved': len(ops['engines']), 'maintained_protected_files': len(src['maintained_files']), 'signed_clean_source_checkouts': 5, 'prior_frozen_packets_unchanged': 35, 'prior_packet_local_commits_verified': 35, 'historical_PIDs_observed_absent': len(old_pids), 'historical_unconfirmed_CKS_exit_receipts': 1, 'historical_exit_receipts_reconstructed_from_PID_absence': False, 'owned_restore_exact': True, 'foreign_path_and_symlink_refusals': 2, 'restored_real_CKS_queries': 1, 'new_CKS_exit0_reaped': 1, 'new_owned_listener_closed': True, 'protected_before_after_equal': True, 'global_preservation_scope': 'current_T22_window_only', 'operational_full_database_or_availability_verified': False, 'self_commit_needs_post_creation_verification': True, 'actual_host_sessions': 0, 'real_model_calls': 0, 'formal_runs': 0, 'candidate_adopted': False, 'production_mutations': 0, 'external_publication': 0, 'formal_completed': 0, 'formal_total': 9, 'ontology_effect_acceptance': 'not_evaluated', 'next': handoff()['next']}


def prepare(source, output):
    require(not output.exists(), 'fresh output required')
    output.mkdir(parents=True)
    for name in TOP:
        shutil.copyfile(source / name, output / name)
    (output / 'query').mkdir()
    for name in QUERY:
        shutil.copyfile(source / 'query' / name, output / 'query' / name)
    shutil.copytree(source / 'backup', output / 'backup')
    (output / 'backup/source/logic.go').rename(output / 'backup/source/logic.go.txt')
    dump(output / 'formal-state.json', module('prepare-source-review-supplement.py').formal_state(ROOT))
    keep = preservation()
    dump(output / 'preservation.json', keep)
    audits = sorted(name for name in keep['files'] if name.endswith('/postrun-process-audit.json'))
    pids = sorted({r['pid'] for name in audits for r in read(ROOT / name)['probes']})
    def probe(pid):
        p = subprocess.run(['ps', '-p', str(pid), '-o', 'pid='], capture_output=True, text=True)
        return {'pid': pid, 'exit_code': p.returncode, 'stdout': p.stdout}
    dump(output / 'historical-process-audit.json', {'source_audits': {name: digest(ROOT / name) for name in audits}, 'pid': os.getpid(), 'self_probe': probe(os.getpid()), 'probes': [probe(pid) for pid in pids], 'PID_absence_is_exit_receipt': False})
    packets = sorted(name for name in keep['files'] if name.startswith(str(BASE) + '/') and name.endswith('/manifest.json'))
    packets = [name for name in packets if Path(name).parent.parent in [BASE, BASE / 'followups']]
    verified = subprocess.run(['git', 'verify-commit', BASE_COMMIT], cwd=ROOT, capture_output=True)
    dump(output / 'commit-boundary.json', {'base_commit': BASE_COMMIT, 'base_signature_exit': verified.returncode, 'packet_commits': {name: subprocess.check_output(['git', 'log', '-1', '--format=%H', BASE_COMMIT, '--', name], cwd=ROOT, text=True).strip() for name in packets}, 'remote_contact_performed': False, 'push_PR_tracker_operations': 0, 'self_commit_embedded': False})
    dump(output / 'case-matrix.json', coverage())
    dump(output / 'handoff.json', handoff())
    dump(output / 'boundary.json', derive(output))
    (output / 'REPORT.txt').write_text('준비36/P08/T22 — 격리 후보 최종 준비22/22, 정식0/9\n현재 T22 전후의 전역 설정 hash/MCP projection·프로젝트 설정·운영 dataset metadata/current pointer·기존 엔진 식별자·원본/후보5개 서명 checkout·바이너리/보호 source·remote refs 일치. 과거 기준선 없는 전역 설정 구간까지 소급 증명하지 않음. 실제 운영 DB 전체 byte/가용성/인증 검증 아님.\n새 소유 fixture의 source/index/config/active pointer4변경을 검출하고 backup bytes로 정확 복구. foreign·symlink escape2경로는 mutation 전 거부. 복구 뒤 pin/snapshot 실제 CKS1조회 partial·exit0/reap/reader EOF·실제 owned listen 양성대조 후 close 확인. 다른 사용자 세션 ChainBench는 보존하며 신호0. 임시 증거/fixture는 검토용으로 유지.\n이전35동결 패킷과 local commit/ancestry를 보존. 역사 PID 현재 부재를 당시 종료 영수증으로 대신하지 않으며 초기 미확인 CKS exit1건과 초기 실패를 그대로 유지. 전체22사례는 격리 준비 범위이며 모델 자율 준수·의미 품질·온톨로지 효과/정식 독립 수용 아님.\npush/PR/tracker/운영 쓰기/후보채택0. 자기 자신을 포함한 최종 commit hash는 패킷 내부에 넣을 수 없으므로 생성 후 실제 서명·clean tree를 별도로 확인한다. 다음은 독립 검토자/최종 담당자·DEV/FINAL/gold/매칭/자원/실행 범위 확정 후 정식9개 평가. 별도 후속12개 유지.\n')
    (output / '.gitattributes').write_text('**/*.db binary\n')
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'P08_T22_final_owned_readiness_not_adoption_or_acceptance', 'input_bindings': {str(p): digest(ROOT / p) for p in INPUTS}, 'generator_sha256': digest(Path(__file__)), 'artifacts': {p.relative_to(output).as_posix(): digest(p) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('T22 frozen:22/22 isolated readiness;35 prior committed packets; formal0/9/adoption0')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.source.resolve(), args.output.resolve())
