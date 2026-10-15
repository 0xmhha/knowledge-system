# 현재 인수인계

최신 진행: [정책 overlay 적용 검증](./POLICY-OVERLAY-ADOPTION.md)에서 별도 source0bf2f4d1 profile로 실제 graph125,089node/176,822edge·정책42/연결146·CKS24도구호출을 확인했다. 수정7건의 source/canonical parity와 음성3개를 통과했고 기존 원본/운영 current는 보존했다. 일반 domain sync는 여전히 기존 정책을 재생성하므로 생성기 수정은 미완료다. OE-01은 질문12개/4개 arm 입력 준비 중이며 기존 retained dataset에 semantic DB가 없어 별도 정합 후보를 준비하는 것이 다음이다. 효과 미실행·새 기능 완료0/7·기존 독립 수용0/9를 유지한다.


2026-10-15 최신 우선 범위: [지식 매니저·비교 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)와 [전용 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json). 온톨로지 효과 평가 OE-01을 우선 필수 검증으로 채택했다. KM-01 프로젝트 격리, KM-02 버전 DB 운영 도구, KM-03 Claude 매니저 플러그인, KM-04 주기 갱신, KM-05 공통 개념 매핑, KM-06 비교 근거 MCP를 요구로 등록했다.7개/하위28개의 완료는0이며 OE-01 입력 준비를 시작했다. 나머지는 미착수이고 설계 초안이다. 기존 WI-5는 KM-03/04로 구체화하여 중복 집계하지 않는다. 기존 선택 후속13개 중 ontology 평가는 OE-01로 이동했으므로 선택 후보는12개다.

로컬 개발·도구/회귀 검증은 정식 독립 수용의 단계0 동결·담당자·유료 모델 지정 없이 계속한다. 실제 모델 실행/독립 수용에 해당 조건을 적용한다. 다음은 정책 연결7건 보완·OE-01 단일 프로젝트 평가, 확장 경로는 KM-01→02→03→04와 KM-05→06 후 다중 프로젝트 평가다. 원래9개 구현/독립 수용 원장은 별도로 유지한다.


