# A8 구조 통합 게이트 기록

상태: **A0–A8 구조 개발 게이트 통과** (2026-10-01, 코드 커밋 `ee6b5e5`). 이 문서는 `DELIVERY-PLAN-V2.md`의 A0–A8 수용 조건과 실행 증거를 분리한다. 테스트 성공은 실모델 검색 품질이나 실제 프로젝트 정책의 진실성을 뜻하지 않는다. B/C의 로컬 Ollama 평가는 사용자 결정에 따라 뒤로 미뤘다.

## 이번 통합 실행

기준 브랜치 `feat/spec-driven-knowledge-system`. 2026-10-01 macOS arm64에서 최근 코드에 대해 아래 검사를 다시 실행했다. 패키지 매트릭스도 이 변경을 포함한 macOS 아카이브와 Linux fixture 빌드에서 재검증했다.

| 게이트 | 결과 | 판정 범위 |
|---|---|---|
| `go test ./...` | 통과 | 현재 변경 포함 전체 Go 단위·통합 테스트 |
| `go test -race ./internal/setup ./internal/system/evidencev2 ./internal/system/semantic ./internal/system/knowledgepack ./internal/system/mcp ./cmd/cks/knowledgecli ./cmd/cks/setupcli` | 통과 | 캡처·의미 투영·v2 인용·팩·MCP·CLI·동시 검토 기록 범위. 다른 패키지의 이전 race 결과는 별도 실행 기록 |
| `go vet ./...`, `make boundaries`, `make docs-check` | 통과 | 정적 검사, 엔진 경계, 103개 살아있는 문서의 CLI 계약 |
| `scripts/wbs-identity-smoke.sh` | 통과 | 실제 CKG/CKV/CKS 세 계층 신원. 손상 시 공개 상태 계약을 `identity_status=invalid`, `reindex_required=true`로 점검하도록 스모크 갱신 |
| `scripts/wbs-source-modes-smoke.sh` | 통과 | 새 캡처 정책에서 Git 작업 트리·비Git 캡처, 빌드 중 변경 거부 |
| `scripts/wbs-smoke.sh` | 통과 | 세 엔진 구성, CKV 청크·CKG 심볼·CKS 추적/테스트 |
| `scripts/wbs-knowledge-lock-smoke.sh`, `scripts/wbs-patch-smoke.sh` | 통과 | 새 캡처 정책에서 잠금, v4 팩 타입, 관계 후보, ADR 검토 기록, 패치 승인·승격·롤백. 실제 MCP v2에서 충돌 정책과 정렬되지 않은 의미 저장소의 기본 인용 보존 |
| `python3 scripts/wbs-legacy-migration-smoke.py` | 통과 | 고정 main 기준선의 세 바이너리와 가짜 Ollama로 v1 응답을 재생하고, 새 `doctor`의 `legacy_unpinned` 및 새 v2 MCP의 `reindex_required` 확인 |
| `scripts/wbs-release-sign-smoke.sh`, `scripts/wbs-package-smoke.sh` | 통과 | 깨끗한 `ee6b5e5`에서 macOS arm64 시험 서명/별도 검증과 추출 바이너리의 세 프로젝트 설치 |
| `scripts/wbs-linux-package-smoke.sh arm64`, `amd64` | 통과 | 깨끗한 `ee6b5e5`에서 각 Linux 아키텍처의 별도 빌드·시험 서명·검증·추출, Go 없는 Debian 런타임에서 세 프로젝트 질의·두 버전 전환·롤백 |

스모크는 결정적 mock 임베딩을 사용했다. Linux 패키지는 시험 키와 `test-signed-preview` 범위만 사용했다. 추출 런타임에 Go 컴파일러가 없던 fixture는 실제 Go 소스가 없는 Go 모듈, TypeScript, 미지원 Python+Markdown이었다. 실제 Go 소스 AST 색인에는 현재 Go 도구 체인이 필요하다.

Linux 스모크를 이번 호스트의 Bash 3.2에서 다시 실행하며 빈 `cache_args` 배열의 `set -u` 확장 오류를 발견했다. 빈 배열의 이식 가능한 확장으로 고친 뒤 arm64와 amd64를 각각 재실행해 통과했다. 최종 세 플랫폼 패키지는 깨끗한 `ee6b5e5` 코드 커밋에서 생성한 구조 시험물이다. 원 저장소의 운영 릴리스 서명이나 의존 라이선스 승인이 아니다.

## 이전 호환과 데이터 마이그레이션

