package buildpipe

import (
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func TestReviewTestFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fn := func(file string) types.Node { return types.Node{Type: types.NodeFunction, FilePath: file} }
	imp := func(from, path string) types.Node {
		return types.Node{Type: types.NodeImport, Name: path, FilePath: from}
	}
	nodes := []types.Node{
		fn("p/a.go"), fn("p/a_test.go"),
		fn("internal/testlog/log.go"), fn("internal/h2/h.go"),
		fn("pkg/lib/lib.go"), fn("internal/used/u.go"), fn("cmd/tool/main.go"),
		imp("p/a_test.go", "example.com/m/internal/testlog"),
		imp("internal/testlog/log.go", "example.com/m/internal/h2"), // helper of a helper
		imp("p/a_test.go", "example.com/m/pkg/lib"),                 // public: may have outside callers
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
	t.Parallel()
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

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// End to end: a built graph answers a review with production callers back to
// the entry point (through interface dispatch), call sites in order, the
// commits that changed the function, and test callers listed but not walked.
func TestReviewOnABuiltGraph(t *testing.T) {
	src := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/r\n\ngo 1.22\n")
	lookup := func(guard string) string {
		return "package p\n\ntype Store interface{ Get(k string) string }\n\ntype mem struct{ m map[string]string }\n\nfunc (s *mem) Get(k string) string { return s.m[k] }\n\n" +
			"func Lookup(s Store, k string) string {\n\tif k == \"" + guard + "\" {\n\t\treturn \"\"\n\t}\n\treturn s.Get(k)\n}\n\n" +
			"func Handler(k string) string { return Lookup(&mem{m: map[string]string{}}, k) }\n\nfunc Main() { _ = Handler(\"x\") }\n"
	}
	write("p/p.go", lookup(""))
	write("internal/testlog/log.go", "package testlog\n\nfunc Log(s string) string { return s }\n")
	write("p/p_test.go", "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.com/r/internal/testlog\"\n)\n\nfunc TestLookup(t *testing.T) { testlog.Log(Lookup(&mem{}, \"a\")) }\n")
	gitRun(t, src, "init", "-q")
	gitRun(t, src, "add", ".")
	gitRun(t, src, "commit", "-qm", "add lookup")
	write("p/p.go", lookup("-"))
	gitRun(t, src, "commit", "-qam", "reject the dash key")

	out := t.TempDir()
	if _, err := Run(Options{SrcRoot: src, OutDir: out, Languages: []string{"go"}, CKGVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	st, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	r, err := st.Review("p.Lookup", persist.ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	names := func(xs []persist.ReviewSymbol) []string {
		var out []string
		for _, x := range xs {
			out = append(out, x.QualifiedName)
		}
		return out
	}
	if got := names(r.Callers); !reflect.DeepEqual(got, []string{"p.Handler"}) {
		t.Errorf("callers = %v", got)
	}
	if len(r.CallerPaths) != 1 || strings.Join(r.CallerPaths[0], " -> ") != "p.Main -> p.Handler -> p.Lookup" {
		t.Errorf("caller paths = %v", r.CallerPaths)
	}
	if got := names(r.TestCallers); !reflect.DeepEqual(got, []string{"p.TestLookup"}) || r.TestCallersTotal != 1 {
		t.Errorf("test callers = %v", got)
	}
	if r.Control["IfStmt"] != 1 || r.ReturnStatements != 2 {
		t.Errorf("control = %v returns = %d", r.Control, r.ReturnStatements)
	}
	var calls []string
	for _, c := range r.CallSites {
		calls = append(calls, c.Text)
	}
	if !slices.Contains(calls, "s.Get(k)") {
		t.Errorf("call sites = %v", calls)
	}
	if impl := r.InterfaceImplementers["p.Store.Get"]; !reflect.DeepEqual(impl, []string{"p.mem"}) {
		t.Errorf("implementers = %v", r.InterfaceImplementers)
	}
	var subjects []string
	for _, c := range r.Commits {
		subjects = append(subjects, c.Subject)
	}
	if !slices.Contains(subjects, "reject the dash key") {
		t.Errorf("commits = %v", subjects)
	}

	// Interface dispatch: the concrete method's callers come through the
	// interface method it implements.
	g, err := st.Review("p.mem.Get", persist.ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(g.ViaInterfaces); !reflect.DeepEqual(got, []string{"p.Store.Get"}) {
		t.Errorf("via interfaces = %v", got)
	}
	if !slices.Contains(names(g.Callers), "p.Lookup") {
		t.Errorf("mem.Get callers = %v", names(g.Callers))
	}

	// Test code and test-only helpers are not reviewable seeds.
	for _, q := range []string{"p.TestLookup", "testlog.Log"} {
		if _, err := st.Review(q, persist.ReviewOptions{}); !errors.Is(err, persist.ErrReviewSeedNotFound) {
			t.Errorf("review %s: %v", q, err)
		}
	}
}
