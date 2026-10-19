<!-- TICKET-RPC-CURRENT -->
# 단계64 — WI-V1/V2 실제 in-process RPC 후보 경로 / 2026-10-19

[최초 명세](./operational-preparation/source-review-supplement/followups/ticket-actual-rpc-v1/SCOPE.json) · [명시적 보정 계획](./operational-preparation/source-review-supplement/followups/ticket-actual-rpc-v1/RECOVERY-SCOPE.json) · [결과](./operational-preparation/source-review-supplement/followups/ticket-actual-rpc-v1/SUMMARY.json) · [검증 근거](./operational-preparation/source-review-supplement/followups/ticket-actual-rpc-v1/validation/SUMMARY.json) · [우선순위·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/ticket-actual-rpc-v1/WORKLIST.json).

**AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다. D04는 미정이며 실명·이메일·연락처를 요구하거나 승인 기록을 대신 만들지 않는다.

| 이번 기술 범위 | 확인한 결과와 한계 |
|---|---|
| 후보·소스 | go-stablenet `0bf2f4d1`의 추적2228개를 기준으로 기존 full-file overlay만 적용. 소유 checkout5개, 역할5개·메서드4종·프로젝트 tree4종. 실제 Git 프로젝트 tree와 시험 파일을 포함한 build tree를 따로 재계산하고 소스·fixture·binary hash를 결속. 후보 commit은 만들지 않음 |
| 최초90건 | 일반45+race1 완료 관측, race3 부분 실행, race41 미진입. 외부25초 한도에 패키지 초기화가 포함됨. 소유 OS 스택2개에서 기존 test init/newTestBlockchain/InsertChain/race 계측 확인. 실패·부분 원문을 삭제하거나 전체 통과로 표시하지 않음 |
| 보정44건 | 첫45건 계획은 이미 완료된1건 때문에, 다음44건 계획은 부분 trace3건 때문에 사전 검사에서 각각 exit1·대상 실행0으로 거부됨. 두 기록을 보존. 최종 고정44건은 원래 시험 파일·소스·같은 race binary를 유지하고 외부 한도만150초로 보정. test20초·관찰2초 유지. 모두 완료 관측 |
| 전체 집계 | 134시도 중 유효 완료 관측90·초기 부분3·미진입41. 실제 RPC465호출(완료450+부분15), 직접 notifier 대조93개(완료90+부분3). 초기 실패와 보정 재시도를 합산하며 자동 숨은 재시도0 |
| 실제 RPC | 소유 `eth` 등록과 `rpc.DialInProc/net.Pipe`에서 실제 Syncing 호출. 알 수 없는 구독 오류, 다른 세션의 같은 ID 해지 거부, 올바른 세션 true, 반복 해지 거부를 각각 확인. 직접 no-notifier 호출은 RPC로 세지 않음 |
| 분기·정리 | 실제 eventLoop 정상30건과 제어 백엔드 완료60건을 분리. 백엔드 ack 보류 시 RPC true와 deferred Unsubscribe 대기를 구분. 상태 송신을 먼저 제시하면 A/B만 요청 수신 전 drain하며 원본/재도입/조기반환은 관찰 뒤 cleanup에서 송신자를 해제. 완료90건은 콜백·sender·loop 종료를 확인. 초기 부분3건의 내부 cleanup은 미증명이며 모든134건 OS process group만 reaped·잔존0으로 확인 |
| 수용 한계 | 같은 노출 DEV 티켓1개의 상관된3일정이다. 전체 Ethereum factory·progress 알림·원래60초 timer 발화·원래5일정·12요건 전체·설치4역할/finalHEAD·독립 oracle/수용 가능 후보 집합은 미완료. 조기반환 후보의 backend membership도 추론하지 않음 |
| 경계·회귀 | 외부 listener·운영 노드·글로벌 설치/설정·게시·유료/외부LLM0. credential 환경변수 상속 없이 기존 캐시로 오프라인 빌드. 새 대상 binary10개와 판정기39·공통64·문서20·notice3 =126개 회귀 검사 통과. 원자료1057개 파일 동일 재생 확인. 현재 문서 검증 결과는 연결된 검증 근거를 따른다. CKS native1738·승인5·설정2 동일: 이전 일반/race 각4158통과·18skip 및 build/fmt/lint 근거를 재사용하며 새 CKS 전체 실행으로 표시하지 않음 |

[보고서](./operational-preparation/source-review-supplement/followups/ticket-actual-rpc-v1/REPORT.txt)를 따른다. 남은 목록/우선순위 삭제나 제한된 관측의 정식 완료 승격은 재봉인해도 거부한다. 초기 판정기 회귀의 예외 형식 오류1개와 공통 검사 중단130도 검증 근거에 보존하고, 명시적 종료 신호 거부를 보강한 뒤126개를 재검증했다. 서명 저장은 실제 Git과 `--require-stored`로 확인한다.

다음 우선순위는 **WI-V1/V2 알림·원래 일정·12요건 및 오라클/설치역할 → WI-6 실제 검색 쌍·관련성/잡음·warm → WI-SP·WI-C**다. 남은 전체는 위 작업 외 WI-1의53발생/3블록·값/타입/unknown/메모리·독립 의미, WI-2 독립 표본/정답·호출자/build profile/지원/MCP/설치역할, WI-3 독립 가족·노출/중복/정답·기본 예산/시작SLA/site거부/부분·공급자오류·범위밖, WI-4 전체 드라이버/host/수신자/시간/반복·독립 가족/오라클/충분성, 익명 역할/독립성·새 DEV/FINAL·완전정답/matching/충분성·F01~08, 정식9개 수용0/9, 온톨로지 독립정답/구현·개념연결/검색효과다. 실제 LLM 답변 평가는 모델·인증·비용상한 결정 후 진행한다. 조직 인증/권한·공개CLI/HTTP/graph export·graph점수/확장·domain flow gold·대규모corpus/긴함수·PR/as-of·legacy출처·CoreML·embedding cache·운영감사·viewer·배포/downstream의 선택12개는 별도 백로그다.
<!-- END-TICKET-RPC-CURRENT -->

이하 단계63 및 이전은 당시 기록이다.

<!-- WI4-SCHEDULES-CURRENT -->
# 단계63 — WI-4 WORK·GOSSIP·POOL 실제 소유 DEV 경로 / 2026-10-19

[사전 명세](./operational-preparation/source-review-supplement/followups/wi4-target-schedules-v1/SCOPE.json) · [GOSSIP 보완 계획](./operational-preparation/source-review-supplement/followups/wi4-target-schedules-v1/GOSSIP-CORRECTIVE-SCOPE.json) · [결과](./operational-preparation/source-review-supplement/followups/wi4-target-schedules-v1/SUMMARY.json) · [검증 근거](./operational-preparation/source-review-supplement/followups/wi4-target-schedules-v1/validation/SUMMARY.json) · [현재·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi4-target-schedules-v1/WORKLIST.json).

**AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다. D04는 미정이며 실명·이메일·연락처를 요청하지 않는다.

| 이번 기술 범위 | 결과와 한계 |
|---|---|
| 소스·실행 | go-stablenet `0bf2f4d1` 추적2228개 보존. 시험 파일만 추가·지정 Go1.23.12 일반/race binary 빌드. 최초72+보완24=96실행 모두 보존하며 유효 관측72·초기 구성 실패24를 구분 |
| WORK24 | 실제 newWorkLoop→Origin을 Anzeon nil로 선택. 실제 pendingMu 대기 장벽에서 송신·exit 준비를 정한 후 해제. 양쪽 준비 시 두 결과 모두 기록하고 유한 backlog 처리만 관측. 정상 대조는 빈 버퍼이며 unbuffered 수신자·무한 공정성·운영 종료SLA 증거가 아님 |
| GOSSIP24 유효 | 실제 Broadcast의 validator membership→Gossip/cache와 소유 self-event mux 사용. 가짜 peer의 성공·오류·지연·같은 hash 중복과 다른 hash 대조를 구분. nil 반환·가짜 성공을 네트워크 전달로 세지 않음 |
| POOL24 | 실제 New/Init/Reset/reorg/reinject/SubscribeTransactions/Stats, 소유 임시 blob 저장소·새 시험 키·제어 chain 사용. 성공 StateAt와 비어 있지 않은 재주입1개 확인. 수신 후Stats·빈 버퍼 정상군, Stats 후수신 제어 구독자의 실제 Send/lock 대기2초, 소유 unsubscribe의 대기 해제 검증. 운영 소비자의 같은 순서 도달성은 미검증 |
| 실패 보존 | 첫 오프라인 빌드는 캐시 경로 누락으로 시험 전에 실패하여 원문·입력을 보존. 기존 로컬 캐시를 명시. 초기 GOSSIP24는 시험 Backend의 Engine 누락으로 송신 전 exit2. panic cleanup의 t.Failed=false를 통과 근거로 쓰지 않음. 별도24계획에서 실제 NewEngine을 초기화하고 엔진/노드는 시작하지 않음 |
| 경계·정리 | 외부 peer/socket·운영 노드 제어·시스템 시각·글로벌 설정·게시·유료모델0. credential 환경변수 상속 없이 오프라인 빌드. 관측과 cleanup/rescue를 분리하고 모든 소유 process group·helper·store 정리 확인. unsubscribe 사례의 해제는 명세에 정한 시험 단계 |
| 범위·회귀 | 이전 SUB5·UDP4와 이번12로 기존21개 ID 모두에 제한된 DEV 실행 근거가 있음. 전체 원래 드라이버·운영 host·독립 수용은0. 관리 CKS native1738·승인5·설정2개 동일하며 이전 일반/race 각4158통과·18skip 근거를 재사용 |

[보고서](./operational-preparation/source-review-supplement/followups/wi4-target-schedules-v1/REPORT.txt)를 따른다. 관측을 독립 가족/정답/seed/동결 arm·제품 결함·품질 점수로 승격하지 않는다. 회귀106개(이번20·공통63·문서20·notice3)·문서134개·원자료709개 파일 동일 재생을 통과했다. 서명 저장은 실제 Git과 `--require-stored`로 확인한다.

다음: **WI-V1/V2 실제API·독립 티켓·오라클·수용 가능 후보 집합의 부채 점검**. 남은 전체: WI1의53발생/3블록·타입/값/메모리·독립 의미, WI2 독립 의미/build profile/지원/설치역할, WI3 독립 의미/새가족·기본예산/시작SLA/site거부 원인, WI4 전체 진입·수신자·시간·독립 계약·가족/오라클/충분성, WI-V1/V2 실제API/독립티켓/수용후보, WI6 nonseed/기본·확장쌍/관련성·잡음/warm, WI-SP 실제overlay/ACL/출처, WI-C 필수항목/우선순위/보고서·그림/전달/설치역할, 새독립DEV/FINAL·완전정답·matching·충분성·F01~08, 정식9개, 온톨로지 독립정답/구현·개념연결/검색효과다. 실제LLM답변 평가는 모델·인증·비용상한 결정 후 진행하며 선택 백로그12개는 별도다.
<!-- END-WI4-SCHEDULES-CURRENT -->

이하 단계62 이전은 당시 기록이다.

<!-- WI4-UDP-CURRENT -->
# 단계62 — WI-4 실제 UDP 등록·응답·합성 deadline 경로 / 2026-10-19

[사전 명세](./operational-preparation/source-review-supplement/followups/wi4-target-udp-v1/SCOPE.json) · [결과](./operational-preparation/source-review-supplement/followups/wi4-target-udp-v1/SUMMARY.json) · [검증 근거](./operational-preparation/source-review-supplement/followups/wi4-target-udp-v1/validation/SUMMARY.json) · [현재·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi4-target-udp-v1/WORKLIST.json).

**AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. D04는 미정이며 실명·이메일·연락처를 요청하지 않는다. 실제 검토·승인을 대신 기록하지 않는다.

| 이번 기술 범위 | 확인한 결과와 한계 |
|---|---|
| 대상·보존 | go-stablenet `0bf2f4d1`, 추적 소스2228개 보존·시험 파일1개만 추가. 지정 Go1.23.12 일반/race binary를 별도로 새 빌드 |
| 고정24회 | 일반/race 각3반복·기존4사례, 누락·대상 재시험0. 실제 pending 등록 뒤 native500ms timeout2개를 받은6건, 한 handleReply 응답 이벤트를 관측한6건, 합성 deadline 주입12건을 분리 |
| 응답 경로 | 같은 조건 matcher2개 중 첫 callback만 실행·nil 완료, 둘째는 관측 확정 때 pending. response arm을 명시했으며 timeout 대체 분기까지 검증했다고 주장하지 않음 |
| 합성 경로 | 실제 loop 안의 gate callback에서 deadline을 주입한 뒤 gate를 제거하여2원소 상태 노출. 먼 미래 head의 errClockWarp와 tail의2초 pending을 cleanup 전에 확인. 정상 monotonic Now.Add로 이 상태에 도달했다는 증거가 아니며 timer.Stop 내부 상태를 직접 관측한 것으로 세지 않음 |
| 한계 | 최소 소유 채널/context로 실제 pending/handleReply/loop를 실행. ListenV4·테이블·socket·운영/설치 host 진입 증거는0. near의 단일 resetTimeout 호출 횟수와 전체 원래 드라이버 수용은0 |
| 실행 경계 | 외부 네트워크·시스템 시각·운영 노드·글로벌 설정·게시·유료모델0. upstream 시험은 credential 환경변수를 상속하지 않고 의존성 다운로드를 차단. 소유 loop/process group 종료와 임시 자원 정리 확인 |
| 최초 환경 실패 | sandbox의 ps 조회 거부는 로컬 restore 후·대상 빌드/시험 전 발생. 원문과 하위 명령 종료 코드 미저장 한계를 보존. 종료 감사를 생략하지 않고 같은 계획을 별도 소유 공간에서 한 번 실행. 보안 위반 판정으로 기록하지 않음 |
| CKS 회귀 재사용 | 관리 native1738개·승인5개·lint 설정2개는 이전 결속과 동일. 일반/race 각4158통과·18skip 근거 재사용이며 이번 새 전체 실행으로 표시하지 않음 |

[보고서](./operational-preparation/source-review-supplement/followups/wi4-target-udp-v1/REPORT.txt)를 따른다. 새 독립 가족/정답/seed/동결 arm·품질 점수는0이며 제품 결함 확정이 아니다. 현재 회귀105개(이번20·공통62·문서20·notice3), 문서134개와 원자료177개 파일 동일 재생을 통과했다. 서명 저장 여부는 실제 Git과 `--require-stored`로 검증한다.

다음: **WI-4 WORK·GOSSIP·POOL 나머지12개 사례의 실제 진입·스케줄·정리 검증**. 남은 전체: WI1의53발생/3블록·타입/값/메모리·독립 의미, WI2 독립 의미/build profile/지원/설치역할, WI3 독립 의미/새가족·기본예산/시작SLA/site거부 원인, WI4 남은12개와 SUB/UDP의 전체 진입·시간·독립 계약·충분성, WI-V1/V2 실제API/독립티켓/수용후보, WI6 nonseed/기본·확장쌍/관련성·잡음/warm, WI-SP 실제overlay/ACL/출처, WI-C 필수항목/우선순위/보고서·그림/전달/설치역할, 새독립DEV/FINAL·완전정답·matching·충분성·F01~08, 정식9개, 온톨로지 독립정답/구현·개념연결/검색효과다. 실제LLM답변 평가는 모델·인증·비용상한 결정 후 진행하며 선택 백로그12개는 별도다.
<!-- END-WI4-UDP-CURRENT -->

이하 단계61 이전은 당시 기록이다.

<!-- WI4-TARGET-CURRENT -->
# 단계61 — WI-4 실제 구독 경로·수신자 종료·반복 관측 / 2026-10-19

[명세](./operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/SCOPE.json) · [보완 명세](./operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/CORRECTIVE-SCOPE.json) · [수신자 종료 명세](./operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/FULL-RECEIVER-SCOPE.json) · [결과](./operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/SUMMARY.json) · [현재·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/WORKLIST.json).

**AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다. D04는 미정이며 실명·이메일·연락처를 요청하지 않는다. 실제 검토 없이 승인 기록을 만들지 않는다.

| 이번 기술 범위 | 확인한 결과와 한계 |
|---|---|
| 실제 대상 | go-stablenet `0bf2f4d1`의 추적 소스2228개와 원래7개 소스 hash 보존. 프로젝트 지정 Go1.23.12 일반/race binary를 각각 새로 빌드하고 시험 파일만 추가 |
| 48회 모두 보존 | 원래 소스 조건 관측30건, 추가3호출 스트레스6건, 처음부터 수신자가 없는 변형3건, 관측기 실패9건. 실패를 통과로 바꿔 쓰지 않음 |
| 실패 원인·보완 | 첫6건은 잘못된 소스 줄126/188을 관측. 이후race3건은 compiler closure 이름 차이로 관측 확정 실패. 실제 send 줄125/186과 같은 goroutine의 method scope를 확인하도록 보완. 단일 수신자의 실제 종료·join을 trigger 전에 확인한 최종6회는 일반/race 각3회 통과 |
| 경로 구분 | 원래 조건30건 중 실제 constructor/eventLoop 경로12건, 실제 Unsubscribe에 제어 수신자를 연결한 경로18건. 제어 수신자3종을 전체 공개API 경로 검증으로 확대하지 않음 |
| 시간 계약 | 실제60초 타이머와 준비 상한75초 뒤2초 관측을 분리. cleanup/rescue 전에 관측 확정. fresh-start부터2초 안에 도달했다는 주장이 아니며 전체 원래 드라이버 시간 계약 수용은0건 |
| 정리·보존 | 모든 소유 process group 종료·임시 자원 정리. 운영 노드·글로벌 설치·게시·모델 호출0. 반복을 새 독립 가족/seed/동결 baseline-fix arm으로 세지 않음 |
| 기존 CKS 회귀 | 관리 Go 입력1738개·승인5개·lint 설정2개는 단계60과 동일. 일반/race 각4158통과·18skip 근거를 재사용하며 새 전체 실행으로 기록하지 않음. 이번 upstream 시험 binary와 구분 |

