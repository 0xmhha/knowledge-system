// cmd/ckg/bench_mcp.go — `ckg bench-mcp` measures p50/p95/p99
// latency of every registered MCP tool against a real graph.db.
// Counterpart to bench-server: where bench-server exercises the
// HTTP layer, bench-mcp exercises the in-process tool handlers
// directly (no subprocess spawn, no JSON-RPC framing).
//
// Why in-process rather than spawning `ckg mcp`: we want to attribute
// latency to the graph layer (store reads, BM25 ranking, subgraph
// walk) rather than to the stdio + JSON-RPC framing on top. If the
// in-process numbers are dominated by the graph layer, the stdio
// hypothesis ("framing dominates") is wrong; if they're trivial,
// then a follow-up bench-mcp-stdio would be worth building. This
// commit takes the cheaper measurement first.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/mcp"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// mcpProbe is one tool to bench. Args mirror what an MCP client
// would send via tools/call so the handler exercises every code
// path it sees in production (param parsing, schema defaulting,
// store reads, response serialization).
//
// Name is the *label* used in output (and in the benchResult.Endpoint
// field). Tool is the *handler key* — the MCP tool to invoke. They
// usually match; they diverge only when the same tool needs to be
// benched multiple times with different arguments (e.g. depth-sweep)
// and the report needs distinct rows per variant.
type mcpProbe struct {
	Name string
	Tool string // empty → defaults to Name (backwards-compatible)
	Args map[string]any
}

// handlerKey returns the tool name to look up in the handler map.
func (p mcpProbe) handlerKey() string {
	if p.Tool != "" {
		return p.Tool
	}
	return p.Name
}

// depthSweepProbes drives the four traversal tools at depth=1 AND
// depth=2 so a single bench run produces side-by-side latency data.
// Used to answer CKG-5: is depth=2 acceptable as the default for
// find_callers / find_callees (currently 1), or should the consumer
// raise it per-query?
//
// Returns nil when seedQname is empty (no Function in the graph).
// Each row in the resulting benchResult carries a `_d1` / `_d2`
// suffix so the JSON output can be diffed without name collisions.
func depthSweepProbes(seedQname string) []mcpProbe {
	if seedQname == "" {
		return nil
	}
	return []mcpProbe{
		{Name: "find_callers_d1", Tool: "find_callers", Args: map[string]any{"qname": seedQname, "depth": 1.0}},
		{Name: "find_callers_d2", Tool: "find_callers", Args: map[string]any{"qname": seedQname, "depth": 2.0}},
		{Name: "find_callees_d1", Tool: "find_callees", Args: map[string]any{"qname": seedQname, "depth": 1.0}},
		{Name: "find_callees_d2", Tool: "find_callees", Args: map[string]any{"qname": seedQname, "depth": 2.0}},
		{Name: "get_subgraph_d1", Tool: "get_subgraph", Args: map[string]any{"seed_qname": seedQname, "depth": 1.0}},
		{Name: "get_subgraph_d2", Tool: "get_subgraph", Args: map[string]any{"seed_qname": seedQname, "depth": 2.0}},
		{Name: "impact_of_change_d1", Tool: "impact_of_change", Args: map[string]any{"seed_qname": seedQname, "depth": 1.0}},
		{Name: "impact_of_change_d2", Tool: "impact_of_change", Args: map[string]any{"seed_qname": seedQname, "depth": 2.0}},
	}
}

