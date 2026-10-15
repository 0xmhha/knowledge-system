# 지식 매니저·프로젝트 비교 전체 작업·시험표

2026-10-15 현재 단계는 KM-01.3 실제 backend·공개 MCP 조회 연결 완료다. 최종 반례 시험 KM-01.4는 진행 중이며 전체28개 중3개 완료·4개 진행 중·21개 미착수다. 실제 상태는 [진행 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json), 목적과 전체 완료 기준은 [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), 기존 운영·선택 후속 범위는 [전체 작업표](../REMAINING-WORK.md)를 따른다.

표의 **통과**는 명시한 시험 범위만 뜻한다. **부분 검증**은 완료 표시가 아니며 **미실행**을 통과0건으로 바꾸지 않는다. 신규7개 기능 전체 완료는0/7이다.

## 전체28개 세부 작업과 시험 단계

| ID | 목적·시험 범위 | 구현/작업 단계 | 검증 단계 |
|---|---|---|---|
| KM-01.1 | 신원·key·cache·join·쓰기 조사와 이행 설계 | 완료 | [재사용/결손 표](./PROJECT-COLLECTION-ISOLATION.md) 작성·코드 대조 |
| KM-01.2 | Project/Version/Collection과 고정 scope 계약 | 완료 | catalog·fork·두 버전·직렬화/재시작 시험 통과 |
| KM-01.3 | 구현 대상 제한·명시 비교·잘못된 scope 거부 | 완료 | 실제 CKG/CKV3DB·공개 MCP24항목 통과; 원시 응답/인용 재검증 |
| KM-01.4 | 동명·fork·두 버전·cache·재시작·쓰기/호환 반례 | 진행 중 | 기반40사례·공개 조회24항목 통과; 동일 바이트 fork·다른 embedding 구성·공개 쓰기 전체 표면 남음 |
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

## 이번 공개 MCP 검증 전체 목록

실제 graph/vector DB3개를 새로 만들고 MCP 프로세스3개를 실행했다. embedding은 `mock-feature-hash-v1`/64차원이며 유료 모델 호출0회다. 도구21호출 중12개 성공·9개 의도한 오류, 총26개 요청·응답이다. 성공한14개 evidence pack의 좌표·인용·보관 원문/범위 해시·본문·integrity를 재검증했다. 아래24개는 이21호출과 시작 거부·보존 검사를 묶은 항목으로 서로 더하지 않는다.

| 순서 | 확인 항목 | 결과 |
|---|---|---|
| 1 | 다른 프로젝트 vector index 설정 시작 거부 | 통과 |
| 2 | 중첩 collection 설정 시작 거부 | 통과 |
| 3 | 다른 sanitizer 설정 시작 거부 | 통과 |
| 4 | 사용 불가 embedding model 시작 거부 | 통과 |
| 5 | member profile 누락 시작 거부 | 통과 |
| 6 | mutable current 경로 시작 거부 | 통과 |
| 7 | A 이전 버전101 조회 | 통과 |
| 8 | B 버전202 조회 | 통과 |
| 9 | A 새 버전303 조회 | 통과 |
| 10 | B 재조회·warm 분리 | 통과 |
| 11 | A 이전 버전 재조회·warm 분리 | 통과 |
| 12 | 세 버전 명시 비교·각 그룹 분리 | 통과 |
| 13 | target 누락 거부 | 통과 |
| 14 | 오래된 catalog digest 거부 | 통과 |
| 15 | 다른 project 좌표 거부 | 통과 |
| 16 | 다른 snapshot 좌표 거부 | 통과 |
| 17 | 없는 collection 거부 | 통과 |
| 18 | caller config 경로 주입 거부 | 통과 |
| 19 | 미지원 mode 거부 | 통과 |
| 20 | B 원문 손상 시 비교 전체 거부·A 정상 유지 | 통과 |
| 21 | 원문 복원 후 비교 회복 | 통과 |
| 22 | 기존 pinned reindex 거부·job 미생성 | 통과 |
| 23 | 재시작·current/소스가 새 버전이어도 이전 버전 유지 | 통과 |
| 24 | 기본33도구·기존 단일 프로젝트 V2 호환 | 통과 |

새 Go 상위 시험6개, 하위 사례 포함14개가 모두 통과했다.

