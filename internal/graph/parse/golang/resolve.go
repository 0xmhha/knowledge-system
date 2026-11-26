package golang

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Resolve unions per-file results and uses go/types to resolve PendingRefs.
// V0 implementation: resolves call-target qnames to existing function/method
// nodes by qname suffix match. Unresolved pending refs are dropped (V0
// simplification — emitting AMBIGUOUS edges would violate the schema's
// foreign-key constraint on edges.dst, so full AMBIGUOUS handling is
// deferred until edge persistence supports nullable dst).
func (p *Parser) Resolve(results []*parse.ParseResult) (*parse.ResolvedGraph, error) {
	out := &parse.ResolvedGraph{}
	p.setBuildFiles(results)
	qIndex := map[string]string{} // qname -> nodeID (Function/Method only)
	// cIndex maps canonical id -> nodeID for Function/Method nodes. A pending
	// ref that carries TargetCanonicalID resolves only through it.
	cIndex := map[string]string{}
	// callSiteParent maps a CallSite node ID to its enclosing Function/Method ID,
	// so pending refs originating from a CallSite can be lifted to the function
	// that actually performs the call (spec §4.6.1 — edges are between named
	// entities, CallSite nodes model the syntactic site).
	callSiteParent := map[string]string{}
	for _, r := range results {
		out.Nodes = append(out.Nodes, r.Nodes...)
		out.Edges = append(out.Edges, r.Edges...)
		for _, n := range r.Nodes {
			if n.Type == types.NodeFunction || n.Type == types.NodeMethod {
				qIndex[n.QualifiedName] = n.ID
				// also index trailing simple name and pkg.Name for partial matches
				suffix := simpleName(n.QualifiedName)
				qIndex[suffix] = n.ID
				if n.CanonicalID != "" {
					if _, dup := cIndex[n.CanonicalID]; !dup {
						cIndex[n.CanonicalID] = n.ID
					}
				}
			}
		}
		// CallSite -> enclosing Function/Method of the same file, by byte
		// span. The CallSite qname prefix ("<parentQname>#CallSite@<start>-<end>")
		// is not unique across packages that share a name (two `package
		// main`, several `init`), so looking the parent up by it attached
		// calls to the wrong function. Spans work for freshly parsed files and
		// for cached files reloaded without their edges.
		for id, parent := range enclosingFuncs(r.Nodes) {
			callSiteParent[id] = parent
		}
		// A call site in a package-level initialiser is inside no function:
		// the variable it initialises makes the call. The contains edge of
		// the site names that variable exactly; enclosingFuncs falls back to
		// spans for files reloaded without their edges.
		for id, owner := range initialiserOwners(r.Nodes, r.Edges) {
			callSiteParent[id] = owner
		}
	}
	// The suffix fallback below is for AST-only builds, where targets are
	// bare names. With type information every resolvable target carries a
	// canonical id or an exact qname; a suffix match there only produced
	// false edges (a conversion `uint(x)` bound to an unrelated method
	// `(*Stream).uint`, a func variable `cancel()` to `(*Downloader).cancel`).
	typed := len(p.pkgs) > 0
	var qnames []string // sorted, so the fallback picks the same target every build
	if !typed {
		qnames = make([]string, 0, len(qIndex))
		for q := range qIndex {
			qnames = append(qnames, q)
		}
		sort.Strings(qnames)
	}
	extAdded := map[string]bool{}
	for _, r := range results {
		for _, pr := range r.Pending {
			conf := types.ConfExtracted
			var id string
			var ok bool
			if pr.TargetCanonicalID != "" {
				// Exact identity only: a short-name match could be another
				// package with the same name (crypto/bn256 vs
				// crypto/bn256/cloudflare). A miss means the callee is outside
				// the graph (stdlib, excluded files).
				id, ok = cIndex[pr.TargetCanonicalID]
			} else if typed && !strings.Contains(pr.TargetQName, ".") {
				// A bare name in a typed build is a callee go/types could not
				// tie to a declaration by canonical id (a method of an
				// interface declared in a function, which the statement pass
				// resolves by position; a call through an expression). qIndex
				// also holds simple names, so such a call bound to whichever
				// method of that name was indexed last.
				ok = false
			} else {
				id, ok = qIndex[pr.TargetQName]
				if !ok && !typed && pr.TargetQName != "" {
					for _, q := range qnames {
						if strings.HasSuffix(q, "."+pr.TargetQName) || q == pr.TargetQName {
							id, ok = qIndex[q], true
							break
						}
					}
				}
			}
			if !ok && pr.TargetCanonicalID != "" {
				// A function or method outside the build (standard library,
				// a dependency, a file of the module the build scope leaves
				// out, the builtin error method): point at its External node.
				if n, found := p.ExternalTarget(pr.TargetCanonicalID); found {
					if !extAdded[n.ID] {
						extAdded[n.ID] = true
						out.Nodes = append(out.Nodes, n)
					}
					id, ok = n.ID, true
				}
			}
			if !ok {
				// No node to point at: the edge is dropped (an edge needs both
				// endpoints), but the reason is counted so missing edges can be
				// explained (docs/graph/BUILD-AND-QUERY-FLOW.md).
				switch {
				case pr.TargetCanonicalID != "":
					out.CountUnresolved(parse.UnresolvedOutsideGraph, pr.TargetCanonicalID, pr.HintFile, pr.Line)
				case pr.TargetQName == "":
					out.CountUnresolved(parse.UnresolvedNoTarget, string(pr.EdgeType), pr.HintFile, pr.Line)
				case typed && !strings.Contains(pr.TargetQName, "."):
					out.CountUnresolved(parse.UnresolvedBareName, pr.TargetQName, pr.HintFile, pr.Line)
				default:
					out.CountUnresolved(parse.UnresolvedNoMatch, pr.TargetQName, pr.HintFile, pr.Line)
				}
				continue
			}
			src := pr.SrcID
			if parentID, ok := callSiteParent[src]; ok {
				src = parentID
			}
			// listens_on PendingRefs from the distributed pass encode
			// (endpoint, handler-name) — but the conventional edge direction
			// is handler → endpoint. Swap src↔dst so downstream consumers
			// don't have to special-case this edge type. Documented in
			// distributed.go's maybeEmitHTTPListensOn fallback comment.
			finalSrc, finalDst := src, id
			if pr.EdgeType == types.EdgeListensOn {
				finalSrc, finalDst = id, src
			}
			out.Edges = append(out.Edges, types.Edge{
				Src: finalSrc, Dst: finalDst, Type: pr.EdgeType,
				Line: pr.Line, Count: 1, Confidence: conf,
				DispatchKind: pr.DispatchKind,
			})
		}
	}
	return out, nil
}

