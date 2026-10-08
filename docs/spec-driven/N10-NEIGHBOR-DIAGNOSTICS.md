# N10 neighbors 미해결 seed 진단

2026-10-06 · **진단 세부 단계 검증 / N10 전체 미완료**. 새 독립 품질·p95·전체 운영 부하 평가는 아직 없다.

## 목적과 바뀐 동작

neighbors의 원문 인용을 그래프 심볼로 해석하지 못하면 기존 코드는 문자열 오류만 반환하고 계측은 backend_error로 기록했다. 구조 DEV에서 이 경우를 저장소 조회 장애, 정상적인 빈 traversal과 구분할 typed 정보가 없는 것을 재현했다. [RED 관측 기록](../../system/eval/b0-knowledge-system/refactoring-n10-neighbor-diagnostics-2026-10-06/red-observed.json).

CKG adapter가 좌표를 심볼로 해석하지 못하면 기존 ErrSeedUnresolved를 감싸 반환한다. 이것은 실제 오류다. 문서가 원래 그래프 확장 대상이 아니었는지, 코드 색인이 누락됐는지, 좌표가 틀렸는지까지 이 sentinel이 판정하지 않는다. NodesByFilePath의 실제 저장소 오류는 그 원인을 그대로 반환하며 정상적으로 찾은 심볼의 이웃0개와도 구분한다.

계측의 outcome=backend_error를 유지하고 CKG/neighbors의 typed 미해결 오류에만 선택형 error_kind=seed_unresolved를 추가한다. 실패 호출·결과 수·elapsed·source argument SHA·순서와 기존 취소/deadline 분모를 보존한다. 문자열 오류 내용을 검색해서 분류하지 않는다. raw 오류 메시지·경로·질문은 진단 필드로 내보내지 않는다.

## 보고서와 이전 자료

matrix 요약기는 선택형 진단을 classified_error_kinds로 집계하며 nonreturned_attempts를 그대로 유지한다. 진단이 없던 기록은 classified_error_kinds=null이고 nonreturned_without_typed_diagnostic으로 남는다. null은 미해결 seed가0개였다는 사실이 아니다. returned·다른 backend/method에 붙은 seed 진단이나 미지원 진단 값은 거부한다.

이전 FINAL 및 source argument를 남기지 않은 역사적 neighbors 오류를 재분류하거나 새 원인으로 소급 단정하지 않는다. 과거의20개 source-bound DEV 직접 재생과 그12개 no-node 결과는 해당20개에만 유효하다. 이번3개 fixture 제어가 과거360개 오류 전체를 진단한 것이 아니다. 원 자료와 기존 승인 입력5개는 그대로 보존한다.

## 검증

[새 진단 원자료](../../system/eval/b0-knowledge-system/refactoring-n10-neighbor-diagnostics-2026-10-06/diagnostic-manifest.json)에 다음을 연결한다.

- adapter의 미해결 좌표/저장소 실패/정상 빈 traversal과 계측의 typed/일반 오류·취소·deadline·다른 method·정상 반환을 검증했다.
- generic 오류와 typed 오류를 넣은 composer 출력의 seed·점수·실패 seed·neighbors가 동일했다. 동일2방향 시도는 모두 backend_error이며 raw PRIVATE 표지는 계측 JSON에 없었다.
- 관련 CKG client/계측/Stage3 3개 패키지 race와 vet, 보고서 Python17개, 계층 경계 검사를 통과했다. 보고서 추가 진단이 인용 품질·반복/언어/오류·타이밍 분모를 바꾸지 않음을 확인했다.
- 실제 Darwin CKG CLI로 작은 Go/Markdown fixture의 SQLite graph를 빌드했다. 현행 adapter/recorder로3호출을 수행해 미해결1·정상 빈 결과1·닫힌 store 오류1을 확인했다. 실패2는 모두 nonreturned로 남고 graph 파일 SHA는 전후 동일했다. fixture 원문·driver·계측 JSON·binary SHA를 보관했다.

native probe는 standalone graph 제어다. pinned v2 source/모델·CKV·실제 BGE 검색·최종 gold·운영 규모/다른 OS의 합격 증거가 아니다. elapsed는 제어 호출의 시간이며 paired p95비나 비용 절감으로 집계하지 않는다.

## 남은 수용 조건

N10의 실제 비용/지연 분해와 개선 판정은 N11에서 검토·동결한 새 입력/source/model/runtime이 필요하다. 같은 pack/ontology8arm·회전/직렬 warm/cold, 성공/오류/incomplete 분모·실제K/MID/호출/크기 및 p95비1.25 기준으로 새 실행을 평가해야 한다. 이번 진단 코드를 넣었다고 성능 개선이나 N10 전체 완료를 표시하지 않는다.

현재 요청15개 중5완료, 잔여 N08/09/10/11/12/14/15/16/17/18 10개. 다음은 실제 DEV/프로토콜·N12 판정 반영 및 독립 FINAL 검토 후 새 비용·지연 실행이다.
