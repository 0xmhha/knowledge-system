# 온톨로지의 영향과 MCP 노출 검토

> 이전 단계 기록이다. 아래의 proposed/대기/완료 수는 해당 검토·실측 시점의 상태다. 현재 작업 상태와 최종 한계 판정은 [O04 최종 판정](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md), [현재 인수인계](./CURRENT-HANDOFF.md)를 따른다. 과거 원자료와 수치는 변경하지 않았다.

2026-10-13 KST · 검토 기준 `0b852cf6`, 브랜치 `feat/setup-tool`. **검토 완료이며 새 기능 구현/운영 활성화는 수행하지 않음.** 기존 후속 원장17/18·잔여N16을 변경하지 않았다. 아래 보완 작업은 새 검토 결과이며 기존18개 완료율에 합산하지 않는다.

## 결론

온톨로지는 공유 개념·용어·관계와 원문/버전/검토 상태를 명시하고, 코드·요구·기준·시험을 연결하는 기능이다. 검색에서는 검토된 관계/개념 텍스트를 제한적인 재순위 신호로 쓴다. **MCP 내부 연결과 응답 필드는 이미 있지만, 독립적인 개념/관계/요구 추적 조회 도구 및 운영 연결 흐름이 부족하다.** 지금 생성된 go-stablenet MCP 설정은 의미 저장소가 비어 있고 모드도 지정되지 않아 온톨로지를 사용하지 않는다. 프로세스의 실제 로딩 설정/응답을 이번 검토에서 직접 호출하지는 않았다.

## 사용자에게 줄 수 있는 영향

예를 들어 “다른 버전의 원문을 사용하면 왜 승격을 막아야 하나?”라는 질문에 다음 정보를 제공하는 것이 목적이다.

- 기본 CKV/CKG: 관련 원문·함수·호출/이력·시험 후보를 검색한다.
- 검토된 의미 관계: ‘snapshot-bound-semantic-relations’라는 개념에 연결된 요구·Given/When/Then 기준·실제 code anchor gate·해당 시험을 함께 제시한다.
- 같은 source/project/dataset의 원문 SHA와 검토 상태를 보여 주며, 검토 경로/시험 실행/criterion 수용을 서로 구분한다.

이 예시는 knowledge-system의 파일럿 개념이다. go-stablenet의 트랜잭션·합의·검증 도메인 개념과 구현 관계가 같은 방식으로 준비됐다는 뜻은 아니다. 다른 프로젝트의 파일럿 semantic DB를 경로만 바꿔 붙이면 신원 불일치로 거부해야 한다.

| 기능 | 기대 영향 | 현재 구현 범위/한계 |
|---|---|---|
| 공유 개념/다국어 용어 | 같은 의미의 표현을 연결하고 모호한 후보를 드러냄 | semantic matcher/store와 CLI lookup-term 존재; 전용 MCP 조회 없음 |
| 구현 관계 재순위 | 검토된 개념→구현이 있는 검색 후보를 우선 배치 | 기존 capped 후보만 재순위, score boost 상한20%; 검색에서 놓친 코드를 새로 넣지 않음 |
| 개념 텍스트 재검색 | 정의·별칭 텍스트로 추가 CKV 신호 확보 | 동일 model/index로 검색, 기존 capped 후보에만 점수 적용; 추가 모델 호출·지연 비용 |
| 요구→기준→code→test 추적 | 구현의 목적과 해당 검증 범위를 설명 | 생산 semantic/CLI trace 존재, v2 knowledge 경로의 제한된 trace 첨부 가능 |
| 검토/버전 경계 | 미검토·오래된·다른 snapshot 관계를 근거로 쓰는 오류 방지 | unavailable/stale/budget_exceeded 등에서 baseline 반환, 자동 verified/accepted 판정 없음 |

semantic_search는 벡터 의미 검색이며 그 자체가 온톨로지 질의는 아니다. `expand=true`는 프로젝트 glossary의 concept→symbol 키워드 확장이다. 이것과 검토된 semantic IMPLEMENTED_BY/TESTED_BY/CHECKED_BY 관계는 별도다. 프로젝트 pack의 domain/flow/CKG governance policy 입력만 붙였다고 semantic projection까지 연결됐다고 볼 수 없다.

## 코드로 확인한 실제 MCP 노출

