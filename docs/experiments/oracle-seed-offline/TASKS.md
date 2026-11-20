# 작업 목록: oracle-seed-offline

설계는 [DESIGN.md](./DESIGN.md)에 있다. 작업이 끝나면 이 표의 상태와 "결과" 칸을 고치고 같은 커밋에 넣는다.

상태 값: `todo`, `doing`, `blocked`, `done`.

## 1단계 (sym 11문항)

| # | 작업 | 상태 | 막힌 이유 또는 결과 |
|---|---|---|---|
| 1 | 설계 문서와 기록 규칙을 저장소에 넣는다 | done | 2026-11-20. [README](../README.md), [DESIGN.md](./DESIGN.md) |
| 2 | 문항의 증상 문장(id와 질문만)을 받는다 | done | 2026-11-20. 문항 파일을 가진 다른 컴퓨터의 세션이 보냈다. [data/questions-symptom.json](./data/questions-symptom.json) |
| 3 | blind 출발 심볼을 고르고 `data/seeds-blind.json`으로 커밋한다 | done | 2026-11-20. 11문항, 심볼 24개. sym-55, sym-75, sym-77은 완전한 blind가 아니다. [data/seeds-blind.json](./data/seeds-blind.json) |
| 4 | 기준 커밋, 수정 커밋, core 라벨, 재생 도구를 받는다 | blocked | 3번 커밋 뒤 2026-11-20에 요청했다. 답을 기다린다 |
| 5 | core 라벨을 사람이 검토하고 수용한다 | todo | 4번 뒤 |
| 6 | near 출발 심볼을 고른다 | todo | 4번 뒤 |
| 7 | 빠진 기준 커밋 9개의 cold 그래프를 빌드한다 | todo | 4번 뒤. sym-55, sym-77의 그래프는 `knowledge-data/agent-eval/runs/20261120-ks-4f02a749-graphs`에 있다 |
| 8 | 같은 입력을 두 번 돌려 출력이 같은지 확인한다 | todo | |
| 9 | 결정 A 비교(grep 대 graph)를 돌린다 | todo | |
| 10 | 실험 1(정답까지의 거리)을 돌린다 | todo | |
| 11 | 1단계 보고서 `REPORT.md`를 쓴다 | todo | |

## 2단계 (문항 확장)

| # | 작업 | 상태 | 막힌 이유 또는 결과 |
|---|---|---|---|
| 12 | 수정 커밋에서 문항 후보를 뽑는다 | todo | 2025-01-01 이후 제목이 `fix`로 시작하는 커밋 109개 |
| 13 | 문항마다 증상 문장, blind 출발 심볼, core 라벨을 만든다 | todo | |
| 14 | 실험 2~4를 돌린다 | todo | |
| 15 | 결정 A를 다시 판정하고 보고서를 고친다 | todo | |
