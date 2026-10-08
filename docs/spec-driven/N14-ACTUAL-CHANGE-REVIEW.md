# N14 실제 변경·기준·정확 시험의 검토 준비

2026-10-08 최신 하위 단계: **N14 생산 exact runner→공개 보고서 등록 통합 검증**. [검증 문서](./N14-EXACT-RUN-ROUNDTRIP.md) — 소유 root/nested CKG 원문 fixture의 정상 등록·재시도/잘못된 proof·덮어쓰기 거부/자동 판정·current 전환 없음; 관련 race47시험·33하위시험 및 vet/boundary/format 통과. N12 미적용 초안의 필수 질문3개를 교정하고 생산 파서·구조 검증했으며 실제 적용/사람 승인0이다. **전체8/18·요청5/15·남은10개 유지**. 다음 N12 명세 범위·관계 및 N11 gold/독립성 판정과 부족 표본 보강. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 아래 기록은 이전 단계 증거를 보존한다.

2026-10-08 최신 하위 단계: **N14 exact 실행 보고서/검토 경로 일치 검사 보완**. [수정과 검증](./N14-EXACT-RUN-PROOF-VALIDATION.md) — 수정 전15불일치 수용 RED, 수정 후16거부·multiple proof/nested package 정상 사례와 관련 race/vet 통과. 기존 실제 held 후보의 generic/exact10요청은 명령 시작 전에 거부됐으며 linked report/사람 decision/current0, 원 권위9파일 불변이다. **전체8/18·요청5/15·남은10개 유지**. 다음 N12 명세 범위·관계 및 N11 gold/독립성 판정 반영 후 실제 N14 후보/시험/수용이다. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 아래 기록은 이전 단계 증거를 보존한다.

2026-10-08 최신 하위 단계: **N12 누락 개념 경로5개 범위 대조·실제 retained matcher→stage2→V2 검증3사례 완료**. [범위 검토와 명세 초안](./N12-CONCEPT-PATH-SCOPE-REVIEW.md) — source18참조/관련5패키지17시험·32하위시험/변조6거부/권위9파일 불변. 기존 미작성5개·reviewed 경로0은 유지하며 기능 개념3개·requirement version2 초안은 미적용이다. **전체8/18·요청5/15·잔여10개**. 다음 명세 범위 선택 및 N11 gold/독립성·N12 개념/관계 판정. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 아래 기록은 이전 단계의 증거를 보존한다.

2026-10-08 · **소스/실행 자료 준비 완료 · 사람 수용/통합 후보 미완료** · [검토 JSON](../../system/eval/b0-knowledge-system/refactoring-n14-actual-change-preparation-2026-10-08/review-packet.json)

목적은 ADR와 실제 요구/기준을 실제 코드 변경 및 테스트 실행에 연결하고, 사람 수용 이후 새 후보 데이터셋을 승격할 수 있게 하는 것이다. mock fixture의 승인자나 GitHub merge 요청을 기준별 사람의 의미 수용으로 사용하지 않는다.

## 실제 변경과 원문 범위