[보고서](./operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/REPORT.txt)와 [검증 요약](./operational-preparation/source-review-supplement/followups/wi4-target-subscription-v1/validation/SUMMARY.json)을 따른다. 현재 회귀104개(이번20·공통61·문서20·notice3)와 문서134개 통과, 원자료444개 파일 동일 재생을 확인했다. 서명 저장 여부는 실제 Git 커밋과 `--require-stored` 검증으로 확인한다. AI 어휘와 실제 조건 관측은 독립 gold·제품 결함 판정·품질 점수가 아니다.

다음: **WI-4 나머지16스케줄의 실제 경로·스케줄·반복과 준비/2초 시간 계약을 점검**한다. 남은 전체: WI1의53발생/3블록·타입/값/메모리·독립 의미, WI2 독립 의미/build profile/지원/설치역할, WI3 독립 의미/새가족·기본예산/시작SLA/site거부 원인, WI4 남은16개/전체API·제어 수신자 범위/시간 계약/독립 가족·오라클·충분성, WI-V1/V2 실제API/독립티켓/수용후보, WI6 nonseed/기본·확장쌍/관련성·잡음/warm, WI-SP 실제overlay/ACL/출처, WI-C 필수항목/우선순위/보고서·그림/전달/설치역할, 새독립DEV/FINAL·완전정답·matching·충분성·F01~08, 정식9개, 온톨로지 독립정답/구현·개념연결/검색효과다. 실제LLM답변 평가는 모델·인증·비용상한 결정 후 진행하며 선택 백로그12개는 별도다.
<!-- END-WI4-TARGET-CURRENT -->

이하 단계60 이전은 당시 기록이다.

<!-- WI3-WALK-CURRENT -->
# 단계60 — WI-3 출처 연결·총 예산·요청 계측 / 2026-10-19

[보고서](./operational-preparation/source-review-supplement/followups/wi3-walk-budget-v1/REPORT.txt) · [사전 명세](./operational-preparation/source-review-supplement/followups/wi3-walk-budget-v1/SCOPE.json) · [검증 근거](./operational-preparation/source-review-supplement/followups/wi3-walk-budget-v1/acceptance-review.json) · [현재·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-walk-budget-v1/WORKLIST.json). AI 초안6/6 준비, **독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

실명·이메일·연락처는 요청하지 않는다. 검토 기록은 익명 역할 ID와 근거 hash·결정·독립성으로 구분한다. D04미정은 개인정보 제출 대기가 아니며, 로컬 준비·기술 검증을 중단하는 조건이 아니다. 실제 검토·승인이 없는데 승인으로 기록하지 않는다.

| 이번 기술 범위 | 확인한 결과와 한계 |
|---|---|
| 출처 연결 | 원래 검색 인용·순서·점수는 보존하고, snapshot에 결속된 resolver가 확인한 정확한 선언 identity만 그래프 간선·멤버십에 사용. 포함·겹침·동명만으로 연결하지 않음. 캐시의 성공/실패와 원래 행 유지 |
| 예산·오류 | 기존 총 시간과 부모 deadline 안에서 최종 확인 시간을10%(최대100ms) 예약. 예산/취소/실제 snapshot변경·증거 크기/정책 거부를 구분하고 불명 공급자 원문을 노출하지 않음. identity 의존 영향/동시성 신호는 예산과 공급자 오류를 seed 부재로 오인하지 않음 |
| 실제 stdio | 이전62/새62=124호출 모두 보존, 세션4개 EOF0. 제어4개와 기존20입력 모두 계측on/off 결과가 경과 시간만 제외하고 일치. 넓은 주석 포함 인용과 선언 인용의 확장 집합 같음. 주석줄·미존재 위치·잘못된pin·상한 등 나머지7개 제어는 구조화 결과 없이 거부 |
| 요청 계측 | 새 요청 summary31개·논리 호출629회·물리 로컬 모델 HTTP63회, pending0. 논리 호출수와 응답 budget.calls 일치. 원문 query/심볼/오류를 호출 계측에 저장하지 않음. 내부 추론 횟수·품질 점수로 바꾸지 않음 |
| 전체 새 검증 | 일반/race 각4158pass·기존18skip·실패0. 이전4123개 통과ID 누락없음, 새35개 parent/subtest 확인. build·fmt·vet·고정lint2.14.0·경계·문서134개 통과. c18821b5+7Go파일 작업 트리/1738native·승인5와 설정2개 hash결속. 실제 호출 binary와 최종 검증 binary bytes 동일 |
| 실패 보존 | 첫 소유 시험은 구체적인 예산 분류 반례로 종료/정리를 확인. 다음 전체 일반/race 통과 뒤lint QF1003 실패를 별도 보존하고 테스트 문법만 수정하여 전체를 다시 검증. 과거 실패를 현재 통과로 바꿔 쓰지 않음 |

요청 검증·회귀 결과는 [검증 요약](./operational-preparation/source-review-supplement/followups/wi3-walk-budget-v1/validation/SUMMARY.json)과 원자료를 따른다. 새 독립 가족·운영/설치역할·게시·유료/외부LLM 실행은0이며, 기본 예산의 일반적 지연SLA·독립 검색 품질·온톨로지 효과를 수용한 결과가 아니다. site resolver의 일부 거부는 기존 일반 seed_unresolved 계약을 유지하므로 세부 원인 구분은 남은 WI3 검토에 포함한다.

기술 범위 저장 완료: 서명 커밋 `ad944ac6`의 실제 증거/source-binding을 연결했고 완료 guard와 저장 검증을 통과했다. W09마감은 [별도 결속 기록](./operational-preparation/source-review-supplement/followups/wi3-walk-budget-v1-finalization.json)을 따른다. 기존 작업 상태는 보존했다.

다음: WI-4 실제 도달성·스케줄·반복 실행의 경로와 관측 근거를 검증한다. 남은 전체: WI1의53발생지점/3블록·타입/값/메모리·독립 의미, WI2 독립 의미/build profile/지원/설치역할, WI3 독립 의미/새가족·기본예산/startup/site거부 원인, WI4 도달성/스케줄/반복, WI-V1/V2 실제API/독립티켓/오라클/수용후보, WI6 nonseed/기본·확장쌍/관련성·잡음/warm, WI-SP 실제overlay/ACL/출처, WI-C 필수항목/우선순위/보고서·그림/전달/설치역할, 새독립DEV/FINAL·완전정답·matching·충분성·F01~08, 정식9개, 온톨로지 독립정답/구현·개념연결/검색효과다. 실제LLM답변 평가는 모델·인증·비용상한 결정 후 진행하며 선택 백로그12개는 별도다.
<!-- END-WI3-WALK-CURRENT -->

이하 단계59 이전은 당시 기록이다.

# 단계59 — WI-3 계측 시 출처 식별 기능 보존 / 2026-10-19

[단계59 보고서](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/REPORT.txt), [명세와 검증 근거](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/acceptance-review.json), [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않는다.

`logging.measure_backend_calls`를 켜면 래퍼가 선택적 CitationResolver를 숨겨 실제 선언 위치 입력이 거부되고 후보 심볼이 사라졌다. 수정 전 어댑터/실제 MCP handler 회귀가 실패했고, 수정 후 관련332개가 통과했다. 래퍼는 공급자가 지원할 때만 그 기능을 보존하고 문맥·인용·반환값·오류를 그대로 전달한다. 기존 계측 scope 안에서는 입력 hash/길이와 안전한 오류 종류만 기록하며 원문 경로·심볼·오류 문구를 기록하지 않는다. 지원하지 않는 공급자에 기능을 만들어 붙이지 않는다.

전체 빌드·일반/race 각4123통과/기존18skip/실패0, fmt·vet·고정 lint2.14.0·경계·문서134개 검증을 새로 수행했다. 소유 복사본의30e43a34 +4Go파일 작업 트리 변경을1738native/5승인 입력과 결속했고 lint/docs 설정2개도 실행 전후 동일함을 확인했다. 최종 커밋 clean build로 이름을 바꿔 기록하지 않는다. [native 원자료](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/native/receipts.json)를 따른다.

실제 이전/새 CKS stdio 비교에서 선언 위치·심볼·보존 인용 확장3경로는 새 계측 on/off 결과가 경과 시간만 제외하고 일치하며, off의 이전/새 결과도 같다. 첫14회 시도의 주석 줄257 오류를 보존했고 선언 줄258·주석 줄을 분리한 이전16회/새16회를 추가했다. 총46호출·소유 세션6개 EOF0, 기존 source/graph/vector 전후 동일이다. 잘못된 commit·dataset·hash, 주석/없는 위치는 구조화 결과 없이 거부한다. 5000ms 상한의 기능 회귀이며 기본1초·warm 지연 수용이나 새 독립 표본으로 세지 않는다.

출처 결속 마감 완료: 기능·검증 근거를 서명 커밋 `d692db85`에 저장하고, 그 실제 커밋의 source-binding을 현재 소스 참조로 연결한 뒤 완료 주장 guard를 통과했다. 기존 작업 상태는 모두 보존했다. [결속 마감 기록](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1-finalization.json)을 따른다. native 빌드 모드는30e43a34 +4파일 작업 트리 검증이며, 그 전체 입력 hash가 현재 커밋과 같다는 뜻이다. 자동 label_suspects 계측 scope와 unanchored traversal 정규화는 이번에 추가하지 않았다.

다음: WI-3의 주석 포함 인용/정확한 선언 identity와 최종 검증 예산 계약. 남은 전체: WI-1의53발생 지점/3블록·타입/값/메모리 의미, WI-2의 독립 의미/build profile/지원/설치역할, WI-3 traversal/예산/startup/전체 호출 관측, WI-4/V1/V2 실제경로·오라클, WI-6 nonseed/쌍/warm, WI-SP overlay/ACL, WI-C 필수항목/전달, 새 독립 DEV/FINAL·완전 정답·매칭·충분성·F01~08, 정식9개, 온톨로지 효과 평가, 별도 선택 백로그12개다. 실제 LLM 답변 평가는 모델·인증·상한 결정 후 진행한다.

현재 Python 회귀96개(resolver14·공통 수용59·문서20·notice3), 문서134개·생성91파일 동일 재생·Python8파일 구문·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/validation/SUMMARY.json)은 새 native4123/race4123·기존 skip18·실제46호출/EOF6과 커밋 후 출처 결속 필요를 명시한다.

이하 단계58 이전은 당시 기록이다.

# 단계58 — WI-1 select 수신 정의 보정 / 2026-10-19

[단계58 보고서](./operational-preparation/source-review-supplement/followups/wi1-select-review-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi1-select-review-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처를 요구하지 않는다. 익명 역할 ID는 검토 여부와 근거를 구분하기 위한 선택지이며, 개인정보 제출 대기가 아니다.

원래30함수·1,654쌍·소스·과거 정답/실제 응답은 보존하고 별도 보완 reference만 생성했다. 고정 CFG가 select 진입의 전체 수신 대입과 선택 분기의 LHS를 모두 담아 원래 관측기가 정의 위치를 미확정으로 남겼다. 새 관측기는 해당 CommClause의 SelectCaseBody에 있는 정확한 AST identifier만 연결한다. 6함수의16정의 위치를 보정하여37쌍 기록이 달라졌다: 구조 unknown→도달32, 도달→비도달2, 도달 유지·이유 변경3. SRC-027의 이전 반복→다음 guard/log 연결2개는 새 수신 정의가 차단한다.

8개 완전한 합성 문맥은 타입 검사에 성공했고, 기존 변수 대입·선언·shadow·send/default·다중 case·closure·comma-ok를 구분했다. CFG에 없는 두 번째 수신 LHS는 unknown으로 남긴다. 실제 컴파일한 단일 합성 witness는 두 번의 수신 전 값3/8, 보정 후5/8을 관찰했다. 프로젝트 전체 타입·모든 스케줄·새 독립 표본의 증거로 확대하지 않는다.

원래30개 실제 MCP 응답을 **오프라인 재매칭**했을 때 구조 차이는0개다. 새 native 호출이나 엔진 수정은 없다. 발생 지점53개와 빈 블록3개는 여전히 매칭 미확정이고, 소비자의 일부 정의 위치가 수신 분기와 다르므로 매칭을 느슨하게 하지 않았다. 값 의미의 unknown1427/positive212/negative15는 그대로이며, 구조 관계만 positive1459/negative195다. 독립 정답·정확도 점수로 사용하지 않는다. 유지 Go 소스가 같아738c4a51의 최신 native 결속·일반/race 각4107통과/18skip을 재사용한다.

다음: WI-3 계측 래퍼의 선택적 CitationResolver 보존과 unanchored traversal 계약을 점검한다. 남은 전체: WI-1~3 의미/부분응답/실제설치경로, WI-4/V1/V2 실제경로·오라클, WI-6 nonseed/쌍/warm, WI-SP overlay/ACL, WI-C 필수항목/전달, 새 독립 DEV/FINAL·완전 정답·매칭·표본 충분성·F01~08, 정식9개, 온톨로지 효과 평가, 별도 선택 백로그12개다. 실제 LLM 답변 평가는 모델·인증·비용 상한 결정 후 진행한다.

현재 회귀91개(WI1 select10·공통 수용58·문서20·notice3), 문서134개·생성15파일 동일 재생·Python7파일 구문·보완 Go helper fmt·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi1-select-review-v1/validation/SUMMARY.json)은 새 소비자 호출0과 기존 source/native 결과 재사용을 명시한다.

이하 단계57 이전은 당시 기록이다.

# 지식 매니저·프로젝트 비교 전체 작업·시험표

## 2026-10-19 단계57: WI2 호출 계약·별칭·opaque helper·index 범위 점검

[단계57 보고서](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않는다.

| 점검 범위 | 확인한 내용과 한계 |
|---|---|
| 차이6건 | 지역 selector 가림2·renamed import2·opaque helper2로 분류. v1은 명시적으로 syntax_only_unverified/not_proven/lexical_may_reach이며 실제 stdlib target/helper 효과 검증이 아님. 원래 의미 negative/positive/unknown과4반환 차이·2미반환 AI양성 유지 |
| 컴파일러 보완 | 원래6F 본문 bytes를 그대로 둔 전체 파일 문맥8개는 타입 검사 통과. 지역 method와 stdlib 별칭 확인. 두 opaque 사례는 F hash가 같아도 helper가 identity/constant일 수 있음을 별도 문맥으로 기록. 본문만으로 helper 효과를 확정하지 않음 |
| 원래 prefix | 선택6개의 기존 prefix는 standalone 프로그램으로 타입 검사 오류가 있음. 새 Fake methods/helper body/import-use prefix는 보완 증거이며 원래 입력이나 retained blob의 묵시적 대체가 아님 |
| 심볼9개/입력17개 | 기록된 graph manifest에 없는4파일: build/ci.go·build/update-license.go·crypto/signify/signify_fuzz.go·metrics/disk_linux.go. 현재16개 MatchFile 관측은 none/gofuzz tag·Linux 파일 선택과 연결. 당시 정확한 GOOS/GOARCH/tags는 미기록이라 배타적 역사 원인은unknown |
| 실제 MCP8회 | 동일 입력·pin·상한의 제어6개 후보 수1/1/0/1/1/0 유지. 과거 최종 출처 확인 deadline2개는 이번 동일1초 예산에서partial analysis/matched_bounded empty rules 반환. 과거 실패 보존·코드 수정/안전/독립 표본·점수로 승격하지 않음 |
| 보존·native | 원래108입력과 정답 분모 유지. 기존 project·controlled graph 전후 동일, 소유 stdio2개 EOF0. 관리 Go 코드 변경0이며738c4a51의1738native/5승인 입력 결속·일반/race4107검증 재사용. 외부/유료LLM·설치역할·운영/게시0 |

[컴파일러 문맥](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/compiler-scope-review.json), [manifest/profile 범위](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/index-scope-review.json), [실제 계약 관측](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/actual-contract-review.json)을 따른다. 현재 규칙팩의 계약 일치와 실제 의미 정확도를 분리한다. 새 pack·지원범위 확대·임계치 변경·정답 수정은 수행하지 않았다.

다음은 **WI1 select 수신 범위의 reference 반례·WI3 traversal/계측 optional resolver 검토**다. 남은 전체는 이 부채와 WI1 값종류/53occurrence/3block, WI2 독립 의미·미기록 build profile·지원 범위·설치 역할, WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달, 새독립DEV/FINAL·완전gold·matching/충분성·F01~08, 정식9개·온톨로지 효과다. 실제 LLM 답변 효과는 모델/인증/비용 상한 확정 후다. 선택12개는 별도 backlog다.

이번 회귀111개(WI2 scope31·공통 수용57·문서20·notice3), 문서134개·생성39파일 동일 재생·Python8파일 구문·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/validation/SUMMARY.json)에 Go helper/입력/출력 hash·버전, 원래108입력·실패 보존과 새 독립 표본0을 기록한다. native/승인 입력 hash가738c4a51의 결속과 같아 새 전체 Go 실행은 하지 않았다.

