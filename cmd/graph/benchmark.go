// cmd/ckg/benchmark.go — `ckg benchmark` measures the token-reduction
// payoff of using the graph instead of feeding the agent the raw corpus.
// Inspired by graphify's benchmark.run_benchmark (benchmark.py:64-110)
// but adapted to CKG's richer node/edge schema:
//
//   - Corpus baseline:  sum of source-file bytes for the indexed languages,
//     converted to tokens via the standard ~4 chars-per-
//     token heuristic (matches OpenAI/Anthropic averages
//     for English+code; fine for ratio reporting).
//
//   - Graph-query cost: pick the top-N god nodes as proxy "questions",
//     BFS k=3 hops from each, render the subgraph as
//     a compact text answer (`Symbol → Symbol` chains),
//     and count the resulting tokens.
//
//   - Reduction ratio:  corpus_tokens / avg(query_tokens). A 100x ratio
//     means the graph reaches the same answer using
//     1% of the tokens a naive grep-everything pass
//     would consume.
//
// The numbers are approximate by design — point estimates for the
// "is the graph worth the build cost" decision, not precision metrics.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// charsPerToken is the standard rule-of-thumb for English/code tokenisers
// (GPT-style BPE averages 3.5-4.5 chars/token on natural language, ~3
// chars/token on code). 4 keeps the math conservative — it overestimates
// the corpus a bit and underestimates the query, which biases toward
// not-overselling the graph's reduction ratio.
const charsPerToken = 4

func newBenchmarkCmd() *cobra.Command {
	var graph string
	var questions, depth, maxNodes int
	var history bool
	var src string
	var format string
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Measure token reduction of graph queries vs reading the raw corpus",
		Long: fmt.Sprintf(`Estimate how many tokens an agent saves by querying the graph
instead of grepping the source corpus. Picks the top-N god nodes
as proxy "questions", walks --depth hops from each, renders the
reached nodes as a compact text answer, and divides the corpus token
count by the average answer token count.

Each answer renders at most --max-nodes nodes (default %d, the seed
included; 0 renders every reached node). God nodes reach far more
nodes than that within a few hops, so with the default cap a deeper
--depth usually changes nothing and the ratio describes the cut
answer. The output says, per query, how many nodes the walk reached
within the depth, how many it rendered and how many the cap cut.

The walk follows history edges too by default (--history=true). Commit
nodes join every symbol a commit touched, so on a graph built with
history a few hops through them reach most of the graph; --history=false
leaves those edges out, and the output says how many there are.

The numbers are rough — the corpus baseline uses 4-chars-per-token
and the query renderer is intentionally simple. Treat the ratio
as a directional indicator rather than a precision benchmark.`, defaultBenchmarkMaxNodes),
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxNodes < 0 {
				return fmt.Errorf("--max-nodes must be 0 or more, got %d", maxNodes)
			}
			if format != "json" && format != "text" && format != "" {
				return fmt.Errorf("unknown --format %q (want text|json)", format)
			}
			store, err := persist.OpenReadOnly(filepath.Join(graph, "graph.db"))
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			manifest, err := store.GetManifest()
			if err != nil {
				return fmt.Errorf("manifest: %w", err)
			}
			nodes, err := store.AllNodes()
			if err != nil {
				return fmt.Errorf("nodes: %w", err)
			}
			edges, err := store.AllEdges()
			if err != nil {
				return fmt.Errorf("edges: %w", err)
			}

			gods := topPageRank(nodes, questions)
			if len(gods) == 0 {
				_, _ = fmt.Fprintln(os.Stderr,
					"benchmark: no PageRank-ranked nodes found — was the graph built without scoring?")
				return nil
			}
			srcRoot, err := resolveSourceRoot(src, graph, manifest.SrcRoot)
			if err != nil {
				return fmt.Errorf("source root: %w", err)
			}
			corpusTokens, err := estimateCorpusTokens(srcRoot, nodes)
			if err != nil {
				return err
			}
			report := runBenchmark(manifest, corpusTokens, nodes, edges, gods, depth, maxNodes, history)
			report.SrcRoot = srcRoot
			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			printBenchmark(os.Stdout, report)
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().StringVar(&src, "src", "", "source root to count the corpus from (default: the manifest's src_root; a relative one is resolved against the folder that holds --graph, never the working directory)")
	cmd.Flags().IntVar(&questions, "questions", 5,
		"number of top-PageRank god nodes to sample as proxy queries")
	cmd.Flags().IntVar(&depth, "depth", 3,
		"hops walked from each proxy query's node (the answer is still cut at --max-nodes)")
	cmd.Flags().BoolVar(&history, "history", defaultBenchmarkHistory,
		"walk the history edges too (the "+historyAxis+" axis: changed_in, blame, has_hunk, adjacent, modifies); --history=false leaves them out")
	cmd.Flags().IntVar(&maxNodes, "max-nodes", defaultBenchmarkMaxNodes,
		"most nodes one answer renders, the seed included; 0 renders every node reached within --depth")
	cmd.Flags().StringVar(&format, "format", "text",
		"output format: text|json. json is the machine-readable form consumed by 'make eval' for baseline diffing.")
	_ = cmd.MarkFlagRequired("graph")
	return cmd
}

