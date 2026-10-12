# 온톨로지 MCP 보완 구현 기록

2026-10-13 · **O01–02 구현·검증 완료, O03–04 미완료**. 기존N17/18·잔여N16 유지. 온톨로지2/4·잔여2개, 전체 잔여3개(N16/O03/O04).

## O01: cks.semantic.status

입력 없음. 검색이나 모델 호출 없이 이 MCP 서버에 고정된 source/dataset와 읽기 전용 의미 projection을 검증한다. `mode`와 `availability`를 분리하므로 검색 mode off여도 자료가 준비돼 있으면 `read_tools_ready=true`, `retrieval_ready=false`다.

- state: off, ready, unreviewed 또는 불가 상태.
- availability: unconfigured, dataset_missing, store_missing, store_unavailable, unsupported_dataset, projection_missing, stale, budget_exceeded, ready. 잘못된 mode는 unsupported.
- reason: 안정된 이유 코드. 원문·DB error·로컬 path는 제공하지 않는다.
- counts/snapshot: retained source와 engine coordinate/해당 evidence 검증 통과 시만 제공. 개념/요구/관계와 verified 수 및 실제 usable 구현 관계 수를 구별한다.
- read_tools_ready는 정렬된 자료를 읽을 수 있다는 뜻. retrieval_ready는 설정 mode에 맞는 검토 개념/구현 자료가 있다는 뜻이며 실제 query match/품질/시험/criterion 수용을 의미하지 않는다. relations mode에는 usable reviewed 구현이 필요하고 text/combined는 reviewed 개념이 필요하다.
- 새 도구가 missing store를 생성하거나 current/semantic activation을 바꾸지 않는다. source/store 불일치와 deadline 뒤 결과는 counts/좌표 없이 거부한다. 기본 budget1초, 서버 설정에서 최대5초.

[코드](../../internal/system/mcp/semantic_status.go), [수용 시험](../../internal/system/mcp/semantic_status_test.go). 실제 retained YAML capture/ontology parser·SQLite에 문서-only fixture를 저장한 positive/off/verified-proposed 및 누락/stale/취소/경로 비노출/current 없음·store 바이트 불변을 확인했다. 코드 앵커/CKV DB가 없는 문서 fixture이며 실제 운영 도메인 수용이라고 기록하지 않는다.

MCP 등록 없음 RED→도구25개 등록/input/output 계약 GREEN. 관련3패키지 전체 race **171시험·156하위시험 pass, fail0/skip0**, vet/엔진 경계/diff 검증 통과. 진행 보호 Python **11시험 pass**; O목록/원장 누락·중복·완료 표기 불일치·완료된 next 거부가 추가됐다. 처음 만든 fixture의 추출기 종류 오류는 제품 regression과 구별해 보존했다. [원자료](../../system/eval/b0-knowledge-system/ontology-o01-status-2026-10-13/verification.json).

다음 O02는 같은 pinned retained 검증을 공유해 용어 후보·개념/관계·요구 추적을 조회한다. O03은 설치 연결, O04는 설명/적용 전후 평가다. 새 실제 도메인 승인/모델 평가/운영 활성화는 아직 없다.

## O02: 개념·용어·요구 추적 읽기 MCP

등록 tool28개. 세 도구는 mode off에서도 같은 pinned retained source/projection을 검증해 읽는다. 상태 도구의 read_reason에 sanitizer_unavailable도 제공한다.

| 도구 | 입력 | 결과 |
|---|---|---|
| cks.semantic.lookup_term | language, term, limit(1..100, 기본20) | 정확 언어/대소문자 정규화 용어, 전체 후보수/ambiguous/truncated, 반환 후보의 review status·원문 |
| cks.semantic.get_concept | concept_id, limit | 정의·범위·다국어 terms·검토자/상태, subject/object 방향을 가진 incident 관계와 evidence, 전체 관계수/truncated |
| cks.semantic.trace_requirement | requirement_id, limit | 요구·버전·Given/When/Then·trace state/누락·충돌·code/test·원문; acceptance_evaluated=false |

반환 state는 ready/no_match/not_found 또는 invalid_request/unconfigured/stale/budget_exceeded/restricted 등이다. 오류는 MCP IsError를 함께 표시한다. Result가 있는 응답은 digest 필드가 비어 있는 상태의 JSON 전체를 SHA256한 digest를 제공한다. raw 문자열은 서버 sanitizer로 검사하며 redaction/drop/fail-closed일 때 의미 데이터를 반환하지 않는다. 텍스트를 escaping한 JSON만 검사해서 multi-line rule을 놓치지 않도록 했다. result bytes>32000이면 budget_exceeded다.

lookup의 반복·번역 후보를 독립 표본으로 세지 않는다. 모호 후보를 limit으로 잘라도 전체수와 ambiguous/truncated를 유지한다. relation verified와 test pass/criterion accepted를 합치지 않으며 command 실행이나 current 전환은 없다. 사례는 document-only 실제 parser/store fixture와 순수 selector 모델을 구분한다. 실제 CKG code 경로 검증은 기존 semantic 회귀 범위로 유지한다.

관련3패키지 전체 race **175시험·169하위시험 pass, fail0/skip0**, vet/boundary/diff 및 MCP28등록·새 입력/output schema 통과. 세 조회도구 미등록 RED→GREEN; test-only sanitizer 작성 오류는 별도로 보존. [원자료](../../system/eval/b0-knowledge-system/ontology-o02-reads-2026-10-13/verification.json).

**현재 O2/4 완료. 다음 O03 프로젝트 ontology/spec 준비·설치 연결. 남은 전체 N16/O03/O04의3개.**
