# 실험 기록

이 폴더는 검색 연구의 실험 설계, 작업 목록, 결과 보고서, 연구일지를 모은다.
11-20 이전의 평가 보고서는 [docs/research](../research/GRAPH-MCP-AGENT-EVAL-2026-11-20.md)에 있다.

## 폴더 구조

```text
docs/experiments/
  README.md                 이 문서. 기록 규칙
  research-log/             연구일지. 시간 순서로 쌓는다
    YYYY-MM-DD-NN-<topic>.md
  <experiment>/             실험 하나에 폴더 하나
    DESIGN.md               무엇을 왜 재는지, 판정선
    TASKS.md                작업 목록과 상태
    REPORT.md               결과 보고서 (결과가 나온 뒤)
    data/                   저장소에 남길 작은 입력과 결과 (JSON, CSV)
```

폴더와 파일 이름은 영어로 쓴다.

## 기록 규칙

1. **실험마다 폴더를 만든다.** 설계, 작업 목록, 보고서, 작은 데이터를 그 폴더에 모은다.
2. **큰 원자료는 저장소 밖에 둔다.** 그래프 DB(기준 커밋마다 약 340MB)와 세션 기록은
   `knowledge-data/agent-eval/runs/<run>`에 둔다. 저장소에는 그 경로와 크기, 해시를 적는다.
3. **작업이 끝나면 문서를 함께 고친다.** `TASKS.md`의 상태를 바꾸고, 결과가 있으면 `REPORT.md`나 `data/`에 넣는다.
   문서를 고치지 않은 작업은 끝난 것으로 보지 않는다.
4. **모든 작업은 커밋으로 남긴다.** 작업 하나가 끝날 때마다 커밋한다. 결과 숫자와 그 숫자를 만든 입력이 같은 커밋에
   들어가게 한다.
5. **연구일지를 남긴다.** 세션에서 정한 것, 알게 된 사실, 다음 세션이 알아야 할 것을 `research-log/`에 쓴다.
   파일 이름은 `YYYY-MM-DD-NN-<topic>.md`다. `NN`은 그날의 순번이다. 지난 일지는 고치지 않고 새 일지로 바로잡는다.
6. **확인한 것과 추정한 것을 나눠 쓴다.** 숫자에는 분모와 출처(파일, 명령)를 붙인다.

## 실험 목록

| 폴더 | 내용 | 상태 |
|---|---|---|
| [oracle-seed-offline](./oracle-seed-offline/DESIGN.md) | 출발 심볼을 고정하고 grep과 graph 탐색을 비교, 멈춤 규칙 탐색 | 닫음, 결정 A는 판정 안 됨([보고서](./oracle-seed-offline/REPORT.md)) |
| [log-backtrace](./log-backtrace/DESIGN.md) | 실패 문구에서 시작해 지금 기능으로 원인까지 이어지는지 조사 | 진행 중 |
