# dogfood zero-recall 시나리오 진단 (2026-07-28)

Status: Tier-3 진단 기록. 전체-코퍼스 dogfood 재측정
(`2026-07-28-gap-integration-design.md` §1.3 후속 F1)에서 recall 0이었던
4개 시나리오의 원인 분석과 처분.

## 요약

recall 0의 원인은 검색 실패가 아니었다 — **4개 시나리오 전부에서
get_for_task가 기대 파일을 실제로 인용하고 있었다**(stdio 직접 질의로 확인).
0이 된 것은 `match_mode: overlap`의 라인-범위 채점에서, 서로 다른 두 원인:

| 모드 | 원인 | 해당 시나리오 | 처분 |
|---|---|---|---|
| A | **기대 스팬 stale** — 시나리오의 라인 번호가 구 cks repo 시점 기준. 통합·이후 커밋으로 심볼이 이동(예: `BM25Search` 140→190, `nodeToCitation` 318→886) | ckg-bm25-translation, concurrency-safety-real-adapters (+composer의 wrap 스팬) | **본 변경에서 재앵커링** — G6 이관(경로만 치환) 시 라인을 안 고친 미완분의 완결 |
| B | **인용 스팬 정밀도** — 옳은 파일이지만 `1-50` 헤더 청크로 인용됨. `composer.go` 기대 54-57(ErrFailClosed var)과 4줄 차이, `intent.go` 기대 129-141(IntentQAReview) | composer-err-fail-closed, qa-review-intent | **retrieval 품질 트랙으로 인계** (아래 증거) |

## 결과 (재앵커링 후, 동일 인덱스 재측정)

- ckg-bm25-translation: 0 → **R=0.50** (`nodeToCitation` 886-893 인용이 정확히 매칭)
- concurrency-safety-real-adapters: 0 → **R=0.50** (ckvclient Real doc 42-55 ↔ 반환 1-50 겹침)
- composer-err-fail-closed / qa-review-intent: 0 유지 (모드 B)
- **전체 9개: avg recall 0.444 → 0.556, nonzero 5/9 → 7/9** — 측정 좌표
  수정만으로의 개선(검색 동작 무변경)

## 모드 B 증거 (retrieval 품질 트랙 입력)

동일 인덱스(모듈 루트 1,399파일 bge-m3 + system 클로저 graph)에서
`cks.context.get_for_task` 직접 질의:

- prompt "composer ErrFailClosed sentinel and the Compose-level error wrap"
  → `internal/system/composer/composer.go:1-50` 인용 (기대: 54-57의 var,
  222-227의 wrap). 8개 인용 중 5개가 문서(.md).
- prompt "QAReview intent definition and how it routes Stage 1 and Stage 2
  retrieval" → `pkg/system/contract/intent.go:1-50` 인용 (기대: 129-141).
  18개 인용 중 상당수가 archive 문서.

가설(검증 미완): 파일 헤더(1-50) 스팬은 ckv의 header/doc 청크가 심볼 청크를
누르고 랭크되거나, 파일 단위 히트가 헤더 스팬으로 성형되는 경로가 있다 —
심볼-앵커드 스팬(886-893처럼 정확한 사례도 공존)과 헤더 스팬이 혼재하는
것이 관찰 사실. archive/설계 문서가 코드 심볼 질의에 다수 인용되는 것도
같은 트랙의 랭킹 이슈.

## 재현

```sh
# 인덱스: make -C system dogfood-eval USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=bge-m3 ...
cd system && ./bin/cks-eval -scenarios eval/scenarios \
  -cks-mcp ./bin/cks-mcp -config cks-dogfood.yaml -output /tmp/report.json
```

직접 질의 스크립트(stdio JSON-RPC, structuredContent 파싱)는 세션
스크래치에서 사용 — 요지: initialize → `cks.context.get_for_task`
`{"prompt": ...}` → `structuredContent.citations[].{file,start_line,end_line}`.
