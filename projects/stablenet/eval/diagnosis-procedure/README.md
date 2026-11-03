# diagnosis-procedure — does the core-dev diagnosis find the owning code on go-stablenet bugs

Measures `/core-dev:diagnose` (stablenet-expert) on go-stablenet bug fixes, with this knowledge system as its
retrieval. The question is the product goal applied end to end: from a symptom-only report, does the agent name
the root cause and every site the real fix changed, and does it avoid a fix that contradicts the code's intent.

The records sit here and not in stablenet-expert on purpose: the agent plugin carries only issue-agnostic rules,
and case-specific results belong to the evaluation of the knowledge system.

## Retention

Kept only while the diagnosis procedure and its retrieval are being developed and researched. When performance
is established, remove this directory (and its catalog rows), recording the removal in
docs/DOCUMENTATION-CLEANUP.json as the documentation policy requires. What counts as "established" is the
owner's decision and has not been set.

## Contents

| directory | what | arms |
|---|---|---|
| pr-77/ | PR-77 (AnzeonTipEnv refresh), the case the procedure changes were derived from | A0 main `9abe391`, A1 `a035610` |
| generalization/ | PR-56, PR-70, PR-75, selected by a fixed rule before reading their fixes | A0 main `9abe391`, A2 `11fb161` |

Each directory's README holds its inputs, the criteria fixed before measuring and the result. `runs/<arm>/`
holds the agent's `diagnosis.md` and wall time; `tally.json` holds tool counts, cost and the leakage audit.
Both arm commits were later squashed into stablenet-expert main `6387a20` (#60).

Copied from auto-coding/test-data/pr-77-procedure and auto-coding/test-data/diagnosis-generalization on
2026-11-03. Not copied, and kept only there: the raw `stream.jsonl` transcripts, cks logs and configs, the
built graph and vector indexes, the cks binary and the working copies. The one byte change is the root path in
generalization/run.sh and setup.sh, replaced by `${AUTO_CODING}`; the run copies had sha256
`3de30e986369feff4e3d8bd4418465d2e606ff04becedd2a1c5cab407589f1a8` (run.sh) and
`a076f694612546d8c93eabeb6f0b96d8170fadc898705de073397a2ec8cb30ba` (setup.sh).

## Result summary

- C1 root cause and C4 ruled-out competing hypothesis: pass in all 8 runs.
- C2 sites: PR-77 misses `RemotesBelowTip` in both arms; PR-75 2/2, PR-56 5/5, PR-70 4/5 in both arms.
- C3 direction: PR-77 both arms still offer clearing the per-transaction tip cache; PR-70 partial in both.
- The changed procedure differs only in writing a "Rejected directions" section (all three generalization cases).
- `review_function` and `value_dependencies` were called 0 times in every run.

## Limits

One run per case and arm: a description, not a performance claim. The problem texts were written by someone who
had read the fix. C1-C4 were judged by the author, not by a domain reviewer, and are not human acceptance.
