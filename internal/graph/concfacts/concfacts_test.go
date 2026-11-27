package concfacts

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestEdgesOfFacts: each fact is stored as the edge type, dispatch kind and
// confidence of its kind and certainty, and Counted folds the uses of one
// edge into its count in the order of first use.
func TestEdgesOfFacts(t *testing.T) {
	inferred := Approx(ReasonAliasing)
	fs := Facts{
		LockOp{Function: "f", Mutex: "m", Acquire: true, Line: 3, File: "a.go", Certainty: Proof},
		UnderLock{Field: "x", Mutex: "m", Propagated: true, File: "a.go", Certainty: inferred},
		ChanOp{Actor: "f", Origin: "c", OriginKind: "make", Dir: Recv, Line: 4, File: "a.go", Certainty: inferred},
		ChanOp{Actor: "f", Origin: "c", OriginKind: "make", Dir: Recv, Line: 4, File: "a.go", Certainty: inferred},
		Trigger{Sender: "f", Receive: "r", Channel: "chan@a.go:9", Line: 5, File: "a.go", Certainty: Approx(ReasonInterleaving)},
	}
	e := fs.Edges()
	if len(e) != 5 || e[0].Type != types.EdgeAcquiresLock || e[0].Confidence != types.ConfExtracted ||
		e[1].DispatchKind != PropagatedKind || e[1].Confidence != types.ConfInferred ||
		e[2].Type != types.EdgeRecvsFrom || e[4].Type != types.EdgeTriggers || e[4].DispatchKind != "chan@a.go:9" {
		t.Fatalf("edges %+v", e)
	}
	c := fs.Counted()
	if len(c) != 4 || c[2].Count != 2 {
		t.Errorf("counted %+v", c)
	}
}
