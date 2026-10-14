# 상위 WI 구현 명세와 진행 원장

2026-10-14 사용자가 WI-V1부터 WI-SP까지 구현 진행을 요청했다. [재감사](./UPPER-WORK-ITEMS-AUDIT.md)는 입력 근거로 고정하고, 이 문서를 구현 진행의 현재 기준으로 사용한다. WI-5는 이번 순서에 포함하지 않는다. 기존 승인·시험·소표본의 한계를 구현 성공으로 덮어쓰지 않는다.

## 목적과 상태 규칙

버그수정의 오라클·수정 선택·진단·비결정 재현을 먼저 연결하고, CKG 값 의존·규칙 분석을 기반으로 깊은 리뷰와 CKS 확장·정책/보안 질의를 제공한다. 그래프 계산은 CKG, 객관 신호 합성은 CKS, 판단과 절차는 coding-agent가 담당한다. 동일 계산을 에이전트 스크립트에 복제하지 않는다.

각 항목은 명세→구현→의미 있는 반례/회귀→실제 공개 경로 실행→근거 고정→작업표 갱신 순으로 진행한다. 구현·시험 완료와 독립 사람 수용을 분리한다. 사람의 수용을 자동 생성하거나 미실행을 PASS로 기록하지 않는다. 부분 지원은 capability·상한·불확실성으로 공개하며, 전체 언어/프로젝트/운영 품질로 일반화하지 않는다.

외부 변경도 이번 요청 범위에 포함한다. coding-agent는 감사한 임시 checkout의 작업 브랜치를 사용하며 커밋·패치/복구 자료를 이 저장소에 고정한다. ChainBench는 기존 개발자의 checkout을 유지하고 필요하면 격리 checkout을 만든다. 실제 시험은 이번 실행이 만든 데이터·노드만 대상으로 한다. 개인 `.claude/`와 개인 `logs/`는 입력에서 제외한다. 푸시·공개·태그·머지는 이 구현 요청으로 자동 수행하지 않는다.

## 진행표

| 순서 | ID | 담당 | 구현·검증 상태 | 전체 목표/독립 수용 |
|---|---|---|---|---|
| 1 | WI-V1 | coding-agent | 구현·controlled 실행/반례 검증 완료·증거 커밋 | 실제 host/체인·독립 수용 보류 |
| 2 | WI-V2 | coding-agent | 구현·최종 모델/Go 실행·반례 검증 통과, 증거 커밋 대기 | 미완료/미수용 |
| 3 | WI-3 | coding-agent+CKS | 대기 | 미완료/미수용 |
| 4 | WI-4 | coding-agent+ChainBench | 대기 | 미완료/미수용 |
| 5 | WI-1 | CKG | 대기 | 미완료/미수용 |
| 6 | WI-2 | CKG/CKS | 대기 | 미완료/미수용 |
| 7 | WI-C | coding-agent | 대기 | 미완료/미수용 |
| 8 | WI-6 | CKS | 대기 | 미완료/미수용 |
| 9 | WI-SP | CKS+CKG | 대기 | 미완료/미수용 |

## 항목별 계약과 필요한 검증

### WI-V1: 오라클 타당성과 완료 차단

- 증상 ID·수용 조건·별도 도출 근거·primary/independent 실행 파일·정상 사례·mutation 계획을 수정 전 커밋에 동결한다. 재현의 RED와 수정의 GREEN을 같은 파일·같은 조건으로 실측한다.
- 독립 오라클은 primary의 기대값/헬퍼를 재사용하지 않고 요구 조건에서 별도 계산한다. 파일이 다르다는 기계 검사는 의미적 독립성의 사람 검토를 대신하지 않는다.
- 두 오라클의 base RED/head GREEN, 정상 사례 base/head GREEN, 증상 재도입 및 과적합 반례를 실행한다. 증상 재도입은 두 오라클, 과적합은 최소 독립 오라클이 검출해야 한다.
- 명시된 assertion 실패만 RED다. 빌드 실패·timeout·잘못된 JSON·다른 증상·시그널 종료·누락은 실행 오류이며 통과/재현으로 인정하지 않는다.
- commit·오라클·계획·원자료 hash와 실행 command/case/status를 묶는다. 변경/누락/현재 commit 불일치·검사 FAIL/미실행은 완료 전이를 차단한다. 과적합·base GREEN은 WARN으로 통과시키지 않는다.
- CLI 실행과 plugin 평가/상태 전이/Write·Edit guard를 연결한다. host hook은 외부 shell·사람의 직접 수정까지 막는 보안 sandbox로 주장하지 않는다.
- 실제 소형 코드 결함에서 미수정/수정·mutation/정상 제어를 실행하고 false-done 차단을 확인한다. production 체인·LLM 독립 reviewer 수용은 별도 실측 상태로 남긴다.

