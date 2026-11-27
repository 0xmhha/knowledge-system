package validate

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func validNode(id, qname string, t types.NodeType) types.Node {
	return types.Node{
		ID: id, Type: t, Name: qname, QualifiedName: qname,
		FilePath: "f.go", StartLine: 1, EndLine: 1,
		Language: "go", Confidence: types.ConfExtracted,
	}
}

func TestSchemaValidator_HappyPath(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("a", "pkg.A", types.NodeFunction),
			validNode("b", "pkg.B", types.NodeFunction),
		},
		Edges: []types.Edge{
			{Src: "a", Dst: "b", Type: types.EdgeCalls, Confidence: types.ConfExtracted, Line: 10, FilePath: "f.go"},
		},
	}
	v := NewSchemaValidator()
	r, err := v.Validate(context.Background(), g, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.HasErrors() {
		t.Errorf("happy path produced errors: %+v", r.Issues)
	}
}

func TestSchemaValidator_EmptyFields(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			{ID: "a", Type: types.NodeFunction, Confidence: types.ConfExtracted}, // missing name+qname+filepath
		},
	}
	v := NewSchemaValidator()
	r, _ := v.Validate(context.Background(), g, nil)
	codes := map[string]bool{}
	for _, iss := range r.Issues {
		codes[iss.Code] = true
	}
	for _, want := range []string{"empty-name", "empty-qname", "empty-file-path"} {
		if !codes[want] {
			t.Errorf("missing expected issue code %q in %v", want, codes)
		}
	}
}

func TestSchemaValidator_DanglingEdge(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{validNode("a", "pkg.A", types.NodeFunction)},
		Edges: []types.Edge{
			{Src: "a", Dst: "ghost", Type: types.EdgeCalls, Confidence: types.ConfExtracted},
		},
	}
	v := NewSchemaValidator()
	r, _ := v.Validate(context.Background(), g, nil)
	found := false
	for _, iss := range r.Issues {
		if iss.Code == "dangling-dst" && strings.Contains(iss.EdgeKey, "ghost") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected dangling-dst issue, got %+v", r.Issues)
	}
}

// TestSchemaValidator_ContextPathBadShape locks down the context-path invariant on
// timeout_path / cancellation_path edges: each must be a self-loop on a
// callable kind (Function/Method). Self-loops on non-callable types
// (e.g. Package) and non-self-loops between callables both fail the shape
// rule. Mirrors TestSchemaValidator_ImplementsBadDst in style.
func TestSchemaValidator_ContextPathBadShape(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("a", "pkg.A", types.NodeFunction),
			validNode("b", "pkg.B", types.NodeFunction),
			validNode("m", "pkg.T.M", types.NodeMethod),
			validNode("p", "pkg", types.NodePackage),
		},
		Edges: []types.Edge{
			// timeout_path: good self-loop on Function.
			{Src: "a", Dst: "a", Type: types.EdgeTimeoutPath, Confidence: types.ConfExtracted, Line: 1, FilePath: "f.go"},
			// timeout_path: bad — not a self-loop (Function → Function).
			{Src: "a", Dst: "b", Type: types.EdgeTimeoutPath, Confidence: types.ConfExtracted, Line: 2, FilePath: "f.go"},
			// timeout_path: bad — self-loop on a non-callable Package node.
			{Src: "p", Dst: "p", Type: types.EdgeTimeoutPath, Confidence: types.ConfExtracted, Line: 3, FilePath: "f.go"},
			// cancellation_path: good self-loop on Method.
			{Src: "m", Dst: "m", Type: types.EdgeCancellationPath, Confidence: types.ConfExtracted, Line: 4, FilePath: "f.go"},
			// cancellation_path: bad — cross-node between callables.
			{Src: "a", Dst: "b", Type: types.EdgeCancellationPath, Confidence: types.ConfExtracted, Line: 5, FilePath: "f.go"},
			// cancellation_path: bad — self-loop on a Package node.
			{Src: "p", Dst: "p", Type: types.EdgeCancellationPath, Confidence: types.ConfExtracted, Line: 6, FilePath: "f.go"},
		},
	}
	v := NewSchemaValidator()
	r, err := v.Validate(context.Background(), g, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	timeoutBadKeys := map[string]bool{}
	cancelBadKeys := map[string]bool{}
	for _, iss := range r.Issues {
		switch iss.Code {
		case "timeout-path-bad-shape":
			timeoutBadKeys[iss.EdgeKey] = true
		case "cancellation-path-bad-shape":
			cancelBadKeys[iss.EdgeKey] = true
		}
	}
	wantTimeout := map[string]bool{
		"timeout_path:a:b:2": true,
		"timeout_path:p:p:3": true,
	}
	wantCancel := map[string]bool{
		"cancellation_path:a:b:5": true,
		"cancellation_path:p:p:6": true,
	}
	if len(timeoutBadKeys) != 2 {
		t.Errorf("expected 2 timeout-path-bad-shape issues, got %d: %v",
			len(timeoutBadKeys), timeoutBadKeys)
	}
	for k := range wantTimeout {
		if !timeoutBadKeys[k] {
			t.Errorf("missing timeout-path-bad-shape EdgeKey %q in %v", k, timeoutBadKeys)
		}
	}
	if len(cancelBadKeys) != 2 {
		t.Errorf("expected 2 cancellation-path-bad-shape issues, got %d: %v",
			len(cancelBadKeys), cancelBadKeys)
	}
	for k := range wantCancel {
		if !cancelBadKeys[k] {
			t.Errorf("missing cancellation-path-bad-shape EdgeKey %q in %v", k, cancelBadKeys)
		}
	}
	// The good self-loops must NOT have produced bad-shape issues.
	for k := range timeoutBadKeys {
		if k == "timeout_path:a:a:1" {
			t.Errorf("good Function self-loop wrongly flagged timeout-path-bad-shape: %q", k)
		}
	}
	for k := range cancelBadKeys {
		if k == "cancellation_path:m:m:4" {
			t.Errorf("good Method self-loop wrongly flagged cancellation-path-bad-shape: %q", k)
		}
	}
	// The other invariant checks (Implements/ListensOn/Calls) must remain
	// silent for this fixture — no edges of those types are present.
	for _, iss := range r.Issues {
		switch iss.Code {
		case "implements-bad-dst",
			"listens-on-bad-src", "listens-on-bad-dst",
			"calls-bad-src", "calls-bad-dst":
			t.Errorf("unexpected unrelated invariant issue %q: %+v", iss.Code, iss)
		}
	}
}

