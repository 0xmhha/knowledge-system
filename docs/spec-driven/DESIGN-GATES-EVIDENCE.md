# D1–D4 설계 게이트 검토 기록

작성일: 2026-09-30 · 기준 코드: `feat/spec-driven-knowledge-system` @ `8834b18` · 범위: 로컬 Ollama, `knowledge-system` 파일럿, macOS arm64/Linux arm64·amd64. 이 문서는 설계 판정이다. v2 구현이나 실모델 성능 합격을 주장하지 않는다.

| 게이트 | 판정 | 근거와 남은 결정 |
|---|---|---|
| D1 도메인 모델 | **검토안 완료, 최종 승인 대기** | 18개 개념의 경계와 6개 능력 질문, 3개 요구사항·4개 기준을 코드/문서와 대조했다. 정의와 수용 기준의 충돌을 수정했다. 도메인 검토자의 승인 없이 전부 `proposed`다. |
| D2 캡처·신원 | **설계 결정 완료** | Go/TypeScript Git worktree와 비Git AST/CKV 실험, 두 staging 경로의 결정성, 독립 blob 읽기를 수행했다. 발견한 정렬·임시 경로·Go 모듈 오류의 수정 계약을 고정했다. 실제 v2 통합·실패 주입은 A4 게이트다. |
| D3 공개 계약 | **설계 결정 완료** | 실제 v1 MCP 입력 fixture의 `task`/`prompt` 불일치를 수정하고 필드/필수 목록 회귀 시험을 추가했다. v1/v2 DTO, 인용/해시/오류/롤백의 버전 경계를 `PUBLIC-CONTRACT-V2.md`에 고정했다. v2 실행 골든과 외부 소비자 재생은 A7.1/A8 게이트다. |
| D4 단계 간 정합성 | **설계 검토 완료** | A5.1이 A4의 원문 보관에 의존하도록 WBS를 고쳤고, 비Git 패치의 신원을 커밋 대신 전후 스냅샷으로 고쳤다. 아래 입력·출력·실패 행렬을 구현의 기준으로 삼는다. |

## D1. 도메인 모델과 질문 검토

파일럿의 주제는 **소스 코드 지식 시스템 자체**다. 온톨로지는 CKS의 의미 계층에서 검토 가능한 업무 용어와 타입 관계를 표현한다. CKG AST 노드와 CKV 청크를 온톨로지 개념으로 복제하지 않는다. `SourceOrigin`, `PatchAttempt`, `CriterionDecision`은 필요한 운영·증거 객체이지만 파일럿의 사용자 질의 어휘와 별개로 CKS 투영/기록 스키마에 둔다. 이 분리가 온톨로지 팩의 무제한 팽창을 막는다.

| 구분해야 할 개념 | 판정 기준과 코드 근거 |
|---|---|
| `project` / `dataset` / `source-snapshot` | 프로젝트는 격리 ID, 데이터셋은 한 번 만든 CKV·CKG·의미 결과, 스냅샷은 캡처한 소스 바이트다. 현재 `internal/system/semantic/model.go`의 `Snapshot`은 Git 커밋만 담고 `internal/setup/reindex.go`의 버전 디렉터리는 별도다. v2에서는 같은 HEAD라도 작업 트리 바이트가 다르면 스냅샷을 분리한다. |
| `source-file` / `evidence-span` / `vector-chunk` | 파일은 캡처 원본, 근거는 origin·경로·줄·해시의 재검증 단위, 청크는 검색용 파생물이다. `pkg/vector/types/chunk.go`의 청크 유사도는 근거의 진실성 승인이 아니다. |
| `code-symbol` / `document-section` | 전자는 `pkg/graph/types/node.go`의 AST 심볼, 후자는 `internal/system/semantic/model.go`의 문서 줄 범위다. 같은 문자열 이름이나 청크 위치로 타입을 대체하지 않는다. |
| `claim` / `semantic-assertion` / `concept` / `term` | 주장은 검토 가능한 문장, 단언은 타입·방향·근거가 있는 관계, 개념은 정의/포함/제외 범위, 용어는 언어별 표현이다. `internal/system/semantic/model.go`가 네 타입을 구분한다. 동음이의어는 `internal/system/semantic/matcher.go`에서 복수 후보로 남긴다. |
| `requirement` / `acceptance-criterion` / `test-case` / `test-run` | 의도, Given/When/Then 조건, 테스트 정의, 특정 스냅샷의 실행 사건이다. `internal/system/semantic/trace.go`의 `linked`와 `test_run.go`의 성공은 `CriterionDecision.approved`가 아니다. |
| `policy` / `evidence-pack` | 정책은 캡처·제외·정화·검토를 제약하며 팩은 `pkg/system/contract/pack.go`의 제한된 응답이다. 팩에 정책 문구가 인용됐다고 정책이 적용됐다고 추론하지 않는다. |

