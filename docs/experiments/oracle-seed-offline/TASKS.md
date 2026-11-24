# 작업 목록: oracle-seed-offline

설계는 [DESIGN.md](./DESIGN.md)에 있다. 작업이 끝나면 이 표의 상태와 "결과" 칸을 고치고 같은 커밋에 넣는다.

상태 값: `todo`, `doing`, `blocked`, `done`, `dropped`(하지 않기로 함).

## 1단계 (sym 11문항)

| # | 작업 | 상태 | 막힌 이유 또는 결과 |
|---|---|---|---|
| 1 | 설계 문서와 기록 규칙을 저장소에 넣는다 | done | 2026-11-20. [README](../README.md), [DESIGN.md](./DESIGN.md) |
| 2 | 문항의 증상 문장(id와 질문만)을 받는다 | done | 2026-11-20. 문항 파일을 가진 다른 컴퓨터의 세션이 보냈다. [data/questions-symptom.json](./data/questions-symptom.json) |
| 3 | blind 출발 심볼을 고르고 `data/seeds-blind.json`으로 커밋한다 | done | 2026-11-20. 11문항, 심볼 24개. sym-55, sym-75, sym-77은 완전한 blind가 아니다. [data/seeds-blind.json](./data/seeds-blind.json) |
| 4 | 기준 커밋, 수정 커밋, core 라벨, 재생 도구를 받는다 | done | 2026-11-20. 11문항의 기준·수정 커밋(부모 관계 11/11 확인), core 라벨 전사본([data/oracle-relayed.json](./data/oracle-relayed.json)), 파일 sha256. 재생 스크립트는 다른 컴퓨터에 있다 |
| 5 | core 라벨을 사람이 검토하고 수용한다 | done | 2026-11-23. 사용자가 검토를 Claude에게 맡겼다. 이상하면 다시 검토한다. 11문항 중 sym-73 `InjectContracts` 한 곳만 core로 올렸다(core 19개에서 20개). [data/oracle-accepted.json](./data/oracle-accepted.json), 실행기는 이 파일을 읽는다. 검토표는 [tools/review_sheet.py](./tools/review_sheet.py). [연구일지 2026-11-23-01](../research-log/2026-11-23-01-label-acceptance.md) |
| 6 | near 출발 심볼을 고른다 | done | 2026-11-23. 수정 diff와 수용한 라벨을 보고 11문항에 심볼 18개를 골랐다. core는 하나도 없다. [data/seeds-near.json](./data/seeds-near.json). 실행기에 `-seeds`를 더했다. near로 돌리면 core 20개 중 10위 안이 graph 8개, grep 5개였다. [data/near-result.json](./data/near-result.json), [연구일지 2026-11-23-04](../research-log/2026-11-23-04-near-seeds.md) |
| 7 | 빠진 기준 커밋 9개의 cold 그래프를 빌드한다 | done | 2026-11-20. `knowledge-data/agent-eval/runs/20261120-ks-4f02a749-oracle-seed-graphs`. 그래프마다 32~35초, 약 345MB, 오류 0. 경로는 [data/base-commits.json](./data/base-commits.json) |
| 7a | 출발 심볼에서 grep과 graph 후보를 뽑는 오프라인 실행기를 쓴다 | done | 2026-11-20. [runner](./runner/doc.go). 단위 테스트 5개 통과. sym-74 한 문항으로 시험 실행(약 3초). 구현 선택은 DESIGN 4장과 [연구일지 06](../research-log/2026-11-20-06-offline-runner.md) |
| 8 | 같은 입력을 두 번 돌려 출력이 같은지 확인한다 | done | 2026-11-20. 11문항 전부를 두 번 돌려 출력 12개 파일의 sha256이 모두 같았다. 확인 중 출발 심볼 처리 결함 2개를 찾아 고쳤다(`1791abc5`). `knowledge-data/agent-eval/runs/20261120-ks-1791abc5-oracle-seed-determinism`. [연구일지 07](../research-log/2026-11-20-07-determinism-check.md) |
| 9 | 결정 A 비교(grep 대 graph)를 돌린다 | done | 2026-11-23. graph 2승, grep 1승, 무승부 8. 판정하지 않고 2단계로 넘어간다(graph 2/3 = 0.67, 판정선 0.818). [data/decision-a-result.json](./data/decision-a-result.json), 원자료 `knowledge-data/agent-eval/runs/20261123-ks-6f729d46-oracle-seed-decision-a`. [연구일지 2026-11-23-02](../research-log/2026-11-23-02-decision-a-first-run.md) |
| 10 | 실험 1(정답까지의 거리)을 돌린다 | done | 2026-11-23. 출발 심볼이 아닌 core 15개 중 정방향으로 6 hop 안에 닿은 것은 6개(1~3 hop), 양방향은 14개(1~3 hop)다. 양방향 최단 경로 대부분은 연결이 많은 노드(`log.Info` 445, `common.Address` 935, `big.Int` 632)를 거친다. 두 번 돌려 출력이 같았다. [data/experiment1-distance.json](./data/experiment1-distance.json), [연구일지 2026-11-23-03](../research-log/2026-11-23-03-experiment1-distance.md) |
| 11 | 1단계 보고서 `REPORT.md`를 쓴다 | done | 2026-11-23. [REPORT.md](./REPORT.md), [연구일지 2026-11-23-05](../research-log/2026-11-23-05-stage1-report.md) |

