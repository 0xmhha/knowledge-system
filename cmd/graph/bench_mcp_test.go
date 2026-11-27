package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/internal/graph/mcp"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestMCPProbes_EveryRegisteredTool: every registered tool has a probe
// rule; each probe's seed is the first candidate of the tool's kinds whose
// answer is real; a tool no candidate answers is listed under
// empty_by_graph and not timed. With one Function seed for every tool, 4
// of 17 probes timed empty answers (find_references on a Function among
// them).
func TestMCPProbes_EveryRegisteredTool(t *testing.T) {
	_, handlers := mcp.NewBenchHandlers(nil)
	names := mcp.SortedToolNames(handlers)
	if len(names) < 17 {
		t.Fatalf("registered tools = %d (%v), want 17 or more", len(names), names)
	}
	fn1 := types.Node{Type: types.NodeFunction, Name: "Empty", QualifiedName: "p.Empty", CanonicalID: "ex/p.Empty"}
	fn2 := types.Node{Type: types.NodeFunction, Name: "Full", QualifiedName: "p.Full", CanonicalID: "ex/p.Full"}
	field := types.Node{Type: types.NodeField, Name: "f", QualifiedName: "p.T.f"}
	candidates := []types.Node{fn1, field, fn2}
	// Every answer key is filled for p.Full and p.T.f, except history
	// (change_history), which no seed has.
	call := func(tool string, args map[string]any) (*mcpgo.CallToolResult, error) {
		raw, _ := json.Marshal(args)
		full := strings.Contains(string(raw), "Full") || strings.Contains(string(raw), "p.T.f")
		out := map[string]any{"nodes": []any{}, "hits": []any{}, "bodies": []any{}, "edges": []any{}, "users": []any{}, "prs": []any{}, "analysis": map[string]any{}}
		if full {
			for _, k := range []string{"nodes", "hits", "bodies", "edges", "users"} {
				out[k] = []any{1}
			}
			out["analysis"] = map[string]any{"x": 1}
		}
		return &mcpgo.CallToolResult{StructuredContent: out}, nil
	}
	probes, skipped, made := mcpProbesFor(names, candidates, call)
	if len(skipped) != 0 {
		t.Errorf("not probed: %+v", skipped)
	}
	seeds := map[string]string{}
	for _, r := range made.Probes {
		seeds[r.Tool] = r.Seed
		if r.Rule == "" {
			t.Errorf("%s: no rule stated", r.Tool)
		}
	}
	for tool, want := range map[string]string{"find_references": "p.T.f", "find_callers": "p.Full", "investigate": "p.Empty"} {
		if seeds[tool] != want {
			t.Errorf("%s seed = %q, want %q", tool, seeds[tool], want)
		}
	}
	if len(made.EmptyByGraph) != 1 || made.EmptyByGraph[0].Tool != "change_history" || made.EmptyByGraph[0].Reason == "" {
		t.Errorf("empty_by_graph = %+v, want change_history with a reason", made.EmptyByGraph)
	}
	if len(probes)+len(made.EmptyByGraph) != len(names) {
		t.Errorf("%d probes + %d empty for %d tools", len(probes), len(made.EmptyByGraph), len(names))
	}
}

// TestRunStdioBench_ChildFailureIsNamed: a child that exits before the
// handshake (here: it cannot write its log) is reported with what it
// wrote to standard error. The bench reported only that the handshake
// read failed.
func TestRunStdioBench_ChildFailureIsNamed(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-ckg")
	const msg = "init logger: open logs/mcp.log: permission denied"
	script := "#!/bin/sh\necho '" + msg + "' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := runStdioBench(bin, dir, 1, func([]string) ([]mcpProbe, []notProbed, *probeSeed) { return nil, nil, nil })
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Fatalf("err = %v, want it to carry %q", err, msg)
	}
}
