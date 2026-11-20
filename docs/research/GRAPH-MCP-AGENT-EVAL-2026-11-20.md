# 그래프 MCP 에이전트 평가 보고서 (2026-11-16 ~ 2026-11-20)

이 문서는 코드 그래프(ckg)와 그 MCP 도구를 고치면서 2026-11-16부터 11-20까지 돌린 실험을 한곳에 모은 기록이다.
질문은 하나였다. **그래프 MCP를 쓰는 에이전트가 왜 grep만 쓰는 에이전트보다 정답을 덜 찾는가, 그리고 무엇을 고치면
정답에 닿는 정보가 한 번의 도구 응답 안에 LLM에게 전달되는가.** 실행 순서와 상태의 정본은
[정확도 로드맵](../ACCURACY-ROADMAP.md)이고, 이 문서는 무엇을 바꿨고 무엇이 측정됐고 무엇이 아직 확인되지 않았는지를 남긴다.

원자료(세션 기록, 점수 파일, 결과 문서)는 저장소 밖 `knowledge-data/agent-eval/runs/<run>`에 있고 커밋하지 않는다.
아래 숫자는 각 run의 `RESULT.md`와 `scores-go-stablenet-v2.json`에서 옮겼다. 세션 수가 적은 비교는 "판정선을
넘었다"까지만 적고, 효과의 크기로 쓰지 않는다.

---

## 1. 평가 방법

### 질문과 점수

- **go-stablenet-v2**: 21문항. `sym-55`~`sym-77`은 실제 버그 수정 11건을 식별자 없이 증상만으로 다시 묻는 질문이고,
  `flow-01`~`flow-10`은 동작으로 설명한 함수와 그 직접 호출자를 모두 찾는 질문(정답은 `gopls call_hierarchy`)이다.
- **tune / holdout 분할**: 튜닝 전에 고정했다(2026-11-17, `harness/exposure_curve.py`).
  TUNE = sym-55, 58, 67, 70, 74, 77. HOLDOUT = sym-56, 63, 69, 73, 75. 튜닝은 tune만 보고, holdout과 flow는 확인에만 쓴다.
- **core recall**: 답이 맞힌 core 선언의 비율. 파일이 같고 심볼 이름의 마지막 부분이나 줄이 맞으면 맞힌 것으로 센다.
  core 라벨은 모델이 만든 것이고 사람이 수용한 것이 아니다.
- **top-1 core**: 답의 첫 번째 must-change 위치가 core인 비율.
- 모델은 전 기간 `claude-sonnet-5-5`, 클라이언트는 Claude Code 2.1.325~2.1.326이다.

### arm

- `nograph`: grep과 파일 읽기만 쓴다.
- `graph`: ckg MCP 도구를 함께 쓴다. 도구를 쓸지는 에이전트가 정한다.
- `graph-forced`: 그래프 도구를 먼저 쓰도록 지시한다.

### 비교 방법

후반부(11-19 이후)의 에이전트 비교는 모두 같은 시각에 arm을 나란히 돌렸고, 판정선을 실행 전에 정했다
(차이 ≥ 0.3이면 "가능성이 높음", ≤ −0.3이면 "해로울 가능성", 그 사이는 판정 불가). 같은 바이너리와 같은 텍스트로 돌린
arm끼리도 core recall이 0.15 차이 났다(J1 base 0.30, K1 base 0.45; 동작이 같은 I3 base 0.45). 세션 10개짜리
비교의 차이 0.2 정도는 우연으로도 나온다.

### 클라이언트 제약 (측정으로 확인)

- Claude Code는 약 25,000 토큰을 넘는 MCP 결과를 오류로 바꾼다. `MAX_MCP_OUTPUT_TOKENS`를 올리면 큰 결과는
  2 KB 미리보기가 붙은 `<persisted-output>`으로 바뀐다.
- 도구 정의의 `_meta["anthropic/maxResultSizeChars"]`를 선언하면 큰 결과도 그대로 모델에게 전달된다.
  `ckg mcp --max-result-chars`가 이것을 선언한다.
