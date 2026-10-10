# N10 한 요청 안에서 archive 검증 공유

2026-10-10 · **코드 변경·시험·DEV 개발 측정 완료 / N10-C는 새 독립 FINAL 필요 / N10 미완료**. 전체13/18, 남은5개.

## 1. 왜 바꿨는가

독립 FINAL에서 warm p95비가 기준 외 7개 arm 모두 1.37–1.97로, 동결 기준 1.25를 넘었다([FINAL 결과](./N11-FINAL-RESULT.md)). backend 시간은 arm 사이에 같았고, 늘어난 시간은 backend 밖에서 생겼다.

## 2. 무엇이 원인이었는가

`setup.verifyVersionIdentityContext`에 임시 계측(커밋하지 않음)을 넣었다. 그리고 DEV 1문항으로 arm마다 4회씩 호출해, 요청 처리 중에 archive 전체 검증이 몇 번 도는지 셌다([기록](../../system/eval/b0-knowledge-system/refactoring-n10-request-verification-2026-10-10/verification-profile.json)). 검증 1회는 약 150–230ms였다.

| arm | 요청당 검증 | 어디서 |
|---|---|---|
| baseline_off | 3 | handler 1, 근거 batch 2 |
| ontology off(concept_text·relations·combined) | 5 | 위 + composer의 ontology projection 2 |
| baseline_on | 6 | 위 기준 + knowledge manifest 1, trace projection 2 |
| ontology on | 8 | 모두 |

이 차이는 FINAL에서 측정한 backend 밖 지연의 증가분과 맞았다. 기준 대비 약 +0.35초, +0.55초, +0.9초다.

## 3. 무엇을 바꿨는가

`setup.BeginRequestVerification`을 추가했다(`37302b35`). 동작은 다음과 같다.

- v2 handler는 요청을 시작할 때 archive를 한 번 검증한다. 이 결과를 context로 넘긴다.
- 같은 요청 안에서 같은 version을 읽을 때는 이 결과를 재사용한다. 해당 경로는 근거 batch, projection 원문 검증, knowledge manifest, pack, trace 원문이다.
- 응답을 돌려주기 전에 `Finish`가 archive 전체를 다시 검증한다. 실패하면 응답 대신 오류를 돌려준다.
- 재사용은 `Finish`나 `Close`에서 끝난다. 다른 version, context 밖의 읽기, 끝난 요청은 각자 검증한다.
- 읽는 blob은 지금처럼 매번 다시 읽고 hash를 확인한다.

"검증 결과를 요청 사이에 재사용하지 않는다"는 기존 보안 계약은 그대로 유지된다. 요청 하나에서 검증하는 횟수는 모든 arm에서 2회로 같아졌다.

## 4. 어떻게 확인했는가

**시험 3개를 추가했다.**
1. 요청 안에서는 손상된 미사용 blob이 보이지 않는다. 같은 손상을 요청 밖의 읽기와 `Finish`는 잡는다. 끝난 뒤의 읽기는 재사용하지 않는다.
2. 요청 안에서도 손상된 blob은 읽기를 거부한다.
3. 다른 version은 재사용하지 않는다. `Close`, 취소, legacy version도 확인한다.

**변이 4개를 시험이 모두 잡았다.** 요청 안 재사용 제거, `Finish` 재검증 생략, 종료 뒤 재사용, version 비교 제거다([기록](../../system/eval/b0-knowledge-system/refactoring-n10-request-verification-2026-10-10/mutation-checks.txt)).

**관련 패키지가 통과했다.** 17개 패키지(setup, semantic, evidencev2, mcp, knowledgepack, patch, cmd/cks)의 race, vet, gofmt가 통과했다([출력](../../system/eval/b0-knowledge-system/refactoring-n10-request-verification-2026-10-10/related-race.txt)).

**계측으로 다시 셌다.** 변경 뒤에는 모든 arm이 요청당 2회였다. 같은 요청 32개 slot의 인용(파일·줄·내용 SHA), semantic 블록, ontology 표시가 변경 전과 모두 같았다.

