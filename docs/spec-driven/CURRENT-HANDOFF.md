# 현재 인수인계

## 최신 상태 — 2026-10-18 / SN-P04

비운영 pilot **4/4 완료**. 실제 go-wemix1350376a의 새 BGE-M3/native1024 held DB와 go-stablenet0bf2f4d1의 보관 DB에서 같은6진단 입력·검색·v2·6개념 비교·출처/혼입 경계를 검증했다. 기존 go-wemix 자료는 mock64이며 새 BGE 자료와 구분한다. 서로 바뀐 실제 graph/vector 설정2개는 기동 거부, 최종정상5+초기/보완전진단6세션은 EOF0·모든 소유 프로세스 회수·강제종료0이다. corpus/정책 범위가 다르며 매핑은 proposed라 품질 우열·운영망 합의·온톨로지 독립 효과를 승인한 결과가 아니다.

새 소스38dd5c54(Go1738개)에서 일반/race 각4097pass/18skip, build/fmt/vet/lint/boundaries/docs/build-bins 및 별도 비교/격리 race232개가 새로 통과했다. 이전 결과 재사용으로 대신하지 않았다. [최신 보고서](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/REPORT.txt)와 [현재 작업리스트](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/WORKLIST.json)가 최신 상태다.

다음은 독립 DEV/FINAL·완전 gold·matching/충분성·F01~08 동결 준비다. 남은 전체는 해당 준비, 정식 WI9개(0/9), 필수 온톨로지 독립 효과 평가, 별도 후속12개다. D04미정·freeze null/execution_allowed=false, 로컬 로드맵7/7·28/28·준비22/22 및 기존 제한 온톨로지 연구완료/inconclusive를 유지한다. 운영 current·전역 설정·외부 게시를 변경하지 않았다.

이하 이전 시점의 구현/연구 기록은 당시 범위이며 현재 상태는 위 최신 보고서·원장을 우선한다.

운영·독립 수용 준비 갱신(2026-10-16): source3a530a25 격리 빌드·CKS33개 스키마 일치/ChainBench81개 metadata·현재 Claude Code2.1.301의 로컬 scripted5개를 새로 검증했다. 실제 모델/노드0, 정식9개 수용0/9·F-01–08 pending이다.9개 baseline/표본/위치 매칭/분모 제안과 역할·입력·recipe의 남은 결정을 [현재 protocol](./OPERATIONAL-ACCEPTANCE-PROTOCOL.md)에 정리했다. 다음은 검토자·범위·독립 입력/gold 검토다.


2026-10-16 기준 지식 매니저·프로젝트 비교 로드맵은 **28/28 세부 작업·7/7 기능 완료**다. 사용자가 멈추라고 하기 전까지 남은 작업을 순서대로 구현·검증하고 단계별 서명 로컬 커밋으로 보존한다. 각 완료 단계마다 현재 단계·검증 결과·다음 작업·남은 전체 범위를 화면 로그와 문서에 기록한다. 세부 상태는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

## 현재 구현과 다음 단계

| 기능 | 상태 | 확인한 범위 |
|---|---|---|
| KM-01 프로젝트 격리 | 4/4 완료 | 실제4DB·동명/포크/버전·인용·공개 읽기/쓰기 거부·rollback/GC |
| KM-02 버전 DB 운영 | 4/4 완료 | 공개 build/status/cancel·영속 job/재시작·유지/실패 matrix |
| KM-03 Claude 매니저 | 4/4 완료 | 등록·문서 없는 수집/초안·검토/회수·실제 설치 CLI8회/68도구; 로컬 시험 응답기 |
| KM-04 지속 갱신 | 4/4 완료 | 실제 periodic/change2프로젝트·20항목·221호출·2EOF0·삭제/no-op·실패/연결 retry·타 DB/인용 보존 |
| KM-05 공통 개념/연결 | 4/4 완료 | 공통 어휘·원문 연결·직접 매핑 검토/회수·적용/지원·L1/L2/network/산업 차이 |
| KM-06 비교 근거 MCP | 4/4 완료 | 실제 go-stablenet/go-wemix4243파일·6개념40셀·22항목19호출·3EOF0·오매핑/범위/예산/원본 보존 |
| OE-01 온톨로지 효과 | 4/4 완료 | 512회 함수 과제+실제2프로젝트 매핑 입력 효과18항목/27호출·원문8셀/양측후보3개; off/inconclusive |

