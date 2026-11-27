package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

// Reasons a registered tool is not probed.
const (
	reasonNoProbe = "no probe arguments are defined for it (add them to mcpProbeArgs)"
)

// maxSeedTries bounds how many candidate seeds a probe tries before it
// says the graph gives the tool no non-empty answer.
const maxSeedTries = 20

// anyAnswer, as a probe's answer key, takes every answer as real: the
// investigate tools answer in prose.
const anyAnswer = ""

// probeArgs is one tool's probe: the kinds of node its seed is taken from,
// whether the seed needs a canonical id, the answer key that must not be
// empty for the answer to count as real, and the arguments made from the
// seed.
type probeArgs struct {
	kinds       []types.NodeType
	canonicalID bool
	key         string
	args        func(seed types.Node) map[string]any
}

var (
	functionKinds  = []types.NodeType{types.NodeFunction, types.NodeMethod}
	valueKinds     = []types.NodeType{types.NodeField, types.NodeVariable}
	lockHolderKind = []types.NodeType{types.NodeMutex, types.NodeField, types.NodeFunction, types.NodeMethod}
)

func qnameArgs(key string) func(types.Node) map[string]any {
	return func(n types.Node) map[string]any { return map[string]any{key: n.QualifiedName} }
}

func qnameDepthArgs(key string, depth float64) func(types.Node) map[string]any {
	return func(n types.Node) map[string]any { return map[string]any{key: n.QualifiedName, "depth": depth} }
}

// mcpProbeArgs are the probes by base tool name. The seed of each is
// picked by one rule (seedRuleFor): the highest-ranked node of its kinds
// whose answer has a non-empty key. The tools probed come from the
// registry; a registered tool missing here is reported as not probed.
var mcpProbeArgs = map[string]probeArgs{
	"find_symbol": {functionKinds, false, "nodes", qnameArgs("name")},
	"search_text": {functionKinds, false, "nodes", func(n types.Node) map[string]any { return map[string]any{"query": n.Name, "top_k": 10.0} }},
	"search_code": {functionKinds, false, "hits", func(n types.Node) map[string]any { return map[string]any{"pattern": regexp.QuoteMeta(n.Name)} }},
	"search_test_code": {functionKinds, false, "hits", func(n types.Node) map[string]any {
		return map[string]any{"pattern": regexp.QuoteMeta(n.Name)}
	}},
	"get_context_for_task": {functionKinds, false, "bodies", func(n types.Node) map[string]any {
		return map[string]any{"task_description": "what calls " + n.Name + " and what it does", "budget_tokens": 4000.0, "max_bodies": 3.0}
	}},
	"evidence_for_intent": {functionKinds, false, "hits", func(n types.Node) map[string]any {
		return map[string]any{"intent": n.Name, "k": 5.0, "budget_tokens": 4000.0}
	}},
	"investigate":        {functionKinds, false, anyAnswer, func(n types.Node) map[string]any { return map[string]any{"words": n.Name} }},
	"investigate_open":   {functionKinds, false, anyAnswer, func(n types.Node) map[string]any { return map[string]any{"open": "card:" + n.QualifiedName} }},
	"find_callers":       {functionKinds, false, "edges", qnameDepthArgs("qname", 1)},
	"find_callees":       {functionKinds, false, "edges", qnameDepthArgs("qname", 1)},
	"get_subgraph":       {functionKinds, false, "edges", qnameDepthArgs("seed_qname", 2)},
	"impact_of_change":   {functionKinds, false, "edges", qnameDepthArgs("seed_qname", 2)},
	"change_history":     {functionKinds, false, "prs", qnameArgs("qname")},
	"find_references":    {valueKinds, false, "users", qnameArgs("qname")},
	"concurrency_impact": {lockHolderKind, false, "edges", qnameArgs("seed_qname")},
	"value_dependencies": {functionKinds, true, "analysis", func(n types.Node) map[string]any { return map[string]any{"canonical_id": n.CanonicalID} }},
	"rule_candidates": {functionKinds, true, "analysis", func(n types.Node) map[string]any {
		return map[string]any{"rule_pack": valueflow.RulePackID, "function": map[string]any{"canonical_id": n.CanonicalID}}
	}},
}

// seedRuleFor states how a probe's seed is picked.
func seedRuleFor(pa probeArgs) string {
	kinds := make([]string, len(pa.kinds))
	for i, k := range pa.kinds {
		kinds[i] = string(k)
	}
	what := "an answer"
	if pa.key != anyAnswer {
		what = fmt.Sprintf("a non-empty %q", pa.key)
	}
	id := ""
	if pa.canonicalID {
		id = " with a canonical id"
	}
	return fmt.Sprintf("the highest-ranked %s%s among the %d top-PageRank nodes whose answer has %s (at most %d tried)",
		strings.Join(kinds, " or "), id, seedCandidates, what, maxSeedTries)
}

