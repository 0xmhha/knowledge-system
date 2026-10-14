# 상위 WI 10개 재감사 명세와 작업표

감사 기준일: 2026-10-14. 현재 단계: 재감사와 문서·증거 검증 완료. **감사 항목10/10을 검토했으며, 기능 전체 완료 판정은0/10이다.** WI-5는 담당 저장소 미확보로 구현·실행 상태를 판정하지 않았다. 나머지9개도 원래 목적 전체의 독립 실행·수용 증거가 부족하다.

## 목적과 완료 조건

이번 요청은 [남은 작업 문서](../REMAINING-WORK.md)의 ‘상위 WI10개 담당 코드·실행·수용 재감사’를 수행한다. 외부 기능 10개를 새로 구현하거나 운영 체인을 변경하는 범위로 확대하지 않는다.

2026-06-26 원 마스터 계획의 목적은 버그수정 워크플로를 먼저 완결하고, 이후 깊은 코드리뷰를 지원하는 것이다. GREEN만으로 원인 수정·오라클 타당성·후보 선택을 완료 판정하지 않는다. 그래프 계산은 CKG, 검색·사실 신호 합성은 CKS, 워크플로 판단은 coding-agent에 둔다. 학습 수집은 별도 ingest MCP와 관리자 검토 경로를 따른다.

각 WI의 감사 완료 조건은 다음과 같다. 대상 미확보도 감사 결과에 남기되 기능 미구현으로 단정하지 않는다.

1. 원 목적·담당·수용 조건을 확인한다.
2. 실제 코드·공개 도구·워크플로 연결을 확인한다.
3. 안전한 기존 시험을 실행하고, 실행하지 않은 부분과 그 이유를 기록한다.
4. 구현 상태, 실행 상태, 사람의 수용 상태를 분리한다.
5. 근거·부족분·다음 작업·우선순위를 기록한다.

외부 저장소는 읽기와 격리된 기존 모의 시험만 수행했다. 외부 코드·운영 설정·노드·학습 데이터는 변경하지 않았다. 개인 `.claude/`와 `logs/`는 감사 입력에서 제외했다. Buddy는 관련 저장소로만 확인했으며 coding-agent 또는 학습 ingest의 대체물로 판정하지 않았다.

## 입력과 증거

원 입력은 `/Users/kevin/work/github/0xmhha/auto-coding/seminar/`의 `12-master-workplan.md`, `07-cks-codingagent-improvements.md`, `11-symptom-suspect-labeling-contract.md`, `10-learning-direction.md`다. [증거 패킷](../../system/eval/upper-work-items-audit-2026-10-14/)에 원문 사본·SHA256·시험 원자료·MCP 응답을 보존했다.

| 담당 | 감사한 revision | 범위·최신성 |
|---|---|---|
| knowledge-system | `220925c89e5f88cbb32fc7f97410d57ee3df958d` | 제품 소스 변경 없음. 원격 main 일치 확인 |
| coding-agent | `3a66d91f6486077edc0f09d00c5549e40dd55081` | 별도 임시 sparse checkout, 원격 main 일치 확인 |
| ChainBench | `662fff77382eb031df687d371a8bc8f63cad581f` | 외부 작업 트리 청결, 원격 main 일치 확인 |
| Buddy, 관련 후보 | `dbeed22b6fef7ae0f18b2a9df60c5343a4592d03` | 동일 담당이라고 추정하지 않음 |
| 학습 ingest·관리 플러그인 | 미확보 | 저장소 지정 필요. 대화 검색 프로젝트를 대신 완료 처리하지 않음 |

지식 시스템 관련 소스404개, coding-agent32개, ChainBench10개의 내용 hash를 [source-inventory.json](../../system/eval/upper-work-items-audit-2026-10-14/source-inventory.json)에 고정했다. 모든 외부 저장소 전체를 시험했다는 의미가 아니다. 원격 최신성은 감사 시점의 관찰이며 미래 자동 동기화 약속이 아니다.

## 감사 작업표와 전체 잔여 기능

‘감사 완료’는 근거와 부족분을 기록했다는 뜻이다. 아래 기능 완료는 별도 구현·실행·수용 기준을 충족해야 한다.

