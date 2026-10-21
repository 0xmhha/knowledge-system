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
	"path/filepath"
	"testing"
)

// ceilingEnv lists directories git does not enter while searching upward for
// a repository.
const ceilingEnv = "GIT_CEILING_DIRECTORIES"

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
