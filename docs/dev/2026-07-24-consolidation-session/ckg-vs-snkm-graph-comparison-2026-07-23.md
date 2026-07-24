# code-knowledge-graph ↔ stablenet-knowledge-mcp/graph 코드 대조 리포트

- **작성일**: 2026-07-23
- **대상**:
  - A = `~/Work/github/code-knowledge-graph` (원 리포, main @ `ddb2bff`)
  - B = `~/Work/github/stablenet-knowledge-mcp/graph` (통합 리포 @ `5c0e1bf`)
- **방법**: 전체 코드/문서 파일을 모듈 경로(`github.com/0xmhha/code-knowledge-graph` ↔
  `github.com/stable-net/stablenet-knowledge-mcp`)·bm25 이동 경로·브랜딩 용어(cks/ckg/CKS/CKG ↔
  stablenet-knowledge)를 정규화한 뒤 diff. 빌드 아티팩트(`.next`, `web_assets`, `bin`,
  `eval/.ckg-data` 등 바이너리)는 제외.

## 배경

`stablenet-knowledge-mcp`는 커밋 `1894d3e` "Consolidate graph, vector, and system into the
stablenet-knowledge-mcp module"에서 세 독립 리포(ckg/ckv/cks)를 **루트 단일 Go 모듈**로 통합한
리포. `graph/`는 ckg의 이식본이며, `vector/`(구 ckv), `system/`(구 cks)과 형제 엔진으로 공존한다.

## 정량 요약

| 분류 | 파일 수 |
|---|---|
| 공통 파일 (아티팩트 제외) | 632 |
| 완전 동일 (import 경로 치환만) | 547 |
| 브랜딩 리네임만 (주석의 cks/ckg → stablenet-knowledge) | 40 |
| **실질 변경** | **45** |
| ckg에만 존재 | 179 (대부분 빌드 아티팩트) |
| graph/에만 존재 (신규) | 2 (`namespace.go`, `TRAVERSAL-DEPTH.md`) |

`eval/ckv-mirror`는 양쪽 내용 동일(A쪽 `.index` 로컬 아티팩트만 차이).

**핵심: 로직 변경은 사실상 2개 축(manifest 이중화, MCP 네임스페이스)에 집중. 나머지는
문서/주석 정합화.**

---

## 실질 변경 1 — Manifest 빌더 식별자 이중화 (최대 코드 변경)

`graph/internal/persist/manifest.go` (+64줄). 통합 빌드에서 graph/vector manifest를 구분하기 위한
additive 변경:

- **`Engine` 필드 신설** (`"graph"`) — 엔진별 버전 키 이름 대신 필드로 구분.
  additive + `omitempty`라 SchemaVersion 범프 없음.
- **`BuilderVersion` 신설 + `CKGVersion` 유지(dual-write)** — 레거시 `ckg_version` 키와 새
  `builder_version` 키에 **같은 값**을 양쪽 기록. 값이 동일하므로 캐시 키 해시가 안 바뀌어
  스퓨리어스 콜드 리빌드 없음.
- 신규 헬퍼:
  - `EffectiveBuilderVersion()` — BuilderVersion 우선, 없으면 CKGVersion 폴백
  - `WithGraphBuilderIdentity()` — 쓰기 직전 Engine/BuilderVersion 채움
- 읽기 경로는 **양방향 미러링**: 레거시 manifest(ckg_version만)도, 미래의 post-removal
  manifest(builder_version만)도 두 필드 모두 채워짐.

적용 지점:

| 파일 | 변경 |
|---|---|
| `internal/persist/manifest.go` `SetManifest` | DB 쓰기 시 dual-write |
| `internal/buildpipe/cache.go:427` | 캐시 재사용 판정 `old.CKGVersion` 직접 비교 → `EffectiveBuilderVersion()` 비교 |
| `internal/buildpipe/pipeline.go` `writeManifestJSON` | identity 주입 |
| `internal/persist/postgres_store.go:1371`, `chunked_export.go:41` | export 전 identity 주입 |
| `cmd/ckg/export_json.go` | JSON 헤더에 `engine`/`builder_version` 추가 |
| `internal/buildpipe/manifest_usable_downgrade_test.go` | builder_version-only manifest usable 판정 테스트 2개 추가 |

평가: back-compat이 양방향으로 처리된 모범적 additive 설계. ckg 쪽에는 이 개념 자체가 없음.

## 실질 변경 2 — MCP 툴 네임스페이스 (클라이언트 관점 breaking)

신규 파일 `graph/pkg/mcphandlers/namespace.go`:

```go
const ToolNamespace = "stablenet_knowledge.context."
func nsTool(name string, opts ...mcp.ToolOption) mcp.Tool // NewTool + prefix
```

- 8개 툴 등록이 전부 `mcp.NewTool(...)` → `nsTool(...)`로 변경
  (`handlers.go`, `impact.go`, `get_context.go`, `evidence.go`, `concurrency.go`,
  `change_history.go`). 클라이언트가 보는 이름: `stablenet_knowledge.context.find_symbol` 등.
  fused system 서버·vector 엔진과 단일 네이밍 컨벤션 통일이 목적.
