# N14 실제 변경의 기준별 수용·승격·rollback

2026-10-10 · **N14 수용 조건 A–E 충족 / N14 완료**. 전체10/18, 남은8개.

## 1. 무엇을 수용했는가

실제 변경 `7c57a388`(perf(semantic): verify the retained archive once per projection load)을 대상으로 했다. N11 DEV 지연을 고치려고 만든 변경이며 수용 시험용으로 만든 것이 아니다. 이 변경은 `ValidateRetainedSources`를 바꾸고, 이 함수는 `PutAlignedRetained`와 `ActivateAlignedRetained`가 부른다. 따라서 기준 ac-source-evidence의 코드 경로를 실제로 바꾼다. 계획은 [N14 실행 계획](./N14-EXECUTION-PLAN.md)에 있다.

## 2. 어떻게 진행했는가

1. **기준선과 결과 후보.** N12 검토 후보(source `0b9085e7`, dataset `daf9608b`)를 소유 경로에 복사해 base로 두고 current로 지정했다. N12 원자료는 바꾸지 않았다. 같은 소스에 바뀐 4개 파일만 넣은 commit `79a4f5ff`을 `--hold-for-review`로 색인했다. 결과는 dataset `86cbe48a`, snapshot `b4d63460`이다. embedding HTTP는 74회(입력 767, 실패 0)였다. 후보 시험 gate(setup·semantic·evidencev2 패키지)가 통과했고, current는 base 그대로였다.
2. **patch attempt.** `cks patch register`가 base와 result archive를 비교해 바뀐 파일 4개와 SHA를 담은 불변 attempt `retained-batch-7c57a388`을 만들었다.
3. **검토 판정 이어받기.** N12의 검토된 판정을 결과 snapshot에 다시 결합했다. 바뀐 파일에 걸린 검토 근거는 없었다. 근거 ID와 섹션 ID는 snapshot에서 파생되므로 내용 키(종류·경로·줄·내용 SHA·canonical)로 바꿔 비교했다. 개념23·요구3·관계43·주장6·섹션27·근거83이 모두 같았다([대조](../../system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10/review-carryover-check.json)). 세 요구 trace는 모두 linked였다.
4. **linked run.** 생산 CLI `cks semantic test --go-test-exact`로 기준별 검토 시험 10개를 결과 snapshot에서 실행했다. 9개가 통과했고 snapshot이 일치했다. F3-T10은 N12와 같이 검토 경로 없음으로 거부되었다.
5. **run 기록과 거부 대조.** 기준별로 1개씩(F3-T03, F3-T05, F3-T12, F3-T15) `cks patch record-run`으로 기록했다. 대조군 4개는 모두 거부되었고, 원인은 API로 직접 확인했다([원인](../../system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10/record-run-refusal-causes.txt)). 이전 snapshot의 stale run은 "stale/not bound", 기준을 바꾼 run과 무관한 시험은 "reviewed code/test path 불일치", 시험이 관측되지 않은 run은 "observed exact test result 필요"였다.
6. **반려 대조.** 같은 result 버전에 별도 patch ID를 등록하고 같은 run을 붙였다. 한 기준에 agent가 쓴 rejected 결정(사람 판정 아님으로 명시)을 기록하자 promote가 거부되었고 current는 base로 남았다.
7. **사람 결정과 승격.** 사용자가 채팅으로 "표기: opus / 초안대로 모두 승인"이라고 답했다. 그래서 기준 4개 모두 reviewer `opus`, outcome approved로 `cks patch decide`에 기록했다. 이유는 agent가 run 결과로 작성한 초안을 사용자가 그대로 채택한 것이다([기록](../../system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10/user-decision.json)). 사용자의 활동 시간은 측정하지 않았으므로 null이다. `cks patch promote`가 성공했고 current는 base에서 result로 바뀌었다.
8. **rollback.** `cks rollback base`는 `version "base" requires explicit patch review`로 거부되었다. base는 N12의 hold 후보를 복사한 것이라 검토 해제(release)된 적이 없다. 이 거부는 제품의 안전 동작이며, 원인은 이 시험의 fixture 구성에 있다. 그래서 rollback은 해제된 두 버전 사이에서 확인했다. base 소스로 hold 없는 일반 버전 `released-base`를 만들어 current로 올린 뒤, `cks rollback result`로 승인된 result에 되돌렸다. 되돌린 current의 신원과 retained source가 검증되었고, 검토된 projection이 current 색인에 다시 결합되어 세 요구 모두 linked였다.
9. **보관과 재생.** base·result 버전, patch 기록, 검토 store, 소스 bundle을 [보관본](../../system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10/accepted-result.tar.gz)(2,939파일, 30,971,570byte)으로 만들었다. 원래 source·dataset·store를 숨기고 보관본만으로 재생했다. 모든 파일 SHA가 같았고, 신원·원문·projection 재검증과 `cks patch status`가 통과했다.

## 3. 상태 구분

| 구분 | 상태 |
|---|---|
| test pass | 후보 시험 gate 3패키지 통과, 관련 패키지 race 통과(N10 변경 기록) |
| semantic linked run | 결과 snapshot에서 9/10 통과, F3-T10 거부(검토 경로 없음) |
| relation verified | 관계43·개념23·요구3·주장6, 검토 라벨 "opus5.5 검토"(N12, AI 라벨) |
| criterion accepted | 4/4 approved, reviewer `opus`(사용자 판정, 사용자가 준 표기) |
| 승격 | 소유 dataset에서 base→result. 사용자 운영 dataset 활성화는 아니다 |

## 4. 한계

- 결정 이유는 agent가 초안을 쓰고 사용자가 그대로 채택한 것이다. 사용자가 기준별 Given/When/Then을 따로 다시 썼다는 기록은 없다. 표기 `opus`는 사용자가 정한 값이며, 이 기록은 사람(사용자)의 판정으로 다룬다.
- knowledge lock에 새로 들어간 승인 항목은 0개다. 정책 담당자가 없어 정책 instance가 없기 때문이며, N12와 같은 한계다.
- 소유 임시 경로에서 한 시험이다. 운영 이식성과 운영 소비자 수용은 N15–N17의 범위다.
- 반려 대조의 rejected 결정은 agent가 만든 대조이며 사람 판정으로 세지 않는다.

- [x] N14-A 검토된 ADR·요구·기준·code/test 관계를 결과 snapshot과 CKG canonical에 결합(승인 overlay 항목0).
- [x] N14-B base/result 변경을 hold-for-review 후보와 불변 patch attempt·archive SHA에 결합, current 보존.
- [x] N14-C exact test run·출력SHA·spec version·snapshot run 기록, stale/불일치/무관/command-only 거부.
- [x] N14-D 사용자의 기준별 승인과 이유·검토자 표기·시각 기록, 활동 시간 null.
- [x] N14-E 승격·반려 대조·rollback·보관본 원문 신원 재생, pass/linked/verified/accepted 구분.

**남은 전체8개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
