# 현재 인수인계

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

negative는 **선언한 규칙의 source–sink 단위**에서 조건이 없다는 의미다. 변환 오류 변수가 `_`가 아니어도 오류를 검사했다는 뜻이 아니며, 다른 명령 인자·다른 선언·다른 반환 slot을 합치지 않는다. positive도 lexical may-reach 후보 검토 조건이며 취약성·공격자 제어·실행 가능성을 증명하지 않는다. unknown을 빈 탐지/음성 정답으로 바꾸지 않았다. 통제용 prefix는 원본에 없던 **AI DEV 문맥 가정**이며 실제 retained 좌표 adapter·대상 정체의 독립 검토가 남는다.

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


## 최신 상태 — 2026-10-18 / WI-1 DEV·AI 정답 초안

**단계44: 기존 프로젝트30함수의 DEV 입력과 범위 내 완전한 AI 초안30개를 작성했다.** 원문25파일에 결속한 지역 binding412개·occurrence1721개를 다루고, 콜백 함수형의 signature4개는 별도 격리했다. 정의–사용쌍1654개(positive556/negative94/unknown1004), 별도 shadow 음성410쌍, 구조 제어 branch–block쌍12968개(positive737/negative12231)를 전수 분류했다. 이는 후보 분석기의 품질 점수나 독립 정답 승인 수가 아니다.

public 입력/원문과 private AI 정답을 분리했다. 정답은 후보 valueflow 출력 없이 원문 관측 및 새 참조 알고리즘으로 작성하고 정의–사용/제어 각각 두 방식으로 대조한다. CFG 생성기는 후보와 같은 x/tools0.48.0이므로 공통 오류 가능성이 있다. 파일 단위 부분 타입 관측과 후보의 고립 함수 타입 문맥도 같다고 가정하지 않는다. 주소·range·capture·상수·미해결 타입·비종료와 실제 consumer 매칭 한계를 보존하며, 지원 밖 의미까지 완전한 운영 정답을 만들었다고 판정하지 않는다.

각 함수의 `private/SRC-xxx-REVIEW.txt`에는 전체 use별 정의 집합과 unknown 사유·블록 매칭·제어 양성/unknown·음성 complement를 사람이 읽을 수 있게 작성했다. [SRC-007 검토 예시](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/private/SRC-007-REVIEW.txt)를 볼 수 있다.

[현재 보고서](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/REPORT.txt), [초안 현황](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/SUMMARY.json), [현재 단계와 남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/WORKLIST.json), [검증 기록](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/validation-summary.json)을 따른다. 다음은 WI1 의미 unknown/값 종류·합성 좌표와 실제 소비자 매칭 보완, 이후 WI2 DEV/AI 초안이다. 나머지8개 WI 초안·새 DEV/FINAL 독립 가족/완전 의미 gold·F01~08 동결, 정식9개 수용, 필수 온톨로지 효과 수용, 별도 후속12개가 남는다.

D04미정·독립 승인0·FINAL0·정식0/9·freeze null/execution_allowed=false를 유지한다. 이전 단계43 packet과 도구 source pin은0f28a9a7의 과거 검증으로 보존하고, 현재 준비 검사는 새 WI1 DEV 종류를 사용한다. 유지보수 Go·승인 입력·원문/DB/운영 current·전역 설정은 변경하지 않는다.

현재 범위 회귀112개·원래0f28a9a7 소스의 과거 결속 회귀49개, 수기 source 통제14개·원래 Git25파일 대조·payload101파일 동일 재생·문서134개·미승인/미동결 거부 검증을 통과했다. 새 보조 관측기는 build/vet/fmt 통과이며 유지보수 Go 전체 시험을 새로 실행한 것으로 기록하지 않는다.

## 이전 준비 — 2026-10-18 / 독립 수용 동결 준비

**단계43의 준비 작업 FP01~04를 완료했다.** 기존 후보206행을 원본 SHA/행 좌표·노출 이력과 연결하고,9개 WI와 온톨로지의 완전 정답·매칭 작성 양식10개, 표본 충분성 원장, F01~08 준비표와 FINAL 접근·봉인 절차를 작성했다.206행은 독립 표본206개가 아니다. 모두 노출된 DEV 작성 자료이고, 새 FINAL·승인 gold는0개다. WI3의20입력은 제안 가족15개, WI6은 운영19개+stress1개, V1/V2는 공유 개발 티켓1개다.

