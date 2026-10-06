# C1 운영 신뢰·지원·복구 검토표

2026-10-04 · **검토 대기**. [작업리스트](./EXECUTION-WORKLIST.md)의 C1-04–07 종료 조건을 위한 결정 자료다. 시험 성공이나 이 문서 작성으로 운영 키·지원·출시를 승인하지 않는다.

## 현재 확인한 계약

[signed sidecar 수집기](../../scripts/release-sidecar.py)는 host-preview만 받아 scope가 test-signed-preview인 Ed25519 sidecar를 생성한다. key_id는 공개 키 DER의 SHA-256이다. PEM 파일 전체 해시와 혼동하지 않는다. [독립 검증기](../../scripts/verify-release.py)는 별도로 신뢰한 공개 키를 입력받아 해제 전에 서명·대상·archive와 내부 metadata/바이너리/고지 해시·tar 구조를 검사한다. [설치 후 Go 검증기](../../cmd/cks/packagecli/verify.go)는 같은 시험 scope와 계약을 검사한다.

현재 계약에는 유효 기간·온라인 폐기 조회·운영 scope가 없다. 이 검증만으로 키 보유자의 운영 권한, 폐기되지 않은 키, 현재 출시 승인까지 증명하지 않는다. 시험 공개 키와 그 fingerprint는 운영 신뢰 루트가 아니다. 운영용 형식이 필요하면 scope/신뢰 정책을 먼저 결정하고 계약·검증기·호환 시험에 반영해야 한다.

## 사람이 결정할 항목

| ID | 결정 | 현재 증거·상태 | 완료 기록 |
|---|---|---|---|
| OP-01 | 출시/서명/복구 소유자와 판정 권한 | 미지정. 정적 gold의 chat-user 승인은 운영 역할 승인과 별개 | 역할별 실제 식별자·책임·승인 근거 |
| OP-02 | 시험 preview와 운영 배포의 범위·형식 | 현재 도구는 test-signed-preview만 지원 | 사용할 배포 범위·schema·지원 대상과 승인 |
| OP-03 | 운영 공개 키·독립 배포 경로 | 실제 운영 키·fingerprint 없음 | 공개 키 DER SHA-256·독립 신뢰 경로·검토자·시각 |
| OP-04 | 개인 키 보관·접근·백업 | 실제 방식·보유자 미확인. 시험 개인 키는 임시 경로에만 있음 | 선택 방식·권한자·접근/복구 증거; 개인 키 본문은 기록하지 않음 |
| OP-05 | 교체·폐기·유출 대응 | 현재 검증기에 기간/폐기 조회 없음 | 효력 시점·구 키/서명 처리·신뢰 루트 교체·복구 담당과 실행 증거 |
| OP-06 | 소스·고지 적합성 | 원문/SHA·지정 source 비교 및 FIX-15 translated SQLite 추가 고지·modernc 선택 source ZIP/h1 검증 범위 존재; [원래 C/헤더 출처 감사](./C1-ORIGINAL-NATIVE-ORIGIN-AUDIT.md)의 SQLite 공식 SHA3·Linux 헤더436개 분류/생성4 재생 자료 추가; Darwin SDK/전체 생성기·적합성 pending | 정확한 버전·source/고지 범위·적합성 판정자·예외 및 잔여 범위 |
| OP-07 | 지원 플랫폼·실모델·비용 한계 | 아래 매트릭스의 진단 범위만 확인 | 실제 지원 대상·필수 환경·운영 비용/모델 실행·known limits |
| OP-08 | 실패 시 이전 데이터/current/키 복구 | 최신 Darwin preview의 실패/손상 거부·새 루트 백업 복원·원본 소스 없는 재생·구 데이터 SHA/재색인/구 v1 rollback 검증 및 Linux arm64 실모델 설치의 이전 version/인용 복구 통과; 최종 운영 복구 미실행 | 최종 패키지의 원본 보존·실패 주입·이전 current/신원/인용 재생과 담당자 |

각 항목은 승인/수정/보류와 판정자를 기록한다. 해당 항목의 근거 버전·해시·시각을 연결하며, 과거 승인을 다른 scope의 승인으로 확장하지 않는다. OP-04에 개인 키나 접근 비밀을 제출할 필요는 없다.

## 지원 매트릭스의 현재 범위

| 대상 | 실제 확인 | 남은 범위 |
|---|---|---|
| macOS arm64/M2 Max | Go 1.26.8 preview·시험 서명/설치/복구 이력, BGE-M3 pinned 소형/전체 입력 진단, FIX-14에서 실제 세 바이너리 dependency metadata 유지 확인 | 승인된 공식 B0/B1/C0/C1 품질·통제된 비용·최종 운영 패키지/키/복구 |
| Linux arm64/Docker VM | Go 1.25.13 source fixture의 paired package SHA 동일·시험 서명·Go 없는 Debian 런타임의 세 독립 mock 프로젝트 및 실제 CPU BGE-M3 설치/재시작/업데이트/롤백·72 v2 응답/96 인용 | 기존 20초 v1 probe 시작 실패·별도 90초 client 성공의 제한; 운영 호스트/대규모 입력 비용·최종 공식 품질/키/복구 |
| Linux amd64/arm64 위 에뮬레이션 | 동일 paired package/서명/mock 설치·복구 계약 | 네이티브 amd64와 실제 모델·운영 비용/최종 품질/키/복구 |

