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

// TestEdgesOfExecutionFacts: a close, the locks held at a site, an operation
// on a sync primitive and a function run later are stored as closes,
// runs_under_lock, operates_on and runs_func edges.
func TestEdgesOfExecutionFacts(t *testing.T) {
	fs := Facts{
		ChanOp{Actor: "f", Origin: "c", OriginKind: "field", Dir: Close, Line: 4, File: "a.go", Certainty: Approx(ReasonAliasing)},
		SiteLock{Site: "s", Mutex: "m", Propagated: true, Line: 5, File: "a.go", Certainty: Approx(ReasonBranches)},
		SyncUse{Site: "s", Primitive: "w", Op: types.SyncWaitGroupAdd, Line: 6, File: "a.go", Certainty: Proof},
		RunsLater{Site: "s", Func: "g", Op: types.SyncOnceDo, Line: 7, File: "a.go", Certainty: Proof},
	}
	e := fs.Edges()
	want := []struct {
		typ  types.EdgeType
		kind string
		conf types.Confidence
	}{
		{types.EdgeCloses, "field", types.ConfInferred},
		{types.EdgeRunsUnderLock, PropagatedKind, types.ConfInferred},
		{types.EdgeOperatesOn, "wg_add", types.ConfExtracted},
		{types.EdgeRunsFunc, "once_do", types.ConfExtracted},
	}
	if len(e) != len(want) {
		t.Fatalf("edges %+v", e)
	}
	for i, w := range want {
		if e[i].Type != w.typ || e[i].DispatchKind != w.kind || e[i].Confidence != w.conf || e[i].Line == 0 {
			t.Errorf("edge %d = %+v, want %v %q %v", i, e[i], w.typ, w.kind, w.conf)
		}
	}
}
