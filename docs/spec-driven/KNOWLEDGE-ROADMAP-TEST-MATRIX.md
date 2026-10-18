# 지식 매니저·프로젝트 비교 전체 작업·시험표

## 최신 비운영 pilot 진행표 — 2026-10-18

단계43: 독립 수용 **준비 작업 FP01~04 완료**, 정식 수용 **0/9**. [준비·검증 보고서](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/REPORT.txt), [전체 작업리스트](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/WORKLIST.json), [검증 기록](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/validation-summary.json)이 최신이다. 아래 pilot4/4와 로컬 로드맵28/28을 독립 정답·동결·수용 완료 수와 합산하지 않는다.

| 단계 | 검증 범위 | 상태 |
|---|---|---|
| SN-P01 | 실제 go-stablenet 소스·데이터·모델 식별·검색 경로 | 기술 범위 완료; 최초 원문 누락은 SN-P02 보완으로 연결 |
| SN-P02 | 외부 원문 보관·convention 구분·기존/새 버전 비교 | 완료 |
| SN-P03 | 출처/무결성과 모델 실패 분류·재시작 없는 복구 | 완료 |
| SN-P04 | 실제 go-wemix BGE 데이터·개념/검색·출처·혼입 비교 | 완료; 새 race232pass·5정상EOF0·2기대기동거부 |
| FP01~04 | 후보206행 노출/작성 인덱스·정답/매칭 양식·충분성/F01~08·온톨로지 설계 | 준비 작업 완료; 새 FINAL/승인 gold0 |
| 정식 준비 | 담당자·새 DEV/FINAL·완전 gold·matching/충분성·F01~08 동결 | 미완료; D04미정·freeze null |
| 정식 수용9개 | WI-1/WI-2/WI-3/WI-4/WI-V1/WI-V2/WI-6/WI-SP/WI-C | 0/9; 실행 미허용 |
| 필수 온톨로지 독립 효과 | 독립 의미/검색 gold·검토 연결·coverage/효과 gate | 미수용; 기존 제한 연구완료/inconclusive 유지 |
| 선택 후속 | 별도 후속12개 | 별도 backlog |

[SN-P04 최신 검증](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/REPORT.txt), [남은 전체 작업](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/WORKLIST.json). 아래28/28 표는 기존 로컬 로드맵의 완료 기록이며 정식 수용 완료 수와 합산하지 않는다.

2026-10-16 현재 KM-01–06을 완료했다. 전체28개 중28개 완료·0개 진행 중·0개 미착수다. 로컬 구현·개발 효과 관찰 범위를 완료했으며 독립 품질 수용과 구분한다. [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)과 [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)를 따른다.

표의 **통과**는 명시한 시험 범위만 뜻한다. **부분 검증**은 완료 표시가 아니며 **미실행**을 통과0건으로 바꾸지 않는다. 신규7개 기능 전체 완료는7/7이다.

## 전체28개 세부 작업과 시험 단계

| ID | 목적·시험 범위 | 구현/작업 단계 | 검증 단계 |
|---|---|---|---|
| KM-01.1 | 신원·key·cache·join·쓰기 조사와 이행 설계 | 완료 | [재사용/결손 표](./PROJECT-COLLECTION-ISOLATION.md) 작성·코드 대조 |
| KM-01.2 | Project/Version/Collection과 고정 scope 계약 | 완료 | catalog·fork·두 버전·직렬화/재시작 시험 통과 |
| KM-01.3 | 구현 대상 제한·명시 비교·잘못된 scope 거부 | 완료 | 실제 CKG/CKV3DB·공개 MCP24항목 통과; 원시 응답/인용 재검증 |
| KM-01.4 | 동명·fork·두 버전·cache·재시작·쓰기/호환 반례 | 완료 | 실제4DB·최종52항목 통과; 동일 원문 fork·모델 분리·쓰기18건 거부·rollback/GC/재시작 보존 |
| KM-02.1 | legacy/versioned capability와 새 요청·상태 계약 | 완료 | [계약](./VERSIONED-MAINTENANCE-CONTRACT.md)·schema·101사례 통과; 새 공개 실행은 .2–04 |
| KM-02.2 | 비동기 build/status/cancel과 정합 gate | 완료 | [공개 실행](./VERSIONED-MAINTENANCE-RUNTIME.md)28항목·실제 retained 조회2건·EOF5개·전체 Go/race·증거 보호10개 통과 |
| KM-02.3 | idempotency/lock/중단/실패/rollback | 완료 | A–D2·실제 E1 host43/E2 retention21·전체 Go/race 검증; .4 별도 |
| KM-02.4 | 공개 도구의 잘못된 대상·부분 실패·취소 보호 | 완료 | 실제 공개36항목·부분 DB·기존 current/reader·마지막 v4 정상 게시/조회 검증 |
| KM-03.1 | 매니저 등록·설치·초기 구축·복구 절차 | 완료 | 등록14·native3·실제 metadata bootstrap9·증거10; 초안/검토/reader 미구현 |
| KM-03.2 | 문서 없는 프로젝트의 근거 기반 지식 초안 | 완료 | 수집23·등록14·실제 Git/CKS native 초안24·증거13; 사람 검토/DB 조회 아님 |
| KM-03.3 | 암묵지 질문·불확실성·검토/채택/회수 | 완료 | 검토35·수집23·등록14·실제 CLI/native24·증거14; 통제 응답, DB 조회/신원 인증 아님 |
| KM-03.4 | 실제 Claude host 전체 구축·조회와 실패 복구 | 완료 | 실제 CLI8회·도구68개, manager103·native3·증거16, 정상 종료0·중단/복구 |
| KM-04.1 | 수동·정기·변경 trigger와 성공 기준 | 완료 | 정책/시각/성공27개·증거10개; 실제 scheduler 미실행 |
| KM-04.2 | 변경/삭제/no-op/stale와 재검토 | 완료 | 입력32·정책27·manager103·실제4DB/16항목·82호출/11EOF0·증거14; 수동 |
| KM-04.3 | 중복·lease·재시작·retry·자원 상한 | 완료 | A/B1/B2/C1/C2 통합; Python268·공개MCP20/222호출·실제Claude2회/36도구·EOF/kill/exact 복구; .4 별도 |
| KM-04.4 | 두 번 변경+no-op+실패 복구·실제 주기 실행 | 완료 | 실제2프로젝트·20항목·221호출·2EOF0·문서/자료 삭제·메모 교체·graph 실패/연결 retry·타 DB hash/인용 불변 |
| KM-05.1 | 산업 개념·정의·별칭·비교 축 | 완료 | 선택형 팩8개념/26축·한영/모호성·버전 pin29사례·새 전체Go/race7패키지 검증 |
| KM-05.2 | 공통 개념과 프로젝트별 코드/정책/시험 연결 | 완료 | 비교59개(어휘29+연결30)·실제 보관 원문3프로젝트/5종류·committed/working-tree·전체Go/race7 검증 |
| KM-05.3 | exact/close/broader 등의 검토·버전 의미 | 완료 | 정책34개·비교93개·정족수/회수/상충/미래·source/pack 변경 stale·과거 버전·키 별칭 거부 |
| KM-05.4 | 비적용/미수집/미지원·L1/L2·network 차이 | 완료 | 적용/지원24개·비교117개·Rust text/AST·L1/L2·정책 차이/내부상충·선언/미결정·schema1/2·손상 |
| KM-06.1 | 명시 버전·개념·축·시점·상한의 비교 요청 | 완료 | 계약50개·비교167개·pin/scope/권한/시각/예산·요청 경로/target 거부; 실제 MCP/조회0 |
| KM-06.2 | 프로젝트별 근거 행렬/bundle | 완료 | 실제 mock2DB·12항목/9도구·2EOF0·프로젝트별 원문/인용·unknown·후보 기본 제외·잘못된 요청/예산 거부 |
| KM-06.3 | 상충·누락·자료 부족·부분 실패 상태 | 완료 | 새 보호21개·비교192개·공개 기반12+보호9항목/18호출·3EOF0/SIGTERM0·다른 프로젝트 보존·복구 hash 동일 |
| KM-06.4 | 둘 이상 실제 프로젝트의 비교·오인·근거 시험 | 완료 | 실제2프로젝트/4243파일·6개념40셀·22항목19도구·3EOF0·모드/범위/해시/오매핑/누락/예산·원본/DB 보존 |
| OE-01.1 | 질문·기대 근거·독립성·분모 고정 | 완료 | 새16질문·14함수 과제/2미채점 제어·5사실군11함수·512회 사전 계획; 자체 source 기준/사람·독립 gold0/새 검색0 |
| OE-01.2 | 같은 자료의 off/concept_text/relations/combined | 완료 | 실제512회·실패/누락0·64capture/4readiness 정상 종료·전체 source/본문/인용/무결성·API 계측·원본 보존; 효과 점수 다음 |
| OE-01.3 | 근거·순위·unknown·지연/비용·답변 효과 | 완료 | 고정 함수 과제P/R/MRR·EX08 한정 이득·EX16 범위 진단·warm/API/token 재계산; 일반/답변/독립 품질 미측정, off/inconclusive |
| OE-01.4 | 다중 프로젝트 개념 매핑·오인·비교 효과 | 완료 | 실제2프로젝트·매핑 기준선/처리40셀·18항목27호출·2EOF0; 원문8셀/양측후보3개·unknown 유지·품질 inconclusive |

