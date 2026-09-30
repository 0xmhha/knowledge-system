package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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
	writeManifest(t, filepath.Join(version, "graph"), map[string]any{
		"src_commit": head, "graph_digest": "g", "schema_version": "1.23", "src_root": root})
	writeManifest(t, filepath.Join(version, "vector"), map[string]any{
		"src_commit": head, "src_root": root, "embedding_model": "mock", "embedding_dim": 8,
		"embedding_checksum": "mock-space", "sources": map[string]any{
			"ckg": map[string]any{"src_commit": head, "graph_digest": "g"}}})
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
