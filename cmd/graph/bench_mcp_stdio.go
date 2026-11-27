// cmd/ckg/bench_mcp_stdio.go — `ckg bench-mcp-stdio` measures MCP
// tool latency through a real `ckg mcp` subprocess. Counterpart to
// bench-mcp's in-process measurement: the difference between the two
// attributes the cost of stdio + JSON-RPC framing to the right side
// of the boundary.
//
// Concurrency is implicitly 1 — the stdio pipe carries one in-flight
// request at a time. Production MCP clients (Claude Desktop, etc.)
// also drive a single connection per server, so this matches the
// production load profile.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/mcp"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

func newBenchMCPStdioCmd() *cobra.Command {
	var (
		graph      string
		iterations int
		ckgBinary  string
		output     string
	)
	cmd := &cobra.Command{
		Use:   "bench-mcp-stdio",
		Short: "Measure MCP tool latency through a real ckg mcp subprocess (stdio + JSON-RPC)",
		Long: `Spawn ckg mcp as a child process, then drive every tool the child
lists (tools/list) through the stdio + JSON-RPC transport layer. Reports the same JSON shape as
bench-mcp so the two numbers can be compared side-by-side: bench-mcp
captures the in-process handler cost, bench-mcp-stdio captures the
production round-trip including the JSON-RPC framing + subprocess
pipe overhead.

Sequential (concurrency=1) by design — the stdio pipe carries one
in-flight request at a time, matching how production clients
(Claude Desktop, etc.) drive an MCP server.

Tools whose probe needs a seed the graph lacks are listed under
not_probed with the reason (see bench-mcp). When the child exits
before answering (for example because it cannot write its log under
the user cache directory), the error carries what the child wrote to
standard error.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if iterations <= 0 || iterations > 1000 {
				return fmt.Errorf("--iterations must be in [1, 1000], got %d", iterations)
			}
			if ckgBinary == "" {
				exe, err := os.Executable()
				if err != nil {
					return fmt.Errorf("os.Executable: %w", err)
				}
				ckgBinary = exe
			}
			// Pluck a Function seed up front so the four seed-dependent
			// probes (find_callers / find_callees / get_subgraph /
			// impact_of_change) get exercised when the graph supports
			// them.
			db := filepath.Join(graph, "graph.db")
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			// Seeds are picked in process, by the rules bench-mcp uses, and
			// sent to the child as arguments.
			candidates, err := store.TopNodes("pagerank", seedCandidates)
			if err != nil {
				_ = store.Close()
				return fmt.Errorf("rank seed candidates: %w", err)
			}
			_, handlers := mcp.NewBenchHandlers(store)
			pick := func(names []string) ([]mcpProbe, []notProbed, *probeSeed) {
				return mcpProbesFor(names, candidates, handlerCall(handlers))
			}

			_, _ = fmt.Fprintf(os.Stderr, "bench-mcp-stdio: graph=%s iters=%d binary=%s\n",
				graph, iterations, ckgBinary)

			results, skipped, made, err := runStdioBench(ckgBinary, graph, iterations, pick)
			_ = store.Close()
			if err != nil {
				return err
			}
			reportNotProbed(os.Stderr, skipped)
			for _, r := range results {
				_, _ = fmt.Fprintf(os.Stderr, "  %-26s p50=%6.2f p95=%6.2f p99=%6.2f mean=%6.2f errors=%d\n",
					r.Endpoint, r.P50Ms, r.P95Ms, r.P99Ms, r.MeanMs, r.ErrorCount)
			}

			// Re-open the store one more time just for the manifest
			// fingerprint in the JSON payload. Cheap, and avoids
			// keeping the store open during the subprocess phase
			// (where SQLite locks could collide with the child's
			// read-only handle on shared filesystems).
			store2, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("re-open graph for manifest: %w", err)
			}
			man, _ := store2.GetManifest()
			_ = store2.Close()
			out := benchOutput{
				GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
				GraphPath:      graph,
				BuildTimestamp: man.BuildTimestamp,
				SrcCommit:      man.SrcCommit,
				Iterations:     iterations,
				Concurrency:    1,
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
			_, _ = fmt.Fprintf(os.Stderr, "bench-mcp-stdio: wrote %s\n", output)
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().IntVar(&iterations, "iterations", 50, "requests per probe (single-threaded; total = iterations)")
	cmd.Flags().StringVar(&ckgBinary, "ckg-binary", "",
		"path to ckg binary (default: os.Executable() — the running binary)")
	cmd.Flags().StringVar(&output, "output", "-", "path for JSON output ('-' or '' for stdout)")
	_ = cmd.MarkFlagRequired("graph")
	return cmd
}

// childStderrTail bounds how much of the child's standard error an error
// message quotes.
const childStderrTail = 4096

// tailBuffer keeps the last childStderrTail bytes written to it.
type tailBuffer struct{ b []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if over := len(t.b) - childStderrTail; over > 0 {
		t.b = t.b[over:]
	}
	return len(p), nil
}

// runStdioBench is the heavy lifting: spawn one `ckg mcp` child,
// run the MCP handshake, list its tools, measure each tool's probe
// `iterations` times, shut the child down cleanly. A failure before the
// tools are listed aborts the run with what the child wrote to standard
// error; failures inside the per-probe loop count as errors but don't
// kill the bench (a single tool with bad args shouldn't take the
// whole pass down).
func runStdioBench(binary, graph string, iterations int, pick func(names []string) ([]mcpProbe, []notProbed, *probeSeed)) ([]benchResult, []notProbed, *probeSeed, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "mcp", "--graph", graph)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	// Server logs go straight through (failure triage) and their tail is
	// kept for the error of a child that exits early.
	var tail tailBuffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &tail)
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start ckg mcp: %w", err)
	}
	waited := false
	wait := func() error {
		_ = stdin.Close()
		if waited {
			return nil
		}
		waited = true
		return cmd.Wait()
	}
	// Cleanup: close stdin → server exits cleanly on EOF; cancel ctx
	// → kills if the server hangs; Wait reaps the process.
	defer func() {
		cancel()
		_ = wait()
	}()
	// childFailed wraps err with how the child ended and its stderr.
	childFailed := func(what string, err error) error {
		exit := wait()
		return fmt.Errorf("%s: %w (ckg mcp exited: %v; its stderr: %s)",
			what, err, exit, strings.TrimSpace(string(tail.b)))
	}

	br := bufio.NewReader(stdout)
	var nextID atomic.Int64

	// Handshake — required before any tools/call. mcp-go's server
	// won't dispatch tool requests until it sees notifications/initialized.
	if err := sendRPC(stdin, map[string]any{
		"jsonrpc": "2.0", "id": nextID.Add(1), "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "ckg-bench", "version": "0"},
		},
	}); err != nil {
		return nil, nil, nil, childFailed("send initialize", err)
	}
	if _, err := readRPC(br); err != nil {
		return nil, nil, nil, childFailed("read initialize response", err)
	}
	if err := sendRPC(stdin, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized",
	}); err != nil {
		return nil, nil, nil, childFailed("send initialized", err)
	}
	names, err := listStdioTools(stdin, br, nextID.Add(1))
	if err != nil {
		return nil, nil, nil, childFailed("list tools", err)
	}
	probes, skipped, made := pick(names)
	reportProbeSeed(os.Stderr, made)

	results := make([]benchResult, 0, len(probes))
	for _, p := range probes {
		// Warmup — same rationale as bench-mcp: prime per-tool caches
		// (the BM25 corpus build for evidence_for_intent) so the
		// timed loop sees steady-state cost. Result discarded.
		_ = sendRPC(stdin, map[string]any{
			"jsonrpc": "2.0", "id": nextID.Add(1), "method": "tools/call",
			"params": map[string]any{"name": p.Name, "arguments": p.Args},
		})
		_, _ = readRPC(br)

		durs := make([]float64, 0, iterations)
		errCount := 0
		ok200 := 0
		for i := 0; i < iterations; i++ {
			start := time.Now()
			if err := sendRPC(stdin, map[string]any{
				"jsonrpc": "2.0", "id": nextID.Add(1), "method": "tools/call",
				"params": map[string]any{"name": p.Name, "arguments": p.Args},
			}); err != nil {
				errCount++
				continue
			}
			resp, err := readRPC(br)
			dur := time.Since(start)
			if err != nil {
				errCount++
				continue
			}
			durs = append(durs, float64(dur.Microseconds())/1000.0)
			if _, hasResult := resp["result"]; hasResult {
				ok200++
			}
		}

		r := benchResult{
			Endpoint:    p.Name,
			N:           len(durs),
			Concurrency: 1,
			ErrorCount:  errCount,
		}
		if len(durs) > 0 {
			r.Status200Pct = 100.0 * float64(ok200) / float64(len(durs))
			r.MinMs = minF(durs)
			r.MaxMs = maxF(durs)
			r.MeanMs = meanF(durs)
			r.P50Ms = percentile(durs, 50)
			r.P95Ms = percentile(durs, 95)
			r.P99Ms = percentile(durs, 99)
		}
		results = append(results, r)
	}
	return results, skipped, made, nil
}

// listStdioTools asks the child for its tools and returns their names in
// sorted order.
func listStdioTools(w io.Writer, br *bufio.Reader, id int64) ([]string, error) {
	if err := sendRPC(w, map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/list"}); err != nil {
		return nil, err
	}
	resp, err := readRPC(br)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(resp["result"])
	if err != nil {
		return nil, fmt.Errorf("tools/list result: %w", err)
	}
	var list struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("tools/list result: %w", err)
	}
	names := make([]string, 0, len(list.Tools))
	for _, t := range list.Tools {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names, nil
}

// sendRPC writes one NDJSON request to the server's stdin. mcp-go's
// stdio transport uses newline-delimited JSON (not LSP-style
// Content-Length headers); see github.com/mark3labs/mcp-go/server/stdio.go.
func sendRPC(w io.Writer, m map[string]any) error {
	buf, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := w.Write(buf); err != nil {
		return err
	}
	if _, err := w.Write([]byte{'\n'}); err != nil {
		return err
	}
	return nil
}

// readRPC blocks for the next NDJSON response line. Returns the
// parsed envelope; the caller decides whether to treat
// `result`-vs-`error` as success.
func readRPC(br *bufio.Reader) (map[string]any, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return nil, fmt.Errorf("parse JSON-RPC: %w (line=%q)", err, line)
	}
	return m, nil
}
