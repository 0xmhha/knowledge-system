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

func queryFixture(t *testing.T) (persist.Store, string) {
	t.Helper()
	s, e := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	if e = s.Migrate(); e != nil {
		t.Fatal(e)
	}
	blob := []byte("func F(x int)int{y:=x;y=1;return y}")
	node := types.Node{ID: "1111111111111111", Type: types.NodeFunction, Name: "F", QualifiedName: "p.F", CanonicalID: "example.invalid/p.F", Language: "go", FilePath: "p.go", StartLine: 12, StartByte: 300, EndByte: 300 + len(blob), EndLine: 12, Confidence: types.ConfExtracted}
	if e = s.InsertNodes([]types.Node{node}); e != nil {
		t.Fatal(e)
	}
	if e = s.InsertBlobs(map[string][]byte{node.ID: blob}); e != nil {
		t.Fatal(e)
	}
	if e = s.SetManifest(persist.Manifest{SchemaVersion: "1.9", SrcCommit: strings.Repeat("a", 40), BuildTimestamp: "2026-10-14T00:00:00Z"}); e != nil {
		t.Fatal(e)
	}
	return s, node.ID
}
func TestRetainedQueryPinsOriginalSourceWithoutLiveFileAccess(t *testing.T) {
	s, id := queryFixture(t)
	out, e := Query(context.Background(), s, QueryInput{NodeID: id, ExpectedCommit: strings.Repeat("a", 40)})
	if e != nil || out.Analysis == nil || out.Status != "analyzed_bounded" {
		t.Fatal(out, e)
	}
	if out.Analysis.Source.StartByte != 300 || out.Analysis.Source.StartLine != 12 || out.Analysis.Source.Commit != strings.Repeat("a", 40) || out.Analysis.Source.SnapshotSHA256 != out.SnapshotSHA256 {
		t.Fatal("source pin/location lost", out.Analysis.Source)
	}
	pinned, e := Query(context.Background(), s, QueryInput{CanonicalID: "example.invalid/p.F", ExpectedSourceSHA256: out.Analysis.Source.SHA256, ExpectedSnapshotSHA256: out.SnapshotSHA256})
	if e != nil || pinned.Analysis == nil {
		t.Fatal(pinned, e)
	}
}
func TestQueryRefusesStaleCommitBlobSnapshotAndMissingIdentity(t *testing.T) {
	s, id := queryFixture(t)
	for _, input := range []QueryInput{{NodeID: id, ExpectedCommit: strings.Repeat("b", 40)}, {NodeID: id, ExpectedSourceSHA256: strings.Repeat("b", 64)}, {NodeID: id, ExpectedSnapshotSHA256: strings.Repeat("b", 64)}} {
		out, e := Query(context.Background(), s, input)
		if e != nil || out.Status != "source_mismatch" || out.Analysis != nil {
			t.Fatal(out, e)
		}
	}
	out, e := Query(context.Background(), s, QueryInput{NodeID: "2222222222222222"})
	if e != nil || out.Status != "not_found" {
		t.Fatal(out, e)
	}
	for _, input := range []QueryInput{{}, {NodeID: id, CanonicalID: "p.F"}, {NodeID: id, ExpectedCommit: "main"}} {
		if _, e = Query(context.Background(), s, input); e == nil {
			t.Fatal("ambiguous/mutable identity accepted")
		}
	}
}

type driftingReader struct {
	store.Reader
	mode                    string
	manifests, blobs, nodes int
}

