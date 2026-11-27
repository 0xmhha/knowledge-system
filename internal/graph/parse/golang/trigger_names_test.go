package golang_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestTriggersNameTheSendersChannel: a triggers edge names the channel the
// send is on (testdata/concurrency/trigger_names.go: the goroutine sends on
// results, xferErrs on errs, and the receive can be on either), and a
// renaming of the channels cannot change which channel an edge names: the
// facts are computed again with the Channel names permuted.
func TestTriggersNameTheSendersChannel(t *testing.T) {
	p, rg := concurrencyGraph(t)
	// triggers returns "sender line -> channel node line" for the fixture
	// file, the channel found by the name the edge carries.
	triggers := func(nodes []types.Node) []string {
		byID, byQname := map[string]types.Node{}, map[string]types.Node{}
		for _, n := range nodes {
			byID[n.ID] = n
			if n.Type == types.NodeChannel {
				byQname[n.QualifiedName] = n
			}
		}
		_, facts, _ := gop.EmitDispatchEdges(p.Pkgs(), nodes)
		var out []string
		for _, e := range facts.Edges() {
			if e.Type != types.EdgeTriggers || !strings.HasSuffix(e.FilePath, "trigger_names.go") {
				continue
			}
			ch := byQname[strings.TrimSuffix(e.DispatchKind, "~aliasing")]
			out = append(out, byID[e.Src].QualifiedName+" -> channel made at line "+strconv.Itoa(ch.StartLine))
		}
		sort.Strings(out)
		return out
	}
	before := triggers(rg.Nodes)
	for _, w := range []string{"xferErrs -> channel made at line 7", "Goroutine", "-> channel made at line 20"} {
		found := false
		for _, b := range before {
			if strings.Contains(b, w) {
				found = true
			}
		}
		if !found {
			t.Errorf("no trigger %q in %v", w, before)
		}
	}
	for _, b := range before {
		if strings.Contains(b, "xferVerify#Goroutine") && !strings.HasSuffix(b, "line 20") || strings.Contains(b, "xferErrs") && !strings.HasSuffix(b, "line 7") {
			t.Errorf("a sender names a channel it does not send on: %s", b)
		}
	}
	// Permute the names of the Channel nodes: reverse their order.
	renamed := append([]types.Node(nil), rg.Nodes...)
	var idx []int
	var names []string
	for i, n := range renamed {
		if n.Type == types.NodeChannel {
			idx = append(idx, i)
			names = append(names, n.QualifiedName)
		}
	}
	sort.Strings(names)
	for i, j := range idx {
		renamed[j].QualifiedName = "renamed/" + names[len(names)-1-i]
	}
	if after := triggers(renamed); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("renaming the channels changed the triggers:\nbefore %v\nafter  %v", before, after)
	}
}

// concurrencyGraph loads and resolves testdata/concurrency, keeping the
// parser for its packages.
func concurrencyGraph(t *testing.T) (*gop.Parser, *parse.ResolvedGraph) {
	t.Helper()
	const dir = "testdata/concurrency"
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedDeps | packages.NeedImports | packages.NeedModule | packages.NeedCompiledGoFiles,
		Dir: dir,
	}, "./...")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	root, _ := filepath.Abs(dir)
	p := gop.New(root)
	p.SetPackages(pkgs)
	var results []*parse.ParseResult
	for _, pkg := range pkgs {
		for _, path := range pkg.GoFiles {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := p.ParseFile(path, src)
			if err != nil {
				t.Fatal(err)
			}
			results = append(results, r)
		}
	}
	rg, err := p.Resolve(results)
	if err != nil {
		t.Fatal(err)
	}
	return p, rg
}

// TestTriggersChannelIdentity: the channel a triggers edge names is one
// both operations can be on, by their own make sites
// (testdata/concurrency/trigger_names.go): a channel stored through a
// parameter another caller passes a different channel to stays apart from
// that one; a close of a parameter one of two channels can be named by both,
// marked; a channel sent on a channel and closed on the other side names the
// make site of the sent one, marked.
func TestTriggersChannelIdentity(t *testing.T) {
	p, rg := concurrencyGraph(t)
	byID := map[string]types.Node{}
	for _, n := range rg.Nodes {
		byID[n.ID] = n
	}
	byQname := map[string]types.Node{}
	for _, n := range rg.Nodes {
		if n.Type == types.NodeChannel {
			byQname[n.QualifiedName] = n
		}
	}
	_, facts, _ := gop.EmitDispatchEdges(p.Pkgs(), rg.Nodes)
	got := map[string]bool{}
	for _, e := range facts.Edges() {
		if e.Type != types.EdgeTriggers || !strings.HasSuffix(e.FilePath, "trigger_names.go") {
			continue
		}
		name, marked := strings.CutSuffix(e.DispatchKind, "~aliasing")
		row := fmt.Sprintf("%s -> %d on %d", strings.TrimPrefix(byID[e.Src].QualifiedName, "mutex_fixture."), byID[e.Dst].StartLine, byQname[name].StartLine)
		if marked {
			row += " ~aliasing"
		}
		got[row] = true
	}
	tests := []struct {
		row  string
		want bool
	}{
		{"xferSync.Cancel -> 44 on 48", true},
		{"xferSync.Cancel -> 44 on 56", false},
		{"xferSync.Cancel -> 44 on 56 ~aliasing", false},
		{"xferOtherSync -> 44 on 56", true},
		{"xferOtherSync -> 44 on 48", false},
		{"xferDone -> 70 on 65 ~aliasing", true},
		{"xferDone -> 70 on 67 ~aliasing", true},
		{"xferServe -> 84 on 82 ~aliasing", true},
	}
	for _, tc := range tests {
		if got[tc.row] != tc.want {
			t.Errorf("%q present=%v, want %v", tc.row, got[tc.row], tc.want)
		}
	}
	if t.Failed() {
		for k := range got {
			t.Log(k)
		}
	}
}
