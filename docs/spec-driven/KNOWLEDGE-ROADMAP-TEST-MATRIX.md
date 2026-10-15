# 지식 매니저·프로젝트 비교 전체 작업·시험표

2026-10-15 현재 단계는 KM-02.2 완료 후 KM-02.3 영속 기록·복구 구현 중이다. KM-01의4개 세부 작업과 KM-02.1/02.2를 완료했으며 전체28개 중6개 완료·4개 진행 중·18개 미착수다. 실제 상태는 [진행 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json), 목적과 전체 완료 기준은 [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), 기존 운영·선택 후속 범위는 [전체 작업표](../REMAINING-WORK.md)를 따른다.

표의 **통과**는 명시한 시험 범위만 뜻한다. **부분 검증**은 완료 표시가 아니며 **미실행**을 통과0건으로 바꾸지 않는다. 신규7개 기능 전체 완료는1/7이다.

## 전체28개 세부 작업과 시험 단계

| ID | 목적·시험 범위 | 구현/작업 단계 | 검증 단계 |
|---|---|---|---|
| KM-01.1 | 신원·key·cache·join·쓰기 조사와 이행 설계 | 완료 | [재사용/결손 표](./PROJECT-COLLECTION-ISOLATION.md) 작성·코드 대조 |
| KM-01.2 | Project/Version/Collection과 고정 scope 계약 | 완료 | catalog·fork·두 버전·직렬화/재시작 시험 통과 |
| KM-01.3 | 구현 대상 제한·명시 비교·잘못된 scope 거부 | 완료 | 실제 CKG/CKV3DB·공개 MCP24항목 통과; 원시 응답/인용 재검증 |
| KM-01.4 | 동명·fork·두 버전·cache·재시작·쓰기/호환 반례 | 완료 | 실제4DB·최종52항목 통과; 동일 원문 fork·모델 분리·쓰기18건 거부·rollback/GC/재시작 보존 |
| KM-02.1 | legacy/versioned capability와 새 요청·상태 계약 | 완료 | [계약](./VERSIONED-MAINTENANCE-CONTRACT.md)·schema·101사례 통과; 새 공개 실행은 .2–04 |
| KM-02.2 | 비동기 build/status/cancel과 정합 gate | 완료 | [공개 실행](./VERSIONED-MAINTENANCE-RUNTIME.md)28항목·실제 retained 조회2건·EOF5개·전체 Go/race·증거 보호10개 통과 |
| KM-02.3 | idempotency/lock/중단/실패/rollback | 진행 중 | private A–C·shared 전환 이력·race141개 통과; 하위 실행 종료 D·공개 영속 연결 E는 미구현·미검증 |
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
| 동일 바이트 fork 실제 DB·서로 다른 embedding 구성·cache 반복 | 실제4DB·프로토콜52항목 통과 | KM-01 완료 |
| 공개 쓰기 도구 전체 표면·다른 project/version DB와 current 보존 | 공개 CKS 변경3도구/18건 거부·보존 통과; 새 version-aware 쓰기 남음 | KM-02.3/04 |
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

KM-01 최종 격리를 완료했다. KM-02.1 계약 설계를 완료했다. 다음은 KM-02.2의 공개 inspect/build/status/cancel·실제 DB gate 연결이다. 신규7개 기능 외의 생성기 보완·운영/독립 수용9개·선택 후속12개는 [전체 작업표](../REMAINING-WORK.md)에 유지한다.

[소스 결속 후속 기록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-source-binding-update/receipt.json)은 구현 커밋16a75b8d의 manifest를 기존 원장에 연결하고 나머지 승인·완료 필드를 그대로 유지한 사실을 기록한다. 공개 조회 packet의 `pending_committed_manifest`는 구현 커밋 당시의 상태이며 현재 완료된 소스 검사는 이 후속 기록을 따른다.

## 최종 KM-01.4 시험 전체 목록

실제 DB4개·공개 MCP52요청/응답·46도구호출·정상 종료3회다. 도구46회 중20회 성공(상태3회+조회17회),26회는 의도한 오류이며 성공 근거20팩을 재검증했다. 다음52항목은 호출/시작/보존 검사를 묶은 분모이므로 다른 시험 수에 더하지 않는다. [원시 증거](../../system/eval/knowledge-roadmap-2026-10-15/km-01-final-isolation/manifest.json)와 [전체 Go/race 사례 목록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-final-isolation/test-inventory.json)을 보존했다.

