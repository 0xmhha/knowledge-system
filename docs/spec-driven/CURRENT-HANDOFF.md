# 현재 인수인계 — 최신 CKS 실제4역할·API 오류 경로 / 2026-10-20

단계89 요청 범위의 로컬 실행·검증을 완료했다. [명세](./operational-preparation/source-review-supplement/followups/latest-four-role-api-v1/SPEC.json) · [결과](./operational-preparation/source-review-supplement/followups/latest-four-role-api-v1/SUMMARY.json) · [검증](./operational-preparation/source-review-supplement/followups/latest-four-role-api-v1/VALIDATION.json) · [임시 자원 정리](./operational-preparation/source-review-supplement/followups/latest-four-role-api-v1/CLEANUP.json).

## 현재 단계 — 실행3/3·증거 회귀112통과

|실행|검증 결과|한계|
|---|---|---|
|최신 CKS 실제4역할|Analyzer/Planner의 실제 partial MCP2회·context write2개·downstream Read3회, 미승인 선택 거부2회·완료 거부4회; host/CKS 정상 종료|로컬 scripted 모델 응답이며 역할 의미 품질·승인 oracle 양성 경로가 아님|
|실제 factory RPC|오류12종×후보5역할×일반/race=120건 예상 오류; 정상 registry 구독/해제·오류 후 health·node close10회 통과|고유 source4개, 재도입은 원본 별칭. native60초 알림/callback overlap·전체12계약 수용은 아님|
|최신 CKS stdio MCP|실제 사용2도구의 잘못된 selector/타입/필드/한도/rule pack·없는 도구, source_mismatch/not_found 등35건과 이후 partial2회 통과|35개 선언 범위이며 모든 MCP 도구나 모든 입력의 보편 검증이 아님|

CKS는 clean HEAD `b8e3488e302e`를 Go1.26.8 기본 프로필로 새로 빌드했고 build-info의 같은 revision·modified=false와 바이너리/입력 hash를 확인했다. 소비자는 `d1ebafb8ddf4`, 실제 Claude Code host는 고정2.1.300이다. 이전 드라이버의 P76/corrective1 문자열은 RUNPLAN에 재사용한 명칭임을 기록했고, 모든 실행은 이번 새 소유 자원에서 수행했다. 과거 CKS49197f의 역할 실행을 최신 실행으로 바꿔 표기하지 않았다.

RPC 원본은 `0bf2f4d1bfeb`이며 실제 node.New/eth.New/Start·활성 indexer 설정·등록된 factory API를 사용했다. 기존 파일에는 선언한 후보의 API overlay만 적용했고 probe main1개를 추가했다. 관찰 hook은 넣지 않았다. full source map·project/build Git tree·일반/race8개 바이너리·실행10개를 결속했다. HTTP 미지원은 실제 ServeHTTP를 소켓 없이 호출했고 나머지는 실제 in-process RPC다. 선언한12종은 HTTP 구독 미지원, 구독명 누락/숫자/null/알 수 없는 이름/namespace/초과 인수, 해제 ID 미존재/누락/숫자/초과 인수/이미 해제된 ID다. 실제 오류 코드와 메시지를 보존했으며 API 오류를 symptom RED로 세지 않는다. RPC true로 내부 deferred Unsubscribe 완료를 추론하지 않는다.

MCP의 source_mismatch/not_found는 구조화된 미확정 상태로, 잘못된 인수/도구는 오류로 구분했다. 오류 뒤에도 같은 세션에서 출처 hash가 일치하는 partial2개를 받고 EOF로 정상 종료했다. 원본 데이터·소스·소비자 및 소유 데이터 전후 inventory가 같고, 소유 프로세스/세션 잔여0이다. BGE-M3 임베딩은 로컬 Ollama를 사용했다. 실제 LLM·유료/외부 LLM 호출0이며 전역 플러그인 등록·운영 적용·게시를 하지 않았다.

새 증거 회귀6개를 포함한 지역 회귀45개와 준비 상태 회귀67개, 합112개 통과·실패0·skip0이다. 이전 CKS 전체 일반/race4273개씩의 서명된 검증은 입력 hash가 같은 근거로 보존하며, 이번에 전체 suite를 새로 실행했다고 주장하지 않는다. 운영 Go 코드 변경은 없다.

이번에 새로 만든 빌드 캐시2개·소유 시험 루트2개(바이너리/데이터/소스 복사본/CLI profile 포함)를 제거했다. 삭제 전 압축 원자료 manifest를 검증했고 약3.49GiB의 임시 할당을 반환했다. 이전42경로 정리 기록도 보존하며 전역 모듈 저장소·원본 DB·원본 checkout은 삭제하지 않았다.

## 다음 작업

WI-V1의 native broadcast와 실제 Syncing RPC callback 종료 overlap을 같은 출처·구독/객체·관측 시각으로 연결한다. 이후 옛 구독·새 등록 전의 전체 channel-use 경계 관측을 보완한다. 실제 owner4/별도 통제 late-drain1이라는 승인 범위를 유지한다. 독립 정답·충분성·동결과 승인 oracle 기반 실제 양성 역할/final HEAD는 별도로 남는다.

## 남은 전체13묶음 — 상위 완료0/13·정식0/9

|묶음|남은 내용|
|---|---|
|WI-V1|native broadcast/실제 RPC callback 종료 overlap·channel-use 경계·독립 티켓/oracle·승인 양성 역할/final HEAD|
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
