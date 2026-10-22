# WI-V1 정답 검토 안내 (gold-reviewer)

이 묶음은 D04에서 독립 내용 검토자로 정한 gold-reviewer가 WI-V1 정답과 매칭 규칙을 확정하기 위한 자료다. 사용자가 2026-10-22에 A안(역할 분리)과 WI-V1 시범을 정했다([기록](../../../reviewer-assignment-2026-10-22.json)).

자료의 모든 `ai_draft_*` 값은 AI가 썼다. 검토자 칸은 모두 비어 있고, AI는 이 칸을 채우지 않는다.

## 1. 먼저 할 일: 독립성 선언

`INDEPENDENCE-DECLARATION.json`의 `declaration` 칸을 검토자가 직접 채운다.

- `reviewer_id`: `gold-reviewer`
- `not_author_of_candidate_outputs`: 후보 결과(수정 후보와 실행 결과)를 만들지 않았으면 `true`
- `candidate_output_exposure_before_gold_finalization`: 정답을 정하기 전에 본 후보 결과. `REVIEW-PACKET.json`의 `exposure_notice.candidate_outputs`에 이 자료가 인용하는 패킷이 적혀 있다
- `ai_draft_exposure`: AI 초안을 보고 정하는지 여부
- `conflicts`: 이해 충돌. 없으면 `"none"`
- `declared_date`: 선언한 날짜

실명과 연락처는 적지 않는다.

## 2. 계약 12개 검토

`REVIEW-PACKET.json`의 `contracts`에 계약마다 AI 초안 주장과 미확인 사항, 단계95 근거와 남은 공백이 있다. 검토자는 `reviewer` 칸을 채운다.

- `decision`: `accept`(그대로 수용), `revise`(고쳐서 수용), `reject`(정답에서 제외), `defer`(이번 시범에서 보류)
- `revise`이면 `revised_assertion`과 `revised_unknown`에 고친 문장을 적는다
- `primary_oracle`: 이 계약을 판정할 1차 판정 기준
- `derived_oracle`: 1차와 독립적으로 도출한 2차 판정 기준. AI의 대기 그래프 초안은 같은 출처라 독립 도출로 인정되지 않는다
- `notes`: 근거나 의견

## 3. 매칭 규칙 11개 검토

`matching_rules`에 매칭 계약의 AI 초안이 있다. 같은 방식으로 `decision`, `revised_rule`, `notes`를 채운다. V2 규칙은 WI-V2 범위라 이번 시범에서 뺐다.

## 4. FINAL 입력 기준

`final_inputs.requirements_proposal`은 프로토콜의 WI-V1 제안(독립 결함 티켓 2개 이상, 정상 대조군, 증상 재도입 변이, 과적합 변이, 두 가지 oracle)이다. 기존 DEV 티켓은 이미 노출되어 FINAL로 쓸 수 없다. 검토자는 `reviewer` 칸에 새 FINAL 티켓, 대조군, 변이, 반복 충분성 기준을 적는다.

## 5. 돌려받은 뒤

AI는 돌려받은 파일을 이 묶음과 비교해 AI 초안 칸이 바뀌지 않았는지 확인하고, 검토자 결정을 원장에 결속한다. 그 뒤 F-05 승인과 WI-V1 입력 동결을 owner-acceptor에게 요청한다. 정식 수용은 owner-acceptor가 결과를 승인해야 1/9로 센다.
