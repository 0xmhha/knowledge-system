# N12 개념 경로 5개: 실제 범위 대조와 명세 수정안

2026-10-08 · **범위 대조·로컬 통합 검증 완료 / 명세 수정안 미적용 / N12·N14 전체 미완료**.

[5개 검토 항목](../../system/eval/b0-knowledge-system/refactoring-n12-concept-gap-audit-2026-10-08/gap-review-items.json) · [원문18참조](../../system/eval/b0-knowledge-system/refactoring-n12-concept-gap-audit-2026-10-08/source-refs.json) · [통합 실행](../../system/eval/b0-knowledge-system/refactoring-n12-concept-gap-audit-2026-10-08/integration-report.json) · [관련 시험](../../system/eval/b0-knowledge-system/refactoring-n12-concept-gap-audit-2026-10-08/focused-tests-summary.json).

목적은 빈 trace 경로를 채우는 것이 아니라, 요구한 동작을 실제로 구현하고 시험하는 관계를 만들고 사람이 그 의미를 검토할 수 있게 하는 것이다. 이전 P12-T01–13은 3개 broad concept의 부분 구현 해석도 검토 대상으로 남겼다. 이번 대조에서 나머지5개는 원문 부족만의 문제가 아니라 **개념·기능·시험 범위가 서로 다른 문제**임을 확인했다.

## 확인한 차이와 남은 기능

| 검토 ID | 개념/요구 | 실제 코드·시험의 범위 | 아직 연결할 수 없는 범위 |
|---|---|---|---|
| P12-G01 | policy / filtered recall | `types.Filter`는 metadata 조건을 AND 결합한다. 검색 함수와 sparse 검색 시험은 eligible K/거리/불완전 상태를 다룬다 | 버전·담당자·권위·검토·공개 통제 정책의 전체 구현이 아니다. policy를 SearchDetailed에 추가 연결하지 않음 |
| P12-G02 | source-snapshot / source evidence | SourceIdentity/sourceSnapshotID는 원문·정책의 신원을 표현/계산한다. capture 시험은 working-tree/non-Git 원문 보관을, alignment는 색인 신원 일치를 다룬다 | 표현·캡처 lifecycle과 의미 관계의 승격 거부는 별도 범위. 타입 또는 해시 함수의 시험만으로 criterion 전체를 수용할 수 없음 |
| P12-G03 | semantic-assertion / source evidence | Assertion은 방향·타입·증거·검토 상태를 표현한다. Projection.Validate는 endpoint/proof/review 조건을 검사하며 alignment에서 호출한다 | alignment가 모든 의미 관계를 구현한다는 주장 또는 fixture의 reviewer를 실제 사람 판정으로 간주할 수 없음 |
| P12-G04 | term / ontology fallback | Term은 언어·표현·preferred를 저장하고 MatchConcepts는 실제 용어 매칭·동률을 처리한다. 기존 fallback 시험은 미리 정한 후보를 반환하는 fake matcher를 쓴다 | fake fallback 시험을 실제 matcher 시험으로 연결하지 않음. 같은 alias의 ambiguity에 대한 실제 파일럿 통합 범위는 아직 없음 |
| P12-G05 | evidence-pack / ontology fallback | stage2는 scored citations를 보존한다. BuildFromRefs는 원문을 읽고 예산·정제·무결성을 적용한다. EvidencePackV2는 응답 타입이다 | stage2와 전체 근거 묶음 구현은 별도. 채워진 graph neighbors·reviewed overlay·실제 hybrid 검색 및 사람의 criterion 수용은 이번 실행에 없음 |

원문18범위를 Git `32d7e2641640b82995cf443bd8a2553dd0ce69b9`, 보관된 실제 source와 현재 바이트에 대조했다. snapshot `a0457cc1…`, dataset `5a24904f…`는 이전 자료와 동일하다. **7개 참조 파일은 retained source에는 있지만 기존 선택 CKG/CKV 입력에 없다.** filter/validate/matcher/matcher_test/fake matcher test/pack secret test/trace가 해당하며, 이를 새 canonical 연결이 검증된 것으로 기록하지 않는다. 타입·행·SHA 증명도 관계 의미의 승인은 아니다.

## 실제로 추가 검증한 동작

기존 proposed SQLite를 read-only로 열어 `LoadAlignedRetained`로 실제 source/index에 정렬된 resolver를 얻고, 생산 MatchConcepts → stage2.Search → retained V2 Build/정제/Stamp/Verify 경로를 실행했다.

