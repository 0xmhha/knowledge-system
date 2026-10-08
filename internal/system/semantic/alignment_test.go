package semantic

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCoordinates(t *testing.T, dir string, v any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty(t *testing.T) {
	p, repo := fixture(t)
	root := t.TempDir()
	graphDir, vectorDir := filepath.Join(root, "graph"), filepath.Join(root, "vector")
	graph := graphCoordinates{SchemaVersion: "1.23", SrcRoot: repo, SrcCommit: p.Snapshot.Commit, GraphDigest: strings.Repeat("a", 64)}
	vector := vectorCoordinates{SrcRoot: repo, SrcCommit: p.Snapshot.Commit}
	vector.Sources.CKG.GraphDigest = graph.GraphDigest
	vector.Sources.CKG.SrcCommit = p.Snapshot.Commit
	write := func() {
		writeCoordinates(t, graphDir, graph)
		writeCoordinates(t, vectorDir, vector)
	}
	write()
	store, err := OpenStore(filepath.Join(root, "semantic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.PutAligned(ctx, p, repo, graphDir, vectorDir); err != nil {
		t.Fatalf("aligned promotion: %v", err)
	}
	if err := store.ActivateAligned(ctx, p.Snapshot.ProjectID, p.Snapshot.DatasetID, repo, graphDir, vectorDir); err != nil {
		t.Fatal(err)
	}
	active, err := store.CurrentAligned(ctx, p.Snapshot.ProjectID, repo, graphDir, vectorDir)
	if err != nil || active.Snapshot() != p.Snapshot {
		t.Fatalf("aligned active projection: %+v, %v", active.Snapshot(), err)
	}
	if status, err := store.TraceStatus(ctx, p.Snapshot.ProjectID, repo, graphDir, vectorDir); err != nil || status.State != "current" || status.Snapshot != p.Snapshot {
		t.Fatalf("current trace diagnostic: %+v, %v", status, err)
	}
	// A different project may use the same short symbol names, but its
	// projection cannot join with this dataset's source root.
	cases := []struct {
		name string
		edit func()
		want string
	}{
		{"graph commit", func() { graph.SrcCommit = strings.Repeat("0", 40) }, "source commit"},
		{"vector commit", func() { vector.SrcCommit = strings.Repeat("0", 40) }, "source commit"},
		{"ledger commit", func() { vector.Sources.CKG.SrcCommit = strings.Repeat("0", 40) }, "source commit"},
		{"graph pin", func() { graph.GraphDigest = "different" }, "digest pin"},
		{"other source", func() { vector.SrcRoot = t.TempDir() }, "source root"},
		{"missing root", func() { graph.SrcRoot = "" }, "source root"},
		{"old graph schema", func() { graph.SchemaVersion = "1.18" }, "canonical_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeGraph, beforeVector := graph, vector
			defer func() { graph, vector = beforeGraph, beforeVector }()
			tc.edit()
			write()
			if _, err := store.CurrentAligned(ctx, p.Snapshot.ProjectID, repo, graphDir, vectorDir); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("stale read expected %q, got %v", tc.want, err)
			}
			status, err := store.TraceStatus(ctx, p.Snapshot.ProjectID, repo, graphDir, vectorDir)
			if err != nil || status.State != "stale" || !strings.Contains(status.Reason, tc.want) || status.Snapshot != p.Snapshot || len(status.Requirements) != 0 {
				t.Fatalf("stale trace leaked paths or missed diagnostic: %+v, %v", status, err)
			}
			other := p
			other.Snapshot.DatasetID = "other-" + strings.ReplaceAll(tc.name, " ", "-")
			other.Evidence = append([]EvidenceSpan(nil), p.Evidence...)
			for i := range other.Evidence {
				other.Evidence[i].Snapshot = other.Snapshot
			}
			if err := store.PutAligned(ctx, other, repo, graphDir, vectorDir); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if _, err := store.Load(ctx, other.Snapshot.ProjectID, other.Snapshot.DatasetID); err == nil {
				t.Fatal("rejected projection was stored")
			}
			if err := store.ActivateAligned(ctx, p.Snapshot.ProjectID, p.Snapshot.DatasetID, repo, graphDir, vectorDir); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("stale activation expected %q, got %v", tc.want, err)
			}
			current, err := store.Current(ctx, p.Snapshot.ProjectID)
			if err != nil || current.Snapshot.DatasetID != p.Snapshot.DatasetID {
				t.Fatalf("failed promotion changed active projection: %+v, %v", current.Snapshot, err)
			}
		})
	}
}