// benchmarkReport is the JSON-serialisable shape of the benchmark output.
// Stable field names because eval/baseline/*.json diffs against this
// shape across runs — renaming a field would break the regression gate.
// Depth, MaxNodes and the per-query node counts say what was compared.
type benchmarkReport struct {
	SrcRoot      string  `json:"src_root"`
	SrcCommit    string  `json:"src_commit,omitempty"`
	CorpusTokens int     `json:"corpus_tokens"`
	AvgQueryTok  int     `json:"avg_query_tokens"`
	Reduction    float64 `json:"reduction_ratio"`
	Depth        int     `json:"depth"`
	MaxNodes     int     `json:"max_nodes"`
	QueriesCut   int     `json:"queries_cut"`
	// History says whether the walk followed history edges, and
	// HistoryEdges how many the graph holds.
	History      bool               `json:"history"`
	HistoryEdges int                `json:"history_edges"`
	PerQuery     []benchmarkPerQRow `json:"per_query"`
}

type benchmarkPerQRow struct {
	QualifiedName string  `json:"qualified_name"`
	Tokens        int     `json:"tokens"`
	Reduction     float64 `json:"reduction_ratio"`
	NodesRendered int     `json:"nodes_rendered"`
	NodesInDepth  int     `json:"nodes_within_depth"`
	NodesCut      int     `json:"nodes_cut"`
}

// runBenchmark walks from each god node and compares the answers with the
// corpus.
func runBenchmark(m persist.Manifest, corpusTokens int, nodes []types.Node, edges []types.Edge, gods []types.Node, depth, maxNodes int, history bool) benchmarkReport {
	idx := make(map[string]types.Node, len(nodes))
	for _, n := range nodes {
		idx[n.ID] = n
	}
	walked, historyEdges := benchmarkWalkEdges(edges, history)
	adj := buildAdjacency(walked)
	r := benchmarkReport{SrcRoot: m.SrcRoot, SrcCommit: m.SrcCommit, CorpusTokens: corpusTokens, Depth: depth, MaxNodes: maxNodes,
		History: history, HistoryEdges: historyEdges}
	total := 0
	for _, gn := range gods {
		w := walkQuery(gn, idx, adj, depth, maxNodes)
		r.PerQuery = append(r.PerQuery, benchmarkPerQRow{
			QualifiedName: gn.QualifiedName, Tokens: w.Tokens, Reduction: ratio(corpusTokens, w.Tokens),
			NodesRendered: w.Rendered, NodesInDepth: w.WithinDepth, NodesCut: w.Cut,
		})
		total += w.Tokens
		if w.Cut > 0 {
			r.QueriesCut++
		}
	}
	if len(r.PerQuery) > 0 {
		r.AvgQueryTok = total / len(r.PerQuery)
	}
	r.Reduction = ratio(corpusTokens, r.AvgQueryTok)
	return r
}

