# A0 요구사항·코드·검증 기준선

작성일: 2026-09-30 · 조사 기준 커밋: `7596bb6` · 대상 브랜치: `feat/spec-driven-knowledge-system`

후속 상태: 이 문서의 D5 미승인 표기는 A0 조사 당시의 기록이다. 사용자는 2026-10-01에 D5-01–07 설계를 [`D5-DECISION-REVIEW.md`](./D5-DECISION-REVIEW.md)대로 승인했다.

## 판정 규칙

이 문서는 기존 기능을 새 WBS의 완료로 승격하지 않는다. `부분`은 현행 구현과 시험이 있지만 v2 수용 조건의 한 축이 없는 상태, `미구현`은 목표 공개 계약을 제공하지 않는 상태다. `구조 통과`는 결정적 fixture 시험만 뜻한다. 모델 검색 품질은 B0/B1까지 `unmeasured`다. D1의 공통 20개 ID는 고정하며 개별 온톨로지 사실은 계속 `proposed`다. D5 세부 정책은 기술 검토용 기준선이며 제품 소유자의 명시적 승인을 기록하지 않는다.

요구사항 원본은 `study/docs/reviews/knowledge-system/spec-driven-requirements.md`, 새 완료 조건은 [`DELIVERY-PLAN-V2.md`](./DELIVERY-PLAN-V2.md), 공개 v2 목표는 [`PUBLIC-CONTRACT-V2.md`](./PUBLIC-CONTRACT-V2.md)이다. 아래 경로는 이 저장소 루트 기준이다. 시험 경로는 **기존 관련 시험**이며, 남은 오라클은 해당 WBS 작업에서 새로 고정해야 한다.

## 기존 요구사항 추적