| 순서 | 검증 ID | 결과 |
|---|---|---|
| 1 | identical-fork-commit-and-all-retained-file-bytes | 통과 |
| 2 | startup-rejected:duplicate-project | 통과 |
| 3 | startup-rejected:duplicate-version | 통과 |
| 4 | startup-rejected:shared-version-path | 통과 |
| 5 | startup-rejected:fork-cycle | 통과 |
| 6 | startup-rejected:wrong-member-model | 통과 |
| 7 | startup-rejected:wrong-member-dimension | 통과 |
| 8 | warm-mixed-read:0:a1 | 통과 |
| 9 | warm-mixed-read:1:b1 | 통과 |
| 10 | warm-mixed-read:2:b2 | 통과 |
| 11 | warm-mixed-read:3:a2 | 통과 |
| 12 | warm-mixed-read:4:b1 | 통과 |
| 13 | warm-mixed-read:5:a1 | 통과 |
| 14 | warm-mixed-read:6:b2 | 통과 |
| 15 | warm-mixed-read:7:a2 | 통과 |
| 16 | identical-forks-and-embedding-versions-not-deduplicated | 통과 |
| 17 | request-rejected:wrong-subcollection | 통과 |
| 18 | request-rejected:other-project-same-commit | 통과 |
| 19 | request-rejected:duplicate-participant | 통과 |
| 20 | runtime-model-digest-drift-refused | 통과 |
| 21 | other-project-works-during-model-drift | 통과 |
| 22 | same-project-other-model-works-during-model-drift | 통과 |
| 23 | restored-model-stays-refused-until-reader-restart | 통과 |
| 24 | writer-rejected:index-full | 통과 |
| 25 | writer-rejected:index-incremental | 통과 |
| 26 | writer-rejected:setup:project-a:root | 통과 |
| 27 | writer-rejected:reindex:project-a:root | 통과 |
| 28 | writer-rejected:setup:project-a:version | 통과 |
| 29 | writer-rejected:reindex:project-a:version | 통과 |
| 30 | writer-rejected:setup:project-a:current | 통과 |
| 31 | writer-rejected:reindex:project-a:current | 통과 |
| 32 | writer-rejected:setup:project-a:new-child | 통과 |
| 33 | writer-rejected:reindex:project-a:new-child | 통과 |
| 34 | writer-rejected:setup:project-b:root | 통과 |
| 35 | writer-rejected:reindex:project-b:root | 통과 |
| 36 | writer-rejected:setup:project-b:version | 통과 |
| 37 | writer-rejected:reindex:project-b:version | 통과 |
| 38 | writer-rejected:setup:project-b:current | 통과 |
| 39 | writer-rejected:reindex:project-b:current | 통과 |
| 40 | writer-rejected:setup:project-b:new-child | 통과 |
| 41 | writer-rejected:reindex:project-b:new-child | 통과 |
| 42 | no-async-job-created | 통과 |
| 43 | GC-protects-all-pins:project-a | 통과 |
| 44 | GC-protects-all-pins:project-b | 통과 |
| 45 | active-new-pin-ignores-rollback-to-old | 통과 |
| 46 | active-old-pin-after-rollback | 통과 |
| 47 | restart-reordered-catalog-keeps-new-pin-while-current-old | 통과 |
| 48 | restart-identical-fork-keeps-model96 | 통과 |
| 49 | restart-same-source-keeps-model128 | 통과 |
| 50 | membership-change-invalidates-old-catalog-pin | 통과 |
| 51 | removed-member-refused-after-restart | 통과 |
| 52 | remaining-member-works-after-membership-change | 통과 |

실패한 첫 실행은 모델 복구 직후 자동 회복을 기대했다. 기존 CKV는 실패 상태를 reader 재시작까지 유지하므로 마지막 기대가 실패했고, 이후 시험은 지속 거부·다른 member 정상·재시작 후 회복을 검증했다. 원래 실패 로그·응답·당시 runner를 `initial-run/`에 보존한다.