// resolveSourceRoot is the one rule for where the benchmark reads the
// source: --src when given; otherwise the manifest's src_root when it is
// absolute; otherwise that relative src_root resolved against the folder
// that holds the graph directory (ckg build --src src --out graph run in
// one folder). Never the working directory.
func resolveSourceRoot(srcFlag, graphDir, manifestRoot string) (string, error) {
	switch {
	case srcFlag != "":
		return filepath.Abs(srcFlag)
	case filepath.IsAbs(manifestRoot):
		return manifestRoot, nil
	case manifestRoot == "":
		return "", errors.New("the manifest records no src_root; give --src")
	}
	absGraph, err := filepath.Abs(graphDir)
	if err != nil {
		return "", fmt.Errorf("resolve --graph: %w", err)
	}
	return filepath.Join(filepath.Dir(absGraph), manifestRoot), nil
}

// estimateCorpusTokens sums the on-disk bytes of every file the graph has
// a File node for, under srcRoot. It fails when any of them is missing:
// a corpus counted from part of the source would make the ratio wrong.
func estimateCorpusTokens(srcRoot string, nodes []types.Node) (int, error) {
	totalBytes, files := 0, 0
	var missing []string
	seen := map[string]bool{}
	for _, n := range nodes {
		if n.Type != types.NodeFile || n.FilePath == "" || seen[n.FilePath] {
			continue
		}
		seen[n.FilePath] = true
		files++
		st, err := os.Stat(filepath.Join(srcRoot, n.FilePath))
		if err != nil || st.IsDir() {
			missing = append(missing, n.FilePath)
			continue
		}
		totalBytes += int(st.Size())
	}
	if files == 0 {
		return 0, errors.New("the graph has no File nodes to count the corpus from")
	}
	if len(missing) > 0 {
		return 0, fmt.Errorf("source not found: %d of %d graph files are missing under %s (first: %s); give --src with the source root the graph was built from "+
			"(a relative src_root is relative to where ckg build ran, which the graph does not record)",
			len(missing), files, srcRoot, missing[0])
	}
	return totalBytes / charsPerToken, nil
}

// buildAdjacency returns id → []neighbourID, undirected. We build it
// once per benchmark invocation and share across all queries — the
// per-question BFS is the hot loop.
func buildAdjacency(edges []types.Edge) map[string][]string {
	adj := make(map[string][]string, len(edges)*2)
	for _, e := range edges {
		adj[e.Src] = append(adj[e.Src], e.Dst)
		adj[e.Dst] = append(adj[e.Dst], e.Src)
	}
	return adj
}

// historyAxis is the report axis of the history edges (edgeToAxis):
// changed_in, blame, has_hunk, adjacent and modifies.
const historyAxis = "G6"

// defaultBenchmarkHistory keeps history edges in the walk by default, as
// the benchmark always walked them; the output says how many there are.
const defaultBenchmarkHistory = true

// benchmarkWalkEdges returns the edges the walk follows and how many
// history edges the graph holds. A Commit node joins every symbol a commit
// touched, so through history edges a few hops reach most of the graph.
func benchmarkWalkEdges(edges []types.Edge, history bool) ([]types.Edge, int) {
	n := 0
	kept := edges[:0:0]
	for _, e := range edges {
		if edgeToAxis(e.Type) == historyAxis {
			n++
			if !history {
				continue
			}
		}
		kept = append(kept, e)
	}
	return kept, n
}

// defaultBenchmarkMaxNodes is how many nodes one query answer renders by
// default. God nodes can have thousands of immediate neighbours (e.g.
// `Errorf` with ~3500 inbound calls); a depth-3 walk from such a seed
// without a cap reaches hundreds of thousands of nodes, so the default
// answer is cut and the reported ratio describes the cut answer.
const defaultBenchmarkMaxNodes = 200

// walkStat is what one proxy query's walk rendered and what it reached.
type walkStat struct {
	// Tokens estimates the rendered answer.
	Tokens int
	// Rendered is how many nodes the answer holds, the seed included.
	Rendered int
	// WithinDepth is how many nodes the walk reaches within the depth,
	// the seed included, whether rendered or not.
	WithinDepth int
	// Cut is WithinDepth minus Rendered: nodes the cap left out.
	Cut int
}

