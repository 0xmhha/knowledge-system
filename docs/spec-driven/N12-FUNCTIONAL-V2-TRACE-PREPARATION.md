# N12 기능 version2의 실제 소스·색인·관계 준비

2026-10-08 최신 하위 단계: **N12 actual foreign-code-anchor 승격·active 보존 통합 회귀 보강**. [새 시험·범위](./N12-CODE-ANCHOR-PROMOTION-REGRESSION.md) — 공개CKG2source/2DB·canonical stable/commit 다름,5사례8쓰기+2activation 거부/old current·저장 자료·reopen 보존. semantic race42시험20하위시험/vet/boundary/format 통과. 최초 fixture YAML 작성 오류는 제품RED와 별도 보존했다. 기존 held66artifact·승인input5 불변, 새 시험은 이전55d8/43ba/f839 pilot 후보에 아직 미포함이다. 전체8/18·남은10개 유지;다음 새 source/index/canonical 관계 결합과 N11 실제 gold·독립성/검토자 판정. 남은 N08/09/10/11/12/14/15/16/17/18. 사람 승인·새FINAL·실모델·운영 활성화0.

2026-10-08 · **새 held 후보와 관계15개 기술 검증 완료 / N12 전체 미완료**. 전체8/18, 요청 N04–18의5/15, 남은10개다.

## 목적과 보관 범위

사용자가 선택한 기능 개념3개를 실제 구현·시험에 연결한다. 과거 broad concept의13관계나 승인 상태를 새 snapshot으로 승계하지 않았다. 원 core20·원 version1·이전 후보를 보존하고, 선택 ontology23개와 requirement version2의3요구/4기준을 같은 소스에 넣었다.

원 코드 커밋 `254b7c8797f804f807dc827770ffa60cc81c3949`의 모든 committed cmd/internal/pkg 파일과 기존 비평가 의존 자료·선택 문서·원 계약3문서를 소유 Git 준비 커밋에 복사했다. **1,437파일/12,225,886bytes**, derived commit `55d8e2db7a55b2956c4e89f986d0c3759fee4d25`다. 바이트와 실행 비트를 대조했고 원본 전체 Git 역사는 복사하지 않았다. 별도 준비 author는 사람 검토자가 아니다. 전체 원 저장소·사용자 .claude/logs나 평가 원자료를 source corpus에 넣었다고 주장하지 않는다.

