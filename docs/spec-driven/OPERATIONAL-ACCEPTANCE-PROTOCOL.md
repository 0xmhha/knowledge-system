# 운영·독립 수용 프로토콜 / 2026-10-19

정본은 [프로토콜 JSON](./OPERATIONAL-ACCEPTANCE-PROTOCOL.json), [수용 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json), [결정 제안](./OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json)이다. 이 문서는 현재 계약을 요약하며 승인·역할·정답·동결을 대신 생성하지 않는다.

로컬 구현9/9·지식 매니저 로드맵7/7(28/28)과 정식 독립 수용0/9를 구분한다. D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false를 유지한다. 실명/연락처는 요구하지 않으며 익명 역할로 검토할 수 있다.

## 수용 대상과 완료의 의미

정식 대상은 WI-1/2/3/4/V1/V2/6/SP/C의9개다. 구현9/9와 로드맵28/28의 로컬 검증은 유지하지만, 새 독립 입력·완전 정답·매칭·표본 충분성·실제 소비자·수용 receipt가 충족되기 전에는 정식 완료로 올리지 않는다. 남은13묶음은 [실행 원장](./REMAINING-13-EXECUTION.json)을 따른다.

보안 전용 gold·권한/비공개/위조 경계와 조직 인증은 사용자 보류다. WI-SP의 현재 작업은 비보안 출처·정책/미태그 범위다. 정식 범위 동결 때 이 제외를 명시하며, 보류를 완료나 승인된 빈 정답으로 바꾸지 않는다.

## F01~08 동결 조건

|ID|확정할 내용|현재|
|---|---|---|
|F-01|대상·완료와 시험/운영/게시의 경계|pending|
|F-02|실제 source·candidate·binary·runtime pin|pending|
|F-03|sampling·완전 정답·matching·scoring|pending|
|F-04|독립 가족·분모·통계 방법·표본 충분성|pending|
|F-05|익명 역할·독립성·검토/수용 기록|미정|
|F-06|실제 LLM 선택 시 모델·인증 방식·비용/자원 상한|로컬 무유료 범위 승인·실제 LLM 미정|
|F-07|새 DEV/FINAL·gold·holdout·노출/중복 관리|pending|
|F-08|정확한 실제 실행 경로·소유 자원·종료/복구|pending|

역할은 실명·연락처 없이 불투명한 익명 ID와 역할/독립성 기록으로 남길 수 있다. AI 작성 초안을 독립 검토로 변경하지 않는다. 담당자 미정인 동안 로컬 구현·기계 검증·AI 초안 보완은 계속할 수 있다.

## 입력·정답·매칭

- 원문 commit/source mode·project/dataset/version/snapshot·file/function hash와 UTF-8 반개구간 좌표를 결속한다. 동명 symbol·경로·import path만으로 다른 프로젝트/버전을 합치지 않는다.
- 경계가 있는 정답 우주에서 관련/잡음/unknown 또는 요구별 모든 집합을 작성한다. 빈 집합에는 검토된 이유가 필요하며 누락을 빈 정답으로 취급하지 않는다.
- 기존 source/옛 질문/노출된 DEV·AI 초안·반복 호출을 새 독립 FINAL로 세지 않는다. DEV/FINAL은 독립 가족과 노출 기록으로 분리한다.
- 실제 결과와 matching 규칙을 고정한다. seed·중복·범위 밖·부분·미도달·오류·unsupported를 보존하고 성공 결과만 분모로 사용하지 않는다. gold는 검색 결과에서 역으로 만들지 않는다.
- 작은 표본·비교 불가능·정의되지 않은 recall/ratio는 inconclusive다. ontology 효과와 corpus 변경 효과, 검색 효과와 LLM 답변 효과를 분리한다.

## 실행과 측정

같은 source/model/전처리/pin/상한의 baseline/처리 arm을 비교한다. warmup과 본 측정을 분리하고, 지연은 arm별 nearest-rank p95를 구한 뒤 두 p95의 비율을 사용한다. 부분/예산/오류 처리 시간을 완전 응답 지연이나 품질 통과로 해석하지 않는다. 실행 후 기준·분모·상한을 바꾸어 성공 결과만 다시 고르지 않는다.

소유 임시 source/index/process/연결만 생성·정리하고 원본·전역 설정·운영 상태를 보존한다. timeout·취소·EOF·명시한 강제 종료와 복구를 따로 기록한다. 원자료/도구/바이너리·Git byte/signature 결속은 저장 검증이며 의미·품질의 독립 승인이 아니다.

## 현재 근거와 한계

[코드·검증 결과](./operational-preparation/source-review-supplement/followups/local-reliability-v1/SUMMARY.json) · [실제 경로](./operational-preparation/source-review-supplement/followups/local-reliability-v1/runtime-summary.json) · [13개 실행 원장](./REMAINING-13-EXECUTION.json) · [남은 전체 작업](../REMAINING-WORK.md)

단계72의 WI-6은627쌍을 기록했으나 본570쌍 모두 partial/budget·완전쌍0이다. 단계73의 실제30함수·캐시/모델 경계46도구·50요청과 새 전체 native 검사는 로컬 계약 근거다. 최초 종료 시험1건의 원인은 반복 진단에서 확정하지 못했고, 반환 후 쓰기를 판별하는 시험을 강화했다. 모든 시도·실패·범위를 유지하며 정식0/9·독립승인0·freeze null을 변경하지 않는다.

실제 LLM 답변 평가는 모델·인증 방식·총 비용 상한이 정해진 뒤 수행한다. CoreML 실제 가중치/토크나이저와 배포는 구체적인 입력/대상이 필요하다. 이 정보가 없어도 다른 로컬 개발과 검증을 중단할 이유로 사용하지 않는다.
