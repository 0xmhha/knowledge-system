package annotations

import (
	"context"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/policy"
	"github.com/0xmhha/knowledge-system/pkg/graph/security"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) persist.Store {
	t.Helper()
	s, e := persist.Open(filepath.Join(t.TempDir(), "g.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	if e = s.Migrate(); e != nil {
		t.Fatal(e)
	}
	nodes := []types.Node{{ID: "seed", Type: types.NodeFunction, Name: "Seed", QualifiedName: "p.Seed", FilePath: "p.go", Language: "go", StartLine: 1, EndLine: 1, Confidence: types.ConfExtracted}, {ID: "empty", Type: types.NodeFunction, Name: "Empty", QualifiedName: "p.Empty", FilePath: "p.go", Language: "go", StartLine: 1, EndLine: 1, Confidence: types.ConfExtracted}, {ID: "private", Type: types.NodeFunction, FilePath: ".claude/notes.go"}}
	p := policy.Resolve(&policy.File{Policies: []policy.Entry{{ID: "manual", Name: "Fork", Description: "retain consensus", Governs: []string{"p.Seed"}}}}, nodes, "policy.yaml")
	sec := security.Resolve(&security.File{SecurityPatterns: []security.Entry{{ID: "risk", Name: "Operator risk", Severity: "high", Description: "double\n\nRemediation: inside description", Remediation: "review call boundary", Matches: []string{"p.Seed"}}}}, nodes, "security.yaml")
	nodes = append(nodes, p.Nodes...)
	nodes = append(nodes, sec.Nodes...)
	if e = s.InsertNodes(nodes); e != nil {
		t.Fatal(e)
	}
	if e = s.InsertEdges(append(p.Edges, sec.Edges...)); e != nil {
		t.Fatal(e)
	}
	if e = s.InsertBlobs(map[string][]byte{"seed": []byte("func Seed() {}")}); e != nil {
		t.Fatal(e)
	}
	if e = s.SetManifest(persist.Manifest{SchemaVersion: "1.24", SrcCommit: strings.Repeat("a", 40)}); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestTypedRoundTripUnknownProvenanceAndEmptySafety(t *testing.T) {
	s := fixture(t)
	out, e := Query(context.Background(), s, Input{NodeID: "seed"})
	if e != nil || out.Status != "annotations_found" || len(out.Annotations) != 2 {
		t.Fatal(out, e)
	}
	for _, a := range out.Annotations {
		if a.Provenance.Origin != "operator_curated_metadata" || a.Provenance.SourceLine.Status != "unknown" || a.Provenance.SourceSHA256.Status != "unknown" || a.Provenance.Version.Status != "unknown" || a.Provenance.Reviewer.Status != "unknown" || a.Provenance.Approval.Status != "unknown" {
			t.Fatal(a)
		}
		if a.Kind == string(types.NodeSecurityPattern) && (a.Explanation.Value != "double\n\nRemediation: inside description" || a.ModificationHint.Value != "review call boundary" || a.Severity.Value != "high") {
			t.Fatal("typed data lost or merged parser invented", a)
		}
	}
	empty, e := Query(context.Background(), s, Input{NodeID: "empty"})
	if e != nil || empty.Status != "no_annotations" || empty.SafetyVerdict != "unknown" {
		t.Fatal(empty, e)
	}

	legacy := &metadataDrift{Reader: s, bounded: s.(store.BoundedEdgeReader), mode: "legacy", counts: map[string]int{}}
	out, e = Query(context.Background(), legacy, Input{NodeID: "seed"})
	if e != nil || len(out.Annotations) != 2 {
		t.Fatal(out, e)
	}
	seen := false
	for _, a := range out.Annotations {
		if a.Kind == string(types.NodeSecurityPattern) {
			seen = true
			if a.ModificationHint.Status != "unknown" || a.Severity.Status != "unknown" || !strings.Contains(a.Explanation.Value, "review call boundary") {
				t.Fatal("legacy hint guessed or prose dropped", a)
			}
		}
	}
	if !seen {
		t.Fatal("legacy relation missing")
	}

}
func TestCapsPinsUnsupportedPrivateAndCancellation(t *testing.T) {
	s := fixture(t)
	for _, c := range []struct {
		q      Input
		status string
	}{{Input{NodeID: "seed", MaxAnnotations: 1}, "partial"}, {Input{NodeID: "missing"}, "not_found"}, {Input{NodeID: "private"}, "unsupported"}, {Input{NodeID: "seed", ExpectedSourceSHA256: strings.Repeat("0", 64)}, "source_mismatch"}, {Input{NodeID: "seed", ExpectedSnapshotSHA256: strings.Repeat("0", 64)}, "source_mismatch"}} {
		out, e := Query(context.Background(), s, c.q)
		if e != nil || out.Status != c.status {
			t.Fatal(out, e)
		}
	}
	out, e := Query(context.Background(), struct{ store.Reader }{s}, Input{NodeID: "seed"})
	if e != nil || out.Status != "unavailable" {
		t.Fatal(out, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, e = Query(ctx, s, Input{NodeID: "seed"})
	if e != nil || out.Status != "budget_exhausted" {
		t.Fatal(out, e)
	}
}

type drift struct {
	store.Reader
	bounded store.BoundedEdgeReader
	reads   int
	mode    string
}

func (r *drift) IncidentEdges(ctx context.Context, q store.EdgePageInput) ([]types.Edge, bool, error) {
	edges, more, e := r.bounded.IncidentEdges(ctx, q)
	r.reads++
	if r.reads == 2 {
		if r.mode == "more" {
			more = !more
		} else {
			edges[0].FilePath = "changed.yaml"
		}
	}
	return edges, more, e
}
func TestFinalPageAndMoreDriftClearAllData(t *testing.T) {
	for _, mode := range []string{"more", "edge"} {
		s := fixture(t)
		r := &drift{Reader: s, bounded: s.(store.BoundedEdgeReader), mode: mode}
		out, e := Query(context.Background(), r, Input{NodeID: "seed"})
		if e != nil || out.Status != "snapshot_changed" || out.Symbol != nil || len(out.Annotations) != 0 {
			t.Fatal(out, e)
		}
	}
}
func TestStrictDecoder(t *testing.T) {
	if _, e := Decode([]byte(`{"node_id":"a","max_annotations":1}`)); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{}`, `[]`, `{"node_id":"a","node_id":"b"}`, `{"node_id":null}`, `{"node_id":"a","max_annotations":0}`, `{"node_id":"a","max_annotations":1.5}`, `{"node_id":"a","max_annotations":129}`, `{"node_id":"a","limits":{}}`, `{"node_id":"a","canonical_id":"b"}`, `{"node_id":"a"} garbage`, `{"node_id":"a","unknown":1}`} {
		if _, e := Decode([]byte(raw)); e == nil {
			t.Fatal(raw)
		}
	}
}

type metadataDrift struct {
	store.Reader
	bounded store.BoundedEdgeReader
	mode    string
	counts  map[string]int
}

func (r *metadataDrift) IncidentEdges(ctx context.Context, q store.EdgePageInput) ([]types.Edge, bool, error) {
	return r.bounded.IncidentEdges(ctx, q)
}
func (r *metadataDrift) GetManifest() (persist.Manifest, error) {
	m, e := r.Reader.GetManifest()
	r.counts["manifest"]++
	if r.mode == "manifest" && r.counts["manifest"] > 1 {
		m.SrcCommit = strings.Repeat("b", 40)
	}
	return m, e
}
func (r *metadataDrift) GetBlob(id string) ([]byte, error) {
	b, e := r.Reader.GetBlob(id)
	r.counts["blob"]++
	if r.mode == "blob" && r.counts["blob"] > 1 {
		b = []byte("changed")
	}
	return b, e
}
func (r *metadataDrift) NodesByIDs(ids []string) ([]types.Node, error) {
	n, e := r.Reader.NodesByIDs(ids)
	if len(n) == 1 && ids[0] == "security:risk" {
		if r.mode == "legacy" {
			n[0].Annotation = nil
		}
		if r.mode == "private" {
			n[0].FilePath = "/owned/.claude/security.yaml"
		}
		if r.mode == "type" {
			n[0].Type = types.NodeFunction
		}
	}
	r.counts[ids[0]]++
	if r.counts[ids[0]] > 1 && (r.mode == "seed" && ids[0] == "seed" || r.mode == "annotation" && ids[0] == "security:risk") {
		n[0].DocComment = "changed"
	}
	return n, e
}
func TestFinalManifestBlobSeedAndTagDriftClearAllData(t *testing.T) {
	for _, mode := range []string{"manifest", "blob", "seed", "annotation"} {
		s := fixture(t)
		r := &metadataDrift{Reader: s, bounded: s.(store.BoundedEdgeReader), mode: mode, counts: map[string]int{}}
		out, e := Query(context.Background(), r, Input{NodeID: "seed"})
		if e != nil || out.Status != "snapshot_changed" || out.Symbol != nil || len(out.Annotations) != 0 {
			t.Fatal(mode, out, e)
		}
	}
}
func TestPrivateAnnotationAndWrongRelationTargetFailClosed(t *testing.T) {
	for _, mode := range []string{"private", "type"} {
		s := fixture(t)
		r := &metadataDrift{Reader: s, bounded: s.(store.BoundedEdgeReader), mode: mode, counts: map[string]int{}}
		want := "unsupported"
		if mode == "type" {
			want = "source_mismatch"
		}
		out, e := Query(context.Background(), r, Input{NodeID: "seed"})
		if e != nil || out.Status != want || len(out.Annotations) != 0 || out.Symbol != nil {
			t.Fatal(mode, out, e)
		}
	}
}

type deadlineReader struct {
	store.Reader
	bounded   store.BoundedEdgeReader
	cancel    context.CancelFunc
	on, reads int
}

func (r *deadlineReader) IncidentEdges(ctx context.Context, q store.EdgePageInput) ([]types.Edge, bool, error) {
	r.reads++
	if r.reads == r.on {
		r.cancel()
	}
	return r.bounded.IncidentEdges(ctx, q)
}
func TestPageAndRecheckDeadlineClearResults(t *testing.T) {
	for _, on := range []int{1, 2} {
		s := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		r := &deadlineReader{Reader: s, bounded: s.(store.BoundedEdgeReader), cancel: cancel, on: on}
		out, e := Query(ctx, r, Input{NodeID: "seed"})
		cancel()
		if e != nil || out.Status != "budget_exhausted" || out.Symbol != nil || len(out.Annotations) != 0 {
			t.Fatal(on, out, e)
		}
	}
}
