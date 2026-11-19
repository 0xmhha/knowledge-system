package mcphandlers

import (
	"context"
	"fmt"

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
		mcp.WithDescription("Find symbols by qualified_name. With exact=true (default), the input must match qualified_name exactly (e.g. \"render.NewTemplate\"). With exact=false, the input is treated as a suffix — a bare symbol name (\"NewTemplate\") matches every qualified_name ending in that segment. Use exact=false when you only know the symbol's short name."),
		mcp.WithString("name", mcp.Required()),
		mcp.WithString("language"),
		mcp.WithBoolean("exact", mcp.DefaultBool(true)),
		mcp.WithBoolean("include_blobs", mcp.DefaultBool(false)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("name", "")
		lang := req.GetString("language", "")
		exact := req.GetBool("exact", true)
		incl := req.GetBool("include_blobs", false)
		out, err := reader.FindSymbol(name, exact, store.FindSymbolOptions{Language: lang})
		if err != nil {
			return nil, err
		}
		traceFrom(ctx).add("find_symbol", map[string]any{"input": name, "exact": exact, "nodes": len(out)})
		return textResult(map[string]any{"nodes": attachBlobs(reader, out, incl)}), nil
	})
}

// RegisterFindCallers walks the reverse call graph from the seed.
// Filters to calls/invokes edges so the BFS only follows real
// invocation links (see callEdgeTypes for the rationale). Default
// depth=2 — see docs/graph/TRAVERSAL-DEPTH.md for the // latency/recall justification.
func RegisterFindCallers(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("find_callers",
		mcp.WithDescription("Functions that call the symbol (reverse call graph). qname may be a full qualified_name (\"render.NewTemplate\") or a bare short name (\"NewTemplate\") — a bare name is resolved by suffix, and an ambiguous bare name returns its candidate qnames instead of an empty result. Follows calls, interface calls and the implementations they can run (dispatches_to); for a field, package-level variable or constant it returns the code that reads or writes it (as find_references). Default depth=2 — see docs/graph/TRAVERSAL-DEPTH.md for the latency/recall justification."),
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
		if ambiguous {
			return textResult(seedAmbiguousResult(q, cands)), nil
		}
		if !ok {
			return textResult(seedNotFoundResult(q)), nil
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
		mcp.WithDescription("Functions called by the symbol (forward call graph). qname may be a full qualified_name or a bare short name (resolved by suffix; an ambiguous bare name returns candidate qnames instead of an empty result). Follows calls, interface calls and the implementations they can run (dispatches_to, not counted as a hop). Default depth=2 — see docs/graph/TRAVERSAL-DEPTH.md for the latency/recall justification."),
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
		if ambiguous {
			return textResult(seedAmbiguousResult(q, cands)), nil
		}
		if !ok {
			return textResult(seedNotFoundResult(q)), nil
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
		if ambiguous {
			return textResult(seedAmbiguousResult(q, cands)), nil
		}
		if !ok {
			return textResult(seedNotFoundResult(q)), nil
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
		mcp.WithDescription("Find symbols by words of their name: full-text search over name + qualified_name + signature + doc_comment (not function bodies, string literals or comments inside bodies; for those use search_code). A list of names joined by | is ORed; any other regular expression is run by search_code. Auto-prefix on short ASCII queries; substring fallback on CJK input. mode=\"or\" (default) ORs multi-token queries; mode=\"and\" requires every token to appear in each hit's indexed columns. language filters the result set to a single source language (go|ts|sol). node_kinds is an optional whitelist of node types; omit it to apply the default symbol-only filter (statement / Commit / Hunk / Import / Export rows are stripped because their FTS hits are typically noise from the enclosing symbol's qname prefix)."),
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
		if raw := req.GetStringSlice("node_kinds", nil); raw != nil {
			opts.NodeKinds = make([]types.NodeType, 0, len(raw))
			for _, k := range raw {
				opts.NodeKinds = append(opts.NodeKinds, types.NodeType(k))
			}
		}
		// grep-style input: a list of names joined by | becomes an OR of
		// prefixes; any other regular expression is answered by search_code
		// over the same graph, so the call does not fail on FTS syntax.
		t := traceFrom(ctx)
		if alt := wordAlternatives(q); alt != "" {
			q = alt
		} else if looksLikeRegex(q) {
			t.add("search_text.route", map[string]any{"route": "search_code", "reason": "regular expression"})
			return textResult(searchTextByPattern(t, reader, q, top)), nil
		}
		hits, err := reader.SearchWithOpts(q, top, opts)
		t.add("search_text.route", map[string]any{"route": "fts", "fts_query": q, "top_k": top,
			"hits": len(hits), "error": fmt.Sprint(err)})
		if err != nil {
			out := searchTextByPattern(t, reader, q, top)
			out["note"] = "the query is not valid full-text syntax (" + err.Error() + "); matched as a literal by search_code instead"
			return textResult(out), nil
		}
		return textResult(map[string]any{"nodes": attachBlobs(reader, hits, incl)}), nil
	})
}

// traversalDepth is the caller's depth (default 2) clamped to
// [0, impact.DepthCap]: an untyped walk on a real graph reaches most of it
// by depth 4.
func traversalDepth(req mcp.CallToolRequest) int {
	return min(max(int(req.GetFloat("depth", 2)), 0), impact.DepthCap)
}
