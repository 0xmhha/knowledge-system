# Diagnosis — WBFT future-message (height+1 / last-round) dropped instead of backlogged

Retrieval backend: stablenet-knowledge OK / serviceable (ckg+ckv), indexed_head = e5a6e9e14 = repo HEAD (no contamination, not stale).

## 1. Root cause

The "how far ahead is too far" check in `consensus/wbft/core/backlog.go` has an off-by-one error. Both
`isSequenceTooFarAhead` (line 52) and `isRoundTooFarAhead` (line 58) treat a difference **equal to**
the threshold as too far: they use `diff.Cmp(threshold) >= 0`. The constants say the opposite:
`sequenceThreshold = 1 // Allow up to 1 future sequence` and `roundThreshold = 10 // Allow up to 10 future rounds`
(lines 45-46). The function comments say "returns true if the difference **exceeds** the threshold"
(lines 49, 55). The call site comment (line 68) says a message for seq+1 "may be received" during the
early phase of block consensus.

The value involved is the view difference `msg.View - currentView` (sequence, and round within the
same sequence):
- produce: `Core.currentView()` (core.go:123), from `c.current`.
- consume / decide: `isTooFarFutureMessage` (backlog.go:64) → `checkMessage` (backlog.go:110) returns
  `errFutureViewTooFar`.
- store (backlog): `handleDecodedMessage` (handler.go:211-227) calls `addToBacklog` only when
  `err == errFutureMessage`.

`errFutureViewTooFar` is a different error, so the message is not backlogged and is silently dropped.
The broken edge is the **consume/compare** edge, the `>=` at backlog.go:52 and backlog.go:58. Results:
- seq diff = 1 (height+1 PRE-PREPARE/PREPARE/COMMIT/ROUND-CHANGE): the check drops the message,
  although it should go to the backlog. With a threshold of 1 and `>=`, **no** future-sequence message
  is ever accepted. The `view.Sequence > curr.Sequence` branch can only drop, so the "allow 1 future
  sequence" intent is dead code. A node that finishes block N late loses the height N+1 proposal and
  votes. It then waits for timeout and round change, or catches up later. This matches the ticket
  symptom and the "future message too far ahead in sequence, dropped" trace.
- same seq, round diff = 10: the check drops the message, although it is the last round of the allowed
  window. Rounds +1..+9 are accepted and +10 is dropped. This is the second symptom in the ticket.

Both dimensions have the same bug: the same `>= threshold` pattern is copied into both helpers, so the
height and round bounds are consistently wrong (both are exclusive where the intent is inclusive).

**Competing hypotheses ruled out:**
- *H2 (store edge): `addToBacklog` / `processBacklog` loses height+1 messages.* Ruled out. The message
  never reaches `addToBacklog`, because `checkMessage` returns `errFutureViewTooFar` before the
  `errFutureMessage` branch (backlog.go:110 runs before the checks at 114-131). `handleDecodedMessage`
  backlogs only on `errFutureMessage`. The trace string in the ticket comes from
  `isTooFarFutureMessage`, not from the backlog code.
- *H3 (produce edge): `currentView()` is stale (sequence not yet advanced), so the diff is inflated.*
  Ruled out. A stale-low current sequence with a message at the true next height gives a diff of 2,
  which should be dropped anyway. The ticket's case is diff = 1 against a correctly advanced (or
  lagging-by-one) node, and diff = 1 is dropped only because of `>=`. The round-window symptom
  (+10 dropped) has nothing to do with sequence staleness, which also points to the shared comparator.
- *H4: the threshold constants are wrong (should be 2 / 11).* Rejected as the cause. The constant
  comments ("Allow up to N") and the helper comments ("exceeds") agree with each other. Only the
  operator disagrees with them.

## 2. Evidence

- `consensus/wbft/core/backlog.go:44-47`: `sequenceThreshold = 1 // Allow up to 1 future sequence`,
  `roundThreshold = 10 // Allow up to 10 future rounds`.
- `consensus/wbft/core/backlog.go:49-53`: `isSequenceTooFarAhead` returns `seqDiff.Cmp(big.NewInt(threshold)) >= 0`.
  The doc comment says "exceeds".
- `consensus/wbft/core/backlog.go:55-59`: `isRoundTooFarAhead` has the same `>= 0` and the same "exceeds" doc.
- `consensus/wbft/core/backlog.go:67-77`: the sequence branch, with the comment "a message with a sequence number one
  higher than the current sequence may be received". It logs "future message too far ahead in sequence, dropped".
- `consensus/wbft/core/backlog.go:79-88`: the round branch (same sequence only). It logs "... in round, dropped".
- `consensus/wbft/core/backlog.go:108-112`: `checkMessage` returns `errFutureViewTooFar` before any `errFutureMessage` path.
- `consensus/wbft/core/backlog.go:119-120, 129-131`: without the drop, seq+1 (any code, including ROUND-CHANGE) would
  return `errFutureMessage`, and so would be backlogged.
