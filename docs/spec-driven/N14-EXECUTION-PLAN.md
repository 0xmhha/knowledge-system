# N14 실제 변경 수용 실행 계획

2026-10-10 · **계획 작성 / 실행 전 / N14 미완료**. 전체9/18, 남은9개.

## 1. 무엇을 증명하려는가

N14는 실제 코드 변경 하나가 "변경 → 검토된 기준의 실제 시험 실행 → 사람의 기준별 수용 → 승격 또는 반려·rollback"을 거치는지 보이는 작업이다. 시험 통과(test pass), 검토된 trace의 snapshot 결합 실행(linked run), 관계 검증(verified), 사람 수용(accepted)을 각각 따로 기록해야 한다. N12에서 기준(criterion)별 수용 결정은 0건이었다.

## 2. 대상 변경

**`7c57a388` perf(semantic): verify the retained archive once per projection load**를 쓴다. N11 DEV 지연을 고치려고 만든 실제 변경이다. 수용 시험용으로 만든 변경이 아니다.

- 바뀐 코드: `internal/system/semantic/validate.go`(`ValidateRetainedSources`), `internal/setup/source_read.go`(`ReadRetainedFiles` 추가).
- 이 함수는 `PutAlignedRetained`와 `ActivateAlignedRetained`가 부른다. 그래서 기준 **ac-source-evidence**("다른 snapshot의 code anchor를 가진 관계는 네 호출 모두에서 snapshot mismatch 오류로 거부되고 active pointer는 그대로")의 코드 경로를 실제로 바꾼다.
- 이 기준의 검토된 시험은 F3-T12 `TestForeignCodeAnchorPromotionPreservesActiveDataset`이다(N12 linked run에서 통과). F3-T10은 검토된 경로가 없어 거부되었고, 이번에도 쓰지 않는다.
- 변경과 함께 추가한 `TestRetainedSourceValidationRejectsForeignArchiveCoordinates`는 검토된 trace 관계가 아니다. 그래서 linked run으로 세지 않고 test pass로만 기록한다.

## 3. 실행 순서

1. **기준선 복원.** N12 보관본 `held-reviewed-candidate.tar.gz`(source `0b9085e7`, dataset `daf9608b`, 검토 projection 활성)를 새 소유 임시 경로에 재생한다. 원 보관본과 원자료는 바꾸지 않는다.
2. **결과 후보 생성.** `7c57a388`이 들어간 현재 소스를 같은 versioned dataset에 hold-for-review 후보로 capture·색인한다(embedding 약 70회). current는 기준선에 그대로 둔다.
3. **projection 재결합.** 같은 검토 판정을 새 snapshot에 다시 결합한다. 근거 span의 바이트가 바뀐 관계는 자동으로 넘기지 않고 재검토 대상으로 표시한다. 바뀐 파일이 검토된 span에 걸리는지는 실행 시점에 diff로 확인한다.
4. **patch 등록.** `cks patch register --dataset <d> --version <result> --patch-id <id>`로 변경 파일과 archive SHA가 묶인 불변 attempt를 만든다.
5. **linked run.** 결과 후보에서 `cks semantic test --go-test-exact`로 F3-T12를 실행하고 `cks patch record-run`으로 붙인다. 다른 세 기준의 검토된 시험도 회귀로 실행한다. 무관한 시험, command만 있는 run, stale snapshot, 불일치 run이 거부되는지 대조군으로 확인한다.
6. **기준별 결정.** `cks patch decide --criterion-id ac-source-evidence --outcome approved|rejected --reviewer <이름> --reason <근거>`. 사람의 활동 시간은 측정하지 않았다면 null로 남긴다.
7. **승격과 반려.** 승인되면 `cks patch promote`로 승격하고 current와 원문 신원을 재생한다. 반려 경로도 보이기 위해 별도 attempt에 rejected 결정을 기록하고 promote가 거부되며 current가 그대로인지 확인한다.
8. **보관과 재생.** 원래 source/index/store 없이 보관본으로 attempt, run, decision, 승격 상태를 재생한다.

## 4. 사용자 결정이 필요한 것

N14-D는 "별도 실제 사람이 기준별 Given/When/Then을 승인/반려하고 검토자를 기록"을 요구한다. 지금까지 쓴 "opus5.5 검토" 라벨은 사용자가 지정한 AI 검토 라벨이다. 그래서 이 라벨로 결정을 기록하면 N14-D의 "별도 실제 사람" 조건은 충족되지 않는다. 다음 중 하나를 사용자가 정해야 한다.

- 사용자 본인이 F3-T12 결과를 보고 기준별로 승인·반려하고, 기록할 이름이나 표기를 정한다.
- AI 라벨로 기록하고, N14-D는 미충족으로 남긴다.

검토자 이름은 추정해서 채우지 않는다.

## 5. 실행 시점과 비용

2–8단계는 색인과 Go 시험으로 CPU를 쓰므로 N11 DEV3·FINAL 측정과 겹치지 않게 실행한다. 다른 세션 작업과 함께 돌리는 것은 측정이 아니므로 문제없다. 모델 호출은 색인 embedding 약 70회다.

## 6. 수용 조건과의 대응

| 조건 | 계획 단계 |
|---|---|
| N14-A ADR→요구→기준→code/test를 검토된 source·lock·snapshot·canonical에 결합 | 1, 3 |
| N14-B 기준선/결과 소스 변경을 hold-for-review 후보와 불변 attempt에 결합, current 보존 | 2, 4 |
| N14-C 검토된 trace의 exact test·출력SHA·spec version·snapshot run 기록, 무관/stale 거부 | 5 |
| N14-D 별도 실제 사람의 기준별 승인/반려와 이유·검토자·시각 | 6 (사용자 결정 필요) |
| N14-E 재색인/승격·반려·rollback·원문 신원 재생, pass/linked/verified/accepted 구분 | 7, 8 |

**남은 전체9개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
