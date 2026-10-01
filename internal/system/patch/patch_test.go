package patch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func patchVersion(t *testing.T, source, dataset, version string) *setup.DatasetIdentity {
	t.Helper()
	vdir := filepath.Join(dataset, version)
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: source, Out: vdir, ProjectID: "pilot", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "inputs")
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"graph", "vector"} {
		dir := filepath.Join(vdir, side)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		manifest := map[string]any{"src_root": source, "graph_digest": "graph", "schema_version": "1.23",
			"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space",
			"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID,
			"dataset_id": expected.DatasetID, "source_mode": captured.Identity.SourceMode,
			"file_manifest_digest":  captured.Identity.FileManifestDigest,
			"capture_policy_digest": captured.Identity.CapturePolicyDigest}
		data, _ := json.Marshal(manifest)
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := setup.PublishCandidateIdentity(vdir, captured.Identity, "inputs")
	if err != nil {
		t.Fatal(err)
	}
	return &identity
}

func TestPatchRegisterPinsChangedFilesAndRefusesReusedID(t *testing.T) {
	source, dataset := t.TempDir(), t.TempDir()
	file := filepath.Join(source, "README.md")
	if err := os.WriteFile(file, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	patchVersion(t, source, dataset, "base")
	if err := os.Symlink("base", filepath.Join(dataset, "current")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("after\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := patchVersion(t, source, dataset, "candidate")
	h := hold{ProjectID: "pilot", DatasetID: first.DatasetID, SnapshotID: first.Source.SnapshotID, BaseVersion: "base"}
	data, _ := json.Marshal(h)
	if err := os.WriteFile(filepath.Join(dataset, "candidate", "review-hold.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := Register(dataset, "candidate", "change-1")
	if err != nil || len(a.ChangedFiles) != 1 || a.ChangedFiles[0].Path != "README.md" || a.State != "unconfirmed" {
		t.Fatalf("registered patch: %+v %v", a, err)
	}
	if _, err := Register(dataset, "candidate", "change-1"); err != nil {
		t.Fatalf("identical retry changed patch: %v", err)
	}
	if _, err := setup.Promote(dataset, "candidate"); err == nil {
		t.Fatal("held patch promoted without review")
	}
	if err := os.WriteFile(file, []byte("different\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := patchVersion(t, source, dataset, "other")
	h = hold{ProjectID: "pilot", DatasetID: second.DatasetID, SnapshotID: second.Source.SnapshotID, BaseVersion: "base"}
	data, _ = json.Marshal(h)
	if err := os.WriteFile(filepath.Join(dataset, "other", "review-hold.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Register(dataset, "other", "change-1"); err == nil {
		t.Fatal("same patch ID accepted different changed bytes")
	}
}
