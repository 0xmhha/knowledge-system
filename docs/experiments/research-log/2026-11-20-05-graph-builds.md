# 2026-11-20 연구일지 05: 기준 커밋 9개의 그래프 빌드

## 한 일

sym-55, sym-77을 뺀 기준 커밋 9개를 cold로 빌드했다.

- 위치: `knowledge-data/agent-eval/runs/20261120-ks-4f02a749-oracle-seed-graphs` (저장소 밖). 빌드 스크립트
  `build.sh`와 로그 `build-all.log`도 거기 있다.
- ckg: `runs/20261120-ks-4f02a749-graphs/bin/ckg.real`을 읽기 전용으로 실행했다. sha256 앞자리 `4a6fc578d008c0ce`가
  그 폴더의 `ckg.sha256`과 같다. knowledge-system `4f02a749`로 빌드한 바이너리다.
- 다른 세션의 그래프 폴더에는 쓰지 않았다.

## 결과

| 값 | 결과 |
|---|---|
| 빌드 시간 | 그래프마다 32~35초 (scope 단계 제외, build.log 시각) |
| 크기 | 그래프마다 345~346MB, 9개 합계 3.0GB |
| 오류 | 0 |
| 경고 | 그래프마다 9~17개. 기존 두 그래프(9개, 13개)와 같은 범위 |

경고 종류는 세 가지였다: `policy governs[] target not found` 93건, `go packages loaded with problems` 9건,
`duplicate node ids: one node of each pair is lost` 6건(9개 합계).

## 알아 둘 것

- 노드와 간선 수는 build.log의 값과 DB를 센 값이 조금 다르다. `8d7930a4`에서 build.log는 179,344개, 다른 세션이 DB에서
  센 값은 179,386개다. 비교할 때는 같은 방법으로 센 값끼리 비교한다.
- `319b84d1`(sym-74 기준)과 `e5a6e9e1`(sym-75 기준)은 노드 181,995개, 간선 496,607개로 같다. `e5a6e9e1`은 sym-74의 수정
  커밋이므로, 그 수정이 노드와 간선의 수를 바꾸지 않았다는 뜻이다. 내용까지 같은지는 확인하지 않았다.
- 11-20 앞선 빌드(46~48초)보다 빨랐다. 원인은 확인하지 않았다. Go 모듈 캐시가 이미 차 있던 것이 한 가지 후보다.
