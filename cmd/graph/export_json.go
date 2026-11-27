// cmd/ckg/export_json.go — single-file JSON export of the graph.
//
// Use case: portable graph snapshot for downstream tools, AI assistants,
// or alternative viewers. One file holds a header, every node (with its
// test role), every edge and the node_prs history rows. What the file does
// not hold is listed once, in exportLeftOut, and written to the help and to
// the header so a reader knows what to fetch from graph.db instead.
//
// Pass --pretty for human-readable indentation; the default packs each row
// on one line for grep/jq friendliness on large files.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// leftOutPart is one part of graph.db that export-json does not write.
type leftOutPart struct {
	Name string
	What string
}

// exportLeftOut is what the v1 export leaves out, the one list both the help
// and the header carry.
var exportLeftOut = []leftOutPart{
	{"blobs", "the source text of each node and the patch of each Hunk node (export-static writes them as files)"},
	{"node_commits", "the links from each node to the Commit nodes that changed it"},
	{"code_edges", "the review index: the links between test code and the production code it reaches"},
	{"pkg_tree", "the package hierarchy"},
	{"topic_tree", "the topic clusters at each resolution"},
	{"nodes_fts", "the full-text search index, derived from the nodes"},
	{"manifest.files", "the per-file list (digests, node ids) the build keeps for staleness checks"},
}

// exportLeftOutNames returns the names of exportLeftOut, in order.
func exportLeftOutNames() []string {
	out := make([]string, len(exportLeftOut))
	for i, p := range exportLeftOut {
		out[i] = p.Name
	}
	return out
}

// exportLeftOutLines renders exportLeftOut as "name: what" lines.
func exportLeftOutLines() []string {
	out := make([]string, len(exportLeftOut))
	for i, p := range exportLeftOut {
		out[i] = p.Name + ": " + p.What
	}
	return out
}

// jsonGraphHeader is the top-level envelope written before the streaming
// nodes/edges arrays. Mirrors persist.Manifest fields the user actually
// wants in a portable export (the build-internal Files[] manifest is
// dropped — it's only useful for incremental cache decisions).
type jsonGraphHeader struct {
	SchemaVersion  string         `json:"schema_version"`
	Engine         string         `json:"engine,omitempty"`
	BuilderVersion string         `json:"builder_version,omitempty"`
	CKGVersion     string         `json:"ckg_version"`
	SrcRoot        string         `json:"src_root"`
	SrcCommit      string         `json:"src_commit,omitempty"`
	BuildTimestamp string         `json:"build_timestamp"`
	Languages      map[string]int `json:"languages,omitempty"`
	// Stats are the counts the build recorded in its manifest. They can
	// differ from what the file holds (rows written after the count);
	// Exported counts the arrays.
	Stats map[string]int `json:"stats,omitempty"`
	// Exported counts the rows of each array of the file (v1 only).
	Exported map[string]int `json:"exported,omitempty"`
	// LeftOut names what the file does not hold (v1 only).
	LeftOut []string `json:"left_out,omitempty"`
}

// minimalNode is the JSON shape emitted under --minimal: drops the
// scoring metadata (PageRank / degree / complexity), the byte-range
// coordinates (StartByte / EndByte — only useful with the matching
// graph.db blob), and the optional textual fields (Visibility / Signature
// / DocComment / SubKind). Keeps the eight fields a downstream consumer
// actually needs to reason about the symbol: ID, Type, Name,
// QualifiedName, FilePath, StartLine/EndLine, Language, Confidence.
type minimalNode struct {
	ID            string           `json:"id"`
	Type          types.NodeType   `json:"type"`
	Name          string           `json:"name"`
	QualifiedName string           `json:"qualified_name"`
	FilePath      string           `json:"file_path"`
	StartLine     int              `json:"start_line"`
	EndLine       int              `json:"end_line"`
	Language      string           `json:"language"`
	Confidence    types.Confidence `json:"confidence"`
}

