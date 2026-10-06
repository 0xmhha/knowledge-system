# 후속 리팩토링 실행 명세와 완료 판정

2026-10-06 · 개발 시작 코드 `c9c91ebe` · [전체 작업·우선순위](./REFACTORING-REMAINING-WORKLIST.md) · [원래 요구사항/WBS 대응](./REFACTORING-ORIGIN-AND-V2-TRACE.md)

## 목적과 유지할 계약

원문에서 정확한 근거를 찾고 의미·요구·코드·시험의 연결을 검토 가능하게 제공한다. 색인 갱신이 현재 근거를 파괴하면 이 목적을 충족할 수 없다. 따라서 N-02–04로 데이터 보존을 먼저 확보하고, 별도 DEV에서 예산·검색·기권·비용을 개선한다. 실제 사람 판정과 운영 사실은 구현 시험으로 대신하지 않는다. 기존 FINAL과 preview 종료 기록은 당시 코드의 역사적 결과로 보존한다.

각 N 항목의 목적·선행·전체 수용 조건은 작업리스트의 해당 행이 기준이다. 이 문서는 구현 전에 그 조건을 세부 설계/시험으로 분해하는 실행 명세다. 조건을 줄여서 완료율을 올리지 않는다. 새 조건이 발견되면 먼저 문서와 시험 범위를 보완한다.

## 완료 판정 절차

상태는 **미착수 → 설계 → 실패 재현 → 구현 → 검증 → 완료**로 기록한다. 구현 이후 시험이 실패하면 구현 상태로 되돌린다. 일부 세부 조건만 통과하면 부분 진행으로 남긴다. 대기/조건부/보류는 완료가 아니다.

완료에는 다음 증거가 모두 필요하다.

1. 원래 목적/FR·INV·NFR·WBS 또는 v2 계약과 연결한 설계 및 필요한 세부 기능.
2. 변경 전 실패 재현(새 기능은 부족한 행동의 시험), 변경 후 같은 시험의 통과.
3. 공개 진입점·성공·실패·취소·호환성 등 해당 작업의 모든 수용 조건별 시험 결과. 코드 존재만으로 판정하지 않는다.
4. 변경 코드/시험의 SHA, 실행 명령·exit code·출력과 한계. 모델/실환경/사람 검토가 필요하면 그 실제 증거.
5. 작업리스트/이 명세/핸드오프의 현재 단계·다음 작업·전체 잔여 동기화. 완료 조건에 미해결 항목이 있으면 체크하지 않는다.

항목 완료마다 위 증거를 기록하고 문서를 갱신한다. 코드/입력 변경은 새 DEV 증거에 바인딩한다. 이전 FINAL 원자료나 그 소스 동결을 새 코드의 검증 결과로 재해석하지 않는다. 커밋·배포와 기능 완료는 별도다.

## N-02: 공개 유지보수 쓰기와 v2 불변 버전

**상태: 완료.** 원 INV-01/02/06/07·FR-10·NFR-05와 v2 A3/A8의 원문·좌표·신원 보존과 versioned candidate→gate→promotion 계약을 보호한다.

### 설계

- `ops.index`의 기존 graph/vector 직접 갱신은 명시적인 legacy 평면 데이터셋에서만 유지한다. 버전 디렉터리, `current` 별칭, pinned manifest, 보관 원문 또는 dataset identity를 발견하면 **도메인 export와 첫 엔진 실행 이전**에 거부한다. 손상된 신원을 legacy로 취급하지 않는다.
- `ops.setup`/`ops.reindex`의 현재 MCP 입력에는 project/source/설정/팩/모델 신원 전체가 없다. 이 입력으로 v2를 유지보수할 수 있다고 안내하지 않는다. 기존 pinned 버전의 재빌드나 legacy 후보로의 downgrade도 거부한다.
- v2 갱신은 기존 `cks setup --config … --version <새 버전> --project-id …` 경로를 사용한다. 같은 설정/선택 팩/원문·모델 pin으로 새 후보를 만들고, shared Reindex의 gate 후 승격하거나 `--hold-for-review`로 보류한다. 새 MCP v2 builder API를 임의 추가하지 않는다.
- 버전 이름은 기존 디렉터리를 덮어쓰지 않는다. BuildPlan/Execute에서도 기존 불변 데이터셋을 거부하여 비동기 job 시작 전 검사만 의존하지 않는다. 동시 writer/검사와 쓰기 사이 경합은 N-03의 OS 잠금 계약으로 추가 해결한다.
- low-level `ckg/ckv`의 임의 직접 실행은 관리 도구의 보호 밖이다. 운영자는 공개 유지보수 경로를 사용하고 MCP 서버는 현재 버전을 계속 pin한다. 승격 후 재시작/재연결 조건을 유지한다.

