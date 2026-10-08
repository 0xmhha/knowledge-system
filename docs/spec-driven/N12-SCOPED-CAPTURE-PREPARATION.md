# N12 명시 소스 범위의 보관 캡처 준비

2026-10-08 · **캡처·복원 준비 검증 완료 / N12 전체 미완료**

목적은 N12의 검토 원문과 코드 근거를 보관 소스에 결합할 수 있는지 확인하는 것이다. 원본 Git `667a9af5f8608e6e984f58f3c3b61350dde0ea48`에서 제안한 소스 입력 1,410개 파일(11,985,941 bytes, Go1,107개)을 선택했고, 실제 `CaptureSource(committed)`가 모두 보관했다. 기존 전체 저장소의 PEM 경로 거부는 유지한다. 실제 사람의 입력 범위·내용·관계 승인을 기록하지 않았다.

## 입력 범위와 출처

| 포함 | 목적 |
|---|---|
| `cmd/`, `internal/`, `pkg/` | CLI·구현·계약과 시험·fixture·embed/native 자산 |
| `third_party/`, `go.mod`, `go.sum` | native 의존 파일과 Go 모듈 신원 |
| `docs/spec-driven/ontology-pilot.yaml`, `spec-pilot.yaml` | 검토한 메타모델20·요구3·기준4의 원문 |
| `docs/spec-driven/PUBLIC-CONTRACT-V2.md`, `WORKSHEET-CATALOG-CONTRACT-V1.md`, `SOURCE-SNAPSHOT-ADR.md` | 정책·ADR6주장/12제안 관계의 원문 |

선택 기준 외의 경로는 [제외 원장](../../system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08/excluded-original-paths.json)에 Git 객체와 함께 기록했다. 평가·운영 산출물을 포함하지 않는 제안 기술 입력이며, 실제 operating corpus나 새 평가 입력의 승인으로 취급하지 않는다. 사용자 미추적 파일은 입력에 포함하지 않았다.

원본 객체의 바이트·Git 실행 비트를 복사한 독립 Git 저장소에 준비 커밋 `562e37aa1c9680d283de85ccd6435e5f235c21d6`을 만들었다. 이 새 커밋과 원본 커밋을 명시적으로 구별한다. 원본 전체 Git 역사를 복제하지 않았으며 보관된 역사는 준비 커밋1개다. 검토자나 사람의 작성 승인으로 이 준비 커밋의 author를 사용하지 않는다.

## 실제 세부 검증

- [x] 원본 선택 1,410개 파일의 경로·Git 객체·SHA·mode와 제외 목록 기록.
- [x] 기존 33개 검토 항목의 69개 원문/코드 참조를 새 입력과 archive 복원본에서 파일·span SHA까지 재대조.
- [x] snapshot-only 실제 캡처 결과1,378개, 제외32개 확인. build31개(기존 승인 Go29개 포함)와 nested vendor 시험 fixture1개가 기본 디렉터리 제외 정책에 해당함.
- [x] 독립 Git의 committed 캡처가 선택 파일1,410개 전체와 승인 Go29개 보관.
- [x] pre-build identity·실제 보관·반복 캡처·복원 파일/실행 비트·archive 재생 신원 일치. 복원 트리의 CKS/CKG/CKV `go list -deps -json` exit0, 의존479·로컬118패키지의 활성 build/embed508파일(7embed 포함) SHA 일치.
- [x] 실제 민감 경로·symlink·상한과 파일/실행 비트/blob/manifest 변조의8개 거부 제어, 원상 복구 후 정상 소스·blob·보관 재검증.

snapshot-only 제외를31개로 가정한 초기 driver assertion은 vendor fixture1개를 누락해 실패했다. 제품 캡처는 성공했고 driver의 대조 가정이 잘못됐으므로 별도 [준비 수정 기록](../../system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08/preparation-correction.json)에 보존했다. 최종 결과에 초기 실패를 시험 성공으로 합산하지 않았다. 캡처 정책·민감 경로 보호·제품 소스 변경0이다.

실제 snapshot ID는 `cef82547a2363de02283d1e82eb20d986df6e23c3bb7d5cbe0ad4d37285a0c74`, project ID는 `knowledge-system-n12-proposed-source`, source mode는 committed다. [소스 신원](../../system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08/capture-source-identity.json)과 [파일 원장](../../system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08/source-input-inventory.json)을 보관한다.

실제 manifest·blob·준비 커밋 bundle을 포함한 [retained-source.tar.gz](../../system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08/retained-source.tar.gz)를 저장했다(5,411,328 bytes). 새 위치에 archive만 풀고, CapturedSource.Root를 존재하지 않는 입력 경로로 설정한 뒤 생산 `MaterializeBuildTree` API로 다시 복원했다. 1,410개와 동일 source identity 및69개 검토 span을 확인했다. 실제 결과는 [archive 재생](../../system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08/archive-replay.json), [전체 검증](../../system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08/verification.json)이다. 실행은 준비용 Go driver의 생산 API 호출이며 새 제품 CLI나 외부 소비자 수용이라고 주장하지 않는다.

## 남은 결합과 완료 판정

이번 소스 캡처는 N12-B의 기술 입력 준비다. 새 신원의 CKV·CKG·knowledge lock·semantic projection 결합은 아직 없다. 이전 graph-only canonical36참조를 새 snapshot에 자동 재사용하지 않으며, 새 신원으로 다시 연결·검증해야 한다. 모든 내용/관계와 검토자·실제 활동 시간은 원래 사람 판정을 기다린다. canonical ID 조회 수정의 전체 Go 회귀는 이전 실행 증거를 참조했고, 제품 소스가 바뀌지 않은 이번 준비에서 전체 시험을 다시 실행했다고 기록하지 않는다. 이번 Go 의존 검사 범위는 Darwin ARM64의 활성 build 입력이며 다른 OS/태그 실행을 입증하지 않는다.

N12-A–D는 미체크다. 새 모델 호출/FINAL질의/의미승격/공개 전송0, coordinated_dataset_built=false, knowledge_lock_built=false, human_approved=0이다. 기존 승인 입력5개와 Go29 범위 파일, 원래 두 검토 책·claim seeds·canonical/audit 자료의 SHA를 재확인했다.

**다음 작업:** 실제 N11/N12 검토 판정과 제안 입력 범위를 반영한 coordinated source/CKV/lock/canonical 연결 준비, 이후 N14의 실제 patch/run과 criterion별 수용. **현재 전체8/18·요청5/15 완료, 남은 전체10개:** N08 검색 품질, N09 무답·기권, N10 비용·지연, N11 독립 평가, N12 실제 파일럿 검토, N14 실제 의미 수용, N15 지원 환경·외부 소비자, N16 운영 권한·고지, N17 운영 복구, N18 최종 수용·출시 판정.

2026-10-08 N12 기술 연결 검증: [동일 신원의 held 색인·의미 준비](./N12-COORDINATED-PREPARATION.md) — 보관소스1,410파일/선택58의 CKG53Go·CKV56입력·666chunks/566심벌 canonical100%, 문서27/27·코드36/고유33·proposed 저장 roundtrip·12거부 제어·archive-only 재생 통과. 원문project knowledge-system로 수정한 별도 후보이며 첫 mismatch 거부와 전체Go범위 DRIFT998 경고를 보존했다. 실제 BGE embedding HTTP124회(색인입력1332·시작probe4), 새DEV/FINAL0·human0·current/semantic activation0. approved overlay/lock·내용/관계/입력범위 판정은 남아 N12-A–D/전체8/18·요청5/15·잔여10개를 유지한다.
