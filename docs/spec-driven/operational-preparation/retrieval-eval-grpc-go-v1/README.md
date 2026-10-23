# grpc-go 검색 평가 입력

[검색 평가 명세](../../RETRIEVAL-EVAL-SPEC.md)에 따라 만든 입력이다. 결과를 보기 전에 고정했다. 결과를 본 뒤 표본, 정답, 질문을 바꾸지 않는다.

- `inputs/sample.json`: 받은 PR, 제외한 PR과 이유, 표본 100건, 각 PR의 기준 커밋·스냅샷·정답 범위
- `inputs/prompts.json`: PR 번호별 한국어 질문. AI가 PR 제목과 설명만 보고 썼다. 사람의 표본 확인은 아직이다
- `inputs/scenarios/<스냅샷>/pr-<번호>.yaml`: `cks eval` 시나리오. `expected_intent`만 쓰고 `intent`는 쓰지 않는다
- `manifest.json`: 위 파일과 생성 도구의 해시

## 다시 만드는 법

```sh
go build -o /tmp/declspans scripts/retrieval_declspans.go
python3 scripts/retrieval_pr_scenarios.py collect --repo <grpc-go 전체 이력 clone> --repo-slug grpc/grpc-go \
  --since 2026-01-01 --until 2026-10-21 --declspans /tmp/declspans --out sample.json
python3 scripts/retrieval_pr_scenarios.py assemble --sample sample.json --prompts prompts.json --out scenarios
```

정답 범위는 Go 표준 파서로 기준 커밋의 원본에서 정한다. 평가 대상인 CKG 그래프를 쓰지 않는다. PR이 공유 스냅샷에 붙으려면 정답 선언의 바이트가 스냅샷과 기준 커밋에서 같아야 하고, 아니면 기준 커밋으로 따로 색인한다.

## 색인과 실행

`scripts/retrieval_snapshot_chain.py`가 스냅샷마다 그래프를 한 번 만들고, 모델마다 가장 이른 스냅샷의 벡터를 전체로 만든 뒤 다음 스냅샷부터는 앞 색인을 복사해 `ckv reindex`로 바뀐 파일만 다시 임베딩한다. 복사한 색인은 `reindex` 전에 이번 스냅샷의 그래프를 가리키게 바꾼다. `90f571db`에서 전체 색인과 비교하면 code 청크는 같고 패키지 convention 요약 청크 12개만 다르다([명세](../../RETRIEVAL-EVAL-SPEC.md) 3장). 모델은 `qwen3-embedding:4b`(2560차원)가 본 평가, `bge-m3`가 기준선이다.

비교 대상은 `scripts/retrieval_baselines.py`가 같은 질문과 색인으로 돌린다. 벡터만, 벡터와 BM25 재정렬, 그래프 낱말 검색, `git grep`이다.