[PR97](https://github.com/0xmhha/knowledge-system/pull/97)은 이전main1ded9b3e에서 결과main eeeab2e7로 실제 스쿼시 머지됐다. 이 자료는 그 변경 중 `req-source-evidence`/`ac-source-evidence` 및 semantic 정렬·patch 승격 경계의7변경 파일을 다룬다. PR 전체를7파일 변경이라고 축소하는 뜻은 아니다. 실제 diff와 전후 파일SHA, 아래11개 inclusive span을 보관했다.

후속 source commit `8051021fb8dd7017e5d302ec3171a4faec415cfd`에서 읽은 span 파일SHA가 머지된 main과 현행 파일에 모두 일치했다. ADR의 초기 문제/과거 worktree 선택을 현행 사실로 쓰지 않고 'graph/vector/semantic 모든 ID 검사 후 current 전환'의 채택 계약을 연결 후보로 사용한다. 최초 준비의 line29 가정은 의도 문장을 포함하지 않아 내부 assert로 거부했고 실제 번호 문장을 찾아 교정했다. 이는 자료 작성 오류이며 제품 실패/사람 반려가 아니다.

| 근거 ID | 정확 원문 좌표 | 자료 지위 |
|---|---|---|
| ADR-source-alignment | docs/spec-driven/SOURCE-SNAPSHOT-ADR.md:28–28 | adopted-design-source |
| req-source-evidence | docs/spec-driven/spec-pilot.yaml:19–29 | proposed-requirement-and-criterion |
| alignment-code | internal/system/semantic/alignment.go:69–138 | implementation-candidate |
| alignment-test | internal/system/semantic/alignment_test.go:26–107 | test-candidate |
| source-test | internal/system/semantic/validate_test.go:88–109 | test-candidate |
| exact-run-gate | internal/system/patch/review.go:53–80 | promotion-gate-implementation |
| human-decision | internal/system/patch/review.go:109–154 | human-decision-implementation |
| promotion-gate | internal/system/patch/review.go:205–282 | promotion-gate-implementation |
| test-command-only | internal/system/patch/patch_test.go:121–137 | gate-control-test |
| test-active-base | internal/system/patch/patch_test.go:91–119 | gate-control-test |
| test-patch-digest | internal/system/patch/patch_test.go:48–89 | gate-control-test |

ADR→requirement와 criterion→code/test 연결은 **proposed**다. 실제 CKG canonical ID·새 coordinated snapshot/lock·semantic trace 검토가 아직 없다. source hash를 verified semantic assertion 또는 accepted criterion으로 바꾸지 않았다. N12의 같은 명세/기준 검토 응답을 실제 source 범위에 연결한 뒤 canonical·단계별 조건을 재검증해야 한다.

## 새 정확 테스트 실행

새 Go JSON 출력에서 아래5개가 각각 run/pass했고2패키지도 pass했다. `-count=1`로 캐시 결과를 재사용하지 않았다. 다른 snapshot·pin·root/schema 거부, immutable patch ID/변경 SHA, 활성base 재검사, command 성공만 있는 승격 거부를 확인한다.

- `TestProjectionRejectsCrossSnapshotOrMissingEvidence`
- `TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty`
- `TestPatchRegisterPinsChangedFilesAndRefusesReusedID`
- `TestReviewedPromotionRechecksActiveBaseAtSwap`
- `TestCriterionPromotionRejectsCommandOnlySuccess`

실행 코드/모듈은 기록한 commit과 머지 main 바이트가 같다. 해당 테스트 안의 데이터/관계/승인자는 소유 control fixture다. 이는 실제 코드의 새 테스트 실행이고 자연어 검색/실제 파일럿의 승인된 semantic trace 실행/운영 수용은 아니다. raw Go 결과를 `cks semantic test --go-test-exact`의 승인된 linked report로 꾸미지 않는다. TestExecution.pass와 CriterionDecision.approved는 분리한다.

## 실제 수용으로 연결할 다음 단계

1. N12의 내용/관계와 위 실제 source 연결을 검토한다. 필요하면 일부 관계를 수정/기권하고 명시한 범위로 새 원문을 고정한다.
2. 실제 프로젝트의 baseline/result를 coordinated capture/knowledge lock/CKG canonical에 결합한다. 새 result는 hold-for-review로 만들고 active current를 보존한다. 실제 `cks patch register`의 before/after archive digest로 이 Git diff와 독립 대조한다.
3. 검토된 trace에서 exact test를 실행하고 명령/출력SHA·spec version·result snapshot을 결합한 immutable run을 기록한다. 무관 test·command-only success·stale/snapshot mismatch는 실제 후보에서 거부한다.
4. 검토자가 `ac-source-evidence`의 Given/When/Then과 결과를 확인해 approved/rejected와 이유를 기록한다. 아직 검토자/시각/의미 판정/실제 활동 시간은null이다. 메시지 대기를 활동 시간으로 바꾸지 않는다.
5. 실제 후보 승격·반려·rollback/current/보관 근거 재생을 실행하고 단계별 raw 자료를 바인딩한다. 모든 기준/trace와 별도 사람 판단을 확인하기 전 promote하지 않는다.

현재 patch_registry_attempt_created=false, semantic_linked_test_report_created=false, human_decision=null, candidate_promoted=false다. N14-A–E와 N14 전체는 미완료다. 새 model/FINAL 질의0, 승인/활성 승격0, releasefalse다.

전체8/18·요청5/15 완료, 잔여N08/09/10/11/12/14/15/16/17/18 10개. 다음 N11/N12 실제 검토 응답 반영, 독립 FINAL 범위 작성과 위 actual candidate 결합이다.

2026-10-08 N12 trace 연결 준비: [13개 proposed 관계와 의미 경계 검증](./N12-PROPOSED-TRACE-RELATIONS.md). 구현3/시험5/기준5 관계를 별도 SQLite에 원문·canonical·CKV로 연결하고 재조회/원본 없는 두 archive 재생·9거부 제어를 통과했다. Projection.Validate 시험을 다른 정렬 함수에 연결하는 부정확한1쌍은 제외, broad concept5경로 미작성 유지. 전체25관계 proposed·3요구 spec_unapproved·검토된경로0/current없음/human0/model0. 신규FINAL로 합산하지 않으며 N11 pool13/8·승인0, 전체8/18·요청5/15·잔여10개 유지. 다음 관계/원문 범위 판정·5경로 보강과 N11 독립 입력 확보다.
