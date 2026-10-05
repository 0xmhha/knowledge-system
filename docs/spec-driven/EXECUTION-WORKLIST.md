# 실행 작업리스트와 완료 재검증

작성: 2026-10-03 · 브랜치: `feat/spec-driven-knowledge-system` · 시작 코드/문서: `329becf3`

사용자 지시: 작업리스트를 문서화하고, 완료된 항목도 다시 확인하며, 우선순위대로 전체 작업을 수행한다. 실행 기준은 [DELIVERY-PLAN-V2](./DELIVERY-PLAN-V2.md)의 D → A → B → C다. 이 문서는 실행 상태와 다음 행동을 관리한다. [EXECUTION](./EXECUTION.md)은 구현 이력, 각 게이트 보고서는 판정 증거다.

## 상태와 완료 규칙

`검증 완료`는 명시한 범위의 현재 증거를 확인한 상태다. `역사적 통과`는 과거 실행 기록은 있지만 이번 머신에서 다시 실행하지 않은 상태다. `진행`은 산출물을 만들거나 검사 중, `대기`는 선행 작업이 필요함, `사람 검토 대기`는 실제 승인 기록이 필요한 상태다. 실패 결과를 기록하거나 도구만 구현한 것으로 해당 품질 게이트를 완료하지 않는다.

작업 종료에는 입력·코드·명령·원자료·판정·제한을 연결한다. 사실 검토, 테스트 성공, 품질 합격, 운영 출시를 각각 판정한다. 최종 질문은 개발 튜닝에 사용하지 않는다. 소표본의 반복 실행은 독립 표본 수를 늘리지 않는다. 임계치나 정답을 관측 결과에 맞춰 바꾸지 않는다.

현재 사람 결정(2026-10-06): 정적 질문 12개·BGE-M3에 이어 **Go 소스 29개 포함·프로토콜·F-01–06·SF-03/05-A/05-B/06의 테스트 사례 범위·STV2 설정**을 사용자가 명시적으로 승인했다. [승인 입력 기록](./B0-APPROVED-INPUTS.md)에 원문·시각·전후 SHA를 연결했다. 검토자 `chat-user`는 로컬 식별자이며 인증된 실명은 아니다. 이전 “검토 후 결정”은 이력으로 보존하며 현재 승인 대기로 취급하지 않는다. 이전 커밋 `329becf3`의 19개 파일은 사용자가 해당 브랜치 푸시를 승인했고 원격에 반영됐다.

매 턴 마지막 보고에는 **현재 단계와 이번 변경/검증 → 바로 다음 작업 → 남은 전체 작업**을 함께 표시한다. 완료/진행/대기를 시각적으로 구분하고 품질 게이트 완료와 도구 준비를 따로 센다. 사용자의 2026-10-04 지시를 이후 재개에도 유지한다. 최신 요약은 [진행 현황판](./EXECUTION-STATUS.md), 상세 선행·증거·완료 조건은 이 문서가 기준이다.

<!-- official-gates: {"B0":"pending","B1":"pending","C0":"pending","C1":"pending"} -->

공식 게이트 판정은 위 원장과 실제 보고서 증거로 갱신한다. pending을 도구 준비만으로 pass/fail/inconclusive로 변경하지 않는다.

## 1. 완료 표시 재검증 목록

| ID | 항목 | 이번 재검증 상태 | 증거·정확한 범위 | 남은 확인 |
|---|---|---|---|---|
| R-01 | D1–D5 설계 승인 | 검증 완료 | DESIGN-GATES-EVIDENCE, D5-DECISION-REVIEW, DELIVERY-PLAN의 승인 기록 확인 | 개별 정책 사실의 승인과 별개 |
| R-02 | A0–A3 필터·모델 신원·세 계층 신원 | 검증 완료 | A8 과거 통과 기록 존재; 최종 전체 Go 시험 통과 | 최종 전체 Go 시험 통과; 원자료 원장 참조 |
| R-03 | A4 캡처·보관·입력·인용 | 수정 후 재검증 | 실제 v2에서 끝 개행의 가상 줄 인용을 발견; 생성기 수정 전 실패/후 통과, 실모델 5응답·25본문 SHA/좌표/integrity 확인 | 기존 전체 코퍼스 진단은 수정 전 빌더; 공식 새 빌드 필요. 파일 집합 동일성 별도 |
| R-04 | A5.1–A5.5 의미·팩·검토·패치 | 검증 완료 | 과거 정상/실패/이전 호환 기록; 최종 전체 Go 시험 통과 | 실제 운영 정책을 자동 verified로 만들지 않음 |
| R-05 | A6 선택형 의미 문맥 | 검증 완료 | v2 문맥·폴백 시험 기록 존재 | 네 가지 B1 MCP 순위 경로 구현 검증 완료; 공식 품질 비교 별도 |
| R-06 | A7.1 CLI/MCP·마이그레이션 | 검증 완료 | 과거 공개 소비자 재생; 최종 전체 Go 시험 통과 | 신원·캡처·팩·패치·세 엔진·구 v1 재생 스모크 통과 |
| R-07 | A7.2–A7.5 플랫폼 패키지 | 검증 완료 | macOS arm64/Linux arm64·amd64 mock 설치·시험 서명 기록 | macOS preview·시험 서명 및 수정 후 Linux arm64 통과; amd64 에뮬레이션 재실행도 통과. 운영 출시·실모델 지원과 별개 |
| R-08 | A8 통합 | 검증 완료 | 전체 Go 시험·vet·관련 race·경계·문서·실제 스모크 통과 | 논리/HTTP 계측의 전체 Go·관련 race/vet·경계와 8경로/기존 v1/v2 재검증 통과; 최신 문서 검사·원자료 원장 참조 |
| R-09 | B0-H 이력 봉인·실행 비트 | 검증 완료 | gate 보고서와 시험 코드 존재; 최종 전체 Go 시험 통과 | 전체 시험의 이력/실행 비트 실패 주입 통과; 큰 Linux 운영 비용은 미측정 |
| R-10 | 새 머신·BGE-M3 실측 | 검증 완료 | environment/verification-m2max JSON, 실제 digest·1024차원·Ollama 0.35.1 | 모델 변경 전후 재검사 |
| R-11 | 독립 고정 코퍼스 | 검증 완료 | commit `71cb71cd55960833e930269e272f7a4a060be3aa`, tree `f020f8f30fd209b8de045756dedd12ff83f65cd9`; alternates 없음 | 새 실행도 같은 독립 입력 사용 |
| R-12 | 정답 12개 승인·모델 확정 | 검증 완료 | human-review-m2max JSON, questions 12/12 승인, approved preflight | 동적 사례 승인과 구분 |
| R-13 | 전체 코퍼스 실모델 진단 | 부분 검증 | 선택된 1,569입력/13,575청크·해시·분할 재조립·strict embedding 검사 통과 | Go 29개 제외 때문에 범위 감사 partial; 공식 품질·통제된 지연 기준선 아님 |
| R-14 | 실모델 소형·장문 스모크 | 검증 완료 | verification-m2max JSON; 작은 실제 조회와 strict 장문 입력 통과 | 전체 질문 품질을 입증하지 않음 |
| R-15 | B0/B1/C0/C1 완료 여부 | 미완료 확인 | 공식 품질 지표 null, 동적/프로토콜 draft, release preview | 아래 전체 실행 목록 |

현재 입력 진단과 승인 기록은 [B0-M2MAX-GATE-REPORT](./B0-M2MAX-GATE-REPORT.md), 과거 구조 시험은 [A8-GATE-REPORT](./A8-GATE-REPORT.md)와 [B0-H-GATE-REPORT](./B0-H-GATE-REPORT.md)를 사용한다. 원자료는 `system/eval/b0-knowledge-system/`에 있다.

## 2. 우선순위와 실행 순서

P0는 잘못된 완료 판정과 입력·검토의 결손을 먼저 해결한다. P1은 평가 실행 경로와 재현 자료, P2는 공식 비교와 실패 수정, P3는 출시 판정이다. 병렬로 준비할 수 있는 도구·픽스처는 승인 대기 중에도 작업한다. 승인 전 공식 점수로 게이트를 닫거나 미검토 의미 사실을 생산 데이터로 승격하지 않는다.

| 순서 | 우선순위 | 작업 | 현재 상태 |
|---|---|---|---|
| 1 | P0 | 완료 재검증 R-01–15, 범위 누락·이력 증거 확인 | 구조 재검증 통과; 세 플랫폼 mock 패키지 재검증 통과 |
| 2 | P0 | B0-01 입력 범위, B0-02/03 프로토콜·동적 검토 | 승인 기록·입력 정의 검증 완료 |
| 3 | P1 | B0-04/06 재현 픽스처·측정 실행 도구 | 개발 6개 준비·원응답/exact/budget·회전/환경/입력 사본 도구 검증; 승인된 공식 입력 연결 대기 |
| 4 | P1 | B1-01/02 실제 ablation 어댑터·팩 축 | 네 경로와 합성 팩 off/on 8경로 구현 검증 완료; 운영 사실/공식 판정 대기 |
| 5 | P1 | B0-05 의미 사실·팩·canonical 출처 결합 검토 | 테스트 사례 승인 완료·새 source-bound 입력 결합 진행 |
| 6 | P2 | B0-07–09 공식 기준선 | 선행 대기 |
| 7 | P2 | B1-03–08 동일 조건 비교·판정 | B0 대기 |
| 8 | P2 | C0-01–06 관측 실패의 스펙·시험·수정 | B1 대기 |
| 9 | P3 | C1-01–07 재평가·운영·출시 결정 | 공식 품질은 C0 대기; C1-04/05/06 preview 준비 진행 |

## 3. B0: 승인된 입력과 재현 가능한 실모델 기준선

| ID | 우선순위·선행 | 작업과 산출물 | 완료 조건 | 상태 |
|---|---|---|---|---|
| B0-01 | P0 · R-13 | CKV 기본 제외로 빠진 `internal/vector/build/` 29개를 목록·해시로 고정하고 기준선 범위를 결정한다 | 기존 범위 유지 또는 정확한 소스 포함 정책을 명시; 모든 arms 동일 입력; 비밀·링크·산출물 필터 유지 | 검증 완료: 승인한 정확한 29개 SHA/경로만 예외; 실제 discovery 1,569→1,598 차이 일치·수정 전 실패/관련 Go/race 검증. 신규 strict 빌드 감사는 B0-07 |
| B0-02 | P0 · R-12 | 정적 개발 4/최종 8, 검색 5회, warm 20회·2 warmup, 별도 cold, 지표·회귀 임계치·소표본 규칙을 검토하여 프로토콜 동결 | 승인자·시각·원문 해시와 승인 프로토콜; 결과를 본 뒤 변경하지 않음 | 검증 완료: 명시적 프로토콜 승인·원문/시각/해시 결합; 분할·반복·임계치·소표본 규칙 동결, input-definition checker 통과 |
| B0-03 | P0 | F-01–F-06의 개발/최종 입력·정답·실패 오라클 검토 | 12변형 검토 기록·원문 해시·독립 표본 단위 확정 | 검증 완료: F-01–06 DEV/FINAL 12변형 승인자·시각·해시 결합; source hash/분할/오라클 선언 검증 통과 |
| B0-04 | P1 · B0-03 | 초안 JSON의 입력을 실제 독립 저장소/커밋으로 생성. F-04는 순차 두 상태, F-05는 서로 다른 프로젝트 | 선언된 파일 SHA·줄·Git 독립성 일치; 재실행 바이트 동일; 기존 디렉터리 덮어쓰기 거부 | 검증 완료: 승인 DEV/FINAL6+6 가족의 독립 소스/커밋/트리 생성; 개발 회귀·최종 두 저장소 반복 생성 bytes 일치·16상태 출처 검사. 최종 질의는 실행하지 않음 |
| B0-05 | P1 · B0-04 | F-03/05/06에 필요한 의미 사실·팩·스펙·관계·코드 앵커를 fixture 자기 스냅샷에 결합 | 검토 기록과 canonical ID·보관 줄 해시 일치; 미검토 자동 승격 0 | 검증 완료: DEV4+FINAL4 의미/정책/코드 앵커를 각 실제 snapshot·보관 줄·native canonical로 결합. FINAL은 승인 정의와 동일 SF 사실 치환 증거; F-03 proposed·acceptance 비승인 유지. 승인 FINAL 입력 archive/source-vector 감사 참조 |
| B0-06 | P1 | 실모델 측정 실행 도구: 전체 v1/v2 원응답, 직접 CKV exact 오라클·budget, model digest 전후, 지연·호출 수·크기 기록 | 입력 해시·실행 바이너리 SHA·모델·좌표·순서 고정; 누락/예외도 원자료; gold 답 본문을 검색 입력에 전달하지 않음 | 검증 완료: 승인 v1 anchor4×5·계측 SDK20 및 v2 DEV3840 전체 응답/measurement ID/backend K·호출/크기·warm/cold·model/입력/바이너리 전후 SHA 결합. direct raw16 full eligible/exact/상한·F01 자격·F02 동점 차이 보존. FINAL은 C1-01 |
| B0-07 | P1 · B0-01/02/03/05/06 | 공식 데이터셋을 동결된 입력으로 빌드하고 보관 원문·그래프·벡터를 감사 | strict embedding·선택 범위·input hashes·split 재조립·정렬·doctor 통과; 빌드 지연 별도 계측 | 검증 완료: 승인 정적1,598파일/13,820벡터 + DEV8/FINAL8 각150벡터 strict 빌드·모델 pin·보관 원문/분할/DB orphan0 감사. 입력/빌더/소스/검토/정체성 결합. 쿼리·품질 판정은 B0-08/09 |
| B0-08 | P2 · B0-07 | 공식 정적 `cks eval --verify-anchors`와 F-01–06 실모델 실행 | 승인 gold와 원응답 연결; 최종 입력 독립; cold/warm 혼동 없음; F-01 자격 실패는 fixture_not_qualified | 검증 완료: 승인 strict 정적 DEV4×5와 동적6가족/8상태·8arm DEV3840 실제 수집 및 source/공개 Go 감사 오류0. static miss3/pass1·미검토 의미 비승격·state guard/stale 오라클 보존. 독립 FINAL은 C1-01 |
| B0-09 | P2 · B0-08 | B0 보고서 작성: 정답·범위·환경·지표·실패·제한 | 승인 입력과 재현 명령·원자료 포함; 실패/기권/소표본을 성공으로 합산하지 않음 | 검증 완료: [승인 DEV 보고](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md)에 정답/범위·환경·원자료·검색 실패·지연/비용·소표본 제한 연결. baseline 품질 개선/사람 답변/출시 pass로 바꾸지 않음 |

F-01 희소 필터/후보 상한, F-02 장문 꼬리와 부모 재조립, F-03 다의어/미등록, F-04 오래된 상태·과거 인용, F-05 프로젝트 격리/서로 다른 정책, F-06 구현·정책 충돌 및 무관 테스트를 각각 확인한다. 검토 세부 내용은 [B0-M2MAX-REVIEW](./B0-M2MAX-REVIEW.md), 동결 후보는 `dynamic-fixtures-m2max-draft.json`과 `protocol-m2max-draft.json`이다.

## 4. B1: 검색·온톨로지·도메인 팩 비교

