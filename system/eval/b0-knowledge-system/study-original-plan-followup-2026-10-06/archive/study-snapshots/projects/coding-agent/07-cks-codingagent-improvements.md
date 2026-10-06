# CKS · coding-agent 추가 개선 제안 (사전 검토용)

> 목적: 시니어 개발자의 디버깅 행동(증상 확인 → 재현 → 코드 flow walk → 원인 확인 → 최선의 수정 선택 → 재현 테스트로 검증)을 **coding-agent + CKS(ckg/ckv)** 로 모방·강화하기 위한 추가 개선 영역 정리.
> 작성일: 2026-06-25
> **주의: 이 문서는 "각 제안이 무엇을 어떻게 해소하는지" 이해용. 구현 설계노트(스키마·엣지타입·MCP 시그니처)는 아직 만들지 않음** — 사용자 확인 후 진행.
> 근거: 4개 레포(coding-agent / code-knowledge-system / code-knowledge-graph / code-knowledge-vector) 코드 매핑(2026-06-25).
> 연계 원칙: **(가)** 오라클로만 검증 말고 오라클 자체를 검증 · **(나)** GREEN은 필요조건이지 충분조건 아님("증상 사라짐" ≠ "원인 고침").

---

## 0. 용어 — AST / CFG / PDG / SSA / CPG / taint

작은 예시 코드:
```go
func handle(req Request) Resp {      // req = (잠재적) 신뢰 못 할 입력
    id := req.UserID                 // (1) 값 생성
    if id == "" {                    // (2) 분기
        return Err()                 // (3)
    }
    row := db.Query(id)              // (4) sink: id가 여기로 흘러감
    return Ok(row)
}
```

| 용어 | 무엇 | 위 예시에서 | CKG 현황 |
|---|---|---|---|
| **AST** | 코드 구조 파스트리 | 함수·if·호출 노드 | [OK] 있음(37 노드 타입) |
| **CFG** (제어흐름) | 실행이 갈 수 있는 경로(블록↔블록) | entry→(1)→(2)→{(3) \| (4)} | [X] 문장 노드는 있으나 **실행 엣지 없음** |
| **데이터 의존** (PDG의 절반) | "이 값을 누가 만들고 누가 쓰나"(def→use) | (1) `id` → (4) `db.Query(id)` | [X] `reads/writes_field`는 *이름*이지 *값 흐름* 아님 |
| **제어 의존** (PDG의 나머지) | "어느 분기가 이 실행을 결정하나" | (2)가 (3)/(4) 실행을 결정 | [X] 없음 |
| **PDG** | 데이터+제어 의존 그래프 | 위 둘 합집합 | [X] 없음 |
| **SSA** | 변수를 1회 대입으로 재명명 → def-use 정확화 *수단* | `id₁` 단일 정의 | [X] `go/types`만, `go/ssa` 미사용 |
| **CPG** (Yamaguchi) | AST+CFG+PDG를 한 그래프로 → traversal로 취약점 질의 | "신뢰 못 할 `req`가 `db.Query`까지 sanitize 없이 도달?" | [X] (이게 (1)의 목표) |
| **taint / source→sink** | 신뢰 못 할 source 값이 위험 sink까지 흐르는지 추적 | req → id → db.Query | [X] |

핵심: **취약점 스멜**과 **produce→store→consume**, 시니어의 **"값을 따라 walk"** 는 전부 *PDG(데이터 의존)* 가 있어야 네이티브 질의가 됨. "이 지점에 도달하는 경로"는 *CFG* 가 있어야 함. 지금은 호출그래프+필드명으로 수동 조립해야 함.

---

## 1. 제안 한눈에 + 상태

