# 현재 실행 명세와 완료 판정

기존17/18·온톨로지4/4 완료, N16 미충족 보류다. 목적·현재 기능은 [목적과 범위](./PURPOSE-AND-SCOPE.md)와 [아키텍처](../ARCHITECTURE.md), 앞으로의 작업은 [남은 전체 작업](../REMAINING-WORK.md)을 따른다.

체크된 조건은 원장의 해당 revision에서 검증한 범위다. 조건 문구와 ID를 문서 정리로 완화하거나 누락하지 않는다. 아래 수용 조건은 이전 명세에서 그대로 유지한다. 완료 경로·당시 구현/시험/실측/승인·한계는 [원장](./REFACTORING-PROGRESS.json)과 [증거 복구](./EVIDENCE-RECOVERY.md)를 통해 읽는다. 현재 source/input SHA, 조건 ID, 작업표와 잔여 수는 `make refactoring-check`가 검사한다. 기계 검사는 사람 사실과 목적 적합성을 자동 판정하지 않는다.

## N-01: 원문 복구와 초기 요구사항 대조

상태: 완료.

초기원문 INV7/FR10/NFR5/반례9와 WBS29 대응의 복구를 완료했다. 상위 외부 WI 감사나 PDF 재검증을 포함하지 않는다.

근거: 원장 N-01의 `evidence_revision` / `manifest`.

## N-02: 공개 유지보수 쓰기와 v2 불변 버전

상태: 완료.
- [x] N02-A 공개 MCP JSON-RPC `ops.index`: full/incremental, v2/버전 별칭/손상된 pin 거부, 엔진 호출·corpus 쓰기 0, 기존 current/identity/보관 원문 불변.
- [x] N02-B 공개 `ops.setup`/`ops.reindex` 우회 차단. 공유 builder도 기존 버전 재사용·v2→legacy downgrade 거부.
- [x] N02-C versioned setup과 공유 Reindex: 성공 시 완성된 신원/원문 후보만 승격; 한 엔진 실패·혼입·취소 시 current/기존 신원/원문 보존.
- [x] N02-D legacy 평면 갱신·새 legacy blue-green 후보 호환. 도구 설명과 이 문서에 migration 경계 반영.
- [x] N02-E 관련 전체 패키지/공개 스키마 회귀, 코드 SHA와 실행 원자료 연결. 미해결 제한은 명시.

근거: 원장 N-02의 `evidence_revision` / `manifest`.

## N-03: 빌드·승격·rollback의 공통 OS 잠금

상태: 완료.
- [x] N03-A 긴 live holder의 오래된 시각/동시 acquire에서 두 writer 없음; 잠금 inode 유지.
- [x] N03-B build holder와 Promote/Rollback/사람 검토 승격이 같은 lock으로 직렬화; 내부 Reindex 성공/보류 경로에는 재획득 오류 없음.
- [x] N03-C 별도 프로세스의 동시 요청/강제 종료 후 OS 자동 회수. 살아 있는 legacy PID는 오래됐어도 탈취하지 않음.
- [x] N03-D symlink/비정규 잠금 거부·legacy 전환 경계 문서·관련 회귀/race/vet와 소스/원자료 바인딩.

근거: 원장 N-03의 `evidence_revision` / `manifest`.

## N-04: 후보 저장·current 전환·복구의 내구성

상태: 완료.
- [x] N04-A DB/WAL/close 및 전체 후보 artifact 내구성 순서.
- [x] N04-B current 전환·parent sync와 단계별 실패 주입, 결과 상태 구분.
- [x] N04-C 일반/검토 승격·hold/release·rollback와 재시작/강제 종료 복구.
- [x] N04-D 플랫폼별 보장/전원 차단 미실측 한계 및 회귀·원자료·소스 바인딩.

근거: 원장 N-04의 `evidence_revision` / `manifest`.

## N-05: 보존 정책·검토 가능한 GC와 reader pin

상태: 완료.
- [x] N05-A 결정적인 dry-run·정책/용량/보호 이유·검토한 계획 digest에만 결합된 실행.
- [x] N05-B current/rollback/reader/review/보관 원문·Git history·semantic source 보호 및 구 reader 호환 경계.
- [x] N05-C 경합·stale plan·취소·중단/trash 복구·실제 삭제 검증, 보호 참조 삭제0.
- [x] N05-D 공개 CLI/서버 pin·상태/오류 비노출·관련 회귀와 소스/실행 증거 바인딩.

근거: 원장 N-05의 `evidence_revision` / `manifest`.

## N-06: setup/config 자원 상한과 build deadline

