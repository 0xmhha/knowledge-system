# 온톨로지 MCP 보완 구현 기록

2026-10-13 · **O01–03 완료, O04 입력·프로토콜/범위 검토 대기**. 기존N17/18·잔여N16 유지. 온톨로지3/4·잔여1개, 전체 잔여2개(N16/O04). [현재 평가 검토안](./ONTOLOGY-EVALUATION-REVIEW-2026-10-13.md). 아래 구현·검증 이력은 당시 상태를 포함한다.

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

## O03 후속: 실제 팩 기반 proposed subset과 원문·앵커 대조

2026-10-13. [검토 묶음](../../projects/stablenet/domain-knowledge/meaning-draft/README.md)에 WBFT quorum/round-change/message-handlers/current-lock 개념4·요구4·기준5와 provenance를 작성했다. 기존 glossary·entry는 그대로이며, 정의/alias와 최초 invariant를 source field/line/SHA로 추적한다. concept kind/scope/preferred·Given/When/example 변환은 저자가 새로 제안한 내용이고 승인된 요구로 취급하지 않는다.

기존 selected4 entry의 verified/mhha 기록과 2026-06-07/09 날짜를 보존했지만 **새 ReviewedBy는 비어 있고 전부 proposed**다. 관측source0bf2f4d1의 파일 SHA와 읽기 전용 graph anchor 점검을 남겼다. pack baselineec6a4e96b 객체가 로컬에 없어 baseline 바이트/diff 대조는 미확인이며 이를 파일 변경으로 판정하지 않는다. 원본47개 입력 SHA 보존. 전체 anchors --check는134checked/105clean/26drift/2review/1unresolved/16loc skipped로 exit1(기존 앵커 drift 관측)이다. 선택한16개는14symbol(12같은 정의 위치/2drift)+2line-only미검증이며, baseline과 file bytes 비교는 모두 미확인이다. 위치 검사는 행동/내용 검증이 아니다.

R04는 기존 summary의 RLock 또는 write Lock 아래 읽기 허용과 첫 invariant의 RLock-only 표현 차이를 명시한 **새 수정 제안**이다. 현재 core.go:162 Lock와:166 current 읽기 관측을 근거로 표현을 구분했으며, 전체 access/race 감사를 했다고 주장하지 않는다. 이 해석과 criterion 변환의 실제 내용 채택이 남았다.

코드 관계 후보16개는 source file/symbol/옛 line을 보존하는 비실행 검토 목록이며 canonical/evidence ID null, semantic assertion0이다. 테스트 목록/validator TestValidatorSet·file SHA 관측은 실행/coverage/criterion mapping이 아니다. source-backed implementation/test 관계 검토를 생략하지 않는다.

실제 ExtractOntology/ExtractSpec/ValidateSources를 전용 Git fixture에서 실행해 13source span·queue8·모두 proposed·trace spec_unapproved4/paths0·외부 project 거부를 확인했다. semantic race44시험20하위시험 pass/실패0/skip0, vet/boundary/docs/progress guard 확인. 전용 fixture에서 YAML를 검증한 것이 운영 go-stablenet의 source/index 결합을 뜻하지 않는다. 초안 파일은 아직 운영 retained snapshot 안에 없으며, 후속 실제 자료 선택·내용 검토 후 새 held candidate에 포함해 다시 검증해야 한다. [원자료](../../system/eval/b0-knowledge-system/ontology-o03-domain-draft-2026-10-13/verification.json).

O03-C/D unchecked, O04-B/C/D unchecked를 유지한다. 다음 C01–C04/R01–R04(특히 R04)의 내용 채택·검토자 기록과 정확한 code/test·criterion 관계 검토, 이후 held source/index/semantic 결합과 실제 off/on 평가. 전체 잔여 N16/O03/O04의3개·기존17/18·온톨로지2/4다.

