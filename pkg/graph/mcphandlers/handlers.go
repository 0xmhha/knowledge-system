package mcphandlers

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/impact"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// RegisterFindSymbol resolves an exact-or-suffix qname / name to nodes.
//
// The description spells out the matching rule because an earlier
// phrasing implied a bare name worked with exact=true, but FindSymbol
// always matches qualified_name — bare names work only with
// exact=false (suffix match). The schema spells that contract out so LLM agents don't
// false-empty on `{"name":"NewTemplate","exact":true}`.
func RegisterFindSymbol(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("find_symbol",
		mcp.WithDescription("Find symbols by qualified_name. With exact=true (default), the input must match qualified_name exactly (e.g. \"render.NewTemplate\"). With exact=false, the input is treated as a suffix — a bare symbol name (\"NewTemplate\") matches every qualified_name ending in that segment, in the case you wrote. Use exact=false when you only know the symbol's short name. "+
			"Each row is one declaration in the source. Whatever is declared inside a function body (local variables, constants and types, and the members of such types) or inside a function literal of a package-level variable's initialiser (it is part of that variable) and External stubs (code outside the build) are left out and counted in left_out; all=true returns them too. "+
			"A method that types get through an embedded field is one row, the declared method, with promoted_to naming those types; asking for it under one of those types returns that row. "+
			"Each node has file, line and end_line. A TypeAlias node is any type that is not a struct or an interface: sub_kind says whether it is an alias (type A = B) or a defined type (type A B). "+
			"total is the number of declarations matched; truncated says that limit cut the list. When nothing matches, other_case lists the names that differ only in case."),
		mcp.WithString("name", mcp.Required()),
		mcp.WithString("language"),
		mcp.WithBoolean("exact", mcp.DefaultBool(true)),
		mcp.WithBoolean("all", mcp.DefaultBool(false), mcp.Description("with exact=false: also return what is declared inside a function body and External stubs")),
		mcp.WithNumber("limit", mcp.DefaultNumber(store.DefaultFindSymbolLimit)),
		mcp.WithBoolean("include_blobs", mcp.DefaultBool(false)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("name", "")
		lang := req.GetString("language", "")
		exact := req.GetBool("exact", true)
		incl := req.GetBool("include_blobs", false)
		limit := int(req.GetFloat("limit", store.DefaultFindSymbolLimit))
		// Every match is read, then ranked and cut here: the store's own cap
		// returned the first 100 rows in no order ("Hash": 16 nodes of that
		// name out of 63 declarations) and could not say that it had cut.
		matches, err := reader.FindSymbol(name, exact, store.FindSymbolOptions{Language: lang, Limit: store.FindSymbolNoLimit})
		if err != nil {
			return nil, err
		}
		// A site name, the form of the candidates of a qualified name that
		// several declarations carry, names one declaration: the resolver
		// every tool shares answers it.
		if _, _, _, site := store.ParseSiteName(name); site && len(matches) == 0 {
			res, err := store.ResolveName(reader, name, lang)
			if err != nil {
				return nil, err
			}
			matches, exact = res.Nodes, true
		}
		kept, leftOut, err := rankNamed(reader, matches, exact || req.GetBool("all", false))
		if err != nil {
			return nil, err
		}
		out := map[string]any{"total": len(kept)}
		if limit > 0 && len(kept) > limit {
			kept = kept[:limit]
			out["truncated"] = true
		}
		rows := attachBlobs(reader, kept, incl)
		// The types a method is promoted to are an attribute of its row: as
		// rows of their own they made total exceed the declarations in the
		// source ("String": 169 rows for 155 declarations on go-stablenet).
		promoted, err := store.PromotedTo(reader, kept)
		if err != nil {
			return nil, err
		}
		for i, n := range kept {
			if to := promoted[n.ID]; len(to) > 0 {
				rows[i]["promoted_to"] = to
			}
		}
		out["nodes"] = rows
		if len(leftOut) > 0 {
			out["left_out"] = leftOut
		}
		if len(matches) == 0 && !exact {
			if res, err := store.ResolveName(reader, name, lang); err == nil && len(res.OtherCase) > 0 {
				out["other_case"] = res.OtherCase
			}
		}
		traceFrom(ctx).add("find_symbol", map[string]any{"input": name, "exact": exact, "nodes": len(kept), "total": out["total"], "left_out": leftOut})
		return textResult(out), nil
	})
}

