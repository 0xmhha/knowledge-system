# 작업 목록: 로그 역추적 조사

설계는 [DESIGN.md](./DESIGN.md)에 있다. 상태 값은 `todo`, `doing`, `blocked`, `done`.

| # | 작업 | 상태 | 막힌 이유 또는 결과 |
|---|---|---|---|
| 1 | 절차를 고정한다 | done | 2026-11-23. [DESIGN.md](./DESIGN.md) |
| 2 | S1, S2를 센다 | done | 2026-11-24. [data/s1-s2.json](./data/s1-s2.json), [tools/trace_message.py](./tools/trace_message.py) |
| 3 | S3을 본다 | done | 2026-11-24. 출력 지점 7개. s2-02는 4개 중 1개, p2-08은 10개 중 0개만 돌렸다. [valueprobe](./valueprobe/main.go) |
| 4 | 결과를 적는다 | done | 2026-11-24. [REPORT.md](./REPORT.md), [연구일지 2026-11-24-01](../research-log/2026-11-24-01-log-backtrace.md) |