OE-01.3 완료: 고정512회 원자료에서 유한한 완전 함수 찾기 과제의 precision/recall·순위·의도한 ontology 개념/범위 진단·warm 지연·실제 로컬 API 호출/서버 token 수를 재계산했다.5사실군 평균 recall은 off54.44%→concept_text/combined56.67%, precision12.87%→13.54%, MRR0.4612→0.4862다. 개선은 EX-08 영어 메시지 처리 질문1개에서 prepare만 회수하던 결과에 commit이 추가된 것으로 한정된다. 나머지13개 채점 질문의 지표는 같았다. hit@10은68.33%로 동일하다. 일반 행동/정책의 semantic relevance나 답변 정확도로 확대하지 않는다.

각 arm의 warm80개 p95는 off583.1ms, concept_text736.2ms(1.263배), relations607.4ms(1.042배), combined697.4ms(1.196배)다. capture batch embedding API는 off/relations1584개·31536서버 prompt tokens, concept_text/combined1664개·56112tokens다. 준비/모델 확인도 포함한 도구 실행 batch 비용이며8호출×16질문의512개 독립 사실이 아니다. 토큰은 서버 보고값이고 금액/전력/하드웨어 비용은 추정하지 않았다. 유료/LLM 답변0이다. 활성 모드는 의도 개념13질문 중9개를 매칭했고, scope 의도 진단3질문 중 외부 go-wemix 대상 EX-16에도 WBFT quorum을 매칭했다. relations는 검토된 구현 연결0·rank 변화0이다. 정의 검색/combined의 최종 인용 순서는32짝 중18짝에서 달랐지만 함수 회수 이득은1질문뿐이다. 작은 source-derived 개발 표본과 지연 관찰로 기본 off·quality=inconclusive를 유지한다.

전체28/28·기능7/7의 문서화된 로컬 로드맵을 완료했다. 별도 정책 생성기의 명시적 소스 대조 재생성 보완도 완료했다. 다음 별도 단계는 운영·독립 수용 범위를 동결하는 일이다. 운영·독립 수용9개와 선택 후속12개는 별도 범위로 유지하며 미실행/미승인 사항을 완료로 바꾸지 않는다.

## 검증과 현재 제한

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. 새 Go 변경이 없음을1273개 Go와 모든 결속 입력의 hash로 확인해 직전 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 실제 corpus/MCP 시험과 과거 Go 실행을 구분한다. 유료 모델 호출0이며 ontology 효과/독립 품질 수용은 아니다. [검증 자료](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/manifest.json)를 따른다.

매니저 plugin0.6.0은 reviewed 기본18도구와 명시 updates6도구를 제공한다. 실제 Claude Code2.1.300과의 갱신 연결2회/36도구도 로컬 시험 응답기로 검증했다. 실제 모델 호출/비용0이며 agent 자율성·답변 품질은 측정하지 않았다. 개인 머신에 서비스나 OS scheduler를 설치하지 않았고 명시 foreground worker가 실행 중인 동안만 주기/변경 polling이 동작한다.

원래 독립 수용의 단계0·담당자·실제 모델/인증/비용 지정은 로컬 개발을 막는 요건이 아니다. 유료 모델·독립 수용·외부 게시에는 해당 실행 범위가 별도로 필요하다. 이번 지속 작업은 단계별 로컬 커밋 범위다. 푸시/PR·전역 설치·운영 current 변경을 완료로 추정하지 않는다.

## 보존할 결정과 품질 관찰

정책 참조 수정7건은 별도 평가 overlay로 사용자 채택 승인을 기록했다. [승인 영수증](../../system/eval/operational-preparation-2026-10-15/adopted-policy-user-approval/receipt.json)과 [적용 검증](./POLICY-OVERLAY-ADOPTION.md)을 따른다. 원본 정책·원래 승인 입력은 보존한다. 기존 무소스 sync의 동작을 보존하면서 명시적 --code-root의 선언 대조/오파일 거부와 재생성을 보완했다.

[OE-01 단일 프로젝트 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)은384회 실패/누락0·인용3,616개 원문 일치, 활성288회 no_match·순위 차이0이었다. 별도 정확 용어8회도 최종 boost0이다. 기본 off·quality=inconclusive를 유지한다. KM-04.4에서 단독 합성 대문자 식별자 질의가 비고 같은 자료가 자연어 질의에 나타난 사례도 OE-01 질의 해석/회수율 후보로 보존했다. 새 기능 구현이나 계획 승인을 독립 정답·품질 승인으로 바꾸지 않는다.

이전 단계의 상세 기록은 각 명세의 checkpoint 절과 서명 커밋·동결 증거 packet에 남긴다. 현재 상태는 이 문서 첫 표와 전용 원장을 우선한다.

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
