# B0 원응답·시간·벡터 오라클 도구

2026-10-04 · 원응답·벡터 오라클·논리/HTTP 호출 계측의 구현/검증 기록. 공식 질문 품질과 B0 게이트는 아직 미측정이다. 동적 입력·프로토콜·입력 범위의 사람 검토는 [실행 작업리스트](./EXECUTION-WORKLIST.md)의 B0-01–03에 남아 있다.

## v1/v2 원응답 캡처

`cks eval capture`는 기존 v1 인용 점수 계산과 별도로, 읽기 전용 v1/v2 `get_for_task` 요청을 실행한다. 입력은 정답 없는 JSON이다.

```json
{
  "schema_version": 1,
  "requests": [
    {"id": "alpha-v2", "tool": "cks.context.get_for_task_v2", "arguments": {"prompt": "Where is Alpha implemented?"}},
    {"id": "alpha-v2-policy", "tool": "cks.context.get_for_task_v2", "arguments": {"prompt": "Where is Alpha implemented?", "include_knowledge": true, "knowledge_as_of": "2026-10-03", "knowledge_subsystem": "fixture"}}
  ]
}
```

ID 중복, 알 수 없는 필드, 쓰기 도구, 정답 인수, 잘못된 인수 타입은 서버를 시작하기 전에 거부한다. 정책 범위 누락은 서버 오류를 시험하는 입력으로 허용한다. `include_knowledge`는 v2에서만 가능하다. 별도 명시 intent도 허용하며 유효 값을 검사한다. 사용자 지정 도구 namespace를 빌드했다면 그 바이너리의 정확한 도구 이름을 사용한다.

```bash
bin/cks eval capture --requests /private/tmp/b0-requests.json \
  --config /private/tmp/b0-mcp.yaml --output /private/tmp/b0-capture.json \
  --warmup 2 --retrieval-runs 5 --warm-runs 20 --cold-runs 3
```

출력 파일은 새 경로여야 하며 덮어쓰기를 거부한다. 하나의 warm 프로세스에서 요청별 `warmup`, `retrieval`, `warm_latency`를 순서대로 실행하고, `cold_process`는 요청별 새 MCP 프로세스에서 첫 도구 응답까지 측정한다. 모델/데몬은 내리지 않는다. 따라서 cold는 **프로세스 시작 지연**이며 모델 미상주 상태의 지연이 아니다. 반복은 독립 질문 표본으로 계산하지 않는다. 8 arms의 순서 회전은 상위 실행 도구가 담당한다.

각 row는 요청 ID·단계·반복 번호, 실제 요청 인수, 도구 호출 시간 ns, 응답 완료 시각, SDK가 해석한 전체 MCP 결과를 보관한다. 원문 본문·인용·좌표·의미 문맥과 `IsError` 구조화 오류를 유지한다. 도구 오류와 전송 실패를 구분한다. 직렬화·기록 비용은 호출 시간과 프로세스 시작→응답 시간에서 제외한다. 호출 실패와 시작 실패는 보고서에 남기고 CLI가 실패한다. `--call-timeout` 기본 90초는 초기화와 각 호출을 제한하며, 초기화 deadline 종료가 정상 서버의 프로세스 수명을 취소하지 않는다. 오류 기대 사례도 보고서는 `partial`이며, 상위 실패 오라클이 실제 오류 코드의 적합성을 판정한다.

요청 원문 SHA, 실행 바이너리 SHA, 설정 SHA를 기록하고 바이너리/설정의 전후 변경을 검사한다. `quality_metrics`는 null이다. 이 명령이 정답 승인, 좌표의 전체 감사, 모델의 운영 상태, 온톨로지/정책 사실의 참을 판정하지 않는다. 실제 모델 다이제스트는 CKV/Ollama의 질의 신원 검사와 아래 프로브·상위 고정 입력 원장으로 연결한다. `measurement_id`는 각 호출의 새 식별자이며 MCP `_meta`로 전달한다. 도구 인수나 EvidencePack에 추가하지 않는다. 아래의 선택형 계측과 연결하며, 계측을 끈 서버에서도 원응답 파일에 이 식별자를 보존한다.

## 선택형 실제 호출 계측

```yaml
logging:
    level: info
    mode: prod
    footprint_dir: /private/tmp/b0-footprint/baseline
    measure_backend_calls: true
```