아래 단계별 기록은 당시 checkpoint의 범위다. 현재 전체 상태는 위 표와 원장을 따른다.

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
| 24 | 기본36도구·기존 단일 프로젝트 V2 호환 | 통과 |

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
| 실제 Claude 매니저 host·등록/구축/실패 복구·갱신 | KM-03 실제 로컬 연결 완료; 정기 갱신 미실행 | KM-04 |
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


## KM-02.3 중간 단계 D2: 소유 staging 정리

| 시험 범위 | 결과 | 완료 판정에 쓰는 의미 |
|---|---|---|
| 전체 Go | 3,811사례: 3,792통과·19skip | 현재 소스 회귀, skip은 통과로 계산하지 않음 |
| 관련6패키지 race | 785사례: 783통과·2skip | setup/maintenance/MCP/config/CLI 경합 |
| maintenance | 144사례 통과 | 이전141 + v1/v2 writer 해제·정리 경계3 |
| 실제 프로세스 kill | record/디렉터리/owner/부분 자료4경계 통과 | 후보 안의 exact owner scratch만 정리 |
| 잘못된 소유/링크/경로 | 10변형 거부·witness 보존 | 다른 자료의 자동 삭제 방지 |
| 실제 CKS/CKG/CKV·MCP | 고정28항목·5연결 EOF0 | 공개 process-lifetime 회귀, durable 완료 증거 아님 |
| build/fmt/vet/lint·문서 | 통과 | 현재 코드/명령 계약 |

[D2 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-owned-staging/manifest.json)과 [실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)를 따른다. 현재 소스1,682입력은 유지 Go1,252개·기타430개이며 원래 승인5개 입력은 바꾸지 않는다. D1은1283a22c 당시 snapshot에서 검증하고 현재 Go 결과를 재사용하지 않는다. 다음은 E 실제 공개 durable 재시작/replay·취소/중단·rollback/read lease/GC이며 KM-02.3은 진행 중, 완료 수6/28이다. 이후 KM-02.4, KM-03.1–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4를 진행한다. 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.3 중간 E1: 공개 durable host 연결

| 검증 | 결과 | 경계 |
|---|---|---|
| 실제 CKS·CKG·CKV/MCP | 43고정 항목 통과 | mock embedding, 품질/독립 수용 아님 |
| 실제 graph 중 부모 SIGKILL | 2회 -9 | interrupted와 accepted cancel의 canceled를 구분 |
| 나머지 연결 EOF | 7회0 | 활성 하위 작업의 취소 의도를 먼저 저장·정리 대기 |
| 원래 요청 replay/terminal | 재시작 후 불변 | mutable source/base 변경에도 다시 구축하지 않음 |
| 전체 Go | 3,812사례: 3,793통과·19skip | 현재 소스 |
| 관련6패키지 race | 786사례: 784통과·2skip | 정상 종료 수정 회귀 포함 |
| build/fmt/vet/lint·문서 | 통과 | 현재 명령 계약 |

[E1 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/manifest.json)과 실행 명세를 따른다. 소스1,683입력은 유지 Go1,253개·기타430개이며 승인5개 입력은 그대로다. E2 실제 read lease/rollback/GC와 과거 publication 이력 보존은 미실행이며 세부6/28을 유지한다. 다음 E2 → KM-02.4 → KM-03.1–4 → KM-04.1–4 → KM-05.1–4 → KM-06.1–4 → OE-01.1–4이며 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.3 E2 완료: 실제 reader·rollback·GC

| 검증 | 결과 | 확인한 경계 |
|---|---|---|
| E1 실제 fixture 복원 | 입력 hash 결속 | 새 소유 경로, 합성 DB 아님 |
| 실제 MCP/CLI | 21항목 통과 | rollback·v4/v5 게시·reader 원문 보존 |
| 읽기 중 GC | 보호·계획 적용 후 조회 통과 | current/rollback/held/명시 보호 별도 |
| reader 해제·다시 연결 | 회수 가능·stale 계획 거부 | 수동 참조 수정 없음 |
| GC 실제 삭제 | v3 삭제·양수 byte 회수 | v1/v2/v4/v5 보존 |
| 삭제 후 기록·조회 | 과거 receipt/replay 불변·v3 시작 거부·v5 조회 | activation을 현재 serving으로 세지 않음 |
| 프로젝트 보존 | A v1·B held v1·history byte 불변 | 다른 프로젝트 영향 없음 |
| 실제 stdio 종료 | 5연결 EOF0 | 비정상 종료 숨김 없음 |
| 캐시 없는 전체 Go | 3,812사례: 3,794통과·18skip | 135패키지, 실패0 |
| 캐시 없는 관련6패키지 race | 786사례: 784통과·2skip | 실패0 |
| build/fmt/vet/lint·문서 | 통과 | 현재 코드 |

기존에 skip이던 TestDefaultRouteIP가 이번 실제 실행에서는 통과했다. raw Go 결과의 상태를 집계했으며 수동으로 skip을 통과로 바꾸지 않았다.

[E2 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-retention/manifest.json)과 실행 명세를 따른다. E1은0fa42cef snapshot에서 별도 검증했다. 소스1,686입력은 유지 Go1,253개·기타433개이며 추가3입력은 E1 실제 fixture/manifest/원시 관찰이다. 원래 승인5입력은 바꾸지 않는다. 증거 보호12개는 wire·인용·계획·CLI 거부·기록/입력·skip·완료 집계 변조를 거부한다.

KM-02.3 완료로 전체7/28이며 다음은 KM-02.4의 공개 잘못된 대상/경로·graph/vector 부분 실패·의미 입력 불일치·timeout/cancel/shutdown 보호 검증이다. 남은21개는 KM-02.4, KM-03.1–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 별도 정책 생성기 보완·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.4 완료: 공개 실패 전체 matrix

