# N11/N12 선택한 범위와 남은 개별 판정

2026-10-08 · **권고 범위2개 선택 반영 / 검토자 이름·개별 판정 대기**.

사용자 직접 응답 “모두 권고 방향으로 진행”으로 아래 N11/N12 권고안을 선택했다. [적용·소스 검증](./N11-N12-SCOPE-ADOPTION.md)과 [선택 파일](./PILOT-SCOPE.json)을 참조한다. 과거 초안·held 후보는 보존했고 새 source-only 파일을 작성했다. 선택할 범위를 다시 묻지 않는다. 실제 검토자 이름은 아직 없고 개별 gold/관계 승인은 별도다.

현재 전체8/18·남은10개다. N11의20질문 책과 N12의 미적용 초안·관계 자료는 준비됐지만, 실제 내용·독립성 판정이 남아 있다. 범위 선택은 아래 개별 내용의 승인을 대신하지 않는다. 이전 B0·preview 승인이나 다음 진행 요청을 새 파일럿 전체 승인으로 기록하지 않는다.

| 결정 | 권고 선택 | 다른 선택과 영향 |
|---|---|---|
| N11 POLICY/TRACE 범주 | POLICY에 명시적 코드 입력·접근·버전 규칙을 포함하고, TRACE에 코드 추적 검증 계약과 실제 검토된 trace를 하위 유형으로 구분 | 조직 운영 정책/실제 검토된 ADR→요구→기준→코드·시험 경로만 평가한다면 운영 소유자·권위·시점과 수용된 실제 경로가 먼저 필요 |
| N12 파일럿 명세 | 요구3개·기준4개의 문장을 유지하고 version2에서 기능 개념3개로 연결. 기존 공통 vocabulary20개는 보존 | 기존 broad concept8개를 모두 요구 경로로 유지한다면 미작성5개와 각 broad 기능/시험의 정확 경로를 추가로 구성·검토 |
| 검토자 표시 | 실제 판정한 사람의 이름을 사용 | fixture reviewer/운영 담당자/chat-user를 추정해 넣지 않음 |

N12 권고 초안은 [교정 ontology](../../system/eval/b0-knowledge-system/refactoring-n14-run-roundtrip-2026-10-08/functional-ontology-option.yaml)와 [version2 spec](../../system/eval/b0-knowledge-system/refactoring-n14-run-roundtrip-2026-10-08/functional-spec-option.yaml)이다. 필수 질문3개를 보완해 생산 파서·구조 검사를 통과했으나 source-backed 후보에 적용하지 않았다. 관계13개와 기존 policy/ADR 검토 자료도 아직 proposed다.

코드 POLICY를 평가에 포함해도 실제 운영 신뢰·권한 N16의 owner/키/회전·폐기 수용은 계속 필요하다. 코드 TRACE를 평가에 포함해도 N12 실제 의미 관계 검토와 N14 실제 linked run·사람 criterion 판단·승격/반려/rollback을 대신하지 않는다. 기존 WHY13/15의 코드 흐름 추론을 WHY로 인정할지도 개별 분류에서 판정한다.

범위 선택 이후 해야 할 일은 다음과 같다.

1. 선택한 범위에 맞는 질문/gold/군·사실군/독립성·오염·기권을 문항별 실제로 검토하고 변경된 파일 SHA에 기록한다. 문항별 승인을 범위 선택만으로 생성하지 않는다.
2. 현재13FINAL/8제안군의 부족 범위를 보강한다. 승인 FINAL은 CODE/WHY/POLICY/TRACE/ABS 모두0이며 각10개가 필요하다. 기존 제안이 모두 승인돼도 각각6/6/9/8/9군이 추가로 필요하다. CODE/WHY 공유군이 있으므로 이 합을 전체 고유 표본 수로 쓰지 않는다.
3. N12의 선택 명세와 승인된 정확 관계를 새 capture/색인/semantic projection에 결합한다. 과거 held source/runtime에 자동 승계하지 않는다.
4. 새 입력과 source/model/config/binary/dataset/runtime을 사전 동결한 뒤 DEV 개선과 독립 FINAL 평가를 실행한다. actual linked pass와 사람 criterion approved는 별도로 기록한다.

근거는 [실행 명세 N11-A/B·N12-A](./REFACTORING-EXECUTION-SPEC.md)의 사람 검토·입력 동결 조건이다. 이 조건의 내용을 모델이 사람 판정으로 대신 작성하지 않는다. 활동 시간을 측정하지 않았으면 null로 남기고 대기 시간을 활동 시간으로 기록하지 않는다.

**다음:** 선택한 범위로 문항/관계별 판정 자료·부족 범위를 보강하고 실제 검토자 이름을 기록한다. 범위 선택은 완료이며 개별 검토·full source/runtime 결합은 남는다. **남은 전체:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
