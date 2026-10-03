# B0 원응답·시간·벡터 오라클 도구

2026-10-03 · 도구 구현/검증 기록. 공식 질문 품질과 B0 게이트는 아직 미측정이다. 동적 입력·프로토콜·입력 범위의 사람 검토는 [실행 작업리스트](./EXECUTION-WORKLIST.md)의 B0-01–03에 남아 있다.

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

요청 원문 SHA, 실행 바이너리 SHA, 설정 SHA를 기록하고 바이너리/설정의 전후 변경을 검사한다. `quality_metrics`는 null이다. 이 명령이 정답 승인, 좌표의 전체 감사, 모델의 운영 상태, 온톨로지/정책 사실의 참을 판정하지 않는다. 실제 모델 다이제스트는 CKV/Ollama의 질의 신원 검사와 아래 프로브·상위 고정 입력 원장으로 연결한다. 내부 모델 HTTP 호출 수는 이 도구의 MCP row 수와 다르므로 footprint에서 별도 집계해야 한다.

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

승인된 프로토콜의 전체 하드웨어·입력·팩 잠금 원장, arms 순서 회전, footprint 내부 호출 수, 공식 정적/dynamic 실행, v2 integrity/좌표·권한·주장 판정, paired 통계와 사람 검토는 남아 있다. 단위 시험용 synthetic 사실과 실제 사람이 검토한 운영 사실을 합치지 않는다. 이전 구조 패키지 검증과 이 새 도구 검증은 범위가 다르며 최종 C1 패키지는 최종 코드에서 다시 확인한다.


실제 pinned BGE-M3 v2 재생에서 EOF 개행을 추가 인용 줄로 센 CKV 생성기 오류를 발견했다. 수정 전 실패 시험과 `source_missing` 5응답을 보존했다. file_header/file_full에서 마지막 개행의 sentinel을 제외하도록 수정하고 동일 소스/모델로 새 데이터셋을 생성했다. 수정 후 v2 5응답, 25본문의 실제 줄·원문 SHA·좌표·sha256-v2를 검증했다. 실제 프로브도 pinned 좌표·exact 일치·DB/manifest 무변경을 확인했다. 이 실행은 Go 검사와 겹친 진단이며 공식 latency가 아니다. 기존 전체 코퍼스의 v2 인용 유효성을 대신 증명하지 않는다. 실패/후속 전체 Go·race·vet·경계·문서 및 원응답은 [measurement-tools-m2max-2026-10-03.json](../../system/eval/b0-knowledge-system/measurement-tools-m2max-2026-10-03.json)에 해시로 연결했다.
