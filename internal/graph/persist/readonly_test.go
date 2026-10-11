package persist

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// OpenReadOnly serves finished, immutable graphs. It must refuse writes and
// must not create WAL sidecars next to the file: a reader that leaves or
// races -wal/-shm files breaks retained dataset directories and cleanup.
func TestOpenReadOnlyRejectsWritesAndCreatesNoSidecars(t *testing.T) {
	// Directory names that a plain "path?query" DSN or an unescaped URI
	// would mangle.
	for _, dirName := range []string{"plain", "with space", "hash#and%41", "query?mark"} {
		t.Run(dirName, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), dirName)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			dbPath := filepath.Join(dir, "graph.db")
			rw, err := Open(dbPath) // WAL mode, like a built graph
			if err != nil {
				t.Fatal(err)
			}
			if err := rw.Migrate(); err != nil {
				t.Fatal(err)
			}
			node := types.Node{ID: "abc123def456ghij", Type: types.NodeFunction, Name: "Foo", QualifiedName: "pkg.Foo",
				FilePath: "pkg/foo.go", StartLine: 1, EndLine: 2, Language: "go", Confidence: types.ConfExtracted}
			if err := rw.InsertNodes([]types.Node{node}); err != nil {
				t.Fatal(err)
			}
			if err := rw.Close(); err != nil {
				t.Fatal(err)
			}
			want := listDir(t, dir)

			ro, err := OpenReadOnly(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			var nodes, fk int
			if err := ro.db.QueryRow("SELECT count(*) FROM nodes").Scan(&nodes); err != nil || nodes != 1 {
				t.Fatalf("read: nodes=%d err=%v", nodes, err)
			}
			if err := ro.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
				t.Fatalf("foreign_keys pragma not applied: %d %v", fk, err)
			}
			if _, err := ro.db.Exec("CREATE TABLE must_not_exist(x)"); err == nil {
				t.Fatal("read-only store accepted a write")
			}
			if got := listDir(t, dir); !equal(got, want) {
				t.Fatalf("files while open = %v, want %v", got, want)
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
			if got := listDir(t, dir); !equal(got, want) {
				t.Fatalf("files after close = %v, want %v", got, want)
			}
		})
	}
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Open must create and reopen the database at exactly the given path, also
// when the path contains URI metacharacters.
func TestOpenUsesTheExactPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "query?mark#x")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "graph.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := st.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v", mode, err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database not at the given path: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(dir)); len(entries) != 1 {
		t.Fatalf("stray files beside the directory: %v", entries)
	}
}
