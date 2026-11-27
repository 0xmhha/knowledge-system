package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// starGraph is a seed with leaves first-level neighbours, each with one
// child of its own: depth 1 reaches 1+leaves nodes, depth 2 1+2*leaves.
func starGraph(leaves int) (types.Node, map[string]types.Node, map[string][]string) {
	seed := types.Node{ID: "seed", Name: "Seed", QualifiedName: "p.Seed", Type: types.NodeFunction}
	idx := map[string]types.Node{seed.ID: seed}
	var edges []types.Edge
	for i := range leaves {
		leaf := types.Node{ID: fmt.Sprintf("l%03d", i), Name: "L", QualifiedName: "p.L", Type: types.NodeFunction}
		child := types.Node{ID: fmt.Sprintf("c%03d", i), Name: "C", QualifiedName: "p.C", Type: types.NodeFunction}
		idx[leaf.ID], idx[child.ID] = leaf, child
		edges = append(edges, types.Edge{Src: seed.ID, Dst: leaf.ID}, types.Edge{Src: leaf.ID, Dst: child.ID})
	}
	return seed, idx, buildAdjacency(edges)
}

// TestBenchmarkWalk_ReportsTheCap: the walk renders at most maxNodes nodes,
// and says how many it reached within the depth and how many it cut. With
// the cap hidden at 200, depth 1 and depth 2 gave the same tokens and the
// ratio came from the cap, not from the depth.
func TestBenchmarkWalk_ReportsTheCap(t *testing.T) {
	seed, idx, adj := starGraph(300)
	for _, c := range []struct {
		depth, maxNodes, rendered, within, cut int
	}{
		{1, 200, 200, 301, 101},
		{2, 200, 200, 601, 401},
		{1, 0, 301, 301, 0},
		{2, 0, 601, 601, 0},
	} {
		got := walkQuery(seed, idx, adj, c.depth, c.maxNodes)
		if got.Rendered != c.rendered || got.WithinDepth != c.within || got.Cut != c.cut {
			t.Errorf("depth %d max %d: rendered %d within %d cut %d, want %d %d %d",
				c.depth, c.maxNodes, got.Rendered, got.WithinDepth, got.Cut, c.rendered, c.within, c.cut)
		}
	}
	if d1, d2 := walkQuery(seed, idx, adj, 1, 0), walkQuery(seed, idx, adj, 2, 0); d2.Tokens <= d1.Tokens {
		t.Errorf("uncapped depth 2 tokens %d not above depth 1 tokens %d", d2.Tokens, d1.Tokens)
	}
}

// TestEvalRetrieval_HelpAndMissingPath: the help says what a fixture holds
// and where an example is (the example exists), and a missing fixtures
// directory is named in the error with where a relative path was looked
// for. The default, eval/retrieval, exists only under graph/.
func TestEvalRetrieval_HelpAndMissingPath(t *testing.T) {
	cmd := newEvalRetrievalCmd()
	for _, want := range []string{"probe.tool", "probe.args", "expected.symbols", "scoring.recall_min", "scoring.precision_min", retrievalExampleFixture} {
		if !strings.Contains(cmd.Long, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "..", retrievalExampleFixture)); err != nil {
		t.Errorf("the documented example is missing: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "no-fixtures")
	cmd.SetArgs([]string{"--graph", t.TempDir(), "--fixtures", missing})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("error %v does not name the missing directory %s", err, missing)
	}
	// The help gives a run that could not be made exit code 2; it exited 1.
	var ce exitCodeError
	if !errors.As(err, &ce) || ce.Code != evalRetrievalInternalExit || !strings.Contains(cmd.Long, "2  the run could not be made") {
		t.Errorf("error %v: exit code %d, want %d as the help says", err, ce.Code, evalRetrievalInternalExit)
	}
}

// TestBenchmarkWalkEdges_History: history edges are counted, kept by
// default and left out with --history=false. Through a Commit hub depth 6
// reached the whole graph for every query.
func TestBenchmarkWalkEdges_History(t *testing.T) {
	edges := []types.Edge{
		{Src: "f", Dst: "g", Type: types.EdgeCalls},
		{Src: "f", Dst: "c", Type: types.EdgeChangedIn},
		{Src: "h", Dst: "c", Type: types.EdgeChangedIn},
	}
	kept, n := benchmarkWalkEdges(edges, true)
	if len(kept) != 3 || n != 2 {
		t.Errorf("with history: %d edges kept, %d history; want 3, 2", len(kept), n)
	}
	kept, n = benchmarkWalkEdges(edges, false)
	if len(kept) != 1 || kept[0].Type != types.EdgeCalls || n != 2 {
		t.Errorf("without history: kept %v, %d history; want the calls edge, 2", kept, n)
	}
	f := newBenchmarkCmd().Flags().Lookup("history")
	if f == nil || f.DefValue != fmt.Sprint(defaultBenchmarkHistory) {
		t.Errorf("--history flag %v, want default %v", f, defaultBenchmarkHistory)
	}
}

// TestResolveSourceRoot_OneRule: --src wins, an absolute src_root is used
// as is, a relative one is resolved against the folder holding the graph,
// never the working directory. A relative "src" gave about 37 tokens from
// one folder and 2,304 from another.
func TestResolveSourceRoot_OneRule(t *testing.T) {
	parent := t.TempDir()
	graph := filepath.Join(parent, "graph")
	for _, c := range []struct{ flag, manifest, want string }{
		{"", "/abs/src", "/abs/src"},
		{"", "src", filepath.Join(parent, "src")},
		{parent, "src", parent},
	} {
		got, err := resolveSourceRoot(c.flag, graph, c.manifest)
		if err != nil || got != c.want {
			t.Errorf("resolveSourceRoot(%q, %q) = %q, %v; want %q", c.flag, c.manifest, got, err, c.want)
		}
	}
	if _, err := resolveSourceRoot("", graph, ""); err == nil {
		t.Error("no src_root and no --src should fail")
	}
}

// TestEstimateCorpusTokens_FailsOnMissingSource: a graph file missing under
// the source root is an error naming the root, not a smaller corpus.
func TestEstimateCorpusTokens_FailsOnMissingSource(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes := []types.Node{{Type: types.NodeFile, FilePath: "a.go"}, {Type: types.NodeFile, FilePath: "b.go"}}
	if _, err := estimateCorpusTokens(root, nodes); err == nil || !strings.Contains(err.Error(), "1 of 2") || !strings.Contains(err.Error(), root) {
		t.Errorf("err = %v, want 1 of 2 missing under %s", err, root)
	}
	if n, err := estimateCorpusTokens(root, nodes[:1]); err != nil || n != len("package a\n")/charsPerToken {
		t.Errorf("tokens = %d, %v", n, err)
	}
}

// TestBenchmark_HelpStatesTheCap: the cap is a flag whose default the help
// states.
func TestBenchmark_HelpStatesTheCap(t *testing.T) {
	cmd := newBenchmarkCmd()
	f := cmd.Flags().Lookup("max-nodes")
	if f == nil {
		t.Fatal("no --max-nodes flag")
	}
	if f.DefValue != fmt.Sprint(defaultBenchmarkMaxNodes) {
		t.Errorf("--max-nodes default = %s, want %d", f.DefValue, defaultBenchmarkMaxNodes)
	}
	if !strings.Contains(cmd.Long, "--max-nodes") {
		t.Error("help does not explain --max-nodes")
	}
}
