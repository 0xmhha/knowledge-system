# N11 독립 FINAL 후보의 부분 준비

2026-10-06 · **후보 작성 / 미승인·미실행 / N11 전체 미완료**. 새 후보4문항은 CODE2/WHY2이며 같은 사실을 묻는 CODE/WHY 쌍은 각각 한 묶음으로 제안했다. 따라서 후보 사실 묶음은2개이고, 승인된 독립 FINAL은 여전히0개다. 중요한 다섯 군의10단위 조건을 충족하지 않는다. 기존 DEV7개·프로토콜 원본 바이트와 과거 FINAL은 변경하지 않았다.

[부분 질문 책](../../system/eval/b0-knowledge-system/refactoring-n11-final-candidate-preparation-2026-10-06/partial-question-book.json)은 DEV7개를 별도 복사하고 새 후보4개를 더했다. 모든 원문을 커밋 f066e67be6f3fd5df23bf46a8027db6d9504606e에 연결했으며 파일·inclusive 줄 span SHA가 일치한다. 기존 DEV의 질문/후보 답/원문 범위는 동일하다. 이전 부분 검토에 응답이 와도 이를 이 새 책/프로토콜 전체의 승인으로 자동 전환하지 않는다.

[부분 프로토콜](../../system/eval/b0-knowledge-system/refactoring-n11-final-candidate-preparation-2026-10-06/partial-protocol-draft.json)은 이전 승인 모델·K·8arm·반복·회전·품질/지연 조건을 유지한다. execution_allowed=false, release_eligible=false, freeze/results=null이다. source commit과 새 책 SHA를 명시한 새 초안이며 실제 dataset/config/binary 실행 동결이 아니다.

## 후보 질문과 원문

### N11-FINAL-CAND-01 · CODE

질문: 오래된 schema의 semantic 저장소를 조회용으로 열 때 생성·migration 대신 어떤 경계에서 거부하나요?

후보 답: OpenStoreReadOnly가 mode=ro로 열고 PRAGMA user_version을 StoreSchemaVersion과 비교한다. 불일치하면 DB를 닫고 오류를 반환하며 조회 중 migration하지 않는다.

제안 사실 묶음: semantic-readonly-open. 원문: [internal/system/semantic/readonly.go:11–36](../../internal/system/semantic/readonly.go). span SHA256 b99d80ca270feda64acca16fc1928f4a3cb26c0587001ec07b78e6a564feab2f.

### N11-FINAL-CAND-02 · WHY

질문: semantic 조회용 open 경로를 쓰기용 OpenStore와 분리한 이유는 무엇인가요?

후보 답: 평가·조회 입력을 생성하거나 migration하지 않기 위한 경계다. 읽기 전용 모드와 schema 확인을 사용하고, 옛 저장소 migration은 운영자가 별도 OpenStore 경로에서 수행한다.

제안 사실 묶음: semantic-readonly-open. 원문: [internal/system/semantic/readonly.go:11–36](../../internal/system/semantic/readonly.go). span SHA256 b99d80ca270feda64acca16fc1928f4a3cb26c0587001ec07b78e6a564feab2f.

### N11-FINAL-CAND-03 · CODE

질문: knowledge manifest의 알 수 없는 YAML 필드와 두 번째 YAML 문서는 어느 단계에서 거부하나요?

후보 답: ReadManifest가 KnownFields(true)로 첫 문서를 읽고 다시 Decode한 결과가 io.EOF인지 검사한다. 첫 문서의 알 수 없는 필드 및 추가 문서는 pack_incompatible 오류로 거부한다.

제안 사실 묶음: knowledge-manifest-parser. 원문: [internal/system/knowledgepack/lock.go:55–76](../../internal/system/knowledgepack/lock.go). span SHA256 8d2ded6423d7ea0908dc08422dcaf1cdae408b223e12adb4902eb01849f6ff8b.

### N11-FINAL-CAND-04 · WHY

질문: knowledge manifest를 한 YAML 문서와 알려진 필드로 제한하는 구현상의 목적을 어떻게 설명할 수 있나요?

후보 답: 명시된 manifest 구조 밖의 필드나 추가 문서를 수용하지 않는 엄격한 입력 경계다. KnownFields(true)와 두 번째 Decode의 EOF 검사가 이를 수행한다. 특정 과거 장애나 운영 규정의 근거는 이 코드에서 확인할 수 없다.

제안 사실 묶음: knowledge-manifest-parser. 원문: [internal/system/knowledgepack/lock.go:55–76](../../internal/system/knowledgepack/lock.go). span SHA256 8d2ded6423d7ea0908dc08422dcaf1cdae408b223e12adb4902eb01849f6ff8b.

## 독립성·오염 판정 경계

CAND01/02는 semantic-readonly-open 한 사실군, CAND03/04는 knowledge-manifest-parser 한 사실군이다. 한영 번역이나 WHY 재질문으로 별도 단위를 만들지 않는다. proposed 값만 기록했으며 승인 unit_id/cluster_id, gold/independence/contamination 판정·검토자·시각은 아직 없다.

읽기 전용 저장소 사실은 이전 입력 불변성 진단과, manifest parser 사실은 과거 pack 입력 검증과 근본 사실이 겹치는지 사람이 검토해야 한다. 현 코드에서 새로운 문장을 찾았다는 사실이나 ID가 다르다는 이유만으로 독립성을 보증하지 않는다. WHY 후보는 구현에서 읽을 수 있는 목적/동작 범위로 제한하며 조직 정책이나 특정 과거 장애의 원인으로 확대하지 않는다.

추가하려던 ‘테스트 성공과 사람 수용의 분리’는 기존 B0-WHY-03/TRACE 사실, ‘pack lock 변경의 dataset 신원 변경’은 B0-WHY-02 사실을 재사용하므로 독립 FINAL 후보에서 제외했다. 이 배제는 이미 관측한 결과를 다시 평가하거나 과거 질문 책을 수정한 것이 아니다.

## 확인 결과와 남은 입력

[입력 검사 결과](../../system/eval/b0-knowledge-system/refactoring-n11-final-candidate-preparation-2026-10-06/input-check-draft.json)는 --require-ready에서 exit2로 거부했다. source/gold 바이트 오류나 이전 승인 프로토콜 임계치 변경 오류는 없고, 미승인·미동결·확정 단위 없음·중요군 미충족이 남는다. 미확정 cluster 때문에 DEV_missing도 출력되며 DEV 원문7개가 실제 책에서 누락됐다는 뜻은 아니다. 승인된 CODE/WHY/POLICY/TRACE/ABS는 모두0, input_ready/execution_ready/product_release_approved=false, sampling_verdict=inconclusive다.

검색 실행·FINAL 예약0이다. 후보를 원문에서 작성하는 작업과 실제 모델 결과 관측을 구분한다. 충분한 CODE/WHY 독립 원문 사실과 실제 검토된 POLICY/TRACE 범위·ABS 답변 부재 대조를 더 준비해야 한다. N12의 메타모델20개/요구3개/기준4개를 독립 FINAL27단위로 세지 않는다. 현재 단계에서 전체 FINAL 승인이나 출시 결정을 요청하지 않는다.

요청15개 중5완료, 전체8/18 완료. 남은 전체 N08/09/10/11/12/14/15/16/17/18 10개. 다음은 기존 DEV/프로토콜·N12 사람 판정 반영, 이 후보의 오염/독립성 검토와 나머지 실제 gold 범위 준비 후 전체 입력 사전 동결이다.
