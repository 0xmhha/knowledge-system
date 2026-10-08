# 남은10개 완료를 위한 우선순위·실행 계획

2026-10-08 최신 하위 단계: **N12 생산 shared-alias matcher→stage2→retained V2 통합 회귀**. [검증 문서](./N12-SHARED-ALIAS-INTEGRATION.md) — 소유한 proposed 개념2/9개의 영문·한글 동률과 상한 fallback·기준 scored 인용/정제 근거 보존·변조10거부; 관련 race116시험104하위시험 및 vet/boundary/format 통과. 실제 파일럿 source/store는 열거나 변경하지 않았고 사람 판정/새 FINAL0이다. **전체8/18·요청5/15·남은10개 유지**. 다음 실제 N12 명세 범위·개념/관계 및 N11 gold/독립성 판정과 부족 표본 보강. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 이전 단계의 원자료는 각 커밋에서 보존한다.

2026-10-08 최신 하위 단계: **N14 exact 실행 보고서/검토 경로 일치 검사 보완**. [수정과 검증](./N14-EXACT-RUN-PROOF-VALIDATION.md) — 수정 전15불일치 수용 RED, 수정 후16거부·multiple proof/nested package 정상 사례와 관련 race/vet 통과. 기존 실제 held 후보의 generic/exact10요청은 명령 시작 전에 거부됐으며 linked report/사람 decision/current0, 원 권위9파일 불변이다. **전체8/18·요청5/15·남은10개 유지**. 다음 N12 명세 범위·관계 및 N11 gold/독립성 판정 반영 후 실제 N14 후보/시험/수용이다. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 아래 기록은 이전 단계 증거를 보존한다.

2026-10-08 최신 하위 단계: **N12 누락 개념 경로5개 범위 대조·실제 retained matcher→stage2→V2 검증3사례 완료**. [범위 검토와 명세 초안](./N12-CONCEPT-PATH-SCOPE-REVIEW.md) — source18참조/관련5패키지17시험·32하위시험/변조6거부/권위9파일 불변. 기존 미작성5개·reviewed 경로0은 유지하며 기능 개념3개·requirement version2 초안은 미적용이다. **전체8/18·요청5/15·잔여10개**. 다음 명세 범위 선택 및 N11 gold/독립성·N12 개념/관계 판정. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 아래 기록은 이전 단계의 증거를 보존한다.

2026-10-08 다음 단계 검증: **N12 원문 기반 proposed 구현·시험·기준 관계13개 준비**. [검토 자료](./N12-PROPOSED-TRACE-RELATIONS.md) — 부정확한 테스트 연결1개 제외·미작성 개념 경로5개 유지, 생산API/SQLite/두archive 재생·9거부 제어 통과. **N11 pool13문항/8제안군과 승인0은 그대로이며 관계13개를 FINAL 표본으로 세지 않는다.** 전체8/18·요청5/15·잔여 N08/09/10/11/12/14/15/16/17/18 10개. 다음 실제 관계·명세 범위 판정 및 미작성 경로/독립 평가 입력 보강이다.

2026-10-08 현재 단계: **N11 독립 후보 보강·실제 판정 대기**. [새 검토 문서](./N11-INDEPENDENT-POOL-EXPANSION.md)의 검토 대상FINAL13/8제안군, DEV7, 제외6을 준비했다. 중요군별 승인0·전체8/18·요청5/15·잔여10개를 유지한다. **다음:** 후보 gold/군/독립성·N12 실제 내용 판정과 부족 범위 보강. **남은 전체:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 실제 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 복구, N18 최종 판정. 아래 날짜별 기록은 이전 준비 이력을 함께 보존한다.

2026-10-08 · 기준 main eeeab2e7(PR97 squash) · 후속 브랜치 feat/refactoring-followup · 전체8/18·요청5/15 완료·잔여10개. 기존 preview 종료와 상위 WI10개는 이 목록에 합산하지 않는다.

## 목적과 현재 근거

원문·정책·요구·코드·시험의 연결을 실제 검토할 수 있고, 갱신/복구 후에도 근거가 보존되는 시스템을 완성한다. 이전 작업을 단지 코드/시험이 존재한다는 이유로 전체 수용 완료로 바꾸지 않는다. 새 검색 gold와 관계의 정확성·실제 운영 사실은 도구 실행과 분리한다.

PR97 head d7de0b89의 CI5/5 성공 후 사용자 지시로 main에 스쿼시 머지했다. GitHub main·origin/main·local main은 eeeab2e7이며 테스트한 공개 head와 Git tree가 같다. 기존 로컬 증거483파일을 main 기반 후속 브랜치에 복구했다. 제품 runtime 차이는0, 사용자 .claude/·logs/ 변경0, 새 공개 전송0이다. 실제 native Linux AMD64의 N15-C만 완료이며 N15 전체 지원과 N17 실제 운영 복구는 미완료다.

## 우선순위와 완료 기준

| 순서 | 대상 | 목적·필요 기능 | 남은 수용과 증거 |
|---|---|---|---|
| P0 | 원장/완료 체크 | 기존 증거 복구와 부분 체크의 실제 커밋 결합; 완료 오표시 방지 | 부분5개(N15-A/B/C,N17-A/B)의 명세·manifest·원자료SHA·task 참조 일치, 누락/중복/미체크/거짓CI 거부. 전체8/18 유지 |
| P1-a | N11 | 튜닝 전 DEV/FINAL 원문 gold·독립 cluster·오염 범위 및 runner 보호 동결 | DEV7/프로토콜 실제 판정; 현행FINAL13후보/8제안군의 독립성 검토와 CODE/WHY/POLICY/TRACE/ABS 각10독립단위 충분성; source/model/config/binary/dataset·예약 결합 |
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