// nameClassLabels names, in a response, the classes of node a name search
// leaves out by default.
var nameClassLabels = map[store.NameClass]string{
	store.NameLocal:    "local",
	store.NameExternal: "external",
	store.NameOther:    "other",
}

// rankNamed turns the nodes one name matched into the declarations to answer
// with, in order: declarations of the build, then (with all) what is declared
// inside a function body, External stubs and the rest; inside a class, wider
// use in the repository (PageRank) first, then qualified name. Without all,
// the classes after the declarations are left out and counted per class.
//
// The copy of a promoted method is not a declaration: it is replaced by the
// method it copies, once.
func rankNamed(reader store.Reader, nodes []types.Node, all bool) (kept []types.Node, leftOut map[string]int, err error) {
	class, err := store.ClassifyNames(reader, nodes)
	if err != nil {
		return nil, nil, err
	}
	have := map[string]bool{}
	var copies []types.Node
	for _, n := range nodes {
		c := class[n.ID]
		if c == store.NamePromoted {
			copies = append(copies, n)
			continue
		}
		if label, skip := nameClassLabels[c]; skip && !all {
			if leftOut == nil {
				leftOut = map[string]int{}
			}
			leftOut[label]++
			continue
		}
		have[n.ID] = true
		kept = append(kept, n)
	}
	byName := map[string][]types.Node{} // declared methods by qualified name
	for _, c := range copies {
		from, _ := store.PromotedFrom(c)
		declared, looked := byName[from]
		if !looked {
			if declared, err = reader.FindSymbol(from, true, store.FindSymbolOptions{Limit: store.FindSymbolNoLimit}); err != nil {
				return nil, nil, err
			}
			byName[from] = declared
		}
		for _, n := range declared {
			if _, isCopy := store.PromotedFrom(n); !isCopy && !have[n.ID] && store.SiteOf(n) == store.SiteOf(c) {
				have[n.ID] = true
				class[n.ID] = store.NameDeclared
				kept = append(kept, n)
			}
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		a, b := kept[i], kept[j]
		if class[a.ID] != class[b.ID] {
			return class[a.ID] < class[b.ID]
		}
		if a.PageRank != b.PageRank {
			return a.PageRank > b.PageRank
		}
		if a.QualifiedName != b.QualifiedName {
			return a.QualifiedName < b.QualifiedName
		}
		return a.ID < b.ID
	})
	return kept, leftOut, nil
}

// RegisterFindCallers walks the reverse call graph from the seed.
// Filters to calls/invokes edges so the BFS only follows real
// invocation links (see callEdgeTypes for the rationale). Default
// depth=2 — see docs/graph/TRAVERSAL-DEPTH.md for the // latency/recall justification.
func RegisterFindCallers(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("find_callers",
		mcp.WithDescription("Functions that call the symbol (reverse call graph). qname may be a full qualified_name (\"render.NewTemplate\") or a bare short name (\"NewTemplate\") — a bare name is resolved by suffix, and an ambiguous bare name returns its candidate qnames instead of an empty result. Follows calls, interface calls and the implementations they can run (dispatches_to); for a Solidity modifier, event or error, the functions that apply, emit or revert with it; for a field, package-level variable, constant or Solidity state variable it returns the code that reads or writes it (as find_references). Default depth=2 — see docs/graph/TRAVERSAL-DEPTH.md for the latency/recall justification."),
		mcp.WithString("qname", mcp.Required()),
		mcp.WithNumber("depth", mcp.DefaultNumber(2)),
		mcp.WithBoolean("include_blobs", mcp.DefaultBool(false)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("qname", "")
		d := traversalDepth(req)
		incl := req.GetBool("include_blobs", false)
		resolved, cands, ambiguous, ok := resolveSeed(reader, q, "")
		traceSeed(traceFrom(ctx), reader, q, resolved, cands, ambiguous, ok)
		if miss := seedMiss(q, resolved, cands, ambiguous, ok); miss != nil {
			return textResult(miss), nil
		}
		// A field, variable or constant has no callers; its uses are value
		// edges. Answer with them instead of an empty call graph that looks
		// like "nobody uses it".
		if own, err := resolved.nodes(reader); err == nil && allValueNodes(own) {
			out, err := valueUsers(reader, resolved, "all", defaultReferenceLimit)
			if err != nil {
				return nil, err
			}
			out["note"] = "this symbol is a " + joinKinds(nodeKinds(own))[2:] + ": calls are recorded for functions and methods only. " +
				"users are the code that reads or writes it (the same as find_references)."
			out["nodes"] = []map[string]any{}
			out["edges"] = []types.Edge{}
			return textResult(resolved.result(out)), nil
		}
		// Union the concrete seed with the interface methods it satisfies, so
		// interface-dispatched callers (recorded as `invokes` to the interface
		// method, not the concrete method) are not missed. See reverseCallersUnion.
		nodes, edges, err := reverseCallersUnionTraced(reader, resolved, d, traceFrom(ctx))
		if err != nil {
			return nil, err
		}
		traceWalk(traceFrom(ctx), d, nodes, edges)
		return textResult(resolved.result(map[string]any{
			"nodes": attachBlobs(reader, nodes, incl),
			"edges": edges,
		})), nil
	})
}