| 범위 | 실제 결과 |
|---|---|
| project/profile/base·version·경로/명령·타 작업 조회/취소 | 공개 거부·current 보존 |
| graph 실패·graph 완료 후 vector 실패 | failed·부분 DB 활성화 없음·scratch 정리 |
| foreign semantic corpus commit/root | 실제 graph 뒤 실패·기존 원문 조회 보존 |
| 구축 중 source/profile 변경 | publication 거부 |
| 실제10초 deadline | failed/deadline_exceeded·writer 종료·scratch 정리 |
| 취소·EOF·SIGKILL | canceled/canceled/interrupted·기존 current·reader·다른 project 보존 |
| 사건별 byte/원문/인용 | 원래 DB/manifest/source/history hash 불변 |
| SQLite reader 공유 파일 | 별도 기록·WAL0byte·reader 종료 후 전체 목록/byte 복원 |
| 마지막 정상 요청 | 실제 v4 게시·기존 v3와 새 v4 reader 좌표/원문 분리 |
| 고정36항목·실제5연결 | 고의 SIGKILL1회 -9·EOF4회0 |
| Go 결과 | E2의1253 Go·모든 결속 입력 동일 후 전체3794통과/18skip·race784통과/2skip 참조 |
| 이번 별도 검사 | build/fmt/vet/lint·문서·증거 보호12개 |

[실패 matrix packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-failures/manifest.json)과 실행 명세를 따른다. 현재 소스1,689입력은 유지 Go1,253개·기타436개이며 추가3입력은 E2 manifest와 실제 module/race raw proof다. 원래 승인5입력은 그대로다. Go를 새로 실행한 것으로 기록하지 않는다. KM-02의4/4를 완료해 전체8/28·기능2/7이다. 남은20개는 KM-03.1–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 다음은 KM-03.1이며 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-03.1 완료: 매니저 패키지·등록·진단

| 범위 | 결과 | 완료로 확대하지 않는 범위 |
|---|---|---|
| 등록 helper | 14개 시험 통과 | DB 구축/원문 문서 생성 아님 |
| project 격리/소유 | UUID·명시 owner·겹침/legacy/변조/링크 거부 | 이름/commit을 project 신원으로 쓰지 않음 |
| 실제 CLI strict validator | manifest/skill/agent3개 통과 | 모델 행동 검증 아님 |
| 실제 metadata bootstrap | 9항목 통과 | 2연결·정상 EOF·4개 도구/37개 schema |
| 소스/DB | 문서 없는 Go source 불변·dataset 미생성 | 초안/검토/reader 미구현 |
| Go proof | E2의1253 Go·모든 입력 동일 후 참조 | 새 Go 실행으로 세지 않음 |
| 이번 별도 gate | build/fmt/lint·문서131개·증거 보호10개 | 독립 수용/품질 아님 |

