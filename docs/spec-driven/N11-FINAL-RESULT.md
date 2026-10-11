# N11 깨끗한 DEV3·독립 FINAL 실행과 N08·N09·N10 판정

2026-10-10 · **N11 완료(판정: inconclusive / 기본값 disabled / 출시 보류)**. N10은 A·B·D 충족, p95비 1.25를 넘어 C 미충족. N08·N09는 개선 근거가 없어 미완료. 전체13/18, 남은5개.

## 1. 왜 실행했는가

N11 입력은 2026-10-09에 동결되었다([동결](./N11-INPUT-FREEZE.md)). 첫 DEV에서 지식 팩과 ontology arm의 warm p95가 기준 arm의 1.85–3.29배였다. 사용자는 FINAL 전에 지연을 고치기로 했다(`final-timing-decision.json`). 그 수정이 `7c57a388`이다([N10 변경](./N10-RETAINED-BATCH.md)). 수정 binary로 돌린 DEV2는 외부 부하(load 7→42) 때문에 오염되었다. 그래서 사용자가 정한 대로 한가한 시간에 DEV3를 다시 돌리고, 이어서 FINAL을 실행했다. FINAL 예약은 되돌릴 수 없으므로, 이 10문항은 앞으로 독립 holdout으로 쓰지 않는다.

## 2. 어떻게 실행했는가

[실행 스크립트](../../system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/run-dev3-final.py)는 다음 순서로 진행했다.

1. 부하를 기록했다.
2. DEV3를 실행했다.
3. DEV3가 exit 0, captured, 오류 0, 1,680행일 때만 FINAL 조건을 확인했다. 조건은 5분 부하 평균 10 이하이며, 대기 시간은 0초였다.
4. FINAL을 예약하고 실행했다.

두 실행 모두 같은 binary(`cks-7c57a388`)와 동결 protocol·질문책·runtime config를 썼다. arm은 8개, retrieval은 5회, warm은 20회, cold는 3회, K=10이다.

| 실행 | 시간(UTC) | 행 | 오류·누락 | 시작 전 부하(1/5/15분) | 종료 부하 |
|---|---|---|---|---|---|
| DEV3 | 02:24–03:21 | 1,680/1,680 | 0 / 0 | 5.04 / 5.52 / 4.92 | 8.86 / 8.38 / 8.86 |
| FINAL | 03:21–04:36 | 2,400/2,400 | 0 / 0 | 8.86 / 8.38 / 8.86 | 10.52 / 9.09 / 8.90 |

FINAL 동안 부하는 DEV3보다 높았다(사용자의 다른 작업, Chrome, VM, Ollama). 그러나 DEV2처럼 급증하거나 cold process가 강제 종료되지는 않았다. 8개 arm은 같은 질문마다 회전 순서로 번갈아 실행되므로, arm 사이의 비율은 같은 부하를 나눠 받는다. 절대 시간은 부하의 영향을 받는다.

## 3. 채점기에서 고친 것

FINAL 첫 채점은 지표를 하나도 출력하기 전에 멈췄다([기록](../../system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/final-scoring-unit-scope.json)). 동결 질문책은 CODE 문항 CAND-01과 WHY 문항 CAND-02를 설계상 같은 사실군 `semantic-readonly-open`에 묶는다. protocol은 그룹마다 단위를 센다. 반면 재사용한 B1 채점기는 단위 하나가 그룹 하나에만 속해야 한다고 강제한다. 그래서 어댑터([scripts/refactoring-eval-summarize.py](../../scripts/refactoring-eval-summarize.py))가 bootstrap 단위로 `그룹/사실군`을 넘기도록 했고, 그룹을 가로지른 사실군 수는 `distinct_fact_clusters`로 따로 남겼다. B1 채점기, 동결 입력, 임계치, 원자료는 바꾸지 않았다.

이 수정 전후로 DEV3를 다시 채점했다. 새 필드 2개와 채점기 SHA 말고는 모든 결과 값이 같았다. 시험은 6개 모두 통과했다(새 시험 1개 포함).

## 4. 결과

### 검색 품질 (N08)

| 실행 | 중요군 독립 단위 | 8arm Recall@10 평균 | 8arm MRR 평균 | arm 간 차이 |
|---|---|---|---|---|
| DEV3 | CODE3·WHY2·POLICY2 | 0.762(모두 같음) | 0.544(모두 같음) | 0, CI [0,0] |
| FINAL | CODE4·WHY1·POLICY1·TRACE2·ABS1 | 1.000(모두 같음) | 0.587(모두 같음) | 0, CI [0,0] |

- DEV3는 35개 retrieval slot 모두에서 8개 arm의 인용 목록이 같았다.
- FINAL은 50개 slot 가운데 CAND-07(TRACE)의 5회만 concept_text·combined arm의 순위가 3번째부터 달랐다. 그래도 Recall@10과 MRR은 같았다.
- 같은 질문을 반복해도 결과는 바뀌지 않았다.
- FINAL 답변 가능 8문항은 기준 arm에서 이미 모두 Recall@10 1.0이다. 그래서 이 FINAL에는 recall을 개선할 여지가 없었다.
- ABS 2문항의 recall 1.0은 기대 근거가 없어서 나온 값이며 의미가 없다.

### 비용·지연 (N10)

