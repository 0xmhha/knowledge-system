# N12 외부 snapshot 거부 gate의 타입 구분

2026-10-09 · **제품 오류 구분 보강·RED→GREEN 검증 / 사람 판정·N12 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 배경과 문제

[관계 사전검토](./N12-CRITERION-GAP-PREREVIEW.md)에서 F3-T08/T11의 구현·시험 대상을 code anchor gate로 바꾸자는 권고가 나왔다. 그런데 `Projection.Validate`(validate.go:86)와 `checkCodeAnchor`(ckg_anchor.go:94)가 모두 "crosses project or snapshot" 문구로 거부했고, 시험은 이 부분 문자열만 비교했다. 그래서 외부 snapshot 앵커를 어느 gate가 거부했는지 시험으로 구분할 수 없었다. 이 상태로는 관계 대상을 바꿔도 그 관계를 증명하는 시험이 대상 gate를 특정하지 못한다.

## 변경

`ErrEvidenceSnapshotMismatch`와 `ErrCodeAnchorSnapshotMismatch`를 공개 sentinel로 추가했다. 두 gate는 각각 자기 sentinel로 unwrap되는 오류를 반환한다. 기존 오류 문자열은 바이트 그대로 유지했다. 저장소에서 이 문자열에 의존하는 비시험 소비자는 없었다(git grep).

## 확인 방법과 결과

1. sentinel을 선언만 하고 반환 경로에 연결하지 않은 상태에서 통합 시험이 실패했다(behavioral RED, [원자료](../../system/eval/b0-knowledge-system/refactoring-n12-gate-attribution-2026-10-09/red-before-wiring.txt)). 그 전에 미정의 식별자로 인한 컴파일 RED도 있었다.
2. 연결 후 `TestForeignCodeAnchorPromotionPreservesActiveDataset`는 사례마다 정확히 한 gate만 맞는지 확인한다. foreign-evidence-snapshot은 evidence gate, stale 실제 CKG와 두 activation 경로는 code anchor gate만 맞고, relabelled·missing 사례는 두 sentinel 모두 아니다. `TestProjectionRejectsCrossSnapshotOrMissingEvidence`는 오류 문자열이 이전과 같은지 확인한다.
3. semantic을 import하는 8개 패키지 race: 시험251/하위시험264 pass, fail/skip0. vet·gofmt 통과. [요약](../../system/eval/b0-knowledge-system/refactoring-n12-gate-attribution-2026-10-09/summary.json).

이 결과는 test pass다. semantic linked run·관계 verified·사람 criterion accepted가 아니다. 검토 대상 후보 `36353d32`에는 이 변경이 들어 있지 않다. 사람 판정이 F3-T08/T11의 대상 변경을 채택하면, 승인된 overlay와 함께 이 변경을 포함한 새 capture/index에 결합한다.

- [x] gate 구분 sentinel·메시지 보존·RED→GREEN·관련 race.
- [ ] 검토자 이름과 대상별 판정. N12-A–D 미완료.

**남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
