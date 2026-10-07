# N15 native Linux AMD64 CI 검증과 공개 전송 기록

2026-10-07 · [원자료/검증 receipt](../../system/eval/b0-knowledge-system/refactoring-n15-native-linux-ci-2026-10-07/verification.json) · [PR #97](https://github.com/0xmhha/knowledge-system/pull/97)

사용자의 “푸시하고 Pr생성해” 지시에 따라 d08cb384의80커밋을 feat/spec-driven-knowledge-system에 전송하고 main 대상 draft PR97을 생성했다. 최초 자동 승인 검토의 범위 미승인 거부는 이전 이력이며 현재 전송 대기 사유가 아니다. CI 수정 d7de0b89는 공개d08 부모에서 .github/workflows/ci.yml만 변경하여 추가 전송했다. 이후 로컬 BGE/후보 출처 receipt4커밋과 이 새 기록은 전송하지 않았다. 사용자 미추적 .claude/·logs/도 포함하지 않았다. PR은 draft, merge/출시 판정은 없다.

## 실행 전 문제와 수정 후 같은 검증

[최초 실행37584211796](https://github.com/0xmhha/knowledge-system/actions/runs/37584211796)의 build/lint/race/domain·vector·native 계약은 통과했다. graph self-indexing이 보관 Go1.26 source module을 Go1.25.13으로 읽다가 실패했고, advisory vuln job은 현재 govulncheck의 Go1.26 요구로 도구 실행 전에 실패했다. 이는 품질 gate 실패나 발견된 호출 취약점 판정이 아니다. 원래 실패 기록을 보존한다.

수정은 graph/vuln 두 job만 Go1.26.8에 pin한다. build/vector/native job은 go.mod의 Go1.25.13을 유지한다. go.mod·제품 runtime·모델·임계치·baseline·보관 source에는 변경이 없다. 깨끗한 공개 source checkout의 local graph eval/gate 및 vuln exit0을 확인했다. local vuln은 호출된 취약점0이며 호출되지 않은 module advisory2개를 남긴다. 운영 native dependency의 적합성 판정은 아니다.

[재실행37586641833](https://github.com/0xmhha/knowledge-system/actions/runs/37586641833)은 **5/5 job success**다: build-and-test(빌드/lint/race/domain), eval-gate-graph, eval-gate-vector, vuln-scan, native-install-contract. 기존 합성 graph/vector 회귀 gate 통과이며 새 독립 N08/N11 평가 결과로 세지 않는다.

## native 실행과 원자료 더블 체크

GitHub Actions ubuntu-latest의 실제 linux/amd64, kernel6.17.0-1022-azure, Go1.25.13을 job과 package/consumer metadata에서 대조했다. ARM 위 에뮬레이션이 아니다. 재실행 package commit67e15083은 PR merge snapshot이며 Git tree가 공개head d7de0b89와 같다. clean checkout, cks/ckg/ckv binary SHA를 package·consumer·recovery 세 기록에서 대조했다.

| 검증 | 최초 실행 | 재실행 |
|---|---|---|
| Native job | success | success |
| Artifact ID | 11465262939 | 11466767201 |
| ZIP SHA256 | d79bd3134c0d801833b9ad64fe608bafd6ef94901abb14c4432cac1877c9cf40 | 00844fc1d8d74c109d3789b52f4553433ada06ce41829b81186c42b86d12b1a8 |
| 압축 내부/다운로드 SHA 대조 | 178파일 일치 | 178파일 일치 |
| 소유 mock 프로젝트 설치/재시작/데이터 갱신/rollback | 3개 | 3개 |
| 별도 public Go module 응답/거부 제어 | 7/4 | 7/4 |
| 갱신/rollback/새 root복구/원본 source 제거 뒤 재생 | 10응답 | 10응답 |
| 원본 전후·backup·restored payload SHA | 13파일 모두 동일 | 13파일 모두 동일 |

server artifact digest와 내려받은 ZIP SHA, 모든 extracted byte SHA를 검증했다. 경로 탈출 항목은 없다. 보관 package archive/실행 바이너리를 다시 내려받아 실행한 시험은 아니며, archive SHA와 binaries SHA는 CI package/consumer/recovery 실행 기록 간 대조다. 손상 입력과 실패 후보/current 보존, 실행 중 MCP baseline pin, original source 없이 보관 citation 재생을 확인했다. v2 raw integrity는 소유 ASCII/integral fixture 범위이며 일반 JCS 인증으로 확장하지 않는다.

## 완료 범위와 다음 작업

**N15-C만 완료 체크**한다. N15-D/E의 실제 모델·규모·독립 운영 소비자 binary upgrade 및 지원 수용은 미완료다. consumer 기록의 native_linux_amd64_accepted=false는 전체 운영 지원 판정을 뜻하며 위 기술 실행 통과와 구별한다. N17-A/B의 native 보강 증거이며 실제 backup/failover 담당·RTO/RPO/키·전원·네트워크 운영 drill은 N17-C/D/E에 남는다. real_model_calls0, new_final_calls0, 사람 수용0, releasefalse다.

현재 전체8/18·요청5/15 완료, 잔여10개 N08/N09/N10/N11/N12/N14/N15/N16/N17/N18. 다음은 N11 DEV/gold/protocol·독립 FINAL 범위/오염 판정과 N12 실제 내용/관계 검토를 받는 것이다. 다음 실행은 실제 응답과 source/runtime binding에 따라 진행하며, 이전 preview 승인을 새 독립 gold/운영 승인으로 사용하지 않는다.
