# 남은 작업

기준: 2026-10-22, main `99214d3f`. 정본은 [실행 원장](./spec-driven/REMAINING-13-EXECUTION.json)이다. 이 문서는 원장을 사람이 읽고 바로 이어서 일할 수 있게 풀어 쓴 것이다.

## 0. 이 문서를 다루는 규칙

- 현재 상태와 남은 일만 적는다. 끝난 단계의 경과는 쓰지 않고, 근거 패킷 링크로만 남긴다.
- 원장과 이 문서가 다르면 원장이 맞다. 이 문서를 원장에 맞춰 고친다.
- 단계를 하나 마칠 때마다 이 문서, [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md), 원장을 같은 커밋에서 갱신하고 위 기준 날짜와 커밋을 바꾼다.
- 새 세션은 이 문서의 3장 순서대로 일한다. 3장에 없는 방향으로 가려면 먼저 사용자에게 묻는다.

## 1. 맥락

이 저장소는 코드 그래프(CKG), 벡터 검색(CKV), 둘을 묶어 에이전트에 제공하는 CKS로 이루어진 지식 시스템이다. 로컬 구현 9/9와 지식 매니저 로드맵 7/7(세부 28/28)은 끝났다.

지금 목표는 남은 13묶음을 거짓 완료 없이 끝내는 것이다. 13묶음 가운데 9개(WI-1, WI-2, WI-3, WI-4, WI-V1, WI-V2, WI-6, WI-SP, WI-C)는 정식 수용으로만 완료된다.

정식 수용은 AI 혼자 끝낼 수 없다. 정답을 만든 쪽과 결과를 낸 쪽이 같으면 같은 착오가 양쪽에 들어가도 걸러지지 않기 때문이다. 그래서 AI가 만든 정답 초안과 실행 결과는 DEV 근거로만 쓴다. 정답과 매칭 규칙은 사람 검토자가 확정하고, 결과를 받아들일지는 최종 승인자가 정한다.

역할은 사용자가 2026-10-22에 D04 A안(역할 분리)으로 정했다([기록](./spec-driven/operational-preparation/reviewer-assignment-2026-10-22.json)). 실명과 연락처는 쓰지 않고 익명 역할 ID만 쓴다.

|역할|ID|하는 일|
|---|---|---|
|독립 내용 검토자, 최종 정답 작성자|gold-reviewer|AI 초안을 보고 정답과 매칭 규칙을 확정한다|
|최종 범위 승인자|owner-acceptor (사용자)|F-05 승인, 입력 동결, 결과 수용, 시범 확대를 정한다|
|실행 담당|AI, owner-acceptor 확인|동결된 입력으로 정식 실행을 하고 로그와 해시를 남긴다|
|초안 보조|AI|초안만 쓴다. 검토자 칸과 독립성 선언은 채우지 않는다|

정식 수용은 WI-V1 한 묶음 시범부터 한다. 나머지 8개로 넓힐지는 시범 결과를 보고 owner-acceptor가 정한다.

## 2. 현재 상태

- 13묶음 완료 0/13, 정식 수용 0/9, 독립 승인 0, FINAL 0
- 동결(freeze) 없음, 정식 실행 불가(`execution_allowed=false`)
- F-01~F-08 동결 결정 모두 대기. F-05는 역할만 정했고 검토자의 독립성 선언을 기다린다.
- 프로토콜의 남은 구조 조건 97개(`scripts/check-operational-acceptance.py`)

## 3. 지금 할 일 (이 순서를 따른다)

### 3.1 WI-V1 시범: gold-reviewer 응답 대기 (최우선)

검토 자료는 [followups/wiv1-reviewer-packet-v1](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-GUIDE.md)에 있다. 계약 12개, 매칭 규칙 11개, FINAL 입력 기준의 검토자 칸과 독립성 선언이 비어 있다. 이 칸은 gold-reviewer만 채운다.

검토자가 채운 파일을 돌려주면 아래 순서로 진행한다.

1. 돌려받은 파일을 새 패킷으로 저장한다. 기존 `wiv1-reviewer-packet-v1`은 고치지 않는다.
2. AI 초안 칸과 원본 해시가 바뀌지 않았는지 확인한다. `scripts/wiv1_reviewer_packet.py`의 `build_review()`가 기준이다.
3. 독립성 선언의 필수 항목이 모두 있는지 확인하고, 검토자 결정을 원장에 연결하는 검증기와 변조 시험을 만든다.
4. owner-acceptor에게 F-05 승인과 WI-V1 입력 동결을 요청한다. 승인 문장은 사용자 메시지 그대로 기록한다.
5. 동결된 입력으로 WI-V1 정식 실행을 하고, owner-acceptor가 결과를 받아들이면 정식 수용을 1/9로 센다.

