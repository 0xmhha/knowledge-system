# N15/N17 현행 패키지·외부 모듈·복구 기술 검증

2026-10-07 · **local 기술 세부 단계 통과 / 전체 N15·N16·N17 미완료**. [원자료 요약](../../system/eval/b0-knowledge-system/refactoring-n15-n17-native-controls-2026-10-07/summary.json)과 [manifest](../../system/eval/b0-knowledge-system/refactoring-n15-n17-native-controls-2026-10-07/technical-manifest.json)를 연결한다. 공개 소스 커밋 `b006e776b72be8f4a01b91966aebc149aaf84704`의 추적 파일만 clean checkout으로 복사해 Darwin ARM64/Go1.26.8 host-preview를 만들었다. 사용자 untracked `.claude/`, `logs/`는 제외했고 원 FINAL/승인 입력5개와 사용자 모델을 변경하지 않았다.

새 루트에 candidate 백업을 복원할 때 current 전환 뒤 MCP 시작이 실패하는 문제를 발견했다. candidate의 reader-protocol.json만 복사됐고 dataset 밖의 reader lease inode가 없어 조회를 시작할 수 없었다. mutation 잠금 아래 신원·review 권한 검사 뒤 reader lease를 생성/sync하도록 수정했다. 기존 inode와 candidate 원문·신원·marker를 보존하고, linked directory/inode와 다중 hardlink는 pointer 전환 전에 거부한다. legacy target의 protocol은 자동 추가하지 않는다. readonly 조회 경로의 보호는 완화하지 않았다.

## 검증한 기술 범위

| 항목 | 실제 확인 |
|---|---|
| 패키지 | clean commit b006e776, archive SHA256 `2315c29dfa4e0008746da1eecfc11bf3ef886b8684b5125dd17bb103f4a496dd`, 세 추출 바이너리·native dependency·modules/고지 SHA |
| 별도 프로젝트 | empty Go/TypeScript/미지원 Python 문서3개, mock 설치·재시작·데이터 갱신·rollback |
| 외부 Go 모듈 | example.invalid의 별도 go.mod + local replace, public pkg만 import; v1 MCP6응답·v2 MCP1응답 |
| 거부 제어 | legacy integrity/좌표·v2 좌표/raw integrity4개; 플랫폼 불일치도 output/dispatch 전에 거부 |
| reader 회귀 | 누락 디렉터리/파일2, linked/hardlinked3, live inode1, reviewed promotion/retry2, legacy1 =9제어; 관련3패키지 race/vet·경계 통과 |
| 복구 | 후보 gate/vector 실패 보존·손상 대상/활성 source 거부·실행 중 pin·갱신/rollback·새 루트 백업 복원·source 없는 재생 |
| 복구 원문 | v1/v2 capture10응답, candidate13파일의 원본/백업/복원 SHA 동일, 새 root의 reader lease 생성 |
| 고지 자료 | 현재 바이너리에 연결된35모듈·4vendored assets; 포함 범위의 누락 고지0; 적합성 판단 pending |

Go public consumer는 저장소 내부 테스트로 실행하지 않았고 consumer-module의 go.mod/build-info를 보존했다. raw v2 JSON integrity·Git 원문/줄·body SHA를 Python에서 독립 대조했다. 제한된 owned ASCII/integral fixture 감사이며 모든 JCS 입력이나 전체 v2 의미 계약 인증이라고 주장하지 않는다. negative control은 제품 품질 표본이 아니다.

첫 복구 실패는 before-recovery에 보존했고 restored capture는 initialize 실패/rows0 상태다. 변경 전 Go RED는 누락/unsafe lease 실패를 보존한다. 새 reviewed 시험의 source fixture 생성 누락으로 인한 중간 시험 실패도 보존했으며, fixture 수정 후 전체3패키지 race가 통과했다. 이 중간 panic은 제품 MCP panic으로 집계하지 않는다. 최종 바이너리를 clean commit에서 다시 만들고 패키지/외부 consumer/복구를 재검증했으므로 첫 패키지 성공을 수정 코드의 성공으로 소급하지 않는다.

패키지 archive/실행 바이너리와 mutable DB는 로컬 임시 경로에 유지한다. 저장소에는 hash·설치/소비자/복구 출력·보관 원문/신원·비바이너리 패키지 자산을 연결했다. 컴파일 시간·fixture 실행 시간을 실제 비용/p95·사람 검토 활동 시간·RTO/RPO로 사용하지 않는다.

## native Linux CI와 실제 미완료 조건

.github/workflows/ci.yml에 Ubuntu runner의 native-install-contract를 추가했다. host package→3mock installs→외부 public module/거부 제어→작은 backup restore/source-absent replay를 실행하고 mock 원자료를 보존한다. expect-os=linux/expect-arch=amd64를 확인하며 Docker/QEMU를 호출하지 않는다. **CI 정의만 존재하고 아직 dispatch/실행 증거는 없다.** 실행 runner의 실제 OS/arch/kernel/Go/패키지 신원 기록이 있어야 native 결과로 반영한다.

해당 저장소는 PUBLIC이며 현재 로컬 변경을 원격에 푸시하고 PR을 만들어야 새 PR-trigger CI를 실행할 수 있다. 이전329becf3의 구체 푸시 승인과 이번 새 전송 범위를 구분한다. native CI 결과가 나오더라도 실제 BGE/규모/외부 운영 소비자 수용, 운영 역할·공개키 신뢰/교체·고지 적합성, 실제 backup/failover 담당·RTO/RPO·network/전원 정책과 운영 훈련은 여전히 별도다. 시험 키/fixture 승인자를 운영 신뢰로 쓰지 않는다.

## 현재·다음·남은 전체

요청15개 중5완료·전체8/18, 남은 전체 N08/09/10/11/12/14/15/16/17/18 10개다. N15-A/B와 N17-A/B의 **위 local 기술 범위만** 체크하며 전체 N15/17 완료는 아니다. N16은 현재 패키지의 byte inventory만 갱신했고 실제 적합성·권한 판단은 없다.

다음은 기존 N11 DEV/프로토콜·N12 내용/관계 판정 반영과 새 독립 gold 준비, native Linux CI 실행에 필요한 구체 원격 전송 범위 확인이다. 이후 N08 검색/N09 기권/N10 비용·지연, N14 실제 의미 수용, N15 실환경/소비자, N16 권한/고지, N17 운영 복구, N18 출시 판정을 진행한다. 입력 없이 사람/운영 사실을 채워 완료율을 올리지 않는다.

## CI formatter의 동결 원문 보호

최종 CI 사전 확인에서 기존 Makefile의 전역 find가 과거 평가 원문3개에 gofmt를 요구했다. [5개 제어](../../system/eval/b0-knowledge-system/refactoring-format-scope-controls-2026-10-07/summary.json)로 이전 실패·새 범위의 통과·유지보수 코드 drift 거부·fmt 수정·수정 후 통과를 확인했다. cmd/internal/pkg/graph/vector/testdata/projects를 대상으로 제한하고 system/eval의 보관 원문은 수정하지 않았다. 해당3개 원문 SHA와 모든 runtime Go 소스는 불변이다. native 패키지는 위 b006e776에서 검증했고 이후 변경은 formatter 대상뿐이므로 바이너리/복구 결과를 새 Go 변경에 확장한 것이 아니다. make fmt-check와 증거 바인딩이 통과했지만 GitHub CI 실행 결과는 아직 없다.
