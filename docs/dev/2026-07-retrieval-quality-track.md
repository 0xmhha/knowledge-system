# retrieval 품질 트랙 요약 (2026-07-28 ~ 07-29)

Status: 트랙 요약(지속 참조용). 사이클별 상세는
`archive/2026-07-28-dogfood-zero-recall-diagnosis.md`(supersede된 시계열
로그)에 있다. 다음 세션의 진입점은 이 문서 하나면 충분하다.

## 결과 한 줄

dogfood 9-시나리오 기준 **avg recall 0.296 → 0.963** (8/9 시나리오 R=1.00),
MRR 계측 도입(최종 0.425), **드리프트 가드 3종** 완비. knowledge-system PR
15건(#37~#38, #40~#46, #48~#52, #54) + coding-agent PR 1건(#63).

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

## 열린 백로그

(#57에서 해소: 필드-수준 조회 갭 — kind 어휘 교정("type"/"const"는 실존하지
않는 노드 타입이라 죽은 필터였음) + value kinds(field/constant/variable)
추가 + **프롬프트-원문·무모호 게이트 부스트**(SymbolWeight×2.0, 결과 1건일
때만). 15-스위트: recall 0.844→**0.911**, MRR 0.488 유지.
bm25-rerank-option 0→R 1.00. 비게이트 3.0 부스트는 recall 0.778로 유해
실측 — 게이트가 본질.)

1. **SymbolWeight 스윕** — mcp-tool-handlers(마지막 R 0.67)의 확정 원인:
   FindSymbol 정확 매칭(Register)의 RRF 기여(1.5/(60+r)≈0.025)가 ckv 상위
   기여(5.0/(60+r)≈0.08)에 밀려 최종 컷 탈락. **선행**: 정확-심볼 질의
   시나리오 5~10개 추가(anchor 필수) 후 1.5/2.5/3.5/5.0 스윕을 MRR·recall
   매트릭스로.
2. low-MRR 시나리오(qa-review 0.14, ckg-bm25 0.42 등) — 상위권 배치 개선.
   재임베딩 노이즈와 구분하려면 runs>1 중앙값 + 표본 확충과 함께.
3. arch_explain 이웃 구성의 defines(struct→field) 8개 — 가치 재평가(유지
   판단이면 근거 기록).
4. `#CallSite@`/`#ReturnStmt@` 문장-수준 서브노드의 이웃/해석 노출 여부
   점검(현재 관측상 문제 없음 — contains 미순회라 격리).

## 재현 절차

```sh
# 인덱스+측정 일체(항상 fresh): ollama + bge-m3 필요
make -C system dogfood-eval USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=bge-m3 \
     CKG=$PWD/bin/ckg CKV=$PWD/bin/ckv
# 수동 측정 시(인덱스 재사용): 신선도 WARNING을 반드시 확인
cd system && ./bin/cks-eval -scenarios eval/scenarios -verify-anchors .. \
     -cks-mcp ./bin/cks-mcp -config cks-dogfood.yaml -output /tmp/report.json
```
