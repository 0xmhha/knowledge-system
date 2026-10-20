#!/usr/bin/env python3
"""Materialize 30 source-grounded, exposed DEV inputs and AI gold drafts."""
import argparse
import gzip
import json
from pathlib import Path
import subprocess

from wi1_dev_gold import BASE, FUNCTIONS, PACKET, ROOT, SOURCES, SOURCE_PATHS as SOURCE_REFS, TOOL_PATHS as TOOLS, derive_gold, public_case, read, render_review, sha, summarize


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def compress(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(gzip.compress(json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode(), mtime=0))


def seal(output):
    write(output / 'manifest.json', {
        'schema_version': 1, 'kind': 'WI1_exposed_DEV_complete_bounded_AI_drafts_not_formal_gold',
        'sources': [dict(path=p, sha256=sha((ROOT / p).read_bytes())) for p in SOURCE_REFS],
        'tools': [dict(path=p, sha256=sha((ROOT / p).read_bytes())) for p in TOOLS],
        'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file() and p != output / 'manifest.json'},
    })


def prepare(output, observer):
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        raise ValueError('refusing to overwrite evidence')
    candidates = read(ROOT / FUNCTIONS)['functions']
    files = {r['Path']: r for r in read(ROOT / SOURCES)}
    inputs = []
    for path, original in files.items():
        starts = [c['source']['start_byte'] for c in candidates if c['source']['path'] == path]
        inputs.append(dict(original, Starts=starts))
    stdin = json.dumps(inputs, ensure_ascii=False).encode()
    result = subprocess.run([str(observer)], input=stdin, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
    write(output / 'observer-receipt.json', {'argv': [str(observer)], 'exit_code': result.returncode,
        'executable_sha256': sha(observer.read_bytes()), 'helper_sha256': sha((ROOT / TOOLS[0]).read_bytes()),
        'stdin_sha256': sha(stdin), 'stdout_sha256': sha(result.stdout), 'stderr_sha256': sha(result.stderr),
        'candidate_valueflow_imported_or_invoked': False, 'shared_dependency_bias': 'same pinned x/tools CFG constructor as candidate; no independent oracle claim'})
    (output / 'observer-stderr.txt').write_bytes(result.stderr)
    (output / 'observer-stdout.json.gz').write_bytes(gzip.compress(result.stdout, mtime=0))
    result.check_returncode()
    observations = json.loads(result.stdout)
    compress(output / 'observations.json.gz', observations)
    controls = read(ROOT / 'scripts/fixtures/wi1-dev-controls.json')
    control_inputs = [dict(Path='controls/' + c['name'] + '.go', Source='package devcontrols\n' + c['source'] + '\n', Starts=[len(b'package devcontrols\n')]) for c in controls]
    control_stdin = json.dumps(control_inputs, ensure_ascii=False).encode()
    control_result = subprocess.run([str(observer)], input=control_stdin, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
    control_result.check_returncode()
    compress(output / 'controls/observations.json.gz', json.loads(control_result.stdout))
    write(output / 'controls/receipt.json', dict(exit_code=control_result.returncode, stdin_sha256=sha(control_stdin),
                                               stdout_sha256=sha(control_result.stdout), source='hand_authored_synthetic_reference_controls_not_project_or_holdout'))
    observed = {(o['path'], o['span']['start']): o for o in observations}
    golds = []
    for candidate in candidates:
        source = candidate['source']; raw = files[source['path']]['Source'].encode()
        assert sha(raw) == source['file_sha256']
        excerpt = raw[source['start_byte']:source['end_byte_exclusive']]
        assert sha(excerpt) == source['function_sha256']
        target = output / ('public/source/' + candidate['candidate_id'] + '.go.txt')
        target.parent.mkdir(parents=True, exist_ok=True); target.write_bytes(excerpt)
        gold = derive_gold(observed[(source['path'], source['start_byte'])], candidate)
        compress(output / ('private/' + candidate['candidate_id'] + '.json.gz'), gold); golds.append(gold)
        (output / ('private/' + candidate['candidate_id'] + '-REVIEW.txt')).write_text(render_review(gold, observed[(source['path'], source['start_byte'])]))
    write(output / 'public/DEV-inputs.json', [public_case(c) for c in candidates])
    write(output / 'SUMMARY.json', summarize(golds))
    write(output / 'matching-contract.json', {
        'status': 'AI_proposed_not_frozen', 'project': 'go-stablenet', 'source_commit': candidates[0]['source']['repository_commit'],
        'identity': ['project', 'commit', 'file/function SHA256', 'absolute UTF8 half-open byte spans', 'declaration identity', 'occurrence role and phase'],
        'data_unit': 'own-binding definition/use pair; same-spelling cross-scope pairs separately negative',
        'control_unit': 'CFG branch/target block pair; node projection separate, no mixing denominators',
        'canonical_node': 'hash of source function, block role/anchor/node descriptors and duplicate ordinal; raw CFG index is not sole identity',
        'branch': 'successor ordinal within pinned x/tools CFG; true/false translation to actual consumer pending',
        'entry': 'receiver/parameters/named-result zero; const compile-time mapping remains unknown',
        'bare_return': 'implicit named-result read at actual return span, separate declaration coordinate',
        'unsupported': 'range/escaped/captured/unmapped/unknown-type/constant domain retained unknown; closures are not executed; no memory mutation or short-circuit/exception feasibility claim',
        'empty_sets': 'an enumerated finite universe may be empty; no missing/null labels and no inferred safety',
        'actual_candidate_mapping': None, 'retained_snapshot_pin': None, 'independent_review': None,
        'scoring_allowed': False, 'precision_recall': None,
    })
    prior = read(ROOT / BASE / 'followups/independent-freeze-v1/WORKLIST.json')
    prior.update(current_stage='44_WI1_DEV30_bounded_complete_AI_gold_drafts',
                 completed_in_this_scope=['WI1_DEV_inputs30', 'WI1_bounded_AI_gold30', 'WI1_source_matching_and_reference_verification'],
                 next_task='WI1_semantic_unknown_and_actual_runtime_matching_review_then_WI2_DEV_AI_drafts')
    prior['AI_DEV_draft_progress'] = {'WI-1': '30/30_bounded_drafts_complete_not_independent_gold',
                                     'remaining_work_items': ['WI-2', 'WI-3', 'WI-4', 'WI-V1', 'WI-V2', 'WI-6', 'WI-SP', 'WI-C']}
    prior['remaining_all'][0].update(status='WI1_AI_DEV30_prepared_other_AI_drafts_and_independent_inputs_pending',
        task='WI1 의미 unknown/실제 매칭 보완 및 나머지8개 WI DEV/AI 초안; D04 역할·독립성, 새 DEV/FINAL 가족·완전 gold·매칭·충분성·F01~08 동결')
    write(output / 'WORKLIST.json', prior)
    lines = ['WI-1 DEV 입력·완전한 범위 내 AI 정답 초안 — 단계44 /2026-10-18', '',
        '기존 go-stablenet30함수를 그대로 DEV로 지정한다. 새 독립 FINAL·승인 gold·정식 수용은0이다.',
        'public/은 입력/원문뿐이며 private/에는 AI 제안 정답을 별도로 둔다. 이 디렉터리 전체를 host에게 제공하지 않는다.',
        '완전성의 의미: 명시한 지역 binding/CFG universe의 모든 정의–사용/branch–block 쌍을 positive/negative/unknown/excluded로 열거한다.',
        '이는 지원 밖 의미까지 모두 아는 완전한 운영 정답이 아니다. unknown을 음성이나 무해함으로 바꾸지 않는다.',
        '원문을 Go parser/types와 고정 x/tools CFG로 관측하고, 새 Python 참조의 kill-free path 탐색으로 정의–사용을 작성한다.',
        '별도 transfer-set 알고리즘과 전수 대조하며, 제어 관계는 path-cut 후지배와 fixed-point 두 방식으로 전수 대조한다.',
        '후보 valueflow 엔진 결과를 정답으로 사용하지 않았다. 단 CFG 생성기는 후보와 공유하므로 공통 오류 위험이 있으며 독립 오라클이 아니다.',
        '부분 파일 타입·range·주소 노출·closure capture·상수/합성 좌표·미해결 identifier와 runtime matching의 한계를 그대로 보존한다.',
        'source/file/function SHA와 UTF8 byte span·선언·read/write phase·synthetic entry/return·shadow를 구분한다.',
        '실제 retained query·SDK/MCP 경로와 baseline·최종 매칭 동결·정확도 점수는 실행/채택하지 않는다.', '',
        '함수별 초안 현황 (P/N/U/X=positive/negative/unknown/excluded; 정답 승인 수가 아님)',
        'ID | data P/N/U/X | control P/N/U | shadow negative']
    for gold in golds:
        d = gold['checks']['data_labels']; c = gold['checks']['control_labels']
        lines.append(gold['case_id'] + ' | ' + '/'.join(str(d.get(k, 0)) for k in ('positive', 'negative', 'unknown', 'excluded')) + ' | ' + '/'.join(str(c.get(k, 0)) for k in ('positive', 'negative', 'unknown')) + ' | ' + str(gold['checks']['shadow_negative_rows']))
    lines += ['', '검증은 validation-summary.json. 기존 소스와 승인 입력이 같을 때만 이전 native 전체 결과를 참조하며 새 Go 전체 실행으로 기록하지 않는다.',
              'D04 미정·freeze null·execution_allowed=false·정식0/9·기본 ontology off를 유지한다.',
              '다음은 WI1 unknown/value-domain/합성 좌표와 실제 소비자 매칭 보완이며, 다른 WI의 DEV/AI 초안은 별도 순차 작업이다.',
              '남은 전체는 WORKLIST.json의 FORMAL-PREP/FORMAL-9/ONTOLOGY/OPTIONAL-12와 AI_DEV_draft_progress를 따른다.']
    summary = summarize(golds)
    lines += ['', '현재 초안 합계',
              f"30함수/25파일, 지역 binding{summary['bindings']}개·occurrence{summary['occurrences']}개·함수형 signature{summary['quarantined_signature_occurrences']}개 별도 격리.",
              f"정의–사용쌍{summary['data_pairs']}개:positive{summary['data_labels'].get('positive',0)}/negative{summary['data_labels'].get('negative',0)}/unknown{summary['data_labels'].get('unknown',0)}, 별도 shadow 음성{summary['shadow_negatives']}쌍.",
              f"구조 제어 branch–block쌍{summary['control_pairs']}개:positive{summary['control_labels'].get('positive',0)}/negative{summary['control_labels'].get('negative',0)}. 이는 후보 엔진의 정확도 점수가 아니다.",
              'private/SRC-xxx-REVIEW.txt는 사람이 읽을 수 있는 전체 use별 정답·블록 매칭·제어 양성/unknown·음성 complement·한계 검토표다.',
              '부족한 의미 정답·호출/alias/타입·실제 consumer 매칭 및 독립 검토를 완료로 체크하지 않는다.',
              '이전 단계43의 도구 결속 검사는0f28a9a7 소스 view에서49개 재검증하며 현재 소스 시험과 구분한다.',
              '현재 guard는 새 WI1 DEV 종류를 검사하고 단계43 packet/manifest/소스 버전은 그대로 보존한다.']
    (output / 'REPORT.txt').write_text('\n'.join(lines) + '\n')
    seal(output)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=Path, default=ROOT / PACKET)
    parser.add_argument('--observer', type=Path, required=True)
    args = parser.parse_args(); prepare(args.output, args.observer)
