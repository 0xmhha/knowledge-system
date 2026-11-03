# Diagnosis — genesis state root diverges with spaced / trailing-comma address lists

Retrieval backend: ok, serviceable (ckg + ckv). Index head 6e41d966 matches repo HEAD.

## 1. Root cause

The value is the **set of addresses written into system-contract genesis storage**. Those
`params.StateParam` slots go into `genesis.Alloc[...].Storage` (`InjectContracts`), and from there
into the genesis state root and block hash.

The broken edge is at the **produce** step: comma-separated address params are split with a bare
`strings.Split(s, ",")`, and each piece goes to `common.HexToAddress` with no `TrimSpace`, no
check for empty pieces, and (at the silent sites) no zero-address check. `common.FromHex` only
strips a `0x` that sits at index 0. So `" 0xabc…"` (leading space) is padded to odd length and
fails hex decode. `Hex2Bytes` throws the error away, so the result is the **zero address**. An
empty trailing piece `""` also becomes the zero address. No error is raised.

The main silent site is `initializeCoinAdapter` (`systemcontracts/coin_adapter.go:51,69`). It is
the only parser that **silently accepts both** variants in the ticket:
- `"a, b"`: writes minters {a, 0x0}. b is lost.
- `"a,b,"`: writes minters {a, b, 0x0}.

Either way the NativeCoinAdapter storage is different, so the state root and genesis hash
change, and no error appears. A second silent site is `initializeValidator`
(`systemcontracts/gov_validator.go:93-94,104,124`): a space in `validators` silently registers a
zero-address validator, together with its operator/BLS mappings.

Hypotheses ruled out:
- **"All list parsers silently drop or keep whitespace"** (gov_base members, govCouncil,
  govMasterMinter). Ruled out by the runtime probe. `initializeBase` rejects both variants with
  "contains invalid zero address". govCouncil and govMasterMinter `TrimSpace` each piece, so
  spaces are harmless there, and they reject the trailing-comma empty piece with an error. These
  sites fail loudly. They do not match the "no error message" symptom. They are inconsistent,
  but they are not the divergence.
- **A genesis JSON / canonicalisation or `SetupGenesisBlockWithOverride` path problem.** Ruled
  out: `SetupGenesisBlockWithOverride` (`core/genesis.go:236-379`) and `InjectContracts`
  (`core/genesis.go:667-686`) copy `transition.States` into the alloc exactly as received. The
  difference already exists in the `StateParam` list that the systemcontracts initializers produce.

## 2. Evidence

- `common/bytes.go:29-37` `FromHex`: `has0xPrefix` (`:51-53`) only checks `str[0]=='0'`. A
  leading space defeats the check, and odd length then pads `"0"+s`. `Hex2Bytes`
  (`common/bytes.go:79-82`) ignores the `hex.DecodeString` error. `common/types.go:229-231`
  `HexToAddress = BytesToAddress(FromHex(s))`, so the result is the zero address.
- `systemcontracts/coin_adapter.go:50-51`: `minters := strings.Split(mintersStr, ",")`.
  `:68-74` calls `common.HexToAddress(minter)` (no trim) and writes `minters[addr]=true` with no
  zero-address check. `:54` `minterAllowed` is split the same way. Its count check at `:55`
  catches a trailing comma only when `minterAllowed` is also given.
- `systemcontracts/gov_validator.go:92-94`: members, validators and blsPublicKeys are split
  without trimming. `:104` `HexToAddress(valAddr)`, `:108` `FromHex(key)` and `:124`
  `HexToAddress(memberAddresses[i])` have no trim and no zero check. The validator, operator and
  BLS slots are then written at `:127-170`.
- `systemcontracts/gov_base.go:164,170` (pack body): split without trim, but the zero-address check
  returns an error (loud).
- `systemcontracts/gov_council.go:130` and `systemcontracts/gov_master_minter.go:91` use
  `strings.TrimSpace`. Combined with the zero check, a trailing comma produces an error (loud).
- Call edges: `systemcontracts/systemcontracts.go:67,76,85,94,103` (`GetSystemContractsTransition`)
  calls `initializeValidator` / `initializeCoinAdapter` / `initializeMinter` /
  `initializeMasterMinter` / `initializeGovCouncil`. `core/genesis.go:668` `InjectContracts`
  calls `GetSystemContractsTransition`, and is itself called by `SetupGenesisBlockWithOverride`
  (`core/genesis.go:236`) and `core/chain_makers.go:429` (graph_neighbors from the get_for_task pack).
- In-repo convention: `p2p/netutil/net.go:71-88` `ParseNetlist` is documented with "Whitespace
  and extra commas are ignored". It strips whitespace and skips empty pieces.
