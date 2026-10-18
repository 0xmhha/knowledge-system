# 운영 검증 실행 protocol과 동결 준비

## 2026-10-18 독립 입력·완전 정답·온톨로지 동결 준비

D04는 사용자 결정대로 **미정**이다. [단계43 보고서](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/REPORT.txt)의 FP01~04는 준비 작업의 완료이며 정식 실행·gold 승인·F01~08 동결의 완료가 아니다. [현재 준비표](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/freeze-readiness.json)는8개 항목의 준비 자료와 남은 결정을 분리한다. 이전 시점의 source/host/도구 수는 아래 당시 기록으로 보존하며, 최신 실행 근거는 현재 JSON의 source-bound packet 참조를 따른다.

현재 준비 참조는 새 `independent_freeze_authoring_not_formal_execution` 종류다. 과거 host/metadata 준비는 `historical_readiness_preparation`에 원래 manifest/hash와9de07a82 소스 버전을 명시해 보존했다. 과거 소스 검사를 현재 코드에 적용해 생긴 불일치를 현재 통과로 바꾸지 않는다. 과거 회귀는 원래 버전에서, 현재 준비·운영 수용 회귀는 현재 코드에서 검증한다. 새 준비 참조는 과거 host 실행을 새 host 실행으로 갱신했다는 뜻이 아니다.

| 준비 결과 | 의미와 남은 조건 |
|---|---|
| 노출 후보206행 | DEV 작성·회귀 자료; 정식 DEV/FINAL 배정0, 독립 승인 단위0 |
| 완전 정답 양식10개 | WI9개+온톨로지; 원문·완전 universe·positive/negative/unknown·매칭·검토 receipt를 실제로 채워야 함 |
| 충분성 원장 | WI3 제안 가족15개/20입력, WI6 운영19개+stress1개, V1/V2 공유 티켓1개; 반복·번역·edge·셀은 가족 수를 늘리지 않음 |
| FINAL 절차 | 새 미노출 가족·독립 의미 검토·별도 gold 접근 통제/열람 기록·manifest/hash 사전 동결; 이미 Git에 노출한 자료는 봉인 FINAL로 재명명하지 않음 |
| 온톨로지 설계 | D06 방법 범위 승인 유지; DEV4모드 후 primary1모드 선택, FINAL20가족/4의도·의미 gold·검토 연결·동일 pin/cap·혼입/인용/지연 gate |

`scripts/check-independent-freeze.py`는 준비 자료 결속을 검사한다. `--require-ready`는 현재 준비 packet의 실행 승격을 거부한다. 기계 검사는 정답의 의미적 진실·독립성·표본 대표성을 대신하지 않는다. 빈 정답 집합에는 검토된 이유가 필요하며 null을 음성 정답으로 세지 않는다.20회 warm 지연의 p95는 기술적 관찰이며 tail 안정성을 증명하지 않는다. FINAL 최소 배정과 정밀도 계획은9개 항목에서 여전히 검토 전이다. 기존 요구·점수·bootstrap 임계치는 변경하지 않았다. 새 LLM 평가를 선택하기 전에는 모델·인증·비용 상한을 요구하지 않으며, 실제 LLM 답변 효과만 별도 승인 실행 범위로 남긴다.

다음은 역할·독립성 지정, 새 입력/완전 gold의 의미 작성·검토, 매칭·표본·실제 recipe의 동결이다. 정식 수용0/9·freeze=null·execution_allowed=false 및 기존 제한 온톨로지 연구완료/inconclusive·기본 off를 유지한다. [전체 남은 작업](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/WORKLIST.json)을 따른다.

## 2026-10-16 현재 준비 결과와 검토할 결정

로컬 로드맵28/28과 명시적 소스 대조 정책 생성기를 완료한 뒤, 운영·독립 수용의 준비 자료를 현재 코드와 대조했다. 새 준비 검증은 정식 운영/품질/사람 수용의0/9를 바꾸지 않는다. 기존 source inventory와 과거 packet은 당시 관측으로 보존한다.

