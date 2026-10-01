# B0 상태 변화형 평가 fixture 설계

상태: **설계 초안 / 실모델 미실행** (2026-10-01). [`END-TO-END-DESIGN.md`](./END-TO-END-DESIGN.md) 9.2절의 질문군 중 정적 파일·줄 정답만으로 검사할 수 없는 경우를 별도 실행 단위로 정의한다. 이 문서는 `questions.json`의 사람 승인이나 `cks eval` 결과를 대신하지 않는다.

## 공통 실행 계약

각 fixture는 고정된 코퍼스 커밋, 임베딩 모델 다이제스트, 빌드 커밋, `project_id/snapshot_id/dataset_id`, 질의·필터·K, 실행 순서, 기대 상태, 실제 EvidencePack 원자료를 기록한다. 변경 전후를 비교하는 경우 각 상태를 **서로 다른 불변 데이터셋**으로 만들고 이전 데이터셋은 보존한다. 검색 실패, 근거 없음, 기권, 스냅샷 불일치, 불완전 검색을 서로 다른 결과로 기록한다. 사람 검토자는 예상 파일·줄·관계·기권 여부를 결과를 보기 전에 승인한다. 최종 판정 사례는 튜닝에 사용하지 않는다.

구조 시험은 모델 없이 조건을 재현하는 A 단계 근거다. 아래의 B 단계 절차는 실제 Ollama 색인과 질의를 추가로 요구한다. 구조 시험 통과를 자연어 검색 품질 통과로 해석하지 않는다.

| ID / 질문군 | 데이터와 상태 변화 | 사전 예상 결과 | 기존 구조 시험 | B 단계 추가 측정 |
|---|---|---|---|---|
| F-01 희소 복합 필터 | 같은 임베딩에 가까운 다수의 비대상 청크와 소수의 대상 청크를 두고 언어·파일 종류·경로 조건을 결합한다. 필터의 대상이 전체 상위 K 밖에 위치하게 한다. | 필터를 만족하는 대상이 충분하면 K개 회수. 후보 상한에 걸리면 `incomplete`와 사유를 보고하고 완전한 결과라고 주장하지 않는다. | `internal/vector/store/sqlitevec/store_test.go`의 `TestStoreCombinedFilterMatchesExactOracle`, `TestStoreFilterLargeCandidateSetFallsBackToExact`, `TestSearchDetailedReportsCandidateLimitWithoutCompleteHits` | 실제 임베딩으로 같은 조건의 recall@K, 완전성 상태, 지연과 원시 후보 수를 기록한다. |
| F-02 긴 문서 끝 | 긴 Markdown 섹션을 입력하고 답이 마지막 하위 청크에만 있게 한다. 앞부분이 의미상 비슷해도 답은 없다. | 마지막 원문 줄을 포함한 청크와 올바른 줄 인용을 회수하며 부모 섹션 ID를 보존한다. | `internal/vector/chunk/chunk_test.go`의 `TestLongMarkdownSectionKeepsTailAndLineCitations` | 긴 설계 문서의 끝부분에서 사람이 승인한 문장을 골라 top-K 회수와 정확한 줄 범위를 본다. |
| F-03 다의어·미등록 표현 | 같은 자연어 표현이 둘 이상의 개념에 연결될 수 있는 팩과 어떤 개념에도 등록되지 않은 표현을 각각 질의한다. | 다의어는 여러 후보와 불확실성을 보존한다. 미등록 표현은 개념 확정 없이 기본 검색 후보를 보존한다. | `internal/system/semantic/matcher_test.go`의 `TestMatchConceptsPreservesAmbiguityAndNaturalLanguage`; `internal/system/composer/stage2/ontology_test.go`의 `TestOntologyBoostOnlyExistingSameSnapshotCanonicalCitation` | 한국어·영어 표현별 기본/개념/관계/결합 ablation에서 오개념 확정률, 후보 손실, 인용 변화를 비교한다. |
| F-04 오래된 인용 | 동일 경로의 본문을 수정하여 이전 인용의 커밋 또는 원문 줄 해시가 현재 데이터셋과 달라지게 한다. | 이전 근거를 현재 근거인 양 인용하지 않는다. stale 또는 snapshot mismatch를 명시하고 현재 데이터셋에 유효한 근거만 인용한다. | `internal/vector/query/citation_test.go`의 `TestEnforceCitationsAt_StaleCommitHashFlag`; `internal/system/semantic/ckg_anchor_test.go`의 `TestCodeAnchorRequiresSameSnapshotAndContainedASTSpan` | 교체 전후 두 데이터셋을 개별 조회하고 출처 좌표 및 인용 해시를 원자료로 비교한다. |
| F-05 두 프로젝트 격리 | 같은 상대 경로와 심볼 이름을 가진 프로젝트 A/B를 별도 설치한다. A의 정책 팩이나 질의 문구가 B와 겹치도록 한다. | A 질의에 B의 코드·정책·스냅샷 근거가 0건이며 그 반대도 같다. 결합 시도는 거부된다. | `internal/setup/identity_test.go`의 `TestVerifyAlignmentRejectsPartialAndCrossProjectIdentity`; `internal/system/semantic/alignment_test.go`의 `TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty` | 동일 문구를 A/B 좌표로 각각 질의하고 팩 유무별 원시 근거 좌표를 대조한다. |
| F-06 스펙·구현 충돌 | 검토된 명세가 요구하는 동작과 현재 코드가 다른 사례를 고정한다. 무관한 테스트 통과 기록도 포함한다. | 명세와 코드를 동시에 보여주고 충돌 상태를 드러낸다. 무관한 테스트 통과를 수용 승인으로 바꾸지 않는다. | `internal/system/semantic/test_run_test.go`의 `TestExecuteLinkedTestRequiresExactReviewedTargetWhenAmbiguous`; `internal/system/semantic/active_test.go`의 `TestActiveProjectionExposesOnlyVerifiedImplementations` | 주장별 사람 검토로 오류 인식, 필요한 관계 회수, 잘못된 승인 주장 건수를 측정한다. |

## B0에서 아직 필요한 결정과 구현

1. F-01/F-02의 실제 코퍼스 파일·줄, F-03의 승인된 표현/팩, F-04/F-05의 두 상태 또는 두 프로젝트 fixture, F-06의 충돌 사례를 고정한다. 파일럿 코퍼스에 없는 상황은 별도 합성 프로젝트로 만들고 파일럿 품질 수치와 섞지 않는다.
2. 각 fixture의 예상 결과와 검토자, 실행군(`development` 또는 `final`)을 기록한다. 현재 어느 것도 승인된 정답이 아니다.
3. 동일 모델의 실제 데이터셋을 설치하고 구조 시험 뒤 런타임 절차를 실행한다. 원시 응답, 에러, 지연, 데이터셋 좌표를 저장한다. `cks eval` v1 인용 수치와 v2 정책/이유/답변 판정은 따로 보고한다.

질문셋 승인 후에도 이 절차가 비어 있으면 B0 기준선은 **부분 결과**다. B1 ablation과 C1 출시 판정은 이 항목들의 실제 실행과 검토를 요구한다.
