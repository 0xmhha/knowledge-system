# N08 사용 타입 선언을 v2 근거에 추가 (후보 B)

2026-10-11 후속: [FINAL2](./FINAL2-RESULT.md)에서 이전 binary 대비 recall 동일, MRR −0.032(삽입 선언이 gold 위로 들어간 5문항, 개선 문항0). 개선 근거 없음, inconclusive.

2026-10-10 · **코드 변경·시험·DEV 개발 측정 완료 / 새 독립 FINAL 필요 / N08 미완료**. 전체13/18, 남은5개.

## 1. 왜 바꿨는가

[N08 분석](./N08-DEV-ANALYSIS.md)의 원인 2와 3에 대한 것이다. "왜"를 설명하는 짧은 주석 달린 타입 선언(예: `reindexLock`)은 dense 검색 순위에서 긴 함수 본문에 밀린다. 그리고 그 선언에 닿는 CKG `uses_type` edge는 v2 응답에 실리지 않았다. 사용자는 재색인이 필요 없는 후보 B를 골랐다.

## 2. 무엇이 잘못되어 있었는가

분석 당시에는 "v2가 composer의 이웃 참조를 싣지 않는다"고 보았다. 구현 중에 더 확인하니 문제가 더 앞 단계에 있었다. CKG의 `uses_type` edge가 cks relation 어휘에 매핑되어 있지 않아서, composer의 이웃 탐색에서 아예 버려지고 있었다(`TestReal_Neighbors_DropsUntranslatableEdges`가 이 동작을 고정하고 있었다).

## 3. 무엇을 바꿨는가

- **새 relation(`f2f463db`).** contract에 `uses_type` relation을 추가했다. CKG client는 호출자가 이 relation을 명시적으로 요청할 때만 `uses_type` edge를 돌려준다. 기본 탐색, intent별 탐색, subgraph는 지금과 같다.
- **v2 handler.** 상위 3개 Go 본문마다 `uses_type` 이웃을 1-hop 조회한다. 프로젝트 타입 선언을 최대 2개까지, 해당 본문 바로 뒤에 넣는다. 이미 인용된 본문과 겹치는 선언과 시험 코드는 제외한다.
- **예산 보호.** 선언을 넣은 pack이 primary 본문을 하나도 빼지 않고 예산(12인용, 32000byte)에 들어갈 때만 채택한다. 그렇지 않으면 기존 pack을 그대로 쓴다. 조회 실패도 기존 pack을 유지한다.
- **doc comment 포함(`7077741f`).** CKG node span은 선언 줄부터 시작해 주석이 빠진다. 그래서 바로 위에 붙은 `//` 주석 묶음(최대 30줄)을 보관 원문에서 읽어 span에 포함한다.
- 질문이나 gold를 코드에 넣지 않았다.

## 4. 어떻게 확인했는가

- **시험 4개.**
  1. 삽입 위치와 상한, 겹침 제외, 조회 옵션
  2. Go가 아닌 본문, 조회 실패, 다른 relation 무시
  3. doc comment 포함과 30줄 상한
  4. 주석까지 넓힌 span이 인용된 본문과 겹치면 제외
- **ckgclient 시험.** 명시적으로 요청한 `uses_type`만 반환하는지 확인했다. 기존 drop 시험도 그대로 통과한다.
- **변이 6개를 모두 잡았다.** 겹침 무시, 상한 무시, 앵커 앞 삽입, 기본 탐색에 uses_type 노출, 다른 relation 허용, 넓히기 전 span으로 겹침 판단이다([기록](../../system/eval/b0-knowledge-system/refactoring-n08-type-neighbors-2026-10-10/mutation-checks.txt)).
- **전체 race.** 116개 패키지의 race, vet, gofmt가 통과했다([출력](../../system/eval/b0-knowledge-system/refactoring-n08-type-neighbors-2026-10-10/related-race.txt)).

## 5. DEV 개발 측정

N10 DEV4와 같은 방식으로 실행했다. N11 runner의 `cks eval matrix`에 DEV3 요청 7개, 동결 runtime config, 반복 횟수(2/5/20/3)를 주고 binary만 `cks-7077741f`로 바꿨다([스크립트](../../system/eval/b0-knowledge-system/refactoring-n08-type-neighbors-2026-10-10/run-dev5.py)). protocol과 검토 기록은 rebind하지 않았으므로 동결 평가가 아니다. 실행은 1,680행, 오류 0, captured로 끝났고 종료 시 강제 종료도 0건이었다. 부하 평균은 시작 시 10.3, 종료 시 5.5였다.

| 지표(baseline_off, 문항 평균) | DEV3 `7c57a388` | DEV4 `37302b35` | DEV5 `7077741f` |
|---|---|---|---|
| Recall@10 | 0.762 | 0.762 | **0.833** |
| MRR | 0.544 | 0.544 | **0.563** |
| warm p95비(기준 외 7arm) | 1.20–1.73 | 0.92–0.98 | 0.98–1.05 |
| warm p50(baseline_off) | 1.07초 | 0.84초 | 0.85초 |

- DEV-09는 `reindexLock` 선언과 주석(reindex.go 60–63)이 2위에 들어와 recall이 0.5에서 1.0으로, MRR이 0.5에서 0.75로 올랐다.
- 대가도 있다. 선언이 앞에 끼어들어 DEV-05의 gold는 3위에서 4위로(MRR 0.333→0.25), DEV-08은 7위에서 9위로(0.143→0.111)밀렸다.
- 8개 arm은 이번에도 문항별 지표가 같다. 이 변경은 모든 arm의 기본 v2 경로를 바꾸기 때문이다.
- CKG 시도는 12,660회에서 13,230회로, 요청당 약 2.7회 늘었다. backend p50은 0.57–0.58초로 같다.

## 6. 한계

- **표본.** DEV 7문항으로 정한 설계다. 개선된 문항은 1개(DEV-09)뿐이고 순위가 내려간 문항은 2개다. 과적합 위험이 있으므로 새 독립 FINAL로 확인해야 한다.
- **arm 비교로는 판정할 수 없다.** 이 변경은 모든 arm에 똑같이 적용되므로, 동결 protocol의 arm 대비 baseline_off 비교로는 효과를 볼 수 없다. 새 FINAL protocol에는 이전 binary와 새 binary를 비교하는 항목이 필요하다. 이 설계는 결과를 보기 전에 검토받아야 한다.
- **knowledge 예산 충돌.** 선언 2개가 더해지면 knowledge를 함께 요청할 때 12인용 예산을 넘기 쉬워진다. 그러면 knowledge 문맥이 `budget_exceeded`로 빠질 수 있다. 평가 corpus에는 knowledge가 없어 측정하지 못했다.
- **다루지 못한 원인.** sentinel 변수(원인 3의 나머지)는 CKG에 참조 edge가 없어 다루지 못한다(후보 A의 범위).

**남은 전체5개:** N08 검색 품질(새 FINAL), N09 기권, N10 비용·지연(새 FINAL), N16 신뢰·권한(서명 키), N18 기본값·출시 판정.