| ID | 우선순위·선행 | 작업과 산출물 | 완료 조건 | 상태 |
|---|---|---|---|---|
| B1-01 | P1 | baseline / concept_text / relations / combined의 네 가지 실행 어댑터 구현 | 실제 실행 경로가 다름을 검증; 기본 꺼짐; 원문 질의 우선; 후보 K 보존·오류/모호성 폴백; 단순 플래그 이름만 추가하지 않음 | 검증 완료: 네 MCP 경로·텍스트/관계 기여 분리·상한/폴백, mock/BGE-M3 각 20요청. 공식 품질 통과와 별개 |
| B1-02 | P1 · B0-05/B1-01 | 팩 off/on 축을 연결하여 8 arms 구성 | 팩 잠금과 데이터셋 신원 분리; 권한·범위·출처 적용; 단순 Markdown을 검토된 팩으로 취급하지 않음 | 구현 검증 완료: 동일 dataset/lock의 8개 v2 경로, mock/BGE-M3 각 48요청·6정책 상태·인용/본문 보존·원문/integrity·DB 무변경 확인. 실제 팩 사실 검토·공식 B1 대기 |
| B1-03 | P2 · B0-09/B1-02 | 같은 모델·입력·질문·K·필터로 paired 실행, arm 순서 회전 | 동시 지연 시험 없음; warm/cold 분리; 반복은 독립 표본으로 합산하지 않음 | 검증 완료: 승인 동일 모델/입력/질문/K·범위와 회전8arm DEV3840·입력 전후 불변. SDK 오류/계측 누락0, repeats와 가족 unit 분리. FINAL 비교는 C1-01 |
| B1-04 | P2 · B1-03 | Recall@10, MRR, precision, 기권, 무관 인용, 오개념·정책 오용·근거 없는 이유를 질문군별 산출 | 평가 분모·누락·실패 기록; v1 인용 0개와 사람의 답변 기권 판정 분리 | 진행: 승인 DEV3840·정적 v1 anchor20/raw 계측20, native raw16·가족 오라클 및 source/공개 Go 감사 완료. static Recall0.25/MRR0.125와 3개 miss, F05/06 모든8arm MRR0.75, F02 동점 순서 차이를 [실제 개발 보고](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md)에 기록. HC01–08 출력 해석/사람 기권 판정과 독립 FINAL 결과 남음 |
| B1-05 | P2 · B1-03 | 좌표·비밀·권한·프로젝트/상태 혼입 검사 | 안전 위반 0; 기본 후보 보존; 미검토/만료/충돌 의미를 확정 사실로 사용한 사례 0 | 검증 완료: DEV3840 출처·좌표/public Go 오류0, F03 proposed 비승격·F04 상태·F05 프로젝트·F06 acceptance/test trace 비승격 및 native10호출 혼입/손상/없음·backend0·private payload 비노출/복원 SHA 확인. 기존 권한/sanitization 회귀 포함; 일반 비밀 탐지/운영 ACL 보증 아님 |
| B1-06 | P2 · B1-03 | warm p50/p95·cold·호출 수·크기·검토 비용 | 순서와 원시 시간 보관; build/query 별도; 환경 경쟁 작업 기록 | 진행: DEV3840의 warm p50/p95·cold·backend 호출·응답 크기·환경 원장 완료. static/dynamic/pooled 원시 관측을 각각 집계하며 combined_on p95비1.292/1.614/1.065와 한계 보존. 사람 검토 비용은 미측정이며 FINAL 비용·사람 기록 남음 |
| B1-07 | P2 · B1-04/05 | 답변 주장·정책·기권에 대한 사람 판정표 작성/검토 | 주장별 근거와 판정자 연결; 자동 구조 점수와 사람 승인을 분리 | 대기: [실제 출력 주장 검토표](./B1-APPROVED-OUTPUT-CLAIM-REVIEW.md) HC01–08 및 비용 응답 요청 완료. 입력①–⑤ 승인을 출력 해석/기권·사람 비용 승인으로 대신하지 않음 |
| B1-08 | P2 · B1-04–07 | paired 비교와 실패 보고서 | 기존 회귀 조건 적용; 불확실 구간·소표본 inconclusive 명시; 최종 사례로 튜닝하지 않음 | 진행: [실제 개발 보고](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md)에 DEV3840 paired·실패/한계·1096소스 동결·FIX22–28 증거 연결. 반복/언어/상태를 독립 표본으로 세지 않으며 작은 중요 질문군 inconclusive. FINAL 비교와 HC/비용 판정 남음 |

기존 설계 회귀 조건: 안전/좌표/비밀 혼입 0, 기권 오인용 증가 0, 전체 Recall@10·MRR 변화 ≥ −0.02, 중요 질문군 변화 ≥ −0.05, warm p95 비율 ≤ 1.25. 작은 최종 질문군의 결론 규칙은 B0-02 검토 대상이다. 수치가 충분하지 않으면 기본 활성 근거로 사용하지 않는다.

## 5. C0: 관측 실패에 따른 수정

| ID | 우선순위·선행 | 작업과 산출물 | 완료 조건 | 상태 |
|---|---|---|---|---|
| C0-01 | P2 · B1-08 | 안전·정확성·품질·지연 실패 분류와 우선순위 | 각 실패가 실제 원자료·질문·arm·좌표에 연결; 안전/정렬 먼저 | 검증 완료: 실제 원자료에 FIX22–28 원인, static raw rank1085/47·52/301·1490의 recall 제한, F02 동점 순서, static/dynamic/pooled 비용을 분류. 소표본·optional 비용/사람·운영 제한은 해결된 품질 pass로 바꾸지 않음 |
| C0-02 | P2 · C0-01 | 실패의 요구사항·스펙·개발 골든/재현 사례 작성 | 원래 계약과 기대 동작 설명; 최종 gold를 성공에 맞춰 고치지 않음 | 검증 완료: exact Go29 발견·stdio/계측·v2 body budget·동일 canonical header boost·요청 내 retained batch/비캐시·독립 corpus label/dataset audit 요구사항과 재현/gold 연결. 승인 질문/정답/모델/K/threshold 유지 |
| C0-03 | P2 · C0-02 | 수정 전 실패하는 의미 있는 시험 추가 | 해당 원인을 재현하며 단순 구현 복제 시험이 아님 | 검증 완료: FIX22–28 수정 전 실제 실패/의미 있는 시험과 후 회귀를 보존. raw recall 제한은 발견/offset/필터/DB 결함으로 확인되지 않아 query/gold를 hardcode하는 제품 시험을 만들지 않음 |
| C0-04 | P2 · C0-03 | 청크/필터/범위/graph/rerank/budget 중 확인된 원인 수정 | 인과관계·기본 경로·취소·상한·구버전 보존; 불필요한 일괄 재작성 없음 | 검증 완료: 확인된 발견/stdio/provider/budget/헤더 boost/반복 원문 검사/보고 결합 원인 수정. FIX26 .6667→.75 전8arm, FIX27 실제 pack4 전체 동일/시간 감소. optional 비용·baseline 제한은 명시하고 disabled 유지 |
| C0-05 | P2 · C0-04 | 영향받은 공개 계약·회귀·race 검증 | 필요한 검사 통과, 변경된 입력/정체성의 이전 정책 기록 | 검증 완료: 변경 제품 관련/race/vet·root Go 및 보관 출처/배치·native source/mixing 검사 통과. B0 Python75/B1보고26 통과, 역사적 원문5파일 모듈 경계 보존. FINAL 입력에 따른 수정 없음 |
| C0-06 | P2 · C0-05 | 개발 세트에서 개선 확인, 수정 버전 동결 | 실패 재현 해소; 최종 세트 재평가는 C1; 실패가 없으면 수정 불필요 근거 기록 | 검증 완료: 승인 DEV3840·독립 source/Go/raw/state/v1 감사 및 FIX25–27 개선 확인 후 source1096파일·실행 SHA·입력5 SHA를 기술 동결. baseline/지연/소표본 잔여 제한 포함; 사람 판정·품질 pass/출시 승인이 아님 |

## 6. C1: 재평가와 출시 결정

| ID | 우선순위·선행 | 작업과 산출물 | 완료 조건 | 상태 |
|---|---|---|---|---|
| C1-01 | P3 · C0-06 | B0/B1과 같은 조건의 최종 재평가 | 버전 차이·입력 차이·실행 원자료 명시; 독립 최종 조건 유지 | 검증 완료: [승인 FINAL 보고](./C1-APPROVED-FINAL-REPORT.md)에 동결 CKS/BGE/K·8arm·2/5/20/3의4800회, source/공개 Go4560 valid·POLICY03 오류240·raw20·v1 anchor40/계측40·F04 stale와5입력/저장소/1096소스 불변 연결. 실패 상태/분모를 보존하며 평가 합격으로 바꾸지 않음 |
| C1-02 | P3 · C1-01 | 품질·안전·지연·질문군 게이트 판정 | 허용 회귀 충족 또는 실패/불확실 판정; 통과 없는 출시 승인 금지 | 검증 완료: FINAL 실행오류240/4800·양성 정적1/6 회수·v1 두 인용0개 guard fail에 따라 품질fail/출시보류, 중요 질문군 n<10 inconclusive. latency population/같은 축 비교 모두 보존. 운영 stage gate 완료/출시 승인은 별도 |
| C1-03 | P3 · C1-02 | 온톨로지 기본값 결정 | 증거·이점·회귀·검토 비용·사람 결정 기록; 불확실하면 disabled 유지 | 검증 완료: 사용자 승인 프로토콜 n<10의 기본 활성/출시 금지 규칙과 FINAL 실패·비용 불확실성에 따라 disabled 유지. 구현 opt-in 확인, 사람 검토 비용은 미측정. 별도 출력·운영 승인을 만들지 않음 |
| C1-04 | P3 · C0-05 | 최종 macOS/Linux 패키지·실제 Ollama·대규모 비용 검사 | OS/CPU별 추출 설치·롤백·의존·실모델 증거; 환경 없는 대상은 pending/preview | 진행: 최신 FIX-19 세 대상 preview·실모델/복구와 부분 대규모13,575청크의60 SDK·480원문/인용·별도RSS2호출 준비 검증; backend 오류840 보존·FTS 후보8개 재현. native amd64·공식 대규모/최종 운영 후보·품질은 남음 |
| C1-05 | P3 | 운영 서명·신뢰 루트·라이선스 검토 자료와 사람 결정 | 실제 운영 키/정책/검토 권한 확인; 시험 키를 운영 키로 승격하지 않음 | 진행: 연결 모듈 35개·수집 고지 50개·Tree-sitter runtime 44개/Solidity 6개 upstream 일치·시험 서명/설치/상한/변조 거부 재검증; JS/TS·SQLite/vec 원문 41개 일치 및 최신 Linux recipe 시험 서명/설치 통과; FIX-14 Linux paired archive 및 FIX-15 SQLite 추가 고지/세 플랫폼 서명·설치·변조 거부 검증 통과; translated modernc 선택 소스의 ZIP/h1 대조와 [원래 SQLite·musl 출처](./C1-ORIGINAL-NATIVE-ORIGIN-AUDIT.md)의 501입력·SQLite C 공식 SHA3·Linux 헤더436개 분류/생성4 재생 완료 범위 존재; Darwin SDK·전체 생성기/전이 native 범위·[운영 검토표](./C1-OPERATIONS-REVIEW.md)·사람 판정 남음 |
| C1-06 | P3 · C1-04 | 마이그레이션·복구·롤백 문서와 실행 | 구 데이터 원본 보존·재색인/오류 경로·이전 current 복구 확인 | 진행: 최신 Darwin 시험 preview의 후보 실패/손상 거부·pin/update/rollback·새 루트 백업 복원·원본 소스 없는 재생, 구 데이터의 첫 신규 소비자 전 SHA·새 재색인·구 v1 rollback 보존 검증. [복구 문서](./C1-RECOVERY-VALIDATION.md); 최종 후보/운영 범위·OP-08 남음 |
| C1-07 | P3 · C1-02–06 | 지원 매트릭스·known limits·최종 게이트/출시 결정 | 전체 작업 종료 증거 또는 명시적 잔여 범위; 품질 승인과 배포 실행 권한 별도 기록 | 대기 |

## 7. 실행 증거 원장

| 날짜 | 작업 | 실행·결과 | 원자료 |
|---|---|---|---|
| 2026-10-03 | 문서화·우선순위 | 완료 항목 15개와 B0/B1/C0/C1 30개를 분리해 재검증/선행/종료 조건 작성 | 이 문서 |
| 2026-10-03 | R-02–09 | 최종 전체 Go 시험, vet, 관련 race, 경계·문서·실제 통합 경로 통과 | [재검증 JSON](../../system/eval/b0-knowledge-system/execution-reaudit-m2max-2026-10-03.json) 및 연결된 원시 로그 |
| 2026-10-03 | REAUDIT-01 | 승인된 gold를 초안으로 가정하던 시험 수정; B0 Python 12개 통과 | `scripts/test_b0_preflight.py`, Python 원시 로그 |
| 2026-10-03 | B0-04 | 개발 6개를 독립 Git 입력으로 생성; 반복 커밋/트리/원문 SHA 일치, F-04 연속 상태·F-05 격리 검사 통과 | `scripts/b0-materialize-fixtures.py`, 개발 materialization JSON |
| 2026-10-03 | B0-06 | `cks eval --record-responses` 구현. freshness/원문 팩 및 실패 기록·시나리오 격리·기본 출력 호환·기록 비용 제외 시험 통과; 실제 mock CLI 원응답 2개 확인 | `internal/system/eval/runner.go`, raw-cli-mock-report JSON |
| 2026-10-03 | REAUDIT-02 / R-07 | Linux 스모크의 `.git` 파일 가정 수정. 추적 입력만 복사하며 수정/삭제/실행 비트/링크·primary/worktree 검사 통과. Linux arm64 재실행 통과, amd64 에뮬레이션 통과 | `scripts/prepare-platform-fixture.py`, `scripts/test_platform_fixture.py`, 플랫폼 로그 |


| 2026-10-03 | B0-06 | `cks eval capture`로 v1/v2 전체 SDK 응답·오류와 ns 시간, 요청/설정/바이너리 전후 SHA·초기화/호출 deadline을 기록. 실제 mock 21 rows에서 정책 범위 오류를 partial로 보존 | [측정 도구](./B0-MEASUREMENT-TOOLS.md), [원자료 원장](../../system/eval/b0-knowledge-system/measurement-tools-m2max-2026-10-03.json) |
| 2026-10-03 | B0-06 | 동일 저장 벡터의 CKV 정상 필터/상한과 독립 전체 스캔 exact 오라클. 읽기 전용 DB·게시 SHA·미봉인 WAL 거부. 실제 BGE-M3 pinned 소형 입력에서 exact 일치·DB/sidecar 무변경 | 같은 원자료 원장; F-01 희소 자격의 공식 통과 아님 |
| 2026-10-03 | REAUDIT-03 | `?`·`#` 파일명을 SQLite URI로 오해한 쓰기 DB 열기 수정. 실패 원자료와 실제 특수 파일명·읽기 전용 쓰기/생성 거부 시험 보존 | `internal/vector/store/sqlitevec`, sqlite-uri-before/후 시험 |
| 2026-10-03 | REAUDIT-04 / R-03 | CKV file_header/file_full의 EOF 개행을 추가 인용 줄로 센 원인 수정. 수정 전 실패 시험·실제 v2 source_missing 보존; 새 빌드의 동일 소스 snapshot/모델·새 dataset ID, 5응답·25본문 원문 SHA/줄/좌표/sha256-v2 확인 | 같은 원자료 원장. 전체 Go·관련 race·vet·경계·113 문서 검사 통과 |