이 필드는 기본 false이며 생략하면 기존 실행·응답을 유지한다. 켜려면 파일 출력, prod JSON 모드와 info/debug 수준이 필요하다. 기록이 사라지는 stderr 전용·dev·warn/error 조합은 설정에서 거부한다. 일반 composer footprint와 함께 `measurement.backend_calls` 이벤트를 쓴다. 해당 이벤트의 `summary.measurement_id`를 캡처 row의 ID에 연결한다. v1/v2 조기 오류도 outcome=tool_error와 시도 목록을 남긴다.

질의 scope에서는 serviceability health, intent 임베딩, 모든 CKV semantic_search와 CKG BM25/FindSymbol/Neighbors 호출을 실제 인터페이스 경계에서 관측한다. 별도 knowledge pass, Stage 1 keyword rerank, Stage 3 방향별 확장, 선택형 개념 텍스트 검색을 모두 포함한다. 원문/기호는 SHA와 UTF-8 크기, 옵션은 실제 K·필터·rerank/그래프 상한, 결과는 개수·성공/실패·ns 시간으로 기록한다. backend 오류 메시지나 원문 프롬프트·벡터를 이 이벤트에 복사하지 않는다. 기존 composer footprint 형식은 그대로다.

2026-10-05 FIX-21부터 Neighbors의 `input_sha256`·`input_bytes`는 실제 `contract.Citation` 인수의 Go `encoding/json.Marshal` SHA·바이트 수다. compact JSON의 필드 순서는 file/start_line/end_line/commit_hash이며 Go 기본 HTML·줄 구분자 이스케이프를 유지한다. 파일/commit 원문은 이벤트에 넣지 않으며 계측 scope가 없으면 직렬화하지 않는다. 인수 길이는 원문/네트워크 바이트가 아니다. project/dataset 전체 좌표는 이 v1 인수에 없으므로 설정/신원·원응답과 별도로 연결한다. 이전 로그의 빈 SHA·0바이트는 소급 채우지 않는다. [새 실제 20입력·4 SDK 검증](./B0-NEIGHBOR-BINDING-DIAGNOSTIC.md)은 on/off 응답 동등성·원문32개·공개 Go 계약과 실제 adapter 입력 SHA/옵션/결과의 일치를 확인했다.

Ollama는 별도의 backend=ollama_http 행으로 HTTP transport **시도**를 관측한다. 리다이렉트와 실패도 각각 세며, method/path·request body의 선언된 크기·실제로 읽은 response body 바이트·HTTP status를 기록한다. 헤더·URL host·본문·임베딩 값은 받지 않는다. 시간은 시도 시작부터 마지막 body read까지이며, JSON decode 사이의 시간은 포함하고 뒤따르는 모델 검증 조회는 해당 시도 시간에 더하지 않는다. 논리 API 행과 그 내부 HTTP 행을 합산해 하나의 “검색 횟수”로 표시하지 않는다. 모델 서버의 내부 계산·tokenizer·SQL 문 수·네트워크 헤더/wire 전체 바이트를 세는 기능은 아니다.

startup.intent_anchors scope는 classifier의 사전 anchor 임베딩과 그 HTTP 시도를 별도로 기록한다. scope 생성 전의 모델 pin/probe·index open은 호출 원장에 포함하지 않는다. 기존 cold_process 시간은 그 초기화를 포함한 프로세스 시작→첫 응답이며, 모델/데몬의 미상주 지연이 아니다. 요청별 scope는 mutex로 격리하고 시작 순번을 유지한다. handler 종료 시 반환되지 않은 시도는 in_flight/pending_calls로 보존하며 늦은 완료가 이미 출력한 기록을 바꾸지 않는다.

backend 자체 ns 시간은 계측의 입력 SHA/옵션 준비와 완료 후 파일 출력 비용을 제외한다. 전체 MCP 시간에는 서버 계측·footprint 출력 비용이 들어간다. 공식 비교는 모든 arms의 계측 옵션을 고정하고 지연 비용을 명시해야 하며, 비계측 응답과의 동등성을 별도로 확인한다. 지연 게이트는 아직 실행하지 않았다.