## 5. DEV 개발 측정

N11 runner가 부르는 `cks eval matrix`를 직접 실행했다([스크립트](../../system/eval/b0-knowledge-system/refactoring-n10-request-verification-2026-10-10/run-dev4.py)). DEV3 요청 7개, 동결 runtime config, protocol 반복 횟수(warmup 2, retrieval 5, warm 20, cold 3)를 그대로 썼고 binary만 `cks-37302b35`로 바꿨다. protocol과 검토 기록은 rebind하지 않았다. 따라서 이 측정은 동결 평가가 아니라 개발 측정이다.

| arm | DEV3 p95비 (`7c57a388`) | DEV4 p95비 (`37302b35`) | DEV4 p50: backend / 그 밖 |
|---|---|---|---|
| baseline_off | 1.00 (1.57초) | 1.00 (1.11초) | 0.57 / 0.27초 |
| baseline_on | 1.71 | 0.92 | 0.55 / 0.29초 |
| concept_text_off | 1.33 | 0.95 | 0.59 / 0.29초 |
| concept_text_on | 1.70 | 0.98 | 0.59 / 0.31초 |
| relations_off | 1.20 | 0.92 | 0.57 / 0.29초 |
| relations_on | 1.66 | 0.95 | 0.57 / 0.32초 |
| combined_off | 1.24 | 0.92 | 0.58 / 0.29초 |
| combined_on | 1.73 | 0.97 | 0.59 / 0.32초 |

- 8개 arm의 Recall@10과 MRR은 문항별로 DEV3와 같았다.
- backend 밖 p50이 줄었다. baseline_off는 0.47초에서 0.27초로, combined_on은 1.44초에서 0.32초로 줄었다.
- cold process p50은 3.2–3.5초다.

## 6. 한계

- **실행 중 오류.** DEV4 matrix는 exit 1, report는 `partial`로 끝났다.
  - 측정 1,680행은 모두 기록되었고 실패한 행은 0이다.
  - 오류는 측정이 끝난 뒤 MCP 서버를 닫을 때 생긴 강제 종료 9건(`signal: killed`)이다. warm 8건, cold 1건이다. 서버가 stdin을 닫은 뒤 2초, SIGTERM 뒤 3초 안에 끝나지 않았다.
  - 채점기는 이 때문에 `not_evaluable_as_complete`로 표시한다.
  - 같은 부하에서 이전·새 binary를 1요청 규모로 두 번씩 번갈아 실행했을 때는 각각 48번 닫아서 강제 종료가 0건이었다([기록](../../system/eval/b0-knowledge-system/refactoring-n10-request-verification-2026-10-10/close-ab-check.txt)). 원인은 확정하지 못했다. DEV2도 높은 부하에서 같은 증상을 보였다.
- **부하.** 부하 평균은 시작 시 12.0/10.2/9.7, 종료 시 6.7/10.7/11.2로, DEV3(약 5)보다 높았다. 비율은 같은 실행 안의 회전 쌍에서 계산하므로 같은 부하를 나눠 받는다.
- **스크립트 실수.** 첫 두 번의 시도는 제 스크립트의 저장소 경로 계산 오류로 config를 찾지 못해 1초 만에 실패했다. 이 기록은 `dev4-attempt1-*`, `dev4-attempt2-*`로 보존했다.
- **N10-C는 아직 충족되지 않는다.** 이번 FINAL 10문항은 이미 관측했다. 동결 protocol은 관측 뒤에 바꾼 코드는 새 DEV와 새 독립 FINAL로 판정하라고 정한다. 그래서 DEV4의 0.92–0.98을 N10-C의 수용 근거로 쓰지 않는다.

- [ ] N10-C(새 독립 FINAL에서 p95비 1.25 판정 필요).

**남은 전체5개:** N08 검색 품질, N09 기권, N10 비용·지연(새 FINAL로 p95 판정), N16 신뢰·권한(서명 키), N18 기본값·출시 판정.
