# 운영·독립 수용 검증 계획

문서 최종 정리 기준일: 2026-10-15 Asia/Seoul. source 후보 관측은2026-10-14에 수행했다. **계획 초안**이며 검증 실행·새 정답·수치 기준 승인·독립 수용은 아직 수행하지 않았다. [구현 원장](./UPPER-WORK-ITEMS-PROGRESS.json)의9/9 implemented와 독립 수용 not_recorded는 유지한다. 준비/실행/판정 상태는 [별도 검증 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)으로 추적한다.

## 첫 검증 대상과 적용 범위

첫 대상은 기존 개인 로컬 운영 범위의 **Go pilot**로 제안한다. 고정된 go-stablenet 소스를 새 격리 작업공간/데이터셋으로 복제하고 knowledge-system, 실제 설치된 coding-agent host, 소유한 ChainBench 시험 노드를 연결한다. 실제 프로젝트 범위와 독립 controlled 사례의 결과를 분리한다. 팀/조직·외부 production 전체의 품질 수용으로 확대하지 않는다.

| 구성 | 관측한 source 기준 | 실행 때 확인할 대상 |
|---|---|---|
| knowledge-system | main ad52d9562ef33a0d0c8bc6cd510033ee39c862ac, PR #103 문서 통합 후 상태 | 소스/빌드 commit·binary SHA·buildinfo·schema·실제 CKS tools/list와 namespace |
| coding-agent | owned checkout fdb4d515d891cbacef032ac0c431c94dc20aa946, tracked checkout clean | 실제 게시/설치 버전·plugin bytes·hook dispatch·host version·33 CKS source schema와 실제 transport contract |
| ChainBench | owned checkout df464d4b97fde4ab308c5c422024dc80518e5956, tracked checkout clean | 실제 게시/설치·CLI/MCP bytes·소유 node/port/data directory·취소/cleanup |
| go-stablenet | 0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8, tracked 변화 없음 | 동일 commit의 새 clone/snapshot과 source hashes; 기존 개발자 checkout은 수정하지 않음 |
| 임베딩 | 기존에 선택한 BGE-M3/native1024를 후보로 유지 | 실제 server/provider/version·완전한 digest·차원·전처리·모델/데이터 신원; source 기록을 현재 runtime 실측으로 대체하지 않음 |
| LLM·검토자 | 새 provider/model/budget·독립 사람 검토자는 미지정 | host의 실제 요청/응답·모델 식별·토큰/시간/비용·검토자 역할/독립성 선언 |

위 source 조회와 source schema33개 확인은 **준비 관측**이다. 설치·서비스 가능성·외부 저장소 반영·모델 runtime 또는 사람 수용을 확인한 것은 아니다. 원문 source 경로와 후보 checkout은 JSON에 기록한다. 실행 디렉터리·node·서비스 포인터는 아직 만들거나 변경하지 않았다.

## 범위와 운영 연결의 완료 정의

1. 공개 CLI/MCP와 **실제 설치된 host/plugin**을 사용한다. helper CLI·fake backend·기존 raw packet 재생만으로 설치된 에이전트 workflow 통과를 주장하지 않는다.
2. bugfix 검증은 Analyzer/Planner/Implementer/Evaluator의 실제 dispatch와 평가 gate까지 확인한다. 외부 PR/Jira/메시지 게시·배포 전달은 이번 초안의 자동 실행 범위에서 제외한다. 실제 workflow의 게시 전 경계에서 멈추고 종료 상태를 기록한다. 게시/COMPLETION이 필수인 전체 workflow 수용은 해당 대상·권한을 따로 확정하기 전에는 partial이다. 모의 URL·수동 terminal state로 누락 단계를 채우지 않는다.
3. code_review는 설치된 review 모드의 typed report·그림·실제 hook·허용된 로컬 전달을 확인한다. model reasoning, 계산 근거, 부분 지원/unknown과 사람 판단을 분리한다.
4. 테스트 CKS/CKG 프로세스와 ChainBench 노드는 owned resource inventory에 등록한 것만 제어한다. 기존 운영 current/config·다른 노드·개발자 checkout·기본 ontology/type 옵션을 변경하지 않는다. 개인 .claude/와 개인 logs는 입력으로 읽지 않는다. 시험 자료는 새 owned 디렉터리에 둔다.
5. 실제 CKV 검색은 실제 선택 모델의 신원·차원·전처리가 맞는 데이터셋에서 검증한다. dummy/mock CKV의 graph tool 성공은 전체 retrieval 운영 연결 성공이 아니다. 기존13/42질문 및 과거 DEV/FINAL/fixture는 회귀용이며 새 독립 정답으로 재사용하지 않는다.