### 수용 조건별 체크

- [x] N02-A 공개 MCP JSON-RPC `ops.index`: full/incremental, v2/버전 별칭/손상된 pin 거부, 엔진 호출·corpus 쓰기 0, 기존 current/identity/보관 원문 불변.
- [x] N02-B 공개 `ops.setup`/`ops.reindex` 우회 차단. 공유 builder도 기존 버전 재사용·v2→legacy downgrade 거부.
- [x] N02-C versioned setup과 공유 Reindex: 성공 시 완성된 신원/원문 후보만 승격; 한 엔진 실패·혼입·취소 시 current/기존 신원/원문 보존.
- [x] N02-D legacy 평면 갱신·새 legacy blue-green 후보 호환. 도구 설명과 이 문서에 migration 경계 반영.
- [x] N02-E 관련 전체 패키지/공개 스키마 회귀, 코드 SHA와 실행 원자료 연결. 미해결 제한은 명시.

### 검증 결과와 한계

[새 DEV 증거](../../system/eval/b0-knowledge-system/refactoring-n02-2026-10-06/cli-results.json): 변경 전 공개 JSON-RPC의 10개 v2 갱신 조합과 비동기 setup 우회가 실패 재현됐다. 변경 후 등록된 공개 MCP 호출에서 10개 거부·4개 비동기 우회 거부·2개 legacy 정상 경로를 확인했다. shared builder의 부분 pin/별칭/계획 이후 pin/기존 rollback 버전 시험도 통과했다. 실제 새 CKS/CKG/CKV CLI로 초기·단일 엔진 실패·프로젝트 혼입·취소·기존 버전·검토 보류·승격 7개 사례를 실행했다. 실패마다 이전 current와 전체 보관 파일 SHA가 같고 보류/승격 후보의 원문·데이터셋 신원이 동일했다.

관련 5개 패키지 race·vet, 공개 입력/출력 스키마 및 경계/문서 검사를 통과했다. 로컬 포트를 쓰는 기존 httptest는 샌드박스에서 차단되어 확장 권한으로 재실행했다. 새 공개 legacy 시험의 인자 누락은 컴파일 단계에서 수정했다. 이 시험은 Darwin ARM64·mock 임베딩의 데이터 보존 검증이며 실제 BGE 품질·외부 daemon 설치·운영/전원 차단 시험은 아니다. 동시 writer와 fsync 한계는 별도 N-03/04에 남는다. 원 FINAL과 승인 입력5개는 변경하지 않았다. 코드·실행·문서 SHA는 이 증거 디렉터리의 manifest에 연결한다.

## N-03: 빌드·승격·rollback의 공통 OS 잠금

**상태: 완료.** INV-01/02와 A3/A8의 단일 writer·부분 후보 비활성 계약을 보호한다. PID/시각은 진단 정보이며 OS 잠금 소유권을 대신하지 않는다.

### 설계

