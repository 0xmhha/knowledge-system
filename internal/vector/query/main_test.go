package query

import (
	"fmt"
	"os"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/testgit"
)

// The sample fixture lives inside this repository; without the ceiling a
// build of it records this repository's HEAD. Tests that need a HEAD use
// testgit.CommittedCopy.
func TestMain(m *testing.M) {
	if err := testgit.IsolateFixtures(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
