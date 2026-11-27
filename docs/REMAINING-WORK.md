# 남은 작업

기준: 2026-11-27, 브랜치 `feat/ckg-value-references`의 로컬 커밋 `4604cb5d`(push하지 않음). 정본은 [실행 원장](./spec-driven/REMAINING-13-EXECUTION.json)이다. 이 문서는 원장을 사람이 읽고 바로 이어서 일할 수 있게 풀어 쓴 것이다.

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

### 그래프와 질의 도구 (2026-11-25부터의 작업)

- 사용자가 2026-11-25에 연구를 새 접근으로 다시 시작하기로 했다. 2026-11-16~20의 에이전트 평가는 근거로 쓰지 않는다([보고서](./research/GRAPH-MCP-AGENT-EVAL-2026-11-20.md) 머리의 안내). 새 접근은 사용자가 설명하기로 했고 아직 설명 전이다.
- 그 전에 그래프가 소스와 같은지부터 확인하고 고쳤다. go-stablenet `996e7901`로 만든 그래프와 질의 도구의 답을, 추출기와 코드를 공유하지 않는 별도 프로그램(go/parser, go/types, solc AST)으로 소스와 대조했다. 사람의 수용은 아니다.
  - Go 선언 73,164/73,164, 이름 있는 대상의 호출 83,667/83,667, `implements` 1,103/1,103, lock 지점 1,634/1,636, `accessed_under_lock` 1,310/1,310, `handles` 4,294/4,294. Solidity 선언 657개와 엣지는 근거 없는 것 0.
  - 질의 도구는 seed 4개 표본에서 호출자, 피호출자, 참조가 소스와 같다. 도구가 출력한 이름 71,249쌍은 다시 넣으면 그 노드 하나로 돌아간다.
  - 같은 소스로 두 번 빌드하면 manifest의 시각 말고 모든 표가 같다.
- 로컬 커밋(위가 최신, push하지 않음):
  - `4604cb5d` refactor(system): decide test code by the graph's is_test outside the graph tools
  - `76fa3d6a` feat(ckg): link goroutines, channel messages and channel locks from SSA (schema 1.42)
  - `81274130` fix(ckg): print CLI node names so each opens its node
  - `3f78fb48` fix(ckg): print every node name so it opens exactly that node
  - `cb74cc3d` fix(ckg): name a mutex variable and its mutex apart in every tool
  - `e42c6fe8` fix(ckg): resolve card names like every tool and point cut cards to an exact call
  - `dc2b47f3` fix(ckg): count literal fields under a lock and pin investigate output (schema 1.41)
  - `05858b50` fix(ckg): record accessed_under_lock only where the lock is held (schema 1.40)
  - `e893009f` feat(ckg): close source-fidelity gaps in the graph and query tools (schema 1.39)
  - `5747130a` docs(research): mark the 2026-11 agent evaluation report as superseded
  - `0deaa8ec` feat(ckg): make Go and Solidity graphs match their source (schema 1.38)
- 스키마는 커밋된 것이 1.42, 작업 트리가 1.43이다. 1.43은 함수 리터럴 안의 호출을 그 리터럴을 인자로 받는 함수에 잇는 `literal_passed_to` 엣지와 메시지 행의 정렬이고, 검증 중이며 커밋 전이다. 같은 작업 트리에 지역 변수가 든 lock channel을 seed로 받는 수정이 있다(검증됨, 커밋 전).
- 증분 빌드는 없앴다. 편집 13개 중 11개에서 전체 빌드와 다른 그래프가 나왔고 더 느렸다. 모든 빌드는 전체 빌드이고 `ckg watch`와 `ckg bench-index`는 실행을 거절한다.
- 동시성 사실은 SSA로 만든다. 그래프에 있는 것: goroutine이 실행하는 코드로 가는 연결, 받기 연산이 실행시키는 코드(`handles`), 송신자가 깨우는 받기 연산(`triggers`), 받은 값의 흐름(`flows_to`), lock으로 쓰는 channel 6개(획득, 해제, 그 아래 접근), 호출을 따라가는 lock 전파(옵션, `propagated` 표시), 언어 공통 계약 `internal/graph/concfacts`(사실마다 증명됨 또는 근사와 이유).
- 새 브랜치 `feat/ckg-execution-flow`에서 실행 순서로 펼쳐 답하는 질의를 만드는 중이다. 별도 worktree에 있고 커밋 전이다.
- go-stablenet의 실사용 그래프 링크는 옮기지 않았다. 새 그래프(스키마 1.41, 이력 포함)를 옆에 만들어 두었고 벡터 색인은 만들지 않았다. 링크는 사용자가 옮긴다.
- 검증 기록은 저장소 밖 평가 데이터 폴더의 `runs/20261126-ks-*`, `runs/20261127-ks-*` 아래 `checks/`에 있다. 커밋하지 않는다.

