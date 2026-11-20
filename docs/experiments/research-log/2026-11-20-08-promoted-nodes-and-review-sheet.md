# 2026-11-20 연구일지 08: 승격 메서드 노드와 core 검토표

세션: second-brain-eval-kit에서 연 Claude 세션. 작업 5 준비.

## 알게 된 사실

### 한 범위에 노드가 여럿인 이유

- **그래프를 만드는 쪽은 설계대로다.** `internal/graph/parse/golang/promoted.go`는 승격 메서드(Go에서 구조체가 다른 타입을
  품으면 그 타입의 메서드가 바깥 타입의 메서드가 되는 것)마다 노드를 따로 만든다. 이름은 바깥 타입으로
  (`logger.dummyStatedb.SetBlacklisted`), 파일과 범위는 원래 메서드의 것으로 붙인다. 주석에 적힌 목적은
  `find_symbol("T.M")`이 찾히게 하는 것이다. 기준 커밋 `8b895799`에서 `eth/tracers/logger/logger_test.go`와
  `eth/tracers/js/tracer_test.go`의 `dummyStatedb`는 `state.StateDB`를 품는다.
- **승격 노드에는 호출 간선이 없다.** 1 hop 이웃을 세면 이렇다(`NeighborhoodByIDs`, 기준 커밋 `8b895799`, `db7c4b43`).

| 노드 | 정방향 간선 | 역방향 간선 |
|---|---|---|
| `state.StateDB.SetBlacklisted` | 6 | 14 |
| `logger.dummyStatedb.SetBlacklisted` | 0 | 1 |
| `params.ChainConfig.CheckCompatible` | 17 | 5 |
| `wbft.chainConfigWrapper.CheckCompatible` | 0 | 1 |

- **결함은 그래프를 읽는 쪽에 있다.** `ckgclient.matchNode`(`internal/system/ckgclient/real.go`)는 citation과 범위가 같은
  선언 노드 중 저장 순서로 첫 노드를 고른다. 첫 노드가 승격 노드면 걷기가 간선 없는 노드에서 시작한다.
  `CheckCompatible`을 citation으로 1 hop 걸으면 정방향 0개, 호출자 0개가 나왔다. 진짜 노드는 정방향 간선이 17개다.
- 이 실험의 출발 심볼 중 두 개가 이 결함에 걸린다. sym-58의 `CheckCompatible`과 sym-73의 `SetAuthorized`다.
  두 문항에서 graph 방식은 이 출발 심볼로부터 아무것도 얻지 못한다. grep 방식은 영향이 없다.
- composer stage 3과 citation을 받는 다른 경로도 같은 함수를 쓴다. 에이전트 실행에서 얼마나 자주 걸렸는지는
  세지 않았다.

### core 검토표

- `docs/experiments/oracle-seed-offline/tools/review_sheet.py`로 검토표를 만들었다. 문항마다 증상, 수정 커밋, oracle 위치 전부(core 먼저), core 위치를 바꾼
  diff hunk를 모았다. 출력은 `knowledge-data/agent-eval/runs/20261120-oracle-seed-core-review/core-label-review.md`
  (1,010줄)다. go-stablenet 코드가 들어가므로 저장소에는 넣지 않았다.
- core 19개 모두 수정 커밋의 hunk가 그 위치를 바꾼다. "바뀌지 않은 core"는 없다.
- **새 파일은 core가 될 수 없다.** oracle 위치는 기준 커밋의 줄 번호다. 그래서 수정 커밋이 새로 만든 파일은 정답에 들어가지
  않는다. 예를 들어 sym-63에서 가장 큰 변경은 새 파일 `systemcontracts/solidity/v2/GovMinter.sol`(1,028줄)이다.

## 바로잡는 것

- 2026-11-20 대화에서 Claude는 "sym-63의 core `SetConfigFromChainConfig`는 증상과 관련이 없어 보인다"고 말했다.
  철회한다. sym-63 증상은 "다음 네트워크 업그레이드부터 돌려받을 수 있게"를 묻는다. 수정은 이 함수에 Boho 하드포크의
  시스템 컨트랙트 업그레이드를 등록한다. 관련이 있다. core로 맞는지는 사람의 검토로 정한다.

## 다음 세션이 알아야 할 것

- 사용자가 검토표의 결정 칸을 채우면 `data/oracle-accepted.json`을 새로 만든다. `oracle-relayed.json`은 고치지 않는다.
- 결정 A는 ckg를 지금 그대로 잰다. `matchNode` 결함을 실험 전에 고칠지는 아직 정하지 않았다.
