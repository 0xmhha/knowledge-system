# N12 검토 판정의 결합·활성화·linked run

2026-10-09 · **N12 수용 조건 A–D 충족(검토자: 사용자 지정 AI 검토 라벨) / N12 완료**. 전체9/18, 요청6/15, 남은9개.

## 1. 판정과 검토자의 성격

사용자는 N12 사전검토 65건(관계·주장·기존 관계35, 개념·요구·기준30)을 채택했고, 검토자를 “opus5.5 검토”로 입력하라고 지시했다([지정 기록](../../system/eval/b0-knowledge-system/refactoring-n12-reviewed-activation-2026-10-09/review-designation.json)). 내용 판단은 Claude Opus 5.5의 기술 사전검토이고, 사용자는 이를 일괄 채택했다. 이 문서와 모든 원자료는 `reviewer_kind=ai_model_label_designated_by_user`, `independent_human_content_review=false`를 기록한다. N12-A의 "사람이 승인하고 검토자로 기록"은 사용자의 채택과 사용자가 지정한 라벨로 충족했다. 독립된 사람의 항목별 내용 검토는 아니다. 이 한계는 N14·N18 판단에 그대로 넘어간다.

## 2. 무엇을 결합했는가

- 검토된 파일: [ontology-pilot-v2](./ontology-pilot-v2.yaml)(core20), [ontology-pilot-functional-v2](./ontology-pilot-functional-v2.yaml)(23), [spec-pilot-v3](./spec-pilot-v3.yaml)(요구3 version3, 기준4). 모두 `status: verified`, `reviewed_by: "opus5.5 검토"`. [PILOT-SCOPE](./PILOT-SCOPE.json)가 이 파일들을 선택한다. 이전 파일과 이전 후보는 바꾸지 않았다.
- 새 captured source `0b9085e7`(1,441파일; 원 저장소 `985e22d8`에서 독립 Git으로 구성), snapshot `813b80a9…0f83`, dataset `daf9608b…3d4f`.
- 실제 CKG65Go·CKV68(Go65+MD3), 765chunks, canonical floor1.0. 선택76파일과 실제 색인 경로·SHA가 일대일로 맞는다. 전체 Go 감사는 build1,055/DB65/missing990/extra0 **DRIFT**이며, 누락은 선택 범위 밖이다.
- 검토된 projection: 개념23·요구3·주장6 verified, 관계43 verified(새 trace 관계31 = F3-T01–17과 F3-R01–14, 기존 SUPPORTS/ABOUT12). F3-T08/T11은 `ValidateCodeAnchors`로 대상을 바꿨고, about:P12-P01은 `dataset`으로 바꿨다. 실제 CKG canonical anchor20, 원문 span83, CKV 문서절27/27 연결.
- knowledge lock: 채택 범위에 승인된 overlay 항목(ADR/정책 instance)이 없다. 정책 담당자가 정해지지 않아 정책 instance는 0이다. 그래서 이번 결합은 semantic projection까지이며, lock에 새로 넣을 승인 항목은 없다.

## 3. 확인 방법과 결과