Linux source commit은 동결된 추적 입력으로 만든 합성 fixture commit이다. 실제 원본 release commit에서 빌드한 운영 후보라는 주장은 하지 않는다. 서로 다른 플랫폼/Go 도구 체인의 archive SHA가 같다는 판정도 아니다. Linux 최소 런타임은 Go 분석기·개발 header가 없어도 문서/TypeScript mock 스모크를 수행하지만 Go 소스 분석은 별도 Go 도구 체인을 요구한다.

## 복구 확인과 출시 판정의 연결

[설치 시범 절차](./INSTALLATION-PILOT.md)의 새 version/rollback은 이전 version을 보존하고 current를 되돌린다. 작업 트리 HEAD가 새 커밋이면 doctor의 commit과 indexed_commit이 다른 것이 정상이며, 복구 후 인용은 indexed_commit에 결합해야 한다. 소형 시험은 이 차이·이전 snapshot/version·본문 인용을 확인했다. 이 증거를 대규모 운영 데이터/구 schema 복구에 자동 적용하지 않는다.

최종 C1-06에서는 실제 후보의 이전 데이터 원본과 current/신원/팩 잠금을 보존하고, 실패 시 이전 current와 인용을 복구한 원자료를 연결한다. 운영 신뢰 루트 교체/폐기와 데이터 rollback은 각각 판정한다. C1-07은 B0 입력/프로토콜/의미 사실의 사람 결정, 공식 B0/B1/C0/C1 품질·안전·지연·소표본 판정, 위 OP 항목의 실제 기록을 함께 확인한다. 현재 출시 verdict는 pending이고 ontology 기본값 활성화 근거도 미완료다.

원자료와 정확한 검증 범위는 [C1 수집·검토 자료](./C1-REVIEW-PREPARATION.md), [최신 작업리스트](./EXECUTION-WORKLIST.md)를 따른다.

2026-10-04 Linux 실모델 후속: [작업리스트 17절](./EXECUTION-WORKLIST.md)의 공식 Ollama image/승인 모델 bytes·CPU residency·read-only model store·별도 caller/model quota를 기록했다. 실제 source/SDK/rollback 검증은 통과했으나 unavailable 팩/관계 폴백의 소형 합성 진단이며 운영 지원/품질 승인이 아니다. 원래 20초 initialize 실패와 90초 진단 client 성공을 함께 읽는다. 운영 역할·키·정책·출시 OP 결정은 그대로 대기다.

2026-10-04 FIX-15 후속: [작업리스트 18절](./EXECUTION-WORKLIST.md)의 최신 세 플랫폼 preview는 추가 SQLite 고지/두 검증기/세 프로젝트 mock 설치·재서명 고지 변조 거부를 확인했다. 실제 모델 검증을 새 패키지에 자동 확장하지 않으며, original C/header/전이 native의 적합성·OP-06과 최종 OP-07/08은 계속 대기다.


2026-10-04 C1-06 후속: [복구 검증](./C1-RECOVERY-VALIDATION.md)의 최신 Darwin 시험 패키지에서 실패 후보 보존·손상 rollback 거부·새 루트 백업 복원·구 데이터 원본 SHA/재색인/구 v1 재생을 실행했다. SDK/source/integrity 독립 감사와 빈 WAL/SHM 별도 원장이 있다. 소형 mock/합성 HTTP 진단이므로 최종 운영 backup/서비스 전환·담당자·OP-08 승인과 전체 품질은 계속 대기다.

2026-10-06 후속 기술 자료: [고정 SQLite·musl 원래 출처](./C1-ORIGINAL-NATIVE-ORIGIN-AUDIT.md)는 원래 C 신원·공식 SHA3와 두 Linux header tree의 원문430/생성4/overlay2를 연결한다. 전체 original C→Go 재생성·Darwin SDK·전이 native/법무 적합성은 판정하지 않았으며 OP-06 실제 검토는 계속 필요하다. 운영 키·검토자 권한·출시 상태를 변경하지 않았다.

## 2026-10-06 최종 후보 기술 검증 후 남은 결정

[최종 package](./C1-FINAL-PACKAGE-VALIDATION.md)의 세 preview·최신 실제 BGE 두 플랫폼·전체 strict1598/13820 비용60+RSS2·최종 구 v1/current 복구와 source/public/backend 감사를 완료했다. C1-04/06 기술 범위 완료, OP 승인이나 실제운영 지원은 미완료다. latest Linux 실제 모델은4CPU/4GiB 서버·1CPU/512MiB caller·180초 deadline이며 과거 실행 수/옵션을 새 후보에 자동 적용하지 않는다. nativeAMD64는 emulation만 확인했다.

[최종 종료 검토](./C1-FINAL-DELIVERY-REVIEW.md)에 품질fail·소그룹inconclusive·disabled·운영출시보류를 명시했다. 기존 요청의 최종 scope 결정은 아직 대기다. 시험 preview를 deliverable로 한정하면 실제 운영 키/역할/신뢰/전체 적합성·backup/failover 및 미제공 환경을 명시적 잔여로 기록해야 한다. 실제 운영 출시가 필요하면 위 OP01–08 실제 결정/증거가 필요하다. 입력①–⑤는 이 결정의 승인 기록이 아니다.

2026-10-06 최신 보고 종료 재검토: 비용 B1-06과 명시적 잔여/지원·출시보류 C1-07 보고는 원래 조건대로 완료했다. 실제운영scope/OP01–08·역할/키/적합성의 C1-05는 미완료다. 전체26/30, 남은전체B1-04·07·08/C1-05이며 일반 다음 지시를 실제운영 승인의 기록으로 바꾸지 않는다.