```bash
python3 scripts/wbs-ontology-relations-smoke.py --pack-matrix --measure-backends \
  --out /private/tmp/b0-calls-mock
python3 scripts/wbs-ontology-relations-smoke.py --pack-matrix --measure-backends \
  --out /private/tmp/b0-calls-ollama --embedder ollama --model-name bge-m3:latest \
  --ollama-url http://127.0.0.1:11434
```

합성 8경로의 mock/BGE-M3 각 48요청과 기존 v1/v2 20요청에서 ID 연결·실제 K·Stage3·원문/인용/무결성·DB 무변경을 확인했다. 실모델 첫 공개 요청의 baseline은 CKV 3회(recall2+knowledge1), CKG BM25 40회·FindSymbol4회·Neighbors20회, intent1회, health2회였다. HTTP는 GET tags10회·POST embed5회였으며 health probe도 포함됐다. text/combined는 논리 CKV1회와 HTTP tags2+embed1을 추가했다. cache 상태에 따른 후속 요청의 HTTP 수는 각 row의 실제 기록을 사용한다. 현재 raw/text K20·knowledge K6을 관측했으며 초안 retrieval_k10을 실행한 것으로 표시하지 않는다.

같은 실모델 데이터셋에서 계측 off/on의 12개 전체 SDK 응답은 같았다. 동시 scope·nonfatal knowledge 오류·Stage3·in_flight·flow 인터페이스 보존·조기 오류, HTTP 리다이렉트·503·취소와 모델 신원 불변 시험, 전체 Go·관련 race·vet·경계 검사가 통과했다. [호출 원장](../../system/eval/b0-knowledge-system/backend-calls-m2max-2026-10-04.json)에 코드/입력/바이너리/원응답/footprint SHA와 한계를 연결한다. 최종 mock/real 구조 진단은 일부 겹쳤으므로 원시 시간을 공식 지연 점수로 사용하지 않는다. 독립 재감사는 v2 106응답·116개 요청 scope·12응답 동등성을 확인했다. 응답 성공과 내부 호출 성공은 다르다. real 48요청에서 BM25 오류 96회·Neighbors 오류 384회, mock 48요청과 기존 호환 20요청에서 Neighbors 오류 각 48/20회를 원자료 그대로 보존했다. 읽기 전용 재현에서 `<convention>` 합성 경로가 FTS 문법 오류를 일으키는 기존 결함과, 문서·비심볼 header 범위에 그래프 노드가 없는 확장 실패를 확인했다. 첫 감사의 “모든 호출 성공” 가정을 수정했고 그 실패 기록도 보존했다. 이 결함 수정은 별도 작업이며, 계측 구현 통과를 내부 오류 0으로 표현하지 않는다.

## CKV 직접 후보 상한과 독립 exact 오라클

`cmd/b0-vector-probe`는 생산 CLI의 임의 튜닝 플래그를 추가하지 않고 `SearchDetailed`의 `MaxExactCandidates` API를 직접 실행하는 평가 전용 Go 도구다. 기존 체크포인트된 vector DB와 공개 sidecar를 읽는다. 모델 바이트/전처리/차원의 checksum, DB/sidecar 신원과 좌표 헤더, 게시 DB SHA를 검증한다.

```bash
go build -o /private/tmp/b0-vector-probe ./cmd/b0-vector-probe
/private/tmp/b0-vector-probe --vector-dir /private/tmp/b0-data/current/vector \
  --query 'Where is the gas refund route implemented?' \
  --filter-file /private/tmp/b0-filter.json --k 5 --max-exact-candidates 2 \
  --embedder ollama --model-name bge-m3:latest \
  > /private/tmp/b0-vector-result.json
```

위 명령은 실행 형태의 예시이며, 동적 gold 승인 전에 F-01의 공식 측정으로 사용하지 않는다. 필터 JSON은 기존 CKV `types.Filter` 형태다.

```json
{"language":"go","path":"src/*.go","symbol_kinds":["Function"],"chunk_kinds":["symbol"],"exclude_tests":true}
```

원문 질의 벡터를 한 번 만들고 같은 벡터·저장 벡터로 무필터, 정상 필터, 후보 상한 검색을 실행한다. 오라클은 후보 SQL이나 저장소 exact fallback 함수를 재사용하지 않고 모든 메타데이터·저장 벡터를 읽고 전체 필터를 적용한다. vec0와 같은 L2 거리와 ID 동점 순서로 상위 K를 계산한다. 게시 DB 밖의 미봉인 WAL/journal과 알 수 없는 필터 필드를 거부한다. 거리/ID·eligible 수·정상 결과·상한의 status/reason, 입력 질의 벡터, 단계별 시간, 모델 신원과 DB/manifest 전후 SHA를 보관한다. eligible 벡터 누락/잘못된 차원/NaN은 누락으로 숨기지 않고 실패한다.

