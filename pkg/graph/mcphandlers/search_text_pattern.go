package mcphandlers

import (
	"regexp"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

var wordAlternation = regexp.MustCompile(`^\s*[A-Za-z_][A-Za-z0-9_]*(\s*\|\s*[A-Za-z_][A-Za-z0-9_]*)+\s*$`)

// wordAlternatives turns "A|B|C" into the FTS query "A* OR B* OR C*", or
// returns "" when q is not a plain list of names.
func wordAlternatives(q string) string {
	if !wordAlternation.MatchString(q) {
		return ""
	}
	parts := strings.Split(q, "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p) + "*"
	}
	return strings.Join(parts, " OR ")
}

// looksLikeRegex reports grep-style syntax that full-text search cannot parse.
func looksLikeRegex(q string) bool {
	return strings.ContainsAny(q, `|\[]^$+?{}`) || strings.Contains(q, ".*") || strings.Contains(q, "(.")
}

// searchTextByPattern answers a pattern sent to search_text with search_code's
// hits, as nodes, and says so.
func searchTextByPattern(t *callTrace, reader store.Reader, q string, top int) map[string]any {
	out := map[string]any{"nodes": []map[string]any{},
		"note": "search_text matches words of symbol names; this query is a regular expression, so it was run by search_code (use search_code directly for matched lines, resolve and expand)"}
	idx, err := codeIndexFor(reader)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	res, err := searchCode(reader, idx, searchCodeArgs{pattern: q, in: "both", tests: "include", limit: max(top, 1), linesPerHit: searchCodeLinesPerHit, resolve: false, trace: t})
	if err != nil {
		res, err = searchCode(reader, idx, searchCodeArgs{pattern: q, fixed: true, in: "both", tests: "include", limit: max(top, 1), linesPerHit: searchCodeLinesPerHit, trace: t})
		if err != nil {
			out["error"] = err.Error()
			return out
		}
	}
	var nodes []map[string]any
	for _, h := range res["hits"].([]map[string]any) {
		n := map[string]any{"qname": h["qname"], "type": h["type"], "file": h["file"], "line": h["start_line"], "matched": h["matched"]}
		if c, ok := h["canonical_id"]; ok {
			n["canonical_id"] = c
		}
		nodes = append(nodes, n)
	}
	if nodes != nil {
		out["nodes"] = nodes
	}
	out["total_declarations"] = res["total_declarations"]
	return out
}
