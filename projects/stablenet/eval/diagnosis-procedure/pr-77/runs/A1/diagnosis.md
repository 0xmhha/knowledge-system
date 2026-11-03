# Diagnosis — DIAG-20261102_150122

Symptom: right after governance changes GasTip, unauthorized (non-validator/minter) txs are priced with the
OLD GasTip while empty blocks follow. EffectiveGasTip < minTip, so the txs are judged under-priced or dropped.
The pool's tip-threshold filter for remote txs shows the same wrong tip. Authorized accounts are not affected.
The symptom clears once a block that changes state arrives.

Retrieval: stablenet-knowledge status ok, serviceable. Index head 0bf2f4d1b equals repo HEAD (fresh, no contamination).
No probe was run; the conclusion is from static analysis.

## 1. Root cause

The value in question is **the header GasTip that `AnzeonTipEnv` returns for unauthorized senders**
(`env.currentBlock.GasTip()`).

The broken edge is the **store edge** in `AnzeonTipEnv.SetCurrentBlock` (`eth/gasprice/anzeon.go:54`). The
txpool's tip environment only replaces its cached head header when the **state root** changes:

```go
if env.currentBlock == nil || env.currentBlock.Root != header.Root {
    env.currentBlock = header
    ...
}
```

The guard key (`Root`) does not cover the field the consumer reads (`currentBlock.GasTip()`, which lives in the
header's WBFT extra). The same applies to `signer` and the fallback `BaseFee`. Here is how the bug plays out:

- Block N runs the governance tx that changes the GovValidator gasTip slot. Its root changes, but header N
  still carries the old tip, because `Prepare` gets the tip from the *parent* state
  (`engine.go:620-645`, `Prepare` at `engine.go:509-542`).
- Block N+1 is the first header that carries the **new** tip. When it is empty, `root(N+1) == root(N)`, so
  `SetCurrentBlock(N+1)` is skipped. The env keeps header N, which has the **old** tip.
- Every later empty block is skipped the same way. The first block whose root differs replaces the header,
  which is why the symptom clears.

This explains each distinguishing feature of the ticket:
- **Unauthorized accounts only:** only that branch reads `currentBlock.GasTip()` (`anzeon.go:111-116`).
  Authorized accounts use `tx.GasTipCap()`.
- **Only while empty blocks follow:** an empty block keeps the root unchanged.
- **Clears on a state-changing block:** a new root passes the guard.
- **Pool filter is wrong too:** `Pending()` calls `SetCurrentBlock(filter.Header)` through the same guard, then
  `EffectiveGasTipIntCmp(minTip, pool.anzeonTipEnv)` (`legacypool.go:585-596`).

The guard came in with #47 (`01f9c467b`, "optimize Anzeon gas tip cap handling") to avoid reloading the
StateDB when nothing changed. That reasoning holds for `currentState`. It does not hold for header-derived fields.

**Competing hypotheses ruled out**
- *H2: the worker or pool minimum tip (`worker.tip` / `pool.gasTip`) is stale.* `updateGasTipFromContract` runs
  on every sealed or imported block (`worker.go:876-883`, `blockchain.go:1853-1864`, async goroutine). It reads
  the tip from **state**, and state(N) already holds the new tip, so that value is not stale during the empty
  window. It also applies to all senders, which contradicts "authorized unaffected". The async goroutine can
  only cause sub-block races, not a window that lasts many blocks and ends on the next state change. Rejected.
- *H3: the receipt-side `NewInstantAnzeonTipEnv` (`receipt.go:346`) is wrong.* Miner, gasprice and feehistory
  build it per block from `block.Header().GasTip()` (`worker.go:1473`, `gasprice.go:254`, `feehistory.go:115`).
  It caches nothing and uses the correct header, so it is not a source.
- *H4: the per-tx cache `tx.anzeonTipCap` is the source.* It is filled at validation from the env
  (`validation.go:337-339`). It is a *downstream copy* of the stale env value, not the origin. It does widen the
  damage (see Affected sites).

## 2. Evidence

- `eth/gasprice/anzeon.go:50-63`: `SetCurrentBlock` uses the Root-equality guard. `currentBlock`, `signer` and
  `currentState` are only refreshed when the root differs. Added in `01f9c467b` (#47); seen in the git show diff.
- `eth/gasprice/anzeon.go:90-120`: `GetAnzeonTipCap` returns `env.currentBlock.GasTip()` for
  `!IsAuthorized(from)` (line 113-115), otherwise `tx.GasTipCap()`.
- `core/types/block.go:111-117`: `Header.GasTip()` reads `ExtractWBFTExtra(h).GasTip`, which is header data
  and not part of the state root.
- `consensus/wbft/engine/engine.go:620-645` (`getGasTip`): reads GovValidator gasTip from **parent** state.
  Called by `Prepare` (engine.go:486-549) and `verifyGasTip` (engine.go:1280-1295), per graph edges
  `getGasTip called_by Prepare/verifyGasTip`. As a result, header N+1 carries the tip set during block N.
- `core/txpool/legacypool/legacypool.go:346` (init), `:1536` (`reset`, on each new head) and `:585` (`Pending`
  filter) all funnel into the guarded `SetCurrentBlock`.
- `core/txpool/legacypool/legacypool.go:594-599`: remote-tx tip filter
  `tx.EffectiveGasTipIntCmp(minTipBig, pool.anzeonTipEnv) < 0` truncates the list using the stale tip.
- `core/types/transaction.go:388-403`: `EffectiveGasTip` = `min(cachedTipCap or env.GetAnzeonTipCap(tx),
  feeCap - baseFee)`, so a stale header tip turns directly into EffectiveGasTip < minTip.
- `core/txpool/validation.go:335-340`: caches `env.GetAnzeonTipCap(tx)` into `tx.anzeonTipCap` once
  ("only cache if not already set").
- `core/txpool/legacypool/list.go:481-503, 559-565`: the priced heap shares the same `anzeonTipEnv` and uses
  `EffectiveGasTipCmp`, so eviction ordering also sees the stale tip.
- Producer side is correct: `miner/worker.go:1200-1225, 361-389` and `core/blockchain.go:1853-1864` read the
  tip from state into `pool.SetGasTip` (`legacypool.go:470-496`).

## 3. Affected sites

| site | role | produces symptom | must fix | note |
|---|---|---|---|---|
| `eth/gasprice/anzeon.go:54` (`SetCurrentBlock` guard) | store (broken edge) | yes | **yes** | Guard keyed on Root but consumers read header fields (GasTip, Number/Time to signer, BaseFee fallback). |
| `eth/gasprice/anzeon.go:111-116` (`GetAnzeonTipCap`) | consumer | yes | maybe | Reads `currentBlock.GasTip()`. Correct once the env tracks the real head. |
| `core/txpool/legacypool/legacypool.go:1536` (`reset`) | caller | yes | no (if guard fixed) | Passes the correct newHead; the guard drops it. |
| `core/txpool/legacypool/legacypool.go:585-599` (`Pending` tip filter) | sibling-path consumer | yes | no (if guard fixed) | The "remote tx filtered by tip threshold" symptom. |
| `core/txpool/legacypool/legacypool.go:346` (init) | caller | n/a | no | Same guard; harmless at startup. |
| `core/txpool/validation.go:337-339` + `core/types/transaction.go:413-426` (`tx.anzeonTipCap`) | cache (copy) | yes, and persists | **review** | A tx validated during the stale window caches the old tip and keeps it after the window ends (no invalidator, intentionally "preserved during reinject"). Needs re-evaluation on GasTip change, or should not cache header tip for unauthorized senders. |
| `core/txpool/legacypool/list.go:500-503` (priced heap) | consumer | ordering only | no | Shares the env plus cached tip; heap order is stale during the window. |

## 4. Confidence

**High** for the broken edge. A single candidate explains every distinguishing feature (unauthorized-only,
empty-block window, clears on a state change, pool filter affected). The guard demonstrably compares a key that
does not cover the field being read.

What would raise it to certain: a throwaway in-process probe or test. Call `AnzeonTipEnv.SetCurrentBlock(hN)`
then `SetCurrentBlock(hN1)`, where `hN1.Root == hN.Root` but the WBFT extra GasTip differs, and observe
`GetAnzeonTipCap(unauthorizedTx)` returning hN's tip. Alternatively, run an e2e governance GasTip change
followed by idle (empty) blocks, and log `env.currentBlock.Number` versus the head number in `reset`.

Lower confidence remains on how much the per-tx `anzeonTipCap` cache adds after the window, because that
depends on reinject and promote paths.

## 5. Suggested direction

Make `AnzeonTipEnv.SetCurrentBlock` always adopt the new head header, and re-derive `signer` from it. Keep the
root comparison only to decide whether `currentState` must be reloaded, so the state-reload optimization from
#47 is kept but header-derived fields (GasTip, Number/Time, BaseFee fallback) can no longer go stale. In
addition, decide how txs whose `anzeonTipCap` was cached under an old header tip get refreshed when the tip
changes. Options are to invalidate or recompute in `reset` or `SetGasTip`, or to skip caching the header-tip
branch for unauthorized senders. Cover it with a unit test of `SetCurrentBlock` for equal roots with different
GasTip, plus a pool-level test for the `Pending` minTip filter across an empty block.

### Side findings (not in scope)
- `anzeon.go:111-115`: the env uses the **head's** header tip, but the next block's header will carry
  `GasTip(state(head))`. Right after the governance block N itself (before any empty block), pool pricing still
  uses N's old header tip for one block, even though block N+1 will be built with the new tip. Confidence:
  medium. If the intended semantics are "price for the next block", the env should read the tip from
  `currentState` rather than the header.
