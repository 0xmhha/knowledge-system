# 현재 인수인계

기준: 2026-11-09, main `e957da66`(PR #140까지 머지). 이 문서는 지금 단계, 진행 중인 작업, 바로 다음 행동만 적는다. 남은 전체 목록, 맥락, 하지 않을 일은 [남은 작업](../REMAINING-WORK.md)에 있고, 정본은 [실행 원장](./REMAINING-13-EXECUTION.json)이다. 새 머신이나 새 세션은 [머신 간 인수인계](./MACHINE-HANDOFF.md)로 환경부터 준비한다.

## 1. 흔들리면 안 되는 기준

이 시스템은 어떤 프로젝트든 지식 DB로 만들고, 느슨하게 묻는 질문(vibe query)에 찾는 코드를 거의 한 번에 압축해 돌려줘야 한다. 사용자가 2026-10-23에 우선순위를 정했다([결정 기록](./operational-preparation/retrieval-priority-2026-10-23/DECISION.json)).

1. 질문의 의도를 정확히 파악한다.
2. 그 의도의 코드와, 그 코드와 연결된 코드를 한 번에 찾아 준다.

테스트는 정답이 이미 확인된 사례로만 한다. 머지된 PR이나 수정 커밋이 정답 원천이다. 2026-10-27에 사용자가 판정 기준을 정했다. 원문: "중요한것은 문제를 해결할수있는 실마리 코드를 찾았는가여부야." 그래서 지금의 주 지표는 "고칠 곳의 함수가 CKS가 LLM에 건네는 팩 본문에 들어갔는가"다. 결과를 본 뒤 표본, 정답, 질문을 바꾸지 않는다.

## 2. 현재 단계

원장 `current_activity`: `106_retrieval_accuracy_v1_symbol_summary_measured_check_fix_decision_next`. [남은 작업](../REMAINING-WORK.md) 3.1의 정확도 개선 단계다. 지금 일은 [정확도 로드맵](../ACCURACY-ROADMAP.md)의 순서를 따른다. 로드맵이 놓친 원인, 제안 목록, 측정 규칙, 결정 기록의 정본이다.

### 평가 틀 (사용자 결정, 2026-11-03)

[indexer-go 평가 결정](../decisions/20261103-indexer-go-eval/decision.md)으로 평가 대상이 바뀌었다.

- **개발 표본**: go-stablenet PR 11건(한국어 증상 질문). 설계와 조정은 여기서 한다. pr72는 바뀌기 전에 있던 선언을 고치지 않아 뺐다.
- **홀드아웃**: indexer-go. grpc-go를 대신한다(D1). go-ethereum 어휘를 go-stablenet과 함께 쓰고, 공개 모델이 미리 알 가능성이 낮고, 저자가 정답을 확인할 수 있어서다. `fix` 커밋 36건에서 증상이 겉으로 보이는 것만 남기고, 정답 함수와 질문은 사용자가 PR마다 확인한다(D2~D4). 확인하지 않은 PR은 세지 않는다.
- **판정(D6)**: go-stablenet의 이득은 보고한다. indexer-go에서는 두 임베딩 모두 잃은 PR이 얻은 PR보다 많으면 안 된다. 같은 이름 선언이 3개 이상인 PR(위험 부분집합)은 한 건도 잃으면 안 된다.
- **비교 조건(D8)**: 두 모델을 같은 조건에 둔 `cks-bge-m3-eq`, `cks-qwen3-eq`. PR마다 그 PR의 기준 커밋으로 색인한다. `code_query`는 다른 에이전트가 질문만 보고 쓰고 측정 전에 고정한다.
- **범위(D7)**: 검색 단계만 잰다. 진단 품질은 따로 평가한다.
- indexer-go를 판정에 쓴 횟수를 센다. 지금 6번이다. 본문 선택을 바꾸는 변경은 일반적이라 부르기 전에 새 홀드아웃으로 다시 확인한다.

### 지금의 파이프라인 기본값

그림과 상수는 [검색 파이프라인](../RETRIEVAL-PIPELINE.md)에 있다.

- 호출하는 에이전트가 질문을 코드 어휘로 다시 쓴 `code_query`를 넘기면 Stage 1이 그것으로 먼저 검색한다(#128). core-dev analyzer와 planner는 항상 넘긴다(stablenet-expert #58).
- 팩 본문은 최대 12개다. seed 할당량 10개, 이웃 예약 2개(#130). knowledge 예약 2개는 12개 밖에 더한다(#134).
- 벡터와 그래프가 같은 함수를 다른 줄 범위로 가리키면 한 후보로 합치고 점수를 더한다(#135).
- 할당량에 잘린 seed는 본문 없이 `cut_candidates`로 이름과 위치만 남긴다(#137).

### 지금 수치 (AI 정답, 사람 확인 전)

go-stablenet 11건에서 정답 함수가 팩 본문에 든 PR 수(#135 뒤):

|임베딩|든 PR|놓친 PR|
|---|---|---|
|bge-m3|8|pr-63(아예 못 찾음), pr-67·pr-73(찾았지만 seed 19위, 16위에서 잘림)|
|qwen3|10|pr-63|

놓치는 원인은 어휘 차이, 구조 문제(확장 지점을 묻는 질문), 임베딩 모델 차이 셋이다(로드맵 2장).

### 진행 중: V1 심볼 요약 (멈춤, 사용자 결정 대기)

로컬 LLM(gemma3:12b)이 10줄 이상 심볼마다 "무엇에 쓰고 언제 바꾸는지"를 영어와 한국어로 쓰고, 그 요약을 함수와 같은 줄 범위의 청크로 색인한다(#138, `--llm-summary-model`, 기본은 꺼짐).

|표본|임베딩|요약 전|요약 후|얻음|잃음|
|---|---|---|---|---|---|
|go-stablenet 11건|bge-m3-eq|8|9|1|0|
|go-stablenet 11건|qwen3-eq|10|10|0|0|
|indexer-go 26건 중 15건|bge-m3-eq|7|6|0|1|
|indexer-go 26건 중 15건|qwen3-eq|8|9|1|0|

bge-m3가 indexer-go에서 1건을 잃어 D6에 걸렸다. 만든 그대로는 채택하지 않았다. 잃은 PR은 정답 함수 셋 모두 요약이 있었는데, 요약으로 순위가 오른 다른 본문이 정답을 밀어냈다. 이어 가는 법, 작업 폴더, 명령은 [심볼 요약 인수인계](../vector/SYMBOL-SUMMARY-HANDOFF.md)에 있다.

### 끝난 것

|PR|내용|
|---|---|
|#121~#127|검색 평가를 1순위로 정한 결정, `cks eval`의 의도·순위 지표, grpc-go 입력 고정과 연쇄 색인, 1차 실행과 첫 수정 묶음, venv 주의|
|#128|v2 평가(PR마다 기준 커밋으로 색인, [결정](../decisions/20261028-retrieval-eval-v2/decision.md))와 `code_query`. grpc-go 시험용 56건에서 정답이 팩에 든 PR이 bge-m3 23 → 36, qwen3 18 → 40. 실행마다 덮어쓰는 `CKV_CHUNK_BUDGET_BYTES`, `CKS_INTENT_UNKNOWN_THRESHOLD`|
|#129|보존 소스에 없는 파일을 색인한 데이터셋을 시작할 때 `serviceable=false`로 알림|
|#130|이웃 예약 4칸 → 2칸|
|#131|`find_symbol`이 정의마다 `canonical_id`를 돌려줘 모호한 seed를 고를 수 있게 함|
|#132|go-stablenet Anzeon tip 갱신 함정 항목(`needs_verification`)|
|#133|core-dev `/core-dev:diagnose` 절차의 go-stablenet 평가 기록. 성능 주장이 아닌 서술이다|
|#134|knowledge 예약을 본문 상한 밖으로 뺌|
|#135|같은 함수의 벡터·그래프 후보를 하나로 합침|
|#136|[검색 파이프라인](../RETRIEVAL-PIPELINE.md) 그림과 Transformer 비교|
|#137|잘린 seed를 `cut_candidates`로 남김|
|#138|V1 심볼 요약 코드, [정확도 로드맵](../ACCURACY-ROADMAP.md), [심볼 요약 인수인계](../vector/SYMBOL-SUMMARY-HANDOFF.md)|
|#139|[indexer-go 평가 결정](../decisions/20261103-indexer-go-eval/decision.md)과 후보 목록 스크립트|

기각한 것: `code_query`에서 키워드 seed를 뽑는 실험은 go-stablenet에서 두 번 PR을 잃었다(2026-11-04). 질문과의 벡터 관련도로 이웃을 고르는 설계는 정답 이웃이 8~38위라 기각했다(2026-10-27).

grpc-go 평가의 입력과 결과는 그대로 남긴다. [grpc-go 입력](./operational-preparation/retrieval-eval-grpc-go-v1/README.md), [1차 결과](./operational-preparation/retrieval-eval-grpc-go-v1/results/README.md), 진단 기록 세 개(`diagnosis-2026-10-26`, `diagnosis-2026-10-27`, `neighbor-rank-2026-10-27`)다. 스냅샷 연쇄 색인을 다시 돌리는 법은 `scripts/retrieval_snapshot_chain.py`와 1차 결과 문서에 있다.

### 임베딩 모델 (사용자 결정)

- `qwen3-embedding:4b`(2560차원, 기본 출력이라 `--embed-dim`을 주지 않는다)와 `bge-m3`(1024차원) 두 개만 쓴다. 사용자가 "그냥 다른 모델 받지 말고"라고 정했다.
- 요약 모델은 임베딩 모델이 아니다. gemma3:12b를 쓰고, gemma4:12b가 비교 후보로 받아져 있다.

## 3. 작업 폴더와 머신

- 평가 작업 폴더는 저장소 밖 `knowledge-data/retrieval-eval-v2`다. 작업 폴더 안에 바이너리(bin-sum), 연쇄 색인 스크립트(tools/chain_sum.py), 실행·판정 스크립트(tools/run_sum.py, tools/judge_d6.py, tools/funnel_c.py, tools/run_exp.py), 실험 기록(EXP-*.md), 요약 캐시가 있다. 이 경로들은 저장소 안의 경로가 아니다. 원응답과 색인은 커밋하지 않는다.
- 이 폴더는 V1 실험을 돌린 빌드 머신(M2 Max)에만 있다. 2026-11-09 기준 다른 머신에는 없다. 다른 머신에서 이어 가려면 폴더를 옮기거나 [심볼 요약 인수인계](../vector/SYMBOL-SUMMARY-HANDOFF.md) 2장의 구성으로 새로 만든다.
- 빌드 머신은 부하가 걸리면 어댑터 정격(약 98W)만큼 써서 충전 중에도 배터리가 준다. 긴 실행 전에 배터리 보호 스크립트를 다시 만든다(같은 문서 5장).
- grpc-go v1 작업 폴더 `knowledge-data/grpc-go-retrieval-eval`도 같은 사정이다.

## 4. 바로 다음 행동 (이 순서를 따른다)

1. **V1 검사 수정을 사용자에게 받는다.** 2026-11-06에 제안했고 아직 승인되지 않았다. 답을 받기 전에는 시작하지 않는다.
   - `Eligible`에서 Go 생성 코드 머리말(`// Code generated ... DO NOT EDIT.`)이 있는 파일을 건너뛴다. indexer-go 요약 거부율 12%의 대부분이 gqlgen 생성 코드다.
   - `Validate`에서 식별자를 대소문자 없이 맞춘다. 지금은 코드가 `graphql`이면 문장 속 "GraphQL"을 없는 이름으로 보고 버린다.
   - 고친 검사로 indexer-go 남은 11건을 빌드하고, 처음 보는 표본으로 판정해 앞의 15건과 따로 보고한다.
   - 두 결함은 판정 결과가 아니라 빌드 수치에서 찾았다. 그래서 고쳐도 홀드아웃에 맞춘 조정이 아니다.
2. **동명 본문 제한 변경의 상태를 확인한다.** indexer-go 결정 D6은 첫 적용 대상으로 커밋 `79c938d1`을 적는다. 이 커밋은 원격 어느 브랜치에도 없고, 측정 결과도 로드맵에 없다. 빌드 머신의 로컬 브랜치와 `EXP-same-name.md`를 보고 채택, 기각, 진행 중 가운데 무엇인지 로드맵 결정 기록에 적는다.
3. **로드맵 순서를 이어 간다.** V1이 이득을 보이면 J1(청크와 심볼을 잇는 표)로 간다. Q1(seed에서 시작하는 personalized PageRank)은 질의만 바꾸는 실험이라 언제든 할 수 있다. 그다음 G1~G3, 시간으로 나눈 학습 세트가 생기면 V4다. 바꿀 때마다 측정 전에 판정 규칙을 적고 D6으로 판정한다.
4. **사람의 확인을 받는다.** 아래는 모두 AI가 만들었고 사람 확인 전이다. 확인할 때는 결과를 이미 본 뒤인지 함께 묻고 그대로 기록한다.
   - grpc-go 정답·질문 표본: [표본 확인](./operational-preparation/retrieval-eval-grpc-go-v1/sample-check/PACKET.md)(seed 20261024). 2026-10-27에 사용자가 AI에게 맡겨 AI 검토만 있다.
   - v2 정답 함수 라벨(grpc-go, go-stablenet).
   - go-stablenet 한국어 질문 11개.
   - indexer-go 증상 질문과 정답 함수(D3, D4). 확인 기록이 빌드 머신 작업 폴더에 있는지 이 문서 기준으로는 확인하지 않았다.
   - Anzeon tip 갱신 함정 항목(#132, 도메인 담당자).
5. **근거의 정밀도를 함께 잰다(로드맵 4.5, 사용자 결정 2026-11-09).** "맞는 근거 몇 개와 모른다는 표시가 틀린 근거 여럿보다 낫다"는 원칙을 이 프로젝트에서 아직 재지 않았다. 지금 지표는 모두 정답이 팩에 들었는지만 봐서, 본문을 더 넣는 변경이 늘 불리하지 않게 보인다.
   - P0: `cks eval`에 `answer_body_count`, `body_precision`, `other_body_tokens`를 더했고, 실험용으로 팩 본문 수를 바꾸는 `CKS_PACK_MAX_BODIES`를 더했다(기본값은 그대로). 다음 측정부터 go-stablenet과 indexer-go 결과에 함께 내고, 본문 3, 6, 12개 곡선을 낸다. 고정된 입력이 빌드 머신에 있으므로 거기서 돌린다.
   - P1: 같은 진단 과제를 본문 12개 팩과 본문 3개 + `cut_candidates` 팩으로 돌려 비교한다. 사례, 반복 수, 비용 상한은 사용자가 정한다. 기록된 진단 사례 4건은 12개 팩으로 모두 C1을 통과해 차이가 드러날 여지가 작다. 그래서 팩이 정답을 놓친 PR(pr-63, pr-67, pr-73)을 더하는 안을 로드맵에 적었다.
   - P2(본문 수를 점수 차이로 줄이기)와 P3(확신 신호, 모르는 의도 표시)는 P1 결과를 본 뒤에 한다.
6. **팩 크기와 토큰을 다시 잰 뒤 묻는다.** 2026-10-27의 "12칸 → 16칸이면 정답 +4, +2, 토큰 약 +33%"는 이웃 예약이 4칸이던 때의 예상이다. #130과 #134 뒤에 다시 재고 사용자에게 묻는다.

단계를 하나 마칠 때마다 이 문서, [남은 작업](../REMAINING-WORK.md), 원장, [정확도 로드맵](../ACCURACY-ROADMAP.md)을 같은 커밋에서 갱신한다.

## 5. 이미 확인한 사실과 주의할 점

- **복사한 벡터 색인을 `reindex`하기 전에 그래프를 바꿔 가리켜야 한다.** 복사한 manifest는 앞 커밋의 그래프를 가리킨다. 그대로 `reindex`하면 canonical ID가 전체 빌드와 629개 달랐다. `scripts/retrieval_snapshot_chain.py`는 `sources.ckg.path`를 바꾼 뒤 `reindex`한다. 그래도 패키지 convention 요약 청크 15,860개 중 12개가 다르다. 이것은 `ckv reindex`의 버그로 보고 나중에 고친다.
- **`9c7231e8` 전의 `ckv`로 `reindex`한 색인은 그래프 커밋이 틀려 있다.** CKS가 모든 요청을 거절한다. 새로 색인할 때는 그 뒤의 `ckv`를 쓴다.
- **청크 상한은 `bge-m3`에만 있다.** `pkg/vector/embed/ollama/adapter.go`의 `chunkBudgetBytes`는 모델 이름이 `bge-m3`일 때만 6,144바이트를 돌려준다. 두 모델을 같은 조건에 두려면 `CKV_CHUNK_BUDGET_BYTES`를 준다(#128). v2에서 상한을 맞춰도 grpc-go 결과는 달라지지 않았다(벡터만 qwen3 34건 대 34건). 상한을 모델 입력 창에서 정하는 설계는 남은 후보다. 모델 이름으로 분기하지 않는다.
- **긴 청크가 잘려서 임베딩된 실행이 있다.** 2026-10-23~26에 세션 임시 디렉터리에서 grpc-go 24개 스냅샷을 따로 색인했을 때 `qwen3-embedding:4b`가 64번, `bge-m3`가 2번 잘렸다. 2026-10-24 실행 로그에는 둘 다 0건이다. 왜 다른지는 아직 확인하지 않았다.
- **의도 기준값 0.55는 `bge-m3`에 맞춰져 있다.** qwen3에서는 대부분 미분류가 된다. 같은 조건 비교에서는 `CKS_INTENT_UNKNOWN_THRESHOLD`로 모델마다 준다(v2 가정: grpc-go 질문 100개에서 모델별 최고 기준 문장 유사도의 중앙값). 정답 의도를 그대로 넣어도 팩은 거의 달라지지 않았다(+1, -1). 그래서 의도 판정 개선은 뒤로 미뤘다.
- **`ckg query`는 한국어 질문에 자주 "no usable tokens" 오류를 낸다.** 오류도 실패로 센다.
- **요약 모델은 manifest에 기록되지 않는다.** 지금은 요약 캐시 키에만 있다. 요약을 켠 색인과 끈 색인을 manifest로 구별할 수 없다.
- **새어 들어가면 안 된다.** 이력이나 LLM으로 만든 지식(V1, V2, V4, G2)은 평가 PR 기준 커밋 뒤의 정보를 보면 안 된다. 요약은 기준 커밋의 코드만 보고 만든다.
- **`pplx-embed-v1-4b`는 쓰지 않는다.** 이 네트워크에서 Hugging Face에 닿지 않고, 양방향 구조라 Ollama에서 돌기 어렵다.

## 6. WI-V1 근거 (보류 중, 봉인된 패킷은 고치지 않는다)

|패킷|담은 것|
|---|---|
|[wiv1-reviewer-packet-v1](./operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-PACKET.json)|검토자에게 넘긴 AI 초안과 빈 검토자 칸|
|[wiv1-contract-evidence-v1](./operational-preparation/source-review-supplement/followups/wiv1-contract-evidence-v1/REVIEW.json)|12계약마다 단계90~94 근거 인용 36개와 남은 공백|
|[wiv1-progress-true-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/SUMMARY.json)|동기화 도중 true 알림의 세대 경계|
|[wiv1-notify-transport-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-notify-transport-generation-v1/SUMMARY.json)|Notify 도중 종료와 gate 없는 연결 종료|
|[wiv1-transport-timer-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json)|연결 종료와 같은 channel timer broadcast|
|[wiv1-generation-boundary-v1](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json)|owner ack 뒤 옛 세대 사용과 새 등록 경계|
|[factory-native-overlap-v1](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json)|원래 60초 timer와 RPC callback 겹침|
|[owner-schedule-contract-v1](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json)|사용자가 승인한 실제 4·통제 1 범위|

모든 근거는 AI가 만든 DEV 근거다. 어느 것도 독립 정답이나 독립 승인이 아니다. 13묶음 완료 0/13, 정식 수용 0/9, 독립 승인 0, FINAL 0, freeze 없음, `execution_allowed=false`다.