## 서로 다른 세 가지 판정

| 판정 | 통과에 필요한 내용 | 아직 부족할 때 |
|---|---|---|
| 운영 연결 | 실제 binary/host/plugin/backend/model/source pin·C1 계약·필수 공개 단계·raw receipts·거부/취소/정상 종료 | not_run/failed/partial |
| 독립 품질 | 새 독립 입력·정답·표본 단위·사전 protocol/threshold·paired baseline·성능/내용 채점 | not_run/inconclusive/failed/scoped_validated |
| 사람의 수용 | 지정된 독립 사람이 criteria별 근거·한계·실패·적용 범위를 검토하고 명시 승인 | not_recorded/awaiting_review/rejected/scope_accepted |

scope_accepted는 승인한 pilot 대상·source/model·사용 범위에 한정한다. 운영 연결 pass나 AI 패널 합의가 사람 수용을 자동 생성하지 않는다. 요구되지 않은 일반화나 production 전체 성공 판정은 이 원장에 쓰지 않는다.

## 실행 전 고정해야 할 입력

- 대상 repo/commit·dirty 여부·source bytes·binary SHA/buildinfo·dependency 및 실제 설치 plugin/host/namespace/tool schemas.
- dataset/model/provider/digest/차원/전처리·manifest/snapshot SHA·온톨로지/팩 설정. 기본 ontology off·type_neighbors off·knowledge 명시 opt-in을 유지하고 arm별 변경을 기록한다.
- 새 case/fact-family ID·public input·별도 gold·출처 span/commit·작성자/검토자·허용 후보/관계 집합·명시 negative/unknown/support scope. gold를 실제 에이전트 요청에 넣지 않는다.
- source/case family를 기준으로 DEV와 독립 FINAL을 분리한다. 동일 사실의 번역·문장 변형·함수 복사·반복 호출을 독립 표본으로 세지 않는다. FINAL을 본 뒤 수정하면 그 FINAL은 이후 회귀로 전환하고 새 독립 입력을 고정한다.
- 계획 반복·seed/schedule/순서·warmup/cold/warm·최대 호출/시간/동시성·node 소유권/포트·취소/cleanup·측정/운영자·LLM 비용 상한과 중단 조건.
- 정답 채점·macro/분모·missing/error/abstention/unsupported 처리·비교 baseline·불확실성 산출 방법·최소 표본·기준·reviewer의 적용 범위 승인.

입력 manifest와 protocol의 review receipt/hash가 없으면 실행 수용 단계는 시작하지 않는다. 운영 상태 조회·inventory 정리는 준비이며 이를 검증 통과 수에 넣지 않는다. 현재 숫자는 **제안**으로 기록하고 승인·동결된 기존 기준을 교체하지 않는다.

## 공통 필수 기준과 실패 처리

