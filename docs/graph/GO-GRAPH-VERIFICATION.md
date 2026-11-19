# Go graph verification against go/types

This page records what `ckg verify` measures and what it found on the go-stablenet graphs. Keep the "Results"
section current when the parser changes; add a row instead of rewriting an old one.

## Why

Every retrieval result rests on the graph: a wrong call edge sends Stage 3 to the wrong neighbour, and a wrong
type edge attaches knowledge to the wrong declaration. The published code-graph systems we compared (LocAgent,
CodexGraph, RepoGraph, GraphCoder) judge their graphs only through a downstream task score; none measures the
graph itself against a reference. For Go a reference exists: the compiler front end. `ckg verify` compares the
graph with it, so graph defects can be counted, fixed and kept fixed without an LLM or human labels.

## What it compares

`ckg verify --src <source root> --graph <graph dir> [--format text|json] [--samples N]`
(`internal/graph/verify`). It loads the source with go/packages and go/types (tests included) and checks only the
Go files recorded in the graph.

| relation | truth (go/types) | graph |
|---|---|---|
| declarations | package-level funcs, methods, interface methods, types, consts, vars, with the documented canonical id format; init functions keyed by position | node of a declaration kind in the same file, same canonical id without the `@line` suffix, span containing the name |
| static calls | `typeutil.StaticCallee` inside a function body, closures included, attributed to the enclosing declaration | `calls` and `invokes` edges |
| interface calls | method value selections whose method is declared by an interface, on an interface receiver or through an interface embedded in a struct | `calls` and `invokes` edges |
| type references | uses of a package-level named type from a declaration, including the signature of a function declared without a Go body (assembly) | `uses_type` and `instantiates` edges |

Per relation a truth pair is matched, misresolved (the graph links the caller to another node of the same name
and kind, and that other edge is not a truth pair either), missing (the edge is absent; an edge to a same-named node
that is itself a truth pair does not stand in for it), or has an endpoint without a node. Graph edges between matched declarations without a truth
pair are counted as unexpected; misresolved edges are among them. Dynamic calls (function values, method values,
closures) and calls in package-level initialisers are counted, not checked: go/types gives no callee for the
first, and the second have no enclosing function.

The canonical id format is reimplemented in the verifier, not imported from the parser, so the two can disagree.

The truth comes from the same go command as the graph. A package it cannot load or type-check (a missing or broken
module, cgo that cannot run for the configuration) is missing from both, and the comparison agrees, so the report
carries the load as well: packages reached, packages with errors, errors by kind with samples, and with `--scope`
the outside modules matched, at another version, from another directory or not loaded (`load` in JSON, `go load`
in text). A clean comparison with load errors is not a clean graph.

## Results

go-stablenet, graphs of the 11 evaluation base commits built with `ckg build --files-from-main ./cmd/gstable`
(knowledge-system 1d5659cd), about 1,005 to 1,010 Go files each. The numbers barely move between commits; the
row is PR 74's base (319b84d1).

| relation | truth | matched | misresolved | missing | recall |
|---|---|---|---|---|---|
| declarations | 17,835 | 17,834 | - | 1 | |
| static calls | 27,545 | 26,871 | 7 | 640 | 0.9765 |
| interface calls | 2,886 | 2,812 | 1 | 60 | 0.9788 |
| type references | 24,028 | 13,076 | 357 | 10,595 | 0.5442 |

After resolving by canonical id (roadmap D2 and D3, same base commit, graph rebuilt with the fixed parser):

| relation | truth | matched | misresolved | missing | recall |
|---|---|---|---|---|---|
| declarations | 17,835 | 17,834 | - | 1 | |
| static calls | 27,545 | 27,205 | 2 | 311 (306 in function literals) | 0.9886 |
| interface calls | 2,886 | 2,812 | 1 | 60 (59 in function literals) | 0.9788 |
| type references | 24,028 | 13,487 | 6 | 10,535 | 0.5613 |

