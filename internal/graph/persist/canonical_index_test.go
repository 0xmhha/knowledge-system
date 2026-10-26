package persist

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Seed resolution looks canonical ids up on every MCP call; without an index
// each lookup scanned the whole nodes table (18 ms on this repository's
// graph, twice per resolution).
func TestFindByCanonicalIDUsesAnIndex(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	rows, err := st.db.Query(`EXPLAIN QUERY PLAN SELECT id FROM nodes WHERE canonical_id = ? LIMIT 1`, "x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var plan []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if p := strings.Join(plan, "; "); !strings.Contains(p, "idx_nodes_canonical_id") {
		t.Fatalf("canonical lookup plan = %q", p)
	}
}

// The suffix lookup runs on every MCP seed resolution. Written as one
// "qualified_name = ? OR simple_name = ?" predicate, SQLite 3.53 scanned the
// whole nodes table (30 ms on this repository's graph); it must use the
// qualified_name and simple_name indexes.
func TestFindSymbolSuffixUsesIndexes(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	// "T.Foo" adds the dotted-suffix arm; it must stay indexed too.
	for _, name := range []string{"Foo", "T.Foo"} {
		q, args := findSymbolQuery(name, false, FindSymbolOptions{Language: "go", Kinds: []types.NodeType{types.NodeFunction}})
		rows, err := st.db.Query("EXPLAIN QUERY PLAN "+q, args...)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, notused int
			var detail string
			if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		_ = rows.Close()
		p := strings.Join(plan, "; ")
		for _, want := range []string{"idx_nodes_qname", "idx_nodes_simple_name"} {
			if !strings.Contains(p, want) {
				t.Errorf("%s: suffix lookup plan %q does not use %s", name, p, want)
			}
		}
		if strings.Contains(p, "SCAN nodes") {
			t.Errorf("%s: suffix lookup scans nodes: %q", name, p)
		}
	}
}
