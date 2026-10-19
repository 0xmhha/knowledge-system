# 현재 인수인계 — 종료 시한 검사 원인 추적 / 2026-10-20

동일 조건의 실제 시각 계측400회 중2회에서 기존 실패를 재현했다. sleep이 먼저 종료되고 셸이 뒤의 무조건 printf를 실행하면서, 파일이 group kill 호출 중·guardian 반환 전에 생겼다. 기존 검사는 이것도 “종료 후 쓰기”로 보고했다. 두 파일은 반환 시점에 이미 존재했고 stat 오류/반환 후 새 생성은 그 사례들에서 관측되지 않았다.

sleep1;printf를 sleep1&&printf로 한 줄만 바꾼 별도 대조400회는 모두 통과했다. 기존200ms 시한·1초 worker·반환 후1.1초 관찰을 유지했다. 채택한 변경은 시험 파일1개이며 운영 종료 코드는 바꾸지 않았다. 파일 조회 오류와 실제 marker 존재를 별도로 보고한다.

새 검사는 guardian 반환 후에만 writer의 release gate를 연다. owner socket은 계속 열어 절대시한이 종료 원인이 된다. 일반/race 각각3회의 lifecycle 집중 검증은27개씩 통과했다. 실제 후손을 남기는 임시 고장 대조군은 기대한 post-return writer 오류로 실패했고 잔여 프로세스0이다. 처음 대조군의 출력 파이프 대기로 시험 제한에 도달한 시도도 소유 자원을 회수해 보존했으며, 최종 검사는 일반 파일에 진단을 기록한다.

운영 종료 코드의 계측 로그를 제거한 추가400회에서는 실패가 재현되지 않았다. 이 결과도 보존하며, 계측이 드문 실행 순서에 영향을 준다는 제한을 함께 기록한다.

최초 과거1회 실패에는 파일 존재·조회 오류·반환 시각 상세가 없다. 그 사건의 정확한 내부 순서를 사후 확정했다고 주장하지 않는다. 확인한 것은 같은 조건에서 재현한 실패 유형의 원인과 오탐 수정이다. 과거 실패와 모든 진단 시도는 보존하며 모든 환경의 종료 무결함이나 독립 품질 수용을 선언하지 않는다.

새 고정 소스로 기본 전체 일반4273/race4273통과·각18skip·실패0, build/fmt/vet/고정lint/docs 통과다. 입력1815개·Go1305개와 변경된 시험 파일1개를 결속했고 최종 CKS/CKG/CKV는 같은 소스의 고정 후보와 일치한다. 실제 모델/LLM·유료 호출0·운영 적용·게시0이다.

[진단 요약](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SUMMARY.json) · [시도·실패·한계](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/REPORT.txt) · [명세](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SPEC.json) · [13묶음 원장](./REMAINING-13-EXECUTION.json).

CoreML은 사용자 지시로 작업·평가·배포와 완료 조건에서 제외했다. 모델/tokenizer/배포 대상을 요구하지 않고 기존 서명된 소스 검증 이력만 보존한다. 기본 Ollama + BGE-M3와 활성 선택 기능9개를 유지한다. [제외 기록](./operational-preparation/source-review-supplement/followups/user-excluded-coreml-scope-v1/SCOPE.json).

## 다음 작업

WI-V1의 남은 전체12계약과 설치된 소비자 oracle 범위를 우선 검토한다. 기존 owner4/별도 통제 late-drain1 범위 채택과 독립 gold/수용을 구분한다. 익명 검토 역할 미정·새 DEV/FINAL·정답/매칭/충분성·F01~08 동결은 임의로 승인하지 않는다.

## 남은 전체13묶음 — 완료0/13

|묶음|현재 남은 내용|
|---|---|
|WI-V1|실제 host/factory/indexer/owner·peer 근거와 승인된 실제4/통제1 범위를 유지; 전체12계약·설치 oracle·독립 티켓|
|WI-V2|유한 후보 구별 근거 유지; 독립 oracle·수용 후보 집합·선정 판단|
|WI-6|warm 쌍627회 보존; 독립 nonseed 관련성/잡음 정답·표본 충분성|
|WI-SP|비보안 수동/자동 출처·policy/미태그 소비자와 독립 의미 검토; 보안 부분은 사용자 보류|
|WI-C|설치 역할5산출물·렌더·인수인계 근거 유지; 독립 필수 항목/우선순위|
|WI-1|선택 수신16수정, range37/빈 블록3 미확정 구분; 값/종류/메모리 매칭·독립 완전 정답|
|WI-2|독립 비보안 표본·호출 대상/호출자·명령/build/MCP 소비자 매칭|
|WI-3|시작 배치66→2와 계측 기능16조합 보존; 가족/노출/중복·기본 상한/partial/오류/범위 밖·독립 관련성|
|WI-4|실제 host·receiver 일정·반복 기준·독립 가족/oracle/충분성|
|독립 수용 준비|익명 검토 역할 미정; 새 DEV/FINAL·완전 정답/매칭/충분성·F01~08 동결|
|온톨로지|독립 의미/검색 정답·구현/개념 연결·같은 corpus off/on 효과 판단|
|정식9개 실행·수용|독립 승인·동결 후 WI-1/2/3/4/V1/V2/6/SP/C 실행; 현재0/9|
|활성 후속9개|캐시 로컬 완료. 공개 형식 후속·graph tuning·flow gold·규모 수용·as-of·legacy 출처·viewer Atlas/접근성·배포/downstream 남음. CoreML은 사용자 제외이며 완료 조건이 아님|

D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false·정식0/9를 유지한다. 실명/연락처를 요구하지 않으며 익명 역할로 검토할 수 있다. 보안 전용 gold/ACL/비공개/위조와 감사 기능 평가는 사용자 보류다. 실제 LLM 답변 평가는 모델·인증·총 비용 상한 결정 후 진행한다.

## 보존한 앞선 근거

- [단계86 고정 소스 검증](./operational-preparation/source-review-supplement/followups/coreml-transport-v1/SUMMARY.json): 최초 race 실패와 원본400회 이전 진단20/80회, BGE 기존/새10hits 일치, 당시4272개 기본 회귀. 제외된 기능의 과거 모델 미수용도 보존한다.
- [단계85 공개 형식](./operational-preparation/source-review-supplement/followups/public-v2-surface-v1/SUMMARY.json): 실제 CLI/HTTP 인용11개·hash 일치와 원시 그래프244989/1964193 전 행 대조, 취소/출력 정리 근거.
- [단계84 규모](./operational-preparation/source-review-supplement/followups/scale-long-functions-v1/SUMMARY.json): retained1.242GB/긴 함수3개·21요청의 partial/budget/복구 근거이며 용량 SLA·의미 품질 수용과 구분한다.
- [단계81 owner 범위 채택](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json): 실제4/통제1, 옛 구독·새 등록 전 범위이며 전체12/독립 gold 수용은 아니다.
- [단계80 viewer](./operational-preparation/source-review-supplement/followups/viewer-lifecycle-v1/SUMMARY.json): 당시 built UI 상호작용·소유 backend 종료이며 Atlas·전체 접근성은 미검증이다.