### WI-V2: 수정 후보와 선택 패널

- 같은 증상/오라클 기준의 최소2개 후보. 후보별 root cause·diff/source pin·범위·회귀·오라클·비용/성능/복잡도·트레이드오프·근거.
- 사전 동결한 평가축과 hard 조건, 분리된 correctness/regression/maintainability 관점, 판단 출처·반대 의견·선택/기각/동률/전부 기각·판정 불가의 typed 출력.
- 적격 후보만 비교한다. 미실행 지표·자기 주장만으로 수치 우위나 ‘최선’을 선언하지 않는다. 설계→구현 전이에 선택 근거를 요구한다.
- 서로 다른 실행 결과·회귀·비용, 우열 없음·합의 없음·전부 부적격·기준 변경·증상 불일치 반례와 실제 후보 실행.

### WI-3: 제한된 의심 후보와 객관 신호

- 소비자 입력 schema와 실제 MCP schema를 맞추고 전 입력 type/required 계약을 검증한다. 호스트 namespace 정규화는 실제 연결 또는 미검증 상태로 공개한다.
- seed와 후보 상한·distance/relation·impact·recency·concurrency·retrieval rank/source/score·policy/security/dataflow 신호를 합성한다. 신호별 provenance, 미지원/미확보 null·이유, LLM 증상 판단을 분리한다.
- 반복 확장은 후보/깊이/예산 상한·누락/중단 이유를 유지한다. 없는 신호를 안전/0으로 해석하지 않는다.
- 소스/스냅샷 혼합·동명 symbol·누락 backend·상한·신호 출처·없는 tag 반례 및 MCP 호출→소비자 후보 산출물 실행.

### WI-4: 반복·주입·관찰

- 실행 전 seed/schedule·반복 분모·증상 failure 조건·전/후 기준·time budget·소유 노드·cleanup을 고정한다. 인프라 실패와 증상 실패를 구분한다.
- 명시 timing/fault 주입, 정상/재현/방어/관찰 분기. 미재현은 원인 미확정·관찰 대기이며 완료한 수정으로 승격하지 않는다.
- 모니터의 structured event/trigger·환경 pin·재개 조건, 통계 부족의 inconclusive와 기존 ChainBench 주입/monitor 연결.
- 실제 반복 결과·timeout/부분 완료·주입 실패·cleanup·재발 이벤트 반례. 기존 또는 외부 노드를 정지하지 않는다.

### WI-1: Go 값 의존 분석

- 첫 생산 경로는 Go 함수의 bounded on-demand CFG/def-use/control/data dependence다. CKG에서 한 번 계산하고 재사용한다. TS/Sol과 미지원 Go constructs를 명시한다.
- 선언/재할당·branch join·loop·return·field·call의 원문 위치/조건·확실/보수적/미지원 구분. alias/closure/반사/동적 호출을 증명된 연결로 만들지 않는다.
- source/revision/function·node/edge kind·최대 크기·truncation·계산 비용·capability 출력과 공개 질의.
- 실제 값 전달·kill/redefinition·분기/루프·제어 의존·동명/다중 파일·부분 지원·상한·비용 fixture 검증. AST statement 개수를 CFG 정확도로 대체하지 않는다.