Unexpected graph edges fell from 931 to 126 (calls) and from 47 to 8 (type references); declaration nodes with no
package-level declaration fell from 123 to 0.

After recording calls inside goroutine function literals and dropping bare-name targets in typed builds (roadmap D4):

| relation | truth | matched | misresolved | missing | recall |
|---|---|---|---|---|---|
| static calls | 27,545 | 27,508 | 0 | 10 (5 in function literals) | 0.9996 |
| interface calls | 2,886 | 2,872 | 0 | 1 | 0.9997 |
| type references | 24,028 | 13,487 | 6 | 10,535 | 0.5613 |

Unexpected call edges fell from 126 to 8. Of the 126, 112 came from `err.Error()`: a method of the builtin `error`
interface has no package, so it has no canonical id, and its bare name `Error` matched whichever `Error` method was
indexed last. The 8 left are calls through an interface embedded in a struct field (`p.rw.ReadMsg`,
`transport.close`); the graph links them to the interface method, which is right, but the verifier counts them as
dynamic calls (go/types gives no static callee and the receiver is the struct, not the interface), so they show up
as unexpected.

Not checked: 1,714 dynamic call sites, 2,229 calls in package-level initialisers.
Unexpected graph edges: 931 call edges, 47 type-reference edges. Declaration-kind graph nodes with no
package-level declaration: 123.

### Defect classes

1. **Same short package name resolves to the wrong package** (misresolved calls). Calls are recorded by the short
   name `pkg.Func` and resolved in `internal/graph/parse/golang/resolve.go` through one map keyed by that name, so
   when two packages share a name the last one indexed wins. Confirmed in the source:
   - `core/vm.runBn256Pairing` calls `crypto/bn256.PairingCheck`; the graph points to
     `crypto/bn256/cloudflare.PairingCheck`.
   - `eth.(*ethHandler).Handle` calls `eth.handleTransactions` (same package); the graph points to the unexported
     `eth/protocols/eth.handleTransactions`, a call Go does not allow.
   - `consensus/wbft/core.New` and `core.NewBlockChain` call `common/prque.New`; the graph points to `log.New`
     and `core/state/snapshot.New`.
2. **Function-local types exposed under a package-level id** (misresolved type references, 357). A type declared
   inside a function, such as the `type Genesis struct` inside `(Genesis).MarshalJSON` in the generated
   `core/gen_genesis.go`, becomes a node with id `core.Genesis@21`. References to the package-level
   `core.Genesis` (`core/genesis.go:58`) resolve to it: the real struct has 0 incoming edges, the local one 140.
   `uses_type.go` maps the short qualified name to the first type node it meets, and the local type and the
   package-level one share `core.Genesis`.
   The same happens for every gencodec type (`engine.ExecutableData`, `types.Withdrawal`,
   `types.SetCodeAuthorization`, ...). The 123 unmatched declaration nodes are these local types and the methods of
   local interfaces.
3. **Suffix fallback creates false edges** (unexpected calls, 931). In `resolve.go` an unresolved target falls back
   to any indexed function whose name ends with the same suffix. The fallback iterates a Go map and takes the first
   hit, so the chosen target can also vary between builds. Targets seen in the samples: conversions (`string(x)`, `uint(x)`), local function variables
   (`cancel()`, `f()`, `callback()`), and same-named methods of unrelated types, e.g.
   `node.convertFileLockError -> rlp.(*Stream).uint`.
4. **Calls inside function literals are not recorded** (missing, 314 static and 59 interface calls). For example
   `cmd/gstable.exportChaindata` starts `go func() { ... log.Info(...) }()`; neither the call nor a pending
   reference for it exists in the graph.
