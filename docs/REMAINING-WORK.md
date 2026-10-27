# 남은 작업

기준: 2026-10-26, 로컬 브랜치 `test/retrieval-eval-grpc-go-run`(main `09be8ea2` 위, push 안 함)에 grpc-go 검색 평가의 진단과 첫 수정 결과를 반영. 정본은 [실행 원장](./spec-driven/REMAINING-13-EXECUTION.json)이다. 이 문서는 원장을 사람이 읽고 바로 이어서 일할 수 있게 풀어 쓴 것이다.

## 0. 이 문서를 다루는 규칙

- 현재 상태와 남은 일만 적는다. 끝난 단계의 경과는 쓰지 않고, 근거 패킷 링크로만 남긴다.
- 원장과 이 문서가 다르면 원장이 맞다. 이 문서를 원장에 맞춰 고친다.
- 단계를 하나 마칠 때마다 이 문서, [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md), 원장을 같은 커밋에서 갱신하고 위 기준 날짜와 커밋을 바꾼다.
- 새 세션은 이 문서의 3장 순서대로 일한다. 3장에 없는 방향으로 가려면 먼저 사용자에게 묻는다.

## 1. 맥락

### 이 시스템이 해야 하는 일

이 저장소는 코드 그래프(CKG), 벡터 검색(CKV), 둘을 묶어 MCP로 제공하는 CKS로 이루어진 지식 시스템이다. 어떤 프로젝트든 지식 DB로 만들어 두면, 개발자가 느슨하게 묻는 질문(vibe query)에 찾는 코드를 거의 한 번에 찾아 압축해 돌려줘야 한다. 이것이 이 시스템의 핵심 가치다([사용자 결정](./spec-driven/operational-preparation/retrieval-priority-2026-10-23/DECISION.json)).

1. **의도 파악**: 질문(요청)이 무엇을 원하는지 정확히 안다.
2. **관련 코드 제공**: 그 의도에 맞는 코드와 그 코드와 연결된 코드를 찾아 준다.

모든 평가와 남은 작업은 이 두 가지를 얼마나 잘하는지를 기준으로 고른다.

### 테스트 원칙

테스트 사례는 정답이 이미 확인된 것으로 고른다. 사례를 쓰기 전에 정답을 먼저 확인한다. 머지된 PR이나 수정 커밋처럼 요청 문장과 실제로 바뀐 코드가 함께 남아 있는 것이 기본 정답 원천이다.

이 원칙은 WI-V1에서 배운 것이다. WI-V1 사례(go-stablenet `eth/downloader` 구독 해제 멈춤)에는 upstream 정답이 없었고, 정답과 후보를 모두 AI가 만들었다. 정답을 뒤늦게 확인해 보니 후보 판정은 맞았지만, 12계약 가운데 11개와 단계91~94는 버그 코드와 고친 코드를 가르지 못했다([확인 기록](./spec-driven/operational-preparation/retrieval-priority-2026-10-23/DECISION.json)).

### 정식 수용과 역할

13묶음 가운데 9개(WI-1, WI-2, WI-3, WI-4, WI-V1, WI-V2, WI-6, WI-SP, WI-C)는 정식 수용으로만 완료된다. AI가 만든 정답과 실행 결과는 DEV 근거로만 쓴다. 정답과 매칭 규칙은 사람 검토자가 확정하고, 결과 수용은 최종 승인자가 정한다. 역할은 D04 A안으로 정했다([기록](./spec-driven/operational-preparation/reviewer-assignment-2026-10-22.json)). 실명과 연락처는 쓰지 않는다.

|역할|ID|하는 일|
|---|---|---|
|독립 내용 검토자, 최종 정답 작성자|gold-reviewer|AI 초안을 보고 정답과 매칭 규칙을 확정한다|
|최종 범위 승인자|owner-acceptor (사용자)|승인, 입력 동결, 결과 수용, 범위를 정한다|
|실행 담당|AI, owner-acceptor 확인|동결된 입력으로 정식 실행을 하고 로그와 해시를 남긴다|
|초안 보조|AI|초안만 쓴다. 검토자 칸과 독립성 선언은 채우지 않는다|