1. 기존 v1 근거 팩/의미 투영은 원래 DTO·해시·저장 바이트를 유지한다. v2는 별도 명시 MCP 도구의 캡처 원문 좌표와 `sha256-v2`를 사용한다. v1/v2/v3/v4 의미 JSON은 읽기 전용으로 읽고, 저장소 테스트에서 v4→v1 롤백 시 원문 바이트가 바뀌지 않았다.
2. 새 의미 쓰기는 v3의 `CHECKED_BY`를 사용하고 v1/v2의 `ACCEPTED_BY`는 구 데이터 읽기 호환으로만 허용한다. v4는 `--include-packs --version-dir`의 명시적 잠금 팩 타입 투영이다. 팩 없는 기본 경로는 v3다.
3. 새 pinned 후보에는 `project_id`·`snapshot_id`·`dataset_id`, 캡처 정책·파일 매니페스트 다이제스트가 필요하다. 구 commit-only/unpinned 데이터는 `doctor`에서 `legacy_unpinned`와 재색인 요구로 분류한다. 작업 트리·비Git 출처 인용은 기존 커밋 인용으로 변환하지 않는다.
4. 정책/ADR/관계의 별도 검토 기록은 검토한 후보의 원문 SHA-256과 스냅샷 ID를 보관한다. 새 잠금과 새 후보를 빌드해야 효력이 생긴다. 구 인라인 한 명 검토 표기는 `min_approvals=1`일 때만 읽기 호환으로 받는다. 로컬 검토자 ID는 인증된 신원이 아니다.
5. 새 패키지는 배포물과 분리된 `release.json`/Ed25519 서명을 독립 검증기로 추출 전에 확인한다. 이전 무서명 압축물은 검증된 새 릴리스로 자동 승격하지 않는다.
6. 민감 경로 선택 규칙을 확장하면서 캡처 정책 식별자를 `capture-policy-2026-10-01.2`로 올렸다. 이 브랜치의 `.1` 시험 데이터셋은 새 신원 검증을 통과하지 않으므로 재색인이 필요하다. 의미 투영의 v1–v4 읽기 호환과는 별개의 캡처 정책 변경이다.
7. 고정 main 커밋 `1ded9b3e`에서 구 CKG/CKV/CKS를 별도로 빌드했다. 가짜 Ollama의 고정 벡터로 만든 실제 v1 데이터셋에 대해 구 MCP v1은 같은 커밋의 README와 Go 소스를 인용했고, 새 `doctor`는 `legacy_unpinned`, `reindex_required=true`를 냈다. 새 v2 MCP는 백엔드 모델 상태에 앞서 `reindex_required`를 반환한다. 이 호환 재생은 실모델 성능 평가가 아니다.
8. 구 바이너리가 새 pinned v2 벡터 DB를 여는 역방향 읽기는 지원 계약이 아니다. 구 CKV가 새 임베딩 신원 포맷을 해석하지 못해 `embedding identity mismatch`로 거부하는 것을 확인했다. 구 데이터셋은 구 도구로 읽고, 새 데이터셋에는 새 바이너리를 사용한다.

## A0–A8 판정과 알려진 제한

