# 남은 전체 작업 / 2026-10-20

[정본 원장](./spec-driven/REMAINING-13-EXECUTION.json) · [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md) · [최신 소스 검증](./spec-driven/operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SUMMARY.json)

사용자 지시 “coreml은 안해도되”에 따라 CoreML을 개발·평가·배포 및 완료 조건에서 제외했다. 모델·토크나이저·배포 대상은 더 이상 요청하지 않는다. 제외는 완료가 아니며 기존 코드와 서명된 검증 기록은 보존한다. 기본 Ollama + BGE-M3를 유지한다. [범위 제외 기록](./spec-driven/operational-preparation/source-review-supplement/followups/user-excluded-coreml-scope-v1/SCOPE.json).

로컬 구현9/9·지식 매니저 로드맵7/7(28/28)과 정식 독립 수용0/9를 구분한다. D04는 A안(역할 분리)으로 정했다. 검토자 gold-reviewer의 독립성 선언 전이므로 F-05는 미승인이고 독립승인0·FINAL0·freeze null·execution_allowed=false다. 실명/연락처를 요구하지 않으며 익명 역할로 검토할 수 있다.

## 현재 검증과 다음 작업

단계91에서 실제 owner ack 이후 옛 구독 세대의 status channel 사용과 명시적 새 등록 세대 경계를 일반/race 16슬롯, 실제32노드로 관측했다([검증 정본](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json)). 단계92에서는 남은 두 경로를 같은 규모로 관측했다. RPC 연결을 닫아 옛 세대를 끝낸 경로와, 같은 channel을 세대2로 다시 등록한 채 자연 60초 done broadcast를 맞는 경로다. 두 경로 모두 ack 후 같은 세대 사용은 0이다. 연결 종료는 8슬롯 모두 callback을 `rpcSub.Err()` 분기로 끝냈고 `notifier.Closed()` 분기는 0회다. delete 없이 ack한 대조군은 channel 사용 집계로는 드러나지 않고, install 직전 membership으로만 2슬롯 모두 드러났다. 가짜 ack는 0이다. [검증 정본](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json)

단계93에서는 Notify 직전에 잡힌 callback의 연결 종료와 타이머 전 gate 없는 연결 종료를 같은 규모로 관측했다. ack 후 같은 세대 사용은 0이다. Notify 도중 종료는 callback 종료가 `rpcSub.Err()` 4회, `notifier.Closed()` 4회로 나뉘어 두 분기를 모두 관측했다. gate 없는 종료는 8회 모두 `rpcSub.Err()`다. [검증 정본](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-notify-transport-generation-v1/SUMMARY.json)

`scripts/` 시험 전체도 복구했다. 과거 단계 시험 110개 모듈은 기록된 증거 커밋의 임시 worktree에서 돌고, 그 뒤 패킷이 바뀌었으면 실패한다. 이전 머신 경로는 도구 이름과 저장소 상대 경로로 비교한다. KS 운영 Go 변경은 0이다. gate 없는 보편 동작과 독립 수용으로 승격하지 않는다.

단계94에서 진행 중 true 알림(Syncing=true)의 세대 경계를 관측했다([결과](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/SUMMARY.json)). 단계95에서 단계90~94 관측을 12계약별로 다시 묶었다([검토](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-contract-evidence-v1/REVIEW.json)). 단계96에서 사용자가 D04를 A안으로 정했다([기록](./spec-driven/operational-preparation/reviewer-assignment-2026-10-22.json)). 다음은 gold-reviewer의 독립성 선언과 단계95 AI 초안을 바탕으로 한 WI-V1 정답·매칭 규칙 확정이다. 승인된 실제 역할 성공 경로와 최종 HEAD 판정도 남았다. [단계90 근거](./spec-driven/operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json)는 보존한다.

## 전체13묶음 — 완료0/13

|묶음|현재 남은 내용|
|---|---|
|WI-V1|최신 CKS 4역할·API 오류·native done/RPC Err overlap·ack 후 세대 경계·transport close/같은 channel timer/Notify 도중 종료/gate 없는 종료/진행 중 true 알림 경로 확인; 승인 oracle/final HEAD·독립 티켓|
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