정답이 PR로 이미 정해진 검색 평가에서는 검토자가 정답 전체를 새로 쓰지 않는다. 질문 문장에 정답이 새지 않았는지와 PR 정답 표본이 맞는지만 확인하면 된다.

## 2. 현재 상태

- 13묶음 완료 0/13, 정식 수용 0/9, 독립 승인 0, FINAL 0
- 동결(freeze) 없음, 정식 실행 불가(`execution_allowed=false`)
- F-01~F-08 동결 결정 모두 대기. F-05는 역할만 정했고 검토자의 독립성 선언을 기다린다.
- 프로토콜의 남은 구조 조건 97개(`scripts/check-operational-acceptance.py`)
- 리팩토링 원장이 인용하는 문서 54개는 2026-10-13 문서 정리(`cd5c35d6`)로 트리에서 빠졌다. [색인](./spec-driven/REFACTORING-RETIRED-DOCUMENTS.json)이 읽을 커밋과 SHA-256을 묶고, `check-refactoring-progress.py`가 인용 경로마다 현재 파일이나 이 색인과 맞는지 검사한다.
- 우선순위: 정답이 있는 검색 평가가 1순위다. WI-V1 정식 수용 시범은 보류했다(취소가 아니다). 코딩 에이전트 묶음(WI-V1, WI-V2, WI-4, WI-C)은 검색 품질을 올린 뒤에 한다.

## 3. 지금 할 일 (이 순서를 따른다)

### 3.1 정답이 있는 검색 평가: grpc-go (최우선)

