# 사람 판정과 paired 비교 종료

2026-10-06 · B1-04/07/08 완료. 사용자가 **“HC-01–11 해석 승인. 시험 preview 범위로 종료.”**라고 결정했다. [승인 원장](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/human-approval.json)은 승인 전 표와 실제 DEV/FINAL 응답·measurement ID·원자료 SHA를 연결한다. 검토자 `chat-user`는 로컬 식별자다.

## 사람 판정의 단위와 결과

| 검토 범위 | 문항 | 실제 판정 |
|---|---|---|
| 개념 | HC-01 | proposed 다의어를 확정 의미로 승격하지 않는 해석 승인1/1 |
| 정책·구현·수용 경계 | HC-02/03/04/05/06/10/11 | 프로젝트 정책 격리·집행/수용 미입증·키/오류 경계 해석 승인7/7 |
| 이유·추적·답변 경계 | HC-04/06/07/08/11 | marker/산술 시험/순위/검색 miss/오류를 과잉 근거로 쓰지 않는 해석 승인5/5 |
| 사람 답변 기권 | HC-09/10 | 운영 담당자·실제 키 소유자를 추정하지 않고 보류하는 판단 승인2/2 |

범주는 중복된다. 총 검토 단위는 **해석 문장11개**, 승인11/11·승인된 해석 중 근거 범위 위반0/11이다. [집계](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/human-assessment.json)에 ID·분모를 기록했다. 제품은 생성 답변을 반환하지 않는다. 전체3,840/4,800 SDK 응답에 대한 생성 답변의 오개념·정책오용·근거 없는 이유 비율은 산출하지 않으며 `null`이다. 이 승인을 운영 정책 승인이나 자동 답변 기권 성공으로 해석하지 않는다. 검토 시간·금액은 `not_measured/null`이다.

## 자동 품질·실패·비용과 결합

[개발 보고](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md)의 정적 양성4문항 Recall0.25/MRR0.125, [독립 FINAL 보고](./C1-APPROVED-FINAL-REPORT.md)의 양성6문항 Recall1/6/MRR0.5/6은 그대로다. FINAL 전체4,800행 중 정상팩4,560·POLICY-03 오류240을 모두 분모에 유지했다. 정상팩 source/public Go 감사와 오류를 구분한다. ABS-01/02의 v1/v2 strict no-citation은 실패이며 사람 보류2/2와 별도다. F-05/06 MRR0.75·F-02 동점 차이·F-03 비승격·F-04 상태격리를 각 보고에 보존했다.

DEV 독립 양성 unit8, FINAL 양성 unit10(정적6+동적4가족), 기권2는 별도다. 중요 질문군별 독립 수는10 미만이며 **inconclusive**다. 반복·언어·상태·11개 해석으로 독립 수를 늘리지 않는다. 기존 paired/불확실 구간과 같은 축 비교는 원자료 보고에 보존했다. 작은 표본의 개선 이점이나 통계적 품질 합격을 주장하지 않는다.

[비용 보고](./B1-COST-REPORT.md)의 warm/cold·backend·크기·공유 환경·build/query 분리를 유지했다. DEV combined_on static/dynamic p95비1.292/1.614, FINAL dynamic concept_text_on/relations_on/combined_on의 baseline_off 대비1.533/1.311/1.511은1.25를 넘는다. 같은 팩 축·같은 ontology 축·pooled 모집단도 함께 보존하며 유리한 축을 사후 선택하지 않는다. FINAL 오류의 빠른 거부 시간은 성공 조건부 지연에 포함하지 않는다.

입력5개·제품 소스1,096개·동결 실행 버전·K·gold·임계치를 바꾸거나 FINAL 결과로 튜닝하지 않았다. HC 승인으로 기존 품질 fail·중요 그룹 inconclusive·ontology disabled·출시 보류를 바꾸지 않았다.

## 종료 판정

B1-04의 질문군 자동 지표와 사람 해석/기권을 위 분모로 결합했고, B1-07 실제 사람 판정을 기록했으며, B1-08 paired 실패·불확실·비용 보고를 종료했다. B1 공식 판정은 **fail**(비용 회귀와 품질 제한 유지), 중요 그룹은 inconclusive다. 작업 종료는 실패를 숨기거나 품질을 승인하는 결정이 아니다. 다음은 [최종 preview 종료 보고](./C1-FINAL-DELIVERY-REVIEW.md)의 전체 완료 재검증이다.
