# A8 구조 통합 게이트 기록

상태: **진행 중, A 단계 전체 미완료** (2026-10-01). 이 문서는 `DELIVERY-PLAN-V2.md`의 A0–A8 수용 조건과 실행 증거를 분리한다. 테스트 성공은 실모델 검색 품질이나 실제 프로젝트 정책의 진실성을 뜻하지 않는다. B/C의 로컬 Ollama 평가는 사용자 결정에 따라 뒤로 미뤘다.

## 이번 통합 실행

기준 브랜치 `feat/spec-driven-knowledge-system`, 최근 코드 검증 시점 HEAD `6121523`. 이 보고서는 검증 뒤 갱신했다. macOS arm64에서 다음을 실행했다. 패키지 매트릭스는 이 커밋 이전의 구조 변경 시점에 시험했다.

| 게이트 | 결과 | 판정 범위 |
|---|---|---|
| `go test ./...` | 통과 | 전체 Go 단위·통합 테스트 |
| `go test -race ./internal/system/semantic ./internal/system/evidencev2 ./internal/system/knowledgepack ./internal/system/eval` 및 `./internal/setup ./internal/system/patch ./cmd/cks/doctorcli` | 통과 | 의미 투영·v2 인용·팩·평가·캡처·승격·상태 진단 동시성 범위 |
| `go vet ./...`, `make boundaries`, `make docs-check` | 통과 | 정적 검사, 엔진 경계, 103개 살아있는 문서의 CLI 계약 |
| `scripts/wbs-identity-smoke.sh` | 통과 | 실제 CKG/CKV/CKS 세 계층 신원 |
| `scripts/wbs-source-modes-smoke.sh` | 통과 | Git 작업 트리·비Git 캡처, 빌드 중 변경 거부 |
| `scripts/wbs-smoke.sh` | 통과 | 세 엔진 구성, CKV 청크·CKG 심볼·CKS 추적/테스트 |
| `scripts/wbs-knowledge-lock-smoke.sh` | 통과 | 잠금, v4 팩 타입, 관계 후보, ADR 검토 기록, 새 스냅샷, 위조 원천 거부 |
| `scripts/wbs-release-sign-smoke.sh`, `scripts/wbs-package-smoke.sh` | 통과 | macOS arm64 외부 서명 검증과 추출 바이너리 세 프로젝트 설치 |
| `scripts/wbs-linux-package-smoke.sh arm64`, `amd64` | 통과 | 각 Linux 아키텍처의 별도 서명/검증/추출, Go 없는 Debian 런타임에서 세 프로젝트 질의 |

스모크는 결정적 mock 임베딩을 사용했다. Linux 패키지는 시험 키와 `test-signed-preview` 범위만 사용했다. 추출 런타임에 Go 컴파일러가 없던 fixture는 실제 Go 소스가 없는 Go 모듈, TypeScript, 미지원 Python+Markdown이었다. 실제 Go 소스 AST 색인에는 현재 Go 도구 체인이 필요하다.

## 이전 호환과 데이터 마이그레이션

1. 기존 v1 근거 팩/의미 투영은 원래 DTO·해시·저장 바이트를 유지한다. v2는 별도 명시 MCP 도구의 캡처 원문 좌표와 `sha256-v2`를 사용한다. v1/v2/v3/v4 의미 JSON은 읽기 전용으로 읽고, 저장소 테스트에서 v4→v1 롤백 시 원문 바이트가 바뀌지 않았다.
2. 새 의미 쓰기는 v3의 `CHECKED_BY`를 사용하고 v1/v2의 `ACCEPTED_BY`는 구 데이터 읽기 호환으로만 허용한다. v4는 `--include-packs --version-dir`의 명시적 잠금 팩 타입 투영이다. 팩 없는 기본 경로는 v3다.
3. 새 pinned 후보에는 `project_id`·`snapshot_id`·`dataset_id`, 캡처 정책·파일 매니페스트 다이제스트가 필요하다. 구 commit-only/unpinned 데이터는 `doctor`에서 `legacy_unpinned`와 재색인 요구로 분류한다. 작업 트리·비Git 출처 인용은 기존 커밋 인용으로 변환하지 않는다.
4. 정책/ADR/관계의 별도 검토 기록은 검토한 후보의 원문 SHA-256과 스냅샷 ID를 보관한다. 새 잠금과 새 후보를 빌드해야 효력이 생긴다. 구 인라인 한 명 검토 표기는 `min_approvals=1`일 때만 읽기 호환으로 받는다. 로컬 검토자 ID는 인증된 신원이 아니다.
5. 새 패키지는 배포물과 분리된 `release.json`/Ed25519 서명을 독립 검증기로 추출 전에 확인한다. 이전 무서명 압축물은 검증된 새 릴리스로 자동 승격하지 않는다.