- 헤드리스 에이전트는 `--add-dir` 없이는 작업 폴더 밖의 파일을 읽지 못한다.
- MCP 결과 안의 글(“다음 부분을 요청하라”, “이 파일을 읽어라”)은 에이전트의 다음 행동을 바꾸지 못했다(3장).

---

## 2. 바뀐 기능

### 그래프 (commit `1e1003c0`)

| 스키마 | 변경 |
|---|---|
| 1.29 | 값 사용 간선 `reads_field`, `writes_field`, `references`와 `find_references` 도구 |
| 1.30 | `investigate`, `investigate_open` 도구 |
| 1.31 | SSA/VTA 값 흐름으로 만든 `dispatches_to` (인터페이스 호출이 구현에 닿는 수 0 → 3,715 / 3,793) |
| 1.32 | 문장 단위 간선(`site_calls`, `cond_reads`, `cond_calls`, `body_writes`, `encloses`)과 채널 송수신 간선 |
| 1.33 | `ckg verify` 정적 검사, `ckg observe`로 테스트 실행 중 호출과 채널 연산 기록 |
| 1.34 | 채널 별칭, 클로저 셀, 타입 계층 dispatch |
| 1.35 | 타입 계층 간선을 PageRank에서 뺌 |
| 1.36 | External 노드: 빌드 밖 대상(표준 라이브러리, 의존 모듈)으로 가는 호출과 타입 사용을 버리지 않고 심볼마다 노드 하나로 잇는다. 증분 빌드도 유지 |
| 1.37 | 패키지 안 테스트가 있는 운영 파일이 테스트 변형을 통해 두 번 걷히던 값 사용 수 중복을 고침 |

검증 결과(`verify2`): 정적 검사 5종 누락 0 / 예상 밖 0. 실행 관찰 대비 recall은 인터페이스 호출 693/693,
함수 값 호출 467/473, 채널 연산 495/544, 채널 쌍 238/267이다.

### search_code와 MCP (commit `1e1003c0`)

- **grep형 검색**: RE2 정규식을 선언마다 소스 줄에 맞추고, 맞은 줄을 그 줄을 감싸는 가장 안쪽 선언에 묶는다.
  선언마다 맞은 줄 전부와 그 줄에서 그래프가 푼 호출과 값 사용(`uses`)을 붙인다. 결과를 자르지 않는다.
- **운영 코드와 테스트 분리**: `search_code`는 운영 코드만, `search_test_code`는 테스트 코드만 찾는다.
- **순서**: 이름 일치 → 일치 횟수 → PageRank. 이 셋이 같은 선언은 이름순으로 정렬하지 않고 인덱스 순서를 유지한다.
  `investigate`의 동점도 이름 대신 노드 id 순이다. (이전에는 이름순 정렬 때문에 a로 시작하는 이름이 항상 앞에 왔다.)
- **빌드 밖 호출 표기**: `uses`에 "calls strings.Split"처럼 패키지 이름으로 적고, 결과 맨 위 `packages`에 import 경로,
  모듈, 버전을 한 번씩 적는다.
- **큰 결과 전달 방식** (3장에서 측정): parts(나눠 보냄), index(목록만 보내고 `ids`로 상세 요청), file(파일로 저장하고 경로만
  보냄), 그리고 `_meta`로 한 번에 보냄.
- **서버 안내문**: search_code 결과의 키를 하나씩 설명한다.
- **측정용 플래그**: `--page-bytes`, `--max-result-chars`, `--result-dir`, `--no-instructions`, `--tool-text`
  (다른 빌드가 보낸 안내문과 도구 정의를 그대로 보내고 동작은 이 빌드 것을 쓴다).

### search_code 선택 모드 (commit `031f8c73`, 기본값 꺼짐)

- `--name-as-both`: `in=name` 요청에 `in=both`로 답한다. 이름 일치가 먼저, 본문 일치가 뒤에 온다.
- `--reason-tail`: `reasons` 인자를 받는다. hit의 그래프 이웃(호출, 값 읽기·쓰기, 양방향, 운영 코드) 가운데 이유 단어를
  가진 선언을 hit 뒤 `related`에 붙인다. hit 순서는 바꾸지 않는다. 여러 낱말로 된 이유("round change")는
  `roundChange`, `round_change`에도 맞는다(I3 측정에서 이 형태가 0행이던 결함을 고친 것).

