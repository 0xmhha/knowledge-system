# grpc-go 검색 평가 진단과 수정 (2026-10-26)

[1차 결과](../results/README.md)에서 CKS가 벡터만 쓴 검색보다 낮았다. 이 문서는 그 원인을 단계별로 찾고, 고치고, 같은 표본으로 다시 잰 기록이다. [남은 작업](../../../../REMAINING-WORK.md) 3.1의 4단계(약한 곳 고치기)에 해당한다. 입력(표본, 정답, 질문)은 바꾸지 않았다. 사람의 표본 확인은 아직이므로 모든 수치는 DEV 근거다.

지표는 grpc-go가 PR 100건의 hit@1과 MRR이다. 녹화한 응답을 커밋 필터 없이 다시 채점한 값(`cks_any_commit`)을 쓰며, 이유는 1차 결과 2장에 있다. go-stablenet은 PR 12건의 파일 단위 r@5, r@10, MRR이다(저장소 밖 `knowledge-data/go-stablenet-pr-eval-20261025`).

## 1. 결과 요약

|빌드|커밋|grpc-go qwen3 hit@1 / MRR|grpc-go bge-m3 hit@1 / MRR|go-stablenet r@10 / MRR|
|---|---|---|---|---|
|1차 결과|`8f7d39a6`|0.14 / 0.189|0.11 / 0.209|0.64 / 0.75|
|디버그 기록 추가|`47321f4a`|0.14 / 0.189|0.11 / 0.209|0.64 / 0.75|
|수식 심볼 해석 수정|`1b0fb26e`|0.15 / 0.199|0.13 / 0.234|0.64 / 0.75|
|RRF 목록 내 중복 제거|`6dbd3834`|0.15 / 0.206|0.14 / 0.243|0.65 / 0.79|
|히트 키워드를 결합에서 제외|`9e4a0f0b`|**0.16 / 0.232**|**0.17 / 0.271**|0.65 / 0.79|
|비교 대상: 벡터만|—|0.24 / 0.292|0.20 / 0.291|0.55 / 0.63|

`6dbd3834` 줄은 `1b0fb26e`와 `6dbd3834`의 두 수정을 함께 넣은 빌드의 값이다. 각 수정만 따로 넣은 값은 3장에 있다. 마지막 줄의 `9e4a0f0b`는 커밋한 트리에서 빌드한 바이너리로 다시 쟀고, 그 결과 팩이 4장의 ablation 변형(`abl-nohitkw`)과 grpc-go 200건, go-stablenet 12건 모두 같았다.

CKS는 1차 결과보다 MRR이 qwen3 +0.043, bge-m3 +0.062, go-stablenet +0.04 올랐다. grpc-go에서는 아직 벡터만 쓴 검색보다 낮다. go-stablenet에서는 이미 벡터만 쓴 검색보다 높다.

## 2. 디버그 기록 장치 (`47321f4a`)

`CKS_DEBUG_TRACE=<파일>`을 설정하면 cks가 compose 호출마다 JSON 한 줄을 덧붙인다(`internal/system/debugtrace`). 각 알고리즘에 들어간 것(before)과 나온 것(after)이 담긴다.

- 의도 분류: 의도별 최고 유사도, 가장 가까운 의도, 기준값, 미분류 여부
- Stage 1: 라운드별 ckv 질의와 히트, 병합, 키워드 후보와 출처(`prompt`, `prompt_compound`, `prompt_morph`, `hit_symbol`, `hit_file`), BM25 재정렬 점수와 탈락 이유, 신뢰도 판정, 키워드 상한
- Stage 2: 키워드마다 BM25 목록과 FindSymbol 목록(개수, 서로 다른 위치 수), 강등 전과 후의 RRF 점수와 그 출처
- ckgclient: 심볼 이름을 해석하려고 시도한 순서와 각 시도의 노드 수
- Stage 3: 시드마다 이웃, 최종 이웃 순위
- Stage 4: 후보마다 선택 또는 탈락 이유(`seed_quota`, `reserve_holdback`, `over_budget`, `citation_cap` 등). 이전에는 `Skipped`에 넣는 세 경우만 보였다

`scripts/retrieval_debug_trace_report.py`는 기록을 시나리오와 프롬프트로 맞춘다. 그리고 체크포인트마다 정답 순위, 정답이 1위를 처음 잃은 단계, 정답이 처음 사라진 단계, 실패 모양 플래그 10종을 낸다. 기록을 켠 빌드의 결과 팩은 끈 빌드와 grpc-go 200건, go-stablenet 12건 모두 같았다.