| 기준 | 제안한 완료 조건 |
|---|---|
| 출처/신원 | 모든 필수 응답의 commit/source/snapshot·근거 위치가 사전 입력과 일치. stale·동명 다른 snapshot·필수 pin 누락은 승인 결과로 반환하지 않음 |
| 도구/소비자 계약 | 실제 advertised CKS tool set와 등록 input semantics가 승인 source의33개 set에 일치, namespace mapping 및 설치 host의 실제 호출/응답 확인 |
| 실패/거부 | 각 적용되는 잘못된 입력·무효 oracle·source 변조·자료 변조·권한/범위 초과 class의 사전 negative case 최소1개, 기대 거부100%. declared negative의 원인·응답/차단 상태를 정확히 확인. 선언하지 않은 전송/빌드·인프라 오류는 negative 성공으로 세지 않음 |
| 자원/종료 | 사전 호출·시간·node·response 예산 준수. 새 owned 서버의 EOF/idle SIGTERM/active SIGTERM 각3회 이상, graceful exit0·lease/owned temp cleanup·강제 종료0. 종료5초 상한은 제안이며 cooperative 분석/행 예산의 hard memory·preemptive deadline 보장으로 확대하지 않음 |
| 필수 단계 | 필수 case·hook·role·oracle·모델·backend 미실행/SKIP는 해당 운영 gate 미충족. mock이나 fixture replay로 실제 단계 누락을 채우지 않음 |
| 오류/표본 | 모든 예정 실행과 결측/취소/인프라·증상 실패를 기록. 완료한 성공만으로 분모를 재작성하지 않음. 최소 표본 미달/불확실성 계획 미고정은 quality inconclusive |
| 근거/판정 | 명령·원요청·원응답·elapsed/exit·source/input/artifact hashes·부정 사례·판정표 및 criteria별 사람 receipt. shell/SDK 응답만으로 내용 수용을 자동 생성하지 않음 |

회귀 비교의 latency/cost는 같은 source/data/model/hardware·예산에서 paired 순서를 고정한다. 최초안은 warmup2·검색5·warm20·cold3회 측정과 순서 교대를 제안한다. 여기서 cold는 새 CKS 프로세스 시작을 뜻한다. 모델 로딩/cache resident 상태는 별도 기록하며 이를 측정하지 않으면 model cold를 주장하지 않는다. 반복은 지연 측정 횟수이며 독립 사실 수가 아니다. 비용은 요청별 토큰·embed/조회 호출·elapsed·활동 시간·측정된 자원으로 분리하고 미측정은 null이다. confidence 방법과 충분한 표본을 승인하기 전에는 평균/point threshold만으로 독립 품질 또는 성능 성공을 선언하지 않는다.

## 항목별 대상과 완료 기준

아래 최소 표본과 수치 기준은 reviewer 검토 후 실행 전에 채택·동결할 초안이다. 이전 통제 fixture의 관측치를 새 합격선이나 새 독립 표본으로 사용하지 않았다.

### WI-V1 — coding-agent

대상: 선정한 실제 결함 티켓과 설치된 host의 Analyzer/Planner/Implementer/Evaluator 경로.

입력/최소 표본 제안: 새 결함2건 이상. 독립 요구에서 도출한 primary/independent oracle, 정상 제어와 증상 재도입·과적합 mutation을 사전 고정.

완료 기준 제안: 모든 선언 oracle의 base RED/head GREEN·정상 제어 GREEN. 무효 oracle·다른 assertion·빌드/timeout·raw 변조·stale source·필수 단계 누락은100% 차단. 독립 검토자가 oracle 의미·원인 수정·범위를 항목별 승인.

판정 경계: 기존 fixture 통과나 CLI 단독 실행을 실제 host 완료로 대체하지 않는다.2건은 이 pilot 연결의 최소 사례이며 전체 false-done 발생률을 입증하지 않는다.

### WI-V2 — coding-agent

대상: 동일 증상에 대한 실제 후보 생성·패널·계획 선택.

입력/최소 표본 제안: WI-V1과 같은 신규 티켓2건 이상, 티켓당 후보2개 이상. 평가축·hard gate·사전 acceptable candidate set을 독립 검토자가 고정.