5. **Calls to methods of generic types are not recorded** (missing, most of the remaining 326 static calls; 224
   outside test files). The top callee types are all generic: `common/prque.Prque[P, V]`, `common/lru.Cache[K, V]`
   and `BasicLRU`, `rlp.RawList[T]`, `systemcontracts.EnumerableSet[T]`.
6. **Most type references are not edges** (missing, 10,595). By the referring declaration: methods 7,712,
   functions 2,163, interfaces 388, type aliases 175, structs 113. `uses_type.go` emits edges only from function
   parameters and results and from struct fields; types used in function bodies, interface method signatures and
   alias targets are not covered. The verifier's truth counts all of these, so part of this gap is a scope
   difference rather than a defect; the split has not been measured.
7. Also seen in the stores, not counted by the verifier: `reads_field` and `references` edges are defined but never
   emitted (0 in every graph); 608 of 7,908 method nodes have no canonical id.

### What these numbers are not

- Recall is over pairs, not call sites; a pair counts once however many times it is called.
- "Misresolved" means the caller has an edge to another node with the same name and kind, none to the expected one,
  and go/types does not support the other edge. Before 2026-11-15 the last condition was missing, so a function that
  refers to both `graphql.Block` and `types.Block` and has only the second edge counted the first as misresolved
  instead of missing; the earlier rows on this page include such cases.
- Truth covers Go only, the files the graph holds, and the host GOOS/GOARCH (no build tags).
- Whether a defect changes an evidence pack is not measured here. That is the next step: fix a class, rebuild,
  and compare packs on the evaluation cases.

### Rebuilt evaluation graphs (2026-11-15)

The 11 go-stablenet base commits rebuilt with knowledge-system fe94d906 (D2, D3, D4, D8) and measured with the
verifier as corrected on 2026-11-15. Base commit 6e41d966, 1,418 Go files, against the 2026-11-03 evaluation graph
of the same commit (built by an older ckg, 9147b083); the other 10 commits differ by a few pairs.

| relation | truth | matched (old -> new) | misresolved (old -> new) | missing (old -> new) | recall (old -> new) |
|---|---|---|---|---|---|
| static calls | 33,370 | 32,421 -> 33,322 | 255 -> 0 | 657 -> 11 | 0.9716 -> 0.9986 |
| interface calls | 3,307 | 3,222 -> 3,288 | 1 -> 0 | 66 -> 1 | 0.9743 -> 0.9943 |
| type references | 28,309 | 14,995 -> 16,062 | 831 -> 0 | 12,480 -> 12,244 | 0.530 -> 0.567 |

Unexpected edges: calls 1,437 -> 14, type references 124 -> 7. Declarations missing: 4 -> 4. The verifier
correction itself moved, on the new graph, 11 type references from misresolved to missing, 19 calls through embedded
interfaces from unexpected to matched, and 4 bodyless assembly declarations from unexpected to matched.

All 21 unexpected edges left on the new graph are graph defects:

- 13 calls and 3 type references, and the 4 missing declarations: node id collision (roadmap D11). The node id
  hashes the qualified name, which uses the package name, not the import path, plus the start byte.
  `crypto/bn256/google` and `crypto/bn256/cloudflare` are different packages that both declare `package bn256`;
  `TestPairings` starts at byte 254 of `main_test.go` in both, so both get one id and cloudflare's node is dropped,
  leaving its edges under google's node.
