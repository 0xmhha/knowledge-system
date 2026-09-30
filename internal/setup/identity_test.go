package setup

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func identityGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	buf, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, buf)
	}
	return strings.TrimSpace(string(buf))
}

func TestCommittedSourceIdentitySeparatesProjectsAndBytes(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "first")
	head := identityGit(t, root, "rev-parse", "HEAD")
	first, err := CommittedSourceIdentity(root, "p-first", head)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CommittedSourceIdentity(root, "p-first", head)
	if err != nil || again != first {
		t.Fatalf("source identity not deterministic: %+v, %v", again, err)
	}
	other, err := CommittedSourceIdentity(root, "p-other", head)
	if err != nil || other.SnapshotID == first.SnapshotID {
		t.Fatalf("different project reused snapshot: %+v, %v", other, err)
	}
	if err := os.WriteFile(file, []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := CommittedSourceIdentity(root, "p-first", head)
	if err != nil || changed.SnapshotID == first.SnapshotID {
		t.Fatalf("same HEAD with changed bytes reused snapshot: %+v, %v", changed, err)
	}
	if err := os.WriteFile(file, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", "link.go")
	if _, err := CommittedSourceIdentity(root, "p-first", head); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("tracked symlink followed: %v", err)
	}
}

func TestDatasetIdentitySeparatesModelAndPolicy(t *testing.T) {
	source := SourceIdentity{ProjectID: "p", SnapshotID: strings.Repeat("a", 64)}
	first, err := NewDatasetIdentity(source, json.RawMessage(`{"model":"m","digest":"one"}`), "policy-one")
	if err != nil {
		t.Fatal(err)
	}
	again, err := NewDatasetIdentity(source, json.RawMessage(`{"digest":"one","model":"m"}`), "policy-one")
	if err != nil || again.DatasetID != first.DatasetID {
		t.Fatalf("JSON key order affected dataset identity: %+v, %v", again, err)
	}
	changedModel, err := NewDatasetIdentity(source, json.RawMessage(`{"model":"m","digest":"two"}`), "policy-one")
	if err != nil || changedModel.DatasetID == first.DatasetID {
		t.Fatalf("model change reused dataset identity: %+v, %v", changedModel, err)
	}
	changedPolicy, err := NewDatasetIdentity(source, json.RawMessage(`{"model":"m","digest":"one"}`), "policy-two")
	if err != nil || changedPolicy.DatasetID == first.DatasetID {
		t.Fatalf("policy change reused dataset identity: %+v, %v", changedPolicy, err)
	}
}

func TestVerifyAlignmentRejectsPartialAndCrossProjectIdentity(t *testing.T) {
	base := t.TempDir()
	graph, vector := filepath.Join(base, "graph"), filepath.Join(base, "vector")
	write := func(graphProject, vectorProject string) {
		writeManifest(t, graph, map[string]any{
			"src_commit": "abc", "graph_digest": "d1", "schema_version": "1.23",
			"project_id": graphProject, "snapshot_id": "s1", "dataset_id": "d1",
		})
		writeManifest(t, vector, map[string]any{
			"src_commit": "abc", "project_id": vectorProject, "snapshot_id": "s1", "dataset_id": "d1",
			"sources": map[string]any{"ckg": map[string]any{"graph_digest": "d1", "src_commit": "abc"}},
		})
	}
	write("first", "first")
	if err := VerifyAlignment(graph, vector, nil); err != nil {
		t.Fatalf("aligned IDs rejected: %v", err)
	}
	write("first", "second")
	if err := VerifyAlignment(graph, vector, nil); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("cross-project IDs accepted: %v", err)
	}
	write("first", "")
	if err := VerifyAlignment(graph, vector, nil); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("partially pinned indexes accepted: %v", err)
	}
}

