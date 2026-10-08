# N12 새 통합 시험을 포함한 동일 소스 관계 검토

2026-10-09 · **source/index/관계 결합 및 보관본 재생 검증 완료 / 개별 내용 승인·N12 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 목적과 결합 신원

[승격 회귀](./N12-CODE-ANCHOR-PROMOTION-REGRESSION.md)의 실제 CKG foreign anchor 시험을 새로운 captured source와 실제 index에 포함했다. 이전 55d8/43ba/f839 후보는 불변이며 당시 포함되지 않은 시험을 소급해 승인하지 않는다.

- runtime 원문 `d6fc8902c2df4c66a6ef84307477f1ab44740b8c`; derived committed source `36353d32f1c21fc0cb2622807fa9e5d05f2daac7`.
- snapshot `672eae93464af931deb5b66605ba6d21615e50ced58c5a09cd82befc12f56ae7`; dataset `1e82983e45ea35eb63debbea9bf88b6961d595cb5c540db312430cc47d19a341`.
- source1,438파일, 명시선택71, 실제 CKG63Go/CKV66(Go63+MD3), 747chunks/625canonical, canonical floor1.0. YAML4와 선택 JSON1은 보관·추출되며 vectorized 입력으로 세지 않는다.
- 실제 AST canonical18(구현3/시험7/문맥8), 원문 evidence81, CKV 문서절27/27. 신규 관계17와 기존 SUPPORTS/ABOUT12 모두 proposed. 개념23(core20+기능3), 요구3 version2/기준4를 원문 그대로 추출했다.

## 완료한 기술 하위 단계

- [x] 실제 selected path/SHA bijection, source identity, retained source, CKG canonical/span SHA, CKV section 검증.
- [x] 관련3패키지 race8시험/29하위시험 pass, fail0/skip0. 새 통합 시험은 actual CKG2build/2snapshot의8쓰기·2activation 거부와 기존 active 보존을 검사한다.
- [x] 제안 projection SQLite 저장→닫기→읽기 전용 재열기 일치, 변조5개 거부.
- [x] 13,792,619byte/1,457file 보관본에 source/history·실제 CKG/CKV DB·proposed 의미 DB 포함. 원본 source/dataset/store 없이 재생; 모든 권위 SHA와 관계·근거·진단 bytes 동일. 운영 이식성 수용 시험과는 별도다.
- [x] 이전 후보 원자료와 승인5입력 SHA 불변 확인. 모든 검토 decision/reviewer/실제 활동시간 null.
- [ ] 개별 개념·요구·관계·criterion의 실제 사람 판정 및 reviewed path.
- [ ] 승인된 overlay/lock·actual semantic linked run·파일럿 사용/criterion 수용. N12-A–D 전체 미완료.

실제 BGE-M3:latest digest790764…16bab/native1024·Ollama0.35.1은 전후 동일하다. index embedding70HTTP/747입력, 시작 probe2HTTP/2입력; 전체 embedding72HTTP/749입력·출력, 실패0. query DEV/FINAL0, semantic linked run0, actual pilot/current activation0, 사람 승인0이다. 색인 비용을 query warm/cold p95나 사람 검토 시간으로 쓰지 않는다.

전체 Go 감사는 build1,055/DB63/in_both63/missing992/extra0 **DRIFT**다. 누락992는 명시 선택63Go 밖이다. 선택 범위의 성공을 전체 repository 색인 성공으로 바꾸지 않는다. Linux/full repository 시험은 이번에 실행하지 않았다.

## 검토할 내용

이미 승인된 것은 기능3개·요구version2를 선택한다는 **범위**다. 아래 내용과 연결의 의미는 아직 승인되지 않았다.

