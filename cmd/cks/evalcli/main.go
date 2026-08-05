// Command cks-eval runs cks retrieval-quality scenarios and emits a
// JSON metric report.
//
// Phase E (Slim, Layer 1) measures cks's evidence-pack quality without
// invoking an LLM:
//
//   - Loads YAML scenarios from -scenarios (file or directory of *.yaml).
//   - Spawns cks-mcp [-config <path>] via mcp-go stdio.
//   - For each scenario, calls cks.context.get_for_task and computes
//     file precision/recall against the scenario's expected_citations.
//   - Folds per-run values into a median per scenario.
//   - Writes a Report JSON to -output (default stdout).
//
// LLM-with-cks metrics (AST diff, semantic diff, test pass rate, PR
// split quality) live in a future Core slice that adds cli-wrapper
// integration.
//
// Usage:
//
//	cks-eval -scenarios ./eval/scenarios/stablenet-pr70.yaml
//	cks-eval -scenarios ./eval/scenarios/ -cks-mcp ./bin/cks-mcp \
//	         -config ./policies/cks.yaml.example -output eval/reports/run.json
package evalcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/eval"
	"github.com/spf13/cobra"
)

// NewCmd builds the `cks eval` command: run retrieval-quality scenarios
// against the fused MCP server and emit a JSON metric report.
func NewCmd() *cobra.Command {
	var scenarios, mcpBinary, mcpConfig, output, verifyAnchors string
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Run retrieval-quality scenarios and emit a JSON metric report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if scenarios == "" {
				return fmt.Errorf("--scenarios is required")
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return run(ctx, scenarios, mcpBinary, mcpConfig, output, verifyAnchors)
		},
	}
	cmd.Flags().StringVar(&scenarios, "scenarios", "", "scenario YAML file or directory (required)")
	cmd.Flags().StringVar(&mcpBinary, "cks-mcp", "", "MCP server binary (empty = this cks binary's own mcp subcommand)")
	cmd.Flags().StringVar(&mcpConfig, "config", "", "path to cks.yaml forwarded to the server")
	cmd.Flags().StringVar(&output, "output", "", "write report to this file (empty = stdout)")
	cmd.Flags().StringVar(&verifyAnchors, "verify-anchors", "", "source root for anchor verification: fail before running when any scenario's expected span no longer contains its declared anchor (guards against line drift)")
	return cmd
}

func run(ctx context.Context, scenariosPath, mcpBinary, mcpConfig, outputPath, anchorRoot string) error {
	paths, err := collectScenarioPaths(scenariosPath)
	if err != nil {
		return fmt.Errorf("collect scenarios: %w", err)
	}
	if len(paths) == 0 {
		return fmt.Errorf("no scenarios found at %q", scenariosPath)
	}

	// Anchor verification runs BEFORE any MCP work: a drifted scenario is
	// a measurement bug, not a retrieval result, so fail fast with every
	// violation listed rather than emitting misleading zeros.
	if anchorRoot != "" {
		var violations []error
		for _, p := range paths {
			s, err := eval.LoadScenario(p)
			if err != nil {
				return fmt.Errorf("load %q: %w", p, err)
			}
			violations = append(violations, s.VerifyAnchors(anchorRoot)...)
		}
		if len(violations) > 0 {
			for _, v := range violations {
				log.Printf("cks-eval: anchor drift: %v", v)
			}
			return fmt.Errorf("%d anchor violation(s) — re-anchor the scenarios before measuring", len(violations))
		}
	}

	runner, err := eval.NewRunner(ctx, eval.RunnerOpts{
		CKSMCPBinary: mcpBinary,
		CKSMCPConfig: mcpConfig,
	})
	if err != nil {
		return fmt.Errorf("start runner: %w", err)
	}
	defer func() { _ = runner.Close() }()

	// Index-freshness warning: expected spans are anchored to the tree at
	// anchorRoot, but the index may have been built from an older commit —
	// then returned chunk coordinates cannot line up with current-tree
	// spans and misses are measurement artifacts, not retrieval results.
	// Warn loud instead of failing: measuring a deliberately pinned older
	// snapshot is a legitimate workflow.
	if anchorRoot != "" {
		if head, ferr := runner.IndexedHead(ctx); ferr == nil && head != "" {
			if out, gerr := exec.Command("git", "-C", anchorRoot, "rev-parse", "HEAD").Output(); gerr == nil {
				treeHead := strings.TrimSpace(string(out))
				if treeHead != "" && treeHead != head {
					log.Printf("cks-eval: WARNING: index indexed_head %.12s != tree HEAD %.12s at %s — "+
						"chunk line coordinates come from the indexed commit; span misses may be stale-index artifacts. "+
						"Rebuild the index (make dogfood-eval rebuilds it) or measure against the indexed commit.",
						head, treeHead, anchorRoot)
				}
			}
		}
	}

	report := eval.NewReport(builderVersion)
	for _, p := range paths {
		s, err := eval.LoadScenario(p)
		if err != nil {
			return fmt.Errorf("load %q: %w", p, err)
		}
		result, err := runner.Execute(ctx, s)
		if err != nil {
			return fmt.Errorf("execute %q: %w", p, err)
		}
		report.Results = append(report.Results, *result)
	}
	report.FinishedAt = time.Now().UTC()
	report.IntentSummary = eval.SummarizeByIntent(report.Results)

	out, closer, err := openOutput(outputPath)
	if err != nil {
		return fmt.Errorf("open output: %w", err)
	}
	defer func() { _ = closer() }()

	if err := eval.WriteJSON(out, report); err != nil {
		return err
	}

	// Knowledge guard, mirroring the anchor guard above: the report is
	// written first so the numbers survive, then a missing expected
	// knowledge scope fails the run. Recall and MRR are citation-based
	// and cannot see the pack's knowledge section, so without this a
	// break there is silent — the same blind spot that hid convention
	// chunks from every pack until #72.
	missing := 0
	for _, r := range report.Results {
		for _, scope := range r.KnowledgeMissing {
			log.Printf("cks-eval: knowledge missing: %s: expected scope %q absent from pack.knowledge", r.Name, scope)
			missing++
		}
	}
	if missing > 0 {
		return fmt.Errorf("%d expected knowledge scope(s) not delivered", missing)
	}
	return nil
}

// collectScenarioPaths accepts either a single YAML file or a
// directory containing *.yaml / *.yml files. Returns paths sorted
// lexicographically so report ordering is deterministic per invocation.
func collectScenarioPaths(p string) ([]string, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{p}, nil
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		out = append(out, filepath.Join(p, name))
	}
	if len(out) == 0 {
		return nil, errors.New("directory contains no *.yaml / *.yml files")
	}
	return out, nil
}

func openOutput(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// builderVersion tags eval reports. Informational only.
var builderVersion = "cks-eval/0.0.1-dev"