func TestCandidateIdentityRequiresBothEnginePins(t *testing.T) {
	version := t.TempDir()
	writeManifest(t, filepath.Join(version, "graph"), map[string]any{
		"src_commit": "abc", "graph_digest": "g", "schema_version": "1.23",
	})
	writeManifest(t, filepath.Join(version, "vector"), map[string]any{
		"src_commit": "abc", "embedding_model": "mock", "embedding_dim": 8,
		"embedding_checksum": "provider=mock;model=mock;dim=8",
	})
	source := SourceIdentity{ProjectID: "p-one", SourceMode: "committed", SourceCommit: "abc",
		FileManifestDigest: strings.Repeat("a", 64), SnapshotID: strings.Repeat("b", 64)}
	identity, err := PublishCandidateIdentity(version, source, "inputs")
	if err != nil || identity.DatasetID == "" {
		t.Fatalf("publish identity: %+v, %v", identity, err)
	}
	if err := VerifyCandidateIdentity(version, source, "inputs"); err != nil {
		t.Fatalf("candidate verification: %v", err)
	}
	if err := VerifyCandidateIdentity(version, source, "other-inputs"); err == nil {
		t.Fatal("changed inputs accepted")
	}
	var vector map[string]any
	if err := readJSON(filepath.Join(version, "vector", "manifest.json"), &vector); err != nil {
		t.Fatal(err)
	}
	vector["dataset_id"] = "another"
	writeManifest(t, filepath.Join(version, "vector"), vector)
	if err := VerifyCandidateIdentity(version, source, "inputs"); err == nil {
		t.Fatal("mismatched engine dataset accepted")
	}
	vector["dataset_id"] = identity.DatasetID
	vector["embedding_checksum"] = "different-space"
	writeManifest(t, filepath.Join(version, "vector"), vector)
	if err := VerifyCandidateIdentity(version, source, "inputs"); err == nil {
		t.Fatal("changed embedding identity accepted")
	}
}

func TestConfiguredInputDigestDetectsPolicyAndPackChanges(t *testing.T) {
	root := t.TempDir()
	policy := filepath.Join(root, "policy.yaml")
	pack := filepath.Join(root, "pack")
	if err := os.Mkdir(pack, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := Options{PolicyFile: policy, DomainKnowledge: pack}
	first, err := ConfiguredInputDigest(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "fact.yaml"), []byte("id: one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := ConfiguredInputDigest(o)
	if err != nil || first == second {
		t.Fatalf("added pack file did not change input digest: %q %q %v", first, second, err)
	}
	if err := os.Symlink(policy, filepath.Join(pack, "linked.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfiguredInputDigest(o); err == nil {
		t.Fatal("symlink in knowledge pack was followed")
	}
}

func TestPromoteRejectsCrossProjectAndTamperedRollback(t *testing.T) {
	dataset := t.TempDir()
	build := func(version, project string) {
		root := filepath.Join(dataset, version)
		writeManifest(t, filepath.Join(root, "graph"), map[string]any{
			"src_commit": "abc", "graph_digest": "g", "schema_version": "1.23",
		})
		writeManifest(t, filepath.Join(root, "vector"), map[string]any{
			"src_commit": "abc", "embedding_model": "mock", "embedding_dim": 8,
			"embedding_checksum": "mock-space", "sources": map[string]any{
				"ckg": map[string]any{"src_commit": "abc", "graph_digest": "g"},
			},
		})
		source := SourceIdentity{ProjectID: project, SourceMode: "committed", SourceCommit: "abc",
			FileManifestDigest: strings.Repeat("a", 64), SnapshotID: strings.Repeat("b", 64)}
		if _, err := PublishCandidateIdentity(root, source, "inputs"); err != nil {
			t.Fatal(err)
		}
	}
	build("v1", "first")
	build("v2", "second")
	if _, err := Promote(dataset, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "v2"); err == nil || !strings.Contains(err.Error(), "project IDs") {
		t.Fatalf("cross-project promotion accepted: %v", err)
	}
	if current, _ := os.Readlink(filepath.Join(dataset, "current")); current != "v1" {
		t.Fatalf("failed promotion moved current to %q", current)
	}
	if _, err := Promote(dataset, "../outside"); err == nil {
		t.Fatal("path traversal accepted by promote")
	}
	if err := os.Remove(filepath.Join(dataset, "v1", "dataset-identity.json")); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(dataset, "v1"); err == nil {
		t.Fatal("rollback accepted a missing pinned identity")
	}
}
