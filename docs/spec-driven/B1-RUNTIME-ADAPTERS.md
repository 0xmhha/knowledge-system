# B1 비교 실행 경로

2026-10-04 · B1-01 네 경로와 B1-02 팩 off/on의 8개 경로를 합성 입력에서 구현 검증했다. 공식 paired 비교와 실제 팩 사실의 사람 검토는 남아 있다. 온톨로지 기본 활성 또는 품질 통과를 선언하지 않는다.

## 현재 실행 계약

기존 설정에서 `semantic.ontology_mode`를 생략하면 이전 실행과 응답 형식을 유지한다. `off` 또는 `baseline`을 명시한 비교 설정은 첫 CKV 요청을 원문 질의로 보낸다. 이후의 기존 glossary/keyword 반복은 유지한다. `concept_text`·`relations`·`combined`도 같은 원문 우선 경로를 사용하며 CKV/CKG 검색이 끝난 뒤 의미 데이터를 검사한다. 비교 arms에서는 glossary·K·intent·모델·필터와 그 밖의 설정을 동일하게 고정해야 한다.

```yaml
semantic:
    store_path: /private/tmp/b1-semantic.db
    ontology_mode: relations
    ontology_budget_ms: 1000
```

`ontology_budget_ms`의 0/생략은 1000ms이며 최대 5000ms다. 관계와 텍스트를 합친 총 boost는 기존 허용 상한 0.2를 사용한다. 잘못된 mode는 설정 단계에서 거부한다.

MCP 시작 때 해석한 고정 version/project/dataset/snapshot/commit으로 의미 projection을 읽는다. `semantic_current`가 다른 버전을 가리키거나 활성 포인터가 없어도 이 tuple만 조회한다. CKV/CKG 좌표·canonical ID·CKV 청크 연결·보관 원문 증거를 다시 검사한다. 읽기 전용 의미 DB는 파일 생성·쓰기·자동 마이그레이션을 하지 않는다. 과거 schema는 운영자가 별도로 마이그레이션해야 한다.

질의에 맞는 복수 개념을 유지하며 verified 개념과 verified `IMPLEMENTED_BY` 증거만 이미 검색된 canonical ID·commit·파일·겹치는 줄의 점수에 영향을 준다. Stage 2의 기본 상위 K ID 집합을 먼저 고정하고 그 안에서 재순위한다. 테스트/문서/아카이브의 기존 감점도 유지한다. 새로운 후보를 추가하거나 모호한 의미 하나로 필터링하지 않는다. 개념 동점이 상한 8을 넘거나 구현 증거가 128을 넘으면 작업을 버리고 기본 결과로 돌아간다.

## 개념 텍스트와 결합 경로

`concept_text`는 질의에 맞는 verified 개념의 정의·포함/제외·다국어 용어를 `ExportTextCorpus`와 같은 바이트로 렌더링한다. 검토자와 개념 자신의 문서 출처가 있어야 한다. 같은 CKV 클라이언트·모델 신원·고정 색인·기본 검색 필터를 재사용하여 이 텍스트를 추가 검색한다. 현재 MCP의 원문 recall과 같은 K=20 및 BM25 rerank를 사용한다. 별도 개념 벡터 색인을 구축하는 방식은 아니다. 출력에 추가 검색 결과를 새 후보로 넣지 않고, 원래 Stage 2 상위 K 집합과 같은 commit의 정확한 인용 키에 해당하는 결과만 `0.2/rank`의 soft boost 신호로 사용한다. 복수 개념은 가장 큰 신호만 사용하며 합산하여 상한을 넘기지 않는다. 구현 관계를 조회하지 않는다.

`combined`는 텍스트 검색과 관계 검증을 모두 수행한다. 관계를 우선 적용하고 텍스트 기여를 더하되 각 인용의 원래 점수 대비 총 증가를 20%로 제한한다. 관계만으로 상한에 도달한 인용은 텍스트 검색에 다시 나와도 텍스트 기여로 중복 계산하지 않는다. 검색 비용과 실제 점수 기여는 서로 다른 수다. 텍스트 검색이 실패하면 관계까지 포함한 optional 변경을 버린다.

