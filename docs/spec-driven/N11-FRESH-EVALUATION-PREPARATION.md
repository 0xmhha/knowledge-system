# N11 새 평가 입력 준비와 실행 잠금

2026-10-06 · **미완료 / 초안**. [새 DEV 후보7개](../../system/eval/b0-knowledge-system/refactoring-n11-input-draft-2026-10-06/dev-candidates.json)는 커밋076f6de2의 원문 파일SHA/줄/spanSHA에 연결했다. 유지보수 ancestor, 자원 예상량, current 내구성 오류, GC reader 보호, 전체 span 예산, 명시 catalog 선택 및 출처 검증을 묻는다. 질문·정답·cluster는 아직 사람이 승인하지 않았고 검색 실행 결과는 없다.

[프로토콜 초안](../../system/eval/b0-knowledge-system/refactoring-n11-input-draft-2026-10-06/protocol-draft.json)은 기존 승인된 모델/K/반복/arm/회전/안전·품질·지연 임계치를 그대로 복사하고 원본 프로토콜SHA를 기록한다. 모델 실서버/하드웨어/원천은 실행 전 다시 확인한다. 기존 FINAL8질문/F01–06 FINAL은 관측 이력이 있어 제외한다. 새 ID로 바꾼 같은 질문/사실/fixture도 독립 FINAL이라고 세지 않는다.

## 아직 필요한 입력

새 FINAL은 아직0단위다. 미승인 DEV7개를 FINAL로 바꾸지 않는다. CODE/WHY/POLICY/TRACE/ABS 각 중요군의 독립 FINAL 단위10개를 원문 gold/권위/범위/시점에 연결하고, 같은 사실의 번역·바꿔쓰기·같은 상태 변형·반복은 한 cluster로 묶어 검토한다. 질문 개수만50개로 늘려 충분한 독립 표본으로 주장하지 않는다. 특히 실제 TRACE/정책의 사실 범위는 [N12 원문 검토](./N12-ACTUAL-PILOT-REVIEW.md)와 연결하며, 테스트용 관계를 실제 정책/사람 수용으로 바꾸지 않는다.

전체 질문/gold/cluster·DEV/FINAL 분할과 비용 조건을 사전 검토한 뒤 바이트SHA·검토자·시점을 동결한다. 실제 결과가 관측되기 전까지 `execution_allowed: false`, `release_eligible: false`, `freeze: null`, `results: null`을 유지한다. 이후 질문/임계치/원문 사실을 바꾸면 새 DEV 버전으로 남기고 독립 FINAL을 회전한다. 중요군10독립단위 미만은 descriptive/inconclusive이며 기본 활성화나 출시의 근거가 아니다.

## 실행 전/후 확인

source snapshot·설정·lock·model digest/차원과 source/gold 파일·span SHA를 확인한다.8arm은 같은 입력/모델/원천을 쓰고 지연 실험을 병렬 실행하지 않는다. 검색/오류/incomplete 분모, strict 인용0 조건, 사람 답변 기권, 본문 예산 partial 진단을 별도로 센다. bootstrap10000/95%CI는 반복 행이 아니라 paired independent cluster를 단위로 한다. 외부 coding-agent 총비용이나 실제 사람의 검토 활동 시간은 도구 실행 시간·응답 대기로 추정하지 않는다.

N11-A–D는 아직 체크하지 않았다. 평가 입력 동결과 실제 사용 증거가 연결돼야 완료한다. 다음은 N12의 원문·후보 관계 판정 반영과 독립 FINAL gold 작성/검토이며, N08–10 튜닝은 그 뒤에 진행한다.

## 검증한 입력 검사 도구

`scripts/refactoring-eval-input-check.py`는 source commit·gold 파일/span SHA, 기존 승인 프로토콜SHA·모델/arm/임계치, 별도 사람 검토 기록과 protocol/questions의 동결SHA·시각을 대조한다. 케이스별 gold/독립성/오염 검토와 unit/cluster를 명시해야 한다. 같은 unit을 여러cluster로 쪼개거나 같은 prompt를 여러독립단위로 세는 경우, DEV/FINAL cluster 중복, 선언된 이전FINAL 사실 재사용, 알려진 관측FINAL cluster 재사용, 관측 후 동결과 동결 뒤 질문/프로토콜 변경을 거부한다. 중요군은 반복 행 수가 아닌 검토된 FINAL cluster 수로 센다.9단위를 번역/반복해18행으로 만들어도 n=9/inconclusive다.

