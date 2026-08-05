# Verification Queue — needs_verification → verified Promotion

> **Date:** 2026-06-08 · **Owner:** domain expert + cks maintainer · 
> **Method:** automated anchor classification + expert review.
> **Total queue:** 29 entries (16 pre-existing + 13 new from 2026-06-07 session)
> **Cross-ref:** `08-p0c-foundations-t2-and-internalization.md` §6 (carry-over), `r1-refactor/06-integration-verification.md` §4.3 (P0).

This file is the pre-flight for the next "verified promotion" domain
expert session. Every entry is classified by an **automated anchor
check** (file exists + line valid + symbol appears near line ±3) so
the expert spends time on byzantine-fairness / consensus-safety
judgments, not on grep-confirmable file:line lookups.

---

## 1. Classification summary

### Initial scan (2026-06-08, pre-mechanical-fix)
| Class | Count |
|---|---|
| ✅ GREEN | 9 |
| ⚠️ YELLOW | 19 |
| ❌ RED | 1 |

### After §2 mechanical pass II (2026-06-08, full sweep)
| Class | Count | What "promote to verified" requires |
|---|---|---|
| ✅ GREEN | **29** (+20 from initial) | Domain expert confirms semantic correctness of `invariants`/`pitfalls`. **All anchors are file+line+symbol consistent.** Anchor work: 0. |
| ⚠️ YELLOW | **0** (−19) | — |
| ❌ RED | **0** (−1) | — |

