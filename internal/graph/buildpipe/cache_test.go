package buildpipe_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// makeMiniGoModule writes a 3-file Go module to dir and returns the file
// path map (rel → absolute) so tests can mutate individual files. The
// fixture is intentionally small (parses in <100ms) so cache-hit timing
// assertions are meaningful.
func makeMiniGoModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), `module example.com/cachetest

go 1.21
`)
	mustWrite(t, filepath.Join(dir, "a.go"), `package cachetest

// Add returns a + b.
func Add(a, b int) int { return a + b }
`)
	mustWrite(t, filepath.Join(dir, "b.go"), `package cachetest

// Mul returns a * b.
func Mul(a, b int) int { return a * b }
`)
	mustWrite(t, filepath.Join(dir, "c.go"), `package cachetest

// Sub returns a - b.
func Sub(a, b int) int { return a - b }
`)
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// runBuild is a small helper that wraps buildpipe.Run with sane test defaults.
// Returns the resulting Manifest so tests can assert FileEntry contents.
func runBuild(t *testing.T, src, out string, opts ...func(*buildpipe.Options)) persist.Manifest {
	t.Helper()
	o := buildpipe.Options{
		SrcRoot: src, OutDir: out,
		Languages: []string{"auto"}, CKGVersion: "test",
	}
	for _, fn := range opts {
		fn(&o)
	}
	m, err := buildpipe.Run(o)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return m
}

// TestRebuildIsCold: every build is cold. A build over an existing graph
// after an edit gives the graph a fresh build of the same tree gives, with
// --no-cache or without (it changes nothing), and a removed file is gone.
func TestRebuildIsCold(t *testing.T) {
	src := makeMiniGoModule(t)
	out := t.TempDir()
	runBuild(t, src, out)
	mustWrite(t, filepath.Join(src, "a.go"), `package cachetest

// Add returns a + b, and calls Mul to tie two files together.
func Add(a, b int) int { return a + b + Mul(0, 0) }
`)
	if err := os.Remove(filepath.Join(src, "c.go")); err != nil {
		t.Fatal(err)
	}
	over := runBuild(t, src, out)
	fresh := runBuild(t, src, t.TempDir())
	noCache := runBuild(t, src, t.TempDir(), func(o *buildpipe.Options) { o.NoCache = true })
	if over.GraphDigest != fresh.GraphDigest || noCache.GraphDigest != fresh.GraphDigest {
		t.Errorf("graph digests differ: over an old graph %s, fresh %s, --no-cache %s", over.GraphDigest, fresh.GraphDigest, noCache.GraphDigest)
	}
	if manifestHasPath(over, "c.go") {
		t.Errorf("removed file still in the manifest: %v", manifestPaths(over))
	}
	if !equalStringSlices(manifestPaths(over), manifestPaths(fresh)) {
		t.Errorf("manifest files differ: %v vs %v", manifestPaths(over), manifestPaths(fresh))
	}
}

// helper: assert two string slices are equal as sets (sorted compare).
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string{}, a...)
	y := append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func manifestPaths(m persist.Manifest) []string {
	out := make([]string, 0, len(m.Files))
	for _, e := range m.Files {
		out = append(out, e.Path)
	}
	sort.Strings(out)
	return out
}

func manifestHasPath(m persist.Manifest, path string) bool {
	for _, e := range m.Files {
		if e.Path == path {
			return true
		}
	}
	return false
}

// TestRebuild_PreservesCrossFileEdges is the regression
// guard for a partial-cache bug: previously, when a callee file was modified
// (cached caller still pointing into it), the partial-cache routing in
// runIncremental dropped the cross-file `calls` edge because cached files
// don't re-emit pending refs. Empirical proof: testdata/synthetic vault.go
// modify took cross-file calls 2 → 0.
//
// The fix removes the partial-cache routing — any non-full-hit case now
// falls back to runCold, which re-runs Pass 2 in full. This test pins
// the contract: cross-file calls survive a callee-side modification.
//
// Module shape:
//
//	caller.go: func Use() { Helper() }
//	callee.go: func Helper() {}
//
// Cold build → 1 cross-file `calls` edge (caller.Use → callee.Helper).
// Modify callee.go → rebuild. Expect: still 1 cross-file `calls` edge.
func TestRebuild_PreservesCrossFileEdges(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), `module example.com/edgepin

go 1.21
`)
	mustWrite(t, filepath.Join(dir, "caller.go"), `package edgepin

// Use invokes Helper from the same package, different file.
func Use() { Helper() }
`)
	calleePath := filepath.Join(dir, "callee.go")
	mustWrite(t, calleePath, `package edgepin

// Helper does nothing. The point is that Use() targets it from caller.go.
func Helper() {}
`)
	out := t.TempDir()

	// Cold build — establish baseline cross-file calls count.
	first := runBuild(t, dir, out)
	if !manifestHasPath(first, "caller.go") || !manifestHasPath(first, "callee.go") {
		t.Fatalf("expected both files in first manifest, got %v", manifestPaths(first))
	}
	coldCalls := countCrossFileCallsEdges(t, filepath.Join(out, "graph.db"))
	if coldCalls < 1 {
		t.Fatalf("cold build must emit ≥1 cross-file calls edge, got %d", coldCalls)
	}

	// Modify callee.go (callee-side change; caller.go remains cached).
	// Without the cold-fallback fix this rebuild would silently drop
	// caller.Use → callee.Helper, taking the cross-file calls count to 0.
	mustWrite(t, calleePath, `package edgepin

// Helper does nothing — comment edited to invalidate this file's cache.
func Helper() {}
`)
	runBuild(t, dir, out)
	warmCalls := countCrossFileCallsEdges(t, filepath.Join(out, "graph.db"))
	if warmCalls != coldCalls {
		t.Errorf("cross-file calls drifted on partial-cache rebuild: cold=%d warm=%d (regression: partial-cache silent edge loss)",
			coldCalls, warmCalls)
	}
}

