package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
)

// Result is the outcome of running one Fixture against a Reader.
type Result struct {
	Fixture Fixture
	Got     []string // unique qualified_names returned by the probe
	Score   Score
	// PassRecall / PassPrecision report whether the per-fixture
	// thresholds were met. Aggregator combines them into the overall
	// gate result.
	PassRecall    bool
	PassPrecision bool
}

// Pass returns true when both threshold gates passed (a missing gate
// passes by default because the zero value is 0.0).
func (r Result) Pass() bool {
	return r.PassRecall && r.PassPrecision
}

// Run executes a single fixture against the given Reader and produces
// a scored Result. A dispatch error (unknown tool, missing arg) is
// returned separately from a low-score Result — the caller distinguishes
// "ran and scored badly" from "could not run at all".
func Run(reader persist.StoreReader, f Fixture) (Result, error) {
	return runOn(newToolServer(reader), f)
}

// RunAll runs every fixture sequentially and returns one Result per
// fixture in input order. Sequential rather than parallel because the
// per-probe cost is microseconds on the synthetic fixture — parallelism
// would add ordering noise to the JSON output for no real speedup.
func RunAll(reader persist.StoreReader, fixtures []Fixture) ([]Result, error) {
	s := newToolServer(reader)
	out := make([]Result, 0, len(fixtures))
	for _, f := range fixtures {
		r, err := runOn(s, f)
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

func runOn(s *server.MCPServer, f Fixture) (Result, error) {
	got, err := dispatchProbe(s, f.Probe)
	if err != nil {
		return Result{}, fmt.Errorf("fixture %s: %w", f.ID, err)
	}
	score := ComputeScore(f.Expected.Symbols, got)
	return Result{
		Fixture:       f,
		Got:           got,
		Score:         score,
		PassRecall:    score.Recall >= f.Scoring.RecallMin,
		PassPrecision: score.Precision >= f.Scoring.PrecisionMin,
	}, nil
}

// newToolServer registers the MCP tools over reader, so a probe runs the
// tool's own handler with the tool's own defaults: the same code path an
// MCP client gets.
func newToolServer(reader persist.StoreReader) *server.MCPServer {
	s := server.NewMCPServer("eval-retrieval", "0")
	mcphandlers.RegisterAll(s, reader)
	return s
}

// probeTools are the tools a fixture may probe; each answers with a
// "nodes" list. seedArg names the argument whose node a traversal answer
// includes and the score leaves out (it is not its own caller or callee).
var probeTools = map[string]string{
	"find_callers": "qname",
	"find_callees": "qname",
	"find_symbol":  "",
	"search_text":  "",
}

// dispatchProbe calls the probe's MCP tool with the fixture's arguments
// and returns the unique qnames of the nodes it answers.
//
// Adding a new tool: add it to probeTools (its answer must carry a nodes
// list) and write a fixture or two under eval/retrieval/.
func dispatchProbe(s *server.MCPServer, p Probe) ([]string, error) {
	seedArg, ok := probeTools[p.Tool]
	if !ok {
		names := slices.Sorted(maps.Keys(probeTools))
		return nil, fmt.Errorf("unknown probe tool %q (supported: %s)", p.Tool, strings.Join(names, ", "))
	}
	tool := s.GetTool(mcphandlers.ToolName(p.Tool))
	if tool == nil {
		return nil, fmt.Errorf("probe tool %q is not registered", p.Tool)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = tool.Tool.Name
	req.Params.Arguments = jsonArgs(p.Args)
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.Tool, err)
	}
	if res.IsError {
		return nil, fmt.Errorf("%s answered with an error: %v", p.Tool, res.Content)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("%s: read the answer: %w", p.Tool, err)
	}
	var answer struct {
		Nodes []struct {
			QName string `json:"qname"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return nil, fmt.Errorf("%s: read the answer: %w", p.Tool, err)
	}
	exclude := ""
	if seedArg != "" {
		exclude, _ = p.Args[seedArg].(string)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(answer.Nodes))
	for _, n := range answer.Nodes {
		// Scored by the name the tool prints, the one that opens exactly
		// one node: a qualified name several declarations share is
		// printed in the site form "<qname>@<file>:<line>", and a fixture
		// names the declaration it means in that form.
		q := n.QName
		if q == "" || q == exclude || seen[q] {
			continue
		}
		seen[q] = true
		out = append(out, q)
	}
	return out, nil
}

// jsonArgs gives the fixture's YAML arguments the types a JSON client
// sends: numbers as float64.
func jsonArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for k, v := range args {
		switch n := v.(type) {
		case int:
			out[k] = float64(n)
		case int64:
			out[k] = float64(n)
		default:
			out[k] = v
		}
	}
	return out
}
