# DECISIONS — 확정된 설계 결정 로그

> 이 세션의 모든 확정 결정. 합의 즉시 정본에 증분 기록(D-016). GLOSSARY와 함께 단일 권위.
> 상태: FROZEN v1 (2026-06-11). DECIDED = 확정 · OPEN = 미결.

---

## D-001 (F1) 티어/서빙 어휘 — DECIDED (2026-06-10)

**결정:** "L1/L2/L3 서빙" 어휘를 폐기하고 **두 개의 직교 축**으로 분리.

- **축 A — 저장/신선도 (storage):** `CURATED` vs `COMPUTED`
  - `CURATED` = 저장·느리게 변함 (규칙/불변식/도메인/에피소드)
  - `COMPUTED` = 코드에서 매 질의 파생, **절대 저장 안 함** (impact_analysis / concurrency_impact / callers)
  - SSoT 스키마에 `storage: curated|computed` 필드로.
- **축 B — 전달 견고함 (delivery, 안전-핵심 CURATED에만):** 실제 시스템의 `L0 → L1 → L2 → L3` 사다리 그대로 보존(`00-system-contract §4.3`). L3 = always-on backstop(바닥).

**근거:** 옛 3-분류 중 `ALWAYS_ON`≈L3, `RETRIEVED`≈L0/L1로 실제 L0~L3과 중복. 고유하게 이름이 필요한 건 `COMPUTED`(저장 금지, #1 무효화 설계의 전제)뿐. "tier/L"은 생태계에서 5중 과적재(ckg 추출 Tier1/2, 도메인 T1/T2 등)라 숫자/“tier” 재사용 회피.

**진짜 정답 출처:** `coding-agent/docs/r1-refactor/00-system-contract.md §4.3` + `plugin/skills/stablenet-invariants/SKILL.md:7` ("always-on backstop — L3"). 이론 문서의 "L1=always-on"은 탐색 전 CPU-캐시 관례로 쓴 오류.

---

## D-002 (F2) 결정성 프레이밍 — DECIDED (2026-06-10)

**결정:** #1(`01-determinism-and-convergence.md`)의 프레이밍이 정본.
- 결정성은 **사실(evidence) 층에만** 적용. 해법 층은 고정 금지(best-of-N + 게이트로 수렴).
- `IntegrityHash` = *결정성 보장*이 아니라 **드리프트 탐지 센서**.
- evidence_key = `hash(normalized_query, code_commit, knowledge_version, retrieval_policy_version)`.

**폐기:** 통합 문서의 "(prompt,commit)→동일 pack **하드 보장** / IntegrityHash로 **보장**" 표현.

**확증:** 실제 cks의 IntegrityHash는 "변조 탐지"용으로 구현됨(탐색 확인) → #1의 "센서" 해석을 코드가 뒷받침. 비결정 주범은 FakeEmbedder.

---

## D-003 (F5) ID 표기 규약 — DECIDED (2026-06-10) → ⚠ SUPERSEDED by D-023(개정, 2026-06-12)

**결정(폐기됨 — 기록 보존용; 정본은 D-023 개정):**
- 불변식: `INV-<DOMAIN>-<SLUG>` 전부 **대문자** (예: `INV-LEDGER-CONSERVATION`).
- 규칙: `RULE-<SLUG>` 대문자.
- **cherry-pick은 규칙(`RULE-CHERRY-PICK`)으로 단일화.** 렌즈의 "근거"에는 규칙/불변식 혼재 허용(렌즈 = 불변식만이 아님).
- 03(`03-rule-invariant-ssot.md`)을 표기 권위로.

---

## D-004 (F3/F4) 결정성 키 합성 — DECIDED (2026-06-11)
`retrieval_policy_version`를 `knowledge_version`에서 제외(이중계산 제거), evidence_key의 형제로만. `retrieval_policy_version` = `_ssot/retrieval-policy.yaml`(intent+모듈→도구·depth)의 버전. `normalized_query`는 LLM 자유생성 금지, 티켓 필드 기반 결정적 구성(D4 해소). → GLOSSARY §2.

## D-005 (F6) enforce 형식 — DECIDED
구조화 리스트 `[{type, ref}]`, `type ∈ {chainbench, static, review}`, 비어있으면 안 됨. → GLOSSARY §4.

## D-006 (F7) ADR 생산지점 — DECIDED
ADR은 DESIGN/freeze에서 생산, FREEZE 산출물에 `adr.md` 추가. → GLOSSARY §6.

## D-007 (F8) constraint-assembler 의존성 — DECIDED
P0 버전은 **degraded**로 동작: L3 always-on 불변식 + impact_analysis만으로. SSoT rules/`governed_by`는 P1에서 보강. 로드맵에 의존성 명시.

## D-008 (F9) 설계 생성 전략 — DECIDED
**best-of-N 병렬 생성 → 캐노니컬 선택(#7 §3.2)**, 잔여 VIOLATION은 **순차 revise ≤K**. 둘 결합. → GLOSSARY §6.

## D-009 (C1) ckg ROI 재해석 — DECIDED (2026-06-11)
"검색이 cat에 패배(0.335)"는 **가짜 임베더 파이프라인의 N=1 측정(폐기됨)**이지 그래프 측정 아님. ckg 자체 eval F1=0.96. 결론: "미검증"이지 "패배" 아님. **Gate 1을 역할별 분리 검증** — ckg traceability(증명됨, 재확인만) vs ckv fuzzy retrieval(실 임베더로 별도 측정). → `06` C1 정정.

## D-010 (A1) 정확한 수렴 + 출력 안정 — DECIDED (2026-06-11)
**둘 다 달성**(사용자 목표). 게이트 위 3층(입력결정화+캐노니컬선택+재검증메모이제이션)으로 VARIANT collapse, 단 **안정성은 정확성 게이트에 종속**(stably-wrong 방지). → `07`.

## D-011 FakeEmbedder 금지 — DECIDED (2026-06-11, 사용자 지시)
**결정:** FakeEmbedder는 개발 중 테스트 용도로만 존재했고, **이제부터 사용 금지.** 현 단계는 *실제 코드 기반 DB(knowledge-data 실 bge-m3 인덱스)로 실동작을 검증*하는 단계. 모든 측정·검증은 실 임베더로. → Gate 0 배관의 핵심, C1(D-009)의 원인 제거.

## D-012 stale 본문 처리 — DECIDED (2026-06-11)
정본은 GLOSSARY에 집약, stale deep-dive 본문엔 경고 헤더만(본문 재작성 보류). **잔여 위험**: 헤더 못 보고 본문 직독 시 오인 가능. **완화 계획:** freeze 직전 정본 확정 후 본문을 정본에 맞춰 1회 정리 → 잔여 위험 0. 그때까지 README §3 정정로그 + GLOSSARY가 우선 권위.

## D-013 모듈 criticality = 점진적 (default + override) — DECIDED (2026-06-11, 사용자)
**결정:** 모든 모듈을 지금 프로파일링하지 않는다(불가능·비효율). **기본값 + 명시 override** 모델: 프로파일 없는 모듈은 안전한 기본값(불변식 + 일반 모범사례), *지배 기준이 분명한 소수 모듈만* 명시 criticality 프로파일(속도/안정성/지연허용 우선순위). 핵심부터(consensus/state/systemcontracts). **나머지는 externalizer 루프로 성장**(리뷰/실패 → 새 프로파일, 사람 게이트). = "만족할 만한 첫 버전 → 루프로 정교화"(00-purpose §4). SSoT(#3)에 `criticality` 차원 추가하되 *optional*.

## D-014 결정 루프 + 검색 표준화 — DECIDED (2026-06-11, 사용자)
**결정:** How를 찾는 운영 흐름을 7단계 결정 루프로 정본화(`08-decision-loop.md`): ①ckg 위치+영향 ②L0/L1/L3 도메인 별도주입 ③수정대상 결정 ④대상 재쿼리(과거히스토리+도메인) ⑤방법 나열(best-of-N, 각 grounded) ⑥criticality 우선순위로 최종결정 ⑦설계→플랜→구현. **L0~L3은 ckg 기능이 아니라 서로 다른 채널**(L0/L1=ckv, L2=ckg impact 동반, L3=skill). **안정성 조건:** LLM은 *판단*, 각 단계 *검색*은 표준 retrieval policy로(같은 입력=같은 경로=출력안정, D4 해소 연장). = ANALYSIS→DESIGN의 내부 상세.

## D-015 결정화 규율 (determinization discipline) — DECIDED (2026-06-11, 사용자)
**결정:** 결정성을 만드는 *모든 지점*은 **하나의 일관된 원리**를 따른다: **"구조화된 입력에서 규칙으로 결정적 도출, LLM 자유생성 금지."** 적용 지점: ① `normalized_query` 구성(GLOSSARY §2) ② retrieval policy(08-decision-loop, 각 단계 검색) ③ 캐노니컬 선택(#7 §3.2). 제각각 임시방편 금지 — *한 곳에서 규율을 정의*하고 모든 지점에 동일 적용. = 출력 안정(#7)의 전제. (사용자: "결정적으로 만드는 원리가 일관되게 적용되어야 한다.")

## D-016 프로세스: 증분 기록 (defer 금지) — DECIDED (2026-06-11, 사용자)
**결정:** 합의·결정 사항은 *그때그때 파일(DECISIONS/docs/memory)에 즉시 기록*한다. "나중에 한 번에 reconciliation"으로 미루지 않는다 — 장기 세션에서 컨텍스트가 요약·삭제되면 보류 항목이 *유실*되기 때문. 드리프트는 *타이밍 지연*이 아니라 *정본(GLOSSARY/DECISIONS)으로 모으는 일관성*으로 관리.

## D-017 ⭐ validation-gated rollout 순서 — DECIDED (출처 06 §7)
구현은 *게이트 순서대로*, 각 게이트 음수면 그 위 투자 중단:
`Gate 0 배관`(실 bge-m3·cks 바이너리·chainbench JSON) → `Gate 1 토대`(역할별: ckg traceability 재확인 / ckv fuzzy "cat 이기나" 별도 측정, D-009) → `Gate 2 제약`(constraint-first가 재작업률↓) → `Gate 3 관점`(렌즈 known-violation recall) → `Gate 4 학습`(externalizer 세로 bench로 repeat-failure↓). **화려한 #2~#7보다 Gate 0·1이 절대 먼저.**

## D-018 externalizer 승격 게이트 + 오염방지 — DECIDED (출처 #4 §5·§6, GLOSSARY §7)
unverified 초안 → verified blocking 규칙 승격은 **세 조건 모두**: ①빈도(같은 부류 N회 재발) ②심각도(도메인 안전이면 임계↓) ③**사람 게이트**(자동 승격 금지). 오염방지: lesson은 **증거앵커 필수**(failing_test+diff+reviewer_quote), LLM 추론 root cause=저신뢰, `unverified`/`stale`=**warning-only(절대 blocking 아님)**, 미재발 draft=decay. 대부분은 규칙 아닌 *에피소드 반례*로 적재.

## D-019 knowledge_version bump 정책 — DECIDED (출처 #4 §8)
**verified+blocking 승격만** `knowledge_version`을 올린다(thrash 방지). 상승 시 evidence 캐시·과거 결론 재오픈하되, 재오픈은 **영향받는 영역(governed_by)으로 국한**, 주로 *in-flight* 작업 대상(merged 티켓 자동 재실행 아님 — 소급 재감사는 별도 옵트인).

## D-020 평가·드리프트 운영 — DECIDED (출처 06 C2/D1)
(C2) enforce.chainbench는 **영향면 걸린 불변식만 매 사이클** 실행, 전수는 merge 전 1회. (D1) anti-drift 임계는 LLM 자기판단이 아니라 **기계 신호**: 구현 diff가 `constraints.md`의 `impact_surface` 파일/심볼을 건드리면 *자동* RE-PLAN(경로 매칭은 결정적).

## D-021 지식 저장 위치 — DECIDED (출처 #3 §7, #4 §7)
L3 backstop = **SSoT `tier:L3` 부분집합에서 파생**(손으로 관리하는 11개 하드코딩 아님). 에피소드 메모리 = **기존 ckv `pr_*` chunk + ckg temporal 재사용**(신규 저장소 없음). 의미·절차 지식 = SSoT → ckv `rule`/`invariant`.

## D-022 (T-2) _ssot 씨앗 = 기존 policy.yaml 승격 — DECIDED (2026-06-11)
**결정:** 새 `_ssot/`를 짓지 않고 **기존 `code-knowledge-graph/policies/stablenet/policy.yaml`(36 entries; governs 앵커 wired = governed_by 111 edges, 단 content는 **0 verified = 전부 needs_verification**, 2026-06-16 정정)을 정본 _ssot 씨앗으로 승격·확장**. 이미 가진 것: `id`, `category`, 풍부한 `description`(불변식 statement+인라인 코드앵커+함정), **`governs:[심볼]` → governed_by 111 엣지(code↔지식 앵커가 이미 동작)**. 추가할 필드: `kind(invariant|rule)`, `severity`, `tier(L0~L3)`, `lens`, **`enforce:[{type,ref}]`**, `verification.status`, `scope:[path glob]`(ckv watch_out 경로앵커 흡수), `criticality`(optional). **Fan-out 컴파일(#3):** 1 소스 → ckg governed_by(이미) / ckv watch_out+domain-corpus / stablenet-invariants L3 스킬 / chainbench invariant→test. 현재 3개 사본(ckg policy.yaml, ckv stablenet.yaml, domain-corpus 36 md)을 *1 소스 → 3 컴파일물*로 통합. 가장 싸고 governed_by 재사용. (근거: IMPLEMENTATION-STATE §4 T-2)

## D-023a (1차 제안, D-003 개정) ID 스킴 — ~~PENDING~~ **SUPERSEDED** (아래 D-023 DECIDED로 대체, 2026-06-12)
> ⚠ 이 항목은 *기록 보존용*. 정본은 바로 아래 "D-023 (개정) … DECIDED". (중복 D-023 헤더 충돌 해소.)
**제안(폐기됨):** 기존 A-taxonomy id(`A14.foundations.equal_power`, category.area.slug)를 *정본 키로 유지*(이미 111 governed_by + 36 corpus + 그래프에 wired). `kind`/`lens`/`tier` 필드 추가로 INV/RULE 구분. **D-003의 `INV-<DOMAIN>-<SLUG>` 폐기**(진공 설계 → wired 현실에 양보). cherry-pick = `A14.foundations.cherry_pick_principle` + `kind: rule`. ← 사용자가 (A 유지 / B 마이그레이션) 확정 시 DECIDED 전환. **→ B(3단 통일)로 확정됨.**

## D-023 (개정) ID 스킴 = 3단 통일 — DECIDED (2026-06-12, 사용자)
**ID = `<category>.<area>.<slug>` 항상 3단**(일관성 위해, 단일-area는 category=area 약한 중복 감수). D-003의 `INV-/RULE-` 폐기 확정. kind/lens/severity/tier는 *필드*로(직교). 단 *category 분류 자체*는 D-024로 재검토(아래).

## D-024 카테고리 기준 = 코드 모듈 (knowledge-topic 아님) — DECIDED (2026-06-12, 사용자 승인)
**발견(데이터):** ckg A1~A14는 *36개 지식노트의 토픽*이고 consensus 편중(14중 5 카테고리, ~20/36 entries). 거대 코드영역에 지식 거의 없음 — EVM(core/vm 7.8K), txpool(6.7K), state/trie(~19K), crypto(12.6K), accounts(15.6K), p2p(19K). 즉 *지식 ~80% consensus vs 코드 ~75% 비-consensus*.
**이미 존재:** ckv `policy/stablenet.yaml`에 코드근거 ~19 모듈 분류(consensus/beacon/state/crypto/p2p/txpool/rpc/systemcontracts/params/miner/evm/rawdb/types/rlp/accounts/tracers/cli). A1~A14보다 코드구조에 부합, 빈 곳 덮음.
**오분류 확증:** A12.seals=BLS seal(→crypto/consensus)+feepayer_sighash(→**fees**, 오배치); A13.sealing=txpool reorg직렬화(→**txpool**, 이름 혼동).
**제안:** 정본 category = **코드 모듈**(ckv식 ~19, 정제). 36 지식entries를 모듈에 재매핑. 빈 모듈(EVM/txpool/state…) = curation·criticality 우선순위. `category-ontology.md`의 A1~A14 rename은 이걸로 대체. ← 사용자 확정 시 DECIDED.

**군집 검증(2026-06-12, 데이터):** ckg `graph.db` 클러스터링 추출 결과 — `topic_tree`(Leiden)는 해상도 0~2 모두 ~20k 단일톤으로 **쓸모없음**; `pkg_tree`는 디렉토리 복원(24 top, core/eth blob, core를 file_path로 펼치면 vm/txpool/state/types/rawdb 확인). **ckg-pkg(펼침) ≈ code-loc 리뷰 ≈ ckv path-taxonomy 수렴**, A1~A14만 outlier. **도출 정본 모듈(~20):** evm·txpool·state·types·rawdb·blockchain · consensus·beacon · p2p·eth-protocol·rpc·node-infra · gasprice·tracers·miner · crypto·accounts·rlp · systemcontracts·params · cli. EVM/txpool/gas/types가 1급 카테고리화.

## D-025 (T-2 완결) _ssot 스키마 확정 — DECIDED (2026-06-12)
`09-ssot-schema.md`로 _ssot 스키마 확정. `_ssot/`={modules.yaml, lenses.yaml, knowledge/<module>.yaml, criticality.yaml, retrieval-policy.yaml, schema/, VERSION}. 엔트리 필드: id(`<module>.<area>.<slug>`)·kind(invariant/rule/doc)·module·lens·statement·severity·tier(L0~L3)·code_anchors(governs 재사용+body_sha)·scope·enforce[{type,ref}]·verification·criticality. 컴파일 fan-out: code_anchors→ckg governed_by, scope+statement→ckv watch_out/corpus, tier:L3→L3스킬, enforce→chainbench catalog, 전체→constraints.md. knowledge_version=hash(entries+modules+criticality+corpus), retrieval_policy_version 별도. 마이그레이션: policy.yaml governs/description 확장 재사용(D-022). → **T-2(SSoT) 완결.**

## D-026 (T-1 해소) decision-loop = 선택 가능 모드 (충돌 아님) — DECIDED (2026-06-12, 사용자)
pre-emptive(08 7단계)와 feedback-gated(기존 planner)를 *충돌로 두지 않고* **두 모드로** 둔다. `decision_strategy: pre-emptive | feedback-gated | adaptive`. **둘 다 동일 산출물 → 하류 모드-블라인드**(bench A/B/C 구조와 동일). 어느 쪽이 효율적인지 *가정 말고 측정* — bench를 전략 축으로 확장, 지표 `token-to-correct`·`bug-cycle 수`. 결과는 context-dependent 가능(consensus=pre-emptive, 단순=feedback-gated). **adaptive:** 모드 선택을 retrieval-policy(D-014)가 module·criticality로 결정(`dominant=safety`→pre-emptive 등), 임계는 측정으로. (사용자: "충돌로 남기지 말고 서로 다른 기능으로, 실험 필요.")

## 남은 표현 정리 (F10~F15)
GLOSSARY/README 상태헤더로 해소(별도 본문 재작성 안 함): F10 "11개"→"tier:L3 부분집합", F11 impact_map↔impact_surface, F12 해시 2종 분리, F13 렌즈 6종, F14 시퀀스 stale 주석, F15 normalized_query.