### WI-2: 결정론 타입/규칙과 취약점 후보

- WI-1 위에 버전된 rule/source/sink·흐름 경로·조건·sanitizer/불확실성·위치/출처·수동/자동 생성 구분을 둔다. 분석 후보는 검증된 취약점 판정과 구분한다.
- opt-in 최소 Go rule pack과 지원 범위·정확도/비용/상한을 명시한다. 미지원/unknown은 안전으로 반환하지 않는다.
- 룰별 실제 양성·음성·sanitized·branch/alias/외부 호출·오탐/미탐·수동 annotation 공존·상한 테스트와 공개 실행.

### WI-C: 구조화 깊은 리뷰

- source pin·entry→sequence/module view·data/state·guard/exception·complexity/resource/I/O·priority/tradeoff를 typed artifact로 고정한다. 원문 근거와 추론·unknown을 분리한다.
- CKS 함수/값 의존/규칙 도구를 조합한다. 추론 diagram edge와 계산된 edge를 구분하고 누락 capability를 공개한다.
- 리뷰 모드 경로·보고서 validator·수용 조건 연결. 실제 fixture의 상태·가드 누락·자원/비용·우선순위 상충과 산출물 검증.

### WI-6: 값 의존 확장

- CKG dependency 결과를 CKS가 조합한다. calls·curated flow와 data/control dependence 관계를 구분한다.
- seed/candidate/hop/계산 예산·cycle/dedup·불확실/미지원·중단 이유·출처를 유지한다.
- 독립 답안이 있는 fixture의 recall/noise/cost를 비교하고 결과 분모를 기록한다. 작은 표본을 전체 최적성으로 승격하지 않는다.

### WI-SP: 정책·보안 태그 질의

- CKG 수동 annotation 간접 공개는 유지하고 CKS에서 typed policy/security 질의·관련 symbol·원문 위치·rule/manual origin·검토 provenance·unknown/not-found를 제공한다.
- 원문/검토자·버전·권한 경계를 명시하고 지원하지 않는 trust/auth 기능을 자동 주장하지 않는다. 실제 기존 HTTP/권한 정책과 모순되는 별도 우회 표면을 만들지 않는다.
- 등록 tools/list·입력/출력·소비자 계약을 갱신한다. 양성/음성·빈 태그·조건/권한 거부·출처·설명/수정 힌트 반환·LLM 판단 분리의 공개 MCP 실행.

## 체크와 증거

각 구현 항목의 세부 체크·커밋·검증 packet 링크는 해당 항목을 구현하며 갱신한다. 아직 실행하지 않은 세부 조건에는 체크를 하지 않는다. 기존 감사 packet은 바꾸지 않고 별도의 구현 packet으로 보존한다.

현재: 구현·controlled 검증1/9, WI-V2 계약 구체화 중. 전체 목표의 독립 수용은0/9다. 다음: WI-V2 구현. 남은 이번 구현 범위: WI-V2/3/4/1/2/C/6/SP. 전체 후속 범위와 WI-5 의존성은 [남은 작업](../REMAINING-WORK.md)에 유지한다.

## WI-V1 검증 기록

coding-agent 로컬 커밋 `5fd101e682b5d0286dbcd27fcd2b4a266bb7bc50`에 실행기·Go adapter·평가/상태 전이·Write/Edit guard와 CI 경로를 구현했다. [검증 packet](../../system/eval/upper-work-items-implementation-2026-10-14/wi-v1/)에는 소스 patch/commit 원자료, fixture Git bundle, 계획·원자료 hash·실제 CLI report가 있다. 임시 checkout을 잃어도 patch와 source hash로 복구할 수 있다.

