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

### 원인 2 — `field` kind가 단일 라인 인용을 양산

`intentToKinds(ArchExplain)`이 `"field"`를 포함해서 FindSymbol이 struct
필드 노드를 반환하고, 각 필드가 **한 줄짜리 인용 한 건**이 된다. 실측:
`Composer` struct의 필드 8개가 `composer.go:62-62 … 69-69`로 **rank 9~16을
연속 점유**한다. `stamp-integrity-lookup`도 동형 — rank 1이 이름만 비슷한
`cmd/system/agent/format.go`의 `evidencePack` struct고, 그 필드 5개가
rank 11~15를 먹는다.

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

### 후속 (착수 순서)

1. **stage2 랭킹에 stage3 calls-엣지 반영** — 상위 시드의 calls 대상이
   인용 후보에 있으면 부스트. 원인 1 대응이자 유일하게 유의미한 개선.
   `graph_neighbors`가 이미 계산돼 있으므로 추가 그래프 질의는 불필요.
2. **`field`를 `intentToKinds(ArchExplain)`에서 제외** — 단 "option 필드는
   아키텍처 표면"이라는 원 근거가 있으니, 필드를 *인용*에서 빼되
   *neighbors*에는 남기는 분리가 맞을 수 있다. 백로그 3의 유지 판정은
   neighbors 기준이었다.
3. **header 강등을 doc 강등에서 분리** — `demoteHeadersFor(intent)`를 따로
   두고 arch_explain에서도 켠다. 가장 작고 독립적인 변경.

`mcp-tool-handlers`의 R 0.67은 별개다 — `Register`(server.go:100-143)가
목록에 아예 없다(server.go는 331-353, 355-368, 1-50만 등장). 항목 5 소관.

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
