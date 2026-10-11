package buildpipe

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func TestReviewTestFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fn := func(file string) types.Node { return types.Node{Type: types.NodeFunction, FilePath: file} }
	imp := func(from, path string) types.Node { return types.Node{Type: types.NodeImport, Name: path, FilePath: from} }
	nodes := []types.Node{
		fn("p/a.go"), fn("p/a_test.go"),
		fn("internal/testlog/log.go"), fn("internal/h2/h.go"),
		fn("pkg/lib/lib.go"), fn("internal/used/u.go"), fn("cmd/tool/main.go"),
		imp("p/a_test.go", "example.com/m/internal/testlog"),
		imp("internal/testlog/log.go", "example.com/m/internal/h2"), // helper of a helper
		imp("p/a_test.go", "example.com/m/pkg/lib"),                // public: may have outside callers
		imp("p/a_test.go", "example.com/m/internal/used"),
		imp("p/a.go", "example.com/m/internal/used"),
		imp("p/a_test.go", "golang.org/x/internal/testlog"), // another module
	}
	got := reviewTestFiles(root, nodes)
	want := []string{"internal/h2/h.go", "internal/testlog/log.go", "p/a_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("test files = %v, want %v", got, want)
	}
	// Without go.mod only the path rule applies.
	if got := reviewTestFiles(t.TempDir(), nodes); !reflect.DeepEqual(got, []string{"p/a_test.go"}) {
		t.Fatalf("without go.mod = %v", got)
	}
}

// A cold build stores the review index.
func TestColdBuildWritesReviewIndex(t *testing.T) {
	out := t.TempDir()
	if _, err := Run(Options{SrcRoot: "../parse/golang/testdata/resolve", OutDir: out, Languages: []string{"auto"}, CKGVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(out, "graph.db")+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var codeEdges, calls int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM code_edges WHERE type = 'calls'), (SELECT count(*) FROM edges e JOIN nodes s ON s.id = e.src JOIN nodes d ON d.id = e.dst WHERE e.type = 'calls' AND s.is_test = 0 AND d.is_test = 0)`).Scan(&codeEdges, &calls); err != nil {
		t.Fatal(err)
	}
	if codeEdges == 0 || codeEdges != calls {
		t.Fatalf("code_edges calls = %d, production calls = %d", codeEdges, calls)
	}
	var version string
	if err := db.QueryRow(`SELECT value FROM manifest WHERE key = 'schema_version'`).Scan(&version); err != nil || version != SchemaVersion {
		t.Fatalf("schema_version = %q, %v", version, err)
	}
}
