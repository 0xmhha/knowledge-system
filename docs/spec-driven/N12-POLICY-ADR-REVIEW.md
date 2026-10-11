# N12 정책·ADR의 실제 주장과 관계 검토 보완

2026-10-06 · 원문 커밋 `ab77ce5691d98328187ab271c1ae580299396265` · **추가 검토 대기 / N12 미완료**.

기존 [27개 타입·명세/기준 검토](./N12-ACTUAL-PILOT-REVIEW.md)는 그대로 유지한다. 원 작업리스트의 claim/정책/ADR 검토가 빠져 있어 아래6개 실제 원문 주장을 추가했다. 문서의 현행 관리 규정과 채택 ADR의 최신 보강 부분만 사용하고, ADR 초기 문제 서술/공유 worktree 기록을 현행 사실로 추출하지 않았다.

각 주장에 DocumentSection SUPPORTS Claim, Claim ABOUT Concept 두 관계를 제안했다.6주장/12관계 모두 proposed이며 사람 승인·verified/accepted는0, precision은null이다. 원문 SHA/좌표와 타입·방향 일치는 도구로 확인했지만 내용의 정확성·권위·관계 의미는 아직 사람이 판단하지 않았다.

새 projection은 standalone committed-source 검토 자료다. 통합 dataset/source archive/lock 또는 CKG canonical 관계를 승격하지 않았다. 아래 코드 좌표도 사용 후보이며 IMPLEMENTED_BY/TESTED_BY 관계라고 생성하지 않았다. 운영 담당자·키 신뢰·native 환경/RTO/RPO 결정은 이 문서의 승인 범위 밖이다.

## P12-P01

Pinned v2 유지보수는 제자리 갱신과 legacy downgrade를 거부하고 검토 설정·project ID와 새 버전으로 후보를 만든다.

