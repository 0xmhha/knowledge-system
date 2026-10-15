# 현재 인수인계

2026-10-16 기준 지식 매니저·프로젝트 비교 로드맵은 **27/28 세부 작업·6/7 기능 완료**다. 사용자가 멈추라고 하기 전까지 남은 작업을 순서대로 구현·검증하고 단계별 서명 로컬 커밋으로 보존한다. 각 완료 단계마다 현재 단계·검증 결과·다음 작업·남은 전체 범위를 화면 로그와 문서에 기록한다. 세부 상태는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

## 현재 구현과 다음 단계

| 기능 | 상태 | 확인한 범위 |
|---|---|---|
| KM-01 프로젝트 격리 | 4/4 완료 | 실제4DB·동명/포크/버전·인용·공개 읽기/쓰기 거부·rollback/GC |
| KM-02 버전 DB 운영 | 4/4 완료 | 공개 build/status/cancel·영속 job/재시작·유지/실패 matrix |
| KM-03 Claude 매니저 | 4/4 완료 | 등록·문서 없는 수집/초안·검토/회수·실제 설치 CLI8회/68도구; 로컬 시험 응답기 |
| KM-04 지속 갱신 | 4/4 완료 | 실제 periodic/change2프로젝트·20항목·221호출·2EOF0·삭제/no-op·실패/연결 retry·타 DB/인용 보존 |
| KM-05 공통 개념/연결 | 4/4 완료 | 공통 어휘·원문 연결·직접 매핑 검토/회수·적용/지원·L1/L2/network/산업 차이 |
| KM-06 비교 근거 MCP | 4/4 완료 | 실제 go-stablenet/go-wemix4243파일·6개념40셀·22항목19호출·3EOF0·오매핑/범위/예산/원본 보존 |
| OE-01 온톨로지 효과 | 3/4 완료 | 512회 함수 회수/순위·의도 범위·warm/API/token 분석; 이득EX08 한정·off/inconclusive; 다중 프로젝트 다음 |

OE-01.3 완료: 고정512회 원자료에서 유한한 완전 함수 찾기 과제의 precision/recall·순위·의도한 ontology 개념/범위 진단·warm 지연·실제 로컬 API 호출/서버 token 수를 재계산했다.5사실군 평균 recall은 off54.44%→concept_text/combined56.67%, precision12.87%→13.54%, MRR0.4612→0.4862다. 개선은 EX-08 영어 메시지 처리 질문1개에서 prepare만 회수하던 결과에 commit이 추가된 것으로 한정된다. 나머지13개 채점 질문의 지표는 같았다. hit@10은68.33%로 동일하다. 일반 행동/정책의 semantic relevance나 답변 정확도로 확대하지 않는다.

각 arm의 warm80개 p95는 off583.1ms, concept_text736.2ms(1.263배), relations607.4ms(1.042배), combined697.4ms(1.196배)다. capture batch embedding API는 off/relations1584개·31536서버 prompt tokens, concept_text/combined1664개·56112tokens다. 준비/모델 확인도 포함한 도구 실행 batch 비용이며8호출×16질문의512개 독립 사실이 아니다. 토큰은 서버 보고값이고 금액/전력/하드웨어 비용은 추정하지 않았다. 유료/LLM 답변0이다. 활성 모드는 의도 개념13질문 중9개를 매칭했고, scope 의도 진단3질문 중 외부 go-wemix 대상 EX-16에도 WBFT quorum을 매칭했다. relations는 검토된 구현 연결0·rank 변화0이다. 정의 검색/combined의 최종 인용 순서는32짝 중18짝에서 달랐지만 함수 회수 이득은1질문뿐이다. 작은 source-derived 개발 표본과 지연 관찰로 기본 off·quality=inconclusive를 유지한다.

전체27/28·기능6/7이다. 다음이자 마지막 세부 작업은 OE-01.4의 실제2프로젝트 공통 개념 매핑 효과·오인/누락 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 실제 LLM 답변·독립 정답·운영 활성화는 미실행/미승인 상태로 보존한다.

## 검증과 현재 제한

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. 새 Go 변경이 없음을1273개 Go와 모든 결속 입력의 hash로 확인해 직전 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 실제 corpus/MCP 시험과 과거 Go 실행을 구분한다. 유료 모델 호출0이며 ontology 효과/독립 품질 수용은 아니다. [검증 자료](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/manifest.json)를 따른다.

매니저 plugin0.6.0은 reviewed 기본18도구와 명시 updates6도구를 제공한다. 실제 Claude Code2.1.300과의 갱신 연결2회/36도구도 로컬 시험 응답기로 검증했다. 실제 모델 호출/비용0이며 agent 자율성·답변 품질은 측정하지 않았다. 개인 머신에 서비스나 OS scheduler를 설치하지 않았고 명시 foreground worker가 실행 중인 동안만 주기/변경 polling이 동작한다.

원래 독립 수용의 단계0·담당자·실제 모델/인증/비용 지정은 로컬 개발을 막는 요건이 아니다. 유료 모델·독립 수용·외부 게시에는 해당 실행 범위가 별도로 필요하다. 이번 지속 작업은 단계별 로컬 커밋 범위다. 푸시/PR·전역 설치·운영 current 변경을 완료로 추정하지 않는다.

## 보존할 결정과 품질 관찰

정책 참조 수정7건은 별도 평가 overlay로 사용자 채택 승인을 기록했다. [승인 영수증](../../system/eval/operational-preparation-2026-10-15/adopted-policy-user-approval/receipt.json)과 [적용 검증](./POLICY-OVERLAY-ADOPTION.md)을 따른다. 원본 정책·원래 승인 입력은 보존한다. 일반 domain sync가 기존 잘못된 참조를 재생성하는 문제는 아직 별도 보완 대상이다.

[OE-01 단일 프로젝트 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)은384회 실패/누락0·인용3,616개 원문 일치, 활성288회 no_match·순위 차이0이었다. 별도 정확 용어8회도 최종 boost0이다. 기본 off·quality=inconclusive를 유지한다. KM-04.4에서 단독 합성 대문자 식별자 질의가 비고 같은 자료가 자연어 질의에 나타난 사례도 OE-01 질의 해석/회수율 후보로 보존했다. 새 기능 구현이나 계획 승인을 독립 정답·품질 승인으로 바꾸지 않는다.

이전 단계의 상세 기록은 각 명세의 checkpoint 절과 서명 커밋·동결 증거 packet에 남긴다. 현재 상태는 이 문서 첫 표와 전용 원장을 우선한다.
