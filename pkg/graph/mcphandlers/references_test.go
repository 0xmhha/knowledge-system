package mcphandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

func callTool(t *testing.T, store persist.StoreReader, name string, args map[string]any) map[string]any {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, store)
	tool := s.GetTool(name)
	if tool == nil || tool.Handler == nil {
		t.Fatalf("%s not registered", name)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}

func users(out map[string]any) map[string]string {
	got := map[string]string{}
	rows, _ := out["users"].([]any)
	for _, r := range rows {
		m := r.(map[string]any)
		got[m["qname"].(string)+" "+m["access"].(string)] = m["file"].(string)
		if m["line"].(float64) == 0 {
			got["zero line"] = m["qname"].(string)
		}
	}
	return got
}

// TestFindReferences: the users of a variable, a constant and a promoted
// field come back with their access kind and site, writers first; a
// function seed gets a note instead.
func TestFindReferences(t *testing.T) {
	store := newFixtureStore(t)

	out := callTool(t, store, "find_references", map[string]any{"qname": "coll1.current"})
	got := users(out)
	for _, want := range []string{"coll1.setCurrent write", "coll1.incCurrent write", "coll1.incCurrent read", "coll1.readCurrent read"} {
		if _, ok := got[want]; !ok {
			t.Errorf("coll1.current: missing %q in %v", want, got)
		}
	}
	if _, bad := got["zero line"]; bad {
		t.Errorf("a user has no line: %v", got)
	}
	if first := out["users"].([]any)[0].(map[string]any); first["access"] != "write" {
		t.Errorf("writers must come first: %v", first)
	}

	if got := users(callTool(t, store, "find_references", map[string]any{"qname": "coll1.Limit"})); len(got) != 2 {
		t.Errorf("coll1.Limit users = %v, want readCurrent and the Registry initializer", got)
	}
	writes := users(callTool(t, store, "find_references", map[string]any{"qname": "coll1.Inner.Depth", "access": "write"}))
	if _, ok := writes["coll1.bump write"]; !ok || len(writes) != 1 {
		t.Errorf("Inner.Depth writers = %v, want only bump", writes)
	}
	// bump writes Depth on two lines: one row, both lines, two uses.
	bump := callTool(t, store, "find_references", map[string]any{"qname": "coll1.Inner.Depth", "access": "write"})
	if rows := bump["users"].([]any); len(rows) != 1 {
		t.Errorf("bump must be one row, got %v", rows)
	} else if r := rows[0].(map[string]any); fmt.Sprint(r["lines"]) != "[35 36]" || r["uses"].(float64) != 2 || r["line"].(float64) != 35 {
		t.Errorf("bump row = %v, want lines [35 36], uses 2, line 35", r)
	}

	fn := callTool(t, store, "find_references", map[string]any{"qname": "coll1.setBox"})
	if fn["note"] == nil || len(users(fn)) != 0 {
		t.Errorf("a function seed should get a note and no users: %v", fn)
	}
}

// TestFindCallers_ValueSeed: find_callers on a field answers with the code
// that reads or writes it and says why, instead of an empty call graph.
func TestFindCallers_ValueSeed(t *testing.T) {
	store := newFixtureStore(t)
	out := callTool(t, store, "find_callers", map[string]any{"qname": "coll1.Box.Val"})
	got := users(out)
	if _, ok := got["coll1.setBox write"]; !ok {
		t.Errorf("missing setBox write in %v", got)
	}
	if _, ok := got["coll1.getBox read"]; !ok {
		t.Errorf("missing getBox read in %v", got)
	}
	if out["note"] == nil {
		t.Error("find_callers on a field must say calls are recorded for functions only")
	}
	if callers := callTool(t, store, "find_callers", map[string]any{"qname": "a.Greet"}); callers["users"] != nil {
		t.Errorf("a function seed keeps the call graph shape: %v", callers)
	}
}

// TestInstructionsNameEveryTool: the server guide names only tools the server
// registers, and names the ones a model most often misses.
func TestInstructionsNameEveryTool(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, newFixtureStore(t))
	text := Instructions()
	for _, name := range []string{"find_symbol", "search_text", "search_code", "find_callers", "find_callees", "find_references",
		"get_subgraph", "get_context_for_task", "impact_of_change", "concurrency_impact", "change_history", "value_dependencies",
		"investigate", "investigate_open"} {
		if s.GetTool(name) == nil {
			t.Errorf("instructions name %q, which is not registered", name)
		}
		if !strings.Contains(text, name) {
			t.Errorf("instructions do not mention %q", name)
		}
	}
	if strings.Contains(text, "{") {
		t.Errorf("unreplaced placeholder in instructions")
	}
}
