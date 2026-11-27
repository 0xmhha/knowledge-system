package mcphandlers

import (
	"context"
	"errors"
	"fmt"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/execflow"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

// executionFlowTool is the base name of the execution flow tool.
const executionFlowTool = "execution_flow"

// executionFlowDescription says what execution_flow answers and how to read
// it; the rules are in package execflow.
const executionFlowDescription = "What happens, in execution order, starting from a function (direction=forward, default), or what leads to it (direction=backward), across function calls, goroutine starts and channel messages, with the locks held, in one call. " +
	"The answer is text: a summary of a few lines (the seed, how many steps were shown of those found, branch points, and what was not opened: depth, budget, recursion, outside the build, test code), then numbered steps, each \"<label> <file>:<line> <what it does> <marks> | <source line>\". " +
	"Forward: steps run in source-line order; an indented step runs inside the one above it (the callee's steps under a call, a goroutine's under its start, the code a message runs under its receive); 2a, 2b are alternatives (the implementations an interface call may run, the receives one send can wake, the cases of a select). " +
	"Backward: 0 is the seed; each numbered step leads to the step it is under: its callers, the function that starts a goroutine running it, and the sender whose message makes a call run. " +
	"Alternatives come in the build first, then production, then by graph rank, cut to branches with the count left out and the call that lists them. " +
	"Marks: ~aliasing, ~branches, ~interleaving say a fact is approximated and why; ~by type an implementation only its type allows; [holding L since N] a lock held there (the graph's must-hold fact at the step's site, marked held by a caller when a caller holds it; ~branches when read by line order for a step without a site). " +
	"Every printed name opens its node: pass a step's name as qname to continue; a step that was not opened ends with the exact call. " +
	"Each step is marked with what it sits in: {for N} repeats with a loop, {if N}/{else N}/{case N} a branch (the then and else of one if are alternatives), {after return N}/{after continue N}/{after break N}/{after goto N} runs unless that earlier return or jump on its path is taken, {defer}. A function literal passed to a function holds its code under it with what that function does with it: calls it before it returns (under the caller's locks), defers it, starts it in a goroutine, keeps it, or hands it on (without the caller's locks). Calls into sync, sync/atomic and time are said as what they do (waits for the WaitGroup, atomically loads, sleeps, arms a timer); other calls outside the build are left out and counted. A lock released in another function says where. " +
	"Without focus, field reads and writes are left out; focus=<field or variable> keeps only the steps that read it, write it or take a lock that guards it (backward: the steps that lead here and read or write it), and the steps that lead to them. via=<name> keeps only the paths through that node; from=<name> (backward) shows the paths from it to the seed, shortest first."

// flowDepth is the depth argument: 0 given means the seed's own steps only.
func flowDepth(req mcp.CallToolRequest) int {
	if v, ok := req.GetArguments()["depth"]; ok {
		if f, ok := v.(float64); ok && f == 0 {
			return execflow.OwnStepsOnly
		}
	}
	return int(req.GetFloat("depth", 0))
}

// RegisterExecutionFlow registers execution_flow, which answers what happens
// from a function, or what leads to it, as an ordered narrative of source
// lines (package execflow).
func RegisterExecutionFlow(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool(executionFlowTool,
		mcp.WithDescription(executionFlowDescription),
		mcp.WithString("qname", mcp.Required(), mcp.Description("the function, method or goroutine to start from, in any name form the other tools take")),
		mcp.WithString("direction", mcp.Description("forward (default): what happens from here; backward: what leads here")),
		mcp.WithString("follow", mcp.Description("links to follow, comma-separated: calls, messages, goroutines (default all)")),
		mcp.WithString("focus", mcp.Description("a field or variable name: keep only the steps that read, write or guard it and those that lead to them (forward)")),
		mcp.WithString("via", mcp.Description("keep only the paths through this function, method or goroutine (any name form); a cut branch point names the next ones to pick from")),
		mcp.WithString("from", mcp.Description("backward: the paths from this function to the seed, shortest first (default depth then 6)")),
		mcp.WithNumber("depth", mcp.Description(fmt.Sprintf("hops (calls, goroutine starts, messages) opened beyond the seed's own steps (default %d, at most %d; 0: the seed's own steps only)", execflow.DefaultDepth, execflow.MaxDepth))),
		mcp.WithNumber("budget", mcp.Description(fmt.Sprintf("steps shown, the nearest first (default %d, at most %d)", execflow.DefaultBudget, execflow.MaxBudget))),
		mcp.WithNumber("branches", mcp.Description(fmt.Sprintf("alternatives shown per branch point (default %d)", execflow.DefaultBranches))),
		mcp.WithString("tests", mcp.Description("exclude (default) or include test code (the files the build records as test code)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		follow, err := execflow.ParseLinks(req.GetString("follow", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		q := execflow.Query{
			Seed:      req.GetString("qname", ""),
			Direction: execflow.Direction(req.GetString("direction", "")),
			Follow:    follow,
			Focus:     req.GetString("focus", ""),
			Via:       req.GetString("via", ""),
			From:      req.GetString("from", ""),
			Depth:     flowDepth(req),
			Budget:    int(req.GetFloat("budget", 0)),
			Branches:  int(req.GetFloat("branches", 0)),
			Tests:     execflow.Tests(req.GetString("tests", "")),
			ToolName:  ToolName(executionFlowTool), CallersTool: ToolName("find_callers"),
			CalleesTool: ToolName("find_callees"), ReferencesTool: ToolName("find_references"),
		}
		text, err := execflow.Trace(reader, q)
		if errors.Is(err, execflow.ErrBadQuery) {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", executionFlowTool, err)
		}
		traceFrom(ctx).add(executionFlowTool, map[string]any{"qname": q.Seed, "direction": string(q.Direction), "bytes": len(text)})
		return mcp.NewToolResultText(text), nil
	})
}