완료 기준 제안: 선택된 후보는 모든 hard gate 통과 및 사전 acceptable set에 포함. 전부 부적격이면 선택 없음. 동률·근거 부족을 명시. source/모델·관점·비용·반대 의견과 사람의 선택 수용을 기록.

판정 경계: 모델 패널 합의는 사람의 독립 수용이 아니다. 실제 측정되지 않은 성능·유지보수성 또는 전역 최적성은 unknown.

### WI-3 — coding-agent+CKS

대상: 실제 텍스트 증상→MCP→설치된 bugfix 분석 역할→typed 후보 산출물.

입력/최소 표본 제안: 독립 원인 위치가 있는 새 사실군20개 이상, 증상 유형4개 이상. top10·baseline·질문·정답/위치·source pin을 고정.

완료 기준 제안: case별 recall@10의 macro 평균>=0.80 제안. 같은 canonical 위치/commit으로 채점. provenance/unknown·반복 확장 상한·호스트 alias·실제 role 연결은 필수 조건. 독립 검토자의 root-cause 근거 확인.

판정 경계: 후보가 원인을 확정하지 않는다. 없는 신호를0/안전으로 채우지 않는다. 미지원/기권은 원래 분모와 coverage에 별도 표시.

### WI-4 — coding-agent+ChainBench

대상: 소유한 격리 노드의 baseline/fix 반복·fault/timing·모드9 관찰.

입력/최소 표본 제안: seed/schedule family5개 이상, 계획상 arm당100회 이상 및 유효 반복 분모. 주입 종류·증상 실패 조건·취소/cleanup·관찰 창10분을 사전 고정.

완료 기준 제안: baseline 증상10회 이상일 때 fix 실패율<=baseline의0.50 제안. family별 실패율·불확실성 구간과 인프라 오류를 분리. 취소/owned-node cleanup·변조/재발/미재발 분기 검증. 관찰 재발이 없으면 inconclusive이며 수정 성공으로 종료하지 않는다.

판정 경계: 반복100회를100개 독립 결함으로 세지 않는다. logging의 timing neutrality는 별도 효과·불확실성 평가가 없으면 unknown. 기존/외부 노드는 정지하지 않는다.

### WI-1 — CKG

대상: 실제 Go 소스의 retained CFG·값/제어 의존.

입력/최소 표본 제안: 새 Go 함수30개 이상, 선언/재정의·분기·루프·shadow·return 군별4개 이상. 수작업/별도 방법의 typed edge·원문 span 정답을 고정.

완료 기준 제안: 지원 범위에서 data/control edge 각각 precision>=0.95, recall>=0.90 제안. 모든 반환 위치·source/snapshot pin 일치. stale·잘못된 입력·상한·unknown 처리는 필수 조건. 함수별 성능·coverage를 기록.

판정 경계: gold는 structural/lexical may-reach 기준이며 실제 실행 가능성을 요구하지 않는다. alias/메모리·미지원 언어/construct는 별도 coverage로 남기고 범위를 사후 축소하지 않는다.

### WI-2 — CKG/CKS

대상: 현재 opt-in Go rule pack의 독립 source→sink 후보.

입력/최소 표본 제안: 지원 rule별 새 양성10개·음성10개 이상, sanitizer/branch/shadow/외부 target 사례 포함. 후보 탐지 정답과 실제 취약점 판단을 분리.

완료 기준 제안: 지원 후보 탐지 precision>=0.80, recall>=0.80 제안, rule별·전체 혼동행렬 모두 공개. source 경로/조건·rule/version·manual/automatic 구분·unknown·상한·SDK/소비자 정합 필수. 독립 검토자의 판정 기록.

판정 경계: 프로젝트에 양성 수가 부족하면 품질 inconclusive. 별도 독립 controlled 사례는 분리된 분모로 보고하며 프로젝트 효과를 채우는 데 사용하지 않는다. 취약점 확인은 별도 수용.