1. 생산 API로 retained source·좌표·CKG anchor·CKV chunk link·canonical coverage를 검증했다. 변조 6종을 거부했다(외부 snapshot 근거, 바뀐 digest, 없는 canonical, 외부 CKV chunk, 검토자 없는 verified 관계, 검토자 없는 verified 개념).
2. 소유 held store에 `PutAlignedRetained`→`ActivateAlignedRetained`로 저장·활성화했다. 읽기 전용으로 다시 열어 저장본과 current가 projection과 같은 바이트임을 확인했다. Trace는 세 요구 모두 **linked**다. `VerifiedImplementations`는 개념별로 1/5/2개를 돌려준다. 이것은 소유 held 후보에서의 활성화이며, 사용자 운영 데이터셋의 활성화가 아니다.
3. **실제 semantic linked run**: 생산 CLI `cks semantic test --go-test-exact`로 CHECKED_BY 10쌍을 실행했다. 9개는 command·test pass, snapshot 일치였다. F3-T10은 "reviewed target이 아님"으로 거부되었다. F3-T08 대상 변경 뒤 `ValidateDatasetAlignment`가 개념의 verified 구현이 아니므로, 좌표만 바꾸는 부분 제어에는 검토된 trace 경로가 없다. 사전검토 판단과 일치하는 결과다. CLI는 원인을 `operation_failed`로만 보여 주어, 같은 API를 직접 호출해 원인을 확인했다([요약](../../system/eval/b0-knowledge-system/refactoring-n12-reviewed-activation-2026-10-09/linked-runs-summary.json)).
4. 보관본 [held-reviewed-candidate.tar.gz](../../system/eval/b0-knowledge-system/refactoring-n12-reviewed-activation-2026-10-09/held-reviewed-candidate.tar.gz)(14,009,812byte/1,470파일; source/history, 실제 CKG/CKV DB, 검토된 SQLite, linked run 보고서 포함). 원래 source·dataset·store를 숨긴 채 재생했고, projection·관계·trace 바이트와 모든 권위 파일 SHA가 같았으며 활성 상태도 유지됐다. 운영 이식성 수용 시험은 아니다.

모델 호출: 색인 embedding HTTP72/입력765, 시작 probe2, 실패0. query DEV/FINAL은0이다. 색인 비용을 검색 지연이나 검토 시간으로 쓰지 않는다.

## 4. 상태 구분

| 구분 | 상태 |
|---|---|
| test pass | 관련 시험 통과(앞 단계 race 포함) |
| semantic linked run | 9/10 실행·통과, 1건 검토 경로 없음으로 거부 |
| relation verified | 관계43·개념23·요구3·주장6, 검토자 라벨 “opus5.5 검토” |
| criterion accepted | 0. 기준별 수용 결정(`patch.Decide`)은 N14의 범위다 |

## 5. N12-C: 검토 결과와 비용

검토 결과는 [관계 사전검토](./N12-CRITERION-GAP-PREREVIEW.md), [core 사전검토](./N12-CORE-PREREVIEW.md)에 남겼다. 추출 오차와 관계 오용은 수정 25건(관계 대상 오류2, 범위 과장4, 문구 과장·충돌·미지원 outcome 등)이다. 사람 활동 시간은 측정하지 않았으므로 null이고, 대기 시간으로 대신하지 않는다. AI 검토 비용(토큰)도 이 저장소에서 측정할 수 없어 null이다.

## 6. N14로 넘기는 실제 의미 수용 계획

N14는 이 검토된 dataset `daf9608b`를 기준으로 한다.

1. 실제 변경 하나를 고른다(후보: F3-T10처럼 검토 경로가 없는 기준을 채우는 시험 추가, 또는 ac-filtered-recall의 "eligible > K" 경계 보강).
2. 변경 전후 source를 hold-for-review 후보와 immutable patch attempt로 만든다.
3. 검토된 trace의 exact test를 `cks semantic test`로 실행해 snapshot에 묶인 run을 남긴다.
4. 기준별 Given/When/Then 수용을 `patch.Decide`로 기록한다. 검토자 종류는 이번과 같이 표시한다.
5. 재색인·승격 또는 반려·rollback과 원문 신원 재생을 확인한다.

- [x] N12-A 판정·검토자 기록(사용자 지정 AI 검토 라벨, 일괄 채택).
- [x] N12-B 새 capture·snapshot·CKG canonical·semantic projection 결합과 재검증(승인 overlay 항목0이라 lock에 새 항목 없음).
- [x] N12-C 검토 결과 기록, 활동 시간·비용 미측정 null.
- [x] N12-D 현행 결합과 N14 계획, pass/linked/verified/accepted 분리.

**남은 전체9개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