| 이번 준비 항목 | 확인한 내용 |
|---|---|
| 현재 source·빌드 | knowledge-system3a530a25를 깨끗한 격리 checkout에서 재빌드; Go1.26.9·vcs.modified=false |
| 소비자 | 채택 coding-agent b7e7868의367개·ChainBench bdf5ad27의2045개 tracked 파일과 commit 대조 |
| 도구 계약 | 실제 CKS33개 입력 스키마가 소비자33개와 모두 일치; ChainBench81개; metadata2세션 EOF0 |
| 현재 host | CLI2.1.301의 valid/tools/incomplete/tampered/cancel5개를 로컬 시험 응답기로 새 실행·재판정 |
| 모델 metadata | BGE-M3 digest790764642607…16bab·Ollama0.40.4; 실제 임베딩/LLM 추론0·개인 인증 미조회 |
| 평가 초안 |9개별 기준선·독립 표본 단위·canonical 매칭·분모/기권 제안; 승인 gold0·DEV/FINAL0 |

host의 API 응답은 로컬 스크립트이고 실제 역할의 추론/판단은 수행하지 않았다. 새 CLI의5개 연결 시험을 과거2.1.300 결과로 대체하지 않았다. 취소 host143은 요청한 종료로 보존하며 MCP5개는0이다. 알려진 게시/정책 완화 거부와 Stop 단일 재진입을 확인했지만 실제 모델·전체 역할·HTTP 소비자·실제 노드/cleanup·다른 플랫폼은 미검증이다. CKS metadata는 graph+mock CKV 설정이므로 실제 BGE 검색 연결의 통과가 아니다. 모델 차원은 이번 metadata 점검에서 재측정하지 않았다.

검토 자료는 [현재 증거](../../system/eval/operational-preparation-2026-10-16/readiness-refresh/manifest.json), [8개 결정표](../../system/eval/operational-preparation-2026-10-16/readiness-refresh/freeze-review-checklist.json), [9개 평가·채점 제안](../../system/eval/operational-preparation-2026-10-16/readiness-refresh/scoring-review-proposals.json)이다. 제안은 AI 초안으로 표시하며 기존9개 input_groups의 정답/사례·담당·모델/비용 필드를 자동 채우지 않는다.

| ID | 권고와 현재 준비 | 아직 필요한 결정/근거 |
|---|---|---|
| F-01 | 격리 Go pilot·게시 전 평가 종료·review 로컬 전달 | 실제 수용 범위 채택 |
| F-02 | 현재 source3a530a25·기존 채택 b7e7868/bdf5ad27·go-stablenet0bf2f4d1 후보 | source/복제/submodule 포함 범위 채택 |
| F-03 |9개별 baseline/표본 단위/위치 매칭/분모 초안 | 비교·채점 계약과 실제 baseline 경로 채택 |
| F-04 | family 단위 paired bootstrap·부족/undefined/inconclusive 제안 | 방법·gate·FINAL 충분성 결정 |
| F-05 | AI 초안과 사람 검토를 구분하는 역할 표 | gold 작성·독립 검토·최종 수용 담당 이름/독립성 |
| F-06 | 현재 준비 비용은 실제 모델 호출0 | 실제 LLM pilot을 선택할 때 provider/model/auth/비용/자원 상한 |
| F-07 | 함수40/review10/확장20·rubric45는 후보 초안 | 새 ticket/case·독립 gold·family/DEV/FINAL 봉인 |
| F-08 | 새 host5개와 metadata 계약/종료 검증 | HTTP·전체 역할·노드/권한·정확한 actual recipe와 cleanup |

이번 준비 검증: snapshot 보호16개·운영 수용 보호36개·문서 계약20개·문서134개 검사 통과. 구조 guard의 pending101개는8개 F 결정 영역 아래의 세부 필드/입력/receipt이며101개 결함이나101개 구현 작업이 아니다. --require-frozen은 예상 exit1로 실행을 거부한다. 새 Go 변경 없이1275개 Go와 결속 입력이 동일한 직전 서명 전체4078/race1066 결과를 참조했고, 새 격리 CLI 빌드·metadata·현재 host 시험은 이번에 실행했다.