- plugin63개(새 oracle 반례23·Go adapter7·기존33), 기존 bench15개, 기존 hooks ALL PASS.
- 실제 Go fixture의 build/race/vet/fmt PASS. dual oracle16회 실행, 기본/정상/증상 재도입/과적합 결과 PASS. 잘못된 report hash는 완료 hook에서 deny.
- 무효 base GREEN·다른 assertion·compile/panic/timeout·source 변경·raw report 변조·stale commit·부족한 다른 gate·필수 e2e SKIP를 차단했다.
- 검사는 actual subject의 committed snapshots에 한정한다. separate derivation/method는 확인한 fixture 구조이며 의미적 독립성·사람 수용·전 입력 정확도는 자동 증명하지 않는다.
- 실제 Claude host·production chain·독립 사람 수용은 미실행이다. 필수 e2e에 실행 가능한 adapter가 없으면 완료를 차단한다. 이 연결의 실제 실행은 WI-4에서도 추적한다.

증거 커밋 `e040fc288a5fb438d35c77ab5281eb7aefce9a68`의 packet을 Git object에서 재검증한 후 구현 완료 상태를 기록했다. [기계 판독 진행 원장](./UPPER-WORK-ITEMS-PROGRESS.json)은 evidence revision·manifest SHA·owner commit과 미수용 범위를 분리한다. 다음은 WI-V2다.

## WI-V2 세부 계약: 다음 구현

후보 생성은 Planner가 같은 증상·동결된 수용 조건에서 최소2개의 실제 수정안을 작성하는 단계다. 패널은 correctness/regression/maintainability 관점을 분리하고 근거 파일·판정·반대 의견을 기록한다. 런타임 검증기는 후보별 commit/oracle·실행된 metric의 근거를 검증하고 hard 부적격을 배제한다. 패널/지표의 자기 주장이나 단순 weighted sum을 최선의 증명으로 취급하지 않는다.

- 후보마다 같은 frozen plan과 source baseline을 유지하고 source commit·diff·V1 실행 보고서를 결속한다.
- 비교축/방향/단위/필수 여부·예산·패널 역할·합의 규칙을 후보 선택 전에 동결한다.
- 측정되지 않은 값은 unknown이다. 부적격·불일치·동률·패널 disagreement·모두 기각은 selection 보류/기각을 반환한다.
- 선택된 commit과 근거는 DESIGN→IMPLEMENTATION/EVALUATION 경로에 연결하고, 실제 구현/HEAD가 선택과 달라지면 재평가한다.
- 다음 시험은 실제 같은 증상을 고치는2개 후보와 literal overfit/회귀 후보의 실행 비교, 근거 변조·기준 drift·동률·전부 기각 반례다.

### WI-V2 실행 schema와 연결 계약

selection specification은 schema_version1, source_base_ref, oracle_plan(작업공간 상대 경로), oracle_sources(필요 시 작업공간의 frozen 파일), criteria(고정된 축/방향/단위/필수 여부), regression_driver, metric_drivers, judges(정확성·회귀·유지보수 역할), candidates(최소2개 실제 변경 제안·rationale·mutation 계획)를 가진다. `oracle_sources`는 테스트/오라클/수용 조건에 한정한다. source_base_ref의 생산 코드를 변경해 RED를 만드는 carrier는 허용하지 않는다.

실행기는 별도 Git index와 commit-tree로 RED carrier와 후보 커밋을 만들며 원래 branch/index/소스는 유지한다. V1은 명시한 후보 commit에서 trial receipt를 생성한다. trial receipt는 최종 현재 HEAD의 완료 receipt로 사용할 수 없다. 각 후보는 같은 수용 조건/정상 제어/오라클·미수정 생산 소스에 결속된다.

회귀/metric command는 frozen adapter에서 실행하며 assertion/실행 오류·unknown을 구분한다. 변경 줄 수는 Git diff 실측값이다. 선택은 hard 적격성→패널 승인→관측된 축의 Pareto 우위 순이다. 우열 없음/필수 값 미측정/패널 충돌/모두 기각은 보류·동률·기각이며 임의 첫 후보로 진행하지 않는다. 판단은 관측된 후보·축의 범위로 한정한다.