func (r *driftingReader) GetManifest() (persist.Manifest, error) {
	r.manifests++
	m, e := r.Reader.GetManifest()
	if r.mode == "manifest" && r.manifests > 1 {
		m.GraphDigest = "changed"
	}
	return m, e
}
func (r *driftingReader) GetBlob(id string) ([]byte, error) {
	r.blobs++
	b, e := r.Reader.GetBlob(id)
	if r.mode == "blob" && r.blobs > 1 {
		b = append(b, ' ')
	}
	return b, e
}
func (r *driftingReader) NodesByIDs(ids []string) ([]types.Node, error) {
	r.nodes++
	nodes, e := r.Reader.NodesByIDs(ids)
	if r.mode == "node" && r.nodes > 1 && len(nodes) > 0 {
		nodes[0].StartLine++
	}
	return nodes, e
}
func TestManifestNodeAndBlobMutationDuringAnalysisIsNotReturned(t *testing.T) {
	for _, mode := range []string{"manifest", "blob", "node"} {
		t.Run(mode, func(t *testing.T) {
			s, id := queryFixture(t)
			r := &driftingReader{Reader: s, mode: mode}
			out, e := Query(context.Background(), r, QueryInput{NodeID: id})
			if e != nil || out.Status != "snapshot_changed" || out.Analysis != nil {
				t.Fatal("mixed snapshot survived", out, e)
			}
		})
	}
}
func TestPrivateSourcesWrongCoordinatesMissingBlobAndUnsupportedLanguage(t *testing.T) {
	for _, mode := range []string{"private", "coordinates", "no_blob", "language", "kind"} {
		t.Run(mode, func(t *testing.T) {
			s, id := queryFixture(t)
			nodes, e := s.NodesByIDs([]string{id})
			if e != nil {
				t.Fatal(e)
			}
			node := nodes[0]
			node.ID = "3333333333333333"
			blob, e := s.GetBlob(id)
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "private":
				node.FilePath = ".claude/private.go"
			case "coordinates":
				node.EndByte++
			case "language":
				node.Language = "ts"
			case "kind":
				node.Type = types.NodeField
			}
			if e = s.InsertNodes([]types.Node{node}); e != nil {
				t.Fatal(e)
			}
			if mode != "no_blob" {
				if e = s.InsertBlobs(map[string][]byte{node.ID: blob}); e != nil {
					t.Fatal(e)
				}
			}
			out, e := Query(context.Background(), s, QueryInput{NodeID: node.ID})
			if e != nil {
				t.Fatal(e)
			}
			expected := map[string]string{"private": "unsupported", "coordinates": "source_mismatch", "no_blob": "unavailable", "language": "unsupported", "kind": "unsupported"}[mode]
			if out.Status != expected {
				t.Fatal(out)
			}
		})
	}
}
func TestStrictQueryInputTypesDuplicateUnknownNullAndBounds(t *testing.T) {
	for _, raw := range []string{`{}`, `{"node_id":"x","node_id":"y"}`, `{"node_id":"x","limits":{"max_nodes":1,"max_nodes":2}}`, `{"node_id":"x","extra":1}`, `{"node_id":"x","limits":{"max_nodes":"2"}}`, `{"node_id":"x","limits":{"max_nodes":1.5}}`, `{"node_id":"x","limits":{"max_nodes":0}}`, `{"node_id":"x","limits":{"max_nodes":1025}}`, `{"node_id":"x","limits":null}`, `{"node_id":"","canonical_id":"f"}`, `{"node_id":"x","expected_commit":""}`, `{"node_id":"x"} {}`} {
		if _, e := DecodeQueryInput([]byte(raw)); e == nil {
			t.Fatal("invalid actual input accepted", raw)
		}
	}
	if _, e := DecodeQueryInput([]byte(`{"node_id":"x","limits":{"max_nodes":12}}`)); e != nil {
		t.Fatal(e)
	}
}
func TestMalformedControlFlowDoesNotPanicThroughQuery(t *testing.T) {
	for _, code := range []string{"func F(){break}", "func F(){continue}"} {
		r, e := Analyze(context.Background(), retained(), []byte(code), Options{})
		if e != nil || r.Status != "unsupported" || !diag(r, "invalid_control_flow") {
			t.Fatal("invalid branch not rejected", r, e)
		}
	}
}

func TestOriginalNodeCoordinatesAndFunctionNameMustMatchRetainedBlob(t *testing.T) {
	s, id := queryFixture(t)
	blob, e := s.GetBlob(id)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.InsertBlobs(map[string][]byte{id: []byte(strings.Replace(string(blob), "func F(", "func G(", 1))}); e != nil {
		t.Fatal(e)
	}
	out, e := Query(context.Background(), s, QueryInput{NodeID: id})
	if e != nil || out.Analysis == nil || out.Status != "unsupported" || !diag(*out.Analysis, "function_identity_mismatch") {
		t.Fatal("wrong function blob accepted", out, e)
	}
}
