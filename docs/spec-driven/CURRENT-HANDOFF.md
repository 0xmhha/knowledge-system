# 현재 인수인계 — owner ack 이후 status channel 사용과 구독 세대 경계 / 2026-10-20

새 머신/새 세션에서는 [머신 간 인수인계](./MACHINE-HANDOFF.md)를 먼저 따라 증거 태그·실행 도구·외부 DB/모델·삭제된 임시 경로를 확인한다.

단계91 요청 범위의 관측을 마쳤다. [명세](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SPEC.json) · [결과](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json) · [검증·시도 이력](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/VALIDATION.json) · [자원 정리](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/CLEANUP.json). 단계90 근거는 [그대로 보존한다](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json).

## 새 머신 복원과 실행 adapter

evidence 태그를 받은 뒤 이력 복원 검사, 진행 원장, 운영 수용 구조 검사, 단계90 검증기가 모두 통과했다. 이전 머신 경로는 쓰지 않았다. 단계90 드라이버에 박힌 경로 7종을 이름 붙인 자리표시자로 바꾸는 adapter를 만들었다. 실제 로컬 경로는 커밋하지 않는다. [사전 점검](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/adapter/PREFLIGHT.json)에서 확인한 내용은 다음과 같다.

- 고정 커밋으로 새로 clone한 go-stablenet은 보존된 2228개 파일 목록과 해시가 같다.
- Go 1.23.12 local과 공용 모듈 캐시만으로 오프라인 의존성 해석이 된다.
- 단계90 계측 규칙을 gofmt 경로만 바꿔 다시 돌리면, 네 역할의 계측 API와 Git build tree가 기록된 해시와 같다.

본체 검증은 처음에 Go 1.26.8로 돌렸다. 이때 race 시험의 buildpipe가 기본 10분 제한을 넘겼고, manager broker 시험 11개가 실패했다. broker fixture가 `go 1.26.9`를 요구하는데 1.26.8을 local로 고정했기 때문이다. 같은 오류를 재현해 확인했다. CI와 같은 공식 Go 1.26.9로 다시 돌리자 build, lint, race 126패키지, manager 268개가 모두 통과했다. 두 실행 기록을 모두 보존했다.

## 현재 단계 — 16슬롯·실제32노드, 대조군 아닌 슬롯의 ack 후 같은 세대 사용 0

trace는 (factory API, status channel) 키마다 세대를 센다. 실제 owner의 install이 일어날 때마다 세대가 하나 오른다. api.go에 있는 모든 status channel 연산 지점(역할별 6~10곳)에 계측을 걸었다. 송신, 수신, 등록, 해제 요청, ack가 모두 포함된다. 같은 세대가 이미 ack된 뒤에 그 세대로 송신이나 수신이 일어나면 위반으로 센다. 세대가 오른 뒤의 사용은 명시적 새 등록이므로 허용한다.

|일정|역할|관측|
|---|---|---|
|overlap(단계90 gate)|CAND-A/B|drain→실제 owner 삭제·ack 뒤 같은 channel 세대2 전달1·새 RPC 전달1, 위반0, 정리 개입0|
|overlap|ORIGINAL|쌍 대기 후 소유 status 수신1로 복구; 재생성은 복구 뒤로 표시; 위반0|
|overlap|MUT-EARLY|조기 반환, 세대마다 실제 owner manual uninstall 3회; 위반0|
|timer(gate 없음)|ORIGINAL·CAND-A/B|60초 타이머 전 옛 세대 ack; 자연 done broadcast는 새 세대만 대상, 위반0|
|timer|MUT-STALE-ACK|delete 없이 ack한 옛 세대에 done 송신 1회(위반 검출); 소유 수신1로 복구|

일반/race 각 1회씩이다. RPC `true` 응답과 owner ack는 따로 기록했고, `true`를 ack로 바꾸어 세지 않았다. 소유 status 수신은 6회, manual uninstall은 6회이며 가짜 ack는 0이다. 진행 중 true 알림, transport close, Notify 도중 종료는 다루지 않았다. 같은 channel 재등록도 install 송신 경로만 시험했다. timer broadcast 경로에서는 stale 항목과 재등록이 map 키 하나로 합쳐지므로 channel 사용만으로는 구별할 수 없다. 이 경우는 membership으로만 드러난다. legacy PoW 시험이며 운영 WBFT 검증이 아니다. 실제 owner4/별도 통제 late1 범위는 바꾸지 않았고 독립 표본으로 합치지 않는다.

시도 실패도 보존했다. 첫 컴파일 점검에서 bridge 함수 이름이 downloader 패키지와 충돌했고, 첫 smoke에서는 하네스가 상대 경로를 넘겼다. 둘 다 노드 실행 0회에서 멈췄다. 첫 보관본에서는 로컬 계정명과 임시 경로가 섞인 것을 커밋 전에 찾아 다시 보관했다. 매트릭스 자체는 자동 재시도 0회로 16슬롯이 모두 성공했다. 원본 checkout은 보존됐다. 캐시·clone·바이너리 약 6.85GiB는 지웠고 남은 소유 프로세스는 0이다. KS 운영 Go 변경, 실제 LLM, 유료 호출, 운영 적용, 게시는 모두 0이다.

## 다음 작업

WI-V1에서는 독립 ticket/oracle, 승인된 실제 역할 양성 경로와 최종 HEAD가 남았다. transport close와 같은 channel timer broadcast 경로의 세대 경계도 남아 있다. 이후 독립 정답·충분성·동결이 필요하다.

## 남은 전체13묶음 — 상위 완료0/13·정식0/9

|묶음|남은 내용|
|---|---|
|WI-V1|독립 티켓/oracle·승인 양성 역할/final HEAD·transport close와 같은 channel timer broadcast 세대 경로|
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

[단계90 native/RPC 겹침](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json) · [12계약·소비자12기준 검토](./operational-preparation/source-review-supplement/followups/wiv1-contract-consumer-review-v1/REVIEW.json) · [기본 고정 소스·종료 시한 진단](./operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SUMMARY.json) · [실제4/통제1 범위 승인](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json) · [전체 원장](./REMAINING-13-EXECUTION.json).
