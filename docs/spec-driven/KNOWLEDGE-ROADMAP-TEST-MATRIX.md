# 지식 매니저·프로젝트 비교 전체 작업·시험표

2026-10-15 현재 단계는 KM-01 공통 격리 계층 검증이다. 완료한 설계/계약과 아직 연결하지 않은 공개 기능을 분리한다. 실제 상태는 [진행 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json), 목적과 전체 완료 기준은 [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), 기존 운영·선택 후속 범위는 [전체 작업표](../REMAINING-WORK.md)를 따른다.

표의 **통과**는 명시한 시험 범위만 뜻한다. **부분 검증**은 완료 표시가 아니며 **미실행**을 통과0건으로 바꾸지 않는다. 신규7개 기능 전체 완료는0/7이다.

## 전체28개 세부 작업과 시험 단계

| ID | 목적·시험 범위 | 구현/작업 단계 | 검증 단계 |
|---|---|---|---|
| KM-01.1 | 신원·key·cache·join·쓰기 조사와 이행 설계 | 완료 | [재사용/결손 표](./PROJECT-COLLECTION-ISOLATION.md) 작성·코드 대조 |
| KM-01.2 | Project/Version/Collection과 고정 scope 계약 | 완료 | catalog·fork·두 버전·직렬화/재시작 시험 통과 |
| KM-01.3 | 구현 대상 제한·명시 비교·잘못된 scope 거부 | 진행 중 | 공통 reader 시험 통과; CLI/MCP adapter 미연결 |
| KM-01.4 | 동명·fork·두 버전·cache·재시작·쓰기/호환 반례 | 진행 중 | 기반40사례 통과; 실제 backend/공개 도구 쓰기·혼입 시험 미실행 |
| KM-02.1 | legacy/versioned capability와 새 요청·상태 계약 | 미착수 | 미실행 |
| KM-02.2 | 비동기 build/status/cancel과 정합 gate | 미착수 | 미실행 |
| KM-02.3 | idempotency/lock/중단/실패/rollback | 미착수 | 미실행 |
| KM-02.4 | 공개 도구의 잘못된 대상·부분 실패·취소 보호 | 미착수 | 미실행 |
| KM-03.1 | 매니저 등록·설치·초기 구축·복구 절차 | 미착수 | 미실행 |
| KM-03.2 | 문서 없는 프로젝트의 근거 기반 지식 초안 | 미착수 | 미실행 |
| KM-03.3 | 암묵지 질문·불확실성·검토/채택/회수 | 미착수 | 미실행 |
| KM-03.4 | 실제 Claude host 전체 구축·조회와 실패 복구 | 미착수 | 미실행 |
| KM-04.1 | 수동·정기·변경 trigger와 성공 기준 | 미착수 | 미실행 |
| KM-04.2 | 변경/삭제/no-op/stale와 재검토 | 미착수 | 미실행 |
| KM-04.3 | 중복·lease·재시작·retry·자원 상한 | 미착수 | 미실행 |
| KM-04.4 | 두 번 변경+no-op+실패 복구·실제 주기 실행 | 미착수 | 미실행 |
| KM-05.1 | 산업 개념·정의·별칭·비교 축 | 미착수 | 미실행 |
| KM-05.2 | 공통 개념과 프로젝트별 코드/정책/시험 연결 | 미착수 | 미실행 |
| KM-05.3 | exact/close/broader 등의 검토·버전 의미 | 미착수 | 미실행 |
| KM-05.4 | 비적용/미수집/미지원·L1/L2·network 차이 | 미착수 | 미실행 |
| KM-06.1 | 명시 버전·개념·축·시점·상한의 비교 요청 | 미착수 | 미실행 |
| KM-06.2 | 프로젝트별 근거 행렬/bundle | 미착수 | 미실행 |
| KM-06.3 | 상충·누락·자료 부족·부분 실패 상태 | 미착수 | 미실행 |
| KM-06.4 | 둘 이상 실제 프로젝트의 비교·오인·근거 시험 | 미착수 | 미실행 |
| OE-01.1 | 질문·기대 근거·독립성·분모 고정 | 진행 중 | 로컬12질문 준비; 확대·독립 정답 검토 남음 |
| OE-01.2 | 같은 자료의 off/concept_text/relations/combined | 진행 중 | 로컬384회 실패/누락0; 확대 범위 남음 |
| OE-01.3 | 근거·순위·unknown·지연/비용·답변 효과 | 진행 중 | 위치/지연 관찰 통과; precision/recall·실제 답변/독립 품질 미실행 |
| OE-01.4 | 다중 프로젝트 개념 매핑·오인·비교 효과 | 미착수 | KM-05/06 이후 실행 |

## 이번 KM-01 공통 계층 시험 전체 목록

[시험 코드](../../internal/system/collection/collection_test.go)의17개 상위 시험, 하위 사례 포함40개다. 모두 race 모드에서 통과했다. fixture의 archive/manifest/실제 Git commit을 사용하며 공개 MCP나 실제 graph/vector 검색의 완료 근거로 확대하지 않는다.

