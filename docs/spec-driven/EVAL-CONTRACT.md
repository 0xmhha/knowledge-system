# CKS 평가 판정 계약

`cks eval`은 CKS MCP가 반환한 EvidencePack을 측정한다. 생성 모델의 최종 답변, 수용 기준의 실행 결과, 온톨로지 해석의 정확도를 판정하지 않는다. 시나리오 YAML은 기존 `version: 1`을 유지한다.

```yaml
version: 1
name: find-handler
prompt: Where is the handler?
expected_commit: 0123456789abcdef0123456789abcdef01234567
expected_citations:
  - file: handler.go
    start_line: 10
    end_line: 20
expected_knowledge: [policy]
runs: 1
```

`expected_commit`은 선택 사항이며 전체 40자리 소문자 Git SHA만 받는다. 지정하면 매 실행 전에 `cks.ops.freshness`의 `indexed_head`를 확인하고, 반환된 각 인용의 `commit_hash`를 검사한다. 다른 커밋 또는 빈 커밋의 인용은 원시 `citation_count`에는 남지만 파일 회수율·정밀도·MRR의 정답 후보에서는 제외한다. 인덱스 커밋이 다르거나 인용 커밋이 다르면 `snapshot_state=conflict`, 인덱스 커밋을 확인할 수 없으면 `unverified`다. 두 상태에서는 JSON 보고서를 먼저 기록한 뒤 CLI가 실패한다. 커밋을 지정하지 않은 과거 시나리오는 `unpinned`로 표시하며 버전 정합성을 주장하지 않는다.

개별 `expected_citations[].commit_hash`도 지정할 수 있다. 이 값이 있으면 해당 인용의 파일·줄과 커밋을 모두 맞춰야 정답으로 계산한다. 상위 `expected_commit`과 서로 다른 커밋을 선언한 시나리오는 입력 오류다. 개별 커밋만 지정한 경우에는 해당 인용의 점수만 커밋을 고려하고 인덱스 전체 스냅샷은 `unpinned`다.

각 시나리오 보고서의 판정은 서로 독립적이다.

| 필드 | 값 | 의미 |
|---|---|---|
| `retrieval_state` | `pass`, `miss`, `error`, `not_evaluated` | 선언한 정답 인용을 모든 실행에서 찾았는지. 도구 실행 오류는 정답 선언이 없어도 `error`다. |
| `evidence_state` | `pass`, `missing`, `error`, `not_evaluated` | 선언한 `expected_knowledge` 범위가 모든 실행의 팩에 실렸는지. |
| `abstention_state` | `pass`, `fail`, `error`, `not_applicable` | `expect_no_citations: true`일 때 인용을 0개 반환했는지. |
| `snapshot_state` | `current`, `conflict`, `unverified`, `unpinned` | 선택적 기대 커밋과 인덱스·인용 커밋의 관계. |

`file_recall` 등의 수치는 종전과 같이 실행별 중앙값이다. `retrieval_state=miss`는 한 번이라도 회수율이 1 미만이면 기록하므로 중앙값이 일시적 실패를 가리지 못한다. 기존 `citation_abstention_passed`는 호환을 위해 유지한다. 빈 `expected_citations`만으로는 보류 판정을 요청하지 않는다. 명시적인 `expect_no_citations: true`가 필요하다.

이 계약의 실제 CKS 스모크는 `scripts/wbs-smoke.sh`에 있다. 고정된 Git fixture, mock 임베더, 답을 찾는 시나리오, 인용을 삼가야 하는 시나리오 및 의도적인 커밋 충돌을 실행한다. 이는 상태 분류와 스냅샷 보호의 구조 검증이다. 검색 품질 게이트에는 실제 임베딩 모델, 고정 질문 세트, 지연·회수율 기준과 별도의 답변 검토가 필요하다.
