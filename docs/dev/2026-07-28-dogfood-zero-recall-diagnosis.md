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

## 모드 B 해소 (2026-07-28, 후속 R2/R3)

가설이 아니라 **청킹 공백**이 진짜 원인이었다. ckv 인덱스 직접 질의 + chunks
테이블 조회로 확정: Go 파서(`internal/vector/parse/golang`)가 top-level
`const`/`var` GenDecl을 스팬으로 방출하지 않았다 — "file_header(첫 50줄)
폴백이 커버한다"는 설계 가정이 50줄 밖의 선언(`ErrFailClosed` 55-58,
`IntentQAReview` 129-141)에서 붕괴. 해당 코드는 **검색 자체가 불가능**했고,
FileHeader(1-50) 인용은 "그 파일에 존재하는 유일한 관련 청크"였기 때문.

수정: const/var 블록당 1 스팬(첫 식별자 이름, `KindConst`/`KindVar`), 블록
doc comment 포함(`parser.ParseComments` 활성화 — func/type 스팬은 `Pos()`
기준이라 불변). 재빌드(청크 +329: Const 175/Var 154) 후 재측정:

- **avg recall 0.556 → 0.722, nonzero 7/9 → 9/9**
- qa-review-intent 0 → **1.00**, composer-err-fail-closed 0 → **0.50**

전체 궤적: 0.296(한정 코퍼스) → 0.444(전체 코퍼스) → 0.556(스팬 재앵커) →
**0.722(const/var 청킹)**. 잔여 개선 여지(0.5~0.67대 시나리오)는 진짜 랭킹/
정밀도 영역 — P가 전반적으로 낮은 것(0.05~0.21)과 doc/archive 청크의 코드
질의 혼입이 다음 신호.

## doc/archive 강등 (2026-07-28, 후속 R4/R5)

stage2 집계기의 기존 `demoteTests` 패턴을 미러링해 두 강등을 추가:

- **doc 강등** (`docDemotionFactor=0.5`): 코드-지향 intent에서 doc 청크
  (ckv `chunk_kind=doc`, `.md` 폴백) 강등. DocsUpdate/ArchExplain/Unknown은
  제외(ADR·설계 문서가 정당한 답인 intent — intentToKinds 철학 미러).
- **archive 강등** (`archiveDemotionFactor=0.2`, **모든 intent**): `archive/`
  경로는 supersede된 문서 — 현행 답이 될 수 없다는 문서 규율의 랭킹 반영.
  doc factor와 비중첩(아카이브 판정 우선).

재측정(동일 인덱스): **avg recall 0.722 유지(무회귀), 9/9 nonzero 유지**.
`file_precision`은 0.12 수준 무변화 — 인용 12~22개 대비 기대 1~2개인 지표
구조상 순위 개선을 반영하지 못함(강등은 제거가 아니라 재순위). 질적 효과는
직접 질의로 확인: ErrFailClosed 질의에서 **정확한 var 청크(55-58)가 rank 2로
부상**(이전 부재), bm25-translation 질의 top-K에서 **archive 문서 소멸**.

잔여 신호: coordination-*/eval-fixture 계열 md가 일부 intent에서 여전히
상위(0.5 factor가 관대) — 과적합 튜닝은 보류, 랭크-민감 지표(MRR/nDCG류)
도입이 선행돼야 factor 조정을 측정 기반으로 할 수 있다.

## MRR 지표 도입 + 베이스라인 (2026-07-28, 후속 R6)

`cks-eval`에 `file_mrr`(기대 인용의 팩-순서 첫 매칭 랭크의 평균 역수,
match_mode 공유) 추가 — P/R/F1은 순서-무감이라 강등/부스트 튜닝을 측정할 수
없던 공백을 메움. per-run 계산·median 집계·intent 롤업(`avg_mrr`)·summary
출력까지 배선.

**베이스라인 (doc/archive 강등 포함 상태, 동일 인덱스): avg MRR 0.389**
(avg recall 0.722 재확인). 랭크 신호가 드러낸 것: ckg-bm25-translation
R=0.50/MRR=0.08(정답이 리스트 하부), qa-review R=1.00/MRR=0.25(rank 4),
composer-pipeline-flow R=1.00/MRR=0.32 — **"찾긴 하는데 상위에 못 올린다"가
현 병목**임을 정량화. 이후 랭킹 변경은 이 0.389 대비로 판정한다.

