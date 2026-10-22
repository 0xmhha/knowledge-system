// Package testgit keeps tests that build fixtures stored inside this
// repository from reading the repository's own Git history.
//
// A build started in such a fixture discovers the enclosing checkout, so its
// history passes walk this repository: the result then depends on its commits
// and the tests take minutes. Tests that exercise history create their own
// repository instead.
package testgit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ceilingEnv lists directories git does not enter while searching upward for
// a repository.
const ceilingEnv = "GIT_CEILING_DIRECTORIES"

// fixtureDate pins the commit CommittedCopy makes, so its SHA does not
// depend on when the test runs.
const fixtureDate = "2000-01-01T00:00:00Z"

// IsolateFixtures sets GIT_CEILING_DIRECTORIES to the module root above the
// working directory, so git started in a fixture directory finds no
// repository. Repositories that tests create elsewhere are unaffected. Call it
// from TestMain before m.Run.
func IsolateFixtures() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return os.Setenv(ceilingEnv, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return errors.New("testgit: no go.mod above the working directory")
		}
		dir = parent
	}
}

// UseRepository lifts the ceiling for a test that runs git on this
// repository itself. The test must not run in parallel.
func UseRepository(t *testing.T) {
	t.Helper()
	t.Setenv(ceilingEnv, "")
}

// CommittedCopy copies the fixture directory src into a fresh temporary
// directory, commits it as the only commit of a new repository and returns
// the copy's path. Tests that need a git HEAD for a fixture stored in this
// repository use it instead of this repository's history. The copy is removed
// when the test ends.
func CommittedCopy(t *testing.T, src string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), filepath.Base(src))
	if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
		t.Fatalf("testgit: copy %s: %v", src, err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.name=testgit", "-c", "user.email=testgit@example.com", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "fixture"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+fixtureDate, "GIT_COMMITTER_DATE="+fixtureDate)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("testgit: git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}
