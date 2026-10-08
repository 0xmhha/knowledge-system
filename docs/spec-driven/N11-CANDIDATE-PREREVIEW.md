# N11 후보 20문항 기술 사전검토

2026-10-09 · **Claude Code 사전검토(사람 판정 아님) / N11 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 목적

N11 후보 책(DEV7·FINAL13)의 정답이 인용 span 안에서 성립하는지, 중요군 분류가 맞는지, 기권 근거가 있는지, DEV와 FINAL이 같은 사실을 공유하지 않는지를 원문으로 확인했다. 검토자가 gold·군·독립성을 판정할 근거를 주려는 것이다. 검색·runner·색인·모델 호출은 하지 않았다. reviewer null, 승인0이다.

## 결과

[항목별 권고](../../system/eval/b0-knowledge-system/refactoring-n11-candidate-prereview-2026-10-09/prereview.json): accept7·조건부1·modify5·증거 보강4·reject3. 모든 span SHA는 source `479491ba`와 일치했다.

**치명 [High]: 후보 정답이 커밋된 문서에 거의 그대로 있다.** 예를 들어 `N11-FINAL-CANDIDATE-PREPARATION.md:31`은 FINAL-CAND-03의 후보 답을 그대로 담고 있다(주 세션 확인). N11-*/N12-* 문서와 system/eval의 질문 책·span 파일이 평가 corpus에 들어가면 평가가 무효가 된다. corpus 동결 때 이 경로를 제외하는 규칙이 필요하다.

**DEV/FINAL 사실 공유 [Mid].** DEV-06(worksheet.go:102-111)과 FINAL-03/04(lock.go:62-72)는 같은 규칙(KnownFields 뒤 두 번째 Decode가 EOF인지 확인, pack_incompatible 거부)을 묻는다(주 세션 확인). DEV-07과 FINAL-14/15도 같은 span SHA 계산 방식을 공유한다. 독립성 판정이나 DEV 교체가 필요하다.

**WHY 분류.** FINAL-04/13/15는 설계 근거 없이 코드에서 추론한 답이라 WHY로 쓰지 말 것을 권고한다. 같은 군의 CODE 문항이 이미 있어 재분류해도 독립 표본은 늘지 않는다.

**ABS 규칙 충돌.** corpus 문서가 "미지정", "RTO/RPO null"을 명시한다. 그 기록을 인용해 기권하는 답도 맞는데, strict_zero_citations는 이를 감점한다. 기대 행동이나 규칙을 정해야 한다.

증거 범위가 좁아 정답이 span 밖에 기대는 문항이 7개다(DEV-01/02/03/05/06/07, FINAL-07/14). 범위를 고치면 SHA를 다시 계산해야 한다.

## 채택 시 남는 독립 군

| 중요군 | FINAL | DEV |
|---|---|---|
| CODE | 4 (DEV 중복을 엄격히 보면 2–3) | 3 |
| WHY | 1 | 2 |
| POLICY | 1 code_contract, organization_policy 0 | 2 |
| TRACE | 2 code_contract, reviewed_path 0 | 0 |
| ABS | 1 | 0 |

모든 군이 10개 미만이다. [표본 부족 규칙](./N11-SHORTFALL-RULE.md)에 따라 지금 평가하면 모든 중요군이 inconclusive다.

- [x] 20문항 기술 사전검토, span SHA 재계산, 노출·공유 사실 2건 재확인.
- [ ] 검토자 이름과 문항별 gold·군·독립성·오염·ABS 판정, corpus 제외 규칙, protocol/runtime 동결. N11-A–D 미완료.

**남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
