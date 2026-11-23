package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func node(id string, t types.NodeType, name string, start, end int) types.Node {
	return types.Node{ID: id, Type: t, Name: name, FilePath: "a.go", StartLine: start, EndLine: end}
}

func TestInnermostDecl(t *testing.T) {
	ns := []types.Node{
		node("s", types.NodeStruct, "T", 1, 50),
		node("m", types.NodeMethod, "M", 10, 20),
		node("f", types.NodeField, "x", 12, 12),
		node("file", types.NodeFile, "a.go", 1, 100),
	}
	cases := []struct {
		line int
		want string
		ok   bool
	}{
		{12, "m", true}, // a field is not a declaration target
		{30, "s", true},
		{80, "", false}, // only the File node holds it
	}
	for _, c := range cases {
		n, ok := innermostDecl(ns, c.line)
		if ok != c.ok || (ok && n.ID != c.want) {
			t.Errorf("line %d: got %q %v, want %q %v", c.line, n.ID, ok, c.want, c.ok)
		}
	}
}

func TestMatchOracle(t *testing.T) {
	ns := []types.Node{
		node("a", types.NodeFunction, "init", 5, 9),
		node("b", types.NodeFunction, "init", 40, 60),
		node("c", types.NodeMethod, "stopTimer", 100, 112),
	}
	cases := []struct {
		ref  oracleRef
		want string
		ok   bool
	}{
		{oracleRef{Symbol: "init", StartLine: 41, EndLine: 60}, "b", true},
		{oracleRef{Symbol: "Core.stopTimer", StartLine: 99, EndLine: 112}, "c", true},
		{oracleRef{Symbol: "Core.stopTimer", StartLine: 1, EndLine: 3}, "", false},
	}
	for _, c := range cases {
		n, ok := matchOracle(ns, c.ref)
		if ok != c.ok || (ok && n.ID != c.want) {
			t.Errorf("%s: got %q %v, want %q %v", c.ref.Symbol, n.ID, ok, c.want, c.ok)
		}
	}
}

type fakeFiles map[string][]types.Node

func TestResolveSeedsFieldAndSharedSpan(t *testing.T) {
	promoted := node("a1", types.NodeMethod, "SetBlacklisted", 434, 439)
	canon := node("b2", types.NodeMethod, "SetBlacklisted", 434, 439)
	canon.CanonicalID = "state.(*StateDB).SetBlacklisted"
	fn := &fileNodes{cache: fakeFiles{"a.go": {
		promoted, canon,
		node("f1", types.NodeField, "EffectiveGasPrice", 71, 71),
		node("f2", types.NodeField, "EffectiveGasPrice", 88, 88),
	}}}
	seeds := []seedRef{
		{Symbol: "SetBlacklisted", File: "a.go"},
		{Symbol: "EffectiveGasPrice", File: "a.go"},
	}
	rs, nodes, err := resolveSeeds(fn, seeds)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs[0].Nodes) != 1 || len(rs[1].Nodes) != 2 {
		t.Fatalf("got %d and %d seed locations, want 1 and 2", len(rs[0].Nodes), len(rs[1].Nodes))
	}
	if len(nodes) != 3 || nodes[0].ID != "b2" {
		t.Errorf("got %d nodes, first %q; want 3 with the canonical node b2 first", len(nodes), nodes[0].ID)
	}
}

func TestMeasure(t *testing.T) {
	l := func(s int) *loc { return &loc{File: "a.go", StartLine: s, EndLine: s + 1} }
	cands := make([]candidate, 0, 12)
	for i := 1; i <= 12; i++ {
		cands = append(cands, candidate{Rank: i, loc: *l(i * 10)})
	}
	gold := []resolvedGold{
		{Node: l(20), Resolved: true},              // rank 2
		{Node: l(120), Resolved: true},             // rank 12
		{Node: l(999), Resolved: true},             // not reached
		{Node: l(5), Resolved: true, IsSeed: true}, // seed
		{Resolved: false},                          // left out
	}
	m := measure(cands, gold)
	if m.GoldTotal != 4 || m.Reached != 3 || m.InTop10 != 2 {
		t.Fatalf("got total %d reached %d top10 %d", m.GoldTotal, m.Reached, m.InTop10)
	}
	want := []int{2, 12, rankNotReached, rankSeed}
	for i, r := range m.GoldRanks {
		if r != want[i] {
			t.Errorf("rank %d: got %d want %d", i, r, want[i])
		}
	}
	if m.MedianRank == nil || *m.MedianRank != 2 {
		t.Errorf("median: got %v want 2", m.MedianRank)
	}
}