검사기는 정답의 null/빈 집합·범위 누락·중복 분류·출처·검토 hash·DEV/FINAL 가족 중복·프로젝트별 위치 매칭을 구분한다. 온톨로지는 DEV4모드 후 FINAL primary1모드 사전 선택, 검토된 구현-개념 연결, 독립 의미 정답, 동일 pin/cap과 혼입·인용·지연 gate를 준비했다. 검색 개선을 실제 LLM 답변 품질로 판정하지 않는다. 검사 통과는 의미 정답의 정확성이나 사람 검토를 증명하지 않는다.

[현재 보고서](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/REPORT.txt), [현재 단계와 남은 전체 작업](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/WORKLIST.json), [F01~08 준비표](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/freeze-readiness.json), [검증 기록](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/validation-summary.json)을 따른다. 다음은 D04 역할·독립성 지정과 새 가족/입력·완전 gold의 작성·검토다. D04미정, 정식0/9, freeze null/execution_allowed=false를 유지한다. 로컬 로드맵7/7·28/28, 격리 준비22/22, 비운영 pilot4/4는 완료 상태를 유지한다. 선택 후속12개는 별도 backlog다.

이번 변경은 Python/문서이며, 결속 소스1738개와 승인 입력5개의 hash가 이전 검증과 동일하다. Go 전체 시험을 새로 수행한 것으로 기록하지 않는다. 원문·데이터·운영 current·전역 설정·이전 증거는 보존했다.

현재 회귀133개·원래 버전9de07a82의 과거 준비 회귀36개, 문서134개·완료 보호·미동결 실행 거부 검증을 통과했다. 현재 준비 참조와 과거 host 준비 참조를 분리해 원래 증거의 소스 범위를 보존한다.

## 이전 검증 — 2026-10-18 / SN-P04

비운영 pilot **4/4 완료**. 실제 go-wemix1350376a의 새 BGE-M3/native1024 held DB와 go-stablenet0bf2f4d1의 보관 DB에서 같은6진단 입력·검색·v2·6개념 비교·출처/혼입 경계를 검증했다. 기존 go-wemix 자료는 mock64이며 새 BGE 자료와 구분한다. 서로 바뀐 실제 graph/vector 설정2개는 기동 거부, 최종정상5+초기/보완전진단6세션은 EOF0·모든 소유 프로세스 회수·강제종료0이다. corpus/정책 범위가 다르며 매핑은 proposed라 품질 우열·운영망 합의·온톨로지 독립 효과를 승인한 결과가 아니다.

새 소스38dd5c54(Go1738개)에서 일반/race 각4097pass/18skip, build/fmt/vet/lint/boundaries/docs/build-bins 및 별도 비교/격리 race232개가 새로 통과했다. 이전 결과 재사용으로 대신하지 않았다. [최신 보고서](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/REPORT.txt)와 [현재 작업리스트](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/WORKLIST.json)가 최신 상태다.

다음은 독립 DEV/FINAL·완전 gold·matching/충분성·F01~08 동결 준비다. 남은 전체는 해당 준비, 정식 WI9개(0/9), 필수 온톨로지 독립 효과 평가, 별도 후속12개다. D04미정·freeze null/execution_allowed=false, 로컬 로드맵7/7·28/28·준비22/22 및 기존 제한 온톨로지 연구완료/inconclusive를 유지한다. 운영 current·전역 설정·외부 게시를 변경하지 않았다.

이하 이전 시점의 구현/연구 기록은 당시 범위이며 현재 상태는 위 최신 보고서·원장을 우선한다.

운영·독립 수용 준비 갱신(2026-10-16): source3a530a25 격리 빌드·CKS33개 스키마 일치/ChainBench81개 metadata·현재 Claude Code2.1.301의 로컬 scripted5개를 새로 검증했다. 실제 모델/노드0, 정식9개 수용0/9·F-01–08 pending이다.9개 baseline/표본/위치 매칭/분모 제안과 역할·입력·recipe의 남은 결정을 [현재 protocol](./OPERATIONAL-ACCEPTANCE-PROTOCOL.md)에 정리했다. 다음은 검토자·범위·독립 입력/gold 검토다.