개념당 렌더링 텍스트는 6144바이트, 질의당 개념은 8개가 상한이다. 초과한 텍스트를 잘라서 부분 성공으로 사용하지 않는다. 전체 협력적 deadline은 관계 경로와 공유한다. 결과에는 추가 검색 시도 수(`text_search_calls`), 실제 텍스트 영향을 받은 인용 수(`text_boosted_citations`), 각 개념의 원문 좌표·줄 SHA·질의 SHA·반환 hit 수(`text_sources`)를 기록한다. 원문 정의 자체는 선택형 메타데이터로 내보내지 않는다. 원자료의 projection·보관 소스와 같은 렌더러로 질의를 재구성할 수 있다. 텍스트 검색 실패는 `unavailable/text_search_failed`로 구분한다.

## 결과와 예산

세 optional mode에서만 `metadata.ontology`를 v1/v2 응답에 첨부한다. 예시는 다음과 같다.

```json
{"mode":"relations","state":"active","baseline_citations":10,"matched_concepts":1,"applied_relations":1,"boosted_citations":2}
```

`applied_relations`는 실제 점수에 사용한 고유 관계 출처 수, `boosted_citations`는 영향을 받은 인용 수다. 한 관계가 두 인용에 영향을 주더라도 관계 수를 두 번 세지 않는다. `baseline_citations`는 Stage 2의 상한 적용 집합 크기이며 최종 본문/인용 수가 아니다. `active`는 경로 실행 상태이며 정책 사실의 참, 구현 품질 또는 사람의 수용 판정이 아니다. 복수·proposed 후보가 남아 있어도 verified 텍스트/관계 외에는 점수를 바꾸지 않는다.

폴백은 `unavailable`(저장소/고정 tuple 누락), `stale`(좌표·증거·projection 무결성 불일치), `budget_exceeded`(deadline/개념/관계 상한)로 구분하고 원래 순위·후보를 유지한다. `no_match`는 질의 개념 없음, `no_candidates`는 원문 검색 후보 없음이다. 오류 상세 파일 경로나 미검토 사실은 상태 메타데이터로 내보내지 않는다. Composer trace에도 같은 상태와 추가 텍스트 검색 시도 수를 기록한다. 기존 trace의 CKVCalls는 recall rounds에 텍스트 호출을 더한 값이며, 과거 knowledge pass는 포함하지 않는다. 이 trace의 과거 의미를 바꾸지 않고 [B0 선택형 계측](./B0-MEASUREMENT-TOOLS.md)으로 실제 CKV/CKG/intent/health와 Ollama HTTP 시도를 원응답 ID에 연결한다. v1 기본 무결성과 v2 `sha256-v2`는 이 선택형 메타데이터까지 보호한다. 기본 mode의 과거 골든은 그대로다.

deadline은 협력적 제한이다. SQL에는 context를 전달하고 각 단계의 종료 후 시간 초과를 확인해 만료된 재순위는 사용하지 않는다. 동기 원문/그래프 검증 자체를 강제로 중단하는 wall-clock 보장은 아니다. 전체 코퍼스 지연과 1.25배 회귀 기준은 실제 공식 비교에서 측정해야 한다.

## 실제 실행 검증

독립 Git 입력과 synthetic `fixture-reviewer` 사실로 네 정상 경로와 각 optional 경로의 누락/변조 상태를 실행한다. 이 식별자는 실제 운영 사실에 대한 사람 승인으로 사용하지 않는다. 기존 출력 디렉터리는 거부한다.