- 1 call and 3 type references: lookups by qualified name inside one build. `systemcontracts/compile` and
  `rlp/rlpgen` are both `package main`, so both `main` functions are `main.main`; several `init` functions of one
  package are all `<pkg>.init`. The edges of one land on the other (`compile.main -> rlpgen.(*Config).process`,
  `core/vm.init` of `contracts.go` -> `twoOperandParams`, which only `instructions_test.go`'s `init` uses).
- 1 type reference: a function-local type bound to the package-level type of the same name
  (`enr.TestSortedGetAndSet` declares its own `type pair`; the graph links it to `p2p/enr.pair`).

### Node identity by file (2026-11-15)

Go node ids now include the declaring file (`parse.NodeID`), package nodes are keyed by package directory, call sites
find their enclosing function by byte span instead of by qualified name, declarations that share a canonical id
(several `init`) are told apart by position, and an object go/types resolves without a canonical id (a local type)
no longer falls back to a package-level declaration of the same short name. Same base commit 6e41d966, graph rebuilt
with `ckg build`:

| measure | before | after |
|---|---|---|
| static call recall | 0.9986 | 0.9989 |
| interface call recall | 0.9943 | 0.9943 |
| type-reference recall | 0.567 | 0.568 |
| misresolved (calls, interface calls, type references) | 0, 0, 0 | 0, 0, 0 |
| unexpected call edges | 14 | 0 |
| unexpected type-reference edges | 7 | 0 |
| missing declarations | 4 | 0 |

Every unexpected edge the corrected verifier reported on this commit was one of these identity defects.

### Built from a build scope (2026-11-15)

`ckg build --scope` reads only the Go files a build scope lists (`cmd/buildscope`) and type-checks them with the
scope's GOOS, GOARCH, cgo setting and tags; `ckg verify --scope` type-checks the truth the same way. Same commit,
`./cmd/gstable` with cgo on:

| configuration | Go files | static | interface | type refs | misresolved | unexpected (calls, types) | missing decls |
|---|---|---|---|---|---|---|---|
| linux/amd64 | 987 | 0.9994 | 0.9955 | 0.5598 | 0, 0, 0 | 0, 0 | 0 |
| darwin/arm64 | 986 | 0.9987 | 0.9955 | 0.5598 | 0, 0, 0 | 0, 0 | 0 |

Without a scope the graph holds every package of the module (1,418 Go files on this commit) with the host's build
constraints.

Load errors (2026-11-16, built and verified on macOS arm64 with the scope's module cache): linux/amd64 has 19 errors
in 10 packages, because cgo cannot run for linux/amd64 on this host and packages that import `C` (the repository's
`crypto/secp256k1`, `net`, `os/user`, pebble, zstd, blst, usb) are type-checked without it; darwin/arm64 has one
(`golang.org/x/tools/internal/tokeninternal`). With an empty module cache every outside module is lost (4,719
errors) while misresolved and unexpected stay 0, which is why the load is part of the report. A linux/amd64 graph
without cgo errors needs a Linux host or a cross C compiler. Type-reference recall stays the open item (D5 in the roadmap).

### Statement links and runtime links (2026-11-18)

`ckg verify` also checks the statement links of schema 1.32 (`internal/graph/verify/statements.go`). The truth is
computed another way than the build: one flat walk keeps the stack of syntax nodes above each node and reads the
innermost control statement and its part (cond, then, else, body, case, go) off that stack; the build walks body
regions recursively. A fact is matched, missing, has a statement without a node, has a value or callee without a
node, or (a graph edge) is unexpected or given to an outer statement because the inner one has no node.

go-stablenet `0bf2f4d1`, linux/amd64 scope, after the fixes below (schema 1.33):

| link | truth | matched | missing | unexpected | endpoint without node |
|---|---|---|---|---|---|
| `site_calls` | 47,806 | 47,798 | 0 | 0 | 8 |
| `cond_reads` | 11,392 | 11,211 | 0 | 0 | 181 |
| `cond_calls` | 6,958 | 6,955 | 0 | 0 | 3 |
| `body_writes` | 2,721 | 2,702 | 0 | 0 | 19 |
| `encloses` | 52,869 | 52,869 | 0 | 0 | 0 |

The first run found, and the fixes removed:
- all init functions of a package share one canonical id, and the statement and value-use passes took the first
  file's init node for every init;
- the statements of a goroutine's function literal had no nodes (980 `encloses` facts without a node, 525 edges given
  to the goroutine instead), and the calls in a go statement's arguments had neither call sites nor `calls` edges
  (static call recall 0.9997 to 1.0000, interface calls 0.9997 to 1.0000);
- a write in a condition (`if w.v, err = f(); ...`) was recorded as `cond_reads`;
- a call of an interface method through a struct that embeds the interface was `static`;
- in the verifier: methods of alias types (no canonical id) were skipped, and a function literal passed as an
  argument of a go statement was taken for the goroutine's body.

Runtime links are checked by observation (`internal/graph/verify/observe`, decision D3 option C as the precision
check). `ckg observe --src S --scope F --out COPY` copies the tree and inserts, on the same line, a call at the start
of every function body that records the caller, call line and callee, and a call before every channel operation on a
named channel that records the channel pointer; `ckg verify --observed DIR` compares the records of a test run in
the copy. Observation shows what ran, so it measures recall, not precision.

go-stablenet `0bf2f4d1`, tests (`-short`) of consensus/wbft, core/txpool, eth/gasprice, miner, event, core/types,
core/vm, rpc, eth/fetcher, eth/protocols/eth, p2p/enode, trie, core/state on macOS arm64: 960 files instrumented
(3 cgo files left out), 22 of 23 packages passed (the failing miner test passes three times alone, in the plain and
the instrumented copy), 210,669 records.

| observed | observed | in graph | recall |
|---|---|---|---|
| interface calls (interface method, implementation) | 693 | 645 | 0.931 |
| function-value calls (caller, line, callee) | 472 | 466 | 0.987 |
| channel operations linked to some channel | 545 | 401 | 0.736 |
| sender and receiver joined by one channel node | 1,396 | 123 | 0.088 |

First run (schema 1.33). 1,111 of the 3,359 `dispatches_to` edges ran. What the samples show:
- interface calls the graph lacks are reached through reflection (rlp builds `Encoder` and `Decoder` values with
  `reflect`) or through code without SSA bodies (a `log.Logger` kept in an `atomic.Value`);
- 677 of the 1,273 unjoined channel pairs have both operations linked, to two nodes for one channel: its make site
  and the field it is stored in; 596 have one operation not linked (channels from interface method results, from
  `event.Feed`, which uses `reflect.Select`, or through outside code).

Second run (schema 1.34, user decision to do both fixes): the runtime package now keeps every channel it records
alive, because the first run's pairs mixed channels whose address the garbage collector had given to a later
channel (1,396 observed pairs fell to 267). The graph gained `flows_to` between the names of one channel (452 edges),
channel origins through cells a closure captures by reference, and `dispatches_to` of kind `type_hierarchy` (418
edges) for implementations the value flow did not link. All 23 test packages passed; 597,204 records.

| observed | observed | in graph | recall |
|---|---|---|---|
| interface calls | 693 | 693 | 1.000 (48 only by the type hierarchy) |
| function-value calls | 473 | 467 | 0.987 |
| channel operations linked to some channel | 544 | 495 | 0.910 |
| sender and receiver joined by one channel | 267 | 238 | 0.891 |

The channel operations left are on channels made outside the module (`time.Timer.C`, `context` channels), which have
no node, and on `event.Feed` channels, which it handles through `reflect.Select`. The statement-link check stays at 0
missing and 0 unexpected.

## Order of fixes (proposed)

By expected effect on retrieval, cheapest first:

1. Resolve calls and type references by the go/types object (canonical id) instead of the short name; drop the
   suffix fallback. Fixes classes 1 and 3 and makes edge resolution deterministic.
2. Keep function-local declarations out of the package-level id space (no canonical id, or a scoped one). Fixes
   class 2.
3. Record calls inside function literals and calls to generic methods (use `Origin()`). Fixes classes 4 and 5.
4. Emit type references from all positions, then `reads_field` and `references`.

Each fix is judged by `ckg verify` first, then by the change in evidence packs on the go-stablenet cases.