- 빌드부터 gate/검토 보류/최종 포인터 전환까지 한 dataset의 OS 배타 잠금을 유지한다. 외부 승격/사람 검토 승격/rollback도 같은 잠금을 획득한다. Reindex의 내부 승격은 보유 중인 잠금 아래 실행하여 스스로 재획득하지 않는다.
- 살아 있는 writer의 파일 나이로 잠금을 회수하지 않는다. OS가 종료/crash 시 잠금을 해제한다. 잠금 파일은 해제할 때 unlink하지 않아, 기다리던 프로세스가 서로 다른 inode를 잠그는 경합을 만들지 않는다.
- symlink/비정규 잠금 파일을 거부한다. 기존 promotion lock의 보호도 유지한다. OS flock 보장 범위는 지원되는 로컬 파일시스템이며 임의 network filesystem 지원을 주장하지 않는다.
- 이전 O_EXCL/PID 방식의 기록은 살아 있는 소유자이면 나이와 무관하게 거부하고, 죽은 소유자의 기록은 공통 OS 잠금 아래 전환한다. PID 접근 권한 불확실성은 살아 있을 가능성으로 처리한다. 구/신 writer를 동시에 실행하는 업그레이드는 지원하지 않으며 구 writer를 종료한 뒤 전환한다.

### 수용 조건별 체크

- [x] N03-A 긴 live holder의 오래된 시각/동시 acquire에서 두 writer 없음; 잠금 inode 유지.
- [x] N03-B build holder와 Promote/Rollback/사람 검토 승격이 같은 lock으로 직렬화; 내부 Reindex 성공/보류 경로에는 재획득 오류 없음.
- [x] N03-C 별도 프로세스의 동시 요청/강제 종료 후 OS 자동 회수. 살아 있는 legacy PID는 오래됐어도 탈취하지 않음.
- [x] N03-D symlink/비정규 잠금 거부·legacy 전환 경계 문서·관련 회귀/race/vet와 소스/원자료 바인딩.

### N-03 검증 결과와 한계

[새 DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n03-2026-10-06/cli-results.json). 변경 전 live holder의 오래된 시각 탈취와 build 중 pointer 승격을 각각 재현했다. 수정 후 긴 holder·legacy live PID·동일 inode 재획득·linked/nonregular 파일 거부·build/승격/rollback/사람 검토 승격 충돌을 시험했다. 별도 OS 프로세스 holder에 8개 동시 contender가 모두 진입하지 못했고 SIGKILL 후 같은 inode로 회수했다. Reindex 내부 정상 승격/검토 보류의 재획득 오류가 없으며 실제 CLI의 다른 빌드/rollback 동시 요청도 실패하고 current와 기존 버전을 보존했다. N-02 CLI 7개 안전 사례를 새 CKS로 다시 실행했다. 관련 5패키지 race/vet·경계·문서 검사를 통과했다.

`.reindex.lock`는 OS 잠금 inode로 영구 유지한다. PID/시간/프로토콜 표시는 진단용이고 live holder의 나이로 삭제하지 않는다. 기존 `.promotion.lock`도 추가 보호로 유지한다. 오래된 writer를 모두 종료하고 새 프로토콜로 전환해야 한다. 구 writer와 신 writer의 동시 업그레이드, flock 의미가 불명확한 network filesystem, 실제 native Linux 실행은 이 증거 범위 밖이다. 이는 N-03의 명시적인 호환/플랫폼 경계이며 운영 환경 사실은 N-15/17에 남긴다. 전원 차단·후보/current fsync 순서는 N-04 미완료다.

## N-04: 후보 저장·current 전환·복구의 내구성

**상태: 완료.** INV-01/02/03·FR-09/10와 A3/A8의 이전 current 또는 완성된 새 후보만 관측하는 계약이다. 프로세스 종료 시험과 전원 차단 시험을 구분한다.

### 설계 범위와 누락 방지

