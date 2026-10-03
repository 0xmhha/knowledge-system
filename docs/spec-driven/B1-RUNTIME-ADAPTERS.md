# B1 비교 실행 경로

2026-10-03 · B1-01 진행. 기본 경로와 관계 경로가 실제 MCP에 연결됐다. 개념 텍스트·결합·팩 축과 공식 paired 비교는 남아 있다. 온톨로지 기본 활성 또는 품질 통과를 선언하지 않는다.

## 현재 실행 계약

기존 설정에서 `semantic.ontology_mode`를 생략하면 이전 실행과 응답 형식을 유지한다. `off` 또는 `baseline`을 명시한 비교 설정은 첫 CKV 요청을 원문 질의로 보낸다. 이후의 기존 glossary/keyword 반복은 유지한다. `relations`도 같은 원문 우선 경로를 사용하며 CKV/CKG 검색이 끝난 뒤 의미 데이터를 검사한다. 비교 arms에서는 glossary·K·intent·모델·필터와 그 밖의 설정을 동일하게 고정해야 한다.

```yaml
semantic:
    store_path: /private/tmp/b1-semantic.db
    ontology_mode: relations
    ontology_budget_ms: 1000
```

`ontology_budget_ms`의 0/생략은 1000ms이며 최대 5000ms다. 관계 boost는 기존 허용 상한 0.2를 사용한다. `concept_text`·`combined`와 잘못된 mode는 아직 실행 경로가 없으므로 설정 단계에서 거부한다. 이름만 바꿔 기본 결과로 실행하지 않는다.

MCP 시작 때 해석한 고정 version/project/dataset/snapshot/commit으로 의미 projection을 읽는다. `semantic_current`가 다른 버전을 가리키거나 활성 포인터가 없어도 이 tuple만 조회한다. CKV/CKG 좌표·canonical ID·CKV 청크 연결·보관 원문 증거를 다시 검사한다. 읽기 전용 의미 DB는 파일 생성·쓰기·자동 마이그레이션을 하지 않는다. 과거 schema는 운영자가 별도로 마이그레이션해야 한다.

질의에 맞는 복수 개념을 유지하며 verified 개념과 verified `IMPLEMENTED_BY` 증거만 이미 검색된 canonical ID·commit·파일·겹치는 줄의 점수에 영향을 준다. Stage 2의 기본 상위 K ID 집합을 먼저 고정하고 그 안에서 재순위한다. 테스트/문서/아카이브의 기존 감점도 유지한다. 새로운 후보를 추가하거나 모호한 의미 하나로 필터링하지 않는다. 개념 동점이 상한 8을 넘거나 구현 증거가 128을 넘으면 작업을 버리고 기본 결과로 돌아간다.

## 결과와 예산

관계 mode에서만 `metadata.ontology`를 v1/v2 응답에 첨부한다. 예시는 다음과 같다.

```json
{"mode":"relations","state":"active","baseline_citations":10,"matched_concepts":1,"applied_relations":1,"boosted_citations":2}
```

`applied_relations`는 실제 점수에 사용한 고유 관계 출처 수, `boosted_citations`는 영향을 받은 인용 수다. 한 관계가 두 인용에 영향을 주더라도 관계 수를 두 번 세지 않는다. `baseline_citations`는 Stage 2의 상한 적용 집합 크기이며 최종 본문/인용 수가 아니다. `active`는 경로 실행 상태이며 정책 사실의 참, 구현 품질 또는 사람의 수용 판정이 아니다. 복수·proposed 후보가 남아 있어도 verified 관계 외에는 점수를 바꾸지 않는다.

폴백은 `unavailable`(저장소/고정 tuple 누락), `stale`(좌표·증거·projection 무결성 불일치), `budget_exceeded`(deadline/개념/관계 상한)로 구분하고 원래 순위·후보를 유지한다. `no_match`는 질의 개념 없음, `no_candidates`는 원문 검색 후보 없음이다. 오류 상세 파일 경로나 미검토 사실은 상태 메타데이터로 내보내지 않는다. Composer trace에도 같은 상태를 기록한다. v1 기본 무결성과 v2 `sha256-v2`는 이 선택형 메타데이터까지 보호한다. 기본 mode의 과거 골든은 그대로다.

deadline은 협력적 제한이다. SQL에는 context를 전달하고 각 단계의 종료 후 시간 초과를 확인해 만료된 재순위는 사용하지 않는다. 동기 원문/그래프 검증 자체를 강제로 중단하는 wall-clock 보장은 아니다. 전체 코퍼스 지연과 1.25배 회귀 기준은 실제 공식 비교에서 측정해야 한다.

## 실제 실행 검증

독립 Git 입력과 synthetic `fixture-reviewer` 사실로 네 상태를 실행한다. 이 식별자는 실제 운영 사실에 대한 사람 승인으로 사용하지 않는다. 기존 출력 디렉터리는 거부한다.

```bash
python3 scripts/wbs-ontology-relations-smoke.py --out /private/tmp/b1-relations-mock
CKV_REQUIRE_COMPLETE_EMBEDDINGS=1 python3 scripts/wbs-ontology-relations-smoke.py \
  --out /private/tmp/b1-relations-ollama --embedder ollama \
  --model-name bge-m3:latest --ollama-url http://127.0.0.1:11434
```

mock과 실제 BGE-M3에서 각각 baseline/relations/missing/stale의 v1/v2 요청 8개를 확인했다. 실제 모델은 승인된 digest `790764642607…16bab`, 1024차원이다. 실제 관계 1개가 인용 2개에 적용됐고, 누락/변조는 기본 인용·본문을 유지했다. `semantic_current`를 활성화하지 않아도 고정 tuple을 읽었으며 의미 DB SHA는 변하지 않았다. v2 무결성은 독립 JSON 정규화로 검사했다. 집중 시험은 원문 우선, 상위 K 보존, 다의어/proposed 무승격, 시간 초과·상한·잘못된 mode·누락 tuple과 읽기 전용 DB 쓰기 거부를 확인한다.

[실행 원장](../../system/eval/b0-knowledge-system/b1-relations-runtime-m2max-2026-10-03.json)에 명령·코드/원자료 SHA와 한계를 연결한다. 이 실행은 합성 입력의 진단이며 공식 B1 품질/지연 표본이 아니다. 공식 gold 또는 미검토 운영 관계를 verified로 승격하지 않았다.

## 남은 B1-01 작업

개념 텍스트 경로는 원문 후보를 보존하면서 검토된 텍스트의 별도 검색·원출처 매핑·예산·실패 폴백을 구현해야 한다. 관계 신호를 텍스트-only arm에 섞어 비교의 의미를 바꾸지 않는다. 결합 경로는 두 출처의 기여·비용을 따로 기록해야 한다. B1-02 팩 off/on의 8 arms, 입력 잠금과 순서 회전, 공식 원자료·주장별 사람 판정은 이후 단계다. 전체 상태는 [실행 작업리스트](./EXECUTION-WORKLIST.md)를 따른다.