F-01–08의 decision은 모두 pending이고 freeze=null·execution_allowed=false다. 사람 이름과 독립 정답을 임의 생성하거나 반복/번역/한 함수의 edge를 독립 표본으로 늘리지 않는다. 준비 검증을 마쳐도 정식9개 수용의 선행 조건이 사라지지 않는다. 다음은 위 구체 자료의 역할·범위 검토와 새 독립 입력/정답 준비다. 선택 후속12개는 별도 범위다.


2026-10-15: 이 문서와 JSON의 미동결 gate는9개 정식 평가를 위한 것이다. [지식 매니저·비교 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)의 로컬 설계/개발/회귀를 막는 조건이 아니다. 담당자·실제 모델/비용 지정은 해당 실제 실행·독립 수용 범위에 적용한다. 온톨로지 효과 평가 OE-01은 별도 필수 우선 범위이며 새7개 작업의 구현 완료와9개 독립 수용의 분모를 합치지 않는다.

정책 입력의 후속 [실제 AST 프런트엔드/연결기 진단](./POLICY-REFERENCE-DIAGNOSIS.md)에서 기존6개 경고와 경고 없는 다른 함수 연결1개를 재현했다. 수정7개는 별도 평가 overlay로 사용자 채택을 기록했고 실제 graph/CKS 연결과 명시적 소스 대조 재생성까지 후속 검증했다. 독립 의미/품질 수용은 별도다. 원본 source-bound 입력·과거 packet·gold·동결 상태는 변경하지 않았다. 이 진단은 stage0 준비 근거이며 실제 설치/index/MCP·독립 정책 의미·품질 수용을 대신하지 않는다.

2026-10-15. 단계0은 **preparing**이다. [계획](./OPERATIONAL-ACCEPTANCE-PLAN.md)의 대상·기준을 [실행용 JSON](./OPERATIONAL-ACCEPTANCE-PROTOCOL.json)으로 옮겼고, [진행 원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)에 준비 근거를 연결했다. source 입력 후보와 runtime recipe 초안을 준비했으며 새 정답·검토 승인·확정된 실행 recipe가 없어 실행 허용은 false, 동결 기록은 null이다.

## 이번에 확인한 준비 근거

[source inventory](./OPERATIONAL-ACCEPTANCE-SOURCE-INVENTORY.json)는 Git commit/tree·전체 tree listing SHA256·Go 파일 분포·submodule pin·source schema bytes를 읽기 전용으로 기록한다. 실제 설치·runtime tools/list·모델 또는 독립 정답의 근거가 아니다.

| 저장소 | source 후보 | Go 파일 / 테스트 제외 | tracked 변경 |
|---|---|---|---|
| knowledge-system | ad52d956 | 1376 / 725 | 없음 |
| coding-agent | fdb4d515 | 15 / 10 | 없음 |
| ChainBench | df464d4b | 1177 / 586 | 없음 |
| go-stablenet | 0bf2f4d1 | 1297 / 882 | 없음 |

knowledge-system 준비 checkout은 계획 문서 commit393fd890이다. 실행 source 후보 ad52d956과 구분한다. Go 파일 수는 generated/test/helper도 포함하는 source 목록 수이며 독립 함수/결함/평가 사례 수가 아니다. go-stablenet의 gitlink4개는 pin만 확인했다. submodule bytes·초기화·실행 의존성은 미확인이다. 실행 clone을 만들 때 필요한 gitlink 범위를 먼저 선언하고 검증한다.

CKS source schema와 coding-agent 소비자 schema는 각각33개다. 이름은 dotted/provider alias로 다르므로 runtime 단계에서 mapping과 입력 의미를 확인해야 한다. source33개 관측을 설치된 host 연결 성공으로 세지 않는다.

## 동결 체크리스트

아래 ID는 이번 운영 검증의 동결 요건이며 과거 F-01–06 사례와 별개다. 모든 결정과 근거가 채워져야 단계0을 완료한다.