검토자가 각 정의의 원천을 확인할 수 있도록 파일럿 항목의 줄과 현재 코드 앵커를 고정한다. 코드 앵커가 없는 항목은 **v2 설계 객체**이거나 기존 AST/테스트 노드로 표현되는 것으로 명시한다.

| 파일럿 개념과 정의 줄 | 현재 코드 또는 목표 계약 |
|---|---|
| `project` (`ontology-pilot.yaml:15–21`) | `internal/system/semantic/model.go:27`의 `ProjectID`; v2 안정 ID는 종단 간 설계 2.1절 |
| `dataset` (`:22–28`) | `internal/setup/reindex.go:16`의 버전 디렉터리/`current`; v2 단일 후보는 종단 간 설계 3절 |
| `source-snapshot` (`:29–35`) | `internal/system/semantic/model.go:27`은 현재 커밋형; v2 캡처 ID는 종단 간 설계 2.1–2.2절 |
| `source-file` (`:36–42`) | CKG `internal/graph/persist/manifest.go:36`와 CKV 매니페스트; v2 origin별 파일 레코드는 2.1절 |
| `code-symbol` (`:43–49`) | `pkg/graph/types/node.go:4`의 AST 노드와 canonical ID |
| `vector-chunk` (`:50–56`) | `pkg/vector/types/chunk.go:201`의 검색 청크 |
| `document-section` (`:57–63`) | `internal/system/semantic/model.go:50`의 원문 절 |
| `claim` (`:64–70`) | `internal/system/semantic/model.go:105`의 주장 |
| `evidence-span` (`:71–77`) | `internal/system/semantic/model.go:36`의 현행 커밋 근거; v2 origin/파일 해시 확장은 5절 |
| `semantic-assertion` (`:78–84`) | `internal/system/semantic/model.go:116`의 타입·방향 관계 |
| `concept` (`:85–91`) | `internal/system/semantic/model.go:61`의 검토 개념 |
| `term` (`:92–98`) | `internal/system/semantic/model.go:73`의 언어별 표현 |
| `requirement` (`:99–105`) | `internal/system/semantic/model.go:81`의 명세 |
| `acceptance-criterion` (`:106–112`) | `internal/system/semantic/model.go:95`의 Given/When/Then |
| `test-case` (`:113–119`) | CKG 테스트 심볼과 `internal/system/semantic/test_run.go:208`의 정확 Go 테스트 선택; 독립 TestCase 레코드는 v2 계약 |
| `test-run` (`:120–126`) | `internal/system/semantic/test_run.go:70`의 실행 보고서; v2 `TestExecution`은 종단 간 설계 6절 |
| `policy` (`:127–133`) | `system/policies/sanitization_rules.yaml`과 종단 간 설계 2.1·8절의 입력/정화 정책 |
| `evidence-pack` (`:134–140`) | `pkg/system/contract/pack.go:176`의 응답 |

관계 어휘는 최소한 `DocumentSection SUPPORTS Claim`, `Claim CONTRADICTS Claim`, `Claim ABOUT Concept`, `Concept IMPLEMENTED_BY CodeSymbol`, `CodeSymbol TESTED_BY TestCase`, `AcceptanceCriterion ACCEPTED_BY TestCase`다. 방향을 뒤집지 않는다. `ACCEPTED_BY`는 **검토된 테스트 연결**이라는 기존 이름을 유지하지만 사람의 수용 판정은 별도 `CriterionDecision`만 기록한다. 모든 verified 관계에는 양쪽 원천·현재 스냅샷·검토자가 필요하다. 요구사항→개념→코드→테스트의 경로는 MDD/SDD/지식 데이터 개발의 연결을 보여 주지만, 테스트 통과와 의미 승인이 없으면 완료 상태로 승격하지 않는다.

