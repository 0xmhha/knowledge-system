package markdown

import "testing"

func TestParseHeadingPathFollowsHierarchy(t *testing.T) {
	src := []byte("# System\nIntro\n## Search\nBody\n### Filters\nDetail\n## Index\nFinal\n")
	spans, err := New().Parse("README.md", src)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"System"}, {"System", "Search"}, {"System", "Search", "Filters"}, {"System", "Index"}}
	if len(spans) != len(want) {
		t.Fatalf("got %d spans, want %d", len(spans), len(want))
	}
	for i, path := range want {
		if len(spans[i].HeadingPath) != len(path) {
			t.Fatalf("span %d heading path=%v, want %v", i, spans[i].HeadingPath, path)
		}
		for j := range path {
			if spans[i].HeadingPath[j] != path[j] {
				t.Fatalf("span %d heading path=%v, want %v", i, spans[i].HeadingPath, path)
			}
		}
	}
}
