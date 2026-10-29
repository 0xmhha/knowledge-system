# Answer-in-pack fixes on 9e4a0f0b (written before any fix run)

Question: does the Stage 4 pack hold code that leads to the fix (an answer span), and when it does not, why.

Diagnosis (found-head9e4a/, scripts/retrieval_found_diagnosis.py, deep pure-vector probe K=1000):

| | qwen3 | bge-m3 |
|---|---|---|
| answer in pack | 39 | 46 |
| cut at Stage 4 | 24 | 24 |
| cut at Stage 2 cap | 2 | 3 |
| never in any candidate list | 35 | 27 |
| ... answer chunk not indexed | 0 | 0 |
| ... answer beyond pure-vector rank 20 | 30 | 22 |

Two defects explain the Stage 4 cuts:

- D1. Test demotion (score x0.25) runs whenever intent != test_add. Intent is Unknown for most qwen3
  prompts, and the classifier's nearest intent is test_add for only 5 (qwen3) / 3 (bge-m3) of the 22
  test_add prompts. Answers that pure vector ranked 1-5 fall to Stage 2 rank 8-26 and are cut:
  11 (qwen3) / 9 (bge-m3) of the Stage 4 cuts are tests-only answers under a non-test intent.
- D2. Stage 4 holds slots for neighbors (4), knowledge (2) and exact symbols (2) even when no such
  candidate exists. All 48 Stage 4 cuts happened in packs of fewer than 12 bodies; only 6 / 4 of 100
  packs reach 12; 68 / 59 packs select no neighbor.

Fixes (one variant each, then both):

- fix-d1: skip test demotion when the prompt itself asks about tests (lexical cue, model independent):
  시험, 테스트, test(s), flaky, e2e, fuzz, 퍼저, benchmark, 벤치마크. Cue coverage measured on inputs
  before the run: grpc-go test_add 21/22, other intents 2/78; go-stablenet 4/12 prompts.
- fix-d2: a reserve only holds a slot while an eligible candidate is still ahead in the list:
  effective neighbor reserve = min(NeighborReserve, neighbors selected + neighbors ahead); knowledge and
  exact-symbol holdbacks are capped by the eligible candidates ahead. MaxCitations stays 12.
- fix-d12: both.

Data: grpc-go 100 scenarios x {qwen3-embedding:4b, bge-m3} on the existing indexes; go-stablenet 12 PRs
(qwen3). Baseline: eval-head9e4a (9e4a0f0b), go-stablenet requery/head9e4a.

Reading rules, fixed now:

1. Primary metric: answer in pack (found at Stage 4, any-commit overlap) per model; also pack MRR
   (cks_any_commit) and hit@1. go-stablenet: file r@5, r@10, MRR.
2. A fix is adopted when found-in-pack rises on both grpc-go models, grpc-go MRR does not fall by more
   than 0.01 on either model, and go-stablenet r@10 does not fall and MRR does not fall by more than 0.03.
3. Mean returned tokens are reported; a rise above 30% is flagged as a cost trade, not a ranking gain.
4. Every better/worse scenario is listed with the trace mechanism (Stage 2 rank, Stage 4 decision);
   a metric move without a matching mechanism is reported as unexplained.
5. Scenarios, answers and prompts do not change. The cue word list does not change after the first run.
6. The 62 never-retrieved scenarios are not targeted by these fixes; they are debugged next.

## Addendum (2026-10-27, written while fix-d1/d2/d12 were running, before any of their results)

Debugging the never-retrieved scenarios (scratch graph probe over Stage 2 top-10 seeds, 2 hops, code edges)
found the answer one graph hop from a seed in 7 (qwen3) / 5 (bge-m3) of them, most often as a caller of
the seed. Stage 3 never returned it because:

- D3. For Unknown and QAReview, Stage 3 passes nil relations, and ckgclient reads nil as a forward-only
  walk (calls, invokes, implements, imports, references, extends, defines). A seed's callers never come
  back, although the code comment calls this the broad fan-out.

Fix variant:

- fix-d3: for nil relations, Stage 3 also runs a called_by pass (two ckg calls per seed instead of one).
- fix-d123: fix-d12 plus fix-d3.

Same data, baseline and reading rules as above. A neighbor only reaches the pack through the neighbor
reserve, which D2 changes, so fix-d3 alone is expected to move little; fix-d123 is the combination read.
Also noted, not fixed here: forward `defines` from a struct seed returns its one-line fields, which fill
the neighbor list with spans that carry no behavior.