### WI-C — coding-agent

대상: 설치된 code_review 모드의 보고서·그림·상태/전달 gate.

입력/최소 표본 제안: 새 함수/모듈 리뷰10건 이상, module5개 이상. state/guard·자원/비용·우선순위의 참조 rubric과 필수 결함/제약을 독립 고정.

완료 기준 제안: 모든 문서/그림·source hash·typed artifact/state 연결 일치. 사전 필수 guard/state/자원 항목의 누락0. 독립 검토자가10건을 기준별 수용. 변조·placeholder-only·미핀 structural 자료의 확정 판단은 차단.

판정 경계: CFG 그림은 runtime sequence가 아니다. partial 지원·assumption·unknown과 추론 edge를 계산된 사실로 승격하지 않는다.

### WI-6 — CKS

대상: 실제 프로젝트에서 같은 seed의 지역 baseline과 bounded 함수 문맥 확장.

입력/최소 표본 제안: 새 독립 intent/seed 사실군20개 이상, 독립 관련 함수 gold와 intent 유형4개 이상. baseline=context.value_dependencies, 확장=context.value_dependency_expansion. 동일 snapshot·고정 상한으로 paired 비교.

완료 기준 제안: macro recall 증가>=0.10, macro noise fraction 증가<=0.10, warm p95 latency ratio<=2.0 제안. 반환 함수/근거·분모·cycle/dedup·호출/값/curated 구분·부분/중단·source pin 필수. 독립 관련성 검토.

판정 경계: 증가량은 percentage-point 단위다. noise fraction=비관련 반환/전체 반환. 빈 반환은 별도 실패/기권으로 기록. 비용 상승을 숨기거나 자동 argument transfer/dataset join을 주장하지 않는다.

### WI-SP — CKS+CKG

대상: 실제 소비자의 policy_security 질의와 운영 HTTP/CIDR 경계.

입력/최소 표본 제안: 새 symbol12개 이상: 정책4·보안4·빈 태그4. manual/automatic·legacy/typed·입력/stale/권한 거부 제어를 별도 고정.

완료 기준 제안: 사전 annotation 정답의 관련 symbol·설명/힌트/severity가100% 일치. 저장되지 않은 YAML row/hash/version/reviewer/approval은 unknown. 빈 태그와 not-found 구분. 직접 peer ACL·위조 헤더·private 경계는 기대 거부100%. 독립 검토자의 범위 수용.

판정 경계: manual 태그는 검증된 취약점/정책 적합 판정이 아니다. graph metadata SHA와 Go commit을 YAML/검토자 증명으로 사용하지 않는다. reviewer 인증은 현재 미지원.

## 통계·내용 채점과 부족분 판정