---

## 3. 실험 결과

### 3.1 출발점: 그래프를 줘도 정답이 늘지 않았다 (11-16 ~ 11-17)

| run | 변경 | core recall |
|---|---|---|
| `20261116-ks-a96c6eff` (v2 21×3) | 기준선 | nograph 0.94, graph 0.88, forced 0.83 |
| `20261117-…-value-refs` | 값 사용 간선 | forced 0.83 → 0.85 ("반복 간 차이 안") |
| `20261117-…-value-refs-manual` | 서버 안내문, 과제형 도구 설명 | graph 0.83 (안내문 없을 때 0.88) |
| `20261117-…-search-code-2` | 첫 search_code | graph 0.89, nograph 0.94 |
| `20261117-…-investigate` (v2 21×5) | investigate 도구 | graph 0.907, nograph 0.894, investigate 먼저 0.873 |
| `20261117-…-trace` (21×5) | 도구 글에서 평가 어휘 제거, 호출 기록 | graph 0.879, nograph 0.901 |

- 기준선 결과 문서의 문장: "The agent barely used the graph." "Forcing the graph lowers accuracy."
- **질문별 손실은 일부 질문에 몰려 있었다.** 10회 반복(`search-code-2-rep10`)에서 sym-77은 nograph 0.55 대 graph 0.10,
  flow-07은 1.00 대 0.74였고 sym-55는 0.65 대 0.70으로 손실이 없었다. flow-07의 손실은 빌드 범위에서 왔다.
- **기록 재생은 결정적이었다.** 그래프 세션 147개의 호출 474개를 다시 돌려 471개가 같은 결과를 냈다(`trace-replay`).
  이후 오프라인 재생 실험은 이 성질에 기댄다.

### 3.2 그래프 데이터를 늘려도 세션의 정답 노출은 바뀌지 않았다 (11-18)

`dispatch`, `stmt`, `replay134`, `replay135`, `replay135t`는 investigate run의 호출 322개를 새 그래프로 다시 돌렸다.
search_code 답이 바뀐 호출은 1.31에서 41/214, 1.32에서 0/214, 1.35에서 9/214였지만, 모든 재생에서
"No session shows a core location with one server and not the other"였다. 1.35 그래프로 돌린 에이전트 run
(`graph135`)도 graph 0.864, nograph 0.890이었다. 그래프 데이터는 검증으로 맞다는 것을 확인했지만(2장),
에이전트가 정답을 보게 되는 데에는 차이를 만들지 못했다.

### 3.3 응답을 자르면 정보가 사라진다: 전달 방식 (11-18 ~ 11-19)

검색 결과를 자르지 않고 보내는 방식을 하나씩 시험했다. 질문은 sym-75, sym-77(×5)이다.

| run | 전달 방식 | core recall | 관찰 |
|---|---|---|---|
| `nocut` (5문항×5) | 60,000바이트 넘으면 parts | 0.72 | 큰 결과 11개가 클라이언트에서 오류로 바뀌어 에이전트에 닿지 않음 |
| `nocut2` | 40,000바이트 parts | 0.85 | 나뉜 결과 9개, `page` 요청 0회 |
| `guidesplit` | 10,000바이트 parts (강제) | 0.65 | 나뉜 결과 11개, `page` 요청 0회 |
| `guidepages` | 안내문에 parts 규칙 설명 | 0.80 | 나뉜 결과 0개라 규칙이 쓰이지 않음 |
| `guideindex` | index + `ids`로 상세 요청 | 0.75 | index 6개, `ids` 요청 0회. 에이전트는 index의 줄 범위를 직접 Read |
| `guidefile` | 파일로 저장, 경로만 응답 | 0.65 | 파일 14개 중 1개만 읽음 |
| `fullmeta` (E0, 5문항×5) | `_meta`로 한 번에 전부 | **0.84** | top-1 22/25. 최대 94,694자. 같은 문항의 nograph 0.84 |