상태: 완료.
- [x] N06-A YAML/CLI/default/shared API의 상한·deadline 전달 및 입력 검증.
- [x] N06-B 초과/timeout/취소·외부 pack 합산에서 부분 후보 비활성·이전current/identity/원문 불변.
- [x] N06-C 예상/실제 free bytes·공개 resource 상태 및 오류/민감 경로 비노출.
- [x] N06-D 관련 공개/호환 회귀와 수용 조건별 소스·실행 원자료 바인딩.

근거: 원장 N-06의 `evidence_revision` / `manifest`.

## N-07: 보관 원문 인용의 본문 예산

상태: 완료.
- [x] N07-A 독립 DEV에서 byte/citation 경계·oversized→fitting·모두초과 RED→GREEN 및 원문좌표/hash 무절단.
- [x] N07-B 안전한 partial/typed budget/정상·중복·zero/drop 분모와 integrity/상한 검증.
- [x] N07-C 생략 span 포함 source_missing/mismatch/비밀·scope/knowledge/trace 허위 수용0.
- [x] N07-D 실제 공개 MCP 경로와 관련 회귀·입력/소스/원자료 바인딩·이전FINAL 불변.

근거: 원장 N-07의 `evidence_revision` / `manifest`.

## N-08: 검색 품질(사용 타입 선언)

상태: 완료.
- [x] N08-A 질문/gold hardcode 없이 원인을 분리하고 후보를 구현·시험. 원인은 타입 선언의 순위와 `uses_type` edge 누락이다.
- [x] N08-B 사전 승인한 새 독립 FINAL에서 이전/새 binary의 품질·회귀를 판정. 중요군 소표본은 inconclusive로 명시하고, 개선 근거가 없음을 기록.
- [x] N08-C 개선 근거가 없으므로 삽입을 opt-in(`retrieval.type_neighbors`, 기본 off)으로 돌림. 기본 응답이 이전 binary와 같고 원문/신원/안전 시험이 유지됨을 확인.

근거: 원장 N-08의 `evidence_revision` / `manifest`.

## N-09: 무답·기권(미지정 사실)

상태: 완료.
- [x] N09-A 무답의 검색 근거 상태와 downstream 보류 계약을 구현·시험. `declared_unknown` 상태에서는 근거 줄만 인용하고, 혼합 인용과 상태 밖 레코드는 거부한다.
- [x] N09-B 승인된 무답/답가능 대조 사례에서 guard를 검증. 무답 knowledge on은 240/240행이 `declared_unknown`이고 근거 외 인용이 0이다. 대조·DEV의 잘못된 기권은 0/600이다.
- [x] N09-C 사람 기권과 기계 무인용을 따로 집계. 생성 답변 평가가 없으므로 답변 수준 기권은 null이고, 소표본은 inconclusive로 기록.

근거: 원장 N-09의 `evidence_revision` / `manifest`.

## N-10: 비용·지연과 비반환 진단

상태: 완료.
- [x] N10-A N11의 사전 검토·동결한 새 입력/source/model/runtime에서 같은8arm·회전·직렬 warm/cold 비용을 측정.
- [x] N10-B 실제K/MID/요청·원문/반환 크기·논리/HTTP 시도와 성공/오류/incomplete 분모를 보존하며 비용·원인 분해.
- [x] N10-C 소스/근거/프로젝트·모델 신원·안전·검색/기권 회귀를 유지한 범위 내 개선과 p95비1.25 수용 판정; 소표본은inconclusive.
- [x] N10-D 현행 소스/입력/원자료·제한과 검토자/비용 기록을 연결하며 과거 unbound 오류의 원인/현재 품질로 소급 단정하지 않음.

근거: 원장 N-10의 `evidence_revision` / `manifest`.

## N-11: 새 평가 사전 동결

상태: 완료.
- [x] N11-A 새 DEV/독립 FINAL과 원문 gold·출처·독립 단위/cluster 및 기존 FINAL 제외를 검토 가능하게 작성.
- [x] N11-B 모델·팩/ontology arm·회전·분모·임계치·비용·표본 조건의 사전 검토 기록과 입력 동결.
- [x] N11-C 실행 전 source/model/입력·평가 단위 검증 및 중요군 부족/관측·변경 시 inconclusive/회전/disabled 보호.
- [x] N11-D 현재 코드/입력/실행 증거에 바인딩하고 N-08/09/10에서 기존 FINAL 튜닝/부풀림 없이 사용.

근거: 원장 N-11의 `evidence_revision` / `manifest`.

## N-12: 실제 파일럿 내용·후보 관계 검토

