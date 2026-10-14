package valueflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

const RulePackID = "go-local-review@1.0.0"
const rulePackDefinition = `{"id":"go-local-review","version":"1.0.0","rules":[{"id":"parameter-command-name","source":"named scalar parameter","sink":"exec.Command arg0 or exec.CommandContext arg1 direct identifier"},{"id":"discarded-conversion-error","source":"strconv.Atoi/ParseInt/ParseUint first result with blank second target","sink":"explicit return direct identifier"}],"flow":"lexical reaching definitions and corresponding RHS uses","target_resolution":"syntax_only","sanitizer":"not_proven"}`

type RuleOptions struct {
	MaxCandidates int `json:"max_candidates"`
	MaxFlowSteps  int `json:"max_flow_steps"`
}
type RuleQueryInput struct {
	RulePack      string     `json:"rule_pack"`
	Function      QueryInput `json:"function"`
	MaxCandidates int        `json:"max_candidates,omitempty"`
	MaxFlowSteps  int        `json:"max_flow_steps,omitempty"`
}
type RuleCondition struct {
	Node      string   `json:"node"`
	Condition string   `json:"condition"`
	Location  Location `json:"location"`
}
type RuleCandidate struct {
	ID               string          `json:"id"`
	RuleID           string          `json:"rule_id"`
	Origin           string          `json:"origin"`
	SourceOccurrence string          `json:"source_occurrence"`
	SourceLocation   Location        `json:"source_location"`
	SinkOccurrence   string          `json:"sink_occurrence"`
	SinkLocation     Location        `json:"sink_location"`
	Path             []string        `json:"path"`
	Conditions       []RuleCondition `json:"conditions"`
	Certainty        string          `json:"certainty"`
	TargetResolution string          `json:"target_resolution"`
	Sanitizer        string          `json:"sanitizer"`
}
type RuleResult struct {
	PackID        string          `json:"pack_id"`
	PackVersion   string          `json:"pack_version"`
	PackSHA256    string          `json:"pack_sha256"`
	EngineVersion string          `json:"engine_version"`
	Origin        string          `json:"origin"`
	Status        string          `json:"status"`
	StopReason    string          `json:"stop_reason,omitempty"`
	Limits        RuleOptions     `json:"limits"`
	FlowSteps     int             `json:"flow_steps"`
	Candidates    []RuleCandidate `json:"candidates"`
	Limitations   []string        `json:"limitations"`
}

