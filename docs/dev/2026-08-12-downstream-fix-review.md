# Reviewing the downstream fix branch, and what it left open

2026-08-12. A downstream deployment repository (a stablenet-specialized
distribution of this tree) opened `fix/mcp-tool-contract-and-temporal-scope`
with five fixes, plus an uncommitted availability track that runs the server
under launchd. This repository shares the code all five touch, so every one of
them was reproduced here before review.

This is a review record: what the five fix, what they left open, and what was
resolved in this repository today.

## The five, and whether this tree carried them

Every defect was confirmed present here by reading the code, not inferred from
the commit message.

| Downstream fix | Defect | Where it lives here |
|---|---|---|
| return an object from `find_invariants` / `get_conventions` | the handlers passed a slice to `NewToolResultStructured`, so `structuredContent` serialised as a JSON array. MCP requires an object, and a conforming client rejects the whole result — both tools errored on every call | `internal/system/mcp/flow.go` |
| report an unresolved impact/concurrency seed | `resolveQname` returns "" for an unknown or ambiguous name; callers passed the raw name on to an exact-match backend, which matched nothing. An empty closure is indistinguishable from "nothing depends on this" | `internal/system/ckgclient/real.go` |
| keep recovery history for deleted files | the build-scope filter was applied to the unreachable-hunk pass too. The include list is derived from the current tree, so a deleted file can never be on it — the scoping dropped exactly the history the recovery track exists for | `internal/graph/buildpipe/temporal_hunks.go` |
| keep a wildcard bind when `--port` moves the server | `overrideListen` read the empty host of `:8930` as "no address" and substituted loopback, so `--port` made a LAN deployment answer only itself | `cmd/cks/mcpcli/serve.go` |
| resolve the build-scope filter once | `Run`, `runCold` and `runIncremental` each re-read `--files-from`; three reads of one input are three chances to disagree | `internal/graph/buildpipe/pipeline.go`, `incremental.go` |

## What the review verified as complete

- **The structured-result fix covers the surface.** All 24
  `NewToolResultStructured` call sites were checked against their argument
  types. Only `FindInvariants` and `GetConventions` returned slices
  (`[]InvariantHit`, `[]ConventionHit`); everything else already returned a
  struct or a map. The downstream fix also tightened `decodeStructured` to
  reject a non-object, so the next occurrence fails in test rather than in a
  client — the original tests passed because they unmarshalled into a slice,
  which accepts either shape.
- **The filter now resolves once**, at `pipeline.go`, and is passed to
  discovery, the cold path and the incremental path.
- **The recovery-hunk exemption is correctly scoped**: one stat per distinct
  path, and a stat error counts as absent, which keeps the hunk. A retained
  hunk costs storage; a dropped one is unrecoverable.

## What it left open

Findings are ordered by how badly a caller is misled.

### A. Only two of three seeded traversals report an unresolved seed — FIXED HERE

`GetSubgraph` still called `resolveQname` directly and passed the raw name
through on failure, so `get_subgraph` kept returning an empty neighborhood for
a name that does not resolve. This is the same defect the downstream fix
describes, on the third surface it names in its own comment.

Confirmed structurally, not by grep: a graph built from the downstream tree
with `ckg build` reports the `calls` edges into `resolveQname` as
`{resolveSeedOrErr, GetSubgraph, test}` after the fix.

### B. `--port` can still relocate a server silently — FIXED HERE

The wildcard case was fixed by removing the `h != ""` test, but the
`err == nil` guard remained: an address that is set and does not parse as
`host:port` still falls back to loopback. That is the same silent loss of
reachability, one case over. The shape is the cause — an override helper that
cannot report a refusal has to guess.

### C. The watchdog has no cooldown and no backoff

`Recoverer.Recover` is "probe, and restart if unhealthy". A permanently broken
instance is therefore restarted every watchdog period for as long as it stays
broken, with each start paying the dataset load (8–13s in the observed logs).
Nothing escalates and nothing widens the interval.

### D. The watchdog cannot fix the failure this host actually produces

`serviceable()` requires `ModelReachable`, so an embedding daemon that is down
makes `/healthz` report 503 while the server itself is healthy. Restarting the
server cannot fix that. Measured on the deployment host: the MCP server had
been up 1d21h without interruption while the embedding daemon had started
minutes after the host woke from a 1h42m sleep — the server survives what the
daemon does not.

The recovery ladder has to start at the dependency: probe the embedder, start
it, re-probe health, and only then consider bouncing the server (which the
server does not need — it recovers on its own once the daemon answers).

### E. The power policy is advisory, and the host currently violates it

`install` and `status` adjudicate `pmset` and print the one privileged command
that fixes every violation, which is the right division (a server binary should
not be asking for root). But nothing enforces it, and on the deployment host
`sleep` was 1 and `autorestart` 0 against a required 0 and 1. The
`caffeinate` assertion covers the window while the server runs; the minute it
stops, an idle sleep is one minute away.

