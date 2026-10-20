package valueflow

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

type Variable struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Declaration Location `json:"declaration"`
}
type Occurrence struct {
	ID        string   `json:"id"`
	Node      string   `json:"node"`
	Variable  string   `json:"variable"`
	Role      string   `json:"role"`
	Location  Location `json:"location"`
	Reachable bool     `json:"reachable"`
}
type DefUse struct {
	Definition string `json:"definition"`
	Use        string `json:"use"`
	Variable   string `json:"variable"`
	Certainty  string `json:"certainty"`
}
type factSet map[string]bool
type flowState map[string]factSet
type effects struct {
	uses []Occurrence
	defs []Occurrence
}
type flow struct {
	a                          *analysis
	variables                  map[types.Object]Variable
	effects                    map[string]effects
	occurrences                map[string]Occurrence
	diagnosticKeys             map[string]bool
	stopped                    bool
	constants                  []string
	liveStateFacts             int
	selectedReceiveAssignments map[*ast.AssignStmt]bool
	selectedReceiveLHS         map[*ast.Ident]bool
}

func (a *analysis) compute() {
	f := &flow{a: a, variables: map[types.Object]Variable{}, effects: map[string]effects{}, occurrences: map[string]Occurrence{}, diagnosticKeys: map[string]bool{}, selectedReceiveAssignments: map[*ast.AssignStmt]bool{}, selectedReceiveLHS: map[*ast.Ident]bool{}}
	a.result.Status = "analyzed_bounded"
	// Package typing remains unavailable. Certainty is lexical may-reach, never
	// runtime path feasibility or memory alias proof.
	for _, d := range a.result.Diagnostics {
		if d.Code != "revision_unavailable" {
			a.result.Status = "partial"
		}
	}
	if len(a.result.Nodes) >= a.result.Limits.MaxNodes {
		f.stop("Parameter entry node exceeds node budget")
		return
	}
	a.result.Nodes = append(a.result.Nodes, Node{ID: "entry", Block: 0, Kind: "entry", Location: a.loc(a.fn.Type), Synthetic: true})
	a.result.Blocks[0].Nodes = append([]string{"entry"}, a.result.Blocks[0].Nodes...)
	// Declaration identity comes from isolated Go type bindings,
	// not from a variable name. Same-name shadow scopes stay separate.
	ast.Inspect(a.fn, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		switch v := node.(type) {
		case *ast.CommClause:
			if assignment, ok := v.Comm.(*ast.AssignStmt); ok && len(assignment.Rhs) == 1 {
				if receive, ok := ast.Unparen(assignment.Rhs[0]).(*ast.UnaryExpr); ok && receive.Op == token.ARROW {
					f.selectedReceiveAssignments[assignment] = true
					for _, lhs := range assignment.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
							f.selectedReceiveLHS[id] = true
						}
					}
				}
			}
		case *ast.ValueSpec:
			for _, id := range v.Names {
				f.declare(id, "local")
			}
		case *ast.AssignStmt:
			if v.Tok == token.DEFINE {
				for _, lhs := range v.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && f.a.typeInfo.Defs[id] != nil {
						f.declare(id, "local")
					}
				}
			}
		case *ast.RangeStmt:
			f.warn("range_binding_unsupported", "Range iteration bindings are not modeled as proven scalar assignments", v)
			for _, expr := range []ast.Expr{v.Key, v.Value} {
				if id, ok := expr.(*ast.Ident); ok && f.a.typeInfo.Defs[id] != nil {
					f.declare(id, "range_unknown")
				}
			}
		}
		return true
	})
	entry := effects{}
	fields := []struct {
		list *ast.FieldList
		kind string
	}{{a.fn.Recv, "receiver"}, {a.fn.Type.Params, "parameter"}, {a.fn.Type.Results, "result_zero"}}
	for _, group := range fields {
		if group.list == nil {
			continue
		}
		for _, field := range group.list.List {
			for _, id := range field.Names {
				f.declare(id, group.kind)
				if o, ok := f.occurrence("entry", id, "definition", true); ok {
					entry.defs = append(entry.defs, o)
				}
			}
		}
	}
	// Constants are compile-time definitions, not invented executable CFG
	// statements. Block -1 identifies their source nodes; entry facts seed
	// lexical visibility while Go type objects preserve constant shadow scopes.
	ast.Inspect(a.fn.Body, func(node ast.Node) bool {
		if _, literal := node.(*ast.FuncLit); literal {
			return false
		}
		declaration, ok := node.(*ast.GenDecl)
		if !ok || declaration.Tok != token.CONST {
			return true
		}
		for _, item := range declaration.Specs {
			spec, ok := item.(*ast.ValueSpec)
			if !ok {
				continue
			}
			known := false
			for _, id := range spec.Names {
				known = known || f.a.typeInfo.Defs[id] != nil
			}
			if !known {
				continue
			}
			if len(a.result.Nodes) >= a.result.Limits.MaxNodes {
				f.stop("Constant source node budget exhausted")
				return false
			}
			location := a.loc(spec)
			id := fmt.Sprintf("constant/%d", location.StartByte)
			a.result.Nodes = append(a.result.Nodes, Node{ID: id, Block: -1, Kind: "compile_time_constant", Location: location, Text: string(a.blob[location.StartByte-a.result.Source.StartByte : location.EndByte-a.result.Source.StartByte])})
			a.astNodes[id] = spec
			e := f.extract(id, spec, true)
			f.effects[id] = e
			f.constants = append(f.constants, id)
			entry.defs = append(entry.defs, e.defs...)
		}
		return true
	})
	f.effects["entry"] = entry
	for _, block := range a.result.Blocks {
		for _, id := range block.Nodes {
			if id == "entry" {
				continue
			}
			f.effects[id] = f.extract(id, a.astNodes[id], block.Reachable)
		}
	}
	modeledReceives := map[Location]bool{}
	for _, occurrence := range f.occurrences {
		if occurrence.Role == "definition" {
			modeledReceives[occurrence.Location] = true
		}
	}
	receiveIDs := make([]*ast.Ident, 0, len(f.selectedReceiveLHS))
	for id := range f.selectedReceiveLHS {
		receiveIDs = append(receiveIDs, id)
	}
	sort.Slice(receiveIDs, func(i, j int) bool { return receiveIDs[i].Pos() < receiveIDs[j].Pos() })
	for _, id := range receiveIDs {
		if !f.check() {
			break
		}
		if !modeledReceives[a.loc(id)] {
			f.warn("selected_receive_lhs_unmodeled", "The CFG has no selected-case owner for this receive target; no preselection definition is invented", id)
		}
	}
	for _, v := range f.variables {
		a.result.Variables = append(a.result.Variables, v)
	}
	sort.Slice(a.result.Variables, func(i, j int) bool {
		return a.result.Variables[i].Declaration.StartByte < a.result.Variables[j].Declaration.StartByte
	})
	for _, o := range f.occurrences {
		a.result.Occurrences = append(a.result.Occurrences, o)
	}
	sort.Slice(a.result.Occurrences, func(i, j int) bool { return a.result.Occurrences[i].ID < a.result.Occurrences[j].ID })
	if !f.stopped {
		f.reaching()
	}
	if !f.stopped {
		f.control()
	}
	if !f.stopped {
		a.result.Capability["def_use"] = "lexical_may_reach"
		a.result.Capability["data_dependence"] = "local_scalar_may_reach"
	}
}
func (f *flow) stop(message string) {
	if f.stopped {
		return
	}
	f.stopped = true
	f.a.result.Status = "budget_exhausted"
	f.a.result.Truncated = true
	f.a.result.Diagnostics = append(f.a.result.Diagnostics, Diagnostic{Code: "budget_exhausted", Message: message})
}
func (f *flow) check() bool {
	if f.stopped {
		return false
	}
	if f.a.context.Err() != nil {
		f.stop("Dependence analysis context ended")
		return false
	}
	return true
}
func (f *flow) iteration() bool {
	if !f.check() {
		return false
	}
	if f.a.result.Iterations >= f.a.result.Limits.MaxIterations {
		f.stop("Fixed-point iteration budget exhausted")
		return false
	}
	f.a.result.Iterations++
	return true
}
func (f *flow) facts(count int) bool {
	if count > f.a.result.Limits.MaxFacts {
		f.stop(fmt.Sprintf("Analysis fact reservation requires %d, limit %d", count, f.a.result.Limits.MaxFacts))
		return false
	}
	f.a.result.FactsPeak = max(f.a.result.FactsPeak, count)
	return f.check()
}
func (f *flow) warn(code, message string, node ast.Node) {
	pos := token.NoPos
	if node != nil {
		pos = node.Pos()
	}
	key := fmt.Sprintf("%s/%d", code, pos)
	if f.diagnosticKeys[key] {
		return
	}
	f.diagnosticKeys[key] = true
	var location *Location
	if node != nil {
		value := f.a.loc(node)
		location = &value
	}
	f.a.result.Diagnostics = append(f.a.result.Diagnostics, Diagnostic{Code: code, Message: message, Location: location})
	if f.a.result.Status != "budget_exhausted" {
		f.a.result.Status = "partial"
	}
}
func (f *flow) declare(id *ast.Ident, kind string) {
	if !f.check() || id.Name == "_" || f.a.typeInfo.Defs[id] == nil {
		return
	}
	object := f.a.typeInfo.Defs[id]
	switch value := object.(type) {
	case *types.Var:
		if value.IsField() {
			return
		}
	case *types.Const:
		kind = "constant"
	default:
		return
	}
	v, found := f.variables[object]
	if !found {
		if !f.facts(len(f.variables) + len(f.occurrences) + 1) {
			return
		}
		location := f.a.loc(id)
		v = Variable{ID: fmt.Sprintf("v%d", location.StartByte), Name: id.Name, Declaration: location}
	}
	v.Kind = kind
	f.variables[object] = v
}
func (f *flow) occurrence(node string, id *ast.Ident, role string, live bool) (Occurrence, bool) {
	return f.occurrenceAt(node, id, role, f.a.loc(id), live)
}
func (f *flow) occurrenceAt(node string, id *ast.Ident, role string, location Location, live bool) (Occurrence, bool) {
	if !f.check() {
		return Occurrence{}, false
	}
	object := f.a.typeInfo.Uses[id]
	if role == "definition" {
		object = f.a.typeInfo.Defs[id]
		if object == nil {
			object = f.a.typeInfo.Uses[id]
		}
	}
	if role == "implicit_return" {
		object = f.a.typeInfo.Defs[id]
	}
	v, ok := f.variables[object]
	if !ok {
		return Occurrence{}, false
	}
	if role == "use" || role == "implicit_return" {
		if _, generic := object.Type().(*types.TypeParam); generic {
			role = "unknown_value"
			f.warn("generic_value_kind_unknown", "Uninstantiated type parameter value kind remains unknown", id)
		} else {
			switch object.Type().Underlying().(type) {
			case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface, *types.Signature:
				if role == "implicit_return" {
					role = "implicit_reference_return"
				} else {
					role = "reference"
				}
				f.warn("reference_alias_unknown", "Reference binding flow does not establish memory alias/effect semantics", id)
			case *types.Struct, *types.Array:
				if role == "implicit_return" {
					role = "implicit_aggregate_return"
				} else {
					role = "aggregate"
				}
				f.warn("aggregate_value_unknown", "Aggregate contents and alias relationships remain outside scalar flow", id)
			}
		}
	}
	if role == "definition" && v.Kind == "constant" {
		role = "constant_definition"
	}
	out := Occurrence{ID: fmt.Sprintf("%s/%s/%s/%d", node, role, v.ID, location.StartByte), Node: node, Variable: v.ID, Role: role, Location: location, Reachable: live}
	if _, exists := f.occurrences[out.ID]; !exists && !f.facts(len(f.variables)+len(f.occurrences)+1) {
		return Occurrence{}, false
	}
	f.occurrences[out.ID] = out
	return out, true
}
func (f *flow) extract(nodeID string, node ast.Node, live bool) effects {
	out := effects{}
	var read func(ast.Expr, string)
	read = func(expr ast.Expr, role string) {
		if expr == nil || !f.check() {
			return
		}
		switch v := expr.(type) {
		case *ast.Ident:
			if o, ok := f.occurrence(nodeID, v, role, live); ok {
				out.uses = append(out.uses, o)
			} else if f.a.typeInfo.Uses[v] == nil && v.Name != "_" && v.Name != "true" && v.Name != "false" && v.Name != "nil" {
				f.warn("external_value_unknown", "Value read has no local lexical definition; no source edge is invented", v)
			}
		case *ast.BasicLit:
		case *ast.ParenExpr:
			read(v.X, role)
		case *ast.BinaryExpr:
			read(v.X, role)
			read(v.Y, role)
		case *ast.UnaryExpr:
			if v.Op == token.AND || v.Op == token.ARROW {
				f.warn("alias_or_channel_effect", "Address/channel value effects are not scalar assignments", v)
				read(v.X, "address")
			} else {
				read(v.X, role)
			}
		case *ast.StarExpr:
			f.warn("memory_read_untracked", "Pointer memory contents are not reaching scalar definitions", v)
			read(v.X, "address")
		case *ast.SelectorExpr:
			f.warn("field_read_untracked", "Field memory/type/alias relationships are not proven from a function blob", v)
			read(v.X, "address")
		case *ast.IndexExpr:
			f.warn("element_read_untracked", "Indexed element memory/alias relationships are unknown", v)
			read(v.X, "address")
			read(v.Index, "use")
		case *ast.IndexListExpr:
			read(v.X, "address")
			for _, index := range v.Indices {
				read(index, "use")
			}
			f.warn("generic_target_unknown", "Generic type/value binding is unavailable", v)
		case *ast.SliceExpr:
			read(v.X, "address")
			read(v.Low, "use")
			read(v.High, "use")
			read(v.Max, "use")
			f.warn("slice_alias_unknown", "Slice result may alias its source; element dependencies are unavailable", v)
		case *ast.CallExpr:
			switch fun := v.Fun.(type) {
			case *ast.Ident:
				if _, ok := f.a.typeInfo.Uses[fun].(*types.Var); ok {
					read(fun, "use")
				}
			case *ast.SelectorExpr:
				read(fun.X, "address")
			default:
				read(v.Fun, "use")
			}
			for _, arg := range v.Args {
				read(arg, "use")
			}
		case *ast.FuncLit:
			f.warn("closure_value_unknown", "Captured variable effects are not inlined into caller execution", v)
		case *ast.CompositeLit:
			for _, element := range v.Elts {
				if kv, ok := element.(*ast.KeyValueExpr); ok {
					if id, ok := kv.Key.(*ast.Ident); ok {
						if object, ok := f.a.typeInfo.Uses[id].(*types.Var); ok && !object.IsField() {
							read(kv.Key, "use")
						}
					} else {
						read(kv.Key, "use")
					}
					read(kv.Value, "use")
				} else {
					read(element, "use")
				}
			}
			f.warn("aggregate_alias_unknown", "Aggregate memory/type relationships are not proven", v)
		case *ast.TypeAssertExpr:
			read(v.X, role)
			f.warn("type_assertion_unknown", "Type assertion success/panic paths are unavailable", v)
		default:
			f.warn("expression_unsupported", "Expression value semantics are outside the scalar subset", expr)
		}
	}
	define := func(expr ast.Expr) {
		if id, ok := expr.(*ast.Ident); ok {
			if _, constant := f.a.typeInfo.Uses[id].(*types.Const); constant && f.a.typeInfo.Defs[id] == nil {
				f.warn("invalid_constant_write", "Invalid constant write is not a new reaching definition", id)
				return
			}
			if o, known := f.occurrence(nodeID, id, "definition", live); known {
				out.defs = append(out.defs, o)
			}
		} else {
			f.warn("memory_write_untracked", "Indirect writes do not kill or define scalar container variables", expr)
			read(expr, "address")
		}
	}
	switch v := node.(type) {
	case *ast.AssignStmt:
		for _, rhs := range v.Rhs {
			read(rhs, "use")
		}
		// The CFG retains the whole receive assignment at select entry to
		// evaluate channel operands. Its definitions belong only to the
		// selected case's separate LHS nodes, never to every alternative.
		if f.selectedReceiveAssignments[v] {
			return out
		}
		for _, lhs := range v.Lhs {
			if v.Tok != token.ASSIGN && v.Tok != token.DEFINE {
				read(lhs, "use")
			}
			define(lhs)
		}
	case *ast.ValueSpec:
		for _, value := range v.Values {
			read(value, "use")
		}
		for _, id := range v.Names {
			define(id)
		}
	case *ast.IncDecStmt:
		read(v.X, "use")
		define(v.X)
	case *ast.ReturnStmt:
		for _, expr := range v.Results {
			read(expr, "use")
		}
		if len(v.Results) == 0 && f.a.fn.Type.Results != nil {
			for _, field := range f.a.fn.Type.Results.List {
				for _, id := range field.Names {
					if occurrence, ok := f.occurrenceAt(nodeID, id, "implicit_return", f.a.loc(v), live); ok {
						out.uses = append(out.uses, occurrence)
					}
				}
			}
		}
	case *ast.ExprStmt:
		read(v.X, "use")
	case *ast.GoStmt:
		read(v.Call, "use")
	case *ast.DeferStmt:
		read(v.Call, "use")
	case *ast.SendStmt:
		read(v.Chan, "address")
		read(v.Value, "use")
		f.warn("channel_write_untracked", "Channel transfer is not a local scalar definition", v)
	case ast.Expr:
		if id, ok := v.(*ast.Ident); ok && f.selectedReceiveLHS[id] {
			define(id)
		} else {
			read(v, "use")
		}
	default:
		if node != nil {
			f.warn("statement_unsupported", "Statement effects are outside the scalar subset", node)
		}
	}
	// Parameters in bare returns use their declaration coordinate; the node
	// coordinate remains the return site, not a fabricated source span.
	return out
}
func copyState(in flowState) flowState {
	out := flowState{}
	for variable, defs := range in {
		set := factSet{}
		for definition := range defs {
			set[definition] = true
		}
		out[variable] = set
	}
	return out
}
func sameState(a, b flowState) bool {
	if len(a) != len(b) {
		return false
	}
	for v, set := range a {
		if len(set) != len(b[v]) {
			return false
		}
		for d := range set {
			if !b[v][d] {
				return false
			}
		}
	}
	return true
}
func transfer(in flowState, ids []string, effects map[string]effects) flowState {
	state := copyState(in)
	for _, id := range ids {
		for _, definition := range effects[id].defs {
			state[definition.Variable] = factSet{definition.ID: true}
		}
	}
	return state
}
func (f *flow) reaching() {
	defer func() { f.liveStateFacts = 0 }()
	count := len(f.a.graph.Blocks)
	pred := make([][]int, count)
	for _, b := range f.a.graph.Blocks {
		if b.Live {
			for _, succ := range b.Succs {
				if succ.Live {
					pred[succ.Index] = append(pred[succ.Index], int(b.Index))
				}
			}
		}
	}
	in, out := make([]flowState, count), make([]flowState, count)
	for i := range count {
		in[i] = flowState{}
		out[i] = flowState{}
	}
	for changed := true; changed; {
		if !f.iteration() {
			return
		}
		changed = false
		for _, block := range f.a.result.Blocks {
			if !block.Reachable {
				continue
			}
			next := flowState{}
			for _, p := range pred[block.ID] {
				for v, defs := range out[p] {
					if next[v] == nil {
						next[v] = factSet{}
					}
					for d := range defs {
						if !next[v][d] && !f.facts(len(f.variables)+len(f.occurrences)+f.liveStateFacts+stateSize(next)+1) {
							return
						}
						next[v][d] = true
					}
				}
			}
			definitions := 0
			for _, id := range block.Nodes {
				definitions += len(f.effects[id].defs)
			}
			if !f.facts(len(f.variables) + len(f.occurrences) + f.liveStateFacts + 2*stateSize(next) + definitions) {
				return
			}
			transferred := transfer(next, block.Nodes, f.effects)
			if !sameState(out[block.ID], transferred) {
				changed = true
			}
			f.liveStateFacts -= stateSize(in[block.ID]) + stateSize(out[block.ID])
			in[block.ID] = next
			out[block.ID] = transferred
			f.liveStateFacts += stateSize(next) + stateSize(transferred)
		}

		if !f.facts(len(f.variables) + len(f.occurrences) + f.liveStateFacts) {
			return
		}
	}
	constantState := flowState{}
	for _, id := range f.constants {
		for _, definition := range f.effects[id].defs {
			constantState[definition.Variable] = factSet{definition.ID: true}
		}
	}
	for _, id := range f.constants {
		if !f.connect(f.effects[id].uses, constantState) {
			return
		}
	}
	for _, block := range f.a.result.Blocks {
		if !block.Reachable {
			continue
		}
		state := copyState(in[block.ID])
		for _, node := range block.Nodes {
			e := f.effects[node]
			if !f.connect(e.uses, state) {
				return
			}
			for _, definition := range e.defs {
				state[definition.Variable] = factSet{definition.ID: true}
			}
		}
	}
}
func (f *flow) edge(edge Edge) bool {
	for _, old := range f.a.result.Edges {
		if old == edge {
			return f.check()
		}
	}
	if len(f.a.result.Edges) >= f.a.result.Limits.MaxEdges {
		f.stop("Dependence edge budget exhausted")
		return false
	}
	f.a.result.Edges = append(f.a.result.Edges, edge)
	return f.check()
}