패널 command는 실제 source diff·V1/회귀/metric 원자료의 hash를 입력으로 받고 역할별 typed 의견과 이유·근거 인용을 반환한다. fixture 패널과 실제 모델/사람의 의견을 구분한다. model/human 자기 표시만으로 독립 수용을 생성하지 않는다. 모델 host 검증은 실제 command 실행 원자료를 남긴다.

DESIGN→IMPLEMENTATION에는 생성·평가·선택 보고서가 필요하고, 실제 적용은 선택된 tree와 일치해야 한다. 이전 V1의 첫 RED carrier 생성은 이 연결에서는 DESIGN의 trial 준비가 앞당겨 수행한다. Implementer는 그 carrier를 새로 만들거나 base를 merge-base로 바꾸지 않는다. 최종 HEAD에서 V1을 다시 실행하며 후보 선택·코드 변경 시 재평가한다.

## WI-V2 실행 검증 진행

실행기는 production checkout/index를 변경하지 않고 신규 테스트/오라클만 담은 RED carrier와 2개 이상 실제 후보 commit을 만든다. V1의 명시 후보 ref 시험은 trial scope이며 최종 HEAD 완료 receipt로 사용할 수 없다. frozen 회귀/metric adapters·role별 adapter hash와 원자료 hash를 검증하고, 현재 적용 tree가 선택 tree와 다르면 차단한다.

새 후보 반례19개를 포함한 plugin82개, 기존 bench15개와 hooks ALL PASS를 확인했다. 최종 모델 원자료와 소스를 커밋하기 전에는 구현 완료 체크하지 않는다.

실제 Go fixture에서 loop/copy/literal3개의 후보를 실행했다. loop/copy는 동일 오라클·회귀를 통과했고, literal은 FAIL이다. loop는 변경7줄·allocation0/call, copy는8줄·1/call이다. fixture 의견임을 명시한 실행에서 선택·실제 적용·final HEAD V1·양쪽 guard가 통과했다. 실제 모델의 정확성/회귀/유지보수3세션도 이 판정을 수행했으며 첫 실행의 원자료는 model-initial로 보존했다. 이후 실행 오류 후보의 부적격 처리 대신 전체 보류 정책을 보강했으므로 최종 소스 모델 실행을 다시 고정한다.

수치/실행 근거와 모델 의견은 구분한다. 같은 모델의 역할별 세션이며 독립 사람 수용·production 프로젝트의 최적성은 자동 생성하지 않는다. 테스트 범위와 실제 모델 adapter invocation은 packet에 구분하여 기록한다.

### WI-V2 최종 증거 고정

coding-agent 로컬 커밋 `068d3c3f2340a89317ca1cbb11f5bf1d30f620b1`. [WI-V2 packet](../../system/eval/upper-work-items-implementation-2026-10-14/wi-v2/)에 source patch/commit object·3개 후보 Git bundle·측정/원자료·모델 receipt·최종 HEAD report를 보존했다. 최종 원자료와 현재 source SHA가 일치함을 확인했다.

- plugin82개, 새 선택 반례19개, 기존 bench15개, hooks ALL PASS.
- 실제 모델3별도 세션(정확성/회귀/유지보수)이 loop를 선호했다. provider가 보고한 모델 label은 claude-opus-5-5이며 사용자 구성 기본값을 사용했다. 이를 독립 사람 수용으로 기록하지 않는다.
- 후보3개·오라클48회와 선택된 실제 HEAD의16회, 총64회 실행. literal 과적합은 oracle/regression FAIL로 기각. loop/copy는 둘 다 PASS, 관측된 변경7/8줄·할당0/1회에서 loop가 우위였다.
- 실제 선택 check·적용 tree check·final HEAD V1·양쪽 completion guard·build/race/vet/fmt PASS. 시험 중 원 checkout/HEAD/index는 유지됐고 적용은 owned fixture에서만 수행했다.
- 최종 선택은 명시한 후보/축/작업부하에 한정된다. 실제 production 코드의 전 후보 최적성·독립 사람 수용·설치된 전체 plugin host 흐름은 미판정이다.

커밋된 packet을 재검증한 뒤 구현 상태를 갱신한다. 다음은 WI-3다.
