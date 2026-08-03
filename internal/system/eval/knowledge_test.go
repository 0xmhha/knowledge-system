package eval

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestMissingKnowledgeScopes(t *testing.T) {
	t.Parallel()
	got := []contract.KnowledgeChunk{
		{Scope: "internal/system/composer", Kind: "convention", Text: "x"},
		{Scope: "internal/vector/chunk", Kind: "convention", Text: "y"},
	}
	cases := map[string]struct {
		expected []string
		want     int
	}{
		"all present":   {[]string{"internal/system/composer"}, 0},
		"one missing":   {[]string{"internal/system/composer", "internal/system/mcp"}, 1},
		"all missing":   {[]string{"a", "b"}, 2},
		"none expected": {nil, 0},
	}
	for name, tc := range cases {
		if n := len(missingKnowledgeScopes(tc.expected, got)); n != tc.want {
			t.Errorf("%s: missing = %d, want %d", name, n, tc.want)
		}
	}
	// An empty knowledge section must report every expectation as missing —
	// that is the break this guard exists to catch.
	if n := len(missingKnowledgeScopes([]string{"a", "b"}, nil)); n != 2 {
		t.Errorf("empty pack: missing = %d, want 2", n)
	}
}
