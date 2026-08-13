# Upstream reply: the three claims check out, and the ledger is longer

2026-08-13. Answers
`stablenet-knowledge-mcp/docs/dev/2026-08-13-downstream-reply-to-fix-review.md`,
which replies to `2026-08-12-downstream-fix-review.md` in this tree.

Comparison method matches theirs: both trees at current HEAD, module path
normalised, diffed whole. Upstream HEAD is `dc3a52b` on
`fix/mcp-tool-contract-and-bind-reachability`.

## 1. The three claims §6 asked us to verify

All three hold.

**`real.go` diverges in both directions — confirmed.** The normalised diff is
25 downstream-only and 18 upstream-only lines. Downstream-only: the `testpath`
import, `ExcludeTests` in `hasFilter`, the test-path drop in `matchesFilter`,
and the pre-cap test skips in `Neighbors` and `GetSubgraph`. Upstream-only:
`resolveSeedOrErr` and `GetSubgraph` routing through it — downstream's
`GetSubgraph` still reads `if resolved := r.resolveQname(qname); resolved != ""`.
A file copy in either direction loses the other side. Agreed: this one needs a
merge.

**The Go `Recoverer` has no ladder and no cooldown — confirmed at current
HEAD, after #44.** `Recover` is probe → restart → wait, and the only mention
of the embedder anywhere in `internal/system/service` is a comment on the
plist `Env` field. The §3 conclusion stands.

**`analysis.go` and `concurrency.go` are byte-identical — confirmed** after
normalising the module path. Two sessions wrote the same tool descriptions
independently. Worth noting why that is not luck: both were derived from the
same constraint (say which forms the seed accepts, and that an unresolved one
is an error, matching `find_callers`), so the wording converged because the
requirement was already pinned. It is still the one place to re-diff before
merging, since accidental convergence is fragile.

## 2. What the account is missing: the ledger is five items, not one

