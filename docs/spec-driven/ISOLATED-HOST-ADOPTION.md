# 소비자 후보 채택과 격리 host 연결 검증

2026-10-15. 사용자는 coding-agent·ChainBench 후보 채택과 격리 host 연결 검증을 요청했고, 후속 질문에서 로컬 연결 검증부터 진행하도록 선택했다. 이번 채택은 **owned local session** 범위다. 독립 검토, 상위 저장소 게시, 생산 설치나 전체 에이전트 흐름의 수용을 뜻하지 않는다. [manifest](../../system/eval/operational-preparation-2026-10-15/isolated-host/manifest.json)에 원자료를 기록했으며 [운영 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)의9개 운영·품질·독립 수용은 미실행이다.

## 채택한 정확한 버전

| 구성요소 | 이번 채택 | 검증 범위 |
|---|---|---|
| coding-agent | b7e78689676769e8536626e76eed7165d870e1ba | 원 후보 b6e2abdc에 SessionStart 보완; tracked367개 파일을 Git과 byte 대조 |
| ChainBench | bdf5ad2758662310b23c508e61b13ee318e92b40 | tracked2045개 파일 대조; 격리 재빌드한 MCP의 buildinfo가 같은 commit이며 vcs.modified=false |
| Claude Code |2.1.300 | 실제 CLI 경로·SHA 고정. 이전2.1.299의 사전 점검을 이번 실행 증거로 사용하지 않음 |

owned root는 `/private/tmp/ks-isolated-host-20261015`다. source clone, bin, adoption.json과 회차별 설정·상태를 분리했다. 원래 source/evidence checkout은 유지한다. 개인 인증·Claude 설정을 profile에 복사하지 않았으며 매번 새 CLAUDE_CONFIG_DIR를 사용하고 user/project/local setting sources를 제외했다. plugin은 --plugin-dir로 해당 세션에만 로드했다. strict MCP 설정은 명시적인 plugin_coding-agent_chainbench alias를 쓴다. host 내장 plugin은 init 원자료에 그대로 기록했다.

실제 서버의81개 tool schema는 이전 frozen regression과 일치한다. host permissions는 chainbench_chains만 노출하고 나머지80개 MCP 도구는 거부한다. Bash/Read/Write는 이번 스크립트형 로컬 fixture 검증에 사용했다. CKS·Jira·embedder·색인과 전체 역할 dispatch는 이번 연결 시험의 대상이 아니다. 이 설정을 머신 전체의 네트워크·파일시스템 격리 증명으로 해석하지 않는다.

## 실제 host에서 발견하고 보완한 문제

첫 SessionStart는 EVALUATED를 active로 표시하며 resume을 요청했다. Stop은 올바르게 종료했지만 이 안내는 이후 모델을 잘못 재개시킬 수 있었다. b7e7868은 이 상태를 로컬 평가 경계로 분리하고 --check-exit를 먼저 요구한다. pipeline 재개·게시를 금지하며 독립 수용으로 표현하지 않는다. hook29개와 기존 평가 policy15개, 도구명 계약·Python syntax·공백 검사가 통과했다.

첫 Write 시도는 Read를 먼저 하지 않아 host builtin이 먼저 거부했다. 이것을 policy guard 증거로 세지 않았다. 최종 시험은 Read 뒤 Write를 요청하여 publication-policy immutable hook deny를 직접 확인했다. 초기 builtin 거부 기록은 preliminary에 보존했다.

## 최종 실제 CLI 연결 시험5개

| 사례 | 실제 host·MCP·hook 결과 | 평가 판정 |
|---|---|---|
| valid | SessionStart는 경계 안내; Stop 입력false1회; CLI0·MCP0·receipt 검사0 | 로컬 평가 경계에서 종료 허용 |
| tools | 실제 metadata 조회 성공; local bare remote push와 policy 완화를 각각 명시적 hook deny로 거부; state 불변·remote 비어 있음 | 알려진 도구 경계 검증 |
| incomplete | Stop 입력false→true, 첫block 뒤 정지; CLI0·MCP0이지만 receipt 검사2 | 미완료 유지, CLI0만으로 통과시키지 않음 |
| tampered | receipt 이후 보고서를 바꿈; false→true, block 뒤 정지; receipt 검사2 | 변조된 평가는 실패 유지 |
| cancel | 로컬 API 요청 중 owned host에 SIGTERM; host143·MCP0 | 요청한 취소이며143을0으로 숨기지 않음 |

