# 현재 인수인계

2026-10-14 통합 확인 기준은 main a214fbb18039f2e63ee93c0f073bfb1b61c3e716이다. [PR #102](https://github.com/0xmhha/knowledge-system/pull/102)는 head e77e59b8의 [CI5/5](https://github.com/0xmhha/knowledge-system/actions/runs/38565382940) 통과 후 스쿼시 머지됐다. 통합 시 로컬 main/원격 main·PR tree 동일성·clean worktree·기록된 source/evidence guard를 확인했다. 현재 checkout은 `git status --short --branch`로 확인하며 고정한 통합 commit을 미래의 실시간 HEAD로 해석하지 않는다.

## 현재 완료와 남은 범위

- 상위 WI-V1/V2/3/4/1/2/C/6/SP는 구현·명시 검증9/9 완료다.9개 운영·사람 독립 수용은 not_recorded이며 [전체 작업표](../REMAINING-WORK.md)에 별도 기록한다.
- WI-5는 담당 저장소 미확보다. 다른9개 구현 완료 수와 구분한다. 일반 후속13개는 기능별 범위 결정이 필요하다.
- go-stablenet 출처 매핑·42질문 재평가·SNK06 범위 검토3/3, 서버 종료 SD-01–04는4/4 완료다. 원래 SNK06 실패·과거 종료 오류는 보존한다. [비교/종료 작업표](./GO-STABLENET-DATASET-COMPARISON.md)를 따른다.
- coding-agent·ChainBench의 원본 commit/patch/bundle은 구현 packet에 보존한다. knowledge-system 머지로 다른 저장소 게시·설치·운영 상태를 완료 처리하지 않는다.

문서 정리는 [PR #103](https://github.com/0xmhha/knowledge-system/pull/103)에서 CI5/5 후 스쿼시 머지됐고 main ad52d956 동기화·기록된 source/evidence guard를 확인했다. [운영·독립 수용 계획](./OPERATIONAL-ACCEPTANCE-PLAN.md)은 로컬 Go pilot 대상·9개 항목의 입력/완료 gate·판정/증거 규칙을 정리한 draft다. [별도 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)의 실행/수용은0/9이며 단계0은 preparing이고 단계1–4는 미실행이다. [실행 protocol·동결 체크리스트](./OPERATIONAL-ACCEPTANCE-PROTOCOL.md)와 읽기 전용 source inventory를 준비했다. source-only 함수40개·review10개·확장 intent20개 후보, CLI metadata와4개 미해결 runtime recipe를 추가했다. [검토 자료](./OPERATIONAL-ACCEPTANCE-REVIEW-DRAFT.md)에10개 review rubric45anchor와3개 비완전 값/제어 초안을 추가했으며 독립 gold 채택은0이다. stress 예제 대신 실제 worker를 review 대상으로 제안했고 auto Stop 재진입도 게시 경계의 미해결 조건이다. 실제 결함/증상·rule/annotation 정답·seed 입력은 미준비다. local bugfix는 auto_merge=false여도 push/PR로 이어지므로 게시 차단 경계가 필요하며 ChainBench MCP의 SIGTERM exit0도 미확인이다. 다음은 담당·모델/한도 지정, 새 gold/입력 검토·runtime 경계 준비와 단계0 동결이다. 구현 상세는 [현재 명세](./UPPER-WORK-ITEMS-IMPLEMENTATION.md), evidence revision·SHA·owner commit은 [구현 원장](./UPPER-WORK-ITEMS-PROGRESS.json)에 있다.

## 먼저 읽을 문서

1. [남은 전체 작업](../REMAINING-WORK.md): N16 보류·태그/배포 보류와 별도 후속 범위.
2. [목적과 범위](./PURPOSE-AND-SCOPE.md): 초기 요구사항과 외부 WI10개 책임.
3. [현재 아키텍처](../ARCHITECTURE.md), [온톨로지 MCP](../../system/docs/ontology-mcp.md).
4. [실행 명세](./REFACTORING-EXECUTION-SPEC.md), [진행 원장](./REFACTORING-PROGRESS.json).
5. [평가 상태](./EVALUATION-STATUS.md), [출시 정책](./RELEASE-POLICY.md).

기존17/18·온톨로지4/4 완료는 기록된 수용 범위다. 독립 온톨로지 품질은inconclusive, 성능수용false, 온톨로지off·knowledge opt-in·타입 선언off를 유지한다. WBFT trace의 missing_code와 acceptance_evaluated=false를 테스트/기준 통과로 설명하지 않는다.

## 변경과 검증 원칙

원장 각 task의 revision에서 근거를 읽는다. 문서 정리에서 삭제된 준비·완료 보고서는 [복구 방법](./EVIDENCE-RECOVERY.md)을 따른다. 기존 증거 branch/commit과 원래 승인 입력·capture·gold·protocol·원문·raw 결과를 보존하며 다시 작성하지 않는다. 사용자 .claude/와 logs 자료는 접근하지 않는다.

현재 문서 목록과 역할은 [catalog](../DOCUMENTATION-CATALOG.json), 삭제 경로와 SHA는 [정리 목록](../DOCUMENTATION-CLEANUP.json)에 있다. CLI/도구/링크는 `make docs-check`, 완료 조건은 `make refactoring-check`로 확인한다. 시험 통과, source alignment, 검토된 관계, 사람 수용과 품질 판정은 분리한다.

다음 필수 작업은 N16 재개 여부 결정이다. 사용자가 보류를 유지하는 동안 키 생성·배포·선택 기능 기본 활성화를 수행하지 않는다. 새 후속 작업은 남은 목록의 별도 범위를 명시하고 승인·입력·완료 기준을 고정해 진행한다.
