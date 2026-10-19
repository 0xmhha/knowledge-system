# 구현·시험 진행표 / 2026-10-19

[코드·검증 결과](./operational-preparation/source-review-supplement/followups/local-reliability-v1/SUMMARY.json) · [실제 경로](./operational-preparation/source-review-supplement/followups/local-reliability-v1/runtime-summary.json) · [13개 실행 원장](./REMAINING-13-EXECUTION.json) · [남은 전체 작업](../REMAINING-WORK.md)

로컬 구현9/9·지식 매니저 로드맵7/7(28/28)과 정식 독립 수용0/9를 구분한다. D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false를 유지한다. 실명/연락처는 요구하지 않으며 익명 역할로 검토할 수 있다.

7개/28개는 기존 로컬 구현·도구 시험의 완료 기록이다. 아래 모든 완료는 당시 명시한 입력/host/프로젝트 범위에 한정되며 독립 정답 승인이나 일반 품질 향상을 뜻하지 않는다. 이전 중간 집계와 순차 실행 로그는 Git 판본 및 불변 manifest에 남긴다. 현재 상태에는 과거의 6/28·미구현 문구를 다시 적용하지 않는다.

## 기존28개 로컬 작업

|ID|목적|상태|
|---|---|---|
|KM-01.1|신원·key·cache·join·쓰기 조사와 이행 설계|기존 로컬 구현 완료|
|KM-01.2|Project/Version/Collection과 고정 scope 계약|기존 로컬 구현 완료|
|KM-01.3|구현 대상 제한·명시 비교·잘못된 scope 거부|기존 로컬 구현 완료|
|KM-01.4|동명·fork·두 버전·cache·재시작·쓰기/호환 반례|기존 로컬 구현 완료|
|KM-02.1|legacy/versioned capability와 새 요청·상태 계약|기존 로컬 구현 완료|
|KM-02.2|비동기 build/status/cancel과 정합 gate|기존 로컬 구현 완료|
|KM-02.3|idempotency/lock/중단/실패/rollback|기존 로컬 구현 완료|
|KM-02.4|공개 도구의 잘못된 대상·부분 실패·취소 보호|기존 로컬 구현 완료|
|KM-03.1|매니저 등록·설치·초기 구축·복구 절차|기존 로컬 구현 완료|
|KM-03.2|문서 없는 프로젝트의 근거 기반 지식 초안|기존 로컬 구현 완료|
|KM-03.3|암묵지 질문·불확실성·검토/채택/회수|기존 로컬 구현 완료|
|KM-03.4|실제 Claude host 전체 구축·조회와 실패 복구|기존 로컬 구현 완료|
|KM-04.1|수동·정기·변경 trigger와 성공 기준|기존 로컬 구현 완료|
|KM-04.2|변경/삭제/no-op/stale와 재검토|기존 로컬 구현 완료|
|KM-04.3|중복·lease·재시작·retry·자원 상한|기존 로컬 구현 완료|
|KM-04.4|두 번 변경+no-op+실패 복구·실제 주기 실행|기존 로컬 구현 완료|
|KM-05.1|산업 개념·정의·별칭·비교 축|기존 로컬 구현 완료|
|KM-05.2|공통 개념과 프로젝트별 코드/정책/시험 연결|기존 로컬 구현 완료|
|KM-05.3|exact/close/broader 등의 검토·버전 의미|기존 로컬 구현 완료|
|KM-05.4|비적용/미수집/미지원·L1/L2·network 차이|기존 로컬 구현 완료|
|KM-06.1|명시 버전·개념·축·시점·상한의 비교 요청|기존 로컬 구현 완료|
|KM-06.2|프로젝트별 근거 행렬/bundle|기존 로컬 구현 완료|
|KM-06.3|상충·누락·자료 부족·부분 실패 상태|기존 로컬 구현 완료|
|KM-06.4|둘 이상 실제 프로젝트의 비교·오인·근거 시험|기존 로컬 구현 완료|
|OE-01.1|질문·기대 근거·독립성·분모 고정|기존 로컬 구현 완료|
|OE-01.2|같은 자료의 off/concept_text/relations/combined|기존 로컬 구현 완료|
|OE-01.3|근거·순위·unknown·지연/비용·답변 효과|기존 로컬 구현 완료|
|OE-01.4|다중 프로젝트 개념 매핑·오인·비교 효과|기존 로컬 구현 완료|

## 범위별 실행 근거