| 능력 질문 | 기대되는 구조 답과 금지되는 해석 |
|---|---|
| 인용과 CKV 청크의 생성 스냅샷은? | 동일 `project_id/dataset_id/snapshot_id/origin_id`와 보관 원문 해시를 반환. 현재 커밋만으로 작업 트리 인용을 답하지 않음. |
| 어떤 심볼이 요구를 구현하고 어떤 테스트가 검증하나? | 검토된 `IMPLEMENTED_BY/TESTED_BY/ACCEPTED_BY` 경로와 별도 실행 상태를 반환. 연결만으로 테스트 통과/수용을 주장하지 않음. |
| 주장과 회수 코드가 같은 스냅샷인가? | 두 EvidenceSpan과 코드 앵커의 좌표를 비교. 경로·줄만 같은 다른 버전은 불일치. |
| 어떤 검토된 두 주장이 상충하나? | `CONTRADICTS`와 양쪽 원문을 제시. 모델의 단독 추측은 확정 관계가 아님. |
| 구현 관계 검증에 빠진 증거는? | 개념 승인, 양쪽 원문, CKG canonical ID, 현재 스냅샷, 검토자 중 결손을 각각 출력. |
| 모호한 한영 표현은 어떤 개념을 가리키나? | 모든 일치 후보와 상태를 제시. 자동 병합/단일 확정/원문 질의 삭제 금지. |

수정한 파일럿은 18개 개념을 유지하고 `source-snapshot`을 Git 커밋뿐 아니라 캡처한 작업 트리·비Git 원본으로 정의한다. `source-file`, `evidence-span`, `test-run`도 스냅샷 좌표에 맞췄다. `spec-pilot.yaml`의 희소 필터 기준은 작은 정확 검색의 거리 순서와 큰 검색의 상한/취소 `incomplete`를 분리했다. 소스 근거 기준도 커밋 전용 문구에서 캡처 스냅샷으로 바꿨다. 세 요구사항은 전체 FR/INV/NFR를 대체하지 않는 파일럿이다.

**검토자에게 남긴 결정:** 18개 개념의 정의·제외 범위, `ACCEPTED_BY` 명칭의 업무상 오해 가능성, 한영 별칭의 다의어와 누락 개념, 여섯 질문의 허용 답, 세 파일럿 요구사항과 네 기준의 의미를 승인/수정/반려해야 한다. 각 결정에는 검토자, 원천 커밋·줄, 날짜, 이유를 남긴다. 현 시점에 검토자 승인이나 질문별 정답을 만들어 내지 않는다. 따라서 이 게이트만 열려 있다.

## D2. 기술 실험과 설계 결정

`/private/tmp/ks-design-d2-mr8wc1hw`에서 `go.mod`, Go 함수, TypeScript 함수, Markdown 한 파일을 만들었다. Git HEAD를 기준으로 두 개의 임시 worktree에 동일한 수정 바이트를 오버레이했다. 동일한 쓰기 가능한 Go 캐시에서 CKG 두 빌드의 그래프 다이제스트는 모두 `e1351f31b7943132c08bd6483cccbff520b6048dc50d9e2eedd532c87e4b7941`이고 두 수정 함수의 canonical ID가 같았다. 각 빌드는 HEAD Commit 노드 1개를 보존했다. 비Git 디렉터리는 CKG 7노드/5엣지, CKV 6청크, canonical 2/2, Commit 노드 0개와 빈 `indexed_head`였다. 별도 `sources/blobs/<sha256>`에 저장한 네 파일은 staging 경로 없이 다시 읽고 SHA-256을 검증했다.

