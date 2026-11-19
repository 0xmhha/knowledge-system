package mcphandlers

import (
	"context"
	"sort"
	"strings"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// valueEdgeTypes are the edges from a user to a value it reads or writes:
// struct fields (reads_field / writes_field) and package-level variables and
// constants (references, dispatch_kind read or write).
var valueEdgeTypes = []string{string(types.EdgeReadsField), string(types.EdgeWritesField), string(types.EdgeReferences)}

const defaultReferenceLimit = 100

// isValueNode reports whether uses of nodes of type t are recorded as value
// edges, not as calls.
func isValueNode(t types.NodeType) bool {
	return t == types.NodeField || t == types.NodeVariable || t == types.NodeConstant
}

// RegisterFindReferences lists the code that reads or writes a field, a
// package-level variable or a constant.
func RegisterFindReferences(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader)
	tool := nsTool("find_references",
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDescription("Who reads or writes a value: the functions (and package-level initializers) that use a struct field, a package-level variable or a constant, "+
			"each with the file, every line it uses the value on (lines; line is the first), the number of uses and access=read|write. Writers come first, production code before tests (test=true). "+
			"Use it to follow a value to where it is set and where it is consumed; calls are in find_callers. "+
			"qname may be a canonical id, a qualified name (\"render.Options.Width\", \"log.DefaultLevel\") or a bare name. "+
			"Recorded for Go; other languages and local variables have no value edges."),
		mcp.WithString("qname", mcp.Required()),
		mcp.WithString("access", mcp.Description("read, write or all (default)")),
		mcp.WithNumber("limit", mcp.DefaultNumber(defaultReferenceLimit)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("qname", "")
		resolved, cands, ambiguous, ok := resolveSeed(reader, q, "")
		traceSeed(traceFrom(ctx), reader, q, resolved, cands, ambiguous, ok)
		if ambiguous {
			return textResult(seedAmbiguousResult(q, cands)), nil
		}
		if !ok {
			return textResult(seedNotFoundResult(q)), nil
		}
		out, err := valueUsers(reader, resolved, req.GetString("access", "all"), int(req.GetFloat("limit", defaultReferenceLimit)))
		if err != nil {
			return nil, err
		}
		return textResult(resolved.result(out)), nil
	})
}

// valueUsers returns the users of the seed's value nodes as a payload:
// "users" (one row per user and access kind), "total", "truncated" and, when
// the seed is not a value, a "note" saying where its uses are recorded.
func valueUsers(reader store.Reader, seed seedRef, access string, limit int) (map[string]any, error) {
	own, err := seed.nodes(reader)
	if err != nil {
		return nil, err
	}
	values := map[string]types.Node{}
	var kinds []string
	for _, n := range own {
		if isValueNode(n.Type) {
			values[n.ID] = n
		} else {
			kinds = append(kinds, string(n.Type))
		}
	}
	out := map[string]any{"users": []map[string]any{}, "total": 0}
	if len(values) == 0 {
		out["note"] = "value uses are recorded for fields, package-level variables and constants; this symbol is " +
			joinKinds(kinds) + ". Use find_callers for the callers of a function or method."
		return out, nil
	}
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	nodes, edges, err := reader.NeighborhoodByIDs(ids, 1, true, valueEdgeTypes...)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]types.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	// One edge per use line; one row per (user, value, access) with all its
	// lines, the first as "line" and the sum of uses as "uses".
	type rowKey struct{ src, dst, kind string }
	rows := map[rowKey]map[string]any{}
	var users []map[string]any
	for _, e := range edges {
		if _, ok := values[e.Dst]; !ok {
			continue
		}
		kind := "read"
		switch {
		case e.Type == types.EdgeWritesField, e.Type == types.EdgeReferences && e.DispatchKind == "write":
			kind = "write"
		}
		if access == "read" && kind != "read" || access == "write" && kind != "write" {
			continue
		}
		src, ok := byID[e.Src]
		if !ok {
			continue
		}
		line := e.Line
		if line == 0 {
			line = src.StartLine
		}
		k := rowKey{e.Src, e.Dst, kind}
		if row, ok := rows[k]; ok {
			row["uses"] = row["uses"].(int) + e.Count
			row["lines"] = append(row["lines"].([]int), line)
			continue
		}
		file := e.FilePath
		if file == "" {
			file = src.FilePath
		}
		row := map[string]any{"qname": src.QualifiedName, "type": string(src.Type), "file": file,
			"access": kind, "uses": e.Count, "lines": []int{line}, "value": values[e.Dst].QualifiedName}
		if src.CanonicalID != "" {
			row["canonical_id"] = src.CanonicalID
		}
		if src.Signature != "" {
			row["signature"] = src.Signature
		}
		if strings.HasSuffix(file, "_test.go") {
			row["test"] = true
		}
		rows[k] = row
		users = append(users, row)
	}
	for _, row := range users {
		ls := row["lines"].([]int)
		sort.Ints(ls)
		row["line"] = ls[0]
	}
	sort.SliceStable(users, func(i, j int) bool {
		a, b := users[i], users[j]
		if a["access"] != b["access"] {
			return a["access"] == "write"
		}
		if ta, tb := a["test"] != nil, b["test"] != nil; ta != tb {
			return tb // production code before tests
		}
		if a["file"] != b["file"] {
			return a["file"].(string) < b["file"].(string)
		}
		return a["line"].(int) < b["line"].(int)
	})
	out["total"] = len(users)
	if limit > 0 && len(users) > limit {
		users = users[:limit]
		out["truncated"] = true
	}
	out["users"] = users
	if len(users) == 0 {
		out["note"] = "no recorded reads or writes of this value"
	}
	return out, nil
}

func joinKinds(kinds []string) string {
	if len(kinds) == 0 {
		return "not a value"
	}
	sort.Strings(kinds)
	s := "a " + kinds[0]
	for i := 1; i < len(kinds); i++ {
		if kinds[i] != kinds[i-1] {
			s += " / " + kinds[i]
		}
	}
	return s
}

func allValueNodes(ns []types.Node) bool {
	for _, n := range ns {
		if !isValueNode(n.Type) {
			return false
		}
	}
	return len(ns) > 0
}

func nodeKinds(ns []types.Node) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, string(n.Type))
	}
	return out
}
