package mcphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// fanInCallers is how many functions call fanin.hub in fanInStore: more than
// any list a tool cuts (searchCodeExpandCap, the investigate card's caller
// cap), so a cut has to choose.
const fanInCallers = 25

// fanInStore holds fanin.hub, which calls fanin.leaf, and fanInCallers
// functions that call hub and leaf.
func fanInStore(t *testing.T) persist.Store {
	t.Helper()
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	const file = "fanin/fanin.go"
	fn := func(id, name string, line int, src string) types.Node {
		return types.Node{ID: id, Type: types.NodeFunction, Name: name, QualifiedName: "fanin." + name,
			CanonicalID: "ex/fanin." + name, FilePath: file, StartLine: line, EndLine: line,
			EndByte: len(src), Language: "go", Confidence: types.ConfExtracted}
	}
	hubSrc := "func hub() { leaf() }"
	nodes := []types.Node{fn("hb00000000000001", "hub", 1, hubSrc), fn("hb00000000000002", "leaf", 2, "func leaf() {}")}
	blobs := map[string][]byte{"hb00000000000001": []byte(hubSrc), "hb00000000000002": []byte("func leaf() {}")}
	edges := []types.Edge{{Src: "hb00000000000001", Dst: "hb00000000000002", Type: types.EdgeCalls, Line: 1, Count: 1, Confidence: types.ConfExtracted}}
	for i := 0; i < fanInCallers; i++ {
		id := fmt.Sprintf("cl%014d", i)
		name := fmt.Sprintf("caller%02d", i)
		src := "func " + name + "() { hub(); leaf() }"
		nodes = append(nodes, fn(id, name, 10+i, src))
		blobs[id] = []byte(src)
		for _, dst := range []string{"hb00000000000001", "hb00000000000002"} {
			edges = append(edges, types.Edge{Src: id, Dst: dst, Type: types.EdgeCalls, Line: 10 + i, Count: 1, Confidence: types.ConfExtracted})
		}
	}
	nodes = append(nodes, types.Node{ID: "fi00000000000001", Type: types.NodeFile, Name: file, QualifiedName: "file:" + file,
		FilePath: file, StartLine: 1, EndLine: 1, EndByte: 1, Language: "go", Confidence: types.ConfExtracted})
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertBlobs(blobs); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEdges(edges); err != nil {
		t.Fatal(err)
	}
	if err := s.RebuildFTS(); err != nil {
		t.Fatal(err)
	}
	return s
}

// determinismCalls are the calls TestToolsAreDeterministic repeats: one
// sequence (calls on one server) per case, and every tool in at least one.
var determinismCalls = [][]struct {
	tool string
	args map[string]any
}{
	{{"search_code", map[string]any{"pattern": `leaf\(\)`, "expand": []any{"callers", "callees", "references"}}}},
	{{"search_code", map[string]any{"pattern": `func hub`, "expand": []any{"callers"}}}},
	{{"search_test_code", map[string]any{"pattern": `hub`}}},
	{{"find_callers", map[string]any{"qname": "fanin.leaf", "depth": 2}}},
	{{"find_callees", map[string]any{"qname": "fanin.caller03", "depth": 2}}},
	{{"find_references", map[string]any{"qname": "fanin.hub"}}},
	{{"get_subgraph", map[string]any{"seed_qname": "fanin.hub"}}},
	{{"impact_of_change", map[string]any{"seed_qname": "fanin.leaf"}}},
	{{"concurrency_impact", map[string]any{"seed_qname": "fanin.hub"}}},
	{{"change_history", map[string]any{"qname": "fanin.hub"}}},
	{{"execution_flow", map[string]any{"qname": "fanin.hub"}}, {"execution_flow", map[string]any{"qname": "fanin.leaf", "direction": "backward"}}},
	{{"evidence_for_intent", map[string]any{"intent": "hub leaf"}}},
	{{"search_text", map[string]any{"query": "caller"}}},
	{{"find_symbol", map[string]any{"name": "hub", "exact": false}}},
	{{"get_context_for_task", map[string]any{"task_description": "what calls hub and leaf"}}},
	{{"value_dependencies", map[string]any{"canonical_id": "ex/fanin.hub"}}},
	{{"rule_candidates", map[string]any{"rule_pack": "go-local-review@1.0.0", "function": map[string]any{"canonical_id": "ex/fanin.hub"}}}},
	{
		{"investigate", map[string]any{"words": "hub|leaf", "budget": 2000}},
		{"investigate_open", map[string]any{"open": "card:fanin.hub"}},
		{"investigate_open", map[string]any{"open": "hop:leaf:1"}},
		{"investigate_open", map[string]any{"open": "rest"}},
	},
}

// runSequence runs one sequence of calls on a fresh server over store and
// returns every response as bytes: its structured content and its text.
// elapsed_micros, the time value_dependencies and rule_candidates measure
// and publish in their output schema, is set to 0: it is the one field a
// repeated call may change.
func runSequence(t *testing.T, store persist.StoreReader, calls []struct {
	tool string
	args map[string]any
}) []byte {
	t.Helper()
	srv := server.NewMCPServer("test", "0")
	RegisterAll(srv, store)
	var out bytes.Buffer
	for _, c := range calls {
		tool := srv.GetTool(c.tool)
		if tool == nil {
			t.Fatalf("%s not registered", c.tool)
		}
		req := mcp.CallToolRequest{}
		req.Params.Name = c.tool
		req.Params.Arguments = c.args
		res, err := tool.Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: %v", c.tool, err)
		}
		raw, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("%s: %v", c.tool, err)
		}
		out.Write(elapsedMicros.ReplaceAll(raw, []byte(`elapsed_micros${q}:0`)))
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// elapsedMicros matches the measured time in a response, in the structured
// content and in the text block, where the quote is escaped.
var elapsedMicros = regexp.MustCompile(`elapsed_micros(?P<q>\\?"):\d+`)

