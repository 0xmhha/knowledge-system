# Related-project retrieval evaluation: decisions (2026-11-03)

## Goal

Check whether a retrieval change raises how often the owning function of a go-stablenet symptom question enters
the pack, and confirm on the unseen related project indexer-go that the gain is not fitted to seen cases.
Diagnosis quality is not part of this evaluation. (Confirmed by the user as this sentence.)

## Decisions

- D1. Independent evaluation project: indexer-go only, replacing grpc-go. Reason: shares go-ethereum types and
  vocabulary with go-stablenet, is not public-model prior knowledge, and its author can confirm labels.
  Rejected: indexer-go plus grpc-go (double index cost, grpc-go's English rewrites and public-identifier caveat
  remain), chainbench (many web UI fixes and non-Go files).
- D2. PR selection at indexer-go main `208ff6be1891ea3fe4678f0b46e13a18497735f3`:
  - Mechanical rule: a commit matched by `--grep=^fix` that changes 1-5 non-test Go functions, subject not
    starting with `test(`, `feat(` or `fix(tests)`. 36 commits (indexer-go-fix-candidates.json,
    indexer_go_candidates.py).
  - Symptom filter, judged by a separate agent from the commit subject and body only (no diff, no labels), then
    confirmed by a person: keep a PR only if the wrong behaviour shows at an interface a caller or user sees
    (API response, stored data, error / crash / hang, a race on a production path) and expected vs actual can be
    written. Drop performance-only, refactoring, test-only and documentation-only changes.
  - Rejected: mechanical rule only (keeps changes with no symptom), lifting the 1-5 function limit (labels get
    unstable).
- D3. Problem text: a separate agent rewrites the commit body into a Korean symptom-only text. User-visible names
  (API fields, CLI flags, error messages) stay; internal function, type and file names are removed. A script
  checks the text for internal identifiers of the changed files; a person approves each text. Rejected: the
  user writes the texts (same leakage risk, more of the user's time), commit subject only (states the fix).
- D4. Labels: AI marks the function whose change fixes the symptom as owning and other changed functions as
  related; the user confirms each PR. PRs not confirmed are reported separately and not counted. Approval of
  this scope is not approval of any individual label. Rejected: every changed function counts (side changes
  would count as found).
- D5. Adoption rule: no dev/test split. go-stablenet 11 PRs are the development set (design and tuning);
  indexer-go is judged once per change with the #130 rule: in both arms gains exceed losses and the net over
  both arms is at least 2. Only human-confirmed PRs count. The number of times indexer-go has been used for a
  judgment is recorded. Rejected: odd/even split (about 15 PRs per half).
- D6. Changes that target a failure seen in the development set (a general rule; which rule applies is written
  down per change before measuring):
  - effect: reported on go-stablenet;
  - no harm: on indexer-go, in both arms losses do not exceed gains;
  - risk subset: PRs whose owning function name has 3 or more declarations at the base (indexer-go and
    go-stablenet), 0 PRs lost in both arms.
  - First use: the same-name body limit (knowledge-system 79c938d1). Rejected: the D5 rule as is (could reject
    a harmless change because indexer-go has few same-name groups).
- D7. Scope: retrieval level only; diagnosis quality is a separate evaluation. Rejected: diagnosis runs in this
  evaluation (cost, run-to-run variation, new C1-C4 criteria).
- D8. Carried over unchanged from retrieval-eval-v2: arms `cks-bge-m3-eq` and `cks-qwen3-eq`, metric
  tools/funnel_c.py (owning function in final pack bodies), tools/run_exp.py with a per-PR base-commit
  index, code_query written by a separate agent from the problem text only and frozen before measuring.
  Rejected: bge-m3 arm only (loses the both-arms check).

## Constraints and what is not done

- Selection, problem texts, code_query rewrites and labels are frozen before the first measurement.
- No change is tuned on indexer-go per-PR results.
- indexer-go and go-stablenet repositories are read only.
- No push, PR, merge or deployment without the user's explicit permission.

## Done when

- The symptom-filtered PR list, problem texts and labels are confirmed by the user and frozen, with the
  indexer-go commit and the sha256 of each frozen file recorded.
- Both arms' indexes exist for every confirmed PR's base commit.
- For each change judged: the per-arm gains and losses on indexer-go, the risk-subset losses (D6 changes) and
  the go-stablenet report, each with the binary's commit and sha256.

## Assumptions (not decided by the user)

- Labels and problem-text approvals are given by the user, as the author of indexer-go.
- The 79c938d1 criteria in knowledge-data/retrieval-eval-v2/EXP-same-name.md (R1/R2 on grpc-go) are superseded
  by D6; that file keeps its original text and gains a note.

## Deferred

- Minimum number of confirmed PRs for a judgment to stand.
- A pass criterion for the go-stablenet effect (D6 reports it only).
- Final ambiguity 0.1825 (threshold 0.2), weakest: success, context.

## Record

See export.md (verbatim answers per topic), rounds.jsonl and state.json.
