# study 자료로 보완한 최초 목적·상위 작업계획 감사

2026-10-06 · 제품 기준 `700397b1`(제품 코드는 `04d3bfdf`와 동일), study 기준 `31ce3adb0ff210c740823a7d55601edcbc699c68`. 파일 내용과 보유 Git 이력을 확인한 문서 감사다. 제품 실행·모델 질의·외부 저장소 구현 검증은 추가로 수행하지 않았다.

**study에서 실제 목적 문서와 마스터 작업계획을 추가 확보했다. 따라서 9월 spec-driven의 잔여18개만으로 전체 시니어 개발자 모방 시스템의 잔여를 설명할 수 없다.** 다만 6월 상위 계획과 9월 요구사항/WBS는 서로 다른 범위다. 이번 보완은 기존18개를 유지하고 상위 WI10개를 원래 ID로 별도 추적한다. 둘을 더한28개를 동일 범위의 미완료 수로 표시하지 않는다.

## 1. 발견한 자료와 시점

| 자료 | 원문·시점 | 쓰임 |
|---|---|---|
| 최초 구상 | [Knowledge System 제안서](../../../../../study/docs/learn/ai/code-intelligence/MCP_KNOWLEDGE_SYSTEM_PROPOSAL.md), 본문2026-03-18 | 코드·문서/PR·런타임 지식을 연결하는 초기 비전. 서버 내부 LLM/동적 수집 제안은 현행 필수 계약으로 자동 채택하지 않음 |
| 목적 원문 | [00-purpose](../../../../../study/projects/ai-knowledge-data/docs/00-purpose.md), 본문2026-06-11, Git 추가 `d5f8061`(06-12) | 사람이 What을 주면 모듈별 우선순위·불변식·규칙으로 How를 정당화하고 사람 리뷰로 지식 개선 |
| 통합 설계/확정 결정 | [architecture](../../../../../study/projects/ai-knowledge-data/docs/knowledge-system-architecture.md), [README](../../../../../study/projects/ai-knowledge-data/docs/README.md), [DECISIONS](../../../../../study/projects/ai-knowledge-data/docs/DECISIONS.md) | 7개 저장소의 역할·검증 게이트·SSoT·제약 산출물·다관점 검토. README/GLOSSARY/DECISIONS의 정정이 오래된 본문보다 우선 |
| 개선 제안 | [07-cks-codingagent-improvements](../../../../../study/projects/coding-agent/07-cks-codingagent-improvements.md), 본문2026-06-25 | dataflow/CFG, 규칙 태깅, 의심 라벨링, 비결정 버그, 학습, 다단 확장의 최초 상세 갭 |
| 사용자 목적의 구체화 | [00_background_goal](../../../../../study/projects/coding-agent/00_background_goal.md) | 의미 추론→재현/진단→수정→검증·PR, 코드 리뷰·신규 설계를 포함한13개 사용자 비전 |
| **마스터 작업계획** | [12-master-workplan](../../../../../study/projects/coding-agent/12-master-workplan.md), 본문2026-06-26, 최초 Git `718cb8a`(06-26) | “이후 작업의 단일 기준”; 버그수정→코드리뷰→신규기능 보류, WI10개·계층별 책임·완료 조건 |
| 학습 범위 개정 | [10-learning-direction](../../../../../study/projects/coding-agent/10-learning-direction.md), 본문2026-06-25 | 별도 전용 MCP/별도 플러그인·관리자 명령·사람 게이트. PR 재색인과 암묵지 학습을 구별; 당시 지금 구현하지 않음 |
| taxonomy/지식 검증 | [category-ontology](../../../../../study/projects/ai-knowledge-data/docs/category-ontology.md), [IMPLEMENTATION-STATE](../../../../../study/projects/ai-knowledge-data/docs/IMPLEMENTATION-STATE.md) | 6월 코드모듈 분류와 당시 구현 진단. 현행 실측/내용 승인을 대체하지 않음 |
| 별도 초기 MVP | [Phase0/1-α 계획](../../../../../study/projects/stablenet-ai-agent/claudedocs/plan-phase0-phase1-cks-mvp.md) | study 안의 별도 Go 모듈/CKS MVP 설계. 현행 배포 저장소의 9월 WBS로 합산하지 않음 |

study의08-21 재정리 커밋 `a944be9`에서 study 내부 경로 “docs/research/coding-agent/12-master-workplan.md”가 현재 “projects/coding-agent/”로 이동했다. Git의 follow 이력으로 최초 추가까지 추적했다. 전체 보유 참조의 파일 이력에서도 9월에 참조한 `spec-driven-requirements.md`, `spec-driven-wbs.md`, `ckv-ckg-ontology-installation-proposal.md`는 찾지 못했다. **상위 목적/계획 복구는 진전됐지만 N-01의 9월 원문 복구는 아직 완료가 아니다.** 원격 fetch는 수행하지 않았다.

