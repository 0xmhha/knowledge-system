package persist

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func TestBoundedIncidentEdgesLimitsFiltersDirectionsAndCancellation(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "graph.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	if e = s.Migrate(); e != nil {
		t.Fatal(e)
	}
	nodes := []types.Node{{ID: "a", Name: "A", Type: types.NodeFunction}, {ID: "b", Name: "B", Type: types.NodeFunction}, {ID: "c", Name: "C", Type: types.NodeFunction}}
	if e = s.InsertNodes(nodes); e != nil {
		t.Fatal(e)
	}
	if e = s.InsertEdges([]types.Edge{{Src: "a", Dst: "b", Type: "calls", Confidence: types.ConfExtracted}, {Src: "a", Dst: "c", Type: "calls", Confidence: types.ConfExtracted}, {Src: "c", Dst: "a", Type: "invokes", Confidence: types.ConfExtracted}}); e != nil {
		t.Fatal(e)
	}
	var reader BoundedEdgeReader = s
	out, more, e := reader.IncidentEdges(context.Background(), EdgePageInput{NodeID: "a", Direction: "outgoing", Types: []string{"calls"}, Limit: 1})
	if e != nil || !more || len(out) != 1 || out[0].Dst != "b" {
		t.Fatal(out, more, e)
	}
	out, more, e = reader.IncidentEdges(context.Background(), EdgePageInput{NodeID: "a", Direction: "incoming", Types: []string{"invokes"}, Limit: 2})
	if e != nil || more || len(out) != 1 || out[0].Src != "c" {
		t.Fatal(out, more, e)
	}
	out, more, e = reader.IncidentEdges(context.Background(), EdgePageInput{NodeID: "a", Direction: "both", Limit: 4})
	if e != nil || more || len(out) != 3 {
		t.Fatal(out, more, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e = reader.IncidentEdges(ctx, EdgePageInput{NodeID: "a", Direction: "both", Limit: 1}); e == nil {
		t.Fatal("canceled SQL context ignored")
	}
}
func TestBoundedEdgeSQLParametersCapsAndDialect(t *testing.T) {
	for _, postgres := range []bool{false, true} {
		q, args, e := edgePageSQL(EdgePageInput{NodeID: "' or true--", Direction: "both", Types: []string{"calls", "invokes"}, Limit: 2}, postgres)
		if e != nil || strings.Contains(q, "' or true") || len(args) != 5 || args[4] != 3 {
			t.Fatal(q, args, e)
		}
		if postgres && !strings.Contains(q, "LIMIT $5") {
			t.Fatal(q)
		}
	}
	for _, input := range []EdgePageInput{{NodeID: "a", Direction: "outgoing", Limit: 0}, {NodeID: "a", Direction: "outgoing", Limit: 257}, {NodeID: "a", Direction: "drop table", Limit: 2}, {NodeID: "a", Direction: "both", Limit: 2, Types: []string{""}}} {
		if _, _, e := edgePageSQL(input, false); e == nil {
			t.Fatal("invalid bound/direction accepted")
		}
	}
}
