package stage1

import "testing"

// TestMorphCandidates pins the noun→verb derivations that bridge prompt
// vocabulary ("tool registration") to declared symbols ("Register") —
// neither ckg BM25 (no stemming) nor FindSymbol (exact match) does this.
func TestMorphCandidates(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"registration":   {"Register", "register"},
		"initialization": {"Initialize", "initialize"},
		"verification":   {"Verify", "verify"},
		"configuration":  {"Configure", "configure"},
		"validation":     {"Validate", "validate"},
		"administration": {"Administer", "administer"},
	}
	for in, want := range cases {
		got := morphCandidates([]string{in})
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("morphCandidates(%q) = %v, want %v", in, got, want)
		}
	}
	// Non-derivational tokens produce nothing.
	if got := morphCandidates([]string{"server", "health", "MCP"}); len(got) != 0 {
		t.Errorf("non-suffix tokens produced %v, want none", got)
	}
	// Guard against over-short stems ("nation" would become "nate").
	if got := morphCandidates([]string{"nation"}); len(got) != 0 {
		t.Errorf("short token produced %v, want none", got)
	}
}
