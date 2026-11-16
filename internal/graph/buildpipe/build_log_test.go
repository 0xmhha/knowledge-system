package buildpipe

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestWarnDuplicateNodeIDs: two different declarations with one id are
// reported; the package node shared by the files of one package is not.
func TestWarnDuplicateNodeIDs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	parts := []*parse.ResolvedGraph{{Nodes: []types.Node{
		{ID: "pkg", Type: types.NodePackage, QualifiedName: "x", FilePath: "a/x/one.go"},
		{ID: "pkg", Type: types.NodePackage, QualifiedName: "x", FilePath: "a/x/two.go"},
		{ID: "do", Type: types.NodeFunction, QualifiedName: "x.Do", FilePath: "a/x/x.go"},
		{ID: "do", Type: types.NodeFunction, QualifiedName: "x.Do", FilePath: "b/x/x.go"},
	}}}
	if n := warnDuplicateNodeIDs(log, parts); n != 1 {
		t.Fatalf("count = %d, want 1 (the package node of one package is shared, not a duplicate)", n)
	}
	out := buf.String()
	if !strings.Contains(out, "duplicate node ids") || !strings.Contains(out, "b/x/x.go") || !strings.Contains(out, "level=WARN") {
		t.Errorf("warning missing or without the pair:\n%s", out)
	}
	buf.Reset()
	if n := warnDuplicateNodeIDs(log, parts[:0]); n != 0 || buf.Len() != 0 {
		t.Errorf("no parts: count %d, log %q", n, buf.String())
	}
}

func TestLogResolveSummary(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := &parse.ResolvedGraph{}
	rg.CountUnresolved(parse.UnresolvedOutsideGraph, "fmt.Println", "a.go", 3)
	rg.CountUnresolved(parse.UnresolvedOutsideGraph, "fmt.Printf", "a.go", 4)
	rg.CountUnresolved(parse.UnresolvedBareName, "Error", "a.go", 5)
	logResolveSummary(log, "go", rg)
	out := buf.String()
	for _, want := range []string{"resolve summary", "unresolved_outside_graph=2", "unresolved_bare_name=1", "unresolved=3", "area=resolve", "fmt.Println (a.go:3)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if got := unresolvedTotal([]*parse.ResolvedGraph{rg, nil, {}}); got != 3 {
		t.Errorf("unresolvedTotal = %d, want 3", got)
	}
}