// probeRecord says how one probe was made, for the output.
type probeRecord struct {
	Tool  string         `json:"tool"`
	Rule  string         `json:"rule"`
	Seed  string         `json:"seed"`
	Tried int            `json:"tried"`
	Args  map[string]any `json:"args"`
}

// emptyByGraph is a probe the graph cannot give a non-empty answer, so it
// is not timed.
type emptyByGraph struct {
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

// probeSeed is how the probes of a run were made.
type probeSeed struct {
	Probes       []probeRecord  `json:"probes"`
	EmptyByGraph []emptyByGraph `json:"empty_by_graph,omitempty"`
}

// probeCall runs one tool call in process, to try a candidate seed.
type probeCall func(tool string, args map[string]any) (*mcpgo.CallToolResult, error)

// handlerCall is a probeCall over in-process handlers.
func handlerCall(handlers map[string]mcpserver.ToolHandlerFunc) probeCall {
	return func(tool string, args map[string]any) (*mcpgo.CallToolResult, error) {
		h, ok := handlers[tool]
		if !ok {
			return nil, fmt.Errorf("tool %s is not registered", tool)
		}
		req := mcpgo.CallToolRequest{}
		req.Params.Name = tool
		req.Params.Arguments = args
		return h(context.Background(), req)
	}
}

// mcpProbesFor returns a probe for each registered tool (names, the
// client-visible names in run order), the tools it does not probe with the
// reason, and how each probe was made. candidates are the nodes seeds are
// taken from, best first; call tries a candidate. A tool for which no
// candidate gives a non-empty answer is listed under empty_by_graph and
// not probed.
func mcpProbesFor(names []string, candidates []types.Node, call probeCall) ([]mcpProbe, []notProbed, *probeSeed) {
	base := make(map[string]string, len(mcpProbeArgs))
	for b := range mcpProbeArgs {
		base[mcphandlers.ToolName(b)] = b
	}
	var probes []mcpProbe
	var skipped []notProbed
	made := &probeSeed{}
	for _, name := range names {
		pa, ok := mcpProbeArgs[base[name]]
		if !ok {
			skipped = append(skipped, notProbed{name, reasonNoProbe})
			continue
		}
		rule := seedRuleFor(pa)
		seed, args, tried, found := pickProbeSeed(name, pa, candidates, call)
		if !found {
			made.EmptyByGraph = append(made.EmptyByGraph, emptyByGraph{Tool: name,
				Reason: fmt.Sprintf("no seed gives a real answer: %s found none (%d tried)", rule, tried)})
			continue
		}
		probes = append(probes, mcpProbe{Name: name, Args: args})
		made.Probes = append(made.Probes, probeRecord{Tool: name, Rule: rule, Seed: seed.QualifiedName, Tried: tried, Args: args})
	}
	return probes, skipped, made
}

// pickProbeSeed tries candidates of the probe's kinds in order and returns
// the first whose answer is real (answerIsReal).
func pickProbeSeed(tool string, pa probeArgs, candidates []types.Node, call probeCall) (types.Node, map[string]any, int, bool) {
	tried := 0
	for _, n := range candidates {
		if tried >= maxSeedTries {
			break
		}
		if !hasKind(pa.kinds, n.Type) || n.QualifiedName == "" || (pa.canonicalID && n.CanonicalID == "") {
			continue
		}
		tried++
		args := pa.args(n)
		res, err := call(tool, args)
		if err == nil && answerIsReal(res, pa.key) {
			return n, args, tried, true
		}
	}
	return types.Node{}, nil, tried, false
}

func hasKind(kinds []types.NodeType, t types.NodeType) bool {
	for _, k := range kinds {
		if k == t {
			return true
		}
	}
	return false
}

// answerIsReal reports whether res answers: not an error, and under key a
// non-empty value (any answer when key is anyAnswer).
func answerIsReal(res *mcpgo.CallToolResult, key string) bool {
	if res == nil || res.IsError {
		return false
	}
	if key == anyAnswer {
		return true
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	switch s := strings.TrimSpace(string(v)); s {
	case "", "null", "[]", "{}", `""`, "0", "false":
		return false
	}
	return true
}

// reportProbeSeed prints how the probes were made.
func reportProbeSeed(w io.Writer, s *probeSeed) {
	if s == nil {
		return
	}
	for _, p := range s.Probes {
		_, _ = fmt.Fprintf(w, "  %-26s seed %s (%d tried)\n", p.Tool, p.Seed, p.Tried)
	}
	for _, e := range s.EmptyByGraph {
		_, _ = fmt.Fprintf(w, "  %-26s EMPTY BY GRAPH (%s)\n", e.Tool, e.Reason)
	}
}