2026-10-16 기준 지식 매니저·프로젝트 비교 로드맵은 **28/28 세부 작업·7/7 기능 완료**다. 사용자가 멈추라고 하기 전까지 남은 작업을 순서대로 구현·검증하고 단계별 서명 로컬 커밋으로 보존한다. 각 완료 단계마다 현재 단계·검증 결과·다음 작업·남은 전체 범위를 화면 로그와 문서에 기록한다. 세부 상태는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

## 현재 구현과 다음 단계

| 기능 | 상태 | 확인한 범위 |
|---|---|---|
| KM-01 프로젝트 격리 | 4/4 완료 | 실제4DB·동명/포크/버전·인용·공개 읽기/쓰기 거부·rollback/GC |
| KM-02 버전 DB 운영 | 4/4 완료 | 공개 build/status/cancel·영속 job/재시작·유지/실패 matrix |
| KM-03 Claude 매니저 | 4/4 완료 | 등록·문서 없는 수집/초안·검토/회수·실제 설치 CLI8회/68도구; 로컬 시험 응답기 |
| KM-04 지속 갱신 | 4/4 완료 | 실제 periodic/change2프로젝트·20항목·221호출·2EOF0·삭제/no-op·실패/연결 retry·타 DB/인용 보존 |
| KM-05 공통 개념/연결 | 4/4 완료 | 공통 어휘·원문 연결·직접 매핑 검토/회수·적용/지원·L1/L2/network/산업 차이 |
| KM-06 비교 근거 MCP | 4/4 완료 | 실제 go-stablenet/go-wemix4243파일·6개념40셀·22항목19호출·3EOF0·오매핑/범위/예산/원본 보존 |
| OE-01 온톨로지 효과 | 4/4 완료 | 512회 함수 과제+실제2프로젝트 매핑 입력 효과18항목/27호출·원문8셀/양측후보3개; off/inconclusive |

OE-01.3 완료: 고정512회 원자료에서 유한한 완전 함수 찾기 과제의 precision/recall·순위·의도한 ontology 개념/범위 진단·warm 지연·실제 로컬 API 호출/서버 token 수를 재계산했다.5사실군 평균 recall은 off54.44%→concept_text/combined56.67%, precision12.87%→13.54%, MRR0.4612→0.4862다. 개선은 EX-08 영어 메시지 처리 질문1개에서 prepare만 회수하던 결과에 commit이 추가된 것으로 한정된다. 나머지13개 채점 질문의 지표는 같았다. hit@10은68.33%로 동일하다. 일반 행동/정책의 semantic relevance나 답변 정확도로 확대하지 않는다.

각 arm의 warm80개 p95는 off583.1ms, concept_text736.2ms(1.263배), relations607.4ms(1.042배), combined697.4ms(1.196배)다. capture batch embedding API는 off/relations1584개·31536서버 prompt tokens, concept_text/combined1664개·56112tokens다. 준비/모델 확인도 포함한 도구 실행 batch 비용이며8호출×16질문의512개 독립 사실이 아니다. 토큰은 서버 보고값이고 금액/전력/하드웨어 비용은 추정하지 않았다. 유료/LLM 답변0이다. 활성 모드는 의도 개념13질문 중9개를 매칭했고, scope 의도 진단3질문 중 외부 go-wemix 대상 EX-16에도 WBFT quorum을 매칭했다. relations는 검토된 구현 연결0·rank 변화0이다. 정의 검색/combined의 최종 인용 순서는32짝 중18짝에서 달랐지만 함수 회수 이득은1질문뿐이다. 작은 source-derived 개발 표본과 지연 관찰로 기본 off·quality=inconclusive를 유지한다.

전체28/28·기능7/7의 문서화된 로컬 로드맵을 완료했다. 별도 정책 생성기의 명시적 소스 대조 재생성 보완도 완료했다. 다음 별도 단계는 운영·독립 수용 범위를 동결하는 일이다. 운영·독립 수용9개와 선택 후속12개는 별도 범위로 유지하며 미실행/미승인 사항을 완료로 바꾸지 않는다.

## 검증과 현재 제한

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. 새 Go 변경이 없음을1273개 Go와 모든 결속 입력의 hash로 확인해 직전 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 실제 corpus/MCP 시험과 과거 Go 실행을 구분한다. 유료 모델 호출0이며 ontology 효과/독립 품질 수용은 아니다. [검증 자료](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/manifest.json)를 따른다.