검증 중 study HEAD가 `aa6d1034`로 이동했다(WBFT 비교 기록 추가). 아래13개 출처 파일은 최초 관찰한 `31ce3adb`의 바이트와 동일함을 각각 확인했다. 다른 작업의 새 커밋을 되돌리지 않고 감사 기준과 관찰된 최신 HEAD를 모두 증거에 기록했다.

## 2. 현재 제품 코드와 대조한 핵심 결과

| 목적/기능 | 현재 증거 | 판정·잔여 연결 |
|---|---|---|
| 실 BGE 신원·근거 출처·fail-loud | 기존 실모델 평가; `internal/system/mcp/alignment.go`, `source_guard.go`, `internal/setup/identity.go` | 6월의 FakeEmbedder 폴백/항상 silent-degrade 주장을 현행 상태로 복사하지 않음. 검색 품질·기권 실패는 N-08–11에 유지 |
| 호출/영향/동시성/변경 이력 탐색 | `internal/system/mcp/graph.go`, `analysis.go`, `concurrency.go`; 기본 depth2와 요청 depth; composer Stage3 | 구조 존재. 여러 hop의 호출 탐색이 값 흐름(def-use/PDG) 분석까지 구현했다는 뜻은 아님 |
| curated flow·증상→failure branch | `pkg/vector/ckv/ckv.go:200`, `internal/vector/query/flow.go:163/362/432`, `internal/system/ckvclient/flow.go:353` | **실제 메서드 구현 존재**. MCP 등록부의 “backend stub” 주석은 오래됨. WI-3의 도달성/최근변경/동시성 등 객관 신호를 합친 일반 코드 후보 라벨링의 완료 증거는 아님 |
| CFG/PDG/함수 간 값 전파 | Go parser·`pkg/graph/types/enums.go`, 의존성/비테스트 Go 소스 검색 | 현행 공개 그래프에 해당 값 흐름 레이어의 구현 근거를 찾지 못함. 필드명 접근·호출 엣지로 완료 표시하지 않음. WI-1/6은 별도 계약/품질 검증 대상 |
| Policy/SecurityPattern | `pkg/graph/policy/policy.go`, `pkg/graph/security/security.go`, graph buildpipe, enum의 `governed_by`/`has_security_pattern` | YAML 오버레이와 그래프 연결 존재. 자동 source→sink 룰 엔진이나 전용 security-pattern 질의의 완료를 뜻하지 않음. v2 정책/ADR 근거는 별도 구현됨 |
| 실제 도메인 지식 승인 | [현 inventory](../../projects/stablenet/domain-knowledge/inventory.md)는44개 중41 verified/3 needs_verification을 기록 | 이는 특정 코드 기준의 기존 도메인 inventory 상태다. study의 “36개 전부 verified”와 “0 verified”를 현재 전역 수치로 재사용하지 않음. 새 pilot20개/요구3개 승인과 동일하지 않으며 N-12는 유지 |
| 규범→기준→테스트→사람 수용 | semantic/knowledgepack·patch/review 장치와 preview 증거 | 장치는 구현. 실제 파일럿 수용은 N-12/14. ChainBench 불변식 전용 게이트나 coding-agent의 오라클 타당성/다중 수정안 선택까지 증명하지 않음 |
| What→How 일관성·다관점·학습 | study의 Gate2–4 및 WI-V1/V2/C/5 | 주 책임은 coding-agent/ChainBench/별도 학습 저장소. 이번 제품 감사만으로 완료/미구현 단정 불가. 외부 저장소의 현재 코드·실행 증거 재감사 필요 |

6월 ontology의 “코드모듈 약20개 taxonomy”와 9월의 “공통20개 concept”는 서로 다른 분류다. 수가 같다는 이유로 승인/내용/완료를 공유하지 않는다. 6월 제안의 Qwen3 권고도 이번 B0의 사용자 승인 BGE-M3를 바꾸지 않는다.

## 3. 복구한 상위 작업계획의 전체10개 항목

아래 상태는 최초 원문의 의사결정 상태를 보존한 **교차 저장소 추적표**다. “외부 미감사”는 미구현이라는 판정이 아니다. 당시 검토/보류/별도 범위를 구현 승인으로 확장하지 않는다.