| 최종 단계 gate | 결과 |
|---|---|
| 실제 Git 동일 원문 fork·4DB·모델64/96/128 | 통과; 로컬 mock/시험 응답기 프로토콜 |
| 공개 CKS 쓰기 표면 | index2모드 + setup/reindex의2프로젝트×4대상 =18건 거부·job 미생성·전체 tree 불변 |
| rollback·GC·재시작·멤버 변경 | 통과;4개 reader pin 보호, current 복원·B current 보존 |
| 원시 MCP/HTTP/Git/인용·본문 재검증 |52요청/응답·46도구호출·근거20팩 확인 |
| Go 전체·race·build/fmt/lint | 소스1,652개 동일 바이트 확인 후 이전 통과 증거 재사용; 이번 Go 변경0 |
| 문서·최신 작업표·증거 변조 방지 | 문서125개·계약20개·안내3개·현재 작업표7개·변조/누락 방지9개 통과 |

이번 단계의 새 실행과 이전 Go 시험을 다시 실행한 것처럼 합산하지 않는다. Go는3,600통과/19skip, 관련 race는444통과/2skip의 원자료와 모든 사례 이름을 그대로 보존했다. 기존 기반 시점의7개 작업표 시험은 동결 원자료다. KM-01 전체 완료 후에도 미완료 기능을 대상으로 허위 완료를 검사하도록 `scripts/test_knowledge_roadmap_current.py`를 현재 회귀 진입점으로 추가했다. 원래 시험/packet을 수정하지 않고7개 검사를 이어받으며 부분 기능 대상 선택만 현재 미완료 항목으로 바꾼다.

남은 기능은 KM-02~06 및 OE-01이다. 산업 개념별 비교·실제 모델 검색 품질·독립 수용이나 새 version-aware 쓰기는 KM-01 완료에 포함하지 않는다.

KM-01 완료 당시의 진입점은 `python3 scripts/check-collection-isolation.py`, `python3 scripts/test_collection_isolation.py`였다. 이후 진행 수가 증가한 현재는 `python3 scripts/verify_committed_collection.py`로 당시 source/문서 snapshot의 증거를 검증하고 현재 작업표는 `python3 scripts/test_knowledge_roadmap_current.py`로 별도 검사한다. 새 검사9개는 정상 packet, 응답 누락, 동일 원문 fork의 재서명된 다른 프로젝트 인용, 다른 embedding 차원, 거부된 쓰기의 파일 변경, 종료 오류, Go skip의 통과 승격, 독립 수용 과장, 다른 모델 dispatch 기록을 검사한다. 종료 후 독립 재캡처한24파일/current2개의 값도 실행 전과 일치한다.

## KM-02.1 설계 적합성 전체 시험

[strict schema·전이 계약](./VERSIONED-MAINTENANCE-CONTRACT.md)과 [원시 결과](../../system/eval/knowledge-roadmap-2026-10-15/km-02-contract/contract-results.json)의101개 사례다. 각 사례의 ID·기대값·실측값·통과 여부를 보존하며 새 runtime job을 실행한 것으로 세지 않는다.

| 분류 | 사례 수 | 확인 범위 |
|---|---|---|
| request | 30 | profile/project/source/input/model/base/version·정책·경로/명령 주입 거부 |
| status | 28 | 9개 상태·후보/gate/publication·외부 좌표·조회 반영 과장 거부 |
| transition | 12 | 정상 전이·terminal 재시작·취소 후 성공 거부 |
| cancel | 9 | 접수/중복/종료/복구/commit 경계의 취소 판정 |
| idempotency | 3 | 새 요청·동일 요청 replay·같은 key의 다른 fingerprint 거부 |
| reference | 4 | project/profile/job 소유 범위 |
| decode | 6 | 정상 JSON·중복/후행/비유한 수/소수 표기/용량 거부 |
| fingerprint | 1 | key 순서와 무관한 동일 요청 digest |
| observation | 8 | status revision 역행·동일 revision의 다른 본문·다른 job·terminal 변경 거부 |