매니저 plugin0.6.0은 reviewed 기본18도구와 명시 updates6도구를 제공한다. 실제 Claude Code2.1.300과의 갱신 연결2회/36도구도 로컬 시험 응답기로 검증했다. 실제 모델 호출/비용0이며 agent 자율성·답변 품질은 측정하지 않았다. 개인 머신에 서비스나 OS scheduler를 설치하지 않았고 명시 foreground worker가 실행 중인 동안만 주기/변경 polling이 동작한다.

원래 독립 수용의 단계0·담당자·실제 모델/인증/비용 지정은 로컬 개발을 막는 요건이 아니다. 유료 모델·독립 수용·외부 게시에는 해당 실행 범위가 별도로 필요하다. 이번 지속 작업은 단계별 로컬 커밋 범위다. 푸시/PR·전역 설치·운영 current 변경을 완료로 추정하지 않는다.

## 보존할 결정과 품질 관찰

정책 참조 수정7건은 별도 평가 overlay로 사용자 채택 승인을 기록했다. [승인 영수증](../../system/eval/operational-preparation-2026-10-15/adopted-policy-user-approval/receipt.json)과 [적용 검증](./POLICY-OVERLAY-ADOPTION.md)을 따른다. 원본 정책·원래 승인 입력은 보존한다. 기존 무소스 sync의 동작을 보존하면서 명시적 --code-root의 선언 대조/오파일 거부와 재생성을 보완했다.

[OE-01 단일 프로젝트 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)은384회 실패/누락0·인용3,616개 원문 일치, 활성288회 no_match·순위 차이0이었다. 별도 정확 용어8회도 최종 boost0이다. 기본 off·quality=inconclusive를 유지한다. KM-04.4에서 단독 합성 대문자 식별자 질의가 비고 같은 자료가 자연어 질의에 나타난 사례도 OE-01 질의 해석/회수율 후보로 보존했다. 새 기능 구현이나 계획 승인을 독립 정답·품질 승인으로 바꾸지 않는다.

이전 단계의 상세 기록은 각 명세의 checkpoint 절과 서명 커밋·동결 증거 packet에 남긴다. 현재 상태는 이 문서 첫 표와 전용 원장을 우선한다.

## OE-01.4 실제 다중 프로젝트 매핑 효과 평가

동일한 실제 go-stablenet/go-wemix 보관 DB와6개 개념·40개 프로젝트/축 셀을 대상으로, binding 입력만 뺀 기준선과 고정된 제안 매핑7개를 비교했다. 양쪽 모두 include_proposed=true이며 기본 검토 필터는 별도 확인했다. 프로토콜을 호출 전에 저장하고 순서를 교대해2회 반복했다. 24개 행렬+describe2개+기본 필터1개=도구27호출, 검증18항목, 두 연결 EOF0이다. 반복80셀을 독립 사실80개로 세지 않고 arm별 고유40셀로 보고한다.

| 관측값 | 매핑 없는 기준선 | 제안 매핑 입력 |
|---|---:|---:|
| 고유 프로젝트/축 셀 | 40 | 40 |
| 원문 근거가 있는 셀 | 0 (0%) | 8 (20%) |
| 선언된 exact/similar/not_applicable 셀 | 0 | 7 (17.5%, 모두 similar) |
| unknown | 0 | 18 |
| not_collected | 40 | 15 |
| 검증한 셀 인용 | 0 | 9 |
| 양쪽 근거가 있는 비교 후보 | 0 | 3 |
| 검사한 프로젝트 그룹 / 혼입 | 12 / 0 | 12 / 0 |

9개 인용의 프로젝트·버전·source mode·원문/범위 hash·본문을 검증했다. 근거0개인 기준선의 source accuracy는 측정 불가(null)다. 처리군의 원문 일치100%는 hash/body 무결성 범위이며 의미 정확도100%가 아니다. 두 arm은 동일한 scope 격리 장치를 사용하므로 혼입0을 매핑만의 효과로 돌리지 않는다. finality 근거1셀은 unknown을 유지하며, staking/정책 미수집을 기능 부재로 단정하지 않는다. 기본 reviewed-only 요청에는 미검토 매핑이 나타나지 않는다. DB/source/config 전후 byte는 동일하다.