1. 엔진의 DB checkpoint/close 성공을 확인한다. 기존 CKV Checkpoint는 `PRAGMA wal_checkpoint(TRUNCATE)`의 오류만 읽으므로 busy/잔여 WAL도 판정해야 한다. graph/vector/선택 semantic DB와 보관 원문·Git archive·manifest·config/팩 신원을 모두 대상으로 한다. 이미 활성화한 버전의 원문이나 identity를 내구성 작업으로 제자리 재작성하지 않는다.
2. 빌드/테스트/gate가 만든 모든 후보 파일을 sync하고 하위 디렉터리부터 후보/부모까지 sync한다. 마지막 검증 이후 후보가 바뀌지 않게 N-03 잠금을 유지한다. 검토 보류 기록도 후보 내구성 범위에 포함한다. symlink와 비정규 산출물·실패한 sync는 승격하지 않는다.
3. 상대 임시 symlink 생성→포인터 rename→dataset 부모 sync의 순서를 고정한다. rename 전에 실패하면 이전 current를 보존하고, rename 뒤 부모 sync 실패는 **미승격으로 단정하지 않는 별도 상태**로 기록한다. 실패 보고를 이유로 무검증 자동 rollback하지 않는다. 재시작은 current/대상 신원·원문·완료 기록을 재검사한다.
4. 일반 승격뿐 아니라 rollback와 사람 검토 승격도 같은 순서를 적용한다. 현재 사람 검토 경로의 `review-release.json`은 pointer 전환 뒤 쓰인다. 이 사이 중단과 기록 쓰기 실패를 복구 설계에 포함하며, 실제로 승격되지 않은 후보에 재사용 가능한 release 승인을 남기지 않는다. 승인 증거/intent와 실제 포인터 전환 완료를 구분하는 설계를 확정한 뒤 구현한다.
5. 로컬 Darwin/Linux의 파일·디렉터리 sync 의미와 지원/오류 처리를 기록한다. 실제 전원 차단/스토리지 손실은 프로세스 SIGKILL만으로 증명하지 않는다. 미실측 플랫폼은 제한/미확인으로 유지한다.

### 검토 승격의 중단 복구 설계 확정

검토 승인 intent를 후보에 먼저 sync하되 rollback 권한을 주는 release와 구분한다. 포인터 전환 뒤 release를 sync한다. 같은 승인/기준 base로 재시도할 때 current가 이미 이 후보이고 intent·신원이 일치하면 부모 sync와 release 기록을 복구한다. 다른 patch/결정/프로젝트/스냅샷/base 또는 이미 다른 current로 이동한 상태에서 기존 승인을 추측해 복구하지 않는다. rename 뒤 sync/release 실패는 `durability_uncertain`으로 보고하여 거짓 미승격을 피한다. 실제 사람 승인의 검증은 기존 patch workflow에 유지한다.

### 실패 주입 단계와 수용 체크

| 단계 | 관측/수용 조건 | 상태 |
|---|---|---|
| 엔진 checkpoint/close, 후보 파일 sync 이전·도중 | 실패 후보 비활성·이전 current/신원/보관 원문 유지 | 통과 |
| 후보 디렉터리/부모 sync, 마지막 gate | 이전 또는 완성 후보; 재시작 시 미완성 후보 승격 없음 | 통과 |
| 임시 symlink·rename 직전 | 이전 current 유지; 임시 파일은 결정적으로 정리/무시 | 통과 |
| rename 뒤·dataset parent sync 실패 | 현재 관측 대상과 durability 불확실성을 구분; 거짓 미승격/자동 rollback 없음 | 통과 |
| 검토 hold/release 기록과 reviewed promotion 사이 | 사람 승인 intent·실제 전환 분리, 실패로 승인 우회·허위 rollback 허용 없음 | 통과 |
| 성공·rollback·재시작/강제 종료 | 이전 또는 신원/DB/원문이 완성된 새 버전만 제공 | 통과 |

- [x] N04-A DB/WAL/close 및 전체 후보 artifact 내구성 순서.
- [x] N04-B current 전환·parent sync와 단계별 실패 주입, 결과 상태 구분.
- [x] N04-C 일반/검토 승격·hold/release·rollback와 재시작/강제 종료 복구.
- [x] N04-D 플랫폼별 보장/전원 차단 미실측 한계 및 회귀·원자료·소스 바인딩.

### N-04 검증 결과와 한계

[새 DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n04-2026-10-06/manifest.json). 변경 전 uncheckpointed WAL 및 linked artifact 승격을 재현했다. 변경 후 DB checkpoint/close·파일·디렉터리·부모·rename 전후의 8개 실패 경계, 검토 intent/rename/parent/release의 5개 경계, 일반/검토 승격의 별도 process SIGKILL 7개 경계에서 이전 또는 신원/보관 원문이 완성된 후보만 관측했다. busy PRAGMA row와 확장자 없는 semantic DB, hardlink DB, vector physical pin 변경, sidecar close 이후 소멸, 검토 hold 완료 이벤트 억제를 검증했다. 동일 승인/기준 base만 재시도하여 release를 복구하며 다른 승인과 일반 rollback의 hold 우회를 거부했다.

