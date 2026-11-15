package golang_test

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// identityGraph resolves testdata/identity and renders each edge between
// declarations as "src -> type -> dst", naming a node by file and name so
// declarations that share a canonical id (the two init functions) stay apart.
func identityGraph(t *testing.T) (map[string]bool, []types.Node) {
	t.Helper()
	g, err := gop.LoadAndResolve("testdata/identity")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	rel := func(p string) string {
		if i := strings.Index(p, "testdata/identity/"); i >= 0 {
			return p[i+len("testdata/identity/"):]
		}
		return p
	}
	byID := map[string]types.Node{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		n.FilePath = rel(n.FilePath)
		if prev, dup := byID[n.ID]; dup && prev.Type == n.Type && prev.FilePath != n.FilePath {
			// Files of one package share its package node; anything else that
			// shares an id across files, or a package node across directories,
			// is a collision.
			if n.Type != types.NodePackage || path.Dir(prev.FilePath) != path.Dir(n.FilePath) {
				t.Errorf("node id %s shared by %s (%s) and %s (%s)", n.ID, prev.QualifiedName, prev.FilePath, n.QualifiedName, n.FilePath)
			}
		}
		byID[n.ID] = *n
	}
	name := func(n types.Node) string {
		s := n.FilePath + ":" + n.Name
		if n.CanonicalID == "" && (n.Type == types.NodeStruct || n.Type == types.NodeTypeAlias) {
			s += "(local)"
		}
		return s
	}
	set := map[string]bool{}
	for _, e := range g.Edges {
		src, okS := byID[e.Src]
		dst, okD := byID[e.Dst]
		if !okS || !okD || src.Type == types.NodeFile || src.Type == types.NodePackage {
			continue
		}
		set[fmt.Sprintf("%s -%s-> %s", name(src), e.Type, name(dst))] = true
	}
	return set, g.Nodes
}

// TestIdentity_DeclarationsDoNotCollide covers the identity defects ckg verify
// found on go-stablenet after the canonical-id fixes (roadmap D11, D12):
//
//   - a/x and b/x are different packages that both declare package x and have
//     byte-identical files, so their declarations have the same qualified
//     name and start byte;
//   - cmd1 and cmd2 are two main packages, both with func main;
//   - package p has two init functions in two files;
//   - p.Lit declares a local type pair next to the package-level pair.
func TestIdentity_DeclarationsDoNotCollide(t *testing.T) {
	set, nodes := identityGraph(t)
	for _, want := range []string{
		"a/x/x.go:Do -calls-> a/x/x.go:helper",
		"b/x/x.go:Do -calls-> b/x/x.go:helper",
		"cmd1/main.go:main -calls-> cmd1/main.go:one",
		"cmd2/main.go:main -calls-> cmd2/main.go:two",
		"p/p1.go:init -calls-> p/p1.go:f1",
		"p/p2.go:init -calls-> p/p2.go:f2",
		"p/local.go:Pkg -uses_type-> p/local.go:pair",
	} {
		if !set[want] {
			t.Errorf("missing edge %q", want)
		}
	}
	for _, bad := range []string{
		"a/x/x.go:Do -calls-> b/x/x.go:helper",
		"b/x/x.go:Do -calls-> a/x/x.go:helper",
		"cmd1/main.go:main -calls-> cmd2/main.go:two",
		"cmd2/main.go:main -calls-> cmd1/main.go:one",
		"p/p1.go:init -calls-> p/p2.go:f2",
		"p/p2.go:init -calls-> p/p1.go:f1",
	} {
		if set[bad] {
			t.Errorf("edge %q crosses packages or functions", bad)
		}
	}
	for e := range set {
		// Lit builds a value of its own local pair; an edge from Lit to the
		// package-level pair binds the local type to it (roadmap D12).
		if strings.HasPrefix(e, "p/local.go:Lit -") && strings.HasSuffix(e, "-> p/local.go:pair") {
			t.Errorf("local type bound to the package-level type: %q", e)
		}
		// Only the init in p2.go builds B (roadmap D11, init lookups).
		if strings.HasPrefix(e, "p/p1.go:init -") && strings.HasSuffix(e, "-> p/p2.go:B") {
			t.Errorf("edge of one init attached to another: %q", e)
		}
	}
	if !set["p/p2.go:init -instantiates-> p/p2.go:B"] {
		t.Errorf("missing edge p/p2.go:init -instantiates-> p/p2.go:B")
	}
	// Both packages named x keep their declarations.
	var dos []string
	for _, n := range nodes {
		if n.Name == "Do" {
			dos = append(dos, n.CanonicalID)
		}
	}
	sort.Strings(dos)
	if strings.Join(dos, ",") != "example.com/identity/a/x.Do,example.com/identity/b/x.Do" {
		t.Errorf("Do declarations = %v, want both packages", dos)
	}
}