```bash
python3 scripts/wbs-ontology-relations-smoke.py --out /private/tmp/b1-relations-mock
CKV_REQUIRE_COMPLETE_EMBEDDINGS=1 python3 scripts/wbs-ontology-relations-smoke.py \
  --out /private/tmp/b1-relations-ollama --embedder ollama \
  --model-name bge-m3:latest --ollama-url http://127.0.0.1:11434
```

mock과 실제 BGE-M3에서 각각 정상 경로 4개와 오류 경로 6개의 v1/v2 요청 20개를 확인했다. 실제 모델은 승인된 digest `790764642607…16bab`, 1024차원이다. 실제 관계 1개가 인용 2개에 적용됐다. 실제 텍스트-only는 검색 1회로 기본 인용 10개 중 8개에 영향을 주고 관계 기여는 0이었다. 결합은 관계 1개와 텍스트 검색 1회, 텍스트 추가 기여 인용 6개를 기록했다. 이 수는 품질 개선 점수가 아니다. 누락/변조는 기본 인용·본문을 유지했다. `semantic_current`를 활성화하지 않아도 고정 tuple을 읽었으며 의미 DB SHA는 변하지 않았다. v2 무결성은 독립 JSON 정규화로 검사했다. 집중 시험은 원문 우선, 상위 K 보존, 다의어/proposed 무승격, 시간 초과·상한·잘못된 mode·누락 tuple과 읽기 전용 DB 쓰기 거부를 확인한다.

[관계 실행 원장](../../system/eval/b0-knowledge-system/b1-relations-runtime-m2max-2026-10-03.json)과 [네 경로 실행 원장](../../system/eval/b0-knowledge-system/b1-text-runtime-m2max-2026-10-03.json)에 명령·코드/원자료 SHA와 한계를 연결한다. 이 실행은 합성 입력의 진단이며 공식 B1 품질/지연 표본이 아니다. 공식 gold 또는 미검토 운영 관계를 verified로 승격하지 않았다.

## 팩 off/on의 8개 경로

`--pack-matrix`는 같은 보관 소스·CKV/CKG·의미 DB·잠긴 팩을 유지하고 v2 요청의 `include_knowledge`만 바꾼다. 팩 off에서도 물리적 팩을 지우거나 색인을 다시 만들지 않는다. 같은 mode의 off/on 설정 SHA는 같으며 전체 설정은 `ontology_mode` 외에 같다. 기존 v1은 이 팩 축을 지원하지 않으므로 이 행렬은 v2로 실행한다. 기존 기본 스모크의 v1/v2 검사는 별도로 유지한다.

| 온톨로지 mode | 팩 off | 팩 on |
|---|---|---|
| baseline | 원문 후보 | 원문 후보 + 적용 가능한 검토 정책 |
| concept_text | 후보 안의 개념 텍스트 신호 | 같은 텍스트 신호 + 검토 정책 |
| relations | 후보 안의 검토 관계 신호 | 같은 관계 신호 + 검토 정책 |
| combined | 후보 안의 상한 적용 결합 | 같은 결합 + 검토 정책 |

```bash
python3 scripts/wbs-ontology-relations-smoke.py --pack-matrix \
  --out /private/tmp/b1-eight-arms-mock
python3 scripts/wbs-ontology-relations-smoke.py --pack-matrix \
  --out /private/tmp/b1-eight-arms-ollama --embedder ollama \
  --model-name bge-m3:latest --ollama-url http://127.0.0.1:11434
```

스크립트는 strict embedding을 요구하고 기존 출력 디렉터리를 거부한다. 합성 `fixture-reviewer` 상태로 공개·proposed·만료·restricted·충돌 정책과 범위 밖 요청을 검사한다. 운영 정책의 승인이나 B0 개발 의미 제안의 승인을 만들지 않는다. source Git commit과 snapshot은 mock/BGE-M3 간 같고, 모델별 dataset ID는 다르다. 각 실행의 8개 경로는 같은 dataset ID와 팩 lock digest를 사용한다. `semantic_current`는 비어 있고 읽기 전후 소스·의미/벡터/그래프 DB SHA가 같다.

