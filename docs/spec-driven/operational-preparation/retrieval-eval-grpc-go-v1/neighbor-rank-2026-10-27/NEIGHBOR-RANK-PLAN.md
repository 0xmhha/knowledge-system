# Neighbor ranking fixes on 67a2a6d5 (written before any fix run)

Question: when Stage 3 reaches the answer as a graph neighbor, why does Stage 4 not give it one of the
neighbor slots, and what ordering would.

Baseline: eval-final (committed 0c509daa, code unchanged in 67a2a6d5). Neighbor-only Stage 4 cuts (answer
reached by Stage 3, not by Stage 2): 4 (qwen3) / 6 (bge-m3). Answer neighbor rank in Stage 3 order:
qwen3 10, 21, 1, 5; bge-m3 6, 29, 37, 3, 27, 2. Neighbor slots used by those packs: 1-4 (one bge-m3 pack 8).

## Diagnosis

- D4. Stage 4 mergeCandidates re-sorts candidates by score, then file path, then start line. Stage 3
  already sorted neighbors by score, then target span (scoring.go documents why span replaced file order),
  and every neighbor of one seed and relation ties on score. Stage 4 throws that order away: for pr-8875
  (bge-m3) the answer is Stage 3 neighbor 2, but three shorter spans of the same file with lower line
  numbers took the four slots first; for pr-8894 (qwen3) the answer is Stage 3 neighbor 1 and lost to
  rpc_util.go by file name.
- D5. Stage 2 demotes test files x0.25 unless the intent is test_add or the prompt asks about tests.
  Stage 3 neighbors get no such demotion: for pr-8792 (qwen3) 19 of the 20 neighbors above the answer are
  test functions reached by called_by.
- D6. relationWeight demotes defines (struct to field, one-line spans) to 0.5 for ArchExplain only; for
  every other intent field neighbors tie with calls/called_by bodies (pr-9100: 5 defines above the answer).

Rejected before any run: ordering neighbors by vector similarity to the prompt. Offline probe
(relprobe.py: cosine from the index's own vectors, validated against ckv query scores on pr-9003 to four
digits) puts the answer neighbor at relevance rank 8-38 in 9 of the 10 cuts; one-line field chunks score
highest (pr-9100: 0.577 vs answer 0.522). Reordering by relevance gains 3 / 1 and loses 0 / 2 in the
offline simulation below.

## Offline simulation (slot count fixed per pack, seeds fixed; reproduces final found 51 / 58 exactly)

| order | qwen3 found | bge-m3 found |
|---|---|---|
| now (score, file, line) | 51 | 58 |
| d4 (Stage 3 order) | 53 (+2) | 57 (+1, -2) |
| d4 + d5 | 54 (+3) | 58 (+2, -2) |
| d4 + d5 + d6 | 55 (+4) | 58 (+2, -2) |

The two bge-m3 losses (pr-8979, pr-9070) were found by file-name order (pr-9070 through a one-line
neighbor inside the answer span). The simulation cannot show slot-count changes, so it is a prior, not
the result.

## Variants

- fix-d4: mergeCandidates breaks score ties by input order (Stage 2 seed order, then Stage 3 neighbor
  order); no file/line re-sort.
- fix-d45: fix-d4 plus Stage 3 applies the Stage 2 test-demotion decision to test-file neighbors (x0.25).
- fix-d456: fix-d45 plus defines weight 0.5 for every intent.

## Reading rules, fixed now (same as FOUND-FIX-PLAN.md)

1. Primary metric: answer in pack per model; also cks_any_commit MRR / hit@1; go-stablenet file r@5,
   r@10, MRR on both the PR-body arm and the Korean-question arm.
2. A variant is adopted when found rises on both grpc-go models, no grpc-go MRR falls by more than 0.01,
   and on both go-stablenet arms r@10 does not fall and MRR does not fall by more than 0.03.
3. Mean tokens are reported; a rise above 30% is flagged.
4. Every gained/lost scenario is listed with the trace mechanism.
5. Scenarios, answers and prompts do not change; the weights above do not change after the first run.

## Readout (2026-10-27, after the runs; nothing above was edited)

All 48 snapshot runs per variant exit 0. Every variant reproduces the offline simulation scenario for scenario.

| variant | qwen3 found / MRR / tokens | bge-m3 found / MRR / tokens | go-stablenet PR body r@10 / MRR | Korean r@10 / MRR |
|---|---|---|---|---|
| final | 51 / 0.280 / 1485 | 58 / 0.317 / 2866 | 0.71 / 0.73 | 0.44 / 0.60 |
| fix-d4 | 53 / 0.280 / 1829 (+23%) | 57 / 0.317 / 3471 (+21%) | 0.71 / 0.73 | 0.44 / 0.60 |
| fix-d45 | 54 / 0.281 / 1633 (+10%) | 58 / 0.317 / 3164 (+10%) | 0.71 / 0.73 | 0.44 / 0.60 |
| fix-d456 | 55 / 0.281 / 1800 (+21%) | 58 / 0.317 / 3221 (+12%) | 0.71 / 0.73 | 0.44 / 0.60 |

- Gained (fix-d456): qwen3 pr-8792 (test callers demoted), pr-8886, pr-8894 (Stage 3 order kept), pr-9008
  (field neighbors demoted); bge-m3 pr-8869 (test callers demoted), pr-8875 (Stage 3 order kept).
- Lost on bge-m3 in every variant: pr-8979 (answer grpclb_test.go:326-340 had led the file-name order; span
  order puts four longer neighbors first, one slot) and pr-9070 (found through config_selector.go:120, a
  one-line neighbor inside the answer span, first by file name).
- Tokens rise because span order prefers longer bodies on ties; D5 takes part of it back.
- Rule 2: found does not rise on bge-m3 (+2 -2) in fix-d45 / fix-d456 and falls in fix-d4. No variant is
  adopted by the rules. Whether to adopt fix-d456 as a defect fix is a user decision.
- Erratum to the pre-run text above (left as written): the relevance rank of the answer neighbor is 8-38 in
  8 of the 10 cuts, not 9; the other two are rank 1 (pr-8894) and 4 (pr-8886, bge-m3).

## User decision (2026-10-27)

Asked which variant to adopt as a defect-fix exception to rule 2. User's answer, verbatim:
"A. fix-d45 채택 (Recommended)". D4 and D5 are committed (91947285, 687a9662); D6 is not. final2, built from
committed 687a9662 in a clean worktree, gives packs identical to fix-d45 in all 200 scenario runs.