2026-10-14 통합 확인 기준은 main a214fbb18039f2e63ee93c0f073bfb1b61c3e716이다. [PR #102](https://github.com/0xmhha/knowledge-system/pull/102)는 head e77e59b8의 [CI5/5](https://github.com/0xmhha/knowledge-system/actions/runs/38565382940) 통과 후 스쿼시 머지됐다. 통합 시 로컬 main/원격 main·PR tree 동일성·clean worktree·기록된 source/evidence guard를 확인했다. 현재 checkout은 `git status --short --branch`로 확인하며 고정한 통합 commit을 미래의 실시간 HEAD로 해석하지 않는다.

## 현재 완료와 남은 범위

- 상위 WI-V1/V2/3/4/1/2/C/6/SP는 구현·명시 검증9/9 완료다.9개 운영·사람 독립 수용은 not_recorded이며 [전체 작업표](../REMAINING-WORK.md)에 별도 기록한다.
- WI-5는 담당 저장소 미확보다. 다른9개 구현 완료 수와 구분한다. 일반 후속13개는 기능별 범위 결정이 필요하다.
- go-stablenet 출처 매핑·42질문 재평가·SNK06 범위 검토3/3, 서버 종료 SD-01–04는4/4 완료다. 원래 SNK06 실패·과거 종료 오류는 보존한다. [비교/종료 작업표](./GO-STABLENET-DATASET-COMPARISON.md)를 따른다.
- coding-agent·ChainBench의 원본 commit/patch/bundle은 구현 packet에 보존한다. knowledge-system 머지로 다른 저장소 게시·설치·운영 상태를 완료 처리하지 않는다.

문서 정리는 [PR #103](https://github.com/0xmhha/knowledge-system/pull/103)에서 CI5/5 후 스쿼시 머지됐고 main ad52d956 동기화·기록된 source/evidence guard를 확인했다. [운영·독립 수용 계획](./OPERATIONAL-ACCEPTANCE-PLAN.md)은 로컬 Go pilot 대상·9개 항목의 입력/완료 gate·판정/증거 규칙을 정리한 draft다. [별도 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)의 실행/수용은0/9이며 단계0은 preparing이고 단계1–4는 미실행이다. [실행 protocol·동결 체크리스트](./OPERATIONAL-ACCEPTANCE-PROTOCOL.md)와 읽기 전용 source inventory를 준비했다. source-only 함수40개·review10개·확장 intent20개 후보, CLI metadata와4개 미해결 runtime recipe를 추가했다. [검토 자료](./OPERATIONAL-ACCEPTANCE-REVIEW-DRAFT.md)에10개 review rubric45anchor와3개 비완전 값/제어 초안을 추가했으며 독립 gold 채택은0이다. stress 예제 대신 실제 worker를 review 대상으로 제안했고 auto Stop 재진입·게시 전 종료는 OP-CA-01 후보 b6e2abdc에서 개발 검증했으며 로컬 실제 host 연결은 [격리 채택 기록](./ISOLATED-HOST-ADOPTION.md)의5개 사례로 검증했다. 실제 모델·전체 역할·노드 수용은 미실행이다. 실제 결함/증상·rule/annotation 정답·seed 입력은 미준비다. local bugfix는 auto_merge=false여도 push/PR로 이어지므로 기본 게시 경로와 별도로 evaluation_only 정책이 필요하며 [ChainBench 종료/상태 격리 보완](./OPERATIONAL-ACCEPTANCE-READINESS-FIXES.md)은 새 후보 bdf5ad27에서 build·전체test·fmt/vet·lint·stdio 회귀시험을 통과했다. source/patch/bundle은 primary에 보존했고 외부 게시·설치·실제 노드 cleanup은 미확인이다. 두 소비자의 로컬 채택 source는 coding-agent b7e7868·ChainBench bdf5ad27이며 실제 CLI2.1.300을 로컬 응답기에 연결했다. 다음은 담당·모델/한도 지정, 새 gold/입력 검토·실제 모델/노드 준비와 단계0 동결이다. 구현 상세는 [현재 명세](./UPPER-WORK-ITEMS-IMPLEMENTATION.md), evidence revision·SHA·owner commit은 [구현 원장](./UPPER-WORK-ITEMS-PROGRESS.json)에 있다.

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

## 정책·보안 입력 준비 추가

후속 [정책 참조 연결 진단](./POLICY-REFERENCE-DIAGNOSIS.md)은 실제 CKG AST 프런트엔드와 policy resolver에서 기존6개 경고 및 경고 없는 잘못된 함수 연결1개를 확인했다. 수정7개 사본에서는146개 연결·경고0개다. 원본 source-bound YAML/기존 packet은 그대로이며 새 overlay 채택·실제 index/MCP·독립 의미 검토는 남는다. 이 진단의 source pin·raw 출력·도구·실패 이력은 별도 manifest에 결속한다. 단계0 preparing·운영 수용0/9를 유지한다.

[검토 자료](./POLICY-SECURITY-INPUT-REVIEW.md)에 기존 project 정책42개/참조146개와 source-only query12개, 두 지원 규칙 관련 호출68개를 기록했다. source 이름 힌트140개·미연결6개는 실제 graph 판정이 아니다. 두 command 인자 힌트는 build 지원 코드이고 conversion-error 힌트는0이다. WI-2의 새 독립 양성·음성 기준을 충족하지 못하므로 actual detector/품질을 통과시키지 않는다. 수동 security metadata4개도 취약점/자동 rule 정답이 아니다. public/private를 분리했고 과거 질문에 나온 splitAndTrim은 제외했다. reviewer·모델/비용 지정과 독립 gold/family/split 채택은 미결이며 단계0 preparing·운영 수용0/9를 유지한다.
