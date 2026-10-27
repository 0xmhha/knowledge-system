# CKS 평가 판정 계약

v1/v2의 전체 원응답과 warmup/검색/warm/프로세스 시작 시간을 수집하는 `cks eval capture`, CKV 직접 후보 상한·독립 exact 오라클은 [B0 측정 도구](https://github.com/0xmhha/knowledge-system/blob/9a39718807ed25a0add30405f7cd1f1ebff4a629/docs/spec-driven/B0-MEASUREMENT-TOOLS.md)를 참고한다. 점수 계산과 정답·정책 사실의 검토는 별도다.

`cks eval`은 CKS MCP가 반환한 EvidencePack을 측정한다. 생성 모델의 최종 답변, 수용 기준의 실행 결과, 온톨로지 해석의 정확도를 판정하지 않는다. 시나리오 YAML은 기존 `version: 1`을 유지한다.

`--record-responses`를 명시하면 각 시나리오의 `raw_calls`에 도구명, 요청 인수, 호출 시간 `elapsed_ns`, SDK가 해석한 전체 MCP 결과를 기록한다. freshness 검사, 오류 도구 결과, 전송 실패도 남기며 성공 실행만 모으지 않는다. 응답은 점수 집계 전에 복사하고 요청에는 프롬프트와 선택적 intent만 전달한다. 정답·후보 답변은 전달하지 않는다. 원응답에는 인용 본문이 포함될 수 있으므로 평가 자료로 관리한다. 이 옵션은 기본 꺼짐이며 기존 보고서 형태를 유지한다. 기록·JSON 직렬화 시간은 도구 호출 지연에서 제외한다. 프로세스 시작과 모델 로딩이 포함된 cold 지연 또는 warmup 분리는 별도 측정 프로토콜에서 처리한다. 이 옵션은 v1 평가 경로만 기록하며 v2는 현재 capture/matrix 경로와 원자료 검증으로 수집한다. 직접 CKV exact/budget 측정은 별도 후보/오라클 경로다.

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

`cks eval --commit-repo <Git 저장소>`를 주면 커밋이 다른 인용 가운데 그 파일이 인용 커밋과 `expected_commit` 사이에서 바뀌지 않은 것(`git diff --quiet <인용 커밋> <expected_commit> -- <파일>`이 0으로 끝난 것)은 점수에 넣고 충돌로 세지 않는다. `ckv reindex`로 이어 만든 색인은 바뀌지 않은 파일에 처음 임베딩한 커밋을 남기기 때문이다. 받아들인 인용 수는 `unchanged_commit_citations`에 적는다. 파일이 바뀌었거나, 커밋이 비었거나, Git이 답하지 못하면 종전처럼 충돌이다. 옵션을 주지 않으면 종전의 엄격한 커밋 일치다.

개별 `expected_citations[].commit_hash`도 지정할 수 있다. 이 값이 있으면 해당 인용의 파일·줄과 커밋을 모두 맞춰야 정답으로 계산한다. 상위 `expected_commit`과 서로 다른 커밋을 선언한 시나리오는 입력 오류다. 개별 커밋만 지정한 경우에는 해당 인용의 점수만 커밋을 고려하고 인덱스 전체 스냅샷은 `unpinned`다.

각 시나리오 보고서의 판정은 서로 독립적이다.

| 필드 | 값 | 의미 |
|---|---|---|
| `retrieval_state` | `pass`, `miss`, `error`, `not_evaluated` | 선언한 정답 인용을 모든 실행에서 찾았는지. 도구 실행 오류는 정답 선언이 없어도 `error`다. |
| `evidence_state` | `pass`, `missing`, `error`, `not_evaluated` | 선언한 `expected_knowledge` 범위가 모든 실행의 팩에 실렸는지. |
| `abstention_state` | `pass`, `fail`, `error`, `not_applicable` | `expect_no_citations: true`일 때 인용을 0개 반환했는지. |
| `snapshot_state` | `current`, `conflict`, `unverified`, `unpinned` | 선택적 기대 커밋과 인덱스·인용 커밋의 관계. |
| `unchanged_commit_citations` | 0 이상 정수(0이면 생략) | `--commit-repo`로 커밋이 달라도 파일이 같아 점수에 넣은 인용 수(실행 중 최댓값). |

`file_recall` 등의 수치는 종전과 같이 실행별 중앙값이다. `retrieval_state=miss`는 한 번이라도 회수율이 1 미만이면 기록하므로 중앙값이 일시적 실패를 가리지 못한다. 기존 `citation_abstention_passed`는 호환을 위해 유지한다. 빈 `expected_citations`만으로는 보류 판정을 요청하지 않는다. 명시적인 `expect_no_citations: true`가 필요하다.

이 계약의 실제 CKS 스모크는 `scripts/wbs-smoke.sh`에 있다. 고정된 Git fixture, mock 임베더, 답을 찾는 시나리오, 인용을 삼가야 하는 시나리오 및 의도적인 커밋 충돌을 실행한다. 이는 상태 분류와 스냅샷 보호의 구조 검증이다. 검색 품질 게이트에는 실제 임베딩 모델, 고정 질문 세트, 지연·회수율 기준과 별도의 답변 검토가 필요하다.

### 의도 분류와 순위 지표

시나리오의 `intent`는 요청에 그대로 넘어가 서버의 의도 분류를 건너뛴다. 그래서 의도 파악을 재려면 `intent` 대신 `expected_intent`를 쓴다. `expected_intent`는 요청에 넣지 않고, 응답 묶음의 `intent`와 비교만 한다. 둘을 함께 쓰면 입력 오류다.

```yaml
version: 1
name: reconnect-stale-config
prompt: 연결이 끊겼다가 다시 붙을 때 예전 설정이 잠깐 쓰이는 문제를 고치고 싶어
expected_intent: bug_fix
expected_citations:
  - file: clientconn.go
    start_line: 120
    end_line: 180
```

|필드|위치|의미|
|---|---|---|
|`expected_intent`, `classified_intents`, `intent_match`|시나리오 결과|기대 의도, 실행마다 분류된 의도, 모든 실행이 기대와 같았는지. 오류 실행이 있으면 `intent_match`는 거짓이다|
|`metrics.first_hit_rank`|시나리오 결과|정답과 겹치는 첫 인용의 순위(중복 인용은 접은 뒤). 0은 못 찾음. 여러 실행이면 못 찾은 실행을 가장 뒤로 놓은 중앙값|
|`metrics.used_tokens`|시나리오 결과|응답 묶음이 쓴 토큰 수|
|`rank_summary`|보고서|정답 인용을 선언한 시나리오의 hit@1, hit@3, 첫 정답 순위의 역수 평균(MRR). 오류 시나리오도 분모에 남아 못 찾은 것으로 센다|
|`intent_accuracy`|보고서|`expected_intent`를 선언한 시나리오의 정확도와 혼동표(기대 → 분류). 분류값이 없으면 `(unknown)`, 오류면 `(error)`|

기존 `file_mrr`는 정답 인용마다의 순위를 평균한 값이라 그대로 둔다. "첫 정답을 몇 번째에서 찾았는가"는 `first_hit_rank`와 `rank_summary.mrr`로 본다.

## 현재 수집 경로와 범위

cks eval capture는 실제 v1/v2 원응답·warmup/retrieval/warm/cold 분모와 실패를 수집한다. cks eval matrix는 명시 입력·실행 환경·모델/소스/질문/설정을 결합한다. cks semantic compare-responses는 저장 팩을 검증하고 차이를 보고하며 품질·지연 합격을 판정하지 않는다. 승인 입력과 사전 protocol을 먼저 고정한다. 과거 FINAL/DEV를 새 독립 holdout으로 재사용하지 않고, 반복·언어 변형·동일 사실군을 독립 표본으로 부풀리지 않는다. 현재 관측과 미입증 범위는 [평가 상태](./EVALUATION-STATUS.md)를 따른다.