## Readout of fix-d1 / fix-d2 / fix-d12 and one more variant (2026-10-27, before any fix-d3 result)

- fix-d2 meets every rule: found +2 (qwen3) / +4 (bge-m3), no scenario lost, MRR +0.001 / +0.001,
  go-stablenet unchanged, tokens +14% / +15%. Adopted.
- fix-d1 raises found +9 / +7 with no scenario lost and MRR +0.046 / +0.043, but go-stablenet MRR falls
  0.79 -> 0.73, past the 0.03 limit. The whole fall is pr63 (rank 1 -> 4): its PR body has a "## Test Plan"
  template section, so the cue fired on boilerplate, not on what was asked. By rule 2 fix-d1 is not
  adopted; by rule 5 the cue list is not edited. Adopting it anyway is left to the user.
- fix-d12: found +11 / +13, go-stablenet as fix-d1, tokens +32% / +40% (flagged by rule 3).

Added variant: fix-d23 = fix-d2 plus fix-d3 (the adoptable combination if fix-d1 stays out).

## Readout of fix-d3 / fix-d23 / fix-d123 and D3b (2026-10-27, before any D3b result)

- fix-d3: found unchanged on both models, go-stablenet unchanged, tokens +1-2%.
- fix-d23: found +3 / +4, go-stablenet unchanged (fix-d2 plus pr-8977 on qwen3).
- fix-d123: found +12 / +13, go-stablenet as fix-d1.

Why D3 moved nothing: the called_by pass returned the seed itself. For pr-9100 (qwen3) the called_by
neighbors of seed xds_resolver.go:575-633 were xds_resolver.go:580-633 twice, the seed's own node.

- D3b. ckgclient keeps the graph's orientation on a callers walk (the caller is Source, the seed is
  Target; its tests read n.Source), while Stage 3 and everything after it read Edge.Target as the neighbor.
  So no caller has ever reached the pack, for any intent that walks called_by (bug_fix, refactor,
  test_add, concurrency_safety, security), not only the new Unknown pass.
- Fix: Stage 3 orients each returned edge from the seed (the endpoint overlapping the seed citation becomes
  Source), the shape ScoredNeighbor.Edge documents.

Variants: fix-d3b = fix-d3 plus D3b; fix-d23b = fix-d2 plus fix-d3 plus D3b. Same rules.

## User decision on fix-d1 (2026-10-27)

Asked whether to adopt fix-d1 although go-stablenet MRR fell past the rule-2 limit. User's answer, verbatim:
"A. 예외로 채택 (Recommended)". fix-d1 is adopted as an exception to rule 2; the recorded reason is that the
whole go-stablenet fall is pr63, whose PR-body prompt carries a "## Test Plan" template section. The cue
list stays as first run (rule 5). The final build is D1 + the D2/D3/D3b variants that pass the rules.

## User decision on snapshot-commit scoring (2026-10-27)

Facts shown: 661 (qwen3) / 651 (bge-m3) pack citations carry an older commit than the scenario snapshot; for
0 of them did the file change between that commit and the snapshot. User's answer, verbatim:
"B. cks eval 판정 고치기 (Recommended)". cks eval will accept a citation whose file is unchanged between
the citation's commit and the expected commit; the evaluation spec is updated with it.

## User decision on the go-stablenet prompt set (2026-10-27)

User's answer, verbatim: "A. 한국어 질문 새로 만들기 (Recommended)". The 12 go-stablenet PRs get new Korean
developer-style questions without file, function or identifier names, leak-checked and frozen before any
run; the PR-body arm stays as recorded. The questions are AI-written and not human-reviewed (DEV).

## Readout of fix-d3b / fix-d23b and user decision (2026-10-27)

- fix-d3b: found unchanged on both models (none lost), MRR +0.002 / +0.001, go-stablenet r@10 0.65 -> 0.71,
  tokens +8% / +5%. Mechanism: one-hop caller answers moved from never retrieved to Stage 3 neighbor rank
  33-44 (pr-9100, pr-9008 on qwen3; pr-8939, pr-9069 on bge-m3), below the four neighbor slots, where
  one-line struct-field `defines` neighbors crowd the list.
- fix-d23b: found +3 / +3 (fix-d2 had +2 / +4), tokens +39% / +23%.
- D3/D3b do not meet rule 2 (found does not rise). Asked whether to adopt them as a defect fix. User's
  answer, verbatim: "A. 채택 (Recommended)". Final build: D1 + D2 + D3 + D3b, measured from the committed tree.
- Next bottleneck: neighbor ranking (four slots; the right caller sits at rank 33-44).
