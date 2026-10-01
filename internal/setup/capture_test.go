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
	if err := os.Remove(blob); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "old.go")
	if err := os.WriteFile(outside, []byte("package first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, blob); err != nil {
		t.Fatal(err)
	}
	if _, err := first.ReadBlob(first.Files[0].SHA256); err == nil || !strings.Contains(err.Error(), "snapshot_mismatch") {
		t.Fatalf("linked retained source accepted: %v", err)
	}
}

func TestCaptureExternalKnowledgeOriginKeepsSamePathDistinct(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	pack := filepath.Join(base, "pack")
	for _, dir := range []string{repo, pack} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(repo, "policy.md"): "code documentation\n",
		filepath.Join(pack, "policy.md"): "organization policy\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	origin := CaptureOrigin{ID: "knowledge:engineering.decisions", Root: pack}
	want, err := SnapshotSourceIdentity(repo, "pilot", "snapshot-only", "", origin)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(base, "candidate")
	got, err := CaptureSource(CaptureOptions{Root: repo, Out: out, ProjectID: "pilot",
		SourceMode: "snapshot-only", ExternalOrigins: []CaptureOrigin{origin}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Identity != want || len(got.Files) != 2 {
		t.Fatalf("identity or file count: %+v", got)
	}
	if got.Files[0].OriginID == got.Files[1].OriginID || got.Files[0].SHA256 == got.Files[1].SHA256 {
		t.Fatalf("same-path origins collapsed: %+v", got.Files)
	}
	if err := VerifyRetainedSource(out, want); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "policy.md"), []byte("changed policy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := got.VerifyAgainst(repo); err == nil {
		t.Fatal("changed external source accepted")
	}
	if err := VerifyRetainedSource(out, want); err != nil {
		t.Fatalf("retained old bytes lost: %v", err)
	}
	if err := os.Symlink(filepath.Join(pack, "policy.md"), filepath.Join(pack, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotSourceIdentity(repo, "pilot", "snapshot-only", "", origin); err == nil {
		t.Fatal("linked external file accepted")
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

func TestCaptureSecretPathClassificationAcrossNestedCaseVariants(t *testing.T) {
	for _, name := range []string{".env.production", "config/ID_RSA", "app/Secrets/token.txt", "App/.AWS/credentials", "nested/cert.PEM", "private/API.KEY"} {
		if !sensitiveCapturePath(name) {
			t.Fatalf("sensitive path was not rejected: %q", name)
		}
	}
	for _, name := range []string{"README.md", "app/secretary.go", "docs/keys.md"} {
		if sensitiveCapturePath(name) {
			t.Fatalf("ordinary path was rejected: %q", name)
		}
	}
}

func TestCaptureEnforcesFileAndTotalByteLimits(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("12345"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	options := CaptureOptions{Root: root, Out: filepath.Join(t.TempDir(), "candidate"), ProjectID: "p", SourceMode: "snapshot-only", MaxFileBytes: 4}
	if _, err := CaptureSource(options); err == nil || !strings.Contains(err.Error(), "file byte limit") {
		t.Fatalf("oversized file accepted: %v", err)
	}
	options.MaxFileBytes = 5
	options.MaxTotalBytes = 9
	if _, err := CaptureSource(options); err == nil || !strings.Contains(err.Error(), "total byte limit") {
		t.Fatalf("oversized aggregate accepted: %v", err)
	}
}

func TestReadSourceFileNoFollowRejectsLinkedParentAndLeaf(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "policy.md"), []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSourceFileNoFollow(root, "linked/policy.md", 1024); err == nil {
		t.Fatal("linked parent escaped the source root")
	}
	if err := os.Symlink(filepath.Join(outside, "policy.md"), filepath.Join(root, "leaf.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSourceFileNoFollow(root, "leaf.md", 1024); err == nil {
		t.Fatal("linked leaf escaped the source root")
	}
}

func TestCaptureRejectsNonPortableNamesAndCaseCollisions(t *testing.T) {
	for _, paths := range [][]string{
		{"A.go", "a.go"},
		{"e\u0301.go"},
		{"nested/../main.go"},
		{"../escape.go"},
	} {
		if err := validateCapturedPaths(paths); err == nil {
			t.Fatalf("non-portable source paths accepted: %q", paths)
		}
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

func TestCommittedCaptureMatchesPrebuildIdentityAndRetainsBytes(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	for name, body := range map[string]string{"main.go": "package sample\n", "README.md": "# Sample\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "base")
	head := identityGit(t, root, "rev-parse", "HEAD")
	want, err := CommittedSourceIdentity(root, "project-one", head)
	if err != nil {
		t.Fatal(err)
	}
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: filepath.Join(t.TempDir(), "candidate"),
		ProjectID: "project-one", SourceMode: "committed", SourceCommit: head})
	if err != nil || captured.Identity != want || len(captured.Files) != 2 {
		t.Fatalf("committed capture differs from pre-build identity: %+v, want %+v: %v", captured, want, err)
	}
	if err := captured.VerifyBlobs(); err != nil {
		t.Fatal(err)
	}
	version := filepath.Dir(filepath.Dir(captured.BlobDir))
	if err := VerifyRetainedSource(version, want); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := captured.VerifyAgainst(root); err == nil {
		t.Fatal("changed committed source was accepted")
	}
	if err := captured.VerifyBlobs(); err != nil {
		t.Fatalf("old citation bytes lost after live source changed: %v", err)
	}
	if err := os.Remove(filepath.Join(captured.BlobDir, captured.Files[0].SHA256)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRetainedSource(version, want); err == nil || !strings.Contains(err.Error(), "source_missing") {
		t.Fatalf("missing retained citation bytes accepted: %v", err)
	}
}