- `filtered-nearest-chunk-search`: Metadata-constrained nearest-chunk retrieval that reports bounded-search incompleteness. 포함: Eligible nearest chunks, exact-sized sparse candidate ordering, cancellation and search budget status. 제외: Versioned policy authority, source capture, semantic truth.
- `snapshot-bound-semantic-relations`: Validation of semantic relation evidence against one captured source identity and coordinated graph/vector indexes. 포함: Typed proof references, source snapshot alignment, rejection before storing or switching a candidate. 제외: Full capture lifecycle implementation, human approval of relation meaning, automatic criterion acceptance.
- `optional-ontology-baseline-preservation`: Optional interpretation of concepts that preserves the baseline retrieval citation set for ambiguous or unreviewed meanings. 포함: Natural-language matching, no unreviewed relation boost, optional fallback. 제외: Whole term storage implementation, whole evidence response construction, live retrieval quality proof.

- `req-filtered-recall` v2: CKV returns eligible nearest chunks under sparse metadata filters.
  - `ac-filtered-recall` — Given: A sparse filter leaves an exact-search-sized candidate set with at least K eligible chunks and many closer excluded chunks; When: A filtered query requests K results; Then: The response contains K eligible chunks in exact distance order.
  - `ac-filtered-completeness` — Given: A larger filtered candidate set contains at least K eligible chunks; When: A filtered query reaches a search budget or is cancelled; Then: The response marks incomplete with a reason and never claims K-result completeness.
- `req-source-evidence` v2: Reviewed semantic relationships remain tied to one captured source snapshot.
  - `ac-source-evidence` — Given: A relation with a code anchor from another snapshot; When: The semantic dataset is promoted; Then: Promotion fails and the active dataset remains unchanged.
- `req-ontology-fallback` v2: Optional concept matching preserves the existing hybrid retrieval candidates.
  - `ac-ontology-fallback` — Given: An ambiguous term or an unverified concept relationship; When: A user asks a natural-language question; Then: The baseline CKV and CKG citation set is preserved.

관계의 방향·대상·시험 범위가 위 문장에 적합한지 승인/수정/반려를 기록한다. 구현3 관계는 해당 구성요소를 나타내며 전체 기능의 단일 구현을 증명하지 않는다.

