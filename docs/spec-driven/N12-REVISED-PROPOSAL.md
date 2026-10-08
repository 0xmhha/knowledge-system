# N12 사전검토 수정의 반영(검토 대기 제안)

2026-10-09 · **사용자 채택 기록·수정 10건 반영한 제안 작성 / verified0·N12 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 배경

[사전검토](./N12-CRITERION-GAP-PREREVIEW.md) 권고(accept 계열25·modify10·reject0)를 확인하는 질문에 사용자는 “남은 작업별 수정을 진행해줘”라고 답했다. 이를 35건 권고 채택과 수정 10건 반영 요청으로 해석했다(확신도 Mid, [결정 기록](../../system/eval/b0-knowledge-system/refactoring-n12-revised-proposal-2026-10-09/decision.json)). 검토자 이름은 아직 없다. 제품은 개념·관계를 verified로 바꿀 때 `ReviewedBy`를 요구하므로, 이번 반영은 모두 **proposed** 상태의 개정 제안이다. 기존 held 후보와 worksheet는 바꾸지 않았다.

## 반영한 내용

[개정 제안](../../system/eval/b0-knowledge-system/refactoring-n12-revised-proposal-2026-10-09/revised-proposal.json)은 원래 17관계에 대한 조작17개와 새 관계14개를 담는다.

- F3-T08은 구현 대상을 `ValidateCodeAnchors`로, F3-T11은 시험 대상을 `ValidateCodeAnchors`로 바꿨다. 실제 거부를 맡는 `PutAligned`/`PutAlignedRetained`/`ActivateAligned`/`ActivateAlignedRetained`를 IMPLEMENTED_BY(F3-R05–08)로, 그중 두 경로를 통합 시험의 TESTED_BY(F3-R09–10)로 추가했다. 거부 gate 구분은 [gate 타입 구분](./N12-GATE-ATTRIBUTION.md)의 sentinel로 확인한다.
- F3-T07/T10/T15/T17은 실제로 확인하는 범위로 좁히고 기준의 완전한 증명으로 세지 않는다.
- ac-filtered-completeness에 큰 후보 집합의 candidate_limit(F3-R01–02)과 filtered 진행 중 취소(F3-R03–04)를, ac-ontology-fallback에 미검증 관계 baseline 시험(F3-R13–14)과 `VerifiedImplementations` 구현·시험(F3-R11–12)을 추가했다.
- 주장 P01/P04/P05 문구를 원문 한정어대로 고쳤고, P02 seed의 권위 표기는 N-03으로 바로잡는다. 6주장 모두 저장소 계약/ADR이며 조직 정책이 아님을 표시했다. about:P12-P01의 대상은 `dataset`으로 바꿨다.

## 확인 방법과 결과

1. 바뀌거나 새로 생긴 대상 16개를 go/ast로 현재 소스에서 찾았다. 각각 정확히 한 선언으로 결정되었고, 파일·행 범위·span SHA를 기록했다. canonical ID는 CKG `goCanonicalID` 형식(포인터·값 receiver 구분)을 따른다. [locator](../../system/eval/b0-knowledge-system/refactoring-n12-revised-proposal-2026-10-09/locate.go.txt).
2. 원래·새 관계가 가리키는 시험 11개를 race로 실행했다: 시험11/하위시험31 pass, fail0. [원자료](../../system/eval/b0-knowledge-system/refactoring-n12-revised-proposal-2026-10-09/named-tests.jsonl.gz). 이것은 test pass이며 semantic linked run이나 관계 verified가 아니다.

개정 후 기준별 상태: ac-filtered-completeness는 완전 증명 3개를 갖는다. ac-filtered-recall은 eligible=K 경계만 덮는다. ac-source-evidence의 F3-T12는 CKV가 좌표만 있는 mock이다. ac-ontology-fallback은 인터페이스로 이어진 두 시험의 합성이며 실제 저장소 하나의 종단 시험은 아니다.

## 남은 조건

- [x] 35건 권고 채택 기록(해석 명시)·수정 10건 반영 제안·대상 위치와 이름 붙은 시험 확인.
- [ ] 실제 검토자 이름. 이름이 없으면 어떤 항목도 verified가 되지 않는다.
- [ ] core20·기능 개념3·요구3·기준4 문구의 판정(이번 채택 범위 밖).
- [ ] 이름과 판정 이후 개정 시험·gate 변경을 포함한 새 coordinated capture/CKG/CKV/projection 결합(N12-B), 검토 활동 시간(N12-C, 미측정 null), 현행 결합과 N14 계획(N12-D).

**남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