func TestGrepFileWordBoundary(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.go")
	src := "a := txMaxSize\nb := txMaxSizeLimit\nc := maxtxMaxSize\nd := (txMaxSize)\n"
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := grepFile(p, []*regexp.Regexp{regexp.MustCompile(`\btxMaxSize\b`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].line != 1 || hits[1].line != 4 {
		t.Errorf("got %+v, want lines 1 and 4", hits)
	}
}

func TestParseFlagsRefusesGraphDir(t *testing.T) {
	_, err := parseFlags([]string{"-data", "d", "-root", "r", "-out", "/x/runs/y/graphs/abc/out"})
	if err == nil {
		t.Fatal("want an error for an -out inside a graph directory")
	}
}

func TestShortestPathsModes(t *testing.T) {
	// s -calls-> a -calls-> g ; s <-contains- f -contains-> g ; b -calls-> g
	edges := []types.Edge{
		{Src: "s", Dst: "a", Type: types.EdgeCalls},
		{Src: "a", Dst: "g", Type: types.EdgeCalls},
		{Src: "f", Dst: "s", Type: types.EdgeContains},
		{Src: "f", Dst: "g", Type: types.EdgeContains},
		{Src: "s", Dst: "a", Type: types.EdgeCalls}, // duplicate is ignored
	}
	adj := newAdjacency(edges)
	if got := adj.degree["s"]; got != 2 {
		t.Fatalf("degree(s) = %d, want 2 (duplicate edge counted once)", got)
	}
	target := map[string]bool{"g": true}
	cases := []struct {
		mode     walkMode
		wantHops int
		wantEdge types.EdgeType
	}{
		{walkForward, 2, types.EdgeCalls},
		{walkUndirected, 2, types.EdgeCalls}, // forward arcs are tried first, so the calls path wins the tie
		{walkUndirectedSemantic, 2, types.EdgeCalls},
	}
	for _, c := range cases {
		p := shortestPaths(adj, []string{"s"}, target, c.mode)["g"]
		if len(p) != c.wantHops || p[0].edge != c.wantEdge {
			t.Errorf("%s: path %+v, want %d hops starting with %s", c.mode, p, c.wantHops, c.wantEdge)
		}
	}
	// Only the structural route connects s to h; the semantic walk must not find it.
	adj2 := newAdjacency([]types.Edge{
		{Src: "f", Dst: "s", Type: types.EdgeContains},
		{Src: "f", Dst: "h", Type: types.EdgeContains},
	})
	h := map[string]bool{"h": true}
	if p := shortestPaths(adj2, []string{"s"}, h, walkUndirected)["h"]; len(p) != 2 || p[0].fwd {
		t.Errorf("undirected: path %+v, want 2 hops, first one backwards", p)
	}
	if _, ok := shortestPaths(adj2, []string{"s"}, h, walkUndirectedSemantic)["h"]; ok {
		t.Error("semantic walk crossed a structural edge")
	}
	if _, ok := shortestPaths(adj2, []string{"s"}, h, walkForward)["h"]; ok {
		t.Error("forward walk went against an edge")
	}
}

func TestShortestPathsMaxHops(t *testing.T) {
	var edges []types.Edge
	ids := []string{"n0", "n1", "n2", "n3", "n4", "n5", "n6", "n7"}
	for i := 0; i+1 < len(ids); i++ {
		edges = append(edges, types.Edge{Src: ids[i], Dst: ids[i+1], Type: types.EdgeCalls})
	}
	adj := newAdjacency(edges)
	got := shortestPaths(adj, []string{"n0"}, map[string]bool{"n6": true, "n7": true}, walkForward)
	if len(got["n6"]) != maxHops {
		t.Errorf("n6: %d hops, want %d", len(got["n6"]), maxHops)
	}
	if _, ok := got["n7"]; ok {
		t.Errorf("n7 is %d hops away but was reached", maxHops+1)
	}
}

func TestChangedDecls(t *testing.T) {
	n := func(id, name string, s, e int) types.Node {
		return types.Node{ID: id, Name: name, Type: types.NodeFunction, FilePath: "a.go", StartLine: s, EndLine: e, CanonicalID: id}
	}
	promoted := n("a0", "g", 20, 30)
	promoted.CanonicalID = ""
	fn := &fileNodes{cache: map[string][]types.Node{"a.go": {n("f", "f", 1, 10), n("g", "g", 20, 30), promoted}}}
	cases := []struct {
		name string
		hs   []hunk
		want []string
	}{
		{"line inside", []hunk{{File: "a.go", Start: 5, End: 5}}, []string{"f"}},
		{"range over two, one per location, canonical node kept", []hunk{{File: "a.go", Start: 8, End: 22}}, []string{"f", "g"}},
		{"between declarations", []hunk{{File: "a.go", Start: 12, End: 14}}, nil},
		{"insert inside", []hunk{{File: "a.go", Start: 5, End: 5, Insert: true}}, []string{"f"}},
		{"insert after the end", []hunk{{File: "a.go", Start: 10, End: 10, Insert: true}}, nil},
	}
	for _, c := range cases {
		got, err := changedDecls(fn, c.hs)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, g := range got {
			ids = append(ids, g.ID)
		}
		if len(ids) != len(c.want) {
			t.Errorf("%s: %v, want %v", c.name, ids, c.want)
			continue
		}
		for i := range ids {
			if ids[i] != c.want[i] {
				t.Errorf("%s: %v, want %v", c.name, ids, c.want)
			}
		}
	}
}

func TestSameFileArmOrder(t *testing.T) {
	mk := func(s, e int) types.Node {
		return types.Node{Type: types.NodeFunction, FilePath: "a.go", StartLine: s, EndLine: e}
	}
	got := sameFileArm([]types.Node{mk(1, 5), mk(50, 60), mk(10, 20), mk(30, 40), mk(30, 40)}, loc{"a.go", 10, 20})
	want := []int{1, 30, 50}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].StartLine != w || got[i].Rank != i+1 {
			t.Errorf("rank %d: start %d, want %d", i+1, got[i].StartLine, w)
		}
	}
}

