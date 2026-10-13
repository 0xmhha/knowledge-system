현재 진행 작업은 [go-stablenet 데이터 생성·비교](./GO-STABLENET-DATASET-COMPARISON.md)다. 일반 진행 보고에는 사용자가 별도로 보류한 출시 사항을 포함하지 않고, 요청한 데이터 작업의 진행·검증·다음 단계만 설명한다.

# 현재 인수인계

2026-10-13 기준 제품 코드는 main9a397188이다. [PR #99](https://github.com/0xmhha/knowledge-system/pull/99)는 스쿼시 머지됐고 로컬 main 동기화와 [main CI5/5](https://github.com/0xmhha/knowledge-system/actions/runs/38362113044)를 확인했다. 현재 checkout/원격 동기화는 `git status --short --branch`로 확인한다. 이 문서에 고정한 검증 commit을 미래의 실시간 HEAD로 사용하지 않는다.

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
