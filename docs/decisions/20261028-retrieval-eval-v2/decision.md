# Retrieval evaluation v2: decisions (2026-10-28)

## Goal

Compare embedding models (qwen3-embedding:4b, bge-m3) on grpc-go and go-stablenet with the same CKS code and the
same questions, each PR indexed at its own pre-merge commit, and judge "found" by whether the pack holds the
function the PR had to change. (Not confirmed as one sentence by the user; the user asked to proceed quickly on the
recommendations.)

## Decisions

- D1. Model-dependent settings: run both arms. The equalized arm (both models chunk budget 6,144 bytes, intent
  threshold per model) is the comparison of record; the as-is arm (current code defaults) is the reference.
  Rejected: equalize only (cannot show what the bge-m3 settings cost), as-is only (unfair comparison).
- D2. grpc-go scope: pilot one PR end to end, then all 100 PRs at exact base. Rejected: 100 without a pilot
  (a setup mistake found after about 25 hours), a 40-PR subset (too small to separate the models).
- D3. Core function labels: AI labels from diff and PR text only, frozen before the run, reported as
  "AI-labelled, not human-confirmed"; human sample check later. Rejected: human check of all 100 before the run
  (blocks the run), a mechanical "largest change" rule (not the root-cause location).
- D4. go-stablenet: the same 12 PRs, all three vector arms rebuilt at each base, chained reindex.
  Rejected: keep the v1 qwen3 index and add bge-m3 (different code and chunking per cell), widen the sample now
  (new prompts and labels delay the run).
- D5. Success criteria and the threshold rule: delegated by the user to the recommendation (see the plan).

## Constraints and what is not done

- Measurement only: no search-logic change is judged on this run. Dev/test split by PR number parity.
- Prompts, truth, labels and the plan are frozen before the first eval run.
- No push, PR, merge or deployment without the user's explicit permission.

## Completion criteria

In the plan: knowledge-data/retrieval-eval-v2/TEST-REDESIGN-PLAN.md (pilot pass conditions, completion table).

## Assumptions (not user decisions)

- Equalized threshold = median best-anchor similarity per model over the 100 grpc-go prompts (0.426 / 0.545).
- Vector-only pack = first 12 hits, the same slot count as the CKS pack.
- go-stablenet pr72 is excluded: it changes no pre-existing declaration.
- v1 indexes are not reused: the v1 build binary reported vcs.modified=true.

## Deferred

- The decision interview stopped at the user's request ("우선은 빠르게 진행해서 테스트 결과를 보는것이 중요햐").
  Last ambiguity score 0.63 (threshold 0.2); goal and success criteria were not stated by the user in their
  own words.
- Human confirmation of the AI labels.
- Widening the go-stablenet sample (D4 option C) if 11 PRs cannot separate the models.

## Outcome (2026-10-29)

The full run finished: grpc-go 97 PRs scored (3 excluded because their v1 truth lines do not match the PR base),
go-stablenet 11. Results are AI-labelled, not human-confirmed. The run directory keeps the frozen plan, readouts,
inputs and scores (knowledge-data/retrieval-eval-v2, TEST-REDESIGN-PLAN.md).

- Equalized pair, grpc-go: core function in the 12-body pack, bge-m3 45/97 vs qwen3 36/97 (CKS); 41 vs 34
  (vector only). Equalizing the chunk budget changed nothing measurable (vector qwen3 34 vs qwen3-cb 34).
- Diagnosis on the dev split (even PRs): the graph stage almost never brought in a core function the seeds
  missed; graph candidates re-ranked by prompt similarity recovered 1 PR. The loss was the prompt vocabulary:
  rewriting the Korean prompt into code vocabulary lifted the core into vector top 12 for 16 -> 23 (bge-m3) and
  12 -> 24 (qwen3) of 35 PRs.
- Change made: get_for_task accepts an optional code_query used as the first semantic search (commit 4d0a6ad4).
  Judged on the test split (56 odd PRs, rewrites written from the prompts alone): core in pack 23 -> 36
  (bge-m3, 17 gained / 4 lost) and 18 -> 40 (qwen3, 22 gained / 0 lost).
- Open: human confirmation of the labels; a CKS-level go-stablenet check of code_query; the consumer contract in
  the agent repository must list code_query.

## Original record

## 결정 기록 원문: Retrieval evaluation v2: model x project at each PR base

### D1. Model-dependent settings (chunk budget, intent threshold): as-is, equalize, or both arms (decided)
- 질문 3: Model-dependent settings: A equalize then measure / B both arms (equalized main + as-is reference) / C as-is only
  - 답: 추천 B로해

### D2. grpc-go scope: all 100 PRs at exact base or a subset (decided)
- 질문 4: grpc-go scope: A all 100 / B 40 subset / C pilot 1 PR end-to-end then all 100
  - 답: . PR 1건으로 시범한 뒤 100건 전체 (추천) 로 진행하자

### D3. Core function labelling owner and human confirmation (decided)
- 질문 5: Core function labels: A AI labels + human confirms all 100 before run / B AI labels (diff+PR text only, no retrieval output) frozen before run, human sample check later, results marked AI-labelled / C mechanical rule (largest non-test change)
  - 답: 추천으로 진행

### D4. go-stablenet: same 12 PRs with a bge-m3 cell (decided)
- 질문 7: go-stablenet: A same 12 PRs, all three vector arms rebuilt at each base, chained reindex in commit order / B keep v1 qwen3, add bge-m3 only / C widen the PR sample
  - 답: 추천으로 진행. 우선 남은 질문들도 모두 추천으로 선택해서 진행하고, 너가 문서로 기록을 남겨놔줘. 우선은 빠르게 진행해서 테스트 결과를 보는것이 중요햐

### D5. Success criteria: pilot pass conditions and completion of the v2 measurement (decided)
- 질문 8: Remaining questions (success criteria, intent threshold rule for the equalized arm): user delegated to the assistant's recommendation
  - 답: 추천으로 진행. 우선 남은 질문들도 모두 추천으로 선택해서 진행하고, 너가 문서로 기록을 남겨놔줘. 우선은 빠르게 진행해서 테스트 결과를 보는것이 중요햐

