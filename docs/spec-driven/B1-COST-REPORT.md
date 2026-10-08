# B1 비용과 미측정 항목 보고

2026-10-06 · B1-06 보고 완료. [비용 원장](../../system/eval/b0-knowledge-system/report-closure-reaudit-m2max-2026-10-06/cost-report.json)은 DEV3840/FINAL4800의 warm p50/p95·process cold·backend 호출·응답 크기·같은 축 비교를 기존 원자료 SHA에 연결한다. 원래 완료 조건은 “순서와 원시 시간 보관; build/query 별도; 환경 경쟁 작업 기록”이며 바꾸지 않았다. [조건 재검토](../../system/eval/b0-knowledge-system/report-closure-reaudit-m2max-2026-10-06/completion-condition-review.json)를 보존했다.

## 측정한 비용

- [개발 보고](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md) 및 [최종 보고](./C1-APPROVED-FINAL-REPORT.md)의 정적/동적/pooled·8arm 원시 지연을 각각 보존했다. 같은 ontology/pack 축 비교도 유지했다. 유리한 모집단만 선택해 합격을 만들지 않는다.
- FINAL 정적 warm은 계획160 중 성공140/arm, cold는24 중 성공21/arm이다. 동적 warm240/cold36은 모두 수집됐다. 실패한 계획240행은 품질 분모에 남고 성공 조건부 지연과 구분한다.
- cold는 resident 모델과 새 CKS 프로세스다. 모델을 언로드하지 않았으며 모델/daemon cold-load 비용은 미측정이다. query latency에 strict build 시간·후속 감사·추가 진단을 섞지 않는다.
- 논리 backend 시도와 Ollama HTTP 호출을 measurement ID에 별도로 연결했다. SDK 크기는 canonical compact JSON 바이트이며 wire/frame 크기와 구분한다. 선택적 backend 오류도 호출 수에 남긴다.
- 공유 M2 Max 환경·회전 arm·동시 formal 지연 시험 없음·입력/모델/바이너리 전후 신원을 기록했다. 물리 자원 독점을 주장하지 않는다.
- [최신 패키지 진단](./C1-FINAL-PACKAGE-VALIDATION.md)의 전체 strict1598/13820·단일DEV60 SDK/별도RSS2는 별도 비용 자료다. 이를 공식 FINAL의 추가 독립 표본이나 지연 회귀 합격으로 합산하지 않는다.

## 측정하지 않은 비용

| 항목 | 값 | 의미 |
|---|---|---|
| 사람 검토 시간 | `not_measured`, 초 값 `null` | 실제 시간이 제공되지 않음 |
| 사람 검토 금액·통화 | `not_measured`, 값 `null` | 실제 비용 자료가 없음 |
| 사람 검토 절감 효과 | `null` | 비교 이점 주장 불가 |
| 모델/daemon cold-load | 미측정 | 사용자 모델을 언로드하지 않음 |

미측정이라는 사실을 보고하는 데 추가 승인은 필요하지 않다. 이 기록의 작성자는 에이전트이고 실제 사람 검토나 비용 승인 기록이 아니다. null을 0초/0비용으로 바꾸지 않았다. 이후 실제 값이 제공되면 새 provenance 기록에 반영할 수 있지만 현재 없는 측정을 만들지 않는다. 기존 frozen DEV/FINAL 보고와 5개 승인 입력은 수정하지 않았다.

B1-06의 보고 완료는 비용 목표 합격이나 사람의 주장 검토 완료와 다르다. B1-04/07/08의 오개념·정책 오용·근거 없는 이유·기권에 대한 실제 사람 판정은 미완료다. 온톨로지 기본 disabled·품질fail·소그룹inconclusive·운영출시보류를 유지한다.
