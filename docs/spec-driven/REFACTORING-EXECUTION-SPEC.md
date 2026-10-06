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

## 다음 작업 설계의 준비 조건

N-03은 빌드/승격/rollback의 동일 OS 잠금, 긴 live holder/동시 요청/owner crash 및 legacy 경계 시험을 먼저 작성한다. N-04는 DB close/checkpoint·후보 파일/디렉터리 sync·포인터 rename/부모 sync와 실패 주입 단계표를 먼저 작성한다. 이 두 항목은 N-02의 구현만으로 완료되지 않는다.

N-05–18도 각각 시작 전에 위 절차로 목적·설계·모든 수용 조건을 세분화한다. 새 독립 FINAL(N-11), 실제 파일럿 사실(N-12/14), 운영(N-15–18)의 별도 판정 필요성을 유지한다.

## 진행 로그

| 시점 | 작업 | 단계/증거 | 완료 여부 |
|---|---|---|---|
| 2026-10-06 | N-01 | 원문 8개/FR10·INV7·NFR5·S9·WBS29 대응 및 SHA 검증, `c9c91ebe` | 완료(기존 기록) |
| 2026-10-06 | N-02 | 공개 MCP RED→GREEN, shared guard, 실제 CLI 7사례·SHA, race/vet/경계/문서 검증 | 완료 |
| 2026-10-06 | N-03 | RED→GREEN, 별도 process 8 contender·SIGKILL 회수, CLI build/rollback 차단, race/vet | 완료 |

**현재:** N-01/02/03 완료(3/18). **다음:** N-04 저장 내구성/전환 순서 설계와 단계별 실패 재현. **전체 잔여:** N-04–18, 15개. 상위 WI10개는 별도 범위이며 [study 추적 문서](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md)에 유지한다.
