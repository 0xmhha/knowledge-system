# 현재 인수인계 — transport close와 같은 channel timer broadcast의 구독 세대 경계 / 2026-10-20

새 머신/새 세션에서는 [머신 간 인수인계](./MACHINE-HANDOFF.md)를 먼저 따라 증거 태그·실행 도구·외부 DB/모델·삭제된 임시 경로를 확인한다.

단계92 요청 범위의 관측을 마쳤다. [명세](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SPEC.json) · [결과](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json) · [검증·시도 이력](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/VALIDATION.json) · [자원 정리](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/CLEANUP.json). 단계91 근거는 [그대로 보존한다](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json).

## scripts 시험 전체 복구

`scripts/` 시험 전체(CI 밖)를 처음 돌렸을 때 2825개 중 581개가 실패했다. 원인은 셋이었다.

- 과거 단계 검증기가 그 단계의 소스·도구·정식 상태를 지금 트리와 비교했다. 이후 단계가 그 입력을 일부러 바꿨으므로 증거 결함이 아니라 drift다.
- jsonschema와 PyYAML이 설치되지 않았다. 패킷은 PyYAML 6.0을 parser 버전으로 기록했다.
- 이전 머신의 Go SDK 경로와 저장소 절대 경로를 그대로 비교하거나 실행했다.

과거 단계 시험 110개 모듈은 이제 [기록된 커밋](../../scripts/evidence-revisions.json)의 임시 worktree에서 돈다. 그 커밋 이후 패킷 파일이 수정되거나 지워졌으면 실패한다. 소스 커밋과 패킷 커밋이 달랐던 경우는 바인딩된 파일만 기록된 소스 커밋에서 되살려 검증 당시 트리를 다시 만든다. 이전 머신 경로는 도구 이름과 저장소 상대 경로로 바꾸고 기록된 해시는 그대로 비교한다. 원래 실행의 임시 DB를 다시 읽는 확인 1건만 건너뛴다. DB가 지워져 재현할 수 없기 때문이다. 준비 절차는 [머신 인수인계 2절](./MACHINE-HANDOFF.md)에 있다.

## 현재 단계 — 16슬롯·실제32노드, ack 후 같은 세대 사용 0

trace 규칙은 단계91과 같다. (factory API, status channel) 키마다 세대를 세고, 실제 owner install이 세대를 올린다. 이번에는 규칙 하나를 더했다. owner가 install 직전의 map membership을 기록한다. 같은 channel을 재등록하면 stale 항목과 재등록이 한 키로 합쳐지기 때문이다.

|일정|역할|관측|
|---|---|---|
|transport(단계90 gate)|CAND-A/B|연결 종료 뒤 drain·실제 owner 삭제·ack, 같은 channel 세대2(install 송신)와 새 연결의 새 RPC 등록 전달, 위반0|
|transport|ORIGINAL|쌍 대기 후 소유 status 수신1로 복구, 이후 재생성, 위반0|
|transport|MUT-EARLY|조기 반환, 세대마다 실제 owner manual uninstall 3회, 위반0|
|timer-same-channel|ORIGINAL·CAND-A/B|타이머 전 옛 세대 ack, 세대2와 새 등록을 둔 채 자연 done broadcast가 둘 다 한 번씩 송신, 위반0|
|timer-same-channel|MUT-STALE-ACK|delete 없이 ack한 항목에 세대2가 합쳐져 broadcast는 세대2로 집계, 위반0; install 직전 membership true로만 드러남|

transport close는 8슬롯 모두 callback을 `rpcSub.Err()` 분기로 끝냈다. 이 커밋의 RPC 서버는 연결이 닫히면 구독 오류 채널을 먼저 닫고 그다음 연결 종료 채널을 닫는다. 그래서 `notifier.Closed()` 분기는 0회다. 분기를 억지로 고르지 않았고, 이 분기는 두 채널이 동시에 준비될 때만 탈 수 있다. timer 경로의 stale-ack 대조군은 channel 사용 집계로는 검출되지 않는다. membership으로만 드러나며 2슬롯 모두 드러났다. 소유 status 수신은 4회, manual uninstall은 6회이며 가짜 ack는 0이다. legacy PoW 시험이며 운영 WBFT 검증이 아니다. 실제 owner4/별도 통제 late1 범위는 바꾸지 않았고 독립 표본으로 합치지 않는다.

시도 실패도 보존했다. 첫 보관본은 race 빌드 로그의 macOS 링커 경고에 Go 임시 링크 폴더가 남아 검증기가 거부했다. 보관 규칙에 자리표시자 변환을 넣어 같은 실행 증거로 다시 보관했다. 컴파일 점검, smoke 2슬롯, 매트릭스 16슬롯은 첫 시도에 통과했고 자동 재시도는 0회다. 원본 checkout은 보존됐다. clone·캐시·바이너리 약 2.87GiB는 지웠고 남은 소유 프로세스는 0이다. KS 운영 Go 변경, 실제 LLM, 유료 호출, 운영 적용, 게시는 모두 0이다.

## 다음 작업

WI-V1에서는 남은 세대 경로가 셋이다. 진행 중 true 알림(Syncing=true), Notify 도중 종료, gate 없는 transport close다. 독립 ticket/oracle 준비, 승인된 실제 역할 양성 경로와 최종 HEAD도 남았다. 이후 독립 정답·충분성·동결이 필요하다.

## 남은 전체13묶음 — 상위 완료0/13·정식0/9

|묶음|남은 내용|
|---|---|
|WI-V1|진행 중 true 알림·Notify 도중 종료·gate 없는 transport close 세대 경로·독립 티켓/oracle·승인 양성 역할/final HEAD|
|WI-V2|독립 oracle·수용 후보 집합·선정 판단|
|WI-6|독립 nonseed 관련성/잡음 정답·가족/표본 충분성|
|WI-SP|비보안 수동/자동 출처·policy/미태그 소비자와 독립 의미 검토; 보안 전용 평가는 사용자 보류|
|WI-C|독립 필수 항목/우선순위·새 사례|
|WI-1|값/종류/메모리 매칭·unknown 구분·독립 완전 정답|
|WI-2|독립 비보안 표본·호출 대상/호출자·명령/build/MCP 소비자 매칭|
|WI-3|가족/노출/중복·상한/partial/오류/범위 밖·독립 관련성|
|WI-4|실제 host/receiver 일정·반복·독립 가족/oracle/충분성|
|독립 수용 준비|익명 역할 미정·새 DEV/FINAL·완전 정답/매칭/충분성·F01~08 동결|
|온톨로지|독립 의미/검색 정답·구현/개념 연결·동일 corpus off/on 효과|
|정식9개 실행·수용|독립 승인·동결 후 WI-1/2/3/4/V1/V2/6/SP/C 실행|
|활성 후속9개|캐시 로컬 완료; 공개 형식·graph tuning·flow gold·규모·as-of·legacy 출처·viewer·배포/downstream 남음|

D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false다. 실명/연락처를 요구하지 않으며 익명 역할을 허용한다. CoreML은 사용자 제외로 완료 조건에서 뺐다. 보안 전용 평가는 사용자 보류다. 실제 LLM 답변 평가는 모델·인증·비용 상한 결정 후 진행한다.

## 보존 근거

[단계91 owner ack 후 세대 경계](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json) · [단계90 native/RPC 겹침](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json) · [12계약·소비자12기준 검토](./operational-preparation/source-review-supplement/followups/wiv1-contract-consumer-review-v1/REVIEW.json) · [실제4/통제1 범위 승인](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json) · [전체 원장](./REMAINING-13-EXECUTION.json).
