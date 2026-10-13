package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetainedReaderReadsArchiveWithoutTemporaryCopies(t *testing.T) {
	version, captured := retainedVersionForRequest(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	reader, err := OpenRetainedSourceReader(context.Background(), version)
	if err != nil {
		t.Fatal(err)
	}
	data, err := reader(context.Background(), "a.md")
	if err != nil || string(data) != "alpha\n" {
		t.Fatalf("archive read: %q %v", data, err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil || len(entries) != 0 {
		t.Fatal("reader created temporary source copies")
	}
	for _, path := range []string{"../a.md", "/a.md", "@cks/docs/00/a.md", "@cks/docs/0/a.md", "absent.md"} {
		if _, err := reader(context.Background(), path); err == nil {
			t.Fatalf("invalid/absent path accepted: %s", path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader(ctx, "a.md"); err != context.Canceled {
		t.Fatalf("cancellation ignored: %v", err)
	}
	alterRetainedBlob(t, captured, "a.md")
	if _, err := reader(context.Background(), "a.md"); err == nil || !strings.Contains(err.Error(), "snapshot_mismatch") {
		t.Fatalf("changed archive accepted: %v", err)
	}
}

func TestRetainedReaderSeparatesSameNamedCorpusAndRepository(t *testing.T) {
	root, docs, version := t.TempDir(), t.TempDir(), t.TempDir()
	for dir, text := range map[string]string{root: "repository\n", docs: "corpus\n"} {
		if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: version, ProjectID: "reader", SourceMode: "snapshot-only", ExternalOrigins: []CaptureOrigin{{ID: "docs:corpus.0", Root: docs}}})
	if err != nil {
		t.Fatal(err)
	}
	writePinnedMockManifests(t, version, root, "", captured.Identity)
	if _, err := PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenRetainedSourceReader(context.Background(), version)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"a.md": "repository\n", "@cks/docs/0/a.md": "corpus\n"} {
		got, err := reader(context.Background(), path)
		if err != nil || string(got) != want {
			t.Fatalf("%s selected wrong origin: %q %v", path, got, err)
		}
	}
	blob := ""
	for _, f := range captured.Files {
		if f.OriginID == "docs:corpus.0" {
			blob = filepath.Join(captured.BlobDir, f.SHA256)
		}
	}
	if err := os.Remove(blob); err != nil {
		t.Fatal(err)
	}
	if _, err := reader(context.Background(), "@cks/docs/0/a.md"); err == nil {
		t.Fatal("missing corpus fell back to same-named repo")
	}
}