[bootstrap packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-bootstrap/manifest.json)과 [매니저 계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,697입력은 유지 Go1,253개·기타444개이며 추가8개는 plugin runtime/skill/agent/config/schema다. 승인5개 입력은 그대로다. 전체9/28·기능2/7 완료다. 남은19개는 KM-03.2–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 다음은 KM-03.2다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개도 별도 유지한다.


## KM-03.2 완료: 근거 수집과 proposed 지식 초안

| 검증 | 결과 | 경계 |
|---|---|---|
| 수집 단위 시험 | 23개 통과 | source 변경·비밀/링크·상한·타 프로젝트·변조·실제 Git diff |
| 기존 등록 회귀 | 14개 통과 | config/engine 소유 검증 유지 |
| 실제 두 프로젝트/native CKS | 24항목 통과 | 문서 없이 등록·수집·초안·validate/lock·재실행 |
| 근거와 변화 | origin/project/file/span hash 보존 | 같은 경로·코드라도 타 프로젝트 근거 분리 |
| 제공 자료 | 결정/명령 결과 보고 분리 | actor 없으면 unknown·명령 실행하지 않음 |
| 기본/사용자 주장 초안 | 모두 proposed·사람 승인 없음 | 실제 native questions overlay 구조/lock 검증 |
| source/DB | 원래 overlay/dataset 생성 없음 | adoption/검색/managed reader는 다음 단계 |
| Go proof | E2 입력1253 Go·모든 결속 byte 동일 후 참조 | 새 실행으로 세지 않음 |
| 별도 gate | build/fmt/vet/lint·문서·증거 보호13개 | 품질·독립 수용 아님 |

[수집 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-collection/manifest.json)과 [실행 계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,698입력은 유지 Go1,253개·기타445개이며 원래 승인5개를 보존한다. 이전 bootstrap은2c2b7ed1 snapshot에서 별도 검증했다. 전체10/28·기능2/7이며 남은18개는 KM-03.3–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 다음은 KM-03.3이다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개도 별도로 유지한다.


## KM-03.3 완료: 질문·제공 검토·filtered input 채택/회수

| 검증 | 결과 | 경계 |
|---|---|---|
| 검토 단위 시험 | 35개 통과 | 질문·중복/해석 후보·CAS·동시 writer·제공 actor/이유 |
| 수집/등록 회귀 | 23개/14개 통과 | 기존 collection 신원·원문 byte·owner 보존 |
| 실제 CLI/native CKS | 24항목/25명령 통과 | 등록→수집→초안→통제 검토→채택→회수 |
| 검토 내구성 | 불변 event chain3개·재실행 동일 | 주입 I/O 실패의 게시 전/후 복구, 실제 SIGKILL 아님 |
| 선택 source/Git 변화 | 추가/삭제/byte/commit 변경 거부 | 이전 검토를 새 근거로 승계하지 않음 |
| 채택 입력 | 공개 source만 복사·accepted4/omitted3 | 원래 source·DB·제외 자료 불변 |
| native overlay | 실제 validate/lock·post-rename locked | policy/ADR 임의 생성 없음·검색 완료 아님 |
| 회수 | 이후 사용/이전 검토로 재채택 거부 | 자료 보존·게시된 DB rollback은 별도 |
| 검토 증거 | controlled_fixture 명시 | 외부 신원 인증/실제 사람 독립 수용 아님 |
| Go proof | E2 동일1253 Go·결속 입력 확인 후 참조 | 새 실행으로 세지 않음 |
| 별도 gate | build/fmt/vet/lint·문서·증거 보호14개 | 모델/온톨로지 품질 아님 |

[review packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-review/manifest.json)과 [계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,699입력은 유지 Go1,253개·기타446개이며 원래 승인5개를 보존한다. 이전 collection은322daf39 snapshot에서 별도 검증했다. 전체11/28·기능2/7이며 남은17개는 KM-03.4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 다음은 KM-03.4이며 정책 생성기·운영/독립 수용9개·선택 후속12개도 별도로 유지한다.


## KM-03.4 중간 A: 검토 지식의 실제 색인·retained 조회

| 검증 | 결과 | 경계 |
|---|---|---|
| 새 일반 문서 입력 | 명시 document_corpora/반복 --document-corpus | 의미 승인/기존 semantic export로 오인하지 않음·기본 비어 있음 |
| 신규 Go 회귀 | 26사례 통과 | root 순서·byte/add/delete/order·빈/링크/비 text·상한·취소·누락 색인 |
| 캐시 없는 전체 Go | 3,838사례=3,820통과+18skip·135패키지·실패0 | 이전 Go 결과 재사용 없음 |
| 캐시 없는 관련6패키지 race | 812사례=810통과+2skip·실패0 | 신규26사례는 위 분모에 포함 |
| 실제 manager 입력→공개 build | held 후보·필수 gate 통과·current 없음 | fixture 검토·mock embedding·품질 아님 |
| 실제 DB 자료 | 검토 note의 별도 docs origin/byte·Markdown chunks | private 원문 재유입 없음 |
| 실제 MCP 원문/인용 | 후보 좌표/project 일치·입력 삭제 후 동일 원문 | manager broker/실제 Claude host 미검증 |
| 실제14항목·3연결 | durable replay·EOF0/0/0 | 상태/게시/serving 구분 |
| 별도 gate | build/fmt/vet/lint·문서131개·증거14개 | 독립 수용 아님 |

[A packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-document-index/manifest.json)과 [계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,702입력=유지 Go1,256개+기타446개이며 승인5개는 그대로다. 초기 개발 실패와 최종 통과의 raw 결과를 구분해 보존했다. 이전 review checkpoint는3d39801c snapshot에서 검증했다. A만 완료했으므로 전체11/28·기능2/7을 유지한다. 남은17개는 KM-03.4(B adoption-bound 유지 보수/reader, C 실제 host), KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 정책 생성기·운영/독립 수용9개·선택 후속12개도 별도로 유지한다.


## KM-03.4 중간 B: 실제 매니저 도구·adoption-bound reader

| 검증 | 결과 | 경계 |
|---|---|---|
| broker unit | 29개 통과 | 실제 CKS·scope·source/회수/손상·CAS/lease·응답 손실 주입 |
| 수집/등록 회귀 | 23개/14개 통과 | 기존 owner/해시/typed refusal 유지 |
| 실제 plugin stdio | 26항목·3연결 EOF0 | 18도구; legacy ops/dummy context 미노출 |
| 실제 매니저 도구 연결 | collect/evidence/draft/review/adopt/select/build/status/reader/retract | 통제 응답·mock·DB 게시 없음 |
| 선택·조회 | adoption/source/input/model/job/후보 좌표 일치 | 초기화와 실제 읽기 관찰 분리 |
| 드리프트/회수/재시작 | 새 사용 거부·기존 admitted job만 exact resume | 새 요청으로 바꾸거나 current로 fallback 없음 |
| 열린 reader 손상 | dataset identity 손상 거부 | fixture 복구 후 같은 좌표 확인 |
| native CLI | strict manifest/skill/agent3개 통과 | 실제 Claude host 모델 흐름은 C |
| Go proof | A의1256 Go·의존/fixture byte 동일 후 참조 | 전체3820통과/18skip·race810통과/2skip, 새 실행 아님 |
| 별도 gate | build/fmt/vet/lint·문서131개·증거14개 | 독립 수용/품질 아님 |

[B packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-broker/manifest.json)과 [계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,703입력=Go1,256개+기타447개이며 원래 승인5개는 그대로다. A는bb71aae3 snapshot에서 검증했다. 초기 native SDK `isError:false` 생략에 대한 시험 단언 오류는 initial-failures에 보존했다. B만 완료했으므로 전체11/28·기능2/7을 유지한다. 남은17개는 KM-03.4(C 실제 Claude host), KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.

## KM-03.4 C: 실제 host·종료·복구

KM-03.4 C 완료: 실제 설치 Claude Code2.1.300을8회 실행해68개 도구 결과를 원시 MCP·실제 API 입력과 대조했다. 문서 없는 입력의 수집→제공 검토→filtered adoption→held build→실제 원문/인용과 재시작 조회, 근거 부족·타 프로젝트 거부, 엔진 누락, 진행 중 정상 종료·host 강제 중단 후 durable 취소/exact replay/새 구축·조회를 확인했다. 정상 매니저 종료는 모두0이며 엔진 누락의 시작 거부2회(각2)와 의도한 host SIGKILL1회(-9)는 별도로 보존했다. 실제 SDK SIGINT에서 매니저가-2로 끝나던 문제를 graceful 종료로 수정하고 SIGINT/SIGTERM 회귀를 추가했다. 등록14·수집23·검토35·broker31의103개를 CI manager-test 진입점에 연결했고 로컬 실행을 통과했다. 원격 CI 실행으로 주장하지 않는다. 전체 Go3,838사례(3,820통과·18skip), 관련6패키지 race812(810통과·2skip)를 새로 실행했으며 build/fmt/vet/lint·문서와 native strict3개·증거 보호16개를 통과했다.

| 실제 host 사례 | 결과 | 범위 |
|---|---|---|
| full / restart |18+2개 도구, held 후보 원문·인용 일치 | 게시·모델 품질 아님 |
| insufficient |1개 실제 도구가 근거 부족 거부 | 지식 발명 없음 |
| missing engine |2회 시작 거부·MCP failed | 정상 종료 실패와 분리 |
| active-stop / recovery |12+12개 도구·취소 이력과 exact replay·새 구축/조회 | 실제 정상 host 종료 |
| killed host / recovery |12+11개 도구·매니저/graph 정리·새 구축/조회 | host만 SIGKILL, 매니저0 |

로컬 시험 응답기는 실제 host가 호출할 동작만 제공하고 MCP 결과는 실제 매니저/CKS에서 받았다. 모델 호출·비용은0이며 자율 에이전트 성능·의미 품질·독립 사람 수용을 판정하지 않았다. strict MCP fixture는 plugin 선언의 실제 launcher를 명시 연결했다. 처음의 strict discovery 설정·실제 SIGINT 종료 오류·시험 parser/retry 단언 및 test-only ResourceWarning은 initial-failures에 보존했다. 원본 source와 기존 current는 수정하지 않았다.

[C packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-host/manifest.json)에 새 Go 원시 결과·전체 사례 목록·103개 로그·8회 raw trace를 보존한다. 현재 소스1,703입력=Go1,256+기타447이며 승인5입력은 그대로다. B는c51f7423 snapshot에서 검증했다. 전체12/28·기능3/7 완료다. 남은16개는 KM-04.1–4(정기/변경 갱신), KM-05.1–4(공통 개념 매핑), KM-06.1–4(비교 근거 MCP), OE-01.1–4(온톨로지 효과 평가)다. 다음은 KM-04.1의 trigger·성공 기준·변경 pin 설계다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 이어간다.

## KM-04.1 명시 정책·trigger·성공 계약

KM-04.1 완료: 명시 정책·수동/정기/변경 polling·clock skew/놓친 일정 합치기·등록/프로젝트/명시 corpus 범위·재시도 상한·required gate와 held/게시 성공 계약을27개 반례로 검증했다. 문서 없는 source의 자동 관찰 입력과 사람의 의미 검토를 분리하고 변경된 verified 지식의 무조건 승계를 금지했다. 이 단계는 설계/정책 판정이며 scheduler 실행·자동 DB 갱신 완료가 아니다. build/fmt/vet/lint·문서132개·증거 보호10개를 통과했다. Go/의존/fixture byte가 C와 같아3,820통과·18skip 및 race810통과·2skip의 committed 원자료를 참조했으며 새 Go 실행으로 세지 않는다.

[정책 계약 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-contract/manifest.json)에27개 전체 사례 이름·실행 로그와 실제 host C의 동결 검증을 보존한다. 소스1,704입력=Go1,256+기타448, 원래 승인5개는 보존한다. 전체13/28·기능3/7이다. 다음은 KM-04.2의 변경/삭제/no-op·자동 관찰 입력 구현이다. 남은15개는 KM-04.2–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 계속한다.

## KM-04.2 원문 변경/삭제와 자동 관찰 입력

KM-04.2 완료: 자동 관찰 입력·선택 파일의 추가/수정/삭제·same-byte rename 후보·명시 knowledge/ontology 원문·제공 기록·검토 stale/no-op pin을 구현했다. 사용자 source에 문서를 쓰거나 review/human_response를 만들지 않고 native knowledge init/validate/lock/validate를 수행한다. 정책27·입력32·기존 manager103의162개와 실제 공개 MCP16항목·4개 held DB·82호출·11연결 EOF0·증거 보호14개를 검증했다. 실제 두 번 변경·삭제 후 최신 원문/인용과 이전 snapshot/타 프로젝트 DB 보존, 실제 inspect/status/reader가 있는 무변경 판정을 확인했다. 메타데이터/input equality만으로 DB no-op을 승인하지 않는다. 이 실행은 수동 검증이며 주기 worker 검증이 아니다. build/fmt/vet/lint·문서132개를 통과했고 Go1256개·의존/fixture byte가 C와 같아 원래3,820통과/18skip·race810통과/2skip을 참조했다. 새 Go 실행으로 세지 않는다.

[관찰 입력 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-inputs/manifest.json)에162개 로그·59개 새 계약/입력 사례 전체 이름,4DB의 실제 MCP 원시 입출력과 소유 fixture를 보존했다. 소스1,705입력=Go1,256+기타449이며 승인5개는 보존했다. 기존 검토의 실제 collection_stale에 대한 초기 시험 단언 이름 불일치는 initial-failures에 남겼다. 전체14/28·기능3/7이며 다음은 KM-04.3의 durable intent·한 writer·재시작/retry·실제 worker와 gated 전환이다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도 유지하며 멈추라는 요청 전까지 단계별 서명 커밋으로 계속한다.


## KM-04.3 중간 A: 실제 원문 전환 전 확인

KM-04.3 A 원문 전환 전 확인 완료: operator profile의 선택적 source_check로 실제 Python/코드·소유 정책/등록/불변 입력을 고정하고 admission/후보 완료/게시 pointer lock 안에서 원래 source fingerprint를 재확인한다. 실제 MCP14항목·90호출·3개 job에서 v1 게시→v2 원문 변경으로 게시 거부/current·기존 reader 보존→v3 복구 게시/조회와5연결 EOF0을 확인했다. Go 전체3,862사례(3,844통과·18skip), 관련6패키지 race836(834통과·2skip)을 새로 실행했고 Python174개·증거 보호14개·build/fmt/vet/lint·문서132개를 통과했다. 추가 source-check23사례와 이미 지난 deadline1사례는 전체/race 분모 안에 포함하며 중복 합산하지 않는다.

처음에는 원문 probe가 native 접수 중의 아직 root owner가 없는 소유 journal과 충돌했다. 원문 전용 probe만 읽기 확인과 DB 소유 검증을 분리했고 기본 등록·doctor·prepare·DB 쓰기의 소유 검증은 유지했다. Native 오류의 실제 structuredContent를 읽도록 검증기도 수정했다. 최초 race의 기존 deadline 시험1회 late-writer 실패는5회 반복에서 재현되지 않았다. timer callback 도착만 가정하지 않도록 exec 직전/직후 절대 시각 확인을 보강하고 과거 deadline 거부 시험을 추가했다. 두 경계를 각각5회 확인한 뒤 최종 전체·race를 순차 실행해 실패0을 확인했다. 최초 실패의 단일 원인을 입증했다고 주장하지 않으며 initial-failures의 원자료를 보존한다.

[원문 확인 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-source-check/manifest.json)을 따른다. 현재 소스1,707입력=Go1,258+기타449이며 승인5개는 보존했다. 직전2c03322f의 입력 검증을 동결 snapshot에서 별도 확인했다. A는 native 원문 검사 범위다. KM-04.3의 B durable worker/일정/중복/retry와 C 중단/복구 연결, KM-04.4 실제 주기 실행은 남는다. 전체14/28·기능3/7을 유지한다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 다음 B를 진행한다.

## KM-04.4 최종 시험

KM-04.4 완료: 실제 두 프로젝트에서20항목·MCP221호출·정상 EOF2개를 검증했다. A는 periodic/change로7→8→9→10을 게시하고 문서·명시 ontology 원자료 root 삭제와 제공 메모 교체를 반영했다. 실제 graph 실패1건은 current/마지막 성공을 보존했고 같은 입력의 연결된 attempt2가 복구했다. no-op은 중복 job을 만들지 않았고 v1 코드/삭제된 자료의 당시 인용을 계속 조회했다. 동명 module/Value를 가진 B의 DB 전체 파일 hash·current·실제 본문/인용은 불변이었다. 비공개 자료 제외·자동 의미/사람 검토 미생성도 확인했다. 이는 mock embedding의 실제 lifecycle 검증이며 검색 품질·온톨로지 효과·독립 수용은 별도다.

전체16/28·기능4/7 완료다. 다음은 KM-05.1의 산업 개념·정의·별칭·포함/제외·비교 축 설계다. 남은12개는 KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 계속한다.

## KM-05.1 개념 팩 시험 목록

최종 실행 결과는 단계 완료 시 원자료와 함께 기록한다. 아래29개는 Go의 상위/하위 실행 수이며29개의 독립적인 산업 사실이나 품질 정답 수가 아니다.

| 상위 시험 | 하위 포함 실행 수 | 검증할 경계 |
|---|---|---|
| TestShippedSchemePinsQuestionsAndDefinitions | 1 | 8개념·26축·정의·정확 pack/version/digest |
| TestBilingualAliasesAndAmbiguityAreExplicit | 1 | 한영/대소문자·두 의미 후보·미등록 문장 unknown |
| TestSchemeLookupNeverReturnsMutableOwnedSlices | 1 | 반환값 변경으로 이후 조회 오염 금지 |
| TestSchemeByteChangesAlterExactPackIdentity | 1 | sidecar 변경은 새 identity; 옛 선택 어휘 불변 |
| TestInvalidSchemeBoundaries | 14 | 버전/pack/외부 개념·누락/중복·경계/역할/질문/조건 |
| TestStrictSchemeJSONRejectsAmbiguousDocuments | 6 | 중복 key·unknown key·후행 문서·크기/깊이 |
| TestSchemeDoesNotFollowRootOrDefinitionSymlinks | 1 | root/sidecar 링크 거부 |
| TestSchemeReusesPackDependencyAndTypeResolution | 4 | 미등록 의존·없는 core 참조·상속 순환 거부 |


## KM-05.1 공통 개념 검증

KM-05.1 완료: 기존 knowledgepack loader를 재사용한 선택형 industry.blockchain1.0.0 팩에8개념·26비교 축을 추가했다. 한영 별칭·포함/제외·역할/버전/network/config/시점 조건, 정확 byte pin과 unknown/ambiguous 계약을29개 Go 사례로 검증했다. 단어의 exact_term 일치를 구현 매핑 승인이나 project/entity 동일성으로 해석하지 않는다. 공통 core20·운영 dataset·기본 자동 선택은 변경하지 않았다. 프로젝트 연결·비교 MCP·ontology 품질은 남는다.

전체17/28·기능4/7이다. 다음은 KM-05.2의 프로젝트별 코드·문서·설정/정책·테스트 근거 연결이다. 남은11개는 KM-05.2–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-05.2 프로젝트 연결 시험 목록

공통 어휘29개에 아래30개를 더한 비교 패키지59개다. 최종 전체 실행이 끝나기 전에는 세부 작업 완료로 확정하지 않는다. 원문 보관 fixture를 실제 graph/vector 검색 품질로 확대하지 않는다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestDifferentNamesMapToSeparateProposedSourceGroups | 1 | Stake/Bond·각5종류 원문·프로젝트별 인용·proposed |
| TestBindingSchemaRejectsUnscopedOrInventedClaims | 21 | 20개 schema/scope/hash/facet 거부와 중복·가짜 verified JSON |
| TestForgedHashAndWrongCollectionReturnNoEvidence | 1 | 재서명 hash·미선택 collection·취소·종료 이후 조회 거부 |
| TestMappingAnnotationsAreSanitizedAndDigestTracksInterpretation | 1 | 역할/이유/조건 정화·변경 digest·반환 slice 격리 |
| TestSameLocalNameDoesNotChooseTheSameCommonConcept | 1 | 같은 Stake 이름의 stake commitment/validator identity 역할 분리 |
| TestBindingReadsRetainedSourceAndRejectsArchiveCorruption | 1 | live 변경 이후 당시 원문 유지·보관 blob 손상 거부 |
| TestBindingsKeepCommittedAndWorkingTreeCitationSemantics | 3 | committed/미커밋 원문의 commit/base 구분 |
| TestBindingIDsAreScopedByProjectVersionAndConditions | 1 | 같은 로컬ID·다른 프로젝트/network·동일 시각 표현 중복 |


## KM-05.2 완료 기록

KM-05.2 완료: 명시 common concept/version/digest와 프로젝트·버전·network/config/역할/관찰 시점에 결속한 원문 연결을 구현했다. 서로 다른 이름의 같은 역할과 같은 이름의 다른 역할을3개 프로젝트의5종류 원문으로 검증하고, 동일 로컬ID를 서로 다른 scope에 보존했다. 기존 collection reader lease·target 검증과 retained evidence 정화/재검증을 재사용한다. code/document/configuration/policy/test와 원문 hash/span을 구분하며 모든 연결은 proposed다. 실제 사람 승인·그래프/벡터 검색 품질·공개 비교 MCP는 완료로 세지 않는다.

전체18/28·기능4/7이다. 다음은 KM-05.3의 직접 매핑 의미·검토/회수·버전/stale 정책이다. 남은10개는 KM-05.3–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-05.3 직접 매핑·검토 시험 목록

기존59개에 아래34개를 더한 비교 패키지93개다. 통제된 검토 기록으로 정책을 검증하며 실제 사람 승인·독립 품질로 세지 않는다. 최종 전체 회귀 후 단계 완료를 확정한다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestDirectMappingMeaningNeverMergesEntitiesOrInfersClosure | 6 | 5종류 직접 방향·broader/narrower 범위·entity/전이 금지 |
| TestMappingReviewQuorumWithdrawalConflictAndTime | 15 | 14개 승인/중복/정족수/철회/상충/최신/미래 판정 |
| TestMappingReviewsAreExactPinsAndProjectLocal | 1 | 해석 변경 stale·타 project/network 기록 미승계·위조 header 거부 |
| TestReviewedBindingChecksCurrentArchiveAndKeepsHistoricalVersion | 1 | 새 버전 stale·당시 retained 승인/원문 유지·손상은 승인으로 우회 금지 |
| TestMappingReviewDoesNotMigrateAcrossPackVersions | 1 | 공통 팩v2의 새 검토 요구·옛v1 선택/검토 보존 |
| TestMappingReviewStrictSchemaAndNoMissingActor | 9 | 검토자/이유/판정/시각/target/source/namespace8개·변조 이벤트/정족수 거부 |
| TestMappingReviewRejectsCaseAliases | 1 | 대소문자·유니코드 속성 별칭 덮기 거부 |


## KM-05.3 완료 기록

KM-05.3 완료: exact/close/broader/narrower/related를 local→shared 직접 관계로 고정하고 entity 동일시/자동 전이를 금지했다. 정확 source/해석/팩 pin의 검토 기록, 중복 검토자 정족수·최신 판정·회수·충돌·미래 시각·stale와 과거 버전 보존을 구현했다. 공개용 reader는 보관 원문을 재확인하며 승인 기록으로 손상을 우회하지 않는다. 검토자 label은 인증된 신원이 아니고 실제 사람 승인은 생성하지 않았다. 입력 필드의 대소문자/유니코드 별칭 덮기도 재현·거부했다.

전체19/28·기능4/7이다. 다음은 KM-05.4의 비적용/미수집/미지원·역할/산업/network/config 차이 검증이다. 남은9개는 KM-05.4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-05.4 적용·지원 범위 시험 목록

기존93개에 아래24개를 더한 비교 패키지117개다. source archive/fixture identity 기반이며 실제 graph/vector 색인 또는 산업 사실의 독립 정답 수가 아니다. 최종 전체 회귀 이후 완료로 확정한다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestCoverageSeparatesCollectionUnknownCapabilityAndScope | 13 | 12개 미수집/미검토/동일·유사 후보/누락 축/network/config/산업/역할/capability |
| TestApplicabilityDeclarationRequiresVersionedSourceAndReview | 1 | 출처 있는 비적용·schema2·기본 미검토 숨김·명시 후보/기록 승인·상태 변경 digest·code-only 거부 |
| TestCoverageWithinProjectConflictDoesNotFlattenProjectDifferences | 1 | 같은 project/scope의 반대 선언과 다른 프로젝트의 정책 차이 분리 |
| TestRustTextAndL2OrderingRemainDistinctFromUnsupportedASTOrConsensus | 1 | Rust 보관 원문 가능·AST 미지원·L2 ordering/L1 consensus 분리 |
| TestSchema1DigestAndCrossScopeRejectionsStayStable | 1 | schema1 digest byte 호환·foreign target/없는 facet 거부 |
| TestCoverageDiscardsEvidenceWhenRetainedArchiveBecomesUnavailable | 1 | 보관 손상 시 원문/정상 상태 반환 금지 |
| TestDeclaredUnknownAndRejectedReviewsNeverBecomeFeatureAbsence | 6 | 5개 미결정·기록 검토·거부/철회/상충은 기능 부재가 아님 |


## KM-05.4 완료 기록

KM-05.4 완료: exact/similar 후보·incomparable·not_collected·unknown·unsupported·not_applicable·conflict·unavailable을 분리했다. 다른 산업/역할·L1 consensus/L2 sequencing·network/config를 구분하고, Rust 원문 읽기와 Rust AST 미지원을 혼동하지 않는다. 비적용/미결정 선언은 source/조건/검토와 함께 처리하며 검색 무응답·거부/회수를 기능 부재로 바꾸지 않는다. schema2 선언을 digest에 포함하고 기존 schema1 식별값은 유지했다. 상충은 같은 프로젝트/조건 안에서만 판단하며 보관 손상은 결과를 폐기한다.

전체20/28·기능5/7 완료다. 다음은 KM-06.1 비교 요청·MCP 계약 설계다. 남은8개는 KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-06.1 공개 비교 요청 계약 시험 목록

기존117개에 아래50개를 더한 비교 패키지167개다. 계약 검증 fixture는 dataset/backend를 열지 않으며 실제 serving readiness/MCP 실행 근거가 아니다. 최종 전체 회귀 후 완료로 확정한다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestComparisonContractRoutesOnlyExplicitParticipantsAndOwnsValues | 1 | 명시 participant·기본 후보 제외·입력/출력 slice 소유·질문은 데이터 |
| TestComparisonRequestRejectionsBeforeAnyBackend | 27 | 26개 schema/mode/scope/collection/concept/facet/time/budget/pin 거부 |
| TestComparisonRequestDecodeRejectsPathsAndAliasOverrides | 6 | 5개 설정/원천/target·case/중복 필드 거부 |
| TestScopeIdentityIncludesVersionConditionsAndKeepsAliasesSeparate | 7 | 6개 project/dataset/snapshot/network/config/role 차이·동등 시각·중복 등록 거부 |
| TestSameProjectExplicitNetworkComparisonIsAllowed | 1 | 같은 project/version·다른 명시 network scope 보존 |
| TestComparisonProfileDecodeIsConfigurationShapeNotReadiness | 8 | 7개 profile 형상 반례·파일이 없어도 readiness로 표시하지 않음 |


## KM-06.1 완료 기록

KM-06.1 완료: 읽기 전용 comparison 모드·고정 profile/catalog·명시 scope 참여자/공통 개념/축·시각/예산 계약을 구현했다. 이름이나 자연어에서 대상을 추측하지 않고2–16개 scope와1–8개 축을 선택한다. stale/타 collection/동일 범위 중복·구현 모드·요청 경로/target 변조·미래 시각/초과 예산을 거부한다. 한 프로젝트의 명시적인 다른 조건도 비교 가능하며 호출자 변경으로 고정 라우팅을 바꾸지 못한다. 이 단계는 계약50개 검증이며 actual source read/MCP 호출0이다.

전체21/28·기능5/7이다. 다음은 KM-06.2의 공개 MCP·프로젝트별 근거 행렬/bundle 구현이다. 남은7개는 KM-06.2–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.2 완료 기록

KM-06.2 완료: 고정된 로컬 profile/catalog·선택 pack/연결/검토·정제 규칙의 실제 byte digest를 고정하는 loader와 독립 stdio 비교 MCP를 구현했다. describe/extract2도구만 노출하고 읽기 전용·비파괴·동일 요청 반복 가능·외부 네트워크 비사용 표시를 명시한다. 각 프로젝트/조건·개념/축의 source 종류·본문/인용·매핑 이유/검토 상태·unknown을 분리하며 양쪽 근거가 있는 공통/차이 후보만 제공한다. 기존 매니저18/updates6도구에 자동 추가된 것으로 세지 않는다.

비교171개(계약167+runtime4), 전체 Go4033(4015통과18skip)·race1007(1005통과2skip,7패키지)·빌드/fmt/vet/lint·문서134개를 검증했다. 실제 native mock2DB 구축 후12항목·9도구호출·2세션 EOF0이며 유료 모델0이다. 통제 source fixture의 실제 조회/transport 검증이며 실제 체인 비교·의미 품질/독립 수용은 아니다.

전체22/28·기능5/7이다. 다음은 KM-06.3 부분 실패·상충/누락·기한/예산·reader/종료 보호의 완성이다. 남은6개는 KM-06.3–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.3 완료 기록

KM-06.3 완료: 모든 참여자를 읽은 뒤 응답 직전에 각 archive를 다시 검증한다. 뒤 프로젝트를 여는 동안 앞 archive가 바뀌면 해당 프로젝트의 본문/인용·후보만 폐기하고 나머지 프로젝트와 명시 좌표는 보존한다. 같은 scope의 적용/비적용 상충과 다른 프로젝트의 차이, 미수집/unknown/손상 상태를 구분한다. 후보 proof의 복사 인용까지 max_citations에 포함하며 예산에 맞지 않으면 후보 전체를 제외하고 omitted_candidates/candidate_state로 알린다. 요청 기한/취소에는 이전 부분 성공을 공개하지 않고 Close는 진행 중 reader 정리를 기다린다.

비교192개(기존171+보호21,47개 상위), 전체 Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint·문서134개를 검증했다. 실제 mock2DB·기반12+보호9항목·18도구호출·4세션(EOF3개와 입력이 열린 SIGTERM1개 모두 exit0)이다. 원문 손상 후 정확히 복구했고 DB/manifest/profile/catalog/binding 전체 hash를 보존했다. 실제 체인 비교·의미 품질/독립 수용은 아니다.

전체23/28·기능5/7이다. 다음은 KM-06.4 둘 이상 실제 프로젝트의 고정 DB·staking/정책·consensus/finality·mempool/ordering 비교 검증이다. 남은5개는 KM-06.4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.4 완료 기록

KM-06.4 완료: go-stablenet0bf2f4d1의committed 원문2,224파일과 go-wemix1350376a의working-tree 원문2,019파일로 실제 native graph/vector DB를 구축했다. go-wemix의 cmd/gwemix→geth alias만 소유 복제본에서 제외하고 BaseCommit·제외 목록·준비 Git 상태를 기록했다. 비교하는7개 Go 파일은 원본 커밋 바이트와 동일하며 원본 저장소는 변경하지 않았다. 모든 매핑은 미검토 후보다.6개 개념·40개 프로젝트/축 셀에서7개similar·18개unknown·15개not_collected를 관찰했다. 이는 선택한 매핑/근거의 범위이며 실제 네트워크나 기능 부재·finality·TPS·보안성·우월성의 판정이 아니다.

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. 새 Go 변경이 없음을1273개 Go와 모든 결속 입력의 hash로 확인해 직전 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 실제 corpus/MCP 시험과 과거 Go 실행을 구분한다. 유료 모델 호출0이며 ontology 효과/독립 품질 수용은 아니다.

전체24/28·기능6/7이다. 다음은 OE-01.1의 새 질문·기대 근거·평가 범위/분모 고정이다. 남은4개는 OE-01.1 질문/기대 근거, OE-01.2 동일 자료4모드, OE-01.3 근거·순위·unknown·지연/비용, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지하며 단계별 서명 로컬 커밋으로 계속한다.


## OE-01.1 완료 기록

OE-01.1 완료: 등록 용어의 한영 표현·다른 용어·다의어(validatorMu/currentMutex)·주석/구현 차이·범위 밖을 포함한 새16질문을 사전 등록했다. 함수 찾기14질문은5사실군·고유 함수11개의 완전한 source span을 기준으로 top10의 함수 회수 precision/recall·순위를 계산하며, 제어2질문은 일반 source relevance를 채점하지 않고 ontology 의도 범위와 원문 무결성을 관찰한다. 일반 행동/정책의 완전한 관련 근거나 답변 정답을 뜻하지 않는다. 기준은 assistant source-derived이며 사람/독립 gold 승인은0이다. 이전13/42문항을 새 독립 효과 입력으로 재사용하지 않고, 기존 사실군과 표현 변형의 중복도 명시한다.

source commit baca2869·동일 비시험 Go882개·승인 정의4개/검토된 구현 연결0개·BGE-M3 digest790764642607…16bab/1024차원을 고정했다. 현재 로컬 Ollama0.40.4를 기록하므로 과거 서버 버전의 절대 지연을 새 관찰과 합산하지 않는다. 동일 자료/모델/모드 외 설정과 ontology mode만 바꾸는4arm, 질문별 준비1/검색2/warm5·순서 교대·총512회 계획이다. 사전 등록 단계의 새 검색/LLM 답변/유료 모델 호출은 모두0이다. 후속 실행은 소유 loopback proxy로 API 경로·해시/bytes·상태·시간·서버가 제공한 token count를 계측하고, bytes에서 token/금액을 추정하지 않는다. 작은 표본은 기술 통계로 보고한다.

전체25/28·기능6/7이다. 다음은 OE-01.2의 고정512회4모드 실행이다. 남은3개는 OE-01.2 동일 자료4모드, OE-01.3 함수 근거·순위·ontology 범위/unknown·지연/API 호출/관측 비용, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 기본 off·quality=inconclusive는 실제 품질 승인 없이 바꾸지 않는다.


## OE-01.2 완료 기록

OE-01.2 완료: 사전 등록16질문×4모드×8호출=512회를 실제 로컬 BGE-M3로 수집했다.4개 readiness와64개 capture 세션의 종료, 질문/모드/단계/반복 슬롯·설정/바이너리·source/snapshot·인용/본문/무결성 hash를 검증했다. 검토된 정의4개/구현 연결0개를 보존하고, mode 외 설정은 동일하다. 원본 graph/vector/source·의미 DB·바이너리·모델/서버가 전후 동일하며 소유 loopback API 계측기는 진행 중 요청0으로 종료했다. 이 단계는 실행/원문 검증이며 효과 점수나 품질 수용이 아니다.

인용4896개·본문4896개·원문 blob65개를 검증했고 실패/누락0이다. 실제 API 20420개를 metadata/준비/질문-arm별로 기록했다. 기본 off·quality=inconclusive와 사람/독립 gold0·실제 LLM 답변0·유료 모델0을 유지한다. 초기 off 기본값 정규화 오류와 계측 로그 전체 재작성에 따른 제어 중단을 별도로 보존했다. 초기 지연/부분 실행은 본512회 지표에 합산하지 않는다.

전체26/28·기능6/7이다. 다음은 OE-01.3의 고정 함수 회수 precision/recall·순위·ontology 의도 범위/unknown·warm 지연/API 호출/관측 비용 재계산이다. 남은2개는 OE-01.3 분석, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다.


## OE-01.3 완료 기록

OE-01.3 완료: 고정512회 원자료에서 유한한 완전 함수 찾기 과제의 precision/recall·순위·의도한 ontology 개념/범위 진단·warm 지연·실제 로컬 API 호출/서버 token 수를 재계산했다.5사실군 평균 recall은 off54.44%→concept_text/combined56.67%, precision12.87%→13.54%, MRR0.4612→0.4862다. 개선은 EX-08 영어 메시지 처리 질문1개에서 prepare만 회수하던 결과에 commit이 추가된 것으로 한정된다. 나머지13개 채점 질문의 지표는 같았다. hit@10은68.33%로 동일하다. 일반 행동/정책의 semantic relevance나 답변 정확도로 확대하지 않는다.

각 arm의 warm80개 p95는 off583.1ms, concept_text736.2ms(1.263배), relations607.4ms(1.042배), combined697.4ms(1.196배)다. capture batch embedding API는 off/relations1584개·31536서버 prompt tokens, concept_text/combined1664개·56112tokens다. 준비/모델 확인도 포함한 도구 실행 batch 비용이며8호출×16질문의512개 독립 사실이 아니다. 토큰은 서버 보고값이고 금액/전력/하드웨어 비용은 추정하지 않았다. 유료/LLM 답변0이다. 활성 모드는 의도 개념13질문 중9개를 매칭했고, scope 의도 진단3질문 중 외부 go-wemix 대상 EX-16에도 WBFT quorum을 매칭했다. relations는 검토된 구현 연결0·rank 변화0이다. 정의 검색/combined의 최종 인용 순서는32짝 중18짝에서 달랐지만 함수 회수 이득은1질문뿐이다. 작은 source-derived 개발 표본과 지연 관찰로 기본 off·quality=inconclusive를 유지한다.

전체27/28·기능6/7이다. 다음이자 마지막 세부 작업은 OE-01.4의 실제2프로젝트 공통 개념 매핑 효과·오인/누락 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 실제 LLM 답변·독립 정답·운영 활성화는 미실행/미승인 상태로 보존한다.

## OE-01.4 실제 다중 프로젝트 매핑 효과 평가

동일한 실제 go-stablenet/go-wemix 보관 DB와6개 개념·40개 프로젝트/축 셀을 대상으로, binding 입력만 뺀 기준선과 고정된 제안 매핑7개를 비교했다. 양쪽 모두 include_proposed=true이며 기본 검토 필터는 별도 확인했다. 프로토콜을 호출 전에 저장하고 순서를 교대해2회 반복했다. 24개 행렬+describe2개+기본 필터1개=도구27호출, 검증18항목, 두 연결 EOF0이다. 반복80셀을 독립 사실80개로 세지 않고 arm별 고유40셀로 보고한다.

| 관측값 | 매핑 없는 기준선 | 제안 매핑 입력 |
|---|---:|---:|
| 고유 프로젝트/축 셀 | 40 | 40 |
| 원문 근거가 있는 셀 | 0 (0%) | 8 (20%) |
| 선언된 exact/similar/not_applicable 셀 | 0 | 7 (17.5%, 모두 similar) |
| unknown | 0 | 18 |
| not_collected | 40 | 15 |
| 검증한 셀 인용 | 0 | 9 |
| 양쪽 근거가 있는 비교 후보 | 0 | 3 |
| 검사한 프로젝트 그룹 / 혼입 | 12 / 0 | 12 / 0 |

9개 인용의 프로젝트·버전·source mode·원문/범위 hash·본문을 검증했다. 근거0개인 기준선의 source accuracy는 측정 불가(null)다. 처리군의 원문 일치100%는 hash/body 무결성 범위이며 의미 정확도100%가 아니다. 두 arm은 동일한 scope 격리 장치를 사용하므로 혼입0을 매핑만의 효과로 돌리지 않는다. finality 근거1셀은 unknown을 유지하며, staking/정책 미수집을 기능 부재로 단정하지 않는다. 기본 reviewed-only 요청에는 미검토 매핑이 나타나지 않는다. DB/source/config 전후 byte는 동일하다.

매핑은 평가자가 원문을 보고 선별한 제안 입력이다. 따라서 이 결과는 **연결 주석을 추가하면 비교 가능한 원문을 구조적으로 모을 수 있다**는 제한된 관찰이며, 온톨로지 어휘 자체의 인과 효과·독립 holdout·자연어 의도 해석·LLM 답변 품질·실제 네트워크 성능 평가가 아니다. 기존 단일 프로젝트 분석과 함께 기본 off·quality=inconclusive를 유지한다. 사람의 매핑/정답 승인과 정식 독립 수용은 기록하지 않았다.

[원자료와 검증](../../system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect/manifest.json)에 protocol·원시 요청/응답·소스·재계산/반례 시험을 보존한다. 로드맵 **28/28·기능7/7의 로컬 구현/명시된 개발 검증을 완료**했다. 정책 생성기 보완, 운영·독립 수용9개, 선택 후속12개는 별도 범위다. 유료 모델·운영 활성화·외부 게시 승인을 이 완료로 대신하지 않는다.

## 소스 대조 정책 생성기 보완 완료 (2026-10-16)

명시적 `cks domain sync --code-root <source>`는 폴더 이름 대신 실제 Go 선언을 읽어 package·receiver·type/field/const/var/interface 이름을 해당 anchor 파일에서 대조한다. receiver 없는 이름은 해당 파일에서 유일할 때만 보완하고, 오파일/모호한 이름/외부·symlink 경로/손상된 소스는 두 출력을 쓰기 전에 거부한다. symbol이 없는 파일/행 전용 anchor는 새 정책 연결로 만들지 않는다. 기존 무소스 sync는 이전 결과를 보존한다. 이 경로는 명시적 옵션이며 기존 setup/운영 profile을 자동으로 전환하지 않는다.

고정 소스0bf2f4d1에서 원래 ErrInvalidFeePayer 파일 참조는 거부되고 두 기존 출력은 보존됐다. 소유 평가 사본의 해당 anchor 파일만 core/txpool/errors.go로 수정한 후, 승인 overlay와 같은42개 정책·146개 연결을 재생성했다. 생성기는 참조를 결정적으로 정렬하므로 승인 overlay의 기존 목록 순서와 byte는 일부 다르지만, 모든 정책 필드와 정렬한 governs 목록은 같다. 원본 entry·정책·승인 overlay와 원본 소스 HEAD/status를 보존했다. 원래 entry의 잘못된 파일 위치를 조용히 찾아 옮기거나 원본 설명/검토 승인을 변경하지 않았다.

새 생성 결과로 실제 graph를 구축하고146개 연결을 고정 AST 위치와 대조했다. 수정7개에 대해 공개 CKS 일반·소스/snapshot pin·canonical ID 조회21회, 잘못된 pin2개·assembler 혼입1개 거부/빈 결과까지 총24회 도구 조회와26쌍 원시 요청/응답을 검증했다. CKS는 EOF0으로 종료했다.843개 파일이 graph에 선택됐고 parse error0이다. source corpus는882개 비시험 Git Go blob이며 대상 프로젝트 코드를 실행하거나 실제 LLM/유료 모델을 호출하지 않았다.

함수뿐 아니라 실제 정책의 구조체 필드·인터페이스도 확인했다. 초기 진단의 필드 누락과 정책 연결을 요청하지 않는 loc anchor 처리, CLI 오류 정제 정책에 대한 평가 도구의 가정을 수정한 이력은 최종 통과와 구분해 보존한다. 원자료 재계산/반례·새 전체 Go/race·빌드/fmt/vet/lint·문서 검증은 [증거](../../system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration/manifest.json)를 따른다. 이는 source-name 연결 정합성의 개발 검증이며 정책 의미의 사람/독립 수용이나 운영 채택이 아니다.

현재 로컬 로드맵28/28·기능7/7과 별도 생성기 보완을 완료했다. 남은 별도 범위는 운영·독립 수용9개와 선택 후속12개다. 다음은 독립 평가를 실제로 수행할 경우 해당 입력·검토/실행 범위·필요한 실제 모델 조건을 고정하는 단계이며, 로컬 검증을 그 승인/수용으로 대신하지 않는다.