| 2026-10-03 | B1-01 관계 경로 | 고정 의미 tuple·읽기 전용 DB·raw-first·기본 Stage 2 상위 K 보존·다의어·예산 폴백. 실제 mock/BGE-M3 각각 4상태×v1/v2 8요청, 고유 관계 1개·실모델 인용 2개 영향, 기본 폴백 본문/인용·v2 integrity 확인 | [실행 계약](./B1-RUNTIME-ADAPTERS.md), [원자료 원장](../../system/eval/b0-knowledge-system/b1-relations-runtime-m2max-2026-10-03.json). 전체 Go·관련 race/vet·경계·116 문서 통과; 공식 B1 미완료 |

| 2026-10-03 | B1-01 텍스트·결합 경로 | 동일 CKV에 검토된 개념 텍스트 추가 검색, 관계 조회 없는 text-only, 원문 상위 K 보존, 결합 총 boost 20% 상한. mock/BGE-M3 각 정상 4·누락/변조 6 경로의 v1/v2 20요청, 원출처 줄 SHA·v2 integrity·읽기 DB 무변경 확인 | [실행 계약](./B1-RUNTIME-ADAPTERS.md), [원자료 원장](../../system/eval/b0-knowledge-system/b1-text-runtime-m2max-2026-10-03.json). 추가 호출 시도는 실패에서도 보존; 공식 품질 지표 null |

| 2026-10-03 | B0-05 준비 / REAUDIT-05 | F-03의 생성 project_id가 원문 온톨로지와 불일치한 문제를 수정. 원문 20개·SHA·커밋 유지, 수정 전 실패/후 통과. F-05 두 프로젝트와 F-06에 proposed 팩/개념/스펙·native 코드 관계를 별도 커밋으로 연결, mock/BGE-M3 4개씩 검증 | [검토 자료](./B0-SEMANTIC-FIXTURE-REVIEW.md), [원자료](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03.json). 사람 검토·최종 변형 미완료 |
| 2026-10-03 | REAUDIT-06 / B1-02 | 실제 ontology+include_knowledge 요청의 knowledge_context_failed를 재현. 지식 Verify 전 diagnostic stamp, 지식 의미 DB 읽기 전용으로 수정. 수정 전 실제/Go 실패와 후 통과, mock/BGE-M3 각 14요청의 proposed 무승격·기본 인용/본문·v2 integrity·DB SHA 확인 | 같은 원자료. 전체 Go·관련 race·vet·Python 15 시험·경계·문서 검사; 공식 품질 지표 null |
| 2026-10-04 | B1-02 구현 / B0-06 원장 | 합성 팩 off/on 8경로를 같은 dataset/lock/config로 실행. mock/BGE-M3 각 48요청의 공개·범위 밖·미검토·만료·제한·충돌, 기본 후보/본문·출처·v2 integrity·DB/소스 SHA 확인. 초기 검증 스크립트의 request_sha256 필드명 오류를 수정하고 실패 원자료 보존 | [8경로 원장](../../system/eval/b0-knowledge-system/b1-pack-matrix-m2max-2026-10-04.json). 현재 raw/text K20·Stage2 cap30 계약 기록, 초안 K10의 공식 평가가 아님; rotation=false·품질 지표 null |

| 2026-10-04 | B0-06 실제 호출 계측 | 선택형 logical CKV/CKG/intent/health와 Ollama HTTP transport 시도를 캡처 ID로 연결. mock/BGE-M3 각 48요청, 기존 v1/v2 20요청, 실모델 비계측 12응답 동등성·실제 K/knowledge/Stage3·DB 무변경 확인. 리다이렉트·503·취소·scope 격리·in_flight·모델 신원 시험 및 전체 Go/race/vet 통과 | [호출 원장](../../system/eval/b0-knowledge-system/backend-calls-m2max-2026-10-04.json). 일부 구조 진단 병행·공식 시간/품질 null, startup pin/probe·SQL·모델 내부 작업은 카운터 밖; arm 회전/승인 K 남음 |

실행 로그의 임시 경로는 머신의 작업 위치다. 최종 요약·해시·실패 증거는 저장소 원자료 디렉터리로 옮겨 재현에 필요한 증거를 남긴다. `.claude/`와 `logs/`의 기존 사용자 파일은 작업 산출물에 포함하지 않는다.

이번 재검증에서 찾은 여섯 구조 미비는 B의 품질 실패를 기다릴 필요가 없어 즉시 수정했다. 이 수정을 C0 전체 완료로 세지 않는다. 큰 실모델 데이터셋/모델 파일/시험 개인 키는 저장소에 넣지 않았다.


## 8. 최신 재검증 결과에 따른 다음 행동

B0-06의 응답 수집·직접 검색 오라클·실제 호출 계측·공유 K·순차 회전·선택형 환경/입력 사본 원장은 구현과 개발 입력 연결을 검증했다. B1-01 네 어댑터와 B1-02 합성 팩 축도 실제 MCP 연결 검증을 완료했다. B0-05의 개발 4개 source-bound 자료는 사람 판정 대기다. 해당 도구의 성공을 공식 입력 승인이나 B0/B1 품질 합격으로 바꾸지 않는다. 공식 실행은 B0-01–03 및 의미 사실의 실제 사람 판정 후 동결 입력과 통제된 환경에서 진행한다. 다음 독립 우선순위는 Linux 환경 수집 재검증·나머지 native 고지 감사와 승인 입력 연결 준비다. 일반 실행 지시를 개별 사실·프로토콜 승인으로 바꾸지 않는다.

기존 소형/전체 코퍼스의 embedding completeness 기록은 유지한다. EOF 인용 오류를 발견했으므로 이 기록에서 “모든 v2 인용이 유효하다”는 결론은 얻을 수 없다. 공식 B0-07은 수정된 빌더 바이트로 새 dataset ID를 생성하고 청크 범위를 보관 원문에 대해 전수 감사해야 한다. 기존 데이터 원본을 덮어쓰거나 최신 데이터로 표시하지 않는다.

## 9. 재감사에서 추가된 즉시 수정 항목

| ID | 우선순위 | 관측·완료 조건 | 상태 |
|---|---|---|---|
| FIX-07 | P1 · B0-06 재감사 | real 48요청의 BM25 오류 96회는 합성 `/<convention>` 경로에서 만든 `<convention>` 후보가 FTS 문법으로 해석되기 때문. knowledge chunk를 코드 심볼 후보로 취급하지 않도록 경계를 정하고 실패 전/후 회귀·실모델 재현을 확인한다. 기존 원자료는 보존한다. | 수정 후 재검증 완료: 수정 전 실패/후 통과, real 8경로 48응답 바이트 동일·BM25 오류 96→0·보존 입력 무변경 |
| FIX-08 | P1 · C1-05 재감사 | host package가 로컬 Solidity grammar 두 LICENSE를 누락. 수집·소스 SHA 원장과 독립/설치 후 검증기에 고지 검사를 추가하고, 정상/이전 형식·원장 불일치 재서명 패키지를 검증한다. | 수정 후 재검증 완료: 이전 Go 검증기 수락 실패 보존, 수정 후 Python/Go 거부·이전 형식 통과; 실제 세 바이너리의 고지·시험 서명·세 독립 저장소 설치 통과. 전체 라이선스 적합성은 미판정 |
| FIX-09 | P1 · 패키지 재감사 | 같은 코드의 두 host preview가 `modules.txt`에 임시 staging 경로를 넣어 archive SHA가 달라짐. build info의 바이너리 표시 경로를 일정하게 만들고 동일 코드/바이너리로 두 번 패키징하여 전체 파일과 archive SHA를 대조한다. | 수정 후 재검증 완료: 실제 Go build info를 상대 바이너리명으로 출력; 두 번 빌드/패키징한 52개 파일 SHA·크기·모드와 전체 archive SHA 일치; 동일 시험 sidecar로 두 archive를 Python/Go 모두 검증 |
| FIX-10 | P1 · B0-06 Linux 환경 재검증 | ARM `/proc/cpuinfo`에 제품명 항목이 없고 최소 런타임에 `ps`가 없어 환경 preflight가 실패한다. 실제 식별자와 `/proc` 숫자 정보로 수집하고 cgroup 제한을 별도로 기록한다. | 수정 후 재검증 완료: 실제 수정 전 실패/후 환경 프로브, Linux arm64·amd64 에뮬레이션 각 3프로젝트·72 SDK 응답·설치/재시작/롤백; quota 변경·상위 그룹·경로 거부 회귀. 네이티브 amd64·Linux 실모델/운영 검증은 별도 |
| FIX-11 | P1 · 정적 입력 경계 재검증 | 기존 질문 ID가 경로 문자를 허용하고 export에서 소문자 파일명으로 바뀌어 경로 이탈/충돌 가능성이 있다. 안전한 ID와 대소문자 구분 없는 유일성을 요구한다. | 수정 후 재검증 완료: 원래 HEAD의 다섯 ID 실패를 모델/네트워크 없이 재현; 수정 후 26개 B0 회귀 통과; 정상 기존 export 13개 파일 바이트 동일 |
| FIX-12 | P1 · native 고지 확대 후 실제 패키지 검증 | Unicode/runtime/toolchain 고지를 포함한 110항목 정상 tar가 Python/Go의 기존 100항목 상한에 걸린다. 유한 상한을 맞추고 경계·변조 거부와 실제 설치를 확인한다. | 수정 후 재검증 완료: 수정 전 Python/Go 실패 보존; 상한 256개 통과·257개 거부, 고지 50개 preview의 두 서명 검증기·세 프로젝트 설치/재시작/업데이트/롤백 통과; 재서명 Unicode 변조 거부. C1 출시/적합성은 별도 |
| FIX-13 | P1 · 최신 호스트 스모크 재검증 | 고지 자산을 두 개로 가정하던 스모크가 새 네 자산의 정상 패키지를 거부한다. 필수 자산·소스/고지 전용 범위·실제 count·중복·SHA를 검사한다. | 수정 후 재검증 완료: 실제 수정 전 assertion 실패와 후 macOS 세 프로젝트 설치/재시작/업데이트/롤백 통과. 현재 두 Linux 시험 서명·설치/복구도 통과 |
| FIX-14 | P1 · Linux 패키지 metadata 재현성 | 같은 바이너리를 두 번 검사한 ldd 주소가 달라 package manifest의 native_dependencies가 변동한다. 라이브러리/해결 경로/오류/버전 정보를 보존하며 일시 주소를 안정화한다. | 수정 후 재검증 완료: 주소만 제거하며 첫 ldd 의존/정적 링크 진단 유지; 실제 Linux 두 대상의 각 4,624개 copy-input·두 공개 packager 빌드·전체 파일/모드/archive SHA 동일·동일 시험 서명/Python/Go 검증·세 프로젝트 설치/복구 통과. 도구 체인/대상별 제한 유지 |
| FIX-15 | P1 · translated native 고지 감사 | modernc.org/sqlite의 별도 SQLITE-LICENSE가 root prefix scan에서 빠진 실제 패키지를 확인했다. 고정 버전 required notice를 수집하고 없으면 missing으로 표시한다. | 수정 후 재검증 완료: 실제 누락/수정 전 회귀 보존, Python 16개 시험·세 플랫폼 새 package/두 검증기/설치/SQLite 고지 단독 변조 거부, ZIP/h1 기반 선택 modernc source 대조. 전체 legal/운영 적합성은 별도 |
| FIX-16 | P1 · 승인 경계 완료 재감사 | 프로토콜/동적 메타데이터만으로 실제 보류 결정 중 최종 export가 통과하며, v2 scope의 matching hash도 pending 결정에 결합됐다. 사람 승인 상태·검토자/시간·각 입력 SHA 결합을 요구한다. | 수정 후 재검증 완료: 두 수정 전 실패, 42개 관련 회귀·실제 CLI 대기/최종 출력 미생성·5개 원문 SHA 동일; 공식 B0/B1 판정은 대기 |
| FIX-17 | P1 · 평가 원자료 최종 조립 경계 | raw probe Go 소스 사본을 `.go`로 보관하여 system 경계 검사와 Go package inventory에 참여했다. 앞선 검사는 사본 조립 전에 실행됐으며 후속 실패 뒤 커밋까지 진행한 검증 순서 오류도 있었다. | 수정 후 재검증: 원문 SHA 보존 `.go.txt` 전환·최종 assembled artifact 경계/Go package inventory·원장 SHA/추적 검사. 제품 검색/공식 C0 품질 판정과 구분 |

호출 원장의 독립 감사는 v2 106응답의 출처/integrity와 116개 요청 scope를 확인했다. 비치명 내부 오류를 숨기지 않는다. real Neighbors 384회는 README/비심볼 header의 대응 노드 부재이며 best-effort 범위다. 첫 감사의 모든 내부 호출 성공 가정은 잘못되어 수정했다. 이 발견은 기존 여섯 수정이나 공식 C0 품질 판정과 별도로 관리한다.

2026-10-04 FIX-07 완료 증거: [수정 원장](../../system/eval/b0-knowledge-system/fix07-knowledge-keywords-m2max-2026-10-04.json). broad recall의 invariant/convention hit는 Hits·별도 knowledge pass에 그대로 유지하고 코드 키워드 후보에서만 제외했다. 같은 데이터셋·K·필터·팩의 8경로 실모델 48응답은 수정 전 SDK 응답과 완전히 같았다. BM25 오류 96→0, Neighbors의 비심볼 범위 실패 384회는 원장에 남겼다. v2 출처/integrity·정책 상태를 독립 재검증했다. 원래 오류 원자료를 수정하지 않았다. 이는 일곱 번째 구조 수정이며 공식 C0 전체 완료와 별개다.

2026-10-04 B0-06 후속: [공유 K 원장](../../system/eval/b0-knowledge-system/recall-k-m2max-2026-10-04.json). `retrieval.recall_k`가 raw/text에 동일 적용되고 생략 K20·knowledge K6·Stage2 cap30을 유지한다. K10은 구조 진단용이며 사람의 프로토콜 승인으로 간주하지 않는다. 전체 B0-06은 arm 순서 회전·공식 입력/환경 원장이 남아 진행 상태다.


## 10. 회전 실행 도구 재검증

2026-10-04 B0-06 후속 [회전 원장](../../system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04.json): 하나의 설정으로 8개 v2 경로를 생성하고 group별 시작 순서를 회전한다. mock/BGE-M3 각 288응답·36 group·56개 프로세스 시작, 별도 warm/cold, 실제 K20/K10와 knowledge K6, 전후 source/DB/HEAD/실행 비트·모델 digest를 확인했다. 독립 감사에서 모든 SDK 응답은 해당 모델의 기존 단일 수집 결과와 같았고 576개의 출처/integrity·정책 상태·measurement_id 연결이 유효했다. 원장에 비치명 Neighbors 실패도 보존했다.