// Normal-exit postdominators. Blocks with a possible path into a non-exiting
// region are excluded, rather than assigning vacuous control dependencies.
func (f *flow) control() {
	n := len(f.a.graph.Blocks)
	exit := n
	canExit := make([]bool, n+1)
	canExit[exit] = true
	for changed := true; changed; {
		if !f.iteration() {
			return
		}
		changed = false
		for _, b := range f.a.graph.Blocks {
			if !b.Live || canExit[b.Index] {
				continue
			}
			yes := len(b.Succs) == 0
			for _, s := range b.Succs {
				yes = yes || canExit[s.Index]
			}
			if yes {
				canExit[b.Index] = true
				changed = true
			}
		}
	}
	eligible := append([]bool(nil), canExit...)
	for changed := true; changed; {
		if !f.iteration() {
			return
		}
		changed = false
		for _, b := range f.a.graph.Blocks {
			if !eligible[b.Index] {
				continue
			}
			for _, s := range b.Succs {
				if !eligible[s.Index] {
					eligible[b.Index] = false
					changed = true
					break
				}
			}
		}
	}
	universe := factSet{fmt.Sprint(exit): true}
	for _, b := range f.a.graph.Blocks {
		if b.Live && eligible[b.Index] {
			universe[fmt.Sprint(b.Index)] = true
		} else if b.Live {
			f.warn("nontermination_control_unknown", "Normal-exit postdominators exclude possible non-exiting paths", b.Stmt)
		}
	}
	if !f.facts(len(universe)*len(universe) + 3*len(universe) + len(f.variables) + len(f.occurrences) + len(f.a.result.DefUse)) {
		return
	}
	pd := make([]factSet, n+1)
	pd[exit] = factSet{fmt.Sprint(exit): true}
	for i := range n {
		pd[i] = factSet{}
		if eligible[i] {
			for value := range universe {
				pd[i][value] = true
			}
		}
	}

	for changed := true; changed; {
		if !f.iteration() {
			return
		}
		changed = false
		for _, b := range f.a.graph.Blocks {
			index := int(b.Index)
			if !b.Live || !eligible[index] {
				continue
			}
			successors := []int{}
			for _, s := range b.Succs {
				successors = append(successors, int(s.Index))
			}
			if len(successors) == 0 {
				successors = []int{exit}
			}
			next := factSet{}
			for value := range pd[successors[0]] {
				present := true
				for _, s := range successors[1:] {
					present = present && pd[s][value]
				}
				if present {
					next[value] = true
				}
			}
			next[fmt.Sprint(index)] = true
			if !sameSet(pd[index], next) {
				pd[index] = next
				changed = true
			}
		}
	}
	for _, b := range f.a.graph.Blocks {
		if !b.Live || !eligible[b.Index] || len(b.Succs) < 2 {
			continue
		}
		from := fmt.Sprintf("b%d", b.Index)
		if nodes := f.a.result.Blocks[b.Index].Nodes; len(nodes) > 0 {
			from = nodes[len(nodes)-1]
		}
		for branch, succ := range b.Succs {
			for _, dependent := range f.a.graph.Blocks {
				key := fmt.Sprint(dependent.Index)
				if !dependent.Live || (pd[b.Index][key] && dependent.Index != b.Index) || !pd[succ.Index][key] {
					continue
				}
				condition := fmt.Sprintf("branch_%d", branch)
				for _, e := range f.a.result.Edges {
					if e.Kind == "cfg" && e.From == fmt.Sprintf("b%d", b.Index) && e.To == fmt.Sprintf("b%d", succ.Index) {
						condition = e.Condition
						break
					}
				}
				targets := f.a.result.Blocks[dependent.Index].Nodes
				if len(targets) == 0 {
					targets = []string{fmt.Sprintf("b%d", dependent.Index)}
				}
				for _, to := range targets {
					if !f.edge(Edge{From: from, To: to, Kind: "control_dependence", Condition: condition, Certainty: "normal_exit_structural"}) {
						return
					}
				}
			}
		}
	}
	f.a.result.Capability["control_dependence"] = "normal_exit_postdominators"
}
func sameSet(a, b factSet) bool {
	if len(a) != len(b) {
		return false
	}
	for x := range a {
		if !b[x] {
			return false
		}
	}
	return true
}

