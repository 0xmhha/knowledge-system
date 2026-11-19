package mcphandlers

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// A trace records what a tool did inside one call, so a wrong or missing
// answer can be traced to the step that lost it: how the input was resolved,
// which declarations matched and in what order, which of them the result
// kept, and which matched lines, calls and value uses it left out.
//
// The call log (LogToolCalls) says that a call happened; the trace says what
// the graph held for it and what the agent was shown. It is opt-in
// (ckg mcp --trace <file>, or CKG_MCP_TRACE) because a record carries the
// full result and every ranked candidate.
//
// One JSON object per line:
//
//	{"seq":1,"time":"...","tool":"search_code","arguments":{...},"seconds":0.01,
//	 "events":[{"stage":"search_code.rank","data":{...}}, ...],
//	 "result_bytes":10907,"result":{...}}

// traceRankCap bounds the ranked candidates kept per call; a pattern such as
// "err" matches thousands of declarations.
const traceRankCap = 300

type traceKey struct{}

// callTrace collects the events of one tool call. A nil *callTrace is a
// valid, disabled trace: every method is a no-op, so handlers call it
// unconditionally.
type callTrace struct {
	mu     sync.Mutex
	events []traceEvent
}

type traceEvent struct {
	Stage string         `json:"stage"`
	Data  map[string]any `json:"data"`
}

func (t *callTrace) add(stage string, data map[string]any) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.events = append(t.events, traceEvent{Stage: stage, Data: data})
	t.mu.Unlock()
}

// on reports whether tracing is enabled, so a handler can skip work that
// only the trace needs.
func (t *callTrace) on() bool { return t != nil }

func traceFrom(ctx context.Context) *callTrace {
	t, _ := ctx.Value(traceKey{}).(*callTrace)
	return t
}

// TraceToolCalls returns a middleware that writes one trace record per tool
// call to w. Writes are serialized; a write error is dropped so a full disk
// never fails a tool call.
//
// Usage:
//
//	f, _ := os.Create("trace.jsonl")
//	s := server.NewMCPServer(name, version, server.WithToolHandlerMiddleware(mcphandlers.TraceToolCalls(f)))
func TraceToolCalls(w io.Writer) server.ToolHandlerMiddleware {
	var mu sync.Mutex
	var seq atomic.Uint64
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			t := &callTrace{}
			start := time.Now()
			res, err := next(context.WithValue(ctx, traceKey{}, t), req)
			rec := map[string]any{
				"seq": seq.Add(1), "time": start.UTC().Format(time.RFC3339Nano), "tool": req.Params.Name,
				"arguments": req.GetArguments(), "seconds": time.Since(start).Seconds(),
				"events": t.events, "result_bytes": resultBytes(res),
			}
			if err != nil {
				rec["error"] = err.Error()
			}
			if res != nil {
				if res.StructuredContent != nil {
					rec["result"] = res.StructuredContent
				} else {
					rec["result"] = res.Content
				}
			}
			mu.Lock()
			_ = enc.Encode(rec)
			mu.Unlock()
			return res, err
		}
	}
}

// topLevelAlternatives splits a pattern on the | that are not inside
// parentheses or brackets, the alternatives search_code counts as "matched".
func topLevelAlternatives(p string) []string {
	var out []string
	depth, class, start := 0, false, 0
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '\\':
			i++
		case class:
			class = c != ']'
		case c == '[':
			class = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == '|' && depth == 0:
			out = append(out, p[start:i])
			start = i + 1
		}
	}
	return append(out, p[start:])
}

// traceRanking lists every matched declaration in rank order (up to
// traceRankCap), with the keys that ranked it and whether the limit cut it.
func traceRanking(idx *codeIndex, ranked []*codeHit, limit int) map[string]any {
	rows := make([]map[string]any, 0, min(len(ranked), traceRankCap))
	for i, h := range ranked {
		if i == traceRankCap {
			break
		}
		d := idx.decls[h.decl]
		terms := make([]string, 0, len(h.terms))
		for t := range h.terms {
			terms = append(terms, t)
		}
		rows = append(rows, map[string]any{"rank": i + 1, "qname": d.node.QualifiedName, "file": d.node.FilePath,
			"start_line": d.node.StartLine, "end_line": d.node.EndLine, "terms": len(terms), "matched": terms,
			"in_name": h.inName, "matches": h.matches, "matched_lines": h.lines, "test": d.test,
			"pagerank": d.node.PageRank, "returned": i < limit})
	}
	return map[string]any{"ranked_total": len(ranked), "cut_by_limit": max(0, len(ranked)-limit),
		"ranked_kept_in_trace": len(rows), "ranked": rows}
}

