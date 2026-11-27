package buildpipe

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/filterlist"
	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestEmitTemporalEdges_BasicGitRepo wires the full temporal path on a
// one-commit repo with a Package, a File and a Function node:
//
//   - One Commit node, carrying its subject and times.
//   - changed_in and blame from the File and the Function (the kinds whose
//     line history is recorded) to the commit; none from the Package.
//   - One Hunk (the whole new file) that modifies the Function.
func TestEmitTemporalEdges_BasicGitRepo(t *testing.T) {
	repo := initGitRepo(t)
	relPath := "main.go"
	commitFileToRepo(t, repo, relPath, "package main\n\nfunc Hello() {}\n", "initial")

	g := buildSyntheticGraph(relPath)
	if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{}, nil); err != nil {
		t.Fatalf("emitTemporalEdges: %v", err)
	}
	commits := nodesByType(g.Nodes, types.NodeCommit)
	if len(commits) != 1 {
		t.Fatalf("expected 1 Commit node, got %d", len(commits))
	}
	c := commits[0]
	if !strings.HasPrefix(c.QualifiedName, "commit:") || c.SubKind != "git" || c.Language != "git" {
		t.Errorf("Commit node shape = %q/%q/%q", c.QualifiedName, c.SubKind, c.Language)
	}
	if c.Commit == nil || c.Commit.Subject != "initial" || c.Commit.AuthorTime == 0 || c.Commit.CommitTime == 0 {
		t.Errorf("Commit metadata = %+v", c.Commit)
	}
	srcs := func(et types.EdgeType) []string {
		var out []string
		for _, e := range edgesByType(g.Edges, et) {
			out = append(out, e.Src)
		}
		slices.Sort(out)
		return out
	}
	want := []string{"file0000aaaa2222", "func0000bbbb3333"}
	if got := srcs(types.EdgeChangedIn); !slices.Equal(got, want) {
		t.Errorf("changed_in sources = %v, want %v (File and Function, not Package)", got, want)
	}
	if got := srcs(types.EdgeBlame); !slices.Equal(got, want) {
		t.Errorf("blame sources = %v, want %v", got, want)
	}
	for _, e := range edgesByType(g.Edges, types.EdgeBlame) {
		if e.Src == "file0000aaaa2222" && e.Count != 3 {
			t.Errorf("File blame covers %d lines, want 3", e.Count)
		}
	}
	if n := len(nodesByType(g.Nodes, types.NodeHunk)); n != 1 {
		t.Errorf("expected 1 Hunk for the newly added file, got %d", n)
	}
	if got := srcsOf(edgesByType(g.Edges, types.EdgeModifies), true); !slices.Equal(got, []string{"func0000bbbb3333"}) {
		t.Errorf("modifies targets = %v, want the Function", got)
	}
	if err := graph.Validate(g); err != nil {
		t.Errorf("graph.Validate after temporal: %v", err)
	}
}

// srcsOf lists the sources (or, with dst, the destinations) of edges, sorted.
func srcsOf(es []types.Edge, dst bool) []string {
	var out []string
	for _, e := range es {
		if dst {
			out = append(out, e.Dst)
		} else {
			out = append(out, e.Src)
		}
	}
	slices.Sort(out)
	return out
}

// TestEmitTemporalEdges_NotAGitRepo verifies graceful degrade: the function
// returns nil and the graph is untouched.
func TestEmitTemporalEdges_NotAGitRepo(t *testing.T) {
	dir := t.TempDir() // not a git checkout
	g := buildSyntheticGraph("main.go")
	beforeNodes := len(g.Nodes)
	beforeEdges := len(g.Edges)
	if _, err := emitTemporalEdges(g, dir, discardLog(), HistoryOptions{}, nil); err != nil {
		t.Fatalf("expected nil err on non-git src, got %v", err)
	}
	if len(g.Nodes) != beforeNodes || len(g.Edges) != beforeEdges {
		t.Errorf("graph was mutated for non-git src: nodes %d→%d edges %d→%d",
			beforeNodes, len(g.Nodes), beforeEdges, len(g.Edges))
	}
}

// symbolHistoryRepo commits a file whose two functions change at different
// times, then removes lines above both, so that today's line numbers no
// longer match those of the older commits:
//
//	c1  add: header comments (lines 2-7), G (8-10), F (11-13)
//	c2  edit G's body (line 9)
//	c3  delete the header comments: G is now 2-4, F 5-7
func symbolHistoryRepo(t *testing.T) (repo, c1, c2, c3 string) {
	t.Helper()
	repo = initGitRepo(t)
	head := "package a\n// c1\n// c2\n// c3\n// c4\n// c5\n// c6\n"
	c1 = commitFileToRepo(t, repo, "a.go", head+"func G() {\n\tg1()\n}\nfunc F() {\n\tf1()\n}\n", "add G and F")
	c2 = commitFileToRepo(t, repo, "a.go", head+"func G() {\n\tg2()\n}\nfunc F() {\n\tf1()\n}\n", "edit G")
	c3 = commitFileToRepo(t, repo, "a.go", "package a\nfunc G() {\n\tg2()\n}\nfunc F() {\n\tf1()\n}\n", "drop the header")
	return repo, c1, c2, c3
}

