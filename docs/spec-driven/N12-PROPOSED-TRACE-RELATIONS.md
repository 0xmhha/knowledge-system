# N12 실제 원문에서의 proposed trace 관계 준비

2026-10-08 · **13개 관계 기술 검증 완료 / 내용·관계 승인 및 N12·N14 전체 미완료**.

[관계 검토 자료](../../system/eval/b0-knowledge-system/refactoring-n12-trace-proposals-2026-10-08/relation-review-items.json) · [검토 ID 대응](../../system/eval/b0-knowledge-system/refactoring-n12-trace-proposals-2026-10-08/relation-index.json) · [실행 요약](../../system/eval/b0-knowledge-system/refactoring-n12-trace-proposals-2026-10-08/summary.json)

원래 목적은 명세·기준의 원문을 실제 코드·시험에 연결한 뒤, 사람이 연결의 의미와 수용 결과를 따로 판정할 수 있게 하는 것이다. 이전 N12 준비는 원문69참조/코드36참조·canonical33 및 SUPPORTS/ABOUT12관계를 보관했지만, 실제 파일럿의 구현·시험·기준 관계는0개였다. 이번에는 이 빈 부분의 **일부를 검토할 수 있는 proposed 관계**로 준비했다. 타입·바이트 일치는 관계의 의미가 맞다는 승인과 다르다.

## 작성한 관계와 실제 연결 범위

기존 held source `32d7e2641640b82995cf443bd8a2553dd0ce69b9`, snapshot `a0457cc1c51791dfdfbdaa3cd7272364bf9197f5aadbe968bd0701c40136bea5`, dataset `5a24904fb0ad21bfd0bae7f6d05d0afef1e52de7c03ffa8f11c97be8966bda47`를 읽었다. 새 source/index/embedding이나 다른 dataset으로 바꾸지 않았다. 기존 v4 projection은 수정하지 않고 **별도 SQLite의 다른 검토 초안**에 원문을 보존했다. dataset ID가 같아도 두 초안의 projection digest는 다르므로 기존 store를 덮어쓰거나 기존 검토 승인을 승계하지 않는다.

| 검토 ID | proposed 관계 | 사람에게 확인할 의미 |
|---|---|---|
| P12-T01 | vector-chunk → SearchDetailed (`IMPLEMENTED_BY`) | 공통 메타모델의 vector-chunk 뜻과 검색 함수 구현을 연결하는 해석이 타당한가 |
| P12-T02/03 | SearchDetailed → TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded (`TESTED_BY`), ac-filtered-recall → 같은 시험 (`CHECKED_BY`) | sparse filter의 K/거리 순서를 이 구현·시험이 해당 기준 범위에서 확인하는가 |
| P12-T04/05 | SearchDetailed → TestSearchDetailedReportsCandidateLimitWithoutCompleteHits, ac-filtered-completeness → 같은 시험 | candidate limit 상태와 incomplete 의미를 확인하는가 |
| P12-T06/07 | SearchDetailed → TestSearchDetailedCancellationIsNotEmptySuccess, ac-filtered-completeness → 같은 시험 | 취소/deadline을 빈 성공과 구분하는 시험이 이 기준을 확인하는가 |
| P12-T08 | evidence-span → ValidateDatasetAlignment (`IMPLEMENTED_BY`) | evidence-span의 전체 타입 구현이라는 과도한 주장인지, 정렬 검증 기능의 부분 연결로 타당한지 |
| P12-T09/10 | ValidateDatasetAlignment → TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty, ac-source-evidence → 같은 시험 | Store.PutAligned를 통한 간접 호출과 snapshot 거부/current 보존의 Given/When/Then이 맞는가 |
| P12-T11 | concept → applyProvidedOntology (`IMPLEMENTED_BY`) | concept 전체의 구현과 optional 해석/후보 보존 기능의 부분 연결을 구분할 수 있는가 |
| P12-T12/13 | applyProvidedOntology → TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline, ac-ontology-fallback → 같은 시험 | Searcher.Search를 통한 간접 호출 및 ambiguity/fallback에서 baseline 보존을 확인하는가 |