// LoadOptions tunes LoadAndResolveWith.
type LoadOptions struct {
	// Tests loads test files too, as a build does: a package with tests is
	// then type-checked a second time as its test variant.
	Tests bool
	// Skip leaves out the files it returns true for (slash-separated paths
	// relative to the root), as a build scope leaves files of the module out
	// of the build. Their packages are still loaded and type-checked.
	Skip func(rel string) bool
}

// LoadAndResolve is a convenience for tests: walks Go files under root,
// runs Pass 1 on each, then Pass 2 across the union.
//
// Type-aware: registers the loaded packages with the parser via
// SetPackages and runs every pass a build runs after Resolve, so a test
// sees the graph a build of the same files would store.
func LoadAndResolve(root string) (*parse.ResolvedGraph, error) {
	return LoadAndResolveWith(root, LoadOptions{})
}

// LoadAndResolveWith is LoadAndResolve with options.
func LoadAndResolveWith(root string, opts LoadOptions) (*parse.ResolvedGraph, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps |
			packages.NeedImports | packages.NeedModule | packages.NeedCompiledGoFiles,
		Dir:   root,
		Tests: opts.Tests,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}
	p := New(root)
	p.SetPackages(pkgs)
	var paths []string
	seen := map[string]struct{}{}
	for _, pkg := range pkgs {
		for _, path := range pkg.GoFiles {
			if _, dup := seen[path]; dup {
				continue
			}
			seen[path] = struct{}{}
			if rel, err := filepath.Rel(root, path); err == nil && opts.Skip != nil && opts.Skip(filepath.ToSlash(rel)) {
				continue
			}
			paths = append(paths, path)
		}
	}
	// A build parses files in path order; packages.Load's order is not fixed.
	sort.Strings(paths)
	var results []*parse.ParseResult
	for _, path := range paths {
		src, err := readFile(path)
		if err != nil {
			return nil, err
		}
		r, err := p.ParseFile(path, src)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	rg, err := p.Resolve(results)
	if err != nil || rg == nil {
		return rg, err
	}
	// implements / extends edges are a post-Resolve pass — they need
	// (a) the union of nodes (qname → ID lookup) and (b) the loaded package
	// scopes (go/types satisfaction queries). Wired here so tests using
	// LoadAndResolve exercise the same code path as production buildpipe.
	implEdges, implPending := EmitImplementsEdges(p.Pkgs(), rg.Nodes)
	rg.Edges = append(rg.Edges, implEdges...)
	// uses_type edges (Function/Method/Struct → Type). Same
	// post-Resolve location for the same reasons. Cross-package types that
	// don't have a node in this graph become PendingRefs (q4=A); LoadAndResolve
	// drops the pending side because tests don't replay them, but the
	// production buildpipe path persists them via runGoPipeline.
	usesEdges, usesPending := EmitUsesTypeEdges(p.Pkgs(), rg.Nodes)
	rg.Edges = append(rg.Edges, usesEdges...)
	extNodes, extEdges := p.ExternalEdges(append(usesPending, implPending...), rg.Nodes)
	rg.Nodes = append(rg.Nodes, extNodes...)
	rg.Edges = append(rg.Edges, extEdges...)
	// instantiates edges (Function/Method → Type). Walks each
	// function body for composite literals and `new(T)` calls.
	rg.Edges = append(rg.Edges, EmitInstantiatesEdges(p.Pkgs(), rg.Nodes)...)
	// Defect C: promoted-method nodes (embedding type -> embedded method).
	// Runs after the per-file nodes are unioned so the declaring method nodes
	// exist for the in-module bound. Emits method nodes + defines edges.
	promNodes, promEdges := EmitPromotedMethods(p.Pkgs(), rg.Nodes)
	rg.Nodes = append(rg.Nodes, promNodes...)
	rg.Edges = append(rg.Edges, promEdges...)
	// One file shows each package, the same on every build (see packages.go).
	CanonicalPackagePaths(rg.Nodes, isGoTestFile)
	// Lock edges: acquires_lock / releases_lock / accessed_under_lock (see lock_edges.go).
	rg.Edges = append(rg.Edges, EmitLockEdges(p.Pkgs(), rg.Nodes)...)
	// Value uses: reads_field / writes_field and references (see value_access.go).
	rg.Edges = append(rg.Edges, EmitValueAccessEdges(p.Pkgs(), rg.Nodes)...)
	// Runtime links: dispatches_to from SSA + VTA (see dispatch.go).
	dispatch, _ := EmitDispatchEdges(p.Pkgs(), rg.Nodes)
	rg.Edges = append(rg.Edges, dispatch...)
	// Statement links: what happens inside a function (see statement_links.go).
	stmtEdges, _ := EmitStatementEdges(p.Pkgs(), rg.Nodes)
	rg.Edges = append(rg.Edges, stmtEdges...)
	return rg, nil
}