101개 모두 통과했다. 현재 MCP35도구를 initialize/tools-list2요청으로 새로 확인했고 새 versioned build/cancel은 미노출·정상 EOF0이다. CLI의 version/project/source-mode/hold/test-gate와 rollback help도 확인했다. 최초 help probe는 stdout에만 상세 flag가 있다고 기대해 실패했다. 실제 flags는 stderr에 있으며 두 stream을 보존하고 결합해 검증하는 최종 probe가 통과했다. 제품 코드는 바꾸지 않았다.

KM-01의 최종 packet은 당시 커밋cb373e94의 소스·문서 snapshot에서 검증한다. 현재 완료 수6을 과거 완료 수4에 억지로 맞추거나 과거 packet을 다시 쓰지 않는다. 현재 작업표28개 일치 검사는 별도로 실행한다. KM-02.1 당시에는1,652개 소스·의존 입력의 바이트가 같아 이전 Go 증거를 재사용했다. 이 수에는 Go1,223개와 기타 입력429개가 포함되며, 이전 표기의 “Go1,652개”를 바로잡는다. KM-02.2는 Go 소스가 변경되어 전체 시험을 새로 실행했다. 새로운101개 설계 사례를 Go 시험 분모에 더하지 않는다.

| 이번 단계 gate | 결과 |
|---|---|
| 설계 schema/상태/취소/오류 적합성 | 101/101 통과 |
| 실제 현재 capability 읽기 전용 probe | MCP35도구·2요청·정상 EOF0, CLI help 통과 |
| 이전 KM-01 증거·Go 바이트 결속 | commit snapshot 검증·Go1,652개와 의존 파일 동일 |
| 현재 작업표·문서·증거 허위 완료 방지 | 현재 작업표7개·문서126개/계약20개/안내3개·새 증거 검사9개 통과 |
| 새 공개 비동기 구축·취소·durable job | 미구현·미실행; KM-02.2–04 |

KM-02.1 이후 공개 연결을 KM-02.2에서 구현·검증했다. 현재 다음 작업은 KM-02.3의 중복·재시작·publication 복구이고, 이후 .4의 공개 실패 경계를 검증한다.

KM-02.1 당시 설계 적합성·증거 검사는 `python3 scripts/check-versioned-maintenance-contract.py`, `python3 scripts/check-maintenance-design-evidence.py`, `python3 scripts/test_maintenance_design_evidence.py`로 실행한다. 증거9개 시험은 정상 packet·사례 누락·거짓 성공·설계의 공개 runtime 승격·변경 도구를 실행한 probe·지원 CLI flag 누락·과거 소스 결속의 위조·실패 gate·source audit 변조를 검사한다. 과거 runtime snapshot과 현재 작업표·설계 적합성을 분리하여 완료 수 증가만으로 과거 증거를 다시 만들지 않는다.


## KM-02.2 공개 구축·조회·취소 완료

[실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)와 [원자료 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-maintenance/manifest.json)을 따른다. 기존 과거 packet은 변경하지 않는다. 설계 증거는 `python3 scripts/verify_committed_maintenance_design.py`로7034ce85 snapshot에서 검사하며, 현재 runtime은 새 Go 시험으로 검증한다.

| 검증 | 결과 |
|---|---|
| 공개 MCP lifecycle 고정 항목 | 28/28 통과; 두 프로젝트 실제 CKG/CKV·deterministic mock embedding |
| 실제 dataset 도구·wire | 90회 호출·전체99쌍; polling 수는 시험 항목 수와 별도 |
| 최초/두 번째 실제 publication | A v1·v3 성공, 정확한 receipt·base·좌표 확인 |
| 검토 보류·취소·격리 | A v2/B v1 held, B v2 실행 중 취소, A v1 불변·B current 없음 |
| 실제 조회 | A 이전 v1 marker101·새 v3 marker404 retained 본문/인용 확인 |
| 연결 종료 | 기본·maintenance·재연결·reader2개 총5개 정상 EOF |
| 전체 Go | 3,665사례=3,646통과+19skip; 실패0, 전체135package 정상 종료 |
| 관련5패키지 race | 634사례=632통과+2skip; 실패0 |
| build/fmt/vet/lint/경계/문서 | 통과; 문서 계약20개·패키지 안내3개 |
| 증거 변조·허위 완료 방지 | 10개 통과; wire 누락·거짓 held/canceled 성공·종료 오류·durability 과장·skip 승격·타 프로젝트 인용·DB 변경·lint 실패 |
| 현재 소스 결속 | 기존 입력+새 Go8개=1,660개; 유지 Go1,231개+기타429개, 승인 상태 변경 없음 |
| 영속 job·재시작 복구 | 미구현·미실행, KM-02.3 |
| 전체 KM-02·독립 수용·검색 품질 | 완료 아님 |

