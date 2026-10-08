# B1 비교 보고 도구와 승인 경계 재검증

2026-10-04 · B1-04/06/08 도구 준비 · **공식 B0/B1 평가와 사람 판정은 대기**.

`scripts/b1-summarize-matrix.py`는 보관한 `report.json`, `requests.json`, `rows.jsonl`, arm 설정, backend 원장과 소스 바이트를 읽는다. MCP·모델·빌드 호출을 하지 않고 기존 증거를 덮어쓰지 않는다. [원장](../../system/eval/b0-knowledge-system/paired-report-preparation-m2max-2026-10-04/summary.json)에 구현·검사·입력 SHA와 제한을 연결한다.

## 완료 재감사에서 발견한 FIX-16

정적 입력 준비기를 다시 확인했더니 프로토콜·동적 manifest/사례의 `approved` 메타데이터만으로 최종 export가 통과했다. 기존 사람 결정 `검토 후 결정`은 이 분기에서 사용되지 않았다. 수정 전 회귀가 `approved_input_definition`을 반환하는 실제 실패를 보존했다.

이제 `human-review`의 `protocol_and_dynamic_fixture_decision`에 실제 `status=approved`, 검토자·시간대 있는 `reviewed_at`, 정확한 `protocol_sha256_after`와 `fixture_manifest_sha256_after`가 있어야 각각의 입력 승인을 결합한다. 입력 상태·검토 메타데이터 검사도 유지한다. 승인 후 입력이 바뀌면 재검토가 필요하다. 이 기록은 로컬 감사 식별자이며 법적 신원 인증이나 전자 서명은 아니다.

정적 v2 scope도 사람이 남긴 `static_v2_query_scopes` 결정의 상태·검토자·시각·`scope_sha256_after`를 모두 요구한다. 수정 전에는 해시가 같으면 `pending` 결정도 `scope_review_record_bound=true`가 되는 회귀를 재현했다. 현재 실제 원문 질문집·프로토콜·동적 입력·사람 기록·scope의 5개 파일은 기존 HEAD 바이트와 같다. 승인 기록을 추가하거나 일반 진행 지시를 개별 승인으로 바꾸지 않았다.

두 준비 검사와 비교 보고기의 실제 최종 경로는 승인 대기를 유지한다. `--require-ready` 최종 입력 발행 및 정적 개발/최종 비교 보고 CLI는 exit 2이고 해당 입력/평가 출력은 생성되지 않는다. 시험 승인 경로는 메모리·임시 파일의 `synthetic-test-only` 기록으로 검사했다. repository의 최종 질문으로 모델을 실행하거나 튜닝하지 않았다.

## 보고기의 검사와 통계 범위

- 캡처 원문 해시·행 수, 고정 8 arm의 회전·질문·phase·반복·인자·measurement ID를 검사한다. retained arm 설정의 SHA와 ontology mode도 대조한다. captured에서 누락된 슬롯은 거부하고 partial의 누락은 명시한다. 바이너리·잠금·HEAD·실행 비트·설정 전후 변화가 있으면 complete 분석으로 분류하지 않는다.
- v2 좌표·commit·query·canonical integrity, 보관 파일 전체/인용 줄/본문 SHA를 검사한다. 경로 이탈·외부 source symlink·잘못된 범위를 거부한다. 실제 참조 소스만 해시로 기록하며 `.git` 등 무관한 파일은 수집하지 않는다.
- 인용 중복은 파일/줄/commit 기준으로 제거한 뒤 순서를 유지하고 상위 10개를 채점한다. 줄 겹침과 commit 일치를 요구한다. 공개 Go 평가기처럼 MRR은 **각 기대 인용의 첫 순위 역수의 평균**이다. 최종 구성된 인용의 순위이며 raw CKV 후보 Recall@10을 측정한 값은 아니다.
- 질문별 검색 반복의 중앙값을 구한다. 같은 질문의 범위/언어/상태 변형은 독립 unit 안에서 평균으로 묶고, unit 단위 paired delta를 bootstrap한다. seed 20261003·10,000 resample·95% descriptive 구간을 기록한다. 군별 독립 unit 10개 미만은 반복 수에 관계없이 `inconclusive`다.
- 실패·누락은 계획된 검색 분모에서 0으로 남긴다. transport/도구/무결성/시간 오류를 표시한다. v1의 인용 0개 guard는 명시된 사례에만 적용하며 사람의 답변 기권·정책 판단과 별도다.
- warmup·retrieval·warm latency·cold-process를 분리한다. warm p50/p95는 성공한 warm 호출에 한정된 nearest-rank이며 계획/관측/실패/누락 분모를 함께 기록한다. cold는 새 MCP process 시작부터 응답까지이며 resident 모델을 유지한 조건이다. 모델 cold load를 측정하지 않는다.
- backend query scope를 SDK measurement ID에 연결하고 ordinal·중복·다른 capture ID를 검사한다. logical backend와 Ollama HTTP 시도, 실제 K, 내부 nonreturned/pending을 분리한다. startup/constructor/probe는 query 밖이다. 누락 계측은 null/missing이며 0으로 만들지 않는다. SQL/모델 내부 작업까지 센 값은 아니다.
- SDK 크기는 JSON을 canonical 재인코딩한 UTF-8 바이트 수다. 원래 wire/frame 크기는 아니다. HTTP body 관측 크기는 별도다. build 비용·사람 검토 비용·답변/주장/정책 verdict는 이 reader에서 만들지 않는다.

