package valueflow

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func expansionFixture(t *testing.T) persist.Store {
	t.Helper()
	s, e := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	if e = s.Migrate(); e != nil {
		t.Fatal(e)
	}
	for _, item := range []struct{ id, name, blob string }{{"a", "A", "func A(x int)int{return B(x)}"}, {"b", "B", "func B(x int)int{return C(x)+A(x)}"}, {"c", "C", "func C(x int)int{return x+1}"}, {"noise", "Noise", "func Noise()int{return 3}"}} {
		blob := []byte(item.blob)
		node := types.Node{ID: item.id, Name: item.name, QualifiedName: "p." + item.name, CanonicalID: "example.invalid/p." + item.name, Type: types.NodeFunction, Language: "go", FilePath: "p.go", StartLine: 1, EndLine: 1, EndByte: len(blob), Confidence: types.ConfExtracted}
		if e = s.InsertNodes([]types.Node{node}); e != nil {
			t.Fatal(e)
		}
		if e = s.InsertBlobs(map[string][]byte{node.ID: blob}); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.InsertEdges([]types.Edge{{Src: "a", Dst: "b", Type: "calls", FilePath: "p.go", Line: 1, Confidence: types.ConfExtracted}, {Src: "a", Dst: "noise", Type: "calls", FilePath: "p.go", Line: 1, Confidence: types.ConfExtracted}, {Src: "b", Dst: "a", Type: "calls", FilePath: "p.go", Line: 1, Confidence: types.ConfExtracted}, {Src: "b", Dst: "c", Type: "calls", FilePath: "p.go", Line: 1, Confidence: types.ConfExtracted}}); e != nil {
		t.Fatal(e)
	}
	if e = s.SetManifest(persist.Manifest{SchemaVersion: "1.24", SrcCommit: strings.Repeat("a", 40)}); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestExpansionLocalCalculationsCallHopsCycleAndNoiseAreDistinct(t *testing.T) {
	s := expansionFixture(t)
	out, e := Expand(context.Background(), s, ExpansionInput{Function: QueryInput{NodeID: "a"}, Limits: ExpansionLimits{MaxHops: 3, MaxFunctions: 8}})
	if e != nil || out.Status != "expanded_bounded" || len(out.Functions) != 4 || out.Deduplicated < 1 {
		t.Fatal(out, e)
	}
	seen := map[string]bool{}
	for _, function := range out.Functions {
		r := function.Result.Analysis
		if r == nil || seen[r.Source.NodeID] || r.Source.SnapshotSHA256 != out.SnapshotSHA256 {
			t.Fatal("mixed/duplicate local result")
		}
		seen[r.Source.NodeID] = true
	}
	for _, hop := range out.Hops {
		if hop.Origin != "retained_call_graph" || hop.ArgumentMapping != "not_computed" {
			t.Fatal("call hop relabelled data transfer", hop)
		}
	}
	if !seen["noise"] {
		t.Fatal("noise must remain visible, not implicitly labelled relevant")
	}
}
func TestExpansionCapsStalePinsUnsupportedReaderAndCancellation(t *testing.T) {
	s := expansionFixture(t)
	for _, limit := range []ExpansionLimits{{MaxFunctions: 1}, {MaxCandidates: 1}, {MaxTotalNodes: 1}, {MaxTotalEdges: 1}, {MaxTotalFacts: 1}} {
		out, e := Expand(context.Background(), s, ExpansionInput{Function: QueryInput{NodeID: "a"}, Limits: limit})
		if e != nil || out.Status != "budget_exhausted" || len(out.StopReasons) == 0 || len(out.Functions) > out.Limits.MaxFunctions || out.Candidates > out.Limits.MaxCandidates || out.TotalNodes > out.Limits.MaxTotalNodes || out.TotalEdges+len(out.Hops) > out.Limits.MaxTotalEdges || out.TotalFacts > out.Limits.MaxTotalFacts {
			t.Fatal(out, e)
		}
	}
	out, e := Expand(context.Background(), s, ExpansionInput{Function: QueryInput{NodeID: "a", ExpectedSourceSHA256: strings.Repeat("b", 64)}})
	if e != nil || out.Status != "source_mismatch" || len(out.Functions) != 0 {
		t.Fatal(out, e)
	}
	out, e = Expand(context.Background(), struct{ store.Reader }{s}, ExpansionInput{Function: QueryInput{NodeID: "a"}})
	if e != nil || out.Status != "partial" || len(out.Functions) != 1 {
		t.Fatal(out, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, e = Expand(ctx, s, ExpansionInput{Function: QueryInput{NodeID: "a"}})
	if e != nil || out.Status != "budget_exhausted" {
		t.Fatal(out, e)
	}
}
func TestExpansionDecoderExactIdentityAndStrictLimits(t *testing.T) {
	good := `{"function":{"canonical_id":"example.invalid/p.A"},"limits":{"max_hops":2}}`
	if _, e := DecodeExpansionInput([]byte(good)); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{}`, `{"function":{"node_id":"a","canonical_id":"p.A"}}`, `{"function":{"node_id":"a","node_id":"b"}}`, `{"function":{"node_id":"a"},"limits":{"max_hops":0}}`, `{"function":{"node_id":"a"},"limits":{"max_hops":1.1}}`, `{"function":{"node_id":"a"},"limits":{"max_hops":4}}`, `{"function":{"node_id":"a"},"unknown":true}`} {
		if _, e := DecodeExpansionInput([]byte(raw)); e == nil {
			t.Fatal("invalid expansion input accepted", raw)
		}
	}
}

type expansionPageDrift struct {
	store.Reader
	bounded store.BoundedEdgeReader
	reads   map[string]int
	mode    string
}

func (r *expansionPageDrift) IncidentEdges(ctx context.Context, q store.EdgePageInput) ([]types.Edge, bool, error) {
	edges, more, e := r.bounded.IncidentEdges(ctx, q)
	r.reads[q.NodeID]++
	if r.reads[q.NodeID] > 1 && q.NodeID == "a" {
		if r.mode == "more" {
			more = !more
		} else if len(edges) > 0 {
			edges[0].Line++
		}
	}
	return edges, more, e
}
func TestExpansionRechecksConsumedEdgesAndTruncationMarker(t *testing.T) {
	for _, mode := range []string{"edge", "more"} {
		s := expansionFixture(t)
		reader := &expansionPageDrift{Reader: s, bounded: s.(store.BoundedEdgeReader), reads: map[string]int{}, mode: mode}
		out, e := Expand(context.Background(), reader, ExpansionInput{Function: QueryInput{NodeID: "a"}})
		if e != nil || out.Status != "snapshot_changed" || len(out.Functions) != 0 || len(out.Hops) != 0 {
			t.Fatal("mixed expansion survived", out, e)
		}
	}
}
