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

## 현재 우선 작업

2026-10-15 사용자 추가 요청을 [지식 매니저·비교 명세](./spec-driven/KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)와 [전용 진행 원장](./spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json)에 반영했다. 신규 기능 전체 완료는0/7이며 세부 작업은28개 중3개 완료다. OE-01의 단일 프로젝트 개발 관찰384회와 별도 진단8회를 실행했고 KM-01의 조사·catalog·실제 backend 공개 조회3/4세부 작업을 완료했고 최종 격리 반례 검증은 진행 중이다. KM-02~06은 미착수다. 독립 품질·확대 범위·다중 프로젝트 평가는 남는다. 문서 등록이나 준비 검증을 기능 완료로 세지 않는다.

| 우선순위 | ID | 작업과 기대 결과 | 상태 |
|---|---|---|---|
| P0 | OE-01 | 온톨로지 off/on 효과·근거 정확도·오인·지연/비용 평가. 단일 프로젝트부터 진행하고 다중 프로젝트 비교 평가로 확장 | 로컬384회 완료, 개선 미관찰·품질 inconclusive; 전체 진행 중 |
| P0 | KM-01 | 프로젝트/포크/버전 격리와 collection 스키마. 동명 코드·정책의 혼입 방지 | 기반40사례·실제 DB 공개24항목 통과; 세부3/4완료, 최종 격리 반례 검증 남음 |
| P0 | KM-02 | 버전 DB 구축·갱신을 위한 비동기 도구 계약. 기존 pinned 쓰기 거부를 유지하며 안전한 경로 추가 | 요구 등록, 설계 초안 |
| P0 | KM-03 | Claude 지식 매니저 플러그인. 문서가 없는 프로젝트도 수집·초안·검토 대기·DB 구축·조회까지 관리 | 필수 기능으로 채택, 미구현 |
| P0 | KM-04 | 매니저의 정기/변경 기반 갱신, stale/삭제·재시도·원자적 전환·복구 | 필수 기능으로 채택, 미구현 |
| P1 | KM-05 | 산업 공통 개념과 프로젝트별 근거 매핑. 같은 용어/다른 의미와 같은 역할/다른 용어 구분 | 요구 등록, 설계 초안 |
| P1 | KM-06 | 프로젝트별 staking·합의·mempool 등의 비교 근거를 구조화하여 MCP/LLM에 제공 | 요구 등록, 설계 초안 |

[정책 수정 overlay의 로컬 graph/CKS 검증](./spec-driven/POLICY-OVERLAY-ADOPTION.md)을 완료했다.7개 수정·저장된146개 연결·실제 source/canonical/음성 제어를 확인했고 원래 생성기는 아직 기존 참조를 재생성하므로 일반 재생성 보완은 남는다. [OE-01 로컬 관찰](./spec-driven/ONTOLOGY-EFFECT-DEV-OBSERVATION.md)에서384회 실패/누락0·인용3,616개를 검증했다. 활성288회 no_match·검색 순위 변화0이며 별도 정확 용어8회도 최종 boost0이다. 독립 정답·자연어 개념 해석/구현 연결 보완·다중 프로젝트 평가는 남는다. [KM-01 공개 조회](./spec-driven/PROJECT-COLLECTION-ISOLATION.md)를 실제 CKG/CKV에 연결했다. 세부3/4 완료, 실제 DB3개·MCP24항목·21호출을 검증했고 다음은 KM-01.4 최종 격리 반례 검증이다. [전체28개 작업·시험표](./spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md)에 각 단계와 미실행 시험을 표시한다. 확장 구현은 KM-01→02→03→04, 격리 계약 이후 KM-05→06, 이후 OE-01 다중 프로젝트 평가 순서다. 사용자는 수동 문서를 먼저 작성할 필요 없이 매니저가 근거 있는 지식 초안을 생성하도록 요청했다. 관찰하지 못한 설계 의도를 임의로 사실화하지 않는다.

로컬 설계·개발·회귀/도구 검증은 독립 평가 담당자·유료 모델/비용 지정 없이 진행한다. 해당 지정은 실제 모델을 호출하거나 독립 수용을 판정하는 범위에 적용한다. 아래9개 정식 평가 계획의 미동결이 이 개발 경로 전체를 차단하지 않는다.

## WI-5 — 지식 매니저로 구체화

WI-5의 기존 수집→ingest→검토→승격/회수→색인 검증 목적은 KM-03/04로 구체화했다. 수동 파일 ingest만이 아니라 최초 DB 구축과 지속 갱신을 관리하는 Claude 플러그인이 필수 범위다. KM-01/02는 그 기반이며 별도 플러그인 저장소·설치 방식은 구현 설계에서 정한다. WI-5를 위 목록에 다시 더해 중복 집계하지 않는다.

