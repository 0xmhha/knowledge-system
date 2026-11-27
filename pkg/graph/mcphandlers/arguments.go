package mcphandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
)

// arguments.go checks every call against the arguments its tool declares:
// a missing required argument and an unknown argument are an error result
// that names them and lists the arguments the tool takes. Without it a
// call such as find_symbol {} or find_symbol {"qname": "x"} answered as if
// nothing matched, and an agent could not tell "no such symbol" from
// "called it wrong". The check reads each tool's declared input schema, so
// a new tool or argument needs no code here.

// seedGroup is a set of arguments of which a call must give one (exactly:
// one and only one; else at least one): a tool that takes one of several
// seeds.
type seedGroup struct {
	args    []string
	exactly bool
}

// seedGroups are the seed groups of the tools that have one, by base tool
// name. A schema cannot say "one of", so they are declared here, the one
// place the argument check reads, and each tool's description says it.
var seedGroups = map[string]seedGroup{
	"impact_of_change":    {args: []string{"seed_qname", "seed_file"}},
	"concurrency_impact":  {args: []string{"seed_qname"}},
	"evidence_for_intent": {args: []string{"intent", "issue_id"}},
	"value_dependencies":  {args: []string{"node_id", "canonical_id"}, exactly: true},
}

// requireDeclaredArguments wraps the handler of every tool registered on s
// with the argument check, says the tool's seed group in its description,
// and turns an error of the handler into an error result: a model gets
// the message, never a protocol error.
func requireDeclaredArguments(s *server.MCPServer) {
	for _, st := range s.ListTools() {
		tool, handler := st.Tool, st.Handler
		props, required := declaredArguments(tool)
		accepted := map[string]bool{}
		for _, p := range props {
			accepted[p] = true
		}
		tool = describeParams(tool)
		group, grouped := seedGroups[baseToolName(tool.Name)]
		if grouped {
			tool.Description += " " + group.rule() + "."
		}
		s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			msg := argumentProblems(tool.Name, req.GetArguments(), props, required, accepted)
			if msg == "" && grouped {
				msg = group.problem(tool.Name, req.GetArguments())
			}
			if msg != "" {
				return mcp.NewToolResultError(msg), nil
			}
			res, err := handler(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(tool.Name + ": " + err.Error()), nil
			}
			return res, nil
		})
	}
}

// rule says the group in words.
func (g seedGroup) rule() string {
	switch {
	case len(g.args) == 1:
		return "Takes " + g.args[0] + " (required)"
	case g.exactly:
		return "Takes exactly one of " + strings.Join(g.args, ", ")
	}
	return "Takes at least one of " + strings.Join(g.args, ", ")
}

// problem says what is wrong with args for the group, or "".
func (g seedGroup) problem(tool string, args map[string]any) string {
	n := 0
	for _, a := range g.args {
		if v, ok := args[a]; ok && v != nil && v != "" {
			n++
		}
	}
	if n == 0 || g.exactly && n > 1 {
		return fmt.Sprintf("%s: %s; got %d of them", tool, strings.ToLower(g.rule()[:1])+g.rule()[1:], n)
	}
	return ""
}

// baseToolName is a tool's name without the namespace prefix ToolName adds.
func baseToolName(name string) string {
	for base := range seedGroups {
		if ToolName(base) == name {
			return base
		}
	}
	for base := range toolParams {
		if ToolName(base) == name {
			return base
		}
	}
	return name
}

// declaredArguments returns the argument names a tool declares, sorted, and
// the required ones, from its input schema (or its raw schema).
func declaredArguments(tool mcp.Tool) ([]string, []string) {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	raw := tool.RawInputSchema
	if len(raw) == 0 {
		b, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, nil
		}
		raw = b
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, nil
	}
	props := make([]string, 0, len(schema.Properties))
	for p := range schema.Properties {
		props = append(props, p)
	}
	sort.Strings(props)
	return props, schema.Required
}

// argumentProblems says what is wrong with args, or "".
func argumentProblems(tool string, args map[string]any, props, required []string, accepted map[string]bool) string {
	var problems []string
	for _, r := range required {
		if _, ok := args[r]; !ok {
			problems = append(problems, fmt.Sprintf("missing required argument %q", r))
		}
	}
	unknown := make([]string, 0)
	for k := range args {
		if !accepted[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		problems = append(problems, fmt.Sprintf("unknown argument %q", k))
	}
	if len(problems) == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %s; the arguments it takes are %s (required: %s)", tool, strings.Join(problems, "; "),
		strings.Join(props, ", "), strings.Join(orNone(required), ", "))
}

func orNone(s []string) []string {
	if len(s) == 0 {
		return []string{"none"}
	}
	return s
}