- **결과 안의 지시는 따르지 않았다.** 다음 부분 요청 0/20, `ids` 요청 0/6, 파일 읽기 1/14.
  결과를 나누거나 미루는 방식은 모두 정보가 에이전트에 닿지 않는 경로가 되었다.
- **읽기는 병목이 아니었다.** `e2read`에서 94,056자와 108,665자 결과를 주고 내용을 묻는 질문 210개를 모두 맞혔다
  ("reading is not the bottleneck at these sizes").
- **결론**: MCP에서는 한 번의 도구 응답 안에 정답에 필요한 정보를 모두 담아야 한다. 이후 실험은 모두 `_meta`로 결과 전체를
  보낸다.

### 3.4 정답이 결과의 어디에 있는가: 오프라인 재생 (11-19)

`e1offline`은 E0 등에서 기록된 서로 다른 search_code 호출 327개를 결과 전체로 다시 돌려 정답 순위를 셌다.

| 분할 | 상위 10위 | 상위 30위 | 결과에 있음 | 순위 중앙값 |
|---|---|---|---|---|
| tune | 51 | 55 | 74 / 496 | 6 |
| holdout | 8 | 15 | 22 / 130 | 14.5 |

`walkthrough`는 sym-77 호출 하나가 입력에서 출력이 되는 과정을 따라갔다. hit 151개(94,056바이트) 중 정답은 89위와
118위였고, 동점 묶음이 465개였다. PageRank는 함수의 85%를 최솟값의 1.5배 안에 몰아 두어 순위를 거의 가르지 못했다.

### 3.5 순위 규칙 변경과 그 뒤의 하락 (11-19)

이름순 동점 정렬을 없애고 테스트를 `search_test_code`로 나눈 뒤 다시 쟀다.

| run | 비교 | 결과 |
|---|---|---|
| `rerank` (오프라인) | 같은 327개 호출 | tune 상위 10위 51 → 49, 상위 30위 55 → 54, 중앙값 6 → 6: "not worse" |
| `fullmeta2` (에이전트, 5문항×5) | E0와 같은 설정, 새 코드 | 0.600 (E0 0.84). 미리 정한 통과선 0.74 미달 |
| `pairold` / `pairnew` (P1, 같은 시각) | 옛 코드 / 새 코드 | 0.75 / 0.40. 차이 0.35 ≥ 0.3: 코드 변경이 하락의 원인일 가능성이 높음 |
| `s1oldtext` / `s1newtext` (S1) | 같은 바이너리, 옛 글 / 새 글 | 0.55 / 0.30. 차이 0.25 < 0.3: 글이 원인이라고 판정하지 못함 |

- **같은 요청에는 거의 같은 JSON이 나왔다.** 차이는 첫 검색의 폭에서 났다. 60세션을 모아 보면 sym-55는 `round|Round` 같은
  넓은 이름 검색으로 시작한 16세션이 0.88, 좁은 `func …` 정규식으로 시작한 14세션이 0.39였다. sym-77은 본문 검색으로
  시작한 13세션이 0.65, `in=name`으로 시작한 16세션이 0.22였다.
- S1 새 글에서 sym-77 첫 검색이 `in=name`인 세션은 8/10이었다(옛 글 1/10).
- 결과 키 `tests`는 0회 쓰였고 `search_test_code`도 0회 호출되었다. 이후 search_code 결과에서 테스트 목록을 뺐다.

### 3.6 `in=name`을 넓히기(D16)와 이유 단어(D18): 오프라인 (11-19)

| 실험 | 내용 | 결과 |
|---|---|---|
| D16 (`offline2/d16`) | 기록된 `in=name` 호출 72개를 `in=both`로 | tune 결과에 있음 8 → 24, 상위 10위 4 → 5 (빠진 쌍 0). holdout 11 → 11. 크기 중앙값 2,975 → 15,732자, 최대 319,258자 |
| 이유 단어 순위 키 A/B (`reason`) | 이유 단어를 순위 키로 | A만 안전. B는 결과에 있음 74 → 168이지만 틀린 이유일 때 상위 10위 49 → 41 |
| D18 B-tail (`offline2/d18`) | 이유 단어로 찾은 이웃을 hit 뒤에 | 맞는 이유: tune 결과에 있음 74 → 168, 상위 10위 49 → 73. 틀린 이유: 상위 10위 49 유지. holdout 22 → 45 (증가분 전부 sym-75) |

