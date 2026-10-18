#!/usr/bin/env python3
"""Prepare exposed DEV authoring material; never select or open FINAL inputs."""
import argparse
from collections import Counter
from pathlib import Path

from independent_acceptance import (
    BASE, PACKET, ROOT, SUPPLEMENT, WORK_IDS, authoring_index, binding_paths, digest, gold_form, native_reuse_record, read, registry,
)
import json


def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def seal(output):
    sources, tools = binding_paths()
    write(output / 'manifest.json', {
        'schema_version': 1, 'kind': 'authoring_preparation_only_not_acceptance',
        'source_refs': [dict(path=p, sha256=digest(ROOT / p)) for p in sorted(sources)],
        'tool_refs': [dict(path=p, sha256=digest(ROOT / p)) for p in tools],
        'artifacts': {p.relative_to(output).as_posix(): digest(p) for p in sorted(output.rglob('*')) if p.is_file() and p != output / 'manifest.json'},
    })


def prepare(output):
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        raise ValueError('refusing to overwrite existing evidence directory')
    protocol = read(ROOT / BASE / 'OPERATIONAL-ACCEPTANCE-PROTOCOL.json')
    proposal = read(ROOT / BASE / 'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json')
    records = registry()
    write(output / 'candidate-ledger.json', {
        'status': 'exposed_DEV_material_not_fresh_holdout',
        'counts_by_stratum': dict(Counter(r['stratum'] for r in records)),
        'records': records, 'formal_FINAL_cases': [], 'reviewed_independent_units': 0,
        'shared_ticket_counted_once': True,
        'WI2_project_calls_are_not_independent_functions': True,
        'cross_work_item_family_overlap_requires_review': True,
    })
    write(output / 'gold-authoring-forms.json', [gold_form(i) for i in (*WORK_IDS, 'ONTOLOGY')])

    # Preserve all source-grounded draft answers and omissions by immutable row
    # reference. A partial anchor never fills a complete answer set.
    write(output / 'private-authoring-index.json', authoring_index(records))

    counts = {
        'WI-1': (30, 'functions', '30/30 data and control sets incomplete; 24 lack any prior relation material'),
        'WI-2': (40, 'controlled drafts; plus 68 project rule calls', '2 rules; 10 positive +10 negative per rule proposed, approved labels zero; project/controlled separate'),
        'WI-3': (20, 'input cards /15 proposed families', 'five-family floor shortfall even before independence/split review; all cards exposed'),
        'WI-4': (5, 'proposed mechanisms', 'actual entry reachability/oracles/plans missing in all five; 21 schedules are correlated variants'),
        'WI-V1': (1, 'exposed development ticket', 'two additional source hypotheses and extracted-method runs are not two independently confirmed tickets'),
        'WI-V2': (1, 'same ticket shared with V1', 'acceptable membership null; two candidate patches do not supply independent ticket sufficiency'),
        'WI-6': (20, 'intent inputs /19 operational sources +1 stress source', 'stress main excluded from operational denominator; 16 seed overlaps with WI1; relevant/noise gold absent'),
        'WI-SP': (12, 'symbol proposals:4 policy/4 security/4 relative untagged', 'source-only saved policy edges cannot prove absent security labels; provenance and refusal gold absent'),
        'WI-C': (10, 'review proposals /8 module labels', '45 source anchors are not complete required-item/priority/report gold; all proposed'),
    }
    summaries = []
    for group in protocol['input_groups']:
        work_id = group['id']
        number, unit, gap = counts[work_id]
        summaries.append({
            'work_item_id': work_id, 'available_exposed_draft_count': number,
            'draft_count_unit': unit, 'known_gap': gap,
            'minimum_requirements_scope_approved_not_run_frozen': group['requirements_proposal'],
            'metrics_unchanged': group['metrics_proposal'],
            'FINAL_minimum_independent_units': group['final_minimum_independent_units'],
            'DEV_allocation': group['dev_sample_allocation'],
            'reviewed_independent_FINAL_units': 0, 'precision_or_power_evidence': None,
            'sufficiency_verdict': 'inconclusive_not_reviewed',
            'allocation_rule': 'declare FINAL count/coverage and separate DEV needs before collection; shared family counted once per relevant denominator',
        })
    summaries.append({
        'work_item_id': 'ONTOLOGY', 'available_exposed_draft_count': None,
        'reviewed_independent_FINAL_units': 0, 'FINAL_minimum_independent_units': 20,
        'intent_types_min': 4, 'DEV_allocation': None,
        'sufficiency_verdict': 'inconclusive_not_reviewed', 'precision_or_power_evidence': None,
        'known_gap': 'old language variants, 512 calls,40 cells and new six diagnostic concepts are not independent untouched FINAL families',
    })
    write(output / 'sample-sufficiency.json', {
        'groups': summaries, 'method': protocol['statistics_proposal'],
        'count_floor_never_certifies_precision': True,
        'precision_plan_before_FINAL': {
            'target_interval_width_or_power': None, 'expected_effect_and_variance': None,
            'reviewer_receipt': None, 'optional_stopping': 'forbidden',
            'method': 'use independent DEV family-level paired deltas for planning; fix sample size before FINAL; retain inconclusive if interval/gate undefined or too broad',
        },
        'repeat_plan': protocol['measurement_proposal'],
        'latency_method_proposal': proposal['measurement_draft']['latency_percentile_method_proposal'],
        'latency_warning': '20 warm timings are descriptive; no tail stability certification; keep process/model residency, arm order, raw timings and serial ownership',
        'missing_runs': 'keep planned/success/cancelled/infra/unreached/symptom separately; never silently drop missing results',
        'bootstrap_unit': 'independent family with all within-family repeats/languages/edges/cells clustered; paired arms resample together',
    })

    preparations = {
        'F-01': ('approved local scope with publication/production boundary retained', ['declare formal task-terminal and eligible API consumers before run']),
        'F-02': ('signed CKS38dd5c54, adopted consumer, real two-project source/data/model evidence referenced', ['freeze actual run tree/binary/config/schema/submodules and exact retained version pins; no moving current alias']),
        'F-03': ('ten task-specific complete-answer forms and matching contracts prepared', ['complete per-case universe/labels, canonical mapping, negative/unknown/abstention and baseline; independent content review']),
        'F-04': ('nine unchanged requirement/metric proposals and ontology gate; family counts and known shortfalls explicit', ['declare FINAL minima/DEV allocation and coverage; assess precision on independent DEV and lock sample size/CI/tail policy']),
        'F-05': ('user-confirmed unassigned; AI provenance and reviewer receipt fields explicit', ['assign gold author/reviewer/acceptor/operator; disclose author/result exposure and role overlap; no independent label for self-review']),
        'F-06': ('zero paid-model preparation; BGE identity in preserved pilot evidence', ['freeze local resource allocation and exact runtime identity; only if actual LLM is selected specify provider/model/auth/cost accounting and cap']),
        'F-07': ('206 exposed draft rows indexed; future FINAL reservation and separation procedure prepared', ['new unseen families/inputs, complete independent gold and review receipts; private immutable manifests/ACL/access logs outside host; hash commitments before run']),
        'F-08': ('completed source/error/real-project pilots and adopted local host evidence reusable within scope', ['resolve exact formal recipes/timeouts/ownership/cleanup and actual API/node reachability; scripted host is not actual LLM role quality']),
    }
    write(output / 'freeze-readiness.json', [dict(
        id=row['id'], topic=row['topic'], preparation=preparations[row['id']][0],
        remaining=preparations[row['id']][1], decision_status='pending', receipt=None,
    ) for row in protocol['freeze_requirements']])
    write(output / 'holdout-procedure.json', {
        'status': 'procedure_prepared_no_FINAL_material_created',
        'formal_DEV_manifest': None, 'formal_FINAL_manifest': None, 'assignment_seed': None,
        'family_review': 'new unseen semantic families required; audit shared function/call chain/defect/contract across work items and both projects; exact-byte/name checks only supplementary',
        'reservation': 'independent assigned reviewer selects after eligibility/stratification review; no filename-only or random hash claim of independence; separate DEV and FINAL source families',
        'before_execution': ['bind source/data/model/consumer/API pins', 'complete source-grounded gold universe and matching', 'record human identity/exposure/independence', 'lock DEV/FINAL family manifest and minimum/coverage', 'select one primary ontology mode using DEV only', 'freeze protocol and private evidence hashes before opening FINAL'],
        'host_visibility': 'only public input/source scope; no private draft/gold/labels/answer-location lists in prompt, settings, MCP roots or logs exposed to host',
        'private_storage': 'outside host-accessible roots, ACL verified and access logged; Git committed drafts here are exposed and cannot be sealed FINAL by renaming',
        'leak_or_code_change_after_FINAL': 'retire affected FINAL family to regression and collect new holdout; preserve old bytes and decision history',
        'human_receipt_is_not_machine_proof_of_truth': True,
    })
    write(output / 'ontology-contract.json', {
        'status': 'approved_method_scope_preparation_only_not_run_frozen',
        'gate': proposal['ontology_new_gate_proposal'],
        'scope_approval_source': 'D06 direct user approval in decision proposal; nested gate historical status is not a formal freeze receipt',
        'DEV_modes': ['off', 'concept_text', 'relations', 'combined'],
        'FINAL_mode': None, 'execution_allowed': False, 'reviewed_link_count': None,
        'primary_estimand': 'off vs one DEV-selected treatment on independent FINAL family-level macro recall@10; CI95 paired family clusters',
        'mapping_input_effect_separate': 'annotations withheld vs supplied is a separate contrast; cannot attribute to ontology vocabulary/ranking alone',
        'link_review': 'verify implementation→concept semantics and project/version/applicability/source spans before relations/combined; zero reviewed links is an inactive-arm limitation, not evidence of no ontology value',
        'pin_controls': ['same source/corpus/data/model digest/dimension/preprocessing', 'same top-k and candidate/local/aggregate caps', 'same tool route and owned hardware/concurrency', 'frozen ontology vocabulary and reviewed link bytes', 'counterbalanced order; warm/cold raw times separate'],
        'coverage': ['at least20 independent FINAL families and4 intent types', 'consensus/transaction/mempool and staking/finality only where source applicability verified', 'multiple projects explicit; different algorithms under shared concept need not have identical behavior', 'negative target-project controls and unavailable/unknown/not_collected/not_applicable distinguished', 'paraphrases/languages/repeats/cells remain within family'],
        'semantic_gold': 'source-reviewed full bounded relevant/irrelevant/unknown universe independent of returned hits, keyword match, or candidate-produced ontology mapping',
        'failure_policy': 'contamination>0 or citation/source mismatch blocks boundary; missing/undefined/small-sample result inconclusive; no demonstrated benefit keeps default off',
        'LLM_answer_quality': {'executed': False, 'inferred_from_retrieval': False, 'model_auth_budget': None, 'future_requirement': 'separate paired blinded answer rubric/factuality/citation/abstention review with approved actual model and cost cap'},
        'WI6_thresholds_unchanged': True, 'multiplicity': 'one primary FINAL contrast; exploratory arms labelled or predeclared adjusted multiplicity plan',
    })
    write(output / 'formal-state.json', {
        'D04': '미정', 'roles': None, 'freeze': None, 'execution_allowed': False,
        'approved_gold': 0, 'frozen_FINAL_cases': 0, 'formal_completed': 0, 'formal_total': 9,
    })
    write(output / 'native-reuse.json', native_reuse_record())
    write(output / 'validation-summary.json', {'status': 'not_run', 'formal_completed': 0,
                                             'formal_total': 9, 'reviewer_assignment': '미정',
                                             'fresh_Go_full_tests_this_stage': False})
    previous = read(ROOT / SUPPLEMENT / 'followups/stablenet-comparison-v1/WORKLIST.json')
    previous.update(current_stage='43_independent_freeze_authoring_prepared_not_frozen',
                    completed_in_this_scope=['FP01_exposure_inventory', 'FP02_gold_matching_authoring_contract', 'FP03_sufficiency_freeze_preparation', 'FP04_ontology_evaluation_preparation'],
                    next_task='FORMAL-PREP_assign_roles_collect_new_families_complete_and_review_gold')
    previous['remaining_all'][0].update(status='mechanical_preparation_complete_human_and_new_inputs_pending')
    write(output / 'WORKLIST.json', previous)
    (output / 'REPORT.txt').write_text('''독립 수용 동결 준비 — 단계43 / 2026-10-18

목적: 입력 개수·반복 실행·원문 anchor를 독립 표본·완전 정답·수용으로 잘못 승격하지 않는다.
범위: D04 미정 상태에서 가능한 DEV 작성 자료·gold/matching 계약·충분성·F01~08·온톨로지 평가 준비.

[작성 완료] FP01: 기존 206개 후보 행을 원본 파일 SHA/행 SHA/좌표로 연결했다.
함수30, 증상20, 스케줄5, 공유 티켓1, 규칙 controlled40/project 호출68,
확장20, annotation12, 리뷰10이다. 이는206개 독립 사례가 아니다.
모두 이미 노출된 DEV 작성/회귀 자료이며 새 FINAL은0개다. source hypothesis2개를
독립 티켓2개로 세지 않는다. 서로 겹친 함수·호출 경로·의미 가족은 추가 검토한다.

[작성 완료] FP02:9개 WI와 온톨로지의 완전 정답 작성 양식10개 및206행 작성 인덱스.
원문 기반 AI 초안은 원본 row에 연결하며 복제·승인하지 않는다. 완전 정답은
bounded universe의 모든 단위를 positive/negative/unknown 또는 task별 집합으로
분류해야 한다. null은 누락, 빈 집합은 검토 근거가 필요한 실제 empty set이다.
Go 값/제어 edge·규칙 source/sink·검색 위치·실제 API oracle·후보 membership·
관련/잡음 함수·annotation provenance·리뷰 필수 항목/우선순위를 각각 정의한다.
matching은 프로젝트/버전/원문 hash·UTF-8 반개방 byte span·심볼/edge 종류에 결속한다.
이 양식은 완전 gold 그 자체가 아니다. 기존30함수 모두 complete data/control gold 미완료다.

[작성 완료] FP03: F01~08 각각 준비 근거와 남은 결정·자료를 분리했다.
WI3 입력20개=제안 가족15개, WI6 운영19개+개발 stress1개, V1/V2 공유 티켓1개.
정식 독립 승인 단위0개이고 기존9개 FINAL 배정/최소값은 미정으로 보존한다.
20회 지연·5회 검색 반복은 가족 수를 늘리지 않으며20 warm p95는 tail 안정성 증명이 아니다.
기존 paired family bootstrap95%/10000회/seed20261015를 보존한다.
독립 DEV의 효과/분산에 따른 정밀도·표본 계획은 결과 전 확정하며 optional stopping 금지.
기술적 하한 충족만으로 충분성 pass를 내리지 않는다. 새 FINAL은 별도 접근 제한/열람 기록과
가족 단위 manifest·hash를 먼저 동결한다. Git에 이미 공개한 자료를 이름만 바꿔 봉인하지 않는다.

[작성 완료] FP04: 온톨로지 DEV4모드→FINAL 사전 선택1모드 대비 off의 설계를 연결했다.
D06 방법 범위 승인과 실제 run freeze는 구분한다. FINAL20가족/4의도,
recall@10 +5%p·paired CI95 하한>0·precision 하락2%p 이내·warm p95비≤1.25,
프로젝트 혼입0·인용 원문 일치100% 기준을 보존했다. WI6의 별도 기준도 변경하지 않는다.
검토된 구현-개념 연결·의미 relevance gold·동일 pin/cap·교대 순서·negative scope가 필요하다.
매핑 추가 효과와 어휘/랭킹 효과, 검색 지표와 실제 LLM 답변 품질을 분리한다.
기존 제한 연구완료/inconclusive 및 기본 off 유지. 새 효과 점수·정답 승인은 없다.

미완료: D04 사람/독립성, 새 DEV/FINAL 선정, 완전한 정답의 의미 작성·독립 검토,
매칭/표본/recipe 실제 동결,9개 정식 실행·수용0/9, 필수 온톨로지 독립 효과 수용.
선택 후속12개는 WORKLIST.json의 별도 backlog 전체 목록을 따른다.
검증 결과는 validation-summary.json. 이번 변경은 Python/문서만이며 새 Go 전체 시험을
실행한 것으로 기록하지 않는다. 직전38dd5c54의 전체 native 결과는 unchanged source일 때만 참조한다.
운영 current·DB·원문·전역 설정·기존 증거를 변경하지 않으며 유료 모델/외부 게시/PR 실행0.
''', encoding='utf-8')
    seal(output)
    return output


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=Path, default=ROOT / PACKET)
    args = parser.parse_args()
    prepare(args.output)