새 관계는 `IMPLEMENTED_BY`3 + `TESTED_BY`5 + `CHECKED_BY`5 =13개다. 기존12개와 합친 총25개 모두 proposed/검토자 없음이다. code 함수3개·test 함수5개·기준4개를 쓴다. CHECKED_BY는 시험 연결이며 사람이 criterion을 accepted/approved로 판정했다는 기록이 아니다. 이번에는 시험을 다시 실행하거나 semantic linked test report를 만들지 않았다.

코드 원문은 기존 source와 현행 바이트가 같고 실제 호출 문맥3span을 추가 대조했다. [정확 호출 문맥](../../system/eval/b0-knowledge-system/refactoring-n12-trace-proposals-2026-10-08/call-context-source-refs.json)은 alignment.go181–196, searcher.go356–366, validate_test.go88–109이다. 이는 원문 증거이며 CKG 호출 그래프의 완전성·runtime coverage 인증은 아니다.

## 부정확한 한 연결의 제외와 남은5개 경로

기존 P12-A03 자료의 `TestProjectionRejectsCrossSnapshotOrMissingEvidence`는 `Projection.Validate()`와 `ValidateSources()`를 직접 검증한다. 이것을 `ValidateDatasetAlignment TESTED_BY 해당 시험`으로 자동 연결하지 않았다. 해당 원문은 검토 문맥으로 남겼으며 별도의 코드 대상/범위가 확인되기 전에는 이번 관계 목록에 넣지 않는다. 기존 자료는 후보라는 지위로 그대로 보존한다.

[의미 경계 제어](../../system/eval/b0-knowledge-system/refactoring-n12-trace-proposals-2026-10-08/semantic-boundary-control.json)에서 이 부정확한 쌍은 같은 snapshot·정확한 code/test canonical·원문 SHA를 가지므로 생산 구조·원문·색인 검사를 통과했다. 이것은 의미를 검증하는 검사도 통과했다는 뜻이 아니다. 의도적으로 만든 control-only proposed 쌍을 실제 검토 projection/SQLite에 넣지 않았다. 구조 검사가 코드와 시험의 의미 관계를 결정하는 기능이라고 주장하지 않는다.

또한 모든 broad concept를 같은 함수에 자동 연결하지 않았다. **미작성5개 개념 경로**는 req-filtered-recall의 policy, req-source-evidence의 source-snapshot/semantic-assertion, req-ontology-fallback의 term/evidence-pack이다. 이미 작성한3개 해석도 사람 검토 대상이다. 13관계를 승인해도 이5개와 명세/개념 검토가 끝나지 않으면 전체 trace가 완성됐다고 판정하지 않는다. [누락 경로](../../system/eval/b0-knowledge-system/refactoring-n12-trace-proposals-2026-10-08/remaining-concept-path-gaps.json)를 유지한다.

## 실제 기술 검증과 이중 확인