| ID | 준비한 내용 | 동결 전에 필요한 내용 |
|---|---|---|
| F-01 | 격리 Go pilot, bugfix 평가 gate·review 로컬 전달 범위 | 대상·게시 전 경계의 범위 채택 기록 |
| F-02 | 4개 source pin·tree·schema inventory | 실행 source 후보 채택, clone/submodule 포함 범위 |
| F-03 | 9개 입력군의 최소 표본·metric 초안 | baseline·canonical 위치/edge 매칭·기권/분모·표본 추출 확정 |
| F-04 | family 단위 paired bootstrap 초안 | 방법·표본 충분성·각 gate/구간 판정·지연 percentile 계산법 채택 |
| F-05 | gold/독립 내용 검토/최종 수용/실행 역할 필드 | 사람·역할·독립성 제한 지정 |
| F-06 | BGE-M3/native1024 후보, 자원 한도 초안 | LLM provider/model/version·비용/통화·계량 출처와 시간/node 한도 |
| F-07 | public input/gold·DEV/FINAL record 필드 | 새 사례·gold·원문 span/hash·작성/검토 receipt와 manifest |
| F-08 | runtime preflight·raw evidence 필드 | 실제 설치 방식·정확한 argv/cwd/env/timeout·owned node/port/lease·중단/cleanup recipe |

현재 F-01/03/04는 검토 가능한 초안, F-02는 source 준비 관측이다. F-05/06 담당·모델 정보는 미정이며 F-07/08에는 source 후보·recipe 초안을 추가했다. 정답·분리·runtime 설정·종료/게시 차단 경계는 미확정이다. **어떤 ID도 approved가 아니다.** 사용자의 ‘다음 진행’은 준비 작업을 계속하라는 요청이며 지정되지 않은 사람이나 새 정답을 생성하지 않는다.

## 새로 준비한 입력 후보와 host 관측

[입력 후보 목록](./OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json)은 고정 go-stablenet source의8개 모듈·211개 파일을 Go 표준 AST로 읽어 후보 함수40개·review10개·확장 intent20개를 선정한다. CKG 결과나 기존 gold를 정답 생성기로 사용하지 않았다. 각 함수의 Git blob·file/function SHA256·line/byte span을 기록했고 세부 body는 원래 commit에서 복구한다. generator와 AST helper의 hash도 고정한다.

module별5개·구문 coverage 위주의 결정적 선정은 **복잡한 구문을 보는 후보 추출**이며 무작위/대표 표본은 아니다. closure 내부 구문도 outer function의 metadata에 포함된다. 반복된 short declaration 이름은 shadowing 가능성 hint이며 실제 lexical shadowing의 정답이 아니다. 기존42개 질문에서 이름이 직접 언급된 함수를 제외했지만 의미·fact-family 중복 여부는 미판정이다. 미래 DEV/FINAL과 충분성은 사람 검토 후 정한다. protocol의 approved cases는0, gold는0이다.

WI-V1/V2의 실제 결함 티켓, WI-3의 증상/원인 정답, WI-4의 재현 seed/schedule, WI-2의 rule별 양성/음성, WI-SP의 실제 annotation 정답은 아직 준비되지 않았다. 추출한 함수나 intent를 실제 결함·관련성 정답으로 승격하지 않는다.

[host preflight](./OPERATIONAL-ACCEPTANCE-HOST-PREFLIGHT.json)에는 실제 CLI2.1.299의 version/help와 CKS mcp help, 소비자 source의 hash를 보존했다. model/auth·설치 plugin·실제 hook/tool 호출은 미확인이다. coding-agent README의 예전 stdio 설정과 달리 현재 plugin source는 HTTP CKS를 사용하므로 source·CLI 기준으로 recipe를 작성한다.

기존 baseline 소비자 source의 local analyze는 Jira intake를 생략해도 평가 후 push/PR 생성을 수행한다. auto_merge=false는 merge만 멈춘다. 기존 baseline git hook은 이 게시 전 종료 경계를 보장하지 않으며 auto Stop hook은 비terminal 평가 상태의 실행을 다시 이어가도록 요청한다. 따라서 HOST-PILOT recipe는 게시 차단 검증이 완료되기 전까지 draft_unresolved다. prompt만으로 차단했다고 판정하거나 모의 PR URL/완료 상태를 쓰지 않는다. baseline ChainBench MCP의 SIGTERM exit -15를 개발 회귀시험에서 재현했고 [준비용 소비자 보완](./OPERATIONAL-ACCEPTANCE-READINESS-FIXES.md)의 새 source 후보 bdf5ad27에서 signal/EOF·pollable stdio와 소유한 상태 root를 구현·검증했다. 실제 노드 cleanup·설치 연결은 미확인이다. 이 개발 시험을9개 항목의 actual runtime/품질 gate에 넣지 않는다.

