# 남은10개 완료를 위한 우선순위·실행 계획

2026-10-08 · 기준 main eeeab2e7(PR97 squash) · 후속 브랜치 feat/refactoring-followup · 전체8/18·요청5/15 완료·잔여10개. 기존 preview 종료와 상위 WI10개는 이 목록에 합산하지 않는다.

## 목적과 현재 근거

원문·정책·요구·코드·시험의 연결을 실제 검토할 수 있고, 갱신/복구 후에도 근거가 보존되는 시스템을 완성한다. 이전 작업을 단지 코드/시험이 존재한다는 이유로 전체 수용 완료로 바꾸지 않는다. 새 검색 gold와 관계의 정확성·실제 운영 사실은 도구 실행과 분리한다.

PR97 head d7de0b89의 CI5/5 성공 후 사용자 지시로 main에 스쿼시 머지했다. GitHub main·origin/main·local main은 eeeab2e7이며 테스트한 공개 head와 Git tree가 같다. 기존 로컬 증거483파일을 main 기반 후속 브랜치에 복구했다. 제품 runtime 차이는0, 사용자 .claude/·logs/ 변경0, 새 공개 전송0이다. 실제 native Linux AMD64의 N15-C만 완료이며 N15 전체 지원과 N17 실제 운영 복구는 미완료다.

## 우선순위와 완료 기준

| 순서 | 대상 | 목적·필요 기능 | 남은 수용과 증거 |
|---|---|---|---|
| P0 | 원장/완료 체크 | 기존 증거 복구와 부분 체크의 실제 커밋 결합; 완료 오표시 방지 | 부분5개(N15-A/B/C,N17-A/B)의 명세·manifest·원자료SHA·task 참조 일치, 누락/중복/미체크/거짓CI 거부. 전체8/18 유지 |
| P1-a | N11 | 튜닝 전 DEV/FINAL 원문 gold·독립 cluster·오염 범위 및 runner 보호 동결 | DEV7/프로토콜 실제 판정; FINAL9후보/6제안군의 독립성 검토와 CODE/WHY/POLICY/TRACE/ABS 각10독립단위 충분성; source/model/config/binary/dataset·예약 결합 |
| P1-b | N12 | 정책·ADR·메타모델/명세/기준의 실제 내용과 관계 검토 | 27항목+6주장/12관계 판정·검토자; 실제 coordinated source/lock/canonical 관계; 활동 시간 미측정은null |
| P2-a | N08 | 근거를 정확하게 검색하도록 DEV 실패 개선 | N11 사전 동결 후 동일8arm 평가·독립 FINAL 회귀와 기존Recall/MRR/중요군 임계치·안전0 유지 |
| P2-b | N09 | 답이 없는 경우 근거와 기권을 정확히 구분 | 원문 답가능/부재 대조; strict인용0과 실제 사람이 답변 기권해야 하는지를 별도 판정; 오인용 증가0 |
| P2-c | N10 | 실제 비용·지연 및 실패 분모 유지 | 같은8arm 직렬 회전·warm2+20·freshCKS3·retrieval5; 실제K/MID/크기/논리·HTTP 시도·오류/incomplete; paired p95비1.25/표본 충분성 |
| P3 | N14 | 실제 수정과 사람의 의미 수용을 데이터 승격에 연결 | ADR→요구→기준→code/test→실제patch/run→사람criterion decision→재색인/승격/거부rollback. merge 승인을 criterion별 수용으로 대체하지 않음 |
| P4-a | N15 | 실제 지원 대상과 외부 소비자 수용 | 완료native C 이력 유지; 운영 규모/모델과 독립 소비자의 설치·실제binary upgrade·지원 한계 판정 |
| P4-b | N16 | 운영 권한·키 신뢰·고지의 실제 관리 | 실제역할/scope/schema·공개키DER fingerprint/독립신뢰·교체/폐기/보관·native/전이 고지 적합성 판정 |
| P4-c | N17 | 실제 장애·백업·서비스 복구 | 실제담당·RTO/RPO/키복구·network/전원 정책과 대상호스트 운영 drill; mock 실행시간을 RTO로 바꾸지 않음 |
| P5 | N18 | 전체 증거에 따른 기본값·출시 결정 | N08–17 실제 수용 및 잔여/제한 검토. small n/inconclusive 또는 disabled/withheld라는 이유만으로 전체 완료 처리하지 않음 |