## 3. 찾은 결함과 수정

### 3.1 수식 심볼이 수신자를 잃음 (`1b0fb26e`)

ckv 히트의 심볼은 `outlierDetectionBalancer.RemoveSubConn` 같은 `타입.메서드` 형태다. 그래프는 이것을 `패키지.타입.메서드`로 저장한다. SQLite `FindSymbol`은 `qualified_name`이나 `simple_name`(마지막 마디)이 정확히 같을 때만 찾으므로, 이 형태는 어느 쪽에도 맞지 않았다. `8ac9dbcb`(#31)가 `qualified_name LIKE '%.' || name`을 `simple_name` 비교로 바꾸면서 빠진 경우다. Postgres 저장소는 지금도 이 형태를 찾는다.

찾지 못하면 ckgclient가 앞 마디를 떼고 `RemoveSubConn`으로 다시 찾는다. 그러면 저장소의 같은 이름 메서드 30개가 모두 나온다. 키워드 하나가 평균 19개 노드로 퍼졌고, 이런 확산이 qwen3에서 52건 있었다. 마지막 마디를 인덱스로 찾은 뒤 `"." + 이름`으로 끝나는 행만 남기는 UNION 갈래를 더했다. 질의는 인덱스를 그대로 탄다.

수정만 넣은 측정은 grpc-go MRR qwen3 0.189 → 0.199, bge-m3 0.209 → 0.234이다. 나빠진 PR은 bge-m3에서 1건(2위 → 3위)이다. go-stablenet은 바뀌지 않았다. 영어 프롬프트에서는 이 확산이 0건이었다.

### 3.2 RRF가 한 목록에서 같은 위치를 여러 번 셈 (`6dbd3834`)

인터페이스를 임베드한 Go 타입마다 승격 메서드 노드가 생기고, 이 노드의 위치는 인터페이스 선언이다. 그래서 맨 이름 `RemoveSubConn`의 FindSymbol 목록에 `balancer/balancer.go:151`이 21번 나왔다. aggregator는 이 21번을 모두 더했고, 그 위치가 0.855점(정상 1위의 약 7배)으로 다른 balancer를 묻는 질문의 1위가 됐다. 한 목록에서는 한 위치를 가장 좋은 순위로 한 번만 세고, 순위는 서로 다른 위치끼리 매기게 바꿨다. RRF가 정의하는 방식이다.

수정만 넣은 측정은 grpc-go MRR qwen3 0.189 → 0.198, bge-m3 0.209 → 0.227, go-stablenet 0.75 → 0.79이다. 나빠진 PR은 qwen3에서 1건(12위 → 13위)이다.

### 3.3 히트에서 뽑은 키워드가 같은 히트에 다시 표를 줌 (`9e4a0f0b`)

Stage 1은 프롬프트의 식별자와 함께, 방금 받은 ckv 히트의 심볼과 파일 이름도 키워드로 썼다. Stage 2는 키워드마다 BM25 목록과 FindSymbol 목록을 RRF에 넣는다. 히트에서 나온 키워드는 대개 그 히트를 1위로 돌려주므로, 벡터 검색이 이미 한 판단을 독립 증거처럼 한 번 더 셌다.

그 크기는 이렇다. 키워드 하나가 1위로 맞으면 +0.041(BM25 1/61 + FindSymbol 1.5/61)을 준다. 반면 ckv 목록의 1위와 30위 차이는 0.026이다. 그래서 최종 순서가 "어느 히트의 이름이 키워드 상한 8개 안에 들었나"로 정해졌다. grpc-go qwen3 최종 키워드의 83%가 히트에서 나왔다. 한국어 질문에는 식별자가 없기 때문이다.

이제 Stage 2에는 프롬프트에서 나온 키워드만 넘긴다. 히트 키워드는 재정렬 신뢰도와 다음 라운드 질의 보강에만 쓴다. 상한은 그 뒤에 적용한다. `stage1.Config.KeepHitKeywords`를 켜면 이전 동작으로 돌아간다. 효과는 4장의 ablation에서 쟀다.

### 3.4 시나리오 좌표 (`436ff2b0`)

`47321f4a`가 `composer.go`에 import 한 줄과 기록용 래퍼를 더해, `composer-err-fail-closed` 시나리오의 기대 범위 두 개가 1줄, 24줄씩 밀렸다. 같은 심볼, 같은 원문을 가리키도록 범위를 옮겼다. 이 커밋 전부터 범위 밖이던 anchor 7개는 그대로 두었다. 대상은 `ckg-bm25-translation` 2개, `concurrency-safety-real-adapters`, `stamp-integrity-lookup` 3개, `structural-qname-filter`다.

## 4. 요인 ablation (`436ff2b0` 기준)

결과를 보기 전에 읽는 기준을 [`ABLATION-PLAN.md`](./ABLATION-PLAN.md)에 고정했다. 측정용 빌드(`builds/abl/ablation.diff`, 커밋하지 않음)는 요인마다 환경 변수 스위치를 하나씩 둔다. 스위치를 모두 끈 빌드는 HEAD 동등 빌드와 grpc-go 200건, go-stablenet 12건 모두 같았다. 핵심 요인의 기준은 grpc-go 두 모델이 같은 방향으로 MRR ±0.02 이상 움직이거나, go-stablenet MRR이 ±0.03 이상 움직이는 것이다.

|변형|바꾼 것|qwen3 MRR|bge-m3 MRR|go-stablenet MRR (r@10)|판정|
|---|---|---|---|---|---|
|기준|—|0.206|0.243|0.79 (0.65)|—|
|`abl-s1norr`|Stage 1 BM25 재정렬 끔|−0.026|+0.033|−0.15 (0.62)|핵심. 언어에 따라 방향이 갈림|
|`abl-nohitkw`|히트 키워드를 결합에서 제외|+0.027|+0.028|0 (0.65)|핵심. 손해 없음 → 3.3으로 반영|
|`abl-nokw`|키워드 목록 전부 제외|+0.029|+0.030|+0.10 (0.57)|핵심. 재현율과 맞교환|
|`abl-nr0`|Stage 4 이웃 몫 0|+0.001|0|0 (0.67)|영향 없음|
|`abl-mc20`|팩 12개 → 20개|+0.010|+0.004|0 (0.64)|용량 요인|
|`abl-it0`|의도 기준값 0|+0.029|−0.003|0 (0.61)|핵심. 모델에 따라 갈림|
|`abl-notd`|테스트 파일 강등 끔|−0.005|−0.029|−0.08 (0.65)|핵심. 강등 유지|

변형별 원자료는 `ablation-results.json`과 `grpc-go/<변형>/`, `go-stablenet/<변형>/`에 있다.

## 5. 해석

**키워드의 출처가 가장 큰 축이고, 그 출처는 프롬프트 언어가 정한다.** go-stablenet 최종 키워드는 모두 프롬프트에서 나왔다(96개 중 96개). 그래서 히트 키워드를 빼도 바뀌지 않는다. 대신 Stage 1 BM25 재정렬을 끄면 MRR이 0.15 떨어진다. 영어 프롬프트의 식별자를 끌어올리는 일이 이 재정렬의 몫이다. grpc-go는 반대다. 키워드의 83%가 히트에서 나오고, 재정렬의 효과는 모델에 따라 엇갈린다. 프롬프트 키워드까지 모두 빼면 go-stablenet MRR은 0.89까지 오르지만 r@10이 0.65에서 0.57로 떨어진다. 프롬프트 키워드는 깊은 순위의 정답을 끌어오는 역할을 한다.

**bge-m3가 CKS에서 앞선 이유는 모델이 아니라 의도 기준값이다.** 의도 분류기는 ckv와 같은 임베딩 모델로 기준 문장과의 유사도를 잰다(`cmd/cks/mcpcli/serve.go`의 `intentEmbedderAdapter`). 미분류 기준값 0.55는 bge-m3에 맞춘 값이다.

|grpc-go|qwen3|bge-m3|
|---|---|---|
|의도 유사도 중앙값|0.426|0.545|
|0.55 기준값 통과|4건|43건|
|가장 가까운 의도 = 정답 의도|37건|29건|
|ckv 상위 8개 중 `.md` (질의당)|0.79|0.22|
|Stage 2 1위가 `.md`|8건|1건|

qwen3는 의도를 더 잘 고르는데도 96건이 미분류가 된다. 미분류이면 문서 강등이 꺼진다(`internal/system/composer/stage2/docpath.go`의 `demoteDocsFor`). 그러면 문서를 많이 끌어오는 qwen3에서 `.md`가 1위 오답이 된다. 정답에 `.md`가 있는 시나리오는 0건이다. 벡터만 쓴 검색과 Stage 1까지는 qwen3가 같거나 앞선다(Stage 1 MRR 0.262 대 0.249).

`abl-it0`(기준값 0)으로 인과를 확인했다. qwen3 MRR은 0.206에서 0.234로 올라 bge-m3(0.240)와 거의 같아졌다. 의도 정확도는 qwen3 0.00에서 0.37로, bge-m3 0.14에서 0.29로 올랐다. 다만 bge-m3에서 나빠진 3건은 모두 의도가 틀린 경우였다. 기준값을 없애는 것이 답이 아니라, 모델의 유사도 크기에 기대지 않는 판정이 필요하다는 뜻이다. 정답 의도는 AI가 붙였고 사람이 확인하지 않았다.

**Stage 4의 팩 상한이 정답을 자른다.** Stage 4까지 온 정답이 빠진 경우는 모두 Stage 2에서 9위 이하였다(qwen3 25건, bge-m3 22건). 팩은 12개이고, 이웃 4개, 지식 2개, 정확 심볼 2개를 예약한다. 이웃 몫만 0으로 하면 예약 슬롯이 대신 자른다. 팩을 20개로 늘리면 LLM이 받는 정답 비율이 qwen3 0.37에서 0.52로 오르지만, 토큰이 약 2배(956 → 1,867)가 된다. 순위 요인이 아니라 비용과 맞바꾸는 문제다.

## 6. 아직 판단하지 않은 것

- 의도 판정을 모델에 맞추는 방법. 후보는 상대값 판정(1위와 2위 의도의 유사도 차이, 기준 문장 분포로 정규화한 점수)과, 임베딩 정체성에 묶은 보정 프로필이다. 모델 이름 분기는 새 모델이나 태그가 바뀌면 같은 문제를 되풀이하므로 후보에서 뺐다.
- 의도가 미분류일 때 문서 강등을 끄는 지금 정책을 바꿀지
- Stage 1 BM25 재정렬을 프롬프트에 식별자가 있는지에 따라 켜고 끌지
- 팩 크기와 예약 슬롯
- 요인을 함께 바꿀 때의 상호작용. 이번에는 하나씩만 쟀다
- 사람의 정답·질문 표본 확인, 스냅샷 커밋 판정 방식(1차 결과 5장)

## 7. 파일

- `ABLATION-PLAN.md`: 결과를 보기 전에 고정한 ablation 계획과 읽는 기준
- `ablation-results.json`: 4장 표의 원자료([`ablation_table.py`](./tools/ablation_table.py)가 만든다)
- `builds/<빌드>/`: 바이너리 해시와 측정용 패치
  - `dbg`: 디버그 기록만
  - `fix51`, `fix52`, `fix5152`: 수정 3.1, 3.2, 그 둘
  - `abl`: ablation 스위치
  - `head9e4a`: 커밋 `9e4a0f0b`를 그대로 빌드
- `grpc-go/<빌드 또는 변형>/`
  - `summary-cmp.json`: `scripts/retrieval_eval_summary.py` 출력
  - `pack-diff-<모델>.json`: 기준 빌드와 다른 팩
  - `debug-report-<모델>.json`: 디버그 기록 보고서의 요약과 시나리오별 순위·플래그. 전체 기록은 작업 폴더에 둔다
- `go-stablenet/<빌드 또는 변형>/`: `score_requery.py` 출력과 디버그 기록 보고서 요약
- `tools/`: 작업 폴더에서 쓴 실행·집계 스크립트
- `manifest.json`: 위 파일과 커밋의 해시

전체 디버그 기록(`debug-trace.jsonl`)은 변형마다 수십 MB여서 저장소에 넣지 않았다. 위치는 `knowledge-data/grpc-go-retrieval-eval/eval-<태그>/eval/<모델>/<스냅샷>/`과 `knowledge-data/go-stablenet-pr-eval-20261025/requery/<태그>/`이다.

다시 재는 법(작업 폴더에서):

```sh
python3 run_qual.py --model qwen3-embedding:4b --tag <태그> [--bin <빌드>] [--env CKS_ABL_...=...]
python3 report_variant.py --tag <태그> --base <기준 태그>
python3 scripts/retrieval_debug_trace_report.py --trace <debug-trace.jsonl> --scenarios <시나리오 폴더> --out report.json
```