mock/BGE-M3 각각 8경로 × 6상태의 48요청을 검증했다. 공개 정책은 on에서만 요구 동작 1개와 자체 출처 인용 1개를 추가했다. 충돌 정책 2개는 출처와 충돌 정보만 제공하고 요구 동작은 비워 둔다. proposed·범위 밖은 unknown, 만료는 stale, 접근 제한은 restricted이며 정책 ID와 요구 동작을 내보내지 않는다. 제한 정책 원문이 기본 인용에 들어가지 않는지도 검사한다. 온톨로지 mode 간 최종 인용/본문 집합은 같은 팩 축 안에서 유지된다. on은 off의 기본 인용/본문을 모두 보존한다. 모든 v2 본문·줄 범위·소스 SHA·좌표·무결성을 원문 및 독립 JSON 정규화로 검사했다.

`matrix-manifest.json`은 8개 경로의 순서, 설정/질의/바이너리/원응답 SHA, 좌표·모델 신원, 입력·DB·본문·응답 JSON 크기와 원시 시간을 기록한다. K는 **현재 컴파일 기본값 계약**인 raw/text recall 20, Stage 2 후보 상한 30, 별도 knowledge pass 6을 출처 코드 SHA와 함께 기록한다. 이는 호출별 backend telemetry가 아니다. 초안 프로토콜의 retrieval_k=10과 같다고 표시하지 않는다. 원시 단일 캡처 시간은 지연 점수로 집계하지 않고, 현재 행렬의 `arm_rotation=false`를 명시한다. 이 최초 원장에는 순서 회전과 호출별 telemetry가 없다. 후속 B0 계측 원장은 실제 K와 logical/HTTP 호출 수를 확인했고, 순서 회전은 B0-06/B1-03에 남아 있다.

[8개 경로 원자료 원장](../../system/eval/b0-knowledge-system/b1-pack-matrix-m2max-2026-10-04.json)에 입력 bundle·잠금·원응답·검증 명령과 최초 스크립트 오류를 보존한다. 공식 품질 지표는 null이다.

## 남은 공식 비교 작업

네 어댑터와 합성 팩 축의 구현 검증은 완료됐다. 공식 프로토콜에서 입력·glossary·모델·K·필터·intent를 잠그고 모든 arms의 실제 조건을 감사해야 한다. 실제 팩 사실의 사람 판정, 승인된 K 적용, 순서 회전·공식 고정 입력 원장, 공식 원자료·주장별 사람 판정은 남아 있다. 전체 상태는 [실행 작업리스트](./EXECUTION-WORKLIST.md)를 따른다.

2026-10-04 FIX-07 수정 후 검증: broad recall의 invariant/convention 청크를 코드 검색어로 사용하지 않도록 수정했다. 지식 증거·별도 pass·raw K/필터를 유지했고, 같은 실모델 데이터셋의 8경로 48개 전체 SDK 응답은 수정 전과 같았다. BM25 오류 96→0, 출처/integrity·정책 상태·DB 무변경 재감사 통과. 문서/header의 비심볼 Neighbors 실패는 보존했다. [수정 원장](../../system/eval/b0-knowledge-system/fix07-knowledge-keywords-m2max-2026-10-04.json). 공식 품질은 미판정이며 다음 작업은 arm 회전과 승인 K 적용 경로다.

2026-10-04 공유 K 설정 후속: `retrieval.recall_k`가 raw/concept-text에 같은 값으로 전달된다. 실모델 8경로 48요청 K10과 기본 v1/v2·폴백 20요청 K20을 호출 원장으로 확인했고 knowledge pass K6·Stage2 cap30을 유지했다. [원장](../../system/eval/b0-knowledge-system/recall-k-m2max-2026-10-04.json). 공식 프로토콜은 여전히 사람 검토 대기이며 다음 독립 작업은 arm 순서 회전·공식 입력/하드웨어 원장이다.