P1-a/b와 P4 운영 입력 요청은 동시에 받는다. 응답을 기다리는 동안 원장 보호와 실제 patch 수용 자료를 준비한다. 승인 없는 DEV/FINAL 검색·의미 accepted·운영 키/역할/지원/출시 사실을 기록하지 않는다. 기존 승인 모델 BGE-M3 bytes/1024·8arm·기존 임계치/프로토콜 및 입력5개·원 FINAL은 그대로 유지한다.

## 실행 로그

- 기존 전체8/18 완료 기록과1153 source/dependency bindings: PASS. 실제 체크5개를 전체5개 완료로 중복 계산하지 않음.
- 2026-10-08 새 실패 재현: N15-C 체크를 유지하면서 native CI evidence를 존재하지 않는 revision/file로 바꾸고 ci_status=not_run으로 바꿔도 이전 progress checker가 exit0/PASS를 출력했다. 원자료는 별도 보관한다. 실제 CI 실패나 진짜 원자료 훼손은 아니며 복사 ledger의 음성 제어다.
- 이 빈틈을 수정하고 부분 조건을 커밋 당시 명세/manifest/원자료/task 참조에 연결한다. 검증 전에는 이 수정의 완료를 주장하지 않는다.

현재 남은 전체 N08/N09/N10/N11/N12/N14/N15/N16/N17/N18. 다음 P0의 같은 실패 제어 및 정상 기록 더블 체크, N11/N12 실제 응답 반영과 독립 gold 준비다. 전체 목표는 미완료이며 기존 앱 목표의 blocked 상태를 완료로 바꾸지 않는다.

P0 검증 완료: 같은 missing-CI 음성 제어는 새 checker에서 거부되며 실제 커밋 증거5부분체크 정상·8개 구조 시험 통과. [원자료](../../system/eval/b0-knowledge-system/refactoring-progress-guard-2026-10-08/verification.json). 전체 완료 수는8/18·요청5/15·잔여10개 그대로다. 다음 N14 실제 변경/기준/시험 검토 자료 준비 및 N11/N12 응답 반영이다.

N14 세부 준비 완료: [실제 변경 검토 자료](./N14-ACTUAL-CHANGE-REVIEW.md)의 실제PR97 7변경 파일/11span/정확5test run·pass를 확인했다. 실제 reviewed canonical trace·hold 후보/patch register·사람 기준 판정·promote/rollback은 미완료이며 전체8/18·요청5/15·잔여10개를 유지한다.

N11 다음 세부 단계: [후보 범위/출처·오염 검토](./N11-COVERAGE-CANDIDATE-REVIEW.md)의 신규7/과거4 중 같은관측군2를 제외해9후보를 유지했다. DEV04 좌표이동을 교정하고 excluded 재삽입 RED→GREEN/22시험·CLI/runner dispatch0을 확인했다. 승인FINAL0·전체8/18·요청5/15·잔여10개, 다음 실제 gold/범주/군/오염·무답 검토와 충분한독립범위 준비다.

N12 다음 세부 단계: [현행 canonical 연결 준비](./N12-CANONICAL-SOURCE-PREPARATION.md)의 현행 원문69참조와 실제 canonical36참조를 확인하면서 공통 SQLite 조회의 ID 누락을 발견·수정했다. root source capture는 public.pem 정책 거부로 미완료이며 source-only 자료와 분리한다. 다음 N11/N12 실제 판정 반영과 approved source scope의 capture/lock 결합, 이후 N14 실제 변경 수용이다. 전체8/18·요청5/15·잔여10개를 유지한다.