| 시험 | 검증 내용 | 결과 |
|---|---|---|
| TestCollectionConfigValidation | 비활성 기본값·절대 경로·중복·누락 등8개 설정 사례 | 통과 |
| TestCollectionConfigKnownFieldsRemainStrict | YAML unknown 필드 거부 | 통과 |
| TestCollectionToolsAreOptInAndReadOnly | 기본 도구 불변·선택2도구·schema와 readOnly | 통과 |
| TestCollectionServiceUsesExistingV2AndClosesOnce | 기존 V2 경로·대상/인용·종료 후 거부·중복 close | 통과 |
| TestCollectionFactoryFailureClosesOpenedBackend | 부분 backend 실패 자원 정리 | 통과 |
| TestCollectionPublicUnknownAndMissingTargetsAreRejected | 공개 unknown 필드·target 누락 거부 | 통과 |

## 남은 격리·기능 시험

| 검증 대상 | 현재 상태 | 연결 작업 |
|---|---|---|
| 실제 CKG/CKV와 프로젝트별 provider 설정·공개 scope 오류/인용 | 이번 범위 통과 | KM-01.3 완료 |
| 동일 바이트 fork 실제 DB·서로 다른 embedding 구성·cache 반복 | 기반 시험만 통과, 공개 확장 남음 | KM-01.4 |
| 공개 쓰기 도구 전체 표면·다른 project/version DB와 current 보존 | pinned reindex1개 거부 통과, 전체 남음 | KM-01.4 및 KM-02.3/04 |
| 실제 Claude 매니저 host·등록/구축/실패 복구·갱신 | 미구현·미실행 | KM-02/03/04 |
| 산업 개념 매핑·비교 행렬·실제 프로젝트 비교 | 미구현·미실행 | KM-05/06 |
| 자연어·검토된 구현 관계·다중 프로젝트 ontology 효과 | 로컬 단일 프로젝트 관찰만 완료 | OE-01 후속 |

## 전체 검증 gate와 기록

[최신 validation](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/validation.json), [Go 시험 전체3,619개와 race446개 목록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/test-inventory.json), [원시 응답·소스 manifest](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/manifest.json)을 함께 보존한다. 기반40사례·신규14사례는 전체 Go/race 분모에 이미 포함된다.

| 최종 gate | 결과 |
|---|---|
| 전체 모듈 빌드 | 통과 |
| 전체 Go 시험 | 3,600개 통과·19개 명시적 skip·실패0 (총3,619개) |
| 관련4패키지 race | 444개 통과·2개 명시적 skip·실패0 (총446개) |
| 기반 collection race 포함 | 17개 상위 시험/40사례 통과·skip0 |
| 실제 DB·공개 MCP | 24항목·21도구호출·26요청/응답·정상 EOF3회 통과 |
| fmt·전체 lint·vet·engine 경계·문서 | 통과; lint0 issues·문서125개·계약20개·패키지 안내3개 |
| 작업표 허위 완료 방지 | 기존7개·새 공개 증거8개 통과 |
| 기존 소스목록 갱신 | 구현 커밋16a75b8d의1,652개 결속·기존 source/input/evidence guard 통과 |

전체 모듈의19개 skip과 관련 race의2개 skip은 명시적인 기존 환경 조건이며 통과로 세지 않는다. 각 이름·패키지·상태는 전체 시험 목록에 보존했다. package skip12개도 별도 집계한다. 이전 [기반 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-01-foundation/manifest.json)의3,605개/공개 미연결 기록은 당시 범위를 보존하며 최신 상태는 이번 packet을 따른다. 처음 fixture의 model 이름을 잘못 지정해 시작이 거부된 이력도 `initial-failure/`에 보존했다.

실제 DB/manifest18파일과 current 포인터2개가 그대로임을 확인했다. 원문 손상은 소유한 fixture에서 재현하고 정확히 복구했다. 유효한 조회 근거14팩을 검증한 사실과 검색 품질·독립 수용은 분리한다. ontology의384회 관찰과 별도8회 진단에 이번 분모를 합치지 않는다.

다음은 KM-01.4의 최종 격리 반례 검증이다. 이후 KM-02.1의 version-aware 도구 계약으로 이어간다. 신규7개 기능 외의 생성기 보완·운영/독립 수용9개·선택 후속12개는 [전체 작업표](../REMAINING-WORK.md)에 유지한다.

[소스 결속 후속 기록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-source-binding-update/receipt.json)은 구현 커밋16a75b8d의 manifest를 기존 원장에 연결하고 나머지 승인·완료 필드를 그대로 유지한 사실을 기록한다. 공개 조회 packet의 `pending_committed_manifest`는 구현 커밋 당시의 상태이며 현재 완료된 소스 검사는 이 후속 기록을 따른다.
