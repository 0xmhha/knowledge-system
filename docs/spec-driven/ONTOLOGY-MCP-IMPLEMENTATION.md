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

## O03: 명시적 설치 연결 — 기술 준비 완료 / 실제 도메인 대기

cks mcp gen-config에 --ontology-mode/--ontology-budget-ms를 추가하고 의미 저장소 경로를 절대 경로로 보존한다. enabled mode+store 미지정·잘못된 mode/budget은 설정을 쓰기 전에 거부한다. default off는 기존 raw-query-first 선택을 바꾸지 않도록 YAML mode를 비워 둔다(표시 상태는 off).

ks-setup은 --project-id/--semantic-store/--ontology-mode/--ontology-budget-ms와 source-relative --ontology/--spec을 지원한다. store는 소스 트리 밖에 두며 graph-only 의미 입력·외부/탈출 경로·잘못된 budget/mode는 거부한다. 새 ontology/spec에는 안정 project ID가 필요하고 기존 init 설정과 다르면 거부한다. 기존 binary에 기본 경로의 새 옵션을 강제 전달하지 않는다.

의미 입력 또는 store 연결이 요청되면 index를 --hold-for-review로 만든다. ontology/spec 파일은 같은 candidate의 **retained source**에서 추출하고 검증 후 저장·제안 JSON을 만든다. --activate는 전달하지 않으며 source status/reviewer를 임의로 바꾸지 않는다. config는 후보 전용 이름/버전 경로를 사용하므로 이전 current·운영 config를 덮어쓰지 않는다.

준비 단계는 정의/요구를 추출할 뿐 code/test 관계를 발명하지 않는다. status smoke는 backend health와 구별해 read_tools_ready를 확인하고, 명시된 enabled mode에는 retrieval_ready를 요구한다. 모호성/미검토는 조회 가능하지만 재순위 수용으로 간주하지 않는다.

자체 **실제 Git source/CKG·CKV DB(mock embedder)/semantic SQLite/CLI3/stdio MCP**에서 긍정/부정 통합을 실행했다. proposed 개념1/요구1을 같은 snapshot에서 읽었고, mode off·read ready true·retrieval ready false였다. 새 source에 옛 store를 빌리거나 다른 project ID의 YAML을 주면 거부됐고 매번 old current/운영 설정 SHA가 같았다. 실모델은 쓰지 않았고 startup은 닫힌 시험 loopback endpoint를 사용해 모델 불가(degraded)이며, 읽기 의미 도구는 독립적으로 동작했다. 이는 실제 go-stablenet의 데이터/내용 수용 또는 검색 품질 시험이 아니다.