검토자가 `defer`나 `reject`를 고른 계약은 정답에서 빠진다. 그 계약을 AI가 대신 채우지 않는다.

### 3.2 기다리는 동안 AI가 할 수 있는 준비

모두 DEV 근거일 뿐이고 완료로 세지 않는다. 사용자가 고르지 않으면 1번부터 한다.

1. **WI-V2 전체 비교표 실행**: 원장 WI-V2의 첫 완료 조건이다. 실제 5역할과 메서드 변형 4개의 구별은 끝났고, 미리 선언한 비교표 전체 범위는 아직 실행하지 않았다. 실제 노드를 많이 띄우므로 전체 `scripts` 시험과 동시에 돌리지 않는다.
2. **선택 기능의 완료 기준 제안**: 6장 선택 기능 가운데 기준이 없는 항목은 기준부터 제안하고 사용자 승인을 받는다.

WI-1의 남은 53곳 분류는 AI 초안으로 이미 끝났다. range 37곳과 빈 블록 3개는 unknown으로 남긴다. 다시 분류하지 않는다.

### 3.3 사람이 정해야 하는 것

|항목|누가|상태|
|---|---|---|
|WI-V1 독립성 선언과 검토 결정|gold-reviewer|대기|
|F-05 승인, WI-V1 입력 동결, 결과 수용|owner-acceptor|검토 결정 뒤|
|시범을 나머지 8개 묶음으로 넓힐지|owner-acceptor|WI-V1 결과 뒤|
|실제 LLM 답변 평가의 모델, 인증, 비용 상한|사용자|미정. 정하기 전에는 유료 모델을 호출하지 않는다|
|보안 전용 평가, 조직 인증, 보안 감사|사용자|보류|

## 4. 하지 않는 것

- AI 초안, AI 실행 결과, 같은 AI의 재검토를 독립 정답이나 독립 승인으로 세지 않는다.
- 검토자 칸, 독립성 선언, 승인 receipt를 AI가 채우거나 추정해서 쓰지 않는다. 역할 ID도 AI가 새로 붙이지 않는다.
- 사용자 범위 승인을 정답 승인으로 바꿔 쓰지 않는다. 이미 노출된 DEV 입력을 FINAL로 쓰지 않는다.
- owner-acceptor 결정 없이 시범을 WI-V1 밖으로 넓히지 않는다.
- 봉인된 패킷(`followups/*`의 manifest에 묶인 파일)을 고치거나 다시 봉인하지 않는다. 바꿀 일이 생기면 새 패킷을 만든다.
- 원장의 과거 결정을 바꾸지 않는다. 현재 준비 근거 검사(`scripts/guardian_deadline.py`)는 기준 커밋과 다른 결정을 거부한다. 예외는 D04 역할 기록뿐이고, `scripts/d04_role_assignment.py` 검사를 통과할 때만 허용된다.
- CoreML은 사용자가 제외했다. 다시 요청하지 않고 완료 조건에도 넣지 않는다.
- 보안 전용 범위는 사용자가 보류했다. 완료로 세지 않는다.
- 실명과 연락처를 요구하거나 기록하지 않는다.
- CI에서 race 시험을 돌리지 않는다. race 시험은 PR 전에 로컬에서 돌린다.

## 5. 13묶음별 남은 일

완료 조건 원문은 원장의 `rows[].done_when`이다.

