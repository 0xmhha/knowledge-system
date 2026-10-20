# 남은 전체 작업 / 2026-10-20

[정본 원장](./spec-driven/REMAINING-13-EXECUTION.json) · [현재 인수인계](./spec-driven/CURRENT-HANDOFF.md) · [최신 소스 검증](./spec-driven/operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1/SUMMARY.json)

사용자 지시 “coreml은 안해도되”에 따라 CoreML을 개발·평가·배포 및 완료 조건에서 제외했다. 모델·토크나이저·배포 대상은 더 이상 요청하지 않는다. 제외는 완료가 아니며 기존 코드와 서명된 검증 기록은 보존한다. 기본 Ollama + BGE-M3를 유지한다. [범위 제외 기록](./spec-driven/operational-preparation/source-review-supplement/followups/user-excluded-coreml-scope-v1/SCOPE.json).

로컬 구현9/9·지식 매니저 로드맵7/7(28/28)과 정식 독립 수용0/9를 구분한다. D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false를 유지한다. 실명/연락처를 요구하지 않으며 익명 역할로 검토할 수 있다.

## 현재 검증과 다음 작업

실제 factory·실제 loopback peer 동기화·원래60초 done broadcast와 같은 RPC 구독의 callback Err 종료를 연결했다. 선언한 송신 직전 gate 아래 일반/race10슬롯·20노드를 실행해 원본/재도입 쌍 대기4건, A/B native drain·실제 owner ack 후 반환4건, 조기 반환2건을 구분했다. 고유 source4개이며 별칭을 독립 표본으로 세지 않는다. 모든 노드·소유 프로세스를 정리했고 cache/시험 루트3개 약2.06GiB를 제거했다. [검증 정본](./spec-driven/operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json).

회귀121종과 최종 결속 집중9개가 통과했다. 최초 보조 fixture 채널 타입 컴파일 오류와 검증기 변경 후 manifest guard 실패는 보존하고, 각각 별도 수정·재결속 뒤 검증했다. KS 운영 Go 변경0이며 이전 본체 전체 일반/race4273개씩의 같은 입력 검증을 재사용한다. gate 없는 보편 동작·모든 post-ack channel-use·독립 수용으로 승격하지 않는다.

다음은 실제 owner ack 후 옛 구독의 channel-use와 명시적 새 등록 세대 경계다. 승인된 독립 oracle 기반 실제 역할 성공 경로와 최종 HEAD 판정도 남아 있다. 앞선 최신 CKS 실제4역할·RPC 오류120건·MCP 오류/미확정35건 근거는 보존한다.

## 전체13묶음 — 완료0/13

|묶음|현재 남은 내용|
|---|---|
|WI-V1|최신 CKS 4역할·API 오류·native done/RPC Err overlap 확인; channel-use/새 세대·승인 oracle/final HEAD·독립 티켓|
|WI-V2|유한 후보 구별 완료; 독립 oracle·수용 후보 집합·선정 판단|
|WI-6|warm 쌍627회 보존; 독립 nonseed 관련성/잡음 정답·표본 충분성|
|WI-SP|비보안 수동/자동 출처·policy/미태그 소비자와 독립 의미 검토; 보안 부분은 사용자 보류|
|WI-C|설치 역할5산출물·렌더·인수인계 확인; 독립 필수 항목/우선순위 검토|
|WI-1|선택 수신16수정; range37/빈 블록3 미확정 구분; 값/종류/메모리 매칭·독립 완전 정답|
|WI-2|독립 비보안 표본·호출 대상/호출자·명령/build/MCP 소비자 매칭|
|WI-3|시작 배치·계측 기능 보존 검증 완료; 가족/노출/중복·기본 상한/partial/오류/범위 밖·독립 관련성|
|WI-4|실제 host·receiver 일정·반복 기준·독립 가족/oracle/충분성; 실제 peer 일반/race 근거 확인|
|독립 수용 준비|익명 검토 역할 미정; 새 DEV/FINAL·완전 정답/매칭/충분성·F01~08 동결|
|온톨로지|독립 의미/검색 정답·구현/개념 연결·동일 corpus off/on 효과 판단|
|정식9개 실행·수용|독립 승인·동결 후 WI-1/2/3/4/V1/V2/6/SP/C 실행; 현재0/9|
|활성 선택 기능9개|캐시 로컬 완료. 공개 형식·graph tuning·flow gold·규모·as-of·legacy 출처·viewer·배포 남음. CoreML 사용자 제외, 보안 감사 사용자 보류|

활성 선택 기능9개 중 캐시는 로컬 완료이며 나머지8개는 각 기준과 입력이 남아 있다. CoreML은 제외하고 보안 전용 평가·조직 인증·보안 감사는 사용자 보류로 유지한다. 실제 LLM 답변 평가는 모델·인증·비용 상한 결정 후 진행한다. 각 작업은 검증·정본 문서 갱신·서명 커밋·저장 내용 대조 후 체크하며 독립 수용을 로컬 시험으로 대신하지 않는다.
