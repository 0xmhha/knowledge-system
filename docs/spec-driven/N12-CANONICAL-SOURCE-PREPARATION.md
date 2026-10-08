# N12 현행 소스·canonical 연결 준비 및 조회 누락 수정

2026-10-08 · **코드 연결 준비 검증 / 실제 내용·관계 승인과 통합 캡처 미완료**

검토 원문 기준은 `24266f3015489559249e2f810028d4bf3da70c52`다. 기존 27개 메타모델·명세·기준과 정책/ADR 6개를 현행 Git 객체·작업 파일에 대조했다. **33개 항목의 69개 참조(원문 33, 코드·테스트 36)**가 모두 기존 바이트·파일 SHA·span SHA·행 좌표와 일치했다. 이전 원문·판정 칸·승인 입력·FINAL 자료는 보존했다.

## 실제 코드 문제와 수정

`NodesByFilePath`가 반환한 모든 코드 후보의 canonical ID가 비어 있어 연결 준비가 실패했다. 실제 DB에는 ID가 저장돼 있었고 `FindByCanonicalID`는 이를 반환했다. 원인은 공통 `nodeColumns`와 row scanner가 `canonical_id`를 읽지 않는 것이었다. FTS와 substring의 별도 SELECT도 같은 누락을 갖고 있었다.

`sqlite_helpers.go`의 공통 조회와 `sqlite_fts.go`의 두 조회가 저장된 ID를 읽도록 수정했다. 기존 ID를 생성하거나 바꾸지 않는다. canonical이 없는 legacy 노드는 빈 문자열을 유지한다. 스키마·검색 순서·점수·필터 조건은 바꾸지 않았다. 퇴역 PostgreSQL 경로에는 이 변경을 적용하지 않았다.

올바르게 구성한 회귀 fixture에서 파일 캐시·ID·정확 심벌·순위·검색·CJK·FTS·내보내기의 **8개 경로 모두 저장된 ID를 잃는 RED**를 확인했다. 실제 cold→부분 증분 빌드에서 unchanged b.go의 `Mul` ID도 확인하도록 기존 캐시 시험을 보강했다. 초기 FTS 색인/정확 이름 fixture 오류는 따로 보존하고 최종 RED의 제품 실패에 합산하지 않았다.

## 실제 source-only 그래프 연결

서로 독립된 Git 저장소에 같은 커밋을 복원하고 선택한 코드·테스트 20개 파일의 SHA를 확인한 뒤 CKG를 cold build했다. 파싱 오류 0, unresolved graph refs 0이다. 그래프 source는 위 원문 커밋이며, 마지막 조회/연결 실행은 이번 SQLite 수정 소스에서 수행했다. 그래프 빌드 당시 소스와 수정된 reader를 혼동하지 않도록 diff와 각각의 SHA를 보관한다.

**36/36 참조 → 33개 고유 canonical ID**, unresolved 0이다. 같은 SourceIdentity·SearchDetailed·테스트를 여러 검토 항목에서 참조한 경우를 서로 독립된 의미 사실이나 시험으로 늘려 세지 않는다. 모든 항목은 여전히 proposed이며, code evidence만 붙이고 IMPLEMENTED_BY/TESTED_BY/CHECKED_BY 관계를 새로 만들지 않았다.