| 순서 | ID | 감사 | 기능 상태 | 남은 핵심 작업 |
|---|---|---|---|---|
| 1 | WI-V1 | 완료 | 부분 구현·실행 미수용 | 독립 오라클 검증, false-done 반례, 강제 차단·경고 정책과 실제 에이전트 실행 |
| 2 | WI-V2 | 완료 | 원인 후보 판별은 있음; 수정안 비교 미확보 | 같은 증상에 대한 복수 수정안·평가축·선택/기각·judge-panel 계약 |
| 3 | WI-3 | 완료 | 탐색·개별 신호·원인 라벨 있음 | 소비자 계약 정합, 제한된 후보 합성·신호별 provenance·null·반복 확장 |
| 4 | WI-4 | 완료 | 장애 주입·모니터·재현 절차 있음 | 반복 분모·실패율·타이밍 주입·재현 불가 관찰의 연결 실행 |
| 5 | WI-1 | 완료 | AST·호출·필드 관계 있음; CFG/PDG 미확보 | 언어·비용·정확도 계약, def-use·제어/값 의존 구현·반례 |
| 6 | WI-2 | 완료 | 수동 YAML 태그 있음 | 결정론 타입/규칙·source→sink 탐지, 룰별 양성/음성 사례 |
| 7 | WI-C | 완료 | 기본 리뷰 보고서·함수 근거 있음 | 시퀀스·모듈·값/상태·가드·성능·우선순위의 구조화 산출물과 수용 |
| 8 | WI-6 | 완료 | bounded 호출 hop·curated flow 있음 | 값 의존 확장은 WI-1 이후, 독립 recall/noise/cost 비교 |
| 9 | WI-SP | 완료 | CKG 일반 도구로 간접 노출 확인 | CKS 전용/합성 질의·출처/권한·typed 태그·없는 태그·소비자 연결 |
| 별도 | WI-5 | 완료: 대상 미확보 기록 | 판정 불가 | 담당 저장소 확보 후 명령 수집→파일→관리자 검토→승격/회수→색인 루프 검증 |

전체 기능 완료는10개 모두 보류한다. 확인한 소스·시험만으로 전체 기능 완료를 선언하지 않는다. WI-5 대상 확보는 다른 항목의 실행을 막지 않는 별도 의존성이다.

## WI별 근거와 설계 부족분

### WI-V1 — 오라클 타당성