상태: 완료.
- [x] N12-A 실제 원문20개/3명세/4기준 및 정책·ADR6주장/12제안 관계의 출처·권위·범위·시점·후보 관계에 대해 사용자가 AI 사전검토 65건을 일괄 채택하고 검토자 라벨을 지정했다. 독립적인 사람의 항목별 내용 검토는 없었다.
- [x] N12-B 승인된 내용/관계를 새 coordinated capture·lock·snapshot/CKG canonical·semantic projection에 결합, 누락/불일치/관계 오용과 자동 승격0을 재검증.
- [x] N12-C 추출 오차/관계 오용의 실제 검토 결과와 실제 활동 시간/비용 기록; 미측정은 null/미측정으로 표시하고 대기 시간으로 대체하지 않음.
- [x] N12-D 현행 소스/입력/원자료/검토 판정 바인딩과 다음 N14 실제 의미 수용 계획 연결; 테스트 pass/linked/verified/accepted를 분리.

근거: 원장 N-12의 `evidence_revision` / `manifest`.

## N-13: worksheet의 선택형 조직 catalog

상태: 완료.
- [x] N13-A 무팩 공통 worksheet에서 조직 hardcode/잘못된 참조 RED→GREEN, code/data 분리.
- [x] N13-B 선택 pack의 version/digest·의존/타입·출처 검증과 변조/누락/경로/입력 실패 보호.
- [x] N13-C 기존 StableNet8항목·필터/앵커/빈 queue·무팩/구팩 호환 및 명시 선택 metadata.
- [x] N13-D 실제 CLI·관련 회귀/경계/문서·소스/선택 자료/원자료 바인딩, 승인/검토 상태 자동 승격0.

근거: 원장 N-13의 `evidence_revision` / `manifest`.

## N-14: 실제 변경과 의미 수용의 종단 간 흐름

상태: 완료.
- [x] N14-A 실제 ADR→요구→기준→code/test를 검토된 source/권위/범위 및 coordinated lock/snapshot·CKG canonical에 결합.
- [x] N14-B 실제 baseline/result 소스 변경을 hold-for-review 후보와 immutable patch attempt/변경 archive SHA에 결합하며 current 보존.
- [x] N14-C 검토된 trace의 실제 exact test/명령·출력SHA·spec version·snapshot-bound run을 기록하고 무관test/command-only/stale/불일치 거부.
- [x] N14-D 별도 실제 사람이 기준별 Given/When/Then을 승인/반려하고 이유·검토자·원문/시각·활동시간(미측정null)을 기록.
- [x] N14-E 실제 재색인/승격·반려·rollback·원문 신원 재생과 전체근거를 결합하며 pass/linked/verified/accepted 및 제한을 구분.

근거: 원장 N-14의 `evidence_revision` / `manifest`.

## N-15: 현행 패키지·외부 모듈·native 환경 검증

상태: 완료.
- [x] N15-A 현재 소스 commit·Go/native dependency·패키지/바이너리/고지 SHA와 clean build receipt 결합.
- [x] N15-B 추출 패키지의 별도 프로젝트 설치/재시작/갱신/rollback 및 외부 Go 모듈의 v1/v2 공개 계약·입력 거부 확인.
- [x] N15-C 실제 native Linux AMD64 호스트의 설치/갱신/복구 및 native dependency 실행 증거. ARM 위 에뮬레이션과 구별.
- [x] N15-D 실제 모델·규모·외부 운영 소비자 설치/upgrade와 지원·비용·제한 수용 판정.
- [x] N15-E 현재 소스/원자료/수용 판정·한계를 문서/원장에 결합. local mock 통과로 전체 지원 또는 품질 합격을 주장하지 않음.

근거: 원장 N-15의 `evidence_revision` / `manifest`.

## N-16: 공개키·신뢰·권한

상태: 미충족·사용자 결정으로 보류.

재개 시 실제 공개키 fingerprint·권한자·신뢰경로·서명 대상·보관/교체/폐기·고지 범위와 근거를 기록한다. 개인키 본문은 저장하지 않는다. 보류를 완료로 바꾸지 않는다.

## N-17: 복구 루트의 reader 보호와 실제 운영 훈련

상태: 완료.
- [x] N17-A 현행 패키지의 실패 후보/current 보존·손상 대상 거부와 원본 payload SHA 기록.
- [x] N17-B 새 루트 백업 복원·reader lease/기존 inode 보존·원본 source 없는 인용 재생; 실제 public CLI/MCP 검증.
- [x] N17-C 실제 backup/failover 담당·RTO/RPO·키 복구·network/전원 정책 및 대상 호스트 판정.
- [x] N17-D 실제 운영 후보의 복구·실패전환 훈련과 조건별 시간/데이터 손실·사람 수용 증거.
- [x] N17-E 코드/원자료/한계·검토 판정 바인딩과 N18 연결. owned mock restore 시간을 실제 RTO/RPO 판정으로 사용하지 않음.

근거: 원장 N-17의 `evidence_revision` / `manifest`.