## 남은 수용 조건과 알려진 제한

| WBS | 현재 남은 조건 | 현재 동작/위험 억제 |
|---|---|---|
| A4 | 적대적 경로 교체·대형 저장소/외부 문서의 넓은 실패 주입과 공개 오류 계약 | 현재 캡처는 macOS/Linux의 openat O_NOFOLLOW로 경로 성분의 링크를 거부하고 보관 blob을 검사한다. 중첩·대소문자 변형의 비밀 경로와 파일/총량 제한을 시험했다. 동시 내용 변경·대형 저장소·외부 문서의 넓은 실패 주입 증거가 더 필요하다. |
| A5.3 | Go 이외 테스트 프레임워크의 기준별 정확 실행과 검토 기록의 동시 작성/권한 정책 | 실패/미승인 패치는 승격되지 않는다. 현재 승격 가능한 exact runner는 Go 테스트다. 사용자 지정 명령의 성공은 실행 기록으로만 남고 기준 승인 근거로 받아들이지 않는다. 활성 버전 교체는 데이터셋 잠금으로 직렬화하고, 검토 승격은 잠금 안에서 기준 버전을 재확인한다. |
| A5.5 | 검토된 정책/ADR→요구→CKG 코드→테스트 외부 앵커, 스냅샷/시점/권한 교차 검증 | 로컬 정책/ADR 양쪽 끝점 관계만 검증·인용한다. 외부 끝점은 `proposed`만 받고 ADR의 선언 ID를 v2 확정 링크로 내보내지 않는다. |
| A6 | 외부 앵커를 포함한 제한 깊이 탐색, 개념 모호성/오류/예산 폴백의 실제 통합과 코딩 문맥 필드 완성 | 기본 CKV+CKG 후보는 유지한다. 선택형 v2는 검토된 공개 로컬 정책/ADR/관계만 인용한다. |
| A7.1/A7.5 | 구 소비자 v2 재생, MCP 구조화 오류와 CLI 데이터셋 ID/설정 이행, 생산 배포의 의존 라이선스 검토 | macOS/Linux 구조 패키지는 검증했다. `sqlite-vec-go-bindings v0.1.6` 모듈 소스 인벤토리에 라이선스 파일이 없으므로 생산 배포의 라이선스 승인으로 간주하지 않는다. |
| A8 | 위 항목이 채워진 후 모든 수용 fixture와 마이그레이션·known limits 재실행 | 현재 통합 검사는 녹색이지만 선행 기능 게이트가 남아 있어 A8은 통과로 표시하지 않는다. |

품질/지연/정답률은 **unmeasured**다. A 전체 완료, B/C 완료, 운영 릴리스 적합성을 선언하지 않는다.

