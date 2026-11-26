package persist_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// TestSearchWords: text reduced to its words is a query the full-text index
// accepts whatever characters the text holds. The inputs are the 14 that made
// get_context_for_task return a raw full-text syntax error on go-stablenet,
// plus prose and text the substring search handles as it is.
func TestSearchWords(t *testing.T) {
	s := newFixtureStore(t)
	for _, tc := range []struct {
		in, want string
	}{
		{"%", ""},
		{"(", ""},
		{"*", ""},
		{"-", ""},
		{"AND*", ""},
		{"NOT x", "x"},
		{"OR*", ""},
		{`\`, ""},
		{"a'b*", "a b"},
		{`a"b`, "a b"},
		{"foo(bar", "foo bar"},
		{"x AND", "x"},
		{"x:y", "x y"},
		{"{}", ""},
		{`where does (FuncB) "reset" the state: my-pkg*`, "where does FuncB reset the state my pkg"},
		{"한글 검색", "한글 검색"},
	} {
		got := persist.SearchWords(tc.in)
		if got != tc.want {
			t.Errorf("SearchWords(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if got == "" {
			continue
		}
		if _, err := s.Search(got, 5); err != nil {
			t.Errorf("Search(SearchWords(%q)) = %v, want no error", tc.in, err)
		}
	}
}
