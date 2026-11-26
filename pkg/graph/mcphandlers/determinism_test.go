package mcphandlers

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

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

// TestToolsAreDeterministic: the same call gives the same bytes. A search
// hit with more than 10 callers listed a different 10 on each call (the
// neighbourhood came out of a map); every tool that returns or cuts a list
// of neighbours is called twice, each time on a fresh server, and compared.
func TestToolsAreDeterministic(t *testing.T) {
	s := fanInStore(t)
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"search_code", map[string]any{"pattern": `leaf\(\)`, "expand": []any{"callers", "callees"}}},
		{"search_code", map[string]any{"pattern": `func hub`, "expand": []any{"callers"}}},
		{"find_callers", map[string]any{"qname": "fanin.leaf", "depth": 2}},
		{"find_callees", map[string]any{"qname": "fanin.caller03", "depth": 2}},
		{"get_subgraph", map[string]any{"seed_qname": "fanin.hub"}},
		{"impact_of_change", map[string]any{"seed_qname": "fanin.leaf"}},
		{"search_text", map[string]any{"query": "caller"}},
		{"find_symbol", map[string]any{"name": "hub", "exact": false}},
	} {
		first := callToolRaw(t, s, tc.tool, tc.args)
		for i := 0; i < 5; i++ {
			if again := callToolRaw(t, s, tc.tool, tc.args); !bytes.Equal(first, again) {
				t.Errorf("%s %v: two calls differ:\n%s\n%s", tc.tool, tc.args, first, again)
				break
			}
		}
	}
	card := func() string {
		srv := newInvServer(t, s)
		head := callTextOn(t, srv, "investigate", map[string]any{"words": "hub|leaf", "budget": 2000})
		return head + callTextOn(t, srv, "investigate_open", map[string]any{"open": "card:fanin.hub"}) +
			callTextOn(t, srv, "investigate_open", map[string]any{"open": "hop:leaf:1"})
	}
	first := card()
	for i := 0; i < 5; i++ {
		if again := card(); again != first {
			t.Errorf("investigate and investigate_open differ:\n%s\n%s", first, again)
			break
		}
	}
}