| # | 영역 | 해소하는 시니어 행동 / 원칙 | 상태(사용자 결정) |
|---|---|---|---|
| (1) | CKG **dataflow(PDG)·CFG 레이어** | 값을 따라 walk · 취약점 스멜 · produce→store→consume | [검토] **추가 검토·리뷰 후 진행** |
| (2) | **온톨로지(타입·태깅)** | 취약점 스멜 · 의미 기반 검색 | [재논의] **재검토 — LLM 비의존 방식으로** |
| (3) | **증상→의심코드 후보** (CKS 추림·라벨링 + LLM walk·판단) | 증상에서 walk — 후보 추림·객관 신호 라벨링은 CKS, 랭킹·판단은 LLM (fault localization) | [OK] **필요 확정 (모델 수정)** |
| (4) | **비결정(flaky) 버그 처리** | <100% 재현(타이밍·동시성) · 원칙 (나) | [고민] **추가 고민 필요** |
| (5) | **symptom→cause→fix 학습 루프** | "예전에 이 증상 어떻게 고쳤지"(경험) | [OK] **필요 확정** |
| (6) | CKS **다단(multi-hop)·dataflow-shaped 확장** | 값을 끝까지 따라가기(1-hop으론 부족) | [OK] **필요 확정** |

상태 범례: [OK] 진행 대상 · [고민] 더 고민 · [검토] 추가 검토 후 · [재논의] 방식 변경 제안

---

## 2. 영역별 상세

각 항목 = **무엇을 위한가 / 지금 있는 것(근거) / 빈 곳 / 어떻게 해소 / 상태**.

### (1) CKG dataflow(PDG)·CFG 레이어 — [검토] 추가 검토 후
- **무엇을 위한가:** (2)(3)(6)과 "취약점 스멜·값 따라가기·produce→store→consume"의 **공통 전제(키스톤)**.
- **지금 있는 것:** AST 노드 37종, 호출엣지(`calls`/`invokes`), `reads_field`/`writes_field`(이름 수준), 동시성 엣지, `IfStmt`/`LoopStmt`/`CallSite` 노드. `internal/parse/golang/dataflow.go`는 **스텁**, `go/ssa` 미사용.
- **빈 곳:** 값 흐름(def→use) 엣지 없음 · CFG(실행 엣지) 없음 · source→sink 도달성 쿼리 없음 · 함수 간 전파 없음.
- **어떻게 해소:** `go/ssa` 기반 dataflow 단계를 실제 emit(def→use + CFG + 함수 간), source→sink 도달성을 네이티브 쿼리로.
- **왜 검토 필요(=네 결정 존중):** 구현 난도·빌드비용이 가장 큼(CKG가 D1으로 미뤄둔 이유). 전체 대신 **"증상 관련 서브그래프 한정 on-demand dataflow(deep 모드)"** 로 범위 축소가 현실적인지, 정확도/비용 트레이드오프를 더 봐야 함. → **검토 항목:** (a) 범위(전체 vs on-demand), (b) 언어별(Go는 ssa 있음 / TS·Solidity는?), (c) 정확도 vs 처리량.

### (2) 온톨로지(타입·태깅) — [재논의] 재검토: LLM 비의존으로 재구성
- **무엇을 위한가:** "이건 state-writer/검증경계/security-sink"처럼 **의미 타입으로 코드를 질의**(취약점 스멜·의도 기반 검색).
- **지금 있는 것:** Policy/SecurityPattern YAML 오버레이(운영자 수동 큐레이션, schema 1.14/1.15) + CKS 지식타입 B1~B7(단, 검색이 타입으로 라우팅 안 함).
- **네가 제기한 문제(정확함):** 내가 앞서 말한 "추론(inference)"은 **항상 LLM 연동**을 전제 → 비용·비결정성·런타임 의존이라는 문제. → **그래서 LLM은 임계경로에서 뺀다.**
- **재구성한 해소책 — 채우는 방법 3층, 우선순위 순:**
  1. **결정론적 정적 규칙(주력, LLM 없음):** CPG((1)) 위에 *룰 기반 디텍터* 로 노드를 태깅 — 예: "외부호출 후 상태쓰기"=reentrancy, "검증 안 거친 입력의 sink 도달"=taint. **이게 CodeQL·Semgrep·Slither가 하는 방식**(전부 룰 기반, 재현 가능, 감사 가능, 쌈).
  2. **수동 YAML 큐레이션(이미 있음):** 규칙으로 못 뽑는 도메인 사실(합의 임계값 등).
  3. **LLM enrichment(선택·오프라인만):** 규칙이 못 닿는 fuzzy 태깅에 *빌드타임 1회·캐시*, **질의 시엔 절대 호출 안 함.**
