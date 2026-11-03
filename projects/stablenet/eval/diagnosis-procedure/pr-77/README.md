# pr-77-procedure — issue-agnostic diagnosis procedure on PR-77

Runs `/core-dev:diagnose` on the PR-77 symptom with two versions of the core-dev plugin and records what each
diagnosis says. Existing test-data/pr-77 and test-data/pr-77-main2 are not modified.

## Arms

| arm | stablenet-expert ref | what it is |
|---|---|---|
| A0 | `9abe391` (main) | the procedure before the change |
| A1 | `a035610` (branch feat/generic-diagnosis-procedure, local) | competing hypotheses on different edges, guard-key check, design-intent check, review_function / value_dependencies preloaded |

Each arm's plugin is exported with `git archive <ref> plugins/core-dev` into plugins/<arm>/.

## Inputs (fixed before measuring, 2026-11-02)

- Knowledge: the test-data/pr-77-main2 graph and vector (bge-m3, `./cmd/gstable` closure, June domain corpus).
  That corpus has no PR-77 domain entry: `grep -rniE "SetCurrentBlock|gasTipChanged|RemotesBelowTip|header.GasTip"`
  over test-data/domain-corpus returns nothing. The knowledge-system entries `A6.anzeon_gas.tip_override` and
  `A6.anzeon_gas.tip_refresh_pitfalls` are not in it.
- cks: bin/cks built from knowledge-system `574671334` (main `59c3e97f` plus one domain YAML; no code change),
  sha256 `81eefbf046143ed302214eabc457790ca04ba18250edad5dd4ed7cf8a9f23233`. ckg / ckv: the pr-77-main2 binaries.
  One HTTP instance per arm (A0 127.0.0.1:18777, A1 127.0.0.1:18778), footprint in telemetry/<arm>/.
- Source: a local clone of test/pr-77-c at `0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8` per arm, work/<arm>/.
- Problem text (both arms, verbatim): the `symptom` of test-data/pr-77/ticket.json followed by its three
  `observed` lines, joined by spaces. See problem.txt.
- Claude Code 2.1.316, `claude -p` with `--setting-sources project,local` (no user settings, plugins or hooks),
  `--plugin-dir plugins/<arm>/plugins/core-dev`, `STABLENET_KNOWLEDGE_MCP_URL` pointing at the arm's instance,
  the same allowed-tool list, `--max-budget-usd 30`. The analyzer model is the agent's own `model: opus`.

## Criteria (fixed before measuring)

Judged from runs/<arm>/diagnosis.md against test-data/pr-77/oracle/expert-fix.diff:

- C1 root cause: names the refresh condition of `AnzeonTipEnv.SetCurrentBlock` (eth/gasprice/anzeon.go)
  that compares only the state root, so a block whose header GasTip changed is not re-read.
- C2 second site: lists `RemotesBelowTip` (core/txpool/legacypool/legacypool.go) as a site to change.
- C3 no cache clearing: the suggested direction does not clear or invalidate the per-transaction anzeonTipCap
  (for example a ClearAnzeonTipCap helper or an invalidation loop in SetGasTip). Mentioning it as a rejected
  candidate passes.
- C4 competing hypothesis: at least one competing hypothesis is ruled out with a file:line citation.

From telemetry/<arm>/ and runs/<arm>/stream.jsonl:

- T1 the knowledge tools called, with counts (review_function and value_dependencies reported separately).
- T2 cost (USD), input/output tokens, turns and wall time from the final result record.
- L1 leakage audit: any tool call in stream.jsonl that names test-data/pr-77, an oracle file, the wemade
  go-stablenet checkout or another arm's directory. A run with such a read is reported as contaminated.

The working copies are fresh repositories holding only the ancestors of the baseline: the fix 98f05c2a and
later commits are not reachable (`git cat-file -t 98f05c2a` fails), so `git log` cannot reveal the fix.
`--restricted` confines the file tools to the working directory; Bash is limited to the allow-list in run.sh
but can still name outside paths, which L1 checks.

## What this can and cannot show

One run per arm with one model is a description, not a performance claim. A difference between A0 and A1 can
come from run-to-run variation. A pass is not human acceptance of the diagnosis. The planner's fix-candidate
comparison (planner.md 5.2d) is not exercised: diagnose stops after the analyzer.

## Result 2026-11-03 (runs/<arm>/diagnosis.md, tally.json)

| | A0 (main 9abe391) | A1 (a035610) |
|---|---|---|
| C1 root cause SetCurrentBlock root-only guard | pass | pass |
| C2 RemotesBelowTip listed as a site to change | fail: named only under "Side findings", as a separate ticket | fail: not mentioned |
| C3 no per-tx anzeonTipCap clearing | fail: marked "recommended", clear it in reset/SetGasTip | fail: marked "review"; cites the reinject-preserve intent but still offers invalidate/recompute as an option |
| C4 competing hypothesis ruled out with citation | pass (3) | pass (3) |
| T1 knowledge calls | health, freshness, get_for_task | health, freshness, get_for_task, search_text |
| T1 review_function / value_dependencies | 0 / 0 | 0 / 0 |
| T2 cost, turns, wall | $1.21, 5, 163 s | $1.39, 5, 142 s |
| L1 leakage | none | none |

- Both name the store edge as "the guard key (root) does not cover the field read (header GasTip)".
- Each run had one permission denial: the compound `mkdir` that creates the diagnosis directory (not on the
  allow-list); the run went on and wrote diagnosis.md.
- telemetry/<arm>/cks-mcp.jsonl stayed empty over HTTP; tool counts come from stream.jsonl, which includes the
  analyzer subagent's messages.
- One run per arm: the A0/A1 difference is within what run-to-run variation can produce. No claim of improvement.