현재 완료6/28·진행4/28(KM-02.3, OE-01.1–03)·미착수18/28다. 다음은 KM-02.3, 이후 .4 → KM-03 → KM-04 → KM-05 → KM-06 → OE-01 확대/다중 프로젝트 평가 순서다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 범위다.


## KM-02.3 중간 단계 A: 기록 기반만 검증

| 검증 | 결과·한계 |
|---|---|
| Go 상태/전이/관찰의 설계 대조 | 28/12/8사례와 parent 시험 통과 |
| maintenance package race | 전체114사례 통과·실패/skip0 |
| journal 기록 | scope/key·단독 writer·재열기·terminal 불변·원자적 교체·손상/거짓 성공·symlink/hardlink·128한도 검사 |
| 별도 시험 프로세스 종료 | 기록 rename 전/후 즉시 종료와 재열기 통과; 실제 DB 작업 복구는 아직 아님 |
| 전체 Go | 3,734사례=3,715통과+19skip·실패0 |
| 관련5패키지 race | 703사례=701통과+2skip; maintenance114개 포함 |
| 중간 증거 보호 | 13개 통과; private 기반을 공개 영속/복구 완료로 표시하는 오류 거부 |
| 공개 기존 동작 회귀 | 다시28항목·5개 정상 EOF; 영속 capability는 계속 process_lifetime |
| 완료 집계 | 6/28 유지; KM-02.3은 진행 중 |

다음은 [실행 명세의 B–E](./VERSIONED-MAINTENANCE-RUNTIME.md)에 따라 공개 service의 durable 접수·replay, publication 전후 증거, 하위 프로세스 소유/중단, 실제 MCP 복구와 rollback/GC/read lease를 연결·검증한다. 이 단계의 기록 저장기 시험을 공개 복구나 독립 수용으로 올리지 않는다.

A 단계의 [검증 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-journal-foundation/manifest.json)에 새 전체 시험 목록·원시 wire를 보존한다. 공개 동작 재실행은112개 도구 호출·121개 전체 wire쌍이며 polling 횟수는 고정28개 항목과 별도다. 현재 소스 결속은1,664입력(유지 Go1,235개·기타429개)이다.


## KM-02.3 중간 단계 B: 비공개 service 연결

| 검증 | 결과·한계 |
|---|---|
| maintenance package race | 123사례 통과; 기존 A114개 포함 |
| 기존 전환 경로 포함 관련5패키지 race | 714사례=712통과+2skip·실패0 |
| 전체 Go | 3,745사례=3,726통과+19skip·실패0 |
| 비공개 durable service | 기록한 종료 상태 재시작/replay, changed pin 재조회, key 충돌·타 프로젝트 거부, coordinator owner, 취소 전 기록·불확실한 snapshot 거부 |
| 실제 orchestration의 고의 graph 실패 | 실패 상태 보존·current 미발행·재시작 같은 결과; 성공 DB 검증을 대신하지 않음 |
| 최초 접수 rename 전/후 기록 오류 | 접수 성공 미응답·graph 미실행, 재시작에서 온전한 없음/queued 기록만 복원 |
| 공개 기존 동작 | 28항목·EOF5개 재검증; 공개 persistence는 여전히 process_lifetime |
| 공개 재시작·publication 장애·하위 작업 정리 | C–E 미구현·미실행; KM-02.3 완료 아님 |

[실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)의 B 범위와 [B packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-service/manifest.json)을 따른다. 다음은 취소 의도 결속과 publication 전후 이력 정합(C), 하위 프로세스 소유/중단(D), 실제 공개 복구·read lease/rollback/GC 검증(E)이다. 현재 전체6개 완료·4개 진행 중·18개 미착수다. 이후 KM-02.4 → KM-03 → KM-04 → KM-05 → KM-06 → OE-01 확대/다중 프로젝트 평가를 진행한다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다.