// A call may end at an External node (schema 1.36) and may not start at one.
func TestSchemaValidator_ExternalCallEnds(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("f", "pkg.F", types.NodeFunction),
			validNode("x", "time.AfterFunc", types.NodeExternal),
			validNode("s", "pkg.S", types.NodeStruct),
		},
		Edges: []types.Edge{
			{Src: "f", Dst: "x", Type: types.EdgeCalls, Line: 1, Confidence: types.ConfExtracted},
			{Src: "f", Dst: "x", Type: types.EdgeInvokes, Line: 2, Confidence: types.ConfExtracted},
			{Src: "x", Dst: "f", Type: types.EdgeCalls, Line: 3, Confidence: types.ConfExtracted},
			{Src: "f", Dst: "s", Type: types.EdgeCalls, Line: 4, Confidence: types.ConfExtracted},
		},
	}
	r, err := NewSchemaValidator().Validate(context.Background(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, iss := range r.Issues {
		got[iss.Code+" "+iss.EdgeKey]++
	}
	for _, key := range []string{"calls-bad-src calls:x:f:3", "calls-bad-dst calls:f:s:4"} {
		if got[key] != 1 {
			t.Errorf("issue %q reported %d times, want once; got %v", key, got[key], got)
		}
		delete(got, key)
	}
	if len(got) != 0 {
		t.Errorf("unexpected issues %v", got)
	}
}

// A package-level variable may start a call (its initialiser makes it, schema
// 1.38) and may not end one.
func TestSchemaValidator_VariableCallEnds(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("f", "pkg.F", types.NodeFunction),
			validNode("v", "pkg.Default", types.NodeVariable),
		},
		Edges: []types.Edge{
			{Src: "v", Dst: "f", Type: types.EdgeCalls, Line: 1, Confidence: types.ConfExtracted},
			{Src: "v", Dst: "f", Type: types.EdgeInvokes, Line: 2, Confidence: types.ConfExtracted},
			{Src: "f", Dst: "v", Type: types.EdgeCalls, Line: 3, Confidence: types.ConfExtracted},
			// The marker of a closure the initialiser calls names no callee.
			{Src: "v", Dst: "v", Type: types.EdgeInvokes, Line: 4, Confidence: types.ConfExtracted, DispatchKind: "closure"},
		},
	}
	r, err := NewSchemaValidator().Validate(context.Background(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, iss := range r.Issues {
		got[iss.Code+" "+iss.EdgeKey]++
	}
	const want = "calls-bad-dst calls:f:v:3"
	if got[want] != 1 || len(got) != 1 {
		t.Errorf("issues %v, want only %q once", got, want)
	}
}

func TestSchemaValidator_ImplementsBadDst(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("s", "pkg.S", types.NodeStruct),
			validNode("f", "pkg.F", types.NodeFunction),
		},
		Edges: []types.Edge{
			// implements onto a Function (wrong) instead of Interface
			{Src: "s", Dst: "f", Type: types.EdgeImplements, Confidence: types.ConfExtracted},
		},
	}
	v := NewSchemaValidator()
	r, _ := v.Validate(context.Background(), g, nil)
	found := false
	for _, iss := range r.Issues {
		if iss.Code == "implements-bad-dst" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected implements-bad-dst issue, got %+v", r.Issues)
	}
}