| arm | DEV3 warm p95비 | FINAL warm p95비 | FINAL p50 (backend / 그 밖) |
|---|---|---|---|
| baseline_off | 1.00 (1.57초) | 1.00 (1.25초) | 1.06초 (0.60 / 0.45) |
| baseline_on | 1.71 | 1.58 | 1.60초 (0.59 / 1.00) |
| concept_text_off | 1.33 | 1.40 | 1.46초 (0.62 / 0.80) |
| concept_text_on | 1.70 | 1.97 | 1.99초 (0.61 / 1.34) |
| relations_off | 1.20 | 1.39 | 1.42초 (0.60 / 0.82) |
| relations_on | 1.66 | 1.90 | 1.97초 (0.59 / 1.36) |
| combined_off | 1.24 | 1.37 | 1.45초 (0.61 / 0.80) |
| combined_on | 1.73 | 1.96 | 2.00초 (0.61 / 1.34) |

- **판정.** 동결 기준은 p95비 1.25 이하다. FINAL에서는 기준 외 7개 arm이 모두 이 기준을 넘었다. DEV3에서도 5개 arm이 넘었다. 따라서 N10-C는 충족되지 않는다.
- **원인.** backend 시간(CKG·CKV·embedding 호출)은 arm 사이에 0.59–0.62초로 거의 같다. 늘어난 시간은 backend 밖에서 생긴다. 팩·ontology projection을 읽고 검증하는 시간과 응답을 조립하는 시간이다.
- **같은 DEV 질문에서 수정 전과 비교.** DEV1(`e53c308e`)과 DEV3(`7c57a388`)를 비교하면 backend 밖 p50이 줄었다.

  | arm 묶음 | DEV1 | DEV3 |
  |---|---|---|
  | ontology off arm | 1.22초 | 0.85초 |
  | 팩 on arm | 1.39–2.21초 | 1.06–1.44초 |

  p95비도 1.85–3.29에서 1.20–1.73으로 내려갔다. 다만 두 실행은 시각과 부하가 달라서 개선 폭을 확정하지 않는다.
- **비용 분해.** 전 arm에서 실패·누락은 0이다. backend 시도와 응답 크기(p50 약 33KB)는 [FINAL 채점](../../system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/final-score.json.gz)의 `arm_summaries`와 `measurements`에 있다. 지연 분해는 [FINAL](../../system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/final-latency-breakdown.json)과 [DEV3](../../system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/dev3-latency-breakdown.json) 파일에 있다.
- **cold process p50.** 기준 arm이 3.12초, 팩 on arm이 4.18–4.22초다.

### 기권 (N09)

FINAL ABS는 사실군 1개(CAND-09·10, 운영 RTO/RPO와 복구 담당자)이며, 두 문항 모두 strict_zero_citations=false다. 동결 규칙은 기록된 미지정 근거를 인용하는 것은 허용하고, 그 밖의 인용은 답으로 본다. 8개 arm 모두 retrieval 10행 전부에 미지정 근거가 아닌 인용이 들어 있었다(off-basis 비율 1.0, 기준 대비 차이 0). 검색 도구는 답할 수 없는 질문에도 항상 인용 10개 남짓을 돌려주며, 기권하는 표면이 없다. 생성 답변은 없으므로 답변 수준의 기권 판정은 null이다.

## 5. 판정

| 작업 | 판정 | 근거 |
|---|---|---|
| N11 | **완료**: inconclusive / disabled / 출시 보류 | 동결 입력·runner 보호·DEV3·FINAL 실행·채점이 현재 코드와 원자료에 결합되었다. 모든 중요군이 독립 단위 10개 미만이므로 동결 소표본 규칙에 따라 inconclusive다. 반복·언어·상태 변형으로 표본을 부풀리지 않았고, 이전 FINAL은 다시 쓰지 않았다. |
| N10 | A·B·D 충족, **C 미충족** | 동결 입력에서 8arm 회전·직렬 warm/cold를 측정하고 분모와 비용을 분해했다. 하지만 p95비가 1.25를 넘는다. |
| N08 | **미완료** | 모든 arm의 품질 지표가 같아서 팩·ontology가 검색을 개선했다는 근거가 없다. FINAL은 기준 arm이 이미 recall 1.0이다. [N08 분석](./N08-DEV-ANALYSIS.md)의 후보 수정 2개는 구현하지 않았다. |
| N09 | **미완료** | 승인된 무답 사례에서 8arm 모두 off-basis 인용이 100%다. 무답을 막는 guard가 없다. 기계 집계는 기록했고 사람 기권은 null이다. |
| N18 | 미판정(선행 미완료) | 현재 근거로는 기본값 disabled, 출시 보류만 가능하다. N08·N09·N10·N16이 남아 있어 완료 처리하지 않는다. |

## 6. 한계

- 입력 검토자는 "opus5.5 검토"다. 사용자가 지정한 AI 검토 라벨이며, 독립적인 사람의 내용 검토가 아니다.
- FINAL 동안 부하 평균이 약 9로, DEV3(약 5)보다 높았다. arm 사이의 비율은 회전 실행으로 같은 조건을 나눠 받지만, 절대 지연은 부하의 영향을 받는다.
- 채점 어댑터는 FINAL 실행 뒤, 지표를 보기 전에 고쳤다. 고친 것은 단위의 이름 범위뿐이고, 판정 규칙과 수치 계산은 바꾸지 않았다.
- 이 FINAL 10문항은 이제 관측된 자료다. 다시 판정하려면 새 DEV와 새 독립 FINAL이 필요하다.

- [x] N11-A–D. 판정은 inconclusive이며, 기본값 disabled·출시 보류를 명시한다.
- [x] N10-A, N10-B, N10-D / [ ] N10-C(p95비 초과).

**남은 전체5개:** N08 검색 품질, N09 기권, N10 비용·지연(p95), N16 신뢰·권한(서명 키), N18 기본값·출시 판정.
