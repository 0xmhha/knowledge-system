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