func simpleName(qname string) string {
	i := strings.LastIndex(qname, ".")
	if i < 0 {
		return qname
	}
	return qname[i+1:]
}

func readFile(path string) ([]byte, error) {
	// indirection for testability
	return readFileOS(path)
}

// enclosingFuncs maps each CallSite node of one file to the node that makes
// the call: the innermost Function or Method whose byte span contains it, or,
// for a site inside no function, the innermost Variable whose span contains
// it (the call is in that variable's package-level initialiser).
func enclosingFuncs(nodes []types.Node) map[string]string {
	var funcs, vars []types.Node
	for _, n := range nodes {
		switch n.Type {
		case types.NodeFunction, types.NodeMethod:
			funcs = append(funcs, n)
		case types.NodeVariable:
			vars = append(vars, n)
		}
	}
	out := map[string]string{}
	for _, n := range nodes {
		if n.Type != types.NodeCallSite {
			continue
		}
		if id := innermost(funcs, n); id != "" {
			out[n.ID] = id
		} else if id := innermost(vars, n); id != "" {
			out[n.ID] = id
		}
	}
	return out
}

// innermost returns the ID of the smallest of owners, in site's file, whose
// byte span contains the start of site; "" when none does.
func innermost(owners []types.Node, site types.Node) string {
	best := -1
	for i, o := range owners {
		if o.FilePath != site.FilePath || site.StartByte < o.StartByte || site.StartByte >= o.EndByte {
			continue
		}
		if best < 0 || o.EndByte-o.StartByte < owners[best].EndByte-owners[best].StartByte {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return owners[best].ID
}

// initialiserOwners maps each CallSite node that a Variable contains to that
// Variable. The declaration pass adds the contains edge when it walks a
// package-level initialiser, and it knows which name a value belongs to,
// which spans do not say for `var a, b = f(), g()`.
func initialiserOwners(nodes []types.Node, edges []types.Edge) map[string]string {
	kind := make(map[string]types.NodeType, len(nodes))
	for _, n := range nodes {
		kind[n.ID] = n.Type
	}
	out := map[string]string{}
	for _, e := range edges {
		if e.Type == types.EdgeContains && kind[e.Src] == types.NodeVariable && kind[e.Dst] == types.NodeCallSite {
			out[e.Dst] = e.Src
		}
	}
	return out
}