## intent 관통 + 헤더 강등 (2026-07-29, 후속 R7)

**선행 발견 — intent-조건 로직이 대부분 꺼진 채 평가되고 있었다**: 러너는
prompt만 전송하고 서버 분류기는 대부분의 시나리오에서 빈/Unknown을 반환
(footprint 실측: 9개 중 test_add만 분류 성공). 즉 stage2 kind 필터, path
glob, #40의 doc 강등 모두 8/9 시나리오에서 비활성이었다 — R7의 첫 시도
(헤더 강등 단독)가 MRR 완전 무변화였던 이유.

수정 2건(한 PR):
1. **intent 인자 관통**(additive): `get_for_task`에 선택적 `intent` 인자
   (`ParseIntent` 검증, 무효값 fail-loud) → `ComposeWithIntent`가 분류기
   우회. 평가 러너는 시나리오 선언 intent를 전달. intent를 아는 프로덕션
   호출자(파이프라인 단계)에게도 동일하게 유효.
2. **file_header 강등**(`headerDemotionFactor=0.5`, doc 강등과 같은 스위치):
   const/var 청킹 이후 헤더는 "오리엔테이션"이지 답이 아니다.

**재측정(동일 인덱스): avg MRR 0.389 → 0.438, avg recall 0.722 → 0.778**.

## doc comment 스팬 포함 (2026-07-29, 후속 R8)

func/type 스팬이 doc comment 라인에서 시작하도록 확장(const/var는 R3에서
기수행; grouped type 블록은 per-spec doc만, 단일-spec은 GenDecl doc 폴백).
자연어 신호("// Real is the in-process adapter...")가 임베딩 텍스트에
포함되고 스팬 라인도 리뷰어 직관과 일치. ckgalign은 exact-start 대신
overlap tier로 정렬 유지(회귀 테스트 고정).

**재측정(전체 재임베딩): avg MRR 0.438 → 0.497, avg recall 0.778 → 0.926**.
concurrency-safety MRR 0.10→0.75·R 0.50→**1.00**(R7 회귀의 예측된 근본
해소 — type Real 청크가 doc 포함 42-53으로 직접 매칭), ckg-bm25-translation
R 0.50→**1.00**·MRR 0.12→0.42, stamp-integrity R 0.67→**1.00**. MRR 하락
2건(pipeline-flow 0.57→0.20, qa-review 0.50→0.12)은 R=1.00 유지 상태에서
재임베딩에 따른 상위권 순서 변동 — 순효과 명백히 양.

**트랙 누적**: recall 0.296 → **0.926**, MRR(도입 후) 0.389 → **0.497**.

## stage1 BM25 rerank 활성화 (2026-07-29, 후속 R9)

ckv 엔진의 candidate-set BM25 rerank(`EnableBM25Rerank` — vector 히트 위에
BM25를 세워 RRF 융합)가 stage1 경로에서 꺼져 있었다. `ckvclient.SearchOpts`
에 `BM25Rerank` 배선 → stage1 본 라운드에서 활성화(프롬프트는 정확
식별자를 상시 포함 — "ErrFailClosed", "BM25Search").

**재측정(동일 인덱스, 쿼리타임 변경만): avg MRR 0.497 → 0.525, recall
0.926 유지**.

## anchor 가드 + 정직 재앵커링 (2026-07-29, 후속 R10)

스팬 드리프트가 세 라운드째 재발(이번엔 mcp-tool-handlers의 Register
60-78→실제 104, test-add의 테스트 3종 ~32줄 시프트)해 **구조적 가드**를
도입: expected citation에 선택적 `anchor`(스팬 안에 존재해야 하는 심볼
문자열) + `cks-eval -verify-anchors <root>`(위반 시 측정 전 fail-loud).
dogfood Makefile에 상시 배선, 9개 시나리오 15개 인용 전부에 anchor 부여.