- WI-3의 case recall은 독립 gold 위치 집합 중 top10에 반환된 위치의 비율이고, macro는 독립 case/fact-family 평균이다. canonical identity·commit·허용 span 매칭 규칙을 사전 고정한다.
- WI-1의 gold는 statement span/edge kind/typed 근거를 기준으로 독립 도출한다. 함수별 data/control 혼동행렬·supported/unsupported coverage와 원문 검증을 함께 기록한다. 구현이 반환한 graph를 gold 생성기로 재사용하지 않는다.
- WI-2는 candidate 탐지와 실제 vulnerability를 분리하고 rule별 TP/TN/FP/FN·기권·미지원·실패 분모를 모두 보존한다. 후자를 검증하지 않았으면 candidate 품질만 판정한다.
- WI-6은 동일 seed의 baseline/확장을 한 쌍으로 기록한다. recall 증가·noise 증가·p95 ratio 및 결과/계산 비용을 모두 보고한다. 기준 초과를 숨기려고 query 상한·분모·실패 처리 규칙을 사후 변경하지 않는다.
- WI-4는 증상/인프라 오류를 분리하고 schedule family별 결과 및 추정 구간을 보존한다. 관측 효과·드문 재발·표본 부족은 inconclusive이다. 관찰 창 종료나 취소는 수정 성공의 증거가 아니다.
- 불확실성의 기본 방법 초안은 독립 fact-family/함수/seed-family를 단위로 하는 paired cluster bootstrap이다. 동일 단위의 baseline/candidate 관측을 함께 재표집하며 재표집10000회·고정 seed20261015·percentile2.5/97.5를 제안한다. 반복 호출·한 함수의 여러 edge를 independent n으로 세지 않는다. 이 방법·단위 정의·표본 충분성은 실행 전 reviewer가 승인해야 한다.
- 원자료에서 먼저 단위별 분모/집계를 고정한다. 어떤 재표집의 필수 분모가0이면 undefined를 기록하며 undefined 비율>5% 또는 최소 독립 단위 미달은 inconclusive로 제안한다. 희귀 오류의0건 관측·좁은 한 프로젝트·소표본의 bootstrap 구간은 일반 production 실패율0의 증거가 아니다.
- point 합격선에 도달해도 구간이 품질의 사전 합격선 또는 paired 개선량0을 가로지르면 품질 성공 대신 inconclusive로 제안한다. 충분성/검정력 판단에 필요한 baseline·효과 크기·추출 계획은 gold 작성 단계에서 별도로 고정한다. 필요한 추가 표본은 새 protocol에 기록하고, FINAL 결과를 본 뒤 합격선·분모를 완화하지 않는다. latency ratio와 noise margin도 같은 사전 비교 규칙을 따른다.