`status=descriptive`는 계획된 캡처를 기술할 수 있다는 뜻이다. 개별 실패가 있을 수 있고 품질 합격을 의미하지 않는다. `quality_metrics=null`, `official_gate_verdict=pending`, 사람 verdict/cost null을 유지한다.

정적 입력을 읽는 경로는 선택된 개발/최종 partition만 허용하고 실제 승인·scope·질문/조회 인자·프로토콜 반복·모델 pin·K10 설정을 결합한다. 참조 소스 bytes는 고정 Git commit의 blob과도 대조한다. 입력 정의 승인을 전체 B0 readiness로 확대하지 않는다. 새 strict 빌드/소스 범위/의미 사실/환경 preflight와 동적 가족의 상태·안전·budget 오라클, raw CKV 순위, 사람 판정은 공식 실행의 남은 선행 조건이다.

## 기존 진단 원자료의 실제 재생

| 보관 캡처 | SDK 응답 | 질문 변형 / 독립 unit | 계측 K | 내부 nonreturned 시도 | 판정 |
|---|---:|---:|---|---:|---|
| matrix-capture · BGE-M3 | 288 | 6 / 1 | 10, 6 | 2,304 | descriptive / inconclusive |
| matrix-capture · mock | 288 | 6 / 1 | 20, 6 | 288 | descriptive / inconclusive |
| environment-ledger · BGE-M3 | 24 | 1 / 1 | 10, 6 | 192 | descriptive / inconclusive |
| environment-ledger · mock | 24 | 1 / 1 | 20, 6 | 24 | descriptive / inconclusive |

총 624개 응답의 소스·본문·integrity·query 연결이 유효했고 SDK/evidence 실패·누락 backend scope는 0이다. 내부 best-effort 실패 시도는 별도 유지했다. 서로 다른 모델·K·바이너리·환경·반복 조건의 캡처를 하나의 paired 실험으로 합산하지 않았다. 지연과 구성 인용 delta는 각 보고서에 있지만 공식 회귀 임계치의 합격 판정에 사용하지 않는다.

제어 gold는 이전에 작성된 `main.go` 3행의 Alpha 정의에 근거한다. 여섯 policy scope 변형은 같은 코드 질문의 unit이다. 팩의 정책 효력·의미 사실을 승인하거나 여섯 독립 정책 질문으로 세지 않는다. 118개 보관 입력/설정/로그/소스/gold의 전후 SHA가 같다. source는 이전 소형 fixture이고 전체 knowledge-system corpus의 최종 평가가 아니다.

42개 Python 회귀가 통과했다: B0 preflight 8, 정적 입력 11, 정적 v2 scope 7, 비교 보고 16. B0 preflight의 최초 localhost bind 두 오류는 sandbox 제약이었고, 합성 localhost 서버 실행 권한으로 재실행해 8개가 통과했다. 실제 모델이나 외부 HTTP를 새로 호출하지 않았다. Go 코드 변경은 없다.

## 재현

기존 진단 원자료를 새 디렉터리에 재생한다. 기존 결과가 있으면 거부한다.

```sh
python3 system/eval/b0-knowledge-system/paired-report-preparation-m2max-2026-10-04/replay-retained.py \
  --out-dir /private/tmp/b1-retained-replay-new
```

아래 정적 경로는 현재 승인 대기로 exit 2이고 보고서를 생성하지 않는다. 보관 소형 캡처를 공식 정적 캡처로 사용할 수 있다는 뜻이 아니다. 승인 이후에는 정확한 partition의 새 캡처와 소스를 제공해야 한다.

```sh
python3 scripts/b1-summarize-matrix.py \
  --capture-dir system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04/real \
  --gold system/eval/b0-knowledge-system/questions.json \
  --source-root /private/tmp/ks-b0-m2max-corpus-20261003 \
  --partition final --output /private/tmp/b1-final-new.json
```

다음 P0는 B0-01/02/03/05와 STV2-01의 실제 사람 결정이다. 결정 뒤 소스 범위·팩/semantic/조회 scope·K를 동결하여 B0-07 새 strict 빌드 → B0-08/09 → B1 공식 paired 비교·사람 판정 → 실제 C0 실패 수정 → 최종 C1 재평가로 진행한다. OP-01–08 운영 결정과 native amd64 범위도 남아 있다. 작업리스트 전체는 미완료다.