| WBS | 구조 게이트 판정 | 증거와 남은 범위 |
|---|---|---|
| A0–A3 | 통과 | 요구·코드·시험 추적과 CKV 정확 필터/상한/취소, 가짜 Ollama 신원 변경, CKG·CKV·CKS의 빌드 전 공유 ID 및 구버전 분류가 통과했다. 실모델 품질은 B다. |
| A4 | 통과 | 작업 트리·비Git 캡처, 세 ID, CKV 입력 해시, 과거 보관본 인용, 링크·비밀·빌드 중 변경 거부를 구조 시험했다. macOS/Linux의 `openat(O_NOFOLLOW)`와 inode 재검사를 사용한다. 기본 파일 10만 개·단일 32 MiB·총 4 GiB 상한을 공유하고 500개 파일·외부 문서의 합산을 시험했다. 10만 파일 운영 비용과 사전 용량 추정은 측정하지 않았다. |
| A5.1–A5.4 | 통과 | 보관본 좌표와 v1–v4 의미 투영 읽기, `CHECKED_BY` 쓰기, 20개 코어의 근거/보류 보고, 외부 패치와 기준별 승인 분리, 팩 타입·잠금·DAG·네임스페이스 실패를 시험했다. 승격 가능한 exact runner는 현재 Go 테스트이며 사용자 지정 명령 성공을 기준 승인으로 오인하지 않는다. 로컬 검토자 ID는 인증된 신원이 아니다. |
| A5.5 | 통과 | 잠금된 `trace-links`의 별도 검토 기록, ADR 선언, 정렬된 의미 투영, CKG 코드·테스트 앵커, 동일 데이터셋의 보관 줄 해시를 모두 맞춘 경로만 v2에 인용한다. 정상/미래 ADR/타 데이터셋과 정책 충돌·비공개 우선·만료·보관본 손상을 시험했다. 충돌 정책을 `required_behavior`로 승격하지 않는다. 조직 인증·실제 정책 사실 판정은 범위 밖이다. |
| A6 | 통과 | 선택형 v2 경로의 기본 CKV+CKG 후보 유지, 모호/미등록 표현 기권, 권한·저장소 오류·예산 초과 폴백, 인용·해시 검증을 시험했다. 실제 MCP에서는 충돌 정책과 불일치 의미 저장소가 기본 인용을 보존했다. 모든 운영 오류 조합을 열거한 것은 아니다. 기본 플래그는 꺼져 있다. |
| A7.1–A7.5 | 통과 | 구 main 바이너리의 v1 조회를 재생하고 새 바이너리의 재색인 오류를 확인했다. `mcp gen-config`는 사용자 파일을 덮어쓰지 않는다. 세 독립 프로젝트는 두 버전 전환·롤백·과거 인용을 확인했다. macOS arm64와 Linux arm64/amd64의 추출 패키지는 시험 서명과 라이선스 파일 목록을 검증했다. 운영 서명 키와 법적 승인은 C1이다. |
| A8 | 통과 | 전체 Go 테스트, 관련 race, vet, boundaries, docs-check, 세 엔진/캡처/지식/패치/구버전/세 플랫폼 스모크를 최종 코드에서 재실행했다. 이 문서에 기능별 known limits를 공개했다. |

품질/지연/정답률은 **unmeasured**다. A의 구조 개발 게이트만 완료했다. B/C와 운영 릴리스 적합성은 완료로 선언하지 않는다.