**반증된 가정:** Git worktree의 CKV는 심볼 두 개 모두 canonical ID를 잃어 0/2였다. 같은 줄 Hunk/구문 노드를 심볼보다 먼저 선택하는 현재 정렬 알고리즘이 원인이다. CKG/CKV 매니페스트에는 staging 절대 경로도 남았다. 쓰기 불가 Go 캐시에서는 CKG 파서 오류 0건이면서 Go canonical ID가 비었다. `SOURCE-SNAPSHOT-ADR.md`와 종단 간 설계 2절은 이 세 결함의 수정 계약을 기록한다. `A4`는 새 규칙의 실패 주입, 큰 저장소·여러 언어, 원본 제거 후 인용, 파일 변경/링크 공격을 통과해야 완료된다.

## D3. 공개 계약 검토

현행 `pkg/system/contract/citation.go`의 v1 키는 커밋을 무시하고, `pack.go`의 해시는 선택형 semantic overlay를 제외한다. 같은 DTO에 v2 좌표를 붙여 사용하면 오래된 소비자의 해시 재계산과 중복 제거가 잘못된다. `PUBLIC-CONTRACT-V2.md`에 도구 이름별 v1/v2 선택, source mode별 인용, 별도 v2 해시, 오류, 재색인·롤백 순서를 고정했다. 현재 등록 도구는 `prompt`를 필수 입력으로 쓰는데 vendored fixture는 `task`를 적고 있었고, 이름만 비교하는 테스트는 이를 통과시켰다. fixture와 필드 검사를 수정했다.

## D4. 단계 간 계약 점검

| 단계 | 입력 → 출력 | 책임/거부 조건 |
|---|---|---|
| D1–D4 → A0 | 승인된 파일럿 범위, 캡처/공개 계약, 추적표 → 고정 fixture | CKS 설계 책임. 미승인 도메인 범위를 verified로 쓰지 않음. |
| A0 → A1/A2/A3 | v1 골든·구현 기준선 → 검색 상태/모델 신원/좌표 | CKV와 CKS 공통 오류. `incomplete`·모델 drift·ID 혼합을 성공 취급하지 않음. |
| A3 → A4 | 프로젝트/스냅샷/데이터셋 ID → 불변 staging·원문 blob | CKS 캡처 책임. CKG/CKV는 동일 파일 목록만 읽고 빌드 경로를 공개하지 않음. |
| A4 → A5.1/A5.2/A5.3 | 보관 원문·공통 ID → 의미 투영·검토·패치 기록 | CKS 의미 책임. Git HEAD만으로 작업 트리/비Git 근거를 검증하지 않음. |
| A1/A3/A5.1 → A6 | 검증된 후보·관계 → 선택형 온톨로지 질의 | CKS 런타임 책임. 모호·미검토·예산 초과 시 기본 후보 집합 보존. |
| A2/A4/A5.3/A6 → A7.1–A7.5 | 세 엔진/의미 좌표·패치 상태 → 설치/서빙/롤백 패키지 | CKS 설치 책임. 한 엔진 실패, 구 소비자 오인, 미검증 바이너리/데이터 승격 거부. |
| A1–A7.5 → A8 | 기능별 성공/실패·이전 골든 → 구조 통합 보고서 | `natural_language_quality=unmeasured`, 기본 온톨로지 off, 패키지는 preview. |
| A8 → B0/B1 | 구조 승인·로컬 Ollama·검토 질문 → 실모델 평가 원자료 | 평가 책임. 모델/정답/하드웨어를 결과 보기 전에 고정; 수치 미측정은 합격 아님. |
| B1 → C0/C1 | 분류된 실패와 고정 조건 → 수정·재평가·기본값/출시 판정 | CKS/CKV/CKG 책임. 같은 최종 질문을 튜닝에 쓰거나 스냅샷 안전 게이트를 완화하지 않음. |

교차 불변식은 `project_id/dataset_id/snapshot_id`의 일치, 원문 보관본의 해시 검증, 한 후보/한 `current`, v1/v2 공개 형식 분리, `linked`/`pass`/`approved` 상태 분리다. 위 WBS 선후관계와 오류가 이를 따라야 한다. B/C의 실제 Ollama 모델과 성능은 의도적으로 나중에 결정하는 **실험 입력·결과**이며 A의 데이터 소유권이나 인용 계약을 바꾸는 자유 변수가 아니다.
