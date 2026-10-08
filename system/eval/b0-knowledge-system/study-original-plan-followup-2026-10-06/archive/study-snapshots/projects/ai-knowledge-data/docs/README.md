# LLM 코딩 지식 아키텍처 — 문서 인덱스

> go-stablenet auto-coding을 위한 지식 시스템의 설계·리뷰 문서 모음.
> **이 README가 진입점이다.** 전부 읽지 말고 아래 reading path를 따르라.
> 용어·키·ID가 헷갈리면 항상 **[GLOSSARY.md](GLOSSARY.md)가 정본**이다.
> 상태: **설계 완료 (DESIGN COMPLETE)** — v1 freeze(2026-06-11, tag `v1-design-freeze`) + post-freeze 갭분석·T-2(SSoT)·T-1(decision-loop 모드) 완료(2026-06-12, tag `v2-ssot-t2`). **다음은 구현(E): Gate 0 배관**(Ollama+bge-m3, chainbench JSON) → Gate 1 토대. 권위=README/GLOSSARY/DECISIONS, deep-dive 본문은 just-in-time 정리(D-012).

---

## 0. 30초 요약

> **북극성([00-purpose.md](00-purpose.md)): 인간은 "What"만 말하고, 시스템이 외부화된 암묵지(특히 *모듈마다 다른 우선순위*)로 "How"를 정당하게 *결정*하고, 같은 요청엔 같은 결과를 주며, 인간의 리뷰로 암묵지를 보완해 점점 나아지는 시스템 — 그 첫 만족스러운 버전.** (서울→부산: 비행기/KTX/버스 중 기준 무게로 고르듯)

