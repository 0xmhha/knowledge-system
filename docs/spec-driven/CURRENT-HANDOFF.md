# 현재 인수인계

기준: 2026-10-23, main `b62430bd`에 검색 평가 우선 결정을 반영. 이 문서는 지금 단계와 바로 다음 행동만 적는다. 남은 전체 목록, 맥락, 하지 않을 일은 [남은 작업](../REMAINING-WORK.md)에 있고, 정본은 [실행 원장](./REMAINING-13-EXECUTION.json)이다. 새 머신이나 새 세션은 [머신 간 인수인계](./MACHINE-HANDOFF.md)로 환경부터 준비한다.

## 현재 단계

정답이 있는 검색 평가를 시작한다(원장 `current_activity`: `98_retrieval_eval_first_answer_known_cases_WIV1_pilot_paused`).

- 사용자가 2026-10-23에 이 시스템의 핵심 가치를 정했다. vibe query의 의도를 정확히 파악하는 것이 1번, 그 의도의 코드와 연결된 코드를 거의 한 번에 찾아 주는 것이 2번이다. 테스트는 정답이 확인된 사례로만 한다([결정 기록](./operational-preparation/retrieval-priority-2026-10-23/DECISION.json)).
- 첫 대상은 grpc-go, 그다음 go-stablenet이다. WI-V1 정식 수용 시범은 보류했다. 코딩 에이전트 묶음은 검색 품질을 올린 뒤에 한다.
- WI-V1 사례는 정답 없이 골랐던 것으로 확인됐다. 정답 시험을 실제 코드로 돌려 후보 판정이 맞았음을 확인했고, 그 시험을 나중에 WI-V1을 다시 열 때 판정 기준으로 쓴다.
- 13묶음 완료 0/13, 정식 수용 0/9, 독립 승인 0, FINAL 0, freeze 없음, `execution_allowed=false`다.

## 바로 다음 행동

1. [남은 작업](../REMAINING-WORK.md) 3.1의 3단계: 스냅샷 색인 24개를 만들고 CKS와 비교 대상을 돌린다. 입력은 고정됐다([입력](./operational-preparation/retrieval-eval-grpc-go-v1/README.md)).
2. 그다음 4~5단계를 차례로 한다.
3. 단계를 마치면 이 문서, [남은 작업](../REMAINING-WORK.md), 원장을 같은 커밋에서 갱신한다.

## WI-V1 근거 (보류 중, 봉인된 패킷은 고치지 않는다)

|패킷|담은 것|
|---|---|
|[wiv1-reviewer-packet-v1](./operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-PACKET.json)|검토자에게 넘긴 AI 초안과 빈 검토자 칸|
|[wiv1-contract-evidence-v1](./operational-preparation/source-review-supplement/followups/wiv1-contract-evidence-v1/REVIEW.json)|12계약마다 단계90~94 근거 인용 36개와 남은 공백|
|[wiv1-progress-true-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/SUMMARY.json)|동기화 도중 true 알림의 세대 경계|
|[wiv1-notify-transport-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-notify-transport-generation-v1/SUMMARY.json)|Notify 도중 종료와 gate 없는 연결 종료|
|[wiv1-transport-timer-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json)|연결 종료와 같은 channel timer broadcast|
|[wiv1-generation-boundary-v1](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json)|owner ack 뒤 옛 세대 사용과 새 등록 경계|
|[factory-native-overlap-v1](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json)|원래 60초 timer와 RPC callback 겹침|
|[owner-schedule-contract-v1](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json)|사용자가 승인한 실제 4·통제 1 범위|

모든 근거는 AI가 만든 DEV 근거다. 어느 것도 독립 정답이나 독립 승인이 아니다.
