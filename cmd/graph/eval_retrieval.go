// cmd/ckg/eval_retrieval.go — `ckg eval-retrieval` runs the LLM-free
// retrieval probes from eval/retrieval/*.yaml against a built graph
// and emits per-fixture recall/precision/F1 plus an aggregate gate
// result.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/eval/retrieval"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// retrievalExampleFixture is a fixture shipped with the repository, given
// in the help as the example; it runs against a graph of
// graph/testdata/synthetic.
const retrievalExampleFixture = "graph/eval/retrieval/R03-find-callers-service-new.yaml"

// defaultRetrievalFixtures is the default --fixtures directory. It is
// relative to the working directory and exists when ckg runs in graph/.
const defaultRetrievalFixtures = "eval/retrieval"

func newEvalRetrievalCmd() *cobra.Command {
	var (
		graph    string
		fixtures string
		output   string
	)
	cmd := &cobra.Command{
		Use:   "eval-retrieval",
		Short: "Run LLM-free retrieval-accuracy probes against a graph",
		Long: `Load every YAML fixture under --fixtures, run each probe
against the graph at --graph, and score the result symbols against the
fixture's expected set. Per-fixture recall/precision/F1 plus an
aggregate gate (any fixture below its threshold → non-zero exit).

The probe-tool layer mirrors the MCP tools (find_callers, find_callees,
find_symbol, search_text) so fixture YAML stays close to how a real
consumer would call them.

A fixture is one YAML file (*.yaml or *.yml) with these fields:

  id                     unique name of the probe (required)
  description            free text
  probe.tool             find_callers, find_callees, find_symbol or
                         search_text (required)
  probe.args             the tool's arguments: qname and depth (callers,
                         callees), name, exact, language, kinds
                         (find_symbol), query, top_k, language, mode,
                         node_kinds (search_text)
  expected.symbols       qualified names the probe must return (required,
                         not empty)
  scoring.recall_min     pass threshold for recall (default 0)
  scoring.precision_min  pass threshold for precision (default 0)

An example shipped with the repository is
` + retrievalExampleFixture + `; its directory holds
the fixtures for a graph of graph/testdata/synthetic. From graph/:

  ckg build --src=testdata/synthetic --out=eval/.synthetic-data --lang=go,ts,sol
  ckg eval-retrieval --graph=eval/.synthetic-data --fixtures=eval/retrieval

The default --fixtures (` + defaultRetrievalFixtures + `) is relative to the
working directory, so it exists only when ckg runs in graph/.

Output JSON shape:
  {
    "graph": "...",
    "fixtures_total": N,
    "passed": M,
    "failed": K,
    "aggregate": { "recall": ..., "precision": ..., "f1": ... },
    "results": [ { "id": ..., "score": {...}, "got": [...], "pass": bool }, ... ]
  }

Exit codes:
  0  every fixture met its threshold
  1  one or more fixtures failed
  2  the run could not be made (graph open, missing fixtures
     directory, fixture parse, etc.); a command-line flag error exits 1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFixturesDir(fixtures); err != nil {
				return err
			}
			db := filepath.Join(graph, "graph.db")
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			fs, err := retrieval.LoadFixtures(fixtures)
			if err != nil {
				return fmt.Errorf("load fixtures: %w", err)
			}
			if len(fs) == 0 {
				return fmt.Errorf("no *.yaml fixtures found under %s", fixtures)
			}

			results, err := retrieval.RunAll(store, fs)
			if err != nil {
				return fmt.Errorf("run fixtures: %w", err)
			}

			report := buildRetrievalReport(graph, results)
			if err := writeReport(output, report); err != nil {
				return err
			}
			// Print a one-line stderr summary so humans see the gate
			// outcome without scrolling through JSON. The numeric
			// detail lives in the report file (or stdout when
			// --output=-).
			_, _ = fmt.Fprintf(os.Stderr,
				"eval-retrieval: %d/%d passed (aggregate R=%.2f P=%.2f F1=%.2f)\n",
				report.Passed, report.FixturesTotal,
				report.Aggregate.Recall, report.Aggregate.Precision, report.Aggregate.F1)
			if report.Failed > 0 {
				// Non-zero exit, but not via Cobra's error wrapping —
				// the error message would shadow the JSON output. Use
				// os.Exit so the report file already wrote.
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory containing graph.db (required)")
	cmd.Flags().StringVar(&fixtures, "fixtures", defaultRetrievalFixtures,
		"directory containing retrieval *.yaml fixtures (relative to the working directory)")
	cmd.Flags().StringVar(&output, "output", "-",
		"path for JSON output ('-' for stdout)")
	_ = cmd.MarkFlagRequired("graph")
	run := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if err := run(c, args); err != nil {
			return exitCodeError{Code: evalRetrievalInternalExit, Err: err}
		}
		return nil
	}
	return cmd
}

// evalRetrievalInternalExit is the exit code of a run that could not be
// made (the help's code 2).
const evalRetrievalInternalExit = 2

// requireFixturesDir fails with the directory's name, and for a relative
// path the directory it was looked for in, when dir is not a directory.
func requireFixturesDir(dir string) error {
	st, err := os.Stat(dir)
	if err == nil && st.IsDir() {
		return nil
	}
	where := dir
	if abs, absErr := filepath.Abs(dir); absErr == nil && abs != dir {
		where = fmt.Sprintf("%s (looked for at %s)", dir, abs)
	}
	if err == nil {
		return fmt.Errorf("--fixtures %s is not a directory", where)
	}
	return fmt.Errorf("--fixtures %s: %w; give the directory of the fixture YAML files (see --help)", where, err)
}

// retrievalReport is the JSON shape consumed by make eval +
// eval/baseline/retrieval.json. Stable field names — renaming any of
// these breaks the regression diff.
type retrievalReport struct {
	Graph         string          `json:"graph"`
	FixturesTotal int             `json:"fixtures_total"`
	Passed        int             `json:"passed"`
	Failed        int             `json:"failed"`
	Aggregate     retrieval.Score `json:"aggregate"`
	Results       []retrievalRow  `json:"results"`
}

type retrievalRow struct {
	ID            string          `json:"id"`
	Tool          string          `json:"tool"`
	Pass          bool            `json:"pass"`
	PassRecall    bool            `json:"pass_recall"`
	PassPrecision bool            `json:"pass_precision"`
	Score         retrieval.Score `json:"score"`
	Got           []string        `json:"got"`
}

// buildRetrievalReport collapses per-fixture results into the JSON shape.
// Aggregate recall/precision/F1 are micro-averaged over the union of
// all expected/got sets — single number that summarises the run, no
// per-fixture weighting choice required.
func buildRetrievalReport(graph string, results []retrieval.Result) retrievalReport {
	r := retrievalReport{
		Graph:         graph,
		FixturesTotal: len(results),
		Results:       make([]retrievalRow, 0, len(results)),
	}
	var allExp, allGot []string
	for _, res := range results {
		if res.Pass() {
			r.Passed++
		} else {
			r.Failed++
		}
		r.Results = append(r.Results, retrievalRow{
			ID:            res.Fixture.ID,
			Tool:          res.Fixture.Probe.Tool,
			Pass:          res.Pass(),
			PassRecall:    res.PassRecall,
			PassPrecision: res.PassPrecision,
			Score:         res.Score,
			Got:           res.Got,
		})
		allExp = append(allExp, res.Fixture.Expected.Symbols...)
		allGot = append(allGot, res.Got...)
	}
	r.Aggregate = retrieval.ComputeScore(allExp, allGot)
	// Aggregate diagnostic lists would be huge and uninteresting (a
	// per-fixture detail already shows them) — drop to keep the
	// top-level report tight.
	r.Aggregate.Missing = nil
	r.Aggregate.Extra = nil
	return r
}

func writeReport(path string, report retrievalReport) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if path == "-" || path == "" {
		_, _ = os.Stdout.Write(payload)
		_, _ = os.Stdout.Write([]byte{'\n'})
		return nil
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
