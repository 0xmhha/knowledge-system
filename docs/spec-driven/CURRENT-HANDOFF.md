# 현재 인수인계 — 단계86 로컬 CoreML 보완 검증 / 2026-10-20

단계86 구현·검증은 서명 커밋 `d5efe8f10b79`에 저장했고387개 파일의 실제 저장 내용과 서명을 대조했다.

CoreML 선택 CKV 경로의 토큰 전송·출력 텐서·모델 소유 수명을 보완했다. 기본 전체 일반4272통과·18skip, 후속 전체race4272통과·18skip, build/fmt/vet/고정lint/docs 통과다. 입력1815개·Go1305개·변경Go13개와 선택 프로필 fixture2개를 고정했고 최종 기본 CKS/CKG/CKV 실행 파일이 실제 시험 후보와 일치했다. 앞서 통과한 동일 입력의 build/general은 재사용한 근거로 명시했다.

최초 전체race에서 기존 guardian absolute deadline 검사1개가 실패했다. 소스를 바꾸지 않은 동일 검사20회와 진단 overlay80회, 후속 전체race가 통과했지만 최초 실패의 원인은 미확정이다. 원본 실패를 보존하며 종료 문제를 해결했다고 표시하지 않는다. 운영 채택 전 다음 우선순위다.

선택 native 프로필 일반/race각28통과·1skip, CGO-off3통과, 빌드/고정lint 통과. 실제 CoreML 배열·feature-provider fixture의 MRC/ARC 검사는 입력101/102/103 보존, Float32/Double/Float16·stride·shape·상한 거부, 독립2모델 객체 해제를 확인했다. 실제 가중치 추론은0이다. tokenizer native 의존성은 기존 고정 버전의 공식 배포 SHA와 대조해 임시 폴더에서만 사용했다.

기본 BGE 기존2회/새1회는 모두exit0·10hits였고 출처·본문·순서·점수 세부값이 일치했다. 최초 차원 정책 불일치와 점수 구조를 잘못 가정한 비교 하네스 오류는 원자료로 보존했다. 비교는 새 모델 호출 없는 오프라인 재분석으로 바로잡았으며 데이터·소유 프로세스 정리를 확인했다.

[단계86 요약](./operational-preparation/source-review-supplement/followups/coreml-transport-v1/SUMMARY.json) · [범위·실패·한계](./operational-preparation/source-review-supplement/followups/coreml-transport-v1/REPORT.txt) · [CoreML 명세](./operational-preparation/source-review-supplement/followups/coreml-transport-v1/SPEC.json) · [지원 범위와 미충족 조건](./operational-preparation/source-review-supplement/followups/coreml-transport-v1/REMAINING-SCOPE-RECONCILIATION.json) · [13묶음 정본 원장](./REMAINING-13-EXECUTION.json).

실제 .mlpackage/.mlmodelc·호환 tokenizer·배포 대상과 reference embedding/pooling 검증은 아직 없다. managed CKS CoreML runtime과 Qwen CoreML 지원도 구현하지 않았다. ANE 사용/성능은 미측정이다. 아래 단계85 근거는 당시 고정 입력의 역사적 시험이며 현재 기본 소스의 전체 회귀 근거는 단계86이다.

선택형 CKS CLI v2·기존 HTTP MCP·독립 원시 그래프 JSON v2를 구현했다. 전체 일반4265/race4265통과·각18skip·실패0, build/fmt/vet/고정lint/docs134통과다. 새 Go 변경14개와 입력1807개를 고정했고, 실제 실행과 최종 native CKS/CKG bytes가 같다. 전체 검증 뒤 공개 계약 문서의 낡은 문장1개만 바로잡아 docs-check를 재실행했으며 기본 CKS/CKG/CKV bytes가 모두 같았다. 코드 검증과 문서 정정의 근거를 분리했다.

검증 원장 회귀67개·새 v2 증거 검사8개·작업 원장4개가 통과했다. 최초 새 원장 검사에서 전달된 검토 자료를 직접 대조하지 않아27개가 실패한 결함을 고쳤고, 최초 검사기/manifest/실패 로그를 보존했다.

[단계85 요약](./operational-preparation/source-review-supplement/followups/public-v2-surface-v1/SUMMARY.json) · [범위·실패·한계](./operational-preparation/source-review-supplement/followups/public-v2-surface-v1/REPORT.txt) · [구현 계약과 후속 입력 후보](./operational-preparation/source-review-supplement/followups/public-v2-surface-v1/SPEC.json) · [공개 형식 계약](./PUBLIC-CONTRACT-V2.md) · [13묶음 정본 원장](./REMAINING-13-EXECUTION.json).