// symbolHistoryGraph is today's graph of symbolHistoryRepo's file.
func symbolHistoryGraph() *graph.Graph {
	node := func(id string, nt types.NodeType, name string, a, b int) types.Node {
		return types.Node{ID: id, Type: nt, Name: name, QualifiedName: "a." + name, FilePath: "a.go",
			StartLine: a, EndLine: b, EndByte: 1, Language: "go", Confidence: types.ConfExtracted}
	}
	return &graph.Graph{Nodes: []types.Node{
		node("file0000000000aa", types.NodeFile, "a.go", 1, 7),
		node("funcG00000000000", types.NodeFunction, "G", 2, 4),
		node("funcF00000000000", types.NodeFunction, "F", 5, 7),
	}}
}

// edgeShas maps each (src, edge type) to the commits its edges reach.
func edgeShas(g *graph.Graph, et types.EdgeType, src string) []string {
	var out []string
	for _, e := range g.Edges {
		if e.Type == et && e.Src == src {
			out = append(out, strings.TrimPrefix(nodeQname(g.Nodes, e.Dst), "commit:"))
		}
	}
	slices.Sort(out)
	return out
}

func sortedOf(s ...string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// TestEmitTemporalEdges_ChangedInFollowsTheSymbolsLines: a declaration's
// changed_in edges are the commits that changed its own lines (located at
// each commit), not every commit of its file; the file's edges are the
// commits of the file.
func TestEmitTemporalEdges_ChangedInFollowsTheSymbolsLines(t *testing.T) {
	repo, c1, c2, c3 := symbolHistoryRepo(t)
	g := symbolHistoryGraph()
	if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		src  string
		want []string
	}{
		{"funcG00000000000", sortedOf(c1, c2)},
		{"funcF00000000000", sortedOf(c1)},
		{"file0000000000aa", sortedOf(c1, c2, c3)},
	} {
		if got := edgeShas(g, types.EdgeChangedIn, tc.src); !slices.Equal(got, tc.want) {
			t.Errorf("changed_in of %s = %v, want %v", tc.src, got, tc.want)
		}
	}
}

// TestEmitTemporalEdges_BlameIsPerLine: blame edges name the commits that
// last changed the node's lines today, with the number of lines.
func TestEmitTemporalEdges_BlameIsPerLine(t *testing.T) {
	repo, c1, c2, _ := symbolHistoryRepo(t)
	g := symbolHistoryGraph()
	if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	lines := map[string]map[string]int{}
	for _, e := range edgesByType(g.Edges, types.EdgeBlame) {
		if lines[e.Src] == nil {
			lines[e.Src] = map[string]int{}
		}
		lines[e.Src][strings.TrimPrefix(nodeQname(g.Nodes, e.Dst), "commit:")] = e.Count
	}
	want := map[string]map[string]int{
		"funcG00000000000": {c1: 2, c2: 1},
		"funcF00000000000": {c1: 3},
		"file0000000000aa": {c1: 6, c2: 1},
	}
	for src, w := range want {
		if got := lines[src]; len(got) != len(w) || got[c1] != w[c1] || got[c2] != w[c2] {
			t.Errorf("blame of %s = %v, want %v", src, got, w)
		}
	}
}

// TestEmitTemporalEdges_ModifiesUsesChangedLinesAtTheCommit: a hunk modifies
// a declaration when a line it changed (not a context line) lies in the
// declaration as it was at the hunk's commit. c2's hunk, whose context lines
// reach today's F, modifies G only; c3's deletion above both modifies none.
func TestEmitTemporalEdges_ModifiesUsesChangedLinesAtTheCommit(t *testing.T) {
	repo, c1, c2, c3 := symbolHistoryRepo(t)
	g := symbolHistoryGraph()
	if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, e := range edgesByType(g.Edges, types.EdgeModifies) {
		sha := strings.SplitN(strings.TrimPrefix(nodeQname(g.Nodes, e.Src), "hunk:"), ":", 2)[0]
		got[sha] = append(got[sha], e.Dst)
	}
	for sha := range got {
		slices.Sort(got[sha])
	}
	want := map[string][]string{
		c1: {"funcF00000000000", "funcG00000000000"},
		c2: {"funcG00000000000"},
	}
	if len(got[c3]) != 0 || !slices.Equal(got[c1], want[c1]) || !slices.Equal(got[c2], want[c2]) {
		t.Errorf("modifies by commit = %v, want %v and none for %s", got, want, c3)
	}
}