매핑은 평가자가 원문을 보고 선별한 제안 입력이다. 따라서 이 결과는 **연결 주석을 추가하면 비교 가능한 원문을 구조적으로 모을 수 있다**는 제한된 관찰이며, 온톨로지 어휘 자체의 인과 효과·독립 holdout·자연어 의도 해석·LLM 답변 품질·실제 네트워크 성능 평가가 아니다. 기존 단일 프로젝트 분석과 함께 기본 off·quality=inconclusive를 유지한다. 사람의 매핑/정답 승인과 정식 독립 수용은 기록하지 않았다.

[원자료와 검증](../../system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect/manifest.json)에 protocol·원시 요청/응답·소스·재계산/반례 시험을 보존한다. 로드맵 **28/28·기능7/7의 로컬 구현/명시된 개발 검증을 완료**했다. 정책 생성기 보완, 운영·독립 수용9개, 선택 후속12개는 별도 범위다. 유료 모델·운영 활성화·외부 게시 승인을 이 완료로 대신하지 않는다.

## 소스 대조 정책 생성기 보완 완료 (2026-10-16)

명시적 `cks domain sync --code-root <source>`는 폴더 이름 대신 실제 Go 선언을 읽어 package·receiver·type/field/const/var/interface 이름을 해당 anchor 파일에서 대조한다. receiver 없는 이름은 해당 파일에서 유일할 때만 보완하고, 오파일/모호한 이름/외부·symlink 경로/손상된 소스는 두 출력을 쓰기 전에 거부한다. symbol이 없는 파일/행 전용 anchor는 새 정책 연결로 만들지 않는다. 기존 무소스 sync는 이전 결과를 보존한다. 이 경로는 명시적 옵션이며 기존 setup/운영 profile을 자동으로 전환하지 않는다.

고정 소스0bf2f4d1에서 원래 ErrInvalidFeePayer 파일 참조는 거부되고 두 기존 출력은 보존됐다. 소유 평가 사본의 해당 anchor 파일만 core/txpool/errors.go로 수정한 후, 승인 overlay와 같은42개 정책·146개 연결을 재생성했다. 생성기는 참조를 결정적으로 정렬하므로 승인 overlay의 기존 목록 순서와 byte는 일부 다르지만, 모든 정책 필드와 정렬한 governs 목록은 같다. 원본 entry·정책·승인 overlay와 원본 소스 HEAD/status를 보존했다. 원래 entry의 잘못된 파일 위치를 조용히 찾아 옮기거나 원본 설명/검토 승인을 변경하지 않았다.

새 생성 결과로 실제 graph를 구축하고146개 연결을 고정 AST 위치와 대조했다. 수정7개에 대해 공개 CKS 일반·소스/snapshot pin·canonical ID 조회21회, 잘못된 pin2개·assembler 혼입1개 거부/빈 결과까지 총24회 도구 조회와26쌍 원시 요청/응답을 검증했다. CKS는 EOF0으로 종료했다.843개 파일이 graph에 선택됐고 parse error0이다. source corpus는882개 비시험 Git Go blob이며 대상 프로젝트 코드를 실행하거나 실제 LLM/유료 모델을 호출하지 않았다.

함수뿐 아니라 실제 정책의 구조체 필드·인터페이스도 확인했다. 초기 진단의 필드 누락과 정책 연결을 요청하지 않는 loc anchor 처리, CLI 오류 정제 정책에 대한 평가 도구의 가정을 수정한 이력은 최종 통과와 구분해 보존한다. 원자료 재계산/반례·새 전체 Go/race·빌드/fmt/vet/lint·문서 검증은 [증거](../../system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration/manifest.json)를 따른다. 이는 source-name 연결 정합성의 개발 검증이며 정책 의미의 사람/독립 수용이나 운영 채택이 아니다.

현재 로컬 로드맵28/28·기능7/7과 별도 생성기 보완을 완료했다. 남은 별도 범위는 운영·독립 수용9개와 선택 후속12개다. 다음은 독립 평가를 실제로 수행할 경우 해당 입력·검토/실행 범위·필요한 실제 모델 조건을 고정하는 단계이며, 로컬 검증을 그 승인/수용으로 대신하지 않는다.
