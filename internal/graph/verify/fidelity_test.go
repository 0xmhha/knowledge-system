package verify

import (
	"path/filepath"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// fidelityFixture is the parser's fixture of constructs a graph once got
// wrong. The truth of this package is computed without the parser's code, so
// a graph of the fixture built by the parser must agree with it: a
// disagreement is a defect of one of the two.
const fidelityFixture = "../parse/golang/testdata/fidelity"

// memGraph is a GraphReader over a resolved graph held in memory.
type memGraph struct {
	nodes []types.Node
	edges []types.Edge
}

func (g memGraph) AllNodes() ([]types.Node, error) { return g.nodes, nil }

func (g memGraph) QueryEdgesByType(t string) ([]types.Edge, error) {
	var out []types.Edge
	for _, e := range g.edges {
		if string(e.Type) == t {
			out = append(out, e)
		}
	}
	return out, nil
}

func verifyFixture(t *testing.T, opts gop.LoadOptions) Report {
	t.Helper()
	root, err := filepath.Abs(fidelityFixture)
	if err != nil {
		t.Fatalf("abs %s: %v", fidelityFixture, err)
	}
	opts.Tests = true // the truth loads test files, as a build does
	rg, err := gop.LoadAndResolveWith(root, opts)
	if err != nil {
		t.Fatalf("build the fixture graph: %v", err)
	}
	r, err := RunGo(root, memGraph{nodes: rg.Nodes, edges: rg.Edges}, Options{Samples: 50})
	if err != nil {
		t.Fatalf("RunGo: %v", err)
	}
	return r
}

// TestRunGo_FixtureGraphAgreesWithTruth: the graph the parser builds for the
// fixture has every declaration, call, type reference and statement link the
// truth finds, and no statement link the truth does not have.
func TestRunGo_FixtureGraphAgreesWithTruth(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	if r.Decls.Missing != 0 {
		t.Errorf("declarations missing from the graph: %v", r.Decls.MissingSamples)
	}
	if r.Decls.OtherSpan != 0 {
		t.Errorf("declarations whose node does not span them, first token through trailing comment: %v", r.Decls.OtherSpanSamples)
	}
	for _, rel := range []struct {
		name string
		s    RelationStats
	}{
		{"static calls", r.StaticCalls}, {"interface calls", r.InterfaceCalls}, {"type refs", r.TypeRefs},
		{"outside calls", r.OutsideCalls}, {"outside type refs", r.OutsideTypeRefs},
	} {
		t.Run(rel.name, func(t *testing.T) {
			if rel.s.Truth == 0 {
				t.Fatalf("the truth holds no %s for the fixture", rel.name)
			}
			if rel.s.Missing != 0 || rel.s.Misresolved != 0 || rel.s.EndpointMissing != 0 {
				t.Errorf("%s: truth %d, missing %d %v, misresolved %d %v, endpoint missing %d", rel.name, rel.s.Truth,
					rel.s.Missing, rel.s.MissingSamples, rel.s.Misresolved, rel.s.MisresolvedSamples, rel.s.EndpointMissing)
			}
		})
	}
	if r.Unexpected.Calls != 0 || r.Unexpected.TypeRefs != 0 {
		t.Errorf("edges the truth does not have: calls %v, type refs %v", r.Unexpected.CallSamples, r.Unexpected.TypeRefSamples)
	}
	for rel, s := range r.Statements {
		t.Run(rel, func(t *testing.T) {
			if s.Missing != 0 || s.Unexpected != 0 {
				t.Errorf("%s: truth %d, matched %d, missing %v, unexpected %v", rel, s.Truth, s.Matched, s.MissingSamples, s.UnexpectedSamples)
			}
		})
	}
	// The constructs of the 2026-11-26 comparison are in the truth, not
	// only in the graph.
	if s := r.Statements[string(types.EdgeEncloses)]; s.Matched == 0 {
		t.Errorf("no encloses link was compared")
	}
}

// TestRunGo_OutsideScope: with a package of the module left out of the
// graph, the truth counts the calls and type references into it as leaving
// the covered files, and the graph has them as edges to External nodes.
func TestRunGo_OutsideScope(t *testing.T) {
	const outside = "scoped/out/"
	r := verifyFixture(t, gop.LoadOptions{Skip: func(rel string) bool { return strings.HasPrefix(rel, outside) }})
	for _, rel := range []struct {
		name string
		s    RelationStats
		want string // a pair that only exists when the package is outside
	}{
		{"outside calls", r.OutsideCalls, "Helper"},
		{"outside type refs", r.OutsideTypeRefs, "T"},
	} {
		if rel.s.Missing != 0 || rel.s.Truth == 0 {
			t.Errorf("%s: truth %d, missing %d %v", rel.name, rel.s.Truth, rel.s.Missing, rel.s.MissingSamples)
		}
	}
	found := map[string]bool{}
	root, _ := filepath.Abs(fidelityFixture)
	truth, err := collectTruth(root, map[string]bool{"scoped/in/in.go": true}, nil)
	if err != nil {
		t.Fatalf("collectTruth: %v", err)
	}
	for _, p := range append(truth.OutsideCalls, truth.OutsideTypeRefs...) {
		found[p.To] = true
	}
	for _, want := range []string{"ckgfidelity.test/scoped/out.Helper", "ckgfidelity.test/scoped/out.(T).Twice", "ckgfidelity.test/scoped/out.T"} {
		if !found[want] {
			t.Errorf("the truth does not count %s as outside the covered files; got %v", want, found)
		}
	}
}
