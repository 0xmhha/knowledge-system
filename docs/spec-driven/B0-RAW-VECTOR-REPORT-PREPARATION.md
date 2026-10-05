# Raw CKV 순위·F-01 보고 준비

2026-10-05. B0-06/B1-04의 진단 도구 준비이며 공식 기준선·F-01 판정은 미완료다. [작업리스트](./EXECUTION-WORKLIST.md)와 [진행 현황판](./EXECUTION-STATUS.md)을 함께 따른다.

`scripts/b0-summarize-vector-probe.py`는 보관된 직접 CKV 프로브를 읽는다. 모델·DB·MCP를 실행하지 않는다. 입력 probe 원문 SHA, 실제 K/필터/질의/상한/모델/좌표, sidecar 전후 SHA·DB seal, 모델 사후 검증을 연결한다. sidecar의 전체 indexed source inventory와 보관 파일 SHA를 대조하며, parser-node text SHA와 선언 줄 안의 바이트 포함을 검사한다. parser text와 전체 줄 SHA를 구분하고 열 단위 AST 위치의 정확성까지 주장하지 않는다. DB seal은 보관 probe의 관측이며 이 reader의 새 DB 검사가 아니다.

## 원시 후보와 점수의 의미

무필터·정상 필터·상한 검색을 따로 기록한다. 원시 chunk ID·파일·줄·commit·1-based 순위·거리·normalized 값, CandidateCount/EligibleCount/SearchedCount와 status/reason을 보존한다. CandidateCount는 SQL prefilter의 상위 집합으로 exact eligible 수와 같다고 가정하지 않는다. null eligible 값은 0으로 바꾸지 않는다.

진단 `raw_ckv_source_span_recall_at_k`는 기대 source span 중 같은 commit/파일에서 겹치는 원시 후보를 찾은 비율이다. precision은 반환된 원시 후보 중 기대 span과 겹치는 비율이고, 역순위는 각 기대 span의 첫 원시 순위 역수 평균이다. distinct chunk가 같은 줄을 가리켜도 후보 위치를 지우지 않는다. 구성 EvidencePack의 인용 dedup/top10 점수와 구분한다. K1을 @10으로 바꾸거나 비어 있는 gold에서 기권 성공을 추론하지 않는다. 공식 질문/답변/정책 점수는 계속 null이다.

전체 probe 오류는 planned=1/failed=1로 남기고 점수·시간을 만들지 않는다. 완료되지 않은 검색의 부분 후보·사유도 보존한다. malformed JSON/NaN/중복 ID/순위·출처/입력 SHA 불일치는 거부한다. CLI는 새 보고서만 mode 0600으로 만들며 기존 입력/출력을 덮어쓰지 않는다. exit 0은 재생 완료, exit 2는 보존된 측정 실패·제어 실패/불변식 위반, exit 1은 입력/출력 거부다. 자격 미충족 보고도 exit 0일 수 있으므로 F-01 state를 반드시 읽는다.

## F-01의 자격과 검색 결과

`cmd/b0-vector-probe`의 독립 exact scan에 `oracle.eligible_hits`를 추가했다. 저장 벡터·전체 `types.Filter.Matches`로 계산한 모든 eligible ID/파일/줄/거리를 거리·ID 순서로 보존하고 별도의 `oracle.hits`만 K로 자른다. 기존 검색 동작과 eligible 벡터 누락 거부는 유지한다.

reader는 explicit diagnostic control의 K/상한·전체 eligible 파일/개수·완전 반환 수와 full inventory를 대조한다. 완전한 무필터 상위 K가 exact target 파일을 적어도 하나 놓쳐야 희소 조건을 만족한다. 목록 누락·다른 K/상한·자격 미충족은 `fixture_not_qualified`다. 자격을 갖춘 진단에서 정상 검색의 ID/파일/줄/거리 exact 일치와 상한 검색의 `incomplete/candidate_limit`·실제 상한 초과를 별도 검사한다. `diagnostic_controls_pass/fail`은 공식 F-01 판정이 아니다. 오래된 helper의 ID/거리 boolean만 믿지 않고 source 위치까지 재비교한다.

현재 reader는 정확한 경로 또는 single-star 경로와 공개 test-support/commit 필터를 지원한다. 다른 glob은 명시적으로 거부하며 지원 확대에 Go Matches 대조가 필요하다. 전체 eligible 목록은 Go 독립 scan의 보관 결과이지 reader가 DB에서 새로 계산한 결과가 아니다. 공식 입력·새 strict 빌드·승인된 DEV/FINAL collector 결합은 별도로 남아 있다. `diagnostic_only=true` 외의 controls는 거부한다.

## 실제 보관 자료 재검증

[원장](../../system/eval/b0-knowledge-system/raw-vector-report-preparation-m2max-2026-10-05/summary.json)에 역사적 Alpha BGE-M3 probe를 원문 그대로 연결했다. sidecar와 source archive의 3개 파일을 복사했고 원래 source commit `56ac119f8555ef5202cb92dfaea8a3a3b907795d`의 Git blob과 모두 같다. indexed source 2개를 reader가 검사했다. 실제 K1·eligible1·세 검색 complete·normal exact 일치, 희소 helper=false이며 과거 자료에는 full eligible 목록이 없다. 작성한 Alpha 단일 대상 diagnostic controls는 F-01 DEV/FINAL gold가 아니다. 결과는 `fixture_not_qualified`이고 공식 품질·F-01 verdict·사람 판정은 null이다. 역사적 지연도 기존 병행 진단 조건을 유지한다.

초기 CLI가 `Path.open(opener=...)`를 사용해 실패한 로그를 보존했다. builtin `open`으로 수정하고 15개 새 Python 회귀, 기존 paired 16개, Go probe 6개/race/vet를 통과했다. 새 Go 회귀는 K2 밖의 세 번째 eligible 파일이 full inventory에 남는지 확인한다. source 변조·잘못된 exact 위치·full 목록 누락/추가·상한 상태/카운터·부분 무필터·잘못된 K·test support·중복 ID·출력 덮어쓰기 등의 실패 오라클을 포함한다. 이 도구 개발 중 실패를 기존 제품의 C0 실패로 세지 않는다.

재생 예시(기존 report를 덮어쓰지 않는 새 경로):

```sh
python3 scripts/b0-summarize-vector-probe.py \
  --probe system/eval/b0-knowledge-system/raw-vector-report-preparation-m2max-2026-10-05/historical-replay-sealed/probe.json \
  --manifest system/eval/b0-knowledge-system/raw-vector-report-preparation-m2max-2026-10-05/historical-replay-sealed/manifest.json \
  --controls system/eval/b0-knowledge-system/raw-vector-report-preparation-m2max-2026-10-05/historical-replay-sealed/controls.json \
  --source-root system/eval/b0-knowledge-system/raw-vector-report-preparation-m2max-2026-10-05/historical-replay-sealed/source \
  --out /private/tmp/b0-raw-replay-new.json
```

다음 독립 작업은 F-02의 장문 꼬리·split/parent 재조립 오라클과 보관 원자료 보고 연결이다. B0-01/02/03/05·STV2-01의 실제 결정이 도착하면 source/pack/semantic/scope/K 동결과 B0-07 새 strict 빌드를 우선한다. 공식 B0/B1/C0/C1·사람 판정·OP-01–08/native/legal/운영 출시 범위는 계속 남아 있다.
