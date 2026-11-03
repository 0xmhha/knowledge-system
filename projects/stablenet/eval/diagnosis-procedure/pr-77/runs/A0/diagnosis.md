# Diagnosis — DIAG-20261102_150123

Retrieval backend: stablenet-knowledge OK / serviceable (ckg + ckv), indexed_head 0bf2f4d1b == HEAD (fresh, no contamination).
Mode: diagnose (read-only). No probe run (the cause is a deterministic guard; static evidence fully explains every distinguishing feature).

## 1. Root cause

The value is **the header GasTip the txpool's `AnzeonTipEnv` uses as the tip cap for unauthorized senders** (`env.currentBlock.GasTip()`).

`AnzeonTipEnv.SetCurrentBlock` (eth/gasprice/anzeon.go:54) only replaces its cached `currentBlock` when **the state root changes** (`env.currentBlock.Root != header.Root`). But the GasTip lives in the header's WBFT extra, not in the state root, and it trails governance state by one block:

- Block N contains the governance tx that changes GasTip in GovValidator. Its root changes. Its header GasTip is still the **old** value, because `Engine.getGasTip` reads it from the **parent** state (consensus/wbft/engine/engine.go:620-645, written into the extra at engine.go ~509/538/542).
- Block N+1 is the first header that carries the **new** GasTip. If N+1 is empty, `N+1.Root == N.Root`, so `SetCurrentBlock(N+1)` returns early and the env keeps header N with the **old** GasTip.
- Every later empty block hits the same early return. The env updates only when a block with a state change (a different root) arrives. That is why the symptom clears on its own.

So the broken edge is **store: the cached copy is not refreshed when its source changes**. The cache key (state root) doesn't cover the cached field it serves (header GasTip). Consumers then compute `EffectiveGasTip = min(oldHeaderTip, feeCap-baseFee)` for unauthorized senders. In `LegacyPool.Pending`, the miner's `MinTip` is `worker.tip`, which is read fresh from GovValidator state (`miner/worker.go:1201-1225`) and is already the new value. Against that, `EffectiveGasTip < MinTip`, and the tx is cut off (legacypool.go:594-598). This is the "remote tip-threshold filter uses the wrong tip" path in the ticket.

How the distinguishing features fit:
- **Only unauthorized accounts:** `GetAnzeonTipCap` returns the header tip only when `!IsAuthorized(from)`. Authorized senders use their own `GasTipCap` (anzeon.go:111-119).
- **Only during empty blocks after the gov change:** an empty block keeps the root identical, so the guard fires.
- **Clears on a state-changing block:** the root differs, so the guard passes and the env refreshes to the current header.

Competing hypotheses ruled out:
- **Stale `worker.tip` (miner/worker.go:374-389, `setGasTipUnsafe` early return).** The worker reads GasTip directly from post-block state, which changed at block N, so `worker.tip` is already new. A stale worker tip would also make the `MinTip` filter more lenient, not reject txs, and it would not single out unauthorized senders.
- **Wrong producer (`Engine.getGasTip` reads parent state).** This is the intended one-block lag and is consistent across nodes. It creates the window but does not cause the persistence across empty blocks. Without the root-only guard, block N+1 would already deliver the new tip.
- **Per-tx cached `anzeonTipCap` (core/txpool/validation.go:337-340) as the primary cause.** This is a write-once cache, so on its own it would not "clear" when a state-changing block arrives. It is a second-hop copy of the same stale value (see Affected sites), not the source.

## 2. Evidence