실제 초기화 실패에서 새 cold 수집기의 typed-nil 인터페이스 패닉을 발견했다. 수정 전 실패 회귀와 running 보고서/48 warm 오류행/패닉 stderr를 보존하고 수정 후 실제 SDK의 96 실패행·partial을 검증했다. 50ms initialize deadline과 SDK의 프로세스별 2초 graceful 종료는 다른 시간이다. 초기 전체 20초 검증 가정과 arguments JSON 중복 decode 가정은 잘못되어 수정했으며 원자료는 바꾸지 않았다. 전체 Go·관련 race·vet·경계·문서 검사를 통과했다.

이 도구 검증은 공식 B1-03 완료가 아니다. 6개의 합성 정책 제어를 반복했으며 공식 프로토콜·입력 범위·동적/의미 사실 승인이 남아 있다. B0-06은 승인 입력·전체 환경 원장의 실제 실행 연결 때문에 진행 상태를 유지한다. 다음 우선순위는 이 상위 원장의 준비와 C1 출시 검토 자료의 독립 준비다.

2026-10-04 C1-05 독립 준비: [출시 검토 자료](./C1-REVIEW-PREPARATION.md)의 28개 연결 모듈 원문·누락 0·해시 검증과 현재 머신 snapshot을 수집했다. native 자산의 비-root 고지·운영 서명 정책·적합성 사람 판정은 남아 있다. 운영 키/출시 상태를 바꾸지 않았고 C1-05는 진행 상태다.

2026-10-04 FIX-08/C1-05 후속: [native 고지 패키지 원장](../../system/eval/b0-knowledge-system/native-license-packaging-m2max-2026-10-04/summary.json). macOS arm64 세 바이너리 연결 35개 모듈의 root 고지 41개와 로컬 Solidity grammar 2개 고지, 소스 8개 SHA를 수집·원문 대조했다. 시험 서명·정상/변조/이전 형식·설치 스모크를 재검증했다. 첫 28개 목록은 당시 `cks`만의 build info이며 새 세 바이너리 패키지 목록과 범위를 구분한다. FIX-08은 여덟 번째 구조 수정이고 공식 C0/C1 품질·운영 출시를 완료하지 않는다. 다음 독립 우선순위는 B0-06의 공식 입력/환경 원장 준비와 나머지 native 고지 범위 감사다.

2026-10-04 FIX-09: [패키지 재현성 원장](../../system/eval/b0-knowledge-system/package-reproducibility-m2max-2026-10-04/summary.json). 수정 전 두 archive의 고지·바이너리·manifest 등은 같고 Go build info의 임시 경로만 달랐다. 수정 후 같은 `19fad138`+dirty 상태의 두 순차 패키지는 52개 파일과 archive SHA가 같았다. 실제 linked build info를 유지하며 출력 머리글만 `ckg`/`ckv`/`cks`로 일정하게 했다. 같은 시험 서명 sidecar를 두 archive에 적용하여 Python/Go 검증을 모두 통과했다. 서로 다른 도구 체인/플랫폼의 빌드 재현성까지 판정하지 않는다. FIX-08 설치 원장에 등재된 합성 `.log` 21개는 일반 로그 제외 규칙의 누락을 확인하여 명시적으로 Git 보존한다. 이 두 수정 뒤에도 공식 B0/B1/C0/C1은 승인·실모델 품질·운영 판정이 남아 있다.

## 11. 환경·입력 원장 연결 재검증

2026-10-04 [환경 원장](../../system/eval/b0-knowledge-system/environment-ledger-m2max-2026-10-04/summary.json): 선택형 환경·strict model 전후 프로브·세 입력 사본을 실제 회전 실행기에 연결했다. mock/BGE-M3 각 24응답·16 startup scope의 기존 SDK 응답 동등성, v2 출처/integrity·정책·실제 K/measurement ID·DB/입력/HEAD/실행 비트·사본 SHA를 확인했다. 실제 BGE-M3의 digest·1024차원·8192 context/batch·Ollama 0.35.1·residency와 M2 Max 12 CPU/64 GiB·OS/load/swap/process 압력을 기록했다. 새로운 모델 프로브는 질의 시간/카운터 밖이다.

초기 mock의 기존 checksum 신원 표현을 새 프로브가 수용하지 못한 0행 실패를 보존하고 정확한 checksum 호환을 보완했다. 실제 다른 모델 설정도 조회 시작 없이 0행 partial·후속 환경 snapshot을 남긴다. 실패 주입·사본 변조·모델 변경·취소·redirect·legacy 신원 시험, 전체 Go·관련 race/vet·경계 검사를 통과했다. 비심볼 Neighbors 실패 real 192/mock 24회는 숨기지 않았다. 세 사본 중 프로토콜/동적 입력은 여전히 draft다. 제어 질의 1개의 반복이므로 공식 품질·정적/dynamic 최종 실행·사람 판정은 미완료다.

## 12. Linux 최소 런타임 환경 재검증

2026-10-04 FIX-10: [Linux 원장](../../system/eval/b0-knowledge-system/linux-environment-m2max-2026-10-04/summary.json). 수정 전 ARM builder에서 CPU 신원 미수집, Go·ps 없는 최소 런타임에서 CPU 신원과 프로세스 압력 미수집을 실제 실패로 확인했다. ARM 식별 필드를 추정 없이 기록하고 `/proc`의 숫자만 읽도록 보완했다. cgroup v2 self·노출된 상위 그룹과 통상 v1/v2 root 값을 별도로 기록하며 제한의 전후 변경을 거부한다. Linux CPU 수·RAM은 커널에 보이는 전체 값이고 프로세스 CPU는 lifetime 평균/PID namespace 범위다. 물리 머신 용량·자원 독점으로 해석하지 않는다.

동결된 `d6477d65` 기반 작업 소스를 Go 1.25.13으로 빌드했다. 원본 Git 메타데이터 없이 빌드한 `devel` 진단 바이너리이며 소스 목록·바이너리 SHA·이미지 ID를 연결했다. Linux arm64와 amd64 에뮬레이션 각각 3개 독립 프로젝트(empty Go/TypeScript/미지원 Python), 설치·조회·MCP 재시작·새 버전·롤백을 검증했다. 프로젝트마다 8경로×retrieval/warm/cold 각 1회, 총 144 SDK 응답·96 startup scope의 환경 전후·1 CPU/512 MiB quota·입력/DB/HEAD/실행 비트·lock 사본·measurement 연결·보관 원문의 줄/본문/파일 SHA·v2 integrity를 확인했다. 의미 저장소가 없는 폴백 진단이고 품질은 null이다. 비심볼 Neighbors 실패는 플랫폼마다 72회 보존했다.

초기 합성 요청에 날짜/subsystem을 누락하여 24행 중 12개 지식 요청이 오류였다. 이 요청 오류와 read-only source에 빌드 출력을 쓰려던 명령 오류, 원장 생성기의 이미지명 오타도 보존했다. 제품 수집 실패와 분리한다. 수정된 입력으로 두 플랫폼 모두 재검증했고 전체 Go·evalcli race·vet·경계·136개 문서 검사를 통과했다. 검증 원자료의 파일 권한은 실행 시점에 확인했으며 Git 보관 권한으로 비공개 출력을 보증하지 않는다.

B0-06 도구의 Linux 최소 런타임 연결은 검증됐지만 공식 게이트는 계속 미완료다. 다음 P1은 승인된 정적 gold와 동결 후보 프로토콜의 개발/최종 경계를 검사하는 공식 입력 preflight 준비다. B0-01 범위, B0-02/03 프로토콜·동적 입력, B0-05 의미 사실의 사람 결정 이후 B0-07 공식 빌드를 진행한다. Linux BGE-M3·네이티브 amd64·운영 패키지/키/라이선스와 최종 품질은 C1의 남은 범위다.

## 13. 공식 입력 경계와 개발 정적 입력 준비

2026-10-04 B0-06/FIX-11: [정적 입력 사전 검증](./B0-STATIC-INPUT-PREFLIGHT.md)과 [원장](../../system/eval/b0-knowledge-system/static-input-preflight-m2max-2026-10-04/summary.json)을 추가했다. 질문·프로토콜·동적 원문·사람 결정의 해시/ID/model pin을 결합하고 정적 개발 4/최종 8 및 동적 12개 분할·소스 해시·네 모드/팩 축·K·기존 임계치·반복/불확실성 선언을 검사한다. 실제 후보는 정적 승인 12/12·동적 0/12이며 프로토콜·동적 승인 대기로 exit 2다. 이 판정은 입력 정의에 한정되고 전체 B0 readiness는 아니다.

개발 4개 v1 파일을 진단용으로 준비했다. 최종 프롬프트·후보 답·v2 matrix 입력을 발행하지 않았다. 최종 export와 `--require-ready` 발행은 출력 디렉터리 생성 전에 거부했고 기존 원문/검토 기록의 전후 SHA가 같다. 군별 최종 독립 질문은 2개씩으로 반복 수와 별개이며 출시 추론은 inconclusive다. 기존 질문 ID 검증에서 경로 문자·대소문자 충돌 다섯 실패를 재현 후 수정했다. 정상 이전 exporter의 전체 13개 파일 바이트는 동일했다. Python B0 26개 회귀와 문서 검사를 통과했다. 독립 코퍼스의 실제 HEAD/tree·깨끗한 checkout·독립 common dir·alternates 없음·unreachable commit 없음도 다시 확인했다.

B0-06의 공식 실행 연결은 계속 남아 있다. v2 팩 축은 날짜/subsystem이 필요하지만 현재 프로토콜은 정적 질문별 지식 범위를 선언하지 않는다. 제안 K10도 실제 설정과 계측에 결합해야 한다. 원래 초안과 정적 gold를 임의로 수정하거나 이 준비를 사람 승인으로 취급하지 않았다. 다음 수행 가능한 독립 작업은 C1-05(P3)의 남은 native 고지 범위·운영/지원 검토 자료 준비다. B0-01/02/03/05의 실제 사람 결정 이후 새 strict 데이터셋과 정적·동적/v2 입력을 함께 동결하여 B0-07부터 공식 실행한다.


## 14. Native 고지 출처와 패키지 상한 재검증

2026-10-04 C1-05/FIX-12: [출시 검토 자료](./C1-REVIEW-PREPARATION.md)와 [원장](../../system/eval/b0-knowledge-system/native-notice-audit-m2max-2026-10-04/summary.json)을 보완했다. 실제 Darwin arm64 Go-selected native package 9개와 toolchain vendor package 15개를 기록했다. Tree-sitter runtime C/header 44개·두 Solidity 버전의 LICENSE/parser/header 6개는 고정 upstream 원문과 바이트가 같다. Go 모듈 checksum 검증도 통과했다. 모든 포함 header·native 코드의 라이선스 적합성까지 확정한 감사는 아니다.

Unicode 내부 고지, Tree-sitter native runtime 원문 고지, Go toolchain/vendor 원문 5개를 추가하여 실제 세 바이너리 preview는 연결 모듈 35개·고지 50개·missing 0이다. 이 수집 범위의 missing 값과 전체 적합성을 구분한다. source/version/고지 변동은 검토 recipe 검사로 거부한다. 고지 파일·원자료·소스 사본·SHA를 보존했고 시험 개인 키/압축파일/바이너리/DB는 제외했다.

110항목의 정상 패키지가 두 검증기의 100항목 제한에 걸린 실제 실패를 발견했다. 기존 크기·경로·중복·타입 제한을 유지하며 공통 상한을 256개로 수정했다. 11개 Python 회귀에는 실제 Go CLI의 110/256 통과·257 거부·기존 inventory 호환·변조 거부가 포함된다. 해당 Go 패키지 compile/vet와 실제 macOS arm64 시험 서명 preview의 두 검증기·세 프로젝트 설치/재시작/업데이트/롤백·재서명된 Unicode 고지 변조 거부가 통과했다. FIX-12는 구조 수정 완료이고 C1-05는 여전히 진행이다.

다음 독립 우선순위는 C1-05의 다른 native 자산(SQLite/vec·JS/TS 포함)의 실제 소스/고지 범위 및 최신 recipe의 Linux 패키지 재검증이다. 이후 운영 서명·지원·복구 검토 자료를 이어서 준비한다. B0-01/02/03/05 사람 결정, 정적 v2 날짜/subsystem·설정 K 결합, 승인 입력의 새 strict 빌드/공식 B0/B1/C0/C1 및 운영 판정은 미완료다. 공식 품질 지표는 계속 null이다.


## 15. 추가 native 출처·세 플랫폼 패키지 재검증

2026-10-04 C1-05/FIX-13: [후속 원장](../../system/eval/b0-knowledge-system/native-source-platform-followup-m2max-2026-10-04/summary.json)의 네 모듈 41개 원문은 고정 origin commit과 같다. root 고지와 선택 CGO·C/header 범위이며 모든 전이 native source의 적합성까지 확인한 것은 아니다. Linux 시스템 SQLite 헤더·설치 패키지 고지와 본문의 attribution 관측도 기록했다.

정상 호스트 패키지의 스모크만 오래된 두 자산 가정으로 실패했다. 실제 수정 후 macOS arm64의 세 프로젝트 설치·조회·재시작·업데이트·롤백이 통과했다. Linux arm64와 amd64 에뮬레이션은 현재 source recipe의 시험 서명 패키지로 두 서명 검증기·Go 없는 Debian 런타임의 같은 세 프로젝트 설치/복구를 통과했다. macOS는 Go 1.26.8·35모듈/50고지, Linux는 Go 1.25.13·33모듈/48고지다. 대상과 도구 체인 차이를 함께 기록했고 원문 SHA도 대조했다. Linux Git commit은 합성 source fixture로 원래 release commit이 아니다. 4,177개 동결 copy-input 해시를 보존했으나 늦은 live builder 대조는 종료된 컨테이너 때문에 수행되지 않았고 실패 원문을 별도로 보존했다.

추가 독립 검사에서 같은 실제 Linux 바이너리의 두 native dependency 결과가 ASLR 주소 때문에 달랐다. FIX-14를 실제 실패로 등록했다. 다음 우선순위는 라이브러리/해결 경로/오류를 유지한 metadata 안정화와 Linux 패키지 재현성 검증이다. 이후 운영 키·신뢰 루트·지원/복구 자료를 준비한다. C1-04/05 운영 판정과 B0/B1/C0/C1 공식 품질은 여전히 미완료이며 사람의 입력·프로토콜·의미 사실·적합성 결정이 필요하다.


## 16. Linux 패키지 재현성과 운영 결정 자료

2026-10-04 FIX-14: [원장](../../system/eval/b0-knowledge-system/fix14-linux-package-reproducibility-m2max-2026-10-04/summary.json)의 실제 동일 바이너리 반복 검사 실패를 수정했다. Linux ldd의 per-process 주소 suffix만 제거한다. Linux에는 heading이 없으므로 기존의 첫 줄 삭제도 제거하여 virtual dependency·정적 링크·누락 진단을 유지했다. macOS otool heading 제거와 라이브러리 버전 정보는 그대로다. 수정 전 두 회귀 실패, 수정 후 Python 15개 시험, 실제 기존 macOS 세 바이너리의 dependency metadata 동등성을 보존했다.

