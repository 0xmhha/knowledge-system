# N11 DEV 실행 결과와 채점 규칙 고정

2026-10-09 · **DEV 실행·채점 완료 / FINAL 미실행(예약0)**. 전체9/18, 남은9개.

## 1. 무엇을 실행했는가

동결한 입력([N11 입력 동결](./N11-INPUT-FREEZE.md))과 새 평가 corpus `e693b312`/dataset `cd9a22e3`, binary `167d8711`, bge-m3 `7907646…6bab`/1024차원으로 `scripts/refactoring-eval-run.py run --split DEV`를 실행했다. DEV7문항 × 8arm × (warmup2 + retrieval5 + warm20 + cold3) = 1,680행이다. 2026-10-08T18:51:56Z–20:01:21Z, exit0, report state `captured`, 누락 slot0, 오류행0, backend 기록 누락0, binary·source·설정·잠금 파일 전후 SHA 동일. FINAL 예약 registry는 비어 있다.

## 2. 채점 규칙을 FINAL 관측 전에 고정했다

[scripts/refactoring-eval-summarize.py](../../scripts/refactoring-eval-summarize.py)는 기존 B1 채점기(`scripts/b1-summarize-matrix.py`)의 Recall@10·MRR(중복 제거·top10·줄 범위 겹침)·분모·cluster 단위 paired bootstrap(seed20261003, 10,000회)·warm/cold 백분위를 그대로 쓴다. 이 어댑터는 동결 질문책을 B1 사례 형식으로 바꾸고, ABS 항목에 대해 동결 규칙(기록된 미지정/null 근거 인용은 허용, 그 밖의 인용은 답으로 본다)을 기계적으로 센다. 생성 답변이 없으므로 답변 수준 기권 판정은 null이다. 합성 시험5개가 통과했다.

## 3. DEV 결과

| arm | Recall@10 평균 | MRR 평균 | warm p50 ms | warm p95 ms | p95비 | 비backend p50 ms |
|---|---|---|---|---|---|---|
| baseline_off | 0.762 | 0.544 | 966 | 1175 | 1.00 | 405 |
| baseline_on | 0.762 | 0.544 | 1933 | 2535 | 2.16 | 1394 |
| concept_text_off | 0.762 | 0.544 | 1785 | 2380 | 2.02 | 1224 |
| concept_text_on | 0.762 | 0.544 | 2756 | 3518 | 2.99 | 2194 |
| relations_off | 0.762 | 0.544 | 1779 | 2179 | 1.85 | 1226 |
| relations_on | 0.762 | 0.544 | 2764 | 3861 | 3.29 | 2209 |
| combined_off | 0.762 | 0.544 | 1783 | 2560 | 2.18 | 1223 |
| combined_on | 0.762 | 0.544 | 2766 | 3405 | 2.90 | 2193 |

- 8arm의 인용 결과가 문항별로 모두 같다. 지식 팩과 ontology 모드는 DEV에서 인용을 바꾸지 않았다. 이전 FINAL의 "8arm 동일" 관측과 같은 양상이다.
- backend가 보고한 시간(p50 538–571ms)은 arm 사이에 거의 같다. 늘어난 시간은 backend 밖에서 생긴다. `include_knowledge`가 약 1.0초, ontology 모드가 약 0.8초를 더하며 둘은 더해진다. composer 단계 기록을 보면 ontology 모드의 증가분은 stage1→stage2 구간에 있고, 지식 팩의 증가분은 composer 앞뒤에 있다([분해](../../system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/dev-latency-breakdown.json)).
- 7개 arm 모두 p95비 1.25를 넘는다. DEV는 조정용이므로 이 수치를 합격·불합격 판정으로 쓰지 않는다.
- DEV 중요군 독립 단위는 CODE3·WHY2·POLICY2이다. 모두 10 미만이므로 동결 규칙에 따라 inconclusive이다.

측정 한계: 실행 시작 시 load average 5.32였다. arm 순서를 회전했으므로 arm 사이 비교에는 같은 부하가 나뉘어 들어가지만, 절대 지연값은 이 머신 상태에 묶인다.

## 4. 상태 구분

| 구분 | 상태 |
|---|---|
| DEV 실행 | 1,680/1,680행, 오류0 |
| FINAL | 미실행, 예약0 |
| N08/N09/N10 판정 | 미판정. FINAL과 사전 결정 이후에 판단 |
| 사람 수용 | 없음. 검토 라벨은 "opus5.5 검토"(AI 라벨) |

**남은 전체9개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
