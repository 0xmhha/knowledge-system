package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/githistory"
)

func TestCaptureManyFilesAndExternalFailureKeepsOnlyPinnedBytes(t *testing.T) {
	base := t.TempDir()
	repo, docs, out := filepath.Join(base, "repo"), filepath.Join(base, "docs"), filepath.Join(base, "candidate")
	for _, dir := range []string{repo, docs} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 500; i++ {
		name := filepath.Join(repo, fmt.Sprintf("source-%04d.go", i))
		if err := os.WriteFile(name, []byte("package fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	doc := filepath.Join(docs, "policy.md")
	if err := os.WriteFile(doc, []byte("reviewed external rule\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := CaptureOptions{Root: repo, Out: out, ProjectID: "many", SourceMode: "snapshot-only",
		ExternalOrigins: []CaptureOrigin{{ID: "knowledge:fixture", Root: docs}}, MaxTotalBytes: int64(500*len("package fixture\n") + len("reviewed external rule\n"))}
	if err := os.Symlink(doc, filepath.Join(docs, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureSource(opts); err == nil {
		t.Fatal("linked external document was accepted after a large repository inventory")
	}
	if err := os.Remove(filepath.Join(docs, "linked.md")); err != nil {
		t.Fatal(err)
	}
	opts.Out = filepath.Join(base, "too-small")
	opts.MaxTotalBytes--
	if _, err := CaptureSource(opts); err == nil || !strings.Contains(err.Error(), "total byte limit") {
		t.Fatalf("external bytes escaped the shared capture budget: %v", err)
	}
	opts.Out, opts.MaxTotalBytes = filepath.Join(base, "valid"), opts.MaxTotalBytes+1
	captured, err := CaptureSource(opts)
	if err != nil || len(captured.Files) != 501 {
		t.Fatalf("large capture failed: %d files, %v", len(captured.Files), err)
	}
	if err := os.WriteFile(doc, []byte("changed external rule\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := captured.VerifyAgainst(repo); err == nil {
		t.Fatal("external document replacement did not invalidate the live source")
	}
	if err := VerifyRetainedSource(opts.Out, captured.Identity); err != nil {
		t.Fatalf("old retained tree changed after external replacement: %v", err)
	}
}

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
	for _, name := range []string{".env.production", "config/ID_RSA", "app/Secrets/token.txt", "App/.AWS/credentials", "nested/cert.PEM", "private/API.KEY", "nested/credentials.json", "config/.npmrc", "keys/id_ed25519.pub", "keys/client.P12"} {
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

func TestCaptureEnforcesCombinedFileCountAcrossOrigins(t *testing.T) {
	base := t.TempDir()
	repo, external := filepath.Join(base, "repo"), filepath.Join(base, "external")
	for _, dir := range []string{repo, external} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "one.go"), []byte("package one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "policy.md"), []byte("reviewed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := CaptureSource(CaptureOptions{Root: repo, Out: filepath.Join(base, "candidate"), ProjectID: "p",
		SourceMode: "snapshot-only", MaxFiles: 1, ExternalOrigins: []CaptureOrigin{{ID: "knowledge:fixture", Root: external}}})
	if err == nil || !strings.Contains(err.Error(), "file count limit") {
		t.Fatalf("external source escaped combined count limit: %v", err)
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

func TestCaptureRejectsReplacementAtLstatOpenBoundary(t *testing.T) {
	for _, replacement := range []string{"linked-leaf", "linked-parent", "same-bytes-new-inode"} {
		t.Run(replacement, func(t *testing.T) {
			rootPath, outside := t.TempDir(), t.TempDir()
			if err := os.Mkdir(filepath.Join(rootPath, "nested"), 0o700); err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(rootPath, "nested", "policy.md")
			if err := os.WriteFile(original, []byte("reviewed rule\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			external := filepath.Join(outside, "policy.md")
			if err := os.WriteFile(external, []byte("outside rule\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			_, err = readCapturedRegularWithOpenHook(root, "nested/policy.md", 1024, func() {
				switch replacement {
				case "linked-leaf":
					if err := os.Remove(original); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(external, original); err != nil {
						t.Fatal(err)
					}
				case "linked-parent":
					if err := os.Rename(filepath.Join(rootPath, "nested"), filepath.Join(rootPath, "old")); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(outside, filepath.Join(rootPath, "nested")); err != nil {
						t.Fatal(err)
					}
				case "same-bytes-new-inode":
					if err := os.Rename(original, filepath.Join(rootPath, "old-policy.md")); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(original, []byte("reviewed rule\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			})
			if err == nil {
				t.Fatal("replaced source was accepted across the Lstat/Open boundary")
			}
		})
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

func TestGitHistoryArchiveLimitTamperAndLegacyV3Read(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "base")
	head := identityGit(t, root, "rev-parse", "HEAD")
	limited := filepath.Join(t.TempDir(), "limited")
	if _, err := CaptureSource(CaptureOptions{Root: root, Out: limited, ProjectID: "p",
		SourceMode: "committed", SourceCommit: head, MaxHistoryBytes: 1}); err == nil || !strings.Contains(err.Error(), "archive exceeds") {
		t.Fatalf("history byte limit did not fail closed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(limited, "sources", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("oversized history published source manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(limited, "sources", "history.bundle")); !os.IsNotExist(err) {
		t.Fatalf("oversized partial Git bundle retained: %v", err)
	}
	out := filepath.Join(t.TempDir(), "candidate")
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: out, ProjectID: "p",
		SourceMode: "committed", SourceCommit: head})
	if err != nil || captured.GitHistory == nil {
		t.Fatalf("capture Git history: %+v %v", captured.GitHistory, err)
	}
	bundle := filepath.Join(out, "sources", "history.bundle")
	original, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bundle); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRetainedSource(out, captured.Identity); err == nil || !strings.Contains(err.Error(), "source_missing") {
		t.Fatalf("missing Git history archive accepted: %v", err)
	}
	corrupt := append([]byte(nil), original...)
	corrupt[len(corrupt)-1] ^= 1
	if err := os.WriteFile(bundle, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRetainedSource(out, captured.Identity); err == nil || !strings.Contains(err.Error(), "snapshot_mismatch") {
		t.Fatalf("changed Git history archive accepted: %v", err)
	}
	if err := os.WriteFile(bundle, original, 0o600); err != nil {
		t.Fatal(err)
	}
	// Previously published v4 used the byte-only file manifest. It and the
	// earlier archive-free v3 remain readable without rewriting their IDs.
	legacy := captured
	legacy.Identity.FileModePolicy = ""
	legacyFiles := make([]sourceFile, 0, len(legacy.Files))
	for _, f := range legacy.Files {
		legacyFiles = append(legacyFiles, sourceFile{OriginID: f.OriginID, Path: f.Path,
			Kind: f.Kind, Size: f.Size, SHA256: f.SHA256})
	}
	legacy.Identity.FileManifestDigest = fileManifestDigest(legacyFiles)
	legacy.Identity.SnapshotID = sourceSnapshotID(legacy.Identity)
	if err := writeJSONAtomic(filepath.Join(out, "sources", "manifest.json"), legacy); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRetainedSource(out, legacy.Identity); err != nil {
		t.Fatalf("legacy v4 retained source rejected: %v", err)
	}
	legacy.Identity.GitArchivePolicy = ""
	legacy.Identity.SnapshotID = sourceSnapshotID(legacy.Identity)
	legacy.GitHistory = nil
	if err := writeJSONAtomic(filepath.Join(out, "sources", "manifest.json"), legacy); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRetainedSource(out, legacy.Identity); err != nil {
		t.Fatalf("legacy v3 retained source rejected: %v", err)
	}
}

func TestGitArchiveCommittedExecutableModeSurvivesRestore(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	script := filepath.Join(root, "run.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "base")
	head := identityGit(t, root, "rev-parse", "HEAD")
	out := filepath.Join(t.TempDir(), "candidate")
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: out,
		ProjectID: "p", SourceMode: "committed", SourceCommit: head})
	if err != nil || captured.Identity.FileModePolicy != fileModePolicyV1 || !captured.Files[0].Executable {
		t.Fatalf("executable capture: %+v %v", captured, err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(t.TempDir(), "build")
	cleanup, err := captured.MaterializeBuildTree(build)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup() }()
	info, err := os.Stat(filepath.Join(build, "run.sh"))
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("restored script lost executable bit: %v %v", info, err)
	}
	if status := identityGit(t, build, "status", "--porcelain"); status != "" {
		t.Fatalf("committed archive checkout is dirty: %q", status)
	}
	if clean, err := testGateSourceClean(build, head); err != nil || !clean {
		t.Fatalf("committed promotion gate rejected restored source: %v %v", clean, err)
	}
	forged := captured
	forged.Files = append([]CapturedFile(nil), captured.Files...)
	forged.Files[0].Executable = false
	if err := writeJSONAtomic(filepath.Join(out, "sources", "manifest.json"), forged); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRetainedSource(out, captured.Identity); err == nil || !strings.Contains(err.Error(), "snapshot_mismatch") {
		t.Fatalf("retained executable bit forgery was accepted: %v", err)
	}
}

func TestWorkingTreeExecutableBitChangesSnapshotAndStaging(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	script := filepath.Join(root, "run.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "base")
	head := identityGit(t, root, "rev-parse", "HEAD")
	before, err := SnapshotSourceIdentity(root, "p", "working-tree", head)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	after, err := SnapshotSourceIdentity(root, "p", "working-tree", head)
	if err != nil || before.FileManifestDigest == after.FileManifestDigest || before.SnapshotID == after.SnapshotID {
		t.Fatalf("executable-bit change reused snapshot: before=%+v after=%+v err=%v", before, after, err)
	}
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: filepath.Join(t.TempDir(), "candidate"),
		ProjectID: "p", SourceMode: "working-tree", SourceCommit: head})
	if err != nil || captured.Identity != after {
		t.Fatalf("working-tree mode capture differs from identity: %v", err)
	}
	build := filepath.Join(t.TempDir(), "build")
	cleanup, err := captured.MaterializeBuildTree(build)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup() }()
	info, err := os.Stat(filepath.Join(build, "run.sh"))
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("staged script lost executable bit: %v %v", info, err)
	}
	if err := os.Chmod(script, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := captured.VerifyAgainst(root); err == nil || !strings.Contains(err.Error(), "mode changed") {
		t.Fatalf("source mode drift was accepted: %v", err)
	}
}

func TestGitHistoryArchiveFromLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	identityGit(t, root, "init", "-q")
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "base")
	base := identityGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("package main\nvar Lost = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "add", ".")
	identityGit(t, root, "-c", "commit.gpgsign=false", "-c", "user.email=t@example.org", "-c", "user.name=Test", "commit", "-qm", "abandoned")
	lost := identityGit(t, root, "rev-parse", "HEAD")
	identityGit(t, root, "reset", "--hard", base)
	linked := filepath.Join(t.TempDir(), "linked")
	identityGit(t, root, "worktree", "add", "--detach", linked, base)
	out := filepath.Join(t.TempDir(), "candidate")
	captured, err := CaptureSource(CaptureOptions{Root: linked, Out: out,
		ProjectID: "p", SourceMode: "committed", SourceCommit: base})
	if err != nil {
		t.Fatal(err)
	}
	identityGit(t, root, "worktree", "remove", "--force", linked)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(t.TempDir(), "build")
	cleanup, err := captured.MaterializeBuildTree(build)
	if err != nil {
		t.Fatalf("linked-worktree archive could not restore without common object store: %v", err)
	}
	defer func() { _ = cleanup() }()
	selected, err := githistory.RecoveryCommitIDs(build, 0)
	if err != nil || len(selected) != 1 || selected[0] != lost {
		t.Fatalf("linked-worktree recovery commit missing: %v %v", selected, err)
	}
}