func TestReachableWalks(t *testing.T) {
	// a writes f, b reads f; c calls a; d calls c; a calls h, e calls h.
	adj := newAdjacency([]types.Edge{
		{Src: "a", Dst: "f", Type: types.EdgeWritesField},
		{Src: "b", Dst: "f", Type: types.EdgeReadsField},
		{Src: "c", Dst: "a", Type: types.EdgeCalls},
		{Src: "d", Dst: "c", Type: types.EdgeCalls},
		{Src: "a", Dst: "h", Type: types.EdgeCalls},
		{Src: "e", Dst: "h", Type: types.EdgeCalls},
	})
	want := map[string]map[string]bool{
		"W1": {"c": true, "d": true, "h": true},
		"W2": {"c": true, "d": true, "h": true, "e": true},
		"W3": {"f": true, "b": true},
		"W4": {"c": true, "d": true, "h": true, "e": true, "f": true, "b": true},
	}
	for _, w := range diagWalks() {
		exp, ok := want[w.name]
		if !ok {
			continue
		}
		got := reachable(adj, "a", w)
		if len(got) != len(exp) {
			t.Errorf("%s: reached %v, want %v", w.name, got, exp)
		}
		for id := range exp {
			if !got[id] {
				t.Errorf("%s: %s not reached", w.name, id)
			}
		}
	}
	hub := diagWalk{name: "hub", passes: []arcFilter{func(a arc) bool { return walkEdges[a.edge] }}, hops: 3, hub: 1}
	if got := reachable(adj, "a", hub); got["e"] || got["d"] || !got["h"] {
		t.Errorf("hub limit: reached %v; h may be listed but not walked through", got)
	}
}

func TestShippedPairs(t *testing.T) {
	l := func(n int) loc { return loc{"a.go", n, n} }
	c := cochangeResult{Changed: []loc{l(1), l(2), l(3)}, Starts: []startResult{
		{Start: l(1), Measures: map[string]armMeasure{string(variantUncapped): {GoldRanks: []int{rankNotReached, 4}}}},
	}}
	got := shippedPairs(c)
	if len(got) != 1 || !got[[2]loc{l(1), l(3)}] {
		t.Errorf("got %v", got)
	}
}

func TestLinkScores(t *testing.T) {
	// a-f, b-f (shared field), a-c direct, c-d, a-h, e-h, x-h (h is a hub).
	lg := newLinkGraph([]types.Edge{
		{Src: "a", Dst: "f", Type: types.EdgeWritesField},
		{Src: "b", Dst: "f", Type: types.EdgeReadsField},
		{Src: "b", Dst: "f", Type: types.EdgeWritesField}, // same pair, counted once
		{Src: "a", Dst: "c", Type: types.EdgeCalls},
		{Src: "d", Dst: "c", Type: types.EdgeCalls},
		{Src: "a", Dst: "h", Type: types.EdgeCalls},
		{Src: "e", Dst: "h", Type: types.EdgeCalls},
		{Src: "x", Dst: "h", Type: types.EdgeCalls},
		{Src: "a", Dst: "z", Type: types.EdgeImplements}, // not in the group
	}, linkGroups()[1])
	got := lg.linkScores("a", linkWeights()[2])
	want := map[string]float64{"f": 1, "c": 1, "h": 1, "b": 1.0 / 2, "d": 1.0 / 2, "e": 1.0 / 3, "x": 1.0 / 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: %v, want %v", id, got[id], w)
		}
	}
	locs := map[string]loc{"b": {"b.go", 1, 2}, "c": {"c.go", 1, 2}, "d": {"a.go", 9, 10}, "e": {"s.go", 1, 1}}
	cands := linkArm(got, locs, loc{"s.go", 1, 1})
	if len(cands) != 3 || cands[0].File != "c.go" || cands[1].File != "a.go" || cands[2].Rank != 3 {
		t.Errorf("ranking %+v", cands)
	}
}
