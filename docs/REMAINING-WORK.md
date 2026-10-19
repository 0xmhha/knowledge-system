# 남은 전체 작업 / 2026-10-20

[정본 원장](./spec-driven/REMAINING-13-EXECUTION.json) · [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md) · [최신 소스 검증](./spec-driven/operational-preparation/source-review-supplement/followups/coreml-transport-v1/SUMMARY.json)

사용자 지시 “coreml은 안해도되”에 따라 CoreML을 개발·평가·배포 및 완료 조건에서 제외했다. 모델·토크나이저·배포 대상은 더 이상 요청하지 않는다. 제외는 완료가 아니며 기존 코드와 서명된 검증 기록은 보존한다. 기본 Ollama + BGE-M3를 유지한다. [범위 제외 기록](./spec-driven/operational-preparation/source-review-supplement/followups/user-excluded-coreml-scope-v1/SCOPE.json).

로컬 구현9/9·지식 매니저 로드맵7/7(28/28)과 정식 독립 수용0/9를 구분한다. D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false를 유지한다. 실명/연락처를 요구하지 않으며 익명 역할로 검토할 수 있다.

## 현재 검증과 다음 작업

최신 고정 소스의 기본 일반/race는 각각4272통과·18skip, build/fmt/vet/고정lint/docs 통과다. 기존 BGE 반복/새 버전의 검색10개는 출처·본문·순서·점수까지 일치했다. 최초 전체race의 guardian 종료 시한 검사1회 실패는 후속 통과에도 원인 미확정으로 유지한다.

다음 우선순위는 이 종료 시한 실패의 원인 추적이다. 실제 peer 동기화·리뷰5산출물 전달·viewer 소유 backend/UI·공개 CLI/HTTP 및 원시 그래프 v2의 앞선 로컬 검증은 보존한다. 독립 정답·품질 수용은 별도다.

## 전체13묶음 — 완료0/13

|묶음|현재 남은 내용|
|---|---|
|WI-V1|실제 host/factory/indexer/owner 확인; 실제 peer 일반/race 확인; 전체 조건·installed oracle·독립 티켓|
|WI-V2|유한 후보 구별 완료; 독립 oracle·수용 후보 집합·선정 판단|
|WI-6|warm 쌍627회 보존; 독립 nonseed 관련성/잡음 정답·표본 충분성|
|WI-SP|비보안 수동/자동 출처·policy/미태그 소비자와 독립 의미 검토; 보안 부분은 사용자 보류|
|WI-C|설치 역할5산출물·렌더·인수인계 확인; 독립 필수 항목/우선순위 검토|
|WI-1|선택 수신16수정; range37/빈 블록3 미확정 구분; 값/종류/메모리 매칭·독립 완전 정답|
|WI-2|독립 비보안 표본·호출 대상/호출자·명령/build/MCP 소비자 매칭|
|WI-3|시작 배치·계측 기능 보존 검증 완료; 가족/노출/중복·기본 상한/partial/오류/범위 밖·독립 관련성|
|WI-4|실제 host·receiver 일정·반복 기준·독립 가족/oracle/충분성; 실제 peer 일반/race 근거 확인|
|독립 수용 준비|익명 검토 역할 미정; 새 DEV/FINAL·완전 정답/매칭/충분성·F01~08 동결|
|온톨로지|독립 의미/검색 정답·구현/개념 연결·동일 corpus off/on 효과 판단|
|정식9개 실행·수용|독립 승인·동결 후 WI-1/2/3/4/V1/V2/6/SP/C 실행; 현재0/9|
|활성 선택 기능9개|캐시 로컬 완료. 공개 형식·graph tuning·flow gold·규모·as-of·legacy 출처·viewer·배포 남음. CoreML 사용자 제외, 보안 감사 사용자 보류|

활성 선택 기능9개 중 캐시는 로컬 완료이며 나머지8개는 각 기준과 입력이 남아 있다. CoreML은 제외하고 보안 전용 평가·조직 인증·보안 감사는 사용자 보류로 유지한다. 실제 LLM 답변 평가는 모델·인증·비용 상한 결정 후 진행한다. 각 작업은 검증·정본 문서 갱신·서명 커밋·저장 내용 대조 후 체크하며 독립 수용을 로컬 시험으로 대신하지 않는다.