| 검토 항목 | 용도 | 실제 canonical ID |
|---|---|---|
| P12-C01 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/setup.SourceIdentity` |
| P12-C02 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/setup.DatasetIdentity` |
| P12-C03 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/setup.SourceIdentity` |
| P12-C04 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/setup.CapturedFile` |
| P12-C05 | candidate_usage | `github.com/0xmhha/knowledge-system/pkg/graph/types.Node` |
| P12-C06 | candidate_usage | `github.com/0xmhha/knowledge-system/pkg/vector/types.Chunk` |
| P12-C07 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.DocumentSection` |
| P12-C08 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.Claim` |
| P12-C09 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.EvidenceSpan` |
| P12-C10 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.Assertion` |
| P12-C11 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.Concept` |
| P12-C12 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.Term` |
| P12-C13 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.Requirement` |
| P12-C14 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.AcceptanceCriterion` |
| P12-C15 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec.TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded` |
| P12-C16 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/semantic.TestRun` |
| P12-C17 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/setup.Options` |
| P12-C18 | candidate_usage | `github.com/0xmhha/knowledge-system/pkg/system/contract.EvidencePackV2` |
| P12-C19 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/patch.Attempt` |
| P12-C20 | candidate_usage | `github.com/0xmhha/knowledge-system/internal/system/patch.Decision` |
| P12-A01 | candidate_implementation | `github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec.(*Store).SearchDetailed` |
| P12-A01 | test:TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded | `github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec.TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded` |
| P12-A02 | candidate_implementation | `github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec.(*Store).SearchDetailed` |
| P12-A02 | test:TestSearchDetailedReportsCandidateLimitWithoutCompleteHits | `github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec.TestSearchDetailedReportsCandidateLimitWithoutCompleteHits` |
| P12-A02 | test:TestSearchDetailedCancellationIsNotEmptySuccess | `github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec.TestSearchDetailedCancellationIsNotEmptySuccess` |
| P12-A03 | candidate_implementation | `github.com/0xmhha/knowledge-system/internal/system/semantic.ValidateDatasetAlignment` |
| P12-A03 | test:TestProjectionRejectsCrossSnapshotOrMissingEvidence | `github.com/0xmhha/knowledge-system/internal/system/semantic.TestProjectionRejectsCrossSnapshotOrMissingEvidence` |
| P12-A03 | test:TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty | `github.com/0xmhha/knowledge-system/internal/system/semantic.TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty` |
| P12-A04 | candidate_implementation | `github.com/0xmhha/knowledge-system/internal/system/composer/stage2.(*Searcher).applyProvidedOntology` |
| P12-A04 | test:TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline | `github.com/0xmhha/knowledge-system/internal/system/composer/stage2.TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline` |
| P12-P01 | candidate_code | `github.com/0xmhha/knowledge-system/internal/setup.checkMaintenanceTarget` |
| P12-P02 | candidate_code | `github.com/0xmhha/knowledge-system/internal/setup.syncCurrentParent` |
| P12-P03 | candidate_code | `github.com/0xmhha/knowledge-system/internal/system/evidencev2.BuildFromRefs` |
| P12-P04 | candidate_code | `github.com/0xmhha/knowledge-system/internal/system/knowledgepack.LoadWorksheetCatalogs` |
| P12-P05 | candidate_code | `github.com/0xmhha/knowledge-system/internal/setup.(CapturedSource).MaterializeBuildTreeContext` |
| P12-P06 | candidate_code | `github.com/0xmhha/knowledge-system/internal/setup.sourceSnapshotID` |

새 namespace에 과거 projection의 snapshot만 덮어쓰면 ontology/spec evidence ID의 출처 일관성을 깨뜨리므로 검증기가 거부했다. 이번 자료는 새 namespace에서 원문을 다시 추출해 20 concepts·3 requirements·6 claims·12 assertions를 **proposed 그대로** 유지한다. 과거 projection을 수정하지 않았다.

다른 snapshot, 다른 경로, AST 범위 밖, 없는 canonical ID, document를 code로 쓰는 시도 5개는 AttachCodeEvidence가 거부했고 proposal은 그대로였다. 잘못된 span SHA는 별도 ValidateSources가 거부했다. CKV 없는 graph-only proposal의 통합 alignment도 거부됐다. 합계 **7개 거부 제어 통과**이며 canonical 조회만으로 원문 SHA를 검증했다는 뜻은 아니다.

## 통합 캡처의 실제 미완료 사유

처음 저장소 전체를 CaptureSource(committed)로 봉인하려 했으나 `system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/darwin-arm64/signature/public.pem`이 unsafe committed source path로 거부됐다. 이 경로의 정책 거부를 기록하고 보호 규칙·원래 평가 파일을 변경하지 않았다. 독립 Git 복원본의 source-only 그래프 검증은 v2 retained capture·snapshot identity·knowledge lock·CKV alignment·승격을 대체하지 않는다.