| 원래 ID | 목표·담당 | 원문 상태 | 이번 확인/남은 검증 | 우선순위·기존 N 연결 |
|---|---|---|---|---|
| WI-V1 | 오라클 타당성·mutation/민감도/독립 재유도; coding-agent | 합의·계약 대기 | 외부 현재 구현 미감사. RED/GREEN만으로 충분한지 실제 false-done 차단 확인 | 상위 Tier1 첫 작업; N-14의 실제 수용과 연계 |
| WI-V2 | 수정 후보 생성·judge-panel 선택; coding-agent | 합의·계약 대기 | 외부 미감사. 같은 증상에 대한 후보별 비교·선택 증거 필요 | V1 다음; N-14와 연계 |
| WI-3 | 증상→코드 후보 추림·객관 신호 라벨링; coding-agent+CKS | 확정 | CKS 개별 탐색/flow 도구 존재, 일반 코드 후보 통합 라벨링 완료 근거 없음. LLM의 증상별 판단과 사실 신호 분리 계약 확인 | Tier1; 검색 기반 N-08/10·외부 워크플로 확인 |
| WI-4 | 비결정/타이밍 재현·방어·모니터링; coding-agent+ChainBench | 추가 고민 | 외부 미감사. 재현 불가/주입/관찰의 완료 조건과 실제 사례 필요 | Tier1 확장, 설계 대기 |
| WI-1 | dataflow/CFG/PDG; CKG | 추가 검토 | 제품 내 값 흐름 레이어 근거 미확보. 언어·on-demand/전체·정확도/비용 계약과 실측 필요 | Tier2 공통 기반; 검토 후 범위 확정 |
| WI-2 | LLM 비의존 규칙 태깅/취약점; CKG/CKS | 방식 재논의 | 수동 Policy/SecurityPattern 있음. source→sink 탐지와 작은 타입/룰 팩 효과 별도 확인 | Tier2, 취약점급은 WI-1 의존; N-12/13 연계 |
| WI-C | 시퀀스/모듈·data/state·가드·성능·우선순위 산출물; coding-agent | 더 설계 | 외부 미감사. 도구/다이어그램 존재와 깊은 리뷰 산출물의 수용은 구별 | Tier2, WI-1/2 의존 |
| WI-6 | 다단·dataflow-shaped 확장; CKS | 확정 | 호출 hop/curated flow 확장 있음. 값 의존성을 따라가는 확장과 비용/누락 검증 필요 | Tier2, WI-1 의존; N-08/10과 부분 연계 |
| WI-SP | security-pattern/policy MCP 노출; CKS+CKG | 추가 검토 | 저장/연결 존재. 전용 공개 질의·권한·출처 계약과 실제 반환 검증 필요; v2 정책 근거로 통째 완료 처리하지 않음 | 별도 하드닝, N-12/13 연계 |
| WI-5 | 암묵지 수집·관리자 학습; 별도 ingest MCP/플러그인 | 사용자 별도 repo 주도 | 외부 미감사·당시 지금 구현하지 않음. 사람 검토·sanitize·dedup·승격/회수·색인 소비 실제 루프 확인 | 별도 트랙; N-12/14의 관리 장치는 부분 기반 |

신규기능 워크플로(Tier3)는 Tier2 후로 보류돼 있다. 제약 assembler·다관점 reviewer·SSoT fan-out·retrieval policy·불변식 테스트 catalog·총비용 bench는 6월 ai-knowledge-data 설계의 별도 게이트 항목이며 WI와 1:1 동일 목록이 아니다. 원래 전체 목표를 다시 감사할 때 두 설계의 관계와 현재 담당 저장소를 대조해야 한다.

## 4. 후속 목록에 반영한 내용과 다음

기존 [N-01–18](./REFACTORING-REMAINING-WORKLIST.md)의 구현 완료0/잔여18개와 preview30/30 종료는 유지한다. N-01에 이번 상위 원문 확보와 9월 원문 미확보를 함께 기록했다. 상위 WI10개도 위 표로 전부 추적하되 제품18개와 중복되는 기반/담당/보류 상태를 먼저 확인한다.

**현재 단계:** study 추가 자료·Git 이동 이력 확인 및 제품 코드 대조 완료. **다음 문서 감사:** 상위 WI의 담당 저장소/현행 계약/실행 증거 대조(특히 WI-V1/V2/3). **다음 제품 구현 우선순위:** N-02→N-03→N-04. **남은 전체 제품 작업:** N-01–18. **상위 목표 추적:** WI-V1/V2/3/4/1/2/C/6/SP/5, 10개; 외부 미감사·부분·검토/보류를 구분한다.

출처 commit·SHA·검색 결과·원문 snapshot·코드 binding·검사 결과는 [study 보완 증거](../../system/eval/b0-knowledge-system/study-original-plan-followup-2026-10-06/source-and-code-bindings.json)에 보관한다. 기존 감사 증거는 당시 commit의 기록으로 유지하고 덮어쓰지 않는다.