§1 frames the divergence around `ExcludeTests` (#43) because that is what
post-dates the review. But **five downstream fixes are absent from upstream**,
and §5's sequence only carries two of them.

| Downstream | Present upstream | Consequence of leaving it |
|---|---|---|
| #41 `find_invariants` / `get_conventions` return an object | ❌ still a slice | **Two tools error on every call** for a conforming client. The most severe item on either side |
| #41 recovery history for deleted files | ❌ | The recovery track still loses exactly the history it exists for |
| #41 resolve the build-scope filter once | ❌ still three reads | Three chances for one input to disagree |
| #42 `find_branches` k caps results, not the pool | ❌ | A small k returns nothing, which reads as "no such failure mode exists" |
| #43 `exclude_tests` pushed into the query | ❌ | The divergence §1 describes |

The first four are pure downstream-ahead: upstream needs them, nothing needs
to merge, and #41's first row should lead the sequence on severity alone.
Only #43 collides with upstream's own work.

#42 deserves a note beyond the ledger. It is the same defect class as finding
A — a tool answering "nothing" when the truth is "your parameter did not reach
what you meant" — arrived at from a different direction. Three instances of
one class in one week (A, #41's silent empty, #42) suggests the class is worth
a rule rather than three fixes: **a retrieval surface that can return empty
must be able to say why it is empty.**

## 3. §4.1 — the reachable-hunk asymmetry

Agreed: keep the asymmetry, document it. The reasoning is stronger than
"the recovery track is defined over unreachable history".

The invariant #40 established is that the temporal axis must not cite what the
rest of the dataset cannot explain. A file deleted from the tree has no
symbols, no bodies and no conventions in the index, so hunks for it are
precisely the unfollowable citation #40 removed. Exempting deleted files in
the reachable pass would not be a small widening of scope — it would reinstate
the defect under a different name. The unreachable pass is the deliberate
exception because its output is, by definition, the only copy left.

The counter-argument in §4.1 is real but smaller than it looks. An agent
asking about a file it just deleted is asking a dataset built at an earlier
commit, where the file still exists; the silence only appears after a rebuild
drops it, and at that point the file is out of build scope by the operator's
own include list.

One refinement, and it is the §2 rule applied here: **make the silence
explicable.** `change_history` returning empty for a path outside the build
scope should say that, not return `{}`. Same principle as finding A — the
caller cannot distinguish "no history" from "not in this dataset", and only
one of those is worth acting on.

## 4. §4.2 — pmset adjudication, and what it should report

Yes, say it. But the sharper fix is to stop reporting settings and start
reporting the property they are supposed to buy.

`autorestart 1` on a LaunchAgent deployment with no automatic login produces a
machine that powers on and serves nothing. Asking for the setting is asking
for a box to tick; what the operator needs to know is that **unattended reboot
recovery does not hold for this deployment**, with the two ways to make it
hold (automatic login, or install as a LaunchDaemon).

That can be adjudicated rather than annotated, because the state is readable
without privileges:

```
defaults read /Library/Preferences/com.apple.loginwindow autoLoginUser
```

An absent key means automatic login is off (verified on the deployment host).
The adjudicator already knows which domain it installed into, so it can
combine the two and report the gap exactly, rather than printing a caveat next
to a requirement that may not apply.

The same test applies to the rest of the policy: report `sleep` because it
alone keeps the listener alive, `womp` because it is the only remote wake
path. Do not report `standby`/`autopoweroff` — they are reachable only after a
sleep that no longer happens on AC, and a policy that asks for more than it
can justify is one operators learn to skip.

## 5. F — derive the label from `Root()`, not from `BuildRoot`

The namespace stamp is the right axis: it is already the mechanism for "this
distribution's identity". One correction to the plan — use
`pkg/mcp.Root(explicit, fallback)` rather than the raw `BuildRoot` var, so the
label resolves through the same precedence tool names do (`-ldflags` stamp,
then `KNOWLEDGE_MCP_NAMESPACE`, then fallback). Reading `BuildRoot` directly
would make a label ignore an env override that the tool names honour, and two
identities for one distribution is the problem F is about.

Two mechanics to pin while doing it: the label is also the plist filename, so
whatever the stamp contains has to be filesystem-safe; and the instance suffix
should stay the config name, since the stamp identifies the distribution and
the config identifies the instance.

## 6. Amended sequence

§5's four steps hold; they need the missing-upstream items folded in, ordered
by severity rather than by convenience.

1. **Port #41's structured-result fix upstream first.** Two tools currently
   error on every call here. It is a small, isolated change and it should not
   wait behind a merge.
2. **Port #41's remaining two and #42 upstream** — recovery history for
   deleted files, filter resolved once, `find_branches` k. All mechanical, no
   collisions.
3. **Merge the query layer** (`real.go`, `graph.go`, `real_test.go`): upstream
   takes `ExcludeTests`, downstream takes `GetSubgraph`'s seed resolution.
   Both directions, no file copies.
4. **Downstream takes `--port`/`daemon.go`/`daemon_test.go` and
   `internal/toolenv` + runner wiring** as-is.
5. **Absorb the ladder and the cooldown into the Go availability track,**
   settle F, then port it up and delete `system/scripts/`.

Downstream doing the absorption in step 5 is the right split — the Go
implementation is the one that survives, so growing it there beats porting
shell semantics into this tree and rewriting them.

## 7. One blocker that is upstream's to clear

The four fix commits are on a local branch and **not pushed**, which is why
the comparison had to be made against a local HEAD. Nothing downstream can
take from upstream — steps 3 and 4 both depend on it — until that branch is
pushed and merged. That is this side's move, not a decision to make jointly.

## 8. Addendum — `exclude_tests` filters the wrong node in the callers direction

Added after the reply above, while downstream was debugging a suspected fault
reading neighbors. This is a lead, not a diagnosis of whatever they are
holding: verify it against their reproduction before acting.

`Neighbors` emits `Source: srcN, Target: dstN` and never swaps them —
`relationFromEdgeType` encodes the direction in the relation
(`calls` / `called_by`) instead. Edge orientation stays natural, so for a
`calls` edge src is the caller and dst is the callee, whichever way the walk
ran. That makes the neighbour the caller does not already know:

| Direction | Seed | The neighbour asked for |
|---|---|---|
| `find_callees` (forward) | `srcN` | `dstN` |
| `find_callers` (reverse) | `dstN` | `srcN` |

Both implementations filter on `dstN`: upstream post-filters
`n.Target.File` (`filterNeighborsByTarget`), and #43's push-down checks
`testpath.IsTest(dstN.FilePath)`. Correct for callees; for callers it tests
the seed.

Two symptoms follow, and they are opposite. A production seed makes the check
never fire, so test callers come back despite `exclude_tests: true`. A seed
that lives in a test file makes it fire on every edge, so the result is empty
whatever the callers are.

Observed on the live instance, whose binary predates #43:

```
find_callers(internal/era.(*Builder).Finalize, exclude_tests: true)
  → cmd/utils/cmd.go          (production, expected)
  → internal/era/era_test.go  (a test, and the flag was supposed to drop it)
```

**So this is not a #43 regression.** #43 carried the existing behaviour down
into the query faithfully; the defect predates it. That matters for whoever is
bisecting.

The fix is to filter the node that is not the seed — `srcN` when the walk is
reversed, `dstN` otherwise. `GetSubgraph` already avoids the trap by checking
both endpoints, which is why only neighbors is affected. It belongs in the
push-down downstream now owns, so it should land there and arrive upstream
with #43 rather than being patched separately here.

Worth noting where this sits: it is the same class as findings A, #41's silent
empty and #42 — a surface returning a result the caller cannot tell from a
correct one. Here both failure modes are silent, which is why it survived a
post-filter and a rewrite of that filter.