|묶음|기능|불변 실행 기록|
|---|---|---|
|KM-01|프로젝트 격리와 collection 스키마|[km-01-foundation](../../system/eval/knowledge-roadmap-2026-10-15/km-01-foundation/manifest.json) · [km-01-public-read](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/manifest.json) · [km-01-final-isolation](../../system/eval/knowledge-roadmap-2026-10-15/km-01-final-isolation/manifest.json)|
|KM-02|버전 DB 구축·갱신 도구 계약|[km-02-contract](../../system/eval/knowledge-roadmap-2026-10-15/km-02-contract/manifest.json) · [km-02-public-maintenance](../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-maintenance/manifest.json) · [km-02-durable-host](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/manifest.json) · [km-02-durable-retention](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-retention/manifest.json) · [km-02-public-failures](../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-failures/manifest.json)|
|KM-03|Claude 지식 매니저 플러그인|[km-03-bootstrap](../../system/eval/knowledge-roadmap-2026-10-15/km-03-bootstrap/manifest.json) · [km-03-collection](../../system/eval/knowledge-roadmap-2026-10-15/km-03-collection/manifest.json) · [km-03-review](../../system/eval/knowledge-roadmap-2026-10-15/km-03-review/manifest.json) · [km-03-document-index](../../system/eval/knowledge-roadmap-2026-10-15/km-03-document-index/manifest.json) · [km-03-broker](../../system/eval/knowledge-roadmap-2026-10-15/km-03-broker/manifest.json) · [km-03-host](../../system/eval/knowledge-roadmap-2026-10-15/km-03-host/manifest.json)|
|KM-04|매니저의 지속·주기 갱신|[km-04-contract](../../system/eval/knowledge-roadmap-2026-10-15/km-04-contract/manifest.json) · [km-04-inputs](../../system/eval/knowledge-roadmap-2026-10-15/km-04-inputs/manifest.json) · [km-04-source-check](../../system/eval/knowledge-roadmap-2026-10-15/km-04-source-check/manifest.json) · [km-04-state](../../system/eval/knowledge-roadmap-2026-10-15/km-04-state/manifest.json) · [km-04-worker](../../system/eval/knowledge-roadmap-2026-10-15/km-04-worker/manifest.json) · [km-04-worker-host](../../system/eval/knowledge-roadmap-2026-10-16/km-04-worker-host/manifest.json) · [km-04-updates-MCP](../../system/eval/knowledge-roadmap-2026-10-16/km-04-updates-MCP/manifest.json) · [km-04-periodic](../../system/eval/knowledge-roadmap-2026-10-16/km-04-periodic/manifest.json)|
|KM-05|공통 산업 개념과 프로젝트별 매핑|[km-05-scheme](../../system/eval/knowledge-roadmap-2026-10-16/km-05-scheme/manifest.json) · [km-05-bindings](../../system/eval/knowledge-roadmap-2026-10-16/km-05-bindings/manifest.json) · [km-05-mapping-review](../../system/eval/knowledge-roadmap-2026-10-16/km-05-mapping-review/manifest.json) · [km-05-applicability](../../system/eval/knowledge-roadmap-2026-10-16/km-05-applicability/manifest.json)|
|KM-06|비교 근거 추출과 MCP 제공|[km-06-contract](../../system/eval/knowledge-roadmap-2026-10-16/km-06-contract/manifest.json) · [km-06-runtime](../../system/eval/knowledge-roadmap-2026-10-16/km-06-runtime/manifest.json) · [km-06-failures](../../system/eval/knowledge-roadmap-2026-10-16/km-06-failures/manifest.json) · [km-06-real-projects](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/manifest.json)|
|OE-01|온톨로지 효과 평가|[ontology-effect-dev](../../docs/spec-driven/operational-preparation/ontology-effect-dev/manifest.json) · [ontology-effect-dev](../../system/eval/operational-preparation-2026-10-15/ontology-effect-dev/manifest.json) · [oe-01-protocol](../../system/eval/knowledge-roadmap-2026-10-16/oe-01-protocol/manifest.json) · [oe-02-capture](../../system/eval/knowledge-roadmap-2026-10-16/oe-02-capture/manifest.json) · [oe-03-score](../../system/eval/knowledge-roadmap-2026-10-16/oe-03-score/manifest.json) · [oe-04-mapping-effect](../../system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect/manifest.json)|

KM-03 host 기록은 실제 설치 host와 로컬 통제 응답의 연결 시험을 포함한다. 실제 모델 답변 품질로 바꾸지 않는다. OE-01은 개발 입력의 off/on·두 프로젝트 비교와 분석 파이프라인의 완료다. 독립 의미/검색 정답은 승인되지 않았으며 효과 수용은 inconclusive·기본 off를 유지한다.

## 현재 변경의 검증

|항목|현재 근거|한계|
|---|---|---|
|전체 native|일반4172·race4172통과, 각18skip; build/fmt/vet/고정 lint|native 입력1741·승인5; 운영/독립 수용 아님|
|WI-1 실제 소비자|30함수·정의16보정|range37 지원 밖·빈블록3 미확정|
|임베딩 캐시|실제 CKS off2/on1 query HTTP·같은 결과/출처|기본0; 일반 품질·속도 수용 아님|
|모델 장애·회복|실제9호출·오류 중계·캐시 뒤 장애 표시·fresh probe 회복|로컬 BGE·소유 중계기; 유료 LLM 아님|
|종료 판정|반환 후 게이트10회·유지보수171개·새 전체 검사|최초1건 원인 미확정·이전 실패 원자료 보존|
|WI-6 반복|627쌍·본570 partial/budget·완전0|독립 품질/완전 응답 지연 판정유보|
|실제 node/eth factory|일반/race·활성 indexer·원래60초 RPC·unsubscribe/node.Close·exit0|전체8노드 중 최종2회 성공; 실제 피어 sync/5일정12계약/독립 수용은 남음|
|정식 수용|0/9|D04/새 DEV·FINAL/gold/F01~08 미정·미동결|

현재13묶음의 완료 기준과 다음 작업은 [전체 실행 원장](./REMAINING-13-EXECUTION.json)을 따른다. 데이터 원본·프로세스 정리·호출/실패 분모·실제 서명 저장을 검증하며, 노출된 DEV·반복 호출·AI 초안을 독립 표본으로 세지 않는다.

단계74의 [실제 생성 경로 원자료](./operational-preparation/source-review-supplement/followups/full-host-factory-v1/SUMMARY.json)는 기존 구현28개 완료를 추가 확대하지 않는다. 실패6회와 실행 전 컴파일 실패1회를 포함한 [보고서](./operational-preparation/source-review-supplement/followups/full-host-factory-v1/REPORT.txt)를 함께 따른다.
