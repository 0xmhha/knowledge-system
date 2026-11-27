package main

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// maxHelpLine is the longest line the observe and verify help texts wrap to.
const maxHelpLine = 110

// TestObserveHelp_SaysWhatLinesAndRuntimeChannelsMean: observe says that a
// deferred call's line is where it ran, and verify says that operations on
// runtime-made channels fall outside the build.
func TestObserveHelp_SaysWhatLinesAndRuntimeChannelsMean(t *testing.T) {
	obs := newObserveCmd().Long
	for _, want := range []string{"defer statement", "timer's C", "outside the build"} {
		if !strings.Contains(obs, want) {
			t.Errorf("observe help lacks %q", want)
		}
	}
	// verify points to observe for call lines instead of repeating them.
	long := newVerifyCmd().Long
	if strings.Contains(long, "defer statement") || !strings.Contains(long, "ckg observe --help") || !strings.Contains(long, "timer's C") {
		t.Errorf("verify help: %q", long)
	}
	for _, help := range []string{obs, long} {
		for _, line := range strings.Split(help, "\n") {
			if len(line) > maxHelpLine {
				t.Errorf("help line of %d characters: %q", len(line), line)
			}
		}
	}
}

// TestDisabledCommands_HelpPrints: --help of a disabled command prints its
// help, which says it is disabled, and succeeds; a run fails. --help
// exited 1 with the disabled message.
func TestDisabledCommands_HelpPrints(t *testing.T) {
	for _, mk := range []func() *cobra.Command{newWatchCmd, newBenchIndexCmd} {
		for _, h := range helpArgs[:2] {
			cmd := mk()
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetArgs([]string{h})
			if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), "Disabled.") {
				t.Errorf("%s %s: err %v, output %q", cmd.Use, h, err, out.String())
			}
		}
		cmd := mk()
		cmd.SetArgs([]string{"--graph", "x"})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err == nil {
			t.Errorf("%s: a run should fail", cmd.Use)
		}
	}
}

// TestBuiltCounts_ReadsTheDatabase: the build's report line counts the
// rows of graph.db, not the manifest's stats (which miss rows written
// after them, such as Policy nodes and governed_by edges).
func TestBuiltCounts_ReadsTheDatabase(t *testing.T) {
	m := exportManifest("committed")
	m.Stats = map[string]int{"nodes": 99, "edges": 99}
	dir := graphExportFixture(t, m, false)
	if n, e := builtCounts(m, dir, ""); n != 2 || e != 1 {
		t.Errorf("builtCounts = %d nodes, %d edges; want the 2 and 1 rows of graph.db", n, e)
	}
}

// TestBuildReport_CountsEveryEdge: the header counts every edge of the
// graph and says how many goroutine code edges the sections below leave
// out. It printed the count without those edges as the graph's edge count
// (2,924,689 of 2,926,247 on a real graph).
func TestBuildReport_CountsEveryEdge(t *testing.T) {
	nodes := []types.Node{
		{ID: "g", Type: types.NodeGoroutine, Name: "g", QualifiedName: "p.g"},
		{ID: "f", Type: types.NodeFunction, Name: "f", QualifiedName: "p.f"},
		{ID: "h", Type: types.NodeFunction, Name: "h", QualifiedName: "p.h"},
	}
	edges := []types.Edge{
		{Src: "f", Dst: "h", Type: types.EdgeCalls, Confidence: types.ConfExtracted},
		{Src: "g", Dst: "h", Type: types.EdgeCalls, DispatchKind: types.DispatchGoroutine, Confidence: types.ConfExtracted},
		{Src: "f", Dst: "h", Type: types.EdgeDispatchesTo, Confidence: types.ConfExtracted},
	}
	if !types.IsGoroutineCode(edges[1]) {
		t.Fatal("fixture: the second edge should be goroutine code")
	}
	report := buildReport(persist.Manifest{}, nodes, edges, nil, 5, func(n types.Node) string { return n.QualifiedName })
	want := "- **Nodes**: 3 • **Edges**: 3 (rows of graph.db; the sections below leave out 1 goroutine code edges"
	if !strings.Contains(report, want) {
		t.Errorf("report header lacks %q:\n%s", want, report[:min(len(report), 600)])
	}
	// Every edge the axis section is given is in an axis or in the
	// outside line: 151,313 edges were in neither on a real graph.
	if !strings.Contains(report, "- **Outside the six axes** — 1 edges: dispatches_to=1") {
		t.Errorf("report does not count the edges outside the axes:\n%s", report[:min(len(report), 900)])
	}
}
