# 남은 전체 작업

2026-10-15 준비 상태 기준이다. 구현·문서 main 통합 근거는2026-10-14 기록을 따른다. 완료 상태는 [구현 원장](./spec-driven/UPPER-WORK-ITEMS-PROGRESS.json)과 기록된 증거를 따른다. 완료한 구현, 운영·독립 수용 검증, 새로 채택해야 할 후속 범위를 구분한다.

## 완료한 요청 범위

| 범위 | 확인한 완료 상태 | 현재 근거 |
|---|---|---|
| 상위 WI 선택9개 구현 | WI-V1/V2/3/4/1/2/C/6/SP 모두 implemented·implementation_verified=true | [현재 명세](./spec-driven/UPPER-WORK-ITEMS-IMPLEMENTATION.md), [진행 원장](./spec-driven/UPPER-WORK-ITEMS-PROGRESS.json) |
| knowledge-system 통합 | [PR #102](https://github.com/0xmhha/knowledge-system/pull/102) 스쿼시 머지·main 동기화 확인, 통합 commit a214fbb1. PR head e77e59b8의 CI5/5 성공 | [CI 실행](https://github.com/0xmhha/knowledge-system/actions/runs/38565382940) |
| go-stablenet 비교·출처 수정 | 요청3개 실행 완료 | [비교 작업표](./spec-driven/GO-STABLENET-DATASET-COMPARISON.md) |
| 서버 종료 보완 | SD-01–04 모두 완료 | [완료 근거](../system/eval/stablenet-shutdown-2026-10-14/completion.json) |
| 온톨로지 MCP 보완 | O-01–04 구현·명시 시험 범위 완료 | [리팩토링 원장](./spec-driven/REFACTORING-PROGRESS.json) |

상위9개 구현의 잔여는0개다. 이 수치는 아래의 독립 수용·새 기능·외부 운영 전체가 완료됐다는 의미로 확대하지 않는다. coding-agent·ChainBench 구현의 원본 commit/patch/bundle은 증거에 보존돼 있으며 knowledge-system PR의 머지로 다른 저장소 반영을 추정하지 않는다.

## 아직 구현 범위가 확정되지 않은 WI-5

WI-5는 암묵지 수집·관리자 학습을 위한 별도 ingest MCP/플러그인이다. 담당 저장소가 미확보여서 구현·실행 상태를 판정하지 않았다. 먼저 담당·프로젝트·수집 대상을 지정한 뒤 명령 수집 → 파일 ingest → 관리자 검토 → 승격·회수 → 색인 검증의 계약과 실제 실행을 고정해야 한다. 다른9개 구현의 완료 수에 포함하지 않는다.

## 구현 후 남은 운영·독립 수용 검증

[대상·완료 기준 계획](./spec-driven/OPERATIONAL-ACCEPTANCE-PLAN.md)과 [별도 검증 원장](./spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json)에 로컬 Go pilot 후보·항목별 입력/제안 gate·필요 receipt를 정리했다. 계획은 draft이며 실행/수용은0/9다. [실행 protocol·동결 체크리스트](./spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.md)와 source inventory를 준비했고 단계0은 preparing이다. 함수40개·review10개·확장 intent20개 source 후보와4개 미해결 실행 recipe를 추가했다. [review/정답 초안](./spec-driven/OPERATIONAL-ACCEPTANCE-REVIEW-DRAFT.md)은10개 rubric45anchor·3개 비완전 값/제어 초안이며 독립 채택은0이다. [ChainBench 종료·시험 상태 격리 보완](./spec-driven/OPERATIONAL-ACCEPTANCE-READINESS-FIXES.md)은 후보 코드·개발 검증 완료이며 실제 설치/노드 검증은 남는다. coding-agent 게시 전 종료·Stop 재진입도 OP-CA-01 후보 b6e2abdc에서 개발 검증을 완료했다. 다음은 두 소비자 후보의 채택·격리 host hook/권한 연결 및 새 입력/정답 검토다. 검토자·모델/비용·새 gold/사례·범위/기준 채택 기록도 필요하다.

9개 모두 독립 수용 상태는 not_recorded다. controlled fixture·공개 MCP 실행·CI 통과는 기록된 구현 검증이며, 실제 host/체인/전체 프로젝트·독립 검토자의 수용 기록은 각 범위를 먼저 확정해 진행한다.

| ID | 남은 운영·독립 수용 검증 | 담당 |
|---|---|---|
| WI-V1 | 실제 에이전트 host·프로젝트에서 오라클·완료 차단 검증과 독립 검토 | coding-agent |
| WI-V2 | 실제 후보 선택 결과의 독립 수용과 품질·비용 평가 | coding-agent |
| WI-3 | production 텍스트 입력과 설치된 전체 bugfix 역할의 연결 실행 | coding-agent+CKS |
| WI-4 | 실제 체인·host에서 반복 재현·방어·관찰·재발 검증 | coding-agent+ChainBench |
| WI-1 | 실제 프로젝트의 값·제어 의존 정확도·비용·지원 범위 수용 | CKG |
| WI-2 | 독립 보안 사례의 오탐·미탐과 규칙 효과 평가 | CKG/CKS |
| WI-C | 실제 에이전트 리뷰 절차·산출물의 독립 내용 검토 | coding-agent |
| WI-6 | 실제 corpus에서 확장 효과·noise·cost 평가 | CKS |
| WI-SP | 실제 소비자의 정책·보안 질의와 운영 권한·출처 경계 수용 | CKS+CKG |

## 별도 범위가 필요한 후속 작업

다음13개는 후속 범위 목록이다. 기존 구현 완료 수에 더하지 않으며, 문서 정리가 새 기능 채택·운영 변경을 승인하지 않는다. 기능별 목적·담당·입력·설계·완료 기준과 실행 증거를 정한 뒤 진행한다.

| 후속 범위 | 현재 코드·증거의 경계 | 다음에 필요한 일 |
|---|---|---|
| 조직 인증·권한 어댑터 | 로컬 reviewer ID 기록·감사는 외부 신원 인증과 다름 | 조직 사용 범위 채택 시 인증·권한·서명·trust·거부/감사 조건과 실제 검증 |
| 공개 v2 CLI/HTTP·독립 graph export | 통합 MCP v2 생산자는 구현됨. 일반 --format=v2·독립 export 확장은 설계 범위 | 표면별 지원/거부·소비자 호환·원문/무결성·실제 실행 계약을 새 범위로 고정 |
| 독립 온톨로지 효과·지연 | 구현·실측 기록 완료, 독립승인0·품질inconclusive·성능수용false | 새 독립 질문/사실군·원문·reviewer·사전 임계치/분모·새 동결·실측 |
| graph expansion 점수·관계 튜닝 | intent별 가중치·max 집계·hop 제한 구현 있음, 독립 최적성 미판정 | 새 per-intent 입력·분모·recall/noise/cost 기준을 동결하여 decay/count 보너스/관계 변화 비교 |
| domain flow 정답 평가 | flow 도구와 curated corpus 경로 있음 | 실제 정본 corpus·get_flow/find_branches 정답과 precision/recall |
| 대형 corpus throughput·장함수 sliding split | 코드·작은 fixture 시험 있음 | 실제 대형/긴 함수 입력·모델/하드웨어 pin·비용/회수율 측정 |
| PR 시점의 문서/flow 지식 | v2 원문 snapshot은 고정됨. 과거 PR에 어떤 도메인 시점 입력을 쓸지 별도 문제 | as-of 정책·과거 원천·leakage 판정과 평가 입력 동결 |
| legacy 본문 출처 보강 | flat 설정은 FilesystemFetcher, checkout drift 경고. v2 retained-source 읽기와 구분 | 유지 범위·코드/DocsRoots byte 검증·citation-only 실패 정책·health 출처 설계와 회귀; [현재 경계](../system/docs/design-pack-body-provenance.md) |
| native CoreML embedder | native 구현 있음, 일반 CLI의 embedder dispatch에는 연결되지 않음 | 지원/실험 범위 결정·명시 연결·native 검증 |
| embedding LRU cache | wrapper 구현 있으나 기본 생성 경로에 연결되지 않음 | 메모리/invalidations·효과 기준·명시 옵션 검토 |
| append-only audit integration | auditlog/observe 구현과 audit_dir 설정은 있음. 운영 composer에 연결 완료 근거 없음 | 감사 대상·보관/개인정보·권한·failure 정책과 실제 wiring |
| 선택적 viewer 상호작용 | 통합 viewer/Atlas 있음. BFS-ripple은 과거 UI 제안 | 채택 시 redraw·성능·접근성 완료 기준과 시험 범위 결정 |
| 배포 머신·다운스트림 동기화 | 로컬 코드/CI만으로 원격 서비스나 다른 저장소의 상태를 알 수 없음 | 필요 시 대상·권한을 정하고 실제 binary/dataset/model/agent/network/power 상태와 소비자 결과 확인 |

## 승인된 필수 잔여와 보류 결정

| 우선순위 | 작업 | 상태·착수 조건 | 완료에 필요한 증거 |
|---|---|---|---|
| 1 | N16 공개키·신뢰·권한 | 사용자 결정으로 보류. 재개 지시 전 키 생성이나 완료 체크를 하지 않음 | 실제 공개키 fingerprint·권한자·신뢰 경로·서명 대상·보관/교체/폐기·고지 적합성 판정·회귀 근거 |
| 2 | 태그·공개 배포 | 보류. 별도 범위·사용자 승인 필요 | 지원 대상, 신뢰·복구·내용/품질/비용 한계, 배포 대상과 권한을 구체적으로 판정 |

다음 필수 작업은 **N16 재개 여부 결정**이다. 보류 유지 중에는 승인 없이 구현을 이어갈 필수 항목이 없다. 완료한17개/4개 항목의 수용 조건과 증거는 [실행 명세](./spec-driven/REFACTORING-EXECUTION-SPEC.md), [이력 원장](./spec-driven/REFACTORING-PROGRESS.json), [범위별 작업표](./spec-driven/REFACTORING-REMAINING-WORKLIST.md)로 검증한다.

## 다음 순서와 중복 집계 방지

문서의 옛 WI 잔여표 정리는 완료했다. 다음은 [운영·독립 수용 계획](./spec-driven/OPERATIONAL-ACCEPTANCE-PLAN.md)의 단계0 대상·gold/reviewer·기준·모델/자원 한도를 검토·동결한 뒤, 단계1 runtime inventory와 설치된 소비자 상태를 확인하는 것이다. WI-5 담당 확보는 별도 의존성이며 다른 검증을 중단시키지 않는다.

과거 재감사의 구현 부족분은 [감사 기록](./spec-driven/UPPER-WORK-ITEMS-AUDIT.md)에 남긴다. 최신9/9 구현을 과거 표로 되돌리지 않는다. 일반 검증 체크리스트의 빈 체크박스는 매 변경의 절차이며 프로젝트 잔여 기능으로 집계하지 않는다. 기존 worksheet catalog·pinned v2 쓰기 거부·온톨로지 MCP·정책/보안 질의를 새 미구현 항목으로 재등록하지 않는다.
