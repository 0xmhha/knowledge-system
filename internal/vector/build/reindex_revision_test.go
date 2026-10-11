package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/vector/embed/mock"
	"github.com/0xmhha/knowledge-system/internal/vector/manifest"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// A --since value is a revision, never a git option: "--output=<file>" once
// made git diff overwrite that file, report no changes and advance the
// indexed head past the real edit.
func TestReindex_SinceIsNeverAGitOption(t *testing.T) {
	src, out, work := t.TempDir(), t.TempDir(), t.TempDir()
	gitIn(t, src, "init", "-q")
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(src, "a.go"), []byte("package a\n\nfunc A() int { return "+body+" }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("1")
	gitIn(t, src, "add", ".")
	gitIn(t, src, "commit", "-qm", "one")
	if _, err := Run(context.Background(), Options{SrcRoot: src, OutDir: out, Embedder: mock.Default(), Now: func() time.Time { return time.Unix(0, 0).UTC() }}); err != nil {
		t.Fatal(err)
	}
	before, err := manifest.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	write("2")
	gitIn(t, src, "commit", "-qam", "two")

	victim := filepath.Join(work, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, since := range []string{"--output=" + victim, "-p", "--no-such-option"} {
		_, err := Reindex(context.Background(), ReindexOptions{SrcRoot: src, OutDir: out, Embedder: mock.Default(), Since: since})
		if err == nil {
			t.Fatalf("since %q accepted", since)
		}
	}
	if got, _ := os.ReadFile(victim); string(got) != "keep\n" {
		t.Fatalf("victim overwritten: %q", got)
	}
	after, err := manifest.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if after.IndexedHead != before.IndexedHead {
		t.Fatalf("indexed head advanced from %s to %s on a rejected revision", before.IndexedHead, after.IndexedHead)
	}

	// A real revision (short form) still works and indexes the change.
	short := gitIn(t, src, "rev-parse", "--short", before.IndexedHead)
	res, err := Reindex(context.Background(), ReindexOptions{SrcRoot: src, OutDir: out, Embedder: mock.Default(), Since: short[:len(short)-1]})
	if err != nil || res.FilesModified != 1 {
		t.Fatalf("valid since: res=%+v err=%v", res, err)
	}
}