Linux arm64와 amd64 에뮬레이션 각각에서 실제 빌더의 copy-input 4,624개 bytes/실행 비트가 동결 입력과 같았다. 같은 합성 source commit으로 공개 package-host를 두 번 실행해 매번 세 바이너리를 빌드했고, 106개 항목의 경로·파일 SHA·크기·모드 및 archive 전체 SHA가 같았다. 대상별 하나의 시험 서명으로 두 archive를 Python/Go에서 모두 검증했다. 추출한 바이너리로 Go 없는 Debian 런타임의 세 프로젝트 설치·MCP 재시작·업데이트·rollback도 통과했다. 실제 세 Linux 바이너리의 dependency 검사를 두 번씩 반복해 manifest와 같은지도 확인했다. 원래 release commit의 운영 빌드, 서로 다른 대상/도구 체인의 동일 SHA, native amd64/실모델 품질을 입증한 것은 아니다.

[C1-OPERATIONS-REVIEW](./C1-OPERATIONS-REVIEW.md)에 운영 역할·배포 scope·public key DER fingerprint/독립 신뢰 경로·개인 키 관리·교체/폐기·적합성·지원·복구의 OP-01–08 결정을 정리했다. 현재 signer/verifier는 test-signed-preview만 지원하며 기간/폐기 조회/운영 scope는 없다. 시험 키를 운영 키로 승격하지 않았고 운영 출시·사람 역할/키/정책은 미정이다. C1-05는 준비 진행 상태를 유지한다.

B0-01 범위, B0-02/03 프로토콜/동적 입력, B0-05 의미 사실의 실제 사람 결정을 다시 요청했다. 기존 정적 gold/BGE-M3 승인 범위를 확장하지 않는다. 정적 v2의 날짜/subsystem과 설정 K 결합도 남아 있다. 다음 가능한 독립 작업은 Linux 실제 Ollama/BGE-M3 진단의 환경·모델 가용성 확인과 남은 native/translated source 검토 자료다. 공식 B0/B1/C0/C1·운영 키/적합성/출시 판정은 계속 미완료다.


## 17. Linux ARM64 실제 CPU 모델과 출처 재검증

2026-10-04 C1-04/B0-06 독립 준비: [원장](../../system/eval/b0-knowledge-system/linux-real-model-m2max-2026-10-04/summary.json)의 실제 Ollama 0.35.1 공식 arm64 image를 고정 digest로 실행했다. 승인 BGE-M3 manifest와 참조 blob 3개를 전후 해시로 확인했고 동일했다. 모델만 노출한 read-only mount·network none·공개 포트 없음, 서버 2 CPU/4 GiB와 Go 없는 Debian caller 1 CPU/512 MiB를 기록했다. API 한·영 2입력은 1024차원·유한값·정규화를 통과했으며 residency는 승인 digest·size_vram=0이다. VM 커널의 CPU/RAM·caller의 PID 압력을 물리 자원 독점이나 별도 모델 PID의 압력으로 해석하지 않는다.

FIX-14 시험 서명 Linux arm64 패키지를 검증한 뒤 empty Go/TypeScript/미지원 Python 세 인공 프로젝트의 strict 실모델 빌드·조회·v1 MCP 재시작·두 버전·rollback을 확인했다. 프로젝트마다 8경로×retrieval/warm/cold 1회로 총 72개 v2 SDK 응답·96개 인용·48 startup scope를 독립 대조했다. 실제 K20/HTTP/measurement ID·arm 회전·모델/환경 전후·입력/설정/DB/HEAD/실행 비트·사본 SHA와 실행 시 파일 권한, 보관 원문/줄/본문 SHA·v2 integrity, 이전 indexed_commit과 현재 HEAD를 확인했다. 복구된 데이터셋의 실제 저장 벡터 5개는 청크 집합·1024차원·유한값·정규화와 같다. 내부 best-effort Neighbors 실패 72회도 보존했다. 보관 원자료 자체의 SDK/source/integrity/backend 연결 재생도 통과했다.

초기 추출 helper는 Debian Python의 filter 인자 미지원으로 프로젝트 생성 전에 실패했다. 사전 서명/구조 검증된 owned archive에만 시스템 tar를 사용하는 진단 adapter로 재실행했다. TypeScript v1 initialize는 기존 probe의 20초 client deadline에서 실패했고, 제품 설정/3초 backend timeout을 유지한 별도 90초 진단 client에서 통과했다. 이를 기본 20초 지원의 통과로 세지 않는다. SQL 심볼의 parser-node text를 전체 줄과 같다고 가정한 verifier 실패도 보존했다. text SHA와 선언 줄 내 바이트 포함을 검사하고 열 단위 정확성은 보증하지 않는다. 반환 인용/본문은 전체 보관 줄 SHA를 엄격히 검사했다. 이번 제품 코드 수정은 없다.

cold는 MCP process 시작 조건이고 resident model은 유지된다. warmup 0·군별 1회인 소형 unavailable 폴백 진단이며 공식 프로토콜/팩 사실/관계 품질을 대신하지 않는다. Python 분석기·Go 도구 체인 지원, native amd64·대규모 비용, 운영 키/최종 패키지/복구·출시를 완료하지 않았다. 품질은 계속 null이다. 전용 모델 서버만 종료/삭제했고 원본 모델 bytes가 같다. [운영 검토표](./C1-OPERATIONS-REVIEW.md)의 Linux ARM 범위를 갱신했다.

다음 우선순위는 B0-01/02/03/05 사람 결정이 도착하면 정적 v2 날짜/subsystem·실제 K와 함께 입력을 동결하여 B0-07 공식 strict 빌드로 진행하는 것이다. 결정 대기 중 수행 가능한 독립 작업은 C1-05의 translated/native/transitive source 범위 및 C1-06 실패 복구 자료 준비다. 공식 B0/B1/C0/C1과 OP-01–08은 계속 미완료다.


## 18. Translated SQLite 고지 누락과 전이 소스 재검증

2026-10-04 C1-05/FIX-15: [원장](../../system/eval/b0-knowledge-system/translated-source-notice-m2max-2026-10-04/summary.json)의 기존 실제 패키지에는 modernc.org/sqlite v1.54.0 wrapper LICENSE만 있었다. 별도 SQLITE-LICENSE를 root prefix scan이 놓쳤다. 수정 전 회귀 실패/actual inventory와 source SHA를 보존하고 고정 버전의 required notice에 추가했다. 파일이 없으면 wrapper가 있어도 missing_source_license로 표시한다. 원문 수집 수정이며 적합성/출시 승인이 아니다.

새 공개 packager의 macOS arm64는 35모듈/51고지/111항목, Linux arm64·amd64 에뮬레이션은 33/49/107이다. 두 Linux 빌더에서 동결 추적 입력 5,230개 bytes/실행 비트 대조 후 동일 합성 commit 854464d9383eb8864edab1d1ed66f1e97adafd5d로 빌드했다. 세 대상 모두 Python/설치 후 Go 시험 서명 검증·세 독립 mock 프로젝트 설치/재시작/업데이트/rollback, 추가 SQLite 고지만 바꾼 재서명 archive의 두 검증기 거부를 통과했다. notice 원문/SHA와 libc의 기존 third-party 고지도 확인했다. Python 16개 회귀에 현재 Go 검증기 연동을 포함했다. Go 코드 변경은 없으며 전체 Go suite를 반복하지 않았다.

실제 도구 체인의 modernc dependency-selected 소스는 Darwin 30 package/225 file, Linux arm64 8/197, amd64 에뮬레이션 8/201이다. 네 모듈의 source·root 고지 9개는 고정 module ZIP과 바이트가 같고, ZIP h1은 go.sum/실제 build info에 연결된다. host/두 builder의 go mod verify도 통과했다. Origin metadata와 직접 Git blob 검증을 구분한다. GitLab 웹 source 읽기 실패와 초기 CLI 디렉터리명 오류도 보존했다. original C/generator·입력 system header 재생이나 전체 native/legal 적합성은 증명하지 않는다.

FIX-15는 고정 버전의 누락 수집 수정 범위에서 완료다. 최신 패키지 설치는 mock이며 본 턴 실제 모델을 실행하지 않았고, 앞선 실제 모델 증거는 FIX-14 패키지 범위로 유지한다. C1-05의 원래 C/header/native 전이 범위·사람 적합성/운영 결정, C1-04/06/07 및 공식 B0/B1/C0/C1은 계속 미완료다. 다음 독립 작업은 C1-06 실패 복구/원본 보존 검증이며 B0 사람이 결정하면 정적 v2 범위/K 동결과 공식 strict 빌드를 우선한다.


## 19. 실패 복구·구 데이터 원본 보존 재검증

2026-10-04 C1-06 독립 준비: [복구 문서](./C1-RECOVERY-VALIDATION.md)와 [원장](../../system/eval/b0-knowledge-system/recovery-m2max-2026-10-04/summary.json)을 추가했다. 최신 FIX-15 Darwin arm64 시험 preview의 추출 바이너리로 소형 TypeScript/mock 후보 테스트 실패·벡터 빌드 실패·손상 대상 거부·실행 중 MCP pin·업데이트/정상 rollback·손상 활성 버전의 직접 rollback 거부·새 루트 백업 복원·원본 소스 없이 보관 인용 재생을 실행했다. 기존 payload 12파일의 크기/SHA/실행 비트와 백업/복원 버전이 같다. 부분 후보와 손상 루트는 보존했다.

구 커밋 1ded9b3e47bc2e09062dba329c2f918423746fa4의 실제 v1 바이너리를 현재 Go 도구 체인으로 빌드했다. 합성 localhost HTTP 벡터로 구 데이터를 만든 뒤, 첫 신규 소비자 전 7개 payload SHA를 기록했다. 새 소비자는 legacy_unpinned/reindex_required를 유지하고 별도 새 버전으로 재색인했다. 구버전 rollback 후 구 v1 commit·인용 파일·serviceable과 원본/백업 SHA가 같다. 새 재색인은 mock이며 원본 schema를 자동으로 pinned로 바꾸지 않았다.

정리한 두 저장소 스크립트를 새 디렉터리에서 실제 재실행했다. 현재 빌드와 새 재색인에 CKV_REQUIRE_COMPLETE_EMBEDDINGS=1을 명시했다. [독립 감사](../../system/eval/b0-knowledge-system/recovery-m2max-2026-10-04/independent-audit.json)는 보존된 SDK 11응답/11인용·v2 integrity 6개·source/본문/신원·typed 실패·pin/legacy·전후 payload 연결을 확인했다. 빈 graph/vector WAL과 SHM의 수명 변화는 전후 파일 차이·별도 snapshot 원장으로 기록하고 payload와 구분했다. 비어 있지 않은 WAL은 검증기가 거부한다. 실행 중 DB의 단순 파일 복사를 운영 백업으로 보증하지 않는다.

초기 helper의 기본 반복 시간 제한·단일 JSON 출력/인용 v1 필드 가정·SQLite sidecar/읽기 전용 blob 권한·legacy 출력 경로/첫 baseline 시점 보완 및 완전 모드 미명시 실행을 보존했다. 제품 Go 코드는 변경하지 않았다. 이번 구조 진단을 최종 운영 복구/품질/지원 승인으로 합산하지 않고 C1-06을 진행으로 변경했다. OP-08과 최종 후보·실제 운영 백업/전환·대규모/플랫폼 범위는 남아 있다.

최상위 P0는 B0-01/02/03/05의 실제 사람 결정이다. 결정 후 정적 v2 날짜/subsystem과 실제 K를 동결하고 B0-07 공식 strict 빌드 → B0-08/09 → B1 paired 비교/사람 판정 → C0 관측 실패 수정 → C1 최종 재평가 순으로 진행한다. OP-01–08의 역할/키/신뢰/적합성/지원/복구와 native amd64 환경을 임의로 대신하지 않는다. 공식 품질은 null이며 작업리스트 전체는 미완료다.


## 20. 재개 후 정적 v2 날짜·subsystem·K 제안 준비

2026-10-04 사용자 재개 지시에 따라 [STV2-01 검토 자료](./B0-STATIC-V2-SCOPE-REVIEW.md)와 [원장](../../system/eval/b0-knowledge-system/static-v2-scope-preparation-m2max-2026-10-04/summary.json)을 준비했다. 실제 고정 corpus committer time은 2026-10-01T20:53:13+09:00이다. 그 시간대의 달력 날짜 2026-10-01과 질문집 corpus_project=knowledge-system을 정적 12개 v2의 조회 범위로 제안하고 runtime retrieval.recall_k=10 patch를 명시했다. 정책 효력/승인 사실을 날짜에서 추론하지 않는다.

실제 고정 커밋의 source-root .cks tree 목록은 비어 있다. 외부 reviewed pack/semantic inventory·source-bound 출처/잠금은 별도 선행 조건이며, pack-on unavailable 폴백을 팩 품질 효과로 판정하지 않는다. 새 파일은 ID/4–8 분할/날짜/subsystem만 포함하고 최종 프롬프트·후보 답을 발행하지 않는다. matrix가 include_knowledge를 소유하는 실제 계약도 반영했다.

새 checker의 최초 회귀에서 ISO week date와 float K10.0을 받아들이는 두 허점을 확인했다. YYYY-MM-DD와 정수 K를 요구한 수정 후 새 회귀 6개·기존 정적 입력 회귀 10개를 통과했다. 실제 CLI는 승인 대기 exit 2, 기존 보고서/gold 출력 거부 exit 1, 잘못된 날짜의 출력 미생성, 입력 5개 전후 SHA 동일을 확인했다. human-review의 scope SHA 연결 없이 approved 메타데이터만 입력해도 pending이며 official_execution_ready/v2_matrix_ready=false·metrics=null을 유지한다.

B0-01/02/03/05와 STV2-01의 결정을 사용자에게 요청했다. 기존 승인 기록을 바꾸거나 이 일반 재개 지시를 개별 승인으로 기록하지 않았다. 사람이 결정하면 protocol/scope의 새 해시를 함께 동결하고 실제 K/config·pack/semantic inventory·새 strict 빌드에 연결한다. 이번 준비는 B0-06 전체 완료나 공식 B0/B1 품질 합격이 아니다.


## 21. 승인 경계 FIX-16과 B1 비교 보고 준비

2026-10-04 [비교 보고 자료](./B1-PAIRED-REPORT-PREPARATION.md)와 [원장](../../system/eval/b0-knowledge-system/paired-report-preparation-m2max-2026-10-04/summary.json)을 추가했다. 완료된 정적 입력 준비 검사를 다시 감사하여, 실제 보류 결정이 있어도 approved 메타데이터만으로 최종 export가 통과하는 실패와 static v2 matching hash가 pending 사람 결정에 결합되는 실패를 재현했다. 사람의 실제 approved 상태·검토자/시간·검토한 protocol/fixture/scope 바이트 SHA를 모두 요구하도록 수정했다. FIX-16은 이 구조 수정 범위에서 완료이고 B0-06/공식 품질 전체 완료는 아니다.

B1-04/06/08은 도구 준비 진행으로 변경했다. 보관 matrix 원문/설정 SHA·회전·phase·질문/인자·measurement ID, source/citation/body/v2 integrity를 결합한다. Go 인용 dedup·commit/겹침·top10·기대 인용별 평균 역순위 규칙을 유지한다. 반복 중앙값과 같은 질문의 범위 변형 unit을 분리하고 독립 unit paired bootstrap, warm 성공 조건부 nearest-rank p50/p95·별도 process cold·분모·backend/HTTP/K/크기를 기록한다. 오류·누락은 검색 분모에서 지우지 않고 missing 계측은 null이다. 구성 인용 점수는 raw CKV Recall@10이나 사람 정책/답변 판정이 아니다.