| ID | 현행 코드·관련 시험 | 현재 판정 → 다음 검증 |
|---|---|---|
| INV-01 | `internal/system/semantic/alignment.go`, `internal/system/mcp/alignment_test.go` | 부분 → A3/A4의 프로젝트·작업 트리·비Git 좌표 일치와 S-07/08 |
| INV-02 | `cmd/cks/setupcli/setup.go`, `internal/system/semantic/store_test.go` | 부분 → A3/A8의 실패 후보 보존·세 엔진 동일 ID·S-09 |
| INV-03 | `internal/vector/chunk/chunk_test.go`, `internal/system/semantic/extract_markdown_test.go` | 부분 → A4의 보관 원문·v2 범위 해시·과거 인용 재검증 |
| INV-04 | `internal/system/semantic/validate.go`, `internal/system/semantic/assertion_test.go` | 부분 → A5.1/A5.5의 새 관계 타입·검토자·스냅샷·이유 검증 |
| INV-05 | `internal/system/composer/stage2/ontology_test.go` | 부분 → A6의 공개 플래그·다의어·오류·예산 초과 후보 보존 |
| INV-06 | `internal/system/mcp/schema_golden_test.go`, `pkg/system/contract/pack.go` | 부분 → A7.1/A8의 v1/v2 DTO·해시·CLI/MCP 골든과 이전 |
| INV-07 | `pkg/vector/types/embed.go`, `pkg/vector/types/embed_test.go` | 부분 → A2의 Ollama 다이제스트·전처리 정체성·구버전 거부 |
| FR-01 | `internal/graph/audit/audit.go`, `internal/graph/persist/sqlite_reader.go`, `cmd/graph/audit_test.go` | 구조 통과 → A8에서 대상 커밋의 현재 File 집합 재감사; 역사 Hunk 제외 |
| FR-02 | `internal/vector/store/sqlitevec/store.go`, `internal/vector/store/sqlitevec/store_test.go` | 부분 → A1의 취소/상한/incomplete, S-01 exact 순위; 실제 Recall@K는 B1 |
| FR-03 | `internal/vector/chunk/chunk_test.go`, `internal/vector/query/citation_test.go` | 부분 → A1/A4의 앞·중·끝 원문 인용·부모/자식 ID, B1 장문 회수율 |
| FR-04 | `internal/system/eval/runner.go`, `internal/system/eval/scenario_test.go` | 부분 → A1/A8의 답 없음·중단·버전 충돌 상태 골든, B1 실제 기권률 |
| FR-05 | `internal/system/semantic/extract_markdown.go`, `internal/system/semantic/ckg_anchor_test.go` | 부분 → A4/A5.1의 보관 원문과 CKV 청크·CKG 코드 연결, B1 추출 오차 |
| FR-06 | `internal/system/semantic/extract_ontology.go`, `internal/system/semantic/extract_ontology_test.go` | 부분 → A5.1/A5.2/A5.4의 20개 고정·팩 버전/타입/롤백 |
| FR-07 | `internal/system/composer/stage2/ontology_test.go` | 부분 → A6의 원문 질의·복수 후보·기본 K 보존, B1 ablation |
| FR-08 | `internal/system/semantic/trace.go`, `internal/system/semantic/test_run_test.go` | 부분 → A5.3/A5.5의 패치/실행/사람 수용 판정 분리 |
| FR-09 | `cmd/cks/doctorcli`, `cmd/cks/setupcli`, `scripts/wbs-install-smoke.sh` | 부분 → A7.1–A7.5의 편의 명령·세 대상 깨끗한 설치/서명/롤백 |
| FR-10 | `cmd/cks/setupcli/setup_test.go`, `cmd/cks/filelistcli/filelist_test.go` | 미구현(작업 트리) → A3/A4의 수정·신규·재수정 스냅샷과 S-08 |
| NFR-01 | `internal/system/eval`, `scripts/wbs-smoke.sh` | 미측정 → B0/B1의 승인 질문/정답·동일 모델/하드웨어 품질 비교 |
| NFR-02 | `internal/vector/discover/discover_test.go`, `cmd/cks/doctorcli/doctor_test.go` | 부분 → A4/A8의 공통 입력 매니페스트·민감 경로/링크·권한 실패 |
| NFR-03 | `internal/system/semantic/store_test.go`, `internal/vector/store/sqlitevec/migrate_test.go` | 부분 → A5.1/A5.4/A8의 v1–v4 의미 투영 읽기/쓰기·롤백 |
| NFR-04 | `cmd/cks/doctorcli/doctor.go`, `scripts/wbs-install-smoke.sh` | 부분 → A4/A7.1의 파서 오류/미지원 수·AST 미추론 공개 상태 |
| NFR-05 | `scripts/check-docs.py`, `internal/system/mcp/schema_golden_test.go` | 구조 통과 → 매 공개 계약 변경마다 `make docs-check`·골든 재생 |

## v2 추가 요구사항 추적

| ID | 현행 충돌/시작점 | 미통과 오라클·선행 작업 |
|---|---|---|
| R2-01 | `EmbeddingIdentity.Checksum`에는 모델 다이제스트·query 정책이 없다 | A2: 같은 태그/차원 다른 바이트, 중간 변경, 재색인 오류 |
| R2-02 | CKS 의미 투영은 프로젝트/데이터셋을 갖지만 CKV/CKG의 공유 불변 좌표는 없다 | A3: 두 저장소·두 스냅샷·두 모델, 승격 혼입 0 |
| R2-03 | committed 원천 검증은 있으나 작업 트리 blob 보관·v2 인용은 없다 | A4: 과거 원문 재검증, 소스 변경/손상/링크 거부 |
| R2-04 | 필터 부족 시 exact 보충은 있으나 부분 결과 상태 계약은 없다 | A1: 취소·시간/메모리 상한에서 정상 완료 금지 |
| R2-05 | `stage2` Go 경로의 선택형 boost는 공개 런타임 플래그가 아니다 | A6: 미등록·모호·저장소 오류에서 원래 상위 K 동일 |
| R2-06 | 정확 테스트 실행 기록은 있으나 외부 패치 신원/기준 판정은 없다 | A5.3: 성공 실행만으로 accepted 금지, 실패 승격 금지 |
| R2-07 | 호스트 패키지/빌드 이미지 스모크만 있다 | A7.2–A7.5: macOS arm64, Linux arm64/amd64 각각 추출·세 프로젝트·변조 거부 |
| R2-08 | 구조 평가와 실모델 품질의 상태를 문서에서 분리했다 | A8은 `unmeasured`, B/C는 별도 정답·원자료·재평가 |
| R2-09 | 현행 `OntologyPack`은 프로젝트 단일 YAML·전역 개념 ID | A5.4: 선택 팩 잠금/의존/타입/D1 불변·v1–v4 이전 |
| R2-10 | 코드·문서의 제안/검토는 있으나 업무 정책 시점·설계 이유 투영은 없다 | A5.5: 무근거/만료/충돌/비공개 이유 확정 0 |
| R2-11 | v1 `EvidencePack` 의미 필드는 선택형이고 도메인 팩 문맥은 없다 | A6/A7.1: 권한 필터→v2 인용/해시, 오류 시 K 보존 |

