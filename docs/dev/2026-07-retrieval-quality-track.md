# retrieval 품질 트랙 요약 (2026-07-28 ~ 07-31)

Status: 트랙 요약(지속 참조용). 사이클별 상세는
`archive/2026-07-28-dogfood-zero-recall-diagnosis.md`(supersede된 시계열
로그)에 있다. 다음 세션의 진입점은 이 문서 하나면 충분하다.

## 결과 한 줄

**15-시나리오 확장 스위트 기준 avg recall 0.978 / MRR 0.555 (14/15 시나리오
R=1.00, fresh 인덱스 — 2026-07-29 R15 기준선, 2026-07-31 R16에서 재현).**
시작점은 구 9-시나리오 스위트의 recall 0.296. **드리프트 가드 3종** 완비.
knowledge-system PR 18건(#37~#38, #40~#46, #48~#52, #54~#57) + coding-agent
PR 1건(#63).

## R16 재측정 · 결정성 (2026-07-31, `a65defc`)

백로그 2의 선행조건("R15 기준 재측정")을 처리하려고 **동일 트리에서 fresh
재임베딩 + 평가를 2회 독립 실행**했다(각 1,419파일, 회당 약 20분).

- **집계는 R15와 동일**: avg recall 0.978 / MRR 0.555, 14/15 R=1.00,
  mcp-tool-handlers R 0.67. R15(`#59` 시점 트리, 1,470파일)와
  R16(`a65defc`, 1,419파일) 사이에 코퍼스가 51파일 줄었는데도 지표는
  움직이지 않았다.
- **재임베딩 노이즈는 관측되지 않았다.** run1과 run2의 리포트를 시나리오별로
  대조하면 **지연시간을 제외한 전 품질 지표가 완전히 일치**한다(precision /
  recall / f1 / MRR / token_utilization / citation_count / body_count,
  15/15 시나리오, MRR 최대 변동 0.0000). 시나리오당 내부 반복은 이미
  `runs: 5`로 돌고 있다.
- **따라서 백로그 2의 "runs>1 중앙값" 선행조건은 불필요**하다. 측정 반복이
  아니라 랭킹 자체가 남은 문제다. 단 이 결정성은 *동일 코퍼스·동일 모델*
  전제이며, 코퍼스가 바뀌면 다시 확인해야 한다.

시나리오별 MRR(R16, 오름차순 — 개선 대상 판단 근거):

| 시나리오 | intent | R | MRR |
|---|---|---|---|
| qa-review-intent | qa_review | 1.00 | **0.14** |
| composer-pipeline-flow | arch_explain | 1.00 | **0.15** |
| mcp-tool-handlers | arch_explain | 0.67 | **0.22** |
| ckg-bm25-translation | refactor | 1.00 | 0.42 |
| stamp-integrity-lookup | arch_explain | 1.00 | 0.44 |
| bm25-rerank-option / bug-fix-rerank-drop / verify-anchors-guard / wal-reap-test-helper | — | 1.00 | 0.50 |
| composer-err-fail-closed | security | 1.00 | 0.58 |
| test-add-filesystem-fetcher | test_add | 1.00 | 0.61 |
| concurrency-safety-real-adapters | concurrency_safety | 1.00 | 0.75 |
| parse-intent-validation / rrfk-constant-lookup / structural-qname-filter | — | 1.00 | 1.00 |

intent 롤업에서 **`arch_explain`이 최하위**(n=4, R 0.92 / MRR 0.45)다 — 4개 중
3개가 하위권에 몰려 있다.

## arch_explain 랭킹 진단 (2026-07-31, `afede78`)

방법: fresh 인덱스에 **MCP stdio 직접 질의**(`cks.context.get_for_task`)로
반환 순위를 그대로 관찰하고, 동일 프롬프트를 다른 intent로 던지는 **반증
실험**으로 원인을 분리했다. 재구성한 순위로 계산한 MRR이 리포트 값과
일치(0.151 vs 0.15)하므로 관찰이 채점과 같은 것을 보고 있다.

원인은 셋이고, **기여도 순서가 직관과 반대**다.

### 원인 1 — 랭킹이 그래프를 안 본다 (지배적)

`composer-pipeline-flow`의 기대 스팬 둘 중 `Compose`(134-260)는 rank 4인데
`assemblePack`(348-420)이 **rank 19**다. 그런데 같은 팩의
`graph_neighbors`에는 이 엣지가 **이미 들어 있다**:

```
calls  167-271  ->  composer.go:348-474      # Compose -> assemblePack
calls  167-271  ->  searcher.go:186-294, expander.go:164-251, ...
```

즉 **stage3는 정답 형제를 정확히 짚어내는데 stage2 랭킹이 그걸 참조하지
않는다.** 정보가 같은 팩 한 섹션 옆에 있는데 순서에 쓰이지 않는다. "흐름을
설명하라"류 다중-스팬 시나리오의 MRR 손실 대부분이 여기서 난다.

### 원인 2 — 필드 인용의 출처 (2026-07-31 정정)

**최초 기재는 `intentToKinds(ArchExplain)`의 `"field"`를 원인으로 지목했으나,
그 8건의 실제 출처는 stage3의 `defines` relation이었다.** body 유무를 대조해
확인했다:

- 팩의 인용 25건 중 **body-backed 10건 + edge-only 15건**. `assemblePack`은
  edge-only 뒤꼬리(rank 19)에 있었다 — 낮게 랭크된 stage2 인용이 아니라
  **`assemblePack()`이 이웃 타깃을 인용 목록 끝에 덧붙인 것**이다.
- 필드 8건(`62-62 … 69-69`)은 전부 이웃 덤프의 `defines 61-70 -> …`와 일치.
  stage3는 `seedKeys`로 **시드인 타깃을 건너뛰므로**(expander.go), 이들이
  이웃으로 나왔다는 것 자체가 stage2 인용이 아니었다는 증거다.
- `intentToKinds`의 `field`가 실제로 낳은 것은 **rank 1의
  `composer.go:342-342`**(`stageTimings`의 필드 줄) 한 건이다. 이건 시드다.

따라서 반증 실험(`bug_fix`)이 필드를 없앤 것도 kind 때문이 아니라
`intentToRelations(BugFix)`에 `defines`가 없기 때문이다. **"stage3 relation
차이는 neighbors에만 작용한다"는 최초 서술이 틀렸다** — edge-only 경로로
인용까지 흘러든다. 그 결과 원인 2와 3의 분리 주장도 성립하지 않는다(둘 다
`bug_fix` 대조에 섞여 있었다).

### 원인 3 — header 강등이 doc 강등에 묶여 함께 꺼진다

`demoteDocsFor(ArchExplain) = false`가 `merge.go`에서 **두 개를 동시에**
끈다 — doc 강등과 `file_header` 강등. 그 결과 `composer.go:1-50`(파일 헤더)이
rank 3, `mcp/server.go:1-50`이 rank 8을 차지한다. 면제의 근거는 주석대로
"ArchExplain은 ADR·설계 문서를 정당하게 참조한다"인데, **file_header는 ADR이
아니다** — #42가 header를 강등 대상에 넣은 이유(심볼 스팬이 생긴 이상 헤더는
답이 아니라 방향 안내)는 intent와 무관하다. 부수효과로 보인다.

참고로 이 셋 중 **문서(.md)가 상위권을 먹는 현상은 관측되지 않았다** — 세
프롬프트 모두 상위권이 전부 코드였다. doc 면제 자체는 이 시나리오들에서
무해했다.

### 정량

동일 프롬프트를 `bug_fix`(callable-only kind + doc/header 강등 켜짐)로 던지면
원인 2·3이 사라진다 — 단일 라인 8건과 header가 목록에서 없어진다. 두 intent가
**인용**에서 갈리는 지점은 정확히 이 둘뿐이다(확인: `intentPathGlob`은
TestAdd만 비어있지 않고, `demoteTests`는 둘 다 true, stage3 relation 차이는
neighbors에만 작용) — 따라서 이 대조는 원인 2·3의 합산 효과를 깨끗하게
분리한다. 다만 2와 3 각각의 기여는 이 실험으로 나뉘지 않는다:

| intent | Compose 순위 | assemblePack 순위 | MRR |
|---|---|---|---|
| `arch_explain` | 4 | 19 | 0.151 |
| `bug_fix`(반증) | 3 | 16 | 0.198 |

**원인 2·3을 다 제거해도 0.151 → 0.198**에 그친다. 나머지는 원인 1이 쥐고
있다 — 착수 순서를 여기에 맞춰야 한다.

### 후속 1 착수 결과 — relation 가중치 (2026-07-31, 실측)

원인 1의 실제 기전을 특정했다: 이웃 점수가
`seed.Score / (1+distance)`로 **relation을 전혀 안 본다**(expander.go). 그래서
`Composer` struct 시드(인용 rank 2)의 `defines`→필드 8건이, 사용자가 물은
`Compose` 본문 시드(rank 4)의 `calls`→`assemblePack`보다 위로 왔다.

`relationWeight(intent, relation)`을 도입해 arch_explain에서만
defines 0.5 / imports 0.7 / calls·implements·embeds 1.0으로 스케일했다
(다른 intent는 전부 1.0 — 기존 동작 그대로).

**동작은 의도대로, 지표는 거의 안 움직였다:**

| | before | after |
|---|---|---|
| 이웃 상위 | `defines`→필드 8건 | `calls` 대상들 |
| `assemblePack` 인용 순위 | 19 | **15** |
| composer-pipeline-flow MRR | 0.15 | **0.16** (+0.007) |
| 15-스위트 avg recall / MRR | 0.978 / 0.555 | **0.978 / 0.555** (불변) |

나머지 14개 시나리오는 ΔMRR 0.000. 회귀 없음.

**천장이 드러났다 — 남은 두 벽:**

1. **body 슬롯이 답에 도달하기 전에 소진된다.** 인용 1~10위는 전부
   body-backed고 `assemblePack`은 body를 못 받는다. 가중치를 고쳐도
   edge-only 뒤꼬리 안에서만 움직이므로 rank 11 위로는 구조적으로 못 올라간다.
   before의 필드 2건(rank 9,10)이 after에는 `allocator.go` 2건으로 바뀌었을
   뿐, 슬롯은 여전히 답이 아닌 것에 간다.
2. **동점 calls 엣지의 정렬이 알파벳순이다.** 한 시드에서 나온 calls 엣지는
   전부 `seed.Score * 1.0 / (1+distance)`로 **점수가 같고**, 그러면
   `results()`의 타이브레이커(파일 경로 사전순 → 시작 줄)가 순서를 정한다.
   `allocator.go` < `composer.go`라서 `assemblePack`이 알파벳으로 밀렸다.
   즉 지금 순위의 상당 부분이 **의미가 아니라 파일명**이다.

### 후속 2 착수 결과 — 동점 타이브레이커 (2026-07-31, 실측)

`results()`의 동점 정렬을 파일 경로 사전순에서 **타깃 span 크기 내림차순**으로
바꿨다(그 뒤는 기존 파일/줄 순서 유지 — 결정성). 근거: 한 시드에서 나온 동일
relation 엣지는 점수가 전부 같으므로 타이브레이커가 사실상 순위를 정하는데,
그게 알파벳이었다. span은 이 지점에서 타깃에 대해 말해주는 유일한 신호다.

| | before | after |
|---|---|---|
| `assemblePack` 인용 순위 | 15 | **11** |
| composer-pipeline-flow MRR | 0.16 | **0.17** (+0.012) |
| 15-스위트 avg recall / MRR | 0.911 / 0.522 | 0.911 / **0.523** |

나머지 14개 ΔMRR 0.000, 회귀 없음.

**이 레버는 여기서 소진됐다.** rank 11은 *첫 edge-only 슬롯*이다 — 1~10위가
전부 body-backed이므로 타이브레이커로는 더 올릴 수 없다. 예측했던 구조적
하한에 정확히 도달했고, 남은 것은 **벽 1(body 슬롯)** 하나다. 실질 개선은
budget 배분이 edge-only 타깃을 body 슬롯으로 승격시킬 수 있어야 나온다.

### 기준선 이동 주의 — `bm25-rerank-option`의 취약성

d0bc9f0에서 **fresh 인덱스를 다시 만들었더니 스위트 기준선이
0.978/0.555 → 0.911/0.522로 내려갔다.** 원인은 단 하나,
`bm25-rerank-option`이 **R 1.00 → 0.00**으로 붕괴한 것이고 나머지 14개는
완전히 동일하다.

- **#68 때문이 아니다.** `relationWeight`는 arch_explain 외 모든 intent에서
  1.0을 반환하고 이 시나리오는 feature_add다 — 코드상 영향이 불가능하다.
- 트리 차이는 코퍼스 1,420 → 1,422파일(#68이 추가한 2개)뿐이다. 즉 **2파일
  변화가 이 시나리오를 1.00에서 0.00으로 뒤집었다.**
- 실측: 기대 `engine.go:112-123`(EnableBM25Rerank 필드)이 반환 10건 어디에도
  없고, 대신 `bm25/rerank.go`·`service_rerank.go` 계열이 상위를 채운다.

**R16의 "재임베딩 노이즈 0" 결론은 *동일 트리* 전제에서만 유효하다**는 단서가
이 사례로 실증됐다.

#### 조사 결과 (2026-07-31)

게이트가 닫힌 게 아니다. 실측으로 확인한 기전:

1. **심볼 경로는 살아 있다.** `FindSymbol("EnableBM25Rerank")` → **정확히
   1건**(`engine.go:123-123`). 프롬프트에 축자로 등장하므로 #57 게이트
   (`len(symbolCits)==1 && promptMentionsVerbatim`)는 **열려 있고**
   SymbolWeight×2.0이 걸린다.
2. **의미 경로는 애초에 없다.** 기대 스팬을 덮는 ckv 청크는 존재하지만
   (`47-134 Options [Struct]`), 이 질의의 ckv 상위 10에 **들지 못한다** —
   88줄짜리 struct는 의미가 흩어져 있고, 파일명·본문이 전부 rerank인
   `bm25/rerank.go`·`service_rerank.go`에 밀린다.
3. **따라서 이 시나리오는 부스트된 심볼 히트 1건이 body 슬롯 컷을 넘느냐에
   전적으로 달려 있고, 그 마진이 거의 없다.** 프롬프트를 심볼만으로 줄이면
   (`"EnableBM25Rerank"`) 기대 인용이 **rank 7**로 등장한다. 원 프롬프트
   (`"... query option that fuses BM25 with vector ranks"`)는 rerank 계열
   BM25 히트를 대량으로 끌어와(bm25_total_hits 95) 같은 인용을 10위 밖으로
   밀어낸다.

즉 **2파일 코퍼스 변화가 뒤집은 게 아니라, 원래 컷에 걸쳐 있던 것이 그 미세한
변화로 넘어간 것**이다. 이 시나리오는 안정 신호로 취급하면 안 된다.

#### 제안 — exact-symbol 예약 슬롯

allocator에는 이미 **같은 형태의 기아 문제를 푸는 선례**가 있다:
`KnowledgeReserve`(기본 2) — "knowledge 히트는 별도 kind-scoped 검색에서 와서
코드 시드보다 낮게 랭크되므로, 단순 greedy 패스로는 절대 선택되지 않는다".

프롬프트-축자·무모호 심볼 히트가 정확히 같은 처지다. 사용자가 이름을 직접
부른 심볼의 **선언 자리**가, 이름만 비슷한 본문들에 슬롯을 뺏기고 있다. RRF
가중치로 경쟁시키는 대신 슬롯 하나를 예약하는 편이 구조적으로 맞다.
구현은 stage2에서 게이트가 열린 인용에 provenance 태그를 붙이고
(`symbol_exact:<kw>` — 현재는 `symbol:<kw>@rank=N`로 부스트 여부가 구분되지
않는다), allocator가 `KnowledgeReserve`와 같은 방식으로 슬롯을 잡아두면 된다.

**착수·완료 (2026-07-31).** 구현하면서 조사 단계에서 못 본 것이 하나 나왔다:
프롬프트가 `EnableBM25Rerank`와 `BM25` **둘 다** 축자로 담고 있어 게이트가
두 번 열리고, 예약을 전역 1슬롯으로 두면 점수가 높은 `BM25` 쪽
(`bm25/rerank.go:112`)이 먼저 슬롯을 먹어 정작 기대 인용이 그대로 탈락했다
(실측: `seedSelected 8 / seedCap 8 / exactSelected 1`에서 드롭). **예약을
"심볼당 1슬롯"으로 바꾸자 해결**됐다.

| | before | after |
|---|---|---|
| bm25-rerank-option | 0.00 / 0.00 | **1.00** / 0.11 |
| 15-스위트 avg recall | 0.911 | **0.978** |
| 15-스위트 avg MRR | 0.523 | 0.530 |

나머지 14개 시나리오 변동 없음, 회귀 0. recall은 R16 수준(0.978)으로 복구됐다.
단 MRR은 R16(0.555)에 못 미치는데, 이 시나리오가 예약 슬롯으로 **뒤쪽에**
들어오기 때문이다(MRR 0.50 → 0.11). 예약은 "답이 팩에 있게" 보장하지 "위에
있게" 하지는 않는다 — 순위까지 올리려면 별도 레버가 필요하다.

계측 도구 주의: 조사에 쓴 stdio 프로브가 자식 프로세스의 stderr를
`DEVNULL`로 버리고 있어서, 계측을 넣고도 한동안 "출력 없음 = 코드가 실행되지
않음"으로 잘못 읽었다. 프로브로 서버 내부를 볼 때는 stderr를 반드시 파일로
받을 것.

### 후속 (남은 순서)
2. **`field`를 `intentToKinds(ArchExplain)`에서 제외** — 단 "option 필드는
   아키텍처 표면"이라는 원 근거가 있으니, 필드를 *인용*에서 빼되
   *neighbors*에는 남기는 분리가 맞을 수 있다. 백로그 3의 유지 판정은
   neighbors 기준이었다.
3. **header 강등을 doc 강등에서 분리** — `demoteHeadersFor(intent)`를 따로
   두고 arch_explain에서도 켠다. 가장 작고 독립적인 변경.

`mcp-tool-handlers`의 R 0.67은 별개다 — `Register`(server.go:100-143)가
목록에 아예 없다(server.go는 331-353, 355-368, 1-50만 등장). 항목 5 소관.

## convention 지식이 팩에 실리지 않던 문제 (2026-07-31)

arch_explain의 벽 1(body 슬롯)을 파다가 나온 별개 결함이다. 증상: stage4가
후보 80개 중 **9개만** 선택하고, **빈 본문 6건**을 버리며, 토큰 예산을 38%만
쓴다.

빈 본문 6건은 전부 `<convention>` 인용이었다. 실측한 성격 차이:

| chunk_kind | 개수 | 0-0줄 합성경로 | 파일 실재 |
|---|---|---|---|
| `convention` | 145 | **145 (전부)** | 없음 |
| `invariant` | 44 | 0 | 있음 |
| `doc` | 3,679 | 0 | 있음 |

**DB 결함이 아니다.** convention은 파일 발췌가 아니라 패키지 파생 요약
("errors: fmt.Errorf_wrap=1, constructors: 1 New*, tests: 120 files")이라
원본 파일이 존재할 수 없고, `<dir>/<convention>` 0-0은 "가리킬 위치가 없다"는
정확한 표현이다. 잘못된 건 **allocator의 가정**("모든 인용은 파일의 줄
범위")과, 그 결과 `KnowledgeReserve`가 **가져올 수 없는 후보를 위해 슬롯을
잡아두고 버리던 것**이다.

### 왜 인용으로 만들지 않았나

처음엔 인덱스 텍스트를 본문으로 공급해 인용으로 실으려 했는데, **계약 위반**을
발견해 중단했다: `Citation.IsValid`는 양수 줄 범위를 요구하고
(`citation.go:43`), `EvidencePack.IsValid`가 모든 인용에 그 검사를 건다. 0-0
인용을 실으면 팩 전체가 무효가 된다(현재 `IsValid`를 부르는 프로덕션 코드가
없어 드러나지 않았을 뿐).

그래서 계약을 완화하는 대신 **팩에 `knowledge` 섹션을 신설**했다. "인용은
파일의 스팬"이라는 전제는 팩 전체가 딛고 선 계약이라, 합성 위치를 받아들이려
그걸 흔드는 건 소비자 전원에게 비용을 떠넘긴다.

### 결과

| | before | after |
|---|---|---|
| 0-0 인용 | 있음(계약 위반) | **없음** |
| `empty_bodies` / `skipped` | 6 / 6 | **0 / 0** |
| 팩의 지식 전달 | 0건 | **24건**(convention 질의 기준) |
| 15-스위트 recall / MRR | 0.978 / 0.530 | **0.978 / 0.530** (불변) |

회귀 0이다. 예상과 달리 지표가 내려가지도 않았는데, convention은 애초에 body
슬롯을 **차지한 적이 없었기**(항상 skip) 때문이다.

**잔여**: `selected_count`는 여전히 9다. `KnowledgeReserve=2`가 이 질의에
존재하지도 않는 knowledge 후보를 위해 슬롯을 잡고 있다 — 가져올 수 없는
후보에는 예약을 걸지 않는 별도 수정이 필요하다.

## 실패로 끝난 두 시도 — 반복 방지 기록 (2026-08-03)

`selected_count`가 10이 아니라 9인 문제(#72 잔여)를 두 방향으로 고쳐봤고
**둘 다 지표를 내려서 폐기**했다. 둘 다 자연스러운 발상이라, 기록이 없으면
다음 세션이 같은 길을 다시 간다.

기준선: main `5493b8c` — recall 0.978 / MRR 0.530,
`composer-pipeline-flow` MRR **0.1705**.

### 시도 A — 채울 수 있을 때만 예약 (KnowledgeReserve 룩어헤드)

발상: knowledge 후보가 남아 있지 않은데도 홀드백이 슬롯을 잡아 코드 후보를
막는다. 남은 후보 수로 예약을 상한하면 낭비가 사라진다.

결과: **의도대로 동작했다** — body 9 → **12개**, `empty_bodies` 0.
그런데 `composer-pipeline-flow` MRR **0.1705 → 0.1667**.
body가 늘수록 edge-only인 `assemblePack`이 뒤로 밀리기 때문이다.
**슬롯을 회수할수록 답이 내려간다.**

### 시도 B — 인용을 증거 강도로 정렬

발상: 인용 목록이 "body 있는 것 전부 → 없는 것 전부" 순인 건 예산 배분의
부산물이지 랭킹이 아니다. 점수로 정렬하면 edge-only도 제 자리를 찾는다.

결과: **`assemblePack`이 11 → 12위로 오히려 내려갔고** MRR도
0.1705 → 0.1667.

### 왜 B가 원리적으로 불가능한가 (이번 사이클의 진짜 소득)

이웃 점수는 정의상 `seed.Score / (1 + distance)`다. 즉 **이웃은 자기를
만들어낸 시드보다 항상 낮다.** 따라서:

> 정렬 방식을 어떻게 바꾸든 **edge-only 인용은 body-backed 인용을 앞지를 수
> 없다.** "edge-only가 뒤에 고정되는 배치가 문제"라는 진단 자체가 틀렸다 —
> 배치가 아니라 **점수식**이 이웃을 하위에 못 박는다.

그리고 A와 B는 서로 상충한다: A는 body를 늘리고, body가 늘면 edge-only는
더 밀린다.

### 부수 발견 — 기존 테스트의 주석/단정 모순

`budget/allocator_knowledge_test.go`의
`TestAllocate_NoKnowledgeCandidatesLeavesReserveUnused`는 주석과 단정이
어긋난다:

- 주석 앞절: "홀드백은 knowledge 후보가 아직 나올 수 있는 동안에만 슬롯을
  잡는다" ← 시도 A가 구현한 동작
- 주석 뒷절 + 단정: "없으면 그냥 비운 채 캡보다 하나 적게 끝난다" ← 현재 동작

어느 쪽이 진짜 의도인지 코드만으로는 판정 불가다. 예약 의미론을 다시
건드릴 때는 이 모순부터 정리할 것.

### 시도 C — 거리 감쇠 재설계 (2026-08-03, 롤백)

A/B의 결론("점수식이 이웃을 시드 아래 못 박는다")을 그대로 따라가 점수식을
고쳤다. `seed.Score / (1 + distance)` → `seed.Score / max(distance, 1)`.
근거: distance 1은 직접 엣지인데 1+1=2로 나눠 **50% 페널티**를 매긴다.
시드가 직접 호출하는 함수는 같은 이야기의 절반이 아니라 동급이다.

결과: **스위트가 무너졌다.**

| | before | after |
|---|---|---|
| avg recall | 0.9778 | **0.7222** |
| avg MRR | 0.5301 | **0.3881** |

15개 중 **5개가 recall을 완전히 상실**(qa-review-intent, ckg-bm25-translation,
bug-fix-rerank-drop이 1.00→0.00; composer-pipeline-flow 1.00→0.50;
mcp-tool-handlers 0.67→0.33). `structural-qname-filter`는 MRR 1.00→0.33.

**즉시 롤백했고 베이스라인(0.9778/0.5301)이 정확히 복원됨을 재측정으로 확인.**

### 그래서 거리 페널티는 임의값이 아니라 하중을 받는 설계다

이웃을 시드와 동급으로 올리면 body 슬롯이 이웃으로 넘쳐 **정작 기대하던 코드
인용이 밀려난다.** `1+distance`의 50% 페널티는 "이웃은 보조 증거"라는 위계를
강제하고 있었고, 그 위계가 무너지자 검색이 답을 잃었다.

A/B/C를 합치면 결론은 이렇다:

> edge-only 인용이 아래에 있는 것은 **버그가 아니라 균형점**이다. 순서를
> 바꾸면(B) 효과가 없고, 슬롯을 늘리면(A) 답이 밀리고, 위계를 없애면(C)
> 검색이 망가진다. 세 방향이 모두 같은 지점을 가리킨다 — 현재 배치는
> 우연이 아니라 이 세 힘이 만나는 자리다.

`assemblePack`을 상위로 올리려면 이웃 위계를 건드리는 대신 **그것이 애초에
시드가 되도록** 하는 쪽(stage2 검색이 직접 찾아내게 하는 것)이 남은 방향으로
보인다. 그건 이웃 확장이 아니라 검색 문제다.

### 시도 D — 예약 의미론 판정 (2026-08-03, 동작 변경 없음)

`TestAllocate_NoKnowledgeCandidatesLeavesReserveUnused`의 주석/단정 모순을
**양쪽 다 실측해서** 판정했다.

| 변경 | recall | MRR | composer-pipeline-flow |
|---|---|---|---|
| 기준선 | 0.9778 | 0.5301 | 0.1705 |
| 룩어헤드 추가(주석이 말하는 동작) | 0.9778 | 0.5299 | **0.1667** |
| `KnowledgeReserve` 2→0 | 0.9778 | 0.5299 | **0.1667** |

두 방식이 **동일한 −0.0038**을 낸다. 슬롯을 회수하는 방법이 무엇이든 결과가
같다는 뜻이고, 이는 A/B/C의 결론(“body가 늘면 edge-only가 밀린다”)과 정확히
일치한다. recall은 어느 쪽도 움직이지 않는다.

**판정: 무조건 예약(현재 동작)을 유지하고 주석을 코드에 맞춘다.** 룩어헤드를
지지하는 측정 근거가 없다.

#### 다만 예약의 명분이 #72로 바뀌었다 — 미해결

예약의 존재 이유는 "도메인 규칙이 코드 본문에 슬롯을 뺏기지 않게"였는데,
#72가 `convention`을 인용에서 빼 `knowledge` 섹션으로 보냈다. 남은 knowledge
후보는 `invariant`와 트리 밖 `doc`뿐이다.

실측(2026-08-03, 두 질의): `arch_explain`은 body 9 / knowledge 6,
`qa_review`는 body 11 / knowledge 0 — **어느 쪽도 knowledge 후보가 body
슬롯을 채우지 않았다.** 즉 예약은 지금 아무도 안 오는 자리를 잡고 있다.

그런데 그 자리를 비우면(위 표) 지표가 내려간다. 예약이 **의도와 무관하게**
"body를 덜 실어 edge-only를 덜 밀어내는" 효과로 값을 하고 있는 셈이다.
우연한 균형이므로, 예약을 손대려면 그 전에 `invariant`/`doc` 후보가 실제로
어떤 질의에서 body 슬롯을 필요로 하는지부터 표본을 잡아야 한다.

### knowledge 회귀 가드 (2026-08-03, A-14)

#72가 `convention`을 `pack.knowledge`로 옮겼는데, **eval의 recall/MRR은 인용
기반이라 그 섹션을 볼 수 없다.** 시나리오가 1.00을 받으면서 지식은 하나도
안 실릴 수 있다 — `bm25-rerank-option`이 무너진 것과 정확히 같은 사각지대다.

시나리오에 선택 필드 `expected_knowledge`(scope 목록)를 추가하고,
`cks-eval`이 앵커 가드와 같은 방식으로 **결손 시 loud fail**하도록 했다.
`composer-pipeline-flow`에 composer 패키지 scope 2개를 걸었다.

- **지표 영향 0**: recall 0.9778 / MRR 0.5301 불변, 15개 시나리오 변동 없음.
  프로덕션 코드는 한 줄도 안 건드렸다(eval 하네스 + 시나리오 YAML만).
- **양방향 검증**: 정상 시 exit 0, 없는 scope를 넣으면
  `knowledge missing: composer-pipeline-flow: expected scope ... absent`
  로그와 함께 **exit 1**. 리포트 JSON은 실패해도 먼저 기록된다(수치 보존).
- 점수가 아니라 **가드**로 설계했다 — Metrics에 넣으면 지식 결손이 인용
  품질과 상쇄되어 묻힌다.

### 시도 E — `assemblePack`을 시드로 만들기 (2026-08-03, 코드 변경 없음)

A-12가 남긴 유일한 방향이었다: 이웃 위계를 건드리는 대신 stage2 검색이
`assemblePack`을 직접 찾게 한다. 진단해보니 **검색 결함이 아니었다.**

- ckv 청크는 존재한다: `345-474 [symbol] assemblePack` (#43대로 doc comment
  부터 시작).
- 그 텍스트는 "builds the final EvidencePack … assembly focused on data
  shaping" — **pack 조립**에 관한 것이다. 프롬프트는 "chain the stages end to
  end", 즉 **스테이지 연결**을 묻는다. 서로 다른 이야기다.
- 실증: 프롬프트에 `and assemble the final pack`을 더하면 ckv가
  **rank 9로 찾아낸다.** 빼면 상위 10에 없다. 문구가 이유임이 증명된다.

즉 이 시나리오에서 `assemblePack`은 **그래프로만 도달 가능**하고, 그것이
graph expansion의 존재 이유다. 실제로 팩은 rank 11로 전달한다.

**판정: 고칠 것이 없다.** 프롬프트가 묘사하지 않는 것을 검색이 반환하게 만들면
이 시나리오 하나에 대한 과적합이다. 이 시나리오의 낮은 MRR(0.17)은 검색
결함이 아니라 **그래프-경유 증거의 비용**을 측정한 값이다. 시나리오 설명에
같은 취지의 주석을 달아, 다음 사람이 이걸 "고쳐야 할 버그"로 읽지 않게 했다.

이로써 A-10~A-15가 모두 닫힌다. `assemblePack`을 상위로 올리는 길은 —
순서(B)·슬롯(A)·위계(C)·시드화(E) 어느 쪽으로도 — 없다.

### 다음에 이 영역을 열 때

`selected_count 9`는 **여전히 사실**이고 낭비도 실재한다. 다만 현재 지표
체계로는 그 개선을 정당화할 수 없다 — 고치면 시나리오가 내려간다. 손대려면
**이웃 점수식**(거리 감쇠가 이웃을 구조적으로 시드 아래 고정하는 것)을
먼저 다뤄야 하고, 그건 검색 랭킹 변경이라 자체 사이클이 필요하다.

## 수치 궤적 (정직 측정 기준)

| 단계 | 변경 | recall | MRR |
|---|---|---|---|
| 시작(한정 코퍼스) | — | 0.296 | — |
| 전체 코퍼스 임베딩 | dogfood 배관 수정(#42 이전 F1) | 0.444 | — |
| 스팬 재앵커 #37 | 측정 교정 | 0.556 | — |
| const/var 청킹 #38 | 파서: 50줄 밖 선언 인덱싱 | 0.722 | — |
| MRR 계측 #41 | 랭크-민감 지표 | 0.722 | 0.389 |
| intent 관통+헤더 강등 #42 | 분류기 우회 인자 | 0.778 | 0.438 |
| doc comment 스팬 #43 | 파서: NL 신호 포함 | 0.926 | 0.497 |
| ckv BM25 rerank #44 | 정확-식별자 리프트 | 0.926 | 0.525 |
| anchor 가드+정직 재앵커 #45 | 인플레이션 제거(측정 교정) | 0.852 | 0.461 |
| 신선도 가드 #46 | stale-인덱스 아티팩트 해소 | 0.907 | 0.436 |
| 키워드 케이스 dedup #51 | 캡 오염 수정 | 0.907 | 0.422 |
| graph_neighbors 복구 #52 | matchQname+anchor 겹침(+형태소 후보) | 0.907 | 0.422 |
| 구조 의사노드 필터 #54 | calls 회복→assemblePack 승격 | **0.963** | 0.425 |

MRR 등락은 재임베딩/키워드셋 변화의 재셔플 노이즈(±0.05)를 포함 — 개별
변경의 판정은 항상 시나리오별 diff로 했다.

## 지속 교훈 (이 트랙의 진짜 산출물)

### 1. 드리프트 3종과 가드 — 측정이 거짓말하지 않게

| 드리프트 | 증상 | 가드 |
|---|---|---|
| 시나리오 스팬 | 코드 이동 → 조용한 recall 0 (3회 재발) | expected citation `anchor:` 필드 + `cks-eval -verify-anchors <root>` (측정 전 fail-loud) |
| intent 분류 | 분류기 Unknown 강등 → intent-조건 로직 전체 침묵 비활성 | `get_for_task`의 명시 `intent` 인자(무효값 fail-loud) — 파이프라인은 stage별 고정 intent 전달(coding-agent #63) |
| 인덱스 신선도 | 구-트리 좌표 청크 ↔ 현-트리 스팬 비교 → 가짜 miss | `cks-eval`이 ops.freshness indexed_head ↔ 트리 HEAD 대조 WARNING |

### 2. 좌표 정확-일치 의존은 스팬 의미가 바뀌면 전부 부러진다

doc-포함 스팬(#43) 하나가 **동일 계열 침묵 고장 3곳**을 만들었다:
ckgalign(#48 tier 2b), matchQname(#52), pack source-anchor(#52). 교훈:
청크↔노드 매칭은 정확-일치가 아니라 **포함/겹침 래더 + 의사노드 제외**로.
의사노드 판별은 접두사(file:/hunk:/import:)만으론 부족 — **경로형 qname
(슬래시 포함)**까지(#54).

### 3. 랭킹 튜닝은 계측이 선행

- P/R은 순서-무감 → 강등/부스트는 MRR 없이는 판정 불가(#41).
- 9-시나리오 스위트의 재셔플 노이즈 ±0.05 — factor 조정은 표본 확충 없이
  하면 과적합. (SymbolWeight 백로그가 이 원칙의 적용 대상.)
- 각 사이클 = 진단(직접 질의·footprint·DB ground truth) → 국소 수정 →
  시나리오별 diff 판정. "지표가 안 움직여도 기능 복구는 실가치"인 경우
  (graph_neighbors)는 프로브로 실증해 선적.

### 4. 반복된 실수 계열 (프로세스)

- 공유 체크아웃 혼입 2회 → **전용 worktree 규칙** (메모리에 저장됨).
- `go build ./cmd/.../eval`( -o 없이) + `git add -A` → 바이너리 커밋 2회
  (#49, #53) → 루트 앵커드 ignore.
- stale 인덱스로 수동 측정 → 신선도 가드(#46)의 동기.

## 백로그

번호는 본문 서술이 참조하므로 종결되어도 **재번호하지 않는다** — 상태 표기로
구분한다.

(R15 fresh 재기준선, 2026-07-29: 전 코퍼스 1,470파일 재임베딩 후 15-스위트
**recall 0.978 / MRR 0.555, 14/15 R=1.00**. structural-qname-filter
0→1.00 — stale-인덱스 예측의 실증. defines(struct→field) 8개는 프로브 결과
Composer 필드들(=스테이지 구성 요소)로 아키텍처 신호가 실재 — **유지 판정**,
백로그 3 종결. 잔여는 mcp-tool-handlers 0.67 단 하나: Register가 파생
키워드(형태소)라 verbatim 게이트 부스트 대상이 아님 — 설계 일관적 미해결로
기록.)

(#57에서 해소: 필드-수준 조회 갭 — kind 어휘 교정("type"/"const"는 실존하지
않는 노드 타입이라 죽은 필터였음) + value kinds(field/constant/variable)
추가 + **프롬프트-원문·무모호 게이트 부스트**(SymbolWeight×2.0, 결과 1건일
때만). 15-스위트: recall 0.844→**0.911**, MRR 0.488 유지.
bm25-rerank-option 0→R 1.00. 비게이트 3.0 부스트는 recall 0.778로 유해
실측 — 게이트가 본질.)

### 열린 항목

2. low-MRR 시나리오 — 상위권 배치 개선. **선행조건 해소(R16, 위 절 참조).**
   종전 기재는 이 수치가 R15 이전 값이라 대상을 다시 정해야 한다고 봤으나,
   R16이 **qa-review 0.14 / ckg-bm25 0.42를 그대로 재현**했다 — 낡은 값이
   아니었다. "runs>1 중앙값"도 불필요(재임베딩 2회 간 품질 지표 변동 0).
   확정 대상은 **R 만점인데 순위가 바닥인 3건**이다(찾아오지만 묻는다 =
   이 항목이 정의한 문제 그 자체):
   - `qa-review-intent` MRR 0.14
   - `composer-pipeline-flow` MRR 0.15 — #54로 R 1.00에 도달했으나 순위는
     최하위권. 종전 목록에 없던 신규 대상.
   - `stamp-integrity-lookup` MRR 0.44 — 역시 신규.
   `ckg-bm25-translation`(0.42)은 유지. `mcp-tool-handlers`(0.22)는 R<1.00
   이라 성격이 달라 항목 5가 계속 보유한다.
   `arch_explain` intent가 롤업 최하위(R 0.92 / MRR 0.45)이고 위 3건 중
   2건이 여기 속하므로, **intent 단위 진단이 시나리오별 땜질보다 앞선다**.
   → **진단 완료(2026-07-31, 위 "arch_explain 랭킹 진단" 절).** 원인 3개 중
   지배적인 것은 *랭킹이 stage3 calls-엣지를 안 본다*는 것이고, 나머지 둘
   (`field` kind의 단일 라인 인용, header 강등이 doc 강등에 묶임)은 다 고쳐도
   0.151→0.198에 그친다. 착수 순서는 그 절의 "후속" 참조.
4. `#CallSite@`/`#ReturnStmt@` 문장-수준 서브노드의 이웃/해석 노출 여부
   점검(현재 관측상 문제 없음 — contains 미순회라 격리).
5. mcp-tool-handlers R 0.67 — 15-스위트 유일 잔여. `Register`가 형태소 파생
   키워드라 #57의 verbatim 게이트 부스트 대상이 아닌 **설계 일관적 한계**.
   과제로 열어둘지 수용으로 닫을지 판단 대기.

### 종결 (근거 기록)

1. ~~SymbolWeight 스윕~~ — **#56에서 종결: 1.5 유지.** 선행조건이던 정확-심볼
   시나리오 확충도 #56이 6개 추가(9→15)로 완료했다. 스윕 결과 1.5와 2.5는
   동률(MRR 0.488/0.492, recall 0.844), 3.5·5.0은 recall 0.778/0.733으로
   저하 — 가중치를 올리면 이름만 맞고 틀린 심볼이 의미 히트를 밀어낸다.
   원인 분석(FindSymbol의 RRF 기여 ≈0.025가 ckv 상위 ≈0.08에 밀림)은
   유효하나, 해법은 전역 가중치가 아니라 #57의 **조건부 게이트 부스트**였다.
   남은 mcp-tool-handlers 0.67은 열린 항목 5로 이관.
3. ~~arch_explain 이웃의 defines(struct→field) 8개~~ — **R15 프로브로 유지
   판정, 종결.** Composer 필드들(=스테이지 구성 요소)이라 아키텍처 신호가
   실재한다.

## 재현 절차

```sh
# 0) ckg/ckv 바이너리 준비 — 아래 CKG=/CKV=가 가리키는 대상.
#    루트 `make build`는 컴파일 체크(go build ./...)일 뿐 bin/을 만들지 않는다.
make build-dataset-bins
# 인덱스+측정 일체(항상 fresh): ollama + bge-m3 필요. 전 코퍼스 재임베딩이라
# 회당 약 20분(1,419파일 기준).
make -C system dogfood-eval USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=bge-m3 \
     CKG=$PWD/bin/ckg CKV=$PWD/bin/ckv
# 수동 측정 시(인덱스 재사용): 신선도 WARNING을 반드시 확인
cd system && ./bin/cks-eval -scenarios eval/scenarios -verify-anchors .. \
     -cks-mcp ./bin/cks-mcp -config cks-dogfood.yaml -output /tmp/report.json
```
