package persist

import (
	"path/filepath"
	"sort"
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