## 새 입력을 작성하는 순서

1. gold 작성자·독립 검토자·최종 수용 담당과 역할 제한을 기록한다. AI 초안은 origin을 AI로 남기고 독립 사람이 요구·원문·별도 방법에서 검토한 부분만 독립 정답으로 채택한다.
2. source pin에서 사례를 선정한다. 결함 티켓2건·증상 사실군20개·Go 함수30개·rule별 양성/음성10개씩·review10건/5module·확장 intent20개·annotation12symbol·schedule family5개가 최소 표본 제안이다. 티켓은 V1/V2에서 공유할 수 있으며 항목/사실군 중복을 manifest에 표시한다. 반복 호출을 새 독립 사례로 세지 않는다.
3. 사례마다 JSON의 case_record_required_fields를 채운다. public input과 별도 gold 파일의 SHA를 기록한다. DEV/FINAL은 family 단위로 나누고 두 manifest에 같은 family가 있으면 분리 실패다. 최소 표본 제안이 FINAL에 어떻게 배정되는지와 DEV 수는 reviewer가 사전에 정한다. 현재 split 비율은 미지정이다.
4. agent가 읽는 public input에는 gold 경로/내용을 전달하지 않는다. gold 작성·리뷰와 실제 agent 실행의 파일 접근 경계를 실행 recipe에 기록한다. FINAL 열람 후 구현을 수정하면 해당 FINAL은 회귀로 전환하고 새 holdout을 선정한다.
5. baseline·허용 위치/span/edge·negative class·미지원·unknown·기권·infra/증상 실패·비용/지연 계산법을 결과 열람 전에 고정한다. 실제 프로젝트의 rule 양성 부족은 inconclusive이며 별도 controlled 자료와 분모를 합치지 않는다.

[검토용 정답·리뷰 초안](./OPERATIONAL-ACCEPTANCE-REVIEW-DRAFT.md)에10개 public review 입력과45개 원문 rubric·3개 비완전 값/제어 anchor 초안을 추가했다. 독립 사람의 채택·전체 edge gold·DEV/FINAL은 미정이다. public에 gold가 섞이거나 비완전 초안을 채점 가능으로 승격하는 상태를 guard가 거부한다. 현재 input_groups의 cases는 모두 빈 배열이다. 이 필드를 채웠다고 gold의 독립성이나 충분성이 자동 입증되는 것은 아니다. 원문·작성 방법·사람의 검토가 필요하다.

## 실행 recipe와 보존할 근거

runtime recipe는 각 단계의 실제 binary/host·argv/cwd·허용 env 이름·timeout·입력·기대 gate·중단/cleanup을 명시한다. 비밀값과 개인 설정은 evidence에 넣지 않는다. 서버/노드는 실행 전에 PID·port·data directory·lease를 owned_resources에 등록한다. 원래 개발자 checkout이나 다른 서비스의 stop/reset을 recipe로 넣지 않는다.

run마다 protocol/input/runtime hash, raw 요청/응답/오류·exit·종료 원인, 실제 role/hook, 예정/성공/결측/취소/infra/증상 분모, latency/token/call/측정 비용, cleanup과 판정을 보존한다. 미측정 비용은 null이다. source pin·schema 점검과 actual host 실행을 구분한다. bugfix의 실제 평가 gate 이후 외부 게시가 필수인 전체 workflow는 이번 범위 밖의 partial로 기록한다. 실행 단계 완료·운영 pass·quality scoped_validated·사람 scope_accepted를 기록할 때 evidence는 실제 manifest의 path/sha256을 포함한다. manifest 존재/hash 검사와 내부 실행/채점의 검토는 별개다.

## 상태 검증과 다음 단계

`python3 scripts/check-operational-acceptance.py`는 현재 JSON 연결·hash·초안 상태와 완료 표기의 선행 조건을 확인한다. 미해결 recipe나 hook/권한을 생략하는 host 옵션도 검사한다. `python3 scripts/check-operational-acceptance.py --require-frozen`은 동결 요건이 미충족이면 실패한다. 이 검사는 사람 신원 인증·gold 의미·통계 충분성·실제 운영 성공 또는 명령의 모든 우회 가능성을 대신하지 않는다.