// walkQuery simulates the cost of an "explain X" agent query: a
// breadth-first walk of depth hops from the seed, rendering each reached
// node as a compact line ("Symbol → Symbol [type] qname") until maxNodes
// nodes are rendered (0 renders every reached node). The walk goes on to
// the full depth after the cap to count what the cap cut.
func walkQuery(seed types.Node, idx map[string]types.Node, adj map[string][]string, depth, maxNodes int) walkStat {
	visited := map[string]bool{seed.ID: true}
	frontier := []string{seed.ID}
	var rendered strings.Builder
	rendered.WriteString(seed.Name)
	rendered.WriteString(" — ")
	rendered.WriteString(seed.QualifiedName)
	rendered.WriteString("\n")
	st := walkStat{Rendered: 1, WithinDepth: 1}
	for d := 0; d < depth && len(frontier) > 0; d++ {
		next := make([]string, 0, len(frontier))
		for _, id := range frontier {
			cur := idx[id]
			for _, nb := range adj[id] {
				if visited[nb] {
					continue
				}
				visited[nb] = true
				other, ok := idx[nb]
				if !ok {
					continue
				}
				st.WithinDepth++
				next = append(next, nb)
				if maxNodes > 0 && st.Rendered >= maxNodes {
					continue
				}
				st.Rendered++
				rendered.WriteString("  ")
				rendered.WriteString(cur.Name)
				rendered.WriteString(" → ")
				rendered.WriteString(other.Name)
				rendered.WriteString(" [")
				rendered.WriteString(string(other.Type))
				rendered.WriteString("] ")
				rendered.WriteString(other.QualifiedName)
				rendered.WriteString("\n")
			}
		}
		frontier = next
	}
	st.Cut = st.WithinDepth - st.Rendered
	st.Tokens = rendered.Len() / charsPerToken
	return st
}

func ratio(corpus, query int) float64 {
	if query <= 0 {
		return 0
	}
	return float64(corpus) / float64(query)
}

func printBenchmark(w *os.File, r benchmarkReport) {
	limit := "no cap"
	if r.MaxNodes > 0 {
		limit = fmt.Sprintf("at most %s nodes per answer", humanInt(r.MaxNodes))
	}
	_, _ = fmt.Fprintln(w, "ckg token-reduction benchmark")
	_, _ = fmt.Fprintf(w, "  Source:    %s\n", r.SrcRoot)
	_, _ = fmt.Fprintf(w, "  Corpus:    ~%s tokens (sum of indexed source files)\n",
		humanInt(r.CorpusTokens))
	_, _ = fmt.Fprintf(w, "  Walk:      depth %d, %s (--max-nodes)\n", r.Depth, limit)
	switch {
	case r.HistoryEdges == 0:
		_, _ = fmt.Fprintln(w, "  History:   the graph has no history edges")
	case r.History:
		_, _ = fmt.Fprintf(w, "  History:   walked %s history edges (--history); Commit nodes join every symbol a commit touched, so reach through them is not code reach; --history=false leaves them out\n",
			humanInt(r.HistoryEdges))
	default:
		_, _ = fmt.Fprintf(w, "  History:   left out %s history edges (--history=false)\n", humanInt(r.HistoryEdges))
	}
	_, _ = fmt.Fprintf(w, "  Avg query: ~%s tokens (top-%d god nodes)\n",
		humanInt(r.AvgQueryTok), len(r.PerQuery))
	if r.AvgQueryTok > 0 {
		_, _ = fmt.Fprintf(w, "  Reduction: %.1fx fewer tokens per query\n", r.Reduction)
	}
	if r.QueriesCut > 0 {
		_, _ = fmt.Fprintf(w, "  Cut:       %d of %d answers hold fewer nodes than the walk reached; the ratio describes the cut answers\n",
			r.QueriesCut, len(r.PerQuery))
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Per-question breakdown:")
	for i, q := range r.PerQuery {
		_, _ = fmt.Fprintf(w, "  %d. `%s` → ~%s tokens (%.1fx reduction); rendered %s of %s nodes within depth %d, %s cut\n",
			i+1, q.QualifiedName, humanInt(q.Tokens), q.Reduction,
			humanInt(q.NodesRendered), humanInt(q.NodesInDepth), r.Depth, humanInt(q.NodesCut))
	}
}

// humanInt formats large integers with commas for the benchmark output.
// Saved here rather than depending on a third-party fmt humaniser to
// keep the binary lean.
func humanInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