coding-agent [plugin/skills/reproduce-first/SKILL.md](https://github.com/0xmhha/coding-agent/blob/3a66d91f6486077edc0f09d00c5549e40dd55081/plugin/skills/reproduce-first/SKILL.md), [plugin/agents/evaluator.md](https://github.com/0xmhha/coding-agent/blob/3a66d91f6486077edc0f09d00c5549e40dd55081/plugin/agents/evaluator.md) §4.7/4.8/7.5c, [docs/adr/ADR-0003-reproduction-and-fix-validity.md](https://github.com/0xmhha/coding-agent/blob/3a66d91f6486077edc0f09d00c5549e40dd55081/docs/adr/ADR-0003-reproduction-and-fix-validity.md)를 확인했다. 동일 증상·RED/GREEN·오라클 변경 금지, 원인 수정과 재현 성공의 분리, sibling 경로·derived-state 검사가 존재한다. 6월의 ‘절차 없음’으로 유지하면 현재 코드와 다르다.

그러나 §4.7에서 이전 코드가 이미 GREEN인 경우는 WARN이고, §4.8 과적합·unit oracle fidelity도 WARN 후 통과 허용이다. e2e RED는 기본적으로 작성 당시 기록을 신뢰하며 엄격 재확인은 선택적이다. 독립 오라클 재유도·민감도·실행 코드 mutation을 모든 경우에 강제하는 계약과 현재 실제 에이전트 실행 증거는 확보하지 않았다.

`bench/p0-mutants`는 계약 객체의 합성 변형을 검사하는 결정론 reference harness다. hard 변형12/12 검출, clean 오탐0/2지만, under-declaration 잔여2/2는 검출하지 못했다. **실제 코드 mutation·독립 오라클 검증 성공률로 바꾸지 않는다.** 다음 구현은 무효 오라클·원인 누락·기각/보류 반례와 독립 기준을 먼저 동결해야 한다.

### WI-V2 — 수정 후보 선택

analyzer §4/§6의 경쟁 원인 A/B discriminator와 runtime probe, planner §3.5의 수정 대상별 impact 조회는 존재한다. 이는 원인 후보 판별·변경 영향 조회다. 같은 증상을 해결하는 복수 **수정안**의 생성·비교·judge-panel 선택 계약과 실행 산출물은 확인하지 못했다.

최소 계약에는 후보별 원인·수정 범위·oracle 결과·회귀·비용·트레이드오프·선택/기각 이유, 패널 의견 불일치·동률·전부 기각 시 동작이 필요하다. 현재 단일 설계의 리뷰를 이 목표 완료로 처리하지 않는다.

### WI-3 — 증상에서 제한된 후보와 객관 라벨

CKS `internal/system/mcp/graph.go`, `analysis.go`, `flow.go`, `search.go`, `pkg/system/contract/{hit,pack,history}.go`는 검색 rank/source/score, 관계·거리, 영향 범주·동시성·변경 이력의 개별 근거를 제공한다. stage3의 후보 cap·거리 감쇠·출처 문자열과 coding-agent의 `related-code.json`/`affected_sites`·candidate/confirmed/refuted 원인 라벨도 확인했다.

원 계약의 모든 객관 신호를 한 후보 schema로 합성하여, 신호별 source/revision·값 미확보의 null·LLM 증상 판단을 분리하고 제한된 후보를 반복 확장하는 전체 경로는 완료 근거가 없다. 특히 변경 이력 존재와 ‘최근성’의 공통 수치 계약은 다르며 수동 태그·값 의존 없는 신호를 0으로 채워서는 안 된다.

소비자·서버 계약은 먼저 대조해야 한다. coding-agent의 `kinds`는 배열이지만 실제 CKS `find_symbol`과 `semantic_search`는 CSV 문자열을 받는다. CKS fixture의 일부 입력 설명도 실제 등록 schema와 다르며 현재 일반 name-only 시험이 이 차이를 검출하지 않는다. get_for_task의 max_citations/budget_tokens도 소비자 계약에는 있으나 실제 입력 schema에는 없고, 서버 fixture의 get_subgraph는 qname을 선언하지만 실제 입력은 symbol이다. 도구명의 underscore/dot은 호스트 정규화 여부를 실측하지 않아 연결 오류로 단정하지 않았다. 실제 schema와 비교 결과는 증거 패킷에 보존했다. 새 통합 도구 구현 전 소비자 입력·오류·호스트 정규화 계약부터 맞춘다.

### WI-4 — 비결정·타이밍 재현과 관찰

ChainBench [internal/testhelper/fault.go](https://github.com/0xmhha/chainbench/blob/662fff77382eb031df687d371a8bc8f63cad581f/internal/testhelper/fault.go)의 노드 stop/start/restart·partition/heal, 기대한 실패의 bounded 확인, [internal/nodemonitor/gate.go](https://github.com/0xmhha/chainbench/blob/662fff77382eb031df687d371a8bc8f63cad581f/internal/nodemonitor/gate.go)의 관찰·제한된 restart·fatal/budget 처리가 존재한다. 선별된 모의 시험47개가 통과했다. coding-agent의 simulation/e2e 재현, 원인 미확정 유지, bounded block-production 모니터와 focused race 검사도 존재한다.

이번 시험은 실제 체인을 실행하지 않았다. 반복 횟수·성공/실패 분모·주입 seed·환경·증상 실패율·개선 판단 기준, 미재현 상태에서 관찰 후 재개하는 전체 루프의 수용 증거는 미확보다. 고정된 횟수의 단순 반복을 비결정 현상의 충분한 통계 검증으로 바꾸지 않는다. 연결 계약과 장애 주입 전/후 실제 반복 사례가 다음 작업이다.

### WI-1 — CFG/PDG와 값 흐름

CKG `pkg/graph/types/enums.go`, `internal/graph/parse/golang/{parser,resolve,field_access}.go`와 생산 파서 경로를 확인했다. AST 문장·호출·field reads/writes·동시성·분산 관계는 존재하지만, CFG basic block/control successor, SSA/def-use, control/data dependence를 위한 제품 schema·계산 경로는 확보하지 못했다. 로컬 변수 `cfg`(packages.Config)나 curated flow를 CFG 구현 근거로 사용하지 않는다.

Go 우선인지 다른 언어도 포함하는지, on-demand/full 계산, 포인터·별칭·콜백·외부 호출의 불확실성, interprocedural 범위·캐시·비용 한계, 정확도 반례를 먼저 고정한다. 현재 호출 그래프 시험 성공은 값 흐름 정확도 시험을 대신하지 않는다.

### WI-2 — 결정론 규칙과 취약점 탐지

`pkg/graph/policy/policy.go`, `pkg/graph/security/security.go`, `internal/graph/buildpipe`의 loader는 수동 YAML의 governs/matches를 resolve한다. 입력 ID·중복·severity·없는 symbol의 기존 시험은 통과했다. 자동 source→sink 흐름 탐지·taint propagation·언어별 결정론 룰 엔진은 확인하지 못했다.

수동 annotation의 `EXTRACTED`는 행 추출 신뢰도이며 취약점이 기계적으로 검증되었다는 뜻이 아니다. 다음 설계는 수동/자동 생성 구분, rule ID/version·source/sink·경로/불확실성·provenance, 양성/음성·오탐/미탐·비용 기준을 포함해야 한다. 위협 평가나 보안 성공 판정으로 확대하지 않는다.

### WI-C — 깊은 코드리뷰 산출물

coding-agent analyzer §7은 light review, planner §7은 위치·설명·권고·severity·최대30개 findings와 code-quality 요약을 생성한다. CKS `internal/system/mcp/review.go`는 함수의 code/calls/dynamic/history 섹션과 지원 한계를 제공한다. 도구와 기본 리뷰 경로는 존재한다.

원 계획의 sequence/module view, 실제 값·상태 변화, guard/exception 경로, 연산·메모리·I/O 비용, 우선순위/트레이드오프를 모두 필수 구조로 요구하는 깊은 리뷰 산출물·독립 검토는 확보하지 못했다. 실제 CKS는28도구를 등록하지만 coding-agent 계약은19개 CKS 도구이며 review_function과 온톨로지 읽기 도구가 포함되지 않는다. 새 도구를 모두 강제 사용해야 한다는 뜻은 아니지만 이 리뷰 경로의 연결·수용 증거는 없다. 단순 리뷰 명령은 기존 PR 코멘트 수집·반영을 뜻하므로 깊은 리뷰 생성과 혼동하지 않는다.

### WI-6 — 다단 및 값 의존 확장

`internal/system/composer/stage3/{intent_relations,expander,scoring,relation_weights}.go`는 intent별1/2 hop, 기본 seed10·neighbor50 제한, relation 가중치·거리 감쇠·MAX 합성·출처를 갖는다. 직접 graph traversal도 bounded depth/response를 사용한다. curated `get_flow`/`expand_flow`는 작성된 producer/consumer 관계를 탐색한다. 관련 기존 시험은 통과했다.

자동 코드 값 의존을 따라 확장하는 기능은 WI-1 의존성이 남아 있다. multi-hop 동작 검증과 독립 회수 품질·잡음·총비용 수용은 별도다. 다음 설계는 호출과 값 의존 edge를 구분하고, 순환·alias·불확실·상한·반복 확장 중단 조건을 포함해야 한다.

### WI-SP — 정책·보안 MCP 노출

이번 실제 CKG stdio probe에서 tools/list10개, 수동 Policy/SecurityPattern의 정확 조회와 `get_subgraph`의 `governed_by`/`has_security_pattern` 반환을 확인했다. 없는 ID는 빈 nodes였고 EOF 종료0을 확인했다. 기존 ‘MCP 미노출’ 설명은 **간접 노출 있음**으로 정정한다.

반면 합성 fixture의 `search_text(query=Audit, node_kinds=[Policy,SecurityPattern])`는 빈 결과였다. 이 입력의 결과를 그대로 보존하며 검색 가능성을 일반화하지 않는다. 일반 노드 반환은 signature에 category/severity를 포함하지만 typed description/remediation·검토 출처·권한 계약 전체가 아니다. CKS `ckgclient.relationFromEdgeType`는 두 annotation edge를 지원하지 않으며 일반 subgraph 변환은 이를 제외한다. 전용 CKS 질의도 확인하지 못했다.

다음 구현은 반환/거부 schema, 수동/자동 tag 구분, source·reviewer·version·권한·unknown/not-found, CKS 노출/소비자 계약과 음성 사례를 고정한다. 온톨로지 읽기4도구나 v2 policy 근거를 이 별도 annotation 목표의 완료로 합산하지 않는다.

### WI-5 — 수집·관리자 학습

개정 방향 `10-learning-direction.md`는 사용자 명령 수집→별도 ingest MCP 파일 저장→별도 관리자 플러그인의 검토·store→CKS 소비를 요구한다. 사람 게이트·sanitize·dedup·검증된 항목만 활성·회수는 유지한다. coding-agent의 이전 자동 capture hook은 revert되었고 현재 `continuous-learning-loop.md`도 DESIGN ONLY다.

담당 ingest/관리 플러그인 저장소는 미확보다. 확인한 `claude-knowledge-vault` main `da1d9602`의 README/PLAN은 개인 대화 이력 검색 프로젝트이며 이 업무로 대체하지 않았다. 개인 이력은 읽거나 색인하지 않았다. Buddy의 관련 기억 저장과 CKL(LSP)도 같은 목적의 완료 근거가 아니다.

이 항목의 다음 의존성은 정확한 저장소 지정이다. 미확보를 ‘기능 없음’ 또는 ‘완료’로 처리하지 않는다. 이후 ingest schema·경로·sanitization·dedup·승격/회수·색인 소비의 실제 수집→검토→활성/회수 사례를 감사한다.

## 실행 결과와 한계

| 실행 | 결과 | 증명 범위 |
|---|---|---|
| 지식 시스템6패키지 기존 시험 | pass349, skip3, fail0 | policy/security loader·MCP handler/등록·bounded expansion의 기존 단위/모의 회귀 |
| ChainBench3패키지 선별 시험 | pass47, fail0 | fake node control/RPC·monitor gate 등. 실제 네트워크 시험 아님 |
| coding-agent 결정론 시험 | pass17 | 계약 객체 변형 검사10 + 검색 실패 정책7. 실제 에이전트 실행 아님 |
| coding-agent 이름 검사 | 121참조·51도구 PASS | 자체 계약 이름 일치. 서버 입력·호스트 호환을 증명하지 않음 |
| P0 reference measurement | hard12/12, clean 오탐0/2, 잔여 검출0/2 | 객체 계약 변형의 작은 합성 표본 |
| P2 reference measurement | 10시나리오, silent6→0, over-block0, 불일치0 | 합성 검색 실패 정책, 실제 MCP 장애를 에이전트에 주입한 측정 아님 |
| CKG stdio 합성 probe | 태그·edge·정확 조회·없는 ID·EOF PASS | 수동 annotation의 간접 공개. 자동 탐지/CKS 연결/조직 권한 아님 |
| CKS 실제 등록 schema probe | overlay 모의 fixture 실행 | 서버 실제 입력과 소비자·fixture 정적 계약 비교; 실제 Claude 호스트 미시험 |

skip3은 `TestImpact_SelfGraph`, `TestImpact_SelfGraph_Deterministic`, `TestFindCallers_InterfaceDispatchBridge`다. 두 self-graph 시험은 CKG_SELF_GRAPH_DB 미지정, interface bridge 시험은 기존 fixture에 coll1.Thing.Hash가 없어 건너뛰었다. PASS 수에 더하지 않았다. 별도 `e2e` build-tag 시험도 실행하지 않았다. 실제 체인·LLM·사용자 세션 산출물·독립 reviewer 수용은 이번 감사에 포함되지 않는다.

## 다음 작업과 우선순위

감사 항목과 문서·원자료 SHA·소스 불변·진행 원장 검증을 완료했다. 로컬 감사 커밋을 보존하며 외부 저장소 변경·새 PR은 수행하지 않는다. 기능 구현의 다음 권고는 **WI-V1**이며, WI-3/SP의 소비자 입력 계약 정합을 연결 실행의 선행 조건으로 같이 고정한다. 그다음 WI-V2→WI-3→WI-4→WI-1→WI-2→WI-C→WI-6→WI-SP다. WI-5는 담당 저장소 확보 후 별도로 진행한다. 각 기능은 구체 명세·반례·실제 실행·독립 수용이 끝나기 전 완료 체크하지 않는다.

태그/공개 배포와 나머지 후속 범위는 [남은 전체 작업](../REMAINING-WORK.md)에 유지한다. 이번 감사로 다른 기능·운영 호스트·독립 품질 평가의 완료 수를 올리지 않는다.
