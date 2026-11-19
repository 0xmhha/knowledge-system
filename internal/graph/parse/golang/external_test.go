package golang_test

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestExternalNodes checks that calls to code outside the build keep their
// edge (schema 1.36): testdata/resolve/coll1/timers.go calls time.AfterFunc,
// (*time.Timer).Stop and context.Context.Done. Each target is one External
// node that names its package, and the edge carries the line of the call.
func TestExternalNodes(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/resolve")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	count := map[string]int{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
		if n.Type == types.NodeExternal {
			count[n.CanonicalID]++
		}
	}
	for _, n := range g.Nodes {
		if n.Type == types.NodeMethod && strings.HasPrefix(n.QualifiedName, "coll1.stamped.") && n.QualifiedName != "coll1.stamped.year" {
			t.Errorf("method promoted from an external type: %s", n.QualifiedName)
		}
	}
	for cid, c := range count {
		if c != 1 {
			t.Errorf("%s has %d External nodes, want 1", cid, c)
		}
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		d, ok := byID[e.Dst]
		if !ok || d.Type != types.NodeExternal {
			continue
		}
		got[fmt.Sprintf("%s %s %s L%d", byID[e.Src].QualifiedName, e.Type, d.QualifiedName, e.Line)] = true
	}
	for _, w := range []string{
		"coll1.clock.arm calls time.AfterFunc L13",
		"coll1.clock.disarm calls time.Timer.Stop L20",
		"coll1.waitDone invokes context.Context.Done L27",
	} {
		if !got[w] {
			t.Errorf("missing %q; have %v", w, keys(got))
		}
	}
	// A struct field of an external type: clock uses time.Timer.
	usesTimer := false
	for k := range got {
		if strings.HasPrefix(k, "coll1.clock uses_type time.Timer") {
			usesTimer = true
		}
	}
	if !usesTimer {
		t.Errorf("missing coll1.clock uses_type time.Timer; have %v", keys(got))
	}
	want := map[string]struct {
		kind, sig string
		pkg       types.ExternalPackage
	}{
		"time.AfterFunc":       {"func", "func time.AfterFunc(d time.Duration, f func()) *time.Timer", types.ExternalPackage{Path: "time", Name: "time", Module: "std"}},
		"time.(*Timer).Stop":   {"method", "func (*time.Timer).Stop() bool", types.ExternalPackage{Path: "time", Name: "time", Module: "std"}},
		"context.Context.Done": {"interface_method", "func (context.Context).Done() <-chan struct{}", types.ExternalPackage{Path: "context", Name: "context", Module: "std"}},
		"time.Timer":           {"type", "type time.Timer struct", types.ExternalPackage{Path: "time", Name: "time", Module: "std"}},
	}
	for cid, w := range want {
		n, ok := byID[gop.ExternalNodeID(cid)]
		if !ok {
			t.Errorf("no External node for %s", cid)
			continue
		}
		if n.SubKind != w.kind || n.Signature != w.sig || n.External == nil || *n.External != w.pkg || n.FilePath != types.ExternalFilePath {
			t.Errorf("%s: kind=%q sig=%q pkg=%+v file=%q; want %q %q %+v", cid, n.SubKind, n.Signature, n.External, n.FilePath, w.kind, w.sig, w.pkg)
		}
	}
}

func keys(m map[string]bool) string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return strings.Join(out, "\n")
}

// TestValueUseCountsOnce: coll1 has an in-package test file, so go/packages
// loads its production files twice (the package and its test variant). Each
// use must still count once: bump writes Inner.Depth once on each of two
// lines.
func TestValueUseCountsOnce(t *testing.T) {
	pkgs, err := loadWithTests("testdata/resolve")
	if err != nil {
		t.Fatal(err)
	}
	g, err := gop.LoadAndResolve("testdata/resolve")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range gop.EmitValueAccessEdges(pkgs, g.Nodes) {
		if e.Count != 1 {
			t.Errorf("%s %s -> %s line %d count %d, want 1", e.Type, e.Src, e.Dst, e.Line, e.Count)
		}
	}
}

// loadWithTests loads the fixture as the build does, test variants included.
func loadWithTests(dir string) ([]*packages.Package, error) {
	return packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports | packages.NeedModule,
		Dir: dir, Tests: true,
	}, "./...")
}