기존 BGE-M3/mock 두 matrix(각 288)와 환경 원장(각 24)의 총 624 SDK 응답을 실제 재생했다. 각 묶음의 독립 unit은 1이고 군별 추론은 inconclusive다. 실모델 K10/6·mock K20/6과 각 바이너리/환경/반복은 별도로 유지했다. SDK/evidence 실패·누락 계측은 0이지만 내부 best-effort nonreturned 2,808시도는 원문 연결로 남겼다. 118개 보관 원자료/설정/로그/소스/gold 전후 SHA와 실제 5개 사람이 검토한 입력의 기존 HEAD SHA가 같다. 실제 모델·공식 최종 질문을 새로 실행하지 않았다.

관련 Python 42개(8+11+7+16)를 통과했고 두 수정 전 회귀 실패 및 localhost sandbox 오류/후 재실행을 보존했다. 실제 입력 준비 final/ready 및 정적 개발/최종 비교 CLI는 exit 2·입력/평가 출력 미생성이다. synthetic-test-only 승인 경로는 메모리/임시 테스트 범위이며 실제 승인 기록을 만들지 않았다. 현재 quality_metrics·사람 verdict/cost는 null이다.

다음 최상위 P0는 B0-01/02/03/05와 STV2-01의 실제 사람 결정이다. 이후 source coverage·팩/semantic·조회 scope/K·protocol을 함께 동결하고 새 strict B0-07 → 공식 B0/B1 → 사람 판단과 실제 C0 수정 → 최종 C1 순으로 수행한다. 정적 최종/동적 전체 오라클·raw CKV 후보 순위·검토 비용·운영 OP-01–08/native amd64·출시 판정은 남아 있고 작업리스트 전체는 미완료다.


## 22. 진행 현황판과 B1-05 안전 제어 감사 준비

2026-10-04 사용자 지시에 따라 이후 마지막 보고에 현재 단계·다음 작업·남은 전체 작업을 항상 포함하는 규칙을 본문과 인계 문서에 기록했다. [진행 현황판](./EXECUTION-STATUS.md)은 이 작업리스트의 30개 행을 읽어 완료/진행/대기·다음 우선순위·28개 미완료 항목과 추가 사람 결정을 표시한다. 구현 완료 2개와 공식 품질 게이트 0/4를 구분한다. 생성기는 전체 ID 집합 누락/중복·모르는 상태를 거부하고 `--check`로 문서 동기화를 확인한다.

[B1-05 검토 자료](./B1-SAFETY-AUDIT-PREPARATION.md)와 [원장](../../system/eval/b0-knowledge-system/safety-audit-preparation-m2max-2026-10-04/summary.json)을 추가했다. 공개 Go EvidencePackV2.Verify로 보관 SDK 624응답을 재생하고, source bytes·내부 의미 인용 2,124개·선언된 팩 lock·6정책 상태·unknown/conflict·합성 payload 비노출을 검사했다. off/on 312쌍의 기본 인용/본문 보존과 비어 있는 의미 tuple fixture의 같은 축 내 후보 집합 동등성을 확인했다. 선언된 제어 범위의 위반은 0이다. fixture-reviewer 상태를 운영 사실 승인으로 바꾸지 않았고 실제 모델/공식 최종 사례를 새로 실행하지 않았다.

Go 계약 감사 helper의 관련 시험·vet와 Python 제어 감사 회귀를 실행했다. 바깥 integrity가 맞아도 내부 foreign 인용, 금지 payload, proposed/expired/restricted 승격, 충돌의 required_behavior 승격, pack-off overlay, 꾸며낸 implementation, dropped base body, 다른 measurement ID/lock, 미지정 uncertainty를 거부한다. 실제 공개 Go 검증기의 변조 거부와 CLI 기존 출력/입력 변조 거부도 원장에 남긴다. 첫 helper 시험 fixture의 필수 integrity 알고리즘 누락은 보완했고 제품 오류로 세지 않는다.

B1-05는 도구 준비 진행이며 공식 안전·권한·비밀 게이트 완료가 아니다. 정책 payload marker 검사는 작성된 합성 문자열 범위이며 일반 비밀 탐지/추론 누출을 보증하지 않는다. 공식 F-04 과거 상태, F-05 별도 프로젝트, F-06 의미/정책·답변의 사람 판정은 남아 있다.

다음 독립 작업은 raw CKV 후보 순위와 F-01 exact/budget 판정의 보고 연결 준비(B0-06/B1-04)다. B0-01/02/03/05·STV2-01의 실제 결정이 도착하면 source/pack/semantic/scope/K 동결과 B0-07 새 strict 빌드를 최우선으로 진행한다. 전체 미완료 항목과 OP-01–08·native/legal/운영 범위는 현황판에서 함께 확인한다.


## 23. Raw CKV 순위와 F-01 exact/상한 보고 재검증

2026-10-05 [보고 자료](./B0-RAW-VECTOR-REPORT-PREPARATION.md)와 [원장](../../system/eval/b0-knowledge-system/raw-vector-report-preparation-m2max-2026-10-05/summary.json)을 추가했다. 직접 probe의 실제 K·원시 chunk 순위/거리·좌표/model/manifest/DB 전후 seal·source SHA를 결합한다. 원시 후보 슬롯과 구성 인용 dedup/top10 점수는 별도이며 null eligible·오류·부분 검색·planned 분모를 보존한다. source text가 parser-node 범위임을 유지하고 전체 줄 SHA와 구분한다. DB/model/최종 질문을 새로 실행하지 않았다.

Go 독립 exact scan에 top K로 자르기 전 전체 eligible 목록을 보존했다. reader가 독립 목록의 개수/파일과 작성된 diagnostic controls, 완전한 무필터 희소 조건, 정상 ID/파일/줄/거리 일치, 실제 상한 초과·incomplete/candidate_limit을 분리한다. 목록 누락·다른 K·부분 무필터는 자격 미충족이며 metadata-only로 공식 verdict를 만들지 않는다.

실제 역사적 BGE-M3 Alpha 프로브와 sidecar/보관 source 3개는 원래 commit Git blob과 같다. K1·eligible1·normal exact 일치는 유지되지만 희소 조건과 full eligible 목록이 부족하여 fixture_not_qualified다. @10/F-01 합격으로 합산하지 않는다. 최초 reader CLI의 Path.open opener 오류를 기록하고 수정했으며, Python 신규 15개/paired 기존 16개·Go probe 6개/race/vet·문서/경계/현황 동기화 검사를 통과했다. 과거 원자료와 실제 사람 입력을 바꾸지 않았다.

B0-06/B1-04는 도구 준비 진행이고 완료 수는 그대로 2/30, 공식 품질 판정 0/4다. 다음 독립 작업은 F-02 장문 꼬리·split/parent 재조립 오라클의 보고 연결이다. B0-01/02/03/05·STV2-01의 실제 결정 후 source/pack/semantic/scope/K 동결 → 새 strict B0-07 → 공식 B0/B1 → 사람/C0 → 최종 C1 순서를 우선한다. OP-01–08·native amd64·전체 native/legal/복구/출시 범위와 전체 28개 미완료 작업을 마지막 보고에 함께 표시한다.


## 24. F-02 장문 꼬리·부모 재조립 개발 진단

2026-10-05 [F-02 보고 자료](./B0-F02-DOCUMENT-REPORT-PREPARATION.md)와 [원장](../../system/eval/b0-knowledge-system/f02-document-preparation-m2max-2026-10-05/summary.json)을 추가했다. 새 최신 바이너리·strict BGE-M3로 미승인 DEV 입력을 진단했다. 203줄/20,290바이트는 5 child의 순서/parent ID·줄/heading/hash와 함께 손실 없이 재조립되고 actual build truncated0이다. 두 raw query는 K10의 tail rank1이며 전체 저장 청크5개를 반환하는 소형 사례다. 공식 품질·독립 표본2개로 합산하지 않는다.

CKV full/default/minimum density 각2응답과 CKS v2 2응답/16 source citations/bodies를 확인했다. default는 꼬리 문장을 보존하지만 minimum signature는 이를 축약하며 citation/parent는 유지한다. 최소 budget20의 실제 estimated105-token floor를 보존한다. 공개 Go Verify·소스 SHA·full exact vector inventory·DB/config/model 불변, raw/text K10과 knowledge K6 scope를 대조했고 best-effort nonreturned14회를 남겼다. Python 11+15·관련 Go 장문/parent 회귀를 통과했다. budget1 공개 거부와 generated YAML/K6 helper 가정 오류도 보존했으며 제품 수정/C0 실패로 세지 않는다.

사용자의 전체 완료까지 지속 진행·세부 완료마다 화면/문서에 현재 단계·전체 미완료·다음 작업을 표시하는 지시를 재기록한다. 현재 구현 완료2/30·진행11·대기17·공식 게이트0/4·미완료28개는 그대로다. F-02 독립 준비를 마쳤고 다음은 F-04/F-05 상태/프로젝트 오라클 보고 연결이다. B0-01/02/03/05·STV2-01과 OP-01–08의 실제 결정은 비동기로 요청했으며 일반 전체 완료 지시를 특정 사실/입력/운영 승인으로 기록하지 않았다. 결정 도착 후 입력 동결·새 strict 공식 B0-07 → B0/B1 → 사람/C0 → 최종 C1 순서를 우선한다.


## 25. FIX-17 최종 조립 자료 경계 재감사

2026-10-05 F-02 마지막 경계 검사에서 이전 raw report의 보관 Go 소스 `.go` 두 파일이 system 영역의 실제 코드로 인식되는 실패를 확인했다. 이전 raw report의 검사는 이 사본을 조립하기 전이었고, F-02 검사가 실패했는데도 작업 흐름이 후속 커밋으로 진행하여 0bc81ab3에는 최종 manifest가 누락됐다. 사용자에게 이를 알리고 먼저 수정했다. 성공으로 기록하거나 승인 대기 문제로 돌리지 않는다.

원문 SHA와 바이트를 그대로 보존한 `.go.txt` 사본으로 전환했다. checker 예외를 추가하거나 엔진 경계를 완화하지 않는다. [이름/원문 대조](../../system/eval/b0-knowledge-system/f02-document-preparation-m2max-2026-10-05/snapshot-rename-audit.json)와 수정 전 실패를 보존하고, 최종 조립된 자료를 대상으로 경계·Go package inventory·문서·현황·diff·모든 manifest SHA/추적 상태를 확인한 뒤 후속 커밋한다. 이는 평가 자료 분류/검증 순서 수정이며 공식 C0 검색 품질 실패의 종료 증거가 아니다. 전체 28개 미완료와 다음 F-04/F-05 준비는 유지한다.


## 26. F-04/F-05 개발 격리 진단과 FIX-18

2026-10-05 [상태·프로젝트 보고 자료](./B0-STATE-ISOLATION-REPORT-PREPARATION.md)와 [원장](../../system/eval/b0-knowledge-system/state-isolation-preparation-m2max-2026-10-05/summary.json)을 추가했다. 네 strict BGE-M3 DEV 상태의 원래/수정 바이너리 8 SDK 응답·28인용/본문은 자기 source/범위/hash/좌표·공개 Go Verify와 같고 선언된 code/policy body 범위를 포함한다. raw/text K10·knowledge K6·best-effort nonreturned8회와 모든 실패/누락을 보존한다. F-05 Markdown은 raw 합성 문서이고 verified pack으로 승격하지 않았다.

FIX-18은 실제 old CKV/new source에서 stale 인용과 fresh=true 요약의 모순이다. Git 커밋 변경/0 hit의 수정 전 실패와 query 전체/race/vet·공개 API/MCP/CLI/client/v2 계약 재검증을 보존한다. 수정 후 같은 실제 모델·DB에서 fresh=false이고 trace ID를 제외한 나머지 응답은 같다. replay 전후 payload/source/config/request·모델 metadata 불변을 확인했다. HEAD 미확인/보관 소스·dry run의 best-effort 호환을 유지하며 공식 C0 종료로 합산하지 않는다.

다른 native 디렉터리 graph/vector를 혼합한 두 설정은 SDK reindex_required/CLI operation_failed로 거부됐다. 초기 helper의 오류 보고서 미생성 가정을 수정하고 보존 partial을 검사했다. 이 설정 거부를 gold의 pinned 혼합 좌표 snapshot_mismatch로 바꾸어 세지 않으며 정확한 공식 혼합 오라클은 남는다. 12개 Python 회귀·기존 F-02 11개와 최종 조립 문서/경계/현황/원장 검사를 수행한다.

완료2/30·진행11·대기17·공식 게이트0/4는 유지한다. 다음 독립 작업은 F-01 실제 희소 무필터/full eligible·상한 오라클의 개발 진단이다. B0-01/02/03/05·STV2-01 결정이 도착하면 입력/source/pack/semantic/K 동결과 새 strict B0-07을 우선한다. B1 공식 paired·사람 판단→C0 관측 수정→C1 독립 최종/운영과 OP-01–08·native/legal/대규모/복구 범위가 모두 남는다.


## 27. F-01 실제 희소 검색·상한 개발 진단

2026-10-05 [F-01 자료](./B0-F01-SPARSE-DIAGNOSTIC.md)와 [원장](../../system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/summary.json)을 추가했다. 고정 DEV source61개/Git blob 동일·strict BGE-M3 Go60파일/122청크/truncated0이다. K5 무필터가 대상 파일4개를 누락하며 전체 eligible Function5개·정상 필터 exact ID/좌표/거리·budget2 incomplete/candidate_limit 조건을 충족했다. coarse SQL 후보60과 eligible5, raw 중복 슬롯과 구성 인용, 실제 @5와 @10을 분리한다.

원본/보관 소스 reader 재생의 diagnostic_controls_pass·invariant0과 probe 전후 DB/manifest/graph·source/model 불변을 확인했다. 원래 초안/승인 입력5개를 바꾸지 않았고 FINAL 실행 없음·공식 F-01/품질/사람 verdict는 null이다. 기존 역사적 Alpha K1 자격 실패를 소급 변경하지 않는다. 기존 reader15개·최종 조립 문서/경계/현황/원장 대조를 수행했다. Go 제품 변경은 없으며 FIX-18의 이미 통과한 query/공개 계약 시험을 불필요하게 반복하지 않았다.

완료2/30·진행11·대기17·공식 게이트0/4는 유지한다. 다음 우선순위는 B0-01/02/03/05·STV2-01 실제 결정→입력/source/pack/semantic/K 동결→B0-07 공식 strict 빌드다. 독립 준비는 기존 F-03/F-06 proposed/unknown·검토 자료와 전체 실행 선행 조건을 재감사한다. 사람 판정/운영/실제 native amd64가 필요한 나머지를 진단 완료로 대신하지 않는다.


## 28. F-03/F-06 보관 proposed 재생과 전체 실행 선행 조건

