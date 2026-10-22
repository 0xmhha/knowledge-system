package testgit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	if err := IsolateFixtures(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func revParse(t *testing.T, dir string) error {
	t.Helper()
	return exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run()
}

// The package directory sits inside this repository but is not its root.
func TestFixtureDirectoryFindsNoRepository(t *testing.T) {
	if err := revParse(t, "."); err == nil {
		t.Fatal("git found the enclosing repository from a fixture directory")
	}
}

func TestOwnRepositoryIsUnaffected(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := revParse(t, filepath.Join(dir, "sub")); err != nil {
		t.Fatalf("temporary repository not found: %v", err)
	}
}

func TestUseRepositoryLiftsTheCeiling(t *testing.T) {
	UseRepository(t)
	if err := revParse(t, "."); err != nil {
		t.Fatalf("enclosing repository not found after UseRepository: %v", err)
	}
}

func TestCommittedCopyHasItsOwnHead(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, second := CommittedCopy(t, src), CommittedCopy(t, src)
	head := func(dir string) string {
		out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatalf("rev-parse HEAD in %s: %v", dir, err)
		}
		return string(out)
	}
	if head(first) != head(second) {
		t.Errorf("copies of the same fixture have different HEADs")
	}
	if _, err := os.Stat(filepath.Join(first, "a.txt")); err != nil {
		t.Errorf("fixture file not copied: %v", err)
	}
	if err := exec.Command("git", "-C", first, "diff", "--quiet", "HEAD").Run(); err != nil {
		t.Errorf("copy has uncommitted changes: %v", err)
	}
}
