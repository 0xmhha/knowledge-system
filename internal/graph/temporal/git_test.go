package temporal_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/temporal"
)

// TestTrackLines_NotAGitCheckout degrades to an empty history.
func TestTrackLines_NotAGitCheckout(t *testing.T) {
	dir := t.TempDir()
	h, err := temporal.TrackLines(context.Background(), dir, []string{"*.go"},
		[]temporal.Item{{ID: "f", Path: "a.go"}}, nil)
	if err != nil {
		t.Fatalf("TrackLines on a non-git dir: %v", err)
	}
	if len(h.Commits) != 0 || len(h.Touches) != 0 {
		t.Errorf("expected an empty history, got %d commits, %d touched items", len(h.Commits), len(h.Touches))
	}
}

// TestTrackLines_PathWithSpaces follows a file whose name has a space.
func TestTrackLines_PathWithSpaces(t *testing.T) {
	repo := initRepo(t)
	c1 := commitFile(t, repo, "a file.go", "hello\n", "spaces")
	h, err := temporal.TrackLines(context.Background(), repo, []string{"*.go"},
		[]temporal.Item{{ID: "f", Path: "a file.go"}}, nil)
	if err != nil {
		t.Fatalf("TrackLines: %v", err)
	}
	if got := h.Touches["f"]; len(got) != 1 || got[0].SHA != c1 {
		t.Errorf("history of \"a file.go\" = %+v, want [%s]", got, c1)
	}
}

// TestTrackLines_SubjectKeptWhole keeps a long subject whole: a cut subject
// ends in the middle of a word or a pull request reference.
func TestTrackLines_SubjectKeptWhole(t *testing.T) {
	repo := initRepo(t)
	subject := "fix: " + strings.Repeat("a long subject ", 20) + "(ethereum#29563) (#133)"
	sha := commitFile(t, repo, "a.go", "package a\n", subject)
	h, err := temporal.TrackLines(context.Background(), repo, []string{"*.go"},
		[]temporal.Item{{ID: "f", Path: "a.go"}}, nil)
	if err != nil {
		t.Fatalf("TrackLines: %v", err)
	}
	if got := h.Commits[sha].Subject; got != subject {
		t.Errorf("subject = %q (%d bytes), want the whole subject (%d bytes)", got, len(got), len(subject))
	}
	if got := temporal.PRNumber(h.Commits[sha].Subject); got != 133 {
		t.Errorf("PRNumber = %d, want 133 (the subject's own number, not the cited one)", got)
	}
}

// TestPRNumber reads both subject forms GitHub writes.
func TestPRNumber(t *testing.T) {
	for _, tc := range []struct {
		subject string
		want    int
	}{
		{"core: fix x (#123)", 123},
		{"Merge pull request #456 from someone/branch", 456},
		{"fix: port upstream change (ethereum#29563) (#133)", 133},
		{"params: release Geth v 1.13.15", 0},
		{"refer to #12 in passing", 0},
		{"core: port a fix (#31991) (#53)", 53},
	} {
		if got := temporal.PRNumber(tc.subject); got != tc.want {
			t.Errorf("PRNumber(%q) = %d, want %d", tc.subject, got, tc.want)
		}
	}
	if n := temporal.PRNumbers("core: port a fix (#31991) (#53)"); n.Own != 53 || len(n.Ported) != 1 || n.Ported[0] != 31991 || n.Reverts != nil {
		t.Errorf("PRNumbers = %+v; want 53 and the ported 31991", n)
	}
	if n := temporal.PRNumbers(`Revert "core: bad change (#120)" (#121)`); n.Own != 121 || len(n.Reverts) != 1 || n.Reverts[0] != 120 || n.Ported != nil {
		t.Errorf("PRNumbers of a revert = %+v; want 121 reverting 120", n)
	}
	if n := temporal.PRNumbers(`Revert "core: bad change (#120)"`); n.Own != 0 || len(n.Reverts) != 1 {
		t.Errorf("PRNumbers of an unmerged revert = %+v; want no own number, reverting 120", n)
	}
}

// --- shared test helpers ---

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	run(t, dir, "git", "config", "user.email", "ckg-test@example.com")
	run(t, dir, "git", "config", "user.name", "ckg-test")
	run(t, dir, "git", "config", "commit.gpgsign", "false")
	return dir
}

func commitFile(t *testing.T, repo, relPath, content, msg string) string {
	t.Helper()
	full := filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	run(t, repo, "git", "add", "--", relPath)
	run(t, repo, "git", "commit", "-q", "-m", msg)
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
