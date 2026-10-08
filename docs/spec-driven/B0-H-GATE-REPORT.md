# B0-H: Git 복구 이력 봉인·격리 게이트

상태: **구조·mock 파일럿 게이트 통과** (2026-10-03). 실모델 B0/B1 품질 평가는 미실행이다.

## 계약과 변경

신규 고정 Git 데이터셋은 기준 HEAD와 CKG가 선택한 최대 100개 복구 커밋을 `sources/history.bundle`에 보관한다. 최초 `snapshot_id` v4는 파일·HEAD·복구 SHA 집합과 bundle 정책을 포함했다. 파일럿에서 실행 비트 손실이 발견되어 신규 v5는 `file_mode_policy=executable-bit-v1`과 파일별 실행 비트를 파일 매니페스트·스냅샷 신원에 추가한다. 커밋 빌드는 Git checkout의 원래 모드를 유지하고 작업 트리 빌드는 캡처한 모드를 재현한다. `sources/manifest.json`은 bundle SHA-256·바이트 수·복구 SHA를 기록한다. CKG/CKV의 빌드 루트는 bundle에서 만든 **독립 Git 저장소**이며 개발 저장소의 reflog·객체 저장소를 공유하지 않는다. 캡처 중 임시 bare 저장소가 원본 객체를 읽는 alternate는 bundle에 들어가지 않는다. 출력은 스트리밍으로 기본 1 GiB에서 제한하며 `cks setup --max-git-history-bytes`로 한도를 조정한다.

기존 `snapshot_id` v2/v3/v4의 읽기와 원문 보관본 검증은 유지한다. 기존 버전의 이력·파일 모드를 사후에 추측하거나 새 v5 데이터셋으로 조용히 변환하지 않는다.

## 수용 증거

| 항목 | 검증 결과 |
|---|---|
| 같은 HEAD/파일, 다른 복구 이력 | 새 복구 커밋 생성·되돌리기 후 `git_recovery_digest`, `snapshot_id`, `dataset_id`가 바뀌고 기존 후보의 승격 게이트가 거부했다. |
| 독립 객체 저장소 | 복원 트리의 Git alternates가 없고 복구 SHA 집합이 캡처와 일치했다. 원본 Git 저장소 삭제 뒤에도 보관본 검증과 재복원이 통과했다. |
| Git linked worktree 입력 | `.git` 파일로 공통 객체 저장소를 가리키는 worktree에서 캡처한 뒤 worktree·원본 저장소를 제거해도 기준 HEAD와 버려진 커밋을 복원했다. |
| 실제 CKG 재생 | 작은 Go 모듈의 삭제 전후 CKG `--no-cache --fail-on-parse-errors` 빌드에서 코드 `graph_digest`, AMBIGUOUS 복구 노드 ID, 총 8개 노드/11개 엣지가 일치했다. `graph_digest`만으로는 복구 노드 일치를 증명할 수 없다. |
| CKS/CKV 게시 | mock 임베더를 사용한 `cks setup --project-id b0h-fixture --version v1`이 graph/vector 후보를 게시하고 `current`를 전환했다. `dataset-identity.json`은 `bundle-selected-recovery-refs-v1`을 기록했고 fixture bundle은 745바이트였다. |
| 실행 비트와 구버전 | 실행 파일이 있는 커밋의 독립 복원 트리는 Git status가 깨끗하고 승격 조건을 통과했다. 작업 트리에서 바이트가 같아도 실행 비트 변경은 다른 v5 스냅샷이며, 복원·원본 변경 감지가 통과했다. 이전 v3/v4 보관본 읽기도 통과했다. |
| 실패 | 1바이트 한도 초과에서 소스 매니페스트 미게시, bundle 삭제·변조에서 `source_missing`/`snapshot_mismatch`를 확인했다. |
| Linux 실행 | 최종 v5 코드를 arm64와 amd64 컨테이너에서 실행해 아카이브 한도·변조, linked worktree 복원, 동일 HEAD 이력 변경, 실행 비트 보존·변경 시험을 통과했다. 두 대상의 최종 v5 시험 키 서명 preview 패키지도 사전 서명 검증과 세 프로젝트 설치 스모크를 통과했다. amd64는 macOS arm64 호스트에서 에뮬레이션했다. |

시험 위치: [`git_archive_repro_test.go`](../../cmd/graph/git_archive_repro_test.go), [`identity_test.go`](../../internal/setup/identity_test.go), [`capture_test.go`](../../internal/setup/capture_test.go). 전체 `go test ./...`, `go vet ./...`, 형식·엔진 경계·문서 검사가 통과했다. 실행 비트 매니페스트 변조 거부 시험은 집중 재시험으로 확인했다.

## 고정 파일럿 운영 측정

고정 코퍼스 `71cb71cd55960833e930269e272f7a4a060be3aa`의 **독립 Git 저장소**에서 mock 임베더로 `cks setup --version b0h-mock`을 실행했다. 첫 v4 시도는 실행 파일 30개의 실행 비트를 잃어 승격 게이트가 거부했고 `current`를 만들지 않았다. v5 수정 후 재빌드는 CKG 102,112노드/414,613엣지(`graph_digest=fa58e1e7…`), CKV 1,569파일/13,469청크를 만들고 `current → b0h-mock`으로 전환했다. `cks doctor`는 `ready`, `pinned`, `history_status=available`, `dirty=false`를 보고했다. 소스 보관본은 1,937파일, Git bundle은 36,550,909바이트, 전체 데이터셋은 약 336 MiB다. 기본 1 GiB bundle 상한은 이 한 코퍼스에서 충분했다. macOS arm64 호스트의 재빌드 계측은 약 107초다. 계측용 `/usr/bin/time -l`은 샌드박스의 `sysctl` 거부로 셸 종료 코드 1을 반환했지만 CKS 게시와 `doctor`는 성공했다.

측정값과 신원은 [원자료 요약 JSON](../../system/eval/b0-knowledge-system/git-archive-ops-2026-10-03.json)에 기록했다.

## 적용 범위와 남은 측정

이 계약은 `project_id`와 버전이 있는 **신규 고정 빌드**에 적용된다. 기존 무고정/구 v2–v4 데이터셋의 역사적 빌드 결과를 다시 계산하지 않는다. 한 파일럿의 크기와 시간은 기본 상한의 보편적 적절성을 입증하지 않는다. Linux의 대용량 재생 비용과 실제 Ollama 검색 품질은 아직 측정하지 않았다. B0 평가의 질문 정답 승인과 최종 모델 고정은 별도 게이트다.
