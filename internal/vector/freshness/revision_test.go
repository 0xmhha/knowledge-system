package freshness

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// An indexed head read from a manifest is a revision, never a git option.
func TestCheckNeverPassesIndexedHeadAsAnOption(t *testing.T) {
	src, work := t.TempDir(), t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", src, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-qm", "one")
	victim := filepath.Join(work, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Check(src, "--output="+victim)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(victim); string(got) != "keep\n" {
		t.Fatalf("victim overwritten: %q", got)
	}
	if r.Fresh || len(r.ChangedFiles) != 0 {
		t.Fatalf("option-like head treated as a revision: %+v", r)
	}
}