2026-10-08 N12 캡처 입력 준비 검증: [명시 소스 범위의 보관 캡처](./N12-SCOPED-CAPTURE-PREPARATION.md) — 원본667a9af5에서 소스·의존·검토 문서1,410개를 독립 준비 Git 커밋562e37aa로 구성해 committed 캡처/복원·archive-only 재생/신원·69검토span·승인Go29·8거부 제어를 확인했다. snapshot-only는 build31/vendor fixture1 등32개를 제외하므로 전체 입력 보관으로 사용하지 않았다. 제품 정책/소스 변경0, 새 모델/FINAL질의·사람 판정0. 제안 입력 범위·내용/관계 승인 및 새 coordinated CKG/CKV/lock 결합은 미완료, 전체8/18·요청5/15·잔여10개 유지.

2026-10-08 N12 기술 연결 검증: [동일 신원의 held 색인·의미 준비](./N12-COORDINATED-PREPARATION.md) — 보관소스1,410파일/선택58의 CKG53Go·CKV56입력·666chunks/566심벌 canonical100%, 문서27/27·코드36/고유33·proposed 저장 roundtrip·12거부 제어·archive-only 재생 통과. 원문project knowledge-system로 수정한 별도 후보이며 첫 mismatch 거부와 전체Go범위 DRIFT998 경고를 보존했다. 실제 BGE embedding HTTP124회(색인입력1332·시작probe4), 새DEV/FINAL0·human0·current/semantic activation0. approved overlay/lock·내용/관계/입력범위 판정은 남아 N12-A–D/전체8/18·요청5/15·잔여10개를 유지한다.

2026-10-08 N12 ADR 기술 lock 연결: [검토 대기 ADR의 보관 lock](./N12-PROPOSED-LOCK-PREPARATION.md) — 원소스1,410 불변, proposed ADR2/정책0(운영 owner 미정)/review0의 partial overlay·lock과 별도 held snapshota0457cc1/dataset5a24904f를 생성했다. 생산 knowledge·include-packs CLI/v4 의미27문서·36코드참조/33canonical·7 lock 거부·원 소스/색인/semantic DB 없이 archive 재생 통과. 새 BGE embed HTTP62=probe2+색인60/666입력, 전체Go DRIFT998 유지. 승인 lock/사람 내용·관계·운영 판정은 미완료, N12-A–D unchecked·전체8/18·요청5/15·잔여10개 유지. 다음 N11 실제 gold/독립성 및 N12 판정/정책 담당자·시점 확인.

2026-10-08 N11 검토 대기 코퍼스 연결: [코퍼스·색인·실행 신원 준비](./N11-HELD-CORPUS-PREPARATION.md) — 이전 N12 색인의6Go 누락/ABS2보관 누락을 실제 대조하고 별도 Git19803801/snapshotb1204cf1/datasetee6f91be의 source1,419/선택66·CKG59/CKV63/785chunks/667canonical을 결합했다. 원문16/16·positive14/14 실제 canonical/chunk·ABS2 uncited basis, 무답 basis 검사 누락 RED11→GREEN25와 실제 DEV/FINAL runner child/output/registry0, 원본 source/index/semantic DB 없이 archive 재생 통과. 새 embedding HTTP71/색인785/probe3, 전체Go DRIFT992 유지. scope/gold/군·독립성/사람 판정/동결은 미승인·FINAL0, N11-A–D unchecked·전체8/18·요청5/15·잔여10개 유지. 다음 N11 실제 판정/독립 FINAL 보강과 N12 판정이다.

2026-10-08 N11 후보 보강: [독립 FINAL 후보·범위 부족분](./N11-INDEPENDENT-POOL-EXPANSION.md). 신규CODE/WHY4문항/2제안군 추가, 기존 사실과 겹칠 위험의 신규4문항 격리. 검토 대상FINAL13/8제안군·DEV7, 승인된 중요군 모두0. 과거DEV/FINAL27gzip/11,632행 및 실제20원문 참조 대조, 정보 메타데이터 교정·새runtime 미결합, 실제input checker exit2/source오류0; 모델/검색/빌드0. 전체8/18·요청5/15·잔여10개 N08/09/10/11/12/14/15/16/17/18 유지. 다음 N11/N12 실제 내용·군/독립성 판정과 부족 범위 보강.

2026-10-08 N12 trace 연결 준비: [13개 proposed 관계와 의미 경계 검증](./N12-PROPOSED-TRACE-RELATIONS.md). 구현3/시험5/기준5 관계를 별도 SQLite에 원문·canonical·CKV로 연결하고 재조회/원본 없는 두 archive 재생·9거부 제어를 통과했다. Projection.Validate 시험을 다른 정렬 함수에 연결하는 부정확한1쌍은 제외, broad concept5경로 미작성 유지. 전체25관계 proposed·3요구 spec_unapproved·검토된경로0/current없음/human0/model0. 신규FINAL로 합산하지 않으며 N11 pool13/8·승인0, 전체8/18·요청5/15·잔여10개 유지. 다음 관계/원문 범위 판정·5경로 보강과 N11 독립 입력 확보다.