// RegisterFindCallees walks the forward call graph from the seed.
// Same edge-type filter as RegisterFindCallers for symmetry.
func RegisterFindCallees(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("find_callees",
		mcp.WithDescription("Functions called by the symbol (forward call graph). qname may be a full qualified_name or a bare short name (resolved by suffix; an ambiguous bare name returns candidate qnames instead of an empty result). Follows calls, interface calls and the implementations they can run (dispatches_to, not counted as a hop); for a Solidity function also the modifiers it applies, the events it emits and the errors it reverts with. Default depth=2 — see docs/graph/TRAVERSAL-DEPTH.md for the latency/recall justification."),
		mcp.WithString("qname", mcp.Required()),
		mcp.WithNumber("depth", mcp.DefaultNumber(2)),
		mcp.WithBoolean("include_blobs", mcp.DefaultBool(false)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("qname", "")
		d := traversalDepth(req)
		incl := req.GetBool("include_blobs", false)
		resolved, cands, ambiguous, ok := resolveSeed(reader, q, "")
		traceSeed(traceFrom(ctx), reader, q, resolved, cands, ambiguous, ok)
		if miss := seedMiss(q, resolved, cands, ambiguous, ok); miss != nil {
			return textResult(miss), nil
		}
		nodes, edges, err := resolved.neighborhood(reader, d, false, callEdgeTypes...)
		if err != nil {
			return nil, err
		}
		nodes, edges, added := withDispatchTargets(reader, nodes, edges)
		traceFrom(ctx).add("dispatch_targets", map[string]any{"added": added})
		traceWalk(traceFrom(ctx), d, nodes, edges)
		return textResult(resolved.result(map[string]any{
			"nodes": attachBlobs(reader, nodes, incl),
			"edges": edges,
		})), nil
	})
}

// RegisterGetSubgraph returns the BFS bidirectional subgraph rooted
// at qname. Unlike find_callers / find_callees this DOES follow every
// edge type — the caller asked for a neighbourhood, not a call graph.
func RegisterGetSubgraph(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("get_subgraph",
		mcp.WithDescription("Subgraph rooted at qname, expanded by depth (both directions). seed_qname may be a full qualified_name or a bare short name (resolved by suffix; an ambiguous bare name returns candidate qnames instead of an empty result)."),
		mcp.WithString("seed_qname", mcp.Required()),
		mcp.WithNumber("depth", mcp.DefaultNumber(2)),
		mcp.WithBoolean("include_blobs", mcp.DefaultBool(false)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("seed_qname", "")
		d := traversalDepth(req)
		incl := req.GetBool("include_blobs", false)
		resolved, cands, ambiguous, ok := resolveSeed(reader, q, "")
		traceSeed(traceFrom(ctx), reader, q, resolved, cands, ambiguous, ok)
		if miss := seedMiss(q, resolved, cands, ambiguous, ok); miss != nil {
			return textResult(miss), nil
		}
		nodes, edges, err := resolved.subgraph(reader, d)
		if err != nil {
			return nil, err
		}
		traceWalk(traceFrom(ctx), d, nodes, edges)
		return textResult(resolved.result(map[string]any{
			"nodes": attachBlobs(reader, nodes, incl),
			"edges": edges,
		})), nil
	})
}