`exact_agreement`는 정상 필터 결과와 같은 저장 벡터의 독립 오라클 일치다. 정답 질문의 Recall/MRR이 아니다. `sparse_fixture_qualified`는 오라클 상위 K의 대상 파일 중 무필터 상위 K가 놓친 파일이 있는지 보여 주는 원시 보조 조건이다. F-01 최종 판정에서는 승인된 전체 eligible 파일 목록·개수·실패 오라클을 함께 확인해야 한다. 후보 상한 결과가 `incomplete/candidate_limit`인 것은 프로브 실행 실패가 아니라 평가할 실제 검색 상태다.

`OpenReadOnly`는 기존 DB만 열며 자동 마이그레이션과 schema 변경을 하지 않는다. 누락 DB 생성, 벡터/manifest 쓰기, 차원 불일치, 변경된 마이그레이션을 거부한다. 기존 `Open`도 파일 경로를 URI 이스케이프하여 `?`·`#`가 포함된 실제 파일을 올바르게 연다. URI를 사용자 설정 문자열로 전달하는 계약은 추가하지 않았다.

## 확인한 범위와 남은 일

단위/실행 검증: 정답/쓰기 요청 거부, 중복/추가 payload 거부, v2 원문·좌표·의미·구조화 오류 보존, 전송 실패·시작 실패 보고, 파일 덮어쓰기 거부, 같은 저장 벡터의 희소 필터/테스트 제외/동점/상한 오라클, 누락 벡터와 게시 DB 변조 거부, 읽기 전용 DB의 쓰기·생성 거부와 `?`·`#` 파일명을 확인했다. 실제 mock MCP는 21 rows를 정상/정책/범위 오류로 수집했다. 기존 Alpha 소형 Ollama 입력의 직접 프로브는 BGE-M3 digest·1024차원, exact 일치, DB 무변경을 확인했다.

승인된 프로토콜의 전체 하드웨어·입력·팩 잠금 원장, arms 순서 회전, 승인 K 적용, 공식 정적/dynamic 실행, v2 integrity/좌표·권한·주장 판정, paired 통계와 사람 검토는 남아 있다. 단위 시험용 synthetic 사실과 실제 사람이 검토한 운영 사실을 합치지 않는다. 이전 구조 패키지 검증과 이 새 도구 검증은 범위가 다르며 최종 C1 패키지는 최종 코드에서 다시 확인한다.


실제 pinned BGE-M3 v2 재생에서 EOF 개행을 추가 인용 줄로 센 CKV 생성기 오류를 발견했다. 수정 전 실패 시험과 `source_missing` 5응답을 보존했다. file_header/file_full에서 마지막 개행의 sentinel을 제외하도록 수정하고 동일 소스/모델로 새 데이터셋을 생성했다. 수정 후 v2 5응답, 25본문의 실제 줄·원문 SHA·좌표·sha256-v2를 검증했다. 실제 프로브도 pinned 좌표·exact 일치·DB/manifest 무변경을 확인했다. 이 실행은 Go 검사와 겹친 진단이며 공식 latency가 아니다. 기존 전체 코퍼스의 v2 인용 유효성을 대신 증명하지 않는다. 실패/후속 전체 Go·race·vet·경계·문서 및 원응답은 [measurement-tools-m2max-2026-10-03.json](../../system/eval/b0-knowledge-system/measurement-tools-m2max-2026-10-03.json)에 해시로 연결했다.

2026-10-04 FIX-07 수정 후 검증: broad recall의 invariant/convention 청크를 코드 검색어로 사용하지 않도록 수정했다. 지식 증거·별도 pass·raw K/필터를 유지했고, 같은 실모델 데이터셋의 8경로 48개 전체 SDK 응답은 수정 전과 같았다. BM25 오류 96→0, 출처/integrity·정책 상태·DB 무변경 재감사 통과. 문서/header의 비심볼 Neighbors 실패는 보존했다. [수정 원장](../../system/eval/b0-knowledge-system/fix07-knowledge-keywords-m2max-2026-10-04.json). 공식 품질은 미판정이며 다음 작업은 arm 회전과 승인 K 적용 경로다.


