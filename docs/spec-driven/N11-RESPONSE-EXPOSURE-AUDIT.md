# N11 과거 응답 본문의 후보 근거 노출 대조

2026-10-08 · **추가 출처 대조 완료 / 독립성·정답 판정과 N11 전체는 미완료**.

[대조 요약](../../system/eval/b0-knowledge-system/refactoring-n11-response-exposure-2026-10-08/response-exposure-summary.json) · [기존 제외 문항의 본문 증명](../../system/eval/b0-knowledge-system/refactoring-n11-response-exposure-2026-10-08/known-exclusion-body-proof.json) · [재현 소스](../../system/eval/b0-knowledge-system/refactoring-n11-response-exposure-2026-10-08/response-exposure-audit.py.txt).

## 목적과 방법

새 독립 FINAL은 이전에 관측한 사실을 질문 이름·번역·세부 질문으로 바꿔 재사용해서는 안 된다. 기존 후보 대조는 명시적 citations와 선택된 함수명 문자열을 검사했다. 이번에는 과거 응답의 실제 bodies·graph_neighbors 및 다른 structuredContent 문자열도 검사하여 출처 대조의 누락 범위를 줄인다. 모델 실행과 질문/gold 조정 없이 보관 응답만 읽는다.

기존 pool의 DEV7/검토 FINAL13/제외6, 총26개 source refs를 책의 원문 commit `479491baa50f818a5f7bb7651f35e9c7192ad906`에서 검증했다. 파일/양끝 포함 span SHA와 현재 원문도 모두 같다. 양성 근거24문항을 검사하며 ABS2개의 basis는 양성 답변 증거로 사용하지 않고 본문 부재로 기권의 타당성을 자동 판정하지 않는다.

27gzip 원자료의 압축 바이트를 현행 Git commit과 대조하고, 압축/해제 SHA·행 수를 이전 대조 원장과 확인했다. 총 **11,632행 중 structuredContent object 11,631행**, body 항목 **71,542개**, graph_neighbor 항목 **0개**다. 다른1행의 비객체 출력은 별도 분모로 남긴다. 중첩 object/list의 문자열 **1,778,430필드/761고유값**을 순회했다. response.content의 문자열 출력은 두 번째 표면으로 다시 검사하지 않았다.

CRLF를 LF로 맞추고 각 행의 앞뒤 공백·빈 행만 정규화한다. 소스 문자열의 literal `\\n`을 새 줄로 다시 해석하지 않는다. 다음 일치를 별도로 기록한다.

1. 원문 span의 전체 nonempty-line 문자열.
2. 연속 nonempty 8행 이상·UTF-8 128bytes 이상·24word tokens 이상의 조각.
3. 완전한 단일 행 중 UTF-8 64bytes 이상·10word tokens 이상의 행.

첫 전체 span/8행 대조는 모든 후보의 일치0이었다. 이미 알려진 CAND05 노출이 한 줄짜리 본문이어서 이 방식만으로는 잡히지 않았다. 초기 결과·방법 소스를 보존하고 단일 정보 행 검사를 추가했다. 이는 검사 범위의 보완이며 제품 RED나 새로운 모델 관측이 아니다. 전체/조각/단일 행의 일치를 한 지표로 합쳐 완전한 오염 검사를 주장하지 않는다.

## 실제 관측과 이중 확인

| 후보 범위 | 전체 span/8행 조각 | 긴 단일 행 | 처리 |
|---|---:|---:|---|
| 이미 제외한 CAND05 | 0 | 480응답 | 기존 제외 유지 |
| 나머지 양성23문항 | 0 | 0 | 독립성 승인 없이 검토 대기 |
| ABS2문항 | 검사하지 않음 | 검사하지 않음 | bound corpus·권위·시점의 기권 판정 대기 |

CAND05는 `internal/system/knowledgepack/review.go:104`의 inline review/quorum 오류 문장이다. B0-POLICY-02/B0-WHY-01의480응답 각각에 대해 실제 body의 file path/fileSHA/start=end=104와 그 행의 원문 바이트 SHA가 같음을 독립 재확인했다. source refs74–106 안에 있는104행이다. **480반복 응답을480독립 표본으로 세지 않는다.** 공유 사실군의 CAND06도 이전 제외를 유지한다.

나머지의 일치0은 이 문자열 방법의 결과다. 짧은 행, 정제/재서술된 본문, 같은 의미의 다른 코드·문서, 이전 개발 튜닝이나 사람의 노출을 완전히 검사하지 않는다. 긴 공통 코드 행은 서로 다른 함수에도 있을 수 있어 일치 자체도 관계 의미·독립성의 자동 판정이 아니다. graph_neighbor0은 실제 보관 응답에 항목이 없다는 뜻이며 populated graph traversal를 시험했다는 뜻이 아니다.

재현 소스 안의10개 합성 제어는 full body, CRLF/indent, 부분8행, 함수명만 있는 입력·짧은 boilerplate 배제, literal escape 보존, nested graph body 순회, 비문자 값 배제, 긴 단일 행 검사를 확인한다. 실제 graph neighbor가 비어 있는 상태와 합성 순회 제어를 구분한다.

재현 소스는 `--out NEW_OUTPUT_DIRECTORY --reference-revision RECORDED_COMMIT`을 받으며 기존 출력 디렉터리를 거부한다. 새 디렉터리에서4출력의 바이트/SHA가 동일함을 확인하고, 기존 출력 재시도의 거부와 바이트 보존을 확인했다. [재생 결과](../../system/eval/b0-knowledge-system/refactoring-n11-response-exposure-2026-10-08/replay-check.json).

## 체크와 다음 실제 판정

- [x] 26원문 refs·현재 바이트,27gzip/11,632행의 기존 원자료 SHA/행 수 재확인.
- [x] 전체 structuredContent 문자열·body/graph 분모와 전체/부분/긴 단일 행 검사·10방법 제어.
- [x] CAND05의480본문을 실제 동일fileSHA/104행 원문 바이트로 이중 확인; 기존 제외와 질문/protocol/review 불변 확인.
- [ ] POLICY/TRACE의 평가 범위 선택과 WHY 추론 분류, 질문별 gold/군/독립성/오염·ABS 판정.
- [ ] 중요군별10승인 독립 FINAL까지 부족 범위 보강, source/runtime/입력 동결 후 평가.

책은 여전히 **DEV7·검토 FINAL13/제안군8·제외6**, 중요군별 승인0이다. 모델 HTTP/새 FINAL dispatch/코퍼스·색인 빌드/사람 decision0, 사람 활동 시간null, sampling=inconclusive를 유지한다. 생산 코드·gold·질문 책·protocol·검토 기록은 수정하지 않았다. 이 작업은 read-only 감사이며 새 Go 회귀를 실행했다고 기록하지 않는다. 이전 시험은 해당 커밋의 증거다.

**전체8/18·요청5/15·남은10개 유지, N11-A–D 미체크.** 다음 선택은 [N11/N12 범위 판정 안내](./N11-N12-SCOPE-DECISIONS.md)에 정리했다. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