- Runtime probe (a throwaway in-package Go test, deleted afterwards; the tree is clean):
  - `HexToAddress(" 0x11…") = 0x0`, `HexToAddress("") = 0x0`
  - coinAdapter `"a, b"`: err=nil, minter slots `{8eec…(a), a6ee…(zero)}`. Clean input gives `{8eec…, c611…(b)}`.
  - coinAdapter `"a,b,"`: err=nil, slots `{a, b, zero}`.
  - govValidator validators `"a, b"`: err=nil, the second validator is the zero address.
  - govBase `"a, b"` and `"a,b,"`, govMasterMinter `"a,b,"`, govCouncil `"a,b,"`: each returns an error.

## 3. Affected sites

| site | role | silent symptom? | must fix |
|---|---|---|---|
| `systemcontracts/coin_adapter.go:51,69` (`initializeCoinAdapter`, `minters`) | producer (primary broken edge) | yes, both space and trailing comma | yes |
| `systemcontracts/coin_adapter.go:54,58` (`minterAllowed` list) | sibling parse | no (a space makes `SetString` fail: loud). Must stay index-aligned with `minters` after normalisation | yes (same normalisation) |
| `systemcontracts/gov_validator.go:92-94,104,108,124` (`initializeValidator`: validators, members[i], blsPublicKeys) | producer (sibling) | yes, a space in validators gives a zero validator; a space in BLS keys gives garbage bytes | yes |
| `systemcontracts/gov_base.go:164,170` (`initializeBase` members; reached by govValidator, govMinter, govMasterMinter, govCouncil) | producer | no (rejects with an error) | yes, for consistency (trim and skip empty) |
| `systemcontracts/gov_council.go:71,87,130` (`initializeAddressSet`: blacklist, authorizedAccounts) | producer | no (trailing comma gives an error) | consistency |
| `systemcontracts/gov_master_minter.go:83,91` (`minters`) | producer | no (trailing comma gives an error) | consistency |
| `systemcontracts/systemcontracts.go:67-103` `GetSystemContractsTransition` / `core/genesis.go:668` `InjectContracts` | consumer (alloc storage → state root) | carries the bad value | no |

Functions behind the ticket's observations: the "different genesis hash/state root" comes from
`InjectContracts` → `Genesis.Commit` / `ToBlock` (`core/genesis.go:668`, `:236`), with the bad slots
produced at `coin_adapter.go:69` and `gov_validator.go:104`. The "no error message" behaviour
comes from `common.Hex2Bytes` (`common/bytes.go:80`) discarding the decode error, combined with
the missing zero-address checks at those two sites.

## 4. Confidence

**High** for the mechanism and the NativeCoinAdapter `minters` site. The runtime probe shows
different storage slots with no error for both ticket variants. **Medium** on which param the
reporter's config actually used: it is minters (both variants silent) or validators (only the
space variant silent). Confidence would rise further with the reporter's two genesis files and a
diff of the `alloc` storage between `gstable init` outputs. Expect extra zero-address keys under
the NativeCoinAdapter and/or GovValidator addresses.

## 5. Suggested direction

Add one shared list parser in `systemcontracts` that splits on `,`, trims whitespace from each
piece, and skips empty pieces. This follows the documented `ParseNetlist` convention, "Whitespace
and extra commas are ignored". Use it for every comma-separated genesis param: govBase members,
govValidator validators/blsPublicKeys and the `members[i]` pairing, coinAdapter
minters/minterAllowed, govMasterMinter minters, govCouncil blacklist/authorizedAccounts. That
way configs that differ only in formatting produce identical storage. As defence in depth, make
the parser (or the callers) reject malformed or zero addresses with an explicit error, for
example via `common.IsHexAddress` after trimming. Without that, any future formatting slip that
the parser does not normalise fails loudly instead of silently changing the state root. This
matters most for coinAdapter minters and govValidator validators, which today have no zero check.
Add table tests for `"a, b"`, `"a,b,"` and `" a ,b "` that assert storage identical to `"a,b"`.

Rejected directions:
- Do not "fix" this in `common.HexToAddress` / `FromHex` by trimming there. Those are upstream
  go-ethereum primitives with broad callers, and their silent-lenient behaviour is relied on
  widely. The defect is the param parsing in `systemcontracts`.
- Do not make the loud sites (govBase, govCouncil, govMasterMinter) the only behaviour, rejecting
  all whitespace everywhere, unless that is a product decision. The ticket's acceptance is that
  formatting-only differences give the same hash. The in-repo convention (`ParseNetlist`)
  tolerates whitespace and extra commas.
