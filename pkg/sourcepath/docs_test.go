package sourcepath

import "testing"

func TestNamespacesRejectAmbiguousAndEscapingPaths(t *testing.T) {
	for _, file := range []string{"@cks/docs/01/a.md", "@cks/docs/-1/a.md", "@cks/docs/0/../secret", "@cks/docs/0//a", "@cks/docs/0/a/../../b", "@cks/docs/0/a\\b", "@cks/unknown/a", "@cks/docs/999999999999999999999/a"} {
		_, _, reserved, valid := Parse(file)
		if !reserved || valid {
			t.Fatalf("accepted malformed reserved path %q", file)
		}
	}
	index, relative, reserved, valid := Parse(Docs(2, "entries/a.md"))
	if !reserved || !valid || index != 2 || relative != "entries/a.md" || Origin(index) != "docs:corpus.2" {
		t.Fatal("namespace round trip failed")
	}
	_, relative, reserved, valid = Parse("entries/a.md")
	if reserved || !valid || relative != "entries/a.md" {
		t.Fatal("repository path changed")
	}
}