- 생산 API로 retained source·동일 CKG/CKV 신원·정확 canonical·CKV chunk를 검증하고 `PutAlignedRetained`→Load→닫기→read-only 재열기의 projection 바이트 일치를 확인했다.
- 실제 LoadAlignedRetained의 Trace 결과는3요구사항 모두 **spec_unapproved/paths=[]**. Plan은 모두 review_spec/unconfirmed=true. proposed 관계가 VerifiedImplementations의 boost 후보로 나오지 않으며 current pointer도 없다.
- cloned proposal의 wrong canonical/뒤집힌 code-test/없는 criterion/criterion→code/legacy ACCEPTED_BY write/다른 snapshot/변조 span SHA/미검토 concept의 verified 구현/기존 immutable content 교체 **9제어를 거부**했다. 실패한 교체 뒤 원래 저장 바이트도 같다. 실제 사용자의 반려나 제품 장애를 재현했다는 뜻은 아니다.
- 기존 graph/vector/semantic DB와 신원·원문 등 권위 파일8개 SHA는 실행 전후 동일하다. 원래 source/version/store와 신규 store를 잠시 숨긴 상태에서 보관 자료 두 개만 복원해 새 proposed JSON·SQLite와 원문·canonical·chunk 및 spec_unapproved 진단을 재검증하고 원본을 복원했다.
- 새 [관계 layer archive](../../system/eval/b0-knowledge-system/refactoring-n12-trace-proposals-2026-10-08/trace-proposal-layer.tar.gz)는 **37,819bytes/6files**, SHA256 `45749424a19bff449b7b67a0b083dcf7a73510522f936dbd358dcc9d5b7a61ae`다. 이것만으로 source/index가 포함된 완전한 archive라고 주장하지 않는다. 기존 [held source/index archive](../../system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08/held-candidate.tar.gz), SHA256 `0c4403f1d7701fd18ef598b16ac99ee51b7087bf26f18a47bdc2deebfbff0ad0`가 함께 필요하다. 복원 경로를 명시적으로 사용했고 논리 source root 좌표는 이전 절대 경로를 유지한다. 운영 이식성/복구 수용은 별도다.

제품 소스 변경0, 새 embedding/model HTTP·DEV/FINAL 질의·current/semantic activation·patch attempt·linked test execution0, 실제 사람 판정0·활동시간null이다. 이전 전체 Go 회귀/25 Python 시험을 이번에 다시 실행한 것처럼 기록하지 않는다. 이번 실행은 실제 source에서의 proposed 관계/API·저장/재생 검증이다.

## N11 부족분에 대한 판정

이번13관계는 이미 노출된 N12 원문/기준의 검토 자료다. **새 독립 FINAL 13표본으로 사용하지 않는다.** N11은 기존 검토 대상13문항/8제안군, 중요군별 승인0을 유지한다.

| 중요군 | 실제로 확인한 부족 범위 | 다음 작업 |
|---|---|---|
| POLICY | P12-P01–04는 DEV/history overlap이며 실제 owner/시점·내용 판정이 없다. typed policy instance0, ADR2도 proposed | 기존 주장을 fresh FINAL로 재사용하지 않고, 코드 계약/조직 권위의 범주를 판정한 뒤 별도의 독립 정책 범위 보강 |
| TRACE | N12의3요구/4기준과 이번13proposed 관계는 독립 FINAL이 아니다. approved trace0·추가 개념 경로5개 미작성 | 관계의 의미/명세·개념 범위 검토, 필요한 연결 보강, 이후 독립 trace 질문/군 준비 |
| ABS | 기존 RTO/RPO·운영 담당자 부재 basis는 한 제안군이다. 문서의 null은 모든 조직 자료에 사실이 없다는 증명과 다르다 | 대상 corpus/권위/시점의 답 부재 판정 및 같은 운영 미지정의 재문장과 다른 독립 무답 범위 확보 |

N11 중요군별10개 승인 조건을 낮추지 않는다. 원문이 있다는 이유로 정책 권위·의미 관계·조직 전체의 부재를 승인한 것으로 바꾸지 않는다.

## 다음 작업·전체 잔여

**다음:** P12-T01–13의 타당성·수정/제외와 기존 N12 명세/개념·33항목/12관계 판정을 반영한다. 확인된 scope로 미작성5경로를 준비하고, 승인된 overlay/lock/trace 및 실제 N14 held patch·linked run·criterion decision을 단계별로 결합한다. N11 독립 표본·gold/오염 및 runtime 사전 동결도 별도로 완료해야 한다.

현재 **8/18**, 요청 N04–18 **5/15** 완료. 남은 전체10개는 **N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 실제 파일럿 검토, N14 실제 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 운영 복구, N18 최종 판정**이다. N12-A–D/N14-A–E 및 상위 작업의 완료 checkbox를 바꾸지 않았다.
