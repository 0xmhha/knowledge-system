package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fixture(t *testing.T) (Projection, string) {
	t.Helper()
	dir := t.TempDir()
	gitOutput(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# Guide\nThe switch requires a review.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, dir, "add", "guide.md")
	gitOutput(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	commit := gitOutput(t, dir, "rev-parse", "HEAD")
	snapshot := Snapshot{ProjectID: "sample", DatasetID: "v1", Commit: commit}
	sum := sha256.Sum256([]byte("The switch requires a review.\n"))
	projection := Projection{
		SchemaVersion: SchemaVersion, Snapshot: snapshot,
		Evidence: []EvidenceSpan{{ID: "e1", Snapshot: snapshot, Kind: SourceDocument,
			Path: "guide.md", StartLine: 2, EndLine: 2, ContentSHA256: hex.EncodeToString(sum[:]), Extractor: "markdown-v1"}},
		Sections: []DocumentSection{{ID: "s1", Heading: "Guide", EvidenceID: "e1"}},
		Claims: []Claim{{ID: "c1", Statement: "The switch requires review", SectionID: "s1",
			EvidenceIDs: []string{"e1"}, Status: StatusProposed}},
	}
	return projection, dir
}

func TestProjectionValidatesSnapshotAndCommittedSource(t *testing.T) {
	p, dir := fixture(t)
	if err := p.ValidateSources(context.Background(), dir); err != nil {
		t.Fatalf("valid projection: %v", err)
	}
	// The committed snapshot remains the source of truth after a local edit.
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# Guide\nChanged in working tree.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateSources(context.Background(), dir); err != nil {
		t.Fatalf("working-tree edit changed committed evidence: %v", err)
	}
	p.Claims[0].Status = StatusVerified
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "reviewed_by") {
		t.Fatalf("unreviewed verified claim accepted: %v", err)
	}
	p.Claims[0].ReviewedBy = "reviewer@example.com"
	if err := p.ValidateSources(context.Background(), dir); err != nil {
		t.Fatalf("reviewed claim with source proof: %v", err)
	}
}

func TestNonCommittedSemanticEvidenceRequiresArchive(t *testing.T) {
	p, repo := fixture(t)
	p.Snapshot.SourceMode = "working-tree"
	p.Snapshot.SnapshotID = strings.Repeat("a", 64)
	p.Evidence[0].Snapshot = p.Snapshot
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateSources(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "requires_v2") {
		t.Fatalf("non-committed evidence read from Git: %v", err)
	}
	p.Snapshot.SourceMode = "snapshot-only"
	p.Snapshot.Commit = ""
	p.Evidence[0].Snapshot = p.Snapshot
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionRejectsCrossSnapshotOrMissingEvidence(t *testing.T) {
	p, dir := fixture(t)
	p.Evidence[0].Snapshot.DatasetID = "other"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "crosses") {
		t.Fatalf("cross-snapshot evidence accepted: %v", err)
	}
	p, _ = fixture(t)
	p.Claims[0].EvidenceIDs = []string{"missing"}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "missing evidence") {
		t.Fatalf("orphan claim evidence accepted: %v", err)
	}
	p, _ = fixture(t)
	p.Evidence[0].Path = "../secret.md"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "location") {
		t.Fatalf("path traversal accepted: %v", err)
	}
	p, dir = fixture(t)
	p.Evidence[0].ContentSHA256 = strings.Repeat("0", 64)
	if err := p.ValidateSources(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("incorrect source digest accepted or misreported: %v", err)
	}
}
