# PR #99 CI 도구 체인 검증 — 2026-10-13

현재 단계: **수정·로컬 검사 완료, 전체 race와 GitHub CI 검증 중**. 이 기록은 기존 개발 수용 판정과 별개의 저장소 전달 작업이다. 기존17/18·온톨로지4/4·N16 보류를 유지한다.

## 목적과 실패 원인

PR의 필수 빌드 작업은 `go.mod`의 Go1.25.13으로 실행됐으나, 고정된 golangci-lint v2.14.0은 Go1.26 이상을 요구했다. 빌드와 vet는 통과했지만 린터 시작에서 중단되어 race 검사에 도달하지 못했다. 최초 공개 실행과 이후 실행의 실패를 성공으로 바꾸어 기록하지 않는다.

취약점 검사의 Go1.26.8은 표준 라이브러리 11개 항목을 보고했다. 공식 [릴리스 기록](https://go.dev/doc/devel/release)의 Go1.26.9 수정과 스캐너의 고정 버전을 대조했다.

## 수정 범위와 설계

CI의 build-and-test, eval-gate-graph, vuln-scan만 Go1.26.9로 고정한다. 벡터 평가와 Linux native 설치 계약은 계속 `go.mod`의 Go1.25.13을 사용하여 최소 버전 검증을 남긴다. go.mod/go.sum과 기본 기능 설정은 변경하지 않는다. `continue-on-error`인 권고용 취약점 검사도 그대로 유지한다.

새 도구 체인으로 드러난 errcheck 두 항목을 수정했다. 응답 비교의 읽기 전용 입력 파일은 디코딩 오류를 그대로 반환하고, 닫기는 명시적으로 정리한다. MCP 준비 상태 시험에서는 t.Cleanup으로 읽기 전용 저장소를 닫으며 닫기 실패도 시험 실패로 기록한다. 린터 규칙은 완화하지 않는다.

로컬 재현은 `GOTOOLCHAIN=go1.26.9 make lint`와 `GOTOOLCHAIN=go1.26.9 make test-race`로 가능하다. 시스템 기본 도구 체인 설정은 변경하지 않았다.

## 완료 조건과 검증

- [x] CI YAML에 세 도구 작업의 Go1.26.9 고정을 확인하고 최소 버전 두 작업을 유지한다.
- [x] Go1.26.9 전체 build와 lint 통과: gofmt/vet/린터0오류/경계/문서315개.
- [x] 도메인 산출물 일치 검사 통과.
- [x] 수정한 semanticcli와 system/mcp 패키지의 기존 race 시험 통과.
- [x] Go1.26.9 취약점 검사와 verbose 검토 통과: 호출 취약점0·import한 패키지 취약점0.
- [ ] 전체 Go race 검사의 종료 코드와 최종 결과를 확인한다.
- [ ] 수정 소스와 결과의 SHA를 실제 커밋에 연결하고 진행기록을 갱신한다.
- [ ] 수정 커밋에서 GitHub CI 다섯 작업의 성공을 확인한다.

## 취약점 검사에서 남는 한계

verbose 결과의 미호출 모듈 항목은 golang.org/x/mod v0.38.0의 [GO-2026-6180](https://pkg.go.dev/vuln/GO-2026-6180)과 [GO-2026-6179](https://pkg.go.dev/vuln/GO-2026-6179)다. sumdb 및 sumdb/tlog의 체크섬 검증에 관한 항목이며 모듈 수정 버전은 v0.40.0이다. 이번 스캔은 해당 패키지 import와 취약 함수 호출을 발견하지 않았다. 두 공식 항목에서 cmd/go 수정 버전은 Go1.25.13·Go1.26.6이므로 CI의 Go1.25.13/1.26.9는 그 도구 자체의 영향 버전에 포함되지 않는다.

이번 범위에서는 의존 모듈을 일괄 올리지 않고 고정 상태와 미호출 결과를 기록한다. 이는 일반적인 무취약 보증이나 C/native 의존성의 판정이 아니다. 향후 sumdb 패키지 도입 또는 모듈 변경 시 재검토한다. 이전 모델·독립 품질·지연·운영 수용 판정은 변경하지 않는다.

## 다음과 남은 전체 작업

다음은 수정 브랜치의 실제 GitHub CI 검증이다. 그 뒤 PR 리뷰가 남으며, main merge는 별도 지시가 필요하다. 기존 개발 잔여는 N16 공개키 fingerprint/신뢰 조건 미충족으로 보류된1개다. 태그·공개 배포도 보류한다.

[검증 원자료](../../system/eval/b0-knowledge-system/ci-toolchain-fix-2026-10-13/local-verification.json), [현재 인수인계](./CURRENT-HANDOFF.md), [작업리스트](./REFACTORING-REMAINING-WORKLIST.md).

### Linux CI 추가 수정 — 2026-10-13

5935fa96의 GitHub CI는4/5 통과했으나 Linux 전용 memory_linux.go에서 읽기 전용 파일 정리의 errcheck1개가 실패했다. macOS 린트 범위에 없는 파일이다. `/proc/meminfo` 닫기를 명시적으로 정리하도록 수정했으며, 최종 Linux CI 통과 전에는 전체 CI 완료로 표시하지 않는다. [실패·수정 기록](../../system/eval/b0-knowledge-system/ci-linux-cleanup-fix-2026-10-13/verification.json)을 보존한다.