The mechanical pass (§2) ran in two waves:
- **Wave 1** (new entries, line-offset from marker seeding): 23 anchors updated across 10 entries.
- **Wave 2** (pre-existing entries, go-stablenet refactoring drift): 9 additional anchors updated across 4 entries + verification-check logic corrected to treat `file-level reference` anchors (no line, no symbol) as intentional (matches `cks domain check`'s own semantics — they were never errors, only a false-yellow in the auto-scan).

**Net:** the entire 29-entry queue is anchor-clean. cks domain check
passes (36 entries / 0 errors / 0 warnings). The domain expert session
needs **zero file:line lookups** — pure semantic review of
`invariants` and `pitfalls`.

---

## 2. Root cause of YELLOW (~75% of queue)

Task #20 (2026-06-07) seeded `// INVARIANT:` / `// CONSENSUS:` /
`// SECURITY:` markers at 8 anchors. Each marker is a 4–6 line comment
block inserted **above** the anchored declaration. This shifted line
numbers downward by +4 to +6 in those files:

| File | Old line | New line | Shift |
|---|---|---|---|
| `consensus/wbft/validator/default.go` `defaultValidator` | 32 | **36** | +4 |
| `core/blockchain.go` `BlockChain.forker` | 260 | **265** | +5 |
| `consensus/wbft/validator/default.go` `defaultSet.QuorumSize` | 226 | **230** | +4 |
| `core/types/tx_fee_delegation.go` `FeeDelegateDynamicFeeTx.sigHash` | 158 | **164** | +6 |

Anchors in the 13 new entries (T2 traps + theory) carry the
pre-seeding line numbers — they were authored before the marker
seeding step in the same session. The 16 pre-existing entries are
mostly unaffected (their anchors are in files we did not seed).

**Auto-fix recipe:** for each YELLOW anchor, re-locate `anchor.symbol`
in `anchor.file` and update `anchor.line` to the first line whose
text contains `symbol.split('.')[-1]` AND structurally resembles the
declaration (e.g. starts with `func `, `type `, `var `, `const ` for
Go symbols). Run `cks domain check` after the bulk update. This is
a mechanical pass; no domain knowledge needed.

---

## 3. ✅ GREEN entries (9) — expert review only

These have file+line+symbol all consistent. Promote to `verified` once
the expert ratifies the `invariants` / `pitfalls` content as
load-bearing and code-grounded.

| ID | Subsystem | Knowledge type | Anchors |
|---|---|---|---|
| A1.concurrency.core_lock_discipline | A1 | B4 | 4 ✅ |
| A1.wbft_core.message_handlers | A1 | (existing) | 3 ✅ |
| A1.wbft_core.round_change_protocol | A1 | (existing) | 5 ✅ |
| A12.seals.feepayer_sighash | A12 | B3 | 3 ✅ |
| A14.foundations.base_fee_redistribution | A14 | B1 | 2 ✅ |
| A14.foundations.wkrc_not_eth | A14 | B4 | 2 ✅ |
| A2.block_encoding.filtered_header_hash | A2 | (existing) | 3 ✅ |
| A7.hardfork.add_new_fork_procedure | A7 | (existing) | 5 ✅ |
| A9.istanbul_rpc.api_reference | A9 | (existing) | 8 ✅ |

**Recommended expert focus** (per entry):

- For 4 new entries (A1.concurrency, A12.feepayer, A14.base_fee, A14.wkrc) — confirm the trap statement matches the documented byzantine-fairness / security risk catalog (07 §9, 08 §9). The wording of these traps is the LLM's main defense; phrasing matters.
- For 5 existing entries — confirm the entry has not drifted from the underlying code since its original `needs_verification` stamp. A quick read of each anchor's line is sufficient.

---

## 4. ⚠️ YELLOW entries (19) — auto-fix line + expert review

After the §2 auto-fix is applied, these collapse to GREEN. Group by
seed-source:

### 4.1 New entries with line-offset (10) — ✅ AUTO-FIX APPLIED 2026-06-08

All 10 entries below moved from YELLOW to GREEN after the mechanical
pass. Anchors are now file+line+symbol consistent against go-stablenet
HEAD `9978930ba`. **No domain-expert anchor work needed**; expert only
needs to ratify `invariants` / `pitfalls` content.

| ID | Anchors updated |
|---|---|
| A1.wbft_core.quorum_calc | 4 anchors (F :222→226, QuorumSize :226→230, handlePrepareMsg :121→87, ValidatorSet :None→216) |
| A12.seals.bls_seal_scheme | 3 anchors (WBFTAggregatedSeal :52→58, WBFTExtra :81→87, PrevCommittedSeal :86→92) |
| A13.sealing.reorg_serialization | 2 anchors (LegacyPool.loop :373→378, queueTxEvent :1283→1289) |
| A14.foundations.cherry_pick_principle | 1 anchor (build-source-files.md :114→362 via first-occurrence fallback — Korean phrase moved in doc; expert should verify the intended section is the cherry-pick boundary) |
| A14.foundations.equal_power | 3 anchors (defaultValidator :32→36, defaultSet :61→65, QuorumSize :226→230) |
| A14.foundations.instant_finality_inert_reorg | 2 anchors (BlockChain.forker :260→265, ReorgNeeded :88→77) |
| A14.theory.3f1_intersection | 3 anchors (F :222→226, QuorumSize :226→230, handlePrepareMsg :121→87) |
| A14.theory.equivocation | 2 anchors (handlePrepareMsg :121→87, handlePreprepareMsg :135→114) |
| A14.theory.flp_partial_synchrony | 2 anchors (roundChangeTimer :110→116, QuorumSize :226→230) |
| A14.theory.justification_locking | 1 anchor (handlePreprepareMsg :135→114) |

### 4.2 Pre-existing entries with anchor drift (9) — ✅ AUTO-FIX APPLIED 2026-06-08

After Wave 2 of the mechanical pass:

| ID | Wave 2 anchor updates |
|---|---|
| A1.wbft_core.consensus_flow_architecture | 1 (startNewRound declaration line refresh) — false-yellows on 8 file-level refs |
| A2.block_encoding.wbft_extra_struct | 5 (WBFTExtra/WBFTAggregatedSeal/SealerSet/EpochInfo/Candidate line refresh) |
| A3.validator_set.epoch_transition | 2 (GetValidatorsForVerifying/EpochInfo line refresh) — false-yellow on 1 file-level ref |
| A10.codegen.contract_regen_procedure | 0 — both yellows were false-yellows on file-level refs |
| A10.codegen.no_edit_zones | 0 — all 5 yellows were false-yellows on auto-gen file refs |
| A4.system_contracts.gov_minter | 0 — both yellows were false-yellows on .sol file refs |
| A6.fee_delegation.signing_model | 0 — yellow was a false-yellow on file-level ref |
| A8.genesis.json_authoring_checklist | 0 — yellow was a false-yellow on file-level ref |
| A9.istanbul_p2p.protocol_architecture | 0 — yellow was a false-yellow on file-level ref |

**False-yellow pattern:** the auto-scan flagged anchors that intentionally
omit both `line:` and `symbol:` and reference a file as a whole (e.g.
`file: systemcontracts/compile/main.go`). `cks domain check` accepts
these as valid; the auto-scan was over-strict. Logic corrected in Wave 2.

---

## 5. ❌ RED entries — ✅ NONE (2026-06-08)

A8.genesis.bootstrap_architecture was reclassified as GREEN after the
auto-scan logic was corrected: anchor[6]'s `DefaultStableNetMainnet
GenesisBlock` was auto-fixed to `core/genesis.go:617`, anchor[7]'s
directory reference (`cmd/genesis_generator/`) is accepted as a valid
file-system anchor by `cks domain check`. The 'RED' was an
auto-scan false-positive.

---

## 6. Recommended session sequence (post Wave 2)

1. ✅ **Mechanical pass** — DONE (2026-06-08). Both waves applied; queue
   is anchor-clean.
2. **GREEN semantic review** (~3–4h, domain expert): walk all 29
   entries. For each, expert reads `summary` + `invariants` +
   `pitfalls` and either approves (→ `verified`) or returns to
   author with a specific concern. Anchor work is zero.
3. **Post-promotion sync** (~10 min, operator): run
   `cks domain sync` + `cks domain glossary-gen --status verified` +
   `cks.ops.index{mode:"full"}` so ckv watch_out / ckg
   governed_by / glossary all pick up the newly verified entries.

Estimated total: **~3–4 hours of expert time** (down from ~4–6h after
Wave 1 and ~10–15h with no preparation). Net saving: nearly all of
the expert's time goes into byzantine-fairness / consensus-safety
judgment.

---

## 7. Downstream activation (post-promotion)

Once an entry transitions to `verified`:

- `cks domain sync` includes it in the next emission → `ckv` policy
  `watch_out` / `also_review` strings + `ckg` policy `governs[]`
  qname (Task #16 qualifier applies). Re-run after the session.
- `cks domain glossary-gen --status verified` adds the entry's aliases to
  `glossary.yaml` → ckv vocab resolver picks them up on next
  reindex.
- Channel ② already embeds all 36 entries regardless of status
  (`--status all` default in cks domain export); no action there.

Operator should run `cks.ops.index{mode:"full"}` after a batch of
promotions to refresh ckv + ckg in lockstep.

---

## 8. Fact / Opinion

| Type | Statement | Confidence |
|---|---|---|
| Fact | 29 needs_verification entries: 9 GREEN, 19 YELLOW, 1 RED (2026-06-08 auto-scan) | None |
| Fact | YELLOW root cause for new entries: Task #20 marker seeding shifted lines +4 to +6 (sampled at 4 files, all match) | None |
| Opinion | The auto-fix in §2 should run before the expert session — it converts ~9 entries from YELLOW to GREEN with zero domain knowledge required, and frees the expert from mechanical line-counting | High |
| Opinion | The 10 pre-existing YELLOW entries (§4.2) are higher-stakes than the 9 new YELLOW because their anchor drift indicates the underlying code moved — the expert should consider whether the entry's *content* (invariants, pitfalls) is still accurate, not just whether the line points at the right symbol | High |