최종5개 세션의 로컬 API 요청은11회이며 실제 MCP tool call은 metadata1회다. 서버5개 모두exit0, 강제kill0이며 proxy/server의 owned PID10개는 종료 후 존재하지 않았다. 별도의 관찰 hook은 host 입력만 기록하며 plugin hook을 대체하지 않는다. 실제 PreToolUse·Stop 입력, hook 출력, MCP 요청/응답, 설정·exit·시간을 보존했다. [verifier](../../scripts/operational-host-probe.py)는 누락된 hook이나 CLI exit0만으로 성공시키지 않고 alias·제한 도구·receipt·거부 사유·server exit를 대조한다.

API는 로컬 스크립트 응답기가 제공하며 실제 provider 인증·모델 추론을 사용하지 않았다. CLI의 합성 usage/cost는 실제 청구나 모델 신원의 증거가 아니다. fixture의 stage 상태와 panel 의견도 실제 역할 수행이나 독립 정답이 아니다. Write에서 기존 사실 태그 guard가 추가로 거부한 기록도 보존했다. host 취소 사례는 활동 node/MCP handler cleanup 시험이 아니므로 WI-4를 대체하지 않는다.

초기 CLI5회와 host 시작 전 sandbox localhost bind 실패1회는 별도로 보존하며 최종5/5에 포함하지 않는다. bind 제한은 정상 실행 권한으로 처리했으며 hook·permission을 우회하지 않았다. 설정은 실제 CLI help와 [공식 환경 변수](https://code.claude.com/docs/en/env-vars), [hook 명세](https://code.claude.com/docs/en/hooks)를 참고했다.

## 재현·복구·회수

[probe](../../scripts/operational-host-probe.py)는 --root·--case·--attempt·--claude를 받는다. root에 위 source와 binary를 먼저 복구하고 attempt는 새 디렉터리 이름을 사용한다. 로컬 fixture용이며 실제 모델 pilot launcher가 아니다. --assess는 동결 기록만 읽는다. verifier 단위 시험8개가 hook 누락·잘못된 alias·CLI0 가짜 완료·Stop 반복·강제kill·bypass 등을 검증한다.

[증분 bundle](../../system/eval/operational-preparation-2026-10-15/isolated-host/coding-agent-context.bundle)은 b6e2abdc를 요구한다. [기존 복구 설명](./EVIDENCE-RECOVERY.md)과 [coding-agent 종료 후보](../../system/eval/operational-preparation-2026-10-15/coding-agent-exit/manifest.json)의 lineage로 부모를 복원한다. 새 clone에서 bundle을 fetch하여 b7e7868의 commit/tree와367개 파일을 검증했다. ChainBench는 [기존 bundle](../../system/eval/operational-preparation-2026-10-15/chainbench-stdio/manifest.json)을 유지한다. runner·source 파일 hash·원자료·검사 결과를 이번 manifest에 결속했다.

회수는 이 owned profile 사용을 멈추거나 별도 세션에서 기존 source를 로드하는 범위다. 개인 설정·다른 source checkout·생산 포인터는 변경하지 않는다. 외부push/PR·global 설치는 이번 채택에 포함하지 않는다.

## 현재 단계·다음 작업·남은 전체 범위

- 이번 범위: source 채택·로컬 실제 host 연결5/5 완료. 단계0은 preparing, 단계1–4는 미시작이다.
- 다음: reviewer 역할·실제 provider/model·비용/자원 상한, 새 case/gold·family·DEV/FINAL·통계/임계치를 검토하고 단계0을 동결한다. 동결 전 실제 pilot을 실행하지 않는다.
- 운영·독립 수용: WI-V1/V2/3/4/1/2/C/6/SP는0/9다.
- 그 외 남은 전체: WI-5·별도 후속13개·태그/공개 배포 결정은 [전체 작업표](../REMAINING-WORK.md)를 따른다.