### F. The launchd label prefix is branding in source

`LabelPrefix` is a hardcoded distribution name. `docs/downstream-sync.md` §3
requires deploy identity to be injected through the build and config, never
edited into source, so this blocks the port upstream as written. Derive it from
the namespace or the config name.

### G. `--port` can be silently ignored by the daemon verbs

`cks mcp up --port` only reaches the child when the registry entry names no
address; otherwise the flag is dropped without a word.

### H. The launchd job has no environment contract

The rendered plist carries no `EnvironmentVariables` at all, because the
installer populates that map from exactly one optional variable. The job
therefore runs with launchd's default `PATH=/usr/bin:/bin:/usr/sbin:/sbin`,
which has no `go`.

Serving is unaffected — it reads a prebuilt dataset and finds the engine
binaries through absolute config paths — so the gap is invisible until someone
rebuilds. `cks.ops.index` / `cks.ops.reindex` run the build as a subprocess
that inherits that environment, and Go discovery shells out to `go list`:

```
$ env -i PATH=/usr/bin:/bin:/usr/sbin:/sbin ckg build --src <fixture> --out <dir> --lang go
ckg: detect go: packages.Load: go command required, not found:
     exec: "go": executable file not found in $PATH
```

The same omission means `CKV_OLLAMA_ENDPOINT` only reaches the server if it
happened to be exported in the installing shell, so one config can mean two
different things depending on how the server was started.

### An open question, not a finding

The reachable-hunk pass keeps no exemption for deleted files, so
`change_history` on a file removed from the tree returns nothing while the
recovery track still holds its unreachable history. That may be the intended
line — the recovery workflow is defined over unreachable history — but it is
an asymmetry worth confirming rather than inheriting.

## Fixed in this repository today

**A — one seed-resolution contract for every seeded traversal.**
`ErrSeedUnresolved` is added to the ckgclient contract, and
`resolveSeedOrErr` names which failure happened, because the caller's next
move differs: an unknown name needs a different spelling, an ambiguous one
needs the `canonical_id` that `find_symbol` reports. `ImpactOfChange`,
`ConcurrencyImpact` **and `GetSubgraph`** all route through it. A seed that
resolves but that the index does not cover still returns empty — that is a
real answer. The three tool descriptions now state which forms the seed
accepts and that an unresolved one is an error.

The regression test exercises all three surfaces in one case table on purpose:
the failure being guarded against is one of them drifting back to the silent
form while the others report.

**B — `overrideListen` refuses instead of guessing.** It returns an error now.
An unset address still falls back to loopback, an empty host is preserved as
the wildcard form, and an address that does not parse stops the launch with
the configured value quoted. Tests cover the wildcard case and both malformed
shapes, and assert the config is not mutated on the error path.

**H — the toolchain is resolved from the machine, not recorded.** The fix
belongs at the boundary where it bites: `internal/setup`'s subprocess runner,
which is the single place every build child is spawned from. It now composes
that child's PATH through the new `internal/toolenv`, which locates `go` and
`git` by asking, in order, the inherited PATH, the operator's login shell
(`$SHELL -lc "command -v go"`), and a short list of standard install
locations. `GOPATH` is carried over from the login shell when the caller left
it unset, so a child reaches the module cache the operator's own builds filled
instead of re-populating `~/go`, and `GOTOOLCHAIN=auto` is set the same way.

Writing the absolute paths into the launchd job at install time would have
worked once. It would also have embedded one machine's home directory in a
generated file and died the next time a version manager switched releases —
`~/.gvm/gos/go1.25.11/bin` is not a stable address. Resolving instead of
recording keeps every committed artifact free of machine paths and lets each
host answer for itself; verified end to end by running `cks setup` under
launchd's own environment (`env -i PATH=/usr/bin:/bin:/usr/sbin:/sbin`), where
it resolved the current GVM release through the login shell and built. With no
usable shell either, the run reports `not found on this machine: go` before
the build starts rather than failing inside go/packages.

**G — `up` stops narrowing the binding in silence.** The finding understated
this one. `--port` was not merely ignored on the `up` path: the registry
always resolves an address, that address became `--http-addr`, and the flag
was dropped without a word — while the registry's default bind is loopback.
An operator asking for a port therefore got a different port on an address
no other machine can reach, with nothing in the output saying so. The
deployment exists to serve agents across a subnet, so that is a functional
failure, not a cosmetic one.

Three changes, one rule: never serve somewhere other than where the operator
asked, and never stay quiet about reachability.