첫 대상은 [grpc-go](https://github.com/grpc/grpc-go)다. DB를 빨리 만들 수 있고, 같은 이름의 인터페이스 메서드가 여러 패키지에 반복돼 검색이 틀리기 쉬워서 골랐다. 그래프 DB는 비이력 clone 기준 70초에 만들어졌다. 고른 근거와 비교한 후보는 [결정 기록](./spec-driven/operational-preparation/retrieval-priority-2026-10-23/DECISION.json)에 있다.

1. **평가 명세 (승인됨)**: [명세](./spec-driven/RETRIEVAL-EVAL-SPEC.md)를 사용자가 2026-10-23에 승인했다. 질문은 한국어로 쓴다. 표본 규칙은 이 승인으로 고정했다. 요점은 아래와 같다.
   - 정답 원천: 머지된 PR. 요청은 PR 제목과 설명, 의도 정답은 PR 종류와 제목, 코드 정답은 PR이 바꾼 함수와 파일(시험 파일, 생성 파일은 따로 센다)
   - 질문: PR을 사람이 묻는 말투의 한국어로 바꾼다. 파일, 함수, 식별자 이름은 넣지 않고, 들어갔는지 자동으로 검사한다
   - 누설 방지: 색인은 PR의 부모 커밋으로 만든다
   - 지표: 의도 종류 정확도와 의도 문장 일치도, 정답 함수 기준 hit@1·hit@3·MRR, 같은 PR이 함께 바꾼 함수의 회수율, 정답 하나를 찾는 데 돌려준 토큰 수
   - 비교 대상: 같은 질문과 색인에 `ckv query`(벡터만), `ckv query --bm25-rerank`, `ckg query`, grep
   - 표본: PR 50~100건으로 시작. 표본 선정 규칙을 실행 전에 고정한다
   - `cks eval`이 의도 분류를 잴 수 있게 `expected_intent`, `first_hit_rank`, `rank_summary`, `intent_accuracy`를 더했다([계약](./spec-driven/EVAL-CONTRACT.md))
   - [`internal/vector/eval/prregress`](../internal/vector/eval/prregress)는 유료 LLM 에이전트를 부르므로 그대로 쓰지 않고, PR checkout과 정답 추출 방식만 참고한다
2. **입력과 도구 (완료)**: 변환 스크립트, 비교 대상 스크립트, 스냅샷 연쇄 색인 스크립트를 만들고 입력을 결과 전에 고정했다([입력](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/README.md)). PR 421건 가운데 245건이 조건을 통과해 100건을 뽑았고, 정답 범위 575개와 한국어 질문 100개, 스냅샷 24개다. 질문 누설 검사와 앵커 검사를 모두 통과했다.
3. **명세대로 실행하고 결과를 보고한다.** 임베딩 모델은 `qwen3-embedding:4b`(2560차원, 본 평가)와 `bge-m3`(기준선)다. 그래프는 스냅샷마다 한 번, 벡터는 모델마다 첫 스냅샷 전체 빌드 뒤 `reindex`로 만든다. `reindex` 전에 manifest를 이번 스냅샷의 그래프로 바꿔야 canonical ID가 맞는다(확인함). 그다음 CKS와 비교 대상 네 가지를 돌린다. 실패와 측정 못 한 값도 분모에 남긴다. 사람의 정답·질문 표본 확인(명세 7장)도 이 단계에서 받는다. **실행은 끝났고 사람의 표본 확인을 기다린다**: 2026-10-24에 두 모델의 색인 48개와 CKS·비교 대상 실행을 마쳤다([1차 결과](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/results/README.md)). PR 100건 기준 hit@1은 벡터만 0.24(qwen3)·0.20(bge-m3), CKS는 `cks eval` 그대로 0.07·0.03, 커밋 필터 없이 다시 채점하면 0.14·0.11이다. CKS 의도 정확도는 0/100·14/100이다. 실행 중 `ckv reindex`가 그래프 커밋을 갱신하지 않던 결함을 고쳤다(`9c7231e8`). 표본 확인을 받기 전까지 이 수치는 DEV 근거다. 스냅샷 커밋 판정을 어떻게 고칠지는 사용자가 정한다([현재 인수인계](./spec-driven/CURRENT-HANDOFF.md) 4장).
4. **결과에서 약한 곳을 고친다.** 의도 파악이 틀린 질문과 정답을 못 찾은 질문을 나눠 원인을 찾고, 고친 뒤 같은 표본으로 다시 잰다. **진행 중**: 2026-10-26에 첫 수정 묶음을 반영했다([진단 기록](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/diagnosis-2026-10-26/README.md)). 단계별 디버그 기록 장치를 넣고, `FindSymbol`의 수식 심볼 해석, RRF의 목록 내 중복 합산, 히트 키워드의 순환 증거를 고쳤다. CKS MRR은 qwen3 0.189 → 0.232, bge-m3 0.209 → 0.271, go-stablenet 0.75 → 0.79다. 요인 ablation으로 의도 기준값(모델에 따라 갈림), Stage 1 BM25 재정렬(언어에 따라 갈림), 테스트 강등(필요), 팩 크기(용량과 맞교환)의 효과를 쟀다. 2026-10-27에 기준을 "고칠 곳의 코드가 LLM에 건네는 팩에 들어가는가"로 정하고 두 번째 묶음을 반영했다([진단 기록](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/diagnosis-2026-10-27/README.md)). 시험 질문의 시험 파일 강등, 쓸 후보가 없는 Stage 4 칸, Stage 3이 호출자를 돌려주지 못하던 결함을 고쳤다. 정답이 팩에 있는 시나리오는 qwen3 39 → 51, bge-m3 46 → 58이고 잃은 것은 없다. MRR은 0.280, 0.317이다(벡터만 0.292, 0.291). 토큰은 약 1.5배다. `cks eval --commit-repo`로 공식 지표가 커밋 무시 값과 같아졌다(사용자 결정). 같은 날 이웃 순위를 고쳤다([이웃 순위 기록](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/neighbor-rank-2026-10-27/README.md)). Stage 4가 동점을 파일 이름 순으로 다시 정렬하던 결함과 시험 파일 이웃을 강등하지 않던 결함이다. 정답이 팩에 있는 시나리오는 qwen3 51 → 54, bge-m3 58 → 58(2건 얻고 2건 잃음)이고 토큰은 약 10% 늘었다(사용자가 예외로 채택). 정답 의도를 그대로 넣어도 팩이 거의 달라지지 않아(+1, -1) 의도 판정 개선은 뒤로 미룬다. 다음은 팩 칸 수와 토큰의 판단(12 → 16칸이면 정답 +4, +2, 토큰 약 +33% 예상)과, 후보에 오르지 못한 정답(33건, 25건)의 어휘 거리다.
5. **go-stablenet으로 같은 평가를 한다.** 이전 go-stablenet 기록과 비교할 수 있게 같은 지표를 쓴다.

### 3.2 그다음: 코딩 에이전트 묶음

검색 품질을 올린 뒤에 WI-V1, WI-V2, WI-4, WI-C를 다시 연다. 에이전트는 지식 시스템이 다 주지 못하는 부분만 맡는다.

WI-V1을 다시 열 때는 이미 확인한 정답 시험을 판정 기준으로 쓴다. 이 시험은 버그 재현(ORIGINAL 멈춤), 수정 확인(CAND-A·B 반환과 event loop 생존), 반례 차단(MUT-EARLY에서 event loop 멈춤)을 가른다([확인 기록](./spec-driven/operational-preparation/retrieval-priority-2026-10-23/DECISION.json)). 12계약은 이 시험을 중심으로 줄인다. gold-reviewer용 [검토 자료](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-GUIDE.md)는 보존하지만 지금은 진행하지 않는다.

### 3.3 사람이 정해야 하는 것

|항목|누가|상태|
|---|---|---|
|검색 평가 명세 확인|owner-acceptor|승인됨 (2026-10-23)|
|PR 정답 표본과 질문 누설 확인|gold-reviewer 또는 owner-acceptor|사용자가 AI에게 맡겨 AI 검토만 있음(2026-10-27). 사람 확인은 대기 ([자료](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/sample-check/PACKET.md))|
|스냅샷 커밋 판정|사용자|정함 (2026-10-27): `cks eval`이 파일이 그대로인 인용을 받아들인다|
|go-stablenet 질의 세트|사용자|정함 (2026-10-27): 한국어 질문 12개를 새로 만들어 고정. AI가 썼고 사람 확인 전|
|D1·D3 수정 채택(미리 정한 기준을 넘지 못함)|사용자|정함 (2026-10-27): 둘 다 채택. 사유는 [진단 기록](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/diagnosis-2026-10-27/README.md) 4.4절|
|이웃 순위 수정 채택(미리 정한 기준을 넘지 못함)|사용자|정함 (2026-10-27): D4·D5 채택, D6 제외. 원문 "A. fix-d45 채택 (Recommended)"|
|팩을 12개까지 채우며 늘어난 토큰(약 1.5배, 이웃 순위 수정으로 약 10% 더)과 팩 칸 수|사용자|미정. 칸 수별 예상은 [이웃 순위 기록](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/neighbor-rank-2026-10-27/README.md) 5장|
|N-12·N-14를 "사용자 일괄 채택(AI 검토 라벨)"로 따로 표시할지, N-10(전체 비용·p95)과 N-17(실제 운영 RTO/RPO)의 남은 범위를 별도 작업으로 뺄지|사용자|미정. 원장 상태는 바꾸지 않았다|
|WI-V1 독립성 선언과 검토 결정|gold-reviewer|보류 (3.2)|
|실제 LLM 답변 평가의 모델, 인증, 비용 상한|사용자|미정. 정하기 전에는 유료 모델을 호출하지 않는다|
|보안 전용 평가, 조직 인증, 보안 감사|사용자|보류|

## 4. 하지 않는 것

- 정답을 확인하지 않은 사례로 테스트하지 않는다. AI가 정답을 만들어야만 하는 사례는 평가 표본으로 고르지 않는다.
- 검색 평가 질문에 파일, 함수, 식별자 이름을 넣지 않는다. 색인을 PR이 머지된 뒤의 코드로 만들지 않는다.
- 검색 품질과 상관없는 성질(예: 버그 코드와 고친 코드를 가르지 못하는 계약)을 큰 실행 비용으로 반복 확인하지 않는다.
- 사용자 결정 없이 보류한 WI-V1 시범을 다시 진행하지 않는다.
- AI 초안, AI 실행 결과, 같은 AI의 재검토를 독립 정답이나 독립 승인으로 세지 않는다.
- 검토자 칸, 독립성 선언, 승인 receipt를 AI가 채우거나 추정해서 쓰지 않는다. 역할 ID도 AI가 새로 붙이지 않는다.
- 사용자 범위 승인을 정답 승인으로 바꿔 쓰지 않는다. 이미 노출된 DEV 입력을 FINAL로 쓰지 않는다.
- 봉인된 패킷(`followups/*`의 manifest에 묶인 파일)을 고치거나 다시 봉인하지 않는다. 바꿀 일이 생기면 새 패킷을 만든다.
- 원장의 과거 결정을 바꾸지 않는다. 현재 준비 근거 검사(`scripts/guardian_deadline.py`)는 기준 커밋과 다른 결정을 거부한다. 예외는 D04 역할 기록뿐이고, `scripts/d04_role_assignment.py` 검사를 통과할 때만 허용된다.
- CoreML은 사용자가 제외했다. 다시 요청하지 않고 완료 조건에도 넣지 않는다.
- 보안 전용 범위는 사용자가 보류했다. 완료로 세지 않는다.
- 실명과 연락처를 요구하거나 기록하지 않는다.
- CI에서 race 시험을 돌리지 않는다. race 시험은 PR 전에 로컬에서 돌린다.

## 5. 13묶음별 남은 일

완료 조건 원문은 원장의 `rows[].done_when`이다. "검색 평가와의 관계"는 3.1 평가 결과가 그 묶음의 근거가 되는지를 적는다.

|묶음|남은 일|막힌 곳|검색 평가와의 관계|
|---|---|---|---|
|WI-V1|12계약의 정답·매칭 확정, 승인된 실제 역할의 양성 oracle과 최종 HEAD, 독립 티켓과 반복 충분성|보류 (3.2)|에이전트 묶음. 검색 품질 뒤|
|WI-V2|선언한 비교표 전체 실행과 설치된 oracle, 독립 oracle과 수용 후보 집합, 선정 또는 결론 보류 판단|보류 (3.2)|에이전트 묶음. 검색 품질 뒤|
|WI-6|독립 nonseed 완전 정답·가족·표본 충분성, 설치된 소비자와 품질·효과 판단. warm 627쌍(warmup 57, measured 570)은 기록됨|검토자|직접 관련: 관련성·잡음|
|WI-SP|실제 policy·미태그 소비자 호출과 수동·자동 출처, 출처가 없는 것은 unknown 유지, 비보안 범위 독립 의미 검토|검토자. 보안 부분은 사용자 보류|일부: 출처 정보의 정확도|
|WI-C|설치된 검토 역할이 구조화 보고서·다이어그램·인수인계를 만드는지, 출처와 추론 구분, 독립 의미 검토|보류 (3.2)|에이전트 묶음. 검색 품질 뒤|
|WI-1|독립된 값·종류·메모리 의미 검토, 설치된 소비자 검토, 독립 완전 정답. 53곳 AI 분류와 16곳 정의 보정은 끝남, range 37곳·빈 블록 3개는 unknown 유지|검토자|일부: 제공하는 코드 분석의 정확도|
|WI-2|명령·호출자·build·MCP 지원 경계 측정, 독립 비보안 표본과 정답 매칭|검토자. 보안 전용은 보류|일부: 호출 관계의 정확도|
|WI-3|운영 입력의 가족·노출·중복과 실패 분모, 설치 역할과 기본 예산·시작·partial·provider·범위 밖 경로, 독립 관련성 검토|검토자|직접 관련: 의도와 질문 배치|
|WI-4|원래 driver·host·receiver 일정 계약 전체와 반복 실행, 선언한 정상·RED·방어·관측·재발과 정리, 독립 가족·oracle·충분성|보류 (3.2)|에이전트 묶음. 검색 품질 뒤|
|독립 수용 준비|검토자 독립성 선언, 새 DEV/FINAL, 완전 정답·매칭·충분성 검토, F-01~F-08 동결 receipt|gold-reviewer, owner-acceptor|검색 평가는 PR 정답이라 표본 확인만 필요|
|온톨로지|검토된 구현·개념 연결과 독립 검색 정답, 같은 corpus에서 켜고 끈 효과·오류·unknown 보고, 실제 LLM 답변 평가|검토자. LLM은 사용자 결정|직접 관련: 의도·개념 연결과 검색 효과|
|정식 9개 실행·수용|독립 승인된 동결 입력, 정식 실행 9회와 서명된 수용 receipt|위 모든 항목|-|
|선택 기능|6장|항목별 기준과 입력|-|

## 6. 선택 기능 (별도 대기열)

|항목|상태|
|---|---|
|임베딩 캐시|로컬 구현과 실제 검증 완료|
|공개 CLI/HTTP 형식(PUBLIC-V2)|로컬 CKS CLI·HTTP·raw graph 검증. 확장 남음|
|viewer|로컬 UI와 소유 수명 관리 검증. 선언한 한계 있음|
|대규모 corpus(SCALE)|큰 corpus·긴 함수 측정. 용량 수용 아님|
|graph 점수 조정, flow 정답, as-of, legacy 출처, 배포|미완료. 각각 기준과 입력이 필요|
|CoreML|사용자 제외 (완료 아님)|
|보안 감사(AUDIT)|사용자 보류|

## 7. 작업 절차

- 환경 준비와 전체 시험 명령은 [머신 간 인수인계](./spec-driven/MACHINE-HANDOFF.md) 2장을 따른다. 전체 `scripts` 시험 전에 `make build-bins`로 `bin/ckg`, `bin/ckv`, `bin/cks`를 만든다. `make build`는 실행 파일을 만들지 않는다.
- Python은 `scripts/requirements.txt`를 설치한 venv를 쓴다. PATH의 `go`는 go.mod가 요구하는 1.25.13 이상이어야 한다.
- 전체 `scripts` 시험은 저장소 파일을 읽으므로 한 번에 하나만 돌린다. 다른 작업은 별도 worktree에서 한다.
- Go 코드를 바꾸면 PR 전에 `make test`, `make test-race`, `make fmt-check vet golangci boundaries`를 로컬에서 통과시킨다.
- PR은 전체 `scripts` 시험이 통과한 뒤에만 올린다. CI가 통과한 PR은 스쿼시 머지하고 `main`을 맞춘다.
- 커밋 전에 `betterleaks git . --pre-commit --staged`를 돌린다. 커밋과 PR은 영어 Conventional Commits로 쓰고, 내부 용어를 넣지 않는다.
- 원장 JSON은 들여쓰기 2칸, `ensure_ascii=False`로 쓴다. `REMAINING-13-EXECUTION.json`을 바꾸면 그 해시를 DECISION-PROPOSAL, PROGRESS, PROTOCOL, GAPS의 `active_remaining_worklist`에 반영한다. 그다음 DECISION-PROPOSAL의 해시를 PROGRESS, PROTOCOL, GAPS의 `adoption_acceptance_decision_proposal`에 반영한다.
- 매 단계 끝에 검증 결과, 현재 단계, 다음 작업, 남은 전체 목록을 사용자에게 보고한다.

## 8. 정본과 근거

- [실행 원장](./spec-driven/REMAINING-13-EXECUTION.json), [운영 수용 프로토콜](./spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json), [결정 제안 D01~D08](./spec-driven/OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json)
- [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md), [머신 간 인수인계](./spec-driven/MACHINE-HANDOFF.md)
- [검색 평가 우선 결정과 WI-V1 정답 확인](./spec-driven/operational-preparation/retrieval-priority-2026-10-23/DECISION.json)
- [D04 역할 기록](./spec-driven/operational-preparation/reviewer-assignment-2026-10-22.json), [WI-V1 검토 자료](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-GUIDE.md), [WI-V1 12계약 근거](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-contract-evidence-v1/REVIEW.json)
- [CoreML 제외 기록](./spec-driven/operational-preparation/source-review-supplement/followups/user-excluded-coreml-scope-v1/SCOPE.json)
