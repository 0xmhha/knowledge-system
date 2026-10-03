# B0-H: Git 복구 이력 봉인·격리 게이트

상태: **구조 게이트 통과** (2026-10-03). 실모델 B0/B1 품질 평가는 미실행이다.

## 계약과 변경

신규 고정 Git 데이터셋은 기준 HEAD와 CKG가 선택한 최대 100개 복구 커밋을 `sources/history.bundle`에 보관한다. `snapshot_id` v4는 파일·HEAD·복구 SHA 집합과 bundle 정책을 포함한다. `sources/manifest.json`은 bundle SHA-256·바이트 수·복구 SHA를 기록한다. CKG/CKV의 빌드 루트는 bundle에서 만든 **독립 Git 저장소**이며 개발 저장소의 reflog·객체 저장소를 공유하지 않는다. 캡처 중 임시 bare 저장소가 원본 객체를 읽는 alternate는 bundle에 들어가지 않는다. 출력은 스트리밍으로 기본 1 GiB에서 제한하며 `cks setup --max-git-history-bytes`로 한도를 조정한다.

기존 `snapshot_id` v2/v3의 읽기와 원문 보관본 검증은 유지한다. 기존 버전의 이력 입력을 사후에 추측하거나 새 v4 데이터셋으로 조용히 변환하지 않는다.

## 수용 증거

| 항목 | 검증 결과 |
|---|---|
| 같은 HEAD/파일, 다른 복구 이력 | 새 복구 커밋 생성·되돌리기 후 `git_recovery_digest`, `snapshot_id`, `dataset_id`가 바뀌고 기존 후보의 승격 게이트가 거부했다. |
| 독립 객체 저장소 | 복원 트리의 Git alternates가 없고 복구 SHA 집합이 캡처와 일치했다. 원본 Git 저장소 삭제 뒤에도 보관본 검증과 재복원이 통과했다. |
| Git linked worktree 입력 | `.git` 파일로 공통 객체 저장소를 가리키는 worktree에서 캡처한 뒤 worktree·원본 저장소를 제거해도 기준 HEAD와 버려진 커밋을 복원했다. |
| 실제 CKG 재생 | 작은 Go 모듈의 삭제 전후 CKG `--no-cache --fail-on-parse-errors` 빌드에서 코드 `graph_digest`, AMBIGUOUS 복구 노드 ID, 총 8개 노드/11개 엣지가 일치했다. `graph_digest`만으로는 복구 노드 일치를 증명할 수 없다. |
| CKS/CKV 게시 | mock 임베더를 사용한 `cks setup --project-id b0h-fixture --version v1`이 graph/vector 후보를 게시하고 `current`를 전환했다. `dataset-identity.json`은 `bundle-selected-recovery-refs-v1`을 기록했고 fixture bundle은 745바이트였다. |
| 실패·구버전 | 1바이트 한도 초과에서 소스 매니페스트 미게시, bundle 삭제·변조에서 `source_missing`/`snapshot_mismatch`, 아카이브 없는 이전 v3 보관본 읽기가 통과했다. |

시험 위치: [`git_archive_repro_test.go`](../../cmd/graph/git_archive_repro_test.go), [`identity_test.go`](../../internal/setup/identity_test.go), [`capture_test.go`](../../internal/setup/capture_test.go). 전체 Go 테스트와 집중 재시험, `go vet`, 엔진 경계 검사, 문서 검사로 회귀를 확인했다. 신규 캡처 패키지는 Linux amd64/arm64에서 교차 컴파일했다. Linux 실행 및 큰 저장소 비용 측정은 아직 수행하지 않았다.

## 적용 범위와 남은 측정

이 계약은 `project_id`와 버전이 있는 **신규 고정 Git 빌드**에 적용된다. 기존 무고정/구 v2·v3 데이터셋의 역사적 빌드 결과를 다시 계산하지 않는다. 큰 저장소에서 bundle 크기·빌드 시간·디스크 상한의 적절성, macOS arm64와 Linux의 실제 대용량 재생 비용은 운영 자료가 쌓일 때 측정한다. B0 평가의 질문 정답 승인, 최종 Ollama 모델 고정, 검색 품질 수치는 별도 게이트다.