// TestEmitTemporalEdges_DepthCapsChangedIn keeps the newest Depth commits
// per node when a cap is set.
func TestEmitTemporalEdges_DepthCapsChangedIn(t *testing.T) {
	repo := initGitRepo(t)
	rel := "y.go"
	for i := 0; i < 5; i++ {
		commitFileToRepo(t, repo, rel, "package y\n//"+strings.Repeat("edit ", i+1)+"\n", "edit")
	}
	g := buildSyntheticGraph(rel)
	if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{Depth: 2}, nil); err != nil {
		t.Fatalf("emitTemporalEdges: %v", err)
	}
	if got := len(edgeShas(g, types.EdgeChangedIn, "file0000aaaa2222")); got != 2 {
		t.Errorf("File has %d changed_in edges under Depth 2, want 2", got)
	}
}

// --- helpers ---

// buildSyntheticGraph hand-constructs a 3-node Graph (Package + File +
// Function) all sharing the same FilePath. Mirrors what graph.Build would
// emit for a single Go file. Avoids spinning up the real parser inside this
// test — the temporal pass is parser-agnostic, it just walks Node.FilePath.
func buildSyntheticGraph(relPath string) *graph.Graph {
	pkgID := "pkg00000000aaaa11"
	fileID := "file0000aaaa2222"
	funcID := "func0000bbbb3333"
	return &graph.Graph{
		Nodes: []types.Node{
			{
				ID: pkgID, Type: types.NodePackage, Name: "main", QualifiedName: "main",
				FilePath: relPath, StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 1,
				Language: "go", Confidence: types.ConfExtracted,
			},
			{
				ID: fileID, Type: types.NodeFile, Name: relPath, QualifiedName: "main/" + relPath,
				FilePath: relPath, StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 1,
				Language: "go", Confidence: types.ConfExtracted,
			},
			{
				ID: funcID, Type: types.NodeFunction, Name: "Hello", QualifiedName: "main.Hello",
				FilePath: relPath, StartLine: 3, EndLine: 3, StartByte: 14, EndByte: 32,
				Language: "go", Confidence: types.ConfExtracted,
			},
		},
		Edges: []types.Edge{
			{Src: pkgID, Dst: fileID, Type: types.EdgeContains, FilePath: relPath, Count: 1, Confidence: types.ConfExtracted},
			{Src: fileID, Dst: funcID, Type: types.EdgeDefines, FilePath: relPath, Count: 1, Confidence: types.ConfExtracted},
		},
	}
}

func nodesByType(ns []types.Node, t types.NodeType) []types.Node {
	out := []types.Node{}
	for _, n := range ns {
		if n.Type == t {
			out = append(out, n)
		}
	}
	return out
}