// minimalEdge keeps the join surface (Src/Dst/Type/Count/Confidence)
// + DispatchKind (the invokes-edge classification — high signal,
// low cost) but drops the auto-increment ID, which is meaningful only
// inside the originating SQLite file.
type minimalEdge struct {
	Src          string           `json:"src"`
	Dst          string           `json:"dst"`
	Type         types.EdgeType   `json:"type"`
	FilePath     string           `json:"file_path,omitempty"`
	Line         int              `json:"line,omitempty"`
	Count        int              `json:"count"`
	Confidence   types.Confidence `json:"confidence"`
	DispatchKind string           `json:"dispatch_kind,omitempty"`
}

// fullNodeRow is a v1 node row: every types.Node field and the node's test
// role (nodes.is_test), which types.Node does not carry.
type fullNodeRow struct {
	types.Node
	IsTest bool `json:"is_test"`
}

// minimalNodeRow is a v1 --minimal node row.
type minimalNodeRow struct {
	minimalNode
	IsTest bool `json:"is_test"`
}

// nodePRRow is one node_prs row: a PR whose merge touched the node. Every
// field is written, empty ones included, so each row has the fields the
// help lists.
type nodePRRow struct {
	NodeID   string    `json:"node_id"`
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Summary  string    `json:"summary"`
	BaseSHA  string    `json:"base_sha"`
	HeadSHA  string    `json:"head_sha"`
	MergedAt time.Time `json:"merged_at"`
	Repo     string    `json:"repo"`
}

func toNodePRRow(nodeID string, r types.PRRef) nodePRRow {
	return nodePRRow{NodeID: nodeID, Number: r.Number, Title: r.Title, Summary: r.Summary,
		BaseSHA: r.BaseSHA, HeadSHA: r.HeadSHA, MergedAt: r.MergedAtUTC, Repo: r.Repo}
}

func toMinimalNode(n types.Node) minimalNode {
	return minimalNode{
		ID: n.ID, Type: n.Type, Name: n.Name, QualifiedName: n.QualifiedName,
		FilePath: n.FilePath, StartLine: n.StartLine, EndLine: n.EndLine,
		Language: n.Language, Confidence: n.Confidence,
	}
}

func toMinimalEdge(e types.Edge) minimalEdge {
	return minimalEdge{
		Src: e.Src, Dst: e.Dst, Type: e.Type, FilePath: e.FilePath,
		Line: e.Line, Count: e.Count, Confidence: e.Confidence,
		DispatchKind: e.DispatchKind,
	}
}

// graphExport is what the v1 export writes: the rows read from graph.db and
// the test role of each file.
type graphExport struct {
	Nodes  []types.Node
	Edges  []types.Edge
	PRs    []nodePRRow
	IsTest func(file string) bool
}

// v2IdentityFlags are the ckg build flags whose values the v2 export needs
// in the manifest; a graph built without them exits with reindex_required.
const v2IdentityFlags = `  ckg build --src DIR --out GRAPH --project-id NAME \
    --dataset-id HEX64 --snapshot-id HEX64 \
    --file-manifest-digest HEX64 --capture-policy-digest HEX64 \
    --source-mode committed|working-tree|snapshot-only`

func exportJSONLong() string {
	return fmt.Sprintf(`Export the graph as one JSON file, for downstream tools that do not
read SQLite. The v1 file (default) is:

  {
    "header": {
      "schema_version": "%s",   (the graph schema the build wrote)
      "engine", "builder_version", "ckg_version", "src_root",
      "src_commit", "build_timestamp", "languages",
      "stats":    {...},  (counts the build recorded in its manifest;
                           they can differ from the arrays)
      "exported": {"nodes": N, "edges": M, "node_prs": P},
                          (rows of each array of this file)
      "left_out": [...]   (what this file does not hold, listed below)
    },
    "nodes":    [ {Node row, with "is_test"} ],
    "edges":    [ {Edge row} ],
    "node_prs": [ {"node_id", "number", "title", "summary", "base_sha",
                   "head_sha", "merged_at", "repo"} ]
                   (every field in every row; an unknown one is "")
  }

Node and Edge rows use the field names of pkg/graph/types (node.go,
edge.go); "is_test" is the test role the build recorded for the node's
file. --minimal keeps fewer fields per row.

Not exported (read graph.db for these):
  %s

--format v2 writes nodes and edges with project, dataset and source
coordinates (no is_test, node_prs or header counts). A failure is
reported as {"code", "message", "cause"}. It needs the graph's
complete source identity in the manifest and otherwise exits with
reindex_required. Build the graph with:

%s

Each HEX64 is 64 hex characters. The export checks only that form: the
identity values are recorded as the build was given them and are not
recomputed from the source. committed and working-tree also need
--src to be a git checkout (its HEAD commit is recorded); snapshot-only
needs a source without git history.`,
		buildpipe.SchemaVersion, strings.Join(exportLeftOutLines(), "\n  "), v2IdentityFlags)
}