- `--port` pins the port of a single-instance registry, and is refused with
  the count when the registry declares several (their ports are the
  registry's to assign). `--http-addr`, which the same path also ignored, is
  refused too.
- `up --lan` binds every interface. It deliberately does not pin the detected
  LAN IP the way `gen-config --lan` does: a pinned address stops resolving
  when DHCP moves it and the instance then fails to bind at all. The printed
  URL still shows a routable address, because `netutil.AdvertiseHostPort`
  resolves the wildcard for display.
- An instance that lands on loopback prints that it did, and what to change.
  Loopback stays the default — widening it is a security decision an operator
  must take deliberately — but silence about it is a different matter.

`start`/`restart` now resolve `--port` against the config before handing the
address to the supervisor, through the same helper the server uses. The
address the supervisor records is therefore provably the one the child binds,
which also closes a smaller gap: an instance started with `--port` used to
have no recorded address, so `status --ready` showed no URL for it and
`reload` reported it as not running.

Verified end to end: `up --lan --port 8977` produced `http_addr: 0.0.0.0:8977`
with `allow_remote: true` derived, `lsof` showed `cks *:8977`, and `--wait`
reported it serviceable.

C–F remain open. C, D and H are the availability track; E is an operational
decision (whether to take the privileged `pmset` change); F is a prerequisite
for porting the launchd work into this repository; G is a small clarity fix.

## When downstream settles

The downstream branch is still moving, so no code is being ported in either
direction yet. The re-review starts when that work is called done, and these
are the positions it starts from rather than re-derives.

**The availability track belongs upstream.** Process supervision is already an
upstream concern — `internal/system/daemon` holds the pidfile supervisor, the
instance registry and the blue-green reload — and nothing in launchd,
caffeinate or pmset is specific to one project pack. Leaving the launchd layer
downstream splits one concern across two repositories, and the current state
is the worst of both: upstream carries shell scripts for this, downstream
carries Go, and they can only diverge. The open question is when and in what
shape, not whether.

Three things travel with that port:

- the shell scripts under `system/scripts/` are deleted, not merged — the Go
  implementation is better on every axis that was compared (tests, `--takeover`,
  pmset adjudication, SSH forced-command);
- one supervisor is declared the owner. Today launchd's `KeepAlive` and
  `cks mcp stop` fight over the same process, and `--takeover` exists because
  of that overlap;
- finding F is settled on the way in, since a hardcoded label prefix is
  exactly what `downstream-sync.md` §3 exists to prevent.

**C is lower priority than it looks.** A watchdog with no cooldown restarts a
broken instance roughly every 90–150s, which is a slow loop, not a storm —
launchd's `ThrottleInterval` already covers fast crash respawn. The real gap is
that repeated failure notifies nobody; a cooldown alone just lengthens the
silence. Pair it with alerting or leave it.

**D is coupled to E.** Restoring the embedder is the fix for the failure that
was actually measured, but E removes its cause. Both, if the deployment is
serious; E first, if only one.

**E, the recommended default.** Narrower than the downstream policy on purpose:
this host is a workstation that also serves, so the AC profile is the only one
that should change, and a policy asking for more than it can justify is a
policy operators ignore.

| Setting | Value | Scope | Why |
|---|---|---|---|
| `sleep` | 0 | AC only (`-c`) | the only setting that keeps the listener alive; caffeinate covers nothing outside the server's own lifetime |
| `autorestart` | 1 | all (`-a`) | returns after a power cut |
| `womp` | 1 | all (`-a`) | lets a remote machine wake it if it does sleep |
| `standby`, `autopoweroff` | unchanged | — | only reachable after a sleep that no longer happens on AC |
| `disksleep`, `displaysleep` | unchanged | — | unrelated to serving |

`autorestart` is only worth setting alongside automatic login or a
LaunchDaemon, because a LaunchAgent needs a session; and a closed lid still
sleeps regardless.

**One finding still unfixed here.** `gen-config --lan` pins this host's
detected LAN IP into the config. When DHCP moves that address the server
cannot bind it at all, and under launchd that becomes a KeepAlive retry loop;
a pinned address also stops answering on loopback, which breaks every health
probe that assumes it. `up --lan` binds the wildcard for both reasons. The two
flags should agree, but changing a documented flag's meaning was left for a
deliberate decision.

## Evidence

- Structure: `ckg build --src <downstream> --lang go` → 87,216 nodes /
  225,504 edges; `calls` edges queried per changed function. Every new
  function has exactly one production call site plus tests — no dead code, and
  `reportPower` is reached from both `install` and `status`, so the power
  adjudication is wired rather than merely tested.
- PATH: the stripped-environment build above.
- Availability: process start times on the deployment host (server 1d21h,
  embedding daemon minutes after wake), `pmset -g log` sleep/wake history, and
  `pmset -g assertions` confirming the launchd `caffeinate` holds
  `PreventUserIdleSystemSleep` and `PreventSystemSleep` on the server's behalf.
