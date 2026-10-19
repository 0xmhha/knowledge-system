# 현재 인수인계 — WI-V1 12계약·설치 소비자 검토 / 2026-10-20

단계88: 원래 12계약과 설치 소비자 판정 기준 12개를 전수 검토했다. [검토 정본](./operational-preparation/source-review-supplement/followups/wiv1-contract-consumer-review-v1/REVIEW.json) · [검증](./operational-preparation/source-review-supplement/followups/wiv1-contract-consumer-review-v1/VALIDATION.json) · [캐시 정리](./operational-preparation/source-review-supplement/followups/wiv1-contract-consumer-review-v1/CLEANUP.json).

계약·trace 29개, 범위 승인 6개, 고정 소비자 mechanical fixture 21개로 56개와 잔여 원장 회귀4개로 총60개 통과·실패0·skip0이다. 운영 수용 준비 검사와 문서 검사도 통과했다. 기존932개 원자료 hash·owner40관측·실제4역할 자료를 다시 대조했다. 새 실제 host/Go build/모델 호출은0이다. 소비자 fixture의 양성 oracle/선택 성공은 실제 WI-V1 티켓의 정답 승인이나 실제4역할 성공 경로와 다르다.

기존 실제4역할의 CKS는 `49197f0f3d59`, 최신 고정 검증 소스는 `87fed3d970cf`다. 최신 CKS에서 실제4역할 연결을 재실행했다고 주장하지 않는다. 소비자 `d1ebafb8ddf4`의 파일과 저장된 실행 자료를 다시 확인했다. 코드의 서로 다른 oracle 파일/command/해시는 구조 조건이며 의미 독립성을 인증하지 않는다. mechanical PASS에도 independent_human_acceptance는 not_recorded다. 선택된 tree 확인과 현재 HEAD에 결속된 oracle 확인을 함께 요구한다.

작업용 Go/npm/lint/Python 캐시·뷰어 의존성과 중간 빌드 산출물42경로를 정리해 약19.3GiB를 확보했다. 당시 여유 공간은 약31→51GiB다. 정확한 경로/할당량은 CLEANUP.json을 따른다. 소스·지식 DB·커밋된 원자료와 내장 뷰어 산출물은 보존했다. 이번 검토는 bytecode 생성과 Go/npm 빌드를 하지 않아 캐시를 재생성하지 않는다.

## 12계약 검토 — 검토12/12, 전체 계약 수용0/12

|계약|확인된 근거|남은 판정|
|---|---|---|
|API-ENTRY|실제 factory 등록 RPC 구독·원래 60초 알림·실제 peer 동기화의 별도 근거가 있다.|모든 후보·실제 RPC callback·blocked 일정이 하나의 trace와 최종 HEAD로 연결된 것은 아니다.|
|API-UNSUB|실제 RPC 해제와 owner delete→ack 관측은 존재하지만 서로 구분한 경로다.|RPC true/registry 삭제와 내부 deferred Unsubscribe 반환의 시각·객체를 같은 trace로 연결해야 한다. peer 시험은 client error channel close를 명시 관측하지 않았다.|
|API-NEGATIVE|실제 API에서 unsupported notifier 오류를 관측했다. 이전 직접 RPC 음성 경로는 별도 범위다.|unsupported transport·malformed/unknown name·unknown id 전체 오류 형태를 실제 factory/후보/고정 버전에서 연결 검증해야 한다. 오류는 symptom RED가 아니다.|
|METHOD-RETURN|후보 A/B는 실제 owner blocked 상태를 drain하여 delete/ack 뒤 반환했고 원본/재도입 대기는 rescue 전에 고정했다.|finite live-loop 조건이며 임의 스케줄·종료 중 event loop·무한 producer fairness·독립 반복 충분성은 증명하지 않았다.|
|METHOD-ONCE|같은 객체의 동시 호출과 owner 요청 1개·ack 전 반환 없음이 선언된 gate 아래 관측됐다.|객체/채널/등록 세대 구분과 개별 호출 전원 join을 새 표본에서도 확인해야 한다. 0개 요청인 early mutant는 성공이 아니다.|
|METHOD-REMOVE|실제 owner map에서 delete 뒤 ack close를 관측했다. source도 같은 순서다.|관찰 hook/gate가 타이밍에 영향을 준다. 실제 RPC overlap과 모든 후보의 무계측 보편 보장은 별도다.|
|METHOD-ACK|실제 owner의 ack 지연 중 반환 보류 및 조기 반환 오류 후보를 구별했다.|옛 구독·새 등록 전의 모든 channel-use 부재를 완전 계측으로 관측한 것은 아니다. 새 등록을 영구 금지하는 기준으로 바꾸지 않는다.|
|NORMAL-DELIVERY|factory RPC 원래 알림·post-done 직접 상태·별도 실제 peer sync 근거가 있다.|post-done 직접 호출과 native broadcast 중 실제 RPC callback 종료 overlap은 같은 증거가 아니다. source/notification/id 연결이 필요하다.|
|LATE-DRAIN|원래 단일 owner는 uninstall 수락→삭제→ack 사이에 상태를 보내지 않는다. 별도 통제 owner의 finite late send 24개를 관측했다.|승인된 실제4/별도통제1로 판정한다. 실제 owner에서 도달 불가능한 중간 송신을 인위적으로 넣거나 통제 증거를 실제 owner 증거로 승격하지 않는다.|
|BLOCKED-DELIVERY|실제 factory의 post-done 직접 status send와 sole receiver 종료를 겹쳐 원본의 양방향 대기와 A/B 복구를 구별했다.|native timer broadcast·실제 Syncing RPC callback 종료·동일 객체의 overlap은 미완이다. peer sync 성공만으로 이 조건을 충족했다고 볼 수 없다.|
|OWNED-CLEANUP|관측 후 rescue 개입 14개를 별도 기록했고 owner 자원·프로세스 잔여 0, 원본 보존을 검증했다.|rescue는 후보 성공이 아니다. 이전 실패와 package-init goroutine 개별 join 제한을 보존하며 새 실행도 종료/복구 증거를 남겨야 한다.|
|SOURCE-BOUNDARY|원본 commit·full-file overlay·project/build tree·실제 binary·드라이버 및 소비자 고정 revision 근거가 있다.|기존 실제 4역할의 CKS는 49197f0f이며 최신 고정 소스 87fed3d9가 아니다. 최종 agent HEAD와 승인 oracle의 실제 성공 경로 연결이 남았다.|

