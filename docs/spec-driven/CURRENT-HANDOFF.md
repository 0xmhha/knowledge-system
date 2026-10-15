# 현재 인수인계

2026-10-16 기준 지식 매니저·프로젝트 비교 로드맵은 **18/28 세부 작업·4/7 기능 완료**다. 사용자가 멈추라고 하기 전까지 남은 작업을 순서대로 구현·검증하고 단계별 서명 로컬 커밋으로 보존한다. 각 완료 단계마다 현재 단계·검증 결과·다음 작업·남은 전체 범위를 화면 로그와 문서에 기록한다. 세부 상태는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

## 현재 구현과 다음 단계

| 기능 | 상태 | 확인한 범위 |
|---|---|---|
| KM-01 프로젝트 격리 | 4/4 완료 | 실제4DB·동명/포크/버전·인용·공개 읽기/쓰기 거부·rollback/GC |
| KM-02 버전 DB 운영 | 4/4 완료 | 공개 build/status/cancel·영속 job/재시작·유지/실패 matrix |
| KM-03 Claude 매니저 | 4/4 완료 | 등록·문서 없는 수집/초안·검토/회수·실제 설치 CLI8회/68도구; 로컬 시험 응답기 |
| KM-04 지속 갱신 | 4/4 완료 | 실제 periodic/change2프로젝트·20항목·221호출·2EOF0·삭제/no-op·실패/연결 retry·타 DB/인용 보존 |
| KM-05 공통 개념/연결 | 2/4 완료 | 선택형 팩8개념/26축·한영 모호성, 프로젝트별 proposed 원문 연결; 검토/버전 정책 다음 |
| KM-06 비교 근거 MCP | 0/4 미착수 | 명시 참여자/조건/예산·행렬·부분 실패·실제 프로젝트 비교 필요 |
| OE-01 온톨로지 효과 | 0/4 완료,3개 진행 중 | 단일 프로젝트 개발 관찰384회+진단8회; 확대/독립 정답·다중 프로젝트 평가 필요 |

최신 KM-05.2는3개 통제 프로젝트의5종류 원문(code/document/configuration/policy/test), 같은 이름/다른 역할과 다른 이름/같은 역할, 같은 로컬ID의 project/version/network 격리를 검증했다. collection reader lease·명시 target·retained evidence 정화/재검증을 재사용한다. 라이브 checkout/current로 대체하지 않고 committed/미커밋 working-tree/snapshot-only의 commit/base 의미를 보존한다. 연결의 review_state는 proposed이며 원문 hash 일치가 의미 승인·정책 내용 승인·시험 실행 통과를 뜻하지 않는다. [연결 계약](./INDUSTRY-CONCEPT-COMPARISON.md)과 [증거](../../system/eval/knowledge-roadmap-2026-10-16/km-05-bindings/manifest.json)를 따른다.

다음은 **KM-05.3 직접 매핑 의미·검토/회수·버전/stale 정책**이다. exact/close/broader/narrower/related의 방향·범위를 명시하고 의미 유사성을 entity 동일성으로 바꾸거나 관계를 자동 전이하지 않는다. 실제 사람 검토를 생성하지 않는다. 그 다음 KM-05.4의 비적용/미수집/미지원·L1/L2/network 차이, KM-06 공개 비교, OE-01 확대/다중 프로젝트 효과 평가 순서로 진행한다.

남은 전체10개는 **KM-05.3–4, KM-06.1–4, OE-01.1–4**다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도이며 [남은 작업](../REMAINING-WORK.md)에 유지한다.

## 검증과 현재 제한

KM-05.2의 최종 소스로 전체 Go3921실행(3903통과18skip,136패키지)과 race895실행(893통과2skip,7패키지)을 순차 검증했다. 비교 패키지는 어휘29+연결30=59개다. 빌드·fmt·vet·lint와 문서133개를 확인했다. 초기 읽기 전용 손상 fixture 오류와 working-tree 인용 검증 오류의 원자료를 보존하고 수정 후 전체 소스로 재검증했다. 이 단계는 실제 retained source archive와 fixture identity manifest를 사용한 library 검증이다. 실제 graph/vector 검색·공개 비교 MCP·산업 사실의 독립 품질 수용으로 확대하지 않는다.

매니저 plugin0.6.0은 reviewed 기본18도구와 명시 updates6도구를 제공한다. 실제 Claude Code2.1.300과의 갱신 연결2회/36도구도 로컬 시험 응답기로 검증했다. 실제 모델 호출/비용0이며 agent 자율성·답변 품질은 측정하지 않았다. 개인 머신에 서비스나 OS scheduler를 설치하지 않았고 명시 foreground worker가 실행 중인 동안만 주기/변경 polling이 동작한다.

원래 독립 수용의 단계0·담당자·실제 모델/인증/비용 지정은 로컬 개발을 막는 요건이 아니다. 유료 모델·독립 수용·외부 게시에는 해당 실행 범위가 별도로 필요하다. 이번 지속 작업은 단계별 로컬 커밋 범위다. 푸시/PR·전역 설치·운영 current 변경을 완료로 추정하지 않는다.

## 보존할 결정과 품질 관찰

정책 참조 수정7건은 별도 평가 overlay로 사용자 채택 승인을 기록했다. [승인 영수증](../../system/eval/operational-preparation-2026-10-15/adopted-policy-user-approval/receipt.json)과 [적용 검증](./POLICY-OVERLAY-ADOPTION.md)을 따른다. 원본 정책·원래 승인 입력은 보존한다. 일반 domain sync가 기존 잘못된 참조를 재생성하는 문제는 아직 별도 보완 대상이다.

[OE-01 단일 프로젝트 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)은384회 실패/누락0·인용3,616개 원문 일치, 활성288회 no_match·순위 차이0이었다. 별도 정확 용어8회도 최종 boost0이다. 기본 off·quality=inconclusive를 유지한다. KM-04.4에서 단독 합성 대문자 식별자 질의가 비고 같은 자료가 자연어 질의에 나타난 사례도 OE-01 질의 해석/회수율 후보로 보존했다. 새 기능 구현이나 계획 승인을 독립 정답·품질 승인으로 바꾸지 않는다.

이전 단계의 상세 기록은 각 명세의 checkpoint 절과 서명 커밋·동결 증거 packet에 남긴다. 현재 상태는 이 문서 첫 표와 전용 원장을 우선한다.
