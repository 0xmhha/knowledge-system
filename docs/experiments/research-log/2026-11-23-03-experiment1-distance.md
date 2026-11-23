# 2026-11-23 연구일지 03: 실험 1, 출발 심볼에서 정답까지의 거리

세션: second-brain-eval-kit에서 연 Claude 세션. 작업 10을 마쳤다.

## 실행

- 실행기에 `-mode distance`를 더했다(`b2136037`). 걷는 방식 셋과 최대 6 hop은 실행 전에 DESIGN 5장에 적었다.
- 원자료: `knowledge-data/agent-eval/runs/20261123-ks-b2136037-oracle-seed-distance`. 두 번 돌려 출력 11개 파일의
  sha256이 같았다. 한 번에 약 13초다.
- 결과 요약: [data/experiment1-distance.json](../oracle-seed-offline/data/experiment1-distance.json).

## 결과

core 20개 중 5개는 출발 심볼 자신이다. 나머지 15개를 센다.

| hop | 정방향 | 양방향 | 양방향, 구조 간선 제외 |
|---|---|---|---|
| 1 | 2 | 3 | 3 |
| 2 | 2 | 8 | 8 |
| 3 | 2 | 3 | 3 |
| 6 hop 안에 없음 | 9 | 1 | 1 |

- **정방향:** 15개 중 6개만 닿았다. 닿은 것은 모두 3 hop 안이다. 거친 간선은 `calls` 10번, `invokes` 1번,
  `dispatches_to` 1번이다.
- **양방향:** 15개 중 14개가 3 hop 안이다. 못 닿은 1개는 sym-56(출발 심볼이 노드로 풀리지 않음)이다.
- **구조 간선을 빼도 같다.** 최단 경로가 애초에 contains, defines, imports, exports를 거치지 않았다.

## 양방향 경로의 모양

양방향 2 hop 경로 8개 중 대부분은 "출발 심볼과 정답이 같은 것을 부르거나 같은 타입을 쓴다"는 모양이다.
가운데 노드의 연결 수가 크다.

| 문항 | 정답 | 가운데 노드 (연결 수) |
|---|---|---|
| sym-58 | `MakeChain` | `log.Info` (445) |
| sym-58 | `New` | `AnzeonEnabled` (131) |
| sym-73 | `initializeGovCouncil` | `common.Address` (935) |
| sym-75 | `isSequenceTooFarAhead`, `isRoundTooFarAhead` | `big.Int.Sub` (71) |
| sym-77 | `RemotesBelowTip` | `big.Int` (632) |
| sym-55 | `newRetrySendingRoundChangeTimer` | `currentLogger` (80) |
| sym-63 | `init` | `SystemContractCode` (10) |

연결 수가 수백인 노드를 거치는 경로는 그 노드를 쓰는 함수 전부로 이어진다. 그래서 "양방향 2 hop 안"이라는 사실만으로는
정답을 가려낼 수 없다. 이 판단은 경로를 본 뒤의 해석이고, 허브 기준을 바꿔 가며 재는 일은 실험 3이다.

## 세 가설에 비춰 보면

1단계는 분포의 모양만 보기로 했다(DESIGN 5장). 판단은 2단계 문항으로 한다.

- **가설 1(정답 대부분이 2 hop 안):** 양방향으로는 맞다(11/15). 그러나 경로 대부분이 허브를 거친다.
- **가설 3(닿지 않는 정답이 많다):** 정방향으로는 맞다(9/15가 6 hop 안에 없음). 정답이 출발 심볼을 부르는 쪽이거나,
  같은 것을 함께 쓰는 형제인 경우가 많다.
- 그러니 1단계 자료는 "방향을 섞어 걷되 허브를 거르는 규칙"이 필요하다는 쪽을 가리킨다. 확인된 결론은 아니다.

## 다음 세션이 알아야 할 것

- 남은 1단계 작업은 6번(near 출발 심볼)과 11번(1단계 보고서)이다.
- 이 실험의 hop은 문장 노드를 거치는 것도 하나로 센다. 이번 최단 경로에는 문장 노드가 없었다.
