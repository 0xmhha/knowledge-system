# Diagnosis — snap-synced node returns null `effectiveGasPrice` on Anzeon chains

Retrieval backend: stablenet-knowledge ok / serviceable (ckg + ckv). The index head `aa28927fb` matches the repo HEAD, so the index is fresh and serves the right commit.

## 1. Root cause

The value in question is `Receipt.EffectiveGasPrice`. On an Anzeon chain the node fills it in only once, when it executes the transaction, and it is kept only in the local **storage** encoding of the receipt. The value cannot be rebuilt later:

- **Produce.** Only `applyTransaction` writes it (`core/state_processor.go:157-159`), and only under `config.AnzeonEnabled()`. It copies `msg.GasPrice`. That value depends on state: `TransactionToMessage` uses the tx's own tip for an authorized sender and the header gas tip for everyone else, because it checks `statedb.IsAuthorized(from)` (`core/state_transition.go:165-171`, `190-192`).
- **Store.** `storedReceiptRLP` carries it as an optional field (`core/types/receipt.go:108`, encode `285-290`, decode `307`).
- **Re-derive on read.** `Receipts.DeriveFields` skips it on purpose when Anzeon is enabled (`core/types/receipt.go:395-397`). It trusts whatever was stored.

**Broken edge: the snap-sync store edge.** A snap-syncing node never executes historical blocks. It gets their receipts from peers in the **consensus/wire** encoding, `receiptRLP` (`receipt.go:96-101`: status, cumulativeGas, bloom, logs). That encoding has no `EffectiveGasPrice`. Decoding goes through `setFromRLP` (`receipt.go:227-230`), so the field stays nil. The downloader then hands those receipts to `BlockChain.InsertReceiptChain` (`core/blockchain.go:1089`), which writes them as-is (`1264` live, `1293` ancient). So the stored receipt has no EGP. On read, `rawdb.ReadReceipts` → `DeriveFields` (`core/rawdb/accessors_chain.go:656`) skips the field because Anzeon is on, and `marshalReceipt` (`internal/ethapi/api.go:1871`) emits `null`.

Why the symptom shows up only on Anzeon chains: on a non-Anzeon chain, `DeriveFields` recomputes the field from the tx and baseFee, so snap-synced receipts are filled in. The probe below shows this.

Why full sync differs: a full-sync node runs `applyTransaction`, so the state-aware value is produced and stored.

Why authorized and normal accounts differ: that is the `IsAuthorized` branch in `TransactionToMessage`. Because the branch needs historical state, `DeriveFields` cannot redo it. A snap-synced node does not have that historical state.

**Competing hypothesis ruled out — H2: the storage codec (`ReceiptForStorage` encode/decode) drops the field.** The runtime probe refutes it. A receipt with EGP=1500 survives the storage round-trip plus `DeriveFields(Anzeon)` with EGP=1500 still set. The field is lost only when the receipt passes through the consensus/wire codec, which is the snap-sync path.

Also considered and rejected: "`DeriveFields` computes the wrong value". Under Anzeon it computes nothing at all. Getting `null`, rather than a wrong number, matches "not computed" and not "wrong source".

## 2. Evidence

