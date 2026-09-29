package setupcli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo creates a committed single-file git repository and returns its
// path and HEAD commit hash.
func gitRepo(t *testing.T) (dir, head string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("`git` not in PATH")
	}
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	head, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return dir, head
}

func TestResolveAutoVersion(t *testing.T) {
	src, head := gitRepo(t)

	// Without a filelist config the name is the commit prefix alone.
	v, err := resolveAutoVersion("", src, "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if v != head[:8] {
		t.Errorf("version = %q, want %q", v, head[:8])
	}

	// With a filelist config the config hash is appended.
	cfg := filepath.Join(t.TempDir(), "filelist.yaml")
	body := []byte("build_roots: [./cmd/app]\n")
	if err := os.WriteFile(cfg, body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	want := head[:8] + "-" + hex.EncodeToString(sum[:])[:8]
	if v, err = resolveAutoVersion("", src, cfg); err != nil {
		t.Fatalf("resolve with config: %v", err)
	}
	if v != want {
		t.Errorf("version = %q, want %q", v, want)
	}

	// An existing version directory under out is refused.
	out := t.TempDir()
	if err := os.MkdirAll(filepath.Join(out, want), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAutoVersion(out, src, cfg); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("existing version dir not refused: %v", err)
	}

	// CKV indexes eligible untracked files, so commit-named versions must
	// reject them rather than aliasing different bytes at the same HEAD.
	if err := os.WriteFile(filepath.Join(src, "untracked.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAutoVersion("", src, ""); err == nil || !strings.Contains(err.Error(), "tracked or untracked") {
		t.Errorf("untracked indexable source was accepted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAutoVersion("", src, ""); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Errorf("dirty tracked tree not refused: %v", err)
	}

	// A non-repo src fails closed.
	if _, err := resolveAutoVersion("", t.TempDir(), ""); err == nil {
		t.Error("non-git src not refused")
	}
	if _, err := resolveAutoVersion("", "", ""); err == nil {
		t.Error("empty src not refused")
	}
}

func TestResolveAutoVersionRejectsGitIgnoredIndexableSource(t *testing.T) {
	src, _ := gitRepo(t)
	if err := os.WriteFile(filepath.Join(src, ".gitignore"), []byte("*.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", src, "add", ".gitignore")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	cmd = exec.Command("git", "-C", src, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-qm", "ignore")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(src, "ignored.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := gitOutput(src, "status", "--porcelain")
	if err != nil || status != "" {
		t.Fatalf("ignored fixture not clean in Git: %q, %v", status, err)
	}
	if _, err := resolveAutoVersion("", src, ""); err == nil || !strings.Contains(err.Error(), "Git-ignored") {
		t.Fatalf("Git-ignored Go source accepted under commit version: %v", err)
	}
}
