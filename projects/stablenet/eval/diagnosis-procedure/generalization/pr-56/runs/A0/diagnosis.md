# Diagnosis — DIAG-20261102_161103

Symptom: nodes initialized from the "same" genesis get a different genesis hash (state root) when a system-contract params address list has spaces after commas or a trailing comma. No error is shown. Removing the whitespace or trailing comma restores the expected hash.

Retrieval: stablenet-knowledge is healthy and serviceable. The graph and vector indexes are both at `6e41d9663`, which matches repo HEAD, so the index is not contaminated.

## 1. Root cause

**The value at fault** is the `common.Address` (and BLS key bytes) parsed from each comma-separated entry of a `systemContracts.<contract>.params` list. That value is written into genesis `Alloc[...].Storage`, and the storage determines the state root.

**Lifecycle:**
- Produce: params string → `strings.Split(s, ",")` → `common.HexToAddress(entry)` / `common.FromHex(entry)` → `params.StateParam`
- Store: `InjectContracts` → `genesis.Alloc[addr].Storage[key] = value`
- Consume: `hashAlloc` / `Commit` → state root → genesis block hash

**The broken edge is the produce edge (parse).** The split entries are never normalized (no `TrimSpace`, empty entries not dropped). They are also never validated: in the affected sites nothing checks for a zero address or empty bytes. `common.FromHex` (`common/bytes.go:29-37`) does not trim whitespace:
- `" 0xB…"` has 43 chars. Because the length is odd, it is turned into `"0 0xB…"`.
- `hex.DecodeString` stops at the first invalid pair, so it returns 0 bytes. `Hex2Bytes` (`common/bytes.go:80`) discards that error.
- The result is the **zero address**. An empty entry `""` from a trailing comma also gives the zero address.

So a different but well-formed set of storage slots goes into the alloc. Concretely, `mapping[0x0]=true` is written, and with the space form the real address is dropped. This gives a different state root and no error.

**Primary site: NativeCoinAdapter `minters`.** In `initializeCoinAdapter` (`systemcontracts/coin_adapter.go:51` split, `:69` `HexToAddress(minter)`), there is no trim and no zero or empty check. This is the only list where *both* forms in the ticket diverge silently:
- **Space form:** `minters[0x0]=true` is written instead of the real minter.
- **Trailing comma:** an extra `minters[0x0]=true` is written.

When `minterAllowed` is also given, a trailing comma instead causes a count mismatch or a decimal-parse error.

**Sibling site: GovValidator `validators` / `blsPublicKeys`.** In `initializeValidator` (`systemcontracts/gov_validator.go:92-94` split, `:104` `HexToAddress(valAddr)`, `:109` `FromHex(key)`, `:124` `HexToAddress(memberAddresses[i])`), there is no trim and no validation:
- **Space form:** silently writes the zero validator and an empty BLS key, which gives a different root.
- **Trailing comma:** caught by the count checks at `:95` and `:98`, so it errors.

**Competing hypotheses ruled out:**
- **GovBase `members` (`gov_base.go:164-175`).** It is also untrimmed, but the zero-address check at `:173` returns an error for both forms. That contradicts "no error". It is the same latent bug, but here it fails loudly.
- **GovCouncil blacklist / authorizedAccounts (`gov_council.go:130`) and GovMasterMinter minters (`gov_master_minter.go:91`).** These already `TrimSpace` and check for zero. The space form parses correctly and a trailing comma errors, so neither gives a silent divergence. They show that the intended convention is "trim and reject".
- **Alloc map iteration order or JSON whitespace.** The trie root does not depend on order. Also, editing only the param *string* fixes the hash, so the difference must come from parsing that string.

## 2. Evidence

- `systemcontracts/coin_adapter.go:51` `minters := strings.Split(mintersStr, ",")`; `:69` `minterAddress := common.HexToAddress(minter)`. No trim and no zero or empty guard. The value is written as `CalculateMappingSlot(mintersSlot, minterAddress) = 1` (`:70-74`).
- `systemcontracts/gov_validator.go:92-94` split members/validators/blsPublicKeys; `:104` and `:109` are unguarded parses. The BLS bytes feed `validatorToBlsKey` and `blsKeyToValidator` (`:142-151`).
- `systemcontracts/gov_base.go:164,170,173`: untrimmed parse, but with a zero guard that errors (ruled out).
- `systemcontracts/gov_council.go:130` and `systemcontracts/gov_master_minter.go:91` use `HexToAddress(strings.TrimSpace(addrStr))` plus a zero guard. This is the correct convention.
- `common/bytes.go:29-37` `FromHex` does no trimming; `common/bytes.go:79-81` `Hex2Bytes` discards the decode error; `common/types.go:231` `HexToAddress = BytesToAddress(FromHex(s))`.
- Call chain from stablenet-knowledge `find_callers` / `get_for_task` graph_neighbors:
  - `initializeCoinAdapter` / `initializeValidator` / `initializeBase` ← `systemcontracts.GetSystemContractsTransition` (`systemcontracts/systemcontracts.go:62-111`)
  - ← `core.InjectContracts` (`core/genesis.go:668-686`, writes `Alloc.Storage`)
  - ← `core/genesis.go:236-379` (genesis setup) and `core/chain_makers.go:429-445`
  - Alloc → `hashAlloc` (`core/genesis.go:118-148`) → `statedb.Commit` → state root.
