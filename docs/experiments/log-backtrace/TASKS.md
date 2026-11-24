# 작업 목록: 로그 역추적 조사

설계는 [DESIGN.md](./DESIGN.md)에 있다. 상태 값은 `todo`, `doing`, `blocked`, `done`.

| # | 작업 | 상태 | 막힌 이유 또는 결과 |
|---|---|---|---|
| 1 | 절차를 고정한다 | done | 2026-11-23. [DESIGN.md](./DESIGN.md) |
| 2 | S1, S2를 센다 | done | 2026-11-24. [data/s1-s2.json](./data/s1-s2.json), [tools/trace_message.py](./tools/trace_message.py) |
| 3 | S3을 본다 | done | 2026-11-24. 출력 지점 7개. s2-02는 4개 중 1개, p2-08은 10개 중 0개만 돌렸다. [valueprobe](./valueprobe/main.go) |
| 4 | 결과를 적는다 | done | 2026-11-24. [REPORT.md](./REPORT.md), [연구일지 2026-11-24-01](../research-log/2026-11-24-01-log-backtrace.md) |
| 5 | 추적 규칙 1판을 고정한다 | done | 2026-11-24. [RULES.md](./RULES.md) |
| 6 | 시제품을 만들고 6문항에서 돌려 본다(개발용 값) | done | 2026-11-24. core 8곳 중 3곳이 10위 안. [tracer](./tracer/doc.go), [data/tracer-dev.json](./data/tracer-dev.json), [연구일지 2026-11-24-02](../research-log/2026-11-24-02-message-tracer.md) |
| 7 | 확인 절차를 고정한다 | done | 2026-11-24. 사용자가 2022~2024년 수정에서 에이전트 하나로 뽑기로 했다. [CONFIRM.md](./CONFIRM.md) |
| 8 | 문항을 뽑고 검사해 고정한다 | done | 2026-11-24. 39개 중 문구가 있는 9개. [data/confirm-freeze.json](./data/confirm-freeze.json) |
| 9 | 확인을 돌리고 결과를 적는다 | done | 2026-11-24. 시제품 3, 기준선 3, 무승부 3. 판정하지 않는다. [REPORT.md](./REPORT.md) 5장, [data/confirm-result.json](./data/confirm-result.json), [연구일지 2026-11-24-03](../research-log/2026-11-24-03-message-confirmation.md) |
| 10 | 수정 PR에서 실제 에러 로그로 발견된 사례를 찾아 실험한다(사용자 요청) | done | 2026-11-24. 12문항에서 세 방식 모두 3개. 판정하지 않는다. 절차 [REALLOGS.md](./REALLOGS.md), [REPORT.md](./REPORT.md) 6장, [data/reallog-result.json](./data/reallog-result.json), [연구일지 2026-11-24-04](../research-log/2026-11-24-04-real-logs.md) |
| 11 | 시험 전체의 특징을 정리한다 | done | 2026-11-24. [cross-test/FINDINGS](../cross-test/FINDINGS.md). 추적 규칙은 더하지 않는다 |
| 12 | 잣대 하나로 모든 묶음을 다시 잰다 | done | 2026-11-24. 사전 판정선은 넘었지만 사후 대조에서 남지 않는다. [cross-test/RESULT](../cross-test/RESULT.md) |
| 13 | 에이전트 예비 시험(3문항) | done | 2026-11-24. 장비 점검에서 그래프 쓰기와 난이도 조건이 떨어졌다. [cross-test/PILOT-RESULT](../cross-test/PILOT-RESULT.md) |
| 14 | 영향 범위 찾기(1단계 자동 비교, 2단계 에이전트 점검) | done | 2026-11-24. field는 도구가 정확하다. 에이전트 점검은 graph 조건이 동작하지 않아 판단할 수 없고, grep 조건은 4문항 모두 F1 1.0이다. [impact-scope/REPORT](../impact-scope/REPORT.md) |
