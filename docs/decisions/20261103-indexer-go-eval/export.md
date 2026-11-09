## 결정 기록 원문: Related-project retrieval evaluation (indexer-go replaces grpc-go)

### D1. Independent evaluation project replacing grpc-go (decided)
- 질문 1: Which project replaces grpc-go as the independent evaluation project: indexer-go only (recommended) or indexer-go plus grpc-go
  - 답: 추천으로 진행하자

### D2. indexer-go PR selection rule (decided)
- 질문 3: indexer-go PR selection: mechanical rule only / mechanical rule plus symptom filter (recommended) / lift the 1-5 function limit
  - 답: 추천으로 진행
- 질문 13: Symptom filter criterion: keep a PR only if the wrong behaviour shows at an interface a caller or user sees (API response, stored data, error / crash / hang, a race on a production path) and expected vs actual can be written; drop performance-only, refactoring, test-only and documentation-only changes (recommended) / also include performance regressions and test-only races
  - 답: 추천 1 진행

### D3. Problem text rule (symptom only, source, writer) (decided)
- 질문 4: Problem text rule: separate agent rewrites the commit body into a Korean symptom-only text (user-visible names kept, internal identifiers removed), mechanical identifier check, human approval (recommended) / user writes / commit subject only
  - 답: 추천으로 진행해

### D4. Owning-function labels: who assigns and who confirms (decided)
- 질문 7: Labels: AI marks the function whose change fixes the symptom (others as related) and the user confirms each PR, unconfirmed PRs reported separately (recommended) / mechanical rule only
  - 답: 추천으로 진행

### D5. Adoption rule on indexer-go (splits, gain/loss thresholds) (decided)
- 질문 8: Adoption rule: no split; go-stablenet 11 for development, indexer-go whole set judged once per change with the #130 rule (both arms gain more than lose, net >= 2 over both), human-confirmed PRs only, number of looks recorded (recommended) / odd-even dev-test split
  - 답: 추천 1번으로 짐행

### D6. Same-name loss check for the same-name body limit (79c938d1) (decided)
- 질문 9: Rule for changes that target a failure observed in the development set (general, written down per change before measuring): effect reported on go-stablenet; no harm on indexer-go (both arms lose no more than gain); risk subset = PRs whose labelled function name has >=3 declarations (indexer-go and go-stablenet), 0 lost in both arms (recommended) / apply the D5 rule as is
  - 답: 추천 1번으로 진행

### D7. Purpose: what this evaluation decides (decided)
- 질문 11: Purpose: retrieval level only - confirm that a retrieval change raises how often the owning function of a go-stablenet symptom question enters the pack, and that the gain is not fitted to seen cases, using the unseen related project indexer-go; diagnosis quality is a separate evaluation (recommended) / include diagnosis level
  - 답: 추천 1로 진행
- 질문 14: Goal sentence confirmation: 'Check whether a retrieval change raises how often the owning function of a go-stablenet symptom question enters the pack, and confirm on the unseen related project indexer-go that the gain is not fitted to seen cases; diagnosis quality is not part of this evaluation.' Minimum confirmed PR count and a go-stablenet pass criterion are deferred.
  - 답: 승인

### D8. What carries over from retrieval-eval-v2 unchanged (decided)
- 질문 12: Carry over unchanged from retrieval-eval-v2: arms cks-bge-m3-eq and cks-qwen3-eq, metric tools/funnel_c.py (owning function in final pack bodies), tools/run_exp.py with per-PR base-commit indexes, code_query written by a separate agent from the problem text only and frozen before measuring (recommended) / bge-m3 arm only
  - 답: 추천 1 진행