### 검증하지 못한 것과 밝혀 둔 한계

- 검증하지 못한 것: cgo가 필요한 패키지 13개(타입 오류로 읽힘), TypeScript와 Proto 추출기, `--lock-propagation`을 켠 그래프(자체 검사만 4,224/4,225), 실제 이력이 있는 그래프에서의 `change_history`와 `evidence_for_intent`, `rule_candidates`, `ckg api`, export 계열, `observe`, `quickstart`. 함수 값 호출의 빠진 대상은 판정할 수 없다(포인터 분석이 필요).
- 밝혀 둔 한계: `x[i] = v`는 x의 쓰기로만 기록한다. 빌드 밖에서 온 promoted 메서드는 노드가 없다. 재귀 타입의 자기 참조 36곳은 엣지가 없다. map 원소로 꺼낸 lock 2곳은 기록하지 않는다. `accessed_under_lock`은 필드만 다루고 패키지 수준 변수는 다루지 않는다. 방향이 반대인 lock channel 규칙은 원리상 신호용 channel을 lock으로 볼 수 있다(이 소스에는 없음). 받은 값의 흐름 1곳(`Seal` 인자)이 이어지지 않는다. 검색은 선언 위 주석과 import를 보지 않는다(프로덕션 175,375줄 중 33,361줄).
- 정책 파일 `projects/stablenet/policies/graph.yaml`의 항목 6개는 어떤 선언도 가리키지 않는다. 사용자 결정으로 그대로 두었다.

### 13묶음 (변화 없음)

- 13묶음 완료 0/13, 정식 수용 0/9, 독립 승인 0, FINAL 0
- 동결(freeze) 없음, 정식 실행 불가(`execution_allowed=false`)
- F-01~F-08 동결 결정 모두 대기. F-05는 역할만 정했고 검토자의 독립성 선언을 기다린다.
- 프로토콜의 남은 구조 조건 97개(`scripts/check-operational-acceptance.py`)
- 리팩토링 원장이 인용하는 문서 54개는 2026-10-13 문서 정리(`cd5c35d6`)로 트리에서 빠졌다. [색인](./spec-driven/REFACTORING-RETIRED-DOCUMENTS.json)이 읽을 커밋과 SHA-256을 묶고, `check-refactoring-progress.py`가 인용 경로마다 현재 파일이나 이 색인과 맞는지 검사한다.
- 우선순위: 지금은 그래프와 질의 도구의 정확성이 먼저다. 검색 평가(go-stablenet 개발, indexer-go 홀드아웃)와 코딩 에이전트 묶음(WI-V1, WI-V2, WI-4, WI-C)은 사용자가 새 연구 접근을 설명한 뒤에 순서를 다시 정한다. WI-V1 정식 수용 시범은 보류했다(취소가 아니다).

## 3. 지금 할 일 (이 순서를 따른다)

### 3.1 정답이 있는 검색 평가와 정확도 개선 (최우선)

지금 할 일의 순서와 측정 규칙은 [정확도 로드맵](./ACCURACY-ROADMAP.md)이 정본이다. 바로 다음 행동은 [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md) 4장에 있다.

