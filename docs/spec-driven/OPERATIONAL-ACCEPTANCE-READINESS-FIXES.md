# 운영 검증 준비를 위한 소비자 보완

2026-10-15. 이 작업은 격리된 소비자 source 후보를 수정한 **개발 회귀검증**이다. [운영 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)의 실제 host·독립 평가·사람 수용은 미실행이며 protocol은 미동결이다. 원래9개 구현의 승인 source/원장/증거는 유지한다.

## OP-CB-01 — ChainBench stdio 종료와 상태 경로 격리

| 항목 | 확인한 내용 |
|---|---|
| 문제 재현 | baseline df464d4b의 initialized idle stdio 서버에 SIGTERM을 보내면 exit -15 |
| 후보 구현 | bdf5ad2758662310b23c508e61b13ee318e92b40, owned checkout /private/tmp/ks-op-chainbench-shutdown |
| request 처리 | 순서대로 처리하면서 별도 input reader가 EOF를 감지; signal/EOF를 cooperative handler context에 전달 |
| stdio 정리 | Unix에서는 owned descriptor를 복제해 runtime poller에 연결하고 취소 시 읽기/쓰기를 해제; 기존 file-status flags는 복원 |
| 오류 구분 | canceled MCP tool은 isError 응답이며 server exit0와 구분. frame limit·broken output 등 별도 오류는 실패로 유지 |
| 상태 격리 | 명시적인 절대 경로 CHAINBENCH_DATA_ROOT 지원; 기본은 기존 사용자 상태 경로. 테스트에서 HOME을 바꾸거나 개인 상태를 사용하지 않음 |
| 게시/설치 | local commit만 기록. 외부 push·PR·설치·운영 포인터 변경은 미수행 |

첫 수정은 os.Pipe 기반 단위 테스트를 통과했으나 실제 subprocess9개는 종료가 멈춰 테스트 cleanup의 강제 kill이 필요했다. 기본 os.Stdin/Stdout wrapper는 pipe 기반 unit fixture와 cancellation 동작이 달랐다. 이 실패를 보존한 뒤 pollable descriptor로 연결하여 재검증했다. 실패한9회를 graceful 성공으로 세지 않는다.

전체 테스트 첫 실행은 sandbox의 localhost bind·개인 상태 lock 접근 제한과 생성된 아키텍처 수치 불일치로 실패했다. 명시적 상태 root·새 owned 시험 디렉터리와 localhost fixture 권한으로 다시 실행했고, HOME을 가정하던 기존3개 실패도 app의 상태 root에 맞게 수정했다. 기본 위치는 별도의 기본 경로 테스트로 유지하고 CLI/MCP pool·workspace의 일치 검증을 보존했다. 생성 package-tree 문서도 실제 source line 수로 갱신했다.

## 최종 검증 결과와 판정 범위

| 검증 | 결과 | 범위 |
|---|---|---|
| 전체 binary build | pass | commit bdf5ad27, buildinfo의 vcs.modified=false |
| 전체 Go test | pass |66개 ok package,13개 no-test package; live/E2E 입력 제외 |
| fmt·전체 vet | pass | 후보 checkout 전체 |
| pinned lint | pass,0 issues | golangci-lint v2.12.2 |
| target race test | pass | stdio lifecycle/오류/flags 복원, actual signal subprocess9개 |
| SIGTERM subprocess | idle3·cooperative active3·blocked output3 모두 exit0 | 최종 강제 kill0; active는 fixture handler의 cleanup 확인 |
| actual binary metadata | EOF3·idle SIGTERM3·SIGINT1 모두 exit0 | initialize/tools/list만 수행, schema81개 baseline과 일치 |
| 복구 | pass | baseline clone에 incremental bundle을 fetch해 동일 commit/tree 복원 |

실제 chain node의 장기 작업·lease/temp cleanup이나 설치된 host/plugin dispatch는 아직 검증하지 않았다. 취소를 무시하는 handler를 강제로 preempt하는 보장도 아니다. 검증 머신은 Darwin arm64이며 다른 platform의 종료 보장으로 확대하지 않는다. EOF는 active request를 취소하므로 응답의 cancellation/error와 완료된 tool 작업을 반드시 구분한다. 위 유한 회귀검증을 운영·독립 품질·사람 수용으로 올리지 않는다.

## 원자료·source와 복구

[manifest](../../system/eval/operational-preparation-2026-10-15/chainbench-stdio/manifest.json)에15개 changed source의 Git blob/SHA, source tree·binary SHA/buildinfo, 명령·원요청/응답·exit/시간·실패한 시도·최종 검사·복구 결과를 기록했다. [patch](../../system/eval/operational-preparation-2026-10-15/chainbench-stdio/change.patch)와 [incremental bundle](../../system/eval/operational-preparation-2026-10-15/chainbench-stdio/chainbench.bundle)은 primary repository에 보존한다. bundle은 baseline df464d4b가 필요하며 기존 WI-4 source recovery lineage를 따른다. [기존 복구 설명](./EVIDENCE-RECOVERY.md)과 [baseline packet](../../system/eval/upper-work-items-implementation-2026-10-14/wi-4-mode9/sources/chainbench.bundle)을 유지한다.

candidate source·artifact hash가 확인된 것은 prepared source 후보의 근거다. publication·installation·독립 수용은 not_recorded이며 source 채택과 실제 runtime의 신원·설정 재확인은 단계0/1에서 별도로 수행한다.

## 다음과 남은 범위

다음 source 준비 작업은 coding-agent의 게시 전 종료 정책과 auto Stop 재진입을 함께 처리하는 것이다. 이어 새 독립 gold·case/family/DEV/FINAL, reviewer 역할·LLM/비용·metric/자원 기준을 고정하고 actual host/node 검증을 수행한다. stage0은 preparing, stage1–4는 미시작, 운영/독립 수용은0/9다. WI-5·별도 후속13개·태그/공개 배포 결정은 [전체 작업표](../REMAINING-WORK.md)에 남긴다.