- `consensus/wbft/core/handler.go:211-227`: `handleDecodedMessage` calls `addToBacklog` only on `errFutureMessage`,
  so `errFutureViewTooFar` causes a drop (the error is returned and handler.go:130-131 `continue`s, with no gossip).
- `consensus/wbft/core/errors.go:51`: `errFutureViewTooFar` "sequence or round difference too large".
- Graph edges (stablenet-knowledge `find_callers isTooFarFutureMessage`, depth 2): `isTooFarFutureMessage` ← `checkMessage`
  (backlog.go:103-180) ← {`handleDecodedMessage` handler.go:211-227, `processBacklog` backlog.go:212-264}.
  No other callers.
- History (`git log -- backlog.go`): the check was introduced in 035fc8873 "fix: future msg drop (#128)",
  with the `>= 0` comparator from the first version. The intent was to drop only *excessive* lead
  (comments unchanged since then).
- No existing unit test covers `checkMessage` or `isTooFarFutureMessage` (`*_test.go` in consensus/wbft/core: only
  extraseal_test.go and justification_test.go). That is why the boundary was never exercised.
- Runtime probe: I tried a throwaway in-package Go test that calls `isTooFarFutureMessage` for
  (seq+1,r0), (seq+2), (r+9), (r+10), (r+11). It was **denied by the session permission policy and not
  run**. The conclusion rests on the pure comparison semantics: `big.Int.Cmp(1,1)=0`, so `>=0` is true
  and the message is dropped.

## 3. Affected sites

| site | role | produces symptom | must fix |
|---|---|---|---|
| `consensus/wbft/core/backlog.go:52` `isSequenceTooFarAhead` (`>= 0`) | consumer / comparator (sequence dimension) | yes: height+1 messages dropped | yes |
| `consensus/wbft/core/backlog.go:58` `isRoundTooFarAhead` (`>= 0`) | consumer / comparator (round dimension), sibling path | yes: round+10 dropped | yes |
| `consensus/wbft/core/backlog.go:64-91` `isTooFarFutureMessage` | the function behind the "dropped" trace (both observed behaviours) | yes (caller of both) | review only (logic correct apart from the helpers) |
| `consensus/wbft/core/backlog.go:103-112` `checkMessage` | gate that maps the drop to `errFutureViewTooFar` | indirectly | no |
| `consensus/wbft/core/handler.go:211-227` `handleDecodedMessage` | backlog decision (only `errFutureMessage` is backlogged) | indirectly (drop sink) | no |
| `consensus/wbft/core/backlog.go:212-264` `processBacklog` | re-checks backlogged messages through `checkMessage` and skips `errFutureViewTooFar` | shares the comparator (consistent after fix) | no |
| `consensus/wbft/core/backlog.go:266-275` `toNegatePriority` | backlog priority (assumes round < 100) | no; a window of +10 rounds stays within the encoding range | no |

## 4. Confidence

**High.** The defect is a pure comparison that contradicts three in-code statements of intent (the
constant comments, the helper doc comments, and the call-site comment), and it explains both reported
symptoms with one operator.

These would raise confidence further:
- A unit test on `isTooFarFutureMessage` / `checkMessage` with currentView (S, R) asserting that
  (S+1, 0) gives `errFutureMessage` (today `errFutureViewTooFar`), (S+2) still gives `errFutureViewTooFar`,
  (S, R+10) gives `errFutureMessage` (today dropped), and (S, R+11) gives `errFutureViewTooFar`.
- An e2e run (chainbench, 4 validators) with one validator delayed at block commit, showing the trace
  "too far ahead in sequence ... diff=1" and a round change on that node before the fix, and none after.

## 5. Suggested direction

Make the two helpers match their contract ("exceeds the threshold"). Change the comparator in
`isSequenceTooFarAhead` and `isRoundTooFarAhead` from `>= 0` to `> 0`, so that a difference equal to the
threshold is allowed: seq+1 and round+10 go through to the normal `errFutureMessage` path and into the
backlog, while seq+2 and round+11 are still dropped. Keep the two dimensions on the same inclusive
semantics. Add boundary table tests (diff = threshold-1, threshold, threshold+1 for both sequence and
round) in `consensus/wbft/core`.

**Rejected directions:**
- *Remove `isTooFarFutureMessage` or route `errFutureViewTooFar` into the backlog.* This contradicts the
  stated design intent at backlog.go:62-63 and 108-109: "prevents the node from processing messages that
  are excessively ahead, which could disrupt the normal flow of the consensus process". The intent is
  sound. Only the boundary is wrong.
- *Bump the constants to 2 / 11 and keep `>=`.* It gives the same behaviour, but it leaves the constant
  comments ("Allow up to 1/10") and the helper docs ("exceeds") describing something other than what
  the code does. Fixing the operator keeps the names and comments truthful.

## Side findings (non-blocking, not in scope)

- low: `processBacklog` (backlog.go:246-256) silently discards backlogged messages that later become
  `errFutureViewTooFar`. After the fix this can only happen if the node's view moves backwards, which
  does not happen normally, so there is no action.