## 2단계 (문항 확장)

| # | 작업 | 상태 | 막힌 이유 또는 결과 |
|---|---|---|---|
| 12 | 수정 커밋에서 문항 후보를 뽑는다 | done | 2026-11-23. 109개 중 1단계 수정 10개를 빼고, 비테스트 Go 파일 1~5개를 바꾸며 그중 하나 이상이 기존 파일인 79개를 후보로 남겼다. 이 중 24개는 로그나 에러 문자열을 바꾸고, 22개는 테스트도 바꾼다. [data/stage2-candidates.json](./data/stage2-candidates.json)(해시와 수치만), [tools/stage2_candidates.py](./tools/stage2_candidates.py). 커밋 제목이 든 표는 `knowledge-data/agent-eval/runs/20261123-oracle-seed-stage2-candidates/candidates.md` |
| 13 | 문항마다 증상 문장, blind 출발 심볼, core 라벨을 만든다 | dropped | 2026-11-23. 두 번 시험했다. 코드를 읽는 방식 10문항, 이름 목록만 보는 방식 5문항 모두에서 core 전부가 출발 심볼이었다. 이 설정으로 빌드되는 후보는 69개 중 36개다. 함께 고친 곳 찾기(13a~13d)로 바꿨다. 연구일지 [06](../research-log/2026-11-23-06-stage2-seed-pilot.md), [07](../research-log/2026-11-23-07-stage2-names-only-pilot.md) |
| 13a | 함께 고친 곳 찾기 실험을 설계하고 돌린다(DESIGN 10장) | done | 2026-11-23. 13번 대신 사용자가 정했다. 새 문항 15개에서 graph 7승, grep 4승, 무승부 4. 판정하지 않는다. 같은 파일 참고 방식이 둘보다 나았다. [data/cochange-result.json](./data/cochange-result.json), [연구일지 2026-11-23-08](../research-log/2026-11-23-08-cochange-result.md) |
| 13b | 다른 파일의 정답에 못 닿은 이유를 나눈다(DESIGN 11장) | done | 2026-11-23. 경로 없는 쌍 0. 방향을 섞지 않는 것이 가장 큰 원인이고, 같은 필드 걷기는 선언 16개로 204쌍 중 44쌍에 닿았다(출하 상태 17). stage 3의 호출자 유실과 간선 7종 한정을 확인했다. [data/reachdiag-result.json](./data/reachdiag-result.json), [연구일지 2026-11-23-09](../research-log/2026-11-23-09-reach-diagnosis.md) |
| 13c | hop마다 양방향으로 넓히는 곡선을 재고 확인용 새 자료를 찾는다 | done | 2026-11-23. 곡선은 [data/hopcurve-result.json](./data/hopcurve-result.json)(탐색용). 새 자료는 go-stablenet 2022~2024년 수정으로 정했다(모듈을 내려받지 않음). [연구일지 2026-11-23-10](../research-log/2026-11-23-10-hop-curve-and-new-data.md) |
| 13d | 새 걷기를 go-stablenet 2022~2024년 수정으로 확인한다(DESIGN 12장) | done | 2026-11-23. 판정하지 않았다(새 걷기 5승, grep 2승, 무승부 20). 문항 47개 중 33개만 빌드됐다. [data/confirm-result.json](./data/confirm-result.json), [연구일지 2026-11-23-11](../research-log/2026-11-23-11-confirmation-result.md) |
| 13e | 결정 A를 판정 안 됨으로 닫고 찾은 결함을 고친다 | done | 2026-11-23. `matchNode`와 stage 3 호출자 유실을 고쳤다(브랜치 fix/graph-walk-defects). 빌드 멈춤도 재현해 고쳤다([연구일지 14](../research-log/2026-11-23-14-dispatch-hang-fix.md)). 합칠 준비를 하고([연구일지 15](../research-log/2026-11-23-15-merge-preparation.md)) 로컬 feat/ckg-value-references에 합쳤다([연구일지 16](../research-log/2026-11-23-16-merged-walk-fixes.md)). 간선 종류와 승격 노드 구조는 아래 "다음 작업"으로 옮겼다. [연구일지 2026-11-23-12](../research-log/2026-11-23-12-ties-and-walk-fixes.md) |
| 14 | 실험 2~4를 돌린다 | dropped | 2026-11-23. 증상 문항을 전제로 한 계획이다. 허브와 hop은 hop 곡선(13c)에서 일부 봤다 |
| 15 | 결정 A를 다시 판정하고 보고서를 고친다 | dropped | 2026-11-23. 판정 안 됨으로 닫았다(13e, [REPORT.md](./REPORT.md) 11장) |