- `eth/gasprice/anzeon.go:50-63`: `SetCurrentBlock` has the guard `if env.currentBlock == nil || env.currentBlock.Root != header.Root { env.currentBlock = header; ... }`. A same-root header with a different extra/GasTip is dropped.
- `eth/gasprice/anzeon.go:90-120`: `GetAnzeonTipCap` returns `env.currentBlock.GasTip()` for unauthorized senders and `tx.GasTipCap()` for authorized ones.
- `core/types/block.go:111-117`: `Header.GasTip()` comes from the WBFT extra (`ExtractWBFTExtra(h).GasTip`), so it is header data, not state.
- `consensus/wbft/engine/engine.go:620-645`: `getGasTip` reads `GetGasTip(govValidator, StateAt(parent.Root))`. Its callers (engine.go:486-549 Prepare path, engine.go:1280-1295) write it into the header extra (`WriteGasTip(gasTip)`). This is why the header tip lags the gov change by one block.
- `core/txpool/legacypool/legacypool.go:585`: `Pending` calls `pool.anzeonTipEnv.SetCurrentBlock(filter.Header)`, then at lines 594-598 runs `tx.EffectiveGasTipIntCmp(minTipBig, pool.anzeonTipEnv) < 0` to truncate remote (non-local) txs.
- `miner/worker.go:1229-1246` (`fillTransactions`): `MinTip = w.tip` (fresh, from state) and `filter.Header = w.chain.CurrentBlock()` (the new header, which the env then ignores).
- `core/types/transaction.go:388-403`: `EffectiveGasTip` uses the cached per-tx `GetAnzeonTipCap()` if set, otherwise `anzeonTipEnv.GetAnzeonTipCap(tx)`, then `min(tip, feeCap-baseFee)`.
- `core/txpool/legacypool/legacypool.go:346` (init) and `:1536` (`reset`): other `SetCurrentBlock` calls, all going through the same root guard.
- `core/txpool/validation.go:335-340`: on admission, `tx.SetAnzeonTipCap(opts.AnzeonTipEnv.GetAnzeonTipCap(tx))` is set only if not already cached (`legacypool.go:695` passes the env).
- Call graph (stablenet-knowledge graph_neighbors): `updateGasTipFromContract` is called by worker.go:819-894 and :260-332, and calls `getGasTipFromContract` and `setGasTip`. `Engine.getGasTip` is called by engine.go:486-549 and :1280-1295. `TxPool.SetGasTip` calls `subpool.SetGasTip`, which reaches `LegacyPool.SetGasTip` and then `lookup.RemotesBelowTip`.
- History: `eth/gasprice/anzeon.go` was introduced in d61c9140d (#29), and the guard/caching was reworked in 01f9c467b ("optimize Anzeon gas tip cap handling", #47). That optimization is the likely origin of the root-only guard.

## 3. Affected sites

| site | role | produces symptom | must fix | note |
|---|---|---|---|---|
| eth/gasprice/anzeon.go:54 (`SetCurrentBlock` guard) | cache (broken edge) | yes | yes | The root-only equality skips headers whose GasTip/extra changed under an identical root. It must also refresh `currentBlock` when the header changes (hash/number or GasTip differs). It can keep reusing `currentState` when the root is equal, to keep the #47 optimization. |
| eth/gasprice/anzeon.go:111-116 (`GetAnzeonTipCap`) | consumer | yes (via stale `currentBlock`) | no (fixed by the guard) | Reads `currentBlock.GasTip()`. |
| core/txpool/legacypool/legacypool.go:585-598 (`Pending` MinTip filter) | consumer / sibling-path | yes | no (fixed by the guard) | The "remote tip-threshold filter" in the ticket. |
| core/txpool/legacypool/legacypool.go:1536 (`reset`) and :346 (init) | producer calls into the cache | yes | no | These pass the correct new head, but the guard drops it. |
| core/txpool/validation.go:335-340 (per-tx `SetAnzeonTipCap` cache) | cache (second copy) | yes, for txs admitted during the stale window | **recommended** | Txs validated in the window cache the old header tip permanently, because it is write-once with no invalidation. They stay under-priced in `Pending` and in priced-heap ordering (list.go:500-502) even after the env recovers. The fix should invalidate or recompute these on GasTip change, for example clear the cached tip cap for pool txs in `reset`/`SetGasTip`, or recompute it when the header GasTip changes. |
| core/txpool/legacypool/list.go:500-502 (`priceHeap` with anzeonTipEnv) | consumer | ordering only | no | Eviction/priority ordering uses the same stale tip during the window. |

Out of scope: `core/types/receipt.go:346` and `eth/gasprice/{gasprice,feehistory}.go` build an instant env from the specific header, so they are not affected.

## 4. Confidence

**High.** The guard is deterministic, and the one-block lag of the header GasTip (parent state) together with empty blocks keeping the same root directly yields every distinguishing feature: unauthorized only, empty-block window, self-clears on a state change, and the Pending MinTip path affected. Two things would make this certain:
(a) A throwaway in-process test: build `AnzeonTipEnv`, call `SetCurrentBlock(hdrN{Root:R, GasTip:old})` then `SetCurrentBlock(hdrN1{Root:R, GasTip:new})`, and observe `GetAnzeonTipCap(unauthorizedTx) == old`.
(b) An e2e repro (chainbench): change GasTip via governance, keep the chain idle (empty blocks), and check that an unauthorized tx priced at the new tip stays out of blocks until any state-changing tx lands.

## 5. Suggested direction

Make the env's header cache key cover what it serves. In `AnzeonTipEnv.SetCurrentBlock`, always adopt the new header (or adopt it whenever the hash/number/GasTip differs) and recompute the signer. Re-resolve `currentState` only when the root actually changed, which keeps the #47 state-lookup saving. Then address the second copy. Tx-level `anzeonTipCap` values cached during the stale window (and in general before any GasTip change) must be invalidated or recomputed when the effective header GasTip changes, for example on pool `reset`/`SetGasTip`. Otherwise txs admitted in the window remain under-priced after the env heals. Add a regression test at the env level (same root, different GasTip) and a pool-level `Pending(MinTip=newTip)` test for an unauthorized sender across an empty-block head change.

## Side findings (not this ticket's fix surface)

- `core/txpool/legacypool/legacypool.go:2058-2066` `RemotesBelowTip`, called from `SetGasTip` at :489. When governance raises the tip, it drops remotes by raw `tx.GasTipCap()`, not by the Anzeon effective tip. For unauthorized senders the protocol overrides the tip with the header GasTip, so their raw `GasTipCap` doesn't decide their priority. The drop can therefore remove unauthorized txs that would be valid under the new header tip, or keep ones whose fee cap can no longer cover it. The predicted symptom is different: unauthorized txs vanish from the pool right after a GasTip increase. Confidence: medium. It survived the self-check because no Anzeon-aware branch exists in `RemotesBelowTip`. Suggest a separate ticket.