// traceHitView says what one returned hit showed and what it left out: the
// matched lines past the per-hit cap, the calls and value uses on them, and
// the calls and value uses on lines of the declaration that did not match.
func traceHitView(d codeDecl, h *codeHit, a searchCodeArgs, uses map[int][]string) map[string]any {
	shown := map[int]bool{}
	for k, ln := range h.lines {
		if a.linesPerHit > 0 && k == a.linesPerHit {
			break
		}
		for l := ln - a.context; l <= ln+a.context; l++ {
			shown[l] = true
		}
	}
	var hidden []int
	for _, ln := range h.lines {
		if !shown[ln] {
			hidden = append(hidden, ln)
		}
	}
	matched := map[int]bool{}
	for _, ln := range h.lines {
		matched[ln] = true
	}
	usesShown, usesHiddenMatched, usesUnmatched := map[int][]string{}, map[int][]string{}, map[int][]string{}
	for ln, u := range uses {
		switch {
		case shown[ln] && a.resolve:
			usesShown[ln] = u
		case matched[ln]:
			usesHiddenMatched[ln] = u
		default:
			usesUnmatched[ln] = u
		}
	}
	return map[string]any{"qname": d.node.QualifiedName, "file": d.node.FilePath, "start_line": d.node.StartLine,
		"end_line": d.node.EndLine, "matched_lines": h.lines, "hidden_matched_lines": hidden,
		"uses_shown": usesShown, "uses_on_hidden_matched_lines": usesHiddenMatched,
		"uses_on_unmatched_lines": usesUnmatched}
}

// collapsedUses lists the edges from the given declarations that stand for
// several uses (count > 1). Since schema 1.30 a value edge is per line, so
// such an edge is several uses on one line; on older graphs it also stood
// for uses on later lines, which were shown at the first line or not at all.
func collapsedUses(reader store.Reader, ids []string) []map[string]any {
	if len(ids) == 0 {
		return nil
	}
	edges, err := reader.QueryEdgesForNodes(ids)
	if err != nil {
		return nil
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var dsts []string
	for _, e := range edges {
		if want[e.Src] && e.Count > 1 {
			dsts = append(dsts, e.Dst)
		}
	}
	targets, err := reader.NodesByIDs(dsts)
	if err != nil {
		return nil
	}
	name := map[string]string{}
	for _, n := range targets {
		name[n.ID] = n.QualifiedName
	}
	var out []map[string]any
	for _, e := range edges {
		if want[e.Src] && e.Count > 1 {
			out = append(out, map[string]any{"src": e.Src, "type": string(e.Type), "dst": name[e.Dst],
				"first_line": e.Line, "count": e.Count})
		}
	}
	return out
}

// traceSeed records how a seed name was resolved: by canonical id, exact
// qualified name, suffix, or not at all. It repeats the lookups of
// resolveSeed, so it runs only when the call is traced.
func traceSeed(t *callTrace, reader store.Reader, seed string, resolved seedRef, cands []string, ambiguous, ok bool) {
	if !t.on() {
		return
	}
	step := "not_found"
	switch {
	case ambiguous:
		step = "ambiguous_suffix"
	case ok:
		step = "suffix"
		if _, found, err := reader.FindByCanonicalID(seed); err == nil && found {
			step = "canonical_id"
		} else if ns, err := reader.FindSymbol(seed, true, store.FindSymbolOptions{}); err == nil && len(ns) > 0 {
			step = "exact_qname"
		}
	}
	t.add("seed", map[string]any{"input": seed, "step": step, "resolved_qname": resolved.QName,
		"resolved_ids": resolved.IDs, "candidates": len(cands)})
}

// traceWalk records the shape of a traversal result: nodes and edges by
// type, and edges that carry no line.
func traceWalk(t *callTrace, depth int, nodes []types.Node, edges []types.Edge) {
	if !t.on() {
		return
	}
	nt, et, noLine := map[string]int{}, map[string]int{}, 0
	for _, n := range nodes {
		nt[string(n.Type)]++
	}
	for _, e := range edges {
		et[string(e.Type)]++
		if e.Line == 0 {
			noLine++
		}
	}
	t.add("walk", map[string]any{"depth": depth, "nodes": len(nodes), "edges": len(edges),
		"node_types": nt, "edge_types": et, "edges_without_line": noLine})
}
