# N12 core20·기능 개념·요구·기준 기술 사전검토

2026-10-09 · **Claude Code 사전검토(사람 판정 아님) / N12 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 목적

앞서 채택한 35건([개정 제안](./N12-REVISED-PROPOSAL.md)) 밖에 남은 개념23(core20+기능3), 요구3, 기준4를 코드와 대조했다. 검토자가 각 항목을 승인·수정·반려할 근거를 주려는 것이다. reviewer는 null, 개별 승인0이다. 원본 `ontology-pilot.yaml`, `ontology-pilot-functional.yaml`, `spec-pilot-v2.yaml`은 held 후보의 입력이므로 바꾸지 않았다.

## 결과

[항목별 권고](../../system/eval/b0-knowledge-system/refactoring-n12-core-prereview-2026-10-09/prereview.json): 개념 accept13·modify10, 요구·기준 accept2·modify5, reject0. 아래 네 가지는 주 세션이 코드에서 다시 확인했다.

1. **criterion-decision**의 includes에 있는 "needs-review decision"은 제품이 지원하지 않는다. `patch.Decide`는 approved/rejected만 받는다(internal/system/patch/review.go:175). 문구를 지우거나 제품에 outcome을 추가해야 한다.
2. **ac-filtered-completeness**는 취소 결과를 "incomplete"라고 하지만, 제품은 취소를 `cancelled`로 돌려준다(sqlitevec/store.go `interruptedSearch`). 새 취소 시험도 `SearchCancelled`를 확인한다. 기준 문구를 "incomplete 또는 cancelled"로 고쳐야 시험과 문구가 맞는다.
3. **ac-ontology-fallback**의 "preserved"는 집합 보존으로 명시해야 한다. 검증된 관계가 있으면 순서는 바뀔 수 있다(ontology_provider.go:213).
4. 한국어 용어는 부분 문자열로 매칭된다(semantic/matcher.go:78-90). "의미 관계"가 "스냅샷에 결합된 의미 관계" 안에, "수용 기준"이 "수용 기준 판정" 안에 들어 있어 두 개념이 늘 함께 걸린다.

나머지 수정은 코드보다 넓게 약속하는 문구(code-symbol의 canonical ID, evidence-span의 origin/file digest, acceptance-criterion의 snapshot, patch-attempt의 check report)와 기준의 미정 범위(exact 크기, "many", "promoted")다.

## 남은 조건

- [x] 30건 기술 사전검토와 핵심4건 코드 재확인.
- [ ] 검토자 이름과 30건 판정. 채택하면 원본을 덮지 않고 새 버전 파일(ontology/spec 다음 버전)로 작성한 뒤 새 capture에 결합한다. criterion-decision을 제품 변경으로 해결할지는 별도 결정이다.

**남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
