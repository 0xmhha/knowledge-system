# 실제 프로젝트 비교 자료

고정 원문의 source excerpt 비교다. 명시한 WBFT/Clique 구현을 실행하거나 실제 네트워크의 정책·staking·finality·성능을 판정한 결과가 아니다. 모든 매핑은 미검토 후보다.

| 공통 개념 | go-stablenet | go-wemix |
|---|---|---|
| staking | entry: not_collected; exit: not_collected; penalty: not_collected | entry: not_collected; exit: not_collected; penalty: not_collected |
| staking-policy | withdrawal-conditions: not_collected; limits: not_collected; exceptions: not_collected | withdrawal-conditions: not_collected; limits: not_collected; exceptions: not_collected |
| consensus | participants: similar; quorum: similar; fault-assumptions: unknown | participants: similar; quorum: unknown; fault-assumptions: unknown |
| finality | condition: unknown; rollback: unknown; settlement: unknown | condition: not_collected; rollback: not_collected; settlement: not_collected |
| mempool-admission | admission: similar; ordering: unknown; replacement: unknown; eviction: unknown; concurrency: unknown | admission: similar; ordering: unknown; replacement: unknown; eviction: unknown; concurrency: unknown |
| transaction-ordering | selection: unknown; ordering: similar; fairness: unknown | selection: unknown; ordering: similar; fairness: unknown |

본문·인용·매핑 이유와 양쪽 비교 후보는 같은 폴더의 matrix-*.json 및 calls.json에 보존했다. not_collected/unknown은 기능 부재가 아니다. 근거 없는 우월성·보안성·TPS·L2 settlement를 만들지 않는다.
