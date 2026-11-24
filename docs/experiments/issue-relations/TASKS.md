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
| 7 | G0: 결함의 나머지 자리를 grep 에이전트가 찾는가 | done | 2026-11-24. 사용자가 골랐다. 확인용 대신 7월 이전 27건에서 9문항. 22곳 중 20곳, 못 봐서 놓친 것 0. 결함 쪽 `relate`를 만들지 않는다. [FIXGATE](./FIXGATE.md), [결과](./FIXGATE-RESULT.md), [연구일지 2026-11-24-17](../research-log/2026-11-24-17-fix-gate-and-critique.md) |
| 8 | 연구 방향과 방법의 비판적 검토 | done | 2026-11-24. [CRITIQUE.md](./CRITIQUE.md) |
| 9 | 다음 방향을 정한다 | done | 2026-11-24. 사용자가 두 시험을 제안했다(10, 11번) |
| 10 | grep을 흉내 낸 그래프 조회를 도구와 에이전트로 견준다 | done | 2026-11-24. 도구끼리 우월(글자 0.33배), 에이전트 차이 없음(토큰 0.991배). [GREPLIKE](./GREPLIKE.md), [결과](./GREPLIKE-RESULT.md), [연구일지 2026-11-24-18](../research-log/2026-11-24-18-grep-like-graph.md) |
| 11 | 심볼 밖에 원인이 있는 결함 1개를 견준다 | done | 2026-11-24. grep 5/5, gk 5/5, 토큰 1.01배 |
| 12 | 이름에 뜻이 없는 코드에서 다시 견준다 | dropped | 2026-11-24. 사용자가 fork 전용 코드로 바꿨다(13번) |
| 13 | go-stablenet에만 있는 코드에서 다시 견준다 | done | 2026-11-24. reach에서 `walk`가 토큰을 줄인다(12문항 중앙값 0.883, 11/12). fork 전용이라서는 아니다. signature와 결함은 이득 없음. [FORKONLY](./FORKONLY.md), [결과](./FORKONLY-RESULT.md), [연구일지 2026-11-24-19](../research-log/2026-11-24-19-fork-only.md) |
| 14 | reach의 비용 이득을 되풀이와 gopls 대조로 확인한다 | blocked | 사용자가 정한다 |