| 검토 ID | 관계 | 출발 대상 | 실제 CKG 대상 원문 |
|---|---|---|---|
| F3-T01 | IMPLEMENTED_BY | `filtered-nearest-chunk-search` | [SearchDetailed](../../internal/vector/store/sqlitevec/store.go#L620) |
| F3-T02 | TESTED_BY | `SearchDetailed` | [TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded](../../internal/vector/store/sqlitevec/store_test.go#L389) |
| F3-T03 | CHECKED_BY | `ac-filtered-recall` | [TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded](../../internal/vector/store/sqlitevec/store_test.go#L389) |
| F3-T04 | TESTED_BY | `SearchDetailed` | [TestSearchDetailedReportsCandidateLimitWithoutCompleteHits](../../internal/vector/store/sqlitevec/store_test.go#L142) |
| F3-T05 | CHECKED_BY | `ac-filtered-completeness` | [TestSearchDetailedReportsCandidateLimitWithoutCompleteHits](../../internal/vector/store/sqlitevec/store_test.go#L142) |
| F3-T06 | TESTED_BY | `SearchDetailed` | [TestSearchDetailedCancellationIsNotEmptySuccess](../../internal/vector/store/sqlitevec/store_test.go#L177) |
| F3-T07 | CHECKED_BY | `ac-filtered-completeness` | [TestSearchDetailedCancellationIsNotEmptySuccess](../../internal/vector/store/sqlitevec/store_test.go#L177) |
| F3-T08 | IMPLEMENTED_BY | `snapshot-bound-semantic-relations` | [ValidateDatasetAlignment](../../internal/system/semantic/alignment.go#L69) |
| F3-T09 | TESTED_BY | `ValidateDatasetAlignment` | [TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty](../../internal/system/semantic/alignment_test.go#L26) |
| F3-T10 | CHECKED_BY | `ac-source-evidence` | [TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty](../../internal/system/semantic/alignment_test.go#L26) |
| F3-T11 | TESTED_BY | `ValidateDatasetAlignment` | [TestForeignCodeAnchorPromotionPreservesActiveDataset](../../internal/system/semantic/anchor_promotion_integration_test.go#L25) |
| F3-T12 | CHECKED_BY | `ac-source-evidence` | [TestForeignCodeAnchorPromotionPreservesActiveDataset](../../internal/system/semantic/anchor_promotion_integration_test.go#L25) |
| F3-T13 | IMPLEMENTED_BY | `optional-ontology-baseline-preservation` | [applyProvidedOntology](../../internal/system/composer/stage2/ontology_provider.go#L76) |
| F3-T14 | TESTED_BY | `applyProvidedOntology` | [TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline](../../internal/system/composer/stage2/ontology_provider_test.go#L49) |
| F3-T15 | CHECKED_BY | `ac-ontology-fallback` | [TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline](../../internal/system/composer/stage2/ontology_provider_test.go#L49) |
| F3-T16 | TESTED_BY | `applyProvidedOntology` | [TestStoredSharedAliasPreservesBaselineAndRetainedEvidence](../../internal/system/composer/stage2/ontology_matcher_integration_test.go#L28) |
| F3-T17 | CHECKED_BY | `ac-ontology-fallback` | [TestStoredSharedAliasPreservesBaselineAndRetainedEvidence](../../internal/system/composer/stage2/ontology_matcher_integration_test.go#L28) |

특히 **F3-T08**의 ValidateDatasetAlignment는 좌표 일치를 담당한다. source byte·actual canonical·활성화 재검사는 checkCodeAnchor 및 PutAligned/PutAlignedRetained/ActivateAligned/ActivateAlignedRetained 경로가 담당한다. **F3-T09/10**은 좌표 변조 부분 제어다. **F3-T11/12**의 새 통합 시험은 이 넓은 gate chain을 실제 CKG DB와 보관 source로 확인한다. 단, CKV는 coordinate-only mock manifest로 vector DB/chunk links/embedding이 없으며 fixture vocabulary/관계도 proposed다. 따라서 actual pilot의 reviewed 관계·criterion 수용으로 자동 승격하지 않는다.

F3-T01–07은 controlled vector fixture이며 실제 코퍼스 검색 품질·지연 판정이 아니다. F3-T13–17은 retained ontology/shared alias→retrieval의 baseline 보존 제어이며 전체 용어 저장/인용 생성의 구현을 뜻하지 않는다. 문맥8 canonical은 관계17개 또는 독립 FINAL8개로 세지 않는다.

[전체 개별 검토 worksheet](../../system/eval/b0-knowledge-system/refactoring-n12-anchor-binding-2026-10-09/human-review-worksheet.json)는 개념23·요구3/기준4·주장6·기존관계12·신규관계17과 source SHA를 담는다. 검토자 이름과 실제 대상별 승인/수정/반려가 필요하다. 먼저 기능3·요구3/기준4·F3-T01–17의 범위 해석을 검토하고, core20/주장6/기존12관계의 별도 내용 검토가 남았음을 유지한다. 전부 승인이라는 범위 선택이나 fixture author를 사람 판정으로 대체하지 않는다.

## 다음과 남은 전체

다음은 위 내용의 실제 검토자·개별 판정을 기록한 뒤 승인된 의미 경로 준비다. N11 독립 gold/군·오염/기권/충분한 FINAL 및 runtime 동결은 별도 선행조건이며 이번 관계17개를 FINAL17문항으로 세지 않는다.

| 우선순위 | 남은 작업 |
|---|---|
| 1 | N12 실제 내용·관계·criterion/파일럿 수용, N11 독립 DEV/FINAL gold·군·오염/기권 판정·부족 표본·runtime 동결 |
| 2 | N08 검색 품질, N09 기권, N10 비용·warm/cold 지연 |
| 3 | N14 실제 변경/linked run·수용, N15 규모·설치·이식성·독립 소비자 |
| 3 | N16 owner·역할·키·고지, N17 실제 운영 복구 |
| 4 | N18 기본값·최종 수용·출시 |

증거는 [refactoring-n12-anchor-binding-2026-10-09](../../system/eval/b0-knowledge-system/refactoring-n12-anchor-binding-2026-10-09/manifest.json)에 결합한다. 완료 N01–07/N13의8개를 유지한다. Codex 직접 수행, Claude/새 외부 전송0.
