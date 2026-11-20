# 2026-11-20 연구일지 01: 출발 심볼 고정 실험 시작

세션: second-brain-eval-kit에서 연 Claude 세션. 기준 knowledge-system `4f02a749`.

## 정한 것

- **연구를 먼저 하고 리팩토링은 뒤에 한다.** 리팩토링은 기록된 호출을 다시 돌려 출력이 같을 때만 한다.
  11-20 보고서에서 재생은 474개 중 471개가 같은 결과를 냈다.
- **첫 실험은 출발 심볼을 고정한 오프라인 비교다.** 설계는
  [DESIGN.md](../oracle-seed-offline/DESIGN.md)에 있다. 결정 A(grep 대 graph)와 멈춤 규칙 실험을 한 장치에 묶었다.
  실행은 1단계(sym 11문항)와 2단계(문항 확장)로 나눈다.
- **측정용 그래프는 cold 빌드만 쓴다.** 같은 기준 커밋의 문항끼리는 그래프를 함께 쓴다. 증분 빌드 그래프는
  package contains 간선이 빠지는 문제가 있어 쓰지 않는다. 증분과 cold의 비교는 연구 뒤에 따로 한다.
- **blind 출발 심볼은 Claude가 고른다.** 사용자 지시다. 고르는 Claude는 11-20 보고서를 읽었으므로 sym-55와
  sym-77은 완전한 blind가 아니다.
- **검색 도구에 목적을 받는 안:** `reasons`(왜 찾는지)와 `search-types`(`debug`, `refactor`, `flow`)를 필수로 받는다.
  보고서에서 선택 항목 `reasons`는 호출 14개 중 2개에만 값이 있었다. type별 서버 효과는 오프라인으로,
  에이전트의 type 선택은 에이전트 실행으로 나눠 잰다. 아직 구현하지 않았다.

## 알게 된 사실

- cold 빌드는 이 컴퓨터에서 기준 커밋마다 약 46~48초였다. 그래프는 335MB, 345MB다
  (`knowledge-data/agent-eval/runs/20261120-ks-4f02a749-graphs`의 build.log와 디렉터리 크기).
- 로컬에 빌드된 두 기준 커밋 `8d7930a4`와 `0bf2f4d1`은 커밋 16개 떨어져 있고, 그 사이에 Go 파일 96개가 바뀌었다.
- go-stablenet에서 2025-01-01 이후 제목이 `fix`로 시작하는 커밋은 109개다. 같은 기간 커밋 167개가 바꾼 비테스트
  Go 파일은 중앙값 2개, 90분위 23개다.
- 문항 파일(`questions.jsonl`, `oracle.jsonl`), 하네스, 원자료 run은 사용자의 다른 컴퓨터에 있다.
  이 컴퓨터의 knowledge-system 세션이 확인해 알려 주었다.

## 다음 세션이 알아야 할 것

- 이 연구의 문서는 브랜치 docs/oracle-seed-offline-experiment에 있다. 작업 폴더는 별도 worktree
  `~/Work/github/knowledge-system-research`다. 다른 세션이 쓰는 원래 작업 트리는 건드리지 않았다.
- 문항 자료는 두 번에 나눠 받는다. 먼저 id와 질문 문장만 받는다. blind 출발 심볼을 커밋한 뒤에 정답과 기준 커밋을
  받는다. `questions/` 폴더를 통째로 받으면 `oracle.jsonl`이 같이 오므로 정답이 보인다.
- 증상 문장 요청은 2026-11-20에 다른 컴퓨터의 세션으로 보냈다. 그 경로는 받았는지 알려 주지 않는다.
  답이 없으면 사용자가 `questions.jsonl`을 이 컴퓨터로 옮겨야 한다.
