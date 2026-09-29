package semantic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMarkdownUsesCKVBoundariesAndRawSourceProof(t *testing.T) {
	dir := t.TempDir()
	gitOutput(t, dir, "init", "-q")
	source := []byte("# Guide\r\n```md\r\n# not a heading\r\n```\r\n## Rule\r\nReview is required.\r\n")
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, dir, "add", "guide.md")
	gitOutput(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	snapshot := Snapshot{ProjectID: "sample", DatasetID: "v1", Commit: gitOutput(t, dir, "rev-parse", "HEAD")}
	p, err := ExtractMarkdown(snapshot, "guide.md", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Sections) != 2 || len(p.Evidence) != 2 {
		t.Fatalf("sections=%d evidence=%d, want 2 each", len(p.Sections), len(p.Evidence))
	}
	if p.Evidence[0].StartLine != 1 || p.Evidence[0].EndLine != 4 || p.Evidence[1].StartLine != 5 || p.Evidence[1].EndLine != 6 {
		t.Fatalf("section lines drifted: %+v", p.Evidence)
	}
	if len(p.Sections[1].HeadingPath) != 2 || p.Sections[1].HeadingPath[1] != "Rule" {
		t.Fatalf("heading hierarchy drifted: %+v", p.Sections[1])
	}
	if err := p.ValidateSources(context.Background(), dir); err != nil {
		t.Fatalf("raw CRLF source proof failed: %v", err)
	}
	again, err := ExtractMarkdown(snapshot, "guide.md", source)
	if err != nil || again.Sections[1].ID != p.Sections[1].ID {
		t.Fatalf("section ID is not stable: again=%+v err=%v", again.Sections, err)
	}
}