## 결정적 fixture와 실패 오라클 버전

| 묶음 | 현행 고정 입력 | 후속 고정/반증 |
|---|---|---|
| `F0-core` | `testdata/wbs-smoke`, `scripts/wbs-smoke.sh`; mock feature hash v1 | S-03/04/09를 한 커밋에서 재생하고 `current` 불변 검사 |
| `F1-vector` | `internal/vector/store/sqlitevec/store_test.go`, `internal/vector/chunk/chunk_test.go` | A1에서 >2,048 후보, K 충족/부족, 취소/상한 및 S-01/02 exact 순위 |
| `F2-meaning` | `docs/spec-driven/ontology-pilot.yaml`, `spec-pilot.yaml`; `internal/system/semantic/*_test.go` | D1 20개 개별 사실은 proposed; A5에서 구/신 술어와 v1–v4 골든 |
| `F3-install` | `scripts/wbs-install-smoke.sh`, `scripts/wbs-package-smoke.sh` | A7에서 대상별 추출 아카이브·세 독립 프로젝트·서명/변조·롤백 |
| `F4-domain` | `DOMAIN-PACK-CONTRACT-V1.md`의 예시만 존재 | A5.4/A5.5에서 팩 없음/잠금 불일치/충돌/비공개 fixture 생성 |
| `F5-public` | `internal/system/mcp/testdata/agent-mcp.schema.json`, v1 DTO 시험 | A7.1에서 실제 v1 JSON/해시와 v2 좌표/오류 골든을 각각 동결 |

fixture는 운영 승인 데이터를 만들지 않는다. `F2`의 `verified` 시험 레코드는 테스트 저장소 안에서만 쓴다. B0의 실제 질문 정답·Ollama 다이제스트·품질 임계치는 아직 없다.

## 지원·검증 매트릭스

| 대상 | 구조 증거 | 지원 판정 |
|---|---|---|
| macOS arm64 | 이 기준선의 전체 Go 테스트·vet·경계·문서 검사 통과; 과거 호스트 추출 아카이브 스모크는 `EXECUTION.md` 기록 | A7.2 새 아카이브/깨끗한 설치 전 `preview` |
| Linux arm64 | 과거 SQLite 헤더 포함 빌드 이미지에서 세 바이너리·세 프로젝트 스모크; `EXECUTION.md` 기록 | A7.3 깨끗한 런타임 아카이브 전 `preview` |
| Linux amd64 | 이 기준선에서 실행 증거 없음 | A7.4 전 `unsupported` |

로컬 Ollama 실모델이 없는 구조 시험은 지원 OS/CPU의 **임베딩 품질**을 증명하지 않는다. macOS의 sqlite-vec CGo는 SDK 사용 중단 경고를 출력했지만 위 네 명령은 종료 코드 0이었다.

## A0 실행 기록

기준 커밋에서 `GOCACHE=/private/tmp/ks-wbs-go-cache GOMODCACHE=/private/tmp/ks-wbs-mod-cache`를 지정해 다음을 실행했다.

| 명령 | 결과 |
|---|---|
| `go test ./...` | 통과 |
| `go vet ./...` | 통과 |
| `make boundaries` | 통과 (`engine boundaries: OK`) |
| `make docs-check` | 통과 (live documents 93개) |

A0는 추적·fixture·플랫폼과 검사 기준선을 고정하는 작업으로 완료한다. A1–A8의 구현·수용 시험, B/C 실모델 품질 판정은 미완료다.
