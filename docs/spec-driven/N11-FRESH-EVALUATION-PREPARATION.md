# N11 새 평가 입력 준비와 실행 잠금

2026-10-06 · **미완료 / 초안**. [새 DEV 후보7개](../../system/eval/b0-knowledge-system/refactoring-n11-input-draft-2026-10-06/dev-candidates.json)는 커밋076f6de2의 원문 파일SHA/줄/spanSHA에 연결했다. 유지보수 ancestor, 자원 예상량, current 내구성 오류, GC reader 보호, 전체 span 예산, 명시 catalog 선택 및 출처 검증을 묻는다. 질문·정답·cluster는 아직 사람이 승인하지 않았고 검색 실행 결과는 없다.

[프로토콜 초안](../../system/eval/b0-knowledge-system/refactoring-n11-input-draft-2026-10-06/protocol-draft.json)은 기존 승인된 모델/K/반복/arm/회전/안전·품질·지연 임계치를 그대로 복사하고 원본 프로토콜SHA를 기록한다. 모델 실서버/하드웨어/원천은 실행 전 다시 확인한다. 기존 FINAL8질문/F01–06 FINAL은 관측 이력이 있어 제외한다. 새 ID로 바꾼 같은 질문/사실/fixture도 독립 FINAL이라고 세지 않는다.

## 아직 필요한 입력

새 FINAL은 아직0단위다. 미승인 DEV7개를 FINAL로 바꾸지 않는다. CODE/WHY/POLICY/TRACE/ABS 각 중요군의 독립 FINAL 단위10개를 원문 gold/권위/범위/시점에 연결하고, 같은 사실의 번역·바꿔쓰기·같은 상태 변형·반복은 한 cluster로 묶어 검토한다. 질문 개수만50개로 늘려 충분한 독립 표본으로 주장하지 않는다. 특히 실제 TRACE/정책의 사실 범위는 [N12 원문 검토](./N12-ACTUAL-PILOT-REVIEW.md)와 연결하며, 테스트용 관계를 실제 정책/사람 수용으로 바꾸지 않는다.

전체 질문/gold/cluster·DEV/FINAL 분할과 비용 조건을 사전 검토한 뒤 바이트SHA·검토자·시점을 동결한다. 실제 결과가 관측되기 전까지 `execution_allowed: false`, `release_eligible: false`, `freeze: null`, `results: null`을 유지한다. 이후 질문/임계치/원문 사실을 바꾸면 새 DEV 버전으로 남기고 독립 FINAL을 회전한다. 중요군10독립단위 미만은 descriptive/inconclusive이며 기본 활성화나 출시의 근거가 아니다.

## 실행 전/후 확인

source snapshot·설정·lock·model digest/차원과 source/gold 파일·span SHA를 확인한다.8arm은 같은 입력/모델/원천을 쓰고 지연 실험을 병렬 실행하지 않는다. 검색/오류/incomplete 분모, strict 인용0 조건, 사람 답변 기권, 본문 예산 partial 진단을 별도로 센다. bootstrap10000/95%CI는 반복 행이 아니라 paired independent cluster를 단위로 한다. 외부 coding-agent 총비용이나 실제 사람의 검토 활동 시간은 도구 실행 시간·응답 대기로 추정하지 않는다.

N11-A–D는 아직 체크하지 않았다. 평가 입력 동결과 실제 사용 증거가 연결돼야 완료한다. 다음은 N12의 원문·후보 관계 판정 반영과 독립 FINAL gold 작성/검토이며, N08–10 튜닝은 그 뒤에 진행한다.
