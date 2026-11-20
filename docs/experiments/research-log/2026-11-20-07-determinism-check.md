# 2026-11-20 연구일지 07: 결정성 확인

세션: second-brain-eval-kit에서 연 Claude 세션. 작업 8.

## 한 것

- 실행기를 11문항 전부에 두 번 돌리고, 출력 파일 12개(문항 11개와 `summary.json`)의 sha256을 비교했다.
- 측정값(도달률, recall@10)은 보지 않았다. core 라벨을 사람이 검토하기 전이라 결정 A를 판정하지 않는다.
  출발 심볼과 core가 노드로 옮겨졌는지만 보았다.

## 알게 된 사실

- **처음 실행(`f878cbe6`)에서 출발 심볼 처리 결함 2개를 찾았다.** 두 번의 출력은 같았지만 내용이 틀렸다.
  - sym-70의 출발 심볼 `EffectiveGasPrice`는 구조체 필드다. 실행기가 필드 노드를 받지 않아 이 심볼이 조용히 빠졌다.
  - 한 범위(span)에 노드가 여럿인 경우가 있었다. 예: `core/state/statedb.go` 434~439줄에
    `state.StateDB.SetBlacklisted`, `logger.dummyStatedb.SetBlacklisted`, `js.dummyStatedb.SetBlacklisted`가
    함께 있다. 뒤의 둘은 canonical id가 비어 있다. ckg가 다른 패키지 타입의 메서드에 이 파일과 범위를 붙인 것으로
    보인다. 왜 그런지는 확인하지 않았다. `params/config.go`의 `CheckCompatible`도 같았다.
- 고친 실행기(`1791abc5`)는 필드도 출발 심볼로 받는다. 한 범위의 노드는 하나만 넘긴다(canonical id가 있는 것,
  다음은 id가 작은 것). stage 3은 citation을 범위로 찾으므로 graph 쪽 결과는 이 선택과 무관하다.
- 고친 실행기로 두 번 돌린 출력 12개의 sha256이 모두 같았다. 한 번에 약 66초 걸렸다. 같은 시각에 betterleaks
  히스토리 검사가 돌고 있었다. 고치기 전 실행은 약 20초였다.
- 문항별 해석 결과(`summary.json`):

| 문항 | 출발 심볼 | 출발 노드 | core 해석 | core가 출발 심볼 |
|---|---|---|---|---|
| sym-55 | 3 | 3 | 2/2 | 0 |
| sym-58 | 2 | 2 | 2/2 | 0 |
| sym-67 | 2 | 2 | 2/2 | 1 |
| sym-70 | 2 | 4 | 2/2 | 1 |
| sym-74 | 2 | 2 | 1/1 | 1 |
| sym-77 | 2 | 2 | 2/2 | 0 |
| sym-56 | 2 | 0 | 1/1 | 0 |
| sym-63 | 3 | 3 | 2/2 | 0 |
| sym-69 | 2 | 2 | 2/2 | 2 |
| sym-73 | 3 | 3 | 1/1 | 0 |
| sym-75 | 1 | 1 | 2/2 | 0 |

- **세 문항은 결정 A에서 반드시 비긴다.** sym-74와 sym-69는 core 전부가 출발 심볼이다. sym-56은 출발 심볼이 없다.
  그래서 승패가 갈릴 수 있는 문항은 많아야 8개다. 판정선은 무승부를 뺀 문항 중 9/11(약 0.818) 이상이다.
  승패가 갈린 문항이 8개면 7개 이상(0.875), 7개면 6개 이상(0.857)이 같은 쪽이어야 넘는다.
- sym-70의 출발 노드 4개는 `DeriveFields` 1개와 `EffectiveGasPrice` 필드 3개
  (`Receipt`, `receiptMarshaling`, `storedReceiptRLP`)다.

## 다음 세션이 알아야 할 것

- 판정에 쓰는 실행기 커밋은 `1791abc5`다. `f878cbe6`의 출력은 결함 기록으로만 남겨 두었다
  (`runs/20261120-ks-f878cbe6-oracle-seed-determinism`).
- 작업 9는 core 라벨 검토(작업 5) 뒤에 돌린다. 사용자와 2026-11-20에 정했다.