배포 의존성 확인: 현재 빌드가 사용하는 `github.com/asg017/sqlite-vec-go-bindings v0.1.6`의 모듈 소스에는 라이선스 파일이 없고 [Go Packages의 해당 버전](https://pkg.go.dev/github.com/asg017/sqlite-vec-go-bindings/cgo?tab=licenses)은 라이선스를 탐지하지 못한다. [상위 저장소의 현재 상태](https://github.com/asg017/sqlite-vec-go-bindings)는 MIT/Apache 파일을 보여 주지만, [누락 이슈](https://github.com/asg017/sqlite-vec-go-bindings/issues/8)가 열려 있다. 이 차이를 해소하거나, 라이선스 근거가 포함된 대체 의존성으로 전환하기 전까지 시험 서명 패키지를 운영 배포물로 선언하지 않는다.

후속 변경: `cks` 최상위 실패 출력은 한 줄의 JSON `{code,message}`와 비정상 종료로 정규화했다. 알려진 오류 접두사를 안정 코드로 매핑하고, 중첩 오류의 원문·비밀·임시 경로를 공개 메시지에 복사하지 않는다. `cmd/cks` 단위 테스트와 지식 잠금/패치 스모크가 통과했다. 이 변경은 MCP v2의 구조화 오류 데이터와 모든 구 소비자 재생을 완료했다는 뜻이 아니다.

추가로 `cks.context.get_for_task_v2`의 실패 결과는 `IsError=true`, `code=<고정 코드>` 텍스트와 동일한 `{code,message}` 구조화 데이터를 함께 반환한다. 원문 오류에 `requires_v2`, `reindex_required`, `snapshot_mismatch`, `source_missing`이 있으면 그 코드를 보존하되 경로·원문·비밀은 복사하지 않는다. 관련 MCP 단위 테스트가 통과했다. 전체 MCP 도구의 오류 정규화와 구 소비자 재생은 여전히 A7.1/A8의 남은 검증이다.

선택형 v2 `coding_context`를 추가해 검토된 정책을 required behavior, 검토된 ADR을 rationale로 분리했다. 검증된 구현 앵커가 없으므로 implemented behavior는 비우고 불확실 사유를 넣는다. 새 필드가 들어간 응답과 필드가 없는 이전 v2 응답을 각각 JSON 왕복 후 무결성 재검증했다. 외부 앵커가 생기기 전까지 이것은 완전한 코딩 경로가 아니다.

동시 승격 회귀: `Promote`, `Rollback`, 검토 승격의 `current` 교체에 같은 비차단 파일 잠금을 적용했다. 패치 승격은 그 잠금 안에서 `base_version`이 여전히 활성인지 재확인하며, 성공적인 교체 뒤에만 재사용 가능한 `review-release.json`을 쓴다. 잠금 경합·링크된 잠금 파일·오래된 기준 버전 시험을 추가했다. 검토 기록 작성의 동시성, 외부 수동 파일 변경, 마커 기록 실패 뒤 복구는 별도 게이트로 남는다.

기준별 검사 게이트 보강: 임의 명령의 종료 코드 0은 특정 테스트가 실행되었다는 증거가 아니다. `patch record-run`과 승인 단계는 이제 Go JSON 스트림에서 연결된 정확한 테스트 이름의 실행·통과가 관찰된 보고서만 받는다. 다른 언어의 exact runner가 추가되기 전까지 그 명령 결과는 일반 실행 증거로만 사용한다.

캡처 경계 보강: 저장소 하위 어느 위치의 `secrets`/`.aws` 디렉터리도 대소문자와 관계없이 민감 경로로 분류한다. 단일 파일 및 전체 캡처 용량 상한, 링크된 부모와 파일을 통한 루트 이탈 거부 시험을 추가했다. 보관 blob도 열기 전후 inode 일치와 no-follow 열기를 검사하며, 같은 해시의 외부 파일로 링크를 바꾼 경우에도 거부한다.

설치 상태 일치: `doctor`와 실제 캡처가 하나의 민감 경로 판정 함수를 사용한다. `.npmrc`, `.netrc`, `credentials.json`, SSH 키, 인증서와 중첩된 `Secrets`/`.AWS`를 같은 방식으로 표시·거부한다. 비Git fixture에서 상태 진단의 `secret_path_count`와 캡처 실패가 일치함을 검증했다.