- **더 솔직한 재검토 — "온톨로지가 최선인가?":** 목표(취약점 스멜·원인 후보)에 **무거운 형식 온톨로지(RDF/OWL류)는 과할 수 있음.** 실제로 필요한 건 (a) **dataflow 레이어((1))** + (b) 작고 실용적인 **타입 어휘 + 결정론적 룰 팩(detector)** 으로 노드를 태깅하는 "온톨로지-lite". 즉 *형식 온톨로지 기계*가 아니라 **룰 기반 태깅 + 타입 필터 질의**가 가치의 핵심.
- **의존성:** 취약점-급 룰은 (1)(dataflow) 필요. 단, **구조 패턴 태깅(YAML·단순 구조 규칙)은 (1) 없이도 지금 가능** → (1)과 분리해 선행 가능.
- **상태:** 방식을 "LLM 추론" → "결정론적 룰 기반 태깅(+선택적 오프라인 LLM)"으로 바꿔 재논의.

### (3) 증상→의심코드 후보 — CKS 추림·라벨링 + LLM walk·판단 — [OK] 필요 확정 (모델 수정)
- **무엇을 위한가:** 시니어가 증상(스택/에러/site)에서 **거꾸로 코드를 walk하며 의심 지점을 좁히는** 행동(fault localization).
- **모델 수정 (사용자 지적 반영):** "CKS가 의심도 *점수*를 매겨 정답 후보를 정렬"하는 방식은 약함 — **의심도는 증상마다 달라지므로 그 *판단*은 LLM 몫.** 사실(객관 신호)과 판단(증상 의존)을 분리한다.
  - **CKS 역할 = 추림(prune) + 객관 신호 라벨링 + 반복 확장 서빙.** 증상 주변 연결을 추려 후보를 모으고, 각 후보를 *문제와 무관하게 일정한* 신호로 라벨링: 도달성(증상 지점에 닿나)·거리(hop)·최근 변경(blame/`change_history`)·동시성 접촉(`concurrency_impact`)·dataflow 경로상 여부((1))·security-pattern 적중. LLM이 "X의 이웃 더 줘"라고 하면 **반복 확장**((6)).
  - **LLM 역할 = 가설 → walk → 추가 연결 요청 → 판단.** 증상 의미로 신호를 *가중*해 의사결정. (= just-in-time / agentic retrieval, 06 문서 B1과 정합. "코드는 그래프 → walk"의 실제 구현.)
- **지금 있는 것:** forward(`impact_analysis`), walk 도구(`find_callers`/`find_callees`/`get_subgraph`), blame/hunk·`change_history`·`concurrency_impact`, 토큰 버짓(composer). → LLM walk를 지원할 1차 도구는 이미 존재.
- **빈 곳:** 점수 정렬이 빠진 게 아니라 — 증상을 받아 *후보 이웃을 추려 객관 신호로 라벨링*해 주는 **통합 진입점**이 없음(지금은 LLM이 개별 툴을 수동 조합).
- **점수는 어디로:** (a) CKS 고정 점수 정렬 = 브리틀(증상별 가중 불가) → **채택 안 함.** (b) LLM이 신호를 증상별 가중 → **채택.** 중간: CKS가 *기본 정렬*(거리+최근변경) 또는 *intent-파라미터라이즈드* 정렬을 **참고용 seed**로만 제공 가능(정답 아님).
- **균형추:** 순수 "LLM 맨몸 walk"는 비용·context rot 위험 → CKS의 추림/라벨링/토큰버짓이 필수. **"CKS 스코어링"도 "LLM 맨몸 탐색"도 아닌 그 사이.**
- **의존성:** 정밀도엔 (1)(역방향 도달성) 도움. 단 (1) 없이 호출그래프+blame+concurrency 조합으로 **1차 버전 가능**(정밀도는 (1) 이후 향상).