실제 owner 일정4개/별도 통제 late-drain1개와 옛 구독·새 등록 전 경계는 사용자 승인대로 유지한다. 동일 channel의 새 구독은 허용된다. RPC true는 downstream 해제 완료가 아니며, 120개 범위 행은 성공120개나 독립 표본120개가 아니다. 정리 rescue와 빠른 반환은 계약 성공으로 세지 않는다.

소비자 판정 기준은 설치 버전, pin/partial 전달, 의미 독립성, RED/GREEN/ERROR, 계약·일정 완전성, 회귀/tier, 후보 동결, 필수 측정, 동률/보류, 실제 HEAD, 완료·게시 경계, 실제4역할 양성 경로의12개다. 세부 근거와 한계는 검토 정본 IC-01~12를 따른다.

## 다음 작업

최신 CKS 실제4역할의 미승인/partial 전달 재검증과 API 음성 경로 전체 matrix를 먼저 준비한다. 이후 native broadcast와 실제 RPC callback 종료 overlap·channel-use 경계, 독립 티켓/정답·충분성·동결, 승인 자료 기반 실제 양성 oracle·final HEAD를 순서대로 진행한다. 익명 역할 미정·독립0·FINAL0·정식0/9·freeze null·execution_allowed=false를 유지한다.

# 보존한 단계87 — 종료 시한 검사 원인 추적

단계87 구현·검증은 서명 커밋 `87fed3d970cf`에 저장했고 변경303개 파일의 실제 저장 내용과 서명을 대조했다. [최종 회귀 기록](./operational-preparation/source-review-supplement/followups/guardian-deadline-verification-v1/REPORT.json).

동일 조건의 실제 시각 계측400회 중2회에서 기존 실패를 재현했다. sleep이 먼저 종료되고 셸이 뒤의 무조건 printf를 실행하면서, 파일이 group kill 호출 중·guardian 반환 전에 생겼다. 기존 검사는 이것도 “종료 후 쓰기”로 보고했다. 두 파일은 반환 시점에 이미 존재했고 stat 오류/반환 후 새 생성은 그 사례들에서 관측되지 않았다.

sleep1;printf를 sleep1&&printf로 한 줄만 바꾼 별도 대조400회는 모두 통과했다. 기존200ms 시한·1초 worker·반환 후1.1초 관찰을 유지했다. 채택한 변경은 시험 파일1개이며 운영 종료 코드는 바꾸지 않았다. 파일 조회 오류와 실제 marker 존재를 별도로 보고한다.

새 검사는 guardian 반환 후에만 writer의 release gate를 연다. owner socket은 계속 열어 절대시한이 종료 원인이 된다. 일반/race 각각3회의 lifecycle 집중 검증은27개씩 통과했다. 실제 후손을 남기는 임시 고장 대조군은 기대한 post-return writer 오류로 실패했고 잔여 프로세스0이다. 처음 대조군의 출력 파이프 대기로 시험 제한에 도달한 시도도 소유 자원을 회수해 보존했으며, 최종 검사는 일반 파일에 진단을 기록한다.

운영 종료 코드의 계측 로그를 제거한 추가400회에서는 실패가 재현되지 않았다. 이 결과도 보존하며, 계측이 드문 실행 순서에 영향을 준다는 제한을 함께 기록한다.

최초 과거1회 실패에는 파일 존재·조회 오류·반환 시각 상세가 없다. 그 사건의 정확한 내부 순서를 사후 확정했다고 주장하지 않는다. 확인한 것은 같은 조건에서 재현한 실패 유형의 원인과 오탐 수정이다. 과거 실패와 모든 진단 시도는 보존하며 모든 환경의 종료 무결함이나 독립 품질 수용을 선언하지 않는다.