감사 기록 정정: 최초 audit의 baseline_file_available=false를 bytes unchanged=false로 표현해 파일 변화로 오독했다. 원래 기록을 보존하고 baseline 부재인 경우 nullable/비교미확인으로 고쳤다. 실제 pack/코드 내용 승인으로 확장하지 않는다.

## O03 완료: 사용자 승인과 실제 held WBFT 파일럿 결합

2026-10-13 사용자 “모두 추천 제안으로 승인”을 C01–C04/R01–R04·기준5개·R04의 read-under-write-lock 해석에 기록했다. [승인본/범위](../../projects/stablenet/domain-knowledge/meaning-approved/README.md). 검토자 개인 이름은 지정되지 않았으므로 current conversation user approval을 사용하며 과거 mhha 검토를 옮기지 않는다. 실제 stable project ID에 envelope/status/reviewed_by만 정렬했고 내용은 승인한 초안과 같다. 관계 후보16개·criterion 실행·품질·N16은 이번 승인 범위가 아니다.

원본 code commit0bf2f4d1를 부모로 하는 별도 Git 복사본에 승인 문서2개만 추가했다(후보 source baca2869118774c5f6219932dc76e197a47e6ecd). WBFT core/validator/messages의 explicit dependency closure501파일(Go494)을 실제 BGE-M3 latest/digest790764642607…16bab·1024차원으로 새 source/graph/vector에 결합했다. 운영 source·current·config는 그대로이며 새 dataset에 current pointer를 만들지 않았다. held source2226파일/45,041,303bytes·Git recovery206,663,101bytes·candidate537,846,197bytes다. 전체 프로젝트 배포나 기존 full-corpus 평가를 대신하지 않는다.

필수 schema/alignment/content/coverage 게이트가 통과했다. full go/packages audit는1256 expected/494 DB로762 missing의 DRIFT soft warning을 남겼다. explicit scope와 DB의 Go494파일을 읽기 전용으로 다시 대조해 missing0/overincluded0이며,762는 scope 밖이다. 전체 audit가 clean이라고 표시하지 않는다.

actual semantic build·실제 stdio MCP28도구를 사용해 read_tools_ready=true/mode off/retrieval_ready=false, verified concept4/requirement4·source-backed lookup/concept/trace를 확인했다. 구현 관계를 만들지 않았으므로 trace4는 missing_code(검토된 코드 연결 부재), acceptance_evaluated=false다. 변조된 definition 승격은 거부돼 store SHA가 유지됐고, 옛 dataset은 새 store를 projection_missing으로 거부했다. source/current/config·활성0을 검증했다.

다중 요구 조회에서 peer requirement/source evidence를 먼저 잘라 p.Validate가 실패하던 제품 버그를 실제 자료로 발견했다. 전체 projection을 검증한 뒤 한 trace를 선택하도록 고쳐 실제4개 조회가 통과했다. 실제 C04 질의의 mixed Latin/Hangul term에 붙은 조사도 매칭되도록 endpoint별 경계를 적용했고 Latin 식별자 안의 오탐을 거부하는 회귀를 유지했다. RED·수정 전 MCP/DEV 관측은 별도 보존한다.

관련4패키지 race218시험222하위시험 pass/실패0/skip0·vet/boundary/docs/progress guard. [실제 연결 원자료](../../system/eval/b0-knowledge-system/ontology-o03-accepted-2026-10-13/verification.json). O03 A–D는 이 명시적 실제 WBFT held 파일럿 연결 범위에서 충족하며 운영 전환/전체 corpus 출시를 완료로 세지 않는다. 기존 N18 공개 배포 보류와 N16 미충족 유지 결정은 바뀌지 않았다.

## O04 후속: 실제 BGE-M3 관측 — 품질 판정 미완료