### (4) 비결정(flaky) 버그 처리 — [고민] 추가 고민
- **무엇을 위한가:** 네가 든 "<100% 재현 = 타이밍·동시성·스케줄링" 클래스. 원칙 **(나)** 의 동시성판.
- **지금 있는 것:** reproduce-first가 **결정론적 단발 재현 가정**. <100%면 "setup 부적절/가설 오류"(anti-pivot D-2)로 처리. `-race`는 식별된 패키지에만, `-count`/stress/fault-injection 없음(chainbench `network_partition`은 시나리오 셋업으로만).
- **빈 곳:** 재현율(reproducibility-rate) 개념 자체가 없음. flaky를 "테스트 한계"가 아니라 "가설 오류"로 오해.
- **잠정 해소 방향:** 오라클을 N회 실행해 실패율 측정 → 결정론(100%)/확률(<100%) 분류 → 확률형은 오라클이 통계적("N회 중 ≥k")이 되고 `-race -count`·partition/timing을 *진단 도구*로 승격. GREEN 기준도 "1회 통과 ≠ 수정, N/N(부하 하) 통과"로.
- **왜 더 고민(=네 결정 존중) — 열린 질문:** (a) N·임계 k를 어떻게 정하나(비용 폭증 위험), (b) 비결정 원인이 코드가 아니라 *환경*(PC 지연·네트워크)일 때 오탐, (c) 통계적 오라클을 어디까지 신뢰할지, (d) 결정론 가정이 깨질 때 파이프라인 상태기계 영향. → 다음 라운드에서 별도 설계 토론.

### (5) symptom→cause→fix 학습 루프 — [OK] 필요 확정
- **무엇을 위한가:** 시니어의 경험 — "이 증상, 예전에 어떻게 고쳤지". 발표의 **"CKS=암묵지 제도화"** 의 실제 구현.
- **지금 있는 것(절반):** CKV에 **PR/commit 코퍼스 파서(`internal/parse/prdoc`)가 이미 존재**하나 빌드에 미연결(`--include-pr-history` 미구현, 처리량 0.74 chunks/s 블로커). why-queries 픽스처는 `pending`. CKS엔 **수정 성공/실패 결과 피드백 루프 전무.**
- **빈 곳:** 과거 증상→원인→수정 트리플의 임베딩/검색이 비활성 · 결과(성공/회귀) 신호로 re-rank·stale 표시 없음.
- **어떻게 해소:** (a) `--include-pr-history` 배선 + 과거 세션 `analysis.md`의 **incident signature(증상→근본원인→수정) 임베딩**. (b) CKS에 **outcome 로그**(검색→수정→성공/회귀) 추가해 재랭킹·stale에 활용. 처리량은 ANE 친화 임베딩 모델 전환과 병행.

### (6) CKS 다단·dataflow-shaped 확장 — [OK] 필요 확정
- **무엇을 위한가:** "값을 끝까지 따라가기" — 1-hop으론 부족.
- **지금 있는 것:** CKS Stage3 그래프 확장이 **1-hop only, 호출엣지 위주**(`internal/composer/stage3/expander.go`).
- **빈 곳:** 다단 traversal 없음, dataflow 엣지 추종 없음.
- **어떻게 해소:** 버그수정 의도일 때 **dataflow 엣지 따라 다단 확장**((1)활용), 온톨로지((2)) 관련성으로 bound.
- **의존성:** dataflow 추종은 (1) 필요. 단, **호출엣지 기준 다단 확장(depth↑)은 (1) 없이도 선행 가능.**

---

## 2b. 언어별 CPG 경로 (웹 검증 · 2026-06-25)