func TestPinnedSemanticProjectionCannotCrossDataset(t *testing.T) {
	p, repo := fixture(t)
	p.Snapshot.SnapshotID = strings.Repeat("b", 64)
	for i := range p.Evidence {
		p.Evidence[i].Snapshot = p.Snapshot
	}
	root := t.TempDir()
	graphDir, vectorDir := filepath.Join(root, "graph"), filepath.Join(root, "vector")
	graph := graphCoordinates{ProjectID: p.Snapshot.ProjectID, SnapshotID: p.Snapshot.SnapshotID,
		DatasetID: p.Snapshot.DatasetID, FileManifestDigest: strings.Repeat("c", 64),
		SchemaVersion: "1.23", SrcRoot: repo, SrcCommit: p.Snapshot.Commit, GraphDigest: strings.Repeat("a", 64)}
	vector := vectorCoordinates{ProjectID: graph.ProjectID, SnapshotID: graph.SnapshotID,
		DatasetID: graph.DatasetID, FileManifestDigest: graph.FileManifestDigest,
		SrcRoot: repo, SrcCommit: p.Snapshot.Commit}
	vector.Sources.CKG.GraphDigest, vector.Sources.CKG.SrcCommit = graph.GraphDigest, p.Snapshot.Commit
	writeCoordinates(t, graphDir, graph)
	writeCoordinates(t, vectorDir, vector)
	if got, err := PinnedSnapshotID(graphDir, p.Snapshot.ProjectID, p.Snapshot.DatasetID); err != nil || got != p.Snapshot.SnapshotID {
		t.Fatalf("pinned snapshot lookup: %q %v", got, err)
	}
	if err := ValidateDatasetAlignment(p, repo, graphDir, vectorDir); err != nil {
		t.Fatalf("pinned projection rejected: %v", err)
	}
	vector.DatasetID = "different"
	writeCoordinates(t, vectorDir, vector)
	if err := ValidateDatasetAlignment(p, repo, graphDir, vectorDir); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("cross-dataset projection accepted: %v", err)
	}
	vector.DatasetID = graph.DatasetID
	writeCoordinates(t, vectorDir, vector)
	p.Snapshot.DatasetID = "different"
	for i := range p.Evidence {
		p.Evidence[i].Snapshot = p.Snapshot
	}
	if err := ValidateDatasetAlignment(p, repo, graphDir, vectorDir); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("projection from another dataset accepted: %v", err)
	}
}

func TestValidateCanonicalCoverageUsesMeasuredProjectFloor(t *testing.T) {
	dir := t.TempDir()
	writeCoordinates(t, dir, map[string]any{"symbol_count": 100, "canonical_count": 94})
	if err := ValidateCanonicalCoverage(dir, 0.94); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalCoverage(dir, 0.95); err == nil || !strings.Contains(err.Error(), "below project minimum") {
		t.Fatalf("under-floor dataset passed: %v", err)
	}
	if err := ValidateCanonicalCoverage(dir, 1.01); err == nil {
		t.Fatal("invalid floor accepted")
	}
	writeCoordinates(t, dir, map[string]any{"symbol_count": 10, "canonical_count": 11})
	if err := ValidateCanonicalCoverage(dir, 0.1); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("invalid counters accepted: %v", err)
	}
}