| 경로 | 현재 상태 | 근거 |
|---|---|---|
| get_for_task/get_for_task_v2 내부 재순위 | 존재, 서버 설정 semantic.ontology_mode로 opt-in | [provider 설정](../../cmd/cks/mcpcli/ontology.go#L25), [MCP startup 연결](../../cmd/cks/mcpcli/serve.go#L267) |
| get_for_task_v2 온톨로지 진단 | metadata.ontology에 mode/state·매칭수·관계수·가중 인용수·text 호출/원문 정보 첨부 | [응답 경계](../../internal/system/mcp/get_for_task_v2.go#L97), [DTO](../../pkg/system/contract/ontology.go#L5) |
| 검토된 정책/결정·외부 trace 첨부 | include_knowledge=true와 날짜/하위시스템 필요. 정렬된 semantic store·retained lock/instances·검토된 trace link 필요 | [knowledge 경로](../../internal/system/mcp/get_for_task_v2.go#L105), [trace 조건](../../internal/system/evidencev2/knowledge_trace.go#L22) |
| 개념 목록·용어 조회·관계 탐색·임의 requirement trace | 독립 MCP 도구 없음 | [등록24도구](../../internal/system/mcp/server.go#L134), [schema 계약](../../internal/system/mcp/testdata/agent-mcp.schema.json) |
| 요청별 ontology mode/설명 선택 | get_for_task_v2 입력에 없음 | [입력 schema](../../internal/system/mcp/get_for_task_v2.go#L20) |
| health의 온톨로지 사용 가능 여부 | ontology mode/semantic store·aligned projection 상태 없음 | [health DTO](../../internal/system/mcp/health.go#L24) |
| review_function의 개념/요구/기준 설명 | code/history/tests 구조 뷰만 있고 semantic 요구 관계 뷰 없음 | [응답 DTO](../../internal/system/mcp/review.go#L63) |

기존 tool description에는 온톨로지 발견/사용 조건 안내가 부족하다. 특히 metadata의 state=active는 arm이 실행됐다는 상태이며, applied_relations/boosted_citations가0이면 관계가 검색에 영향을 줬다는 증거가 아니다. concept 후보 전체·관계/기준 설명을 v2 진단 카운터만으로 보여 줄 수 없다. include_knowledge를 켜는 것이 ontology mode를 켜는 동작도 아니다.

## 운영 연결에서 발견한 빈틈

두 생성 설정(go-stablenet-cks.yaml/stdio.yaml)의 semantic block은 `store_path: ""`이고 ontology_mode는 없다. 별도 [관측 필드](../../system/eval/b0-knowledge-system/ontology-mcp-exposure-review-2026-10-13/observed-config-fields.json)에 필요한 필드와 파일 SHA만 기록했다. 사용자 credential/다른 설정 내용은 저장하지 않았다.

`cks mcp gen-config`에는 `--semantic-store`가 있지만 ontology mode/budget 생성 옵션은 없다. [Generate](../../internal/system/config/generate.go#L173)는 StorePath만 설정한다. [ks-setup](../../tools/setup/ks_setup/pipeline.py#L80)은 semantic store/mode를 gen-config에 넘기는 동작과 프로젝트 ontology/spec→검토된 projection 생성/결합 단계를 제공하지 않는다. 따라서 DB build·health·review_function·get_for_task_v2 확인이 성공해도 온톨로지 연결 성공 여부는 확인하지 못한다.

최소 활성 조건은 같은 프로젝트/동일 dataset/snapshot의 ontology/spec/근거 입력, 검토된 의미 projection, 실제 CKG canonical·CKV link 검증, 저장소 경로, explicit mode다. YAML mode만 켜고 다른 corpus의 held DB를 빌리거나 정의파일을 domain corpus로만 색인해 같은 기능이라고 표시하지 않는다.

## 효과가 보이지 않은 이유를 평가 원자료로 확인

[FINAL2 평가](./FINAL2-RESULT.md)의 기존2400행을 읽기만 해 진단을 집계했다. 새 FINAL 실행/예약/튜닝은 없다. 각 ontology arm의300회 응답은 반복/언어/phase를 포함하며 독립300표본이 아니다. _on/_off 접미사는 knowledge 요청 차이이고 ontology 활성/비활성은 baseline/concept_text/relations/combined arm으로 구별한다.

- 각 ontology arm300행 중240행이 no_match,60행은 active.
- relations 두 arm에서 **applied_relations0/boosted_citations0가300/300**. 개념 매칭이 있어도 실제 capped CKV 후보와 정렬된 검토 구현이 만나 점수에 기여한 관계가 없었다.
- concept_text/combined 네 arm은 각각30행에서4인용이 가중됐고270행에서는 가중0. 관계 기여는 전부0.
- 기존 결과에서 ontology arms 간 품질 개선은 확인되지 않았다. 이를 온톨로지 기능이 보편적으로 쓸모없다는 결론으로 확대할 수 없다. 실제 도메인 자료/매칭/관계 coverage·후보 중복·본문 budget 단계를 별도 측정해야 한다.
- FINAL2의 MRR −0.032는 **N08 타입 선언 삽입**의 결과다. 온톨로지 자체의 회귀 수치로 쓰지 않는다.

[진단 집계](../../system/eval/b0-knowledge-system/ontology-mcp-exposure-review-2026-10-13/historical-final2-ontology-diagnostics.json). 검색 재순위는 원래 capped 후보 집합을 유지하므로 그 단계에서 recall 집합을 넓히는 개선은 기대할 수 없다. 바뀐 순서가 downstream body budget 선택/최종 응답에 영향을 줄 수 있는지는 최종 pack까지 대조해야 한다. 시간·개념/관계 작성/검토·동일 버전 유지 비용도 포함된다.

## 권고하는 다음 설계와 수용 기준

다음 이름은 제안이며 현재 등록된 tool 이름이 아니다. 쓰기/승격 권한을 MCP 조회 권한과 합치지 않는다.

1. **온톨로지 capability/status 노출.** `cks.semantic.status` 또는 ops.health의 선택형 field에 mode, projected dataset/snapshot, store configured/aligned, 개념/verified 관계 수, 사용 불가 이유를 path-safe 상태로 제공. ‘기능 off’와 ‘자료 없음’·stale·unsupported 구분.
2. **읽기 도구.** `cks.semantic.lookup_term`, `cks.semantic.get_concept`, `cks.semantic.trace_requirement` 등을 aligned retained API로 제공. 다의어 후보/정의/포함·제외/검토자/원문과 IMPLEMENTED_BY·TESTED_BY·CHECKED_BY 경로·미연결/충돌 이유를 명시. 정책 인스턴스 첨부 없이 requirement 조회가 가능하도록 독립시키되 접근 범위를 유지.
3. **운영 준비 흐름.** ks-setup/gen-config에 명시적 ontology/spec/store/mode/budget 경로를 설계하고, 후보 dataset과 projection 신원이 일치해야 ready. 자동 검토 승인/자동 semantic activation은 하지 않음. health·조회 smoke에 unavailable·stale·미검토/다른 source를 포함.
4. **효과 설명과 평가.** 기존 get_for_task_v2 description/schema와 diagnostic에 개념/관계에 의해 어떤 기존 후보가 어떻게 바뀌었는지 설명. 필요 시 요청별 허용 arm 선택을 추가하되 공유 설정을 mutate하지 않음. 먼저 실제 go-stablenet 도메인의 소수 개념/요구/테스트 경로를 검토하고 동일 질문으로 off/on 원문·인용·순위·본문·지연을 비교. 관측된 FINAL2는 신규 독립 FINAL로 재사용하지 않음.

수용 시험: tools/list 실제 등록+input/output schema, retained source/canonical/span SHA, 실제 의미 자료의 positive 경로, missing/stale/unreviewed/ambiguous/budget fallback, off/on baseline 보존, request 격리, 진단 무결성·출처/권한, 구조 caller와 의미 TESTED_BY 구분. 등록 개수만 늘리거나 status=active만으로 기능 완료라고 판정하지 않는다.

## 이번 검증과 상태

관련 기존3패키지 `stage2/mcp/mcpcli` race **19시험/39하위시험/3패키지 pass, fail0/skip0**. 내용: 원문 검색 이후 bounded rerank, text/relations 구별, unverified/missing/stale/time budget fallback, pinned provider, MCP v2 진단 무결성/knowledge 조합, tools/input 계약 등록. fixture 시험이며 live model·운영 서버·새 실제 domain projection 수용은 미실행이다. [시험 요약](../../system/eval/b0-knowledge-system/ontology-mcp-exposure-review-2026-10-13/related-tests-summary.json).

**현재 단계:** 온톨로지 영향/MCP 노출·운영 연결 검토 완료. **다음:** 위1의 사용 가능 상태와2의 읽기 MCP 도구 설계, 이어3의 실제 프로젝트 자료 결합·4의 설명/독립 평가. **기존18개 남은 전체:** N16 공개키 fingerprint/서명 신뢰 경로(사용자 결정으로 미충족 유지). 이번 권고4단계는 추가 보완 제안이며 기존 완료를 자동 취소하거나 새 구현 완료로 표시하지 않는다.
