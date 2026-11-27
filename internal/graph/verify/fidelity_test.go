package verify

import (
	"path/filepath"
	"reflect"
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
	// accessed_under_lock: no edge the source does not support, and of the
	// true pairs only the one in a function with a goto, which the
	// extractor leaves out by rule, is missing.
	if r.UnderLock.Unsupported != 0 {
		t.Errorf("accessed_under_lock edges the source does not support: %v", r.UnderLock.UnsupportedSamples)
	}
	if want := []string{"locks/ledger.go:21:jumped -> locks/ledger.go:7:mu"}; !reflect.DeepEqual(r.UnderLock.MissingSamples, want) {
		t.Errorf("true accessed_under_lock pairs missing: %v, want %v", r.UnderLock.MissingSamples, want)
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

// TestRunGo_PropagatedUnderLock: with lock propagation the graph's
// propagated accessed_under_lock edges are exactly the pairs the truth's own
// call walk finds: a field used in a function called while the mutex is
// held, where it is still held at the use (Counter.viaCall and Counter.deep
// under Ledger.mu, not Counter.afterCall).
func TestRunGo_PropagatedUnderLock(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{LockPropagation: true})
	p := r.UnderLockPropagated
	if p.Edges == 0 || p.Unsupported != 0 || p.Matched != p.Truth {
		t.Errorf("propagated: %d edges, %d unsupported, %d of %d true pairs; unsupported %v missing %v",
			p.Edges, p.Unsupported, p.Matched, p.Truth, p.UnsupportedSamples, p.MissingSamples)
	}
	if r.UnderLock.Unsupported != 0 {
		t.Errorf("direct edges unsupported with propagation on: %v", r.UnderLock.UnsupportedSamples)
	}
	// Counter.viaCall and Counter.deep; Counter.afterCall is used after the
	// Unlock.
	if p.Truth != 2 {
		t.Errorf("propagated truth pairs = %d, want 2", p.Truth)
	}
}

// TestRunGo_MessageLinks: the handles and triggers edges are exactly what
// the syntax says on the messages fixture (a worker loop with range, a
// select over two channels and a done signal, a send through a parameter,
// an unbuffered channel, a field channel).
func TestRunGo_MessageLinks(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	for name, st := range map[string]UnderLockStats{"handles": r.Handles, "triggers": r.Triggers} {
		if st.Truth == 0 || st.Unsupported != 0 || st.Matched != st.Truth {
			t.Errorf("%s: %d edges, %d unsupported, %d of %d true; unsupported %v missing %v",
				name, st.Edges, st.Unsupported, st.Matched, st.Truth, st.UnsupportedSamples, st.MissingSamples)
		}
	}
}

// TestRunGo_ChannelLocks: the graph's channel locks and the fields under
// them agree with the truth's own rules: the inverted lock (a select send
// takes it) and a lock that reaches a method through a parameter and a
// field; no field is under the lock it holds.
func TestRunGo_ChannelLocks(t *testing.T) {
	r := verifyFixture(t, gop.LoadOptions{})
	if r.UnderLock.Unsupported != 0 {
		t.Errorf("unsupported: %v", r.UnderLock.UnsupportedSamples)
	}
	want := []string{"chanlock/chanlock2.go:5:runs -> chanlock/chanlock2.go:10:channel", "chanlock/chanlock2.go:28:w -> chanlock/chanlock2.go:33:channel"}
	for _, w := range want {
		for _, m := range r.UnderLock.MissingSamples {
			if m == w {
				t.Errorf("missing %s", w)
			}
		}
	}
}
