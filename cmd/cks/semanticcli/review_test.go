package semanticcli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestReviewCommandValidatesSourceBeforeReporting(t *testing.T) {
	dir := t.TempDir()
	testGit(t, dir, "init", "-q")
	content := []byte("# Spec\nA claim needs review.\n")
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", "spec.md")
	testGit(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "source")
	snap := semantic.Snapshot{ProjectID: "fixture", DatasetID: "d1", Commit: testGit(t, dir, "rev-parse", "HEAD")}
	sum := sha256.Sum256([]byte("A claim needs review.\n"))
	p := semantic.Projection{SchemaVersion: semantic.SchemaVersion, Snapshot: snap,
		Evidence: []semantic.EvidenceSpan{{ID: "e", Snapshot: snap, Kind: semantic.SourceDocument,
			Path: "spec.md", StartLine: 2, EndLine: 2, ContentSHA256: hex.EncodeToString(sum[:]), Extractor: "test"}},
		Sections: []semantic.DocumentSection{{ID: "s", Heading: "Spec", EvidenceID: "e"}},
		Claims: []semantic.Claim{{ID: "c", Statement: "A claim needs review", SectionID: "s",
			EvidenceIDs: []string{"e"}, Status: semantic.StatusProposed}},
	}
	input := filepath.Join(dir, "projection.json")
	write := func() {
		t.Helper()
		buf, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(input, buf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write()
	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"review", "--input", input, "--repo", dir, "--sample", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report semantic.ReviewReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Proposed != 1 || report.Precision != nil || len(report.Sample) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	p.Evidence[0].ContentSHA256 = strings.Repeat("0", 64)
	write()
	cmd = NewCmd()
	cmd.SetArgs([]string{"review", "--input", input, "--repo", dir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("source mismatch allowed: %v", err)
	}
}
