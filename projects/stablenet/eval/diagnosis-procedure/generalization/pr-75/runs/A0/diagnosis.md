# Diagnosis — WBFT future message (seq+1 / last allowed round) dropped instead of backlogged

Retrieval backend: stablenet-knowledge status=ok, serviceable=true; graph+vector indexed_head
e5a6e9e14c1e1d225c341b55798f31cd07b0bfcd == repo HEAD (fresh, no contamination).

## 1. Root cause

The value at issue is the **allowed future-view distance** (`seqDiff = msg.Sequence - curr.Sequence`,
`roundDiff = msg.Round - curr.Round`) checked against `sequenceThreshold = 1` and `roundThreshold = 10`.
The constants are documented as inclusive limits ("Allow up to 1 future sequence", "Allow up to 10
future rounds"), and the helper comments say "returns true if the difference **exceeds** the
threshold". The consume edge — the comparison in the two helpers — uses `>= 0` instead of `> 0`:

- `consensus/wbft/core/backlog.go:52` `isSequenceTooFarAhead`: `seqDiff.Cmp(threshold) >= 0`
- `consensus/wbft/core/backlog.go:58` `isRoundTooFarAhead`: `roundDiff.Cmp(threshold) >= 0`

Because `isTooFarFutureMessage` only calls the sequence helper when `msg.Sequence > curr.Sequence`
(line 67), the smallest possible `seqDiff` is 1. `1 >= 1` is true, so **every** future-sequence
message is dropped and **no** message ever reaches the backlog with a higher sequence. The window
the code allows is effectively zero. For rounds, `roundDiff == 10` (the last round inside the
intended window) is dropped as well.

Failure flow: `handleDecodedMessage` (handler.go:211) → `checkMessage` (backlog.go:103) →
`isTooFarFutureMessage` returns true → `errFutureViewTooFar` is returned at backlog.go:110-111. That
happens **before** the `view.Cmp(currentView()) > 0 → errFutureMessage` branch (line 129), and
`handleDecodedMessage` backlogs only on `errFutureMessage` (handler.go:215-216). So the PREPREPARE,
PREPARE and COMMIT for height H+1 that reach a node still finishing H are discarded silently, with only
the trace log "future message too far ahead in sequence, dropped". When that node advances to H+1,
`processBacklog` (backlog.go:212, called from core.go:289-302 on every state change) has nothing to
replay. The node then waits for a round-change timeout or for retransmissions, which matches the
ticket's "goes through round change / joins consensus late".

This bug has been present since the gate was introduced in commit 035fc8873 ("fix: future msg drop
(#128)"). Both helpers had `>= 0` from the start. Nothing else in the repo uses them, and no test
covers them.

**Competing hypotheses ruled out:**
- *Threshold constant too small (should be 2 / 11).* Ruled out. The constants and their comments
  ("up to 1", "up to 10") match the ticket's expected behavior (seq+1 kept, last round kept). Only the
  comparison operator contradicts the documented intent, and it also contradicts the helpers' own
  comment ("exceeds").
- *Comparing against the wrong base (curr.Sequence vs curr.Sequence+1).* Ruled out. Using
  `curr.Sequence` as the base is correct: the gate asks "how far ahead of my current height". With
  `>` this gives exactly seq+1 allowed and seq+2 or more dropped.
- *Backlog store/replay broken (addToBacklog / processBacklog / toNegatePriority).* Ruled out for this
  symptom. The ticket's distinguishing signal is the "too far ahead ... dropped" trace line. That line
  is emitted only from `isTooFarFutureMessage`, which returns before any backlog code runs. The message
  never reaches the store edge.
- *RoundChange special path.* Not a separate cause. `isTooFarFutureMessage` runs before the
  RoundChange branch (line 114), so ROUND-CHANGE messages at seq+1 or round+10 are dropped by the same
  edge.

## 2. Evidence

- `consensus/wbft/core/backlog.go:44-47` — `sequenceThreshold = 1 // Allow up to 1 future sequence`,
  `roundThreshold = 10 // Allow up to 10 future rounds` (states the inclusive intent).
- `consensus/wbft/core/backlog.go:49-53` — comment "returns true if the sequence difference exceeds
  the threshold"; code `seqDiff.Cmp(big.NewInt(threshold)) >= 0` (**broken edge**).
- `consensus/wbft/core/backlog.go:55-59` — same pattern for rounds (**broken edge, sibling**).
- `consensus/wbft/core/backlog.go:67-69` — the sequence gate runs only when `view.Sequence >
  curr.Sequence`, so `seqDiff >= 1` and `>= 1` always holds, and every future height is dropped. The
  comment on line 68 says a seq+1 message "may be received", which shows it was meant to be accepted.
- `consensus/wbft/core/backlog.go:79-80` — the round gate runs for the same sequence when
  `view.Round > curr.Round`. `roundDiff == 10` is dropped.
- `consensus/wbft/core/backlog.go:108-112` — the gate returns `errFutureViewTooFar` before the
  `errFutureMessage` classification at line 129.
- `consensus/wbft/core/handler.go:211-227` `handleDecodedMessage` — `addToBacklog` only when
  `err == errFutureMessage`. `errFutureViewTooFar` falls through to `return err` (dropped).
- `consensus/wbft/core/errors.go:51` — `errFutureViewTooFar` definition.
- stablenet-knowledge `find_callers(isTooFarFutureMessage, depth=3)`: `checkMessage` (backlog.go:103-180) is
  called by `handleDecodedMessage` (handler.go:211-227) and by `processBacklog` (backlog.go:212-264).
  `handleDecodedMessage` is called by handler.go:102-181 and handler.go:188-209. `processBacklog` is
  called by core.go:289-302.
- `git log -- consensus/wbft/core/backlog.go`: the gate and `>= 0` were introduced together in
  035fc8873 (#128) and have not changed since.
- Runtime probe: a throwaway unit probe calling the two helpers with diffs {1,2} / {9,10,11} was
  attempted but blocked, because the tool needed approval in this session. Not run; nothing was left in
  the tree. The conclusion rests on direct arithmetic: `Cmp(1,1) = 0`, and `0 >= 0` is true.

## 3. Affected sites

| site | role | produces symptom | must fix |
|---|---|---|---|
| `consensus/wbft/core/backlog.go:52` (`isSequenceTooFarAhead`, `>= 0`) | consumer (comparison) | yes — seq+1 dropped | yes (primary broken edge) |
| `consensus/wbft/core/backlog.go:58` (`isRoundTooFarAhead`, `>= 0`) | sibling-path | yes — round+10 dropped | yes |
| `consensus/wbft/core/backlog.go:64-91` (`isTooFarFutureMessage`) | caller | indirectly (via helpers) | no logic change needed. The log text and call shape stay correct once the helpers are fixed |
| `consensus/wbft/core/backlog.go:103-112` (`checkMessage`) | caller | indirectly | no |
| `consensus/wbft/core/handler.go:211-227`, `backlog.go:212-264` | consumers of checkMessage | no (behave correctly once a seq+1 message is classified `errFutureMessage`) | no |

No other references to the helpers or thresholds exist in the repo (grep). The change is local to two
comparisons.

Side finding (not in scope, medium confidence): `toNegatePriority` (backlog.go:266-275) encodes the
priority as `seq*1000 + round*10 + code`. Its FIXME notes that round is limited to 0-99. With
`roundThreshold = 10` this stays well inside the range, so this fix introduces no collision. Messages
with a large absolute round number (≥100, after many round changes) could still alias the priority of
the next sequence. That is pre-existing and unrelated to this ticket.

## 4. Confidence

**High.** The defect is a deterministic comparison whose intent is stated three times in the same file:
the constant comments, the helper doc comments ("exceeds"), and the comment on line 68 about receiving
a seq+1 message. Every observed behavior (seq+1 always dropped, the last round dropped, the trace line,
late join and round change) follows directly from it. Two things would raise confidence further. First,
a unit test showing `isTooFarFutureMessage(seq+1) == true` and `(seq, round+10) == true` on base, and
false after the fix, with `checkMessage` then returning `errFutureMessage` and the message entering the
backlog. Second, a multi-node run in which a lagging validator joins H+1 at round 0 without the
"dropped" trace.

## 5. Suggested direction

Change both helpers to the strict comparison `Cmp(threshold) > 0`, so a difference equal to the
threshold is treated as inside the window. That gives the documented behavior: seq+1 is accepted,
seq+2 or more is dropped, rounds up to +10 are accepted, and +11 or more is dropped. Accepted messages
then fall through to the existing `errFutureMessage → addToBacklog` path and are replayed by
`processBacklog` on the next state change. Leave the threshold constants and the gate's placement
unchanged. Add table-driven tests for `isTooFarFutureMessage` / `checkMessage` at the boundaries
(diff = threshold-1, threshold, threshold+1, for both sequence and round, including the RoundChange
code), and check that a seq+1 PREPREPARE ends up in `c.backlogs`.