0. **그래프와 질의 도구의 정확성 (진행 중, 사용자 결정 2026-11-13과 2026-11-25)**: 새 제안보다 만들어진 데이터의 검증과 수정을 먼저 한다. 현재 상태는 2장에 있다. 이어서 할 일은 이 순서다.
   1. 작업 트리의 두 수정(지역 lock channel seed, `literal_passed_to`와 메시지 행 정렬, 스키마 1.43)의 독립 검증을 마치고 커밋한다. 커밋은 사용자가 요청할 때만 하고 push하지 않는다.
   2. 브랜치 `feat/ckg-execution-flow`의 실행 순서 질의를 끝내고, 답이 소스와 같은지 독립 검증한다. 효과는 정답을 아는 사례(수정 커밋)로만 잰다. 판정 규칙은 측정 전에 적는다.
   3. 사용자가 정할 것을 받는다: race와 liveness 후보를 계산해 그래프에 넣는 판단 층을 만들지, 새 연구 접근이 무엇인지(3.3).
   4. 2장의 "검증하지 못한 것"은 새 연구 접근에서 쓰이는 것부터 검증한다.
   아래 1~7은 사용자가 새 연구 접근을 설명하기 전에는 진행하지 않는다. 2026-11-25의 지시("이전의 테스트는 모두 무시")가 이 항목들까지 포함하는지는 사용자에게 확인하지 않았다.