> (1)(dataflow/PDG·CFG)의 "[검토] 추가 검토" 중 *언어별 실현 가능성* 항목. 1차 출처(공식 docs·repo) 검증. 등급: [OK]VERIFIED / [부분]PARTIAL / [X]미확인.

**아키텍처 원칙(재확인):** 하나의 보편 dataflow 엔진을 만들지 말고 — **CKG가 언어-중립 CPG 스키마를 정의 → 언어별 어댑터가 "각 언어 최선의 분석기"에서 정규화(normalize)**. 언어별 정밀도 차이는 CKG의 `confidence`(EXTRACTED/INFERRED/AMBIGUOUS)로 표시해, 적대적 verify가 "TS는 dataflow 신뢰도 낮음 → 런타임 probe 보강" 식으로 판단.

### 요약
| 언어 | 권장 경로 | SSA·CFG·DDG | 소비 방식 | 정밀도 | 비고 |
|---|---|---|---|---|---|
| **Go** | `go/ssa` + `go/callgraph` (자체) | [OK] SSA+CFG, 콜그래프(cha/rta/vta) | 라이브러리 in-proc | 높음 | 주 스택, 가장 쉬움 |
| **Solidity** | **Slither(SlithIR=SSA)** Python API | [OK] SSA IR + CFG + data-dependency | Python API(권장) / JSON·SARIF(findings만) | 높음(보안 특화) | detector 118개. Joern 미지원 |
| **TypeScript** | CodeQL **또는** tsc 타입체커 / Joern jssrc2cpg | [부분] dataflow+taint(CodeQL) / 근사(tsc) | CodeQL DB / 라이브러리 | 중~낮음 | 동적성으로 난도↑, interproc 어려움 |

### Go — [OK] 자체 구축, 최고 정밀
- `golang.org/x/tools/go/ssa`: *"a static single-assignment (SSA) form intermediate representation (IR)"*; BasicBlocks Preds/Succs = *"the control-flow graph or CFG"*. ([OK] pkg.go.dev, x/tools v0.46.0)
- `go/callgraph` + `cha`/`rta`/`vta`/`static` = 함수 간 콜그래프. ([OK])
- → SSA·CFG·콜그래프가 표준 제공 → DDG(def-use)를 자체 emit 가능. CKG는 현재 `go/types`만 사용(이 단계 비어 있음).
- Joern에도 `gosrc2cpg` 있으나 외부 `goastgen` 바이너리 의존 + 공식 maturity 라벨 없음([부분]) → 자체 `go/ssa`가 더 직접적.

### Solidity — [OK] Slither에 기댐 (재발명 불필요)
- **SlithIR가 SSA 형태:** *"Slither possess a Static Single Assignment (SSA) form representation of SlithIR ... a key component for building an efficient data-dependency analysis."* ([OK] Slither wiki: SlithIR-SSA)
- CFG printer + data-dependency 보유. ([OK] README) — 단 "taint"는 Slither가 아니라 **Mythril** 문서 용어([부분]: Slither는 "data-dependency"로 문서화).
- **detector 118개**(현재 Detector-Documentation 기준; 레거시 "90+" 아님). ([OK])
- solc 경유 컴파일, crytic-compile로 Foundry/Hardhat 통합. ([OK])
- **CPG 적재용 소비:** `--json`/`--sarif`는 *findings*만 담음; **Python API가 SlithIR(SSA)+CFG+data-dependency 전체를 노출** → CKG CPG 스키마 적재엔 Python API가 정답. ([OK])
- 대안: `solc --ast-compact-json`(AST) + `--ir`/`--ir-optimized`(Yul IR). ([OK] soliditylang docs)
- 보조: Mythril(심볼릭 실행+taint, 바이트코드), Securify v2(~37 checks, 바이트코드). ([OK])
- [주의] **정정:** Joern은 **Solidity 프론트엔드가 없음**(지원 목록에 부재). 앞서 내가 "존재"라 한 것은 오류. ([OK] 부재 확인)