// RegisterSearchText runs the smart Search router (FTS5 with
// auto-prefix for ASCII, LIKE substring fallback for CJK). Routes
// through attachBlobs so the response shape matches
// find_symbol / find_callers / get_subgraph — LLM clients can parse
// one schema across the toolbox.
//
// mode = "or" (default) ORs multi-token queries via rewriteFTSQuery
// (any token match surfaces the candidate, BM25 + PageRank + usage
// rerank). mode = "and" requires every token to appear in the hit's
// FTS-indexed columns; useful for precise multi-keyword retrieval.
// language pushes a `WHERE language = ?` filter into the SQL when
// non-empty.
//
// node_kinds is the optional whitelist of node types to return. The
// default (omitted / empty) applies a symbol-only filter that strips
// statement-level rows (IfStmt/LoopStmt/CallSite/ReturnStmt/SwitchStmt/
// AwaitPoint), meta rows (Commit/Hunk), and path-only rows
// (Import/Export). Pass an explicit array — typically the full set
// produced by types.AllNodeTypes — to disable the default narrowing
// and surface every FTS match.
func RegisterSearchText(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("search_text",
		mcp.WithDescription("Find symbols by words of their name: full-text search over name + qualified_name + signature + doc_comment (not function bodies, string literals or comments inside bodies; for those use search_code). A declaration whose name is exactly the query (or one of the names joined by |) comes first; exact_names counts them and truncated says that top_k cut them. A list of names joined by | is ORed; any other regular expression is run by search_code. Auto-prefix on short ASCII queries; substring fallback on CJK input. mode=\"or\" (default) ORs multi-token queries; mode=\"and\" requires every token to appear in each hit's indexed columns. language filters the result set to a single source language (go|ts|sol). node_kinds is an optional whitelist of node types; omit it to apply the default symbol-only filter (statement / Commit / Hunk / Import / Export rows are stripped because their FTS hits are typically noise from the enclosing symbol's qname prefix; whatever is declared inside a function body or inside a function literal of a package-level variable's initialiser, and copies of promoted methods, are left out too)."),
		mcp.WithString("query", mcp.Required()),
		mcp.WithNumber("top_k", mcp.DefaultNumber(10)),
		mcp.WithString("language"),
		mcp.WithString("mode"),
		mcp.WithArray("node_kinds"),
		mcp.WithBoolean("include_blobs", mcp.DefaultBool(false)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("query", "")
		top := int(req.GetFloat("top_k", 10))
		incl := req.GetBool("include_blobs", false)
		lang := req.GetString("language", "")
		mode := req.GetString("mode", "")
		opts := store.SearchFTSOptions{
			Language: lang,
			Mode:     mode,
		}
		raw := req.GetStringSlice("node_kinds", nil)
		if raw != nil {
			opts.NodeKinds = make([]types.NodeType, 0, len(raw))
			for _, k := range raw {
				opts.NodeKinds = append(opts.NodeKinds, types.NodeType(k))
			}
		}
		// grep-style input: a list of names joined by | becomes an OR of
		// prefixes; any other regular expression is answered by search_code
		// over the same graph, so the call does not fail on FTS syntax.
		t := traceFrom(ctx)
		names := queryNames(q)
		if alt := wordAlternatives(q); alt != "" {
			q = alt
		} else if looksLikeRegex(q) {
			t.add("search_text.route", map[string]any{"route": "search_code", "reason": "regular expression"})
			return textResult(searchTextByPattern(t, reader, q, top)), nil
		}
		// Without node_kinds only declarations are answered; with it the
		// caller chose the kinds and gets every node of them.
		declaredOnly := raw == nil
		exact, err := namedExactly(reader, names, opts, declaredOnly)
		if err != nil {
			return nil, err
		}
		hits, err := searchTextHits(reader, q, top, opts, declaredOnly)
		t.add("search_text.route", map[string]any{"route": "fts", "fts_query": q, "top_k": top,
			"exact_names": len(exact), "hits": len(hits), "error": fmt.Sprint(err)})
		if err != nil {
			out := searchTextByPattern(t, reader, q, top)
			out["note"] = "the query is not valid full-text syntax (" + err.Error() + "); matched as a literal by search_code instead"
			return textResult(out), nil
		}
		out := map[string]any{}
		if len(exact) > 0 {
			out["exact_names"] = len(exact)
			if len(exact) > top {
				out["truncated"] = true
			}
		}
		out["nodes"] = attachBlobs(reader, firstNodes(top, exact, hits), incl)
		return textResult(out), nil
	})
}