## 공유 recall K 설정 경로

```yaml
retrieval:
    recall_k: 10
```

생략/0은 기존 raw/concept-text K20을 유지하고 1..1000의 요청 상한을 허용한다. 이 값은 Stage 1 각 recall과 선택형 concept_text/combined 검색에 같이 전달한다. Stage 2 union 후보 cap30, CKG keyword별 K, 별도 knowledge pass K6과는 다르다. 기본 생성 YAML에는 새 필드를 출력하지 않는다. `--recall-k`가 추가된 합성 스모크는 설정 값과 실제 호출 원장을 함께 확인한다.

```sh
python3 scripts/wbs-ontology-relations-smoke.py --pack-matrix --measure-backends --recall-k 10 --embedder ollama --out /private/tmp/ks-recall-k10-real-20261004
```

실모델 8경로 48요청에서 raw/text K10과 knowledge K6을 확인했고, 설정 생략의 기존 v1/v2·오류 폴백 20요청은 K20을 유지했다. 설정 범위/기본 YAML, composer의 실제 raw 호출·별도 지식 예산, v2 출처/integrity·정책·보존 DB를 검사했다. [원장](../../system/eval/b0-knowledge-system/recall-k-m2max-2026-10-04.json). 이는 연결 경로 진단이며 초안 K10/프로토콜 승인이나 공식 Recall@10·지연 통과가 아니다. 공식 실행에서 승인된 K를 선택하고 arm 순서를 회전해야 한다.

## 8경로 회전 캡처

```sh
cks eval matrix --requests request-only-v2.json --config base.yaml --output new-private-directory \
  --warmup 2 --retrieval-runs 5 --warm-runs 20 --cold-runs 3
```

요청은 기존 `capture`의 schema_version=1 형식이며 v2만 허용한다. `include_knowledge`는 입력에서 생략하고 도구가 off/on 축으로 설정한다. prompt·intent·지식 날짜/범위는 모든 경로에 그대로 전달한다. gold/expected-answer 필드·v1·중복 ID·추가 JSON은 거부한다. 잘못된 범위 요청도 오류 오라클용으로 기록할 수 있다.

기본 설정 하나에서 baseline/concept_text/relations/combined × off/on 8개 프로파일을 만든다. 모드와 관측 로그 경로, 요청의 팩 boolean만 달라진다. stdio와 backend 계측을 켜며 기존 prod/info/debug 요건을 검사한다. K·필터·모델·소스·저장소·팩은 같고 원래 설정 파일은 보존한다. 출력은 새 0700 디렉터리여야 하며 고정 소스·데이터셋 안이나 그 별칭에 만들 수 없다. 프로파일·보고서·원응답은 0600으로 쓴다.

warm 세션 8개를 순차 초기화하고 실제 호출은 한 번에 하나만 실행한다. 고정 경로 목록은 baseline_off/on, concept_text_off/on, relations_off/on, combined_off/on이다. warmup → retrieval → warm_latency → cold_process를 구분하고, 각 단계에서 질문별 반복 하나가 8개 경로의 한 group이다. 전체 group 번호를 8로 나눈 나머지만큼 시작 경로를 왼쪽으로 회전한다. 8 group마다 각 경로가 각 위치에 한 번 온다. group·position·sequence·질문/반복 ID를 행에 보존한다. warm 세션은 마지막에 닫고 cold는 매 행마다 새 프로세스를 생성·종료한다. cold 시간은 시작부터 첫 도구 응답까지이며 모델/데몬 미상주 시간은 아니다.

`rows.jsonl`에 SDK 전체 원응답과 measurement_id·ns 시간·오류를 응답마다 직렬화·sync한다. 기록 비용은 도구 시간과 cold 시작→응답 시간에서 제외된다. 파일 기록은 다음 호출 전의 간격에 포함된다. `report.json`은 최초 running이며 종료 후 captured/partial로 갱신하고 rows SHA를 기록한다. 초기화에 실패한 경로는 `warm session unavailable` 오류 행으로 남기며 다른 경로를 계속 수집한다. 도구/전송 오류도 raw 행에 남긴다. 취소는 새 호출을 멈추고 세션을 닫는다. 강제 종료가 running 보고서를 남기면 완료로 판정하지 않는다.