func newExportJSONCmd() *cobra.Command {
	var graph, out, format string
	var pretty, minimal bool
	cmd := &cobra.Command{
		Use:   "export-json",
		Short: "Export the graph (nodes, edges, PR history rows) as one JSON file",
		Long:  exportJSONLong(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "v1" && format != "v2" {
				return graphExportError("invalid_request")
			}
			if format == "v2" {
				return exportGraphV2(cmd.Context(), graph, out, pretty, minimal)
			}
			_, cleanup, err := newLogger(rootVerbose, rootLogFile)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}
			defer cleanup()

			db, err := graphDB(graph)
			if err != nil {
				return err
			}
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			manifest, err := store.GetManifest()
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}
			if err := legacyGraphExportAllowed(manifest); err != nil {
				return err
			}
			data, err := loadGraphExport(store)
			if err != nil {
				return err
			}
			return writeGraphJSONFile(out, manifest, data, pretty, minimal)
		},
	}
	cmd.Flags().StringVar(&format, "format", "v1", "output format: v1 (header, nodes, edges, node_prs) or v2 (nodes and edges with project/dataset/source coordinates; needs a graph built with the source identity flags, see above)")
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().StringVar(&out, "out", "", "output JSON file path (required)")
	cmd.Flags().BoolVar(&pretty, "pretty", false,
		"pretty-print with 2-space indentation (larger file; default packs each row on one line)")
	cmd.Flags().BoolVar(&minimal, "minimal", false,
		"drop metadata fields (PageRank / degree / byte ranges / Visibility / Signature / DocComment / SubKind / Edge.ID) — typically 10-20% smaller, but the bigger win is a tighter schema for downstream consumers that don't have the originating graph.db handy")
	_ = cmd.MarkFlagRequired("graph")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

// loadGraphExport reads the rows the v1 export writes.
func loadGraphExport(store persist.StoreReader) (graphExport, error) {
	nodes, err := store.AllNodes()
	if err != nil {
		return graphExport{}, fmt.Errorf("load nodes: %w", err)
	}
	edges, err := store.AllEdges()
	if err != nil {
		return graphExport{}, fmt.Errorf("load edges: %w", err)
	}
	isTest, err := persist.TestFileRule(store)
	if err != nil {
		return graphExport{}, fmt.Errorf("load test roles: %w", err)
	}
	var prs []nodePRRow
	for _, n := range nodes {
		refs, err := store.GetNodePRs(n.ID, time.Time{})
		if err != nil {
			return graphExport{}, fmt.Errorf("load node_prs: %w", err)
		}
		for _, r := range refs {
			prs = append(prs, toNodePRRow(n.ID, r))
		}
	}
	return graphExport{Nodes: nodes, Edges: edges, PRs: prs, IsTest: isTest}, nil
}