이하 단계56 이전은 당시 기록이다.

## 2026-10-19 단계56: WI3 오류 사유 구분·새 native/실제 MCP 검증

[단계56 보고서](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않으며, D04미정은 실제 검토·승인 대기다.

| 구현·검증 | 결과와 한계 |
|---|---|
| 실패 분류 | typed/wrapped 미해석 심볼은 error/seed_unresolved, 다른 불명 백엔드 실패는 backend_error, 예산 소진은 budget_exhausted. 원래 null·partial·출처·이후 성공에도 남는 실패 상태와 내부 오류 내용 비노출 유지 |
| 새 native 검증 | 전체 빌드·일반/race 각각4107pass/18skip/0fail, fmt·vet·고정 golangci-lint2.14.0(0issues)·경계·CLI 문서 통과. 이전에 통과했던4097개 테스트 누락없음. 이전 native 결과를 현재 결과로 바꾸지 않음 |
| 새 source/binary | 소유 복제본3869a548+정확히2개Go 변경, native/의존 입력1738(Go1282)·승인5 hash 결속. working-tree 빌드이며 현재 HEAD의 clean binary로 주장하지 않음. 시험 candidate와 전체 native에서 만든 binary bytes 동일 |
| 순차 MCP | native 프로세스 종료 후 원래20입력·검색1초 예산 그대로20partial. 후보68개의 출처 신원·순서는20건 모두 이전과 동일. 신호별51error는 이제seed_unresolved, 신호별9observed·8not_requested 유지. 의미 검색 품질 점수는null |
| 제어7건 | 미해석 단독·미해석/정상 양순서의 error/null/partial, 정상 단독의observed, 다른project·잘못된cap·stalecommit 거부를 실제 경로에서 확인 |
| 모든 시도 | 첫 initialize20초 제한은 검색0/미도달20/EOF0. 다음120초 초기 준비 한도 시도는 native 동시 실행 중20호출에서11tool_error/9partial. 순차 실행과 별도 보존하며 앞선 오류를 삭제하거나1초 예산을 늘리지 않음 |
| 모델·자원 | 실제 BGE-M3/native1024 digest와 Ollama0.40.4·graph/source/vector 전후 동일성·EOF0 확인. 초기화와 동시 부하의 배타적 원인·production SLA를 입증하지 않음. 시작 anchor embedding 물리 호출수미측정, 외부/유료LLM·설치역할·운영적용·게시0 |

[현재20건](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/private/current-cases.json)과 원자료는 실제 관측만 기록한다. aggregate reason은 완전한 per-call 실패 이력이나 유일 원인 증명이 아니다. 기존20결과·독립 정답·표본을 변경하지 않았다. 새 전체 source binding을 커밋 후 별도 결속하여 완료 검사를 통과했다.

다음은 **WI2 syntax-only 호출 계약/alias·WI1 select 수신 범위의 reference 반례·WI3 traversal/계측 wrapper 경계 검토**다. 남은 전체는 이 부채와 값 종류/지원 밖/실제 설치역할·독립 의미, WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달, 새독립DEV/FINAL·완전gold·matching/충분성·F01~08, 정식9개와 온톨로지 효과다. 실제 LLM 답변 효과는 모델/인증/비용 상한 확정 후다. 선택12개는 별도 backlog다.

추가·공통 회귀115개(오류 사유36·공통 수용56·문서20·notice3), 새 일반/race 각4107pass/18skip/0fail과 전체 빌드/fmt/vet/lint, 문서134개·원자료를 포함한 생성72파일 동일 재생·Python8파일 구문을 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/validation/SUMMARY.json)은 초기화 실패·동시 실행20건·순차20건을 전부 유지하며 독립 점수를 만들지 않는다. 변경된 Go 소스의 완료 검사도 새 결속 파일 커밋 후 갱신했다.

새 native source 결속을 구현·증거 커밋738c4a51의 source-binding.json에 연결했고, 커밋된 자료를 읽는 완료 검사도 통과했다. [결속 완료 기록](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1-finalization.json)에 서명 확인·1738개 입력·과거 binding 보존과 정식0/9 범위를 기록한다. 이 결속은 새 native 검증의 추적성이며 독립 의미 승인이나 전체 운영 수용이 아니다.

이하 단계55 이전은 당시 기록이다.

## 2026-10-19 단계55: WI1~3 검토 부채와 실제 graph provider 진단

[단계55 보고서](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/WORKLIST.json)이 최신 진입점이다. AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false다. 실명/연락처를 요구하지 않으며 검토 승인만 대기한다.

| 점검 대상 | 실제 확인과 남은 범위 |
|---|---|
| WI3 원래 결과 | 원래20건 모두partial·후보68개 보존. 17건의 시작 심볼26개가 미해석. 조합기는 하나라도 미해석이면 분석 대상3개 모두error로 두며 뒤의 성공이 그 오류를 지우지 않음 |
| 오류 분모 | impact51+concurrency51=102개는 공유되는 출력 label이며 독립 장애102회가 아님. 나머지 RQ-003/004/019는 신호별observed9개. 신호별8개는not_requested. 과거 각 호출의 typed error trace가 없어 배타적 원인/과거 장애0을 단정하지 않음 |
| 현재 실제 provider | 같은 원래binary/retained 데이터에서 관측된 서로 다른40심볼×2도구=80회 모두구조화 응답. 별도6회 탐색 중 미등록심볼2회 명시 거부. 소유 stdio2개 exit0,80회 실행 전후 graph/source/vector inventory 원래 자료와 동일 |
| 결과 보존 | 두 CKS의 시작 시 모델목록 신원 조회는 sandbox 거부로 실패했고 CKV는 degraded였다. 실제 graph 전용 검증이며 전체 retrieval/모델 준비 성공이 아니다. 새 semantic/embedding 추론·LLM·설치에이전트 실행0. 직접 호출 성공으로 원래partial/error를 성공으로 교체하지 않음. 범위 밖/매칭불가66개는unknown 유지. legacy 직접응답이 retained 출처/품질 정답을 대체하지 않음 |
| WI1 부채 | 58개: 암묵 occurrence53·도달불가 빈 block3·select loop 수명 lexical차이2. 값 종류·메모리/타입 의미·독립 검토는 미해결 |
| WI2 부채 | source unknown13·미확인심볼9·분석불가2·반환된 source차이4·미반환 AI양성2. stdlib이 아닌 local/import 대상에 대한 잘못된 매칭과 alias flow를 우선 수정하며 정답/매칭을 결과에 맞춰 완화하지 않음 |

[진단별 원래 사례](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/private/original-case-diagnostic.json), [현재 직접 호출](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/private/current-provider-diagnostic.json), [WI1~3 검토 부채](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/REVIEW-LEDGER.json)을 따른다. 모든 미확정 의미·독립 검토와 설치 역할은 미해결로 유지한다. native 코드 수정은 아직 하지 않았고 소스1738(Go1282)/승인5 동일성으로 이전 전체 native 검증을 재사용한다.

다음은 **WI3의 seed_unresolved·실제 backend_error·budget 구분 구현과 공유 시작 심볼 회귀, 새 native/MCP 검증**이다. 이후 WI2 호출대상/alias·WI1 select 범위/unknown·WI3 unanchored traversal·설치 역할/독립 의미, WI4/V1/V2 실제 경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 필수항목/실제전달, 새 독립 DEV/FINAL·완전gold·matching/충분성·F01~08, 정식9개와 온톨로지 효과가 남아 있다. 실제 LLM 답변 효과는 모델/인증/비용 상한 확정 후다. 선택12개는 별도 backlog다.

현재 회귀111개(진단33·공통 수용55·문서20·notice3), 문서134개·생성26파일 동일 재생·Python6파일 구문·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. 기존 원자료와 pinned source별 tool hash를 유지하고 과거 packet을 다시 seal하지 않았다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/validation/SUMMARY.json)에 현재graph80회/탐색6회와 모델목록 조회 거부2회·CKV degraded 한계를 분리한다. native 구현은 아직 미수정이며 이전 전체 검증 재사용이다.

이하 단계54 이전은 당시 기록이다.

## 2026-10-19 단계54: WI-C DEV10·필수 항목/우선순위·현재 소비자 전달 계약 AI 초안

[단계54 보고서](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **요청한 AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않는다. 익명 역할 ID와 실제 검토/승인·근거 hash·독립성을 구분한다. D04미정은 개인정보 제출 대기가 아니라 검토·승인 대기다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 기존 DEV·원문 | 노출된10함수/8모듈. 원래 Git/file/function/span과 소스 관찰45개+호출자/문맥19개를 확인 |
| 유한 AI 필수 목록 | 소스 관찰64개와 추가 확인 질문20개. 질문의 답은unknown이며 실제 측정/계약을 지어내지 않음. 선택 실험10개 미실행. 완전한 제품 요구·독립 gold가 아님 |
| 이름/우선순위 | 원래 getStartEnd 질문을 보존하며 실제 calculateBlockRange로 정규화. 요구·도달성·영향에 따른 조건부 검토만 작성, 확정 우선순위와 omission 점수null |
| 채택 소비자 대조 | coding-agent d1ebafb8의 Git/파일 proof와 채택 hash 확인. Analyzer5도구 노출·Planner high/medium/low 최대32는 이미 해결됐으며 과거 공백을 재등록하지 않음 |
| 보고서/그림 | typed JSON·근거 JSON·Markdown·Mermaid2개의 정확한5개 산출물. sequence는 지역 CFG flowchart, modules는 파일 소속이며 runtime trace/호출 그래프가 아님. 형식/바이트 검증은 렌더링·의미 품질 승인이 아님 |
| 인수인계/역할 | Analyzer→Planner→Orchestrator, 실제 CLI/hook/state/hash 전달 계약을 기록. 현재 소스 계약·합성 fixture 검증과 실제 설치된 모델 역할 실행을 구분 |

[AI 초안](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/private/AI-gold.json.gz), [현재 소비자 계약](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/consumer-contract.json), [남은 검토](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/REVIEW-LEDGER.json)을 따른다. 같은 기준 ID나6절·우선순위 enum만으로 필수 내용/중요도 정확성을 채점하지 않는다. structural snapshot은 identity_only_unverified_snapshot 한계를 유지한다. 실제 프로젝트 dual-tool·설치 역할·렌더링0, 품질 점수null이다.

다음은 **WI1~3 미확정 의미·partial·공급자 오류·범위 밖·설치 소비자 경로 부채 점검**이다. 남은 전체는 해당 부채, WI4/V1/V2 실제경로/oracle·WI6 nonseed gold/실제쌍/warm·WI-SP 독립 의미/실제overlay/ACL·WI-C 독립필수항목/우선순위/실제보고서/역할, 새 독립 DEV/FINAL·완전 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 효과다. 실제 LLM 답변 효과는 모델·인증·비용 상한 확정 후 수행한다. 선택12개는 별도 backlog다.

현재 회귀136개(WI-C37·공통 수용54·문서20·notice3·비교22), 3bdf444f 기준 과거 WI-SP36개, 문서134개·생성36파일 동일 재생·Python6파일 구문·공백 검사를 통과했다. 초기 검증 중 발생한 fixture cache와 변경 후 이전 tool hash 불일치를 보존하고, 소유 cache 제거·새 미동결 packet 결속 후 전체를 재검증했다. 정식 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/validation/SUMMARY.json)에 범위를 기록한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용한다.

이하 단계53 이전 내용은 당시 기록이다.

## 2026-10-19 단계53: WI-SP DEV12·수동 overlay/출처·거부 경계 AI 초안