배포 의존성 확인: 이전 `sqlite-vec-go-bindings v0.1.6`과 후속 `v0.1.7-alpha.2` 태그 아카이브에는 라이선스 파일이 없었다. [상위 저장소의 누락 이슈](https://github.com/asg017/sqlite-vec-go-bindings/issues/11)와 별도로, MIT·Apache 파일이 실제 Go 모듈 아카이브에 포함된 [커밋 `b64d0e563e61`](https://github.com/asg017/sqlite-vec-go-bindings/tree/b64d0e563e615c99c19df431909a5970963d8fc0)을 의존성으로 고정했다. 해당 커밋의 C 확장은 `v0.1.7-alpha.2`이며 종전 `v0.1.6`과 바이트가 다르다. CKV 저장/색인/조회 대상 테스트와 전체 Go 테스트·vet·문서·경계 검사가 통과했다. macOS arm64와 Linux arm64/amd64 시험 서명 패키지에서 연결된 Go 모듈 35개의 원천 라이선스 파일 누락 0건을 자동 검사했고, sqlite-vec의 `LICENSE-MIT`/`LICENSE-APACHE` 두 파일도 포함됐다. 이는 원천 파일 수집 확인이며 운영 배포의 법적 적합성 판정은 아니다. 시험 서명 패키지를 운영 배포물로 선언하지 않는다.

후속 변경: `cks` 최상위 실패 출력은 한 줄의 JSON `{code,message}`와 비정상 종료로 정규화했다. 알려진 오류 접두사를 안정 코드로 매핑하고, 중첩 오류의 원문·비밀·임시 경로를 공개 메시지에 복사하지 않는다. `cmd/cks` 단위 테스트와 지식 잠금/패치 스모크가 통과했다. 이 변경은 MCP v2의 구조화 오류 데이터와 모든 구 소비자 재생을 완료했다는 뜻이 아니다.

추가로 `cks.context.get_for_task_v2`의 실패 결과는 `IsError=true`, `code=<고정 코드>` 텍스트와 동일한 `{code,message}` 구조화 데이터를 함께 반환한다. 원문 오류에 `requires_v2`, `reindex_required`, `snapshot_mismatch`, `source_missing`이 있으면 그 코드를 보존하되 경로·원문·비밀은 복사하지 않는다. 관련 MCP 단위 테스트와 구 v1 소비자의 실제 조회 재생이 통과했다. 다른 MCP 도구의 모든 오류를 동일한 v2 형식으로 바꾸는 작업은 이 게이트의 계약에 포함되지 않는다.

외부 추적 경로 추가: `.cks/knowledge/trace-links/*.yaml`에 ADR→요구/수용 기준→CKG 코드→CKG 테스트의 선언을 보관하고, 별도 원문 해시·스냅샷 결합 검토 기록으로 사람 검토를 받는다. `semantic.store_path`가 설정된 선택형 v2 조회는 `LoadAlignedRetained`로 같은 데이터셋의 CKG/CKV/원문 좌표를 재검증한 다음, 의미 투영의 단일 `linked` 경로와 모든 ID·보관 줄 해시를 맞춘다. 인용 한도 12개와 본문 32 KB를 지키며, 구현 사실은 검증된 경로의 코드 인용으로만 표시한다. 정상 경로, 아직 유효하지 않은 ADR, 다른 데이터셋, 이전 응답 불변성, v2 JSON 왕복 테스트가 통과했다. 설정 저장소가 없거나 정렬되지 않으면 기존 정책/ADR 문맥과 기본 후보를 보존하고 구현 연결은 미검증으로 남긴다. 이 결과는 테스트의 실제 실행 성공이나 수용 기준의 사람 승인을 뜻하지 않는다.

선택형 v2 `coding_context`를 추가해 검토된 정책을 required behavior, 검토된 ADR을 rationale로 분리했다. 검증된 구현 앵커가 없으므로 implemented behavior는 비우고 불확실 사유를 넣는다. 새 필드가 들어간 응답과 필드가 없는 이전 v2 응답을 각각 JSON 왕복 후 무결성 재검증했다. 외부 앵커가 생기기 전까지 이것은 완전한 코딩 경로가 아니다.

동시 승격 회귀: `Promote`, `Rollback`, 검토 승격의 `current` 교체에 같은 비차단 파일 잠금을 적용했다. 패치 승격은 그 잠금 안에서 `base_version`이 여전히 활성인지 재확인하며, 성공적인 교체 뒤에만 재사용 가능한 `review-release.json`을 쓴다. 잠금 경합·링크된 잠금 파일·오래된 기준 버전과 24개 동시 검토 기록 작성의 race 시험이 통과했다. 외부 수동 파일 변경과 마커 기록 실패 뒤 자동 복구는 운영 범위의 알려진 제한이다.

기준별 검사 게이트 보강: 임의 명령의 종료 코드 0은 특정 테스트가 실행되었다는 증거가 아니다. `patch record-run`과 승인 단계는 이제 Go JSON 스트림에서 연결된 정확한 테스트 이름의 실행·통과가 관찰된 보고서만 받는다. 다른 언어의 exact runner가 추가되기 전까지 그 명령 결과는 일반 실행 증거로만 사용한다.

캡처 경계 보강: 저장소 하위 어느 위치의 `secrets`/`.aws` 디렉터리도 대소문자와 관계없이 민감 경로로 분류한다. 단일 파일 및 전체 캡처 용량 상한, 링크된 부모와 파일을 통한 루트 이탈 거부 시험을 추가했다. 보관 blob도 열기 전후 inode 일치와 no-follow 열기를 검사하며, 같은 해시의 외부 파일로 링크를 바꾼 경우에도 거부한다.

추가 실패 주입: 500개 소스 파일과 외부 문서 한 개를 함께 캡처할 때 외부 링크를 거부하고, 저장소와 외부 문서의 바이트를 합산한 한도를 적용하며, 이후 외부 원문 교체에도 이전 보관본을 유지하는 fixture가 통과했다. 검토 기록은 임시 파일 동기화 후 `os.Root.Link`로 공개해 동시 독자가 부분 YAML을 성공 기록으로 읽을 수 없게 했고, 24개 동시 작성·기존 파일 덮어쓰기 거부를 race 검사로 확인했다. 두 검사는 모든 파일 시스템·운영 규모의 성능 보증은 아니다.

설치 상태 일치: `doctor`와 실제 캡처가 하나의 민감 경로 판정 함수를 사용한다. `.npmrc`, `.netrc`, `credentials.json`, SSH 키, 인증서와 중첩된 `Secrets`/`.AWS`를 같은 방식으로 표시·거부한다. 비Git fixture에서 상태 진단의 `secret_path_count`와 캡처 실패가 일치함을 검증했다.

손상되거나 예전 정책으로 기록된 pinned 데이터셋은 `doctor`에서 `identity_status=invalid`, `reindex_required=true`로 표시하고 내부 오류 원문을 상태 JSON에 복사하지 않는다. 손상된 신원 fixture로 확인했다.