// queryName is a query that is one symbol name, bare or qualified.
var queryName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// queryNames returns the symbol names a search_text query asks for by name:
// the query itself when it is one name, the names of an A|B|C list, or nil
// for words and patterns.
func queryNames(q string) []string {
	q = strings.TrimSpace(q)
	if queryName.MatchString(q) {
		return []string{q}
	}
	if !wordAlternation.MatchString(q) {
		return nil
	}
	parts := strings.Split(q, "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// namedExactly returns the nodes whose name is exactly one of names, in the
// order rankNamed gives them. They go before the full-text hits: BM25 scores
// a name by the words of its qualified name, signature and doc comment, so
// the declaration named by the query was not always in the first top_k (14
// of 20 on go-stablenet at the default top_k of 10).
func namedExactly(reader store.Reader, names []string, opts store.SearchFTSOptions, declaredOnly bool) ([]types.Node, error) {
	kinds := opts.NodeKinds
	if len(kinds) == 0 {
		kinds = types.SymbolNodeTypes()
	}
	var out []types.Node
	seen := map[string]bool{}
	for _, name := range names {
		nodes, err := reader.FindSymbol(name, false, store.FindSymbolOptions{Language: opts.Language, Kinds: kinds, Limit: store.FindSymbolNoLimit})
		if err != nil {
			return nil, err
		}
		// With node_kinds the caller chose the kinds and gets every node of
		// them; without, only declarations (rankNamed leaves the rest out).
		ranked, _, err := rankNamed(reader, nodes, !declaredOnly)
		if err != nil {
			return nil, err
		}
		for _, n := range ranked {
			if !seen[n.ID] {
				seen[n.ID] = true
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// searchTextOverfetch is how many times top_k the full-text search reads
// when nodes that are not declarations are dropped afterwards, and
// searchTextRounds how often it widens that when too few are left. About 9%
// of go-stablenet's hits were function-local names, but a query such as "err"
// matches hundreds of them before the first declaration.
const (
	searchTextOverfetch = 3
	searchTextRounds    = 3
)

// searchTextHits runs the full-text search and, with declaredOnly, drops the
// nodes that are not declarations of the build: whatever is declared inside a
// function body (a local variable, a field of a local type, a goroutine or
// channel) is named after its function and matched every query for that
// function's name (436 of 7,247 rows on go-stablenet after local variables
// alone were dropped).
func searchTextHits(reader store.Reader, q string, top int, opts store.SearchFTSOptions, declaredOnly bool) ([]types.Node, error) {
	if !declaredOnly {
		return reader.SearchWithOpts(q, top, opts)
	}
	fetch := top
	var kept []types.Node
	for round := 0; round < searchTextRounds; round++ {
		fetch *= searchTextOverfetch
		hits, err := reader.SearchWithOpts(q, fetch, opts)
		if err != nil {
			return nil, err
		}
		class, err := store.ClassifyNames(reader, hits)
		if err != nil {
			return nil, err
		}
		kept = kept[:0]
		for _, n := range hits {
			if class[n.ID] == store.NameDeclared {
				kept = append(kept, n)
			}
		}
		if len(kept) >= top || len(hits) < fetch {
			break
		}
	}
	return kept, nil
}

// firstNodes returns the first top nodes of the lists in order, each node
// once.
func firstNodes(top int, lists ...[]types.Node) []types.Node {
	out := make([]types.Node, 0, top)
	seen := map[string]bool{}
	for _, list := range lists {
		for _, n := range list {
			if len(out) >= top {
				return out
			}
			if !seen[n.ID] {
				seen[n.ID] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// traversalDepth is the caller's depth (default 2) clamped to
// [0, impact.DepthCap]: an untyped walk on a real graph reaches most of it
// by depth 4.
func traversalDepth(req mcp.CallToolRequest) int {
	return min(max(int(req.GetFloat("depth", 2)), 0), impact.DepthCap)
}
