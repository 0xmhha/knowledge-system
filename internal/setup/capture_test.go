package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureNonGitSourceRetainsOldBytesAndDetectsCorruption(t *testing.T) {
	root, out := filepath.Join(t.TempDir(), "source"), filepath.Join(t.TempDir(), "candidate")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := CaptureSource(CaptureOptions{Root: root, Out: out, ProjectID: "p-one", SourceMode: "snapshot-only"})
	if err != nil || len(first.Files) != 1 {
		t.Fatalf("capture: %+v %v", first, err)
	}
	if err := os.WriteFile(file, []byte("package second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := first.VerifyAgainst(root); err == nil {
		t.Fatal("source changed after capture but still verified")
	}
	old, err := first.ReadBlob(first.Files[0].SHA256)
	if err != nil || string(old) != "package first\n" {
		t.Fatalf("past source bytes unavailable: %q %v", old, err)
	}
	second, err := CaptureSource(CaptureOptions{Root: root, Out: filepath.Join(t.TempDir(), "second"), ProjectID: "p-one", SourceMode: "snapshot-only"})
	if err != nil || second.Identity.SnapshotID == first.Identity.SnapshotID {
		t.Fatalf("changed non-Git file reused snapshot: %+v %v", second, err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := second.VerifyAgainst(root); err == nil || !strings.Contains(err.Error(), "file set") {
		t.Fatalf("new file after capture went unnoticed: %v", err)
	}
	blob := filepath.Join(first.BlobDir, first.Files[0].SHA256)
	if err := os.Chmod(blob, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blob, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := first.ReadBlob(first.Files[0].SHA256); err == nil || !strings.Contains(err.Error(), "snapshot_mismatch") {
		t.Fatalf("corrupt past source accepted: %v", err)
	}
}

func TestCaptureRefusesLinksSecretsAndRecursiveOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := CaptureOptions{Root: root, Out: filepath.Join(t.TempDir(), "candidate"), ProjectID: "p", SourceMode: "snapshot-only"}
	if _, err := CaptureSource(CaptureOptions{Root: root, Out: filepath.Join(root, "candidate"), ProjectID: "p", SourceMode: "snapshot-only"}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("recursive candidate accepted: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "README.md"), filepath.Join(root, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureSource(o); err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("symlink accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureSource(o); err == nil || !strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("secret file captured: %v", err)
	}
}

func TestWorkingTreeCaptureMaterializesModifiedAndNewBytesWithBaseHistory(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "base")
	head := identityGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("package after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.md"), []byte("# New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CaptureSource(CaptureOptions{Root: root, Out: filepath.Join(t.TempDir(), "candidate"),
		ProjectID: "p", SourceMode: "working-tree", SourceCommit: head})
	if err != nil || len(result.Files) != 2 {
		t.Fatalf("capture working tree: %+v %v", result, err)
	}
	build := filepath.Join(t.TempDir(), "build")
	cleanup, err := result.MaterializeBuildTree(build)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup() }()
	if got := identityGit(t, build, "rev-parse", "HEAD"); got != head {
		t.Fatalf("staged Git base changed: %q", got)
	}
	for file, want := range map[string]string{"main.go": "package after\n", "new.md": "# New\n"} {
		buf, err := os.ReadFile(filepath.Join(build, file))
		if err != nil || string(buf) != want {
			t.Fatalf("staged file %s: %q %v", file, buf, err)
		}
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	cleanup = func() error { return nil }
	if _, err := os.Stat(build); !os.IsNotExist(err) {
		t.Fatalf("temporary build root retained: %v", err)
	}
}