### TypeScript — [부분] 가능하나 가장 어려움
- **CodeQL**: JS+TS 지원(TS는 JS extractor로, TS 2.6–5.9), *"Global taint tracking"* + **inter-procedural** TaintTracking 라이브러리 보유. 단 **배치형**(CodeQL DB 추출 후 질의), 라이브 린터 아님. ([OK])
- **Semgrep taint 모드**: TS 지원하나 **기본(CE)은 함수 내(intra-procedural)**; *"Interprocedural taint analysis is a Semgrep Pro feature"* — 교차함수/교차파일은 Pro 전용(인터파일 ~8GB/core). ([OK] — 중요 단서: "Semgrep이 교차함수 taint" 라고 단정 금지)
- **tsc 타입체커**(`program.getTypeChecker()`) + **ts-morph**(컴파일러 API 래퍼)로 타입·심볼 해소 → def-use 근사의 토대. ([OK])
- **Joern `jssrc2cpg`**가 TS 처리(기본 TS 타입 생성, `--no-tsTypes` 플래그). 공식 maturity 라벨 없음(2023 블로그가 타입 전파를 "experimental"로 표현). ([부분])
- 어려움 근거: CodeQL/Semgrep 모두 고차함수·콜백·동적 디스패치로 인한 dataflow 난도를 공식 인정(정확 문구는 아님, 취지 [부분]).
- → 현실: **tsc 기반 intraprocedural 근사부터 → interprocedural 정밀 PDG는 CodeQL 연동 또는 점증.**

### CPG 정의 (출처 확정)
- CPG = AST + CFG + PDG(데이터+제어 의존)를 한 그래프로. Yamaguchi et al., *"Modeling and Discovering Vulnerabilities with Code Property Graphs"*, IEEE S&P 2014 — 추상: *"combines properties of abstract syntax trees, control flow graphs and program dependence graphs"*, Linux 커널 신규 취약점 18건. ([OK] — 너희 `04-verification` §1과 동일 출처)
- **Joern** = 이 CPG의 실용 플랫폼(다언어, Scala DSL 질의). 프론트엔드: c2cpg, csharp2cpg, ghidra2cpg, **gosrc2cpg**, javasrc2cpg, jimple2cpg, **jssrc2cpg(JS+TS)**, kotlin2cpg, php2cpg, pysrc2cpg, rubysrc2cpg, swiftsrc2cpg — **Solidity 없음**. 프론트엔드별 stable/experimental 공식 라벨 없음([X]). ([OK] 목록 / [X] 라벨)

### 권장 시퀀스 & 함의
1. **Go** — `go/ssa` 자체, 즉시·최고 정밀.
2. **Solidity** — Slither Python API → CPG 스키마 정규화. 보안 직결 + SSA/detector를 거의 공짜로 → **ROI 최고** (slop-guards의 Slither 채택과 일관).
3. **TypeScript** — 가장 나중. tsc 근사(intra) → CodeQL(interproc) 점증, 낮은 정밀도는 `confidence`로 표시.
- **함의:** "한 엔진 3언어"보다 **언어별 분석기 → 공통 CPG 스키마 정규화**가 현실적. Joern은 Go/TS엔 옵션이나 **Solidity엔 불가** → Solidity는 Slither가 사실상 유일한 성숙 경로.
- → (1)의 "언어별" 검토 항목은 본 절로 1차 해소. 남은 (1) 검토는 **범위(증상 서브그래프 on-demand vs 전체)** 와 **정밀도/빌드비용** 트레이드오프.

### 미확정·주의 (04-verification 규율)
- Joern 프론트엔드 stable/experimental 라벨, `gosrc2cpg`·`jssrc2cpg` maturity: **공식 문서에 라벨 없음**([X]) → 착수 전 PoC로 정밀도 직접 측정 권장.
- Slither "taint" 표현은 회피(공식은 "data-dependency").
- soliditylang "latest" 페이지 직접 fetch 403 — 옵션은 인덱스/버전 미러(v0.8.20)로 확인, 착수 전 라이브 재확인 권장.

