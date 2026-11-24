# 작업 목록: issue-relations

검토 보고서는 [REPORT.md](./REPORT.md)에 있다. 상태 값: `todo`, `doing`, `blocked`, `done`, `dropped`.

| # | 작업 | 상태 | 결과 |
|---|---|---|---|
| 1 | 결함 패턴 자료를 옮기고 다시 센다 | done | 2026-11-24. 저장소 밖 `20261124-defect-patterns`. 211건, 빠뜨림 111 일치 |
| 2 | 방향을 검토하고 보고서를 쓴다 | done | 2026-11-24. [REPORT.md](./REPORT.md), [data/summary.json](./data/summary.json), [연구일지 2026-11-24-15](../research-log/2026-11-24-15-issue-relations-review.md) |
| 3 | 첫 시험을 정한다(G0, G0R 또는 둘 다) | done | 2026-11-24. 사용자가 G0R(리팩토링부터)를 골랐다 |
| 4 | G0R: 여러 hop 변경을 grep 에이전트가 풀 수 있는가 | done | 2026-11-24. 10문항 모두 F1 1.000, 10만 자 초과 1개. 사전 표에 따라 리팩토링 쪽을 접는다. [HOPGATE](./HOPGATE.md), [결과](./HOPGATE-RESULT.md), [연구일지 2026-11-24-16](../research-log/2026-11-24-16-hop-gate-and-deque.md) |
| 5 | 응답 자료구조를 덱으로 바꾼다(설계) | done | 2026-11-24. 사용자 제안. REPORT 6장. hop을 바로 세는 이득을 쟀다 |
| 6 | `find_callers`, `find_callees`의 hop 세기를 고친다 | blocked | 사용자가 정한다. 고치면 모든 문항 묶음에서 다시 잰다 |
| 7 | G0: 결함의 나머지 자리를 grep 에이전트가 찾는가 | blocked | 사용자가 정한다. 확인용 16건 중 10건을 쓴다 |