## 다음 작업 (2026-11-23 검토, 추천 순서)

| 순서 | 작업 | 상태 | 비고 |
|---|---|---|---|
| 1 | 로그 역추적 조사 | done | 2026-11-24. 6문항 중 4문항이 이어졌고 연결이 모두 다르다. [보고서](../log-backtrace/REPORT.md) |
| 2 | 걷는 간선 설계: 필드를 더하고 빌드 밖 노드는 거치지 않게 하고, 새 자료로 확인한다 | todo | 빈 모듈 캐시로 cosmos-sdk 최신 커밋이 빌드됐다(90초, 패닉 6개를 멈춤 없이 넘김). 먼저 정답에서 지역 선언을 뺀다 |
| 3 | stage 3 목록이 길어진 영향: 뒤 단계 토큰과 grpc-go 검색 평가 | todo | |
| 4 | 승격 메서드를 노드 대신 간선으로 | todo | 그래프 형식 변경. 2번과 함께 하면 다시 빌드를 한 번 줄인다 |
| 5 | `reasons`와 `search-types` | todo | 1, 2번 결과가 필요하다 |
| 6 | 에이전트로 재기 | todo | 원격 하네스 |

시험마다 기능을 더하지 않는다(사용자 지시, 2026-11-24). 특징과 규칙은 [cross-test/FINDINGS](../cross-test/FINDINGS.md). 위 2~5번은 이 규칙에 맞춰 다시 정하기 전까지 시작하지 않는다.

2026-11-24 연구를 마감했다. 위 "다음 작업" 2~6번은 시작하지 않은 채 남는다. 이어 갈지는 [cross-test/CLOSING](../cross-test/CLOSING.md) 4장에서 다시 정한다.