// StableRelations is useful to compare repeated executions without machine
// timing. It never hides diagnostics/capability or promotes partial to precise.
func StableRelations(r Result) []string {
	out := []string{}
	for _, e := range r.Edges {
		out = append(out, strings.Join([]string{e.From, e.To, e.Kind, e.Variable, e.Condition, e.Certainty}, "|"))
	}
	sort.Strings(out)
	return out
}

func (f *flow) connect(uses []Occurrence, state flowState) bool {
	for _, use := range uses {
		definitions := state[use.Variable]
		if len(definitions) == 0 {
			f.warn("reaching_definition_unknown", "No reachable local definition; missing value source remains unknown", f.a.astNodes[use.Node])
			continue
		}
		sorted := make([]string, 0, len(definitions))
		for id := range definitions {
			sorted = append(sorted, id)
		}
		sort.Strings(sorted)
		for _, id := range sorted {
			definition := f.occurrences[id]
			if len(f.a.result.DefUse) >= f.a.result.Limits.MaxFacts {
				f.stop("Def-use relation budget exhausted")
				return false
			}
			if !f.facts(f.liveStateFacts + len(f.variables) + len(f.occurrences) + len(f.a.result.DefUse) + 1) {
				return false
			}
			f.a.result.DefUse = append(f.a.result.DefUse, DefUse{Definition: id, Use: use.ID, Variable: use.Variable, Certainty: "lexical_may_reach"})
			kind := "data_dependence"
			switch use.Role {
			case "address":
				kind = "address_dependence"
			case "reference", "implicit_reference_return":
				kind = "reference_dependence"
			case "aggregate", "implicit_aggregate_return":
				kind = "aggregate_dependence"
			case "unknown_value":
				kind = "unknown_dependence"
			}
			if !f.edge(Edge{From: definition.Node, To: use.Node, Kind: kind, Variable: use.Variable, Certainty: "may_reach"}) {
				return false
			}
		}
	}
	return f.check()
}

func stateSize(state flowState) int {
	n := 0
	for _, defs := range state {
		n += len(defs)
	}
	return n
}
