package buildpipe

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

const optionalInputsPolicy = `
policies:
  - id: "fixture.greet-policy"
    name: "Greet API contract"
    category: "fixture"
    description: "Synthetic policy for the optional-inputs test."
    governs:
      - "a.Greet"
`

// policyRows returns the Policy nodes and governed_by edges of the graph in
// out, as comparable strings.
func policyRows(t *testing.T, out string) []string {
	t.Helper()
	st, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatalf("open graph.db: %v", err)
	}
	defer func() { _ = st.Close() }()
	nodes, err := st.AllNodes()
	if err != nil {
		t.Fatalf("AllNodes: %v", err)
	}
	var rows []string
	for _, n := range nodes {
		if n.Type == types.NodePolicy {
			rows = append(rows, strings.Join([]string{"node", n.ID, n.QualifiedName, n.FilePath}, "|"))
		}
	}
	edges, err := st.QueryEdgesByType(string(types.EdgeGovernedBy))
	if err != nil {
		t.Fatalf("QueryEdgesByType: %v", err)
	}
	for _, e := range edges {
		rows = append(rows, strings.Join([]string{"edge", e.Src, e.Dst, e.FilePath}, "|"))
	}
	sort.Strings(rows)
	return rows
}

// TestColdBuild_OptionalInputsRecorded: the manifest records each optional
// input the build was given (the path as given and its content hash), and
// the Policy rows carry a path that does not depend on where the file is:
// two builds with the file at two places outside the source root write the
// same rows (base name and hash, no machine path); a file inside the source
// root is named relative to it.
func TestColdBuild_OptionalInputsRecorded(t *testing.T) {
	const fixture = "../parse/golang/testdata/resolve"
	build := func(policy string) (persist.Manifest, []string) {
		out := t.TempDir()
		m, err := Run(Options{SrcRoot: fixture, OutDir: out, Languages: []string{"auto"}, CKGVersion: "test", PolicyFile: policy})
		if err != nil {
			t.Fatalf("build with %s: %v", policy, err)
		}
		return m, policyRows(t, out)
	}
	var paths []string
	for range 2 {
		p := filepath.Join(t.TempDir(), "nested", "graph.yaml")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(optionalInputsPolicy), 0o644); err != nil {
			t.Fatalf("write policy: %v", err)
		}
		paths = append(paths, p)
	}
	m1, rows1 := build(paths[0])
	m2, rows2 := build(paths[1])
	if len(rows1) == 0 {
		t.Fatal("no policy rows")
	}
	if !reflect.DeepEqual(rows1, rows2) {
		t.Errorf("policy rows differ with the file at two places:\n%v\n%v", rows1, rows2)
	}
	for _, r := range rows1 {
		if strings.Contains(r, string(filepath.Separator)+"nested") || !strings.Contains(r, "graph.yaml@") {
			t.Errorf("row %q carries a machine path, want the base name and hash", r)
		}
	}
	if m1.EnrichDigest != m2.EnrichDigest {
		t.Errorf("enrich digest moved with the file's place: %s vs %s", m1.EnrichDigest, m2.EnrichDigest)
	}
	tests := []struct {
		name string
		m    persist.Manifest
		path string
	}{
		{"first place", m1, paths[0]},
		{"second place", m2, paths[1]},
	}
	for _, tc := range tests {
		if len(tc.m.OptionalInputs) != 1 {
			t.Fatalf("%s: optional inputs = %v, want one", tc.name, tc.m.OptionalInputs)
		}
		in := tc.m.OptionalInputs[0]
		if in.Kind != inputPolicyFile || in.Path != tc.path || len(in.SHA256) != 64 || !strings.HasPrefix(in.RowPath, "graph.yaml@") {
			t.Errorf("%s: optional input = %+v", tc.name, in)
		}
	}
	if m, _ := build(""); len(m.OptionalInputs) != 0 {
		t.Errorf("a build without optional inputs records %v", m.OptionalInputs)
	}
	if got := rowPathOf(filepath.Join(fixture, "policies", "graph.yaml"), fixture, strings.Repeat("0", 64)); got != "policies/graph.yaml" {
		t.Errorf("a file inside the source root: row path %q, want policies/graph.yaml", got)
	}
}
