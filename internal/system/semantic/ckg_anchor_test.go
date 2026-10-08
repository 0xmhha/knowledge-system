package semantic

import (
	"strings"
	"testing"

	graphstore "github.com/0xmhha/knowledge-system/pkg/graph/store"
	graphtypes "github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func TestCodeAnchorRequiresSameSnapshotAndContainedASTSpan(t *testing.T) {
	snapshot := Snapshot{ProjectID: "p", DatasetID: "d", Commit: strings.Repeat("a", 40)}
	manifest := graphstore.Manifest{CommitHash: snapshot.Commit}
	node := graphtypes.Node{CanonicalID: "pkg.Func", FilePath: "src/main.go", StartLine: 10, EndLine: 30}
	e := EvidenceSpan{ID: "e", Snapshot: snapshot, Kind: SourceCode, CanonicalID: node.CanonicalID,
		Path: node.FilePath, StartLine: 15, EndLine: 18}
	if err := checkCodeAnchor(snapshot, manifest, node, e); err != nil {
		t.Fatalf("valid anchor: %v", err)
	}
	wrong := e
	wrong.Snapshot.DatasetID = "other"
	if err := checkCodeAnchor(snapshot, manifest, node, wrong); err == nil {
		t.Fatal("cross-dataset anchor accepted")
	}
	manifest.CommitHash = strings.Repeat("b", 40)
	if err := checkCodeAnchor(snapshot, manifest, node, e); err == nil {
		t.Fatal("stale CKG anchor accepted")
	}
	manifest.CommitHash = snapshot.Commit
	wrong = e
	wrong.EndLine = 31
	if err := checkCodeAnchor(snapshot, manifest, node, wrong); err == nil {
		t.Fatal("out-of-symbol anchor accepted")
	}
	wrong = e
	wrong.Path = "other/main.go"
	if err := checkCodeAnchor(snapshot, manifest, node, wrong); err == nil {
		t.Fatal("wrong-file anchor accepted")
	}
}