// notProbed is a registered tool a bench run did not call, and why.
type notProbed struct {
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

// reportNotProbed prints the tools a run did not call.
func reportNotProbed(w io.Writer, skipped []notProbed) {
	for _, s := range skipped {
		_, _ = fmt.Fprintf(w, "  %-26s NOT PROBED (%s)\n", s.Tool, s.Reason)
	}
}

func newBenchMCPCmd() *cobra.Command {
	var (
		graph       string
		iterations  int
		concurrency int
		output      string
		depthSweep  bool
	)
	cmd := &cobra.Command{
		Use:   "bench-mcp",
		Short: "Measure in-process MCP tool p50/p95/p99 latency against a graph.db",
		Long: `Build a fresh MCP server in-process, then drive every tool it
registers with a parallel load profile. Reports the same JSON
shape as bench-server so the two numbers can be compared
side-by-side: bench-server captures HTTP latency, bench-mcp captures
the underlying graph layer without stdio + JSON-RPC framing.

The tools come from the server's registry. Each probe's seed is picked
from the graph by one rule per tool: the highest-ranked node of the kinds
the tool asks about (Functions for the traversal and search tools, Fields
and Variables for find_references, Mutexes, Fields and Functions for
concurrency_impact) whose answer is real, that is whose answer key is
not empty; at most 20 candidates are tried. The output (probe_seed) gives
each probe's rule, seed and arguments. A tool for which the graph gives
no real answer (change_history on a graph built without history) is
listed under empty_by_graph with the reason and is not timed; a tool
without probe arguments is listed under not_probed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if iterations <= 0 || iterations > 10000 {
				return fmt.Errorf("--iterations must be in [1, 10000], got %d", iterations)
			}
			if concurrency <= 0 || concurrency > 64 {
				return fmt.Errorf("--concurrency must be in [1, 64], got %d", concurrency)
			}
			db := filepath.Join(graph, "graph.db")
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			seed, _ := pickFunctionSeed(store)
			_, handlers := mcp.NewBenchHandlers(store)
			candidates, err := store.TopNodes("pagerank", seedCandidates)
			if err != nil {
				return fmt.Errorf("rank seed candidates: %w", err)
			}
			var probes []mcpProbe
			var skipped []notProbed
			var made *probeSeed
			if depthSweep {
				probes = depthSweepProbes(seed.QualifiedName)
				if len(probes) == 0 {
					return fmt.Errorf("--depth-sweep requested but no Function seed found in graph")
				}
			} else {
				probes, skipped, made = mcpProbesFor(mcp.SortedToolNames(handlers), candidates, handlerCall(handlers))
			}
			reportProbeSeed(os.Stderr, made)
			reportNotProbed(os.Stderr, skipped)

			_, _ = fmt.Fprintf(os.Stderr, "bench-mcp: graph=%s iters=%d concurrency=%d probes=%d\n",
				graph, iterations, concurrency, len(probes))

			results := make([]benchResult, 0, len(probes))
			for _, p := range probes {
				h, ok := handlers[p.handlerKey()]
				if !ok {
					_, _ = fmt.Fprintf(os.Stderr, "  %-26s SKIP (handler not registered)\n", p.Name)
					continue
				}
				r := runMCPBench(h, p, iterations, concurrency)
				results = append(results, r)
				_, _ = fmt.Fprintf(os.Stderr, "  %-26s p50=%6.2f p95=%6.2f p99=%6.2f mean=%6.2f errors=%d result_bytes=%d\n",
					r.Endpoint, r.P50Ms, r.P95Ms, r.P99Ms, r.MeanMs, r.ErrorCount, r.ResultBytes)
			}

			man, _ := store.GetManifest()
			out := benchOutput{
				GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
				GraphPath:      graph,
				BuildTimestamp: man.BuildTimestamp,
				SrcCommit:      man.SrcCommit,
				Iterations:     iterations,
				Concurrency:    concurrency,
				Results:        results,
				NotProbed:      skipped,
				ProbeSeed:      made,
			}
			payload, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal output: %w", err)
			}
			if output == "-" || output == "" {
				_, _ = os.Stdout.Write(payload)
				_, _ = os.Stdout.Write([]byte{'\n'})
				return nil
			}
			if err := os.WriteFile(output, append(payload, '\n'), 0o644); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			_, _ = fmt.Fprintf(os.Stderr, "bench-mcp: wrote %s\n", output)
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory containing graph.db (required)")
	cmd.Flags().IntVar(&iterations, "iterations", 100,
		"requests per probe per worker (total = iterations * concurrency)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4,
		"parallel workers driving each probe (≤ 64)")
	cmd.Flags().StringVar(&output, "output", "-",
		"path for JSON output ('-' or '' for stdout)")
	cmd.Flags().BoolVar(&depthSweep, "depth-sweep", false,
		"replace the default probes with a depth=1 vs depth=2 sweep over the four traversal tools (CKG-5)")
	_ = cmd.MarkFlagRequired("graph")
	return cmd
}

// runMCPBench is the MCP analogue of runBench: drive one handler
// `iterations * concurrency` total times, collect latencies, return
// a benchResult. Errors are counted; non-200 has no analogue here
// (the handler returns either a *CallToolResult or an error), so we
// treat any error as a "non-200" for status_200_pct purposes.
//
// A warmup call runs before the timed loop so cold-start costs (the
// evidence cache's first BM25 corpus build, for instance) don't
// pollute the measurement window. The warmup result is discarded.
func runMCPBench(handler mcpserver.ToolHandlerFunc, probe mcpProbe, iterations, concurrency int) benchResult {
	type sample struct {
		dur time.Duration
		err error
	}
	resultBytes := 0
	{
		// Warmup: prime any per-tool caches so the bench measures the
		// steady-state path the way bench-server does (which benefits
		// from prewarmTicketIndex / prewarmEdgeCounts at boot).
		warmReq := mcpgo.CallToolRequest{}
		warmReq.Params.Name = probe.Name
		warmReq.Params.Arguments = probe.Args
		if res, err := handler(context.Background(), warmReq); err == nil && res != nil {
			if b, err := json.Marshal(res); err == nil {
				resultBytes = len(b)
			}
		}
	}
	samples := make(chan sample, iterations*concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for w := 0; w < concurrency; w++ {
		go func() {
			defer wg.Done()
			ctx := context.Background()
			req := mcpgo.CallToolRequest{}
			req.Params.Name = probe.Name
			req.Params.Arguments = probe.Args
			for i := 0; i < iterations; i++ {
				start := time.Now()
				_, err := handler(ctx, req)
				samples <- sample{dur: time.Since(start), err: err}
			}
		}()
	}
	wg.Wait()
	close(samples)

	durs := make([]float64, 0, iterations*concurrency)
	errCount := 0
	ok200 := 0
	for s := range samples {
		durs = append(durs, float64(s.dur.Microseconds())/1000.0)
		if s.err != nil {
			errCount++
			continue
		}
		ok200++
	}
	r := benchResult{
		Endpoint:    probe.Name,
		N:           len(durs),
		Concurrency: concurrency,
		ErrorCount:  errCount,
		ResultBytes: resultBytes,
	}
	if len(durs) == 0 {
		return r
	}
	r.Status200Pct = 100.0 * float64(ok200) / float64(len(durs))
	r.MinMs = minF(durs)
	r.MaxMs = maxF(durs)
	r.MeanMs = meanF(durs)
	r.P50Ms = percentile(durs, 50)
	r.P95Ms = percentile(durs, 95)
	r.P99Ms = percentile(durs, 99)
	return r
}
