# Remaining Work — consolidated, code-verified

> Tier 2 (living index). Single source of truth for what is left. Supersedes the two
> dated snapshots that drifted apart:
> - [`remaining-work-2026-07-10.md`](./archive/remaining-work-2026-07-10.md) (audit E1–E5 / M1–M7)
> - [`session-handoff-2026-07-10.md`](./archive/session-handoff-2026-07-10.md) §4 (P0–P4 priorities)
>
> Those two disagreed on M2 (one said "ready now", the other "blocked by degraded
> serving"); this file resolves the conflict against the code. Keep this file current;
> let the dated files stand as historical record.
>
> **Verified**: 2026-07-12 on `main` (after PR #33 squash-merge).
> Re-confirm each item's evidence before starting — the tree changes fast.
>
> **Re-verified 2026-07-13 (office machine, `136eed3`)**: code claims hold — go.mod has
> NO replace, `CKGNodeID` grep = prose comment only, build+tests clean, tool fixture = 19.
> **New finding: the "Live serving" note below is
> machine-scoped** — it was verified on the 192.168.0.x machine. THIS machine
> (<office-host>) still runs the stale 7/10 instance (`cks-mcp/0.1.0-1de321e5-dirty`,
> dataset `pr-77-2`, `serviceable:false` — fail-loud, so consumers are safe) and has
> NO `pr-77-gstable` dataset locally. See the new **P0-officeM** row.
>
> **Re-verified 2026-07-19 (`e436fae`, after #43/#44)**: go.mod now pins the tagged
> releases **`code-knowledge-graph v0.1.0` + `code-knowledge-vector v0.1.0`** (no replace)
> — the earlier pseudo-version hashes are superseded. #44 was a comment-only fix; #43
> landed the knowledge-reserve doc-kind rescue (`internal/system/composer/budget/allocator.go`)
> whose **deploy step is still pending** (see "Open items outside prior scope" below).
>
> **Re-verified 2026-08-20 (`378c4c9`)** — this pass corrected four rows that had
> gone stale, so trust them over the older summaries above: PR **#28/#29 are
> merged** (the table said "in review"); **P0-officeM is closed**; the M7 anchor
> path moved and its counts were wrong; and the "Quick resume checks" block named
> **three scripts that no longer exist**. What landed since is in
> §"Availability + cross-repo lockstep". The office host's address reads
> `<office-host>` because #94 redacted it repo-wide — that is the final form, not
> a placeholder awaiting a value.

---

## Remaining work (2026-08-20)

The downstream port that headed this list since 2026-07-28 is **done**, and the
relationship it set up now runs both ways: upstream **#95** took four
graph-surface fixes back *from* the distribution (#50/#52/#53/#54). Go source is
fully converged — of 1,751 common files, zero `.go` files differ once the module
path is rewritten, and 7 differ only in import order.

What is left is two documentation items, three operator actions, and two that
belong to other repositories:

| # | Item | Severity | Where |
|---|---|---|---|
| **D-1** | **Cross-tree documentation drift** — 71 files differ in content (39 archive, 32 live). Not branding: they are **stale path references** left by the consolidation, and they point **both ways**. | [권장] | both trees |
| **D-2** | **Pack knowledge in engine code** — `cmd/cks/domaincli/worksheet.go` holds a promotion catalog that is pack-level domain knowledge. The boundary check exempts it by name rather than silencing it. | [권장] | this repo |
| **OPS-1** | **`.network` agent not installed** — the serving host runs 2 of the 3 agents `service install` writes. | [권장] | serving host |
| **OPS-2** | **`ops.index` writes live** — the alignment gate added in #47/#91 reports corruption when it happens but leaves no prior version to fall back to. | [권장] | this repo |
| **OPS-3** | **pmset policy is broader than needed** — narrow to AC-only, and report the property (unattended reboot recovery) rather than the setting. | [권장] | this repo + host |
| **KR-deploy** | Task 2 — restart the remote MCP on the new binary and re-capture the live doc-kind rescue. A **remote operator action**, not code. | [권장] | remote deployment |
| **M2-multi** | The A/B/C total-cost benchmark. Harness lives in `../coding-agent`; large, explicit opt-in. | [권장] | coding-agent + chainbench |

D-1, D-2, OPS-2 and OPS-3 are detailed in §"Open items outside prior scope".
Everything else below is closed or historical; see the tables for evidence.

---

## ckg_node_id retirement — code done, data-side still open

The retirement landed in code via **PR #33 (squash-merged to main, 2026-07-12)**:

1. **M6 ✅ — code retirement merged.** ckv `origin/main` (`7f62683`) dropped the
   `CKGNodeID` field; cks dropped `Hit.CKGNodeID` + the `real.go` mapping + comment
   sites + the b7-test observation. `grep ckg_node_id|CKGNodeID` → only a prose comment.
2. **M1′ ✅ — go.mod pinned, no replace.** ckg/ckv now pinned to the tagged releases
   `code-knowledge-graph v0.1.0` / `code-knowledge-vector v0.1.0` (2026-07-19; was the
   column-removed pseudo-version `7f6268307669`). Reproducible on CI / other machines.

**Data side — ✅ closed (2026-07-12).** PR #33 closed the *code* side of ADR-0001; the
*served* index is now `pr-77-gstable/vector-db`, built by the column-removed ckv, so the
`ckg_node_id` column is physically gone (verified: served binary `cks-mcp/0.1.0-90dc885d`,
`serviceable:true`). The stale-binary failure mode fired on the first cutover and was caught
fail-loud — see `M6-data` in the table.

Also separate: cks-seminar deck/asset `ckg_node_id`→`canonical_id` sync (that repo).

---

## Consolidated task table

Severity: `[중요]` high / `[권장]` recommended. Status verified against code on 2026-07-12.

| ID | Task | Severity | Status (verified) | Gate / prerequisite |
|---|---|---|---|---|
| **P0** | Serve the current dataset with a fresh, aligned index. | [중요] | ✅ Done (2026-07-12) via **cutover, not a fresh reindex** — the docs' premise was stale (see note below). Cut serving over to `pr-77-gstable` (already built by another session: column-removed ckv + sources ledger, ckg schema 1.23, commit `0bf2f4d1b`). Verified: `serviceable:true`, `alignment.ok:true` (digest actual==expected, source_root_ok), `builder_version cks-mcp/0.1.0-90dc885d`. | — |
| **M6-data** | ADR-0001 data-side close: the served index must be built by the column-removed ckv so `ckg_node_id` is physically gone. | [권장] | ✅ Done (2026-07-12) — served index (`pr-77-gstable/vector-db`) has no `ckg_node_id` column, and the serving binary was rebuilt from column-removed main. The predicted failure mode fired and was caught: the first cutover with a **stale `bin/cks-mcp`** hit `ckv.Open: no such column: ckg_node_id → degraded` (fail-loud); rebuilding `bin/cks-mcp` from current main resolved it. | — |
| **M6** | Retire `ckg_node_id` (cks code side): drop `Hit.CKGNodeID`, `real.go` mapping, comment sites, JSON-contract note, reflect in `symbol-identity-design.md`. | [권장] | ✅ Done (PR #33, 2026-07-12) — build + tests clean. Data side tracked as `M6-data`. | — |
| **M1′** | Remove committed `replace ckv => ../` and restore a proper module pin. | [중요] | ✅ Done (PR #33, 2026-07-12) — ckv pinned to `7f6268307669` (origin/main). | — |
| **M2** | Run the cks bench arm (cks-bench live, `pr-77-gstable`, 30 golden Qs). | [권장] | ✅ Done (2026-07-12). **Result: cks retrieval ~doubles correctness vs the fair (grep-only) baseline.** vs `M1_fair` (grep-only, 33.3% correct, 12,883 tok, 0 halluc): cks auto-pack `M4_get_for_task` = 70.0% correct / 4,899 tok (−62%); `M3_incremental` = 73.3% / 1,780 tok; `M2_graph_full` = 56.7% / 35,891 tok. Oracle ceiling `M1_raw` (golden files injected) = 93.3%. **Hallucination caveat resolved:** the apparent M3=19/M4=16 hallucinations were a scorer artifact — cks methods cite domain-knowledge docs (which exist in the cks corpus, not the go-stablenet tree), and the scorer flagged them as fabricated code. Fixed in coding-agent #61; re-scored (no new LLM calls) → **M3=0, M4=0 code fabrications**. So cks retrieval ~doubles correctness over grep-only with zero hallucination. N=30, single run; single-turn Q&A ≠ the coding-agent bug-cycle-cost thesis. | — |
| **E4** | `symbol-identity-design.md` §7 — mark Phase 1/2 complete; only remaining is M7. | [권장] | ✅ Done (2026-07-12). | — |
| **E5** | `coordination-response-cks-2026-06-29.md` T1 overstated the 2 knowledge tools as shipped with the flow-4. | [권장] | ✅ Done (2026-07-12) — added a dated correction: find_invariants/get_conventions shipped separately via M5 (cks #34 + ckv facade #35), so T1's 6 tools are now all exposed. | — |
| **M7** | Domain-knowledge anchor `kind:` migration (def vs loc). | [권장] | **Deferred — needs the source-of-truth commit.** (Path corrected 2026-08-20: the entries live at `projects/stablenet/domain-knowledge/entries/` — **44 files, 5 carrying `kind:`**, not the 43/2 the evidence pointer claimed.) ~150/164 anchors are def (back-compat correct, no change); only a handful are loc. Accurate def/loc classification = "is `line` the declaration of `symbol`?", which must be checked against go-stablenet **at the commit the entries were authored against** (line numbers drift). The reason-text heuristic is unreliable — it cannot distinguish "def of X" from "loc using X" and produces false positives (e.g. `NativeCoinManagerAddress:219` reads as loc but is a def; `ExtractWBFTExtra:251` names the *called* symbol, not the enclosing one). Blind bulk editing would corrupt curated knowledge. | Pin the authoring go-stablenet commit, then do a source-verified pass. Back-compat working meanwhile — no functional issue. |
| **M3** | T7 — composer causal orchestration (multi-hop `expand_flow`). | [권장] | **Spec = draft, insufficient (review 2026-07-13)**: the "approved" header was self-declared; a sufficiency review against the ORIGIN requirements (coding-agent agreed shape: invalidator-annotated produce→store→consume graph; root-cause-lifecycle: exhaustive copy/cache enumeration) found gaps **G1** flow-corpus-only anchoring cannot even seed the known validation bug (`get_flow(SetCurrentBlock)`=no flow — needs ckg fallback/fusion), **G2** no invalidator annotation (the agreed core output), **G3** no exhaustiveness contract, **G4** invariant-violation candidates dropped, **G5** no problem-grounded acceptance (GasTip case), **G6** (minor) value-identity matching is string-contains only. Review: `system/docs/superpowers/specs/2026-07-13-t7-spec-review.md`. Remaining: ① revise spec (G1–G6) ② review/approve ③ impl plan ④ implement. | Avoid clashing with M2 measurement freeze. |
| **M4** | Embedding-dimension measurement. | [권장] | Waiting. | External: reindex-B (qwen3) index, CKV-owned. |
| **P0-officeM** | Office machine (<office-host>) serving is stale: 7/10 dirty binary + `pr-77-2` (degraded since the 7/10 restructure), and `pr-77-gstable` is absent locally. Decide topology: (a) stop the local instance and point office `CKS_MCP_URL` at the 192.168.0.x host, or (b) sync/build `pr-77-gstable` here + `make build-bins` + restart. Fail-loud keeps consumers safe meanwhile. | [권장] | **✅ Closed (2026-08-20).** Settled as option (b), reached from the other direction: the host now builds and serves its own dataset out of `../knowledge-data`, supervised by launchd instead of a shell. Verified live — agent `com.stablenet.knowledge.go-stablenet` running under `caffeinate`, `/healthz` → `serviceable: true, status: ok`. The topology question is therefore answered **per-machine**, and availability became a supervised property rather than an operator habit (see §"Availability + cross-repo lockstep"). | — |
| **M5** | Expose `find_invariants` / `get_conventions` as dedicated tools. | [권장] | ✅ Done (2026-07-12). cks: FlowClient + MCP tools (cks #34 + ckv facade #35, repin #35). Live e2e against `pr-77-gstable`: `find_invariants` → 151 real invariants (file/tier filtered), `get_conventions` → per-package idioms. coding-agent: analyzer granted both tools + prompt pointer (coding-agent #60, 0.1.53) — the consumer gap that blocked the diagnose path. Only an autonomous-diagnose *observation* is left as an optional demo (plumbing proven end-to-end; needs a plugin reload + a full diagnose run). | — |

**Resolved (no rework):** E1, E2, E3, M1, **M6 + M1′ + E4** (2026-07-12), **M5**
(cks #34/#35 + coding-agent #60; live e2e proven, autonomous-diagnose demo optional),
**P0 + M6-data** (2026-07-12, cutover to `pr-77-gstable`), **M2** (2026-07-12, cks-bench
live: cks ~2× correctness over grep-only; hallucination follow-up noted).

**Live serving (superseded 2026-08-20).** The 2026-07-12 note described a
shell-started `cks-stablenet` instance on a pinned LAN IP, regenerated by
`scripts/gen-cks-config.sh` — a script that **no longer exists**. Current shape:
the `go-stablenet` instance is a launchd agent on the office host, started as
`caffeinate -s -i bin/cks mcp --config cks.yaml --name go-stablenet`, bound to
the wildcard address rather than a pinned IP so it survives a DHCP change, and
serving a dataset under `../knowledge-data` selected by the `current` symlink.
`cks.yaml` is gitignored; generate it with `cks mcp gen-config` and install the
agents with `cks mcp service install --config cks.yaml`. Addresses are
deliberately absent here — #94 removed them repo-wide. Read the live state with
`cks mcp service status --config cks.yaml`, never from this file.

**Ground-truth note (docs drift):** the old P0 plan (reindex `pr-77-2`, `SRC=vector-db-5`, "serving
degraded") was stale. Actual: serving was healthy on `pr-77` (pre-retire, still had the column); the
new `pr-77-gstable` had already been built by another session with the column-removed + sources-ledger
ckv. P0 became a **cutover + binary rebuild**, not a reindex.

**Recommended order (2026-08-20):** `OPS-1 → D-1 (live 32) → OPS-2 → D-2 → OPS-3 → M3`.
OPS-1 first because it is the only item where the gap is currently *live* — the host
is one network move away from an outage no supervisor would catch — and it costs one
command. D-1 next because it is the largest and it grows: every cycle that ports code
without reconciling docs widens it, and the fix should include widening `docs-check`
so the drift cannot silently resume. OPS-2 needs a design answer before code. M3 (T7)
remains the substantial feature item; M4 and M7 stay externally blocked (M4 waits on a
CKV-owned qwen3 index, M7 on pinning the authoring go-stablenet commit). The core
items (P0, M2, M5, M6) are done, including the M2 hallucination root-cause (a scorer
artifact, fixed in coding-agent #61). M5's optional autonomous-diagnose demo can
piggyback on any diagnose run — it needs a plugin reload, not dedicated work.

---

## Open items outside prior scope (2026-07-19, extended 2026-08-20)

Genuinely open, and outside the E/M lineage above. The first four came from the
2026-07-19 docs review; **D-1, D-2 and OPS-1/2/3 were added 2026-08-20** by the
availability + lockstep cycle. Each row states how its status was established,
because two of them contradict what this file said before.

| ID | Task | Severity | Status | Source |
|---|---|---|---|---|
| **F-4** | Enforce the `path_glob` filter the graph tools advertise. | [권장] | **Closed (2026-07-28, #28).** `PathGlob` is applied client-side in `SearchFTS` and `FindSymbol` (`internal/system/ckgclient/real.go`, `filepath.Match` on `FilePath`). `CommitHash` is intentionally left unenforced — no tool exposes it and a single-commit index has no subset semantics. | `followups-from-dogfood-2026-05-19.md` F-4 |
| **composer-trace** | Report the exact ckg call count in the composer trace (was hardcoded 0). | [권장] | **Closed — PR #29, merged (verified 2026-08-20; this row read "in review" until then).** Stage 2 counts its actual `BM25Search`+`FindSymbol` (+glob pass) calls into `Stage2Output.CKGCalls`; the trace surfaces it. Stage 3 expansion runs after the trace and is out of scope. | `composer.go` TODO(trace) |
| **KR-deploy** | Knowledge-reserve doc-kind rescue: deploy + live re-capture. | [권장] | **Not a code task — remote operator action.** Task 1 code is in `main` (`budget/allocator.go` doc-kind reserve). Task 2 requires restarting the **remote** MCP on the new binary and re-running the live capture against the `pr-77-gstable` domain index. Verified 2026-07-28: a local proxy on `go-stablenet@0bf2f4d1b` serves the KR GasTip query correctly (relevant code bodies returned), but the exact re-capture needs the remote host + that specific domain index (different domain organization locally). | `fix-knowledge-reserve-doc-kind.md` §Task 2 |
| **M2-multi** | Multi-cycle total-cost judgment (the coding-agent bug-cycle-cost thesis). | [권장] | **Not a knowledge-system task — separate bench run.** The harness is complete but lives in `../coding-agent` (`plugin/skills/bench-orchestration`, `bench/compare.py`), targets go-stablenet, and needs chainbench plus many full agent-pipeline runs. Execute it from the coding-agent context; it is a large, explicit opt-in, not runnable from this repo. | `HANDOFF-cks-evaluation-remaining.md` §5.1(d) |
| **D-1** | Reconcile the documentation drift between this tree and the distribution. | [권장] | **Open (found 2026-08-20).** 71 common files differ after the module path is rewritten: **39 archive, 32 live**. The content is not branding — it is **stale path references** the consolidation left behind (`internal/parse/` → `internal/graph/parse/`, `docs/` → `docs/graph/`), and it points **in both directions**. Verified instances: this tree's `projects/stablenet/setup.yaml` cites `cmd/filelist-gen`, which does not exist (the real surface is `cks filelist`, `cmd/cks/filelistcli`), and `tools/viewer/README.md` says `make viewer` when the target is at `graph/Makefile:16` — both correct downstream; conversely `docs/graph/adr/README.md` downstream points at `docs/DOC-MAP.md`, which does not exist (it is `docs/graph/DOC-MAP.md`). **So this cannot be resolved by copying one tree over the other**; it is a per-file judgment. Both repos' CI is green, because `make docs-check` audits the binaries' `--help` surface and never opens a documentation path reference — closing D-1 without widening that check just restarts the drift. Suggested scope: the 32 live files; the 39 archive ones are historical records where a period-accurate path is arguably correct. | cross-tree diff, 2026-08-20 |
| **D-2** | Extract the promotion catalog out of `cmd/cks/domaincli/worksheet.go` into the pack. | [권장] | **Open (recorded 2026-08-14).** The catalog is pack-level domain knowledge sitting in engine code, so the pack-name boundary rule genuinely fires on it. `scripts/check-boundaries.sh` lists it as a named exemption with its reason rather than silencing the rule — the debt is visible, not hidden. Filing the name off would satisfy the linter and keep the defect. | [`../../docs/dev/2026-08-14-pack-knowledge-in-engine-code.md`](../../docs/dev/2026-08-14-pack-knowledge-in-engine-code.md) |
| **OPS-1** | Install the `.network` agent on the serving host. | [권장] | **Open (verified on the host, 2026-08-20).** `service install` writes three agents; the host has two — `com.stablenet.knowledge.go-stablenet` and `.watchdog` are loaded, `.network` is absent. It postdates the last install there. Consequence while absent: a network move leaves the server serving happily on its wildcard bind while every client holds a URL that no longer routes — the one failure neither `KeepAlive` nor the watchdog can see. Fix is re-running `service install`, which **restarts a live server**, so it is an operator action, not an automated one. | live host inspection |
| **OPS-2** | Route `ops.index` through the blue-green path instead of writing in place. | [권장] | **Open (design decision needed).** #47/#91 added an alignment gate keyed on what the deployment *has* rather than what a refresh rebuilt, so a mismatched pair is now reported at the moment it appears. That is detection, not recovery: an in-place write leaves no prior version to fall back to. Blocking question is what `mode=incremental` should mean once every write produces a new version. | [`../../docs/dev/2026-08-14-what-broke-the-dataset.md`](../../docs/dev/2026-08-14-what-broke-the-dataset.md) |
| **OPS-3** | Narrow the pmset policy, and adjudicate the property rather than the setting. | [권장] | **Open (recommended, not implemented).** Today four settings are required flat: `sleep 0`, `standby 0`, `womp 1`, `autorestart 1`. Two refinements: require `sleep 0` on **AC only** (a laptop on battery should be allowed to sleep), and report *"the host does not recover unattended after a power cut"* rather than *"autorestart is 0"* — the setting is one way to satisfy the property, and hardware that cannot expose it is not thereby non-compliant. | [`../../docs/dev/2026-08-13-upstream-reply-to-downstream.md`](../../docs/dev/2026-08-13-upstream-reply-to-downstream.md) §4.2 |

(F-1 from the dogfood doc is now closed — `real.go:202-243` consumes real ckg Score verbatim; F-2/F-3/F-6/F-7 unverified.)

## MCP-server hardening + review-driven improvements (2026-07-27–28) — done

The fused server's operate + integrate mission (serve knowledge over MCP to
local and **remote** agents, with easy setup) drove a hardening sequence, then a
pre-port review round that fixed several fail-open cases and operability gaps.
All of it is merged to `main`. The downstream port this section once named as
its last step is **done** (2026-08-20) and now runs in both directions — see
§"Availability + cross-repo lockstep" and the runbook
[`../../docs/downstream-sync.md`](../../docs/downstream-sync.md).

Hardening sequence (#14–#18):

| Item | What landed | PR |
|---|---|---|
| source_root single source of truth | the server derives `source_root` from the graph manifest when config leaves it empty, so the dataset is authoritative and the config↔dataset assertion is eliminated by design | #14 |
| LAN-reachable serving + registry | `netutil` advertised-host resolution; serve logs the reachable URL; `gen-config --lan`; `daemon up`/`down` over an `instances.yaml` registry with auto-assigned ports (one server per dataset) | #15 |
| blue-green reload | `/healthz` readiness probe; `daemon reload` starts a green instance on a temp port, gates on `/healthz`, then swaps — a bad rebuild never takes the running server down | #16 |
| namespace-aligned identity | the instance name defaults to the deployment namespace root (`KNOWLEDGE_MCP_NAMESPACE` / `-ldflags`) instead of the literal "cks" | #17 |
| async long ops + connection snippet | `Jobs.StartFunc` generalizes the async job pattern; `ops.reindex` MCP tool (build → gate → promote `current`, polled via `ops.setup_status`); `print-mcp-config` emits a ready-to-paste client config | #18 |

Review-driven improvements (#20–#26), from a three-lens code review of the
serving, setup, and config/identity layers plus end-to-end validation:

| Item | What landed | PR |
|---|---|---|
| route-default LAN IP | the advertised IP is resolved from the default route, not `net.InterfaceAddrs` enumeration order (correct on a multi-homed host) | #21 |
| registry engine binaries + readiness wait | `graph_binary`/`vector_binary` in the registry (so `ops.reindex`/`ops.index` work on a registry-launched instance); `daemon up --wait` polls `/healthz` until serviceable before returning a URL | #20 |
| fail-loud startup + safe reindex versions | daemon startup-crash detection by reap (not a zombie-blind signal-0 probe); reindex version validation (rejects `current`/`..`/`a/b`); `/healthz` reason withheld from non-loopback clients; empty instance name → namespace; duplicate fixed ports rejected | #22 |
| **security fail-closed** | prod requires a sanitize `rules_path` — an empty path (NOOP redaction) is rejected unless `logging.mode=dev`, and `gen-config` defaults an absolute in-repo baseline; alignment fails closed when a configured ckv index has no verifiable manifest (was a warning) | #23 |
| async job lifecycle | bounded per-job event + finished-job retention (memory flat under many builds); a registry base context whose `Shutdown` cancels in-flight builds on server exit; `Cancel(id)` | #24 |
| reindex robustness | stale-lock recovery (owner-PID/age based) so a crash no longer wedges reindex; a warning when the canonical-coverage gate is disabled (`min_canonical_ratio=0`) | #25 |
| daemon/CLI operability | `daemon status`/`list --ready` shows serviceability; `print-mcp-config --http-addr` overrides the emitted URL; auto-port probes the actual bind host | #26 |

> **Operator note (security, #23):** a production config now requires a
> non-empty `sanitize.rules_path`. `gen-config` fills it with an absolute path to
> the shipped baseline automatically; a hand-written prod config that leaves it
> empty is rejected at load. Use `logging.mode: dev` only for local, non-serving
> use where NOOP redaction is acceptable.

This supersedes the shell serving/reindex path in
[`ops-blue-green-reindex.md`](./ops-blue-green-reindex.md) (see its "Go 경로"
section); the legacy scripts still function. Per-script retirement status is in
the coverage audit
([`../../docs/dev/2026-07-27-legacy-ops-go-coverage-audit.md`](../../docs/dev/2026-07-27-legacy-ops-go-coverage-audit.md)).

## Availability + cross-repo lockstep (2026-08-12 – 08-20) — done

Two tracks ran together and are both merged. They started from one report — the
distribution could not answer `ckv` queries — which turned out to have two
independent causes, and separating them is most of what this section records.

**Cause 1: deterministic tool-contract defects.** Not intermittent, not
environmental. Four surfaced and all four are fixed:

| Item | What was wrong | PR |
|---|---|---|
| tool output schemas | all 22 tools shipped without one, so a client could not know the shape of `structuredContent`; the only guard was this repo's own decode helper, which is what let a handler return a bare slice — serialised as an array a conforming client rejects before the caller ever sees it | #95 (from downstream #50) |
| unresolved seed | `find_callers`, `find_callees` and `change_history` accepted any symbol and never said they had not understood it: `find_callers("Finalize")` answered about `crypto/bn256`'s `GT.Finalize` out of 11 definitions. Now `ErrSeedUnresolved`. | #95 (#52) |
| traversal depth | the documented default was not the served one | #95 (#53) |
| interface dispatch | the bridge was missing from `impact` and `subgraph` | #95 (#54) |
| `exclude_tests` on neighbours | the filter judged the **seed**, not the neighbour, so a callers walk kept test callers. Proven on a pre-#43 binary, which established it was not a #43 regression. | #90 |

**Cause 2: the embedding daemon does not survive host sleep.** `serviceable()`
requires the model to be reachable, so a daemon that is down makes `/healthz`
report 503 while the server itself is fine. Restarting the server there is
downtime that ends in another 503. Recovery therefore probes the dependency
first and only restarts the instance if restoring it did not help.

**What the availability track landed** (#91, wiring fixed in #92, documented in
#93): launchd agents generated from the config — server under `caffeinate`,
a watchdog, and a link watcher; SSH remote recovery pinned to a forced command
with a three-word vocabulary; restart rate limiting persisted across watchdog
ticks; `pmset` adjudication that prints the fix rather than running it. Full
description in [`ops-availability.md`](./ops-availability.md).

**Two half-shipped changes, both caught only by porting.** Worth recording as a
method note, because in both cases the upstream tree looked correct:

1. **The label prefix (#91 → #92).** `Deployment.LabelPrefix` shipped with a
   comment claiming config wiring that did not exist, so the field was inert and
   the default silently changed for every deployment. On the live host the
   watchdog then failed every tick against a label launchd did not hold —
   `runs = 3191, last exit code = 1`. A label is how launchd *finds* a job:
   changing it does not rename what is running, it loses the handle on it.
2. **The boundary rule (#91 → #92).** The pack-name rule matched the
   distribution's own module path, producing 11 import-only violations there
   while linting clean here. Fixed by excluding own-module imports.

Neither was visible from this repo alone. The downstream port is not a delivery
step after the work; it is part of the verification.

**Convergence measured 2026-08-20** (upstream `378c4c9`, downstream `63ce224`):
1,751 common files, **zero Go source differences** once the module path is
rewritten, 7 files differing only in import order, and 71 documentation files
still drifting — the last of which is item **D-1** above.

## Evidence pointers (re-verify before acting)

- M6 (done): `grep -rn CKGNodeID --include='*.go'` → prose comment only (hit.go:33);
  checklist in [`retire-ckg-node-id.md`](./retire-ckg-node-id.md).
- M1′ (done): go.mod has no replace; ckg/ckv pinned to `v0.1.0` (2026-07-19; was pseudo-versions).
- M7: `projects/stablenet/domain-knowledge/entries/*.yaml` — **44 files, 5 with `kind:`**
  (corrected 2026-08-20; the old pointer named a path that no longer exists and counts of 43/2).
- P0 / serving state: [`session-handoff-2026-07-10.md`](./archive/session-handoff-2026-07-10.md) §3.5,
  [`ops-blue-green-reindex.md`](./ops-blue-green-reindex.md).
- Quick resume checks (**rewritten 2026-08-20** — the previous block invoked
  `scripts/serve-cks-http.sh`, `scripts/reindex-dataset.sh` and
  `scripts/gen-cks-config.sh`, none of which exist any more; the Go CLI replaced
  all three):
  ```bash
  bin/cks mcp service status --config cks.yaml   # agents loaded? serving? power verdict
  # then cks.ops.health → serviceable / alignment.ok / builder_version
  bin/cks mcp gen-config --help                  # config generation (was gen-cks-config.sh)
  grep -rn "ckg_node_id\|CKGNodeID" --include='*.go' .   # M6 (→ prose comment only)
  ./scripts/check-boundaries.sh                  # engine isolation + pack-name rule
  ```
- Cross-tree convergence (D-1): compare against the distribution with the module
  path rewritten, or every Go file reads as different for that reason alone.
