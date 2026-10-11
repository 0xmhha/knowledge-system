# N11 후보 책 수정 반영과 기권 인용 규칙

2026-10-09 · **사용자 채택 기록·개정 제안 책/protocol 초안·입력 검사 규칙 보강 / 승인0·N11 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 배경

[N11 사전검토](./N11-CANDIDATE-PREREVIEW.md) 권고를 사용자가 채택했고, 공유 사실은 DEV 쪽을 교체하기로 했다. corpus에서는 평가 준비 문서를 빼고, ABS는 "미지정" 기록을 인용하며 기권하는 것을 허용하기로 했다([결정 기록](../../system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09/decision.json)). 검토자 이름은 없다. 그래서 결과는 승인된 gold가 아니라 개정 **제안**이다. 원래 책과 protocol 초안은 그대로다.

## 반영한 내용

[개정 책](../../system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09/candidate-question-book.json)과 [protocol 초안](../../system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09/candidate-protocol.json)을 [생성 스크립트](../../system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09/build-revision.py)로 만들었다.

- 증거 범위: DEV-01/02/03/05와 FINAL-07/14를 넓히거나 보강했다. span SHA는 source `479491ba`에서 다시 계산했다. 계산 방식이 기존에 기록된 모든 digest를 그대로 재현하는지 먼저 확인했다.
- 답 수정: DEV-03(호출자 지침), DEV-04(보호 사유 세 가지 분리), FINAL-08(조건 범위).
- 철회: FINAL-04/13/15(근거 없는 WHY), DEV-06/07(FINAL과 사실 공유, 대체 DEV 필요).
- ABS: FINAL-09/10은 `strict_zero_citations=false`로 두고 기권 시 인용 규칙과 근거 원문을 둔다. protocol 초안에 `abstention_scoring`과 corpus 제외 경로(`docs/spec-driven/N11-*`, `N12-*`, `system/eval/**`)를 넣었다.

결과는 DEV5·FINAL10문항이고, FINAL 군은 CODE4·WHY1·POLICY1·TRACE2·ABS1이다. 모두 10개 미만이라 [표본 부족 규칙](./N11-SHORTFALL-RULE.md)에 따라 inconclusive가 된다.

## 입력 검사 규칙 보강

기존 입력 검사는 ABS 기권에 `strict_zero_citations=true`만 허용했다. 사용자 규칙을 쓰려면 검사가 바뀌어야 했다. 먼저 실패하는 시험을 넣었고(RED), 검사를 고쳤다. 이제 protocol에 `abstention_scoring`이 있고, 문항에 인용 규칙과 근거 원문이 있을 때만 인용 허용 기권을 받는다. 셋 중 하나라도 없으면 계속 거부하며, strict=false만 있는 기존 음성 사례도 그대로 거부된다. 입력 검사 시험17·runner 시험9 통과([원자료](../../system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09/script-tests.txt)).

개정 초안에 실제 검사(`--require-ready`)를 돌린 결과 exit2(실행 거부)다. source/gold 바이트 오류는0이며, 남은 사유는 사람 판정·군 판정·동결 누락이다. 기권 사유 `abstention_scope_missing`은 사라졌다.

한계: 검사는 입력 형식만 확인한다. 실제 평가에서 "미지정 기록 인용 기권"을 정답으로 채점하는 부분은 아직 없으며 N09 평가 단계에서 구현해야 한다. 철회 상태(`withdrawn_proposed`)는 검사가 따로 인식하지 않는다. 철회 문항은 동결 전에 책에서 빼야 한다.

## 남은 조건

- [x] 채택·corpus·ABS 규칙 기록, 개정 책/protocol 초안, 입력 검사 보강과 시험.
- [ ] 검토자 이름과 문항별 gold·군·독립성·오염 승인, 대체 DEV2, 동결(N11-A–D).

**남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