- Second consumer: `consensus/wbft/config.go:200-209` `GetSystemContractsStateTransition` → `GetSystemContractsTransition`, called from `consensus/wbft/engine/engine.go:929-970` for mid-chain `SystemContractUpgrades`.
- Runtime probe: I tried a throwaway in-process go test that calls `initializeCoinAdapter` and `initializeBase` with clean, space and trailing-comma lists. The permission system denied it (no approval surface), so it never ran and nothing needed reverting. The behaviour of `hex.DecodeString` on `"0 0x…"` (0 bytes, under go 1.22) is inferred from the stdlib source, not observed.

## 3. Affected sites

| site | role | silently diverges? | must_fix |
|---|---|---|---|
| `systemcontracts/coin_adapter.go:51,69` (minters; also `:54/:59` minterAllowed split/parse) | producer (primary) | yes, both forms | yes |
| `systemcontracts/gov_validator.go:93,104` (validators) | sibling producer | yes, space form | yes |
| `systemcontracts/gov_validator.go:94,109` (blsPublicKeys via `FromHex`) | sibling producer | yes, space form (empty BLS key) | yes |
| `systemcontracts/gov_validator.go:92,124` (members re-split for operator mapping) | sibling producer | currently shielded by `initializeBase` erroring | yes (same parse must match base) |
| `systemcontracts/gov_base.go:164,170` (members) | sibling producer | no (errors) | yes, for consistent normalization |
| `systemcontracts/gov_council.go:71,87,130` / `gov_master_minter.go:83,91` | already trim | no (trailing comma errors) | optional: decide on the empty-entry policy |
| `consensus/wbft/config.go:200` upgrade path | second consumer of the same parsers | same divergence would fork the state at the upgrade block | covered by fixing the parsers |

A single shared helper used by all of the above is the natural fix surface.

## 4. Confidence

**Medium-high.** The static chain is complete and explains both distinguishing features:
- No error, because no zero or empty guard exists at the coin-adapter and validator sites.
- Fixing the string fixes the hash, because the difference comes only from parsing.

The ruled-out sites are excluded by their own error paths. Confidence is not "high" because the runtime probe was blocked, and the ticket does not say which contract's list was edited (coin-adapter `minters` fits both forms; `validators`/`blsPublicKeys` fit only the space form).

What would raise it:
- A go test showing that `initializeCoinAdapter` with `"A, B"` and `"A,B,"` returns `err == nil`, a `mintersSlot` key for `0x0`, and a different alloc root from `"A,B"`.
- The divergent node's actual genesis params, to identify which list was edited.

## 5. Suggested direction

Add one shared parser in `systemcontracts` for comma-separated address lists and one for hex lists. Each should:
- `strings.TrimSpace` every entry;
- either drop empty entries or reject them explicitly (decide the policy once — a trailing comma should behave the same everywhere);
- validate strictly, using `common.IsHexAddress` instead of `HexToAddress`'s lossy decoding, and checking non-empty, well-formed BLS keys;
- return an error that names the param.

Route `initializeCoinAdapter` (`minters`, `minterAllowed`), `initializeValidator` (`members`, `validators`, `blsPublicKeys`), `initializeBase` (`members`), `initializeGovCouncil` and `initializeGovMasterMinter` through it. Then equivalent configs give identical storage, and malformed configs fail loudly instead of producing a different root. Normalizing changes the root only for configs that were mis-parsed before. Still check whether any live network's genesis or `SystemContractUpgrades` relied on the old parse, because the upgrade path in `consensus/wbft/config.go:200` replays these parsers at block-processing time.

Side finding (not in scope): even with clean input, `initializeValidator` never rejects a zero validator or an empty BLS key, so a malformed key list can create a validator set that cannot sign. Treat this as a separate hardening item.