### 출처 (1차)
- Go: pkg.go.dev/golang.org/x/tools/go/ssa · /go/callgraph
- Solidity: github.com/crytic/slither (+ wiki: SlithIR-SSA, Detector-Documentation, Usage, Python-API) · docs.soliditylang.org/en/latest/using-the-compiler.html · github.com/crytic/crytic-compile · github.com/ConsenSys/mythril · github.com/eth-sri/securify2
- TS/JS: codeql.github.com/docs (supported-languages, analyzing-data-flow-in-javascript-and-typescript) · docs.semgrep.dev/writing-rules/data-flow/taint-mode · github.com/microsoft/TypeScript/wiki/Using-the-Compiler-API · github.com/dsherret/ts-morph
- Joern/CPG: docs.joern.io (/, /frontends, /code-property-graph) · github.com/joernio/joern · Yamaguchi et al. IEEE S&P 2014 (comsecuris.com/papers/06956589.pdf)

---

## 3. 적대적 verify와의 연결 (왜 (1)(2)(3)이 중요한가)

이전 논의의 적대적 verify가 *theater*가 안 되려면 **독립 증거**가 필요. (1)(dataflow)·(2)(타입 태깅)·(3)(의심 랭킹)이 그 객관적 증거를 공급함:
- **원칙(가) 오라클 검증:** (4)(flaky-class) + 증상 앵커가 "이 테스트가 진짜 그 증상을 그 이유로 잡나"를 데이터로.
- **수정 선택 judge-panel:** "최선의 수정 포인트인가"를 *의견*이 아니라 (1)도달성·(2)타입·(3)의심랭킹·`impact_analysis`로 채점 → produce 지점 vs consume(증상가림) 구분, write-site 완전성을 그래프로 증명.

(참고: 앞서 합의한 두 개선 — **오라클 검증 서브스텝**, **수정선택 judge-panel** — 은 coding-agent에 부분만 존재: reproduce-first의 symptom-bound RED·`reproduction_inadequate`·부모커밋 재확인은 있으나 민감도/특이도(mutation) 검사·독립 재유도 없음 / 수정선택은 first-plausible-fix이고 §5.2b·§4.8은 *무결성 게이트*이지 *후보 점수화*가 아님.)

---

## 4. 의존성 & 선후

```
(1) dataflow/PDG·CFG (키스톤, [검토]검토) ──┬─→ (2) 룰기반 온톨로지 태깅([재논의], 취약점-급)
                                      ├─→ (3) 증상→의심 랭킹([OK], 정확도 향상분)
                                      └─→ (6) dataflow-shaped 확장([OK], 추종분)

(1) 없이도 선행 가능한 부분:
  (2) 구조패턴·YAML 태깅(LLM 없음) / (3) 호출그래프+blame 1차버전 / (6) 호출엣지 다단 확장
(4) flaky([고민] 추가 고민) ── reproduce-first/evaluator, (1)과 독립
(5) 학습 루프(PARTIALLY, 설계 존재·미구현) ── 권위: continuous-learning-loop.md(P1 capture부터), (1)과 독립 — §6 참조
```

요지: **(1)은 (2)(3)(6)의 "정밀" 버전을 여는 키스톤**이지만, (2)(3)(6) 모두 **(1) 없이 1차(저정밀) 버전을 먼저** 낼 수 있음. (4)(5)는 (1)과 무관하게 진행 가능.

---

## 5. 다음 단계 (사용자 확인 후)
- 각 레포 설계노트는 **아직 만들지 않음**. 본 문서로 "무엇을 어떻게 해소하는지" 합의가 끝나면:
  - [OK] 확정((3)(5)(6)): 구현 계약(엣지/툴 시그니처) 초안 작성 대상.
  - [재논의] (2): "결정론적 룰 기반 태깅(+선택적 오프라인 LLM)" 방식으로 합의되면 룰 팩 범위 정의.
  - [고민] (4): 재현율·N·환경오탐을 별도 설계 토론.
  - [검토] (1): 범위(on-demand deep)·언어별·정확도/비용 추가 검토.

