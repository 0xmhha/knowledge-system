package ksfixture

import "testing"

func TestAlpha(t *testing.T) {
	if got := Alpha(); got != "alpha" {
		t.Fatalf("Alpha() = %q", got)
	}
}
