# 운영 검증 실행 protocol과 동결 준비

## 2026-10-19 단계52: WI-6 DEV20·관련성/잡음 AI 초안·시작 함수 제외 매칭

[단계52 보고서](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 기존 DEV | 원래20질문/선택20함수/14파일·4intent 유지. 운영 source 후보19개+개발 stress source-only1개. stress main·노드/채굴/거래 실행 금지 |
| 완전 범위 AI 정답 | 400질문/함수 단위: relevant25(시작함수20+명시적 문맥5)·local noise360·unknown15. 원래 질문의 지역 구현/명시 source 문맥 범위이며 전체 저장소·전이/runtime 관련성 정답이 아님 |
| 명시 문맥 | Prune↔RecoverPruning·Init↔newLimbo의 실제 caller/callee와 WriteBadBlock→ReadAllBadBlocks의 저장 데이터 소비 근거를 구분. 동일파일/intent/topic만으로 관련성을 추가하지 않음 |
| seed·분모 | 두 arm에서 같은 시작함수를 primary 점수 universe에서 제외하고 포함 민감도는 별도 보고하는 제안. nonseed 관련 정답이 있는5개와 없는15개 구분. 빈 정답/빈 반환은 recall0/1·개선으로 채우지 않고 undefined/inconclusive |
| 출처·매칭 | 원래 Git0bf2f4d1의14파일 commit/tree/blob과 전체 함수/body·40anchor 해시 검증. project/dataset/commit/source mode/snapshot·정확한 함수/witness 매칭 제안. 중복·unknown·범위밖 자리를 보존하며 상한 뒤 후보로 보충하지 않음 |
| 쌍·warm 제안 | 같은 retained seed/snapshot과 arm별 상한/순서·전수 실패/partial 슬롯 필요. 새 미동결 제안: warmup3쌍+측정30쌍/운영 case, 단일 동시성·AB/BA 교대·같은 소유 stdio 재사용, client end-to-end monotonic 시간 |
| 지연 의미 | 각 arm nearest-rank p95의 비율. paired ratio들의 p95와 구분. baseline비양수/부족/환경변경은 inconclusive. 다른 평가 warm20을 자동 적용하지 않음. 실제 paired/warm 원자료는 아직0 |

[정답 초안](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/private/AI-gold.json.gz)과 [함수·근거 목록](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/private/location-catalog.json), [검토 원장](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/REVIEW-LEDGER.json)을 따른다. seed만 정답에 포함된15개는 더 넓은 nonseed helper·독립 의미 정답이 없으면 확장 효과를 판정할 수 없다. unknown/범위밖을 잡음으로 계산하지 않는다.

새 MCP/모델/대상/노드 실행0·점수null이며 실제 retained seed와 두 요청·전체 설치 역할은 미검증이다. 코드·관계 확장을 argument-return 전이/curated join이나 incoming call 확장으로 일반화하지 않는다. 소스1738개(Go1282개)·승인 입력5개 동일성으로 이전 전체 Go 검증을 재사용한다. 단계51 자료는875cfc29에서 원형 보존한다.

다음은 **WI-SP 수동/자동 policy/security·미태그·거부 경계의 AI 초안**이다. 남은 AI2개는 WI-SP/WI-C다. 남은 전체는 이2개, WI1~3 검토 부채와 WI4/V1/V2 실제 경로·oracle/WI6 nonseed gold·실제쌍·warm, D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 독립 효과다. 선택12개는 별도 backlog로 유지한다.

현재 회귀129개(WI6 35·공통 수용49·문서20·notice3·비교22),875cfc29 원래 소스의 과거 회귀41개, 문서134개·생성45파일 동일 재생·원래 Git14파일/40anchor·Python compile 검사를 통과했다. 원문 signed Git commit의 GPG folded blank header에 대해 최초 staged 공백 검사exit2를 기록하고, 그 원본 commit artifact 한 경로에만 -whitespace를 적용해 재검사exit0으로 확인했다. 원문 바이트와 코드 fmt 규칙은 보존하며, 미승인 gold·미동결 실행은 거부한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용한다.

이하 단계51 이전 항목은 당시의 기록이다.

## 2026-10-19 단계51: WI-V1/WI-V2 공유 DEV 티켓·oracle·후보 AI 초안

[단계51 보고서](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 공유 DEV | 이미 노출된 티켓1개를 V1/V2가 공유. 스케줄5개·후보/대조군 역할5개이며 실제 method 바이트는4종. ORIGINAL=REINTRO를 독립 결함/표본으로 세지 않음 |
| 완전 범위 AI 대조 | 요구사항12개 × 후보/스케줄25조합=300단위 전수 분류. conditional source fit89·source conflict26·runtime unknown125·not applicable60. 실제 API/실행 관측 결과가 아님 |
| 후보 정답 초안 | 명시한 유한 메서드 조건에서 A/B 적합, 원본/재도입/조기반환 부적합. 실제 API·제품 수용 membership는5개 모두unknown, 독립 acceptable set은null·선택 후보없음 |
| 실제 경로·원문 | 원래 Git5파일·API anchor13개, source/file/method witness·제안 전체 파일 overlay/diff SHA 작성. 실제 적용 commit/tree/binary/driver는 미정 |
| oracle 의미 | 완전한 통제 event trace를 대상으로 삭제→ack→반환·요청1개·반환후 channel 미사용·정상알림·소유정리를 확인하는 초안. RPC eth_unsubscribe true는 내부 ack/반환 대체 불가 |
| 독립성·선택 경계 | 별도 wait graph도 같은 AI/가설이며 독립 executable oracle이 아님. 실제API·정답/후보 독립 검토·필수 gate unknown은DEFERRED. all-bad 무선택·tie 허용·성능/공정성 점수 미생성 |

[요구사항과 원문](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/private/requirements.json), [25조합 정답 초안](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/private/AI-matrix.json), [후보 집합 초안](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/private/AI-membership.json)을 검토할 수 있다. 초안의 conditional_source_fit은 코드/유한 전제에 대한 AI 판정이며 실제 callback 도달성·종료 공정성·완전 제품 계약·독립 승인에 해당하지 않는다. 미도달/도구/timeout/오류는 symptom RED나 정상 GREEN으로 바꾸지 않는다.

새 API/대상/MCP/모델/노드 실행0, 품질 점수null이다. 소스1738개(Go1282개)·승인 입력5개 동일성으로 이전 전체 Go 검증을 재사용한다. 단계50 packet과 도구는20f470f2에서 원형 보존한다.

다음은 **WI-6 기본/확장 DEV 쌍과 범위 내 완전 AI 관련성·잡음 정답**이다. 남은 AI3개는 WI-6/WI-SP/WI-C다. 남은 전체 필수 범위는 이3개, WI1~3 검토 부채·WI4/V1/V2 실제 경로/독립 oracle, D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 독립 효과다. 별도 선택12개는 기존 backlog로 유지한다.

현재 회귀134개(공유 티켓41·공통 수용48·문서20·notice3·비교22),20f470f2 원래 소스의 과거 회귀43개, 문서134개·생성23파일 동일 재생·Python compile/공백 검사를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용하며 새 Go 전체 시험으로 기록하지 않는다.

검사 기록 보완: 최초 staged 공백 검사에서 원문 unified diff의 문맥 접두사(공백+Go tab/빈 문맥행) 때문에 exit2가 발생했다. 결과 확인 전에 커밋한 실행 순서 오류를 기록하며, 원문 packet 33파일은 3caf64d6과 바이트 동일하게 보존했다. 후속 .gitattributes는 해당 diff 자료 경로에만 -whitespace를 적용한다. git show --check 재검증은exit0이며 코드의 fmt 규칙에는 예외를 추가하지 않는다. [검사 보완 기록](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1-format-review.json)을 따른다.

이하 단계50 이전 항목은 당시의 기록이다.

## 2026-10-19 단계50: WI-4 DEV21·조건부 결과 AI 초안

[단계50 보고서](./operational-preparation/source-review-supplement/followups/wi4-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi4-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. 사용자가 요청한 AI 초안6개·WI1~3 부채·독립 수용 준비·정식9개·온톨로지 효과를 순서대로 진행한다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| WI-4 DEV | 기존5개 메커니즘 제안·21스케줄의 순서를 유지. 실제 진입·채널/버퍼·잠금·동시 준비·원문 버전을 전제조건으로 보강 |
| 결과 AI 초안 | 명시한 유한 vocabulary150단위 전수 분류: legal22·조건부 symptom_candidate11·unknown33·infra/unreached84. 각 결과의 source span·hash·사유와 빈 집합 사유 작성. 모든 실행 trace/제품 계약의 완전 gold는 아님 |
| 두 의미 축 | Go/원문에서 허용되는 동작과 제품 계약 위반을 구분. WORK-both의 send/exit 모두 합법, backlog의 즉시 종료·선택 빈도·공정성을 가정하지 않음 |
| 미확정/경계 | UDP 합성 deadline과 실제 monotonic 경로·타이머 firing을 분리. Gossip 재시도 억제를 결함으로 단정하지 않음. pool은 실제 adds/구독·버퍼/Stats 순서를 요구. 구독 ack·반환·구조 수신의 관측 순서 구분 |
| 매칭·분모 | 정확한 family/schedule/source·binary/driver/environment·seed/arm/order·진입 trace 요구. 결과 ID만으로 실행·독립 oracle 승인 불가. 계획·증상·unknown·오류·미도달·누락·cleanup 오류 보존 |
| 동결 전 부채 | 반복21개가 독립21가족이 아님. 독립5가족·arm당 계획100·baseline증상10·비율0.50·family paired 불확실성·600초 관찰·설치 역할은 미동결/미실행. 무재발·baseline0은 inconclusive |

unknown을 기존3집합 formal 작성 양식의 legal/infra에 숨겨 넣지 않고 **별도 네 번째 AI 집합**으로 유지한다. 정식 양식·제품 계약의 독립 검토가 남는다. [검토 원장](./operational-preparation/source-review-supplement/followups/wi4-dev-gold-v1/REVIEW-LEDGER.json)에 실제 도달성·독립 oracle·동결 arm/오라클·정리/모드9·instrumentation·설치 역할 부채를 기록한다. 원래 Git chain과 파일/좌표/hash, 과거 go1.26.9 stdlib 원문을 검증하며 대상 toolchain 채택으로 승격하지 않는다.

새 대상 실행·MCP·모델·노드·품질 점수는0이다. 유지보수 소스1738개(Go1282개)·승인 입력5개의 동일성을 확인해 이전 전체 native 검증을 재사용하며 새 Go 전체 시험으로 기록하지 않는다. 이전 단계49 증거/도구는d95b3e91 소스에 결속해 보존한다.

다음은 **WI-V1/WI-V2 공유 DEV 티켓의 완전 AI oracle·수용 가능 후보 집합**이다. 남은 AI5개는 WI-V1/V2/6/SP/C다. WI1~3 부채와 WI4 실제 도달성/독립 oracle·수용 준비(D04·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08)·정식9개(0/9)·필수 온톨로지 독립 효과는 미완료다. 별도 선택 후속12개는 기존 backlog이며 이번 요청의 필수 범위와 구분한다.

현재 회귀135개(WI4 43·공통 수용47·문서20·notice3·비교22), d95b3e91 원래 소스의 과거 회귀37개, 문서134개·생성30파일 동일 재생·Python compile/공백 검사를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부한다. 결속 소스1738개(Go1282개)·승인 입력5개 동일성으로 이전 native 전체 검증을 재사용하며 새 전체 Go 시험으로 기록하지 않는다.

이하 단계49 이전 항목은 당시의 기록이다.

## 2026-10-19 단계49: WI-3 source unknown·지원 문맥·실제 증상 검색 매칭

D04는 **미정**이다. [단계49 보고서](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/WORKLIST.json)이 최신 진입점이다. **독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다. 단계48 원본400단위·도구는8553fe97 근거로 그대로 보존한다.

| 점검·검증 범위 | 관찰과 한계 |
|---|---|
| source unknown | 원래400단위 유지. runReorg의 명시적 예약→승격→알림 연결을 근거로 RQ-007/008 상호2조합만 별도 AI 지원 문맥으로 보완. 새 관련25·비관련364·unknown11이며 runtime 원인/독립 승인이 아님 |
| 실제 증상 입력 | 원래20문안만 실제 CKS stdio로 순서대로 요청. symbol/file/expand 정답 힌트 없음.1000ms·후보10·seed3·enriched3·call32·depth2 유지, primary 재시도0 |
| 실제 응답 |20개 모두 partial. 후보68개·출처657곳을 보존. symbol_identity_unavailable·unanchored_edge와 일부 call/edge/candidate/seed 상한, provider error·unsupported를 숨기거나 성공 응답으로 변경하지 않음 |
| 원문·프로젝트 경계 | 후보/중첩 신호657곳의 legacy/V2 일치, project/dataset/source snapshot/commit/mode/origin·파일/내용 SHA 확인.46파일을 retained archive에서 보존하고 원래 Git0bf2f4d1 원문과 대조 |
| 엄격/보완 매칭 | 단계48 엄격 제안 매칭0은 그대로 기록. 문서 주석이 붙은 전체 함수2범위를 Go AST의 Doc+FuncDecl 소유·좌표·해시로 확인해 새 보완 제안에서2개 연결. 임의 포함/이름/canonical 단독 매칭은 불허 |
| 범위 밖·분모 | 나머지66개는 선택20함수 범위 밖·미라벨 unknown. 원래 후보 자리/순서 보존, 중복 제거 후11위 보충 없음. 실제 검색 출력으로 의미 gold를 만들거나 precision/recall을 계산하지 않음 |
| 모델·거부·종료 | 로컬 BGE-M3 기존 digest/native1024·Ollama0.40.4 확인. 잘못된 commit·후보 상한21·다른 프로젝트 확장 citation을 실제 거부. EOF0·retained 전후 hash 동일 |
| 실행 수용 경계 | 실제 CKS 연결·출처·매칭 관찰이며 설치 agent의 조사·원인 판정/독립 수용이 아님. 유료/외부 모델·설치 역할 실행0. 같은20문항의 반복 자료를 독립 가족으로 세지 않음 |

[검토 부채 원장](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/REVIEW-LEDGER.json)에 source unknown11개, 모든 partial/stop reason, 범위 밖66자리와 provider·설치 역할·독립 의미/gold/평가 계약의 부채를 유지한다. 문서 주석 보완은 **관찰 후 제안한 DEV adapter**다. 기존 엄격 계약의 결과를 소급해 변경하지 않으며 정식 matching 승인은 미정이다. 직접 동작20개와 지원 문맥5개의 AI 관련성은 실제 원인이나 의미 정답 승인 수가 아니다.

유지보수 Go1738개·승인 입력5개의 동일성을 확인해 이전 native 전체 검증을 재사용한다. 별도 AST 관측기의 build/vet/fmt는 새로 검증하며 전체 Go 시험을 새로 수행한 것으로 기록하지 않는다. 운영 current·전역 설정·원문 DB·외부 게시를 변경하지 않았다.

다음은 **WI-4 DEV 입력과 범위 내 완전한 AI 결과 정답 초안**이다. 남은 AI 작성6개는 WI-4/V1/V2/6/SP/C다. 남은 전체는 이6개 작성과 WI1·WI2·WI3 검토 부채, D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용(0/9), 필수 온톨로지 독립 효과, 별도 선택 후속12개다. [전체 작업표](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/WORKLIST.json)에 모두 미완료로 유지한다.

현재 회귀128개(WI3 37·공통 수용46·문서20·notice3·비교22), 원래8553fe97 소스의 과거 회귀34개, 문서134개·생성31파일 동일 재생·Git46파일 대조·출처657곳 검증을 통과했다. 보조 AST 관측기 build/vet/fmt 통과, 미승인 gold·미동결 실행은 예상대로 거부했다. [검증 기록](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/validation-summary.json)을 따른다.

이하 단계48 이전 항목은 당시의 기록이다.

## 2026-10-19 단계48: WI-3 DEV20·선택 함수 범위 내 완전한 AI 정답 초안

D04는 **미정**이다. [단계48 보고서](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지하며 단계47 근거와 도구는5b72156d의 과거 검증으로 보존한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| DEV 입력 | 기존 증상 전용20문안·7유형 유지. 공개 요청에는 정답 위치·함수명·가족/정답 경로를 넣지 않음 |
| 완전 universe | 선택20함수/17파일 ×20질문, 총400단위 전수 AI 분류. 관련23·비관련364·unknown13. 전체 저장소의 완전 정답은 아님 |
| 의미 근거 |20함수 전체 body를 검토하고 원래40개 소스 사실 근거·새 정확한 witness 좌표·해시·개별 판정 사유를 결속. 직접 동작20개와 지원 문맥3개를 구분 |
| unknown | 같은 가족/파일/용어를 근거로 관련 위치를 자동 승인하지 않음. 전이·comparator·caller/consumer·runtime 계약이 미확정인13단위는 유지 |
| 정답 내용 | 질문별20위치의 관련/비관련/unknown 집합과 빈 집합 사유, 원문 commit/file/function SHA·UTF8 위치, 범위 밖 unknown 정책을 작성 |
| 가족·노출 | 기존15그룹 제안·190쌍 중복·840건 과거 질문 대조 유지.20카드는 독립20가족이 아니며 노출된 DEV. 독립 가족 승인0·새 FINAL0 |
| 매칭 제안 | project/dataset/commit/source mode/graph snapshot과 파일·함수 SHA, 정확한 전체 함수 또는 명시 witness span을 요구. 이름/canonical 단독·임의 겹침은 unknown |
| top-10 제안 | label_suspects 최종 후보의 원래 앞10자리를 사용, 확인된 같은 함수만 중복 제거하고 뒤 순위로 보충하지 않음. unknown/오류/범위 밖 자리와 원래 계획 분모 유지 |
| 실행 경계 | 실제 증상 검색·새 MCP·모델·설치 역할 실행0, 품질 점수null. 과거 pilot pin과 wire 형식은 새 검색/정식 baseline 승인 근거가 아님 |

[검토 가능한 각 질문의 정답](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/private/RQ-001-REVIEW.txt)과 [전수 AI 정답](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/private/AI-gold.json)을 작성했다. 비관련은 이20함수의 명시적 local 동작 검토 범위이며 안전/무관한 전체 저장소를 증명하지 않는다. 범위 밖 위치는 미라벨·unknown으로 유지한다. 입력·정답의 경로 분리는 OS 격리나 봉인된 FINAL이 아니므로 실제 평가 host에는 public만 제공해야 한다.

실제 retained 검색 응답을 검증한 adapter·표본 충분성·독립 의미 gold·원인 계약·case/family macro·baseline/빈 분모·F01~08 동결은 남아 있다. 이번 완료는 bounded AI DEV 작성과 오프라인 매칭 제안의 검증이다. 유지보수 Go1738개·승인 입력5개의 동일성을 확인해 이전 native 전체 검증을 재사용하며 새 전체 Go 시험으로 기록하지 않는다.

다음은 **WI-3 unknown·지원 문맥·실제 top-10 소비자 매칭 보완 후 WI-4 DEV/AI 초안**이다. 남은 AI 작성6개는 WI-4/V1/V2/6/SP/C다. 남은 전체는 위WI3 보완과6개 작성, WI1·WI2 검토 부채, D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용(0/9), 필수 온톨로지 독립 효과, 별도 선택 후속12개다. [전체 작업표](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/WORKLIST.json)에 미완료로 유지한다.

현재 회귀124개(WI3 34·공통 수용45·문서20·notice3·비교22), 원래5b72156d 소스의 과거 회귀38개, 문서134개·생성51파일 동일 재생·원래 Git17파일 대조를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부했다. [검증 기록](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/validation-summary.json)을 따른다.

이하 단계47 이전 항목은 당시의 기록이다.

## 2026-10-18 단계47: WI-2 unknown·호출 문맥·실제 소비자 매칭 보완

D04는 **미정**이다. [단계47 보고서](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/WORKLIST.json)이 최신 진입점이다. 단계46 원본108개 AI 초안과 당시 도구를0993202d 소스에 결속해 보존했다. **독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 보완·검증 항목 | 결과와 한계 |
|---|---|
| unknown 의미 | 원래538단위를 보존. 읽기 전용 scalar 매개변수 캡처를 쓰기·주소 노출과 분리해 CALL-046의 cmd/args 2단위만 보완. 새 AI 분류 positive20·negative505·unknown13. 기존15 unknown은 원본에 유지 |
| 호출 대상 | 선언 import·동명 객체·별칭·정확한 result/binding을 구분. 패키지 전체 타입 확인·링크·공격자 제어·실행 가능성을 증명한 것이 아님 |
| 호출자14개 | 원래 Git 파일/호출 좌표·해시·import/가림을 확인. literal13개·identifier1개를 해당 site의 문맥으로만 기록. wrapper 전체 안전·전이 flow·독립 표본으로 승격하지 않음 |
| 실제 소비자 | 프로젝트51함수를 탐색해42개 실제 CKS rule 조회, 통제40개도 CKS stdio 조회. 원래68호출 질문은 함수 응답을 중복하지 않고 source-result/sink 단위로 연결 |
| 미도달·상한 | 심볼 미도달9함수/20단위, primary budget_exhausted2함수/16단위는 그대로 unknown. 나머지80함수의 규칙 계산은 matched_bounded이며 기본 분석 partial 의미 한계와 구분 |
| 실제 매칭 | 반환22후보 전부 선언 단위에 연결, 미연결0. 소스 AI 초안 기준 positive18·negative2·unknown2; negative/unknown4개와 미반환 alias positive2개는 검토 원장에 보존. 품질 점수 아님 |
| 조건·좌표 | 실제 조건8개를 원문 노드·CFG 분기 ordinal로 연결. 형제 클로저는 깊이가 같아도 owner 좌표로 분리. 통제 원문 조각은 유지하며 같은 길이의 고유 package prefix와 원래/실제 파일 SHA를 명시 |
| 경계·보존 | 두 CKS 경로 각각3종 잘못된 pin·pack/상한 오류·incomplete 전달 검증. 최종4세션 EOF0, 원래 retained graph/sources/vector 전후 hash 동일. 초기 harness 오류·별도42조회도 진단 근거로 보존 |
| 소스 통제 | 읽기/쓰기·주소 노출·참조형·가림·형제 클로저·한글 식별자7개. 실제 보조 관측기 build/vet/fmt 검증. 불투명 helper·간접/복합 sink·bare return·클로저 실행은 unknown 유지 |

[검토 부채 원장](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/REVIEW-LEDGER.json)은 소스 unknown13개, 미도달9함수, 상한2함수, 반환 후보의 소스 초안 차이4개와 미반환 positive2개를 별도 축으로 기록한다. 중복되는 축을 독립 사례 수로 합산하지 않는다. 빈 후보는 안전이나 음성 정답이 아니며, 실제 소비자 출력을 소스 의미 정답으로 복사하지 않았다. 정답 초안과 공통 x/tools CFG의 한계, caller 전이·대상 정체·조건 의미의 독립 검토가 남는다.

이번 완료 범위는 **AI DEV 보완과 실제 소비자 매칭 관찰**이다. 정식 gold 사용은 거부한다. 유지보수 Go1738개·승인 입력5개의 동일성을 확인해 이전 전체 native build/일반/race/fmt/vet/lint 근거를 재사용하며, 새 전체 native 시험으로 기록하지 않는다. 운영 current·전역 설정·외부 게시를 변경하지 않았다.

다음은 **WI-3 DEV 입력과 범위 내 완전한 AI 정답 초안**이다. 남은 AI 작성7개는 WI-3/4/V1/V2/6/SP/C다. 남은 전체는 이7개 작성, WI1·WI2 검토 부채와 D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용(0/9), 필수 온톨로지 독립 효과 평가, 선택 후속12개다. [전체 작업표](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/WORKLIST.json)에 모두 미완료로 유지한다.

현재 범위 회귀127개(WI2 38·공통 수용44·문서20·notice3·비교22), 원래0993202d 소스의 과거 회귀33개, 소스 통제7개, 문서134개, 생성73파일 동일 재생·원래 Git34파일 대조를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부한다. 상세 검증 기록은 [validation-summary](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/validation-summary.json)에 유지한다.

이하 단계46 이전 항목은 당시의 기록이다.

## 2026-10-18 단계46: WI-2 DEV 입력과 범위 내 완전한 AI 정답 초안

D04는 **미정**이다. [단계46 보고서](./operational-preparation/source-review-supplement/followups/wi2-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi2-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. DEV 초안의 작성·기계 검증은 독립 승인·정식 수용과 구분하며 **정식0/9·freeze null·execution_allowed=false**를 유지한다. 단계45 자료와 도구는5d84449e 소스에 결속해 과거 근거로 보존한다.

| 항목 | 이번 결과와 범위 |
|---|---|
| 통제 DEV | 원래 함수 조각40개 바이트 보존, 별도의 명시적 import/type/bodyless helper 문맥 가정.40개/49단위, AI positive18·negative23·unknown8 |
| 프로젝트 DEV | 원래68호출/51함수/34파일, commit0bf2f4d1·file/function/call SHA·UTF8 좌표에 결속.68개/489단위, AI positive1·negative481·unknown7 |
| 완전 universe | 명령 이름 slot × 모든 named parameter 또는 no-source sentinel; 변환 호출 × 같은 closure 깊이의 모든 반환 slot·bare/void/no-return sentinel.538단위 전부 분류 |
| 정답 내용 | rule/source/sink/binding·callee/import/shadow·RHS/CFG 의존 witness·조건 ordinal·구체 사유, 전수 answer sets와 빈 집합의 AI 사유 |
| 반환 identity | 하나의 bare return이 여러 named result를 반환하면 같은 좌표라도 sink binding으로 분리.여러 결과를 하나로 접지 않음 |
| 문맥/한계 | alias·동명 객체·opaque helper·복합/간접 sink·closure·주소/range·부분 타입을 별도로 기록.소비자 계약 분류와 소스 검토 분류를 분리 |
| 별도 호출자 | 기존14개 참조를 source-only 문맥으로 유지.108입력이나 독립 표본에 합산하지 않고 runtime/attacker control은 미확정 |
| 승인/실행 | source-only AI 제안; candidate engine·새 MCP·외부 모델 호출0.독립 gold0·FINAL0·실제 retained 소비자 매칭/동결 미실행 |

negative는 **선언한 규칙의 source–sink 단위**에서 조건이 없다는 의미다. 변환 오류 변수가 `_`가 아니어도 오류를 검사했다는 뜻이 아니며, 다른 명령 인자·다른 선언·다른 반환 slot을 합치지 않는다. positive도 lexical may-reach 후보 검토 조건이며 취약성·공격자 제어·실행 가능성을 증명하지 않는다. unknown을 빈 탐지/음성 정답으로 바꾸지 않았다. 통제용 prefix는 원본에 없던 **AI DEV 문맥 가정**이며 실제 retained 좌표 adapter·대상 정체의 독립 검토가 남는다. 소비자는 함수 전체의 두 규칙 결과를 반환하므로, 다음 단계에서51함수 응답을68선택 호출에 매칭·중복 정리하고 미연결 출력은 unknown으로 유지해야 한다. 반복 호출을 독립 표본으로 세지 않는다.

공개 DEV 입력과 비공개 정답 경로를 분리했다. 이 packet 전체를 평가 host에 제공하지 않으며 Git에 노출된 자료를 새 FINAL로 재명명하지 않는다. source AST/부분 go-types/CFG 참조 계산은 후보 분석기의 출력으로 정답을 복사하지 않는다. 동일 x/tools CFG 구성기를 공유하는 공통 오류 한계가 있다. `check-wi2-dev-gold.py --require-formal-gold`는 정식 gold 사용을 거부한다. 전체 native Go 검증은 소스 결속을 확인해 이전 근거를 재사용하며, 보조 관측기 검증과 구분한다.

다음은 **WI-2 unknown·callee/호출자 문맥·실제 소비자 매칭 검토 보완 후 WI-3 DEV/AI 초안**이다. 남은 AI 작성7개는 WI-3/4/V1/V2/6/SP/C다. WI1 검토 부채, D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용, 필수 온톨로지 독립 효과, 선택 후속12개는 [전체 작업표](./operational-preparation/source-review-supplement/followups/wi2-dev-gold-v1/WORKLIST.json)에 미완료로 유지한다.


## 2026-10-18 단계45: WI-1 unknown 의미·값 종류·합성 좌표·실제 소비자 매칭

D04는 **미정**이며 독립 승인0·정식 수용0/9·freeze null·execution_allowed=false를 유지한다. [보완 보고서](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/REPORT.txt)와 [현재 단계·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/WORKLIST.json)이 최신 진입점이다. 단계44 원본 초안과 manifest는 그대로 보존하고 당시 검사는4529b6d1 소스에 결속한다.

| 보완 항목 | 검증 결과와 한계 |
|---|---|
| unknown 의미 | lexical 경로 유무, 값 종류, 구체 값, memory/alias/range/capture, 좌표 매칭, 분석기의 partial 상태를 분리. unknown을 음성·빈 집합·안전으로 바꾸지 않음 |
| 값 종류 | 같은 선언의 부분 파일/고립 함수 타입 문자열·오류·분류를 대조;412선언 중73개 분류 차이. 모든 구체 실행 값은 unknown |
| 보수적 의미 초안 | 기존 unknown1004는 원본 보존. 새 값 의미 축은1654쌍 중unknown1427·positive212·negative15. reference/aggregate는 내용·별칭 증명이 아니므로 구조 경로와 구분 |
| 실제 원문·소비자 | 원래 go-stablenet30함수, 원문 commit/함수 SHA/graph snapshot pin으로 실제 CKS stdio 호출. 파일·줄의 중복 후보는 정규 심볼 ID로 구분 |
| 합성 좌표 | 별도 소유 인덱스14사례; entry와 선언 좌표, Rbrace 합성 반환, bare return, UTF8, shadow, read/write phase, block -1 상수를 구분. 프로젝트30 표본에 합산하지 않음 |
| 노드·선언·블록 | 노드·선언 미연결0. predecessor/successor·분기 순서로 빈 블록을 대조, 미연결3개는 unreachable 빈 블록. live branch–block12968쌍 매칭 누락/구조 차이0 |
| 남은 검토 부채 | occurrence phase53개(range 암묵 정의37·소스 CFG 소유 중복16), 매칭된1553쌍 중 lexical 차이2개, 역방향 미연결 소비자 def-use32개. [검토 원장](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/REVIEW-LEDGER.json)에 보존; 품질 점수 아님 |
| 상한·종료·보존 | 실제 호출1회가 최종 원문 검증 상한으로 budget_exhausted; 응답 보존 후 동일 pin/상한 재확인에서 partial 반환.4세션 정상EOF0, 잘못된3종 pin 거부, retained 파일 전후 hash 동일 |

단계45는 **AI DEV 보완과 실제 매칭 관찰**의 완료다. 의미 정답/독립성·정밀도/재현율·실행 수용의 승인이 아니다. 공유 CFG 구성기의 공통 오류 한계, memory·호출 효과·실제 경로 가능성의 한계가 남는다. 소비자 응답을 정답으로 복사하지 않았고 구조 차이2개도 초안 정답으로 자동 승격하지 않았다. `check-wi1-consumer-matching.py --require-formal-gold`는 정식 정답 사용을 거부한다. 새로운 전체 native Go 검증으로 포장하지 않고, 소스1738개·승인 입력5개의 동일성을 확인해 이전 native 검증을 재사용한다.

다음은 **WI-2 DEV 입력·완전 AI 정답 초안**이다. 나머지 DEV/AI 작성8개는 WI-2/3/4/V1/V2/6/SP/C이며, WI1 검토 부채·D04 역할/독립성·새 미노출 FINAL·독립 완전 gold·matching/충분성·F01~08 동결은 별도 미완료다. 정식9개 실행·수용과 필수 온톨로지 독립 효과 평가, 선택 후속12개도 [전체 작업표](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/WORKLIST.json)에 유지한다.


## 2026-10-18 WI-1 DEV 입력과 범위 내 완전한 AI 정답 초안

[단계44 자료](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/REPORT.txt)는 기존30함수의 노출된 DEV 입력과 AI 제안 정답이다. formal split·새 FINAL·독립 승인·품질 점수는 생성하지 않았다. public 입력과 private 정답을 다른 디렉터리에 두며, 이 전체 packet을 host의 읽기 root로 사용하지 않는다. 실제 수용 사례에는 private 정답 접근 통제가 별도로 필요하다.

| 작성 항목 | 범위·남은 조건 |
|---|---|
| DEV30개 | 원래 프로젝트/commit/file/function SHA·절대 UTF8 span·원문 bytes에 결속; 독립 holdout으로 세지 않음 |
| 정의–사용 초안 | 1654쌍 전수 분류, unknown1004 유지; source/sink·read/write phase·entry·shadow를 구분 |
| 제어 초안 | branch–block12968쌍과 별도 node projection; normal-exit 구조, 비종료 영역은 unknown으로 별도 기록 |
| signature 경계 | callback 함수형 선언4개를 바깥 함수의 지역 binding/정답에서 격리 |
| 매칭 | 선언·좌표·node kind/역할·합성 entry/return·중복 ordinal 제안; 실제 retained ID/snapshot/소비자 변환·최종 동결 미완료 |
| 독립성 | 원문/AI 참조 초안; CFG 라이브러리 공유와 부분 파일 타입 문맥의 공통 오류·문맥 차이를 명시; 독립 사람 검토 전 |

`scripts/check-wi1-dev-gold.py`는 원문·좌표·전수 pair universe·label·대조 계산·독립 승인 미승격을 검사한다. `--require-formal-gold`는 이 초안을 정식 정답으로 사용하는 요청을 거부한다. 현재 준비 참조의 종류는 `WI1_DEV_AI_gold_not_formal_execution`이며, 단계43 자료의 도구 결속 검사는0f28a9a7 버전으로 보존한다. 과거 packet/manifest를 현재 도구에 맞춰 다시 쓰지 않는다.

정의–사용은 명시한 지역 binding의 may-reach 관계이며 memory alias/호출 효과·경로 가능성이나 취약점 안전성의 정답이 아니다. 파일 단위 부분 타입 정보는 실제 고립 함수 분석의 타입 결과와 같다고 간주하지 않는다. 상수·range·주소 노출·capture·미해결 타입·매칭이 불명확한 관계를 unknown으로 남긴다. F03/F07의 완전한 **독립 의미 정답** 조건은 이 초안만으로 충족되지 않는다. D04미정·정식0/9·freeze null을 유지한다.

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
