# N10 projection 검증의 archive 재검증 횟수 축소

2026-10-10 · **코드 변경·시험 완료 / 깨끗한 DEV 측정 대기 / N10 미완료**. 전체9/18, 남은9개.

## 1. 왜 바꿨는가

첫 N11 DEV에서 지식 팩과 ontology arm의 warm p95가 baseline_off의 1.85–3.29배였다. backend 호출 시간은 arm 사이에 거의 같았다. 늘어난 시간은 projection을 읽을 때마다 하는 검증에서 생겼다([DEV 결과](./N11-DEV-RESULT.md)).

## 2. 무엇이 원인이었는가

평가 데이터셋에서 단계별로 시간을 쟀다. `ValidateRetainedSources` 하나가 약 1.0초였다. 이 함수는 근거 파일마다 `ReadRetainedFile`을 불렀고, 그때마다 archive 전체 신원 검증(warm 약 155ms)이 다시 돌았다. 근거 파일 5개면 검증도 5회였다([측정](../../system/eval/b0-knowledge-system/refactoring-n10-retained-batch-2026-10-09/profile.json)).

## 3. 무엇을 바꿨는가

`setup.ReadRetainedFiles`를 추가했다. 이미 있던 `VisitRetainedLineSpans`와 같은 방식으로, 요청 단위 batch의 앞뒤에서 archive를 한 번씩 검증한다. 각 파일 blob은 여전히 다시 읽고 hash를 확인한다. `ValidateRetainedSources`는 근거 파일을 이 batch로 읽는다. 검증 결과를 요청 사이에 재사용하지 않는다는 기존 보안 계약은 그대로다. 그래서 projection 한 번 읽는 데 검증 2회 비용은 남는다. 같은 측정에서 약 1.0초가 약 0.34초가 되었다.

## 4. 어떻게 확인했는가

- batch 읽기 시험: 단일 읽기와 바이트·신원 동일, 잘못된 경로·없는 파일·빈 경로·origin 거부, 취소 거부, 선택되지 않은 blob 손상 거부.
- 좌표 불일치 거부 시험(snapshot·source mode). 이 비교는 변경 전에도 있었지만 시험이 없었다.
- 관련 8패키지 race, vet, gofmt 통과([출력](../../system/eval/b0-knowledge-system/refactoring-n10-retained-batch-2026-10-09/related-race.txt)).
- 변이: M1(좌표 비교 제거)은 새 시험이 잡는다. M2(batch 뒤 재검증 제거)는 살아남는다. batch 도중 archive를 바꾸는 시험이 없기 때문이며, 기존 `VisitRetainedLineSpans`도 같은 빈틈이 있다([기록](../../system/eval/b0-knowledge-system/refactoring-n10-retained-batch-2026-10-09/mutation-checks.json)).

## 5. 한계와 다음

이 binary로 돌린 DEV2는 외부 부하(load average 7→42)로 오염되어 지연 측정으로 쓰지 않는다. 깨끗한 DEV3와 FINAL은 사용자가 정한 한가한 시간에 실행한다. p95비 1.25 판정은 그 결과로 한다. 이 변경은 N14의 실제 변경 대상으로도 쓴다([계획](./N14-EXECUTION-PLAN.md)).

**남은 전체9개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
