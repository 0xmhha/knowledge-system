# N12 관계 기술 사전검토와 기준 누락 시험 보강

2026-10-09 · **Claude Code 기술 사전검토·기준 누락 시험3개 보강 / 사람 판정·N12 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 목적

[최신 N12 검토 자료](./N12-ANCHOR-BOUND-TRACE-REVIEW.md)의 F3-T01–17, 정책·ADR 주장6, 기존 SUPPORTS/ABOUT12가 실제 코드·시험과 맞는지 사람 검토자가 판단할 근거를 준비한다. 이 문서는 **사람 판정이 아니다**. reviewer는 null, 개별 승인0이다. 검토 대상 후보 신원은 그대로 원문`36353d32`/snapshot`672eae93`/dataset`1e82983e`이다.

## 사전검토 결과 요약

[전체 항목별 권고](../../system/eval/b0-knowledge-system/refactoring-n12-criterion-gap-tests-2026-10-09/prereview.json)에 file:line 근거를 남겼다.

- **F3-T08/T11 대상 부정확.** `ValidateDatasetAlignment`는 manifest 좌표만 비교한다. 다른 snapshot의 code anchor는 `ValidateCodeAnchors`/`checkCodeAnchor`와 `Projection.Validate`가 거부한다. T11의 통합 시험은 `ValidateDatasetAlignment`를 **통과해야 하는 양성 대조**로 쓰므로(anchor_promotion_integration_test.go:99) TESTED_BY 대상을 gate 함수로 바꾸는 수정을 권고한다.
- **F3-T10은 ac-source-evidence의 Given을 만들지 않는다.** fixture에 code anchor가 없고 좌표만 바꾼다. ac-source-evidence의 실제 근거는 F3-T12 하나다.
- **F3-T03**은 결과 집합만 확인하고 거리 순서를 확인하지 않았다. **F3-T07**은 빈 store·무필터의 사전 취소만 확인했다. **F3-T15/T17**은 "검증된 개념+미검증 관계"와 CKG 인용 보존을 다루지 않았다.
- 주장 P01/P05는 원문의 한정어(공개 관리 경로, 신규 캡처)를 빠뜨려 실제보다 넓다. P05는 legacy worktree 경로(capture.go:473-489)가 남아 있어 그대로면 현재 코드에 대해 거짓이다. P04는 "승인"이 빠졌다. 6주장 모두 저장소 계약 문서/ADR에서 나왔으며 owner·시행일이 있는 조직 정책이 아니다.
- 기존12관계는 SUPPORTS6 모두 범위·방향이 맞고, ABOUT은 P01을 `dataset`으로 바꾸는 수정을 권고한다.

## 보강한 시험

기준 문장 중 어떤 시험도 확인하지 않던 부분을 시험으로 채웠다. 제품 코드는 바꾸지 않았고 새 단언은 모두 현재 제품에서 통과했다. 따라서 **제품 RED가 아니라 기준 적용 범위 보강**이다.

1. `TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded`에 정확한 거리 순서 단언을 추가했다(ac-filtered-recall의 Then).
2. `TestSearchDetailedFilteredCancellationAfterStartIsNotComplete`(신규): exact 크기와 2,052개 큰 후보 집합 모두에서, 검색 진입 뒤 각 ctx 확인 지점마다 차례로 취소를 주입한다. 취소가 관측되면 `SearchCancelled/context_cancelled`이어야 하고 완료를 주장하면 안 된다. 취소가 관측되지 않은 마지막 실행은 K개 완료여야 한다(ac-filtered-completeness).
3. `TestProvidedOntologyUnverifiedRelationPreservesCKGAndCKVBaseline`(신규): 검증된 개념이지만 관계가 검증되지 않으면 CKG 전용 인용을 포함한 기준 인용이 그대로다. 검증된 관계 양성 대조는 순서를 바꿀 수 있음을 확인한다(ac-ontology-fallback). 미검증 관계를 걸러 내는 부분 자체는 `TestActiveProjectionExposesOnlyVerifiedImplementations`가 담당하며, 두 시험은 인터페이스로 이어진다. 하나의 실제 저장소 종단 시험은 아니다.

변이 확인: 2번 시험은 취소의 상태 변환 경로를 제거하면 실패했다(M3). post-loop 확인 하나, exact per-chunk 확인 하나를 각각 제거한 변이는 driver와 이후 확인 지점이 같은 취소를 잡아 살아남았다(M1/M2). 이는 중복 보호이며, 두 줄 각각을 고정하는 시험은 아니다. [변이 기록](../../system/eval/b0-knowledge-system/refactoring-n12-criterion-gap-tests-2026-10-09/mutation-checks.json).

관련3패키지(sqlitevec, stage2, semantic) race: 시험132/하위시험102 pass, fail/skip0. vet·gofmt 통과. [요약](../../system/eval/b0-knowledge-system/refactoring-n12-criterion-gap-tests-2026-10-09/related-tests-summary.json). 이 통과는 test pass이며 semantic linked run·관계 verified·사람 criterion accepted가 아니다.

## 한계와 다음

- 새 시험은 최신 후보 `36353d32`에 포함되지 않았다. 후보의 관계 판정은 그 후보의 시험 바이트를 기준으로 한다. 사람 판정이 새 시험을 근거로 관계를 추가·변경하기로 하면, 그때 승인된 overlay와 함께 새 capture/index에 결합한다. 기존 후보와 원자료는 소급 변경하지 않는다.
- 사람 검토자 이름과 대상별 승인/수정/반려가 여전히 필요하다. core20 개념 정의 각각의 내용 검토는 이번 사전검토 범위 밖이다.

- [x] F3-T01–17·주장6·기존12관계 기술 사전검토와 근거 기록(사람 판정 아님).
- [x] 기준 누락3곳 시험 보강, 관련 race/vet/fmt와 변이 확인.
- [ ] 실제 검토자 이름과 대상별 판정. N12-A–D 전체 미완료.

**남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
