package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSourceLineBytesPreservesCRLFAndFinalLine(t *testing.T) {
	data := []byte("first\r\nsecond\nlast")
	span, err := sourceLineBytes(data, 1, 2)
	if err != nil || string(span) != "first\r\nsecond\n" {
		t.Fatalf("CRLF span changed: %q %v", span, err)
	}
	span, err = sourceLineBytes(data, 3, 3)
	if err != nil || string(span) != "last" {
		t.Fatalf("final line changed: %q %v", span, err)
	}
	if _, err := sourceLineBytes([]byte("one\n"), 2, 2); err == nil {
		t.Fatal("phantom line after final LF accepted")
	}
}

func TestRetainedBatchPreservesSpansAndRechecksUnselectedDamage(t *testing.T) {
	root, version := t.TempDir(), t.TempDir()
	for name, text := range map[string]string{"README.md": "first\r\nsecond\nlast", "unused.md": "uncited archived source\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: version, ProjectID: "batch", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	writePinnedMockManifests(t, version, root, "", captured.Identity)
	if _, err := PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	spans := []RetainedLineSpan{{"repo", "README.md", 3, 3}, {"repo", "README.md", 1, 2}, {"repo", "README.md", 3, 3}}
	want := make([]RetainedEvidence, 0, len(spans))
	for _, s := range spans {
		e, err := ReadRetainedLines(version, s.OriginID, s.Path, s.First, s.Last)
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, e)
	}
	got, err := ReadRetainedLineSpans(version, spans)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("batch changed bytes, ordering, duplicates or source identity: %+v %v", got, err)
	}
	for _, invalid := range []RetainedLineSpan{{"repo", "../README.md", 1, 1}, {"knowledge:absent", "README.md", 1, 1}, {"repo", "README.md", 4, 4}} {
		if out, err := ReadRetainedLineSpans(version, append(spans, invalid)); err == nil || out != nil {
			t.Fatalf("invalid batch returned partial evidence: %+v %v", out, err)
		}
	}
	var unused CapturedFile
	for _, f := range captured.Files {
		if f.Path == "unused.md" {
			unused = f
		}
	}
	path := filepath.Join(captured.BlobDir, unused.SHA256)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("uncited altered source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := ReadRetainedLineSpans(version, spans); err == nil || out != nil {
		t.Fatalf("previous verification hid unselected archive damage: %+v %v", out, err)
	}
}

func TestRetainedCitationUsesPastBytesAndRejectsDamage(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	file := filepath.Join(root, "README.md")
	old := []byte("# Header\r\nPast reason\n")
	if err := os.WriteFile(file, old, 0o644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "base")
	head := identityGit(t, root, "rev-parse", "HEAD")
	version := t.TempDir()
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: version,
		ProjectID: "p-one", SourceMode: "committed", SourceCommit: head})
	if err != nil {
		t.Fatal(err)
	}
	writePinnedMockManifests(t, version, root, head, captured.Identity)
	if _, err := PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("# Header\nNew reason\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evidence, err := ReadRetainedLines(version, "repo", "README.md", 2, 2)
	if err != nil || evidence.Text != "Past reason\n" || evidence.CommitHash != head ||
		evidence.SnapshotID != captured.Identity.SnapshotID {
		t.Fatalf("past citation read mutable HEAD or wrong coordinate: %+v %v", evidence, err)
	}
	sum := sha256.Sum256([]byte("Past reason\n"))
	if evidence.ContentSHA256 != hex.EncodeToString(sum[:]) || evidence.FileSHA256 != captured.Files[0].SHA256 {
		t.Fatalf("v2 source hashes incorrect: %+v", evidence)
	}
	readTree, cleanup, err := MaterializeRetainedReadTree(version)
	if err != nil {
		t.Fatalf("materialize archive for retrieval: %v", err)
	}
	retained, err := os.ReadFile(filepath.Join(readTree, "README.md"))
	if err != nil || string(retained) != string(old) {
		t.Fatalf("retrieval tree used live source: %q %v", retained, err)
	}
	cleanup()
	if _, err := os.Stat(readTree); !os.IsNotExist(err) {
		t.Fatalf("retrieval tree not removed: %v", err)
	}
	if _, err := ReadRetainedLines(version, "repo", "../README.md", 1, 1); err == nil {
		t.Fatal("traversal citation accepted")
	}
	if _, err := ReadRetainedLines(version, "repo", "README.md", 3, 3); err == nil {
		t.Fatal("out-of-range citation accepted")
	}
	if err := os.Remove(filepath.Join(captured.BlobDir, captured.Files[0].SHA256)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRetainedLines(version, "repo", "README.md", 2, 2); err == nil || !strings.Contains(err.Error(), "source_missing") {
		t.Fatalf("missing historical source fell back to live file: %v", err)
	}
}

func TestRetainedKnowledgeCitationUsesOriginAndArchivedBytes(t *testing.T) {
	root, pack, version := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "policy.md"), []byte("repository note\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(pack, "policy.md")
	if err := os.WriteFile(policy, []byte("reviewed policy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: version, ProjectID: "p-one",
		SourceMode: "snapshot-only", ExternalOrigins: []CaptureOrigin{{ID: "knowledge:decisions", Root: pack}}})
	if err != nil {
		t.Fatal(err)
	}
	writePinnedMockManifests(t, version, root, "", captured.Identity)
	if _, err := PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("changed policy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence, err := ReadRetainedLines(version, "knowledge:decisions", "policy.md", 1, 1)
	if err != nil || evidence.Text != "reviewed policy\n" || evidence.OriginID != "knowledge:decisions" || evidence.CommitHash != "" {
		t.Fatalf("external citation did not use retained origin: %+v %v", evidence, err)
	}
	repoEvidence, err := ReadRetainedLines(version, "repo", "policy.md", 1, 1)
	if err != nil || repoEvidence.Text != "repository note\n" {
		t.Fatalf("repo origin crossed: %+v %v", repoEvidence, err)
	}
	if _, err := ReadRetainedLines(version, "knowledge:other", "policy.md", 1, 1); err == nil {
		t.Fatal("unregistered external origin returned citation")
	}
}

func writePinnedMockManifests(t *testing.T, version, root, commit string, source SourceIdentity) {
	t.Helper()
	identity, err := NewDatasetIdentity(source, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "inputs")
	if err != nil {
		t.Fatal(err)
	}
	graph := map[string]any{"src_commit": commit, "graph_digest": "g", "schema_version": "1.23", "src_root": root}
	vector := map[string]any{"src_commit": commit, "src_root": root, "embedding_model": "mock", "embedding_dim": 8,
		"embedding_checksum": "mock-space", "sources": map[string]any{
			"ckg": map[string]any{"src_commit": commit, "graph_digest": "g"}}}
	for _, manifest := range []map[string]any{graph, vector} {
		addNativePins(manifest, source, identity.DatasetID)
	}
	writeManifest(t, filepath.Join(version, "graph"), graph)
	writeManifest(t, filepath.Join(version, "vector"), vector)
}