B 증거 보호13개도 통과했다. 실제 공개 회귀는81개 도구 호출·전체90개 wire쌍이며 고정28개 항목과 별도로 센다. 현재 소스 결속은1,668입력(유지 Go1,239개·기타429개)이며 기존 승인/완료 상태는 바꾸지 않는다.


## KM-02.3 중간 단계 C: shared 전환 이력과 비공개 복구

| 검증 | 결과·한계 |
|---|---|
| 실제 pointer/강제 종료/후속 writer | setup19사례 통과; 5개 지점의 실제 프로세스 kill·일반 전환·rollback 후 기존 이력 보존·손상 거부 |
| 비공개 복구·기존 maintenance | maintenance race141사례 통과; job/source/model/base 결속·취소 의도·여러 재시작·충돌 보류 |
| 실제 보관 held DB | 기존 source-bound SQLite·원문·hold marker 검사 뒤 held 복구; current 미발행 |
| 합성 history 정책 시험 | 별도 unit scope; 실제 DB 게시로 집계하지 않음 |
| 전체 Go | 3,782사례=3,763통과+19skip·실패0 |
| 관련5패키지 race | 751사례=749통과+2skip·실패0; 위141개와 setup19개 포함 |
| 공개 기존 동작 회귀 | 28항목·97도구호출·106wire쌍·5개 정상 EOF; shared history와 실제 receipt도 대조 |
| 증거 보호 | 14개 통과; 외부 project로 재서명한 실제 history·합성 승격·D/E 완료 과장 거부 |
| build/fmt/vet/lint/경계/문서 | 통과; 문서 계약20개·안내3개·live 문서127개 |
| 공개 durable·pre-gate child 종료 | D/E 미구현·미검증; KM-02.3 완료 아님 |

[C packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-publication-recovery/manifest.json)에 전체 시험 목록·원시 protocol·실제 pointer history를 보존한다. 현재 소스 결속1,674입력은 유지 Go1,244개·기타430개이며, 실제 held DB 검사에 사용한 기존 immutable fixture도 포함한다. 기존5개 approved input과 승인/완료 상태는 바꾸지 않는다.

현재 전체6개 완료·4개 진행 중·18개 미착수다. 다음은 D의 부모 생존/deadline 감독·하위 writer lease·안전한 staging 정리, 이후 E의 실제 공개 재시작·lease/rollback/GC 검증이다. 그 뒤 KM-02.4 → KM-03.1–4 → KM-04.1–4 → KM-05.1–4 → KM-06.1–4 → OE-01.1–4를 진행한다. 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.3 중간 단계 D1: 실행 감독·소유 lease

| 검증 | 결과·한계 |
|---|---|
| 감독 경로7개 | argv/작업 위치·출력, 실제 부모 kill, 취소·늦은 쓰기, 독립 절대 deadline, 상속 lease, freshness context 통과 |
| race로 발견한 descriptor 경쟁 | 수정 후 같은7개 race 통과; Start/Wait가 파일 수명을 소유 |
| 전체 Go | 3,789사례=3,770통과+19skip·실패0 |
| 관련6패키지 race | 763사례=761통과+2skip·실패0; CLI setup도 포함 |
| 실제 공개 회귀 | CKS 내부 감독 명령 사용28항목·90도구호출·99wire쌍·정상 EOF5개 |
| build/fmt/vet/lint/문서·증거 | 통과; 계약20개·안내3개·live127개·증거 보호13개 |
| owned staging 정리·전체 D/E | 미구현·미검증; 전체 완료 수6/28 유지 |

[D1 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-command-supervision/manifest.json)과 [실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)를 따른다. 현재 소스1,679입력은 유지 Go1,249개·기타430개다. 다음은 D2의 owner token/프로젝트/후보와 결속된 staging 정리와 quiescence 판정 연결이다. 이후 E 실제 공개 재시작·lease/rollback/GC → KM-02.4 → KM-03.1–4 → KM-04.1–4 → KM-05.1–4 → KM-06.1–4 → OE-01.1–4를 진행한다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도 유지한다.
