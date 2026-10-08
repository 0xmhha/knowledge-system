# 보관 소스·혼합 pinned 좌표 SDK 개발 진단

2026-10-05, FIX-19. [원자료 요약](../../system/eval/b0-knowledge-system/retained-guard-preparation-m2max-2026-10-05/summary.json)과 [해시 원장](../../system/eval/b0-knowledge-system/retained-guard-preparation-m2max-2026-10-05/evidence-manifest.json)은 실제 BGE-M3 DEV 진단이다. 공식 입력/FINAL·품질·사람 verdict는 null이고 기존 승인 입력 5개는 시작 HEAD와 같다.

## 확인된 문제와 수정

`InspectVersionIdentity`가 보관 blob 변조/삭제를 구분해도 v2 MCP 사전검사는 모든 오류를 reindex_required로 덮었다. 서로 다른 상태/프로젝트의 engine identity 불일치도 구체적인 코드를 주지 않았다. 수정 전 단위 시험의 네 subtest와 기존 바이너리의 실제 SDK 음성 제어 네 건이 실패했다.

FIX-19는 알려진 사전검사 코드를 기존 제한된 `{code,message}` 응답으로 전달하고 engine의 프로젝트/스냅샷/데이터셋/manifest identity 불일치를 snapshot_mismatch로 구분한다. legacy/nil identity와 미분류 오류는 기존 reindex_required를 유지한다. 원문 오류·경로·비밀 값을 공개 응답에 넣지 않는다. backend health가 나빠도 source/identity 검사가 먼저 이뤄지는 계약을 유지했다.

## 실제 SDK 제어와 복원

동결된 F-04-DEV-old/new와 F-05-DEV-a/b는 이전 [격리 진단](./B0-STATE-ISOLATION-REPORT-PREPARATION.md)의 strict BGE-M3 데이터셋을 사용한다. 테스트는 원본을 읽기만 하고 독립 임시 사본을 만든다. 정상 pinned SDK 연결을 초기화한 후 사본의 전체 vector 디렉터리를 다른 상태 또는 프로젝트의 vector로 교체한다. 이는 기존의 서로 떨어진 graph/vector 설정 거부와 다른 제어다.

| 제어 | 수정 전 SDK | 수정 후 SDK | backend 호출 |
|---|---|---|---:|
| F-04 같은 프로젝트의 다른 상태 vector | reindex_required | snapshot_mismatch | 0 |
| F-05 다른 프로젝트 vector | reindex_required | snapshot_mismatch | 0 |
| 보관 blob 변조 | reindex_required | snapshot_mismatch | 0 |
| 보관 blobs 디렉터리 제거 | reindex_required | source_missing | 0 |

수정 전후 각각 계획/관측 10회·누락 0회다. 각 실행의 정상/복원 6응답·20인용·20본문은 공개 Go Verify, outer sha256-v2, 자기 원문 줄/전체 파일/본문 SHA·프로젝트/스냅샷/데이터셋 좌표를 통과했다. 복원 전후와 바이너리 수정 전후의 정상 인용·본문·좌표 집합은 같다. 원본 두 쌍과 최종 사본의 전체 payload 해시가 그대로다. 실제 backend 로그의 요청 measurement ID를 SDK 호출과 연결했고 정상 조회는 raw K10/별도 K6, 사전 거부는 호출 0회를 확인했다. 설정/바이너리 전후 SHA와 BGE-M3 digest·metadata도 같다. 지연은 진단 원자료이며 공식 warm/cold 표본으로 합산하지 않는다.

SDK helper의 최초 public 필드명 오류와 SQLite 빈 WAL/SHM을 payload 변경으로 오판한 로그도 보존했다. 수정한 사본 생성기는 빈 WAL과 그에 대응하는 SHM만 생략하고, nonempty WAL이면 별도로 checkpoint된 DEV 입력을 요구하며 실패한다. 원본 seal에는 두 파일도 포함한다. 해시 검사를 느슨하게 하여 실패를 숨기지 않았다.

## 검증과 남은 범위

setup/MCP/evidence-v2/eval/CLI 계약 시험, setup/MCP/evidence-v2 race, 관련 vet, 실제 SDK 수정 후 시험과 독립 Python 원문/측정 로그 재생이 통과했다. opt-in `TestRetainedGuardLiveSDK`는 명시적 DEV descriptor·기존 바이너리·새 출력 파일을 요구하며 일반 CI에서는 실행 조건을 안내하고 skip한다. archived Go 원문은 `.go.txt`로 보관한다.

B0-06/B1-05의 개발 음성 제어 준비를 보강했지만 공식 F-04/F-05 완료로 합산하지 않는다. B0-01/02/03/05·STV2-01 결정과 입력 동결, 새 strict B0-07, 공식 기준선/paired/사람 판정·C0·독립 C1이 남는다. 최신 최종 패키지는 FIX-19 이후 동결 후보로 다시 만들어야 하며 OP-01–08·실제 native amd64·전체 native/legal·대규모 비용·최종 운영 복구/출시 결정도 필요하다.
