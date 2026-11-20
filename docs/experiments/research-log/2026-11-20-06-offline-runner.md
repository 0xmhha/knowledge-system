# 2026-11-20 연구일지 06: 오프라인 실행기

세션: second-brain-eval-kit에서 연 Claude 세션. 작업 7a.

## 정한 것

결정 A의 결과를 보기 전에 정했다. [DESIGN.md](../oracle-seed-offline/DESIGN.md) 4장에도 적었다.

- **graph 방식은 composer stage 3을 그대로 부른다.** 에이전트용 MCP 도구가 아니라 composer가 3 hop으로 넓히는
  코드(`internal/system/composer/stage3`)다. 설계의 "3-hop 이웃과 기본 순위"에 가장 가깝다. 점수는
  `1/(1+거리)`이고, 같은 점수는 범위가 넓은 노드, 파일, 줄 순서로 놓인다.
- **intent는 Unknown이다.** 정방향 간선 전부와 호출자를 걷는다. BugFix intent는 호출과 호출자만 걷는다.
  실험 2(간선 하나씩 빼기)가 전체 간선에서 출발하므로 전체를 걷는 쪽을 골랐다.
- **결정 A는 상한을 끈 목록으로 판정한다.** 출하 설정은 출발 심볼당 50개, 최종 50개로 자른다. 자르면 도달률이
  상한에 묶인다. 출하 설정 목록도 함께 기록만 한다.
- **grep은 Go 안에서 한다.** 이 컴퓨터의 `rg`는 셸 함수라서 다른 프로그램이 부를 수 없다. Go의 `\b`는
  `rg -w`와 같은 ASCII 단어 경계다. 대상 파일은 그래프 build-scope의 파일 전부다(테스트 포함, graph 쪽도
  테스트를 빼지 않으므로 맞췄다).
- **후보의 단위는 선언 노드다.** 맞은 줄은 그 줄을 감싸는 가장 작은 선언 노드로 옮긴다. 필드, 인자, 지역 변수,
  import는 선언 노드로 보지 않는다. 어떤 선언에도 들지 않는 줄은 버리고 그 수를 기록한다.
- **정답 core를 노드로 옮기는 규칙:** 같은 파일에서 이름이 oracle 심볼의 마지막 부분과 같고, 범위가 oracle
  범위와 겹치는 선언 노드다. 여럿이면 시작 줄이 가까운 쪽이다. 다른 컴퓨터의 재생 스크립트 규칙(같은 파일,
  패키지를 뗀 qname이 같음)을 그래프 노드에 맞춘 것이다.
- **출발 심볼 자신은 두 방식 모두 후보에서 뺀다.** 정답 core가 출발 심볼이면 두 방식 모두 0위로 찾은 것으로 센다.

## 알게 된 사실

- 그래프는 `immutable=1` 읽기 전용으로 열린다(`internal/graph/persist/sqlite.go`의 `OpenReadOnly`).
  실행기는 그래프 폴더 안에 출력하지 않도록 막았다.
- 단위 테스트 5개가 통과했다(`go test ./docs/experiments/oracle-seed-offline/runner/`).
- sym-74 한 문항으로 시험 실행했다. 약 3초 걸렸다. 후보 수는 grep 5개, graph 104개(상한 없음),
  50개(출하 상한)였다.
- **sym-74의 정답 core `txMaxSize`는 blind 출발 심볼 중 하나다.** 그래서 sym-74는 두 방식이 반드시 비긴다.
  결정 A에서 무승부로 빠진다. 증상 문장("허용되는 최대 크기에 가까운 컨트랙트를 배포하는 트랜잭션을 보내면
  노드가 받자마자 거절한다")이 txpool의 크기 검사를 가리켜서, 그 상수를 출발 심볼로 골랐다.
- sym-74에서 graph의 3 hop 목록은 104개뿐이었다. hop별로 보면 1 hop 55개, 2 hop 37개, 3 hop 12개다.
  왜 3 hop에서 적은지는 확인하지 않았다.

## 다음 세션이 알아야 할 것

- 실행 명령(작업 트리 루트에서):
  `go run ./docs/experiments/oracle-seed-offline/runner -data docs/experiments/oracle-seed-offline/data -root <knowledge-data를 담은 폴더> -out <출력 폴더>`.
  `-only sym-74`처럼 문항을 고를 수 있다. 샌드박스에서는 `GOCACHE`를 임시 폴더로 바꿔야 빌드된다.
- 출력은 문항마다 `<id>.json`(두 방식의 전체 후보 목록)과 `summary.json`이다. 시각을 넣지 않았으므로 같은 입력이면
  파일이 같아야 한다. 확인은 작업 8에서 한다.
- core 라벨은 아직 사람이 수용하지 않았다(작업 5). 라벨이 바뀌면 `data/oracle-relayed.json`을 새 파일로 두고
  다시 돌린다.