native v2 version layout과 정렬·보관 원문을 먼저 검증한다. 엔진 DB/manifest·dataset identity·보관 source/Git archive 전체 파일, 존재하는 선택된 live repo 파일, 의미 저장소·glossary·sanitize 설정·요청·바이너리·추가 `--lock-file`을 전후 해시한다. 원본 HEAD와 live 실행 비트, 생성 프로파일도 전후 비교한다. live 파일 부재는 missing으로 보존하므로 오래된/결손 fixture를 사용할 수 있다. 입력 읽기와 초기 해시 사이의 변경도 거부한다. 변경·초기화·도구·전송·취소 오류가 있으면 partial로 보존하고 CLI는 실패한다. 이 경계 검사로 실행 중 잠깐 변경 후 원복하는 외부 행위까지 증명하지는 않는다.

보고서의 기본 runtime은 Go·OS·arch·논리 CPU 수다. 아래 선택형 환경 원장이 머신·모델 daemon·입력 사본의 전후 기록을 연결한다. 승인 입력/protocol의 실제 판정과 공식 실행은 별도다. `captured`는 원응답 수집 완료이며 v2 주장·출처·정답·품질·운영 출시 합격을 의미하지 않는다. 반복 수를 독립 질문 표본으로 계산하지 않는다.

회전 도구의 실제 초기화 실패 시험에서 typed-nil cold 세션 처리를 회귀 시험으로 고정하고 수정했다. SDK의 `Close`는 프로세스마다 2초 graceful 대기 후 SIGTERM, 이어 3초 대기 후 kill과 추가 3초 대기를 수행한다. `--call-timeout`은 initialize/호출 deadline이며 전체 행 처리나 종료 정리 시간의 상한이 아니다. cold 초기화 실패 행의 시간은 factory가 오류·정리를 끝낼 때까지이고 첫 응답이 없으므로 정상 cold 지연 표본으로 합산하지 않는다. 50ms deadline·56개 실패 프로세스의 약 116초 전체 시간은 이 정리 정책을 포함한다. 초기 전체 20초 검증 가정은 잘못되어 수정하고 실패 가정 기록을 보존한다.

## 선택형 환경·입력 원장

```sh
bin/cks eval matrix --requests /private/tmp/b0-requests.json \
  --config /private/tmp/b0-base.yaml --output /private/tmp/b0-matrix-ledger \
  --environment-ledger --environment-note 'Development diagnostic; external competing work not excluded.' \
  --lock-file system/eval/b0-knowledge-system/questions.json \
  --lock-file system/eval/b0-knowledge-system/protocol-m2max-draft.json \
  --lock-file system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json
```

명령은 개발 입력의 실행 형태다. 최종 입력은 실제 사람 승인과 동결 후 사용한다. 기본 반복 수는 기존 matrix 계약을 유지한다. `--environment-ledger`를 생략하면 기존 경로를 유지하며 `--environment-note`는 이 플래그와 함께 사용한다.

조회 전에 CPU 신원·보이는 논리 CPU 수·RAM·OS, load·memory/swap·thermal 상태와 숫자로만 된 process 압력을 관측한다. 프로세스의 이름·인자·환경 변수는 보관하지 않는다. query/SDK 세션 종료와 입력 감사 후 같은 정보를 다시 기록한다. macOS arm64와 Go·ps 없는 Linux arm64 컨테이너에서 실제 실행을 검증했다. Linux amd64는 에뮬레이션 검증과 네이티브 호스트 검증을 구분한다. thermal 정보는 선택형이며 필수 환경 정보를 얻지 못하면 partial로 남기고 조회를 시작하지 않는다.

Linux ARM의 `/proc/cpuinfo`에 `model name`/`Hardware`가 없으면 실제 implementer/architecture/variant/part/revision 식별자를 기록하며 제품명을 추정하지 않는다. `ps` 대신 `/proc/<pid>/stat`의 CPU lifetime 평균·RSS와 PID namespace에서 읽을 수 있는 프로세스 수를 사용한다. clock tick은 `getconf CLK_TCK`, RSS page 크기는 OS에서 읽으며 실패하면 임의 상수를 넣지 않는다. macOS의 `ps` 수치와 측정 창이 다르다. Linux CPU/RAM은 커널에 보이는 전체 값이고, 별도 `resource_limits`는 노출된 cgroup v2 self·상위 그룹 및 통상 v1/v2 mount root 값이다. 자원 제한 값의 전후 변경은 partial이다. 이 정보는 실제 가용량이나 자원 독점을 보증하지 않는다.

