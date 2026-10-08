# N12 실제 파일럿 원문·현행 코드 검토 자료

2026-10-06 · 원문 커밋 `076f6de27b38423a3a2538cb95e41994d092e91b` · **검토 대기 / N12 미완료**.

기존 D1의 뜻과 범위 승인은 유지한다. 이번 자료는 같은20개 공통 타입의 실제 원문 추출과3요구사항/4기준의 현행 코드·테스트 후보를 검토하기 위한 것이다. 산업 사실20개를 승인해 달라는 요청이 아니다. 원문27span의 SHA/좌표 일치를 독립 확인했다. 모든 추출 객체는 proposed이고 자동 verified/accepted 승격은0이다.

[검토 JSON](../../system/eval/b0-knowledge-system/refactoring-n12-review-2026-10-06/review-items.json)에 원문SHA·spanSHA·적용범위·권위·용어/포함·제외·코드 후보를 기록했다. CKG canonical/통합 snapshot/lock 및 실제 변경 수용은 아직 확인되지 않아 source 좌표만으로 관계를 verified로 쓰지 않는다.

## 20개 실제 원문 객체와 용도 후보

| 검토ID | 타입 | 원문 정의 | 원문 좌표 | 현행 사용 후보 |
|---|---|---|---|---|
| P12-C01 | project | A named isolation boundary for source, indexes, reviews, and queries. | [docs/spec-driven/ontology-pilot.yaml:15-21](../../docs/spec-driven/ontology-pilot.yaml) | [type SourceIdentity struct {](../../internal/setup/identity.go) |
| P12-C02 | dataset | One immutable coordinated build of graph, vector, and optional semantic projections. | [docs/spec-driven/ontology-pilot.yaml:22-28](../../docs/spec-driven/ontology-pilot.yaml) | [type DatasetIdentity struct {](../../internal/setup/identity.go) |
| P12-C03 | source-snapshot | A captured and identified set of source bytes and policies to which all evidence in a dataset refers. | [docs/spec-driven/ontology-pilot.yaml:29-35](../../docs/spec-driven/ontology-pilot.yaml) | [type SourceIdentity struct {](../../internal/setup/identity.go) |
| P12-C04 | source-file | A source document or code file addressed relative to one source snapshot. | [docs/spec-driven/ontology-pilot.yaml:36-42](../../docs/spec-driven/ontology-pilot.yaml) | [type CapturedFile struct {](../../internal/setup/capture.go) |
| P12-C05 | code-symbol | A language parser's named code construct with a source range and stable join identity. | [docs/spec-driven/ontology-pilot.yaml:43-49](../../docs/spec-driven/ontology-pilot.yaml) | [type Node struct {](../../pkg/graph/types/node.go) |
| P12-C06 | vector-chunk | A bounded source span represented by an embedding and searchable metadata. | [docs/spec-driven/ontology-pilot.yaml:50-56](../../docs/spec-driven/ontology-pilot.yaml) | [type Chunk struct {](../../pkg/vector/types/chunk.go) |
| P12-C07 | document-section | A heading-scoped span of a source document with exact source lines. | [docs/spec-driven/ontology-pilot.yaml:57-63](../../docs/spec-driven/ontology-pilot.yaml) | [type DocumentSection struct {](../../internal/system/semantic/model.go) |
| P12-C08 | claim | A reviewable statement about the project that cites a source section. | [docs/spec-driven/ontology-pilot.yaml:64-70](../../docs/spec-driven/ontology-pilot.yaml) | [type Claim struct {](../../internal/system/semantic/model.go) |
| P12-C09 | evidence-span | An origin, relative path, line range, file digest, and range digest tied to one source snapshot. | [docs/spec-driven/ontology-pilot.yaml:71-77](../../docs/spec-driven/ontology-pilot.yaml) | [type EvidenceSpan struct {](../../internal/system/semantic/model.go) |
| P12-C10 | semantic-assertion | A typed and directed relationship between two identified objects with separate evidence and review state. | [docs/spec-driven/ontology-pilot.yaml:78-84](../../docs/spec-driven/ontology-pilot.yaml) | [type Assertion struct {](../../internal/system/semantic/model.go) |
| P12-C11 | concept | A stable domain meaning with a definition, scope, aliases, and review status. | [docs/spec-driven/ontology-pilot.yaml:85-91](../../docs/spec-driven/ontology-pilot.yaml) | [type Concept struct {](../../internal/system/semantic/model.go) |
| P12-C12 | term | A language-specific preferred or alternative expression for a concept. | [docs/spec-driven/ontology-pilot.yaml:92-98](../../docs/spec-driven/ontology-pilot.yaml) | [type Term struct {](../../internal/system/semantic/model.go) |
| P12-C13 | requirement | A versioned statement of intended system behavior independent of implementation. | [docs/spec-driven/ontology-pilot.yaml:99-105](../../docs/spec-driven/ontology-pilot.yaml) | [type Requirement struct {](../../internal/system/semantic/model.go) |
| P12-C14 | acceptance-criterion | A checkable condition for accepting one requirement at a named snapshot. | [docs/spec-driven/ontology-pilot.yaml:106-112](../../docs/spec-driven/ontology-pilot.yaml) | [type AcceptanceCriterion struct {](../../internal/system/semantic/model.go) |
| P12-C15 | test-case | A repeatable input and expected observation used to verify behavior. | [docs/spec-driven/ontology-pilot.yaml:113-119](../../docs/spec-driven/ontology-pilot.yaml) | [func TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded(t *testing.T) {](../../internal/vector/store/sqlitevec/store_test.go) |
| P12-C16 | test-run | An execution of tests with recorded command, environment, source snapshot, and outcome. | [docs/spec-driven/ontology-pilot.yaml:120-126](../../docs/spec-driven/ontology-pilot.yaml) | [type TestRun struct {](../../internal/system/semantic/test_run.go) |
| P12-C17 | policy | A versioned constraint controlling indexing, access, review, or evidence release. | [docs/spec-driven/ontology-pilot.yaml:127-133](../../docs/spec-driven/ontology-pilot.yaml) | [type Options struct {](../../internal/setup/plan.go) |
| P12-C18 | evidence-pack | A bounded and sanitized CKS response containing citations, bodies, graph neighbors, and integrity metadata. | [docs/spec-driven/ontology-pilot.yaml:134-140](../../docs/spec-driven/ontology-pilot.yaml) | [type EvidencePackV2 struct {](../../pkg/system/contract/v2.go) |
| P12-C19 | patch-attempt | An immutable attempt to apply a change from one base snapshot, producing a result snapshot and check evidence. | [docs/spec-driven/ontology-pilot.yaml:141-147](../../docs/spec-driven/ontology-pilot.yaml) | [type Attempt struct {](../../internal/system/patch/patch.go) |
| P12-C20 | criterion-decision | A human decision with reasons and evidence about one acceptance criterion at a named specification version and result snapshot. | [docs/spec-driven/ontology-pilot.yaml:148-154](../../docs/spec-driven/ontology-pilot.yaml) | [type Decision struct {](../../internal/system/patch/patch.go) |

## 3개 명세와4개 수용 기준

명세 내용과 후보 구현/테스트 연결의 타당성을 각각 판정한다. 테스트 JSON의 pass는 현재 원문에서 해당 테스트가 실행됐다는 증거이고, 사람의 Given/When/Then 판정 또는 실제 변경의 수용이 아니다.

### P12-R01 · req-filtered-recall

CKV returns eligible nearest chunks under sparse metadata filters.

**P12-A01 · ac-filtered-recall**

- Given: A sparse filter leaves an exact-search-sized candidate set with at least K eligible chunks and many closer excluded chunks
- When: A filtered query requests K results
- Then: The response contains K eligible chunks in exact distance order
- 구현 후보: [func (s *Store) SearchDetailed(ctx context.Context, query []float32, k int, filter types.Filter, opts SearchOptions) (SearchResult, error) {](../../internal/vector/store/sqlitevec/store.go)
- 테스트 실행 pass: `TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded` · 후보 연결은 미승인

**P12-A02 · ac-filtered-completeness**

- Given: A larger filtered candidate set contains at least K eligible chunks
- When: A filtered query reaches a search budget or is cancelled
- Then: The response marks incomplete with a reason and never claims K-result completeness
- 구현 후보: [func (s *Store) SearchDetailed(ctx context.Context, query []float32, k int, filter types.Filter, opts SearchOptions) (SearchResult, error) {](../../internal/vector/store/sqlitevec/store.go)
- 테스트 실행 pass: `TestSearchDetailedReportsCandidateLimitWithoutCompleteHits` · 후보 연결은 미승인
- 테스트 실행 pass: `TestSearchDetailedCancellationIsNotEmptySuccess` · 후보 연결은 미승인

### P12-R02 · req-source-evidence

Reviewed semantic relationships remain tied to one captured source snapshot.

**P12-A03 · ac-source-evidence**

- Given: A relation with a code anchor from another snapshot
- When: The semantic dataset is promoted
- Then: Promotion fails and the active dataset remains unchanged
- 구현 후보: [func ValidateDatasetAlignment(p Projection, repoRoot, graphDir, vectorDir string) error {](../../internal/system/semantic/alignment.go)
- 테스트 실행 pass: `TestProjectionRejectsCrossSnapshotOrMissingEvidence` · 후보 연결은 미승인
- 테스트 실행 pass: `TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty` · 후보 연결은 미승인

### P12-R03 · req-ontology-fallback

Optional concept matching preserves the existing hybrid retrieval candidates.

**P12-A04 · ac-ontology-fallback**

- Given: An ambiguous term or an unverified concept relationship
- When: A user asks a natural-language question
- Then: The baseline CKV and CKG citation set is preserved
- 구현 후보: [func (s *Searcher) applyProvidedOntology(ctx context.Context, agg *aggregator, prompt string, hits []contract.Hit, baseline []ScoredCitation, demoteTests, demoteDocs bool) ([]ScoredCitation, []semantic.ConceptCandidate, *contract.OntologyDiagnostic) {](../../internal/system/composer/stage2/ontology_provider.go)
- 테스트 실행 pass: `TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline` · 후보 연결은 미승인

## 기록할 판정

P12-C01–20, R01–03, A01–04 각 항목에 승인/수정/기권과 이유, 검토자 이름을 기록한다. 사용 후보의 원문/범위가 틀리면 해당ID를 수정한다. 승인 시에도 새로운 통합 원문/lock에 대한 검증과 N14 실제 변경 수용은 별도로 남는다.

실제 검토에 쓴 활동 시간(분)을 알 수 있으면 알려준다. 미측정이면 null/미측정으로 유지한다. 메시지 대기 시간은 검토 시간으로 계산하지 않는다. 테스트·추출의 자동 실행 시간도 사람 비용으로 바꾸지 않는다.

현재 완료8/18, 요청15개 중5완료. 잔여10개: N08 검색 품질, N09 무답·기권, N10 비용·지연, N11 독립 평가, N12 실제 내용 검토, N14 실제 의미 수용, N15 지원 환경, N16 운영 권한·고지, N17 운영 복구, N18 출시 판정.

2026-10-08 현행 대조·연결 보완: [현행 canonical 연결 준비](./N12-CANONICAL-SOURCE-PREPARATION.md)에서 기존 33항목의 원문/코드 69참조가 동일함을 확인했다. 실제 CKG 코드·테스트 20파일에서 36참조/33고유 canonical ID를 결합했으나 code evidence만 붙였으며 내용·관계 승인, 통합 capture/lock/CKV 또는 활성 승격은 없다. 공통 SQLite 조회의 canonical_id 누락을 재현·수정했고 7개 근거 거부 제어를 통과했다. 저장소 전체 committed capture는 평가 public.pem 경로 때문에 실제 거부됐으므로 별도 source-only Git 검증을 통합 캡처 완료로 쓰지 않는다. N12 전체와 사람 판정은 대기를 유지한다.

2026-10-08 N12 캡처 입력 준비 검증: [명시 소스 범위의 보관 캡처](./N12-SCOPED-CAPTURE-PREPARATION.md) — 원본667a9af5에서 소스·의존·검토 문서1,410개를 독립 준비 Git 커밋562e37aa로 구성해 committed 캡처/복원·archive-only 재생/신원·69검토span·승인Go29·8거부 제어를 확인했다. snapshot-only는 build31/vendor fixture1 등32개를 제외하므로 전체 입력 보관으로 사용하지 않았다. 제품 정책/소스 변경0, 새 모델/FINAL질의·사람 판정0. 제안 입력 범위·내용/관계 승인 및 새 coordinated CKG/CKV/lock 결합은 미완료, 전체8/18·요청5/15·잔여10개 유지.

2026-10-08 N12 기술 연결 검증: [동일 신원의 held 색인·의미 준비](./N12-COORDINATED-PREPARATION.md) — 보관소스1,410파일/선택58의 CKG53Go·CKV56입력·666chunks/566심벌 canonical100%, 문서27/27·코드36/고유33·proposed 저장 roundtrip·12거부 제어·archive-only 재생 통과. 원문project knowledge-system로 수정한 별도 후보이며 첫 mismatch 거부와 전체Go범위 DRIFT998 경고를 보존했다. 실제 BGE embedding HTTP124회(색인입력1332·시작probe4), 새DEV/FINAL0·human0·current/semantic activation0. approved overlay/lock·내용/관계/입력범위 판정은 남아 N12-A–D/전체8/18·요청5/15·잔여10개를 유지한다.

2026-10-08 N12 trace 연결 준비: [13개 proposed 관계와 의미 경계 검증](./N12-PROPOSED-TRACE-RELATIONS.md). 구현3/시험5/기준5 관계를 별도 SQLite에 원문·canonical·CKV로 연결하고 재조회/원본 없는 두 archive 재생·9거부 제어를 통과했다. Projection.Validate 시험을 다른 정렬 함수에 연결하는 부정확한1쌍은 제외, broad concept5경로 미작성 유지. 전체25관계 proposed·3요구 spec_unapproved·검토된경로0/current없음/human0/model0. 신규FINAL로 합산하지 않으며 N11 pool13/8·승인0, 전체8/18·요청5/15·잔여10개 유지. 다음 관계/원문 범위 판정·5경로 보강과 N11 독립 입력 확보다.