B-tail은 B의 tune 숫자를 본 뒤에 설계했으므로 holdout이 확인선이다.

### 3.7 에이전트로 다시 재기: 옵션보다 글이 첫 검색을 바꿨다 (11-20)

모두 sym-55, sym-77 각 5회, 같은 시각 실행, 결과 전체 전달이다.

| run | 서버 동작 | 도구 글 | core recall | sym-55 / sym-77 | sym-77 첫 검색 `in=name` | 비용 |
|---|---|---|---|---|---|---|
| `i3base` | 옵션 끔 | 기본 | 0.45 | 0.80 / 0.10 | 5/5 | $1.54 |
| `i3opts` | `--name-as-both --reason-tail` | 옵션 글 | 0.80 | 1.00 / 0.60 | 0/5 | $4.21 |
| `j1base` | 옵션 끔 | 기본 | 0.30 | 0.60 / 0.00 | 5/5 | $1.51 |
| `j1text` | 옵션 끔 | I3 opts 글 그대로 | 0.65 | 1.00 / 0.30 | 0/5 | $1.70 |
| `j1nameboth` | `--name-as-both`만 | 그 문장 하나 | 0.50 | 0.60 / 0.40 | 4/5 | $1.67 |
| `k1base` | 옵션 끔 | 기본 | 0.45 | 0.60 / 0.30 | 4/5 | $1.68 |
| `k1para` | 옵션 끔 | 안내문에 Reasons 문단만 | 0.45 | 0.50 / 0.40 | 5/5 | $1.66 |
| `k1schema` | 옵션 끔 | `reasons` schema 항목만 | 0.65 | 0.50 / 0.80 | 0/5 | $1.41 |

- **I3 판정은 "옵션이 도움이 될 가능성이 높음"(0.35)이었지만 철회했다.** sym-77에서 opts의 search_code 호출 9개 중
  `in=name`이나 `reasons`를 쓴 호출이 0개였다. 서버 동작은 base와 같았고 첫 검색만 달랐다.
- **J1**: 같은 글을 옵션 없이 보낸 arm이 0.65로 base보다 0.35 높았다(판정선 통과: 글이 recall을 움직였을 가능성이 높음).
  `--name-as-both`만 켠 arm은 +0.20으로 판정 불가.
- **K1**: 문단만 넣은 arm(+0.00)과 schema 항목만 넣은 arm(+0.20)은 recall로는 둘 다 판정 불가다. 그러나 첫 검색의 형태는
  schema 항목 하나로 갈렸다. `reasons` schema가 있는 세 arm은 sym-77 첫 검색이 15/15 본문 검색이었고 sym-55는 15/15
  `in=name` 넓은 이름 검색이었다. schema가 없는 다섯 arm은 sym-77 첫 검색이 23/25 `in=name`이었다.
- **사후 묶음**: `reasons` schema가 있는 arm 3개는 0.80, 0.65, 0.65, 없는 arm 5개는 0.30~0.50이다. 실행 전에 정한 비교가
  아니므로 판정에 쓰지 않는다.
- **related는 쓰이지 않았다.** I3에서 `reasons`를 보낸 호출은 2/14개, related는 각 273행(236행이 `lock` 한 단어로 걸림),
  결과는 최대 305,450자였다. core가 related에서 처음 나타난 경우 0/20, related 선언을 이후 읽거나 검색한 세션 0이다.
- 왜 schema 설명("words for why you search: the behaviour or mechanism the problem is about")이 첫 검색을 바꾸는지는
  관찰되지 않았다.

---

## 4. 확인된 것과 확인되지 않은 것

### 확인된 것