같은 후보/dataset/model/binary·질문·요청 옵션·raw-query-first에서 baseline off와 concept_text on을 비교했다. 실제4개 질문은 승인된 alias 기반 DEV probe이며 gold/블라인드 독립 질문이 아니다. 각 arm warmup2/retrieval5/warm20/cold0로108calls·총216응답/108쌍, 실패/누락/pending0. 수정 전/후 관측을 별도 고정했고 기존 FINAL은 재사용0이다. [원자료/한계](../../system/eval/b0-knowledge-system/ontology-o04-actual-preview-2026-10-13/verification.json).

수정 후 retrieval5회에서 C01–03은 매번 applied·인용 순서/본문 변화, C04는 mixed term에 매칭되지만 매번 no_effect였다. 전체108쌍 중81쌍 순위/본문 변화. source definition으로 raw candidate pool 안의 score/rank만 바뀌며 code/test 관계가 새로 증명되지는 않는다. C04의0효과를 연결 불가와 구별했다.

warm80회/arm의 aggregate p95는 off452.05ms/on588.72ms(약30.2% 높음). 요청 scope의 Ollama HTTP는1320/1653이며 그 중 /api/embed440/551, /api/tags880/1102다. startup scope는 별도이고 constructor probe의 scope 밖 요청은 이 분모에 없다. 모델 호출 전체를0/단일 embedding 수로 오독하지 않는다. 실행은 순차 off→on, 모델/cache cold 또는 randomized latency acceptance가 아니다.

이 결과는 실제 기계적 효과·비용 관측이며 검색 정답 품질 판정은 not_evaluated다. 독립 단위/gold/answer reference가 없어 MRR/Recall/F1·답변 평가·성능 수용을 계산하지 않았다. O04-B의 actual runtime 자료는 준비됐으나 독립 평가 조건·정답 범위 C와 최종 실제 평가/판정 결합 D가 남아 전체 O04 완료로 체크하지 않는다.

다음은 독립 평가 질문·인용 gold/정답·채점 단위/검토 자료를 사전에 마련하고 실제 품질·한계를 판정하는 일이다. 현재 기존17/18·온톨로지3/4 완료, 전체 잔여 N16/O04의2개다.
## O04 평가 입력 준비 — 검토 대기

2026-10-13: [검토할 질문·후보 답·실행 계약](./ONTOLOGY-EVALUATION-REVIEW-2026-10-13.md)을 별도 제안 입력으로 기록했다. 실제 baca2869 retained source에서 코드 위치 gold10개/graph canonical/vector chunk를 확인했고 원문7파일을 .go.txt/.yaml.txt로 보관했다. 색인 text는 끝 newline을 생략할 수 있으므로 원문 행 SHA와 색인 text SHA를 같은 표현으로 가정하지 않는다. canonical 빈 문자열 노드는 location receipt에 사용하지 않았다. 준비 과정에서 이 두 가정을 바로잡았으며 제품 소스 오류 또는 검색 품질 실패로 기록하지 않는다.

검색6문항의 제안 사실군4개는 기존 DEV와 겹친다. trace2문항은 missing_code/acceptance_evaluated=false를 확인하는 별도 제어이며 품질·무인용 군으로 합산하지 않는다. 입력 검사기는 proposed→approved 또는 FINAL로 이름만 바꾸는 동작, 타tuple/commit/gold 변조와 경로 탈출, 내용 승인→실행 기준 통과의 오기를 거부한다. 준비 검사 통과와 평가 실행 가능 상태를 분리했다. 새 MCP/모델 호출0, 기존 B0 동결 변경0, 운영 source/current/config 보존.

입력25·기존 위치채점17·adapter7·진행guard11 =60시험 pass. O04-A만 체크, B/C/D와 O04 전체는 미완료다. 기존17/18·온톨로지3/4·전체 잔여2개 N16/O04. 다음 내용·프로토콜·범위 판정과 별도 동결 후 off/on 측정/비용/품질 한계 기록이다. 독립 평가 요구를 유지할 때 필요한 미관측 질문/독립 단위 보강도 검토안에 명시했다.