[단계53 보고서](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

검토자 개인정보 정책: 실명·이메일·연락처는 요구하지 않는다. 새 승인 기록은 `reviewer_id`에 gold-reviewer / acceptance-reviewer 같은 익명 역할 ID를 사용할 수 있다. 기존 name 필드는 과거 식별자 형식의 호환성으로만 유지한다. 역할 ID는 실제 검토나 승인을 대신하지 않으며, 근거 hash·승인 결정·역할 분리/독립성만 별도로 검증한다. D04미정은 실명 제출 대기가 아니라 실제 검토·승인 상태를 뜻한다. [익명 역할 정책](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/reviewer-identity-policy.json)을 따른다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 기존 DEV·원문 | 원래12심볼/5파일·함수 전체 body·Git/file/span 확인. 승인 평가용 policy42개와 기존 미적용 AI security proposal4개를 완전 목록으로 보존 |
| 유한 관계 AI 정답 | 12×46=552개 명시 selector 단위 전수 분류: 직접 지정9·지정없음543. 모든 semantic applicability는unknown. 문자열/본문 언급이 governs/matches 관계를 만들지 않음 |
| 두 overlay 조건 | policy-only 예상5관계/상대 미태그8심볼. security제안까지 정확히 별도 ingest한 조건의 예상9관계/상대 미태그4심볼. 현재 retained 응답·security ingest 결과가 아님 |
| typed/legacy·출처 | 실제 enum Policy/SecurityPattern과 typed operator description/hint/severity·known 빈 문자열을 유지. legacy 혼합 설명에서 severity/hint 추측금지. 원래 YAML hash/row/version/reviewer/time/approval는 runtime unknown |
| 실제 경계 공백 | query wire에 project/dataset 명시필드가 없어 실제 routed store/session/canonical·source blob/commit/snapshot 결속 필요. 전체 파일 SHA를 retained 함수 blob SHA로 넣지 않음. YAML 작성 근거/graph metadata SHA는 검토자 인증이 아님 |
| 거부 제안16개 | strict decode/selector/cap·pin·private·위조관계·snapshot_changed·peer ACL/forwarded spoof·빈/미발견/부분/자동출처·다른 저장소를 분리. ACL은 per-symbol/project 권한이나 검토자 인증을 증명하지 않음 |

[정답 초안](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/private/AI-gold.json.gz), [전체 overlay·YAML 근거](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/private/overlay-catalog.json), [거부/경계 제안](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/private/controls.json)을 따른다. 수동low는 운영자 검토 제안값이며 취약점 판정/정책 준수 정답이 아니다. no_annotations는 알려진 심볼/완전 페이지의 선택 overlay 범위이며 not_found/unavailable/unsupported/partial과 다르고 안전을 뜻하지 않는다. actual automatic rule query는 별도 opt-in이다.

새 graph ingest·수동/자동 MCP·listener·모델/대상 실행0·품질 점수null이다. 동일 정책/파일/계약의12심볼을 독립12가족으로 승인하지 않는다. 100% annotation/refusal 기준은 실제 경로·독립 정답·분모 동결 후 평가한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 전체 Go 검증을 재사용한다. 단계52 자료는fc4186b9에서 원형 보존한다.

다음은 **WI-C 필수/선택·우선순위·보고서/그림·인수인계/설치 역할 AI 초안**이다. 남은 AI1개는 WI-C다. 남은 전체는 WI-C, WI1~3 부채와 WI4/V1/V2 경로/oracle·WI6 nonseed gold/쌍/warm·WI-SP 의미/ingest/store-session/수동자동/ACL/역할, D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 독립 효과다. 선택12개는 별도 backlog다.

현재 회귀159개(WI-SP36·공통 수용53·익명 검토자 GoldAndSplit25·문서20·notice3·비교22),fc4186b9 원래 소스의 과거 회귀35개, 문서134개·정책/전체 보고서를 포함한 생성24파일 동일 재생·Python compile/공백 검사를 통과했다. 실명 없이 reviewer_id로 형식 검증하되 실제 승인/hash/독립성을 요구하며, 미승인 gold·미동결 실행은 거부한다. 원문·데이터·운영 설정 보존, 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용한다.

이하 단계52 이전 항목은 당시의 기록이다.

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


## 최신 비운영 pilot 진행표 — 2026-10-18

단계44: **WI1 DEV30개·범위 내 AI 정답 초안30/30 작성**. 독립 승인0·정식 수용0/9·FINAL0이다. [현재 초안/검증 보고서](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/REPORT.txt), [현재 전체 작업리스트](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/WORKLIST.json)를 우선한다. 아래 단계43은 당시 준비 기록이다.

단계43: 독립 수용 **준비 작업 FP01~04 완료**, 정식 수용 **0/9**. [준비·검증 보고서](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/REPORT.txt), [전체 작업리스트](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/WORKLIST.json), [검증 기록](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/validation-summary.json)이 최신이다. 아래 pilot4/4와 로컬 로드맵28/28을 독립 정답·동결·수용 완료 수와 합산하지 않는다.

| 단계 | 검증 범위 | 상태 |
|---|---|---|
| SN-P01 | 실제 go-stablenet 소스·데이터·모델 식별·검색 경로 | 기술 범위 완료; 최초 원문 누락은 SN-P02 보완으로 연결 |
| SN-P02 | 외부 원문 보관·convention 구분·기존/새 버전 비교 | 완료 |
| SN-P03 | 출처/무결성과 모델 실패 분류·재시작 없는 복구 | 완료 |
| SN-P04 | 실제 go-wemix BGE 데이터·개념/검색·출처·혼입 비교 | 완료; 새 race232pass·5정상EOF0·2기대기동거부 |
| FP01~04 | 후보206행 노출/작성 인덱스·정답/매칭 양식·충분성/F01~08·온톨로지 설계 | 준비 작업 완료; 새 FINAL/승인 gold0 |
| WI1 DEV/AI | 원래30함수의 전수 지역 binding/구조 제어 초안·unknown·shadow·signature 경계 | 30/30 작성; 단계45 실제 매칭 보완 완료, 의미 정답·독립 검토/동결 미완료 |
| WI2 DEV/AI | 통제40·프로젝트68호출/51함수·전수538단위 AI 정답 | 범위 내 작성 완료; 독립 승인·실제 소비자 매칭/동결 미완료 |
| 정식 준비 | 담당자·새 DEV/FINAL·완전 gold·matching/충분성·F01~08 동결 | 미완료; D04미정·freeze null |
| 정식 수용9개 | WI-1/WI-2/WI-3/WI-4/WI-V1/WI-V2/WI-6/WI-SP/WI-C | 0/9; 실행 미허용 |
| 필수 온톨로지 독립 효과 | 독립 의미/검색 gold·검토 연결·coverage/효과 gate | 미수용; 기존 제한 연구완료/inconclusive 유지 |
| 선택 후속 | 별도 후속12개 | 별도 backlog |

[SN-P04 최신 검증](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/REPORT.txt), [남은 전체 작업](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/WORKLIST.json). 아래28/28 표는 기존 로컬 로드맵의 완료 기록이며 정식 수용 완료 수와 합산하지 않는다.

2026-10-16 현재 KM-01–06을 완료했다. 전체28개 중28개 완료·0개 진행 중·0개 미착수다. 로컬 구현·개발 효과 관찰 범위를 완료했으며 독립 품질 수용과 구분한다. [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)과 [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)를 따른다.

표의 **통과**는 명시한 시험 범위만 뜻한다. **부분 검증**은 완료 표시가 아니며 **미실행**을 통과0건으로 바꾸지 않는다. 신규7개 기능 전체 완료는7/7이다.

## 전체28개 세부 작업과 시험 단계

| ID | 목적·시험 범위 | 구현/작업 단계 | 검증 단계 |
|---|---|---|---|
| KM-01.1 | 신원·key·cache·join·쓰기 조사와 이행 설계 | 완료 | [재사용/결손 표](./PROJECT-COLLECTION-ISOLATION.md) 작성·코드 대조 |
| KM-01.2 | Project/Version/Collection과 고정 scope 계약 | 완료 | catalog·fork·두 버전·직렬화/재시작 시험 통과 |
| KM-01.3 | 구현 대상 제한·명시 비교·잘못된 scope 거부 | 완료 | 실제 CKG/CKV3DB·공개 MCP24항목 통과; 원시 응답/인용 재검증 |
| KM-01.4 | 동명·fork·두 버전·cache·재시작·쓰기/호환 반례 | 완료 | 실제4DB·최종52항목 통과; 동일 원문 fork·모델 분리·쓰기18건 거부·rollback/GC/재시작 보존 |
| KM-02.1 | legacy/versioned capability와 새 요청·상태 계약 | 완료 | [계약](./VERSIONED-MAINTENANCE-CONTRACT.md)·schema·101사례 통과; 새 공개 실행은 .2–04 |
| KM-02.2 | 비동기 build/status/cancel과 정합 gate | 완료 | [공개 실행](./VERSIONED-MAINTENANCE-RUNTIME.md)28항목·실제 retained 조회2건·EOF5개·전체 Go/race·증거 보호10개 통과 |
| KM-02.3 | idempotency/lock/중단/실패/rollback | 완료 | A–D2·실제 E1 host43/E2 retention21·전체 Go/race 검증; .4 별도 |
| KM-02.4 | 공개 도구의 잘못된 대상·부분 실패·취소 보호 | 완료 | 실제 공개36항목·부분 DB·기존 current/reader·마지막 v4 정상 게시/조회 검증 |
| KM-03.1 | 매니저 등록·설치·초기 구축·복구 절차 | 완료 | 등록14·native3·실제 metadata bootstrap9·증거10; 초안/검토/reader 미구현 |
| KM-03.2 | 문서 없는 프로젝트의 근거 기반 지식 초안 | 완료 | 수집23·등록14·실제 Git/CKS native 초안24·증거13; 사람 검토/DB 조회 아님 |
| KM-03.3 | 암묵지 질문·불확실성·검토/채택/회수 | 완료 | 검토35·수집23·등록14·실제 CLI/native24·증거14; 통제 응답, DB 조회/신원 인증 아님 |
| KM-03.4 | 실제 Claude host 전체 구축·조회와 실패 복구 | 완료 | 실제 CLI8회·도구68개, manager103·native3·증거16, 정상 종료0·중단/복구 |
| KM-04.1 | 수동·정기·변경 trigger와 성공 기준 | 완료 | 정책/시각/성공27개·증거10개; 실제 scheduler 미실행 |
| KM-04.2 | 변경/삭제/no-op/stale와 재검토 | 완료 | 입력32·정책27·manager103·실제4DB/16항목·82호출/11EOF0·증거14; 수동 |
| KM-04.3 | 중복·lease·재시작·retry·자원 상한 | 완료 | A/B1/B2/C1/C2 통합; Python268·공개MCP20/222호출·실제Claude2회/36도구·EOF/kill/exact 복구; .4 별도 |
| KM-04.4 | 두 번 변경+no-op+실패 복구·실제 주기 실행 | 완료 | 실제2프로젝트·20항목·221호출·2EOF0·문서/자료 삭제·메모 교체·graph 실패/연결 retry·타 DB hash/인용 불변 |
| KM-05.1 | 산업 개념·정의·별칭·비교 축 | 완료 | 선택형 팩8개념/26축·한영/모호성·버전 pin29사례·새 전체Go/race7패키지 검증 |
| KM-05.2 | 공통 개념과 프로젝트별 코드/정책/시험 연결 | 완료 | 비교59개(어휘29+연결30)·실제 보관 원문3프로젝트/5종류·committed/working-tree·전체Go/race7 검증 |
| KM-05.3 | exact/close/broader 등의 검토·버전 의미 | 완료 | 정책34개·비교93개·정족수/회수/상충/미래·source/pack 변경 stale·과거 버전·키 별칭 거부 |
| KM-05.4 | 비적용/미수집/미지원·L1/L2·network 차이 | 완료 | 적용/지원24개·비교117개·Rust text/AST·L1/L2·정책 차이/내부상충·선언/미결정·schema1/2·손상 |
| KM-06.1 | 명시 버전·개념·축·시점·상한의 비교 요청 | 완료 | 계약50개·비교167개·pin/scope/권한/시각/예산·요청 경로/target 거부; 실제 MCP/조회0 |
| KM-06.2 | 프로젝트별 근거 행렬/bundle | 완료 | 실제 mock2DB·12항목/9도구·2EOF0·프로젝트별 원문/인용·unknown·후보 기본 제외·잘못된 요청/예산 거부 |
| KM-06.3 | 상충·누락·자료 부족·부분 실패 상태 | 완료 | 새 보호21개·비교192개·공개 기반12+보호9항목/18호출·3EOF0/SIGTERM0·다른 프로젝트 보존·복구 hash 동일 |
| KM-06.4 | 둘 이상 실제 프로젝트의 비교·오인·근거 시험 | 완료 | 실제2프로젝트/4243파일·6개념40셀·22항목19도구·3EOF0·모드/범위/해시/오매핑/누락/예산·원본/DB 보존 |
| OE-01.1 | 질문·기대 근거·독립성·분모 고정 | 완료 | 새16질문·14함수 과제/2미채점 제어·5사실군11함수·512회 사전 계획; 자체 source 기준/사람·독립 gold0/새 검색0 |
| OE-01.2 | 같은 자료의 off/concept_text/relations/combined | 완료 | 실제512회·실패/누락0·64capture/4readiness 정상 종료·전체 source/본문/인용/무결성·API 계측·원본 보존; 효과 점수 다음 |
| OE-01.3 | 근거·순위·unknown·지연/비용·답변 효과 | 완료 | 고정 함수 과제P/R/MRR·EX08 한정 이득·EX16 범위 진단·warm/API/token 재계산; 일반/답변/독립 품질 미측정, off/inconclusive |
| OE-01.4 | 다중 프로젝트 개념 매핑·오인·비교 효과 | 완료 | 실제2프로젝트·매핑 기준선/처리40셀·18항목27호출·2EOF0; 원문8셀/양측후보3개·unknown 유지·품질 inconclusive |

아래 단계별 기록은 당시 checkpoint의 범위다. 현재 전체 상태는 위 표와 원장을 따른다.

## 이번 KM-01 공통 계층 시험 전체 목록

[시험 코드](../../internal/system/collection/collection_test.go)의17개 상위 시험, 하위 사례 포함40개다. 모두 race 모드에서 통과했다. fixture의 archive/manifest/실제 Git commit을 사용하며 공개 MCP나 실제 graph/vector 검색의 완료 근거로 확대하지 않는다.

| 시험 | 검증 내용 | 결과 |
|---|---|---|
| TestSameCommitForksAndNamesRemainSeparate | 같은 이름·URI·코드·commit의 fork를 별도 그룹/인용으로 유지 | 통과 |
| TestImplementationDispatchesOnlyExplicitTarget | 구현 대상1개만 backend 호출 | 통과 |
| TestTwoVersionsAndWarmProviderCannotCrossTargets | 두 버전·잘못된 warm cache 응답 거부·scope namespace 분리 | 통과 |
| TestRequestRejectionsNeverDispatch | 누락/불일치 target·catalog·mode·참여자 등14개 거부 | 통과 |
| TestCatalogRoundtripRestartAndCallerMutation | 저장/재시작 같은 pin, caller 변경과 종료 후 요청 거부 | 통과 |
| TestCatalogMembershipDigestChangesAndOrderDoesNot | 순서 독립·멤버 변경 시 digest 변경 | 통과 |
| TestDuplicateRegistrationAndForkErrors | 중복 등록·공유 경로·잘못된 fork/origin 등9개 거부 | 통과 |
| TestStrictCatalogDecode | 중복 JSON key·unknown·후행 값·용량 제한 | 통과 |
| TestScopedLocalSymbolKeysKeepProjectVersionAndOrigin | 동명 import/symbol의 프로젝트·버전·origin 키 분리 | 통과 |
| TestForeignBodyAndHashesRejectedEvenWhenRestamped | 재서명된 오본문/해시 거부와 정상 정제 본문 허용 | 통과 |
| TestCurrentAliasesAndMismatchedArchivesRejected | symlink·다른 archive·미수집 origin 거부 | 통과 |
| TestComparisonFailureDiscardsEarlierGroup | 뒤 backend 실패 시 앞 그룹도 공개하지 않음 | 통과 |
| TestConcurrentTargetReadsAndClose | 동시8개 대상 읽기와 종료 이후 거부 | 통과 |
| TestCanceledReadDoesNotDispatch | 취소 요청은 backend 미호출 | 통과 |
| TestArchiveMutationDuringLaterComparisonDiscardsAll | 뒤 참여자 실행 중 앞 archive 손상 시 전체 폐기 | 통과 |
| TestCloseWaitsForInFlightRead | 진행 중 요청 전 lease 해제 방지 | 통과 |
| TestReaderLeaseAndExistingRollbackStayCompatible | 기존 GC reader 보호·rollback/current·단일 retained 읽기 호환 | 통과 |

초기 fixture의 commit manifest 누락, 작성 중 unused import, 읽기 전용 blob의 손상 재현 실패도 원자료에 보존한다. 최종 통과 로그와 구분한다.

## 이번 공개 MCP 검증 전체 목록

실제 graph/vector DB3개를 새로 만들고 MCP 프로세스3개를 실행했다. embedding은 `mock-feature-hash-v1`/64차원이며 유료 모델 호출0회다. 도구21호출 중12개 성공·9개 의도한 오류, 총26개 요청·응답이다. 성공한14개 evidence pack의 좌표·인용·보관 원문/범위 해시·본문·integrity를 재검증했다. 아래24개는 이21호출과 시작 거부·보존 검사를 묶은 항목으로 서로 더하지 않는다.

| 순서 | 확인 항목 | 결과 |
|---|---|---|
| 1 | 다른 프로젝트 vector index 설정 시작 거부 | 통과 |
| 2 | 중첩 collection 설정 시작 거부 | 통과 |
| 3 | 다른 sanitizer 설정 시작 거부 | 통과 |
| 4 | 사용 불가 embedding model 시작 거부 | 통과 |
| 5 | member profile 누락 시작 거부 | 통과 |
| 6 | mutable current 경로 시작 거부 | 통과 |
| 7 | A 이전 버전101 조회 | 통과 |
| 8 | B 버전202 조회 | 통과 |
| 9 | A 새 버전303 조회 | 통과 |
| 10 | B 재조회·warm 분리 | 통과 |
| 11 | A 이전 버전 재조회·warm 분리 | 통과 |
| 12 | 세 버전 명시 비교·각 그룹 분리 | 통과 |
| 13 | target 누락 거부 | 통과 |
| 14 | 오래된 catalog digest 거부 | 통과 |
| 15 | 다른 project 좌표 거부 | 통과 |
| 16 | 다른 snapshot 좌표 거부 | 통과 |
| 17 | 없는 collection 거부 | 통과 |
| 18 | caller config 경로 주입 거부 | 통과 |
| 19 | 미지원 mode 거부 | 통과 |
| 20 | B 원문 손상 시 비교 전체 거부·A 정상 유지 | 통과 |
| 21 | 원문 복원 후 비교 회복 | 통과 |
| 22 | 기존 pinned reindex 거부·job 미생성 | 통과 |
| 23 | 재시작·current/소스가 새 버전이어도 이전 버전 유지 | 통과 |
| 24 | 기본36도구·기존 단일 프로젝트 V2 호환 | 통과 |

새 Go 상위 시험6개, 하위 사례 포함14개가 모두 통과했다.

| 시험 | 검증 내용 | 결과 |
|---|---|---|
| TestCollectionConfigValidation | 비활성 기본값·절대 경로·중복·누락 등8개 설정 사례 | 통과 |
| TestCollectionConfigKnownFieldsRemainStrict | YAML unknown 필드 거부 | 통과 |
| TestCollectionToolsAreOptInAndReadOnly | 기본 도구 불변·선택2도구·schema와 readOnly | 통과 |
| TestCollectionServiceUsesExistingV2AndClosesOnce | 기존 V2 경로·대상/인용·종료 후 거부·중복 close | 통과 |
| TestCollectionFactoryFailureClosesOpenedBackend | 부분 backend 실패 자원 정리 | 통과 |
| TestCollectionPublicUnknownAndMissingTargetsAreRejected | 공개 unknown 필드·target 누락 거부 | 통과 |

## 남은 격리·기능 시험

| 검증 대상 | 현재 상태 | 연결 작업 |
|---|---|---|
| 실제 CKG/CKV와 프로젝트별 provider 설정·공개 scope 오류/인용 | 이번 범위 통과 | KM-01.3 완료 |
| 동일 바이트 fork 실제 DB·서로 다른 embedding 구성·cache 반복 | 실제4DB·프로토콜52항목 통과 | KM-01 완료 |
| 공개 쓰기 도구 전체 표면·다른 project/version DB와 current 보존 | 공개 CKS 변경3도구/18건 거부·보존 통과; 새 version-aware 쓰기 남음 | KM-02.3/04 |
| 실제 Claude 매니저 host·등록/구축/실패 복구·갱신 | KM-03 실제 로컬 연결 완료; 정기 갱신 미실행 | KM-04 |
| 산업 개념 매핑·비교 행렬·실제 프로젝트 비교 | 미구현·미실행 | KM-05/06 |
| 자연어·검토된 구현 관계·다중 프로젝트 ontology 효과 | 로컬 단일 프로젝트 관찰만 완료 | OE-01 후속 |

## 전체 검증 gate와 기록

[최신 validation](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/validation.json), [Go 시험 전체3,619개와 race446개 목록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/test-inventory.json), [원시 응답·소스 manifest](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/manifest.json)을 함께 보존한다. 기반40사례·신규14사례는 전체 Go/race 분모에 이미 포함된다.

| 최종 gate | 결과 |
|---|---|
| 전체 모듈 빌드 | 통과 |
| 전체 Go 시험 | 3,600개 통과·19개 명시적 skip·실패0 (총3,619개) |
| 관련4패키지 race | 444개 통과·2개 명시적 skip·실패0 (총446개) |
| 기반 collection race 포함 | 17개 상위 시험/40사례 통과·skip0 |
| 실제 DB·공개 MCP | 24항목·21도구호출·26요청/응답·정상 EOF3회 통과 |
| fmt·전체 lint·vet·engine 경계·문서 | 통과; lint0 issues·문서125개·계약20개·패키지 안내3개 |
| 작업표 허위 완료 방지 | 기존7개·새 공개 증거8개 통과 |
| 기존 소스목록 갱신 | 구현 커밋16a75b8d의1,652개 결속·기존 source/input/evidence guard 통과 |

전체 모듈의19개 skip과 관련 race의2개 skip은 명시적인 기존 환경 조건이며 통과로 세지 않는다. 각 이름·패키지·상태는 전체 시험 목록에 보존했다. package skip12개도 별도 집계한다. 이전 [기반 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-01-foundation/manifest.json)의3,605개/공개 미연결 기록은 당시 범위를 보존하며 최신 상태는 이번 packet을 따른다. 처음 fixture의 model 이름을 잘못 지정해 시작이 거부된 이력도 `initial-failure/`에 보존했다.

실제 DB/manifest18파일과 current 포인터2개가 그대로임을 확인했다. 원문 손상은 소유한 fixture에서 재현하고 정확히 복구했다. 유효한 조회 근거14팩을 검증한 사실과 검색 품질·독립 수용은 분리한다. ontology의384회 관찰과 별도8회 진단에 이번 분모를 합치지 않는다.

KM-01 최종 격리를 완료했다. KM-02.1 계약 설계를 완료했다. 다음은 KM-02.2의 공개 inspect/build/status/cancel·실제 DB gate 연결이다. 신규7개 기능 외의 생성기 보완·운영/독립 수용9개·선택 후속12개는 [전체 작업표](../REMAINING-WORK.md)에 유지한다.

[소스 결속 후속 기록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-source-binding-update/receipt.json)은 구현 커밋16a75b8d의 manifest를 기존 원장에 연결하고 나머지 승인·완료 필드를 그대로 유지한 사실을 기록한다. 공개 조회 packet의 `pending_committed_manifest`는 구현 커밋 당시의 상태이며 현재 완료된 소스 검사는 이 후속 기록을 따른다.

## 최종 KM-01.4 시험 전체 목록

실제 DB4개·공개 MCP52요청/응답·46도구호출·정상 종료3회다. 도구46회 중20회 성공(상태3회+조회17회),26회는 의도한 오류이며 성공 근거20팩을 재검증했다. 다음52항목은 호출/시작/보존 검사를 묶은 분모이므로 다른 시험 수에 더하지 않는다. [원시 증거](../../system/eval/knowledge-roadmap-2026-10-15/km-01-final-isolation/manifest.json)와 [전체 Go/race 사례 목록](../../system/eval/knowledge-roadmap-2026-10-15/km-01-final-isolation/test-inventory.json)을 보존했다.

| 순서 | 검증 ID | 결과 |
|---|---|---|
| 1 | identical-fork-commit-and-all-retained-file-bytes | 통과 |
| 2 | startup-rejected:duplicate-project | 통과 |
| 3 | startup-rejected:duplicate-version | 통과 |
| 4 | startup-rejected:shared-version-path | 통과 |
| 5 | startup-rejected:fork-cycle | 통과 |
| 6 | startup-rejected:wrong-member-model | 통과 |
| 7 | startup-rejected:wrong-member-dimension | 통과 |
| 8 | warm-mixed-read:0:a1 | 통과 |
| 9 | warm-mixed-read:1:b1 | 통과 |
| 10 | warm-mixed-read:2:b2 | 통과 |
| 11 | warm-mixed-read:3:a2 | 통과 |
| 12 | warm-mixed-read:4:b1 | 통과 |
| 13 | warm-mixed-read:5:a1 | 통과 |
| 14 | warm-mixed-read:6:b2 | 통과 |
| 15 | warm-mixed-read:7:a2 | 통과 |
| 16 | identical-forks-and-embedding-versions-not-deduplicated | 통과 |
| 17 | request-rejected:wrong-subcollection | 통과 |
| 18 | request-rejected:other-project-same-commit | 통과 |
| 19 | request-rejected:duplicate-participant | 통과 |
| 20 | runtime-model-digest-drift-refused | 통과 |
| 21 | other-project-works-during-model-drift | 통과 |
| 22 | same-project-other-model-works-during-model-drift | 통과 |
| 23 | restored-model-stays-refused-until-reader-restart | 통과 |
| 24 | writer-rejected:index-full | 통과 |
| 25 | writer-rejected:index-incremental | 통과 |
| 26 | writer-rejected:setup:project-a:root | 통과 |
| 27 | writer-rejected:reindex:project-a:root | 통과 |
| 28 | writer-rejected:setup:project-a:version | 통과 |
| 29 | writer-rejected:reindex:project-a:version | 통과 |
| 30 | writer-rejected:setup:project-a:current | 통과 |
| 31 | writer-rejected:reindex:project-a:current | 통과 |
| 32 | writer-rejected:setup:project-a:new-child | 통과 |
| 33 | writer-rejected:reindex:project-a:new-child | 통과 |
| 34 | writer-rejected:setup:project-b:root | 통과 |
| 35 | writer-rejected:reindex:project-b:root | 통과 |
| 36 | writer-rejected:setup:project-b:version | 통과 |
| 37 | writer-rejected:reindex:project-b:version | 통과 |
| 38 | writer-rejected:setup:project-b:current | 통과 |
| 39 | writer-rejected:reindex:project-b:current | 통과 |
| 40 | writer-rejected:setup:project-b:new-child | 통과 |
| 41 | writer-rejected:reindex:project-b:new-child | 통과 |
| 42 | no-async-job-created | 통과 |
| 43 | GC-protects-all-pins:project-a | 통과 |
| 44 | GC-protects-all-pins:project-b | 통과 |
| 45 | active-new-pin-ignores-rollback-to-old | 통과 |
| 46 | active-old-pin-after-rollback | 통과 |
| 47 | restart-reordered-catalog-keeps-new-pin-while-current-old | 통과 |
| 48 | restart-identical-fork-keeps-model96 | 통과 |
| 49 | restart-same-source-keeps-model128 | 통과 |
| 50 | membership-change-invalidates-old-catalog-pin | 통과 |
| 51 | removed-member-refused-after-restart | 통과 |
| 52 | remaining-member-works-after-membership-change | 통과 |

실패한 첫 실행은 모델 복구 직후 자동 회복을 기대했다. 기존 CKV는 실패 상태를 reader 재시작까지 유지하므로 마지막 기대가 실패했고, 이후 시험은 지속 거부·다른 member 정상·재시작 후 회복을 검증했다. 원래 실패 로그·응답·당시 runner를 `initial-run/`에 보존한다.

| 최종 단계 gate | 결과 |
|---|---|
| 실제 Git 동일 원문 fork·4DB·모델64/96/128 | 통과; 로컬 mock/시험 응답기 프로토콜 |
| 공개 CKS 쓰기 표면 | index2모드 + setup/reindex의2프로젝트×4대상 =18건 거부·job 미생성·전체 tree 불변 |
| rollback·GC·재시작·멤버 변경 | 통과;4개 reader pin 보호, current 복원·B current 보존 |
| 원시 MCP/HTTP/Git/인용·본문 재검증 |52요청/응답·46도구호출·근거20팩 확인 |
| Go 전체·race·build/fmt/lint | 소스1,652개 동일 바이트 확인 후 이전 통과 증거 재사용; 이번 Go 변경0 |
| 문서·최신 작업표·증거 변조 방지 | 문서125개·계약20개·안내3개·현재 작업표7개·변조/누락 방지9개 통과 |

이번 단계의 새 실행과 이전 Go 시험을 다시 실행한 것처럼 합산하지 않는다. Go는3,600통과/19skip, 관련 race는444통과/2skip의 원자료와 모든 사례 이름을 그대로 보존했다. 기존 기반 시점의7개 작업표 시험은 동결 원자료다. KM-01 전체 완료 후에도 미완료 기능을 대상으로 허위 완료를 검사하도록 `scripts/test_knowledge_roadmap_current.py`를 현재 회귀 진입점으로 추가했다. 원래 시험/packet을 수정하지 않고7개 검사를 이어받으며 부분 기능 대상 선택만 현재 미완료 항목으로 바꾼다.

남은 기능은 KM-02~06 및 OE-01이다. 산업 개념별 비교·실제 모델 검색 품질·독립 수용이나 새 version-aware 쓰기는 KM-01 완료에 포함하지 않는다.

KM-01 완료 당시의 진입점은 `python3 scripts/check-collection-isolation.py`, `python3 scripts/test_collection_isolation.py`였다. 이후 진행 수가 증가한 현재는 `python3 scripts/verify_committed_collection.py`로 당시 source/문서 snapshot의 증거를 검증하고 현재 작업표는 `python3 scripts/test_knowledge_roadmap_current.py`로 별도 검사한다. 새 검사9개는 정상 packet, 응답 누락, 동일 원문 fork의 재서명된 다른 프로젝트 인용, 다른 embedding 차원, 거부된 쓰기의 파일 변경, 종료 오류, Go skip의 통과 승격, 독립 수용 과장, 다른 모델 dispatch 기록을 검사한다. 종료 후 독립 재캡처한24파일/current2개의 값도 실행 전과 일치한다.

## KM-02.1 설계 적합성 전체 시험

[strict schema·전이 계약](./VERSIONED-MAINTENANCE-CONTRACT.md)과 [원시 결과](../../system/eval/knowledge-roadmap-2026-10-15/km-02-contract/contract-results.json)의101개 사례다. 각 사례의 ID·기대값·실측값·통과 여부를 보존하며 새 runtime job을 실행한 것으로 세지 않는다.

| 분류 | 사례 수 | 확인 범위 |
|---|---|---|
| request | 30 | profile/project/source/input/model/base/version·정책·경로/명령 주입 거부 |
| status | 28 | 9개 상태·후보/gate/publication·외부 좌표·조회 반영 과장 거부 |
| transition | 12 | 정상 전이·terminal 재시작·취소 후 성공 거부 |
| cancel | 9 | 접수/중복/종료/복구/commit 경계의 취소 판정 |
| idempotency | 3 | 새 요청·동일 요청 replay·같은 key의 다른 fingerprint 거부 |
| reference | 4 | project/profile/job 소유 범위 |
| decode | 6 | 정상 JSON·중복/후행/비유한 수/소수 표기/용량 거부 |
| fingerprint | 1 | key 순서와 무관한 동일 요청 digest |
| observation | 8 | status revision 역행·동일 revision의 다른 본문·다른 job·terminal 변경 거부 |

101개 모두 통과했다. 현재 MCP35도구를 initialize/tools-list2요청으로 새로 확인했고 새 versioned build/cancel은 미노출·정상 EOF0이다. CLI의 version/project/source-mode/hold/test-gate와 rollback help도 확인했다. 최초 help probe는 stdout에만 상세 flag가 있다고 기대해 실패했다. 실제 flags는 stderr에 있으며 두 stream을 보존하고 결합해 검증하는 최종 probe가 통과했다. 제품 코드는 바꾸지 않았다.

KM-01의 최종 packet은 당시 커밋cb373e94의 소스·문서 snapshot에서 검증한다. 현재 완료 수6을 과거 완료 수4에 억지로 맞추거나 과거 packet을 다시 쓰지 않는다. 현재 작업표28개 일치 검사는 별도로 실행한다. KM-02.1 당시에는1,652개 소스·의존 입력의 바이트가 같아 이전 Go 증거를 재사용했다. 이 수에는 Go1,223개와 기타 입력429개가 포함되며, 이전 표기의 “Go1,652개”를 바로잡는다. KM-02.2는 Go 소스가 변경되어 전체 시험을 새로 실행했다. 새로운101개 설계 사례를 Go 시험 분모에 더하지 않는다.

| 이번 단계 gate | 결과 |
|---|---|
| 설계 schema/상태/취소/오류 적합성 | 101/101 통과 |
| 실제 현재 capability 읽기 전용 probe | MCP35도구·2요청·정상 EOF0, CLI help 통과 |
| 이전 KM-01 증거·Go 바이트 결속 | commit snapshot 검증·Go1,652개와 의존 파일 동일 |
| 현재 작업표·문서·증거 허위 완료 방지 | 현재 작업표7개·문서126개/계약20개/안내3개·새 증거 검사9개 통과 |
| 새 공개 비동기 구축·취소·durable job | 미구현·미실행; KM-02.2–04 |

KM-02.1 이후 공개 연결을 KM-02.2에서 구현·검증했다. 현재 다음 작업은 KM-02.3의 중복·재시작·publication 복구이고, 이후 .4의 공개 실패 경계를 검증한다.

KM-02.1 당시 설계 적합성·증거 검사는 `python3 scripts/check-versioned-maintenance-contract.py`, `python3 scripts/check-maintenance-design-evidence.py`, `python3 scripts/test_maintenance_design_evidence.py`로 실행한다. 증거9개 시험은 정상 packet·사례 누락·거짓 성공·설계의 공개 runtime 승격·변경 도구를 실행한 probe·지원 CLI flag 누락·과거 소스 결속의 위조·실패 gate·source audit 변조를 검사한다. 과거 runtime snapshot과 현재 작업표·설계 적합성을 분리하여 완료 수 증가만으로 과거 증거를 다시 만들지 않는다.


## KM-02.2 공개 구축·조회·취소 완료

[실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)와 [원자료 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-maintenance/manifest.json)을 따른다. 기존 과거 packet은 변경하지 않는다. 설계 증거는 `python3 scripts/verify_committed_maintenance_design.py`로7034ce85 snapshot에서 검사하며, 현재 runtime은 새 Go 시험으로 검증한다.

| 검증 | 결과 |
|---|---|
| 공개 MCP lifecycle 고정 항목 | 28/28 통과; 두 프로젝트 실제 CKG/CKV·deterministic mock embedding |
| 실제 dataset 도구·wire | 90회 호출·전체99쌍; polling 수는 시험 항목 수와 별도 |
| 최초/두 번째 실제 publication | A v1·v3 성공, 정확한 receipt·base·좌표 확인 |
| 검토 보류·취소·격리 | A v2/B v1 held, B v2 실행 중 취소, A v1 불변·B current 없음 |
| 실제 조회 | A 이전 v1 marker101·새 v3 marker404 retained 본문/인용 확인 |
| 연결 종료 | 기본·maintenance·재연결·reader2개 총5개 정상 EOF |
| 전체 Go | 3,665사례=3,646통과+19skip; 실패0, 전체135package 정상 종료 |
| 관련5패키지 race | 634사례=632통과+2skip; 실패0 |
| build/fmt/vet/lint/경계/문서 | 통과; 문서 계약20개·패키지 안내3개 |
| 증거 변조·허위 완료 방지 | 10개 통과; wire 누락·거짓 held/canceled 성공·종료 오류·durability 과장·skip 승격·타 프로젝트 인용·DB 변경·lint 실패 |
| 현재 소스 결속 | 기존 입력+새 Go8개=1,660개; 유지 Go1,231개+기타429개, 승인 상태 변경 없음 |
| 영속 job·재시작 복구 | 미구현·미실행, KM-02.3 |
| 전체 KM-02·독립 수용·검색 품질 | 완료 아님 |

현재 완료6/28·진행4/28(KM-02.3, OE-01.1–03)·미착수18/28다. 다음은 KM-02.3, 이후 .4 → KM-03 → KM-04 → KM-05 → KM-06 → OE-01 확대/다중 프로젝트 평가 순서다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 범위다.


## KM-02.3 중간 단계 A: 기록 기반만 검증

| 검증 | 결과·한계 |
|---|---|
| Go 상태/전이/관찰의 설계 대조 | 28/12/8사례와 parent 시험 통과 |
| maintenance package race | 전체114사례 통과·실패/skip0 |
| journal 기록 | scope/key·단독 writer·재열기·terminal 불변·원자적 교체·손상/거짓 성공·symlink/hardlink·128한도 검사 |
| 별도 시험 프로세스 종료 | 기록 rename 전/후 즉시 종료와 재열기 통과; 실제 DB 작업 복구는 아직 아님 |
| 전체 Go | 3,734사례=3,715통과+19skip·실패0 |
| 관련5패키지 race | 703사례=701통과+2skip; maintenance114개 포함 |
| 중간 증거 보호 | 13개 통과; private 기반을 공개 영속/복구 완료로 표시하는 오류 거부 |
| 공개 기존 동작 회귀 | 다시28항목·5개 정상 EOF; 영속 capability는 계속 process_lifetime |
| 완료 집계 | 6/28 유지; KM-02.3은 진행 중 |

다음은 [실행 명세의 B–E](./VERSIONED-MAINTENANCE-RUNTIME.md)에 따라 공개 service의 durable 접수·replay, publication 전후 증거, 하위 프로세스 소유/중단, 실제 MCP 복구와 rollback/GC/read lease를 연결·검증한다. 이 단계의 기록 저장기 시험을 공개 복구나 독립 수용으로 올리지 않는다.

A 단계의 [검증 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-journal-foundation/manifest.json)에 새 전체 시험 목록·원시 wire를 보존한다. 공개 동작 재실행은112개 도구 호출·121개 전체 wire쌍이며 polling 횟수는 고정28개 항목과 별도다. 현재 소스 결속은1,664입력(유지 Go1,235개·기타429개)이다.


## KM-02.3 중간 단계 B: 비공개 service 연결

| 검증 | 결과·한계 |
|---|---|
| maintenance package race | 123사례 통과; 기존 A114개 포함 |
| 기존 전환 경로 포함 관련5패키지 race | 714사례=712통과+2skip·실패0 |
| 전체 Go | 3,745사례=3,726통과+19skip·실패0 |
| 비공개 durable service | 기록한 종료 상태 재시작/replay, changed pin 재조회, key 충돌·타 프로젝트 거부, coordinator owner, 취소 전 기록·불확실한 snapshot 거부 |
| 실제 orchestration의 고의 graph 실패 | 실패 상태 보존·current 미발행·재시작 같은 결과; 성공 DB 검증을 대신하지 않음 |
| 최초 접수 rename 전/후 기록 오류 | 접수 성공 미응답·graph 미실행, 재시작에서 온전한 없음/queued 기록만 복원 |
| 공개 기존 동작 | 28항목·EOF5개 재검증; 공개 persistence는 여전히 process_lifetime |
| 공개 재시작·publication 장애·하위 작업 정리 | C–E 미구현·미실행; KM-02.3 완료 아님 |

[실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)의 B 범위와 [B packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-service/manifest.json)을 따른다. 다음은 취소 의도 결속과 publication 전후 이력 정합(C), 하위 프로세스 소유/중단(D), 실제 공개 복구·read lease/rollback/GC 검증(E)이다. 현재 전체6개 완료·4개 진행 중·18개 미착수다. 이후 KM-02.4 → KM-03 → KM-04 → KM-05 → KM-06 → OE-01 확대/다중 프로젝트 평가를 진행한다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다.

B 증거 보호13개도 통과했다. 실제 공개 회귀는81개 도구 호출·전체90개 wire쌍이며 고정28개 항목과 별도로 센다. 현재 소스 결속은1,668입력(유지 Go1,239개·기타429개)이며 기존 승인/완료 상태는 바꾸지 않는다.


## KM-02.3 중간 단계 C: shared 전환 이력과 비공개 복구

| 검증 | 결과·한계 |
|---|---|
| 실제 pointer/강제 종료/후속 writer | setup19사례 통과; 5개 지점의 실제 프로세스 kill·일반 전환·rollback 후 기존 이력 보존·손상 거부 |
| 비공개 복구·기존 maintenance | maintenance race141사례 통과; job/source/model/base 결속·취소 의도·여러 재시작·충돌 보류 |
| 실제 보관 held DB | 기존 source-bound SQLite·원문·hold marker 검사 뒤 held 복구; current 미발행 |
| 합성 history 정책 시험 | 별도 unit scope; 실제 DB 게시로 집계하지 않음 |
| 전체 Go | 3,782사례=3,763통과+19skip·실패0 |
| 관련5패키지 race | 751사례=749통과+2skip·실패0; 위141개와 setup19개 포함 |
| 공개 기존 동작 회귀 | 28항목·97도구호출·106wire쌍·5개 정상 EOF; shared history와 실제 receipt도 대조 |
| 증거 보호 | 14개 통과; 외부 project로 재서명한 실제 history·합성 승격·D/E 완료 과장 거부 |
| build/fmt/vet/lint/경계/문서 | 통과; 문서 계약20개·안내3개·live 문서127개 |
| 공개 durable·pre-gate child 종료 | D/E 미구현·미검증; KM-02.3 완료 아님 |

[C packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-publication-recovery/manifest.json)에 전체 시험 목록·원시 protocol·실제 pointer history를 보존한다. 현재 소스 결속1,674입력은 유지 Go1,244개·기타430개이며, 실제 held DB 검사에 사용한 기존 immutable fixture도 포함한다. 기존5개 approved input과 승인/완료 상태는 바꾸지 않는다.

현재 전체6개 완료·4개 진행 중·18개 미착수다. 다음은 D의 부모 생존/deadline 감독·하위 writer lease·안전한 staging 정리, 이후 E의 실제 공개 재시작·lease/rollback/GC 검증이다. 그 뒤 KM-02.4 → KM-03.1–4 → KM-04.1–4 → KM-05.1–4 → KM-06.1–4 → OE-01.1–4를 진행한다. 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.3 중간 단계 D1: 실행 감독·소유 lease

| 검증 | 결과·한계 |
|---|---|
| 감독 경로7개 | argv/작업 위치·출력, 실제 부모 kill, 취소·늦은 쓰기, 독립 절대 deadline, 상속 lease, freshness context 통과 |
| race로 발견한 descriptor 경쟁 | 수정 후 같은7개 race 통과; Start/Wait가 파일 수명을 소유 |
| 전체 Go | 3,789사례=3,770통과+19skip·실패0 |
| 관련6패키지 race | 763사례=761통과+2skip·실패0; CLI setup도 포함 |
| 실제 공개 회귀 | CKS 내부 감독 명령 사용28항목·90도구호출·99wire쌍·정상 EOF5개 |
| build/fmt/vet/lint/문서·증거 | 통과; 계약20개·안내3개·live127개·증거 보호13개 |
| owned staging 정리·전체 D/E | 미구현·미검증; 전체 완료 수6/28 유지 |

[D1 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-command-supervision/manifest.json)과 [실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)를 따른다. 현재 소스1,679입력은 유지 Go1,249개·기타430개다. 다음은 D2의 owner token/프로젝트/후보와 결속된 staging 정리와 quiescence 판정 연결이다. 이후 E 실제 공개 재시작·lease/rollback/GC → KM-02.4 → KM-03.1–4 → KM-04.1–4 → KM-05.1–4 → KM-06.1–4 → OE-01.1–4를 진행한다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도 유지한다.


## KM-02.3 중간 단계 D2: 소유 staging 정리

| 시험 범위 | 결과 | 완료 판정에 쓰는 의미 |
|---|---|---|
| 전체 Go | 3,811사례: 3,792통과·19skip | 현재 소스 회귀, skip은 통과로 계산하지 않음 |
| 관련6패키지 race | 785사례: 783통과·2skip | setup/maintenance/MCP/config/CLI 경합 |
| maintenance | 144사례 통과 | 이전141 + v1/v2 writer 해제·정리 경계3 |
| 실제 프로세스 kill | record/디렉터리/owner/부분 자료4경계 통과 | 후보 안의 exact owner scratch만 정리 |
| 잘못된 소유/링크/경로 | 10변형 거부·witness 보존 | 다른 자료의 자동 삭제 방지 |
| 실제 CKS/CKG/CKV·MCP | 고정28항목·5연결 EOF0 | 공개 process-lifetime 회귀, durable 완료 증거 아님 |
| build/fmt/vet/lint·문서 | 통과 | 현재 코드/명령 계약 |

[D2 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-owned-staging/manifest.json)과 [실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)를 따른다. 현재 소스1,682입력은 유지 Go1,252개·기타430개이며 원래 승인5개 입력은 바꾸지 않는다. D1은1283a22c 당시 snapshot에서 검증하고 현재 Go 결과를 재사용하지 않는다. 다음은 E 실제 공개 durable 재시작/replay·취소/중단·rollback/read lease/GC이며 KM-02.3은 진행 중, 완료 수6/28이다. 이후 KM-02.4, KM-03.1–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4를 진행한다. 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.3 중간 E1: 공개 durable host 연결

| 검증 | 결과 | 경계 |
|---|---|---|
| 실제 CKS·CKG·CKV/MCP | 43고정 항목 통과 | mock embedding, 품질/독립 수용 아님 |
| 실제 graph 중 부모 SIGKILL | 2회 -9 | interrupted와 accepted cancel의 canceled를 구분 |
| 나머지 연결 EOF | 7회0 | 활성 하위 작업의 취소 의도를 먼저 저장·정리 대기 |
| 원래 요청 replay/terminal | 재시작 후 불변 | mutable source/base 변경에도 다시 구축하지 않음 |
| 전체 Go | 3,812사례: 3,793통과·19skip | 현재 소스 |
| 관련6패키지 race | 786사례: 784통과·2skip | 정상 종료 수정 회귀 포함 |
| build/fmt/vet/lint·문서 | 통과 | 현재 명령 계약 |

[E1 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/manifest.json)과 실행 명세를 따른다. 소스1,683입력은 유지 Go1,253개·기타430개이며 승인5개 입력은 그대로다. E2 실제 read lease/rollback/GC와 과거 publication 이력 보존은 미실행이며 세부6/28을 유지한다. 다음 E2 → KM-02.4 → KM-03.1–4 → KM-04.1–4 → KM-05.1–4 → KM-06.1–4 → OE-01.1–4이며 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.3 E2 완료: 실제 reader·rollback·GC

| 검증 | 결과 | 확인한 경계 |
|---|---|---|
| E1 실제 fixture 복원 | 입력 hash 결속 | 새 소유 경로, 합성 DB 아님 |
| 실제 MCP/CLI | 21항목 통과 | rollback·v4/v5 게시·reader 원문 보존 |
| 읽기 중 GC | 보호·계획 적용 후 조회 통과 | current/rollback/held/명시 보호 별도 |
| reader 해제·다시 연결 | 회수 가능·stale 계획 거부 | 수동 참조 수정 없음 |
| GC 실제 삭제 | v3 삭제·양수 byte 회수 | v1/v2/v4/v5 보존 |
| 삭제 후 기록·조회 | 과거 receipt/replay 불변·v3 시작 거부·v5 조회 | activation을 현재 serving으로 세지 않음 |
| 프로젝트 보존 | A v1·B held v1·history byte 불변 | 다른 프로젝트 영향 없음 |
| 실제 stdio 종료 | 5연결 EOF0 | 비정상 종료 숨김 없음 |
| 캐시 없는 전체 Go | 3,812사례: 3,794통과·18skip | 135패키지, 실패0 |
| 캐시 없는 관련6패키지 race | 786사례: 784통과·2skip | 실패0 |
| build/fmt/vet/lint·문서 | 통과 | 현재 코드 |

기존에 skip이던 TestDefaultRouteIP가 이번 실제 실행에서는 통과했다. raw Go 결과의 상태를 집계했으며 수동으로 skip을 통과로 바꾸지 않았다.

[E2 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-durable-retention/manifest.json)과 실행 명세를 따른다. E1은0fa42cef snapshot에서 별도 검증했다. 소스1,686입력은 유지 Go1,253개·기타433개이며 추가3입력은 E1 실제 fixture/manifest/원시 관찰이다. 원래 승인5입력은 바꾸지 않는다. 증거 보호12개는 wire·인용·계획·CLI 거부·기록/입력·skip·완료 집계 변조를 거부한다.

KM-02.3 완료로 전체7/28이며 다음은 KM-02.4의 공개 잘못된 대상/경로·graph/vector 부분 실패·의미 입력 불일치·timeout/cancel/shutdown 보호 검증이다. 남은21개는 KM-02.4, KM-03.1–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 별도 정책 생성기 보완·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-02.4 완료: 공개 실패 전체 matrix

| 범위 | 실제 결과 |
|---|---|
| project/profile/base·version·경로/명령·타 작업 조회/취소 | 공개 거부·current 보존 |
| graph 실패·graph 완료 후 vector 실패 | failed·부분 DB 활성화 없음·scratch 정리 |
| foreign semantic corpus commit/root | 실제 graph 뒤 실패·기존 원문 조회 보존 |
| 구축 중 source/profile 변경 | publication 거부 |
| 실제10초 deadline | failed/deadline_exceeded·writer 종료·scratch 정리 |
| 취소·EOF·SIGKILL | canceled/canceled/interrupted·기존 current·reader·다른 project 보존 |
| 사건별 byte/원문/인용 | 원래 DB/manifest/source/history hash 불변 |
| SQLite reader 공유 파일 | 별도 기록·WAL0byte·reader 종료 후 전체 목록/byte 복원 |
| 마지막 정상 요청 | 실제 v4 게시·기존 v3와 새 v4 reader 좌표/원문 분리 |
| 고정36항목·실제5연결 | 고의 SIGKILL1회 -9·EOF4회0 |
| Go 결과 | E2의1253 Go·모든 결속 입력 동일 후 전체3794통과/18skip·race784통과/2skip 참조 |
| 이번 별도 검사 | build/fmt/vet/lint·문서·증거 보호12개 |

[실패 matrix packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-failures/manifest.json)과 실행 명세를 따른다. 현재 소스1,689입력은 유지 Go1,253개·기타436개이며 추가3입력은 E2 manifest와 실제 module/race raw proof다. 원래 승인5입력은 그대로다. Go를 새로 실행한 것으로 기록하지 않는다. KM-02의4/4를 완료해 전체8/28·기능2/7이다. 남은20개는 KM-03.1–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 다음은 KM-03.1이며 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.


## KM-03.1 완료: 매니저 패키지·등록·진단

| 범위 | 결과 | 완료로 확대하지 않는 범위 |
|---|---|---|
| 등록 helper | 14개 시험 통과 | DB 구축/원문 문서 생성 아님 |
| project 격리/소유 | UUID·명시 owner·겹침/legacy/변조/링크 거부 | 이름/commit을 project 신원으로 쓰지 않음 |
| 실제 CLI strict validator | manifest/skill/agent3개 통과 | 모델 행동 검증 아님 |
| 실제 metadata bootstrap | 9항목 통과 | 2연결·정상 EOF·4개 도구/37개 schema |
| 소스/DB | 문서 없는 Go source 불변·dataset 미생성 | 초안/검토/reader 미구현 |
| Go proof | E2의1253 Go·모든 입력 동일 후 참조 | 새 Go 실행으로 세지 않음 |
| 이번 별도 gate | build/fmt/lint·문서131개·증거 보호10개 | 독립 수용/품질 아님 |

[bootstrap packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-bootstrap/manifest.json)과 [매니저 계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,697입력은 유지 Go1,253개·기타444개이며 추가8개는 plugin runtime/skill/agent/config/schema다. 승인5개 입력은 그대로다. 전체9/28·기능2/7 완료다. 남은19개는 KM-03.2–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 다음은 KM-03.2다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개도 별도 유지한다.


## KM-03.2 완료: 근거 수집과 proposed 지식 초안

| 검증 | 결과 | 경계 |
|---|---|---|
| 수집 단위 시험 | 23개 통과 | source 변경·비밀/링크·상한·타 프로젝트·변조·실제 Git diff |
| 기존 등록 회귀 | 14개 통과 | config/engine 소유 검증 유지 |
| 실제 두 프로젝트/native CKS | 24항목 통과 | 문서 없이 등록·수집·초안·validate/lock·재실행 |
| 근거와 변화 | origin/project/file/span hash 보존 | 같은 경로·코드라도 타 프로젝트 근거 분리 |
| 제공 자료 | 결정/명령 결과 보고 분리 | actor 없으면 unknown·명령 실행하지 않음 |
| 기본/사용자 주장 초안 | 모두 proposed·사람 승인 없음 | 실제 native questions overlay 구조/lock 검증 |
| source/DB | 원래 overlay/dataset 생성 없음 | adoption/검색/managed reader는 다음 단계 |
| Go proof | E2 입력1253 Go·모든 결속 byte 동일 후 참조 | 새 실행으로 세지 않음 |
| 별도 gate | build/fmt/vet/lint·문서·증거 보호13개 | 품질·독립 수용 아님 |

[수집 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-collection/manifest.json)과 [실행 계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,698입력은 유지 Go1,253개·기타445개이며 원래 승인5개를 보존한다. 이전 bootstrap은2c2b7ed1 snapshot에서 별도 검증했다. 전체10/28·기능2/7이며 남은18개는 KM-03.3–4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 다음은 KM-03.3이다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개도 별도로 유지한다.


## KM-03.3 완료: 질문·제공 검토·filtered input 채택/회수

| 검증 | 결과 | 경계 |
|---|---|---|
| 검토 단위 시험 | 35개 통과 | 질문·중복/해석 후보·CAS·동시 writer·제공 actor/이유 |
| 수집/등록 회귀 | 23개/14개 통과 | 기존 collection 신원·원문 byte·owner 보존 |
| 실제 CLI/native CKS | 24항목/25명령 통과 | 등록→수집→초안→통제 검토→채택→회수 |
| 검토 내구성 | 불변 event chain3개·재실행 동일 | 주입 I/O 실패의 게시 전/후 복구, 실제 SIGKILL 아님 |
| 선택 source/Git 변화 | 추가/삭제/byte/commit 변경 거부 | 이전 검토를 새 근거로 승계하지 않음 |
| 채택 입력 | 공개 source만 복사·accepted4/omitted3 | 원래 source·DB·제외 자료 불변 |
| native overlay | 실제 validate/lock·post-rename locked | policy/ADR 임의 생성 없음·검색 완료 아님 |
| 회수 | 이후 사용/이전 검토로 재채택 거부 | 자료 보존·게시된 DB rollback은 별도 |
| 검토 증거 | controlled_fixture 명시 | 외부 신원 인증/실제 사람 독립 수용 아님 |
| Go proof | E2 동일1253 Go·결속 입력 확인 후 참조 | 새 실행으로 세지 않음 |
| 별도 gate | build/fmt/vet/lint·문서·증거 보호14개 | 모델/온톨로지 품질 아님 |

[review packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-review/manifest.json)과 [계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,699입력은 유지 Go1,253개·기타446개이며 원래 승인5개를 보존한다. 이전 collection은322daf39 snapshot에서 별도 검증했다. 전체11/28·기능2/7이며 남은17개는 KM-03.4, KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 다음은 KM-03.4이며 정책 생성기·운영/독립 수용9개·선택 후속12개도 별도로 유지한다.


## KM-03.4 중간 A: 검토 지식의 실제 색인·retained 조회

| 검증 | 결과 | 경계 |
|---|---|---|
| 새 일반 문서 입력 | 명시 document_corpora/반복 --document-corpus | 의미 승인/기존 semantic export로 오인하지 않음·기본 비어 있음 |
| 신규 Go 회귀 | 26사례 통과 | root 순서·byte/add/delete/order·빈/링크/비 text·상한·취소·누락 색인 |
| 캐시 없는 전체 Go | 3,838사례=3,820통과+18skip·135패키지·실패0 | 이전 Go 결과 재사용 없음 |
| 캐시 없는 관련6패키지 race | 812사례=810통과+2skip·실패0 | 신규26사례는 위 분모에 포함 |
| 실제 manager 입력→공개 build | held 후보·필수 gate 통과·current 없음 | fixture 검토·mock embedding·품질 아님 |
| 실제 DB 자료 | 검토 note의 별도 docs origin/byte·Markdown chunks | private 원문 재유입 없음 |
| 실제 MCP 원문/인용 | 후보 좌표/project 일치·입력 삭제 후 동일 원문 | manager broker/실제 Claude host 미검증 |
| 실제14항목·3연결 | durable replay·EOF0/0/0 | 상태/게시/serving 구분 |
| 별도 gate | build/fmt/vet/lint·문서131개·증거14개 | 독립 수용 아님 |

[A packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-document-index/manifest.json)과 [계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,702입력=유지 Go1,256개+기타446개이며 승인5개는 그대로다. 초기 개발 실패와 최종 통과의 raw 결과를 구분해 보존했다. 이전 review checkpoint는3d39801c snapshot에서 검증했다. A만 완료했으므로 전체11/28·기능2/7을 유지한다. 남은17개는 KM-03.4(B adoption-bound 유지 보수/reader, C 실제 host), KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 정책 생성기·운영/독립 수용9개·선택 후속12개도 별도로 유지한다.


## KM-03.4 중간 B: 실제 매니저 도구·adoption-bound reader

| 검증 | 결과 | 경계 |
|---|---|---|
| broker unit | 29개 통과 | 실제 CKS·scope·source/회수/손상·CAS/lease·응답 손실 주입 |
| 수집/등록 회귀 | 23개/14개 통과 | 기존 owner/해시/typed refusal 유지 |
| 실제 plugin stdio | 26항목·3연결 EOF0 | 18도구; legacy ops/dummy context 미노출 |
| 실제 매니저 도구 연결 | collect/evidence/draft/review/adopt/select/build/status/reader/retract | 통제 응답·mock·DB 게시 없음 |
| 선택·조회 | adoption/source/input/model/job/후보 좌표 일치 | 초기화와 실제 읽기 관찰 분리 |
| 드리프트/회수/재시작 | 새 사용 거부·기존 admitted job만 exact resume | 새 요청으로 바꾸거나 current로 fallback 없음 |
| 열린 reader 손상 | dataset identity 손상 거부 | fixture 복구 후 같은 좌표 확인 |
| native CLI | strict manifest/skill/agent3개 통과 | 실제 Claude host 모델 흐름은 C |
| Go proof | A의1256 Go·의존/fixture byte 동일 후 참조 | 전체3820통과/18skip·race810통과/2skip, 새 실행 아님 |
| 별도 gate | build/fmt/vet/lint·문서131개·증거14개 | 독립 수용/품질 아님 |

[B packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-broker/manifest.json)과 [계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다. 소스1,703입력=Go1,256개+기타447개이며 원래 승인5개는 그대로다. A는bb71aae3 snapshot에서 검증했다. 초기 native SDK `isError:false` 생략에 대한 시험 단언 오류는 initial-failures에 보존했다. B만 완료했으므로 전체11/28·기능2/7을 유지한다. 남은17개는 KM-03.4(C 실제 Claude host), KM-04.1–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 별도 정책 생성기·운영/독립 수용9개·선택 후속12개도 유지한다.

## KM-03.4 C: 실제 host·종료·복구

KM-03.4 C 완료: 실제 설치 Claude Code2.1.300을8회 실행해68개 도구 결과를 원시 MCP·실제 API 입력과 대조했다. 문서 없는 입력의 수집→제공 검토→filtered adoption→held build→실제 원문/인용과 재시작 조회, 근거 부족·타 프로젝트 거부, 엔진 누락, 진행 중 정상 종료·host 강제 중단 후 durable 취소/exact replay/새 구축·조회를 확인했다. 정상 매니저 종료는 모두0이며 엔진 누락의 시작 거부2회(각2)와 의도한 host SIGKILL1회(-9)는 별도로 보존했다. 실제 SDK SIGINT에서 매니저가-2로 끝나던 문제를 graceful 종료로 수정하고 SIGINT/SIGTERM 회귀를 추가했다. 등록14·수집23·검토35·broker31의103개를 CI manager-test 진입점에 연결했고 로컬 실행을 통과했다. 원격 CI 실행으로 주장하지 않는다. 전체 Go3,838사례(3,820통과·18skip), 관련6패키지 race812(810통과·2skip)를 새로 실행했으며 build/fmt/vet/lint·문서와 native strict3개·증거 보호16개를 통과했다.

| 실제 host 사례 | 결과 | 범위 |
|---|---|---|
| full / restart |18+2개 도구, held 후보 원문·인용 일치 | 게시·모델 품질 아님 |
| insufficient |1개 실제 도구가 근거 부족 거부 | 지식 발명 없음 |
| missing engine |2회 시작 거부·MCP failed | 정상 종료 실패와 분리 |
| active-stop / recovery |12+12개 도구·취소 이력과 exact replay·새 구축/조회 | 실제 정상 host 종료 |
| killed host / recovery |12+11개 도구·매니저/graph 정리·새 구축/조회 | host만 SIGKILL, 매니저0 |

로컬 시험 응답기는 실제 host가 호출할 동작만 제공하고 MCP 결과는 실제 매니저/CKS에서 받았다. 모델 호출·비용은0이며 자율 에이전트 성능·의미 품질·독립 사람 수용을 판정하지 않았다. strict MCP fixture는 plugin 선언의 실제 launcher를 명시 연결했다. 처음의 strict discovery 설정·실제 SIGINT 종료 오류·시험 parser/retry 단언 및 test-only ResourceWarning은 initial-failures에 보존했다. 원본 source와 기존 current는 수정하지 않았다.

[C packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-host/manifest.json)에 새 Go 원시 결과·전체 사례 목록·103개 로그·8회 raw trace를 보존한다. 현재 소스1,703입력=Go1,256+기타447이며 승인5입력은 그대로다. B는c51f7423 snapshot에서 검증했다. 전체12/28·기능3/7 완료다. 남은16개는 KM-04.1–4(정기/변경 갱신), KM-05.1–4(공통 개념 매핑), KM-06.1–4(비교 근거 MCP), OE-01.1–4(온톨로지 효과 평가)다. 다음은 KM-04.1의 trigger·성공 기준·변경 pin 설계다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 이어간다.

## KM-04.1 명시 정책·trigger·성공 계약

KM-04.1 완료: 명시 정책·수동/정기/변경 polling·clock skew/놓친 일정 합치기·등록/프로젝트/명시 corpus 범위·재시도 상한·required gate와 held/게시 성공 계약을27개 반례로 검증했다. 문서 없는 source의 자동 관찰 입력과 사람의 의미 검토를 분리하고 변경된 verified 지식의 무조건 승계를 금지했다. 이 단계는 설계/정책 판정이며 scheduler 실행·자동 DB 갱신 완료가 아니다. build/fmt/vet/lint·문서132개·증거 보호10개를 통과했다. Go/의존/fixture byte가 C와 같아3,820통과·18skip 및 race810통과·2skip의 committed 원자료를 참조했으며 새 Go 실행으로 세지 않는다.

[정책 계약 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-contract/manifest.json)에27개 전체 사례 이름·실행 로그와 실제 host C의 동결 검증을 보존한다. 소스1,704입력=Go1,256+기타448, 원래 승인5개는 보존한다. 전체13/28·기능3/7이다. 다음은 KM-04.2의 변경/삭제/no-op·자동 관찰 입력 구현이다. 남은15개는 KM-04.2–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 계속한다.

## KM-04.2 원문 변경/삭제와 자동 관찰 입력

KM-04.2 완료: 자동 관찰 입력·선택 파일의 추가/수정/삭제·same-byte rename 후보·명시 knowledge/ontology 원문·제공 기록·검토 stale/no-op pin을 구현했다. 사용자 source에 문서를 쓰거나 review/human_response를 만들지 않고 native knowledge init/validate/lock/validate를 수행한다. 정책27·입력32·기존 manager103의162개와 실제 공개 MCP16항목·4개 held DB·82호출·11연결 EOF0·증거 보호14개를 검증했다. 실제 두 번 변경·삭제 후 최신 원문/인용과 이전 snapshot/타 프로젝트 DB 보존, 실제 inspect/status/reader가 있는 무변경 판정을 확인했다. 메타데이터/input equality만으로 DB no-op을 승인하지 않는다. 이 실행은 수동 검증이며 주기 worker 검증이 아니다. build/fmt/vet/lint·문서132개를 통과했고 Go1256개·의존/fixture byte가 C와 같아 원래3,820통과/18skip·race810통과/2skip을 참조했다. 새 Go 실행으로 세지 않는다.

[관찰 입력 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-inputs/manifest.json)에162개 로그·59개 새 계약/입력 사례 전체 이름,4DB의 실제 MCP 원시 입출력과 소유 fixture를 보존했다. 소스1,705입력=Go1,256+기타449이며 승인5개는 보존했다. 기존 검토의 실제 collection_stale에 대한 초기 시험 단언 이름 불일치는 initial-failures에 남겼다. 전체14/28·기능3/7이며 다음은 KM-04.3의 durable intent·한 writer·재시작/retry·실제 worker와 gated 전환이다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도 유지하며 멈추라는 요청 전까지 단계별 서명 커밋으로 계속한다.


## KM-04.3 중간 A: 실제 원문 전환 전 확인

KM-04.3 A 원문 전환 전 확인 완료: operator profile의 선택적 source_check로 실제 Python/코드·소유 정책/등록/불변 입력을 고정하고 admission/후보 완료/게시 pointer lock 안에서 원래 source fingerprint를 재확인한다. 실제 MCP14항목·90호출·3개 job에서 v1 게시→v2 원문 변경으로 게시 거부/current·기존 reader 보존→v3 복구 게시/조회와5연결 EOF0을 확인했다. Go 전체3,862사례(3,844통과·18skip), 관련6패키지 race836(834통과·2skip)을 새로 실행했고 Python174개·증거 보호14개·build/fmt/vet/lint·문서132개를 통과했다. 추가 source-check23사례와 이미 지난 deadline1사례는 전체/race 분모 안에 포함하며 중복 합산하지 않는다.

처음에는 원문 probe가 native 접수 중의 아직 root owner가 없는 소유 journal과 충돌했다. 원문 전용 probe만 읽기 확인과 DB 소유 검증을 분리했고 기본 등록·doctor·prepare·DB 쓰기의 소유 검증은 유지했다. Native 오류의 실제 structuredContent를 읽도록 검증기도 수정했다. 최초 race의 기존 deadline 시험1회 late-writer 실패는5회 반복에서 재현되지 않았다. timer callback 도착만 가정하지 않도록 exec 직전/직후 절대 시각 확인을 보강하고 과거 deadline 거부 시험을 추가했다. 두 경계를 각각5회 확인한 뒤 최종 전체·race를 순차 실행해 실패0을 확인했다. 최초 실패의 단일 원인을 입증했다고 주장하지 않으며 initial-failures의 원자료를 보존한다.

[원문 확인 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-source-check/manifest.json)을 따른다. 현재 소스1,707입력=Go1,258+기타449이며 승인5개는 보존했다. 직전2c03322f의 입력 검증을 동결 snapshot에서 별도 확인했다. A는 native 원문 검사 범위다. KM-04.3의 B durable worker/일정/중복/retry와 C 중단/복구 연결, KM-04.4 실제 주기 실행은 남는다. 전체14/28·기능3/7을 유지한다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 다음 B를 진행한다.

## KM-04.4 최종 시험

KM-04.4 완료: 실제 두 프로젝트에서20항목·MCP221호출·정상 EOF2개를 검증했다. A는 periodic/change로7→8→9→10을 게시하고 문서·명시 ontology 원자료 root 삭제와 제공 메모 교체를 반영했다. 실제 graph 실패1건은 current/마지막 성공을 보존했고 같은 입력의 연결된 attempt2가 복구했다. no-op은 중복 job을 만들지 않았고 v1 코드/삭제된 자료의 당시 인용을 계속 조회했다. 동명 module/Value를 가진 B의 DB 전체 파일 hash·current·실제 본문/인용은 불변이었다. 비공개 자료 제외·자동 의미/사람 검토 미생성도 확인했다. 이는 mock embedding의 실제 lifecycle 검증이며 검색 품질·온톨로지 효과·독립 수용은 별도다.

전체16/28·기능4/7 완료다. 다음은 KM-05.1의 산업 개념·정의·별칭·포함/제외·비교 축 설계다. 남은12개는 KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 계속한다.

## KM-05.1 개념 팩 시험 목록

최종 실행 결과는 단계 완료 시 원자료와 함께 기록한다. 아래29개는 Go의 상위/하위 실행 수이며29개의 독립적인 산업 사실이나 품질 정답 수가 아니다.

| 상위 시험 | 하위 포함 실행 수 | 검증할 경계 |
|---|---|---|
| TestShippedSchemePinsQuestionsAndDefinitions | 1 | 8개념·26축·정의·정확 pack/version/digest |
| TestBilingualAliasesAndAmbiguityAreExplicit | 1 | 한영/대소문자·두 의미 후보·미등록 문장 unknown |
| TestSchemeLookupNeverReturnsMutableOwnedSlices | 1 | 반환값 변경으로 이후 조회 오염 금지 |
| TestSchemeByteChangesAlterExactPackIdentity | 1 | sidecar 변경은 새 identity; 옛 선택 어휘 불변 |
| TestInvalidSchemeBoundaries | 14 | 버전/pack/외부 개념·누락/중복·경계/역할/질문/조건 |
| TestStrictSchemeJSONRejectsAmbiguousDocuments | 6 | 중복 key·unknown key·후행 문서·크기/깊이 |
| TestSchemeDoesNotFollowRootOrDefinitionSymlinks | 1 | root/sidecar 링크 거부 |
| TestSchemeReusesPackDependencyAndTypeResolution | 4 | 미등록 의존·없는 core 참조·상속 순환 거부 |


## KM-05.1 공통 개념 검증

KM-05.1 완료: 기존 knowledgepack loader를 재사용한 선택형 industry.blockchain1.0.0 팩에8개념·26비교 축을 추가했다. 한영 별칭·포함/제외·역할/버전/network/config/시점 조건, 정확 byte pin과 unknown/ambiguous 계약을29개 Go 사례로 검증했다. 단어의 exact_term 일치를 구현 매핑 승인이나 project/entity 동일성으로 해석하지 않는다. 공통 core20·운영 dataset·기본 자동 선택은 변경하지 않았다. 프로젝트 연결·비교 MCP·ontology 품질은 남는다.

전체17/28·기능4/7이다. 다음은 KM-05.2의 프로젝트별 코드·문서·설정/정책·테스트 근거 연결이다. 남은11개는 KM-05.2–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-05.2 프로젝트 연결 시험 목록

공통 어휘29개에 아래30개를 더한 비교 패키지59개다. 최종 전체 실행이 끝나기 전에는 세부 작업 완료로 확정하지 않는다. 원문 보관 fixture를 실제 graph/vector 검색 품질로 확대하지 않는다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestDifferentNamesMapToSeparateProposedSourceGroups | 1 | Stake/Bond·각5종류 원문·프로젝트별 인용·proposed |
| TestBindingSchemaRejectsUnscopedOrInventedClaims | 21 | 20개 schema/scope/hash/facet 거부와 중복·가짜 verified JSON |
| TestForgedHashAndWrongCollectionReturnNoEvidence | 1 | 재서명 hash·미선택 collection·취소·종료 이후 조회 거부 |
| TestMappingAnnotationsAreSanitizedAndDigestTracksInterpretation | 1 | 역할/이유/조건 정화·변경 digest·반환 slice 격리 |
| TestSameLocalNameDoesNotChooseTheSameCommonConcept | 1 | 같은 Stake 이름의 stake commitment/validator identity 역할 분리 |
| TestBindingReadsRetainedSourceAndRejectsArchiveCorruption | 1 | live 변경 이후 당시 원문 유지·보관 blob 손상 거부 |
| TestBindingsKeepCommittedAndWorkingTreeCitationSemantics | 3 | committed/미커밋 원문의 commit/base 구분 |
| TestBindingIDsAreScopedByProjectVersionAndConditions | 1 | 같은 로컬ID·다른 프로젝트/network·동일 시각 표현 중복 |


## KM-05.2 완료 기록

KM-05.2 완료: 명시 common concept/version/digest와 프로젝트·버전·network/config/역할/관찰 시점에 결속한 원문 연결을 구현했다. 서로 다른 이름의 같은 역할과 같은 이름의 다른 역할을3개 프로젝트의5종류 원문으로 검증하고, 동일 로컬ID를 서로 다른 scope에 보존했다. 기존 collection reader lease·target 검증과 retained evidence 정화/재검증을 재사용한다. code/document/configuration/policy/test와 원문 hash/span을 구분하며 모든 연결은 proposed다. 실제 사람 승인·그래프/벡터 검색 품질·공개 비교 MCP는 완료로 세지 않는다.

전체18/28·기능4/7이다. 다음은 KM-05.3의 직접 매핑 의미·검토/회수·버전/stale 정책이다. 남은10개는 KM-05.3–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-05.3 직접 매핑·검토 시험 목록

기존59개에 아래34개를 더한 비교 패키지93개다. 통제된 검토 기록으로 정책을 검증하며 실제 사람 승인·독립 품질로 세지 않는다. 최종 전체 회귀 후 단계 완료를 확정한다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestDirectMappingMeaningNeverMergesEntitiesOrInfersClosure | 6 | 5종류 직접 방향·broader/narrower 범위·entity/전이 금지 |
| TestMappingReviewQuorumWithdrawalConflictAndTime | 15 | 14개 승인/중복/정족수/철회/상충/최신/미래 판정 |
| TestMappingReviewsAreExactPinsAndProjectLocal | 1 | 해석 변경 stale·타 project/network 기록 미승계·위조 header 거부 |
| TestReviewedBindingChecksCurrentArchiveAndKeepsHistoricalVersion | 1 | 새 버전 stale·당시 retained 승인/원문 유지·손상은 승인으로 우회 금지 |
| TestMappingReviewDoesNotMigrateAcrossPackVersions | 1 | 공통 팩v2의 새 검토 요구·옛v1 선택/검토 보존 |
| TestMappingReviewStrictSchemaAndNoMissingActor | 9 | 검토자/이유/판정/시각/target/source/namespace8개·변조 이벤트/정족수 거부 |
| TestMappingReviewRejectsCaseAliases | 1 | 대소문자·유니코드 속성 별칭 덮기 거부 |


## KM-05.3 완료 기록

KM-05.3 완료: exact/close/broader/narrower/related를 local→shared 직접 관계로 고정하고 entity 동일시/자동 전이를 금지했다. 정확 source/해석/팩 pin의 검토 기록, 중복 검토자 정족수·최신 판정·회수·충돌·미래 시각·stale와 과거 버전 보존을 구현했다. 공개용 reader는 보관 원문을 재확인하며 승인 기록으로 손상을 우회하지 않는다. 검토자 label은 인증된 신원이 아니고 실제 사람 승인은 생성하지 않았다. 입력 필드의 대소문자/유니코드 별칭 덮기도 재현·거부했다.

전체19/28·기능4/7이다. 다음은 KM-05.4의 비적용/미수집/미지원·역할/산업/network/config 차이 검증이다. 남은9개는 KM-05.4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-05.4 적용·지원 범위 시험 목록

기존93개에 아래24개를 더한 비교 패키지117개다. source archive/fixture identity 기반이며 실제 graph/vector 색인 또는 산업 사실의 독립 정답 수가 아니다. 최종 전체 회귀 이후 완료로 확정한다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestCoverageSeparatesCollectionUnknownCapabilityAndScope | 13 | 12개 미수집/미검토/동일·유사 후보/누락 축/network/config/산업/역할/capability |
| TestApplicabilityDeclarationRequiresVersionedSourceAndReview | 1 | 출처 있는 비적용·schema2·기본 미검토 숨김·명시 후보/기록 승인·상태 변경 digest·code-only 거부 |
| TestCoverageWithinProjectConflictDoesNotFlattenProjectDifferences | 1 | 같은 project/scope의 반대 선언과 다른 프로젝트의 정책 차이 분리 |
| TestRustTextAndL2OrderingRemainDistinctFromUnsupportedASTOrConsensus | 1 | Rust 보관 원문 가능·AST 미지원·L2 ordering/L1 consensus 분리 |
| TestSchema1DigestAndCrossScopeRejectionsStayStable | 1 | schema1 digest byte 호환·foreign target/없는 facet 거부 |
| TestCoverageDiscardsEvidenceWhenRetainedArchiveBecomesUnavailable | 1 | 보관 손상 시 원문/정상 상태 반환 금지 |
| TestDeclaredUnknownAndRejectedReviewsNeverBecomeFeatureAbsence | 6 | 5개 미결정·기록 검토·거부/철회/상충은 기능 부재가 아님 |


## KM-05.4 완료 기록

KM-05.4 완료: exact/similar 후보·incomparable·not_collected·unknown·unsupported·not_applicable·conflict·unavailable을 분리했다. 다른 산업/역할·L1 consensus/L2 sequencing·network/config를 구분하고, Rust 원문 읽기와 Rust AST 미지원을 혼동하지 않는다. 비적용/미결정 선언은 source/조건/검토와 함께 처리하며 검색 무응답·거부/회수를 기능 부재로 바꾸지 않는다. schema2 선언을 digest에 포함하고 기존 schema1 식별값은 유지했다. 상충은 같은 프로젝트/조건 안에서만 판단하며 보관 손상은 결과를 폐기한다.

전체20/28·기능5/7 완료다. 다음은 KM-06.1 비교 요청·MCP 계약 설계다. 남은8개는 KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-06.1 공개 비교 요청 계약 시험 목록

기존117개에 아래50개를 더한 비교 패키지167개다. 계약 검증 fixture는 dataset/backend를 열지 않으며 실제 serving readiness/MCP 실행 근거가 아니다. 최종 전체 회귀 후 완료로 확정한다.

| 상위 시험 | 하위 포함 실행 수 | 경계 |
|---|---|---|
| TestComparisonContractRoutesOnlyExplicitParticipantsAndOwnsValues | 1 | 명시 participant·기본 후보 제외·입력/출력 slice 소유·질문은 데이터 |
| TestComparisonRequestRejectionsBeforeAnyBackend | 27 | 26개 schema/mode/scope/collection/concept/facet/time/budget/pin 거부 |
| TestComparisonRequestDecodeRejectsPathsAndAliasOverrides | 6 | 5개 설정/원천/target·case/중복 필드 거부 |
| TestScopeIdentityIncludesVersionConditionsAndKeepsAliasesSeparate | 7 | 6개 project/dataset/snapshot/network/config/role 차이·동등 시각·중복 등록 거부 |
| TestSameProjectExplicitNetworkComparisonIsAllowed | 1 | 같은 project/version·다른 명시 network scope 보존 |
| TestComparisonProfileDecodeIsConfigurationShapeNotReadiness | 8 | 7개 profile 형상 반례·파일이 없어도 readiness로 표시하지 않음 |


## KM-06.1 완료 기록

KM-06.1 완료: 읽기 전용 comparison 모드·고정 profile/catalog·명시 scope 참여자/공통 개념/축·시각/예산 계약을 구현했다. 이름이나 자연어에서 대상을 추측하지 않고2–16개 scope와1–8개 축을 선택한다. stale/타 collection/동일 범위 중복·구현 모드·요청 경로/target 변조·미래 시각/초과 예산을 거부한다. 한 프로젝트의 명시적인 다른 조건도 비교 가능하며 호출자 변경으로 고정 라우팅을 바꾸지 못한다. 이 단계는 계약50개 검증이며 actual source read/MCP 호출0이다.

전체21/28·기능5/7이다. 다음은 KM-06.2의 공개 MCP·프로젝트별 근거 행렬/bundle 구현이다. 남은7개는 KM-06.2–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.2 완료 기록

KM-06.2 완료: 고정된 로컬 profile/catalog·선택 pack/연결/검토·정제 규칙의 실제 byte digest를 고정하는 loader와 독립 stdio 비교 MCP를 구현했다. describe/extract2도구만 노출하고 읽기 전용·비파괴·동일 요청 반복 가능·외부 네트워크 비사용 표시를 명시한다. 각 프로젝트/조건·개념/축의 source 종류·본문/인용·매핑 이유/검토 상태·unknown을 분리하며 양쪽 근거가 있는 공통/차이 후보만 제공한다. 기존 매니저18/updates6도구에 자동 추가된 것으로 세지 않는다.

비교171개(계약167+runtime4), 전체 Go4033(4015통과18skip)·race1007(1005통과2skip,7패키지)·빌드/fmt/vet/lint·문서134개를 검증했다. 실제 native mock2DB 구축 후12항목·9도구호출·2세션 EOF0이며 유료 모델0이다. 통제 source fixture의 실제 조회/transport 검증이며 실제 체인 비교·의미 품질/독립 수용은 아니다.

전체22/28·기능5/7이다. 다음은 KM-06.3 부분 실패·상충/누락·기한/예산·reader/종료 보호의 완성이다. 남은6개는 KM-06.3–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.3 완료 기록

KM-06.3 완료: 모든 참여자를 읽은 뒤 응답 직전에 각 archive를 다시 검증한다. 뒤 프로젝트를 여는 동안 앞 archive가 바뀌면 해당 프로젝트의 본문/인용·후보만 폐기하고 나머지 프로젝트와 명시 좌표는 보존한다. 같은 scope의 적용/비적용 상충과 다른 프로젝트의 차이, 미수집/unknown/손상 상태를 구분한다. 후보 proof의 복사 인용까지 max_citations에 포함하며 예산에 맞지 않으면 후보 전체를 제외하고 omitted_candidates/candidate_state로 알린다. 요청 기한/취소에는 이전 부분 성공을 공개하지 않고 Close는 진행 중 reader 정리를 기다린다.

비교192개(기존171+보호21,47개 상위), 전체 Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint·문서134개를 검증했다. 실제 mock2DB·기반12+보호9항목·18도구호출·4세션(EOF3개와 입력이 열린 SIGTERM1개 모두 exit0)이다. 원문 손상 후 정확히 복구했고 DB/manifest/profile/catalog/binding 전체 hash를 보존했다. 실제 체인 비교·의미 품질/독립 수용은 아니다.

전체23/28·기능5/7이다. 다음은 KM-06.4 둘 이상 실제 프로젝트의 고정 DB·staking/정책·consensus/finality·mempool/ordering 비교 검증이다. 남은5개는 KM-06.4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.4 완료 기록

KM-06.4 완료: go-stablenet0bf2f4d1의committed 원문2,224파일과 go-wemix1350376a의working-tree 원문2,019파일로 실제 native graph/vector DB를 구축했다. go-wemix의 cmd/gwemix→geth alias만 소유 복제본에서 제외하고 BaseCommit·제외 목록·준비 Git 상태를 기록했다. 비교하는7개 Go 파일은 원본 커밋 바이트와 동일하며 원본 저장소는 변경하지 않았다. 모든 매핑은 미검토 후보다.6개 개념·40개 프로젝트/축 셀에서7개similar·18개unknown·15개not_collected를 관찰했다. 이는 선택한 매핑/근거의 범위이며 실제 네트워크나 기능 부재·finality·TPS·보안성·우월성의 판정이 아니다.

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. 새 Go 변경이 없음을1273개 Go와 모든 결속 입력의 hash로 확인해 직전 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 실제 corpus/MCP 시험과 과거 Go 실행을 구분한다. 유료 모델 호출0이며 ontology 효과/독립 품질 수용은 아니다.

전체24/28·기능6/7이다. 다음은 OE-01.1의 새 질문·기대 근거·평가 범위/분모 고정이다. 남은4개는 OE-01.1 질문/기대 근거, OE-01.2 동일 자료4모드, OE-01.3 근거·순위·unknown·지연/비용, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지하며 단계별 서명 로컬 커밋으로 계속한다.


## OE-01.1 완료 기록

OE-01.1 완료: 등록 용어의 한영 표현·다른 용어·다의어(validatorMu/currentMutex)·주석/구현 차이·범위 밖을 포함한 새16질문을 사전 등록했다. 함수 찾기14질문은5사실군·고유 함수11개의 완전한 source span을 기준으로 top10의 함수 회수 precision/recall·순위를 계산하며, 제어2질문은 일반 source relevance를 채점하지 않고 ontology 의도 범위와 원문 무결성을 관찰한다. 일반 행동/정책의 완전한 관련 근거나 답변 정답을 뜻하지 않는다. 기준은 assistant source-derived이며 사람/독립 gold 승인은0이다. 이전13/42문항을 새 독립 효과 입력으로 재사용하지 않고, 기존 사실군과 표현 변형의 중복도 명시한다.

source commit baca2869·동일 비시험 Go882개·승인 정의4개/검토된 구현 연결0개·BGE-M3 digest790764642607…16bab/1024차원을 고정했다. 현재 로컬 Ollama0.40.4를 기록하므로 과거 서버 버전의 절대 지연을 새 관찰과 합산하지 않는다. 동일 자료/모델/모드 외 설정과 ontology mode만 바꾸는4arm, 질문별 준비1/검색2/warm5·순서 교대·총512회 계획이다. 사전 등록 단계의 새 검색/LLM 답변/유료 모델 호출은 모두0이다. 후속 실행은 소유 loopback proxy로 API 경로·해시/bytes·상태·시간·서버가 제공한 token count를 계측하고, bytes에서 token/금액을 추정하지 않는다. 작은 표본은 기술 통계로 보고한다.

전체25/28·기능6/7이다. 다음은 OE-01.2의 고정512회4모드 실행이다. 남은3개는 OE-01.2 동일 자료4모드, OE-01.3 함수 근거·순위·ontology 범위/unknown·지연/API 호출/관측 비용, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 기본 off·quality=inconclusive는 실제 품질 승인 없이 바꾸지 않는다.


## OE-01.2 완료 기록

OE-01.2 완료: 사전 등록16질문×4모드×8호출=512회를 실제 로컬 BGE-M3로 수집했다.4개 readiness와64개 capture 세션의 종료, 질문/모드/단계/반복 슬롯·설정/바이너리·source/snapshot·인용/본문/무결성 hash를 검증했다. 검토된 정의4개/구현 연결0개를 보존하고, mode 외 설정은 동일하다. 원본 graph/vector/source·의미 DB·바이너리·모델/서버가 전후 동일하며 소유 loopback API 계측기는 진행 중 요청0으로 종료했다. 이 단계는 실행/원문 검증이며 효과 점수나 품질 수용이 아니다.

인용4896개·본문4896개·원문 blob65개를 검증했고 실패/누락0이다. 실제 API 20420개를 metadata/준비/질문-arm별로 기록했다. 기본 off·quality=inconclusive와 사람/독립 gold0·실제 LLM 답변0·유료 모델0을 유지한다. 초기 off 기본값 정규화 오류와 계측 로그 전체 재작성에 따른 제어 중단을 별도로 보존했다. 초기 지연/부분 실행은 본512회 지표에 합산하지 않는다.

전체26/28·기능6/7이다. 다음은 OE-01.3의 고정 함수 회수 precision/recall·순위·ontology 의도 범위/unknown·warm 지연/API 호출/관측 비용 재계산이다. 남은2개는 OE-01.3 분석, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다.


## OE-01.3 완료 기록

OE-01.3 완료: 고정512회 원자료에서 유한한 완전 함수 찾기 과제의 precision/recall·순위·의도한 ontology 개념/범위 진단·warm 지연·실제 로컬 API 호출/서버 token 수를 재계산했다.5사실군 평균 recall은 off54.44%→concept_text/combined56.67%, precision12.87%→13.54%, MRR0.4612→0.4862다. 개선은 EX-08 영어 메시지 처리 질문1개에서 prepare만 회수하던 결과에 commit이 추가된 것으로 한정된다. 나머지13개 채점 질문의 지표는 같았다. hit@10은68.33%로 동일하다. 일반 행동/정책의 semantic relevance나 답변 정확도로 확대하지 않는다.

각 arm의 warm80개 p95는 off583.1ms, concept_text736.2ms(1.263배), relations607.4ms(1.042배), combined697.4ms(1.196배)다. capture batch embedding API는 off/relations1584개·31536서버 prompt tokens, concept_text/combined1664개·56112tokens다. 준비/모델 확인도 포함한 도구 실행 batch 비용이며8호출×16질문의512개 독립 사실이 아니다. 토큰은 서버 보고값이고 금액/전력/하드웨어 비용은 추정하지 않았다. 유료/LLM 답변0이다. 활성 모드는 의도 개념13질문 중9개를 매칭했고, scope 의도 진단3질문 중 외부 go-wemix 대상 EX-16에도 WBFT quorum을 매칭했다. relations는 검토된 구현 연결0·rank 변화0이다. 정의 검색/combined의 최종 인용 순서는32짝 중18짝에서 달랐지만 함수 회수 이득은1질문뿐이다. 작은 source-derived 개발 표본과 지연 관찰로 기본 off·quality=inconclusive를 유지한다.

전체27/28·기능6/7이다. 다음이자 마지막 세부 작업은 OE-01.4의 실제2프로젝트 공통 개념 매핑 효과·오인/누락 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 실제 LLM 답변·독립 정답·운영 활성화는 미실행/미승인 상태로 보존한다.

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
