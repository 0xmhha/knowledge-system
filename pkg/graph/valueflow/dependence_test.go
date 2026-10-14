package valueflow

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func analyze(t *testing.T, code string) Result {
	t.Helper()
	r, e := Analyze(context.Background(), retained(), []byte(code), Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Truncated || r.Capability["data_dependence"] == "not_computed" {
		t.Fatalf("analysis incomplete: %+v", r)
	}
	return r
}
func idWithText(t *testing.T, r Result, text string) string {
	t.Helper()
	ids := []string{}
	for _, node := range r.Nodes {
		if node.Text == text {
			ids = append(ids, node.ID)
		}
	}
	if len(ids) != 1 {
		t.Fatalf("expected one %q, got%v in%+v", text, ids, r.Nodes)
	}
	return ids[0]
}
func relation(r Result, kind, from, to string) bool {
	for _, edge := range r.Edges {
		if edge.Kind == kind && edge.From == from && edge.To == to {
			return true
		}
	}
	return false
}
func diag(r Result, code string) bool {
	for _, d := range r.Diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}
func TestScalarRedefinitionKillsOldValueButPreservesNewInput(t *testing.T) {
	r := analyze(t, "func F(a,b int)int{x:=a;x=b;return x}")
	old, current, ret := idWithText(t, r, "x:=a"), idWithText(t, r, "x=b"), idWithText(t, r, "return x")
	if relation(r, "data_dependence", old, ret) || !relation(r, "data_dependence", current, ret) || !relation(r, "data_dependence", "entry", current) {
		t.Fatalf("wrong strong kill or input relation: %+v", r.Edges)
	}
	if r.Status != "analyzed_bounded" {
		t.Fatal(r.Status, r.Diagnostics)
	}
}
func TestBranchJoinRetainsBothReachingDefinitionsAndOnlyBranchBodyIsControlled(t *testing.T) {
	r := analyze(t, "func F(a,b int,flag bool)int{y:=a;if flag{y=b};return y}")
	initial, branch, ret, condition := idWithText(t, r, "y:=a"), idWithText(t, r, "y=b"), idWithText(t, r, "return y"), idWithText(t, r, "flag")
	if !relation(r, "data_dependence", initial, ret) || !relation(r, "data_dependence", branch, ret) {
		t.Fatal("join dropped possible value", r.Edges)
	}
	if !relation(r, "control_dependence", condition, branch) || relation(r, "control_dependence", condition, ret) {
		t.Fatal("wrong postdominance control relation", r.Edges)
	}
}
func TestEarlyReturnsAreControlledByOppositeConditionBranches(t *testing.T) {
	r := analyze(t, "func F(flag bool)int{if flag{return 1};return 2}")
	condition := idWithText(t, r, "flag")
	expected := map[string]string{idWithText(t, r, "return 1"): "true", idWithText(t, r, "return 2"): "false"}
	for _, edge := range r.Edges {
		if edge.Kind == "control_dependence" && edge.From == condition {
			if want, ok := expected[edge.To]; ok {
				if edge.Condition != want {
					t.Fatal("opposite branch mislabeled", edge)
				}
				delete(expected, edge.To)
			}
		}
	}
	if len(expected) > 0 {
		t.Fatal("missing exit control relations", expected)
	}
}
func TestLexicalShadowScopeCannotLeakIntoOuterReturn(t *testing.T) {
	r := analyze(t, "func F(a,b int)int{x:=a;{x:=b;_ = x};return x}")
	outer, inner, ret := idWithText(t, r, "x:=a"), idWithText(t, r, "x:=b"), idWithText(t, r, "return x")
	if !relation(r, "data_dependence", outer, ret) || relation(r, "data_dependence", inner, ret) {
		t.Fatal("shadow name merged across scopes", r.Edges)
	}
	variables := []Variable{}
	for _, v := range r.Variables {
		if v.Name == "x" {
			variables = append(variables, v)
		}
	}
	if len(variables) != 2 || variables[0].ID == variables[1].ID {
		t.Fatal("shadow declarations lost identity", variables)
	}
}
func TestParallelAssignmentUsesOldValuesBeforeAllKills(t *testing.T) {
	r := analyze(t, "func F(a,b int)int{x:=a;y:=b;x,y=y,x;return x+y}")
	initialX, initialY, swap, ret := idWithText(t, r, "x:=a"), idWithText(t, r, "y:=b"), idWithText(t, r, "x,y=y,x"), idWithText(t, r, "return x+y")
	for _, from := range []string{initialX, initialY} {
		if !relation(r, "data_dependence", from, swap) || relation(r, "data_dependence", from, ret) {
			t.Fatal("parallel assignment ordering is wrong", r.Edges)
		}
	}
	if !relation(r, "data_dependence", swap, ret) {
		t.Fatal("swapped values not returned")
	}
}
func TestLoopFixedPointRetainsZeroIterationAndBackEdgeValues(t *testing.T) {
	r := analyze(t, "func F(x int)int{y:=0;for x>0{x--;y=x};return y}")
	initial, loop, ret := idWithText(t, r, "y:=0"), idWithText(t, r, "y=x"), idWithText(t, r, "return y")
	if !relation(r, "data_dependence", initial, ret) || !relation(r, "data_dependence", loop, ret) {
		t.Fatal("loop lost zero/multiple iteration possibilities", r.Edges)
	}
	decrement := idWithText(t, r, "x--")
	if !relation(r, "data_dependence", decrement, decrement) {
		t.Fatal("back-edge reaching definition lost", r.DefUse)
	}
	if r.Iterations < 2 {
		t.Fatal("no fixed point computation")
	}
}
func TestNamedReturnUsesActualReturnSiteAndZeroInitialization(t *testing.T) {
	r := analyze(t, "func F(flag bool)(out int){if flag{out=1};return}")
	ret := idWithText(t, r, "return")
	assignment := idWithText(t, r, "out=1")
	if !relation(r, "data_dependence", "entry", ret) || !relation(r, "data_dependence", assignment, ret) {
		t.Fatal("named return zero/assigned values lost", r.Edges)
	}
	returnLocation := Location{}
	for _, n := range r.Nodes {
		if n.ID == ret {
			returnLocation = n.Location
		}
	}
	found := false
	for _, o := range r.Occurrences {
		if o.Node == ret && o.Role == "implicit_return" {
			found = true
			if o.Location != returnLocation {
				t.Fatal("fabricated return-use citation", o)
			}
		}
	}
	if !found {
		t.Fatal("implicit return occurrence missing")
	}
}
func TestMemoryFieldWritesAreUnknownAndDoNotRedefineContainer(t *testing.T) {
	r := analyze(t, "func F(p *T,x int)int{p.value=x;return p.value}")
	if r.Status != "partial" || !diag(r, "memory_write_untracked") || !diag(r, "field_read_untracked") {
		t.Fatal("memory proof invented", r.Diagnostics)
	}
	writes := idWithText(t, r, "p.value=x")
	ret := idWithText(t, r, "return p.value")
	if relation(r, "data_dependence", writes, ret) {
		t.Fatal("unknown field flow treated as proven")
	}
	for _, o := range r.Occurrences {
		if o.Role == "definition" && o.Node == writes {
			t.Fatal("indirect write strongly killed scalar pointer", o)
		}
	}
}
func TestClosureBodyAndUnreachableWritesNeverKillOuterScalar(t *testing.T) {
	r := analyze(t, "func F(x int)int{fn:=func(){x=9};_ = fn;return x;x=2}")
	if !diag(r, "closure_value_unknown") {
		t.Fatal("closure uncertainty omitted")
	}
	ret := idWithText(t, r, "return x")
	if !relation(r, "data_dependence", "entry", ret) {
		t.Fatal("closure or unreachable write killed x", r.Edges)
	}
	dead := idWithText(t, r, "x=2")
	if relation(r, "data_dependence", dead, ret) {
		t.Fatal("unreachable effect entered flow")
	}
}
func TestNonExitingPathCannotAcquireNormalExitControlProof(t *testing.T) {
	r := analyze(t, "func F(flag bool){if flag{for{}}}")
	if !diag(r, "nontermination_control_unknown") || r.Status != "partial" {
		t.Fatal("non-exiting branch unreported", r.Diagnostics)
	}
	for _, edge := range r.Edges {
		if edge.Kind == "control_dependence" {
			t.Fatal("control proof through non-exiting region", edge)
		}
	}
}
func TestIterationFactsAndEdgeBudgetsNeverBecomeCompleteAnalysis(t *testing.T) {
	for _, options := range []Options{{MaxIterations: 1}, {MaxFacts: 1}, {MaxEdges: 3}} {
		r, e := Analyze(context.Background(), retained(), []byte("func F(a,b int,flag bool)int{x:=a;if flag{x=b};return x}"), options)
		if e != nil {
			t.Fatal(e)
		}
		if r.Status != "budget_exhausted" || !r.Truncated {
			t.Fatal("budget not honored", options, r)
		}
	}
}
func TestStableIdentityRelationsAndMissingExternalSources(t *testing.T) {
	code := "func F(x int)int{return x+global}"
	r := analyze(t, code)
	again := analyze(t, code)
	if !diag(r, "external_value_unknown") || r.Status != "partial" {
		t.Fatal("unknown global looked safe")
	}
	if !reflect.DeepEqual(StableRelations(r), StableRelations(again)) {
		t.Fatal("nondeterministic relations")
	}
	for _, v := range r.Variables {
		if v.Name == "global" {
			t.Fatal("invented local/global identity")
		}
	}
	for _, edge := range r.Edges {
		if strings.Contains(edge.Variable, "global") {
			t.Fatal("invented external value definition")
		}
	}
}

func TestBoundedCostAndCitationCoverageOnLargerScalarFunction(t *testing.T) {
	code := "func F(x int)int{" + strings.Repeat("x+=1;", 250) + "return x}"
	r := analyze(t, code)
	if r.Status != "analyzed_bounded" || len(r.Nodes) > r.Limits.MaxNodes || len(r.Edges) > r.Limits.MaxEdges || r.FactsPeak > r.Limits.MaxFacts || r.Iterations > r.Limits.MaxIterations {
		t.Fatal("bounded result/accounting failed", r.Status, r.FactsPeak, r.Iterations)
	}
	if len(r.DefUse) < 250 {
		t.Fatal("larger chain dependencies dropped")
	}
	for _, o := range r.Occurrences {
		if o.Location.StartByte < retained().StartByte || o.Location.EndByte > retained().StartByte+len(code) || o.Location.EndLine < o.Location.StartLine {
			t.Fatal("occurrence citation outside frozen source", o)
		}
	}
	if r.ElapsedMicros > int64(r.Limits.TimeoutMS)*1000 {
		t.Fatal("completed result exceeded frozen time budget", r.ElapsedMicros)
	}
}

func TestMethodLocalScalarBindingsSurviveMissingReceiverPackageType(t *testing.T) {
	r := analyze(t, "func (owner *Missing) F(x int)int{y:=x;return y}")
	if !relation(r, "data_dependence", idWithText(t, r, "y:=x"), idWithText(t, r, "return y")) {
		t.Fatal("method locals dropped with receiver metadata", r.Variables, r.Diagnostics)
	}
	if r.Status != "partial" {
		t.Fatal("missing receiver type incorrectly validated")
	}
}

func TestLoopConditionCanControlItsNextEvaluation(t *testing.T) {
	r := analyze(t, "func F(x int)int{for x>0{x--};return x}")
	condition := idWithText(t, r, "x>0")
	if !relation(r, "control_dependence", condition, condition) {
		t.Fatal("loop self control dependence missing", r.Edges)
	}
}
func TestReferenceCopiesAreNotClaimedScalarOrMemoryDependencies(t *testing.T) {
	r := analyze(t, "func F(pointer *int)*int{copy:=pointer;return copy}")
	assignment := idWithText(t, r, "copy:=pointer")
	returned := idWithText(t, r, "return copy")
	if !diag(r, "reference_alias_unknown") || r.Status != "partial" || relation(r, "data_dependence", assignment, returned) || !relation(r, "reference_dependence", assignment, returned) {
		t.Fatal("reference binding was promoted to scalar/memory proof", r.Edges, r.Diagnostics)
	}
}

func TestLocalConstantsHaveDefinitionUsesWithoutInvalidWriteKills(t *testing.T) {
	r := analyze(t, "func F()int{const value=3;return value}")
	definition := idWithText(t, r, "value=3")
	returned := idWithText(t, r, "return value")
	if !relation(r, "data_dependence", definition, returned) {
		t.Fatal("local constant source lost", r.Nodes, r.Edges, r.Diagnostics)
	}
	if len(r.Variables) != 1 || r.Variables[0].Kind != "constant" {
		t.Fatal("constant provenance lost", r.Variables)
	}
	invalid := analyze(t, "func F()int{const value=3;value=4;return value}")
	if !diag(invalid, "invalid_constant_write") || relation(invalid, "data_dependence", idWithText(t, invalid, "value=4"), idWithText(t, invalid, "return value")) {
		t.Fatal("invalid constant assignment became a definition", invalid.Edges)
	}
}
func TestGenericValueKindRemainsUnknownInsteadOfReferenceOrScalarProof(t *testing.T) {
	r := analyze(t, "func F[T any](input T)T{copy:=input;return copy}")
	assignment := idWithText(t, r, "copy:=input")
	returned := idWithText(t, r, "return copy")
	if !diag(r, "generic_value_kind_unknown") || r.Status != "partial" || !relation(r, "unknown_dependence", assignment, returned) || relation(r, "data_dependence", assignment, returned) || relation(r, "reference_dependence", assignment, returned) {
		t.Fatal("uninstantiated generic value got a false type claim", r.Edges, r.Diagnostics)
	}
}
