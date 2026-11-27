package persist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// graphRoundTripEnv names a built graph directory to check every printed
// name of; the test is skipped without it (the graph is large and not part
// of the repository).
const graphRoundTripEnv = "CKG_ROUNDTRIP_GRAPH"

// TestPrintedNameRoundTrip_Graph: on a real graph, the name printed for
// every node resolves back to exactly that node (a promoted copy of a
// method excepted, as in TestNameOfRoundTrip), Channel nodes included,
// which are named by their line (and column when several are made on it).
func TestPrintedNameRoundTrip_Graph(t *testing.T) {
	dir := os.Getenv(graphRoundTripEnv)
	if dir == "" {
		t.Skipf("set %s to a graph directory", graphRoundTripEnv)
	}
	s, err := persist.OpenReadOnly(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	all, err := s.AllNodes()
	if err != nil {
		t.Fatal(err)
	}
	bad, checked := 0, 0
	for _, n := range all {
		if _, isCopy := persist.PromotedFrom(n); isCopy {
			continue
		}
		name := persist.PrintedName(s, n)
		res, err := persist.ResolveName(s, name, "")
		if err != nil {
			t.Fatalf("ResolveName(%s): %v", name, err)
		}
		checked++
		if len(res.Nodes) != 1 || res.Nodes[0].ID != n.ID {
			bad++
			if bad <= 10 {
				t.Errorf("%s %s printed as %q resolves to %d nodes", n.Type, n.ID, name, len(res.Nodes))
			}
		}
	}
	t.Logf("%d of %d printed names resolve to another node set", bad, checked)
}
