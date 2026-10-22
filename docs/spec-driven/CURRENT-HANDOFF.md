# 현재 인수인계 — 진행 중 true 알림(Syncing=true)의 구독 세대 경계 / 2026-10-21

새 머신/새 세션에서는 [머신 간 인수인계](./MACHINE-HANDOFF.md)를 먼저 따라 증거 태그·실행 도구·외부 DB/모델·삭제된 임시 경로를 확인한다.

단계94 요청 범위의 관측을 마쳤다. [명세](./operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/SPEC.json) · [결과](./operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/SUMMARY.json) · [검증·시도 이력](./operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/VALIDATION.json) · [자원 정리](./operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/CLEANUP.json). 단계91·92·93 근거는 그대로 보존한다([단계91](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json), [단계92](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json), [단계93](./operational-preparation/source-review-supplement/followups/wiv1-notify-transport-generation-v1/SUMMARY.json)).

## 현재 단계 — 16슬롯·실제32노드, ack 후 같은 세대 사용 0

원래 60초 타이머는 동기화가 시작됐고 `SyncProgress.Done()`이 거짓일 때 진행 중 true를 보낸다. seeder에 1024블록 체인을 주고, receiver가 첫 블록을 넣으면 유일한 peer를 끊었다. receiver는 이미 받은 블록을 넣고 멈췄다(일반 1022, race 384~576). `HighestBlock`은 1024로 남아 타이머가 실제 progress를 보냈다. 단계91 adapter와 단계92 계측 규칙은 바이트 그대로 다시 썼다.

|일정|역할|관측|
|---|---|---|
|progress-retained|ORIGINAL·CAND-A/B|옛 세대가 progress를 받은 뒤 실제 eth_unsubscribe와 owner ack; 세대2와 새 RPC 등록이 다음 progress를 한 번씩 받음, 위반0|
|progress-retained|MUT-STALE-ACK|stale 항목에 세대2가 합쳐져 channel 사용으로는 위반0; install 직전 membership true로만 드러남|
|progress-to-done|ORIGINAL·CAND-A/B|옛 세대 progress 수신과 종료 뒤 peer를 다시 붙여 동기화를 끝냄; 세대2와 새 등록이 done을 한 번씩 받음, 위반0|
|progress-to-done|MUT-EARLY|조기 반환, 세대마다 owner manual uninstall 3회, 위반0|

16슬롯 모두 옛 세대가 진행 중 true를 받았고 `rpcSub.Err()`로 끝났다. 등록할 때 동기화가 끝나지 않았으므로 install send는 0이다. 가짜 ack는 0이고, manual uninstall 6회는 모두 MUT-EARLY 대조군이다. legacy PoW 시험이며 운영 WBFT 검증이 아니다. 트랜잭션 색인이 덜 끝나서 생기는 true와 snap sync는 시험하지 않았다.

시도 실패도 보존했다. smoke1의 trace에서 eth_unsubscribe 기록이 callback 종료보다 뒤에 찍힌 것을 찾았다. 호출이 서버가 구독을 끝낸 뒤에 반환되기 때문이다. 호출 직전에 시작 기록을 추가하고 smoke2로 확인했다. 첫 매트릭스 run-pt1(16슬롯 통과)은 버렸다. 원본 upstream clone을 checkout하지 않아 원본 보존 비교가 빈 목록끼리의 비교였기 때문이다. checkout한 clone으로 run-pt2를 다시 돌렸고, 검증기는 이제 원본 목록이 보존된 고정 트리와 같아야 통과시킨다. 자동 재시도는 0회다. clone·캐시·바이너리 약 2.54GiB를 지웠고 남은 소유 프로세스는 0이다.

## 다음 작업

WI-V1의 세대 경로 관측은 단계94로 요청 범위를 마쳤다. 단계95에서 단계90~94 관측을 12계약별로 다시 묶었다([검토](./operational-preparation/source-review-supplement/followups/wiv1-contract-evidence-v1/REVIEW.json)). 인용 36개는 모두 봉인된 패킷 manifest와 요약값에 결속되고, 새 근거가 있는 계약은 11개, 없는 계약은 LATE-DRAIN 1개다. 충족·독립 승인으로 올린 계약은 0개다. 단계96에서 사용자가 D04를 A안으로 정했다([기록](./operational-preparation/reviewer-assignment-2026-10-22.json)). 단계97에서 gold-reviewer에게 넘길 검토 묶음을 만들었다([안내](./operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-GUIDE.md)). 계약 12개, 매칭 규칙 11개, FINAL 입력 기준의 검토자 칸과 독립성 선언은 비어 있고 AI는 채우지 않는다. 다음은 gold-reviewer가 선언과 결정을 채워 돌려주는 것이다. 승인된 실제 역할 양성 경로와 최종 HEAD도 남았다.

## 남은 전체13묶음 — 상위 완료0/13·정식0/9

|묶음|남은 내용|
|---|---|
|WI-V1|독립 티켓/oracle·승인 양성 역할/final HEAD|
|WI-V2|독립 oracle·수용 후보 집합·선정 판단|
|WI-6|독립 nonseed 관련성/잡음 정답·가족/표본 충분성|
|WI-SP|비보안 수동/자동 출처·policy/미태그 소비자와 독립 의미 검토; 보안 전용 평가는 사용자 보류|
|WI-C|독립 필수 항목/우선순위·새 사례|
|WI-1|값/종류/메모리 매칭·unknown 구분·독립 완전 정답|
|WI-2|독립 비보안 표본·호출 대상/호출자·명령/build/MCP 소비자 매칭|
|WI-3|가족/노출/중복·상한/partial/오류/범위 밖·독립 관련성|
|WI-4|실제 host/receiver 일정·반복·독립 가족/oracle/충분성|
|독립 수용 준비|D04 A안 결정(gold-reviewer·owner-acceptor, WI-V1 시범); 검토자 독립성 선언·새 DEV/FINAL·완전 정답/매칭/충분성·F01~08 동결|
|온톨로지|독립 의미/검색 정답·구현/개념 연결·동일 corpus off/on 효과|
|정식9개 실행·수용|독립 승인·동결 후 WI-1/2/3/4/V1/V2/6/SP/C 실행|
|활성 후속9개|캐시 로컬 완료; 공개 형식·graph tuning·flow gold·규모·as-of·legacy 출처·viewer·배포/downstream 남음|

D04는 A안(역할 분리)으로 정했다. 검토자 gold-reviewer의 독립성 선언 전이므로 F-05는 미승인이고 독립승인0·FINAL0·freeze null·execution_allowed=false다. 실명/연락처를 요구하지 않으며 익명 역할을 허용한다. CoreML은 사용자 제외로 완료 조건에서 뺐다. 보안 전용 평가는 사용자 보류다. 실제 LLM 답변 평가는 모델·인증·비용 상한 결정 후 진행한다.

## 보존 근거

[단계92 transport close/timer](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json) · [단계91 owner ack 후 세대 경계](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json) · [단계90 native/RPC 겹침](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json) · [12계약·소비자12기준 검토](./operational-preparation/source-review-supplement/followups/wiv1-contract-consumer-review-v1/REVIEW.json) · [실제4/통제1 범위 승인](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json) · [전체 원장](./REMAINING-13-EXECUTION.json).