동결 때 protocol의 freeze 필드를 제외한 JSON 내용을 canonical JSON(sort_keys, separators comma/colon, UTF-8, ensure_ascii false)으로 해시한다. freeze 기록에는 그 protocol_sha256·frozen_at·범위 수용 receipt를 넣는다. receipt는 kind=human, name·decision=approved·protocol_sha256을 갖고 모든 F 요건의 receipt도 같은 해시에 연결한다. freeze_requirements의 receipt와 input_groups의 review_receipt에 있는 protocol_sha256만 해시 계산에서 제외해 순환을 피한다. execution_allowed와 blocked_by도 파생 상태이므로 해시에서 제외한다. 파일/hash 존재 확인은 이름 기록의 인증을 의미하지 않는다.

준비 검증: guard 회귀28개·문서 계약20개·package notice3개 통과, 현재 문서116개 검사 통과. 미동결 상태의 require-frozen은 의도대로 exit1이며 실행 허용을 거부한다. 새 함수 후보40개의 Git blob·파일/span hash·위치 연결과 기존 source/evidence guard·2069개 원본 연결 검사도 통과했고 기존 구현/수용 원장 bytes는 변경하지 않았다. 이 결과는 **준비 문서와 상태 검사**의 근거이며 runtime/독립 품질/사람 수용 성공 수에 포함하지 않는다.

다음은 F-05/06 정보를 반영하며 새 입력·gold와 runtime recipe를 준비하고 F-01–08의 구체 자료를 검토하는 것이다. 단계0 동결 후 단계1 runtime inventory, 단계2 actual host, 단계3 독립 평가, 단계4 사람 수용으로 진행한다. 실행/독립 수용은0/9이며 [전체 남은 작업](../REMAINING-WORK.md)의 별도 범위도 그대로 남는다.

## coding-agent 준비 후보와 운영 경계

[OP-CA-01](./OPERATIONAL-ACCEPTANCE-READINESS-FIXES.md)의 evaluation_only/로컬 EVALUATED 정책을 개발 검증했다.
기본 auto_merge=false만으로는 게시를 막지 못한다. source 후보 채택·실제 host의 hook/도구 권한 연결은 미확인이다.
known-command/tool guard를 네트워크 sandbox로 간주하지 않으며 격리 clone·제한된 host/MCP 정책을 검증해야 한다.
단계0 승인·모델/비용·독립 gold가 정해지기 전 actual pilot을 실행하거나9개 gate를 완료 처리하지 않는다.

## 로컬 host 연결 준비 결과

[격리 채택·연결 명세](./ISOLATED-HOST-ADOPTION.md)의 source 채택과 실제 CLI/로컬 스크립트 응답기5개 사례를 검증했다. coding-agent는 SessionStart 보완 b7e7868, ChainBench는 bdf5ad27이며 source·binary·설정·hook 입력·MCP frame·종료를 결속했다. strict MCP alias와 알려진 게시 거부, Stop 단일 재진입, host 취소/서버 종료를 확인했다.

전체 실제 agent/model/CKS/노드 pilot은 여전히 미실행이다. source/로컬 host 사전 검증만 별도 준비 기록으로 채택하며 단계0 preparing·동결 전 execution_allowed=false·운영/품질/독립 수용0/9를 유지한다. 다음은 reviewer·모델/비용·자원·새 gold/case/family·DEV/FINAL·기준을 확정하는 것이다.

## 정책·보안 source 준비의 판정 범위

[정책·보안 입력 검토](./POLICY-SECURITY-INPUT-REVIEW.md)는 기존 YAML 참조와 source 구문,12개 질의·수동 annotation 초안을 준비한다. 새 정답·독립 사실군·실제 graph/detector 실행으로 간주하지 않는다. stage0에서 독립 검토자와 project/controlled 분모·표본 부족 처리·family/split을 채택해야 한다. 단순 이름 중복 필터는 의미상 독립성이나 FINAL 봉인을 증명하지 않는다. source packet과 public/private 누출·조기 승인/채점/graph 승격을 검사하며 실행/수용0/9를 유지한다.