// writeGraphJSONFile writes the v1 export to path and reports what it wrote.
func writeGraphJSONFile(path string, m persist.Manifest, data graphExport, pretty, minimal bool) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create out: %w", err)
	}
	w := bufio.NewWriter(f)
	if err := writeGraphJSON(w, m, data, pretty, minimal); err != nil {
		_ = f.Close()
		return fmt.Errorf("write json: %w", err)
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return fmt.Errorf("write json: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close out: %w", err)
	}
	mode := "full"
	if minimal {
		mode = "minimal"
	}
	_, _ = fmt.Fprintf(os.Stderr,
		"ckg: exported %d nodes / %d edges / %d node_prs rows to %s (%s); not exported: %s\n",
		len(data.Nodes), len(data.Edges), len(data.PRs), path, mode, strings.Join(exportLeftOutNames(), ", "))
	return nil
}

// writeGraphJSON streams the envelope and the nodes, edges and node_prs
// arrays. We hand-roll the array brackets so each row is marshalled on its
// own — building the whole structure in memory would peak at ~3x file size
// on a 220K-node graph.
//
// minimal=true emits the slim node and edge shapes; minimal=false preserves
// the full pkg/types schema. Both carry is_test.
func writeGraphJSON(w *bufio.Writer, m persist.Manifest, data graphExport, pretty, minimal bool) error {
	hdr := jsonGraphHeader{
		SchemaVersion:  m.SchemaVersion,
		Engine:         "graph",
		BuilderVersion: m.EffectiveBuilderVersion(),
		CKGVersion:     m.CKGVersion,
		SrcRoot:        m.SrcRoot,
		SrcCommit:      m.SrcCommit,
		BuildTimestamp: m.BuildTimestamp,
		Languages:      m.Languages,
		Stats:          m.Stats,
		Exported:       map[string]int{"nodes": len(data.Nodes), "edges": len(data.Edges), "node_prs": len(data.PRs)},
		LeftOut:        exportLeftOutLines(),
	}
	if _, err := w.WriteString("{\n"); err != nil {
		return err
	}
	if err := writeKeyValue(w, "header", hdr, pretty); err != nil {
		return err
	}
	isTest := data.IsTest
	if isTest == nil {
		isTest = func(string) bool { return false }
	}
	if err := writeJSONArray(w, "nodes", len(data.Nodes), pretty, func(i int) any {
		n := data.Nodes[i]
		if minimal {
			return minimalNodeRow{minimalNode: toMinimalNode(n), IsTest: isTest(n.FilePath)}
		}
		return fullNodeRow{Node: n, IsTest: isTest(n.FilePath)}
	}); err != nil {
		return err
	}
	if err := writeJSONArray(w, "edges", len(data.Edges), pretty, func(i int) any {
		if minimal {
			return toMinimalEdge(data.Edges[i])
		}
		return data.Edges[i]
	}); err != nil {
		return err
	}
	if err := writeJSONArray(w, "node_prs", len(data.PRs), pretty, func(i int) any { return data.PRs[i] }); err != nil {
		return err
	}
	_, err := w.WriteString("\n}\n")
	return err
}

// writeJSONArray writes `,"key": [row,row,...]`, marshalling row(i) for
// i in [0,n) one at a time.
func writeJSONArray(w *bufio.Writer, key string, n int, pretty bool, row func(i int) any) error {
	if _, err := fmt.Fprintf(w, ",\n  %q: [", key); err != nil {
		return err
	}
	for i := range n {
		if i > 0 {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
		if pretty {
			if _, err := w.WriteString("\n    "); err != nil {
				return err
			}
		}
		// json.Encoder appends a trailing newline; Marshal keeps the
		// `[item,item]` shape.
		b, err := json.Marshal(row(i))
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	if pretty && n > 0 {
		if _, err := w.WriteString("\n  "); err != nil {
			return err
		}
	}
	_, err := w.WriteString("]")
	return err
}

// writeKeyValue emits "  key": value" with optional pretty indentation.
// Used for the header object only; arrays are streamed separately.
func writeKeyValue(w *bufio.Writer, key string, v any, pretty bool) error {
	if _, err := fmt.Fprintf(w, "  %q: ", key); err != nil {
		return err
	}
	var b []byte
	var err error
	if pretty {
		b, err = json.MarshalIndent(v, "  ", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
