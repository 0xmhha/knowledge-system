# diagnosis-generalization — the diagnosis procedure on go-stablenet bugs other than PR-77

PR-77 was used to find the procedure's gaps (test-data/pr-77-procedure), so it cannot show whether the rules
added afterwards generalize. This set runs `/core-dev:diagnose` on other go-stablenet bug fixes.

## Case selection (fixed 2026-11-03, before reading any fix diff)

From the 11 go-stablenet PRs of knowledge-data/retrieval-eval-v2 (go-stablenet-commits.json), excluding PR-77:
a PR whose title starts with `fix`, whose non-test changes touch at most 6 files and at most 60 changed lines
(`git diff --stat <merge>^1 <merge>`, files ending `_test.go` or under a `test/` directory excluded).
Result: PR-56 (2e046947, base 6e41d9663), PR-70 (c55bf2a8, base aa28927fb), PR-75 (0bf2f4d1, base e5a6e9e14).
Not selected: PR-55 (race), PR-63 (feature), PR-67 (fork move), PR-69 (refactor), PR-74 (limit constant) by
title; PR-58 and PR-73 by size.

## Arms

| arm | stablenet-expert ref |
|---|---|
| A0 | `9abe391` (main) |
| A2 | `11fb161` (feat/generic-diagnosis-procedure: a035610 plus observation-to-site mapping, consumer comparison, rejected directions) |

## Problem texts (fixed before measuring)

<case>.problem.txt, written after reading each fix diff, under one rule: state only what an operator observes
(behaviour, conditions, who is affected, log or RPC output); no file, function or variable name, no cause, no fix.
PR-77's ticket was written the same way. Because the author read the fix, a wording hint cannot be ruled out;
it is the same for both arms.

## Inputs

- Source: per case, a fresh repository holding only the ancestors of the base (fetched from test/go-stablenet),
  in <case>/src (indexed) and <case>/work/<arm> (where the agent runs).
- Graph: knowledge-data/retrieval-eval-v2/bin-main2/ckg, `--files-from-main ./cmd/gstable --no-cache`, no policy file.
- Vector: bin-main2/ckv, bge-m3, `--files-from` the same closure (<case>/scope.files-from.json), `--ckg`.
  No `--docs`: the domain corpus is left out so no post-fix knowledge reaches the index. (PR-77 runs included the
  June corpus; this is a difference between the sets, not between arms.)
- cks: test-data/pr-77-procedure/bin/cks (sha256 81eefbf0...). One HTTP instance per (case, arm).
- Runner: run.sh, the same flags as test-data/pr-77-procedure/run.sh, `--max-budget-usd 30`.

## Criteria (fixed before measuring)

Oracle sites, the non-test functions the fix changes (cosmetic changes excluded):
- PR-56: initializeCoinAdapter, initializeBase, initializeGovCouncil, initializeMasterMinter, initializeValidator (5).
- PR-70: Receipts.DeriveFields; its callers ReadReceipts, BlockChain.collectLogs, GenerateChain; and a way to tell an
  authorized sender without state (the fix: StateTransition.TransitionDb emits a log) (5).
- PR-75: Core.isSequenceTooFarAhead, Core.isRoundTooFarAhead (2).

Judged from <case>/runs/<arm>/diagnosis.md:
- C1 root cause names the primary defect: PR-56 comma-separated parameters split without trimming or dropping
  empty items in system contract initialization; PR-70 DeriveFields leaves EffectiveGasPrice unset when Anzeon is
  enabled; PR-75 the `>=` comparison against the threshold in the too-far-ahead check.
- C2 site coverage: oracle sites named in Root cause or Affected sites, as a count out of the total.
- C3 direction: the suggested direction would produce the fix's behaviour. Judged by the author of this file, not
  a human domain review.
- C4 a competing hypothesis is ruled out with a file:line citation.
- T1/T2/L1 as in test-data/pr-77-procedure (tool calls, cost, leakage; leakage pattern also covers other cases).

One run per (case, arm). Three cases and one run each describe; they do not establish an effect.

## Result 2026-11-03 (<case>/runs/<arm>/diagnosis.md, <case>/tally.json)

| case | arm | C1 | C2 sites | C3 direction | C4 | Rejected directions | cost | knowledge calls |
|---|---|---|---|---|---|---|---|---|
| PR-75 | A0 | pass | 2/2 | pass (`>` in both helpers) | pass | - | $1.09 | health, find_callers |
| PR-75 | A2 | pass | 2/2 | pass | pass | 2 (remove the gate; raise constants) | $1.01 | health, find_callers |
| PR-56 | A0 | pass | 5/5 | pass (shared trim helper; "drop or reject" empty items) | pass | - | $1.32 | health, get_for_task, find_callers |
| PR-56 | A2 | pass | 5/5 | pass (shared trim-and-skip-empty helper) | pass | 2 (trim in HexToAddress; reject all whitespace) | $1.22 | health, get_for_task |
| PR-70 | A0 | pass | 4/5 (no way to tell an authorized sender without state) | partial: normal senders via header GasTip in DeriveFields; authorized case left open | pass | - | $1.17 | health, freshness, get_for_task |
| PR-70 | A2 | pass | 4/5 (same) | partial: same, two options for the authorized case | pass | 2 (drop the Anzeon guard; add the field to consensus receiptRLP) | $1.47 | health, freshness, get_for_task, change_history |

- No leakage in any run (tally.py L1). Each run had one permission denial (a probe or a compound command).
- review_function and value_dependencies: 0 calls in all six runs.
- C2 and C1 do not separate A0 and A2 on any case. The only consistent difference is the Rejected directions
  section, which A2 writes in all three; its entries cite code or stated intent. One run per (case, arm).