func ruleOptions(o RuleOptions) (RuleOptions, error) {
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 64
	}
	if o.MaxFlowSteps == 0 {
		o.MaxFlowSteps = 4096
	}
	if o.MaxCandidates < 1 || o.MaxCandidates > 64 || o.MaxFlowSteps < 1 || o.MaxFlowSteps > 4096 {
		return o, fmt.Errorf("rule limits outside supported bounds")
	}
	return o, nil
}
func DecodeRuleQueryInput(raw []byte) (RuleQueryInput, error) {
	var q RuleQueryInput
	if len(raw) > 8192 {
		return q, fmt.Errorf("rule input exceeds8KiB")
	}
	if e := validateInputObject(raw); e != nil {
		return q, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&q); e != nil {
		return q, e
	}
	if q.RulePack != RulePackID {
		return q, fmt.Errorf("explicit supported rule_pack required")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	function, e := DecodeQueryInput(fields["function"])
	if e != nil {
		return q, e
	}
	q.Function = function
	for _, name := range []string{"max_candidates", "max_flow_steps"} {
		if field, ok := fields[name]; ok {
			var n int
			if e = json.Unmarshal(field, &n); e != nil || n < 1 {
				return q, fmt.Errorf("explicit positive integer rule bound required")
			}
		}
	}
	_, e = ruleOptions(RuleOptions{q.MaxCandidates, q.MaxFlowSteps})
	return q, e
}
func QueryRules(ctx context.Context, r store.Reader, q RuleQueryInput) (QueryResult, error) {
	if q.RulePack != RulePackID {
		return QueryResult{}, fmt.Errorf("explicit supported rule_pack required")
	}
	o, e := ruleOptions(RuleOptions{q.MaxCandidates, q.MaxFlowSteps})
	if e != nil {
		return QueryResult{}, e
	}
	return query(ctx, r, q.Function, &o)
}

// AnalyzeRules reuses one parse/type/CFG computation; ordinary Analyze is off.
func AnalyzeRules(ctx context.Context, source Source, blob []byte, limits Options, pack string, o RuleOptions) (Result, error) {
	if pack != RulePackID {
		return Result{}, fmt.Errorf("explicit supported rule_pack required")
	}
	o, e := ruleOptions(o)
	if e != nil {
		return Result{}, e
	}
	return build(ctx, source, blob, limits, true, &o)
}
func selector(call *ast.CallExpr) (string, string) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", ""
	}
	return id.Name, sel.Sel.Name
}
func (a *analysis) matchRules(o RuleOptions) {
	r := newRuleResult(o)
	a.result.Rules = r
	stop := func(reason string) { r.Status = "incomplete"; r.StopReason = reason }
	if a.result.Truncated || a.result.Status == "unsupported" || a.result.Capability["def_use"] == "not_computed" {
		stop("base_analysis_incomplete")
		return
	}
	step := func() bool {
		if a.context.Err() != nil {
			stop("deadline")
			return false
		}
		if r.FlowSteps >= o.MaxFlowSteps {
			stop("flow_steps")
			return false
		}
		r.FlowSteps++
		return true
	}
	nodes := map[string]Node{}
	occ := map[string]Occurrence{}
	uses := map[string][]Occurrence{}
	incoming := map[string][]Occurrence{}
	variables := map[string]Variable{}
	for _, n := range a.result.Nodes {
		if !step() {
			return
		}
		nodes[n.ID] = n
	}
	for _, v := range a.result.Variables {
		if !step() {
			return
		}
		variables[v.ID] = v
	}
	definitions := map[string][]Occurrence{}
	for _, v := range a.result.Occurrences {
		if !step() {
			return
		}
		occ[v.ID] = v
		if v.Role == "definition" && v.Reachable {
			definitions[v.Node] = append(definitions[v.Node], v)
		}
		if v.Role == "use" && v.Reachable {
			uses[v.Node] = append(uses[v.Node], v)
		}
	}
	for _, e := range a.result.DefUse {
		if !step() {
			return
		}
		incoming[e.Use] = append(incoming[e.Use], occ[e.Definition])
	}
	// Conversion source identity is an occurrence, never a name-based join.
	discarded := map[string]bool{}
	for _, n := range a.result.Nodes {
		if !step() {
			return
		}
		stmt, ok := a.astNodes[n.ID].(*ast.AssignStmt)
		if !ok || len(stmt.Lhs) != 2 || len(stmt.Rhs) != 1 {
			continue
		}
		blank, ok := stmt.Lhs[1].(*ast.Ident)
		if !ok || blank.Name != "_" {
			continue
		}
		call, ok := stmt.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		owner, name := selector(call)
		if owner != "strconv" || (name != "Atoi" && name != "ParseInt" && name != "ParseUint") {
			continue
		}
		for _, v := range definitions[n.ID] {
			if v.Location == a.loc(stmt.Lhs[0]) {
				discarded[v.ID] = true
			}
		}
	}
	seenCandidates := map[string]bool{}
	// Walk each direct-identifier sink backwards through occurrence-level def-use.
	emit := func(rule string, sink Occurrence, sinkLocation Location) {
		type work struct {
			use  Occurrence
			path []string
		}
		pending := []work{{sink, []string{sink.Node}}}
		visited := map[string]bool{}
		for len(pending) > 0 && r.Status != "incomplete" {
			w := pending[0]
			pending = pending[1:]
			if visited[w.use.ID] {
				continue
			}
			visited[w.use.ID] = true
			for _, def := range incoming[w.use.ID] {
				if !step() {
					return
				}
				path := append([]string{def.Node}, w.path...)
				source := rule == "parameter-command-name" && def.Node == "entry" && variables[def.Variable].Kind == "parameter" || rule == "discarded-conversion-error" && discarded[def.ID]
				if source {
					id := hashBytes([]byte(r.PackSHA256 + "/" + a.result.Source.NodeID + "/" + a.result.Source.SnapshotSHA256 + "/" + a.result.Source.SHA256 + "/" + rule + "/" + def.ID + "/" + sink.ID))
					if seenCandidates[id] {
						continue
					}
					seenCandidates[id] = true
					if len(r.Candidates) >= o.MaxCandidates {
						stop("candidates")
						return
					}
					conditions := []RuleCondition{}
					seen := map[string]bool{}
					for _, edge := range a.result.Edges {
						if edge.Kind != "control_dependence" {
							continue
						}
						if !step() {
							return
						}
						for _, node := range path {
							if edge.To == node {
								key := edge.From + "/" + edge.Condition
								if !seen[key] {
									seen[key] = true
									conditions = append(conditions, RuleCondition{edge.From, edge.Condition, nodes[edge.From].Location})
								}
							}
						}
					}
					sort.Slice(conditions, func(i, j int) bool {
						return conditions[i].Node+conditions[i].Condition < conditions[j].Node+conditions[j].Condition
					})
					r.Candidates = append(r.Candidates, RuleCandidate{id, rule, "automatic", def.ID, def.Location, sink.ID, sinkLocation, path, conditions, "lexical_may_reach", "syntax_only_unverified", "not_proven"})
					continue
				}
				// Only RHS feeding the selected definition; never traverse unrelated
				// command arguments or a different parallel assignment's RHS.
				expressions := []ast.Expr{}
				switch stmt := a.astNodes[def.Node].(type) {
				case *ast.AssignStmt:
					for i, lhs := range stmt.Lhs {
						if a.loc(lhs) == def.Location {
							if len(stmt.Rhs) == len(stmt.Lhs) {
								expressions = append(expressions, stmt.Rhs[i])
								if stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE {
									expressions = append(expressions, lhs)
								}
							} else {
								expressions = append(expressions, stmt.Rhs...)
							}
						}
					}
				case *ast.ValueSpec:
					for i, lhs := range stmt.Names {
						if a.loc(lhs) == def.Location {
							if len(stmt.Values) == len(stmt.Names) {
								expressions = append(expressions, stmt.Values[i])
							} else {
								expressions = append(expressions, stmt.Values...)
							}
						}
					}
				}
				for _, use := range uses[def.Node] {
					if !step() {
						return
					}
					for _, expr := range expressions {
						loc := a.loc(expr)
						if use.Location.StartByte >= loc.StartByte && use.Location.EndByte <= loc.EndByte {
							pending = append(pending, work{use, path})
							break
						}
					}
				}
			}
		}
	}
	for _, n := range a.result.Nodes {
		if !step() {
			return
		}
		node := a.astNodes[n.ID]
		if node == nil {
			continue
		}
		ast.Inspect(node, func(item ast.Node) bool {
			if r.Status == "incomplete" {
				return false
			}
			if !step() {
				return false
			}
			if _, ok := item.(*ast.FuncLit); ok {
				return false
			}
			var args []ast.Expr
			rule := ""
			switch v := item.(type) {
			case *ast.CallExpr:
				owner, name := selector(v)
				index := 0
				if name == "CommandContext" {
					index = 1
				}
				if owner == "exec" && (name == "Command" || name == "CommandContext") && len(v.Args) > index {
					args = []ast.Expr{v.Args[index]}
					rule = "parameter-command-name"
				}
			case *ast.ReturnStmt:
				args = v.Results
				rule = "discarded-conversion-error"
			}
			for _, arg := range args {
				if _, ok := arg.(*ast.Ident); !ok {
					continue
				}
				location := a.loc(arg)
				for _, use := range uses[n.ID] {
					if use.Location == location {
						emit(rule, use, location)
					}
				}
			}
			return true
		})
	}
	sort.Slice(r.Candidates, func(i, j int) bool { return r.Candidates[i].ID < r.Candidates[j].ID })
}

func newRuleResult(o RuleOptions) *RuleResult {
	return &RuleResult{PackID: "go-local-review", PackVersion: "1.0.0", PackSHA256: hashBytes([]byte(rulePackDefinition)), EngineVersion: "1", Origin: "automatic", Status: "matched_bounded", Limits: o, Candidates: []RuleCandidate{}, Limitations: []string{"empty_candidates_do_not_establish_safety", "syntax_targets_unresolved", "parameters_not_proven_attacker_controlled", "lexical_may_reach_not_path_feasibility", "sanitizers_and_opaque_call_effects_not_proven", "aliases_closures_fields_and_indirect_sinks_unsupported"}}
}