**정직 재앵커링의 효과 — 측정 교정(검색 무변경)**:
- test-add R 0.67→**1.00**, stamp-integrity MRR 0.46→0.53 (진짜 개선)
- composer-err-fail-closed R 1.00→0.50, pipeline-flow R 1.00→0.50 — 기존
  1.00이 **stale 스팬의 우연 겹침 인플레이션**이었음이 드러남. 정직한
  스팬(fail_closed wrap 249-254, assemblePack 348-420)은 실제로 인용되지
  않는다 → **대형 함수 내부 영역의 function_split 청크 인용 커버리지**가
  다음 진짜 신호.
- 정직 집계: **avg MRR 0.461, avg recall 0.852** — 이후 비교의 새 기준선.
  (표면상 하락은 인플레이션 제거분; anchor 가드가 재발을 차단한다.)

## 인덱스 신선도 드리프트 (2026-07-29, 후속 P1)

R10이 "function_split 커버리지"로 지목했던 신호의 실체는 **세 번째 드리프트
종류 — 인덱스-코드 신선도**였다: dogfood 인덱스가 #42 머지 전 트리에서
빌드되어 청크 좌표가 구버전(ComposeTraced 144-244)이었고, 현행-트리 기준의
정직 스팬(wrap 249-254)과 구조적으로 겹칠 수 없었다. `make dogfood-eval`은
매회 재빌드라 안전하지만 수동 측정이 stale 인덱스를 재사용한 것.

재발 방지: `cks-eval`이 `cks.ops.freshness`의 indexed_head를 조회해
`-verify-anchors` 트리의 git HEAD와 대조, 불일치 시 **명시 WARNING**(과거
커밋을 고의 측정하는 워크플로는 정당하므로 fail 아님).

fresh 재빌드 후 재측정: **avg recall 0.852 → 0.907** —
composer-err-fail-closed R 0.50→**1.00**(아티팩트 해소 실증). 잔여 진짜
신호 1건: composer-pipeline-flow의 `assemblePack`(348-429)은 fresh
좌표에서도 미인용 — "pipeline flow" 프롬프트의 시맨틱 매칭에서 헬퍼가
Compose 계열 청크에 밀린다(R 0.50 고정, avg MRR 0.436 — 재임베딩 순서
변동 포함). 드리프트 3종(시나리오 스팬·intent 분류·인덱스 신선도)이 모두
가드를 갖춘 상태가 되었다. composer-pipeline-flow MRR 0.20→**1.00**(R8 재임베딩 순서
요동의 안정화 — 의도한 효과), mcp-tool-handlers 0.15→0.22. 하락 2건
(bug-fix 1.00→0.50, stamp-integrity 0.58→0.46)은 rank 1→2 수준 재배열 —
순효과 양.
잔여: mcp-tool-handlers·test-add(R 0.67), MRR 상위권 안정화(재임베딩마다
순서 요동) — 후자는 정확-식별자 BM25 rerank(stage1 ckv 경로) 후보.
composer-err-fail-closed MRR 0.25→0.55·R 0.50→**1.00**, pipeline-flow
0.32→0.57, qa-review 0.25→0.50. 유일 회귀 concurrency-safety MRR
0.50→0.10(R 불변): 이전 첫 매칭이 **우연히 헤더 청크**(1-50이 기대 42-55와
겹침)였던 케이스 — 대리 매칭의 소멸이며, 근본 해법은 TypeSpec 스팬에 doc
comment을 포함해 type 심볼 청크 자체가 랭크되게 하는 것(후속).

주의(배포): `--ckg` 정렬 빌드에서 const/var 청크는 canonical_id가 대부분
비게 된다 — ckg는 ValueSpec **per-spec** 노드, ckv는 **블록** 청크라
granularity 불일치. canonical ratio 게이트(`gate-min-canonical`) 분모 희석에
유의. 후속 옵션: (a) ratio 분모에서 Const/Var 제외, (b) ckgalign에 블록↔
spec 매칭 추가.

## 재현

```sh
# 인덱스: make -C system dogfood-eval USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=bge-m3 ...
cd system && ./bin/cks-eval -scenarios eval/scenarios \
  -cks-mcp ./bin/cks-mcp -config cks-dogfood.yaml -output /tmp/report.json
```

직접 질의 스크립트(stdio JSON-RPC, structuredContent 파싱)는 세션
스크래치에서 사용 — 요지: initialize → `cks.context.get_for_task`
`{"prompt": ...}` → `structuredContent.citations[].{file,start_line,end_line}`.