실제 CLI/HTTP는 좌표·질의·인용11개·무결성 해시가 일치했다. 기본 markdown을 유지하고, v2는 소유 MCP의 성공 종료 뒤 출력한다. 기존 파일은 덮어쓰지 않는다. 원시 그래프 v2는 프로젝트·데이터·소스 좌표와 캡처 digest를 보존한다. 실제347378485바이트/244989노드/1964193관계를 SQLite와 전 행 대조했다. 실행 중 종료 요청은 취소 오류·exit1이며 미완성 출력/임시 파일/잔여 프로세스0, 데이터 보존이다. 이 원시 그래프는 검색 EvidencePack의 원문 검증·정화·sha256-v2가 아니다.

최종 연결 시험은 로컬 BGE embedding HTTP28회/입력284개다. 초기 별도 후보의 연결 시험도 원자료로 보존했고 최종 후보 수치에 합치지 않았다. 실제 LLM·유료 호출·전역 설치·push/PR은0이다. 초기 그래프 시험 상수명 오류, 하네스 모듈 경로/출력 폴더 오류와 sandbox lint cache 경고를 숨기지 않았다. 직접 CKV retained v2와 정적 분할 v2는 아직 구현하지 않았다. 기존 v1은 비커밋 소스를 거부하며, 직접 CKV에는 보관 원문·reader lease·필수 본문 처리의 엔진 독립 공유 경로가 먼저 필요하다.

최종 원장 회귀67개·CoreML 증거8개·작업 원장4개와 문서 검사를 통과했다. 검증 하네스의 sandbox 프로세스 조회 거부와 Python3.9 선택 오류는 [실행 환경 기록](./operational-preparation/source-review-supplement/followups/coreml-verification-environment-v1/REPORT.json)에 원자료와 함께 남겼다. 동일67개를 명시한 Python3.11로 재검증했으며 승인 조건이나 제품 소스를 바꾸지 않았다.

## 다음 작업

최초 guardian 종료 시한 실패를 원래 검사와 조건을 유지하며 조사한다. 재현하지 못한 문제를 통과 반복만으로 해결 처리하지 않는다. 실제 CoreML 모델 품질 평가는 호환 패키지·tokenizer·배포 대상·reference 입력 확보 후 진행하며 기본 BGE 기준선을 유지한다.

로컬 Qwen3 임베딩0.6b/4b는 실제 존재하며1024/2560차원 Ollama GGUF이다. CoreML 패키지·별도 tokenizer는 지정 루트/색인에서 찾지 못했으며 전체 홈의 부재 증명은 아니다. GGUF를 CoreML 실물로 기록하거나 BGE 기준선을 바꾸지 않았다.

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
|활성 후속10개|캐시 로컬 완료. 공개 형식 후속·graph tuning·flow gold·규모 수용·as-of·legacy 출처·CoreML 실모델/운영·viewer Atlas/접근성·배포/downstream 남음|

D04미정·독립승인0·FINAL0·freeze null·execution_allowed=false를 유지한다. 실명/연락처를 요구하지 않으며 익명 역할로 검토할 수 있다. 보안 전용 정답/ACL/비공개/위조와 감사 기능 평가는 사용자 요청으로 보류하며 완료로 세지 않는다. 실제 LLM 답변 평가는 모델·인증·총 비용 상한 결정 후 진행한다. AI 초안과 기술 시험을 사람의 독립 승인으로 승격하지 않는다.

## 보존한 앞선 근거

- [단계80 viewer](./operational-preparation/source-review-supplement/followups/viewer-lifecycle-v1/SUMMARY.json): 당시 실제 UI 검색·본문·2D/상한/키보드·탐색과 소유 API 종료 검증. Atlas backend·전체 접근성은 미검증이다. 현재 Go의 전체 근거는 단계86이며 실제 UI 관찰은 당시 소스의 근거다.
- [단계81 승인 범위 적용](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json): 실제 owner4·통제 late-drain1과 옛 구독·새 등록 전 경계, 독립 정답 승인과 구분한다.
- [단계82 AI 초안 당시 검사](./operational-preparation/source-review-supplement/followups/draft-history-reconciliation-v1/SUMMARY.json): 8범위/검사기7개가 각 원래 커밋의 격리 복사본에서 통과했다. 현재 정답 승인과 다르다.
- [단계83 모델 목록](./operational-preparation/source-review-supplement/followups/coreml-local-model-inventory-v1/INVENTORY.json): Qwen3 GGUF 헤더·metadata만 확인했다.
- [단계84 실규모](./operational-preparation/source-review-supplement/followups/scale-long-functions-v1/SUMMARY.json): 1.242GB에서 긴 함수3개/21요청 partial18·budget_exhausted3과 복구·EOFexit0을 확인했다. 용량 SLA나 의미 품질 수용은 아니다.
