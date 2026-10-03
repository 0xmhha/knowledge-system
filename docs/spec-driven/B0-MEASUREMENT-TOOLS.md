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