실제 CKS/CKG/CKV CLI의 7안전 사례 및 build/rollback 동시 요청 2개를 최종 바이너리로 다시 검증했다. 실제 CLI에서 발견한 sidecar 목록의 소멸 순서 오류를 내부 진단으로 확인해 수정했으며 실패 원자료도 보존한다. 관련 9패키지 race, 추가 close 경계의 5패키지 race, vet·경계·문서 검사를 통과했다. 첫 회귀 명령의 잘못된 패키지 경로와 sandbox 로컬 port 제약은 수정/확장 실행한 결과와 구분한다.

현재 구현은 로컬 파일·디렉터리 fsync 오류를 전파한다. rename 후 parent sync/release 실패는 `durability_uncertain`이며 이전 current 보존이라고 거짓 보고하거나 자동 rollback하지 않는다. Darwin ARM64 로컬 파일시스템에서 실행했다. native Linux·스토리지 컨트롤러/전원 차단·network filesystem의 내구성 실측은 이 결과로 주장하지 않는다. fsync가 장치/파일시스템의 전원 장애 보장을 대체하지 않으며 실제 지원/복구 환경은 N-15/17에서 판정한다. 구/신 writer 동시 운영 금지와 low-level 임의 쓰기의 관리 보호 밖 경계를 유지한다. 기존 승인 입력5개·FINAL 원자료는 그대로다.

### 완료 표시의 자동 검사

[진행 원장](./REFACTORING-PROGRESS.json)을 갱신하고 완료 기록 전 `make refactoring-check`를 반드시 실행한다. 직접 명령은 `python3 scripts/check-refactoring-progress.py`다. 완료 상태는 커밋에 실제 포함된 원자료/문서/변경 코드 SHA 및 당시 완료 명세의 전체 수용 체크를 검증한다. 최신 소스/의존 파일과 승인 입력의 바인딩, 새 Go 파일 누락, 완료표/잔여수/다음 작업의 불일치도 검사한다. 문서만 완료로 바꾸거나 미검증 코드를 추가하면 실패한다. 새 항목 완료 시 그 커밋·manifest·전체 체크와 최신 소스 바인딩을 함께 갱신한다. 정상 상태와 허위 N-04 완료/누락 원자료/옛 소스 바인딩/수용 조건 삭제 4개 음성 제어를 검증했다. [검사 원자료](../../system/eval/b0-knowledge-system/refactoring-progress-guard-2026-10-06/negative-controls.json). 기계 검사는 기록 일관성을 보장하며 목적 적합성·사람/운영 사실의 진실성을 대신하지 않는다. 기존 source inventory의 1099/1100 표시는 Go뿐 아니라 go.mod/go.sum 2개를 포함한 소스·의존 파일 수다.

## 다음 작업 설계의 준비 조건

N-04/05의 전체 수용 조건과 실행 증거를 확인했다. 다음 N-06은 아래 자원 상한/deadline 명세를 따라 실패 재현부터 진행한다. 설계 기록과 완료 표시를 구분한다.

N-05–18도 각각 시작 전에 위 절차로 목적·설계·모든 수용 조건을 세분화한다. 새 독립 FINAL(N-11), 실제 파일럿 사실(N-12/14), 운영(N-15–18)의 별도 판정 필요성을 유지한다.

## 진행 로그

| 시점 | 작업 | 단계/증거 | 완료 여부 |
|---|---|---|---|
| 2026-10-06 | N-01 | 원문 8개/FR10·INV7·NFR5·S9·WBS29 대응 및 SHA 검증, `c9c91ebe` | 완료(기존 기록) |
| 2026-10-06 | N-02 | 공개 MCP RED→GREEN, shared guard, 실제 CLI 7사례·SHA, race/vet/경계/문서 검증 | 완료 |
| 2026-10-06 | N-03 | RED→GREEN, 별도 process 8 contender·SIGKILL 회수, CLI build/rollback 차단, race/vet | 완료 |
| 2026-10-06 | N-04 | WAL·linked RED→GREEN, 8+5 실패 경계·7 SIGKILL 복구·CLI 7+2·9패키지 race | 완료 |
| 2026-10-06 | N-05 | GC API RED→GREEN·5 중단/취소 경계·2 process crash·공개 CLI 12사례와 live MCP pin·6패키지 race | 완료 |

