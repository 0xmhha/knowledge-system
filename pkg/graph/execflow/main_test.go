package execflow

import (
	"os"
	"testing"
)

// fixtureRoot is the directory the fixture graph is built in, removed when
// the tests end.
var fixtureRoot string

// tempDir makes the directory of the fixture graph.
func tempDir() (string, error) {
	dir, err := os.MkdirTemp("", "execflow-fixture-")
	if err != nil {
		return "", err
	}
	fixtureRoot = dir
	return dir, nil
}

func TestMain(m *testing.M) {
	code := m.Run()
	if fixtureRoot != "" {
		_ = os.RemoveAll(fixtureRoot)
	}
	os.Exit(code)
}