- 원문: [docs/spec-driven/PUBLIC-CONTRACT-V2.md:23-23](../../docs/spec-driven/PUBLIC-CONTRACT-V2.md)
- 권위: 현행 공개 계약의 N02 규정; 운영 담당자 승인 기록 아님
- 범위: CKS 공개 관리 경로; low-level engine 직접 쓰기는 보호 밖
- 코드 사용 후보: [func checkMaintenanceTarget(path string, versioned bool) error {](../../internal/setup/maintenance.go)
- 제안 관계: `supports:P12-P01` SUPPORTS, `about:P12-P01` ABOUT → `policy`
- DEV 중복 묶음: maintenance-ancestor
- 판정: 승인 / 수정 / 기권 — 검토자·이유 미기록

## P12-P02

후보 checkpoint/sync 뒤 current를 전환하고 부모를 sync한다. rename 후 sync 실패의 durability_uncertain은 current 불변을 뜻하지 않는다.

- 원문: [docs/spec-driven/PUBLIC-CONTRACT-V2.md:27-27](../../docs/spec-driven/PUBLIC-CONTRACT-V2.md)
- 권위: 현행 공개 계약 N03/N04; 운영 복구 승인 기록 아님
- 범위: 협력하는 local writer의 내구성 순서; 실제 전원 차단·운영 지원은 별도
- 코드 사용 후보: [func syncCurrentParent(dataset string) error {](../../internal/setup/durability.go)
- 제안 관계: `supports:P12-P02` SUPPORTS, `about:P12-P02` ABOUT → `dataset`
- DEV 중복 묶음: promotion-parent-sync
- 판정: 승인 / 수정 / 기권 — 검토자·이유 미기록

## P12-P03

v2 core evidence는32000bytes/12citation 안에서 검증된 전체 원문 span을 선택하고 생략을 partial/예산 진단으로 기록하며 긴 span을 잘라 반환하지 않는다.

- 원문: [docs/spec-driven/PUBLIC-CONTRACT-V2.md:110-112](../../docs/spec-driven/PUBLIC-CONTRACT-V2.md)
- 권위: 현행 공개 계약 N07; 새 검색 품질 합격 아님
- 범위: retained_core 선택; semantic 추가 후 전체body 상한도 검사
- 코드 사용 후보: [func BuildFromRefs(ctx context.Context, versionDir, query string, refs []Ref, cleaner *sanitize.Engine) (contract.EvidencePackV2, error) {](../../internal/system/evidencev2/pack.go)
- 제안 관계: `supports:P12-P03` SUPPORTS, `about:P12-P03` ABOUT → `evidence-pack`
- DEV 중복 묶음: whole-span-budget
- 판정: 승인 / 수정 / 기권 — 검토자·이유 미기록

## P12-P04

Worksheet의 조직 위험 목록은 명시적으로 선택한 local pack 자료에만 들어 있다. 생성은 앵커 검사를 실행하거나 항목을 verified로 승격하지 않는다.

- 원문: [docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md:3-3](../../docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md)
- 권위: 현행 worksheet 계약 N13; 실제 사람 승인 기록 아님
- 범위: 공통 review 질문과 명시 선택형 local catalog; 운영 정책의 권위 승인 아님
- 코드 사용 후보: [func LoadWorksheetCatalogs(projectRoot string, selections []Selection) ([]WorksheetCatalog, error) {](../../internal/system/knowledgepack/worksheet.go)
- 제안 관계: `supports:P12-P04` SUPPORTS, `about:P12-P04` ABOUT → `policy`
- DEV 중복 묶음: worksheet-selection
- 판정: 승인 / 수정 / 기권 — 검토자·이유 미기록

## P12-P05

Git 소스는 기준HEAD/선택 복구 객체를 해시된 history.bundle로 봉인하고 독립 저장소로 복원해 빌드하여 원본의 이후 이력 변화와 분리한다.

- 원문: [docs/spec-driven/SOURCE-SNAPSHOT-ADR.md:63-63](../../docs/spec-driven/SOURCE-SNAPSHOT-ADR.md)
- 권위: 채택 ADR의 최신 Git 이력 격리 규정; 실제 지원 규모·운영 복구 합격 아님
- 범위: ADR의 B0-H 보강 계약; 최초 공유 worktree 선택보다 우선
- 코드 사용 후보: [func (c CapturedSource) MaterializeBuildTreeContext(ctx context.Context, path string) (func() error, error) {](../../internal/setup/capture.go)
- 제안 관계: `supports:P12-P05` SUPPORTS, `about:P12-P05` ABOUT → `source-snapshot`
- DEV 중복 묶음: 미지정; 기존 관측 사실과 별도 오염 검토 필요
- 판정: 승인 / 수정 / 기권 — 검토자·이유 미기록

## P12-P06

신규 executable-bit-v1 입력은 파일 실행 비트도 identity에 포함하며 snapshot v5와 구 v2–v4 읽기를 구분해 과거 바이트·신원을 재해석하지 않는다.

- 원문: [docs/spec-driven/SOURCE-SNAPSHOT-ADR.md:69-69](../../docs/spec-driven/SOURCE-SNAPSHOT-ADR.md)
- 권위: 채택 ADR의 최신 파일 모드 규정; 개별 후보 source 검토는 별도
- 범위: ADR의 실행 비트 교정 계약; 내용 바이트만으로 식별하지 않음
- 코드 사용 후보: [func sourceSnapshotID(s SourceIdentity) string {](../../internal/setup/identity.go)
- 제안 관계: `supports:P12-P06` SUPPORTS, `about:P12-P06` ABOUT → `source-snapshot`
- DEV 중복 묶음: 미지정; 기존 관측 사실과 별도 오염 검토 필요
- 판정: 승인 / 수정 / 기권 — 검토자·이유 미기록

## 검증과 필요한 판정

[검토 JSON](../../system/eval/b0-knowledge-system/refactoring-n12-policy-adr-review-2026-10-06/review-items.json)에 파일/spanSHA·적용범위/권위·코드 후보/관계와 빈 판정 칸을 기록했다. actual CLI source-hash/cross-snapshot/wrong-direction/unreviewed-parent4개 음성 제어가 모두 거부됐고 원래 projection은 바뀌지 않았다. 생성 초기의 절 범위/필수 evidence 누락도 기록하고 교정했다. 이는 검토 자료 작성 오류이며 제품 실패나 실제 사람의 반려로 집계하지 않는다.

P12-P01–06의 주장 내용과 SUPPORTS/ABOUT 관계를 승인/수정/기권하고 검토자 이름·이유를 알려준다. 기존27개와 별도 항목이므로 이전 항목 승인만으로 이6개가 승인되지는 않는다. 실측 활동 시간이 있으면 기록하고 없으면 미측정/null을 유지한다. 이 자료를 새 FINAL로 지정하거나 공개 질문 수를 독립 표본으로 합산하지 않았다.

전체 완료8/18, 요청15개 중5완료; 잔여N08/09/10/11/12/14/15/16/17/18 10개. 다음은 실제 검토 판정 반영과 source/lock·canonical 결합 및 새 독립 평가 입력 검토다.

2026-10-08 현행 대조·연결 보완: [현행 canonical 연결 준비](./N12-CANONICAL-SOURCE-PREPARATION.md)에서 기존 33항목의 원문/코드 69참조가 동일함을 확인했다. 실제 CKG 코드·테스트 20파일에서 36참조/33고유 canonical ID를 결합했으나 code evidence만 붙였으며 내용·관계 승인, 통합 capture/lock/CKV 또는 활성 승격은 없다. 공통 SQLite 조회의 canonical_id 누락을 재현·수정했고 7개 근거 거부 제어를 통과했다. 저장소 전체 committed capture는 평가 public.pem 경로 때문에 실제 거부됐으므로 별도 source-only Git 검증을 통합 캡처 완료로 쓰지 않는다. N12 전체와 사람 판정은 대기를 유지한다.

2026-10-08 N12 캡처 입력 준비 검증: [명시 소스 범위의 보관 캡처](./N12-SCOPED-CAPTURE-PREPARATION.md) — 원본667a9af5에서 소스·의존·검토 문서1,410개를 독립 준비 Git 커밋562e37aa로 구성해 committed 캡처/복원·archive-only 재생/신원·69검토span·승인Go29·8거부 제어를 확인했다. snapshot-only는 build31/vendor fixture1 등32개를 제외하므로 전체 입력 보관으로 사용하지 않았다. 제품 정책/소스 변경0, 새 모델/FINAL질의·사람 판정0. 제안 입력 범위·내용/관계 승인 및 새 coordinated CKG/CKV/lock 결합은 미완료, 전체8/18·요청5/15·잔여10개 유지.

2026-10-08 N12 기술 연결 검증: [동일 신원의 held 색인·의미 준비](./N12-COORDINATED-PREPARATION.md) — 보관소스1,410파일/선택58의 CKG53Go·CKV56입력·666chunks/566심벌 canonical100%, 문서27/27·코드36/고유33·proposed 저장 roundtrip·12거부 제어·archive-only 재생 통과. 원문project knowledge-system로 수정한 별도 후보이며 첫 mismatch 거부와 전체Go범위 DRIFT998 경고를 보존했다. 실제 BGE embedding HTTP124회(색인입력1332·시작probe4), 새DEV/FINAL0·human0·current/semantic activation0. approved overlay/lock·내용/관계/입력범위 판정은 남아 N12-A–D/전체8/18·요청5/15·잔여10개를 유지한다.

2026-10-08 N12 ADR 기술 lock 연결: [검토 대기 ADR의 보관 lock](./N12-PROPOSED-LOCK-PREPARATION.md) — 원소스1,410 불변, proposed ADR2/정책0(운영 owner 미정)/review0의 partial overlay·lock과 별도 held snapshota0457cc1/dataset5a24904f를 생성했다. 생산 knowledge·include-packs CLI/v4 의미27문서·36코드참조/33canonical·7 lock 거부·원 소스/색인/semantic DB 없이 archive 재생 통과. 새 BGE embed HTTP62=probe2+색인60/666입력, 전체Go DRIFT998 유지. 승인 lock/사람 내용·관계·운영 판정은 미완료, N12-A–D unchecked·전체8/18·요청5/15·잔여10개 유지. 다음 N11 실제 gold/독립성 및 N12 판정/정책 담당자·시점 확인.

2026-10-08 N12 trace 연결 준비: [13개 proposed 관계와 의미 경계 검증](./N12-PROPOSED-TRACE-RELATIONS.md). 구현3/시험5/기준5 관계를 별도 SQLite에 원문·canonical·CKV로 연결하고 재조회/원본 없는 두 archive 재생·9거부 제어를 통과했다. Projection.Validate 시험을 다른 정렬 함수에 연결하는 부정확한1쌍은 제외, broad concept5경로 미작성 유지. 전체25관계 proposed·3요구 spec_unapproved·검토된경로0/current없음/human0/model0. 신규FINAL로 합산하지 않으며 N11 pool13/8·승인0, 전체8/18·요청5/15·잔여10개 유지. 다음 관계/원문 범위 판정·5경로 보강과 N11 독립 입력 확보다.