- 큰 결과를 나누거나, 목록만 주거나, 파일로 미루면 에이전트는 나머지를 가져오지 않는다(0/20, 0/6, 1/14).
- `_meta["anthropic/maxResultSizeChars"]`로 결과 전체를 보내면 11만 자 안팎까지 모델이 정확히 읽는다(210/210).
- 그래프 간선의 정확도는 정적 검사와 실행 관찰로 확인했다(2장 검증 숫자).
- 같은 입력에는 같은 결과가 나온다(474개 중 471개). 세션 간 차이는 주로 에이전트의 첫 검색에서 난다.
- `reasons` schema 항목 하나가 첫 검색의 형태를 바꾼다(K1, 세 arm에서 같은 방향).

### 확인되지 않은 것

- 그래프 MCP가 grep보다 낫다는 것. 21문항 전체에서 graph가 nograph를 판정선 이상으로 넘은 run은 없다.
- 첫 검색 변화에서 오는 recall 차이의 크기. arm별 차이는 판정선 아래였고, 같은 조건끼리도 0.15 차이 났다.
- sym-55, sym-77 밖의 질문에서도 같은 글이 같은 효과를 내는지.
- `--reason-tail`의 related가 에이전트에게 쓸모 있는지(쓰인 적 0회).
- `--name-as-both`의 효과(+0.20, 판정 불가). 응답이 최대 30만 자까지 커진다.

---

## 5. 남은 결정과 알려진 문제

- **D22**: 첫 검색을 바꾼 `reasons` schema를 실제 동작과 맞게 내보내는 방법. 후보는 (L1) `reasons`를 실제로 쓰고 related에
  상한을 두는 것, (L2) `reasons` 인자를 없애고 `pattern` 설명을 바꾸는 것이다. 지금 텍스트를 옵션 없이 그대로 내보내면
  서버가 무시하는 인자를 설명하게 되므로 내보내지 않는다.
- **D3 나머지**: PageRank를 순위 키로 둘지. PageRank는 인접 순서 8,150개 중 6,463개를 정했지만 값 대부분이 최솟값 근처에 몰려 있다.
- **D9**: 결과 밖 채널(클라이언트 hook)로 에이전트의 다음 행동을 바꿀지. hook 출력 필드의 동작은 확인하지 않았다.
- **안내문**: "search_code with in=name" 줄이 본문 일치가 빠진다는 것을 말하지 않는다. index 행 키 목록을 지금 키와 다시 맞춰야 한다.
- **증분 빌드**: package contains 간선이 빠지는 기존 문제가 있다(기록만 함, 고치지 않음).

---

## 6. 원자료 위치

모두 `knowledge-data/agent-eval/runs/` 아래다. 결과 문서가 없는 run은 `search-code`(수치는 `search-code-2`에),
`trace`, `trace-replay`(`analysis/TRACE-REPLAY.md`), `replay134`(`analysis/DEBUG-REPLAY.md`)이고,
`rerank`는 `offline/RESULT.md`, `walkthrough`는 `WALKTHROUGH.md`에 있다. 짝 arm의 결과 문서는 한쪽에만 있다
(pairnew, s1newtext, i3opts, j1text, k1para).

| 기간 | run |
|---|---|
| 기준선과 첫 도구 | `20261116-ks-a96c6eff`, `20261117-ks-a96c6eff-{investigate,search-code,search-code-2,search-code-2-rep10,trace,trace-replay,value-refs,value-refs-manual}` |
| 그래프 데이터와 검증 | `20261118-ks-a96c6eff-{dispatch,stmt,replay134,replay135,replay135t,verify,verify2,external,graph135,graph135y}` |
| 전달 방식 | `20261118-ks-a96c6eff-{nocut,nocut2,guidesplit,guidepages,guideindex}`, `20261119-ks-a96c6eff-{guidefile,fullmeta,e2read}` |
| 순위와 원인 분리 | `20261119-ks-a96c6eff-{e1offline,walkthrough,rerank,fullmeta2,pairold,pairnew,s1oldtext,s1newtext,reason}`, `20261119-ks-1e1003c0-offline2/{d16,d18}` |
| 옵션과 글 분리 | `20261120-ks-{i3base,i3opts,j1base,j1text,j1nameboth,k1base,k1para,k1schema}` |