이 도구는 입력의 일관성 검사다. 검토자의 실체나 gold/독립성 판정의 진실성을 인증하지 않고 실제 검색을 실행하지 않는다. `input_ready`가 참이어도 live model/통합 dataset·binary/source/runner 관측상태 검사는 별도이므로 `execution_ready`와 `product_release_approved`는 항상false다. 입력 체크 결과를 제품 품질이나 출시 승인으로 읽지 않는다.

[입력 검사 증거](../../system/eval/b0-knowledge-system/refactoring-n11-input-controls-2026-10-06/input-control-manifest.json)는10개 구조 검증 시험과 현재 실제 초안의 CLI 거부(exit2)를 연결한다. 시험의 승인자·50개 케이스는 검증기용 합성 control이며 실제 사람 승인·독립 FINAL이 아니다. 현재 CODE/WHY/POLICY/TRACE/ABS의 승인된 FINAL 수는 모두0이다. N11-C의 입력 검사 부분을 구현했지만 전체 수용 조건은 미완료다.

[알려진 관측 목록](../../system/eval/b0-knowledge-system/refactoring-n11-input-controls-2026-10-06/known-final-observations.json)은 기존 승인 FINAL4800행의14개 질문/fixture 묶음을 원자료에서 추출했다.9개 원시 rows SHA가 기존 corrected summary와 모두 일치했다. 시각은 이 승인 FINAL 수집에서 가장 먼저 보관된 응답 완료 시각이고 전체 과거 실험의 최초 시각이라고 주장하지 않는다. 다른 과거 관측/사실도 사전 오염 검토에 포함해야 한다. 원자료를 다시 질의하거나 새 평가 결과로 합산하지 않았다.

현재 초안 확인 명령(미승인/FINAL0이므로 exit2가 정상):

```bash
python3 scripts/refactoring-eval-input-check.py \
  --protocol system/eval/b0-knowledge-system/refactoring-n11-input-draft-2026-10-06/protocol-draft.json \
  --questions system/eval/b0-knowledge-system/refactoring-n11-input-draft-2026-10-06/dev-candidates.json \
  --observations system/eval/b0-knowledge-system/refactoring-n11-input-controls-2026-10-06/known-final-observations.json \
  --require-ready
```

검토 기록은 별도 `--review FILE`로 준다. schema는 status=approved, scope=fresh-evaluation-inputs, reviewer, timezone이 있는 reviewed_at/frozen_at, protocol_sha256/questions_sha256, cases 객체다. 각 cases[questionID]는 승인한 unit_id/cluster_id와 gold_approved/independence_approved/contamination_checked=true를 가진다. 질문은 승인된 unit_id/cluster_id·review_state/source_commit과 split=DEV/FINAL을 갖는다. ABS는 빈 gold evidence와 strict_zero_citations=true 및 abstention_basis가 필요하다. 실제 판정 전 이러한true값을 채우지 않는다. 작은 표본의 입력 검토가 끝나도 sampling_verdict=inconclusive이며 실제 목표의 합격을 주장할 수 없다.

2026-10-06T07:03:38Z의 [live 식별 확인](../../system/eval/b0-knowledge-system/refactoring-n11-input-controls-2026-10-06/live-model-identity.json)에서 BGE-M3 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`/1024차원/runtime options가 승인 입력과 일치했고 서버는Ollama0.35.1이었다. 식별 문장1회이며 평가 질문 실행0, unload 요청0이다. 이 시점의 신원 확인이므로 실제 실험 시작/종료 시 다시 확인하고 데이터셋·바이너리 결합과 runner 관측 보호를 추가해야 한다. 검색 품질·비용·지원 환경 합격으로 합산하지 않는다.

[개발7개 부분 검토 자료](./N11-DEV-INPUT-REVIEW.md)에 기존 질문/후보 답/출처·사실 묶음과 보존된 프로토콜 조건을 정리하고 판정을 요청했다. 원 JSON·source/spanSHA는 불변이다. 새 FINAL0이므로 전체 평가 승인으로 요청하지 않았고 부분 검토만으로 execution_ready나 N11 완료 상태를 바꾸지 않는다.

[실행 보호 경로](./N11-EVALUATION-RUNNER-CONTRACT.md)를 추가했다. 기존 input checker는 기록 일관성 검사이고 execution_ready=false를 유지한다. 새 runner가 전체 승인 기록과 runtime pin·scope를 확인하고 FINAL_reserved를 내구성 저장한 뒤 실제 matrix를 호출한다. 예약 이력은 실제 응답 관측이라고 표시하지 않으며, 현재 초안은 실행 불가/새 FINAL0이다.
