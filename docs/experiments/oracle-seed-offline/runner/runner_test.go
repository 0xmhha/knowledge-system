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