참고: paired 관측을 같은 행으로 재표집하는 원리와 percentile 방법은 [NIST Dataplot 설명](https://www.itl.nist.gov/div898/software/dataplot/refman1/auxillar/bootplot.htm)을 참고했다. 작은 원 표본의 한계를 bootstrap으로 해결하지 못한다는 설명은 [UC Berkeley 통계 강의](https://stat135.berkeley.edu/fall-2026/Modules/12-Bootstrap.html)를 따른다. 본 계획의 family 단위·표본 수·10000회·합격선은 이 자료가 승인한 수치가 아니라 검토할 자체 제안이다.

## 실행 전 검토할 제안과 미정 항목

| 결정 | 권고 초안 | 아직 필요한 확정 |
|---|---|---|
| 대상 | 고정 go-stablenet의 새 격리 복제본·개인 로컬 Go pilot | 실행 위치/운영자·설치된 host/plugin과 대상 endpoint 확인 |
| 범위 | 실제 bugfix 역할의 게시 전 평가 gate, local code_review 전달 | 외부 게시가 필수인 전체 workflow는 별도 승인/검증; terminal 범위 receipt |
| 기준 | 위 항목별 최소 독립 사례·정확도/회수율·비용 및 bootstrap 초안 | reviewer의 수치·방법·충분성/적용 범위 채택 또는 수정 후 freeze |
| 자원 | 전체 측정 세션 동시 실행1, owned node 최대2, index90분/host case30분/반복 arm30분/관찰10분 상한 제안 | 실제 머신 용량·호스트/시나리오 특성에 맞게 실행 전 한도 채택 |
| 모델 | 기존 선택 BGE-M3/native1024의 신원 재확인 | LLM provider/model/version·세션/토큰·비용 상한은 미지정, 확정 전 호출하지 않음 |
| 독립 역할 | 구현/수정·gold 도출·측정·독립 내용 검토·운영 수용의 역할 구분 | 실제 reviewer·gold 작성자·독립성/이해충돌 및 criteria별 receipt |

숫자를 정리한 것은 실행 승인이나 통계 충분성 확인이 아니다. 표본이 부족하거나 대상에서 rule 양성이 확보되지 않으면 해당 quality는 inconclusive이며 별도 controlled 입력의 성공으로 채우지 않는다. 계획은 위 미정 항목을 검토하기 위한 구체 초안이다.

## 검토자와 수용 receipt

구현/수정 작성, gold 도출, 실행/측정, 독립 내용 검토, 운영 범위 수용의 담당을 기록한다. 독립 검토자는 해당 구현이나 그 실행 결과에서 정답을 복사하지 않고 요구·원문·별도 방법으로 확인한다. 같은 사람이 여러 역할을 맡으면 독립성 제한을 명시하며 누락된 독립 검토를 완료 처리하지 않는다.

AI 모델은 제안/사전 검토자로 별도 origin에 기록한다. 과거 모델 이름·사용자의 일괄 채택을 새 사람 검토로 변환하지 않는다. receipt에는 reviewer kind/name·역할/독립성·대상 criteria·source/input/result/artifact hashes·승인/기각/보류·한계·승인 시점을 담는다. 이름 기록은 인증/서명 trust 기능이 아니다. 현재 reviewer·새 receipt는 모두 미지정이다.

## 순차 작업표와 실제 완료 체크

| 순서 | 단계 | 완료에 필요한 내용 | 현재 상태 |
|---|---|---|---|
| 0 | 범위·기준 검토/동결 | pilot 대상·terminal/delivery 범위·담당·새 gold/표본·수치/confidence/비용 기준의 receipt/hash | not_started |
| 1 | runtime inventory·설치/연결 | fresh binary/model/dataset/plugin/host/tool pin·현재/후보/rollback 경계·owned node 확인 | not_started, source 후보만 관측 |
| 2 | 실제 host pilot | 새 결함2건·code_review10건의 허용된 실제 역할/도구/hook/산출물과 필수 반례 | not_started |
| 3 | 독립 평가 | WI-1/2/3/4/6/SP 새 입력·정답·baseline·분모·성능/내용 채점·실패/부족분 | not_started |
| 4 | 사람의 기준별 수용 |9개 각각 운영/quality 결과·제약·independent receipt·승인 범위 | not_started |

운영 검증의 첫 파일 쓰기/노드 실행/모델 호출은 단계0에서 정한 대상·한도 안에서 수행한다. 이번 요청은 계획 작성이며 단계0 승인이나 실제 실행 완료가 아니다. 다음 단계에서 결정할 항목은 대상/설치 방식, 게시 전 terminal 범위, gold/reviewer 역할, 새 입력·통계/수치 gate, 모델·비용/시간·node 한도다. 정리 작업은 이 결정을 대신하지 않는다.

## 증거 보존과 상태 갱신

실행별 owned evidence 디렉터리에 scope/input/protocol/freeze receipt, source/host/model/dataset/tool inventory, gold/case manifest, raw commands/requests/responses/errors/exits, latency/cost/cleanup, verdict table 및 reviewer receipts를 보존한다. 파일명은 실행 protocol에서 고정하고 모든 참조의 SHA와 Git 또는 별도 보관 revision을 검증한다.

운영 상태만 바꾸거나 gate 숫자만 적은 보고서는 충분하지 않다. 명시한 단계가 실제 실행되고 근거가 검증된 뒤에만 별도 검증 원장을 갱신한다. human receipt 없는 independent_acceptance는 not_recorded/awaiting_review로 남긴다. 기존 구현 원장·과거 승인 입력/정답/결과·ontology 기본값·운영 current/config은 이 계획의 작성으로 변경하지 않는다.

## 다음과 전체 후속 범위

다음은 단계0의 대상·새 입력·검토자·완료 기준을 검토·동결하는 것이다. 이 계획의 미실행 단계는0–4 전부이며 운영/독립 수용 완료는0/9다. WI-5 및 일반 후속13개·태그/공개 배포 결정은 [전체 작업표](../REMAINING-WORK.md)의 별도 범위로 유지한다. 계획을 독립 품질 성공이나 조직/외부 production 전체 수용으로 확대하지 않는다.