// A Solidity modifier body makes calls, so a Modifier may start a calls edge
// and may not end one. A reverts_with edge starts at a function or modifier
// and ends at an Error.
func TestSchemaValidator_SolidityEndpoints(t *testing.T) {
	nodes := []types.Node{
		validNode("f", "C.f", types.NodeFunction),
		validNode("m", "C.onlyOwner", types.NodeModifier),
		validNode("e", "C.NotOwner", types.NodeError),
		validNode("v", "C.Paused", types.NodeEvent),
		validNode("s", "C.total", types.NodeField),
		validNode("k", "C.LIMIT", types.NodeConstant),
	}
	cases := []struct {
		name string
		edge types.Edge
		want string
	}{
		{"modifier calls a function", types.Edge{Src: "m", Dst: "f", Type: types.EdgeCalls, Line: 1}, ""},
		{"state variable initialiser calls a function", types.Edge{Src: "s", Dst: "f", Type: types.EdgeCalls, Line: 7}, ""},
		{"constant initialiser calls a function", types.Edge{Src: "k", Dst: "f", Type: types.EdgeCalls, Line: 8}, ""},
		{"function calls a state variable", types.Edge{Src: "f", Dst: "s", Type: types.EdgeCalls, Line: 9}, "calls-bad-dst"},
		{"function calls a modifier", types.Edge{Src: "f", Dst: "m", Type: types.EdgeCalls, Line: 2}, "calls-bad-dst"},
		{"function reverts with an error", types.Edge{Src: "f", Dst: "e", Type: types.EdgeRevertsWith, Line: 3}, ""},
		{"modifier reverts with an error", types.Edge{Src: "m", Dst: "e", Type: types.EdgeRevertsWith, Line: 4}, ""},
		{"reverts with an event", types.Edge{Src: "f", Dst: "v", Type: types.EdgeRevertsWith, Line: 5}, "reverts-with-bad-dst"},
		{"an error reverts", types.Edge{Src: "e", Dst: "e", Type: types.EdgeRevertsWith, Line: 6}, "reverts-with-bad-src"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.edge.Confidence = types.ConfExtracted
			g := &graph.Graph{Nodes: nodes, Edges: []types.Edge{tc.edge}}
			r, err := NewSchemaValidator().Validate(context.Background(), g, nil)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, iss := range r.Issues {
				got = append(got, iss.Code)
			}
			switch {
			case tc.want == "" && len(got) != 0:
				t.Errorf("issues %v, want none", got)
			case tc.want != "" && (len(got) != 1 || got[0] != tc.want):
				t.Errorf("issues %v, want only %q", got, tc.want)
			}
		})
	}
}

