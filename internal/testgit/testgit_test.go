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
