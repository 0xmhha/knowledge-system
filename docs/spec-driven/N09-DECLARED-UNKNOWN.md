# N09 검토된 미지정 사실에서의 기권

2026-10-11 후속: [FINAL2](./FINAL2-RESULT.md)에서 승인 unknowns 2건으로 무답 on arm 240/240 `declared_unknown`, 대조 잘못된 기권0. 무답2단위라 inconclusive.

2026-10-10 · **제품 변경·시험 완료 / 저장소 자료·검토·새 FINAL 대기 / N09 미완료**. 전체13/18, 남은5개.

## 1. 왜 바꿨는가

새 FINAL에서 무답 문항(운영 RTO/RPO, 복구 담당자)이 나왔을 때, 8개 arm 모두 미지정 근거가 아닌 인용을 100% 돌려줬다([FINAL 결과](./N11-FINAL-RESULT.md)). 검색 도구에는 "이 사실은 정해지지 않았다"고 답하는 표면이 없었다. 사용자는 점수 임계값 대신 **미지정 사실을 명시적으로 등록하는 방식**을 골랐다.

## 2. 무엇을 바꿨는가 (`22e45ff7`)

**레코드 형식.** knowledge overlay에 `unknowns/*.yaml` 레코드(DeclaredUnknown)를 추가했다. 필드는 다음과 같다.
- 문장
- scope
- 기록일(`recorded_on`)과 해소일(`resolved_on`)
- 일치 용어 묶음(`match_any`): 묶음 1–16개, 묶음당 용어 1–6개, 용어는 2글자 이상
- 근거 span(`basis`): 1–4개
- 검토 상태와 공개 범위

lock digest, 검토 기록(`target_kind: unknown`), `cks knowledge init` 디렉터리 목록에도 포함했다.

**선택 규칙.** 다음 조건을 모두 만족하는 레코드만 일치로 본다.
- `verified` 상태이고 scope가 맞는다.
- 기록일 ≤ 조회일 < 해소일이다.
- 한 묶음의 용어가 모두 질의에 들어 있다(대소문자 무시).

**기권 동작.** knowledge 요청이 일치하면 v2 pack이 검색된 근거를 버리고, 레코드의 근거 줄만 보관 원문에서 다시 읽어 인용한다. knowledge 상태는 `declared_unknown`이 되고, 레코드 ID·검토자·기록일·근거 인용이 `declared_unknowns`에 실린다.

**안전장치.**
- 일치한 레코드가 restricted이면 식별자를 숨기고 `restricted`로 표시한다. 기존 근거는 유지한다.
- 근거 줄을 검증하지 못하면 기권하지 않는다. 기존 근거를 유지하고 knowledge를 unavailable로 표시한다.
- 계약 검증은 `declared_unknown` 상태에서 정책·결정·관계·trace 주장이나 근거가 아닌 인용이 섞이면 거부한다. 다른 상태에서 `declared_unknowns`가 나와도 거부한다. 이후의 trace 첨부도 이 상태에서는 하지 않는다.

## 3. 어떻게 확인했는가

- **evidencev2 시험 4개.**
  1. 근거 줄만 인용하고 대소문자를 무시해 일치한다.
  2. 다음 경우에는 기존 응답이 바뀌지 않는다: 용어 없음, 묶음의 일부만 일치, 기록 전, 해소 후, proposed.
  3. restricted이거나 근거가 범위 밖이면 기존 근거를 유지한다.
  4. 혼합 인용과 상태 밖 레코드를 검증이 거부한다.
- **knowledgepack 시험 2개.** 선택 규칙, 그리고 잘못된 레코드 11종 거부를 확인한다.
- **변이 6개를 모두 잡았다.** guard 미호출, 검증의 근거 외 인용 허용, proposed 선택, 묶음 첫 용어만 검사, 해소일 무시, restricted 기권이다([기록](../../system/eval/b0-knowledge-system/refactoring-n09-declared-unknown-2026-10-10/mutation-checks.txt)).
- **관련 race.** 59개 패키지의 race, vet, gofmt가 통과했다([출력](../../system/eval/b0-knowledge-system/refactoring-n09-declared-unknown-2026-10-10/related-race.txt)).

## 4. 아직 하지 않은 것

- **저장소 자료가 없다.** knowledge-system 저장소에는 `.cks/knowledge` pack이 없다. 지금 평가 corpus에서는 이 기능이 작동할 대상이 없다. 그리고 미지정 사실 레코드는 프로젝트 사실이므로, 문장·일치 용어·근거를 사람이 검토해야 한다.
- **예전 FINAL ABS는 쓸 수 없다.** 예전 FINAL ABS가 묻던 운영 RTO/RPO는 2026-10-10 로컬 운영 결정(RTO 1시간, RPO 24시간)으로 정해졌다. 그래서 새 corpus 시점에서는 무답이 아니다.
- **새 평가가 필요하다.** 무답·답가능 대조 사례가 들어간 새 DEV와 새 독립 FINAL이 필요하다. 레코드와 FINAL 질문을 같은 agent가 쓰면 독립성이 약하다. 이 점은 검토할 때 함께 판단해야 한다.
- **pack off arm에는 효과가 없다.** 이 guard는 knowledge를 요청한 응답에만 작동하므로, pack off arm의 무답 인용은 그대로다.

**남은 전체5개:** N08 검색 품질, N09 기권(자료·검토·새 FINAL), N10 비용·지연(새 FINAL), N16 신뢰·권한(서명 키), N18 기본값·출시 판정.
