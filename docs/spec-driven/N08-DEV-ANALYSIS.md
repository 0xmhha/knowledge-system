# N08 DEV 검색 품질 원인 분석

2026-10-10 · **DEV 원인 분석 완료 / 개선 미구현 / N08 미완료**. 전체9/18, 남은9개.

## 1. 왜 분석했는가

첫 N11 DEV에서 8개 arm의 인용이 문항별로 모두 같았다(Recall@10 0.762, MRR 0.544). 지식 팩과 ontology는 검색 결과를 바꾸지 않았다. 어디서 기대 근거가 빠지는지 확인해야 개선 방향을 정할 수 있다. FINAL 문항은 조정에 쓰지 않으므로 DEV 7문항만 읽었다.

## 2. 어떻게 확인했는가

[analyze.py](../../system/eval/b0-knowledge-system/refactoring-n08-dev-analysis-2026-10-10/analyze.py)가 세 가지를 대조한다. 첫째, 평가 ontology(`ontology-pilot-functional-v2`, 개념23·용어46)의 용어가 DEV 질문에 들어 있는지. 둘째, 첫 DEV의 baseline_off 응답에서 gold 범위가 몇 위에 있는지. 셋째, 동결 색인의 scratch 복사본에 raw CKV 질의(K=60, threshold 없음)를 보냈을 때 gold 청크가 몇 위에 오는지. 동결 데이터셋 자체는 열지 않았고 footprint도 쓰지 않았다. CKG/CKV DB는 읽기 전용(immutable)으로 조회했다.

## 3. 결과

| 문항 | 군 | 매칭 용어 | 놓친 gold 범위 | 응답 순위 | raw CKV 순위(청크) |
|---|---|---|---|---|---|
| DEV-01 | CODE | 0 | `ErrVersionedSetupRequired` var (maintenance.go:12) | 없음 | 60위 밖(파일 머리 청크만 3위) |
| DEV-03 | WHY | 0 | `ErrDurabilityUncertain` var (durability.go:21–23) | 없음 | 60위 밖(파일 머리 청크만 25위) |
| DEV-03 | WHY | 0 | promote 함수 중간 (reindex.go:441–454) | 없음 | 31위 |
| DEV-09 | WHY | 1 (데이터셋) | `reindexLock` struct와 주석 (reindex.go:60–63) | 없음 | 18위 |

나머지 gold 범위는 응답 1–7위 안에 있다([전체](../../system/eval/b0-knowledge-system/refactoring-n08-dev-analysis-2026-10-10/analysis.json)).

### 원인 1: ontology 어휘가 질문 영역을 덮지 않는다

46개 용어는 의미 계층(소스 스냅샷, 근거 묶음, 주장, 수용 기준 등)을 다룬다. DEV 질문은 setup 운영 코드(유지보수 경계, GC 보호, 자원 보고, 내구성, 캡처 상한, 잠금)를 묻는다. 7문항 중 6문항은 매칭할 용어가 없다. DEV-09만 일반 개념 "데이터셋"과 맞지만, 그 개념의 검증된 구현 관계는 잠금 코드를 가리키지 않는다. 따라서 ontology arm이 인용을 바꾸지 못하는 것은 matcher 결함이 아니라 어휘 범위의 문제다.

### 원인 2: 짧은 선언 청크가 dense 순위에서 밀린다

놓친 범위는 모두 CKV에 정확한 청크로 들어 있다(Var 10–12, Var 21–23, Struct 60–63). 색인 누락이 아니다. "왜"에 대한 근거가 들어 있는 짧은 sentinel 오류 선언과 주석 달린 타입이, 한국어 질문과의 embedding 거리에서 긴 함수 본문과 문서 절에 진다. 응답은 10개 본문에서 잘리므로 18위도 들어오지 못한다.

### 원인 3: 그래프 확장이 이 선언에 닿지 못하거나, 닿아도 응답에 실리지 않는다

CKG에는 함수가 패키지 변수를 읽는 참조 edge가 없다. `checkMaintenanceTarget`(응답 1위)이 `ErrVersionedSetupRequired`를 반환하지만 둘 사이 edge는 `defines`(File)뿐이다. `reindexLock`은 `acquireReindexLock`(응답 1위)에서 `uses_type`으로 닿는다. 그러나 v2 응답은 composer의 edge 전용 이웃 참조를 싣지 않는다(`internal/system/mcp/get_for_task_v2.go`의 주석: 12인용 예산 계약 보호).

## 4. 개선 후보 (미구현)

| 후보 | 내용 | 장점 | 단점 |
|---|---|---|---|
| A | 선택된 함수 본문이 참조하는 같은 패키지의 주석 달린 선언(sentinel 오류·타입)을 예산 안에서 본문으로 추가 | 원인 2·3을 함께 다룸 | CKG에 변수 참조 edge가 필요해 extractor 변경과 재색인, corpus 재동결이 필요함 |
| B | v2 응답이 `uses_type` 등 기존 edge의 이웃을 12인용/32000byte 예산 안에서 본문으로 실음 | 재색인 없이 composer/v2만 수정 | sentinel 변수는 edge가 없어 못 다룸. 이웃이 기존 본문을 밀어낼 위험이 있음 |

어느 후보든 질문·gold를 코드에 넣지 않는다. DEV 3개 범위에 과적합할 위험이 있으므로 기존 회귀와 안전 조건을 같이 지켜야 한다. 구현하면 binary가 바뀌므로 재동결과 새 DEV가 필요하다. 사용자 결정(2026-10-09)에 따라 FINAL은 다음 한가한 시간에 현재 수정 수준으로 실행한다. 이 개선을 그 전에 넣을지는 별도로 정한다.

## 5. 상태

- [x] DEV 원인 분석과 재현 스크립트
- [ ] 개선 구현과 DEV 확인
- [ ] FINAL 판정(중요군 10 미만이라 inconclusive 예정)

**남은 전체9개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