1. **grpc-go 평가 (끝남, 사람 확인 대기)**: [명세](./spec-driven/RETRIEVAL-EVAL-SPEC.md)는 사용자가 2026-10-23에 승인했다. 입력은 [grpc-go 입력](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/README.md)에 고정돼 있다. 1차 실행(스냅샷 24개 공유)과 진단 기록 세 개를 거쳐 첫 수정 묶음을 반영했다(#126, #128). v2는 PR마다 기준 커밋으로 색인하고 두 모델을 같은 조건에 두었다([결정](./decisions/20261028-retrieval-eval-v2/decision.md)). 그 결과로 `code_query`를 더했고, 시험용 56건에서 정답이 팩에 든 PR이 bge-m3 23 → 36, qwen3 18 → 40이 됐다. 정답과 질문 표본의 사람 확인은 아직이다([표본 확인](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/sample-check/PACKET.md)).
2. **평가 틀을 바꿈 (사용자 결정, 2026-11-03)**: [indexer-go 평가 결정](./decisions/20261103-indexer-go-eval/decision.md)으로 독립 평가 프로젝트를 grpc-go에서 indexer-go로 바꿨다. go-stablenet 11건은 개발 표본이다. 변경은 D6으로 판정한다. go-stablenet의 이득을 보고하고, indexer-go에서는 두 임베딩 모두 잃은 PR이 얻은 PR보다 많으면 안 되며, 같은 이름 선언이 많은 위험 부분집합에서는 한 건도 잃으면 안 된다. 검색 단계만 잰다.
3. **팩 구성 수정 (끝남)**: 이웃 예약 2칸(#130), knowledge 예약을 본문 상한 밖으로(#134), 같은 함수의 벡터·그래프 후보 합치기(#135), 잘린 seed를 `cut_candidates`로 남기기(#137). 지금 go-stablenet에서 정답 함수가 팩 본문에 든 PR은 bge-m3 8/11, qwen3 10/11이다. `code_query`에서 키워드 seed를 뽑는 실험은 PR을 잃어 기각했다.
4. **V1 심볼 요약 (진행 중)**: 만든 그대로는 채택하지 않았다(indexer-go 15건에서 bge-m3 1건 잃음). 2026-11-09에 검사 결함 세 가지를 고치고 요약 모델을 `gemma4:12b`로 바꿨다(사용자 결정). 다음은 indexer-go 남은 11건을 처음 보는 표본으로 판정하는 것이다. 이어 가는 법은 [심볼 요약 인수인계](./vector/SYMBOL-SUMMARY-HANDOFF.md)에 있다.
5. **근거 정밀도 (P1 끝남)**: 로드맵 4.5다. P0으로 팩 본문 가운데 정답이 아닌 몫을 재는 지표와 본문 수 설정을 더했다. P1(에이전트 수준 비교, 42번 실행)은 미리 정한 규칙으로 "차이 없음"이어서 P2와 P3는 시작하지 않는다.
6. **그다음**: 로드맵 5장 순서대로 J1(청크와 심볼을 잇는 표), Q1(personalized PageRank), G1~G3, V4를 측정한다. 의도 판정 개선과 문서 강등 정책은 정답 의도를 넣어도 팩이 거의 달라지지 않아 뒤로 미뤘다.
7. **"어떤 프로젝트든"의 판단**: go-stablenet과 indexer-go 결과까지 본 뒤에 한다. 지금 수치는 모두 AI 정답 기준의 DEV 근거다.

`internal/vector/eval/prregress`는 유료 LLM 에이전트를 부르므로 그대로 쓰지 않고, PR checkout과 정답 추출 방식만 참고한다.

### 3.2 그다음: 코딩 에이전트 묶음

검색 품질을 올린 뒤에 WI-V1, WI-V2, WI-4, WI-C를 다시 연다. 에이전트는 지식 시스템이 다 주지 못하는 부분만 맡는다.

WI-V1을 다시 열 때는 이미 확인한 정답 시험을 판정 기준으로 쓴다. 이 시험은 버그 재현(ORIGINAL 멈춤), 수정 확인(CAND-A·B 반환과 event loop 생존), 반례 차단(MUT-EARLY에서 event loop 멈춤)을 가른다([확인 기록](./spec-driven/operational-preparation/retrieval-priority-2026-10-23/DECISION.json)). 12계약은 이 시험을 중심으로 줄인다. gold-reviewer용 [검토 자료](./spec-driven/operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-GUIDE.md)는 보존하지만 지금은 진행하지 않는다.

### 3.3 사람이 정해야 하는 것

|항목|누가|상태|
|---|---|---|
|검색 평가 명세 확인|owner-acceptor|승인됨 (2026-10-23)|
|PR 정답 표본과 질문 누설 확인|gold-reviewer 또는 owner-acceptor|사용자가 AI에게 맡겨 AI 검토만 있음(2026-10-27). 사람 확인은 대기 ([자료](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/sample-check/PACKET.md))|
|스냅샷 커밋 판정|사용자|정함 (2026-10-27): `cks eval`이 파일이 그대로인 인용을 받아들인다|
|go-stablenet 질의 세트|사용자|정함 (2026-10-27): 한국어 질문 12개를 새로 만들어 고정. v2부터 pr72를 빼 11건. AI가 썼고 사람 확인 전|
|D1·D3 수정 채택(미리 정한 기준을 넘지 못함)|사용자|정함 (2026-10-27): 둘 다 채택. 사유는 [진단 기록](./spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/diagnosis-2026-10-27/README.md) 4.4절|
|이웃 순위 수정 채택(미리 정한 기준을 넘지 못함)|사용자|정함 (2026-10-27): D4·D5 채택, D6 제외. 원문 "A. fix-d45 채택 (Recommended)"|
|v2 평가 조건(두 모델 같은 조건 비교, PR마다 기준 커밋 색인, AI 라벨)|사용자|정함 (2026-10-28): 추천안으로 위임. 원문 "우선은 빠르게 진행해서 테스트 결과를 보는것이 중요햐". [결정](./decisions/20261028-retrieval-eval-v2/decision.md)|
|v2 정답 함수 라벨|gold-reviewer 또는 owner-acceptor|AI 라벨만 있음. 사람 확인 대기|
|독립 평가 프로젝트와 판정 규칙|사용자|정함 (2026-11-03): grpc-go 대신 indexer-go, D6 규칙. [결정](./decisions/20261103-indexer-go-eval/decision.md)|
|indexer-go 증상 질문과 정답 함수|사용자(indexer-go 저자)|정함 (2026-11-03): 검토 문서 전체를 확인해 고정. 원문 "전체 확인으로 기록"(선택 36건, 질문 26건, 정답 라벨 26건). 기록은 빌드 머신 작업 폴더 `indexer-go/freeze.txt`에 있고 저장소에는 없다|
|동명 본문 제한 변경(`79c938d1`)|사용자|정함 (2026-11-04): 기각. 원문 "추천 1로 진행". go-stablenet 위험 부분집합 pr-58을 bge-m3에서 잃음. 브랜치는 빌드 머신에만 두고 push하지 않는다|
|판정에 필요한 최소 확인 PR 수, go-stablenet 효과의 통과 기준|사용자|미정 (indexer-go 결정의 보류 항목)|
|V1 검사 수정과 요약 모델|사용자|정함 (2026-11-09): 생성 파일 건너뛰기, 대소문자 무시, 머리말 없는 두 줄 받기. 요약 모델은 `gemma4:12b`(thinking 끔). 원문 "벤치만 보면 gemma4:12b 사용해야겠어.", "A로 진행하고, V1 결함도 묶어서 처리해"|
|시뮬레이션 시험 머신과 Ollama 모델|사용자|정함 (2026-11-09): 이 머신(Apple M4, 24 GB)에서 진행. 모델은 `bge-m3`, `qwen3-embedding:4b`, `gemma4:12b`만 둔다|
|근거 정밀도 P1(에이전트 수준 비교)의 사례, 반복 수, 비용 상한|사용자|정함 (2026-11-10~13): 이 머신에서 `claude -p`로 7건 × 2조건 × 3회, 실행당 환산 5 USD. 결정 기록은 `~/.claude/decisions/knowledge-system/20261109-evidence-precision-p1/decision.md`|
|작업 순서: 데이터 검증을 새 제안보다 먼저|사용자|정함 (2026-11-13). 원문 "이미 생성된 데이터 부터 문제가 있으면 당연히 결과가 좋지 않자나 ??", 검증기부터 만드는 A안 "A로 진행해."|
|팩 칸 수와 토큰|사용자|미정. 2026-10-27 예상(12 → 16칸이면 정답 +4, +2, 토큰 약 +33%)은 이웃 예약이 4칸이던 때 값이다. #130·#134 뒤에 다시 재고 묻는다|
|N-12·N-14를 "사용자 일괄 채택(AI 검토 라벨)"로 따로 표시할지, N-10(전체 비용·p95)과 N-17(실제 운영 RTO/RPO)의 남은 범위를 별도 작업으로 뺄지|사용자|미정. 원장 상태는 바꾸지 않았다|
|WI-V1 독립성 선언과 검토 결정|gold-reviewer|보류 (3.2)|
|실제 LLM 답변 평가의 모델, 인증, 비용 상한|사용자|미정. 정하기 전에는 유료 모델을 호출하지 않는다|
|보안 전용 평가, 조직 인증, 보안 감사|사용자|보류|
|새 연구를 어디서 시작할지|사용자|정함 (2026-11-25): 사용자가 새 접근을 설명한다. 그 전에 정보 수집과 상황 이해를 먼저 한다. 설명은 아직 전|
|커밋하지 않은 related cap 코드|사용자|정함 (2026-11-26): 패치로 저장소 밖에 보관하고 작업 트리에서 되돌림|
|2026-11-16~20 평가 보고서|사용자|정함 (2026-11-26): 근거로 쓰지 않는다는 안내를 보고서 머리에 둠|
|그래프 결함을 고치는 방식|사용자|정함 (2026-11-25): 실패하는 fixture 시험을 먼저 쓰고, 규칙을 한 곳에 모은 뒤 고침|
|건너뛰던 구조(receiver 타입 참조, 익명 struct 필드, `:=` 지역 이름)|사용자|정함 (2026-11-25): 그래프에 넣음|
|Solidity 추출기를 수정 범위에 넣을지|사용자|정함 (2026-11-25): 넣음|
|패키지 수준 초기화식 안의 호출의 호출자|사용자|정함 (2026-11-25): 그 변수(Variable)|
|검증에서 나온 불일치를 어디까지 고칠지|사용자|정함 (2026-11-26): 원인이 드러난 것은 모두 고치고 같은 검증을 다시 함|
|검증된 수정본의 커밋 시점(여러 차례)|사용자|정함 (2026-11-26~27): 검증된 상태를 먼저 로컬 커밋하고 남은 것은 다음 커밋으로. push하지 않음|
|세션이 만든 임시 데이터와 평가 데이터 정리|사용자|정함 (2026-11-26): 다시 만들 수 있는 것만 지움. 다른 연구 보고서가 인용하는 run 폴더 35개는 남김|
|수정을 어디서 멈출지|사용자|정함 (2026-11-26): 남은 것도 고침. composite literal 초기화도 lock 아래 접근으로 셈|
|출력된 이름이 한 노드로 돌아가지 않는 문제|사용자|정함 (2026-11-26): 고침. 여러 선언이 쓰는 이름은 `이름@파일:줄`로 출력|
|다른 세션에 그래프 변경을 알릴지|사용자|정함 (2026-11-26): 알리지 않음. 다른 세션은 종료함|
|실사용 그래프 링크|사용자|정함 (2026-11-26): 새 그래프만 옆에 만들고 링크는 사용자가 옮김|
|남은 작업 네 가지(lock 전파 정리, 미검증 엣지 대조, 증분 빌드 시험, test 판단 규칙 통일)|사용자|정함 (2026-11-26): 모두 진행. 끝남|
|정책 파일의 이름 오류 6건|사용자|정함 (2026-11-27): 고치지 않고 목록만 전달|
|비동기 동시성 코드의 처리|사용자|정함 (2026-11-27): SSA로 처리. goroutine은 호출자이고, 같은 channel에 보내는 쪽이 그 코드를 실행시킨다는 점을 그래프에 넣음|
|메시지가 실행시키는 코드의 범위|사용자|정함 (2026-11-27): 범위 전체는 `handles`, 값의 흐름은 `flows_to`로 구분|
|증분 빌드|사용자|정함 (2026-11-27): 쓰지 않게 막고 빌드는 항상 전체 빌드|
|동시성 사실 계약|사용자|정함 (2026-11-27): 최소 계약(사실 종류와 증명됨/근사 표시)으로 시작해 Go와 함께 키움|
|lock처럼 쓰는 channel|사용자|정함 (2026-11-27): lock으로 표현|
|LLM의 읽는 방식에 맞춘 방법 중 먼저 시험할 것|사용자|정함 (2026-11-27): 실행 순서로 펼쳐서 주기. 별도 브랜치에서 진행|
|race와 liveness 후보를 계산해 넣는 판단 층|사용자|미정. 사실 층(위 동시성 사실)은 끝났고 후보 층은 시작하지 않음|
|새 연구 접근|사용자|미정. 사용자가 설명하기로 함|
|이력 스캔 결과(기존 탐지 약 135,000건)의 처리|사용자|미정. 정리 전에는 이 저장소에서 push하지 않음|

2026-11-25 이후의 결정 원문은 `~/.claude/decisions/knowledge-system/20261125-research-restart/`에 있다(저장소 밖).

## 4. 하지 않는 것

- 정답을 확인하지 않은 사례로 테스트하지 않는다. AI가 정답을 만들어야만 하는 사례는 평가 표본으로 고르지 않는다.
- 검색 평가 질문에 파일, 함수, 식별자 이름을 넣지 않는다. 색인을 PR이 머지된 뒤의 코드로 만들지 않는다.
- indexer-go의 PR별 결과를 보고 변경을 조정하지 않는다. 조정은 go-stablenet에서만 하고, 판정 규칙은 측정 전에 적는다.
- LLM이나 이력으로 만든 지식(요약, 예상 질문, 공변경)에 평가 PR 기준 커밋 뒤의 정보를 넣지 않는다.
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
- 2026-11-16~20의 에이전트 평가 결과를 설계나 주장의 근거로 쓰지 않는다. 사용자가 되살리기 전에는 그 실험(세 조건 비교)을 이어 가지 않는다.
- 사용자가 새 연구 접근을 설명하기 전에 새 실험을 제안하거나 시작하지 않는다.
- 이력 스캔 결과가 정리되기 전에는 push, PR, 머지를 하지 않는다. 커밋도 사용자가 요청할 때만 한다.
- 증분 빌드를 되살리지 않는다. 다시 만들려면 전체 빌드와 모든 표가 같은지를 기준으로 삼는다.
- go-stablenet 저장소, 실사용 그래프 링크, 정책 파일을 사용자 결정 없이 바꾸지 않는다.
- 근사로 만든 연결(INFERRED, 계약의 근사 표시)을 증명된 사실처럼 쓰지 않는다. race와 liveness 후보 층은 사용자 결정 전에 만들지 않는다.
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
