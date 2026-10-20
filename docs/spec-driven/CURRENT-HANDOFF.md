# 현재 인수인계 — 실제 native broadcast·RPC callback 종료 연결 / 2026-10-20

새 머신/새 세션에서는 [머신 간 인수인계](./MACHINE-HANDOFF.md)를 먼저 따라 증거 태그·실행 도구·외부 DB/모델·삭제된 임시 경로를 확인한다.

단계90 요청 범위의 연결 검증을 완료했다. [명세](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SPEC.json) · [결과](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json) · [회귀·시도 이력](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/VALIDATION.json) · [자원 정리](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/CLEANUP.json).

## 현재 단계 — 일반/race10슬롯·실제20노드 종료·회귀121종 통과

|역할|일반/race 관측|정리와 판정|
|---|---|---|
|원본·재도입 별칭|4건 모두 같은 구독의 native 상태 송신과 Unsubscribe 송신 쌍 대기; 삭제/ack/반환0|관측 고정 후 소유 status 수신으로 실제 owner를 진행; rescue를 후보 성공으로 세지 않음|
|후보 A/B|4건 모두 실제 native 상태1개 drain→owner delete→ack→method/callback 반환|정리 개입0; gate 아래 finite 경로이며 독립 후보 승인 아님|
|조기 반환 대조군|2건 모두 callback/method 반환은 있지만 owner 삭제/ack0, native 송신 pending|관측 후 소유 status 수신+실제 owner manual uninstall로 복구; 가짜 ack0|

실제 node.New/eth.New/Start로 슬롯마다 두 노드를 만들고 loopback peer가3블록을 실제 동기화했다. 선택한 factory API의 downloader/blockchain/mux 포인터가 실제 receiver 객체와 같고, API/channel/RPC 구독 ID를 trace 전체에 결속했다. Start/Done 이벤트와 진행 높이를 주입하지 않았다. 활성 indexer와 동일 head를 확인했다. 원본60초 타이머는 유지했고 선택 receiver의 관측 나이는 60.0008~60.0018초였다.

송신 직전의 선언된 gate에서 같은 세션의 실제 eth_unsubscribe를 호출했다. true 응답을 받는 동안 owner 삭제/ack는0이고, 실제 Syncing의 rpcSub.Err 분기와 deferred Unsubscribe 진입을 확인한 뒤 gate를 열었다. 이후 원본의 대기와 A/B의 native drain·실제 map 삭제/ack를 같은 trace에서 구별했다. 원본·별칭의 frozen stack은 native send와 uninstall send를 같은 실제 callback 경로에서 보여준다. wire true를 내부 해제 완료로 바꾸지 않았다.

이 gate와 관측 hook은 실행 순서를 통제한다. 완료 상태 false broadcast와 RPC 해제 Err 분기에 한정된 검증이다. 진행 중 true 알림·transport close/Notify 중 종료·무계측 발생 확률/무한 생산자 fairness·모든 post-ack channel-use·독립 품질 수용을 주장하지 않는다. legacy PoW ethash 시험이며 운영 WBFT 검증이 아니다. 실제 owner4/별도 통제 late1 및 옛 구독·새 등록 전이라는 기존 승인 범위를 유지한다.

소스0bf2f4d1bfeb의 고유 후보4개와 원본 별칭을 구분했다. 기존 파일 변경은 선언한 API 관측 hook/후보 method뿐이고 bridge/main2개를 추가했다. full source map·project/build Git tree·일반/race8바이너리·실행10개를 결속했다. 최초 보조 코드 빌드는 ack 채널 타입 오류로 실패했고 실제 노드 실행0이다. 해당 한 곳을 원본 chan interface 타입에 맞춘 corrective1에서 전체10슬롯을 실행했다. 최초 소스/실패 로그와 수정 범위를 모두 보존했다. 자동 재시도0이다.

지역 회귀54개와 준비 상태67개로 고유121종이 통과했다. 검증기 결속 강화 뒤 기존 manifest가 소스 변경을 탐지한 집중 검사1회 실패도 보존했고, 현재 해시로 재결속한 최종 집중9개는 통과했다. 원자료/실행 누락, 타이머 단축, RPC 분기 변경, owner ack 없는 완료, source drift와 독립 승인 승격을 거부한다. KS 운영 Go 코드는 그대로이며 이전 서명된 본체 전체 일반/race4273개씩의 같은 입력 근거를 유지한다. 이번에 본체 전체 suite를 새로 돌렸다고 주장하지 않는다.

관측 후 rescue status 수신6개·early 대상 실제 owner cleanup 요청2개를 별도 기록했고 가짜 ack0이다. 실제20노드가 닫혔고 소유 프로세스 잔여0·임시 데이터 잔여0·원본 checkout 보존을 확인했다. 증거 hash 확인 후 이번 캐시/초기·수정 시험 루트3개 약2.06GiB를 제거했다. 실제 LLM·유료 호출·운영 적용·게시0이다.

## 다음 작업

실제 owner ack 뒤 옛 구독의 모든 channel-use 관측과 명시적 새 등록의 세대 경계를 보완한다. 같은 channel의 새 구독은 허용되므로 영구 불사용으로 판정하지 않는다. 이후 독립 정답/충분성/동결과 승인 자료 기반 실제 역할 양성 oracle·최종 HEAD가 남는다.

## 남은 전체13묶음 — 상위 완료0/13·정식0/9

|묶음|남은 내용|
|---|---|
|WI-V1|옛 구독 ack 후 channel-use·새 등록 세대 경계·독립 티켓/oracle·승인 양성 역할/final HEAD|
|WI-V2|독립 oracle·수용 후보 집합·선정 판단|
|WI-6|독립 nonseed 관련성/잡음 정답·가족/표본 충분성|
|WI-SP|비보안 수동/자동 출처·policy/미태그 소비자와 독립 의미 검토; 보안 전용 평가는 사용자 보류|
|WI-C|독립 필수 항목/우선순위·새 사례|
|WI-1|값/종류/메모리 매칭·unknown 구분·독립 완전 정답|
|WI-2|독립 비보안 표본·호출 대상/호출자·명령/build/MCP 소비자 매칭|
|WI-3|가족/노출/중복·상한/partial/오류/범위 밖·독립 관련성|
|WI-4|실제 host/receiver 일정·반복·독립 가족/oracle/충분성|
|독립 수용 준비|익명 역할 미정·새 DEV/FINAL·완전 정답/매칭/충분성·F01~08 동결|
|온톨로지|독립 의미/검색 정답·구현/개념 연결·동일 corpus off/on 효과|
|정식9개 실행·수용|독립 승인·동결 후 WI-1/2/3/4/V1/V2/6/SP/C 실행|
|활성 후속9개|캐시 로컬 완료; 공개 형식·graph tuning·flow gold·규모·as-of·legacy 출처·viewer·배포/downstream 남음|

D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false다. 실명/연락처를 요구하지 않으며 익명 역할을 허용한다. CoreML은 사용자 제외로 완료 조건에서 뺐다. 보안 전용 평가는 사용자 보류다. 실제 LLM 답변 평가는 모델·인증·비용 상한 결정 후 진행한다.

## 보존 근거

[12계약·소비자12기준 검토](./operational-preparation/source-review-supplement/followups/wiv1-contract-consumer-review-v1/REVIEW.json) · [기본 고정 소스·종료 시한 진단](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SUMMARY.json) · [실제4/통제1 범위 승인](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json) · [전체 원장](./REMAINING-13-EXECUTION.json).