Ollama는 데이터셋 신원의 모델·digest·차원·runtime context/batch에 설정을 대조하고, local loopback HTTP의 version/tags·strict native embed 1회·tags 재확인으로 실제 바이트와 차원을 검증한다. 이 프로브는 전후 각 1회이며 질의 시간·backend 카운터 밖이다. metadata GET은 3초, embed는 90초 상한이고 취소 문맥을 따른다. redirect와 원격/인증/추가 경로 endpoint를 거부한다. 선택 모델의 residency와 다른 resident 모델 개수만 보관한다. mock은 데이터셋 선언을 확인하며 기존 checksum 형태도 정확히 검증한다. ONNX/CoreML의 새 환경 프로브는 구현하지 않았고 기존 기본 matrix 경로는 유지한다.

모델·환경 신원이 전후 달라지거나 필요한 후속 관측을 얻지 못하면 원응답을 보존한 partial이다. load/swap/process 압력 변화는 원시 관측으로 남긴다. 두 snapshot이 자원 독점이나 중간 변경 부재를 증명하지 않는다. `environment-note`도 검증되지 않은 작업자 설명이다.

`--lock-file` 원문을 private `locked-inputs/`에 복사하고 원본/사본의 전후 SHA를 연결한다. 파일당 8 MiB 상한이며 출력 파일 권한은 0600, 디렉터리는 0700이다. 정답·프로토콜·동적 초안의 사본은 검색 요청에 전달하지 않으며 복사가 사실/프로토콜 승인으로 해석되지 않는다.

2026-10-04 [환경 원장 재검증](../../system/eval/b0-knowledge-system/environment-ledger-m2max-2026-10-04/summary.json): mock/BGE-M3 각각 개발 제어 `alpha` 1개·24 SDK 응답·16 startup scope. 48개 응답은 기존 해당 모델 응답과 같았고 출처/integrity·정책·회전·실제 K/measurement 연결·입력 사본을 감사했다. 실제 모델 불일치는 0행·startup footprint 없음·partial로 보존했다. 초기 mock 신원 표현 불일치도 원자료를 남기고 호환을 보완했다. 이 반복은 독립 질문 표본을 늘리지 않으며 품질 지표는 null이다. 비심볼 Neighbors 실패 real 192/mock 24회는 기존 best-effort 범위로 그대로 보존했다.

2026-10-04 [Linux 최소 런타임 원장](../../system/eval/b0-knowledge-system/linux-environment-m2max-2026-10-04/summary.json): ARM CPU 이름 부재·`ps` 미설치로 발생한 실제 환경 preflight 실패를 보존하고 수정했다. Linux arm64와 amd64 에뮬레이션에서 각각 3프로젝트·72 SDK 응답·48 startup scope, 1 CPU/512 MiB cgroup·mock64·이전 스냅샷 롤백 인용·파일/본문/줄/integrity·입력 사본·호출 연결을 검증했다. 재현 도구는 `scripts/wbs-linux-environment-smoke.py`다. Go·ps 없는 cgroup v2 최소 컨테이너와 빌드된 세 바이너리를 필요로 하며 기존 출력 디렉터리를 거부한다. 의미 저장소 미구성의 폴백 검증이며 공식 품질·네이티브 amd64·Linux 실모델 비용을 입증하지 않는다.

정적 공식 입력의 개발/최종 경계는 [B0-STATIC-INPUT-PREFLIGHT.md](./B0-STATIC-INPUT-PREFLIGHT.md)의 준비기로 검사한다. 현재는 프로토콜·동적 입력 미승인 때문에 pending이다. 개발 4개 v1 진단 파일만 준비하며 최종 export와 ready 발행을 거부한다. 이 준비기는 v2 날짜/subsystem이나 실제 runtime K를 추정하여 matrix 요청을 발행하지 않는다. 입력 정의 승인, 실제 실행 readiness, 전체 B0 품질 판정은 각각 확인한다.