## N-18: 기본값·출시 판정

상태: 완료.
- [x] N18-A 독립 평가(N11·FINAL2), 비용(N10), 원래 회귀(당시 1개 간헐 실패 기록, 이후 `396242f0`에서 수정)로 기능별 기본값을 판정. 합격 근거가 없는 선택 기능은 disabled 또는 opt-in으로 유지.
- [x] N18-B 운영 결정(N15·N17 로컬 개인 운영, N16 미충족)으로 출시 범위를 판정. 로컬 운영은 허용하고, `main` 머지는 2026-10-12 별도 사용자 지시로 완료했다. 태그·공개 배포는 보류한다.
- [x] N18-C 판정 문서, 회귀 결과, 한계를 원장에 결합. 머지 권한은 2026-10-12 별도로 받아 실행했고, 태그·공개 배포 권한은 부여되지 않았다.

근거: 원장 N-18의 `evidence_revision` / `manifest`.

## O-01: MCP 온톨로지 준비 상태와 사용 가능 이유 노출

상태: 완료.
- [x] O01-A 실제 MCP 등록/input/output schema 및 CLI startup mode/budget 전달.
- [x] O01-B off/미설정/store·dataset·projection 누락/invalid mode/stale/예산 및 ready 구분.
- [x] O01-C ready에 same dataset/snapshot/source·실제 validator를 통과한 좌표와 검토/전체 개수만 제공; readiness는 품질/criterion acceptance가 아님.
- [x] O01-D 자료 없음에서 DB 생성0, source/store/pointer 불변, source/data 오류에 raw path 누출0, 읽기 전용 및 취소 제어.
- [x] O01-E 의미 있는 기존·새 시험/회귀·도구 schema·SHA·한계와 현재 소스를 커밋 원장에 결합.

근거: 원장 O-01의 `evidence_revision` / `manifest`.

## O-02: O01 기반 개념·용어·관계·요구 추적 읽기 MCP

상태: 완료.
- [x] O02-A tools/list·input/output schema, lookup/get/trace 계약과 invalid/missing 인수 거부.
- [x] O02-B 정확 용어·모호성·검토 상태/관계 방향·원문 evidence·same snapshot 및 비검토 관계 과장0.
- [x] O02-C requirement 기준/trace state·미연결/충돌·상한/truncated, 명령 실행/자동 의미 승인0.
- [x] O02-D O01의 missing/stale/read-only/권한·deadline 보호를 공유; raw 오류/잘못된 원문·다른 dataset 혼입0.
- [x] O02-E 실제 공개 handler/등록·원자료 및 current/committed source SHA와 문서/진행 결합.

근거: 원장 O-02의 `evidence_revision` / `manifest`.

## O-03: O02 기반 프로젝트 ontology/spec/store/mode 설치 연결

상태: 완료.
- [x] O03-A gen-config/setup의 explicit semantic store/mode/budget·ontology/spec 준비 옵션, off 기본/잘못된 조합 거부 및 기존 경로 호환.
- [x] O03-B 준비·검토·연결·activation 단계를 구분, 대상 dataset와 source 입력을 결합하고 projection 검증 없이 ready/activate0.
- [x] O03-C controlled 설치→MCP status/조회 positive 및 missing/stale 실패 제어; 실제 도메인 채택/리뷰 기록은 명확한 사용자 판정 필요.
- [x] O03-D 운영 설정/새 인덱스/선택 팩과의 연계·재색인 시 stale 전환·기존 current 보존, 실제 대상 원문·한계와 source SHA 기록.

근거: 원장 O-03의 `evidence_revision` / `manifest`.

## O-04: 온톨로지 영향 설명과 적용 전후 대조

상태: 완료.
- [x] O04-A query에 매칭 개념·사용한 검토 관계/원문과 후보 점수·rank/body 변화의 bounded 설명; off/no_match/active-but-zero 구분.
- [x] O04-B 같은 dataset/model/질문·off/on의 인용·본문·순위·상태/latency 비교 및 실패 분모/모델 호출, 기존 FINAL 새 독립 표본 재사용0.
- [x] O04-C controlled 기능 설명과 실제 도메인 효과 판정을 분리, 실제 도메인 자료/독립 평가 조건·부족 표본/활동시간·quality 한계 기록.
- [x] O04-D 구현/시험/평가 원자료·사전 조건/판정·current/committed SHA와 설명 문서·진행 원장 결합; 효과 미확인 상태를 품질 성공으로 체크하지 않음.

근거: 원장 O-04의 `evidence_revision` / `manifest`.

작업 완료는 실제 기능/실측·독립 자격 부족과 한계를 판정한 범위다. 승인 독립단위0·quality inconclusive·성능수용false·기본off는 유지한다.