- MCP 서버명 `"ckg"` → `"stablenet-knowledge-graph"` (`internal/mcp/server.go:23`).
- `internal/mcp/bench.go:44` — `s.GetTool(mcphandlers.ToolNamespace + name)`로 대응.
- 테스트 6개 파일(example_test, handlers_smoke_test, impact_test, evidence_handler_test,
  integration_test)이 네임스페이스드 이름으로 갱신.

**주의**: bare 이름(`find_symbol`)을 기대하는 기존 ckg용 MCP 클라이언트에는 breaking.
manifest와 달리 dual-name 등록 같은 하위호환 계층 없음.

## 실질 변경 3 — bm25 패키지 루트 승격

`graph/pkg/bm25` 삭제 → 리포 루트 `pkg/bm25`로 이동(vector 엔진과 공유).

- `okapi.go` / `scorer.go` / `scorer_test.go` / `tokenize.go`: **바이트 동일**
- `doc.go` / `example_external_test.go`: "shared core used by both engines" 취지의 주석만 갱신
- 소비처(`pkg/evidence`, `pkg/smartctx`, `pkg/store/external_surface_test.go` 등)는 import 경로만 변경

## 실질 변경 4 — 문서 재편

- **`docs/archive/` 31개 파일 미이관** — 의도적. `docs/DOC-MAP.md`에 "archive는 통합 리포로
  가져오지 않았고 pre-consolidation ckg 리포의 git history에서 읽는다"고 명시. 이에 맞춰 코드
  주석 속 아카이브 문서 참조 9곳이 `... (archived; pre-consolidation git history)` 표기로 일괄
  수정 (`cache.go`, `pr_history.go`, `pipeline.go`, `pr_ref.go`, `search_hit.go`,
  `find_symbol_options.go` 등).
- **`docs/TRAVERSAL-DEPTH.md` 신규** (graph/에만) — `find_callers`/`find_callees`/
  `impact_of_change`의 `depth=2` 기본값 근거를 Tier-2 문서로 승격. 기존 아카이브 대상
  `ckg5-depth-sweep-report-2026-05-20.md`를 가리키던 툴 설명·주석 4곳이 이 문서를 가리키도록 변경.
- `pkg/types/node.go:14` — CanonicalID 참조 문서를
  `code-knowledge-system docs/symbol-identity-design.md` → `graph/docs/adr/0001-canonical-symbol-id.md`로 교체.
- CLAUDE.md / README.md / VISION.md / CONTINUITY.md / CODE-STRUCTURE.md — "세 개의 sister repo" →
  "한 리포의 세 엔진(`graph/`·`vector/`·`system/`)" 서술로 전면 갱신. README 라이선스 링크는
  루트 `../LICENSE`로.
- `eval/stablenet/func-verify/ground-truth-A.json` — `db_rel`이
  `../../../../code-knowledge-system/data/...` → `../../../../system/data/...`.

## 실질 변경 5 — 브랜딩 리네임 (40파일, 주석만)

`internal/parse/{golang,typescript,solidity}` 테스트·주석, `pkg/types/enums.go`(30줄 전부 주석),
`internal/temporal` 등 — "CKS G5/G6", "cks 소비자" 표현을 "stablenet-knowledge …"로 변경. 코드 동일.

---

## ckg에만 있는 것 (179파일)의 정체

| 그룹 | 내용 |
|---|---|
| 빌드 아티팩트 | `web/viewer-next/.next`·`out`, `internal/server/web_assets`(~120파일), 루트 `ckg` 바이너리 — gitignore성 로컬 산물 |
| 루트로 이동 | `LICENSE`, `go.mod`/`go.sum`(단일 모듈화), `pkg/bm25`(6파일) |
| 의도적 미이관 | `docs/archive/`(31), `eval/stablenet/func-verify/Report-A*.md`·`results-A*.json`(7), `eval/stablenet-keyword/results.json` |
| 리포 루트 소관 이동 | `.githooks`/`.github`/`.claude` |
| 재빌드 산물 | `eval/.ckg-data`·`.synthetic-data`의 `graph.db`/`manifest.json` 차이 |

## 종합 평가

1. **포크-후-발산이 아니라 규율 있는 이식.** 547/632 파일이 경로 치환 외 동일, 로직 변경은
   manifest 이중화 + MCP 네임스페이스 두 축뿐.
2. **manifest 변경은 모범적**: dual-write + 양방향 폴백 + 캐시 무효화 회피 + 테스트 보강.
3. **비대칭 하나**: MCP 툴명은 하위호환 없이 일괄 리네임 — ckg용 클라이언트 설정은 graph/
   서버에 그대로 붙일 수 없음. 두 리포 병행 운용 시 유일하게 체감되는 breaking 포인트.
4. ckg(원 리포)는 통합 후에도 커밋 진행 중(`ddb2bff` README refresh 등) — 향후 ckg → graph/
   포워드-포팅 시 위 두 축(manifest·네임스페이스)과의 충돌만 주의하면 됨.
