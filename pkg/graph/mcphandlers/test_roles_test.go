package mcphandlers

import (
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

// rolesFixture is a graph whose build scope gives two files the role their
// path does not: p/helper.go is test code, p/check_test.go production code.
func rolesFixture(t *testing.T) persist.Store {
	t.Helper()
	out := t.TempDir()
	if _, err := buildpipe.Run(buildpipe.Options{
		SrcRoot: "testdata/test_roles", OutDir: out, Languages: []string{"go"}, CKGVersion: "test", NoCache: true,
		FileRoles: buildpipe.FileRoles{
			"p/a.go":          buildpipe.FileRoleProduction,
			"p/helper.go":     buildpipe.FileRoleTest,
			"p/check_test.go": buildpipe.FileRoleProduction,
		},
	}); err != nil {
		t.Fatal(err)
	}
	s, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestTestCodeFollowsTheGraphRecord: every graph tool takes test code from
// the role the build recorded on the nodes, not from the path; a file the
// graph does not hold is judged by the path rule. Before, search_code,
// search_test_code and find_references used the path (_test.go, testdata).
func TestTestCodeFollowsTheGraphRecord(t *testing.T) {
	s := rolesFixture(t)
	isTest, err := store.TestFileRule(s)
	if err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]bool{"p/a.go": false, "p/helper.go": true, "p/check_test.go": false, "q/other_test.go": true, "q/other.go": false} {
		if got := isTest(file); got != want {
			t.Errorf("TestFileRule(%q) = %v, want %v", file, got, want)
		}
	}
	prod := callTool(t, s, "search_code", map[string]any{"pattern": `t\.v`})
	tests := callTool(t, s, "search_test_code", map[string]any{"pattern": `t\.v`})
	for _, c := range []struct {
		out        map[string]any
		tool, name string
		want       bool
	}{
		{prod, "search_code", "p.checkUses", true},
		{prod, "search_code", "p.HelperUses", false},
		{tests, "search_test_code", "p.HelperUses", true},
		{tests, "search_test_code", "p.checkUses", false},
	} {
		if got := hitByName(c.out, c.name) != nil; got != c.want {
			t.Errorf("%s found %s: %v, want %v (%v)", c.tool, c.name, got, c.want, hitNames(c.out))
		}
	}
	refs := callTool(t, s, "find_references", map[string]any{"qname": "p.T.v"})
	for _, r := range asSlice(refs["users"]) {
		row := r.(map[string]any)
		want := row["qname"] == "p.HelperUses"
		if got := row["test"] == true; got != want {
			t.Errorf("find_references: %v test = %v, want %v", row["qname"], got, want)
		}
	}
	if n := len(asSlice(refs["users"])); n != 3 {
		t.Errorf("find_references users = %d, want 3: %v", n, refs)
	}
}
