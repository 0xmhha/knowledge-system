현재 추가 요청: 서버 종료 보완4/4 완료.378호출 EOF 종료에서 임시 원문 트리 삭제 지연을 재현했고 검증된 archive 직접 읽기로 제거했다. 실측2세션756팩·질의/종료 오류0, Close11.1/10.9ms, EOF·대기/실행 SIGTERM 모두 exit0·강제 종료없음·reader lease 해제. 현재/커밋 SHA1839개와 기존 진행 guard PASS. workspace `bin/cks`도 실측 바이너리로 갱신했다. 이 요청의 남은 작업0개·다음 작업없음. [작업표](./GO-STABLENET-DATASET-COMPARISON.md), [완료 근거](../../system/eval/stablenet-shutdown-2026-10-14/completion.json).

현재 진행 작업은 [상위 WI 구현 명세](./UPPER-WORK-ITEMS-IMPLEMENTATION.md)의 요청9개 순차 구현이다. 구현 검증8/9: WI-V1/V2/3/4/1/2/C/6 완료·독립 전체 수용 미판정. WI-1 source337300b0·증거6332da9d의 [최종 packet](../../system/eval/upper-work-items-implementation-2026-10-14/wi-1/README.md)74개 산출물/1617개 source·dependency SHA PASS, clean binary15개 실제 Go index→namespaced MCP 검증/31요청/EOF0. `pkg/graph/valueflow.Query`와 CKG `value_dependencies`가 계산을 공유하며 명시된 package/memory/effect/unknown 한계는 유지한다. WI-2 source2e1e7791·증거3f0bb039의 [최종 packet](../../system/eval/upper-work-items-implementation-2026-10-14/wi-2/README.md)은 CKG/CKS39개 실제 검증/23사례·기존15개 회귀/전체 build/test/fmt/lint PASS와 오탐2/미탐4·한계를 기록한다. WI-C primary3a12951/coding-agent45d9bc8b의 [최종 packet](../../system/eval/upper-work-items-implementation-2026-10-14/wi-c/README.md)은16개 actual artifact/hook/delivery·142개 plugin/전체 primary/rule39/value15 회귀 PASS와 구조적 snapshot 미핀·부분 리뷰/독립 수용 미판정을 기록한다. WI-6 primary3b0c7521/consumer4e65eefc의 [최종 packet](../../system/eval/upper-work-items-implementation-2026-10-14/wi-6/README.md)은 CKS32 계약·12 actual 검사·전체 회귀·단일 corpus 관련함수1/4→4/4+noise1/5, source/예산/관계 구분과 한계를 기록한다. 다음이자 남은 전체 요청 구현은 WI-SP다. 설계 고정 상태이며 typed 공개 기능·거부/권한/출처 검증은 아직 미완료다. 원장은 [진행 JSON](./UPPER-WORK-ITEMS-PROGRESS.json)이다. 개인 .claude/와 개인 logs는 입력으로 읽지 않는다. push/PR/merge는 이 구현 요청에 자동 포함하지 않는다. 아래 데이터 작업은 완료된 선행 기록이다.

[go-stablenet 비교 작업표](./GO-STABLENET-DATASET-COMPARISON.md)의 출처 매핑 수정·42질문 재평가·SNK06 검토는3/3 실행 완료다. 수정 데이터의756회 질의 오류0, 정상 pack 원문 검증 통과, 기존 SNK06 실패와 캡처 종료 오류2회 보존. 현재/커밋 SHA1833개와 기존 진행 guard PASS. 남은 요청 작업0개이며 근거는 진행 JSON을 따른다.

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
