# 운영 검증 준비를 위한 소비자 보완

2026-10-15. 이 작업은 격리된 소비자 source 후보를 수정한 **개발 회귀검증**이다. [운영 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)의 실제 모델/역할 pilot·독립 평가·사람 수용은 미실행이며 protocol은 미동결이다. 원래9개 구현의 승인 source/원장/증거는 유지한다.

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

coding-agent 게시 전 종료·Stop 처리는 아래 OP-CA-01 후보에서 개발 검증을 완료했다. 두 소비자의 로컬 채택·실제 host 연결은 아래 OP-HOST-01에 기록했다. 다음은 새 입력/정답 검토·모델/자원 지정과 단계0 동결이다. 이어 새 독립 gold·case/family/DEV/FINAL, reviewer 역할·LLM/비용·metric/자원 기준을 고정하고 actual host/node 검증을 수행한다. stage0은 preparing, stage1–4는 미시작, 운영/독립 수용은0/9다. WI-5·별도 후속13개·태그/공개 배포 결정은 [전체 작업표](../REMAINING-WORK.md)에 남긴다.

## OP-CA-01 — 게시 전 평가 종료와 auto Stop 재진입

후보 `b6e2abdc8892bb5e82044997f4f2285ece9c5b35`을 `/private/tmp/ks-op-coding-agent-evaluation-only`의 별도 branch에서 구현했다.
기존 baseline fdb4d515와 승인된 구현 증거는 변경하지 않았다. 이 보완 당시 source 채택·설치/게시·실제 host 검증은 pending이었다. 이후 로컬 채택은 OP-HOST-01을 따른다.

| 세부 기능·목적 | 개발 검증 상태 |
|---|---|
| local/full/bugfix auto 전용 `--evaluation-only`; auto-merge/미지원 유형 조합 거부 | 완료 |
| intake 정책·ticket/repo/workspace/base 신원 lock; 사후 완화 차단 | 완료 |
| 기존 oracle completion·applied selection·완료 stage/plan/report·clean tracked source 검사 재사용 | 완료 |
| 검증된 EVALUATED와 state/source/artifact/runner hash receipt; PR/COMPLETION 미기록 | 완료 |
| 정상/반복 Stop 종료, 미완료·변조 시 평가/진단 요구, stop_hook_active 단일 재진입 해제 | 완료 |
| 오래된 policy/다른 auto_merge 티켓에 의한 경계 해제·게시 재진입 방지 | 완료 |
| 알려진 push/PR/Jira 등 도구 차단·Write/Edit 정책/가짜 PR 차단 | 완료, 아래 제한 적용 |
| 회귀·실제 CLI/hook subprocess 원자료·patch/bundle·복구 | 완료, manifest 참조 |

새 경계 시험 15개, 전체 plugin script 시험 157개,
bench 시험 15개, 기존 hook26개, Python syntax·hook JSON·shell syntax·공백·C1 도구명 계약을 통과했다.
통제 smoke는 실제 Python 결함·두 source 후보·dual-oracle/control/mutant를 실행하고 정상 종료/반복 Stop/게시 거부/변조 거부의 원요청·응답을 보존한다.
fixture panel 의견은 독립 검토가 아니다. 첫 정책 시험의 실패1건과 기존 hook fixture의 실패1건은 원자료에 남기며 최종 pass로 세지 않는다.
기존 hook fixture는 synthetic Git commit 실패를 무시하던 문제를 수정했다. 서명 비활성은 일회용 fixture 명령에만 적용하며 후보 source commit의 GPG 설정은 유지한다.

EVALUATED는 로컬 평가 gate 종료이며 게시·독립 품질·사람 수용이 아니다. 필요한 ChainBench/regression skip은 기존 tier 규칙을 따른다.
종료 뒤 host의 Agent PostToolUse 기록이 이어지므로 `logs/impl.log`와 `logs/agent-transcript.jsonl`은 append telemetry로 보존하되 exit gate hash 대상에서 제외한다.
그 외 report/source/state 변경은 receipt를 무효화한다. receipt hash는 검토자 신원 인증이나 각 역할의 실제 host dispatch 증명이 아니다.

publication guard는 알려진 Bash 표기와 MCP 도구 범위의 유한 검사다. 임의 script/alias/encoded 명령·직접 shell 파일 수정·host bypass의 전체 외부 전송을 막는 sandbox가 아니다.
이번 시험의 model 호출·설치 host session·외부 게시 호출은 모두0이다. 실제 host hook 연결/권한·제한된 MCP/네트워크·pilot 실행은 단계0/1에서 검증한다.
전용 격리 clone에서만 사용하고 게시가 필요한 다른 작업과 섞지 않는다. 기존 publish/default 경로는 유지한다.

[manifest](../../system/eval/operational-preparation-2026-10-15/coding-agent-exit/manifest.json)에 source blob/SHA·검사·실패 로그·통제 smoke·복구를 결속했다.
[patch](../../system/eval/operational-preparation-2026-10-15/coding-agent-exit/change.patch)와
[bundle](../../system/eval/operational-preparation-2026-10-15/coding-agent-exit/coding-agent.bundle)을 primary에 보존한다.
bundle은 baseline fdb4d515가 필요하며 기존 [coding-agent 복구 lineage](./EVIDENCE-RECOVERY.md)를 유지한다.
운영/독립 수용은 여전히0/9다.

## OP-HOST-01 — 로컬 source 채택과 실제 host 연결

[채택·검증 명세](./ISOLATED-HOST-ADOPTION.md)에 coding-agent b7e7868·ChainBench bdf5ad27의 local session 채택과 실제 CLI2.1.300 연결을 기록했다. b7e7868은 실제 SessionStart에서 발견한 EVALUATED 재개 안내를 수정한다. hook29개·policy15개·host verdict 단위8개와 최종 CLI5개 사례를 검증했다. Stop의 false→true 입력, MCP alias·metadata 호출, 명시적인 push/정책 완화 deny, receipt 실패 유지, host143/서버0 취소와 owned PID10개 정리를 확인했다.

이 결과는 로컬 스크립트 API·통제 fixture의 연결 증거다. provider 모델·실제 agent 역할/CKS·활동 노드·독립 gold 수용은 미실행이다. 원래 개발 시험과 초기 문제 기록을 보존하고 로컬 연결을9개 운영 수용으로 올리지 않는다. 새 source/runner/원자료·복구 bundle과104개 artifact는 [manifest](../../system/eval/operational-preparation-2026-10-15/isolated-host/manifest.json)를 따른다.