| 시험 | 검증 내용 | 결과 |
|---|---|---|
| TestSameCommitForksAndNamesRemainSeparate | 같은 이름·URI·코드·commit의 fork를 별도 그룹/인용으로 유지 | 통과 |
| TestImplementationDispatchesOnlyExplicitTarget | 구현 대상1개만 backend 호출 | 통과 |
| TestTwoVersionsAndWarmProviderCannotCrossTargets | 두 버전·잘못된 warm cache 응답 거부·scope namespace 분리 | 통과 |
| TestRequestRejectionsNeverDispatch | 누락/불일치 target·catalog·mode·참여자 등14개 거부 | 통과 |
| TestCatalogRoundtripRestartAndCallerMutation | 저장/재시작 같은 pin, caller 변경과 종료 후 요청 거부 | 통과 |
| TestCatalogMembershipDigestChangesAndOrderDoesNot | 순서 독립·멤버 변경 시 digest 변경 | 통과 |
| TestDuplicateRegistrationAndForkErrors | 중복 등록·공유 경로·잘못된 fork/origin 등9개 거부 | 통과 |
| TestStrictCatalogDecode | 중복 JSON key·unknown·후행 값·용량 제한 | 통과 |
| TestScopedLocalSymbolKeysKeepProjectVersionAndOrigin | 동명 import/symbol의 프로젝트·버전·origin 키 분리 | 통과 |
| TestForeignBodyAndHashesRejectedEvenWhenRestamped | 재서명된 오본문/해시 거부와 정상 정제 본문 허용 | 통과 |
| TestCurrentAliasesAndMismatchedArchivesRejected | symlink·다른 archive·미수집 origin 거부 | 통과 |
| TestComparisonFailureDiscardsEarlierGroup | 뒤 backend 실패 시 앞 그룹도 공개하지 않음 | 통과 |
| TestConcurrentTargetReadsAndClose | 동시8개 대상 읽기와 종료 이후 거부 | 통과 |
| TestCanceledReadDoesNotDispatch | 취소 요청은 backend 미호출 | 통과 |
| TestArchiveMutationDuringLaterComparisonDiscardsAll | 뒤 참여자 실행 중 앞 archive 손상 시 전체 폐기 | 통과 |
| TestCloseWaitsForInFlightRead | 진행 중 요청 전 lease 해제 방지 | 통과 |
| TestReaderLeaseAndExistingRollbackStayCompatible | 기존 GC reader 보호·rollback/current·단일 retained 읽기 호환 | 통과 |

초기 fixture의 commit manifest 누락, 작성 중 unused import, 읽기 전용 blob의 손상 재현 실패도 원자료에 보존한다. 최종 통과 로그와 구분한다.

## 공개 경로로 넘어갈 때 남은 시험

| 검증 대상 | 현재 상태 | 연결 작업 |
|---|---|---|
| 실제 CKG/CKV 검색과 project별 provider 구성 | 미실행 | KM-01.3/04와 공개 adapter |
| CLI/MCP의 scope 요청·오류·최종 인용 분리 | 미연결·미실행 | KM-01.3/04, KM-02/KM-06 |
| 다른 프로젝트/버전의 DB 쓰기 및 current 불변 | 미연결·미실행 | KM-02.3/04 |
| 실제 host 재시작·검색 cache 옵션/권한 전체 key | 미실행 | 공개 adapter와 KM-03 |
| 등록된 collection으로 산업 개념 비교 | 미구현·미실행 | KM-05/06 |
| 자연어 표현·검토된 구현 관계 보완 후 ontology 효과 | 미실행 | OE-01 후속·KM-05 |

## 전체 검증 gate와 기록

[validation 원자료](../../system/eval/knowledge-roadmap-2026-10-15/km-01-foundation/validation.json)와 [Go 시험 전체3,605개 목록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-foundation/test-inventory.json)에 종료코드·사례별 상태를 보존했다. 아래 모듈 분모에 collection 사례가 이미 포함되므로40개를 다시 더하지 않는다.

| 최종 gate | 결과 |
|---|---|
| 전체 모듈 빌드 | 통과 |
| 전체 Go 시험 | 3,586개 통과·19개 명시적 skip·실패0 (총3,605개) |
| collection race | 17개 상위 시험/40사례 통과·skip0 |
| fmt | 통과 |
| lint·vet·engine 경계 | 통과, lint0 issues |
| 문서 | 125개, 문서 계약20개·패키지 안내3개 통과 |
| 작업표 허위 완료 방지 | 7개 시험 통과 |
| 기존 source/input/bundle 바이트 결속 | 2,069개 통과 |
| 새 Go 파일을 포함한 기존 소스목록 guard | 1,647개 결속, 검사 통과 |

전체 모듈에서 skip된19개는 실제 모델/자체 graph/특정 플랫폼·환경 지정 등이 필요한 기존 조건부 시험이며 통과로 세지 않았다. 목록에19개 모두 `state=skip`으로 보존한다. Go가 package skip으로 보고한12개 패키지는 별도 집계한다.

[증거 manifest](../../system/eval/knowledge-roadmap-2026-10-15/km-01-foundation/manifest.json)는 실행 로그·전체 시험 목록과 소스를 결속한다. 기존 source 목록에는 새 Go3개를 추가한 [별도 바이트 결속](../../system/eval/knowledge-roadmap-2026-10-15/km-01-foundation/source-binding.json)을 만들고 이전 승인·완료 판정은 유지한다. [이전 ontology 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)의384회 및 별도8회와 이번 collection 시험 분모는 합치지 않는다.

다음은 KM-01.3의 실제 backend·공개 조회 adapter 연결과 경계 검증이다. 이후 KM-02.1의 version-aware 도구 계약으로 이어간다. 신규 기능7개 외의 운영·독립 수용9개와 선택 후속12개는 [전체 작업표](../REMAINING-WORK.md)에 그대로 유지한다.

소스 결속 갱신을 완료했다. [별도 검사 기록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-source-binding-update/receipt.json)은 구현 커밋을 참조하며, 기존 원장의 나머지 필드·승인·완료 판정이 그대로임을 확인한다. 기반 packet의 `pending committed manifest`는 첫 커밋 시점의 기록이며 현재 상태는 이 후속 기록을 따른다.
