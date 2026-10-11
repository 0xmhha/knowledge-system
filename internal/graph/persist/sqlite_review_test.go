package persist

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func reviewFixture(t *testing.T) *sqliteStore {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	n := func(id string, typ types.NodeType, q, file string) types.Node {
		return types.Node{ID: id, Type: typ, Name: q, QualifiedName: q, FilePath: file, StartLine: 1, EndLine: 2, Language: "go", Confidence: types.ConfExtracted}
	}
	e := func(src, dst string, typ types.EdgeType) types.Edge {
		return types.Edge{Src: src, Dst: dst, Type: typ, Count: 1, Confidence: types.ConfExtracted}
	}
	nodes := []types.Node{
		n("fileaaaaaaaaaaaa", types.NodeFile, "p/a.go", "p/a.go"),
		n("prodaaaaaaaaaaaa", types.NodeFunction, "p.A", "p/a.go"),
		n("prodbbbbbbbbbbbb", types.NodeFunction, "p.B", "p/a.go"),
		n("testaaaaaaaaaaaa", types.NodeFunction, "p.TestA", "p/a_test.go"),
		n("helperaaaaaaaaaa", types.NodeFunction, "testlog.New", "internal/testlog/log.go"),
		n("structaaaaaaaaaa", types.NodeStruct, "p.S", "p/a.go"),
		n("methodaaaaaaaaaa", types.NodeMethod, "p.S.M", "p/a.go"),
		n("commitaaaaaaaaaa", types.NodeCommit, "commit:aaaa", ""),
		n("hunkaaaaaaaaaaaa", types.NodeHunk, "hunk:aaaa:p/a.go:0", "p/a.go"),
	}
	edges := []types.Edge{
		e("prodaaaaaaaaaaaa", "prodbbbbbbbbbbbb", types.EdgeCalls),
		e("testaaaaaaaaaaaa", "prodaaaaaaaaaaaa", types.EdgeCalls),
		e("prodaaaaaaaaaaaa", "helperaaaaaaaaaa", types.EdgeCalls),
		e("structaaaaaaaaaa", "methodaaaaaaaaaa", types.EdgeDefines),
		e("fileaaaaaaaaaaaa", "prodaaaaaaaaaaaa", types.EdgeDefines),
		e("prodaaaaaaaaaaaa", "commitaaaaaaaaaa", types.EdgeChangedIn),
		e("commitaaaaaaaaaa", "hunkaaaaaaaaaaaa", types.EdgeHasHunk),
		e("hunkaaaaaaaaaaaa", "prodaaaaaaaaaaaa", types.EdgeModifies),
	}
	if err := st.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertEdges(edges); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestRebuildReviewIndex(t *testing.T) {
	st := reviewFixture(t)
	for round := 0; round < 2; round++ { // idempotent
		if err := st.RebuildReviewIndex([]string{"p/a_test.go", "internal/testlog/log.go"}); err != nil {
			t.Fatal(err)
		}
	}
	var tests []string
	rows, err := st.db.Query(`SELECT qualified_name FROM nodes WHERE is_test = 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var q string
		_ = rows.Scan(&q)
		tests = append(tests, q)
	}
	_ = rows.Close()
	sort.Strings(tests)
	if len(tests) != 2 || tests[0] != "p.TestA" || tests[1] != "testlog.New" {
		t.Fatalf("is_test nodes = %v", tests)
	}
	var code []string
	rows, err = st.db.Query(`SELECT s.qualified_name || ' ' || c.type || ' ' || d.qualified_name FROM code_edges c JOIN nodes s ON s.id = c.src JOIN nodes d ON d.id = c.dst ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var x string
		_ = rows.Scan(&x)
		code = append(code, x)
	}
	_ = rows.Close()
	want := []string{"p.A calls p.B", "p.S defines p.S.M"}
	if len(code) != len(want) || code[0] != want[0] || code[1] != want[1] {
		t.Fatalf("code_edges = %v, want %v (no test, helper, file-defines or history edges)", code, want)
	}
	var node, commit string
	if err := st.db.QueryRow(`SELECT n.qualified_name, c.qualified_name FROM node_commits x JOIN nodes n ON n.id = x.node_id JOIN nodes c ON c.id = x.commit_id`).Scan(&node, &commit); err != nil || node != "p.A" || commit != "commit:aaaa" {
		t.Fatalf("node_commits = %s %s %v", node, commit, err)
	}
	// A node deletion cascades out of both derived tables.
	if err := st.DeleteNodesByFilePath("p/a.go"); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = st.db.QueryRow(`SELECT (SELECT count(*) FROM code_edges) + (SELECT count(*) FROM node_commits)`).Scan(&left)
	if left != 0 {
		t.Fatalf("%d derived rows survived the file delete", left)
	}
}

// The seed's statements come from its own contains edges, not a scan of all
// of them.
func TestReviewContainsQueryUsesTheSourceIndex(t *testing.T) {
	st := reviewFixture(t)
	rows, err := st.db.Query("EXPLAIN QUERY PLAN "+reviewContainsQuery, "x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var plan []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		_ = rows.Scan(&id, &parent, &notused, &detail)
		plan = append(plan, detail)
	}
	if p := strings.Join(plan, "; "); !strings.Contains(p, "idx_edges_src") {
		t.Fatalf("contains plan = %q", p)
	}
}

// A widely used function has thousands of transitive callers; the response
// keeps 40 callers and 12 paths, so the walk stops at a budget and says so.
func TestReviewCallerWalkIsBounded(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	fn := func(id, q string) types.Node {
		return types.Node{ID: id, Type: types.NodeFunction, Name: q, QualifiedName: q, FilePath: "p/a.go", StartLine: 1, EndLine: 2, Language: "go", Confidence: types.ConfExtracted}
	}
	nodes := []types.Node{fn("seed000000000000", "p.Hot")}
	var edges []types.Edge
	// Two levels: 60 direct callers, each called by 60 others (3,660 callers).
	for i := 0; i < 60; i++ {
		d := fmt.Sprintf("d%015d", i)
		nodes = append(nodes, fn(d, fmt.Sprintf("p.D%d", i)))
		edges = append(edges, types.Edge{Src: d, Dst: "seed000000000000", Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted})
		for j := 0; j < 60; j++ {
			u := fmt.Sprintf("u%07d%08d", i, j)
			nodes = append(nodes, fn(u, fmt.Sprintf("p.U%d_%d", i, j)))
			edges = append(edges, types.Edge{Src: u, Dst: d, Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted})
		}
	}
	if err := st.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertEdges(edges); err != nil {
		t.Fatal(err)
	}
	if err := st.RebuildReviewIndex(nil); err != nil {
		t.Fatal(err)
	}
	r, err := st.Review("p.Hot", ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.CallerWalkTruncated || len(r.Callers) != 40 || len(r.CallerPaths) != 12 {
		t.Fatalf("truncated=%v callers=%d paths=%d", r.CallerWalkTruncated, len(r.Callers), len(r.CallerPaths))
	}
	for _, p := range r.CallerPaths {
		if p[len(p)-1] != "p.Hot" || p[0] == "" {
			t.Fatalf("path %v", p)
		}
	}
	// A narrow function is walked to its entry points without truncation.
	small, err := st.Review("p.D0", ReviewOptions{})
	if err != nil || small.CallerWalkTruncated || len(small.Callers) != 40 {
		t.Fatalf("p.D0: truncated=%v callers=%d %v", small.CallerWalkTruncated, len(small.Callers), err)
	}
}

// With many frontier IDs SQLite chose idx_nodes_type and read every function
// and method per level (240 ms per review on the go-stablenet graph); the
// walk must stay on idx_code_edges_dst.
func TestReviewCallerQueryUsesTheDestinationIndex(t *testing.T) {
	st := reviewFixture(t)
	ids := make([]any, 300)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%d", i)
	}
	rows, err := st.db.Query("EXPLAIN QUERY PLAN "+reviewCallersQuery(len(ids)), ids...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var plan []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		_ = rows.Scan(&id, &parent, &notused, &detail)
		plan = append(plan, detail)
	}
	p := strings.Join(plan, "; ")
	if !strings.Contains(p, "idx_code_edges_dst") || strings.Contains(p, "idx_nodes_type") {
		t.Fatalf("caller walk plan = %q", p)
	}
}
