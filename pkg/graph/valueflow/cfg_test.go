package valueflow

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func retained() Source {
	return Source{NodeID: "function", CanonicalID: "example.invalid/p.F", File: "p/a.go", Commit: strings.Repeat("a", 40), Language: "go", StartLine: 20, StartByte: 400}
}
func TestCFGPreservesBranchJoinSuccessorsAndRetainedCoordinates(t *testing.T) {
	source := []byte("func F(x int) int {\n y := x\n if x > 0 { y = 1 } else { y = 2 }\n return y\n}")
	r, e := BuildCFG(context.Background(), retained(), source, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != "partial" || r.Capability["data_dependence"] != "not_computed" || r.Source.Commit != retained().Commit {
		t.Fatalf("unsupported completion claim: %+v", r)
	}
	byKind := map[string]int{}
	for _, b := range r.Blocks {
		byKind[b.Kind] = b.ID
	}
	then, okThen := byKind["IfThen"]
	otherwise, okElse := byKind["IfElse"]
	join, okJoin := byKind["IfDone"]
	if !okThen || !okElse || !okJoin {
		t.Fatalf("missing branch/join: %+v", r.Blocks)
	}
	expected := map[string]string{"b0/b" + fmtInt(then): "true", "b0/b" + fmtInt(otherwise): "false", "b" + fmtInt(then) + "/b" + fmtInt(join): "", "b" + fmtInt(otherwise) + "/b" + fmtInt(join): ""}
	for _, edge := range r.Edges {
		key := edge.From + "/" + edge.To
		if label, ok := expected[key]; ok {
			if edge.Condition != label {
				t.Fatal("incorrect branch label", edge)
			}
			delete(expected, key)
		}
	}
	if len(expected) > 0 {
		t.Fatal("wrong executable successors", expected)
	}
	found := false
	for _, node := range r.Nodes {
		start, end := node.Location.StartByte-400, node.Location.EndByte-400
		if node.Text != string(source[start:end]) {
			t.Fatal("coordinates do not match retained blob")
		}
		if node.Text == "return y" {
			found = true
			if node.Location.StartLine != 23 {
				t.Fatal("retained line offset", node)
			}
		}
	}
	if !found {
		t.Fatal("return omitted")
	}
}
func fmtInt(value int) string { return strconv.Itoa(value) }
func TestCFGLoopBackEdgeAndUnreachableReturnAreNotLinearStatements(t *testing.T) {
	source := []byte("func F(x int) int { for x > 0 { x-- }; return x; x = 99 }")
	r, e := BuildCFG(context.Background(), retained(), source, Options{})
	if e != nil {
		t.Fatal(e)
	}
	loop, body := -1, -1
	for _, b := range r.Blocks {
		if b.Kind == "ForLoop" {
			loop = b.ID
		}
		if b.Kind == "ForBody" {
			body = b.ID
		}
	}
	back := false
	for _, edge := range r.Edges {
		if edge.From == "b"+fmtInt(body) && edge.To == "b"+fmtInt(loop) {
			back = true
		}
	}
	unreachable := false
	for _, b := range r.Blocks {
		if !b.Reachable && len(b.Nodes) > 0 {
			unreachable = true
		}
	}
	if !back || !unreachable {
		t.Fatal("loop/unreachable semantics lost", r.Blocks, r.Edges)
	}
}
func TestBoundsCancellationAndUnsupportedInputsStayExplicit(t *testing.T) {
	source := []byte("func F() { for { } }")
	for _, options := range []Options{{MaxSourceBytes: 8}, {MaxASTNodes: 2}, {MaxBlocks: 1}, {MaxNodes: 1}, {MaxEdges: 1}} {
		blob := source
		if options.MaxNodes == 1 {
			blob = []byte("func F(){x:=1;x++;_ = x}")
		}
		r, e := BuildCFG(context.Background(), retained(), blob, options)
		if e != nil {
			t.Fatal(e)
		}
		if r.Status != "budget_exhausted" || !r.Truncated {
			t.Fatalf("budget silently ignored: %+v", r)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := BuildCFG(ctx, retained(), source, Options{})
	if e != nil || r.Status != "budget_exhausted" {
		t.Fatal(r, e)
	}
	s := retained()
	s.Language = "ts"
	r, e = BuildCFG(context.Background(), s, []byte("function F(){}"), Options{})
	if e != nil || r.Status != "unsupported" {
		t.Fatal(r, e)
	}
	for _, blob := range []string{"func F()", "func F(){};func G(){}", "not Go"} {
		r, e = BuildCFG(context.Background(), retained(), []byte(blob), Options{})
		if e != nil || r.Status != "unsupported" {
			t.Fatal(r, e)
		}
	}
	if _, e = BuildCFG(context.Background(), retained(), source, Options{MaxNodes: 2000}); e == nil {
		t.Fatal("unbounded caller limit accepted")
	}
}
func TestOpaqueAndExceptionalEffectsAreNotClaimedPrecise(t *testing.T) {
	r, e := BuildCFG(context.Background(), retained(), []byte("func F(x bool){if x && check(){defer clean();go work();panic(1)}}"), Options{})
	if e != nil {
		t.Fatal(e)
	}
	found := map[string]bool{}
	for _, d := range r.Diagnostics {
		found[d.Code] = true
	}
	for _, code := range []string{"short_circuit_unsupported", "async_or_deferred_effect", "opaque_call", "dependence_not_computed"} {
		if !found[code] {
			t.Fatal("missing uncertainty", code)
		}
	}
}

func TestImplicitReturnLocationNeverEscapesRetainedSource(t *testing.T) {
	blob := []byte("func F() {}")
	r, e := BuildCFG(context.Background(), retained(), blob, Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, node := range r.Nodes {
		if node.Location.StartByte < 400 || node.Location.EndByte > 400+len(blob) || node.Location.EndLine < node.Location.StartLine {
			t.Fatalf("invalid implicit return coordinate: %+v", node)
		}
	}
}