현행 CKS·CKG·CKV3개를 새로 빌드하고 binary SHA를 보관했다. Go1.26.8/Darwin ARM64이며 승인 BGE-M3 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`/1024차원·Ollama0.35.1이 전후 같다. 첫 sandbox의 loopback 메타데이터 접근 거부는 별도 환경 원자료에 남겼고 허용된 로컬 경로에서 완료했다.

## 실제 held 신원과 색인

| 좌표 | 값 |
|---|---|
| project | knowledge-system |
| snapshot | `43ba745283e2d8d409020475708e1e9c10c99fcf3977202eea3b5f39b58555b9` |
| dataset | `f83921128851b5e5bd6067419ba22ea9d9d154a545d3742d07126d36fe606c17` |
| source mode / commit | committed / `55d8e2db7a55b2956c4e89f986d0c3759fee4d25` |
| lifecycle | functional-v2-candidate, hold-for-review; current 없음 |

생산 filelist는 pinned Darwin ARM64 pkg/graph/types의 의존·시험, 기존 승인 Go29/필수 원문, 새 matcher·검색 호출자·검증 문맥 등을 합쳐70파일이다. **실제 CKG62Go/CKV65(같은62Go+Markdown3)**와 각 지원 언어 집합이 정확히 같고 SHA도 source와 같다. core/spec YAML4개와 PILOT-SCOPE.json은 보관하며 vectorized로 세지 않는다. CKG10,324nodes/23,918edges, parse errors/unresolved refs0이다. CKV742chunks,621/621코드 심벌 canonical 연결이며 floor1.0을 유지했다.

범위 없는 생산 CKG 감사는 Go1,054↔DB62, in_both62/missing992/extra0, exit1 **DRIFT**다.992개는 모두 선택 Go 범위 밖이다. setup의 기존 soft warning을 보존했다. 선택 범위 대조 통과가 전체 저장소 parity/검색 품질 통과라는 뜻은 아니다. [범위 증거](../../system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08/index-scope-check.json).

## 새 관계 후보와 정확한 시험 범위

[관계 검토 자료](../../system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08/relation-review-items.json)의 **F2-T01–15**는 IMPLEMENTED_BY3/TESTED_BY6/CHECKED_BY6이다. 기존12개 SUPPORTS/ABOUT도 새 소스에서 재추출했으며 총27assertions 모두 proposed다. 정확한 실제 CKG 함수/시험 canonical11개는 관계 대상9개와 추가 문맥2개다. [canonical·원문](../../system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08/canonical-source-refs.json).

| 기능 개념 / 구현 | 연결 시험 | 범위와 한계 |
|---|---|---|
| filtered-nearest-chunk-search / SearchDetailed | sparse-filter K·exact order, candidate limit, cancellation3시험 | 통제한 local vector fixture; 실제 corpus의 품질/지연 판정은 별도 |
| snapshot-bound-semantic-relations / ValidateDatasetAlignment | TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty | PutAligned/ActivateAligned 간접 호출; manifest 변조·저장/활성화 거부·기존 current 보존. 실제 foreign CKG code anchor 승격 통합 시험은 아직 없음 |
| optional-ontology-baseline-preservation / applyProvidedOntology | 기존 fallback/ambiguity + TestStoredSharedAliasPreservesBaselineAndRetainedEvidence | 전자는 fake resolver; 후자는 실제 parser/SQLite/matcher/retained body와 통제 engine manifest·Fake CKG/hits. live hybrid 검색 품질 증명은 별도 |

추가 문맥 `checkCodeAnchor`와 `TestCodeAnchorRequiresSameSnapshotAndContainedASTSpan`은 actual canonical에 연결했으나 자동으로 criterion CHECKED_BY에 넣지 않았다. helper의 snapshot/AST 검사와 promotion 전체 경로는 서로 다른 검증 범위다. 또한 Projection.Validate 시험을 ValidateDatasetAlignment 시험이라고 연결하지 않았다.

새3개 요구 개념 각각에는 구현·시험 관계 **후보**가 있다. 과거 version1의 broad concept 미작성5개는 역사 기록이며, 이를 지워 완료로 바꾸지 않는다. 새 기능 범위의 사람이 검토한 경로는0이다. [기준별 검증·누락](../../system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08/criterion-coverage-review.json)에 따라 ac-source-evidence의 actual foreign-anchor promotion/active 보존 통합 시험을 우선 보강한다.

## 실행과 이중 확인

- [x] 실제 소스 캡처·현행 CLI3개·승인 모델/차원·strict embedding·canonical floor1.0·held 후보 생성과 CKG62/CKV65 파일/SHA 대조.
- [x] 생산 추출기/retained 검증기에서 ontology23·요구3/version2·criteria4·claims6/총27assertions·74evidence를 검증. 실제 Markdown27/27 CKV section 연결, canonical11, 별도 SQLite PutAlignedRetained→닫기→read-only Load의 JSON 바이트 동일.
- [x] foreign snapshot/보관 코드 SHA/없는 CKG canonical/다른 CKV chunk/미검토 concept의 verified 구현 **5제어 거부**. 개별 의미 판정의 대체가 아니다.
- [x] 소유 Git source에서 관계의 실제 시험7개/하위24개/패키지3개 race 통과, fail0/skip0. 전체 Go·Linux·live 질의 시험은 새로 수행하지 않았다. semantic linked run이나 사람 criterion accepted 기록을 만들지 않았다.
- [x] 보관 archive13,659,082bytes/1,456files를 독립 위치에 복원. 원 source/dataset/semantic DB가 모두 없는 상태에서 archive만으로 실제 identity/source/canonical/CKV/read-only SQLite를 재검증했다. projection/canonical/관계/trace 바이트 및 archive 권위 파일 SHA가 같고 원본은 finally에서 복원했다. 실제 운영 이동/복구 수용은 별도다.
- [ ] actual foreign CKG anchor promotion·기존 active 보존 통합 시험.
- [ ] 실제 검토자와 개념/요구/관계·criterion 내용 판정, 승인 overlay/lock 및 N14 실제 linked run/수용.

[재생 증거](../../system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08/archive-replay.json) · [held archive](../../system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08/held-functional-candidate.tar.gz) · [실제 시험](../../system/eval/b0-knowledge-system/refactoring-n12-functional-rebind-2026-10-08/related-tests-summary.json).

색인 비용은 로컬 relay HTTP219/embedding71회다. **probe2회/2입력 + 색인69회/742입력**, 성공744/실패0/truncate:false다. 원문 본문과 vector를 HTTP 관측 자료에 저장하지 않았다. 검색 DEV/FINAL·warm/cold/p95·사람 활동 시간으로 사용하지 않는다. archive 재생의 model 호출0이다.

**사람의 범위 선택2건은 기존 기록 그대로, 새 개별 내용/관계/criterion 판정0·검토자 이름null·활동시간null.** Trace3요구 모두 spec_unapproved/paths없음, verified boost0, dataset/semantic current0, linked run0, actual patch0, 새 FINAL0이다. 새 자료를 N11 독립 FINAL15문항으로 세지 않는다. 원 승인5입력/core·legacy/선택 scope·old book/protocol/worksheet·이전 projection/archive16파일의 current/Git SHA를 대조해 보존했다. Claude 실행·외부 전송0, Codex 직접 수행이다.

## 다음 우선순위와 남은 전체

다음은 **N12 ac-source-evidence 통합 시험 보강**이다. 이후 새 시험을 별도 source 신원에 결합하며 이 held 후보를 덮어쓰지 않는다. N11은 reviewer 이름과 문항별 gold/군·독립성·오염/ABS·protocol 판정 및 각 중요군10승인 확보, runtime 전체 동결이 필요하다.

| 우선순위 | 남은 작업 | 완료에 필요한 결과 |
|---|---|---|
| 1 | N12 실제 파일럿 검토 | 누락 통합 시험, 새 정확한 근거, 실제 개념·요구·관계 내용 판정 |
| 1 | N11 독립 평가 입력 | 독립 사실군/개별 gold 승인·reviewer·충분한 FINAL·전체 freeze |
| 2 | N08 검색 품질 | 승인·동결 입력의 실제 검색 지표/회귀 판정 |
| 2 | N09 기권 | answerability/인용0·사람 기권을 분리한 실측 |
| 2 | N10 비용·지연 | 같은 평가 arm의 호출·warm/cold·p95 실측 |
| 3 | N14 실제 변경 수용 | 검토 경로에 결합한 실제 patch/run·사람 criterion 결정 |
| 3 | N15 규모·설치·이식성 | 실제 지원 규모/환경·독립 소비자 수용 |
| 3 | N16 신뢰·권한 | 실제 owner·역할·키·고지 판정 |
| 3 | N17 실제 복구 | 운영 조건·RTO/RPO·키 복원·장애 훈련 수용 |
| 4 | N18 기본값·출시 | 위 결과의 최종 수용·default/release 결정 |

N12-A–D와 N11-A–D를 완료로 체크하지 않았다. 완료8개는 N01–07/N13, 남은10개는 N08/09/10/11/12/14/15/16/17/18이다.
