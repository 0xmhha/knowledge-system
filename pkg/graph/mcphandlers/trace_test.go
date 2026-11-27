package mcphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// tracedCall runs one tool through TraceToolCalls and returns its trace record.
func tracedCall(t *testing.T, store persist.StoreReader, name string, args map[string]any) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, store)
	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("%s not registered", name)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	if _, err := TraceToolCalls(&buf)(tool.Handler)(context.Background(), req); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("trace record %q: %v", buf.String(), err)
	}
	return rec
}

func traceStage(rec map[string]any, stage string) map[string]any {
	evs, _ := rec["events"].([]any)
	for _, e := range evs {
		if m := e.(map[string]any); m["stage"] == stage {
			return m["data"].(map[string]any)
		}
	}
	return nil
}

// TestTrace_SearchCode: a traced search_code call records every ranked
// declaration and whether it was returned, and for each returned hit the
// matched lines it did not show. search_code shows every declaration and
// every matched line, so nothing is cut or hidden; a larger result is split
// into parts, and the trace records how many.
func TestTrace_SearchCode(t *testing.T) {
	store := newFixtureStore(t)
	rec := tracedCall(t, store, "search_code", map[string]any{"pattern": `.`})
	if rec["tool"] != "search_code" || rec["result"] == nil {
		t.Fatalf("record without tool or result: %v", rec)
	}
	for _, st := range []string{"search_code.pattern", "search_code.scan", "search_code.rank", "search_code.view", "search_code.page"} {
		if traceStage(rec, st) == nil {
			t.Fatalf("stage %s missing: %v", st, rec["events"])
		}
	}
	rank := traceStage(rec, "search_code.rank")
	total := rank["ranked_total"].(float64)
	if total <= 2 || rank["cut_by_limit"].(float64) != 0 {
		t.Errorf("limit is not a tool argument any more; nothing may be cut: %v", rank)
	}
	returned := 0
	for _, r := range rank["ranked"].([]any) {
		if r.(map[string]any)["returned"] == true {
			returned++
		}
	}
	if float64(returned) != total {
		t.Errorf("returned flags = %d, want all %v", returned, total)
	}
	long := false
	for _, v := range traceStage(rec, "search_code.view")["hits"].([]any) {
		m := v.(map[string]any)
		if h, _ := m["hidden_matched_lines"].([]any); len(h) > 0 {
			t.Errorf("%s: hidden %v, want none", m["qname"], h)
		}
		if len(m["matched_lines"].([]any)) > searchCodeLinesPerHit {
			long = true
		}
	}
	if !long {
		t.Error("no hit has more matched lines than the old per-hit cap; the test proves nothing")
	}
}

// TestTrace_SeedAndWalk: traversal tools record how the seed resolved and
// the shape of what they returned.
func TestTrace_SeedAndWalk(t *testing.T) {
	store := newFixtureStore(t)
	rec := tracedCall(t, store, "find_callers", map[string]any{"qname": "Greet"})
	seed := traceStage(rec, "seed")
	if seed == nil || seed["step"] != "suffix" || seed["resolved_qname"] != "a.Greet" {
		t.Errorf("seed = %v", seed)
	}
	if w := traceStage(rec, "walk"); w == nil || w["nodes"].(float64) == 0 {
		t.Errorf("walk = %v", w)
	}
	miss := tracedCall(t, store, "find_references", map[string]any{"qname": "no.Such"})
	if s := traceStage(miss, "seed"); s == nil || s["step"] != "not_found" {
		t.Errorf("seed for a missing name = %v", s)
	}
}

// TestTrace_Off: without the middleware no trace is collected and the tools
// answer as before.
func TestTrace_Off(t *testing.T) {
	store := newFixtureStore(t)
	out := callTool(t, store, "search_code", map[string]any{"pattern": `current`})
	if len(hitNames(out)) == 0 {
		t.Fatal("search_code returned nothing without a trace")
	}
}

func TestTopLevelAlternatives(t *testing.T) {
	for in, want := range map[string][]string{
		`a|b|c`:          {"a", "b", "c"},
		`func (a|b)\(|x`: {`func (a|b)\(`, "x"},
		`[|]|y`:          {"[|]", "y"},
		`a\|b`:           {`a\|b`},
	} {
		if got := topLevelAlternatives(in); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
