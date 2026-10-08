# 승인 입력의 실제 개발 출력: 주장·정책·기권 검토

2026-10-06 · B1-07 · HC-01–11 해석 승인 완료. 입력 승인과 출력 판정은 구분한다.

제품은 생성 답변을 반환하지 않고 인용·본문·검토된 의미 문맥을 반환한다. 아래는 실제 SDK 출력과 승인 원문으로부터 작성한 **검토용 해석 문장**이다. 사용자가 아래 해석 문장을 승인했다. 이를 제품이 자동 생성한 답변으로 기록하지 않는다. [실제 응답 4개와 measurement ID](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/human-claim-review/manifest.json)는 FIX-25 개발 캡처에 결합한다. FIX-26/27에서 순위·읽기 비용을 수정하지만 소스 사실·정책·승인 범위는 바꾸지 않는다. 최종 점수/출시 승인과도 별개다.

| ID | 실제 근거와 검토용 해석 | 판정 |
|---|---|---|
| HC-01 | F-03의 여러 `core` 개념은 proposed다. 한 의미를 확정 정책/구현으로 선택하지 않는다. 실제 온톨로지 가산 0, 기본 인용·본문 유지. “뜻을 더 지정해야 한다”는 해석은 가능하지만 자동 정책 승인/답변 기권 성공을 주장하지 않는다. | 승인 · chat-user |
| HC-02 | F-05-A의 `docs/policy.md:3`, 해당 프로젝트의 검토 정책: 상한 10. B의 상한 20을 A에 적용하지 않는다. | 승인 · chat-user |
| HC-03 | F-05-B의 `docs/policy.md:3`, 해당 프로젝트의 검토 정책: 상한 20. A의 상한 10을 B에 적용하지 않는다. | 승인 · chat-user |
| HC-04 | F-05-A/B의 `main.go:3`의 Alpha는 프로젝트 marker를 반환한다. 정책 상한을 실제로 집행하거나 준수한다는 근거가 아니다. | 승인 · chat-user |
| HC-05 | F-06 `main.go:3`에서 RefundLimit은 20을 반환하고 `README.md:3`/검토 요구는 10 이하다. 원문을 사람이 대조하면 불일치다. DTO가 자동 수치 충돌 판정까지 했다고 주장하지 않는다. | 승인 · chat-user |
| HC-06 | F-06 `main_test.go:5`의 TestHealth는 관련 없는 산술 검사다. 테스트 성공을 환급 상한 acceptance 승인으로 사용하지 않는다. 실제 test/trace link는 비어 있다. | 승인 · chat-user |
| HC-07 | F-06에는 reviewed ADR→requirement→criterion→code→test 전체 경로가 없다. implemented behavior/trace의 미확정 상태를 유지하며 IMPLEMENTED_BY 순위 근거만으로 완전 추적 경로를 주장하지 않는다. | 승인 · chat-user |
| HC-08 | 정적 DEV v1에서 CODE/POLICY/TRACE의 기대 근거를 못 찾았다. 이 출력만으로 기대 답을 입증했다고 판단하지 않는다. WHY는 Promote의 기대 근거를 찾았다. 인용 0개 조건과 사람이 답변을 보류할지는 별도 판정한다. | 승인 · chat-user |

HC-02–07의 “검토 정책/요구”는 승인된 synthetic 테스트 범위다. 운영 정책·criterion acceptance·법적 적합성은 승인하지 않는다. 기계 검증은 원문·좌표·해시/프로젝트 격리와 계약을 확인하고, 사람 판정은 해석 문장이 그 근거를 넘지 않는지 확인한다.

검토 기록에는 승인/수정 ID, 로컬 검토자 식별자, 승인 기록 시각, 측정한 검토 시간·비용(가능할 때)을 남긴다. 이 표를 읽은 시간을 자동 추정하거나 사람이 재지 않은 비용을 0으로 채우지 않는다. 시간·비용을 측정하지 않았다면 `unavailable/not_measured`로 명시한다. 실제 FINAL 응답은 아래 추가 검토 범위에 결합했다.

## FINAL 실제 출력의 추가 검토

[대표 실제 응답 3건](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/final-human-claim-review/manifest.json)은 기술 동결한 FINAL의 baseline_off·retrieval 1회 원행·measurement ID에 연결한다. 최종 정적 8arm 전체 원자료는 [FINAL 보고](./C1-APPROVED-FINAL-REPORT.md)에 보존했다. 입력 승인만으로 아래 사람 판정을 채우지 않는다.

| ID | 실제 관측과 검토할 해석 | 판정 |
|---|---|---|
| HC-09 | ABS-01 실제 출력에는 인용이 있으나 운영 가스 환불 정책의 승인 담당자를 입증하는 근거가 없다. 담당자 이름을 추정하지 않고 답변을 보류해야 한다. 인용0개 guard 실패와 사람의 답변 보류를 각각 기록한다. | 승인 · chat-user |
| HC-10 | ABS-02 실제 출력의 시험 preview 서명 근거를 production key나 실제 private key holder로 해석하지 않는다. 운영 키/소유자 답변을 보류해야 한다. v1은 인용25개, v2도 인용이 있어 strict no-citation 실패다. | 승인 · chat-user |
| HC-11 | POLICY-03은 모든8arm에서 v2_evidence_failed로 종료돼 정상 정책 근거 팩을 반환하지 않았다. 실제 빌더 진단은 retained body 33,104바이트가 32,000 상한을 넘었음을 확인한다. 오류만으로 비공개 정책 내용/ID를 공개하거나 접근을 허용하는 답변을 만들 수 없으며, 해당 응답의 정책 답변 근거는 미제공이다. 제품의 권한 회귀 시험과 이 검색 실패를 구분한다. | 승인 · chat-user |

HC-09/10은 시스템이 생성한 답변이 아니라 사람이 실제 EvidencePack으로부터 내릴 판단이다. 승인된 후보 기권 답과 같은 방향이지만 실제 출력에 대한 아래 해석의 적정성을 사용자가 승인했다. HC-11의 진단이 기존 승인 gold나 정책 규칙을 바꾸지 않는다. 사람 검토 시간·비용은 계속 미측정이며 `not_measured` 상태를 0 비용·개선 이점으로 바꾸지 않는다.

## 실제 사람 판정 기록

사용자 원문: **“HC-01–11 해석 승인. 시험 preview 범위로 종료.”**. 로컬 검토자 `chat-user`, 기록 시각 `2026-10-06T01:17:16.923743+00:00`. 정확한 메시지 수신 시각과 실제 검토 소요 시간은 측정하지 않았다. [승인 원장](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/human-approval.json)은 승인 전 표 SHA·11개 해석·실제 응답/measurement ID·개발/최종 증거 manifest에 결합한다. 이전 pending 원장은 당시 기록으로 보존했다.

검토한 해석11/11 승인, 이 승인된 해석 중 근거 범위 위반0/11이다. [사람 판정 집계](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/human-assessment.json)의 개념1·정책/수용 경계7·이유/추적 경계5 범주는 중복을 포함한다. HC-09/10의 사람 답변 보류2/2를 승인했으며 기계의 strict no-citation 실패는 그대로다. 이11개는 독립 최종 품질 문항11개나 모델 생성 답변11개가 아니다. 전체3840/4800응답의 오개념·정책오용·근거 없는 이유 비율은 생성 답변이 없어 산출하지 않으며 미판정 응답을0건으로 만들지 않는다.

승인 범위는 해석과 시험 preview 종료다. 운영 정책/키/역할·법적 적합성·품질 합격·출시/배포·새 Git push를 승인한 것으로 확장하지 않는다.