// TestToolsAreDeterministic: the same calls give the same bytes, on a fresh
// server each time and on two stores built from the same nodes. A search hit
// with more than 10 callers listed a different 10 on each call (the
// neighbourhood came out of a map), and investigate differed across fresh
// servers on 5 of 30 queries (a guard's statement line was read from a
// promoted copy in map order). Every registered tool is called.
func TestToolsAreDeterministic(t *testing.T) {
	stores := []persist.Store{fanInStore(t), fanInStore(t)}
	srv := server.NewMCPServer("test", "0")
	RegisterAll(srv, stores[0])
	covered := map[string]bool{}
	for _, seq := range determinismCalls {
		for _, c := range seq {
			covered[c.tool] = true
		}
	}
	for name := range srv.ListTools() {
		if !covered[name] {
			t.Errorf("tool %s has no call in determinismCalls", name)
		}
	}
	for _, seq := range determinismCalls {
		first := runSequence(t, stores[0], seq)
		for i, s := range []persist.StoreReader{stores[0], stores[0], stores[1], stores[1]} {
			if again := runSequence(t, s, seq); !bytes.Equal(first, again) {
				t.Errorf("%s: run %d differs:\n%s\n%s", seq[0].tool, i, first, again)
				break
			}
		}
	}
	// The message and goroutine keys, on subjects with several entries on
	// one line (the concurrency fixture's Two, RunTwo and multi).
	msg := []persist.Store{messagesFixture(t), messagesFixture(t)}
	for _, seq := range messageDeterminismCalls {
		first := runSequence(t, msg[0], seq)
		for i := 0; i < 8; i++ {
			if again := runSequence(t, msg[i%2], seq); !bytes.Equal(first, again) {
				t.Errorf("%s %v: run %d differs:\n%s\n%s", seq[0].tool, seq[0].args, i, first, again)
				break
			}
		}
	}
}

// messageDeterminismCalls cover handled_by, goroutine_callers,
// closure_calls, triggers and goroutine_users.
var messageDeterminismCalls = [][]struct {
	tool string
	args map[string]any
}{
	{{"find_callers", map[string]any{"qname": "mutex_fixture.multi", "depth": 1}}},
	{{"find_callees", map[string]any{"qname": "mutex_fixture.RunTwo", "depth": 1}}},
	{{"find_references", map[string]any{"qname": "mutex_fixture.hits"}}},
	{{"concurrency_impact", map[string]any{"seed_qname": "mutex_fixture.RunTwo"}}},
}

// TestExpandTotalIsFindCallersAtDepthOne: the callers an expansion counts are
// exactly what find_callers returns at depth 1 besides the hit, as the guide
// says; find_callers' default depth 2 returns more. The guide said they
// "return them all" without the depth.
func TestExpandTotalIsFindCallersAtDepthOne(t *testing.T) {
	s := fanInStore(t)
	out := callTool(t, s, "search_code", map[string]any{"pattern": `^func leaf`, "expand": []any{"callers"}})
	h := hitByName(out, "fanin.leaf")
	if h == nil || h["callers_total"] != float64(fanInCallers+1) {
		t.Fatalf("hit %v, want callers_total %d", h, fanInCallers+1)
	}
	callers := callTool(t, s, "find_callers", map[string]any{"qname": h["canonical_id"], "depth": 1})
	var names []string
	for _, n := range asSlice(callers["nodes"]) {
		if q := n.(map[string]any)["qname"]; q != "fanin.leaf" {
			names = append(names, q.(string))
		}
	}
	if len(names) != fanInCallers+1 {
		t.Errorf("find_callers depth=1 returns %d callers, the expansion counts %v", len(names), h["callers_total"])
	}
	shown := rowNames(h, "callers")
	if want := sorted(names)[:searchCodeExpandCap]; !reflect.DeepEqual(shown, want) {
		t.Errorf("shown %v, want the first %d of find_callers by name: %v", shown, searchCodeExpandCap, want)
	}
}

// TestIndexKeepsCanonicalIDOfSharedName: a hit printed in the site form has
// a qualified name that other declarations carry; its index row keeps the
// canonical_id it had while such rows shared the plain qname (535 search
// hits on go-stablenet lost it).
func TestIndexKeepsCanonicalIDOfSharedName(t *testing.T) {
	// The budgets below are sized for one copy of the answer.
	withContentMode(t, ContentStructured)
	old := searchCodePageBytes
	t.Cleanup(func() { searchCodePageBytes = old })
	searchCodePageBytes = 2500
	long := []map[string]any{{"line": 1, "text": strings.Repeat("x", 2000)}}
	out := map[string]any{"pattern": "x", "hits": []map[string]any{
		{"qname": "eth.testKey@eth/handler_test.go:43", "canonical_id": "ex/eth.testKey", "file": "eth/handler_test.go", "start_line": 43, "end_line": 43, "lines": long},
		{"qname": "p.unique", "canonical_id": "ex/p.unique", "file": "p/p.go", "start_line": 1, "end_line": 1, "lines": long},
	}}
	res, ok := indexSearchCode(out, nil, "search_code")
	if !ok {
		t.Fatal("no index for a result larger than a part")
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"canonical_id":"ex/eth.testKey"`)) {
		t.Errorf("index lost the canonical_id of a site-form row: %s", raw)
	}
}