|묶음|남은 일|막힌 곳|
|---|---|---|
|WI-V1|12계약의 정답·매칭 확정, 승인된 실제 역할의 양성 oracle과 최종 HEAD, 독립 티켓과 반복 충분성|gold-reviewer 응답 (3.1)|
|WI-V2|선언한 비교표 전체 실행과 설치된 oracle, 독립 oracle과 수용 후보 집합, 선정 또는 결론 보류 판단|비교표 실행은 AI 가능 (3.2), 나머지는 검토자|
|WI-6|독립 nonseed 완전 정답·가족·표본 충분성, 설치된 소비자와 품질·효과 판단. warm 627쌍(warmup 57, measured 570)은 기록됨|검토자|
|WI-SP|실제 policy·미태그 소비자 호출과 수동·자동 출처, 출처가 없는 것은 unknown 유지, 비보안 범위 독립 의미 검토|검토자. 보안 부분은 사용자 보류|
|WI-C|설치된 검토 역할이 구조화 보고서·다이어그램·인수인계를 만드는지, 출처와 추론 구분, 독립 의미 검토|검토자|
|WI-1|독립된 값·종류·메모리 의미 검토, 설치된 소비자 검토, 독립 완전 정답. 53곳 AI 분류와 16곳 정의 보정은 끝남, range 37곳·빈 블록 3개는 unknown 유지|검토자|
|WI-2|명령·호출자·build·MCP 지원 경계 측정, 독립 비보안 표본과 정답 매칭|검토자. 보안 전용은 보류|
|WI-3|운영 입력의 가족·노출·중복과 실패 분모, 설치 역할과 기본 예산·시작·partial·provider·범위 밖 경로, 독립 관련성 검토|검토자|
|WI-4|원래 driver·host·receiver 일정 계약 전체와 반복 실행, 선언한 정상·RED·방어·관측·재발과 정리, 독립 가족·oracle·충분성|검토자|
|독립 수용 준비|검토자 독립성 선언, 새 DEV/FINAL, 완전 정답·매칭·충분성 검토, F-01~F-08 동결 receipt|gold-reviewer, owner-acceptor|
|온톨로지|검토된 구현·개념 연결과 독립 검색 정답, 같은 corpus에서 켜고 끈 효과·오류·unknown 보고, 실제 LLM 답변 평가|검토자. LLM은 사용자 결정|
|정식 9개 실행·수용|독립 승인된 동결 입력, 정식 실행 9회와 서명된 수용 receipt|위 모든 항목|
|선택 기능|6장|항목별 기준과 입력|

## 6. 선택 기능 (별도 대기열)

|항목|상태|
|---|---|
|임베딩 캐시|로컬 구현과 실제 검증 완료|
|공개 CLI/HTTP 형식(PUBLIC-V2)|로컬 CKS CLI·HTTP·raw graph 검증. 확장 남음|
|viewer|로컬 UI와 소유 수명 관리 검증. 선언한 한계 있음|
|대규모 corpus(SCALE)|큰 corpus·긴 함수 측정. 용량 수용 아님|
|graph 점수 조정, flow 정답, as-of, legacy 출처, 배포|미완료. 각각 기준과 입력이 필요|
|CoreML|사용자 제외 (완료 아님)|
|보안 감사(AUDIT)|사용자 보류|

## 7. 작업 절차

- 환경 준비와 전체 시험 명령은 [머신 간 인수인계](./spec-driven/MACHINE-HANDOFF.md) 2장을 따른다. 전체 `scripts` 시험 전에 `make build-bins`로 `bin/ckg`, `bin/ckv`, `bin/cks`를 만든다. `make build`는 실행 파일을 만들지 않는다.
- Python은 `scripts/requirements.txt`를 설치한 venv를 쓴다. PATH의 `go`는 go.mod가 요구하는 1.25.13 이상이어야 한다.
- 전체 `scripts` 시험은 저장소 파일을 읽으므로 한 번에 하나만 돌린다. 다른 작업은 별도 worktree에서 한다.
- Go 코드를 바꾸면 PR 전에 `make test`, `make test-race`, `make fmt-check vet golangci boundaries`를 로컬에서 통과시킨다.
- PR은 전체 `scripts` 시험이 통과한 뒤에만 올린다. CI가 통과한 PR은 스쿼시 머지하고 `main`을 맞춘다.
- 커밋 전에 `betterleaks git . --pre-commit --staged`를 돌린다. 커밋과 PR은 영어 Conventional Commits로 쓰고, 내부 용어를 넣지 않는다.
- 원장 JSON은 들여쓰기 2칸, `ensure_ascii=False`로 쓴다. `REMAINING-13-EXECUTION.json`을 바꾸면 그 해시를 DECISION-PROPOSAL, PROGRESS, PROTOCOL, GAPS의 `active_remaining_worklist`에 반영한다. 그다음 DECISION-PROPOSAL의 해시를 PROGRESS, PROTOCOL, GAPS의 `adoption_acceptance_decision_proposal`에 반영한다.
- 매 단계 끝에 검증 결과, 현재 단계, 다음 작업, 남은 전체 목록을 사용자에게 보고한다.

## 8. 정본과 근거

- [실행 원장](./spec-driven/REMAINING-13-EXECUTION.json), [운영 수용 프로토콜](./spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json), [결정 제안 D01~D08](./spec-driven/OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json)
- [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md), [머신 간 인수인계](./spec-driven/MACHINE-HANDOFF.md)
- [D04 역할 기록](./spec-driven/operational-preparation/reviewer-assignment-2026-10-22.json), [WI-V1 검토 자료](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-GUIDE.md), [WI-V1 12계약 근거](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-contract-evidence-v1/REVIEW.json)
- [CoreML 제외 기록](./spec-driven/operational-preparation/source-review-supplement/followups/user-excluded-coreml-scope-v1/SCOPE.json)