2026-10-05 [전체 실행 재감사](./EXECUTION-READINESS-REAUDIT.md)와 [원장](../../system/eval/b0-knowledge-system/readiness-reaudit-m2max-2026-10-05/summary.json)을 추가했다. 보관 mock/BGE-M3 F-03/F-06 SDK20응답/source·투영 줄 SHA·요청/설정/바이너리·공개 Go Verify와 baseline/combined 인용/본문 집합 동일성을 확인했다. proposed/draft·관계적용/boost0·F-06 unknown/빈 규범 tuple·CHECKED_BY 무승격을 유지한다. mock no_candidates와 real 다의어2를 구분하고 새로운 모델/FINAL 실행·품질 합격으로 합산하지 않았다.

정적 입력과 v2 scope의 실제 checker를 다시 실행해 exit2/pending·정적12승인/동적0승인·protocol/dynamic/scope 사람 SHA 연결 없음·official_execution_ready=false를 확인했다. 입력5개와 과거 원자료 바이트가 같다. 전체 여섯 가족의 진단/남은 공식 오라클을 연결하고, FIX-18 이후 최종 패키지는 새 공식 후보로 재빌드해야 함을 기록했다. 사람의 ‘검토 후 결정’·SF/OP/운영/native 선행 조건을 일반 계속 지시로 대체하지 않는다.

완료2/30·진행11·대기17·공식 판정0/4, 전체28미완료다. 다음은 B0-01/02/03/05·STV2-01 실제 결정 기록→source/팩/의미/scope/K 동결→새 strict B0-07→공식 B0/B1→사람/C0→최종 C1이다. OP-01–08·실제 native amd64·전체 native/legal·대규모 비용·최종 복구/출시도 남는다. 승인 도착 전 미검토 입력으로 최종 실행하거나 사람 verdict를 만들어 작업을 종료하지 않는다.


## 29. FIX-19 보관 소스·혼합 pinned 좌표의 실제 SDK 오류 구분

2026-10-05 [보관 소스 제어 진단](./B0-RETAINED-GUARD-DIAGNOSTIC.md)의 수정 전 네 subtest와 실제 SDK 음성4건을 보존했다. InspectVersionIdentity의 알려진 source 오류를 MCP가 reindex_required로 덮고 engine 좌표 불일치에 구체적 코드가 없던 원인을 수정했다. 정상 pinned SDK 초기화 후 DEV 사본의 전체 vector를 다른 상태/프로젝트로 교체해 snapshot_mismatch, blob 변조 snapshot_mismatch·삭제 source_missing과 backend0을 확인했다. 과거 비인접 설정 거부를 소급 변경하지 않았다.

수정 전후 각 SDK10회/누락0·정상복원6응답/20인용/20본문은 공개 Go Verify·원문/좌표/본문 SHA를 통과했고 정상 evidence 집합이 같다. 원본/최종 사본 DB/source·설정/바이너리·BGE digest/metadata 불변과 실제 rawK10/별도K6을 확인했다. helper 필드명/빈 WAL-SHM 가정 실패는 제품 실패와 분리해 보존했다. 비어 있지 않은 WAL은 사본 생성에서 거부한다. 영향받은 계약 시험/race/vet·SDK·독립 reader·최종 문서/경계/현황/원장을 확인한다.

완료2/30·진행11·대기17·공식 게이트0/4·전체28미완료는 유지한다. B0/STV2 실제 결정 후 공식 strict 빌드가 최우선이며, 독립 준비로 최신 FIX-19 패키지의 시험 범위 검증을 진행한다. B0-01–09·B1-03–08·C0-01–06·C1-01–07과 STV2-01·OP-01–08·native amd64·대규모/최종 복구/출시가 모두 남는다.


## 30. 최신 FIX-19 package·실모델·복구 준비 재검증

2026-10-05 [최신 preview 검증](./C1-LATEST-PREVIEW-VALIDATION.md)을 추가했다. 31def76b macOS/동일 tracked bytes Linux arm64·amd64 에뮬레이션 preview를 새로 빌드했고 archive/바이너리/고지 SHA·시험 서명·mock 세 프로젝트 설치/조회/재시작/업데이트/롤백과 macOS archive 재현성을 확인했다. Darwin35/51·Linux33/49·111/107항목이며 적합성은 pending이다. 잘못된 캐시 경로의 첫 Linux 실패를 그대로 보존했다.

설치할 Darwin cks의 실제 BGE SDK10·음성4/정상6·20인용·본문을 다시 검증했다. Linux arm64는 read-only 승인 model byte SHA/digest·Ollama0.35.1·CPU residency와 모델2CPU/4GiB/caller1CPU/512MiB를 기록했다. 세 fixture 실제 설치/업데이트/rollback 뒤 baseline/knowledge6 SDK·8인용·본문의 Go Verify/원문 SHA·벡터1024/finite/정규화·실제K10/6·계측 연결/입력 불변을 확인했다. 첫90초 initialize 실패·종료 부근400과 원래65anchor 직접200·후속180초 진단 deadline을 분리한다. 미완료된 첫24 SDK 계획을 후속6응답으로 채워 성공 처리하지 않았으며 공식 지연/비용은 null이다. 임시 서버를 종료/자동 삭제했다.

최신 extracted preview의 작은 backup clone/원본 소스 없는 재생·실패 후보/손상 대상 거부와 고정1ded9b3 v1 소비자/합성 HTTP·새 mock v2/legacy rollback도 통과했다. 운영 복구·실제 legacy migration·native amd64·전체 legal·지원/대규모 비용·출시 판정은 남는다. 원자료504개는 binaries/DB/model/private keys를 제외해 보관했고 최종 원장/문서/경계/현황을 검증한다.

전체완료2/30·진행11·대기17·공식0/4·28미완료는 유지한다. 다음 작업은 실제 B0/STV2/SF 결정 기록과 공식 입력 동결/strict 빌드이며 B0-01–09·B1-03–08·C0-01–06·C1-01–07 및 STV2-01·OP-01–08·native/운영 최종 범위를 모두 이어간다. 공식 후보는 품질/수정 동결 후 다시 패키징한다.


## 31. 부분 대규모 데이터셋 비용과 Backend 재현

2026-10-05 [C1 비용 진단](./C1-LARGE-COST-DIAGNOSTIC.md)의 최신 FIX-19 패키지/승인 BGE-M3로 기존 부분 데이터셋1,569파일/13,575청크를 별도 사본에서 조회했다. 개발1문항의 baseline/knowledge 각 warmup2·검색5·warm20·cold process3, 계획60 SDK·480인용/본문이 source SHA/줄/좌표/독립 Python·public Go Verify를 통과했다. warm p50/p95는 baseline3.521/3.737초·knowledge3.824/4.408초, cold process 시작부터 응답 p50은6.627/6.224초다. single DEV 반복/비회전/비격리·모델 residency·수정 전 부분 빌드29 Go 제외의 한계를 유지하고 공식 품질/지연으로 합산하지 않는다.

SDK 실패/누락0과 backend 오류840(BM25480·neighbors360)을 구분했다. HTTP639/별도 startup7개 anchor embed455/HTTP1,365는 모두200이다. input SHA8개에 결합한 문서 후보의 bare FTS 오류와 literal quoted0결과를 읽기 전용 Python SQLite에서 재현했다. neighbors 전체 원인은 아직 확정하지 않았다. 별도RSS 보완2 SDK/16인용·본문도 같은 근거/원문/Go Verify를 통과했다. 실제 MCP의 observed RSS 최대82,992KiB와 동시 합106,656KiB는 샘플 peak/공유 페이지/모델 서버 미포함을 명시한다. 주60과 보완2의 분모를 분리한다.

원본/사본 DB/manifest/source·고정 source tree·package binary/config/request·모델 tag/version/digest·사람 입력5개 불변이다. 원자료48개+SHA manifest를 결합했다. 신규 제품 변경은 아직 없고 기존 공개 verifier·독립 원문 감사를 재사용했다. 다음 독립 작업은 BM25 후보의 실제 FTS 경계 회귀 재현/영향 검토이며, 공식 최우선은 실제 B0-01/02/03/05·STV2 결정→동결→새 strict B0-07이다. 완료2/30·진행11·대기17·공식게이트0/4·전체28미완료 및 OP-01–08/native/legal/운영복구·출시는 유지한다.


## 32. FIX-20 후보 문자열의 FTS 문법 오류 수정

2026-10-05 [FIX-20 진단](./B0-FTS-KEYWORD-DIAGNOSTIC.md)에 대규모 DEV의 BM25480오류 중 후보8개를 input SHA/보관 source에 결합했다. 후보를 FTS literal로 전달하는 helper를 Stage1 rerank·Stage2 기본/테스트 보조 검색에 적용했다. 일반 식별자 기존 query·원래 symbol 이름·명시적 raw FTS/public search_text OR 계약을 유지한다. 수정 전 두 회귀 exit1→실제SQLite의 punctuation/연산자/quote/한글/문법 삽입 비확장→관련7패키지/race3/vet→동일 실모델DEV2 SDK/16인용·본문/public Go Verify를 확인했다. 실제 literal 후보SHA8개와 BM25 오류 호출당8→0·근거/좌표 보존을 대조했다.

원본/사본 source/DB·model·binary/config/request·사람 입력5개 불변, SDK 실패/누락0이다. neighbors12오류는 미수정이며 실제 seed/노드 경계를 다음 독립 작업으로 확인한다.31원자료+해시를 기록한다. 기존 FIX-19 패키지는 이 수정본을 검증한 것이 아니며 최종 품질/수정 동결 후 새 패키지가 필요하다. FIX-20을 공식 C0 검색 품질 완료로 세지 않고2/30·진행11·대기17·0/4·전체28미완료를 유지한다. 공식 최우선은 실제 B0/SF/STV2 결정→동결→새 strict B0-07이다.


## 33. 선택된 문서·코드의 실제 Go neighbors 재생

2026-10-05 [neighbors 진단](./B0-NEIGHBOR-REPLAY-DIAGNOSTIC.md)에서 FIX-20 SDK가 선택한8인용을 real Go adapter에 직접 재생했다. 문서6개는 symbol 후보0·qname미해결·no node at이며, 코드2개는 오류 없이 조회된다. Git hunk/path/file 구조 노드를 코드 관계로 승격하지 않았다. 임시 진단 Go 테스트를 실행하고 원문을 `.go.txt`로 보존한 뒤 코드 트리에서 제거했다. 제품 변경/일반CI 추가·실모델/SDK/FINAL 추가 실행 없음이다.

실제 이전 neighbors 원장은 source 인수를 기록하지 않으므로 직접8개를 과거360 또는 새SDK12의 전체seed로 일대일 결합할 수 없다. 기존 failed_seeds6과 정합적이지만 전체 원인 확정/안전·품질 통과로 확대하지 않는다. 입력capture/adapter/helper SHA와 원본/사본 DB/source·사람 입력5개 불변·원자료5개를 기록했다. 다음 공식 최우선은 실제 B0-01/02/03/05·STV2 결정→입력 동결→새 strict B0-07이다. 완료2/진행11/대기17·공식0/4·미완료28개와 OP/native/legal/최종복구·출시를 유지한다.

## 34. FIX-21 Neighbors 계측의 실제 입력 결합

2026-10-05 [인수 해시 진단](./B0-NEIGHBOR-BINDING-DIAGNOSTIC.md)에서 기존 neighbors의 빈 입력 계측을 보완했다. 실제 Citation의 Go JSON SHA/바이트만 기존 원장에 기록하며 원문 경로·commit을 노출하지 않고 scope가 없으면 직렬화하지 않는다. 수정 전 실패→관련6패키지/race2/vet/build→같은 BGE/부분13,575청크의 계측 on/off4 SDK·32인용/본문·Go Verify를 확인했다. 전체 SDK 응답은 동일하다.

새 실제 neighbors20개를 indexed source 후보의 SHA/길이로 결합하고 같은 옵션의 real Go adapter 재생으로20/20 결과·개수 일치를 확인했다. 문서12개 symbol없음/no node at·코드8개 정상이며 과거 미결합360/12오류는 소급 판정하지 않는다. source/원본/사본 DB·모델·바이너리/설정·실제 사람 입력5개 불변·39원자료를 기록했다. 임시 Go 테스트는 원문 `.go.txt`로 보관 후 제거했다. 조립 보조 도구 필드 착오는 별도 실패 기록 후 수정했으며 SDK/제품 실패가 아니다.

B0-06의 계측 보완 완료를 공식 품질 게이트 완료로 세지 않는다. 완료2/진행11/대기17·미완료28·게이트0/4다. 다음 공식 최우선은 실제 B0-01/02/03/05·STV2 결정→입력 동결→새 strict B0-07이다. FIX-19 preview는 FIX-20/21 이후 최종 패키지 검증이 아니며 최종 동결 후 재빌드한다. B0/B1/C0/C1·STV2·OP-01–08·native amd64·고지/복구/지원/출시는 남는다.

## 35. C1-05 원래 SQLite·musl 및 Linux 헤더 출처 보완

2026-10-06 [원래 native 출처 감사](./C1-ORIGINAL-NATIVE-ORIGIN-AUDIT.md)에서 고정 modernc/sqlite 3.53.3의 원래 C 공식 SHA3-256·C/공개 헤더 버전/source ID와 배포 모듈 상수가 같다. libc builder/archive가 지정한 고정 musl 원문을 수집해 C/헤더/템플릿/고지2,337항목을 봉인했다. 두 모듈 ZIP h1 재계산/go.sum과 선택 source/recipe/header/overlay501파일 바이트 일치·전후불변을 확인했다.

Linux arm64/amd64 각218헤더는 원래215·생성2·overlay1로 분류했다. 원래 Makefile의 좁은 명령으로 alltypes/syscall4개를 재생하고 float overlay2개를 대조했다. 최초 helper의 빈 줄 가정·수정 문법/출력 디렉터리 오류는 실패 원자료로 구분해 보존했고 별도 경로 재생6검사 통과를 확인했다. 49개 증거와 실제 사람 입력5개 불변, 제품/키/출시 변경 없음이다.

Darwin SDK·실제 include-use/생성기 버전·전체 C→Go 재생성과 전이 native 적합성/OP-06은 남는다. 이 기술 자료로 법무/운영 승인이나 C1-05 완료를 대신하지 않는다. 완료2/진행11/대기17·미완료28·공식0/4다. 다음 공식 최우선은 실제 B0-01/02/03/05·STV2 결정→동결→새 strict B0-07이다. 전체 B0/B1/C0/C1·STV2·OP-01–08·native amd64·최종 고지/복구/지원/출시를 현황판에 유지한다.

## 36. 2026-10-06 명시적 승인·FIX-22 범위 보완

사용자가 다섯 결정을 명시적으로 승인했다. [새 입력 기록](./B0-APPROVED-INPUTS.md)의 전후 해시·원문·로컬 검토자/시각으로 프로토콜, 12개 동적 변형, STV2, 테스트 사례 SF 판정을 연결했다. 이전 pending 답변은 보존한다. 운영 키/법무/출시와 실제 답변 주장 판정의 승인은 포함하지 않는다.

FIX-22는 정확한 Go 파일 경로 목록만 CKV 기본 build/ 제외를 통과시킨다. 기존 명시적 ignore·다른 기본 제외·비밀·링크·binary·크기·allowlist/build_roots 필터는 유지한다. 옵션 JSON bytes는 setup의 configured input digest와 dataset recipe에 포함하고 CKV manifest에 경로를 남긴다. 수정 전 실제 누락 시험 실패; 관련 Go 6패키지·수집/설정 race·B0 Python72 통과. 실제 고정 소스 discovery의 1,569→1,598 차이가 승인 29개와 정확히 같다. 이는 인용/청크/graph audit의 성공을 대신하지 않는다.

