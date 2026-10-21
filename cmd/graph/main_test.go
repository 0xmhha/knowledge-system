package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/testgit"
)

// Fixtures live inside this repository; without the ceiling every cold build
// in these tests walks the repository's own history.
func TestMain(m *testing.M) {
	if err := testgit.IsolateFixtures(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