새 고정 소스로 기본 전체 일반4273/race4273통과·각18skip·실패0, build/fmt/vet/고정lint/docs 통과다. 입력1815개·Go1305개와 변경된 시험 파일1개를 결속했고 최종 CKS/CKG/CKV는 같은 소스의 고정 후보와 일치한다. 실제 모델/LLM·유료 호출0·운영 적용·게시0이다.

[진단 요약](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SUMMARY.json) · [시도·실패·한계](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/REPORT.txt) · [명세](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SPEC.json) · [13묶음 원장](./REMAINING-13-EXECUTION.json).

CoreML은 사용자 지시로 작업·평가·배포와 완료 조건에서 제외했다. 모델/tokenizer/배포 대상을 요구하지 않고 기존 서명된 소스 검증 이력만 보존한다. 기본 Ollama + BGE-M3와 활성 선택 기능9개를 유지한다. [제외 기록](./operational-preparation/source-review-supplement/followups/user-excluded-coreml-scope-v1/SCOPE.json).

## 다음 작업

12계약·설치 소비자 검토는 단계88에서 완료했고 위의 재검증·미해결 계약 순서를 따른다. 기존 owner4/별도 통제 late-drain1 범위 채택과 독립 gold/수용을 구분한다. 익명 검토 역할 미정·새 DEV/FINAL·정답/매칭/충분성·F01~08 동결은 임의로 승인하지 않는다.

## 남은 전체13묶음 — 완료0/13

|묶음|현재 남은 내용|
|---|---|
|WI-V1|실제 host/factory/indexer/owner·peer 근거와 승인된 실제4/통제1 범위를 유지; 전체12계약·설치 oracle·독립 티켓|
|WI-V2|유한 후보 구별 근거 유지; 독립 oracle·수용 후보 집합·선정 판단|
|WI-6|warm 쌍627회 보존; 독립 nonseed 관련성/잡음 정답·표본 충분성|
|WI-SP|비보안 수동/자동 출처·policy/미태그 소비자와 독립 의미 검토; 보안 부분은 사용자 보류|
|WI-C|설치 역할5산출물·렌더·인수인계 근거 유지; 독립 필수 항목/우선순위|
|WI-1|선택 수신16수정, range37/빈 블록3 미확정 구분; 값/종류/메모리 매칭·독립 완전 정답|
|WI-2|독립 비보안 표본·호출 대상/호출자·명령/build/MCP 소비자 매칭|
|WI-3|시작 배치66→2와 계측 기능16조합 보존; 가족/노출/중복·기본 상한/partial/오류/범위 밖·독립 관련성|
|WI-4|실제 host·receiver 일정·반복 기준·독립 가족/oracle/충분성|
|독립 수용 준비|익명 검토 역할 미정; 새 DEV/FINAL·완전 정답/매칭/충분성·F01~08 동결|
|온톨로지|독립 의미/검색 정답·구현/개념 연결·같은 corpus off/on 효과 판단|
|정식9개 실행·수용|독립 승인·동결 후 WI-1/2/3/4/V1/V2/6/SP/C 실행; 현재0/9|
|활성 후속9개|캐시 로컬 완료. 공개 형식 후속·graph tuning·flow gold·규모 수용·as-of·legacy 출처·viewer Atlas/접근성·배포/downstream 남음. CoreML은 사용자 제외이며 완료 조건이 아님|

D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false·정식0/9를 유지한다. 실명/연락처를 요구하지 않으며 익명 역할로 검토할 수 있다. 보안 전용 gold/ACL/비공개/위조와 감사 기능 평가는 사용자 보류다. 실제 LLM 답변 평가는 모델·인증·총 비용 상한 결정 후 진행한다.

## 보존한 앞선 근거

- [단계86 고정 소스 검증](./operational-preparation/source-review-supplement/followups/coreml-transport-v1/SUMMARY.json): 최초 race 실패와 원본400회 이전 진단20/80회, BGE 기존/새10hits 일치, 당시4272개 기본 회귀. 제외된 기능의 과거 모델 미수용도 보존한다.
- [단계85 공개 형식](./operational-preparation/source-review-supplement/followups/public-v2-surface-v1/SUMMARY.json): 실제 CLI/HTTP 인용11개·hash 일치와 원시 그래프244989/1964193 전 행 대조, 취소/출력 정리 근거.
- [단계84 규모](./operational-preparation/source-review-supplement/followups/scale-long-functions-v1/SUMMARY.json): retained1.242GB/긴 함수3개·21요청의 partial/budget/복구 근거이며 용량 SLA·의미 품질 수용과 구분한다.
- [단계81 owner 범위 채택](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json): 실제4/통제1, 옛 구독·새 등록 전 범위이며 전체12/독립 gold 수용은 아니다.
- [단계80 viewer](./operational-preparation/source-review-supplement/followups/viewer-lifecycle-v1/SUMMARY.json): 당시 built UI 상호작용·소유 backend 종료이며 Atlas·전체 접근성은 미검증이다.