현재 B0-02/03 입력 동결과 B0-01 범위 검증 완료. B0-07 새 strict 전체 빌드 실행 중이며 SF 새 커밋·review·잠금·투영을 준비한다. 다음은 원문/청크/embedding/정렬·doctor 감사 후 공식 DEV 기준선과 8-arm 비교다. 최종 질문으로 튜닝하지 않는다.

## 37. FIX-23 공식 실행의 stderr backpressure와 재개

새 승인 DEV 실행에서 서버 시작 계측260 logical/HTTP 이벤트가 stderr pipe를 채웠다. eval.NewRunner는 transport의 stderr reader를 소비하지 않아서 initialize가 멈췄다. 이번 Python wrapper/CKS 두 프로세스만 종료하고, 시도·실행 프로세스·시작 footprint·바이너리/설정 bytes를 보존한다. 이 시도에는 점수나 유효 지연 결과가 없다.

실제 Go 자식이 초기화 전에 stderr2MiB를 쓰는 회귀 시험은 수정 전 deadline 초과로 실패했다. 초기화 전부터 기본 os.Stderr로 비동기 전달하고 Close/초기화 실패는 drain 종료를 기다리도록 수정했다. 사용자 지정 sink 실패도 남은 pipe를 소비해 protocol을 막지 않는다. 정상/실패 초기화·로그 보존·idempotent close와 평가/matrix의 관련 시험·race·vet 통과. raw diagnostics는 평가 실행 로그에 남는다.

검증된 전체 DB는 기존 builder SHA를 보존하고 새 eval client/server SHA를 별도로 기록한다. FIX-23은 평가 transport 소비 변경이며 검색·CKV/CKG 빌더 코드는 같다. 동일 승인 입력/DB에서 새 출력 루트로 공식 DEV 실행을 재개한다. 이후 SF/나머지 동적 입력·최종 독립 평가와 사람 판정은 남는다.

## 38. FIX-24 모델 기본 제공자와 실제 정적 DEV 실행

정적 v1은 DEV4×5 원응답을 기록했으며 CODE/POLICY/TRACE는 miss, WHY만 pass다. 실행 exit0을 품질 합격으로 합산하지 않는다. v1 전체 인용 지표와 v2 top10 지표는 구분한다. 첫 두 실행의 HTTP 설정은 작업자가 잘못 전달한 독립 오류다. stdio로 바로잡고 알려진 HTTP 설정은 자식 시작 전에 거부하며, 외부 호환 서버의 opaque 설정은 기존대로 전달한다. 기존 opaque-config deadline 시험의 최초 실패와 호환 수정 후 통과를 보존했다.

8-arm 첫 preflight는 generated config의 빈 provider를 Ollama 서버 기본값과 다르게 해석하여 rows0/partial로 끝났다. 실제 수정 전 기본 제공자 회귀 실패를 보존하고 serving path의 DefaultProvider를 공유한다. 명시적 mock/다른 제공자·digest·dimension·redirect·cancelled 거부 계약은 유지한다. 관련 두 Go 패키지 시험/race/vet 후 새 runtime SHA를 별도 결합했다. builder/검증 DB와 승인 입력5개는 그대로다.

새 정적 DEV matrix는 환경 preflight valid=true로 시작했다. 네 질문·8arms·warmup2/검색5/warm20/cold3 순차 수집 후 독립 source/integrity/backend K/paired 지표 감사를 진행한다. 현재 완료6/진행12/대기12·전체24 미완료·공식0/4다. 다음은 matrix 감사와 F-01/02/04 새 strict 입력/동적 실행이다. 전체 B0-05–09·B1-03–08·C0-01–06·C1-01–07 및 OP-01–08/native amd64/고지·복구·지원·출시는 남는다.

## 39. FIX-25 v2 선택 본문과 추가 그래프 인용 분리

[C0 실제 실패/수정](./C0-V2-BODY-BUDGET.md)은 승인 DEV에서 모든8arms의 WHY 오류를 재현한 뒤 v2의 선택 본문만 retained 원문으로 검증하도록 수정했다. 기존 v1 그래프·검색/정답/상한은 그대로다. 첫 matrix77/960행·17실패·미측정883행을 보존하며 공식 품질/지연 판정에 합산하지 않는다. 수정 전 handler 실패→관련4패키지/3race/4vet→실제 DEV20 Go 재생/SDK4 source-integrity 감사 통과다. 정적 miss3개와 전체 동적/최종·사람 판정은 남아 있다.

F-01/02/04는 독립 source4개와 새 strict BGE-M3 dataset으로 빌드했다. F-02 문서 전용 doctor degraded는 예상 제한이며 ready로 바꾸지 않는다. wrapper의 doctor 옵션 착오/모든 사례 strict-ready 가정은 별도 실패로 보존하고 성공 빌드는 재사용했다. 다음은 실제 source/chunk/parent/vector와 F-01 exact/budget·F-04 state 오라클 감사, 새 전체8arm 평가다. 완료6·진행17·대기7·전체24 미완료·공식0/4다. B0-05–09/B1-03–08/C0-01–06/C1-01–07 및 OP/native/legal/복구/지원/출시가 남는다.


## 40. FIX-26/27 실제 DEV 순위 회귀·보관 검증 비용

승인된 동적 8상태/6가족의 2,880 SDK 행을 계획대로 완전 캡처했다. 모든 행의 원문/인용/integrity 및 공개 Go v2 계약 재생 위반은 0이다. F-01은 public SDK의 비필터 요청과 별도 실제 SearchDetailed K5/필터/exact/상한 오라클을 구분한다. F-02 tail480, F-03 proposed 확정 금지/후보 유지720, F-04 상태 격리480, F-05 프로젝트/정책 격리480, F-06 정책 표시/미검토 acceptance 금지480을 감사했다. 반복은 독립 표본 수를 늘리지 않는다.

F-05-A/B와 F-06의 세 온톨로지 모드에서 MRR0.75→0.666667 회귀를 확인했다. 같은 canonical 함수의 본문+헤더에 추가 점수를 중복 적용해 정책 순위가 밀리는 원인을 FIX-26으로 재현/수정했다. FIX-27은 요청-local batch의 전후 전체 보관 검증과 각 blob 해시를 유지하면서 본문별 전체 반복 검증을 줄였다. 관련4시험/race/vet 통과, 실제 정적 SDK4 pack 동일/오류0이며 단일 진단2.09–4.55초는 공식 warm 판정이 아니다. [수정 요구/시험/증거](./C0-ONTOLOGY-HEADER-AND-ARCHIVE-BATCH.md)를 참조한다.

정적 matrix는112/960행을 partial로 보존했다. 새 runtime으로 모든 DEV3,840행을 같은 승인 모델/입력/질문/K/반복으로 다시 수집한다. 원자료112MB를9.3MB lossless archive로 보관하며 압축 전후 SHA를 결합했다. 전체 Go 검사는 과거 불완전 source excerpt가 패키지로 발견되는 실패를 보존하고, 원자료 bytes/경로를 그대로 둔 nested-module 경계 수정 후 통과했다. B0 Python74검사도 로컬 테스트 서버 권한으로 통과했다.

FINAL8상태 strict 빌드는 완료했으며 source/chunk/vector 감사 중이다. 승인한 FINAL 정의와 SF 사실은 marker/module/project namespace 치환 후 정확히 같은 원문으로 결합한다. cap/반환 값/TestHealth/vocabulary 변화를 기존 승인으로 넘기지 않는다. 새 human review를 했다고 기록하지 않으며 F-03 proposed와 acceptance 비승인 범위를 유지한다. 최종 질의·점수는 아직 실행하지 않았고 개발 동결 후 C1에서 평가한다.

[B1 실제 출력 주장 검토표](./B1-APPROVED-OUTPUT-CLAIM-REVIEW.md)의 HC-01–08과 시간/비용을 사람 판정에 요청했다. 입력 승인과 답변/정책 해석, 시간 미측정과0비용을 구분한다. 현재 완료6/진행17/대기7·전체24미완료·공식0/4다. 다음은 새 DEV 순위/지연·정적 miss 분류→개발 동결→FINAL 재평가이며 전체 B0-05–09/B1-03–08/C0-01–06/C1-01–07·OP/native/legal/최종복구·지원·출시가 남는다.

FINAL source/vector 감사8상태150청크 통과. [승인 FINAL 입력 원장](../../system/eval/b0-knowledge-system/approved-final-inputs-m2max-2026-10-06/artifact-manifest.json)에 source·bundle·native manifest·semantic 투영·검토 상속 증거를 연결했다. B0-05/07의 명시된 입력 결합/빌드 완료를 검증 완료로 변경하며, 쿼리/품질/사람·출시 판정은 남긴다. 현재 완료8/진행15/대기7·전체22미완료·공식0/4다.

## 41. 새 개발 동적 2,880 응답의 독립 검증과 후속 검사 연결

2026-10-06 FIX26/27 실행 바이너리 `3b80c36d77b5c052894058ec59b12d64f2e5f9e5800aa11314be9e362189d0e6`로 승인 DEV 동적8상태의 2,880회를 완료했다. 각 source/identity·원문/인용 SHA·상태/프로젝트 marker·미검토 의미 비승격 오라클 위반0, backend 계측 누락0이다. 공개 Go EvidencePackV2.Verify도 동일 2,880행을 재생하여 오류0이다. F05 A/B와 F06의 두 기대 인용 평균 역순위는 모든8arm에서 .75이며 수정 전 optional .6666667 회귀가 해소됐다. 이 지표는 첫 관련 인용 MRR나 사람 답변 합격으로 바꾸지 않는다.

정적960 수집 종료 뒤에만 독립 정적 보고/Go Verify → 승인 DEV 복사본의 실제 MCP 상태·프로젝트 혼입/원문 손상·없음 → Python B0 회귀 → 전체 DEV raw CKV exact/상한 → 새 실행 바이너리 v1 anchor 검사를 순차 실행하도록 후속 드라이버를 연결했다. 수집 중에는 추가 모델/빌드/시험을 실행하지 않는다. 소형 동적12요청/2,880반복을 12개 또는2,880개 독립 표본으로 합산하지 않는다. F01 필터/exact@5는 별도 native 오라클이며 공개 v2에는 filter 인자가 없다.

최종4800 수집 드라이버는 개발 동결 기록·승인5입력 SHA·바이너리/제품 소스 SHA가 일치해야 실행할 수 있다. 아직 최종 질의를 실행하지 않았고 개발 원자료를 최종 결과로 재사용하지 않는다. HC01–08 출력 해석/사람 검토 비용과 C1 시험 preview/실운영 종료 범위는 이미 요청된 응답을 기다리며, 입력①–⑤를 다시 묻지 않는다. 전체 완료8/30·미완료22·공식 게이트0/4이고 B0-06·08·09, B1-03–08, C0-01–06, C1-01–07 및 OP/native amd64/고지·복구·지원/출시가 남는다.

## 42. DEV3840 자동 검증 완료·기술 동결과 독립 FINAL 시작

2026-10-06 [승인 개발 결과 보고](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md)에 static960+dynamic2880 전체, 공개 Go/source 오류0·measurement 누락0·입력/저장소/바이너리 SHA 불변을 연결했다. 현재 SDK guard10호출 정상6/부정4, source/프로젝트/state 혼입 snapshot_mismatch 및 missing source_missing·backend0·원본/복원 불변을 확인했다. F04 old CKV/new source stale 실제검사, raw16, 새 v1 anchor20 및 measurement/backend/source 연결20 완료. B0 Python75/B1보고26 통과.

정적 baseline miss3는 raw full exact ranks1085/47·52/301·1490으로 top10 밖이며 WHY raw1/composed2다. F02 noise 동점의 순서만 다른2건은 original exact_agreement=false를 유지한다. static combined_on warm ratio1.292, dynamic on1.315–1.614, pooled combined_on1.065를 모두 보여준다. 사후 선택한 pool로 전체 pass를 만들지 않는다. 사람 HC/비용, 작은 중요 질문군 및 운영 범위는 남는다.

제품 실행 SHA3b80c36d…89d0e6과 source1096파일·승인5입력을 동결했다. corpus label과 별도 dataset project ID는 독립 strict 전체 identity audit로만 결합한다. 보고 첫 오류와 기존 pending 시험의 real 승인 의존 오류를 보존하고 test 사본에 명시적 pending을 주었다. 기존 opt-in guard 시험은 승인 Prompt 선택을 지원하면서 빈 Prompt의 이전 동작을 유지했다. 런타임·gold·threshold는 바꾸지 않았다.

C1-01 FINAL4800을 별도 소스로 실행 중이다. static transport manifest의 누락된 public v2 tool 이름만 해당 capture 전 명시했고8prompt/date/subsystem은 그대로이며 수정 전후 SHA를 남겼다. 이 표현 보완을 최종 결과 튜닝으로 사용하지 않았다. B0 baseline와 C0 technical task 완료를 B1 사람 판단·C1 품질/운영 승인으로 합산하지 않는다.

## 43. 승인 FINAL4800 실패 포함 감사와 기본값 결정

2026-10-06 [최종 보고](./C1-APPROVED-FINAL-REPORT.md): 8동적상태2880·정적8문항1920 전체 계획행 수집. 정상팩4560의 source/public Go 유효, POLICY03의240개 오류는 공개 Go의 capture_error와 정확히 같은 sequence다. 실제 Go 원문10개 검증 후33104바이트에서 budget_exceeded가 발생한다. 현재32KB 계약 상한을 올리거나 FINAL에 맞춰 코드를 수정하지 않았다. source 혼입/파일손실로 확인된 실패가 아니며 후보의 질의 처리 제한으로 기록한다.

정적 양성6개 중 POLICY02만 회수(Recall1/6·MRR0.5/6), v1도1pass/5miss. 두 기권 문항의 인용0개 guard는 모두fail이며 사람의 답변 기권은 별도다. 모든8arm 정적 양성점수가 같고 F05/06 모든8arm MRR.75. raw20 F01 자격·F02 동점만 다른 순서·F04 actual stale 및 v1 measurement40 검증 완료. 실패240을 성공시간/점수에서 지우지 않으며 성공 조건부warm/cold와 전체계획 분모를 함께 남겼다.

진단 원장 경로 재사용으로 추가한9로그는 전체파일/추가tail/원측정prefix 및3SHA를 보관하고240개 원 measurement ID의 prefix로 복원했다. report/rows·5승인입력·native 저장소·동결바이너리/1096소스는 그대로다. 원자료237MB를20.37MB 무손실압축 및oracle 원장으로 보관했다. 최초 Go 실패exit코드 가정/보고 분모 오류도 수정 전후 자료로 보존한다.

C1-01/02/03의 수집·판정·disabled 결정 범위를 완료했다. 전체22완료/8미완료이며 official stage ledger는 사람·운영 종료가 남아0/4다. 다음 우선순위는 최종 소스 checkpoint 후 최신 세 플랫폼 package/실모델·대규모/복구 검증이다. 남은 전체 B1-04·06·07·08, C1-04·05·06·07 및 HC/비용·OP01–08/native amd64/적합성·지원/출시 범위를 유지한다.