// TestRebuild_ImplementsEdgesNoDrift (formerly an incremental test) is the regression guard for the
// incremental-path implements/extends bug: cold builds emit implements via
// the post-Resolve EmitImplementsEdges pass, but runGoPipelineIncremental
// previously did not, AND the runIncremental DROP step did not include the
// "implements"/"extends" types. Net effect: the first incremental rebuild
// after touching any Go file silently dropped the row count to zero (no
// re-emission) OR doubled it (no DROP), depending on call order.
//
// Module shape:
//
//	greeter.go: type Greeter interface { Greet() string }
//	hello.go:   type Hello struct{}; func (Hello) Greet() string { ... }
//
// Cold build → 1 implements edge (hello.Hello → greeter.Greeter — same pkg
// here, but the same code path drives cross-pkg satisfaction). Touch
// hello.go to dirty it → second build is incremental. Implements count
// must equal cold count (no drift, no duplicates).
func TestRebuild_ImplementsEdgesNoDrift(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), `module example.com/implpin

go 1.21
`)
	mustWrite(t, filepath.Join(dir, "greeter.go"), `package implpin

// Greeter is satisfied by Hello below.
type Greeter interface {
	Greet() string
}
`)
	helloPath := filepath.Join(dir, "hello.go")
	mustWrite(t, helloPath, `package implpin

// Hello satisfies Greeter via a value-receiver method.
type Hello struct{}

func (h Hello) Greet() string { return "hi" }
`)
	out := t.TempDir()

	// Cold build — establishes baseline implements count.
	_ = runBuild(t, dir, out)
	dbPath := filepath.Join(out, "graph.db")
	coldImpl := countEdgesByType(t, dbPath, "implements")
	if coldImpl < 1 {
		t.Fatalf("cold build must emit ≥1 implements edge, got %d", coldImpl)
	}

	// Incremental rebuild — touch hello.go's mtime AND content (comment edit)
	// to force the cache classifier to mark it dirty.
	mustWrite(t, helloPath, `package implpin

// Hello satisfies Greeter via a value-receiver method (comment edited
// to invalidate this file's cache for the incremental drift test).
type Hello struct{}

func (h Hello) Greet() string { return "hi" }
`)
	runBuild(t, dir, out)
	incImpl := countEdgesByType(t, dbPath, "implements")
	if incImpl != coldImpl {
		t.Errorf("implements drift on incremental rebuild: cold=%d inc=%d (regression: incremental path missing EmitImplementsEdges or DROP)",
			coldImpl, incImpl)
	}
}

// countEdgesByType opens the SQLite graph at dbPath read-only and returns
// the number of edges of the given type. Used by edge-preservation tests.
func countEdgesByType(t *testing.T, dbPath, edgeType string) int {
	t.Helper()
	store, err := persist.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("open db %s: %v", dbPath, err)
	}
	defer func() { _ = store.Close() }()
	edges, err := store.QueryEdgesByType(edgeType)
	if err != nil {
		t.Fatalf("query %s edges: %v", edgeType, err)
	}
	return len(edges)
}

// countCrossFileCallsEdges queries the SQLite graph for `calls` edges whose
// src and dst nodes live in different files. Used by edge-preservation tests.
func countCrossFileCallsEdges(t *testing.T, dbPath string) int {
	t.Helper()
	store, err := persist.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("open db %s: %v", dbPath, err)
	}
	defer func() { _ = store.Close() }()
	// QueryEdgesByType doesn't filter by file — fetch all calls, then
	// look up each endpoint's file_path and count cross-file pairs.
	edges, err := store.QueryEdgesByType("calls")
	if err != nil {
		t.Fatalf("query calls edges: %v", err)
	}
	if len(edges) == 0 {
		return 0
	}
	idSet := map[string]bool{}
	for _, e := range edges {
		idSet[e.Src] = true
		idSet[e.Dst] = true
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	nodes, err := store.NodesByIDs(ids)
	if err != nil {
		t.Fatalf("nodes by ids: %v", err)
	}
	pathByID := map[string]string{}
	for _, n := range nodes {
		pathByID[n.ID] = n.FilePath
	}
	cross := 0
	for _, e := range edges {
		if pathByID[e.Src] != "" && pathByID[e.Dst] != "" &&
			pathByID[e.Src] != pathByID[e.Dst] {
			cross++
		}
	}
	return cross
}
