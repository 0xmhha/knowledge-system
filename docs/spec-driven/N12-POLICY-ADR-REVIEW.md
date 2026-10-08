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
