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
| interface calls | method value selections on an interface receiver | `calls` and `invokes` edges |
| type references | uses of a package-level named type from a declaration | `uses_type` and `instantiates` edges |

Per relation a truth pair is matched, misresolved (the graph links the caller to another node of the same name
and kind), missing, or has an endpoint without a node. Graph edges between matched declarations without a truth
pair are counted as unexpected; misresolved edges are among them. Dynamic calls (function values, method values,
closures) and calls in package-level initialisers are counted, not checked: go/types gives no callee for the
first, and the second have no enclosing function.

The canonical id format is reimplemented in the verifier, not imported from the parser, so the two can disagree.

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
- "Misresolved" means the caller has an edge to another node with the same name and kind and none to the expected
  one. When the caller really calls both (an edge to one is right, the call to the other is missing for another
  reason, e.g. it sits in a function literal), the pair is still counted as misresolved. The 2 static and 6
  type-reference cases left after D2 look like this; they were not checked one by one.
- Truth covers Go only, the files the graph holds, and the host GOOS/GOARCH (no build tags).
- Whether a defect changes an evidence pack is not measured here. That is the next step: fix a class, rebuild,
  and compare packs on the evaluation cases.

## Order of fixes (proposed)

By expected effect on retrieval, cheapest first:

1. Resolve calls and type references by the go/types object (canonical id) instead of the short name; drop the
   suffix fallback. Fixes classes 1 and 3 and makes edge resolution deterministic.
2. Keep function-local declarations out of the package-level id space (no canonical id, or a scoped one). Fixes
   class 2.
3. Record calls inside function literals and calls to generic methods (use `Origin()`). Fixes classes 4 and 5.
4. Emit type references from all positions, then `reads_field` and `references`.

Each fix is judged by `ckg verify` first, then by the change in evidence packs on the go-stablenet cases.