---

## 6. 검토 갱신 — 기존 코드/로드맵 대조 (2026-06-25)

4개 레포 최근 커밋 재검토로 갱신. (CKG는 의존성/CI 범프만 → 기능 변화 없음.)

### 기존 자산·로드맵이 이미 있음 (중복 제안 회피)
- **(5) 학습 루프 = 이미 설계 존재(미구현).** `coding-agent/docs/design/continuous-learning-loop.md`(2026-06-08)가 권위. 6-stage(capture→sanitize/dedup→curate→PR→activate→bench), two-key 승격(no auto-verify), cks read-only(PR로만 진입). → 내 별도 설계 폐기, `08` 정정판이 그 설계에 정렬. **상태: PARTIALLY(설계 있음, 미구현).**
- **하드닝 백로그 존재:** `coding-agent/docs/cks-ckg-ckv-hardening-backlog-2026-06-19.md`. **merged:** A1(ckv embedding identity), A2(ckg loud incompleteness), B1(ckg N+1), B2(ckg sqlite pragma). **pending:** B3(reverse-deps LIKE index), B4(impact 1-pass), B5(context/goroutine), Policy/SecurityPattern 노드 MCP 미노출 등.

### 제안별 현황 정정
- **(1) dataflow/PDG·CFG:** 여전히 NOT PRESENT.
- **(2) 온톨로지/룰 태깅:** NOT PRESENT. 단 CKG가 Policy/SecurityPattern 노드를 인덱싱하나 **MCP 미노출**(백로그 pending) — 룰 태깅의 부분 토대.
- **(3) 증상→의심:** PARTIALLY. `root-cause-lifecycle`(역방향 consumer 추론+self-refute) + `investigative-probe`(런타임 관찰)는 있음. **CKS측 라벨/랭킹·후보 pruning은 없음** → #3 모델(추림·라벨링)과 정확히 일치하는 갭.
- **(4) flaky:** NOT PRESENT(재현은 여전히 binary).
- **(6) 다단 확장:** PARTIALLY. `get_subgraph`·`concurrency_impact`(goroutine/channel/lock 전이 reach)는 있으나 **dataflow-shaped 다단은 없음**.

### V1·V2 및 신규 인접 기능
- **side_findings (coding-agent #27, merged):** analyzer가 `related-code.json.side_findings[]`로 "고치다 발견한 인접 결함(다른 증상, in_scope:false)"을 PR `## Follow-ups`로 라우팅. **이건 V2(같은 증상에 대한 다중 수정안 선택)가 아님** — 별 증상을 *수정 안 하고 라우팅*. V1/V2와 인접하나 별개.
- **V1 오라클 검증:** 오라클 게이트(RED/GREEN, 부모커밋 재확인)는 있으나 **mutation/민감도 없음** → 여전히 갭.
- **V2 수정선택:** 여전히 first-plausible-fix(티켓당 단일 plan). **다중 후보 생성·judge-panel 없음.**
- **D-7(evaluator #26):** per-cycle 단위 게이트가 "수정 자신의 테스트만" 실행, 광역 회귀는 §8.0로 분리.

### CKV/CKS 정정 사실
- **CKV `Filter`에 `ChunkKind` 멤버 없음** → PR/incident 청크를 질의에서 선택/배제 불가(내 이전 "chunk_kind 필터 존재"는 오류). 단 (5) 권위 설계는 신규 코퍼스가 아니라 entry 승격이라 영향 적음.
- **CKV embedding-identity 강제(#12)·canonical_id 청크 상속(#9):** 인덱스 임베더 정체성 체크섬 강제 / chunk→ckg 교차링크 가능.
- **임베딩 모델:** Qwen3-Embedding-4B 권고(정밀도↑, 처리량 동일).
- **CKS:** 13 MCP 툴·RRF 가중·outcome 피드백 부재 — 변화 없음.