// An Error node is declared in a file and must name it.
func TestSchemaValidator_ErrorNeedsFilePath(t *testing.T) {
	n := validNode("e", "C.NotOwner", types.NodeError)
	n.FilePath = ""
	r, err := NewSchemaValidator().Validate(context.Background(), &graph.Graph{Nodes: []types.Node{n}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Issues) != 1 || r.Issues[0].Code != "empty-file-path" {
		t.Errorf("issues %v, want one empty-file-path", r.Issues)
	}
}

// The execution-order facts (schema 1.44) have fixed shapes: closes from a
// closer to a channel, runs_under_lock from a site to a Mutex, operates_on
// from a site to a primitive's declaration with a known operation, runs_func
// from a site to a declared function.
// TestSchemaValidator_JumpShapes: jumps_to goes from a BranchStmt (a
// ReturnStmt for return) to a target of its kind; a FuncLit may start
// literal_passed_to.
func TestSchemaValidator_JumpShapes(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("f", "pkg.F", types.NodeFunction),
			validNode("b", "pkg.F#BranchStmt@1", types.NodeBranchStmt),
			validNode("r", "pkg.F#ReturnStmt@2", types.NodeReturnStmt),
			validNode("l", "pkg.F#LoopStmt@3", types.NodeLoopStmt),
			validNode("c", "pkg.F#CaseClause@4", types.NodeCaseClause),
			validNode("s", "pkg.F#SwitchStmt@5", types.NodeSwitchStmt),
			validNode("x", "pkg.F#FuncLit@6", types.NodeFuncLit),
		},
		Edges: []types.Edge{
			{Src: "b", Dst: "l", Type: types.EdgeJumpsTo, Line: 1, Confidence: types.ConfExtracted, DispatchKind: types.JumpContinue},
			{Src: "b", Dst: "s", Type: types.EdgeJumpsTo, Line: 1, Confidence: types.ConfExtracted, DispatchKind: types.JumpBreak},
			{Src: "b", Dst: "c", Type: types.EdgeJumpsTo, Line: 1, Confidence: types.ConfExtracted, DispatchKind: types.JumpFallthrough},
			{Src: "r", Dst: "x", Type: types.EdgeJumpsTo, Line: 2, Confidence: types.ConfExtracted, DispatchKind: types.JumpReturn},
			{Src: "x", Dst: "f", Type: types.EdgeLiteralPassedTo, Line: 6, Confidence: types.ConfExtracted, DispatchKind: types.LiteralUseGo},
			// Bad: a continue cannot go to a switch; a return starts at a ReturnStmt.
			{Src: "b", Dst: "s", Type: types.EdgeJumpsTo, Line: 7, Confidence: types.ConfExtracted, DispatchKind: types.JumpContinue},
			{Src: "b", Dst: "f", Type: types.EdgeJumpsTo, Line: 8, Confidence: types.ConfExtracted, DispatchKind: types.JumpReturn},
		},
	}
	r, err := NewSchemaValidator().Validate(context.Background(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, iss := range r.Issues {
		got[iss.Code+" "+iss.EdgeKey]++
	}
	want := map[string]int{"jumps-to-bad-shape jumps_to:b:s:7": 1, "jumps-to-bad-shape jumps_to:b:f:8": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("issues %v, want %v", got, want)
	}
}

func TestSchemaValidator_ExecutionOrderShapes(t *testing.T) {
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("f", "pkg.F", types.NodeFunction),
			validNode("c", "pkg.S.quit", types.NodeField),
			validNode("s", "pkg.F#CallSite@1-2", types.NodeCallSite),
			validNode("m", "pkg.S.mu#mutex", types.NodeMutex),
			validNode("w", "pkg.F.wg", types.NodeLocalVariable),
			validNode("x", "pkg.F#CallSite@3-4", types.NodeCallSite),
			validNode("e", "time.After", types.NodeExternal),
		},
		Edges: []types.Edge{
			{Src: "f", Dst: "c", Type: types.EdgeCloses, Line: 1, Confidence: types.ConfInferred, DispatchKind: "field"},
			{Src: "s", Dst: "m", Type: types.EdgeRunsUnderLock, Line: 1, Confidence: types.ConfInferred},
			{Src: "s", Dst: "w", Type: types.EdgeOperatesOn, Line: 1, Confidence: types.ConfExtracted, DispatchKind: "wg_add"},
			{Src: "s", Dst: "x", Type: types.EdgeOperatesOn, Line: 1, Confidence: types.ConfExtracted, DispatchKind: "timer_recv"},
			{Src: "s", Dst: "f", Type: types.EdgeRunsFunc, Line: 1, Confidence: types.ConfExtracted, DispatchKind: "once_do"},
			// Bad: a site does not close; a Field is not a Mutex; an unknown
			// operation; a function is not a primitive; a field is not run.
			{Src: "s", Dst: "c", Type: types.EdgeCloses, Line: 2, Confidence: types.ConfInferred},
			{Src: "s", Dst: "c", Type: types.EdgeRunsUnderLock, Line: 2, Confidence: types.ConfInferred},
			{Src: "s", Dst: "w", Type: types.EdgeOperatesOn, Line: 2, Confidence: types.ConfExtracted, DispatchKind: "spin"},
			{Src: "s", Dst: "f", Type: types.EdgeOperatesOn, Line: 3, Confidence: types.ConfExtracted, DispatchKind: "wg_add"},
			{Src: "s", Dst: "e", Type: types.EdgeOperatesOn, Line: 4, Confidence: types.ConfExtracted, DispatchKind: "timer_recv"},
			{Src: "s", Dst: "c", Type: types.EdgeRunsFunc, Line: 2, Confidence: types.ConfExtracted, DispatchKind: "once_do"},
		},
	}
	r, err := NewSchemaValidator().Validate(context.Background(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, iss := range r.Issues {
		got[iss.Code+" "+iss.EdgeKey]++
	}
	for _, key := range []string{"closes-bad-shape closes:s:c:2", "runs_under_lock-bad-shape runs_under_lock:s:c:2",
		"operates_on-bad-shape operates_on:s:w:2", "operates_on-bad-shape operates_on:s:f:3", "operates_on-bad-shape operates_on:s:e:4", "runs_func-bad-shape runs_func:s:c:2"} {
		if got[key] != 1 {
			t.Errorf("issue %q reported %d times, want once; got %v", key, got[key], got)
		}
		delete(got, key)
	}
	if len(got) != 0 {
		t.Errorf("unexpected issues %v", got)
	}
}

// The combined graph (schema 1.46) holds the shapes of every line of work at
// once: a call ending at an External stub, the line history (changed_in
// from a declaration with its copy marks, blame with a line count, a hunk
// that modifies the declaration) and the execution-flow facts (a statement
// node with jumps_to, a FuncLit passed to its callee, a CallSite whose sync
// operation is unknown). None of them is an issue.
func TestSchemaValidator_UnionOfShapes(t *testing.T) {
	site := validNode("s", "pkg.F#CallSite@3", types.NodeCallSite)
	site.SyncUnknown = []types.SyncUnknown{{Op: types.SyncOp("lock"), Reason: types.SyncNoOrigin}}
	commit := types.Node{ID: "c", Type: types.NodeCommit, Name: "abc", QualifiedName: "commit:abc",
		Confidence: types.ConfExtracted, Commit: &types.CommitMeta{Subject: "fix: x (#1)", PR: 1}}
	hunk := types.Node{ID: "h", Type: types.NodeHunk, Name: "h1", QualifiedName: "hunk:abc:f.go:1",
		FilePath: "f.go", Confidence: types.ConfExtracted}
	g := &graph.Graph{
		Nodes: []types.Node{
			validNode("f", "pkg.F", types.NodeFunction),
			validNode("x", "time.Now", types.NodeExternal),
			validNode("r", "pkg.F#ReturnStmt@2", types.NodeReturnStmt),
			validNode("l", "pkg.F#FuncLit@4", types.NodeFuncLit),
			site, commit, hunk,
		},
		Edges: []types.Edge{
			{Src: "f", Dst: "x", Type: types.EdgeCalls, Line: 1, Confidence: types.ConfExtracted},
			{Src: "f", Dst: "c", Type: types.EdgeChangedIn, Line: 1, Confidence: types.ConfExtracted},
			{Src: "f", Dst: "c", Type: types.EdgeChangedIn, Line: 2, Confidence: types.ConfExtracted, DispatchKind: types.DispatchCopy},
			{Src: "f", Dst: "c", Type: types.EdgeChangedIn, Line: 3, Confidence: types.ConfExtracted, DispatchKind: types.DispatchPastCopy},
			{Src: "f", Dst: "c", Type: types.EdgeBlame, Count: 3, Confidence: types.ConfExtracted},
			{Src: "c", Dst: "h", Type: types.EdgeHasHunk, Confidence: types.ConfExtracted},
			{Src: "h", Dst: "f", Type: types.EdgeModifies, Confidence: types.ConfExtracted},
			{Src: "r", Dst: "f", Type: types.EdgeJumpsTo, Line: 2, Confidence: types.ConfExtracted, DispatchKind: types.JumpReturn},
			{Src: "l", Dst: "f", Type: types.EdgeLiteralPassedTo, Line: 4, Confidence: types.ConfExtracted, DispatchKind: types.LiteralUseGo},
		},
	}
	r, err := NewSchemaValidator().Validate(context.Background(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Issues) != 0 {
		t.Errorf("issues %+v, want none", r.Issues)
	}
}
