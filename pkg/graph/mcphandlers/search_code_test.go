package mcphandlers

import (
	"strings"
	"testing"
)

func hitNames(out map[string]any) []string {
	var names []string
	rows, _ := out["hits"].([]any)
	for _, r := range rows {
		names = append(names, r.(map[string]any)["qname"].(string))
	}
	return names
}

func hitByName(out map[string]any, qname string) map[string]any {
	rows, _ := out["hits"].([]any)
	for _, r := range rows {
		if m := r.(map[string]any); m["qname"] == qname {
			return m
		}
	}
	return nil
}

// TestSearchCode_Alternatives: a grep-style alternation answers per
// declaration, records which alternatives each matched, orders by matches
// (not by how many alternatives matched: user decision 2026-11-19) and
// counts everything it found.
func TestSearchCode_Alternatives(t *testing.T) {
	store := newFixtureStore(t)
	out := callTool(t, store, "search_code", map[string]any{"pattern": `current|Limit`})
	names := hitNames(out)
	rc := hitByName(out, "coll1.readCurrent")
	if rc == nil || len(asSlice(rc["matched"])) != 2 {
		t.Fatalf("readCurrent must be found with both alternatives in matched: %v", rc)
	}
	prev := 1 << 30
	for _, r := range out["hits"].([]any) {
		h := r.(map[string]any)
		if m := int(h["matches"].(float64)); h["name_match"] == nil && m > prev {
			t.Errorf("hits not in order of matches: %v", names)
		} else if h["name_match"] == nil {
			prev = m
		}
	}
	for _, want := range []string{"coll1.setCurrent", "coll1.incCurrent", "coll1.Registry"} {
		if hitByName(out, want) == nil {
			t.Errorf("missing %s in %v", want, names)
		}
	}
	if out["total_declarations"].(float64) < float64(len(names)) || out["total_matches"].(float64) == 0 {
		t.Errorf("totals missing: %v", out)
	}
	if m := rc["matched"].([]any); len(m) != 2 {
		t.Errorf("readCurrent matched = %v, want both alternatives", m)
	}
}

// TestSearchCode_ResolveAndExpand: each matched line says what the graph
// resolved on it, and expand attaches the next hop.
func TestSearchCode_ResolveAndExpand(t *testing.T) {
	store := newFixtureStore(t)
	out := callTool(t, store, "search_code", map[string]any{"pattern": `current = n`})
	h := hitByName(out, "coll1.setCurrent")
	if h == nil {
		t.Fatalf("setCurrent not found: %v", hitNames(out))
	}
	line := h["lines"].([]any)[0].(map[string]any)
	if uses, _ := line["uses"].([]any); len(uses) == 0 || !strings.Contains(uses[0].(string), "writes coll1.current") {
		t.Errorf("line uses = %v, want writes coll1.current", line["uses"])
	}

	exp := callTool(t, store, "search_code", map[string]any{"pattern": `func Greet`, "expand": []any{"callers"}})
	g := hitByName(exp, "a.Greet")
	if g == nil {
		t.Fatalf("a.Greet not found: %v", hitNames(exp))
	}
	callers, _ := g["callers"].([]any)
	if len(callers) == 0 || callers[0].(map[string]any)["qname"] != "b.Hello" {
		t.Errorf("callers of a.Greet = %v, want b.Hello", callers)
	}

	vals := callTool(t, store, "search_code", map[string]any{"pattern": `\.current$`, "in": "name", "expand": []any{"references"}})
	if v := hitByName(vals, "coll1.current"); v == nil || len(v["references"].([]any)) == 0 {
		t.Errorf("expand references on a variable: %v", v)
	}
}

// TestSearchCode_Filters: name-only search, path and an invalid pattern.
func TestSearchCode_Filters(t *testing.T) {
	store := newFixtureStore(t)
	byName := callTool(t, store, "search_code", map[string]any{"pattern": `Current$`, "in": "name"})
	for _, n := range hitNames(byName) {
		if !strings.HasSuffix(n, "Current") {
			t.Errorf("name search returned %s", n)
		}
	}
	if len(hitNames(byName)) != 3 {
		t.Errorf("want setCurrent, readCurrent, incCurrent: %v", hitNames(byName))
	}
	if other := callTool(t, store, "search_code", map[string]any{"pattern": `func`, "path": "a/"}); len(hitNames(other)) == 0 {
		t.Error("path prefix a/ found nothing")
	} else {
		for _, r := range other["hits"].([]any) {
			if f := r.(map[string]any)["file"].(string); !strings.HasPrefix(f, "a/") {
				t.Errorf("path filter let %s through", f)
			}
		}
	}
	header := callTool(t, store, "search_code", map[string]any{"pattern": `func \w*Current\(`, "in": "name"})
	if len(hitNames(header)) != 3 {
		t.Errorf("name mode must match declaration headers as grep does: %v", hitNames(header))
	}
	narrow := callTool(t, store, "search_code", map[string]any{"pattern": `current`, "path": "nowhere/"})
	if narrow["without_filters"] == nil {
		t.Errorf("an empty result emptied by filters must say how many match without them: %v", narrow)
	}
	bad := callTool(t, store, "search_code", map[string]any{"pattern": `(`})
	if bad["error"] == nil {
		t.Error("an invalid pattern must say why instead of failing the call")
	}
}

// TestSearchText_GrepStyleInput: alternation and regex input no longer fail
// with an FTS syntax error.
func TestSearchText_GrepStyleInput(t *testing.T) {
	store := newFixtureStore(t)
	alt := callTool(t, store, "search_text", map[string]any{"query": "setCurrent|readCurrent"})
	if nodes, _ := alt["nodes"].([]any); len(nodes) < 2 {
		t.Errorf("alternation of names: %v", alt)
	}
	rx := callTool(t, store, "search_text", map[string]any{"query": `current\s*=`})
	if rx["note"] == nil {
		t.Errorf("a regex should be routed to search_code with a note: %v", rx)
	}
	if nodes, _ := rx["nodes"].([]any); len(nodes) == 0 {
		t.Errorf("regex routed to search_code found nothing: %v", rx)
	}
}