- `core/types/receipt.go:384-436` `Receipts.DeriveFields`, with the guard at `:395-397`: `if !config.AnzeonEnabled() { rs[i].EffectiveGasPrice = txs[i].inner.effectiveGasPrice(new(big.Int), baseFee) }`.
- `core/types/receipt.go:96-101` (`receiptRLP`, no EGP), `:104-109` (`storedReceiptRLP` with an optional EGP), `:162-230` (consensus decode: `setFromRLP` never sets EGP), `:273-311` (storage codec keeps EGP).
- `core/state_processor.go:157-159`: the only producer under Anzeon (`receipt.EffectiveGasPrice = msg.GasPrice`).
- `core/state_transition.go:159-200` `TransactionToMessage`: the authorized-vs-normal tip choice at `:165-171` and the effective price at `:190-192`. This is the function behind the ticket's third observation (the two account classes are computed differently).
- `core/state/statedb.go:321` `IsAuthorized` → `state_object.go:598` (`types.IsAuthorized(s.data.Extra)`). The authorized flag lives in account state, so it cannot be derived from block or receipt data.
- Snap/fast-sync receipt path: `eth/protocols/eth/handlers.go:473` `handleReceipts` (decodes `ReceiptsPacket` through the consensus codec) → `eth/downloader/downloader.go:1731,1743` → `core/blockchain.go:1089` `InsertReceiptChain` → `rawdb.WriteReceipts` at `:1264` and `writeAncient` at `:1293`.
- Read path: `core/rawdb/accessors_chain.go:630-661` `ReadReceipts` → `DeriveFields` at `:656`. RPC output is `internal/ethapi/api.go:1871`.
- Graph edges from `get_for_task`: `DeriveFields` is called by `core/rawdb/accessors_chain.go:630-661`, `core/blockchain.go:2102-2123` and `core/chain_makers.go:308-424`, and it calls `MakeSigner`/`Sender`.
- Design history, commit `d61c9140d` (#29): "DeriveFields() cannot access stateDB, making it impossible to check if the sender is an authorized account. For authorized accounts, calculate effectiveGasPrice in applyTransaction(). For normal accounts, keep the existing DeriveFields() logic unchanged." That commit also added and later removed a `StateReader` passed to `DeriveFields`. In the current code the guard skips derivation for **all** accounts, not only authorized ones.
- **Runtime probe** (a throwaway `core/types/zz_probe_test.go`; deleted afterwards and `git status` is clean). Base fee 1000, tx tip 500, stored EGP 1500:
  - full sync, storage round-trip → `DeriveFields(Anzeon)`: EGP=1500
  - after wire (consensus) decode: EGP=`<nil>`
  - snap-sync path, wire → storage → `DeriveFields(Anzeon)`: EGP=`<nil>` (the symptom)
  - same snap path with `DeriveFields(non-Anzeon config)`: EGP=1500

## 3. Affected sites

| site | role | produces symptom | must fix |
|---|---|---|---|
| `core/types/receipt.go:395-397` (`DeriveFields` Anzeon guard) | consumer/derive | yes: leaves EGP nil when nothing was stored | yes (primary) |
| `core/blockchain.go:1089-1300` `InsertReceiptChain` (writes at `:1264`, `:1293`) | store (snap path) | yes: persists receipts without EGP | yes, or the network path below must supply it |
| `eth/protocols/eth/handlers.go:473` / `core/types/receipt.go:96-101,227-230` (wire receipt codec) | transport | yes: the wire format cannot carry EGP | candidate (only if the fix moves EGP over the network) |
| `core/state_processor.go:157-159` | producer (full sync) | no (correct) | keep consistent: do not let a derived value overwrite it |
| `core/state_transition.go:159-200` `TransactionToMessage` | implements "authorized vs normal compute differently" | no (correct) | reference: any derive or recompute logic must match it exactly |
| `core/rawdb/accessors_chain.go:630-661` `ReadReceipts` | consumer | passes the nil through | possibly (if a StateReader or header gas tip has to be threaded in) |
| `core/blockchain.go:2102-2123`, `core/chain_makers.go:308-424` | other `DeriveFields` callers | no | regression check: freshly executed receipts must keep the applyTransaction value |
| `internal/ethapi/api.go:1855-1871` `marshalReceipt` | RPC output | emits null | no (output only) |

## 4. Confidence

**High.** Every edge is cited, and a runtime probe at unit level reproduces each step: the wire decode drops the field, the storage codec keeps it, and `DeriveFields` fills it only on non-Anzeon chains. Confidence would go higher with an end-to-end check: snap-sync a chainbench EN on an Anzeon chain with both authorized and normal txs, and compare `eth_getTransactionReceipt.effectiveGasPrice` for the same tx hashes against a full-sync node.

## 5. Suggested direction

When no EGP was stored, `DeriveFields` should fill it in on Anzeon chains and never overwrite a stored value (guard on `rs[i].EffectiveGasPrice == nil`). The hard part is the authorized/normal split, which needs `IsAuthorized` at the parent state.

Recommended approach:
- For normal senders, derive `min(headerGasTip + baseFee, gasFeeCap)`. This requires passing the header gas tip (or the header) into `DeriveFields` / `ReadReceipts`.
- For the authorized case, there are two options; the Planner should choose:
  - use a state reader when historical state exists, or
  - have snap sync persist an EGP computed from data it can trust.
- Snap sync has no historical state. A complete fix therefore has to either accept a documented limitation for authorized senders, or carry the authorized status (or EGP) through a non-consensus channel. Either way, the result must match `TransactionToMessage`.

### Rejected directions

- **Drop the Anzeon guard and always use `tx.inner.effectiveGasPrice` (tx tip + baseFee).** For normal accounts this returns the wrong value, because Anzeon uses the header gas tip for them. It would also overwrite the state-aware value stored at execution. That contradicts the #29 intent: "For authorized accounts, calculate effectiveGasPrice in applyTransaction()".
- **Add EGP to the consensus `receiptRLP`.** That encoding feeds the receipt trie (`EncodeIndex`/`DeriveSha`), so the change would alter receipt roots and break consensus. Any network carriage has to be outside the consensus encoding.

## Side findings (not in scope, not fixed)

- `graphql/graphql.go:289-313` `Transaction.EffectiveGasPrice`, medium confidence. Under Anzeon it returns `headerGasTip + baseFee` for every sender. It ignores authorized senders and skips the `min(…, gasFeeCap)` cap. Predicted symptom: GraphQL EGP disagrees with the full-sync receipt for authorized-account txs.
- `internal/ethapi/api.go:1525-1538` `effectiveGasPrice` (used for RPC tx `gasPrice`), medium confidence. It applies the header gas tip regardless of authorization. Predicted symptom: `eth_getTransactionByHash.gasPrice` is wrong for authorized senders.