- 등록되지 않은 입력, `소스 스냅샷`, `소스 스냅샷 소스 파일`의 3사례에서 실제 matcher의 후보0/1/2개를 확인했다. 모든 후보는 proposed이며 boost 관계0, baseline의 scored citations3개는 동일하다.
- 각 baseline/optional에서 생성한 근거 묶음의 좌표·인용·정제 본문3개·빈 graph neighbors·evidence state가 같다. ontology 진단을 추가하면 무결성 hash는 달라지며, 진단/본문 변조6개는 거부된다. 서로 다른 diagnostic까지 포함한 전체 JSON이 같다는 주장과 구분한다.
- CKG backend는 `ckgclient.Fake`, CKV 입력은 실제 canonical 원문에 놓인 제어용 인용3개다. **실제 hybrid 검색 품질, 공유 alias ambiguity 또는 전체 graph 확장을 검증한 실행이 아니다.** multi-term2개와 같은 용어의 동률은 별도다.
- 기존 관련 시험5패키지·17개·하위32개를 `-count=1`로 다시 실행해 모두 통과했다. 이 중 별도 합성 fixture는 공유 alias ambiguity, typed review/proof, capture 및 secret drop을 검사한다. 전체 Go 회귀·semantic linked test report·실제 사용자 검토로 집계하지 않는다.
- 기존 권위 파일9개 SHA는 전후 동일하며, semantic current가 없고3명세는 `spec_unapproved/paths=[]`다. 원 projection/관계/명세/ontology/lock을 바꾸지 않았다.

offline stage2 Search6회/V2 Build6회는 위 통합 제어 실행 횟수다. 모델 HTTP·DEV/FINAL matrix dispatch·새 독립 FINAL·활성화·patch·실제 사람 판정은0, 사람 활동 시간은null이다.

## 검토할 구체적 명세 선택

현재 Trace는 `Requirement.ConceptIDs`의 **모든 개념**에 reviewed 구현과 시험 경로를 요구한다. 문맥용 개념을 자동 생략하는 필드나 기능은 없다. 따라서 미완성을 숨기려고 현재5개를 삭제하거나 Trace의 검사를 완화하지 않았다.

1. **현재 범위 유지:** 원래8개 broad concept 경로를 모두 유지한다. 정책/캡처/관계/용어/근거 묶음의 각각 실제 기능·시험·canonical 경로를 보강하고 부분 구현 해석을 사람이 검토한다. 기존 미작성5개는 남는다.
2. **기능 범위의 파일럿 명세로 수정(제안):** 요구마다 좁은 행동 개념1개를 정의하고 requirement version2로 검토한다. 원래20개 공통 vocabulary는 보존하며 broader 개념은 이 수정안의 문맥으로 기록한다. 기존 P12-T01/T08/T11도 새 개념과의 정확한 관계로 다시 검토하고 승인을 승계하지 않는다.

| 요구 ID | 제안하는 기능 개념 | 후보 구현 |
|---|---|---|
| req-filtered-recall | filtered-nearest-chunk-search | SearchDetailed |
| req-source-evidence | snapshot-bound-semantic-relations | ValidateDatasetAlignment와 검증 delegation의 명시적 범위 |
| req-ontology-fallback | optional-ontology-baseline-preservation | applyProvidedOntology와 실제 matcher/fallback 시험의 분리 |

검토 가능한 [기능 vocabulary 초안](../../system/eval/b0-knowledge-system/refactoring-n12-concept-gap-audit-2026-10-08/functional-ontology-option.yaml)과 [version2 명세 초안](../../system/eval/b0-knowledge-system/refactoring-n12-concept-gap-audit-2026-10-08/functional-spec-option.yaml)을 작성했다. YAML 구조·개념 참조를 확인했고 원래 statement/4criteria 값은 그대로다. **초안은 production 파일, 기존 retained source·projection·lock에 미적용이며 내용·관계 승인도 없다.** 원문 결합 validator를 통과한 새 source-backed projection이라고 주장하지 않는다. 문맥 개념 필드를 제품 schema에 추가한 것도 아니다.

기능 범위 수정안을 제안하는 이유는 파일럿의 세 행동과 구현·시험의 책임을 직접 대조할 수 있기 때문이다. 더 좁은 trace라도 broader 정책·캡처·근거 묶음의 실제 기능과 수용 작업은 해당 N12/N14/N16 요구에서 별도로 검토해야 한다. 어떤 선택도 시험 pass를 실제 criterion accepted로 바꾸지 않는다.

## 체크와 다음 순서

- [x] 기존5개 누락 경로와 실제 원문·시험 범위 대조.
- [x] 실제 retained matcher→stage2→V2 경로3사례 및 무결성 거부6제어.
- [x] 관련 기존17시험 실행, 기존 권위9파일 불변, proposed/current 없는 상태 확인.
- [x] 두 범위 선택과 미적용 version2 명세/vocabulary 초안 작성·구조 대조.
- [ ] 선택한 명세 범위·개념·정확 관계의 실제 검토자/판정 기록.
- [ ] 선택 결과를 새 source capture/색인/projection에 결합; 같은 source/approval을 가장하지 않음.
- [ ] 실제 semantic linked test 및 사람 criterion decision, patch/승격·반려·rollback 수행.

**전체8/18·요청5/15 완료, 남은10개 유지.** 새 관계0·새 reviewed 경로0·기존 미작성 경로5개 유지이며 N12-A–D/N14-A–E는 미체크다. 다음은 명세 범위 선택과 N11 gold/독립성·N12 개념/관계 판정 반영이다. 남은 전체는 N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 실제 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 최종 기본값·출시 판정이다.