실제 reviewed source 범위와 캡처 입력 구성을 정한 뒤 coordinated source/lock/canonical/semantic을 결합해야 한다. 현재 full_v2_capture_complete=false, coordinated_dataset_built=false, human_approved=0, activated=false다. N12-A–D는 미체크를 유지한다.

## 검증 기록과 다음 작업

관련 5개 패키지의 기존/신규 top-level 시험 240개와 보강된 실제 증분 빌드 시험이 통과했다. 전체 Go 회귀도 **117개 시험 패키지·2327개 top-level 시험 통과**(시험 없는 패키지 12개, 시험 skip 16개, process exit0)했다. 최종 결과는 [verification.json](../../system/eval/b0-knowledge-system/refactoring-n12-canonical-preparation-2026-10-08/verification.json)에 기록한다. 운영체제는 Darwin ARM64이며 이번 수정의 native Linux 실행은 아직 없다. 새 모델·FINAL 질의와 실제 사람 승인·의미 수용은 0이다.

[코드·테스트 연결](../../system/eval/b0-knowledge-system/refactoring-n12-canonical-preparation-2026-10-08/canonical-anchors.json), [원문 대조](../../system/eval/b0-knowledge-system/refactoring-n12-canonical-preparation-2026-10-08/source-audit.json), [거부 제어](../../system/eval/b0-knowledge-system/refactoring-n12-canonical-preparation-2026-10-08/negative-controls.json), [변경 diff](../../system/eval/b0-knowledge-system/refactoring-n12-canonical-preparation-2026-10-08/source-change.diff).

다음은 N11 후보/gold·독립성 및 N12 내용/관계의 실제 판정을 반영하고, approved 범위의 캡처 입력·통합 snapshot/lock 연결을 준비하는 것이다. 이후 N14의 실제 변경·linked run·사람의 criterion별 수용으로 이어진다.

현재 전체 8/18·요청 5/15 완료, 남은 10개: N08 검색 품질, N09 무답·기권, N10 비용·지연, N11 독립 평가, N12 실제 파일럿 검토, N14 실제 의미 수용, N15 지원 환경·외부 소비자, N16 운영 권한·고지, N17 운영 복구, N18 최종 수용·출시 판정.

2026-10-08 N12 캡처 입력 준비 검증: [명시 소스 범위의 보관 캡처](./N12-SCOPED-CAPTURE-PREPARATION.md) — 원본667a9af5에서 소스·의존·검토 문서1,410개를 독립 준비 Git 커밋562e37aa로 구성해 committed 캡처/복원·archive-only 재생/신원·69검토span·승인Go29·8거부 제어를 확인했다. snapshot-only는 build31/vendor fixture1 등32개를 제외하므로 전체 입력 보관으로 사용하지 않았다. 제품 정책/소스 변경0, 새 모델/FINAL질의·사람 판정0. 제안 입력 범위·내용/관계 승인 및 새 coordinated CKG/CKV/lock 결합은 미완료, 전체8/18·요청5/15·잔여10개 유지.

2026-10-08 N12 기술 연결 검증: [동일 신원의 held 색인·의미 준비](./N12-COORDINATED-PREPARATION.md) — 보관소스1,410파일/선택58의 CKG53Go·CKV56입력·666chunks/566심벌 canonical100%, 문서27/27·코드36/고유33·proposed 저장 roundtrip·12거부 제어·archive-only 재생 통과. 원문project knowledge-system로 수정한 별도 후보이며 첫 mismatch 거부와 전체Go범위 DRIFT998 경고를 보존했다. 실제 BGE embedding HTTP124회(색인입력1332·시작probe4), 새DEV/FINAL0·human0·current/semantic activation0. approved overlay/lock·내용/관계/입력범위 판정은 남아 N12-A–D/전체8/18·요청5/15·잔여10개를 유지한다.