시니어가 코드 외에 보는 4가지(정책·도메인·근본원인·영향도)를 LLM이 *회상*이 아니라 *근거*로 쓰게 만드는 시스템. 이론은 ~80% 이미 7개 저장소(ckg/ckv/cks/coding-agent/chainbench/go-stablenet/knowledge-data)에 구현돼 있고, 남은 격차가 사용자 통증(비일관·환각·멀티뷰포인트·재작업)을 만든다. 처방: **사실·잣대는 고정(노이즈 제거), 해법은 freeze 전 다관점 수렴 후 봉인, 틀리면 SSoT 환류+재설계.** 일관성은 캐시가 아니라 grounding+검증의 창발물 — 그리고 정확한 수렴 + 출력 안정을 *둘 다* 노린다(#7).

---

## 1. Reading Path (목적별)

- **"전체 그림만":** [README](README.md) → [GLOSSARY](GLOSSARY.md) → `knowledge-system-architecture.md` §0·§2·§3 (다이어그램).
- **"왜 이렇게 설계했나":** `llm-coding-knowledge-architecture.md`(이론) → `01`(결정성) → `07`(수렴+안정).
- **"구현하러 간다":** [DECISIONS](DECISIONS.md) → `03`(SSoT 스키마) → `02`(파이프라인) → `06 §7`(rollout 순서).
- **"이게 정말 되나 (회의적)":** `06`(반대심문) → `01`/`07`(반론) → §3 아래 정정 로그.

---

## 2. 문서 목록

### 설계 (spec)
| 문서 | 내용 | 상태 |
|---|---|---|
| **`00-purpose.md`** | **북극성 — What/How, 서울→부산, 목적(수단 아님)** | 불변 목적 |
| `llm-coding-knowledge-architecture.md` | 이론 — 암묵지 외부화, 4지식 CoALA 분류, 설계 강제 7기제 | RECONCILED* |
| `knowledge-system-architecture.md` | 7-repo 통합, 컴포넌트/시퀀스 다이어그램, 이론↔현실 격차 | RECONCILED* |
| `01-determinism-and-convergence.md` | 사실/규범 고정 vs 해법 탐색, 무효화 키 | RECONCILED |
| `02-constraint-first-and-perspective-review.md` | constraint-assembler + perspective-reviewer + design-freeze | RECONCILED |
| `03-rule-invariant-ssot.md` | SSoT 1벌 → 다중 컴파일, enforce 실행화 | RECONCILED* |
| `04-externalizer-seci-loop.md` | 실패→교훈 환류, 승격 게이트, 오염방지 | RECONCILED |
| `07-convergence-and-stability.md` | **정확한 수렴 + 출력 안정 둘 다** (A1 해소) | RECONCILED |
| `08-decision-loop.md` | **How를 찾는 7단계 결정 루프** (ANALYSIS→DESIGN 상세, 검색 표준화) | RECONCILED |

### 리뷰 (review record)
| 문서 | 내용 |
|---|---|
| `05-consistency-audit.md` | 문서 간 정합성 15건(F1~F15) |
| `06-adversarial-review.md` | 반대심문 — C1 정정, A1 해소(#7) 반영 |

### 구현 진단 + T-2 SSoT (post-freeze)
| 문서 | 역할 |
|---|---|
| `IMPLEMENTATION-STATE.md` | 설계 v1 ↔ 7-repo 현재 구현 갭 분석(2026-06-11). Gate 0 진단 입력 |
| `category-ontology.md` | 정본 코드모듈 taxonomy(~20) + 36 entries 재매핑 + 지식 갭(D-024) |
| `09-ssot-schema.md` | _ssot 스키마 확정(엔트리·criticality·retrieval-policy·fan-out, D-025) |

### 권위 (authority)
| 문서 | 역할 |
|---|---|
| **`GLOSSARY.md`** | 용어·키·ID·enforce·렌즈 **단일 권위** |
| **`DECISIONS.md`** | 확정 결정 로그(D-001~) |

`*` = 본문에 일부 stale 표현이 남았으나 상단 상태 헤더 + GLOSSARY로 정정됨(아래 §3).

---

## 3. 정정·supersedes 로그 (본문보다 이게 우선)

| 항목 | 어느 문서가 무엇을 | 정본 |
|---|---|---|
| **티어 어휘** | 이론 §4.7 "L1=always-on", #3 §7 → 폐기 | GLOSSARY §1 (축A CURATED/COMPUTED + 축B L0~L3) |
| **결정성** | 통합 §3·§5.3·§7 "하드 보장 / IntegrityHash로 보장" → 폐기 | `01` + GLOSSARY §2 (사실층만, 해시=센서) |
| **출력 안정** | A1이 "게이트≠안정" 지적 | `07` (게이트 위 3층 collapse + 종속 철칙) |
| **ckg < cat (C1)** | 06 원판정 🔴 "검색이 파일덤프에 패배" | **정정 🟡** — 그건 가짜임베더 파이프라인 측정(N=1, 폐기됨). ckg 자체 F1=0.96. "미검증"이지 "패배" 아님 |
| **ID 스킴** | D-003 `INV-<DOMAIN>-<SLUG>`/`RULE-<SLUG>` → **폐기**(D-023 개정, 2026-06-12) | `<category>.<area>.<slug>` 3단, category=코드모듈(D-024). GLOSSARY §3 / `category-ontology.md` / `09-ssot-schema.md` |
| **ID/enforce/렌즈** | 문서별 표기 드리프트(F5/F6/F13) | GLOSSARY §3·§4·§5 |
| **시퀀스 다이어그램** | 통합 §3 — #1·#2·#4 이전 프레이밍 잔존(F14) | `01/02/04` 확정본 + GLOSSARY §6 |

---

## 4. 다음 단계 (이상적 흐름)

`A 수렴(완료)` → `B reconciliation(완료)` → `C 리뷰(완료)` → `D freeze(완료 — v1 봉인)` → **`E 구현(현재 — 06 §7 rollout)`**.

- 리뷰는 이 README + GLOSSARY + DECISIONS만 봐도 충분(깊은 문서는 선택).
- freeze 시 `git init` 권장(baseline 태그/CHANGELOG로 "봉인"이 실의미를 가짐 — 현재 비-git).
- 구현은 **06 §7 validation-gated rollout**: Gate 0(배관) → Gate 1(토대, 역할별) → … 음수 게이트면 그 위 투자 중단.
