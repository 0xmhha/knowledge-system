# Retrieval pipeline, drawn the way "Attention Is All You Need" draws its model

The Transformer paper (Vaswani et al., 2017) shows its whole model in one picture, Figure 1: what enters each
block, what leaves it and which operation runs inside. This page draws the CKS retrieval pipeline the same way,
then compares the two systems: where the analogy holds, where it does not, and what is worth borrowing.

The stage order is the one in `internal/system/composer/composer.go`. Constants are the defaults on main after #135
(2026-11-04); stage names match the debug trace.

## Figure: question to evidence pack

```
 [Indexing, offline]
   source code  --ckg-->  code graph   (function and type nodes; calls / called_by / defines / imports edges)
   source code  --ckv-->  vector index (one chunk per symbol, embedded with bge-m3 or qwen3)
   source code  --ckv-->  invariant chunks (same index, chunk_kind=invariant: marker comments such as
                           // INVARIANT: and policy-worded panic / error sites)

 [Query, online]              input                         output
 +--------------------+
 | 0. Query rewrite    | question (Korean or English)  code_query (English, code vocabulary)
 |    (calling agent)  |                               the only step that "translates"
 +--------------------+
 | 1. Intent           | question                      bug_fix / concurrency / arch_explain / ... / unknown
 +--------------------+
 | 2. Stage 1 recall   | code_query (else question)    about 30 hits + keywords
 |                     |  ckv semantic search k=20 with BM25 rerank, up to 2 rounds
 |                     |  -> keyword candidates -> keyword cap 8 -> invariant pass
 +--------------------+
 | 3. Stage 2 rank     | hits + keywords               about 30 ranked seeds
 |                     |  BM25 and symbol lookup per keyword, then weighted RRF:
 |                     |  score = sum_i w_i / (60 + rank_i)
 +--------------------+
 | 4. Stage 3 expand   | seeds                         up to 50 graph neighbors
 |                     |  1-hop edges per seed; neighbor score =
 |                     |  seed score x relation weight / (1 + distance)
 +--------------------+
 | 5. Stage 4 allocate | seeds + neighbors             pack bodies: up to 12, plus up to 2 invariants
 |                     |  a seed that is also a neighbor (same function) adds the neighbor's score;
 |                     |  greedy by score: seed quota 10, neighbor reserve 2, knowledge reserve 2
 |                     |  (outside the cap), exact-symbol reserve 2; skip spans inside a selected
 |                     |  body; token budget 8000 x 0.9 = 7200, head snippet of 8 lines to fit
 +--------------------+
 | 6. Sanitize, pack   | selected bodies               evidence pack (bodies, citations, edges)
 +--------------------+
            |
            v
   diagnosing agent
```

Notes on the figure:

- The relation weight is 1.0 for every intent except `arch_explain` (`defines` 0.5, `imports` 0.7), so for a
  bug-fix question a 1-hop neighbor scores exactly half its seed: the halving comes from `1 + distance`.
- Hit-mined keywords are dropped before Stage 2 (Stage 1 `KeepHitKeywords` off). A Korean question has no
  identifiers of its own, so for such questions Stage 2 usually receives no keyword and its order equals the vector
  rank (measured on go-stablenet, 2026-11-04).
- Vector chunks start at the doc comment and graph spans at the `func` line, so one function has two spans. Stage 4
  treats them as one candidate since #135.

## Where the analogy holds

- **The problem frame.** Both turn text in one form into the matching content in another: a question in Korean or
  English into the code locations that own it. That is translation and retrieval at once.
- **Attention is soft retrieval.** A query is compared with keys and the values are weighted by the similarity.
  Stage 1 vector search does the same comparison and keeps the top k instead of weighting everything.
- **One picture with fixed inputs and outputs helps debugging.** The stage funnel (where the owning function is
  present at each stage) is how the 2026-11 pack defects were found: the knowledge reserve taking the neighbor
  slots (#134) and the two spans of one function (#135).

## Where it does not hold

- **The paper's gain did not come from listing stages.** It came from dropping recurrence and comparing every
  position with one function, softmax(QK^T / sqrt(d)) V, learned end to end against one objective and stacked in
  identical layers. CKS is the opposite: each stage has its own hand-set rule (seed quota, reserves, caps) over
  scores that are not on one scale. The 2026-11 defects were collisions between such rules.
- **The output differs.** Translation generates a new sentence; CKS selects among code that already exists. The
  closer literature is retrieve-then-rerank (dense retrieval, cross-encoder rerank) and natural-language-to-code
  search.
- **There is no training data to learn the weights.** The labelled sets are 11 go-stablenet PRs (AI-assigned
  labels) and 26 indexer-go PRs (human-confirmed). Learned weights would fit them at once. The usable lesson is to
  make the scores comparable, not to learn them.

## What is worth borrowing

- **One score space instead of slot rules.** Combine vector rank, BM25, symbol match and graph proximity into one
  score and take the top N. #135 is a first step: a seed that is also a graph neighbor of a stronger seed now adds
  that neighbor's score, which lifted the go-stablenet owning functions of pr-75 and pr-77 from seed ranks 16 and
  21 to 6. Risk: the reserves exist because single-score selection starved neighbors and invariants before; a
  unified score must keep them reachable.
- **Relations as heads.** Score `calls`, `called_by`, `defines` and other edges separately, like attention heads,
  and mix them by intent. Risk: too little data to set the mix.

## Known gaps (2026-11-04)

- The keyword channel is empty for Korean questions (see the note above). Taking the identifiers quoted in
  `code_query` as keywords was tried twice and lost PRs on go-stablenet both times: keyword seeds reorder the Stage 3
  neighbors. Not adopted.
- Questions whose owning code is an extension point (for example where a new hardfork registers its system contract
  upgrade) are reached neither by vector nor by keyword; domain knowledge is the likely path.