관련 Go3패키지 race55시험/48하위시험 pass·Python26pass/optional E2E1skip. 별도 실제 통합은 실행했다. [원자료](../../system/eval/b0-knowledge-system/ontology-o03-installation-2026-10-13/verification.json), [공개 설치 사용법](../../tools/setup/README.md#reviewed-meaning-and-ontology-readiness).

O03-A/B 기술 조건은 충족. O03-C의 실제 도메인 채택과 O03-D의 대상 운영 결합은 **미완료**다. go-stablenet의 ontology/spec/관계·검토자와 적용할 원문/버전이 아직 정해지지 않았다. 기존 knowledge-system 파일럿 DB를 다른 corpus로 전용하지 않는다. 실제 자료가 정해지기 전에는 온톨로지 2/4 완료와 전체 잔여 N16/O03/O04의3개를 유지한다.

## O04: 설명과 저장 응답 비교 — 실제 효과 평가 대기

v2 get_for_task의 `explain_ontology=true`는 요청 하나에만 설명을 붙인다. mode/store 설정을 켜거나 바꾸지 않고 기본 응답에는 새 설명을 붙이지 않는다. 최대8개 매칭 개념 ID/status, 최대20개 점수·순위 변화와 전체 변화 수/잘림, 사용한 relation/text evidence ID를 보여 준다. 후보 키는 SHA256으로 표현해 인용되지 않은 path/본문을 직접 노출하지 않는다. `selected_by_composer`는 composer 본문 선택 여부이며, v2 최종 budget 추가 누락은 `final_body_budget_partial`와 실제 공개 Bodies로 확인한다.

기존 ontology diagnostic의 active는 처리 경로가 실행됐다는 뜻이다. 설명의 applied는 점수 또는 순위가 바뀌었다는 뜻이고, no_effect는 active여도 변화0이라는 뜻이다. off/no_match/unavailable/stale/budget_exceeded는 구별한다. **applied가 더 정확한 답변이라는 뜻은 아니다.** `quality_evaluated=false`다. 설명의 string values도 기존 sanitizer를 통과해야 하며 restricted면 세부 ID/변화 없이 안정 state만 반환하고 이미 검증된 본문은 유지한다. 점수는 소수 문자열로 출력해 v2 정수 전용 canonical 규약을 유지하며 새 metadata도 최종 hash에 포함한다.

`cks semantic compare-responses --off off.json --on on.json`은 저장된 v2 JSON(각4MiB 이하)의 hash/같은 query/coordinates를 검증한다. off와 on mode를 구별하고 인용 membership/order, 최종 body 출력 변화와 hash를 보고한다. 원문 질문/경로/본문은 비교 보고서에 넣지 않는다. 서로 다른 입력·좌표/무결성 변조/잘못된 arm/unknown state/trailing JSON/초과 입력을 거부한다. 두 응답의 구조만으로는 같은 binary/config/model/모든 요청 옵션·활동시간/실패 분모/모델 호출/정답/독립 단위를 입증할 수 없다. `quality_evaluated`, `latency_evaluated`, `control_conditions_verified`는 false이며 기존 FINAL을 새 독립 표본으로 계산하지 않았다.

검증은 실제 retained source를 읽는 MCP v2 handler에 **controlled provider/검색 fixture**를 결합해 인과 동작을 고립시켰다. fake의 검토 relation/canonical은 실제 corpus 관계나 사람 승인을 뜻하지 않는다. off→on 후보2의 순위2→1과 score 증가, 선택/사용 관계 표시·hash 검증, 기본 explanation 없음, active-but-zero/no_effect, unavailable, restricted 세부 제거/원본문 유지가 통과했다. 별도 실제 v2 builder 출력의 동일 입력/order/body 변화와 changed membership/no_effect·6개 불일치 거부, Cobra CLI 및 파일 한계도 검증했다.

관련5패키지 race **227시험/232하위시험, 실패0·skip0**, vet/boundary/docs/progress guard 확인. 원자료에는 baseline 설명 옵션 부재 RED, 초기 score float의 v2 canonical 충돌, 이후 회귀, 마지막 시험의 기존 restricted 코드에 대한 잘못된 기대값 수정 이력도 구별해 보존한다. [원자료](../../system/eval/b0-knowledge-system/ontology-o04-explanation-2026-10-13/verification.json).

**O04-A만 충족**. B 실제 paired 실행·timing/실패 분모·모델 호출 및 같은 조건의 receipt, C 실제 도메인 채점·독립성/표본/quality 판정, D 그 실제 평가와 source 결합은 남았다. 공개 문서/저장 비교 도구가 있어도 실제 실행/품질 완료로 체크하지 않는다.

### 다음 작업과 전체 잔여

- N16: 사용자가 미충족 유지한 서명 키/fingerprint. 로컬 commit/binary SHA 신뢰 범위 유지; 키를 임의로 만들지 않는다.
- O03: 사용할 실제 프로젝트의 ontology/spec/관계와 검토 기록·대상 원문/commit/version을 확정하고 held candidate로 준비·검증한다. 기존 운영 current/config는 그대로 두었다.
- O04: O03 자료가 정해진 뒤 같은 dataset/model/binary/config/질문·요청 옵션/seed·실행 순서·warm 조건과 독립 평가 조건을 먼저 동결하고 off/on 인용·본문·상태·지연/실패 분모/모델 호출을 비교한다. 표본 부족은 inconclusive이며 효과 개선을 보장하지 않는다.

현재 기존 N17/18·온톨로지2/4 완료, 전체 잔여3개다. 이 문서와 원장의 부분 체크는 기술 준비와 실제 수용을 나누며, 완료로 바꿀 때에는 해당 전체 조건과 committed/current SHA 결합이 필요하다.

## O04 후속: 실제 stdio 수집 연결과 같은 검색 전략의 시험

2026-10-13. 빈 후보가 정상인 no_candidates 상태를 설명과 compare-responses가 보존하도록 보완했다. 기존 eval capture의 요청-only 입력 validator에 v2 boolean explain_ontology가 빠져 있었으므로 이를 허용하고 그대로 전송하게 했다. v1에서의 옵션 사용, boolean 아닌 값, gold/쓰기 도구는 계속 거부한다. 세 누락은 변경 전 RED 원자료로 구별해 보존했다.

자체 retained-source/CKG·CKV(mock-feature-hash-v1,64차원)/proposed 의미 저장소와 현재 실제 바이너리로 stdio MCP를 실행했다. 단순 default off는 Stage1 검색 전략도 바꿀 수 있으므로 off arm에는 semantic mode baseline, on에는 relations를 지정해 양쪽 모두 raw-query-first로 고정했다. 차이는 ontology_mode와 기록용 footprint 목적지뿐이며 그 외 config·바이너리·모델·질문·설명 옵션이 같음을 검증했다.

자체 질문2개, 각 arm warmup1/retrieval1/warm2/cold-process1로 총20개 응답을 기록하고, 10쌍을 실제 compare-responses로 해시·동일 query/coordinates·인용·본문 대조했다. 실패0/누락0/pending backend call0, scope별 backend 기록 모두 연결됨. on 상태는 no_candidates5/no_match5, 인용/본문 변화0이었다. mock의 scoped Ollama HTTP 요청은0이고 논리 호출/시작 scope는 별도로 기록했다. 외부 실모델을 쓴 결과가 아니다.

지연은 warmup/retrieval/warm_latency/cold_process별 관측값과 median/min/max만 기록했다. 반복은 독립 표본이 아니며 cold는 새 MCP process를 뜻하고 filesystem/model cache cold가 아니다. sequential off→on 순서도 명시했다. **질문2개나 무변화로 품질 성공/실패·성능 수용을 판정하지 않았고**, O04-B의 실제 도메인 실행과 C/D는 여전히 미완료다. 기존 current와 operator config SHA는 같다.

관련6패키지 race278시험/260하위시험 pass·실패0. skip2는 명시적 DEV fixture를 요구하는 RetainedGuardLiveSDK와 Linux artifact probe이며 이번 실행으로 해당 시험을 대체했다고 주장하지 않는다. vet/boundary/docs/progress guard를 확인한다. [후속 원자료](../../system/eval/b0-knowledge-system/ontology-o04-runtime-2026-10-13/verification.json).

실제 운영 설정의 소스 commit은 0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8이다. graph/vector가 같은 candidate에 있고 dataset identity·retained source manifest도 있으나 의미 store 경로는 비어 있으며 추적 ontology/spec 입력이 없다. 기존 projects/stablenet 팩은 glossary/flow/검토 자료로 존재하고 pack의 code baseline은 ec6a4e96b다. 이 자료를 새 source의 검토 ontology/spec/관계로 자동 승격하지 않는다. 기존 팩 기반 proposed 초안·출처/앵커 대조부터 준비한 뒤 실제 내용 검토와 대상 연결이 필요하다.

다음 O03 proposed 입력/출처 정합성 준비, 이후 실제 검토·held 연결과 O04 actual off/on 평가. 전체 잔여는 N16/O03/O04의3개, 기존17/18·온톨로지2/4 유지다.