func edgesByType(es []types.Edge, t types.EdgeType) []types.Edge {
	out := []types.Edge{}
	for _, e := range es {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func nodeQname(ns []types.Node, id string) string {
	for _, n := range ns {
		if n.ID == id {
			return n.QualifiedName
		}
	}
	return ""
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// initGitRepo / commitFileToRepo are renamed copies of staleness_test.go's
// helpers — kept here so this test file is self-contained (the package's
// test files live in two packages: external `_test` and internal).
// TODO: factor a shared `internal/buildpipe/internal/testgit` once a third
// callsite appears (YAGNI for now).
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runCmd(t, dir, "git", "init", "-q", "-b", "main")
	runCmd(t, dir, "git", "config", "user.email", "ckg-test@example.com")
	runCmd(t, dir, "git", "config", "user.name", "ckg-test")
	runCmd(t, dir, "git", "config", "commit.gpgsign", "false")
	return dir
}

func commitFileToRepo(t *testing.T, repo, relPath, content, msg string) string {
	t.Helper()
	full := filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runCmd(t, repo, "git", "add", "--", relPath)
	runCmd(t, repo, "git", "commit", "-q", "-m", msg)
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

// TestEmitTemporalEdges_FilterScopesHunks pins the build-scope contract on
// the temporal axis. git history is enumerated independently of the
// discovery pass, so without the filter the graph collects Hunk nodes for
// files the dataset excludes — nodes no symbol, body or convention backs,
// which a retrieval surface can still cite. The leak was silent because
// only hunks carry the out-of-scope path; changed_in and blame resolve
// through the graph's own nodes and cannot reach outside.
func TestEmitTemporalEdges_FilterScopesHunks(t *testing.T) {
	repo := initGitRepo(t)
	commitFileToRepo(t, repo, "in_scope.go", "package main\n\nfunc Hello() {}\n", "in scope")
	commitFileToRepo(t, repo, "out_of_scope.go", "package main\n\nfunc Bye() {}\n", "out of scope")

	hunkPaths := func(filter *filterlist.FilterList) map[string]int {
		g := buildSyntheticGraph("in_scope.go")
		if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{}, filter); err != nil {
			t.Fatalf("emitTemporalEdges: %v", err)
		}
		out := map[string]int{}
		for _, n := range nodesByType(g.Nodes, types.NodeHunk) {
			out[n.FilePath]++
		}
		return out
	}

	unfiltered := hunkPaths(nil)
	if unfiltered["out_of_scope.go"] == 0 {
		t.Fatalf("fixture is not exercising the leak: no out-of-scope hunks without a filter (%v)", unfiltered)
	}

	scoped := hunkPaths(&filterlist.FilterList{Include: []string{"in_scope.go"}})
	if n := scoped["out_of_scope.go"]; n != 0 {
		t.Errorf("out-of-scope hunks survived the filter: %d", n)
	}
	if scoped["in_scope.go"] != unfiltered["in_scope.go"] {
		t.Errorf("in-scope hunks = %d, want %d (the filter must not drop them)",
			scoped["in_scope.go"], unfiltered["in_scope.go"])
	}
}

// TestEmitTemporalEdges_FilterKeepsRecoveryForDeletedFiles pins the carve-out
// the build-scope filter needs on the recovery track. Unreachable hunks exist
// so a human can bring back code an agent overwrote (temporal §11.3). The
// include list is derived from the CURRENT tree, so a file that was deleted
// can never appear on it — scoping the recovery track on the list alone drops
// exactly the history the track exists for. A file that is merely out of
// scope but still on disk stays filtered, which is the storage win scoping
// was introduced for.
func TestEmitTemporalEdges_FilterKeepsRecoveryForDeletedFiles(t *testing.T) {
	repo := initGitRepo(t)
	commitFileToRepo(t, repo, "in_scope.go", "package main\n\nfunc Hello() {}\n", "base")

	// Two commits that get abandoned: one touches a file that comes back into
	// the tree, the other a file that stays deleted.
	commitFileToRepo(t, repo, "present.go", "package main\n\nfunc Present() {}\n", "abandoned present")
	commitFileToRepo(t, repo, "deleted.go", "package main\n\nfunc Gone() {}\n", "abandoned deleted")
	runCmd(t, repo, "git", "reset", "--hard", "-q", "HEAD~2")
	// present.go is back on disk (and in history); deleted.go is not.
	commitFileToRepo(t, repo, "present.go", "package main\n\nfunc Present() {}\n", "restored")

	g := buildSyntheticGraph("in_scope.go")
	filter := &filterlist.FilterList{Include: []string{"in_scope.go"}}
	if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{}, filter); err != nil {
		t.Fatalf("emitTemporalEdges: %v", err)
	}
	recovery := map[string]int{}
	for _, n := range nodesByType(g.Nodes, types.NodeHunk) {
		if n.Confidence == types.ConfAmbiguous {
			recovery[n.FilePath]++
		}
	}

	if recovery["deleted.go"] == 0 {
		t.Errorf("recovery hunks for the deleted file were dropped: %v", recovery)
	}
	if n := recovery["present.go"]; n != 0 {
		t.Errorf("out-of-scope file still in the tree kept %d recovery hunks; only deleted files earn the carve-out", n)
	}
}

// TestEmitTemporalEdges_ChangedInCarriesThePlaceAtTheCommit: after a rename
// and a shift, an older changed_in edge names the file and line the
// declaration had in that commit, while the node keeps today's.
func TestEmitTemporalEdges_ChangedInCarriesThePlaceAtTheCommit(t *testing.T) {
	repo := initGitRepo(t)
	c1 := commitFileToRepo(t, repo, "old/a.go", "package a\nfunc F() {\n\tf()\n}\n", "add F")
	runCmd(t, repo, "git", "mv", "old/a.go", "a.go")
	runCmd(t, repo, "git", "commit", "-q", "-m", "move")
	commitFileToRepo(t, repo, "a.go", "package a\n\n// F does f.\nfunc F() {\n\tf2()\n}\n", "edit F")
	g := &graph.Graph{Nodes: []types.Node{{ID: "funcF00000000000", Type: types.NodeFunction, Name: "F", QualifiedName: "a.F",
		FilePath: "a.go", StartLine: 4, EndLine: 6, EndByte: 1, Language: "go", Confidence: types.ConfExtracted}}}
	if _, err := emitTemporalEdges(g, repo, discardLog(), HistoryOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	for _, e := range edgesByType(g.Edges, types.EdgeChangedIn) {
		if nodeQname(g.Nodes, e.Dst) == "commit:"+c1 && (e.FilePath != "old/a.go" || e.Line != 2) {
			t.Errorf("edge to the first commit is at %s:%d, want old/a.go:2", e.FilePath, e.Line)
		}
	}
}