**현재:** N-01–05 완료(5/18). **다음:** N-06 자원 상한/deadline 실패 재현과 구현. **전체 잔여:** N-06–18, 13개. 상위 WI10개는 별도 범위이며 [study 추적 문서](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md)에 유지한다.

## N-05: 보존 정책·검토 가능한 GC와 reader pin

**상태: 완료.** 원 INV-01/02/03·FR-10 및 v2 A3/A8의 원문·좌표·활성/rollback 보존 계약을 유지하면서 미참조 버전을 회수한다. 파일을 지우는 기능의 존재만으로 완료하지 않는다.

### 설계와 호환 경계

- GC 계획과 실행은 N-03 공통 dataset 잠금 아래 수행한다. 기본은 dry-run이며 버전별 크기·나이·보호 이유·보존 정책·회수 예정/회수 불가 용량을 공개한다. 실제 실행은 검토한 계획의 digest와 동일한 참조/신원/파일 상태에서만 진행한다. 경합/새 pin/포인터 이동/파일 변경은 stale plan으로 거부한다.
- current, 최근 rollback 보존 수, 명시적인 보호 버전, 미해제 review hold/intent, 살아 있는 reader의 shared OS lease를 보호한다. 서버가 실제 backend·보관 원문을 열기 전에 lease를 얻고 닫은 뒤 해제한다. lease inode는 dataset 관리 영역에 두어 불변 source/DB를 변경하지 않는다. reader 종료/crash 시 OS가 해제한다.
- 기존 서버는 lease를 기록하지 않는다. reader protocol 이전 후보는 자동 회수 대상으로 삼지 않는다. 새로운 builder가 새 후보에 프로토콜을 기록하며 구 서버를 중단 후 업그레이드하는 경계를 문서화한다. 레거시 reader가 없는 것을 PID/파일 나이로 추측하지 않는다.
- 버전 전체를 회수 단위로 삼아 source blobs/Git history/semantic source를 부분 삭제하지 않는다. 다른 보존 버전에 공유되는 hardlink/외부 symlink는 회수 가능한 후보로 분류하지 않는다. 보존 수/기간·용량은 공개 정책이며 보호 버전 때문에 상한을 못 맞추면 그대로 보고한다.
- 실행 중 삭제는 관리된 trash로 원자 이동·디렉터리 sync 후 수행한다. 중단된 trash는 기록된 계획/참조와 신원을 재검증하여 재개한다. 취소/프로세스 종료 중에도 보호 버전과 current를 바꾸지 않는다.

### 수용 조건

- [x] N05-A 결정적인 dry-run·정책/용량/보호 이유·검토한 계획 digest에만 결합된 실행.
- [x] N05-B current/rollback/reader/review/보관 원문·Git history·semantic source 보호 및 구 reader 호환 경계.
- [x] N05-C 경합·stale plan·취소·중단/trash 복구·실제 삭제 검증, 보호 참조 삭제0.
- [x] N05-D 공개 CLI/서버 pin·상태/오류 비노출·관련 회귀와 소스/실행 증거 바인딩.

### N-05 검증과 공개 경로

새 기능의 변경 전 시험은 GC/reader API 부재로 실패했다. 변경 후 결정적인 계획, 활성/실제 rollback/명시 보호/reader/legacy/review 보호와 capacity 초과 보고, reader/포인터/파일 변경의 stale plan 거부, 5개 trash/취소 경계, 별도 reader crash·부분 삭제 SIGKILL, foreign/unknown trash 및 손상된 current/release/미완료 intent의 fail-closed를 시험했다. noop 재승격이 실제 rollback 참조를 잃지 않는다. 미복구 trash 용량도 보고하고 복구 전 새 삭제를 거부한다.