## 구현 후 남은 운영·독립 수용 검증

[대상·완료 기준 계획](./spec-driven/OPERATIONAL-ACCEPTANCE-PLAN.md)과 [별도 검증 원장](./spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json)에 로컬 Go pilot 후보·항목별 입력/제안 gate·필요 receipt를 정리했다. 계획은 draft이며 실행/수용은0/9다. [실행 protocol·동결 체크리스트](./spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.md)와 source inventory를 준비했고 단계0은 preparing이다. 함수40개·review10개·확장 intent20개 source 후보와4개 미해결 실행 recipe를 추가했다. [review/정답 초안](./spec-driven/OPERATIONAL-ACCEPTANCE-REVIEW-DRAFT.md)은10개 rubric45anchor·3개 비완전 값/제어 초안이며 독립 채택은0이다. [ChainBench 종료·시험 상태 격리 보완](./spec-driven/OPERATIONAL-ACCEPTANCE-READINESS-FIXES.md)은 후보 코드·개발 검증 완료이며 실제 설치/노드 검증은 남는다. coding-agent 게시 전 종료·Stop 재진입도 OP-CA-01 후보 b6e2abdc에서 개발 검증을 완료했다. 두 소비자는 [격리 host 채택·연결](./spec-driven/ISOLATED-HOST-ADOPTION.md) 범위에서 검증했다. coding-agent는 SessionStart를 보완한 b7e7868, ChainBench는 bdf5ad27을 채택했고 실제 CLI2.1.300의 로컬 시험 응답기 사례5/5를 통과했다. 실제 모델·전체 역할·CKS·노드 검증은 남는다. [정책·보안 입력 검토 자료](./spec-driven/POLICY-SECURITY-INPUT-REVIEW.md)에 기존 정책42개·참조146개, source 호출68개와 query 후보12개를 추가했다. 수동 보안4개는 미승인 metadata 제안이며 독립 gold는0이다. [실제 프런트엔드 진단](./spec-driven/POLICY-REFERENCE-DIAGNOSIS.md)에서6개 미연결과 경고 없이 다른 함수에 붙는1개를 확인했다. 수정7개는 별도 평가 profile에서 실제 graph 저장·CKS 조회까지 검증했다. 원본·운영 설정은 그대로이며 독립 의미 수용과 생성기 보완은 남는다. 다음 개발 경로는 위 우선 작업표를 따른다. 정식 독립 평가를 실행할 때에는 별도로 입력·검토자·모델/비용·범위/기준을 고정한다.

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

기존13개 중 온톨로지 효과 평가는 위 OE-01의 필수 우선 검증으로 이동했다. 아래12개는 선택적 후속 범위 목록이다. 기존 구현 완료 수에 더하지 않으며, 문서 정리가 새 기능 채택·운영 변경을 승인하지 않는다. 기능별 목적·담당·입력·설계·완료 기준과 실행 증거를 정한 뒤 진행한다.

| 후속 범위 | 현재 코드·증거의 경계 | 다음에 필요한 일 |
|---|---|---|
| 조직 인증·권한 어댑터 | 로컬 reviewer ID 기록·감사는 외부 신원 인증과 다름 | 조직 사용 범위 채택 시 인증·권한·서명·trust·거부/감사 조건과 실제 검증 |
| 공개 v2 CLI/HTTP·독립 graph export | 통합 MCP v2 생산자는 구현됨. 일반 --format=v2·독립 export 확장은 설계 범위 | 표면별 지원/거부·소비자 호환·원문/무결성·실제 실행 계약을 새 범위로 고정 |
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

문서의 옛 WI 잔여표 정리는 완료했다. 다음은 위 우선 작업표의 KM-01.4 최종 격리 반례 검증을 진행하며, 정책 생성기 보완과 OE-01의 확대·다중 프로젝트 검증도 유지한다. [운영·독립 수용 계획](./spec-driven/OPERATIONAL-ACCEPTANCE-PLAN.md)의 단계0 동결과9개 정식 검증은 별도 경로다. WI-5는 KM-03/04에 통합 집계하며 저장소 지정이 가능한 로컬 개발 전체를 중단시키지 않는다.

과거 재감사의 구현 부족분은 [감사 기록](./spec-driven/UPPER-WORK-ITEMS-AUDIT.md)에 남긴다. 최신9/9 구현을 과거 표로 되돌리지 않는다. 일반 검증 체크리스트의 빈 체크박스는 매 변경의 절차이며 프로젝트 잔여 기능으로 집계하지 않는다. 기존 worksheet catalog·pinned v2 쓰기 거부·온톨로지 MCP·정책/보안 질의를 새 미구현 항목으로 재등록하지 않는다.