공개 CLI는 `cks gc --out <dataset> --dry-run --plan-file <plan.json>`이 기본이며, `cks gc --out <dataset> --apply --plan-file <plan.json>`은 이 digest의 동일 참조/바이트 계획만 실행한다. `cks gc --out <dataset> --resume`은 검증한 journal만 재개한다. 기본 최근2개·최소30일, 명시적인 보호 버전과 logical file-byte 용량 목표를 보고하며 용량을 이유로 보호를 해제하지 않는다. current가 손상되면 다른 rollback 후보의 삭제를 진행하지 않는다.

실제 새 CKS/CKG/CKV로 5개 pinned 버전을 만들고 공개 stdio MCP의 health에서 양 backend·model·alignment/serviceable과 v1 pin을 확인했다. current는 v5인 채로, 새 reader 때문에 이전 계획이 거부됐으며 fresh 계획으로 v2/v3만 삭제하고 v1 reader가 계속 serviceable임을 확인했다. reader 종료 후 v1을 회수하고 v4 rollback/v5 current 및 모든 v5 파일 SHA를 보존했다. 6관련 패키지 race·vet/경계·공개 CLI 모드 검증을 수행했다. 첫 MCP 시험의 잘못된 logging.mode 값은 실제 허용값 prod로 수정했고 실패 원자료를 유지했다. [DEV 자료](../../system/eval/b0-knowledge-system/refactoring-n05-2026-10-06/cli-results.json).

lease는 `.readers/<version>.lock`의 OS shared flock이며 inode를 삭제하지 않는다. server는 backend/semantic/보관 원문을 열기 전에 lease를 얻고 모든 backend/임시 원문을 닫은 뒤 해제한다. 새 pinned builder만 reader protocol을 기록한다. 이전 서버/직접 low-level engine reader를 GC와 병행하지 않으며 모두 중단 후 새 managed CKS로 업그레이드한다. 이전 후보·미완성/검증 불가 버전은 자동 회수하지 않는다. 공유 hardlink/외부 symlink와 network filesystem은 허용하지 않는다. native Darwin ARM64·mock 모델 검증이며 native Linux/운영 소비자/장치 전원 장애 지원 판정은 N-15/17에 남는다.

## N-06: setup/config 자원 상한과 build deadline

**상태: 설계.** NFR-03 및 v2 캡처/빌드 자원 계약을 공개 입력·shared Reindex·진단에 일관되게 적용한다.

- setup YAML과 명시 CLI의 우선순위를 유지하며 최대100000파일·단일32MiB·총4GiB의 기본값과 설정값을 pre-build identity/실제 capture/최종 source 재검증에 동일하게 전달한다. 운영 상한 때문에 일부 원문을 성공 후보로 만드는 대신 후보 전체를 거부한다. negative/overflow/외부 pack 포함 합산의 경계를 검증한다.
- 기본 build2h deadline을 CLI 준비와 shared versioned Reindex에 적용한다. 캡처·staging·엔진/gate·후보 sync와 rename 직전에 취소/timeout을 확인한다. pointer rename 이후에는 필수 내구성 마무리를 완료해 거짓 unchanged/자동 rollback을 피한다. 일반 fsync의 kernel I/O는 cooperative deadline의 강제 중단으로 주장하지 않는다.
- 원문 inventory와 보관/staging 최소 필요 바이트, 출력 filesystem의 현재 free bytes, 엔진 출력 크기 미예측 한계를 공개 resource 상태에 기록한다. 여유 공간 부족은 빌드/승격 전에 거부하고 이전 current를 유지한다. resource/error DTO에는 절대 경로·원문 내용·비밀을 넣지 않는다.

- [ ] N06-A YAML/CLI/default/shared API의 상한·deadline 전달 및 입력 검증.
- [ ] N06-B 초과/timeout/취소·외부 pack 합산에서 부분 후보 비활성·이전current/identity/원문 불변.
- [ ] N06-C 예상/실제 free bytes·공개 resource 상태 및 오류/민감 경로 비노출.
- [ ] N06-D 관련 공개/호환 회귀와 수용 조건별 소스·실행 원자료 바인딩.
